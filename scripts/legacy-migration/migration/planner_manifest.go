package migration

import (
	"sort"
)

func buildTableManifests(snapshot Snapshot, tables []PreparedTable, provenance []Provenance) ([]TableManifest, error) {
	preparedBySource := map[string][]PreparedRow{}
	for _, table := range tables {
		for _, row := range table.Rows {
			preparedBySource[row.SourceTable] = append(preparedBySource[row.SourceTable], row)
		}
	}
	targetsBySource := map[string]map[string]bool{}
	for _, item := range provenance {
		if targetsBySource[item.SourceTable] == nil {
			targetsBySource[item.SourceTable] = map[string]bool{}
		}
		targetsBySource[item.SourceTable][item.TargetTable] = true
	}
	result := make([]TableManifest, 0, len(LegacyTables))
	for _, sourceTable := range LegacyTables {
		rows := append([]Row(nil), snapshot.Tables[sourceTable]...)
		sort.Slice(rows, func(left, right int) bool {
			return sourceID(sourceTable, rows[left]) < sourceID(sourceTable, rows[right])
		})
		ids := make([]string, 0, len(rows))
		for _, row := range rows {
			ids = append(ids, sourceID(sourceTable, row))
		}
		targets := make([]string, 0, len(targetsBySource[sourceTable]))
		for target := range targetsBySource[sourceTable] {
			targets = append(targets, target)
		}
		sort.Strings(targets)
		idsChecksum, err := Checksum(ids)
		if err != nil {
			return nil, err
		}

		sourceChecksum, err := Checksum(rows)
		if err != nil {
			return nil, err
		}

		transformed := preparedBySource[sourceTable]
		sort.Slice(transformed, func(left, right int) bool {
			return preparedRowSortKey(transformed[left]) < preparedRowSortKey(transformed[right])
		})
		transformedChecksum, err := Checksum(transformed)
		if err != nil {
			return nil, err
		}

		result = append(result, TableManifest{SourceTable: sourceTable, TargetTables: targets, SourceCount: len(rows), TransformedCount: len(transformed), SourceIDs: ids, SourceIDsChecksum: idsChecksum, SourceChecksum: sourceChecksum, TransformedChecksum: transformedChecksum})
	}
	return result, nil
}

func buildHistograms(snapshot Snapshot) []Histogram {
	result := make([]Histogram, 0)
	for _, table := range LegacyTables {
		rows := snapshot.Tables[table]
		if table != "LeetcodeProblemCategoryMapping" && table != "KickStudentEvent" {
			values := map[string]int{"null": 0, "present": 0}
			for _, row := range rows {
				if row["DeletedAtUtc"] == nil || stringValue(row["DeletedAtUtc"]) == "" {
					values["null"]++
				} else {
					values["present"]++
				}
			}
			result = append(result, Histogram{Table: table, Field: "DeletedAtUtc", Values: values})
		}
	}
	for _, item := range []struct{ table, field string }{{"User", "IsAdmin"}, {"User", "IsGraduate"}, {"Season", "IsDataBackFilled"}, {"MockInterview", "IsPass"}, {"KickStudentEvent", "DeletedAtUtc"}, {"Enrollment", "Role"}, {"Enrollment", "StudentRolePromotion"}, {"LeetcodeProblem", "LeetcodeProblemDifficulty"}} {
		values := map[string]int{}
		for _, row := range snapshot.Tables[item.table] {
			values[stringValue(row[item.field])]++
		}
		result = append(result, Histogram{Table: item.table, Field: item.field, Values: values})
	}
	sort.Slice(result, func(left, right int) bool {
		return result[left].Table+"\x00"+result[left].Field < result[right].Table+"\x00"+result[right].Field
	})
	return result
}
