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

func (a *API) requireSeasonAdmin(w http.ResponseWriter, r *http.Request) (programme.SeasonRecord, bool) {
	seasonID := r.PathValue("id")
	season, err := a.db.GetSeason(r.Context(), seasonID)
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return programme.SeasonRecord{}, false
	}

	actor := actorFrom(r.Context())
	if !actor.CanManageSeason(seasonID, season.Status == "open") || !actor.HasRecentMFA(time.Now()) {
		writeErrorResponse(w, http.StatusForbidden, "season_admin_required", "An administrator of an open season with recent MFA is required.")
		return programme.SeasonRecord{}, false
	}
	return season, true
}

func canGrantCoordinator(actor authz.Actor, seasonID string) bool {
	if actor.IsPrivileged() {
		return true
	}
	enrollment, ok := actor.Enrollment(seasonID)
	return ok && enrollment.Role == authz.Coordinator && enrollment.State == authz.Active
}

func (a *API) listWeeks(w http.ResponseWriter, r *http.Request) {
	seasonID := r.PathValue("id")
	if !actorFrom(r.Context()).CanViewSeason(seasonID) {
		writeErrorResponse(w, http.StatusForbidden, "season_access_required", "The current account cannot access this season.")
		return
	}
	if _, err := a.db.GetSeason(r.Context(), seasonID); err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}

	sortBy, err := requestedSort(r, "number:asc", "number:asc", "id:asc")
	if err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "validation_failed", "sort must be number:asc or id:asc")
		return
	}

	direction, err := requestedDirection(r)
	if err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "validation_failed", "direction must be forward or backward")
		return
	}

	binding := "weeks|season=" + seasonID + "|sort=" + sortBy
	limit, boundary, err := a.page(r, binding)
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
		a, binding, direction, boundary, items, more,
		func(v programme.WeekRecord) string { return v.ID },
	)
	response := Page[programme.WeekRecord]{
		Items:      items,
		PageInfo:   pageInfo,
		TotalCount: total,
	}
	writeJSONResponse(w, http.StatusOK, response)
}

func (a *API) createWeek(w http.ResponseWriter, r *http.Request) {
	season, ok := a.requireSeasonAdmin(w, r)
	if !ok {
		return
	}
	var in struct {
		Number      int       `json:"number"`
		StartAt     time.Time `json:"startAt"`
		EndAt       time.Time `json:"endAt"`
		ResourceURL string    `json:"resourceUrl"`
	}
	if err := decodeJSON(w, r, &in); err != nil || in.Number < 1 || !in.EndAt.After(in.StartAt) || (in.ResourceURL != "" && !validWebURL(in.ResourceURL, true)) {
		writeErrorResponse(w, http.StatusBadRequest, "validation_failed", "number, valid dates, and an HTTPS resource URL are required")
		return
	}
	if in.StartAt.Before(season.StartAt) || in.EndAt.After(season.EndAt) {
		writeErrorResponse(w, http.StatusBadRequest, "validation_failed", "week dates must fall within the season dates")
		return
	}
	v := programme.WeekRecord{ID: id.New(), SeasonID: r.PathValue("id"), Number: in.Number, StartAt: in.StartAt.UTC(), EndAt: in.EndAt.UTC(), ResourceURL: in.ResourceURL, Revision: 1}
	actor := actorFrom(r.Context())
	created, err := a.db.CreateWeek(r.Context(), v, actor.UserID, time.Now().UTC())
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}

	writeJSONResponse(w, http.StatusCreated, created)
}

