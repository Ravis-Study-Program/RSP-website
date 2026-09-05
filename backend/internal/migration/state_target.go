package migration

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"syscall"
	"time"
)

type StoredRun struct {
	Manifest     Manifest   `json:"manifest"`
	State        string     `json:"state"`
	AppliedAt    time.Time  `json:"appliedAt"`
	VerifiedAt   *time.Time `json:"verifiedAt,omitempty"`
	RolledBackAt *time.Time `json:"rolledBackAt,omitempty"`
}

type StoredRecord struct {
	RunID       string `json:"runId"`
	SourceTable string `json:"sourceTable"`
	SourceID    string `json:"sourceId"`
	ID          string `json:"id"`
	Values      Row    `json:"values"`
}

type TargetState struct {
	Runs       map[string]StoredRun               `json:"runs"`
	Tables     map[string]map[string]StoredRecord `json:"tables"`
	Provenance map[string][]Provenance            `json:"provenance"`
}

func newTargetState() TargetState {
	return TargetState{Runs: map[string]StoredRun{}, Tables: map[string]map[string]StoredRecord{}, Provenance: map[string][]Provenance{}}
}

// StateTarget is a durable fixture target used by rehearsal and unit tests. A
// PostgreSQL adapter implements the same transaction contract for deployment.
type StateTarget struct {
	Path string
	Now  func() time.Time
}

func (target *StateTarget) Begin(_ context.Context) (TargetTx, error) {
	if target.Path == "" {
		return nil, errors.New("target state path is required")
	}
	now := target.Now
	if now == nil {
		now = time.Now
	}
	state, err := loadTargetState(target.Path)
	if err != nil {
		return nil, err
	}
	return &stateTx{path: target.Path, state: state, now: now}, nil
}

type stateTx struct {
	path      string
	state     TargetState
	now       func() time.Time
	lockFile  *os.File
	locked    bool
	completed bool
}

func (tx *stateTx) AcquireAdvisoryLock(_ context.Context, key int64) error {
	if key != AdvisoryLockKey {
		return fmt.Errorf("unexpected advisory lock key %d", key)
	}
	if tx.locked {
		return nil
	}
	file, err := os.OpenFile(tx.path+".lock", os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return err
	}
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX); err != nil {
		_ = file.Close()
		return err
	}
	// Reload after locking so concurrent processes cannot commit stale state.
	state, err := loadTargetState(tx.path)
	if err != nil {
		_ = syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
		_ = file.Close()
		return err
	}

	tx.state, tx.lockFile, tx.locked = state, file, true
	return nil
}

func (tx *stateTx) HasRun(_ context.Context, runID string) (bool, error) {
	_, exists := tx.state.Runs[runID]
	return exists, nil
}

// Apply applies the operation.
func (tx *stateTx) Apply(_ context.Context, prepared PreparedImport) error {
	if !tx.locked {
		return errors.New("migration advisory lock is not held")
	}
	if _, exists := tx.state.Runs[prepared.Manifest.RunID]; exists {
		return ErrAlreadyApplied
	}
	for _, table := range prepared.Tables {
		if tx.state.Tables[table.Name] == nil {
			tx.state.Tables[table.Name] = map[string]StoredRecord{}
		}
		for _, row := range table.Rows {
			if existing, exists := tx.state.Tables[table.Name][row.ID]; exists {
				return fmt.Errorf("target row %s/%s already belongs to run %s", table.Name, row.ID, existing.RunID)
			}
			tx.state.Tables[table.Name][row.ID] = StoredRecord{RunID: prepared.Manifest.RunID, SourceTable: row.SourceTable, SourceID: row.SourceID, ID: row.ID, Values: row.Values}
		}
	}
	tx.state.Provenance[prepared.Manifest.RunID] = append([]Provenance(nil), prepared.Provenance...)
	tx.state.Runs[prepared.Manifest.RunID] = StoredRun{Manifest: prepared.Manifest, State: "applied", AppliedAt: tx.now().UTC()}
	return nil
}

