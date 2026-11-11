package api

import (
	"net/http"
	"time"
)

// deleteSeason removes a season with all of its members, weeks and
// invitations, whatever has happened in it.
func (a *API) deleteSeason(w http.ResponseWriter, r *http.Request) {
	actor := actorFrom(r.Context())
	if !actor.IsDirectorOrSystemAdmin() || !actor.HasRecentMFA(time.Now()) {
		writeErrorResponse(w, 403, "Director or System Admin access with recent MFA is required.")
		return
	}
	if err := a.db.DeleteSeason(r.Context(), r.PathValue("id"), actor.UserID, time.Now().UTC()); err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
