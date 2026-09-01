// Package dbtable contains schema-qualified PostgreSQL table names used by the
// runtime database adapters.
package dbtable

const (
	Users                          = "app.users"
	UserAuthLinks                  = "app.user_auth_links"
	IdentityEventReceipts          = "app.identity_event_receipts"
	GlobalRoleAssignments          = "app.global_role_assignments"
	Seasons                        = "app.seasons"
	SeasonWeeks                    = "app.season_weeks"
	SeasonCloseEvents              = "app.season_close_events"
	Enrollments                    = "app.enrollments"
	EnrollmentRemovalEvents        = "app.enrollment_removal_events"
	Mentorships                    = "app.mentorships"
	PracticeGoals                  = "app.practice_goals"
	Problems                       = "app.problems"
	LeetcodeProblems               = "app.leetcode_problems"
	LeetcodeCategories             = "app.leetcode_problem_categories"
	LeetcodeCategoryMappings       = "app.leetcode_problem_category_mappings"
	ProblemAttempts                = "app.problem_attempts"
	MockInterviews                 = "app.mock_interviews"
	MockInterviewRounds            = "app.mock_interview_rounds"
	BehaviouralMockInterviewRounds = "app.behavioural_mock_interview_rounds"
	LeetcodeMockInterviewRounds    = "app.leetcode_mock_interview_rounds"
	CustomMockInterviewRounds      = "app.custom_mock_interview_rounds"
	MockInterviewVersions          = "app.mock_interview_versions"
	AuditEvents                    = "app.audit_events"
	LeetcodeSyncRuns               = "app.leetcode_sync_runs"
	MigrationRuns                  = "migration.runs"
)
