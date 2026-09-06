package api

import (
	"net/http"
	"strings"
	"time"

	"github.com/magedmg/RSP-website/backend/internal/accounts"
	"github.com/magedmg/RSP-website/backend/internal/authz"
	"github.com/magedmg/RSP-website/backend/internal/dal"
	"github.com/magedmg/RSP-website/backend/internal/platform/id"
	"github.com/magedmg/RSP-website/backend/internal/programme"
)

func (a *API) listSeasonMembers(w http.ResponseWriter, r *http.Request) {
	actor := actorFrom(r.Context())
	seasonID := r.PathValue("id")
	if !actor.CanViewSeason(seasonID) {
		writeErrorResponse(w, http.StatusForbidden, "No season access yet.")
		return
	}
	canSeeInactive := actor.CanReviewSeason(seasonID)
	role, state := r.URL.Query().Get("role"), r.URL.Query().Get("state")
	if role != "" && role != "student" && role != "mentor" && role != "coordinator" {
		writeErrorResponse(w, http.StatusBadRequest, "role must be student, mentor, or coordinator")
		return
	}
	if state != "" && state != "active" && state != "completed" && state != "kicked" && state != "withdrawn" {
		writeErrorResponse(w, http.StatusBadRequest, "state must be active, completed, kicked, or withdrawn")
		return
	}
	sortBy, err := parseSort(r.URL.Query().Get("sort"), "id:asc", "id:asc", "role:asc")
	if err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "sort must be id:asc or role:asc")
		return
	}

	direction, err := parsePageDirection(r.URL.Query().Get("direction"))
	if err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "direction must be forward or backward")
		return
	}

	binding := "members|season=" + seasonID + "|role=" + role + "|state=" + state + "|sort=" + sortBy
	limit, boundary, err := parsePagination(r.URL.Query().Get("limit"), r.URL.Query().Get("cursor"), binding, a.cursorSecret)
	if err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "The cursor does not match the selected member filters.")
		return
	}

	items, more, total, err := a.db.ListEnrollments(r.Context(), dal.EnrollmentQuery{
		SeasonID:        seasonID,
		Boundary:        boundary,
		Limit:           limit,
		Role:            role,
		State:           state,
		SortBy:          sortBy,
		Direction:       direction,
		IncludeInactive: canSeeInactive,
	})
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}

	userIDs := make([]string, 0, len(items))
	for _, enrollment := range items {
		userIDs = append(userIDs, enrollment.UserID)
	}
	members, err := a.db.SeasonMemberSummaries(r.Context(), seasonID, userIDs)
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}
	for i := range items {
		if member, ok := members[items[i].UserID]; ok {
			items[i].Member = &member
		}
		if !actor.IsSeasonAdmin(seasonID) {
			items[i].RemovalReason = nil
		}
	}
	pageInfo, err := pageInfoForKeyset(
		a.cursorSecret, binding, direction, boundary, items, more,
		func(member programme.EnrollmentRecord) string { return member.ID },
	)
	if err != nil {
		a.logger.Error("cursor encoding failed", "error", err)
		writeErrorResponse(w, http.StatusInternalServerError, "The request could not be completed.")
		return
	}
	response := Page[programme.EnrollmentRecord]{
		Items:      items,
		PageInfo:   pageInfo,
		TotalCount: total,
	}
	writeJSONResponse(w, http.StatusOK, response)
}

func (a *API) listEnrollmentCandidates(w http.ResponseWriter, r *http.Request) {
	actor := actorFrom(r.Context())
	managedSeason, err := a.db.GetSeason(r.Context(), r.PathValue("id"))
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}
	if !actor.IsSeasonAdmin(managedSeason.ID) {
		writeErrorResponse(w, http.StatusForbidden, "Season administrator access is required.")
		return
	}
	if managedSeason.Status != "open" {
		writeErrorResponse(w, http.StatusForbidden, "The season must be open.")
		return
	}
	if !actor.HasRecentMFA(time.Now()) {
		writeErrorResponse(w, http.StatusForbidden, "Recent MFA is required.")
		return
	}
	query := strings.TrimSpace(r.URL.Query().Get("query"))
	if _, err := parseSort(r.URL.Query().Get("sort"), "id:asc", "id:asc"); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "sort must be id:asc")
		return
	}

	direction, err := parsePageDirection(r.URL.Query().Get("direction"))
	if err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "direction must be forward or backward")
		return
	}

	binding := "enrollment-candidates|id:asc|season=" + r.PathValue("id") + "|query=" + query
	limit, boundary, err := parsePagination(r.URL.Query().Get("limit"), r.URL.Query().Get("cursor"), binding, a.cursorSecret)
	if err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "cursor is invalid for the selected season and query")
		return
	}

	items, more, total, err := a.db.ListEnrollmentCandidates(r.Context(), dal.EnrollmentCandidateQuery{
		SeasonID:  r.PathValue("id"),
		Search:    query,
		Boundary:  boundary,
		Limit:     limit,
		Direction: direction,
	})
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}

	pageInfo, err := pageInfoForKeyset(
		a.cursorSecret, binding, direction, boundary, items, more,
		func(record accounts.EnrollmentCandidate) string { return record.ID },
	)
	if err != nil {
		a.logger.Error("cursor encoding failed", "error", err)
		writeErrorResponse(w, http.StatusInternalServerError, "The request could not be completed.")
		return
	}
	response := Page[accounts.EnrollmentCandidate]{
		Items:      items,
		PageInfo:   pageInfo,
		TotalCount: total,
	}
	writeJSONResponse(w, http.StatusOK, response)
}

