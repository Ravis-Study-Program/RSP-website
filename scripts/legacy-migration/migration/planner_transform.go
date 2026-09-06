package migration

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

type snapshotTransformer struct {
	snapshot            Snapshot
	now                 time.Time
	autoFixes           *[]AutoFix
	byTarget            map[string][]PreparedRow
	provenance          []Provenance
	leetcodes           map[string]Row
	customProblems      map[string]Row
	enrollments         map[string]Row
	kicksByMemberSeason map[string]Row
	endedSeasons        map[string]string
}

func transformSnapshot(snapshot Snapshot, now time.Time, autoFixes *[]AutoFix) ([]PreparedTable, []Provenance, error) {
	t := snapshotTransformer{
		snapshot: snapshot, now: now, autoFixes: autoFixes,
		byTarget: make(map[string][]PreparedRow), provenance: make([]Provenance, 0),
		leetcodes: indexRows(snapshot, "LeetcodeProblem"), customProblems: indexRows(snapshot, "CustomProblem"),
		enrollments: indexRows(snapshot, "Enrollment"), kicksByMemberSeason: activeKickEvents(snapshot), endedSeasons: map[string]string{},
	}
	if err := t.transformSeasons(); err != nil {
		return nil, nil, err
	}
	if err := t.transformUsers(); err != nil {
		return nil, nil, err
	}
	if err := t.transformProgramme(); err != nil {
		return nil, nil, err
	}
	if err := t.transformProblems(); err != nil {
		return nil, nil, err
	}
	if err := t.transformAttempts(); err != nil {
		return nil, nil, err
	}
	if err := t.transformMockInterviews(); err != nil {
		return nil, nil, err
	}
	if err := t.transformMockRounds(); err != nil {
		return nil, nil, err
	}
	return t.finish()
}

func (t *snapshotTransformer) addRow(target, sourceTable string, source Row, id string, values Row, duplicateGroup string) error {
	sourceChecksum, err := Checksum(source)
	if err != nil {
		return err
	}

	transformedChecksum, err := Checksum(values)
	if err != nil {
		return err
	}

	sourceIDValue := sourceID(sourceTable, source)
	targetID := targetUUID(target, id, t.now)
	if target == "leetcode_problem_category_mappings" {
		parts := strings.SplitN(id, "|", 2)
		if len(parts) != 2 {
			return fmt.Errorf("invalid category mapping id %q", id)
		}
		targetID = targetUUID("leetcode_problems", parts[0], t.now) + "|" + targetUUID("leetcode_problem_categories", parts[1], t.now)
	}
	if referencedTable, ok := uuidReferenceTables[target][targetKeyColumn[target]]; ok {
		targetID = targetUUID(referencedTable, id, t.now)
	}
	if _, ok := values["id"]; ok {
		values["id"] = targetID
	}
	t.byTarget[target] = append(t.byTarget[target], PreparedRow{ID: targetID, SourceTable: sourceTable, SourceID: sourceIDValue, Values: values})
	t.provenance = append(t.provenance, Provenance{
		Mapping:        Mapping{SourceTable: sourceTable, SourceID: sourceIDValue, TargetTable: target, TargetID: targetID},
		SourceChecksum: sourceChecksum, TransformedChecksum: transformedChecksum, DuplicateGroup: duplicateGroup,
	})
	return nil
}

