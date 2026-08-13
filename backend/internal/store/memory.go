package store

import (
	"context"
	"errors"
	"sort"
	"strings"
	"sync"

	"github.com/magedmg/RSP-website/backend/internal/authz"
	"github.com/magedmg/RSP-website/backend/internal/model"
	"github.com/magedmg/RSP-website/backend/internal/platform/id"
)

type Memory struct {
	mu           sync.RWMutex
	Users        map[string]model.User
	AuthSubjects map[string]authz.Actor
	Seasons      map[string]model.Season
	Problems     map[string]model.Problem
	Attempts     map[string]model.Attempt
	Audits       []model.AuditEvent
}

func NewMemory() *Memory {
	return &Memory{Users: map[string]model.User{}, AuthSubjects: map[string]authz.Actor{}, Seasons: map[string]model.Season{}, Problems: map[string]model.Problem{}, Attempts: map[string]model.Attempt{}}
}
func (m *Memory) ApplyIdentityEvent(_ context.Context, event IdentityEvent) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	actor, exists := m.AuthSubjects[event.AuthUserID]
	switch event.Type {
	case "auth_user_created":
		if exists {
			return nil
		}
		name := strings.TrimSpace(strings.Split(event.Email, "@")[0])
		if name == "" {
			name = "New member"
		}
		userID := id.New()
		slug := "member-" + strings.ReplaceAll(userID, "-", "")[:12]
		m.Users[userID] = model.User{ID: userID, Slug: slug, Name: name, Email: event.Email, Timezone: "Australia/Adelaide", AccountState: "active", Revision: 1}
		m.AuthSubjects[event.AuthUserID] = authz.Actor{UserID: userID, EmailVerified: event.EmailVerified, AccountState: authz.AccountActive, GlobalRoles: map[authz.GlobalRole]bool{}}
	case "email_verified":
		if !exists {
			return ErrNotFound
		}
		actor.EmailVerified = true
		m.AuthSubjects[event.AuthUserID] = actor
	case "deletion_requested", "deletion_cancelled", "auth_pseudonymized":
		if !exists {
			return ErrNotFound
		}
		state := authz.DeletionPending
		if event.Type == "deletion_cancelled" {
			state = authz.AccountActive
		}
		if event.Type == "auth_pseudonymized" {
			state = authz.Deleted
		}
		actor.AccountState = state
		m.AuthSubjects[event.AuthUserID] = actor
		user := m.Users[actor.UserID]
		user.AccountState = string(state)
		user.Revision++
		if state == authz.Deleted {
			user.Name = "Deleted member"
			user.Email = ""
			user.AvatarURL = nil
		}
		m.Users[user.ID] = user
	case "sessions_revoked":
		if !exists {
			return ErrNotFound
		}
	default:
		return errors.New("unsupported identity event")
	}
	return nil
}
func (m *Memory) ResolveAuthSubject(_ context.Context, sub string) (authz.Actor, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	v, ok := m.AuthSubjects[sub]
	if !ok {
		return authz.Actor{}, ErrNotFound
	}
	return v, nil
}
func (m *Memory) GetUser(_ context.Context, id string) (model.User, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	v, ok := m.Users[id]
	if !ok {
		return model.User{}, ErrNotFound
	}
	return v, nil
}
func (m *Memory) ListUsers(_ context.Context, after string, limit int) ([]model.User, bool, int64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	items := make([]model.User, 0, len(m.Users))
	for _, v := range m.Users {
		if v.ID > after {
			items = append(items, v)
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	more := len(items) > limit
	if more {
		items = items[:limit]
	}
	return items, more, int64(len(m.Users)), nil
}
func (m *Memory) UpdateUser(_ context.Context, id string, revision int64, fn func(*model.User) error) (model.User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	v, ok := m.Users[id]
	if !ok {
		return model.User{}, ErrNotFound
	}
	if v.Revision != revision {
		return model.User{}, ErrConflict
	}
	if err := fn(&v); err != nil {
		return model.User{}, err
	}
	v.Revision++
	m.Users[id] = v
	return v, nil
}
func (m *Memory) GetSeason(_ context.Context, id string) (model.Season, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	v, ok := m.Seasons[id]
	if !ok {
		return model.Season{}, ErrNotFound
	}
	return v, nil
}
func (m *Memory) ListSeasons(_ context.Context, after string, limit int) ([]model.Season, bool, int64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	items := make([]model.Season, 0, len(m.Seasons))
	for _, v := range m.Seasons {
		if v.ID > after {
			items = append(items, v)
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	more := len(items) > limit
	if more {
		items = items[:limit]
	}
	return items, more, int64(len(m.Seasons)), nil
}
func (m *Memory) CreateSeason(_ context.Context, v model.Season) (model.Season, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.Seasons[v.ID]; ok {
		return model.Season{}, ErrDuplicate
	}
	for _, old := range m.Seasons {
		if old.Slug == v.Slug {
			return model.Season{}, ErrDuplicate
		}
	}
	m.Seasons[v.ID] = v
	return v, nil
}
func (m *Memory) UpdateSeason(_ context.Context, id string, revision int64, fn func(*model.Season) error) (model.Season, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	v, ok := m.Seasons[id]
	if !ok {
		return model.Season{}, ErrNotFound
	}
	if v.Revision != revision {
		return model.Season{}, ErrConflict
	}
	if err := fn(&v); err != nil {
		return model.Season{}, err
	}
	v.Revision++
	m.Seasons[id] = v
	return v, nil
}
func (m *Memory) ListProblems(_ context.Context, after string, limit int) ([]model.Problem, bool, int64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	items := make([]model.Problem, 0, len(m.Problems))
	for _, v := range m.Problems {
		if v.ID > after {
			items = append(items, v)
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	more := len(items) > limit
	if more {
		items = items[:limit]
	}
	return items, more, int64(len(m.Problems)), nil
}
func (m *Memory) GetAttempt(_ context.Context, id string) (model.Attempt, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	v, ok := m.Attempts[id]
	if !ok || v.DeletedAt != nil {
		return model.Attempt{}, ErrNotFound
	}
	return v, nil
}
func (m *Memory) ListAttempts(_ context.Context, userID, after string, limit int) ([]model.Attempt, bool, int64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	items := []model.Attempt{}
	var total int64
	for _, v := range m.Attempts {
		if v.UserID == userID && v.DeletedAt == nil {
			total++
			if v.ID > after {
				items = append(items, v)
			}
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	more := len(items) > limit
	if more {
		items = items[:limit]
	}
	return items, more, total, nil
}
func (m *Memory) CreateAttempt(_ context.Context, v model.Attempt) (model.Attempt, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.Attempts[v.ID]; ok {
		return model.Attempt{}, ErrDuplicate
	}
	m.Attempts[v.ID] = v
	return v, nil
}
func (m *Memory) UpdateAttempt(_ context.Context, id, userID string, revision int64, fn func(*model.Attempt) error) (model.Attempt, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	v, ok := m.Attempts[id]
	if !ok || v.DeletedAt != nil {
		return model.Attempt{}, ErrNotFound
	}
	if v.UserID != userID {
		return model.Attempt{}, ErrNotFound
	}
	if v.Revision != revision {
		return model.Attempt{}, ErrConflict
	}
	if err := fn(&v); err != nil {
		return model.Attempt{}, err
	}
	v.Revision++
	m.Attempts[id] = v
	return v, nil
}
func (m *Memory) DeleteAttempt(_ context.Context, id, userID string, revision int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	v, ok := m.Attempts[id]
	if !ok || v.DeletedAt != nil || v.UserID != userID {
		return ErrNotFound
	}
	if v.Revision != revision {
		return ErrConflict
	}
	delete(m.Attempts, id)
	return nil
}
func (m *Memory) AppendAudit(_ context.Context, v model.AuditEvent) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Audits = append(m.Audits, v)
	return nil
}
