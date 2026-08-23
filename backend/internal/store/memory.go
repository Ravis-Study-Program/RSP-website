package store

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/magedmg/RSP-website/backend/internal/authz"
	"github.com/magedmg/RSP-website/backend/internal/mockinterviews"
	"github.com/magedmg/RSP-website/backend/internal/model"
	"github.com/magedmg/RSP-website/backend/internal/platform/id"
	"github.com/magedmg/RSP-website/backend/internal/practice"
	"github.com/magedmg/RSP-website/backend/internal/programme"
)

// Memory represents a backend data structure.
type Memory struct {
	mu                       sync.RWMutex
	Users                    map[string]model.User
	PracticeSettings         map[string]model.PracticeSettings
	AuthSubjects             map[string]authz.Actor
	Seasons                  map[string]model.Season
	Weeks                    map[string]model.Week
	Enrollments              map[string]model.Enrollment
	Mentorships              map[string]model.Mentorship
	Problems                 map[string]model.Problem
	Attempts                 map[string]model.Attempt
	Recommendations          map[string]practice.Recommendation
	RecommendationDismissals map[string][]practice.Dismissal
	Mocks                    map[string]mockinterviews.Interview
	MockVersions             []MockVersion
	closeCompleted           map[string]map[string]bool
	closeAssignmentStates    map[string]map[string]string
	Audits                   []model.AuditEvent
	IdentityEventReceipts    map[string]bool
	IdentityEventHashes      map[string]string
	GlobalRoleAssignments    map[string]GlobalRoleAssignment
	MFAConfigured            map[string]bool
}

// NewMemory creates a new value.
func NewMemory() *Memory {
	return &Memory{Users: map[string]model.User{}, PracticeSettings: map[string]model.PracticeSettings{}, AuthSubjects: map[string]authz.Actor{}, Seasons: map[string]model.Season{}, Weeks: map[string]model.Week{}, Enrollments: map[string]model.Enrollment{}, Mentorships: map[string]model.Mentorship{}, Problems: map[string]model.Problem{}, Attempts: map[string]model.Attempt{}, Recommendations: map[string]practice.Recommendation{}, RecommendationDismissals: map[string][]practice.Dismissal{}, Mocks: map[string]mockinterviews.Interview{}, closeCompleted: map[string]map[string]bool{}, closeAssignmentStates: map[string]map[string]string{}, IdentityEventReceipts: map[string]bool{}, IdentityEventHashes: map[string]string{}, GlobalRoleAssignments: map[string]GlobalRoleAssignment{}, MFAConfigured: map[string]bool{}}
}

func memoryPage[T any](items []T, boundary string, limit int, direction string, identity func(T) string) ([]T, bool, error) {
	if limit < 1 {
		return nil, false, ErrConflict
	}
	boundaryIndex := -1
	if boundary != "" {
		for index := range items {
			if identity(items[index]) == boundary {
				boundaryIndex = index
				break
			}
		}
		if boundaryIndex < 0 {
			return nil, false, ErrNotFound
		}
	}
	start, end := 0, len(items)
	if direction == "backward" {
		if boundaryIndex >= 0 {
			end = boundaryIndex
		}
		start = max(0, end-limit)
		return items[start:end], start > 0, nil
	}
	if boundaryIndex >= 0 {
		start = boundaryIndex + 1
	}
	end = min(len(items), start+limit)
	return items[start:end], end < len(items), nil
}

// ApplyIdentityEvent applies the operation.
func (m *Memory) ApplyIdentityEvent(_ context.Context, event IdentityEvent) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if event.EventID == "" {
		return errors.New("identity event id is required")
	}
	payloadHash := IdentityEventHash(event)
	if m.IdentityEventReceipts[event.EventID] {
		if m.IdentityEventHashes[event.EventID] != payloadHash {
			return ErrConflict
		}
		return nil
	}
	actor, exists := m.AuthSubjects[event.AuthUserID]
	if exists && event.SecurityVersion < actor.SecurityVersion {
		m.IdentityEventReceipts[event.EventID] = true
		m.IdentityEventHashes[event.EventID] = payloadHash
		return nil
	}
	if exists && event.SecurityVersion > actor.SecurityVersion {
		actor.SecurityVersion = event.SecurityVersion
		m.AuthSubjects[event.AuthUserID] = actor
	}
	switch event.Type {
	case "auth_user_created":
		if exists {
			m.IdentityEventReceipts[event.EventID] = true
			m.IdentityEventHashes[event.EventID] = payloadHash
			return nil
		}
		name := strings.TrimSpace(strings.Split(event.Email, "@")[0])
		if name == "" {
			name = "New member"
		}
		userID := id.New()
		slug := "member-" + strings.ReplaceAll(userID, "-", "")[:12]
		m.Users[userID] = model.User{ID: userID, Slug: slug, Name: name, Email: event.Email, Timezone: "Australia/Adelaide", TimezoneConfigured: false, AccountState: "active", Revision: 1}
		m.AuthSubjects[event.AuthUserID] = authz.Actor{UserID: userID, EmailVerified: event.EmailVerified, AccountState: authz.AccountActive, GlobalRoles: map[authz.GlobalRole]bool{}, SecurityVersion: event.SecurityVersion}
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
			user.Slug = "deleted-" + user.ID
			user.Name = "Deleted member"
			user.Email = ""
			user.AvatarURL = nil
			user.Timezone = "UTC"
			user.TimezoneConfigured = false
		}
		m.Users[user.ID] = user
		action := map[string]string{"deletion_requested": "account.deletion_requested", "deletion_cancelled": "account.deletion_cancelled", "auth_pseudonymized": "account.pseudonymized"}[event.Type]
		m.appendAuditLocked(actor.UserID, action, "user", user.ID, nil, event.OccurredAt)
	case "sessions_revoked":
		if !exists {
			return ErrNotFound
		}
		m.appendAuditLocked(actor.UserID, "account.sessions_revoked", "user", actor.UserID, map[string]any{"reason": event.Reason}, event.OccurredAt)
	case "mfa_configured":
		if !exists {
			return ErrNotFound
		}
		m.MFAConfigured[actor.UserID] = true
		for enrollmentID, enrollment := range m.Enrollments {
			if enrollment.UserID == actor.UserID && enrollment.Role == "coordinator" && enrollment.State == "active" && enrollment.AssignmentState == "pending_mfa" {
				enrollment.AssignmentState = "active"
				enrollment.Revision++
				m.Enrollments[enrollmentID] = enrollment
				actor.Enrollments = append(actor.Enrollments, authz.Enrollment{SeasonID: enrollment.SeasonID, Role: authz.Coordinator, State: authz.Active})
				m.appendAuditLocked(actor.UserID, "coordinator_role.activated", "enrollment", enrollmentID, map[string]any{"seasonId": enrollment.SeasonID, "reason": "mfa_configured"}, event.OccurredAt)
			}
		}
		for _, states := range m.closeAssignmentStates {
			for enrollmentID, assignmentState := range states {
				enrollment := m.Enrollments[enrollmentID]
				if enrollment.UserID == actor.UserID && enrollment.Role == "coordinator" && assignmentState == "pending_mfa" {
					states[enrollmentID] = "active"
				}
			}
		}
		m.AuthSubjects[event.AuthUserID] = actor
		if actor.GlobalRoles == nil {
			actor.GlobalRoles = map[authz.GlobalRole]bool{}
		}
		for assignmentID, assignment := range m.GlobalRoleAssignments {
			if assignment.UserID == actor.UserID && assignment.State == "pending_mfa" {
				assignment.State, assignment.Revision = "active", assignment.Revision+1
				m.GlobalRoleAssignments[assignmentID] = assignment
				actor.GlobalRoles[authz.GlobalRole(assignment.Role)] = true
				m.appendAuditLocked(actor.UserID, "global_role.activated", "global_role_assignment", assignmentID, map[string]any{"role": assignment.Role, "reason": "mfa_configured"}, event.OccurredAt)
			}
		}
		m.AuthSubjects[event.AuthUserID] = actor
	case "mfa_disabled":
		if !exists {
			return ErrNotFound
		}
		m.MFAConfigured[actor.UserID] = false
		for assignmentID, assignment := range m.GlobalRoleAssignments {
			if assignment.UserID == actor.UserID && assignment.State == "active" {
				assignment.State, assignment.Revision = "pending_mfa", assignment.Revision+1
				m.GlobalRoleAssignments[assignmentID] = assignment
				delete(actor.GlobalRoles, authz.GlobalRole(assignment.Role))
				m.appendAuditLocked(actor.UserID, "global_role.pending_mfa", "global_role_assignment", assignmentID, map[string]any{"reason": "mfa_disabled"}, event.OccurredAt)
			}
		}
		for enrollmentID, enrollment := range m.Enrollments {
			if enrollment.UserID == actor.UserID && enrollment.Role == "coordinator" && enrollment.State == "active" && enrollment.AssignmentState == "active" {
				enrollment.AssignmentState, enrollment.Revision = "pending_mfa", enrollment.Revision+1
				m.Enrollments[enrollmentID] = enrollment
				m.appendAuditLocked(actor.UserID, "coordinator_role.pending_mfa", "enrollment", enrollmentID, map[string]any{"reason": "mfa_disabled"}, event.OccurredAt)
			}
		}
		for _, states := range m.closeAssignmentStates {
			for enrollmentID, assignmentState := range states {
				enrollment := m.Enrollments[enrollmentID]
				if enrollment.UserID == actor.UserID && enrollment.Role == "coordinator" && assignmentState == "active" {
					states[enrollmentID] = "pending_mfa"
				}
			}
		}
		filtered := actor.Enrollments[:0]
		for _, enrollment := range actor.Enrollments {
			if enrollment.Role != authz.Coordinator {
				filtered = append(filtered, enrollment)
			}
		}
		actor.Enrollments = filtered
		m.AuthSubjects[event.AuthUserID] = actor
	case "email_changed":
		if !exists || strings.TrimSpace(event.Email) == "" {
			return ErrNotFound
		}
		user := m.Users[actor.UserID]
		user.Email = strings.TrimSpace(event.Email)
		user.Revision++
		m.Users[user.ID] = user
		m.appendAuditLocked(actor.UserID, "account.email_changed", "user", user.ID, nil, event.OccurredAt)
	case "account_state_changed":
		if !exists || (event.AccountState != "active" && event.AccountState != "suspended") {
			return ErrNotFound
		}
		actor.AccountState = authz.AccountState(event.AccountState)
		m.AuthSubjects[event.AuthUserID] = actor
		user := m.Users[actor.UserID]
		user.AccountState = event.AccountState
		user.Revision++
		m.Users[user.ID] = user
		auditActor := actor.UserID
		if event.ActorUserID != "" {
			auditActor = event.ActorUserID
		}
		m.appendAuditLocked(auditActor, "account.state_changed", "user", user.ID, map[string]any{"accountState": event.AccountState, "reason": event.Reason}, event.OccurredAt)
	default:
		return errors.New("unsupported identity event")
	}
	m.IdentityEventReceipts[event.EventID] = true
	m.IdentityEventHashes[event.EventID] = payloadHash
	return nil
}

