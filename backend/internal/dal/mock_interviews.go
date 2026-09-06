package dal

import (
	"context"
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/magedmg/RSP-website/backend/internal/mockinterviews"
	"github.com/magedmg/RSP-website/backend/internal/platform/id"
)

const mockInterviewColumns = `mi.id,mi.interviewer_user_id,mi.interviewee_user_id,mi.activity_season_id,
	mi.scheduled_at,mi.duration_minutes,COALESCE(mi.interviewer_notes_html,''),mi.deleted_at`

func scanMockInterview(row pgx.Row) (mockinterviews.Interview, error) {
	var interview mockinterviews.Interview
	err := row.Scan(&interview.ID, &interview.InterviewerID, &interview.IntervieweeID,
		&interview.SeasonID, &interview.OccurredAt, &interview.DurationMinutes,
		&interview.Notes, &interview.DeletedAt)
	return interview, noRows(err)
}

type MockInterviewVisibility string

const (
	MockInterviewsGiven    MockInterviewVisibility = "given"
	MockInterviewsReceived MockInterviewVisibility = "received"
	MockInterviewsRelated  MockInterviewVisibility = "related"
	MockInterviewsAll      MockInterviewVisibility = "all"
)

type MockInterviewQuery struct {
	SeasonID   string
	Year       int
	ViewerID   string
	Visibility MockInterviewVisibility
	Boundary   string
	Limit      int
	SortBy     string
	Direction  string
}

func (p *Store) ListMockInterviews(ctx context.Context, q MockInterviewQuery) ([]mockinterviews.Interview, bool, int64, error) {
	userID := q.ViewerID
	where := `(mi.interviewer_user_id=$1 OR mi.interviewee_user_id=$1)`
	if q.Visibility == MockInterviewsGiven {
		where = `mi.interviewer_user_id=$1`
	} else if q.Visibility == MockInterviewsReceived {
		where = `mi.interviewee_user_id=$1`
	} else if q.Visibility == MockInterviewsAll {
		where = `$1::text IS NOT NULL`
	} else if q.Visibility == MockInterviewsRelated {
		where = `(mi.interviewer_user_id=$1 OR mi.interviewee_user_id=$1 OR EXISTS (
			SELECT 1 FROM app.enrollments viewer
			WHERE viewer.user_id=$1 AND viewer.season_id=mi.activity_season_id AND viewer.state IN ('active','completed') AND viewer.deleted_at IS NULL
			AND (viewer.role='coordinator' OR (viewer.role='mentor' AND EXISTS (
				SELECT 1 FROM app.mentorships m
				JOIN app.enrollments mentor ON mentor.id=m.mentor_enrollment_id
				JOIN app.enrollments student ON student.id=m.student_enrollment_id
				WHERE m.season_id=mi.activity_season_id AND m.created_at<=mi.scheduled_at AND (m.ended_at IS NULL OR mi.scheduled_at<m.ended_at) AND m.deleted_at IS NULL
				AND mentor.user_id=$1 AND student.user_id=mi.interviewee_user_id
		)))))`
	}
	where = "(" + where + ") AND ($2 = '' OR mi.activity_season_id=NULLIF($2,'')::uuid) AND ($3 = 0 OR EXTRACT(YEAR FROM mi.scheduled_at AT TIME ZONE 'Australia/Adelaide')=$3)"
	var total int64
	if err := p.pool.QueryRow(ctx, `SELECT count(*) FROM app.scoped_mock_interviews mi WHERE mi.deleted_at IS NULL AND `+where, userID, q.SeasonID, q.Year).Scan(&total); err != nil {
		return nil, false, 0, err
	}

	comparator, order := ">", "ASC"
	key, boundaryKey := "mi.id", "b.id"
	boundarySelect := "id"
	if q.Direction == "backward" {
		comparator, order = "<", "DESC"
	}
	if q.SortBy == "occurredAt:desc" {
		key, boundaryKey = "ROW(mi.scheduled_at,mi.id)", "ROW(b.scheduled_at,b.id)"
		boundarySelect = "scheduled_at,id"
		comparator, order = "<", "DESC"
		if q.Direction == "backward" {
			comparator, order = ">", "ASC"
		}
	}
	orderBy := "mi.id " + order
	if q.SortBy == "occurredAt:desc" {
		orderBy = "mi.scheduled_at " + order + ",mi.id " + order
	}
	query := `WITH boundary AS (
		SELECT ` + boundarySelect + ` FROM app.mock_interviews WHERE id=NULLIF($4,'')::uuid AND deleted_at IS NULL
	) SELECT ` + mockInterviewColumns + ` FROM app.scoped_mock_interviews mi
	WHERE mi.deleted_at IS NULL AND ` + where + `
	  AND (NULLIF($4,'')::uuid IS NULL OR EXISTS (SELECT 1 FROM boundary b WHERE ` + key + ` ` + comparator + ` ` + boundaryKey + `))
	ORDER BY ` + orderBy + ` LIMIT $5`
	rows, err := p.pool.Query(ctx, query, userID, q.SeasonID, q.Year, q.Boundary, q.Limit+1)
	if err != nil {
		return nil, false, 0, err
	}

	defer rows.Close()
	items := []mockinterviews.Interview{}
	for rows.Next() {
		interview, err := scanMockInterview(rows)
		if err != nil {
			return nil, false, 0, err
		}
		items = append(items, interview)
	}
	if err := rows.Err(); err != nil {
		return nil, false, 0, err
	}
	rows.Close()

	items, more := finishPage(items, q.Limit, q.Direction)
	if err := loadMockRounds(ctx, p.pool, items); err != nil {
		return nil, false, 0, err
	}
	return items, more, total, nil
}

