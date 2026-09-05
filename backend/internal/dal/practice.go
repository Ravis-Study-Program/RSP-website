package dal

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/magedmg/RSP-website/backend/internal/platform/audit"
	"github.com/magedmg/RSP-website/backend/internal/platform/id"
	"github.com/magedmg/RSP-website/backend/internal/practice"
)

func (p *Store) GetPracticeSettings(ctx context.Context, userID string) (practice.PracticeSettings, error) {
	var settings practice.PracticeSettings
	err := p.pool.QueryRow(ctx, `SELECT COALESCE(g.enabled,false),
		COALESCE(g.easy_minutes,20),COALESCE(g.medium_minutes,35),COALESCE(g.hard_minutes,50),
		COALESCE(g.revision,1)
		FROM app.users u LEFT JOIN app.practice_goals g ON g.user_id=u.id
		WHERE u.id=$1 AND u.deleted_at IS NULL`, userID).Scan(
		&settings.GoalsEnabled, &settings.EasyMinutes, &settings.MediumMinutes,
		&settings.HardMinutes, &settings.Revision,
	)
	return settings, noRows(err)
}

func (p *Store) UpdatePracticeSettings(ctx context.Context, userID string, revision int64, easy, medium, hard int, actorID string, at time.Time) (practice.PracticeSettings, error) {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return practice.PracticeSettings{}, err
	}

	defer tx.Rollback(context.Background())
	if err := ensurePracticeGoals(ctx, tx, userID); err != nil {
		return practice.PracticeSettings{}, err
	}
	var currentRevision int64
	if err := tx.QueryRow(ctx, `SELECT revision FROM app.practice_goals WHERE user_id = $1 FOR UPDATE`, userID).Scan(&currentRevision); err != nil {
		return practice.PracticeSettings{}, err
	}
	if currentRevision != revision {
		return practice.PracticeSettings{}, ErrConflict
	}
	if easy < 1 || medium < 1 || hard < 1 {
		return practice.PracticeSettings{}, errors.New("practice goals must be positive")
	}
	result, err := tx.Exec(ctx, `UPDATE app.practice_goals SET easy_minutes=$1,medium_minutes=$2,hard_minutes=$3,revision=revision + 1 WHERE user_id = $4 AND revision = $5`, easy, medium, hard, userID, revision)
	if err != nil {
		return practice.PracticeSettings{}, err
	}
	if result.RowsAffected() == 0 {
		return practice.PracticeSettings{}, ErrConflict
	}
	settings := practice.PracticeSettings{GoalsEnabled: true, EasyMinutes: easy, MediumMinutes: medium, HardMinutes: hard, Revision: revision + 1}
	if err := tx.QueryRow(ctx, `SELECT enabled FROM app.practice_goals WHERE user_id = $1`, userID).Scan(&settings.GoalsEnabled); err != nil {
		return practice.PracticeSettings{}, err
	}

	if err := appendAuditTx(ctx, tx, audit.Event{
		ID:          id.New(),
		ActorID:     &actorID,
		Action:      "practice_settings.updated",
		SubjectType: "user",
		SubjectID:   userID,
		Data:        map[string]any{"easyMinutes": easy, "mediumMinutes": medium, "hardMinutes": hard},
		OccurredAt:  at.UTC(),
	}); err != nil {
		return practice.PracticeSettings{}, err
	}
	return settings, tx.Commit(ctx)
}