// ResolveAuthSubject performs the operation.
func (m *Memory) ResolveAuthSubject(_ context.Context, sub string) (authz.Actor, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	v, ok := m.AuthSubjects[sub]
	if !ok {
		return authz.Actor{}, ErrNotFound
	}
	if v.SecurityVersion == 0 {
		v.SecurityVersion = 1
	}
	return v, nil
}

// ResolveAuthSubjectForUser performs the operation.
func (m *Memory) ResolveAuthSubjectForUser(_ context.Context, userID string) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for subject, actor := range m.AuthSubjects {
		if actor.UserID == userID {
			return subject, nil
		}
	}
	return "", ErrNotFound
}

// GrantGlobalRole performs the operation.
func (m *Memory) GrantGlobalRole(_ context.Context, userID, role string, activate bool, reason, actorID string, at time.Time) (GlobalRoleAssignment, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.Users[userID]; !ok {
		return GlobalRoleAssignment{}, ErrNotFound
	}
	for _, assignment := range m.GlobalRoleAssignments {
		if assignment.UserID == userID && assignment.Role == role && assignment.State != "revoked" {
			return GlobalRoleAssignment{}, ErrDuplicate
		}
	}
	state := "pending_mfa"
	if activate {
		state = "active"
	}
	v := GlobalRoleAssignment{ID: id.New(), UserID: userID, Role: role, State: state, Revision: 1}
	m.GlobalRoleAssignments[v.ID] = v
	if activate {
		for subject, targetActor := range m.AuthSubjects {
			if targetActor.UserID == userID {
				if targetActor.GlobalRoles == nil {
					targetActor.GlobalRoles = map[authz.GlobalRole]bool{}
				}
				targetActor.GlobalRoles[authz.GlobalRole(role)] = true
				m.AuthSubjects[subject] = targetActor
			}
		}
	}
	m.appendAuditLocked(actorID, "global_role.granted_"+state, "global_role_assignment", v.ID, map[string]any{"userId": userID, "role": role, "reason": reason}, at)
	return v, nil
}

// ListGlobalRoles lists matching values.
func (m *Memory) ListGlobalRoles(_ context.Context, userID string) ([]GlobalRoleAssignment, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if _, ok := m.Users[userID]; !ok {
		return nil, ErrNotFound
	}
	items := []GlobalRoleAssignment{}
	for _, assignment := range m.GlobalRoleAssignments {
		if assignment.UserID == userID && assignment.State != "revoked" {
			items = append(items, assignment)
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Role < items[j].Role })
	return items, nil
}

// RevokeGlobalRole performs the operation.
func (m *Memory) RevokeGlobalRole(_ context.Context, userID, role string, revision int64, reason, actorID string, at time.Time) (GlobalRoleAssignment, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for assignmentID, assignment := range m.GlobalRoleAssignments {
		if assignment.UserID != userID || assignment.Role != role || assignment.State == "revoked" {
			continue
		}
		if assignment.Revision != revision {
			return GlobalRoleAssignment{}, ErrConflict
		}
		assignment.State, assignment.Revision = "revoked", assignment.Revision+1
		m.GlobalRoleAssignments[assignmentID] = assignment
		for subject, targetActor := range m.AuthSubjects {
			if targetActor.UserID == userID {
				delete(targetActor.GlobalRoles, authz.GlobalRole(role))
				m.AuthSubjects[subject] = targetActor
			}
		}
		m.appendAuditLocked(actorID, "global_role.revoked", "global_role_assignment", assignmentID, map[string]any{"userId": userID, "role": role, "reason": reason}, at)
		return assignment, nil
	}
	return GlobalRoleAssignment{}, ErrNotFound
}

// GetUser retrieves a value.
func (m *Memory) GetUser(_ context.Context, id string) (model.User, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	v, ok := m.Users[id]
	if !ok {
		for _, candidate := range m.Users {
			if strings.EqualFold(candidate.Slug, id) {
				v, ok = candidate, true
				break
			}
		}
	}
	if !ok {
		return model.User{}, ErrNotFound
	}
	return m.enrichUserLocked(v), nil
}

// SuggestUserSlug performs the operation.
func (m *Memory) SuggestUserSlug(_ context.Context) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for attempt := 0; attempt < 32; attempt++ {
		candidate := "member-" + strings.ReplaceAll(id.New(), "-", "")[:12]
		available := true
		for _, user := range m.Users {
			available = available && !strings.EqualFold(user.Slug, candidate)
		}
		if available {
			return candidate, nil
		}
	}
	return "", ErrConflict
}

