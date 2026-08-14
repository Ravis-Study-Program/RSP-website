package httpapi

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/magedmg/RSP-website/backend/internal/authz"
	"github.com/magedmg/RSP-website/backend/internal/mockinterviews"
	"github.com/magedmg/RSP-website/backend/internal/model"
	"github.com/magedmg/RSP-website/backend/internal/platform/id"
)

func (a *API) eligibleMockActor(r *http.Request) bool {
	actor := actorFrom(r.Context())
	participant, err := a.store.GetMockParticipant(r.Context(), actor.UserID)
	return err == nil && participant.ProgrammeAccessEligible()
}

func (a *API) listMockParticipants(w http.ResponseWriter, r *http.Request) {
	actor := actorFrom(r.Context())
	if !a.eligibleMockActor(r) && !actor.IsPrivileged() {
		a.fail(w, r, http.StatusForbidden, "mock_participant_required", "Mock interview access required", "Only programme members may select eligible mock-interview participants.", nil)
		return
	}
	if _, err := requestedSort(r, "id:asc", "id:asc"); err != nil {
		validation(a, w, r, "sort must be id:asc")
		return
	}
	direction, err := requestedDirection(r)
	if err != nil {
		validation(a, w, r, "direction must be forward or backward")
		return
	}
	query := strings.TrimSpace(r.URL.Query().Get("query"))
	if len(query) > 100 {
		validation(a, w, r, "query must be at most 100 characters")
		return
	}
	binding := "mock-interview-participants|sort=id:asc|query=" + query
	limit, boundary, err := a.page(r, binding)
	if err != nil {
		a.fail(w, r, http.StatusBadRequest, "invalid_cursor", "Invalid cursor", "The cursor does not match the selected participant filters and sort.", nil)
		return
	}
	users, more, total, err := a.store.ListUsers(r.Context(), boundary, limit, direction, query, "", "")
	if err != nil {
		storeFailure(a, w, r, err)
		return
	}
	items := make([]mockinterviews.ParticipantSummary, 0, len(users))
	for _, user := range users {
		items = append(items, mockinterviews.ParticipantSummary{ID: user.ID, Slug: user.Slug, Name: user.Name, AvatarURL: user.AvatarURL})
	}
	writeJSON(w, http.StatusOK, model.Page[mockinterviews.ParticipantSummary]{
		Items:      items,
		PageInfo:   pageInfoForKeyset(a, binding, direction, boundary, items, more, func(v mockinterviews.ParticipantSummary) string { return v.ID }),
		TotalCount: total,
	})
}

func (a *API) listMocks(w http.ResponseWriter, r *http.Request) {
	actor := actorFrom(r.Context())
	if !a.eligibleMockActor(r) && !actor.IsPrivileged() && !historicalMockReviewer(actor) {
		a.fail(w, r, 403, "mock_participant_required", "Mock interview access required", "Only active members and alumni may access mock interviews.", nil)
		return
	}
	mode := r.URL.Query().Get("mode")
	if mode == "" {
		mode = "received"
	}
	if mode != "received" && mode != "given" && mode != "all" {
		validation(a, w, r, "mode must be received, given, or all")
		return
	}
	if mode == "all" && !a.auditSystemAdminPrivateRead(w, r, "mock_interview_collection", actor.UserID) {
		return
	}
	sortBy, err := requestedSort(r, "occurredAt:desc", "occurredAt:desc", "id:asc")
	if err != nil {
		validation(a, w, r, "sort must be occurredAt:desc or id:asc")
		return
	}
	direction, err := requestedDirection(r)
	if err != nil {
		validation(a, w, r, "direction must be forward or backward")
		return
	}
	binding := "mock-interviews|mode=" + mode + "|sort=" + sortBy
	limit, boundary, err := a.page(r, binding)
	if err != nil {
		a.fail(w, r, 400, "invalid_cursor", "Invalid cursor", "The cursor does not match the selected mode and sort.", nil)
		return
	}
	items, more, total, err := a.store.ListMockInterviews(r.Context(), actor, mode, boundary, limit, sortBy, direction)
	if err != nil {
		storeFailure(a, w, r, err)
		return
	}
	for index := range items {
		items[index] = a.withMockParticipantSummaries(r, items[index])
	}
	writeJSON(w, 200, model.Page[mockinterviews.Interview]{Items: items, PageInfo: pageInfoForKeyset(a, binding, direction, boundary, items, more, func(v mockinterviews.Interview) string { return v.ID }), TotalCount: total})
}

