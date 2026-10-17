package dal

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/magedmg/RSP-website/backend/internal/practice"
	"github.com/magedmg/RSP-website/backend/internal/programme"
)

// mondayUpdateColumns selects one update with the student's public profile.
const mondayUpdateColumns = `u.id,u.season_id,u.student_user_id,s.slug,s.display_name,s.avatar_url,u.start_date::text,u.end_date::text,u.content_html,u.hours::float8,u.created_at`

// mondayUpdateMarkColumns selects one mark with the mentor's public profile.
const mondayUpdateMarkColumns = `k.id,k.mentor_user_id,m.slug,m.display_name,m.avatar_url,k.mark::text,COALESCE(k.comment_html,''),k.updated_at`

// mondayUpdateFilters selects updates whose own range overlaps the requested
// range. The mentor filter keeps only the students that user mentors this
// season, so a mentor's list is scoped in SQL rather than in the handler.
const mondayUpdateFilters = `u.season_id=@seasonID AND u.deleted_at IS NULL
 AND u.start_date <= @endDate::date AND u.end_date >= @startDate::date
 AND (COALESCE(cardinality(@studentIDs::uuid[]),0)=0 OR u.student_user_id=ANY(@studentIDs::uuid[]))
 AND (@mentorUserID='' OR EXISTS (
   SELECT 1 FROM app.mentorships m
   JOIN app.enrollments mentor ON mentor.id=m.mentor_enrollment_id
   JOIN app.enrollments student ON student.id=m.student_enrollment_id
   WHERE m.season_id=u.season_id AND mentor.user_id=NULLIF(@mentorUserID,'')::uuid
    AND student.user_id=u.student_user_id AND m.ended_at IS NULL AND m.deleted_at IS NULL))`

// An update's activity is derived from the update's own dates, never from the
// range the caller selected, so the range is joined through app.monday_updates.
const mondayUpdateAttemptRange = `JOIN app.problem_attempts a ON a.user_id=u.student_user_id AND a.deleted_at IS NULL
 AND (a.attempted_at AT TIME ZONE 'Australia/Adelaide') >= u.start_date
 AND (a.attempted_at AT TIME ZONE 'Australia/Adelaide') < (u.end_date + 1)`

const mondayUpdateMockRange = `JOIN app.mock_interviews m ON m.interviewee_user_id=u.student_user_id AND m.deleted_at IS NULL
 AND (m.scheduled_at AT TIME ZONE 'Australia/Adelaide') >= u.start_date
 AND (m.scheduled_at AT TIME ZONE 'Australia/Adelaide') < (u.end_date + 1)`

func scanMondayUpdate(row pgx.Row) (programme.MondayUpdateRecord, error) {
	var v programme.MondayUpdateRecord
	err := row.Scan(&v.ID, &v.SeasonID, &v.Student.ID, &v.Student.Slug, &v.Student.Name, &v.Student.AvatarURL,
		&v.StartDate, &v.EndDate, &v.ContentHTML, &v.Hours, &v.CreatedAt)
	return v, noRows(err)
}

// normalizeMondayUpdate keeps the collections JSON requires to be arrays.
func normalizeMondayUpdate(v programme.MondayUpdateRecord) programme.MondayUpdateRecord {
	if v.Marks == nil {
		v.Marks = []programme.MondayUpdateMarkRecord{}
	}
	if v.Activity.Attempts == nil {
		v.Activity.Attempts = []programme.MondayUpdateAttempt{}
	}
	if v.Activity.Mocks == nil {
		v.Activity.Mocks = []programme.MondayUpdateMock{}
	}
	return v
}

// mondayUpdateRecordIDs indexes the loaded updates by id for batch hydration.
func mondayUpdateRecordIDs(items []programme.MondayUpdateRecord) ([]string, map[string]*programme.MondayUpdateRecord) {
	ids := make([]string, len(items))
	byID := make(map[string]*programme.MondayUpdateRecord, len(items))
	for i := range items {
		ids[i] = items[i].ID
		byID[items[i].ID] = &items[i]
	}
	return ids, byID
}

