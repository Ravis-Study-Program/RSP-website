package dal

import (
	"context"
	"encoding/json"
	"time"

	"github.com/magedmg/RSP-website/backend/internal/platform/audit"
	"github.com/magedmg/RSP-website/backend/internal/platform/dbtable"
	"github.com/magedmg/RSP-website/backend/internal/platform/id"
	"gorm.io/gorm"
)

func (p *Store) AppendAudit(ctx context.Context, event audit.Event) error {
	return appendAuditTx(ctx, p.DB, event)
}

func appendAuditTx(ctx context.Context, db *gorm.DB, event audit.Event) error {
	raw, err := json.Marshal(event.Data)
	if err != nil {
		return err
	}
	return db.WithContext(ctx).Table(dbtable.AuditEvents).Create(map[string]any{
		"id":            event.ID,
		"actor_user_id": event.ActorID,
		"action":        event.Action,
		"subject_type":  event.SubjectType,
		"subject_id":    event.SubjectID,
		"data":          gorm.Expr("?::jsonb", string(raw)),
		"occurred_at":   event.OccurredAt.UTC(),
	}).Error
}

func newAudit(actorID, action, subjectType, subjectID string, data map[string]any, at time.Time) audit.Event {
	actor := actorID
	if data == nil {
		data = map[string]any{}
	}
	return audit.Event{ID: id.New(), ActorID: &actor, Action: action, SubjectType: subjectType, SubjectID: subjectID, Data: data, OccurredAt: at.UTC()}
}
