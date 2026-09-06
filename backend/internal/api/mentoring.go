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

func canGrantCoordinator(actor authz.Actor, seasonID string) bool {
	if actor.IsDirectorOrSystemAdmin() {
		return true
	}
	enrollment, ok := actor.Enrollment(seasonID)
	return ok && enrollment.Role == authz.Coordinator && enrollment.State == authz.Active
}

func (a *API) listWeeks(w http.ResponseWriter, r *http.Request) {
	actor := actorFrom(r.Context())
	seasonID := r.PathValue("id")
	if !actor.CanViewSeason(seasonID) {
		writeErrorResponse(w, http.StatusForbidden, "season_access_required", "The current account cannot access this season.")
		return
	}
	if _, err := a.db.GetSeason(r.Context(), seasonID); err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}

	sortBy, err := parseSort(r.URL.Query().Get("sort"), "number:asc", "number:asc", "id:asc")
	if err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "validation_failed", "sort must be number:asc or id:asc")
		return
	}

	direction, err := parsePageDirection(r.URL.Query().Get("direction"))
	if err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "validation_failed", "direction must be forward or backward")
		return
	}

	binding := "weeks|season=" + seasonID + "|sort=" + sortBy
	limit, boundary, err := parsePagination(r.URL.Query().Get("limit"), r.URL.Query().Get("cursor"), binding, a.cursorSecret)
	if err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "invalid_cursor", "The cursor does not match the selected season and sort.")
		return
	}

	items, more, total, err := a.db.ListWeeks(r.Context(), dal.WeekQuery{
		SeasonID:  seasonID,
		Boundary:  boundary,
		Limit:     limit,
		SortBy:    sortBy,
		Direction: direction,
	})
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}

	pageInfo := pageInfoForKeyset(
		a.cursorSecret, binding, direction, boundary, items, more,
		func(week programme.WeekRecord) string { return week.ID },
	)
	response := Page[programme.WeekRecord]{
		Items:      items,
		PageInfo:   pageInfo,
		TotalCount: total,
	}
	writeJSONResponse(w, http.StatusOK, response)
}

type createWeekRequest struct {
	Number      int       `json:"number"`
	StartAt     time.Time `json:"startAt"`
	EndAt       time.Time `json:"endAt"`
	ResourceURL string    `json:"resourceUrl"`
}

func (a *API) createWeek(w http.ResponseWriter, r *http.Request) {
	actor := actorFrom(r.Context())
	season, err := a.db.GetSeason(r.Context(), r.PathValue("id"))
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}
	if !actor.IsSeasonAdmin(season.ID) {
		writeErrorResponse(w, http.StatusForbidden, "season_admin_required", "Season administrator access is required.")
		return
	}
	if season.Status != "open" {
		writeErrorResponse(w, http.StatusForbidden, "season_admin_required", "The season must be open.")
		return
	}
	if !actor.HasRecentMFA(time.Now()) {
		writeErrorResponse(w, http.StatusForbidden, "season_admin_required", "Recent MFA is required.")
		return
	}
	var request createWeekRequest
	if err := decodeJSON(r.Body, &request); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "validation_failed", err.Error())
		return
	}
	if request.Number < 1 || !request.EndAt.After(request.StartAt) || (request.ResourceURL != "" && !validWebURL(request.ResourceURL, true)) {
		writeErrorResponse(w, http.StatusBadRequest, "validation_failed", "number, valid dates, and an HTTPS resource URL are required")
		return
	}
	if request.StartAt.Before(season.StartAt) || request.EndAt.After(season.EndAt) {
		writeErrorResponse(w, http.StatusBadRequest, "validation_failed", "week dates must fall within the season dates")
		return
	}
	week := programme.WeekRecord{
		ID:          id.New(),
		SeasonID:    r.PathValue("id"),
		Number:      request.Number,
		StartAt:     request.StartAt.UTC(),
		EndAt:       request.EndAt.UTC(),
		ResourceURL: request.ResourceURL,
	}
	created, err := a.db.CreateWeek(r.Context(), week, actor.UserID, time.Now().UTC())
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}

	writeJSONResponse(w, http.StatusCreated, created)
}

type updateWeekRequest struct {
	Number      int       `json:"number"`
	StartAt     time.Time `json:"startAt"`
	EndAt       time.Time `json:"endAt"`
	ResourceURL string    `json:"resourceUrl"`
}

