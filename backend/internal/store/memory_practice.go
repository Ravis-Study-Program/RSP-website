package store

import (
	"context"
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/magedmg/RSP-website/backend/internal/platform/audit"
	"github.com/magedmg/RSP-website/backend/internal/platform/id"
	"github.com/magedmg/RSP-website/backend/internal/practice"
)

func defaultPracticeSettings() practice.PracticeSettings {
	return practice.PracticeSettings{EasyMinutes: 20, MediumMinutes: 35, HardMinutes: 50, Revision: 1}
}

// GetPracticeSettings retrieves a value.
func (m *Memory) GetPracticeSettings(_ context.Context, userID string) (practice.PracticeSettings, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if _, ok := m.Users[userID]; !ok {
		return practice.PracticeSettings{}, ErrNotFound
	}
	settings, ok := m.PracticeSettings[userID]
	if !ok {
		settings = defaultPracticeSettings()
	}
	return settings, nil
}

// UpdatePracticeSettings updates a value.
func (m *Memory) UpdatePracticeSettings(_ context.Context, userID string, revision int64, easy, medium, hard int, actorID string, at time.Time) (practice.PracticeSettings, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.Users[userID]; !ok {
		return practice.PracticeSettings{}, ErrNotFound
	}
	settings, ok := m.PracticeSettings[userID]
	if !ok {
		settings = defaultPracticeSettings()
	}
	if settings.Revision != revision {
		return practice.PracticeSettings{}, ErrConflict
	}
	if settings.GoalsEnabled && (easy < 1 || medium < 1 || hard < 1) {
		return practice.PracticeSettings{}, errors.New("practice goals must be positive")
	}
	settings.EasyMinutes, settings.MediumMinutes, settings.HardMinutes = easy, medium, hard
	settings.Revision++
	m.PracticeSettings[userID] = settings
	actor := actorID
	m.Audits = append(m.Audits, audit.Event{ID: id.New(), ActorID: &actor, Action: "practice_settings.updated", SubjectType: "user", SubjectID: userID, Data: map[string]any{"easyMinutes": easy, "mediumMinutes": medium, "hardMinutes": hard}, OccurredAt: at.UTC()})
	return settings, nil
}

// EnablePracticeGoals performs the operation.
func (m *Memory) EnablePracticeGoals(_ context.Context, userID string, revision int64, actorID, seasonID string, at time.Time) (practice.PracticeSettings, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.Users[userID]; !ok {
		return practice.PracticeSettings{}, ErrNotFound
	}
	settings, ok := m.PracticeSettings[userID]
	if !ok {
		settings = defaultPracticeSettings()
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

// CreateAttempt creates a value.
func (m *Memory) CreateAttempt(_ context.Context, v practice.AttemptRecord) (practice.AttemptRecord, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.Attempts[v.ID]; ok {
		return practice.AttemptRecord{}, ErrDuplicate
	}
	if _, ok := m.Problems[v.ProblemID]; !ok {
		return practice.AttemptRecord{}, ErrNotFound
	}
	if v.WeekID != nil && v.SeasonID == nil {
		return practice.AttemptRecord{}, ErrNotFound
	}
	if v.SeasonID != nil {
		season, seasonExists := m.Seasons[*v.SeasonID]
		activeEnrollment := false
		for _, enrollment := range m.Enrollments {
			activeEnrollment = activeEnrollment || (enrollment.UserID == v.UserID && enrollment.SeasonID == *v.SeasonID && enrollment.State == "active")
		}
		if !seasonExists || season.Status != "open" || !activeEnrollment {
			return practice.AttemptRecord{}, ErrNotFound
		}
		if v.WeekID != nil {
			week, ok := m.Weeks[*v.WeekID]
			if !ok || week.SeasonID != *v.SeasonID {
				return practice.AttemptRecord{}, ErrNotFound
			}
		}
	}
	m.Attempts[v.ID] = v
	actor := v.UserID
	m.Audits = append(m.Audits, audit.Event{ID: id.New(), ActorID: &actor, Action: "attempt.created", SubjectType: "problem_attempt", SubjectID: v.ID, Data: map[string]any{}, OccurredAt: time.Now().UTC()})
	return v, nil
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
