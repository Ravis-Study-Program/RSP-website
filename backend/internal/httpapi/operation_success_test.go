package httpapi

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/magedmg/RSP-website/backend/internal/accounts"
	"github.com/magedmg/RSP-website/backend/internal/authz"
	"github.com/magedmg/RSP-website/backend/internal/generated"
	"github.com/magedmg/RSP-website/backend/internal/mockinterviews"
	"github.com/magedmg/RSP-website/backend/internal/model"
	"github.com/magedmg/RSP-website/backend/internal/practice"
	"github.com/magedmg/RSP-website/backend/internal/programme"
)

type operationSuccessCase struct {
	operationID string
	method      string
	path        string
	actor       string
	body        string
	want        int
	setup       func(*fixture)
}

func TestEveryProtectedOpenAPIOperationHasExecutablePrimarySuccess(t *testing.T) {
	base := time.Now().UTC().Truncate(time.Second)
	spec, err := generated.GetSwagger()
	if err != nil {
		t.Fatal(err)
	}

	seasonMutation := fmt.Sprintf(`{"name":"Updated season","slug":"updated-season","startAt":%q,"endAt":%q,"location":"Adelaide","imageUrl":"https://rsp.test/image","resourcesUrl":"https://rsp.test/resources","revision":1}`, base.Add(-24*time.Hour).Format(time.RFC3339), base.Add(30*24*time.Hour).Format(time.RFC3339))
	seasonCreate := strings.TrimSuffix(seasonMutation, `,"revision":1}`) + `}`
	weekMutation := fmt.Sprintf(`{"number":1,"startAt":%q,"endAt":%q,"resourceUrl":"https://rsp.test/week"}`, base.Format(time.RFC3339), base.Add(24*time.Hour).Format(time.RFC3339))
	attemptCreate := fmt.Sprintf(`{"problemId":"problem","outcome":"independently_solved","confidence":5,"minutes":12,"attemptedAt":%q}`, base.Format(time.RFC3339))
	attemptUpdate := strings.TrimSuffix(attemptCreate, `}`) + `,"revision":1}`
	mockMutation := fmt.Sprintf(`{"occurredAt":%q,"durationMinutes":60,"notes":"<p>notes</p>","rounds":[{"id":"round","type":"behavioural","scores":{"behavioural":7}}]}`, base.Format(time.RFC3339))
	mockCreate := fmt.Sprintf(`{"interviewee":{"userId":"other"},"occurredAt":%q,"durationMinutes":60,"notes":"<p>notes</p>","rounds":[{"id":"round","type":"behavioural","scores":{"behavioural":7}}]}`, base.Format(time.RFC3339))
	mockUpdate := strings.TrimSuffix(mockMutation, `}`) + `,"revision":1}`

	seedStudent := func(f *fixture) {
		f.repository.Enrollments["student-enrollment"] = model.Enrollment{ID: "student-enrollment", SeasonID: "season", UserID: "student", Role: "student", StudentLevel: "beginner", State: "active", AssignmentState: "active", Revision: 1}
	}
	seedWeek := func(f *fixture) {
		season := f.repository.Seasons["season"]
		season.StartAt, season.EndAt = base.Add(-24*time.Hour), base.Add(30*24*time.Hour)
		f.repository.Seasons["season"] = season
		f.repository.Weeks["week"] = model.Week{ID: "week", SeasonID: "season", Number: 1, StartAt: base, EndAt: base.Add(24 * time.Hour), ResourceURL: "https://rsp.test/week", Revision: 1}
	}
	seedMentorshipParties := func(f *fixture) {
		for _, userID := range []string{"mentor-a", "mentor-b"} {
			f.repository.Users[userID] = accounts.User{ID: userID, Slug: userID, Name: userID, AccountState: "active", Timezone: "Australia/Adelaide", Revision: 1}
			f.repository.Enrollments[userID+"-enrollment"] = model.Enrollment{ID: userID + "-enrollment", SeasonID: "season", UserID: userID, Role: "mentor", State: "active", AssignmentState: "active", Revision: 1}
		}
	}
	seedMentorship := func(f *fixture) {
		seedMentorshipParties(f)
		f.repository.Mentorships["mentorship"] = programme.MentorshipRecord{ID: "mentorship", SeasonID: "season", MentorUserID: "mentor-a", StudentUserID: "other", Revision: 1}
	}
	seedMock := func(f *fixture, interviewerID, intervieweeID string) {
		seedStudent(f)
		score := 7
		f.repository.Mocks["mock"] = mockinterviews.Interview{ID: "mock", InterviewerID: interviewerID, IntervieweeID: intervieweeID, OccurredAt: base, DurationMinutes: 60, Notes: "<p>notes</p>", Rounds: []mockinterviews.Round{{ID: "round", Type: mockinterviews.Behavioural, Scores: mockinterviews.Scores{Behavioural: &score}}}, Revision: 1}
		f.actors["other-actor"] = authz.Actor{UserID: "other", EmailVerified: true, AccountState: authz.AccountActive, GlobalRoles: map[authz.GlobalRole]bool{}, Enrollments: []authz.Enrollment{{SeasonID: "season", Role: authz.Student, State: authz.Active}}}
	}

	cases := []operationSuccessCase{
		{"getMe", http.MethodGet, "/api/v2/me", "student", "", http.StatusOK, nil},
		{"updateMe", http.MethodPatch, "/api/v2/me", "student", `{"name":"Updated student","slug":"updated-student","timezone":"Australia/Adelaide","revision":1}`, http.StatusOK, nil},
		{"suggestMeSlug", http.MethodGet, "/api/v2/me/slug-suggestion", "student", "", http.StatusOK, nil},
		{"getPracticeSettings", http.MethodGet, "/api/v2/me/practice-settings", "student", "", http.StatusOK, nil},
		{"updatePracticeSettings", http.MethodPatch, "/api/v2/me/practice-settings", "student", `{"premiumOptIn":false,"easyMinutes":20,"mediumMinutes":35,"hardMinutes":50,"revision":1}`, http.StatusOK, nil},
		{"listUsers", http.MethodGet, "/api/v2/users?limit=25&direction=forward&sort=id:asc", "student", "", http.StatusOK, nil},
		{"getUser", http.MethodGet, "/api/v2/users/other", "student", "", http.StatusOK, nil},
		{"enableUserPracticeGoals", http.MethodPost, "/api/v2/users/other/practice-goals/enable", "coordinator", `{"seasonId":"season","revision":1}`, http.StatusOK, nil},
		{"getUserPracticeSettings", http.MethodGet, "/api/v2/users/other/practice-settings", "coordinator", "", http.StatusOK, nil},
		{"listSeasons", http.MethodGet, "/api/v2/seasons?limit=25&direction=forward&sort=id:asc", "student", "", http.StatusOK, nil},
		{"createSeason", http.MethodPost, "/api/v2/seasons", "director", seasonCreate, http.StatusCreated, nil},
		{"getSeason", http.MethodGet, "/api/v2/seasons/season", "student", "", http.StatusOK, nil},
		{"updateSeason", http.MethodPatch, "/api/v2/seasons/season", "director", seasonMutation, http.StatusOK, nil},
		{"closeSeason", http.MethodPost, "/api/v2/seasons/season/close", "coordinator", `{"reason":"programme complete","revision":1}`, http.StatusOK, nil},
		{"updateSeasonResources", http.MethodPatch, "/api/v2/seasons/season/resources", "coordinator", `{"resourcesUrl":"https://rsp.test/new-resources","revision":1}`, http.StatusOK, nil},
		{"listEnrollmentCandidates", http.MethodGet, "/api/v2/seasons/season/enrollment-candidates?limit=25&direction=forward&sort=id:asc", "coordinator", "", http.StatusOK, nil},
		{"reopenSeason", http.MethodPost, "/api/v2/seasons/season/reopen", "director", `{"reason":"correction","revision":1}`, http.StatusOK, func(f *fixture) {
			season := f.repository.Seasons["season"]
			season.Status = "closed"
			f.repository.Seasons["season"] = season
		}},
		{"listSeasonWeeks", http.MethodGet, "/api/v2/seasons/season/weeks?limit=25&direction=forward&sort=number:asc", "student", "", http.StatusOK, nil},
		{"createSeasonWeek", http.MethodPost, "/api/v2/seasons/season/weeks", "coordinator", weekMutation, http.StatusCreated, func(f *fixture) {
			season := f.repository.Seasons["season"]
			season.StartAt, season.EndAt = base.Add(-24*time.Hour), base.Add(30*24*time.Hour)
			f.repository.Seasons["season"] = season
		}},
		{"updateSeasonWeek", http.MethodPatch, "/api/v2/seasons/season/weeks/week", "coordinator", strings.TrimSuffix(weekMutation, `}`) + `,"revision":1}`, http.StatusOK, seedWeek},
		{"deleteSeasonWeek", http.MethodDelete, "/api/v2/seasons/season/weeks/week?revision=1", "coordinator", "", http.StatusNoContent, seedWeek},
		{"listSeasonMembers", http.MethodGet, "/api/v2/seasons/season/members?limit=25&direction=forward&sort=id:asc", "student", "", http.StatusOK, nil},
		{"createSeasonMember", http.MethodPost, "/api/v2/seasons/season/members", "coordinator", `{"userId":"candidate","role":"student"}`, http.StatusCreated, func(f *fixture) {
			f.repository.Users["candidate"] = accounts.User{ID: "candidate", Slug: "candidate", Name: "Candidate", AccountState: "active", Timezone: "Australia/Adelaide", Revision: 1}
		}},
		{"promoteSeasonMember", http.MethodPost, "/api/v2/seasons/season/members/other-enrollment/promote", "coordinator", `{"role":"mentor","reason":"progression","revision":1}`, http.StatusOK, nil},
		{"updateSeasonMember", http.MethodPatch, "/api/v2/seasons/season/members/other-enrollment", "coordinator", `{"role":"student","studentLevel":"advanced","revision":1}`, http.StatusOK, nil},
		{"removeSeasonMember", http.MethodPost, "/api/v2/seasons/season/members/other-enrollment/remove", "coordinator", `{"reason":"withdrawal requested","revision":1}`, http.StatusOK, nil},
		{"listSeasonMentorships", http.MethodGet, "/api/v2/seasons/season/mentorships?limit=25&direction=forward&sort=id:asc", "student", "", http.StatusOK, nil},
		{"createMentorship", http.MethodPost, "/api/v2/seasons/season/mentorships", "coordinator", `{"mentorUserId":"mentor-a","studentUserId":"other"}`, http.StatusCreated, seedMentorshipParties},
		{"updateMentorship", http.MethodPatch, "/api/v2/seasons/season/mentorships/mentorship", "coordinator", `{"mentorUserId":"mentor-b","studentUserId":"other","revision":1}`, http.StatusOK, seedMentorship},
		{"deleteMentorship", http.MethodDelete, "/api/v2/seasons/season/mentorships/mentorship?revision=1", "coordinator", "", http.StatusNoContent, seedMentorship},
		{"listLeetcodeProblems", http.MethodGet, "/api/v2/leetcode-problems?limit=25&direction=forward&sort=id:asc", "student", "", http.StatusOK, nil},
		{"listProblemAttempts", http.MethodGet, "/api/v2/problem-attempts?limit=25&direction=forward&sort=id:asc", "student", "", http.StatusOK, nil},
		{"createProblemAttempt", http.MethodPost, "/api/v2/problem-attempts", "student", attemptCreate, http.StatusCreated, nil},
		{"updateProblemAttempt", http.MethodPatch, "/api/v2/problem-attempts/owned", "student", attemptUpdate, http.StatusOK, nil},
		{"deleteProblemAttempt", http.MethodDelete, "/api/v2/problem-attempts/owned?revision=1", "student", "", http.StatusNoContent, nil},
		{"getCurrentRecommendation", http.MethodGet, "/api/v2/recommendations/current", "student", "", http.StatusOK, func(f *fixture) {
			seedStudent(f)
			f.repository.Problems["candidate"] = model.Problem{ID: "candidate", Number: 2, Title: "Candidate", Link: "https://rsp.test/problems/candidate", Difficulty: "easy", Categories: []string{"arrays"}, Revision: 1}
		}},
		{"dismissCurrentRecommendation", http.MethodPost, "/api/v2/recommendations/current/dismiss", "student", `{"reason":"later","revision":1}`, http.StatusNoContent, func(f *fixture) {
			f.repository.Recommendations["student"] = practice.Recommendation{ID: "recommendation", UserID: "student", Problem: practice.Problem{ID: "problem", Title: "Two Sum", Difficulty: practice.Easy, Categories: []string{"arrays"}}, Difficulty: practice.Easy, Category: "arrays", Rationale: "Practice arrays", RuleVersion: "v1", CreatedAt: base, Revision: 1}
		}},
		{"listMockInterviewParticipants", http.MethodGet, "/api/v2/mock-interviews/participants?limit=25&direction=forward&sort=id:asc", "student", "", http.StatusOK, func(f *fixture) { seedStudent(f) }},
		{"listMockInterviews", http.MethodGet, "/api/v2/mock-interviews?limit=25&direction=forward&sort=occurredAt:desc&mode=received", "student", "", http.StatusOK, func(f *fixture) { seedStudent(f) }},
		{"createMockInterview", http.MethodPost, "/api/v2/mock-interviews", "student", mockCreate, http.StatusCreated, func(f *fixture) { seedStudent(f) }},
		{"updateMockInterview", http.MethodPatch, "/api/v2/mock-interviews/mock", "student", mockUpdate, http.StatusOK, func(f *fixture) { seedMock(f, "student", "other") }},
		{"deleteMockInterview", http.MethodDelete, "/api/v2/mock-interviews/mock?revision=1", "student", "", http.StatusNoContent, func(f *fixture) { seedMock(f, "student", "other") }},
		{"reviewMockInterviewRound", http.MethodPatch, "/api/v2/mock-interviews/mock/rounds/round/review", "other-actor", `{"reviewed":true,"comment":"reviewed","revision":1}`, http.StatusOK, func(f *fixture) { seedMock(f, "student", "other") }},
		{"correctMockInterviewIdentities", http.MethodPost, "/api/v2/mock-interviews/mock/identity-correction", "director", `{"interviewerId":"student","intervieweeId":"other","reason":"corrected identities","revision":1}`, http.StatusOK, func(f *fixture) { seedMock(f, "other", "student") }},
		{"requestLeetcodeSync", http.MethodPost, "/api/v2/admin/leetcode/sync", "director", "", http.StatusAccepted, nil},
		{"listAdminUsers", http.MethodGet, "/api/v2/admin/users?limit=25&direction=forward&sort=id:asc", "admin", "", http.StatusOK, nil},
		{"setAdminUserAccountState", http.MethodPost, "/api/v2/admin/users/other/account-state", "admin", `{"state":"suspended","reason":"security review","revision":1}`, http.StatusOK, nil},
		{"grantAdminUserGlobalRole", http.MethodPost, "/api/v2/admin/users/other/global-roles", "admin", `{"role":"director","reason":"programme oversight"}`, http.StatusCreated, nil},
		{"listAdminUserGlobalRoles", http.MethodGet, "/api/v2/admin/users/other/global-roles", "admin", "", http.StatusOK, nil},
		{"revokeAdminUserGlobalRole", http.MethodDelete, "/api/v2/admin/users/other/global-roles/director?revision=1&reason=role-ended", "admin", "", http.StatusOK, func(f *fixture) {
			f.repository.GlobalRoleAssignments["other-director"] = accounts.GlobalRoleAssignment{ID: "other-director", UserID: "other", Role: "director", State: "active", Revision: 1}
		}},
	}

	covered := make(map[string]bool, len(cases))
	for _, testCase := range cases {
		t.Run(testCase.operationID, func(t *testing.T) {
			f := newFixture()
			if testCase.setup != nil {
				testCase.setup(&f)
			}
			response := request(t, f, testCase.method, testCase.path, testCase.actor, testCase.body)
			if response.Code != testCase.want {
				t.Fatalf("status=%d want=%d body=%s", response.Code, testCase.want, response.Body.String())
			}
			assertOpenAPIResponse(t, spec, testCase.method, testCase.path, response)
			covered[strings.ToLower(testCase.operationID)] = true
		})
	}

	public := map[string]bool{"getliveness": true, "getreadiness": true, "getmetrics": true, "getopenapi": true}
	for _, item := range spec.Paths.Map() {
		for _, operation := range item.Operations() {
			operationID := strings.ToLower(operation.OperationID)
			if !public[operationID] && !covered[operationID] {
				t.Errorf("protected OpenAPI operation %s has no primary-success execution", operation.OperationID)
			}
		}
	}
}

