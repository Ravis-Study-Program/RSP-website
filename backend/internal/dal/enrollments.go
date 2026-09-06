package dal

import (
	"context"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/magedmg/RSP-website/backend/internal/platform/audit"
	"github.com/magedmg/RSP-website/backend/internal/platform/id"
	"github.com/magedmg/RSP-website/backend/internal/programme"
)

const enrollmentColumns = `e.id,e.season_id,s.slug,e.user_id,e.role::text,e.student_level::text,e.state::text,e.assignment_state::text,CASE WHEN e.state IN ('kicked','withdrawn') THEN (SELECT reason FROM app.enrollment_removal_events WHERE enrollment_id=e.id ORDER BY occurred_at DESC,id DESC LIMIT 1) END`

func scanEnrollment(row pgx.Row) (programme.EnrollmentRecord, error) {
	var v programme.EnrollmentRecord
	if err := row.Scan(&v.ID, &v.SeasonID, &v.SeasonSlug, &v.UserID, &v.Role, &v.StudentLevel, &v.State, &v.AssignmentState, &v.RemovalReason); err != nil {
		return v, noRows(err)
	}
	return v, nil
}

type EnrollmentQuery struct {
	SeasonID        string
	Boundary        string
	Limit           int
	Role            string
	State           string
	SortBy          string
	Direction       string
	IncludeInactive bool
}

func (p *Store) ListEnrollments(ctx context.Context, q EnrollmentQuery) ([]programme.EnrollmentRecord, bool, int64, error) {
	const filters = `e.season_id = @seasonID AND (e.deleted_at IS NULL OR (@includeInactive AND e.state IN ('kicked','withdrawn')))
		AND (@role = '' OR e.role::text = @role) AND (@state = '' OR e.state::text = @state)
		AND (@includeInactive OR e.state = 'active' OR (e.role = 'student' AND e.state = 'completed'))`
	args := pgx.NamedArgs{
		"seasonID":        q.SeasonID,
		"role":            q.Role,
		"state":           q.State,
		"includeInactive": q.IncludeInactive,
	}
	var total int64
	if err := p.pool.QueryRow(ctx, `SELECT count(*) FROM app.enrollments e WHERE `+filters, args).Scan(&total); err != nil {
		return nil, false, 0, err
	}

	comparator, order := ">", "ASC"
	if q.Direction == "backward" {
		comparator, order = "<", "DESC"
	}
	key, boundaryKey := "e.id", "b.id"
	boundarySelect := "id"
	if q.SortBy == "role:asc" {
		key, boundaryKey = "ROW(e.role::text,e.id)", "ROW(b.role::text,b.id)"
		boundarySelect = "role,id"
	}
	orderBy := "e.id " + order
	if q.SortBy == "role:asc" {
		orderBy = "e.role::text " + order + ",e.id " + order
	}
	query := `WITH boundary AS (
		SELECT ` + boundarySelect + ` FROM app.enrollments
		WHERE id=NULLIF(@boundary, '')::uuid AND season_id = @seasonID AND deleted_at IS NULL
	) SELECT ` + enrollmentColumns + `
	FROM app.enrollments e JOIN app.seasons s ON s.id=e.season_id
	WHERE ` + filters + `
	  AND (NULLIF(@boundary, '')::uuid IS NULL OR EXISTS (SELECT 1 FROM boundary b WHERE ` + key + ` ` + comparator + ` ` + boundaryKey + `))
	ORDER BY ` + orderBy + ` LIMIT @limit`
	args["boundary"] = q.Boundary
	args["limit"] = q.Limit + 1
	rows, err := p.pool.Query(ctx, query, args)
	if err != nil {
		return nil, false, 0, err
	}

	defer rows.Close()
	items := make([]programme.EnrollmentRecord, 0, q.Limit+1)
	for rows.Next() {
		v, scanErr := scanEnrollment(rows)
		if scanErr != nil {
			return nil, false, 0, scanErr
		}
		items = append(items, v)
	}
	if err := rows.Err(); err != nil {
		return nil, false, 0, err
	}

	items, more := finishPage(items, q.Limit, q.Direction)
	return items, more, total, nil
}

func (p *Store) ListEnrollmentsForUser(ctx context.Context, userID string) ([]programme.EnrollmentRecord, error) {
	return p.listEnrollments(ctx, `e.user_id=$1`, userID)
}

