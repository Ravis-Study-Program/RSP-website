package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/magedmg/RSP-website/backend/internal/accounts"
	"github.com/magedmg/RSP-website/backend/internal/authz"
	"github.com/magedmg/RSP-website/backend/internal/platform/audit"
	"github.com/magedmg/RSP-website/backend/internal/platform/dbgen"
	"github.com/magedmg/RSP-website/backend/internal/platform/id"
)

// ApplyIdentityEvent applies the operation.
func (p *Postgres) ApplyIdentityEvent(ctx context.Context, event accounts.IdentityEvent) error {
	if event.EventID == "" {
		return errors.New("identity event id is required")
	}
	tx, err := p.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}

	defer tx.Rollback(ctx)
	payloadHash := accounts.IdentityEventHash(event)
	var inserted bool
	err = tx.QueryRow(ctx, `INSERT INTO app.identity_event_receipts(event_id,auth_subject,event_type,security_version,payload_hash) VALUES($1,$2,$3,$4,$5) ON CONFLICT(event_id) DO NOTHING RETURNING true`, event.EventID, event.AuthUserID, event.Type, event.SecurityVersion, payloadHash).Scan(&inserted)
	if errors.Is(err, pgx.ErrNoRows) {
		var existingSubject, existingType, existingHash string
		var existingVersion int64
		if receiptErr := tx.QueryRow(ctx, `SELECT auth_subject,event_type,security_version,payload_hash FROM app.identity_event_receipts WHERE event_id=$1`, event.EventID).Scan(&existingSubject, &existingType, &existingVersion, &existingHash); receiptErr != nil {
			return receiptErr
		}
		if existingSubject != event.AuthUserID || existingType != event.Type || existingVersion != event.SecurityVersion || existingHash != payloadHash {
			return ErrConflict
		}
		return tx.Commit(ctx)
	}
	if err != nil {
		return err
	}

	var userID string
	var currentSecurityVersion int64
	err = tx.QueryRow(ctx, `SELECT l.user_id,u.security_version FROM app.user_auth_links l JOIN app.users u ON u.id=l.user_id WHERE l.auth_subject=$1 FOR UPDATE OF l,u`, event.AuthUserID).Scan(&userID, &currentSecurityVersion)
	if errors.Is(err, pgx.ErrNoRows) && event.Type == "auth_user_created" {
		userID = id.New()
		name := event.Email
		if at := strings.IndexByte(name, '@'); at > 0 {
			name = name[:at]
		}
		if strings.TrimSpace(name) == "" {
			name = "New member"
		}
		slug := "member-" + strings.ReplaceAll(userID, "-", "")[:12]
		if _, err = tx.Exec(ctx, `INSERT INTO app.users(id,slug,display_name,email,account_state,timezone,timezone_configured,revision,security_version) VALUES($1,$2,$3,$4,'active','Australia/Adelaide',false,1,$5)`, userID, slug, name, event.Email, event.SecurityVersion); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `INSERT INTO app.user_auth_links(auth_subject,user_id,provider,provider_account_id,active,revoked_at) VALUES($1,$2,'better_auth',$1,$3,CASE WHEN $3 THEN NULL ELSE $4::timestamptz END)`, event.AuthUserID, userID, event.EmailVerified, event.OccurredAt.UTC()); err != nil {
			return err
		}
		return tx.Commit(ctx)
	}
	if err != nil {
		return noRows(err)
	}
	if event.SecurityVersion < 1 {
		return errors.New("invalid security version")
	}
	if event.SecurityVersion < currentSecurityVersion {
		return tx.Commit(ctx)
	}
	if _, err = tx.Exec(ctx, `UPDATE app.users SET security_version=GREATEST(security_version,$2) WHERE id=$1`, userID, event.SecurityVersion); err != nil {
		return err
	}

	switch event.Type {
	case "auth_user_created":
	case "sessions_revoked":
		err = appendAuditTx(ctx, tx, newAudit(userID, "account.sessions_revoked", "user", userID, map[string]any{"reason": event.Reason}, event.OccurredAt))
	case "email_changed":
		if strings.TrimSpace(event.Email) == "" {
			return errors.New("email_changed requires email")
		}
		_, err = tx.Exec(ctx, `UPDATE app.users SET email=$2,revision=revision+1 WHERE id=$1`, userID, strings.TrimSpace(event.Email))
		if err == nil {
			err = appendAuditTx(ctx, tx, newAudit(userID, "account.email_changed", "user", userID, nil, event.OccurredAt))
		}
	case "account_state_changed":
		if event.AccountState != "active" && event.AccountState != "suspended" {
			return errors.New("account_state_changed requires active or suspended")
		}
		_, err = tx.Exec(ctx, `UPDATE app.users SET account_state=$2,suspended_at=CASE WHEN $2='suspended' THEN $3 ELSE NULL END,revision=revision+1 WHERE id=$1`, userID, event.AccountState, event.OccurredAt.UTC())
		if err == nil {
			auditActor := userID
			if event.ActorUserID != "" {
				auditActor = event.ActorUserID
			}
			err = appendAuditTx(ctx, tx, newAudit(auditActor, "account.state_changed", "user", userID, map[string]any{"accountState": event.AccountState, "reason": event.Reason}, event.OccurredAt))
		}
	case "email_verified":
		_, err = tx.Exec(ctx, `UPDATE app.user_auth_links SET active=true,revoked_at=NULL WHERE auth_subject=$1`, event.AuthUserID)
	case "deletion_requested":
		_, err = tx.Exec(ctx, `UPDATE app.users SET account_state='deletion_pending',deletion_requested_at=$2,deletion_due_at=$3,revision=revision+1 WHERE id=$1`, userID, event.OccurredAt, event.RecoveryDeadline)
		if err == nil {
			err = appendAuditTx(ctx, tx, newAudit(userID, "account.deletion_requested", "user", userID, map[string]any{"recoveryDeadline": event.RecoveryDeadline}, event.OccurredAt))
		}
	case "deletion_cancelled":
		_, err = tx.Exec(ctx, `UPDATE app.users SET account_state='active',deletion_requested_at=NULL,deletion_due_at=NULL,revision=revision+1 WHERE id=$1`, userID)
		if err == nil {
			err = appendAuditTx(ctx, tx, newAudit(userID, "account.deletion_cancelled", "user", userID, nil, event.OccurredAt))
		}
	case "auth_pseudonymized":
		_, err = tx.Exec(ctx, `UPDATE app.users SET slug='deleted-'||id,display_name='Deleted member',email=NULL,discord_id=NULL,avatar_url=NULL,timezone='UTC',timezone_configured=false,account_state='deleted',pseudonymized_at=$2,deleted_at=$2,security_version=GREATEST(security_version,$3),revision=revision+1 WHERE id=$1`, userID, event.OccurredAt, event.SecurityVersion)
		if err == nil {
			_, err = tx.Exec(ctx, `UPDATE app.user_auth_links SET active=false,revoked_at=$2 WHERE auth_subject=$1`, event.AuthUserID, event.OccurredAt)
		}
		if err == nil {
			err = appendAuditTx(ctx, tx, newAudit(userID, "account.pseudonymized", "user", userID, nil, event.OccurredAt))
		}
	case "mfa_configured":
		if err == nil {
			if _, updateErr := tx.Exec(ctx, `UPDATE app.users SET mfa_configured=true WHERE id=$1`, userID); updateErr != nil {
				return updateErr
			}
			rows, rowsErr := tx.Query(ctx, `UPDATE app.global_role_assignments SET state='active',activated_at=$2,revision=revision+1 WHERE user_id=$1 AND state='pending_mfa' RETURNING id,role::text`, userID, event.OccurredAt.UTC())
			if rowsErr != nil {
				return rowsErr
			}
			activated := map[string]string{}
			for rows.Next() {
				var assignmentID, role string
				if scanErr := rows.Scan(&assignmentID, &role); scanErr != nil {
					rows.Close()
					return scanErr
				}
				activated[assignmentID] = role
			}
			if rowsErr := rows.Err(); rowsErr != nil {
				rows.Close()
				return rowsErr
			}
			rows.Close()
			for assignmentID, role := range activated {
				if auditErr := appendAuditTx(ctx, tx, audit.Event{ID: id.New(), ActorID: &userID, Action: "global_role.activated", SubjectType: "global_role_assignment", SubjectID: assignmentID, Data: map[string]any{"role": role, "reason": "mfa_configured"}, OccurredAt: event.OccurredAt.UTC()}); auditErr != nil {
					return auditErr
				}
			}
			seasonRows, rowsErr := tx.Query(ctx, `UPDATE app.enrollments SET assignment_state='active',activated_at=$2,revision=revision+1 WHERE user_id=$1 AND role='coordinator' AND state='active' AND assignment_state='pending_mfa' AND deleted_at IS NULL RETURNING id,season_id`, userID, event.OccurredAt.UTC())
			if rowsErr != nil {
				return rowsErr
			}
			for seasonRows.Next() {
				var enrollmentID, seasonID string
				if scanErr := seasonRows.Scan(&enrollmentID, &seasonID); scanErr != nil {
					seasonRows.Close()
					return scanErr
				}
				if auditErr := appendAuditTx(ctx, tx, newAudit(userID, "coordinator_role.activated", "enrollment", enrollmentID, map[string]any{"seasonId": seasonID, "reason": "mfa_configured"}, event.OccurredAt)); auditErr != nil {
					seasonRows.Close()
					return auditErr
				}
			}
			if rowsErr := seasonRows.Err(); rowsErr != nil {
				seasonRows.Close()
				return rowsErr
			}
			seasonRows.Close()
			if _, updateErr := tx.Exec(ctx, `UPDATE app.enrollments SET close_assignment_state='active',close_activated_at=$2 WHERE user_id=$1 AND role='coordinator' AND state='completed' AND completed_by_close_id IS NOT NULL AND close_assignment_state='pending_mfa' AND deleted_at IS NULL`, userID, event.OccurredAt.UTC()); updateErr != nil {
				return updateErr
			}
		}
	case "mfa_disabled":
		if _, updateErr := tx.Exec(ctx, `UPDATE app.users SET mfa_configured=false WHERE id=$1`, userID); updateErr != nil {
			return updateErr
		}
		rows, rowsErr := tx.Query(ctx, `UPDATE app.global_role_assignments SET state='pending_mfa',activated_at=NULL,revoked_at=NULL,revision=revision+1 WHERE user_id=$1 AND state='active' RETURNING id,role::text`, userID)
		if rowsErr != nil {
			return rowsErr
		}
		for rows.Next() {
			var assignmentID, role string
			if scanErr := rows.Scan(&assignmentID, &role); scanErr != nil {
				rows.Close()
				return scanErr
			}
			if auditErr := appendAuditTx(ctx, tx, newAudit(userID, "global_role.pending_mfa", "global_role_assignment", assignmentID, map[string]any{"role": role, "reason": "mfa_disabled"}, event.OccurredAt)); auditErr != nil {
				rows.Close()
				return auditErr
			}
		}
		rows.Close()
		seasonRows, rowsErr := tx.Query(ctx, `UPDATE app.enrollments SET assignment_state='pending_mfa',activated_at=NULL,revision=revision+1 WHERE user_id=$1 AND role='coordinator' AND state='active' AND assignment_state='active' AND deleted_at IS NULL RETURNING id,season_id`, userID)
		if rowsErr != nil {
			return rowsErr
		}
		for seasonRows.Next() {
			var enrollmentID, seasonID string
			if scanErr := seasonRows.Scan(&enrollmentID, &seasonID); scanErr != nil {
				seasonRows.Close()
				return scanErr
			}
			if auditErr := appendAuditTx(ctx, tx, newAudit(userID, "coordinator_role.pending_mfa", "enrollment", enrollmentID, map[string]any{"seasonId": seasonID, "reason": "mfa_disabled"}, event.OccurredAt)); auditErr != nil {
				seasonRows.Close()
				return auditErr
			}
		}
		seasonRows.Close()
		if _, updateErr := tx.Exec(ctx, `UPDATE app.enrollments SET close_assignment_state='pending_mfa',close_activated_at=NULL WHERE user_id=$1 AND role='coordinator' AND state='completed' AND completed_by_close_id IS NOT NULL AND close_assignment_state='active' AND deleted_at IS NULL`, userID); updateErr != nil {
			return updateErr
		}
	default:
		return errors.New("unsupported identity event")
	}
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (p *Postgres) ResolveAuthSubject(ctx context.Context, sub string) (authz.Actor, error) {
	var a authz.Actor
	var state string
	err := p.Pool.QueryRow(ctx, `SELECT u.id,u.account_state::text,u.security_version FROM app.user_auth_links l JOIN app.users u ON u.id=l.user_id WHERE l.auth_subject=$1 AND l.active AND u.deleted_at IS NULL`, sub).Scan(&a.UserID, &state, &a.SecurityVersion)
	if err != nil {
		return a, noRows(err)
	}

	a.EmailVerified = true
	a.AccountState = authz.AccountState(state)
	a.GlobalRoles = map[authz.GlobalRole]bool{}
	rows, err := p.Pool.Query(ctx, `SELECT role::text FROM app.global_role_assignments WHERE user_id=$1 AND state='active'`, a.UserID)
	if err != nil {
		return a, err
	}

	for rows.Next() {
		var role string
		if err := rows.Scan(&role); err != nil {
			rows.Close()
			return a, err
		}

		a.GlobalRoles[authz.GlobalRole(role)] = true
	}
	rows.Close()
	rows, err = p.Pool.Query(ctx, `SELECT season_id,role::text,state::text FROM app.enrollments WHERE user_id=$1 AND deleted_at IS NULL AND (role<>'coordinator' OR state='completed' OR assignment_state='active')`, a.UserID)
	if err != nil {
		return a, err
	}

	defer rows.Close()
	for rows.Next() {
		var e authz.Enrollment
		var role, state string
		if err := rows.Scan(&e.SeasonID, &role, &state); err != nil {
			return a, err
		}

		e.Role = authz.SeasonRole(role)
		e.State = authz.EnrollmentState(state)
		a.Enrollments = append(a.Enrollments, e)
	}
	return a, rows.Err()
}

