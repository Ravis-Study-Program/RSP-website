package worker

import (
	"context"
	"errors"
	"testing"
	"time"
)

type lock struct {
	ok       bool
	unlocked bool
}

func (l *lock) TryLock(context.Context, int64) (bool, error) { return l.ok, nil }

func (l *lock) Unlock(context.Context, int64) error { l.unlocked = true; return nil }

type state struct {
	last *time.Time
	runs []Run
}

func (s *state) LastSuccessfulSyncAt(context.Context) (*time.Time, error) { return s.last, nil }

func (s *state) LastScheduledAttemptAt(context.Context) (*time.Time, error) {
	for i := len(s.runs) - 1; i >= 0; i-- {
		if s.runs[i].TriggerKind == "schedule" || s.runs[i].TriggerKind == "catch_up" {
			return &s.runs[i].StartedAt, nil
		}
	}
	return nil, nil
}

func (s *state) RecordSyncRun(_ context.Context, r Run) error {
	s.runs = append(s.runs, r)
	if r.Error == "" {
		v := r.FinishedAt
		s.last = &v
	}
	return nil
}

type syncer struct {
	calls     int
	failUntil int
	partial   bool
}

type blockingSyncer struct{}

func (blockingSyncer) SyncCatalogue(ctx context.Context) (Report, error) {
	<-ctx.Done()
	return Report{}, ctx.Err()
}

func (s *syncer) SyncCatalogue(context.Context) (Report, error) {
	s.calls++
	if s.calls <= s.failUntil {
		return Report{}, errors.New("temporary")
	}
	if s.partial {
		return Report{Fetched: 10, Failed: 1}, nil
	}
	return Report{Fetched: 10, Applied: 10}, nil
}

func TestDecideDueIsPureAndDistinguishesScheduleFromCatchup(t *testing.T) {
	now := time.Date(2026, time.January, 11, 3, 0, 30, 0, time.UTC)
	decision := DecideDue(now, nil, nil)
	if !decision.Run || decision.TriggerKind != "schedule" {
		t.Fatalf("sunday decision=%#v", decision)
	}

	last := now.Add(-time.Second)
	if decision := DecideDue(now, &last, nil); decision.Run {
		t.Fatalf("successful current-window run was scheduled: %#v", decision)
	}

	catchup := DecideDue(now.Add(time.Hour), nil, nil)
	if !catchup.Run || catchup.TriggerKind != "catch_up" {
		t.Fatalf("catch-up decision=%#v", catchup)
	}
}

func TestTimeoutIsRecordedAndDueWindowIsAttemptedOnlyOnce(t *testing.T) {
	now := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)
	st := &state{}
	sy := LeetCodeScheduler{
		Locker:      &lock{ok: true},
		State:       st,
		Syncer:      blockingSyncer{},
		Now:         func() time.Time { return now },
		Timeout:     5 * time.Millisecond,
		MaxAttempts: 1,
	}
	ran, err := sy.RunDue(context.Background())
	if !ran || !errors.Is(err, context.DeadlineExceeded) || len(st.runs) != 1 || st.runs[0].Error == "" {
		t.Fatalf("timeout run: ran=%v err=%v runs=%#v", ran, err, st.runs)
	}

	ran, err = sy.RunDue(context.Background())
	if err != nil || ran || len(st.runs) != 1 {
		t.Fatalf("due window retried: ran=%v err=%v runs=%d", ran, err, len(st.runs))
	}
}

func TestSundayScheduleCatchupAndLock(t *testing.T) {
	now := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)
	l := &lock{ok: true}
	st := &state{}
	sy := LeetCodeScheduler{
		Locker: l,
		State:  st,
		Syncer: &syncer{},
		Now:    func() time.Time { return now },
	}
	ran, err := sy.RunDue(context.Background())
	if err != nil || !ran || len(st.runs) != 1 || !l.unlocked || st.runs[0].TriggerKind != "catch_up" {
		t.Fatalf("catchup failed: %v", err)
	}

	ran, err = sy.RunDue(context.Background())
	if err != nil || ran {
		t.Fatal("ran twice")
	}

	sy.Locker = &lock{ok: false}
	if err := sy.RunManual(context.Background(), "queued-run"); !errors.Is(err, ErrLocked) {
		t.Fatalf("lock: %v", err)
	}
}

