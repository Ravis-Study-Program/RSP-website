package postgres

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/magedmg/RSP-website/backend/internal/authz"
	"github.com/magedmg/RSP-website/backend/internal/mockinterviews"
	"github.com/magedmg/RSP-website/backend/internal/platform/id"
)

func (p *Postgres) ListMockInterviews(ctx context.Context, actor authz.Actor, mode, boundary string, limit int, sortBy, direction string) ([]mockinterviews.Interview, bool, int64, error) {
	userID := actor.UserID
	where := `(mi.interviewer_user_id=$1 OR mi.interviewee_user_id=$1)`
	if mode == "given" {
		where = `mi.interviewer_user_id=$1`
	} else if mode == "received" {
		where = `mi.interviewee_user_id=$1`
	} else if mode == "all" && actor.IsPrivileged() {
		where = `$1::text IS NOT NULL`
	} else if mode == "all" {
		where = `(mi.interviewer_user_id=$1 OR mi.interviewee_user_id=$1 OR EXISTS (
			SELECT 1 FROM app.enrollments viewer
			WHERE viewer.user_id=$1 AND viewer.season_id=mi.season_id AND viewer.state IN ('active','completed') AND viewer.deleted_at IS NULL
			AND (viewer.role='coordinator' OR (viewer.role='mentor' AND EXISTS (
				SELECT 1 FROM app.mentorships m
				JOIN app.enrollments mentor ON mentor.id=m.mentor_enrollment_id
				JOIN app.enrollments student ON student.id=m.student_enrollment_id
				WHERE m.season_id=mi.season_id AND m.ended_at IS NULL AND m.deleted_at IS NULL
				AND mentor.user_id=$1 AND student.user_id IN (mi.interviewer_user_id,mi.interviewee_user_id)
		)))))`
	}
	var total int64
	if err := p.Pool.QueryRow(ctx, `SELECT count(*) FROM app.mock_interviews mi WHERE mi.deleted_at IS NULL AND `+where, userID).Scan(&total); err != nil {
		return nil, false, 0, err
	}

	comparator, order := ">", "ASC"
	key, boundaryKey := "mi.id", "b.id"
	boundarySelect := "id"
	if direction == "backward" {
		comparator, order = "<", "DESC"
	}
	if sortBy == "occurredAt:desc" {
		key, boundaryKey = "ROW(mi.scheduled_at,mi.id)", "ROW(b.scheduled_at,b.id)"
		boundarySelect = "scheduled_at,id"
		comparator, order = "<", "DESC"
		if direction == "backward" {
			comparator, order = ">", "ASC"
		}
	}
	orderBy := "mi.id " + order
	if sortBy == "occurredAt:desc" {
		orderBy = "mi.scheduled_at " + order + ",mi.id " + order
	}
	query := `WITH boundary AS (
		SELECT ` + boundarySelect + ` FROM app.mock_interviews WHERE id=NULLIF($2,'')::uuid AND deleted_at IS NULL
	) SELECT mi.id FROM app.mock_interviews mi
	WHERE mi.deleted_at IS NULL AND ` + where + `
	  AND (NULLIF($2,'')::uuid IS NULL OR EXISTS (SELECT 1 FROM boundary b WHERE ` + key + ` ` + comparator + ` ` + boundaryKey + `))
	ORDER BY ` + orderBy + ` LIMIT $3`
	rows, err := p.Pool.Query(ctx, query, userID, boundary, limit+1)
	if err != nil {
		return nil, false, 0, err
	}

	defer rows.Close()
	ids := []string{}
	for rows.Next() {
		var interviewID string
		if err := rows.Scan(&interviewID); err != nil {
			return nil, false, 0, err
		}

		ids = append(ids, interviewID)
	}
	if err := rows.Err(); err != nil {
		return nil, false, 0, err
	}

	ids, more := finishPostgresPage(ids, limit, direction)
	items := make([]mockinterviews.Interview, 0, len(ids))
	for _, interviewID := range ids {
		v, err := p.GetMockInterview(ctx, interviewID)
		if err != nil {
			return nil, false, 0, err
		}

		items = append(items, v)
	}
	return items, more, total, nil
}

// GetMockParticipant retrieves a value.
func (p *Postgres) GetMockParticipant(ctx context.Context, userID string) (mockinterviews.Participant, error) {
	var ptn mockinterviews.Participant
	var state string
	var hasEnrollment, hasKicked bool
	err := p.Pool.QueryRow(ctx, `SELECT u.id,u.account_state::text,u.is_test,EXISTS(SELECT 1 FROM app.enrollments e WHERE e.user_id=u.id AND e.state='active' AND e.deleted_at IS NULL),EXISTS(SELECT 1 FROM app.enrollments e WHERE e.user_id=u.id AND e.role='student' AND e.state='completed' AND e.deleted_at IS NULL),EXISTS(SELECT 1 FROM app.enrollments e WHERE e.user_id=u.id AND e.state='completed' AND e.deleted_at IS NULL),EXISTS(SELECT 1 FROM app.enrollments e WHERE e.user_id=u.id AND e.deleted_at IS NULL),EXISTS(SELECT 1 FROM app.enrollments e WHERE e.user_id=u.id AND e.state='kicked' AND e.deleted_at IS NULL) FROM app.users u WHERE u.id=$1`, userID).Scan(&ptn.UserID, &state, &ptn.Test, &ptn.ActiveMember, &ptn.Alumni, &ptn.FormerMember, &hasEnrollment, &hasKicked)
	if err != nil {
		return ptn, noRows(err)
	}

	ptn.Suspended = state == "suspended"
	ptn.Deleted = state == "deleted"
	ptn.Inactive = state != "active"
	ptn.KickedOnly = hasEnrollment && hasKicked && !ptn.ActiveMember && !ptn.FormerMember
	return ptn, nil
}

