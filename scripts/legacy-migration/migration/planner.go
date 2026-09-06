package migration

import (
	"encoding/json"
	"sort"
	"time"
)

type Planner struct {
	now func() time.Time
}

func preparedRowSortKey(row PreparedRow) string {
	encoded, _ := json.Marshal(row.Values)
	return row.ID + "\x00" + string(encoded)
}

func NewPlanner(now func() time.Time) *Planner {
	if now == nil {
		now = time.Now
	}
	return &Planner{now: now}
}

func (p *Planner) Plan(snapshot Snapshot, resolutionFile *ResolutionFile) (PreparedImport, error) {
	original, err := cloneSnapshot(snapshot)
	if err != nil {
		return PreparedImport{}, err
	}

	working, err := cloneSnapshot(snapshot)
	if err != nil {
		return PreparedImport{}, err
	}

	schemaFingerprint, err := Checksum(sortedSchema(original.Schema))
	if err != nil {
		return PreparedImport{}, err
	}

	snapshotChecksum, err := snapshotContentChecksum(original)
	if err != nil {
		return PreparedImport{}, err
	}

	initialAnomalies := analyzeSnapshot(working)
	resolutionChecksum := ""
	resolved := map[string]bool{}
	if resolutionFile != nil {
		if err := ValidateResolutionFile(*resolutionFile); err != nil {
			return PreparedImport{}, err
		}
		if resolutionFile.SourceSchemaFingerprint != schemaFingerprint || resolutionFile.SourceSnapshotChecksum != snapshotChecksum {
			return PreparedImport{}, ErrResolutionBinding
		}
		resolutionChecksum = resolutionFile.Checksum
		if err := applyResolutions(&working, initialAnomalies, resolutionFile.Resolutions, resolved); err != nil {
			return PreparedImport{}, err
		}
	}

	autoFixes := make([]AutoFix, 0)
	tables, provenance, err := transformSnapshot(working, original.CapturedAt.UTC(), &autoFixes)
	if err != nil {
		return PreparedImport{}, err
	}

	anomalies := analyzeSnapshot(working)
	for index := range anomalies {
		if resolved[anomalyKey(anomalies[index])] {
			anomalies[index].ResolvedByChecksum = resolutionChecksum
		}
	}
	// Approved transformations intentionally leave the source row unchanged;
	// retain the original anomaly with its resolution evidence.
	for _, anomaly := range initialAnomalies {
		if !resolved[anomalyKey(anomaly)] || hasAnomaly(anomalies, anomalyKey(anomaly)) {
			continue
		}
		anomaly.ResolvedByChecksum = resolutionChecksum
		anomalies = append(anomalies, anomaly)
	}
	sortAnomalies(anomalies)
	sortAutoFixes(autoFixes)

	createdAt := p.now().UTC()
	runSeed, err := Checksum(struct {
		Snapshot string
		At       string
	}{snapshotChecksum, createdAt.Format(time.RFC3339Nano)})
	if err != nil {
		return PreparedImport{}, err
	}

	manifest := Manifest{
		Version:                 ManifestVersion,
		RunID:                   targetUUID("legacy-import", "legacy-"+createdAt.Format("20060102T150405.000000000Z")+"-"+runSeed[:12], createdAt),
		CreatedAt:               createdAt,
		SourceSnapshotAt:        original.CapturedAt.UTC(),
		SourceSchemaFingerprint: schemaFingerprint,
		SourceSnapshotChecksum:  snapshotChecksum,
		EFHistory:               sortedStrings(original.EFHistory),
		Anomalies:               anomalies,
		AutoFixes:               autoFixes,
		Histograms:              buildHistograms(original),
		ResolutionChecksum:      resolutionChecksum,
	}
	if resolutionFile != nil {
		manifest.Resolutions = append([]Resolution(nil), resolutionFile.Resolutions...)
		sort.Slice(manifest.Resolutions, func(left, right int) bool {
			return manifest.Resolutions[left].key() < manifest.Resolutions[right].key()
		})
	}
	manifest.Mappings = make([]Mapping, 0, len(provenance))
	for _, item := range provenance {
		manifest.Mappings = append(manifest.Mappings, item.Mapping)
	}
	sortMappings(manifest.Mappings)
	manifest.Tables, err = buildTableManifests(original, tables, provenance)
	if err != nil {
		return PreparedImport{}, err
	}
	if err := SealManifest(&manifest); err != nil {
		return PreparedImport{}, err
	}

	prepared := PreparedImport{Manifest: manifest, Tables: tables, Provenance: provenance}
	if manifest.HasBlockingAnomalies() {
		count := 0
		for _, anomaly := range manifest.Anomalies {
			if anomaly.Blocking() {
				count++
			}
		}
		return prepared, &BlockingError{Count: count}
	}
	return prepared, nil
}