func (p *Store) listEnrollments(ctx context.Context, predicate, value string) ([]programme.EnrollmentRecord, error) {
	rows, err := p.pool.Query(ctx, `SELECT `+enrollmentColumns+` FROM app.enrollments e JOIN app.seasons s ON s.id=e.season_id WHERE `+predicate+` AND (e.deleted_at IS NULL OR e.state IN ('kicked','withdrawn')) ORDER BY e.id`, value)
	if err != nil {
		return nil, err
	}

	defer rows.Close()
	items := []programme.EnrollmentRecord{}
	for rows.Next() {
		v, err := scanEnrollment(rows)
		if err != nil {
			return nil, err
		}

		items = append(items, v)
	}
	return items, rows.Err()
}

func (p *Store) GetEnrollment(ctx context.Context, enrollmentID string) (programme.EnrollmentRecord, error) {
	return scanEnrollment(p.pool.QueryRow(ctx, `SELECT `+enrollmentColumns+` FROM app.enrollments e JOIN app.seasons s ON s.id=e.season_id WHERE e.id=$1 AND e.deleted_at IS NULL`, enrollmentID))
}

func (p *Store) CreateEnrollment(ctx context.Context, v programme.EnrollmentRecord, actorID string, at time.Time) (programme.EnrollmentRecord, error) {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return v, err
	}

	defer tx.Rollback(context.Background())
	studentLevel := "not_applicable"
	if v.Role == "student" {
		studentLevel = "novice"
	}
	assignmentState := "active"
	if v.Role == "coordinator" {
		assignmentState = "pending_mfa"
	}
	err = tx.QueryRow(ctx, `INSERT INTO app.enrollments(id,user_id,season_id,role,student_level,state,assignment_state) VALUES($1,$2,$3,$4,$5,$6,$7) RETURNING assignment_state::text`, v.ID, v.UserID, v.SeasonID, v.Role, studentLevel, v.State, assignmentState).Scan(&v.AssignmentState)
	if err != nil {
		return v, mapDatabaseError(err)
	}

	v.StudentLevel = studentLevel
	if err := appendAuditTx(ctx, tx, newAudit(actorID, "enrollment.created", "enrollment", v.ID, nil, at)); err != nil {
		return v, err
	}
	return v, tx.Commit(ctx)
}

type UpdateEnrollmentRoleAndLevelInput struct {
	SeasonID     string
	EnrollmentID string
	Role         string
	StudentLevel string
	ActorID      string
	ChangedAt    time.Time
}

func (p *Store) UpdateEnrollmentRoleAndLevel(ctx context.Context, input UpdateEnrollmentRoleAndLevelInput) (programme.EnrollmentRecord, error) {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return programme.EnrollmentRecord{}, err
	}

	defer tx.Rollback(context.Background())
	v, err := scanEnrollment(tx.QueryRow(ctx, `UPDATE app.enrollments e SET role=$3::app.season_role,student_level=$4,state_changed_at=CASE WHEN role<>$3::app.season_role THEN $5::timestamptz ELSE state_changed_at END,assignment_state=CASE WHEN $3::app.season_role='coordinator' THEN 'pending_mfa'::app.assignment_state ELSE 'active'::app.assignment_state END,activated_at=CASE WHEN $3::app.season_role='coordinator' THEN NULL ELSE $5::timestamptz END FROM app.seasons s WHERE e.id=$1 AND e.season_id=$2 AND e.state='active' AND e.deleted_at IS NULL AND s.id=e.season_id RETURNING `+enrollmentColumns, input.EnrollmentID, input.SeasonID, input.Role, input.StudentLevel, input.ChangedAt.UTC()))
	if err != nil {
		return v, err
	}
	if input.Role != "student" {
		_, err = tx.Exec(ctx, `UPDATE app.mentorships SET ended_at=$2 WHERE student_enrollment_id=$1 AND ended_at IS NULL AND deleted_at IS NULL`, input.EnrollmentID, input.ChangedAt.UTC())
	} else {
		_, err = tx.Exec(ctx, `UPDATE app.mentorships SET ended_at=$2 WHERE mentor_enrollment_id=$1 AND ended_at IS NULL AND deleted_at IS NULL`, input.EnrollmentID, input.ChangedAt.UTC())
	}
	if err != nil {
		return v, err
	}
	if err := appendAuditTx(ctx, tx, newAudit(input.ActorID, "enrollment.updated", "enrollment", input.EnrollmentID, map[string]any{"role": input.Role, "studentLevel": input.StudentLevel}, input.ChangedAt)); err != nil {
		return v, err
	}
	return v, tx.Commit(ctx)
}

type PromoteEnrollmentInput struct {
	EnrollmentID string
	Role         string
	ActorID      string
	ChangedAt    time.Time
}