func TestSundayScheduledTrigger(t *testing.T) {
	now := time.Date(2026, 8, 16, 3, 0, 20, 0, time.UTC)
	st := &state{}
	sy := LeetCodeScheduler{
		Locker: &lock{ok: true},
		State:  st,
		Syncer: &syncer{},
		Now:    func() time.Time { return now },
	}
	ran, err := sy.RunDue(context.Background())
	if err != nil || !ran || len(st.runs) != 1 || st.runs[0].TriggerKind != "schedule" || st.runs[0].ID != "" {
		t.Fatalf("scheduled trigger: ran=%v runs=%#v err=%v", ran, st.runs, err)
	}
}

func TestRetriesAndPartialFailure(t *testing.T) {
	now := time.Now()
	l := &lock{ok: true}
	st := &state{}
	sync := &syncer{failUntil: 2}
	sy := LeetCodeScheduler{
		Locker:      l,
		State:       st,
		Syncer:      sync,
		Now:         func() time.Time { return now },
		MaxAttempts: 3,
	}
	if err := sy.RunManual(context.Background(), "retry-run"); err != nil || sync.calls != 3 {
		t.Fatalf("retry: %v calls %d", err, sync.calls)
	}

	sync = &syncer{partial: true}
	sy.Syncer = sync
	if err := sy.RunManual(context.Background(), "partial-run"); err == nil || len(st.runs) != 2 || sync.calls != 3 {
		t.Fatal("partial failure was accepted")
	}
}

func TestManualRunKeepsQueuedIDAndBypassesWeeklySchedule(t *testing.T) {
	now := time.Date(2026, 8, 16, 4, 0, 0, 0, time.UTC)
	lastSuccess := now.Add(-time.Minute)
	st := &state{last: &lastSuccess}
	sy := LeetCodeScheduler{
		Locker: &lock{ok: true},
		State:  st,
		Syncer: &syncer{},
		Now:    func() time.Time { return now },
	}
	if err := sy.RunManual(context.Background(), "queued-run"); err != nil {
		t.Fatal(err)
	}
	if len(st.runs) != 1 || st.runs[0].ID != "queued-run" || st.runs[0].TriggerKind != "manual" || st.runs[0].Report.Applied != 10 {
		t.Fatalf("manual run=%#v", st.runs)
	}
	if ran, err := sy.RunDue(context.Background()); err != nil || ran {
		t.Fatalf("successful manual run should satisfy the current window: ran=%v err=%v", ran, err)
	}
}

func TestFailedManualRunDoesNotConsumeScheduledAttempt(t *testing.T) {
	now := time.Date(2026, 8, 16, 4, 0, 0, 0, time.UTC)
	st := &state{}
	sy := LeetCodeScheduler{
		Locker:      &lock{ok: true},
		State:       st,
		Syncer:      &syncer{partial: true},
		Now:         func() time.Time { return now },
		MaxAttempts: 1,
	}
	if err := sy.RunManual(context.Background(), "failed-manual-run"); err == nil {
		t.Fatal("partial failure was accepted")
	}
	sy.Syncer = &syncer{}
	if ran, err := sy.RunDue(context.Background()); err != nil || !ran {
		t.Fatalf("catch-up after failed manual run: ran=%v err=%v", ran, err)
	}
	if len(st.runs) != 2 || st.runs[1].ID != "" || st.runs[1].TriggerKind != "catch_up" {
		t.Fatalf("catch-up reused manual run identity: %#v", st.runs)
	}
}

func TestManualRunRequiresQueuedID(t *testing.T) {
	st := &state{}
	sync := &syncer{}
	sy := LeetCodeScheduler{
		Locker: &lock{ok: true},
		State:  st,
		Syncer: sync,
	}
	if err := sy.RunManual(context.Background(), ""); err == nil || sync.calls != 0 || len(st.runs) != 0 {
		t.Fatalf("missing ID started a run: calls=%d runs=%#v err=%v", sync.calls, st.runs, err)
	}
}