func (p *Store) EnablePracticeGoals(ctx context.Context, userID string, revision int64, actorID, seasonID string, at time.Time) (practice.PracticeSettings, error) {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return practice.PracticeSettings{}, err
	}

	defer tx.Rollback(context.Background())
	if err := ensurePracticeGoals(ctx, tx, userID); err != nil {
		return practice.PracticeSettings{}, err
	}
	var stored struct {
		Enabled                                 bool
		EasyMinutes, MediumMinutes, HardMinutes int
		Revision                                int64
	}
	if err := tx.QueryRow(ctx, `SELECT enabled,easy_minutes,medium_minutes,hard_minutes,revision FROM app.practice_goals WHERE user_id=$1 FOR UPDATE`, userID).Scan(&stored.Enabled, &stored.EasyMinutes, &stored.MediumMinutes, &stored.HardMinutes, &stored.Revision); err != nil {
		return practice.PracticeSettings{}, noRows(err)
	}
	if stored.Revision != revision {
		return practice.PracticeSettings{}, ErrConflict
	}
	changed := !stored.Enabled
	if changed {
		result, err := tx.Exec(ctx, `UPDATE app.practice_goals SET enabled=$1,enabled_by_user_id=$2,enabled_at=$3,revision=revision + 1 WHERE user_id = $4 AND revision = $5`, true, actorID, at.UTC(), userID, revision)
		if err != nil {
			return practice.PracticeSettings{}, err
		}
		if result.RowsAffected() == 0 {
			return practice.PracticeSettings{}, ErrConflict
		}
		stored.Enabled = true
		stored.Revision++
	}
	if changed {
		if err := appendAuditTx(ctx, tx, audit.Event{
			ID:          id.New(),
			ActorID:     &actorID,
			Action:      "practice_goals.enabled",
			SubjectType: "user",
			SubjectID:   userID,
			Data:        map[string]any{"seasonId": seasonID},
			OccurredAt:  at.UTC(),
		}); err != nil {
			return practice.PracticeSettings{}, err
		}
	}
	settings := practice.PracticeSettings{
		GoalsEnabled:  stored.Enabled,
		EasyMinutes:   stored.EasyMinutes,
		MediumMinutes: stored.MediumMinutes,
		HardMinutes:   stored.HardMinutes,
		Revision:      stored.Revision,
	}
	return settings, tx.Commit(ctx)
}

func ensurePracticeGoals(ctx context.Context, tx pgx.Tx, userID string) error {
	_, err := tx.Exec(ctx, `INSERT INTO app.practice_goals(user_id,enabled,easy_minutes,medium_minutes,hard_minutes,revision)
 VALUES($1,$2,$3,$4,$5,$6) ON CONFLICT DO NOTHING`, userID, false, 20, 35, 50, 1)
	return err
}

func scanProblem(row rowScanner) (practice.ProblemRecord, error) {
	var v practice.ProblemRecord
	var categories []byte
	if err := row.Scan(&v.ID, &v.Number, &v.Title, &v.Link, &v.Difficulty, &v.Premium, &categories, &v.Revision); err != nil {
		return v, noRows(err)
	}
	if err := json.Unmarshal(categories, &v.Categories); err != nil {
		return v, err
	}
	return v, nil
}

const problemQuery = `SELECT l.id,l.leetcode_number,p.title,COALESCE(p.url,''),l.difficulty::text,l.is_premium,
	COALESCE(to_jsonb(array_agg(c.normalized_name) FILTER (WHERE c.id IS NOT NULL)),'[]'::jsonb),l.revision
	FROM app.leetcode_problems l
	JOIN app.problems p ON p.id=l.problem_id
	LEFT JOIN app.leetcode_problem_category_mappings m ON m.leetcode_problem_id=l.id
	LEFT JOIN app.leetcode_problem_categories c ON c.id=m.category_id
	WHERE l.deleted_at IS NULL AND p.deleted_at IS NULL`

func (p *Store) ListProblems(ctx context.Context, boundary string, limit int, difficulty, category string, premium *bool, direction string) ([]practice.ProblemRecord, bool, int64, error) {
	const filters = `(@difficulty = '' OR l.difficulty::text = @difficulty)
		AND (@category = '' OR EXISTS (
			SELECT 1 FROM app.leetcode_problem_category_mappings fm
			JOIN app.leetcode_problem_categories fc ON fc.id = fm.category_id
			WHERE fm.leetcode_problem_id = l.id AND fc.deleted_at IS NULL
			AND lower(fc.normalized_name) = lower(@category)
		))
		AND (CAST(@premium AS boolean) IS NULL OR l.is_premium = @premium)`
	args := pgx.NamedArgs{"difficulty": difficulty, "category": category, "premium": premium}
	var total int64
	err := p.pool.QueryRow(ctx, `SELECT count(*) FROM app.leetcode_problems l
		JOIN app.problems p ON p.id = l.problem_id
		WHERE l.deleted_at IS NULL AND p.deleted_at IS NULL AND `+filters, args).Scan(&total)
	if err != nil {
		return nil, false, 0, err
	}

	comparison, order := `l.id > NULLIF(@boundary, '')::uuid`, `ASC`
	if direction == "backward" {
		comparison, order = `l.id < NULLIF(@boundary, '')::uuid`, `DESC`
	}
	if boundary == "" {
		comparison = `TRUE`
	}
	args["boundary"] = boundary
	args["limit"] = limit + 1
	rows, err := p.pool.Query(ctx, problemQuery+`
		AND `+comparison+` AND `+filters+`
		GROUP BY l.id,p.id
		ORDER BY l.id `+order+` LIMIT @limit`, args)
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
	out, more := finishStorePage(out, limit, direction)
	return out, more, total, rows.Err()
}