func (t *snapshotTransformer) transformSeasons() error {
	for _, source := range t.snapshot.Tables["Season"] {
		id := sourceID("Season", source)
		closed := timeBefore(source["EndDateInclusiveUtc"], t.snapshot.CapturedAt)
		status := "open"
		var closedAt any
		if closed {
			status = "closed"
			closedAt = t.snapshot.CapturedAt.UTC().Format(time.RFC3339Nano)
			closeID := "legacy-close:" + id
			t.endedSeasons[id] = closeID
			*t.autoFixes = append(*t.autoFixes, AutoFix{Code: "CLOSE_ENDED_SEASON", SourceTable: "Season", SourceID: id, Detail: "ended legacy season imported as closed", Before: "open", After: "closed"})
			closeValues := Row{"id": closeID, "season_id": id, "closed_by_user_id": nil, "close_reason": "Legacy season ended before migration snapshot", "closed_at": closedAt, "reopened_by_user_id": nil, "reopen_reason": nil, "reopened_at": nil}
			if err := t.addRow("season_close_events", "Season", source, closeID, closeValues, ""); err != nil {
				return err
			}
		}
		values := Row{"id": id, "slug": source["Slug"], "name": source["Name"], "status": status, "start_at": source["StartDateInclusiveUtc"], "end_at": source["EndDateInclusiveUtc"], "location": source["Location"], "image_url": source["ImageUrl"], "resources_url": source["ResourcesUrl"], "closed_at": closedAt, "deleted_at": source["DeletedAtUtc"], "created_at": source["CreatedAtUtc"], "updated_at": source["UpdatedAtUtc"]}
		if err := t.addRow("seasons", "Season", source, id, values, ""); err != nil {
			return err
		}
	}
	return nil
}

func (t *snapshotTransformer) transformUsers() error {
	for _, source := range t.snapshot.Tables["User"] {
		id := sourceID("User", source)
		accountState := "active"
		slug, displayName, email := source["Slug"], source["Name"], source["Email"]
		discordID, avatarURL := source["DiscordId"], source["ProfileImage"]
		if source["DeletedAtUtc"] != nil && stringValue(source["DeletedAtUtc"]) != "" {
			accountState, slug, displayName, email = "deleted", "deleted-"+id, "Deleted member", nil
			discordID, avatarURL = nil, nil
			*t.autoFixes = append(*t.autoFixes, AutoFix{Code: "PSEUDONYMIZE_DELETED_USER", SourceTable: "User", SourceID: id, Detail: "legacy-deleted user PII removed while retaining opaque identity", Before: "legacy profile PII", After: "pseudonymized"})
		}
		timezone := "Australia/Adelaide"
		if accountState == "deleted" {
			timezone = "UTC"
		}
		values := Row{"id": id, "account_state": accountState, "deleted_at": source["DeletedAtUtc"], "created_at": source["CreatedAtUtc"], "updated_at": source["UpdatedAtUtc"]}
		if err := t.addRow("users", "User", source, id, values, ""); err != nil {
			return err
		}
		if err := t.addRow("user_profiles", "User", source, id, Row{"user_id": id, "slug": slug, "display_name": displayName, "avatar_url": avatarURL, "updated_at": source["UpdatedAtUtc"]}, ""); err != nil {
			return err
		}
		if err := t.addRow("user_preferences", "User", source, id, Row{"user_id": id, "timezone": timezone, "timezone_configured": false, "updated_at": source["UpdatedAtUtc"]}, ""); err != nil {
			return err
		}
		if err := t.addRow("user_contacts", "User", source, id, Row{"user_id": id, "email": email, "discord_id": discordID, "updated_at": source["UpdatedAtUtc"]}, ""); err != nil {
			return err
		}
		if err := t.addRow("user_security", "User", source, id, Row{"user_id": id, "security_version": 1, "mfa_configured": false, "updated_at": source["UpdatedAtUtc"]}, ""); err != nil {
			return err
		}
		legacyGraduate, present := optionalBool(source["IsGraduate"])
		derivedAlumni := derivesAlumni(t.snapshot, stringValue(source["UserId"]), t.endedSeasons, t.kicksByMemberSeason)
		if present && legacyGraduate != derivedAlumni {
			*t.autoFixes = append(*t.autoFixes, AutoFix{Code: "DROP_LEGACY_GRADUATE_FLAG", SourceTable: "User", SourceID: id, Detail: "legacy graduate flag retained as reconciliation evidence; alumni is derived from completed student enrollment", Before: legacyGraduate, After: derivedAlumni})
		}
		if boolValue(source["IsAdmin"]) && (source["DeletedAtUtc"] == nil || stringValue(source["DeletedAtUtc"]) == "") {
			assignmentID := "legacy-director:" + id
			assignment := Row{"id": assignmentID, "user_id": id, "role": "director", "state": "pending_mfa", "granted_by_user_id": nil, "granted_at": source["CreatedAtUtc"], "activated_at": nil, "revoked_at": nil}
			*t.autoFixes = append(*t.autoFixes, AutoFix{Code: "MAP_LEGACY_ADMIN_TO_PENDING_DIRECTOR", SourceTable: "User", SourceID: id, Detail: "legacy admin retained as pending Director until TOTP setup", Before: true, After: "director/pending_mfa"})
			if err := t.addRow("global_role_assignments", "User", source, assignmentID, assignment, ""); err != nil {
				return err
			}
		}
	}
	return nil
}