func solvedOutcomeNames() []string {
	outcomes := practice.SolvedOutcomes()
	names := make([]string, len(outcomes))
	for i, outcome := range outcomes {
		names[i] = string(outcome)
	}
	return names
}

type MondayUpdateQuery struct {
	SeasonID   string
	StartDate  string
	EndDate    string
	StudentIDs []string
	// MentorUserID scopes the list to the students that user mentors.
	MentorUserID string
	Boundary     string
	Limit        int
	IncludeMarks bool
}

func (p *Store) ListMondayUpdates(ctx context.Context, q MondayUpdateQuery) ([]programme.MondayUpdateRecord, bool, int64, error) {
	args := pgx.NamedArgs{
		"seasonID":     q.SeasonID,
		"startDate":    q.StartDate,
		"endDate":      q.EndDate,
		"studentIDs":   q.StudentIDs,
		"mentorUserID": q.MentorUserID,
	}
	var total int64
	if err := p.pool.QueryRow(ctx, `SELECT count(*) FROM app.monday_updates u WHERE `+mondayUpdateFilters, args).Scan(&total); err != nil {
		return nil, false, 0, err
	}

	args["boundary"] = q.Boundary
	args["limit"] = q.Limit + 1
	rows, err := p.pool.Query(ctx, `SELECT `+mondayUpdateColumns+`
		FROM app.monday_updates u
		JOIN app.user_profiles s ON s.user_id=u.student_user_id
		WHERE `+mondayUpdateFilters+`
		  AND (NULLIF(@boundary,'')::uuid IS NULL OR u.id < NULLIF(@boundary,'')::uuid)
		ORDER BY u.id DESC LIMIT @limit`, args)
	if err != nil {
		return nil, false, 0, err
	}

	defer rows.Close()
	items := []programme.MondayUpdateRecord{}
	for rows.Next() {
		record, err := scanMondayUpdate(rows)
		if err != nil {
			return nil, false, 0, err
		}
		items = append(items, normalizeMondayUpdate(record))
	}
	if err := rows.Err(); err != nil {
		return nil, false, 0, err
	}
	rows.Close()

	// Rows are already newest first, so the page keeps the forward direction.
	items, more := finishPage(items, q.Limit, "forward")
	if len(items) == 0 {
		return items, more, total, nil
	}
	ids, byID := mondayUpdateRecordIDs(items)
	if q.IncludeMarks {
		marks, err := p.loadMondayUpdateMarks(ctx, ids)
		if err != nil {
			return nil, false, 0, err
		}
		for id, records := range marks {
			byID[id].Marks = records
		}
	}
	if err := p.loadMondayUpdateActivity(ctx, items); err != nil {
		return nil, false, 0, err
	}
	return items, more, total, nil
}

func (p *Store) GetMondayUpdate(ctx context.Context, seasonID, updateID string) (programme.MondayUpdateRecord, error) {
	record, err := scanMondayUpdate(p.pool.QueryRow(ctx, `SELECT `+mondayUpdateColumns+`
		FROM app.monday_updates u
		JOIN app.user_profiles s ON s.user_id=u.student_user_id
		WHERE u.id=$1 AND u.season_id=$2 AND u.deleted_at IS NULL`, updateID, seasonID))
	if err != nil {
		return record, err
	}
	return normalizeMondayUpdate(record), nil
}

func (p *Store) GetMondayUpdateDetail(ctx context.Context, seasonID, updateID string) (programme.MondayUpdateRecord, error) {
	record, err := p.GetMondayUpdate(ctx, seasonID, updateID)
	if err != nil {
		return record, err
	}
	marks, err := p.loadMondayUpdateMarks(ctx, []string{record.ID})
	if err != nil {
		return record, err
	}
	if records := marks[record.ID]; records != nil {
		record.Marks = records
	}
	items := []programme.MondayUpdateRecord{record}
	if err := p.loadMondayUpdateActivity(ctx, items); err != nil {
		return record, err
	}
	return items[0], nil
}

