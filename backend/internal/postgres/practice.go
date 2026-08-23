package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/magedmg/RSP-website/backend/internal/platform/audit"
	"github.com/magedmg/RSP-website/backend/internal/platform/id"
	"github.com/magedmg/RSP-website/backend/internal/practice"
)

func (p *Postgres) GetPracticeSettings(ctx context.Context, userID string) (practice.PracticeSettings, error) {
	var settings practice.PracticeSettings
	err := p.Pool.QueryRow(ctx, `
SELECT u.leetcode_premium_opt_in,
       COALESCE(g.enabled,false), COALESCE(g.easy_minutes,20),
       COALESCE(g.medium_minutes,35), COALESCE(g.hard_minutes,50),
       COALESCE(g.revision,1)
FROM app.users u
LEFT JOIN app.practice_goals g ON g.user_id=u.id
WHERE u.id=$1 AND u.deleted_at IS NULL`, userID).Scan(
		&settings.PremiumOptIn, &settings.GoalsEnabled, &settings.EasyMinutes,
		&settings.MediumMinutes, &settings.HardMinutes, &settings.Revision,
	)
	return settings, noRows(err)
}

// UpdatePracticeSettings updates a value.
func (p *Postgres) UpdatePracticeSettings(ctx context.Context, userID string, revision int64, premium bool, easy, medium, hard int, actorID string, at time.Time) (practice.PracticeSettings, error) {
	tx, err := p.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return practice.PracticeSettings{}, err
	}

	defer tx.Rollback(ctx)
	var currentRevision int64
	var enabled bool
	err = tx.QueryRow(ctx, `
INSERT INTO app.practice_goals(user_id,enabled,easy_minutes,medium_minutes,hard_minutes,revision)
VALUES($1,false,20,35,50,1)
ON CONFLICT(user_id) DO UPDATE SET user_id=EXCLUDED.user_id
RETURNING enabled,revision`, userID).Scan(&enabled, &currentRevision)
	if err != nil {
		return practice.PracticeSettings{}, err
	}
	if currentRevision != revision {
		return practice.PracticeSettings{}, ErrConflict
	}
	if easy < 1 || medium < 1 || hard < 1 {
		return practice.PracticeSettings{}, errors.New("practice goals must be positive")
	}
	if _, err = tx.Exec(ctx, `UPDATE app.users SET leetcode_premium_opt_in=$2,revision=revision+1 WHERE id=$1 AND deleted_at IS NULL`, userID, premium); err != nil {
		return practice.PracticeSettings{}, err
	}

	var settings practice.PracticeSettings
	settings.PremiumOptIn = premium
	err = tx.QueryRow(ctx, `
UPDATE app.practice_goals
SET easy_minutes=$2,medium_minutes=$3,hard_minutes=$4,revision=revision+1
WHERE user_id=$1 AND revision=$5
RETURNING enabled,easy_minutes,medium_minutes,hard_minutes,revision`, userID, easy, medium, hard, revision).Scan(
		&settings.GoalsEnabled, &settings.EasyMinutes, &settings.MediumMinutes, &settings.HardMinutes, &settings.Revision,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return practice.PracticeSettings{}, ErrConflict
	}
	if err != nil {
		return practice.PracticeSettings{}, err
	}

	data, _ := json.Marshal(map[string]any{"premiumOptIn": premium, "easyMinutes": easy, "mediumMinutes": medium, "hardMinutes": hard})
	if _, err = tx.Exec(ctx, `INSERT INTO app.audit_events(id,actor_user_id,action,subject_type,subject_id,data,occurred_at) VALUES($1,$2,'practice_settings.updated','user',$3,$4,$5)`, id.New(), actorID, userID, data, at.UTC()); err != nil {
		return practice.PracticeSettings{}, err
	}
	return settings, tx.Commit(ctx)
}

