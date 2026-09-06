package dal

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/magedmg/RSP-website/backend/internal/platform/audit"
	"github.com/magedmg/RSP-website/backend/internal/platform/id"
	"github.com/magedmg/RSP-website/backend/internal/programme"
)

func scanSeason(row pgx.Row) (programme.SeasonRecord, error) {
	var v programme.SeasonRecord
	if err := row.Scan(&v.ID, &v.Slug, &v.Name, &v.Status, &v.StartAt, &v.EndAt, &v.Location, &v.ImageURL, &v.ResourcesURL); err != nil {
		return v, noRows(err)
	}
	return v, nil
}

const seasonColumns = `id,slug,name,status::text,start_at,end_at,location,image_url,resources_url`

func (p *Store) GetSeason(ctx context.Context, id string) (programme.SeasonRecord, error) {
	return scanSeason(p.pool.QueryRow(ctx, `SELECT `+seasonColumns+` FROM app.seasons WHERE id=$1 AND deleted_at IS NULL`, id))
}

type SeasonQuery struct {
	Boundary  string
	Limit     int
	Direction string
	Status    string
}

func (p *Store) ListSeasons(ctx context.Context, q SeasonQuery) ([]programme.SeasonRecord, bool, int64, error) {
	const filters = `deleted_at IS NULL AND (@status = '' OR status::text = @status)`
	args := pgx.NamedArgs{"status": q.Status}
	var total int64
	if err := p.pool.QueryRow(ctx, `SELECT count(*) FROM app.seasons WHERE `+filters, args).Scan(&total); err != nil {
		return nil, false, 0, err
	}

	comparison, order := `id > NULLIF(@boundary, '')::uuid`, `ASC`
	if q.Direction == "backward" {
		comparison, order = `id < NULLIF(@boundary, '')::uuid`, `DESC`
	}
	if q.Boundary == "" {
		comparison = `TRUE`
	}
	args["boundary"] = q.Boundary
	args["limit"] = q.Limit + 1
	rows, err := p.pool.Query(ctx, `SELECT `+seasonColumns+`
		FROM app.seasons WHERE `+comparison+` AND `+filters+`
		ORDER BY id `+order+` LIMIT @limit`, args)
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
	out, more := finishPage(out, q.Limit, q.Direction)
	return out, more, total, rows.Err()
}

func (p *Store) CreateSeason(ctx context.Context, v programme.SeasonRecord, actorID string, at time.Time) (programme.SeasonRecord, error) {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return v, err
	}

	defer tx.Rollback(context.Background())
	_, err = tx.Exec(ctx, `INSERT INTO app.seasons(id,slug,name,status,start_at,end_at,location,image_url,resources_url) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)`, v.ID, v.Slug, v.Name, v.Status, v.StartAt, v.EndAt, v.Location, v.ImageURL, v.ResourcesURL)
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

type UpdateSeasonDefinitionInput struct {
	SeasonID     string
	ActorID      string
	ChangedAt    time.Time
	Name         string
	Slug         string
	Location     string
	ImageURL     string
	ResourcesURL string
	StartAt      time.Time
	EndAt        time.Time
}

