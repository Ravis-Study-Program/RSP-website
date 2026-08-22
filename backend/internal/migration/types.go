// Package migration plans, applies, verifies, and rolls back legacy imports.
// It intentionally separates source snapshots and target transactions so the
// safety properties are testable without a live database.
package migration

import (
	"context"
	"errors"
	"fmt"
	"time"
)

const (
	// ManifestVersion is a public value used by the backend.
	ManifestVersion = 1
	// ResolutionVersion is a public value used by the backend.
	ResolutionVersion = 1
	// AdvisoryLockKey is a public value used by the backend.
	AdvisoryLockKey = int64(593215744214775351)
)

var (
	// ErrBlockingAnomalies is a public value used by the backend.
	ErrBlockingAnomalies = errors.New("legacy snapshot contains unresolved blocking anomalies")
	// ErrManifestChecksum is a public value used by the backend.
	ErrManifestChecksum = errors.New("manifest checksum is invalid")
	// ErrResolutionChecksum is a public value used by the backend.
	ErrResolutionChecksum = errors.New("resolution file checksum is invalid")
	// ErrResolutionBinding is a public value used by the backend.
	ErrResolutionBinding = errors.New("resolution file does not match the source snapshot")
	// ErrSourceDrift is a public value used by the backend.
	ErrSourceDrift = errors.New("legacy source changed after the manifest was created")
	// ErrAlreadyApplied is a public value used by the backend.
	ErrAlreadyApplied = errors.New("migration run has already been applied")
	// ErrRunNotFound is a public value used by the backend.
	ErrRunNotFound = errors.New("migration run was not found")
)

// Row contains JSON-compatible source or transformed values. Fixture readers
// retain numbers as json.Number so hashes do not lose integer precision.
type Row map[string]any

// Column represents a backend data structure.
type Column struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Nullable bool   `json:"nullable"`
}

// TableSchema represents a backend data structure.
type TableSchema struct {
	Name       string   `json:"name"`
	PrimaryKey []string `json:"primaryKey"`
	Columns    []Column `json:"columns"`
}

// Snapshot represents a backend data structure.
type Snapshot struct {
	CapturedAt time.Time        `json:"capturedAt"`
	EFHistory  []string         `json:"efHistory"`
	Schema     []TableSchema    `json:"schema"`
	Tables     map[string][]Row `json:"tables"`
}

// SnapshotOptions represents a backend data structure.
type SnapshotOptions struct {
	ReadOnly  bool
	Isolation string
}

// IsolationRepeatableRead is a public value used by the backend.
const IsolationRepeatableRead = "repeatable_read"

// Source must produce all legacy rows from one READ ONLY, REPEATABLE READ
// snapshot. Implementations should not return until the snapshot transaction
// has committed or rolled back.
type Source interface {
	Snapshot(context.Context, SnapshotOptions) (Snapshot, error)
}

// Mapping represents a backend data structure.
type Mapping struct {
	SourceTable string `json:"sourceTable"`
	SourceID    string `json:"sourceId"`
	TargetTable string `json:"targetTable"`
	TargetID    string `json:"targetId"`
}

// Provenance represents a backend data structure.
type Provenance struct {
	Mapping
	SourceChecksum      string `json:"sourceChecksum"`
	TransformedChecksum string `json:"transformedChecksum"`
	DuplicateGroup      string `json:"duplicateGroup,omitempty"`
}

// Anomaly represents a backend data structure.
type Anomaly struct {
	Code               string `json:"code"`
	SourceTable        string `json:"sourceTable"`
	SourceID           string `json:"sourceId,omitempty"`
	Severity           string `json:"severity"`
	Detail             string `json:"detail"`
	Context            Row    `json:"context,omitempty"`
	ResolvedByChecksum string `json:"resolvedByChecksum,omitempty"`
}

// Blocking performs the operation.
func (a Anomaly) Blocking() bool {
	return a.Severity == "blocking" && a.ResolvedByChecksum == ""
}

// AutoFix represents a backend data structure.
type AutoFix struct {
	Code        string `json:"code"`
	SourceTable string `json:"sourceTable"`
	SourceID    string `json:"sourceId,omitempty"`
	Detail      string `json:"detail"`
	Before      any    `json:"before,omitempty"`
	After       any    `json:"after,omitempty"`
}

