package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/magedmg/RSP-website/backend/internal/accounts"
	"github.com/magedmg/RSP-website/backend/internal/authz"
	"github.com/magedmg/RSP-website/backend/internal/platform/audit"
	"github.com/magedmg/RSP-website/backend/internal/platform/dbtable"
	"github.com/magedmg/RSP-website/backend/internal/platform/id"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type identityReceipt struct {
	AuthSubject     string
	EventType       string
	SecurityVersion int64
	PayloadHash     string
}

// ApplyIdentityEvent applies an event from the authentication service exactly
// once. The receipt, account changes, and audit event share one transaction.
func (p *Postgres) ApplyIdentityEvent(ctx context.Context, event accounts.IdentityEvent) error {
	if event.EventID == "" {
		return errors.New("identity event id is required")
	}

	tx, err := begin(ctx, p.DB)
	if err != nil {
		return err
	}
	defer rollback(tx)

	payloadHash := accounts.IdentityEventHash(event)
	receipt := tx.Table(dbtable.IdentityEventReceipts).Clauses(clause.OnConflict{DoNothing: true}).Create(map[string]any{
		"event_id":         event.EventID,
		"auth_subject":     event.AuthUserID,
		"event_type":       event.Type,
		"security_version": event.SecurityVersion,
		"payload_hash":     payloadHash,
	})
	if receipt.Error != nil {
		return receipt.Error
	}
	if receipt.RowsAffected == 0 {
		var existing identityReceipt
		err := tx.Table(dbtable.IdentityEventReceipts).
			Select("auth_subject, event_type, security_version, payload_hash").
			Where("event_id = ?", event.EventID).
			Take(&existing).Error
		if err != nil {
			return err
		}
		if existing.AuthSubject != event.AuthUserID || existing.EventType != event.Type ||
			existing.SecurityVersion != event.SecurityVersion || existing.PayloadHash != payloadHash {
			return ErrConflict
		}
		return commit(tx)
	}

	var link struct {
		UserID          string
		SecurityVersion int64
	}
	err = tx.Raw(`SELECT l.user_id,u.security_version
		FROM app.user_auth_links l
		JOIN app.users u ON u.id=l.user_id
		WHERE l.auth_subject=?
		FOR UPDATE OF l,u`, event.AuthUserID).Scan(&link).Error
	if err != nil {
		return err
	}
	if link.UserID == "" && event.Type == "auth_user_created" {
		return p.createIdentityUser(tx, event)
	}
	if link.UserID == "" {
		return ErrNotFound
	}
	if event.SecurityVersion < 1 {
		return errors.New("invalid security version")
	}
	if event.SecurityVersion < link.SecurityVersion {
		return commit(tx)
	}
	if err := tx.Table(dbtable.Users).Where("id = ?", link.UserID).
		Update("security_version", gorm.Expr("GREATEST(security_version, ?)", event.SecurityVersion)).Error; err != nil {
		return err
	}

	if err := applyIdentityChange(ctx, tx, link.UserID, event); err != nil {
		return err
	}
	return commit(tx)
}

func (p *Postgres) createIdentityUser(tx *gorm.DB, event accounts.IdentityEvent) error {
	userID := id.New()
	name := event.Email
	if at := strings.IndexByte(name, '@'); at > 0 {
		name = name[:at]
	}
	if strings.TrimSpace(name) == "" {
		name = "New member"
	}
	slug := "member-" + strings.ReplaceAll(userID, "-", "")[:12]
	if err := tx.Table(dbtable.Users).Create(map[string]any{
		"id":                  userID,
		"slug":                slug,
		"display_name":        name,
		"email":               event.Email,
		"account_state":       "active",
		"timezone":            "Australia/Adelaide",
		"timezone_configured": false,
		"revision":            1,
		"security_version":    event.SecurityVersion,
	}).Error; err != nil {
		return err
	}
	var revokedAt *time.Time
	if !event.EmailVerified {
		value := event.OccurredAt.UTC()
		revokedAt = &value
	}
	if err := tx.Table(dbtable.UserAuthLinks).Create(map[string]any{
		"auth_subject":        event.AuthUserID,
		"user_id":             userID,
		"provider":            "better_auth",
		"provider_account_id": event.AuthUserID,
		"active":              event.EmailVerified,
		"revoked_at":          revokedAt,
	}).Error; err != nil {
		return err
	}
	return commit(tx)
}