func scanAttempt(row rowScanner) (practice.AttemptRecord, error) {
	var v practice.AttemptRecord
	if err := row.Scan(&v.ID, &v.UserID, &v.ProblemID, &v.Outcome, &v.Confidence, &v.Minutes, &v.Notes, &v.AttemptedAt, &v.SeasonID, &v.WeekID, &v.Revision, &v.DeletedAt); err != nil {
		return v, noRows(err)
	}
	return v, nil
}

const attemptColumns = `a.id,a.user_id,COALESCE((SELECT id FROM app.leetcode_problems WHERE problem_id=a.problem_id),a.problem_id),a.outcome::text,a.confidence,a.time_taken_minutes,COALESCE(a.notes_html,''),a.attempted_at,e.season_id,a.season_week_id,a.revision,a.deleted_at`

func (p *Store) GetAttempt(ctx context.Context, id string) (practice.AttemptRecord, error) {
	return scanAttempt(p.pool.QueryRow(ctx, `SELECT `+attemptColumns+`
		FROM app.problem_attempts a LEFT JOIN app.enrollments e ON e.id=a.enrollment_id
		WHERE a.id=$1`, id))
}

func (p *Store) ListAttempts(ctx context.Context, userID, boundary string, limit int, outcome, difficulty, direction string) ([]practice.AttemptRecord, bool, int64, error) {
	const filters = `a.user_id = @userID AND a.deleted_at IS NULL
		AND (@outcome = '' OR a.outcome::text = @outcome)
		AND (@difficulty = '' OR EXISTS (
			SELECT 1 FROM app.leetcode_problems l
			WHERE l.problem_id = a.problem_id AND l.difficulty::text = @difficulty AND l.deleted_at IS NULL
		))`
	args := pgx.NamedArgs{"userID": userID, "outcome": outcome, "difficulty": difficulty}
	var total int64
	err := p.pool.QueryRow(ctx, `SELECT count(*) FROM app.problem_attempts a WHERE `+filters, args).Scan(&total)
	if err != nil {
		return nil, false, 0, err
	}

	comparison, order := `a.id > NULLIF(@boundary, '')::uuid`, `ASC`
	if direction == "backward" {
		comparison, order = `a.id < NULLIF(@boundary, '')::uuid`, `DESC`
	}
	if boundary == "" {
		comparison = `TRUE`
	}
	args["boundary"] = boundary
	args["limit"] = limit + 1
	rows, err := p.pool.Query(ctx, `SELECT `+attemptColumns+`
		FROM app.problem_attempts a
		LEFT JOIN app.enrollments e ON e.id = a.enrollment_id
		WHERE `+filters+` AND `+comparison+`
		ORDER BY a.id `+order+` LIMIT @limit`, args)
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
	out, more := finishStorePage(out, limit, direction)
	return out, more, total, rows.Err()
}