func (a *API) updateWeek(w http.ResponseWriter, r *http.Request) {
	actor := actorFrom(r.Context())
	season, err := a.db.GetSeason(r.Context(), r.PathValue("id"))
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}
	if !actor.IsSeasonAdmin(season.ID) {
		writeErrorResponse(w, http.StatusForbidden, "season_admin_required", "Season administrator access is required.")
		return
	}
	if season.Status != "open" {
		writeErrorResponse(w, http.StatusForbidden, "season_admin_required", "The season must be open.")
		return
	}
	if !actor.HasRecentMFA(time.Now()) {
		writeErrorResponse(w, http.StatusForbidden, "season_admin_required", "Recent MFA is required.")
		return
	}
	var request updateWeekRequest
	if err := decodeJSON(r.Body, &request); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "validation_failed", err.Error())
		return
	}
	if request.Number < 1 || !request.EndAt.After(request.StartAt) || (request.ResourceURL != "" && !validWebURL(request.ResourceURL, true)) {
		writeErrorResponse(w, http.StatusBadRequest, "validation_failed", "number, valid dates, and an HTTPS resource URL are required")
		return
	}
	if request.StartAt.Before(season.StartAt) || request.EndAt.After(season.EndAt) {
		writeErrorResponse(w, http.StatusBadRequest, "validation_failed", "week dates must fall within the season dates")
		return
	}
	week, err := a.db.UpdateWeek(r.Context(), dal.UpdateWeekInput{
		SeasonID:  r.PathValue("id"),
		WeekID:    r.PathValue("weekId"),
		Week:      programme.WeekRecord{Number: request.Number, StartAt: request.StartAt.UTC(), EndAt: request.EndAt.UTC(), ResourceURL: request.ResourceURL},
		ActorID:   actor.UserID,
		ChangedAt: time.Now().UTC(),
	})
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}

	writeJSONResponse(w, http.StatusOK, week)
}

