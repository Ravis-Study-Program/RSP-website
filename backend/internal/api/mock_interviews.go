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

func (a *API) listMockInterviewParticipants(w http.ResponseWriter, r *http.Request) {
	actor := actorFrom(r.Context())
	memberStatus, err := a.db.GetMemberStatus(r.Context(), actor.UserID)
	if err != nil && !errors.Is(err, dal.ErrNotFound) {
		a.writeStoreErrorResponse(w, err)
		return
	}
	eligible := err == nil && memberStatus.CanAccessProgramme()
	if !eligible && !actor.IsDirectorOrSystemAdmin() {
		writeErrorResponse(w, http.StatusForbidden, "Only programme members may select eligible mock-interview participants.")
		return
	}
	if _, err := parseSort(r.URL.Query().Get("sort"), "id:asc", "id:asc"); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "sort must be id:asc")
		return
	}

	direction, err := parsePageDirection(r.URL.Query().Get("direction"))
	if err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "direction must be forward or backward")
		return
	}

	query := strings.TrimSpace(r.URL.Query().Get("query"))
	if len(query) > 100 {
		writeErrorResponse(w, http.StatusBadRequest, "query must be at most 100 characters")
		return
	}
	binding := "mock-interview-participants|sort=id:asc|query=" + query
	limit, boundary, err := parsePagination(r.URL.Query().Get("limit"), r.URL.Query().Get("cursor"), binding, a.cursorSecret)
	if err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "The cursor does not match the selected participant filters and sort.")
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
		items = append(items, mockinterviews.ParticipantSummary{
			ID:        user.ID,
			Slug:      user.Slug,
			Name:      user.Name,
			AvatarURL: user.AvatarURL,
		})
	}
	pageInfo, err := pageInfoForKeyset(
		a.cursorSecret, binding, direction, boundary, items, more,
		func(storedInterview mockinterviews.ParticipantSummary) string { return storedInterview.ID },
	)
	if err != nil {
		a.logger.Error("cursor encoding failed", "error", err)
		writeErrorResponse(w, http.StatusInternalServerError, "The request could not be completed.")
		return
	}
	response := Page[mockinterviews.ParticipantSummary]{
		Items:      items,
		PageInfo:   pageInfo,
		TotalCount: total,
	}
	writeJSONResponse(w, http.StatusOK, response)
}

