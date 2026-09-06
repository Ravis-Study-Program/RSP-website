package api

import (
	"net/http"

	"github.com/magedmg/RSP-website/backend/internal/authz"
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