// ResolveAuthSubjectForUser performs the operation.
func (p *Postgres) ResolveAuthSubjectForUser(ctx context.Context, userID string) (string, error) {
	var subject string
	err := p.Pool.QueryRow(ctx, `SELECT auth_subject FROM app.user_auth_links WHERE user_id=$1 AND active ORDER BY linked_at,auth_subject LIMIT 1`, userID).Scan(&subject)
	return subject, noRows(err)
}

// GrantGlobalRole performs the operation.
func (p *Postgres) GrantGlobalRole(ctx context.Context, userID, role string, activate bool, reason, actorID string, at time.Time) (accounts.GlobalRoleAssignment, error) {
	tx, err := p.Pool.Begin(ctx)
	if err != nil {
		return accounts.GlobalRoleAssignment{}, err
	}

	defer tx.Rollback(ctx)
	state := "pending_mfa"
	var activatedAt *time.Time
	if activate {
		state = "active"
		activated := at.UTC()
		activatedAt = &activated
	}
	v := accounts.GlobalRoleAssignment{ID: id.New(), UserID: userID, Role: role, State: state, Revision: 1}
	err = tx.QueryRow(ctx, `INSERT INTO app.global_role_assignments(id,user_id,role,state,granted_by_user_id,granted_at,activated_at,revision) VALUES($1,$2,$3,$4,$5,$6,$7,1) ON CONFLICT(user_id,role) DO UPDATE SET state=$4,granted_by_user_id=$5,granted_at=$6,activated_at=$7,revoked_at=NULL,revision=app.global_role_assignments.revision+1 WHERE app.global_role_assignments.state='revoked' RETURNING id,state::text,revision`, v.ID, userID, role, state, actorID, at.UTC(), activatedAt).Scan(&v.ID, &v.State, &v.Revision)
	if err != nil {
		return v, mapPostgresError(err)
	}
	if err := appendAuditTx(ctx, tx, newAudit(actorID, "global_role.granted_"+state, "global_role_assignment", v.ID, map[string]any{"userId": userID, "role": role, "reason": reason}, at)); err != nil {
		return v, err
	}
	return v, tx.Commit(ctx)
}