func (a *API) listMockInterviews(w http.ResponseWriter, r *http.Request) {
	actor := actorFrom(r.Context())
	memberStatus, err := a.db.GetMemberStatus(r.Context(), actor.UserID)
	if err != nil && !errors.Is(err, dal.ErrNotFound) {
		a.writeStoreErrorResponse(w, err)
		return
	}
	eligible := err == nil && memberStatus.CanAccessProgramme()
	if !eligible && !actor.IsDirectorOrSystemAdmin() && !historicalMockReviewer(actor) {
		writeErrorResponse(w, http.StatusForbidden, "Only active members and alumni may access mock interviews.")
		return
	}
	mode := r.URL.Query().Get("mode")
	if mode == "" {
		mode = "received"
	}
	if mode != "received" && mode != "given" && mode != "all" {
		writeErrorResponse(w, http.StatusBadRequest, "mode must be received, given, or all")
		return
	}
	if mode == "all" {
		if err := a.auditPrivateDataRead(r.Context(), actor, "mock_interview_collection", actor.UserID); err != nil {
			writeErrorResponse(w, http.StatusInternalServerError, "Private data was not returned because its access could not be audited.")
			return
		}
	}
	sortBy, err := parseSort(r.URL.Query().Get("sort"), "occurredAt:desc", "occurredAt:desc", "id:asc")
	if err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "sort must be occurredAt:desc or id:asc")
		return
	}

	direction, err := parsePageDirection(r.URL.Query().Get("direction"))
	if err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "direction must be forward or backward")
		return
	}

	binding := "mock-interviews|mode=" + mode + "|sort=" + sortBy
	limit, boundary, err := parsePagination(r.URL.Query().Get("limit"), r.URL.Query().Get("cursor"), binding, a.cursorSecret)
	if err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "The cursor does not match the selected mode and sort.")
		return
	}

	visibility := dal.MockInterviewsReceived
	switch mode {
	case "given":
		visibility = dal.MockInterviewsGiven
	case "all":
		visibility = dal.MockInterviewsRelated
		if actor.IsDirectorOrSystemAdmin() {
			visibility = dal.MockInterviewsAll
		}
	}
	items, more, total, err := a.db.ListMockInterviews(r.Context(), dal.MockInterviewQuery{
		ViewerID:   actor.UserID,
		Visibility: visibility,
		Boundary:   boundary,
		Limit:      limit,
		SortBy:     sortBy,
		Direction:  direction,
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
	pageInfo, err := pageInfoForKeyset(
		a.cursorSecret, binding, direction, boundary, items, more,
		func(storedInterview mockinterviews.Interview) string { return storedInterview.ID },
	)
	if err != nil {
		a.logger.Error("cursor encoding failed", "error", err)
		writeErrorResponse(w, http.StatusInternalServerError, "The request could not be completed.")
		return
	}
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

type createMockInterviewRequest struct {
	Interviewee struct {
		UserID string `json:"userId"`
	} `json:"interviewee"`
	SeasonID        *string            `json:"seasonId"`
	OccurredAt      time.Time          `json:"occurredAt"`
	DurationMinutes int                `json:"durationMinutes"`
	Notes           string             `json:"notes"`
	Rounds          []mockRoundRequest `json:"rounds"`
}

func (a *API) createMockInterview(w http.ResponseWriter, r *http.Request) {
	actor := actorFrom(r.Context())
	memberStatus, err := a.db.GetMemberStatus(r.Context(), actor.UserID)
	if err != nil && !errors.Is(err, dal.ErrNotFound) {
		a.writeStoreErrorResponse(w, err)
		return
	}
	eligible := err == nil && memberStatus.CanAccessProgramme()
	if !eligible {
		writeErrorResponse(w, http.StatusForbidden, "The interviewer must be an active member or alumnus.")
		return
	}
	var request createMockInterviewRequest
	if err := decodeJSON(r.Body, &request); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	// Eligibility is loaded from app state; the request can only identify the
	// interviewee.
	interviewee, err := a.db.GetMemberStatus(r.Context(), request.Interviewee.UserID)
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}
	if request.SeasonID != nil {
		season, err := a.db.GetSeason(r.Context(), *request.SeasonID)
		if err != nil {
			if errors.Is(err, dal.ErrNotFound) {
				writeErrorResponse(w, http.StatusBadRequest, "The selected season must exist.")
			} else {
				a.writeStoreErrorResponse(w, err)
			}
			return
		}
		if season.Status != "open" {
			writeErrorResponse(w, http.StatusConflict, "Season-linked mock interview records are locked until the season is reopened.")
			return
		}
		if request.OccurredAt.Before(season.StartAt) || request.OccurredAt.After(season.EndAt) {
			writeErrorResponse(w, http.StatusBadRequest, "The interview date must fall within the selected season.")
			return
		}
		for _, participantID := range []string{actor.UserID, interviewee.UserID} {
			enrollments, err := a.db.ListEnrollmentsForUser(r.Context(), participantID)
			if err != nil {
				a.writeStoreErrorResponse(w, err)
				return
			}
			if !programme.IsEnrolledInSeason(enrollments, season.ID) {
				writeErrorResponse(w, http.StatusBadRequest, "Both participants must belong to the selected season.")
				return
			}
		}
	}
	now := time.Now().UTC()
	input := mockinterviews.CreateInput{
		Interviewee:     interviewee,
		SeasonID:        request.SeasonID,
		OccurredAt:      request.OccurredAt,
		DurationMinutes: request.DurationMinutes,
		Notes:           request.Notes,
		Rounds:          mockRoundsFromRequest(request.Rounds),
	}
	storedInterview, err := a.mockRules.Create(actor.UserID, input, now)
	if err != nil {
		a.writeMockErrorResponse(w, err)
		return
	}
	storedInterview.ID = id.New()
	storedInterview, err = a.db.CreateMockInterview(r.Context(), storedInterview, actor.UserID, now)
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}

	summaries, err := a.db.ListMockParticipantSummaries(r.Context(), []string{storedInterview.InterviewerID, storedInterview.IntervieweeID})
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}
	interview := withMockParticipantSummaries(storedInterview, summaries)
	response := buildMockInterviewResponse(interview)
	writeJSONResponse(w, http.StatusCreated, response)
}

