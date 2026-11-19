package api

import (
	"net/http"

	"github.com/Ravis-Study-Program/RSP-website/backend/internal/program"
)

func (a *API) getUserParticipation(w http.ResponseWriter, r *http.Request) {
	actor := actorFrom(r.Context())
	seasonID, _, err := activityFilters(r, actor)
	if err != nil {
		writeErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	if !actor.CanRecordActivity() {
		writeErrorResponse(w, http.StatusForbidden, "Program membership is required.")
		return
	}
	if !a.authorizeActivityRead(w, r, r.PathValue("id"), seasonID) {
		return
	}
	items, err := a.db.ListUserParticipation(r.Context(), r.PathValue("id"))
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}
	visible := []program.Participation{}
	for _, item := range items {
		if actor.UserID == item.UserID || actor.CanViewSeason(item.SeasonID) {
			visible = append(visible, item)
		}
	}
	writeJSONResponse(w, http.StatusOK, Page[program.Participation]{Items: visible, TotalCount: int64(len(visible))})
}
