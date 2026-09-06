package dal

import (
	"context"
	"encoding/json"

	"github.com/magedmg/RSP-website/backend/internal/programme"
)

func (p *Store) ListUserParticipation(ctx context.Context, userID string) ([]programme.Participation, error) {
	rows, err := p.pool.Query(ctx, `SELECT e.id,e.season_id,s.slug,e.user_id,e.role::text,e.student_level::text,e.state::text,e.assignment_state::text,
 s.name,s.start_at,s.end_at,
 COALESCE(NULLIF(e.student_level::text,'not_applicable'),(
  SELECT data->>'studentLevel' FROM app.audit_events
  WHERE subject_type='enrollment' AND subject_id=e.id::text
   AND data->>'studentLevel' IN ('novice','beginner','intermediate','advanced')
  ORDER BY occurred_at DESC,id DESC LIMIT 1),'not_applicable'),
 COALESCE((SELECT jsonb_agg(jsonb_build_object('role',p.role,'startedAt',p.started_at,'endedAt',p.ended_at) ORDER BY p.started_at,p.id)
  FROM app.enrollment_participation p WHERE p.enrollment_id=e.id),'[]'::jsonb)
 FROM app.enrollments e JOIN app.seasons s ON s.id=e.season_id
 WHERE e.user_id=$1 AND (e.deleted_at IS NULL OR e.state IN ('kicked','withdrawn')) AND s.deleted_at IS NULL
 ORDER BY s.start_at DESC,e.id`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []programme.Participation{}
	for rows.Next() {
		var item programme.Participation
		var periods []byte
		if err := rows.Scan(&item.ID, &item.SeasonID, &item.SeasonSlug, &item.UserID, &item.Role, &item.StudentLevel, &item.State, &item.AssignmentState, &item.SeasonName, &item.StartAt, &item.EndAt, &item.LastStudentLevel, &periods); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(periods, &item.Periods); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}
