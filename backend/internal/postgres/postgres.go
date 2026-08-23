package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/magedmg/RSP-website/backend/internal/accounts"
	"github.com/magedmg/RSP-website/backend/internal/authz"
	"github.com/magedmg/RSP-website/backend/internal/mockinterviews"
	"github.com/magedmg/RSP-website/backend/internal/platform/audit"
	"github.com/magedmg/RSP-website/backend/internal/platform/dbgen"
	"github.com/magedmg/RSP-website/backend/internal/platform/id"
	"github.com/magedmg/RSP-website/backend/internal/platform/observability"
	"github.com/magedmg/RSP-website/backend/internal/practice"
	"github.com/magedmg/RSP-website/backend/internal/programme"
	"github.com/magedmg/RSP-website/backend/internal/store"
)

var (
	ErrNotFound  = store.ErrNotFound
	ErrConflict  = store.ErrConflict
	ErrDuplicate = store.ErrDuplicate
)

// Postgres represents a backend data structure.
type Postgres struct {
	Pool    *pgxpool.Pool
	Queries *dbgen.Queries
}

func finishPostgresPage[T any](items []T, limit int, direction string) ([]T, bool) {
	more := len(items) > limit
	if more {
		items = items[:limit]
	}
	if direction == "backward" {
		for left, right := 0, len(items)-1; left < right; left, right = left+1, right-1 {
			items[left], items[right] = items[right], items[left]
		}
	}
	return items, more
}

// Open opens a connection.
func Open(ctx context.Context, url string) (*Postgres, error) {
	config, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, err
	}

	config.ConnConfig.RuntimeParams["timezone"] = "UTC"
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return &Postgres{Pool: pool, Queries: dbgen.New(pool)}, nil
}

// Close closes a value.
func (p *Postgres) Close() { p.Pool.Close() }

// Ping performs the operation.
func (p *Postgres) Ping(ctx context.Context) error { return p.Pool.Ping(ctx) }

// ObservabilitySnapshot performs the operation.
func (p *Postgres) ObservabilitySnapshot(ctx context.Context) (observability.Snapshot, error) {
	pool := p.Pool.Stat()
	snapshot := observability.Snapshot{
		DBPoolAcquiredConnections: pool.AcquiredConns(),
		DBPoolIdleConnections:     pool.IdleConns(),
		WorkerRuns: map[string]uint64{
			"success":         0,
			"partial_failure": 0,
			"failure":         0,
		},
		MigrationState: "none",
	}
	rows, err := p.Pool.Query(ctx, `
SELECT CASE
         WHEN succeeded THEN 'success'
         WHEN error_summary = 'partial LeetCode sync failure' THEN 'partial_failure'
         ELSE 'failure'
       END AS result,
       count(*)
FROM app.leetcode_sync_runs
WHERE finished_at IS NOT NULL
GROUP BY result`)
	if err != nil {
		return observability.Snapshot{}, err
	}

	for rows.Next() {
		var result string
		var count uint64
		if err := rows.Scan(&result, &count); err != nil {
			rows.Close()
			return observability.Snapshot{}, err
		}
		if _, bounded := snapshot.WorkerRuns[result]; bounded {
			snapshot.WorkerRuns[result] = count
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return observability.Snapshot{}, err
	}

	rows.Close()
	err = p.Pool.QueryRow(ctx, `
SELECT state::text
FROM migration.runs
ORDER BY created_at DESC, id DESC
LIMIT 1`).Scan(&snapshot.MigrationState)
	if errors.Is(err, pgx.ErrNoRows) {
		snapshot.MigrationState = "none"
		return snapshot, nil
	}
	if err != nil {
		return observability.Snapshot{}, err
	}
	return snapshot, nil
}

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

func noRows(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	return err
}

// ResolveAuthSubject performs the operation.
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
func (p *Postgres) GetPracticeSettings(ctx context.Context, userID string) (practice.PracticeSettings, error) {
	var settings practice.PracticeSettings
	err := p.Pool.QueryRow(ctx, `
SELECT u.leetcode_premium_opt_in,
       COALESCE(g.enabled,false), COALESCE(g.easy_minutes,20),
       COALESCE(g.medium_minutes,35), COALESCE(g.hard_minutes,50),
       COALESCE(g.revision,1)
FROM app.users u
LEFT JOIN app.practice_goals g ON g.user_id=u.id
WHERE u.id=$1 AND u.deleted_at IS NULL`, userID).Scan(
		&settings.PremiumOptIn, &settings.GoalsEnabled, &settings.EasyMinutes,
		&settings.MediumMinutes, &settings.HardMinutes, &settings.Revision,
	)
	return settings, noRows(err)
}

// UpdatePracticeSettings updates a value.
func (p *Postgres) UpdatePracticeSettings(ctx context.Context, userID string, revision int64, premium bool, easy, medium, hard int, actorID string, at time.Time) (practice.PracticeSettings, error) {
	tx, err := p.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return practice.PracticeSettings{}, err
	}

	defer tx.Rollback(ctx)
	var currentRevision int64
	var enabled bool
	err = tx.QueryRow(ctx, `
INSERT INTO app.practice_goals(user_id,enabled,easy_minutes,medium_minutes,hard_minutes,revision)
VALUES($1,false,20,35,50,1)
ON CONFLICT(user_id) DO UPDATE SET user_id=EXCLUDED.user_id
RETURNING enabled,revision`, userID).Scan(&enabled, &currentRevision)
	if err != nil {
		return practice.PracticeSettings{}, err
	}
	if currentRevision != revision {
		return practice.PracticeSettings{}, ErrConflict
	}
	if easy < 1 || medium < 1 || hard < 1 {
		return practice.PracticeSettings{}, errors.New("practice goals must be positive")
	}
	if _, err = tx.Exec(ctx, `UPDATE app.users SET leetcode_premium_opt_in=$2,revision=revision+1 WHERE id=$1 AND deleted_at IS NULL`, userID, premium); err != nil {
		return practice.PracticeSettings{}, err
	}

	var settings practice.PracticeSettings
	settings.PremiumOptIn = premium
	err = tx.QueryRow(ctx, `
UPDATE app.practice_goals
SET easy_minutes=$2,medium_minutes=$3,hard_minutes=$4,revision=revision+1
WHERE user_id=$1 AND revision=$5
RETURNING enabled,easy_minutes,medium_minutes,hard_minutes,revision`, userID, easy, medium, hard, revision).Scan(
		&settings.GoalsEnabled, &settings.EasyMinutes, &settings.MediumMinutes, &settings.HardMinutes, &settings.Revision,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return practice.PracticeSettings{}, ErrConflict
	}
	if err != nil {
		return practice.PracticeSettings{}, err
	}

	data, _ := json.Marshal(map[string]any{"premiumOptIn": premium, "easyMinutes": easy, "mediumMinutes": medium, "hardMinutes": hard})
	if _, err = tx.Exec(ctx, `INSERT INTO app.audit_events(id,actor_user_id,action,subject_type,subject_id,data,occurred_at) VALUES($1,$2,'practice_settings.updated','user',$3,$4,$5)`, id.New(), actorID, userID, data, at.UTC()); err != nil {
		return practice.PracticeSettings{}, err
	}
	return settings, tx.Commit(ctx)
}

// EnablePracticeGoals performs the operation.
func (p *Postgres) EnablePracticeGoals(ctx context.Context, userID string, revision int64, actorID, seasonID string, at time.Time) (practice.PracticeSettings, error) {
	tx, err := p.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return practice.PracticeSettings{}, err
	}

	defer tx.Rollback(ctx)
	var settings practice.PracticeSettings
	changed := true
	err = tx.QueryRow(ctx, `
INSERT INTO app.practice_goals(user_id,enabled,enabled_by_user_id,enabled_at,easy_minutes,medium_minutes,hard_minutes,revision)
SELECT $1,true,$2,$3,20,35,50,2 WHERE $4=1
ON CONFLICT(user_id) DO UPDATE
SET enabled=true, enabled_by_user_id=EXCLUDED.enabled_by_user_id, enabled_at=EXCLUDED.enabled_at, revision=app.practice_goals.revision+1
WHERE NOT app.practice_goals.enabled AND app.practice_goals.revision=$4
RETURNING enabled,easy_minutes,medium_minutes,hard_minutes,revision`, userID, actorID, at.UTC(), revision).Scan(
		&settings.GoalsEnabled, &settings.EasyMinutes, &settings.MediumMinutes, &settings.HardMinutes, &settings.Revision,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		changed = false
		err = tx.QueryRow(ctx, `SELECT enabled,easy_minutes,medium_minutes,hard_minutes,revision FROM app.practice_goals WHERE user_id=$1`, userID).Scan(&settings.GoalsEnabled, &settings.EasyMinutes, &settings.MediumMinutes, &settings.HardMinutes, &settings.Revision)
		if errors.Is(err, pgx.ErrNoRows) {
			return practice.PracticeSettings{}, ErrConflict
		}
		if err == nil && settings.Revision != revision {
			return practice.PracticeSettings{}, ErrConflict
		}
	}
	if err != nil {
		return practice.PracticeSettings{}, noRows(err)
	}
	if err = tx.QueryRow(ctx, `SELECT leetcode_premium_opt_in FROM app.users WHERE id=$1`, userID).Scan(&settings.PremiumOptIn); err != nil {
		return practice.PracticeSettings{}, noRows(err)
	}
	if changed {
		if _, err = tx.Exec(ctx, `INSERT INTO app.audit_events(id,actor_user_id,action,subject_type,subject_id,data,occurred_at) VALUES($1,$2,'practice_goals.enabled','user',$3,jsonb_build_object('seasonId',$4::text),$5)`, id.New(), actorID, userID, seasonID, at.UTC()); err != nil {
			return practice.PracticeSettings{}, err
		}
	}
	return settings, tx.Commit(ctx)
}

func scanSeason(row pgx.Row) (programme.SeasonRecord, error) {
	var v programme.SeasonRecord
	if err := row.Scan(&v.ID, &v.Slug, &v.Name, &v.Status, &v.StartAt, &v.EndAt, &v.Location, &v.ImageURL, &v.ResourcesURL, &v.Revision); err != nil {
		return v, noRows(err)
	}
	return v, nil
}

const seasonColumns = `id,slug,name,status::text,start_at,end_at,location,image_url,resources_url,revision`

// GetSeason retrieves a value.
func (p *Postgres) GetSeason(ctx context.Context, id string) (programme.SeasonRecord, error) {
	row, err := p.Queries.GetSeasonByID(ctx, dbgen.GetSeasonByIDParams{ID: id})
	if err != nil {
		return programme.SeasonRecord{}, noRows(err)
	}
	return programme.SeasonRecord{ID: row.ID, Slug: row.Slug, Name: row.Name, Status: string(row.Status), StartAt: row.StartAt.Time, EndAt: row.EndAt.Time, Location: row.Location, ImageURL: row.ImageUrl, ResourcesURL: row.ResourcesUrl, Revision: row.Revision}, nil
}

