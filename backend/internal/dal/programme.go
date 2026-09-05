package dal

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/magedmg/RSP-website/backend/internal/platform/audit"
	"github.com/magedmg/RSP-website/backend/internal/platform/id"
	"github.com/magedmg/RSP-website/backend/internal/programme"
)

func scanSeason(row rowScanner) (programme.SeasonRecord, error) {
	var v programme.SeasonRecord
	if err := row.Scan(&v.ID, &v.Slug, &v.Name, &v.Status, &v.StartAt, &v.EndAt, &v.Location, &v.ImageURL, &v.ResourcesURL, &v.Revision); err != nil {
		return v, noRows(err)
	}
	return v, nil
}

const seasonColumns = `id,slug,name,status::text,start_at,end_at,location,image_url,resources_url,revision`

func (p *Store) GetSeason(ctx context.Context, id string) (programme.SeasonRecord, error) {
	return scanSeason(p.DB.WithContext(ctx).Raw(`SELECT `+seasonColumns+` FROM app.seasons WHERE id=? AND deleted_at IS NULL`, id).Row())
}

func (p *Store) ListSeasons(ctx context.Context, boundary string, limit int, direction, status string) ([]programme.SeasonRecord, bool, int64, error) {
	const filters = `deleted_at IS NULL AND (@status = '' OR status::text = @status)`
	args := []any{sql.Named("status", status)}
	var total int64
	if err := p.DB.WithContext(ctx).Raw(`SELECT count(*) FROM app.seasons WHERE `+filters, args...).Row().Scan(&total); err != nil {
		return nil, false, 0, err
	}

	comparison, order := `id > NULLIF(@boundary, '')::uuid`, `ASC`
	if direction == "backward" {
		comparison, order = `id < NULLIF(@boundary, '')::uuid`, `DESC`
	}
	if boundary == "" {
		comparison = `TRUE`
	}
	args = append(args, sql.Named("boundary", boundary), sql.Named("limit", limit+1))
	rows, err := p.DB.WithContext(ctx).Raw(`SELECT `+seasonColumns+`
		FROM app.seasons WHERE `+comparison+` AND `+filters+`
		ORDER BY id `+order+` LIMIT @limit`, args...).Rows()
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
	out, more := finishStorePage(out, limit, direction)
	return out, more, total, rows.Err()
}

func (p *Store) CreateSeason(ctx context.Context, v programme.SeasonRecord, actorID string, at time.Time) (programme.SeasonRecord, error) {
	tx, err := begin(ctx, p.DB)
	if err != nil {
		return v, err
	}

	defer rollback(tx)
	err = tx.Raw(`INSERT INTO app.seasons(id,slug,name,status,start_at,end_at,location,image_url,resources_url,revision) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10) RETURNING revision`, v.ID, v.Slug, v.Name, v.Status, v.StartAt, v.EndAt, v.Location, v.ImageURL, v.ResourcesURL, v.Revision).Row().Scan(&v.Revision)
	if err != nil {
		if isUnique(err) {
			return v, ErrDuplicate
		}
		return v, err
	}
	if err := appendAuditTx(ctx, tx, newAudit(actorID, "season.created", "season", v.ID, nil, at)); err != nil {
		return v, err
	}
	return v, commit(tx)
}

