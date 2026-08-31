package httpapi

import (
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/magedmg/RSP-website/backend/internal/accounts"
	"github.com/magedmg/RSP-website/backend/internal/authz"
	"github.com/magedmg/RSP-website/backend/internal/platform/cursor"
	"github.com/magedmg/RSP-website/backend/internal/platform/id"
	"github.com/magedmg/RSP-website/backend/internal/programme"
	"github.com/magedmg/RSP-website/backend/internal/store"
)

type meSeasonRole struct {
	SeasonID   string `json:"seasonId"`
	SeasonSlug string `json:"seasonSlug"`
	Role       string `json:"role"`
	State      string `json:"state"`
}

type meResponse struct {
	accounts.User
	EmailVerified bool           `json:"emailVerified"`
	MFAVerified   bool           `json:"mfaVerified"`
	SeasonRoles   []meSeasonRole `json:"seasonRoles"`
	Alumni        bool           `json:"alumni"`
}

func (a *API) currentUserResponse(r *http.Request, actor authz.Actor, user accounts.User) (meResponse, error) {
	roles := make([]meSeasonRole, 0, len(actor.Enrollments))
	for _, enrollment := range actor.Enrollments {
		if enrollment.State != authz.Active && enrollment.State != authz.Completed {
			continue
		}

		season, err := a.store.GetSeason(r.Context(), enrollment.SeasonID)
		if err != nil {
			return meResponse{}, err
		}

		roles = append(roles, meSeasonRole{SeasonID: enrollment.SeasonID, SeasonSlug: season.Slug, Role: string(enrollment.Role), State: string(enrollment.State)})
	}
	sort.Slice(roles, func(i, j int) bool { return roles[i].SeasonSlug < roles[j].SeasonSlug })
	return meResponse{User: user, EmailVerified: actor.EmailVerified, MFAVerified: actor.HasRecentMFA(time.Now()), SeasonRoles: roles, Alumni: actor.Alumni()}, nil
}

func (a *API) me(w http.ResponseWriter, r *http.Request) {
	actor := actorFrom(r.Context())
	user, err := a.store.GetUser(r.Context(), actor.UserID)
	if err != nil {
		storeFailure(a, w, r, err)
		return
	}

	response, err := a.currentUserResponse(r, actor, user)
	if err != nil {
		storeFailure(a, w, r, err)
		return
	}

	writeJSONResponse(w, http.StatusOK, response)
}

func (a *API) suggestMeSlug(w http.ResponseWriter, r *http.Request) {
	slug, err := a.store.SuggestUserSlug(r.Context())
	if err != nil {
		storeFailure(a, w, r, err)
		return
	}

	writeJSONResponse(w, http.StatusOK, map[string]string{"slug": slug})
}

