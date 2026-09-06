package api

import (
	"net/http"
	"strings"

	"github.com/magedmg/RSP-website/backend/internal/accounts"
	"github.com/magedmg/RSP-website/backend/internal/authz"
	"github.com/magedmg/RSP-website/backend/internal/dal"
	"github.com/magedmg/RSP-website/backend/internal/platform/cursor"
	"github.com/magedmg/RSP-website/backend/internal/programme"
)

func (a *API) listUsers(w http.ResponseWriter, r *http.Request) {
	actor := actorFrom(r.Context())
	if !actor.CanAccessMemberDirectory() && !actor.IsDirectorOrSystemAdmin() {
		writeErrorResponse(w, http.StatusForbidden, "No season access yet.")
		return
	}

	_, err := parseSort(r.URL.Query().Get("sort"), "id:asc", "id:asc")
	if err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "sort must be id:asc")
		return
	}

	direction := r.URL.Query().Get("direction")
	if direction == "" {
		direction = "forward"
	}

	if direction != "forward" && direction != "backward" {
		writeErrorResponse(w, http.StatusBadRequest, "direction must be forward or backward")
		return
	}

	query := strings.TrimSpace(r.URL.Query().Get("query"))
	seasonRole, globalRole := r.URL.Query().Get("seasonRole"), r.URL.Query().Get("globalRole")
	if len(query) > 100 || seasonRole != "" && seasonRole != "student" && seasonRole != "mentor" && seasonRole != "coordinator" || globalRole != "" && globalRole != "director" && globalRole != "system_admin" {
		writeErrorResponse(w, http.StatusBadRequest, "query and valid seasonRole/globalRole filters are required")
		return
	}

	binding := "users|sort=id:asc|query=" + query + "|seasonRole=" + seasonRole + "|globalRole=" + globalRole
	limit, boundary, err := parsePagination(r.URL.Query().Get("limit"), r.URL.Query().Get("cursor"), binding, a.cursorSecret)
	if err != nil {
		writeErrorResponse(w, http.StatusBadRequest, cursor.ErrInvalid.Error())
		return
	}

	items, more, total, err := a.db.ListUsers(r.Context(), dal.UserQuery{
		Boundary:   boundary,
		Limit:      limit,
		Direction:  direction,
		Search:     query,
		SeasonRole: seasonRole,
		GlobalRole: globalRole,
	})
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}

	for i := range items {
		items[i].Email = ""
		items[i].AccountState = ""
	}
	pageInfo, err := pageInfoForKeyset(
		a.cursorSecret, binding, direction, boundary, items, more,
		func(record accounts.User) string { return record.ID },
	)
	if err != nil {
		a.logger.Error("cursor encoding failed", "error", err)
		writeErrorResponse(w, http.StatusInternalServerError, "The request could not be completed.")
		return
	}
	response := Page[accounts.User]{
		Items:      items,
		PageInfo:   pageInfo,
		TotalCount: total,
	}
	writeJSONResponse(w, http.StatusOK, response)
}

func (a *API) getUser(w http.ResponseWriter, r *http.Request) {
	actor := actorFrom(r.Context())
	if !actor.CanAccessMemberDirectory() && !actor.IsDirectorOrSystemAdmin() {
		writeErrorResponse(w, http.StatusForbidden, "No season access yet.")
		return
	}

	user, err := a.db.GetUser(r.Context(), r.PathValue("id"))
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}

	if actor.UserID != user.ID && !actor.IsDirectorOrSystemAdmin() {
		participant, eligibilityErr := a.db.GetMemberStatus(r.Context(), user.ID)
		if eligibilityErr != nil {
			a.writeStoreErrorResponse(w, eligibilityErr)
			return
		}
		if !participant.IsVisibleInDirectory() {
			writeErrorResponse(w, http.StatusNotFound, "The requested resource does not exist.")
			return
		}
	}
	if actor.UserID != user.ID && !actor.IsDirectorOrSystemAdmin() {
		canPrivate := actor.CanViewMemberPrivateData(user.ID, authz.MemberRelationship{})
		var targetEnrollments []programme.EnrollmentRecord
		targetLoaded := false
		for _, enrollment := range actor.Enrollments {
			if canPrivate {
				break
			}
			if enrollment.State != authz.Active && enrollment.State != authz.Completed {
				continue
			}
			relationship := authz.MemberRelationship{SeasonID: enrollment.SeasonID}
			if enrollment.Role == authz.Coordinator {
				if !targetLoaded {
					var err error
					targetEnrollments, err = a.db.ListEnrollmentsForUser(r.Context(), user.ID)
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
				assigned, err := a.db.IsMentorAssigned(r.Context(), enrollment.SeasonID, actor.UserID, user.ID)
				if err != nil {
					a.writeStoreErrorResponse(w, err)
					return
				}
				relationship.AssignedMentor = assigned
			}
			if actor.CanViewMemberPrivateData(user.ID, relationship) {
				canPrivate = true
				break
			}
		}

		if !canPrivate {
			user.Email = ""
			user.AccountState = ""
		}
	}
	if err := a.auditPrivateDataRead(r.Context(), actor, "user", user.ID); err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "Private data was not returned because its access could not be audited.")
		return
	}
	writeJSONResponse(w, http.StatusOK, user)
}