func (p *Store) UpdateSeason(ctx context.Context, id string, revision int64, fn func(*programme.SeasonRecord) error, actorID string, at time.Time) (programme.SeasonRecord, error) {
	tx, err := begin(ctx, p.DB)
	if err != nil {
		return programme.SeasonRecord{}, err
	}

	defer rollback(tx)
	v, err := scanSeason(tx.Raw(`SELECT `+seasonColumns+` FROM app.seasons WHERE id=$1 AND deleted_at IS NULL FOR UPDATE`, id).Row())
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
	if err := tx.Raw(`SELECT EXISTS(SELECT 1 FROM app.season_weeks WHERE season_id=$1 AND deleted_at IS NULL AND (start_at<$2 OR end_at>$3)) OR EXISTS(SELECT 1 FROM app.mock_interviews WHERE season_id=$1 AND deleted_at IS NULL AND (scheduled_at<$2 OR scheduled_at>$3))`, id, v.StartAt, v.EndAt).Row().Scan(&invalidDates); err != nil {
		return v, err
	}
	if invalidDates {
		return v, ErrConflict
	}
	err = tx.Raw(`UPDATE app.seasons SET slug=$2,name=$3,status=$4::app.season_status,start_at=$5,end_at=$6,location=$7,image_url=$8,resources_url=$9,closed_at=CASE WHEN $4::app.season_status='closed' THEN COALESCE(closed_at,now()) ELSE NULL END,revision=revision+1 WHERE id=$1 AND status='open' AND revision=$10 RETURNING revision`, id, v.Slug, v.Name, v.Status, v.StartAt, v.EndAt, v.Location, v.ImageURL, v.ResourcesURL, revision).Row().Scan(&v.Revision)
	if err != nil {
		return v, noRows(err)
	}
	if err := appendAuditTx(ctx, tx, newAudit(actorID, "season.updated", "season", id, nil, at)); err != nil {
		return v, err
	}
	if err := commit(tx); err != nil {
		return v, err
	}
	return v, nil
}

// CloseSeason closes a value.
func (p *Store) CloseSeason(ctx context.Context, seasonID string, revision int64, actorID, reason string, at time.Time) (programme.SeasonRecord, error) {
	tx, err := begin(ctx, p.DB)
	if err != nil {
		return programme.SeasonRecord{}, err
	}

	defer rollback(tx)
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
	if err := tx.Exec(`INSERT INTO app.season_close_events(id,season_id,closed_by_user_id,close_reason,closed_at) VALUES($1,$2,$3,$4,$5)`, closeEventID, seasonID, actorID, reason, closedAt).Error; err != nil {
		return programme.SeasonRecord{}, mapStoreError(err)
	}
	for index, enrollment := range transition.Season.Enrollments {
		if enrollments[index].State != "active" || enrollment.State != "completed" {
			continue
		}
		if err := tx.Exec(`UPDATE app.enrollments SET state='completed',completed_by_close_id=$2,state_changed_at=$3,close_assignment_state=assignment_state,close_activated_at=activated_at,assignment_state='revoked',activated_at=NULL,revision=revision+1 WHERE id=$1 AND revision=$4 AND state='active' AND deleted_at IS NULL`, enrollment.ID, closeEventID, closedAt, enrollments[index].Revision).Error; err != nil {
			return programme.SeasonRecord{}, err
		}
	}

	v, err := scanSeason(tx.Raw(`UPDATE app.seasons SET status='closed',closed_at=$3,revision=revision+1 WHERE id=$1 AND status='open' AND revision=$2 AND deleted_at IS NULL RETURNING `+seasonColumns, seasonID, revision, closedAt).Row())
	if err != nil {
		return programme.SeasonRecord{}, err
	}
	if err := appendAuditTx(ctx, tx, audit.Event{ID: id.New(), ActorID: &actorID, Action: "season.closed", SubjectType: "season", SubjectID: seasonID, Data: map[string]any{"reason": reason, "closeEventId": closeEventID}, OccurredAt: closedAt}); err != nil {
		return programme.SeasonRecord{}, err
	}
	if err := commit(tx); err != nil {
		return programme.SeasonRecord{}, err
	}
	return v, nil
}

