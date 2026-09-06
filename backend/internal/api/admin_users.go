// Package api implements the RSP HTTP API.
package api

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/magedmg/RSP-website/backend/internal/accounts"
	"github.com/magedmg/RSP-website/backend/internal/authz"
	"github.com/magedmg/RSP-website/backend/internal/dal"
	"github.com/magedmg/RSP-website/backend/internal/platform/audit"
	"github.com/magedmg/RSP-website/backend/internal/platform/id"
)

func (a *API) auditPrivateDataRead(ctx context.Context, actor authz.Actor, subjectType, subjectID string) error {
	if !actor.HasGlobalRole(authz.SystemAdmin) || subjectID == actor.UserID && subjectType == "user" {
		return nil
	}
	actorID := actor.UserID
	return a.db.AppendAudit(ctx, audit.Event{
		ID:          id.New(),
		ActorID:     &actorID,
		Action:      "private_data.viewed",
		SubjectType: subjectType,
		SubjectID:   subjectID,
		Data:        map[string]any{},
		OccurredAt:  time.Now().UTC(),
	})
}

func (a *API) listAdminUsers(w http.ResponseWriter, r *http.Request) {
	actor := actorFrom(r.Context())
	if !actor.HasGlobalRole(authz.SystemAdmin) || !actor.HasRecentMFA(time.Now()) {
		writeErrorResponse(w, http.StatusForbidden, "system_admin_mfa_required", "System Admin access with recent MFA is required.")
		return
	}
	if _, err := parseSort(r.URL.Query().Get("sort"), "id:asc", "id:asc"); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "validation_failed", "sort must be id:asc")
		return
	}

	direction, err := parsePageDirection(r.URL.Query().Get("direction"))
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
	limit, boundary, err := parsePagination(r.URL.Query().Get("limit"), r.URL.Query().Get("cursor"), binding, a.cursorSecret)
	if err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "invalid_cursor", "The cursor does not match the selected admin user filters and sort.")
		return
	}

	items, more, total, err := a.db.ListAdminUsers(r.Context(), dal.AdminUserQuery{
		Boundary:     boundary,
		Limit:        limit,
		Direction:    direction,
		Search:       query,
		AccountState: accountState,
		GlobalRole:   globalRole,
	})
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}
	if err := a.auditPrivateDataRead(r.Context(), actor, "admin_user_collection", actor.UserID); err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "audit_failed", "Private data was not returned because its access could not be audited.")
		return
	}
	pageInfo := pageInfoForKeyset(
		a.cursorSecret, binding, direction, boundary, items, more,
		func(user accounts.User) string { return user.ID },
	)
	response := Page[accounts.User]{
		Items:      items,
		PageInfo:   pageInfo,
		TotalCount: total,
	}
	writeJSONResponse(w, http.StatusOK, response)
}

type setUserAccountStateRequest struct {
	State  string `json:"state"`
	Reason string `json:"reason"`
}

func (a *API) setUserAccountState(w http.ResponseWriter, r *http.Request) {
	actor := actorFrom(r.Context())
	if !actor.HasGlobalRole(authz.SystemAdmin) || !actor.HasRecentMFA(time.Now()) {
		writeErrorResponse(w, http.StatusForbidden, "system_admin_mfa_required", "System Admin access with recent MFA is required.")
		return
	}
	var request setUserAccountStateRequest
	if err := decodeJSON(r.Body, &request); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "validation_failed", err.Error())
		return
	}
	if (request.State != "active" && request.State != "suspended") || strings.TrimSpace(request.Reason) == "" || len(request.Reason) > 500 {
		writeErrorResponse(w, http.StatusBadRequest, "validation_failed", "state active or suspended and a reason up to 500 characters are required")
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
	targetID = current.ID
	if targetID == actor.UserID && request.State == "suspended" {
		writeErrorResponse(w, http.StatusConflict, "self_suspension_forbidden", "A System Admin cannot suspend their own account.")
		return
	}
	subject, err := a.db.ResolveAuthSubjectForUser(r.Context(), targetID)
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}
	if err := a.setAccountState(r.Context(), subject, request.State, strings.TrimSpace(request.Reason), actor.UserID); err != nil {
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

type grantUserGlobalRoleRequest struct {
	Role   string `json:"role"`
	Reason string `json:"reason"`
}

func (a *API) grantUserGlobalRole(w http.ResponseWriter, r *http.Request) {
	actor := actorFrom(r.Context())
	if !actor.HasGlobalRole(authz.SystemAdmin) || !actor.HasRecentMFA(time.Now()) {
		writeErrorResponse(w, http.StatusForbidden, "system_admin_mfa_required", "System Admin access with recent MFA is required.")
		return
	}
	var request grantUserGlobalRoleRequest
	if err := decodeJSON(r.Body, &request); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "validation_failed", err.Error())
		return
	}
	if (request.Role != "director" && request.Role != "system_admin") || strings.TrimSpace(request.Reason) == "" || len(request.Reason) > 500 {
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

	assignment, err := a.db.GrantGlobalRole(r.Context(), dal.GrantGlobalRoleInput{
		UserID:        target.ID,
		Role:          request.Role,
		MFAConfigured: configured,
		Reason:        strings.TrimSpace(request.Reason),
		ActorID:       actor.UserID,
		ChangedAt:     time.Now().UTC(),
	})
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}

	writeJSONResponse(w, http.StatusCreated, assignment)
}

func (a *API) listUserGlobalRoles(w http.ResponseWriter, r *http.Request) {
	actor := actorFrom(r.Context())
	if !actor.HasGlobalRole(authz.SystemAdmin) || !actor.HasRecentMFA(time.Now()) {
		writeErrorResponse(w, http.StatusForbidden, "system_admin_mfa_required", "System Admin access with recent MFA is required.")
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
	actor := actorFrom(r.Context())
	if !actor.HasGlobalRole(authz.SystemAdmin) || !actor.HasRecentMFA(time.Now()) {
		writeErrorResponse(w, http.StatusForbidden, "system_admin_mfa_required", "System Admin access with recent MFA is required.")
		return
	}
	targetID, role := r.PathValue("id"), r.PathValue("role")
	if (role != "director" && role != "system_admin") || strings.TrimSpace(r.URL.Query().Get("reason")) == "" || len(r.URL.Query().Get("reason")) > 500 {
		writeErrorResponse(w, http.StatusBadRequest, "validation_failed", "valid role and reason up to 500 characters are required")
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
	assignment, err := a.db.RevokeGlobalRole(r.Context(), dal.RevokeGlobalRoleInput{
		UserID:    targetID,
		Role:      role,
		Reason:    strings.TrimSpace(r.URL.Query().Get("reason")),
		ActorID:   actor.UserID,
		ChangedAt: time.Now().UTC(),
	})
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}

	writeJSONResponse(w, http.StatusOK, assignment)
}
