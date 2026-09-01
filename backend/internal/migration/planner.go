package migration

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	platformid "github.com/magedmg/RSP-website/backend/internal/platform/id"
	platformsanitize "github.com/magedmg/RSP-website/backend/internal/platform/sanitize"
)

// LegacyTables is a public value used by the backend.
var LegacyTables = []string{
	"BehaviouralMockInterviewRound",
	"CustomMockInterviewRound",
	"CustomProblem",
	"Enrollment",
	"KickStudentEvent",
	"LeetcodeMockInterviewRound",
	"LeetcodeProblem",
	"LeetcodeProblemCategory",
	"LeetcodeProblemCategoryMapping",
	"Mentorship",
	"MockInterview",
	"MockInterviewRound",
	"Problem",
	"ProblemAttempt",
	"Season",
	"SeasonWeek",
	"User",
}

var legacyIDFields = map[string][]string{
	"BehaviouralMockInterviewRound":  {"BehaviouralMockInterviewRoundId"},
	"CustomMockInterviewRound":       {"CustomMockInterviewRoundId"},
	"CustomProblem":                  {"CustomProblemId"},
	"Enrollment":                     {"EnrollmentId"},
	"KickStudentEvent":               {"KickStudentEventId"},
	"LeetcodeMockInterviewRound":     {"LeetcodeMockInterviewRoundId"},
	"LeetcodeProblem":                {"LeetcodeProblemId"},
	"LeetcodeProblemCategory":        {"LeetcodeProblemCategoryId"},
	"LeetcodeProblemCategoryMapping": {"LeetcodeProblemCategoriesLeetcodeProblemCategoryId", "LeetcodeProblemEntityLeetcodeProblemId"},
	"Mentorship":                     {"MentorshipId"},
	"MockInterview":                  {"MockInterviewId"},
	"MockInterviewRound":             {"MockInterviewRoundId"},
	"Problem":                        {"ProblemId"},
	"ProblemAttempt":                 {"ProblemAttemptId"},
	"Season":                         {"SeasonId"},
	"SeasonWeek":                     {"SeasonWeekId"},
	"User":                           {"UserId"},
}

var targetOrder = []string{
	"users",
	"global_role_assignments",
	"seasons",
	"season_close_events",
	"season_weeks",
	"enrollments",
	"mentorships",
	"enrollment_removal_events",
	"problems",
	"leetcode_problems",
	"custom_problems",
	"leetcode_problem_categories",
	"leetcode_problem_category_mappings",
	"problem_attempts",
	"mock_interviews",
	"mock_interview_rounds",
	"behavioural_mock_interview_rounds",
	"leetcode_mock_interview_rounds",
	"custom_mock_interview_rounds",
	"mock_interview_versions",
}

// uuidReferenceTables maps transformed foreign-key fields to their target
// table. Polymorphic audit and provenance fields stay text by design.

var uuidReferenceTables = map[string]map[string]string{
	"user_auth_links":                    {},
	"global_role_assignments":            {"user_id": "users", "granted_by_user_id": "users"},
	"account_deletion_requests":          {"user_id": "users"},
	"season_close_events":                {"season_id": "seasons", "closed_by_user_id": "users", "reopened_by_user_id": "users"},
	"season_weeks":                       {"season_id": "seasons"},
	"enrollments":                        {"user_id": "users", "season_id": "seasons", "completed_by_close_id": "season_close_events"},
	"mentorships":                        {"season_id": "seasons", "mentor_enrollment_id": "enrollments", "student_enrollment_id": "enrollments"},
	"enrollment_removal_events":          {"enrollment_id": "enrollments", "season_id": "seasons", "subject_user_id": "users", "actor_user_id": "users"},
	"leetcode_problems":                  {"problem_id": "problems"},
	"custom_problems":                    {"problem_id": "problems"},
	"leetcode_problem_category_mappings": {"leetcode_problem_id": "leetcode_problems", "category_id": "leetcode_problem_categories"},
	"practice_goals":                     {"user_id": "users", "enabled_by_user_id": "users"},
	"problem_attempts":                   {"user_id": "users", "problem_id": "problems", "enrollment_id": "enrollments", "season_week_id": "season_weeks"},
	"mock_interviews":                    {"interviewer_user_id": "users", "interviewee_user_id": "users", "season_id": "seasons", "season_week_id": "season_weeks"},
	"mock_interview_rounds":              {"mock_interview_id": "mock_interviews"},
	"behavioural_mock_interview_rounds":  {"mock_interview_round_id": "mock_interview_rounds"},
	"leetcode_mock_interview_rounds":     {"mock_interview_round_id": "mock_interview_rounds", "leetcode_problem_id": "leetcode_problems"},
	"custom_mock_interview_rounds":       {"mock_interview_round_id": "mock_interview_rounds"},
	"mock_interview_versions":            {"mock_interview_id": "mock_interviews", "actor_user_id": "users"},
	"leetcode_sync_runs":                 {"requested_by_user_id": "users"},
}

func targetUUID(table, legacyID string, at time.Time) string {
	seed := sha256.Sum256([]byte(table + "\x00" + legacyID))
	return platformid.NewAt(at, bytes.NewReader(seed[:]))
}

// Planner represents a backend data structure.
type Planner struct {
	now func() time.Time
}

// NewPlanner creates a new value.
func NewPlanner(now func() time.Time) *Planner {
	if now == nil {
		now = time.Now
	}
	return &Planner{now: now}
}

// Plan plans the operation.
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
		RunID:                   targetUUID("migration.runs", "legacy-"+createdAt.Format("20060102T150405.000000000Z")+"-"+runSeed[:12], createdAt),
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

