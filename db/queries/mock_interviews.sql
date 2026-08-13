-- name: CreateMockInterview :one
INSERT INTO app.mock_interviews (
  id, interviewer_user_id, interviewee_user_id, season_id, season_week_id,
  scheduled_at, duration_minutes, interviewer_notes_html
) VALUES (
  sqlc.arg(id), sqlc.arg(interviewer_user_id), sqlc.arg(interviewee_user_id),
  sqlc.narg(season_id), sqlc.narg(season_week_id), sqlc.arg(scheduled_at),
  sqlc.arg(duration_minutes), sqlc.narg(interviewer_notes_html)
)
RETURNING *;

-- name: GetMockInterview :one
SELECT * FROM app.mock_interviews WHERE id = $1 AND deleted_at IS NULL;

-- name: ListMockInterviewsForParticipant :many
SELECT mi.*,
       interviewer.slug AS interviewer_slug,
       interviewer.display_name AS interviewer_name,
       interviewee.slug AS interviewee_slug,
       interviewee.display_name AS interviewee_name
FROM app.mock_interviews AS mi
JOIN app.users AS interviewer ON interviewer.id = mi.interviewer_user_id
JOIN app.users AS interviewee ON interviewee.id = mi.interviewee_user_id
WHERE mi.deleted_at IS NULL
  AND (
    sqlc.arg(mode)::text = 'all'
    OR (sqlc.arg(mode)::text = 'given' AND mi.interviewer_user_id = sqlc.arg(user_id))
    OR (sqlc.arg(mode)::text = 'received' AND mi.interviewee_user_id = sqlc.arg(user_id))
  )
  AND (sqlc.narg(after_id)::text IS NULL OR (mi.scheduled_at, mi.id) < (sqlc.narg(after_at)::timestamptz, sqlc.narg(after_id)::text))
ORDER BY mi.scheduled_at DESC, mi.id DESC
LIMIT sqlc.arg(page_limit);

-- name: UpdateMockInterview :one
UPDATE app.mock_interviews
SET scheduled_at = sqlc.arg(scheduled_at),
    duration_minutes = sqlc.arg(duration_minutes),
    interviewer_notes_html = sqlc.narg(interviewer_notes_html),
    revision = revision + 1
WHERE id = sqlc.arg(id)
  AND interviewer_user_id = sqlc.arg(actor_user_id)
  AND revision = sqlc.arg(revision)
  AND deleted_at IS NULL
RETURNING *;

-- name: SoftDeleteMockInterview :one
UPDATE app.mock_interviews
SET deleted_at = sqlc.arg(deleted_at), revision = revision + 1
WHERE id = sqlc.arg(id)
  AND interviewer_user_id = sqlc.arg(actor_user_id)
  AND revision = sqlc.arg(revision)
  AND deleted_at IS NULL
RETURNING revision;

-- name: ListMockInterviewRounds :many
SELECT * FROM app.mock_interview_rounds
WHERE mock_interview_id = $1 AND deleted_at IS NULL
ORDER BY position, id;

-- name: UpdateRoundReview :one
UPDATE app.mock_interview_rounds AS r
SET review_status = sqlc.arg(review_status),
    interviewee_comment_html = sqlc.narg(interviewee_comment_html),
    reviewed_at = sqlc.narg(reviewed_at),
    revision = r.revision + 1
FROM app.mock_interviews AS mi
WHERE r.id = sqlc.arg(round_id)
  AND mi.id = r.mock_interview_id
  AND mi.interviewee_user_id = sqlc.arg(actor_user_id)
  AND r.revision = sqlc.arg(revision)
  AND r.deleted_at IS NULL
RETURNING r.*;

-- name: AppendMockInterviewVersion :one
INSERT INTO app.mock_interview_versions (
  id, mock_interview_id, version, actor_user_id, reason, snapshot
) VALUES (
  sqlc.arg(id), sqlc.arg(mock_interview_id), sqlc.arg(version),
  sqlc.narg(actor_user_id), sqlc.arg(reason), sqlc.arg(snapshot)
)
RETURNING *;