// ListSeasons lists matching values.
func (p *Postgres) ListSeasons(ctx context.Context, boundary string, limit int, direction, status string) ([]programme.SeasonRecord, bool, int64, error) {
	var total int64
	if err := p.Pool.QueryRow(ctx, `SELECT count(*) FROM app.seasons WHERE deleted_at IS NULL AND ($1='' OR status::text=$1)`, status).Scan(&total); err != nil {
		return nil, false, 0, err
	}

	comparison, order := `id>NULLIF($1,'')::uuid`, `ASC`
	if direction == "backward" {
		comparison, order = `id<NULLIF($1,'')::uuid`, `DESC`
	}
	if boundary == "" {
		comparison = `$1::text IS NOT NULL`
	}
	rows, err := p.Pool.Query(ctx, `SELECT `+seasonColumns+` FROM app.seasons WHERE `+comparison+` AND deleted_at IS NULL AND ($3='' OR status::text=$3) ORDER BY id `+order+` LIMIT $2`, boundary, limit+1, status)
	if err != nil {
		return nil, false, 0, err
	}

	defer rows.Close()
	out := []programme.SeasonRecord{}
	for rows.Next() {
		v, err := scanSeason(rows)
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

// CreateSeason creates a value.
func (p *Postgres) CreateSeason(ctx context.Context, v programme.SeasonRecord, actorID string, at time.Time) (programme.SeasonRecord, error) {
	tx, err := p.Pool.Begin(ctx)
	if err != nil {
		return v, err
	}

	defer tx.Rollback(ctx)
	err = tx.QueryRow(ctx, `INSERT INTO app.seasons(id,slug,name,status,start_at,end_at,location,image_url,resources_url,revision) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10) RETURNING revision`, v.ID, v.Slug, v.Name, v.Status, v.StartAt, v.EndAt, v.Location, v.ImageURL, v.ResourcesURL, v.Revision).Scan(&v.Revision)
	if err != nil {
		if isUnique(err) {
			return v, ErrDuplicate
		}
		return v, err
	}
	if err := appendAuditTx(ctx, tx, newAudit(actorID, "season.created", "season", v.ID, nil, at)); err != nil {
		return v, err
	}
	return v, tx.Commit(ctx)
}

// UpdateSeason updates a value.
func (p *Postgres) UpdateSeason(ctx context.Context, id string, revision int64, fn func(*programme.SeasonRecord) error, actorID string, at time.Time) (programme.SeasonRecord, error) {
	tx, err := p.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return programme.SeasonRecord{}, err
	}

	defer tx.Rollback(ctx)
	v, err := scanSeason(tx.QueryRow(ctx, `SELECT `+seasonColumns+` FROM app.seasons WHERE id=$1 AND deleted_at IS NULL FOR UPDATE`, id))
	if err != nil {
		return v, err
	}
	if v.Revision != revision || v.Status != "open" {
		return v, ErrConflict
	}
	if err := fn(&v); err != nil {
		return v, err
	}

	var invalidDates bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM app.season_weeks WHERE season_id=$1 AND deleted_at IS NULL AND (start_at<$2 OR end_at>$3)) OR EXISTS(SELECT 1 FROM app.mock_interviews WHERE season_id=$1 AND deleted_at IS NULL AND (scheduled_at<$2 OR scheduled_at>$3))`, id, v.StartAt, v.EndAt).Scan(&invalidDates); err != nil {
		return v, err
	}
	if invalidDates {
		return v, ErrConflict
	}
	err = tx.QueryRow(ctx, `UPDATE app.seasons SET slug=$2,name=$3,status=$4::app.season_status,start_at=$5,end_at=$6,location=$7,image_url=$8,resources_url=$9,closed_at=CASE WHEN $4::app.season_status='closed' THEN COALESCE(closed_at,now()) ELSE NULL END,revision=revision+1 WHERE id=$1 AND status='open' AND revision=$10 RETURNING revision`, id, v.Slug, v.Name, v.Status, v.StartAt, v.EndAt, v.Location, v.ImageURL, v.ResourcesURL, revision).Scan(&v.Revision)
	if err != nil {
		return v, noRows(err)
	}
	if err := appendAuditTx(ctx, tx, newAudit(actorID, "season.updated", "season", id, nil, at)); err != nil {
		return v, err
	}
	if err := tx.Commit(ctx); err != nil {
		return v, err
	}
	return v, nil
}

// CloseSeason closes a value.
func (p *Postgres) CloseSeason(ctx context.Context, seasonID string, revision int64, actorID, reason string, at time.Time) (programme.SeasonRecord, error) {
	tx, err := p.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return programme.SeasonRecord{}, err
	}

	defer tx.Rollback(ctx)
	domainSeason, enrollments, err := loadProgrammeSeason(ctx, tx, seasonID)
	if err != nil {
		return programme.SeasonRecord{}, err
	}
	if domainSeason.Revision != revision {
		return programme.SeasonRecord{}, ErrConflict
	}
	closedAt := at.UTC()
	closeEventID := id.New()
	transition, err := programme.Close(domainSeason, actorID, reason, closeEventID, closedAt)
	if err != nil {
		return programme.SeasonRecord{}, ErrConflict
	}
	if _, err := tx.Exec(ctx, `INSERT INTO app.season_close_events(id,season_id,closed_by_user_id,close_reason,closed_at) VALUES($1,$2,$3,$4,$5)`, closeEventID, seasonID, actorID, reason, closedAt); err != nil {
		return programme.SeasonRecord{}, mapPostgresError(err)
	}
	for index, enrollment := range transition.Season.Enrollments {
		if enrollments[index].State != "active" || enrollment.State != "completed" {
			continue
		}
		if _, err := tx.Exec(ctx, `UPDATE app.enrollments SET state='completed',completed_by_close_id=$2,state_changed_at=$3,close_assignment_state=assignment_state,close_activated_at=activated_at,assignment_state='revoked',activated_at=NULL,revision=revision+1 WHERE id=$1 AND revision=$4 AND state='active' AND deleted_at IS NULL`, enrollment.ID, closeEventID, closedAt, enrollments[index].Revision); err != nil {
			return programme.SeasonRecord{}, err
		}
	}

	v, err := scanSeason(tx.QueryRow(ctx, `UPDATE app.seasons SET status='closed',closed_at=$3,revision=revision+1 WHERE id=$1 AND status='open' AND revision=$2 AND deleted_at IS NULL RETURNING `+seasonColumns, seasonID, revision, closedAt))
	if err != nil {
		return programme.SeasonRecord{}, err
	}
	if err := appendAuditTx(ctx, tx, audit.Event{ID: id.New(), ActorID: &actorID, Action: "season.closed", SubjectType: "season", SubjectID: seasonID, Data: map[string]any{"reason": reason, "closeEventId": closeEventID}, OccurredAt: closedAt}); err != nil {
		return programme.SeasonRecord{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return programme.SeasonRecord{}, err
	}
	return v, nil
}

// ReopenSeason reopens a value.
func (p *Postgres) ReopenSeason(ctx context.Context, seasonID string, revision int64, actorID, reason string, at time.Time) (programme.SeasonRecord, error) {
	tx, err := p.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return programme.SeasonRecord{}, err
	}

	defer tx.Rollback(ctx)
	var closeEventID string
	if err := tx.QueryRow(ctx, `SELECT id FROM app.season_close_events WHERE season_id=$1 AND reopened_at IS NULL FOR UPDATE`, seasonID).Scan(&closeEventID); err != nil {
		return programme.SeasonRecord{}, noRows(err)
	}
	domainSeason, enrollments, err := loadProgrammeReopenSeason(ctx, tx, seasonID, closeEventID)
	if err != nil {
		return programme.SeasonRecord{}, err
	}
	if domainSeason.Revision != revision {
		return programme.SeasonRecord{}, ErrConflict
	}

	reopenedAt := at.UTC()
	reopened, err := programme.Reopen(domainSeason, programme.CloseEvent{ID: closeEventID}, programme.SystemAdmin)
	if err != nil {
		return programme.SeasonRecord{}, ErrConflict
	}
	if _, err := tx.Exec(ctx, `UPDATE app.season_close_events SET reopened_by_user_id=$2,reopen_reason=$3,reopened_at=$4 WHERE id=$1 AND reopened_at IS NULL`, closeEventID, actorID, reason, reopenedAt); err != nil {
		return programme.SeasonRecord{}, err
	}
	for index, enrollment := range reopened.Enrollments {
		if enrollments[index].State != "completed" || enrollment.State != "active" {
			continue
		}
		if _, err := tx.Exec(ctx, `UPDATE app.enrollments SET state='active',completed_by_close_id=NULL,state_changed_at=$2::timestamptz,assignment_state=COALESCE(close_assignment_state,'active'::app.assignment_state),activated_at=CASE WHEN COALESCE(close_assignment_state,'active'::app.assignment_state)='active' THEN COALESCE(close_activated_at,$2::timestamptz) ELSE NULL END,close_assignment_state=NULL,close_activated_at=NULL,revision=revision+1 WHERE id=$1 AND revision=$3 AND completed_by_close_id=$4 AND state='completed' AND deleted_at IS NULL`, enrollment.ID, reopenedAt, enrollments[index].Revision, closeEventID); err != nil {
			return programme.SeasonRecord{}, err
		}
	}

	v, err := scanSeason(tx.QueryRow(ctx, `UPDATE app.seasons SET status='open',closed_at=NULL,revision=revision+1 WHERE id=$1 AND status='closed' AND revision=$2 AND deleted_at IS NULL RETURNING `+seasonColumns, seasonID, revision))
	if err != nil {
		return programme.SeasonRecord{}, err
	}
	if err := appendAuditTx(ctx, tx, audit.Event{ID: id.New(), ActorID: &actorID, Action: "season.reopened", SubjectType: "season", SubjectID: seasonID, Data: map[string]any{"reason": reason, "closeEventId": closeEventID}, OccurredAt: reopenedAt}); err != nil {
		return programme.SeasonRecord{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return programme.SeasonRecord{}, err
	}
	return v, nil
}

func scanWeek(row pgx.Row) (programme.WeekRecord, error) {
	var v programme.WeekRecord
	if err := row.Scan(&v.ID, &v.SeasonID, &v.Number, &v.StartAt, &v.EndAt, &v.ResourceURL, &v.Revision); err != nil {
		return v, noRows(err)
	}
	return v, nil
}

// ListWeeks lists matching values.
func (p *Postgres) ListWeeks(ctx context.Context, seasonID, boundary string, limit int, sortBy, direction string) ([]programme.WeekRecord, bool, int64, error) {
	var total int64
	if err := p.Pool.QueryRow(ctx, `SELECT count(*) FROM app.season_weeks WHERE season_id=$1 AND deleted_at IS NULL`, seasonID).Scan(&total); err != nil {
		return nil, false, 0, err
	}

	comparator, order := ">", "ASC"
	if direction == "backward" {
		comparator, order = "<", "DESC"
	}
	key := "w.id"
	boundaryKey := "b.id"
	boundarySelect := "id"
	if sortBy == "number:asc" {
		key = "ROW(w.week_number,w.id)"
		boundaryKey = "ROW(b.week_number,b.id)"
		boundarySelect = "week_number,id"
	}
	orderBy := "w.id " + order
	if sortBy == "number:asc" {
		orderBy = "w.week_number " + order + ",w.id " + order
	}
	query := `WITH boundary AS (
		SELECT ` + boundarySelect + ` FROM app.season_weeks WHERE id=NULLIF($2,'')::uuid AND season_id=$1 AND deleted_at IS NULL
	) SELECT w.id,w.season_id,w.week_number,w.start_at,w.end_at,COALESCE(w.resource_url,''),w.revision
	FROM app.season_weeks w
	WHERE w.season_id=$1 AND w.deleted_at IS NULL
	  AND (NULLIF($2,'')::uuid IS NULL OR EXISTS (SELECT 1 FROM boundary b WHERE ` + key + ` ` + comparator + ` ` + boundaryKey + `))
	ORDER BY ` + orderBy + ` LIMIT $3`
	rows, err := p.Pool.Query(ctx, query, seasonID, boundary, limit+1)
	if err != nil {
		return nil, false, 0, err
	}

	defer rows.Close()
	items := make([]programme.WeekRecord, 0, limit+1)
	for rows.Next() {
		v, scanErr := scanWeek(rows)
		if scanErr != nil {
			return nil, false, 0, scanErr
		}
		items = append(items, v)
	}
	if err := rows.Err(); err != nil {
		return nil, false, 0, err
	}

	items, more := finishPostgresPage(items, limit, direction)
	return items, more, total, nil
}

// CreateWeek creates a value.
func (p *Postgres) CreateWeek(ctx context.Context, v programme.WeekRecord, actorID string, at time.Time) (programme.WeekRecord, error) {
	tx, err := p.Pool.Begin(ctx)
	if err != nil {
		return v, err
	}

	defer tx.Rollback(ctx)
	err = tx.QueryRow(ctx, `INSERT INTO app.season_weeks(id,season_id,week_number,start_at,end_at,resource_url,revision) VALUES($1,$2,$3,$4,$5,$6,$7) RETURNING revision`, v.ID, v.SeasonID, v.Number, v.StartAt, v.EndAt, v.ResourceURL, v.Revision).Scan(&v.Revision)
	if err != nil {
		return v, mapPostgresError(err)
	}
	if err := appendAuditTx(ctx, tx, newAudit(actorID, "week.created", "week", v.ID, nil, at)); err != nil {
		return v, err
	}
	return v, tx.Commit(ctx)
}

// UpdateWeek updates a value.
func (p *Postgres) UpdateWeek(ctx context.Context, seasonID, weekID string, revision int64, candidate programme.WeekRecord, actorID string, at time.Time) (programme.WeekRecord, error) {
	tx, err := p.Pool.Begin(ctx)
	if err != nil {
		return programme.WeekRecord{}, err
	}

	defer tx.Rollback(ctx)
	v, err := scanWeek(tx.QueryRow(ctx, `UPDATE app.season_weeks SET week_number=$4,start_at=$5,end_at=$6,resource_url=$7,revision=revision+1 WHERE id=$1 AND season_id=$2 AND revision=$3 AND deleted_at IS NULL RETURNING id,season_id,week_number,start_at,end_at,resource_url,revision`, weekID, seasonID, revision, candidate.Number, candidate.StartAt, candidate.EndAt, candidate.ResourceURL))
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return v, p.classifyRevision(ctx, tx, `SELECT revision FROM app.season_weeks WHERE id=$1 AND season_id=$2 AND deleted_at IS NULL`, weekID, seasonID)
		}
		return v, mapPostgresError(err)
	}
	if err := appendAuditTx(ctx, tx, newAudit(actorID, "week.updated", "week", weekID, nil, at)); err != nil {
		return v, err
	}
	return v, tx.Commit(ctx)
}

// DeleteWeek deletes a value.
func (p *Postgres) DeleteWeek(ctx context.Context, seasonID, weekID string, revision int64, actorID string, at time.Time) error {
	tx, err := p.Pool.Begin(ctx)
	if err != nil {
		return err
	}

	defer tx.Rollback(ctx)
	tag, err := tx.Exec(ctx, `UPDATE app.season_weeks SET deleted_at=$4,revision=revision+1 WHERE id=$1 AND season_id=$2 AND revision=$3 AND deleted_at IS NULL`, weekID, seasonID, revision, at.UTC())
	if err != nil {
		return mapPostgresError(err)
	}
	if tag.RowsAffected() == 0 {
		return p.classifyRevision(ctx, tx, `SELECT revision FROM app.season_weeks WHERE id=$1 AND season_id=$2 AND deleted_at IS NULL`, weekID, seasonID)
	}
	if err := appendAuditTx(ctx, tx, newAudit(actorID, "week.deleted", "week", weekID, nil, at)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

const enrollmentColumns = `e.id,e.season_id,s.slug,e.user_id,e.role::text,e.student_level::text,e.state::text,e.assignment_state::text,CASE WHEN e.state IN ('kicked','withdrawn') THEN (SELECT reason FROM app.enrollment_removal_events WHERE enrollment_id=e.id ORDER BY occurred_at DESC,id DESC LIMIT 1) END,e.revision`

func scanEnrollment(row pgx.Row) (programme.EnrollmentRecord, error) {
	var v programme.EnrollmentRecord
	if err := row.Scan(&v.ID, &v.SeasonID, &v.SeasonSlug, &v.UserID, &v.Role, &v.StudentLevel, &v.State, &v.AssignmentState, &v.RemovalReason, &v.Revision); err != nil {
		return v, noRows(err)
	}
	return v, nil
}

// ListEnrollments lists matching values.
func (p *Postgres) ListEnrollments(ctx context.Context, seasonID, boundary string, limit int, role, state, sortBy, direction string, includeInactive bool) ([]programme.EnrollmentRecord, bool, int64, error) {
	const filters = `e.season_id=$1 AND e.deleted_at IS NULL
		AND ($2='' OR e.role::text=$2) AND ($3='' OR e.state::text=$3)
		AND ($4 OR e.state='active' OR (e.role='student' AND e.state='completed'))`
	var total int64
	if err := p.Pool.QueryRow(ctx, `SELECT count(*) FROM app.enrollments e WHERE `+filters, seasonID, role, state, includeInactive).Scan(&total); err != nil {
		return nil, false, 0, err
	}

	comparator, order := ">", "ASC"
	if direction == "backward" {
		comparator, order = "<", "DESC"
	}
	key, boundaryKey := "e.id", "b.id"
	boundarySelect := "id"
	if sortBy == "role:asc" {
		key, boundaryKey = "ROW(e.role::text,e.id)", "ROW(b.role::text,b.id)"
		boundarySelect = "role,id"
	}
	orderBy := "e.id " + order
	if sortBy == "role:asc" {
		orderBy = "e.role::text " + order + ",e.id " + order
	}
	query := `WITH boundary AS (
		SELECT ` + boundarySelect + ` FROM app.enrollments
		WHERE id=NULLIF($5,'')::uuid AND season_id=$1 AND deleted_at IS NULL
	) SELECT ` + enrollmentColumns + `
	FROM app.enrollments e JOIN app.seasons s ON s.id=e.season_id
	WHERE ` + filters + `
	  AND (NULLIF($5,'')::uuid IS NULL OR EXISTS (SELECT 1 FROM boundary b WHERE ` + key + ` ` + comparator + ` ` + boundaryKey + `))
	ORDER BY ` + orderBy + ` LIMIT $6`
	rows, err := p.Pool.Query(ctx, query, seasonID, role, state, includeInactive, boundary, limit+1)
	if err != nil {
		return nil, false, 0, err
	}

	defer rows.Close()
	items := make([]programme.EnrollmentRecord, 0, limit+1)
	for rows.Next() {
		v, scanErr := scanEnrollment(rows)
		if scanErr != nil {
			return nil, false, 0, scanErr
		}
		items = append(items, v)
	}
	if err := rows.Err(); err != nil {
		return nil, false, 0, err
	}

	items, more := finishPostgresPage(items, limit, direction)
	return items, more, total, nil
}

// ListEnrollmentsForUser lists matching values.
func (p *Postgres) ListEnrollmentsForUser(ctx context.Context, userID string) ([]programme.EnrollmentRecord, error) {
	return p.listEnrollments(ctx, `e.user_id=$1`, userID)
}

func (p *Postgres) listEnrollments(ctx context.Context, predicate, value string) ([]programme.EnrollmentRecord, error) {
	rows, err := p.Pool.Query(ctx, `SELECT `+enrollmentColumns+` FROM app.enrollments e JOIN app.seasons s ON s.id=e.season_id WHERE `+predicate+` AND e.deleted_at IS NULL ORDER BY e.id`, value)
	if err != nil {
		return nil, err
	}

	defer rows.Close()
	items := []programme.EnrollmentRecord{}
	for rows.Next() {
		v, err := scanEnrollment(rows)
		if err != nil {
			return nil, err
		}

		items = append(items, v)
	}
	return items, rows.Err()
}

// GetEnrollment retrieves a value.
func (p *Postgres) GetEnrollment(ctx context.Context, enrollmentID string) (programme.EnrollmentRecord, error) {
	return scanEnrollment(p.Pool.QueryRow(ctx, `SELECT `+enrollmentColumns+` FROM app.enrollments e JOIN app.seasons s ON s.id=e.season_id WHERE e.id=$1 AND e.deleted_at IS NULL`, enrollmentID))
}

// CreateEnrollment creates a value.
func (p *Postgres) CreateEnrollment(ctx context.Context, v programme.EnrollmentRecord, actorID string, at time.Time) (programme.EnrollmentRecord, error) {
	tx, err := p.Pool.Begin(ctx)
	if err != nil {
		return v, err
	}

	defer tx.Rollback(ctx)
	studentLevel := "not_applicable"
	if v.Role == "student" {
		studentLevel = "novice"
	}
	assignmentState := "active"
	if v.Role == "coordinator" {
		assignmentState = "pending_mfa"
	}
	err = tx.QueryRow(ctx, `INSERT INTO app.enrollments(id,user_id,season_id,role,student_level,state,assignment_state,revision) VALUES($1,$2,$3,$4,$5,$6,$7,$8) RETURNING revision,assignment_state::text`, v.ID, v.UserID, v.SeasonID, v.Role, studentLevel, v.State, assignmentState, v.Revision).Scan(&v.Revision, &v.AssignmentState)
	if err != nil {
		return v, mapPostgresError(err)
	}

	v.StudentLevel = studentLevel
	if err := appendAuditTx(ctx, tx, newAudit(actorID, "enrollment.created", "enrollment", v.ID, nil, at)); err != nil {
		return v, err
	}
	return v, tx.Commit(ctx)
}

// UpdateEnrollmentDetails updates a value.
func (p *Postgres) UpdateEnrollmentDetails(ctx context.Context, seasonID, enrollmentID string, revision int64, role, studentLevel, actorID string, at time.Time) (programme.EnrollmentRecord, error) {
	tx, err := p.Pool.Begin(ctx)
	if err != nil {
		return programme.EnrollmentRecord{}, err
	}

	defer tx.Rollback(ctx)
	v, err := scanEnrollment(tx.QueryRow(ctx, `UPDATE app.enrollments e SET role=$4::app.season_role,student_level=$5,assignment_state=CASE WHEN $4::app.season_role='coordinator' THEN 'pending_mfa'::app.assignment_state ELSE 'active'::app.assignment_state END,activated_at=CASE WHEN $4::app.season_role='coordinator' THEN NULL ELSE $6::timestamptz END,revision=e.revision+1 FROM app.seasons s WHERE e.id=$1 AND e.season_id=$2 AND e.revision=$3 AND e.state='active' AND e.deleted_at IS NULL AND s.id=e.season_id RETURNING `+enrollmentColumns, enrollmentID, seasonID, revision, role, studentLevel, at.UTC()))
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return v, p.classifyRevision(ctx, tx, `SELECT revision FROM app.enrollments WHERE id=$1 AND season_id=$2 AND state='active' AND deleted_at IS NULL`, enrollmentID, seasonID)
		}
		return v, mapPostgresError(err)
	}
	if role != "student" {
		_, err = tx.Exec(ctx, `UPDATE app.mentorships SET ended_at=$2,revision=revision+1 WHERE student_enrollment_id=$1 AND ended_at IS NULL AND deleted_at IS NULL`, enrollmentID, at.UTC())
	} else {
		_, err = tx.Exec(ctx, `UPDATE app.mentorships SET ended_at=$2,revision=revision+1 WHERE mentor_enrollment_id=$1 AND ended_at IS NULL AND deleted_at IS NULL`, enrollmentID, at.UTC())
	}
	if err != nil {
		return v, err
	}
	if err := appendAuditTx(ctx, tx, newAudit(actorID, "enrollment.updated", "enrollment", enrollmentID, map[string]any{"role": role, "studentLevel": studentLevel}, at)); err != nil {
		return v, err
	}
	return v, tx.Commit(ctx)
}

// UpdateEnrollment updates a value.
func (p *Postgres) UpdateEnrollment(ctx context.Context, enrollmentID string, revision int64, role, state, reason, actorID string, at time.Time) (programme.EnrollmentRecord, error) {
	tx, err := p.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return programme.EnrollmentRecord{}, err
	}

	defer tx.Rollback(ctx)
	v, err := scanEnrollment(tx.QueryRow(ctx, `SELECT `+enrollmentColumns+` FROM app.enrollments e JOIN app.seasons s ON s.id=e.season_id WHERE e.id=$1 AND e.deleted_at IS NULL FOR UPDATE OF e`, enrollmentID))
	if err != nil {
		return v, err
	}
	if v.Revision != revision || v.State != "active" {
		return v, ErrConflict
	}
	if role != "" {
		studentLevel := "not_applicable"
		if role == "student" {
			studentLevel = "novice"
		}
		if err := tx.QueryRow(ctx, `UPDATE app.enrollments SET role=$2::app.season_role,student_level=$3,assignment_state=CASE WHEN $2::app.season_role='coordinator' THEN 'pending_mfa'::app.assignment_state ELSE 'active'::app.assignment_state END,activated_at=CASE WHEN $2::app.season_role='coordinator' THEN NULL ELSE $5::timestamptz END,revision=revision+1 WHERE id=$1 AND revision=$4 AND state='active' RETURNING revision,assignment_state::text`, enrollmentID, role, studentLevel, revision, at.UTC()).Scan(&v.Revision, &v.AssignmentState); err != nil {
			return v, noRows(err)
		}

		v.Role = role
		v.StudentLevel = studentLevel
		if _, err := tx.Exec(ctx, `UPDATE app.mentorships SET ended_at=$2,revision=revision+1 WHERE student_enrollment_id=$1 AND ended_at IS NULL AND deleted_at IS NULL`, v.ID, at.UTC()); err != nil {
			return v, err
		}
		if err := appendAuditTx(ctx, tx, audit.Event{ID: id.New(), ActorID: &actorID, Action: "enrollment.promoted", SubjectType: "enrollment", SubjectID: v.ID, Data: map[string]any{"role": role}, OccurredAt: at.UTC()}); err != nil {
			return v, err
		}
	}
	if state != "" {
		changedAt := at.UTC()
		if err := tx.QueryRow(ctx, `UPDATE app.enrollments SET state=$2,state_changed_at=$3,assignment_state='revoked',activated_at=NULL,revision=revision+1 WHERE id=$1 AND revision=$4 AND state='active' RETURNING revision`, enrollmentID, state, changedAt, revision).Scan(&v.Revision); err != nil {
			return v, noRows(err)
		}

		v.State = state
		if _, err := tx.Exec(ctx, `UPDATE app.mentorships SET ended_at=$2,revision=revision+1 WHERE (student_enrollment_id=$1 OR mentor_enrollment_id=$1) AND ended_at IS NULL AND deleted_at IS NULL`, v.ID, changedAt); err != nil {
			return v, err
		}

		trimmed := strings.TrimSpace(reason)
		v.RemovalReason = &trimmed
		if _, err := tx.Exec(ctx, `INSERT INTO app.enrollment_removal_events(id,enrollment_id,season_id,subject_user_id,actor_user_id,resulting_state,reason,occurred_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, id.New(), v.ID, v.SeasonID, v.UserID, actorID, state, trimmed, changedAt); err != nil {
			return v, err
		}
		if err := appendAuditTx(ctx, tx, audit.Event{ID: id.New(), ActorID: &actorID, Action: "enrollment.removed", SubjectType: "enrollment", SubjectID: v.ID, Data: map[string]any{"reason": trimmed, "state": state}, OccurredAt: changedAt}); err != nil {
			return v, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return v, err
	}
	return v, nil
}

