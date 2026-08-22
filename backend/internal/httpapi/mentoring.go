package httpapi

import (
	"net/http"
	"strings"
	"time"

	"github.com/magedmg/RSP-website/backend/internal/authz"
	"github.com/magedmg/RSP-website/backend/internal/model"
	"github.com/magedmg/RSP-website/backend/internal/platform/id"
)

func (a *API) canViewSeason(r *http.Request, seasonID string) bool {
	actor := actorFrom(r.Context())
	if actor.IsPrivileged() {
		return true
	}
	enrollment, ok := actor.Enrollment(seasonID)
	return ok && (enrollment.State == authz.Active || enrollment.State == authz.Completed && actor.EligibleMember())
}

func (a *API) seasonAdmin(r *http.Request) (model.Season, bool) {
	seasonID := r.PathValue("id")
	season, err := a.store.GetSeason(r.Context(), seasonID)
	if err != nil {
		return model.Season{}, false
	}

	actor := actorFrom(r.Context())
	return season, actor.CanManageSeason(seasonID, season.Status == "open") && actor.HasRecentMFA(time.Now())
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
	if !a.canViewSeason(r, seasonID) {
		a.fail(w, r, 403, "season_access_required", "Season access required", "The current account cannot access this season.", nil)
		return
	}
	if _, err := a.store.GetSeason(r.Context(), seasonID); err != nil {
		storeFailure(a, w, r, err)
		return
	}

	sortBy, err := requestedSort(r, "number:asc", "number:asc", "id:asc")
	if err != nil {
		validation(a, w, r, "sort must be number:asc or id:asc")
		return
	}

	direction, err := requestedDirection(r)
	if err != nil {
		validation(a, w, r, "direction must be forward or backward")
		return
	}

	binding := "weeks|season=" + seasonID + "|sort=" + sortBy
	limit, boundary, err := a.page(r, binding)
	if err != nil {
		a.fail(w, r, 400, "invalid_cursor", "Invalid cursor", "The cursor does not match the selected season and sort.", nil)
		return
	}

	items, more, total, err := a.store.ListWeeks(r.Context(), seasonID, boundary, limit, sortBy, direction)
	if err != nil {
		storeFailure(a, w, r, err)
		return
	}

	writeJSON(w, 200, model.Page[model.Week]{Items: items, PageInfo: pageInfoForKeyset(a, binding, direction, boundary, items, more, func(v model.Week) string { return v.ID }), TotalCount: total})
}

func (a *API) createWeek(w http.ResponseWriter, r *http.Request) {
	season, ok := a.seasonAdmin(r)
	if !ok {
		a.fail(w, r, 403, "season_admin_required", "Season administration required", "An administrator of an open season with recent MFA is required.", nil)
		return
	}
	var in struct {
		Number      int       `json:"number"`
		StartAt     time.Time `json:"startAt"`
		EndAt       time.Time `json:"endAt"`
		ResourceURL string    `json:"resourceUrl"`
	}
	if err := decodeJSON(w, r, &in); err != nil || in.Number < 1 || !in.EndAt.After(in.StartAt) || (in.ResourceURL != "" && !validWebURL(in.ResourceURL, true)) {
		validation(a, w, r, "number, valid dates, and an HTTPS resource URL are required")
		return
	}
	if in.StartAt.Before(season.StartAt) || in.EndAt.After(season.EndAt) {
		validation(a, w, r, "week dates must fall within the season dates")
		return
	}
	v := model.Week{ID: id.New(), SeasonID: r.PathValue("id"), Number: in.Number, StartAt: in.StartAt.UTC(), EndAt: in.EndAt.UTC(), ResourceURL: in.ResourceURL, Revision: 1}
	actor := actorFrom(r.Context())
	created, err := a.store.CreateWeek(r.Context(), v, actor.UserID, time.Now().UTC())
	if err != nil {
		storeFailure(a, w, r, err)
		return
	}

	writeJSON(w, 201, created)
}

func (a *API) updateWeek(w http.ResponseWriter, r *http.Request) {
	season, ok := a.seasonAdmin(r)
	if !ok {
		a.fail(w, r, 403, "season_admin_required", "Season administration required", "An administrator of an open season with recent MFA is required.", nil)
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
		validation(a, w, r, "number, valid dates, HTTPS resource URL, and revision are required")
		return
	}
	if in.StartAt.Before(season.StartAt) || in.EndAt.After(season.EndAt) {
		validation(a, w, r, "week dates must fall within the season dates")
		return
	}
	actor := actorFrom(r.Context())
	v, err := a.store.UpdateWeek(r.Context(), r.PathValue("id"), r.PathValue("weekId"), in.Revision, model.Week{Number: in.Number, StartAt: in.StartAt.UTC(), EndAt: in.EndAt.UTC(), ResourceURL: in.ResourceURL}, actor.UserID, time.Now().UTC())
	if err != nil {
		storeFailure(a, w, r, err)
		return
	}

	writeJSON(w, 200, v)
}

