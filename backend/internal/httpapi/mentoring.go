package httpapi

import (
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/magedmg/RSP-website/backend/internal/model"
	"github.com/magedmg/RSP-website/backend/internal/platform/id"
)

func (a *API) seasonAdmin(r *http.Request) (model.Season, bool) {
	seasonID := r.PathValue("id")
	season, err := a.store.GetSeason(r.Context(), seasonID)
	if err != nil {
		return model.Season{}, false
	}
	return season, actorFrom(r.Context()).CanManageSeason(seasonID, season.Status == "open")
}
func (a *API) listWeeks(w http.ResponseWriter, r *http.Request) {
	a.mu.Lock()
	defer a.mu.Unlock()
	items := []model.Week{}
	for _, v := range a.weeks {
		if v.SeasonID == r.PathValue("id") {
			items = append(items, v)
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Number < items[j].Number })
	writeJSON(w, 200, model.Page[model.Week]{Items: items, PageInfo: model.PageInfo{HasMore: false}, TotalCount: int64(len(items))})
}
func (a *API) createWeek(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.seasonAdmin(r); !ok {
		a.fail(w, r, 403, "season_admin_required", "Season administration required", "Only an administrator of an open season can create weeks.", nil)
		return
	}
	var in struct {
		Number         int `json:"number"`
		StartAt, EndAt time.Time
		ResourceURL    string `json:"resourceUrl"`
	}
	if err := decodeJSON(w, r, &in); err != nil || in.Number < 1 || !in.EndAt.After(in.StartAt) {
		validation(a, w, r, "number and valid dates are required")
		return
	}
	v := model.Week{ID: id.New(), SeasonID: r.PathValue("id"), Number: in.Number, StartAt: in.StartAt.UTC(), EndAt: in.EndAt.UTC(), ResourceURL: in.ResourceURL, Revision: 1}
	a.mu.Lock()
	for _, old := range a.weeks {
		if old.SeasonID == v.SeasonID && old.Number == v.Number {
			a.mu.Unlock()
			a.fail(w, r, 409, "duplicate", "Conflict", "That week number already exists.", nil)
			return
		}
	}
	a.weeks[v.ID] = v
	a.mu.Unlock()
	a.audit(r, "week.created", "week", v.ID, nil)
	writeJSON(w, 201, v)
}
func (a *API) listMembers(w http.ResponseWriter, r *http.Request) {
	actor := actorFrom(r.Context())
	if !actor.EligibleMember() && !actor.IsPrivileged() {
		a.fail(w, r, 403, "season_access_required", "Season access required", "No season access yet.", nil)
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	items := []model.Enrollment{}
	for _, v := range a.enrollments {
		if v.SeasonID == r.PathValue("id") {
			items = append(items, v)
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	writeJSON(w, 200, model.Page[model.Enrollment]{Items: items, PageInfo: model.PageInfo{HasMore: false}, TotalCount: int64(len(items))})
}
func (a *API) createMember(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.seasonAdmin(r); !ok {
		a.fail(w, r, 403, "season_admin_required", "Season administration required", "Only an administrator of an open season can add members.", nil)
		return
	}
	var in struct{ UserID, Role string }
	if err := decodeJSON(w, r, &in); err != nil || (in.Role != "student" && in.Role != "mentor" && in.Role != "coordinator") || strings.TrimSpace(in.UserID) == "" {
		validation(a, w, r, "userId and a valid role are required")
		return
	}
	v := model.Enrollment{ID: id.New(), SeasonID: r.PathValue("id"), UserID: in.UserID, Role: in.Role, State: "active", Revision: 1}
	a.mu.Lock()
	for _, old := range a.enrollments {
		if old.SeasonID == v.SeasonID && old.UserID == v.UserID {
			a.mu.Unlock()
			a.fail(w, r, 409, "duplicate", "Conflict", "The user already has a season role.", nil)
			return
		}
	}
	a.enrollments[v.ID] = v
	a.mu.Unlock()
	a.audit(r, "enrollment.created", "enrollment", v.ID, nil)
	writeJSON(w, 201, v)
}
func (a *API) promoteMember(w http.ResponseWriter, r *http.Request) { a.changeMember(w, r, true) }
func (a *API) removeMember(w http.ResponseWriter, r *http.Request)  { a.changeMember(w, r, false) }
func (a *API) changeMember(w http.ResponseWriter, r *http.Request, promote bool) {
	actor := actorFrom(r.Context())
	seasonID, memberID := r.PathValue("id"), r.PathValue("memberId")
	a.mu.Lock()
	v, ok := a.enrollments[memberID]
	assigned := false
	for _, m := range a.mentorships {
		if m.SeasonID == seasonID && m.MentorUserID == actor.UserID && m.StudentUserID == v.UserID {
			assigned = true
		}
	}
	a.mu.Unlock()
	if !ok || v.SeasonID != seasonID {
		a.fail(w, r, 404, "not_found", "Not found", "The enrollment does not exist.", nil)
		return
	}
	if !actor.CanPromoteOrRemove(seasonID, assigned, v.State == "active") {
		a.fail(w, r, 403, "member_admin_required", "Member administration required", "This member cannot be changed by the current account.", nil)
		return
	}
	var in struct {
		Role, Reason string
		Revision     int64
	}
	if err := decodeJSON(w, r, &in); err != nil || in.Revision != v.Revision || (!promote && strings.TrimSpace(in.Reason) == "") {
		if in.Revision != v.Revision {
			a.fail(w, r, 409, "stale_revision", "Conflict", "The enrollment changed since it was loaded.", nil)
		} else {
			validation(a, w, r, "revision and removal reason are required")
		}
		return
	}
	if promote {
		if in.Role != "mentor" && in.Role != "coordinator" {
			validation(a, w, r, "role must be mentor or coordinator")
			return
		}
		v.Role = in.Role
	} else {
		v.State = "kicked"
		reason := strings.TrimSpace(in.Reason)
		v.RemovalReason = &reason
	}
	v.Revision++
	a.mu.Lock()
	a.enrollments[memberID] = v
	a.mu.Unlock()
	action := "enrollment.promoted"
	if !promote {
		action = "enrollment.removed"
	}
	a.audit(r, action, "enrollment", v.ID, map[string]any{"reason": in.Reason, "role": in.Role})
	writeJSON(w, 200, v)
}
func (a *API) listMentorships(w http.ResponseWriter, r *http.Request) {
	a.mu.Lock()
	defer a.mu.Unlock()
	items := []model.Mentorship{}
	for _, v := range a.mentorships {
		if v.SeasonID == r.PathValue("id") {
			items = append(items, v)
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	writeJSON(w, 200, model.Page[model.Mentorship]{Items: items, PageInfo: model.PageInfo{HasMore: false}, TotalCount: int64(len(items))})
}
func (a *API) createMentorship(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.seasonAdmin(r); !ok {
		a.fail(w, r, 403, "season_admin_required", "Season administration required", "Only an administrator of an open season can assign mentorships.", nil)
		return
	}
	var in struct{ MentorUserID, StudentUserID string }
	if err := decodeJSON(w, r, &in); err != nil || in.MentorUserID == "" || in.StudentUserID == "" || in.MentorUserID == in.StudentUserID {
		validation(a, w, r, "different mentorUserId and studentUserId are required")
		return
	}
	v := model.Mentorship{ID: id.New(), SeasonID: r.PathValue("id"), MentorUserID: in.MentorUserID, StudentUserID: in.StudentUserID, Revision: 1}
	a.mu.Lock()
	a.mentorships[v.ID] = v
	a.mu.Unlock()
	a.audit(r, "mentorship.created", "mentorship", v.ID, nil)
	writeJSON(w, 201, v)
}
