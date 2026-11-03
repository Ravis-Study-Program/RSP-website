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

// GetPracticeSettings returns a member's student override, or the programme
// goals when none is set. Legacy personal goals in app.practice_goals are
// historical data only and never apply.
func (p *Store) GetPracticeSettings(ctx context.Context, userID string) (practice.PracticeSettings, error) {
	return practiceSettings(ctx, p.pool, userID)
}

func practiceSettings(ctx context.Context, db queryer, userID string) (practice.PracticeSettings, error) {
	defaults := practice.DefaultGoals()
	settings := practice.PracticeSettings{GoalsEnabled: true}
	var override bool
	err := db.QueryRow(ctx, `SELECT
		COALESCE(s.easy_minutes, g.easy_minutes, $2),
		COALESCE(s.medium_minutes, g.medium_minutes, $3),
		COALESCE(s.hard_minutes, g.hard_minutes, $4),
		s.user_id IS NOT NULL
	FROM app.users u
	LEFT JOIN app.programme_practice_goals g ON true
	LEFT JOIN app.student_practice_goals s ON s.user_id=u.id
	WHERE u.id=$1 AND u.deleted_at IS NULL`, userID, defaults.EasyMinutes, defaults.MediumMinutes, defaults.HardMinutes).
		Scan(&settings.EasyMinutes, &settings.MediumMinutes, &settings.HardMinutes, &override)
	if err != nil {
		return practice.PracticeSettings{}, noRows(err)
	}
	settings.Source = practice.GoalSourceProgramme
	if override {
		settings.Source = practice.GoalSourceStudent
	}
	return settings, nil
}

// GetProgrammePracticeGoals returns the goals that apply to every member
// without a student override.
func (p *Store) GetProgrammePracticeGoals(ctx context.Context) (practice.Goals, error) {
	return programmeGoals(ctx, p.pool)
}

func programmeGoals(ctx context.Context, db queryer) (practice.Goals, error) {
	goals := practice.DefaultGoals()
	err := db.QueryRow(ctx, `SELECT easy_minutes,medium_minutes,hard_minutes FROM app.programme_practice_goals`).
		Scan(&goals.EasyMinutes, &goals.MediumMinutes, &goals.HardMinutes)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return practice.Goals{}, err
	}
	return goals, nil
}

type UpdateProgrammePracticeGoalsInput struct {
	Goals     practice.Goals
	ActorID   string
	ChangedAt time.Time
}

