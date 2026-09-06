package migration

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

func anomaly(code, table, id, severity, detail string, context Row) Anomaly {
	return Anomaly{Code: code, SourceTable: table, SourceID: id, Severity: severity, Detail: detail, Context: context}
}

func orphan(table, id, field, targetID string) Anomaly {
	return anomaly("ORPHAN_REFERENCE", table, id, "blocking", "foreign key does not identify a source row", Row{"field": field, "targetId": targetID})
}

func anomalyKey(item Anomaly) string {
	return item.Code + "\x00" + item.SourceTable + "\x00" + item.SourceID
}

func hasAnomaly(items []Anomaly, key string) bool {
	for _, item := range items {
		if anomalyKey(item) == key {
			return true
		}
	}
	return false
}

func sortAnomalies(items []Anomaly) {
	sort.Slice(items, func(left, right int) bool {
		return anomalyKey(items[left])+"\x00"+items[left].Detail < anomalyKey(items[right])+"\x00"+items[right].Detail
	})
}

func sortAutoFixes(items []AutoFix) {
	sort.Slice(items, func(left, right int) bool {
		l, r := items[left], items[right]
		return l.SourceTable+"\x00"+l.SourceID+"\x00"+l.Code < r.SourceTable+"\x00"+r.SourceID+"\x00"+r.Code
	})
}

func sortMappings(items []Mapping) {
	sort.Slice(items, func(left, right int) bool {
		l, r := items[left], items[right]
		return l.SourceTable+"\x00"+l.SourceID+"\x00"+l.TargetTable+"\x00"+l.TargetID < r.SourceTable+"\x00"+r.SourceID+"\x00"+r.TargetTable+"\x00"+r.TargetID
	})
}

func sortedStrings(values []string) []string {
	result := append([]string(nil), values...)
	sort.Strings(result)
	return result
}

func indexRows(snapshot Snapshot, table string) map[string]Row {
	result := make(map[string]Row, len(snapshot.Tables[table]))
	for _, row := range snapshot.Tables[table] {
		result[sourceID(table, row)] = row
	}
	return result
}

func findRow(snapshot Snapshot, table, id string) Row {
	for _, row := range snapshot.Tables[table] {
		if sourceID(table, row) == id {
			return row
		}
	}
	return nil
}

func findEnrollment(snapshot Snapshot, userID, seasonID string) Row {
	for _, row := range snapshot.Tables["Enrollment"] {
		if stringValue(row["UserId"]) == userID && stringValue(row["SeasonId"]) == seasonID {
			return row
		}
	}
	return nil
}

func validateOptionalReference(result *[]Anomaly, table, id, field string, row Row, targets map[string]Row) {
	value := stringValue(row[field])
	if value == "" {
		return
	}
	if _, ok := targets[value]; !ok {
		*result = append(*result, orphan(table, id, field, value))
	}
}

func invalidTimeRange(start, end any) bool {
	startAt, startOK := parseTime(start)
	endAt, endOK := parseTime(end)
	return startOK && endOK && endAt.Before(startAt)
}

func timeBefore(value any, comparison time.Time) bool {
	parsed, ok := parseTime(value)
	return ok && parsed.Before(comparison)
}

func parseTime(value any) (time.Time, bool) {
	text := stringValue(value)
	if text == "" {
		return time.Time{}, false
	}
	parsed, err := time.Parse(time.RFC3339Nano, text)
	return parsed, err == nil
}

func stringValue(value any) string {
	if value == nil {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return typed
	case json.Number:
		return typed.String()
	case fmt.Stringer:
		return typed.String()
	case bool:
		return strconv.FormatBool(typed)
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64)
	case int:
		return strconv.Itoa(typed)
	case int64:
		return strconv.FormatInt(typed, 10)
	default:
		return fmt.Sprint(typed)
	}
}

func numberValue(value any) int {
	parsed, _ := strconv.Atoi(stringValue(value))
	return parsed
}

func boolValue(value any) bool {
	parsed, _ := strconv.ParseBool(stringValue(value))
	return parsed
}

func optionalBool(value any) (bool, bool) {
	if value == nil || stringValue(value) == "" {
		return false, false
	}
	parsed, err := strconv.ParseBool(stringValue(value))
	return parsed, err == nil
}