func analyzeSnapshot(snapshot Snapshot) []Anomaly {
	result := make([]Anomaly, 0)
	for _, table := range LegacyTables {
		rows, exists := snapshot.Tables[table]
		if !exists {
			result = append(result, anomaly("MISSING_SOURCE_TABLE", table, "", "blocking", "required legacy table is absent", nil))
			continue
		}
		seen := map[string]bool{}
		for _, row := range rows {
			id := sourceID(table, row)
			if id == "" || strings.Contains(id, "||") || strings.HasPrefix(id, "|") || strings.HasSuffix(id, "|") {
				result = append(result, anomaly("MISSING_SOURCE_ID", table, id, "blocking", "source primary key is missing", nil))
			}
			if seen[id] && table != "LeetcodeProblemCategoryMapping" {
				result = append(result, anomaly("DUPLICATE_SOURCE_ID", table, id, "blocking", "source primary key is duplicated", nil))
			}
			seen[id] = true
		}
	}

	users := indexRows(snapshot, "User")
	seasons := indexRows(snapshot, "Season")
	enrollments := indexRows(snapshot, "Enrollment")
	weeks := indexRows(snapshot, "SeasonWeek")
	problems := indexRows(snapshot, "Problem")
	leetcodes := indexRows(snapshot, "LeetcodeProblem")
	customProblems := indexRows(snapshot, "CustomProblem")
	categories := indexRows(snapshot, "LeetcodeProblemCategory")
	mocks := indexRows(snapshot, "MockInterview")
	behaviouralRounds := indexRows(snapshot, "BehaviouralMockInterviewRound")
	leetcodeRounds := indexRows(snapshot, "LeetcodeMockInterviewRound")
	customRounds := indexRows(snapshot, "CustomMockInterviewRound")
	kicksByMemberSeason := map[string]Row{}
	for _, row := range snapshot.Tables["KickStudentEvent"] {
		key := stringValue(row["StudentId"]) + "\x00" + stringValue(row["SeasonId"])
		current := kicksByMemberSeason[key]
		if current == nil || stringValue(row["KickedAtUtc"]) > stringValue(current["KickedAtUtc"]) {
			kicksByMemberSeason[key] = row
		}
	}

	checkUnique := func(field, code string) {
		seen := map[string]string{}
		for _, row := range snapshot.Tables["User"] {
			id := sourceID("User", row)
			value := strings.ToLower(strings.TrimSpace(stringValue(row[field])))
			if value == "" {
				continue
			}
			if previous, exists := seen[value]; exists && previous != id {
				result = append(result, anomaly(code, "User", id, "blocking", "value conflicts with another legacy person", Row{"field": field, "value": value, "otherId": previous}))
			} else {
				seen[value] = id
			}
		}
	}
	checkUnique("Email", "CONFLICTING_USER_EMAIL")
	checkUnique("Slug", "CONFLICTING_USER_SLUG")

	for _, row := range snapshot.Tables["Season"] {
		id := sourceID("Season", row)
		if invalidTimeRange(row["StartDateInclusiveUtc"], row["EndDateInclusiveUtc"]) {
			result = append(result, anomaly("INVALID_SEASON_RANGE", "Season", id, "blocking", "season end precedes start", Row{"field": "EndDateInclusiveUtc"}))
		}
	}
	for _, row := range snapshot.Tables["SeasonWeek"] {
		id := sourceID("SeasonWeek", row)
		seasonID := stringValue(row["SeasonId"])
		if _, ok := seasons[seasonID]; !ok {
			result = append(result, orphan("SeasonWeek", id, "SeasonId", seasonID))
		}
		if invalidTimeRange(row["StartDate"], row["EndDate"]) {
			result = append(result, anomaly("INVALID_WEEK_RANGE", "SeasonWeek", id, "blocking", "week end precedes start", Row{"field": "EndDate"}))
		}
		if numberValue(row["WeekNumber"]) <= 0 {
			result = append(result, anomaly("INVALID_WEEK_NUMBER", "SeasonWeek", id, "blocking", "week number must be positive", Row{"field": "WeekNumber"}))
		}
		if season := seasons[seasonID]; season != nil {
			weekStart, weekStartOK := parseTime(row["StartDate"])
			weekEnd, weekEndOK := parseTime(row["EndDate"])
			seasonStart, seasonStartOK := parseTime(season["StartDateInclusiveUtc"])
			seasonEnd, seasonEndOK := parseTime(season["EndDateInclusiveUtc"])
			if weekStartOK && weekEndOK && seasonStartOK && seasonEndOK && (weekStart.Before(seasonStart) || weekEnd.After(seasonEnd)) {
				result = append(result, anomaly("WEEK_OUTSIDE_SEASON", "SeasonWeek", id, "blocking", "week dates fall outside its season", Row{"field": "StartDate"}))
			}
		}
	}
	for _, row := range snapshot.Tables["Enrollment"] {
		id := sourceID("Enrollment", row)
		if _, ok := users[stringValue(row["UserId"])]; !ok {
			result = append(result, orphan("Enrollment", id, "UserId", stringValue(row["UserId"])))
		}
		if _, ok := seasons[stringValue(row["SeasonId"])]; !ok {
			result = append(result, orphan("Enrollment", id, "SeasonId", stringValue(row["SeasonId"])))
		}
		role := numberValue(row["Role"])
		level := numberValue(row["StudentRolePromotion"])
		if role < 0 || role > 2 || level < 0 || level > 4 || (role == 0 && level == 0) || (role != 0 && level != 0) {
			result = append(result, anomaly("INVALID_ENROLLMENT_ROLE", "Enrollment", id, "blocking", "role and student promotion are inconsistent", Row{"field": "Role"}))
		}
		if row["DeletedAtUtc"] != nil && stringValue(row["DeletedAtUtc"]) != "" {
			key := stringValue(row["UserId"]) + "\x00" + stringValue(row["SeasonId"])
			if kicksByMemberSeason[key] == nil {
				result = append(result, anomaly("DELETED_ENROLLMENT_WITHOUT_KICK", "Enrollment", id, "blocking", "deleted enrollment has no lossless legacy state mapping", Row{"field": "DeletedAtUtc"}))
			}
		}
	}
	for _, row := range snapshot.Tables["Mentorship"] {
		id := sourceID("Mentorship", row)
		mentor, mentorOK := enrollments[stringValue(row["MentorEnrollmentId"])]
		student, studentOK := enrollments[stringValue(row["MenteeEnrollmentId"])]
		if !mentorOK {
			result = append(result, orphan("Mentorship", id, "MentorEnrollmentId", stringValue(row["MentorEnrollmentId"])))
		}
		if !studentOK {
			result = append(result, orphan("Mentorship", id, "MenteeEnrollmentId", stringValue(row["MenteeEnrollmentId"])))
		}
		if mentorOK && studentOK {
			if stringValue(mentor["SeasonId"]) != stringValue(student["SeasonId"]) {
				result = append(result, anomaly("CROSS_SEASON_MENTORSHIP", "Mentorship", id, "blocking", "mentor and student enrollments belong to different seasons", Row{"field": "MenteeEnrollmentId"}))
			}
			if numberValue(mentor["Role"]) == 0 || numberValue(student["Role"]) != 0 {
				result = append(result, anomaly("INVALID_MENTORSHIP_ROLES", "Mentorship", id, "blocking", "mentorship endpoints have invalid roles", Row{"field": "MentorEnrollmentId"}))
			}
		}
	}
	for _, row := range snapshot.Tables["CustomProblem"] {
		if _, ok := problems[stringValue(row["ProblemId"])]; !ok {
			result = append(result, orphan("CustomProblem", sourceID("CustomProblem", row), "ProblemId", stringValue(row["ProblemId"])))
		}
	}
	for _, row := range snapshot.Tables["LeetcodeProblem"] {
		id := sourceID("LeetcodeProblem", row)
		if _, ok := problems[stringValue(row["ProblemId"])]; !ok {
			result = append(result, orphan("LeetcodeProblem", id, "ProblemId", stringValue(row["ProblemId"])))
		}
		difficulty := numberValue(row["LeetcodeProblemDifficulty"])
		if difficulty < 0 || difficulty > 2 {
			result = append(result, anomaly("INVALID_DIFFICULTY", "LeetcodeProblem", id, "blocking", "difficulty enum is outside the known range", Row{"field": "LeetcodeProblemDifficulty"}))
		}
	}
	problemSubtype := map[string]string{}
	for _, row := range snapshot.Tables["LeetcodeProblem"] {
		problemSubtype[stringValue(row["ProblemId"])] = "leetcode"
	}
	for _, row := range snapshot.Tables["CustomProblem"] {
		problemID := stringValue(row["ProblemId"])
		if existing := problemSubtype[problemID]; existing != "" {
			result = append(result, anomaly("MULTI_SUBTYPE_PROBLEM", "CustomProblem", sourceID("CustomProblem", row), "blocking", "base problem is linked to multiple subtype rows", Row{"field": "ProblemId", "existingSubtype": existing}))
		}
		problemSubtype[problemID] = "custom"
	}
	normalizedCategories := map[string]string{}
	for _, row := range snapshot.Tables["LeetcodeProblemCategory"] {
		id := sourceID("LeetcodeProblemCategory", row)
		normalized := normalizeCategory(stringValue(row["Name"]))
		if previous := normalizedCategories[normalized]; previous != "" && previous != id {
			result = append(result, anomaly("CONFLICTING_NORMALIZED_CATEGORY", "LeetcodeProblemCategory", id, "blocking", "categories collide after lookup normalization", Row{"field": "Name", "otherId": previous}))
		} else {
			normalizedCategories[normalized] = id
		}
	}
	for _, row := range snapshot.Tables["LeetcodeProblemCategoryMapping"] {
		id := sourceID("LeetcodeProblemCategoryMapping", row)
		if _, ok := leetcodes[stringValue(row["LeetcodeProblemEntityLeetcodeProblemId"])]; !ok {
			result = append(result, orphan("LeetcodeProblemCategoryMapping", id, "LeetcodeProblemEntityLeetcodeProblemId", stringValue(row["LeetcodeProblemEntityLeetcodeProblemId"])))
		}
		if _, ok := categories[stringValue(row["LeetcodeProblemCategoriesLeetcodeProblemCategoryId"])]; !ok {
			result = append(result, orphan("LeetcodeProblemCategoryMapping", id, "LeetcodeProblemCategoriesLeetcodeProblemCategoryId", stringValue(row["LeetcodeProblemCategoriesLeetcodeProblemCategoryId"])))
		}
	}
	for _, row := range snapshot.Tables["ProblemAttempt"] {
		id := sourceID("ProblemAttempt", row)
		if _, ok := users[stringValue(row["UserId"])]; !ok {
			result = append(result, orphan("ProblemAttempt", id, "UserId", stringValue(row["UserId"])))
		}
		leetcodeID, customID := stringValue(row["LeetcodeProblemId"]), stringValue(row["CustomProblemId"])
		if (leetcodeID == "") == (customID == "") {
			result = append(result, anomaly("MULTI_SUBTYPE_PROBLEM_ATTEMPT", "ProblemAttempt", id, "blocking", "attempt must reference exactly one problem subtype", Row{"field": "LeetcodeProblemId"}))
		} else if leetcodeID != "" {
			if _, ok := leetcodes[leetcodeID]; !ok {
				result = append(result, orphan("ProblemAttempt", id, "LeetcodeProblemId", leetcodeID))
			}
		} else if _, ok := customProblems[customID]; !ok {
			result = append(result, orphan("ProblemAttempt", id, "CustomProblemId", customID))
		}
		if numberValue(row["TimeTakenInMinutes"]) <= 0 {
			result = append(result, anomaly("INVALID_ATTEMPT_DURATION", "ProblemAttempt", id, "blocking", "attempt duration must be positive", Row{"field": "TimeTakenInMinutes"}))
		}
		validateOptionalReference(&result, "ProblemAttempt", id, "EnrollmentId", row, enrollments)
		validateOptionalReference(&result, "ProblemAttempt", id, "SeasonWeekId", row, weeks)
		if enrollment := enrollments[stringValue(row["EnrollmentId"])]; enrollment != nil {
			if stringValue(enrollment["UserId"]) != stringValue(row["UserId"]) {
				result = append(result, anomaly("ATTEMPT_ENROLLMENT_USER_MISMATCH", "ProblemAttempt", id, "blocking", "attempt enrollment belongs to another user", Row{"field": "EnrollmentId"}))
			}
			if week := weeks[stringValue(row["SeasonWeekId"])]; week != nil && stringValue(enrollment["SeasonId"]) != stringValue(week["SeasonId"]) {
				result = append(result, anomaly("CROSS_SEASON_ATTEMPT", "ProblemAttempt", id, "blocking", "attempt enrollment and week belong to different seasons", Row{"field": "SeasonWeekId"}))
			}
		}
	}
	for _, row := range snapshot.Tables["MockInterview"] {
		id := sourceID("MockInterview", row)
		for _, field := range []string{"InterviewerUserId", "IntervieweeUserId"} {
			if _, ok := users[stringValue(row[field])]; !ok {
				result = append(result, orphan("MockInterview", id, field, stringValue(row[field])))
			}
		}
		validateOptionalReference(&result, "MockInterview", id, "SeasonId", row, seasons)
		validateOptionalReference(&result, "MockInterview", id, "SeasonWeekId", row, weeks)
		if numberValue(row["TimeTakenInMinutes"]) <= 0 {
			result = append(result, anomaly("INVALID_MOCK_DURATION", "MockInterview", id, "blocking", "mock interview duration must be positive", Row{"field": "TimeTakenInMinutes"}))
		}
		seasonID, weekID := stringValue(row["SeasonId"]), stringValue(row["SeasonWeekId"])
		if seasonID != "" && weekID != "" && weeks[weekID] != nil && stringValue(weeks[weekID]["SeasonId"]) != seasonID {
			result = append(result, anomaly("CROSS_SEASON_MOCK_WEEK", "MockInterview", id, "blocking", "mock interview week belongs to another season", Row{"field": "SeasonWeekId"}))
		}
	}
	for _, row := range snapshot.Tables["MockInterviewRound"] {
		id := sourceID("MockInterviewRound", row)
		if _, ok := mocks[stringValue(row["MockInterviewId"])]; !ok {
			result = append(result, orphan("MockInterviewRound", id, "MockInterviewId", stringValue(row["MockInterviewId"])))
		}
		refs := []struct {
			field string
			rows  map[string]Row
		}{
			{"BehaviouralMockInterviewRoundId", behaviouralRounds},
			{"LeetcodeMockInterviewRoundId", leetcodeRounds},
			{"CustomMockInterviewRoundId", customRounds},
		}
		count := 0
		for _, ref := range refs {
			value := stringValue(row[ref.field])
			if value == "" {
				continue
			}
			count++
			if _, ok := ref.rows[value]; !ok {
				result = append(result, orphan("MockInterviewRound", id, ref.field, value))
			}
		}
		if count != 1 {
			result = append(result, anomaly("MULTI_SUBTYPE_MOCK_ROUND", "MockInterviewRound", id, "blocking", "mock round must reference exactly one subtype", Row{"field": "BehaviouralMockInterviewRoundId"}))
		}
	}
	parentReferences := map[string]int{}
	for _, row := range snapshot.Tables["MockInterviewRound"] {
		for _, item := range []struct{ kind, field string }{{"behavioural", "BehaviouralMockInterviewRoundId"}, {"leetcode", "LeetcodeMockInterviewRoundId"}, {"custom", "CustomMockInterviewRoundId"}} {
			if id := stringValue(row[item.field]); id != "" {
				parentReferences[item.kind+"\x00"+id]++
			}
		}
	}
	for _, item := range []struct {
		table string
		kind  string
		rows  []Row
	}{{"BehaviouralMockInterviewRound", "behavioural", snapshot.Tables["BehaviouralMockInterviewRound"]}, {"LeetcodeMockInterviewRound", "leetcode", snapshot.Tables["LeetcodeMockInterviewRound"]}, {"CustomMockInterviewRound", "custom", snapshot.Tables["CustomMockInterviewRound"]}} {
		for _, row := range item.rows {
			id := sourceID(item.table, row)
			count := parentReferences[item.kind+"\x00"+id]
			if count != 1 {
				result = append(result, anomaly("MOCK_SUBTYPE_PARENT_COUNT", item.table, id, "blocking", "mock subtype must be referenced by exactly one parent round", Row{"field": legacyIDFields[item.table][0], "parentCount": count}))
			}
		}
	}
	for _, row := range snapshot.Tables["LeetcodeMockInterviewRound"] {
		id := sourceID("LeetcodeMockInterviewRound", row)
		if _, ok := leetcodes[stringValue(row["LeetcodeProblemId"])]; !ok {
			result = append(result, orphan("LeetcodeMockInterviewRound", id, "LeetcodeProblemId", stringValue(row["LeetcodeProblemId"])))
		}
		for _, field := range []string{"ConfirmQuestionScore", "AlgorithmDesignScore", "ComplexityAnalysisScore", "CodingScore", "TestingScore"} {
			if score := numberValue(row[field]); score < 0 || score > 10 {
				result = append(result, anomaly("INVALID_MOCK_SCORE", "LeetcodeMockInterviewRound", id, "blocking", "mock score must be between 0 and 10", Row{"field": field}))
			}
		}
	}
	for _, row := range snapshot.Tables["BehaviouralMockInterviewRound"] {
		if score := numberValue(row["BehavioralScore"]); score < 0 || score > 10 {
			result = append(result, anomaly("INVALID_MOCK_SCORE", "BehaviouralMockInterviewRound", sourceID("BehaviouralMockInterviewRound", row), "blocking", "mock score must be between 0 and 10", Row{"field": "BehavioralScore"}))
		}
	}
	for _, row := range snapshot.Tables["CustomMockInterviewRound"] {
		if score := numberValue(row["Score"]); score < 0 || score > 10 {
			result = append(result, anomaly("INVALID_MOCK_SCORE", "CustomMockInterviewRound", sourceID("CustomMockInterviewRound", row), "blocking", "mock score must be between 0 and 10", Row{"field": "Score"}))
		}
	}
	for _, row := range snapshot.Tables["KickStudentEvent"] {
		id := sourceID("KickStudentEvent", row)
		for _, field := range []string{"MentorId", "StudentId"} {
			if _, ok := users[stringValue(row[field])]; !ok {
				result = append(result, orphan("KickStudentEvent", id, field, stringValue(row[field])))
			}
		}
		if _, ok := seasons[stringValue(row["SeasonId"])]; !ok {
			result = append(result, orphan("KickStudentEvent", id, "SeasonId", stringValue(row["SeasonId"])))
		}
		if findEnrollment(snapshot, stringValue(row["StudentId"]), stringValue(row["SeasonId"])) == nil {
			result = append(result, anomaly("KICK_WITHOUT_ENROLLMENT", "KickStudentEvent", id, "blocking", "kick event has no matching student enrollment", Row{"field": "StudentId"}))
		}
	}
	sortAnomalies(result)
	return result
}

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