func (p *Store) GetMockInterview(ctx context.Context, interviewID string) (mockinterviews.Interview, error) {
	return loadMockInterview(ctx, p.pool, interviewID)
}

func loadMockInterview(ctx context.Context, db queryer, interviewID string) (mockinterviews.Interview, error) {
	interview, err := scanMockInterview(db.QueryRow(ctx, `SELECT `+mockInterviewColumns+`
		FROM app.scoped_mock_interviews mi WHERE mi.id=$1 AND mi.deleted_at IS NULL`, interviewID))
	if err != nil {
		return mockinterviews.Interview{}, err
	}
	items := []mockinterviews.Interview{interview}
	if err := loadMockRounds(ctx, db, items); err != nil {
		return mockinterviews.Interview{}, err
	}
	return items[0], nil
}

func loadMockRounds(ctx context.Context, db queryer, interviews []mockinterviews.Interview) error {
	if len(interviews) == 0 {
		return nil
	}
	ids := make([]string, len(interviews))
	byID := make(map[string]*mockinterviews.Interview, len(interviews))
	for i := range interviews {
		ids[i] = interviews[i].ID
		byID[interviews[i].ID] = &interviews[i]
	}
	rows, err := db.Query(ctx, `SELECT r.mock_interview_id,r.id,r.kind::text,r.review_status='reviewed',
		COALESCE(r.interviewee_comment_html,''),b.behavioural_score,l.leetcode_problem_id,
		l.clarify_question_score,l.algorithm_design_score,l.complexity_analysis_score,
		l.coding_score,l.testing_score,COALESCE(c.content_html,''),COALESCE(c.url,''),c.score
		FROM app.mock_interview_rounds r
		LEFT JOIN app.behavioural_mock_interview_rounds b ON b.mock_interview_round_id=r.id
		LEFT JOIN app.leetcode_mock_interview_rounds l ON l.mock_interview_round_id=r.id
		LEFT JOIN app.custom_mock_interview_rounds c ON c.mock_interview_round_id=r.id
		WHERE r.mock_interview_id = ANY($1::uuid[]) AND r.deleted_at IS NULL
		ORDER BY r.mock_interview_id,r.position,r.id`, ids)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var round mockinterviews.Round
		var interviewID, kind string
		var behavioural, clarify, algorithm, complexity, coding, testing, custom *int16
		var problemID *string
		if err := rows.Scan(&interviewID, &round.ID, &kind, &round.Reviewed, &round.IntervieweeComment,
			&behavioural, &problemID, &clarify, &algorithm, &complexity, &coding, &testing,
			&round.Content, &round.Link, &custom); err != nil {
			return err
		}
		round.Type = mockinterviews.RoundType(kind)
		round.Scores = scoresFromDB(behavioural, clarify, algorithm, complexity, coding, testing, custom)
		if problemID != nil {
			round.ProblemID = *problemID
		}
		interview := byID[interviewID]
		interview.Rounds = append(interview.Rounds, round)
	}
	return rows.Err()
}

