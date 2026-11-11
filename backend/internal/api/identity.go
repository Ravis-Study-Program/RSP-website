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
	Name             string     `json:"name"`
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

// authorizeIdentityService admits only the auth service, by its shared token.
func authorizeIdentityService(w http.ResponseWriter, r *http.Request) bool {
	expected := os.Getenv("IDENTITY_SERVICE_TOKEN")
	header := r.Header.Get("Authorization")
	provided := strings.TrimPrefix(header, "Bearer ")
	if !strings.HasPrefix(header, "Bearer ") || expected == "" || len(provided) != len(expected) || subtle.ConstantTimeCompare([]byte(provided), []byte(expected)) != 1 {
		writeErrorResponse(w, http.StatusUnauthorized, "A valid service token is required.")
		return false
	}
	return true
}

// checkInvitationEmail tells the auth service whether an invitation token was
// sent to an email address. Opening the invitation link proves the person
// reads that inbox, so an account signed up with it needs no verification email.
func (a *API) checkInvitationEmail(w http.ResponseWriter, r *http.Request) {
	if !authorizeIdentityService(w, r) {
		return
	}
	var request struct {
		Token string `json:"token"`
		Email string `json:"email"`
	}
	if err := decodeJSON(r.Body, &request); err != nil || len(request.Token) != 43 || request.Email == "" {
		writeErrorResponse(w, http.StatusBadRequest, "A token and email are required.")
		return
	}
	matches, err := a.db.InvitationSentTo(r.Context(), hashInvitationToken(request.Token), request.Email, time.Now().UTC())
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}
	writeJSONResponse(w, http.StatusOK, map[string]bool{"matches": matches})
}

func (a *API) receiveIdentityLifecycleEvent(w http.ResponseWriter, r *http.Request) {
	if !authorizeIdentityService(w, r) {
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
		Name:             request.Name,
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
