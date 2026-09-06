package dal

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/magedmg/RSP-website/backend/internal/platform/id"
	"github.com/magedmg/RSP-website/backend/internal/programme"
)

type MentorshipQuery struct {
	IncludeEnded  bool
	SeasonID      string
	Boundary      string
	Limit         int
	SortBy        string
	Direction     string
	MentorUserID  string
	StudentUserID string
}

func (p *Store) ListMentorships(ctx context.Context, q MentorshipQuery) ([]programme.MentorshipRecord, bool, int64, error) {
	const base = ` FROM app.mentorships m
		JOIN app.enrollments mentor ON mentor.id=m.mentor_enrollment_id
		JOIN app.enrollments student ON student.id=m.student_enrollment_id`
	const filters = `m.season_id = @seasonID AND (@includeEnded OR m.ended_at IS NULL) AND m.deleted_at IS NULL
		AND (@mentorUserID = '' OR mentor.user_id = NULLIF(@mentorUserID, '')::uuid)
		AND (@studentUserID = '' OR student.user_id = NULLIF(@studentUserID, '')::uuid)`
	args := pgx.NamedArgs{
		"seasonID":      q.SeasonID,
		"includeEnded":  q.IncludeEnded,
		"mentorUserID":  q.MentorUserID,
		"studentUserID": q.StudentUserID,
	}
	var total int64
	if err := p.pool.QueryRow(ctx, `SELECT count(*)`+base+` WHERE `+filters, args).Scan(&total); err != nil {
		return nil, false, 0, err
	}

	comparator, order := ">", "ASC"
	if q.Direction == "backward" {
		comparator, order = "<", "DESC"
	}
	key, boundaryKey := "m.id", "b.id"
	boundarySelect := "m.id"
	if q.SortBy == "student:asc" {
		key, boundaryKey = "ROW(student.user_id,m.id)", "ROW(b.student_user_id,b.id)"
		boundarySelect = "m.id,student.user_id AS student_user_id"
	}
	orderBy := "m.id " + order
	if q.SortBy == "student:asc" {
		orderBy = "student.user_id " + order + ",m.id " + order
	}
	query := `WITH boundary AS (
		SELECT ` + boundarySelect + base + ` WHERE m.id = NULLIF(@boundary, '')::uuid AND ` + filters + `
	) SELECT m.id,m.season_id,mentor.user_id,student.user_id,m.created_at,m.ended_at` + base + `
	WHERE ` + filters + `
	  AND (NULLIF(@boundary, '')::uuid IS NULL OR EXISTS (SELECT 1 FROM boundary b WHERE ` + key + ` ` + comparator + ` ` + boundaryKey + `))
	ORDER BY ` + orderBy + ` LIMIT @limit`
	args["boundary"] = q.Boundary
	args["limit"] = q.Limit + 1
	rows, err := p.pool.Query(ctx, query, args)
	if err != nil {
		return nil, false, 0, err
	}

	defer rows.Close()
	items := make([]programme.MentorshipRecord, 0, q.Limit+1)
	for rows.Next() {
		var v programme.MentorshipRecord
		if err := rows.Scan(&v.ID, &v.SeasonID, &v.MentorUserID, &v.StudentUserID, &v.CreatedAt, &v.EndedAt); err != nil {
			return nil, false, 0, err
		}

		items = append(items, v)
	}
	if err := rows.Err(); err != nil {
		return nil, false, 0, err
	}

	items, more := finishPage(items, q.Limit, q.Direction)
	return items, more, total, nil
}

func (p *Store) CreateMentorship(ctx context.Context, v programme.MentorshipRecord, actorID string, at time.Time) (programme.MentorshipRecord, error) {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return v, err
	}

	defer tx.Rollback(context.Background())
	_, err = tx.Exec(ctx, `INSERT INTO app.mentorships(id,season_id,mentor_enrollment_id,student_enrollment_id) VALUES($1,$2,(SELECT id FROM app.enrollments WHERE season_id=$2 AND user_id=$3 AND role IN ('mentor','coordinator') AND state='active' AND deleted_at IS NULL),(SELECT id FROM app.enrollments WHERE season_id=$2 AND user_id=$4 AND role='student' AND state='active' AND deleted_at IS NULL))`, v.ID, v.SeasonID, v.MentorUserID, v.StudentUserID)
	if err != nil {
		return v, mapDatabaseError(err)
	}
	if err := appendAuditTx(ctx, tx, newAudit(actorID, "mentorship.created", "mentorship", v.ID, nil, at)); err != nil {
		return v, err
	}
	return v, tx.Commit(ctx)
}