func (p *Store) UpdateSeasonDefinition(ctx context.Context, input UpdateSeasonDefinitionInput) (programme.SeasonRecord, error) {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return programme.SeasonRecord{}, err
	}

	defer tx.Rollback(context.Background())
	v, err := scanSeason(tx.QueryRow(ctx, `SELECT `+seasonColumns+` FROM app.seasons WHERE id=$1 AND deleted_at IS NULL FOR UPDATE`, input.SeasonID))
	if err != nil {
		return v, err
	}
	if v.Status != "open" {
		return v, ErrConflict
	}
	v.Name = input.Name
	v.Slug = input.Slug
	v.Location = input.Location
	v.ImageURL = input.ImageURL
	v.ResourcesURL = input.ResourcesURL
	v.StartAt = input.StartAt
	v.EndAt = input.EndAt

	var invalidDates bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM app.season_weeks WHERE season_id=$1 AND deleted_at IS NULL AND (start_at<$2 OR end_at>$3))`, input.SeasonID, v.StartAt, v.EndAt).Scan(&invalidDates); err != nil {
		return v, err
	}
	if invalidDates {
		return v, ErrConflict
	}
	err = tx.QueryRow(ctx, `UPDATE app.seasons SET slug=$2,name=$3,status=$4::app.season_status,start_at=$5,end_at=$6,location=$7,image_url=$8,resources_url=$9,closed_at=CASE WHEN $4::app.season_status='closed' THEN COALESCE(closed_at,now()) ELSE NULL END WHERE id=$1 AND status='open' AND deleted_at IS NULL RETURNING `+seasonColumns, input.SeasonID, v.Slug, v.Name, v.Status, v.StartAt, v.EndAt, v.Location, v.ImageURL, v.ResourcesURL).Scan(&v.ID, &v.Slug, &v.Name, &v.Status, &v.StartAt, &v.EndAt, &v.Location, &v.ImageURL, &v.ResourcesURL)
	if err != nil {
		return v, noRows(err)
	}
	if err := appendAuditTx(ctx, tx, newAudit(input.ActorID, "season.updated", "season", input.SeasonID, nil, input.ChangedAt)); err != nil {
		return v, err
	}
	if err := tx.Commit(ctx); err != nil {
		return v, err
	}
	return v, nil
}

type UpdateSeasonResourcesInput struct {
	SeasonID     string
	ActorID      string
	ChangedAt    time.Time
	ResourcesURL string
}

func (p *Store) UpdateSeasonResources(ctx context.Context, input UpdateSeasonResourcesInput) (programme.SeasonRecord, error) {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return programme.SeasonRecord{}, err
	}

	defer tx.Rollback(context.Background())
	v, err := scanSeason(tx.QueryRow(ctx, `SELECT `+seasonColumns+` FROM app.seasons WHERE id=$1 AND deleted_at IS NULL FOR UPDATE`, input.SeasonID))
	if err != nil {
		return v, err
	}
	if v.Status != "open" {
		return v, ErrConflict
	}
	v.ResourcesURL = input.ResourcesURL

	var invalidDates bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM app.season_weeks WHERE season_id=$1 AND deleted_at IS NULL AND (start_at<$2 OR end_at>$3))`, input.SeasonID, v.StartAt, v.EndAt).Scan(&invalidDates); err != nil {
		return v, err
	}
	if invalidDates {
		return v, ErrConflict
	}
	err = tx.QueryRow(ctx, `UPDATE app.seasons SET slug=$2,name=$3,status=$4::app.season_status,start_at=$5,end_at=$6,location=$7,image_url=$8,resources_url=$9,closed_at=CASE WHEN $4::app.season_status='closed' THEN COALESCE(closed_at,now()) ELSE NULL END WHERE id=$1 AND status='open' AND deleted_at IS NULL RETURNING `+seasonColumns, input.SeasonID, v.Slug, v.Name, v.Status, v.StartAt, v.EndAt, v.Location, v.ImageURL, v.ResourcesURL).Scan(&v.ID, &v.Slug, &v.Name, &v.Status, &v.StartAt, &v.EndAt, &v.Location, &v.ImageURL, &v.ResourcesURL)
	if err != nil {
		return v, noRows(err)
	}
	if err := appendAuditTx(ctx, tx, newAudit(input.ActorID, "season.updated", "season", input.SeasonID, nil, input.ChangedAt)); err != nil {
		return v, err
	}
	if err := tx.Commit(ctx); err != nil {
		return v, err
	}
	return v, nil
}

// CloseSeason closes a value.
type CloseSeasonInput struct {
	SeasonID  string
	ActorID   string
	Reason    string
	ChangedAt time.Time
}

