package api

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/magedmg/RSP-website/backend/internal/authz"
	"github.com/magedmg/RSP-website/backend/internal/dal"
	"github.com/magedmg/RSP-website/backend/internal/mockinterviews"
	"github.com/magedmg/RSP-website/backend/internal/platform/id"
	"github.com/magedmg/RSP-website/backend/internal/programme"
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

func (a *API) loadMockEligibility(r *http.Request) (bool, error) {
	actor := actorFrom(r.Context())
	participant, err := a.db.GetMockParticipant(r.Context(), actor.UserID)
	if errors.Is(err, dal.ErrNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return participant.ProgrammeAccessEligible(), nil
}

func (a *API) listMockParticipants(w http.ResponseWriter, r *http.Request) {
	actor := actorFrom(r.Context())
	eligible, err := a.loadMockEligibility(r)
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}
	if !eligible && !actor.IsDirectorOrSystemAdmin() {
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

	users, more, total, err := a.db.ListUsers(r.Context(), dal.UserQuery{
		Boundary:  boundary,
		Limit:     limit,
		Direction: direction,
		Search:    query,
	})
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
	eligible, err := a.loadMockEligibility(r)
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}
	if !eligible && !actor.IsDirectorOrSystemAdmin() && !historicalMockReviewer(actor) {
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
	if mode == "all" {
		if err := a.auditPrivateDataRead(r.Context(), actorFrom(r.Context()), "mock_interview_collection", actor.UserID); err != nil {
			writeErrorResponse(w, http.StatusInternalServerError, "audit_failed", "Private data was not returned because its access could not be audited.")
			return
		}
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

	items, more, total, err := a.db.ListMockInterviews(r.Context(), actor, dal.MockInterviewQuery{
		Mode:      mode,
		Boundary:  boundary,
		Limit:     limit,
		SortBy:    sortBy,
		Direction: direction,
	})
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}

	userIDs := make([]string, 0, 2*len(items))
	for _, interview := range items {
		userIDs = append(userIDs, interview.InterviewerID, interview.IntervieweeID)
	}
	summaries, err := a.db.ListMockParticipantSummaries(r.Context(), userIDs)
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}
	for index := range items {
		items[index] = withMockParticipantSummaries(items[index], summaries)
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
	eligible, err := a.loadMockEligibility(r)
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}
	if !eligible {
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
	if err := decodeJSON(r.Body, &in); err != nil {
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
	if in.SeasonID != nil {
		season, err := a.db.GetSeason(r.Context(), *in.SeasonID)
		if err != nil {
			if errors.Is(err, dal.ErrNotFound) {
				writeErrorResponse(w, http.StatusBadRequest, "invalid_mock_season", "The selected season must exist.")
			} else {
				a.writeStoreErrorResponse(w, err)
			}
			return
		}
		if season.Status != "open" {
			writeErrorResponse(w, http.StatusConflict, "season_closed", "Season-linked mock interview records are locked until the season is reopened.")
			return
		}
		if in.OccurredAt.Before(season.StartAt) || in.OccurredAt.After(season.EndAt) {
			writeErrorResponse(w, http.StatusBadRequest, "invalid_mock_season", "The interview date must fall within the selected season.")
			return
		}
		for _, participantID := range []string{actor.UserID, interviewee.UserID} {
			enrollments, err := a.db.ListEnrollmentsForUser(r.Context(), participantID)
			if err != nil {
				a.writeStoreErrorResponse(w, err)
				return
			}
			if !programme.IsEnrolledInSeason(enrollments, season.ID) {
				writeErrorResponse(w, http.StatusBadRequest, "invalid_mock_season", "Both participants must belong to the selected season.")
				return
			}
		}
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
		writeMockErrorResponse(w, err)
		return
	}
	v.ID = id.New()
	v, err = a.db.CreateMockInterview(r.Context(), v, actor.UserID, now)
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}

	summaries, err := a.db.ListMockParticipantSummaries(r.Context(), []string{v.InterviewerID, v.IntervieweeID})
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}
	interview := withMockParticipantSummaries(v, summaries)
	response := withPass(interview)
	writeJSONResponse(w, http.StatusCreated, response)
}