func (a *API) updateWeek(w http.ResponseWriter, r *http.Request) {
	season, ok := a.requireSeasonAdmin(w, r)
	if !ok {
		return
	}
	var in struct {
		Number      int       `json:"number"`
		StartAt     time.Time `json:"startAt"`
		EndAt       time.Time `json:"endAt"`
		ResourceURL string    `json:"resourceUrl"`
		Revision    int64     `json:"revision"`
	}
	if err := decodeJSON(w, r, &in); err != nil || in.Revision < 1 || in.Number < 1 || !in.EndAt.After(in.StartAt) || (in.ResourceURL != "" && !validWebURL(in.ResourceURL, true)) {
		writeErrorResponse(w, http.StatusBadRequest, "validation_failed", "number, valid dates, HTTPS resource URL, and revision are required")
		return
	}
	if in.StartAt.Before(season.StartAt) || in.EndAt.After(season.EndAt) {
		writeErrorResponse(w, http.StatusBadRequest, "validation_failed", "week dates must fall within the season dates")
		return
	}
	actor := actorFrom(r.Context())
	v, err := a.db.UpdateWeek(r.Context(), r.PathValue("id"), r.PathValue("weekId"), in.Revision, programme.WeekRecord{Number: in.Number, StartAt: in.StartAt.UTC(), EndAt: in.EndAt.UTC(), ResourceURL: in.ResourceURL}, actor.UserID, time.Now().UTC())
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}

	writeJSONResponse(w, http.StatusOK, v)
}

func (a *API) deleteWeek(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.requireSeasonAdmin(w, r); !ok {
		return
	}
	revision, err := parseRevision(r)
	if err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "validation_failed", "revision query parameter is required")
		return
	}

	actor := actorFrom(r.Context())
	if err := a.db.DeleteWeek(r.Context(), r.PathValue("id"), r.PathValue("weekId"), revision, actor.UserID, time.Now().UTC()); err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (a *API) listMembers(w http.ResponseWriter, r *http.Request) {
	seasonID := r.PathValue("id")
	if !actorFrom(r.Context()).CanViewSeason(seasonID) {
		writeErrorResponse(w, http.StatusForbidden, "season_access_required", "No season access yet.")
		return
	}
	actor := actorFrom(r.Context())
	seasonEnrollment, _ := actor.Enrollment(seasonID)
	canSeeInactive := actor.IsPrivileged() || (seasonEnrollment.Role == authz.Coordinator && seasonEnrollment.State == authz.Active)
	role, state := r.URL.Query().Get("role"), r.URL.Query().Get("state")
	if role != "" && role != "student" && role != "mentor" && role != "coordinator" {
		writeErrorResponse(w, http.StatusBadRequest, "validation_failed", "role must be student, mentor, or coordinator")
		return
	}
	if state != "" && state != "active" && state != "completed" && state != "kicked" && state != "withdrawn" {
		writeErrorResponse(w, http.StatusBadRequest, "validation_failed", "state must be active, completed, kicked, or withdrawn")
		return
	}
	sortBy, err := requestedSort(r, "id:asc", "id:asc", "role:asc")
	if err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "validation_failed", "sort must be id:asc or role:asc")
		return
	}

	direction, err := requestedDirection(r)
	if err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "validation_failed", "direction must be forward or backward")
		return
	}

	binding := "members|season=" + seasonID + "|role=" + role + "|state=" + state + "|sort=" + sortBy
	limit, boundary, err := a.page(r, binding)
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
		a, binding, direction, boundary, items, more,
		func(v programme.EnrollmentRecord) string { return v.ID },
	)
	response := Page[programme.EnrollmentRecord]{
		Items:      items,
		PageInfo:   pageInfo,
		TotalCount: total,
	}
	writeJSONResponse(w, http.StatusOK, response)
}

func (a *API) listEnrollmentCandidates(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.requireSeasonAdmin(w, r); !ok {
		return
	}
	query := strings.TrimSpace(r.URL.Query().Get("query"))
	if _, err := requestedSort(r, "id:asc", "id:asc"); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "validation_failed", "sort must be id:asc")
		return
	}

	direction, err := requestedDirection(r)
	if err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "validation_failed", "direction must be forward or backward")
		return
	}

	binding := "enrollment-candidates|id:asc|season=" + r.PathValue("id") + "|query=" + query
	limit, boundary, err := a.page(r, binding)
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
		a, binding, direction, boundary, items, more,
		func(v accounts.EnrollmentCandidate) string { return v.ID },
	)
	response := Page[accounts.EnrollmentCandidate]{
		Items:      items,
		PageInfo:   pageInfo,
		TotalCount: total,
	}
	writeJSONResponse(w, http.StatusOK, response)
}