// ListMentorships lists matching values.
func (p *Postgres) ListMentorships(ctx context.Context, seasonID, boundary string, limit int, sortBy, direction, mentorUserID, studentUserID string) ([]programme.MentorshipRecord, bool, int64, error) {
	const base = ` FROM app.mentorships m
		JOIN app.enrollments mentor ON mentor.id=m.mentor_enrollment_id
		JOIN app.enrollments student ON student.id=m.student_enrollment_id`
	const filters = `m.season_id=$1 AND m.ended_at IS NULL AND m.deleted_at IS NULL AND ($4='' OR mentor.user_id=$4) AND ($5='' OR student.user_id=$5)`
	countFilters := strings.NewReplacer("$4", "$2", "$5", "$3").Replace(filters)
	var total int64
	if err := p.Pool.QueryRow(ctx, `SELECT count(*)`+base+` WHERE `+countFilters, seasonID, mentorUserID, studentUserID).Scan(&total); err != nil {
		return nil, false, 0, err
	}

	comparator, order := ">", "ASC"
	if direction == "backward" {
		comparator, order = "<", "DESC"
	}
	key, boundaryKey := "m.id", "b.id"
	boundarySelect := "m.id"
	if sortBy == "student:asc" {
		key, boundaryKey = "ROW(student.user_id,m.id)", "ROW(b.student_user_id,b.id)"
		boundarySelect = "m.id,student.user_id AS student_user_id"
	}
	orderBy := "m.id " + order
	if sortBy == "student:asc" {
		orderBy = "student.user_id " + order + ",m.id " + order
	}
	query := `WITH boundary AS (
		SELECT ` + boundarySelect + base + ` WHERE m.id=$2 AND ` + filters + `
	) SELECT m.id,m.season_id,mentor.user_id,student.user_id,m.revision` + base + `
	WHERE ` + filters + `
	  AND (NULLIF($2,'')::uuid IS NULL OR EXISTS (SELECT 1 FROM boundary b WHERE ` + key + ` ` + comparator + ` ` + boundaryKey + `))
	ORDER BY ` + orderBy + ` LIMIT $3`
	rows, err := p.Pool.Query(ctx, query, seasonID, boundary, limit+1, mentorUserID, studentUserID)
	if err != nil {
		return nil, false, 0, err
	}

	defer rows.Close()
	items := make([]programme.MentorshipRecord, 0, limit+1)
	for rows.Next() {
		var v programme.MentorshipRecord
		if err := rows.Scan(&v.ID, &v.SeasonID, &v.MentorUserID, &v.StudentUserID, &v.Revision); err != nil {
			return nil, false, 0, err
		}

		items = append(items, v)
	}
	if err := rows.Err(); err != nil {
		return nil, false, 0, err
	}

	items, more := finishPostgresPage(items, limit, direction)
	return items, more, total, nil
}