func historicalMockReviewer(actor authz.Actor) bool {
	for _, enrollment := range actor.Enrollments {
		if enrollment.State == authz.Completed && (enrollment.Role == authz.Mentor || enrollment.Role == authz.Coordinator) {
			return true
		}
	}
	return false
}

func (a *API) createMock(w http.ResponseWriter, r *http.Request) {
	actor := actorFrom(r.Context())
	if !a.eligibleMockActor(r) {
		a.fail(w, r, 403, "mock_participant_required", "Mock interview access required", "The interviewer must be an active member or alumnus.", nil)
		return
	}
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
	// Every eligibility flag is derived from app state; request booleans are
	// ignored so a client cannot make an unaffiliated identity eligible.
	interviewee, err := a.store.GetMockParticipant(r.Context(), in.Interviewee.UserID)
	if err != nil {
		storeFailure(a, w, r, err)
		return
	}
	if !a.validMockSeason(w, r, in.SeasonID, in.OccurredAt, actor.UserID, interviewee.UserID) {
		return
	}
	service := mockinterviews.Service{Sanitize: a.mockService.Sanitize}
	now := time.Now().UTC()
	v, err := service.Create(actor.UserID, mockinterviews.CreateInput{Interviewee: interviewee, SeasonID: in.SeasonID, OccurredAt: in.OccurredAt, DurationMinutes: in.DurationMinutes, Notes: in.Notes, Rounds: in.Rounds}, now)
	if err != nil {
		mockFailure(a, w, r, err)
		return
	}
	v.ID = id.New()
	v, err = a.store.CreateMockInterview(r.Context(), v, actor.UserID, now)
	if err != nil {
		storeFailure(a, w, r, err)
		return
	}
	writeJSON(w, 201, withPass(a.withMockParticipantSummaries(r, v)))
}

func (a *API) updateMock(w http.ResponseWriter, r *http.Request) {
	actor := actorFrom(r.Context())
	if !a.eligibleMockActor(r) && !actor.IsPrivileged() {
		a.fail(w, r, 403, "mock_participant_required", "Mock interview access required", "Only active members and alumni may update mock interviews.", nil)
		return
	}
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
	v, err := a.store.GetMockInterview(r.Context(), r.PathValue("id"))
	if err != nil {
		storeFailure(a, w, r, err)
		return
	}
	if actor.UserID != v.InterviewerID {
		a.fail(w, r, 404, "not_found", "Not found", "The requested resource does not exist.", nil)
		return
	}
	if !a.validMockSeason(w, r, v.SeasonID, in.OccurredAt, v.InterviewerID, v.IntervieweeID) {
		return
	}
	if !a.mockSeasonWritable(w, r, v.SeasonID) {
		return
	}
	service := mockinterviews.Service{Sanitize: a.mockService.Sanitize}
	now := time.Now().UTC()
	if err := service.Update(&v, actor.UserID, mockinterviews.UpdateInput{ExpectedRevision: in.Revision, OccurredAt: in.OccurredAt, DurationMinutes: in.DurationMinutes, Notes: in.Notes, Rounds: in.Rounds}, now); err != nil {
		mockFailure(a, w, r, err)
		return
	}
	v, err = a.store.UpdateMockInterview(r.Context(), v, actor.UserID, "updated", now)
	if err != nil {
		storeFailure(a, w, r, err)
		return
	}
	writeJSON(w, 200, withPass(a.withMockParticipantSummaries(r, v)))
}

func (a *API) deleteMock(w http.ResponseWriter, r *http.Request) {
	actor := actorFrom(r.Context())
	if !a.eligibleMockActor(r) && !actor.IsPrivileged() {
		a.fail(w, r, 403, "mock_participant_required", "Mock interview access required", "Only active members and alumni may delete mock interviews.", nil)
		return
	}
	revision, err := parseRevision(r)
	if err != nil {
		validation(a, w, r, "revision query parameter is required")
		return
	}
	v, err := a.store.GetMockInterview(r.Context(), r.PathValue("id"))
	if err != nil {
		storeFailure(a, w, r, err)
		return
	}
	if actor.UserID != v.InterviewerID {
		a.fail(w, r, 404, "not_found", "Not found", "The requested resource does not exist.", nil)
		return
	}
	if !a.mockSeasonWritable(w, r, v.SeasonID) {
		return
	}
	service := mockinterviews.Service{Sanitize: a.mockService.Sanitize}
	now := time.Now().UTC()
	if err := service.Delete(&v, actor.UserID, revision, now); err != nil {
		mockFailure(a, w, r, err)
		return
	}
	if _, err := a.store.UpdateMockInterview(r.Context(), v, actor.UserID, "soft deleted", now); err != nil {
		storeFailure(a, w, r, err)
		return
	}
	w.WriteHeader(204)
}

