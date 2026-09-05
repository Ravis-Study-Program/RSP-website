package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"time"

	"github.com/magedmg/RSP-website/backend/internal/platform/audit"
	"github.com/magedmg/RSP-website/backend/internal/platform/dbtable"
	"github.com/magedmg/RSP-website/backend/internal/platform/id"
	"github.com/magedmg/RSP-website/backend/internal/practice"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (p *Postgres) GetPracticeSettings(ctx context.Context, userID string) (practice.PracticeSettings, error) {
	var settings practice.PracticeSettings
	err := p.DB.WithContext(ctx).Raw(`SELECT COALESCE(g.enabled,false),
		COALESCE(g.easy_minutes,20),COALESCE(g.medium_minutes,35),COALESCE(g.hard_minutes,50),
		COALESCE(g.revision,1)
		FROM app.users u LEFT JOIN app.practice_goals g ON g.user_id=u.id
		WHERE u.id=? AND u.deleted_at IS NULL`, userID).Row().Scan(
		&settings.GoalsEnabled, &settings.EasyMinutes, &settings.MediumMinutes,
		&settings.HardMinutes, &settings.Revision,
	)
	return settings, noRows(err)
}

func (p *Postgres) UpdatePracticeSettings(ctx context.Context, userID string, revision int64, easy, medium, hard int, actorID string, at time.Time) (practice.PracticeSettings, error) {
	tx, err := begin(ctx, p.DB)
	if err != nil {
		return practice.PracticeSettings{}, err
	}

	defer rollback(tx)
	if err := ensurePracticeGoals(tx, userID); err != nil {
		return practice.PracticeSettings{}, err
	}
	var currentRevision int64
	if err := tx.Table(dbtable.PracticeGoals).Select("revision").Where("user_id = ?", userID).
		Clauses(clause.Locking{Strength: "UPDATE"}).Row().Scan(&currentRevision); err != nil {
		return practice.PracticeSettings{}, err
	}
	if currentRevision != revision {
		return practice.PracticeSettings{}, ErrConflict
	}
	if easy < 1 || medium < 1 || hard < 1 {
		return practice.PracticeSettings{}, errors.New("practice goals must be positive")
	}
	result := tx.Table(dbtable.PracticeGoals).Where("user_id = ? AND revision = ?", userID, revision).
		Updates(map[string]any{"easy_minutes": easy, "medium_minutes": medium, "hard_minutes": hard, "revision": gorm.Expr("revision + 1")})
	if result.Error != nil {
		return practice.PracticeSettings{}, result.Error
	}
	if result.RowsAffected == 0 {
		return practice.PracticeSettings{}, ErrConflict
	}
	settings := practice.PracticeSettings{GoalsEnabled: true, EasyMinutes: easy, MediumMinutes: medium, HardMinutes: hard, Revision: revision + 1}
	if err := tx.Table(dbtable.PracticeGoals).Select("enabled").Where("user_id = ?", userID).Row().Scan(&settings.GoalsEnabled); err != nil {
		return practice.PracticeSettings{}, err
	}

	if err := appendAuditTx(ctx, tx, audit.Event{
		ID:          id.New(),
		ActorID:     &actorID,
		Action:      "practice_settings.updated",
		SubjectType: "user",
		SubjectID:   userID,
		Data:        map[string]any{"easyMinutes": easy, "mediumMinutes": medium, "hardMinutes": hard},
		OccurredAt:  at.UTC(),
	}); err != nil {
		return practice.PracticeSettings{}, err
	}
	return settings, commit(tx)
}

func (p *Postgres) EnablePracticeGoals(ctx context.Context, userID string, revision int64, actorID, seasonID string, at time.Time) (practice.PracticeSettings, error) {
	tx, err := begin(ctx, p.DB)
	if err != nil {
		return practice.PracticeSettings{}, err
	}

	defer rollback(tx)
	if err := ensurePracticeGoals(tx, userID); err != nil {
		return practice.PracticeSettings{}, err
	}
	var stored struct {
		Enabled                                 bool
		EasyMinutes, MediumMinutes, HardMinutes int
		Revision                                int64
	}
	if err := tx.Table(dbtable.PracticeGoals).Where("user_id = ?", userID).Clauses(clause.Locking{Strength: "UPDATE"}).Take(&stored).Error; err != nil {
		return practice.PracticeSettings{}, noRows(err)
	}
	if stored.Revision != revision {
		return practice.PracticeSettings{}, ErrConflict
	}
	changed := !stored.Enabled
	if changed {
		result := tx.Table(dbtable.PracticeGoals).Where("user_id = ? AND revision = ?", userID, revision).
			Updates(map[string]any{"enabled": true, "enabled_by_user_id": actorID, "enabled_at": at.UTC(), "revision": gorm.Expr("revision + 1")})
		if result.Error != nil {
			return practice.PracticeSettings{}, result.Error
		}
		if result.RowsAffected == 0 {
			return practice.PracticeSettings{}, ErrConflict
		}
		stored.Enabled = true
		stored.Revision++
	}
	if changed {
		if err := appendAuditTx(ctx, tx, audit.Event{
			ID:          id.New(),
			ActorID:     &actorID,
			Action:      "practice_goals.enabled",
			SubjectType: "user",
			SubjectID:   userID,
			Data:        map[string]any{"seasonId": seasonID},
			OccurredAt:  at.UTC(),
		}); err != nil {
			return practice.PracticeSettings{}, err
		}
	}
	settings := practice.PracticeSettings{
		GoalsEnabled:  stored.Enabled,
		EasyMinutes:   stored.EasyMinutes,
		MediumMinutes: stored.MediumMinutes,
		HardMinutes:   stored.HardMinutes,
		Revision:      stored.Revision,
	}
	return settings, commit(tx)
}

