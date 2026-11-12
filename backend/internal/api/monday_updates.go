package api

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/Ravis-Study-Program/RSP-website/backend/internal/authz"
	"github.com/Ravis-Study-Program/RSP-website/backend/internal/dal"
	"github.com/Ravis-Study-Program/RSP-website/backend/internal/platform/id"
	"github.com/Ravis-Study-Program/RSP-website/backend/internal/program"
)

// mondayUpdateRange reads the startDate and endDate parameters Monday update
// reads require.
func mondayUpdateRange(w http.ResponseWriter, r *http.Request) (string, string, bool) {
	startDate, endDate := r.URL.Query().Get("startDate"), r.URL.Query().Get("endDate")
	if _, err := program.ParseMondayUpdateDate(startDate); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, err.Error())
		return "", "", false
	}
	if _, err := program.ParseMondayUpdateDate(endDate); err != nil {
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
		func(update program.MondayUpdateRecord) string { return update.ID },
	)
	if err != nil {
		a.logger.Error("cursor encoding failed", "error", err)
		writeErrorResponse(w, http.StatusInternalServerError, "The request could not be completed.")
		return
	}
	writeJSONResponse(w, http.StatusOK, Page[program.MondayUpdateRecord]{
		Items:      items,
		PageInfo:   pageInfo,
		TotalCount: total,
	})
}