// ListGlobalRoles lists matching values.
func (p *Postgres) ListGlobalRoles(ctx context.Context, userID string) ([]accounts.GlobalRoleAssignment, error) {
	var exists bool
	if err := p.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM app.users WHERE id=$1 AND deleted_at IS NULL)`, userID).Scan(&exists); err != nil {
		return nil, err
	}
	if !exists {
		return nil, ErrNotFound
	}
	rows, err := p.Pool.Query(ctx, `SELECT id,user_id,role::text,state::text,revision FROM app.global_role_assignments WHERE user_id=$1 AND state<>'revoked' ORDER BY role`, userID)
	if err != nil {
		return nil, err
	}

	defer rows.Close()
	items := []accounts.GlobalRoleAssignment{}
	for rows.Next() {
		var assignment accounts.GlobalRoleAssignment
		if err := rows.Scan(&assignment.ID, &assignment.UserID, &assignment.Role, &assignment.State, &assignment.Revision); err != nil {
			return nil, err
		}

		items = append(items, assignment)
	}
	return items, rows.Err()
}

// RevokeGlobalRole performs the operation.
func (p *Postgres) RevokeGlobalRole(ctx context.Context, userID, role string, revision int64, reason, actorID string, at time.Time) (accounts.GlobalRoleAssignment, error) {
	tx, err := p.Pool.Begin(ctx)
	if err != nil {
		return accounts.GlobalRoleAssignment{}, err
	}

	defer tx.Rollback(ctx)
	v := accounts.GlobalRoleAssignment{UserID: userID, Role: role}
	err = tx.QueryRow(ctx, `UPDATE app.global_role_assignments SET state='revoked',revoked_at=$4,activated_at=NULL,revision=revision+1 WHERE user_id=$1 AND role=$2 AND revision=$3 AND state<>'revoked' RETURNING id,state::text,revision`, userID, role, revision, at.UTC()).Scan(&v.ID, &v.State, &v.Revision)
	if errors.Is(err, pgx.ErrNoRows) {
		var current int64
		lookupErr := tx.QueryRow(ctx, `SELECT revision FROM app.global_role_assignments WHERE user_id=$1 AND role=$2 AND state<>'revoked'`, userID, role).Scan(&current)
		if lookupErr == nil {
			return v, ErrConflict
		}
		return v, noRows(lookupErr)
	}
	if err != nil {
		return v, err
	}
	if err := appendAuditTx(ctx, tx, newAudit(actorID, "global_role.revoked", "global_role_assignment", v.ID, map[string]any{"userId": userID, "role": role, "reason": reason}, at)); err != nil {
		return v, err
	}
	return v, tx.Commit(ctx)
}

const userColumns = `u.id,u.slug,u.display_name,u.avatar_url,u.timezone,u.timezone_configured,COALESCE(u.email,''),u.account_state::text,COALESCE(array_agg(g.role::text) FILTER (WHERE g.state='active'),'{}')::text[],u.is_test,u.leetcode_premium_opt_in,u.revision,
COALESCE((SELECT jsonb_agg(jsonb_build_object('seasonId',e.season_id,'seasonSlug',s.slug,'role',e.role::text,'state',e.state::text) ORDER BY s.slug,e.id) FROM app.enrollments e JOIN app.seasons s ON s.id=e.season_id WHERE e.user_id=u.id AND e.deleted_at IS NULL AND (e.state='active' OR (e.role='student' AND e.state='completed'))),'[]'::jsonb),
(SELECT count(*) FROM app.problem_attempts a WHERE a.user_id=u.id AND a.deleted_at IS NULL),
(SELECT count(*) FROM app.mock_interviews mi WHERE (mi.interviewer_user_id=u.id OR mi.interviewee_user_id=u.id) AND mi.deleted_at IS NULL)`

func scanUser(row pgx.Row) (accounts.User, error) {
	var v accounts.User
	var seasonRoles []byte
	if err := row.Scan(&v.ID, &v.Slug, &v.Name, &v.AvatarURL, &v.Timezone, &v.TimezoneConfigured, &v.Email, &v.AccountState, &v.GlobalRoles, &v.IsTest, &v.PremiumOptIn, &v.Revision, &seasonRoles, &v.AttemptCount, &v.MockInterviewCount); err != nil {
		return v, noRows(err)
	}
	if err := json.Unmarshal(seasonRoles, &v.SeasonRoles); err != nil {
		return v, err
	}
	return v, nil
}

// GetUser retrieves a value.
func (p *Postgres) GetUser(ctx context.Context, id string) (accounts.User, error) {
	row, err := p.Queries.GetUserPrivateByID(ctx, dbgen.GetUserPrivateByIDParams{ID: id})
	if err != nil {
		return accounts.User{}, noRows(err)
	}

	v := accounts.User{ID: row.ID, Slug: row.Slug, Name: row.DisplayName, AvatarURL: row.AvatarUrl, Timezone: row.Timezone, TimezoneConfigured: row.TimezoneConfigured, AccountState: string(row.AccountState), GlobalRoles: []string{}, IsTest: row.IsTest, PremiumOptIn: row.LeetcodePremiumOptIn, Revision: row.Revision}
	if row.Email != nil {
		v.Email = *row.Email
	}
	roles, err := p.Queries.ListGlobalRolesForUser(ctx, dbgen.ListGlobalRolesForUserParams{UserID: v.ID})
	if err != nil {
		return accounts.User{}, err
	}

	for _, role := range roles {
		if role.State == dbgen.AppAssignmentStateActive {
			v.GlobalRoles = append(v.GlobalRoles, string(role.Role))
		}
	}
	var seasonRoles []byte
	err = p.Pool.QueryRow(ctx, `SELECT COALESCE(jsonb_agg(jsonb_build_object('seasonId',e.season_id,'seasonSlug',s.slug,'role',e.role::text,'state',e.state::text) ORDER BY s.slug,e.id),'[]'::jsonb),(SELECT count(*) FROM app.problem_attempts a WHERE a.user_id=$1 AND a.deleted_at IS NULL),(SELECT count(*) FROM app.mock_interviews mi WHERE (mi.interviewer_user_id=$1 OR mi.interviewee_user_id=$1) AND mi.deleted_at IS NULL) FROM app.enrollments e JOIN app.seasons s ON s.id=e.season_id WHERE e.user_id=$1 AND e.deleted_at IS NULL AND (e.state='active' OR (e.role='student' AND e.state='completed'))`, v.ID).Scan(&seasonRoles, &v.AttemptCount, &v.MockInterviewCount)
	if err != nil {
		return accounts.User{}, err
	}
	if err := json.Unmarshal(seasonRoles, &v.SeasonRoles); err != nil {
		return accounts.User{}, err
	}
	return v, nil
}

// SuggestUserSlug performs the operation.
func (p *Postgres) SuggestUserSlug(ctx context.Context) (string, error) {
	for attempt := 0; attempt < 32; attempt++ {
		candidate := "member-" + strings.ReplaceAll(id.New(), "-", "")[:12]
		var available bool
		if err := p.Pool.QueryRow(ctx, `SELECT NOT EXISTS(SELECT 1 FROM app.users WHERE lower(slug)=lower($1))`, candidate).Scan(&available); err != nil {
			return "", err
		}
		if available {
			return candidate, nil
		}
	}
	return "", ErrConflict
}

// ListUsers lists matching values.
func (p *Postgres) ListUsers(ctx context.Context, boundary string, limit int, direction, search, seasonRole, globalRole string) ([]accounts.User, bool, int64, error) {
	search = strings.TrimSpace(search)
	const filters = `u.account_state='active' AND NOT u.is_test AND u.deleted_at IS NULL
		AND ($3='' OR u.display_name ILIKE '%'||$3||'%' OR u.slug ILIKE '%'||$3||'%')
		AND EXISTS(SELECT 1 FROM app.enrollments e WHERE e.user_id=u.id AND e.deleted_at IS NULL AND (e.state='active' OR (e.role='student' AND e.state='completed')))
		AND ($4='' OR EXISTS(SELECT 1 FROM app.enrollments e WHERE e.user_id=u.id AND e.deleted_at IS NULL AND (e.state='active' OR (e.role='student' AND e.state='completed')) AND e.role::text=$4))
		AND ($5='' OR EXISTS(SELECT 1 FROM app.global_role_assignments role_filter WHERE role_filter.user_id=u.id AND role_filter.state='active' AND role_filter.role::text=$5))`
	countFilters := strings.NewReplacer("$3", "$1", "$4", "$2", "$5", "$3").Replace(filters)
	var total int64
	if err := p.Pool.QueryRow(ctx, `SELECT count(*) FROM app.users u WHERE `+countFilters, search, seasonRole, globalRole).Scan(&total); err != nil {
		return nil, false, 0, err
	}

	comparison, order := `u.id>NULLIF($1,'')::uuid`, `ASC`
	if direction == "backward" {
		comparison, order = `u.id<NULLIF($1,'')::uuid`, `DESC`
	}
	if boundary == "" {
		comparison = `$1::text IS NOT NULL`
	}
	rows, err := p.Pool.Query(ctx, `SELECT `+userColumns+` FROM app.users u LEFT JOIN app.global_role_assignments g ON g.user_id=u.id WHERE `+comparison+` AND `+filters+` GROUP BY u.id ORDER BY u.id `+order+` LIMIT $2`, boundary, limit+1, search, seasonRole, globalRole)
	if err != nil {
		return nil, false, 0, err
	}

	defer rows.Close()
	out := []accounts.User{}
	for rows.Next() {
		v, err := scanUser(rows)
		if err != nil {
			return nil, false, 0, err
		}

		out = append(out, v)
	}
	more := len(out) > limit
	if more {
		out = out[:limit]
	}
	if direction == "backward" {
		sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	}
	return out, more, total, rows.Err()
}

// ListAdminUsers lists matching values.
func (p *Postgres) ListAdminUsers(ctx context.Context, boundary string, limit int, direction, search, accountState, globalRole string) ([]accounts.User, bool, int64, error) {
	search = strings.TrimSpace(search)
	const filters = `u.account_state<>'deleted' AND u.deleted_at IS NULL
		AND ($3='' OR u.display_name ILIKE '%'||$3||'%' OR u.slug ILIKE '%'||$3||'%' OR COALESCE(u.email,'') ILIKE '%'||$3||'%')
		AND ($4='' OR u.account_state::text=$4)
		AND ($5='' OR EXISTS(SELECT 1 FROM app.global_role_assignments role_filter WHERE role_filter.user_id=u.id AND role_filter.state<>'revoked' AND role_filter.role::text=$5))`
	countFilters := strings.NewReplacer("$3", "$1", "$4", "$2", "$5", "$3").Replace(filters)
	var total int64
	if err := p.Pool.QueryRow(ctx, `SELECT count(*) FROM app.users u WHERE `+countFilters, search, accountState, globalRole).Scan(&total); err != nil {
		return nil, false, 0, err
	}

	comparison, order := `u.id>NULLIF($1,'')::uuid`, `ASC`
	if direction == "backward" {
		comparison, order = `u.id<NULLIF($1,'')::uuid`, `DESC`
	}
	if boundary == "" {
		comparison = `$1::text IS NOT NULL`
	}
	rows, err := p.Pool.Query(ctx, `SELECT `+userColumns+` FROM app.users u LEFT JOIN app.global_role_assignments g ON g.user_id=u.id WHERE `+comparison+` AND `+filters+` GROUP BY u.id ORDER BY u.id `+order+` LIMIT $2`, boundary, limit+1, search, accountState, globalRole)
	if err != nil {
		return nil, false, 0, err
	}

	defer rows.Close()
	items := []accounts.User{}
	for rows.Next() {
		user, err := scanUser(rows)
		if err != nil {
			return nil, false, 0, err
		}

		items = append(items, user)
	}
	more := len(items) > limit
	if more {
		items = items[:limit]
	}
	if direction == "backward" {
		sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	}
	return items, more, total, rows.Err()
}

// ListEnrollmentCandidates lists matching values.
func (p *Postgres) ListEnrollmentCandidates(ctx context.Context, seasonID, query, boundary string, limit int, direction string) ([]accounts.EnrollmentCandidate, bool, int64, error) {
	query = strings.TrimSpace(query)
	const eligible = `u.account_state='active' AND NOT u.is_test AND u.deleted_at IS NULL
