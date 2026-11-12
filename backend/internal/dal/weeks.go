package dal

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/Ravis-Study-Program/RSP-website/backend/internal/program"
)

func scanWeek(row pgx.Row) (program.WeekRecord, error) {
	var v program.WeekRecord
	if err := row.Scan(&v.ID, &v.SeasonID, &v.Number, &v.StartAt, &v.EndAt, &v.ResourcesHTML); err != nil {
		return v, noRows(err)
	}
	return v, nil
}

type WeekQuery struct {
	SeasonID  string
	Boundary  string
	Limit     int
	SortBy    string
	Direction string
}

func (p *Store) ListWeeks(ctx context.Context, q WeekQuery) ([]program.WeekRecord, bool, int64, error) {
	args := pgx.NamedArgs{"seasonID": q.SeasonID}
	var total int64
	if err := p.conn().QueryRow(ctx, `SELECT count(*) FROM app.season_weeks WHERE season_id = @seasonID AND deleted_at IS NULL`, args).Scan(&total); err != nil {
		return nil, false, 0, err
	}

	comparator, order := ">", "ASC"
	if q.Direction == "backward" {
		comparator, order = "<", "DESC"
	}
	key := "w.id"
	boundaryKey := "b.id"
	boundarySelect := "id"
	if q.SortBy == "number:asc" {
		key = "ROW(w.week_number,w.id)"
		boundaryKey = "ROW(b.week_number,b.id)"
		boundarySelect = "week_number,id"
	}
	orderBy := "w.id " + order
	if q.SortBy == "number:asc" {
		orderBy = "w.week_number " + order + ",w.id " + order
	}
	query := `WITH boundary AS (
		SELECT ` + boundarySelect + ` FROM app.season_weeks WHERE id=NULLIF(@boundary, '')::uuid AND season_id = @seasonID AND deleted_at IS NULL
	) SELECT w.id,w.season_id,w.week_number,w.start_at,w.end_at,w.resources_html
	FROM app.season_weeks w
	WHERE w.season_id = @seasonID AND w.deleted_at IS NULL
	  AND (NULLIF(@boundary, '')::uuid IS NULL OR EXISTS (SELECT 1 FROM boundary b WHERE ` + key + ` ` + comparator + ` ` + boundaryKey + `))
	ORDER BY ` + orderBy + ` LIMIT @limit`
	args["boundary"] = q.Boundary
	args["limit"] = q.Limit + 1
	rows, err := p.conn().Query(ctx, query, args)
	if err != nil {
		return nil, false, 0, err
	}

	defer rows.Close()
	items := make([]program.WeekRecord, 0, q.Limit+1)
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

	items, more := finishPage(items, q.Limit, q.Direction)
	return items, more, total, nil
}

func (p *Store) CreateWeek(ctx context.Context, v program.WeekRecord, actorID string, at time.Time) (program.WeekRecord, error) {
	tx, err := p.conn().Begin(ctx)
	if err != nil {
		return v, err
	}

	defer tx.Rollback(context.Background())
	if err = lockOpenSeason(ctx, tx, v.SeasonID); err != nil {
		return v, err
	}
	_, err = tx.Exec(ctx, `INSERT INTO app.season_weeks(id,season_id,week_number,start_at,end_at,resources_html) VALUES($1,$2,$3,$4,$5,$6)`, v.ID, v.SeasonID, v.Number, v.StartAt, v.EndAt, v.ResourcesHTML)
	if err != nil {
		return v, mapDatabaseError(err)
	}
	if err := checkWeekOrder(ctx, tx, v.SeasonID); err != nil {
		return v, err
	}
	if err := appendAuditTx(ctx, tx, newAudit(actorID, "week.created", "week", v.ID, nil, at)); err != nil {
		return v, err
	}
	return v, tx.Commit(ctx)
}

type UpdateWeekInput struct {
	SeasonID  string
	WeekID    string
	Week      program.WeekRecord
	ActorID   string
	ChangedAt time.Time
}

