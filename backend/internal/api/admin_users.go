// Package api implements the RSP HTTP API.
package api

import (
	"net/http"
	"strings"
	"time"

	"github.com/magedmg/RSP-website/backend/internal/accounts"
	"github.com/magedmg/RSP-website/backend/internal/authz"
	"github.com/magedmg/RSP-website/backend/internal/platform/audit"
	"github.com/magedmg/RSP-website/backend/internal/platform/id"
)

func (a *API) auditSystemAdminPrivateRead(w http.ResponseWriter, r *http.Request, subjectType, subjectID string) bool {
	actor := actorFrom(r.Context())
	if !actor.IsGlobal(authz.SystemAdmin) || subjectID == actor.UserID && subjectType == "user" {
		return true
	}
	actorID := actor.UserID
	err := a.db.AppendAudit(r.Context(), audit.Event{ID: id.New(), ActorID: &actorID, Action: "private_data.viewed", SubjectType: subjectType, SubjectID: subjectID, Data: map[string]any{}, OccurredAt: time.Now().UTC()})
	if err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "audit_failed", "Private data was not returned because its access could not be audited.")
		return false
	}
	return true
}

func (a *API) systemAdmin(w http.ResponseWriter, r *http.Request) (authz.Actor, bool) {
	actor := actorFrom(r.Context())
	if !actor.IsGlobal(authz.SystemAdmin) || !actor.HasRecentMFA(time.Now()) {
		writeErrorResponse(w, http.StatusForbidden, "system_admin_mfa_required", "System Admin access with recent MFA is required.")
		return authz.Actor{}, false
	}
	return actor, true
}

func (a *API) listAdminUsers(w http.ResponseWriter, r *http.Request) {
	actor, ok := a.systemAdmin(w, r)
	if !ok {
		return
	}
	if _, err := requestedSort(r, "id:asc", "id:asc"); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "validation_failed", "sort must be id:asc")
		return
	}

	direction, err := requestedDirection(r)
	if err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "validation_failed", "direction must be forward or backward")
		return
	}

	query := strings.TrimSpace(r.URL.Query().Get("query"))
	accountState := r.URL.Query().Get("accountState")
	globalRole := r.URL.Query().Get("globalRole")
	if len(query) > 100 || accountState != "" && accountState != "active" && accountState != "suspended" && accountState != "deletion_pending" || globalRole != "" && globalRole != "director" && globalRole != "system_admin" {
		writeErrorResponse(w, http.StatusBadRequest, "validation_failed", "query and valid accountState/globalRole filters are required")
		return
	}
	binding := "admin-users|sort=id:asc|query=" + query + "|accountState=" + accountState + "|globalRole=" + globalRole
	limit, boundary, err := a.page(r, binding)
	if err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "invalid_cursor", "The cursor does not match the selected admin user filters and sort.")
		return
	}

	items, more, total, err := a.db.ListAdminUsers(r.Context(), boundary, limit, direction, query, accountState, globalRole)
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}
	if !a.auditSystemAdminPrivateRead(w, r, "admin_user_collection", actor.UserID) {
		return
	}
	pageInfo := pageInfoForKeyset(
		a, binding, direction, boundary, items, more,
		func(user accounts.User) string { return user.ID },
	)
	response := Page[accounts.User]{
		Items:      items,
		PageInfo:   pageInfo,
		TotalCount: total,
	}
	writeJSONResponse(w, http.StatusOK, response)
}

