package dal

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/magedmg/RSP-website/backend/internal/platform/id"
)

func (s *Store) BootstrapAdmin(ctx context.Context, subject, email string) error {
	subject = strings.TrimSpace(subject)
	email = strings.TrimSpace(strings.ToLower(email))
	if subject == "" || email == "" {
		return errors.New("--auth-subject and --email are required")
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}

	defer tx.Rollback(context.Background())
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(7277500101)`); err != nil {
		return err
	}

	var count int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM app.global_role_assignments WHERE role='system_admin'`).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return errors.New("a System Admin has already been bootstrapped")
	}

	var userID string
	err = tx.QueryRow(ctx, `SELECT u.id FROM app.users u JOIN app.user_auth_links l ON l.user_id=u.id WHERE l.auth_subject=$1 AND lower(u.email)=lower($2) AND l.active AND u.account_state='active' FOR UPDATE`, subject, email).Scan(&userID)
	if err != nil {
		return fmt.Errorf("verified linked user not found: %w", err)
	}

	assignmentID := id.New()
	var assignmentState string
	err = tx.QueryRow(ctx, `INSERT INTO app.global_role_assignments(id,user_id,role,state,granted_by_user_id,granted_at,revision) VALUES($1,$2,'system_admin','pending_mfa',NULL,now(),1) RETURNING state::text`, assignmentID, userID).Scan(&assignmentState)
	if err != nil {
		return err
	}

	auditAction, err := bootstrapAuditAction(assignmentState)
	if err != nil {
		return err
	}

	_, err = tx.Exec(ctx, `INSERT INTO app.audit_events(id,actor_user_id,action,subject_type,subject_id,data) VALUES($1,NULL,$2,'global_role_assignment',$3,jsonb_build_object('userId',$4::text,'state',$5::text))`, id.New(), auditAction, assignmentID, userID, assignmentState)
	if err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func bootstrapAuditAction(assignmentState string) (string, error) {
	switch assignmentState {
	case "active":
		return "system_admin.bootstrap_activated", nil
	case "pending_mfa":
		return "system_admin.bootstrap_pending_mfa", nil
	default:
		return "", fmt.Errorf("unexpected bootstrap assignment state %q", assignmentState)
	}
}