// CreateMentorship creates a value.
func (p *Postgres) CreateMentorship(ctx context.Context, v programme.MentorshipRecord, actorID string, at time.Time) (programme.MentorshipRecord, error) {
	tx, err := p.Pool.Begin(ctx)
	if err != nil {
		return v, err
	}

	defer tx.Rollback(ctx)
	err = tx.QueryRow(ctx, `INSERT INTO app.mentorships(id,season_id,mentor_enrollment_id,student_enrollment_id,revision) VALUES($1,$2,(SELECT id FROM app.enrollments WHERE season_id=$2 AND user_id=$3 AND role IN ('mentor','coordinator') AND state='active' AND deleted_at IS NULL),(SELECT id FROM app.enrollments WHERE season_id=$2 AND user_id=$4 AND role='student' AND state='active' AND deleted_at IS NULL),$5) RETURNING revision`, v.ID, v.SeasonID, v.MentorUserID, v.StudentUserID, v.Revision).Scan(&v.Revision)
	if err != nil {
		return v, mapPostgresError(err)
	}
	if err := appendAuditTx(ctx, tx, newAudit(actorID, "mentorship.created", "mentorship", v.ID, nil, at)); err != nil {
		return v, err
	}
	return v, tx.Commit(ctx)
}

// UpdateMentorship updates a value.
func (p *Postgres) UpdateMentorship(ctx context.Context, seasonID, mentorshipID string, revision int64, mentorUserID, studentUserID, actorID string, at time.Time) (programme.MentorshipRecord, error) {
	tx, err := p.Pool.Begin(ctx)
	if err != nil {
		return programme.MentorshipRecord{}, err
	}

	defer tx.Rollback(ctx)
	var v programme.MentorshipRecord
	err = tx.QueryRow(ctx, `UPDATE app.mentorships SET mentor_enrollment_id=(SELECT id FROM app.enrollments WHERE season_id=$2 AND user_id=$4 AND role IN ('mentor','coordinator') AND state='active' AND deleted_at IS NULL),student_enrollment_id=(SELECT id FROM app.enrollments WHERE season_id=$2 AND user_id=$5 AND role='student' AND state='active' AND deleted_at IS NULL),revision=revision+1 WHERE id=$1 AND season_id=$2 AND revision=$3 AND ended_at IS NULL AND deleted_at IS NULL RETURNING id,season_id,$4::text,$5::text,revision`, mentorshipID, seasonID, revision, mentorUserID, studentUserID).Scan(&v.ID, &v.SeasonID, &v.MentorUserID, &v.StudentUserID, &v.Revision)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return v, p.classifyRevision(ctx, tx, `SELECT revision FROM app.mentorships WHERE id=$1 AND season_id=$2 AND ended_at IS NULL AND deleted_at IS NULL`, mentorshipID, seasonID)
		}
		return v, mapPostgresError(err)
	}
	if err := appendAuditTx(ctx, tx, newAudit(actorID, "mentorship.updated", "mentorship", mentorshipID, nil, at)); err != nil {
		return v, err
	}
	return v, tx.Commit(ctx)
}