func ensurePracticeGoals(tx *gorm.DB, userID string) error {
	return tx.Table(dbtable.PracticeGoals).Clauses(clause.OnConflict{DoNothing: true}).Create(map[string]any{
		"user_id": userID, "enabled": false, "easy_minutes": 20,
		"medium_minutes": 35, "hard_minutes": 50, "revision": 1,
	}).Error
}

func scanProblem(row rowScanner) (practice.ProblemRecord, error) {
	var v practice.ProblemRecord
	var categories []byte
	if err := row.Scan(&v.ID, &v.Number, &v.Title, &v.Link, &v.Difficulty, &v.Premium, &categories, &v.Revision); err != nil {
		return v, noRows(err)
	}
	if err := json.Unmarshal(categories, &v.Categories); err != nil {
		return v, err
	}
	return v, nil
}

const problemQuery = `SELECT l.id,l.leetcode_number,p.title,COALESCE(p.url,''),l.difficulty::text,l.is_premium,COALESCE(to_jsonb(array_agg(c.normalized_name) FILTER (WHERE c.id IS NOT NULL)),'[]'::jsonb),l.revision FROM app.leetcode_problems l JOIN app.problems p ON p.id=l.problem_id LEFT JOIN app.leetcode_problem_category_mappings m ON m.leetcode_problem_id=l.id LEFT JOIN app.leetcode_problem_categories c ON c.id=m.category_id WHERE l.deleted_at IS NULL AND p.deleted_at IS NULL`

