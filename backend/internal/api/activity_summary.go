package api

import "net/http"

func (a *API) getUserActivitySummary(w http.ResponseWriter, r *http.Request) {
	actor := actorFrom(r.Context())
	seasonID, year, err := activityFilters(r, actor)
	if err != nil {
		writeErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	if !actor.CanRecordActivity() {
		writeErrorResponse(w, http.StatusForbidden, "Programme membership is required.")
		return
	}
	if !a.authorizeActivityRead(w, r, r.PathValue("id"), seasonID) {
		return
	}
	summary, err := a.db.GetActivitySummary(r.Context(), r.PathValue("id"), seasonID, year)
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}
	writeJSONResponse(w, http.StatusOK, summary)
}

func (a *API) getSeasonActivitySummary(w http.ResponseWriter, r *http.Request) {
	actor := actorFrom(r.Context())
	if !actor.CanViewSeason(r.PathValue("id")) {
		writeErrorResponse(w, http.StatusForbidden, "Season access is required.")
		return
	}
	if _, err := a.db.GetSeason(r.Context(), r.PathValue("id")); err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}
	summary, err := a.db.GetActivitySummary(r.Context(), "", r.PathValue("id"), 0)
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}
	writeJSONResponse(w, http.StatusOK, summary)
}