func (a *API) reviewRound(w http.ResponseWriter, r *http.Request) {
	actor := actorFrom(r.Context())
	if !a.eligibleMockActor(r) && !actor.IsPrivileged() {
		a.fail(w, r, 403, "mock_participant_required", "Mock interview access required", "Only active members and alumni may review mock interviews.", nil)
		return
	}
	var in struct {
		Reviewed bool   `json:"reviewed"`
		Comment  string `json:"comment"`
		Revision int64  `json:"revision"`
	}
	if err := decodeJSON(w, r, &in); err != nil {
		validation(a, w, r, err.Error())
		return
	}
	v, err := a.store.GetMockInterview(r.Context(), r.PathValue("id"))
	if err != nil {
		storeFailure(a, w, r, err)
		return
	}
	if actor.UserID != v.IntervieweeID {
		a.fail(w, r, 404, "not_found", "Not found", "The requested resource does not exist.", nil)
		return
	}
	if !a.mockSeasonWritable(w, r, v.SeasonID) {
		return
	}
	service := mockinterviews.Service{Sanitize: a.mockService.Sanitize}
	now := time.Now().UTC()
	if err := service.Review(&v, actor.UserID, r.PathValue("roundId"), in.Comment, in.Reviewed, in.Revision, now); err != nil {
		mockFailure(a, w, r, err)
		return
	}
	v, err = a.store.UpdateMockInterview(r.Context(), v, actor.UserID, "interviewee review", now)
	if err != nil {
		storeFailure(a, w, r, err)
		return
	}
	writeJSON(w, 200, withPass(a.withMockParticipantSummaries(r, v)))
}

func (a *API) correctMockIdentities(w http.ResponseWriter, r *http.Request) {
	actor := actorFrom(r.Context())
	if !actor.IsPrivileged() || !actor.HasRecentMFA(time.Now()) {
		a.fail(w, r, 403, "privileged_mfa_required", "Privileged access required", "Director or System Admin access with recent MFA is required.", nil)
		return
	}
	var in struct {
		InterviewerID string  `json:"interviewerId"`
		IntervieweeID string  `json:"intervieweeId"`
		SeasonID      *string `json:"seasonId"`
		Reason        string  `json:"reason"`
		Revision      int64   `json:"revision"`
	}
	if err := decodeJSON(w, r, &in); err != nil || in.InterviewerID == "" || in.IntervieweeID == "" || in.InterviewerID == in.IntervieweeID || in.Revision < 1 || in.Reason == "" {
		validation(a, w, r, "different interviewerId and intervieweeId, reason, and revision are required")
		return
	}
	v, err := a.store.GetMockInterview(r.Context(), r.PathValue("id"))
	if err != nil {
		storeFailure(a, w, r, err)
		return
	}
	for _, userID := range []string{in.InterviewerID, in.IntervieweeID} {
		participant, err := a.store.GetMockParticipant(r.Context(), userID)
		if err != nil || !participant.Eligible() {
			a.fail(w, r, 400, "invalid_mock_participant", "Invalid mock participant", "Corrected participants must be active members or alumni.", nil)
			return
		}
	}
	if !a.validMockSeason(w, r, in.SeasonID, v.OccurredAt, in.InterviewerID, in.IntervieweeID) {
		return
	}
	now := time.Now().UTC()
	if err := a.mockService.CorrectIdentities(&v, actor.UserID, in.InterviewerID, in.IntervieweeID, in.SeasonID, in.Reason, true, in.Revision, now); err != nil {
		mockFailure(a, w, r, err)
		return
	}
	v, err = a.store.UpdateMockInterview(r.Context(), v, actor.UserID, "identity correction: "+in.Reason, now)
	if err != nil {
		storeFailure(a, w, r, err)
		return
	}
	writeJSON(w, 200, withPass(a.withMockParticipantSummaries(r, v)))
}