func (p *Store) CloseSeason(ctx context.Context, input CloseSeasonInput) (programme.SeasonRecord, error) {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return programme.SeasonRecord{}, err
	}

	defer tx.Rollback(context.Background())
	domainSeason, enrollments, err := loadProgrammeSeason(ctx, tx, input.SeasonID)
	if err != nil {
		return programme.SeasonRecord{}, err
	}
	closedAt := input.ChangedAt.UTC()
	closeEventID := id.New()
	transition, err := programme.Close(domainSeason, input.ActorID, input.Reason, closeEventID, closedAt)
	if err != nil {
		return programme.SeasonRecord{}, ErrConflict
	}
	if _, err := tx.Exec(ctx, `INSERT INTO app.season_close_events(id,season_id,closed_by_user_id,close_reason,closed_at) VALUES($1,$2,$3,$4,$5)`, closeEventID, input.SeasonID, input.ActorID, input.Reason, closedAt); err != nil {
		return programme.SeasonRecord{}, mapDatabaseError(err)
	}
	for index, enrollment := range transition.Season.Enrollments {
		if enrollments[index].State != "active" || enrollment.State != "completed" {
			continue
		}
		if _, err := tx.Exec(ctx, `UPDATE app.enrollments SET state='completed',completed_by_close_id=$2,state_changed_at=$3,close_assignment_state=assignment_state,close_activated_at=activated_at,assignment_state='revoked',activated_at=NULL WHERE id=$1 AND state='active' AND deleted_at IS NULL`, enrollment.ID, closeEventID, closedAt); err != nil {
			return programme.SeasonRecord{}, err
		}
	}

	v, err := scanSeason(tx.QueryRow(ctx, `UPDATE app.seasons SET status='closed',closed_at=$2 WHERE id=$1 AND status='open' AND deleted_at IS NULL RETURNING `+seasonColumns, input.SeasonID, closedAt))
	if err != nil {
		return programme.SeasonRecord{}, err
	}
	if err := appendAuditTx(ctx, tx, audit.Event{
		ID:          id.New(),
		ActorID:     &input.ActorID,
		Action:      "season.closed",
		SubjectType: "season",
		SubjectID:   input.SeasonID,
		Data:        map[string]any{"reason": input.Reason, "closeEventId": closeEventID},
		OccurredAt:  closedAt,
	}); err != nil {
		return programme.SeasonRecord{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return programme.SeasonRecord{}, err
	}
	return v, nil
}

// ReopenSeason reopens a value.
type ReopenSeasonInput struct {
	SeasonID  string
	ActorID   string
	Reason    string
	ChangedAt time.Time
}

func (p *Store) ReopenSeason(ctx context.Context, input ReopenSeasonInput) (programme.SeasonRecord, error) {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return programme.SeasonRecord{}, err
	}

	defer tx.Rollback(context.Background())
	var closeEventID string
	if err := tx.QueryRow(ctx, `SELECT id FROM app.season_close_events WHERE season_id=$1 AND reopened_at IS NULL FOR UPDATE`, input.SeasonID).Scan(&closeEventID); err != nil {
		return programme.SeasonRecord{}, noRows(err)
	}
	domainSeason, enrollments, err := loadProgrammeReopenSeason(ctx, tx, input.SeasonID, closeEventID)
	if err != nil {
		return programme.SeasonRecord{}, err
	}

	reopenedAt := input.ChangedAt.UTC()
	reopened := programme.Reopen(domainSeason, programme.CloseEvent{ID: closeEventID})
	if _, err := tx.Exec(ctx, `UPDATE app.season_close_events SET reopened_by_user_id=$2,reopen_reason=$3,reopened_at=$4 WHERE id=$1 AND reopened_at IS NULL`, closeEventID, input.ActorID, input.Reason, reopenedAt); err != nil {
		return programme.SeasonRecord{}, err
	}
	for index, enrollment := range reopened.Enrollments {
		if enrollments[index].State != "completed" || enrollment.State != "active" {
			continue
		}
		if _, err := tx.Exec(ctx, `UPDATE app.enrollments SET state='active',completed_by_close_id=NULL,state_changed_at=$2::timestamptz,assignment_state=COALESCE(close_assignment_state,'active'::app.assignment_state),activated_at=CASE WHEN COALESCE(close_assignment_state,'active'::app.assignment_state)='active' THEN COALESCE(close_activated_at,$2::timestamptz) ELSE NULL END,close_assignment_state=NULL,close_activated_at=NULL WHERE id=$1 AND completed_by_close_id=$3 AND state='completed' AND deleted_at IS NULL`, enrollment.ID, reopenedAt, closeEventID); err != nil {
			return programme.SeasonRecord{}, err
		}
	}

	v, err := scanSeason(tx.QueryRow(ctx, `UPDATE app.seasons SET status='open',closed_at=NULL WHERE id=$1 AND status='closed' AND deleted_at IS NULL RETURNING `+seasonColumns, input.SeasonID))
	if err != nil {
		return programme.SeasonRecord{}, err
	}
	if err := appendAuditTx(ctx, tx, audit.Event{
		ID:          id.New(),
		ActorID:     &input.ActorID,
		Action:      "season.reopened",
		SubjectType: "season",
		SubjectID:   input.SeasonID,
		Data:        map[string]any{"reason": input.Reason, "closeEventId": closeEventID},
		OccurredAt:  reopenedAt,
	}); err != nil {
		return programme.SeasonRecord{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return programme.SeasonRecord{}, err
	}
	return v, nil
}