func (a *API) deleteWeek(w http.ResponseWriter, r *http.Request) {
	actor := actorFrom(r.Context())
	managedSeason, err := a.db.GetSeason(r.Context(), r.PathValue("id"))
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}
	if !actor.IsSeasonAdmin(managedSeason.ID) {
		writeErrorResponse(w, http.StatusForbidden, "season_admin_required", "Season administrator access is required.")
		return
	}
	if managedSeason.Status != "open" {
		writeErrorResponse(w, http.StatusForbidden, "season_admin_required", "The season must be open.")
		return
	}
	if !actor.HasRecentMFA(time.Now()) {
		writeErrorResponse(w, http.StatusForbidden, "season_admin_required", "Recent MFA is required.")
		return
	}
	if err := a.db.DeleteWeek(r.Context(), dal.DeleteWeekInput{
		SeasonID:  r.PathValue("id"),
		WeekID:    r.PathValue("weekId"),
		ActorID:   actor.UserID,
		ChangedAt: time.Now().UTC(),
	}); err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (a *API) listSeasonMembers(w http.ResponseWriter, r *http.Request) {
	actor := actorFrom(r.Context())
	seasonID := r.PathValue("id")
	if !actor.CanViewSeason(seasonID) {
		writeErrorResponse(w, http.StatusForbidden, "season_access_required", "No season access yet.")
		return
	}
	seasonEnrollment, _ := actor.Enrollment(seasonID)
	canSeeInactive := actor.IsDirectorOrSystemAdmin() || (seasonEnrollment.Role == authz.Coordinator && seasonEnrollment.State == authz.Active)
	role, state := r.URL.Query().Get("role"), r.URL.Query().Get("state")
	if role != "" && role != "student" && role != "mentor" && role != "coordinator" {
		writeErrorResponse(w, http.StatusBadRequest, "validation_failed", "role must be student, mentor, or coordinator")
		return
	}
	if state != "" && state != "active" && state != "completed" && state != "kicked" && state != "withdrawn" {
		writeErrorResponse(w, http.StatusBadRequest, "validation_failed", "state must be active, completed, kicked, or withdrawn")
		return
	}
	sortBy, err := parseSort(r.URL.Query().Get("sort"), "id:asc", "id:asc", "role:asc")
	if err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "validation_failed", "sort must be id:asc or role:asc")
		return
	}

	direction, err := parsePageDirection(r.URL.Query().Get("direction"))
	if err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "validation_failed", "direction must be forward or backward")
		return
	}

	binding := "members|season=" + seasonID + "|role=" + role + "|state=" + state + "|sort=" + sortBy
	limit, boundary, err := parsePagination(r.URL.Query().Get("limit"), r.URL.Query().Get("cursor"), binding, a.cursorSecret)
	if err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "invalid_cursor", "The cursor does not match the selected member filters.")
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

	pageInfo := pageInfoForKeyset(
		a.cursorSecret, binding, direction, boundary, items, more,
		func(member programme.EnrollmentRecord) string { return member.ID },
	)
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
		writeErrorResponse(w, http.StatusForbidden, "season_admin_required", "Season administrator access is required.")
		return
	}
	if managedSeason.Status != "open" {
		writeErrorResponse(w, http.StatusForbidden, "season_admin_required", "The season must be open.")
		return
	}
	if !actor.HasRecentMFA(time.Now()) {
		writeErrorResponse(w, http.StatusForbidden, "season_admin_required", "Recent MFA is required.")
		return
	}
	query := strings.TrimSpace(r.URL.Query().Get("query"))
	if _, err := parseSort(r.URL.Query().Get("sort"), "id:asc", "id:asc"); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "validation_failed", "sort must be id:asc")
		return
	}

	direction, err := parsePageDirection(r.URL.Query().Get("direction"))
	if err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "validation_failed", "direction must be forward or backward")
		return
	}

	binding := "enrollment-candidates|id:asc|season=" + r.PathValue("id") + "|query=" + query
	limit, boundary, err := parsePagination(r.URL.Query().Get("limit"), r.URL.Query().Get("cursor"), binding, a.cursorSecret)
	if err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "validation_failed", "cursor is invalid for the selected season and query")
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

	pageInfo := pageInfoForKeyset(
		a.cursorSecret, binding, direction, boundary, items, more,
		func(record accounts.EnrollmentCandidate) string { return record.ID },
	)
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
		writeErrorResponse(w, http.StatusForbidden, "season_admin_required", "Season administrator access is required.")
		return
	}
	if managedSeason.Status != "open" {
		writeErrorResponse(w, http.StatusForbidden, "season_admin_required", "The season must be open.")
		return
	}
	if !actor.HasRecentMFA(time.Now()) {
		writeErrorResponse(w, http.StatusForbidden, "season_admin_required", "Recent MFA is required.")
		return
	}
	var request createSeasonMemberRequest
	if err := decodeJSON(r.Body, &request); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "validation_failed", err.Error())
		return
	}
	if (request.Role != "student" && request.Role != "mentor" && request.Role != "coordinator") || strings.TrimSpace(request.UserID) == "" {
		writeErrorResponse(w, http.StatusBadRequest, "validation_failed", "userId and a valid role are required")
		return
	}
	if request.Role == "coordinator" && !canGrantCoordinator(actor, r.PathValue("id")) {
		writeErrorResponse(w, http.StatusForbidden, "privileged_role_grant_required", "Only the season Coordinator, a Director, or a System Admin may grant Coordinator access.")
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
		writeErrorResponse(w, http.StatusForbidden, "season_admin_required", "Season administrator access is required.")
		return
	}
	if managedSeason.Status != "open" {
		writeErrorResponse(w, http.StatusForbidden, "season_admin_required", "The season must be open.")
		return
	}
	if !actor.HasRecentMFA(time.Now()) {
		writeErrorResponse(w, http.StatusForbidden, "season_admin_required", "Recent MFA is required.")
		return
	}
	var request updateSeasonMemberRequest

	if err := decodeJSON(r.Body, &request); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "validation_failed", err.Error())
		return
	}
	switch request.Role {
	case "student":
		if !isValidStudentLevel(request.StudentLevel) {
			writeErrorResponse(w, http.StatusBadRequest, "validation_failed", "studentLevel must be novice, beginner, intermediate, or advanced for students")
			return
		}
	case "mentor":
		request.StudentLevel = "not_applicable"
	default:
		writeErrorResponse(w, http.StatusBadRequest, "validation_failed", "role must be student or mentor")
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
		writeErrorResponse(w, http.StatusConflict, "season_closed", "Season members cannot be changed until the season is reopened.")
		return
	}
	member, err := a.db.GetEnrollment(r.Context(), memberID)
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}
	if member.SeasonID != seasonID {
		writeErrorResponse(w, http.StatusNotFound, "not_found", "The enrollment does not exist.")
		return
	}
	if member.Role != "student" {
		writeErrorResponse(w, http.StatusForbidden, "student_action_required", "Only an active student enrollment can be promoted or removed through this operation.")
		return
	}
	assigned, err := a.db.IsMentorAssigned(r.Context(), seasonID, actor.UserID, member.UserID)
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}
	if !actor.CanPromoteOrRemoveStudent(seasonID, assigned, member.State == "active") {
		writeErrorResponse(w, http.StatusForbidden, "member_admin_required", "This member cannot be changed by the current account.")
		return
	}
	seasonRole, _ := actor.Enrollment(seasonID)
	if (actor.IsDirectorOrSystemAdmin() || seasonRole.Role == authz.Coordinator) && !actor.HasRecentMFA(time.Now()) {
		writeErrorResponse(w, http.StatusForbidden, "privileged_mfa_required", "Recent MFA is required for this administrative action.")
		return
	}
	var request promoteSeasonMemberRequest
	if err := decodeJSON(r.Body, &request); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "validation_failed", err.Error())
		return
	}

	if request.Role != "mentor" && request.Role != "coordinator" {
		writeErrorResponse(w, http.StatusBadRequest, "validation_failed", "role must be mentor or coordinator")
		return
	}
	if request.Role == "coordinator" && !canGrantCoordinator(actor, seasonID) {
		writeErrorResponse(w, http.StatusForbidden, "privileged_role_grant_required", "Only the season Coordinator, a Director, or a System Admin may grant Coordinator access.")
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
		writeErrorResponse(w, http.StatusConflict, "season_closed", "Season members cannot be changed until the season is reopened.")
		return
	}
	member, err := a.db.GetEnrollment(r.Context(), memberID)
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}
	if member.SeasonID != seasonID {
		writeErrorResponse(w, http.StatusNotFound, "not_found", "The enrollment does not exist.")
		return
	}
	if member.Role != "student" {
		writeErrorResponse(w, http.StatusForbidden, "student_action_required", "Only an active student enrollment can be promoted or removed through this operation.")
		return
	}
	assigned, err := a.db.IsMentorAssigned(r.Context(), seasonID, actor.UserID, member.UserID)
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}
	if !actor.CanPromoteOrRemoveStudent(seasonID, assigned, member.State == "active") {
		writeErrorResponse(w, http.StatusForbidden, "member_admin_required", "This member cannot be changed by the current account.")
		return
	}
	seasonRole, _ := actor.Enrollment(seasonID)
	if (actor.IsDirectorOrSystemAdmin() || seasonRole.Role == authz.Coordinator) && !actor.HasRecentMFA(time.Now()) {
		writeErrorResponse(w, http.StatusForbidden, "privileged_mfa_required", "Recent MFA is required for this administrative action.")
		return
	}
	var request removeSeasonMemberRequest
	if err := decodeJSON(r.Body, &request); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "validation_failed", err.Error())
		return
	}

	if strings.TrimSpace(request.Reason) == "" {
		writeErrorResponse(w, http.StatusBadRequest, "validation_failed", "removal reason is required")
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

func (a *API) listMentorships(w http.ResponseWriter, r *http.Request) {
	actor := actorFrom(r.Context())
	seasonID := r.PathValue("id")
	if !actor.CanViewSeason(seasonID) {
		writeErrorResponse(w, http.StatusForbidden, "season_access_required", "The current account cannot access this season.")
		return
	}
	sortBy, err := parseSort(r.URL.Query().Get("sort"), "id:asc", "id:asc", "student:asc")
	if err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "validation_failed", "sort must be id:asc or student:asc")
		return
	}

	direction, err := parsePageDirection(r.URL.Query().Get("direction"))
	if err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "validation_failed", "direction must be forward or backward")
		return
	}

	mentorUserID, studentUserID := strings.TrimSpace(r.URL.Query().Get("mentorUserId")), strings.TrimSpace(r.URL.Query().Get("studentUserId"))
	binding := "mentorships|season=" + seasonID + "|sort=" + sortBy + "|mentor=" + mentorUserID + "|student=" + studentUserID
	limit, boundary, err := parsePagination(r.URL.Query().Get("limit"), r.URL.Query().Get("cursor"), binding, a.cursorSecret)
	if err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "invalid_cursor", "The cursor does not match the selected season and sort.")
		return
	}

	items, more, total, err := a.db.ListMentorships(r.Context(), dal.MentorshipQuery{
		SeasonID:      seasonID,
		Boundary:      boundary,
		Limit:         limit,
		SortBy:        sortBy,
		Direction:     direction,
		MentorUserID:  mentorUserID,
		StudentUserID: studentUserID,
	})
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}

	pageInfo := pageInfoForKeyset(
		a.cursorSecret, binding, direction, boundary, items, more,
		func(mentorship programme.MentorshipRecord) string { return mentorship.ID },
	)
	response := Page[programme.MentorshipRecord]{
		Items:      items,
		PageInfo:   pageInfo,
		TotalCount: total,
	}
	writeJSONResponse(w, http.StatusOK, response)
}