func (tx *stateTx) Verification(_ context.Context, manifest Manifest) (Verification, error) {
	run, exists := tx.state.Runs[manifest.RunID]
	if !exists || run.State == "rolled_back" {
		return Verification{}, ErrRunNotFound
	}
	if run.Manifest.Checksum != manifest.Checksum {
		return Verification{}, ErrManifestChecksum
	}
	rowsBySource := map[string][]PreparedRow{}
	for _, table := range tx.state.Tables {
		for _, record := range table {
			if record.RunID != manifest.RunID {
				continue
			}
			rowsBySource[record.SourceTable] = append(rowsBySource[record.SourceTable], PreparedRow{ID: record.ID, SourceTable: record.SourceTable, SourceID: record.SourceID, Values: record.Values})
		}
	}
	verification := Verification{RunID: manifest.RunID, ManifestChecksum: manifest.Checksum, Counts: map[string]int{}, Checksums: map[string]string{}}
	for _, expected := range manifest.Tables {
		rows := rowsBySource[expected.SourceTable]
		sort.Slice(rows, func(left, right int) bool { return rows[left].ID < rows[right].ID })
		checksum, err := Checksum(rows)
		if err != nil {
			return Verification{}, err
		}

		verification.Counts[expected.SourceTable] = len(rows)
		verification.Checksums[expected.SourceTable] = checksum
		if len(rows) != expected.TransformedCount || checksum != expected.TransformedChecksum {
			return Verification{}, fmt.Errorf("verification mismatch for %s: count %d/%d checksum %s/%s", expected.SourceTable, len(rows), expected.TransformedCount, checksum, expected.TransformedChecksum)
		}
	}
	verifiedAt := tx.now().UTC()
	run.State, run.VerifiedAt = "verified", &verifiedAt
	tx.state.Runs[manifest.RunID] = run
	return verification, nil
}

// RollbackRun rolls back the operation.
func (tx *stateTx) RollbackRun(_ context.Context, runID string) error {
	if !tx.locked {
		return errors.New("migration advisory lock is not held")
	}
	run, exists := tx.state.Runs[runID]
	if !exists {
		return ErrRunNotFound
	}
	if run.State == "rolled_back" {
		return errors.New("migration run is already rolled back")
	}
	// Reverse target dependency order to model RESTRICT-safe PostgreSQL deletes.
	for index := len(targetOrder) - 1; index >= 0; index-- {
		table := tx.state.Tables[targetOrder[index]]
		for id, record := range table {
			if record.RunID == runID {
				delete(table, id)
			}
		}
	}
	rolledBackAt := tx.now().UTC()
	run.State, run.RolledBackAt = "rolled_back", &rolledBackAt
	tx.state.Runs[runID] = run
	return nil
}

func (tx *stateTx) Commit(_ context.Context) error {
	if tx.completed {
		return errors.New("transaction is already complete")
	}
	if !tx.locked {
		return errors.New("migration advisory lock is not held")
	}
	if err := saveTargetState(tx.path, tx.state); err != nil {
		return err
	}

	tx.completed = true
	return tx.release()
}

func (tx *stateTx) Abort(_ context.Context) error {
	if tx.completed {
		return nil
	}
	tx.completed = true
	return tx.release()
}

func (tx *stateTx) release() error {
	if tx.lockFile == nil {
		return nil
	}
	unlockErr := syscall.Flock(int(tx.lockFile.Fd()), syscall.LOCK_UN)
	closeErr := tx.lockFile.Close()
	tx.lockFile = nil
	if unlockErr != nil {
		return unlockErr
	}
	return closeErr
}

func loadTargetState(path string) (TargetState, error) {
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return newTargetState(), nil
	}
	if err != nil {
		return TargetState{}, err
	}

	defer file.Close()
	decoder := json.NewDecoder(file)
	decoder.UseNumber()
	state := newTargetState()
	if err := decoder.Decode(&state); err != nil {
		return TargetState{}, fmt.Errorf("decode target state: %w", err)
	}
	return state, nil
}

func saveTargetState(path string, state TargetState) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}

	encoded, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}

	encoded = append(encoded, '\n')
	temporary, err := os.CreateTemp(filepath.Dir(path), ".rsp-migrate-state-*")
	if err != nil {
		return err
	}

	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(0o600); err != nil {
		_ = temporary.Close()
		return err
	}
	if _, err := temporary.Write(encoded); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return os.Rename(temporaryPath, path)
}
