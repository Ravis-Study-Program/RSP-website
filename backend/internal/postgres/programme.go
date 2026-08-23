package postgres

import (
	"context"
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/magedmg/RSP-website/backend/internal/platform/audit"
	"github.com/magedmg/RSP-website/backend/internal/platform/dbgen"
	"github.com/magedmg/RSP-website/backend/internal/platform/id"
	"github.com/magedmg/RSP-website/backend/internal/programme"
)

func scanSeason(row pgx.Row) (programme.SeasonRecord, error) {
	var v programme.SeasonRecord
	if err := row.Scan(&v.ID, &v.Slug, &v.Name, &v.Status, &v.StartAt, &v.EndAt, &v.Location, &v.ImageURL, &v.ResourcesURL, &v.Revision); err != nil {
		return v, noRows(err)
	}
	return v, nil
}

const seasonColumns = `id,slug,name,status::text,start_at,end_at,location,image_url,resources_url,revision`

// GetSeason retrieves a value.
func (p *Postgres) GetSeason(ctx context.Context, id string) (programme.SeasonRecord, error) {
	row, err := p.Queries.GetSeasonByID(ctx, dbgen.GetSeasonByIDParams{ID: id})
	if err != nil {
		return programme.SeasonRecord{}, noRows(err)
	}
	return programme.SeasonRecord{ID: row.ID, Slug: row.Slug, Name: row.Name, Status: string(row.Status), StartAt: row.StartAt.Time, EndAt: row.EndAt.Time, Location: row.Location, ImageURL: row.ImageUrl, ResourcesURL: row.ResourcesUrl, Revision: row.Revision}, nil
}

