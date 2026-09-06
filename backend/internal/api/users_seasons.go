package api

import (
	"context"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/magedmg/RSP-website/backend/internal/accounts"
	"github.com/magedmg/RSP-website/backend/internal/authz"
	"github.com/magedmg/RSP-website/backend/internal/dal"
	"github.com/magedmg/RSP-website/backend/internal/platform/cursor"
	"github.com/magedmg/RSP-website/backend/internal/platform/id"
	"github.com/magedmg/RSP-website/backend/internal/programme"
)

type currentUserSeasonRole struct {
	SeasonID   string `json:"seasonId"`
	SeasonSlug string `json:"seasonSlug"`
	Role       string `json:"role"`
	State      string `json:"state"`
}

type currentUserResponse struct {
	accounts.User
	EmailVerified bool                    `json:"emailVerified"`
	MFAVerified   bool                    `json:"mfaVerified"`
	SeasonRoles   []currentUserSeasonRole `json:"seasonRoles"`
	Alumni        bool                    `json:"alumni"`
}

func (a *API) loadCurrentUserSeasonRoles(ctx context.Context, enrollments []authz.Enrollment) ([]currentUserSeasonRole, error) {
	roles := make([]currentUserSeasonRole, 0, len(enrollments))
	for _, enrollment := range enrollments {
		if enrollment.State != authz.Active && enrollment.State != authz.Completed {
			continue
		}

		season, err := a.db.GetSeason(ctx, enrollment.SeasonID)
		if err != nil {
			return nil, err
		}

		roles = append(roles, currentUserSeasonRole{
			SeasonID:   enrollment.SeasonID,
			SeasonSlug: season.Slug,
			Role:       string(enrollment.Role),
			State:      string(enrollment.State),
		})
	}
	sort.Slice(roles, func(i, j int) bool { return roles[i].SeasonSlug < roles[j].SeasonSlug })
	return roles, nil
}

func buildCurrentUserResponse(actor authz.Actor, user accounts.User, roles []currentUserSeasonRole, now time.Time) currentUserResponse {
	return currentUserResponse{
		User:          user,
		EmailVerified: actor.EmailVerified,
		MFAVerified:   actor.HasRecentMFA(now),
		SeasonRoles:   roles,
		Alumni:        actor.IsStudentAlumnus(),
	}
}

func (a *API) getCurrentUser(w http.ResponseWriter, r *http.Request) {
	actor := actorFrom(r.Context())
	user, err := a.db.GetUser(r.Context(), actor.UserID)
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}

	roles, err := a.loadCurrentUserSeasonRoles(r.Context(), actor.Enrollments)
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}

	response := buildCurrentUserResponse(actor, user, roles, time.Now())
	writeJSONResponse(w, http.StatusOK, response)
}

func (a *API) suggestCurrentUserSlug(w http.ResponseWriter, r *http.Request) {
	slug, err := a.db.SuggestUserSlug(r.Context())
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}

	writeJSONResponse(w, http.StatusOK, map[string]string{"slug": slug})
}

type updateCurrentUserRequest struct {
	Name      string  `json:"name"`
	Slug      *string `json:"slug"`
	AvatarURL *string `json:"avatarUrl"`
	Timezone  string  `json:"timezone"`
}