func (p *Postgres) ListProblems(ctx context.Context, boundary string, limit int, difficulty, category string, premium *bool, direction string) ([]practice.ProblemRecord, bool, int64, error) {
	var total int64
	err := p.DB.WithContext(ctx).Raw(`SELECT count(*) FROM app.leetcode_problems l
		JOIN app.problems p ON p.id=l.problem_id
		WHERE l.deleted_at IS NULL AND p.deleted_at IS NULL
		AND (?='' OR l.difficulty::text=?)
		AND (?='' OR EXISTS(SELECT 1 FROM app.leetcode_problem_category_mappings fm JOIN app.leetcode_problem_categories fc ON fc.id=fm.category_id WHERE fm.leetcode_problem_id=l.id AND fc.deleted_at IS NULL AND lower(fc.normalized_name)=lower(?)))
		AND (?::boolean IS NULL OR l.is_premium=?)`, difficulty, difficulty, category, category, premium, premium).Row().Scan(&total)
	if err != nil {
		return nil, false, 0, err
	}

	comparison, order := `l.id>NULLIF($1,'')::uuid`, `ASC`
	if direction == "backward" {
		comparison, order = `l.id<NULLIF($1,'')::uuid`, `DESC`
	}
	if boundary == "" {
		comparison = `$1::text IS NOT NULL`
	}
	rows, err := p.DB.WithContext(ctx).Raw(problemQuery+` AND `+comparison+` AND ($3='' OR l.difficulty::text=$3) AND ($4='' OR EXISTS(SELECT 1 FROM app.leetcode_problem_category_mappings fm JOIN app.leetcode_problem_categories fc ON fc.id=fm.category_id WHERE fm.leetcode_problem_id=l.id AND fc.deleted_at IS NULL AND lower(fc.normalized_name)=lower($4))) AND ($5::boolean IS NULL OR l.is_premium=$5) GROUP BY l.id,p.id ORDER BY l.id `+order+` LIMIT $2`, boundary, limit+1, difficulty, category, premium).Rows()
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

func scanAttempt(row rowScanner) (practice.AttemptRecord, error) {
	var v practice.AttemptRecord
	if err := row.Scan(&v.ID, &v.UserID, &v.ProblemID, &v.Outcome, &v.Confidence, &v.Minutes, &v.Notes, &v.AttemptedAt, &v.SeasonID, &v.WeekID, &v.Revision, &v.DeletedAt); err != nil {
		return v, noRows(err)
	}
	return v, nil
}

const attemptColumns = `a.id,a.user_id,COALESCE((SELECT id FROM app.leetcode_problems WHERE problem_id=a.problem_id),a.problem_id),a.outcome::text,a.confidence,a.time_taken_minutes,COALESCE(a.notes_html,''),a.attempted_at,e.season_id,a.season_week_id,a.revision,a.deleted_at`

func (p *Postgres) GetAttempt(ctx context.Context, id string) (practice.AttemptRecord, error) {
	return scanAttempt(p.DB.WithContext(ctx).Raw(`SELECT `+attemptColumns+`
		FROM app.problem_attempts a LEFT JOIN app.enrollments e ON e.id=a.enrollment_id
		WHERE a.id=?`, id).Row())
}

func (p *Postgres) ListAttempts(ctx context.Context, userID, boundary string, limit int, outcome, difficulty, direction string) ([]practice.AttemptRecord, bool, int64, error) {
	var total int64
	err := p.DB.WithContext(ctx).Raw(`SELECT count(*) FROM app.problem_attempts a
		WHERE a.user_id=? AND a.deleted_at IS NULL
		AND (?='' OR a.outcome::text=?)
		AND (?='' OR EXISTS(SELECT 1 FROM app.leetcode_problems l WHERE l.problem_id=a.problem_id AND l.difficulty::text=? AND l.deleted_at IS NULL))`, userID, outcome, outcome, difficulty, difficulty).Row().Scan(&total)
	if err != nil {
		return nil, false, 0, err
	}

	comparison, order := `a.id>NULLIF($2,'')::uuid`, `ASC`
	if direction == "backward" {
		comparison, order = `a.id<NULLIF($2,'')::uuid`, `DESC`
	}
	if boundary == "" {
		comparison = `$2::text IS NOT NULL`
	}
	rows, err := p.DB.WithContext(ctx).Raw(`SELECT `+attemptColumns+` FROM app.problem_attempts a LEFT JOIN app.enrollments e ON e.id=a.enrollment_id WHERE a.user_id=$1 AND `+comparison+` AND a.deleted_at IS NULL AND ($4='' OR a.outcome::text=$4) AND ($5='' OR EXISTS(SELECT 1 FROM app.leetcode_problems l WHERE l.problem_id=a.problem_id AND l.difficulty::text=$5 AND l.deleted_at IS NULL)) ORDER BY a.id `+order+` LIMIT $3`, userID, boundary, limit+1, outcome, difficulty).Rows()
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

func (p *Postgres) CreateAttempt(ctx context.Context, v practice.AttemptRecord) (practice.AttemptRecord, error) {
	tx, err := begin(ctx, p.DB)
	if err != nil {
		return practice.AttemptRecord{}, err
	}
	defer rollback(tx)

	var enrollmentID *string
	if v.SeasonID != nil {
		scopedEnrollmentID, err := resolveAttemptEnrollment(tx, v.UserID, *v.SeasonID)
		if err != nil {
			return practice.AttemptRecord{}, noRows(err)
		}
		enrollmentID = &scopedEnrollmentID
	}
	if v.WeekID != nil && v.SeasonID == nil {
		return practice.AttemptRecord{}, ErrNotFound
	}
	if v.WeekID != nil {
		valid, err := attemptWeekExists(tx, *v.WeekID, *v.SeasonID)
		if err != nil {
			return practice.AttemptRecord{}, err
		}
		if !valid {
			return practice.AttemptRecord{}, ErrNotFound
		}
	}
	baseProblemID, err := resolveBaseProblem(tx, v.ProblemID)
	if err != nil {
		return practice.AttemptRecord{}, noRows(err)
	}
	result := tx.Table(dbtable.ProblemAttempts).Create(map[string]any{
		"id": v.ID, "user_id": v.UserID, "problem_id": baseProblemID,
		"enrollment_id": enrollmentID, "season_week_id": v.WeekID,
		"attempted_at": v.AttemptedAt.UTC(), "time_taken_minutes": v.Minutes,
		"outcome": v.Outcome, "confidence": v.Confidence, "notes_html": v.Notes,
		"revision": v.Revision,
	})
	if result.Error != nil {
		return v, mapPostgresError(result.Error)
	}

	actorID := v.UserID
	if err := appendAuditTx(ctx, tx, audit.Event{ID: id.New(), ActorID: &actorID, Action: "attempt.created", SubjectType: "problem_attempt", SubjectID: v.ID, Data: map[string]any{}, OccurredAt: time.Now().UTC()}); err != nil {
		return v, err
	}
	if err := commit(tx); err != nil {
		return v, err
	}
	return v, nil
}

func (p *Postgres) UpdateAttempt(ctx context.Context, id, userID string, revision int64, fn func(*practice.AttemptRecord) error) (practice.AttemptRecord, error) {
	tx, err := begin(ctx, p.DB)
	if err != nil {
		return practice.AttemptRecord{}, err
	}

	defer rollback(tx)
	v, err := scanAttempt(tx.Raw(`SELECT `+attemptColumns+` FROM app.problem_attempts a
		LEFT JOIN app.enrollments e ON e.id=a.enrollment_id
		WHERE a.id=? AND a.user_id=? AND a.deleted_at IS NULL FOR UPDATE OF a`, id, userID).Row())
	if err != nil {
		return practice.AttemptRecord{}, err
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
		resolved, err := resolveAttemptEnrollment(tx, userID, *v.SeasonID)
		if err != nil {
			return v, ErrConflict
		}

		enrollmentID = &resolved
		if v.WeekID != nil {
			valid, err := attemptWeekExists(tx, *v.WeekID, *v.SeasonID)
			if err != nil || !valid {
				return v, ErrConflict
			}
		}
	}
	baseProblemID, err := resolveBaseProblem(tx, v.ProblemID)
	if err != nil {
		return v, noRows(err)
	}

	result := tx.Table(dbtable.ProblemAttempts).Where("id = ? AND user_id = ? AND revision = ?", id, userID, revision).
		Updates(map[string]any{
			"problem_id": baseProblemID, "enrollment_id": enrollmentID,
			"attempted_at": v.AttemptedAt.UTC(), "time_taken_minutes": v.Minutes,
			"outcome": v.Outcome, "confidence": v.Confidence, "notes_html": v.Notes,
			"season_week_id": v.WeekID, "revision": gorm.Expr("revision + 1"),
		})
	if result.Error != nil {
		return v, mapPostgresError(result.Error)
	}
	if result.RowsAffected == 0 {
		return v, ErrConflict
	}
	v.Revision++
	if err := appendAuditTx(ctx, tx, newAudit(userID, "attempt.updated", "problem_attempt", id, nil, time.Now())); err != nil {
		return v, err
	}
	if err := commit(tx); err != nil {
		return v, err
	}
	return v, nil
}

func (p *Postgres) DeleteAttempt(ctx context.Context, id, userID string, revision int64) error {
	tx, err := begin(ctx, p.DB)
	if err != nil {
		return err
	}

	defer rollback(tx)
	now := time.Now().UTC()
	result := tx.Table(dbtable.ProblemAttempts).
		Where("id = ? AND user_id = ? AND revision = ? AND deleted_at IS NULL", id, userID, revision).
		Updates(map[string]any{"deleted_at": now, "revision": gorm.Expr("revision + 1")})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		var count int64
		if err := tx.Table(dbtable.ProblemAttempts).Where("id = ? AND user_id = ? AND deleted_at IS NULL", id, userID).Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			return ErrConflict
		}
		return ErrNotFound
	}
	if err := appendAuditTx(ctx, tx, newAudit(userID, "attempt.deleted", "problem_attempt", id, nil, now)); err != nil {
		return err
	}
	return commit(tx)
}

func resolveAttemptEnrollment(tx *gorm.DB, userID, seasonID string) (string, error) {
	var enrollmentID string
	err := tx.Raw(`SELECT e.id FROM app.enrollments e JOIN app.seasons s ON s.id=e.season_id
		WHERE e.user_id=? AND e.season_id=? AND e.state='active' AND e.deleted_at IS NULL
		AND s.status='open' AND s.deleted_at IS NULL`, userID, seasonID).Row().Scan(&enrollmentID)
	return enrollmentID, err
}

func attemptWeekExists(tx *gorm.DB, weekID, seasonID string) (bool, error) {
	var count int64
	err := tx.Table(dbtable.SeasonWeeks).Where("id = ? AND season_id = ? AND deleted_at IS NULL", weekID, seasonID).Count(&count).Error
	return count > 0, err
}

func resolveBaseProblem(tx *gorm.DB, leetcodeProblemID string) (string, error) {
	var problemID string
	err := tx.Table(dbtable.LeetcodeProblems).Select("problem_id").Where("id = ? AND deleted_at IS NULL", leetcodeProblemID).Row().Scan(&problemID)
	return problemID, err
}
