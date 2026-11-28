package api

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/Ravis-Study-Program/RSP-website/backend/internal/accounts"
	"github.com/Ravis-Study-Program/RSP-website/backend/internal/authz"
	"github.com/Ravis-Study-Program/RSP-website/backend/internal/dal"
	"github.com/Ravis-Study-Program/RSP-website/backend/internal/practice"
)

// authorizeStudentDetails allows a student's details (level, goals, weekly
// hours) to a season administrator or the student's current mentor, not
// another mentor or a past one. It writes the refusal when it returns false.
func (a *API) authorizeStudentDetails(w http.ResponseWriter, r *http.Request) bool {
	actor := actorFrom(r.Context())
	seasonID := r.PathValue("id")
	if !actor.CanSetStudentLevel(seasonID) {
		writeErrorResponse(w, http.StatusForbidden, "An active mentor or administrator for this season is required.")
		return false
	}
	if actor.IsSeasonAdmin(seasonID) {
		return true
	}
	member, err := a.db.GetEnrollment(r.Context(), r.PathValue("memberId"))
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return false
	}
	if member.SeasonID != seasonID {
		writeErrorResponse(w, http.StatusNotFound, "The enrollment does not exist.")
		return false
	}
	assigned, err := a.db.IsMentorAssigned(r.Context(), seasonID, actor.UserID, member.UserID)
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return false
	}
	if !assigned {
		writeErrorResponse(w, http.StatusForbidden, "Only the student's current mentor or a season administrator can change their details.")
		return false
	}
	return true
}

func (a *API) updateStudentLevel(w http.ResponseWriter, r *http.Request) {
	actor := actorFrom(r.Context())
	seasonID := r.PathValue("id")
	if !a.authorizeStudentDetails(w, r) {
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
	seasonID := r.PathValue("id")
	if !a.authorizeStudentDetails(w, r) {
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
	if !a.authorizeStudentDetails(w, r) {
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

// updateStudentProfile lets a student's current mentor or a coordinator
// change the student's name and username, which students cannot change
// themselves. Someone who can rename themselves (a mentor elsewhere, say) is
// left to do so.
func (a *API) updateStudentProfile(w http.ResponseWriter, r *http.Request) {
	actor := actorFrom(r.Context())
	seasonID := r.PathValue("id")
	if !a.authorizeStudentDetails(w, r) {
		return
	}
	var request struct {
		Name string `json:"name"`
		Slug string `json:"slug"`
	}
	if err := decodeJSON(r.Body, &request); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	request.Name = strings.TrimSpace(request.Name)
	request.Slug = strings.ToLower(strings.TrimSpace(request.Slug))
	if len(request.Name) < 1 || len(request.Name) > 100 || len(request.Slug) < 2 || len(request.Slug) > 50 || !validSlug(request.Slug) {
		writeErrorResponse(w, http.StatusBadRequest, "Enter a name and a 2–50 character username using lowercase letters, numbers and hyphens.")
		return
	}
	member, err := a.db.GetEnrollment(r.Context(), r.PathValue("memberId"))
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}
	if member.SeasonID != seasonID || member.Role != string(authz.Student) {
		writeErrorResponse(w, http.StatusNotFound, "The student does not exist in this season.")
		return
	}
	student, err := a.db.ActorForUser(r.Context(), member.UserID)
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}
	if student.CanRenameSelf() {
		writeErrorResponse(w, http.StatusForbidden, "They can change their own name and username from their profile.")
		return
	}
	saved, err := a.db.UpdateAdminProfile(r.Context(), accounts.AdminProfile{ID: member.UserID, Name: request.Name, Slug: request.Slug}, actor.UserID, time.Now().UTC())
	if errors.Is(err, dal.ErrDuplicate) {
		writeErrorResponse(w, http.StatusConflict, "That username is taken. Choose another.")
		return
	}
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}
	writeJSONResponse(w, http.StatusOK, map[string]string{"name": saved.Name, "slug": saved.Slug})
}