type updateMockInterviewRequest struct {
	OccurredAt      time.Time          `json:"occurredAt"`
	DurationMinutes int                `json:"durationMinutes"`
	Notes           string             `json:"notes"`
	Rounds          []mockRoundRequest `json:"rounds"`
}

func (a *API) updateMockInterview(w http.ResponseWriter, r *http.Request) {
	actor := actorFrom(r.Context())
	memberStatus, err := a.db.GetMemberStatus(r.Context(), actor.UserID)
	if err != nil && !errors.Is(err, dal.ErrNotFound) {
		a.writeStoreErrorResponse(w, err)
		return
	}
	eligible := err == nil && memberStatus.CanAccessProgramme()
	if !eligible && !actor.IsDirectorOrSystemAdmin() {
		writeErrorResponse(w, http.StatusForbidden, "Only active members and alumni may update mock interviews.")
		return
	}
	var request updateMockInterviewRequest
	if err := decodeJSON(r.Body, &request); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	storedInterview, err := a.db.GetMockInterview(r.Context(), r.PathValue("id"))
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}
	if actor.UserID != storedInterview.InterviewerID {
		writeErrorResponse(w, http.StatusNotFound, "The requested resource does not exist.")
		return
	}
	if storedInterview.SeasonID != nil {
		season, err := a.db.GetSeason(r.Context(), *storedInterview.SeasonID)
		if err != nil {
			if errors.Is(err, dal.ErrNotFound) {
				writeErrorResponse(w, http.StatusBadRequest, "The selected season must exist.")
			} else {
				a.writeStoreErrorResponse(w, err)
			}
			return
		}
		if season.Status != "open" {
			writeErrorResponse(w, http.StatusConflict, "Season-linked mock interview records are locked until the season is reopened.")
			return
		}
		if request.OccurredAt.Before(season.StartAt) || request.OccurredAt.After(season.EndAt) {
			writeErrorResponse(w, http.StatusBadRequest, "The interview date must fall within the selected season.")
			return
		}
		for _, participantID := range []string{storedInterview.InterviewerID, storedInterview.IntervieweeID} {
			enrollments, err := a.db.ListEnrollmentsForUser(r.Context(), participantID)
			if err != nil {
				a.writeStoreErrorResponse(w, err)
				return
			}
			if !programme.IsEnrolledInSeason(enrollments, season.ID) {
				writeErrorResponse(w, http.StatusBadRequest, "Both participants must belong to the selected season.")
				return
			}
		}
	}
	now := time.Now().UTC()
	input := mockinterviews.UpdateInput{
		OccurredAt:      request.OccurredAt,
		DurationMinutes: request.DurationMinutes,
		Notes:           request.Notes,
		Rounds:          mockRoundsFromRequest(request.Rounds),
	}
	storedInterview, err = a.mockRules.Update(storedInterview, actor.UserID, input)
	if err != nil {
		a.writeMockErrorResponse(w, err)
		return
	}
	storedInterview, err = a.db.UpdateMockInterview(r.Context(), storedInterview, actor.UserID, now)
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}

	summaries, err := a.db.ListMockParticipantSummaries(r.Context(), []string{storedInterview.InterviewerID, storedInterview.IntervieweeID})
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}
	interview := withMockParticipantSummaries(storedInterview, summaries)
	response := buildMockInterviewResponse(interview)
	writeJSONResponse(w, http.StatusOK, response)
}