// loadMondayUpdateMarks hydrates every requested update's marks in one query.
func (p *Store) loadMondayUpdateMarks(ctx context.Context, updateIDs []string) (map[string][]programme.MondayUpdateMarkRecord, error) {
	marks := make(map[string][]programme.MondayUpdateMarkRecord, len(updateIDs))
	if len(updateIDs) == 0 {
		return marks, nil
	}
	rows, err := p.pool.Query(ctx, `SELECT k.update_id,`+mondayUpdateMarkColumns+`
		FROM app.monday_update_marks k
		JOIN app.user_profiles m ON m.user_id=k.mentor_user_id
		WHERE k.update_id=ANY($1::uuid[])
		ORDER BY k.created_at,k.id`, updateIDs)
	if err != nil {
		return nil, err
	}

	defer rows.Close()
	for rows.Next() {
		var updateID string
		var record programme.MondayUpdateMarkRecord
		if err := rows.Scan(&updateID, &record.ID, &record.Mentor.ID, &record.Mentor.Slug, &record.Mentor.Name, &record.Mentor.AvatarURL,
			&record.Mark, &record.CommentHTML, &record.UpdatedAt); err != nil {
			return nil, err
		}
		record.UpdateID = updateID
		marks[updateID] = append(marks[updateID], record)
	}
	return marks, rows.Err()
}

// loadMondayUpdateActivity hydrates the attempts and mocks of every loaded
// update in two queries, each bounded by the update's own dates.
func (p *Store) loadMondayUpdateActivity(ctx context.Context, items []programme.MondayUpdateRecord) error {
	if len(items) == 0 {
		return nil
	}
	ids, byID := mondayUpdateRecordIDs(items)
	attemptRows, err := p.pool.Query(ctx, `SELECT u.id,a.id,COALESCE(l.id,a.problem_id),pr.title,COALESCE(pr.url,''),
		COALESCE(l.difficulty::text,''),a.outcome::text,a.time_taken_minutes,a.attempted_at
		FROM app.monday_updates u
		`+mondayUpdateAttemptRange+`
		JOIN app.problems pr ON pr.id=a.problem_id
		LEFT JOIN app.leetcode_problems l ON l.problem_id=a.problem_id AND l.deleted_at IS NULL
		WHERE u.id=ANY($1::uuid[]) AND u.deleted_at IS NULL
		ORDER BY u.id,a.attempted_at,a.id`, ids)
	if err != nil {
		return err
	}

	defer attemptRows.Close()
	for attemptRows.Next() {
		var updateID string
		var attempt programme.MondayUpdateAttempt
		if err := attemptRows.Scan(&updateID, &attempt.ID, &attempt.ProblemID, &attempt.Problem, &attempt.ProblemURL,
			&attempt.Difficulty, &attempt.Outcome, &attempt.Minutes, &attempt.AttemptedAt); err != nil {
			return err
		}
		record := byID[updateID]
		record.Activity.Attempts = append(record.Activity.Attempts, attempt)
	}
	if err := attemptRows.Err(); err != nil {
		return err
	}
	attemptRows.Close()

	mockRows, err := p.pool.Query(ctx, `SELECT u.id,m.id,m.scheduled_at,m.duration_minutes
		FROM app.monday_updates u
		`+mondayUpdateMockRange+`
		WHERE u.id=ANY($1::uuid[]) AND u.deleted_at IS NULL
		ORDER BY u.id,m.scheduled_at,m.id`, ids)
	if err != nil {
		return err
	}

	defer mockRows.Close()
	for mockRows.Next() {
		var updateID string
		var mock programme.MondayUpdateMock
		if err := mockRows.Scan(&updateID, &mock.ID, &mock.OccurredAt, &mock.DurationMinutes); err != nil {
			return err
		}
		record := byID[updateID]
		record.Activity.Mocks = append(record.Activity.Mocks, mock)
	}
	return mockRows.Err()
}

