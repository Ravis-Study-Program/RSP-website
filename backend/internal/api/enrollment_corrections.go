package api

import (
	"github.com/magedmg/RSP-website/backend/internal/dal"
	"net/http"
	"time"
)

func (a *API) correctEnrollment(w http.ResponseWriter, r *http.Request) {
	actor := actorFrom(r.Context())
	source := r.PathValue("id")
	if !actor.IsSeasonAdmin(source) || !actor.HasRecentMFA(time.Now()) {
		writeErrorResponse(w, 403, "Season administrator access with recent MFA is required.")
		return
	}
	var request struct {
		UserID   string `json:"userId"`
		SeasonID string `json:"seasonId"`
	}
	if err := decodeJSON(r.Body, &request); err != nil {
		writeErrorResponse(w, 400, err.Error())
		return
	}
	if !uuidSelection(request.UserID) || !uuidSelection(request.SeasonID) {
		writeErrorResponse(w, 400, "Choose a valid person and season.")
		return
	}
	if !actor.IsSeasonAdmin(request.SeasonID) {
		writeErrorResponse(w, 403, "Administrator access to the destination season is required.")
		return
	}
	updated, err := a.db.CorrectEnrollment(r.Context(), dal.CorrectEnrollmentInput{EnrollmentID: r.PathValue("memberId"), SourceSeasonID: source, SeasonID: request.SeasonID, UserID: request.UserID, ActorID: actor.UserID, ChangedAt: time.Now().UTC()})
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}
	writeJSONResponse(w, 200, updated)
}
