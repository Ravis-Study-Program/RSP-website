package httpapi

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/magedmg/RSP-website/backend/internal/authz"
	"github.com/magedmg/RSP-website/backend/internal/platform/cursor"
	"github.com/magedmg/RSP-website/backend/internal/platform/id"
	"github.com/magedmg/RSP-website/backend/internal/platform/sanitize"
	"github.com/magedmg/RSP-website/backend/internal/practice"
	"github.com/magedmg/RSP-website/backend/internal/programme"
	"github.com/magedmg/RSP-website/backend/internal/store"
)

func (a *API) requirePracticeAccess(w http.ResponseWriter, r *http.Request) bool {
	actor := actorFrom(r.Context())
	if actor.ProgrammeAccess() || actor.IsPrivileged() {
		return true
	}
	writeErrorResponse(w, r, http.StatusForbidden, "season_access_required", "Season access required", "No season access yet.")
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
			validation(a, w, r, "premium must be true or false")
			return
		}
		premium = &parsed
	}
	if _, err := requestedSort(r, "id:asc", "id:asc"); err != nil {
		validation(a, w, r, "sort must be id:asc")
		return
	}

	direction := r.URL.Query().Get("direction")
	if direction == "" {
		direction = "forward"
	}
	if direction != "forward" && direction != "backward" {
		validation(a, w, r, "direction must be forward or backward")
		return
	}

	binding := "problems|id:asc|difficulty=" + difficulty + "|category=" + category + "|premium=" + r.URL.Query().Get("premium")
	limit, after, err := a.page(r, binding)
	if err != nil {
		writeErrorResponse(w, r, 400, "invalid_cursor", "Invalid cursor", cursor.ErrInvalid.Error())
		return
	}

	items, more, total, err := a.store.ListProblems(r.Context(), after, limit, difficulty, category, premium, direction)
	if err != nil {
		storeFailure(a, w, r, err)
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
		writeErrorResponse(w, r, http.StatusForbidden, "directory_access_required", "Directory access required", "Only active members and student alumni may view another member's public problem history.")
		return
	}
	if targetID != actor.UserID && !actor.IsPrivileged() {
		participant, err := a.store.GetMockParticipant(r.Context(), targetID)
		if err != nil || !participant.Eligible() {
			writeErrorResponse(w, r, 404, "not_found", "Not found", "The requested member does not exist.")
			return
		}
	}

	outcome, difficulty := r.URL.Query().Get("outcome"), r.URL.Query().Get("difficulty")
	if _, err := requestedSort(r, "id:asc", "id:asc"); err != nil {
		validation(a, w, r, "sort must be id:asc")
		return
	}

	direction := r.URL.Query().Get("direction")
	if direction == "" {
		direction = "forward"
	}
	if direction != "forward" && direction != "backward" {
		validation(a, w, r, "direction must be forward or backward")
		return
	}

	binding := "attempts|id:asc|user=" + targetID + "|outcome=" + outcome + "|difficulty=" + difficulty
	limit, after, err := a.page(r, binding)
	if err != nil {
		writeErrorResponse(w, r, 400, "invalid_cursor", "Invalid cursor", cursor.ErrInvalid.Error())
		return
	}

	items, more, total, err := a.store.ListAttempts(r.Context(), targetID, after, limit, outcome, difficulty, direction)
	if err != nil {
		storeFailure(a, w, r, err)
		return
	}

	if targetID != actor.UserID && !a.auditSystemAdminPrivateRead(w, r, "problem_attempt_history", targetID) {
		return
	}
	if targetID != actor.UserID && !actor.IsPrivileged() {
		canPrivate, err := a.canViewMemberPrivate(r, targetID)
		if err != nil {
			storeFailure(a, w, r, err)
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
		validation(a, w, r, "valid problem, outcome, confidence, duration, and date are required")
		return
	}

	v := practice.AttemptRecord{ID: id.New(), UserID: actor.UserID, ProblemID: in.ProblemID, Outcome: in.Outcome, Confidence: in.Confidence, Minutes: in.Minutes, Notes: sanitize.New().String(in.Notes), AttemptedAt: in.AttemptedAt.UTC(), SeasonID: in.SeasonID, WeekID: in.WeekID, Revision: 1}
	created, fulfilled, err := a.store.CreateAttempt(r.Context(), v)
	if err != nil {
		storeFailure(a, w, r, err)
		return
	}

	if fulfilled {
		a.telemetry.observeRecommendation("attempted")
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
		validation(a, w, r, "valid fields and revision are required")
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
		storeFailure(a, w, r, err)
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
		validation(a, w, r, "revision query parameter is required")
		return
	}
	if err := a.store.DeleteAttempt(r.Context(), r.PathValue("id"), actor.UserID, revision); err != nil {
		storeFailure(a, w, r, err)
		return
	}

	w.WriteHeader(204)
}

