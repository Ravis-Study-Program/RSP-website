package store

import (
	"context"
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/magedmg/RSP-website/backend/internal/accounts"
	"github.com/magedmg/RSP-website/backend/internal/authz"
	"github.com/magedmg/RSP-website/backend/internal/platform/id"
)

// ApplyIdentityEvent applies the operation.
func (m *Memory) ApplyIdentityEvent(_ context.Context, event accounts.IdentityEvent) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if event.EventID == "" {
		return errors.New("identity event id is required")
	}
	payloadHash := accounts.IdentityEventHash(event)
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
		m.Users[userID] = accounts.User{ID: userID, Slug: slug, Name: name, Email: event.Email, Timezone: "Australia/Adelaide", TimezoneConfigured: false, AccountState: "active", Revision: 1}
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
func (m *Memory) GrantGlobalRole(_ context.Context, userID, role string, activate bool, reason, actorID string, at time.Time) (accounts.GlobalRoleAssignment, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.Users[userID]; !ok {
		return accounts.GlobalRoleAssignment{}, ErrNotFound
	}
	for _, assignment := range m.GlobalRoleAssignments {
		if assignment.UserID == userID && assignment.Role == role && assignment.State != "revoked" {
			return accounts.GlobalRoleAssignment{}, ErrDuplicate
		}
	}
	state := "pending_mfa"
	if activate {
		state = "active"
	}
	v := accounts.GlobalRoleAssignment{ID: id.New(), UserID: userID, Role: role, State: state, Revision: 1}
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
func (m *Memory) ListGlobalRoles(_ context.Context, userID string) ([]accounts.GlobalRoleAssignment, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if _, ok := m.Users[userID]; !ok {
		return nil, ErrNotFound
	}
	items := []accounts.GlobalRoleAssignment{}
	for _, assignment := range m.GlobalRoleAssignments {
		if assignment.UserID == userID && assignment.State != "revoked" {
			items = append(items, assignment)
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Role < items[j].Role })
	return items, nil
}

// RevokeGlobalRole performs the operation.
func (m *Memory) RevokeGlobalRole(_ context.Context, userID, role string, revision int64, reason, actorID string, at time.Time) (accounts.GlobalRoleAssignment, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for assignmentID, assignment := range m.GlobalRoleAssignments {
		if assignment.UserID != userID || assignment.Role != role || assignment.State == "revoked" {
			continue
		}
		if assignment.Revision != revision {
			return accounts.GlobalRoleAssignment{}, ErrConflict
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
	return accounts.GlobalRoleAssignment{}, ErrNotFound
}

// GetUser retrieves a value.
func (m *Memory) GetUser(_ context.Context, id string) (accounts.User, error) {
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
		return accounts.User{}, ErrNotFound
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
func (m *Memory) ListUsers(_ context.Context, boundary string, limit int, direction, query, seasonRole, globalRole string) ([]accounts.User, bool, int64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	items := make([]accounts.User, 0, len(m.Users))
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
func (m *Memory) ListAdminUsers(_ context.Context, boundary string, limit int, direction, query, accountState, globalRole string) ([]accounts.User, bool, int64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	query = strings.ToLower(strings.TrimSpace(query))
	items := make([]accounts.User, 0, len(m.Users))
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
func (m *Memory) ListEnrollmentCandidates(_ context.Context, seasonID, query, boundary string, limit int, direction string) ([]accounts.EnrollmentCandidate, bool, int64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	query = strings.ToLower(strings.TrimSpace(query))
	items := []accounts.EnrollmentCandidate{}
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
			items = append(items, accounts.EnrollmentCandidate{ID: user.ID, Slug: user.Slug, Name: user.Name, AvatarURL: user.AvatarURL, Revision: user.Revision})
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

func (m *Memory) enrichUserLocked(v accounts.User) accounts.User {
	if v.GlobalRoles == nil {
		v.GlobalRoles = []string{}
	}
	v.SeasonRoles = []accounts.UserSeasonRole{}
	v.AttemptCount, v.MockInterviewCount = 0, 0
	for _, enrollment := range m.Enrollments {
		if enrollment.UserID != v.ID || (enrollment.State != "active" && !(enrollment.Role == "student" && enrollment.State == "completed")) {
			continue
		}
		season := m.Seasons[enrollment.SeasonID]
		v.SeasonRoles = append(v.SeasonRoles, accounts.UserSeasonRole{SeasonID: enrollment.SeasonID, SeasonSlug: season.Slug, Role: enrollment.Role, State: enrollment.State})
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
func (m *Memory) UpdateUser(_ context.Context, id string, revision int64, fn func(*accounts.User) error, actorID string, at time.Time) (accounts.User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	v, ok := m.Users[id]
	if !ok {
		return accounts.User{}, ErrNotFound
	}
	if v.Revision != revision {
		return accounts.User{}, ErrConflict
	}
	if err := fn(&v); err != nil {
		return accounts.User{}, err
	}

	for otherID, other := range m.Users {
		if otherID != id && strings.EqualFold(other.Slug, v.Slug) {
			return accounts.User{}, ErrDuplicate
		}
	}
	v.Revision++
	m.Users[id] = v
	m.appendAuditLocked(actorID, "user.updated", "user", id, nil, at)
	return m.enrichUserLocked(v), nil
}
