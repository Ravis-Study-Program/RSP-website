package main

import (
	"context"
	"errors"
	"log/slog"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/magedmg/RSP-website/backend/internal/leetcode"
	"github.com/magedmg/RSP-website/backend/internal/platform/id"
	"github.com/magedmg/RSP-website/backend/internal/worker"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		logger.Error("DATABASE_URL is required")
		os.Exit(1)
	}

	syncURL, err := resolvedSyncURL(os.Getenv("LEETCODE_SYNC_URL"), os.Getenv("APP_ENV"))
	if err != nil {
		logger.Error("invalid worker configuration", "error", err)
		os.Exit(1)
	}

	conn, err := pgx.Connect(ctx, url)
	if err != nil {
		logger.Error("database connection failed", "error", err)
		os.Exit(1)
	}

	defer conn.Close(context.WithoutCancel(ctx))
	state := &postgresState{conn: conn}
	scheduler := worker.Scheduler{Locker: state, State: state, Syncer: leetcode.Client{URL: syncURL, Sink: leetcode.PostgresSink{DB: conn}}, Retries: 3, Timeout: 2 * time.Minute}
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	run := func() {
		ran, err := state.runPendingManual(ctx, scheduler)
		if err == nil && !ran {
			ran, err = scheduler.RunDue(ctx)
		}
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

func resolvedSyncURL(raw, appEnvironment string) (string, error) {
	if strings.TrimSpace(raw) == "" {
		raw = leetcode.DefaultURL
	}

	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" || parsed.User != nil || parsed.Fragment != "" || parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", errors.New("LEETCODE_SYNC_URL must be an absolute HTTP or HTTPS URL without credentials or a fragment")
	}

	if appEnvironment == "production" && parsed.Scheme != "https" {
		return "", errors.New("LEETCODE_SYNC_URL must use HTTPS in production")
	}

	return parsed.String(), nil
}

type postgresState struct {
	conn        *pgx.Conn
	manualRunID string
}

func (p *postgresState) runPendingManual(ctx context.Context, scheduler worker.Scheduler) (bool, error) {
	var runID string
	err := p.conn.QueryRow(ctx, `SELECT id FROM app.leetcode_sync_runs WHERE trigger_kind='manual' AND finished_at IS NULL ORDER BY started_at,id LIMIT 1`).Scan(&runID)
	if err == pgx.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	p.manualRunID = runID
	defer func() { p.manualRunID = "" }()
	return true, scheduler.Run(ctx)
}

func (p *postgresState) TryLock(ctx context.Context, key int64) (bool, error) {
	var ok bool
	err := p.conn.QueryRow(ctx, "SELECT pg_try_advisory_lock($1)", key).Scan(&ok)
	return ok, err
}

func (p *postgresState) Unlock(ctx context.Context, key int64) error {
	_, err := p.conn.Exec(ctx, "SELECT pg_advisory_unlock($1)", key)
	return err
}

func (p *postgresState) LastSuccess(ctx context.Context, job string) (*time.Time, error) {
	var at *time.Time
	err := p.conn.QueryRow(ctx, "SELECT max(finished_at) FROM app.leetcode_sync_runs WHERE succeeded").Scan(&at)
	return at, err
}

func (p *postgresState) LastAttempt(ctx context.Context, job string) (*time.Time, error) {
	var at *time.Time
	err := p.conn.QueryRow(ctx, "SELECT max(finished_at) FROM app.leetcode_sync_runs WHERE trigger_kind IN ('schedule','catch_up')").Scan(&at)
	return at, err
}

func (p *postgresState) Record(ctx context.Context, run worker.Run) error {
	if p.manualRunID != "" {
		_, err := p.conn.Exec(ctx, `UPDATE app.leetcode_sync_runs SET finished_at=$2,succeeded=$3,fetched_count=$4,changed_count=$5,error_summary=$6 WHERE id=$1 AND finished_at IS NULL`, p.manualRunID, run.FinishedAt, run.Error == "", run.Report.Fetched, run.Report.Inserted+run.Report.Updated, nullString(run.Error))
		return err
	}

	trigger := run.TriggerKind
	if trigger != "catch_up" {
		trigger = "schedule"
	}
	_, err := p.conn.Exec(ctx, `INSERT INTO app.leetcode_sync_runs(id,trigger_kind,started_at,finished_at,succeeded,fetched_count,changed_count,error_summary) VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, id.New(), trigger, run.StartedAt, run.FinishedAt, run.Error == "", run.Report.Fetched, run.Report.Inserted+run.Report.Updated, nullString(run.Error))
	return err
}

func nullString(v string) any {
	if v == "" {
		return nil
	}
	return v
}