func (a *API) deleteWeek(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.seasonAdmin(r); !ok {
		a.fail(w, r, 403, "season_admin_required", "Season administration required", "An administrator of an open season with recent MFA is required.", nil)
		return
	}
	revision, err := parseRevision(r)
	if err != nil {
		validation(a, w, r, "revision query parameter is required")
		return
	}

	actor := actorFrom(r.Context())
	if err := a.store.DeleteWeek(r.Context(), r.PathValue("id"), r.PathValue("weekId"), revision, actor.UserID, time.Now().UTC()); err != nil {
		storeFailure(a, w, r, err)
		return
	}

	w.WriteHeader(204)
}

func (a *API) listMembers(w http.ResponseWriter, r *http.Request) {
	seasonID := r.PathValue("id")
	if !a.canViewSeason(r, seasonID) {
		a.fail(w, r, 403, "season_access_required", "Season access required", "No season access yet.", nil)
		return
	}
	actor := actorFrom(r.Context())
	seasonEnrollment, _ := actor.Enrollment(seasonID)
	canSeeInactive := actor.IsPrivileged() || (seasonEnrollment.Role == authz.Coordinator && seasonEnrollment.State == authz.Active)
	role, state := r.URL.Query().Get("role"), r.URL.Query().Get("state")
	if role != "" && role != "student" && role != "mentor" && role != "coordinator" {
		validation(a, w, r, "role must be student, mentor, or coordinator")
		return
	}
	if state != "" && state != "active" && state != "completed" && state != "kicked" && state != "withdrawn" {
		validation(a, w, r, "state must be active, completed, kicked, or withdrawn")
		return
	}
	sortBy, err := requestedSort(r, "id:asc", "id:asc", "role:asc")
	if err != nil {
		validation(a, w, r, "sort must be id:asc or role:asc")
		return
	}

	direction, err := requestedDirection(r)
	if err != nil {
		validation(a, w, r, "direction must be forward or backward")
		return
	}

	binding := "members|season=" + seasonID + "|role=" + role + "|state=" + state + "|sort=" + sortBy
	limit, boundary, err := a.page(r, binding)
	if err != nil {
		a.fail(w, r, 400, "invalid_cursor", "Invalid cursor", "The cursor does not match the selected member filters.", nil)
		return
	}

	items, more, total, err := a.store.ListEnrollments(r.Context(), seasonID, boundary, limit, role, state, sortBy, direction, canSeeInactive)
	if err != nil {
		storeFailure(a, w, r, err)
		return
	}

	writeJSON(w, 200, model.Page[model.Enrollment]{Items: items, PageInfo: pageInfoForKeyset(a, binding, direction, boundary, items, more, func(v model.Enrollment) string { return v.ID }), TotalCount: total})
}

func (a *API) listEnrollmentCandidates(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.seasonAdmin(r); !ok {
		a.fail(w, r, 403, "season_admin_required", "Season administration required", "An administrator of an open season with recent MFA is required.", nil)
		return
	}
	query := strings.TrimSpace(r.URL.Query().Get("query"))
	if _, err := requestedSort(r, "id:asc", "id:asc"); err != nil {
		validation(a, w, r, "sort must be id:asc")
		return
	}

	direction, err := requestedDirection(r)
	if err != nil {
		validation(a, w, r, "direction must be forward or backward")
		return
	}

	binding := "enrollment-candidates|id:asc|season=" + r.PathValue("id") + "|query=" + query
	limit, boundary, err := a.page(r, binding)
	if err != nil {
		validation(a, w, r, "cursor is invalid for the selected season and query")
		return
	}

	items, more, total, err := a.store.ListEnrollmentCandidates(r.Context(), r.PathValue("id"), query, boundary, limit, direction)
	if err != nil {
		storeFailure(a, w, r, err)
		return
	}

	writeJSON(w, 200, model.Page[model.EnrollmentCandidate]{Items: items, PageInfo: pageInfoForKeyset(a, binding, direction, boundary, items, more, func(v model.EnrollmentCandidate) string { return v.ID }), TotalCount: total})
}