// ListMockParticipantSummaries returns public display fields in one query.
// Missing and deleted users are omitted so callers can apply an explicit fallback.
func (p *Store) ListMockParticipantSummaries(ctx context.Context, userIDs []string) (map[string]mockinterviews.ParticipantSummary, error) {
	byID := make(map[string]mockinterviews.ParticipantSummary, len(userIDs))
	if len(userIDs) == 0 {
		return byID, nil
	}
	rows, err := p.pool.Query(ctx, `SELECT u.id,p.slug,p.display_name,p.avatar_url FROM app.users u
		JOIN app.user_profiles p ON p.user_id=u.id
		WHERE u.id=ANY($1::uuid[]) AND u.deleted_at IS NULL AND u.account_state<>'deleted'`, userIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var summary mockinterviews.ParticipantSummary
		if err := rows.Scan(&summary.ID, &summary.Slug, &summary.Name, &summary.AvatarURL); err != nil {
			return nil, err
		}
		byID[summary.ID] = summary
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return byID, nil
}

func scoresFromDB(values ...*int16) mockinterviews.Scores {
	toInt := func(v *int16) *int {
		if v == nil {
			return nil
		}
		n := int(*v)
		return &n
	}
	return mockinterviews.Scores{
		Behavioural:        toInt(values[0]),
		ConfirmQuestions:   toInt(values[1]),
		AlgorithmDesign:    toInt(values[2]),
		ComplexityAnalysis: toInt(values[3]),
		Coding:             toInt(values[4]),
		Testing:            toInt(values[5]),
		Custom:             toInt(values[6]),
	}
}

func (p *Store) CreateMockInterview(ctx context.Context, v mockinterviews.Interview, actorID string, at time.Time) (mockinterviews.Interview, error) {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return mockinterviews.Interview{}, err
	}

	defer tx.Rollback(context.Background())
	if _, err := tx.Exec(ctx, `INSERT INTO app.mock_interviews(id,interviewer_user_id,interviewee_user_id,season_id,scheduled_at,duration_minutes,interviewer_notes_html)
	VALUES($1,$2,$3,$4,$5,$6,$7)`, v.ID, v.InterviewerID, v.IntervieweeID, nil, v.OccurredAt.UTC(), v.DurationMinutes, v.Notes); err != nil {
		return v, mapDatabaseError(err)
	}
	if err := reconcileMockRounds(ctx, tx, v); err != nil {
		return v, err
	}
	v, err = loadMockInterview(ctx, tx, v.ID)
	if err != nil {
		return v, err
	}
	if err := appendMockVersionTx(ctx, tx, v, actorID, "created", at); err != nil {
		return v, err
	}
	if err := appendAuditTx(ctx, tx, newAudit(actorID, "mock_interview.created", "mock_interview", v.ID, nil, at)); err != nil {
		return v, err
	}
	if err := tx.Commit(ctx); err != nil {
		return v, err
	}
	return v, nil
}