// TableManifest represents a backend data structure.
type TableManifest struct {
	SourceTable         string   `json:"sourceTable"`
	TargetTables        []string `json:"targetTables"`
	SourceCount         int      `json:"sourceCount"`
	TransformedCount    int      `json:"transformedCount"`
	SourceIDs           []string `json:"sourceIds"`
	SourceIDsChecksum   string   `json:"sourceIdsChecksum"`
	SourceChecksum      string   `json:"sourceChecksum"`
	TransformedChecksum string   `json:"transformedChecksum"`
}

// Histogram represents a backend data structure.
type Histogram struct {
	Table  string         `json:"table"`
	Field  string         `json:"field"`
	Values map[string]int `json:"values"`
}

// Manifest represents a backend data structure.
type Manifest struct {
	Version                 int             `json:"version"`
	RunID                   string          `json:"runId"`
	CreatedAt               time.Time       `json:"createdAt"`
	SourceSnapshotAt        time.Time       `json:"sourceSnapshotAt"`
	SourceSchemaFingerprint string          `json:"sourceSchemaFingerprint"`
	SourceSnapshotChecksum  string          `json:"sourceSnapshotChecksum"`
	EFHistory               []string        `json:"efHistory"`
	Tables                  []TableManifest `json:"tables"`
	Mappings                []Mapping       `json:"mappings"`
	Anomalies               []Anomaly       `json:"anomalies"`
	AutoFixes               []AutoFix       `json:"autoFixes"`
	Histograms              []Histogram     `json:"histograms"`
	Resolutions             []Resolution    `json:"resolutions,omitempty"`
	ResolutionChecksum      string          `json:"resolutionChecksum,omitempty"`
	Checksum                string          `json:"checksum"`
}

// HasBlockingAnomalies performs the operation.
func (m Manifest) HasBlockingAnomalies() bool {
	for _, anomaly := range m.Anomalies {
		if anomaly.Blocking() {
			return true
		}
	}
	return false
}

// Resolution represents a backend data structure.
type Resolution struct {
	AnomalyCode string `json:"anomalyCode"`
	SourceTable string `json:"sourceTable"`
	SourceID    string `json:"sourceId,omitempty"`
	Action      string `json:"action"`
	Value       any    `json:"value,omitempty"`
}

func (r Resolution) key() string {
	return r.AnomalyCode + "\x00" + r.SourceTable + "\x00" + r.SourceID
}

// ResolutionFile represents a backend data structure.
type ResolutionFile struct {
	Version                 int          `json:"version"`
	SourceSchemaFingerprint string       `json:"sourceSchemaFingerprint"`
	SourceSnapshotChecksum  string       `json:"sourceSnapshotChecksum"`
	Resolutions             []Resolution `json:"resolutions"`
	Checksum                string       `json:"checksum"`
}

// PreparedRow represents a backend data structure.
type PreparedRow struct {
	ID          string `json:"id"`
	SourceTable string `json:"sourceTable"`
	SourceID    string `json:"sourceId"`
	Values      Row    `json:"values"`
}

// PreparedTable represents a backend data structure.
type PreparedTable struct {
	Name string        `json:"name"`
	Rows []PreparedRow `json:"rows"`
}

// PreparedImport represents a backend data structure.
type PreparedImport struct {
	Manifest   Manifest        `json:"manifest"`
	Tables     []PreparedTable `json:"tables"`
	Provenance []Provenance    `json:"provenance"`
}

// Verification represents a backend data structure.
type Verification struct {
	RunID            string            `json:"runId"`
	ManifestChecksum string            `json:"manifestChecksum"`
	Counts           map[string]int    `json:"counts"`
	Checksums        map[string]string `json:"checksums"`
	ForeignKeyErrors int               `json:"foreignKeyErrors"`
	VerifiedAt       time.Time         `json:"verifiedAt"`
}

// TargetTx represents one target transaction. Apply and rollback acquire the
// dedicated advisory lock before reading or writing import-owned state.
type TargetTx interface {
	AcquireAdvisoryLock(context.Context, int64) error
	HasRun(context.Context, string) (bool, error)
	Apply(context.Context, PreparedImport) error
	Verification(context.Context, Manifest) (Verification, error)
	RollbackRun(context.Context, string) error
	Commit(context.Context) error
	Abort(context.Context) error
}

// Target defines a backend interface.
type Target interface {
	Begin(context.Context) (TargetTx, error)
}

// BlockingError represents a backend data structure.
type BlockingError struct {
	Count int
}

// Error performs the operation.
func (e *BlockingError) Error() string {
	return fmt.Sprintf("%v: %d", ErrBlockingAnomalies, e.Count)
}

// Unwrap performs the operation.
func (e *BlockingError) Unwrap() error { return ErrBlockingAnomalies }
