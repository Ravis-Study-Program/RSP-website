package postgres

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/magedmg/RSP-website/backend/internal/authz"
	"github.com/magedmg/RSP-website/backend/internal/mockinterviews"
	"github.com/magedmg/RSP-website/backend/internal/platform/dbtable"
	"github.com/magedmg/RSP-website/backend/internal/platform/id"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
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
	if err := p.DB.WithContext(ctx).Raw(`SELECT count(*) FROM app.mock_interviews mi WHERE mi.deleted_at IS NULL AND `+where, userID).Row().Scan(&total); err != nil {
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
	rows, err := p.DB.WithContext(ctx).Raw(query, userID, boundary, limit+1).Rows()
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
	var row struct {
		ID, AccountState                           string
		IsTest, ActiveMember, Alumni, FormerMember bool
		HasEnrollment, HasKicked                   bool
	}
	err := p.DB.WithContext(ctx).Raw(`SELECT u.id,u.account_state::text AS account_state,u.is_test,
		EXISTS(SELECT 1 FROM app.enrollments e WHERE e.user_id=u.id AND e.state='active' AND e.deleted_at IS NULL) AS active_member,
		EXISTS(SELECT 1 FROM app.enrollments e WHERE e.user_id=u.id AND e.role='student' AND e.state='completed' AND e.deleted_at IS NULL) AS alumni,
		EXISTS(SELECT 1 FROM app.enrollments e WHERE e.user_id=u.id AND e.state='completed' AND e.deleted_at IS NULL) AS former_member,
		EXISTS(SELECT 1 FROM app.enrollments e WHERE e.user_id=u.id AND e.deleted_at IS NULL) AS has_enrollment,
		EXISTS(SELECT 1 FROM app.enrollments e WHERE e.user_id=u.id AND e.state='kicked' AND e.deleted_at IS NULL) AS has_kicked
		FROM app.users u WHERE u.id=?`, userID).Scan(&row).Error
	if err != nil || row.ID == "" {
		if err == nil {
			err = gorm.ErrRecordNotFound
		}
		return mockinterviews.Participant{}, noRows(err)
	}

	ptn := mockinterviews.Participant{
		UserID:       row.ID,
		ActiveMember: row.ActiveMember,
		Alumni:       row.Alumni,
		FormerMember: row.FormerMember,
		Inactive:     row.AccountState != "active",
		Suspended:    row.AccountState == "suspended",
		Deleted:      row.AccountState == "deleted",
		Test:         row.IsTest,
	}
	ptn.KickedOnly = row.HasEnrollment && row.HasKicked && !ptn.ActiveMember && !ptn.FormerMember
	return ptn, nil
}

// GetMockInterview retrieves a value.
func (p *Postgres) GetMockInterview(ctx context.Context, interviewID string) (mockinterviews.Interview, error) {
	return loadMockInterview(ctx, p.DB, interviewID)
}

