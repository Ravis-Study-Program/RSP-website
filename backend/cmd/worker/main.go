// Package main runs scheduled backend workers.
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

	"github.com/magedmg/RSP-website/backend/internal/leetcode"
	"github.com/magedmg/RSP-website/backend/internal/postgres"
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

	repository, err := postgres.Open(ctx, url)
	if err != nil {
		logger.Error("database connection failed", "error", err)
		os.Exit(1)
	}

	defer repository.Close()
	db, closeConnection, err := repository.PinnedConnection(ctx)
	if err != nil {
		logger.Error("database connection pinning failed", "error", err)
		os.Exit(1)
	}
	defer closeConnection()

	state := &postgres.WorkerState{DB: db}
	scheduler := worker.Scheduler{Locker: state, State: state, Syncer: leetcode.Client{URL: syncURL, Sink: leetcode.PostgresSink{DB: db}}, Retries: 3, Timeout: 2 * time.Minute}
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	run := func() {
		ran, err := state.RunPendingManual(ctx, scheduler)
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
