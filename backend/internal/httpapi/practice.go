package httpapi

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/magedmg/RSP-website/backend/internal/authz"
	"github.com/magedmg/RSP-website/backend/internal/platform/cursor"
	"github.com/magedmg/RSP-website/backend/internal/platform/id"
	"github.com/magedmg/RSP-website/backend/internal/platform/sanitize"
	"github.com/magedmg/RSP-website/backend/internal/practice"
	"github.com/magedmg/RSP-website/backend/internal/programme"
)

func (a *API) requirePracticeAccess(w http.ResponseWriter, r *http.Request) bool {
	actor := actorFrom(r.Context())
	if actor.ProgrammeAccess() || actor.IsPrivileged() {
		return true
	}
	writeErrorResponse(w, http.StatusForbidden, "season_access_required", "No season access yet.")
	return false
}

func (a *API) canViewMemberPrivate(r *http.Request, targetID string) (bool, error) {
	actor := actorFrom(r.Context())
	if actor.UserID == targetID || actor.IsPrivileged() {
		return true, nil
	}

	var targetEnrollments []programme.EnrollmentRecord
	targetLoaded := false
	for _, enrollment := range actor.Enrollments {
		if enrollment.State != authz.Active && enrollment.State != authz.Completed {
			continue
		}
		if enrollment.Role == authz.Coordinator {
			if !targetLoaded {
				var err error
				targetEnrollments, err = a.store.ListEnrollmentsForUser(r.Context(), targetID)
				if err != nil {
					return false, err
				}

				targetLoaded = true
			}
			for _, target := range targetEnrollments {
				if target.SeasonID == enrollment.SeasonID && (target.State == "active" || target.State == "completed") {
					return true, nil
				}
			}
		}
		if enrollment.Role == authz.Mentor {
			assigned, err := a.store.IsMentorAssigned(r.Context(), enrollment.SeasonID, actor.UserID, targetID)
			if err != nil {
				return false, err
			}
			if assigned {
				return true, nil
			}
		}
	}

	return false, nil
}

func (a *API) problems(w http.ResponseWriter, r *http.Request) {
	if !a.requirePracticeAccess(w, r) {
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
	if _, err := requestedSort(r, "id:asc", "id:asc"); err != nil {
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
	limit, after, err := a.page(r, binding)
	if err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "invalid_cursor", cursor.ErrInvalid.Error())
		return
	}

	items, more, total, err := a.store.ListProblems(r.Context(), after, limit, difficulty, category, premium, direction)
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}

	pageInfo := pageInfoForKeyset(
		a, binding, direction, after, items, more,
		func(v practice.ProblemRecord) string { return v.ID },
	)
	response := Page[practice.ProblemRecord]{
		Items:      items,
		PageInfo:   pageInfo,
		TotalCount: total,
	}
	writeJSONResponse(w, http.StatusOK, response)
}