func (t *snapshotTransformer) transformProgramme() error {
	for _, source := range t.snapshot.Tables["SeasonWeek"] {
		id := sourceID("SeasonWeek", source)
		values := Row{"id": id, "season_id": source["SeasonId"], "week_number": source["WeekNumber"], "start_at": source["StartDate"], "end_at": source["EndDate"], "deleted_at": source["DeletedAtUtc"], "created_at": source["CreatedAtUtc"], "updated_at": source["UpdatedAtUtc"]}
		if err := t.addRow("season_weeks", "SeasonWeek", source, id, values, ""); err != nil {
			return err
		}
	}
	for _, source := range t.snapshot.Tables["Enrollment"] {
		id := sourceID("Enrollment", source)
		seasonID := stringValue(source["SeasonId"])
		state := "active"
		var completedBy any
		var stateChanged any
		kick := t.kicksByMemberSeason[stringValue(source["UserId"])+"\x00"+seasonID]
		if kick != nil {
			state, stateChanged = "kicked", kick["KickedAtUtc"]
			*t.autoFixes = append(*t.autoFixes, AutoFix{Code: "DERIVE_KICKED_ENROLLMENT_STATE", SourceTable: "Enrollment", SourceID: id, Detail: "derived kicked state from matching immutable kick event", Before: "deleted", After: "kicked"})
		} else if source["DeletedAtUtc"] != nil && stringValue(source["DeletedAtUtc"]) != "" {
			state, stateChanged = "withdrawn", source["DeletedAtUtc"]
		} else if closeID := t.endedSeasons[seasonID]; closeID != "" {
			state, completedBy, stateChanged = "completed", closeID, t.snapshot.CapturedAt.UTC().Format(time.RFC3339Nano)
			*t.autoFixes = append(*t.autoFixes, AutoFix{Code: "COMPLETE_ENDED_SEASON_ENROLLMENT", SourceTable: "Enrollment", SourceID: id, Detail: "active enrollment completed by migration close event", Before: "active", After: "completed"})
		}
		role := mapEnum(numberValue(source["Role"]), []string{"student", "mentor", "coordinator"})
		level := mapEnum(numberValue(source["StudentRolePromotion"]), []string{"not_applicable", "novice", "beginner", "intermediate", "advanced"})
		values := Row{"id": id, "user_id": source["UserId"], "season_id": source["SeasonId"], "role": role, "student_level": level, "state": state, "completed_by_close_id": completedBy, "state_changed_at": stateChanged, "deleted_at": source["DeletedAtUtc"], "created_at": source["CreatedAtUtc"], "updated_at": source["UpdatedAtUtc"]}
		if err := t.addRow("enrollments", "Enrollment", source, id, values, ""); err != nil {
			return err
		}
	}
	for _, source := range t.snapshot.Tables["Mentorship"] {
		id := sourceID("Mentorship", source)
		mentor := t.enrollments[stringValue(source["MentorEnrollmentId"])]
		values := Row{"id": id, "season_id": mentor["SeasonId"], "mentor_enrollment_id": source["MentorEnrollmentId"], "student_enrollment_id": source["MenteeEnrollmentId"], "ended_at": nil, "deleted_at": source["DeletedAtUtc"], "created_at": source["CreatedAtUtc"], "updated_at": source["UpdatedAtUtc"]}
		if err := t.addRow("mentorships", "Mentorship", source, id, values, ""); err != nil {
			return err
		}
	}
	for _, source := range t.snapshot.Tables["KickStudentEvent"] {
		id := sourceID("KickStudentEvent", source)
		if isLegacyDeleted(source["DeletedAtUtc"]) {
			*t.autoFixes = append(*t.autoFixes, AutoFix{Code: "DROP_LEGACY_DELETED_KICK_EVENT", SourceTable: "KickStudentEvent", SourceID: id, Detail: "deleted legacy kick event is not imported into the current append-only history", Before: source["DeletedAtUtc"], After: "not imported"})
			continue
		}
		enrollment := findEnrollment(t.snapshot, stringValue(source["StudentId"]), stringValue(source["SeasonId"]))
		values := Row{"id": id, "enrollment_id": sourceID("Enrollment", enrollment), "season_id": source["SeasonId"], "subject_user_id": source["StudentId"], "actor_user_id": source["MentorId"], "resulting_state": "kicked", "reason": source["KickReason"], "occurred_at": source["KickedAtUtc"]}
		if err := t.addRow("enrollment_removal_events", "KickStudentEvent", source, id, values, ""); err != nil {
			return err
		}
	}
	return nil
}