func applyIdentityChange(ctx context.Context, tx *gorm.DB, userID string, event accounts.IdentityEvent) error {
	users := tx.Table(dbtable.Users).Where("id = ?", userID)
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
		if err := users.Updates(map[string]any{"email": email, "revision": gorm.Expr("revision + 1")}).Error; err != nil {
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
		if err := users.Updates(map[string]any{
			"account_state": event.AccountState,
			"suspended_at":  suspendedAt,
			"revision":      gorm.Expr("revision + 1"),
		}).Error; err != nil {
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
		return tx.Table(dbtable.UserAuthLinks).Where("auth_subject = ?", event.AuthUserID).
			Updates(map[string]any{"active": true, "revoked_at": nil}).Error
	case "deletion_requested":
		if err := users.Updates(map[string]any{
			"account_state":         "deletion_pending",
			"deletion_requested_at": event.OccurredAt,
			"deletion_due_at":       event.RecoveryDeadline,
			"revision":              gorm.Expr("revision + 1"),
		}).Error; err != nil {
			return err
		}
		return appendAuditTx(ctx, tx, newAudit(userID, "account.deletion_requested", "user", userID, map[string]any{"recoveryDeadline": event.RecoveryDeadline}, event.OccurredAt))
	case "deletion_cancelled":
		if err := users.Updates(map[string]any{
			"account_state":         "active",
			"deletion_requested_at": nil,
			"deletion_due_at":       nil,
			"revision":              gorm.Expr("revision + 1"),
		}).Error; err != nil {
			return err
		}
		return appendAuditTx(ctx, tx, newAudit(userID, "account.deletion_cancelled", "user", userID, nil, event.OccurredAt))
	case "auth_pseudonymized":
		if err := users.Updates(map[string]any{
			"slug":                gorm.Expr("'deleted-' || id"),
			"display_name":        "Deleted member",
			"email":               nil,
			"discord_id":          nil,
			"avatar_url":          nil,
			"timezone":            "UTC",
			"timezone_configured": false,
			"account_state":       "deleted",
			"pseudonymized_at":    event.OccurredAt,
			"deleted_at":          event.OccurredAt,
			"security_version":    gorm.Expr("GREATEST(security_version, ?)", event.SecurityVersion),
			"revision":            gorm.Expr("revision + 1"),
		}).Error; err != nil {
			return err
		}
		if err := tx.Table(dbtable.UserAuthLinks).Where("auth_subject = ?", event.AuthUserID).
			Updates(map[string]any{"active": false, "revoked_at": event.OccurredAt}).Error; err != nil {
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

func applyMFAConfigured(ctx context.Context, tx *gorm.DB, userID string, at time.Time) error {
	if err := tx.Table(dbtable.Users).Where("id = ?", userID).Update("mfa_configured", true).Error; err != nil {
		return err
	}
	var global []activatedAssignment
	if err := tx.Raw(`UPDATE app.global_role_assignments
		SET state='active',activated_at=?,revision=revision+1
		WHERE user_id=? AND state='pending_mfa'
		RETURNING id,role::text AS role`, at.UTC(), userID).Scan(&global).Error; err != nil {
		return err
	}
	for _, assignment := range global {
		event := audit.Event{ID: id.New(), ActorID: &userID, Action: "global_role.activated", SubjectType: "global_role_assignment", SubjectID: assignment.ID, Data: map[string]any{"role": assignment.Role, "reason": "mfa_configured"}, OccurredAt: at.UTC()}
		if err := appendAuditTx(ctx, tx, event); err != nil {
			return err
		}
	}

	var seasonal []activatedAssignment
	if err := tx.Raw(`UPDATE app.enrollments
		SET assignment_state='active',activated_at=?,revision=revision+1
		WHERE user_id=? AND role='coordinator' AND state='active'
		  AND assignment_state='pending_mfa' AND deleted_at IS NULL
		RETURNING id,season_id`, at.UTC(), userID).Scan(&seasonal).Error; err != nil {
		return err
	}
	for _, enrollment := range seasonal {
		if err := appendAuditTx(ctx, tx, newAudit(userID, "coordinator_role.activated", "enrollment", enrollment.ID, map[string]any{"seasonId": enrollment.SeasonID, "reason": "mfa_configured"}, at)); err != nil {
			return err
		}
	}
	return tx.Table(dbtable.Enrollments).
		Where("user_id = ? AND role = 'coordinator' AND state = 'completed' AND completed_by_close_id IS NOT NULL AND close_assignment_state = 'pending_mfa' AND deleted_at IS NULL", userID).
		Updates(map[string]any{"close_assignment_state": "active", "close_activated_at": at.UTC()}).Error
}

func applyMFADisabled(ctx context.Context, tx *gorm.DB, userID string, at time.Time) error {
	if err := tx.Table(dbtable.Users).Where("id = ?", userID).Update("mfa_configured", false).Error; err != nil {
		return err
	}
	var global []activatedAssignment
	if err := tx.Raw(`UPDATE app.global_role_assignments
		SET state='pending_mfa',activated_at=NULL,revoked_at=NULL,revision=revision+1
		WHERE user_id=? AND state='active'
		RETURNING id,role::text AS role`, userID).Scan(&global).Error; err != nil {
		return err
	}
	for _, assignment := range global {
		if err := appendAuditTx(ctx, tx, newAudit(userID, "global_role.pending_mfa", "global_role_assignment", assignment.ID, map[string]any{"role": assignment.Role, "reason": "mfa_disabled"}, at)); err != nil {
			return err
		}
	}

	var seasonal []activatedAssignment
	if err := tx.Raw(`UPDATE app.enrollments
		SET assignment_state='pending_mfa',activated_at=NULL,revision=revision+1
		WHERE user_id=? AND role='coordinator' AND state='active'
		  AND assignment_state='active' AND deleted_at IS NULL
		RETURNING id,season_id`, userID).Scan(&seasonal).Error; err != nil {
		return err
	}
	for _, enrollment := range seasonal {
		if err := appendAuditTx(ctx, tx, newAudit(userID, "coordinator_role.pending_mfa", "enrollment", enrollment.ID, map[string]any{"seasonId": enrollment.SeasonID, "reason": "mfa_disabled"}, at)); err != nil {
			return err
		}
	}
	return tx.Table(dbtable.Enrollments).
		Where("user_id = ? AND role = 'coordinator' AND state = 'completed' AND completed_by_close_id IS NOT NULL AND close_assignment_state = 'active' AND deleted_at IS NULL", userID).
		Updates(map[string]any{"close_assignment_state": "pending_mfa", "close_activated_at": nil}).Error
}

func (p *Postgres) ResolveAuthSubject(ctx context.Context, subject string) (authz.Actor, error) {
	var actor authz.Actor
	var accountState string
	err := p.DB.WithContext(ctx).Raw(`SELECT u.id,u.account_state::text,u.security_version
		FROM app.user_auth_links l
		JOIN app.users u ON u.id=l.user_id
		WHERE l.auth_subject=? AND l.active AND u.deleted_at IS NULL`, subject).
		Row().Scan(&actor.UserID, &accountState, &actor.SecurityVersion)
	if err != nil {
		return actor, noRows(err)
	}

	actor.EmailVerified = true
	actor.AccountState = authz.AccountState(accountState)
	actor.GlobalRoles = map[authz.GlobalRole]bool{}
	var roles []string
	if err := p.DB.WithContext(ctx).Table(dbtable.GlobalRoleAssignments).
		Where("user_id = ? AND state = 'active'", actor.UserID).
		Pluck("role", &roles).Error; err != nil {
		return actor, err
	}
	for _, role := range roles {
		actor.GlobalRoles[authz.GlobalRole(role)] = true
	}

	rows, err := p.DB.WithContext(ctx).Table(dbtable.Enrollments).
		Select("season_id, role::text, state::text").
		Where("user_id = ? AND deleted_at IS NULL AND (role <> 'coordinator' OR state = 'completed' OR assignment_state = 'active')", actor.UserID).
		Rows()
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

func (p *Postgres) ResolveAuthSubjectForUser(ctx context.Context, userID string) (string, error) {
	var subject string
	err := p.DB.WithContext(ctx).Table(dbtable.UserAuthLinks).
		Select("auth_subject").
		Where("user_id = ? AND active", userID).
		Order("linked_at, auth_subject").
		Limit(1).
		Row().Scan(&subject)
	return subject, noRows(err)
}

func (p *Postgres) GrantGlobalRole(ctx context.Context, userID, role string, activate bool, reason, actorID string, at time.Time) (accounts.GlobalRoleAssignment, error) {
	tx, err := begin(ctx, p.DB)
	if err != nil {
		return accounts.GlobalRoleAssignment{}, err
	}
	defer rollback(tx)

	state := "pending_mfa"
	var activatedAt *time.Time
	if activate {
		state = "active"
		activated := at.UTC()
		activatedAt = &activated
	}
	assignment := accounts.GlobalRoleAssignment{ID: id.New(), UserID: userID, Role: role, State: state, Revision: 1}
	err = tx.Raw(`INSERT INTO app.global_role_assignments(id,user_id,role,state,granted_by_user_id,granted_at,activated_at,revision)
		VALUES(?,?,?,?,?,?,?,1)
		ON CONFLICT(user_id,role) DO UPDATE
		SET state=EXCLUDED.state,granted_by_user_id=EXCLUDED.granted_by_user_id,
			granted_at=EXCLUDED.granted_at,activated_at=EXCLUDED.activated_at,
			revoked_at=NULL,revision=app.global_role_assignments.revision+1
		WHERE app.global_role_assignments.state='revoked'
		RETURNING id,state::text,revision`, assignment.ID, userID, role, state, actorID, at.UTC(), activatedAt).
		Row().Scan(&assignment.ID, &assignment.State, &assignment.Revision)
	if err != nil {
		return assignment, mapPostgresError(err)
	}
	if err := appendAuditTx(ctx, tx, newAudit(actorID, "global_role.granted_"+state, "global_role_assignment", assignment.ID, map[string]any{"userId": userID, "role": role, "reason": reason}, at)); err != nil {
		return assignment, err
	}
	return assignment, commit(tx)
}

func (p *Postgres) ListGlobalRoles(ctx context.Context, userID string) ([]accounts.GlobalRoleAssignment, error) {
	var userCount int64
	if err := p.DB.WithContext(ctx).Table(dbtable.Users).Where("id = ? AND deleted_at IS NULL", userID).Count(&userCount).Error; err != nil {
		return nil, err
	}
	if userCount == 0 {
		return nil, ErrNotFound
	}

	var assignments []accounts.GlobalRoleAssignment
	err := p.DB.WithContext(ctx).Table(dbtable.GlobalRoleAssignments).
		Select("id, user_id, role::text AS role, state::text AS state, revision").
		Where("user_id = ? AND state <> 'revoked'", userID).
		Order("role").
		Scan(&assignments).Error
	return assignments, err
}

func (p *Postgres) RevokeGlobalRole(ctx context.Context, userID, role string, revision int64, reason, actorID string, at time.Time) (accounts.GlobalRoleAssignment, error) {
	tx, err := begin(ctx, p.DB)
	if err != nil {
		return accounts.GlobalRoleAssignment{}, err
	}
	defer rollback(tx)

	assignment := accounts.GlobalRoleAssignment{UserID: userID, Role: role}
	err = tx.Raw(`UPDATE app.global_role_assignments
		SET state='revoked',revoked_at=?,activated_at=NULL,revision=revision+1
		WHERE user_id=? AND role=? AND revision=? AND state<>'revoked'
		RETURNING id,state::text,revision`, at.UTC(), userID, role, revision).
		Row().Scan(&assignment.ID, &assignment.State, &assignment.Revision)
	if errors.Is(err, sql.ErrNoRows) {
		var current int64
		lookupErr := tx.Table(dbtable.GlobalRoleAssignments).
			Select("revision").
			Where("user_id = ? AND role = ? AND state <> 'revoked'", userID, role).
			Row().Scan(&current)
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
	return assignment, commit(tx)
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

func (p *Postgres) GetUser(ctx context.Context, userID string) (accounts.User, error) {
	query := `SELECT ` + userColumns + `
		FROM app.users u
		LEFT JOIN app.global_role_assignments g ON g.user_id=u.id
		WHERE u.id=? AND u.deleted_at IS NULL
		GROUP BY u.id`
	return scanUser(p.DB.WithContext(ctx).Raw(query, userID).Row())
}

func (p *Postgres) SuggestUserSlug(ctx context.Context) (string, error) {
	for attempt := 0; attempt < 32; attempt++ {
		candidate := "member-" + strings.ReplaceAll(id.New(), "-", "")[:12]
		var count int64
		if err := p.DB.WithContext(ctx).Table(dbtable.Users).Where("lower(slug) = lower(?)", candidate).Count(&count).Error; err != nil {
			return "", err
		}
		if count == 0 {
			return candidate, nil
		}
	}
	return "", ErrConflict
}

func (p *Postgres) ListUsers(ctx context.Context, boundary string, limit int, direction, search, seasonRole, globalRole string) ([]accounts.User, bool, int64, error) {
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
	args := []any{
		sql.Named("search", strings.TrimSpace(search)),
		sql.Named("seasonRole", seasonRole),
		sql.Named("globalRole", globalRole),
	}
	var total int64
	if err := p.DB.WithContext(ctx).Raw(`SELECT count(*) FROM app.users u WHERE `+filters, args...).Row().Scan(&total); err != nil {
		return nil, false, 0, err
	}
	items, more, err := p.listUsers(ctx, boundary, limit, direction, filters, args...)
	return items, more, total, err
}

func (p *Postgres) ListAdminUsers(ctx context.Context, boundary string, limit int, direction, search, accountState, globalRole string) ([]accounts.User, bool, int64, error) {
	const filters = `u.account_state <> 'deleted' AND u.deleted_at IS NULL
		AND (@search = '' OR u.display_name ILIKE '%' || @search || '%'
			OR u.slug ILIKE '%' || @search || '%' OR COALESCE(u.email, '') ILIKE '%' || @search || '%')
		AND (@accountState = '' OR u.account_state::text = @accountState)
		AND (@globalRole = '' OR EXISTS (
			SELECT 1 FROM app.global_role_assignments role_filter
			WHERE role_filter.user_id = u.id AND role_filter.state <> 'revoked'
			AND role_filter.role::text = @globalRole
		))`
	args := []any{
		sql.Named("search", strings.TrimSpace(search)),
		sql.Named("accountState", accountState),
		sql.Named("globalRole", globalRole),
	}
	var total int64
	if err := p.DB.WithContext(ctx).Raw(`SELECT count(*) FROM app.users u WHERE `+filters, args...).Row().Scan(&total); err != nil {
		return nil, false, 0, err
	}
	items, more, err := p.listUsers(ctx, boundary, limit, direction, filters, args...)
	return items, more, total, err
}

func (p *Postgres) listUsers(ctx context.Context, boundary string, limit int, direction, filters string, args ...any) ([]accounts.User, bool, error) {
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
	args = append(args, sql.Named("boundary", boundary), sql.Named("limit", limit+1))
	rows, err := p.DB.WithContext(ctx).Raw(query, args...).Rows()
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
	items, more := finishPostgresPage(items, limit, direction)
	return items, more, nil
}

func (p *Postgres) ListEnrollmentCandidates(ctx context.Context, seasonID, query, boundary string, limit int, direction string) ([]accounts.EnrollmentCandidate, bool, int64, error) {
	const filters = `u.account_state = 'active' AND NOT u.is_test AND u.deleted_at IS NULL
		AND EXISTS (SELECT 1 FROM app.user_auth_links l WHERE l.user_id = u.id AND l.active)
		AND NOT EXISTS (SELECT 1 FROM app.enrollments e WHERE e.user_id = u.id AND e.season_id = @seasonID)
		AND (@search = '' OR u.display_name ILIKE '%' || @search || '%' OR u.slug ILIKE '%' || @search || '%')`
	args := []any{sql.Named("seasonID", seasonID), sql.Named("search", strings.TrimSpace(query))}
	var total int64
	if err := p.DB.WithContext(ctx).Raw(`SELECT count(*) FROM app.users u WHERE `+filters, args...).Row().Scan(&total); err != nil {
		return nil, false, 0, err
	}

	comparison, order := `u.id > NULLIF(@boundary, '')::uuid`, `ASC`
	if direction == "backward" {
		comparison, order = `u.id < NULLIF(@boundary, '')::uuid`, `DESC`
	}
	if boundary == "" {
		comparison = `TRUE`
	}
	args = append(args, sql.Named("boundary", boundary), sql.Named("limit", limit+1))
	rows, err := p.DB.WithContext(ctx).Raw(`SELECT u.id,u.slug,u.display_name,u.avatar_url,u.revision
		FROM app.users u
		WHERE `+filters+` AND `+comparison+`
		ORDER BY u.id `+order+` LIMIT @limit`, args...).Rows()
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
	items, more := finishPostgresPage(items, limit, direction)
	return items, more, total, nil
}

func (p *Postgres) UpdateUser(ctx context.Context, userID string, revision int64, update func(*accounts.User) error, actorID string, at time.Time) (accounts.User, error) {
	tx, err := begin(ctx, p.DB)
	if err != nil {
		return accounts.User{}, err
	}
	defer rollback(tx)

	var user accounts.User
	err = tx.Raw(`SELECT id,slug,display_name,avatar_url,timezone,timezone_configured,
		COALESCE(email,''),account_state::text,is_test,revision
		FROM app.users WHERE id=? AND deleted_at IS NULL FOR UPDATE`, userID).
		Row().Scan(&user.ID, &user.Slug, &user.Name, &user.AvatarURL, &user.Timezone, &user.TimezoneConfigured, &user.Email, &user.AccountState, &user.IsTest, &user.Revision)
	if err != nil {
		return user, noRows(err)
	}
	if user.Revision != revision {
		return user, ErrConflict
	}
	if err := update(&user); err != nil {
		return user, err
	}
	result := tx.Table(dbtable.Users).
		Where("id = ? AND revision = ?", userID, revision).
		Updates(map[string]any{
			"slug":                user.Slug,
			"display_name":        user.Name,
			"avatar_url":          user.AvatarURL,
			"timezone":            user.Timezone,
			"timezone_configured": user.TimezoneConfigured,
			"revision":            gorm.Expr("revision + 1"),
		})
	if result.Error != nil {
		return user, mapPostgresError(result.Error)
	}
	if result.RowsAffected == 0 {
		return user, ErrConflict
	}
	user.Revision++
	if err := appendAuditTx(ctx, tx, newAudit(actorID, "user.updated", "user", userID, nil, at)); err != nil {
		return user, err
	}
	if err := commit(tx); err != nil {
		return user, err
	}
	return user, nil
}
