package migration

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

var ErrSchemaContract = errors.New("legacy schema does not provide every column the planner reads")

var legacyPlannerColumns = map[string][]string{
	"BehaviouralMockInterviewRound": {"BehavioralScore", "CreatedAtUtc", "DeletedAtUtc", "UpdatedAtUtc"},
	"CustomMockInterviewRound":      {"Content", "CreatedAtUtc", "DeletedAtUtc", "Link", "Score", "UpdatedAtUtc"},
	"CustomProblem":                 {"CreatedAtUtc", "DeletedAtUtc", "Difficulty", "ProblemId", "Question", "UpdatedAtUtc"},
	"Enrollment":                    {"CreatedAtUtc", "DeletedAtUtc", "Role", "SeasonId", "StudentRolePromotion", "UpdatedAtUtc", "UserId"},
	"KickStudentEvent":              {"DeletedAtUtc", "KickReason", "KickedAtUtc", "MentorId", "SeasonId", "StudentId"},
	"LeetcodeMockInterviewRound":    {"AlgorithmDesignScore", "CodingScore", "ComplexityAnalysisScore", "ConfirmQuestionScore", "CreatedAtUtc", "DeletedAtUtc", "LeetcodeProblemId", "TestingScore", "UpdatedAtUtc"},
	"LeetcodeProblem":               {"CreatedAtUtc", "DeletedAtUtc", "IsPremium", "LeetcodeNumber", "LeetcodeProblemDifficulty", "ProblemId", "UpdatedAtUtc"},
	"LeetcodeProblemCategory":       {"CreatedAtUtc", "DeletedAtUtc", "Name", "UpdatedAtUtc"},
	"LeetcodeProblemCategoryMapping": {
		"LeetcodeProblemCategoriesLeetcodeProblemCategoryId",
		"LeetcodeProblemEntityLeetcodeProblemId",
	},
	"Mentorship":         {"CreatedAtUtc", "DeletedAtUtc", "MenteeEnrollmentId", "MentorEnrollmentId", "UpdatedAtUtc"},
	"MockInterview":      {"CreatedAtUtc", "DeletedAtUtc", "IntervieweeUserId", "InterviewerUserId", "IsPass", "Notes", "SeasonId", "SeasonWeekId", "StartDate", "TimeTakenInMinutes", "UpdatedAtUtc"},
	"MockInterviewRound": {"BehaviouralMockInterviewRoundId", "CreatedAtUtc", "CustomMockInterviewRoundId", "DeletedAtUtc", "IntervieweeComment", "IsReviewedByInterviewee", "LeetcodeMockInterviewRoundId", "MockInterviewId", "UpdatedAtUtc"},
	"Problem":            {"CreatedAtUtc", "DeletedAtUtc", "Link", "Title", "UpdatedAtUtc"},
	"ProblemAttempt":     {"AttemptStartDateUtc", "CreatedAtUtc", "CustomProblemId", "DeletedAtUtc", "EnrollmentId", "LeetcodeProblemId", "Notes", "SeasonWeekId", "TimeTakenInMinutes", "UpdatedAtUtc", "UserId"},
	"Season":             {"CreatedAtUtc", "DeletedAtUtc", "EndDateInclusiveUTC", "ImageUrl", "Location", "Name", "ResourcesUrl", "Slug", "StartDateInclusiveUTC", "UpdatedAtUtc"},
	"SeasonWeek":         {"CreatedAtUtc", "DeletedAtUtc", "EndDate", "SeasonId", "StartDate", "UpdatedAtUtc", "WeekNumber"},
	"User":               {"CreatedAtUtc", "DeletedAtUtc", "DiscordId", "Email", "IsAdmin", "IsGraduate", "Name", "ProfileImage", "Slug", "UserId"},
}

func AssertSchemaContract(schema []TableSchema) error {
	present := make(map[string]map[string]bool, len(schema))
	for _, table := range schema {
		columns := make(map[string]bool, len(table.Columns))
		for _, column := range table.Columns {
			columns[column.Name] = true
		}
		present[table.Name] = columns
	}

	missing := make([]string, 0)
	for table, columns := range legacyPlannerColumns {
		available, ok := present[table]
		if !ok {
			continue
		}
		for _, column := range columns {
			if !available[column] {
				missing = append(missing, table+"."+column)
			}
		}
	}
	if len(missing) == 0 {
		return nil
	}

	sort.Strings(missing)
	return fmt.Errorf("%w: %s", ErrSchemaContract, strings.Join(missing, ", "))
}
