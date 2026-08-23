package postgres

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/magedmg/RSP-website/backend/internal/platform/id"
	"github.com/magedmg/RSP-website/backend/internal/worker"
)

// WorkerState persists scheduler locks and run state in PostgreSQL.
type WorkerState struct {
	Conn        *pgx.Conn
	ManualRunID string
}

// RunPendingManual runs the oldest unfinished manual sync, if one exists.
func (p *WorkerState) RunPendingManual(ctx context.Context, scheduler worker.Scheduler) (bool, error) {
	var runID string
	err := p.Conn.QueryRow(ctx, `SELECT id FROM app.leetcode_sync_runs WHERE trigger_kind='manual' AND finished_at IS NULL ORDER BY started_at,id LIMIT 1`).Scan(&runID)
	if err == pgx.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	p.ManualRunID = runID
	defer func() { p.ManualRunID = "" }()
	return true, scheduler.Run(ctx)
}

// TryLock obtains the PostgreSQL advisory lock for a job.
func (p *WorkerState) TryLock(ctx context.Context, key int64) (bool, error) {
	var ok bool
	err := p.Conn.QueryRow(ctx, "SELECT pg_try_advisory_lock($1)", key).Scan(&ok)
	return ok, err
}

// Unlock releases the PostgreSQL advisory lock for a job.
func (p *WorkerState) Unlock(ctx context.Context, key int64) error {
	_, err := p.Conn.Exec(ctx, "SELECT pg_advisory_unlock($1)", key)
	return err
}

// LastSuccess returns the last successful sync time.
func (p *WorkerState) LastSuccess(ctx context.Context, _ string) (*time.Time, error) {
	var at *time.Time
	err := p.Conn.QueryRow(ctx, "SELECT max(finished_at) FROM app.leetcode_sync_runs WHERE succeeded").Scan(&at)
	return at, err
}

// LastAttempt returns the last scheduled or catch-up sync time.
func (p *WorkerState) LastAttempt(ctx context.Context, _ string) (*time.Time, error) {
	var at *time.Time
	err := p.Conn.QueryRow(ctx, "SELECT max(finished_at) FROM app.leetcode_sync_runs WHERE trigger_kind IN ('schedule','catch_up')").Scan(&at)
	return at, err
}

// Record persists a completed scheduler run.
func (p *WorkerState) Record(ctx context.Context, run worker.Run) error {
	if p.ManualRunID != "" {
		_, err := p.Conn.Exec(ctx, `UPDATE app.leetcode_sync_runs SET finished_at=$2,succeeded=$3,fetched_count=$4,changed_count=$5,error_summary=$6 WHERE id=$1 AND finished_at IS NULL`, p.ManualRunID, run.FinishedAt, run.Error == "", run.Report.Fetched, run.Report.Inserted+run.Report.Updated, nullString(run.Error))
		return err
	}

	trigger := run.TriggerKind
	if trigger != "catch_up" {
		trigger = "schedule"
	}
	_, err := p.Conn.Exec(ctx, `INSERT INTO app.leetcode_sync_runs(id,trigger_kind,started_at,finished_at,succeeded,fetched_count,changed_count,error_summary) VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, id.New(), trigger, run.StartedAt, run.FinishedAt, run.Error == "", run.Report.Fetched, run.Report.Inserted+run.Report.Updated, nullString(run.Error))
	return err
}

func nullString(v string) any {
	if v == "" {
		return nil
	}
	return v
}