func (t *snapshotTransformer) transformProblems() error {
	for _, source := range t.snapshot.Tables["Problem"] {
		id := sourceID("Problem", source)
		values := Row{"id": id, "title": source["Title"], "url": source["Link"], "deleted_at": source["DeletedAtUtc"], "created_at": source["CreatedAtUtc"], "updated_at": source["UpdatedAtUtc"]}
		if err := t.addRow("problems", "Problem", source, id, values, ""); err != nil {
			return err
		}
	}
	for _, source := range t.snapshot.Tables["LeetcodeProblem"] {
		id := sourceID("LeetcodeProblem", source)
		values := Row{"id": id, "problem_id": source["ProblemId"], "leetcode_number": source["LeetcodeNumber"], "difficulty": mapEnum(numberValue(source["LeetcodeProblemDifficulty"]), []string{"easy", "medium", "hard"}), "is_premium": source["IsPremium"], "deleted_at": source["DeletedAtUtc"], "created_at": source["CreatedAtUtc"], "updated_at": source["UpdatedAtUtc"]}
		if err := t.addRow("leetcode_problems", "LeetcodeProblem", source, id, values, ""); err != nil {
			return err
		}
	}
	for _, source := range t.snapshot.Tables["CustomProblem"] {
		id := sourceID("CustomProblem", source)
		question, changed := sanitizeLegacyHTML(stringValue(source["Question"]))
		if changed {
			*t.autoFixes = append(*t.autoFixes, AutoFix{Code: "SANITIZE_LEGACY_HTML", SourceTable: "CustomProblem", SourceID: id, Detail: "unsafe custom problem HTML removed", Before: source["Question"], After: question})
		}
		values := Row{"id": id, "problem_id": source["ProblemId"], "difficulty_label": source["Difficulty"], "question_html": question, "deleted_at": source["DeletedAtUtc"], "created_at": source["CreatedAtUtc"], "updated_at": source["UpdatedAtUtc"]}
		if err := t.addRow("custom_problems", "CustomProblem", source, id, values, ""); err != nil {
			return err
		}
	}
	for _, source := range t.snapshot.Tables["LeetcodeProblemCategory"] {
		id := sourceID("LeetcodeProblemCategory", source)
		normalized := normalizeCategory(stringValue(source["Name"]))
		if normalized != stringValue(source["Name"]) {
			*t.autoFixes = append(*t.autoFixes, AutoFix{Code: "NORMALIZE_CATEGORY_SHADOW", SourceTable: "LeetcodeProblemCategory", SourceID: id, Detail: "normalized lookup value while preserving display name", Before: source["Name"], After: normalized})
		}
		values := Row{"id": id, "name": source["Name"], "normalized_name": normalized, "deleted_at": source["DeletedAtUtc"], "created_at": source["CreatedAtUtc"], "updated_at": source["UpdatedAtUtc"]}
		if err := t.addRow("leetcode_problem_categories", "LeetcodeProblemCategory", source, id, values, ""); err != nil {
			return err
		}
	}
	seenJoin := map[string]string{}
	joinOccurrences := map[string]int{}
	for _, source := range sortedRows(t.snapshot.Tables)["LeetcodeProblemCategoryMapping"] {
		id := sourceID("LeetcodeProblemCategoryMapping", source)
		leetcodeID := stringValue(source["LeetcodeProblemEntityLeetcodeProblemId"])
		categoryID := stringValue(source["LeetcodeProblemCategoriesLeetcodeProblemCategoryId"])
		targetID := leetcodeID + "|" + categoryID
		joinOccurrences[targetID]++
		values := Row{"leetcode_problem_id": leetcodeID, "category_id": categoryID}
		if firstID, duplicate := seenJoin[targetID]; duplicate {
			provenanceID := id + "#duplicate:" + strconv.Itoa(joinOccurrences[targetID])
			*t.autoFixes = append(*t.autoFixes, AutoFix{Code: "COLLAPSE_EXACT_PURE_JOIN_DUPLICATE", SourceTable: "LeetcodeProblemCategoryMapping", SourceID: provenanceID, Detail: "duplicate join represented by shared provenance", Before: id, After: firstID})
			sourceChecksum, _ := Checksum(source)
			transformedChecksum, _ := Checksum(values)
			t.provenance = append(t.provenance, Provenance{Mapping: Mapping{SourceTable: "LeetcodeProblemCategoryMapping", SourceID: provenanceID, TargetTable: "leetcode_problem_category_mappings", TargetID: targetID}, SourceChecksum: sourceChecksum, TransformedChecksum: transformedChecksum, DuplicateGroup: targetID})
			continue
		}
		seenJoin[targetID] = id
		if err := t.addRow("leetcode_problem_category_mappings", "LeetcodeProblemCategoryMapping", source, targetID, values, targetID); err != nil {
			return err
		}
	}
	return nil
}

