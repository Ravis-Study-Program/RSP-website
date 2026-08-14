package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	legacyrouter "github.com/getkin/kin-openapi/routers/legacy"
	"github.com/magedmg/RSP-website/backend/internal/generated"
	"github.com/magedmg/RSP-website/backend/internal/model"
)

func assertOpenAPIResponse(t *testing.T, spec *openapi3.T, method, path string, response *httptest.ResponseRecorder) {
	t.Helper()
	router, err := legacyrouter.NewRouter(spec)
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(method, path, nil)
	route, pathParams, err := router.FindRoute(request)
	if err != nil {
		t.Fatalf("find OpenAPI route: %v", err)
	}
	input := &openapi3filter.ResponseValidationInput{
		RequestValidationInput: &openapi3filter.RequestValidationInput{Request: request, PathParams: pathParams, Route: route},
		Status:                 response.Code,
		Header:                 response.Header().Clone(),
		Options:                &openapi3filter.Options{IncludeResponseStatus: true},
	}
	input.SetBodyBytes(response.Body.Bytes())
	if err := openapi3filter.ValidateResponse(context.Background(), input); err != nil {
		t.Fatalf("response violates OpenAPI status/content schema: %v\nbody=%s", err, response.Body.String())
	}
}

func TestEveryOpenAPIOperationHasExecutableRouteAndAuthenticationCoverage(t *testing.T) {
	season := `{"name":"Season","slug":"season","startAt":"2026-08-01T00:00:00Z","endAt":"2026-08-31T00:00:00Z","location":"Adelaide","imageUrl":"https://rsp.test/image","resourcesUrl":"https://rsp.test/resources"}`
	week := `{"number":1,"startAt":"2026-08-01T00:00:00Z","endAt":"2026-08-07T00:00:00Z","resourceUrl":"https://rsp.test/week"}`
	attempt := `{"problemId":"problem","outcome":"independently_solved","confidence":5,"minutes":12,"attemptedAt":"2026-08-13T00:00:00Z"}`
	mockCreate := `{"interviewee":{"userId":"other"},"occurredAt":"2026-08-13T00:00:00Z","durationMinutes":60,"rounds":[{"id":"round","type":"behavioural","scores":{"behavioural":7}}]}`
	mockUpdate := `{"occurredAt":"2026-08-13T00:00:00Z","durationMinutes":60,"rounds":[{"id":"round","type":"behavioural","scores":{"behavioural":7}}],"revision":1}`
	cases := []struct {
		operationID string
		method      string
		path        string
		body        string
		public      bool
	}{
		{"getLiveness", http.MethodGet, "/api/v2/health/live", "", true},
		{"getReadiness", http.MethodGet, "/api/v2/health/ready", "", true},
		{"getMetrics", http.MethodGet, "/api/v2/metrics", "", true},
		{"getOpenAPI", http.MethodGet, "/api/v2/openapi.json", "", true},
		{"getMe", http.MethodGet, "/api/v2/me", "", false},
		{"updateMe", http.MethodPatch, "/api/v2/me", `{"name":"Student","timezone":"Australia/Adelaide","revision":1}`, false},
		{"suggestMeSlug", http.MethodGet, "/api/v2/me/slug-suggestion", "", false},
		{"getPracticeSettings", http.MethodGet, "/api/v2/me/practice-settings", "", false},
		{"updatePracticeSettings", http.MethodPatch, "/api/v2/me/practice-settings", `{"premiumOptIn":false,"easyMinutes":20,"mediumMinutes":35,"hardMinutes":50,"revision":1}`, false},
		{"listUsers", http.MethodGet, "/api/v2/users?limit=25&direction=forward&sort=id:asc", "", false},
		{"getUser", http.MethodGet, "/api/v2/users/user", "", false},
		{"enableUserPracticeGoals", http.MethodPost, "/api/v2/users/user/practice-goals/enable", `{"seasonId":"season","revision":1}`, false},
		{"getUserPracticeSettings", http.MethodGet, "/api/v2/users/user/practice-settings", "", false},
		{"listSeasons", http.MethodGet, "/api/v2/seasons?limit=25&direction=forward&sort=id:asc", "", false},
		{"createSeason", http.MethodPost, "/api/v2/seasons", season, false},
		{"getSeason", http.MethodGet, "/api/v2/seasons/season", "", false},
		{"updateSeason", http.MethodPatch, "/api/v2/seasons/season", strings.TrimSuffix(season, "}") + `,"revision":1}`, false},
		{"closeSeason", http.MethodPost, "/api/v2/seasons/season/close", `{"reason":"complete","revision":1}`, false},
		{"updateSeasonResources", http.MethodPatch, "/api/v2/seasons/season/resources", `{"resourcesUrl":"https://rsp.test/resources","revision":1}`, false},
		{"listEnrollmentCandidates", http.MethodGet, "/api/v2/seasons/season/enrollment-candidates?limit=25&direction=forward&sort=id:asc", "", false},
		{"reopenSeason", http.MethodPost, "/api/v2/seasons/season/reopen", `{"reason":"correction","revision":1}`, false},
		{"listSeasonWeeks", http.MethodGet, "/api/v2/seasons/season/weeks?limit=25&direction=forward&sort=id:asc", "", false},
		{"createSeasonWeek", http.MethodPost, "/api/v2/seasons/season/weeks", week, false},
		{"updateSeasonWeek", http.MethodPatch, "/api/v2/seasons/season/weeks/week", strings.TrimSuffix(week, "}") + `,"revision":1}`, false},
		{"deleteSeasonWeek", http.MethodDelete, "/api/v2/seasons/season/weeks/week?revision=1", "", false},
		{"listSeasonMembers", http.MethodGet, "/api/v2/seasons/season/members?limit=25&direction=forward&sort=id:asc", "", false},
		{"createSeasonMember", http.MethodPost, "/api/v2/seasons/season/members", `{"userId":"user","role":"student"}`, false},
		{"promoteSeasonMember", http.MethodPost, "/api/v2/seasons/season/members/member/promote", `{"role":"mentor","revision":1}`, false},
		{"updateSeasonMember", http.MethodPatch, "/api/v2/seasons/season/members/member", `{"role":"student","studentLevel":"beginner","revision":1}`, false},
		{"removeSeasonMember", http.MethodPost, "/api/v2/seasons/season/members/member/remove", `{"reason":"withdrawn","revision":1}`, false},
		{"listSeasonMentorships", http.MethodGet, "/api/v2/seasons/season/mentorships?limit=25&direction=forward&sort=id:asc", "", false},
		{"createMentorship", http.MethodPost, "/api/v2/seasons/season/mentorships", `{"mentorUserId":"mentor","studentUserId":"student"}`, false},
		{"updateMentorship", http.MethodPatch, "/api/v2/seasons/season/mentorships/mentorship", `{"mentorUserId":"mentor","studentUserId":"student","revision":1}`, false},
		{"deleteMentorship", http.MethodDelete, "/api/v2/seasons/season/mentorships/mentorship?revision=1", "", false},
		{"listLeetcodeProblems", http.MethodGet, "/api/v2/leetcode-problems?limit=25&direction=forward&sort=id:asc", "", false},
		{"listProblemAttempts", http.MethodGet, "/api/v2/problem-attempts?limit=25&direction=forward&sort=id:asc", "", false},
		{"createProblemAttempt", http.MethodPost, "/api/v2/problem-attempts", attempt, false},
		{"updateProblemAttempt", http.MethodPatch, "/api/v2/problem-attempts/attempt", strings.TrimSuffix(attempt, "}") + `,"revision":1}`, false},
		{"deleteProblemAttempt", http.MethodDelete, "/api/v2/problem-attempts/attempt?revision=1", "", false},
		{"getCurrentRecommendation", http.MethodGet, "/api/v2/recommendations/current", "", false},
		{"dismissCurrentRecommendation", http.MethodPost, "/api/v2/recommendations/current/dismiss", `{"reason":"later","revision":1}`, false},
		{"listMockInterviewParticipants", http.MethodGet, "/api/v2/mock-interviews/participants?limit=25&direction=forward&sort=id:asc", "", false},
		{"listMockInterviews", http.MethodGet, "/api/v2/mock-interviews?limit=25&direction=forward&sort=date:desc&mode=received", "", false},
		{"createMockInterview", http.MethodPost, "/api/v2/mock-interviews", mockCreate, false},
		{"updateMockInterview", http.MethodPatch, "/api/v2/mock-interviews/mock", mockUpdate, false},
		{"deleteMockInterview", http.MethodDelete, "/api/v2/mock-interviews/mock?revision=1", "", false},
		{"reviewMockInterviewRound", http.MethodPatch, "/api/v2/mock-interviews/mock/rounds/round/review", `{"reviewed":true,"comment":"reviewed","revision":1}`, false},
		{"correctMockInterviewIdentities", http.MethodPost, "/api/v2/mock-interviews/mock/identity-correction", `{"interviewerId":"mentor","intervieweeId":"student","reason":"correction","revision":1}`, false},
		{"requestLeetcodeSync", http.MethodPost, "/api/v2/admin/leetcode/sync", "", false},
		{"listAdminUsers", http.MethodGet, "/api/v2/admin/users?limit=25&direction=forward&sort=id:asc", "", false},
		{"setAdminUserAccountState", http.MethodPost, "/api/v2/admin/users/user/account-state", `{"state":"suspended","reason":"security review","revision":1}`, false},
		{"grantAdminUserGlobalRole", http.MethodPost, "/api/v2/admin/users/user/global-roles", `{"role":"director","reason":"programme oversight"}`, false},
		{"listAdminUserGlobalRoles", http.MethodGet, "/api/v2/admin/users/user/global-roles", "", false},
		{"revokeAdminUserGlobalRole", http.MethodDelete, "/api/v2/admin/users/user/global-roles/director?revision=1&reason=role-ended", "", false},
	}

	spec, err := generated.GetSwagger()
	if err != nil {
		t.Fatal(err)
	}
	contractOperations := map[string]bool{}
	for _, item := range spec.Paths.Map() {
		for _, operation := range item.Operations() {
			contractOperations[strings.ToLower(operation.OperationID)] = true
		}
	}
	covered := map[string]bool{}
	fixture := newFixture()
	for _, testCase := range cases {
		t.Run(testCase.operationID, func(t *testing.T) {
			operationKey := strings.ToLower(testCase.operationID)
			if covered[operationKey] {
				t.Fatalf("duplicate executable case for %s", testCase.operationID)
			}
			covered[operationKey] = true
			response := request(t, fixture, testCase.method, testCase.path, "", testCase.body)
			assertOpenAPIResponse(t, spec, testCase.method, testCase.path, response)
			if testCase.public {
				if response.Code != http.StatusOK {
					t.Fatalf("public operation status=%d body=%s", response.Code, response.Body.String())
				}
				return
			}
			if response.Code != http.StatusUnauthorized || problemCode(t, response) != "authentication_required" {
				t.Fatalf("protected operation status=%d body=%s", response.Code, response.Body.String())
			}
			var details map[string]any
			if err := json.Unmarshal(response.Body.Bytes(), &details); err != nil {
				t.Fatal(err)
			}
			if details["instance"] != strings.Split(testCase.path, "?")[0] {
				t.Fatalf("problem instance=%v path=%s", details["instance"], testCase.path)
			}
		})
	}
	for operationID := range contractOperations {
		if !covered[operationID] {
			t.Errorf("OpenAPI operation %s has no executable route case", operationID)
		}
	}
	for operationID := range covered {
		if !contractOperations[operationID] {
			t.Errorf("executable route case %s is absent from OpenAPI", operationID)
		}
	}
}