func (a *API) updateMe(w http.ResponseWriter, r *http.Request) {
	actor := actorFrom(r.Context())
	var in struct {
		Name      string  `json:"name"`
		Slug      *string `json:"slug"`
		AvatarURL *string `json:"avatarUrl"`
		Timezone  string  `json:"timezone"`
		Revision  int64   `json:"revision"`
	}
	if err := decodeJSON(w, r, &in); err != nil {
		validation(a, w, r, err.Error())
		return
	}
	if strings.TrimSpace(in.Name) == "" || in.Revision < 1 {
		validation(a, w, r, "name and revision are required")
		return
	}
	if _, err := time.LoadLocation(in.Timezone); err != nil {
		validation(a, w, r, "timezone must be an IANA timezone")
		return
	}
	if in.AvatarURL != nil && *in.AvatarURL != "" && !validWebURL(*in.AvatarURL, true) {
		validation(a, w, r, "avatarUrl must be an HTTPS URL")
		return
	}

	if in.Slug != nil {
		slug := strings.ToLower(strings.TrimSpace(*in.Slug))
		if len(slug) < 3 || len(slug) > 50 || !validSlug(slug) {
			validation(a, w, r, "slug must be 3-50 lowercase letters, numbers, or hyphens")
			return
		}
		in.Slug = &slug
	}

	updated, err := a.store.UpdateUser(r.Context(), actor.UserID, in.Revision, func(u *accounts.User) error {
		u.Name = strings.TrimSpace(in.Name)
		if in.Slug != nil {
			u.Slug = *in.Slug
		}
		u.AvatarURL = in.AvatarURL
		u.Timezone = in.Timezone
		u.TimezoneConfigured = true
		return nil
	}, actor.UserID, time.Now().UTC())
	if err != nil {
		storeFailure(a, w, r, err)
		return
	}

	response, err := a.currentUserResponse(r, actor, updated)
	if err != nil {
		storeFailure(a, w, r, err)
		return
	}

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

func (a *API) users(w http.ResponseWriter, r *http.Request) {
	actor := actorFrom(r.Context())
	if !canAccessDirectory(actor) && !actor.IsPrivileged() {
		writeErrorResponse(w, r, 403, "season_access_required", "Season access required", "No season access yet.")
		return
	}

	_, err := requestedSort(r, "id:asc", "id:asc")
	if err != nil {
		validation(a, w, r, "sort must be id:asc")
		return
	}

	direction := r.URL.Query().Get("direction")
	if direction == "" {
		direction = "forward"
	}

	if direction != "forward" && direction != "backward" {
		validation(a, w, r, "direction must be forward or backward")
		return
	}

	query := strings.TrimSpace(r.URL.Query().Get("query"))
	seasonRole, globalRole := r.URL.Query().Get("seasonRole"), r.URL.Query().Get("globalRole")
	if len(query) > 100 || seasonRole != "" && seasonRole != "student" && seasonRole != "mentor" && seasonRole != "coordinator" || globalRole != "" && globalRole != "director" && globalRole != "system_admin" {
		validation(a, w, r, "query and valid seasonRole/globalRole filters are required")
		return
	}

	binding := "users|sort=id:asc|query=" + query + "|seasonRole=" + seasonRole + "|globalRole=" + globalRole
	limit, boundary, err := a.page(r, binding)
	if err != nil {
		writeErrorResponse(w, r, 400, "invalid_cursor", "Invalid cursor", cursor.ErrInvalid.Error())
		return
	}

	items, more, total, err := a.store.ListUsers(r.Context(), boundary, limit, direction, query, seasonRole, globalRole)
	if err != nil {
		storeFailure(a, w, r, err)
		return
	}

	for i := range items {
		items[i].Email = ""
		items[i].AccountState = ""
	}
	pageInfo := pageInfoForKeyset(
		a, binding, direction, boundary, items, more,
		func(v accounts.User) string { return v.ID },
	)
	response := Page[accounts.User]{
		Items:      items,
		PageInfo:   pageInfo,
		TotalCount: total,
	}
	writeJSONResponse(w, http.StatusOK, response)
}

func (a *API) user(w http.ResponseWriter, r *http.Request) {
	actor := actorFrom(r.Context())
	if !canAccessDirectory(actor) && !actor.IsPrivileged() {
		writeErrorResponse(w, r, 403, "season_access_required", "Season access required", "No season access yet.")
		return
	}

	user, err := a.store.GetUser(r.Context(), r.PathValue("id"))
	if err != nil {
		storeFailure(a, w, r, err)
		return
	}

	if actor.UserID != user.ID && !actor.IsPrivileged() {
		participant, eligibilityErr := a.store.GetMockParticipant(r.Context(), user.ID)
		if eligibilityErr != nil || !participant.DirectoryEligible() {
			writeErrorResponse(w, r, 404, "not_found", "Not found", "The requested resource does not exist.")
			return
		}
	}
	if actor.UserID != user.ID && !actor.IsPrivileged() {
		canPrivate, permissionErr := a.canViewMemberPrivate(r, user.ID)
		if permissionErr != nil {
			storeFailure(a, w, r, permissionErr)
			return
		}
		if !canPrivate {
			user.Email = ""
			user.AccountState = ""
		}
	}
	if !a.auditSystemAdminPrivateRead(w, r, "user", user.ID) {
		return
	}
	writeJSONResponse(w, http.StatusOK, user)
}

func canAccessDirectory(actor authz.Actor) bool {
	return actor.EligibleMember()
}

func (a *API) seasons(w http.ResponseWriter, r *http.Request) {
	actor := actorFrom(r.Context())
	if !actor.ProgrammeAccess() && !actor.IsPrivileged() {
		writeErrorResponse(w, r, 403, "season_access_required", "Season access required", "No season access yet.")
		return
	}

	_, err := requestedSort(r, "id:asc", "id:asc")
	if err != nil {
		validation(a, w, r, "sort must be id:asc")
		return
	}

	direction := r.URL.Query().Get("direction")
	if direction == "" {
		direction = "forward"
	}
	if direction != "forward" && direction != "backward" {
		validation(a, w, r, "direction must be forward or backward")
		return
	}

	status := r.URL.Query().Get("status")
	if status != "" && status != "open" && status != "closed" {
		validation(a, w, r, "status must be open or closed")
		return
	}

	binding := "seasons|sort=id:asc|status=" + status
	if actor.IsPrivileged() {
		limit, boundary, pageErr := a.page(r, binding)
		if pageErr != nil {
			writeErrorResponse(w, r, 400, "invalid_cursor", "Invalid cursor", cursor.ErrInvalid.Error())
			return
		}
		items, more, total, listErr := a.store.ListSeasons(r.Context(), boundary, limit, direction, status)
		if listErr != nil {
			storeFailure(a, w, r, listErr)
			return
		}
		pageInfo := pageInfoForKeyset(
			a, binding, direction, boundary, items, more,
			func(v programme.SeasonRecord) string { return v.ID },
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
		if seen[enrollment.SeasonID] || !canViewSeason(actor, enrollment.SeasonID) {
			continue
		}
		season, seasonErr := a.store.GetSeason(r.Context(), enrollment.SeasonID)
		if seasonErr != nil {
			storeFailure(a, w, r, seasonErr)
			return
		}
		if status != "" && season.Status != status {
			continue
		}
		seen[season.ID] = true
		items = append(items, season)
	}

	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	page, pageInfo, err := paginateOrdered(a, r, binding, items, func(v programme.SeasonRecord) string { return v.ID })
	if err != nil {
		writeErrorResponse(w, r, 400, "invalid_cursor", "Invalid cursor", "The cursor does not match the selected season sort.")
		return
	}

	response := Page[programme.SeasonRecord]{
		Items:      page,
		PageInfo:   pageInfo,
		TotalCount: int64(len(items)),
	}
	writeJSONResponse(w, http.StatusOK, response)
}

func (a *API) season(w http.ResponseWriter, r *http.Request) {
	actor := actorFrom(r.Context())
	if !canViewSeason(actor, r.PathValue("id")) {
		writeErrorResponse(w, r, 404, "not_found", "Not found", "The requested season does not exist.")
		return
	}

	v, err := a.store.GetSeason(r.Context(), r.PathValue("id"))
	if err != nil {
		storeFailure(a, w, r, err)
		return
	}

	writeJSONResponse(w, http.StatusOK, v)
}

func canViewSeason(actor authz.Actor, seasonID string) bool {
	if actor.IsPrivileged() {
		return true
	}
	for _, enrollment := range actor.Enrollments {
		if enrollment.SeasonID != seasonID {
			continue
		}
		if enrollment.State == authz.Active || enrollment.State == authz.Completed && actor.EligibleMember() {
			return true
		}
	}
	return false
}

type seasonInput struct {
	Name         string    `json:"name"`
	Slug         string    `json:"slug"`
	Location     string    `json:"location"`
	ImageURL     string    `json:"imageUrl"`
	ResourcesURL string    `json:"resourcesUrl"`
	StartAt      time.Time `json:"startAt"`
	EndAt        time.Time `json:"endAt"`
	Revision     int64     `json:"revision"`
}

func validSeason(in seasonInput) bool {
	if strings.TrimSpace(in.Name) == "" || !validSeasonSlug(in.Slug) || !in.EndAt.After(in.StartAt) {
		return false
	}
	for _, raw := range []string{in.ImageURL, in.ResourcesURL} {
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
	if !actor.IsPrivileged() || !actor.HasRecentMFA(time.Now()) {
		writeErrorResponse(w, r, 403, "privileged_mfa_required", "Privileged access required", "Director or System Admin access with recent MFA is required.")
		return
	}
	var in seasonInput
	if err := decodeJSON(w, r, &in); err != nil || !validSeason(in) {
		validation(a, w, r, "valid name, slug, dates, and HTTPS URLs are required")
		return
	}

	v := programme.SeasonRecord{ID: id.New(), Slug: in.Slug, Name: in.Name, Status: "open", StartAt: in.StartAt.UTC(), EndAt: in.EndAt.UTC(), Location: in.Location, ImageURL: in.ImageURL, ResourcesURL: in.ResourcesURL, Revision: 1}
	created, err := a.store.CreateSeason(r.Context(), v, actor.UserID, time.Now().UTC())
	if err != nil {
		storeFailure(a, w, r, err)
		return
	}

	writeJSONResponse(w, http.StatusCreated, created)
}

func (a *API) updateSeason(w http.ResponseWriter, r *http.Request) {
	actor := actorFrom(r.Context())
	seasonID := r.PathValue("id")
	if !actor.IsPrivileged() || !actor.HasRecentMFA(time.Now()) {
		writeErrorResponse(w, r, 403, "privileged_mfa_required", "Privileged access required", "Director or System Admin access with recent MFA is required.")
		return
	}

	current, err := a.store.GetSeason(r.Context(), seasonID)
	if err != nil {
		storeFailure(a, w, r, err)
		return
	}

	if current.Status != "open" {
		writeErrorResponse(w, r, http.StatusConflict, "season_closed", "Season closed", "The season must be reopened before its definition can be changed.")
		return
	}

	var in seasonInput
	if err := decodeJSON(w, r, &in); err != nil || !validSeason(in) || in.Revision < 1 {
		validation(a, w, r, "valid fields and revision are required")
		return
	}

	updated, err := a.store.UpdateSeason(r.Context(), seasonID, in.Revision, func(v *programme.SeasonRecord) error {
		v.Name = in.Name
		v.Slug = in.Slug
		v.Location = in.Location
		v.ImageURL = in.ImageURL
		v.ResourcesURL = in.ResourcesURL
		v.StartAt = in.StartAt.UTC()
		v.EndAt = in.EndAt.UTC()
		return nil
	}, actor.UserID, time.Now().UTC())
	if err != nil {
		storeFailure(a, w, r, err)
		return
	}

	writeJSONResponse(w, http.StatusOK, updated)
}

func (a *API) updateSeasonResources(w http.ResponseWriter, r *http.Request) {
	seasonID := r.PathValue("id")
	season, ok := a.seasonAdmin(r)
	if !ok || season.ID != seasonID {
		writeErrorResponse(w, r, http.StatusForbidden, "season_admin_required", "Season administration required", "An administrator of this open season with recent MFA is required.")
		return
	}

	var in struct {
		ResourcesURL string `json:"resourcesUrl"`
		Revision     int64  `json:"revision"`
	}
	if err := decodeJSON(w, r, &in); err != nil || in.Revision < 1 || in.ResourcesURL != "" && !validWebURL(in.ResourcesURL, true) {
		validation(a, w, r, "an HTTPS resourcesUrl and revision are required")
		return
	}

	actor := actorFrom(r.Context())
	updated, err := a.store.UpdateSeason(r.Context(), seasonID, in.Revision, func(v *programme.SeasonRecord) error {
		v.ResourcesURL = in.ResourcesURL
		return nil
	}, actor.UserID, time.Now().UTC())
	if err != nil {
		storeFailure(a, w, r, err)
		return
	}

	writeJSONResponse(w, http.StatusOK, updated)
}

func (a *API) closeSeason(w http.ResponseWriter, r *http.Request) {
	actor := actorFrom(r.Context())
	seasonID := r.PathValue("id")
	season, err := a.store.GetSeason(r.Context(), seasonID)
	if err != nil {
		storeFailure(a, w, r, err)
		return
	}

	if !actor.CanCloseSeason(seasonID) || !actor.HasRecentMFA(time.Now()) {
		writeErrorResponse(w, r, 403, "season_admin_required", "Season administration required", "Recent MFA and season administration access are required.")
		return
	}

	if season.Status != "open" {
		storeFailure(a, w, r, store.ErrConflict)
		return
	}

	var in struct {
		Reason   string `json:"reason"`
		Revision int64  `json:"revision"`
	}
	if err := decodeJSON(w, r, &in); err != nil || strings.TrimSpace(in.Reason) == "" {
		validation(a, w, r, "reason and revision are required")
		return
	}

	updated, err := a.store.CloseSeason(r.Context(), seasonID, in.Revision, actor.UserID, strings.TrimSpace(in.Reason), time.Now().UTC())
	if err != nil {
		storeFailure(a, w, r, err)
		return
	}

	writeJSONResponse(w, http.StatusOK, updated)
}

func (a *API) reopenSeason(w http.ResponseWriter, r *http.Request) {
	actor := actorFrom(r.Context())
	if !actor.CanReopenSeason(time.Now()) {
		writeErrorResponse(w, r, 403, "privileged_mfa_required", "Privileged access required", "Director or System Admin access with recent MFA is required.")
		return
	}

	seasonID := r.PathValue("id")
	var in struct {
		Reason   string `json:"reason"`
		Revision int64  `json:"revision"`
	}
	if err := decodeJSON(w, r, &in); err != nil || strings.TrimSpace(in.Reason) == "" {
		validation(a, w, r, "reason and revision are required")
		return
	}

	updated, err := a.store.ReopenSeason(r.Context(), seasonID, in.Revision, actor.UserID, strings.TrimSpace(in.Reason), time.Now().UTC())
	if err != nil {
		storeFailure(a, w, r, err)
		return
	}

	writeJSONResponse(w, http.StatusOK, updated)
}