type createSeasonMemberRequest struct {
	UserID string `json:"userId"`
	Role   string `json:"role"`
}

func (a *API) createSeasonMember(w http.ResponseWriter, r *http.Request) {
	actor := actorFrom(r.Context())
	managedSeason, err := a.db.GetSeason(r.Context(), r.PathValue("id"))
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}
	if !actor.IsSeasonAdmin(managedSeason.ID) {
		writeErrorResponse(w, http.StatusForbidden, "Season administrator access is required.")
		return
	}
	if managedSeason.Status != "open" {
		writeErrorResponse(w, http.StatusForbidden, "The season must be open.")
		return
	}
	if !actor.HasRecentMFA(time.Now()) {
		writeErrorResponse(w, http.StatusForbidden, "Recent MFA is required.")
		return
	}
	var request createSeasonMemberRequest
	if err := decodeJSON(r.Body, &request); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	if strings.TrimSpace(request.UserID) == "" {
		writeErrorResponse(w, http.StatusBadRequest, "userId is required")
		return
	}
	switch request.Role {
	case "student", "mentor", "coordinator":
	default:
		writeErrorResponse(w, http.StatusBadRequest, "role must be student, mentor, or coordinator")
		return
	}
	if request.Role == "coordinator" && !canGrantCoordinator(actor, r.PathValue("id")) {
		writeErrorResponse(w, http.StatusForbidden, "Only the season Coordinator, a Director, or a System Admin may grant Coordinator access.")
		return
	}
	member := programme.EnrollmentRecord{
		ID:       id.New(),
		SeasonID: r.PathValue("id"),
		UserID:   request.UserID,
		Role:     request.Role,
		State:    "active",
	}
	created, err := a.db.CreateEnrollment(r.Context(), member, actor.UserID, time.Now().UTC())
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}

	writeJSONResponse(w, http.StatusCreated, created)
}

type updateSeasonMemberRequest struct {
	Role         string `json:"role"`
	StudentLevel string `json:"studentLevel"`
}

func (a *API) updateSeasonMember(w http.ResponseWriter, r *http.Request) {
	actor := actorFrom(r.Context())
	managedSeason, err := a.db.GetSeason(r.Context(), r.PathValue("id"))
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}
	if !actor.IsSeasonAdmin(managedSeason.ID) {
		writeErrorResponse(w, http.StatusForbidden, "Season administrator access is required.")
		return
	}
	if managedSeason.Status != "open" {
		writeErrorResponse(w, http.StatusForbidden, "The season must be open.")
		return
	}
	if !actor.HasRecentMFA(time.Now()) {
		writeErrorResponse(w, http.StatusForbidden, "Recent MFA is required.")
		return
	}
	var request updateSeasonMemberRequest

	if err := decodeJSON(r.Body, &request); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	switch request.Role {
	case "student":
		if !isValidStudentLevel(request.StudentLevel) {
			writeErrorResponse(w, http.StatusBadRequest, "studentLevel must be novice, beginner, intermediate, or advanced for students")
			return
		}
	case "mentor":
		request.StudentLevel = "not_applicable"
	default:
		writeErrorResponse(w, http.StatusBadRequest, "role must be student or mentor")
		return
	}
	member, err := a.db.UpdateEnrollmentRoleAndLevel(r.Context(), dal.UpdateEnrollmentRoleAndLevelInput{
		SeasonID:     r.PathValue("id"),
		EnrollmentID: r.PathValue("memberId"),
		Role:         request.Role,
		StudentLevel: request.StudentLevel,
		ActorID:      actor.UserID,
		ChangedAt:    time.Now().UTC(),
	})
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}

	writeJSONResponse(w, http.StatusOK, member)
}

type promoteSeasonMemberRequest struct {
	Role   string `json:"role"`
	Reason string `json:"reason"`
}

