package api

import (
	"net/http"
	"time"

	"github.com/magedmg/RSP-website/backend/internal/dal"
	"github.com/magedmg/RSP-website/backend/internal/platform/id"
	"github.com/magedmg/RSP-website/backend/internal/programme"
)

func (a *API) listWeeks(w http.ResponseWriter, r *http.Request) {
	actor := actorFrom(r.Context())
	seasonID := r.PathValue("id")
	if !actor.CanViewSeason(seasonID) {
		writeErrorResponse(w, http.StatusForbidden, "The current account cannot access this season.")
		return
	}
	if _, err := a.db.GetSeason(r.Context(), seasonID); err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}

	sortBy, err := parseSort(r.URL.Query().Get("sort"), "number:asc", "number:asc", "id:asc")
	if err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "sort must be number:asc or id:asc")
		return
	}

	direction, err := parsePageDirection(r.URL.Query().Get("direction"))
	if err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "direction must be forward or backward")
		return
	}

	binding := "weeks|season=" + seasonID + "|sort=" + sortBy
	limit, boundary, err := parsePagination(r.URL.Query().Get("limit"), r.URL.Query().Get("cursor"), binding, a.cursorSecret)
	if err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "The cursor does not match the selected season and sort.")
		return
	}

	items, more, total, err := a.db.ListWeeks(r.Context(), dal.WeekQuery{
		SeasonID:  seasonID,
		Boundary:  boundary,
		Limit:     limit,
		SortBy:    sortBy,
		Direction: direction,
	})
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}

	pageInfo, err := pageInfoForKeyset(
		a.cursorSecret, binding, direction, boundary, items, more,
		func(week programme.WeekRecord) string { return week.ID },
	)
	if err != nil {
		a.logger.Error("cursor encoding failed", "error", err)
		writeErrorResponse(w, http.StatusInternalServerError, "The request could not be completed.")
		return
	}
	response := Page[programme.WeekRecord]{
		Items:      items,
		PageInfo:   pageInfo,
		TotalCount: total,
	}
	writeJSONResponse(w, http.StatusOK, response)
}

type createWeekRequest struct {
	Number      int       `json:"number"`
	StartAt     time.Time `json:"startAt"`
	EndAt       time.Time `json:"endAt"`
	ResourceURL string    `json:"resourceUrl"`
}

func (a *API) createWeek(w http.ResponseWriter, r *http.Request) {
	actor := actorFrom(r.Context())
	season, err := a.db.GetSeason(r.Context(), r.PathValue("id"))
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}
	if !actor.IsSeasonAdmin(season.ID) {
		writeErrorResponse(w, http.StatusForbidden, "Season administrator access is required.")
		return
	}
	if season.Status != "open" {
		writeErrorResponse(w, http.StatusForbidden, "The season must be open.")
		return
	}
	if !actor.HasRecentMFA(time.Now()) {
		writeErrorResponse(w, http.StatusForbidden, "Recent MFA is required.")
		return
	}
	var request createWeekRequest
	if err := decodeJSON(r.Body, &request); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	if request.Number < 1 {
		writeErrorResponse(w, http.StatusBadRequest, "number must be at least 1")
		return
	}
	if !request.EndAt.After(request.StartAt) {
		writeErrorResponse(w, http.StatusBadRequest, "endAt must be after startAt")
		return
	}
	if request.ResourceURL != "" && !isValidHTTPSURL(request.ResourceURL) {
		writeErrorResponse(w, http.StatusBadRequest, "resourceUrl must be an HTTPS URL")
		return
	}
	if request.StartAt.Before(season.StartAt) || request.EndAt.After(season.EndAt) {
		writeErrorResponse(w, http.StatusBadRequest, "week dates must fall within the season dates")
		return
	}
	week := programme.WeekRecord{
		ID:          id.New(),
		SeasonID:    r.PathValue("id"),
		Number:      request.Number,
		StartAt:     request.StartAt.UTC(),
		EndAt:       request.EndAt.UTC(),
		ResourceURL: request.ResourceURL,
	}
	created, err := a.db.CreateWeek(r.Context(), week, actor.UserID, time.Now().UTC())
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}

	writeJSONResponse(w, http.StatusCreated, created)
}

type updateWeekRequest struct {
	Number      int       `json:"number"`
	StartAt     time.Time `json:"startAt"`
	EndAt       time.Time `json:"endAt"`
	ResourceURL string    `json:"resourceUrl"`
}

func (a *API) updateWeek(w http.ResponseWriter, r *http.Request) {
	actor := actorFrom(r.Context())
	season, err := a.db.GetSeason(r.Context(), r.PathValue("id"))
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}
	if !actor.IsSeasonAdmin(season.ID) {
		writeErrorResponse(w, http.StatusForbidden, "Season administrator access is required.")
		return
	}
	if season.Status != "open" {
		writeErrorResponse(w, http.StatusForbidden, "The season must be open.")
		return
	}
	if !actor.HasRecentMFA(time.Now()) {
		writeErrorResponse(w, http.StatusForbidden, "Recent MFA is required.")
		return
	}
	var request updateWeekRequest
	if err := decodeJSON(r.Body, &request); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	if request.Number < 1 {
		writeErrorResponse(w, http.StatusBadRequest, "number must be at least 1")
		return
	}
	if !request.EndAt.After(request.StartAt) {
		writeErrorResponse(w, http.StatusBadRequest, "endAt must be after startAt")
		return
	}
	if request.ResourceURL != "" && !isValidHTTPSURL(request.ResourceURL) {
		writeErrorResponse(w, http.StatusBadRequest, "resourceUrl must be an HTTPS URL")
		return
	}
	if request.StartAt.Before(season.StartAt) || request.EndAt.After(season.EndAt) {
		writeErrorResponse(w, http.StatusBadRequest, "week dates must fall within the season dates")
		return
	}
	week, err := a.db.UpdateWeek(r.Context(), dal.UpdateWeekInput{
		SeasonID:  r.PathValue("id"),
		WeekID:    r.PathValue("weekId"),
		Week:      programme.WeekRecord{Number: request.Number, StartAt: request.StartAt.UTC(), EndAt: request.EndAt.UTC(), ResourceURL: request.ResourceURL},
		ActorID:   actor.UserID,
		ChangedAt: time.Now().UTC(),
	})
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}

	writeJSONResponse(w, http.StatusOK, week)
}

func (a *API) deleteWeek(w http.ResponseWriter, r *http.Request) {
	actor := actorFrom(r.Context())
	managedSeason, err := a.db.GetSeason(r.Context(), r.PathValue("id"))
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}
	if !actor.IsSeasonAdmin(managedSeason.ID) {
		writeErrorResponse(w, http.StatusForbidden, "Season administrator access is required.")
		return
	}
	if managedSeason.Status != "open" {
		writeErrorResponse(w, http.StatusForbidden, "The season must be open.")
		return
	}
	if !actor.HasRecentMFA(time.Now()) {
		writeErrorResponse(w, http.StatusForbidden, "Recent MFA is required.")
		return
	}
	if err := a.db.DeleteWeek(r.Context(), dal.DeleteWeekInput{
		SeasonID:  r.PathValue("id"),
		WeekID:    r.PathValue("weekId"),
		ActorID:   actor.UserID,
		ChangedAt: time.Now().UTC(),
	}); err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
