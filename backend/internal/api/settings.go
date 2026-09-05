package api

import (
	"net/http"
	"time"

	"github.com/magedmg/RSP-website/backend/internal/authz"
)

func (a *API) practiceSettings(w http.ResponseWriter, r *http.Request) {
	settings, err := a.db.GetPracticeSettings(r.Context(), actorFrom(r.Context()).UserID)
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}

	writeJSONResponse(w, http.StatusOK, settings)
}

func (a *API) userPracticeSettings(w http.ResponseWriter, r *http.Request) {
	target, err := a.db.GetUser(r.Context(), r.PathValue("id"))
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}

	allowed, err := a.canViewMemberPrivate(r, target.ID)
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}

	if !allowed {
		writeErrorResponse(w, http.StatusNotFound, "not_found", "The requested member does not exist.")
		return
	}
	if !a.auditSystemAdminPrivateRead(w, r, "practice_settings", target.ID) {
		return
	}

	settings, err := a.db.GetPracticeSettings(r.Context(), target.ID)
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}

	writeJSONResponse(w, http.StatusOK, settings)
}

func (a *API) updatePracticeSettings(w http.ResponseWriter, r *http.Request) {
	actor := actorFrom(r.Context())
	var in struct {
		EasyMinutes   int `json:"easyMinutes"`
		MediumMinutes int `json:"mediumMinutes"`
		HardMinutes   int `json:"hardMinutes"`
	}
	if err := decodeJSON(w, r, &in); err != nil || !validGoal(in.EasyMinutes) || !validGoal(in.MediumMinutes) || !validGoal(in.HardMinutes) {
		writeErrorResponse(w, http.StatusBadRequest, "validation_failed", "goals between 1 and 180 minutes are required")
		return
	}

	current, err := a.db.GetPracticeSettings(r.Context(), actor.UserID)
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}

	if !current.GoalsEnabled && (in.EasyMinutes != current.EasyMinutes || in.MediumMinutes != current.MediumMinutes || in.HardMinutes != current.HardMinutes) {
		writeErrorResponse(w, http.StatusForbidden, "practice_goals_not_enabled", "An assigned mentor or programme administrator must enable personal goals before they can be changed.")
		return
	}

	updated, err := a.db.UpdatePracticeSettings(r.Context(), actor.UserID, in.EasyMinutes, in.MediumMinutes, in.HardMinutes, actor.UserID, time.Now().UTC())
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}

	writeJSONResponse(w, http.StatusOK, updated)
}

func validGoal(minutes int) bool { return minutes >= 1 && minutes <= 180 }

func (a *API) enablePracticeGoals(w http.ResponseWriter, r *http.Request) {
	actor := actorFrom(r.Context())
	var in struct {
		SeasonID string `json:"seasonId"`
	}
	if err := decodeJSON(w, r, &in); err != nil || in.SeasonID == "" {
		writeErrorResponse(w, http.StatusBadRequest, "validation_failed", "seasonId is required")
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
		if enrollment.SeasonID == in.SeasonID && enrollment.Role == "student" && enrollment.State == "active" {
			targetActiveStudent = true
			break
		}
	}
	if !targetActiveStudent {
		writeErrorResponse(w, http.StatusNotFound, "active_student_not_found", "The target is not an active student in this season.")
		return
	}

	allowed := false
	if actor.IsPrivileged() {
		if !actor.HasRecentMFA(time.Now()) {
			writeErrorResponse(w, http.StatusForbidden, "privileged_mfa_required", "Recent MFA is required.")
			return
		}
		allowed = true
	} else if enrollment, ok := actor.Enrollment(in.SeasonID); ok && enrollment.State == authz.Active {
		switch enrollment.Role {
		case authz.Coordinator:
			if !actor.HasRecentMFA(time.Now()) {
				writeErrorResponse(w, http.StatusForbidden, "privileged_mfa_required", "Recent MFA is required for this administrative action.")
				return
			}
			allowed = true
		case authz.Mentor:
			allowed, err = a.db.IsMentorAssigned(r.Context(), in.SeasonID, actor.UserID, targetID)
			if err != nil {
				a.writeStoreErrorResponse(w, err)
				return
			}
		}
	}
	if !allowed {
		writeErrorResponse(w, http.StatusForbidden, "forbidden", "Only the assigned mentor or an administrator for this season may enable personal goals.")
		return
	}

	settings, err := a.db.EnablePracticeGoals(r.Context(), targetID, actor.UserID, in.SeasonID, time.Now().UTC())
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}

	writeJSONResponse(w, http.StatusOK, settings)
}