// UpdateMockInterview saves interviewer-owned fields and reconciles rounds by ID.
func (p *Store) UpdateMockInterview(ctx context.Context, v mockinterviews.Interview, actorID string, at time.Time) (mockinterviews.Interview, error) {
	return p.writeMockInterview(ctx, v, actorID, "updated", "mock_interview.updated", at, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `UPDATE app.mock_interviews SET duration_minutes=$1,interviewer_notes_html=$2 WHERE id = $3`, v.DurationMinutes, v.Notes, v.ID); err != nil {
			return err
		}
		return reconcileMockRounds(ctx, tx, v)
	})
}

// DeleteMockInterview leaves rounds and previous versions intact.
func (p *Store) DeleteMockInterview(ctx context.Context, v mockinterviews.Interview, actorID string, at time.Time) (mockinterviews.Interview, error) {
	return p.writeMockInterview(ctx, v, actorID, "soft deleted", "mock_interview.deleted", at, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `UPDATE app.mock_interviews SET deleted_at=$1 WHERE id = $2`, v.DeletedAt, v.ID)
		return err
	})
}

// ReviewMockInterviewRound changes only the selected round's interviewee-owned fields.
type ReviewMockInterviewRoundInput struct {
	Interview mockinterviews.Interview
	RoundID   string
	ActorID   string
	ChangedAt time.Time
}

func (p *Store) ReviewMockInterviewRound(ctx context.Context, input ReviewMockInterviewRoundInput) (mockinterviews.Interview, error) {
	return p.writeMockInterview(ctx, input.Interview, input.ActorID, "interviewee review", "mock_interview.reviewed", input.ChangedAt, func(tx pgx.Tx) error {

		var reviewedRound *mockinterviews.Round
		for i := range input.Interview.Rounds {
			if input.Interview.Rounds[i].ID == input.RoundID {
				reviewedRound = &input.Interview.Rounds[i]
				break
			}
		}
		if reviewedRound == nil {
			return mockinterviews.ErrInvalid
		}
		status := "pending"
		var reviewedAt *time.Time
		if reviewedRound.Reviewed {
			status = "reviewed"
			timestamp := input.ChangedAt.UTC()
			reviewedAt = &timestamp
		}
		result, err := tx.Exec(ctx, `UPDATE app.mock_interview_rounds SET review_status=$1,reviewed_at=$2,interviewee_comment_html=$3 WHERE id = $4 AND mock_interview_id = $5 AND deleted_at IS NULL`, status, reviewedAt, reviewedRound.IntervieweeComment, input.RoundID, input.Interview.ID)
		if err != nil {
			return err
		}
		if result.RowsAffected() != 1 {
			return ErrNotFound
		}
		return nil
	})
}

// CorrectMockInterviewIdentities preserves all interview content and round metadata.
type CorrectMockInterviewIdentitiesInput struct {
	Interview mockinterviews.Interview
	ActorID   string
	Reason    string
	ChangedAt time.Time
}

func (p *Store) CorrectMockInterviewIdentities(ctx context.Context, input CorrectMockInterviewIdentitiesInput) (mockinterviews.Interview, error) {
	return p.writeMockInterview(ctx, input.Interview, input.ActorID, "identity correction: "+input.Reason, "mock_interview.identities_corrected", input.ChangedAt, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `UPDATE app.mock_interviews SET interviewer_user_id=$1,interviewee_user_id=$2,season_id=$3 WHERE id = $4`, input.Interview.InterviewerID, input.Interview.IntervieweeID, nil, input.Interview.ID)
		return err
	})
}