// DeleteMentorship deletes a value.
func (p *Postgres) DeleteMentorship(ctx context.Context, seasonID, mentorshipID string, revision int64, actorID string, at time.Time) error {
	tx, err := p.Pool.Begin(ctx)
	if err != nil {
		return err
	}

	defer tx.Rollback(ctx)
	tag, err := tx.Exec(ctx, `UPDATE app.mentorships SET ended_at=$4,revision=revision+1 WHERE id=$1 AND season_id=$2 AND revision=$3 AND ended_at IS NULL AND deleted_at IS NULL`, mentorshipID, seasonID, revision, at.UTC())
	if err != nil {
		return mapPostgresError(err)
	}
	if tag.RowsAffected() == 0 {
		return p.classifyRevision(ctx, tx, `SELECT revision FROM app.mentorships WHERE id=$1 AND season_id=$2 AND ended_at IS NULL AND deleted_at IS NULL`, mentorshipID, seasonID)
	}
	if err := appendAuditTx(ctx, tx, newAudit(actorID, "mentorship.deleted", "mentorship", mentorshipID, nil, at)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// IsMentorAssigned performs the operation.
func (p *Postgres) IsMentorAssigned(ctx context.Context, seasonID, mentorUserID, studentUserID string) (bool, error) {
	var assigned bool
	err := p.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM app.mentorships m JOIN app.enrollments mentor ON mentor.id=m.mentor_enrollment_id JOIN app.enrollments student ON student.id=m.student_enrollment_id WHERE m.season_id=$1 AND mentor.user_id=$2 AND student.user_id=$3 AND m.ended_at IS NULL AND m.deleted_at IS NULL)`, seasonID, mentorUserID, studentUserID).Scan(&assigned)
	return assigned, err
}

func scanProblem(row pgx.Row) (practice.ProblemRecord, error) {
	var v practice.ProblemRecord
	if err := row.Scan(&v.ID, &v.Number, &v.Title, &v.Link, &v.Difficulty, &v.Premium, &v.Categories, &v.Revision); err != nil {
		return v, noRows(err)
	}
	return v, nil
}

const problemQuery = `SELECT l.id,l.leetcode_number,p.title,COALESCE(p.url,''),l.difficulty::text,l.is_premium,COALESCE(array_agg(c.normalized_name) FILTER (WHERE c.id IS NOT NULL),'{}')::text[],l.revision FROM app.leetcode_problems l JOIN app.problems p ON p.id=l.problem_id LEFT JOIN app.leetcode_problem_category_mappings m ON m.leetcode_problem_id=l.id LEFT JOIN app.leetcode_problem_categories c ON c.id=m.category_id WHERE l.deleted_at IS NULL AND p.deleted_at IS NULL`

// ListProblems lists matching values.
func (p *Postgres) ListProblems(ctx context.Context, boundary string, limit int, difficulty, category string, premium *bool, direction string) ([]practice.ProblemRecord, bool, int64, error) {
	var total int64
	if err := p.Pool.QueryRow(ctx, `SELECT count(*) FROM app.leetcode_problems l WHERE l.deleted_at IS NULL AND ($1='' OR l.difficulty::text=$1) AND ($2='' OR EXISTS(SELECT 1 FROM app.leetcode_problem_category_mappings m JOIN app.leetcode_problem_categories c ON c.id=m.category_id WHERE m.leetcode_problem_id=l.id AND c.deleted_at IS NULL AND lower(c.normalized_name)=lower($2))) AND ($3::boolean IS NULL OR l.is_premium=$3)`, difficulty, category, premium).Scan(&total); err != nil {
		return nil, false, 0, err
	}

	comparison, order := `l.id>NULLIF($1,'')::uuid`, `ASC`
	if direction == "backward" {
		comparison, order = `l.id<NULLIF($1,'')::uuid`, `DESC`
	}
	if boundary == "" {
		comparison = `$1::text IS NOT NULL`
	}
	rows, err := p.Pool.Query(ctx, problemQuery+` AND `+comparison+` AND ($3='' OR l.difficulty::text=$3) AND ($4='' OR EXISTS(SELECT 1 FROM app.leetcode_problem_category_mappings fm JOIN app.leetcode_problem_categories fc ON fc.id=fm.category_id WHERE fm.leetcode_problem_id=l.id AND fc.deleted_at IS NULL AND lower(fc.normalized_name)=lower($4))) AND ($5::boolean IS NULL OR l.is_premium=$5) GROUP BY l.id,p.id ORDER BY l.id `+order+` LIMIT $2`, boundary, limit+1, difficulty, category, premium)
	if err != nil {
		return nil, false, 0, err
	}

	defer rows.Close()
	out := []practice.ProblemRecord{}
	for rows.Next() {
		v, err := scanProblem(rows)
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

func scanAttempt(row pgx.Row) (practice.AttemptRecord, error) {
	var v practice.AttemptRecord
	if err := row.Scan(&v.ID, &v.UserID, &v.ProblemID, &v.Outcome, &v.Confidence, &v.Minutes, &v.Notes, &v.AttemptedAt, &v.SeasonID, &v.WeekID, &v.Revision, &v.DeletedAt, &v.Migrated); err != nil {
		return v, noRows(err)
	}
	return v, nil
}

const attemptColumns = `a.id,a.user_id,COALESCE((SELECT id FROM app.leetcode_problems WHERE problem_id=a.problem_id),a.problem_id),a.outcome::text,a.confidence,a.time_taken_minutes,COALESCE(a.notes_html,''),a.attempted_at,e.season_id,a.season_week_id,a.revision,a.deleted_at,EXISTS(SELECT 1 FROM migration.row_provenance rp WHERE rp.target_table='problem_attempts' AND rp.target_id=a.id::text)`

// GetAttempt retrieves a value.
func (p *Postgres) GetAttempt(ctx context.Context, id string) (practice.AttemptRecord, error) {
	return scanAttempt(p.Pool.QueryRow(ctx, `SELECT `+attemptColumns+` FROM app.problem_attempts a LEFT JOIN app.enrollments e ON e.id=a.enrollment_id WHERE a.id=$1 AND a.deleted_at IS NULL`, id))
}

// ListAttempts lists matching values.
func (p *Postgres) ListAttempts(ctx context.Context, userID, boundary string, limit int, outcome, difficulty, direction string) ([]practice.AttemptRecord, bool, int64, error) {
	var total int64
	if err := p.Pool.QueryRow(ctx, `SELECT count(*) FROM app.problem_attempts a WHERE a.user_id=$1 AND a.deleted_at IS NULL AND ($2='' OR a.outcome::text=$2) AND ($3='' OR EXISTS(SELECT 1 FROM app.leetcode_problems l WHERE l.problem_id=a.problem_id AND l.difficulty::text=$3 AND l.deleted_at IS NULL))`, userID, outcome, difficulty).Scan(&total); err != nil {
		return nil, false, 0, err
	}

	comparison, order := `a.id>NULLIF($2,'')::uuid`, `ASC`
	if direction == "backward" {
		comparison, order = `a.id<NULLIF($2,'')::uuid`, `DESC`
	}
	if boundary == "" {
		comparison = `$2::text IS NOT NULL`
	}
	rows, err := p.Pool.Query(ctx, `SELECT `+attemptColumns+` FROM app.problem_attempts a LEFT JOIN app.enrollments e ON e.id=a.enrollment_id WHERE a.user_id=$1 AND `+comparison+` AND a.deleted_at IS NULL AND ($4='' OR a.outcome::text=$4) AND ($5='' OR EXISTS(SELECT 1 FROM app.leetcode_problems l WHERE l.problem_id=a.problem_id AND l.difficulty::text=$5 AND l.deleted_at IS NULL)) ORDER BY a.id `+order+` LIMIT $3`, userID, boundary, limit+1, outcome, difficulty)
	if err != nil {
		return nil, false, 0, err
	}

	defer rows.Close()
	out := []practice.AttemptRecord{}
	for rows.Next() {
		v, err := scanAttempt(rows)
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

// RecommendationSnapshot performs the operation.
func (p *Postgres) RecommendationSnapshot(ctx context.Context, userID string, _ practice.Goals) (practice.RecommendationSnapshot, error) {
	snapshot := practice.RecommendationSnapshot{ProblemHistory: map[string]practice.ProblemHistory{}, CategoryExposure: map[string]int{}}
	qualityRows, err := p.Pool.Query(ctx, `SELECT `+attemptColumns+` FROM app.problem_attempts a LEFT JOIN app.enrollments e ON e.id=a.enrollment_id WHERE a.user_id=$1 AND a.deleted_at IS NULL AND a.outcome<>'unknown' AND NOT EXISTS(SELECT 1 FROM migration.row_provenance rp WHERE rp.target_table='problem_attempts' AND rp.target_id=a.id::text) ORDER BY a.attempted_at DESC,a.id ASC LIMIT 20`, userID)
	if err != nil {
		return snapshot, err
	}

	for qualityRows.Next() {
		attempt, err := scanAttempt(qualityRows)
		if err != nil {
			qualityRows.Close()
			return snapshot, err
		}

		snapshot.QualityAttempts = append(snapshot.QualityAttempts, attempt)
	}
	if err := qualityRows.Err(); err != nil {
		qualityRows.Close()
		return snapshot, err
	}

	qualityRows.Close()

	exposureRows, err := p.Pool.Query(ctx, `SELECT c.normalized_name,count(*) FROM app.problem_attempts a JOIN app.leetcode_problems l ON l.problem_id=a.problem_id AND l.deleted_at IS NULL JOIN app.leetcode_problem_category_mappings m ON m.leetcode_problem_id=l.id JOIN app.leetcode_problem_categories c ON c.id=m.category_id AND c.deleted_at IS NULL WHERE a.user_id=$1 AND a.deleted_at IS NULL GROUP BY c.normalized_name`, userID)
	if err != nil {
		return snapshot, err
	}

	for exposureRows.Next() {
		var category string
		var count int
		if err := exposureRows.Scan(&category, &count); err != nil {
			exposureRows.Close()
			return snapshot, err
		}

		snapshot.CategoryExposure[category] = count
	}
	if err := exposureRows.Err(); err != nil {
		exposureRows.Close()
		return snapshot, err
	}

	exposureRows.Close()

	qualityProblems := map[string]bool{}
	for _, attempt := range snapshot.QualityAttempts {
		if qualityProblems[attempt.ProblemID] {
			continue
		}
		problem, err := scanProblem(p.Pool.QueryRow(ctx, problemQuery+` AND l.id=$1 GROUP BY l.id,p.id`, attempt.ProblemID))
		if err != nil {
			return snapshot, err
		}

		snapshot.Problems = append(snapshot.Problems, problem)
		qualityProblems[attempt.ProblemID] = true
	}
	return snapshot, nil
}

// RecommendationCandidates performs the operation.
func (p *Postgres) RecommendationCandidates(ctx context.Context, userID string, criteria practice.Criteria, premiumOptIn bool, goals practice.Goals, now time.Time) ([]practice.ProblemRecord, map[string]practice.ProblemHistory, error) {
	now = now.UTC()
	goal := goals[criteria.Difficulty]
	if goal <= 0 {
		goal = practice.DefaultGoals()[criteria.Difficulty]
	}
	filters := ` AND l.difficulty::text=$2
		AND ($3='' OR EXISTS(SELECT 1 FROM app.leetcode_problem_category_mappings fm JOIN app.leetcode_problem_categories fc ON fc.id=fm.category_id WHERE fm.leetcode_problem_id=l.id AND fc.deleted_at IS NULL AND lower(fc.normalized_name)=lower($3)))
		AND ($4 OR NOT l.is_premium)
		AND NOT EXISTS(SELECT 1 FROM app.recommendation_dismissals d WHERE d.user_id=$1 AND d.leetcode_problem_id=l.id AND d.excluded_until>$5)`
	unseen, err := scanProblem(p.Pool.QueryRow(ctx, problemQuery+filters+`
		AND NOT EXISTS(SELECT 1 FROM app.problem_attempts a WHERE a.user_id=$1 AND a.problem_id=l.problem_id AND a.deleted_at IS NULL)
		GROUP BY l.id,p.id ORDER BY l.leetcode_number ASC,l.id ASC LIMIT 1`, userID, string(criteria.Difficulty), criteria.Category, premiumOptIn, now))
	if err == nil {
		return []practice.ProblemRecord{unseen}, map[string]practice.ProblemHistory{}, nil
	}
	if !errors.Is(err, ErrNotFound) {
		return nil, nil, err
	}

	retry, err := scanProblem(p.Pool.QueryRow(ctx, problemQuery+filters+`
		AND EXISTS(
			SELECT 1 FROM app.problem_attempts a
			WHERE a.user_id=$1 AND a.problem_id=l.problem_id AND a.deleted_at IS NULL
			GROUP BY a.problem_id
			HAVING max(a.attempted_at)<=$5::timestamptz-interval '90 days'
			AND bool_or(
				NOT EXISTS(SELECT 1 FROM migration.row_provenance rp WHERE rp.target_table='problem_attempts' AND rp.target_id=a.id::text)
				AND (a.outcome IN ('not_solved','solved_with_hints') OR a.confidence<4 OR a.time_taken_minutes>$6)
			)
		)
		GROUP BY l.id,p.id ORDER BY l.leetcode_number ASC,l.id ASC LIMIT 1`, userID, string(criteria.Difficulty), criteria.Category, premiumOptIn, now, goal))
	if errors.Is(err, ErrNotFound) {
		return []practice.ProblemRecord{}, map[string]practice.ProblemHistory{}, nil
	}
	if err != nil {
		return nil, nil, err
	}

	var lastAttemptedAt time.Time
	if err := p.Pool.QueryRow(ctx, `SELECT max(a.attempted_at) FROM app.problem_attempts a JOIN app.leetcode_problems l ON l.problem_id=a.problem_id WHERE a.user_id=$1 AND l.id=$2 AND a.deleted_at IS NULL`, userID, retry.ID).Scan(&lastAttemptedAt); err != nil {
		return nil, nil, err
	}
	return []practice.ProblemRecord{retry}, map[string]practice.ProblemHistory{retry.ID: {LastAttemptedAt: lastAttemptedAt.UTC(), Weak: true}}, nil
}

// CreateAttempt creates a value.
func (p *Postgres) CreateAttempt(ctx context.Context, v practice.AttemptRecord) (practice.AttemptRecord, bool, error) {
	tx, err := p.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return practice.AttemptRecord{}, false, err
	}

	defer tx.Rollback(ctx)

	var enrollmentID *string
	if v.SeasonID != nil {
		var scopedEnrollmentID string
		err = tx.QueryRow(ctx, `SELECT e.id FROM app.enrollments e JOIN app.seasons s ON s.id=e.season_id WHERE e.user_id=$1 AND e.season_id=$2 AND e.state='active' AND e.deleted_at IS NULL AND s.status='open' AND s.deleted_at IS NULL`, v.UserID, *v.SeasonID).Scan(&scopedEnrollmentID)
		if err != nil {
			return practice.AttemptRecord{}, false, noRows(err)
		}

		enrollmentID = &scopedEnrollmentID
	}
	if v.WeekID != nil && v.SeasonID == nil {
		return practice.AttemptRecord{}, false, ErrNotFound
	}
	if v.WeekID != nil {
		var valid bool
		if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM app.season_weeks WHERE id=$1 AND season_id=$2 AND deleted_at IS NULL)`, *v.WeekID, *v.SeasonID).Scan(&valid); err != nil {
			return practice.AttemptRecord{}, false, err
		}
		if !valid {
			return practice.AttemptRecord{}, false, ErrNotFound
		}
	}
	var baseProblemID string
	if err = tx.QueryRow(ctx, `SELECT problem_id FROM app.leetcode_problems WHERE id=$1 AND deleted_at IS NULL`, v.ProblemID).Scan(&baseProblemID); err != nil {
		return practice.AttemptRecord{}, false, noRows(err)
	}
	if err = tx.QueryRow(ctx, `INSERT INTO app.problem_attempts(id,user_id,problem_id,enrollment_id,season_week_id,attempted_at,time_taken_minutes,outcome,confidence,notes_html,revision) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11) RETURNING revision`, v.ID, v.UserID, baseProblemID, enrollmentID, v.WeekID, v.AttemptedAt, v.Minutes, v.Outcome, v.Confidence, v.Notes, v.Revision).Scan(&v.Revision); err != nil {
		return v, false, mapPostgresError(err)
	}

	tag, err := tx.Exec(ctx, `UPDATE app.recommendations SET state='attempted',fulfilled_by_attempt_id=$3,state_changed_at=$4,revision=revision+1 WHERE user_id=$1 AND state='active' AND leetcode_problem_id=$2 AND deleted_at IS NULL`, v.UserID, v.ProblemID, v.ID, time.Now().UTC())
	if err != nil {
		return v, false, err
	}

	actorID := v.UserID
	if err := appendAuditTx(ctx, tx, audit.Event{ID: id.New(), ActorID: &actorID, Action: "attempt.created", SubjectType: "problem_attempt", SubjectID: v.ID, Data: map[string]any{}, OccurredAt: time.Now().UTC()}); err != nil {
		return v, false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return v, false, err
	}
	return v, tag.RowsAffected() > 0, nil
}

