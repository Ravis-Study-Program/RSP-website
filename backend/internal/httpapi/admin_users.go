// Package httpapi implements the RSP HTTP API.
package httpapi

import (
	"net/http"
	"strings"
	"time"

	"github.com/magedmg/RSP-website/backend/internal/authz"
	"github.com/magedmg/RSP-website/backend/internal/model"
	"github.com/magedmg/RSP-website/backend/internal/platform/id"
	"github.com/magedmg/RSP-website/backend/internal/store"
)

func (a *API) auditSystemAdminPrivateRead(w http.ResponseWriter, r *http.Request, subjectType, subjectID string) bool {
	actor := actorFrom(r.Context())
	if !actor.IsGlobal(authz.SystemAdmin) || subjectID == actor.UserID && subjectType == "user" {
		return true
	}
	actorID := actor.UserID
	err := a.store.AppendAudit(r.Context(), model.AuditEvent{ID: id.New(), ActorID: &actorID, Action: "private_data.viewed", SubjectType: subjectType, SubjectID: subjectID, Data: map[string]any{}, OccurredAt: time.Now().UTC()})
	if err != nil {
		a.fail(w, r, http.StatusInternalServerError, "audit_failed", "Audit failed", "Private data was not returned because its access could not be audited.", nil)
		return false
	}
	return true
}

func (a *API) systemAdmin(w http.ResponseWriter, r *http.Request) (authz.Actor, bool) {
	actor := actorFrom(r.Context())
	if !actor.IsGlobal(authz.SystemAdmin) || !actor.HasRecentMFA(time.Now()) {
		a.fail(w, r, http.StatusForbidden, "system_admin_mfa_required", "System Admin access required", "System Admin access with recent MFA is required.", nil)
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
		validation(a, w, r, "sort must be id:asc")
		return
	}

	direction, err := requestedDirection(r)
	if err != nil {
		validation(a, w, r, "direction must be forward or backward")
		return
	}

	query := strings.TrimSpace(r.URL.Query().Get("query"))
	accountState := r.URL.Query().Get("accountState")
	globalRole := r.URL.Query().Get("globalRole")
	if len(query) > 100 || accountState != "" && accountState != "active" && accountState != "suspended" && accountState != "deletion_pending" || globalRole != "" && globalRole != "director" && globalRole != "system_admin" {
		validation(a, w, r, "query and valid accountState/globalRole filters are required")
		return
	}
	binding := "admin-users|sort=id:asc|query=" + query + "|accountState=" + accountState + "|globalRole=" + globalRole
	limit, boundary, err := a.page(r, binding)
	if err != nil {
		a.fail(w, r, http.StatusBadRequest, "invalid_cursor", "Invalid cursor", "The cursor does not match the selected admin user filters and sort.", nil)
		return
	}

	items, more, total, err := a.store.ListAdminUsers(r.Context(), boundary, limit, direction, query, accountState, globalRole)
	if err != nil {
		storeFailure(a, w, r, err)
		return
	}
	if !a.auditSystemAdminPrivateRead(w, r, "admin_user_collection", actor.UserID) {
		return
	}
	writeJSON(w, http.StatusOK, model.Page[model.User]{Items: items, PageInfo: pageInfoForKeyset(a, binding, direction, boundary, items, more, func(user model.User) string { return user.ID }), TotalCount: total})
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
		validation(a, w, r, "state active or suspended, revision, and a reason up to 500 characters are required")
		return
	}

	targetID := r.PathValue("id")
	if a.setAccountState == nil {
		a.fail(w, r, http.StatusServiceUnavailable, "auth_service_unavailable", "Authentication service unavailable", "Account state administration is temporarily unavailable.", nil)
		return
	}
	current, err := a.store.GetUser(r.Context(), targetID)
	if err != nil {
		storeFailure(a, w, r, err)
		return
	}
	if current.Revision != in.Revision {
		storeFailure(a, w, r, store.ErrConflict)
		return
	}
	targetID = current.ID
	if targetID == actor.UserID && in.State == "suspended" {
		a.fail(w, r, http.StatusConflict, "self_suspension_forbidden", "Conflict", "A System Admin cannot suspend their own account.", nil)
		return
	}
	subject, err := a.store.ResolveAuthSubjectForUser(r.Context(), targetID)
	if err != nil {
		storeFailure(a, w, r, err)
		return
	}
	if err := a.setAccountState(r.Context(), subject, in.State, strings.TrimSpace(in.Reason), actor.UserID); err != nil {
		a.fail(w, r, http.StatusBadGateway, "auth_service_failed", "Authentication service failed", "The account state was not changed.", nil)
		return
	}

	updated, err := a.store.GetUser(r.Context(), targetID)
	if err != nil {
		storeFailure(a, w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, updated)
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
		validation(a, w, r, "role and a reason up to 500 characters are required")
		return
	}
	if a.getMFAState == nil {
		a.fail(w, r, http.StatusServiceUnavailable, "auth_service_unavailable", "Authentication service unavailable", "MFA state could not be verified.", nil)
		return
	}
	target, err := a.store.GetUser(r.Context(), r.PathValue("id"))
	if err != nil {
		storeFailure(a, w, r, err)
		return
	}

	subject, err := a.store.ResolveAuthSubjectForUser(r.Context(), target.ID)
	if err != nil {
		storeFailure(a, w, r, err)
		return
	}

	configured, err := a.getMFAState(r.Context(), subject)
	if err != nil {
		a.fail(w, r, http.StatusBadGateway, "auth_service_failed", "Authentication service failed", "MFA state could not be verified.", nil)
		return
	}

	assignment, err := a.store.GrantGlobalRole(r.Context(), target.ID, in.Role, configured, strings.TrimSpace(in.Reason), actor.UserID, time.Now().UTC())
	if err != nil {
		storeFailure(a, w, r, err)
		return
	}

	writeJSON(w, http.StatusCreated, assignment)
}

func (a *API) listUserGlobalRoles(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.systemAdmin(w, r); !ok {
		return
	}
	target, err := a.store.GetUser(r.Context(), r.PathValue("id"))
	if err != nil {
		storeFailure(a, w, r, err)
		return
	}

	items, err := a.store.ListGlobalRoles(r.Context(), target.ID)
	if err != nil {
		storeFailure(a, w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (a *API) revokeUserGlobalRole(w http.ResponseWriter, r *http.Request) {
	actor, ok := a.systemAdmin(w, r)
	if !ok {
		return
	}
	targetID, role := r.PathValue("id"), r.PathValue("role")
	if (role != "director" && role != "system_admin") || strings.TrimSpace(r.URL.Query().Get("reason")) == "" || len(r.URL.Query().Get("reason")) > 500 {
		validation(a, w, r, "valid role, revision, and reason up to 500 characters are required")
		return
	}
	target, err := a.store.GetUser(r.Context(), targetID)
	if err != nil {
		storeFailure(a, w, r, err)
		return
	}

	targetID = target.ID
	if targetID == actor.UserID && role == "system_admin" {
		a.fail(w, r, http.StatusConflict, "self_role_revocation_forbidden", "Conflict", "A System Admin cannot revoke their own System Admin role.", nil)
		return
	}
	revision, err := parseRevision(r)
	if err != nil {
		validation(a, w, r, "valid role, revision, and reason up to 500 characters are required")
		return
	}

	assignment, err := a.store.RevokeGlobalRole(r.Context(), targetID, role, revision, strings.TrimSpace(r.URL.Query().Get("reason")), actor.UserID, time.Now().UTC())
	if err != nil {
		storeFailure(a, w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, assignment)
}
