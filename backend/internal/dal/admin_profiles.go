package dal

import (
	"context"
	"github.com/jackc/pgx/v5"
	"github.com/magedmg/RSP-website/backend/internal/accounts"
	"time"
)

const adminProfileQuery = `SELECT u.id,p.display_name,p.slug,p.avatar_url,COALESCE(c.email,''),c.discord_id FROM app.users u JOIN app.user_profiles p ON p.user_id=u.id JOIN app.user_contacts c ON c.user_id=u.id WHERE u.id=$1 AND u.deleted_at IS NULL`

func scanAdminProfile(row pgx.Row) (accounts.AdminProfile, error) {
	var v accounts.AdminProfile
	err := row.Scan(&v.ID, &v.Name, &v.Slug, &v.AvatarURL, &v.Email, &v.DiscordID)
	return v, noRows(err)
}
func (p *Store) GetAdminProfile(ctx context.Context, userID string) (accounts.AdminProfile, error) {
	return scanAdminProfile(p.pool.QueryRow(ctx, adminProfileQuery, userID))
}
func (p *Store) UpdateAdminProfile(ctx context.Context, v accounts.AdminProfile, actorID string, at time.Time) (accounts.AdminProfile, error) {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return v, err
	}
	defer tx.Rollback(context.Background())
	before, err := scanAdminProfile(tx.QueryRow(ctx, adminProfileQuery+` FOR UPDATE OF u,p,c`, v.ID))
	if err != nil {
		return v, err
	}
	if _, err = tx.Exec(ctx, `UPDATE app.user_profiles SET display_name=$2,slug=$3,avatar_url=$4 WHERE user_id=$1`, v.ID, v.Name, v.Slug, v.AvatarURL); err != nil {
		return v, mapDatabaseError(err)
	}
	if _, err = tx.Exec(ctx, `UPDATE app.user_contacts SET discord_id=$2 WHERE user_id=$1`, v.ID, v.DiscordID); err != nil {
		return v, err
	}
	v.Email = before.Email
	if err = appendAuditTx(ctx, tx, newAudit(actorID, "user.profile_corrected", "user", v.ID, map[string]any{"before": before, "after": v}, at)); err != nil {
		return v, err
	}
	return v, tx.Commit(ctx)
}
