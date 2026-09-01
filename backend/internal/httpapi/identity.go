package httpapi

import (
	"crypto/subtle"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/magedmg/RSP-website/backend/internal/accounts"
)

func (a *API) identityLifecycle(w http.ResponseWriter, r *http.Request) {
	expected := os.Getenv("IDENTITY_SERVICE_TOKEN")
	header := r.Header.Get("Authorization")
	if !strings.HasPrefix(header, "Bearer ") {
		writeErrorResponse(w, http.StatusUnauthorized, "invalid_service_token", "A valid service token is required.")
		return
	}
	provided := strings.TrimPrefix(header, "Bearer ")
	if expected == "" || len(provided) != len(expected) || subtle.ConstantTimeCompare([]byte(provided), []byte(expected)) != 1 {
		writeErrorResponse(w, http.StatusUnauthorized, "invalid_service_token", "A valid service token is required.")
		return
	}
	var in struct {
		EventID          string     `json:"eventId"`
		Type             string     `json:"type"`
		AuthUserID       string     `json:"authUserId"`
		Email            string     `json:"email"`
		EmailVerified    bool       `json:"emailVerified"`
		OccurredAt       time.Time  `json:"occurredAt"`
		RecoveryDeadline *time.Time `json:"recoveryDeadline"`
		Reason           string     `json:"reason"`
		AccountState     string     `json:"accountState"`
		ActorUserID      string     `json:"actorUserId"`
		SecurityVersion  int64      `json:"securityVersion"`
	}
	if err := decodeJSON(w, r, &in); err != nil || in.EventID == "" || in.AuthUserID == "" || in.OccurredAt.IsZero() || in.SecurityVersion < 1 {
		writeErrorResponse(w, http.StatusBadRequest, "validation_failed", "valid eventId, type, authUserId, securityVersion, and occurredAt are required")
		return
	}

	err := a.store.ApplyIdentityEvent(r.Context(), accounts.IdentityEvent{EventID: in.EventID, Type: in.Type, AuthUserID: in.AuthUserID, Email: in.Email, EmailVerified: in.EmailVerified, SecurityVersion: in.SecurityVersion, OccurredAt: in.OccurredAt.UTC(), RecoveryDeadline: in.RecoveryDeadline, Reason: in.Reason, AccountState: in.AccountState, ActorUserID: in.ActorUserID})
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
