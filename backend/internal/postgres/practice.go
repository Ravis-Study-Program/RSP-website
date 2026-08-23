package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/magedmg/RSP-website/backend/internal/platform/id"
	"github.com/magedmg/RSP-website/backend/internal/practice"
)

func (p *Postgres) GetPracticeSettings(ctx context.Context, userID string) (practice.PracticeSettings, error) {
	var settings practice.PracticeSettings
	err := p.Pool.QueryRow(ctx, `
SELECT u.leetcode_premium_opt_in,
       COALESCE(g.enabled,false), COALESCE(g.easy_minutes,20),
       COALESCE(g.medium_minutes,35), COALESCE(g.hard_minutes,50),
       COALESCE(g.revision,1)
FROM app.users u
LEFT JOIN app.practice_goals g ON g.user_id=u.id
WHERE u.id=$1 AND u.deleted_at IS NULL`, userID).Scan(
		&settings.PremiumOptIn, &settings.GoalsEnabled, &settings.EasyMinutes,
		&settings.MediumMinutes, &settings.HardMinutes, &settings.Revision,
	)
	return settings, noRows(err)
}

// UpdatePracticeSettings updates a value.
func (p *Postgres) UpdatePracticeSettings(ctx context.Context, userID string, revision int64, premium bool, easy, medium, hard int, actorID string, at time.Time) (practice.PracticeSettings, error) {
	tx, err := p.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return practice.PracticeSettings{}, err
	}

	defer tx.Rollback(ctx)
	var currentRevision int64
	var enabled bool
	err = tx.QueryRow(ctx, `
INSERT INTO app.practice_goals(user_id,enabled,easy_minutes,medium_minutes,hard_minutes,revision)
VALUES($1,false,20,35,50,1)
ON CONFLICT(user_id) DO UPDATE SET user_id=EXCLUDED.user_id
RETURNING enabled,revision`, userID).Scan(&enabled, &currentRevision)
	if err != nil {
		return practice.PracticeSettings{}, err
	}
	if currentRevision != revision {
		return practice.PracticeSettings{}, ErrConflict
	}
	if easy < 1 || medium < 1 || hard < 1 {
		return practice.PracticeSettings{}, errors.New("practice goals must be positive")
	}
	if _, err = tx.Exec(ctx, `UPDATE app.users SET leetcode_premium_opt_in=$2,revision=revision+1 WHERE id=$1 AND deleted_at IS NULL`, userID, premium); err != nil {
		return practice.PracticeSettings{}, err
	}

	var settings practice.PracticeSettings
	settings.PremiumOptIn = premium
	err = tx.QueryRow(ctx, `
UPDATE app.practice_goals
SET easy_minutes=$2,medium_minutes=$3,hard_minutes=$4,revision=revision+1
WHERE user_id=$1 AND revision=$5
RETURNING enabled,easy_minutes,medium_minutes,hard_minutes,revision`, userID, easy, medium, hard, revision).Scan(
		&settings.GoalsEnabled, &settings.EasyMinutes, &settings.MediumMinutes, &settings.HardMinutes, &settings.Revision,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return practice.PracticeSettings{}, ErrConflict
	}
	if err != nil {
		return practice.PracticeSettings{}, err
	}

	data, _ := json.Marshal(map[string]any{"premiumOptIn": premium, "easyMinutes": easy, "mediumMinutes": medium, "hardMinutes": hard})
	if _, err = tx.Exec(ctx, `INSERT INTO app.audit_events(id,actor_user_id,action,subject_type,subject_id,data,occurred_at) VALUES($1,$2,'practice_settings.updated','user',$3,$4,$5)`, id.New(), actorID, userID, data, at.UTC()); err != nil {
		return practice.PracticeSettings{}, err
	}
	return settings, tx.Commit(ctx)
}

// EnablePracticeGoals performs the operation.
func (p *Postgres) EnablePracticeGoals(ctx context.Context, userID string, revision int64, actorID, seasonID string, at time.Time) (practice.PracticeSettings, error) {
	tx, err := p.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return practice.PracticeSettings{}, err
	}

	defer tx.Rollback(ctx)
	var settings practice.PracticeSettings
	changed := true
	err = tx.QueryRow(ctx, `
INSERT INTO app.practice_goals(user_id,enabled,enabled_by_user_id,enabled_at,easy_minutes,medium_minutes,hard_minutes,revision)
SELECT $1,true,$2,$3,20,35,50,2 WHERE $4=1
ON CONFLICT(user_id) DO UPDATE
SET enabled=true, enabled_by_user_id=EXCLUDED.enabled_by_user_id, enabled_at=EXCLUDED.enabled_at, revision=app.practice_goals.revision+1
WHERE NOT app.practice_goals.enabled AND app.practice_goals.revision=$4
RETURNING enabled,easy_minutes,medium_minutes,hard_minutes,revision`, userID, actorID, at.UTC(), revision).Scan(
		&settings.GoalsEnabled, &settings.EasyMinutes, &settings.MediumMinutes, &settings.HardMinutes, &settings.Revision,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		changed = false
		err = tx.QueryRow(ctx, `SELECT enabled,easy_minutes,medium_minutes,hard_minutes,revision FROM app.practice_goals WHERE user_id=$1`, userID).Scan(&settings.GoalsEnabled, &settings.EasyMinutes, &settings.MediumMinutes, &settings.HardMinutes, &settings.Revision)
		if errors.Is(err, pgx.ErrNoRows) {
			return practice.PracticeSettings{}, ErrConflict
		}
		if err == nil && settings.Revision != revision {
			return practice.PracticeSettings{}, ErrConflict
		}
	}
	if err != nil {
		return practice.PracticeSettings{}, noRows(err)
	}
	if err = tx.QueryRow(ctx, `SELECT leetcode_premium_opt_in FROM app.users WHERE id=$1`, userID).Scan(&settings.PremiumOptIn); err != nil {
		return practice.PracticeSettings{}, noRows(err)
	}
	if changed {
		if _, err = tx.Exec(ctx, `INSERT INTO app.audit_events(id,actor_user_id,action,subject_type,subject_id,data,occurred_at) VALUES($1,$2,'practice_goals.enabled','user',$3,jsonb_build_object('seasonId',$4::text),$5)`, id.New(), actorID, userID, seasonID, at.UTC()); err != nil {
			return practice.PracticeSettings{}, err
		}
	}
	return settings, tx.Commit(ctx)
}
