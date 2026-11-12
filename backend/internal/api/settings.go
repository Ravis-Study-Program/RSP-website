package api

import (
	"fmt"
	"net/http"
	"time"

	"github.com/Ravis-Study-Program/RSP-website/backend/internal/authz"
	"github.com/Ravis-Study-Program/RSP-website/backend/internal/practice"
	"github.com/Ravis-Study-Program/RSP-website/backend/internal/program"
)

func (a *API) getCurrentUserPracticeSettings(w http.ResponseWriter, r *http.Request) {
	actor := actorFrom(r.Context())
	settings, err := a.db.GetPracticeSettings(r.Context(), actor.UserID)
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}

	writeJSONResponse(w, http.StatusOK, settings)
}

func (a *API) getUserPracticeSettings(w http.ResponseWriter, r *http.Request) {
	actor := actorFrom(r.Context())
	target, err := a.db.GetUser(r.Context(), r.PathValue("id"))
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}

	allowed := actor.CanViewMemberPrivateData(target.ID, authz.MemberRelationship{})
	var targetEnrollments []program.EnrollmentRecord
	targetLoaded := false
	for _, enrollment := range actor.Enrollments {
		if allowed {
			break
		}
		if enrollment.State != authz.Active && enrollment.State != authz.Completed {
			continue
		}
		relationship := authz.MemberRelationship{SeasonID: enrollment.SeasonID}
		if enrollment.Role == authz.Coordinator {
			if !targetLoaded {
				var err error
				targetEnrollments, err = a.db.ListEnrollmentsForUser(r.Context(), target.ID)
				if err != nil {
					a.writeStoreErrorResponse(w, err)
					return
				}

				targetLoaded = true
			}
			for _, target := range targetEnrollments {
				if target.SeasonID == enrollment.SeasonID && (target.State == "active" || target.State == "completed") {
					relationship.TargetEnrolled = true
					break
				}
			}
		}
		if enrollment.Role == authz.Mentor {
			assigned, err := a.db.IsMentorAssigned(r.Context(), enrollment.SeasonID, actor.UserID, target.ID)
			if err != nil {
				a.writeStoreErrorResponse(w, err)
				return
			}
			relationship.AssignedMentor = assigned
		}
		if actor.CanViewMemberPrivateData(target.ID, relationship) {
			allowed = true
			break
		}
	}

	if !allowed {
		writeErrorResponse(w, http.StatusNotFound, "The requested member does not exist.")
		return
	}
	if err := a.auditPrivateDataRead(r.Context(), actor, "practice_settings", target.ID); err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "Private data was not returned because its access could not be audited.")
		return
	}

	settings, err := a.db.GetPracticeSettings(r.Context(), target.ID)
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}

	writeJSONResponse(w, http.StatusOK, settings)
}

// updatePracticeSettings changes the program's practice goals, which apply
// to every member. Coordinators and administrators may set them.
func (a *API) updatePracticeSettings(w http.ResponseWriter, r *http.Request) {
	actor := actorFrom(r.Context())
	if !actor.CanSetPracticeGoals() {
		writeErrorResponse(w, http.StatusForbidden, "Only coordinators and administrators can change practice goals.")
		return
	}
	var request struct {
		EasyMinutes   int `json:"easyMinutes"`
		MediumMinutes int `json:"mediumMinutes"`
		HardMinutes   int `json:"hardMinutes"`
		WeeklyHours   int `json:"weeklyHours"`
	}
	if err := decodeJSON(r.Body, &request); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	settings := practice.PracticeSettings{
		GoalsEnabled:  true,
		EasyMinutes:   request.EasyMinutes,
		MediumMinutes: request.MediumMinutes,
		HardMinutes:   request.HardMinutes,
		WeeklyHours:   request.WeeklyHours,
	}
	if !settings.Valid() {
		writeErrorResponse(w, http.StatusBadRequest, fmt.Sprintf("Each goal must be a whole number of minutes from 1 to %d.", practice.MaxGoalMinutes))
		return
	}
	if !settings.ValidWeeklyHours() {
		writeErrorResponse(w, http.StatusBadRequest, fmt.Sprintf("Weekly hours must be a whole number from 1 to %d.", practice.MaxWeeklyHours))
		return
	}
	saved, err := a.db.SetPracticeSettings(r.Context(), actor.UserID, settings, time.Now().UTC())
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}
	writeJSONResponse(w, http.StatusOK, saved)
}
