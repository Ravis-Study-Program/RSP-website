//go:build integration

package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/magedmg/RSP-website/backend/internal/authz"
	"github.com/magedmg/RSP-website/backend/internal/dal"
)

func TestDatabaseFailuresAreNotReportedAsPermissionDenials(t *testing.T) {
	newPostgresFixture(t)
	brokenDB, err := dal.Open(context.Background(), os.Getenv("TEST_DATABASE_URL"))
	if err != nil {
		t.Fatal(err)
	}
	brokenDB.Close()
	api := New(Config{
		DB: brokenDB,
		Authenticator: AuthenticatorFunc(func(*http.Request) (authz.Actor, error) {
			return authz.Actor{
				UserID: studentID, EmailVerified: true, AccountState: authz.AccountActive,
				Enrollments: []authz.Enrollment{{SeasonID: seasonID, Role: authz.Student, State: authz.Active}},
			}, nil
		}),
		CursorSecret: []byte("0123456789abcdef"),
	})
	handler := api.Handler()
	for _, tc := range []struct{ method, path string }{
		{http.MethodPost, "/api/v2/seasons/" + seasonID + "/weeks"},
		{http.MethodPatch, "/api/v2/seasons/" + seasonID + "/resources"},
		{http.MethodGet, "/api/v2/mock-interviews"},
		{http.MethodGet, "/api/v2/mock-interviews/participants"},
		{http.MethodPost, "/api/v2/mock-interviews"},
	} {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			response := testRequest(t, handler, tc.method, tc.path, "", "{}")
			assertDatabaseFailure(t, response)
		})
	}
	t.Run("selected mock season", func(t *testing.T) {
		response := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/", nil)
		selectedSeason := seasonID
		if api.validMockSeason(response, request, &selectedSeason, time.Now()) {
			t.Fatal("unavailable database accepted a season")
		}
		assertDatabaseFailure(t, response)
	})
}

func assertDatabaseFailure(t *testing.T, response *httptest.ResponseRecorder) {
	t.Helper()
	if response.Code != http.StatusInternalServerError || errorCode(t, response) != "internal_error" {
		t.Fatalf("database failure status=%d body=%s", response.Code, response.Body.String())
	}
	if strings.Contains(response.Body.String(), "database is closed") {
		t.Fatal("database implementation error leaked into response")
	}
}

func TestSeasonAdminDistinguishesMissingSeasonFromForbidden(t *testing.T) {
	fixture := newPostgresFixture(t)
	missingID := "00000000-0000-7000-8000-000000000199"
	response := testRequest(t, fixture.handler, http.MethodPost, "/api/v2/seasons/"+missingID+"/weeks", "student", "{}")
	if response.Code != http.StatusNotFound || errorCode(t, response) != "not_found" {
		t.Fatalf("missing season status=%d body=%s", response.Code, response.Body.String())
	}
	response = testRequest(t, fixture.handler, http.MethodPost, "/api/v2/seasons/"+seasonID+"/weeks", "student", "{}")
	if response.Code != http.StatusForbidden || errorCode(t, response) != "season_admin_required" {
		t.Fatalf("forbidden season status=%d body=%s", response.Code, response.Body.String())
	}
}
