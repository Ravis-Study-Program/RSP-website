package httpapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/magedmg/RSP-website/backend/internal/authz"
	"github.com/magedmg/RSP-website/backend/internal/model"
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
	repo.Users["student"] = model.User{ID: "student", Slug: "student", Name: "Student", Email: "private@example.com", Timezone: "Australia/Adelaide", Revision: 1}
	repo.Users["other"] = model.User{ID: "other", Slug: "other", Name: "Other", Email: "other@example.com", Timezone: "Australia/Adelaide", Revision: 1}
	repo.Seasons["season"] = model.Season{ID: "season", Slug: "s26", Name: "Season", Status: "open", StartAt: time.Now(), EndAt: time.Now().Add(24 * time.Hour), Revision: 1}
	repo.Problems["problem"] = model.Problem{ID: "problem", Number: 1, Title: "Two Sum", Difficulty: "easy", Categories: []string{"arrays"}, Revision: 1}
	repo.Attempts["owned"] = model.Attempt{ID: "owned", UserID: "student", ProblemID: "problem", Outcome: "unknown", Minutes: 10, AttemptedAt: time.Now(), Revision: 1}
	repo.Attempts["foreign"] = model.Attempt{ID: "foreign", UserID: "other", ProblemID: "problem", Outcome: "unknown", Minutes: 10, AttemptedAt: time.Now(), Revision: 1}
	recent := time.Now().Add(-time.Minute)
	actors := map[string]authz.Actor{"student": {UserID: "student", EmailVerified: true, AccountState: authz.AccountActive, GlobalRoles: map[authz.GlobalRole]bool{}, Enrollments: []authz.Enrollment{{SeasonID: "season", Role: authz.Student, State: authz.Active}}}, "director": {UserID: "student", EmailVerified: true, AccountState: authz.AccountActive, GlobalRoles: map[authz.GlobalRole]bool{authz.Director: true}, MFAAt: &recent}, "suspended": {UserID: "student", EmailVerified: true, AccountState: authz.Suspended}}
	auth := AuthenticatorFunc(func(r *http.Request) (authz.Actor, error) {
		a, ok := actors[r.Header.Get("X-Test-Actor")]
		if !ok {
			return authz.Actor{}, errors.New("missing token")
		}
		return a, nil
	})
	api := New(Config{Store: repo, Authenticator: auth, PublicOrigin: "https://rsp.test", CursorSecret: []byte("0123456789abcdef")})
	return fixture{api, repo, api.Handler(), actors}
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
func problemCode(t *testing.T, w *httptest.ResponseRecorder) string {
	t.Helper()
	if got := w.Header().Get("Content-Type"); got != "application/problem+json" {
		t.Fatalf("content type %q body %s", got, w.Body.String())
	}
	var p map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &p); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"type", "title", "status", "code", "requestId"} {
		if _, ok := p[key]; !ok {
			t.Fatalf("missing %s in %#v", key, p)
		}
	}
	return p["code"].(string)
}
func TestHealthAndAuthenticationProblemContract(t *testing.T) {
	f := newFixture()
	if w := request(t, f, "GET", "/api/v2/health/live", "", ""); w.Code != 200 {
		t.Fatal(w.Code)
	}
	w := request(t, f, "GET", "/api/v2/me", "", "")
	if w.Code != 401 || problemCode(t, w) != "authentication_required" {
		t.Fatalf("got %d %s", w.Code, w.Body.String())
	}
	w = request(t, f, "GET", "/api/v2/me", "suspended", "")
	if w.Code != 403 || problemCode(t, w) != "account_unavailable" {
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
	if w.Code != 400 || problemCode(t, w) != "openapi_validation_failed" {
		t.Fatalf("actor id accepted: %d %s", w.Code, w.Body.String())
	}
}
func TestIDORRevisionConflictAndOrigin(t *testing.T) {
	f := newFixture()
	w := request(t, f, "DELETE", "/api/v2/problem-attempts/foreign?revision=1", "student", "")
	if w.Code != 404 {
		t.Fatalf("foreign delete leaked existence: %d", w.Code)
	}
	w = request(t, f, "DELETE", "/api/v2/problem-attempts/owned?revision=2", "student", "")
	if w.Code != 409 || problemCode(t, w) != "stale_revision" {
		t.Fatalf("stale delete: %d", w.Code)
	}
	r := httptest.NewRequest("DELETE", "/api/v2/problem-attempts/owned?revision=1", nil)
	r.Header.Set("X-Test-Actor", "student")
	r.Header.Set("Origin", "https://evil.test")
	w = httptest.NewRecorder()
	f.handler.ServeHTTP(w, r)
	if w.Code != 403 || problemCode(t, w) != "origin_rejected" {
		t.Fatalf("origin: %d", w.Code)
	}
}
func TestPaginationCursorIsFilterBound(t *testing.T) {
	f := newFixture()
	for i := 0; i < 30; i++ {
		id := string(rune('a'+i/26)) + string(rune('a'+i%26))
		f.repository.Problems[id] = model.Problem{ID: id, Difficulty: "easy", Categories: []string{"arrays"}}
	}
	w := request(t, f, "GET", "/api/v2/leetcode-problems?limit=2&difficulty=easy", "student", "")
	if w.Code != 200 {
		t.Fatalf("list: %d %s", w.Code, w.Body.String())
	}
	var page struct {
		PageInfo model.PageInfo `json:"pageInfo"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &page)
	if page.PageInfo.NextCursor == nil {
		t.Fatal("missing next cursor")
	}
	w = request(t, f, "GET", "/api/v2/leetcode-problems?limit=2&difficulty=hard&cursor="+*page.PageInfo.NextCursor, "student", "")
	if w.Code != 400 || problemCode(t, w) != "invalid_cursor" {
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
	if w.Code != 429 || w.Header().Get("Retry-After") == "" || problemCode(t, w) != "rate_limited" {
		t.Fatalf("rate limit: %d %#v", w.Code, w.Header())
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

func TestIdentityLifecycleRequiresServiceTokenAndCreatesLink(t *testing.T) {
	t.Setenv("IDENTITY_SERVICE_TOKEN", "internal-secret")
	f := newFixture()
	body := `{"type":"auth_user_created","authUserId":"auth-new","email":"new@example.com","emailVerified":false,"occurredAt":"2026-08-13T00:00:00Z"}`
	w := request(t, f, "POST", "/internal/auth/lifecycle-events", "", body)
	if w.Code != 401 {
		t.Fatalf("unauthorized lifecycle=%d", w.Code)
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