// EnablePracticeGoals performs the operation.
func (p *Postgres) EnablePracticeGoals(ctx context.Context, userID string, revision int64, actorID, seasonID string, at time.Time) (practice.PracticeSettings, error) {
	tx, err := p.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return practice.PracticeSettings{}, err
	}

	defer tx.Rollback(ctx)
	var settings practice.PracticeSettings
	changed := true
	err = tx.QueryRow(ctx, `
INSERT INTO app.practice_goals(user_id,enabled,enabled_by_user_id,enabled_at,easy_minutes,medium_minutes,hard_minutes,revision)
SELECT $1,true,$2,$3,20,35,50,2 WHERE $4=1
ON CONFLICT(user_id) DO UPDATE
SET enabled=true, enabled_by_user_id=EXCLUDED.enabled_by_user_id, enabled_at=EXCLUDED.enabled_at, revision=app.practice_goals.revision+1
WHERE NOT app.practice_goals.enabled AND app.practice_goals.revision=$4
RETURNING enabled,easy_minutes,medium_minutes,hard_minutes,revision`, userID, actorID, at.UTC(), revision).Scan(
		&settings.GoalsEnabled, &settings.EasyMinutes, &settings.MediumMinutes, &settings.HardMinutes, &settings.Revision,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		changed = false
		err = tx.QueryRow(ctx, `SELECT enabled,easy_minutes,medium_minutes,hard_minutes,revision FROM app.practice_goals WHERE user_id=$1`, userID).Scan(&settings.GoalsEnabled, &settings.EasyMinutes, &settings.MediumMinutes, &settings.HardMinutes, &settings.Revision)
		if errors.Is(err, pgx.ErrNoRows) {
			return practice.PracticeSettings{}, ErrConflict
		}
		if err == nil && settings.Revision != revision {
			return practice.PracticeSettings{}, ErrConflict
		}
	}
	if err != nil {
		return practice.PracticeSettings{}, noRows(err)
	}
	if err = tx.QueryRow(ctx, `SELECT leetcode_premium_opt_in FROM app.users WHERE id=$1`, userID).Scan(&settings.PremiumOptIn); err != nil {
		return practice.PracticeSettings{}, noRows(err)
	}
	if changed {
		if _, err = tx.Exec(ctx, `INSERT INTO app.audit_events(id,actor_user_id,action,subject_type,subject_id,data,occurred_at) VALUES($1,$2,'practice_goals.enabled','user',$3,jsonb_build_object('seasonId',$4::text),$5)`, id.New(), actorID, userID, seasonID, at.UTC()); err != nil {
			return practice.PracticeSettings{}, err
		}
	}
	return settings, tx.Commit(ctx)
}

func scanProblem(row pgx.Row) (practice.ProblemRecord, error) {
	var v practice.ProblemRecord
	if err := row.Scan(&v.ID, &v.Number, &v.Title, &v.Link, &v.Difficulty, &v.Premium, &v.Categories, &v.Revision); err != nil {
		return v, noRows(err)
	}
	return v, nil
}

const problemQuery = `SELECT l.id,l.leetcode_number,p.title,COALESCE(p.url,''),l.difficulty::text,l.is_premium,COALESCE(array_agg(c.normalized_name) FILTER (WHERE c.id IS NOT NULL),'{}')::text[],l.revision FROM app.leetcode_problems l JOIN app.problems p ON p.id=l.problem_id LEFT JOIN app.leetcode_problem_category_mappings m ON m.leetcode_problem_id=l.id LEFT JOIN app.leetcode_problem_categories c ON c.id=m.category_id WHERE l.deleted_at IS NULL AND p.deleted_at IS NULL`

