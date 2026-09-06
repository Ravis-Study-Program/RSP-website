package api

import (
	"errors"
	"net/http"
	"time"

	"github.com/magedmg/RSP-website/backend/internal/dal"
)

func (a *API) deleteEmptySeason(w http.ResponseWriter, r *http.Request) {
	actor := actorFrom(r.Context())
	if !actor.IsDirectorOrSystemAdmin() || !actor.HasRecentMFA(time.Now()) {
		writeErrorResponse(w, 403, "Director or System Admin access with recent MFA is required.")
		return
	}
	if err := a.db.DeleteEmptySeason(r.Context(), r.PathValue("id"), actor.UserID, time.Now().UTC()); err != nil {
		if errors.Is(err, dal.ErrConflict) {
			writeErrorResponse(w, 409, "This season has enrollment history, recorded activity, or pending invitations and cannot be deleted.")
		} else {
			a.writeStoreErrorResponse(w, err)
		}
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
