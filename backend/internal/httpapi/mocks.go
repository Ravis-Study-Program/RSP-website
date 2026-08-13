package httpapi

import (
	"net/http"
	"sort"
	"time"

	"github.com/magedmg/RSP-website/backend/internal/mockinterviews"
)

func (a *API) listMocks(w http.ResponseWriter, r *http.Request) {
	actor := actorFrom(r.Context())
	mode := r.URL.Query().Get("mode")
	if mode == "" {
		mode = "received"
	}
	a.mu.Lock()
	items := []mockinterviews.Interview{}
	for _, v := range a.mocks {
		if v.DeletedAt != nil {
			continue
		}
		if mode == "received" && v.IntervieweeID != actor.UserID {
			continue
		}
		if mode == "given" && v.InterviewerID != actor.UserID {
			continue
		}
		if mode == "all" && v.InterviewerID != actor.UserID && v.IntervieweeID != actor.UserID && !actor.IsPrivileged() {
			continue
		}
		items = append(items, v)
	}
	a.mu.Unlock()
	sort.Slice(items, func(i, j int) bool { return items[i].OccurredAt.After(items[j].OccurredAt) })
	writeJSON(w, 200, map[string]any{"items": items, "pageInfo": map[string]bool{"hasMore": false}, "totalCount": len(items)})
}
func (a *API) createMock(w http.ResponseWriter, r *http.Request) {
	actor := actorFrom(r.Context())
	var in struct {
		Interviewee     mockinterviews.Participant `json:"interviewee"`
		SeasonID        *string                    `json:"seasonId"`
		OccurredAt      time.Time                  `json:"occurredAt"`
		DurationMinutes int                        `json:"durationMinutes"`
		Notes           string                     `json:"notes"`
		Rounds          []mockinterviews.Round     `json:"rounds"`
	}
	if err := decodeJSON(w, r, &in); err != nil {
		validation(a, w, r, err.Error())
		return
	}
	v, err := a.mockService.Create(actor.UserID, mockinterviews.CreateInput{Interviewee: in.Interviewee, SeasonID: in.SeasonID, OccurredAt: in.OccurredAt, DurationMinutes: in.DurationMinutes, Notes: in.Notes, Rounds: in.Rounds}, time.Now())
	if err != nil {
		a.fail(w, r, 400, "invalid_mock_interview", "Invalid mock interview", err.Error(), nil)
		return
	}
	a.mu.Lock()
	a.mocks[v.ID] = v
	a.mu.Unlock()
	a.audit(r, "mock_interview.created", "mock_interview", v.ID, nil)
	writeJSON(w, 201, withPass(v))
}
func (a *API) updateMock(w http.ResponseWriter, r *http.Request) {
	actor := actorFrom(r.Context())
	var in struct {
		Revision        int64                  `json:"revision"`
		OccurredAt      time.Time              `json:"occurredAt"`
		DurationMinutes int                    `json:"durationMinutes"`
		Notes           string                 `json:"notes"`
		Rounds          []mockinterviews.Round `json:"rounds"`
	}
	if err := decodeJSON(w, r, &in); err != nil {
		validation(a, w, r, err.Error())
		return
	}
	a.mu.Lock()
	v, ok := a.mocks[r.PathValue("id")]
	if !ok {
		a.mu.Unlock()
		a.fail(w, r, 404, "not_found", "Not found", "The mock interview does not exist.", nil)
		return
	}
	err := a.mockService.Update(&v, actor.UserID, mockinterviews.UpdateInput{ExpectedRevision: in.Revision, OccurredAt: in.OccurredAt, DurationMinutes: in.DurationMinutes, Notes: in.Notes, Rounds: in.Rounds}, time.Now())
	if err == nil {
		a.mocks[v.ID] = v
	}
	a.mu.Unlock()
	if err != nil {
		mockFailure(a, w, r, err)
		return
	}
	a.audit(r, "mock_interview.updated", "mock_interview", v.ID, nil)
	writeJSON(w, 200, withPass(v))
}
func (a *API) deleteMock(w http.ResponseWriter, r *http.Request) {
	actor := actorFrom(r.Context())
	revision, err := parseRevision(r)
	if err != nil {
		validation(a, w, r, "revision query parameter is required")
		return
	}
	a.mu.Lock()
	v, ok := a.mocks[r.PathValue("id")]
	if !ok {
		a.mu.Unlock()
		a.fail(w, r, 404, "not_found", "Not found", "The mock interview does not exist.", nil)
		return
	}
	err = a.mockService.Delete(&v, actor.UserID, revision, time.Now())
	if err == nil {
		a.mocks[v.ID] = v
	}
	a.mu.Unlock()
	if err != nil {
		mockFailure(a, w, r, err)
		return
	}
	a.audit(r, "mock_interview.deleted", "mock_interview", v.ID, nil)
	w.WriteHeader(204)
}
func (a *API) reviewRound(w http.ResponseWriter, r *http.Request) {
	actor := actorFrom(r.Context())
	var in struct {
		Reviewed bool   `json:"reviewed"`
		Comment  string `json:"comment"`
		Revision int64  `json:"revision"`
	}
	if err := decodeJSON(w, r, &in); err != nil {
		validation(a, w, r, err.Error())
		return
	}
	a.mu.Lock()
	v, ok := a.mocks[r.PathValue("id")]
	if !ok {
		a.mu.Unlock()
		a.fail(w, r, 404, "not_found", "Not found", "The mock interview does not exist.", nil)
		return
	}
	err := a.mockService.Review(&v, actor.UserID, r.PathValue("roundId"), in.Comment, in.Reviewed, in.Revision, time.Now())
	if err == nil {
		a.mocks[v.ID] = v
	}
	a.mu.Unlock()
	if err != nil {
		mockFailure(a, w, r, err)
		return
	}
	a.audit(r, "mock_interview.reviewed", "mock_interview", v.ID, nil)
	writeJSON(w, 200, withPass(v))
}
func mockFailure(a *API, w http.ResponseWriter, r *http.Request, err error) {
	switch err {
	case mockinterviews.ErrForbidden:
		a.fail(w, r, 403, "forbidden", "Forbidden", "The current account cannot perform this action.", nil)
	case mockinterviews.ErrConflict:
		a.fail(w, r, 409, "stale_revision", "Conflict", "The mock interview changed since it was loaded.", nil)
	default:
		a.fail(w, r, 400, "invalid_mock_interview", "Invalid mock interview", err.Error(), nil)
	}
}
func withPass(v mockinterviews.Interview) map[string]any {
	return map[string]any{"interview": v, "passed": mockinterviews.Passed(v)}
}
func (a *API) adminSync(w http.ResponseWriter, r *http.Request) {
	actor := actorFrom(r.Context())
	if !actor.IsPrivileged() || !actor.HasRecentMFA(time.Now()) {
		a.fail(w, r, 403, "privileged_mfa_required", "Privileged access required", "Recent MFA is required.", nil)
		return
	}
	if a.syncLeetCode != nil {
		if err := a.syncLeetCode(); err != nil {
			a.fail(w, r, 503, "sync_failed", "Sync failed", err.Error(), nil)
			return
		}
	}
	a.audit(r, "leetcode.sync_requested", "worker", "leetcode_sync", nil)
	writeJSON(w, 202, map[string]string{"status": "accepted"})
}
