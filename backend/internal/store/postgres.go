package store

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/magedmg/RSP-website/backend/internal/authz"
	"github.com/magedmg/RSP-website/backend/internal/model"
)

type Postgres struct{ Pool *pgxpool.Pool }

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
	return &Postgres{pool}, nil
}
func (p *Postgres) Close()                         { p.Pool.Close() }
func (p *Postgres) Ping(ctx context.Context) error { return p.Pool.Ping(ctx) }
func (p *Postgres) ApplyIdentityEvent(ctx context.Context, event IdentityEvent) error {
	tx, err := p.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var userID string
	err = tx.QueryRow(ctx, `SELECT user_id FROM app.user_auth_links WHERE auth_subject=$1 FOR UPDATE`, event.AuthUserID).Scan(&userID)
	if errors.Is(err, pgx.ErrNoRows) && event.Type == "auth_user_created" {
		userID = event.AuthUserID
		name := event.Email
		if at := strings.IndexByte(name, '@'); at > 0 {
			name = name[:at]
		}
		if strings.TrimSpace(name) == "" {
			name = "New member"
		}
		slug := "member-" + strings.NewReplacer("|", "-", "_", "-").Replace(event.AuthUserID)
		if len(slug) > 80 {
			slug = slug[:80]
		}
		if _, err = tx.Exec(ctx, `INSERT INTO app.users(id,slug,display_name,email,account_state,timezone,revision) VALUES($1,$2,$3,$4,'active','Australia/Adelaide',1)`, userID, slug, name, event.Email); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `INSERT INTO app.user_auth_links(auth_subject,user_id,provider,provider_account_id,active) VALUES($1,$2,'better_auth',$1,true)`, event.AuthUserID, userID); err != nil {
			return err
		}
		return tx.Commit(ctx)
	}
	if err != nil {
		return noRows(err)
	}
	switch event.Type {
	case "auth_user_created", "sessions_revoked":
	case "email_verified":
		_, err = tx.Exec(ctx, `UPDATE app.user_auth_links SET active=true,revoked_at=NULL WHERE auth_subject=$1`, event.AuthUserID)
	case "deletion_requested":
		_, err = tx.Exec(ctx, `UPDATE app.users SET account_state='deletion_pending',deletion_requested_at=$2,deletion_due_at=$3,revision=revision+1 WHERE id=$1`, userID, event.OccurredAt, event.RecoveryDeadline)
	case "deletion_cancelled":
		_, err = tx.Exec(ctx, `UPDATE app.users SET account_state='active',deletion_requested_at=NULL,deletion_due_at=NULL,revision=revision+1 WHERE id=$1`, userID)
	case "auth_pseudonymized":
		_, err = tx.Exec(ctx, `UPDATE app.users SET display_name='Deleted member',email=NULL,avatar_url=NULL,account_state='deleted',pseudonymized_at=$2,deleted_at=$2,revision=revision+1 WHERE id=$1`, userID, event.OccurredAt)
		if err == nil {
			_, err = tx.Exec(ctx, `UPDATE app.user_auth_links SET active=false,revoked_at=$2 WHERE auth_subject=$1`, event.AuthUserID, event.OccurredAt)
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
func (p *Postgres) ResolveAuthSubject(ctx context.Context, sub string) (authz.Actor, error) {
	var a authz.Actor
	var state string
	err := p.Pool.QueryRow(ctx, `SELECT u.id,u.account_state::text FROM app.user_auth_links l JOIN app.users u ON u.id=l.user_id WHERE l.auth_subject=$1 AND l.active AND u.deleted_at IS NULL`, sub).Scan(&a.UserID, &state)
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
	rows, err = p.Pool.Query(ctx, `SELECT season_id,role::text,state::text FROM app.enrollments WHERE user_id=$1 AND deleted_at IS NULL`, a.UserID)
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

const userColumns = `u.id,u.slug,u.display_name,u.avatar_url,u.timezone,COALESCE(u.email,''),u.account_state::text,COALESCE(array_agg(g.role::text) FILTER (WHERE g.state='active'),'{}')::text[],u.revision`

func scanUser(row pgx.Row) (model.User, error) {
	var v model.User
	if err := row.Scan(&v.ID, &v.Slug, &v.Name, &v.AvatarURL, &v.Timezone, &v.Email, &v.AccountState, &v.GlobalRoles, &v.Revision); err != nil {
		return v, noRows(err)
	}
	return v, nil
}
func (p *Postgres) GetUser(ctx context.Context, id string) (model.User, error) {
	return scanUser(p.Pool.QueryRow(ctx, `SELECT `+userColumns+` FROM app.users u LEFT JOIN app.global_role_assignments g ON g.user_id=u.id WHERE u.id=$1 AND u.deleted_at IS NULL GROUP BY u.id`, id))
}
func (p *Postgres) ListUsers(ctx context.Context, after string, limit int) ([]model.User, bool, int64, error) {
	var total int64
	if err := p.Pool.QueryRow(ctx, `SELECT count(*) FROM app.users WHERE deleted_at IS NULL`).Scan(&total); err != nil {
		return nil, false, 0, err
	}
	rows, err := p.Pool.Query(ctx, `SELECT `+userColumns+` FROM app.users u LEFT JOIN app.global_role_assignments g ON g.user_id=u.id WHERE u.id>$1 AND u.deleted_at IS NULL GROUP BY u.id ORDER BY u.id LIMIT $2`, after, limit+1)
	if err != nil {
		return nil, false, 0, err
	}
	defer rows.Close()
	out := []model.User{}
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
	return out, more, total, rows.Err()
}
func (p *Postgres) UpdateUser(ctx context.Context, id string, revision int64, fn func(*model.User) error) (model.User, error) {
	tx, err := p.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return model.User{}, err
	}
	defer tx.Rollback(ctx)
	var v model.User
	err = tx.QueryRow(ctx, `SELECT id,slug,display_name,avatar_url,timezone,COALESCE(email,''),account_state::text,revision FROM app.users WHERE id=$1 AND deleted_at IS NULL FOR UPDATE`, id).Scan(&v.ID, &v.Slug, &v.Name, &v.AvatarURL, &v.Timezone, &v.Email, &v.AccountState, &v.Revision)
	if err != nil {
		return v, noRows(err)
	}
	if v.Revision != revision {
		return v, ErrConflict
	}
	if err := fn(&v); err != nil {
		return v, err
	}
	err = tx.QueryRow(ctx, `UPDATE app.users SET display_name=$2,avatar_url=$3,timezone=$4,revision=revision+1 WHERE id=$1 AND revision=$5 RETURNING revision`, id, v.Name, v.AvatarURL, v.Timezone, revision).Scan(&v.Revision)
	if err != nil {
		return v, noRows(err)
	}
	if err := tx.Commit(ctx); err != nil {
		return v, err
	}
	return v, nil
}
func scanSeason(row pgx.Row) (model.Season, error) {
	var v model.Season
	if err := row.Scan(&v.ID, &v.Slug, &v.Name, &v.Status, &v.StartAt, &v.EndAt, &v.Location, &v.ImageURL, &v.ResourcesURL, &v.Revision); err != nil {
		return v, noRows(err)
	}
	return v, nil
}

const seasonColumns = `id,slug,name,status::text,start_at,end_at,location,image_url,resources_url,revision`

func (p *Postgres) GetSeason(ctx context.Context, id string) (model.Season, error) {
	return scanSeason(p.Pool.QueryRow(ctx, `SELECT `+seasonColumns+` FROM app.seasons WHERE id=$1 AND deleted_at IS NULL`, id))
}
func (p *Postgres) ListSeasons(ctx context.Context, after string, limit int) ([]model.Season, bool, int64, error) {
	var total int64
	if err := p.Pool.QueryRow(ctx, `SELECT count(*) FROM app.seasons WHERE deleted_at IS NULL`).Scan(&total); err != nil {
		return nil, false, 0, err
	}
	rows, err := p.Pool.Query(ctx, `SELECT `+seasonColumns+` FROM app.seasons WHERE id>$1 AND deleted_at IS NULL ORDER BY id LIMIT $2`, after, limit+1)
	if err != nil {
		return nil, false, 0, err
	}
	defer rows.Close()
	out := []model.Season{}
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
	return out, more, total, rows.Err()
}
func (p *Postgres) CreateSeason(ctx context.Context, v model.Season) (model.Season, error) {
	err := p.Pool.QueryRow(ctx, `INSERT INTO app.seasons(id,slug,name,status,start_at,end_at,location,image_url,resources_url,revision) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10) RETURNING revision`, v.ID, v.Slug, v.Name, v.Status, v.StartAt, v.EndAt, v.Location, v.ImageURL, v.ResourcesURL, v.Revision).Scan(&v.Revision)
	if err != nil {
		if isUnique(err) {
			return v, ErrDuplicate
		}
		return v, err
	}
	return v, nil
}
func (p *Postgres) UpdateSeason(ctx context.Context, id string, revision int64, fn func(*model.Season) error) (model.Season, error) {
	tx, err := p.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return model.Season{}, err
	}
	defer tx.Rollback(ctx)
	v, err := scanSeason(tx.QueryRow(ctx, `SELECT `+seasonColumns+` FROM app.seasons WHERE id=$1 AND deleted_at IS NULL FOR UPDATE`, id))
	if err != nil {
		return v, err
	}
	if v.Revision != revision {
		return v, ErrConflict
	}
	if err := fn(&v); err != nil {
		return v, err
	}
	err = tx.QueryRow(ctx, `UPDATE app.seasons SET slug=$2,name=$3,status=$4,start_at=$5,end_at=$6,location=$7,image_url=$8,resources_url=$9,closed_at=CASE WHEN $4='closed' THEN COALESCE(closed_at,now()) ELSE NULL END,revision=revision+1 WHERE id=$1 AND revision=$10 RETURNING revision`, id, v.Slug, v.Name, v.Status, v.StartAt, v.EndAt, v.Location, v.ImageURL, v.ResourcesURL, revision).Scan(&v.Revision)
	if err != nil {
		return v, noRows(err)
	}
	if err := tx.Commit(ctx); err != nil {
		return v, err
	}
	return v, nil
}
func scanProblem(row pgx.Row) (model.Problem, error) {
	var v model.Problem
	if err := row.Scan(&v.ID, &v.Number, &v.Title, &v.Link, &v.Difficulty, &v.Premium, &v.Categories, &v.Revision); err != nil {
		return v, noRows(err)
	}
	return v, nil
}

const problemQuery = `SELECT l.id,l.leetcode_number,p.title,COALESCE(p.url,''),l.difficulty::text,l.is_premium,COALESCE(array_agg(c.normalized_name) FILTER (WHERE c.id IS NOT NULL),'{}')::text[],l.revision FROM app.leetcode_problems l JOIN app.problems p ON p.id=l.problem_id LEFT JOIN app.leetcode_problem_category_mappings m ON m.leetcode_problem_id=l.id LEFT JOIN app.leetcode_problem_categories c ON c.id=m.category_id WHERE l.deleted_at IS NULL AND p.deleted_at IS NULL`

func (p *Postgres) ListProblems(ctx context.Context, after string, limit int) ([]model.Problem, bool, int64, error) {
	var total int64
	if err := p.Pool.QueryRow(ctx, `SELECT count(*) FROM app.leetcode_problems WHERE deleted_at IS NULL`).Scan(&total); err != nil {
		return nil, false, 0, err
	}
	rows, err := p.Pool.Query(ctx, problemQuery+` AND l.id>$1 GROUP BY l.id,p.id ORDER BY l.id LIMIT $2`, after, limit+1)
	if err != nil {
		return nil, false, 0, err
	}
	defer rows.Close()
	out := []model.Problem{}
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
	return out, more, total, rows.Err()
}
func scanAttempt(row pgx.Row) (model.Attempt, error) {
	var v model.Attempt
	if err := row.Scan(&v.ID, &v.UserID, &v.ProblemID, &v.Outcome, &v.Confidence, &v.Minutes, &v.Notes, &v.AttemptedAt, &v.SeasonID, &v.WeekID, &v.Revision, &v.DeletedAt); err != nil {
		return v, noRows(err)
	}
	return v, nil
}

const attemptColumns = `a.id,a.user_id,COALESCE((SELECT id FROM app.leetcode_problems WHERE problem_id=a.problem_id),a.problem_id),a.outcome::text,a.confidence,a.time_taken_minutes,COALESCE(a.notes_html,''),a.attempted_at,e.season_id,a.season_week_id,a.revision,a.deleted_at`

func (p *Postgres) GetAttempt(ctx context.Context, id string) (model.Attempt, error) {
	return scanAttempt(p.Pool.QueryRow(ctx, `SELECT `+attemptColumns+` FROM app.problem_attempts a LEFT JOIN app.enrollments e ON e.id=a.enrollment_id WHERE a.id=$1 AND a.deleted_at IS NULL`, id))
}
func (p *Postgres) ListAttempts(ctx context.Context, userID, after string, limit int) ([]model.Attempt, bool, int64, error) {
	var total int64
	if err := p.Pool.QueryRow(ctx, `SELECT count(*) FROM app.problem_attempts WHERE user_id=$1 AND deleted_at IS NULL`, userID).Scan(&total); err != nil {
		return nil, false, 0, err
	}
	rows, err := p.Pool.Query(ctx, `SELECT `+attemptColumns+` FROM app.problem_attempts a LEFT JOIN app.enrollments e ON e.id=a.enrollment_id WHERE a.user_id=$1 AND a.id>$2 AND a.deleted_at IS NULL ORDER BY a.id LIMIT $3`, userID, after, limit+1)
	if err != nil {
		return nil, false, 0, err
	}
	defer rows.Close()
	out := []model.Attempt{}
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
	return out, more, total, rows.Err()
}
func (p *Postgres) CreateAttempt(ctx context.Context, v model.Attempt) (model.Attempt, error) {
	var enrollmentID *string
	if v.SeasonID != nil {
		_ = p.Pool.QueryRow(ctx, `SELECT id FROM app.enrollments WHERE user_id=$1 AND season_id=$2 AND deleted_at IS NULL`, v.UserID, *v.SeasonID).Scan(&enrollmentID)
	}
	err := p.Pool.QueryRow(ctx, `INSERT INTO app.problem_attempts(id,user_id,problem_id,enrollment_id,season_week_id,attempted_at,time_taken_minutes,outcome,confidence,notes_html,revision) VALUES($1,$2,(SELECT problem_id FROM app.leetcode_problems WHERE id=$3),$4,$5,$6,$7,$8,$9,$10,$11) RETURNING revision`, v.ID, v.UserID, v.ProblemID, enrollmentID, v.WeekID, v.AttemptedAt, v.Minutes, v.Outcome, v.Confidence, v.Notes, v.Revision).Scan(&v.Revision)
	if err != nil {
		return v, err
	}
	return v, nil
}
func (p *Postgres) UpdateAttempt(ctx context.Context, id, userID string, revision int64, fn func(*model.Attempt) error) (model.Attempt, error) {
	tx, err := p.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return model.Attempt{}, err
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
	err = tx.QueryRow(ctx, `UPDATE app.problem_attempts SET problem_id=(SELECT problem_id FROM app.leetcode_problems WHERE id=$2),attempted_at=$3,time_taken_minutes=$4,outcome=$5,confidence=$6,notes_html=$7,season_week_id=$8,revision=revision+1 WHERE id=$1 AND user_id=$9 AND revision=$10 RETURNING revision`, id, v.ProblemID, v.AttemptedAt, v.Minutes, v.Outcome, v.Confidence, v.Notes, v.WeekID, userID, revision).Scan(&v.Revision)
	if err != nil {
		return v, noRows(err)
	}
	if err := tx.Commit(ctx); err != nil {
		return v, err
	}
	return v, nil
}
func (p *Postgres) DeleteAttempt(ctx context.Context, id, userID string, revision int64) error {
	tag, err := p.Pool.Exec(ctx, `UPDATE app.problem_attempts SET deleted_at=now(),revision=revision+1 WHERE id=$1 AND user_id=$2 AND revision=$3 AND deleted_at IS NULL`, id, userID, revision)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		var exists bool
		if err := p.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM app.problem_attempts WHERE id=$1 AND user_id=$2 AND deleted_at IS NULL)`, id, userID).Scan(&exists); err != nil {
			return err
		}
		if exists {
			return ErrConflict
		}
		return ErrNotFound
	}
	return nil
}
func (p *Postgres) AppendAudit(ctx context.Context, v model.AuditEvent) error {
	raw, err := json.Marshal(v.Data)
	if err != nil {
		return err
	}
	_, err = p.Pool.Exec(ctx, `INSERT INTO app.audit_events(id,actor_user_id,action,subject_type,subject_id,data,occurred_at) VALUES($1,$2,$3,$4,$5,$6,$7)`, v.ID, v.ActorID, v.Action, v.SubjectType, v.SubjectID, raw, v.OccurredAt)
	return err
}
func isUnique(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
