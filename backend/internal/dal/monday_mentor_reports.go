package dal

import (
	"context"
	"errors"
	"sort"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/magedmg/RSP-website/backend/internal/practice"
	"github.com/magedmg/RSP-website/backend/internal/program"
)

// UpsertMondayWeekMark saves a mentor's mark on a student's week that has
// no update, replacing their earlier one.
func (p *Store) UpsertMondayWeekMark(ctx context.Context, v program.MondayWeekMark, actorID string) (program.MondayWeekMark, error) {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return v, err
	}
	defer tx.Rollback(context.Background())
	var markID string
	if err := tx.QueryRow(ctx, `INSERT INTO app.monday_week_marks(season_id,student_user_id,mentor_user_id,start_date,end_date,mark,comment_html)
		VALUES($1,$2,$3,$4::date,$5::date,$6::app.monday_update_mark,NULLIF($7,''))
		ON CONFLICT (season_id,student_user_id,start_date,mentor_user_id) DO UPDATE
		SET mark=EXCLUDED.mark,comment_html=EXCLUDED.comment_html,updated_at=now()
		RETURNING id,updated_at`, v.SeasonID, v.StudentID, v.MentorID, v.StartDate, v.EndDate, v.Mark, v.CommentHTML).Scan(&markID, &v.UpdatedAt); err != nil {
		return v, mapDatabaseError(err)
	}
	if err := appendAuditTx(ctx, tx, newAudit(actorID, "monday_week_mark.saved", "monday_week_mark", markID, nil, time.Now())); err != nil {
		return v, err
	}
	return v, tx.Commit(ctx)
}

