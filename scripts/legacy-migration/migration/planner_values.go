package migration

import (
	"sort"
	"strings"
)

func snapshotContentChecksum(snapshot Snapshot) (string, error) {
	return Checksum(struct {
		EFHistory []string         `json:"efHistory"`
		Schema    []TableSchema    `json:"schema"`
		Tables    map[string][]Row `json:"tables"`
	}{sortedStrings(snapshot.EFHistory), sortedSchema(snapshot.Schema), sortedRows(snapshot.Tables)})
}

func sortedSchema(schema []TableSchema) []TableSchema {
	result := append([]TableSchema(nil), schema...)
	for index := range result {
		result[index].PrimaryKey = sortedStrings(result[index].PrimaryKey)
		sort.Slice(result[index].Columns, func(left, right int) bool {
			return result[index].Columns[left].Name < result[index].Columns[right].Name
		})
	}
	sort.Slice(result, func(left, right int) bool { return result[left].Name < result[right].Name })
	return result
}

func sortedRows(tables map[string][]Row) map[string][]Row {
	result := make(map[string][]Row, len(tables))
	for table, rows := range tables {
		result[table] = append([]Row(nil), rows...)
		sort.Slice(result[table], func(left, right int) bool {
			return sourceID(table, result[table][left]) < sourceID(table, result[table][right])
		})
	}
	return result
}

func sourceID(table string, row Row) string {
	fields := legacyIDFields[table]
	parts := make([]string, 0, len(fields))
	for _, field := range fields {
		parts = append(parts, stringValue(row[field]))
	}
	return strings.Join(parts, "|")
}
