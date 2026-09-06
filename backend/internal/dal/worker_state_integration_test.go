//go:build integration

package dal

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/magedmg/RSP-website/backend/internal/worker"
)

type workerTestSyncer struct {
	report worker.Report
	calls  int
}

func (s *workerTestSyncer) Sync(context.Context) (worker.Report, error) {
	s.calls++
	return s.report, nil
}

type storedWorkerRun struct {
	ID                string
	TriggerKind       string
	RequestedByUserID sql.NullString
	StartedAt         time.Time
	FinishedAt        sql.NullTime
	Succeeded         sql.NullBool
	FetchedCount      sql.NullInt64
	ChangedCount      sql.NullInt64
	ErrorSummary      sql.NullString
}

func testLeetCodeWorkerRuns(t *testing.T, ctx context.Context, repository *Store, actorID string) {
	t.Helper()
	state, err := repository.OpenWorkerSession(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer state.Close()
	loadRun := func(runID string) storedWorkerRun {
		t.Helper()
		var run storedWorkerRun
		if err := repository.pool.QueryRow(ctx, `SELECT id,trigger_kind,requested_by_user_id,started_at,finished_at,succeeded,fetched_count,changed_count,error_summary FROM app.leetcode_sync_runs WHERE id=$1`, runID).Scan(&run.ID, &run.TriggerKind, &run.RequestedByUserID, &run.StartedAt, &run.FinishedAt, &run.Succeeded, &run.FetchedCount, &run.ChangedCount, &run.ErrorSummary); err != nil {
			t.Fatal(err)
		}
		return run
	}

	manualID, err := state.PendingManualRunID(ctx)
	if err != nil || manualID == "" {
		t.Fatalf("pending manual run=%q err=%v", manualID, err)
	}
	queued := loadRun(manualID)
	now := time.Now().UTC().Add(time.Hour)
	syncer := &workerTestSyncer{report: worker.Report{
		Fetched: 3,
		Applied: 2,
		Failed:  1,
	}}
	scheduler := worker.LeetCodeScheduler{
		Locker:  state,
		State:   state,
		Syncer:  syncer,
		Now:     func() time.Time { return now },
		Retries: 1,
	}
	if err := scheduler.RunManual(ctx, manualID); err == nil {
		t.Fatal("partial manual failure was accepted")
	}
	failed := loadRun(manualID)
	if failed.TriggerKind != "manual" || failed.RequestedByUserID.String != actorID || !failed.StartedAt.Equal(queued.StartedAt) ||
		!failed.FinishedAt.Valid || !failed.Succeeded.Valid || failed.Succeeded.Bool || failed.FetchedCount.Int64 != 3 || failed.ChangedCount.Int64 != 2 || !failed.ErrorSummary.Valid {
		t.Fatalf("manual completion did not preserve request provenance or counts: %#v", failed)
	}
	if pendingID, err := state.PendingManualRunID(ctx); err != nil || pendingID != "" {
		t.Fatalf("failed request remained pending: id=%q err=%v", pendingID, err)
	}
	if lastAttempt, err := state.LastAttempt(ctx); err != nil || lastAttempt != nil {
		t.Fatalf("manual failure consumed scheduled attempt: last=%v err=%v", lastAttempt, err)
	}
	var auditCount int
	if err := repository.pool.QueryRow(ctx, `SELECT count(*) FROM app.audit_events WHERE action='leetcode.sync_requested' AND subject_id=$1 AND request_id='integration-request'`, manualID).Scan(&auditCount); err != nil || auditCount != 1 {
		t.Fatalf("manual request audit count=%d err=%v", auditCount, err)
	}

	// Move to the next Sunday so this run is scheduled, even after a failed manual request.
	nextSunday := now.AddDate(0, 0, 7-int(now.Weekday()))
	now = time.Date(nextSunday.Year(), nextSunday.Month(), nextSunday.Day(), 3, 0, 20, 0, time.UTC)
	syncer.report = worker.Report{Fetched: 3, Applied: 3}
	if ran, err := scheduler.RunDue(ctx); err != nil || !ran {
		t.Fatalf("scheduled sync: ran=%v err=%v", ran, err)
	}
	var scheduledID string
	if err := repository.pool.QueryRow(ctx, `SELECT id FROM app.leetcode_sync_runs WHERE trigger_kind='schedule'`).Scan(&scheduledID); err != nil || scheduledID == "" || scheduledID == manualID {
		t.Fatalf("scheduled run reused manual ID: id=%q err=%v", scheduledID, err)
	}
	scheduled := loadRun(scheduledID)
	if scheduled.RequestedByUserID.Valid || !scheduled.StartedAt.Equal(now) || !scheduled.Succeeded.Bool || scheduled.ChangedCount.Int64 != 3 || scheduled.ErrorSummary.Valid {
		t.Fatalf("scheduled completion=%#v", scheduled)
	}
	if retained := loadRun(manualID); retained.Succeeded.Bool || retained.ChangedCount.Int64 != 2 || retained.TriggerKind != "manual" {
		t.Fatalf("scheduled sync changed the completed manual row: %#v", retained)
	}
	if ran, err := scheduler.RunDue(ctx); err != nil || ran || syncer.calls != 2 {
		t.Fatalf("scheduled window repeated: ran=%v calls=%d err=%v", ran, syncer.calls, err)
	}

	now = now.AddDate(0, 0, 8)
	if ran, err := scheduler.RunDue(ctx); err != nil || !ran {
		t.Fatalf("catch-up sync: ran=%v err=%v", ran, err)
	}
	var catchupCount int
	if err := repository.pool.QueryRow(ctx, `SELECT count(*) FROM app.leetcode_sync_runs WHERE trigger_kind='catch_up' AND succeeded`).Scan(&catchupCount); err != nil || catchupCount != 1 {
		t.Fatalf("catch-up runs=%d err=%v", catchupCount, err)
	}

	// A new manual request still runs after the current weekly window succeeded.
	if err := repository.QueueLeetCodeSync(ctx, actorID, "worker-second-manual-request"); err != nil {
		t.Fatal(err)
	}
	secondManualID, err := state.PendingManualRunID(ctx)
	if err != nil || secondManualID == "" || secondManualID == manualID {
		t.Fatalf("second pending manual run=%q err=%v", secondManualID, err)
	}
	if err := scheduler.RunManual(ctx, secondManualID); err != nil {
		t.Fatal(err)
	}
	secondManual := loadRun(secondManualID)
	if secondManual.TriggerKind != "manual" || secondManual.RequestedByUserID.String != actorID || !secondManual.Succeeded.Bool || secondManual.ChangedCount.Int64 != 3 || syncer.calls != 4 {
		t.Fatalf("second manual completion=%#v calls=%d", secondManual, syncer.calls)
	}
	if pendingID, err := state.PendingManualRunID(ctx); err != nil || pendingID != "" {
		t.Fatalf("successful manual request remained pending: id=%q err=%v", pendingID, err)
	}
}