func loadMockInterview(ctx context.Context, db *gorm.DB, interviewID string) (mockinterviews.Interview, error) {
	var v mockinterviews.Interview
	err := db.WithContext(ctx).Raw(`SELECT id,interviewer_user_id,interviewee_user_id,season_id,
		scheduled_at,duration_minutes,COALESCE(interviewer_notes_html,''),revision,deleted_at
		FROM app.mock_interviews WHERE id=? AND deleted_at IS NULL`, interviewID).Row().Scan(
		&v.ID, &v.InterviewerID, &v.IntervieweeID, &v.SeasonID, &v.OccurredAt,
		&v.DurationMinutes, &v.Notes, &v.Revision, &v.DeletedAt,
	)
	if err != nil {
		return mockinterviews.Interview{}, noRows(err)
	}
	rows, err := db.WithContext(ctx).Raw(`SELECT r.id,r.kind::text,r.review_status='reviewed',
		COALESCE(r.interviewee_comment_html,''),b.behavioural_score,l.leetcode_problem_id,
		l.clarify_question_score,l.algorithm_design_score,l.complexity_analysis_score,
		l.coding_score,l.testing_score,COALESCE(c.content_html,''),COALESCE(c.url,''),c.score
		FROM app.mock_interview_rounds r
		LEFT JOIN app.behavioural_mock_interview_rounds b ON b.mock_interview_round_id=r.id
		LEFT JOIN app.leetcode_mock_interview_rounds l ON l.mock_interview_round_id=r.id
		LEFT JOIN app.custom_mock_interview_rounds c ON c.mock_interview_round_id=r.id
		WHERE r.mock_interview_id=? AND r.deleted_at IS NULL ORDER BY r.position,r.id`, interviewID).Rows()
	if err != nil {
		return v, err
	}
	defer rows.Close()
	for rows.Next() {
		var round mockinterviews.Round
		var kind string
		var behavioural, clarify, algorithm, complexity, coding, testing, custom *int16
		var problemID *string
		if err := rows.Scan(&round.ID, &kind, &round.Reviewed, &round.IntervieweeComment,
			&behavioural, &problemID, &clarify, &algorithm, &complexity, &coding, &testing,
			&round.Content, &round.Link, &custom); err != nil {
			return v, err
		}
		round.Type = mockinterviews.RoundType(kind)
		round.Scores = scoresFromDB(behavioural, clarify, algorithm, complexity, coding, testing, custom)
		if problemID != nil {
			round.ProblemID = *problemID
		}
		v.Rounds = append(v.Rounds, round)
	}
	if err := rows.Err(); err != nil {
		return v, err
	}
	return v, nil
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
	tx, err := begin(ctx, p.DB)
	if err != nil {
		return mockinterviews.Interview{}, err
	}

	defer rollback(tx)
	if err := tx.Table(dbtable.MockInterviews).Create(map[string]any{
		"id": v.ID, "interviewer_user_id": v.InterviewerID,
		"interviewee_user_id": v.IntervieweeID, "season_id": v.SeasonID,
		"scheduled_at": v.OccurredAt.UTC(), "duration_minutes": v.DurationMinutes,
		"interviewer_notes_html": v.Notes, "revision": v.Revision,
	}).Error; err != nil {
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
	if err := commit(tx); err != nil {
		return v, err
	}
	return v, nil
}

// UpdateMockInterview updates a value.
func (p *Postgres) UpdateMockInterview(ctx context.Context, v mockinterviews.Interview, actorID, reason string, at time.Time) (mockinterviews.Interview, error) {
	tx, err := begin(ctx, p.DB)
	if err != nil {
		return mockinterviews.Interview{}, err
	}

	defer rollback(tx)
	var currentRevision int64
	err = tx.Table(dbtable.MockInterviews).Select("revision").Where("id = ? AND deleted_at IS NULL", v.ID).
		Clauses(clause.Locking{Strength: "UPDATE"}).Row().Scan(&currentRevision)
	if err != nil {
		return v, noRows(err)
	}
	if v.Revision != currentRevision+1 {
		return v, ErrConflict
	}
	if reason == "soft deleted" {
		if err := tx.Table(dbtable.MockInterviews).Where("id = ?", v.ID).
			Updates(map[string]any{"deleted_at": v.DeletedAt, "revision": v.Revision}).Error; err != nil {
			return v, err
		}
	} else {
		if err := tx.Table(dbtable.MockInterviews).Where("id = ?", v.ID).Updates(map[string]any{
			"interviewer_user_id": v.InterviewerID, "interviewee_user_id": v.IntervieweeID,
			"season_id": v.SeasonID, "scheduled_at": v.OccurredAt.UTC(),
			"duration_minutes": v.DurationMinutes, "interviewer_notes_html": v.Notes,
			"revision": v.Revision,
		}).Error; err != nil {
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
	if err := commit(tx); err != nil {
		return v, err
	}
	return v, nil
}

func replaceMockRounds(ctx context.Context, tx *gorm.DB, v mockinterviews.Interview) error {
	if err := tx.WithContext(ctx).Table(dbtable.MockInterviewRounds).Where("mock_interview_id = ?", v.ID).Delete(nil).Error; err != nil {
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
		if err := tx.Table(dbtable.MockInterviewRounds).Create(map[string]any{
			"id": round.ID, "mock_interview_id": v.ID, "position": position + 1,
			"kind": string(round.Type), "review_status": status,
			"interviewee_comment_html": round.IntervieweeComment,
			"reviewed_at":              reviewedAt, "revision": v.Revision,
		}).Error; err != nil {
			return err
		}

		switch round.Type {
		case mockinterviews.Behavioural:
			if round.Scores.Behavioural == nil {
				return mockinterviews.ErrInvalid
			}
			err = tx.Table(dbtable.BehaviouralMockInterviewRounds).Create(map[string]any{
				"id": id.New(), "mock_interview_round_id": round.ID, "behavioural_score": *round.Scores.Behavioural,
			}).Error
		case mockinterviews.LeetCode:
			if round.Scores.ConfirmQuestions == nil || round.Scores.AlgorithmDesign == nil || round.Scores.ComplexityAnalysis == nil || round.Scores.Coding == nil || round.Scores.Testing == nil {
				return mockinterviews.ErrInvalid
			}
			err = tx.Table(dbtable.LeetcodeMockInterviewRounds).Create(map[string]any{
				"id": id.New(), "mock_interview_round_id": round.ID,
				"leetcode_problem_id":       round.ProblemID,
				"clarify_question_score":    *round.Scores.ConfirmQuestions,
				"algorithm_design_score":    *round.Scores.AlgorithmDesign,
				"complexity_analysis_score": *round.Scores.ComplexityAnalysis,
				"coding_score":              *round.Scores.Coding, "testing_score": *round.Scores.Testing,
			}).Error
		case mockinterviews.Custom:
			if round.Scores.Custom == nil {
				return mockinterviews.ErrInvalid
			}
			var link *string
			if round.Link != "" {
				link = &round.Link
			}
			err = tx.Table(dbtable.CustomMockInterviewRounds).Create(map[string]any{
				"id": id.New(), "mock_interview_round_id": round.ID,
				"content_html": round.Content, "url": link, "score": *round.Scores.Custom,
			}).Error
		default:
			return mockinterviews.ErrInvalid
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func appendMockVersionTx(ctx context.Context, tx *gorm.DB, v mockinterviews.Interview, actorID, reason string, at time.Time) error {
	raw, err := json.Marshal(v)
	if err != nil {
		return err
	}

	return tx.WithContext(ctx).Table(dbtable.MockInterviewVersions).Create(map[string]any{
		"id": id.New(), "mock_interview_id": v.ID, "version": v.Revision,
		"actor_user_id": actorID, "reason": reason,
		"snapshot": gorm.Expr("?::jsonb", string(raw)), "created_at": at.UTC(),
	}).Error
}

// AppendAudit performs the operation.