// ListSeasons lists matching values.
func (p *Postgres) ListSeasons(ctx context.Context, boundary string, limit int, direction, status string) ([]programme.SeasonRecord, bool, int64, error) {
	var total int64
	if err := p.Pool.QueryRow(ctx, `SELECT count(*) FROM app.seasons WHERE deleted_at IS NULL AND ($1='' OR status::text=$1)`, status).Scan(&total); err != nil {
		return nil, false, 0, err
	}

	comparison, order := `id>NULLIF($1,'')::uuid`, `ASC`
	if direction == "backward" {
		comparison, order = `id<NULLIF($1,'')::uuid`, `DESC`
	}
	if boundary == "" {
		comparison = `$1::text IS NOT NULL`
	}
	rows, err := p.Pool.Query(ctx, `SELECT `+seasonColumns+` FROM app.seasons WHERE `+comparison+` AND deleted_at IS NULL AND ($3='' OR status::text=$3) ORDER BY id `+order+` LIMIT $2`, boundary, limit+1, status)
	if err != nil {
		return nil, false, 0, err
	}

	defer rows.Close()
	out := []programme.SeasonRecord{}
	for rows.Next() {
		v, err := scanSeason(rows)
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

// CreateSeason creates a value.
func (p *Postgres) CreateSeason(ctx context.Context, v programme.SeasonRecord, actorID string, at time.Time) (programme.SeasonRecord, error) {
	tx, err := p.Pool.Begin(ctx)
	if err != nil {
		return v, err
	}

	defer tx.Rollback(ctx)
	err = tx.QueryRow(ctx, `INSERT INTO app.seasons(id,slug,name,status,start_at,end_at,location,image_url,resources_url,revision) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10) RETURNING revision`, v.ID, v.Slug, v.Name, v.Status, v.StartAt, v.EndAt, v.Location, v.ImageURL, v.ResourcesURL, v.Revision).Scan(&v.Revision)
	if err != nil {
		if isUnique(err) {
			return v, ErrDuplicate
		}
		return v, err
	}
	if err := appendAuditTx(ctx, tx, newAudit(actorID, "season.created", "season", v.ID, nil, at)); err != nil {
		return v, err
	}
	return v, tx.Commit(ctx)
}

// UpdateSeason updates a value.
func (p *Postgres) UpdateSeason(ctx context.Context, id string, revision int64, fn func(*programme.SeasonRecord) error, actorID string, at time.Time) (programme.SeasonRecord, error) {
	tx, err := p.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return programme.SeasonRecord{}, err
	}

	defer tx.Rollback(ctx)
	v, err := scanSeason(tx.QueryRow(ctx, `SELECT `+seasonColumns+` FROM app.seasons WHERE id=$1 AND deleted_at IS NULL FOR UPDATE`, id))
	if err != nil {
		return v, err
	}
	if v.Revision != revision || v.Status != "open" {
		return v, ErrConflict
	}
	if err := fn(&v); err != nil {
		return v, err
	}

	var invalidDates bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM app.season_weeks WHERE season_id=$1 AND deleted_at IS NULL AND (start_at<$2 OR end_at>$3)) OR EXISTS(SELECT 1 FROM app.mock_interviews WHERE season_id=$1 AND deleted_at IS NULL AND (scheduled_at<$2 OR scheduled_at>$3))`, id, v.StartAt, v.EndAt).Scan(&invalidDates); err != nil {
		return v, err
	}
	if invalidDates {
		return v, ErrConflict
	}
	err = tx.QueryRow(ctx, `UPDATE app.seasons SET slug=$2,name=$3,status=$4::app.season_status,start_at=$5,end_at=$6,location=$7,image_url=$8,resources_url=$9,closed_at=CASE WHEN $4::app.season_status='closed' THEN COALESCE(closed_at,now()) ELSE NULL END,revision=revision+1 WHERE id=$1 AND status='open' AND revision=$10 RETURNING revision`, id, v.Slug, v.Name, v.Status, v.StartAt, v.EndAt, v.Location, v.ImageURL, v.ResourcesURL, revision).Scan(&v.Revision)
	if err != nil {
		return v, noRows(err)
	}
	if err := appendAuditTx(ctx, tx, newAudit(actorID, "season.updated", "season", id, nil, at)); err != nil {
		return v, err
	}
	if err := tx.Commit(ctx); err != nil {
		return v, err
	}
	return v, nil
}

// CloseSeason closes a value.
func (p *Postgres) CloseSeason(ctx context.Context, seasonID string, revision int64, actorID, reason string, at time.Time) (programme.SeasonRecord, error) {
	tx, err := p.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return programme.SeasonRecord{}, err
	}

	defer tx.Rollback(ctx)
	domainSeason, enrollments, err := loadProgrammeSeason(ctx, tx, seasonID)
	if err != nil {
		return programme.SeasonRecord{}, err
	}
	if domainSeason.Revision != revision {
		return programme.SeasonRecord{}, ErrConflict
	}
	closedAt := at.UTC()
	closeEventID := id.New()
	transition, err := programme.Close(domainSeason, actorID, reason, closeEventID, closedAt)
	if err != nil {
		return programme.SeasonRecord{}, ErrConflict
	}
	if _, err := tx.Exec(ctx, `INSERT INTO app.season_close_events(id,season_id,closed_by_user_id,close_reason,closed_at) VALUES($1,$2,$3,$4,$5)`, closeEventID, seasonID, actorID, reason, closedAt); err != nil {
		return programme.SeasonRecord{}, mapPostgresError(err)
	}
	for index, enrollment := range transition.Season.Enrollments {
		if enrollments[index].State != "active" || enrollment.State != "completed" {
			continue
		}
		if _, err := tx.Exec(ctx, `UPDATE app.enrollments SET state='completed',completed_by_close_id=$2,state_changed_at=$3,close_assignment_state=assignment_state,close_activated_at=activated_at,assignment_state='revoked',activated_at=NULL,revision=revision+1 WHERE id=$1 AND revision=$4 AND state='active' AND deleted_at IS NULL`, enrollment.ID, closeEventID, closedAt, enrollments[index].Revision); err != nil {
			return programme.SeasonRecord{}, err
		}
	}

	v, err := scanSeason(tx.QueryRow(ctx, `UPDATE app.seasons SET status='closed',closed_at=$3,revision=revision+1 WHERE id=$1 AND status='open' AND revision=$2 AND deleted_at IS NULL RETURNING `+seasonColumns, seasonID, revision, closedAt))
	if err != nil {
		return programme.SeasonRecord{}, err
	}
	if err := appendAuditTx(ctx, tx, audit.Event{ID: id.New(), ActorID: &actorID, Action: "season.closed", SubjectType: "season", SubjectID: seasonID, Data: map[string]any{"reason": reason, "closeEventId": closeEventID}, OccurredAt: closedAt}); err != nil {
		return programme.SeasonRecord{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return programme.SeasonRecord{}, err
	}
	return v, nil
}

// ReopenSeason reopens a value.
func (p *Postgres) ReopenSeason(ctx context.Context, seasonID string, revision int64, actorID, reason string, at time.Time) (programme.SeasonRecord, error) {
	tx, err := p.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return programme.SeasonRecord{}, err
	}

	defer tx.Rollback(ctx)
	var closeEventID string
	if err := tx.QueryRow(ctx, `SELECT id FROM app.season_close_events WHERE season_id=$1 AND reopened_at IS NULL FOR UPDATE`, seasonID).Scan(&closeEventID); err != nil {
		return programme.SeasonRecord{}, noRows(err)
	}
	domainSeason, enrollments, err := loadProgrammeReopenSeason(ctx, tx, seasonID, closeEventID)
	if err != nil {
		return programme.SeasonRecord{}, err
	}
	if domainSeason.Revision != revision {
		return programme.SeasonRecord{}, ErrConflict
	}

	reopenedAt := at.UTC()
	reopened, err := programme.Reopen(domainSeason, programme.CloseEvent{ID: closeEventID}, programme.SystemAdmin)
	if err != nil {
		return programme.SeasonRecord{}, ErrConflict
	}
	if _, err := tx.Exec(ctx, `UPDATE app.season_close_events SET reopened_by_user_id=$2,reopen_reason=$3,reopened_at=$4 WHERE id=$1 AND reopened_at IS NULL`, closeEventID, actorID, reason, reopenedAt); err != nil {
		return programme.SeasonRecord{}, err
	}
	for index, enrollment := range reopened.Enrollments {
		if enrollments[index].State != "completed" || enrollment.State != "active" {
			continue
		}
		if _, err := tx.Exec(ctx, `UPDATE app.enrollments SET state='active',completed_by_close_id=NULL,state_changed_at=$2::timestamptz,assignment_state=COALESCE(close_assignment_state,'active'::app.assignment_state),activated_at=CASE WHEN COALESCE(close_assignment_state,'active'::app.assignment_state)='active' THEN COALESCE(close_activated_at,$2::timestamptz) ELSE NULL END,close_assignment_state=NULL,close_activated_at=NULL,revision=revision+1 WHERE id=$1 AND revision=$3 AND completed_by_close_id=$4 AND state='completed' AND deleted_at IS NULL`, enrollment.ID, reopenedAt, enrollments[index].Revision, closeEventID); err != nil {
			return programme.SeasonRecord{}, err
		}
	}

	v, err := scanSeason(tx.QueryRow(ctx, `UPDATE app.seasons SET status='open',closed_at=NULL,revision=revision+1 WHERE id=$1 AND status='closed' AND revision=$2 AND deleted_at IS NULL RETURNING `+seasonColumns, seasonID, revision))
	if err != nil {
		return programme.SeasonRecord{}, err
	}
	if err := appendAuditTx(ctx, tx, audit.Event{ID: id.New(), ActorID: &actorID, Action: "season.reopened", SubjectType: "season", SubjectID: seasonID, Data: map[string]any{"reason": reason, "closeEventId": closeEventID}, OccurredAt: reopenedAt}); err != nil {
		return programme.SeasonRecord{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return programme.SeasonRecord{}, err
	}
	return v, nil
}

func scanWeek(row pgx.Row) (programme.WeekRecord, error) {
	var v programme.WeekRecord
	if err := row.Scan(&v.ID, &v.SeasonID, &v.Number, &v.StartAt, &v.EndAt, &v.ResourceURL, &v.Revision); err != nil {
		return v, noRows(err)
	}
	return v, nil
}

// ListWeeks lists matching values.
func (p *Postgres) ListWeeks(ctx context.Context, seasonID, boundary string, limit int, sortBy, direction string) ([]programme.WeekRecord, bool, int64, error) {
	var total int64
	if err := p.Pool.QueryRow(ctx, `SELECT count(*) FROM app.season_weeks WHERE season_id=$1 AND deleted_at IS NULL`, seasonID).Scan(&total); err != nil {
		return nil, false, 0, err
	}

	comparator, order := ">", "ASC"
	if direction == "backward" {
		comparator, order = "<", "DESC"
	}
	key := "w.id"
	boundaryKey := "b.id"
	boundarySelect := "id"
	if sortBy == "number:asc" {
		key = "ROW(w.week_number,w.id)"
		boundaryKey = "ROW(b.week_number,b.id)"
		boundarySelect = "week_number,id"
	}
	orderBy := "w.id " + order
	if sortBy == "number:asc" {
		orderBy = "w.week_number " + order + ",w.id " + order
	}
	query := `WITH boundary AS (
		SELECT ` + boundarySelect + ` FROM app.season_weeks WHERE id=NULLIF($2,'')::uuid AND season_id=$1 AND deleted_at IS NULL
	) SELECT w.id,w.season_id,w.week_number,w.start_at,w.end_at,COALESCE(w.resource_url,''),w.revision
	FROM app.season_weeks w
	WHERE w.season_id=$1 AND w.deleted_at IS NULL
	  AND (NULLIF($2,'')::uuid IS NULL OR EXISTS (SELECT 1 FROM boundary b WHERE ` + key + ` ` + comparator + ` ` + boundaryKey + `))
	ORDER BY ` + orderBy + ` LIMIT $3`
	rows, err := p.Pool.Query(ctx, query, seasonID, boundary, limit+1)
	if err != nil {
		return nil, false, 0, err
	}

	defer rows.Close()
	items := make([]programme.WeekRecord, 0, limit+1)
	for rows.Next() {
		v, scanErr := scanWeek(rows)
		if scanErr != nil {
			return nil, false, 0, scanErr
		}
		items = append(items, v)
	}
	if err := rows.Err(); err != nil {
		return nil, false, 0, err
	}

	items, more := finishPostgresPage(items, limit, direction)
	return items, more, total, nil
}

// CreateWeek creates a value.
func (p *Postgres) CreateWeek(ctx context.Context, v programme.WeekRecord, actorID string, at time.Time) (programme.WeekRecord, error) {
	tx, err := p.Pool.Begin(ctx)
	if err != nil {
		return v, err
	}

	defer tx.Rollback(ctx)
	err = tx.QueryRow(ctx, `INSERT INTO app.season_weeks(id,season_id,week_number,start_at,end_at,resource_url,revision) VALUES($1,$2,$3,$4,$5,$6,$7) RETURNING revision`, v.ID, v.SeasonID, v.Number, v.StartAt, v.EndAt, v.ResourceURL, v.Revision).Scan(&v.Revision)
	if err != nil {
		return v, mapPostgresError(err)
	}
	if err := appendAuditTx(ctx, tx, newAudit(actorID, "week.created", "week", v.ID, nil, at)); err != nil {
		return v, err
	}
	return v, tx.Commit(ctx)
}

// UpdateWeek updates a value.
func (p *Postgres) UpdateWeek(ctx context.Context, seasonID, weekID string, revision int64, candidate programme.WeekRecord, actorID string, at time.Time) (programme.WeekRecord, error) {
	tx, err := p.Pool.Begin(ctx)
	if err != nil {
		return programme.WeekRecord{}, err
	}

	defer tx.Rollback(ctx)
	v, err := scanWeek(tx.QueryRow(ctx, `UPDATE app.season_weeks SET week_number=$4,start_at=$5,end_at=$6,resource_url=$7,revision=revision+1 WHERE id=$1 AND season_id=$2 AND revision=$3 AND deleted_at IS NULL RETURNING id,season_id,week_number,start_at,end_at,resource_url,revision`, weekID, seasonID, revision, candidate.Number, candidate.StartAt, candidate.EndAt, candidate.ResourceURL))
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return v, p.classifyRevision(ctx, tx, `SELECT revision FROM app.season_weeks WHERE id=$1 AND season_id=$2 AND deleted_at IS NULL`, weekID, seasonID)
		}
		return v, mapPostgresError(err)
	}
	if err := appendAuditTx(ctx, tx, newAudit(actorID, "week.updated", "week", weekID, nil, at)); err != nil {
		return v, err
	}
	return v, tx.Commit(ctx)
}

// DeleteWeek deletes a value.
func (p *Postgres) DeleteWeek(ctx context.Context, seasonID, weekID string, revision int64, actorID string, at time.Time) error {
	tx, err := p.Pool.Begin(ctx)
	if err != nil {
		return err
	}

	defer tx.Rollback(ctx)
	tag, err := tx.Exec(ctx, `UPDATE app.season_weeks SET deleted_at=$4,revision=revision+1 WHERE id=$1 AND season_id=$2 AND revision=$3 AND deleted_at IS NULL`, weekID, seasonID, revision, at.UTC())
	if err != nil {
		return mapPostgresError(err)
	}
	if tag.RowsAffected() == 0 {
		return p.classifyRevision(ctx, tx, `SELECT revision FROM app.season_weeks WHERE id=$1 AND season_id=$2 AND deleted_at IS NULL`, weekID, seasonID)
	}
	if err := appendAuditTx(ctx, tx, newAudit(actorID, "week.deleted", "week", weekID, nil, at)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

const enrollmentColumns = `e.id,e.season_id,s.slug,e.user_id,e.role::text,e.student_level::text,e.state::text,e.assignment_state::text,CASE WHEN e.state IN ('kicked','withdrawn') THEN (SELECT reason FROM app.enrollment_removal_events WHERE enrollment_id=e.id ORDER BY occurred_at DESC,id DESC LIMIT 1) END,e.revision`

func scanEnrollment(row pgx.Row) (programme.EnrollmentRecord, error) {
	var v programme.EnrollmentRecord
	if err := row.Scan(&v.ID, &v.SeasonID, &v.SeasonSlug, &v.UserID, &v.Role, &v.StudentLevel, &v.State, &v.AssignmentState, &v.RemovalReason, &v.Revision); err != nil {
		return v, noRows(err)
	}
	return v, nil
}

// ListEnrollments lists matching values.
func (p *Postgres) ListEnrollments(ctx context.Context, seasonID, boundary string, limit int, role, state, sortBy, direction string, includeInactive bool) ([]programme.EnrollmentRecord, bool, int64, error) {
	const filters = `e.season_id=$1 AND e.deleted_at IS NULL
		AND ($2='' OR e.role::text=$2) AND ($3='' OR e.state::text=$3)
		AND ($4 OR e.state='active' OR (e.role='student' AND e.state='completed'))`
	var total int64
	if err := p.Pool.QueryRow(ctx, `SELECT count(*) FROM app.enrollments e WHERE `+filters, seasonID, role, state, includeInactive).Scan(&total); err != nil {
		return nil, false, 0, err
	}

	comparator, order := ">", "ASC"
	if direction == "backward" {
		comparator, order = "<", "DESC"
	}
	key, boundaryKey := "e.id", "b.id"
	boundarySelect := "id"
	if sortBy == "role:asc" {
		key, boundaryKey = "ROW(e.role::text,e.id)", "ROW(b.role::text,b.id)"
		boundarySelect = "role,id"
	}
	orderBy := "e.id " + order
	if sortBy == "role:asc" {
		orderBy = "e.role::text " + order + ",e.id " + order
	}
	query := `WITH boundary AS (
		SELECT ` + boundarySelect + ` FROM app.enrollments
		WHERE id=NULLIF($5,'')::uuid AND season_id=$1 AND deleted_at IS NULL
	) SELECT ` + enrollmentColumns + `
	FROM app.enrollments e JOIN app.seasons s ON s.id=e.season_id
	WHERE ` + filters + `
	  AND (NULLIF($5,'')::uuid IS NULL OR EXISTS (SELECT 1 FROM boundary b WHERE ` + key + ` ` + comparator + ` ` + boundaryKey + `))
	ORDER BY ` + orderBy + ` LIMIT $6`
	rows, err := p.Pool.Query(ctx, query, seasonID, role, state, includeInactive, boundary, limit+1)
	if err != nil {
		return nil, false, 0, err
	}

	defer rows.Close()
	items := make([]programme.EnrollmentRecord, 0, limit+1)
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

	items, more := finishPostgresPage(items, limit, direction)
	return items, more, total, nil
}

// ListEnrollmentsForUser lists matching values.
func (p *Postgres) ListEnrollmentsForUser(ctx context.Context, userID string) ([]programme.EnrollmentRecord, error) {
	return p.listEnrollments(ctx, `e.user_id=$1`, userID)
}

func (p *Postgres) listEnrollments(ctx context.Context, predicate, value string) ([]programme.EnrollmentRecord, error) {
	rows, err := p.Pool.Query(ctx, `SELECT `+enrollmentColumns+` FROM app.enrollments e JOIN app.seasons s ON s.id=e.season_id WHERE `+predicate+` AND e.deleted_at IS NULL ORDER BY e.id`, value)
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

// GetEnrollment retrieves a value.
func (p *Postgres) GetEnrollment(ctx context.Context, enrollmentID string) (programme.EnrollmentRecord, error) {
	return scanEnrollment(p.Pool.QueryRow(ctx, `SELECT `+enrollmentColumns+` FROM app.enrollments e JOIN app.seasons s ON s.id=e.season_id WHERE e.id=$1 AND e.deleted_at IS NULL`, enrollmentID))
}

// CreateEnrollment creates a value.
func (p *Postgres) CreateEnrollment(ctx context.Context, v programme.EnrollmentRecord, actorID string, at time.Time) (programme.EnrollmentRecord, error) {
	tx, err := p.Pool.Begin(ctx)
	if err != nil {
		return v, err
	}

	defer tx.Rollback(ctx)
	studentLevel := "not_applicable"
	if v.Role == "student" {
		studentLevel = "novice"
	}
	assignmentState := "active"
	if v.Role == "coordinator" {
		assignmentState = "pending_mfa"
	}
	err = tx.QueryRow(ctx, `INSERT INTO app.enrollments(id,user_id,season_id,role,student_level,state,assignment_state,revision) VALUES($1,$2,$3,$4,$5,$6,$7,$8) RETURNING revision,assignment_state::text`, v.ID, v.UserID, v.SeasonID, v.Role, studentLevel, v.State, assignmentState, v.Revision).Scan(&v.Revision, &v.AssignmentState)
	if err != nil {
		return v, mapPostgresError(err)
	}

	v.StudentLevel = studentLevel
	if err := appendAuditTx(ctx, tx, newAudit(actorID, "enrollment.created", "enrollment", v.ID, nil, at)); err != nil {
		return v, err
	}
	return v, tx.Commit(ctx)
}

// UpdateEnrollmentDetails updates a value.
func (p *Postgres) UpdateEnrollmentDetails(ctx context.Context, seasonID, enrollmentID string, revision int64, role, studentLevel, actorID string, at time.Time) (programme.EnrollmentRecord, error) {
	tx, err := p.Pool.Begin(ctx)
	if err != nil {
		return programme.EnrollmentRecord{}, err
	}

	defer tx.Rollback(ctx)
	v, err := scanEnrollment(tx.QueryRow(ctx, `UPDATE app.enrollments e SET role=$4::app.season_role,student_level=$5,assignment_state=CASE WHEN $4::app.season_role='coordinator' THEN 'pending_mfa'::app.assignment_state ELSE 'active'::app.assignment_state END,activated_at=CASE WHEN $4::app.season_role='coordinator' THEN NULL ELSE $6::timestamptz END,revision=e.revision+1 FROM app.seasons s WHERE e.id=$1 AND e.season_id=$2 AND e.revision=$3 AND e.state='active' AND e.deleted_at IS NULL AND s.id=e.season_id RETURNING `+enrollmentColumns, enrollmentID, seasonID, revision, role, studentLevel, at.UTC()))
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return v, p.classifyRevision(ctx, tx, `SELECT revision FROM app.enrollments WHERE id=$1 AND season_id=$2 AND state='active' AND deleted_at IS NULL`, enrollmentID, seasonID)
		}
		return v, mapPostgresError(err)
	}
	if role != "student" {
		_, err = tx.Exec(ctx, `UPDATE app.mentorships SET ended_at=$2,revision=revision+1 WHERE student_enrollment_id=$1 AND ended_at IS NULL AND deleted_at IS NULL`, enrollmentID, at.UTC())
	} else {
		_, err = tx.Exec(ctx, `UPDATE app.mentorships SET ended_at=$2,revision=revision+1 WHERE mentor_enrollment_id=$1 AND ended_at IS NULL AND deleted_at IS NULL`, enrollmentID, at.UTC())
	}
	if err != nil {
		return v, err
	}
	if err := appendAuditTx(ctx, tx, newAudit(actorID, "enrollment.updated", "enrollment", enrollmentID, map[string]any{"role": role, "studentLevel": studentLevel}, at)); err != nil {
		return v, err
	}
	return v, tx.Commit(ctx)
}

// UpdateEnrollment updates a value.
func (p *Postgres) UpdateEnrollment(ctx context.Context, enrollmentID string, revision int64, role, state, reason, actorID string, at time.Time) (programme.EnrollmentRecord, error) {
	tx, err := p.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return programme.EnrollmentRecord{}, err
	}

	defer tx.Rollback(ctx)
	v, err := scanEnrollment(tx.QueryRow(ctx, `SELECT `+enrollmentColumns+` FROM app.enrollments e JOIN app.seasons s ON s.id=e.season_id WHERE e.id=$1 AND e.deleted_at IS NULL FOR UPDATE OF e`, enrollmentID))
	if err != nil {
		return v, err
	}
	if v.Revision != revision || v.State != "active" {
		return v, ErrConflict
	}
	if role != "" {
		studentLevel := "not_applicable"
		if role == "student" {
			studentLevel = "novice"
		}
		if err := tx.QueryRow(ctx, `UPDATE app.enrollments SET role=$2::app.season_role,student_level=$3,assignment_state=CASE WHEN $2::app.season_role='coordinator' THEN 'pending_mfa'::app.assignment_state ELSE 'active'::app.assignment_state END,activated_at=CASE WHEN $2::app.season_role='coordinator' THEN NULL ELSE $5::timestamptz END,revision=revision+1 WHERE id=$1 AND revision=$4 AND state='active' RETURNING revision,assignment_state::text`, enrollmentID, role, studentLevel, revision, at.UTC()).Scan(&v.Revision, &v.AssignmentState); err != nil {
			return v, noRows(err)
		}

		v.Role = role
		v.StudentLevel = studentLevel
		if _, err := tx.Exec(ctx, `UPDATE app.mentorships SET ended_at=$2,revision=revision+1 WHERE student_enrollment_id=$1 AND ended_at IS NULL AND deleted_at IS NULL`, v.ID, at.UTC()); err != nil {
			return v, err
		}
		if err := appendAuditTx(ctx, tx, audit.Event{ID: id.New(), ActorID: &actorID, Action: "enrollment.promoted", SubjectType: "enrollment", SubjectID: v.ID, Data: map[string]any{"role": role}, OccurredAt: at.UTC()}); err != nil {
			return v, err
		}
	}
	if state != "" {
		changedAt := at.UTC()
		if err := tx.QueryRow(ctx, `UPDATE app.enrollments SET state=$2,state_changed_at=$3,assignment_state='revoked',activated_at=NULL,revision=revision+1 WHERE id=$1 AND revision=$4 AND state='active' RETURNING revision`, enrollmentID, state, changedAt, revision).Scan(&v.Revision); err != nil {
			return v, noRows(err)
		}

		v.State = state
		if _, err := tx.Exec(ctx, `UPDATE app.mentorships SET ended_at=$2,revision=revision+1 WHERE (student_enrollment_id=$1 OR mentor_enrollment_id=$1) AND ended_at IS NULL AND deleted_at IS NULL`, v.ID, changedAt); err != nil {
			return v, err
		}

		trimmed := strings.TrimSpace(reason)
		v.RemovalReason = &trimmed
		if _, err := tx.Exec(ctx, `INSERT INTO app.enrollment_removal_events(id,enrollment_id,season_id,subject_user_id,actor_user_id,resulting_state,reason,occurred_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, id.New(), v.ID, v.SeasonID, v.UserID, actorID, state, trimmed, changedAt); err != nil {
			return v, err
		}
		if err := appendAuditTx(ctx, tx, audit.Event{ID: id.New(), ActorID: &actorID, Action: "enrollment.removed", SubjectType: "enrollment", SubjectID: v.ID, Data: map[string]any{"reason": trimmed, "state": state}, OccurredAt: changedAt}); err != nil {
			return v, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return v, err
	}
	return v, nil
}

