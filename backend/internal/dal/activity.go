package dal

import (
	"context"

	"github.com/magedmg/RSP-website/backend/internal/programme"
)

// The same derived views power the lists and their totals. SQL aggregates keep
// roster reads bounded without downloading each person's activity history.
const activitySummaryJoins = `
 LEFT JOIN LATERAL (
  SELECT count(*) AS count,max(a.attempted_at) AS latest
  FROM app.scoped_problem_attempts a
  WHERE a.deleted_at IS NULL AND (subject.user_id IS NULL OR a.user_id=subject.user_id)
   AND ($1='' OR a.activity_season_id=NULLIF($1,'')::uuid)
   AND ($2=0 OR EXTRACT(YEAR FROM a.attempted_at AT TIME ZONE 'Australia/Adelaide')=$2)
 ) attempts ON true
 LEFT JOIN LATERAL (
  SELECT count(*) AS count,max(m.scheduled_at) AS latest,
   count(*) FILTER (WHERE subject.user_id IS NULL OR m.interviewee_user_id=subject.user_id) AS received,
   count(*) FILTER (WHERE subject.user_id IS NULL OR m.interviewer_user_id=subject.user_id) AS conducted
  FROM app.scoped_mock_interviews m
  WHERE m.deleted_at IS NULL
   AND (subject.user_id IS NULL OR m.interviewee_user_id=subject.user_id OR m.interviewer_user_id=subject.user_id)
   AND ($1='' OR m.activity_season_id=NULLIF($1,'')::uuid)
   AND ($2=0 OR EXTRACT(YEAR FROM m.scheduled_at AT TIME ZONE 'Australia/Adelaide')=$2)
 ) mocks ON true`

const activitySummaryColumns = `attempts.count,mocks.count,mocks.received,mocks.conducted,GREATEST(attempts.latest,mocks.latest)`

func (p *Store) GetActivitySummary(ctx context.Context, userID, seasonID string, year int) (programme.ActivitySummary, error) {
	var summary programme.ActivitySummary
	err := p.pool.QueryRow(ctx, `SELECT `+activitySummaryColumns+`
 FROM (SELECT NULLIF($3,'')::uuid AS user_id) subject`+activitySummaryJoins, seasonID, year, userID).
		Scan(&summary.AttemptCount, &summary.MockInterviewCount, &summary.MocksReceived, &summary.MocksConducted, &summary.LastActivityAt)
	return summary, err
}

func (p *Store) SeasonMemberSummaries(ctx context.Context, seasonID string, userIDs []string) (map[string]programme.SeasonMemberSummary, error) {
	out := map[string]programme.SeasonMemberSummary{}
	if len(userIDs) == 0 {
		return out, nil
	}
	rows, err := p.pool.Query(ctx, `SELECT subject.user_id,profile.slug,profile.display_name,profile.avatar_url,`+activitySummaryColumns+`
 FROM (SELECT unnest($3::uuid[]) AS user_id) subject
 JOIN app.user_profiles profile ON profile.user_id=subject.user_id
 JOIN app.users u ON u.id=subject.user_id AND u.account_state<>'deleted' AND u.deleted_at IS NULL
 `+activitySummaryJoins, seasonID, 0, userIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var member programme.SeasonMemberSummary
		v := &member.Activity
		if err := rows.Scan(&member.ID, &member.Slug, &member.Name, &member.AvatarURL, &v.AttemptCount, &v.MockInterviewCount, &v.MocksReceived, &v.MocksConducted, &v.LastActivityAt); err != nil {
			return nil, err
		}
		out[member.ID] = member
	}
	return out, rows.Err()
}
