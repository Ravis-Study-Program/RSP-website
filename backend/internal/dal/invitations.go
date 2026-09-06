package dal

import (
	"context"
	"github.com/jackc/pgx/v5"
	"github.com/magedmg/RSP-website/backend/internal/platform/id"
	"github.com/magedmg/RSP-website/backend/internal/programme"
	"time"
)

const invitationColumns = `i.id,i.season_id,s.name,s.slug,i.name,i.email,i.role::text,
 CASE WHEN i.accepted_at IS NOT NULL THEN 'accepted' WHEN i.cancelled_at IS NOT NULL THEN 'cancelled' WHEN i.expires_at<=now() THEN 'expired' ELSE 'pending' END,
 i.expires_at,i.sent_at,i.delivery_id`

func scanInvitation(row pgx.Row) (programme.Invitation, error) {
	var v programme.Invitation
	err := row.Scan(&v.ID, &v.SeasonID, &v.SeasonName, &v.SeasonSlug, &v.Name, &v.Email, &v.Role, &v.Status, &v.ExpiresAt, &v.SentAt, &v.DeliveryID)
	return v, noRows(err)
}

// Taking the season lock serializes enrollment/invitation writes with closure
// and empty-season deletion.
func lockOpenSeason(ctx context.Context, tx pgx.Tx, seasonID string) error {
	var status string
	if err := tx.QueryRow(ctx, `SELECT status::text FROM app.seasons WHERE id=$1 AND deleted_at IS NULL FOR UPDATE`, seasonID).Scan(&status); err != nil {
		return noRows(err)
	}
	if status != "open" {
		return ErrConflict
	}
	return nil
}

func (p *Store) ListInvitations(ctx context.Context, seasonID, boundary string, limit int) ([]programme.Invitation, bool, int64, error) {
	var total int64
	if err := p.pool.QueryRow(ctx, `SELECT count(*) FROM app.season_invitations WHERE season_id=$1`, seasonID).Scan(&total); err != nil {
		return nil, false, 0, err
	}
	rows, err := p.pool.Query(ctx, `SELECT `+invitationColumns+` FROM app.season_invitations i JOIN app.seasons s ON s.id=i.season_id WHERE i.season_id=$1 AND ($2='' OR i.id>NULLIF($2,'')::uuid) ORDER BY i.id LIMIT $3`, seasonID, boundary, limit+1)
	if err != nil {
		return nil, false, 0, err
	}
	defer rows.Close()
	items := []programme.Invitation{}
	for rows.Next() {
		v, err := scanInvitation(rows)
		if err != nil {
			return nil, false, 0, err
		}
		items = append(items, v)
	}
	items, more := finishPage(items, limit, "forward")
	return items, more, total, rows.Err()
}

