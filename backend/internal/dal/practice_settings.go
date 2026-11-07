package dal

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/magedmg/RSP-website/backend/internal/practice"
)

// GetPracticeSettings returns the program's practice goals, which apply to
// every member. Personal settings are historical data only.
func (p *Store) GetPracticeSettings(ctx context.Context, userID string) (practice.PracticeSettings, error) {
	var storedID string
	if err := p.pool.QueryRow(ctx, `SELECT id FROM app.users WHERE id=$1 AND deleted_at IS NULL`, userID).Scan(&storedID); err != nil {
		return practice.PracticeSettings{}, noRows(err)
	}
	settings, err := p.programPracticeSettings(ctx)
	if err != nil {
		return settings, err
	}
	// A student's own requirements, from their latest active enrollment,
	// replace the program's defaults.
	var own practice.StudentRequirements
	err = p.pool.QueryRow(ctx, `SELECT r.easy_minutes,r.medium_minutes,r.hard_minutes,r.weekly_hours
		FROM app.student_requirements r JOIN app.enrollments e ON e.id=r.enrollment_id
		WHERE e.user_id=$1 AND e.role='student' AND e.state='active' AND e.deleted_at IS NULL
		ORDER BY e.created_at DESC LIMIT 1`, userID).Scan(&own.EasyMinutes, &own.MediumMinutes, &own.HardMinutes, &own.WeeklyHours)
	if errors.Is(err, pgx.ErrNoRows) {
		return settings, nil
	}
	if err != nil {
		return settings, err
	}
	return own.Apply(settings), nil
}

// GetStudentRequirements returns a student enrollment's own requirements;
// unset values are nil.
func (p *Store) GetStudentRequirements(ctx context.Context, seasonID, enrollmentID string) (practice.StudentRequirements, error) {
	var own practice.StudentRequirements
	var found bool
	if err := p.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM app.enrollments WHERE id=$1 AND season_id=$2 AND role='student' AND deleted_at IS NULL)`,
		enrollmentID, seasonID).Scan(&found); err != nil {
		return own, err
	}
	if !found {
		return own, ErrNotFound
	}
	err := p.pool.QueryRow(ctx, `SELECT easy_minutes,medium_minutes,hard_minutes,weekly_hours
		FROM app.student_requirements WHERE enrollment_id=$1`,
		enrollmentID).Scan(&own.EasyMinutes, &own.MediumMinutes, &own.HardMinutes, &own.WeeklyHours)
	if errors.Is(err, pgx.ErrNoRows) {
		return own, nil
	}
	return own, err
}

// SetStudentRequirements replaces a student enrollment's own requirements.
func (p *Store) SetStudentRequirements(ctx context.Context, seasonID, enrollmentID string, own practice.StudentRequirements, actorID string, changedAt time.Time) (practice.StudentRequirements, error) {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return own, err
	}
	defer tx.Rollback(context.Background())
	var found bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM app.enrollments WHERE id=$1 AND season_id=$2 AND role='student' AND deleted_at IS NULL)`,
		enrollmentID, seasonID).Scan(&found); err != nil {
		return own, err
	}
	if !found {
		return own, ErrNotFound
	}
	if _, err := tx.Exec(ctx, `INSERT INTO app.student_requirements(enrollment_id,easy_minutes,medium_minutes,hard_minutes,weekly_hours,
			updated_by_user_id,updated_at)
		VALUES($1,$2,$3,$4,$5,$6,$7)
		ON CONFLICT (enrollment_id) DO UPDATE SET easy_minutes=EXCLUDED.easy_minutes,medium_minutes=EXCLUDED.medium_minutes,
			hard_minutes=EXCLUDED.hard_minutes,weekly_hours=EXCLUDED.weekly_hours,
			updated_by_user_id=EXCLUDED.updated_by_user_id,updated_at=EXCLUDED.updated_at`,
		enrollmentID, own.EasyMinutes, own.MediumMinutes, own.HardMinutes, own.WeeklyHours, actorID, changedAt); err != nil {
		return own, mapDatabaseError(err)
	}
	if err := appendAuditTx(ctx, tx, newAudit(actorID, "student_requirements.updated", "enrollment", enrollmentID, nil, changedAt)); err != nil {
		return own, err
	}
	return own, tx.Commit(ctx)
}

func (p *Store) programPracticeSettings(ctx context.Context) (practice.PracticeSettings, error) {
	settings := practice.PracticeSettings{GoalsEnabled: true}
	err := p.pool.QueryRow(ctx, `SELECT easy_minutes,medium_minutes,hard_minutes,weekly_hours
		FROM app.program_practice_goals`).
		Scan(&settings.EasyMinutes, &settings.MediumMinutes, &settings.HardMinutes, &settings.WeeklyHours)
	if errors.Is(err, pgx.ErrNoRows) {
		// Nobody has set them yet.
		return practice.DefaultSettings(), nil
	}
	return settings, err
}

// SetPracticeSettings changes the program's practice goals for everyone.
func (p *Store) SetPracticeSettings(ctx context.Context, actorID string, settings practice.PracticeSettings, changedAt time.Time) (practice.PracticeSettings, error) {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return practice.PracticeSettings{}, err
	}
	defer tx.Rollback(context.Background())
	if _, err := tx.Exec(ctx, `INSERT INTO app.program_practice_goals(easy_minutes,medium_minutes,hard_minutes,weekly_hours,
			updated_by_user_id,updated_at)
		VALUES($1,$2,$3,$4,$5,$6)
		ON CONFLICT (singleton) DO UPDATE SET easy_minutes=EXCLUDED.easy_minutes,medium_minutes=EXCLUDED.medium_minutes,
			hard_minutes=EXCLUDED.hard_minutes,weekly_hours=EXCLUDED.weekly_hours,
			updated_by_user_id=EXCLUDED.updated_by_user_id,updated_at=EXCLUDED.updated_at`,
		settings.EasyMinutes, settings.MediumMinutes, settings.HardMinutes, settings.WeeklyHours, actorID, changedAt); err != nil {
		return practice.PracticeSettings{}, mapDatabaseError(err)
	}
	if err := appendAuditTx(ctx, tx, newAudit(actorID, "practice_goals.updated", "program", "practice_goals", map[string]any{
		"easyMinutes": settings.EasyMinutes, "mediumMinutes": settings.MediumMinutes, "hardMinutes": settings.HardMinutes,
		"weeklyHours": settings.WeeklyHours,
	}, changedAt)); err != nil {
		return practice.PracticeSettings{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return practice.PracticeSettings{}, err
	}
	return p.programPracticeSettings(ctx)
}
