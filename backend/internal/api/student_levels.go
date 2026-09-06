package api

import (
	"net/http"
	"time"

	"github.com/magedmg/RSP-website/backend/internal/dal"
)

func (a *API) updateStudentLevel(w http.ResponseWriter, r *http.Request) {
	actor := actorFrom(r.Context())
	seasonID := r.PathValue("id")
	if !actor.CanSetStudentLevel(seasonID) {
		writeErrorResponse(w, http.StatusForbidden, "An active mentor or administrator for this season is required.")
		return
	}
	var request struct {
		StudentLevel string `json:"studentLevel"`
	}
	if err := decodeJSON(r.Body, &request); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	if !isValidStudentLevel(request.StudentLevel) {
		writeErrorResponse(w, http.StatusBadRequest, "studentLevel must be novice, beginner, intermediate, or advanced")
		return
	}
	member, err := a.db.UpdateStudentLevel(r.Context(), dal.UpdateStudentLevelInput{
		SeasonID: seasonID, EnrollmentID: r.PathValue("memberId"),
		StudentLevel: request.StudentLevel, ActorID: actor.UserID, ChangedAt: time.Now().UTC(),
	})
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}
	writeJSONResponse(w, http.StatusOK, member)
}