func (a *API) updateCurrentUser(w http.ResponseWriter, r *http.Request) {
	actor := actorFrom(r.Context())
	var request updateCurrentUserRequest
	if err := decodeJSON(r.Body, &request); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "validation_failed", err.Error())
		return
	}
	if strings.TrimSpace(request.Name) == "" {
		writeErrorResponse(w, http.StatusBadRequest, "validation_failed", "name is required")
		return
	}
	if _, err := time.LoadLocation(request.Timezone); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "validation_failed", "timezone must be an IANA timezone")
		return
	}
	if request.AvatarURL != nil && *request.AvatarURL != "" && !validWebURL(*request.AvatarURL, true) {
		writeErrorResponse(w, http.StatusBadRequest, "validation_failed", "avatarUrl must be an HTTPS URL")
		return
	}

	if request.Slug != nil {
		slug := strings.ToLower(strings.TrimSpace(*request.Slug))
		if len(slug) < 3 || len(slug) > 50 || !validSlug(slug) {
			writeErrorResponse(w, http.StatusBadRequest, "validation_failed", "slug must be 3-50 lowercase letters, numbers, or hyphens")
			return
		}
		request.Slug = &slug
	}

	updated, err := a.db.UpdateUserProfile(r.Context(), dal.UpdateUserProfileInput{
		UserID:    actor.UserID,
		Name:      strings.TrimSpace(request.Name),
		Slug:      request.Slug,
		AvatarURL: request.AvatarURL,
		Timezone:  request.Timezone,
		ActorID:   actor.UserID,
		ChangedAt: time.Now().UTC(),
	})
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}

	roles, err := a.loadCurrentUserSeasonRoles(r.Context(), actor.Enrollments)
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}

	response := buildCurrentUserResponse(actor, updated, roles, time.Now())
	writeJSONResponse(w, http.StatusOK, response)
}

func validSlug(slug string) bool {
	if slug[0] == '-' || slug[len(slug)-1] == '-' {
		return false
	}
	for _, ch := range slug {
		if (ch < 'a' || ch > 'z') && (ch < '0' || ch > '9') && ch != '-' {
			return false
		}
	}
	return true
}

func (a *API) listUsers(w http.ResponseWriter, r *http.Request) {
	actor := actorFrom(r.Context())
	if !actor.CanAccessMemberDirectory() && !actor.IsDirectorOrSystemAdmin() {
		writeErrorResponse(w, http.StatusForbidden, "season_access_required", "No season access yet.")
		return
	}

	_, err := parseSort(r.URL.Query().Get("sort"), "id:asc", "id:asc")
	if err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "validation_failed", "sort must be id:asc")
		return
	}

	direction := r.URL.Query().Get("direction")
	if direction == "" {
		direction = "forward"
	}

	if direction != "forward" && direction != "backward" {
		writeErrorResponse(w, http.StatusBadRequest, "validation_failed", "direction must be forward or backward")
		return
	}

	query := strings.TrimSpace(r.URL.Query().Get("query"))
	seasonRole, globalRole := r.URL.Query().Get("seasonRole"), r.URL.Query().Get("globalRole")
	if len(query) > 100 || seasonRole != "" && seasonRole != "student" && seasonRole != "mentor" && seasonRole != "coordinator" || globalRole != "" && globalRole != "director" && globalRole != "system_admin" {
		writeErrorResponse(w, http.StatusBadRequest, "validation_failed", "query and valid seasonRole/globalRole filters are required")
		return
	}

	binding := "users|sort=id:asc|query=" + query + "|seasonRole=" + seasonRole + "|globalRole=" + globalRole
	limit, boundary, err := parsePagination(r.URL.Query().Get("limit"), r.URL.Query().Get("cursor"), binding, a.cursorSecret)
	if err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "invalid_cursor", cursor.ErrInvalid.Error())
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
	pageInfo := pageInfoForKeyset(
		a.cursorSecret, binding, direction, boundary, items, more,
		func(record accounts.User) string { return record.ID },
	)
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
		writeErrorResponse(w, http.StatusForbidden, "season_access_required", "No season access yet.")
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
			writeErrorResponse(w, http.StatusNotFound, "not_found", "The requested resource does not exist.")
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
		writeErrorResponse(w, http.StatusInternalServerError, "audit_failed", "Private data was not returned because its access could not be audited.")
		return
	}
	writeJSONResponse(w, http.StatusOK, user)
}

