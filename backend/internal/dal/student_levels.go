package dal

import (
	"context"
	"time"

	"github.com/magedmg/RSP-website/backend/internal/platform/audit"
	"github.com/magedmg/RSP-website/backend/internal/platform/id"
	"github.com/magedmg/RSP-website/backend/internal/programme"
)

type UpdateStudentLevelInput struct {
	SeasonID, EnrollmentID, StudentLevel, ActorID string
	ChangedAt                                     time.Time
}

func (p *Store) UpdateStudentLevel(ctx context.Context, input UpdateStudentLevelInput) (programme.EnrollmentRecord, error) {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return programme.EnrollmentRecord{}, err
	}
	defer tx.Rollback(context.Background())
	// Lock the season first, as CloseSeason does, so a level change cannot
	// race with completing its enrollments.
	var status string
	if err := tx.QueryRow(ctx, `SELECT status FROM app.seasons WHERE id=$1 AND deleted_at IS NULL FOR UPDATE`, input.SeasonID).Scan(&status); err != nil {
		return programme.EnrollmentRecord{}, noRows(err)
	}
	if status != "open" {
		return programme.EnrollmentRecord{}, ErrConflict
	}
	member, err := scanEnrollment(tx.QueryRow(ctx, `SELECT `+enrollmentColumns+` FROM app.enrollments e JOIN app.seasons s ON s.id=e.season_id WHERE e.id=$1 AND e.season_id=$2 AND e.deleted_at IS NULL FOR UPDATE OF e`, input.EnrollmentID, input.SeasonID))
	if err != nil {
		return member, err
	}
	if member.Role != "student" || member.State != "active" {
		return member, ErrConflict
	}
	previous := member.StudentLevel
	if previous == input.StudentLevel {
		return member, tx.Commit(ctx)
	}
	if _, err := tx.Exec(ctx, `UPDATE app.enrollments SET student_level=$2 WHERE id=$1`, member.ID, input.StudentLevel); err != nil {
		return member, err
	}
	member.StudentLevel = input.StudentLevel
	if err := appendAuditTx(ctx, tx, audit.Event{
		ID: id.New(), ActorID: &input.ActorID, Action: "enrollment.student_level_updated",
		SubjectType: "enrollment", SubjectID: member.ID,
		Data:       map[string]any{"seasonId": input.SeasonID, "previousLevel": previous, "studentLevel": input.StudentLevel},
		OccurredAt: input.ChangedAt.UTC(),
	}); err != nil {
		return member, err
	}
	return member, tx.Commit(ctx)
}
