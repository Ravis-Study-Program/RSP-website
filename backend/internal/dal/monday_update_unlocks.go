package dal

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/magedmg/RSP-website/backend/internal/program"
)

const mondayUpdateUnlockColumns = `k.id,k.season_id,k.student_user_id,s.slug,s.display_name,s.avatar_url,
	k.start_date::text,k.end_date::text,k.reason,k.state::text,k.requested_at,k.decided_at,
	d.user_id,d.slug,d.display_name,d.avatar_url`

const mondayUpdateUnlockFrom = `FROM app.monday_update_unlocks k
	JOIN app.user_profiles s ON s.user_id=k.student_user_id
	LEFT JOIN app.user_profiles d ON d.user_id=k.decided_by_user_id`

func scanMondayUpdateUnlock(row pgx.Row) (program.MondayUpdateUnlock, error) {
	var v program.MondayUpdateUnlock
	var deciderID, deciderSlug, deciderName, deciderAvatar *string
	err := row.Scan(&v.ID, &v.SeasonID, &v.Student.ID, &v.Student.Slug, &v.Student.Name, &v.Student.AvatarURL,
		&v.StartDate, &v.EndDate, &v.Reason, &v.State, &v.RequestedAt, &v.DecidedAt,
		&deciderID, &deciderSlug, &deciderName, &deciderAvatar)
	if err != nil {
		return v, noRows(err)
	}
	if deciderID != nil {
		v.DecidedBy = &program.PersonSummary{ID: *deciderID, Slug: deref(deciderSlug), Name: deref(deciderName), AvatarURL: deciderAvatar}
	}
	return v, nil
}

func deref(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

// CreateMondayUpdateUnlock records a student's request to unlock a late
// program week. A week holds one open request or unlock at a time.
func (p *Store) CreateMondayUpdateUnlock(ctx context.Context, v program.MondayUpdateUnlock, actorID string) (program.MondayUpdateUnlock, error) {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return v, err
	}
	defer tx.Rollback(context.Background())
	if err := lockOpenSeason(ctx, tx, v.SeasonID); err != nil {
		return v, err
	}
	var configured, open bool
	if err := tx.QueryRow(ctx, `SELECT
		EXISTS(SELECT 1 FROM app.season_weeks WHERE season_id=$1 AND deleted_at IS NULL
			AND date_trunc('week', start_at AT TIME ZONE 'Australia/Adelaide')::date=$2::date),
		EXISTS(SELECT 1 FROM app.monday_update_unlocks WHERE season_id=$1 AND student_user_id=$3
			AND start_date=$2::date AND state IN ('pending','approved'))`,
		v.SeasonID, v.StartDate, v.Student.ID).Scan(&configured, &open); err != nil {
		return v, err
	}
	if !configured {
		return v, ErrMondayUpdateWeek
	}
	if open {
		return v, ErrMondayUpdateUnlockOpen
	}
	if _, err := tx.Exec(ctx, `INSERT INTO app.monday_update_unlocks(id,season_id,student_user_id,start_date,end_date,reason)
		VALUES($1,$2,$3,$4::date,$5::date,$6)`, v.ID, v.SeasonID, v.Student.ID, v.StartDate, v.EndDate, v.Reason); err != nil {
		return v, mapDatabaseError(err)
	}
	if err := appendAuditTx(ctx, tx, newAudit(actorID, "monday_update_unlock.requested", "monday_update_unlock", v.ID, nil, time.Now())); err != nil {
		return v, err
	}
	if err := tx.Commit(ctx); err != nil {
		return v, err
	}
	return p.GetMondayUpdateUnlock(ctx, v.SeasonID, v.ID)
}

func (p *Store) GetMondayUpdateUnlock(ctx context.Context, seasonID, unlockID string) (program.MondayUpdateUnlock, error) {
	return scanMondayUpdateUnlock(p.pool.QueryRow(ctx, `SELECT `+mondayUpdateUnlockColumns+` `+mondayUpdateUnlockFrom+`
		WHERE k.id=$1 AND k.season_id=$2`, unlockID, seasonID))
}

// MondayUpdateUnlockQuery picks whose requests to list: one student's, a
// mentor's current mentees', or (neither set) everyone's in the season.
type MondayUpdateUnlockQuery struct {
	SeasonID      string
	StudentUserID string
	MentorUserID  string
}

