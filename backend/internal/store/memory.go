package store

import (
	"context"
	"errors"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/magedmg/RSP-website/backend/internal/accounts"
	"github.com/magedmg/RSP-website/backend/internal/authz"
	"github.com/magedmg/RSP-website/backend/internal/mockinterviews"
	"github.com/magedmg/RSP-website/backend/internal/platform/audit"
	"github.com/magedmg/RSP-website/backend/internal/platform/id"
	"github.com/magedmg/RSP-website/backend/internal/practice"
	"github.com/magedmg/RSP-website/backend/internal/programme"
)

// Memory represents a backend data structure.
type Memory struct {
	mu                       sync.RWMutex
	Users                    map[string]accounts.User
	PracticeSettings         map[string]practice.PracticeSettings
	AuthSubjects             map[string]authz.Actor
	Seasons                  map[string]programme.SeasonRecord
	Weeks                    map[string]programme.WeekRecord
	Enrollments              map[string]programme.EnrollmentRecord
	Mentorships              map[string]programme.MentorshipRecord
	Problems                 map[string]practice.ProblemRecord
	Attempts                 map[string]practice.AttemptRecord
	Recommendations          map[string]practice.Recommendation
	RecommendationDismissals map[string][]practice.Dismissal
	Mocks                    map[string]mockinterviews.Interview
	MockVersions             []mockinterviews.Version
	closeCompleted           map[string]map[string]bool
	closeAssignmentStates    map[string]map[string]string
	closeEventIDs            map[string]string
	Audits                   []audit.Event
	IdentityEventReceipts    map[string]bool
	IdentityEventHashes      map[string]string
	GlobalRoleAssignments    map[string]accounts.GlobalRoleAssignment
	MFAConfigured            map[string]bool
}

