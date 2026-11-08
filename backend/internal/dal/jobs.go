package dal

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/magedmg/RSP-website/backend/internal/jobs"
)

const jobPositionColumns = `p.id,c.id,c.name,c.category,p.title,p.job_type,p.eligibility,p.application_url,p.locations,p.notes,p.status`
const jobPositionJoin = `app.job_positions p JOIN app.job_companies c ON c.id=p.company_id`

func positionTargets(v *jobs.Position) []any {
	return []any{&v.ID, &v.Company.ID, &v.Company.Name, &v.Company.Category, &v.Title, &v.JobType, &v.Eligibility, &v.ApplicationURL, &v.Locations, &v.Notes, &v.Status}
}

// The small beta serializes Jobs writes only. This makes deduplication,
// merges and application/stage mutations atomic even with simultaneous tabs.
// Readers and all other product writes remain independent.
func (p *Store) beginJobs(ctx context.Context, actorID string) (pgx.Tx, error) {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended('rsp.jobs',0))`); err == nil {
		var userID string
		err = tx.QueryRow(ctx, `SELECT id FROM app.users WHERE id=$1 AND account_state='active' FOR SHARE`, actorID).Scan(&userID)
	}
	if err != nil {
		_ = tx.Rollback(context.Background())
		return nil, noRows(err)
	}
	return tx, nil
}

func (p *Store) ListJobCompanies(ctx context.Context) ([]jobs.Company, error) {
	rows, err := p.pool.Query(ctx, `SELECT id,name,category FROM app.job_companies ORDER BY lower(name),id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []jobs.Company{}
	for rows.Next() {
		var v jobs.Company
		if err := rows.Scan(&v.ID, &v.Name, &v.Category); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func (p *Store) ListJobPositions(ctx context.Context, boundary string, limit int) ([]jobs.Position, bool, int64, error) {
	var total int64
	if err := p.pool.QueryRow(ctx, `SELECT count(*) FROM app.job_positions WHERE merged_into_id IS NULL`).Scan(&total); err != nil {
		return nil, false, 0, err
	}
	rows, err := p.pool.Query(ctx, `SELECT `+jobPositionColumns+` FROM `+jobPositionJoin+`
 WHERE p.merged_into_id IS NULL AND ($1='' OR p.id>NULLIF($1,'')::uuid) ORDER BY p.id LIMIT $2`, boundary, limit+1)
	if err != nil {
		return nil, false, 0, err
	}
	defer rows.Close()
	out := []jobs.Position{}
	for rows.Next() {
		var v jobs.Position
		if err := rows.Scan(positionTargets(&v)...); err != nil {
			return nil, false, 0, err
		}
		out = append(out, v)
	}
	out, more := finishPage(out, limit, "forward")
	return out, more, total, rows.Err()
}

func jobCanonicalID(ctx context.Context, q queryer, positionID string) (string, error) {
	var canonical string
	err := q.QueryRow(ctx, `WITH RECURSIVE chain AS (
 SELECT id,merged_into_id FROM app.job_positions WHERE id=$1
 UNION ALL SELECT p.id,p.merged_into_id FROM app.job_positions p JOIN chain c ON p.id=c.merged_into_id
 ) SELECT id FROM chain WHERE merged_into_id IS NULL`, positionID).Scan(&canonical)
	return canonical, noRows(err)
}

func (p *Store) GetJobPosition(ctx context.Context, positionID string) (jobs.Position, error) {
	canonical, err := jobCanonicalID(ctx, p.pool, positionID)
	if err != nil {
		return jobs.Position{}, err
	}
	var v jobs.Position
	err = p.pool.QueryRow(ctx, `SELECT `+jobPositionColumns+` FROM `+jobPositionJoin+` WHERE p.id=$1`, canonical).Scan(positionTargets(&v)...)
	return v, noRows(err)
}

func jobCompanyID(ctx context.Context, tx pgx.Tx, input jobs.PositionInput) (string, error) {
	var companyID string
	err := tx.QueryRow(ctx, `INSERT INTO app.job_companies(name,category) VALUES($1,$2)
 ON CONFLICT (lower(trim(name))) DO UPDATE SET name=app.job_companies.name RETURNING id`, input.CompanyName, input.Category).Scan(&companyID)
	return companyID, mapDatabaseError(err)
}

func (p *Store) SaveJobPosition(ctx context.Context, positionID, actorID, normalizedURL string, input jobs.PositionInput) (jobs.Position, error) {
	tx, err := p.beginJobs(ctx, actorID)
	if err != nil {
		return jobs.Position{}, err
	}
	defer tx.Rollback(context.Background())
	creating := positionID == ""
	if !creating {
		var merged *string
		if err := tx.QueryRow(ctx, `SELECT merged_into_id FROM app.job_positions WHERE id=$1`, positionID).Scan(&merged); err != nil {
			return jobs.Position{}, noRows(err)
		}
		if merged != nil {
			return jobs.Position{}, ErrConflict
		}
	}
	companyID, err := jobCompanyID(ctx, tx, input)
	if err != nil {
		return jobs.Position{}, err
	}
	var duplicate bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM app.job_positions WHERE normalized_url=$1 AND id<>COALESCE(NULLIF($2,'')::uuid,'00000000-0000-0000-0000-000000000000'::uuid))`, normalizedURL, positionID).Scan(&duplicate); err != nil {
		return jobs.Position{}, err
	}
	if duplicate {
		return jobs.Position{}, ErrDuplicate
	}
	args := []any{companyID, input.Title, input.JobType, input.Eligibility, input.ApplicationURL, normalizedURL, input.Locations, input.Notes, input.Status}
	if creating {
		err = tx.QueryRow(ctx, `INSERT INTO app.job_positions(company_id,title,job_type,eligibility,application_url,normalized_url,locations,notes,status,created_by_user_id)
  VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10) RETURNING id`, append(args, actorID)...).Scan(&positionID)
	} else {
		_, err = tx.Exec(ctx, `UPDATE app.job_positions SET company_id=$1,title=$2,job_type=$3,eligibility=$4,application_url=$5,
  normalized_url=$6,locations=$7,notes=$8,status=$9,updated_at=now() WHERE id=$10`, append(args, positionID)...)
	}
	if err != nil {
		return jobs.Position{}, mapDatabaseError(err)
	}
	action := "job_position.updated"
	if creating {
		action = "job_position.created"
	}
	if err = appendAuditTx(ctx, tx, newAudit(actorID, action, "job_position", positionID, nil, time.Now().UTC())); err != nil {
		return jobs.Position{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return jobs.Position{}, err
	}
	return p.GetJobPosition(ctx, positionID)
}

func (p *Store) UpdateJobCompany(ctx context.Context, actorID string, v jobs.Company) (jobs.Company, error) {
	tx, err := p.beginJobs(ctx, actorID)
	if err != nil {
		return v, err
	}
	defer tx.Rollback(context.Background())
	err = tx.QueryRow(ctx, `UPDATE app.job_companies SET name=$1,category=$2 WHERE id=$3 RETURNING id`, v.Name, v.Category, v.ID).Scan(&v.ID)
	if err != nil {
		return v, mapDatabaseError(noRows(err))
	}
	if err = appendAuditTx(ctx, tx, newAudit(actorID, "job_company.updated", "job_company", v.ID, nil, time.Now().UTC())); err != nil {
		return v, err
	}
	return v, tx.Commit(ctx)
}

func (p *Store) ListJobApplications(ctx context.Context, userID, boundary string, limit int) ([]jobs.Application, bool, int64, error) {
	var total int64
	if err := p.pool.QueryRow(ctx, `SELECT count(*) FROM app.job_applications WHERE user_id=$1`, userID).Scan(&total); err != nil {
		return nil, false, 0, err
	}
	rows, err := p.pool.Query(ctx, `SELECT a.id,a.status,`+jobPositionColumns+` FROM app.job_applications a JOIN app.job_positions p ON a.position_id=p.id JOIN app.job_companies c ON c.id=p.company_id
 WHERE a.user_id=$1 AND ($2='' OR a.id>NULLIF($2,'')::uuid) ORDER BY a.id LIMIT $3`, userID, boundary, limit+1)
	if err != nil {
		return nil, false, 0, err
	}
	out := []jobs.Application{}
	for rows.Next() {
		v := jobs.Application{Stages: []jobs.Stage{}}
		if err = rows.Scan(append([]any{&v.ID, &v.Status}, positionTargets(&v.Position)...)...); err != nil {
			rows.Close()
			return nil, false, 0, err
		}
		out = append(out, v)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, false, 0, err
	}
	out, more := finishPage(out, limit, "forward")
	indices := map[string]int{}
	ids := []string{}
	for i, v := range out {
		indices[v.ID] = i
		ids = append(ids, v.ID)
	}
	if len(ids) == 0 {
		return out, more, total, nil
	}
	rows, err = p.pool.Query(ctx, `SELECT s.application_id,s.id,s.title,s.happened_on::text,s.notes,s.initial
 FROM app.job_application_stages s JOIN app.job_applications a ON a.id=s.application_id
 WHERE a.user_id=$1 AND a.id=ANY($2::uuid[]) ORDER BY s.initial DESC,s.happened_on,s.created_at,s.id`, userID, ids)
	if err != nil {
		return nil, false, 0, err
	}
	defer rows.Close()
	for rows.Next() {
		var appID string
		var stage jobs.Stage
		if err := rows.Scan(&appID, &stage.ID, &stage.Title, &stage.Date, &stage.Notes, &stage.Initial); err != nil {
			return nil, false, 0, err
		}
		i := indices[appID]
		out[i].Stages = append(out[i].Stages, stage)
	}
	return out, more, total, rows.Err()
}

func (p *Store) ApplyToJob(ctx context.Context, userID, positionID, date string) (string, error) {
	tx, err := p.beginJobs(ctx, userID)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(context.Background())
	canonical, err := jobCanonicalID(ctx, tx, positionID)
	if err != nil {
		return "", err
	}
	var appID string
	err = tx.QueryRow(ctx, `INSERT INTO app.job_applications(user_id,position_id) VALUES($1,$2)
 ON CONFLICT(user_id,position_id) DO UPDATE SET position_id=EXCLUDED.position_id RETURNING id`, userID, canonical).Scan(&appID)
	if err != nil {
		return "", err
	}
	_, err = tx.Exec(ctx, `INSERT INTO app.job_application_stages(application_id,title,happened_on,initial)
 VALUES($1,'Applied',$2,true) ON CONFLICT(application_id) WHERE initial DO NOTHING`, appID, date)
	if err != nil {
		return "", err
	}
	return appID, tx.Commit(ctx)
}

func ownedJobApplication(ctx context.Context, tx pgx.Tx, userID, applicationID string) error {
	var found string
	return noRows(tx.QueryRow(ctx, `SELECT id FROM app.job_applications WHERE id=$1 AND user_id=$2`, applicationID, userID).Scan(&found))
}

func (p *Store) UpdateJobApplicationStatus(ctx context.Context, userID, applicationID, status, date string) error {
	tx, err := p.beginJobs(ctx, userID)
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())
	var current string
	if err = tx.QueryRow(ctx, `SELECT status FROM app.job_applications WHERE id=$1 AND user_id=$2 FOR UPDATE`, applicationID, userID).Scan(&current); err != nil {
		return mapDatabaseError(noRows(err))
	}
	if current == status {
		return tx.Commit(ctx)
	}
	if _, err = tx.Exec(ctx, `UPDATE app.job_applications SET status=$1 WHERE id=$2 AND user_id=$3`, status, applicationID, userID); err != nil {
		return err
	}
	title := map[string]string{"active": "Active", "inactive": "Rejected", "offer": "Offer"}[status]
	if _, err = tx.Exec(ctx, `INSERT INTO app.job_application_stages(application_id,title,happened_on) VALUES($1,$2,$3)`, applicationID, title, date); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (p *Store) DeleteJobApplication(ctx context.Context, userID, applicationID string) error {
	tx, err := p.beginJobs(ctx, userID)
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())
	if err = ownedJobApplication(ctx, tx, userID, applicationID); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `DELETE FROM app.job_applications WHERE id=$1 AND user_id=$2`, applicationID, userID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (p *Store) SaveJobStage(ctx context.Context, userID, applicationID, stageID string, input jobs.StageInput) (jobs.Stage, error) {
	v := jobs.Stage{ID: stageID, Title: input.Title, Date: input.Date, Notes: input.Notes}
	tx, err := p.beginJobs(ctx, userID)
	if err != nil {
		return v, err
	}
	defer tx.Rollback(context.Background())
	if err = ownedJobApplication(ctx, tx, userID, applicationID); err != nil {
		return v, err
	}
	if stageID == "" {
		err = tx.QueryRow(ctx, `INSERT INTO app.job_application_stages(application_id,title,happened_on,notes) VALUES($1,$2,$3,$4) RETURNING id`, applicationID, input.Title, input.Date, input.Notes).Scan(&v.ID)
	} else {
		if err = tx.QueryRow(ctx, `SELECT initial FROM app.job_application_stages WHERE id=$1 AND application_id=$2`, stageID, applicationID).Scan(&v.Initial); err != nil {
			return v, noRows(err)
		}
		if v.Initial && input.Title != "Applied" {
			return v, ErrConflict
		}
		_, err = tx.Exec(ctx, `UPDATE app.job_application_stages SET title=$1,happened_on=$2,notes=$3 WHERE id=$4 AND application_id=$5`, input.Title, input.Date, input.Notes, stageID, applicationID)
	}
	if err != nil {
		return v, mapDatabaseError(err)
	}
	return v, tx.Commit(ctx)
}

func (p *Store) DeleteJobStage(ctx context.Context, userID, applicationID, stageID string) error {
	tx, err := p.beginJobs(ctx, userID)
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())
	if err = ownedJobApplication(ctx, tx, userID, applicationID); err != nil {
		return err
	}
	var initial bool
	if err = tx.QueryRow(ctx, `SELECT initial FROM app.job_application_stages WHERE id=$1 AND application_id=$2`, stageID, applicationID).Scan(&initial); err != nil {
		return noRows(err)
	}
	if initial {
		return ErrConflict
	}
	if _, err = tx.Exec(ctx, `DELETE FROM app.job_application_stages WHERE id=$1`, stageID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// MergeJobPositions never returns or audits private applications. If someone
// tracked both listings, retain every stage and keep the source Applied notes
// as a separate stage. Old position IDs continue to resolve.
func (p *Store) MergeJobPositions(ctx context.Context, actorID, sourceID, targetID string) (jobs.Position, error) {
	tx, err := p.beginJobs(ctx, actorID)
	if err != nil {
		return jobs.Position{}, err
	}
	defer tx.Rollback(context.Background())
	target, err := jobCanonicalID(ctx, tx, targetID)
	if err != nil {
		return jobs.Position{}, err
	}
	source, err := jobCanonicalID(ctx, tx, sourceID)
	if err != nil {
		return jobs.Position{}, err
	}
	if source == target {
		if err = tx.Commit(ctx); err != nil {
			return jobs.Position{}, err
		}
		return p.GetJobPosition(ctx, target)
	}
	// A stale source that has already been merged elsewhere must not merge
	// its new canonical listing unexpectedly.
	if source != sourceID {
		return jobs.Position{}, ErrConflict
	}
	var sourceLocations, targetLocations []string
	if err = tx.QueryRow(ctx, `SELECT locations FROM app.job_positions WHERE id=$1`, source).Scan(&sourceLocations); err != nil {
		return jobs.Position{}, err
	}
	if err = tx.QueryRow(ctx, `SELECT locations FROM app.job_positions WHERE id=$1`, target).Scan(&targetLocations); err != nil {
		return jobs.Position{}, err
	}
	locations := jobs.UniqueLocations(append(targetLocations, sourceLocations...))
	if len(locations) > 20 {
		return jobs.Position{}, ErrConflict
	}
	rows, err := tx.Query(ctx, `SELECT a.id,other.id FROM app.job_applications a LEFT JOIN app.job_applications other
 ON other.user_id=a.user_id AND other.position_id=$2 WHERE a.position_id=$1`, source, target)
	if err != nil {
		return jobs.Position{}, err
	}
	type pair struct {
		source string
		target *string
	}
	pairs := []pair{}
	for rows.Next() {
		var v pair
		if err = rows.Scan(&v.source, &v.target); err != nil {
			rows.Close()
			return jobs.Position{}, err
		}
		pairs = append(pairs, v)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return jobs.Position{}, err
	}
	for _, v := range pairs {
		if v.target == nil {
			_, err = tx.Exec(ctx, `UPDATE app.job_applications SET position_id=$1 WHERE id=$2`, target, v.source)
		} else {
			// Keep both notes without the normal input length cap: accumulated
			// private notes are stored as another historical stage when necessary.
			var sourceInitial jobs.Stage
			err = tx.QueryRow(ctx, `SELECT id,title,happened_on::text,notes,initial FROM app.job_application_stages WHERE application_id=$1 AND initial`, v.source).Scan(&sourceInitial.ID, &sourceInitial.Title, &sourceInitial.Date, &sourceInitial.Notes, &sourceInitial.Initial)
			if err != nil {
				return jobs.Position{}, err
			}
			_, err = tx.Exec(ctx, `UPDATE app.job_application_stages SET happened_on=LEAST(happened_on,$2::date) WHERE application_id=$1 AND initial`, *v.target, sourceInitial.Date)
			if err != nil {
				return jobs.Position{}, err
			}
			if strings.TrimSpace(sourceInitial.Notes) != "" {
				_, err = tx.Exec(ctx, `UPDATE app.job_application_stages SET initial=false,title='Applied (merged listing)' WHERE id=$1`, sourceInitial.ID)
			} else {
				_, err = tx.Exec(ctx, `DELETE FROM app.job_application_stages WHERE id=$1`, sourceInitial.ID)
			}
			if err != nil {
				return jobs.Position{}, err
			}
			_, err = tx.Exec(ctx, `UPDATE app.job_application_stages SET application_id=$1 WHERE application_id=$2`, *v.target, v.source)
			if err != nil {
				return jobs.Position{}, err
			}
			_, err = tx.Exec(ctx, `DELETE FROM app.job_applications WHERE id=$1`, v.source)
		}
		if err != nil {
			return jobs.Position{}, err
		}
	}
	if _, err = tx.Exec(ctx, `UPDATE app.job_positions SET locations=$1,updated_at=now() WHERE id=$2`, locations, target); err != nil {
		return jobs.Position{}, err
	}
	if _, err = tx.Exec(ctx, `UPDATE app.job_positions SET merged_into_id=$1,updated_at=now() WHERE id=$2`, target, source); err != nil {
		return jobs.Position{}, err
	}
	if err = appendAuditTx(ctx, tx, newAudit(actorID, "job_position.merged", "job_position", source, map[string]any{"targetId": target}, time.Now().UTC())); err != nil {
		return jobs.Position{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return jobs.Position{}, err
	}
	return p.GetJobPosition(ctx, target)
}

// JobPositionForURL also follows merged aliases, so old external links cannot
// accidentally recreate a listing which staff already deduplicated.
func (p *Store) JobPositionForURL(ctx context.Context, normalizedURL string) (string, error) {
	var positionID string
	err := p.pool.QueryRow(ctx, `SELECT id FROM app.job_positions WHERE normalized_url=$1 ORDER BY merged_into_id NULLS FIRST,id LIMIT 1`, normalizedURL).Scan(&positionID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return jobCanonicalID(ctx, p.pool, positionID)
}
