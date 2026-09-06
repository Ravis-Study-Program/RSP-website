package api

import (
	"net/http"
	"strconv"

	"github.com/magedmg/RSP-website/backend/internal/dal"
	"github.com/magedmg/RSP-website/backend/internal/platform/cursor"
	"github.com/magedmg/RSP-website/backend/internal/practice"
)

func (a *API) listLeetCodeProblems(w http.ResponseWriter, r *http.Request) {
	actor := actorFrom(r.Context())
	if !actor.CanAccessProgramme() && !actor.IsDirectorOrSystemAdmin() {
		writeErrorResponse(w, http.StatusForbidden, "No season access yet.")
		return
	}

	difficulty, category := r.URL.Query().Get("difficulty"), r.URL.Query().Get("category")
	var premium *bool
	if raw := r.URL.Query().Get("premium"); raw != "" {
		parsed, parseErr := strconv.ParseBool(raw)
		if parseErr != nil {
			writeErrorResponse(w, http.StatusBadRequest, "premium must be true or false")
			return
		}
		premium = &parsed
	}
	if _, err := parseSort(r.URL.Query().Get("sort"), "id:asc", "id:asc"); err != nil {
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

	binding := "problems|id:asc|difficulty=" + difficulty + "|category=" + category + "|premium=" + r.URL.Query().Get("premium")
	limit, after, err := parsePagination(r.URL.Query().Get("limit"), r.URL.Query().Get("cursor"), binding, a.cursorSecret)
	if err != nil {
		writeErrorResponse(w, http.StatusBadRequest, cursor.ErrInvalid.Error())
		return
	}

	items, more, total, err := a.db.ListProblems(r.Context(), dal.ProblemQuery{
		Boundary:   after,
		Limit:      limit,
		Difficulty: difficulty,
		Category:   category,
		Premium:    premium,
		Direction:  direction,
	})
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}

	pageInfo, err := pageInfoForKeyset(
		a.cursorSecret, binding, direction, after, items, more,
		func(record practice.ProblemRecord) string { return record.ID },
	)
	if err != nil {
		a.logger.Error("cursor encoding failed", "error", err)
		writeErrorResponse(w, http.StatusInternalServerError, "The request could not be completed.")
		return
	}
	response := Page[practice.ProblemRecord]{
		Items:      items,
		PageInfo:   pageInfo,
		TotalCount: total,
	}
	writeJSONResponse(w, http.StatusOK, response)
}