func (a *API) listSeasons(w http.ResponseWriter, r *http.Request) {
	actor := actorFrom(r.Context())
	if !actor.CanAccessProgramme() && !actor.IsDirectorOrSystemAdmin() {
		writeErrorResponse(w, http.StatusForbidden, "season_access_required", "No season access yet.")
		return
	}

	_, err := parseSort(r.URL.Query().Get("sort"), "id:asc", "id:asc")
	if err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "validation_failed", "sort must be id:asc")
		return
	}

	direction := r.URL.Query().Get("direction")
	if direction == "" {
		direction = "forward"
	}
	if direction != "forward" && direction != "backward" {
		writeErrorResponse(w, http.StatusBadRequest, "validation_failed", "direction must be forward or backward")
		return
	}

	status := r.URL.Query().Get("status")
	if status != "" && status != "open" && status != "closed" {
		writeErrorResponse(w, http.StatusBadRequest, "validation_failed", "status must be open or closed")
		return
	}

	binding := "seasons|sort=id:asc|status=" + status
	if actor.IsDirectorOrSystemAdmin() {
		limit, boundary, pageErr := parsePagination(r.URL.Query().Get("limit"), r.URL.Query().Get("cursor"), binding, a.cursorSecret)
		if pageErr != nil {
			writeErrorResponse(w, http.StatusBadRequest, "invalid_cursor", cursor.ErrInvalid.Error())
			return
		}
		items, more, total, listErr := a.db.ListSeasons(r.Context(), dal.SeasonQuery{
			Boundary:  boundary,
			Limit:     limit,
			Direction: direction,
			Status:    status,
		})
		if listErr != nil {
			a.writeStoreErrorResponse(w, listErr)
			return
		}
		pageInfo := pageInfoForKeyset(
			a.cursorSecret, binding, direction, boundary, items, more,
			func(seasonRecord programme.SeasonRecord) string { return seasonRecord.ID },
		)
		response := Page[programme.SeasonRecord]{
			Items:      items,
			PageInfo:   pageInfo,
			TotalCount: total,
		}
		writeJSONResponse(w, http.StatusOK, response)
		return
	}

	items := []programme.SeasonRecord{}
	seen := map[string]bool{}
	for _, enrollment := range actor.Enrollments {
		if seen[enrollment.SeasonID] || !actor.CanViewSeason(enrollment.SeasonID) {
			continue
		}
		season, seasonErr := a.db.GetSeason(r.Context(), enrollment.SeasonID)
		if seasonErr != nil {
			a.writeStoreErrorResponse(w, seasonErr)
			return
		}
		if status != "" && season.Status != status {
			continue
		}
		seen[season.ID] = true
		items = append(items, season)
	}

	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	limit, boundary, err := parsePagination(r.URL.Query().Get("limit"), r.URL.Query().Get("cursor"), binding, a.cursorSecret)
	if err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "invalid_cursor", cursor.ErrInvalid.Error())
		return
	}
	page, pageInfo, err := paginateOrdered(a.cursorSecret, binding, direction, boundary, limit, items, func(seasonRecord programme.SeasonRecord) string { return seasonRecord.ID })
	if err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "invalid_cursor", "The cursor does not match the selected season sort.")
		return
	}

	response := Page[programme.SeasonRecord]{
		Items:      page,
		PageInfo:   pageInfo,
		TotalCount: int64(len(items)),
	}
	writeJSONResponse(w, http.StatusOK, response)
}

func (a *API) getSeason(w http.ResponseWriter, r *http.Request) {
	actor := actorFrom(r.Context())
	if !actor.CanViewSeason(r.PathValue("id")) {
		writeErrorResponse(w, http.StatusNotFound, "not_found", "The requested season does not exist.")
		return
	}

	seasonRecord, err := a.db.GetSeason(r.Context(), r.PathValue("id"))
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}

	writeJSONResponse(w, http.StatusOK, seasonRecord)
}

type seasonInput struct {
	Name         string    `json:"name"`
	Slug         string    `json:"slug"`
	Location     string    `json:"location"`
	ImageURL     string    `json:"imageUrl"`
	ResourcesURL string    `json:"resourcesUrl"`
	StartAt      time.Time `json:"startAt"`
	EndAt        time.Time `json:"endAt"`
}

