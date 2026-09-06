package dal

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/magedmg/RSP-website/backend/internal/platform/audit"
	"github.com/magedmg/RSP-website/backend/internal/platform/id"
	"github.com/magedmg/RSP-website/backend/internal/practice"
)

func scanAttempt(row pgx.Row) (practice.AttemptRecord, error) {
	var v practice.AttemptRecord
	if err := row.Scan(&v.ID, &v.UserID, &v.ProblemID, &v.Outcome, &v.Confidence, &v.Minutes, &v.Notes, &v.AttemptedAt, &v.SeasonID, &v.WeekID, &v.DeletedAt); err != nil {
		return v, noRows(err)
	}
	return v, nil
}

const attemptColumns = `a.id,a.user_id,COALESCE((SELECT id FROM app.leetcode_problems WHERE problem_id=a.problem_id),a.problem_id),a.outcome::text,a.confidence,a.time_taken_minutes,COALESCE(a.notes_html,''),a.attempted_at,e.season_id,a.season_week_id,a.deleted_at`

func (p *Store) GetAttempt(ctx context.Context, id string) (practice.AttemptRecord, error) {
	return scanAttempt(p.pool.QueryRow(ctx, `SELECT `+attemptColumns+`
		FROM app.problem_attempts a LEFT JOIN app.enrollments e ON e.id=a.enrollment_id
		WHERE a.id=$1`, id))
}

type AttemptQuery struct {
	UserID     string
	Boundary   string
	Limit      int
	Outcome    string
	Difficulty string
	Direction  string
}

func (p *Store) ListAttempts(ctx context.Context, q AttemptQuery) ([]practice.AttemptRecord, bool, int64, error) {
	const filters = `a.user_id = @userID AND a.deleted_at IS NULL
		AND (@outcome = '' OR a.outcome::text = @outcome)
		AND (@difficulty = '' OR EXISTS (
			SELECT 1 FROM app.leetcode_problems l
			WHERE l.problem_id = a.problem_id AND l.difficulty::text = @difficulty AND l.deleted_at IS NULL
		))`
	args := pgx.NamedArgs{
		"userID":     q.UserID,
		"outcome":    q.Outcome,
		"difficulty": q.Difficulty,
	}
	var total int64
	err := p.pool.QueryRow(ctx, `SELECT count(*) FROM app.problem_attempts a WHERE `+filters, args).Scan(&total)
	if err != nil {
		return nil, false, 0, err
	}

	comparison, order := `a.id > NULLIF(@boundary, '')::uuid`, `ASC`
	if q.Direction == "backward" {
		comparison, order = `a.id < NULLIF(@boundary, '')::uuid`, `DESC`
	}
	if q.Boundary == "" {
		comparison = `TRUE`
	}
	args["boundary"] = q.Boundary
	args["limit"] = q.Limit + 1
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
	out, more := finishPage(out, q.Limit, q.Direction)
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
	_, err = tx.Exec(ctx, `INSERT INTO app.problem_attempts(id,user_id,problem_id,enrollment_id,season_week_id,attempted_at,time_taken_minutes,outcome,confidence,notes_html)
	 VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`, v.ID, v.UserID, baseProblemID, enrollmentID, v.WeekID, v.AttemptedAt.UTC(), v.Minutes, v.Outcome, v.Confidence, v.Notes)
	if err != nil {
		return v, mapDatabaseError(err)
	}

	actorID := v.UserID
	if err := appendAuditTx(ctx, tx, audit.Event{
		ID:          id.New(),
		ActorID:     &actorID,
		Action:      "attempt.created",
		SubjectType: "problem_attempt",
		SubjectID:   v.ID,
		Data:        map[string]any{},
		OccurredAt:  time.Now().UTC(),
	}); err != nil {
		return v, err
	}
	if err := tx.Commit(ctx); err != nil {
		return v, err
	}
	return v, nil
}

type UpdateAttemptInput struct {
	AttemptID   string
	UserID      string
	ProblemID   string
	Outcome     string
	Confidence  *int
	Minutes     int
	Notes       string
	AttemptedAt time.Time
	SeasonID    *string
	WeekID      *string
	ChangedAt   time.Time
}

func (p *Store) UpdateAttempt(ctx context.Context, input UpdateAttemptInput) (practice.AttemptRecord, error) {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return practice.AttemptRecord{}, err
	}

	defer tx.Rollback(context.Background())
	v, err := scanAttempt(tx.QueryRow(ctx, `SELECT `+attemptColumns+` FROM app.problem_attempts a
		LEFT JOIN app.enrollments e ON e.id=a.enrollment_id
		WHERE a.id=$1 AND a.user_id=$2 AND a.deleted_at IS NULL FOR UPDATE OF a`, input.AttemptID, input.UserID))
	if err != nil {
		return practice.AttemptRecord{}, err
	}
	v.ProblemID = input.ProblemID
	v.Outcome = input.Outcome
	v.Confidence = input.Confidence
	v.Minutes = input.Minutes
	v.Notes = input.Notes
	v.AttemptedAt = input.AttemptedAt
	v.SeasonID = input.SeasonID
	v.WeekID = input.WeekID
	if v.WeekID != nil && v.SeasonID == nil {
		return v, ErrConflict
	}
	var enrollmentID *string
	if v.SeasonID != nil {
		resolved, err := resolveAttemptEnrollment(ctx, tx, input.UserID, *v.SeasonID)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				return v, ErrConflict
			}
			return v, err
		}

		enrollmentID = &resolved
		if v.WeekID != nil {
			valid, err := attemptWeekExists(ctx, tx, *v.WeekID, *v.SeasonID)
			if err != nil {
				return v, err
			}
			if !valid {
				return v, ErrConflict
			}
		}
	}
	baseProblemID, err := resolveBaseProblem(ctx, tx, v.ProblemID)
	if err != nil {
		return v, noRows(err)
	}

	_, err = tx.Exec(ctx, `UPDATE app.problem_attempts SET problem_id=$1,enrollment_id=$2,attempted_at=$3,time_taken_minutes=$4,outcome=$5,confidence=$6,notes_html=$7,season_week_id=$8 WHERE id = $9 AND user_id = $10`, baseProblemID, enrollmentID, v.AttemptedAt.UTC(), v.Minutes, v.Outcome, v.Confidence, v.Notes, v.WeekID, input.AttemptID, input.UserID)
	if err != nil {
		return v, mapDatabaseError(err)
	}
	if err := appendAuditTx(ctx, tx, newAudit(input.UserID, "attempt.updated", "problem_attempt", input.AttemptID, nil, input.ChangedAt)); err != nil {
		return v, err
	}
	if err := tx.Commit(ctx); err != nil {
		return v, err
	}
	return v, nil
}

func (p *Store) DeleteAttempt(ctx context.Context, id, userID string) error {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return err
	}

	defer tx.Rollback(context.Background())
	now := time.Now().UTC()
	result, err := tx.Exec(ctx, `UPDATE app.problem_attempts SET deleted_at=$1 WHERE id = $2 AND user_id = $3 AND deleted_at IS NULL`, now, id, userID)
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