// ListMondayUpdateUnlocks returns requests newest first, pending ones first.
func (p *Store) ListMondayUpdateUnlocks(ctx context.Context, q MondayUpdateUnlockQuery) ([]program.MondayUpdateUnlock, error) {
	rows, err := p.pool.Query(ctx, `SELECT `+mondayUpdateUnlockColumns+` `+mondayUpdateUnlockFrom+`
		WHERE k.season_id=@seasonID
		  AND (@studentID::uuid IS NULL OR k.student_user_id=@studentID::uuid)
		  AND (@mentorID::uuid IS NULL OR EXISTS(SELECT 1 FROM app.mentorships m
			JOIN app.enrollments mentor ON mentor.id=m.mentor_enrollment_id
			JOIN app.enrollments student ON student.id=m.student_enrollment_id
			WHERE m.season_id=k.season_id AND mentor.user_id=@mentorID::uuid AND student.user_id=k.student_user_id
			  AND m.ended_at IS NULL AND m.deleted_at IS NULL))
		ORDER BY (k.state='pending') DESC,k.requested_at DESC,k.id`,
		pgx.NamedArgs{"seasonID": q.SeasonID, "studentID": nullableID(q.StudentUserID), "mentorID": nullableID(q.MentorUserID)})
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []program.MondayUpdateUnlock{}
	for rows.Next() {
		item, err := scanMondayUpdateUnlock(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func nullableID(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

// DecideMondayUpdateUnlock approves or declines a pending request.
func (p *Store) DecideMondayUpdateUnlock(ctx context.Context, seasonID, unlockID string, approve bool, actorID string) (program.MondayUpdateUnlock, error) {
	state := "declined"
	if approve {
		state = "approved"
	}
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return program.MondayUpdateUnlock{}, err
	}
	defer tx.Rollback(context.Background())
	result, err := tx.Exec(ctx, `UPDATE app.monday_update_unlocks
		SET state=$3::app.monday_update_unlock_state,decided_by_user_id=$4,decided_at=now()
		WHERE id=$1 AND season_id=$2 AND state='pending'`, unlockID, seasonID, state, actorID)
	if err != nil {
		return program.MondayUpdateUnlock{}, mapDatabaseError(err)
	}
	if result.RowsAffected() == 0 {
		return program.MondayUpdateUnlock{}, ErrConflict
	}
	if err := appendAuditTx(ctx, tx, newAudit(actorID, "monday_update_unlock."+state, "monday_update_unlock", unlockID, nil, time.Now())); err != nil {
		return program.MondayUpdateUnlock{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return program.MondayUpdateUnlock{}, err
	}
	return p.GetMondayUpdateUnlock(ctx, seasonID, unlockID)
}

// HasApprovedMondayUpdateUnlock reports whether a late week is unlocked for
// the student.
func (p *Store) HasApprovedMondayUpdateUnlock(ctx context.Context, seasonID, studentUserID, startDate string) (bool, error) {
	var approved bool
	err := p.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM app.monday_update_unlocks
		WHERE season_id=$1 AND student_user_id=$2 AND start_date=$3::date AND state='approved')`,
		seasonID, studentUserID, startDate).Scan(&approved)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	return approved, err
}

// UnlockMondayUpdate unlocks a student's late week for them: it approves
// their pending request, keeps an existing unlock, or records a new one.
func (p *Store) UnlockMondayUpdate(ctx context.Context, v program.MondayUpdateUnlock, actorID string) (program.MondayUpdateUnlock, error) {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return v, err
	}
	defer tx.Rollback(context.Background())
	var unlockID string
	err = tx.QueryRow(ctx, `UPDATE app.monday_update_unlocks
		SET state='approved',decided_by_user_id=$4,decided_at=COALESCE(decided_at,now())
		WHERE season_id=$1 AND student_user_id=$2 AND start_date=$3::date AND state IN ('pending','approved')
		RETURNING id`, v.SeasonID, v.Student.ID, v.StartDate, actorID).Scan(&unlockID)
	if errors.Is(err, pgx.ErrNoRows) {
		unlockID = v.ID
		_, err = tx.Exec(ctx, `INSERT INTO app.monday_update_unlocks(id,season_id,student_user_id,start_date,end_date,reason,state,decided_by_user_id,decided_at)
			VALUES($1,$2,$3,$4::date,$5::date,$6,'approved',$7,now())`, v.ID, v.SeasonID, v.Student.ID, v.StartDate, v.EndDate, v.Reason, actorID)
	}
	if err != nil {
		return v, mapDatabaseError(err)
	}
	if err := appendAuditTx(ctx, tx, newAudit(actorID, "monday_update_unlock.approved", "monday_update_unlock", unlockID, nil, time.Now())); err != nil {
		return v, err
	}
	if err := tx.Commit(ctx); err != nil {
		return v, err
	}
	return p.GetMondayUpdateUnlock(ctx, v.SeasonID, unlockID)
}

// MondayUpdateWeekConfigured reports whether a Monday is the start of one of
// the season's program weeks.
func (p *Store) MondayUpdateWeekConfigured(ctx context.Context, seasonID, startDate string) (bool, error) {
	var configured bool
	err := p.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM app.season_weeks WHERE season_id=$1 AND deleted_at IS NULL
		AND date_trunc('week', start_at AT TIME ZONE 'Australia/Adelaide')::date=$2::date)`, seasonID, startDate).Scan(&configured)
	return configured, err
}
