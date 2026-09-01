package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/magedmg/RSP-website/backend/internal/accounts"
	"github.com/magedmg/RSP-website/backend/internal/authz"
	"github.com/magedmg/RSP-website/backend/internal/mockinterviews"
	"github.com/magedmg/RSP-website/backend/internal/practice"
	"github.com/magedmg/RSP-website/backend/internal/programme"
	"github.com/magedmg/RSP-website/backend/internal/store"
)

type fixture struct {
	api        *API
	repository *store.Memory
	handler    http.Handler
	actors     map[string]authz.Actor
}

func newFixture() fixture {
	repo := store.NewMemory()
	repo.Users["student"] = accounts.User{ID: "student", Slug: "student", Name: "Student", Email: "private@example.com", AccountState: "active", Timezone: "Australia/Adelaide", Revision: 1}
	repo.Users["other"] = accounts.User{ID: "other", Slug: "other", Name: "Other", Email: "other@example.com", AccountState: "active", Timezone: "Australia/Adelaide", Revision: 1}
	repo.Seasons["season"] = programme.SeasonRecord{ID: "season", Slug: "s26", Name: "Season", Status: "open", StartAt: time.Now(), EndAt: time.Now().Add(24 * time.Hour), Revision: 1}
	repo.Enrollments["other-enrollment"] = programme.EnrollmentRecord{ID: "other-enrollment", SeasonID: "season", UserID: "other", Role: "student", State: "active", Revision: 1}
	repo.Problems["problem"] = practice.ProblemRecord{ID: "problem", Number: 1, Title: "Two Sum", Link: "https://rsp.test/problems/two-sum", Difficulty: "easy", Categories: []string{"arrays"}, Revision: 1}
	repo.Attempts["owned"] = practice.AttemptRecord{ID: "owned", UserID: "student", ProblemID: "problem", Outcome: "unknown", Minutes: 10, AttemptedAt: time.Now(), Revision: 1}
	repo.Attempts["foreign"] = practice.AttemptRecord{ID: "foreign", UserID: "other", ProblemID: "problem", Outcome: "unknown", Minutes: 10, AttemptedAt: time.Now(), Revision: 1}
	recent := time.Now().Add(-time.Minute)
	actors := map[string]authz.Actor{
		"student":       {UserID: "student", EmailVerified: true, AccountState: authz.AccountActive, GlobalRoles: map[authz.GlobalRole]bool{}, Enrollments: []authz.Enrollment{{SeasonID: "season", Role: authz.Student, State: authz.Active}}},
		"unverified":    {UserID: "student", EmailVerified: false, AccountState: authz.AccountActive},
		"nonmember":     {UserID: "student", EmailVerified: true, AccountState: authz.AccountActive},
		"kicked":        {UserID: "student", EmailVerified: true, AccountState: authz.AccountActive, Enrollments: []authz.Enrollment{{SeasonID: "season", Role: authz.Student, State: authz.Kicked}}},
		"withdrawn":     {UserID: "student", EmailVerified: true, AccountState: authz.AccountActive, Enrollments: []authz.Enrollment{{SeasonID: "season", Role: authz.Student, State: authz.Withdrawn}}},
		"graduate":      {UserID: "student", EmailVerified: true, AccountState: authz.AccountActive, Enrollments: []authz.Enrollment{{SeasonID: "season", Role: authz.Student, State: authz.Completed}}},
		"former-mentor": {UserID: "student", EmailVerified: true, AccountState: authz.AccountActive, Enrollments: []authz.Enrollment{{SeasonID: "season", Role: authz.Mentor, State: authz.Completed}}},
		"coordinator":   {UserID: "student", EmailVerified: true, AccountState: authz.AccountActive, GlobalRoles: map[authz.GlobalRole]bool{}, Enrollments: []authz.Enrollment{{SeasonID: "season", Role: authz.Coordinator, State: authz.Active}}, MFAAt: &recent},
		"director":      {UserID: "student", EmailVerified: true, AccountState: authz.AccountActive, GlobalRoles: map[authz.GlobalRole]bool{authz.Director: true}, MFAAt: &recent},
		"admin":         {UserID: "student", EmailVerified: true, AccountState: authz.AccountActive, GlobalRoles: map[authz.GlobalRole]bool{authz.SystemAdmin: true}, MFAAt: &recent},
		"suspended":     {UserID: "student", EmailVerified: true, AccountState: authz.Suspended},
		"deleted":       {UserID: "student", EmailVerified: true, AccountState: authz.Deleted},
	}
	auth := AuthenticatorFunc(func(r *http.Request) (authz.Actor, error) {
		a, ok := actors[r.Header.Get("X-Test-Actor")]
		if !ok {
			return authz.Actor{}, errors.New("missing token")
		}
		return a, nil
	})
	for subject, actor := range map[string]authz.Actor{
		"auth-student": actors["student"],
		"auth-other":   {UserID: "other", EmailVerified: true, AccountState: authz.AccountActive, GlobalRoles: map[authz.GlobalRole]bool{}},
	} {
		actor.SecurityVersion = 1
		repo.AuthSubjects[subject] = actor
	}
	setAccountState := func(ctx context.Context, subject, state, reason, actorUserID string) error {
		target, err := repo.ResolveAuthSubject(ctx, subject)
		if err != nil {
			return err
		}

		now := time.Now().UTC()
		return repo.ApplyIdentityEvent(ctx, accounts.IdentityEvent{EventID: "test-state-" + subject + "-" + state + "-" + now.Format(time.RFC3339Nano), Type: "account_state_changed", AuthUserID: subject, AccountState: state, Reason: reason, ActorUserID: actorUserID, SecurityVersion: target.SecurityVersion + 1, OccurredAt: now})
	}
	getMFAState := func(ctx context.Context, subject string) (bool, error) {
		target, err := repo.ResolveAuthSubject(ctx, subject)
		if err != nil {
			return false, err
		}
		return repo.MFAConfigured[target.UserID], nil
	}
	api := New(Config{Store: repo, Authenticator: auth, PublicOrigin: "https://rsp.test", CursorSecret: []byte("0123456789abcdef"), SyncLeetCode: func(context.Context, string, string) error { return nil }, SetAccountState: setAccountState, GetMFAState: getMFAState})
	return fixture{api, repo, api.Handler(), actors}
}