func (a *API) createMember(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.seasonAdmin(r); !ok {
		a.fail(w, r, 403, "season_admin_required", "Season administration required", "An administrator of an open season with recent MFA is required.", nil)
		return
	}
	var in struct {
		UserID string `json:"userId"`
		Role   string `json:"role"`
	}
	if err := decodeJSON(w, r, &in); err != nil || (in.Role != "student" && in.Role != "mentor" && in.Role != "coordinator") || strings.TrimSpace(in.UserID) == "" {
		validation(a, w, r, "userId and a valid role are required")
		return
	}
	if in.Role == "coordinator" && !canGrantCoordinator(actorFrom(r.Context()), r.PathValue("id")) {
		a.fail(w, r, 403, "privileged_role_grant_required", "Privileged role grant required", "Only the season Coordinator, a Director, or a System Admin may grant Coordinator access.", nil)
		return
	}
	v := model.Enrollment{ID: id.New(), SeasonID: r.PathValue("id"), UserID: in.UserID, Role: in.Role, State: "active", Revision: 1}
	actor := actorFrom(r.Context())
	created, err := a.store.CreateEnrollment(r.Context(), v, actor.UserID, time.Now().UTC())
	if err != nil {
		storeFailure(a, w, r, err)
		return
	}

	writeJSON(w, 201, created)
}

func (a *API) updateMember(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.seasonAdmin(r); !ok {
		a.fail(w, r, 403, "season_admin_required", "Season administration required", "An administrator of an open season with recent MFA is required.", nil)
		return
	}
	var in struct {
		Role         string `json:"role"`
		StudentLevel string `json:"studentLevel"`
		Revision     int64  `json:"revision"`
	}
	if err := decodeJSON(w, r, &in); err != nil || in.Revision < 1 || (in.Role != "student" && in.Role != "mentor") {
		validation(a, w, r, "role and revision are required")
		return
	}
	if in.Role == "student" {
		if in.StudentLevel != "novice" && in.StudentLevel != "beginner" && in.StudentLevel != "intermediate" && in.StudentLevel != "advanced" {
			validation(a, w, r, "studentLevel is required for students")
			return
		}
	} else {
		in.StudentLevel = "not_applicable"
	}
	actor := actorFrom(r.Context())
	v, err := a.store.UpdateEnrollmentDetails(r.Context(), r.PathValue("id"), r.PathValue("memberId"), in.Revision, in.Role, in.StudentLevel, actor.UserID, time.Now().UTC())
	if err != nil {
		storeFailure(a, w, r, err)
		return
	}

	writeJSON(w, 200, v)
}

func (a *API) promoteMember(w http.ResponseWriter, r *http.Request) { a.changeMember(w, r, true) }

func (a *API) removeMember(w http.ResponseWriter, r *http.Request) { a.changeMember(w, r, false) }

func (a *API) changeMember(w http.ResponseWriter, r *http.Request, promote bool) {
	actor := actorFrom(r.Context())
	seasonID, memberID := r.PathValue("id"), r.PathValue("memberId")
	season, err := a.store.GetSeason(r.Context(), seasonID)
	if err != nil {
		storeFailure(a, w, r, err)
		return
	}
	if season.Status != "open" {
		a.fail(w, r, http.StatusConflict, "season_closed", "Season closed", "Season members cannot be changed until the season is reopened.", nil)
		return
	}
	v, err := a.store.GetEnrollment(r.Context(), memberID)
	if err != nil || v.SeasonID != seasonID {
		a.fail(w, r, 404, "not_found", "Not found", "The enrollment does not exist.", nil)
		return
	}
	if v.Role != "student" {
		a.fail(w, r, http.StatusForbidden, "student_action_required", "Student action required", "Only an active student enrollment can be promoted or removed through this operation.", nil)
		return
	}
	assigned, err := a.store.IsMentorAssigned(r.Context(), seasonID, actor.UserID, v.UserID)
	if err != nil {
		storeFailure(a, w, r, err)
		return
	}
	if !actor.CanPromoteOrRemove(seasonID, assigned, v.State == "active") {
		a.fail(w, r, 403, "member_admin_required", "Member administration required", "This member cannot be changed by the current account.", nil)
		return
	}
	seasonRole, _ := actor.Enrollment(seasonID)
	if (actor.IsPrivileged() || seasonRole.Role == authz.Coordinator) && !actor.HasRecentMFA(time.Now()) {
		a.fail(w, r, 403, "privileged_mfa_required", "Recent MFA required", "Recent MFA is required for this administrative action.", nil)
		return
	}
	var in struct {
		Role     string `json:"role"`
		Reason   string `json:"reason"`
		Revision int64  `json:"revision"`
	}
	if err := decodeJSON(w, r, &in); err != nil || in.Revision < 1 || (!promote && strings.TrimSpace(in.Reason) == "") {
		validation(a, w, r, "revision and removal reason are required")
		return
	}
	if promote && in.Role != "mentor" && in.Role != "coordinator" {
		validation(a, w, r, "role must be mentor or coordinator")
		return
	}
	if promote && in.Role == "coordinator" && !canGrantCoordinator(actor, seasonID) {
		a.fail(w, r, 403, "privileged_role_grant_required", "Privileged role grant required", "Only the season Coordinator, a Director, or a System Admin may grant Coordinator access.", nil)
		return
	}
	role, state := "", ""
	if promote {
		role = in.Role
	} else {
		state = "kicked"
	}
	updated, err := a.store.UpdateEnrollment(r.Context(), memberID, in.Revision, role, state, in.Reason, actor.UserID, time.Now())
	if err != nil {
		storeFailure(a, w, r, err)
		return
	}

	writeJSON(w, 200, updated)
}