func (a *API) createMember(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.requireSeasonAdmin(w, r); !ok {
		return
	}
	var in struct {
		UserID string `json:"userId"`
		Role   string `json:"role"`
	}
	if err := decodeJSON(w, r, &in); err != nil || (in.Role != "student" && in.Role != "mentor" && in.Role != "coordinator") || strings.TrimSpace(in.UserID) == "" {
		writeErrorResponse(w, http.StatusBadRequest, "validation_failed", "userId and a valid role are required")
		return
	}
	if in.Role == "coordinator" && !canGrantCoordinator(actorFrom(r.Context()), r.PathValue("id")) {
		writeErrorResponse(w, http.StatusForbidden, "privileged_role_grant_required", "Only the season Coordinator, a Director, or a System Admin may grant Coordinator access.")
		return
	}
	v := programme.EnrollmentRecord{ID: id.New(), SeasonID: r.PathValue("id"), UserID: in.UserID, Role: in.Role, State: "active", Revision: 1}
	actor := actorFrom(r.Context())
	created, err := a.db.CreateEnrollment(r.Context(), v, actor.UserID, time.Now().UTC())
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}

	writeJSONResponse(w, http.StatusCreated, created)
}

func (a *API) updateMember(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.requireSeasonAdmin(w, r); !ok {
		return
	}
	var in struct {
		Role         string `json:"role"`
		StudentLevel string `json:"studentLevel"`
		Revision     int64  `json:"revision"`
	}
	if err := decodeJSON(w, r, &in); err != nil || in.Revision < 1 || (in.Role != "student" && in.Role != "mentor") {
		writeErrorResponse(w, http.StatusBadRequest, "validation_failed", "role and revision are required")
		return
	}
	if in.Role == "student" {
		if in.StudentLevel != "novice" && in.StudentLevel != "beginner" && in.StudentLevel != "intermediate" && in.StudentLevel != "advanced" {
			writeErrorResponse(w, http.StatusBadRequest, "validation_failed", "studentLevel is required for students")
			return
		}
	} else {
		in.StudentLevel = "not_applicable"
	}
	actor := actorFrom(r.Context())
	v, err := a.db.UpdateEnrollmentDetails(r.Context(), r.PathValue("id"), r.PathValue("memberId"), in.Revision, in.Role, in.StudentLevel, actor.UserID, time.Now().UTC())
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}

	writeJSONResponse(w, http.StatusOK, v)
}

func (a *API) promoteMember(w http.ResponseWriter, r *http.Request) { a.changeMember(w, r, true) }

func (a *API) removeMember(w http.ResponseWriter, r *http.Request) { a.changeMember(w, r, false) }

func (a *API) changeMember(w http.ResponseWriter, r *http.Request, promote bool) {
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
	v, err := a.db.GetEnrollment(r.Context(), memberID)
	if err != nil || v.SeasonID != seasonID {
		writeErrorResponse(w, http.StatusNotFound, "not_found", "The enrollment does not exist.")
		return
	}
	if v.Role != "student" {
		writeErrorResponse(w, http.StatusForbidden, "student_action_required", "Only an active student enrollment can be promoted or removed through this operation.")
		return
	}
	assigned, err := a.db.IsMentorAssigned(r.Context(), seasonID, actor.UserID, v.UserID)
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}
	if !actor.CanPromoteOrRemove(seasonID, assigned, v.State == "active") {
		writeErrorResponse(w, http.StatusForbidden, "member_admin_required", "This member cannot be changed by the current account.")
		return
	}
	seasonRole, _ := actor.Enrollment(seasonID)
	if (actor.IsPrivileged() || seasonRole.Role == authz.Coordinator) && !actor.HasRecentMFA(time.Now()) {
		writeErrorResponse(w, http.StatusForbidden, "privileged_mfa_required", "Recent MFA is required for this administrative action.")
		return
	}
	var in struct {
		Role     string `json:"role"`
		Reason   string `json:"reason"`
		Revision int64  `json:"revision"`
	}
	if err := decodeJSON(w, r, &in); err != nil || in.Revision < 1 || (!promote && strings.TrimSpace(in.Reason) == "") {
		writeErrorResponse(w, http.StatusBadRequest, "validation_failed", "revision and removal reason are required")
		return
	}
	if promote && in.Role != "mentor" && in.Role != "coordinator" {
		writeErrorResponse(w, http.StatusBadRequest, "validation_failed", "role must be mentor or coordinator")
		return
	}
	if promote && in.Role == "coordinator" && !canGrantCoordinator(actor, seasonID) {
		writeErrorResponse(w, http.StatusForbidden, "privileged_role_grant_required", "Only the season Coordinator, a Director, or a System Admin may grant Coordinator access.")
		return
	}
	role, state := "", ""
	if promote {
		role = in.Role
	} else {
		state = "kicked"
	}
	updated, err := a.db.UpdateEnrollment(r.Context(), memberID, in.Revision, role, state, in.Reason, actor.UserID, time.Now())
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}

	writeJSONResponse(w, http.StatusOK, updated)
}

