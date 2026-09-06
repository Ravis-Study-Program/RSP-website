package api

import (
	"errors"
	"net/http"

	"github.com/magedmg/RSP-website/backend/internal/dal"
)

// authorizeActivityRead checks the target, independently of contact-data access.
// A former student's history can still be reviewed in their season by its staff.
func (a *API) authorizeActivityRead(w http.ResponseWriter, r *http.Request, targetID, seasonID string) bool {
	actor := actorFrom(r.Context())
	if targetID == actor.UserID || actor.IsDirectorOrSystemAdmin() {
		return true
	}
	status, err := a.db.GetMemberStatus(r.Context(), targetID)
	if err != nil && !errors.Is(err, dal.ErrNotFound) {
		a.writeStoreErrorResponse(w, err)
		return false
	}
	if err == nil && actor.CanReadSharedActivity() && status.IsVisibleInDirectory() {
		return true
	}
	if err == nil && !status.Deleted && actor.CanReviewSeason(seasonID) {
		enrollments, err := a.db.ListEnrollmentsForUser(r.Context(), targetID)
		if err != nil {
			a.writeStoreErrorResponse(w, err)
			return false
		}
		for _, enrollment := range enrollments {
			if enrollment.SeasonID == seasonID {
				return true
			}
		}
	}
	writeErrorResponse(w, http.StatusNotFound, "The requested member's activity is not available.")
	return false
}