func (p *Store) CreateAttempt(ctx context.Context, v practice.AttemptRecord) (practice.AttemptRecord, error) {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return practice.AttemptRecord{}, err
	}
	defer tx.Rollback(context.Background())

	var enrollmentID *string
	if v.SeasonID != nil {
		scopedEnrollmentID, err := resolveAttemptEnrollment(ctx, tx, v.UserID, *v.SeasonID)
		if err != nil {
			return practice.AttemptRecord{}, noRows(err)
		}
		enrollmentID = &scopedEnrollmentID
	}
	if v.WeekID != nil && v.SeasonID == nil {
		return practice.AttemptRecord{}, ErrNotFound
	}
	if v.WeekID != nil {
		valid, err := attemptWeekExists(ctx, tx, *v.WeekID, *v.SeasonID)
		if err != nil {
			return practice.AttemptRecord{}, err
		}
		if !valid {
			return practice.AttemptRecord{}, ErrNotFound
		}
	}
	baseProblemID, err := resolveBaseProblem(ctx, tx, v.ProblemID)
	if err != nil {
		return practice.AttemptRecord{}, noRows(err)
	}
	_, err = tx.Exec(ctx, `INSERT INTO app.problem_attempts(id,user_id,problem_id,enrollment_id,season_week_id,attempted_at,time_taken_minutes,outcome,confidence,notes_html,revision)
 VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`, v.ID, v.UserID, baseProblemID, enrollmentID, v.WeekID, v.AttemptedAt.UTC(), v.Minutes, v.Outcome, v.Confidence, v.Notes, v.Revision)
	if err != nil {
		return v, mapStoreError(err)
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

func (p *Store) UpdateAttempt(ctx context.Context, id, userID string, revision int64, fn func(*practice.AttemptRecord) error) (practice.AttemptRecord, error) {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return practice.AttemptRecord{}, err
	}

	defer tx.Rollback(context.Background())
	v, err := scanAttempt(tx.QueryRow(ctx, `SELECT `+attemptColumns+` FROM app.problem_attempts a
		LEFT JOIN app.enrollments e ON e.id=a.enrollment_id
		WHERE a.id=$1 AND a.user_id=$2 AND a.deleted_at IS NULL FOR UPDATE OF a`, id, userID))
	if err != nil {
		return practice.AttemptRecord{}, err
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
		resolved, err := resolveAttemptEnrollment(ctx, tx, userID, *v.SeasonID)
		if err != nil {
			return v, ErrConflict
		}

		enrollmentID = &resolved
		if v.WeekID != nil {
			valid, err := attemptWeekExists(ctx, tx, *v.WeekID, *v.SeasonID)
			if err != nil || !valid {
				return v, ErrConflict
			}
		}
	}
	baseProblemID, err := resolveBaseProblem(ctx, tx, v.ProblemID)
	if err != nil {
		return v, noRows(err)
	}

	result, err := tx.Exec(ctx, `UPDATE app.problem_attempts SET problem_id=$1,enrollment_id=$2,attempted_at=$3,time_taken_minutes=$4,outcome=$5,confidence=$6,notes_html=$7,season_week_id=$8,revision=revision + 1 WHERE id = $9 AND user_id = $10 AND revision = $11`, baseProblemID, enrollmentID, v.AttemptedAt.UTC(), v.Minutes, v.Outcome, v.Confidence, v.Notes, v.WeekID, id, userID, revision)
	if err != nil {
		return v, mapStoreError(err)
	}
	if result.RowsAffected() == 0 {
		return v, ErrConflict
	}
	v.Revision++
	if err := appendAuditTx(ctx, tx, newAudit(userID, "attempt.updated", "problem_attempt", id, nil, time.Now())); err != nil {
		return v, err
	}
	if err := tx.Commit(ctx); err != nil {
		return v, err
	}
	return v, nil
}

func (p *Store) DeleteAttempt(ctx context.Context, id, userID string, revision int64) error {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return err
	}

	defer tx.Rollback(context.Background())
	now := time.Now().UTC()
	result, err := tx.Exec(ctx, `UPDATE app.problem_attempts SET deleted_at=$1,revision=revision + 1 WHERE id = $2 AND user_id = $3 AND revision = $4 AND deleted_at IS NULL`, now, id, userID, revision)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		var count int64
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM app.problem_attempts WHERE id = $1 AND user_id = $2 AND deleted_at IS NULL`, id, userID).Scan(&count); err != nil {
			return err
		}
		if count > 0 {
			return ErrConflict
		}
		return ErrNotFound
	}
	if err := appendAuditTx(ctx, tx, newAudit(userID, "attempt.deleted", "problem_attempt", id, nil, now)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func resolveAttemptEnrollment(ctx context.Context, tx pgx.Tx, userID, seasonID string) (string, error) {
	var enrollmentID string
	err := tx.QueryRow(ctx, `SELECT e.id FROM app.enrollments e JOIN app.seasons s ON s.id=e.season_id
		WHERE e.user_id=$1 AND e.season_id=$2 AND e.state='active' AND e.deleted_at IS NULL
		AND s.status='open' AND s.deleted_at IS NULL`, userID, seasonID).Scan(&enrollmentID)
	return enrollmentID, err
}

func attemptWeekExists(ctx context.Context, tx pgx.Tx, weekID, seasonID string) (bool, error) {
	var count int64
	err := tx.QueryRow(ctx, `SELECT count(*) FROM app.season_weeks WHERE id = $1 AND season_id = $2 AND deleted_at IS NULL`, weekID, seasonID).Scan(&count)
	return count > 0, err
}

func resolveBaseProblem(ctx context.Context, tx pgx.Tx, leetcodeProblemID string) (string, error) {
	var problemID string
	err := tx.QueryRow(ctx, `SELECT problem_id FROM app.leetcode_problems WHERE id = $1 AND deleted_at IS NULL`, leetcodeProblemID).Scan(&problemID)
	return problemID, err
}
