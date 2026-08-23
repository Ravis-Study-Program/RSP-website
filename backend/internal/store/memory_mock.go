package store

import (
	"context"
	"encoding/json"
	"sort"
	"strings"
	"time"

	"github.com/magedmg/RSP-website/backend/internal/authz"
	"github.com/magedmg/RSP-website/backend/internal/mockinterviews"
)

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
	m.MockVersions = append(m.MockVersions, mockinterviews.Version{InterviewID: v.ID, Revision: v.Revision, ActorID: actorID, Reason: reason, SavedAt: at.UTC(), Snapshot: raw})
}
