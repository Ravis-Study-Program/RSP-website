package api

import (
	"crypto/subtle"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/magedmg/RSP-website/backend/internal/accounts"
)

var discordAvatarPattern = regexp.MustCompile(`^https://cdn\.discordapp\.com/avatars/[0-9]{1,20}/(a_)?[a-f0-9]{32}\.png\?size=256$`)

type receiveIdentityLifecycleEventRequest struct {
	AvatarURL        *string    `json:"avatarUrl"`
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

func (a *API) receiveIdentityLifecycleEvent(w http.ResponseWriter, r *http.Request) {
	expected := os.Getenv("IDENTITY_SERVICE_TOKEN")
	header := r.Header.Get("Authorization")
	if !strings.HasPrefix(header, "Bearer ") {
		writeErrorResponse(w, http.StatusUnauthorized, "A valid service token is required.")
		return
	}
	provided := strings.TrimPrefix(header, "Bearer ")
	if expected == "" || len(provided) != len(expected) || subtle.ConstantTimeCompare([]byte(provided), []byte(expected)) != 1 {
		writeErrorResponse(w, http.StatusUnauthorized, "A valid service token is required.")
		return
	}
	var request receiveIdentityLifecycleEventRequest
	if err := decodeJSON(r.Body, &request); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	if request.EventID == "" || request.AuthUserID == "" || request.OccurredAt.IsZero() || request.SecurityVersion < 1 {
		writeErrorResponse(w, http.StatusBadRequest, "valid eventId, type, authUserId, securityVersion, and occurredAt are required")
		return
	}

	if request.Type == "discord_avatar_updated" && request.AvatarURL != nil && !discordAvatarPattern.MatchString(*request.AvatarURL) {
		writeErrorResponse(w, http.StatusBadRequest, "Invalid Discord avatar URL")
		return
	}

	err := a.db.ApplyIdentityEvent(r.Context(), accounts.IdentityEvent{
		EventID:          request.EventID,
		AvatarURL:        request.AvatarURL,
		Type:             request.Type,
		AuthUserID:       request.AuthUserID,
		Email:            request.Email,
		EmailVerified:    request.EmailVerified,
		SecurityVersion:  request.SecurityVersion,
		OccurredAt:       request.OccurredAt.UTC(),
		RecoveryDeadline: request.RecoveryDeadline,
		Reason:           request.Reason,
		AccountState:     request.AccountState,
		ActorUserID:      request.ActorUserID,
	})
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
