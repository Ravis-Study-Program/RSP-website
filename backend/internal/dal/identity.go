package dal

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/magedmg/RSP-website/backend/internal/accounts"
	"github.com/magedmg/RSP-website/backend/internal/platform/audit"
	"github.com/magedmg/RSP-website/backend/internal/platform/id"
)

type identityReceipt struct {
	AuthSubject     string
	EventType       string
	SecurityVersion int64
	PayloadHash     string
}

// ApplyIdentityEvent applies an event from the authentication service exactly
// once. The receipt, account changes, and audit event share one transaction.
func (p *Store) ApplyIdentityEvent(ctx context.Context, event accounts.IdentityEvent) error {
	if event.EventID == "" {
		return errors.New("identity event id is required")
	}

	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())

	payloadHash := accounts.IdentityEventHash(event)
	receipt, err := tx.Exec(ctx, `INSERT INTO app.identity_event_receipts(event_id,auth_subject,event_type,security_version,payload_hash)
 VALUES($1,$2,$3,$4,$5) ON CONFLICT DO NOTHING`, event.EventID, event.AuthUserID, event.Type, event.SecurityVersion, payloadHash)
	if err != nil {
		return err
	}
	if receipt.RowsAffected() == 0 {
		var existing identityReceipt
		err := tx.QueryRow(ctx, `SELECT auth_subject,event_type,security_version,payload_hash FROM app.identity_event_receipts WHERE event_id=$1`, event.EventID).Scan(&existing.AuthSubject, &existing.EventType, &existing.SecurityVersion, &existing.PayloadHash)
		if err != nil {
			return err
		}
		if existing.AuthSubject != event.AuthUserID || existing.EventType != event.Type ||
			existing.SecurityVersion != event.SecurityVersion || existing.PayloadHash != payloadHash {
			return ErrConflict
		}
		return tx.Commit(ctx)
	}

	var link struct {
		UserID          string
		SecurityVersion int64
	}
	err = tx.QueryRow(ctx, `SELECT l.user_id,s.security_version
		FROM app.user_auth_links l
		JOIN app.users u ON u.id=l.user_id
		JOIN app.user_security s ON s.user_id=u.id
		WHERE l.auth_subject=$1
		FOR UPDATE OF l,u`, event.AuthUserID).Scan(&link.UserID, &link.SecurityVersion)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	if link.UserID == "" && event.Type == "auth_user_created" {
		return p.createIdentityUser(ctx, tx, event)
	}
	if link.UserID == "" {
		return ErrNotFound
	}
	if event.SecurityVersion < 1 {
		return errors.New("invalid security version")
	}
	if event.SecurityVersion < link.SecurityVersion {
		return tx.Commit(ctx)
	}
	if _, err := tx.Exec(ctx, `UPDATE app.user_security SET security_version=GREATEST(security_version, $1) WHERE user_id = $2`, event.SecurityVersion, link.UserID); err != nil {
		return err
	}

	if err := applyIdentityChange(ctx, tx, link.UserID, event); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (p *Store) createIdentityUser(ctx context.Context, tx pgx.Tx, event accounts.IdentityEvent) error {
	userID := id.New()
	name := event.Email
	if at := strings.IndexByte(name, '@'); at > 0 {
		name = name[:at]
	}
	if strings.TrimSpace(name) == "" {
		name = "New member"
	}
	slug := "member-" + strings.ReplaceAll(userID, "-", "")[:12]
	if _, err := tx.Exec(ctx, `INSERT INTO app.users(id,account_state)
 VALUES($1,$2)`, userID, "active"); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO app.user_profiles(user_id,slug,display_name) VALUES($1,$2,$3)`, userID, slug, name); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO app.user_preferences(user_id,timezone,timezone_configured) VALUES($1,$2,$3)`, userID, "Australia/Adelaide", false); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO app.user_contacts(user_id,email) VALUES($1,$2)`, userID, event.Email); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO app.user_security(user_id,security_version) VALUES($1,$2)`, userID, event.SecurityVersion); err != nil {
		return err
	}
	var revokedAt *time.Time
	if !event.EmailVerified {
		value := event.OccurredAt.UTC()
		revokedAt = &value
	}
	if _, err := tx.Exec(ctx, `INSERT INTO app.user_auth_links(auth_subject,user_id,provider,provider_account_id,active,revoked_at)
 VALUES($1,$2,$3,$4,$5,$6)`, event.AuthUserID, userID, "better_auth", event.AuthUserID, event.EmailVerified, revokedAt); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func applyIdentityChange(ctx context.Context, tx pgx.Tx, userID string, event accounts.IdentityEvent) error {
	switch event.Type {
	case "auth_user_created":
		return nil
	case "sessions_revoked":
		return appendAuditTx(ctx, tx, newAudit(userID, "account.sessions_revoked", "user", userID, map[string]any{"reason": event.Reason}, event.OccurredAt))
	case "email_changed":
		email := strings.TrimSpace(event.Email)
		if email == "" {
			return errors.New("email_changed requires email")
		}
		if _, err := tx.Exec(ctx, `UPDATE app.user_contacts SET email=$1 WHERE user_id = $2`, email, userID); err != nil {
			return err
		}
		return appendAuditTx(ctx, tx, newAudit(userID, "account.email_changed", "user", userID, nil, event.OccurredAt))
	case "account_state_changed":
		if event.AccountState != "active" && event.AccountState != "suspended" {
			return errors.New("account_state_changed requires active or suspended")
		}
		if _, err := tx.Exec(ctx, `UPDATE app.users SET account_state=$1 WHERE id = $2`, event.AccountState, userID); err != nil {
			return err
		}
		auditActor := userID
		if event.ActorUserID != "" {
			auditActor = event.ActorUserID
		}
		return appendAuditTx(ctx, tx, newAudit(auditActor, "account.state_changed", "user", userID, map[string]any{
			"accountState": event.AccountState,
			"reason":       event.Reason,
		}, event.OccurredAt))
	case "email_verified":
		_, err := tx.Exec(ctx, `UPDATE app.user_auth_links SET active=$1,revoked_at=$2 WHERE auth_subject = $3`, true, nil, event.AuthUserID)
		return err
	case "deletion_requested":
		if _, err := tx.Exec(ctx, `UPDATE app.users SET account_state=$1 WHERE id = $2`, "deletion_pending", userID); err != nil {
			return err
		}
		return appendAuditTx(ctx, tx, newAudit(userID, "account.deletion_requested", "user", userID, map[string]any{"recoveryDeadline": event.RecoveryDeadline}, event.OccurredAt))
	case "deletion_cancelled":
		if _, err := tx.Exec(ctx, `UPDATE app.users SET account_state=$1 WHERE id = $2`, "active", userID); err != nil {
			return err
		}
		return appendAuditTx(ctx, tx, newAudit(userID, "account.deletion_cancelled", "user", userID, nil, event.OccurredAt))
	case "auth_pseudonymized":
		if _, err := tx.Exec(ctx, `UPDATE app.user_profiles SET slug='deleted-' || user_id,display_name=$1,avatar_url=NULL WHERE user_id=$2`, "Deleted member", userID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE app.user_contacts SET email=NULL,discord_id=NULL WHERE user_id=$1`, userID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE app.user_preferences SET timezone='UTC',timezone_configured=false WHERE user_id=$1`, userID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE app.user_security SET security_version=GREATEST(security_version, $1) WHERE user_id=$2`, event.SecurityVersion, userID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE app.users SET account_state=$1,deleted_at=$2 WHERE id = $3`, "deleted", event.OccurredAt, userID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE app.user_auth_links SET active=$1,revoked_at=$2 WHERE auth_subject = $3`, false, event.OccurredAt, event.AuthUserID); err != nil {
			return err
		}
		return appendAuditTx(ctx, tx, newAudit(userID, "account.pseudonymized", "user", userID, nil, event.OccurredAt))
	case "mfa_configured":
		return applyMFAConfigured(ctx, tx, userID, event.OccurredAt)
	case "mfa_disabled":
		return applyMFADisabled(ctx, tx, userID, event.OccurredAt)
	default:
		return errors.New("unsupported identity event")
	}
}

type activatedAssignment struct {
	ID       string
	Role     string
	SeasonID string
}

func applyMFAConfigured(ctx context.Context, tx pgx.Tx, userID string, at time.Time) error {
	if _, err := tx.Exec(ctx, `UPDATE app.user_security SET mfa_configured=$1 WHERE user_id = $2`, true, userID); err != nil {
		return err
	}
	globalRows, err := tx.Query(ctx, `UPDATE app.global_role_assignments
		SET state='active',activated_at=$1
		WHERE user_id=$2 AND state='pending_mfa'
		RETURNING id,role::text AS role`, at.UTC(), userID)
	if err != nil {
		return err
	}
	global, err := pgx.CollectRows(globalRows, pgx.RowToStructByNameLax[activatedAssignment])
	if err != nil {
		return err
	}
	for _, assignment := range global {
		event := audit.Event{ID: id.New(), ActorID: &userID, Action: "global_role.activated", SubjectType: "global_role_assignment", SubjectID: assignment.ID, Data: map[string]any{"role": assignment.Role, "reason": "mfa_configured"}, OccurredAt: at.UTC()}
		if err := appendAuditTx(ctx, tx, event); err != nil {
			return err
		}
	}

	seasonalRows, err := tx.Query(ctx, `UPDATE app.enrollments
		SET assignment_state='active',activated_at=$1
		WHERE user_id=$2 AND role='coordinator' AND state='active'
		  AND assignment_state='pending_mfa' AND deleted_at IS NULL
		RETURNING id,season_id`, at.UTC(), userID)
	if err != nil {
		return err
	}
	seasonal, err := pgx.CollectRows(seasonalRows, pgx.RowToStructByNameLax[activatedAssignment])
	if err != nil {
		return err
	}
	for _, enrollment := range seasonal {
		if err := appendAuditTx(ctx, tx, newAudit(userID, "coordinator_role.activated", "enrollment", enrollment.ID, map[string]any{"seasonId": enrollment.SeasonID, "reason": "mfa_configured"}, at)); err != nil {
			return err
		}
	}
	_, err = tx.Exec(ctx, `UPDATE app.enrollments SET close_assignment_state=$1,close_activated_at=$2 WHERE user_id = $3 AND role = 'coordinator' AND state = 'completed' AND completed_by_close_id IS NOT NULL AND close_assignment_state = 'pending_mfa' AND deleted_at IS NULL`, "active", at.UTC(), userID)
	return err
}

func applyMFADisabled(ctx context.Context, tx pgx.Tx, userID string, at time.Time) error {
	if _, err := tx.Exec(ctx, `UPDATE app.user_security SET mfa_configured=$1 WHERE user_id = $2`, false, userID); err != nil {
		return err
	}
	globalRows, err := tx.Query(ctx, `UPDATE app.global_role_assignments
		SET state='pending_mfa',activated_at=NULL,revoked_at=NULL
		WHERE user_id=$1 AND state='active'
		RETURNING id,role::text AS role`, userID)
	if err != nil {
		return err
	}
	global, err := pgx.CollectRows(globalRows, pgx.RowToStructByNameLax[activatedAssignment])
	if err != nil {
		return err
	}
	for _, assignment := range global {
		if err := appendAuditTx(ctx, tx, newAudit(userID, "global_role.pending_mfa", "global_role_assignment", assignment.ID, map[string]any{"role": assignment.Role, "reason": "mfa_disabled"}, at)); err != nil {
			return err
		}
	}

	seasonalRows, err := tx.Query(ctx, `UPDATE app.enrollments
		SET assignment_state='pending_mfa',activated_at=NULL
		WHERE user_id=$1 AND role='coordinator' AND state='active'
		  AND assignment_state='active' AND deleted_at IS NULL
		RETURNING id,season_id`, userID)
	if err != nil {
		return err
	}
	seasonal, err := pgx.CollectRows(seasonalRows, pgx.RowToStructByNameLax[activatedAssignment])
	if err != nil {
		return err
	}
	for _, enrollment := range seasonal {
		if err := appendAuditTx(ctx, tx, newAudit(userID, "coordinator_role.pending_mfa", "enrollment", enrollment.ID, map[string]any{"seasonId": enrollment.SeasonID, "reason": "mfa_disabled"}, at)); err != nil {
			return err
		}
	}
	_, err = tx.Exec(ctx, `UPDATE app.enrollments SET close_assignment_state=$1,close_activated_at=$2 WHERE user_id = $3 AND role = 'coordinator' AND state = 'completed' AND completed_by_close_id IS NOT NULL AND close_assignment_state = 'active' AND deleted_at IS NULL`, "pending_mfa", nil, userID)
	return err
}