func (a *API) updateMock(w http.ResponseWriter, r *http.Request) {
	actor := actorFrom(r.Context())
	eligible, err := a.loadMockEligibility(r)
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}
	if !eligible && !actor.IsDirectorOrSystemAdmin() {
		writeErrorResponse(w, http.StatusForbidden, "mock_participant_required", "Only active members and alumni may update mock interviews.")
		return
	}
	var in struct {
		OccurredAt      time.Time          `json:"occurredAt"`
		DurationMinutes int                `json:"durationMinutes"`
		Notes           string             `json:"notes"`
		Rounds          []mockRoundRequest `json:"rounds"`
	}
	if err := decodeJSON(r.Body, &in); err != nil {
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
	if v.SeasonID != nil {
		season, err := a.db.GetSeason(r.Context(), *v.SeasonID)
		if err != nil {
			if errors.Is(err, dal.ErrNotFound) {
				writeErrorResponse(w, http.StatusBadRequest, "invalid_mock_season", "The selected season must exist.")
			} else {
				a.writeStoreErrorResponse(w, err)
			}
			return
		}
		if season.Status != "open" {
			writeErrorResponse(w, http.StatusConflict, "season_closed", "Season-linked mock interview records are locked until the season is reopened.")
			return
		}
		if in.OccurredAt.Before(season.StartAt) || in.OccurredAt.After(season.EndAt) {
			writeErrorResponse(w, http.StatusBadRequest, "invalid_mock_season", "The interview date must fall within the selected season.")
			return
		}
		for _, participantID := range []string{v.InterviewerID, v.IntervieweeID} {
			enrollments, err := a.db.ListEnrollmentsForUser(r.Context(), participantID)
			if err != nil {
				a.writeStoreErrorResponse(w, err)
				return
			}
			if !programme.IsEnrolledInSeason(enrollments, season.ID) {
				writeErrorResponse(w, http.StatusBadRequest, "invalid_mock_season", "Both participants must belong to the selected season.")
				return
			}
		}
	}
	now := time.Now().UTC()
	input := mockinterviews.UpdateInput{
		OccurredAt:      in.OccurredAt,
		DurationMinutes: in.DurationMinutes,
		Notes:           in.Notes,
		Rounds:          mockRoundsFromRequest(in.Rounds),
	}
	v, err = a.mockRules.Update(v, actor.UserID, input, now)
	if err != nil {
		writeMockErrorResponse(w, err)
		return
	}
	v, err = a.db.UpdateMockInterview(r.Context(), v, actor.UserID, now)
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}

	summaries, err := a.db.ListMockParticipantSummaries(r.Context(), []string{v.InterviewerID, v.IntervieweeID})
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}
	interview := withMockParticipantSummaries(v, summaries)
	response := withPass(interview)
	writeJSONResponse(w, http.StatusOK, response)
}

func (a *API) deleteMock(w http.ResponseWriter, r *http.Request) {
	actor := actorFrom(r.Context())
	eligible, err := a.loadMockEligibility(r)
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}
	if !eligible && !actor.IsDirectorOrSystemAdmin() {
		writeErrorResponse(w, http.StatusForbidden, "mock_participant_required", "Only active members and alumni may delete mock interviews.")
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
	if v.SeasonID != nil {
		season, err := a.db.GetSeason(r.Context(), *v.SeasonID)
		if err != nil {
			a.writeStoreErrorResponse(w, err)
			return
		}
		if season.Status != "open" {
			writeErrorResponse(w, http.StatusConflict, "season_closed", "Season-linked mock interview records are locked until the season is reopened.")
			return
		}
	}
	now := time.Now().UTC()
	v, err = a.mockRules.Delete(v, actor.UserID, now)
	if err != nil {
		writeMockErrorResponse(w, err)
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
	eligible, err := a.loadMockEligibility(r)
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}
	if !eligible && !actor.IsDirectorOrSystemAdmin() {
		writeErrorResponse(w, http.StatusForbidden, "mock_participant_required", "Only active members and alumni may review mock interviews.")
		return
	}
	var in struct {
		Reviewed bool   `json:"reviewed"`
		Comment  string `json:"comment"`
	}
	if err := decodeJSON(r.Body, &in); err != nil {
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
	if v.SeasonID != nil {
		season, err := a.db.GetSeason(r.Context(), *v.SeasonID)
		if err != nil {
			a.writeStoreErrorResponse(w, err)
			return
		}
		if season.Status != "open" {
			writeErrorResponse(w, http.StatusConflict, "season_closed", "Season-linked mock interview records are locked until the season is reopened.")
			return
		}
	}
	now := time.Now().UTC()
	v, err = a.mockRules.Review(v, actor.UserID, r.PathValue("roundId"), in.Comment, in.Reviewed, now)
	if err != nil {
		writeMockErrorResponse(w, err)
		return
	}
	v, err = a.db.ReviewMockInterviewRound(r.Context(), v, r.PathValue("roundId"), actor.UserID, now)
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}

	summaries, err := a.db.ListMockParticipantSummaries(r.Context(), []string{v.InterviewerID, v.IntervieweeID})
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}
	interview := withMockParticipantSummaries(v, summaries)
	response := withPass(interview)
	writeJSONResponse(w, http.StatusOK, response)
}

