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
		COALESCE(g.easy_minutes,20),COALESCE(g.medium_minutes,35),COALESCE(g.hard_minutes,50),
		COALESCE(g.revision,1)
		FROM app.users u LEFT JOIN app.practice_goals g ON g.user_id=u.id
		WHERE u.id=$1 AND u.deleted_at IS NULL`, userID).Scan(
		&settings.GoalsEnabled, &settings.EasyMinutes, &settings.MediumMinutes,
		&settings.HardMinutes, &settings.Revision,
	)
	return settings, noRows(err)
}

func (p *Store) UpdatePracticeSettings(ctx context.Context, userID string, revision int64, easy, medium, hard int, actorID string, at time.Time) (practice.PracticeSettings, error) {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return practice.PracticeSettings{}, err
	}

	defer tx.Rollback(context.Background())
	if err := ensurePracticeGoals(ctx, tx, userID); err != nil {
		return practice.PracticeSettings{}, err
	}
	var currentRevision int64
	if err := tx.QueryRow(ctx, `SELECT revision FROM app.practice_goals WHERE user_id = $1 FOR UPDATE`, userID).Scan(&currentRevision); err != nil {
		return practice.PracticeSettings{}, err
	}
	if currentRevision != revision {
		return practice.PracticeSettings{}, ErrConflict
	}
	if easy < 1 || medium < 1 || hard < 1 {
		return practice.PracticeSettings{}, errors.New("practice goals must be positive")
	}
	result, err := tx.Exec(ctx, `UPDATE app.practice_goals SET easy_minutes=$1,medium_minutes=$2,hard_minutes=$3,revision=revision + 1 WHERE user_id = $4 AND revision = $5`, easy, medium, hard, userID, revision)
	if err != nil {
		return practice.PracticeSettings{}, err
	}
	if result.RowsAffected() == 0 {
		return practice.PracticeSettings{}, ErrConflict
	}
	settings := practice.PracticeSettings{GoalsEnabled: true, EasyMinutes: easy, MediumMinutes: medium, HardMinutes: hard, Revision: revision + 1}
	if err := tx.QueryRow(ctx, `SELECT enabled FROM app.practice_goals WHERE user_id = $1`, userID).Scan(&settings.GoalsEnabled); err != nil {
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
	return settings, tx.Commit(ctx)
}

func (p *Store) EnablePracticeGoals(ctx context.Context, userID string, revision int64, actorID, seasonID string, at time.Time) (practice.PracticeSettings, error) {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return practice.PracticeSettings{}, err
	}

	defer tx.Rollback(context.Background())
	if err := ensurePracticeGoals(ctx, tx, userID); err != nil {
		return practice.PracticeSettings{}, err
	}
	var stored struct {
		Enabled                                 bool
		EasyMinutes, MediumMinutes, HardMinutes int
		Revision                                int64
	}
	if err := tx.QueryRow(ctx, `SELECT enabled,easy_minutes,medium_minutes,hard_minutes,revision FROM app.practice_goals WHERE user_id=$1 FOR UPDATE`, userID).Scan(&stored.Enabled, &stored.EasyMinutes, &stored.MediumMinutes, &stored.HardMinutes, &stored.Revision); err != nil {
		return practice.PracticeSettings{}, noRows(err)
	}
	if stored.Revision != revision {
		return practice.PracticeSettings{}, ErrConflict
	}
	changed := !stored.Enabled
	if changed {
		result, err := tx.Exec(ctx, `UPDATE app.practice_goals SET enabled=$1,enabled_by_user_id=$2,enabled_at=$3,revision=revision + 1 WHERE user_id = $4 AND revision = $5`, true, actorID, at.UTC(), userID, revision)
		if err != nil {
			return practice.PracticeSettings{}, err
		}
		if result.RowsAffected() == 0 {
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
	return settings, tx.Commit(ctx)
}

func ensurePracticeGoals(ctx context.Context, tx pgx.Tx, userID string) error {
	_, err := tx.Exec(ctx, `INSERT INTO app.practice_goals(user_id,enabled,easy_minutes,medium_minutes,hard_minutes,revision)
 VALUES($1,$2,$3,$4,$5,$6) ON CONFLICT DO NOTHING`, userID, false, 20, 35, 50, 1)
	return err
}