func (a *API) recommendation(w http.ResponseWriter, r *http.Request) {
	if !a.requirePracticeAccess(w, r) {
		return
	}
	actor := actorFrom(r.Context())
	current, err := a.store.GetActiveRecommendation(r.Context(), actor.UserID)
	if err != nil {
		storeFailure(a, w, r, err)
		return
	}
	if current != nil {
		a.telemetry.observeRecommendation("reused")
		writeJSONResponse(w, http.StatusOK, current)
		return
	}

	dismissals, err := a.store.ListRecommendationDismissals(r.Context(), actor.UserID)
	if err != nil {
		storeFailure(a, w, r, err)
		return
	}

	settings, err := a.store.GetPracticeSettings(r.Context(), actor.UserID)
	if err != nil {
		storeFailure(a, w, r, err)
		return
	}

	goals := practice.DefaultGoals()
	if settings.GoalsEnabled {
		goals = practice.Goals{practice.Easy: settings.EasyMinutes, practice.Medium: settings.MediumMinutes, practice.Hard: settings.HardMinutes}
	}
	snapshot, err := a.store.RecommendationSnapshot(r.Context(), actor.UserID, goals)
	if err != nil {
		storeFailure(a, w, r, err)
		return
	}

	attempts := make([]practice.Attempt, 0, len(snapshot.QualityAttempts))
	problemByID := map[string]practice.ProblemRecord{}
	for _, p := range snapshot.Problems {
		problemByID[p.ID] = p
	}
	for _, v := range snapshot.QualityAttempts {
		p := problemByID[v.ProblemID]
		attempts = append(attempts, practice.Attempt{ProblemID: v.ProblemID, Difficulty: practice.Difficulty(p.Difficulty), Categories: p.Categories, Outcome: practice.Outcome(v.Outcome), Confidence: v.Confidence, Minutes: v.Minutes, AttemptedAt: v.AttemptedAt, Migrated: v.Migrated})
	}
	level := practice.NonStudent
	enrollments, err := a.store.ListEnrollmentsForUser(r.Context(), actor.UserID)
	if err != nil {
		storeFailure(a, w, r, err)
		return
	}

	for _, enrollment := range enrollments {
		if enrollment.Role == "student" && enrollment.State == "active" {
			switch enrollment.StudentLevel {
			case "novice":
				level = practice.Novice
			case "beginner":
				level = practice.Beginner
			case "intermediate":
				level = practice.Intermediate
			case "advanced":
				level = practice.Advanced
			}
			break
		}
	}

	now := time.Now().UTC()
	criteria := practice.CriteriaFor(practice.Request{Level: level, QualityAttempts: attempts, CategoryExposure: snapshot.CategoryExposure, Goals: goals})
	candidateModels, problemHistory, err := a.store.RecommendationCandidates(r.Context(), actor.UserID, criteria, settings.PremiumOptIn, goals, now)
	if err != nil {
		storeFailure(a, w, r, err)
		return
	}

	problems := make([]practice.Problem, 0, len(candidateModels))
	for _, p := range candidateModels {
		problems = append(problems, practice.Problem{ID: p.ID, Number: p.Number, Title: p.Title, Link: p.Link, Difficulty: practice.Difficulty(p.Difficulty), Categories: p.Categories, Premium: p.Premium, Revision: p.Revision})
	}
	selected, err := practice.Select(practice.Request{UserID: actor.UserID, Level: level, PremiumOptIn: settings.PremiumOptIn, Problems: problems, QualityAttempts: attempts, ProblemHistory: problemHistory, CategoryExposure: snapshot.CategoryExposure, Dismissals: dismissals, Goals: goals, Now: now})
	if err != nil {
		a.telemetry.observeRecommendation("unavailable")
		writeErrorResponse(w, r, 404, "no_recommendation", "No recommendation available", "No suitable problem is currently available.")
		return
	}

	selected.ID = id.New()
	selected, err = a.store.SaveRecommendation(r.Context(), selected)
	if err != nil {
		storeFailure(a, w, r, err)
		return
	}

	a.telemetry.observeRecommendation("generated")
	writeJSONResponse(w, http.StatusOK, selected)
}

func (a *API) dismissRecommendation(w http.ResponseWriter, r *http.Request) {
	if !a.requirePracticeAccess(w, r) {
		return
	}

	actor := actorFrom(r.Context())
	var in struct {
		Reason   string `json:"reason"`
		Revision int64  `json:"revision"`
	}
	if r.Body != nil && r.ContentLength != 0 {
		body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
		if err != nil {
			validation(a, w, r, err.Error())
			return
		}
		if len(bytes.TrimSpace(body)) > 0 {
			r.Body = io.NopCloser(bytes.NewReader(body))
			if err := decodeJSON(w, r, &in); err != nil {
				validation(a, w, r, err.Error())
				return
			}
		}
	}
	if in.Revision < 1 {
		validation(a, w, r, "revision is required")
		return
	}

	_, err := a.store.DismissRecommendation(r.Context(), actor.UserID, in.Revision, strings.TrimSpace(in.Reason), time.Now(), id.New())
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeErrorResponse(w, r, 404, "no_recommendation", "No recommendation available", "There is no active recommendation.")
		} else {
			storeFailure(a, w, r, err)
		}
		return
	}

	a.telemetry.observeRecommendation("dismissed")
	w.WriteHeader(204)
}

func parseRevision(r *http.Request) (int64, error) {
	var v int64
	_, err := fmt.Sscan(r.URL.Query().Get("revision"), &v)
	if v < 1 {
		return 0, fmt.Errorf("invalid revision")
	}
	return v, err
}