func (a *API) correctMockIdentities(w http.ResponseWriter, r *http.Request) {
	actor := actorFrom(r.Context())
	if !actor.IsDirectorOrSystemAdmin() || !actor.HasRecentMFA(time.Now()) {
		writeErrorResponse(w, http.StatusForbidden, "privileged_mfa_required", "Director or System Admin access with recent MFA is required.")
		return
	}
	var in struct {
		InterviewerID string  `json:"interviewerId"`
		IntervieweeID string  `json:"intervieweeId"`
		SeasonID      *string `json:"seasonId"`
		Reason        string  `json:"reason"`
	}
	if err := decodeJSON(r.Body, &in); err != nil || in.InterviewerID == "" || in.IntervieweeID == "" || in.InterviewerID == in.IntervieweeID || in.Reason == "" {
		writeErrorResponse(w, http.StatusBadRequest, "validation_failed", "different interviewerId and intervieweeId, reason, are required")
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
	if in.SeasonID != nil {
		season, err := a.db.GetSeason(r.Context(), *in.SeasonID)
		if err != nil {
			if errors.Is(err, dal.ErrNotFound) {
				writeErrorResponse(w, http.StatusBadRequest, "invalid_mock_season", "The selected season must exist.")
			} else {
				a.writeStoreErrorResponse(w, err)
			}
			return
		}
		if season.Status != "open" {
			writeErrorResponse(w, http.StatusConflict, "season_closed", "Season-linked mock interview records are locked until the season is reopened.")
			return
		}
		if v.OccurredAt.Before(season.StartAt) || v.OccurredAt.After(season.EndAt) {
			writeErrorResponse(w, http.StatusBadRequest, "invalid_mock_season", "The interview date must fall within the selected season.")
			return
		}
		for _, participantID := range []string{in.InterviewerID, in.IntervieweeID} {
			enrollments, err := a.db.ListEnrollmentsForUser(r.Context(), participantID)
			if err != nil {
				a.writeStoreErrorResponse(w, err)
				return
			}
			if !programme.IsEnrolledInSeason(enrollments, season.ID) {
				writeErrorResponse(w, http.StatusBadRequest, "invalid_mock_season", "Both participants must belong to the selected season.")
				return
			}
		}
	}
	now := time.Now().UTC()
	v, err = a.mockRules.CorrectIdentities(v, actor.UserID, in.InterviewerID, in.IntervieweeID, in.SeasonID, in.Reason, true, now)
	if err != nil {
		writeMockErrorResponse(w, err)
		return
	}
	v, err = a.db.CorrectMockInterviewIdentities(r.Context(), v, actor.UserID, in.Reason, now)
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}

	summaries, err := a.db.ListMockParticipantSummaries(r.Context(), []string{v.InterviewerID, v.IntervieweeID})
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}
	interview := withMockParticipantSummaries(v, summaries)
	response := withPass(interview)
	writeJSONResponse(w, http.StatusOK, response)
}

func writeMockErrorResponse(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, mockinterviews.ErrForbidden):
		writeErrorResponse(w, http.StatusForbidden, "forbidden", "The current account cannot perform this action.")
	default:
		writeErrorResponse(w, http.StatusBadRequest, "invalid_mock_interview", err.Error())
	}
}

func withPass(v mockinterviews.Interview) map[string]any {
	return map[string]any{"interview": v, "passed": mockinterviews.Passed(v)}
}

func withMockParticipantSummaries(interview mockinterviews.Interview, summaries map[string]mockinterviews.ParticipantSummary) mockinterviews.Interview {
	lookup := func(userID string) mockinterviews.ParticipantSummary {
		if summary, ok := summaries[userID]; ok {
			return summary
		}
		return mockinterviews.ParticipantSummary{ID: userID, Name: "Deleted member"}
	}
	interview.Interviewer = lookup(interview.InterviewerID)
	interview.Interviewee = lookup(interview.IntervieweeID)
	return interview
}

func (a *API) adminSync(w http.ResponseWriter, r *http.Request) {
	actor := actorFrom(r.Context())
	if !actor.IsDirectorOrSystemAdmin() || !actor.HasRecentMFA(time.Now()) {
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
