package store

import (
	"context"
	"sort"
	"strings"
	"time"

	"github.com/magedmg/RSP-website/backend/internal/platform/audit"
	"github.com/magedmg/RSP-website/backend/internal/platform/id"
	"github.com/magedmg/RSP-website/backend/internal/programme"
)

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
