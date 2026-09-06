package dal

import (
	"context"
	"encoding/json"
	"github.com/magedmg/RSP-website/backend/internal/programme"
	"sort"
	"time"
)

type CorrectEnrollmentInput struct {
	EnrollmentID   string
	SourceSeasonID string
	SeasonID       string
	UserID         string
	ActorID        string
	ChangedAt      time.Time
}

// CorrectEnrollment retains the membership's participation periods. Activities
// continue to belong to their original recorder; their season is derived again.
func (p *Store) CorrectEnrollment(ctx context.Context, input CorrectEnrollmentInput) (programme.EnrollmentRecord, error) {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return programme.EnrollmentRecord{}, err
	}
	defer tx.Rollback(context.Background())
	seasons := []string{input.SourceSeasonID}
	if input.SeasonID != input.SourceSeasonID {
		seasons = append(seasons, input.SeasonID)
	}
	sort.Strings(seasons)
	for _, seasonID := range seasons {
		if err = lockOpenSeason(ctx, tx, seasonID); err != nil {
			return programme.EnrollmentRecord{}, err
		}
	}
	before, err := scanEnrollment(tx.QueryRow(ctx, `SELECT `+enrollmentColumns+` FROM app.enrollments e JOIN app.seasons s ON s.id=e.season_id WHERE e.id=$1 AND e.season_id=$2 AND (e.deleted_at IS NULL OR e.state IN ('kicked','withdrawn')) FOR UPDATE OF e`, input.EnrollmentID, input.SourceSeasonID))
	if err != nil {
		return before, err
	}
	if before.UserID == input.UserID && before.SeasonID == input.SeasonID {
		return before, nil
	}
	var eligible, duplicate bool
	err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM app.users u WHERE u.id=$1 AND u.account_state='active' AND u.deleted_at IS NULL AND EXISTS(SELECT 1 FROM app.user_auth_links l WHERE l.user_id=u.id AND l.active)),EXISTS(SELECT 1 FROM app.enrollments e WHERE e.user_id=$1 AND e.season_id=$2 AND e.id<>$3)`, input.UserID, input.SeasonID, input.EnrollmentID).Scan(&eligible, &duplicate)
	if err != nil {
		return before, err
	}
	if !eligible {
		return before, ErrConflict
	}
	if duplicate {
		return before, ErrDuplicate
	}
	var assignments json.RawMessage
	if err = tx.QueryRow(ctx, `SELECT COALESCE(jsonb_agg(to_jsonb(m)),'[]'::jsonb) FROM app.mentorships m WHERE (student_enrollment_id=$1 OR mentor_enrollment_id=$1) AND deleted_at IS NULL`, input.EnrollmentID).Scan(&assignments); err != nil {
		return before, err
	}
	if _, err = tx.Exec(ctx, `UPDATE app.mentorships SET ended_at=COALESCE(ended_at,$2),deleted_at=$2 WHERE (student_enrollment_id=$1 OR mentor_enrollment_id=$1) AND deleted_at IS NULL`, input.EnrollmentID, input.ChangedAt); err != nil {
		return before, err
	}
	// Legacy stored links are no longer authoritative and would otherwise refer
	// to the wrong person after a correction. Preserve their owners and timestamps.
	cleared, err := tx.Exec(ctx, `UPDATE app.problem_attempts SET enrollment_id=NULL,season_week_id=NULL WHERE enrollment_id=$1`, input.EnrollmentID)
	if err != nil {
		return before, err
	}
	after, err := scanEnrollment(tx.QueryRow(ctx, `UPDATE app.enrollments e SET user_id=$3,season_id=$4 FROM app.seasons s WHERE e.id=$1 AND e.season_id=$2 AND s.id=$4 RETURNING `+enrollmentColumns, input.EnrollmentID, input.SourceSeasonID, input.UserID, input.SeasonID))
	if err != nil {
		return before, mapDatabaseError(err)
	}
	if err = appendAuditTx(ctx, tx, newAudit(input.ActorID, "enrollment.corrected", "enrollment", input.EnrollmentID, map[string]any{"before": before, "after": after, "removedMentorships": assignments, "clearedLegacyAttemptLinks": cleared.RowsAffected()}, input.ChangedAt)); err != nil {
		return after, err
	}
	return after, tx.Commit(ctx)
}
