package migration

import (
	"fmt"
)

func applyResolutions(snapshot *Snapshot, anomalies []Anomaly, resolutions []Resolution, resolved map[string]bool) error {
	anomalyByKey := make(map[string]Anomaly, len(anomalies))
	for _, item := range anomalies {
		anomalyByKey[anomalyKey(item)] = item
	}
	seen := map[string]bool{}
	for _, resolution := range resolutions {
		key := resolution.key()
		if seen[key] {
			return fmt.Errorf("duplicate resolution for %s/%s/%s", resolution.AnomalyCode, resolution.SourceTable, resolution.SourceID)
		}
		seen[key] = true
		anomaly, ok := anomalyByKey[key]
		if !ok {
			return fmt.Errorf("resolution does not match an anomaly: %s/%s/%s", resolution.AnomalyCode, resolution.SourceTable, resolution.SourceID)
		}
		switch resolution.Action {
		case "approve_transform":
			resolved[key] = true
		case "map_id", "use_value":
			field := stringValue(anomaly.Context["field"])
			if field == "" {
				return fmt.Errorf("resolution %s cannot set a value without an anomaly field", resolution.AnomalyCode)
			}
			row := findRow(*snapshot, resolution.SourceTable, resolution.SourceID)
			if row == nil {
				return fmt.Errorf("resolution source row no longer exists")
			}
			row[field] = resolution.Value
			resolved[key] = true
		case "skip_source_row":
			rows := snapshot.Tables[resolution.SourceTable]
			filtered := rows[:0]
			for _, row := range rows {
				if sourceID(resolution.SourceTable, row) != resolution.SourceID {
					filtered = append(filtered, row)
				}
			}
			if len(filtered) == len(rows) {
				return fmt.Errorf("resolution source row no longer exists")
			}
			snapshot.Tables[resolution.SourceTable] = filtered
			resolved[key] = true
		default:
			return fmt.Errorf("unsupported resolution action %q", resolution.Action)
		}
	}
	return nil
}
