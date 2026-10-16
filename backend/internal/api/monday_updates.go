package api

import (
	"net/http"
	"strings"
	"time"

	"github.com/magedmg/RSP-website/backend/internal/authz"
	"github.com/magedmg/RSP-website/backend/internal/dal"
	"github.com/magedmg/RSP-website/backend/internal/platform/id"
	"github.com/magedmg/RSP-website/backend/internal/programme"
)

// mondayUpdateRange reads the startDate and endDate parameters Monday update
// reads require.
func mondayUpdateRange(w http.ResponseWriter, r *http.Request) (string, string, bool) {
	startDate, endDate := r.URL.Query().Get("startDate"), r.URL.Query().Get("endDate")
	if _, err := programme.ParseMondayUpdateDate(startDate); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, err.Error())
		return "", "", false
	}
	if _, err := programme.ParseMondayUpdateDate(endDate); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, err.Error())
		return "", "", false
	}
	return startDate, endDate, true
}

func (a *API) listMondayUpdates(w http.ResponseWriter, r *http.Request) {
	actor := actorFrom(r.Context())
	seasonID := r.PathValue("id")
	startDate, endDate, ok := mondayUpdateRange(w, r)
	if !ok {
		return
	}
	if _, err := a.db.GetSeason(r.Context(), seasonID); err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}
	studentIDs, err := selection(r.URL.Query(), "studentId", uuidSelection)
	if err != nil {
		writeErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	// The season scope decides which students the actor may read and whether
	// marks are visible at all.
	var mentorUserID string
	includeMarks := false
	enrollment, enrolled := actor.Enrollment(seasonID)
	switch {
	case actor.IsSeasonAdmin(seasonID):
		includeMarks = true
	case enrolled && (enrollment.State == authz.Active || enrollment.State == authz.Completed) && enrollment.Role == authz.Mentor:
		mentorUserID = actor.UserID
		includeMarks = true
	case enrolled && enrollment.State == authz.Active && enrollment.Role == authz.Student:
		studentIDs = []string{actor.UserID}
	default:
		writeErrorResponse(w, http.StatusForbidden, "The current account cannot access this season's Monday updates.")
		return
	}

	binding := "monday-updates|season=" + seasonID + "|start=" + startDate + "|end=" + endDate +
		"|students=" + strings.Join(studentIDs, ",") + "|mentor=" + mentorUserID
	limit, boundary, err := parsePagination(r.URL.Query().Get("limit"), r.URL.Query().Get("cursor"), binding, a.cursorSecret)
	if err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "The cursor does not match the selected season and range.")
		return
	}

	items, more, total, err := a.db.ListMondayUpdates(r.Context(), dal.MondayUpdateQuery{
		SeasonID:     seasonID,
		StartDate:    startDate,
		EndDate:      endDate,
		StudentIDs:   studentIDs,
		MentorUserID: mentorUserID,
		Boundary:     boundary,
		Limit:        limit,
		IncludeMarks: includeMarks,
	})
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}

	pageInfo, err := pageInfoForKeyset(
		a.cursorSecret, binding, "forward", boundary, items, more,
		func(update programme.MondayUpdateRecord) string { return update.ID },
	)
	if err != nil {
		a.logger.Error("cursor encoding failed", "error", err)
		writeErrorResponse(w, http.StatusInternalServerError, "The request could not be completed.")
		return
	}
	writeJSONResponse(w, http.StatusOK, Page[programme.MondayUpdateRecord]{
		Items:      items,
		PageInfo:   pageInfo,
		TotalCount: total,
	})
}

type createMondayUpdateRequest struct {
	StartDate   string `json:"startDate"`
	EndDate     string `json:"endDate"`
	ContentHTML string `json:"contentHtml"`
}

