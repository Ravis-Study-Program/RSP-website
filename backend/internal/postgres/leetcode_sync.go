// Package store defines repository contracts and storage implementations.
package postgres

import (
	"context"
	"time"

	"github.com/magedmg/RSP-website/backend/internal/platform/dbtable"
	"github.com/magedmg/RSP-website/backend/internal/platform/id"
	"gorm.io/gorm"
)

// QueueLeetCodeSync records the worker request and audit event atomically.
func (p *Postgres) QueueLeetCodeSync(ctx context.Context, actorID, requestID string) error {
	tx, err := begin(ctx, p.DB)
	if err != nil {
		return err
	}
	defer rollback(tx)

	now := time.Now().UTC()
	runID := id.New()
	if err := tx.Table(dbtable.LeetcodeSyncRuns).Create(map[string]any{
		"id": runID, "trigger_kind": "manual", "requested_by_user_id": actorID, "started_at": now,
	}).Error; err != nil {
		return err
	}
	if err := tx.Table(dbtable.AuditEvents).Create(map[string]any{
		"id": id.New(), "actor_user_id": actorID,
		"action": "leetcode.sync_requested", "subject_type": "worker",
		"subject_id": runID, "request_id": requestID, "data": gorm.Expr("'{}'::jsonb"),
		"occurred_at": now,
	}).Error; err != nil {
		return err
	}
	return commit(tx)
}
