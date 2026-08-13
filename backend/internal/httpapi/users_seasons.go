package httpapi

import (
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/magedmg/RSP-website/backend/internal/model"
	"github.com/magedmg/RSP-website/backend/internal/platform/cursor"
	"github.com/magedmg/RSP-website/backend/internal/platform/id"
	"github.com/magedmg/RSP-website/backend/internal/store"
)

func (a *API) me(w http.ResponseWriter, r *http.Request) {
	actor := actorFrom(r.Context())
	user, err := a.store.GetUser(r.Context(), actor.UserID)
	if err != nil {
		storeFailure(a, w, r, err)
		return
	}
	writeJSON(w, 200, user)
}
func (a *API) updateMe(w http.ResponseWriter, r *http.Request) {
	actor := actorFrom(r.Context())
	var in struct {
		Name      string  `json:"name"`
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
	updated, err := a.store.UpdateUser(r.Context(), actor.UserID, in.Revision, func(u *model.User) error {
		u.Name = strings.TrimSpace(in.Name)
		u.AvatarURL = in.AvatarURL
		u.Timezone = in.Timezone
		return nil
	})
	if err != nil {
		storeFailure(a, w, r, err)
		return
	}
	a.audit(r, "user.updated", "user", actor.UserID, nil)
	writeJSON(w, 200, updated)
}
func (a *API) users(w http.ResponseWriter, r *http.Request) {
	actor := actorFrom(r.Context())
	if !actor.EligibleMember() && !actor.IsPrivileged() {
		a.fail(w, r, 403, "season_access_required", "Season access required", "No season access yet.", nil)
		return
	}
	limit, after, err := a.page(r, "users|id:asc")
	if err != nil {
		a.fail(w, r, 400, "invalid_cursor", "Invalid cursor", cursor.ErrInvalid.Error(), nil)
		return
	}
	items, more, total, err := a.store.ListUsers(r.Context(), after, limit)
	if err != nil {
		storeFailure(a, w, r, err)
		return
	}
	for i := range items {
		items[i].Email = ""
		items[i].AccountState = ""
	}
	var last string
	if len(items) > 0 {
		last = items[len(items)-1].ID
	}
	writeJSON(w, 200, model.Page[model.User]{Items: items, PageInfo: model.PageInfo{NextCursor: a.next(last, "users|id:asc", more), HasMore: more}, TotalCount: total})
}
func (a *API) user(w http.ResponseWriter, r *http.Request) {
	actor := actorFrom(r.Context())
	if !actor.EligibleMember() && !actor.IsPrivileged() {
		a.fail(w, r, 403, "season_access_required", "Season access required", "No season access yet.", nil)
		return
	}
	user, err := a.store.GetUser(r.Context(), r.PathValue("id"))
	if err != nil {
		storeFailure(a, w, r, err)
		return
	}
	if actor.UserID != user.ID && !actor.IsPrivileged() {
		user.Email = ""
		user.AccountState = ""
	}
	writeJSON(w, 200, user)
}
func (a *API) seasons(w http.ResponseWriter, r *http.Request) {
	limit, after, err := a.page(r, "seasons|id:asc")
	if err != nil {
		a.fail(w, r, 400, "invalid_cursor", "Invalid cursor", cursor.ErrInvalid.Error(), nil)
		return
	}
	items, more, total, err := a.store.ListSeasons(r.Context(), after, limit)
	if err != nil {
		storeFailure(a, w, r, err)
		return
	}
	var last string
	if len(items) > 0 {
		last = items[len(items)-1].ID
	}
	writeJSON(w, 200, model.Page[model.Season]{Items: items, PageInfo: model.PageInfo{NextCursor: a.next(last, "seasons|id:asc", more), HasMore: more}, TotalCount: total})
}
func (a *API) season(w http.ResponseWriter, r *http.Request) {
	v, err := a.store.GetSeason(r.Context(), r.PathValue("id"))
	if err != nil {
		storeFailure(a, w, r, err)
		return
	}
	writeJSON(w, 200, v)
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
	if strings.TrimSpace(in.Name) == "" || strings.TrimSpace(in.Slug) == "" || !in.EndAt.After(in.StartAt) {
		return false
	}
	for _, raw := range []string{in.ImageURL, in.ResourcesURL} {
		if raw != "" {
			u, err := url.Parse(raw)
			if err != nil || u.Scheme != "https" {
				return false
			}
		}
	}
	return true
}
func (a *API) createSeason(w http.ResponseWriter, r *http.Request) {
	actor := actorFrom(r.Context())
	if !actor.IsPrivileged() || !actor.HasRecentMFA(time.Now()) {
		a.fail(w, r, 403, "privileged_mfa_required", "Privileged access required", "Director or System Admin access with recent MFA is required.", nil)
		return
	}
	var in seasonInput
	if err := decodeJSON(w, r, &in); err != nil || !validSeason(in) {
		validation(a, w, r, "valid name, slug, dates, and HTTPS URLs are required")
		return
	}
	v := model.Season{ID: id.New(), Slug: in.Slug, Name: in.Name, Status: "open", StartAt: in.StartAt.UTC(), EndAt: in.EndAt.UTC(), Location: in.Location, ImageURL: in.ImageURL, ResourcesURL: in.ResourcesURL, Revision: 1}
	created, err := a.store.CreateSeason(r.Context(), v)
	if err != nil {
		storeFailure(a, w, r, err)
		return
	}
	a.audit(r, "season.created", "season", created.ID, nil)
	writeJSON(w, 201, created)
}
func (a *API) updateSeason(w http.ResponseWriter, r *http.Request) {
	actor := actorFrom(r.Context())
	seasonID := r.PathValue("id")
	season, err := a.store.GetSeason(r.Context(), seasonID)
	if err != nil {
		storeFailure(a, w, r, err)
		return
	}
	if !actor.CanManageSeason(seasonID, season.Status == "open") || !actor.HasRecentMFA(time.Now()) {
		a.fail(w, r, 403, "season_admin_required", "Season administration required", "This season cannot be managed by the current account.", nil)
		return
	}
	var in seasonInput
	if err := decodeJSON(w, r, &in); err != nil || !validSeason(in) || in.Revision < 1 {
		validation(a, w, r, "valid fields and revision are required")
		return
	}
	updated, err := a.store.UpdateSeason(r.Context(), seasonID, in.Revision, func(v *model.Season) error {
		v.Name = in.Name
		v.Slug = in.Slug
		v.Location = in.Location
		v.ImageURL = in.ImageURL
		v.ResourcesURL = in.ResourcesURL
		v.StartAt = in.StartAt.UTC()
		v.EndAt = in.EndAt.UTC()
		return nil
	})
	if err != nil {
		storeFailure(a, w, r, err)
		return
	}
	a.audit(r, "season.updated", "season", seasonID, nil)
	writeJSON(w, 200, updated)
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
		a.fail(w, r, 403, "season_admin_required", "Season administration required", "Recent MFA and season administration access are required.", nil)
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
	updated, err := a.store.UpdateSeason(r.Context(), seasonID, in.Revision, func(v *model.Season) error { v.Status = "closed"; return nil })
	if err != nil {
		storeFailure(a, w, r, err)
		return
	}
	a.mu.Lock()
	for key, e := range a.enrollments {
		if e.SeasonID == seasonID && e.State == "active" {
			e.State = "completed"
			e.Revision++
			a.enrollments[key] = e
		}
	}
	a.mu.Unlock()
	a.audit(r, "season.closed", "season", seasonID, map[string]any{"reason": in.Reason})
	writeJSON(w, 200, updated)
}
func (a *API) reopenSeason(w http.ResponseWriter, r *http.Request) {
	actor := actorFrom(r.Context())
	if !actor.CanReopenSeason(time.Now()) {
		a.fail(w, r, 403, "privileged_mfa_required", "Privileged access required", "Director or System Admin access with recent MFA is required.", nil)
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
	updated, err := a.store.UpdateSeason(r.Context(), seasonID, in.Revision, func(v *model.Season) error { v.Status = "open"; return nil })
	if err != nil {
		storeFailure(a, w, r, err)
		return
	}
	a.audit(r, "season.reopened", "season", seasonID, map[string]any{"reason": in.Reason})
	writeJSON(w, 200, updated)
}
