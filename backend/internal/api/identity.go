package api

import (
	"crypto/subtle"
	"errors"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/Ravis-Study-Program/RSP-website/backend/internal/accounts"
	"github.com/Ravis-Study-Program/RSP-website/backend/internal/dal"
)

var discordAvatarPattern = regexp.MustCompile(`^https://cdn\.discordapp\.com/avatars/[0-9]{1,20}/(a_)?[a-f0-9]{32}\.png\?size=256$`)

type receiveIdentityLifecycleEventRequest struct {
	AvatarURL        *string    `json:"avatarUrl"`
	Name             string     `json:"name"`
	Username         string     `json:"username"`
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

// checkInvitationEmail tells the auth service whether someone signing up has
// a live invitation, since accounts are by invitation only, and the name it
// was sent to, which their account takes. With a token, the invitation must
// be that one and sent to this email: opening its link proves the person
// reads that inbox, so the account needs no verification email. Without one
// (Google sign-up, which proves the address itself), any live invitation to
// the email counts. A username chosen at sign-up is checked and saved to the
// invitation, so accepting it keeps that username.
func (a *API) checkInvitationEmail(w http.ResponseWriter, r *http.Request) {
	if !authorizeIdentityService(w, r) {
		return
	}
	var request struct {
		Token    string `json:"token"`
		Email    string `json:"email"`
		Username string `json:"username"`
	}
	if err := decodeJSON(r.Body, &request); err != nil || (request.Token != "" && len(request.Token) != 43) || strings.TrimSpace(request.Email) == "" || (request.Username != "" && request.Token == "") {
		writeErrorResponse(w, http.StatusBadRequest, "An email (and a token with any username) is required.")
		return
	}
	var invitee dal.Invitee
	var err error
	if request.Token != "" {
		invitee, err = a.db.InvitationInvitee(r.Context(), hashInvitationToken(request.Token), time.Now().UTC())
	} else {
		invitee, err = a.db.InvitationForEmail(r.Context(), request.Email, time.Now().UTC())
	}
	var response struct {
		Matches bool   `json:"matches"`
		Name    string `json:"name,omitempty"`
		Problem string `json:"problem,omitempty"`
	}
	switch {
	case errors.Is(err, dal.ErrNotFound):
		writeJSONResponse(w, http.StatusOK, response)
		return
	case err != nil:
		a.writeStoreErrorResponse(w, err)
		return
	}
	if !strings.EqualFold(invitee.Email, strings.TrimSpace(request.Email)) {
		writeJSONResponse(w, http.StatusOK, response)
		return
	}
	response.Matches, response.Name = true, invitee.Name
	if slug := strings.ToLower(strings.TrimPrefix(strings.TrimSpace(request.Username), "@")); slug != "" {
		if response.Problem = a.slugProblem(r, slug, ""); response.Problem == "" && slug != invitee.Slug {
			if err := a.db.SetInvitationSlug(r.Context(), invitee.InvitationID, slug); err != nil {
				a.writeStoreErrorResponse(w, err)
				return
			}
		}
	}
	writeJSONResponse(w, http.StatusOK, response)
}

// findUnclaimedMember tells the auth service whether an email belongs to a
// member who has never had a sign-in, so a password reset requested for it
// can make one: with the member's name, and the security version their
// access tokens must carry. The reset link still has to be opened from that
// inbox before anyone can sign in.
func (a *API) findUnclaimedMember(w http.ResponseWriter, r *http.Request) {
	if !authorizeIdentityService(w, r) {
		return
	}
	var request struct {
		Email string `json:"email"`
	}
	if err := decodeJSON(r.Body, &request); err != nil || strings.TrimSpace(request.Email) == "" {
		writeErrorResponse(w, http.StatusBadRequest, "An email is required.")
		return
	}
	var response struct {
		Found           bool   `json:"found"`
		Name            string `json:"name,omitempty"`
		SecurityVersion int64  `json:"securityVersion,omitempty"`
	}
	member, err := a.db.UnclaimedMemberByEmail(r.Context(), request.Email)
	switch {
	case errors.Is(err, dal.ErrNotFound):
	case err != nil:
		a.writeStoreErrorResponse(w, err)
		return
	default:
		response.Found, response.Name, response.SecurityVersion = true, member.Name, member.SecurityVersion
	}
	writeJSONResponse(w, http.StatusOK, response)
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
		Username:         request.Username,
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