func (a *API) deleteMockInterview(w http.ResponseWriter, r *http.Request) {
	actor := actorFrom(r.Context())
	memberStatus, err := a.db.GetMemberStatus(r.Context(), actor.UserID)
	if err != nil && !errors.Is(err, dal.ErrNotFound) {
		a.writeStoreErrorResponse(w, err)
		return
	}
	eligible := err == nil && memberStatus.CanAccessProgramme()
	if !eligible && !actor.IsDirectorOrSystemAdmin() {
		writeErrorResponse(w, http.StatusForbidden, "Only active members and alumni may delete mock interviews.")
		return
	}

	storedInterview, err := a.db.GetMockInterview(r.Context(), r.PathValue("id"))
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}
	if actor.UserID != storedInterview.InterviewerID {
		writeErrorResponse(w, http.StatusNotFound, "The requested resource does not exist.")
		return
	}
	if storedInterview.SeasonID != nil {
		season, err := a.db.GetSeason(r.Context(), *storedInterview.SeasonID)
		if err != nil {
			a.writeStoreErrorResponse(w, err)
			return
		}
		if season.Status != "open" {
			writeErrorResponse(w, http.StatusConflict, "Season-linked mock interview records are locked until the season is reopened.")
			return
		}
	}
	now := time.Now().UTC()
	storedInterview, err = a.mockRules.Delete(storedInterview, actor.UserID, now)
	if err != nil {
		a.writeMockErrorResponse(w, err)
		return
	}
	if _, err = a.db.DeleteMockInterview(r.Context(), storedInterview, actor.UserID, now); err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type reviewMockInterviewRoundRequest struct {
	Reviewed bool   `json:"reviewed"`
	Comment  string `json:"comment"`
}

func (a *API) reviewMockInterviewRound(w http.ResponseWriter, r *http.Request) {
	actor := actorFrom(r.Context())
	memberStatus, err := a.db.GetMemberStatus(r.Context(), actor.UserID)
	if err != nil && !errors.Is(err, dal.ErrNotFound) {
		a.writeStoreErrorResponse(w, err)
		return
	}
	eligible := err == nil && memberStatus.CanAccessProgramme()
	if !eligible && !actor.IsDirectorOrSystemAdmin() {
		writeErrorResponse(w, http.StatusForbidden, "Only active members and alumni may review mock interviews.")
		return
	}
	var request reviewMockInterviewRoundRequest
	if err := decodeJSON(r.Body, &request); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	storedInterview, err := a.db.GetMockInterview(r.Context(), r.PathValue("id"))
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}
	if actor.UserID != storedInterview.IntervieweeID {
		writeErrorResponse(w, http.StatusNotFound, "The requested resource does not exist.")
		return
	}
	if storedInterview.SeasonID != nil {
		season, err := a.db.GetSeason(r.Context(), *storedInterview.SeasonID)
		if err != nil {
			a.writeStoreErrorResponse(w, err)
			return
		}
		if season.Status != "open" {
			writeErrorResponse(w, http.StatusConflict, "Season-linked mock interview records are locked until the season is reopened.")
			return
		}
	}
	now := time.Now().UTC()
	storedInterview, err = a.mockRules.ReviewRound(storedInterview, mockinterviews.ReviewRoundInput{ActorID: actor.UserID, RoundID: r.PathValue("roundId"), Comment: request.Comment, Reviewed: request.Reviewed})
	if err != nil {
		a.writeMockErrorResponse(w, err)
		return
	}
	storedInterview, err = a.db.ReviewMockInterviewRound(r.Context(), dal.ReviewMockInterviewRoundInput{
		Interview: storedInterview,
		RoundID:   r.PathValue("roundId"),
		ActorID:   actor.UserID,
		ChangedAt: now,
	})
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}

	summaries, err := a.db.ListMockParticipantSummaries(r.Context(), []string{storedInterview.InterviewerID, storedInterview.IntervieweeID})
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}
	interview := withMockParticipantSummaries(storedInterview, summaries)
	response := buildMockInterviewResponse(interview)
	writeJSONResponse(w, http.StatusOK, response)
}

type correctMockInterviewIdentitiesRequest struct {
	InterviewerID string  `json:"interviewerId"`
	IntervieweeID string  `json:"intervieweeId"`
	SeasonID      *string `json:"seasonId"`
	Reason        string  `json:"reason"`
}

