package dal

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"github.com/Ravis-Study-Program/RSP-website/backend/internal/jobs"
)

const referralGraduate = `u.account_state='active' AND u.deleted_at IS NULL AND EXISTS (
 SELECT 1 FROM app.enrollments e WHERE e.user_id=u.id AND e.deleted_at IS NULL
 AND e.role='student' AND e.state='completed')`
const referralStudent = `u.account_state='active' AND u.deleted_at IS NULL AND EXISTS (
 SELECT 1 FROM app.enrollments e WHERE e.user_id=u.id AND e.deleted_at IS NULL
 AND e.role='student' AND e.state IN ('active','completed'))`

func (p *Store) GetJobReferralBoard(ctx context.Context, userID string) (jobs.ReferralBoard, error) {
	v := jobs.ReferralBoard{Requests: []string{}, Companies: []jobs.Company{}}
	err := p.pool.QueryRow(ctx, `SELECT reason,resume_url FROM app.job_referral_profiles WHERE user_id=$1`, userID).Scan(&v.Profile.Reason, &v.Profile.ResumeURL)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return v, err
	}
	rows, err := p.pool.Query(ctx, `SELECT r.position_id FROM app.job_referral_requests r JOIN app.job_positions p ON p.id=r.position_id
 WHERE r.user_id=$1 AND p.status='open' AND p.merged_into_id IS NULL ORDER BY r.position_id`, userID)
	if err != nil {
		return v, err
	}
	v.Requests, err = pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return v, err
	}
	rows, err = p.pool.Query(ctx, `SELECT c.id,c.name,c.category FROM app.job_referrals f JOIN app.job_companies c ON c.id=f.company_id
 JOIN app.users u ON u.id=f.user_id WHERE f.user_id=$1 AND f.matching_enabled AND `+referralGraduate+` ORDER BY lower(c.name),c.id`, userID)
	if err != nil {
		return v, err
	}
	v.Companies, err = pgx.CollectRows(rows, func(row pgx.CollectableRow) (jobs.Company, error) {
		var company jobs.Company
		err := row.Scan(&company.ID, &company.Name, &company.Category)
		return company, err
	})
	return v, err
}

