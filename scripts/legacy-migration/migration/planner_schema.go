package migration

import (
	"bytes"
	"crypto/sha256"
	"time"
)

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
	"user_profiles",
	"user_preferences",
	"user_contacts",
	"user_security",
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

var targetKeyColumn = map[string]string{
	"user_profiles":    "user_id",
	"user_preferences": "user_id",
	"user_contacts":    "user_id",
	"user_security":    "user_id",
}

// uuidReferenceTables maps transformed foreign-key fields to their target
// table. Polymorphic audit and provenance fields stay text by design.

var uuidReferenceTables = map[string]map[string]string{
	"user_auth_links":                    {},
	"user_profiles":                      {"user_id": "users"},
	"user_preferences":                   {"user_id": "users"},
	"user_contacts":                      {"user_id": "users"},
	"user_security":                      {"user_id": "users"},
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
	return newMigrationIDAt(at, bytes.NewReader(seed[:]))
}