// ListMentorships lists matching values.
func (p *Postgres) ListMentorships(ctx context.Context, seasonID, boundary string, limit int, sortBy, direction, mentorUserID, studentUserID string) ([]programme.MentorshipRecord, bool, int64, error) {
	const base = ` FROM app.mentorships m
		JOIN app.enrollments mentor ON mentor.id=m.mentor_enrollment_id
		JOIN app.enrollments student ON student.id=m.student_enrollment_id`
	const filters = `m.season_id=$1 AND m.ended_at IS NULL AND m.deleted_at IS NULL AND ($4='' OR mentor.user_id=$4) AND ($5='' OR student.user_id=$5)`
	countFilters := strings.NewReplacer("$4", "$2", "$5", "$3").Replace(filters)
	var total int64
	if err := p.Pool.QueryRow(ctx, `SELECT count(*)`+base+` WHERE `+countFilters, seasonID, mentorUserID, studentUserID).Scan(&total); err != nil {
		return nil, false, 0, err
	}

	comparator, order := ">", "ASC"
	if direction == "backward" {
		comparator, order = "<", "DESC"
	}
	key, boundaryKey := "m.id", "b.id"
	boundarySelect := "m.id"
	if sortBy == "student:asc" {
		key, boundaryKey = "ROW(student.user_id,m.id)", "ROW(b.student_user_id,b.id)"
		boundarySelect = "m.id,student.user_id AS student_user_id"
	}
	orderBy := "m.id " + order
	if sortBy == "student:asc" {
		orderBy = "student.user_id " + order + ",m.id " + order
	}
	query := `WITH boundary AS (
		SELECT ` + boundarySelect + base + ` WHERE m.id=$2 AND ` + filters + `
	) SELECT m.id,m.season_id,mentor.user_id,student.user_id,m.revision` + base + `
	WHERE ` + filters + `
	  AND (NULLIF($2,'')::uuid IS NULL OR EXISTS (SELECT 1 FROM boundary b WHERE ` + key + ` ` + comparator + ` ` + boundaryKey + `))
	ORDER BY ` + orderBy + ` LIMIT $3`
	rows, err := p.Pool.Query(ctx, query, seasonID, boundary, limit+1, mentorUserID, studentUserID)
	if err != nil {
		return nil, false, 0, err
	}

	defer rows.Close()
	items := make([]programme.MentorshipRecord, 0, limit+1)
	for rows.Next() {
		var v programme.MentorshipRecord
		if err := rows.Scan(&v.ID, &v.SeasonID, &v.MentorUserID, &v.StudentUserID, &v.Revision); err != nil {
			return nil, false, 0, err
		}

		items = append(items, v)
	}
	if err := rows.Err(); err != nil {
		return nil, false, 0, err
	}

	items, more := finishPostgresPage(items, limit, direction)
	return items, more, total, nil
}

