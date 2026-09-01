package httpapi

import (
	"errors"
	"net/http"
	"time"

	"github.com/magedmg/RSP-website/backend/internal/authz"
	"github.com/magedmg/RSP-website/backend/internal/store"
)

func (a *API) practiceSettings(w http.ResponseWriter, r *http.Request) {
	settings, err := a.store.GetPracticeSettings(r.Context(), actorFrom(r.Context()).UserID)
	if err != nil {
		switch {
		case errors.Is(err, store.ErrNotFound):
			writeErrorResponse(w, http.StatusNotFound, "not_found", "The requested resource does not exist.")
		case errors.Is(err, store.ErrConflict):
			writeErrorResponse(w, http.StatusConflict, "stale_revision", "The resource changed since it was loaded.")
		case errors.Is(err, store.ErrDuplicate):
			writeErrorResponse(w, http.StatusConflict, "duplicate", "A resource with that unique value already exists.")
		default:
			writeErrorResponse(w, http.StatusInternalServerError, "internal_error", "The request could not be completed.")
		}
		return
	}

	writeJSONResponse(w, http.StatusOK, settings)
}

func (a *API) userPracticeSettings(w http.ResponseWriter, r *http.Request) {
	target, err := a.store.GetUser(r.Context(), r.PathValue("id"))
	if err != nil {
		switch {
		case errors.Is(err, store.ErrNotFound):
			writeErrorResponse(w, http.StatusNotFound, "not_found", "The requested resource does not exist.")
		case errors.Is(err, store.ErrConflict):
			writeErrorResponse(w, http.StatusConflict, "stale_revision", "The resource changed since it was loaded.")
		case errors.Is(err, store.ErrDuplicate):
			writeErrorResponse(w, http.StatusConflict, "duplicate", "A resource with that unique value already exists.")
		default:
			writeErrorResponse(w, http.StatusInternalServerError, "internal_error", "The request could not be completed.")
		}
		return
	}

	allowed, err := a.canViewMemberPrivate(r, target.ID)
	if err != nil {
		switch {
		case errors.Is(err, store.ErrNotFound):
			writeErrorResponse(w, http.StatusNotFound, "not_found", "The requested resource does not exist.")
		case errors.Is(err, store.ErrConflict):
			writeErrorResponse(w, http.StatusConflict, "stale_revision", "The resource changed since it was loaded.")
		case errors.Is(err, store.ErrDuplicate):
			writeErrorResponse(w, http.StatusConflict, "duplicate", "A resource with that unique value already exists.")
		default:
			writeErrorResponse(w, http.StatusInternalServerError, "internal_error", "The request could not be completed.")
		}
		return
	}

	if !allowed {
		writeErrorResponse(w, http.StatusNotFound, "not_found", "The requested member does not exist.")
		return
	}
	if !a.auditSystemAdminPrivateRead(w, r, "practice_settings", target.ID) {
		return
	}

	settings, err := a.store.GetPracticeSettings(r.Context(), target.ID)
	if err != nil {
		switch {
		case errors.Is(err, store.ErrNotFound):
			writeErrorResponse(w, http.StatusNotFound, "not_found", "The requested resource does not exist.")
		case errors.Is(err, store.ErrConflict):
			writeErrorResponse(w, http.StatusConflict, "stale_revision", "The resource changed since it was loaded.")
		case errors.Is(err, store.ErrDuplicate):
			writeErrorResponse(w, http.StatusConflict, "duplicate", "A resource with that unique value already exists.")
		default:
			writeErrorResponse(w, http.StatusInternalServerError, "internal_error", "The request could not be completed.")
		}
		return
	}

	writeJSONResponse(w, http.StatusOK, settings)
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
		writeErrorResponse(w, http.StatusBadRequest, "validation_failed", "premium preference, goals between 5 and 180 minutes, and revision are required")
		return
	}

	current, err := a.store.GetPracticeSettings(r.Context(), actor.UserID)
	if err != nil {
		switch {
		case errors.Is(err, store.ErrNotFound):
			writeErrorResponse(w, http.StatusNotFound, "not_found", "The requested resource does not exist.")
		case errors.Is(err, store.ErrConflict):
			writeErrorResponse(w, http.StatusConflict, "stale_revision", "The resource changed since it was loaded.")
		case errors.Is(err, store.ErrDuplicate):
			writeErrorResponse(w, http.StatusConflict, "duplicate", "A resource with that unique value already exists.")
		default:
			writeErrorResponse(w, http.StatusInternalServerError, "internal_error", "The request could not be completed.")
		}
		return
	}

	if !current.GoalsEnabled && (in.EasyMinutes != current.EasyMinutes || in.MediumMinutes != current.MediumMinutes || in.HardMinutes != current.HardMinutes) {
		writeErrorResponse(w, http.StatusForbidden, "practice_goals_not_enabled", "An assigned mentor or programme administrator must enable personal goals before they can be changed.")
		return
	}

	updated, err := a.store.UpdatePracticeSettings(r.Context(), actor.UserID, in.Revision, in.PremiumOptIn, in.EasyMinutes, in.MediumMinutes, in.HardMinutes, actor.UserID, time.Now().UTC())
	if err != nil {
		switch {
		case errors.Is(err, store.ErrNotFound):
			writeErrorResponse(w, http.StatusNotFound, "not_found", "The requested resource does not exist.")
		case errors.Is(err, store.ErrConflict):
			writeErrorResponse(w, http.StatusConflict, "stale_revision", "The resource changed since it was loaded.")
		case errors.Is(err, store.ErrDuplicate):
			writeErrorResponse(w, http.StatusConflict, "duplicate", "A resource with that unique value already exists.")
		default:
			writeErrorResponse(w, http.StatusInternalServerError, "internal_error", "The request could not be completed.")
		}
		return
	}

	writeJSONResponse(w, http.StatusOK, updated)
}

