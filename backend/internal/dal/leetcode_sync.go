package dal

import (
	"context"
	"time"

	"github.com/magedmg/RSP-website/backend/internal/platform/id"
)

// QueueLeetCodeSync records the worker request and audit event atomically.
func (s *Store) QueueLeetCodeSync(ctx context.Context, actorID, requestID string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())
	now, runID := time.Now().UTC(), id.New()
	if _, err := tx.Exec(ctx, `INSERT INTO app.leetcode_sync_runs(id,trigger_kind,requested_by_user_id,started_at)
  VALUES($1,'manual',$2,$3)`, runID, actorID, now); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO app.audit_events(id,actor_user_id,action,subject_type,subject_id,request_id,data,occurred_at)
  VALUES($1,$2,'leetcode.sync_requested','worker',$3,$4,'{}'::jsonb,$5)`, id.New(), actorID, runID, requestID, now); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