func (p *Store) CreateMondayUpdate(ctx context.Context, v programme.MondayUpdateRecord, actorID string) (programme.MondayUpdateRecord, error) {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return v, err
	}

	defer tx.Rollback(context.Background())
	if _, err := tx.Exec(ctx, `INSERT INTO app.monday_updates(id,season_id,student_user_id,start_date,end_date,content_html,hours)
		VALUES($1,$2,$3,$4::date,$5::date,$6,$7)`, v.ID, v.SeasonID, v.Student.ID, v.StartDate, v.EndDate, v.ContentHTML, v.Hours); err != nil {
		return v, mapDatabaseError(err)
	}
	if err := appendAuditTx(ctx, tx, newAudit(actorID, "monday_update.created", "monday_update", v.ID, nil, time.Now())); err != nil {
		return v, err
	}
	if err := tx.Commit(ctx); err != nil {
		return v, err
	}
	return p.GetMondayUpdateDetail(ctx, v.SeasonID, v.ID)
}

func (p *Store) DeleteMondayUpdate(ctx context.Context, seasonID, updateID, studentUserID, actorID string) error {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return err
	}

	defer tx.Rollback(context.Background())
	result, err := tx.Exec(ctx, `UPDATE app.monday_updates SET deleted_at=now()
		WHERE id=$1 AND season_id=$2 AND student_user_id=$3 AND deleted_at IS NULL`, updateID, seasonID, studentUserID)
	if err != nil {
		return mapDatabaseError(err)
	}
	if result.RowsAffected() == 0 {
		return ErrNotFound
	}
	if err := appendAuditTx(ctx, tx, newAudit(actorID, "monday_update.deleted", "monday_update", updateID, nil, time.Now())); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (p *Store) UpsertMondayUpdateMark(ctx context.Context, v programme.MondayUpdateMarkRecord, actorID string) (programme.MondayUpdateMarkRecord, error) {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return v, err
	}

	defer tx.Rollback(context.Background())
	var markID string
	if err := tx.QueryRow(ctx, `INSERT INTO app.monday_update_marks(id,update_id,mentor_user_id,mark,comment_html)
		VALUES($1,$2,$3,NULLIF($4,'')::app.monday_update_mark,NULLIF($5,''))
		ON CONFLICT (update_id,mentor_user_id) DO UPDATE
		SET mark=EXCLUDED.mark,comment_html=EXCLUDED.comment_html,comment_sanitization_changed=EXCLUDED.comment_sanitization_changed,updated_at=now()
		RETURNING id`,
		v.ID, v.UpdateID, v.Mentor.ID, v.Mark, v.CommentHTML).Scan(&markID); err != nil {
		return v, mapDatabaseError(err)
	}
	if err := appendAuditTx(ctx, tx, newAudit(actorID, "monday_update.mark_saved", "monday_update_mark", markID, nil, time.Now())); err != nil {
		return v, err
	}
	if err := tx.Commit(ctx); err != nil {
		return v, err
	}
	return scanMondayUpdateMark(p.pool.QueryRow(ctx, `SELECT k.update_id,`+mondayUpdateMarkColumns+`
		FROM app.monday_update_marks k
		JOIN app.user_profiles m ON m.user_id=k.mentor_user_id
		WHERE k.update_id=$1 AND k.mentor_user_id=$2`, v.UpdateID, v.Mentor.ID))
}

func scanMondayUpdateMark(row pgx.Row) (programme.MondayUpdateMarkRecord, error) {
	var v programme.MondayUpdateMarkRecord
	err := row.Scan(&v.UpdateID, &v.ID, &v.Mentor.ID, &v.Mentor.Slug, &v.Mentor.Name, &v.Mentor.AvatarURL, &v.Mark, &v.CommentHTML, &v.UpdatedAt)
	return v, noRows(err)
}