func (t *snapshotTransformer) transformAttempts() error {
	for _, source := range t.snapshot.Tables["ProblemAttempt"] {
		id := sourceID("ProblemAttempt", source)
		problemID := ""
		if leetcode := t.leetcodes[stringValue(source["LeetcodeProblemId"])]; leetcode != nil {
			problemID = stringValue(leetcode["ProblemId"])
		} else if custom := t.customProblems[stringValue(source["CustomProblemId"])]; custom != nil {
			problemID = stringValue(custom["ProblemId"])
		}
		notes, changed := sanitizeLegacyHTML(stringValue(source["Notes"]))
		if changed {
			*t.autoFixes = append(*t.autoFixes, AutoFix{Code: "SANITIZE_LEGACY_HTML", SourceTable: "ProblemAttempt", SourceID: id, Detail: "unsafe notes HTML removed", Before: source["Notes"], After: notes})
		}
		values := Row{"id": id, "user_id": source["UserId"], "problem_id": problemID, "enrollment_id": source["EnrollmentId"], "season_week_id": source["SeasonWeekId"], "attempted_at": source["AttemptStartDateUtc"], "time_taken_minutes": source["TimeTakenInMinutes"], "outcome": "unknown", "confidence": nil, "notes_html": nilIfEmpty(notes), "notes_sanitization_changed": changed, "deleted_at": source["DeletedAtUtc"], "created_at": source["CreatedAtUtc"], "updated_at": source["UpdatedAtUtc"]}
		if err := t.addRow("problem_attempts", "ProblemAttempt", source, id, values, ""); err != nil {
			return err
		}
	}
	return nil
}