func (p *Store) UpdateWeek(ctx context.Context, input UpdateWeekInput) (program.WeekRecord, error) {
	tx, err := p.conn().Begin(ctx)
	if err != nil {
		return program.WeekRecord{}, err
	}

	defer tx.Rollback(context.Background())
	v, err := scanWeek(tx.QueryRow(ctx, `UPDATE app.season_weeks SET week_number=$3,start_at=$4,end_at=$5 WHERE id=$1 AND season_id=$2 AND deleted_at IS NULL RETURNING id,season_id,week_number,start_at,end_at,resources_html`, input.WeekID, input.SeasonID, input.Week.Number, input.Week.StartAt, input.Week.EndAt))
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return v, noRows(err)
		}
		return v, mapDatabaseError(err)
	}
	if err := checkWeekOrder(ctx, tx, input.SeasonID); err != nil {
		return v, err
	}
	if err := appendAuditTx(ctx, tx, newAudit(input.ActorID, "week.updated", "week", input.WeekID, nil, input.ChangedAt)); err != nil {
		return v, err
	}
	return v, tx.Commit(ctx)
}

type DeleteWeekInput struct {
	SeasonID  string
	WeekID    string
	ActorID   string
	ChangedAt time.Time
}

// UpdateWeekResourcesInput replaces one week's resources page.
type UpdateWeekResourcesInput struct {
	SeasonID  string
	WeekID    string
	HTML      string
	ActorID   string
	ChangedAt time.Time
}

// UpdateWeekResources replaces a week's resources page. The caller has
// already sanitized the HTML.
func (p *Store) UpdateWeekResources(ctx context.Context, input UpdateWeekResourcesInput) (program.WeekRecord, error) {
	tx, err := p.conn().Begin(ctx)
	if err != nil {
		return program.WeekRecord{}, err
	}

	defer tx.Rollback(context.Background())
	if err = lockOpenSeason(ctx, tx, input.SeasonID); err != nil {
		return program.WeekRecord{}, err
	}
	v, err := scanWeek(tx.QueryRow(ctx, `UPDATE app.season_weeks SET resources_html=$3 WHERE id=$1 AND season_id=$2 AND deleted_at IS NULL RETURNING id,season_id,week_number,start_at,end_at,resources_html`, input.WeekID, input.SeasonID, input.HTML))
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return v, noRows(err)
		}
		return v, mapDatabaseError(err)
	}
	if err := appendAuditTx(ctx, tx, newAudit(input.ActorID, "week.resources_updated", "week", input.WeekID, nil, input.ChangedAt)); err != nil {
		return v, err
	}
	return v, tx.Commit(ctx)
}

func (p *Store) DeleteWeek(ctx context.Context, input DeleteWeekInput) error {
	tx, err := p.conn().Begin(ctx)
	if err != nil {
		return err
	}

	defer tx.Rollback(context.Background())
	result, err := tx.Exec(ctx, `UPDATE app.season_weeks SET deleted_at=$3 WHERE id=$1 AND season_id=$2 AND deleted_at IS NULL`, input.WeekID, input.SeasonID, input.ChangedAt.UTC())
	if err != nil {
		return mapDatabaseError(err)
	}
	if result.RowsAffected() == 0 {
		return noRows(err)
	}
	if err := appendAuditTx(ctx, tx, newAudit(input.ActorID, "week.deleted", "week", input.WeekID, nil, input.ChangedAt)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// checkWeekOrder refuses a season whose weeks overlap or are out of order:
// by week number, each week must start no earlier than the one before ends.
func checkWeekOrder(ctx context.Context, tx pgx.Tx, seasonID string) error {
	var broken bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM (
		SELECT start_at, lag(end_at) OVER (ORDER BY week_number) AS previous_end
		FROM app.season_weeks WHERE season_id=$1 AND deleted_at IS NULL) w
		WHERE previous_end IS NOT NULL AND start_at < previous_end)`, seasonID).Scan(&broken); err != nil {
		return err
	}
	if broken {
		return ErrConflict
	}
	return nil
}
