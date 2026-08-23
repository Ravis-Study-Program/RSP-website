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
	"github.com/magedmg/RSP-website/backend/internal/authz"
	"github.com/magedmg/RSP-website/backend/internal/mockinterviews"
	"github.com/magedmg/RSP-website/backend/internal/platform/audit"
	"github.com/magedmg/RSP-website/backend/internal/platform/dbgen"
	"github.com/magedmg/RSP-website/backend/internal/platform/id"
	"github.com/magedmg/RSP-website/backend/internal/platform/observability"
	"github.com/magedmg/RSP-website/backend/internal/platform/repository"
	"github.com/magedmg/RSP-website/backend/internal/practice"
)

var (
	ErrNotFound  = repository.ErrNotFound
	ErrConflict  = repository.ErrConflict
	ErrDuplicate = repository.ErrDuplicate
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

func noRows(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	return err
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