// GetMockInterview retrieves a value.
func (p *Postgres) GetMockInterview(ctx context.Context, interviewID string) (mockinterviews.Interview, error) {
	return loadMockInterview(ctx, p.Pool, interviewID)
}

type rowQuerier interface {
	QueryRow(context.Context, string, ...any) pgx.Row
	Query(context.Context, string, ...any) (pgx.Rows, error)
}

func loadMockInterview(ctx context.Context, db rowQuerier, interviewID string) (mockinterviews.Interview, error) {
	var v mockinterviews.Interview
	if err := db.QueryRow(ctx, `SELECT id,interviewer_user_id,interviewee_user_id,season_id,scheduled_at,duration_minutes,COALESCE(interviewer_notes_html,''),revision,deleted_at FROM app.mock_interviews WHERE id=$1 AND deleted_at IS NULL`, interviewID).Scan(&v.ID, &v.InterviewerID, &v.IntervieweeID, &v.SeasonID, &v.OccurredAt, &v.DurationMinutes, &v.Notes, &v.Revision, &v.DeletedAt); err != nil {
		return v, noRows(err)
	}

	rows, err := db.Query(ctx, `SELECT r.id,r.kind::text,r.review_status='reviewed',COALESCE(r.interviewee_comment_html,''),b.behavioural_score,l.leetcode_problem_id,l.clarify_question_score,l.algorithm_design_score,l.complexity_analysis_score,l.coding_score,l.testing_score,COALESCE(c.content_html,''),COALESCE(c.url,''),c.score FROM app.mock_interview_rounds r LEFT JOIN app.behavioural_mock_interview_rounds b ON b.mock_interview_round_id=r.id LEFT JOIN app.leetcode_mock_interview_rounds l ON l.mock_interview_round_id=r.id LEFT JOIN app.custom_mock_interview_rounds c ON c.mock_interview_round_id=r.id WHERE r.mock_interview_id=$1 AND r.deleted_at IS NULL ORDER BY r.position,r.id`, interviewID)
	if err != nil {
		return v, err
	}

	defer rows.Close()
	for rows.Next() {
		var round mockinterviews.Round
		var kind string
		var behavioural, clarify, algorithm, complexity, coding, testing, custom *int16
		var problemID *string
		if err := rows.Scan(&round.ID, &kind, &round.Reviewed, &round.IntervieweeComment, &behavioural, &problemID, &clarify, &algorithm, &complexity, &coding, &testing, &round.Content, &round.Link, &custom); err != nil {
			return v, err
		}

		round.Type = mockinterviews.RoundType(kind)
		if problemID != nil {
			round.ProblemID = *problemID
		}
		round.Scores = scoresFromDB(behavioural, clarify, algorithm, complexity, coding, testing, custom)
		v.Rounds = append(v.Rounds, round)
	}
	return v, rows.Err()
}

func scoresFromDB(values ...*int16) mockinterviews.Scores {
	toInt := func(v *int16) *int {
		if v == nil {
			return nil
		}
		n := int(*v)
		return &n
	}
	return mockinterviews.Scores{Behavioural: toInt(values[0]), ConfirmQuestions: toInt(values[1]), AlgorithmDesign: toInt(values[2]), ComplexityAnalysis: toInt(values[3]), Coding: toInt(values[4]), Testing: toInt(values[5]), Custom: toInt(values[6])}
}