func (p *Store) CreateInvitation(ctx context.Context, v programme.Invitation, hash, actorID string, at time.Time) (programme.Invitation, error) {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return v, err
	}
	defer tx.Rollback(context.Background())
	if err = lockOpenSeason(ctx, tx, v.SeasonID); err != nil {
		return v, err
	}
	var enrolled bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM app.enrollments e JOIN app.user_contacts c ON c.user_id=e.user_id WHERE e.season_id=$1 AND lower(c.email)=$2)`, v.SeasonID, v.Email).Scan(&enrolled); err != nil {
		return v, err
	}
	if enrolled {
		return v, ErrDuplicate
	}
	if _, err = tx.Exec(ctx, `INSERT INTO app.season_invitations(id,season_id,name,email,role,token_hash,delivery_id,expires_at,created_by) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)`, v.ID, v.SeasonID, v.Name, v.Email, v.Role, hash, v.DeliveryID, v.ExpiresAt, actorID); err != nil {
		return v, mapDatabaseError(err)
	}
	if err = appendAuditTx(ctx, tx, newAudit(actorID, "invitation.created", "invitation", v.ID, map[string]any{"seasonId": v.SeasonID, "role": v.Role}, at)); err != nil {
		return v, err
	}
	v, err = scanInvitation(tx.QueryRow(ctx, `SELECT `+invitationColumns+` FROM app.season_invitations i JOIN app.seasons s ON s.id=i.season_id WHERE i.id=$1`, v.ID))
	if err != nil {
		return v, err
	}
	return v, tx.Commit(ctx)
}

func (p *Store) ChangeInvitation(ctx context.Context, seasonID, inviteID, hash, deliveryID, actorID string, cancel bool, at time.Time) (programme.Invitation, error) {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return programme.Invitation{}, err
	}
	defer tx.Rollback(context.Background())
	if err = lockOpenSeason(ctx, tx, seasonID); err != nil {
		return programme.Invitation{}, err
	}
	v, err := scanInvitation(tx.QueryRow(ctx, `SELECT `+invitationColumns+` FROM app.season_invitations i JOIN app.seasons s ON s.id=i.season_id WHERE i.id=$1 AND i.season_id=$2 AND i.accepted_at IS NULL AND i.cancelled_at IS NULL FOR UPDATE OF i`, inviteID, seasonID))
	if err != nil {
		return v, err
	}
	action := "invitation.resent"
	if cancel {
		action = "invitation.cancelled"
		_, err = tx.Exec(ctx, `UPDATE app.season_invitations SET cancelled_at=$2 WHERE id=$1`, inviteID, at)
		v.Status = "cancelled"
	} else {
		v.DeliveryID = deliveryID
		v.ExpiresAt = at.Add(7 * 24 * time.Hour)
		v.SentAt = nil
		v.Status = "pending"
		_, err = tx.Exec(ctx, `UPDATE app.season_invitations SET token_hash=$2,delivery_id=$3,expires_at=$4,sent_at=NULL WHERE id=$1`, inviteID, hash, deliveryID, v.ExpiresAt)
	}
	if err != nil {
		return v, mapDatabaseError(err)
	}
	if err = appendAuditTx(ctx, tx, newAudit(actorID, action, "invitation", v.ID, map[string]any{"seasonId": seasonID}, at)); err != nil {
		return v, err
	}
	return v, tx.Commit(ctx)
}
func (p *Store) MarkInvitationSent(ctx context.Context, inviteID, deliveryID string, at time.Time) error {
	_, err := p.pool.Exec(ctx, `UPDATE app.season_invitations SET sent_at=$3 WHERE id=$1 AND delivery_id=$2`, inviteID, deliveryID, at)
	return err
}

func (p *Store) InvitationForUser(ctx context.Context, hash, userID string) (programme.Invitation, error) {
	return scanInvitation(p.pool.QueryRow(ctx, `SELECT `+invitationColumns+` FROM app.season_invitations i JOIN app.seasons s ON s.id=i.season_id JOIN app.user_contacts c ON lower(c.email)=i.email JOIN app.users u ON u.id=c.user_id
 WHERE i.token_hash=$1 AND c.user_id=$2 AND u.account_state='active' AND u.deleted_at IS NULL AND i.expires_at>now() AND i.accepted_at IS NULL AND i.cancelled_at IS NULL AND s.deleted_at IS NULL`, hash, userID))
}
func (p *Store) AcceptInvitation(ctx context.Context, hash, userID string, at time.Time) (programme.EnrollmentRecord, error) {
	v, err := p.InvitationForUser(ctx, hash, userID)
	if err != nil {
		return programme.EnrollmentRecord{}, err
	}
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return programme.EnrollmentRecord{}, err
	}
	defer tx.Rollback(context.Background())
	if err = lockOpenSeason(ctx, tx, v.SeasonID); err != nil {
		return programme.EnrollmentRecord{}, err
	}
	var inviteID string
	err = tx.QueryRow(ctx, `SELECT i.id FROM app.season_invitations i JOIN app.user_contacts c ON lower(c.email)=i.email JOIN app.users u ON u.id=c.user_id WHERE i.token_hash=$1 AND c.user_id=$2 AND u.account_state='active' AND u.deleted_at IS NULL AND i.expires_at>$3 AND i.accepted_at IS NULL AND i.cancelled_at IS NULL FOR UPDATE OF i,c,u`, hash, userID, at).Scan(&inviteID)
	if err != nil {
		return programme.EnrollmentRecord{}, noRows(err)
	}
	member := programme.EnrollmentRecord{ID: id.New(), SeasonID: v.SeasonID, SeasonSlug: v.SeasonSlug, UserID: userID, Role: v.Role, State: "active", StudentLevel: "not_applicable", AssignmentState: "active"}
	if member.Role == "student" {
		member.StudentLevel = "novice"
	}
	err = tx.QueryRow(ctx, `INSERT INTO app.enrollments(id,user_id,season_id,role,student_level,state) VALUES($1,$2,$3,$4,$5,'active') RETURNING assignment_state::text`, member.ID, member.UserID, member.SeasonID, member.Role, member.StudentLevel).Scan(&member.AssignmentState)
	if err != nil {
		return member, mapDatabaseError(err)
	}
	if _, err = tx.Exec(ctx, `UPDATE app.season_invitations SET accepted_at=$2,accepted_by=$3 WHERE id=$1`, inviteID, at, userID); err != nil {
		return member, err
	}
	if err = appendAuditTx(ctx, tx, newAudit(userID, "invitation.accepted", "invitation", inviteID, map[string]any{"seasonId": v.SeasonID, "enrollmentId": member.ID, "role": member.Role}, at)); err != nil {
		return member, err
	}
	return member, tx.Commit(ctx)
}
