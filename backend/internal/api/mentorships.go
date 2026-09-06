package api

import (
	"net/http"
	"strings"
	"time"

	"github.com/magedmg/RSP-website/backend/internal/dal"
	"github.com/magedmg/RSP-website/backend/internal/platform/id"
	"github.com/magedmg/RSP-website/backend/internal/programme"
)

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

	pageInfo, err := pageInfoForKeyset(
		a.cursorSecret, binding, direction, boundary, items, more,
		func(mentorship programme.MentorshipRecord) string { return mentorship.ID },
	)
	if err != nil {
		a.logger.Error("cursor encoding failed", "error", err)
		writeErrorResponse(w, http.StatusInternalServerError, "internal_error", "The request could not be completed.")
		return
	}
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
	if request.MentorUserID == "" {
		writeErrorResponse(w, http.StatusBadRequest, "validation_failed", "mentorUserId is required")
		return
	}
	if request.StudentUserID == "" {
		writeErrorResponse(w, http.StatusBadRequest, "validation_failed", "studentUserId is required")
		return
	}
	if request.MentorUserID == request.StudentUserID {
		writeErrorResponse(w, http.StatusBadRequest, "validation_failed", "mentorUserId and studentUserId must be different")
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
	if request.MentorUserID == "" {
		writeErrorResponse(w, http.StatusBadRequest, "validation_failed", "mentorUserId is required")
		return
	}
	if request.StudentUserID == "" {
		writeErrorResponse(w, http.StatusBadRequest, "validation_failed", "studentUserId is required")
		return
	}
	if request.MentorUserID == request.StudentUserID {
		writeErrorResponse(w, http.StatusBadRequest, "validation_failed", "mentorUserId and studentUserId must be different")
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
