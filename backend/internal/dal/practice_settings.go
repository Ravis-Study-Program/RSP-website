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
	return p.programPracticeSettings(ctx)
}

func (p *Store) programPracticeSettings(ctx context.Context) (practice.PracticeSettings, error) {
	settings := practice.PracticeSettings{GoalsEnabled: true}
	err := p.pool.QueryRow(ctx, `SELECT easy_minutes,medium_minutes,hard_minutes FROM app.program_practice_goals`).
		Scan(&settings.EasyMinutes, &settings.MediumMinutes, &settings.HardMinutes)
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
	if _, err := tx.Exec(ctx, `INSERT INTO app.program_practice_goals(easy_minutes,medium_minutes,hard_minutes,updated_by_user_id,updated_at)
		VALUES($1,$2,$3,$4,$5)
		ON CONFLICT (singleton) DO UPDATE SET easy_minutes=EXCLUDED.easy_minutes,medium_minutes=EXCLUDED.medium_minutes,
			hard_minutes=EXCLUDED.hard_minutes,updated_by_user_id=EXCLUDED.updated_by_user_id,updated_at=EXCLUDED.updated_at`,
		settings.EasyMinutes, settings.MediumMinutes, settings.HardMinutes, actorID, changedAt); err != nil {
		return practice.PracticeSettings{}, mapDatabaseError(err)
	}
	if err := appendAuditTx(ctx, tx, newAudit(actorID, "practice_goals.updated", "program", "practice_goals", map[string]any{
		"easyMinutes": settings.EasyMinutes, "mediumMinutes": settings.MediumMinutes, "hardMinutes": settings.HardMinutes,
	}, changedAt)); err != nil {
		return practice.PracticeSettings{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return practice.PracticeSettings{}, err
	}
	return p.programPracticeSettings(ctx)
}