// ListProblems lists matching values.
func (p *Postgres) ListProblems(ctx context.Context, boundary string, limit int, difficulty, category string, premium *bool, direction string) ([]practice.ProblemRecord, bool, int64, error) {
	var total int64
	if err := p.Pool.QueryRow(ctx, `SELECT count(*) FROM app.leetcode_problems l WHERE l.deleted_at IS NULL AND ($1='' OR l.difficulty::text=$1) AND ($2='' OR EXISTS(SELECT 1 FROM app.leetcode_problem_category_mappings m JOIN app.leetcode_problem_categories c ON c.id=m.category_id WHERE m.leetcode_problem_id=l.id AND c.deleted_at IS NULL AND lower(c.normalized_name)=lower($2))) AND ($3::boolean IS NULL OR l.is_premium=$3)`, difficulty, category, premium).Scan(&total); err != nil {
		return nil, false, 0, err
	}

	comparison, order := `l.id>NULLIF($1,'')::uuid`, `ASC`
	if direction == "backward" {
		comparison, order = `l.id<NULLIF($1,'')::uuid`, `DESC`
	}
	if boundary == "" {
		comparison = `$1::text IS NOT NULL`
	}
	rows, err := p.Pool.Query(ctx, problemQuery+` AND `+comparison+` AND ($3='' OR l.difficulty::text=$3) AND ($4='' OR EXISTS(SELECT 1 FROM app.leetcode_problem_category_mappings fm JOIN app.leetcode_problem_categories fc ON fc.id=fm.category_id WHERE fm.leetcode_problem_id=l.id AND fc.deleted_at IS NULL AND lower(fc.normalized_name)=lower($4))) AND ($5::boolean IS NULL OR l.is_premium=$5) GROUP BY l.id,p.id ORDER BY l.id `+order+` LIMIT $2`, boundary, limit+1, difficulty, category, premium)
	if err != nil {
		return nil, false, 0, err
	}

	defer rows.Close()
	out := []practice.ProblemRecord{}
	for rows.Next() {
		v, err := scanProblem(rows)
		if err != nil {
			return nil, false, 0, err
		}

		out = append(out, v)
	}
	more := len(out) > limit
	if more {
		out = out[:limit]
	}
	if direction == "backward" {
		sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	}
	return out, more, total, rows.Err()
}

func scanAttempt(row pgx.Row) (practice.AttemptRecord, error) {
	var v practice.AttemptRecord
	if err := row.Scan(&v.ID, &v.UserID, &v.ProblemID, &v.Outcome, &v.Confidence, &v.Minutes, &v.Notes, &v.AttemptedAt, &v.SeasonID, &v.WeekID, &v.Revision, &v.DeletedAt, &v.Migrated); err != nil {
		return v, noRows(err)
	}
	return v, nil
}

const attemptColumns = `a.id,a.user_id,COALESCE((SELECT id FROM app.leetcode_problems WHERE problem_id=a.problem_id),a.problem_id),a.outcome::text,a.confidence,a.time_taken_minutes,COALESCE(a.notes_html,''),a.attempted_at,e.season_id,a.season_week_id,a.revision,a.deleted_at,EXISTS(SELECT 1 FROM migration.row_provenance rp WHERE rp.target_table='problem_attempts' AND rp.target_id=a.id::text)`

// GetAttempt retrieves a value.
func (p *Postgres) GetAttempt(ctx context.Context, id string) (practice.AttemptRecord, error) {
	return scanAttempt(p.Pool.QueryRow(ctx, `SELECT `+attemptColumns+` FROM app.problem_attempts a LEFT JOIN app.enrollments e ON e.id=a.enrollment_id WHERE a.id=$1 AND a.deleted_at IS NULL`, id))
}

// ListAttempts lists matching values.
func (p *Postgres) ListAttempts(ctx context.Context, userID, boundary string, limit int, outcome, difficulty, direction string) ([]practice.AttemptRecord, bool, int64, error) {
	var total int64
	if err := p.Pool.QueryRow(ctx, `SELECT count(*) FROM app.problem_attempts a WHERE a.user_id=$1 AND a.deleted_at IS NULL AND ($2='' OR a.outcome::text=$2) AND ($3='' OR EXISTS(SELECT 1 FROM app.leetcode_problems l WHERE l.problem_id=a.problem_id AND l.difficulty::text=$3 AND l.deleted_at IS NULL))`, userID, outcome, difficulty).Scan(&total); err != nil {
		return nil, false, 0, err
	}

	comparison, order := `a.id>NULLIF($2,'')::uuid`, `ASC`
	if direction == "backward" {
		comparison, order = `a.id<NULLIF($2,'')::uuid`, `DESC`
	}
	if boundary == "" {
		comparison = `$2::text IS NOT NULL`
	}
	rows, err := p.Pool.Query(ctx, `SELECT `+attemptColumns+` FROM app.problem_attempts a LEFT JOIN app.enrollments e ON e.id=a.enrollment_id WHERE a.user_id=$1 AND `+comparison+` AND a.deleted_at IS NULL AND ($4='' OR a.outcome::text=$4) AND ($5='' OR EXISTS(SELECT 1 FROM app.leetcode_problems l WHERE l.problem_id=a.problem_id AND l.difficulty::text=$5 AND l.deleted_at IS NULL)) ORDER BY a.id `+order+` LIMIT $3`, userID, boundary, limit+1, outcome, difficulty)
	if err != nil {
		return nil, false, 0, err
	}

	defer rows.Close()
	out := []practice.AttemptRecord{}
	for rows.Next() {
		v, err := scanAttempt(rows)
		if err != nil {
			return nil, false, 0, err
		}

		out = append(out, v)
	}
	more := len(out) > limit
	if more {
		out = out[:limit]
	}
	if direction == "backward" {
		sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	}
	return out, more, total, rows.Err()
}

