package dal

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/magedmg/RSP-website/backend/internal/programme"
)

func scanWeek(row pgx.Row) (programme.WeekRecord, error) {
	var v programme.WeekRecord
	if err := row.Scan(&v.ID, &v.SeasonID, &v.Number, &v.StartAt, &v.EndAt, &v.ResourceURL, &v.Revision); err != nil {
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

func (p *Store) ListWeeks(ctx context.Context, q WeekQuery) ([]programme.WeekRecord, bool, int64, error) {
	args := pgx.NamedArgs{"seasonID": q.SeasonID}
	var total int64
	if err := p.pool.QueryRow(ctx, `SELECT count(*) FROM app.season_weeks WHERE season_id = @seasonID AND deleted_at IS NULL`, args).Scan(&total); err != nil {
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
	) SELECT w.id,w.season_id,w.week_number,w.start_at,w.end_at,COALESCE(w.resource_url,''),w.revision
	FROM app.season_weeks w
	WHERE w.season_id = @seasonID AND w.deleted_at IS NULL
	  AND (NULLIF(@boundary, '')::uuid IS NULL OR EXISTS (SELECT 1 FROM boundary b WHERE ` + key + ` ` + comparator + ` ` + boundaryKey + `))
	ORDER BY ` + orderBy + ` LIMIT @limit`
	args["boundary"] = q.Boundary
	args["limit"] = q.Limit + 1
	rows, err := p.pool.Query(ctx, query, args)
	if err != nil {
		return nil, false, 0, err
	}

	defer rows.Close()
	items := make([]programme.WeekRecord, 0, q.Limit+1)
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

func (p *Store) CreateWeek(ctx context.Context, v programme.WeekRecord, actorID string, at time.Time) (programme.WeekRecord, error) {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return v, err
	}

	defer tx.Rollback(context.Background())
	err = tx.QueryRow(ctx, `INSERT INTO app.season_weeks(id,season_id,week_number,start_at,end_at,resource_url,revision) VALUES($1,$2,$3,$4,$5,$6,$7) RETURNING revision`, v.ID, v.SeasonID, v.Number, v.StartAt, v.EndAt, v.ResourceURL, v.Revision).Scan(&v.Revision)
	if err != nil {
		return v, mapDatabaseError(err)
	}
	if err := appendAuditTx(ctx, tx, newAudit(actorID, "week.created", "week", v.ID, nil, at)); err != nil {
		return v, err
	}
	return v, tx.Commit(ctx)
}

func (p *Store) UpdateWeek(ctx context.Context, seasonID, weekID string, revision int64, candidate programme.WeekRecord, actorID string, at time.Time) (programme.WeekRecord, error) {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return programme.WeekRecord{}, err
	}

	defer tx.Rollback(context.Background())
	v, err := scanWeek(tx.QueryRow(ctx, `UPDATE app.season_weeks SET week_number=$4,start_at=$5,end_at=$6,resource_url=$7,revision=revision+1 WHERE id=$1 AND season_id=$2 AND revision=$3 AND deleted_at IS NULL RETURNING id,season_id,week_number,start_at,end_at,resource_url,revision`, weekID, seasonID, revision, candidate.Number, candidate.StartAt, candidate.EndAt, candidate.ResourceURL))
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return v, p.classifyRevision(ctx, tx, `SELECT revision FROM app.season_weeks WHERE id=$1 AND season_id=$2 AND deleted_at IS NULL`, weekID, seasonID)
		}
		return v, mapDatabaseError(err)
	}
	if err := appendAuditTx(ctx, tx, newAudit(actorID, "week.updated", "week", weekID, nil, at)); err != nil {
		return v, err
	}
	return v, tx.Commit(ctx)
}

func (p *Store) DeleteWeek(ctx context.Context, seasonID, weekID string, revision int64, actorID string, at time.Time) error {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return err
	}

	defer tx.Rollback(context.Background())
	result, err := tx.Exec(ctx, `UPDATE app.season_weeks SET deleted_at=$4,revision=revision+1 WHERE id=$1 AND season_id=$2 AND revision=$3 AND deleted_at IS NULL`, weekID, seasonID, revision, at.UTC())
	if err != nil {
		return mapDatabaseError(err)
	}
	if result.RowsAffected() == 0 {
		return p.classifyRevision(ctx, tx, `SELECT revision FROM app.season_weeks WHERE id=$1 AND season_id=$2 AND deleted_at IS NULL`, weekID, seasonID)
	}
	if err := appendAuditTx(ctx, tx, newAudit(actorID, "week.deleted", "week", weekID, nil, at)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