// Action and reason are metadata only; the supplied mutation selects the writable fields.
func (p *Store) writeMockInterview(ctx context.Context, v mockinterviews.Interview, actorID, reason, action string, at time.Time, mutate func(pgx.Tx) error) (mockinterviews.Interview, error) {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return mockinterviews.Interview{}, err
	}
	defer tx.Rollback(context.Background())
	var recordedAt time.Time
	err = tx.QueryRow(ctx, `SELECT scheduled_at FROM app.mock_interviews WHERE id = $1 AND deleted_at IS NULL FOR UPDATE`, v.ID).Scan(&recordedAt)
	if err != nil {
		return v, noRows(err)
	}
	if !recordedAt.Equal(v.OccurredAt) {
		return v, ErrConflict
	}
	if err := mutate(tx); err != nil {
		return v, mapDatabaseError(err)
	}
	if v.DeletedAt == nil {
		// Snapshot the persisted result, including fields this operation did not own.
		v, err = loadMockInterview(ctx, tx, v.ID)
		if err != nil {
			return v, err
		}
	}
	if err := appendMockVersionTx(ctx, tx, v, actorID, reason, at); err != nil {
		return v, err
	}
	if err := appendAuditTx(ctx, tx, newAudit(actorID, action, "mock_interview", v.ID, map[string]any{"reason": reason}, at)); err != nil {
		return v, err
	}
	if err := tx.Commit(ctx); err != nil {
		return v, err
	}
	return v, nil
}

func reconcileMockRounds(ctx context.Context, tx pgx.Tx, v mockinterviews.Interview) error {
	type storedRound struct {
		ID       string
		Kind     mockinterviews.RoundType
		Position int
	}
	rows, err := tx.Query(ctx, `SELECT id,kind::text,position FROM app.mock_interview_rounds WHERE mock_interview_id=$1`, v.ID)
	if err != nil {
		return err
	}
	existing, err := pgx.CollectRows(rows, pgx.RowToStructByName[storedRound])
	if err != nil {
		return err
	}
	byID := make(map[string]mockinterviews.RoundType, len(existing))
	positions := make(map[string]int, len(existing))
	maxPosition := len(v.Rounds)
	for _, round := range existing {
		byID[round.ID] = round.Kind
		positions[round.ID] = round.Position
		if round.Position > maxPosition {
			maxPosition = round.Position
		}
	}
	kept := make(map[string]bool, len(v.Rounds))
	reordered := false
	for position, round := range v.Rounds {
		kept[round.ID] = true
		if previous, exists := positions[round.ID]; exists && previous != position+1 {
			reordered = true
		}
	}
	for _, round := range existing {
		if !kept[round.ID] {
			if _, err := tx.Exec(ctx, `DELETE FROM app.mock_interview_rounds WHERE id = $1`, round.ID); err != nil {
				return err
			}
		}
	}
	// Move existing positions outside both ranges before reordering: the unique
	// (interview, position) constraint is checked immediately, including swaps.
	if reordered {
		if _, err := tx.Exec(ctx, `UPDATE app.mock_interview_rounds SET position=position + $1 WHERE mock_interview_id = $2`, maxPosition, v.ID); err != nil {
			return err
		}
	}
	for position, round := range v.Rounds {
		oldKind, exists := byID[round.ID]
		if !exists {
			if _, err := tx.Exec(ctx, `INSERT INTO app.mock_interview_rounds(id,mock_interview_id,position,kind)
	VALUES($1,$2,$3,$4)`, round.ID, v.ID, position+1, string(round.Type)); err != nil {
				return err
			}
		} else {
			if oldKind != round.Type {
				if err := deleteMockRoundSubtype(ctx, tx, round.ID, oldKind); err != nil {
					return err
				}
			}
			if reordered || oldKind != round.Type {
				if _, err := tx.Exec(ctx, `UPDATE app.mock_interview_rounds SET position=$1,kind=$2 WHERE id = $3`, position+1, string(round.Type), round.ID); err != nil {
					return err
				}
			}
		}
		if err := saveMockRoundSubtype(ctx, tx, round); err != nil {
			return err
		}
	}
	return nil
}