func (a *API) correctMockInterviewIdentities(w http.ResponseWriter, r *http.Request) {
	actor := actorFrom(r.Context())
	if !actor.IsDirectorOrSystemAdmin() || !actor.HasRecentMFA(time.Now()) {
		writeErrorResponse(w, http.StatusForbidden, "Director or System Admin access with recent MFA is required.")
		return
	}
	var request correctMockInterviewIdentitiesRequest
	if err := decodeJSON(r.Body, &request); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	if request.InterviewerID == "" || request.IntervieweeID == "" || request.InterviewerID == request.IntervieweeID || request.Reason == "" {
		writeErrorResponse(w, http.StatusBadRequest, "different interviewerId and intervieweeId, reason, are required")
		return
	}

	storedInterview, err := a.db.GetMockInterview(r.Context(), r.PathValue("id"))
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}

	for _, userID := range []string{request.InterviewerID, request.IntervieweeID} {
		participant, err := a.db.GetMemberStatus(r.Context(), userID)
		if err != nil && !errors.Is(err, dal.ErrNotFound) {
			a.writeStoreErrorResponse(w, err)
			return
		}
		if err != nil || !participant.CanBeMockInterviewParticipant() {
			writeErrorResponse(w, http.StatusBadRequest, "Corrected participants must be active members or alumni.")
			return
		}
	}
	if request.SeasonID != nil {
		season, err := a.db.GetSeason(r.Context(), *request.SeasonID)
		if err != nil {
			if errors.Is(err, dal.ErrNotFound) {
				writeErrorResponse(w, http.StatusBadRequest, "The selected season must exist.")
			} else {
				a.writeStoreErrorResponse(w, err)
			}
			return
		}
		if season.Status != "open" {
			writeErrorResponse(w, http.StatusConflict, "Season-linked mock interview records are locked until the season is reopened.")
			return
		}
		if storedInterview.OccurredAt.Before(season.StartAt) || storedInterview.OccurredAt.After(season.EndAt) {
			writeErrorResponse(w, http.StatusBadRequest, "The interview date must fall within the selected season.")
			return
		}
		for _, participantID := range []string{request.InterviewerID, request.IntervieweeID} {
			enrollments, err := a.db.ListEnrollmentsForUser(r.Context(), participantID)
			if err != nil {
				a.writeStoreErrorResponse(w, err)
				return
			}
			if !programme.IsEnrolledInSeason(enrollments, season.ID) {
				writeErrorResponse(w, http.StatusBadRequest, "Both participants must belong to the selected season.")
				return
			}
		}
	}
	now := time.Now().UTC()
	storedInterview, err = a.mockRules.CorrectIdentities(storedInterview, mockinterviews.CorrectIdentitiesInput{InterviewerID: request.InterviewerID, IntervieweeID: request.IntervieweeID, SeasonID: request.SeasonID, Reason: request.Reason})
	if err != nil {
		a.writeMockErrorResponse(w, err)
		return
	}
	storedInterview, err = a.db.CorrectMockInterviewIdentities(r.Context(), dal.CorrectMockInterviewIdentitiesInput{
		Interview: storedInterview,
		ActorID:   actor.UserID,
		Reason:    request.Reason,
		ChangedAt: now,
	})
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}

	summaries, err := a.db.ListMockParticipantSummaries(r.Context(), []string{storedInterview.InterviewerID, storedInterview.IntervieweeID})
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}
	interview := withMockParticipantSummaries(storedInterview, summaries)
	response := buildMockInterviewResponse(interview)
	writeJSONResponse(w, http.StatusOK, response)
}

func (a *API) writeMockErrorResponse(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, mockinterviews.ErrForbidden):
		writeErrorResponse(w, http.StatusForbidden, "The current account cannot perform this action.")
	case errors.Is(err, mockinterviews.ErrInvalid):
		writeErrorResponse(w, http.StatusBadRequest, err.Error())
	default:
		a.logger.Error("mock interview operation failed", "error", err)
		writeErrorResponse(w, http.StatusInternalServerError, "The request could not be completed.")
	}
}

type mockInterviewResponse struct {
	Interview mockinterviews.Interview `json:"interview"`
	Passed    bool                     `json:"passed"`
}

func buildMockInterviewResponse(interview mockinterviews.Interview) mockInterviewResponse {
	return mockInterviewResponse{Interview: interview, Passed: mockinterviews.Passed(interview)}
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
