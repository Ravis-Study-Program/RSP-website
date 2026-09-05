package httpapi

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/magedmg/RSP-website/backend/internal/authz"
	"github.com/magedmg/RSP-website/backend/internal/mockinterviews"
	"github.com/magedmg/RSP-website/backend/internal/platform/id"
)

type mockRoundRequest struct {
	ID        string                   `json:"id"`
	Type      mockinterviews.RoundType `json:"type"`
	ProblemID string                   `json:"problemId"`
	Content   string                   `json:"content"`
	Link      string                   `json:"link"`
	Scores    mockinterviews.Scores    `json:"scores"`
}

func mockRoundsFromRequest(rounds []mockRoundRequest) []mockinterviews.Round {
	result := make([]mockinterviews.Round, len(rounds))
	for i, round := range rounds {
		result[i] = mockinterviews.Round{
			ID:        round.ID,
			Type:      round.Type,
			ProblemID: round.ProblemID,
			Content:   round.Content,
			Link:      round.Link,
			Scores:    round.Scores,
		}
	}
	return result
}

func (a *API) eligibleMockActor(r *http.Request) bool {
	actor := actorFrom(r.Context())
	participant, err := a.db.GetMockParticipant(r.Context(), actor.UserID)
	return err == nil && participant.ProgrammeAccessEligible()
}

func (a *API) listMockParticipants(w http.ResponseWriter, r *http.Request) {
	actor := actorFrom(r.Context())
	if !a.eligibleMockActor(r) && !actor.IsPrivileged() {
		writeErrorResponse(w, http.StatusForbidden, "mock_participant_required", "Only programme members may select eligible mock-interview participants.")
		return
	}
	if _, err := requestedSort(r, "id:asc", "id:asc"); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "validation_failed", "sort must be id:asc")
		return
	}

	direction, err := requestedDirection(r)
	if err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "validation_failed", "direction must be forward or backward")
		return
	}

	query := strings.TrimSpace(r.URL.Query().Get("query"))
	if len(query) > 100 {
		writeErrorResponse(w, http.StatusBadRequest, "validation_failed", "query must be at most 100 characters")
		return
	}
	binding := "mock-interview-participants|sort=id:asc|query=" + query
	limit, boundary, err := a.page(r, binding)
	if err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "invalid_cursor", "The cursor does not match the selected participant filters and sort.")
		return
	}

	users, more, total, err := a.db.ListUsers(r.Context(), boundary, limit, direction, query, "", "")
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}

	items := make([]mockinterviews.ParticipantSummary, 0, len(users))
	for _, user := range users {
		items = append(items, mockinterviews.ParticipantSummary{ID: user.ID, Slug: user.Slug, Name: user.Name, AvatarURL: user.AvatarURL})
	}
	pageInfo := pageInfoForKeyset(
		a, binding, direction, boundary, items, more,
		func(v mockinterviews.ParticipantSummary) string { return v.ID },
	)
	response := Page[mockinterviews.ParticipantSummary]{
		Items:      items,
		PageInfo:   pageInfo,
		TotalCount: total,
	}
	writeJSONResponse(w, http.StatusOK, response)
}

