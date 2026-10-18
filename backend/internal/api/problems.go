package api

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/magedmg/RSP-website/backend/internal/dal"
	"github.com/magedmg/RSP-website/backend/internal/mockinterviews"
	"github.com/magedmg/RSP-website/backend/internal/platform/cursor"
	"github.com/magedmg/RSP-website/backend/internal/practice"
)

const maxProblemSearchLength = 100

func (a *API) listLeetCodeProblems(w http.ResponseWriter, r *http.Request) {
	actor := actorFrom(r.Context())
	if !actor.CanRecordActivity() && !actor.IsDirectorOrSystemAdmin() {
		writeErrorResponse(w, http.StatusForbidden, "No season access yet.")
		return
	}

	difficulty, category := r.URL.Query().Get("difficulty"), r.URL.Query().Get("category")
	search := strings.TrimSpace(r.URL.Query().Get("q"))
	if utf8.RuneCountInString(search) > maxProblemSearchLength {
		writeErrorResponse(w, http.StatusBadRequest, "q must be at most 100 characters")
		return
	}
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

	binding := "problems|id:asc|difficulty=" + difficulty + "|category=" + category + "|premium=" + r.URL.Query().Get("premium") + "|q=" + search
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
		Search:     search,
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

func (a *API) listLeetCodeCategories(w http.ResponseWriter, r *http.Request) {
	actor := actorFrom(r.Context())
	if !actor.CanRecordActivity() && !actor.IsDirectorOrSystemAdmin() {
		writeErrorResponse(w, http.StatusForbidden, "No season access yet.")
		return
	}
	items, err := a.db.ListProblemCategories(r.Context())
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}
	writeJSONResponse(w, http.StatusOK, struct {
		Items []string `json:"items"`
	}{Items: items})
}

// attachAttemptProblems embeds the catalogue entry of each attempt so clients
// never need the full catalogue to label history.
func (a *API) attachAttemptProblems(ctx context.Context, attempts []practice.AttemptRecord) error {
	ids := make([]string, 0, len(attempts))
	for _, attempt := range attempts {
		ids = append(ids, attempt.ProblemID)
	}
	problems, err := a.db.ProblemsByID(ctx, ids)
	if err != nil {
		return err
	}
	for index := range attempts {
		if problem, ok := problems[attempts[index].ProblemID]; ok {
			attempts[index].Problem = &problem
		}
	}
	return nil
}

// attachRoundProblems embeds the catalogue entry of each LeetCode round.
func (a *API) attachRoundProblems(ctx context.Context, interviews []mockinterviews.Interview) error {
	ids := []string{}
	for _, interview := range interviews {
		for _, round := range interview.Rounds {
			if round.ProblemID != "" {
				ids = append(ids, round.ProblemID)
			}
		}
	}
	problems, err := a.db.ProblemsByID(ctx, ids)
	if err != nil {
		return err
	}
	for i := range interviews {
		for j := range interviews[i].Rounds {
			if problem, ok := problems[interviews[i].Rounds[j].ProblemID]; ok {
				interviews[i].Rounds[j].Problem = &problem
			}
		}
	}
	return nil
}