type createMentorshipRequest struct {
	MentorUserID  string `json:"mentorUserId"`
	StudentUserID string `json:"studentUserId"`
}

func (a *API) createMentorship(w http.ResponseWriter, r *http.Request) {
	actor := actorFrom(r.Context())
	managedSeason, err := a.db.GetSeason(r.Context(), r.PathValue("id"))
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}
	if !actor.IsSeasonAdmin(managedSeason.ID) {
		writeErrorResponse(w, http.StatusForbidden, "season_admin_required", "Season administrator access is required.")
		return
	}
	if managedSeason.Status != "open" {
		writeErrorResponse(w, http.StatusForbidden, "season_admin_required", "The season must be open.")
		return
	}
	if !actor.HasRecentMFA(time.Now()) {
		writeErrorResponse(w, http.StatusForbidden, "season_admin_required", "Recent MFA is required.")
		return
	}
	var request createMentorshipRequest
	if err := decodeJSON(r.Body, &request); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "validation_failed", err.Error())
		return
	}
	if request.MentorUserID == "" || request.StudentUserID == "" || request.MentorUserID == request.StudentUserID {
		writeErrorResponse(w, http.StatusBadRequest, "validation_failed", "different mentorUserId and studentUserId are required")
		return
	}

	mentorship := programme.MentorshipRecord{
		ID:            id.New(),
		SeasonID:      r.PathValue("id"),
		MentorUserID:  request.MentorUserID,
		StudentUserID: request.StudentUserID,
	}
	created, err := a.db.CreateMentorship(r.Context(), mentorship, actor.UserID, time.Now().UTC())
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}

	writeJSONResponse(w, http.StatusCreated, created)
}