func validGoal(minutes int) bool { return minutes >= 5 && minutes <= 180 }

func (a *API) enablePracticeGoals(w http.ResponseWriter, r *http.Request) {
	actor := actorFrom(r.Context())
	var in struct {
		SeasonID string `json:"seasonId"`
		Revision int64  `json:"revision"`
	}
	if err := decodeJSON(w, r, &in); err != nil || in.SeasonID == "" || in.Revision < 1 {
		writeErrorResponse(w, http.StatusBadRequest, "validation_failed", "seasonId and revision are required")
		return
	}

	targetID := r.PathValue("id")
	targetActiveStudent := false
	enrollments, err := a.store.ListEnrollmentsForUser(r.Context(), targetID)
	if err != nil {
		switch {
		case errors.Is(err, store.ErrNotFound):
			writeErrorResponse(w, http.StatusNotFound, "not_found", "The requested resource does not exist.")
		case errors.Is(err, store.ErrConflict):
			writeErrorResponse(w, http.StatusConflict, "stale_revision", "The resource changed since it was loaded.")
		case errors.Is(err, store.ErrDuplicate):
			writeErrorResponse(w, http.StatusConflict, "duplicate", "A resource with that unique value already exists.")
		default:
			writeErrorResponse(w, http.StatusInternalServerError, "internal_error", "The request could not be completed.")
		}
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
			allowed, err = a.store.IsMentorAssigned(r.Context(), in.SeasonID, actor.UserID, targetID)
			if err != nil {
				switch {
				case errors.Is(err, store.ErrNotFound):
					writeErrorResponse(w, http.StatusNotFound, "not_found", "The requested resource does not exist.")
				case errors.Is(err, store.ErrConflict):
					writeErrorResponse(w, http.StatusConflict, "stale_revision", "The resource changed since it was loaded.")
				case errors.Is(err, store.ErrDuplicate):
					writeErrorResponse(w, http.StatusConflict, "duplicate", "A resource with that unique value already exists.")
				default:
					writeErrorResponse(w, http.StatusInternalServerError, "internal_error", "The request could not be completed.")
				}
				return
			}
		}
	}
	if !allowed {
		writeErrorResponse(w, http.StatusForbidden, "forbidden", "Only the assigned mentor or an administrator for this season may enable personal goals.")
		return
	}

	settings, err := a.store.EnablePracticeGoals(r.Context(), targetID, in.Revision, actor.UserID, in.SeasonID, time.Now().UTC())
	if err != nil {
		if err == store.ErrNotFound {
			writeErrorResponse(w, http.StatusNotFound, "not_found", "The requested member does not exist.")
		} else {
			switch {
			case errors.Is(err, store.ErrNotFound):
				writeErrorResponse(w, http.StatusNotFound, "not_found", "The requested resource does not exist.")
			case errors.Is(err, store.ErrConflict):
				writeErrorResponse(w, http.StatusConflict, "stale_revision", "The resource changed since it was loaded.")
			case errors.Is(err, store.ErrDuplicate):
				writeErrorResponse(w, http.StatusConflict, "duplicate", "A resource with that unique value already exists.")
			default:
				writeErrorResponse(w, http.StatusInternalServerError, "internal_error", "The request could not be completed.")
			}
		}
		return
	}

	writeJSONResponse(w, http.StatusOK, settings)
}
