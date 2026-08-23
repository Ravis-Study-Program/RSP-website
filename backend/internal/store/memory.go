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