// CreateMockInterview creates a value.
func (p *Postgres) CreateMockInterview(ctx context.Context, v mockinterviews.Interview, actorID string, at time.Time) (mockinterviews.Interview, error) {
	tx, err := p.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return mockinterviews.Interview{}, err
	}

	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `INSERT INTO app.mock_interviews(id,interviewer_user_id,interviewee_user_id,season_id,scheduled_at,duration_minutes,interviewer_notes_html,revision) VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, v.ID, v.InterviewerID, v.IntervieweeID, v.SeasonID, v.OccurredAt, v.DurationMinutes, v.Notes, v.Revision); err != nil {
		return v, mapPostgresError(err)
	}
	if err := replaceMockRounds(ctx, tx, v); err != nil {
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

// UpdateMockInterview updates a value.
func (p *Postgres) UpdateMockInterview(ctx context.Context, v mockinterviews.Interview, actorID, reason string, at time.Time) (mockinterviews.Interview, error) {
	tx, err := p.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return mockinterviews.Interview{}, err
	}

	defer tx.Rollback(ctx)
	var currentRevision int64
	if err := tx.QueryRow(ctx, `SELECT revision FROM app.mock_interviews WHERE id=$1 AND deleted_at IS NULL FOR UPDATE`, v.ID).Scan(&currentRevision); err != nil {
		return v, noRows(err)
	}
	if v.Revision != currentRevision+1 {
		return v, ErrConflict
	}
	if reason == "soft deleted" {
		if _, err := tx.Exec(ctx, `UPDATE app.mock_interviews SET deleted_at=$2,revision=$3 WHERE id=$1`, v.ID, v.DeletedAt, v.Revision); err != nil {
			return v, err
		}
	} else {
		if _, err := tx.Exec(ctx, `UPDATE app.mock_interviews SET interviewer_user_id=$2,interviewee_user_id=$3,season_id=$4,scheduled_at=$5,duration_minutes=$6,interviewer_notes_html=$7,revision=$8 WHERE id=$1`, v.ID, v.InterviewerID, v.IntervieweeID, v.SeasonID, v.OccurredAt, v.DurationMinutes, v.Notes, v.Revision); err != nil {
			return v, err
		}
		if err := replaceMockRounds(ctx, tx, v); err != nil {
			return v, err
		}
	}
	if err := appendMockVersionTx(ctx, tx, v, actorID, reason, at); err != nil {
		return v, err
	}

	action := "mock_interview.updated"
	if reason == "soft deleted" {
		action = "mock_interview.deleted"
	} else if strings.HasPrefix(reason, "identity correction:") {
		action = "mock_interview.identities_corrected"
	} else if reason == "interviewee review" {
		action = "mock_interview.reviewed"
	}
	if err := appendAuditTx(ctx, tx, newAudit(actorID, action, "mock_interview", v.ID, map[string]any{"reason": reason}, at)); err != nil {
		return v, err
	}
	if err := tx.Commit(ctx); err != nil {
		return v, err
	}
	return v, nil
}

func replaceMockRounds(ctx context.Context, tx pgx.Tx, v mockinterviews.Interview) error {
	if _, err := tx.Exec(ctx, `DELETE FROM app.mock_interview_rounds WHERE mock_interview_id=$1`, v.ID); err != nil {
		return err
	}

	for position, round := range v.Rounds {
		var err error
		status := "pending"
		var reviewedAt *time.Time
		if round.Reviewed {
			status = "reviewed"
			now := time.Now().UTC()
			reviewedAt = &now
		}
		if _, err := tx.Exec(ctx, `INSERT INTO app.mock_interview_rounds(id,mock_interview_id,position,kind,review_status,interviewee_comment_html,reviewed_at,revision) VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, round.ID, v.ID, position+1, string(round.Type), status, round.IntervieweeComment, reviewedAt, v.Revision); err != nil {
			return err
		}

		switch round.Type {
		case mockinterviews.Behavioural:
			if round.Scores.Behavioural == nil {
				return mockinterviews.ErrInvalid
			}
			_, err = tx.Exec(ctx, `INSERT INTO app.behavioural_mock_interview_rounds(id,mock_interview_round_id,behavioural_score) VALUES($1,$2,$3)`, id.New(), round.ID, *round.Scores.Behavioural)
		case mockinterviews.LeetCode:
			if round.Scores.ConfirmQuestions == nil || round.Scores.AlgorithmDesign == nil || round.Scores.ComplexityAnalysis == nil || round.Scores.Coding == nil || round.Scores.Testing == nil {
				return mockinterviews.ErrInvalid
			}
			_, err = tx.Exec(ctx, `INSERT INTO app.leetcode_mock_interview_rounds(id,mock_interview_round_id,leetcode_problem_id,clarify_question_score,algorithm_design_score,complexity_analysis_score,coding_score,testing_score) VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, id.New(), round.ID, round.ProblemID, *round.Scores.ConfirmQuestions, *round.Scores.AlgorithmDesign, *round.Scores.ComplexityAnalysis, *round.Scores.Coding, *round.Scores.Testing)
		case mockinterviews.Custom:
			if round.Scores.Custom == nil {
				return mockinterviews.ErrInvalid
			}
			_, err = tx.Exec(ctx, `INSERT INTO app.custom_mock_interview_rounds(id,mock_interview_round_id,content_html,url,score) VALUES($1,$2,$3,NULLIF($4,''),$5)`, id.New(), round.ID, round.Content, round.Link, *round.Scores.Custom)
		default:
			return mockinterviews.ErrInvalid
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func appendMockVersionTx(ctx context.Context, tx pgx.Tx, v mockinterviews.Interview, actorID, reason string, at time.Time) error {
	raw, err := json.Marshal(v)
	if err != nil {
		return err
	}

	_, err = tx.Exec(ctx, `INSERT INTO app.mock_interview_versions(id,mock_interview_id,version,actor_user_id,reason,snapshot,created_at) VALUES($1,$2,$3,$4,$5,$6,$7)`, id.New(), v.ID, v.Revision, actorID, reason, raw, at.UTC())
	return err
}

// AppendAudit performs the operation.
