package api

import (
	"net/http"
	"time"

	"github.com/magedmg/RSP-website/backend/internal/authz"
	"github.com/magedmg/RSP-website/backend/internal/dal"
	"github.com/magedmg/RSP-website/backend/internal/programme"
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
	var targetEnrollments []programme.EnrollmentRecord
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

type updateCurrentUserPracticeSettingsRequest struct {
	EasyMinutes   int `json:"easyMinutes"`
	MediumMinutes int `json:"mediumMinutes"`
	HardMinutes   int `json:"hardMinutes"`
}

func (a *API) updateCurrentUserPracticeSettings(w http.ResponseWriter, r *http.Request) {
	actor := actorFrom(r.Context())
	var request updateCurrentUserPracticeSettingsRequest
	if err := decodeJSON(r.Body, &request); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	if !validGoal(request.EasyMinutes) {
		writeErrorResponse(w, http.StatusBadRequest, "easyMinutes must be between 1 and 180")
		return
	}
	if !validGoal(request.MediumMinutes) {
		writeErrorResponse(w, http.StatusBadRequest, "mediumMinutes must be between 1 and 180")
		return
	}
	if !validGoal(request.HardMinutes) {
		writeErrorResponse(w, http.StatusBadRequest, "hardMinutes must be between 1 and 180")
		return
	}

	current, err := a.db.GetPracticeSettings(r.Context(), actor.UserID)
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}

	if !current.GoalsEnabled && (request.EasyMinutes != current.EasyMinutes || request.MediumMinutes != current.MediumMinutes || request.HardMinutes != current.HardMinutes) {
		writeErrorResponse(w, http.StatusForbidden, "An assigned mentor or programme administrator must enable personal goals before they can be changed.")
		return
	}

	updated, err := a.db.UpdatePracticeSettings(r.Context(), dal.UpdatePracticeSettingsInput{
		UserID:        actor.UserID,
		EasyMinutes:   request.EasyMinutes,
		MediumMinutes: request.MediumMinutes,
		HardMinutes:   request.HardMinutes,
		ActorID:       actor.UserID,
		ChangedAt:     time.Now().UTC(),
	})
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}

	writeJSONResponse(w, http.StatusOK, updated)
}

func validGoal(minutes int) bool { return minutes >= 1 && minutes <= 180 }

type enablePracticeGoalsRequest struct {
	SeasonID string `json:"seasonId"`
}

func (a *API) enablePracticeGoals(w http.ResponseWriter, r *http.Request) {
	actor := actorFrom(r.Context())
	var request enablePracticeGoalsRequest
	if err := decodeJSON(r.Body, &request); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	if request.SeasonID == "" {
		writeErrorResponse(w, http.StatusBadRequest, "seasonId is required")
		return
	}

	targetID := r.PathValue("id")
	targetActiveStudent := false
	enrollments, err := a.db.ListEnrollmentsForUser(r.Context(), targetID)
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}

	for _, enrollment := range enrollments {
		if enrollment.SeasonID == request.SeasonID && enrollment.Role == "student" && enrollment.State == "active" {
			targetActiveStudent = true
			break
		}
	}
	if !targetActiveStudent {
		writeErrorResponse(w, http.StatusNotFound, "The target is not an active student in this season.")
		return
	}

	allowed := false
	if actor.IsDirectorOrSystemAdmin() {
		if !actor.HasRecentMFA(time.Now()) {
			writeErrorResponse(w, http.StatusForbidden, "Recent MFA is required.")
			return
		}
		allowed = true
	} else if enrollment, ok := actor.Enrollment(request.SeasonID); ok && enrollment.State == authz.Active {
		switch enrollment.Role {
		case authz.Coordinator:
			if !actor.HasRecentMFA(time.Now()) {
				writeErrorResponse(w, http.StatusForbidden, "Recent MFA is required for this administrative action.")
				return
			}
			allowed = true
		case authz.Mentor:
			allowed, err = a.db.IsMentorAssigned(r.Context(), request.SeasonID, actor.UserID, targetID)
			if err != nil {
				a.writeStoreErrorResponse(w, err)
				return
			}
		}
	}
	if !allowed {
		writeErrorResponse(w, http.StatusForbidden, "Only the assigned mentor or an administrator for this season may enable personal goals.")
		return
	}

	settings, err := a.db.EnablePracticeGoals(r.Context(), dal.EnablePracticeGoalsInput{
		UserID:    targetID,
		ActorID:   actor.UserID,
		SeasonID:  request.SeasonID,
		ChangedAt: time.Now().UTC(),
	})
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}

	writeJSONResponse(w, http.StatusOK, settings)
}
