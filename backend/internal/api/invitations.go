package api

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"github.com/magedmg/RSP-website/backend/internal/dal"
	"net/http"
	"net/mail"
	"strings"
	"time"

	"github.com/magedmg/RSP-website/backend/internal/authadmin"
	"github.com/magedmg/RSP-website/backend/internal/platform/id"
	"github.com/magedmg/RSP-website/backend/internal/programme"
)

func invitationToken() (string, string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", "", err
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	return token, hashInvitationToken(token), nil
}
func hashInvitationToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}
func (a *API) authorizeInvitations(w http.ResponseWriter, r *http.Request, write bool) bool {
	actor := actorFrom(r.Context())
	seasonID := r.PathValue("id")
	if !actor.IsSeasonAdmin(seasonID) || !actor.HasRecentMFA(time.Now()) {
		writeErrorResponse(w, http.StatusForbidden, "Season administrator access with recent MFA is required.")
		return false
	}
	season, err := a.db.GetSeason(r.Context(), seasonID)
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return false
	}
	if write && season.Status != "open" {
		writeErrorResponse(w, http.StatusConflict, "Reopen this season before changing invitations.")
		return false
	}
	return true
}
func (a *API) listInvitations(w http.ResponseWriter, r *http.Request) {
	if !a.authorizeInvitations(w, r, false) {
		return
	}
	binding := "invitations|" + r.PathValue("id")
	limit, boundary, err := parsePagination(r.URL.Query().Get("limit"), r.URL.Query().Get("cursor"), binding, a.cursorSecret)
	if err != nil {
		writeErrorResponse(w, 400, "Invalid invitation cursor.")
		return
	}
	items, more, total, err := a.db.ListInvitations(r.Context(), r.PathValue("id"), boundary, limit)
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}
	pageInfo, err := pageInfoForKeyset(a.cursorSecret, binding, "forward", boundary, items, more, func(v programme.Invitation) string { return v.ID })
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}
	writeJSONResponse(w, 200, Page[programme.Invitation]{Items: items, PageInfo: pageInfo, TotalCount: total})
}
func (a *API) sendInvitationEmail(r *http.Request, v *programme.Invitation, token string) {
	if a.sendInvitation == nil {
		return
	}
	err := a.sendInvitation(r.Context(), authadmin.InvitationEmailInput{ID: v.DeliveryID, To: v.Email, Name: v.Name, Season: v.SeasonName, Role: v.Role, URL: strings.TrimRight(a.publicOrigin, "/") + "/invitations/accept#token=" + token})
	if err != nil {
		a.logger.Error("invitation email enqueue failed", "invitationId", v.ID)
		return
	}
	at := time.Now().UTC()
	if err = a.db.MarkInvitationSent(r.Context(), v.ID, v.DeliveryID, at); err != nil {
		a.logger.Error("invitation delivery receipt failed", "invitationId", v.ID)
		return
	}
	v.SentAt = &at
}
func (a *API) createInvitation(w http.ResponseWriter, r *http.Request) {
	if !a.authorizeInvitations(w, r, true) {
		return
	}
	var request struct {
		Name  string `json:"name"`
		Email string `json:"email"`
		Role  string `json:"role"`
	}
	if err := decodeJSON(r.Body, &request); err != nil {
		writeErrorResponse(w, 400, err.Error())
		return
	}
	request.Name = strings.TrimSpace(request.Name)
	request.Email = strings.ToLower(strings.TrimSpace(request.Email))
	address, err := mail.ParseAddress(request.Email)
	if err != nil || address.Address != request.Email || len(request.Email) > 254 || len(request.Name) < 1 || len(request.Name) > 100 {
		writeErrorResponse(w, 400, "Enter a name and a valid email address.")
		return
	}
	if request.Role != "student" && request.Role != "mentor" && request.Role != "coordinator" {
		writeErrorResponse(w, 400, "Choose student, mentor or coordinator.")
		return
	}
	if request.Role == "coordinator" && !canGrantCoordinator(actorFrom(r.Context()), r.PathValue("id")) {
		writeErrorResponse(w, 403, "Coordinator access cannot be granted.")
		return
	}
	token, hash, err := invitationToken()
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}
	at := time.Now().UTC()
	v, err := a.db.CreateInvitation(r.Context(), programme.Invitation{ID: id.New(), SeasonID: r.PathValue("id"), Name: request.Name, Email: request.Email, Role: request.Role, DeliveryID: id.New(), ExpiresAt: at.Add(7 * 24 * time.Hour)}, hash, actorFrom(r.Context()).UserID, at)
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}
	a.sendInvitationEmail(r, &v, token)
	writeJSONResponse(w, 201, v)
}
func (a *API) resendInvitation(w http.ResponseWriter, r *http.Request) {
	a.changeInvitation(w, r, false)
}
func (a *API) cancelInvitation(w http.ResponseWriter, r *http.Request) {
	a.changeInvitation(w, r, true)
}
func (a *API) changeInvitation(w http.ResponseWriter, r *http.Request, cancel bool) {
	if !a.authorizeInvitations(w, r, true) {
		return
	}
	token, hash, err := invitationToken()
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}
	v, err := a.db.ChangeInvitation(r.Context(), r.PathValue("id"), r.PathValue("invitationId"), hash, id.New(), actorFrom(r.Context()).UserID, cancel, time.Now().UTC())
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}
	if !cancel {
		a.sendInvitationEmail(r, &v, token)
	}
	writeJSONResponse(w, 200, v)
}
func invitationRequest(w http.ResponseWriter, r *http.Request) (string, bool) {
	var request struct {
		Token string `json:"token"`
	}
	if err := decodeJSON(r.Body, &request); err != nil || len(request.Token) != 43 {
		writeErrorResponse(w, 400, "This invitation link is invalid.")
		return "", false
	}
	return hashInvitationToken(request.Token), true
}
func (a *API) previewInvitation(w http.ResponseWriter, r *http.Request) {
	hash, ok := invitationRequest(w, r)
	if !ok {
		return
	}
	v, err := a.db.InvitationForUser(r.Context(), hash, actorFrom(r.Context()).UserID)
	if errors.Is(err, dal.ErrNotFound) {
		writeErrorResponse(w, 404, "This invitation is expired, cancelled, already used, or belongs to another email address.")
		return
	}
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}
	writeJSONResponse(w, 200, v)
}
func (a *API) acceptInvitation(w http.ResponseWriter, r *http.Request) {
	hash, ok := invitationRequest(w, r)
	if !ok {
		return
	}
	v, err := a.db.AcceptInvitation(r.Context(), hash, actorFrom(r.Context()).UserID, time.Now().UTC())
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}
	writeJSONResponse(w, 200, v)
}