// CreateMentorship creates a value.
func (p *Postgres) CreateMentorship(ctx context.Context, v programme.MentorshipRecord, actorID string, at time.Time) (programme.MentorshipRecord, error) {
	tx, err := p.Pool.Begin(ctx)
	if err != nil {
		return v, err
	}

	defer tx.Rollback(ctx)
	err = tx.QueryRow(ctx, `INSERT INTO app.mentorships(id,season_id,mentor_enrollment_id,student_enrollment_id,revision) VALUES($1,$2,(SELECT id FROM app.enrollments WHERE season_id=$2 AND user_id=$3 AND role IN ('mentor','coordinator') AND state='active' AND deleted_at IS NULL),(SELECT id FROM app.enrollments WHERE season_id=$2 AND user_id=$4 AND role='student' AND state='active' AND deleted_at IS NULL),$5) RETURNING revision`, v.ID, v.SeasonID, v.MentorUserID, v.StudentUserID, v.Revision).Scan(&v.Revision)
	if err != nil {
		return v, mapPostgresError(err)
	}
	if err := appendAuditTx(ctx, tx, newAudit(actorID, "mentorship.created", "mentorship", v.ID, nil, at)); err != nil {
		return v, err
	}
	return v, tx.Commit(ctx)
}

// UpdateMentorship updates a value.
func (p *Postgres) UpdateMentorship(ctx context.Context, seasonID, mentorshipID string, revision int64, mentorUserID, studentUserID, actorID string, at time.Time) (programme.MentorshipRecord, error) {
	tx, err := p.Pool.Begin(ctx)
	if err != nil {
		return programme.MentorshipRecord{}, err
	}

	defer tx.Rollback(ctx)
	var v programme.MentorshipRecord
	err = tx.QueryRow(ctx, `UPDATE app.mentorships SET mentor_enrollment_id=(SELECT id FROM app.enrollments WHERE season_id=$2 AND user_id=$4 AND role IN ('mentor','coordinator') AND state='active' AND deleted_at IS NULL),student_enrollment_id=(SELECT id FROM app.enrollments WHERE season_id=$2 AND user_id=$5 AND role='student' AND state='active' AND deleted_at IS NULL),revision=revision+1 WHERE id=$1 AND season_id=$2 AND revision=$3 AND ended_at IS NULL AND deleted_at IS NULL RETURNING id,season_id,$4::text,$5::text,revision`, mentorshipID, seasonID, revision, mentorUserID, studentUserID).Scan(&v.ID, &v.SeasonID, &v.MentorUserID, &v.StudentUserID, &v.Revision)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return v, p.classifyRevision(ctx, tx, `SELECT revision FROM app.mentorships WHERE id=$1 AND season_id=$2 AND ended_at IS NULL AND deleted_at IS NULL`, mentorshipID, seasonID)
		}
		return v, mapPostgresError(err)
	}
	if err := appendAuditTx(ctx, tx, newAudit(actorID, "mentorship.updated", "mentorship", mentorshipID, nil, at)); err != nil {
		return v, err
	}
	return v, tx.Commit(ctx)
}