// UpdateAttempt updates a value.
func (p *Postgres) UpdateAttempt(ctx context.Context, id, userID string, revision int64, fn func(*practice.AttemptRecord) error) (practice.AttemptRecord, error) {
	tx, err := p.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return practice.AttemptRecord{}, err
	}

	defer tx.Rollback(ctx)
	v, err := scanAttempt(tx.QueryRow(ctx, `SELECT `+attemptColumns+` FROM app.problem_attempts a LEFT JOIN app.enrollments e ON e.id=a.enrollment_id WHERE a.id=$1 AND a.user_id=$2 AND a.deleted_at IS NULL FOR UPDATE OF a`, id, userID))
	if err != nil {
		return v, err
	}
	if v.Revision != revision {
		return v, ErrConflict
	}
	if err := fn(&v); err != nil {
		return v, err
	}
	if v.WeekID != nil && v.SeasonID == nil {
		return v, ErrConflict
	}
	var enrollmentID *string
	if v.SeasonID != nil {
		var resolved string
		if err := tx.QueryRow(ctx, `SELECT e.id FROM app.enrollments e JOIN app.seasons s ON s.id=e.season_id WHERE e.user_id=$1 AND e.season_id=$2 AND e.state='active' AND e.deleted_at IS NULL AND s.status='open' AND s.deleted_at IS NULL`, userID, *v.SeasonID).Scan(&resolved); err != nil {
			return v, ErrConflict
		}

		enrollmentID = &resolved
		if v.WeekID != nil {
			var valid bool
			if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM app.season_weeks WHERE id=$1 AND season_id=$2 AND deleted_at IS NULL)`, *v.WeekID, *v.SeasonID).Scan(&valid); err != nil || !valid {
				return v, ErrConflict
			}
		}
	}
	var baseProblemID string
	if err := tx.QueryRow(ctx, `SELECT problem_id FROM app.leetcode_problems WHERE id=$1 AND deleted_at IS NULL`, v.ProblemID).Scan(&baseProblemID); err != nil {
		return v, noRows(err)
	}

	err = tx.QueryRow(ctx, `UPDATE app.problem_attempts SET problem_id=$2,enrollment_id=$3,attempted_at=$4,time_taken_minutes=$5,outcome=$6,confidence=$7,notes_html=$8,season_week_id=$9,revision=revision+1 WHERE id=$1 AND user_id=$10 AND revision=$11 RETURNING revision`, id, baseProblemID, enrollmentID, v.AttemptedAt, v.Minutes, v.Outcome, v.Confidence, v.Notes, v.WeekID, userID, revision).Scan(&v.Revision)
	if err != nil {
		return v, mapPostgresError(err)
	}
	if err := appendAuditTx(ctx, tx, newAudit(userID, "attempt.updated", "problem_attempt", id, nil, time.Now())); err != nil {
		return v, err
	}
	if err := tx.Commit(ctx); err != nil {
		return v, err
	}
	return v, nil
}

// DeleteAttempt deletes a value.
func (p *Postgres) DeleteAttempt(ctx context.Context, id, userID string, revision int64) error {
	tx, err := p.Pool.Begin(ctx)
	if err != nil {
		return err
	}

	defer tx.Rollback(ctx)
	now := time.Now().UTC()
	tag, err := tx.Exec(ctx, `UPDATE app.problem_attempts SET deleted_at=$4,revision=revision+1 WHERE id=$1 AND user_id=$2 AND revision=$3 AND deleted_at IS NULL`, id, userID, revision, now)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		var exists bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM app.problem_attempts WHERE id=$1 AND user_id=$2 AND deleted_at IS NULL)`, id, userID).Scan(&exists); err != nil {
			return err
		}
		if exists {
			return ErrConflict
		}
		return ErrNotFound
	}
	if err := appendAuditTx(ctx, tx, newAudit(userID, "attempt.deleted", "problem_attempt", id, nil, now)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// GetActiveRecommendation retrieves a value.
func (p *Postgres) GetActiveRecommendation(ctx context.Context, userID string) (*practice.Recommendation, error) {
	const query = `SELECT r.id,r.user_id,l.id,l.leetcode_number,p.title,COALESCE(p.url,''),l.difficulty::text,l.is_premium,COALESCE(array_agg(c.normalized_name) FILTER (WHERE c.id IS NOT NULL),'{}')::text[],l.revision,r.difficulty::text,COALESCE(rc.normalized_name,''),r.rationale,r.rule_version,r.generated_at,r.revision FROM app.recommendations r JOIN app.leetcode_problems l ON l.id=r.leetcode_problem_id JOIN app.problems p ON p.id=l.problem_id LEFT JOIN app.leetcode_problem_category_mappings pcm ON pcm.leetcode_problem_id=l.id LEFT JOIN app.leetcode_problem_categories c ON c.id=pcm.category_id LEFT JOIN app.leetcode_problem_categories rc ON rc.id=r.category_id WHERE r.user_id=$1 AND r.state='active' AND r.deleted_at IS NULL GROUP BY r.id,l.id,p.id,rc.id`
	var v practice.Recommendation
	var difficulty, recommendationDifficulty string
	err := p.Pool.QueryRow(ctx, query, userID).Scan(&v.ID, &v.UserID, &v.Problem.ID, &v.Problem.Number, &v.Problem.Title, &v.Problem.Link, &difficulty, &v.Problem.Premium, &v.Problem.Categories, &v.Problem.Revision, &recommendationDifficulty, &v.Category, &v.Rationale, &v.RuleVersion, &v.CreatedAt, &v.Revision)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	v.Problem.Difficulty = practice.Difficulty(difficulty)
	v.Difficulty = practice.Difficulty(recommendationDifficulty)
	return &v, nil
}

// SaveRecommendation saves a value.
func (p *Postgres) SaveRecommendation(ctx context.Context, v practice.Recommendation) (practice.Recommendation, error) {
	if v.Revision < 1 {
		v.Revision = 1
	}
	var categoryID *string
	if v.Category != "" {
		_ = p.Pool.QueryRow(ctx, `SELECT id FROM app.leetcode_problem_categories WHERE lower(normalized_name)=lower($1) AND deleted_at IS NULL`, v.Category).Scan(&categoryID)
	}
	err := p.Pool.QueryRow(ctx, `INSERT INTO app.recommendations(id,user_id,leetcode_problem_id,category_id,difficulty,rationale,rule_version,generated_at,revision) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9) ON CONFLICT (id) DO NOTHING RETURNING id,revision`, v.ID, v.UserID, v.Problem.ID, categoryID, string(v.Difficulty), v.Rationale, v.RuleVersion, v.CreatedAt.UTC(), v.Revision).Scan(&v.ID, &v.Revision)
	if errors.Is(err, pgx.ErrNoRows) {
		active, getErr := p.GetActiveRecommendation(ctx, v.UserID)
		if getErr != nil {
			return practice.Recommendation{}, getErr
		}
		if active != nil && active.ID == v.ID {
			return *active, nil
		}
		return practice.Recommendation{}, ErrConflict
	}
	if err != nil {
		return practice.Recommendation{}, mapPostgresError(err)
	}
	return v, nil
}

// ListRecommendationDismissals lists matching values.
func (p *Postgres) ListRecommendationDismissals(ctx context.Context, userID string) ([]practice.Dismissal, error) {
	rows, err := p.Pool.Query(ctx, `SELECT l.id,d.dismissed_at FROM app.recommendation_dismissals d JOIN app.leetcode_problems l ON l.id=d.leetcode_problem_id WHERE d.user_id=$1 ORDER BY d.dismissed_at DESC`, userID)
	if err != nil {
		return nil, err
	}

	defer rows.Close()
	items := []practice.Dismissal{}
	for rows.Next() {
		var v practice.Dismissal
		if err := rows.Scan(&v.ProblemID, &v.DismissedAt); err != nil {
			return nil, err
		}

		items = append(items, v)
	}
	return items, rows.Err()
}

// DismissRecommendation performs the operation.
func (p *Postgres) DismissRecommendation(ctx context.Context, userID string, revision int64, reason string, at time.Time, dismissalID string) (practice.Recommendation, error) {
	tx, err := p.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return practice.Recommendation{}, err
	}

	defer tx.Rollback(ctx)
	var recommendationID, problemID string
	var currentRevision int64
	if err := tx.QueryRow(ctx, `SELECT id,leetcode_problem_id,revision FROM app.recommendations WHERE user_id=$1 AND state='active' AND deleted_at IS NULL FOR UPDATE`, userID).Scan(&recommendationID, &problemID, &currentRevision); err != nil {
		return practice.Recommendation{}, noRows(err)
	}
	if currentRevision != revision {
		return practice.Recommendation{}, ErrConflict
	}
	dismissedAt := at.UTC()
	if _, err := tx.Exec(ctx, `UPDATE app.recommendations SET state='dismissed',state_changed_at=$2,revision=revision+1 WHERE id=$1 AND state='active' AND revision=$3`, recommendationID, dismissedAt, revision); err != nil {
		return practice.Recommendation{}, err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO app.recommendation_dismissals(id,recommendation_id,user_id,leetcode_problem_id,reason,dismissed_at,excluded_until) VALUES($1,$2,$3,$4,NULLIF($5,''),$6,$7)`, dismissalID, recommendationID, userID, problemID, strings.TrimSpace(reason), dismissedAt, dismissedAt.Add(30*24*time.Hour)); err != nil {
		return practice.Recommendation{}, err
	}
	if err := appendAuditTx(ctx, tx, newAudit(userID, "recommendation.dismissed", "recommendation", recommendationID, map[string]any{"reason": strings.TrimSpace(reason)}, dismissedAt)); err != nil {
		return practice.Recommendation{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return practice.Recommendation{}, err
	}
	return practice.Recommendation{ID: recommendationID, UserID: userID, Problem: practice.Problem{ID: problemID}, DismissedAt: &dismissedAt, Revision: revision + 1}, nil
}

// FulfillRecommendation performs the operation.
func (p *Postgres) FulfillRecommendation(ctx context.Context, userID string, attempt practice.AttemptRecord, at time.Time) (bool, error) {
	tag, err := p.Pool.Exec(ctx, `UPDATE app.recommendations SET state='attempted',fulfilled_by_attempt_id=$3,state_changed_at=$4,revision=revision+1 WHERE user_id=$1 AND state='active' AND leetcode_problem_id=$2 AND deleted_at IS NULL`, userID, attempt.ProblemID, attempt.ID, at.UTC())
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

// ListMockInterviews lists matching values.
func (p *Postgres) ListMockInterviews(ctx context.Context, actor authz.Actor, mode, boundary string, limit int, sortBy, direction string) ([]mockinterviews.Interview, bool, int64, error) {
	userID := actor.UserID
	where := `(mi.interviewer_user_id=$1 OR mi.interviewee_user_id=$1)`
	if mode == "given" {
		where = `mi.interviewer_user_id=$1`
	} else if mode == "received" {
		where = `mi.interviewee_user_id=$1`
	} else if mode == "all" && actor.IsPrivileged() {
		where = `$1::text IS NOT NULL`
	} else if mode == "all" {
		where = `(mi.interviewer_user_id=$1 OR mi.interviewee_user_id=$1 OR EXISTS (
			SELECT 1 FROM app.enrollments viewer
			WHERE viewer.user_id=$1 AND viewer.season_id=mi.season_id AND viewer.state IN ('active','completed') AND viewer.deleted_at IS NULL
			AND (viewer.role='coordinator' OR (viewer.role='mentor' AND EXISTS (
				SELECT 1 FROM app.mentorships m
				JOIN app.enrollments mentor ON mentor.id=m.mentor_enrollment_id
				JOIN app.enrollments student ON student.id=m.student_enrollment_id
				WHERE m.season_id=mi.season_id AND m.ended_at IS NULL AND m.deleted_at IS NULL
				AND mentor.user_id=$1 AND student.user_id IN (mi.interviewer_user_id,mi.interviewee_user_id)
		)))))`
	}
	var total int64
	if err := p.Pool.QueryRow(ctx, `SELECT count(*) FROM app.mock_interviews mi WHERE mi.deleted_at IS NULL AND `+where, userID).Scan(&total); err != nil {
		return nil, false, 0, err
	}

	comparator, order := ">", "ASC"
	key, boundaryKey := "mi.id", "b.id"
	boundarySelect := "id"
	if direction == "backward" {
		comparator, order = "<", "DESC"
	}
	if sortBy == "occurredAt:desc" {
		key, boundaryKey = "ROW(mi.scheduled_at,mi.id)", "ROW(b.scheduled_at,b.id)"
		boundarySelect = "scheduled_at,id"
		comparator, order = "<", "DESC"
		if direction == "backward" {
			comparator, order = ">", "ASC"
		}
	}
	orderBy := "mi.id " + order
	if sortBy == "occurredAt:desc" {
		orderBy = "mi.scheduled_at " + order + ",mi.id " + order
	}
	query := `WITH boundary AS (
		SELECT ` + boundarySelect + ` FROM app.mock_interviews WHERE id=NULLIF($2,'')::uuid AND deleted_at IS NULL
	) SELECT mi.id FROM app.mock_interviews mi
	WHERE mi.deleted_at IS NULL AND ` + where + `
	  AND (NULLIF($2,'')::uuid IS NULL OR EXISTS (SELECT 1 FROM boundary b WHERE ` + key + ` ` + comparator + ` ` + boundaryKey + `))
	ORDER BY ` + orderBy + ` LIMIT $3`
	rows, err := p.Pool.Query(ctx, query, userID, boundary, limit+1)
	if err != nil {
		return nil, false, 0, err
	}

	defer rows.Close()
	ids := []string{}
	for rows.Next() {
		var interviewID string
		if err := rows.Scan(&interviewID); err != nil {
			return nil, false, 0, err
		}

		ids = append(ids, interviewID)
	}
	if err := rows.Err(); err != nil {
		return nil, false, 0, err
	}

	ids, more := finishPostgresPage(ids, limit, direction)
	items := make([]mockinterviews.Interview, 0, len(ids))
	for _, interviewID := range ids {
		v, err := p.GetMockInterview(ctx, interviewID)
		if err != nil {
			return nil, false, 0, err
		}

		items = append(items, v)
	}
	return items, more, total, nil
}

// GetMockParticipant retrieves a value.
func (p *Postgres) GetMockParticipant(ctx context.Context, userID string) (mockinterviews.Participant, error) {
	var ptn mockinterviews.Participant
	var state string
	var hasEnrollment, hasKicked bool
	err := p.Pool.QueryRow(ctx, `SELECT u.id,u.account_state::text,u.is_test,EXISTS(SELECT 1 FROM app.enrollments e WHERE e.user_id=u.id AND e.state='active' AND e.deleted_at IS NULL),EXISTS(SELECT 1 FROM app.enrollments e WHERE e.user_id=u.id AND e.role='student' AND e.state='completed' AND e.deleted_at IS NULL),EXISTS(SELECT 1 FROM app.enrollments e WHERE e.user_id=u.id AND e.state='completed' AND e.deleted_at IS NULL),EXISTS(SELECT 1 FROM app.enrollments e WHERE e.user_id=u.id AND e.deleted_at IS NULL),EXISTS(SELECT 1 FROM app.enrollments e WHERE e.user_id=u.id AND e.state='kicked' AND e.deleted_at IS NULL) FROM app.users u WHERE u.id=$1`, userID).Scan(&ptn.UserID, &state, &ptn.Test, &ptn.ActiveMember, &ptn.Alumni, &ptn.FormerMember, &hasEnrollment, &hasKicked)
	if err != nil {
		return ptn, noRows(err)
	}

	ptn.Suspended = state == "suspended"
	ptn.Deleted = state == "deleted"
	ptn.Inactive = state != "active"
	ptn.KickedOnly = hasEnrollment && hasKicked && !ptn.ActiveMember && !ptn.FormerMember
	return ptn, nil
}

// GetMockInterview retrieves a value.
func (p *Postgres) GetMockInterview(ctx context.Context, interviewID string) (mockinterviews.Interview, error) {
	return loadMockInterview(ctx, p.Pool, interviewID)
}

type rowQuerier interface {
	QueryRow(context.Context, string, ...any) pgx.Row
	Query(context.Context, string, ...any) (pgx.Rows, error)
}

func loadMockInterview(ctx context.Context, db rowQuerier, interviewID string) (mockinterviews.Interview, error) {
	var v mockinterviews.Interview
	if err := db.QueryRow(ctx, `SELECT id,interviewer_user_id,interviewee_user_id,season_id,scheduled_at,duration_minutes,COALESCE(interviewer_notes_html,''),revision,deleted_at FROM app.mock_interviews WHERE id=$1 AND deleted_at IS NULL`, interviewID).Scan(&v.ID, &v.InterviewerID, &v.IntervieweeID, &v.SeasonID, &v.OccurredAt, &v.DurationMinutes, &v.Notes, &v.Revision, &v.DeletedAt); err != nil {
		return v, noRows(err)
	}

	rows, err := db.Query(ctx, `SELECT r.id,r.kind::text,r.review_status='reviewed',COALESCE(r.interviewee_comment_html,''),b.behavioural_score,l.leetcode_problem_id,l.clarify_question_score,l.algorithm_design_score,l.complexity_analysis_score,l.coding_score,l.testing_score,COALESCE(c.content_html,''),COALESCE(c.url,''),c.score FROM app.mock_interview_rounds r LEFT JOIN app.behavioural_mock_interview_rounds b ON b.mock_interview_round_id=r.id LEFT JOIN app.leetcode_mock_interview_rounds l ON l.mock_interview_round_id=r.id LEFT JOIN app.custom_mock_interview_rounds c ON c.mock_interview_round_id=r.id WHERE r.mock_interview_id=$1 AND r.deleted_at IS NULL ORDER BY r.position,r.id`, interviewID)
	if err != nil {
		return v, err
	}

	defer rows.Close()
	for rows.Next() {
		var round mockinterviews.Round
		var kind string
		var behavioural, clarify, algorithm, complexity, coding, testing, custom *int16
		var problemID *string
		if err := rows.Scan(&round.ID, &kind, &round.Reviewed, &round.IntervieweeComment, &behavioural, &problemID, &clarify, &algorithm, &complexity, &coding, &testing, &round.Content, &round.Link, &custom); err != nil {
			return v, err
		}

		round.Type = mockinterviews.RoundType(kind)
		if problemID != nil {
			round.ProblemID = *problemID
		}
		round.Scores = scoresFromDB(behavioural, clarify, algorithm, complexity, coding, testing, custom)
		v.Rounds = append(v.Rounds, round)
	}
	return v, rows.Err()
}

func scoresFromDB(values ...*int16) mockinterviews.Scores {
	toInt := func(v *int16) *int {
		if v == nil {
			return nil
		}
		n := int(*v)
		return &n
	}
	return mockinterviews.Scores{Behavioural: toInt(values[0]), ConfirmQuestions: toInt(values[1]), AlgorithmDesign: toInt(values[2]), ComplexityAnalysis: toInt(values[3]), Coding: toInt(values[4]), Testing: toInt(values[5]), Custom: toInt(values[6])}
}

// CreateMockInterview creates a value.
func (p *Postgres) CreateMockInterview(ctx context.Context, v mockinterviews.Interview, actorID string, at time.Time) (mockinterviews.Interview, error) {
	tx, err := p.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return mockinterviews.Interview{}, err
	}

	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `INSERT INTO app.mock_interviews(id,interviewer_user_id,interviewee_user_id,season_id,scheduled_at,duration_minutes,interviewer_notes_html,revision) VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, v.ID, v.InterviewerID, v.IntervieweeID, v.SeasonID, v.OccurredAt, v.DurationMinutes, v.Notes, v.Revision); err != nil {
		return v, mapPostgresError(err)
	}
	if err := replaceMockRounds(ctx, tx, v); err != nil {
		return v, err
	}
	if err := appendMockVersionTx(ctx, tx, v, actorID, "created", at); err != nil {
		return v, err
	}
	if err := appendAuditTx(ctx, tx, newAudit(actorID, "mock_interview.created", "mock_interview", v.ID, nil, at)); err != nil {
		return v, err
	}
	if err := tx.Commit(ctx); err != nil {
		return v, err
	}
	return v, nil
}