func (a *API) validMockSeason(w http.ResponseWriter, r *http.Request, seasonID *string, occurredAt time.Time, userIDs ...string) bool {
	if seasonID == nil {
		return true
	}
	season, err := a.store.GetSeason(r.Context(), *seasonID)
	if err != nil {
		a.fail(w, r, 400, "invalid_mock_season", "Invalid mock season", "The selected season must exist.", nil)
		return false
	}
	if season.Status != "open" {
		a.fail(w, r, http.StatusConflict, "season_closed", "Season closed", "Season-linked mock interview records are locked until the season is reopened.", nil)
		return false
	}
	when := occurredAt.UTC()
	if when.Before(season.StartAt) || when.After(season.EndAt) {
		a.fail(w, r, 400, "invalid_mock_season", "Invalid mock season", "The interview date must fall within the selected season.", nil)
		return false
	}
	for _, userID := range userIDs {
		enrollments, listErr := a.store.ListEnrollmentsForUser(r.Context(), userID)
		if listErr != nil {
			storeFailure(a, w, r, listErr)
			return false
		}
		member := false
		for _, enrollment := range enrollments {
			if enrollment.SeasonID == *seasonID && (enrollment.State == "active" || enrollment.State == "completed") {
				member = true
				break
			}
		}
		if !member {
			a.fail(w, r, 400, "invalid_mock_season", "Invalid mock season", "Both participants must belong to the selected season.", nil)
			return false
		}
	}
	return true
}

func (a *API) mockSeasonWritable(w http.ResponseWriter, r *http.Request, seasonID *string) bool {
	if seasonID == nil {
		return true
	}
	season, err := a.store.GetSeason(r.Context(), *seasonID)
	if err != nil {
		storeFailure(a, w, r, err)
		return false
	}
	if season.Status != "open" {
		a.fail(w, r, http.StatusConflict, "season_closed", "Season closed", "Season-linked mock interview records are locked until the season is reopened.", nil)
		return false
	}
	return true
}

func mockFailure(a *API, w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, mockinterviews.ErrForbidden):
		a.fail(w, r, 403, "forbidden", "Forbidden", "The current account cannot perform this action.", nil)
	case errors.Is(err, mockinterviews.ErrConflict):
		a.fail(w, r, 409, "stale_revision", "Conflict", "The mock interview changed since it was loaded.", nil)
	default:
		a.fail(w, r, 400, "invalid_mock_interview", "Invalid mock interview", err.Error(), nil)
	}
}

func withPass(v mockinterviews.Interview) map[string]any {
	return map[string]any{"interview": v, "passed": mockinterviews.Passed(v)}
}

func (a *API) withMockParticipantSummaries(r *http.Request, interview mockinterviews.Interview) mockinterviews.Interview {
	load := func(userID string) mockinterviews.ParticipantSummary {
		summary := mockinterviews.ParticipantSummary{ID: userID, Name: "Deleted member"}
		if user, err := a.store.GetUser(r.Context(), userID); err == nil {
			summary.Slug, summary.Name, summary.AvatarURL = user.Slug, user.Name, user.AvatarURL
		}
		return summary
	}
	interview.Interviewer = load(interview.InterviewerID)
	interview.Interviewee = load(interview.IntervieweeID)
	return interview
}

func (a *API) adminSync(w http.ResponseWriter, r *http.Request) {
	actor := actorFrom(r.Context())
	if !actor.IsPrivileged() || !actor.HasRecentMFA(time.Now()) {
		a.fail(w, r, 403, "privileged_mfa_required", "Privileged access required", "Recent MFA is required.", nil)
		return
	}
	if a.syncLeetCode == nil {
		a.fail(w, r, 503, "sync_unavailable", "Sync unavailable", "The LeetCode worker queue is unavailable.", nil)
		return
	}
	if err := a.syncLeetCode(r.Context(), actor.UserID, requestIDFrom(r.Context())); err != nil {
		a.logger.Error("LeetCode sync enqueue failed", "requestId", requestIDFrom(r.Context()), "error", err.Error())
		a.fail(w, r, 503, "sync_failed", "Sync failed", "The LeetCode sync request could not be queued.", nil)
		return
	}
	writeJSON(w, 202, map[string]string{"status": "accepted"})
}
