package dal

import (
	"context"
	"time"
)

func (p *Store) IsMentorAssignedAt(ctx context.Context, seasonID, mentorID, studentID string, at time.Time) (bool, error) {
	var assigned bool
	err := p.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM app.mentorships m
 JOIN app.enrollments mentor ON mentor.id=m.mentor_enrollment_id
 JOIN app.enrollments student ON student.id=m.student_enrollment_id
 WHERE m.season_id=$1 AND mentor.user_id=$2 AND student.user_id=$3
 AND m.created_at<=$4 AND (m.ended_at IS NULL OR $4<m.ended_at) AND m.deleted_at IS NULL)`, seasonID, mentorID, studentID, at).Scan(&assigned)
	return assigned, err
}
