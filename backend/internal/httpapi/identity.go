package httpapi

import (
	"crypto/subtle"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/magedmg/RSP-website/backend/internal/store"
)

func (a *API) identityLifecycle(w http.ResponseWriter, r *http.Request) {
	expected := os.Getenv("IDENTITY_SERVICE_TOKEN")
	provided := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	if expected == "" || len(provided) != len(expected) || subtle.ConstantTimeCompare([]byte(provided), []byte(expected)) != 1 {
		a.fail(w, r, 401, "invalid_service_token", "Internal authentication failed", "A valid service token is required.", nil)
		return
	}
	var in struct {
		Type             string     `json:"type"`
		AuthUserID       string     `json:"authUserId"`
		Email            string     `json:"email"`
		EmailVerified    bool       `json:"emailVerified"`
		OccurredAt       time.Time  `json:"occurredAt"`
		RecoveryDeadline *time.Time `json:"recoveryDeadline"`
		Reason           string     `json:"reason"`
	}
	if err := decodeJSON(w, r, &in); err != nil || in.AuthUserID == "" || in.OccurredAt.IsZero() {
		validation(a, w, r, "valid type, authUserId, and occurredAt are required")
		return
	}
	err := a.store.ApplyIdentityEvent(r.Context(), store.IdentityEvent{Type: in.Type, AuthUserID: in.AuthUserID, Email: in.Email, EmailVerified: in.EmailVerified, OccurredAt: in.OccurredAt.UTC(), RecoveryDeadline: in.RecoveryDeadline, Reason: in.Reason})
	if err != nil {
		storeFailure(a, w, r, err)
		return
	}
	w.WriteHeader(204)
}