func (a *API) attempts(w http.ResponseWriter, r *http.Request) {
	if !a.requirePracticeAccess(w, r) {
		return
	}

	actor := actorFrom(r.Context())
	targetID := r.URL.Query().Get("userId")
	if targetID == "" {
		targetID = actor.UserID
	}
	if targetID != actor.UserID && !actor.EligibleMember() && !actor.IsPrivileged() {
		writeErrorResponse(w, http.StatusForbidden, "directory_access_required", "Only active members and student alumni may view another member's public problem history.")
		return
	}
	if targetID != actor.UserID && !actor.IsPrivileged() {
		participant, err := a.store.GetMockParticipant(r.Context(), targetID)
		if err != nil || !participant.Eligible() {
			writeErrorResponse(w, http.StatusNotFound, "not_found", "The requested member does not exist.")
			return
		}
	}

	outcome, difficulty := r.URL.Query().Get("outcome"), r.URL.Query().Get("difficulty")
	if _, err := requestedSort(r, "id:asc", "id:asc"); err != nil {
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
	limit, after, err := a.page(r, binding)
	if err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "invalid_cursor", cursor.ErrInvalid.Error())
		return
	}

	items, more, total, err := a.store.ListAttempts(r.Context(), targetID, after, limit, outcome, difficulty, direction)
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}

	if targetID != actor.UserID && !a.auditSystemAdminPrivateRead(w, r, "problem_attempt_history", targetID) {
		return
	}
	if targetID != actor.UserID && !actor.IsPrivileged() {
		canPrivate, err := a.canViewMemberPrivate(r, targetID)
		if err != nil {
			a.writeStoreErrorResponse(w, err)
			return
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
		a, binding, direction, after, items, more,
		func(v practice.AttemptRecord) string { return v.ID },
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
	Revision                  int64
}

func validateAttempt(in attemptInput) bool {
	if in.ProblemID == "" || in.Minutes <= 0 || in.AttemptedAt.IsZero() || (in.WeekID != nil && in.SeasonID == nil) {
		return false
	}
	switch in.Outcome {
	case string(practice.Independent), string(practice.WithHints), string(practice.NotSolved):
	default:
		return false
	}
	return in.Confidence == nil || (*in.Confidence >= 1 && *in.Confidence <= 5)
}

func (a *API) createAttempt(w http.ResponseWriter, r *http.Request) {
	if !a.requirePracticeAccess(w, r) {
		return
	}
	actor := actorFrom(r.Context())
	var in attemptInput
	if err := decodeJSON(w, r, &in); err != nil || !validateAttempt(in) {
		writeErrorResponse(w, http.StatusBadRequest, "validation_failed", "valid problem, outcome, confidence, duration, and date are required")
		return
	}

	v := practice.AttemptRecord{ID: id.New(), UserID: actor.UserID, ProblemID: in.ProblemID, Outcome: in.Outcome, Confidence: in.Confidence, Minutes: in.Minutes, Notes: sanitize.New().String(in.Notes), AttemptedAt: in.AttemptedAt.UTC(), SeasonID: in.SeasonID, WeekID: in.WeekID, Revision: 1}
	created, err := a.store.CreateAttempt(r.Context(), v)
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}

	writeJSONResponse(w, http.StatusCreated, created)
}

func (a *API) updateAttempt(w http.ResponseWriter, r *http.Request) {
	if !a.requirePracticeAccess(w, r) {
		return
	}

	actor := actorFrom(r.Context())
	var in attemptInput
	if err := decodeJSON(w, r, &in); err != nil || !validateAttempt(in) || in.Revision < 1 {
		writeErrorResponse(w, http.StatusBadRequest, "validation_failed", "valid fields and revision are required")
		return
	}

	updated, err := a.store.UpdateAttempt(r.Context(), r.PathValue("id"), actor.UserID, in.Revision, func(v *practice.AttemptRecord) error {
		v.ProblemID = in.ProblemID
		v.Outcome = in.Outcome
		v.Confidence = in.Confidence
		v.Minutes = in.Minutes
		v.Notes = sanitize.New().String(in.Notes)
		v.AttemptedAt = in.AttemptedAt.UTC()
		v.SeasonID = in.SeasonID
		v.WeekID = in.WeekID
		return nil
	})
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}

	writeJSONResponse(w, http.StatusOK, updated)
}

func (a *API) deleteAttempt(w http.ResponseWriter, r *http.Request) {
	if !a.requirePracticeAccess(w, r) {
		return
	}
	actor := actorFrom(r.Context())
	revision, err := parseRevision(r)
	if err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "validation_failed", "revision query parameter is required")
		return
	}
	if err := a.store.DeleteAttempt(r.Context(), r.PathValue("id"), actor.UserID, revision); err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func parseRevision(r *http.Request) (int64, error) {
	var v int64
	_, err := fmt.Sscan(r.URL.Query().Get("revision"), &v)
	if v < 1 {
		return 0, fmt.Errorf("invalid revision")
	}
	return v, err
}
