package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/magedmg/RSP-website/backend/internal/dal"
	"github.com/magedmg/RSP-website/backend/internal/platform/cursor"
	"github.com/magedmg/RSP-website/backend/internal/platform/id"
	"github.com/magedmg/RSP-website/backend/internal/platform/sanitize"
	"github.com/magedmg/RSP-website/backend/internal/practice"
)

func (a *API) listProblemAttempts(w http.ResponseWriter, r *http.Request) {
	actor := actorFrom(r.Context())
	if !actor.CanRecordActivity() && !actor.IsDirectorOrSystemAdmin() {
		writeErrorResponse(w, http.StatusForbidden, "No season access yet.")
		return
	}
	targetID := r.URL.Query().Get("userId")
	if targetID == "" {
		targetID = actor.UserID
	}
	seasonID, year, filterErr := activityFilters(r, actor)
	if filterErr != nil {
		writeErrorResponse(w, http.StatusBadRequest, filterErr.Error())
		return
	}
	if !a.authorizeActivityRead(w, r, targetID, seasonID) {
		return
	}
	outcome := r.URL.Query().Get("outcome")
	difficulties, err := selection(r.URL.Query(), "difficulty", difficultySelection)
	if err != nil {
		writeErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	categories, err := selection(r.URL.Query(), "category", nil)
	if err != nil {
		writeErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	weeks, err := selection(r.URL.Query(), "weekId", uuidSelection)
	if err != nil {
		writeErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	if len(weeks) > 0 && seasonID == "" {
		writeErrorResponse(w, http.StatusBadRequest, "select a season before filtering weeks")
		return
	}
	filterBinding, _ := json.Marshal([][]string{difficulties, categories, weeks})
	if _, err := parseSort(r.URL.Query().Get("sort"), "id:asc", "id:asc"); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "sort must be id:asc")
		return
	}

	direction := r.URL.Query().Get("direction")
	if direction == "" {
		direction = "forward"
	}
	if direction != "forward" && direction != "backward" {
		writeErrorResponse(w, http.StatusBadRequest, "direction must be forward or backward")
		return
	}

	binding := "attempts|id:asc|user=" + targetID + "|outcome=" + outcome + "|selection=" + string(filterBinding) + fmt.Sprintf("|season=%s|year=%d", seasonID, year)
	limit, after, err := parsePagination(r.URL.Query().Get("limit"), r.URL.Query().Get("cursor"), binding, a.cursorSecret)
	if err != nil {
		writeErrorResponse(w, http.StatusBadRequest, cursor.ErrInvalid.Error())
		return
	}

	items, more, total, err := a.db.ListAttempts(r.Context(), dal.AttemptQuery{
		SeasonID: seasonID, Year: year,
		UserID:       targetID,
		Boundary:     after,
		Limit:        limit,
		Outcome:      outcome,
		Difficulties: difficulties, Categories: categories, WeekIDs: weeks,
		Direction: direction,
	})
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}

	if targetID != actor.UserID {
		if err := a.auditPrivateDataRead(r.Context(), actor, "problem_attempt_history", targetID); err != nil {
			writeErrorResponse(w, http.StatusInternalServerError, "Private data was not returned because its access could not be audited.")
			return
		}
	}

	pageInfo, err := pageInfoForKeyset(
		a.cursorSecret, binding, direction, after, items, more,
		func(attempt practice.AttemptRecord) string { return attempt.ID },
	)
	if err != nil {
		a.logger.Error("cursor encoding failed", "error", err)
		writeErrorResponse(w, http.StatusInternalServerError, "The request could not be completed.")
		return
	}
	response := Page[practice.AttemptRecord]{
		Items:      items,
		PageInfo:   pageInfo,
		TotalCount: total,
	}
	writeJSONResponse(w, http.StatusOK, response)
}

type attemptInput struct {
	ProblemID, Outcome, Notes string
	Confidence                *int
	Minutes                   int
	AttemptedAt               time.Time
}

func validateAttempt(request attemptInput) error {
	if request.ProblemID == "" {
		return fmt.Errorf("problemId is required")
	}
	if request.Minutes <= 0 {
		return fmt.Errorf("minutes must be greater than zero")
	}
	if request.AttemptedAt.IsZero() {
		return fmt.Errorf("attemptedAt is required")
	}
	switch request.Outcome {
	case string(practice.Independent), string(practice.WithHints), string(practice.NotSolved):
	default:
		return fmt.Errorf("outcome must be independently_solved, solved_with_hints, or not_solved")
	}
	if request.Confidence != nil && (*request.Confidence < 1 || *request.Confidence > 5) {
		return fmt.Errorf("confidence must be between 1 and 5")
	}
	return nil
}

func (a *API) createProblemAttempt(w http.ResponseWriter, r *http.Request) {
	actor := actorFrom(r.Context())
	if !actor.CanRecordActivity() && !actor.IsDirectorOrSystemAdmin() {
		writeErrorResponse(w, http.StatusForbidden, "No season access yet.")
		return
	}
	var request attemptInput
	if err := decodeJSON(r.Body, &request); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := validateAttempt(request); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	attempt := practice.AttemptRecord{
		ID:          id.New(),
		UserID:      actor.UserID,
		ProblemID:   request.ProblemID,
		Outcome:     request.Outcome,
		Confidence:  request.Confidence,
		Minutes:     request.Minutes,
		Notes:       sanitize.New().String(request.Notes),
		AttemptedAt: request.AttemptedAt.UTC(),
	}
	created, err := a.db.CreateAttempt(r.Context(), attempt)
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}

	writeJSONResponse(w, http.StatusCreated, created)
}

func (a *API) updateProblemAttempt(w http.ResponseWriter, r *http.Request) {
	actor := actorFrom(r.Context())
	if !actor.CanRecordActivity() && !actor.IsDirectorOrSystemAdmin() {
		writeErrorResponse(w, http.StatusForbidden, "No season access yet.")
		return
	}
	var request attemptInput
	if err := decodeJSON(r.Body, &request); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := validateAttempt(request); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	updated, err := a.db.UpdateAttempt(r.Context(), dal.UpdateAttemptInput{
		AttemptID:   r.PathValue("id"),
		UserID:      actor.UserID,
		ProblemID:   request.ProblemID,
		Outcome:     request.Outcome,
		Confidence:  request.Confidence,
		Minutes:     request.Minutes,
		Notes:       sanitize.New().String(request.Notes),
		AttemptedAt: request.AttemptedAt.UTC(),
		ChangedAt:   time.Now().UTC(),
	})
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}

	writeJSONResponse(w, http.StatusOK, updated)
}

func (a *API) deleteProblemAttempt(w http.ResponseWriter, r *http.Request) {
	actor := actorFrom(r.Context())
	if !actor.CanRecordActivity() && !actor.IsDirectorOrSystemAdmin() {
		writeErrorResponse(w, http.StatusForbidden, "No season access yet.")
		return
	}
	if err := a.db.DeleteAttempt(r.Context(), r.PathValue("id"), actor.UserID); err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
