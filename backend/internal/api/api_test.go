package api

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

	"github.com/magedmg/RSP-website/backend/internal/authz"
)

func testHandler(authenticator Authenticator) http.Handler {
	return New(Config{
		Authenticator: authenticator,
		PublicOrigin:  "https://rsp.test",
		CursorSecret:  []byte("0123456789abcdef"),
		Logger:        slog.New(slog.NewTextHandler(io.Discard, nil)),
	}).Handler()
}

func testRequest(t *testing.T, handler http.Handler, method, path, actor, body string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	if actor != "" {
		request.Header.Set("Authorization", "Bearer "+actor)
	}
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func errorMessage(t *testing.T, response *httptest.ResponseRecorder) string {
	t.Helper()
	if got := response.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("content type %q body %s", got, response.Body.String())
	}
	var body map[string]string
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"message", "requestId"} {
		if _, ok := body[field]; !ok {
			t.Fatalf("missing %s in %#v", field, body)
		}
	}
	if len(body) != 2 {
		t.Fatalf("unexpected error response fields: %#v", body)
	}
	if body["requestId"] != response.Header().Get("X-Request-ID") {
		t.Fatalf("request ID does not match response header: %#v", body)
	}
	return body["message"]
}

func TestRequestContextNormalizesRequestIDAndRecovers(t *testing.T) {
	handler := requestContext(slog.New(slog.NewTextHandler(io.Discard, nil)), func(int, time.Duration) {}, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("boom")
	}))
	request := httptest.NewRequest(http.MethodGet, "/panic", nil)
	request.Header.Set("X-Request-ID", strings.Repeat("x", 200)+"\nunsafe")
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusInternalServerError || response.Header().Get("X-Request-ID") == request.Header.Get("X-Request-ID") {
		t.Fatalf("status=%d requestID=%q", response.Code, response.Header().Get("X-Request-ID"))
	}
	if got := errorMessage(t, response); got != "The request could not be completed." {
		t.Fatalf("message=%s body=%s", got, response.Body.String())
	}
}

func TestRequestBodyLimitStopsBeforeTheHandler(t *testing.T) {
	authenticator := AuthenticatorFunc(func(context.Context, string) (authz.Actor, error) {
		return authz.Actor{
			UserID:        "test",
			EmailVerified: true,
			AccountState:  authz.AccountActive,
		}, nil
	})
	body := strings.Repeat("x", int(maxRequestBodyBytes)+1)
	response := testRequest(t, testHandler(authenticator), http.MethodPatch, "/api/v2/me", "test", body)

	if response.Code != http.StatusBadRequest || errorMessage(t, response) != "The request body is too large or unreadable." {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestHealthAuthenticationAndOriginChecksDoNotExposeInternalErrors(t *testing.T) {
	authenticator := AuthenticatorFunc(func(_ context.Context, token string) (authz.Actor, error) {
		if token == "student" {
			return authz.Actor{
				UserID:        "student",
				EmailVerified: true,
				AccountState:  authz.AccountActive,
			}, nil
		}
		return authz.Actor{}, errors.New("private authentication failure")
	})
	handler := testHandler(authenticator)

	if response := testRequest(t, handler, http.MethodGet, "/api/v2/health/live", "", ""); response.Code != http.StatusOK {
		t.Fatalf("health status=%d", response.Code)
	}
	response := testRequest(t, handler, http.MethodGet, "/api/v2/me", "", "")
	if response.Code != http.StatusUnauthorized || errorMessage(t, response) != "A valid access token is required." || bytes.Contains(response.Body.Bytes(), []byte("private authentication failure")) {
		t.Fatalf("authentication status=%d body=%s", response.Code, response.Body.String())
	}

	request := httptest.NewRequest(http.MethodDelete, "/api/v2/problem-attempts/anything", nil)
	request.Header.Set("Authorization", "Bearer student")
	request.Header.Set("Origin", "https://evil.test")
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden || errorMessage(t, response) != "State-changing requests must come from the configured application origin." {
		t.Fatalf("origin status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestRequestCannotSupplyItsOwnActorID(t *testing.T) {
	authenticator := AuthenticatorFunc(func(context.Context, string) (authz.Actor, error) {
		return authz.Actor{
			UserID:        "student",
			EmailVerified: true,
			AccountState:  authz.AccountActive,
			Enrollments:   []authz.Enrollment{{SeasonID: "season", Role: authz.Student, State: authz.Active}},
		}, nil
	})
	body := `{"problemId":"problem","outcome":"independently_solved","confidence":5,"minutes":12,"attemptedAt":"2026-09-01T00:00:00Z","userId":"someone-else"}`
	response := testRequest(t, testHandler(authenticator), http.MethodPost, "/api/v2/problem-attempts", "student", body)

	if response.Code != http.StatusBadRequest || errorMessage(t, response) != `json: unknown field "userId"` {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}