func (a *API) listMentorships(w http.ResponseWriter, r *http.Request) {
	seasonID := r.PathValue("id")
	if !a.canViewSeason(r, seasonID) {
		a.fail(w, r, 403, "season_access_required", "Season access required", "The current account cannot access this season.", nil)
		return
	}
	sortBy, err := requestedSort(r, "id:asc", "id:asc", "student:asc")
	if err != nil {
		validation(a, w, r, "sort must be id:asc or student:asc")
		return
	}

	direction, err := requestedDirection(r)
	if err != nil {
		validation(a, w, r, "direction must be forward or backward")
		return
	}

	mentorUserID, studentUserID := strings.TrimSpace(r.URL.Query().Get("mentorUserId")), strings.TrimSpace(r.URL.Query().Get("studentUserId"))
	binding := "mentorships|season=" + seasonID + "|sort=" + sortBy + "|mentor=" + mentorUserID + "|student=" + studentUserID
	limit, boundary, err := a.page(r, binding)
	if err != nil {
		a.fail(w, r, 400, "invalid_cursor", "Invalid cursor", "The cursor does not match the selected season and sort.", nil)
		return
	}

	items, more, total, err := a.store.ListMentorships(r.Context(), seasonID, boundary, limit, sortBy, direction, mentorUserID, studentUserID)
	if err != nil {
		storeFailure(a, w, r, err)
		return
	}

	writeJSON(w, 200, model.Page[model.Mentorship]{Items: items, PageInfo: pageInfoForKeyset(a, binding, direction, boundary, items, more, func(v model.Mentorship) string { return v.ID }), TotalCount: total})
}

func (a *API) createMentorship(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.seasonAdmin(r); !ok {
		a.fail(w, r, 403, "season_admin_required", "Season administration required", "An administrator of an open season with recent MFA is required.", nil)
		return
	}
	var in struct {
		MentorUserID  string `json:"mentorUserId"`
		StudentUserID string `json:"studentUserId"`
	}
	if err := decodeJSON(w, r, &in); err != nil || in.MentorUserID == "" || in.StudentUserID == "" || in.MentorUserID == in.StudentUserID {
		validation(a, w, r, "different mentorUserId and studentUserId are required")
		return
	}

	v := model.Mentorship{ID: id.New(), SeasonID: r.PathValue("id"), MentorUserID: in.MentorUserID, StudentUserID: in.StudentUserID, Revision: 1}
	actor := actorFrom(r.Context())
	created, err := a.store.CreateMentorship(r.Context(), v, actor.UserID, time.Now().UTC())
	if err != nil {
		storeFailure(a, w, r, err)
		return
	}

	writeJSON(w, 201, created)
}

func (a *API) updateMentorship(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.seasonAdmin(r); !ok {
		a.fail(w, r, 403, "season_admin_required", "Season administration required", "An administrator of an open season with recent MFA is required.", nil)
		return
	}
	var in struct {
		MentorUserID  string `json:"mentorUserId"`
		StudentUserID string `json:"studentUserId"`
		Revision      int64  `json:"revision"`
	}
	if err := decodeJSON(w, r, &in); err != nil || in.Revision < 1 || in.MentorUserID == "" || in.StudentUserID == "" || in.MentorUserID == in.StudentUserID {
		validation(a, w, r, "different mentorUserId and studentUserId plus revision are required")
		return
	}

	actor := actorFrom(r.Context())
	v, err := a.store.UpdateMentorship(r.Context(), r.PathValue("id"), r.PathValue("mentorshipId"), in.Revision, in.MentorUserID, in.StudentUserID, actor.UserID, time.Now().UTC())
	if err != nil {
		storeFailure(a, w, r, err)
		return
	}

	writeJSON(w, 200, v)
}

func (a *API) deleteMentorship(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.seasonAdmin(r); !ok {
		a.fail(w, r, 403, "season_admin_required", "Season administration required", "An administrator of an open season with recent MFA is required.", nil)
		return
	}
	revision, err := parseRevision(r)
	if err != nil {
		validation(a, w, r, "revision query parameter is required")
		return
	}

	actor := actorFrom(r.Context())
	if err := a.store.DeleteMentorship(r.Context(), r.PathValue("id"), r.PathValue("mentorshipId"), revision, actor.UserID, time.Now().UTC()); err != nil {
		storeFailure(a, w, r, err)
		return
	}

	w.WriteHeader(204)
}