// RecommendationSnapshot performs the operation.
func (p *Postgres) RecommendationSnapshot(ctx context.Context, userID string, _ practice.Goals) (practice.RecommendationSnapshot, error) {
	snapshot := practice.RecommendationSnapshot{ProblemHistory: map[string]practice.ProblemHistory{}, CategoryExposure: map[string]int{}}
	qualityRows, err := p.Pool.Query(ctx, `SELECT `+attemptColumns+` FROM app.problem_attempts a LEFT JOIN app.enrollments e ON e.id=a.enrollment_id WHERE a.user_id=$1 AND a.deleted_at IS NULL AND a.outcome<>'unknown' AND NOT EXISTS(SELECT 1 FROM migration.row_provenance rp WHERE rp.target_table='problem_attempts' AND rp.target_id=a.id::text) ORDER BY a.attempted_at DESC,a.id ASC LIMIT 20`, userID)
	if err != nil {
		return snapshot, err
	}

	for qualityRows.Next() {
		attempt, err := scanAttempt(qualityRows)
		if err != nil {
			qualityRows.Close()
			return snapshot, err
		}

		snapshot.QualityAttempts = append(snapshot.QualityAttempts, attempt)
	}
	if err := qualityRows.Err(); err != nil {
		qualityRows.Close()
		return snapshot, err
	}

	qualityRows.Close()

	exposureRows, err := p.Pool.Query(ctx, `SELECT c.normalized_name,count(*) FROM app.problem_attempts a JOIN app.leetcode_problems l ON l.problem_id=a.problem_id AND l.deleted_at IS NULL JOIN app.leetcode_problem_category_mappings m ON m.leetcode_problem_id=l.id JOIN app.leetcode_problem_categories c ON c.id=m.category_id AND c.deleted_at IS NULL WHERE a.user_id=$1 AND a.deleted_at IS NULL GROUP BY c.normalized_name`, userID)
	if err != nil {
		return snapshot, err
	}

	for exposureRows.Next() {
		var category string
		var count int
		if err := exposureRows.Scan(&category, &count); err != nil {
			exposureRows.Close()
			return snapshot, err
		}

		snapshot.CategoryExposure[category] = count
	}
	if err := exposureRows.Err(); err != nil {
		exposureRows.Close()
		return snapshot, err
	}

	exposureRows.Close()

	qualityProblems := map[string]bool{}
	for _, attempt := range snapshot.QualityAttempts {
		if qualityProblems[attempt.ProblemID] {
			continue
		}
		problem, err := scanProblem(p.Pool.QueryRow(ctx, problemQuery+` AND l.id=$1 GROUP BY l.id,p.id`, attempt.ProblemID))
		if err != nil {
			return snapshot, err
		}

		snapshot.Problems = append(snapshot.Problems, problem)
		qualityProblems[attempt.ProblemID] = true
	}
	return snapshot, nil
}

