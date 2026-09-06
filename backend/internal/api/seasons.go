package api

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/magedmg/RSP-website/backend/internal/dal"
	"github.com/magedmg/RSP-website/backend/internal/platform/cursor"
	"github.com/magedmg/RSP-website/backend/internal/platform/id"
	"github.com/magedmg/RSP-website/backend/internal/programme"
)

func (a *API) listSeasons(w http.ResponseWriter, r *http.Request) {
	actor := actorFrom(r.Context())
	if !actor.CanAccessProgramme() && !actor.IsDirectorOrSystemAdmin() {
		writeErrorResponse(w, http.StatusForbidden, "No season access yet.")
		return
	}

	_, err := parseSort(r.URL.Query().Get("sort"), "id:asc", "id:asc")
	if err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "sort must be id:asc")
		return
	}

	direction := r.URL.Query().Get("direction")
	if direction == "" {
		direction = "forward"
	}
	if direction != "forward" && direction != "backward" {
		writeErrorResponse(w, http.StatusBadRequest, "direction must be forward or backward")
		return
	}

	status := r.URL.Query().Get("status")
	if status != "" && status != "open" && status != "closed" {
		writeErrorResponse(w, http.StatusBadRequest, "status must be open or closed")
		return
	}

	binding := "seasons|sort=id:asc|status=" + status
	if actor.IsDirectorOrSystemAdmin() {
		limit, boundary, pageErr := parsePagination(r.URL.Query().Get("limit"), r.URL.Query().Get("cursor"), binding, a.cursorSecret)
		if pageErr != nil {
			writeErrorResponse(w, http.StatusBadRequest, cursor.ErrInvalid.Error())
			return
		}
		items, more, total, listErr := a.db.ListSeasons(r.Context(), dal.SeasonQuery{
			Boundary:  boundary,
			Limit:     limit,
			Direction: direction,
			Status:    status,
		})
		if listErr != nil {
			a.writeStoreErrorResponse(w, listErr)
			return
		}
		pageInfo, err := pageInfoForKeyset(
			a.cursorSecret, binding, direction, boundary, items, more,
			func(seasonRecord programme.SeasonRecord) string { return seasonRecord.ID },
		)
		if err != nil {
			a.logger.Error("cursor encoding failed", "error", err)
			writeErrorResponse(w, http.StatusInternalServerError, "The request could not be completed.")
			return
		}
		response := Page[programme.SeasonRecord]{
			Items:      items,
			PageInfo:   pageInfo,
			TotalCount: total,
		}
		writeJSONResponse(w, http.StatusOK, response)
		return
	}

	items := []programme.SeasonRecord{}
	seen := map[string]bool{}
	for _, enrollment := range actor.Enrollments {
		if seen[enrollment.SeasonID] || !actor.CanViewSeason(enrollment.SeasonID) {
			continue
		}
		season, seasonErr := a.db.GetSeason(r.Context(), enrollment.SeasonID)
		if seasonErr != nil {
			a.writeStoreErrorResponse(w, seasonErr)
			return
		}
		if status != "" && season.Status != status {
			continue
		}
		seen[season.ID] = true
		items = append(items, season)
	}

	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	limit, boundary, err := parsePagination(r.URL.Query().Get("limit"), r.URL.Query().Get("cursor"), binding, a.cursorSecret)
	if err != nil {
		writeErrorResponse(w, http.StatusBadRequest, cursor.ErrInvalid.Error())
		return
	}
	page, pageInfo, err := paginateOrdered(a.cursorSecret, binding, direction, boundary, limit, items, func(seasonRecord programme.SeasonRecord) string { return seasonRecord.ID })
	if err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "The cursor does not match the selected season sort.")
		return
	}

	response := Page[programme.SeasonRecord]{
		Items:      page,
		PageInfo:   pageInfo,
		TotalCount: int64(len(items)),
	}
	writeJSONResponse(w, http.StatusOK, response)
}

func (a *API) getSeason(w http.ResponseWriter, r *http.Request) {
	actor := actorFrom(r.Context())
	if !actor.CanViewSeason(r.PathValue("id")) {
		writeErrorResponse(w, http.StatusNotFound, "The requested season does not exist.")
		return
	}

	seasonRecord, err := a.db.GetSeason(r.Context(), r.PathValue("id"))
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}

	writeJSONResponse(w, http.StatusOK, seasonRecord)
}

type seasonInput struct {
	Name         string    `json:"name"`
	Slug         string    `json:"slug"`
	Location     string    `json:"location"`
	ImageURL     string    `json:"imageUrl"`
	ResourcesURL string    `json:"resourcesUrl"`
	StartAt      time.Time `json:"startAt"`
	EndAt        time.Time `json:"endAt"`
}

func validateSeason(request seasonInput) error {
	if strings.TrimSpace(request.Name) == "" {
		return fmt.Errorf("name is required")
	}
	if !validSeasonSlug(request.Slug) {
		return fmt.Errorf("slug must be 1-50 lowercase letters or numbers separated by single hyphens")
	}
	if !request.EndAt.After(request.StartAt) {
		return fmt.Errorf("endAt must be after startAt")
	}
	if request.ImageURL != "" && !isValidHTTPSURL(request.ImageURL) {
		return fmt.Errorf("imageUrl must be an HTTPS URL")
	}
	if request.ResourcesURL != "" && !isValidHTTPSURL(request.ResourcesURL) {
		return fmt.Errorf("resourcesUrl must be an HTTPS URL")
	}
	return nil
}