func TestPracticePermissionMatrix(t *testing.T) {
	f := newFixture()
	for actor, want := range map[string]int{
		"": http.StatusUnauthorized, "unverified": http.StatusForbidden, "nonmember": http.StatusForbidden,
		"kicked": http.StatusForbidden, "withdrawn": http.StatusForbidden, "student": http.StatusOK,
		"graduate": http.StatusOK, "director": http.StatusOK, "admin": http.StatusOK,
		"suspended": http.StatusForbidden, "deleted": http.StatusForbidden,
	} {
		w := request(t, f, "GET", "/api/v2/leetcode-problems", actor, "")
		if w.Code != want {
			t.Errorf("actor %q status=%d want=%d body=%s", actor, w.Code, want, w.Body.String())
		}
	}
}

func request(t *testing.T, f fixture, method, path, actor, body string) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	if actor != "" {
		r.Header.Set("X-Test-Actor", actor)
	}
	if body != "" {
		r.Header.Set("Content-Type", "application/json")
	}
	w := httptest.NewRecorder()
	f.handler.ServeHTTP(w, r)
	return w
}

func errorCode(t *testing.T, w *httptest.ResponseRecorder) string {
	t.Helper()
	if got := w.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("content type %q body %s", got, w.Body.String())
	}
	var p map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &p); err != nil {
		t.Fatal(err)
	}

	for _, key := range []string{"code", "message", "requestId"} {
		if _, ok := p[key]; !ok {
			t.Fatalf("missing %s in %#v", key, p)
		}
	}
	if len(p) != 3 {
		t.Fatalf("unexpected error response fields: %#v", p)
	}
	return p["code"].(string)
}