// RecommendationCandidates performs the operation.
func (p *Postgres) RecommendationCandidates(ctx context.Context, userID string, criteria practice.Criteria, premiumOptIn bool, goals practice.Goals, now time.Time) ([]practice.ProblemRecord, map[string]practice.ProblemHistory, error) {
	now = now.UTC()
	goal := goals[criteria.Difficulty]
	if goal <= 0 {
		goal = practice.DefaultGoals()[criteria.Difficulty]
	}
	filters := ` AND l.difficulty::text=$2
		AND ($3='' OR EXISTS(SELECT 1 FROM app.leetcode_problem_category_mappings fm JOIN app.leetcode_problem_categories fc ON fc.id=fm.category_id WHERE fm.leetcode_problem_id=l.id AND fc.deleted_at IS NULL AND lower(fc.normalized_name)=lower($3)))
		AND ($4 OR NOT l.is_premium)
		AND NOT EXISTS(SELECT 1 FROM app.recommendation_dismissals d WHERE d.user_id=$1 AND d.leetcode_problem_id=l.id AND d.excluded_until>$5)`
	unseen, err := scanProblem(p.Pool.QueryRow(ctx, problemQuery+filters+`
		AND NOT EXISTS(SELECT 1 FROM app.problem_attempts a WHERE a.user_id=$1 AND a.problem_id=l.problem_id AND a.deleted_at IS NULL)
		GROUP BY l.id,p.id ORDER BY l.leetcode_number ASC,l.id ASC LIMIT 1`, userID, string(criteria.Difficulty), criteria.Category, premiumOptIn, now))
	if err == nil {
		return []practice.ProblemRecord{unseen}, map[string]practice.ProblemHistory{}, nil
	}
	if !errors.Is(err, ErrNotFound) {
		return nil, nil, err
	}

	retry, err := scanProblem(p.Pool.QueryRow(ctx, problemQuery+filters+`
		AND EXISTS(
			SELECT 1 FROM app.problem_attempts a
			WHERE a.user_id=$1 AND a.problem_id=l.problem_id AND a.deleted_at IS NULL
			GROUP BY a.problem_id
			HAVING max(a.attempted_at)<=$5::timestamptz-interval '90 days'
			AND bool_or(
				NOT EXISTS(SELECT 1 FROM migration.row_provenance rp WHERE rp.target_table='problem_attempts' AND rp.target_id=a.id::text)
				AND (a.outcome IN ('not_solved','solved_with_hints') OR a.confidence<4 OR a.time_taken_minutes>$6)
			)
		)
		GROUP BY l.id,p.id ORDER BY l.leetcode_number ASC,l.id ASC LIMIT 1`, userID, string(criteria.Difficulty), criteria.Category, premiumOptIn, now, goal))
	if errors.Is(err, ErrNotFound) {
		return []practice.ProblemRecord{}, map[string]practice.ProblemHistory{}, nil
	}
	if err != nil {
		return nil, nil, err
	}

	var lastAttemptedAt time.Time
	if err := p.Pool.QueryRow(ctx, `SELECT max(a.attempted_at) FROM app.problem_attempts a JOIN app.leetcode_problems l ON l.problem_id=a.problem_id WHERE a.user_id=$1 AND l.id=$2 AND a.deleted_at IS NULL`, userID, retry.ID).Scan(&lastAttemptedAt); err != nil {
		return nil, nil, err
	}
	return []practice.ProblemRecord{retry}, map[string]practice.ProblemHistory{retry.ID: {LastAttemptedAt: lastAttemptedAt.UTC(), Weak: true}}, nil
}