func (t *snapshotTransformer) transformMockInterviews() error {
	for _, source := range t.snapshot.Tables["MockInterview"] {
		id := sourceID("MockInterview", source)
		notes, changed := sanitizeLegacyHTML(stringValue(source["Notes"]))
		if changed {
			*t.autoFixes = append(*t.autoFixes, AutoFix{Code: "SANITIZE_LEGACY_HTML", SourceTable: "MockInterview", SourceID: id, Detail: "unsafe interviewer notes HTML removed", Before: source["Notes"], After: notes})
		}
		values := Row{"id": id, "interviewer_user_id": source["InterviewerUserId"], "interviewee_user_id": source["IntervieweeUserId"], "season_id": source["SeasonId"], "season_week_id": source["SeasonWeekId"], "scheduled_at": source["StartDate"], "duration_minutes": source["TimeTakenInMinutes"], "interviewer_notes_html": nilIfEmpty(notes), "notes_sanitization_changed": changed, "deleted_at": source["DeletedAtUtc"], "created_at": source["CreatedAtUtc"], "updated_at": source["UpdatedAtUtc"]}
		if err := t.addRow("mock_interviews", "MockInterview", source, id, values, ""); err != nil {
			return err
		}
		legacyPass, present := optionalBool(source["IsPass"])
		derivedPass := derivesMockPass(t.snapshot, id)
		if present && legacyPass != derivedPass {
			*t.autoFixes = append(*t.autoFixes, AutoFix{Code: "DROP_LEGACY_MOCK_PASS", SourceTable: "MockInterview", SourceID: id, Detail: "current mock interview pass status is derived from round scores", Before: legacyPass, After: derivedPass})
		}

		versionValues := Row{"id": "legacy-version:" + id, "mock_interview_id": id, "version": 1, "actor_user_id": nil, "reason": "Initial legacy import", "snapshot": values, "created_at": t.now.Format(time.RFC3339Nano)}
		if err := t.addRow("mock_interview_versions", "MockInterview", source, "legacy-version:"+id, versionValues, ""); err != nil {
			return err
		}
	}
	return nil
}