type MondayUpdateReviewQuery struct {
	SeasonID  string
	StartDate string
	EndDate   string
	Boundary  string
	Limit     int
}

// ListMondayUpdateReview returns one row per student enrolled in the season,
// including students with no update in the range. Every update and count for
// the page is hydrated in batched queries, never per row.
func (p *Store) ListMondayUpdateReview(ctx context.Context, q MondayUpdateReviewQuery) ([]programme.MondayUpdateReviewRow, bool, int64, error) {
	args := pgx.NamedArgs{"seasonID": q.SeasonID}
	var total int64
	if err := p.pool.QueryRow(ctx, `SELECT count(*) FROM app.enrollments e
		WHERE e.season_id=@seasonID AND e.role='student' AND e.deleted_at IS NULL`, args).Scan(&total); err != nil {
		return nil, false, 0, err
	}

	args["boundary"] = q.Boundary
	args["limit"] = q.Limit + 1
	rows, err := p.pool.Query(ctx, `WITH boundary AS (
		SELECT p.user_id,lower(p.display_name) AS sort_name FROM app.user_profiles p WHERE p.user_id=NULLIF(@boundary,'')::uuid
	) SELECT e.user_id,p.slug,p.display_name,p.avatar_url
	FROM app.enrollments e
	JOIN app.user_profiles p ON p.user_id=e.user_id
	WHERE e.season_id=@seasonID AND e.role='student' AND e.deleted_at IS NULL
	  AND (NULLIF(@boundary,'')::uuid IS NULL OR EXISTS (
	    SELECT 1 FROM boundary b WHERE (lower(p.display_name),e.user_id) > (b.sort_name,b.user_id)))
	ORDER BY lower(p.display_name),e.user_id LIMIT @limit`, args)
	if err != nil {
		return nil, false, 0, err
	}

	defer rows.Close()
	items := []programme.MondayUpdateReviewRow{}
	for rows.Next() {
		var row programme.MondayUpdateReviewRow
		if err := rows.Scan(&row.Student.ID, &row.Student.Slug, &row.Student.Name, &row.Student.AvatarURL); err != nil {
			return nil, false, 0, err
		}
		row.Updates = []programme.MondayUpdateReviewUpdate{}
		items = append(items, row)
	}
	if err := rows.Err(); err != nil {
		return nil, false, 0, err
	}
	rows.Close()

	// Students are already ordered ascending by name, so keep "forward".
	items, more := finishPage(items, q.Limit, "forward")
	if len(items) == 0 {
		return items, more, total, nil
	}
	studentIDs := make([]string, len(items))
	byStudent := make(map[string]*programme.MondayUpdateReviewRow, len(items))
	for i := range items {
		studentIDs[i] = items[i].Student.ID
		byStudent[items[i].Student.ID] = &items[i]
	}
	updateIDs, err := p.loadMondayUpdateReviewUpdates(ctx, q, studentIDs, byStudent)
	if err != nil {
		return nil, false, 0, err
	}
	for i := range items {
		items[i].UpdateCount = int64(len(items[i].Updates))
	}
	if len(updateIDs) == 0 {
		return items, more, total, nil
	}
	if err := p.loadMondayUpdateReviewCounts(ctx, updateIDs, byStudent); err != nil {
		return nil, false, 0, err
	}
	return items, more, total, nil
}