func TestRequestContextNormalizesRequestIDAndRecoversWithCompleteError(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := requestContext(logger, func(int, time.Duration) {}, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("boom")
	}))
	r := httptest.NewRequest("GET", "/panic", nil)
	r.Header.Set("X-Request-ID", strings.Repeat("x", 200)+"\nunsafe")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	if w.Code != http.StatusInternalServerError || w.Header().Get("X-Request-ID") == r.Header.Get("X-Request-ID") {
		t.Fatalf("status=%d requestID=%q", w.Code, w.Header().Get("X-Request-ID"))
	}
	if got := errorCode(t, w); got != "internal_error" {
		t.Fatalf("code=%s body=%s", got, w.Body.String())
	}

	var details map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &details); err != nil {
		t.Fatal(err)
	}
	if details["message"] == "" || details["requestId"] != w.Header().Get("X-Request-ID") {
		t.Fatalf("incomplete error response: %#v", details)
	}
}

func TestRequestBodyLimitRunsBeforeOpenAPIValidation(t *testing.T) {
	f := newFixture()
	body := strings.Repeat("x", int(maxRequestBodyBytes)+1)
	w := request(t, f, http.MethodPatch, "/api/v2/me", "student", body)

	if w.Code != http.StatusBadRequest || errorCode(t, w) != "invalid_request_body" {
		t.Fatalf("oversized body: %d %s", w.Code, w.Body.String())
	}
	if f.repository.Users["student"].Name != "Student" {
		t.Fatal("oversized request reached the endpoint handler")
	}
}

func TestHealthAndAuthenticationProblemContract(t *testing.T) {
	f := newFixture()
	if w := request(t, f, "GET", "/api/v2/health/live", "", ""); w.Code != 200 {
		t.Fatal(w.Code)
	}
	w := request(t, f, "GET", "/api/v2/me", "", "")
	if w.Code != 401 || errorCode(t, w) != "authentication_required" {
		t.Fatalf("got %d %s", w.Code, w.Body.String())
	}
	if bytes.Contains(w.Body.Bytes(), []byte("missing token")) || !bytes.Contains(w.Body.Bytes(), []byte("valid access token")) {
		t.Fatalf("authentication cause leaked or stable detail missing: %s", w.Body.String())
	}
	w = request(t, f, "GET", "/api/v2/me", "suspended", "")
	if w.Code != 403 || errorCode(t, w) != "account_unavailable" {
		t.Fatal(w.Code)
	}
}

func TestPrivateFieldsAndActorIDSpoofing(t *testing.T) {
	f := newFixture()
	w := request(t, f, "GET", "/api/v2/users/other", "student", "")
	if w.Code != 200 || bytes.Contains(w.Body.Bytes(), []byte("other@example.com")) {
		t.Fatalf("private field leaked: %s", w.Body.String())
	}
	body := `{"problemId":"problem","outcome":"independently_solved","confidence":5,"minutes":12,"attemptedAt":"2026-08-13T00:00:00Z","userId":"other"}`
	w = request(t, f, "POST", "/api/v2/problem-attempts", "student", body)
	if w.Code != 400 || errorCode(t, w) != "openapi_validation_failed" {
		t.Fatalf("actor id accepted: %d %s", w.Code, w.Body.String())
	}
}

