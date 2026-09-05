package dal

import (
	"context"
	"encoding/json"
	"time"

	"github.com/magedmg/RSP-website/backend/internal/platform/audit"
	"github.com/magedmg/RSP-website/backend/internal/platform/id"
)

func (p *Store) AppendAudit(ctx context.Context, event audit.Event) error {
	return appendAuditTx(ctx, p.pool, event)
}

func appendAuditTx(ctx context.Context, db queryer, event audit.Event) error {
	raw, err := json.Marshal(event.Data)
	if err != nil {
		return err
	}
	_, err = db.Exec(ctx, `INSERT INTO app.audit_events(id,actor_user_id,action,subject_type,subject_id,data,occurred_at)
 VALUES($1,$2,$3,$4,$5,$6::jsonb,$7)`, event.ID, event.ActorID, event.Action, event.SubjectType, event.SubjectID, raw, event.OccurredAt.UTC())
	return err
}

func newAudit(actorID, action, subjectType, subjectID string, data map[string]any, at time.Time) audit.Event {
	actor := actorID
	if data == nil {
		data = map[string]any{}
	}
	return audit.Event{ID: id.New(), ActorID: &actor, Action: action, SubjectType: subjectType, SubjectID: subjectID, Data: data, OccurredAt: at.UTC()}
}