type createMondayUpdateRequest struct {
	StartDate   string   `json:"startDate"`
	EndDate     string   `json:"endDate"`
	ContentHTML string   `json:"contentHtml"`
	Hours       *float64 `json:"hours"`
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
	if err := program.ValidateMondayUpdateDates(request.StartDate, request.EndDate, time.Now()); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	// Writing is optional: an update can be just its hours and activity.
	content := a.sanitizeHTML(request.ContentHTML)
	if request.Hours == nil {
		writeErrorResponse(w, http.StatusBadRequest, "hours is required")
		return
	}
	if err := program.ValidateMondayUpdateHours(*request.Hours); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	// A week outside the program is reported as such, before the late lock.
	if configured, err := a.db.MondayUpdateWeekConfigured(r.Context(), seasonID, request.StartDate); err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	} else if !configured {
		a.writeStoreErrorResponse(w, dal.ErrMondayUpdateWeek)
		return
	}
	if !a.mondayUpdateUnlocked(w, r, seasonID, actor.UserID, request.StartDate, request.EndDate) {
		return
	}
	created, err := a.db.CreateMondayUpdate(r.Context(), program.MondayUpdateRecord{
		ID:          id.New(),
		SeasonID:    seasonID,
		Student:     program.PersonSummary{ID: actor.UserID},
		StartDate:   request.StartDate,
		EndDate:     request.EndDate,
		ContentHTML: content,
		Hours:       *request.Hours,
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
	if !a.mondayUpdateUnlocked(w, r, seasonID, actor.UserID, record.StartDate, record.EndDate) {
		return
	}
	if err := a.db.DeleteMondayUpdate(r.Context(), seasonID, updateID, actor.UserID, actor.UserID); err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

type editMondayUpdateRequest struct {
	ContentHTML string   `json:"contentHtml"`
	Hours       *float64 `json:"hours"`
}

// editMondayUpdate lets a student rewrite their own update's content and
// hours while the season is open.
func (a *API) editMondayUpdate(w http.ResponseWriter, r *http.Request) {
	actor := actorFrom(r.Context())
	seasonID, updateID := r.PathValue("id"), r.PathValue("updateId")
	var request editMondayUpdateRequest
	if err := decodeJSON(r.Body, &request); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	// Writing is optional: an update can be just its hours and activity.
	content := a.sanitizeHTML(request.ContentHTML)
	if request.Hours == nil {
		writeErrorResponse(w, http.StatusBadRequest, "hours is required")
		return
	}
	if err := program.ValidateMondayUpdateHours(*request.Hours); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	record, err := a.db.GetMondayUpdate(r.Context(), seasonID, updateID)
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}
	if record.Student.ID != actor.UserID {
		writeErrorResponse(w, http.StatusForbidden, "Only the author can edit a Monday update.")
		return
	}
	if !a.mondayUpdateUnlocked(w, r, seasonID, actor.UserID, record.StartDate, record.EndDate) {
		return
	}
	updated, err := a.db.UpdateMondayUpdate(r.Context(), seasonID, updateID, actor.UserID, content, *request.Hours, actor.UserID)
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}

	writeJSONResponse(w, http.StatusOK, updated)
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
	// An empty mark is "Not marked": it clears the mentor's mark.
	if request.Mark != "" && !program.ValidMondayUpdateMark(request.Mark) {
		writeErrorResponse(w, http.StatusBadRequest, "mark must be one of way_ahead, doing_good, on_track, a_bit_behind, at_risk or kicked, or empty to clear it")
		return
	}
	record, err := a.db.GetMondayUpdate(r.Context(), seasonID, updateID)
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}
	assigned, err := a.db.IsMentorForWeek(r.Context(), seasonID, actor.UserID, record.Student.ID, record.StartDate)
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}
	if !assigned {
		writeErrorResponse(w, http.StatusForbidden, "Only a mentor of this student can mark this update.")
		return
	}
	if request.Mark == "" {
		if err := a.db.ClearMondayUpdateMark(r.Context(), updateID, actor.UserID); err != nil {
			a.writeStoreErrorResponse(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
		return
	}

	saved, err := a.db.UpsertMondayUpdateMark(r.Context(), program.MondayUpdateMarkRecord{
		ID:          id.New(),
		UpdateID:    updateID,
		Mentor:      program.PersonSummary{ID: actor.UserID},
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
		func(row program.MondayUpdateReviewRow) string { return row.Student.ID },
	)
	if err != nil {
		a.logger.Error("cursor encoding failed", "error", err)
		writeErrorResponse(w, http.StatusInternalServerError, "The request could not be completed.")
		return
	}
	writeJSONResponse(w, http.StatusOK, Page[program.MondayUpdateReviewRow]{
		Items:      items,
		PageInfo:   pageInfo,
		TotalCount: total,
	})
}

// getMondayUpdateActivity returns the active student's own LeetCode attempts
// and mock interviews inside a date range, using the same Adelaide date bounds
// an update with that range would show, so a student sees it before posting.
func (a *API) getMondayUpdateActivity(w http.ResponseWriter, r *http.Request) {
	actor := actorFrom(r.Context())
	seasonID := r.PathValue("id")
	enrollment, enrolled := actor.Enrollment(seasonID)
	if !enrolled || enrollment.Role != authz.Student || enrollment.State != authz.Active {
		writeErrorResponse(w, http.StatusForbidden, "Only an active student enrollment can read its Monday update activity in this season.")
		return
	}
	startDate, endDate, ok := mondayUpdateRange(w, r)
	if !ok {
		return
	}
	if endDate < startDate {
		writeErrorResponse(w, http.StatusBadRequest, "endDate must not be before startDate")
		return
	}
	activity, err := a.db.GetMondayUpdateRangeActivity(r.Context(), actor.UserID, startDate, endDate)
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}
	writeJSONResponse(w, http.StatusOK, activity)
}

// mondayNow is the clock the late lock reads; tests pin it.
var mondayNow = time.Now

// mondayUpdateUnlocked writes a 409 and returns false when a week is past its
// due date and the student's mentor has not unlocked it.
func (a *API) mondayUpdateUnlocked(w http.ResponseWriter, r *http.Request, seasonID, studentID, startDate, endDate string) bool {
	if !program.MondayUpdatePastDue(endDate, mondayNow()) {
		return true
	}
	approved, err := a.db.HasApprovedMondayUpdateUnlock(r.Context(), seasonID, studentID, startDate)
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return false
	}
	if !approved {
		writeErrorResponse(w, http.StatusConflict, "This week's update is past its due date. Ask your mentor to unlock it first.")
		return false
	}
	return true
}

type createMondayUpdateUnlockRequest struct {
	StartDate string `json:"startDate"`
	EndDate   string `json:"endDate"`
	Reason    string `json:"reason"`
	// StudentID is set when a mentor or coordinator unlocks a student's
	// week directly.
	StudentID string `json:"studentId"`
}

