package store

import (
	"context"
	"time"

	"github.com/magedmg/RSP-website/backend/internal/platform/audit"
	"github.com/magedmg/RSP-website/backend/internal/platform/id"
)

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