func (p *Store) SaveJobReferralProfile(ctx context.Context, userID string, profile jobs.ReferralProfile) error {
	tx, err := p.beginJobs(ctx, userID)
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())
	if _, err := tx.Exec(ctx, `INSERT INTO app.job_referral_profiles(user_id,reason,resume_url) VALUES($1,$2,$3)
 ON CONFLICT (user_id) DO UPDATE SET reason=excluded.reason,resume_url=excluded.resume_url`, userID, profile.Reason, profile.ResumeURL); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (p *Store) DeleteJobReferralProfile(ctx context.Context, userID string) error {
	tx, err := p.beginJobs(ctx, userID)
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())
	if _, err := tx.Exec(ctx, `DELETE FROM app.job_referral_profiles WHERE user_id=$1`, userID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (p *Store) SetJobReferralRequest(ctx context.Context, userID, positionID string, wanted bool) error {
	tx, err := p.beginJobs(ctx, userID)
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())
	if !wanted {
		if _, err := tx.Exec(ctx, `DELETE FROM app.job_referral_requests WHERE user_id=$1 AND position_id=$2`, userID, positionID); err != nil {
			return err
		}
		return tx.Commit(ctx)
	}

	var open bool
	if err := tx.QueryRow(ctx, `SELECT status='open' AND merged_into_id IS NULL FROM app.job_positions WHERE id=$1`, positionID).Scan(&open); err != nil {
		return noRows(err)
	}
	if !open {
		return jobs.ErrReferralPositionClosed
	}
	var hasProfile, alreadyRequested bool
	var requests int
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM app.job_referral_profiles WHERE user_id=$1),
 count(*),COALESCE(bool_or(position_id=$2),false) FROM app.job_referral_requests WHERE user_id=$1`, userID, positionID).Scan(&hasProfile, &requests, &alreadyRequested); err != nil {
		return err
	}
	if !hasProfile {
		return jobs.ErrReferralProfileRequired
	}
	if alreadyRequested {
		return tx.Commit(ctx)
	}
	if requests >= jobs.MaxReferralRequests {
		return jobs.ErrReferralLimit
	}
	if _, err := tx.Exec(ctx, `INSERT INTO app.job_referral_requests(user_id,position_id) VALUES($1,$2)`, userID, positionID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (p *Store) SetJobReferralOffer(ctx context.Context, userID, companyID string, enabled bool) error {
	tx, err := p.beginJobs(ctx, userID)
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())
	if enabled {
		tag, err := tx.Exec(ctx, `INSERT INTO app.job_referrals(user_id,company_id,matching_enabled)
 SELECT $1,id,true FROM app.job_companies WHERE id=$2
 ON CONFLICT (user_id,company_id) DO UPDATE SET matching_enabled=true,updated_at=now()`, userID, companyID)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return ErrNotFound
		}
	} else if _, err := tx.Exec(ctx, `UPDATE app.job_referrals SET matching_enabled=false,updated_at=now()
 WHERE user_id=$1 AND company_id=$2 AND matching_enabled`, userID, companyID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (p *Store) ListJobReferralCounts(ctx context.Context) ([]jobs.ReferralCount, error) {
	rows, err := p.pool.Query(ctx, `SELECT f.company_id,count(*) FROM app.job_referrals f JOIN app.users u ON u.id=f.user_id
 WHERE f.matching_enabled AND `+referralGraduate+` GROUP BY f.company_id`)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (jobs.ReferralCount, error) {
		var v jobs.ReferralCount
		err := row.Scan(&v.CompanyID, &v.Count)
		return v, err
	})
}

// Match on the company of the requested position. Opting out or withdrawing the
// last request at a company removes the associated profile from the queue.
const referralMatch = `FROM app.job_referral_requests r JOIN app.job_positions p ON p.id=r.position_id
 JOIN app.users u ON u.id=r.user_id JOIN app.job_referral_profiles profile ON profile.user_id=r.user_id
 WHERE p.company_id=$2 AND p.status='open' AND p.merged_into_id IS NULL AND r.user_id<>$1 AND ` + referralStudent + `
 AND EXISTS (SELECT 1 FROM app.job_referrals f JOIN app.users u ON u.id=f.user_id
 WHERE f.user_id=$1 AND f.company_id=$2 AND f.matching_enabled AND ` + referralGraduate + `)`

func (p *Store) ListJobReferralCandidates(ctx context.Context, userID, companyID, boundary string, limit int) ([]jobs.ReferralCandidate, bool, int64, error) {
	var total int64
	if err := p.pool.QueryRow(ctx, `SELECT count(DISTINCT r.user_id) `+referralMatch, userID, companyID).Scan(&total); err != nil {
		return nil, false, 0, err
	}
	rows, err := p.pool.Query(ctx, `SELECT r.user_id,up.display_name,profile.reason,profile.resume_url,
 jsonb_agg(jsonb_build_object('id',p.id,'title',p.title) ORDER BY p.title,p.id)
 FROM app.user_profiles up JOIN (SELECT DISTINCT r.user_id `+referralMatch+`
 AND ($3='' OR r.user_id>NULLIF($3,'')::uuid) ORDER BY r.user_id LIMIT $4) page ON page.user_id=up.user_id
 JOIN app.job_referral_requests r ON r.user_id=page.user_id
 JOIN app.job_positions p ON p.id=r.position_id
 JOIN app.job_referral_profiles profile ON profile.user_id=r.user_id
 WHERE p.company_id=$2 AND p.status='open' AND p.merged_into_id IS NULL
 GROUP BY r.user_id,up.display_name,profile.reason,profile.resume_url ORDER BY r.user_id`, userID, companyID, boundary, limit+1)
	if err != nil {
		return nil, false, 0, err
	}
	out, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (jobs.ReferralCandidate, error) {
		var v jobs.ReferralCandidate
		err := row.Scan(&v.ID, &v.Name, &v.Reason, &v.ResumeURL, &v.Positions)
		return v, err
	})
	out, more := finishPage(out, limit, "forward")
	return out, more, total, err
}