func TestGrowingCollectionsReturnCompleteExecutablePageEnvelope(t *testing.T) {
	fixture := newFixture()
	fixture.repository.Enrollments["student-member"] = model.Enrollment{ID: "student-member", SeasonID: "season", UserID: "student", Role: "student", State: "active", Revision: 1}
	cases := []struct {
		name  string
		path  string
		actor string
	}{
		{"users", "/api/v2/users?limit=1&direction=forward&sort=id:asc", "student"},
		{"seasons", "/api/v2/seasons?limit=1&direction=forward&sort=id:asc", "student"},
		{"enrollment candidates", "/api/v2/seasons/season/enrollment-candidates?limit=1&direction=forward&sort=id:asc", "coordinator"},
		{"weeks", "/api/v2/seasons/season/weeks?limit=1&direction=forward&sort=id:asc", "student"},
		{"members", "/api/v2/seasons/season/members?limit=1&direction=forward&sort=id:asc", "student"},
		{"mentorships", "/api/v2/seasons/season/mentorships?limit=1&direction=forward&sort=id:asc", "student"},
		{"problems", "/api/v2/leetcode-problems?limit=1&direction=forward&sort=id:asc", "student"},
		{"attempts", "/api/v2/problem-attempts?limit=1&direction=forward&sort=id:asc", "student"},
		{"mock participants", "/api/v2/mock-interviews/participants?limit=1&direction=forward&sort=id:asc", "student"},
		{"mock interviews", "/api/v2/mock-interviews?limit=1&direction=forward&sort=id:asc&mode=received", "student"},
		{"admin users", "/api/v2/admin/users?limit=1&direction=forward&sort=id:asc", "admin"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			response := request(t, fixture, http.MethodGet, testCase.path, testCase.actor, "")
			if response.Code != http.StatusOK {
				t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
			}
			var page map[string]json.RawMessage
			if err := json.Unmarshal(response.Body.Bytes(), &page); err != nil {
				t.Fatal(err)
			}
			for _, field := range []string{"items", "pageInfo", "totalCount"} {
				if _, ok := page[field]; !ok {
					t.Fatalf("page missing %s: %s", field, response.Body.String())
				}
			}
			var pageInfo map[string]json.RawMessage
			if err := json.Unmarshal(page["pageInfo"], &pageInfo); err != nil {
				t.Fatal(err)
			}
			for _, field := range []string{"nextCursor", "previousCursor", "hasMore"} {
				if _, ok := pageInfo[field]; !ok {
					t.Fatalf("pageInfo missing %s: %s", field, response.Body.String())
				}
			}
		})
	}
}