func TestMutableResourceFamiliesRejectStaleRevisionsWithCompleteProblems(t *testing.T) {
	base := time.Now().UTC().Truncate(time.Second)
	seasonBody := fmt.Sprintf(`{"name":"Season","slug":"season","startAt":%q,"endAt":%q,"location":"Adelaide","imageUrl":"https://rsp.test/image","resourcesUrl":"https://rsp.test/resources","revision":99}`, base.Add(-time.Hour).Format(time.RFC3339), base.Add(48*time.Hour).Format(time.RFC3339))
	attemptBody := fmt.Sprintf(`{"problemId":"problem","outcome":"independently_solved","confidence":5,"minutes":12,"attemptedAt":%q,"revision":99}`, base.Format(time.RFC3339))
	mockBody := fmt.Sprintf(`{"occurredAt":%q,"durationMinutes":60,"rounds":[{"id":"round","type":"behavioural","scores":{"behavioural":7}}],"revision":99}`, base.Format(time.RFC3339))
	cases := []operationSuccessCase{
		{"profile", http.MethodPatch, "/api/v2/me", "student", `{"name":"Student","timezone":"Australia/Adelaide","revision":99}`, http.StatusConflict, nil},
		{"practice settings", http.MethodPatch, "/api/v2/me/practice-settings", "student", `{"premiumOptIn":false,"easyMinutes":20,"mediumMinutes":35,"hardMinutes":50,"revision":99}`, http.StatusConflict, nil},
		{"practice goal enablement", http.MethodPost, "/api/v2/users/other/practice-goals/enable", "coordinator", `{"seasonId":"season","revision":99}`, http.StatusConflict, nil},
		{"season definition", http.MethodPatch, "/api/v2/seasons/season", "director", seasonBody, http.StatusConflict, nil},
		{"season close", http.MethodPost, "/api/v2/seasons/season/close", "coordinator", `{"reason":"complete","revision":99}`, http.StatusConflict, nil},
		{"week", http.MethodPatch, "/api/v2/seasons/season/weeks/week", "coordinator", fmt.Sprintf(`{"number":1,"startAt":%q,"endAt":%q,"resourceUrl":"https://rsp.test/week","revision":99}`, base.Format(time.RFC3339), base.Add(time.Hour).Format(time.RFC3339)), http.StatusConflict, func(f *fixture) {
			season := f.repository.Seasons["season"]
			season.StartAt, season.EndAt = base.Add(-time.Hour), base.Add(48*time.Hour)
			f.repository.Seasons["season"] = season
			f.repository.Weeks["week"] = model.Week{ID: "week", SeasonID: "season", Number: 1, StartAt: base, EndAt: base.Add(time.Hour), Revision: 1}
		}},
		{"enrollment", http.MethodPatch, "/api/v2/seasons/season/members/other-enrollment", "coordinator", `{"role":"student","studentLevel":"advanced","revision":99}`, http.StatusConflict, nil},
		{"mentorship", http.MethodPatch, "/api/v2/seasons/season/mentorships/mentorship", "coordinator", `{"mentorUserId":"mentor","studentUserId":"other","revision":99}`, http.StatusConflict, func(f *fixture) {
			f.repository.Users["mentor"] = accounts.User{ID: "mentor", Slug: "mentor", Name: "Mentor", AccountState: "active", Revision: 1}
			f.repository.Enrollments["mentor-enrollment"] = model.Enrollment{ID: "mentor-enrollment", SeasonID: "season", UserID: "mentor", Role: "mentor", State: "active", AssignmentState: "active", Revision: 1}
			f.repository.Mentorships["mentorship"] = programme.MentorshipRecord{ID: "mentorship", SeasonID: "season", MentorUserID: "mentor", StudentUserID: "other", Revision: 1}
		}},
		{"attempt", http.MethodPatch, "/api/v2/problem-attempts/owned", "student", attemptBody, http.StatusConflict, nil},
		{"recommendation dismissal", http.MethodPost, "/api/v2/recommendations/current/dismiss", "student", `{"reason":"later","revision":99}`, http.StatusConflict, func(f *fixture) {
			f.repository.Recommendations["student"] = practice.Recommendation{ID: "recommendation", UserID: "student", Problem: practice.Problem{ID: "problem", Difficulty: practice.Easy}, Difficulty: practice.Easy, Rationale: "Practice", RuleVersion: "v1", CreatedAt: base, Revision: 1}
		}},
		{"mock interview", http.MethodPatch, "/api/v2/mock-interviews/mock", "student", mockBody, http.StatusConflict, func(f *fixture) {
			score := 7
			f.repository.Enrollments["student-enrollment"] = model.Enrollment{ID: "student-enrollment", SeasonID: "season", UserID: "student", Role: "student", State: "active", AssignmentState: "active", Revision: 1}
			f.repository.Mocks["mock"] = mockinterviews.Interview{ID: "mock", InterviewerID: "student", IntervieweeID: "other", OccurredAt: base, DurationMinutes: 60, Rounds: []mockinterviews.Round{{ID: "round", Type: mockinterviews.Behavioural, Scores: mockinterviews.Scores{Behavioural: &score}}}, Revision: 1}
		}},
		{"account lifecycle", http.MethodPost, "/api/v2/admin/users/other/account-state", "admin", `{"state":"suspended","reason":"review","revision":99}`, http.StatusConflict, nil},
		{"global role", http.MethodDelete, "/api/v2/admin/users/other/global-roles/director?revision=99&reason=ended", "admin", "", http.StatusConflict, func(f *fixture) {
			f.repository.GlobalRoleAssignments["other-director"] = accounts.GlobalRoleAssignment{ID: "other-director", UserID: "other", Role: "director", State: "active", Revision: 1}
		}},
	}
	for _, testCase := range cases {
		t.Run(testCase.operationID, func(t *testing.T) {
			f := newFixture()
			if testCase.setup != nil {
				testCase.setup(&f)
			}
			response := request(t, f, testCase.method, testCase.path, testCase.actor, testCase.body)
			if response.Code != testCase.want || problemCode(t, response) != "stale_revision" {
				t.Fatalf("status=%d want=%d body=%s", response.Code, testCase.want, response.Body.String())
			}
		})
	}
}
