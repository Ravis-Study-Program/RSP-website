package api

import (
	"fmt"
	"net/http"
	"time"

	"github.com/magedmg/RSP-website/backend/internal/dal"
	"github.com/magedmg/RSP-website/backend/internal/practice"
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

// getStudentRequirements returns a student's own goals and weekly hours, for
// the mentors and coordinators who can change them.
func (a *API) getStudentRequirements(w http.ResponseWriter, r *http.Request) {
	actor := actorFrom(r.Context())
	seasonID := r.PathValue("id")
	if !actor.CanSetStudentLevel(seasonID) {
		writeErrorResponse(w, http.StatusForbidden, "An active mentor or administrator for this season is required.")
		return
	}
	own, err := a.db.GetStudentRequirements(r.Context(), seasonID, r.PathValue("memberId"))
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}
	writeJSONResponse(w, http.StatusOK, own)
}

// putStudentRequirements sets a student's own goals and weekly hours; a
// null value follows the program's default.
func (a *API) putStudentRequirements(w http.ResponseWriter, r *http.Request) {
	actor := actorFrom(r.Context())
	seasonID := r.PathValue("id")
	if !actor.CanSetStudentLevel(seasonID) {
		writeErrorResponse(w, http.StatusForbidden, "An active mentor or administrator for this season is required.")
		return
	}
	var request practice.StudentRequirements
	if err := decodeJSON(r.Body, &request); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	if !request.Valid() {
		writeErrorResponse(w, http.StatusBadRequest, fmt.Sprintf("Goals must be 1 to %d minutes and weekly hours 1 to %d.", practice.MaxGoalMinutes, practice.MaxWeeklyHours))
		return
	}
	saved, err := a.db.SetStudentRequirements(r.Context(), seasonID, r.PathValue("memberId"), request, actor.UserID, time.Now().UTC())
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}
	writeJSONResponse(w, http.StatusOK, saved)
}
