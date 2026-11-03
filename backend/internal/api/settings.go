package api

import (
	"net/http"
	"time"

	"github.com/magedmg/RSP-website/backend/internal/authz"
	"github.com/magedmg/RSP-website/backend/internal/dal"
	"github.com/magedmg/RSP-website/backend/internal/practice"
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

	// Goals judge attempts that programme members already share, so anyone who
	// can read shared activity may see them. Former staff without that access
	// still reach them through the audited private-data relationship policy.
	if actor.CanReadSharedActivity() {
		settings, err := a.db.GetPracticeSettings(r.Context(), target.ID)
		if err != nil {
			a.writeStoreErrorResponse(w, err)
			return
		}
		writeJSONResponse(w, http.StatusOK, settings)
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

func (a *API) getProgrammePracticeGoals(w http.ResponseWriter, r *http.Request) {
	goals, err := a.db.GetProgrammePracticeGoals(r.Context())
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}
	writeJSONResponse(w, http.StatusOK, goals)
}

func (a *API) updateProgrammePracticeGoals(w http.ResponseWriter, r *http.Request) {
	actor := actorFrom(r.Context())
	if !actor.CanSetProgrammePracticeGoals() {
		writeErrorResponse(w, http.StatusForbidden, "A Director or System Admin is required to change the programme time goals.")
		return
	}
	goals, ok := decodePracticeGoals(w, r)
	if !ok {
		return
	}
	updated, err := a.db.UpdateProgrammePracticeGoals(r.Context(), dal.UpdateProgrammePracticeGoalsInput{
		Goals: goals, ActorID: actor.UserID, ChangedAt: time.Now().UTC(),
	})
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}
	writeJSONResponse(w, http.StatusOK, updated)
}

func (a *API) putStudentPracticeGoals(w http.ResponseWriter, r *http.Request) {
	goals, ok := decodePracticeGoals(w, r)
	if !ok {
		return
	}
	a.setStudentPracticeGoals(w, r, &goals)
}

func (a *API) deleteStudentPracticeGoals(w http.ResponseWriter, r *http.Request) {
	a.setStudentPracticeGoals(w, r, nil)
}

func (a *API) setStudentPracticeGoals(w http.ResponseWriter, r *http.Request, goals *practice.Goals) {
	actor := actorFrom(r.Context())
	seasonID, memberID := r.PathValue("id"), r.PathValue("memberId")
	member, err := a.db.GetEnrollment(r.Context(), memberID)
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}
	if member.SeasonID != seasonID || member.Role != "student" {
		writeErrorResponse(w, http.StatusNotFound, "The student enrollment does not exist.")
		return
	}
	assigned, err := a.db.IsMentorAssigned(r.Context(), seasonID, actor.UserID, member.UserID)
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}
	if !actor.CanSetStudentPracticeGoals(seasonID, assigned, member.State == "active") {
		writeErrorResponse(w, http.StatusForbidden, "Only this student's mentor or a season administrator can change their time goals.")
		return
	}
	settings, err := a.db.SetStudentPracticeGoals(r.Context(), dal.SetStudentPracticeGoalsInput{
		SeasonID: seasonID, EnrollmentID: memberID, ActorID: actor.UserID, Goals: goals, ChangedAt: time.Now().UTC(),
	})
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}
	writeJSONResponse(w, http.StatusOK, settings)
}

func decodePracticeGoals(w http.ResponseWriter, r *http.Request) (practice.Goals, bool) {
	var goals practice.Goals
	if err := decodeJSON(r.Body, &goals); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, err.Error())
		return goals, false
	}
	if !goals.Valid() {
		writeErrorResponse(w, http.StatusBadRequest, "easyMinutes, mediumMinutes and hardMinutes must each be from 1 to 180")
		return goals, false
	}
	return goals, true
}
