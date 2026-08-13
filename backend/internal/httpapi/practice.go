package httpapi

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/magedmg/RSP-website/backend/internal/model"
	"github.com/magedmg/RSP-website/backend/internal/platform/cursor"
	"github.com/magedmg/RSP-website/backend/internal/platform/id"
	"github.com/magedmg/RSP-website/backend/internal/platform/sanitize"
	"github.com/magedmg/RSP-website/backend/internal/practice"
)

func (a *API) problems(w http.ResponseWriter, r *http.Request) {
	difficulty, category := r.URL.Query().Get("difficulty"), r.URL.Query().Get("category")
	binding := "problems|id:asc|difficulty=" + difficulty + "|category=" + category
	limit, after, err := a.page(r, binding)
	if err != nil {
		a.fail(w, r, 400, "invalid_cursor", "Invalid cursor", cursor.ErrInvalid.Error(), nil)
		return
	}
	items, more, total, err := a.store.ListProblems(r.Context(), after, limit)
	if err != nil {
		storeFailure(a, w, r, err)
		return
	}
	filtered := items[:0]
	for _, p := range items {
		if difficulty != "" && p.Difficulty != difficulty {
			continue
		}
		if category != "" && !hasCategory(p, category) {
			continue
		}
		filtered = append(filtered, p)
	}
	items = filtered
	var last string
	if len(items) > 0 {
		last = items[len(items)-1].ID
	}
	writeJSON(w, 200, model.Page[model.Problem]{Items: items, PageInfo: model.PageInfo{NextCursor: a.next(last, binding, more), HasMore: more}, TotalCount: total})
}
func (a *API) attempts(w http.ResponseWriter, r *http.Request) {
	actor := actorFrom(r.Context())
	binding := "attempts|id:asc|user=" + actor.UserID
	limit, after, err := a.page(r, binding)
	if err != nil {
		a.fail(w, r, 400, "invalid_cursor", "Invalid cursor", cursor.ErrInvalid.Error(), nil)
		return
	}
	items, more, total, err := a.store.ListAttempts(r.Context(), actor.UserID, after, limit)
	if err != nil {
		storeFailure(a, w, r, err)
		return
	}
	var last string
	if len(items) > 0 {
		last = items[len(items)-1].ID
	}
	writeJSON(w, 200, model.Page[model.Attempt]{Items: items, PageInfo: model.PageInfo{NextCursor: a.next(last, binding, more), HasMore: more}, TotalCount: total})
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
	if in.ProblemID == "" || in.Minutes <= 0 || in.AttemptedAt.IsZero() {
		return false
	}
	switch in.Outcome {
	case string(practice.Independent), string(practice.WithHints), string(practice.NotSolved), string(practice.Unknown):
	default:
		return false
	}
	return in.Confidence == nil || (*in.Confidence >= 1 && *in.Confidence <= 5)
}
func (a *API) createAttempt(w http.ResponseWriter, r *http.Request) {
	actor := actorFrom(r.Context())
	var in attemptInput
	if err := decodeJSON(w, r, &in); err != nil || !validateAttempt(in) {
		validation(a, w, r, "valid problem, outcome, confidence, duration, and date are required")
		return
	}
	v := model.Attempt{ID: id.New(), UserID: actor.UserID, ProblemID: in.ProblemID, Outcome: in.Outcome, Confidence: in.Confidence, Minutes: in.Minutes, Notes: sanitize.New().String(in.Notes), AttemptedAt: in.AttemptedAt.UTC(), SeasonID: in.SeasonID, WeekID: in.WeekID, Revision: 1}
	created, err := a.store.CreateAttempt(r.Context(), v)
	if err != nil {
		storeFailure(a, w, r, err)
		return
	}
	a.audit(r, "attempt.created", "problem_attempt", created.ID, nil)
	writeJSON(w, 201, created)
}
func (a *API) updateAttempt(w http.ResponseWriter, r *http.Request) {
	actor := actorFrom(r.Context())
	var in attemptInput
	if err := decodeJSON(w, r, &in); err != nil || !validateAttempt(in) || in.Revision < 1 {
		validation(a, w, r, "valid fields and revision are required")
		return
	}
	updated, err := a.store.UpdateAttempt(r.Context(), r.PathValue("id"), actor.UserID, in.Revision, func(v *model.Attempt) error {
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
	a.audit(r, "attempt.updated", "problem_attempt", updated.ID, nil)
	writeJSON(w, 200, updated)
}
func (a *API) deleteAttempt(w http.ResponseWriter, r *http.Request) {
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
	a.audit(r, "attempt.deleted", "problem_attempt", r.PathValue("id"), nil)
	w.WriteHeader(204)
}
func (a *API) recommendation(w http.ResponseWriter, r *http.Request) {
	actor := actorFrom(r.Context())
	a.mu.Lock()
	active := a.recommendations[actor.UserID]
	dismissals := append([]practice.Dismissal(nil), a.dismissals[actor.UserID]...)
	a.mu.Unlock()
	attemptRows, _, _, _ := a.store.ListAttempts(r.Context(), actor.UserID, "", 100)
	problemRows, _, _, _ := a.store.ListProblems(r.Context(), "", 100)
	attempts := make([]practice.Attempt, 0, len(attemptRows))
	problemByID := map[string]model.Problem{}
	for _, p := range problemRows {
		problemByID[p.ID] = p
	}
	for _, v := range attemptRows {
		p := problemByID[v.ProblemID]
		attempts = append(attempts, practice.Attempt{ProblemID: v.ProblemID, Difficulty: practice.Difficulty(p.Difficulty), Categories: p.Categories, Outcome: practice.Outcome(v.Outcome), Confidence: v.Confidence, Minutes: v.Minutes, AttemptedAt: v.AttemptedAt})
	}
	problems := make([]practice.Problem, 0, len(problemRows))
	for _, p := range problemRows {
		problems = append(problems, practice.Problem{ID: p.ID, Title: p.Title, Link: p.Link, Difficulty: practice.Difficulty(p.Difficulty), Categories: p.Categories, Premium: p.Premium})
	}
	var current *practice.Recommendation
	if active.ID != "" {
		current = &active
	}
	selected, err := practice.Select(practice.Request{UserID: actor.UserID, Level: practice.NonStudent, Problems: problems, Attempts: attempts, Dismissals: dismissals, Active: current, Now: time.Now()})
	if err != nil {
		a.fail(w, r, 404, "no_recommendation", "No recommendation available", "No suitable problem is currently available.", nil)
		return
	}
	a.mu.Lock()
	a.recommendations[actor.UserID] = selected
	a.mu.Unlock()
	writeJSON(w, 200, selected)
}
func (a *API) dismissRecommendation(w http.ResponseWriter, r *http.Request) {
	actor := actorFrom(r.Context())
	var in struct {
		Reason string `json:"reason"`
	}
	if err := decodeJSON(w, r, &in); err != nil {
		validation(a, w, r, err.Error())
		return
	}
	a.mu.Lock()
	current, ok := a.recommendations[actor.UserID]
	if ok && current.DismissedAt == nil {
		now := time.Now().UTC()
		current.DismissedAt = &now
		a.recommendations[actor.UserID] = current
		a.dismissals[actor.UserID] = append(a.dismissals[actor.UserID], practice.Dismissal{ProblemID: current.Problem.ID, DismissedAt: now})
	}
	a.mu.Unlock()
	if !ok {
		a.fail(w, r, 404, "no_recommendation", "No recommendation available", "There is no active recommendation.", nil)
		return
	}
	a.audit(r, "recommendation.dismissed", "recommendation", current.ID, map[string]any{"reason": strings.TrimSpace(in.Reason)})
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
