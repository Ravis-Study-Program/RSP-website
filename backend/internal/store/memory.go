package store

import (
	"context"
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