// ListUsers lists matching values.
func (m *Memory) ListUsers(_ context.Context, boundary string, limit int, direction, query, seasonRole, globalRole string) ([]model.User, bool, int64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	items := make([]model.User, 0, len(m.Users))
	var total int64
	for _, v := range m.Users {
		eligible := false
		matchesSeasonRole := seasonRole == ""
		for _, enrollment := range m.Enrollments {
			if enrollment.UserID == v.ID && (enrollment.State == "active" || (enrollment.Role == "student" && enrollment.State == "completed")) {
				eligible = true
				matchesSeasonRole = matchesSeasonRole || enrollment.Role == seasonRole
			}
		}
		matchesGlobalRole := globalRole == ""
		for _, assignment := range m.GlobalRoleAssignments {
			matchesGlobalRole = matchesGlobalRole || assignment.UserID == v.ID && assignment.Role == globalRole && assignment.State == "active"
		}
		for _, role := range v.GlobalRoles {
			matchesGlobalRole = matchesGlobalRole || role == globalRole
		}
		matchesQuery := query == "" || strings.Contains(strings.ToLower(v.Name+" "+v.Slug), strings.ToLower(query))
		if eligible && matchesSeasonRole && matchesGlobalRole && matchesQuery && v.AccountState != "suspended" && v.AccountState != "deleted" && !v.IsTest {
			total++
			if boundary == "" || direction != "backward" && v.ID > boundary || direction == "backward" && v.ID < boundary {
				items = append(items, m.enrichUserLocked(v))
			}
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	if direction == "backward" {
		sort.Slice(items, func(i, j int) bool { return items[i].ID > items[j].ID })
	}
	more := len(items) > limit
	if more {
		items = items[:limit]
	}
	if direction == "backward" {
		sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	}
	return items, more, total, nil
}

// ListAdminUsers lists matching values.
func (m *Memory) ListAdminUsers(_ context.Context, boundary string, limit int, direction, query, accountState, globalRole string) ([]model.User, bool, int64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	query = strings.ToLower(strings.TrimSpace(query))
	items := make([]model.User, 0, len(m.Users))
	var total int64
	for _, user := range m.Users {
		if user.AccountState == "deleted" || accountState != "" && user.AccountState != accountState {
			continue
		}
		if query != "" && !strings.Contains(strings.ToLower(user.Name+" "+user.Slug+" "+user.Email), query) {
			continue
		}
		if globalRole != "" {
			matched := false
			for _, assignment := range m.GlobalRoleAssignments {
				matched = matched || assignment.UserID == user.ID && assignment.Role == globalRole && assignment.State != "revoked"
			}
			if !matched {
				continue
			}
		}
		total++
		if boundary == "" || direction != "backward" && user.ID > boundary || direction == "backward" && user.ID < boundary {
			items = append(items, m.enrichUserLocked(user))
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	if direction == "backward" {
		sort.Slice(items, func(i, j int) bool { return items[i].ID > items[j].ID })
	}
	more := len(items) > limit
	if more {
		items = items[:limit]
	}
	if direction == "backward" {
		sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	}
	return items, more, total, nil
}

// ListEnrollmentCandidates lists matching values.
func (m *Memory) ListEnrollmentCandidates(_ context.Context, seasonID, query, boundary string, limit int, direction string) ([]model.EnrollmentCandidate, bool, int64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	query = strings.ToLower(strings.TrimSpace(query))
	items := []model.EnrollmentCandidate{}
	var total int64
	for _, user := range m.Users {
		if user.AccountState != "active" || user.IsTest {
			continue
		}
		affiliated := false
		for _, enrollment := range m.Enrollments {
			if enrollment.UserID == user.ID && enrollment.SeasonID == seasonID {
				affiliated = true
				break
			}
		}
		if affiliated || (query != "" && !strings.Contains(strings.ToLower(user.Name+" "+user.Slug), query)) {
			continue
		}
		total++
		if boundary == "" || direction != "backward" && user.ID > boundary || direction == "backward" && user.ID < boundary {
			items = append(items, model.EnrollmentCandidate{ID: user.ID, Slug: user.Slug, Name: user.Name, AvatarURL: user.AvatarURL, Revision: user.Revision})
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	if direction == "backward" {
		sort.Slice(items, func(i, j int) bool { return items[i].ID > items[j].ID })
	}
	more := len(items) > limit
	if more {
		items = items[:limit]
	}
	if direction == "backward" {
		sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	}
	return items, more, total, nil
}

func (m *Memory) enrichUserLocked(v model.User) model.User {
	if v.GlobalRoles == nil {
		v.GlobalRoles = []string{}
	}
	v.SeasonRoles = []model.UserSeasonRole{}
	v.AttemptCount, v.MockInterviewCount = 0, 0
	for _, enrollment := range m.Enrollments {
		if enrollment.UserID != v.ID || (enrollment.State != "active" && !(enrollment.Role == "student" && enrollment.State == "completed")) {
			continue
		}
		season := m.Seasons[enrollment.SeasonID]
		v.SeasonRoles = append(v.SeasonRoles, model.UserSeasonRole{SeasonID: enrollment.SeasonID, SeasonSlug: season.Slug, Role: enrollment.Role, State: enrollment.State})
	}
	sort.Slice(v.SeasonRoles, func(i, j int) bool { return v.SeasonRoles[i].SeasonSlug < v.SeasonRoles[j].SeasonSlug })
	for _, attempt := range m.Attempts {
		if attempt.UserID == v.ID && attempt.DeletedAt == nil {
			v.AttemptCount++
		}
	}
	for _, interview := range m.Mocks {
		if interview.DeletedAt == nil && (interview.InterviewerID == v.ID || interview.IntervieweeID == v.ID) {
			v.MockInterviewCount++
		}
	}
	return v
}

// UpdateUser updates a value.
func (m *Memory) UpdateUser(_ context.Context, id string, revision int64, fn func(*model.User) error, actorID string, at time.Time) (model.User, error) {
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

	for otherID, other := range m.Users {
		if otherID != id && strings.EqualFold(other.Slug, v.Slug) {
			return model.User{}, ErrDuplicate
		}
	}
	v.Revision++
	m.Users[id] = v
	m.appendAuditLocked(actorID, "user.updated", "user", id, nil, at)
	return m.enrichUserLocked(v), nil
}

func defaultPracticeSettings(premium bool) model.PracticeSettings {
	return model.PracticeSettings{PremiumOptIn: premium, EasyMinutes: 20, MediumMinutes: 35, HardMinutes: 50, Revision: 1}
}

// GetPracticeSettings retrieves a value.
func (m *Memory) GetPracticeSettings(_ context.Context, userID string) (model.PracticeSettings, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	user, ok := m.Users[userID]
	if !ok {
		return model.PracticeSettings{}, ErrNotFound
	}
	settings, ok := m.PracticeSettings[userID]
	if !ok {
		settings = defaultPracticeSettings(user.PremiumOptIn)
	}
	settings.PremiumOptIn = user.PremiumOptIn
	return settings, nil
}

// UpdatePracticeSettings updates a value.
func (m *Memory) UpdatePracticeSettings(_ context.Context, userID string, revision int64, premium bool, easy, medium, hard int, actorID string, at time.Time) (model.PracticeSettings, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	user, ok := m.Users[userID]
	if !ok {
		return model.PracticeSettings{}, ErrNotFound
	}
	settings, ok := m.PracticeSettings[userID]
	if !ok {
		settings = defaultPracticeSettings(user.PremiumOptIn)
	}
	if settings.Revision != revision {
		return model.PracticeSettings{}, ErrConflict
	}
	if settings.GoalsEnabled && (easy < 1 || medium < 1 || hard < 1) {
		return model.PracticeSettings{}, errors.New("practice goals must be positive")
	}
	settings.PremiumOptIn, settings.EasyMinutes, settings.MediumMinutes, settings.HardMinutes = premium, easy, medium, hard
	settings.Revision++
	user.PremiumOptIn = premium
	user.Revision++
	m.Users[userID], m.PracticeSettings[userID] = user, settings
	actor := actorID
	m.Audits = append(m.Audits, model.AuditEvent{ID: id.New(), ActorID: &actor, Action: "practice_settings.updated", SubjectType: "user", SubjectID: userID, Data: map[string]any{"premiumOptIn": premium, "easyMinutes": easy, "mediumMinutes": medium, "hardMinutes": hard}, OccurredAt: at.UTC()})
	return settings, nil
}

// EnablePracticeGoals performs the operation.
func (m *Memory) EnablePracticeGoals(_ context.Context, userID string, revision int64, actorID, seasonID string, at time.Time) (model.PracticeSettings, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	user, ok := m.Users[userID]
	if !ok {
		return model.PracticeSettings{}, ErrNotFound
	}
	settings, ok := m.PracticeSettings[userID]
	if !ok {
		settings = defaultPracticeSettings(user.PremiumOptIn)
	}
	if settings.Revision != revision {
		return model.PracticeSettings{}, ErrConflict
	}
	if !settings.GoalsEnabled {
		settings.GoalsEnabled = true
		settings.Revision++
		m.PracticeSettings[userID] = settings
		actor := actorID
		m.Audits = append(m.Audits, model.AuditEvent{ID: id.New(), ActorID: &actor, Action: "practice_goals.enabled", SubjectType: "user", SubjectID: userID, Data: map[string]any{"seasonId": seasonID}, OccurredAt: at.UTC()})
	}
	return settings, nil
}

// GetSeason retrieves a value.
func (m *Memory) GetSeason(_ context.Context, id string) (model.Season, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	v, ok := m.Seasons[id]
	if !ok {
		return model.Season{}, ErrNotFound
	}
	return v, nil
}

// ListSeasons lists matching values.
func (m *Memory) ListSeasons(_ context.Context, boundary string, limit int, direction, status string) ([]model.Season, bool, int64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	items := make([]model.Season, 0, len(m.Seasons))
	var total int64
	for _, v := range m.Seasons {
		if status != "" && v.Status != status {
			continue
		}
		total++
		if boundary == "" || direction != "backward" && v.ID > boundary || direction == "backward" && v.ID < boundary {
			items = append(items, v)
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	if direction == "backward" {
		sort.Slice(items, func(i, j int) bool { return items[i].ID > items[j].ID })
	}
	more := len(items) > limit
	if more {
		items = items[:limit]
	}
	if direction == "backward" {
		sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	}
	return items, more, total, nil
}

// CreateSeason creates a value.
func (m *Memory) CreateSeason(_ context.Context, v model.Season, actorID string, at time.Time) (model.Season, error) {
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
	m.appendAuditLocked(actorID, "season.created", "season", v.ID, nil, at)
	return v, nil
}

// UpdateSeason updates a value.
func (m *Memory) UpdateSeason(_ context.Context, id string, revision int64, fn func(*model.Season) error, actorID string, at time.Time) (model.Season, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	v, ok := m.Seasons[id]
	if !ok {
		return model.Season{}, ErrNotFound
	}
	if v.Revision != revision || v.Status != "open" {
		return model.Season{}, ErrConflict
	}
	if err := fn(&v); err != nil {
		return model.Season{}, err
	}

	for _, week := range m.Weeks {
		if week.SeasonID == id && (week.StartAt.Before(v.StartAt) || week.EndAt.After(v.EndAt)) {
			return model.Season{}, ErrConflict
		}
	}
	for _, interview := range m.Mocks {
		if interview.SeasonID != nil && *interview.SeasonID == id && (interview.OccurredAt.Before(v.StartAt) || interview.OccurredAt.After(v.EndAt)) {
			return model.Season{}, ErrConflict
		}
	}
	v.Revision++
	m.Seasons[id] = v
	m.appendAuditLocked(actorID, "season.updated", "season", id, nil, at)
	return v, nil
}

// CloseSeason closes a value.
func (m *Memory) CloseSeason(_ context.Context, seasonID string, revision int64, actorID, reason string, at time.Time) (model.Season, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	v, ok := m.Seasons[seasonID]
	if !ok {
		return model.Season{}, ErrNotFound
	}
	if v.Revision != revision {
		return model.Season{}, ErrConflict
	}
	domainSeason := programme.Season{ID: v.ID, Status: v.Status, Revision: v.Revision}
	for _, enrollment := range m.Enrollments {
		if enrollment.SeasonID == seasonID {
			domainSeason.Enrollments = append(domainSeason.Enrollments, programme.Enrollment{ID: enrollment.ID, State: enrollment.State})
		}
	}
	transition, err := programme.Close(domainSeason, actorID, reason, at)
	if err != nil {
		return model.Season{}, ErrConflict
	}
	v.Status = transition.Season.Status
	v.Revision = transition.Season.Revision
	m.Seasons[seasonID] = v
	completed := map[string]bool{}
	assignmentStates := map[string]string{}
	for _, changed := range transition.Season.Enrollments {
		enrollment, exists := m.Enrollments[changed.ID]
		if exists && changed.State == "completed" && enrollment.State == "active" {
			assignmentStates[changed.ID] = enrollment.AssignmentState
			enrollment.State = "completed"
			enrollment.AssignmentState = "revoked"
			enrollment.Revision++
			m.Enrollments[changed.ID] = enrollment
			completed[changed.ID] = true
		}
	}
	m.closeCompleted[seasonID] = completed
	m.closeAssignmentStates[seasonID] = assignmentStates
	actor := actorID
	m.Audits = append(m.Audits, model.AuditEvent{ID: id.New(), ActorID: &actor, Action: "season.closed", SubjectType: "season", SubjectID: seasonID, Data: map[string]any{"reason": reason}, OccurredAt: at.UTC()})
	return v, nil
}

// ReopenSeason reopens a value.
func (m *Memory) ReopenSeason(_ context.Context, seasonID string, revision int64, actorID, reason string, at time.Time) (model.Season, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	v, ok := m.Seasons[seasonID]
	if !ok {
		return model.Season{}, ErrNotFound
	}
	if v.Revision != revision {
		return model.Season{}, ErrConflict
	}
	domainSeason := programme.Season{ID: v.ID, Status: v.Status, Revision: v.Revision}
	closeID := "close-" + seasonID
	for _, enrollment := range m.Enrollments {
		if enrollment.SeasonID == seasonID {
			completedBy := ""
			if m.closeCompleted[seasonID][enrollment.ID] {
				completedBy = closeID
			}
			domainSeason.Enrollments = append(domainSeason.Enrollments, programme.Enrollment{ID: enrollment.ID, State: enrollment.State, CompletedByCloseID: stringPointer(completedBy)})
		}
	}
	reopened, err := programme.Reopen(domainSeason, programme.CloseEvent{ID: closeID}, programme.SystemAdmin)
	if err != nil {
		return model.Season{}, ErrConflict
	}
	v.Status = reopened.Status
	v.Revision = reopened.Revision
	m.Seasons[seasonID] = v
	for enrollmentID := range m.closeCompleted[seasonID] {
		enrollment, exists := m.Enrollments[enrollmentID]
		if exists && enrollment.State == "completed" {
			enrollment.State = "active"
			enrollment.AssignmentState = m.closeAssignmentStates[seasonID][enrollmentID]
			if enrollment.AssignmentState == "" {
				enrollment.AssignmentState = "active"
			}
			enrollment.Revision++
			m.Enrollments[enrollmentID] = enrollment
		}
	}
	delete(m.closeCompleted, seasonID)
	delete(m.closeAssignmentStates, seasonID)
	actor := actorID
	m.Audits = append(m.Audits, model.AuditEvent{ID: id.New(), ActorID: &actor, Action: "season.reopened", SubjectType: "season", SubjectID: seasonID, Data: map[string]any{"reason": reason}, OccurredAt: at.UTC()})
	return v, nil
}

func stringPointer(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

// ListWeeks lists matching values.
func (m *Memory) ListWeeks(_ context.Context, seasonID, boundary string, limit int, sortBy, direction string) ([]model.Week, bool, int64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	items := []model.Week{}
	for _, v := range m.Weeks {
		if v.SeasonID == seasonID {
			items = append(items, v)
		}
	}
	if sortBy == "id:asc" {
		sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	} else {
		sort.Slice(items, func(i, j int) bool {
			return items[i].Number < items[j].Number || items[i].Number == items[j].Number && items[i].ID < items[j].ID
		})
	}
	total := int64(len(items))
	page, more, err := memoryPage(items, boundary, limit, direction, func(v model.Week) string { return v.ID })
	return page, more, total, err
}

// CreateWeek creates a value.
func (m *Memory) CreateWeek(_ context.Context, v model.Week, actorID string, at time.Time) (model.Week, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	season, exists := m.Seasons[v.SeasonID]
	if !exists {
		return model.Week{}, ErrNotFound
	}
	if season.Status != "open" || !v.EndAt.After(v.StartAt) || v.StartAt.Before(season.StartAt) || v.EndAt.After(season.EndAt) {
		return model.Week{}, ErrConflict
	}
	if _, exists := m.Weeks[v.ID]; exists {
		return model.Week{}, ErrDuplicate
	}
	for _, old := range m.Weeks {
		if old.SeasonID == v.SeasonID && old.Number == v.Number {
			return model.Week{}, ErrDuplicate
		}
	}
	m.Weeks[v.ID] = v
	m.appendAuditLocked(actorID, "week.created", "week", v.ID, nil, at)
	return v, nil
}

// UpdateWeek updates a value.
func (m *Memory) UpdateWeek(_ context.Context, seasonID, weekID string, revision int64, candidate model.Week, actorID string, at time.Time) (model.Week, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	current, ok := m.Weeks[weekID]
	if !ok || current.SeasonID != seasonID {
		return model.Week{}, ErrNotFound
	}
	if current.Revision != revision {
		return model.Week{}, ErrConflict
	}
	season, ok := m.Seasons[seasonID]
	if !ok {
		return model.Week{}, ErrNotFound
	}
	if season.Status != "open" || !candidate.EndAt.After(candidate.StartAt) || candidate.StartAt.Before(season.StartAt) || candidate.EndAt.After(season.EndAt) {
		return model.Week{}, ErrConflict
	}
	for id, week := range m.Weeks {
		if id != weekID && week.SeasonID == seasonID && week.Number == candidate.Number {
			return model.Week{}, ErrDuplicate
		}
	}
	candidate.ID, candidate.SeasonID, candidate.Revision = current.ID, current.SeasonID, current.Revision+1
	m.Weeks[weekID] = candidate
	m.appendAuditLocked(actorID, "week.updated", "week", weekID, nil, at)
	return candidate, nil
}

// DeleteWeek deletes a value.
func (m *Memory) DeleteWeek(_ context.Context, seasonID, weekID string, revision int64, actorID string, at time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	current, ok := m.Weeks[weekID]
	if !ok || current.SeasonID != seasonID {
		return ErrNotFound
	}
	if current.Revision != revision {
		return ErrConflict
	}
	delete(m.Weeks, weekID)
	m.appendAuditLocked(actorID, "week.deleted", "week", weekID, nil, at)
	return nil
}

// ListEnrollments lists matching values.
func (m *Memory) ListEnrollments(_ context.Context, seasonID, boundary string, limit int, role, state, sortBy, direction string, includeInactive bool) ([]model.Enrollment, bool, int64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	items := m.listEnrollmentsLocked(func(v model.Enrollment) bool {
		if v.SeasonID != seasonID || role != "" && v.Role != role || state != "" && v.State != state {
			return false
		}
		return includeInactive || v.State == "active" || v.Role == "student" && v.State == "completed"
	})
	if sortBy == "role:asc" {
		sort.Slice(items, func(i, j int) bool {
			return items[i].Role < items[j].Role || items[i].Role == items[j].Role && items[i].ID < items[j].ID
		})
	}
	total := int64(len(items))
	page, more, err := memoryPage(items, boundary, limit, direction, func(v model.Enrollment) string { return v.ID })
	return page, more, total, err
}

// ListEnrollmentsForUser lists matching values.
func (m *Memory) ListEnrollmentsForUser(_ context.Context, userID string) ([]model.Enrollment, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.listEnrollmentsLocked(func(v model.Enrollment) bool { return v.UserID == userID }), nil
}

func (m *Memory) listEnrollmentsLocked(include func(model.Enrollment) bool) []model.Enrollment {
	items := []model.Enrollment{}
	for _, v := range m.Enrollments {
		v = normalizeEnrollment(v)
		if include(v) {
			if season, ok := m.Seasons[v.SeasonID]; ok {
				v.SeasonSlug = season.Slug
			}
			items = append(items, v)
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	return items
}

// GetEnrollment retrieves a value.
func (m *Memory) GetEnrollment(_ context.Context, enrollmentID string) (model.Enrollment, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	v, ok := m.Enrollments[enrollmentID]
	if !ok {
		return model.Enrollment{}, ErrNotFound
	}
	return normalizeEnrollment(v), nil
}

func normalizeEnrollment(v model.Enrollment) model.Enrollment {
	if v.Role == "student" {
		if v.StudentLevel == "" {
			v.StudentLevel = "novice"
		}
	} else {
		v.StudentLevel = "not_applicable"
	}
	if v.AssignmentState == "" {
		if v.State == "active" {
			v.AssignmentState = "active"
		} else {
			v.AssignmentState = "revoked"
		}
	}
	return v
}

// CreateEnrollment creates a value.
func (m *Memory) CreateEnrollment(_ context.Context, v model.Enrollment, actorID string, at time.Time) (model.Enrollment, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.Users[v.UserID]; !exists {
		return model.Enrollment{}, ErrNotFound
	}
	if _, exists := m.Seasons[v.SeasonID]; !exists {
		return model.Enrollment{}, ErrNotFound
	}
	for _, old := range m.Enrollments {
		if old.SeasonID == v.SeasonID && old.UserID == v.UserID {
			return model.Enrollment{}, ErrDuplicate
		}
	}
	if v.AssignmentState == "" {
		v.AssignmentState = "active"
	}
	if v.Role == "coordinator" {
		v.AssignmentState = "pending_mfa"
		if m.MFAConfigured[v.UserID] {
			v.AssignmentState = "active"
		}
	}
	v = normalizeEnrollment(v)
	m.Enrollments[v.ID] = v
	m.appendAuditLocked(actorID, "enrollment.created", "enrollment", v.ID, nil, at)
	return v, nil
}

// UpdateEnrollmentDetails updates a value.
func (m *Memory) UpdateEnrollmentDetails(_ context.Context, seasonID, enrollmentID string, revision int64, role, studentLevel, actorID string, at time.Time) (model.Enrollment, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	v, ok := m.Enrollments[enrollmentID]
	if !ok || v.SeasonID != seasonID {
		return model.Enrollment{}, ErrNotFound
	}
	if v.Revision != revision || v.State != "active" {
		return model.Enrollment{}, ErrConflict
	}
	v.Role, v.StudentLevel, v.Revision = role, studentLevel, v.Revision+1
	v.AssignmentState = "active"
	if role == "coordinator" {
		v.AssignmentState = "pending_mfa"
		if m.MFAConfigured[v.UserID] {
			v.AssignmentState = "active"
		}
	}
	if role != "student" {
		for mentorshipID, mentorship := range m.Mentorships {
			if mentorship.SeasonID == seasonID && mentorship.StudentUserID == v.UserID {
				delete(m.Mentorships, mentorshipID)
			}
		}
	}
	if role == "student" {
		for mentorshipID, mentorship := range m.Mentorships {
			if mentorship.SeasonID == seasonID && mentorship.MentorUserID == v.UserID {
				delete(m.Mentorships, mentorshipID)
			}
		}
	}
	v = normalizeEnrollment(v)
	m.Enrollments[enrollmentID] = v
	m.appendAuditLocked(actorID, "enrollment.updated", "enrollment", enrollmentID, map[string]any{"role": role, "studentLevel": studentLevel}, at)
	return v, nil
}

// UpdateEnrollment updates a value.
func (m *Memory) UpdateEnrollment(_ context.Context, enrollmentID string, revision int64, role, state, reason, actorID string, at time.Time) (model.Enrollment, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	v, ok := m.Enrollments[enrollmentID]
	if !ok {
		return model.Enrollment{}, ErrNotFound
	}
	if v.Revision != revision || v.State != "active" {
		return model.Enrollment{}, ErrConflict
	}
	if role != "" {
		v.Role = role
		v.AssignmentState = "active"
		if role == "coordinator" {
			v.AssignmentState = "pending_mfa"
			if m.MFAConfigured[v.UserID] {
				v.AssignmentState = "active"
			}
		}
		for mentorshipID, mentorship := range m.Mentorships {
			if mentorship.SeasonID == v.SeasonID && mentorship.StudentUserID == v.UserID {
				delete(m.Mentorships, mentorshipID)
			}
		}
		actor := actorID
		m.Audits = append(m.Audits, model.AuditEvent{ID: id.New(), ActorID: &actor, Action: "enrollment.promoted", SubjectType: "enrollment", SubjectID: enrollmentID, Data: map[string]any{"role": role}, OccurredAt: at.UTC()})
	}
	if state != "" {
		v.State = state
		v.AssignmentState = "revoked"
		trimmed := strings.TrimSpace(reason)
		v.RemovalReason = &trimmed
		for mentorshipID, mentorship := range m.Mentorships {
			if mentorship.SeasonID == v.SeasonID && (mentorship.StudentUserID == v.UserID || mentorship.MentorUserID == v.UserID) {
				delete(m.Mentorships, mentorshipID)
			}
		}
		actor := actorID
		m.Audits = append(m.Audits, model.AuditEvent{ID: id.New(), ActorID: &actor, Action: "enrollment.removed", SubjectType: "enrollment", SubjectID: enrollmentID, Data: map[string]any{"reason": trimmed, "state": state}, OccurredAt: at.UTC()})
	}
	v = normalizeEnrollment(v)
	v.Revision++
	m.Enrollments[enrollmentID] = v
	return v, nil
}

// ListMentorships lists matching values.
func (m *Memory) ListMentorships(_ context.Context, seasonID, boundary string, limit int, sortBy, direction, mentorUserID, studentUserID string) ([]model.Mentorship, bool, int64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	items := []model.Mentorship{}
	for _, v := range m.Mentorships {
		if v.SeasonID == seasonID && (mentorUserID == "" || v.MentorUserID == mentorUserID) && (studentUserID == "" || v.StudentUserID == studentUserID) {
			items = append(items, v)
		}
	}
	if sortBy == "student:asc" {
		sort.Slice(items, func(i, j int) bool {
			return items[i].StudentUserID < items[j].StudentUserID || items[i].StudentUserID == items[j].StudentUserID && items[i].ID < items[j].ID
		})
	} else {
		sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	}
	total := int64(len(items))
	page, more, err := memoryPage(items, boundary, limit, direction, func(v model.Mentorship) string { return v.ID })
	return page, more, total, err
}

// CreateMentorship creates a value.
func (m *Memory) CreateMentorship(_ context.Context, v model.Mentorship, actorID string, at time.Time) (model.Mentorship, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var mentorOK, studentOK bool
	for _, enrollment := range m.Enrollments {
		if enrollment.SeasonID != v.SeasonID || enrollment.State != "active" {
			continue
		}
		mentorOK = mentorOK || (enrollment.UserID == v.MentorUserID && (enrollment.Role == "mentor" || enrollment.Role == "coordinator"))
		studentOK = studentOK || (enrollment.UserID == v.StudentUserID && enrollment.Role == "student")
	}
	if !mentorOK || !studentOK {
		return model.Mentorship{}, ErrConflict
	}
	for _, old := range m.Mentorships {
		if old.SeasonID == v.SeasonID && old.StudentUserID == v.StudentUserID {
			return model.Mentorship{}, ErrDuplicate
		}
	}
	m.Mentorships[v.ID] = v
	m.appendAuditLocked(actorID, "mentorship.created", "mentorship", v.ID, nil, at)
	return v, nil
}

// UpdateMentorship updates a value.
func (m *Memory) UpdateMentorship(_ context.Context, seasonID, mentorshipID string, revision int64, mentorUserID, studentUserID, actorID string, at time.Time) (model.Mentorship, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	v, ok := m.Mentorships[mentorshipID]
	if !ok || v.SeasonID != seasonID {
		return model.Mentorship{}, ErrNotFound
	}
	if v.Revision != revision {
		return model.Mentorship{}, ErrConflict
	}
	var mentorOK, studentOK bool
	for _, enrollment := range m.Enrollments {
		if enrollment.SeasonID != seasonID || enrollment.State != "active" {
			continue
		}
		mentorOK = mentorOK || (enrollment.UserID == mentorUserID && (enrollment.Role == "mentor" || enrollment.Role == "coordinator"))
		studentOK = studentOK || (enrollment.UserID == studentUserID && enrollment.Role == "student")
	}
	if !mentorOK || !studentOK || mentorUserID == studentUserID {
		return model.Mentorship{}, ErrConflict
	}
	for id, other := range m.Mentorships {
		if id != mentorshipID && other.SeasonID == seasonID && other.StudentUserID == studentUserID {
			return model.Mentorship{}, ErrDuplicate
		}
	}
	v.MentorUserID, v.StudentUserID, v.Revision = mentorUserID, studentUserID, v.Revision+1
	m.Mentorships[mentorshipID] = v
	m.appendAuditLocked(actorID, "mentorship.updated", "mentorship", mentorshipID, nil, at)
	return v, nil
}

// DeleteMentorship deletes a value.
func (m *Memory) DeleteMentorship(_ context.Context, seasonID, mentorshipID string, revision int64, actorID string, at time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	v, ok := m.Mentorships[mentorshipID]
	if !ok || v.SeasonID != seasonID {
		return ErrNotFound
	}
	if v.Revision != revision {
		return ErrConflict
	}
	delete(m.Mentorships, mentorshipID)
	m.appendAuditLocked(actorID, "mentorship.deleted", "mentorship", mentorshipID, nil, at)
	return nil
}

// IsMentorAssigned performs the operation.
func (m *Memory) IsMentorAssigned(_ context.Context, seasonID, mentorUserID, studentUserID string) (bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, v := range m.Mentorships {
		if v.SeasonID == seasonID && v.MentorUserID == mentorUserID && v.StudentUserID == studentUserID {
			return true, nil
		}
	}
	return false, nil
}

// ListProblems lists matching values.
func (m *Memory) ListProblems(_ context.Context, boundary string, limit int, difficulty, category string, premium *bool, direction string) ([]model.Problem, bool, int64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	items := make([]model.Problem, 0, len(m.Problems))
	var total int64
	for _, v := range m.Problems {
		if difficulty != "" && v.Difficulty != difficulty {
			continue
		}
		if category != "" {
			matched := false
			for _, problemCategory := range v.Categories {
				matched = matched || strings.EqualFold(problemCategory, category)
			}
			if !matched {
				continue
			}
		}
		if premium != nil && v.Premium != *premium {
			continue
		}
		total++
		if boundary == "" || direction != "backward" && v.ID > boundary || direction == "backward" && v.ID < boundary {
			items = append(items, v)
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	if direction == "backward" {
		sort.Slice(items, func(i, j int) bool { return items[i].ID > items[j].ID })
	}
	more := len(items) > limit
	if more {
		items = items[:limit]
	}
	if direction == "backward" {
		sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	}
	return items, more, total, nil
}

// GetAttempt retrieves a value.
func (m *Memory) GetAttempt(_ context.Context, id string) (model.Attempt, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	v, ok := m.Attempts[id]
	if !ok || v.DeletedAt != nil {
		return model.Attempt{}, ErrNotFound
	}
	return v, nil
}

// ListAttempts lists matching values.
func (m *Memory) ListAttempts(_ context.Context, userID, boundary string, limit int, outcome, difficulty, direction string) ([]model.Attempt, bool, int64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	items := []model.Attempt{}
	var total int64
	for _, v := range m.Attempts {
		if v.UserID == userID && v.DeletedAt == nil {
			if outcome != "" && v.Outcome != outcome {
				continue
			}
			if difficulty != "" && m.Problems[v.ProblemID].Difficulty != difficulty {
				continue
			}
			total++
			if boundary == "" || direction != "backward" && v.ID > boundary || direction == "backward" && v.ID < boundary {
				items = append(items, v)
			}
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	if direction == "backward" {
		sort.Slice(items, func(i, j int) bool { return items[i].ID > items[j].ID })
	}
	more := len(items) > limit
	if more {
		items = items[:limit]
	}
	if direction == "backward" {
		sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	}
	return items, more, total, nil
}

// RecommendationSnapshot performs the operation.
func (m *Memory) RecommendationSnapshot(_ context.Context, userID string, _ practice.Goals) (RecommendationSnapshot, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	snapshot := RecommendationSnapshot{ProblemHistory: map[string]practice.ProblemHistory{}, CategoryExposure: map[string]int{}}
	for _, attempt := range m.Attempts {
		if attempt.UserID != userID || attempt.DeletedAt != nil {
			continue
		}
		problem, exists := m.Problems[attempt.ProblemID]
		if !exists {
			continue
		}
		for _, category := range problem.Categories {
			snapshot.CategoryExposure[category]++
		}
		if !attempt.Migrated && attempt.Outcome != string(practice.Unknown) {
			snapshot.QualityAttempts = append(snapshot.QualityAttempts, attempt)
		}
	}
	sort.Slice(snapshot.QualityAttempts, func(i, j int) bool {
		return snapshot.QualityAttempts[i].AttemptedAt.After(snapshot.QualityAttempts[j].AttemptedAt) || snapshot.QualityAttempts[i].AttemptedAt.Equal(snapshot.QualityAttempts[j].AttemptedAt) && snapshot.QualityAttempts[i].ID < snapshot.QualityAttempts[j].ID
	})
	if len(snapshot.QualityAttempts) > 20 {
		snapshot.QualityAttempts = snapshot.QualityAttempts[:20]
	}
	qualityProblems := map[string]bool{}
	for _, attempt := range snapshot.QualityAttempts {
		if !qualityProblems[attempt.ProblemID] {
			snapshot.Problems = append(snapshot.Problems, m.Problems[attempt.ProblemID])
			qualityProblems[attempt.ProblemID] = true
		}
	}
	return snapshot, nil
}

// RecommendationCandidates performs the operation.
func (m *Memory) RecommendationCandidates(_ context.Context, userID string, criteria practice.Criteria, premiumOptIn bool, goals practice.Goals, now time.Time) ([]model.Problem, map[string]practice.ProblemHistory, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	dismissed := map[string]bool{}
	for _, dismissal := range m.RecommendationDismissals[userID] {
		if now.UTC().Sub(dismissal.DismissedAt.UTC()) < 30*24*time.Hour {
			dismissed[dismissal.ProblemID] = true
		}
	}
	history := map[string]practice.ProblemHistory{}
	for _, attempt := range m.Attempts {
		if attempt.UserID != userID || attempt.DeletedAt != nil {
			continue
		}
		problem, exists := m.Problems[attempt.ProblemID]
		if !exists {
			continue
		}
		entry := history[attempt.ProblemID]
		if attempt.AttemptedAt.After(entry.LastAttemptedAt) {
			entry.LastAttemptedAt = attempt.AttemptedAt
		}
		if !attempt.Migrated && (attempt.Outcome == string(practice.NotSolved) || attempt.Outcome == string(practice.WithHints) || attempt.Confidence != nil && *attempt.Confidence < 4 || attempt.Minutes > goals[practice.Difficulty(problem.Difficulty)]) {
			entry.Weak = true
		}
		history[attempt.ProblemID] = entry
	}
	better := func(candidate model.Problem, current *model.Problem) bool {
		return current == nil || candidate.Number < current.Number || candidate.Number == current.Number && candidate.ID < current.ID
	}
	var unseen, retry *model.Problem
	for _, value := range m.Problems {
		problem := value
		hasCategory := criteria.Category == ""
		for _, category := range problem.Categories {
			hasCategory = hasCategory || strings.EqualFold(category, criteria.Category)
		}
		if problem.Difficulty != string(criteria.Difficulty) || !premiumOptIn && problem.Premium || dismissed[problem.ID] || !hasCategory {
			continue
		}
		entry, seen := history[problem.ID]
		if !seen && better(problem, unseen) {
			copy := problem
			unseen = &copy
			continue
		}
		if unseen == nil && entry.Weak && !entry.LastAttemptedAt.IsZero() && now.UTC().Sub(entry.LastAttemptedAt.UTC()) >= 90*24*time.Hour && better(problem, retry) {
			copy := problem
			retry = &copy
		}
	}
	if unseen != nil {
		return []model.Problem{*unseen}, map[string]practice.ProblemHistory{}, nil
	}
	if retry != nil {
		return []model.Problem{*retry}, map[string]practice.ProblemHistory{retry.ID: history[retry.ID]}, nil
	}
	return []model.Problem{}, map[string]practice.ProblemHistory{}, nil
}

// CreateAttempt creates a value.
func (m *Memory) CreateAttempt(_ context.Context, v model.Attempt) (model.Attempt, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.Attempts[v.ID]; ok {
		return model.Attempt{}, false, ErrDuplicate
	}
	if _, ok := m.Problems[v.ProblemID]; !ok {
		return model.Attempt{}, false, ErrNotFound
	}
	if v.WeekID != nil && v.SeasonID == nil {
		return model.Attempt{}, false, ErrNotFound
	}
	if v.SeasonID != nil {
		season, seasonExists := m.Seasons[*v.SeasonID]
		activeEnrollment := false
		for _, enrollment := range m.Enrollments {
			activeEnrollment = activeEnrollment || (enrollment.UserID == v.UserID && enrollment.SeasonID == *v.SeasonID && enrollment.State == "active")
		}
		if !seasonExists || season.Status != "open" || !activeEnrollment {
			return model.Attempt{}, false, ErrNotFound
		}
		if v.WeekID != nil {
			week, ok := m.Weeks[*v.WeekID]
			if !ok || week.SeasonID != *v.SeasonID {
				return model.Attempt{}, false, ErrNotFound
			}
		}
	}
	m.Attempts[v.ID] = v
	fulfilled := false
	if recommendation, ok := m.Recommendations[v.UserID]; ok && recommendation.DismissedAt == nil && recommendation.FulfilledAt == nil && recommendation.Problem.ID == v.ProblemID {
		at := time.Now().UTC()
		recommendation.FulfilledAt = &at
		m.Recommendations[v.UserID] = recommendation
		fulfilled = true
	}
	actor := v.UserID
	m.Audits = append(m.Audits, model.AuditEvent{ID: id.New(), ActorID: &actor, Action: "attempt.created", SubjectType: "problem_attempt", SubjectID: v.ID, Data: map[string]any{}, OccurredAt: time.Now().UTC()})
	return v, fulfilled, nil
}

// UpdateAttempt updates a value.
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
	if _, ok := m.Problems[v.ProblemID]; !ok || (v.WeekID != nil && v.SeasonID == nil) {
		return model.Attempt{}, ErrNotFound
	}
	if v.SeasonID != nil {
		season, exists := m.Seasons[*v.SeasonID]
		active := false
		for _, enrollment := range m.Enrollments {
			active = active || (enrollment.UserID == userID && enrollment.SeasonID == *v.SeasonID && enrollment.State == "active")
		}
		if !exists || season.Status != "open" || !active {
			return model.Attempt{}, ErrConflict
		}
		if v.WeekID != nil && m.Weeks[*v.WeekID].SeasonID != *v.SeasonID {
			return model.Attempt{}, ErrConflict
		}
	}
	v.Revision++
	m.Attempts[id] = v
	m.appendAuditLocked(userID, "attempt.updated", "problem_attempt", id, nil, time.Now())
	return v, nil
}

// DeleteAttempt deletes a value.
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
	now := time.Now().UTC()
	v.DeletedAt = &now
	v.Revision++
	m.Attempts[id] = v
	m.appendAuditLocked(userID, "attempt.deleted", "problem_attempt", id, nil, now)
	return nil
}

// GetActiveRecommendation retrieves a value.
func (m *Memory) GetActiveRecommendation(_ context.Context, userID string) (*practice.Recommendation, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	v, ok := m.Recommendations[userID]
	if !ok || v.DismissedAt != nil || v.FulfilledAt != nil {
		return nil, nil
	}
	problem, ok := m.Problems[v.Problem.ID]
	if !ok {
		return nil, ErrNotFound
	}
	v.Problem = practiceProblem(problem)
	return &v, nil
}

// SaveRecommendation saves a value.
func (m *Memory) SaveRecommendation(_ context.Context, v practice.Recommendation) (practice.Recommendation, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if active, ok := m.Recommendations[v.UserID]; ok && active.DismissedAt == nil && active.FulfilledAt == nil && active.ID != v.ID {
		return practice.Recommendation{}, ErrConflict
	}
	if v.Revision < 1 {
		v.Revision = 1
	}
	problem, ok := m.Problems[v.Problem.ID]
	if !ok {
		return practice.Recommendation{}, ErrNotFound
	}
	v.Problem = practiceProblem(problem)
	m.Recommendations[v.UserID] = v
	return v, nil
}

func practiceProblem(problem model.Problem) practice.Problem {
	return practice.Problem{ID: problem.ID, Number: problem.Number, Title: problem.Title, Link: problem.Link, Difficulty: practice.Difficulty(problem.Difficulty), Categories: append([]string(nil), problem.Categories...), Premium: problem.Premium, Revision: problem.Revision}
}

// ListRecommendationDismissals lists matching values.
func (m *Memory) ListRecommendationDismissals(_ context.Context, userID string) ([]practice.Dismissal, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return append([]practice.Dismissal(nil), m.RecommendationDismissals[userID]...), nil
}

// DismissRecommendation performs the operation.
func (m *Memory) DismissRecommendation(_ context.Context, userID string, revision int64, reason string, at time.Time, dismissalID string) (practice.Recommendation, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	v, ok := m.Recommendations[userID]
	if !ok || v.DismissedAt != nil || v.FulfilledAt != nil {
		return practice.Recommendation{}, ErrNotFound
	}
	if v.Revision != revision {
		return practice.Recommendation{}, ErrConflict
	}
	dismissedAt := at.UTC()
	v.DismissedAt = &dismissedAt
	v.Revision++
	m.Recommendations[userID] = v
	m.RecommendationDismissals[userID] = append(m.RecommendationDismissals[userID], practice.Dismissal{ProblemID: v.Problem.ID, DismissedAt: dismissedAt})
	m.appendAuditLocked(userID, "recommendation.dismissed", "recommendation", v.ID, map[string]any{"reason": strings.TrimSpace(reason)}, dismissedAt)
	_ = dismissalID
	return v, nil
}

// FulfillRecommendation performs the operation.
func (m *Memory) FulfillRecommendation(_ context.Context, userID string, attempt model.Attempt, at time.Time) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	v, ok := m.Recommendations[userID]
	if !ok || v.DismissedAt != nil || v.FulfilledAt != nil || v.Problem.ID != attempt.ProblemID {
		return false, nil
	}
	fulfilledAt := at.UTC()
	v.FulfilledAt = &fulfilledAt
	m.Recommendations[userID] = v
	return true, nil
}

// ListMockInterviews lists matching values.
func (m *Memory) ListMockInterviews(_ context.Context, actor authz.Actor, mode, boundary string, limit int, sortBy, direction string) ([]mockinterviews.Interview, bool, int64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	userID := actor.UserID
	items := []mockinterviews.Interview{}
	for _, v := range m.Mocks {
		if v.DeletedAt != nil {
			continue
		}
		if mode == "received" && v.IntervieweeID != userID {
			continue
		}
		if mode == "given" && v.InterviewerID != userID {
			continue
		}
		if mode == "all" && !actor.IsPrivileged() && v.InterviewerID != userID && v.IntervieweeID != userID {
			visible := false
			if v.SeasonID != nil {
				if enrollment, ok := actor.Enrollment(*v.SeasonID); ok && (enrollment.State == authz.Active || enrollment.State == authz.Completed) {
					visible = enrollment.Role == authz.Coordinator
					if enrollment.Role == authz.Mentor {
						for _, mentorship := range m.Mentorships {
							visible = visible || (mentorship.SeasonID == *v.SeasonID && mentorship.MentorUserID == userID && (mentorship.StudentUserID == v.IntervieweeID || mentorship.StudentUserID == v.InterviewerID))
						}
					}
				}
			}
			if !visible {
				continue
			}
		}
		items = append(items, v)
	}
	if sortBy == "id:asc" {
		sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	} else {
		sort.Slice(items, func(i, j int) bool {
			return items[i].OccurredAt.After(items[j].OccurredAt) || items[i].OccurredAt.Equal(items[j].OccurredAt) && items[i].ID > items[j].ID
		})
	}
	total := int64(len(items))
	page, more, err := memoryPage(items, boundary, limit, direction, func(v mockinterviews.Interview) string { return v.ID })
	return page, more, total, err
}

// GetMockParticipant retrieves a value.
func (m *Memory) GetMockParticipant(_ context.Context, userID string) (mockinterviews.Participant, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	user, ok := m.Users[userID]
	if !ok {
		return mockinterviews.Participant{}, ErrNotFound
	}
	p := mockinterviews.Participant{UserID: userID, Suspended: user.AccountState == "suspended", Deleted: user.AccountState == "deleted", Inactive: user.AccountState != "" && user.AccountState != "active", Test: user.IsTest}
	hasAny, kicked := false, false
	for _, enrollment := range m.Enrollments {
		if enrollment.UserID != userID {
			continue
		}
		hasAny = true
		p.ActiveMember = p.ActiveMember || enrollment.State == "active"
		p.Alumni = p.Alumni || (enrollment.Role == "student" && enrollment.State == "completed")
		p.FormerMember = p.FormerMember || enrollment.State == "completed"
		kicked = kicked || enrollment.State == "kicked"
	}
	p.KickedOnly = hasAny && kicked && !p.ActiveMember && !p.FormerMember
	return p, nil
}

// GetMockInterview retrieves a value.
func (m *Memory) GetMockInterview(_ context.Context, interviewID string) (mockinterviews.Interview, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	v, ok := m.Mocks[interviewID]
	if !ok || v.DeletedAt != nil {
		return mockinterviews.Interview{}, ErrNotFound
	}
	return v, nil
}

// CreateMockInterview creates a value.
func (m *Memory) CreateMockInterview(_ context.Context, v mockinterviews.Interview, actorID string, at time.Time) (mockinterviews.Interview, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.Mocks[v.ID]; ok {
		return mockinterviews.Interview{}, ErrDuplicate
	}
	m.Mocks[v.ID] = v
	m.appendMockVersionLocked(v, actorID, "created", at)
	m.appendAuditLocked(actorID, "mock_interview.created", "mock_interview", v.ID, nil, at)
	return v, nil
}

// UpdateMockInterview updates a value.
func (m *Memory) UpdateMockInterview(_ context.Context, v mockinterviews.Interview, actorID, reason string, at time.Time) (mockinterviews.Interview, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	current, ok := m.Mocks[v.ID]
	if !ok || current.DeletedAt != nil {
		return mockinterviews.Interview{}, ErrNotFound
	}
	if v.Revision != current.Revision+1 {
		return mockinterviews.Interview{}, ErrConflict
	}
	m.Mocks[v.ID] = v
	m.appendMockVersionLocked(v, actorID, reason, at)
	action := "mock_interview.updated"
	if reason == "soft deleted" {
		action = "mock_interview.deleted"
	} else if strings.HasPrefix(reason, "identity correction:") {
		action = "mock_interview.identities_corrected"
	} else if reason == "interviewee review" {
		action = "mock_interview.reviewed"
	}
	m.appendAuditLocked(actorID, action, "mock_interview", v.ID, map[string]any{"reason": reason}, at)
	return v, nil
}

func (m *Memory) appendMockVersionLocked(v mockinterviews.Interview, actorID, reason string, at time.Time) {
	raw, _ := json.Marshal(v)
	m.MockVersions = append(m.MockVersions, MockVersion{InterviewID: v.ID, Revision: v.Revision, ActorID: actorID, Reason: reason, SavedAt: at.UTC(), Snapshot: raw})
}

// AppendAudit performs the operation.
func (m *Memory) AppendAudit(_ context.Context, v model.AuditEvent) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Audits = append(m.Audits, v)
	return nil
}

func (m *Memory) appendAuditLocked(actorID, action, subjectType, subjectID string, data map[string]any, at time.Time) {
	actor := actorID
	if data == nil {
		data = map[string]any{}
	}
	m.Audits = append(m.Audits, model.AuditEvent{ID: id.New(), ActorID: &actor, Action: action, SubjectType: subjectType, SubjectID: subjectID, Data: data, OccurredAt: at.UTC()})
}
