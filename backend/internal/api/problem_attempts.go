package api

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/magedmg/RSP-website/backend/internal/authz"
	"github.com/magedmg/RSP-website/backend/internal/dal"
	"github.com/magedmg/RSP-website/backend/internal/platform/cursor"
	"github.com/magedmg/RSP-website/backend/internal/platform/id"
	"github.com/magedmg/RSP-website/backend/internal/platform/sanitize"
	"github.com/magedmg/RSP-website/backend/internal/practice"
	"github.com/magedmg/RSP-website/backend/internal/programme"
)

func (a *API) listProblemAttempts(w http.ResponseWriter, r *http.Request) {
	actor := actorFrom(r.Context())
	if !actor.CanAccessProgramme() && !actor.IsDirectorOrSystemAdmin() {
		writeErrorResponse(w, http.StatusForbidden, "season_access_required", "No season access yet.")
		return
	}
	targetID := r.URL.Query().Get("userId")
	if targetID == "" {
		targetID = actor.UserID
	}
	if targetID != actor.UserID && !actor.CanAccessMemberDirectory() && !actor.IsDirectorOrSystemAdmin() {
		writeErrorResponse(w, http.StatusForbidden, "directory_access_required", "Only active members and student alumni may view another member's public problem history.")
		return
	}
	if targetID != actor.UserID && !actor.IsDirectorOrSystemAdmin() {
		participant, err := a.db.GetMemberStatus(r.Context(), targetID)
		if err != nil && !errors.Is(err, dal.ErrNotFound) {
			a.writeStoreErrorResponse(w, err)
			return
		}
		if err != nil || !participant.IsVisibleInDirectory() {
			writeErrorResponse(w, http.StatusNotFound, "not_found", "The requested member does not exist.")
			return
		}
	}

	outcome, difficulty := r.URL.Query().Get("outcome"), r.URL.Query().Get("difficulty")
	if _, err := parseSort(r.URL.Query().Get("sort"), "id:asc", "id:asc"); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "validation_failed", "sort must be id:asc")
		return
	}

	direction := r.URL.Query().Get("direction")
	if direction == "" {
		direction = "forward"
	}
	if direction != "forward" && direction != "backward" {
		writeErrorResponse(w, http.StatusBadRequest, "validation_failed", "direction must be forward or backward")
		return
	}

	binding := "attempts|id:asc|user=" + targetID + "|outcome=" + outcome + "|difficulty=" + difficulty
	limit, after, err := parsePagination(r.URL.Query().Get("limit"), r.URL.Query().Get("cursor"), binding, a.cursorSecret)
	if err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "invalid_cursor", cursor.ErrInvalid.Error())
		return
	}

	items, more, total, err := a.db.ListAttempts(r.Context(), dal.AttemptQuery{
		UserID:     targetID,
		Boundary:   after,
		Limit:      limit,
		Outcome:    outcome,
		Difficulty: difficulty,
		Direction:  direction,
	})
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}

	if targetID != actor.UserID {
		if err := a.auditPrivateDataRead(r.Context(), actor, "problem_attempt_history", targetID); err != nil {
			writeErrorResponse(w, http.StatusInternalServerError, "audit_failed", "Private data was not returned because its access could not be audited.")
			return
		}
	}
	if targetID != actor.UserID && !actor.IsDirectorOrSystemAdmin() {
		canPrivate := actor.CanViewMemberPrivateData(targetID, authz.MemberRelationship{})
		var targetEnrollments []programme.EnrollmentRecord
		targetLoaded := false
		for _, enrollment := range actor.Enrollments {
			if canPrivate {
				break
			}
			if enrollment.State != authz.Active && enrollment.State != authz.Completed {
				continue
			}
			relationship := authz.MemberRelationship{SeasonID: enrollment.SeasonID}
			if enrollment.Role == authz.Coordinator {
				if !targetLoaded {
					var err error
					targetEnrollments, err = a.db.ListEnrollmentsForUser(r.Context(), targetID)
					if err != nil {
						a.writeStoreErrorResponse(w, err)
						return
					}

					targetLoaded = true
				}
				for _, target := range targetEnrollments {
					if target.SeasonID == enrollment.SeasonID && (target.State == "active" || target.State == "completed") {
						relationship.TargetEnrolled = true
						break
					}
				}
			}
			if enrollment.Role == authz.Mentor {
				assigned, err := a.db.IsMentorAssigned(r.Context(), enrollment.SeasonID, actor.UserID, targetID)
				if err != nil {
					a.writeStoreErrorResponse(w, err)
					return
				}
				relationship.AssignedMentor = assigned
			}
			if actor.CanViewMemberPrivateData(targetID, relationship) {
				canPrivate = true
				break
			}
		}

		if !canPrivate {
			for i := range items {
				items[i].Notes = ""
				items[i].Confidence = nil
				items[i].SeasonID = nil
				items[i].WeekID = nil
			}
		}
	}

	pageInfo, err := pageInfoForKeyset(
		a.cursorSecret, binding, direction, after, items, more,
		func(attempt practice.AttemptRecord) string { return attempt.ID },
	)
	if err != nil {
		a.logger.Error("cursor encoding failed", "error", err)
		writeErrorResponse(w, http.StatusInternalServerError, "internal_error", "The request could not be completed.")
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
	SeasonID, WeekID          *string
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
	if request.WeekID != nil && request.SeasonID == nil {
		return fmt.Errorf("seasonId is required when weekId is provided")
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
	if !actor.CanAccessProgramme() && !actor.IsDirectorOrSystemAdmin() {
		writeErrorResponse(w, http.StatusForbidden, "season_access_required", "No season access yet.")
		return
	}
	var request attemptInput
	if err := decodeJSON(r.Body, &request); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "validation_failed", err.Error())
		return
	}
	if err := validateAttempt(request); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "validation_failed", err.Error())
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
		SeasonID:    request.SeasonID,
		WeekID:      request.WeekID,
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
	if !actor.CanAccessProgramme() && !actor.IsDirectorOrSystemAdmin() {
		writeErrorResponse(w, http.StatusForbidden, "season_access_required", "No season access yet.")
		return
	}
	var request attemptInput
	if err := decodeJSON(r.Body, &request); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "validation_failed", err.Error())
		return
	}
	if err := validateAttempt(request); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "validation_failed", err.Error())
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
		SeasonID:    request.SeasonID,
		WeekID:      request.WeekID,
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
	if !actor.CanAccessProgramme() && !actor.IsDirectorOrSystemAdmin() {
		writeErrorResponse(w, http.StatusForbidden, "season_access_required", "No season access yet.")
		return
	}
	if err := a.db.DeleteAttempt(r.Context(), r.PathValue("id"), actor.UserID); err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
