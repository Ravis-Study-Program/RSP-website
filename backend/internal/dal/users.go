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

const userColumns = `u.id,p.slug,p.display_name,p.avatar_url,pr.timezone,pr.timezone_configured,
	COALESCE(c.email,''),u.account_state::text,
	COALESCE(to_jsonb(array_agg(g.role::text) FILTER (WHERE g.state='active')),'[]'::jsonb),
	COALESCE((SELECT jsonb_agg(jsonb_build_object('seasonId',e.season_id,'seasonSlug',s.slug,'role',e.role::text,'state',e.state::text) ORDER BY s.slug,e.id)
		FROM app.enrollments e JOIN app.seasons s ON s.id=e.season_id
		WHERE e.user_id=u.id AND e.deleted_at IS NULL AND (e.state='active' OR (e.role='student' AND e.state='completed'))),'[]'::jsonb),
	(SELECT count(*) FROM app.problem_attempts a WHERE a.user_id=u.id AND a.deleted_at IS NULL),
	(SELECT count(*) FROM app.mock_interviews mi WHERE (mi.interviewer_user_id=u.id OR mi.interviewee_user_id=u.id) AND mi.deleted_at IS NULL)`

func scanUser(row pgx.Row) (accounts.User, error) {
	var user accounts.User
	var globalRoles, seasonRoles []byte
	if err := row.Scan(&user.ID, &user.Slug, &user.Name, &user.AvatarURL, &user.Timezone, &user.TimezoneConfigured, &user.Email, &user.AccountState, &globalRoles, &seasonRoles, &user.AttemptCount, &user.MockInterviewCount); err != nil {
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
		JOIN app.user_profiles p ON p.user_id=u.id
		JOIN app.user_preferences pr ON pr.user_id=u.id
		JOIN app.user_contacts c ON c.user_id=u.id
		LEFT JOIN app.global_role_assignments g ON g.user_id=u.id
		WHERE (u.id::text=$1 OR p.slug=$1) AND u.deleted_at IS NULL
		GROUP BY u.id,p.slug,p.display_name,p.avatar_url,pr.timezone,pr.timezone_configured,c.email,u.account_state`
	return scanUser(p.pool.QueryRow(ctx, query, userID))
}

func (p *Store) SuggestUserSlug(ctx context.Context) (string, error) {
	for attempt := 0; attempt < 32; attempt++ {
		candidate := "member-" + strings.ReplaceAll(id.New(), "-", "")[:12]
		var count int64
		if err := p.pool.QueryRow(ctx, `SELECT count(*) FROM app.user_profiles WHERE lower(slug) = lower($1)`, candidate).Scan(&count); err != nil {
			return "", err
		}
		if count == 0 {
			return candidate, nil
		}
	}
	return "", ErrConflict
}

type UserQuery struct {
	IncludeFormerParticipants bool
	Boundary                  string
	Limit                     int
	Direction                 string
	Search                    string
	SeasonRole                string
	GlobalRole                string
}

func (p *Store) ListUsers(ctx context.Context, q UserQuery) ([]accounts.User, bool, int64, error) {
	const filters = `u.account_state = 'active' AND u.deleted_at IS NULL
		AND (@search = '' OR p.display_name ILIKE '%' || @search || '%' OR p.slug ILIKE '%' || @search || '%')
		AND EXISTS (
			SELECT 1 FROM app.enrollments e
			WHERE e.user_id = u.id AND (e.deleted_at IS NULL OR (@formerParticipants AND e.state IN ('kicked','withdrawn')))
			AND (@formerParticipants OR e.state = 'active' OR (e.role = 'student' AND e.state = 'completed'))
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
	args := pgx.NamedArgs{
		"search":             strings.TrimSpace(q.Search),
		"formerParticipants": q.IncludeFormerParticipants,
		"seasonRole":         q.SeasonRole,
		"globalRole":         q.GlobalRole,
	}
	var total int64
	if err := p.pool.QueryRow(ctx, `SELECT count(*) FROM app.users u JOIN app.user_profiles p ON p.user_id=u.id WHERE `+filters, args).Scan(&total); err != nil {
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
		AND (@search = '' OR p.display_name ILIKE '%' || @search || '%'
			OR p.slug ILIKE '%' || @search || '%' OR COALESCE(c.email, '') ILIKE '%' || @search || '%')
		AND (@accountState = '' OR u.account_state::text = @accountState)
		AND (@globalRole = '' OR EXISTS (
			SELECT 1 FROM app.global_role_assignments role_filter
			WHERE role_filter.user_id = u.id AND role_filter.state <> 'revoked'
			AND role_filter.role::text = @globalRole
		))`
	args := pgx.NamedArgs{
		"search":       strings.TrimSpace(q.Search),
		"accountState": q.AccountState,
		"globalRole":   q.GlobalRole,
	}
	var total int64
	if err := p.pool.QueryRow(ctx, `SELECT count(*) FROM app.users u JOIN app.user_profiles p ON p.user_id=u.id JOIN app.user_contacts c ON c.user_id=u.id WHERE `+filters, args).Scan(&total); err != nil {
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
		JOIN app.user_profiles p ON p.user_id=u.id
		JOIN app.user_preferences pr ON pr.user_id=u.id
		JOIN app.user_contacts c ON c.user_id=u.id
		LEFT JOIN app.global_role_assignments g ON g.user_id=u.id
		WHERE ` + comparison + ` AND ` + filters + `
		GROUP BY u.id,p.slug,p.display_name,p.avatar_url,pr.timezone,pr.timezone_configured,c.email,u.account_state
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
	const filters = `u.account_state = 'active' AND u.deleted_at IS NULL
		AND EXISTS (SELECT 1 FROM app.user_auth_links l WHERE l.user_id = u.id AND l.active)
		AND NOT EXISTS (SELECT 1 FROM app.enrollments e WHERE e.user_id = u.id AND e.season_id = @seasonID)
		AND (@search = '' OR p.display_name ILIKE '%' || @search || '%' OR p.slug ILIKE '%' || @search || '%')`
	args := pgx.NamedArgs{"seasonID": q.SeasonID, "search": strings.TrimSpace(q.Search)}
	var total int64
	if err := p.pool.QueryRow(ctx, `SELECT count(*) FROM app.users u JOIN app.user_profiles p ON p.user_id=u.id WHERE `+filters, args).Scan(&total); err != nil {
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
	rows, err := p.pool.Query(ctx, `SELECT u.id,p.slug,p.display_name,p.avatar_url
		FROM app.users u
		JOIN app.user_profiles p ON p.user_id=u.id
		WHERE `+filters+` AND `+comparison+`
		ORDER BY u.id `+order+` LIMIT @limit`, args)
	if err != nil {
		return nil, false, 0, err
	}
	defer rows.Close()
	items := []accounts.EnrollmentCandidate{}
	for rows.Next() {
		var candidate accounts.EnrollmentCandidate
		if err := rows.Scan(&candidate.ID, &candidate.Slug, &candidate.Name, &candidate.AvatarURL); err != nil {
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

type UpdateUserProfileInput struct {
	UserID    string
	Name      string
	Slug      *string
	AvatarURL *string
	Timezone  string
	ActorID   string
	ChangedAt time.Time
}

func (p *Store) UpdateUserProfile(ctx context.Context, input UpdateUserProfileInput) (accounts.User, error) {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return accounts.User{}, err
	}
	defer tx.Rollback(context.Background())

	var user accounts.User
	err = tx.QueryRow(ctx, `SELECT u.id,p.slug,p.display_name,p.avatar_url,pr.timezone,pr.timezone_configured,
		COALESCE(c.email,''),u.account_state::text
		FROM app.users u
		JOIN app.user_profiles p ON p.user_id=u.id
		JOIN app.user_preferences pr ON pr.user_id=u.id
		JOIN app.user_contacts c ON c.user_id=u.id
		WHERE u.id=$1 AND u.deleted_at IS NULL FOR UPDATE OF u`, input.UserID).Scan(&user.ID, &user.Slug, &user.Name, &user.AvatarURL, &user.Timezone, &user.TimezoneConfigured, &user.Email, &user.AccountState)
	if err != nil {
		return user, noRows(err)
	}
	user.Name = input.Name
	if input.Slug != nil {
		user.Slug = *input.Slug
	}
	user.AvatarURL = input.AvatarURL
	user.Timezone = input.Timezone
	user.TimezoneConfigured = true
	if _, err := tx.Exec(ctx, `UPDATE app.user_profiles SET slug=$1,display_name=$2,avatar_url=$3 WHERE user_id=$4`, user.Slug, user.Name, user.AvatarURL, input.UserID); err != nil {
		return user, err
	}
	if _, err := tx.Exec(ctx, `UPDATE app.user_preferences SET timezone=$1,timezone_configured=$2 WHERE user_id=$3`, user.Timezone, user.TimezoneConfigured, input.UserID); err != nil {
		return user, err
	}
	if err := appendAuditTx(ctx, tx, newAudit(input.ActorID, "user.updated", "user", input.UserID, nil, input.ChangedAt)); err != nil {
		return user, err
	}
	if err := tx.Commit(ctx); err != nil {
		return user, err
	}
	return user, nil
}