func (a *API) listMocks(w http.ResponseWriter, r *http.Request) {
	actor := actorFrom(r.Context())
	if !a.eligibleMockActor(r) && !actor.IsPrivileged() && !historicalMockReviewer(actor) {
		writeErrorResponse(w, http.StatusForbidden, "mock_participant_required", "Only active members and alumni may access mock interviews.")
		return
	}
	mode := r.URL.Query().Get("mode")
	if mode == "" {
		mode = "received"
	}
	if mode != "received" && mode != "given" && mode != "all" {
		writeErrorResponse(w, http.StatusBadRequest, "validation_failed", "mode must be received, given, or all")
		return
	}
	if mode == "all" && !a.auditSystemAdminPrivateRead(w, r, "mock_interview_collection", actor.UserID) {
		return
	}
	sortBy, err := requestedSort(r, "occurredAt:desc", "occurredAt:desc", "id:asc")
	if err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "validation_failed", "sort must be occurredAt:desc or id:asc")
		return
	}

	direction, err := requestedDirection(r)
	if err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "validation_failed", "direction must be forward or backward")
		return
	}

	binding := "mock-interviews|mode=" + mode + "|sort=" + sortBy
	limit, boundary, err := a.page(r, binding)
	if err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "invalid_cursor", "The cursor does not match the selected mode and sort.")
		return
	}

	items, more, total, err := a.db.ListMockInterviews(r.Context(), actor, mode, boundary, limit, sortBy, direction)
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}

	for index := range items {
		items[index] = a.withMockParticipantSummaries(r, items[index])
	}
	pageInfo := pageInfoForKeyset(
		a, binding, direction, boundary, items, more,
		func(v mockinterviews.Interview) string { return v.ID },
	)
	response := Page[mockinterviews.Interview]{
		Items:      items,
		PageInfo:   pageInfo,
		TotalCount: total,
	}
	writeJSONResponse(w, http.StatusOK, response)
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
		writeErrorResponse(w, http.StatusForbidden, "mock_participant_required", "The interviewer must be an active member or alumnus.")
		return
	}
	var in struct {
		Interviewee struct {
			UserID string `json:"userId"`
		} `json:"interviewee"`
		SeasonID        *string            `json:"seasonId"`
		OccurredAt      time.Time          `json:"occurredAt"`
		DurationMinutes int                `json:"durationMinutes"`
		Notes           string             `json:"notes"`
		Rounds          []mockRoundRequest `json:"rounds"`
	}
	if err := decodeJSON(w, r, &in); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "validation_failed", err.Error())
		return
	}
	// Eligibility is loaded from app state; the request can only identify the
	// interviewee.
	interviewee, err := a.db.GetMockParticipant(r.Context(), in.Interviewee.UserID)
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}
	if !a.validMockSeason(w, r, in.SeasonID, in.OccurredAt, actor.UserID, interviewee.UserID) {
		return
	}
	now := time.Now().UTC()
	input := mockinterviews.CreateInput{
		Interviewee:     interviewee,
		SeasonID:        in.SeasonID,
		OccurredAt:      in.OccurredAt,
		DurationMinutes: in.DurationMinutes,
		Notes:           in.Notes,
		Rounds:          mockRoundsFromRequest(in.Rounds),
	}
	v, err := a.mockRules.Create(actor.UserID, input, now)
	if err != nil {
		mockFailure(a, w, r, err)
		return
	}
	v.ID = id.New()
	v, err = a.db.CreateMockInterview(r.Context(), v, actor.UserID, now)
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}

	interview := a.withMockParticipantSummaries(r, v)
	response := withPass(interview)
	writeJSONResponse(w, http.StatusCreated, response)
}

func (a *API) updateMock(w http.ResponseWriter, r *http.Request) {
	actor := actorFrom(r.Context())
	if !a.eligibleMockActor(r) && !actor.IsPrivileged() {
		writeErrorResponse(w, http.StatusForbidden, "mock_participant_required", "Only active members and alumni may update mock interviews.")
		return
	}
	var in struct {
		Revision        int64              `json:"revision"`
		OccurredAt      time.Time          `json:"occurredAt"`
		DurationMinutes int                `json:"durationMinutes"`
		Notes           string             `json:"notes"`
		Rounds          []mockRoundRequest `json:"rounds"`
	}
	if err := decodeJSON(w, r, &in); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "validation_failed", err.Error())
		return
	}

	v, err := a.db.GetMockInterview(r.Context(), r.PathValue("id"))
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}
	if actor.UserID != v.InterviewerID {
		writeErrorResponse(w, http.StatusNotFound, "not_found", "The requested resource does not exist.")
		return
	}
	if !a.validMockSeason(w, r, v.SeasonID, in.OccurredAt, v.InterviewerID, v.IntervieweeID) {
		return
	}
	if !a.mockSeasonWritable(w, r, v.SeasonID) {
		return
	}
	now := time.Now().UTC()
	input := mockinterviews.UpdateInput{
		ExpectedRevision: in.Revision,
		OccurredAt:       in.OccurredAt,
		DurationMinutes:  in.DurationMinutes,
		Notes:            in.Notes,
		Rounds:           mockRoundsFromRequest(in.Rounds),
	}
	v, err = a.mockRules.Update(v, actor.UserID, input, now)
	if err != nil {
		mockFailure(a, w, r, err)
		return
	}
	v, err = a.db.UpdateMockInterview(r.Context(), v, actor.UserID, now)
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}

	interview := a.withMockParticipantSummaries(r, v)
	response := withPass(interview)
	writeJSONResponse(w, http.StatusOK, response)
}