// NewMemory creates a new value.
func NewMemory() *Memory {
	return &Memory{Users: map[string]accounts.User{}, PracticeSettings: map[string]practice.PracticeSettings{}, AuthSubjects: map[string]authz.Actor{}, Seasons: map[string]programme.SeasonRecord{}, Weeks: map[string]programme.WeekRecord{}, Enrollments: map[string]programme.EnrollmentRecord{}, Mentorships: map[string]programme.MentorshipRecord{}, Problems: map[string]practice.ProblemRecord{}, Attempts: map[string]practice.AttemptRecord{}, Recommendations: map[string]practice.Recommendation{}, RecommendationDismissals: map[string][]practice.Dismissal{}, Mocks: map[string]mockinterviews.Interview{}, closeCompleted: map[string]map[string]bool{}, closeAssignmentStates: map[string]map[string]string{}, closeEventIDs: map[string]string{}, IdentityEventReceipts: map[string]bool{}, IdentityEventHashes: map[string]string{}, GlobalRoleAssignments: map[string]accounts.GlobalRoleAssignment{}, MFAConfigured: map[string]bool{}}
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

func defaultPracticeSettings(premium bool) practice.PracticeSettings {
	return practice.PracticeSettings{PremiumOptIn: premium, EasyMinutes: 20, MediumMinutes: 35, HardMinutes: 50, Revision: 1}
}

// GetPracticeSettings retrieves a value.
func (m *Memory) GetPracticeSettings(_ context.Context, userID string) (practice.PracticeSettings, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	user, ok := m.Users[userID]
	if !ok {
		return practice.PracticeSettings{}, ErrNotFound
	}
	settings, ok := m.PracticeSettings[userID]
	if !ok {
		settings = defaultPracticeSettings(user.PremiumOptIn)
	}
	settings.PremiumOptIn = user.PremiumOptIn
	return settings, nil
}

// UpdatePracticeSettings updates a value.
func (m *Memory) UpdatePracticeSettings(_ context.Context, userID string, revision int64, premium bool, easy, medium, hard int, actorID string, at time.Time) (practice.PracticeSettings, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	user, ok := m.Users[userID]
	if !ok {
		return practice.PracticeSettings{}, ErrNotFound
	}
	settings, ok := m.PracticeSettings[userID]
	if !ok {
		settings = defaultPracticeSettings(user.PremiumOptIn)
	}
	if settings.Revision != revision {
		return practice.PracticeSettings{}, ErrConflict
	}
	if settings.GoalsEnabled && (easy < 1 || medium < 1 || hard < 1) {
		return practice.PracticeSettings{}, errors.New("practice goals must be positive")
	}
	settings.PremiumOptIn, settings.EasyMinutes, settings.MediumMinutes, settings.HardMinutes = premium, easy, medium, hard
	settings.Revision++
	user.PremiumOptIn = premium
	user.Revision++
	m.Users[userID], m.PracticeSettings[userID] = user, settings
	actor := actorID
	m.Audits = append(m.Audits, audit.Event{ID: id.New(), ActorID: &actor, Action: "practice_settings.updated", SubjectType: "user", SubjectID: userID, Data: map[string]any{"premiumOptIn": premium, "easyMinutes": easy, "mediumMinutes": medium, "hardMinutes": hard}, OccurredAt: at.UTC()})
	return settings, nil
}

// EnablePracticeGoals performs the operation.
func (m *Memory) EnablePracticeGoals(_ context.Context, userID string, revision int64, actorID, seasonID string, at time.Time) (practice.PracticeSettings, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	user, ok := m.Users[userID]
	if !ok {
		return practice.PracticeSettings{}, ErrNotFound
	}
	settings, ok := m.PracticeSettings[userID]
	if !ok {
		settings = defaultPracticeSettings(user.PremiumOptIn)
	}
	if settings.Revision != revision {
		return practice.PracticeSettings{}, ErrConflict
	}
	if !settings.GoalsEnabled {
		settings.GoalsEnabled = true
		settings.Revision++
		m.PracticeSettings[userID] = settings
		actor := actorID
		m.Audits = append(m.Audits, audit.Event{ID: id.New(), ActorID: &actor, Action: "practice_goals.enabled", SubjectType: "user", SubjectID: userID, Data: map[string]any{"seasonId": seasonID}, OccurredAt: at.UTC()})
	}
	return settings, nil
}

// GetSeason retrieves a value.
func (m *Memory) GetSeason(_ context.Context, id string) (programme.SeasonRecord, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	v, ok := m.Seasons[id]
	if !ok {
		return programme.SeasonRecord{}, ErrNotFound
	}
	return v, nil
}

// ListSeasons lists matching values.
func (m *Memory) ListSeasons(_ context.Context, boundary string, limit int, direction, status string) ([]programme.SeasonRecord, bool, int64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	items := make([]programme.SeasonRecord, 0, len(m.Seasons))
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
func (m *Memory) CreateSeason(_ context.Context, v programme.SeasonRecord, actorID string, at time.Time) (programme.SeasonRecord, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.Seasons[v.ID]; ok {
		return programme.SeasonRecord{}, ErrDuplicate
	}
	for _, old := range m.Seasons {
		if old.Slug == v.Slug {
			return programme.SeasonRecord{}, ErrDuplicate
		}
	}
	m.Seasons[v.ID] = v
	m.appendAuditLocked(actorID, "season.created", "season", v.ID, nil, at)
	return v, nil
}

// UpdateSeason updates a value.
func (m *Memory) UpdateSeason(_ context.Context, id string, revision int64, fn func(*programme.SeasonRecord) error, actorID string, at time.Time) (programme.SeasonRecord, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	v, ok := m.Seasons[id]
	if !ok {
		return programme.SeasonRecord{}, ErrNotFound
	}
	if v.Revision != revision || v.Status != "open" {
		return programme.SeasonRecord{}, ErrConflict
	}
	if err := fn(&v); err != nil {
		return programme.SeasonRecord{}, err
	}

	for _, week := range m.Weeks {
		if week.SeasonID == id && (week.StartAt.Before(v.StartAt) || week.EndAt.After(v.EndAt)) {
			return programme.SeasonRecord{}, ErrConflict
		}
	}
	for _, interview := range m.Mocks {
		if interview.SeasonID != nil && *interview.SeasonID == id && (interview.OccurredAt.Before(v.StartAt) || interview.OccurredAt.After(v.EndAt)) {
			return programme.SeasonRecord{}, ErrConflict
		}
	}
	v.Revision++
	m.Seasons[id] = v
	m.appendAuditLocked(actorID, "season.updated", "season", id, nil, at)
	return v, nil
}

// CloseSeason closes a value.
func (m *Memory) CloseSeason(_ context.Context, seasonID string, revision int64, actorID, reason string, at time.Time) (programme.SeasonRecord, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	v, ok := m.Seasons[seasonID]
	if !ok {
		return programme.SeasonRecord{}, ErrNotFound
	}
	if v.Revision != revision {
		return programme.SeasonRecord{}, ErrConflict
	}
	domainSeason := programme.Season{ID: v.ID, Status: v.Status, Revision: v.Revision}
	for _, enrollment := range m.Enrollments {
		if enrollment.SeasonID == seasonID {
			domainSeason.Enrollments = append(domainSeason.Enrollments, programme.Enrollment{ID: enrollment.ID, State: enrollment.State, AssignmentState: enrollment.AssignmentState, Revision: enrollment.Revision})
		}
	}
	transition, err := programme.Close(domainSeason, actorID, reason, id.New(), at)
	if err != nil {
		return programme.SeasonRecord{}, ErrConflict
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
	m.closeEventIDs[seasonID] = transition.CloseEvent.ID
	actor := actorID
	m.Audits = append(m.Audits, audit.Event{ID: id.New(), ActorID: &actor, Action: "season.closed", SubjectType: "season", SubjectID: seasonID, Data: map[string]any{"reason": reason}, OccurredAt: at.UTC()})
	return v, nil
}

// ReopenSeason reopens a value.
func (m *Memory) ReopenSeason(_ context.Context, seasonID string, revision int64, actorID, reason string, at time.Time) (programme.SeasonRecord, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	v, ok := m.Seasons[seasonID]
	if !ok {
		return programme.SeasonRecord{}, ErrNotFound
	}
	if v.Revision != revision {
		return programme.SeasonRecord{}, ErrConflict
	}
	domainSeason := programme.Season{ID: v.ID, Status: v.Status, Revision: v.Revision}
	closeID := m.closeEventIDs[seasonID]
	if closeID == "" {
		// Test fixtures and legacy in-memory state can represent a closed
		// season without retaining its close event. There are no associated
		// enrollments to restore in that state, but the season transition is
		// still valid.
		closeID = "close-" + seasonID
	}
	for _, enrollment := range m.Enrollments {
		if enrollment.SeasonID == seasonID {
			completedBy := ""
			if m.closeCompleted[seasonID][enrollment.ID] {
				completedBy = closeID
			}
			closeAssignment := ""
			if completedBy != "" {
				closeAssignment = m.closeAssignmentStates[seasonID][enrollment.ID]
			}
			domainSeason.Enrollments = append(domainSeason.Enrollments, programme.Enrollment{ID: enrollment.ID, State: enrollment.State, AssignmentState: enrollment.AssignmentState, CloseAssignmentState: closeAssignment, Revision: enrollment.Revision, CompletedByCloseID: stringPointer(completedBy)})
		}
	}
	reopened, err := programme.Reopen(domainSeason, programme.CloseEvent{ID: closeID}, programme.SystemAdmin)
	if err != nil {
		return programme.SeasonRecord{}, ErrConflict
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
	delete(m.closeEventIDs, seasonID)
	actor := actorID
	m.Audits = append(m.Audits, audit.Event{ID: id.New(), ActorID: &actor, Action: "season.reopened", SubjectType: "season", SubjectID: seasonID, Data: map[string]any{"reason": reason}, OccurredAt: at.UTC()})
	return v, nil
}

func stringPointer(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

// ListWeeks lists matching values.
func (m *Memory) ListWeeks(_ context.Context, seasonID, boundary string, limit int, sortBy, direction string) ([]programme.WeekRecord, bool, int64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	items := []programme.WeekRecord{}
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
	page, more, err := memoryPage(items, boundary, limit, direction, func(v programme.WeekRecord) string { return v.ID })
	return page, more, total, err
}

// CreateWeek creates a value.
func (m *Memory) CreateWeek(_ context.Context, v programme.WeekRecord, actorID string, at time.Time) (programme.WeekRecord, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	season, exists := m.Seasons[v.SeasonID]
	if !exists {
		return programme.WeekRecord{}, ErrNotFound
	}
	if season.Status != "open" || !v.EndAt.After(v.StartAt) || v.StartAt.Before(season.StartAt) || v.EndAt.After(season.EndAt) {
		return programme.WeekRecord{}, ErrConflict
	}
	if _, exists := m.Weeks[v.ID]; exists {
		return programme.WeekRecord{}, ErrDuplicate
	}
	for _, old := range m.Weeks {
		if old.SeasonID == v.SeasonID && old.Number == v.Number {
			return programme.WeekRecord{}, ErrDuplicate
		}
	}
	m.Weeks[v.ID] = v
	m.appendAuditLocked(actorID, "week.created", "week", v.ID, nil, at)
	return v, nil
}

// UpdateWeek updates a value.
func (m *Memory) UpdateWeek(_ context.Context, seasonID, weekID string, revision int64, candidate programme.WeekRecord, actorID string, at time.Time) (programme.WeekRecord, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	current, ok := m.Weeks[weekID]
	if !ok || current.SeasonID != seasonID {
		return programme.WeekRecord{}, ErrNotFound
	}
	if current.Revision != revision {
		return programme.WeekRecord{}, ErrConflict
	}
	season, ok := m.Seasons[seasonID]
	if !ok {
		return programme.WeekRecord{}, ErrNotFound
	}
	if season.Status != "open" || !candidate.EndAt.After(candidate.StartAt) || candidate.StartAt.Before(season.StartAt) || candidate.EndAt.After(season.EndAt) {
		return programme.WeekRecord{}, ErrConflict
	}
	for id, week := range m.Weeks {
		if id != weekID && week.SeasonID == seasonID && week.Number == candidate.Number {
			return programme.WeekRecord{}, ErrDuplicate
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
func (m *Memory) ListEnrollments(_ context.Context, seasonID, boundary string, limit int, role, state, sortBy, direction string, includeInactive bool) ([]programme.EnrollmentRecord, bool, int64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	items := m.listEnrollmentsLocked(func(v programme.EnrollmentRecord) bool {
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
	page, more, err := memoryPage(items, boundary, limit, direction, func(v programme.EnrollmentRecord) string { return v.ID })
	return page, more, total, err
}

// ListEnrollmentsForUser lists matching values.
func (m *Memory) ListEnrollmentsForUser(_ context.Context, userID string) ([]programme.EnrollmentRecord, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.listEnrollmentsLocked(func(v programme.EnrollmentRecord) bool { return v.UserID == userID }), nil
}

func (m *Memory) listEnrollmentsLocked(include func(programme.EnrollmentRecord) bool) []programme.EnrollmentRecord {
	items := []programme.EnrollmentRecord{}
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
func (m *Memory) GetEnrollment(_ context.Context, enrollmentID string) (programme.EnrollmentRecord, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	v, ok := m.Enrollments[enrollmentID]
	if !ok {
		return programme.EnrollmentRecord{}, ErrNotFound
	}
	return normalizeEnrollment(v), nil
}

func normalizeEnrollment(v programme.EnrollmentRecord) programme.EnrollmentRecord {
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
func (m *Memory) CreateEnrollment(_ context.Context, v programme.EnrollmentRecord, actorID string, at time.Time) (programme.EnrollmentRecord, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.Users[v.UserID]; !exists {
		return programme.EnrollmentRecord{}, ErrNotFound
	}
	if _, exists := m.Seasons[v.SeasonID]; !exists {
		return programme.EnrollmentRecord{}, ErrNotFound
	}
	for _, old := range m.Enrollments {
		if old.SeasonID == v.SeasonID && old.UserID == v.UserID {
			return programme.EnrollmentRecord{}, ErrDuplicate
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
func (m *Memory) UpdateEnrollmentDetails(_ context.Context, seasonID, enrollmentID string, revision int64, role, studentLevel, actorID string, at time.Time) (programme.EnrollmentRecord, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	v, ok := m.Enrollments[enrollmentID]
	if !ok || v.SeasonID != seasonID {
		return programme.EnrollmentRecord{}, ErrNotFound
	}
	if v.Revision != revision || v.State != "active" {
		return programme.EnrollmentRecord{}, ErrConflict
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
func (m *Memory) UpdateEnrollment(_ context.Context, enrollmentID string, revision int64, role, state, reason, actorID string, at time.Time) (programme.EnrollmentRecord, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	v, ok := m.Enrollments[enrollmentID]
	if !ok {
		return programme.EnrollmentRecord{}, ErrNotFound
	}
	if v.Revision != revision || v.State != "active" {
		return programme.EnrollmentRecord{}, ErrConflict
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
		m.Audits = append(m.Audits, audit.Event{ID: id.New(), ActorID: &actor, Action: "enrollment.promoted", SubjectType: "enrollment", SubjectID: enrollmentID, Data: map[string]any{"role": role}, OccurredAt: at.UTC()})
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
		m.Audits = append(m.Audits, audit.Event{ID: id.New(), ActorID: &actor, Action: "enrollment.removed", SubjectType: "enrollment", SubjectID: enrollmentID, Data: map[string]any{"reason": trimmed, "state": state}, OccurredAt: at.UTC()})
	}
	v = normalizeEnrollment(v)
	v.Revision++
	m.Enrollments[enrollmentID] = v
	return v, nil
}

// ListMentorships lists matching values.
func (m *Memory) ListMentorships(_ context.Context, seasonID, boundary string, limit int, sortBy, direction, mentorUserID, studentUserID string) ([]programme.MentorshipRecord, bool, int64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	items := []programme.MentorshipRecord{}
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
	page, more, err := memoryPage(items, boundary, limit, direction, func(v programme.MentorshipRecord) string { return v.ID })
	return page, more, total, err
}

// CreateMentorship creates a value.
func (m *Memory) CreateMentorship(_ context.Context, v programme.MentorshipRecord, actorID string, at time.Time) (programme.MentorshipRecord, error) {
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
		return programme.MentorshipRecord{}, ErrConflict
	}
	for _, old := range m.Mentorships {
		if old.SeasonID == v.SeasonID && old.StudentUserID == v.StudentUserID {
			return programme.MentorshipRecord{}, ErrDuplicate
		}
	}
	m.Mentorships[v.ID] = v
	m.appendAuditLocked(actorID, "mentorship.created", "mentorship", v.ID, nil, at)
	return v, nil
}

// UpdateMentorship updates a value.
func (m *Memory) UpdateMentorship(_ context.Context, seasonID, mentorshipID string, revision int64, mentorUserID, studentUserID, actorID string, at time.Time) (programme.MentorshipRecord, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	v, ok := m.Mentorships[mentorshipID]
	if !ok || v.SeasonID != seasonID {
		return programme.MentorshipRecord{}, ErrNotFound
	}
	if v.Revision != revision {
		return programme.MentorshipRecord{}, ErrConflict
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
		return programme.MentorshipRecord{}, ErrConflict
	}
	for id, other := range m.Mentorships {
		if id != mentorshipID && other.SeasonID == seasonID && other.StudentUserID == studentUserID {
			return programme.MentorshipRecord{}, ErrDuplicate
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
func (m *Memory) ListProblems(_ context.Context, boundary string, limit int, difficulty, category string, premium *bool, direction string) ([]practice.ProblemRecord, bool, int64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	items := make([]practice.ProblemRecord, 0, len(m.Problems))
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
func (m *Memory) GetAttempt(_ context.Context, id string) (practice.AttemptRecord, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	v, ok := m.Attempts[id]
	if !ok || v.DeletedAt != nil {
		return practice.AttemptRecord{}, ErrNotFound
	}
	return v, nil
}

// ListAttempts lists matching values.
func (m *Memory) ListAttempts(_ context.Context, userID, boundary string, limit int, outcome, difficulty, direction string) ([]practice.AttemptRecord, bool, int64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	items := []practice.AttemptRecord{}
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
func (m *Memory) RecommendationSnapshot(_ context.Context, userID string, _ practice.Goals) (practice.RecommendationSnapshot, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	snapshot := practice.RecommendationSnapshot{ProblemHistory: map[string]practice.ProblemHistory{}, CategoryExposure: map[string]int{}}
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
func (m *Memory) RecommendationCandidates(_ context.Context, userID string, criteria practice.Criteria, premiumOptIn bool, goals practice.Goals, now time.Time) ([]practice.ProblemRecord, map[string]practice.ProblemHistory, error) {
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
	better := func(candidate practice.ProblemRecord, current *practice.ProblemRecord) bool {
		return current == nil || candidate.Number < current.Number || candidate.Number == current.Number && candidate.ID < current.ID
	}
	var unseen, retry *practice.ProblemRecord
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
		return []practice.ProblemRecord{*unseen}, map[string]practice.ProblemHistory{}, nil
	}
	if retry != nil {
		return []practice.ProblemRecord{*retry}, map[string]practice.ProblemHistory{retry.ID: history[retry.ID]}, nil
	}
	return []practice.ProblemRecord{}, map[string]practice.ProblemHistory{}, nil
}

// CreateAttempt creates a value.
func (m *Memory) CreateAttempt(_ context.Context, v practice.AttemptRecord) (practice.AttemptRecord, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.Attempts[v.ID]; ok {
		return practice.AttemptRecord{}, false, ErrDuplicate
	}
	if _, ok := m.Problems[v.ProblemID]; !ok {
		return practice.AttemptRecord{}, false, ErrNotFound
	}
	if v.WeekID != nil && v.SeasonID == nil {
		return practice.AttemptRecord{}, false, ErrNotFound
	}
	if v.SeasonID != nil {
		season, seasonExists := m.Seasons[*v.SeasonID]
		activeEnrollment := false
		for _, enrollment := range m.Enrollments {
			activeEnrollment = activeEnrollment || (enrollment.UserID == v.UserID && enrollment.SeasonID == *v.SeasonID && enrollment.State == "active")
		}
		if !seasonExists || season.Status != "open" || !activeEnrollment {
			return practice.AttemptRecord{}, false, ErrNotFound
		}
		if v.WeekID != nil {
			week, ok := m.Weeks[*v.WeekID]
			if !ok || week.SeasonID != *v.SeasonID {
				return practice.AttemptRecord{}, false, ErrNotFound
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
	m.Audits = append(m.Audits, audit.Event{ID: id.New(), ActorID: &actor, Action: "attempt.created", SubjectType: "problem_attempt", SubjectID: v.ID, Data: map[string]any{}, OccurredAt: time.Now().UTC()})
	return v, fulfilled, nil
}

// UpdateAttempt updates a value.
func (m *Memory) UpdateAttempt(_ context.Context, id, userID string, revision int64, fn func(*practice.AttemptRecord) error) (practice.AttemptRecord, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	v, ok := m.Attempts[id]
	if !ok || v.DeletedAt != nil {
		return practice.AttemptRecord{}, ErrNotFound
	}
	if v.UserID != userID {
		return practice.AttemptRecord{}, ErrNotFound
	}
	if v.Revision != revision {
		return practice.AttemptRecord{}, ErrConflict
	}
	if err := fn(&v); err != nil {
		return practice.AttemptRecord{}, err
	}
	if _, ok := m.Problems[v.ProblemID]; !ok || (v.WeekID != nil && v.SeasonID == nil) {
		return practice.AttemptRecord{}, ErrNotFound
	}
	if v.SeasonID != nil {
		season, exists := m.Seasons[*v.SeasonID]
		active := false
		for _, enrollment := range m.Enrollments {
			active = active || (enrollment.UserID == userID && enrollment.SeasonID == *v.SeasonID && enrollment.State == "active")
		}
		if !exists || season.Status != "open" || !active {
			return practice.AttemptRecord{}, ErrConflict
		}
		if v.WeekID != nil && m.Weeks[*v.WeekID].SeasonID != *v.SeasonID {
			return practice.AttemptRecord{}, ErrConflict
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

func practiceProblem(problem practice.ProblemRecord) practice.Problem {
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
func (m *Memory) FulfillRecommendation(_ context.Context, userID string, attempt practice.AttemptRecord, at time.Time) (bool, error) {
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

// AppendAudit performs the operation.
func (m *Memory) AppendAudit(_ context.Context, v audit.Event) error {
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
	m.Audits = append(m.Audits, audit.Event{ID: id.New(), ActorID: &actor, Action: action, SubjectType: subjectType, SubjectID: subjectID, Data: data, OccurredAt: at.UTC()})
}