// CreateAttempt creates a value.
func (p *Postgres) CreateAttempt(ctx context.Context, v practice.AttemptRecord) (practice.AttemptRecord, bool, error) {
	tx, err := p.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return practice.AttemptRecord{}, false, err
	}

	defer tx.Rollback(ctx)

	var enrollmentID *string
	if v.SeasonID != nil {
		var scopedEnrollmentID string
		err = tx.QueryRow(ctx, `SELECT e.id FROM app.enrollments e JOIN app.seasons s ON s.id=e.season_id WHERE e.user_id=$1 AND e.season_id=$2 AND e.state='active' AND e.deleted_at IS NULL AND s.status='open' AND s.deleted_at IS NULL`, v.UserID, *v.SeasonID).Scan(&scopedEnrollmentID)
		if err != nil {
			return practice.AttemptRecord{}, false, noRows(err)
		}

		enrollmentID = &scopedEnrollmentID
	}
	if v.WeekID != nil && v.SeasonID == nil {
		return practice.AttemptRecord{}, false, ErrNotFound
	}
	if v.WeekID != nil {
		var valid bool
		if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM app.season_weeks WHERE id=$1 AND season_id=$2 AND deleted_at IS NULL)`, *v.WeekID, *v.SeasonID).Scan(&valid); err != nil {
			return practice.AttemptRecord{}, false, err
		}
		if !valid {
			return practice.AttemptRecord{}, false, ErrNotFound
		}
	}
	var baseProblemID string
	if err = tx.QueryRow(ctx, `SELECT problem_id FROM app.leetcode_problems WHERE id=$1 AND deleted_at IS NULL`, v.ProblemID).Scan(&baseProblemID); err != nil {
		return practice.AttemptRecord{}, false, noRows(err)
	}
	if err = tx.QueryRow(ctx, `INSERT INTO app.problem_attempts(id,user_id,problem_id,enrollment_id,season_week_id,attempted_at,time_taken_minutes,outcome,confidence,notes_html,revision) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11) RETURNING revision`, v.ID, v.UserID, baseProblemID, enrollmentID, v.WeekID, v.AttemptedAt, v.Minutes, v.Outcome, v.Confidence, v.Notes, v.Revision).Scan(&v.Revision); err != nil {
		return v, false, mapPostgresError(err)
	}

	tag, err := tx.Exec(ctx, `UPDATE app.recommendations SET state='attempted',fulfilled_by_attempt_id=$3,state_changed_at=$4,revision=revision+1 WHERE user_id=$1 AND state='active' AND leetcode_problem_id=$2 AND deleted_at IS NULL`, v.UserID, v.ProblemID, v.ID, time.Now().UTC())
	if err != nil {
		return v, false, err
	}

	actorID := v.UserID
	if err := appendAuditTx(ctx, tx, audit.Event{ID: id.New(), ActorID: &actorID, Action: "attempt.created", SubjectType: "problem_attempt", SubjectID: v.ID, Data: map[string]any{}, OccurredAt: time.Now().UTC()}); err != nil {
		return v, false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return v, false, err
	}
	return v, tag.RowsAffected() > 0, nil
}

// UpdateAttempt updates a value.
func (p *Postgres) UpdateAttempt(ctx context.Context, id, userID string, revision int64, fn func(*practice.AttemptRecord) error) (practice.AttemptRecord, error) {
	tx, err := p.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return practice.AttemptRecord{}, err
	}

	defer tx.Rollback(ctx)
	v, err := scanAttempt(tx.QueryRow(ctx, `SELECT `+attemptColumns+` FROM app.problem_attempts a LEFT JOIN app.enrollments e ON e.id=a.enrollment_id WHERE a.id=$1 AND a.user_id=$2 AND a.deleted_at IS NULL FOR UPDATE OF a`, id, userID))
	if err != nil {
		return v, err
	}
	if v.Revision != revision {
		return v, ErrConflict
	}
	if err := fn(&v); err != nil {
		return v, err
	}
	if v.WeekID != nil && v.SeasonID == nil {
		return v, ErrConflict
	}
	var enrollmentID *string
	if v.SeasonID != nil {
		var resolved string
		if err := tx.QueryRow(ctx, `SELECT e.id FROM app.enrollments e JOIN app.seasons s ON s.id=e.season_id WHERE e.user_id=$1 AND e.season_id=$2 AND e.state='active' AND e.deleted_at IS NULL AND s.status='open' AND s.deleted_at IS NULL`, userID, *v.SeasonID).Scan(&resolved); err != nil {
			return v, ErrConflict
		}

		enrollmentID = &resolved
		if v.WeekID != nil {
			var valid bool
			if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM app.season_weeks WHERE id=$1 AND season_id=$2 AND deleted_at IS NULL)`, *v.WeekID, *v.SeasonID).Scan(&valid); err != nil || !valid {
				return v, ErrConflict
			}
		}
	}
	var baseProblemID string
	if err := tx.QueryRow(ctx, `SELECT problem_id FROM app.leetcode_problems WHERE id=$1 AND deleted_at IS NULL`, v.ProblemID).Scan(&baseProblemID); err != nil {
		return v, noRows(err)
	}

	err = tx.QueryRow(ctx, `UPDATE app.problem_attempts SET problem_id=$2,enrollment_id=$3,attempted_at=$4,time_taken_minutes=$5,outcome=$6,confidence=$7,notes_html=$8,season_week_id=$9,revision=revision+1 WHERE id=$1 AND user_id=$10 AND revision=$11 RETURNING revision`, id, baseProblemID, enrollmentID, v.AttemptedAt, v.Minutes, v.Outcome, v.Confidence, v.Notes, v.WeekID, userID, revision).Scan(&v.Revision)
	if err != nil {
		return v, mapPostgresError(err)
	}
	if err := appendAuditTx(ctx, tx, newAudit(userID, "attempt.updated", "problem_attempt", id, nil, time.Now())); err != nil {
		return v, err
	}
	if err := tx.Commit(ctx); err != nil {
		return v, err
	}
	return v, nil
}

