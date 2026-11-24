package dal

import (
	"context"
	"fmt"
	"time"
)

// seedJobs is an additive, one-time local fixture. The transaction and marker
// preserve manual edits (including deletions) when just fake-data runs again.
func (s *Store) seedJobs(ctx context.Context, now time.Time) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(73194621)`); err != nil {
		return err
	}
	var exists bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM app.audit_events WHERE action='development_seed.jobs_v1')`).Scan(&exists); err != nil {
		return err
	}
	if exists {
		return nil
	}

	// Reuse the existing local cohort so every referral has a working profile.
	rows, err := tx.Query(ctx, `SELECT user_id FROM app.user_profiles WHERE slug IN ('dev-student','dev-mentor','dev-coordinator','dev-site-admin','dev-director','dev-admin-mentor') OR starts_with(slug,'dev-cohort-') ORDER BY slug`)
	if err != nil {
		return err
	}
	var users []string
	for rows.Next() {
		var userID string
		if err = rows.Scan(&userID); err != nil {
			rows.Close()
			return err
		}
		users = append(users, userID)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	if len(users) == 0 {
		return fmt.Errorf("seed Jobs: development users missing")
	}

	companies := []struct{ name, category string }{
		{"Google", "faang"}, {"Amazon", "faang"},
		{"Canva", "big_tech"}, {"Atlassian", "big_tech"},
		{"Optiver", "quant"}, {"Jane Street", "quant"},
		{"Dolby", "other"}, {"Bank of America", "other"},
	}
	roles := []struct {
		title, kind, eligibility, details string
		locations                         []string
	}{
		{"Software Engineering Intern", "intern", "Penultimate-year students. Available for a full-time 12-week summer placement.", "Local sample listing. Team matching happens after the interviews.\nInclude your expected graduation date and a link to one project you can explain in detail.", []string{"Sydney, Australia", "Melbourne, Australia"}},
		{"Graduate Software Engineer", "grad", "Graduating this year or within the previous two years. Australian work rights required.", "Local sample listing. Rotations across backend, infrastructure and product teams.\nThe process includes an online assessment, technical interview and team conversation.", []string{"Sydney, Australia"}},
		{"Senior Platform Engineer — Developer Infrastructure", "experienced", "Three or more years of production engineering experience. Familiarity with distributed systems and on-call operations.", "Local sample listing. Flexible hybrid arrangement; discuss office attendance with the hiring team.\nExperience with Go, observability and cloud infrastructure is useful. Applications are reviewed on a rolling basis.", []string{"Sydney, Australia", "Melbourne, Australia", "Remote, Australia"}},
	}
	referralNotes := []string{
		"Happy to refer interns and graduates. Send your CV, the listing URL and a short introduction. Please contact me before applying; I cannot attach referrals retrospectively.",
		"Backend and infrastructure roles only. Please include a project demonstrating your experience and confirm your work rights. Allow a week for a reply.",
		"Open to internship referrals for penultimate-year students. Include your graduation month and availability for the full summer placement.",
		"Experienced engineering roles only (3+ years). Send the job ID and a concise summary of why your background matches. I can review two requests per week.",
		"Graduate roles in Sydney or Melbourne. Happy to review a CV first. Please send one message with the role link, CV and preferred location.",
		"Please check all eligibility criteria first. I can refer candidates for Australian offices, but cannot answer questions about overseas hiring or sponsorship.",
	}
	var positions []string
	for c, company := range companies {
		if _, err = tx.Exec(ctx, `INSERT INTO app.job_companies(name,category) VALUES($1,$2) ON CONFLICT (lower(trim(name))) DO NOTHING`, company.name, company.category); err != nil {
			return err
		}
		var companyID string
		if err = tx.QueryRow(ctx, `SELECT id FROM app.job_companies WHERE lower(trim(name))=lower($1)`, company.name).Scan(&companyID); err != nil {
			return err
		}
		for r, role := range roles {
			status := "open"
			if c%3 == 0 && r == 2 {
				status = "closed"
			}
			url := fmt.Sprintf("https://example.invalid/rsp-fake-jobs/%d/%d", c, r)
			details := role.details
			if c == 7 && r == 0 {
				details = ""
			} // An empty Details case too.
			var positionID string
			if err = tx.QueryRow(ctx, `INSERT INTO app.job_positions(company_id,title,job_type,eligibility,application_url,normalized_url,locations,notes,status,created_by_user_id,created_at,updated_at) VALUES($1,$2,$3,$4,$5,$5,$6,$7,$8,$9,$10,$10) RETURNING id`, companyID, role.title+" (Local Sample)", role.kind, role.eligibility, url, role.locations, details, status, users[c%len(users)], now.AddDate(0, 0, -(c*3+r))).Scan(&positionID); err != nil {
				return err
			}
			positions = append(positions, positionID)
		}
		for n := 0; n < 3; n++ {
			if _, err = tx.Exec(ctx, `INSERT INTO app.job_referrals(user_id,company_id,notes,matching_enabled) VALUES($1,$2,$3,true) ON CONFLICT(user_id,company_id) DO NOTHING`, users[(c*3+n)%len(users)], companyID, referralNotes[(c+n)%len(referralNotes)]); err != nil {
				return err
			}
		}
	}

	// Every fake login and cohort member gets twelve private applications, with
	// enough variety for pagination, filters, stage editing and reordering.
	stageTitles := []string{"Applied", "Online Assessment", "Recruiter Screen", "Technical Interview", "Team Interview"}
	stageNotes := []string{"Tailored my CV to this role and saved the application confirmation.", "Two coding questions in 90 minutes. Practise graph traversal and explain complexity clearly.", "Discussed graduation date, location preferences and team interests. Follow up next week if there is no response.", "Implemented a cache and discussed eviction strategies. Review concurrency and boundary cases before the next round.", "Talked through ownership of a university project and handling an incident. Ask about onboarding and mentorship."}
	for u, userID := range users {
		for a := 0; a < 12; a++ {
			status := "active"
			if a%4 == 2 {
				status = "inactive"
			}
			if a%4 == 3 {
				status = "offer"
			}
			start := now.AddDate(0, 0, -60+a*2)
			var applicationID string
			if err = tx.QueryRow(ctx, `INSERT INTO app.job_applications(user_id,position_id,status,created_at) VALUES($1,$2,$3,$4) RETURNING id`, userID, positions[(a+u)%len(positions)], status, start).Scan(&applicationID); err != nil {
				return err
			}
			count := 1 + a%5
			if status == "offer" {
				count = 5
			}
			for stage := 0; stage < count; stage++ {
				notes := stageNotes[stage]
				if a%3 == 0 && stage == 0 {
					notes = ""
				}
				if _, err = tx.Exec(ctx, `INSERT INTO app.job_application_stages(application_id,title,happened_on,notes,initial,sort_order) VALUES($1,$2,$3,$4,$5,$6)`, applicationID, stageTitles[stage], start.AddDate(0, 0, stage*4), notes, stage == 0, stage); err != nil {
					return err
				}
			}
			if status != "active" {
				title, notes := "Rejected", "Received the outcome by email. Requested feedback and saved this history for the next hiring cycle."
				if status == "offer" {
					title, notes = "Offer", "Written offer received. Review the start date, location and response deadline before deciding."
				}
				if _, err = tx.Exec(ctx, `INSERT INTO app.job_application_stages(application_id,title,happened_on,notes,sort_order) VALUES($1,$2,$3,$4,$5)`, applicationID, title, start.AddDate(0, 0, count*4), notes, count); err != nil {
					return err
				}
			}
		}
	}
	if _, err = tx.Exec(ctx, `INSERT INTO app.audit_events(action,subject_type,subject_id,data) VALUES('development_seed.jobs_v1','development_fixture','jobs-v1','{}')`); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
