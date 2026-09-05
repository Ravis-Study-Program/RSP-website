package dal

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/magedmg/RSP-website/backend/internal/platform/id"
	"github.com/magedmg/RSP-website/backend/internal/worker"
)

// WorkerSession holds the connection used for advisory locks and sync writes.
// Close releases it only after the scheduler has unlocked it.
type WorkerSession struct{ conn *pgxpool.Conn }

func (s *Store) OpenWorkerSession(ctx context.Context) (*WorkerSession, error) {
	conn, err := s.pool.Acquire(ctx)
	if err != nil {
		return nil, err
	}
	return &WorkerSession{conn: conn}, nil
}

func (s *WorkerSession) Close() { s.conn.Release() }

func (s *WorkerSession) PendingManualRunID(ctx context.Context) (string, error) {
	var runID string
	err := s.conn.QueryRow(ctx, `SELECT id FROM app.leetcode_sync_runs
  WHERE trigger_kind='manual' AND finished_at IS NULL ORDER BY started_at,id LIMIT 1`).Scan(&runID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	return runID, err
}

func (s *WorkerSession) TryLock(ctx context.Context, key int64) (bool, error) {
	var locked bool
	err := s.conn.QueryRow(ctx, `SELECT pg_try_advisory_lock($1)`, key).Scan(&locked)
	return locked, err
}
func (s *WorkerSession) Unlock(ctx context.Context, key int64) error {
	_, err := s.conn.Exec(ctx, `SELECT pg_advisory_unlock($1)`, key)
	return err
}
func (s *WorkerSession) LastSuccess(ctx context.Context) (*time.Time, error) {
	var at *time.Time
	err := s.conn.QueryRow(ctx, `SELECT max(finished_at) FROM app.leetcode_sync_runs WHERE succeeded`).Scan(&at)
	return at, err
}
func (s *WorkerSession) LastAttempt(ctx context.Context) (*time.Time, error) {
	var at *time.Time
	err := s.conn.QueryRow(ctx, `SELECT max(started_at) FROM app.leetcode_sync_runs WHERE trigger_kind IN ('schedule','catch_up')`).Scan(&at)
	return at, err
}
func (s *WorkerSession) Record(ctx context.Context, run worker.Run) error {
	if run.ID != "" {
		_, err := s.conn.Exec(ctx, `UPDATE app.leetcode_sync_runs SET finished_at=$2,succeeded=$3,
   fetched_count=$4,changed_count=$5,error_summary=$6 WHERE id=$1`, run.ID, run.FinishedAt.UTC(), run.Error == "", run.Report.Fetched, run.Report.Applied, nullableString(run.Error))
		return err
	}
	_, err := s.conn.Exec(ctx, `INSERT INTO app.leetcode_sync_runs(id,trigger_kind,started_at,finished_at,succeeded,fetched_count,changed_count,error_summary)
  VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, id.New(), run.TriggerKind, run.StartedAt.UTC(), run.FinishedAt.UTC(), run.Error == "", run.Report.Fetched, run.Report.Applied, nullableString(run.Error))
	return err
}
func nullableString(value string) any {
	if value == "" {
		return nil
	}
	return value
}