func (a *API) deleteMock(w http.ResponseWriter, r *http.Request) {
	actor := actorFrom(r.Context())
	if !a.eligibleMockActor(r) && !actor.IsPrivileged() {
		writeErrorResponse(w, http.StatusForbidden, "mock_participant_required", "Only active members and alumni may delete mock interviews.")
		return
	}
	revision, err := parseRevision(r)
	if err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "validation_failed", "revision query parameter is required")
		return
	}

	v, err := a.db.GetMockInterview(r.Context(), r.PathValue("id"))
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}
	if actor.UserID != v.InterviewerID {
		writeErrorResponse(w, http.StatusNotFound, "not_found", "The requested resource does not exist.")
		return
	}
	if !a.mockSeasonWritable(w, r, v.SeasonID) {
		return
	}
	now := time.Now().UTC()
	v, err = a.mockRules.Delete(v, actor.UserID, revision, now)
	if err != nil {
		mockFailure(a, w, r, err)
		return
	}
	if _, err = a.db.DeleteMockInterview(r.Context(), v, actor.UserID, now); err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) reviewRound(w http.ResponseWriter, r *http.Request) {
	actor := actorFrom(r.Context())
	if !a.eligibleMockActor(r) && !actor.IsPrivileged() {
		writeErrorResponse(w, http.StatusForbidden, "mock_participant_required", "Only active members and alumni may review mock interviews.")
		return
	}
	var in struct {
		Reviewed bool   `json:"reviewed"`
		Comment  string `json:"comment"`
		Revision int64  `json:"revision"`
	}
	if err := decodeJSON(w, r, &in); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "validation_failed", err.Error())
		return
	}

	v, err := a.db.GetMockInterview(r.Context(), r.PathValue("id"))
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}
	if actor.UserID != v.IntervieweeID {
		writeErrorResponse(w, http.StatusNotFound, "not_found", "The requested resource does not exist.")
		return
	}
	if !a.mockSeasonWritable(w, r, v.SeasonID) {
		return
	}
	now := time.Now().UTC()
	v, err = a.mockRules.Review(v, actor.UserID, r.PathValue("roundId"), in.Comment, in.Reviewed, in.Revision, now)
	if err != nil {
		mockFailure(a, w, r, err)
		return
	}
	v, err = a.db.ReviewMockInterviewRound(r.Context(), v, r.PathValue("roundId"), actor.UserID, now)
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}

	interview := a.withMockParticipantSummaries(r, v)
	response := withPass(interview)
	writeJSONResponse(w, http.StatusOK, response)
}

func (a *API) correctMockIdentities(w http.ResponseWriter, r *http.Request) {
	actor := actorFrom(r.Context())
	if !actor.IsPrivileged() || !actor.HasRecentMFA(time.Now()) {
		writeErrorResponse(w, http.StatusForbidden, "privileged_mfa_required", "Director or System Admin access with recent MFA is required.")
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
		writeErrorResponse(w, http.StatusBadRequest, "validation_failed", "different interviewerId and intervieweeId, reason, and revision are required")
		return
	}

	v, err := a.db.GetMockInterview(r.Context(), r.PathValue("id"))
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}

	for _, userID := range []string{in.InterviewerID, in.IntervieweeID} {
		participant, err := a.db.GetMockParticipant(r.Context(), userID)
		if err != nil || !participant.Eligible() {
			writeErrorResponse(w, http.StatusBadRequest, "invalid_mock_participant", "Corrected participants must be active members or alumni.")
			return
		}
	}
	if !a.validMockSeason(w, r, in.SeasonID, v.OccurredAt, in.InterviewerID, in.IntervieweeID) {
		return
	}
	now := time.Now().UTC()
	v, err = a.mockRules.CorrectIdentities(v, actor.UserID, in.InterviewerID, in.IntervieweeID, in.SeasonID, in.Reason, true, in.Revision, now)
	if err != nil {
		mockFailure(a, w, r, err)
		return
	}
	v, err = a.db.CorrectMockInterviewIdentities(r.Context(), v, actor.UserID, in.Reason, now)
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}

	interview := a.withMockParticipantSummaries(r, v)
	response := withPass(interview)
	writeJSONResponse(w, http.StatusOK, response)
}