// loadMondayUpdateReviewUpdates attaches each student's updates in the reviewed
// range, marks included, and returns the loaded update ids.
func (p *Store) loadMondayUpdateReviewUpdates(ctx context.Context, q MondayUpdateReviewQuery, studentIDs []string, byStudent map[string]*programme.MondayUpdateReviewRow) ([]string, error) {
	args := pgx.NamedArgs{"seasonID": q.SeasonID, "startDate": q.StartDate, "endDate": q.EndDate, "studentIDs": studentIDs}
	rows, err := p.pool.Query(ctx, `SELECT u.student_user_id,u.id,u.start_date::text,u.end_date::text,u.content_html,u.hours::float8,u.created_at
		FROM app.monday_updates u
		WHERE u.season_id=@seasonID AND u.deleted_at IS NULL
		  AND u.start_date <= @endDate::date AND u.end_date >= @startDate::date
		  AND u.student_user_id=ANY(@studentIDs::uuid[])
		ORDER BY u.student_user_id,u.start_date DESC,u.id`, args)
	if err != nil {
		return nil, err
	}

	defer rows.Close()
	updateIDs := []string{}
	for rows.Next() {
		var studentID string
		var update programme.MondayUpdateReviewUpdate
		if err := rows.Scan(&studentID, &update.ID, &update.StartDate, &update.EndDate, &update.ContentHTML, &update.Hours, &update.CreatedAt); err != nil {
			return nil, err
		}
		update.Marks = []programme.MondayUpdateMarkRecord{}
		record := byStudent[studentID]
		record.Updates = append(record.Updates, update)
		updateIDs = append(updateIDs, update.ID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	rows.Close()

	marks, err := p.loadMondayUpdateMarks(ctx, updateIDs)
	if err != nil {
		return nil, err
	}
	for _, record := range byStudent {
		for i := range record.Updates {
			if records := marks[record.Updates[i].ID]; records != nil {
				record.Updates[i].Marks = records
			}
		}
	}
	return updateIDs, nil
}

// loadMondayUpdateReviewCounts counts each student's solved problems and mocks
// once, however many of that student's updates in the range contain them.
func (p *Store) loadMondayUpdateReviewCounts(ctx context.Context, updateIDs []string, byStudent map[string]*programme.MondayUpdateReviewRow) error {
	solvedRows, err := p.pool.Query(ctx, `SELECT u.student_user_id,l.difficulty::text,count(DISTINCT a.id)
		FROM app.monday_updates u
		`+mondayUpdateAttemptRange+`
		JOIN app.leetcode_problems l ON l.problem_id=a.problem_id AND l.deleted_at IS NULL
		WHERE u.id=ANY(@updateIDs::uuid[]) AND u.deleted_at IS NULL
		  AND a.outcome::text=ANY(@solvedOutcomes::text[])
		GROUP BY u.student_user_id,l.difficulty::text`,
		pgx.NamedArgs{"updateIDs": updateIDs, "solvedOutcomes": solvedOutcomeNames()})
	if err != nil {
		return err
	}

	defer solvedRows.Close()
	for solvedRows.Next() {
		var studentID, difficulty string
		var count int64
		if err := solvedRows.Scan(&studentID, &difficulty, &count); err != nil {
			return err
		}
		counts := &byStudent[studentID].LeetCodeCounts
		switch difficulty {
		case "easy":
			counts.Easy = count
		case "medium":
			counts.Medium = count
		case "hard":
			counts.Hard = count
		}
	}
	if err := solvedRows.Err(); err != nil {
		return err
	}
	solvedRows.Close()

	mockRows, err := p.pool.Query(ctx, `SELECT u.student_user_id,count(DISTINCT m.id)
		FROM app.monday_updates u
		`+mondayUpdateMockRange+`
		WHERE u.id=ANY(@updateIDs::uuid[]) AND u.deleted_at IS NULL
		GROUP BY u.student_user_id`, pgx.NamedArgs{"updateIDs": updateIDs})
	if err != nil {
		return err
	}

	defer mockRows.Close()
	for mockRows.Next() {
		var studentID string
		var count int64
		if err := mockRows.Scan(&studentID, &count); err != nil {
			return err
		}
		byStudent[studentID].MockCount = count
	}
	return mockRows.Err()
}