func (p *Store) UpdateProgrammePracticeGoals(ctx context.Context, input UpdateProgrammePracticeGoalsInput) (practice.Goals, error) {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return practice.Goals{}, err
	}
	defer tx.Rollback(context.Background())
	// Serialise edits so each audit event records the goals it replaced.
	if _, err := tx.Exec(ctx, `LOCK TABLE app.programme_practice_goals IN SHARE ROW EXCLUSIVE MODE`); err != nil {
		return practice.Goals{}, err
	}
	previous, err := programmeGoals(ctx, tx)
	if err != nil {
		return practice.Goals{}, err
	}
	if previous == input.Goals {
		return previous, tx.Commit(ctx)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO app.programme_practice_goals(easy_minutes,medium_minutes,hard_minutes,updated_by_user_id)
		VALUES($1,$2,$3,$4)
		ON CONFLICT (singleton) DO UPDATE SET easy_minutes=EXCLUDED.easy_minutes, medium_minutes=EXCLUDED.medium_minutes,
			hard_minutes=EXCLUDED.hard_minutes, updated_by_user_id=EXCLUDED.updated_by_user_id`,
		input.Goals.EasyMinutes, input.Goals.MediumMinutes, input.Goals.HardMinutes, input.ActorID); err != nil {
		return practice.Goals{}, err
	}
	if err := appendAuditTx(ctx, tx, audit.Event{
		ID: id.New(), ActorID: &input.ActorID, Action: "practice_goals.programme_updated",
		SubjectType: "programme_practice_goals", SubjectID: "programme",
		Data:       map[string]any{"previous": previous, "goals": input.Goals},
		OccurredAt: input.ChangedAt.UTC(),
	}); err != nil {
		return practice.Goals{}, err
	}
	return input.Goals, tx.Commit(ctx)
}

type SetStudentPracticeGoalsInput struct {
	SeasonID, EnrollmentID, ActorID string
	// Goals replaces the student's override; nil clears it so the programme
	// goals apply again.
	Goals     *practice.Goals
	ChangedAt time.Time
}

// SetStudentPracticeGoals changes an active student's override in an open
// season. Callers check that the actor may manage this student first.
func (p *Store) SetStudentPracticeGoals(ctx context.Context, input SetStudentPracticeGoalsInput) (practice.PracticeSettings, error) {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return practice.PracticeSettings{}, err
	}
	defer tx.Rollback(context.Background())
	// Lock the season first, as UpdateStudentLevel does, so the change cannot
	// race with closing the season.
	var status string
	if err := tx.QueryRow(ctx, `SELECT status FROM app.seasons WHERE id=$1 AND deleted_at IS NULL FOR SHARE`, input.SeasonID).Scan(&status); err != nil {
		return practice.PracticeSettings{}, noRows(err)
	}
	if status != "open" {
		return practice.PracticeSettings{}, ErrConflict
	}
	var studentID, role, state string
	if err := tx.QueryRow(ctx, `SELECT user_id,role,state FROM app.enrollments WHERE id=$1 AND season_id=$2 AND deleted_at IS NULL FOR SHARE`,
		input.EnrollmentID, input.SeasonID).Scan(&studentID, &role, &state); err != nil {
		return practice.PracticeSettings{}, noRows(err)
	}
	if role != "student" || state != "active" {
		return practice.PracticeSettings{}, ErrConflict
	}
	previous, err := practiceSettings(ctx, tx, studentID)
	if err != nil {
		return practice.PracticeSettings{}, err
	}
	action := "practice_goals.student_cleared"
	if input.Goals == nil {
		if previous.Source != practice.GoalSourceStudent {
			return previous, tx.Commit(ctx)
		}
		if _, err := tx.Exec(ctx, `DELETE FROM app.student_practice_goals WHERE user_id=$1`, studentID); err != nil {
			return practice.PracticeSettings{}, err
		}
	} else {
		if previous.Source == practice.GoalSourceStudent && previous.Goals == *input.Goals {
			return previous, tx.Commit(ctx)
		}
		action = "practice_goals.student_updated"
		if _, err := tx.Exec(ctx, `INSERT INTO app.student_practice_goals(user_id,easy_minutes,medium_minutes,hard_minutes,set_by_user_id)
			VALUES($1,$2,$3,$4,$5)
			ON CONFLICT (user_id) DO UPDATE SET easy_minutes=EXCLUDED.easy_minutes, medium_minutes=EXCLUDED.medium_minutes,
				hard_minutes=EXCLUDED.hard_minutes, set_by_user_id=EXCLUDED.set_by_user_id`,
			studentID, input.Goals.EasyMinutes, input.Goals.MediumMinutes, input.Goals.HardMinutes, input.ActorID); err != nil {
			return practice.PracticeSettings{}, err
		}
	}
	settings, err := practiceSettings(ctx, tx, studentID)
	if err != nil {
		return practice.PracticeSettings{}, err
	}
	if err := appendAuditTx(ctx, tx, audit.Event{
		ID: id.New(), ActorID: &input.ActorID, Action: action,
		SubjectType: "user", SubjectID: studentID,
		Data:       map[string]any{"seasonId": input.SeasonID, "enrollmentId": input.EnrollmentID, "previous": previous, "settings": settings},
		OccurredAt: input.ChangedAt.UTC(),
	}); err != nil {
		return practice.PracticeSettings{}, err
	}
	return settings, tx.Commit(ctx)
}
