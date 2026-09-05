// Package dal owns PostgreSQL queries and transactional writes for the application.
package dal

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/magedmg/RSP-website/backend/internal/platform/observability"
)

// Store uses one connection pool. Call Close after all users of the store stop.
type Store struct{ pool *pgxpool.Pool }

// New uses the supplied pool, including its connection settings and tracer.
func New(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

func Open(ctx context.Context, databaseURL string) (*Store, error) {
	config, err := pgxpool.ParseConfig(databaseURL)
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
	return New(pool), nil
}

func (s *Store) Close()                         { s.pool.Close() }
func (s *Store) Ping(ctx context.Context) error { return s.pool.Ping(ctx) }

// queryer lets read and audit helpers use either the pool or an existing transaction.
type queryer interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
}

func (s *Store) ObservabilitySnapshot(ctx context.Context) (observability.Snapshot, error) {
	stats := s.pool.Stat()
	snapshot := observability.Snapshot{
		DBPoolAcquiredConnections: stats.AcquiredConns(), DBPoolIdleConnections: stats.IdleConns(),
		WorkerRuns: map[string]uint64{"success": 0, "partial_failure": 0, "failure": 0}, MigrationState: "none",
	}
	rows, err := s.pool.Query(ctx, `SELECT CASE WHEN succeeded THEN 'success'
  WHEN error_summary IS NOT NULL THEN 'failure' ELSE 'partial_failure' END AS result, count(*)
  FROM app.leetcode_sync_runs WHERE finished_at IS NOT NULL GROUP BY result`)
	if err != nil {
		return observability.Snapshot{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var result string
		var count uint64
		if err := rows.Scan(&result, &count); err != nil {
			return observability.Snapshot{}, err
		}
		snapshot.WorkerRuns[result] = count
	}
	if err := rows.Err(); err != nil {
		return observability.Snapshot{}, err
	}
	err = s.pool.QueryRow(ctx, `SELECT state::text FROM migration.runs ORDER BY started_at DESC,id DESC LIMIT 1`).Scan(&snapshot.MigrationState)
	if err != nil && err != pgx.ErrNoRows {
		return observability.Snapshot{}, err
	}
	return snapshot, nil
}