func (t *snapshotTransformer) transformMockRounds() error {
	roundPosition := roundPositions(t.snapshot.Tables["MockInterviewRound"])
	roundParent := map[string]string{}
	for _, source := range t.snapshot.Tables["MockInterviewRound"] {
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
		if err := t.addRow("mock_interview_rounds", "MockInterviewRound", source, id, values, ""); err != nil {
			return err
		}
	}
	for _, source := range t.snapshot.Tables["BehaviouralMockInterviewRound"] {
		id := sourceID("BehaviouralMockInterviewRound", source)
		values := Row{"id": id, "mock_interview_round_id": roundParent["behavioural\x00"+id], "behavioural_score": source["BehavioralScore"], "deleted_at": source["DeletedAtUtc"], "created_at": source["CreatedAtUtc"], "updated_at": source["UpdatedAtUtc"]}
		*t.autoFixes = append(*t.autoFixes, AutoFix{Code: "MAP_BEHAVIOURAL_SPELLING", SourceTable: "BehaviouralMockInterviewRound", SourceID: id, Detail: "mapped legacy BehavioralScore to Australian behavioural spelling", Before: "BehavioralScore", After: "behavioural_score"})
		if err := t.addRow("behavioural_mock_interview_rounds", "BehaviouralMockInterviewRound", source, id, values, ""); err != nil {
			return err
		}
	}
	for _, source := range t.snapshot.Tables["LeetcodeMockInterviewRound"] {
		id := sourceID("LeetcodeMockInterviewRound", source)
		values := Row{"id": id, "mock_interview_round_id": roundParent["leetcode\x00"+id], "leetcode_problem_id": source["LeetcodeProblemId"], "clarify_question_score": source["ConfirmQuestionScore"], "algorithm_design_score": source["AlgorithmDesignScore"], "complexity_analysis_score": source["ComplexityAnalysisScore"], "coding_score": source["CodingScore"], "testing_score": source["TestingScore"], "deleted_at": source["DeletedAtUtc"], "created_at": source["CreatedAtUtc"], "updated_at": source["UpdatedAtUtc"]}
		if err := t.addRow("leetcode_mock_interview_rounds", "LeetcodeMockInterviewRound", source, id, values, ""); err != nil {
			return err
		}
	}
	for _, source := range t.snapshot.Tables["CustomMockInterviewRound"] {
		id := sourceID("CustomMockInterviewRound", source)
		content, changed := sanitizeLegacyHTML(stringValue(source["Content"]))
		if changed {
			*t.autoFixes = append(*t.autoFixes, AutoFix{Code: "SANITIZE_LEGACY_HTML", SourceTable: "CustomMockInterviewRound", SourceID: id, Detail: "unsafe custom round HTML removed", Before: source["Content"], After: content})
		}
		values := Row{"id": id, "mock_interview_round_id": roundParent["custom\x00"+id], "content_html": nilIfEmpty(content), "url": source["Link"], "score": source["Score"], "content_sanitization_changed": changed, "deleted_at": source["DeletedAtUtc"], "created_at": source["CreatedAtUtc"], "updated_at": source["UpdatedAtUtc"]}
		if err := t.addRow("custom_mock_interview_rounds", "CustomMockInterviewRound", source, id, values, ""); err != nil {
			return err
		}
	}

	return nil
}

func (t *snapshotTransformer) finish() ([]PreparedTable, []Provenance, error) {
	// Resolve all legacy foreign-key values after every target ID exists. This
	// keeps the planner deterministic while allowing the target schema to use
	// PostgreSQL UUID columns instead of carrying legacy text IDs forward.
	for target, rows := range t.byTarget {
		for index := range rows {
			for field, referencedTable := range uuidReferenceTables[target] {
				value, ok := rows[index].Values[field].(string)
				if !ok || value == "" {
					continue
				}
				rows[index].Values[field] = targetUUID(referencedTable, value, t.now)
			}
		}
	}
	// Version snapshots can share row values. Hash only after every reference is final.
	for target, rows := range t.byTarget {
		for index := range rows {
			checksum, err := Checksum(rows[index].Values)
			if err != nil {
				return nil, nil, err
			}

			for provenanceIndex := range t.provenance {
				if t.provenance[provenanceIndex].TargetTable == target && t.provenance[provenanceIndex].TargetID == rows[index].ID {
					t.provenance[provenanceIndex].TransformedChecksum = checksum
				}
			}
		}
	}

	tables := make([]PreparedTable, 0, len(t.byTarget))
	for _, name := range targetOrder {
		rows := t.byTarget[name]
		if len(rows) == 0 {
			continue
		}
		sort.Slice(rows, func(left, right int) bool { return rows[left].ID < rows[right].ID })
		tables = append(tables, PreparedTable{Name: name, Rows: rows})
	}
	sort.Slice(t.provenance, func(left, right int) bool {
		l, r := t.provenance[left], t.provenance[right]
		return l.SourceTable+"\x00"+l.SourceID+"\x00"+l.TargetTable+"\x00"+l.TargetID < r.SourceTable+"\x00"+r.SourceID+"\x00"+r.TargetTable+"\x00"+r.TargetID
	})
	return tables, t.provenance, nil
}
