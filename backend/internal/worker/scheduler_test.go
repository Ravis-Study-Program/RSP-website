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
func (l *lock) Unlock(context.Context, int64) error          { l.unlocked = true; return nil }

type state struct {
	last *time.Time
	runs []Run
}

func (s *state) LastSuccess(context.Context, string) (*time.Time, error) { return s.last, nil }
func (s *state) Record(_ context.Context, r Run) error {
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

func (s *syncer) Sync(context.Context) (Report, error) {
	s.calls++
	if s.calls <= s.failUntil {
		return Report{}, errors.New("temporary")
	}
	if s.partial {
		return Report{Fetched: 10, Failed: 1}, nil
	}
	return Report{Fetched: 10, Updated: 10}, nil
}
func TestSundayScheduleCatchupAndLock(t *testing.T) {
	now := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)
	l := &lock{ok: true}
	st := &state{}
	sy := Scheduler{Locker: l, State: st, Syncer: &syncer{}, Now: func() time.Time { return now }}
	ran, err := sy.RunDue(context.Background())
	if err != nil || !ran || len(st.runs) != 1 || !l.unlocked {
		t.Fatalf("catchup failed: %v", err)
	}
	ran, err = sy.RunDue(context.Background())
	if err != nil || ran {
		t.Fatal("ran twice")
	}
	sy.Locker = &lock{ok: false}
	if err := sy.Run(context.Background()); !errors.Is(err, ErrLocked) {
		t.Fatalf("lock: %v", err)
	}
}
func TestRetriesAndPartialFailure(t *testing.T) {
	now := time.Now()
	l := &lock{ok: true}
	st := &state{}
	sync := &syncer{failUntil: 2}
	sy := Scheduler{Locker: l, State: st, Syncer: sync, Now: func() time.Time { return now }, Retries: 3}
	if err := sy.Run(context.Background()); err != nil || sync.calls != 3 {
		t.Fatalf("retry: %v calls %d", err, sync.calls)
	}
	sync = &syncer{partial: true}
	sy.Syncer = sync
	if err := sy.Run(context.Background()); err == nil || len(st.runs) != 2 {
		t.Fatal("partial failure was accepted")
	}
}
