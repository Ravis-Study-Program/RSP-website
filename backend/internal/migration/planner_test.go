package migration

import (
	"errors"
	"path/filepath"
	"testing"
	"time"
)

func fixedNow() time.Time {
	return time.Date(2026, 8, 13, 12, 30, 0, 0, time.UTC)
}

func validSnapshot(t *testing.T) Snapshot {
	t.Helper()
	snapshot, err := LoadSnapshot(filepath.Join("testdata", "valid_snapshot.json"))
	if err != nil {
		t.Fatal(err)
	}
	return snapshot
}

func TestPlannerBuildsStableCompleteManifest(t *testing.T) {
	planner := NewPlanner(fixedNow)
	first, err := planner.Plan(validSnapshot(t), nil)
	if err != nil {
		t.Fatal(err)
	}
	second, err := planner.Plan(validSnapshot(t), nil)
	if err != nil {
		t.Fatal(err)
	}
	if first.Manifest.Checksum != second.Manifest.Checksum {
		t.Fatalf("manifest is not stable: %s != %s", first.Manifest.Checksum, second.Manifest.Checksum)
	}
	if err := ValidateManifest(first.Manifest); err != nil {
		t.Fatal(err)
	}
	if got, want := len(first.Manifest.Tables), len(LegacyTables); got != want {
		t.Fatalf("table manifests = %d, want %d", got, want)
	}
	if first.Manifest.HasBlockingAnomalies() {
		t.Fatalf("unexpected blocking anomalies: %+v", first.Manifest.Anomalies)
	}
	if !hasAutoFix(first.Manifest.AutoFixes, "NORMALIZE_CATEGORY_SHADOW") {
		t.Fatal("missing normalized category auto-fix")
	}
	if !hasAutoFix(first.Manifest.AutoFixes, "SANITIZE_LEGACY_HTML") {
		t.Fatal("missing HTML sanitation report")
	}
	row := preparedSourceRow(first, "mock-1")
	if got := stringValue(row.Values["interviewer_notes_html"]); got != "<p>Useful <strong>feedback</strong></p>" {
		t.Fatalf("sanitized notes = %q", got)
	}
}

func TestManifestTamperingIsRejected(t *testing.T) {
	prepared, err := NewPlanner(fixedNow).Plan(validSnapshot(t), nil)
	if err != nil {
		t.Fatal(err)
	}
	prepared.Manifest.Tables[0].SourceCount++
	if !errors.Is(ValidateManifest(prepared.Manifest), ErrManifestChecksum) {
		t.Fatal("tampered manifest was accepted")
	}
}

func TestBlockingDuplicateEmailNeedsBoundResolution(t *testing.T) {
	snapshot := validSnapshot(t)
	duplicate := cloneRow(snapshot.Tables["User"][1])
	duplicate["UserId"] = "user-duplicate"
	duplicate["Slug"] = "different-slug"
	duplicate["Email"] = "MENTOR@example.test"
	snapshot.Tables["User"] = append(snapshot.Tables["User"], duplicate)
	planner := NewPlanner(fixedNow)
	prepared, err := planner.Plan(snapshot, nil)
	if !errors.Is(err, ErrBlockingAnomalies) {
		t.Fatalf("error = %v, want blocking anomalies", err)
	}
	if !prepared.Manifest.HasBlockingAnomalies() {
		t.Fatal("manifest did not retain blocking anomaly")
	}
	fingerprint := prepared.Manifest.SourceSchemaFingerprint
	snapshotChecksum := prepared.Manifest.SourceSnapshotChecksum
	resolution := &ResolutionFile{
		Version:                 ResolutionVersion,
		SourceSchemaFingerprint: fingerprint,
		SourceSnapshotChecksum:  snapshotChecksum,
		Resolutions: []Resolution{{
			AnomalyCode: "CONFLICTING_USER_EMAIL",
			SourceTable: "User",
			SourceID:    "user-duplicate",
			Action:      "use_value",
			Value:       "unique@example.test",
		}},
	}
	if err := SealResolutionFile(resolution); err != nil {
		t.Fatal(err)
	}
	resolved, err := planner.Plan(snapshot, resolution)
	if err != nil {
		t.Fatal(err)
	}
	if resolved.Manifest.HasBlockingAnomalies() {
		t.Fatal("checksum-bound resolution did not clear the anomaly")
	}
	resolution.SourceSnapshotChecksum = "wrong"
	if err := SealResolutionFile(resolution); err != nil {
		t.Fatal(err)
	}
	if _, err := planner.Plan(snapshot, resolution); !errors.Is(err, ErrResolutionBinding) {
		t.Fatalf("error = %v, want resolution binding error", err)
	}
}

func TestMissingTableAndOrphanAreBlocking(t *testing.T) {
	snapshot := validSnapshot(t)
	delete(snapshot.Tables, "KickStudentEvent")
	snapshot.Tables["Enrollment"][0]["UserId"] = "missing-user"
	prepared, err := NewPlanner(fixedNow).Plan(snapshot, nil)
	if !errors.Is(err, ErrBlockingAnomalies) {
		t.Fatalf("error = %v", err)
	}
	if !hasAnomalyCode(prepared.Manifest.Anomalies, "MISSING_SOURCE_TABLE") || !hasAnomalyCode(prepared.Manifest.Anomalies, "ORPHAN_REFERENCE") {
		t.Fatalf("anomalies = %+v", prepared.Manifest.Anomalies)
	}
}

func TestEndedSeasonCreatesCloseEventAndCompletesEnrollments(t *testing.T) {
	snapshot := validSnapshot(t)
	snapshot.Tables["Season"][0]["EndDateInclusiveUtc"] = "2026-08-01T00:00:00Z"
	prepared, err := NewPlanner(fixedNow).Plan(snapshot, nil)
	if err != nil {
		t.Fatal(err)
	}
	season := preparedSourceRow(prepared, "season-1")
	if season.Values["status"] != "closed" {
		t.Fatalf("season status = %v", season.Values["status"])
	}
	enrollment := preparedSourceRow(prepared, "e-student")
	if enrollment.Values["state"] != "completed" || enrollment.Values["completed_by_close_id"] != targetUUID("season_close_events", "legacy-close:season-1", snapshot.CapturedAt) {
		t.Fatalf("enrollment = %+v", enrollment.Values)
	}
	_ = preparedSourceRow(prepared, "season-1")
}

func preparedSourceRow(prepared PreparedImport, sourceID string) PreparedRow {
	for _, table := range prepared.Tables {
		for _, row := range table.Rows {
			if row.SourceID == sourceID {
				return row
			}
		}
	}
	panic("prepared source row not found: " + sourceID)
}

func cloneRow(row Row) Row {
	copy := make(Row, len(row))
	for key, value := range row {
		copy[key] = value
	}
	return copy
}

func hasAutoFix(items []AutoFix, code string) bool {
	for _, item := range items {
		if item.Code == code {
			return true
		}
	}
	return false
}

func hasAnomalyCode(items []Anomaly, code string) bool {
	for _, item := range items {
		if item.Code == code {
			return true
		}
	}
	return false
}