func transformSnapshot(snapshot Snapshot, now time.Time, autoFixes *[]AutoFix) ([]PreparedTable, []Provenance, error) {
	byTarget := make(map[string][]PreparedRow)
	provenance := make([]Provenance, 0)
	add := func(target, sourceTable string, source Row, id string, values Row, duplicateGroup string) error {
		sourceChecksum, err := Checksum(source)
		if err != nil {
			return err
		}

		transformedChecksum, err := Checksum(values)
		if err != nil {
			return err
		}

		sourceIDValue := sourceID(sourceTable, source)
		targetID := targetUUID(target, id, now)
		if target == "leetcode_problem_category_mappings" {
			parts := strings.SplitN(id, "|", 2)
			if len(parts) != 2 {
				return fmt.Errorf("invalid category mapping id %q", id)
			}
			targetID = targetUUID("leetcode_problems", parts[0], now) + "|" + targetUUID("leetcode_problem_categories", parts[1], now)
		}
		if _, ok := values["id"]; ok {
			values["id"] = targetID
		}
		byTarget[target] = append(byTarget[target], PreparedRow{ID: targetID, SourceTable: sourceTable, SourceID: sourceIDValue, Values: values})
		provenance = append(provenance, Provenance{
			Mapping:        Mapping{SourceTable: sourceTable, SourceID: sourceIDValue, TargetTable: target, TargetID: targetID},
			SourceChecksum: sourceChecksum, TransformedChecksum: transformedChecksum, DuplicateGroup: duplicateGroup,
		})
		return nil
	}

	leetcodes := indexRows(snapshot, "LeetcodeProblem")
	customProblems := indexRows(snapshot, "CustomProblem")
	enrollments := indexRows(snapshot, "Enrollment")
	kicksByMemberSeason := map[string]Row{}
	for _, row := range snapshot.Tables["KickStudentEvent"] {
		key := stringValue(row["StudentId"]) + "\x00" + stringValue(row["SeasonId"])
		current := kicksByMemberSeason[key]
		if current == nil || stringValue(row["KickedAtUtc"]) > stringValue(current["KickedAtUtc"]) {
			kicksByMemberSeason[key] = row
		}
	}
	endedSeasons := map[string]string{}
	for _, source := range snapshot.Tables["Season"] {
		id := sourceID("Season", source)
		closed := timeBefore(source["EndDateInclusiveUtc"], snapshot.CapturedAt)
		status := "open"
		var closedAt any
		if closed {
			status = "closed"
			closedAt = snapshot.CapturedAt.UTC().Format(time.RFC3339Nano)
			closeID := "legacy-close:" + id
			endedSeasons[id] = closeID
			*autoFixes = append(*autoFixes, AutoFix{Code: "CLOSE_ENDED_SEASON", SourceTable: "Season", SourceID: id, Detail: "ended legacy season imported as closed", Before: "open", After: "closed"})
			closeValues := Row{"id": closeID, "season_id": id, "closed_by_user_id": nil, "close_reason": "Legacy season ended before migration snapshot", "closed_at": closedAt, "reopened_by_user_id": nil, "reopen_reason": nil, "reopened_at": nil, "migration_run_id": nil}
			if err := add("season_close_events", "Season", source, closeID, closeValues, ""); err != nil {
				return nil, nil, err
			}
		}
		values := Row{"id": id, "slug": source["Slug"], "name": source["Name"], "status": status, "start_at": source["StartDateInclusiveUtc"], "end_at": source["EndDateInclusiveUtc"], "location": source["Location"], "image_url": source["ImageUrl"], "resources_url": source["ResourcesUrl"], "legacy_data_backfilled": source["IsDataBackFilled"], "closed_at": closedAt, "deleted_at": source["DeletedAtUtc"], "created_at": source["CreatedAtUtc"], "updated_at": source["UpdatedAtUtc"]}
		if err := add("seasons", "Season", source, id, values, ""); err != nil {
			return nil, nil, err
		}
	}
	for _, source := range snapshot.Tables["User"] {
		id := sourceID("User", source)
		accountState := "active"
		slug, displayName, email := source["Slug"], source["Name"], source["Email"]
		discordID, avatarURL, pseudonymizedAt := source["DiscordId"], source["ProfileImage"], any(nil)
		if source["DeletedAtUtc"] != nil && stringValue(source["DeletedAtUtc"]) != "" {
			accountState, slug, displayName, email = "deleted", "deleted-"+id, "Deleted member", nil
			discordID, avatarURL, pseudonymizedAt = nil, nil, source["DeletedAtUtc"]
			*autoFixes = append(*autoFixes, AutoFix{Code: "PSEUDONYMIZE_DELETED_USER", SourceTable: "User", SourceID: id, Detail: "legacy-deleted user PII removed while retaining opaque identity", Before: "legacy profile PII", After: "pseudonymized"})
		}
		timezone := "Australia/Adelaide"
		if accountState == "deleted" {
			timezone = "UTC"
		}
		values := Row{"id": id, "slug": slug, "display_name": displayName, "email": email, "discord_id": discordID, "avatar_url": avatarURL, "account_state": accountState, "timezone": timezone, "timezone_configured": false, "is_test": source["IsTestUser"], "legacy_is_admin": source["IsAdmin"], "legacy_is_graduate": source["IsGraduate"], "pseudonymized_at": pseudonymizedAt, "deleted_at": source["DeletedAtUtc"], "created_at": source["CreatedAtUtc"], "updated_at": source["UpdatedAtUtc"]}
		if err := add("users", "User", source, id, values, ""); err != nil {
			return nil, nil, err
		}
		if boolValue(source["IsAdmin"]) && (source["DeletedAtUtc"] == nil || stringValue(source["DeletedAtUtc"]) == "") {
			assignmentID := "legacy-director:" + id
			assignment := Row{"id": assignmentID, "user_id": id, "role": "director", "state": "pending_mfa", "granted_by_user_id": nil, "granted_at": source["CreatedAtUtc"], "activated_at": nil, "revoked_at": nil}
			*autoFixes = append(*autoFixes, AutoFix{Code: "MAP_LEGACY_ADMIN_TO_PENDING_DIRECTOR", SourceTable: "User", SourceID: id, Detail: "legacy admin retained as pending Director until TOTP setup", Before: true, After: "director/pending_mfa"})
			if err := add("global_role_assignments", "User", source, assignmentID, assignment, ""); err != nil {
				return nil, nil, err
			}
		}
	}
	for _, source := range snapshot.Tables["SeasonWeek"] {
		id := sourceID("SeasonWeek", source)
		values := Row{"id": id, "season_id": source["SeasonId"], "week_number": source["WeekNumber"], "start_at": source["StartDate"], "end_at": source["EndDate"], "deleted_at": source["DeletedAtUtc"], "created_at": source["CreatedAtUtc"], "updated_at": source["UpdatedAtUtc"]}
		if err := add("season_weeks", "SeasonWeek", source, id, values, ""); err != nil {
			return nil, nil, err
		}
	}
	for _, source := range snapshot.Tables["Enrollment"] {
		id := sourceID("Enrollment", source)
		seasonID := stringValue(source["SeasonId"])
		state := "active"
		var completedBy any
		var stateChanged any
		kick := kicksByMemberSeason[stringValue(source["UserId"])+"\x00"+seasonID]
		if kick != nil {
			state, stateChanged = "kicked", kick["KickedAtUtc"]
			*autoFixes = append(*autoFixes, AutoFix{Code: "DERIVE_KICKED_ENROLLMENT_STATE", SourceTable: "Enrollment", SourceID: id, Detail: "derived kicked state from matching immutable kick event", Before: "deleted", After: "kicked"})
		} else if source["DeletedAtUtc"] != nil && stringValue(source["DeletedAtUtc"]) != "" {
			state, stateChanged = "withdrawn", source["DeletedAtUtc"]
		} else if closeID := endedSeasons[seasonID]; closeID != "" {
			state, completedBy, stateChanged = "completed", closeID, snapshot.CapturedAt.UTC().Format(time.RFC3339Nano)
			*autoFixes = append(*autoFixes, AutoFix{Code: "COMPLETE_ENDED_SEASON_ENROLLMENT", SourceTable: "Enrollment", SourceID: id, Detail: "active enrollment completed by migration close event", Before: "active", After: "completed"})
		}
		role := mapEnum(numberValue(source["Role"]), []string{"student", "mentor", "coordinator"})
		level := mapEnum(numberValue(source["StudentRolePromotion"]), []string{"not_applicable", "novice", "beginner", "intermediate", "advanced"})
		values := Row{"id": id, "user_id": source["UserId"], "season_id": source["SeasonId"], "role": role, "student_level": level, "state": state, "completed_by_close_id": completedBy, "state_changed_at": stateChanged, "deleted_at": source["DeletedAtUtc"], "created_at": source["CreatedAtUtc"], "updated_at": source["UpdatedAtUtc"]}
		if err := add("enrollments", "Enrollment", source, id, values, ""); err != nil {
			return nil, nil, err
		}
	}
	for _, source := range snapshot.Tables["Mentorship"] {
		id := sourceID("Mentorship", source)
		mentor := enrollments[stringValue(source["MentorEnrollmentId"])]
		values := Row{"id": id, "season_id": mentor["SeasonId"], "mentor_enrollment_id": source["MentorEnrollmentId"], "student_enrollment_id": source["MenteeEnrollmentId"], "ended_at": nil, "deleted_at": source["DeletedAtUtc"], "created_at": source["CreatedAtUtc"], "updated_at": source["UpdatedAtUtc"]}
		if err := add("mentorships", "Mentorship", source, id, values, ""); err != nil {
			return nil, nil, err
		}
	}
	for _, source := range snapshot.Tables["KickStudentEvent"] {
		id := sourceID("KickStudentEvent", source)
		enrollment := findEnrollment(snapshot, stringValue(source["StudentId"]), stringValue(source["SeasonId"]))
		values := Row{"id": id, "enrollment_id": sourceID("Enrollment", enrollment), "season_id": source["SeasonId"], "subject_user_id": source["StudentId"], "actor_user_id": source["MentorId"], "resulting_state": "kicked", "reason": source["KickReason"], "occurred_at": source["KickedAtUtc"], "deleted_at_legacy": source["DeletedAtUtc"]}
		if err := add("enrollment_removal_events", "KickStudentEvent", source, id, values, ""); err != nil {
			return nil, nil, err
		}
	}
	for _, source := range snapshot.Tables["Problem"] {
		id := sourceID("Problem", source)
		values := Row{"id": id, "title": source["Title"], "url": source["Link"], "deleted_at": source["DeletedAtUtc"], "created_at": source["CreatedAtUtc"], "updated_at": source["UpdatedAtUtc"]}
		if err := add("problems", "Problem", source, id, values, ""); err != nil {
			return nil, nil, err
		}
	}
	for _, source := range snapshot.Tables["LeetcodeProblem"] {
		id := sourceID("LeetcodeProblem", source)
		values := Row{"id": id, "problem_id": source["ProblemId"], "leetcode_number": source["LeetcodeNumber"], "difficulty": mapEnum(numberValue(source["LeetcodeProblemDifficulty"]), []string{"easy", "medium", "hard"}), "is_premium": source["IsPremium"], "deleted_at": source["DeletedAtUtc"], "created_at": source["CreatedAtUtc"], "updated_at": source["UpdatedAtUtc"]}
		if err := add("leetcode_problems", "LeetcodeProblem", source, id, values, ""); err != nil {
			return nil, nil, err
		}
	}
	for _, source := range snapshot.Tables["CustomProblem"] {
		id := sourceID("CustomProblem", source)
		question, changed := sanitizeLegacyHTML(stringValue(source["Question"]))
		if changed {
			*autoFixes = append(*autoFixes, AutoFix{Code: "SANITIZE_LEGACY_HTML", SourceTable: "CustomProblem", SourceID: id, Detail: "unsafe custom problem HTML removed", Before: source["Question"], After: question})
		}
		values := Row{"id": id, "problem_id": source["ProblemId"], "difficulty_label": source["Difficulty"], "question_html": question, "deleted_at": source["DeletedAtUtc"], "created_at": source["CreatedAtUtc"], "updated_at": source["UpdatedAtUtc"]}
		if err := add("custom_problems", "CustomProblem", source, id, values, ""); err != nil {
			return nil, nil, err
		}
	}
	for _, source := range snapshot.Tables["LeetcodeProblemCategory"] {
		id := sourceID("LeetcodeProblemCategory", source)
		normalized := normalizeCategory(stringValue(source["Name"]))
		if normalized != stringValue(source["Name"]) {
			*autoFixes = append(*autoFixes, AutoFix{Code: "NORMALIZE_CATEGORY_SHADOW", SourceTable: "LeetcodeProblemCategory", SourceID: id, Detail: "normalized lookup value while preserving display name", Before: source["Name"], After: normalized})
		}
		values := Row{"id": id, "name": source["Name"], "normalized_name": normalized, "deleted_at": source["DeletedAtUtc"], "created_at": source["CreatedAtUtc"], "updated_at": source["UpdatedAtUtc"]}
		if err := add("leetcode_problem_categories", "LeetcodeProblemCategory", source, id, values, ""); err != nil {
			return nil, nil, err
		}
	}
	seenJoin := map[string]string{}
	joinOccurrences := map[string]int{}
	for _, source := range sortedRows(snapshot.Tables)["LeetcodeProblemCategoryMapping"] {
		id := sourceID("LeetcodeProblemCategoryMapping", source)
		leetcodeID := stringValue(source["LeetcodeProblemEntityLeetcodeProblemId"])
		categoryID := stringValue(source["LeetcodeProblemCategoriesLeetcodeProblemCategoryId"])
		targetID := leetcodeID + "|" + categoryID
		joinOccurrences[targetID]++
		values := Row{"leetcode_problem_id": leetcodeID, "category_id": categoryID}
		if firstID, duplicate := seenJoin[targetID]; duplicate {
			provenanceID := id + "#duplicate:" + strconv.Itoa(joinOccurrences[targetID])
			*autoFixes = append(*autoFixes, AutoFix{Code: "COLLAPSE_EXACT_PURE_JOIN_DUPLICATE", SourceTable: "LeetcodeProblemCategoryMapping", SourceID: provenanceID, Detail: "duplicate join represented by shared provenance", Before: id, After: firstID})
			sourceChecksum, _ := Checksum(source)
			transformedChecksum, _ := Checksum(values)
			provenance = append(provenance, Provenance{Mapping: Mapping{SourceTable: "LeetcodeProblemCategoryMapping", SourceID: provenanceID, TargetTable: "leetcode_problem_category_mappings", TargetID: targetID}, SourceChecksum: sourceChecksum, TransformedChecksum: transformedChecksum, DuplicateGroup: targetID})
			continue
		}
		seenJoin[targetID] = id
		if err := add("leetcode_problem_category_mappings", "LeetcodeProblemCategoryMapping", source, targetID, values, targetID); err != nil {
			return nil, nil, err
		}
	}
	for _, source := range snapshot.Tables["ProblemAttempt"] {
		id := sourceID("ProblemAttempt", source)
		problemID := ""
		if leetcode := leetcodes[stringValue(source["LeetcodeProblemId"])]; leetcode != nil {
			problemID = stringValue(leetcode["ProblemId"])
		} else if custom := customProblems[stringValue(source["CustomProblemId"])]; custom != nil {
			problemID = stringValue(custom["ProblemId"])
		}
		notes, changed := sanitizeLegacyHTML(stringValue(source["Notes"]))
		if changed {
			*autoFixes = append(*autoFixes, AutoFix{Code: "SANITIZE_LEGACY_HTML", SourceTable: "ProblemAttempt", SourceID: id, Detail: "unsafe notes HTML removed", Before: source["Notes"], After: notes})
		}
		values := Row{"id": id, "user_id": source["UserId"], "problem_id": problemID, "enrollment_id": source["EnrollmentId"], "season_week_id": source["SeasonWeekId"], "attempted_at": source["AttemptStartDateUtc"], "time_taken_minutes": source["TimeTakenInMinutes"], "outcome": "unknown", "confidence": nil, "notes_html": nilIfEmpty(notes), "notes_sanitization_changed": changed, "deleted_at": source["DeletedAtUtc"], "created_at": source["CreatedAtUtc"], "updated_at": source["UpdatedAtUtc"]}
		if err := add("problem_attempts", "ProblemAttempt", source, id, values, ""); err != nil {
			return nil, nil, err
		}
	}
	for _, source := range snapshot.Tables["MockInterview"] {
		id := sourceID("MockInterview", source)
		notes, changed := sanitizeLegacyHTML(stringValue(source["Notes"]))
		if changed {
			*autoFixes = append(*autoFixes, AutoFix{Code: "SANITIZE_LEGACY_HTML", SourceTable: "MockInterview", SourceID: id, Detail: "unsafe interviewer notes HTML removed", Before: source["Notes"], After: notes})
		}
		values := Row{"id": id, "interviewer_user_id": source["InterviewerUserId"], "interviewee_user_id": source["IntervieweeUserId"], "season_id": source["SeasonId"], "season_week_id": source["SeasonWeekId"], "scheduled_at": source["StartDate"], "duration_minutes": source["TimeTakenInMinutes"], "legacy_is_pass": source["IsPass"], "interviewer_notes_html": nilIfEmpty(notes), "notes_sanitization_changed": changed, "deleted_at": source["DeletedAtUtc"], "created_at": source["CreatedAtUtc"], "updated_at": source["UpdatedAtUtc"]}
		if err := add("mock_interviews", "MockInterview", source, id, values, ""); err != nil {
			return nil, nil, err
		}

		versionValues := Row{"id": "legacy-version:" + id, "mock_interview_id": id, "version": 1, "actor_user_id": nil, "reason": "Initial legacy import", "snapshot": values, "created_at": now.Format(time.RFC3339Nano)}
		if err := add("mock_interview_versions", "MockInterview", source, "legacy-version:"+id, versionValues, ""); err != nil {
			return nil, nil, err
		}
	}
	roundPosition := roundPositions(snapshot.Tables["MockInterviewRound"])
	roundParent := map[string]string{}
	for _, source := range snapshot.Tables["MockInterviewRound"] {
		id := sourceID("MockInterviewRound", source)
		kind, subtypeID := mockRoundKind(source)
		roundParent[kind+"\x00"+subtypeID] = id
		comment, changed := sanitizeLegacyHTML(stringValue(source["IntervieweeComment"]))
		status := "pending"
		var reviewedAt any
		if boolValue(source["IsReviewedByInterviewee"]) {
			status, reviewedAt = "reviewed", source["UpdatedAtUtc"]
		}
		values := Row{"id": id, "mock_interview_id": source["MockInterviewId"], "position": roundPosition[id], "kind": kind, "review_status": status, "interviewee_comment_html": nilIfEmpty(comment), "review_sanitization_changed": changed, "reviewed_at": reviewedAt, "deleted_at": source["DeletedAtUtc"], "created_at": source["CreatedAtUtc"], "updated_at": source["UpdatedAtUtc"]}
		if err := add("mock_interview_rounds", "MockInterviewRound", source, id, values, ""); err != nil {
			return nil, nil, err
		}
	}
	for _, source := range snapshot.Tables["BehaviouralMockInterviewRound"] {
		id := sourceID("BehaviouralMockInterviewRound", source)
		values := Row{"id": id, "mock_interview_round_id": roundParent["behavioural\x00"+id], "behavioural_score": source["BehavioralScore"], "deleted_at": source["DeletedAtUtc"], "created_at": source["CreatedAtUtc"], "updated_at": source["UpdatedAtUtc"]}
		*autoFixes = append(*autoFixes, AutoFix{Code: "MAP_BEHAVIOURAL_SPELLING", SourceTable: "BehaviouralMockInterviewRound", SourceID: id, Detail: "mapped legacy BehavioralScore to Australian behavioural spelling", Before: "BehavioralScore", After: "behavioural_score"})
		if err := add("behavioural_mock_interview_rounds", "BehaviouralMockInterviewRound", source, id, values, ""); err != nil {
			return nil, nil, err
		}
	}
	for _, source := range snapshot.Tables["LeetcodeMockInterviewRound"] {
		id := sourceID("LeetcodeMockInterviewRound", source)
		values := Row{"id": id, "mock_interview_round_id": roundParent["leetcode\x00"+id], "leetcode_problem_id": source["LeetcodeProblemId"], "clarify_question_score": source["ConfirmQuestionScore"], "algorithm_design_score": source["AlgorithmDesignScore"], "complexity_analysis_score": source["ComplexityAnalysisScore"], "coding_score": source["CodingScore"], "testing_score": source["TestingScore"], "deleted_at": source["DeletedAtUtc"], "created_at": source["CreatedAtUtc"], "updated_at": source["UpdatedAtUtc"]}
		if err := add("leetcode_mock_interview_rounds", "LeetcodeMockInterviewRound", source, id, values, ""); err != nil {
			return nil, nil, err
		}
	}
	for _, source := range snapshot.Tables["CustomMockInterviewRound"] {
		id := sourceID("CustomMockInterviewRound", source)
		content, changed := sanitizeLegacyHTML(stringValue(source["Content"]))
		if changed {
			*autoFixes = append(*autoFixes, AutoFix{Code: "SANITIZE_LEGACY_HTML", SourceTable: "CustomMockInterviewRound", SourceID: id, Detail: "unsafe custom round HTML removed", Before: source["Content"], After: content})
		}
		values := Row{"id": id, "mock_interview_round_id": roundParent["custom\x00"+id], "content_html": nilIfEmpty(content), "url": source["Link"], "score": source["Score"], "content_sanitization_changed": changed, "deleted_at": source["DeletedAtUtc"], "created_at": source["CreatedAtUtc"], "updated_at": source["UpdatedAtUtc"]}
		if err := add("custom_mock_interview_rounds", "CustomMockInterviewRound", source, id, values, ""); err != nil {
			return nil, nil, err
		}
	}

	// Resolve all legacy foreign-key values after every target ID exists. This
	// keeps the planner deterministic while allowing the target schema to use
	// PostgreSQL UUID columns instead of carrying legacy text IDs forward.
	for target, rows := range byTarget {
		for index := range rows {
			for field, referencedTable := range uuidReferenceTables[target] {
				value, ok := rows[index].Values[field].(string)
				if !ok || value == "" {
					continue
				}
				rows[index].Values[field] = targetUUID(referencedTable, value, now)
			}
			checksum, err := Checksum(rows[index].Values)
			if err != nil {
				return nil, nil, err
			}

			for provenanceIndex := range provenance {
				if provenance[provenanceIndex].TargetTable == target && provenance[provenanceIndex].TargetID == rows[index].ID {
					provenance[provenanceIndex].TransformedChecksum = checksum
				}
			}
		}
	}

	tables := make([]PreparedTable, 0, len(byTarget))
	for _, name := range targetOrder {
		rows := byTarget[name]
		if len(rows) == 0 {
			continue
		}
		sort.Slice(rows, func(left, right int) bool { return rows[left].ID < rows[right].ID })
		tables = append(tables, PreparedTable{Name: name, Rows: rows})
	}
	sort.Slice(provenance, func(left, right int) bool {
		l, r := provenance[left], provenance[right]
		return l.SourceTable+"\x00"+l.SourceID+"\x00"+l.TargetTable+"\x00"+l.TargetID < r.SourceTable+"\x00"+r.SourceID+"\x00"+r.TargetTable+"\x00"+r.TargetID
	})
	return tables, provenance, nil
}

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
		sort.Slice(transformed, func(left, right int) bool { return transformed[left].ID < transformed[right].ID })
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
	for _, item := range []struct{ table, field string }{{"User", "IsTestUser"}, {"User", "IsGraduate"}, {"Enrollment", "Role"}, {"Enrollment", "StudentRolePromotion"}, {"LeetcodeProblem", "LeetcodeProblemDifficulty"}} {
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

// sanitizeLegacyHTML is deliberately conservative. It strips active-content
// elements, inline event handlers, javascript URLs, and embeds while retaining
// the legacy formatting markup for the application sanitizer's allowlist.

func sanitizeLegacyHTML(input string) (string, bool) {
	result := platformsanitize.New().String(input)
	return result, result != input
}
