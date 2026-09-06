package api

import (
	"net/http"

	"github.com/magedmg/RSP-website/backend/internal/programme"
)

func (a *API) getUserParticipation(w http.ResponseWriter, r *http.Request) {
	actor := actorFrom(r.Context())
	seasonID, _, err := activityFilters(r, actor)
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
	items, err := a.db.ListUserParticipation(r.Context(), r.PathValue("id"))
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}
	visible := []programme.Participation{}
	for _, item := range items {
		if actor.UserID == item.UserID || actor.CanViewSeason(item.SeasonID) {
			visible = append(visible, item)
		}
	}
	writeJSONResponse(w, http.StatusOK, Page[programme.Participation]{Items: visible, TotalCount: int64(len(visible))})
}