func validSeason(request seasonInput) bool {
	if strings.TrimSpace(request.Name) == "" || !validSeasonSlug(request.Slug) || !request.EndAt.After(request.StartAt) {
		return false
	}
	for _, raw := range []string{request.ImageURL, request.ResourcesURL} {
		if raw != "" {
			if !validWebURL(raw, true) {
				return false
			}
		}
	}
	return true
}

func validSeasonSlug(slug string) bool {
	if len(slug) < 1 || len(slug) > 50 || slug[0] == '-' || slug[len(slug)-1] == '-' {
		return false
	}
	previousHyphen := false
	for _, ch := range slug {
		if ch == '-' {
			if previousHyphen {
				return false
			}
			previousHyphen = true
			continue
		}
		if !((ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9')) {
			return false
		}
		previousHyphen = false
	}
	return true
}

func validWebURL(raw string, httpsOnly bool) bool {
	u, err := url.ParseRequestURI(strings.TrimSpace(raw))
	if err != nil || u.Host == "" || u.User != nil || u.Fragment != "" {
		return false
	}
	if httpsOnly {
		return u.Scheme == "https"
	}
	return u.Scheme == "http" || u.Scheme == "https"
}

func (a *API) createSeason(w http.ResponseWriter, r *http.Request) {
	actor := actorFrom(r.Context())
	if !actor.IsDirectorOrSystemAdmin() || !actor.HasRecentMFA(time.Now()) {
		writeErrorResponse(w, http.StatusForbidden, "privileged_mfa_required", "Director or System Admin access with recent MFA is required.")
		return
	}
	var request seasonInput
	if err := decodeJSON(r.Body, &request); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "validation_failed", err.Error())
		return
	}
	if !validSeason(request) {
		writeErrorResponse(w, http.StatusBadRequest, "validation_failed", "valid name, slug, dates, and HTTPS URLs are required")
		return
	}

	seasonRecord := programme.SeasonRecord{
		ID:           id.New(),
		Slug:         request.Slug,
		Name:         request.Name,
		Status:       "open",
		StartAt:      request.StartAt.UTC(),
		EndAt:        request.EndAt.UTC(),
		Location:     request.Location,
		ImageURL:     request.ImageURL,
		ResourcesURL: request.ResourcesURL,
	}
	created, err := a.db.CreateSeason(r.Context(), seasonRecord, actor.UserID, time.Now().UTC())
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}

	writeJSONResponse(w, http.StatusCreated, created)
}

func (a *API) updateSeason(w http.ResponseWriter, r *http.Request) {
	actor := actorFrom(r.Context())
	seasonID := r.PathValue("id")
	if !actor.IsDirectorOrSystemAdmin() || !actor.HasRecentMFA(time.Now()) {
		writeErrorResponse(w, http.StatusForbidden, "privileged_mfa_required", "Director or System Admin access with recent MFA is required.")
		return
	}

	current, err := a.db.GetSeason(r.Context(), seasonID)
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}

	if current.Status != "open" {
		writeErrorResponse(w, http.StatusConflict, "season_closed", "The season must be reopened before its definition can be changed.")
		return
	}

	var request seasonInput
	if err := decodeJSON(r.Body, &request); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "validation_failed", err.Error())
		return
	}
	if !validSeason(request) {
		writeErrorResponse(w, http.StatusBadRequest, "validation_failed", "valid fields are required")
		return
	}

	updated, err := a.db.UpdateSeasonDefinition(r.Context(), dal.UpdateSeasonDefinitionInput{
		SeasonID:     seasonID,
		Name:         request.Name,
		Slug:         request.Slug,
		Location:     request.Location,
		ImageURL:     request.ImageURL,
		ResourcesURL: request.ResourcesURL,
		StartAt:      request.StartAt.UTC(),
		EndAt:        request.EndAt.UTC(),
		ActorID:      actor.UserID,
		ChangedAt:    time.Now().UTC(),
	})
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}

	writeJSONResponse(w, http.StatusOK, updated)
}

type updateSeasonResourcesRequest struct {
	ResourcesURL string `json:"resourcesUrl"`
}

