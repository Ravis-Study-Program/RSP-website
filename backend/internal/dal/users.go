package dal

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/magedmg/RSP-website/backend/internal/accounts"
	"github.com/magedmg/RSP-website/backend/internal/platform/id"
)

const userColumns = `u.id,u.slug,u.display_name,u.avatar_url,u.timezone,u.timezone_configured,
	COALESCE(u.email,''),u.account_state::text,
	COALESCE(to_jsonb(array_agg(g.role::text) FILTER (WHERE g.state='active')),'[]'::jsonb),u.is_test,u.revision,
	COALESCE((SELECT jsonb_agg(jsonb_build_object('seasonId',e.season_id,'seasonSlug',s.slug,'role',e.role::text,'state',e.state::text) ORDER BY s.slug,e.id)
		FROM app.enrollments e JOIN app.seasons s ON s.id=e.season_id
		WHERE e.user_id=u.id AND e.deleted_at IS NULL AND (e.state='active' OR (e.role='student' AND e.state='completed'))),'[]'::jsonb),
	(SELECT count(*) FROM app.problem_attempts a WHERE a.user_id=u.id AND a.deleted_at IS NULL),
	(SELECT count(*) FROM app.mock_interviews mi WHERE (mi.interviewer_user_id=u.id OR mi.interviewee_user_id=u.id) AND mi.deleted_at IS NULL)`

func scanUser(row pgx.Row) (accounts.User, error) {
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

type UserQuery struct {
	Boundary   string
	Limit      int
	Direction  string
	Search     string
	SeasonRole string
	GlobalRole string
}

func (p *Store) ListUsers(ctx context.Context, q UserQuery) ([]accounts.User, bool, int64, error) {
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
	args := pgx.NamedArgs{"search": strings.TrimSpace(q.Search), "seasonRole": q.SeasonRole, "globalRole": q.GlobalRole}
	var total int64
	if err := p.pool.QueryRow(ctx, `SELECT count(*) FROM app.users u WHERE `+filters, args).Scan(&total); err != nil {
		return nil, false, 0, err
	}
	items, more, err := p.listUsers(ctx, q.Boundary, q.Limit, q.Direction, filters, args)
	return items, more, total, err
}

type AdminUserQuery struct {
	Boundary     string
	Limit        int
	Direction    string
	Search       string
	AccountState string
	GlobalRole   string
}

func (p *Store) ListAdminUsers(ctx context.Context, q AdminUserQuery) ([]accounts.User, bool, int64, error) {
	const filters = `u.account_state <> 'deleted' AND u.deleted_at IS NULL
		AND (@search = '' OR u.display_name ILIKE '%' || @search || '%'
			OR u.slug ILIKE '%' || @search || '%' OR COALESCE(u.email, '') ILIKE '%' || @search || '%')
		AND (@accountState = '' OR u.account_state::text = @accountState)
		AND (@globalRole = '' OR EXISTS (
			SELECT 1 FROM app.global_role_assignments role_filter
			WHERE role_filter.user_id = u.id AND role_filter.state <> 'revoked'
			AND role_filter.role::text = @globalRole
		))`
	args := pgx.NamedArgs{"search": strings.TrimSpace(q.Search), "accountState": q.AccountState, "globalRole": q.GlobalRole}
	var total int64
	if err := p.pool.QueryRow(ctx, `SELECT count(*) FROM app.users u WHERE `+filters, args).Scan(&total); err != nil {
		return nil, false, 0, err
	}
	items, more, err := p.listUsers(ctx, q.Boundary, q.Limit, q.Direction, filters, args)
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
	items, more := finishPage(items, limit, direction)
	return items, more, nil
}

type EnrollmentCandidateQuery struct {
	SeasonID  string
	Search    string
	Boundary  string
	Limit     int
	Direction string
}

func (p *Store) ListEnrollmentCandidates(ctx context.Context, q EnrollmentCandidateQuery) ([]accounts.EnrollmentCandidate, bool, int64, error) {
	const filters = `u.account_state = 'active' AND NOT u.is_test AND u.deleted_at IS NULL
		AND EXISTS (SELECT 1 FROM app.user_auth_links l WHERE l.user_id = u.id AND l.active)
		AND NOT EXISTS (SELECT 1 FROM app.enrollments e WHERE e.user_id = u.id AND e.season_id = @seasonID)
		AND (@search = '' OR u.display_name ILIKE '%' || @search || '%' OR u.slug ILIKE '%' || @search || '%')`
	args := pgx.NamedArgs{"seasonID": q.SeasonID, "search": strings.TrimSpace(q.Search)}
	var total int64
	if err := p.pool.QueryRow(ctx, `SELECT count(*) FROM app.users u WHERE `+filters, args).Scan(&total); err != nil {
		return nil, false, 0, err
	}

	comparison, order := `u.id > NULLIF(@boundary, '')::uuid`, `ASC`
	if q.Direction == "backward" {
		comparison, order = `u.id < NULLIF(@boundary, '')::uuid`, `DESC`
	}
	if q.Boundary == "" {
		comparison = `TRUE`
	}
	args["boundary"] = q.Boundary
	args["limit"] = q.Limit + 1
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
	items, more := finishPage(items, q.Limit, q.Direction)
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
		return user, mapDatabaseError(err)
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
