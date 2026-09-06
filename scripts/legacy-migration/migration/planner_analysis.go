package migration

import (
	"strings"
)

func analyzeSnapshot(snapshot Snapshot) []Anomaly {
	result := make([]Anomaly, 0)
	result = append(result, analyzeSourceKeys(snapshot)...)
	result = append(result, analyzeUsers(snapshot)...)
	result = append(result, analyzeProgramme(snapshot)...)
	result = append(result, analyzePractice(snapshot)...)
	result = append(result, analyzeMockInterviews(snapshot)...)
	result = append(result, analyzeRemovalEvents(snapshot)...)
	sortAnomalies(result)
	return result
}

func analyzeSourceKeys(snapshot Snapshot) []Anomaly {
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

	return result
}

func analyzeUsers(snapshot Snapshot) []Anomaly {
	result := make([]Anomaly, 0)
	kicksByMemberSeason := activeKickEvents(snapshot)

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
	endedSeasons := map[string]string{}
	for _, row := range snapshot.Tables["Season"] {
		if timeBefore(row["EndDateInclusiveUtc"], snapshot.CapturedAt) {
			id := sourceID("Season", row)
			endedSeasons[id] = id
		}
	}
	for _, row := range snapshot.Tables["User"] {
		legacyGraduate, present := optionalBool(row["IsGraduate"])
		if !present {
			continue
		}
		derivedAlumni := derivesAlumni(snapshot, stringValue(row["UserId"]), endedSeasons, kicksByMemberSeason)
		if legacyGraduate != derivedAlumni {
			result = append(result, anomaly(
				"LEGACY_GRADUATE_MISMATCH", "User", sourceID("User", row), "warning",
				"legacy graduate flag differs from alumni derived from completed student enrollment",
				Row{"field": "IsGraduate", "legacyValue": legacyGraduate, "derivedAlumni": derivedAlumni},
			))
		}
	}
	checkUnique("Email", "CONFLICTING_USER_EMAIL")
	checkUnique("Slug", "CONFLICTING_USER_SLUG")

	return result
}

func analyzeProgramme(snapshot Snapshot) []Anomaly {
	result := make([]Anomaly, 0)
	users := indexRows(snapshot, "User")
	seasons := indexRows(snapshot, "Season")
	enrollments := indexRows(snapshot, "Enrollment")
	kicksByMemberSeason := activeKickEvents(snapshot)

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
	return result
}

func analyzePractice(snapshot Snapshot) []Anomaly {
	result := make([]Anomaly, 0)
	users := indexRows(snapshot, "User")
	enrollments := indexRows(snapshot, "Enrollment")
	weeks := indexRows(snapshot, "SeasonWeek")
	problems := indexRows(snapshot, "Problem")
	leetcodes := indexRows(snapshot, "LeetcodeProblem")
	customProblems := indexRows(snapshot, "CustomProblem")
	categories := indexRows(snapshot, "LeetcodeProblemCategory")

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
	return result
}

func analyzeMockInterviews(snapshot Snapshot) []Anomaly {
	result := make([]Anomaly, 0)
	users := indexRows(snapshot, "User")
	seasons := indexRows(snapshot, "Season")
	weeks := indexRows(snapshot, "SeasonWeek")
	leetcodes := indexRows(snapshot, "LeetcodeProblem")
	mocks := indexRows(snapshot, "MockInterview")
	behaviouralRounds := indexRows(snapshot, "BehaviouralMockInterviewRound")
	leetcodeRounds := indexRows(snapshot, "LeetcodeMockInterviewRound")
	customRounds := indexRows(snapshot, "CustomMockInterviewRound")

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
		legacyPass, present := optionalBool(row["IsPass"])
		if present {
			derivedPass := derivesMockPass(snapshot, id)
			if legacyPass != derivedPass {
				result = append(result, anomaly(
					"MOCK_PASS_MISMATCH", "MockInterview", id, "blocking",
					"legacy pass result differs from the current score-derived result",
					Row{"field": "IsPass", "legacyValue": legacyPass, "derivedPass": derivedPass},
				))
			}
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
	return result
}

func analyzeRemovalEvents(snapshot Snapshot) []Anomaly {
	result := make([]Anomaly, 0)
	users := indexRows(snapshot, "User")
	seasons := indexRows(snapshot, "Season")

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
	return result
}