// createMondayUpdateUnlock asks the student's mentor to unlock a late week,
// or, from a mentor or coordinator, unlocks it straight away.
func (a *API) createMondayUpdateUnlock(w http.ResponseWriter, r *http.Request) {
	actor := actorFrom(r.Context())
	seasonID := r.PathValue("id")
	var request createMondayUpdateUnlockRequest
	if err := decodeJSON(r.Body, &request); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	if request.StudentID != "" && request.StudentID != actor.UserID {
		a.unlockMondayUpdateForStudent(w, r, seasonID, request)
		return
	}
	enrollment, enrolled := actor.Enrollment(seasonID)
	if !enrolled || enrollment.Role != authz.Student || enrollment.State != authz.Active {
		writeErrorResponse(w, http.StatusForbidden, "Only an active student enrollment can ask to unlock a Monday update.")
		return
	}
	if err := program.ValidateMondayUpdateDates(request.StartDate, request.EndDate, time.Now()); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	if !program.MondayUpdatePastDue(request.EndDate, mondayNow()) {
		writeErrorResponse(w, http.StatusBadRequest, "This week is not past its due date, so it does not need unlocking.")
		return
	}
	reason := strings.TrimSpace(request.Reason)
	if reason == "" || len([]rune(reason)) > program.MaxUnlockReasonLength {
		writeErrorResponse(w, http.StatusBadRequest, fmt.Sprintf("Give a reason of up to %d characters.", program.MaxUnlockReasonLength))
		return
	}
	created, err := a.db.CreateMondayUpdateUnlock(r.Context(), program.MondayUpdateUnlock{
		ID:        id.New(),
		SeasonID:  seasonID,
		Student:   program.PersonSummary{ID: actor.UserID},
		StartDate: request.StartDate,
		EndDate:   request.EndDate,
		Reason:    reason,
	}, actor.UserID)
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}
	writeJSONResponse(w, http.StatusCreated, created)
}

// listMondayUpdateUnlocks returns the requests the actor may see: a
// student's own, a mentor's mentees', or every one for coordinators.
func (a *API) listMondayUpdateUnlocks(w http.ResponseWriter, r *http.Request) {
	actor := actorFrom(r.Context())
	seasonID := r.PathValue("id")
	query := dal.MondayUpdateUnlockQuery{SeasonID: seasonID}
	enrollment, enrolled := actor.Enrollment(seasonID)
	switch {
	case actor.IsSeasonAdmin(seasonID):
	case enrolled && (enrollment.State == authz.Active || enrollment.State == authz.Completed) && enrollment.Role == authz.Mentor:
		query.MentorUserID = actor.UserID
	case enrolled && enrollment.Role == authz.Student:
		query.StudentUserID = actor.UserID
	default:
		writeErrorResponse(w, http.StatusForbidden, "The current account cannot access this season's Monday updates.")
		return
	}
	items, err := a.db.ListMondayUpdateUnlocks(r.Context(), query)
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}
	writeJSONResponse(w, http.StatusOK, map[string]any{"items": items})
}

type decideMondayUpdateUnlockRequest struct {
	Decision string `json:"decision"`
}

// decideMondayUpdateUnlock lets the student's mentor, or a coordinator,
// approve or decline a pending request.
func (a *API) decideMondayUpdateUnlock(w http.ResponseWriter, r *http.Request) {
	actor := actorFrom(r.Context())
	seasonID, unlockID := r.PathValue("id"), r.PathValue("unlockId")
	var request decideMondayUpdateUnlockRequest
	if err := decodeJSON(r.Body, &request); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	if request.Decision != "approved" && request.Decision != "declined" {
		writeErrorResponse(w, http.StatusBadRequest, "decision must be approved or declined")
		return
	}
	unlock, err := a.db.GetMondayUpdateUnlock(r.Context(), seasonID, unlockID)
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}
	if !actor.IsSeasonAdmin(seasonID) {
		assigned, err := a.db.IsMentorAssigned(r.Context(), seasonID, actor.UserID, unlock.Student.ID)
		if err != nil {
			a.writeStoreErrorResponse(w, err)
			return
		}
		if !assigned {
			writeErrorResponse(w, http.StatusForbidden, "Only a mentor of this student can decide this request.")
			return
		}
	}
	decided, err := a.db.DecideMondayUpdateUnlock(r.Context(), seasonID, unlockID, request.Decision == "approved", actor.UserID)
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}
	writeJSONResponse(w, http.StatusOK, decided)
}

