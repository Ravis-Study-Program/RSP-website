package api

import (
	"errors"
	"github.com/magedmg/RSP-website/backend/internal/dal"
	"github.com/magedmg/RSP-website/backend/internal/practice"
	"net/http"
)

func (a *API) suggestProblem(w http.ResponseWriter, r *http.Request) {
	actor := actorFrom(r.Context())
	if !actor.CanRecordActivity() {
		writeErrorResponse(w, http.StatusForbidden, "No practice access yet.")
		return
	}
	difficulties, err := selection(r.URL.Query(), "difficulty", difficultySelection)
	if err != nil {
		writeErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	categories, err := selection(r.URL.Query(), "category", nil)
	if err != nil {
		writeErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	result, err := a.db.SuggestProblem(r.Context(), actor.UserID, difficulties, categories)
	var problem *practice.ProblemRecord
	if errors.Is(err, dal.ErrNotFound) {
		problem = nil
	} else if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	} else {
		problem = &result
	}
	writeJSONResponse(w, http.StatusOK, struct {
		Problem *practice.ProblemRecord `json:"problem"`
	}{problem})
}
