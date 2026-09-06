// Package worker schedules LeetCode catalogue synchronization.
package worker

import (
	"context"
	"errors"
	"time"
)

const LeetCodeAdvisoryLock int64 = 0x5253504c4353594e

var ErrLocked = errors.New("sync is already running")

type Locker interface {
	TryLock(context.Context, int64) (bool, error)
	Unlock(context.Context, int64) error
}

type State interface {
	LastSuccessfulSyncAt(context.Context) (*time.Time, error)
	LastScheduledAttemptAt(context.Context) (*time.Time, error)
	RecordSyncRun(context.Context, Run) error
}

type Syncer interface {
	SyncCatalogue(context.Context) (Report, error)
}

// Report counts catalogue items fetched, successfully upserted, or rejected.
type Report struct{ Fetched, Applied, Failed int }

type Run struct {
	ID                    string // Existing queue row for a manual run; empty for scheduled runs.
	TriggerKind           string
	StartedAt, FinishedAt time.Time
	Report                Report
	Error                 string
}

// ScheduleDecision is the pure result of evaluating the sync schedule.
type ScheduleDecision struct {
	Run         bool
	TriggerKind string
}

// DecideDue determines whether a sync should run without reading or changing
// external state.
func DecideDue(now time.Time, lastSuccess, lastAttempt *time.Time) ScheduleDecision {
	now = now.UTC()
	due := mostRecentSunday(now)
	if lastSuccess != nil && !lastSuccess.Before(due) {
		return ScheduleDecision{}
	}
	if lastAttempt != nil && !lastAttempt.Before(due) {
		return ScheduleDecision{}
	}
	trigger := "catch_up"
	if now.Sub(due) < time.Minute {
		trigger = "schedule"
	}
	return ScheduleDecision{Run: true, TriggerKind: trigger}
}

// LeetCodeScheduler runs weekly, catch-up, and explicitly queued syncs.
type LeetCodeScheduler struct {
	Locker      Locker
	State       State
	Syncer      Syncer
	Now         func() time.Time
	Timeout     time.Duration
	MaxAttempts int
}

func (s LeetCodeScheduler) RunDue(ctx context.Context) (bool, error) {
	now := s.now()
	last, err := s.State.LastSuccessfulSyncAt(ctx)
	if err != nil {
		return false, err
	}

	lastAttempt, err := s.State.LastScheduledAttemptAt(ctx)
	if err != nil {
		return false, err
	}
	decision := DecideDue(now, last, lastAttempt)
	if !decision.Run {
		return false, nil
	}
	return true, s.run(ctx, "", decision.TriggerKind)
}

// RunManual completes an existing queued request, regardless of the weekly schedule.
func (s LeetCodeScheduler) RunManual(ctx context.Context, runID string) error {
	if runID == "" {
		return errors.New("manual sync run ID is required")
	}
	return s.run(ctx, runID, "manual")
}

func (s LeetCodeScheduler) run(ctx context.Context, runID, triggerKind string) error {
	ok, err := s.Locker.TryLock(ctx, LeetCodeAdvisoryLock)
	if err != nil {
		return err
	}
	if !ok {
		return ErrLocked
	}
	defer func() { _ = s.Locker.Unlock(context.Background(), LeetCodeAdvisoryLock) }()
	start := s.now()
	timeout := s.Timeout
	if timeout == 0 {
		timeout = 2 * time.Minute
	}
	maxAttempts := s.MaxAttempts
	if maxAttempts == 0 {
		maxAttempts = 3
	}
	var report Report
	var runErr error
	for attempt := 0; attempt < maxAttempts; attempt++ {
		runCtx, cancel := context.WithTimeout(ctx, timeout)
		report, runErr = s.Syncer.SyncCatalogue(runCtx)
		cancel()
		if runErr == nil && report.Failed == 0 {
			break
		}
		if runErr == nil {
			runErr = errors.New("partial LeetCode sync failure")
		}
		if ctx.Err() != nil {
			break
		}
	}
	run := Run{
		ID:          runID,
		TriggerKind: triggerKind,
		StartedAt:   start,
		FinishedAt:  s.now(),
		Report:      report,
	}
	if runErr != nil {
		run.Error = runErr.Error()
	}
	if err := s.State.RecordSyncRun(ctx, run); err != nil {
		return err
	}
	return runErr
}

func (s LeetCodeScheduler) now() time.Time {
	if s.Now != nil {
		return s.Now().UTC()
	}
	return time.Now().UTC()
}

func mostRecentSunday(now time.Time) time.Time {
	now = now.UTC()
	days := (int(now.Weekday()) - int(time.Sunday) + 7) % 7
	candidate := time.Date(now.Year(), now.Month(), now.Day(), 3, 0, 0, 0, time.UTC).AddDate(0, 0, -days)
	if candidate.After(now) {
		candidate = candidate.AddDate(0, 0, -7)
	}
	return candidate
}