// ReopenSeason reopens a value.
func (p *Store) ReopenSeason(ctx context.Context, seasonID string, revision int64, actorID, reason string, at time.Time) (programme.SeasonRecord, error) {
	tx, err := begin(ctx, p.DB)
	if err != nil {
		return programme.SeasonRecord{}, err
	}

	defer rollback(tx)
	var closeEventID string
	if err := tx.Raw(`SELECT id FROM app.season_close_events WHERE season_id=$1 AND reopened_at IS NULL FOR UPDATE`, seasonID).Row().Scan(&closeEventID); err != nil {
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
	if err := tx.Exec(`UPDATE app.season_close_events SET reopened_by_user_id=$2,reopen_reason=$3,reopened_at=$4 WHERE id=$1 AND reopened_at IS NULL`, closeEventID, actorID, reason, reopenedAt).Error; err != nil {
		return programme.SeasonRecord{}, err
	}
	for index, enrollment := range reopened.Enrollments {
		if enrollments[index].State != "completed" || enrollment.State != "active" {
			continue
		}
		if err := tx.Exec(`UPDATE app.enrollments SET state='active',completed_by_close_id=NULL,state_changed_at=$2::timestamptz,assignment_state=COALESCE(close_assignment_state,'active'::app.assignment_state),activated_at=CASE WHEN COALESCE(close_assignment_state,'active'::app.assignment_state)='active' THEN COALESCE(close_activated_at,$2::timestamptz) ELSE NULL END,close_assignment_state=NULL,close_activated_at=NULL,revision=revision+1 WHERE id=$1 AND revision=$3 AND completed_by_close_id=$4 AND state='completed' AND deleted_at IS NULL`, enrollment.ID, reopenedAt, enrollments[index].Revision, closeEventID).Error; err != nil {
			return programme.SeasonRecord{}, err
		}
	}

	v, err := scanSeason(tx.Raw(`UPDATE app.seasons SET status='open',closed_at=NULL,revision=revision+1 WHERE id=$1 AND status='closed' AND revision=$2 AND deleted_at IS NULL RETURNING `+seasonColumns, seasonID, revision).Row())
	if err != nil {
		return programme.SeasonRecord{}, err
	}
	if err := appendAuditTx(ctx, tx, audit.Event{ID: id.New(), ActorID: &actorID, Action: "season.reopened", SubjectType: "season", SubjectID: seasonID, Data: map[string]any{"reason": reason, "closeEventId": closeEventID}, OccurredAt: reopenedAt}); err != nil {
		return programme.SeasonRecord{}, err
	}
	if err := commit(tx); err != nil {
		return programme.SeasonRecord{}, err
	}
	return v, nil
}

func scanWeek(row rowScanner) (programme.WeekRecord, error) {
	var v programme.WeekRecord
	if err := row.Scan(&v.ID, &v.SeasonID, &v.Number, &v.StartAt, &v.EndAt, &v.ResourceURL, &v.Revision); err != nil {
		return v, noRows(err)
	}
	return v, nil
}

func (p *Store) ListWeeks(ctx context.Context, seasonID, boundary string, limit int, sortBy, direction string) ([]programme.WeekRecord, bool, int64, error) {
	args := []any{sql.Named("seasonID", seasonID)}
	var total int64
	if err := p.DB.WithContext(ctx).Raw(`SELECT count(*) FROM app.season_weeks WHERE season_id = @seasonID AND deleted_at IS NULL`, args...).Row().Scan(&total); err != nil {
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
		SELECT ` + boundarySelect + ` FROM app.season_weeks WHERE id=NULLIF(@boundary, '')::uuid AND season_id = @seasonID AND deleted_at IS NULL
	) SELECT w.id,w.season_id,w.week_number,w.start_at,w.end_at,COALESCE(w.resource_url,''),w.revision
	FROM app.season_weeks w
	WHERE w.season_id = @seasonID AND w.deleted_at IS NULL
	  AND (NULLIF(@boundary, '')::uuid IS NULL OR EXISTS (SELECT 1 FROM boundary b WHERE ` + key + ` ` + comparator + ` ` + boundaryKey + `))
	ORDER BY ` + orderBy + ` LIMIT @limit`
	args = append(args, sql.Named("boundary", boundary), sql.Named("limit", limit+1))
	rows, err := p.DB.WithContext(ctx).Raw(query, args...).Rows()
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

	items, more := finishStorePage(items, limit, direction)
	return items, more, total, nil
}

func (p *Store) CreateWeek(ctx context.Context, v programme.WeekRecord, actorID string, at time.Time) (programme.WeekRecord, error) {
	tx, err := begin(ctx, p.DB)
	if err != nil {
		return v, err
	}

	defer rollback(tx)
	err = tx.Raw(`INSERT INTO app.season_weeks(id,season_id,week_number,start_at,end_at,resource_url,revision) VALUES($1,$2,$3,$4,$5,$6,$7) RETURNING revision`, v.ID, v.SeasonID, v.Number, v.StartAt, v.EndAt, v.ResourceURL, v.Revision).Row().Scan(&v.Revision)
	if err != nil {
		return v, mapStoreError(err)
	}
	if err := appendAuditTx(ctx, tx, newAudit(actorID, "week.created", "week", v.ID, nil, at)); err != nil {
		return v, err
	}
	return v, commit(tx)
}

func (p *Store) UpdateWeek(ctx context.Context, seasonID, weekID string, revision int64, candidate programme.WeekRecord, actorID string, at time.Time) (programme.WeekRecord, error) {
	tx, err := begin(ctx, p.DB)
	if err != nil {
		return programme.WeekRecord{}, err
	}

	defer rollback(tx)
	v, err := scanWeek(tx.Raw(`UPDATE app.season_weeks SET week_number=$4,start_at=$5,end_at=$6,resource_url=$7,revision=revision+1 WHERE id=$1 AND season_id=$2 AND revision=$3 AND deleted_at IS NULL RETURNING id,season_id,week_number,start_at,end_at,resource_url,revision`, weekID, seasonID, revision, candidate.Number, candidate.StartAt, candidate.EndAt, candidate.ResourceURL).Row())
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return v, p.classifyRevision(ctx, tx, `SELECT revision FROM app.season_weeks WHERE id=$1 AND season_id=$2 AND deleted_at IS NULL`, weekID, seasonID)
		}
		return v, mapStoreError(err)
	}
	if err := appendAuditTx(ctx, tx, newAudit(actorID, "week.updated", "week", weekID, nil, at)); err != nil {
		return v, err
	}
	return v, commit(tx)
}

func (p *Store) DeleteWeek(ctx context.Context, seasonID, weekID string, revision int64, actorID string, at time.Time) error {
	tx, err := begin(ctx, p.DB)
	if err != nil {
		return err
	}

	defer rollback(tx)
	result := tx.Exec(`UPDATE app.season_weeks SET deleted_at=$4,revision=revision+1 WHERE id=$1 AND season_id=$2 AND revision=$3 AND deleted_at IS NULL`, weekID, seasonID, revision, at.UTC())
	err = result.Error
	if err != nil {
		return mapStoreError(err)
	}
	if result.RowsAffected == 0 {
		return p.classifyRevision(ctx, tx, `SELECT revision FROM app.season_weeks WHERE id=$1 AND season_id=$2 AND deleted_at IS NULL`, weekID, seasonID)
	}
	if err := appendAuditTx(ctx, tx, newAudit(actorID, "week.deleted", "week", weekID, nil, at)); err != nil {
		return err
	}
	return commit(tx)
}

const enrollmentColumns = `e.id,e.season_id,s.slug,e.user_id,e.role::text,e.student_level::text,e.state::text,e.assignment_state::text,CASE WHEN e.state IN ('kicked','withdrawn') THEN (SELECT reason FROM app.enrollment_removal_events WHERE enrollment_id=e.id ORDER BY occurred_at DESC,id DESC LIMIT 1) END,e.revision`

func scanEnrollment(row rowScanner) (programme.EnrollmentRecord, error) {
	var v programme.EnrollmentRecord
	if err := row.Scan(&v.ID, &v.SeasonID, &v.SeasonSlug, &v.UserID, &v.Role, &v.StudentLevel, &v.State, &v.AssignmentState, &v.RemovalReason, &v.Revision); err != nil {
		return v, noRows(err)
	}
	return v, nil
}

func (p *Store) ListEnrollments(ctx context.Context, seasonID, boundary string, limit int, role, state, sortBy, direction string, includeInactive bool) ([]programme.EnrollmentRecord, bool, int64, error) {
	const filters = `e.season_id = @seasonID AND e.deleted_at IS NULL
		AND (@role = '' OR e.role::text = @role) AND (@state = '' OR e.state::text = @state)
		AND (@includeInactive OR e.state = 'active' OR (e.role = 'student' AND e.state = 'completed'))`
	args := []any{
		sql.Named("seasonID", seasonID), sql.Named("role", role), sql.Named("state", state),
		sql.Named("includeInactive", includeInactive),
	}
	var total int64
	if err := p.DB.WithContext(ctx).Raw(`SELECT count(*) FROM app.enrollments e WHERE `+filters, args...).Row().Scan(&total); err != nil {
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
		WHERE id=NULLIF(@boundary, '')::uuid AND season_id = @seasonID AND deleted_at IS NULL
	) SELECT ` + enrollmentColumns + `
	FROM app.enrollments e JOIN app.seasons s ON s.id=e.season_id
	WHERE ` + filters + `
	  AND (NULLIF(@boundary, '')::uuid IS NULL OR EXISTS (SELECT 1 FROM boundary b WHERE ` + key + ` ` + comparator + ` ` + boundaryKey + `))
	ORDER BY ` + orderBy + ` LIMIT @limit`
	args = append(args, sql.Named("boundary", boundary), sql.Named("limit", limit+1))
	rows, err := p.DB.WithContext(ctx).Raw(query, args...).Rows()
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

	items, more := finishStorePage(items, limit, direction)
	return items, more, total, nil
}

func (p *Store) ListEnrollmentsForUser(ctx context.Context, userID string) ([]programme.EnrollmentRecord, error) {
	return p.listEnrollments(ctx, `e.user_id=$1`, userID)
}

func (p *Store) listEnrollments(ctx context.Context, predicate, value string) ([]programme.EnrollmentRecord, error) {
	rows, err := p.DB.WithContext(ctx).Raw(`SELECT `+enrollmentColumns+` FROM app.enrollments e JOIN app.seasons s ON s.id=e.season_id WHERE `+predicate+` AND e.deleted_at IS NULL ORDER BY e.id`, value).Rows()
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
	return scanEnrollment(p.DB.WithContext(ctx).Raw(`SELECT `+enrollmentColumns+` FROM app.enrollments e JOIN app.seasons s ON s.id=e.season_id WHERE e.id=$1 AND e.deleted_at IS NULL`, enrollmentID).Row())
}

func (p *Store) CreateEnrollment(ctx context.Context, v programme.EnrollmentRecord, actorID string, at time.Time) (programme.EnrollmentRecord, error) {
	tx, err := begin(ctx, p.DB)
	if err != nil {
		return v, err
	}

	defer rollback(tx)
	studentLevel := "not_applicable"
	if v.Role == "student" {
		studentLevel = "novice"
	}
	assignmentState := "active"
	if v.Role == "coordinator" {
		assignmentState = "pending_mfa"
	}
	err = tx.Raw(`INSERT INTO app.enrollments(id,user_id,season_id,role,student_level,state,assignment_state,revision) VALUES($1,$2,$3,$4,$5,$6,$7,$8) RETURNING revision,assignment_state::text`, v.ID, v.UserID, v.SeasonID, v.Role, studentLevel, v.State, assignmentState, v.Revision).Row().Scan(&v.Revision, &v.AssignmentState)
	if err != nil {
		return v, mapStoreError(err)
	}

	v.StudentLevel = studentLevel
	if err := appendAuditTx(ctx, tx, newAudit(actorID, "enrollment.created", "enrollment", v.ID, nil, at)); err != nil {
		return v, err
	}
	return v, commit(tx)
}

func (p *Store) UpdateEnrollmentDetails(ctx context.Context, seasonID, enrollmentID string, revision int64, role, studentLevel, actorID string, at time.Time) (programme.EnrollmentRecord, error) {
	tx, err := begin(ctx, p.DB)
	if err != nil {
		return programme.EnrollmentRecord{}, err
	}

	defer rollback(tx)
	v, err := scanEnrollment(tx.Raw(`UPDATE app.enrollments e SET role=$4::app.season_role,student_level=$5,assignment_state=CASE WHEN $4::app.season_role='coordinator' THEN 'pending_mfa'::app.assignment_state ELSE 'active'::app.assignment_state END,activated_at=CASE WHEN $4::app.season_role='coordinator' THEN NULL ELSE $6::timestamptz END,revision=e.revision+1 FROM app.seasons s WHERE e.id=$1 AND e.season_id=$2 AND e.revision=$3 AND e.state='active' AND e.deleted_at IS NULL AND s.id=e.season_id RETURNING `+enrollmentColumns, enrollmentID, seasonID, revision, role, studentLevel, at.UTC()).Row())
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return v, p.classifyRevision(ctx, tx, `SELECT revision FROM app.enrollments WHERE id=$1 AND season_id=$2 AND state='active' AND deleted_at IS NULL`, enrollmentID, seasonID)
		}
		return v, mapStoreError(err)
	}
	if role != "student" {
		err = tx.Exec(`UPDATE app.mentorships SET ended_at=$2,revision=revision+1 WHERE student_enrollment_id=$1 AND ended_at IS NULL AND deleted_at IS NULL`, enrollmentID, at.UTC()).Error
	} else {
		err = tx.Exec(`UPDATE app.mentorships SET ended_at=$2,revision=revision+1 WHERE mentor_enrollment_id=$1 AND ended_at IS NULL AND deleted_at IS NULL`, enrollmentID, at.UTC()).Error
	}
	if err != nil {
		return v, err
	}
	if err := appendAuditTx(ctx, tx, newAudit(actorID, "enrollment.updated", "enrollment", enrollmentID, map[string]any{"role": role, "studentLevel": studentLevel}, at)); err != nil {
		return v, err
	}
	return v, commit(tx)
}