func (a *API) setUserAccountState(w http.ResponseWriter, r *http.Request) {
	actor, ok := a.systemAdmin(w, r)
	if !ok {
		return
	}
	var in struct {
		State    string `json:"state"`
		Reason   string `json:"reason"`
		Revision int64  `json:"revision"`
	}
	if err := decodeJSON(w, r, &in); err != nil || in.Revision < 1 || (in.State != "active" && in.State != "suspended") || strings.TrimSpace(in.Reason) == "" || len(in.Reason) > 500 {
		writeErrorResponse(w, http.StatusBadRequest, "validation_failed", "state active or suspended, revision, and a reason up to 500 characters are required")
		return
	}

	targetID := r.PathValue("id")
	if a.setAccountState == nil {
		writeErrorResponse(w, http.StatusServiceUnavailable, "auth_service_unavailable", "Account state administration is temporarily unavailable.")
		return
	}
	current, err := a.db.GetUser(r.Context(), targetID)
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}
	if current.Revision != in.Revision {
		writeErrorResponse(w, http.StatusConflict, "stale_revision", "The resource changed since it was loaded.")
		return
	}
	targetID = current.ID
	if targetID == actor.UserID && in.State == "suspended" {
		writeErrorResponse(w, http.StatusConflict, "self_suspension_forbidden", "A System Admin cannot suspend their own account.")
		return
	}
	subject, err := a.db.ResolveAuthSubjectForUser(r.Context(), targetID)
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}
	if err := a.setAccountState(r.Context(), subject, in.State, strings.TrimSpace(in.Reason), actor.UserID); err != nil {
		writeErrorResponse(w, http.StatusBadGateway, "auth_service_failed", "The account state was not changed.")
		return
	}

	updated, err := a.db.GetUser(r.Context(), targetID)
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}

	writeJSONResponse(w, http.StatusOK, updated)
}

func (a *API) grantUserGlobalRole(w http.ResponseWriter, r *http.Request) {
	actor, ok := a.systemAdmin(w, r)
	if !ok {
		return
	}
	var in struct {
		Role   string `json:"role"`
		Reason string `json:"reason"`
	}
	if err := decodeJSON(w, r, &in); err != nil || (in.Role != "director" && in.Role != "system_admin") || strings.TrimSpace(in.Reason) == "" || len(in.Reason) > 500 {
		writeErrorResponse(w, http.StatusBadRequest, "validation_failed", "role and a reason up to 500 characters are required")
		return
	}
	if a.getMFAState == nil {
		writeErrorResponse(w, http.StatusServiceUnavailable, "auth_service_unavailable", "MFA state could not be verified.")
		return
	}
	target, err := a.db.GetUser(r.Context(), r.PathValue("id"))
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}

	subject, err := a.db.ResolveAuthSubjectForUser(r.Context(), target.ID)
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}

	configured, err := a.getMFAState(r.Context(), subject)
	if err != nil {
		writeErrorResponse(w, http.StatusBadGateway, "auth_service_failed", "MFA state could not be verified.")
		return
	}

	assignment, err := a.db.GrantGlobalRole(r.Context(), target.ID, in.Role, configured, strings.TrimSpace(in.Reason), actor.UserID, time.Now().UTC())
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}

	writeJSONResponse(w, http.StatusCreated, assignment)
}

func (a *API) listUserGlobalRoles(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.systemAdmin(w, r); !ok {
		return
	}
	target, err := a.db.GetUser(r.Context(), r.PathValue("id"))
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}

	items, err := a.db.ListGlobalRoles(r.Context(), target.ID)
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}

	writeJSONResponse(w, http.StatusOK, map[string]any{"items": items})
}

func (a *API) revokeUserGlobalRole(w http.ResponseWriter, r *http.Request) {
	actor, ok := a.systemAdmin(w, r)
	if !ok {
		return
	}
	targetID, role := r.PathValue("id"), r.PathValue("role")
	if (role != "director" && role != "system_admin") || strings.TrimSpace(r.URL.Query().Get("reason")) == "" || len(r.URL.Query().Get("reason")) > 500 {
		writeErrorResponse(w, http.StatusBadRequest, "validation_failed", "valid role, revision, and reason up to 500 characters are required")
		return
	}
	target, err := a.db.GetUser(r.Context(), targetID)
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}

	targetID = target.ID
	if targetID == actor.UserID && role == "system_admin" {
		writeErrorResponse(w, http.StatusConflict, "self_role_revocation_forbidden", "A System Admin cannot revoke their own System Admin role.")
		return
	}
	revision, err := parseRevision(r)
	if err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "validation_failed", "valid role, revision, and reason up to 500 characters are required")
		return
	}

	assignment, err := a.db.RevokeGlobalRole(r.Context(), targetID, role, revision, strings.TrimSpace(r.URL.Query().Get("reason")), actor.UserID, time.Now().UTC())
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}

	writeJSONResponse(w, http.StatusOK, assignment)
}
