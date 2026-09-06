package api

import (
	"github.com/magedmg/RSP-website/backend/internal/accounts"
	"github.com/magedmg/RSP-website/backend/internal/authadmin"
	"github.com/magedmg/RSP-website/backend/internal/authz"
	"github.com/magedmg/RSP-website/backend/internal/platform/audit"
	"github.com/magedmg/RSP-website/backend/internal/platform/id"
	"net/http"
	"net/mail"
	"strings"
	"time"
)

func (a *API) authorizeAdminProfile(w http.ResponseWriter, r *http.Request) bool {
	actor := actorFrom(r.Context())
	if !actor.HasGlobalRole(authz.SystemAdmin) || !actor.HasRecentMFA(time.Now()) {
		writeErrorResponse(w, 403, "System Admin access with recent MFA is required.")
		return false
	}
	return true
}
func (a *API) getAdminProfile(w http.ResponseWriter, r *http.Request) {
	if !a.authorizeAdminProfile(w, r) {
		return
	}
	v, err := a.db.GetAdminProfile(r.Context(), r.PathValue("id"))
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}
	if err = a.auditPrivateDataRead(r.Context(), actorFrom(r.Context()), "user", v.ID); err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}
	writeJSONResponse(w, 200, v)
}
func (a *API) updateAdminProfile(w http.ResponseWriter, r *http.Request) {
	if !a.authorizeAdminProfile(w, r) {
		return
	}
	var request struct {
		Name      string  `json:"name"`
		Slug      string  `json:"slug"`
		AvatarURL *string `json:"avatarUrl"`
		DiscordID *string `json:"discordId"`
	}
	if err := decodeJSON(r.Body, &request); err != nil {
		writeErrorResponse(w, 400, err.Error())
		return
	}
	request.Name = strings.TrimSpace(request.Name)
	request.Slug = strings.ToLower(strings.TrimSpace(request.Slug))
	if len(request.Name) < 1 || len(request.Name) > 100 || len(request.Slug) < 3 || len(request.Slug) > 50 || !validSlug(request.Slug) {
		writeErrorResponse(w, 400, "Enter a name and a 3–50 character profile slug using letters, numbers and hyphens.")
		return
	}
	if request.AvatarURL != nil && *request.AvatarURL != "" && !isValidHTTPSURL(*request.AvatarURL) {
		writeErrorResponse(w, 400, "Avatar URL must use HTTPS.")
		return
	}
	if request.DiscordID != nil {
		value := strings.TrimSpace(*request.DiscordID)
		if len(value) > 100 {
			writeErrorResponse(w, 400, "Discord must be at most 100 characters.")
			return
		}
		request.DiscordID = &value
	}
	v, err := a.db.UpdateAdminProfile(r.Context(), accounts.AdminProfile{ID: r.PathValue("id"), Name: request.Name, Slug: request.Slug, AvatarURL: request.AvatarURL, DiscordID: request.DiscordID}, actorFrom(r.Context()).UserID, time.Now().UTC())
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}
	writeJSONResponse(w, 200, v)
}
func (a *API) requestAdminEmailChange(w http.ResponseWriter, r *http.Request) {
	if !a.authorizeAdminProfile(w, r) {
		return
	}
	var request struct {
		Email string `json:"email"`
	}
	if err := decodeJSON(r.Body, &request); err != nil {
		writeErrorResponse(w, 400, err.Error())
		return
	}
	request.Email = strings.ToLower(strings.TrimSpace(request.Email))
	address, err := mail.ParseAddress(request.Email)
	if err != nil || address.Address != request.Email || len(request.Email) > 254 {
		writeErrorResponse(w, 400, "Enter a valid new email address.")
		return
	}
	if a.requestEmailChange == nil {
		writeErrorResponse(w, 503, "Email verification is temporarily unavailable.")
		return
	}
	subject, err := a.db.ResolveAuthSubjectForUser(r.Context(), r.PathValue("id"))
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}
	actorID := actorFrom(r.Context()).UserID
	if err = a.db.AppendAudit(r.Context(), audit.Event{ID: id.New(), ActorID: &actorID, Action: "user.email_change_requested", SubjectType: "user", SubjectID: r.PathValue("id"), Data: map[string]any{"newEmail": request.Email}, OccurredAt: time.Now().UTC()}); err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}
	if err = a.requestEmailChange(r.Context(), authadmin.EmailChangeInput{AuthUserID: subject, Email: request.Email, DeliveryID: id.New()}); err != nil {
		writeErrorResponse(w, 502, "Verification could not be requested. Check that this email is not already in use, then try again.")
		return
	}
	writeJSONResponse(w, 202, map[string]string{"message": "Verification sent to the new address. The current email remains until it is verified."})
}
