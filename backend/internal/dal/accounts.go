package dal

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/magedmg/RSP-website/backend/internal/accounts"
	"github.com/magedmg/RSP-website/backend/internal/authz"
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
	err = tx.QueryRow(ctx, `SELECT l.user_id,u.security_version
		FROM app.user_auth_links l
		JOIN app.users u ON u.id=l.user_id
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
	if _, err := tx.Exec(ctx, `UPDATE app.users SET security_version=GREATEST(security_version, $1) WHERE id = $2`, event.SecurityVersion, link.UserID); err != nil {
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
	if _, err := tx.Exec(ctx, `INSERT INTO app.users(id,slug,display_name,email,account_state,timezone,timezone_configured,revision,security_version)
 VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)`, userID, slug, name, event.Email, "active", "Australia/Adelaide", false, 1, event.SecurityVersion); err != nil {
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
		if _, err := tx.Exec(ctx, `UPDATE app.users SET email=$1,revision=revision + 1 WHERE id = $2`, email, userID); err != nil {
			return err
		}
		return appendAuditTx(ctx, tx, newAudit(userID, "account.email_changed", "user", userID, nil, event.OccurredAt))
	case "account_state_changed":
		if event.AccountState != "active" && event.AccountState != "suspended" {
			return errors.New("account_state_changed requires active or suspended")
		}
		var suspendedAt *time.Time
		if event.AccountState == "suspended" {
			value := event.OccurredAt.UTC()
			suspendedAt = &value
		}
		if _, err := tx.Exec(ctx, `UPDATE app.users SET account_state=$1,suspended_at=$2,revision=revision + 1 WHERE id = $3`, event.AccountState, suspendedAt, userID); err != nil {
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
		if _, err := tx.Exec(ctx, `UPDATE app.users SET account_state=$1,deletion_requested_at=$2,deletion_due_at=$3,revision=revision + 1 WHERE id = $4`, "deletion_pending", event.OccurredAt, event.RecoveryDeadline, userID); err != nil {
			return err
		}
		return appendAuditTx(ctx, tx, newAudit(userID, "account.deletion_requested", "user", userID, map[string]any{"recoveryDeadline": event.RecoveryDeadline}, event.OccurredAt))
	case "deletion_cancelled":
		if _, err := tx.Exec(ctx, `UPDATE app.users SET account_state=$1,deletion_requested_at=$2,deletion_due_at=$3,revision=revision + 1 WHERE id = $4`, "active", nil, nil, userID); err != nil {
			return err
		}
		return appendAuditTx(ctx, tx, newAudit(userID, "account.deletion_cancelled", "user", userID, nil, event.OccurredAt))
	case "auth_pseudonymized":
		if _, err := tx.Exec(ctx, `UPDATE app.users SET slug='deleted-' || id,display_name=$1,email=$2,discord_id=$3,avatar_url=$4,timezone=$5,timezone_configured=$6,account_state=$7,pseudonymized_at=$8,deleted_at=$9,security_version=GREATEST(security_version, $10),revision=revision + 1 WHERE id = $11`, "Deleted member", nil, nil, nil, "UTC", false, "deleted", event.OccurredAt, event.OccurredAt, event.SecurityVersion, userID); err != nil {
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
	if _, err := tx.Exec(ctx, `UPDATE app.users SET mfa_configured=$1 WHERE id = $2`, true, userID); err != nil {
		return err
	}
	globalRows, err := tx.Query(ctx, `UPDATE app.global_role_assignments
		SET state='active',activated_at=$1,revision=revision+1
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
		SET assignment_state='active',activated_at=$1,revision=revision+1
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
	if _, err := tx.Exec(ctx, `UPDATE app.users SET mfa_configured=$1 WHERE id = $2`, false, userID); err != nil {
		return err
	}
	globalRows, err := tx.Query(ctx, `UPDATE app.global_role_assignments
		SET state='pending_mfa',activated_at=NULL,revoked_at=NULL,revision=revision+1
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
		SET assignment_state='pending_mfa',activated_at=NULL,revision=revision+1
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

func (p *Store) ResolveAuthSubject(ctx context.Context, subject string) (authz.Actor, error) {
	var actor authz.Actor
	var accountState string
	err := p.pool.QueryRow(ctx, `SELECT u.id,u.account_state::text,u.security_version
		FROM app.user_auth_links l
		JOIN app.users u ON u.id=l.user_id
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

	rows, err := p.pool.Query(ctx, `SELECT season_id, role::text, state::text FROM app.enrollments WHERE user_id = $1 AND deleted_at IS NULL AND (role <> 'coordinator' OR state = 'completed' OR assignment_state = 'active')`, actor.UserID)
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
		return assignment, mapStoreError(err)
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

const userColumns = `u.id,u.slug,u.display_name,u.avatar_url,u.timezone,u.timezone_configured,
	COALESCE(u.email,''),u.account_state::text,
	COALESCE(to_jsonb(array_agg(g.role::text) FILTER (WHERE g.state='active')),'[]'::jsonb),u.is_test,u.revision,
	COALESCE((SELECT jsonb_agg(jsonb_build_object('seasonId',e.season_id,'seasonSlug',s.slug,'role',e.role::text,'state',e.state::text) ORDER BY s.slug,e.id)
		FROM app.enrollments e JOIN app.seasons s ON s.id=e.season_id
		WHERE e.user_id=u.id AND e.deleted_at IS NULL AND (e.state='active' OR (e.role='student' AND e.state='completed'))),'[]'::jsonb),
	(SELECT count(*) FROM app.problem_attempts a WHERE a.user_id=u.id AND a.deleted_at IS NULL),
	(SELECT count(*) FROM app.mock_interviews mi WHERE (mi.interviewer_user_id=u.id OR mi.interviewee_user_id=u.id) AND mi.deleted_at IS NULL)`

type rowScanner interface {
	Scan(dest ...any) error
}

func scanUser(row rowScanner) (accounts.User, error) {
	var user accounts.User
	var globalRoles, seasonRoles []byte
	if err := row.Scan(&user.ID, &user.Slug, &user.Name, &user.AvatarURL, &user.Timezone, &user.TimezoneConfigured, &user.Email, &user.AccountState, &globalRoles, &user.IsTest, &user.Revision, &seasonRoles, &user.AttemptCount, &user.MockInterviewCount); err != nil {
		return user, noRows(err)
	}
	if err := json.Unmarshal(globalRoles, &user.GlobalRoles); err != nil {
		return user, err
	}
	if err := json.Unmarshal(seasonRoles, &user.SeasonRoles); err != nil {
		return user, err
	}
	return user, nil
}

func (p *Store) GetUser(ctx context.Context, userID string) (accounts.User, error) {
	query := `SELECT ` + userColumns + `
		FROM app.users u
		LEFT JOIN app.global_role_assignments g ON g.user_id=u.id
		WHERE u.id=$1 AND u.deleted_at IS NULL
		GROUP BY u.id`
	return scanUser(p.pool.QueryRow(ctx, query, userID))
}

func (p *Store) SuggestUserSlug(ctx context.Context) (string, error) {
	for attempt := 0; attempt < 32; attempt++ {
		candidate := "member-" + strings.ReplaceAll(id.New(), "-", "")[:12]
		var count int64
		if err := p.pool.QueryRow(ctx, `SELECT count(*) FROM app.users WHERE lower(slug) = lower($1)`, candidate).Scan(&count); err != nil {
			return "", err
		}
		if count == 0 {
			return candidate, nil
		}
	}
	return "", ErrConflict
}

func (p *Store) ListUsers(ctx context.Context, boundary string, limit int, direction, search, seasonRole, globalRole string) ([]accounts.User, bool, int64, error) {
	const filters = `u.account_state = 'active' AND NOT u.is_test AND u.deleted_at IS NULL
		AND (@search = '' OR u.display_name ILIKE '%' || @search || '%' OR u.slug ILIKE '%' || @search || '%')
		AND EXISTS (
			SELECT 1 FROM app.enrollments e
			WHERE e.user_id = u.id AND e.deleted_at IS NULL
			AND (e.state = 'active' OR (e.role = 'student' AND e.state = 'completed'))
		)
		AND (@seasonRole = '' OR EXISTS (
			SELECT 1 FROM app.enrollments e
			WHERE e.user_id = u.id AND e.deleted_at IS NULL
			AND (e.state = 'active' OR (e.role = 'student' AND e.state = 'completed'))
			AND e.role::text = @seasonRole
		))
		AND (@globalRole = '' OR EXISTS (
			SELECT 1 FROM app.global_role_assignments role_filter
			WHERE role_filter.user_id = u.id AND role_filter.state = 'active'
			AND role_filter.role::text = @globalRole
		))`
	args := pgx.NamedArgs{"search": strings.TrimSpace(search), "seasonRole": seasonRole, "globalRole": globalRole}
	var total int64
	if err := p.pool.QueryRow(ctx, `SELECT count(*) FROM app.users u WHERE `+filters, args).Scan(&total); err != nil {
		return nil, false, 0, err
	}
	items, more, err := p.listUsers(ctx, boundary, limit, direction, filters, args)
	return items, more, total, err
}

func (p *Store) ListAdminUsers(ctx context.Context, boundary string, limit int, direction, search, accountState, globalRole string) ([]accounts.User, bool, int64, error) {
	const filters = `u.account_state <> 'deleted' AND u.deleted_at IS NULL
		AND (@search = '' OR u.display_name ILIKE '%' || @search || '%'
			OR u.slug ILIKE '%' || @search || '%' OR COALESCE(u.email, '') ILIKE '%' || @search || '%')
		AND (@accountState = '' OR u.account_state::text = @accountState)
		AND (@globalRole = '' OR EXISTS (
			SELECT 1 FROM app.global_role_assignments role_filter
			WHERE role_filter.user_id = u.id AND role_filter.state <> 'revoked'
			AND role_filter.role::text = @globalRole
		))`
	args := pgx.NamedArgs{"search": strings.TrimSpace(search), "accountState": accountState, "globalRole": globalRole}
	var total int64
	if err := p.pool.QueryRow(ctx, `SELECT count(*) FROM app.users u WHERE `+filters, args).Scan(&total); err != nil {
		return nil, false, 0, err
	}
	items, more, err := p.listUsers(ctx, boundary, limit, direction, filters, args)
	return items, more, total, err
}

func (p *Store) listUsers(ctx context.Context, boundary string, limit int, direction, filters string, args pgx.NamedArgs) ([]accounts.User, bool, error) {
	comparison, order := `u.id > NULLIF(@boundary, '')::uuid`, `ASC`
	if direction == "backward" {
		comparison, order = `u.id < NULLIF(@boundary, '')::uuid`, `DESC`
	}
	if boundary == "" {
		comparison = `TRUE`
	}
	query := `SELECT ` + userColumns + ` FROM app.users u
		LEFT JOIN app.global_role_assignments g ON g.user_id=u.id
		WHERE ` + comparison + ` AND ` + filters + `
		GROUP BY u.id
		ORDER BY u.id ` + order + ` LIMIT @limit`
	args["boundary"] = boundary
	args["limit"] = limit + 1
	rows, err := p.pool.Query(ctx, query, args)
	if err != nil {
		return nil, false, err
	}
	defer rows.Close()
	items := []accounts.User{}
	for rows.Next() {
		user, err := scanUser(rows)
		if err != nil {
			return nil, false, err
		}
		items = append(items, user)
	}
	if err := rows.Err(); err != nil {
		return nil, false, err
	}
	items, more := finishStorePage(items, limit, direction)
	return items, more, nil
}

func (p *Store) ListEnrollmentCandidates(ctx context.Context, seasonID, query, boundary string, limit int, direction string) ([]accounts.EnrollmentCandidate, bool, int64, error) {
	const filters = `u.account_state = 'active' AND NOT u.is_test AND u.deleted_at IS NULL
		AND EXISTS (SELECT 1 FROM app.user_auth_links l WHERE l.user_id = u.id AND l.active)
		AND NOT EXISTS (SELECT 1 FROM app.enrollments e WHERE e.user_id = u.id AND e.season_id = @seasonID)
		AND (@search = '' OR u.display_name ILIKE '%' || @search || '%' OR u.slug ILIKE '%' || @search || '%')`
	args := pgx.NamedArgs{"seasonID": seasonID, "search": strings.TrimSpace(query)}
	var total int64
	if err := p.pool.QueryRow(ctx, `SELECT count(*) FROM app.users u WHERE `+filters, args).Scan(&total); err != nil {
		return nil, false, 0, err
	}

	comparison, order := `u.id > NULLIF(@boundary, '')::uuid`, `ASC`
	if direction == "backward" {
		comparison, order = `u.id < NULLIF(@boundary, '')::uuid`, `DESC`
	}
	if boundary == "" {
		comparison = `TRUE`
	}
	args["boundary"] = boundary
	args["limit"] = limit + 1
	rows, err := p.pool.Query(ctx, `SELECT u.id,u.slug,u.display_name,u.avatar_url,u.revision
		FROM app.users u
		WHERE `+filters+` AND `+comparison+`
		ORDER BY u.id `+order+` LIMIT @limit`, args)
	if err != nil {
		return nil, false, 0, err
	}
	defer rows.Close()
	items := []accounts.EnrollmentCandidate{}
	for rows.Next() {
		var candidate accounts.EnrollmentCandidate
		if err := rows.Scan(&candidate.ID, &candidate.Slug, &candidate.Name, &candidate.AvatarURL, &candidate.Revision); err != nil {
			return nil, false, 0, err
		}
		items = append(items, candidate)
	}
	if err := rows.Err(); err != nil {
		return nil, false, 0, err
	}
	items, more := finishStorePage(items, limit, direction)
	return items, more, total, nil
}

func (p *Store) UpdateUser(ctx context.Context, userID string, revision int64, update func(*accounts.User) error, actorID string, at time.Time) (accounts.User, error) {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return accounts.User{}, err
	}
	defer tx.Rollback(context.Background())

	var user accounts.User
	err = tx.QueryRow(ctx, `SELECT id,slug,display_name,avatar_url,timezone,timezone_configured,
		COALESCE(email,''),account_state::text,is_test,revision
		FROM app.users WHERE id=$1 AND deleted_at IS NULL FOR UPDATE`, userID).Scan(&user.ID, &user.Slug, &user.Name, &user.AvatarURL, &user.Timezone, &user.TimezoneConfigured, &user.Email, &user.AccountState, &user.IsTest, &user.Revision)
	if err != nil {
		return user, noRows(err)
	}
	if user.Revision != revision {
		return user, ErrConflict
	}
	if err := update(&user); err != nil {
		return user, err
	}
	result, err := tx.Exec(ctx, `UPDATE app.users SET slug=$1,display_name=$2,avatar_url=$3,timezone=$4,timezone_configured=$5,revision=revision + 1 WHERE id = $6 AND revision = $7`, user.Slug, user.Name, user.AvatarURL, user.Timezone, user.TimezoneConfigured, userID, revision)
	if err != nil {
		return user, mapStoreError(err)
	}
	if result.RowsAffected() == 0 {
		return user, ErrConflict
	}
	user.Revision++
	if err := appendAuditTx(ctx, tx, newAudit(actorID, "user.updated", "user", userID, nil, at)); err != nil {
		return user, err
	}
	if err := tx.Commit(ctx); err != nil {
		return user, err
	}
	return user, nil
}
