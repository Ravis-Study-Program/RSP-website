package dal

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/magedmg/RSP-website/backend/internal/programme"
)

type MentorshipQuery struct {
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
	const filters = `m.season_id = @seasonID AND m.ended_at IS NULL AND m.deleted_at IS NULL
		AND (@mentorUserID = '' OR mentor.user_id = NULLIF(@mentorUserID, '')::uuid)
		AND (@studentUserID = '' OR student.user_id = NULLIF(@studentUserID, '')::uuid)`
	args := pgx.NamedArgs{"seasonID": q.SeasonID, "mentorUserID": q.MentorUserID, "studentUserID": q.StudentUserID}
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
	) SELECT m.id,m.season_id,mentor.user_id,student.user_id,m.revision` + base + `
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
		if err := rows.Scan(&v.ID, &v.SeasonID, &v.MentorUserID, &v.StudentUserID, &v.Revision); err != nil {
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
	err = tx.QueryRow(ctx, `INSERT INTO app.mentorships(id,season_id,mentor_enrollment_id,student_enrollment_id,revision) VALUES($1,$2,(SELECT id FROM app.enrollments WHERE season_id=$2 AND user_id=$3 AND role IN ('mentor','coordinator') AND state='active' AND deleted_at IS NULL),(SELECT id FROM app.enrollments WHERE season_id=$2 AND user_id=$4 AND role='student' AND state='active' AND deleted_at IS NULL),$5) RETURNING revision`, v.ID, v.SeasonID, v.MentorUserID, v.StudentUserID, v.Revision).Scan(&v.Revision)
	if err != nil {
		return v, mapDatabaseError(err)
	}
	if err := appendAuditTx(ctx, tx, newAudit(actorID, "mentorship.created", "mentorship", v.ID, nil, at)); err != nil {
		return v, err
	}
	return v, tx.Commit(ctx)
}

func (p *Store) UpdateMentorship(ctx context.Context, seasonID, mentorshipID string, revision int64, mentorUserID, studentUserID, actorID string, at time.Time) (programme.MentorshipRecord, error) {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return programme.MentorshipRecord{}, err
	}

	defer tx.Rollback(context.Background())
	var v programme.MentorshipRecord
	err = tx.QueryRow(ctx, `UPDATE app.mentorships SET mentor_enrollment_id=(SELECT id FROM app.enrollments WHERE season_id=$2 AND user_id=$4 AND role IN ('mentor','coordinator') AND state='active' AND deleted_at IS NULL),student_enrollment_id=(SELECT id FROM app.enrollments WHERE season_id=$2 AND user_id=$5 AND role='student' AND state='active' AND deleted_at IS NULL),revision=revision+1 WHERE id=$1 AND season_id=$2 AND revision=$3 AND ended_at IS NULL AND deleted_at IS NULL RETURNING id,season_id,$4::text,$5::text,revision`, mentorshipID, seasonID, revision, mentorUserID, studentUserID).Scan(&v.ID, &v.SeasonID, &v.MentorUserID, &v.StudentUserID, &v.Revision)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return v, p.classifyRevision(ctx, tx, `SELECT revision FROM app.mentorships WHERE id=$1 AND season_id=$2 AND ended_at IS NULL AND deleted_at IS NULL`, mentorshipID, seasonID)
		}
		return v, mapDatabaseError(err)
	}
	if err := appendAuditTx(ctx, tx, newAudit(actorID, "mentorship.updated", "mentorship", mentorshipID, nil, at)); err != nil {
		return v, err
	}
	return v, tx.Commit(ctx)
}

func (p *Store) DeleteMentorship(ctx context.Context, seasonID, mentorshipID string, revision int64, actorID string, at time.Time) error {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return err
	}

	defer tx.Rollback(context.Background())
	result, err := tx.Exec(ctx, `UPDATE app.mentorships SET ended_at=$4,revision=revision+1 WHERE id=$1 AND season_id=$2 AND revision=$3 AND ended_at IS NULL AND deleted_at IS NULL`, mentorshipID, seasonID, revision, at.UTC())
	if err != nil {
		return mapDatabaseError(err)
	}
	if result.RowsAffected() == 0 {
		return p.classifyRevision(ctx, tx, `SELECT revision FROM app.mentorships WHERE id=$1 AND season_id=$2 AND ended_at IS NULL AND deleted_at IS NULL`, mentorshipID, seasonID)
	}
	if err := appendAuditTx(ctx, tx, newAudit(actorID, "mentorship.deleted", "mentorship", mentorshipID, nil, at)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (p *Store) IsMentorAssigned(ctx context.Context, seasonID, mentorUserID, studentUserID string) (bool, error) {
	var assigned bool
	err := p.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM app.mentorships m JOIN app.enrollments mentor ON mentor.id=m.mentor_enrollment_id JOIN app.enrollments student ON student.id=m.student_enrollment_id WHERE m.season_id=$1 AND mentor.user_id=$2 AND student.user_id=$3 AND m.ended_at IS NULL AND m.deleted_at IS NULL)`, seasonID, mentorUserID, studentUserID).Scan(&assigned)
	return assigned, err
}
