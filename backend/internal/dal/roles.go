package dal

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/magedmg/RSP-website/backend/internal/accounts"
	"github.com/magedmg/RSP-website/backend/internal/platform/id"
)

func (p *Store) GrantGlobalRole(ctx context.Context, userID, role string, activate bool, reason, actorID string, at time.Time) (accounts.GlobalRoleAssignment, error) {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return accounts.GlobalRoleAssignment{}, err
	}
	defer tx.Rollback(context.Background())

	state := "pending_mfa"
	var activatedAt *time.Time
	if activate {
		state = "active"
		activated := at.UTC()
		activatedAt = &activated
	}
	assignment := accounts.GlobalRoleAssignment{ID: id.New(), UserID: userID, Role: role, State: state, Revision: 1}
	err = tx.QueryRow(ctx, `INSERT INTO app.global_role_assignments(id,user_id,role,state,granted_by_user_id,granted_at,activated_at,revision)
		VALUES($1,$2,$3,$4,$5,$6,$7,1)
		ON CONFLICT(user_id,role) DO UPDATE
		SET state=EXCLUDED.state,granted_by_user_id=EXCLUDED.granted_by_user_id,
			granted_at=EXCLUDED.granted_at,activated_at=EXCLUDED.activated_at,
			revoked_at=NULL,revision=app.global_role_assignments.revision+1
		WHERE app.global_role_assignments.state='revoked'
		RETURNING id,state::text,revision`, assignment.ID, userID, role, state, actorID, at.UTC(), activatedAt).Scan(&assignment.ID, &assignment.State, &assignment.Revision)
	if err != nil {
		return assignment, mapDatabaseError(err)
	}
	if err := appendAuditTx(ctx, tx, newAudit(actorID, "global_role.granted_"+state, "global_role_assignment", assignment.ID, map[string]any{"userId": userID, "role": role, "reason": reason}, at)); err != nil {
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

	rows, err := p.pool.Query(ctx, `SELECT id,user_id,role::text,state::text,revision FROM app.global_role_assignments WHERE user_id=$1 AND state<>'revoked' ORDER BY role`, userID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByName[accounts.GlobalRoleAssignment])
}

func (p *Store) RevokeGlobalRole(ctx context.Context, userID, role string, revision int64, reason, actorID string, at time.Time) (accounts.GlobalRoleAssignment, error) {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return accounts.GlobalRoleAssignment{}, err
	}
	defer tx.Rollback(context.Background())

	assignment := accounts.GlobalRoleAssignment{UserID: userID, Role: role}
	err = tx.QueryRow(ctx, `UPDATE app.global_role_assignments
		SET state='revoked',revoked_at=$1,activated_at=NULL,revision=revision+1
		WHERE user_id=$2 AND role=$3 AND revision=$4 AND state<>'revoked'
		RETURNING id,state::text,revision`, at.UTC(), userID, role, revision).Scan(&assignment.ID, &assignment.State, &assignment.Revision)
	if errors.Is(err, pgx.ErrNoRows) {
		var current int64
		lookupErr := tx.QueryRow(ctx, `SELECT revision FROM app.global_role_assignments WHERE user_id = $1 AND role = $2 AND state <> 'revoked'`, userID, role).Scan(&current)
		if lookupErr == nil {
			return assignment, ErrConflict
		}
		return assignment, noRows(lookupErr)
	}
	if err != nil {
		return assignment, err
	}
	if err := appendAuditTx(ctx, tx, newAudit(actorID, "global_role.revoked", "global_role_assignment", assignment.ID, map[string]any{"userId": userID, "role": role, "reason": reason}, at)); err != nil {
		return assignment, err
	}
	return assignment, tx.Commit(ctx)
}
