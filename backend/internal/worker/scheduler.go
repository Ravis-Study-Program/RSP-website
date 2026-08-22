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
	LastSuccess(context.Context, string) (*time.Time, error)
	LastAttempt(context.Context, string) (*time.Time, error)
	Record(context.Context, Run) error
}

type Syncer interface {
	Sync(context.Context) (Report, error)
}

type Report struct{ Fetched, Inserted, Updated, Failed int }

type Run struct {
	Job                   string
	TriggerKind           string
	StartedAt, FinishedAt time.Time
	Report                Report
	Error                 string
}

type Scheduler struct {
	Locker  Locker
	State   State
	Syncer  Syncer
	Now     func() time.Time
	Timeout time.Duration
	Retries int
}

func (s Scheduler) RunDue(ctx context.Context) (bool, error) {
	now := s.now()
	last, err := s.State.LastSuccess(ctx, "leetcode_sync")
	if err != nil {
		return false, err
	}

	due := mostRecentSunday(now)
	if last != nil && !last.Before(due) {
		return false, nil
	}
	lastAttempt, err := s.State.LastAttempt(ctx, "leetcode_sync")
	if err != nil {
		return false, err
	}
	if lastAttempt != nil && !lastAttempt.Before(due) {
		return false, nil
	}
	trigger := "catch_up"
	if now.Sub(due) < time.Minute {
		trigger = "schedule"
	}
	return true, s.run(ctx, trigger)
}

func (s Scheduler) Run(ctx context.Context) error {
	return s.run(ctx, "schedule")
}

func (s Scheduler) run(ctx context.Context, triggerKind string) error {
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
	retries := s.Retries
	if retries == 0 {
		retries = 3
	}
	var report Report
	var runErr error
	for attempt := 0; attempt < retries; attempt++ {
		runCtx, cancel := context.WithTimeout(ctx, timeout)
		report, runErr = s.Syncer.Sync(runCtx)
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
	run := Run{Job: "leetcode_sync", TriggerKind: triggerKind, StartedAt: start, FinishedAt: s.now(), Report: report}
	if runErr != nil {
		run.Error = runErr.Error()
	}
	if err := s.State.Record(ctx, run); err != nil {
		return err
	}
	return runErr
}

func (s Scheduler) now() time.Time {
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