func (a *API) createMondayUpdate(w http.ResponseWriter, r *http.Request) {
	actor := actorFrom(r.Context())
	seasonID := r.PathValue("id")
	enrollment, enrolled := actor.Enrollment(seasonID)
	if !enrolled || enrollment.Role != authz.Student || enrollment.State != authz.Active {
		writeErrorResponse(w, http.StatusForbidden, "Only an active student enrollment can post a Monday update in this season.")
		return
	}
	var request createMondayUpdateRequest
	if err := decodeJSON(r.Body, &request); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := programme.ValidateMondayUpdateDates(request.StartDate, request.EndDate, time.Now()); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	content := a.sanitizeHTML(request.ContentHTML)
	if strings.TrimSpace(content) == "" {
		writeErrorResponse(w, http.StatusBadRequest, "contentHtml is required")
		return
	}

	created, err := a.db.CreateMondayUpdate(r.Context(), programme.MondayUpdateRecord{
		ID:          id.New(),
		SeasonID:    seasonID,
		Student:     programme.PersonSummary{ID: actor.UserID},
		StartDate:   request.StartDate,
		EndDate:     request.EndDate,
		ContentHTML: content,
	}, actor.UserID)
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}

	writeJSONResponse(w, http.StatusCreated, created)
}

func (a *API) deleteMondayUpdate(w http.ResponseWriter, r *http.Request) {
	actor := actorFrom(r.Context())
	seasonID, updateID := r.PathValue("id"), r.PathValue("updateId")
	record, err := a.db.GetMondayUpdate(r.Context(), seasonID, updateID)
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}
	if record.Student.ID != actor.UserID {
		writeErrorResponse(w, http.StatusForbidden, "Only the author can delete a Monday update.")
		return
	}
	if err := a.db.DeleteMondayUpdate(r.Context(), seasonID, updateID, actor.UserID, actor.UserID); err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

type mondayUpdateMarkRequest struct {
	Mark        string `json:"mark"`
	CommentHTML string `json:"commentHtml"`
}

func (a *API) putMondayUpdateMark(w http.ResponseWriter, r *http.Request) {
	actor := actorFrom(r.Context())
	seasonID, updateID := r.PathValue("id"), r.PathValue("updateId")
	var request mondayUpdateMarkRequest
	if err := decodeJSON(r.Body, &request); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	if !programme.ValidMondayUpdateMark(request.Mark) {
		writeErrorResponse(w, http.StatusBadRequest, "mark must be one of way_ahead, doing_good, on_track, a_bit_behind or at_risk")
		return
	}
	record, err := a.db.GetMondayUpdate(r.Context(), seasonID, updateID)
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}
	assigned, err := a.db.IsMentorAssigned(r.Context(), seasonID, actor.UserID, record.Student.ID)
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}
	if !assigned {
		writeErrorResponse(w, http.StatusForbidden, "Only a mentor of this student can mark this update.")
		return
	}

	saved, err := a.db.UpsertMondayUpdateMark(r.Context(), programme.MondayUpdateMarkRecord{
		ID:          id.New(),
		UpdateID:    updateID,
		Mentor:      programme.PersonSummary{ID: actor.UserID},
		Mark:        request.Mark,
		CommentHTML: a.sanitizeHTML(request.CommentHTML),
	}, actor.UserID)
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}

	writeJSONResponse(w, http.StatusOK, saved)
}

func (a *API) listMondayUpdateReview(w http.ResponseWriter, r *http.Request) {
	actor := actorFrom(r.Context())
	seasonID := r.PathValue("id")
	if !actor.IsSeasonAdmin(seasonID) {
		writeErrorResponse(w, http.StatusForbidden, "Season administrator access is required.")
		return
	}
	startDate, endDate, ok := mondayUpdateRange(w, r)
	if !ok {
		return
	}
	binding := "monday-update-review|season=" + seasonID + "|start=" + startDate + "|end=" + endDate
	limit, boundary, err := parsePagination(r.URL.Query().Get("limit"), r.URL.Query().Get("cursor"), binding, a.cursorSecret)
	if err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "The cursor does not match the selected season and range.")
		return
	}

	items, more, total, err := a.db.ListMondayUpdateReview(r.Context(), dal.MondayUpdateReviewQuery{
		SeasonID:  seasonID,
		StartDate: startDate,
		EndDate:   endDate,
		Boundary:  boundary,
		Limit:     limit,
	})
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}

	pageInfo, err := pageInfoForKeyset(
		a.cursorSecret, binding, "forward", boundary, items, more,
		func(row programme.MondayUpdateReviewRow) string { return row.Student.ID },
	)
	if err != nil {
		a.logger.Error("cursor encoding failed", "error", err)
		writeErrorResponse(w, http.StatusInternalServerError, "The request could not be completed.")
		return
	}
	writeJSONResponse(w, http.StatusOK, Page[programme.MondayUpdateReviewRow]{
		Items:      items,
		PageInfo:   pageInfo,
		TotalCount: total,
	})
}