func isLegacyDeleted(value any) bool {
	return value != nil && stringValue(value) != ""
}

func activeKickEvents(snapshot Snapshot) map[string]Row {
	result := map[string]Row{}
	for _, row := range snapshot.Tables["KickStudentEvent"] {
		if isLegacyDeleted(row["DeletedAtUtc"]) {
			continue
		}
		key := stringValue(row["StudentId"]) + "\x00" + stringValue(row["SeasonId"])
		current := result[key]
		if current == nil || stringValue(row["KickedAtUtc"]) > stringValue(current["KickedAtUtc"]) {
			result[key] = row
		}
	}
	return result
}

func derivesAlumni(snapshot Snapshot, userID string, endedSeasons map[string]string, kicks map[string]Row) bool {
	for _, enrollment := range snapshot.Tables["Enrollment"] {
		if stringValue(enrollment["UserId"]) != userID || numberValue(enrollment["Role"]) != 0 {
			continue
		}
		seasonID := stringValue(enrollment["SeasonId"])
		if kicks[stringValue(enrollment["UserId"])+"\x00"+seasonID] != nil {
			continue
		}
		if isLegacyDeleted(enrollment["DeletedAtUtc"]) {
			continue
		}
		if endedSeasons[seasonID] != "" {
			return true
		}
	}
	return false
}

func derivesMockPass(snapshot Snapshot, mockID string) bool {
	hasScore := false
	for _, round := range snapshot.Tables["MockInterviewRound"] {
		if stringValue(round["MockInterviewId"]) != mockID || isLegacyDeleted(round["DeletedAtUtc"]) {
			continue
		}
		refs := []struct {
			field  string
			table  string
			scores []string
		}{
			{"BehaviouralMockInterviewRoundId", "BehaviouralMockInterviewRound", []string{"BehavioralScore"}},
			{"LeetcodeMockInterviewRoundId", "LeetcodeMockInterviewRound", []string{"ConfirmQuestionScore", "AlgorithmDesignScore", "ComplexityAnalysisScore", "CodingScore", "TestingScore"}},
			{"CustomMockInterviewRoundId", "CustomMockInterviewRound", []string{"Score"}},
		}
		for _, ref := range refs {
			id := stringValue(round[ref.field])
			if id == "" {
				continue
			}
			for _, subtype := range snapshot.Tables[ref.table] {
				if sourceID(ref.table, subtype) != id || isLegacyDeleted(subtype["DeletedAtUtc"]) {
					continue
				}
				for _, field := range ref.scores {
					hasScore = true
					if numberValue(subtype[field]) < 5 {
						return false
					}
				}
			}
		}
	}
	return hasScore
}

func mapEnum(index int, values []string) string {
	if index < 0 || index >= len(values) {
		return ""
	}
	return values[index]
}

func nilIfEmpty(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func normalizeCategory(value string) string {
	fields := strings.Fields(strings.ToLower(strings.TrimSpace(value)))
	return strings.Join(fields, " ")
}

func mockRoundKind(row Row) (string, string) {
	if id := stringValue(row["BehaviouralMockInterviewRoundId"]); id != "" {
		return "behavioural", id
	}
	if id := stringValue(row["LeetcodeMockInterviewRoundId"]); id != "" {
		return "leetcode", id
	}
	return "custom", stringValue(row["CustomMockInterviewRoundId"])
}

func roundPositions(rows []Row) map[string]int {
	grouped := map[string][]Row{}
	for _, row := range rows {
		grouped[stringValue(row["MockInterviewId"])] = append(grouped[stringValue(row["MockInterviewId"])], row)
	}
	result := map[string]int{}
	for _, group := range grouped {
		sort.Slice(group, func(left, right int) bool {
			leftKey := stringValue(group[left]["CreatedAtUtc"]) + "\x00" + sourceID("MockInterviewRound", group[left])
			rightKey := stringValue(group[right]["CreatedAtUtc"]) + "\x00" + sourceID("MockInterviewRound", group[right])
			return leftKey < rightKey
		})
		for index, row := range group {
			result[sourceID("MockInterviewRound", row)] = index + 1
		}
	}
	return result
}
