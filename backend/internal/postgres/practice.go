package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/magedmg/RSP-website/backend/internal/platform/audit"
	"github.com/magedmg/RSP-website/backend/internal/platform/id"
	"github.com/magedmg/RSP-website/backend/internal/practice"
)

func (p *Postgres) GetPracticeSettings(ctx context.Context, userID string) (practice.PracticeSettings, error) {
	var settings practice.PracticeSettings
	err := p.Pool.QueryRow(ctx, `
	SELECT COALESCE(g.enabled,false), COALESCE(g.easy_minutes,20),
	       COALESCE(g.medium_minutes,35), COALESCE(g.hard_minutes,50),
       COALESCE(g.revision,1)
FROM app.users u
LEFT JOIN app.practice_goals g ON g.user_id=u.id
WHERE u.id=$1 AND u.deleted_at IS NULL`, userID).Scan(
		&settings.GoalsEnabled, &settings.EasyMinutes,
		&settings.MediumMinutes, &settings.HardMinutes, &settings.Revision,
	)
	return settings, noRows(err)
}

// UpdatePracticeSettings updates a value.
func (p *Postgres) UpdatePracticeSettings(ctx context.Context, userID string, revision int64, easy, medium, hard int, actorID string, at time.Time) (practice.PracticeSettings, error) {
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
	var settings practice.PracticeSettings
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

	data, _ := json.Marshal(map[string]any{"easyMinutes": easy, "mediumMinutes": medium, "hardMinutes": hard})
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
	if err := row.Scan(&v.ID, &v.UserID, &v.ProblemID, &v.Outcome, &v.Confidence, &v.Minutes, &v.Notes, &v.AttemptedAt, &v.SeasonID, &v.WeekID, &v.Revision, &v.DeletedAt); err != nil {
		return v, noRows(err)
	}
	return v, nil
}

const attemptColumns = `a.id,a.user_id,COALESCE((SELECT id FROM app.leetcode_problems WHERE problem_id=a.problem_id),a.problem_id),a.outcome::text,a.confidence,a.time_taken_minutes,COALESCE(a.notes_html,''),a.attempted_at,e.season_id,a.season_week_id,a.revision,a.deleted_at`

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

// CreateAttempt creates a value.
func (p *Postgres) CreateAttempt(ctx context.Context, v practice.AttemptRecord) (practice.AttemptRecord, error) {
	tx, err := p.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return practice.AttemptRecord{}, err
	}

	defer tx.Rollback(ctx)

	var enrollmentID *string
	if v.SeasonID != nil {
		var scopedEnrollmentID string
		err = tx.QueryRow(ctx, `SELECT e.id FROM app.enrollments e JOIN app.seasons s ON s.id=e.season_id WHERE e.user_id=$1 AND e.season_id=$2 AND e.state='active' AND e.deleted_at IS NULL AND s.status='open' AND s.deleted_at IS NULL`, v.UserID, *v.SeasonID).Scan(&scopedEnrollmentID)
		if err != nil {
			return practice.AttemptRecord{}, noRows(err)
		}

		enrollmentID = &scopedEnrollmentID
	}
	if v.WeekID != nil && v.SeasonID == nil {
		return practice.AttemptRecord{}, ErrNotFound
	}
	if v.WeekID != nil {
		var valid bool
		if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM app.season_weeks WHERE id=$1 AND season_id=$2 AND deleted_at IS NULL)`, *v.WeekID, *v.SeasonID).Scan(&valid); err != nil {
			return practice.AttemptRecord{}, err
		}
		if !valid {
			return practice.AttemptRecord{}, ErrNotFound
		}
	}
	var baseProblemID string
	if err = tx.QueryRow(ctx, `SELECT problem_id FROM app.leetcode_problems WHERE id=$1 AND deleted_at IS NULL`, v.ProblemID).Scan(&baseProblemID); err != nil {
		return practice.AttemptRecord{}, noRows(err)
	}
	if err = tx.QueryRow(ctx, `INSERT INTO app.problem_attempts(id,user_id,problem_id,enrollment_id,season_week_id,attempted_at,time_taken_minutes,outcome,confidence,notes_html,revision) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11) RETURNING revision`, v.ID, v.UserID, baseProblemID, enrollmentID, v.WeekID, v.AttemptedAt, v.Minutes, v.Outcome, v.Confidence, v.Notes, v.Revision).Scan(&v.Revision); err != nil {
		return v, mapPostgresError(err)
	}

	actorID := v.UserID
	if err := appendAuditTx(ctx, tx, audit.Event{ID: id.New(), ActorID: &actorID, Action: "attempt.created", SubjectType: "problem_attempt", SubjectID: v.ID, Data: map[string]any{}, OccurredAt: time.Now().UTC()}); err != nil {
		return v, err
	}
	if err := tx.Commit(ctx); err != nil {
		return v, err
	}
	return v, nil
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

// ListMockInterviews lists matching values.
