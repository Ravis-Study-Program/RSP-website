package dal

import (
	"context"

	"github.com/jackc/pgx/v5"

	"github.com/magedmg/RSP-website/backend/internal/authz"
)

func (p *Store) ResolveAuthSubject(ctx context.Context, subject string) (authz.Actor, error) {
	var actor authz.Actor
	var accountState string
	err := p.pool.QueryRow(ctx, `SELECT u.id,u.account_state::text,s.security_version
		FROM app.user_auth_links l
		JOIN app.users u ON u.id=l.user_id
		JOIN app.user_security s ON s.user_id=u.id
		WHERE l.auth_subject=$1 AND l.active AND u.deleted_at IS NULL`, subject).Scan(&actor.UserID, &accountState, &actor.SecurityVersion)
	if err != nil {
		return actor, noRows(err)
	}

	actor.EmailVerified = true
	actor.AccountState = authz.AccountState(accountState)
	actor.GlobalRoles = map[authz.GlobalRole]bool{}
	roleRows, err := p.pool.Query(ctx, `SELECT role::text FROM app.global_role_assignments WHERE user_id=$1 AND state='active'`, actor.UserID)
	if err != nil {
		return actor, err
	}
	roles, err := pgx.CollectRows(roleRows, pgx.RowTo[string])
	if err != nil {
		return actor, err
	}
	for _, role := range roles {
		actor.GlobalRoles[authz.GlobalRole(role)] = true
	}

	rows, err := p.pool.Query(ctx, `SELECT season_id, role::text, state::text FROM app.enrollments WHERE user_id = $1 AND (deleted_at IS NULL OR state IN ('kicked','withdrawn')) AND (role <> 'coordinator' OR state IN ('completed','kicked','withdrawn') OR assignment_state = 'active')`, actor.UserID)
	if err != nil {
		return actor, err
	}
	defer rows.Close()
	for rows.Next() {
		var enrollment authz.Enrollment
		var role, state string
		if err := rows.Scan(&enrollment.SeasonID, &role, &state); err != nil {
			return actor, err
		}
		enrollment.Role = authz.SeasonRole(role)
		enrollment.State = authz.EnrollmentState(state)
		actor.Enrollments = append(actor.Enrollments, enrollment)
	}
	return actor, rows.Err()
}

func (p *Store) ResolveAuthSubjectForUser(ctx context.Context, userID string) (string, error) {
	var subject string
	err := p.pool.QueryRow(ctx, `SELECT auth_subject FROM app.user_auth_links WHERE user_id = $1 AND active ORDER BY linked_at, auth_subject LIMIT 1`, userID).Scan(&subject)
	return subject, noRows(err)
}