type updateMentorshipRequest struct {
	MentorUserID  string `json:"mentorUserId"`
	StudentUserID string `json:"studentUserId"`
}

func (a *API) updateMentorship(w http.ResponseWriter, r *http.Request) {
	actor := actorFrom(r.Context())
	managedSeason, err := a.db.GetSeason(r.Context(), r.PathValue("id"))
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}
	if !actor.IsSeasonAdmin(managedSeason.ID) {
		writeErrorResponse(w, http.StatusForbidden, "season_admin_required", "Season administrator access is required.")
		return
	}
	if managedSeason.Status != "open" {
		writeErrorResponse(w, http.StatusForbidden, "season_admin_required", "The season must be open.")
		return
	}
	if !actor.HasRecentMFA(time.Now()) {
		writeErrorResponse(w, http.StatusForbidden, "season_admin_required", "Recent MFA is required.")
		return
	}
	var request updateMentorshipRequest
	if err := decodeJSON(r.Body, &request); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "validation_failed", err.Error())
		return
	}
	if request.MentorUserID == "" || request.StudentUserID == "" || request.MentorUserID == request.StudentUserID {
		writeErrorResponse(w, http.StatusBadRequest, "validation_failed", "different mentorUserId and studentUserId are required")
		return
	}
	mentorship, err := a.db.UpdateMentorship(r.Context(), dal.UpdateMentorshipInput{
		SeasonID:      r.PathValue("id"),
		MentorshipID:  r.PathValue("mentorshipId"),
		MentorUserID:  request.MentorUserID,
		StudentUserID: request.StudentUserID,
		ActorID:       actor.UserID,
		ChangedAt:     time.Now().UTC(),
	})
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}

	writeJSONResponse(w, http.StatusOK, mentorship)
}

func (a *API) deleteMentorship(w http.ResponseWriter, r *http.Request) {
	actor := actorFrom(r.Context())
	managedSeason, err := a.db.GetSeason(r.Context(), r.PathValue("id"))
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}
	if !actor.IsSeasonAdmin(managedSeason.ID) {
		writeErrorResponse(w, http.StatusForbidden, "season_admin_required", "Season administrator access is required.")
		return
	}
	if managedSeason.Status != "open" {
		writeErrorResponse(w, http.StatusForbidden, "season_admin_required", "The season must be open.")
		return
	}
	if !actor.HasRecentMFA(time.Now()) {
		writeErrorResponse(w, http.StatusForbidden, "season_admin_required", "Recent MFA is required.")
		return
	}
	if err := a.db.DeleteMentorship(r.Context(), dal.DeleteMentorshipInput{
		SeasonID:     r.PathValue("id"),
		MentorshipID: r.PathValue("mentorshipId"),
		ActorID:      actor.UserID,
		ChangedAt:    time.Now().UTC(),
	}); err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func isValidStudentLevel(level string) bool {
	switch level {
	case "novice", "beginner", "intermediate", "advanced":
		return true
	default:
		return false
	}
}