func (p *Store) UpdateEnrollment(ctx context.Context, enrollmentID string, revision int64, role, state, reason, actorID string, at time.Time) (programme.EnrollmentRecord, error) {
	tx, err := begin(ctx, p.DB)
	if err != nil {
		return programme.EnrollmentRecord{}, err
	}

	defer rollback(tx)
	v, err := scanEnrollment(tx.Raw(`SELECT `+enrollmentColumns+` FROM app.enrollments e JOIN app.seasons s ON s.id=e.season_id WHERE e.id=$1 AND e.deleted_at IS NULL FOR UPDATE OF e`, enrollmentID).Row())
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
		if err := tx.Raw(`UPDATE app.enrollments SET role=$2::app.season_role,student_level=$3,assignment_state=CASE WHEN $2::app.season_role='coordinator' THEN 'pending_mfa'::app.assignment_state ELSE 'active'::app.assignment_state END,activated_at=CASE WHEN $2::app.season_role='coordinator' THEN NULL ELSE $5::timestamptz END,revision=revision+1 WHERE id=$1 AND revision=$4 AND state='active' RETURNING revision,assignment_state::text`, enrollmentID, role, studentLevel, revision, at.UTC()).Row().Scan(&v.Revision, &v.AssignmentState); err != nil {
			return v, noRows(err)
		}

		v.Role = role
		v.StudentLevel = studentLevel
		if err := tx.Exec(`UPDATE app.mentorships SET ended_at=$2,revision=revision+1 WHERE student_enrollment_id=$1 AND ended_at IS NULL AND deleted_at IS NULL`, v.ID, at.UTC()).Error; err != nil {
			return v, err
		}
		if err := appendAuditTx(ctx, tx, audit.Event{ID: id.New(), ActorID: &actorID, Action: "enrollment.promoted", SubjectType: "enrollment", SubjectID: v.ID, Data: map[string]any{"role": role}, OccurredAt: at.UTC()}); err != nil {
			return v, err
		}
	}
	if state != "" {
		changedAt := at.UTC()
		if err := tx.Raw(`UPDATE app.enrollments SET state=$2,state_changed_at=$3,assignment_state='revoked',activated_at=NULL,revision=revision+1 WHERE id=$1 AND revision=$4 AND state='active' RETURNING revision`, enrollmentID, state, changedAt, revision).Row().Scan(&v.Revision); err != nil {
			return v, noRows(err)
		}

		v.State = state
		if err := tx.Exec(`UPDATE app.mentorships SET ended_at=$2,revision=revision+1 WHERE (student_enrollment_id=$1 OR mentor_enrollment_id=$1) AND ended_at IS NULL AND deleted_at IS NULL`, v.ID, changedAt).Error; err != nil {
			return v, err
		}

		trimmed := strings.TrimSpace(reason)
		v.RemovalReason = &trimmed
		if err := tx.Exec(`INSERT INTO app.enrollment_removal_events(id,enrollment_id,season_id,subject_user_id,actor_user_id,resulting_state,reason,occurred_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, id.New(), v.ID, v.SeasonID, v.UserID, actorID, state, trimmed, changedAt).Error; err != nil {
			return v, err
		}
		if err := appendAuditTx(ctx, tx, audit.Event{ID: id.New(), ActorID: &actorID, Action: "enrollment.removed", SubjectType: "enrollment", SubjectID: v.ID, Data: map[string]any{"reason": trimmed, "state": state}, OccurredAt: changedAt}); err != nil {
			return v, err
		}
	}
	if err := commit(tx); err != nil {
		return v, err
	}
	return v, nil
}

func (p *Store) ListMentorships(ctx context.Context, seasonID, boundary string, limit int, sortBy, direction, mentorUserID, studentUserID string) ([]programme.MentorshipRecord, bool, int64, error) {
	const base = ` FROM app.mentorships m
		JOIN app.enrollments mentor ON mentor.id=m.mentor_enrollment_id
		JOIN app.enrollments student ON student.id=m.student_enrollment_id`
	const filters = `m.season_id = @seasonID AND m.ended_at IS NULL AND m.deleted_at IS NULL
		AND (@mentorUserID = '' OR mentor.user_id = NULLIF(@mentorUserID, '')::uuid)
		AND (@studentUserID = '' OR student.user_id = NULLIF(@studentUserID, '')::uuid)`
	args := []any{
		sql.Named("seasonID", seasonID), sql.Named("mentorUserID", mentorUserID), sql.Named("studentUserID", studentUserID),
	}
	var total int64
	if err := p.DB.WithContext(ctx).Raw(`SELECT count(*)`+base+` WHERE `+filters, args...).Row().Scan(&total); err != nil {
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
		SELECT ` + boundarySelect + base + ` WHERE m.id = NULLIF(@boundary, '')::uuid AND ` + filters + `
	) SELECT m.id,m.season_id,mentor.user_id,student.user_id,m.revision` + base + `
	WHERE ` + filters + `
	  AND (NULLIF(@boundary, '')::uuid IS NULL OR EXISTS (SELECT 1 FROM boundary b WHERE ` + key + ` ` + comparator + ` ` + boundaryKey + `))
	ORDER BY ` + orderBy + ` LIMIT @limit`
	args = append(args, sql.Named("boundary", boundary), sql.Named("limit", limit+1))
	rows, err := p.DB.WithContext(ctx).Raw(query, args...).Rows()
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

	items, more := finishStorePage(items, limit, direction)
	return items, more, total, nil
}

func (p *Store) CreateMentorship(ctx context.Context, v programme.MentorshipRecord, actorID string, at time.Time) (programme.MentorshipRecord, error) {
	tx, err := begin(ctx, p.DB)
	if err != nil {
		return v, err
	}

	defer rollback(tx)
	err = tx.Raw(`INSERT INTO app.mentorships(id,season_id,mentor_enrollment_id,student_enrollment_id,revision) VALUES($1,$2,(SELECT id FROM app.enrollments WHERE season_id=$2 AND user_id=$3 AND role IN ('mentor','coordinator') AND state='active' AND deleted_at IS NULL),(SELECT id FROM app.enrollments WHERE season_id=$2 AND user_id=$4 AND role='student' AND state='active' AND deleted_at IS NULL),$5) RETURNING revision`, v.ID, v.SeasonID, v.MentorUserID, v.StudentUserID, v.Revision).Row().Scan(&v.Revision)
	if err != nil {
		return v, mapStoreError(err)
	}
	if err := appendAuditTx(ctx, tx, newAudit(actorID, "mentorship.created", "mentorship", v.ID, nil, at)); err != nil {
		return v, err
	}
	return v, commit(tx)
}

func (p *Store) UpdateMentorship(ctx context.Context, seasonID, mentorshipID string, revision int64, mentorUserID, studentUserID, actorID string, at time.Time) (programme.MentorshipRecord, error) {
	tx, err := begin(ctx, p.DB)
	if err != nil {
		return programme.MentorshipRecord{}, err
	}

	defer rollback(tx)
	var v programme.MentorshipRecord
	err = tx.Raw(`UPDATE app.mentorships SET mentor_enrollment_id=(SELECT id FROM app.enrollments WHERE season_id=$2 AND user_id=$4 AND role IN ('mentor','coordinator') AND state='active' AND deleted_at IS NULL),student_enrollment_id=(SELECT id FROM app.enrollments WHERE season_id=$2 AND user_id=$5 AND role='student' AND state='active' AND deleted_at IS NULL),revision=revision+1 WHERE id=$1 AND season_id=$2 AND revision=$3 AND ended_at IS NULL AND deleted_at IS NULL RETURNING id,season_id,$4::text,$5::text,revision`, mentorshipID, seasonID, revision, mentorUserID, studentUserID).Row().Scan(&v.ID, &v.SeasonID, &v.MentorUserID, &v.StudentUserID, &v.Revision)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return v, p.classifyRevision(ctx, tx, `SELECT revision FROM app.mentorships WHERE id=$1 AND season_id=$2 AND ended_at IS NULL AND deleted_at IS NULL`, mentorshipID, seasonID)
		}
		return v, mapStoreError(err)
	}
	if err := appendAuditTx(ctx, tx, newAudit(actorID, "mentorship.updated", "mentorship", mentorshipID, nil, at)); err != nil {
		return v, err
	}
	return v, commit(tx)
}