AND EXISTS(SELECT 1 FROM app.user_auth_links l WHERE l.user_id=u.id AND l.active)
AND NOT EXISTS(SELECT 1 FROM app.enrollments e WHERE e.user_id=u.id AND e.season_id=$1)`
	var total int64
	if err := p.Pool.QueryRow(ctx, `SELECT count(*) FROM app.users u WHERE `+eligible+` AND ($2='' OR u.display_name ILIKE '%'||$2||'%' OR u.slug ILIKE '%'||$2||'%')`, seasonID, query).Scan(&total); err != nil {
		return nil, false, 0, err
	}

	comparison, order := `u.id>$3`, `ASC`
	if direction == "backward" {
		comparison, order = `u.id<$3`, `DESC`
	}
	if boundary == "" {
		comparison = `$3::text IS NOT NULL`
	}
	rows, err := p.Pool.Query(ctx, `SELECT u.id,u.slug,u.display_name,u.avatar_url,u.revision FROM app.users u WHERE `+eligible+` AND ($2='' OR u.display_name ILIKE '%'||$2||'%' OR u.slug ILIKE '%'||$2||'%') AND `+comparison+` ORDER BY u.id `+order+` LIMIT $4`, seasonID, query, boundary, limit+1)
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
	more := len(items) > limit
	if more {
		items = items[:limit]
	}
	if direction == "backward" {
		sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	}
	return items, more, total, rows.Err()
}

// UpdateUser updates a value.
func (p *Postgres) UpdateUser(ctx context.Context, id string, revision int64, fn func(*accounts.User) error, actorID string, at time.Time) (accounts.User, error) {
	tx, err := p.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return accounts.User{}, err
	}

	defer tx.Rollback(ctx)
	var v accounts.User
	err = tx.QueryRow(ctx, `SELECT id,slug,display_name,avatar_url,timezone,timezone_configured,COALESCE(email,''),account_state::text,is_test,leetcode_premium_opt_in,revision FROM app.users WHERE id=$1 AND deleted_at IS NULL FOR UPDATE`, id).Scan(&v.ID, &v.Slug, &v.Name, &v.AvatarURL, &v.Timezone, &v.TimezoneConfigured, &v.Email, &v.AccountState, &v.IsTest, &v.PremiumOptIn, &v.Revision)
	if err != nil {
		return v, noRows(err)
	}
	if v.Revision != revision {
		return v, ErrConflict
	}
	if err := fn(&v); err != nil {
		return v, err
	}

	err = tx.QueryRow(ctx, `UPDATE app.users SET slug=$2,display_name=$3,avatar_url=$4,timezone=$5,timezone_configured=$6,revision=revision+1 WHERE id=$1 AND revision=$7 RETURNING revision`, id, v.Slug, v.Name, v.AvatarURL, v.Timezone, v.TimezoneConfigured, revision).Scan(&v.Revision)
	if err != nil {
		return v, mapPostgresError(err)
	}
	if err := appendAuditTx(ctx, tx, newAudit(actorID, "user.updated", "user", id, nil, at)); err != nil {
		return v, err
	}
	if err := tx.Commit(ctx); err != nil {
		return v, err
	}
	return v, nil
}

// GetPracticeSettings retrieves a value.