func (a *API) updateSeasonResources(w http.ResponseWriter, r *http.Request) {
	actor := actorFrom(r.Context())
	seasonID := r.PathValue("id")
	managedSeason, err := a.db.GetSeason(r.Context(), r.PathValue("id"))
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}
	if !actor.IsSeasonAdmin(managedSeason.ID) {
		writeErrorResponse(w, http.StatusForbidden, "season_admin_required", "Season administrator access is required.")
		return
	}
	if managedSeason.Status != "open" {
		writeErrorResponse(w, http.StatusForbidden, "season_admin_required", "The season must be open.")
		return
	}
	if !actor.HasRecentMFA(time.Now()) {
		writeErrorResponse(w, http.StatusForbidden, "season_admin_required", "Recent MFA is required.")
		return
	}

	var request updateSeasonResourcesRequest
	if err := decodeJSON(r.Body, &request); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "validation_failed", err.Error())
		return
	}
	if request.ResourcesURL != "" && !validWebURL(request.ResourcesURL, true) {
		writeErrorResponse(w, http.StatusBadRequest, "validation_failed", "an HTTPS resourcesUrl is required")
		return
	}
	updated, err := a.db.UpdateSeasonResources(r.Context(), dal.UpdateSeasonResourcesInput{
		SeasonID:     seasonID,
		ResourcesURL: request.ResourcesURL,
		ActorID:      actor.UserID,
		ChangedAt:    time.Now().UTC(),
	})
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}

	writeJSONResponse(w, http.StatusOK, updated)
}

type closeSeasonRequest struct {
	Reason string `json:"reason"`
}

func (a *API) closeSeason(w http.ResponseWriter, r *http.Request) {
	actor := actorFrom(r.Context())
	seasonID := r.PathValue("id")
	season, err := a.db.GetSeason(r.Context(), seasonID)
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}

	if !actor.CanCloseSeason(seasonID) || !actor.HasRecentMFA(time.Now()) {
		writeErrorResponse(w, http.StatusForbidden, "season_admin_required", "Recent MFA and season administration access are required.")
		return
	}

	if season.Status != "open" {
		writeErrorResponse(w, http.StatusConflict, "season_closed", "The season is already closed.")
		return
	}

	var request closeSeasonRequest
	if err := decodeJSON(r.Body, &request); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "validation_failed", err.Error())
		return
	}
	if strings.TrimSpace(request.Reason) == "" {
		writeErrorResponse(w, http.StatusBadRequest, "validation_failed", "reason is required")
		return
	}

	updated, err := a.db.CloseSeason(r.Context(), dal.CloseSeasonInput{
		SeasonID:  seasonID,
		ActorID:   actor.UserID,
		Reason:    strings.TrimSpace(request.Reason),
		ChangedAt: time.Now().UTC(),
	})
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}

	writeJSONResponse(w, http.StatusOK, updated)
}

type reopenSeasonRequest struct {
	Reason string `json:"reason"`
}

func (a *API) reopenSeason(w http.ResponseWriter, r *http.Request) {
	actor := actorFrom(r.Context())
	if !actor.CanReopenSeason(time.Now()) {
		writeErrorResponse(w, http.StatusForbidden, "privileged_mfa_required", "Director or System Admin access with recent MFA is required.")
		return
	}

	seasonID := r.PathValue("id")
	var request reopenSeasonRequest
	if err := decodeJSON(r.Body, &request); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "validation_failed", err.Error())
		return
	}
	if strings.TrimSpace(request.Reason) == "" {
		writeErrorResponse(w, http.StatusBadRequest, "validation_failed", "reason is required")
		return
	}

	updated, err := a.db.ReopenSeason(r.Context(), dal.ReopenSeasonInput{
		SeasonID:  seasonID,
		ActorID:   actor.UserID,
		Reason:    strings.TrimSpace(request.Reason),
		ChangedAt: time.Now().UTC(),
	})
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}

	writeJSONResponse(w, http.StatusOK, updated)
}