func (a *API) validMockSeason(w http.ResponseWriter, r *http.Request, seasonID *string, occurredAt time.Time, userIDs ...string) bool {
	if seasonID == nil {
		return true
	}
	season, err := a.db.GetSeason(r.Context(), *seasonID)
	if err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "invalid_mock_season", "The selected season must exist.")
		return false
	}
	if season.Status != "open" {
		writeErrorResponse(w, http.StatusConflict, "season_closed", "Season-linked mock interview records are locked until the season is reopened.")
		return false
	}
	when := occurredAt.UTC()
	if when.Before(season.StartAt) || when.After(season.EndAt) {
		writeErrorResponse(w, http.StatusBadRequest, "invalid_mock_season", "The interview date must fall within the selected season.")
		return false
	}
	for _, userID := range userIDs {
		enrollments, listErr := a.db.ListEnrollmentsForUser(r.Context(), userID)
		if listErr != nil {
			a.writeStoreErrorResponse(w, listErr)
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
			writeErrorResponse(w, http.StatusBadRequest, "invalid_mock_season", "Both participants must belong to the selected season.")
			return false
		}
	}
	return true
}

func (a *API) mockSeasonWritable(w http.ResponseWriter, r *http.Request, seasonID *string) bool {
	if seasonID == nil {
		return true
	}
	season, err := a.db.GetSeason(r.Context(), *seasonID)
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return false
	}
	if season.Status != "open" {
		writeErrorResponse(w, http.StatusConflict, "season_closed", "Season-linked mock interview records are locked until the season is reopened.")
		return false
	}
	return true
}

func mockFailure(a *API, w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, mockinterviews.ErrForbidden):
		writeErrorResponse(w, http.StatusForbidden, "forbidden", "The current account cannot perform this action.")
	case errors.Is(err, mockinterviews.ErrConflict):
		writeErrorResponse(w, http.StatusConflict, "stale_revision", "The mock interview changed since it was loaded.")
	default:
		writeErrorResponse(w, http.StatusBadRequest, "invalid_mock_interview", err.Error())
	}
}

func withPass(v mockinterviews.Interview) map[string]any {
	return map[string]any{"interview": v, "passed": mockinterviews.Passed(v)}
}

func (a *API) withMockParticipantSummaries(r *http.Request, interview mockinterviews.Interview) mockinterviews.Interview {
	load := func(userID string) mockinterviews.ParticipantSummary {
		summary := mockinterviews.ParticipantSummary{ID: userID, Name: "Deleted member"}
		if user, err := a.db.GetUser(r.Context(), userID); err == nil {
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
		writeErrorResponse(w, http.StatusForbidden, "privileged_mfa_required", "Recent MFA is required.")
		return
	}
	if a.syncLeetCode == nil {
		writeErrorResponse(w, http.StatusServiceUnavailable, "sync_unavailable", "The LeetCode worker queue is unavailable.")
		return
	}
	if err := a.syncLeetCode(r.Context(), actor.UserID, requestIDFrom(r.Context())); err != nil {
		a.logger.Error("LeetCode sync enqueue failed", "requestId", requestIDFrom(r.Context()), "error", err.Error())
		writeErrorResponse(w, http.StatusServiceUnavailable, "sync_failed", "The LeetCode sync request could not be queued.")
		return
	}

	writeJSONResponse(w, http.StatusAccepted, map[string]string{"status": "accepted"})
}
