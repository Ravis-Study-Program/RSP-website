package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/magedmg/RSP-website/backend/internal/platform/dbgen"
	"github.com/magedmg/RSP-website/backend/internal/platform/observability"
	"github.com/magedmg/RSP-website/backend/internal/platform/repository"
)

var (
	ErrNotFound  = repository.ErrNotFound
	ErrConflict  = repository.ErrConflict
	ErrDuplicate = repository.ErrDuplicate
)

// Postgres represents a backend data structure.
type Postgres struct {
	Pool    *pgxpool.Pool
	Queries *dbgen.Queries
}

func finishPostgresPage[T any](items []T, limit int, direction string) ([]T, bool) {
	more := len(items) > limit
	if more {
		items = items[:limit]
	}
	if direction == "backward" {
		for left, right := 0, len(items)-1; left < right; left, right = left+1, right-1 {
			items[left], items[right] = items[right], items[left]
		}
	}
	return items, more
}

// Open opens a connection.
func Open(ctx context.Context, url string) (*Postgres, error) {
	config, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, err
	}

	config.ConnConfig.RuntimeParams["timezone"] = "UTC"
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return &Postgres{Pool: pool, Queries: dbgen.New(pool)}, nil
}

// Close closes a value.
func (p *Postgres) Close() { p.Pool.Close() }

// Ping performs the operation.
func (p *Postgres) Ping(ctx context.Context) error { return p.Pool.Ping(ctx) }

// ObservabilitySnapshot performs the operation.
func (p *Postgres) ObservabilitySnapshot(ctx context.Context) (observability.Snapshot, error) {
	pool := p.Pool.Stat()
	snapshot := observability.Snapshot{
		DBPoolAcquiredConnections: pool.AcquiredConns(),
		DBPoolIdleConnections:     pool.IdleConns(),
		WorkerRuns: map[string]uint64{
			"success":         0,
			"partial_failure": 0,
			"failure":         0,
		},
		MigrationState: "none",
	}
	rows, err := p.Pool.Query(ctx, `
SELECT CASE
         WHEN succeeded THEN 'success'
         WHEN error_summary = 'partial LeetCode sync failure' THEN 'partial_failure'
         ELSE 'failure'
       END AS result,
       count(*)
FROM app.leetcode_sync_runs
WHERE finished_at IS NOT NULL
GROUP BY result`)
	if err != nil {
		return observability.Snapshot{}, err
	}

	for rows.Next() {
		var result string
		var count uint64
		if err := rows.Scan(&result, &count); err != nil {
			rows.Close()
			return observability.Snapshot{}, err
		}
		if _, bounded := snapshot.WorkerRuns[result]; bounded {
			snapshot.WorkerRuns[result] = count
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return observability.Snapshot{}, err
	}

	rows.Close()
	err = p.Pool.QueryRow(ctx, `
SELECT state::text
FROM migration.runs
ORDER BY created_at DESC, id DESC
LIMIT 1`).Scan(&snapshot.MigrationState)
	if errors.Is(err, pgx.ErrNoRows) {
		snapshot.MigrationState = "none"
		return snapshot, nil
	}
	if err != nil {
		return observability.Snapshot{}, err
	}
	return snapshot, nil
}

func noRows(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	return err
}

func (p *Postgres) classifyRevision(ctx context.Context, db rowQuerier, query string, args ...any) error {
	var revision int64
	if err := db.QueryRow(ctx, query, args...).Scan(&revision); err != nil {
		return noRows(err)
	}
	return ErrConflict
}

func mapPostgresError(err error) error {
	if isUnique(err) {
		return ErrDuplicate
	}

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && (pgErr.Code == "23503" || pgErr.Code == "23514" || pgErr.Code == "23502") {
		return ErrConflict
	}
	return noRows(err)
}

func isUnique(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