func (p *Store) PromoteEnrollment(ctx context.Context, input PromoteEnrollmentInput) (programme.EnrollmentRecord, error) {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return programme.EnrollmentRecord{}, err
	}

	defer tx.Rollback(context.Background())
	v, err := scanEnrollment(tx.QueryRow(ctx, `SELECT `+enrollmentColumns+` FROM app.enrollments e JOIN app.seasons s ON s.id=e.season_id WHERE e.id=$1 AND e.deleted_at IS NULL FOR UPDATE OF e`, input.EnrollmentID))
	if err != nil {
		return v, err
	}
	if v.State != "active" {
		return v, ErrConflict
	}
	studentLevel := "not_applicable"
	if input.Role == "student" {
		studentLevel = "novice"
	}
	if err := tx.QueryRow(ctx, `UPDATE app.enrollments SET role=$2::app.season_role,student_level=$3,state_changed_at=$4,assignment_state=CASE WHEN $2::app.season_role='coordinator' THEN 'pending_mfa'::app.assignment_state ELSE 'active'::app.assignment_state END,activated_at=CASE WHEN $2::app.season_role='coordinator' THEN NULL ELSE $4::timestamptz END WHERE id=$1 AND state='active' RETURNING assignment_state::text`, input.EnrollmentID, input.Role, studentLevel, input.ChangedAt.UTC()).Scan(&v.AssignmentState); err != nil {
		return v, noRows(err)
	}

	v.Role = input.Role
	v.StudentLevel = studentLevel
	if _, err := tx.Exec(ctx, `UPDATE app.mentorships SET ended_at=$2 WHERE student_enrollment_id=$1 AND ended_at IS NULL AND deleted_at IS NULL`, v.ID, input.ChangedAt.UTC()); err != nil {
		return v, err
	}
	if err := appendAuditTx(ctx, tx, audit.Event{
		ID:          id.New(),
		ActorID:     &input.ActorID,
		Action:      "enrollment.promoted",
		SubjectType: "enrollment",
		SubjectID:   v.ID,
		Data:        map[string]any{"role": input.Role},
		OccurredAt:  input.ChangedAt.UTC(),
	}); err != nil {
		return v, err
	}
	if err := tx.Commit(ctx); err != nil {
		return v, err
	}
	return v, nil
}

type RemoveEnrollmentInput struct {
	EnrollmentID string
	Reason       string
	ActorID      string
	ChangedAt    time.Time
}

func (p *Store) RemoveEnrollment(ctx context.Context, input RemoveEnrollmentInput) (programme.EnrollmentRecord, error) {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return programme.EnrollmentRecord{}, err
	}

	defer tx.Rollback(context.Background())
	v, err := scanEnrollment(tx.QueryRow(ctx, `SELECT `+enrollmentColumns+` FROM app.enrollments e JOIN app.seasons s ON s.id=e.season_id WHERE e.id=$1 AND e.deleted_at IS NULL FOR UPDATE OF e`, input.EnrollmentID))
	if err != nil {
		return v, err
	}
	if v.State != "active" {
		return v, ErrConflict
	}
	state := "kicked"
	changedAt := input.ChangedAt.UTC()
	if err := tx.QueryRow(ctx, `UPDATE app.enrollments SET state=$2,state_changed_at=$3,assignment_state='revoked',activated_at=NULL WHERE id=$1 AND state='active' RETURNING id`, input.EnrollmentID, state, changedAt).Scan(&v.ID); err != nil {
		return v, noRows(err)
	}

	v.State = state
	if _, err := tx.Exec(ctx, `UPDATE app.mentorships SET ended_at=$2 WHERE (student_enrollment_id=$1 OR mentor_enrollment_id=$1) AND ended_at IS NULL AND deleted_at IS NULL`, v.ID, changedAt); err != nil {
		return v, err
	}

	trimmed := strings.TrimSpace(input.Reason)
	v.RemovalReason = &trimmed
	if _, err := tx.Exec(ctx, `INSERT INTO app.enrollment_removal_events(id,enrollment_id,season_id,subject_user_id,actor_user_id,resulting_state,reason,occurred_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, id.New(), v.ID, v.SeasonID, v.UserID, input.ActorID, state, trimmed, changedAt); err != nil {
		return v, err
	}
	if err := appendAuditTx(ctx, tx, audit.Event{
		ID:          id.New(),
		ActorID:     &input.ActorID,
		Action:      "enrollment.removed",
		SubjectType: "enrollment",
		SubjectID:   v.ID,
		Data:        map[string]any{"reason": trimmed, "state": state},
		OccurredAt:  changedAt,
	}); err != nil {
		return v, err
	}
	if err := tx.Commit(ctx); err != nil {
		return v, err
	}
	return v, nil
}
