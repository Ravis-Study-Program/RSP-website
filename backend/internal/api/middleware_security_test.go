package api

import (
	"bytes"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/magedmg/RSP-website/backend/internal/authn"
	"github.com/magedmg/RSP-website/backend/internal/authz"
	"github.com/magedmg/RSP-website/backend/internal/dal"
)

func TestClaimsMustMatchCurrentAccountStateAndSecurityVersion(t *testing.T) {
	actor := authz.Actor{AccountState: authz.AccountActive, SecurityVersion: 4}
	for name, claims := range map[string]authn.Claims{
		"current":       {AccountState: "active", SecurityVersion: 4},
		"stale version": {AccountState: "active", SecurityVersion: 3},
		"wrong state":   {AccountState: "suspended", SecurityVersion: 4},
		"missing":       {AccountState: "active"},
	} {
		want := name == "current"
		if got := claimsMatchActor(claims, actor); got != want {
			t.Fatalf("%s: got %v, want %v", name, got, want)
		}
	}
}

func TestWriteStoreErrorResponse(t *testing.T) {
	unexpected := errors.New("private database failure")
	tests := []struct {
		name        string
		err         error
		wantStatus  int
		wantMessage string
		wantLog     bool
	}{
		{"not found", dal.ErrNotFound, http.StatusNotFound, "The requested resource does not exist.", false},
		{"wrapped conflict", errors.Join(errors.New("update failed"), dal.ErrConflict), http.StatusConflict, "The requested operation cannot be completed.", false},
		{"duplicate", dal.ErrDuplicate, http.StatusConflict, "A resource with that unique value already exists.", false},
		{"unexpected", unexpected, http.StatusInternalServerError, "The request could not be completed.", true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var logs bytes.Buffer
			api := New(Config{Logger: slog.New(slog.NewJSONHandler(&logs, nil))})
			response := httptest.NewRecorder()
			response.Header().Set("X-Request-ID", "test-request")

			api.writeStoreErrorResponse(response, test.err)

			if response.Code != test.wantStatus || errorMessage(t, response) != test.wantMessage {
				t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
			}
			if strings.Contains(response.Body.String(), unexpected.Error()) {
				t.Fatal("response exposed the internal error")
			}
			if got := strings.Contains(logs.String(), unexpected.Error()); got != test.wantLog {
				t.Fatalf("unexpected error logged=%v, want %v: %s", got, test.wantLog, logs.String())
			}
		})
	}
}
