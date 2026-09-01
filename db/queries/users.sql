-- name: ResolveActiveAuthSubject :one
SELECT u.*
FROM app.user_auth_links AS l
JOIN app.users AS u ON u.id = l.user_id
WHERE l.auth_subject = $1
  AND l.active
  AND u.account_state = 'active'
  AND u.deleted_at IS NULL;

-- name: GetUserPrivateByID :one
SELECT * FROM app.users
WHERE (id::text = sqlc.arg(id)::text OR lower(slug) = lower(sqlc.arg(id)::text))
  AND deleted_at IS NULL;

-- name: GetUserPublicBySlug :one
SELECT id, slug, display_name, avatar_url, timezone, created_at
FROM app.users
WHERE lower(slug) = lower($1)
  AND account_state = 'active'
  AND NOT is_test
  AND deleted_at IS NULL;

-- name: ListDirectoryUsers :many
SELECT DISTINCT u.id, u.slug, u.display_name, u.avatar_url, u.timezone, u.created_at
FROM app.users AS u
JOIN app.enrollments AS e ON e.user_id = u.id
WHERE u.account_state = 'active'
  AND NOT u.is_test
  AND u.deleted_at IS NULL
  AND e.deleted_at IS NULL
  AND e.state IN ('active', 'completed')
  AND (sqlc.narg(after_id)::uuid IS NULL OR (u.display_name, u.id) > (sqlc.narg(after_name)::text, sqlc.narg(after_id)::uuid))
ORDER BY u.display_name ASC, u.id ASC
LIMIT sqlc.arg(page_limit);

-- name: UpdateUserProfile :one
UPDATE app.users
SET slug = sqlc.arg(slug),
    display_name = sqlc.arg(display_name),
    avatar_url = sqlc.narg(avatar_url),
    timezone = sqlc.arg(timezone),
    revision = revision + 1
WHERE id = sqlc.arg(id)
  AND revision = sqlc.arg(revision)
  AND account_state = 'active'
  AND deleted_at IS NULL
RETURNING *;

-- name: ListGlobalRolesForUser :many
SELECT role, state, granted_at, activated_at
FROM app.global_role_assignments
WHERE user_id = $1 AND state <> 'revoked'
ORDER BY role;

-- name: RevokeAuthLinksForUser :execrows
UPDATE app.user_auth_links
SET active = false, revoked_at = sqlc.arg(revoked_at)
WHERE user_id = sqlc.arg(user_id) AND active;

-- name: BeginAccountDeletion :one
UPDATE app.users
SET account_state = 'deletion_pending',
    deletion_requested_at = sqlc.arg(requested_at),
    deletion_due_at = sqlc.arg(due_at),
    revision = revision + 1
WHERE id = sqlc.arg(user_id)
  AND account_state = 'active'
  AND revision = sqlc.arg(revision)
RETURNING *;

-- name: CancelAccountDeletion :one
UPDATE app.users
SET account_state = 'active',
    deletion_requested_at = NULL,
    deletion_due_at = NULL,
    revision = revision + 1
WHERE id = sqlc.arg(user_id)
  AND account_state = 'deletion_pending'
RETURNING *;

-- name: ListDeletionRequestsDue :many
SELECT r.*, u.revision AS user_revision
FROM app.account_deletion_requests AS r
JOIN app.users AS u ON u.id = r.user_id
WHERE r.cancelled_at IS NULL
  AND r.completed_at IS NULL
  AND r.expires_at <= sqlc.arg(now_at)
  AND u.account_state = 'deletion_pending'
ORDER BY r.expires_at, r.id
LIMIT sqlc.arg(batch_limit)
FOR UPDATE OF r, u SKIP LOCKED;

-- name: IsAlumni :one
SELECT EXISTS (
  SELECT 1
  FROM app.enrollments
  WHERE user_id = $1
    AND role = 'student'
    AND state = 'completed'
    AND deleted_at IS NULL
) AS is_alumni;
