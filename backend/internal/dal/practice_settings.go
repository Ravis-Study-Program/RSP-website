package dal

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/magedmg/RSP-website/backend/internal/platform/audit"
	"github.com/magedmg/RSP-website/backend/internal/platform/id"
	"github.com/magedmg/RSP-website/backend/internal/practice"
)

func (p *Store) GetPracticeSettings(ctx context.Context, userID string) (practice.PracticeSettings, error) {
	var settings practice.PracticeSettings
	err := p.pool.QueryRow(ctx, `SELECT COALESCE(g.enabled,false),
		COALESCE(g.easy_minutes,10),COALESCE(g.medium_minutes,20),COALESCE(g.hard_minutes,45)
		FROM app.users u LEFT JOIN app.practice_goals g ON g.user_id=u.id
		WHERE u.id=$1 AND u.deleted_at IS NULL`, userID).Scan(
		&settings.GoalsEnabled, &settings.EasyMinutes, &settings.MediumMinutes,
		&settings.HardMinutes,
	)
	return settings, noRows(err)
}

type UpdatePracticeSettingsInput struct {
	UserID        string
	EasyMinutes   int
	MediumMinutes int
	HardMinutes   int
	ActorID       string
	ChangedAt     time.Time
}

func (p *Store) UpdatePracticeSettings(ctx context.Context, input UpdatePracticeSettingsInput) (practice.PracticeSettings, error) {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return practice.PracticeSettings{}, err
	}

	defer tx.Rollback(context.Background())
	if err := ensurePracticeGoals(ctx, tx, input.UserID); err != nil {
		return practice.PracticeSettings{}, err
	}
	if input.EasyMinutes < 1 || input.MediumMinutes < 1 || input.HardMinutes < 1 {
		return practice.PracticeSettings{}, errors.New("practice goals must be positive")
	}
	_, err = tx.Exec(ctx, `UPDATE app.practice_goals SET easy_minutes=$1,medium_minutes=$2,hard_minutes=$3 WHERE user_id = $4`, input.EasyMinutes, input.MediumMinutes, input.HardMinutes, input.UserID)
	if err != nil {
		return practice.PracticeSettings{}, err
	}
	settings := practice.PracticeSettings{
		GoalsEnabled:  true,
		EasyMinutes:   input.EasyMinutes,
		MediumMinutes: input.MediumMinutes,
		HardMinutes:   input.HardMinutes,
	}
	if err := tx.QueryRow(ctx, `SELECT enabled FROM app.practice_goals WHERE user_id = $1`, input.UserID).Scan(&settings.GoalsEnabled); err != nil {
		return practice.PracticeSettings{}, err
	}

	if err := appendAuditTx(ctx, tx, audit.Event{
		ID:          id.New(),
		ActorID:     &input.ActorID,
		Action:      "practice_settings.updated",
		SubjectType: "user",
		SubjectID:   input.UserID,
		Data:        map[string]any{"easyMinutes": input.EasyMinutes, "mediumMinutes": input.MediumMinutes, "hardMinutes": input.HardMinutes},
		OccurredAt:  input.ChangedAt.UTC(),
	}); err != nil {
		return practice.PracticeSettings{}, err
	}
	return settings, tx.Commit(ctx)
}

type EnablePracticeGoalsInput struct {
	UserID    string
	ActorID   string
	SeasonID  string
	ChangedAt time.Time
}

func (p *Store) EnablePracticeGoals(ctx context.Context, input EnablePracticeGoalsInput) (practice.PracticeSettings, error) {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return practice.PracticeSettings{}, err
	}

	defer tx.Rollback(context.Background())
	if err := ensurePracticeGoals(ctx, tx, input.UserID); err != nil {
		return practice.PracticeSettings{}, err
	}
	var stored struct {
		Enabled                                 bool
		EasyMinutes, MediumMinutes, HardMinutes int
	}
	if err := tx.QueryRow(ctx, `SELECT enabled,easy_minutes,medium_minutes,hard_minutes FROM app.practice_goals WHERE user_id=$1 FOR UPDATE`, input.UserID).Scan(&stored.Enabled, &stored.EasyMinutes, &stored.MediumMinutes, &stored.HardMinutes); err != nil {
		return practice.PracticeSettings{}, noRows(err)
	}
	changed := !stored.Enabled
	if changed {
		_, err := tx.Exec(ctx, `UPDATE app.practice_goals SET enabled=$1,enabled_by_user_id=$2,enabled_at=$3 WHERE user_id = $4`, true, input.ActorID, input.ChangedAt.UTC(), input.UserID)
		if err != nil {
			return practice.PracticeSettings{}, err
		}
		stored.Enabled = true
	}
	if changed {
		if err := appendAuditTx(ctx, tx, audit.Event{
			ID:          id.New(),
			ActorID:     &input.ActorID,
			Action:      "practice_goals.enabled",
			SubjectType: "user",
			SubjectID:   input.UserID,
			Data:        map[string]any{"seasonId": input.SeasonID},
			OccurredAt:  input.ChangedAt.UTC(),
		}); err != nil {
			return practice.PracticeSettings{}, err
		}
	}
	settings := practice.PracticeSettings{
		GoalsEnabled:  stored.Enabled,
		EasyMinutes:   stored.EasyMinutes,
		MediumMinutes: stored.MediumMinutes,
		HardMinutes:   stored.HardMinutes,
	}
	return settings, tx.Commit(ctx)
}

func ensurePracticeGoals(ctx context.Context, tx pgx.Tx, userID string) error {
	_, err := tx.Exec(ctx, `INSERT INTO app.practice_goals(user_id,enabled,easy_minutes,medium_minutes,hard_minutes)
	VALUES($1,$2,$3,$4,$5) ON CONFLICT DO NOTHING`, userID, false, 10, 20, 45)
	return err
}