// DeleteAttempt deletes a value.
func (p *Postgres) DeleteAttempt(ctx context.Context, id, userID string, revision int64) error {
	tx, err := p.Pool.Begin(ctx)
	if err != nil {
		return err
	}

	defer tx.Rollback(ctx)
	now := time.Now().UTC()
	tag, err := tx.Exec(ctx, `UPDATE app.problem_attempts SET deleted_at=$4,revision=revision+1 WHERE id=$1 AND user_id=$2 AND revision=$3 AND deleted_at IS NULL`, id, userID, revision, now)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		var exists bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM app.problem_attempts WHERE id=$1 AND user_id=$2 AND deleted_at IS NULL)`, id, userID).Scan(&exists); err != nil {
			return err
		}
		if exists {
			return ErrConflict
		}
		return ErrNotFound
	}
	if err := appendAuditTx(ctx, tx, newAudit(userID, "attempt.deleted", "problem_attempt", id, nil, now)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// GetActiveRecommendation retrieves a value.
func (p *Postgres) GetActiveRecommendation(ctx context.Context, userID string) (*practice.Recommendation, error) {
	const query = `SELECT r.id,r.user_id,l.id,l.leetcode_number,p.title,COALESCE(p.url,''),l.difficulty::text,l.is_premium,COALESCE(array_agg(c.normalized_name) FILTER (WHERE c.id IS NOT NULL),'{}')::text[],l.revision,r.difficulty::text,COALESCE(rc.normalized_name,''),r.rationale,r.rule_version,r.generated_at,r.revision FROM app.recommendations r JOIN app.leetcode_problems l ON l.id=r.leetcode_problem_id JOIN app.problems p ON p.id=l.problem_id LEFT JOIN app.leetcode_problem_category_mappings pcm ON pcm.leetcode_problem_id=l.id LEFT JOIN app.leetcode_problem_categories c ON c.id=pcm.category_id LEFT JOIN app.leetcode_problem_categories rc ON rc.id=r.category_id WHERE r.user_id=$1 AND r.state='active' AND r.deleted_at IS NULL GROUP BY r.id,l.id,p.id,rc.id`
	var v practice.Recommendation
	var difficulty, recommendationDifficulty string
	err := p.Pool.QueryRow(ctx, query, userID).Scan(&v.ID, &v.UserID, &v.Problem.ID, &v.Problem.Number, &v.Problem.Title, &v.Problem.Link, &difficulty, &v.Problem.Premium, &v.Problem.Categories, &v.Problem.Revision, &recommendationDifficulty, &v.Category, &v.Rationale, &v.RuleVersion, &v.CreatedAt, &v.Revision)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	v.Problem.Difficulty = practice.Difficulty(difficulty)
	v.Difficulty = practice.Difficulty(recommendationDifficulty)
	return &v, nil
}

// SaveRecommendation saves a value.
func (p *Postgres) SaveRecommendation(ctx context.Context, v practice.Recommendation) (practice.Recommendation, error) {
	if v.Revision < 1 {
		v.Revision = 1
	}
	var categoryID *string
	if v.Category != "" {
		_ = p.Pool.QueryRow(ctx, `SELECT id FROM app.leetcode_problem_categories WHERE lower(normalized_name)=lower($1) AND deleted_at IS NULL`, v.Category).Scan(&categoryID)
	}
	err := p.Pool.QueryRow(ctx, `INSERT INTO app.recommendations(id,user_id,leetcode_problem_id,category_id,difficulty,rationale,rule_version,generated_at,revision) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9) ON CONFLICT (id) DO NOTHING RETURNING id,revision`, v.ID, v.UserID, v.Problem.ID, categoryID, string(v.Difficulty), v.Rationale, v.RuleVersion, v.CreatedAt.UTC(), v.Revision).Scan(&v.ID, &v.Revision)
	if errors.Is(err, pgx.ErrNoRows) {
		active, getErr := p.GetActiveRecommendation(ctx, v.UserID)
		if getErr != nil {
			return practice.Recommendation{}, getErr
		}
		if active != nil && active.ID == v.ID {
			return *active, nil
		}
		return practice.Recommendation{}, ErrConflict
	}
	if err != nil {
		return practice.Recommendation{}, mapPostgresError(err)
	}
	return v, nil
}

// ListRecommendationDismissals lists matching values.
func (p *Postgres) ListRecommendationDismissals(ctx context.Context, userID string) ([]practice.Dismissal, error) {
	rows, err := p.Pool.Query(ctx, `SELECT l.id,d.dismissed_at FROM app.recommendation_dismissals d JOIN app.leetcode_problems l ON l.id=d.leetcode_problem_id WHERE d.user_id=$1 ORDER BY d.dismissed_at DESC`, userID)
	if err != nil {
		return nil, err
	}

	defer rows.Close()
	items := []practice.Dismissal{}
	for rows.Next() {
		var v practice.Dismissal
		if err := rows.Scan(&v.ProblemID, &v.DismissedAt); err != nil {
			return nil, err
		}

		items = append(items, v)
	}
	return items, rows.Err()
}

// DismissRecommendation performs the operation.
func (p *Postgres) DismissRecommendation(ctx context.Context, userID string, revision int64, reason string, at time.Time, dismissalID string) (practice.Recommendation, error) {
	tx, err := p.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return practice.Recommendation{}, err
	}

	defer tx.Rollback(ctx)
	var recommendationID, problemID string
	var currentRevision int64
	if err := tx.QueryRow(ctx, `SELECT id,leetcode_problem_id,revision FROM app.recommendations WHERE user_id=$1 AND state='active' AND deleted_at IS NULL FOR UPDATE`, userID).Scan(&recommendationID, &problemID, &currentRevision); err != nil {
		return practice.Recommendation{}, noRows(err)
	}
	if currentRevision != revision {
		return practice.Recommendation{}, ErrConflict
	}
	dismissedAt := at.UTC()
	if _, err := tx.Exec(ctx, `UPDATE app.recommendations SET state='dismissed',state_changed_at=$2,revision=revision+1 WHERE id=$1 AND state='active' AND revision=$3`, recommendationID, dismissedAt, revision); err != nil {
		return practice.Recommendation{}, err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO app.recommendation_dismissals(id,recommendation_id,user_id,leetcode_problem_id,reason,dismissed_at,excluded_until) VALUES($1,$2,$3,$4,NULLIF($5,''),$6,$7)`, dismissalID, recommendationID, userID, problemID, strings.TrimSpace(reason), dismissedAt, dismissedAt.Add(30*24*time.Hour)); err != nil {
		return practice.Recommendation{}, err
	}
	if err := appendAuditTx(ctx, tx, newAudit(userID, "recommendation.dismissed", "recommendation", recommendationID, map[string]any{"reason": strings.TrimSpace(reason)}, dismissedAt)); err != nil {
		return practice.Recommendation{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return practice.Recommendation{}, err
	}
	return practice.Recommendation{ID: recommendationID, UserID: userID, Problem: practice.Problem{ID: problemID}, DismissedAt: &dismissedAt, Revision: revision + 1}, nil
}

// FulfillRecommendation performs the operation.
func (p *Postgres) FulfillRecommendation(ctx context.Context, userID string, attempt practice.AttemptRecord, at time.Time) (bool, error) {
	tag, err := p.Pool.Exec(ctx, `UPDATE app.recommendations SET state='attempted',fulfilled_by_attempt_id=$3,state_changed_at=$4,revision=revision+1 WHERE user_id=$1 AND state='active' AND leetcode_problem_id=$2 AND deleted_at IS NULL`, userID, attempt.ProblemID, attempt.ID, at.UTC())
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

// ListMockInterviews lists matching values.
