package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/magedmg/RSP-website/backend/internal/leetcode"
	"github.com/magedmg/RSP-website/backend/internal/worker"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		slog.Error("DATABASE_URL is required")
		os.Exit(1)
	}
	conn, err := pgx.Connect(ctx, url)
	if err != nil {
		slog.Error("database connection failed", "error", err)
		os.Exit(1)
	}
	defer conn.Close(context.WithoutCancel(ctx))
	state := postgresState{conn}
	scheduler := worker.Scheduler{Locker: state, State: state, Syncer: leetcode.Client{URL: os.Getenv("LEETCODE_SYNC_URL")}, Retries: 3, Timeout: 2 * time.Minute}
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	run := func() {
		ran, err := scheduler.RunDue(ctx)
		if err != nil {
			slog.Error("scheduled sync failed", "error", err)
		} else if ran {
			slog.Info("scheduled sync completed")
		}
	}
	run()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			run()
		}
	}
}

type postgresState struct{ conn *pgx.Conn }

func (p postgresState) TryLock(ctx context.Context, key int64) (bool, error) {
	var ok bool
	err := p.conn.QueryRow(ctx, "SELECT pg_try_advisory_lock($1)", key).Scan(&ok)
	return ok, err
}
func (p postgresState) Unlock(ctx context.Context, key int64) error {
	_, err := p.conn.Exec(ctx, "SELECT pg_advisory_unlock($1)", key)
	return err
}
func (p postgresState) LastSuccess(ctx context.Context, job string) (*time.Time, error) {
	var at *time.Time
	err := p.conn.QueryRow(ctx, "SELECT max(finished_at) FROM app.leetcode_sync_runs WHERE succeeded").Scan(&at)
	return at, err
}
func (p postgresState) Record(ctx context.Context, run worker.Run) error {
	_, err := p.conn.Exec(ctx, `INSERT INTO app.leetcode_sync_runs(id,trigger_kind,started_at,finished_at,succeeded,fetched_count,changed_count,error_summary) VALUES($1,'schedule',$2,$3,$4,$5,$6,$7)`, run.StartedAt.UTC().Format("20060102T150405.000000000"), run.StartedAt, run.FinishedAt, run.Error == "", run.Report.Fetched, run.Report.Inserted+run.Report.Updated, nullString(run.Error))
	return err
}
func nullString(v string) any {
	if v == "" {
		return nil
	}
	return v
}
