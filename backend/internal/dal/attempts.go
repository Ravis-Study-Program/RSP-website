package dal

import (
	"context"
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

const attemptColumns = `a.id,a.user_id,COALESCE((SELECT id FROM app.leetcode_problems WHERE problem_id=a.problem_id),a.problem_id),a.outcome::text,a.confidence,a.time_taken_minutes,COALESCE(a.notes_html,''),a.attempted_at,a.activity_season_id,a.activity_week_id,a.deleted_at`

func (p *Store) GetAttempt(ctx context.Context, id string) (practice.AttemptRecord, error) {
	return scanAttempt(p.pool.QueryRow(ctx, `SELECT `+attemptColumns+`
		FROM app.scoped_problem_attempts a
		WHERE a.id=$1`, id))
}

type AttemptQuery struct {
	Difficulties []string
	Categories   []string
	WeekIDs      []string
	SeasonID     string
	Year         int
	UserID       string
	Boundary     string
	Limit        int
	Outcome      string
	Difficulty   string
	Direction    string
}

func (p *Store) ListAttempts(ctx context.Context, q AttemptQuery) ([]practice.AttemptRecord, bool, int64, error) {
	const filters = `a.user_id = @userID AND a.deleted_at IS NULL
 AND (@seasonID = '' OR a.activity_season_id=NULLIF(@seasonID,'')::uuid)
 AND (@year = 0 OR EXTRACT(YEAR FROM a.attempted_at AT TIME ZONE 'Australia/Adelaide')=@year)
		AND (@outcome = '' OR a.outcome::text = @outcome)
  AND (COALESCE(cardinality(@weeks::uuid[]),0)=0 OR a.activity_week_id=ANY(@weeks::uuid[]))
  AND (COALESCE(cardinality(@difficulties::text[]),0)=0 OR EXISTS (
   SELECT 1 FROM app.leetcode_problems l WHERE l.problem_id=a.problem_id AND l.difficulty::text=ANY(@difficulties::text[]) AND l.deleted_at IS NULL))
  AND (COALESCE(cardinality(@categories::text[]),0)=0 OR EXISTS (
   SELECT 1 FROM app.leetcode_problems l
   JOIN app.leetcode_problem_category_mappings mapping ON mapping.leetcode_problem_id=l.id
   JOIN app.leetcode_problem_categories category ON category.id=mapping.category_id
   WHERE l.problem_id=a.problem_id AND l.deleted_at IS NULL AND category.deleted_at IS NULL AND category.name=ANY(@categories::text[])))`
	difficulties := q.Difficulties
	if q.Difficulty != "" {
		difficulties = append(append([]string{}, difficulties...), q.Difficulty)
	}
	args := pgx.NamedArgs{
		"userID":       q.UserID,
		"seasonID":     q.SeasonID,
		"year":         q.Year,
		"outcome":      q.Outcome,
		"difficulties": difficulties,
		"categories":   q.Categories, "weeks": q.WeekIDs,
	}
	var total int64
	err := p.pool.QueryRow(ctx, `SELECT count(*) FROM app.scoped_problem_attempts a WHERE `+filters, args).Scan(&total)
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
		FROM app.scoped_problem_attempts a
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

	baseProblemID, err := resolveBaseProblem(ctx, tx, v.ProblemID)
	if err != nil {
		return practice.AttemptRecord{}, noRows(err)
	}
	_, err = tx.Exec(ctx, `INSERT INTO app.problem_attempts(id,user_id,problem_id,enrollment_id,season_week_id,attempted_at,time_taken_minutes,outcome,confidence,notes_html)
	 VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`, v.ID, v.UserID, baseProblemID, nil, nil, v.AttemptedAt.UTC(), v.Minutes, v.Outcome, v.Confidence, v.Notes)
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
	return p.GetAttempt(ctx, v.ID)
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
	ChangedAt   time.Time
}

func (p *Store) UpdateAttempt(ctx context.Context, input UpdateAttemptInput) (practice.AttemptRecord, error) {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return practice.AttemptRecord{}, err
	}

	defer tx.Rollback(context.Background())
	if err := tx.QueryRow(ctx, `SELECT id FROM app.problem_attempts WHERE id=$1 AND user_id=$2 AND deleted_at IS NULL FOR UPDATE`, input.AttemptID, input.UserID).Scan(new(string)); err != nil {
		return practice.AttemptRecord{}, noRows(err)
	}
	v, err := scanAttempt(tx.QueryRow(ctx, `SELECT `+attemptColumns+` FROM app.scoped_problem_attempts a
		WHERE a.id=$1 AND a.user_id=$2 AND a.deleted_at IS NULL`, input.AttemptID, input.UserID))
	if err != nil {
		return practice.AttemptRecord{}, err
	}
	v.ProblemID = input.ProblemID
	v.Outcome = input.Outcome
	v.Confidence = input.Confidence
	v.Minutes = input.Minutes
	v.Notes = input.Notes
	v.AttemptedAt = input.AttemptedAt
	baseProblemID, err := resolveBaseProblem(ctx, tx, v.ProblemID)
	if err != nil {
		return v, noRows(err)
	}

	_, err = tx.Exec(ctx, `UPDATE app.problem_attempts SET problem_id=$1,enrollment_id=$2,attempted_at=$3,time_taken_minutes=$4,outcome=$5,confidence=$6,notes_html=$7,season_week_id=$8 WHERE id = $9 AND user_id = $10`, baseProblemID, nil, v.AttemptedAt.UTC(), v.Minutes, v.Outcome, v.Confidence, v.Notes, nil, input.AttemptID, input.UserID)
	if err != nil {
		return v, mapDatabaseError(err)
	}
	if err := appendAuditTx(ctx, tx, newAudit(input.UserID, "attempt.updated", "problem_attempt", input.AttemptID, nil, input.ChangedAt)); err != nil {
		return v, err
	}
	if err := tx.Commit(ctx); err != nil {
		return v, err
	}
	return p.GetAttempt(ctx, v.ID)
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

func resolveBaseProblem(ctx context.Context, tx pgx.Tx, leetcodeProblemID string) (string, error) {
	var problemID string
	err := tx.QueryRow(ctx, `SELECT problem_id FROM app.leetcode_problems WHERE id = $1 AND deleted_at IS NULL`, leetcodeProblemID).Scan(&problemID)
	return problemID, err
}
