package api

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/magedmg/RSP-website/backend/internal/authz"
	"github.com/magedmg/RSP-website/backend/internal/dal"
	"github.com/magedmg/RSP-website/backend/internal/platform/cursor"
	"github.com/magedmg/RSP-website/backend/internal/platform/id"
	"github.com/magedmg/RSP-website/backend/internal/platform/sanitize"
	"github.com/magedmg/RSP-website/backend/internal/practice"
	"github.com/magedmg/RSP-website/backend/internal/programme"
)

func (a *API) listLeetCodeProblems(w http.ResponseWriter, r *http.Request) {
	actor := actorFrom(r.Context())
	if !actor.CanAccessProgramme() && !actor.IsDirectorOrSystemAdmin() {
		writeErrorResponse(w, http.StatusForbidden, "season_access_required", "No season access yet.")
		return
	}

	difficulty, category := r.URL.Query().Get("difficulty"), r.URL.Query().Get("category")
	var premium *bool
	if raw := r.URL.Query().Get("premium"); raw != "" {
		parsed, parseErr := strconv.ParseBool(raw)
		if parseErr != nil {
			writeErrorResponse(w, http.StatusBadRequest, "validation_failed", "premium must be true or false")
			return
		}
		premium = &parsed
	}
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

	binding := "problems|id:asc|difficulty=" + difficulty + "|category=" + category + "|premium=" + r.URL.Query().Get("premium")
	limit, after, err := parsePagination(r.URL.Query().Get("limit"), r.URL.Query().Get("cursor"), binding, a.cursorSecret)
	if err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "invalid_cursor", cursor.ErrInvalid.Error())
		return
	}

	items, more, total, err := a.db.ListProblems(r.Context(), dal.ProblemQuery{
		Boundary:   after,
		Limit:      limit,
		Difficulty: difficulty,
		Category:   category,
		Premium:    premium,
		Direction:  direction,
	})
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}

	pageInfo := pageInfoForKeyset(
		a.cursorSecret, binding, direction, after, items, more,
		func(record practice.ProblemRecord) string { return record.ID },
	)
	response := Page[practice.ProblemRecord]{
		Items:      items,
		PageInfo:   pageInfo,
		TotalCount: total,
	}
	writeJSONResponse(w, http.StatusOK, response)
}

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
		if err != nil || !participant.CanBeMockInterviewParticipant() {
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

	pageInfo := pageInfoForKeyset(
		a.cursorSecret, binding, direction, after, items, more,
		func(attempt practice.AttemptRecord) string { return attempt.ID },
	)
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

func validateAttempt(request attemptInput) bool {
	if request.ProblemID == "" || request.Minutes <= 0 || request.AttemptedAt.IsZero() || (request.WeekID != nil && request.SeasonID == nil) {
		return false
	}
	switch request.Outcome {
	case string(practice.Independent), string(practice.WithHints), string(practice.NotSolved):
	default:
		return false
	}
	return request.Confidence == nil || (*request.Confidence >= 1 && *request.Confidence <= 5)
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
	if !validateAttempt(request) {
		writeErrorResponse(w, http.StatusBadRequest, "validation_failed", "valid problem, outcome, confidence, duration, and date are required")
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
	if !validateAttempt(request) {
		writeErrorResponse(w, http.StatusBadRequest, "validation_failed", "valid fields are required")
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