type UpdateMentorshipInput struct {
	SeasonID      string
	MentorshipID  string
	MentorUserID  string
	StudentUserID string
	ActorID       string
	ChangedAt     time.Time
}

func (p *Store) UpdateMentorship(ctx context.Context, input UpdateMentorshipInput) (programme.MentorshipRecord, error) {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return programme.MentorshipRecord{}, err
	}

	defer tx.Rollback(context.Background())
	var old programme.MentorshipRecord
	err = tx.QueryRow(ctx, `SELECT m.id,m.season_id,mentor.user_id,student.user_id FROM app.mentorships m
 JOIN app.enrollments mentor ON mentor.id=m.mentor_enrollment_id
 JOIN app.enrollments student ON student.id=m.student_enrollment_id
 WHERE m.id=$1 AND m.season_id=$2 AND m.ended_at IS NULL AND m.deleted_at IS NULL FOR UPDATE OF m`, input.MentorshipID, input.SeasonID).Scan(&old.ID, &old.SeasonID, &old.MentorUserID, &old.StudentUserID)
	if err != nil {
		return old, noRows(err)
	}
	if old.MentorUserID == input.MentorUserID && old.StudentUserID == input.StudentUserID {
		return old, tx.Commit(ctx)
	}
	if _, err := tx.Exec(ctx, `UPDATE app.mentorships SET ended_at=$2 WHERE id=$1`, old.ID, input.ChangedAt); err != nil {
		return old, err
	}
	v := programme.MentorshipRecord{ID: id.New(), SeasonID: input.SeasonID, MentorUserID: input.MentorUserID, StudentUserID: input.StudentUserID}
	_, err = tx.Exec(ctx, `INSERT INTO app.mentorships(id,season_id,mentor_enrollment_id,student_enrollment_id,created_at)
 VALUES($1,$2,(SELECT id FROM app.enrollments WHERE season_id=$2 AND user_id=$3 AND role IN ('mentor','coordinator') AND state='active' AND deleted_at IS NULL),
 (SELECT id FROM app.enrollments WHERE season_id=$2 AND user_id=$4 AND role='student' AND state='active' AND deleted_at IS NULL),$5)`, v.ID, v.SeasonID, v.MentorUserID, v.StudentUserID, input.ChangedAt)
	if err != nil {
		return v, mapDatabaseError(err)
	}
	if err := appendAuditTx(ctx, tx, newAudit(input.ActorID, "mentorship.reassigned", "mentorship", old.ID, map[string]any{"replacementId": v.ID}, input.ChangedAt)); err != nil {
		return v, err
	}
	return v, tx.Commit(ctx)
}

type DeleteMentorshipInput struct {
	SeasonID     string
	MentorshipID string
	ActorID      string
	ChangedAt    time.Time
}

func (p *Store) DeleteMentorship(ctx context.Context, input DeleteMentorshipInput) error {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return err
	}

	defer tx.Rollback(context.Background())
	result, err := tx.Exec(ctx, `UPDATE app.mentorships SET ended_at=$3 WHERE id=$1 AND season_id=$2 AND ended_at IS NULL AND deleted_at IS NULL`, input.MentorshipID, input.SeasonID, input.ChangedAt.UTC())
	if err != nil {
		return mapDatabaseError(err)
	}
	if result.RowsAffected() == 0 {
		return ErrNotFound
	}
	if err := appendAuditTx(ctx, tx, newAudit(input.ActorID, "mentorship.deleted", "mentorship", input.MentorshipID, nil, input.ChangedAt)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (p *Store) IsMentorAssigned(ctx context.Context, seasonID, mentorUserID, studentUserID string) (bool, error) {
	var assigned bool
	err := p.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM app.mentorships m JOIN app.enrollments mentor ON mentor.id=m.mentor_enrollment_id JOIN app.enrollments student ON student.id=m.student_enrollment_id WHERE m.season_id=$1 AND mentor.user_id=$2 AND student.user_id=$3 AND m.ended_at IS NULL AND m.deleted_at IS NULL)`, seasonID, mentorUserID, studentUserID).Scan(&assigned)
	return assigned, err
}
