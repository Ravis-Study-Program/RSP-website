package migration

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// Engine represents a backend data structure.
type Engine struct {
	planner *Planner
	now     func() time.Time
}

// NewEngine creates a new value.
func NewEngine(now func() time.Time) *Engine {
	if now == nil {
		now = time.Now
	}
	return &Engine{planner: NewPlanner(now), now: now}
}

// DryRun performs the operation.
func (e *Engine) DryRun(ctx context.Context, source Source, resolutions *ResolutionFile) (PreparedImport, error) {
	return e.planSource(ctx, source, resolutions, time.Time{})
}

func (e *Engine) planSource(ctx context.Context, source Source, resolutions *ResolutionFile, approvedSnapshotAt time.Time) (PreparedImport, error) {
	snapshot, err := source.Snapshot(ctx, SnapshotOptions{ReadOnly: true, Isolation: IsolationRepeatableRead})
	if err != nil {
		return PreparedImport{}, fmt.Errorf("read legacy snapshot: %w", err)
	}
	if !approvedSnapshotAt.IsZero() {
		// CapturedAt is an import transform input (for example, initial mock
		// interview history and ended-season closure). Re-reading unchanged
		// source rows at apply time must reuse the checksum-approved instant.
		snapshot.CapturedAt = approvedSnapshotAt.UTC()
	}
	return e.planner.Plan(snapshot, resolutions)
}

// Apply applies the operation.
func (e *Engine) Apply(ctx context.Context, source Source, target Target, manifest Manifest, resolutions *ResolutionFile) error {
	if err := ValidateManifest(manifest); err != nil {
		return err
	}
	if manifest.HasBlockingAnomalies() {
		return ErrBlockingAnomalies
	}
	prepared, planErr := e.planSource(ctx, source, resolutions, manifest.SourceSnapshotAt)
	if planErr != nil {
		return planErr
	}
	if !sameSourcePlan(manifest, prepared.Manifest) {
		return ErrSourceDrift
	}
	prepared.Manifest = manifest

	tx, err := target.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin target transaction: %w", err)
	}

	committed := false
	defer func() {
		if !committed {
			_ = tx.Abort(context.WithoutCancel(ctx))
		}
	}()
	if err := tx.AcquireAdvisoryLock(ctx, AdvisoryLockKey); err != nil {
		return fmt.Errorf("acquire migration advisory lock: %w", err)
	}

	exists, err := tx.HasRun(ctx, manifest.RunID)
	if err != nil {
		return fmt.Errorf("check migration run: %w", err)
	}
	if exists {
		return ErrAlreadyApplied
	}
	if err := tx.Apply(ctx, prepared); err != nil {
		return fmt.Errorf("apply legacy import: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit legacy import: %w", err)
	}

	committed = true
	return nil
}

// Verify verifies the operation.
func (e *Engine) Verify(ctx context.Context, target Target, manifest Manifest) (Verification, error) {
	if err := ValidateManifest(manifest); err != nil {
		return Verification{}, err
	}

	tx, err := target.Begin(ctx)
	if err != nil {
		return Verification{}, fmt.Errorf("begin target transaction: %w", err)
	}

	committed := false
	defer func() {
		if !committed {
			_ = tx.Abort(context.WithoutCancel(ctx))
		}
	}()
	if err := tx.AcquireAdvisoryLock(ctx, AdvisoryLockKey); err != nil {
		return Verification{}, fmt.Errorf("acquire migration advisory lock: %w", err)
	}

	verification, err := tx.Verification(ctx, manifest)
	if err != nil {
		return Verification{}, err
	}

	verification.VerifiedAt = e.now().UTC()
	if verification.ForeignKeyErrors != 0 {
		return Verification{}, fmt.Errorf("verification found %d invalid foreign keys", verification.ForeignKeyErrors)
	}
	if err := tx.Commit(ctx); err != nil {
		return Verification{}, fmt.Errorf("commit verification: %w", err)
	}

	committed = true
	return verification, nil
}

// Rollback rolls back the operation.
func (e *Engine) Rollback(ctx context.Context, target Target, runID string) error {
	if runID == "" {
		return errors.New("run id is required")
	}
	tx, err := target.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin target transaction: %w", err)
	}

	committed := false
	defer func() {
		if !committed {
			_ = tx.Abort(context.WithoutCancel(ctx))
		}
	}()
	if err := tx.AcquireAdvisoryLock(ctx, AdvisoryLockKey); err != nil {
		return fmt.Errorf("acquire migration advisory lock: %w", err)
	}

	exists, err := tx.HasRun(ctx, runID)
	if err != nil {
		return err
	}
	if !exists {
		return ErrRunNotFound
	}
	if err := tx.RollbackRun(ctx, runID); err != nil {
		return fmt.Errorf("rollback legacy import: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit rollback: %w", err)
	}

	committed = true
	return nil
}

func sameSourcePlan(expected, actual Manifest) bool {
	if expected.SourceSchemaFingerprint != actual.SourceSchemaFingerprint ||
		expected.SourceSnapshotChecksum != actual.SourceSnapshotChecksum ||
		expected.ResolutionChecksum != actual.ResolutionChecksum ||
		len(expected.Tables) != len(actual.Tables) {
		return false
	}
	for index := range expected.Tables {
		left, right := expected.Tables[index], actual.Tables[index]
		if left.SourceTable != right.SourceTable ||
			left.SourceCount != right.SourceCount ||
			left.TransformedCount != right.TransformedCount ||
			left.SourceIDsChecksum != right.SourceIDsChecksum ||
			left.SourceChecksum != right.SourceChecksum ||
			left.TransformedChecksum != right.TransformedChecksum {
			return false
		}
	}
	return true
}