func (a *API) listMentorships(w http.ResponseWriter, r *http.Request) {
	seasonID := r.PathValue("id")
	if !actorFrom(r.Context()).CanViewSeason(seasonID) {
		writeErrorResponse(w, http.StatusForbidden, "season_access_required", "The current account cannot access this season.")
		return
	}
	sortBy, err := requestedSort(r, "id:asc", "id:asc", "student:asc")
	if err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "validation_failed", "sort must be id:asc or student:asc")
		return
	}

	direction, err := requestedDirection(r)
	if err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "validation_failed", "direction must be forward or backward")
		return
	}

	mentorUserID, studentUserID := strings.TrimSpace(r.URL.Query().Get("mentorUserId")), strings.TrimSpace(r.URL.Query().Get("studentUserId"))
	binding := "mentorships|season=" + seasonID + "|sort=" + sortBy + "|mentor=" + mentorUserID + "|student=" + studentUserID
	limit, boundary, err := a.page(r, binding)
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
		a, binding, direction, boundary, items, more,
		func(v programme.MentorshipRecord) string { return v.ID },
	)
	response := Page[programme.MentorshipRecord]{
		Items:      items,
		PageInfo:   pageInfo,
		TotalCount: total,
	}
	writeJSONResponse(w, http.StatusOK, response)
}

func (a *API) createMentorship(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.requireSeasonAdmin(w, r); !ok {
		return
	}
	var in struct {
		MentorUserID  string `json:"mentorUserId"`
		StudentUserID string `json:"studentUserId"`
	}
	if err := decodeJSON(w, r, &in); err != nil || in.MentorUserID == "" || in.StudentUserID == "" || in.MentorUserID == in.StudentUserID {
		writeErrorResponse(w, http.StatusBadRequest, "validation_failed", "different mentorUserId and studentUserId are required")
		return
	}

	v := programme.MentorshipRecord{ID: id.New(), SeasonID: r.PathValue("id"), MentorUserID: in.MentorUserID, StudentUserID: in.StudentUserID, Revision: 1}
	actor := actorFrom(r.Context())
	created, err := a.db.CreateMentorship(r.Context(), v, actor.UserID, time.Now().UTC())
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}

	writeJSONResponse(w, http.StatusCreated, created)
}

func (a *API) updateMentorship(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.requireSeasonAdmin(w, r); !ok {
		return
	}
	var in struct {
		MentorUserID  string `json:"mentorUserId"`
		StudentUserID string `json:"studentUserId"`
		Revision      int64  `json:"revision"`
	}
	if err := decodeJSON(w, r, &in); err != nil || in.Revision < 1 || in.MentorUserID == "" || in.StudentUserID == "" || in.MentorUserID == in.StudentUserID {
		writeErrorResponse(w, http.StatusBadRequest, "validation_failed", "different mentorUserId and studentUserId plus revision are required")
		return
	}

	actor := actorFrom(r.Context())
	v, err := a.db.UpdateMentorship(r.Context(), r.PathValue("id"), r.PathValue("mentorshipId"), in.Revision, in.MentorUserID, in.StudentUserID, actor.UserID, time.Now().UTC())
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}

	writeJSONResponse(w, http.StatusOK, v)
}

func (a *API) deleteMentorship(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.requireSeasonAdmin(w, r); !ok {
		return
	}
	revision, err := parseRevision(r)
	if err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "validation_failed", "revision query parameter is required")
		return
	}

	actor := actorFrom(r.Context())
	if err := a.db.DeleteMentorship(r.Context(), r.PathValue("id"), r.PathValue("mentorshipId"), revision, actor.UserID, time.Now().UTC()); err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