func (p *Store) DeleteMentorship(ctx context.Context, seasonID, mentorshipID string, revision int64, actorID string, at time.Time) error {
	tx, err := begin(ctx, p.DB)
	if err != nil {
		return err
	}

	defer rollback(tx)
	result := tx.Exec(`UPDATE app.mentorships SET ended_at=$4,revision=revision+1 WHERE id=$1 AND season_id=$2 AND revision=$3 AND ended_at IS NULL AND deleted_at IS NULL`, mentorshipID, seasonID, revision, at.UTC())
	err = result.Error
	if err != nil {
		return mapStoreError(err)
	}
	if result.RowsAffected == 0 {
		return p.classifyRevision(ctx, tx, `SELECT revision FROM app.mentorships WHERE id=$1 AND season_id=$2 AND ended_at IS NULL AND deleted_at IS NULL`, mentorshipID, seasonID)
	}
	if err := appendAuditTx(ctx, tx, newAudit(actorID, "mentorship.deleted", "mentorship", mentorshipID, nil, at)); err != nil {
		return err
	}
	return commit(tx)
}

func (p *Store) IsMentorAssigned(ctx context.Context, seasonID, mentorUserID, studentUserID string) (bool, error) {
	var assigned bool
	err := p.DB.WithContext(ctx).Raw(`SELECT EXISTS(SELECT 1 FROM app.mentorships m JOIN app.enrollments mentor ON mentor.id=m.mentor_enrollment_id JOIN app.enrollments student ON student.id=m.student_enrollment_id WHERE m.season_id=$1 AND mentor.user_id=$2 AND student.user_id=$3 AND m.ended_at IS NULL AND m.deleted_at IS NULL)`, seasonID, mentorUserID, studentUserID).Row().Scan(&assigned)
	return assigned, err
}