func TestProfileUpdateChangesOnlyActorAndUsesServerUniqueSlugSuggestion(t *testing.T) {
	f := newFixture()
	w := request(t, f, http.MethodGet, "/api/v2/me/slug-suggestion", "student", "")
	if w.Code != http.StatusOK {
		t.Fatalf("slug suggestion: %d %s", w.Code, w.Body.String())
	}
	var suggestion struct {
		Slug string `json:"slug"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &suggestion); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(suggestion.Slug, "member-") || len(suggestion.Slug) < 3 || len(suggestion.Slug) > 50 {
		t.Fatalf("invalid suggested slug %q", suggestion.Slug)
	}
	body := `{"name":"Updated Student","slug":"` + suggestion.Slug + `","timezone":"Australia/Adelaide","revision":1}`
	w = request(t, f, http.MethodPatch, "/api/v2/me", "student", body)
	if w.Code != http.StatusOK || !bytes.Contains(w.Body.Bytes(), []byte(`"slug":"`+suggestion.Slug+`"`)) {
		t.Fatalf("profile update: %d %s", w.Code, w.Body.String())
	}
	if f.repository.Users["other"].Slug != "other" || f.repository.Users["other"].Name != "Other" {
		t.Fatalf("another profile changed: %#v", f.repository.Users["other"])
	}
	w = request(t, f, http.MethodPatch, "/api/v2/me", "student", `{"name":"Updated Student","slug":"other","timezone":"Australia/Adelaide","revision":2}`)
	if w.Code != http.StatusConflict || errorCode(t, w) != "duplicate" {
		t.Fatalf("duplicate slug update: %d %s", w.Code, w.Body.String())
	}
}

func TestFormerMemberCanUseOwnPracticeButNotAnotherMembersHistory(t *testing.T) {
	f := newFixture()
	w := request(t, f, "GET", "/api/v2/problem-attempts", "former-mentor", "")
	if w.Code != http.StatusOK || !bytes.Contains(w.Body.Bytes(), []byte(`"id":"owned"`)) {
		t.Fatalf("former member own practice: %d %s", w.Code, w.Body.String())
	}
	w = request(t, f, "GET", "/api/v2/problem-attempts?userId=other", "former-mentor", "")
	if w.Code != http.StatusForbidden || errorCode(t, w) != "directory_access_required" {
		t.Fatalf("former member viewed another history: %d %s", w.Code, w.Body.String())
	}
}

func TestBrowserAttemptCannotUseMigratedUnknownOutcome(t *testing.T) {
	f := newFixture()
	body := `{"problemId":"problem","outcome":"unknown","minutes":12,"attemptedAt":"2026-08-13T00:00:00Z"}`
	w := request(t, f, "POST", "/api/v2/problem-attempts", "student", body)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("unknown browser attempt accepted: %d %s", w.Code, w.Body.String())
	}
}

func TestAdminUserListIncludesNonmembersAndSuspendedWithoutWideningDirectory(t *testing.T) {
	f := newFixture()
	f.repository.Users["new-account"] = accounts.User{ID: "new-account", Slug: "new-account", Name: "New Account", Email: "new@example.com", AccountState: "active", Timezone: "Australia/Adelaide", Revision: 1}
	f.repository.Users["suspended-account"] = accounts.User{ID: "suspended-account", Slug: "suspended-account", Name: "Suspended Account", Email: "suspended@example.com", AccountState: "suspended", Timezone: "Australia/Adelaide", Revision: 2}

	w := request(t, f, "GET", "/api/v2/admin/users?accountState=suspended&sort=id:asc", "admin", "")
	if w.Code != http.StatusOK || !bytes.Contains(w.Body.Bytes(), []byte(`"id":"suspended-account"`)) || !bytes.Contains(w.Body.Bytes(), []byte(`"email":"suspended@example.com"`)) || bytes.Contains(w.Body.Bytes(), []byte(`"id":"new-account"`)) {
		t.Fatalf("admin account list: %d %s", w.Code, w.Body.String())
	}
	w = request(t, f, "GET", "/api/v2/users?query=New%20Account", "student", "")
	if w.Code != http.StatusOK || bytes.Contains(w.Body.Bytes(), []byte("new-account")) || bytes.Contains(w.Body.Bytes(), []byte("new@example.com")) {
		t.Fatalf("public directory widened to nonmember: %d %s", w.Code, w.Body.String())
	}
}

func TestPracticeSettingsRequireRelationshipEnablement(t *testing.T) {
	f := newFixture()
	f.repository.Users["student"] = accounts.User{ID: "student", Slug: "student", Name: "Student", Email: "private@example.com", AccountState: "active", Timezone: "Australia/Adelaide", Revision: 1}
	f.repository.Enrollments["student-enrollment"] = programme.EnrollmentRecord{ID: "student-enrollment", SeasonID: "season", UserID: "student", Role: "student", State: "active", Revision: 1}
	f.repository.Users["mentor"] = accounts.User{ID: "mentor", Slug: "mentor", Name: "Mentor", AccountState: "active", Timezone: "Australia/Adelaide", Revision: 1}
	f.repository.Enrollments["mentor-enrollment"] = programme.EnrollmentRecord{ID: "mentor-enrollment", SeasonID: "season", UserID: "mentor", Role: "mentor", State: "active", Revision: 1}
	f.repository.Mentorships["assignment"] = programme.MentorshipRecord{ID: "assignment", SeasonID: "season", MentorUserID: "mentor", StudentUserID: "student", Revision: 1}
	f.actors["mentor"] = authz.Actor{UserID: "mentor", EmailVerified: true, AccountState: authz.AccountActive, GlobalRoles: map[authz.GlobalRole]bool{}, Enrollments: []authz.Enrollment{{SeasonID: "season", Role: authz.Mentor, State: authz.Active}}}

	w := request(t, f, "GET", "/api/v2/me/practice-settings", "student", "")
	if w.Code != http.StatusOK || !bytes.Contains(w.Body.Bytes(), []byte(`"easyMinutes":20`)) {
		t.Fatalf("defaults: %d %s", w.Code, w.Body.String())
	}
	w = request(t, f, "GET", "/api/v2/users/student/practice-settings", "mentor", "")
	if w.Code != http.StatusOK || !bytes.Contains(w.Body.Bytes(), []byte(`"revision":1`)) {
		t.Fatalf("mentor target settings: %d %s", w.Code, w.Body.String())
	}
	w = request(t, f, "PATCH", "/api/v2/me/practice-settings", "student", `{"premiumOptIn":true,"easyMinutes":25,"mediumMinutes":35,"hardMinutes":50,"revision":1}`)
	if w.Code != http.StatusForbidden || errorCode(t, w) != "practice_goals_not_enabled" {
		t.Fatalf("unapproved goals changed: %d %s", w.Code, w.Body.String())
	}
	w = request(t, f, "POST", "/api/v2/users/student/practice-goals/enable", "mentor", `{"seasonId":"season","revision":1}`)
	if w.Code != http.StatusOK || !bytes.Contains(w.Body.Bytes(), []byte(`"goalsEnabled":true`)) {
		t.Fatalf("mentor enablement: %d %s", w.Code, w.Body.String())
	}
	w = request(t, f, "PATCH", "/api/v2/me/practice-settings", "student", `{"premiumOptIn":true,"easyMinutes":25,"mediumMinutes":40,"hardMinutes":60,"revision":2}`)
	if w.Code != http.StatusOK || !bytes.Contains(w.Body.Bytes(), []byte(`"premiumOptIn":true`)) || !bytes.Contains(w.Body.Bytes(), []byte(`"hardMinutes":60`)) {
		t.Fatalf("approved goals update: %d %s", w.Code, w.Body.String())
	}
}

func TestIDORRevisionConflictAndOrigin(t *testing.T) {
	f := newFixture()
	w := request(t, f, "DELETE", "/api/v2/problem-attempts/foreign?revision=1", "student", "")
	if w.Code != 404 {
		t.Fatalf("foreign delete leaked existence: %d", w.Code)
	}
	w = request(t, f, "DELETE", "/api/v2/problem-attempts/owned?revision=2", "student", "")
	if w.Code != 409 || errorCode(t, w) != "stale_revision" {
		t.Fatalf("stale delete: %d", w.Code)
	}
	r := httptest.NewRequest("DELETE", "/api/v2/problem-attempts/owned?revision=1", nil)
	r.Header.Set("X-Test-Actor", "student")
	r.Header.Set("Origin", "https://evil.test")
	w = httptest.NewRecorder()
	f.handler.ServeHTTP(w, r)
	if w.Code != 403 || errorCode(t, w) != "origin_rejected" {
		t.Fatalf("origin: %d", w.Code)
	}
}

func TestPaginationCursorIsFilterBound(t *testing.T) {
	f := newFixture()
	for i := 0; i < 30; i++ {
		id := string(rune('a'+i/26)) + string(rune('a'+i%26))
		f.repository.Problems[id] = practice.ProblemRecord{ID: id, Difficulty: "easy", Categories: []string{"arrays"}}
	}
	w := request(t, f, "GET", "/api/v2/leetcode-problems?limit=2&difficulty=easy", "student", "")
	if w.Code != 200 {
		t.Fatalf("list: %d %s", w.Code, w.Body.String())
	}
	var page struct {
		PageInfo PageInfo `json:"pageInfo"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &page)
	if page.PageInfo.NextCursor == nil {
		t.Fatal("missing next cursor")
	}
	w = request(t, f, "GET", "/api/v2/leetcode-problems?limit=2&difficulty=hard&cursor="+*page.PageInfo.NextCursor, "student", "")
	if w.Code != 400 || errorCode(t, w) != "invalid_cursor" {
		t.Fatalf("cursor rebound: %d", w.Code)
	}
}

