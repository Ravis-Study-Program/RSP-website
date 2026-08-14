-- name: ListSeasonEnrollments :many
SELECT e.*, u.slug, u.display_name, u.avatar_url
FROM app.enrollments AS e
JOIN app.users AS u ON u.id = e.user_id
WHERE e.season_id = sqlc.arg(season_id)
  AND e.deleted_at IS NULL
  AND (sqlc.narg(role)::app.season_role IS NULL OR e.role = sqlc.narg(role))
  AND (sqlc.narg(state)::app.enrollment_state IS NULL OR e.state = sqlc.narg(state))
  AND (sqlc.narg(after_id)::text IS NULL OR (u.display_name, e.id) > (sqlc.narg(after_name)::text, sqlc.narg(after_id)::text))
ORDER BY u.display_name ASC, e.id ASC
LIMIT sqlc.arg(page_limit);

-- name: CreateEnrollment :one
INSERT INTO app.enrollments (
  id, user_id, season_id, role, student_level
) VALUES (
  sqlc.arg(id), sqlc.arg(user_id), sqlc.arg(season_id),
  sqlc.arg(role), sqlc.arg(student_level)
)
RETURNING *;

-- name: PromoteStudentLevel :one
UPDATE app.enrollments
SET student_level = sqlc.arg(student_level), revision = revision + 1
WHERE id = sqlc.arg(id)
  AND role = 'student'
  AND state = 'active'
  AND revision = sqlc.arg(revision)
  AND deleted_at IS NULL
RETURNING *;

-- name: RemoveEnrollment :one
UPDATE app.enrollments
SET state = sqlc.arg(resulting_state),
    state_changed_at = sqlc.arg(occurred_at),
    assignment_state = 'revoked',
    activated_at = NULL,
    revision = revision + 1
WHERE id = sqlc.arg(id)
  AND state = 'active'
  AND revision = sqlc.arg(revision)
  AND deleted_at IS NULL
RETURNING *;

-- name: RecordEnrollmentRemoval :one
INSERT INTO app.enrollment_removal_events (
  id, enrollment_id, season_id, subject_user_id, actor_user_id,
  resulting_state, reason, occurred_at
) VALUES (
  sqlc.arg(id), sqlc.arg(enrollment_id), sqlc.arg(season_id),
  sqlc.arg(subject_user_id), sqlc.narg(actor_user_id),
  sqlc.arg(resulting_state), sqlc.arg(reason), sqlc.arg(occurred_at)
)
RETURNING *;

-- name: CreateMentorship :one
INSERT INTO app.mentorships (
  id, season_id, mentor_enrollment_id, student_enrollment_id
) VALUES (
  sqlc.arg(id), sqlc.arg(season_id), sqlc.arg(mentor_enrollment_id),
  sqlc.arg(student_enrollment_id)
)
RETURNING *;

-- name: EndMentorship :one
UPDATE app.mentorships
SET ended_at = sqlc.arg(ended_at), revision = revision + 1
WHERE id = sqlc.arg(id)
  AND ended_at IS NULL
  AND revision = sqlc.arg(revision)
  AND deleted_at IS NULL
RETURNING *;

-- name: ListMentorshipsForSeason :many
SELECT m.*,
       mentor_user.id AS mentor_user_id,
       mentor_user.display_name AS mentor_name,
       student_user.id AS student_user_id,
       student_user.display_name AS student_name
FROM app.mentorships AS m
JOIN app.enrollments AS mentor_enrollment ON mentor_enrollment.id = m.mentor_enrollment_id
JOIN app.users AS mentor_user ON mentor_user.id = mentor_enrollment.user_id
JOIN app.enrollments AS student_enrollment ON student_enrollment.id = m.student_enrollment_id
JOIN app.users AS student_user ON student_user.id = student_enrollment.user_id
WHERE m.season_id = $1 AND m.ended_at IS NULL AND m.deleted_at IS NULL
ORDER BY mentor_user.display_name, student_user.display_name, m.id;
