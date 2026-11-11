package dal

import (
	"context"
	"time"
)

// DeleteSeason removes a season and everything in it from the site: its
// weeks, members, mentorships and pending invitations go with it. Rows are
// marked deleted rather than dropped, so the audit record of what existed is
// kept, but nothing of the season is shown or reachable afterwards.
func (p *Store) DeleteSeason(ctx context.Context, seasonID, actorID string, at time.Time) error {
	tx, err := p.conn().Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())
	var name string
	if err = tx.QueryRow(ctx, `SELECT name FROM app.seasons WHERE id=$1 AND deleted_at IS NULL FOR UPDATE`, seasonID).Scan(&name); err != nil {
		return noRows(err)
	}
	if _, err = tx.Exec(ctx, `UPDATE app.mentorships SET deleted_at=$2 WHERE season_id=$1 AND deleted_at IS NULL`, seasonID, at); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `UPDATE app.enrollments SET deleted_at=$2 WHERE season_id=$1 AND deleted_at IS NULL`, seasonID, at); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `UPDATE app.season_invitations SET cancelled_at=$2 WHERE season_id=$1 AND accepted_at IS NULL AND cancelled_at IS NULL`, seasonID, at); err != nil {
		return err
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