func validSeasonSlug(slug string) bool {
	if len(slug) < 1 || len(slug) > 50 || slug[0] == '-' || slug[len(slug)-1] == '-' {
		return false
	}
	previousHyphen := false
	for _, ch := range slug {
		if ch == '-' {
			if previousHyphen {
				return false
			}
			previousHyphen = true
			continue
		}
		if !((ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9')) {
			return false
		}
		previousHyphen = false
	}
	return true
}

func (a *API) createSeason(w http.ResponseWriter, r *http.Request) {
	actor := actorFrom(r.Context())
	if !actor.IsDirectorOrSystemAdmin() || !actor.HasRecentMFA(time.Now()) {
		writeErrorResponse(w, http.StatusForbidden, "Director or System Admin access with recent MFA is required.")
		return
	}
	var request seasonInput
	if err := decodeJSON(r.Body, &request); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := validateSeason(request); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	seasonRecord := programme.SeasonRecord{
		ID:           id.New(),
		Slug:         request.Slug,
		Name:         request.Name,
		Status:       "open",
		StartAt:      request.StartAt.UTC(),
		EndAt:        request.EndAt.UTC(),
		Location:     request.Location,
		ImageURL:     request.ImageURL,
		ResourcesURL: request.ResourcesURL,
	}
	created, err := a.db.CreateSeason(r.Context(), seasonRecord, actor.UserID, time.Now().UTC())
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}

	writeJSONResponse(w, http.StatusCreated, created)
}

func (a *API) updateSeason(w http.ResponseWriter, r *http.Request) {
	actor := actorFrom(r.Context())
	seasonID := r.PathValue("id")
	if !actor.IsDirectorOrSystemAdmin() || !actor.HasRecentMFA(time.Now()) {
		writeErrorResponse(w, http.StatusForbidden, "Director or System Admin access with recent MFA is required.")
		return
	}

	current, err := a.db.GetSeason(r.Context(), seasonID)
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}

	if current.Status != "open" {
		writeErrorResponse(w, http.StatusConflict, "The season must be reopened before its definition can be changed.")
		return
	}

	var request seasonInput
	if err := decodeJSON(r.Body, &request); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := validateSeason(request); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	updated, err := a.db.UpdateSeasonDefinition(r.Context(), dal.UpdateSeasonDefinitionInput{
		SeasonID:     seasonID,
		Name:         request.Name,
		Slug:         request.Slug,
		Location:     request.Location,
		ImageURL:     request.ImageURL,
		ResourcesURL: request.ResourcesURL,
		StartAt:      request.StartAt.UTC(),
		EndAt:        request.EndAt.UTC(),
		ActorID:      actor.UserID,
		ChangedAt:    time.Now().UTC(),
	})
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}

	writeJSONResponse(w, http.StatusOK, updated)
}

type updateSeasonResourcesRequest struct {
	ResourcesURL string `json:"resourcesUrl"`
}

func (a *API) updateSeasonResources(w http.ResponseWriter, r *http.Request) {
	actor := actorFrom(r.Context())
	seasonID := r.PathValue("id")
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

	var request updateSeasonResourcesRequest
	if err := decodeJSON(r.Body, &request); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	if request.ResourcesURL != "" && !isValidHTTPSURL(request.ResourcesURL) {
		writeErrorResponse(w, http.StatusBadRequest, "an HTTPS resourcesUrl is required")
		return
	}
	updated, err := a.db.UpdateSeasonResources(r.Context(), dal.UpdateSeasonResourcesInput{
		SeasonID:     seasonID,
		ResourcesURL: request.ResourcesURL,
		ActorID:      actor.UserID,
		ChangedAt:    time.Now().UTC(),
	})
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}

	writeJSONResponse(w, http.StatusOK, updated)
}

type closeSeasonRequest struct {
	Reason string `json:"reason"`
}

func (a *API) closeSeason(w http.ResponseWriter, r *http.Request) {
	actor := actorFrom(r.Context())
	seasonID := r.PathValue("id")
	season, err := a.db.GetSeason(r.Context(), seasonID)
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}

	if !actor.CanCloseSeason(seasonID) || !actor.HasRecentMFA(time.Now()) {
		writeErrorResponse(w, http.StatusForbidden, "Recent MFA and season administration access are required.")
		return
	}

	if season.Status != "open" {
		writeErrorResponse(w, http.StatusConflict, "The season is already closed.")
		return
	}

	var request closeSeasonRequest
	if err := decodeJSON(r.Body, &request); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	if strings.TrimSpace(request.Reason) == "" {
		writeErrorResponse(w, http.StatusBadRequest, "reason is required")
		return
	}

	updated, err := a.db.CloseSeason(r.Context(), dal.CloseSeasonInput{
		SeasonID:  seasonID,
		ActorID:   actor.UserID,
		Reason:    strings.TrimSpace(request.Reason),
		ChangedAt: time.Now().UTC(),
	})
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}

	writeJSONResponse(w, http.StatusOK, updated)
}

type reopenSeasonRequest struct {
	Reason string `json:"reason"`
}

func (a *API) reopenSeason(w http.ResponseWriter, r *http.Request) {
	actor := actorFrom(r.Context())
	if !actor.CanReopenSeason(time.Now()) {
		writeErrorResponse(w, http.StatusForbidden, "Director or System Admin access with recent MFA is required.")
		return
	}

	seasonID := r.PathValue("id")
	var request reopenSeasonRequest
	if err := decodeJSON(r.Body, &request); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	if strings.TrimSpace(request.Reason) == "" {
		writeErrorResponse(w, http.StatusBadRequest, "reason is required")
		return
	}

	updated, err := a.db.ReopenSeason(r.Context(), dal.ReopenSeasonInput{
		SeasonID:  seasonID,
		ActorID:   actor.UserID,
		Reason:    strings.TrimSpace(request.Reason),
		ChangedAt: time.Now().UTC(),
	})
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}

	writeJSONResponse(w, http.StatusOK, updated)
}
