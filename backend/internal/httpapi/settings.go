package httpapi

import (
	"net/http"
	"time"

	"github.com/magedmg/RSP-website/backend/internal/authz"
	"github.com/magedmg/RSP-website/backend/internal/store"
)

func (a *API) practiceSettings(w http.ResponseWriter, r *http.Request) {
	settings, err := a.store.GetPracticeSettings(r.Context(), actorFrom(r.Context()).UserID)
	if err != nil {
		storeFailure(a, w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, settings)
}

func (a *API) userPracticeSettings(w http.ResponseWriter, r *http.Request) {
	target, err := a.store.GetUser(r.Context(), r.PathValue("id"))
	if err != nil {
		storeFailure(a, w, r, err)
		return
	}
	allowed, err := a.canViewMemberPrivate(r, target.ID)
	if err != nil {
		storeFailure(a, w, r, err)
		return
	}
	if !allowed {
		a.fail(w, r, http.StatusNotFound, "not_found", "Not found", "The requested member does not exist.", nil)
		return
	}
	if !a.auditSystemAdminPrivateRead(w, r, "practice_settings", target.ID) {
		return
	}
	settings, err := a.store.GetPracticeSettings(r.Context(), target.ID)
	if err != nil {
		storeFailure(a, w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, settings)
}

func (a *API) updatePracticeSettings(w http.ResponseWriter, r *http.Request) {
	actor := actorFrom(r.Context())
	var in struct {
		PremiumOptIn  bool  `json:"premiumOptIn"`
		EasyMinutes   int   `json:"easyMinutes"`
		MediumMinutes int   `json:"mediumMinutes"`
		HardMinutes   int   `json:"hardMinutes"`
		Revision      int64 `json:"revision"`
	}
	if err := decodeJSON(w, r, &in); err != nil || in.Revision < 1 || !validGoal(in.EasyMinutes) || !validGoal(in.MediumMinutes) || !validGoal(in.HardMinutes) {
		validation(a, w, r, "premium preference, goals between 5 and 180 minutes, and revision are required")
		return
	}
	current, err := a.store.GetPracticeSettings(r.Context(), actor.UserID)
	if err != nil {
		storeFailure(a, w, r, err)
		return
	}
	if !current.GoalsEnabled && (in.EasyMinutes != current.EasyMinutes || in.MediumMinutes != current.MediumMinutes || in.HardMinutes != current.HardMinutes) {
		a.fail(w, r, http.StatusForbidden, "practice_goals_not_enabled", "Practice goals not enabled", "An assigned mentor or programme administrator must enable personal goals before they can be changed.", nil)
		return
	}
	updated, err := a.store.UpdatePracticeSettings(r.Context(), actor.UserID, in.Revision, in.PremiumOptIn, in.EasyMinutes, in.MediumMinutes, in.HardMinutes, actor.UserID, time.Now().UTC())
	if err != nil {
		storeFailure(a, w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

func validGoal(minutes int) bool { return minutes >= 5 && minutes <= 180 }

func (a *API) enablePracticeGoals(w http.ResponseWriter, r *http.Request) {
	actor := actorFrom(r.Context())
	var in struct {
		SeasonID string `json:"seasonId"`
		Revision int64  `json:"revision"`
	}
	if err := decodeJSON(w, r, &in); err != nil || in.SeasonID == "" || in.Revision < 1 {
		validation(a, w, r, "seasonId and revision are required")
		return
	}
	targetID := r.PathValue("id")
	targetActiveStudent := false
	enrollments, err := a.store.ListEnrollmentsForUser(r.Context(), targetID)
	if err != nil {
		storeFailure(a, w, r, err)
		return
	}
	for _, enrollment := range enrollments {
		if enrollment.SeasonID == in.SeasonID && enrollment.Role == "student" && enrollment.State == "active" {
			targetActiveStudent = true
			break
		}
	}
	if !targetActiveStudent {
		a.fail(w, r, http.StatusNotFound, "active_student_not_found", "Active student not found", "The target is not an active student in this season.", nil)
		return
	}
	allowed := false
	if actor.IsPrivileged() {
		if !actor.HasRecentMFA(time.Now()) {
			a.fail(w, r, http.StatusForbidden, "privileged_mfa_required", "Privileged access required", "Recent MFA is required.", nil)
			return
		}
		allowed = true
	} else if enrollment, ok := actor.Enrollment(in.SeasonID); ok && enrollment.State == authz.Active {
		switch enrollment.Role {
		case authz.Coordinator:
			if !actor.HasRecentMFA(time.Now()) {
				a.fail(w, r, http.StatusForbidden, "privileged_mfa_required", "Recent MFA required", "Recent MFA is required for this administrative action.", nil)
				return
			}
			allowed = true
		case authz.Mentor:
			allowed, err = a.store.IsMentorAssigned(r.Context(), in.SeasonID, actor.UserID, targetID)
			if err != nil {
				storeFailure(a, w, r, err)
				return
			}
		}
	}
	if !allowed {
		a.fail(w, r, http.StatusForbidden, "forbidden", "Forbidden", "Only the assigned mentor or an administrator for this season may enable personal goals.", nil)
		return
	}
	settings, err := a.store.EnablePracticeGoals(r.Context(), targetID, in.Revision, actor.UserID, in.SeasonID, time.Now().UTC())
	if err != nil {
		if err == store.ErrNotFound {
			a.fail(w, r, http.StatusNotFound, "not_found", "Not found", "The requested member does not exist.", nil)
		} else {
			storeFailure(a, w, r, err)
		}
		return
	}
	writeJSON(w, http.StatusOK, settings)
}