func (a *API) promoteSeasonMember(w http.ResponseWriter, r *http.Request) {
	actor := actorFrom(r.Context())
	seasonID, memberID := r.PathValue("id"), r.PathValue("memberId")
	season, err := a.db.GetSeason(r.Context(), seasonID)
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}
	if season.Status != "open" {
		writeErrorResponse(w, http.StatusConflict, "Season members cannot be changed until the season is reopened.")
		return
	}
	member, err := a.db.GetEnrollment(r.Context(), memberID)
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}
	if member.SeasonID != seasonID {
		writeErrorResponse(w, http.StatusNotFound, "The enrollment does not exist.")
		return
	}
	if member.Role != "student" {
		writeErrorResponse(w, http.StatusForbidden, "Only an active student enrollment can be promoted or removed through this operation.")
		return
	}
	assigned, err := a.db.IsMentorAssigned(r.Context(), seasonID, actor.UserID, member.UserID)
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}
	if !actor.CanPromoteOrRemoveStudent(seasonID, assigned, member.State == "active") {
		writeErrorResponse(w, http.StatusForbidden, "This member cannot be changed by the current account.")
		return
	}
	seasonRole, _ := actor.Enrollment(seasonID)
	if (actor.IsDirectorOrSystemAdmin() || seasonRole.Role == authz.Coordinator) && !actor.HasRecentMFA(time.Now()) {
		writeErrorResponse(w, http.StatusForbidden, "Recent MFA is required for this administrative action.")
		return
	}
	var request promoteSeasonMemberRequest
	if err := decodeJSON(r.Body, &request); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	if request.Role != "mentor" && request.Role != "coordinator" {
		writeErrorResponse(w, http.StatusBadRequest, "role must be mentor or coordinator")
		return
	}
	if request.Role == "coordinator" && !canGrantCoordinator(actor, seasonID) {
		writeErrorResponse(w, http.StatusForbidden, "Only the season Coordinator, a Director, or a System Admin may grant Coordinator access.")
		return
	}
	updated, err := a.db.PromoteEnrollment(r.Context(), dal.PromoteEnrollmentInput{
		EnrollmentID: memberID,
		Role:         request.Role,
		ActorID:      actor.UserID,
		ChangedAt:    time.Now().UTC(),
	})

	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}
	writeJSONResponse(w, http.StatusOK, updated)
}

type removeSeasonMemberRequest struct {
	Role   string `json:"role"`
	Reason string `json:"reason"`
}

func (a *API) removeSeasonMember(w http.ResponseWriter, r *http.Request) {
	actor := actorFrom(r.Context())
	seasonID, memberID := r.PathValue("id"), r.PathValue("memberId")
	season, err := a.db.GetSeason(r.Context(), seasonID)
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}
	if season.Status != "open" {
		writeErrorResponse(w, http.StatusConflict, "Season members cannot be changed until the season is reopened.")
		return
	}
	member, err := a.db.GetEnrollment(r.Context(), memberID)
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}
	if member.SeasonID != seasonID {
		writeErrorResponse(w, http.StatusNotFound, "The enrollment does not exist.")
		return
	}
	if member.Role == "student" {
		assigned, err := a.db.IsMentorAssigned(r.Context(), seasonID, actor.UserID, member.UserID)
		if err != nil {
			a.writeStoreErrorResponse(w, err)
			return
		}
		if !actor.CanPromoteOrRemoveStudent(seasonID, assigned, member.State == "active") {
			writeErrorResponse(w, 403, "This member cannot be removed by the current account.")
			return
		}
	} else if !actor.IsSeasonAdmin(seasonID) || member.State != "active" {
		writeErrorResponse(w, 403, "Only a season administrator can remove an active mentor or coordinator.")
		return
	}
	seasonRole, _ := actor.Enrollment(seasonID)
	if (actor.IsDirectorOrSystemAdmin() || seasonRole.Role == authz.Coordinator) && !actor.HasRecentMFA(time.Now()) {
		writeErrorResponse(w, http.StatusForbidden, "Recent MFA is required for this administrative action.")
		return
	}
	var request removeSeasonMemberRequest
	if err := decodeJSON(r.Body, &request); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	if strings.TrimSpace(request.Reason) == "" || len(request.Reason) > 500 {
		writeErrorResponse(w, http.StatusBadRequest, "removal reason is required")
		return
	}
	updated, err := a.db.RemoveEnrollment(r.Context(), dal.RemoveEnrollmentInput{
		EnrollmentID: memberID,
		Reason:       request.Reason,
		ActorID:      actor.UserID,
		ChangedAt:    time.Now().UTC(),
	})

	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}
	writeJSONResponse(w, http.StatusOK, updated)
}

func canGrantCoordinator(actor authz.Actor, seasonID string) bool {
	if actor.IsDirectorOrSystemAdmin() {
		return true
	}
	enrollment, ok := actor.Enrollment(seasonID)
	return ok && enrollment.Role == authz.Coordinator && enrollment.State == authz.Active
}

func isValidStudentLevel(level string) bool {
	switch level {
	case "novice", "beginner", "intermediate", "advanced":
		return true
	default:
		return false
	}
}