func deleteMockRoundSubtype(ctx context.Context, tx pgx.Tx, roundID string, kind mockinterviews.RoundType) error {
	var table string
	switch kind {
	case mockinterviews.Behavioural:
		table = "app.behavioural_mock_interview_rounds"
	case mockinterviews.LeetCode:
		table = "app.leetcode_mock_interview_rounds"
	case mockinterviews.Custom:
		table = "app.custom_mock_interview_rounds"
	default:
		return mockinterviews.ErrInvalid
	}
	_, err := tx.Exec(ctx, "DELETE FROM "+table+" WHERE mock_interview_round_id=$1", roundID)
	return err
}

func saveMockRoundSubtype(ctx context.Context, tx pgx.Tx, round mockinterviews.Round) error {
	switch round.Type {
	case mockinterviews.Behavioural:
		if round.Scores.Behavioural == nil {
			return mockinterviews.ErrInvalid
		}
		_, err := tx.Exec(ctx, `INSERT INTO app.behavioural_mock_interview_rounds(id,mock_interview_round_id,behavioural_score)
   VALUES($1,$2,$3) ON CONFLICT(mock_interview_round_id) DO UPDATE SET behavioural_score=EXCLUDED.behavioural_score`, id.New(), round.ID, *round.Scores.Behavioural)
		return err
	case mockinterviews.LeetCode:
		if round.Scores.ConfirmQuestions == nil || round.Scores.AlgorithmDesign == nil || round.Scores.ComplexityAnalysis == nil || round.Scores.Coding == nil || round.Scores.Testing == nil {
			return mockinterviews.ErrInvalid
		}
		_, err := tx.Exec(ctx, `INSERT INTO app.leetcode_mock_interview_rounds(id,mock_interview_round_id,leetcode_problem_id,clarify_question_score,algorithm_design_score,complexity_analysis_score,coding_score,testing_score)
   VALUES($1,$2,$3,$4,$5,$6,$7,$8) ON CONFLICT(mock_interview_round_id) DO UPDATE SET
   leetcode_problem_id=EXCLUDED.leetcode_problem_id,clarify_question_score=EXCLUDED.clarify_question_score,
   algorithm_design_score=EXCLUDED.algorithm_design_score,complexity_analysis_score=EXCLUDED.complexity_analysis_score,
   coding_score=EXCLUDED.coding_score,testing_score=EXCLUDED.testing_score`, id.New(), round.ID, round.ProblemID,
			*round.Scores.ConfirmQuestions, *round.Scores.AlgorithmDesign, *round.Scores.ComplexityAnalysis, *round.Scores.Coding, *round.Scores.Testing)
		return err
	case mockinterviews.Custom:
		if round.Scores.Custom == nil {
			return mockinterviews.ErrInvalid
		}
		_, err := tx.Exec(ctx, `INSERT INTO app.custom_mock_interview_rounds(id,mock_interview_round_id,content_html,url,score)
   VALUES($1,$2,$3,$4,$5) ON CONFLICT(mock_interview_round_id) DO UPDATE SET content_html=EXCLUDED.content_html,url=EXCLUDED.url,score=EXCLUDED.score`, id.New(), round.ID, round.Content, nullableString(round.Link), *round.Scores.Custom)
		return err
	default:
		return mockinterviews.ErrInvalid
	}
}

func appendMockVersionTx(ctx context.Context, tx pgx.Tx, v mockinterviews.Interview, actorID, reason string, at time.Time) error {
	raw, err := json.Marshal(v)
	if err != nil {
		return err
	}

	var version int64
	if err := tx.QueryRow(ctx, `SELECT COALESCE(MAX(version), 0) + 1 FROM app.mock_interview_versions WHERE mock_interview_id=$1`, v.ID).Scan(&version); err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO app.mock_interview_versions(id,mock_interview_id,version,actor_user_id,reason,snapshot,created_at)
	VALUES($1,$2,$3,$4,$5,$6::jsonb,$7)`, id.New(), v.ID, version, actorID, reason, string(raw), at.UTC())
	return err
}