// UpdateMockInterview updates a value.
func (p *Postgres) UpdateMockInterview(ctx context.Context, v mockinterviews.Interview, actorID, reason string, at time.Time) (mockinterviews.Interview, error) {
	tx, err := p.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return mockinterviews.Interview{}, err
	}

	defer tx.Rollback(ctx)
	var currentRevision int64
	if err := tx.QueryRow(ctx, `SELECT revision FROM app.mock_interviews WHERE id=$1 AND deleted_at IS NULL FOR UPDATE`, v.ID).Scan(&currentRevision); err != nil {
		return v, noRows(err)
	}
	if v.Revision != currentRevision+1 {
		return v, ErrConflict
	}
	if reason == "soft deleted" {
		if _, err := tx.Exec(ctx, `UPDATE app.mock_interviews SET deleted_at=$2,revision=$3 WHERE id=$1`, v.ID, v.DeletedAt, v.Revision); err != nil {
			return v, err
		}
	} else {
		if _, err := tx.Exec(ctx, `UPDATE app.mock_interviews SET interviewer_user_id=$2,interviewee_user_id=$3,season_id=$4,scheduled_at=$5,duration_minutes=$6,interviewer_notes_html=$7,revision=$8 WHERE id=$1`, v.ID, v.InterviewerID, v.IntervieweeID, v.SeasonID, v.OccurredAt, v.DurationMinutes, v.Notes, v.Revision); err != nil {
			return v, err
		}
		if err := replaceMockRounds(ctx, tx, v); err != nil {
			return v, err
		}
	}
	if err := appendMockVersionTx(ctx, tx, v, actorID, reason, at); err != nil {
		return v, err
	}

	action := "mock_interview.updated"
	if reason == "soft deleted" {
		action = "mock_interview.deleted"
	} else if strings.HasPrefix(reason, "identity correction:") {
		action = "mock_interview.identities_corrected"
	} else if reason == "interviewee review" {
		action = "mock_interview.reviewed"
	}
	if err := appendAuditTx(ctx, tx, newAudit(actorID, action, "mock_interview", v.ID, map[string]any{"reason": reason}, at)); err != nil {
		return v, err
	}
	if err := tx.Commit(ctx); err != nil {
		return v, err
	}
	return v, nil
}