// ListMondayMentorReports builds each mentor's week (one mentor's when
// mentorID is set): their current mentees, each with their update, activity
// and the mentor's mark, in a fixed number of queries.
func (p *Store) ListMondayMentorReports(ctx context.Context, seasonID, startDate, endDate, mentorID string) ([]program.MondayMentorReport, error) {
	defaults, err := p.programPracticeSettings(ctx)
	if err != nil {
		return nil, err
	}
	args := pgx.NamedArgs{"seasonID": seasonID, "startDate": startDate, "endDate": endDate, "mentorID": nullableID(mentorID)}
	rows, err := p.pool.Query(ctx, `SELECT mentor.user_id,mp.slug,mp.display_name,mp.avatar_url,
			student.user_id,sp.slug,sp.display_name,sp.avatar_url,student.student_level::text,
			r.weekly_hours,
			-- Listed students were kicked during or after this week; either
			-- way they will not post, so the week reads as kicked.
			CASE WHEN student.state='kicked' THEN student.state_changed_at END
		FROM app.mentorships m
		JOIN app.enrollments mentor ON mentor.id=m.mentor_enrollment_id
		JOIN app.enrollments student ON student.id=m.student_enrollment_id
		JOIN app.user_profiles mp ON mp.user_id=mentor.user_id
		JOIN app.user_profiles sp ON sp.user_id=student.user_id
		LEFT JOIN app.student_requirements r ON r.enrollment_id=student.id
		WHERE m.season_id=@seasonID AND m.deleted_at IS NULL
		  AND student.role='student' AND student.deleted_at IS NULL
		  -- Current students, and kicked ones up to the week they were kicked.
		  AND ((m.ended_at IS NULL AND student.state='active')
		    OR (student.state='kicked' AND m.ended_at IS NOT NULL
		      AND (m.ended_at AT TIME ZONE 'Australia/Adelaide')::date >= @startDate::date))
		  AND (@mentorID::uuid IS NULL OR mentor.user_id=@mentorID::uuid)
		ORDER BY mp.display_name,mentor.user_id,sp.display_name,student.user_id`, args)
	if err != nil {
		return nil, err
	}
	byMentor := map[string]*program.MondayMentorReport{}
	order := []string{}
	studentIDs := []string{}
	seen := map[string]bool{}
	for rows.Next() {
		var mentor, student program.PersonSummary
		var level string
		var kickedAt *time.Time
		var own practice.StudentRequirements
		if err := rows.Scan(&mentor.ID, &mentor.Slug, &mentor.Name, &mentor.AvatarURL,
			&student.ID, &student.Slug, &student.Name, &student.AvatarURL, &level,
			&own.WeeklyHours, &kickedAt); err != nil {
			rows.Close()
			return nil, err
		}
		report := byMentor[mentor.ID]
		if report == nil {
			report = &program.MondayMentorReport{Mentor: mentor, StartDate: startDate, EndDate: endDate, Students: []program.MondayMentorReportStudent{}}
			byMentor[mentor.ID] = report
			order = append(order, mentor.ID)
		}
		report.Students = append(report.Students, program.MondayMentorReportStudent{
			Student: student, StudentLevel: level, KickedAt: kickedAt, Targets: weeklyTargets(own.Apply(defaults)),
		})
		if !seen[student.ID] {
			seen[student.ID] = true
			studentIDs = append(studentIDs, student.ID)
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	reports := []program.MondayMentorReport{}
	if len(order) == 0 {
		return reports, nil
	}
	args["studentIDs"] = studentIDs

	// Each student's update for the week.
	type posted struct {
		id       string
		postedAt time.Time
		editedAt *time.Time
		hours    float64
	}
	updates := map[string]posted{}
	if rows, err = p.pool.Query(ctx, `SELECT student_user_id,id,created_at,edited_at,hours::float8 FROM app.monday_updates
		WHERE season_id=@seasonID AND deleted_at IS NULL AND start_date=@startDate::date AND end_date=@endDate::date
		  AND student_user_id=ANY(@studentIDs::uuid[])`, args); err != nil {
		return nil, err
	}
	for rows.Next() {
		var studentID string
		var u posted
		if err := rows.Scan(&studentID, &u.id, &u.postedAt, &u.editedAt, &u.hours); err != nil {
			rows.Close()
			return nil, err
		}
		updates[studentID] = u
	}
	rows.Close()

	// The mentors' marks: on the update, or on the week when there is none.
	type markKey struct{ student, mentor string }
	type markValue struct{ mark, comment string }
	marks := map[markKey]markValue{}
	if rows, err = p.pool.Query(ctx, `SELECT u.student_user_id,k.mentor_user_id,k.mark::text,COALESCE(k.comment_html,'')
		FROM app.monday_update_marks k JOIN app.monday_updates u ON u.id=k.update_id
		WHERE u.season_id=@seasonID AND u.deleted_at IS NULL AND u.start_date=@startDate::date AND u.end_date=@endDate::date
		  AND u.student_user_id=ANY(@studentIDs::uuid[])
		UNION ALL
		SELECT w.student_user_id,w.mentor_user_id,w.mark::text,COALESCE(w.comment_html,'')
		FROM app.monday_week_marks w
		WHERE w.season_id=@seasonID AND w.start_date=@startDate::date AND w.student_user_id=ANY(@studentIDs::uuid[])
		  AND NOT EXISTS(SELECT 1 FROM app.monday_updates u WHERE u.season_id=w.season_id AND u.student_user_id=w.student_user_id
			AND u.start_date=w.start_date AND u.deleted_at IS NULL)`, args); err != nil {
		return nil, err
	}
	for rows.Next() {
		var key markKey
		var value markValue
		if err := rows.Scan(&key.student, &key.mentor, &value.mark, &value.comment); err != nil {
			rows.Close()
			return nil, err
		}
		marks[key] = value
	}
	rows.Close()

	// The week's solved LeetCode by difficulty and mock interviews.
	counts := map[string]*program.MondayUpdateDifficultyCounts{}
	mocks := map[string]int64{}
	if rows, err = p.pool.Query(ctx, `SELECT a.user_id,l.difficulty::text,count(*)
		FROM app.problem_attempts a
		JOIN app.leetcode_problems l ON l.problem_id=a.problem_id AND l.deleted_at IS NULL
		WHERE a.user_id=ANY(@studentIDs::uuid[]) AND a.deleted_at IS NULL
		  AND a.outcome::text=ANY(@solvedOutcomes::text[])
		  AND (a.attempted_at AT TIME ZONE 'Australia/Adelaide') >= @startDate::date
		  AND (a.attempted_at AT TIME ZONE 'Australia/Adelaide') < (@endDate::date + 1)
		GROUP BY a.user_id,l.difficulty::text`, pgx.NamedArgs{"studentIDs": studentIDs, "startDate": startDate, "endDate": endDate, "solvedOutcomes": solvedOutcomeNames()}); err != nil {
		return nil, err
	}
	for rows.Next() {
		var studentID, difficulty string
		var count int64
		if err := rows.Scan(&studentID, &difficulty, &count); err != nil {
			rows.Close()
			return nil, err
		}
		c := counts[studentID]
		if c == nil {
			c = &program.MondayUpdateDifficultyCounts{}
			counts[studentID] = c
		}
		switch difficulty {
		case "easy":
			c.Easy += count
		case "medium":
			c.Medium += count
		case "hard":
			c.Hard += count
		}
	}
	rows.Close()
	if rows, err = p.pool.Query(ctx, `SELECT m.interviewee_user_id,count(*) FROM app.mock_interviews m
		WHERE m.interviewee_user_id=ANY(@studentIDs::uuid[]) AND m.deleted_at IS NULL
		  AND (m.scheduled_at AT TIME ZONE 'Australia/Adelaide') >= @startDate::date
		  AND (m.scheduled_at AT TIME ZONE 'Australia/Adelaide') < (@endDate::date + 1)
		GROUP BY m.interviewee_user_id`, args); err != nil {
		return nil, err
	}
	for rows.Next() {
		var studentID string
		var count int64
		if err := rows.Scan(&studentID, &count); err != nil {
			rows.Close()
			return nil, err
		}
		mocks[studentID] = count
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	for _, id := range order {
		report := byMentor[id]
		for i := range report.Students {
			student := &report.Students[i]
			if u, ok := updates[student.Student.ID]; ok {
				updateID, postedAt, hours := u.id, u.postedAt, u.hours
				student.UpdateID, student.PostedAt, student.EditedAt, student.Hours = &updateID, &postedAt, u.editedAt, &hours
			}
			if c := counts[student.Student.ID]; c != nil {
				student.LeetCodeCounts = *c
			}
			student.MockCount = mocks[student.Student.ID]
			if m, ok := marks[markKey{student.Student.ID, id}]; ok {
				mark := m.mark
				student.Mark, student.CommentHTML = &mark, m.comment
			}
		}
		sort.SliceStable(report.Students, func(a, b int) bool { return report.Students[a].Student.Name < report.Students[b].Student.Name })
		reports = append(reports, *report)
	}
	return reports, nil
}

func weeklyTargets(settings practice.PracticeSettings) program.MondayWeeklyTargets {
	return program.MondayWeeklyTargets{Hours: settings.WeeklyHours}
}

// ClearMondayWeekMark removes a mentor's mark on a student's week.
func (p *Store) ClearMondayWeekMark(ctx context.Context, seasonID, studentID, mentorID, startDate string) error {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())
	var markID string
	err = tx.QueryRow(ctx, `DELETE FROM app.monday_week_marks
		WHERE season_id=$1 AND student_user_id=$2 AND mentor_user_id=$3 AND start_date=$4::date RETURNING id`,
		seasonID, studentID, mentorID, startDate).Scan(&markID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if err := appendAuditTx(ctx, tx, newAudit(mentorID, "monday_week_mark.cleared", "monday_week_mark", markID, nil, time.Now())); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// ClearMondayUpdateMark removes a mentor's mark on an update.
func (p *Store) ClearMondayUpdateMark(ctx context.Context, updateID, mentorID string) error {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())
	var markID string
	err = tx.QueryRow(ctx, `DELETE FROM app.monday_update_marks WHERE update_id=$1 AND mentor_user_id=$2 RETURNING id`,
		updateID, mentorID).Scan(&markID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if err := appendAuditTx(ctx, tx, newAudit(mentorID, "monday_update.mark_cleared", "monday_update_mark", markID, nil, time.Now())); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