// DeleteMentorship deletes a value.
func (p *Postgres) DeleteMentorship(ctx context.Context, seasonID, mentorshipID string, revision int64, actorID string, at time.Time) error {
	tx, err := p.Pool.Begin(ctx)
	if err != nil {
		return err
	}

	defer tx.Rollback(ctx)
	tag, err := tx.Exec(ctx, `UPDATE app.mentorships SET ended_at=$4,revision=revision+1 WHERE id=$1 AND season_id=$2 AND revision=$3 AND ended_at IS NULL AND deleted_at IS NULL`, mentorshipID, seasonID, revision, at.UTC())
	if err != nil {
		return mapPostgresError(err)
	}
	if tag.RowsAffected() == 0 {
		return p.classifyRevision(ctx, tx, `SELECT revision FROM app.mentorships WHERE id=$1 AND season_id=$2 AND ended_at IS NULL AND deleted_at IS NULL`, mentorshipID, seasonID)
	}
	if err := appendAuditTx(ctx, tx, newAudit(actorID, "mentorship.deleted", "mentorship", mentorshipID, nil, at)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// IsMentorAssigned performs the operation.
func (p *Postgres) IsMentorAssigned(ctx context.Context, seasonID, mentorUserID, studentUserID string) (bool, error) {
	var assigned bool
	err := p.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM app.mentorships m JOIN app.enrollments mentor ON mentor.id=m.mentor_enrollment_id JOIN app.enrollments student ON student.id=m.student_enrollment_id WHERE m.season_id=$1 AND mentor.user_id=$2 AND student.user_id=$3 AND m.ended_at IS NULL AND m.deleted_at IS NULL)`, seasonID, mentorUserID, studentUserID).Scan(&assigned)
	return assigned, err
}
