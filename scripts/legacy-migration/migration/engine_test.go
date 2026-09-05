package migration

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestStateTargetApplyAndVerify(t *testing.T) {
	ctx := context.Background()
	engine := NewEngine(fixedNow)
	source := &FixtureSource{Value: validSnapshot(t)}
	prepared, err := engine.DryRun(ctx, source, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !source.LastOptions.ReadOnly || source.LastOptions.Isolation != IsolationRepeatableRead {
		t.Fatalf("unsafe source options: %+v", source.LastOptions)
	}
	target := &StateTarget{Path: filepath.Join(t.TempDir(), "target-state.json"), Now: fixedNow}
	if err := engine.Apply(ctx, source, target, prepared.Manifest, nil); err != nil {
		t.Fatal(err)
	}
	if err := engine.Apply(ctx, source, target, prepared.Manifest, nil); !errors.Is(err, ErrAlreadyApplied) {
		t.Fatalf("second apply error = %v", err)
	}

	verification, err := engine.Verify(ctx, target, prepared.Manifest)
	if err != nil {
		t.Fatal(err)
	}
	if verification.ForeignKeyErrors != 0 || verification.ManifestChecksum != prepared.Manifest.Checksum {
		t.Fatalf("verification = %+v", verification)
	}
}

func TestApplyRejectsSourceDriftBeforeTargetTransaction(t *testing.T) {
	ctx := context.Background()
	engine := NewEngine(fixedNow)
	source := &FixtureSource{Value: validSnapshot(t)}
	prepared, err := engine.DryRun(ctx, source, nil)
	if err != nil {
		t.Fatal(err)
	}

	source.Value.Tables["Problem"][0]["Title"] = "Changed after approval"
	target := &countingTarget{}
	if err := engine.Apply(ctx, source, target, prepared.Manifest, nil); !errors.Is(err, ErrSourceDrift) {
		t.Fatalf("error = %v, want source drift", err)
	}
	if target.beginCount != 0 {
		t.Fatal("target transaction began before source drift was rejected")
	}
}

func TestApplyPinsChecksumApprovedSnapshotTime(t *testing.T) {
	ctx := context.Background()
	engine := NewEngine(fixedNow)
	source := &advancingSnapshotSource{snapshot: validSnapshot(t), step: time.Second}
	prepared, err := engine.DryRun(ctx, source, nil)
	if err != nil {
		t.Fatal(err)
	}

	target := &StateTarget{Path: filepath.Join(t.TempDir(), "target-state.json"), Now: fixedNow}
	if err := engine.Apply(ctx, source, target, prepared.Manifest, nil); err != nil {
		t.Fatalf("unchanged rows at a later transaction timestamp drifted: %v", err)
	}
}

func TestApplyLocksBeforeWritingAndAbortsOnFailure(t *testing.T) {
	ctx := context.Background()
	engine := NewEngine(fixedNow)
	source := &FixtureSource{Value: validSnapshot(t)}
	prepared, err := engine.DryRun(ctx, source, nil)
	if err != nil {
		t.Fatal(err)
	}

	tx := &recordingTx{applyErr: errors.New("fixture write failed")}
	target := &fixedTarget{tx: tx}
	err = engine.Apply(ctx, source, target, prepared.Manifest, nil)
	if err == nil || !strings.Contains(err.Error(), "fixture write failed") {
		t.Fatalf("apply error = %v", err)
	}
	if got, want := strings.Join(tx.events, ","), "lock,has-run,apply,abort"; got != want {
		t.Fatalf("events = %q, want %q", got, want)
	}
}

func TestExactPureJoinDuplicatesCollapseWithProvenance(t *testing.T) {
	snapshot := validSnapshot(t)
	snapshot.Tables["LeetcodeProblemCategoryMapping"] = append(
		snapshot.Tables["LeetcodeProblemCategoryMapping"],
		cloneRow(snapshot.Tables["LeetcodeProblemCategoryMapping"][0]),
	)
	prepared, err := NewPlanner(fixedNow).Plan(snapshot, nil)
	if err != nil {
		t.Fatal(err)
	}

	table := findPreparedTable(prepared, "leetcode_problem_category_mappings")
	if len(table.Rows) != 1 {
		t.Fatalf("join target rows = %d, want 1", len(table.Rows))
	}
	count := 0
	ids := map[string]bool{}
	for _, item := range prepared.Provenance {
		if item.SourceTable == "LeetcodeProblemCategoryMapping" {
			count++
			ids[item.SourceID] = true
		}
	}
	if count != 2 || len(ids) != 2 {
		t.Fatalf("duplicate provenance count=%d uniqueIds=%d", count, len(ids))
	}
	if !hasAutoFix(prepared.Manifest.AutoFixes, "COLLAPSE_EXACT_PURE_JOIN_DUPLICATE") {
		t.Fatal("duplicate collapse was not reported")
	}
}

type countingTarget struct{ beginCount int }

type advancingSnapshotSource struct {
	snapshot Snapshot
	step     time.Duration
	calls    int
}

func (source *advancingSnapshotSource) Snapshot(_ context.Context, _ SnapshotOptions) (Snapshot, error) {
	value, err := cloneSnapshot(source.snapshot)
	if err != nil {
		return Snapshot{}, err
	}

	value.CapturedAt = value.CapturedAt.Add(time.Duration(source.calls) * source.step)
	source.calls++
	return value, nil
}

func (target *countingTarget) Begin(context.Context) (TargetTx, error) {
	target.beginCount++
	return nil, errors.New("unexpected begin")
}

type fixedTarget struct{ tx TargetTx }

func (target *fixedTarget) Begin(context.Context) (TargetTx, error) { return target.tx, nil }

type recordingTx struct {
	events   []string
	applyErr error
}

func (tx *recordingTx) AcquireAdvisoryLock(context.Context, int64) error {
	tx.events = append(tx.events, "lock")
	return nil
}

func (tx *recordingTx) HasRun(context.Context, string) (bool, error) {
	tx.events = append(tx.events, "has-run")
	return false, nil
}

func (tx *recordingTx) Apply(context.Context, PreparedImport) error {
	tx.events = append(tx.events, "apply")
	return tx.applyErr
}

func (tx *recordingTx) Verification(context.Context, Manifest) (Verification, error) {
	return Verification{}, errors.New("unexpected verification")
}

func (tx *recordingTx) Commit(context.Context) error {
	tx.events = append(tx.events, "commit")
	return nil
}

func (tx *recordingTx) Abort(context.Context) error {
	tx.events = append(tx.events, "abort")
	return nil
}

func findPreparedTable(prepared PreparedImport, name string) PreparedTable {
	for _, table := range prepared.Tables {
		if table.Name == name {
			return table
		}
	}
	panic("prepared table not found: " + name)
}
