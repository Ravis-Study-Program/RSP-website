package dal

import (
	"context"
	"time"
)

// DeleteEmptySeason keeps the audit record and rejects any membership or
// activity history, including previously removed memberships and records.
func (p *Store) DeleteEmptySeason(ctx context.Context, seasonID, actorID string, at time.Time) error {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())
	var name string
	if err = tx.QueryRow(ctx, `SELECT name FROM app.seasons WHERE id=$1 AND deleted_at IS NULL FOR UPDATE`, seasonID).Scan(&name); err != nil {
		return noRows(err)
	}
	var occupied bool
	err = tx.QueryRow(ctx, `SELECT
 EXISTS(SELECT 1 FROM app.enrollments WHERE season_id=$1)
 OR EXISTS(SELECT 1 FROM app.mock_interviews WHERE season_id=$1 OR season_week_id IN(SELECT id FROM app.season_weeks WHERE season_id=$1))
 OR EXISTS(SELECT 1 FROM app.problem_attempts WHERE season_week_id IN(SELECT id FROM app.season_weeks WHERE season_id=$1))
 OR EXISTS(SELECT 1 FROM app.season_invitations WHERE season_id=$1 AND accepted_at IS NULL AND cancelled_at IS NULL AND expires_at>$2)`, seasonID, at).Scan(&occupied)
	if err != nil {
		return err
	}
	if occupied {
		return ErrConflict
	}
	if _, err = tx.Exec(ctx, `UPDATE app.season_weeks SET deleted_at=$2 WHERE season_id=$1 AND deleted_at IS NULL`, seasonID, at); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `UPDATE app.seasons SET deleted_at=$2 WHERE id=$1`, seasonID, at); err != nil {
		return err
	}
	if err = appendAuditTx(ctx, tx, newAudit(actorID, "season.deleted", "season", seasonID, map[string]any{"name": name}, at)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