func TestCreateStatusRateLimitAndRetryAfter(t *testing.T) {
	f := newFixture()
	body := `{"problemId":"problem","outcome":"independently_solved","confidence":5,"minutes":12,"attemptedAt":"2026-08-13T00:00:00Z"}`
	w := request(t, f, "POST", "/api/v2/problem-attempts", "student", body)
	if w.Code != 201 {
		t.Fatalf("create status %d %s", w.Code, w.Body.String())
	}
	for i := 0; i < 5; i++ {
		_ = request(t, f, "GET", "/api/v2/recommendations/current", "student", "")
	}
	w = request(t, f, "GET", "/api/v2/recommendations/current", "student", "")
	if w.Code != 429 || w.Header().Get("Retry-After") == "" || errorCode(t, w) != "rate_limited" {
		t.Fatalf("rate limit: %d %#v", w.Code, w.Header())
	}
}

func TestRecommendationProblemSatisfiesLeetcodeProblemResponseContract(t *testing.T) {
	f := newFixture()
	f.repository.Enrollments["student-enrollment"] = programme.EnrollmentRecord{ID: "student-enrollment", SeasonID: "season", UserID: "student", Role: "student", StudentLevel: "beginner", State: "active", AssignmentState: "active", Revision: 1}
	f.repository.Problems["contract-candidate"] = practice.ProblemRecord{ID: "contract-candidate", Number: 42, Title: "Contract Candidate", Link: "https://rsp.test/problems/contract-candidate", Difficulty: "easy", Categories: []string{"arrays"}, Revision: 7}

	for _, phase := range []string{"generated", "active"} {
		w := request(t, f, http.MethodGet, "/api/v2/recommendations/current", "student", "")
		if w.Code != http.StatusOK {
			t.Fatalf("%s recommendation: %d %s", phase, w.Code, w.Body.String())
		}
		var response struct {
			Problem map[string]any `json:"problem"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Fatal(err)
		}

		for _, field := range []string{"id", "number", "title", "link", "difficulty", "categories", "premium", "revision"} {
			if _, ok := response.Problem[field]; !ok {
				t.Fatalf("%s recommendation problem missing %s: %s", phase, field, w.Body.String())
			}
		}
		if response.Problem["number"] != float64(42) || response.Problem["revision"] != float64(7) {
			t.Fatalf("%s recommendation problem has zero/wrong required values: %s", phase, w.Body.String())
		}
	}
}

func TestPrivilegedSeasonCreationRequiresMFACsrfAndUses201(t *testing.T) {
	f := newFixture()
	body := `{"name":"New","slug":"new","location":"Adelaide","imageUrl":"https://example.com/image","resourcesUrl":"https://example.com/resources","startAt":"2026-08-13T00:00:00Z","endAt":"2026-08-14T00:00:00Z"}`
	w := request(t, f, "POST", "/api/v2/seasons", "student", body)
	if w.Code != 403 {
		t.Fatalf("student created season: %d", w.Code)
	}
	r := httptest.NewRequest("POST", "/api/v2/seasons", strings.NewReader(body))
	r.Header.Set("X-Test-Actor", "director")
	r.Header.Set("Origin", "https://rsp.test")
	r.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	f.handler.ServeHTTP(w, r)
	if w.Code != 201 {
		t.Fatalf("director create: %d %s", w.Code, w.Body.String())
	}
}

func TestSeasonSlugWeekBoundsAndClosedWriteLocks(t *testing.T) {
	f := newFixture()
	invalidSlug := `{"name":"Season","slug":"not / safe","startAt":"2026-08-01T00:00:00Z","endAt":"2026-08-31T00:00:00Z","location":"Adelaide","imageUrl":"https://rsp.test/image","resourcesUrl":"https://rsp.test/resources"}`
	w := request(t, f, "POST", "/api/v2/seasons", "director", invalidSlug)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("invalid season slug accepted: %d %s", w.Code, w.Body.String())
	}
	season := f.repository.Seasons["season"]
	outOfBounds := `{"number":1,"startAt":"` + season.StartAt.Add(-time.Hour).UTC().Format(time.RFC3339) + `","endAt":"` + season.StartAt.Add(time.Hour).UTC().Format(time.RFC3339) + `","resourceUrl":"https://rsp.test/week"}`
	w = request(t, f, "POST", "/api/v2/seasons/season/weeks", "coordinator", outOfBounds)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("out-of-bounds week accepted: %d %s", w.Code, w.Body.String())
	}
	season.Status = "closed"
	f.repository.Seasons["season"] = season
	w = request(t, f, "POST", "/api/v2/seasons/season/members/other-enrollment/promote", "director", `{"role":"mentor","revision":1}`)
	if w.Code != http.StatusConflict || errorCode(t, w) != "season_closed" {
		t.Fatalf("closed enrollment promoted: %d %s", w.Code, w.Body.String())
	}
	update := `{"name":"Season","slug":"season","startAt":"` + season.StartAt.UTC().Format(time.RFC3339) + `","endAt":"` + season.EndAt.UTC().Format(time.RFC3339) + `","location":"Adelaide","imageUrl":"https://rsp.test/image","resourcesUrl":"https://rsp.test/resources","revision":1}`
	w = request(t, f, "PATCH", "/api/v2/seasons/season", "director", update)
	if w.Code != http.StatusConflict || errorCode(t, w) != "season_closed" {
		t.Fatalf("closed season definition updated: %d %s", w.Code, w.Body.String())
	}
}

func TestFormerNonStudentCannotReadSeasonDirectoryOrResources(t *testing.T) {
	f := newFixture()
	for _, path := range []string{"/api/v2/seasons/season", "/api/v2/seasons/season/weeks", "/api/v2/seasons/season/members", "/api/v2/seasons/season/mentorships"} {
		w := request(t, f, "GET", path, "former-mentor", "")
		if w.Code != http.StatusForbidden && w.Code != http.StatusNotFound {
			t.Errorf("former mentor path %s status=%d body=%s", path, w.Code, w.Body.String())
		}
	}
}

func TestIdentityLifecycleRequiresServiceTokenAndCreatesLink(t *testing.T) {
	t.Setenv("IDENTITY_SERVICE_TOKEN", "internal-secret")
	f := newFixture()
	body := `{"eventId":"event-create-auth-new","type":"auth_user_created","authUserId":"auth-new","email":"new@example.com","emailVerified":false,"securityVersion":1,"occurredAt":"2026-08-13T00:00:00Z"}`
	w := request(t, f, "POST", "/internal/auth/lifecycle-events", "", body)
	if w.Code != 401 {
		t.Fatalf("unauthorized lifecycle=%d", w.Code)
	}
	raw := httptest.NewRequest("POST", "/internal/auth/lifecycle-events", strings.NewReader(body))
	raw.Header.Set("Authorization", "internal-secret")
	raw.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	f.handler.ServeHTTP(w, raw)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("raw service token accepted: %d", w.Code)
	}
	trailing := httptest.NewRequest("POST", "/internal/auth/lifecycle-events", strings.NewReader(body+` trailing`))
	trailing.Header.Set("Authorization", "Bearer internal-secret")
	trailing.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	f.handler.ServeHTTP(w, trailing)
	if w.Code != http.StatusBadRequest || errorCode(t, w) != "validation_failed" {
		t.Fatalf("malformed trailing JSON accepted: %d %s", w.Code, w.Body.String())
	}
	r := httptest.NewRequest("POST", "/internal/auth/lifecycle-events", strings.NewReader(body))
	r.Header.Set("Authorization", "Bearer internal-secret")
	r.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	f.handler.ServeHTTP(w, r)
	if w.Code != 204 {
		t.Fatalf("lifecycle=%d %s", w.Code, w.Body.String())
	}
	if _, ok := f.repository.AuthSubjects["auth-new"]; !ok {
		t.Fatal("auth subject was not linked")
	}
}

func TestMockEligibilityComesFromRepositoryAndInterviewsSurviveAPIRecreation(t *testing.T) {
	f := newFixture()
	f.repository.Enrollments["student-member"] = programme.EnrollmentRecord{ID: "student-member", SeasonID: "season", UserID: "student", Role: "student", State: "active", Revision: 1}
	body := `{"interviewee":{"userId":"other","activeMember":true},"occurredAt":"2026-08-13T00:00:00Z","durationMinutes":60,"rounds":[{"id":"round","type":"behavioural","scores":{"behavioural":7},"reviewed":false,"intervieweeComment":""}]}`
	w := request(t, f, "POST", "/api/v2/mock-interviews", "student", body)
	if w.Code != http.StatusBadRequest || errorCode(t, w) != "openapi_validation_failed" {
		t.Fatalf("caller-supplied eligibility accepted: %d %s", w.Code, w.Body.String())
	}

	f.repository.Enrollments["other-member"] = programme.EnrollmentRecord{ID: "other-member", SeasonID: "season", UserID: "other", Role: "mentor", State: "active", Revision: 1}
	body = `{"interviewee":{"userId":"other"},"occurredAt":"2026-08-13T00:00:00Z","durationMinutes":60,"rounds":[{"id":"round","type":"behavioural","scores":{"behavioural":7}}]}`
	w = request(t, f, "POST", "/api/v2/mock-interviews", "student", body)
	if w.Code != http.StatusCreated {
		t.Fatalf("stored eligibility was not authoritative: %d %s", w.Code, w.Body.String())
	}

	f.handler = New(Config{Store: f.repository, Authenticator: f.api.auth, PublicOrigin: "https://rsp.test", CursorSecret: []byte("0123456789abcdef")}).Handler()
	w = request(t, f, "GET", "/api/v2/mock-interviews?mode=given", "student", "")
	if w.Code != http.StatusOK || !bytes.Contains(w.Body.Bytes(), []byte(`"totalCount":1`)) {
		t.Fatalf("mock interview was process-local: %d %s", w.Code, w.Body.String())
	}
}

func TestClosedSeasonLocksLinkedMockMutations(t *testing.T) {
	f := newFixture()
	season := f.repository.Seasons["season"]
	season.Status = "closed"
	f.repository.Seasons["season"] = season
	f.repository.Enrollments["student-member"] = programme.EnrollmentRecord{ID: "student-member", SeasonID: "season", UserID: "student", Role: "student", State: "completed", Revision: 2}
	score := 7
	seasonID := "season"
	f.repository.Mocks["closed-mock"] = mockinterviews.Interview{ID: "closed-mock", InterviewerID: "student", IntervieweeID: "other", SeasonID: &seasonID, OccurredAt: season.StartAt, DurationMinutes: 60, Rounds: []mockinterviews.Round{{ID: "round", Type: mockinterviews.Behavioural, Scores: mockinterviews.Scores{Behavioural: &score}}}, Revision: 1}

	w := request(t, f, "DELETE", "/api/v2/mock-interviews/closed-mock?revision=1", "student", "")
	if w.Code != http.StatusConflict || errorCode(t, w) != "season_closed" {
		t.Fatalf("closed mock mutation: %d %s", w.Code, w.Body.String())
	}
}