func replaceMockRounds(ctx context.Context, tx pgx.Tx, v mockinterviews.Interview) error {
	if _, err := tx.Exec(ctx, `DELETE FROM app.mock_interview_rounds WHERE mock_interview_id=$1`, v.ID); err != nil {
		return err
	}

	for position, round := range v.Rounds {
		var err error
		status := "pending"
		var reviewedAt *time.Time
		if round.Reviewed {
			status = "reviewed"
			now := time.Now().UTC()
			reviewedAt = &now
		}
		if _, err := tx.Exec(ctx, `INSERT INTO app.mock_interview_rounds(id,mock_interview_id,position,kind,review_status,interviewee_comment_html,reviewed_at,revision) VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, round.ID, v.ID, position+1, string(round.Type), status, round.IntervieweeComment, reviewedAt, v.Revision); err != nil {
			return err
		}

		switch round.Type {
		case mockinterviews.Behavioural:
			if round.Scores.Behavioural == nil {
				return mockinterviews.ErrInvalid
			}
			_, err = tx.Exec(ctx, `INSERT INTO app.behavioural_mock_interview_rounds(id,mock_interview_round_id,behavioural_score) VALUES($1,$2,$3)`, id.New(), round.ID, *round.Scores.Behavioural)
		case mockinterviews.LeetCode:
			if round.Scores.ConfirmQuestions == nil || round.Scores.AlgorithmDesign == nil || round.Scores.ComplexityAnalysis == nil || round.Scores.Coding == nil || round.Scores.Testing == nil {
				return mockinterviews.ErrInvalid
			}
			_, err = tx.Exec(ctx, `INSERT INTO app.leetcode_mock_interview_rounds(id,mock_interview_round_id,leetcode_problem_id,clarify_question_score,algorithm_design_score,complexity_analysis_score,coding_score,testing_score) VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, id.New(), round.ID, round.ProblemID, *round.Scores.ConfirmQuestions, *round.Scores.AlgorithmDesign, *round.Scores.ComplexityAnalysis, *round.Scores.Coding, *round.Scores.Testing)
		case mockinterviews.Custom:
			if round.Scores.Custom == nil {
				return mockinterviews.ErrInvalid
			}
			_, err = tx.Exec(ctx, `INSERT INTO app.custom_mock_interview_rounds(id,mock_interview_round_id,content_html,url,score) VALUES($1,$2,$3,NULLIF($4,''),$5)`, id.New(), round.ID, round.Content, round.Link, *round.Scores.Custom)
		default:
			return mockinterviews.ErrInvalid
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func appendMockVersionTx(ctx context.Context, tx pgx.Tx, v mockinterviews.Interview, actorID, reason string, at time.Time) error {
	raw, err := json.Marshal(v)
	if err != nil {
		return err
	}

	_, err = tx.Exec(ctx, `INSERT INTO app.mock_interview_versions(id,mock_interview_id,version,actor_user_id,reason,snapshot,created_at) VALUES($1,$2,$3,$4,$5,$6,$7)`, id.New(), v.ID, v.Revision, actorID, reason, raw, at.UTC())
	return err
}

// AppendAudit performs the operation.
func (p *Postgres) AppendAudit(ctx context.Context, v audit.Event) error {
	return appendAuditTx(ctx, p.Pool, v)
}

type execer interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
}

func appendAuditTx(ctx context.Context, db execer, v audit.Event) error {
	raw, err := json.Marshal(v.Data)
	if err != nil {
		return err
	}

	_, err = db.Exec(ctx, `INSERT INTO app.audit_events(id,actor_user_id,action,subject_type,subject_id,data,occurred_at) VALUES($1,$2,$3,$4,$5,$6,$7)`, v.ID, v.ActorID, v.Action, v.SubjectType, v.SubjectID, raw, v.OccurredAt)
	return err
}

func newAudit(actorID, action, subjectType, subjectID string, data map[string]any, at time.Time) audit.Event {
	actor := actorID
	if data == nil {
		data = map[string]any{}
	}
	return audit.Event{ID: id.New(), ActorID: &actor, Action: action, SubjectType: subjectType, SubjectID: subjectID, Data: data, OccurredAt: at.UTC()}
}

func (p *Postgres) classifyRevision(ctx context.Context, db rowQuerier, query string, args ...any) error {
	var revision int64
	if err := db.QueryRow(ctx, query, args...).Scan(&revision); err != nil {
		return noRows(err)
	}
	return ErrConflict
}

func mapPostgresError(err error) error {
	if isUnique(err) {
		return ErrDuplicate
	}

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && (pgErr.Code == "23503" || pgErr.Code == "23514" || pgErr.Code == "23502") {
		return ErrConflict
	}
	return noRows(err)
}

func isUnique(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
