package dal

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/magedmg/RSP-website/backend/internal/accounts"
	"github.com/magedmg/RSP-website/backend/internal/platform/id"
)

type GrantGlobalRoleInput struct {
	UserID        string
	Role          string
	MFAConfigured bool
	Reason        string
	ActorID       string
	ChangedAt     time.Time
}

func (p *Store) GrantGlobalRole(ctx context.Context, input GrantGlobalRoleInput) (accounts.GlobalRoleAssignment, error) {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return accounts.GlobalRoleAssignment{}, err
	}
	defer tx.Rollback(context.Background())

	state := "pending_mfa"
	var activatedAt *time.Time
	if input.MFAConfigured {
		state = "active"
		activated := input.ChangedAt.UTC()
		activatedAt = &activated
	}
	assignment := accounts.GlobalRoleAssignment{
		ID:     id.New(),
		UserID: input.UserID,
		Role:   input.Role,
		State:  state,
	}
	err = tx.QueryRow(ctx, `INSERT INTO app.global_role_assignments(id,user_id,role,state,granted_by_user_id,granted_at,activated_at)
		VALUES($1,$2,$3,$4,$5,$6,$7)
		ON CONFLICT(user_id,role) DO UPDATE
		SET state=EXCLUDED.state,granted_by_user_id=EXCLUDED.granted_by_user_id,
			granted_at=EXCLUDED.granted_at,activated_at=EXCLUDED.activated_at,
			revoked_at=NULL
		WHERE app.global_role_assignments.state='revoked'
		RETURNING id,state::text`, assignment.ID, input.UserID, input.Role, state, input.ActorID, input.ChangedAt.UTC(), activatedAt).Scan(&assignment.ID, &assignment.State)
	if err != nil {
		return assignment, mapDatabaseError(err)
	}
	if err := appendAuditTx(ctx, tx, newAudit(input.ActorID, "global_role.granted_"+state, "global_role_assignment", assignment.ID, map[string]any{"userId": input.UserID, "role": input.Role, "reason": input.Reason}, input.ChangedAt)); err != nil {
		return assignment, err
	}
	return assignment, tx.Commit(ctx)
}

func (p *Store) ListGlobalRoles(ctx context.Context, userID string) ([]accounts.GlobalRoleAssignment, error) {
	var userCount int64
	if err := p.pool.QueryRow(ctx, `SELECT count(*) FROM app.users WHERE id = $1 AND deleted_at IS NULL`, userID).Scan(&userCount); err != nil {
		return nil, err
	}
	if userCount == 0 {
		return nil, ErrNotFound
	}

	rows, err := p.pool.Query(ctx, `SELECT id,user_id,role::text,state::text FROM app.global_role_assignments WHERE user_id=$1 AND state<>'revoked' ORDER BY role`, userID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByName[accounts.GlobalRoleAssignment])
}

type RevokeGlobalRoleInput struct {
	UserID    string
	Role      string
	Reason    string
	ActorID   string
	ChangedAt time.Time
}

func (p *Store) RevokeGlobalRole(ctx context.Context, input RevokeGlobalRoleInput) (accounts.GlobalRoleAssignment, error) {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return accounts.GlobalRoleAssignment{}, err
	}
	defer tx.Rollback(context.Background())

	assignment := accounts.GlobalRoleAssignment{UserID: input.UserID, Role: input.Role}
	err = tx.QueryRow(ctx, `UPDATE app.global_role_assignments
		SET state='revoked',revoked_at=$1,activated_at=NULL
		WHERE user_id=$2 AND role=$3 AND state<>'revoked'
		RETURNING id,state::text`, input.ChangedAt.UTC(), input.UserID, input.Role).Scan(&assignment.ID, &assignment.State)
	if errors.Is(err, pgx.ErrNoRows) {
		return assignment, ErrNotFound
	}
	if err != nil {
		return assignment, err
	}
	if err := appendAuditTx(ctx, tx, newAudit(input.ActorID, "global_role.revoked", "global_role_assignment", assignment.ID, map[string]any{"userId": input.UserID, "role": input.Role, "reason": input.Reason}, input.ChangedAt)); err != nil {
		return assignment, err
	}
	return assignment, tx.Commit(ctx)
}
