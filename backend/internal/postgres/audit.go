package postgres

import (
	"context"
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/magedmg/RSP-website/backend/internal/platform/audit"
	"github.com/magedmg/RSP-website/backend/internal/platform/id"
)

func (p *Postgres) AppendAudit(ctx context.Context, v audit.Event) error {
	return appendAuditTx(ctx, p.Pool, v)
}

type execer interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
}

func appendAuditTx(ctx context.Context, db execer, v audit.Event) error {
	raw, err := json.Marshal(v.Data)
	if err != nil {
		return err
	}

	_, err = db.Exec(ctx, `INSERT INTO app.audit_events(id,actor_user_id,action,subject_type,subject_id,data,occurred_at) VALUES($1,$2,$3,$4,$5,$6,$7)`, v.ID, v.ActorID, v.Action, v.SubjectType, v.SubjectID, raw, v.OccurredAt)
	return err
}

func newAudit(actorID, action, subjectType, subjectID string, data map[string]any, at time.Time) audit.Event {
	actor := actorID
	if data == nil {
		data = map[string]any{}
	}
	return audit.Event{ID: id.New(), ActorID: &actor, Action: action, SubjectType: subjectType, SubjectID: subjectID, Data: data, OccurredAt: at.UTC()}
}