// unlockMondayUpdateForStudent is a mentor or coordinator unlocking a
// student's late week, approving their request if they made one.
func (a *API) unlockMondayUpdateForStudent(w http.ResponseWriter, r *http.Request, seasonID string, request createMondayUpdateUnlockRequest) {
	actor := actorFrom(r.Context())
	if !actor.IsSeasonAdmin(seasonID) {
		assigned, err := a.db.IsMentorAssigned(r.Context(), seasonID, actor.UserID, request.StudentID)
		if err != nil {
			a.writeStoreErrorResponse(w, err)
			return
		}
		if !assigned {
			writeErrorResponse(w, http.StatusForbidden, "Only a mentor of this student can unlock their Monday update.")
			return
		}
	}
	if err := program.ValidateMondayUpdateDates(request.StartDate, request.EndDate, time.Now()); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	unlocked, err := a.db.UnlockMondayUpdate(r.Context(), program.MondayUpdateUnlock{
		ID:        id.New(),
		SeasonID:  seasonID,
		Student:   program.PersonSummary{ID: request.StudentID},
		StartDate: request.StartDate,
		EndDate:   request.EndDate,
		Reason:    "Unlocked by a mentor.",
	}, actor.UserID)
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}
	writeJSONResponse(w, http.StatusCreated, unlocked)
}

type mondayWeekMarkRequest struct {
	StudentID   string `json:"studentId"`
	StartDate   string `json:"startDate"`
	EndDate     string `json:"endDate"`
	Mark        string `json:"mark"`
	CommentHTML string `json:"commentHtml"`
}

// putMondayWeekMark saves a mentor's mark on a mentee's week that has no
// update, so every week can be marked before the mentor sends it.
func (a *API) putMondayWeekMark(w http.ResponseWriter, r *http.Request) {
	actor := actorFrom(r.Context())
	seasonID := r.PathValue("id")
	var request mondayWeekMarkRequest
	if err := decodeJSON(r.Body, &request); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	// An empty mark is "Not marked": it clears the mentor's mark.
	if request.Mark != "" && !program.ValidMondayUpdateMark(request.Mark) {
		writeErrorResponse(w, http.StatusBadRequest, "mark must be one of way_ahead, doing_good, on_track, a_bit_behind, at_risk or kicked, or empty to clear it")
		return
	}
	if err := program.ValidateMondayUpdateDates(request.StartDate, request.EndDate, time.Now()); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	assigned, err := a.db.IsMentorForWeek(r.Context(), seasonID, actor.UserID, request.StudentID, request.StartDate)
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}
	if !assigned {
		writeErrorResponse(w, http.StatusForbidden, "Only a mentor of this student can mark their week.")
		return
	}
	if request.Mark == "" {
		if err := a.db.ClearMondayWeekMark(r.Context(), seasonID, request.StudentID, actor.UserID, request.StartDate); err != nil {
			a.writeStoreErrorResponse(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
		return
	}
	saved, err := a.db.UpsertMondayWeekMark(r.Context(), program.MondayWeekMark{
		SeasonID:    seasonID,
		StudentID:   request.StudentID,
		MentorID:    actor.UserID,
		StartDate:   request.StartDate,
		EndDate:     request.EndDate,
		Mark:        request.Mark,
		CommentHTML: a.sanitizeHTML(request.CommentHTML),
	}, actor.UserID)
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}
	writeJSONResponse(w, http.StatusOK, saved)
}

// listMondayMentorReports returns each mentor's week for the coordinator,
// or the signed-in mentor's own.
func (a *API) listMondayMentorReports(w http.ResponseWriter, r *http.Request) {
	actor := actorFrom(r.Context())
	seasonID := r.PathValue("id")
	startDate, endDate := r.URL.Query().Get("startDate"), r.URL.Query().Get("endDate")
	if err := program.ValidateMondayUpdateDates(startDate, endDate, time.Now()); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	mentorID := ""
	enrollment, enrolled := actor.Enrollment(seasonID)
	switch {
	case actor.IsSeasonAdmin(seasonID):
	case enrolled && (enrollment.State == authz.Active || enrollment.State == authz.Completed) && enrollment.Role == authz.Mentor:
		mentorID = actor.UserID
	default:
		writeErrorResponse(w, http.StatusForbidden, "Only mentors and coordinators can read mentor reports.")
		return
	}
	reports, err := a.db.ListMondayMentorReports(r.Context(), seasonID, startDate, endDate, mentorID)
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}
	writeJSONResponse(w, http.StatusOK, map[string]any{"items": reports})
}
