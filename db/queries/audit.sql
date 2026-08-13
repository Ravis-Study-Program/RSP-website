-- name: AppendAuditEvent :one
INSERT INTO app.audit_events (
  id, actor_user_id, action, subject_type, subject_id, request_id, ip_hash, data, occurred_at
) VALUES (
  sqlc.arg(id), sqlc.narg(actor_user_id), sqlc.arg(action),
  sqlc.arg(subject_type), sqlc.arg(subject_id), sqlc.narg(request_id),
  sqlc.narg(ip_hash), sqlc.arg(data), sqlc.arg(occurred_at)
)
RETURNING *;

-- name: ListAuditEvents :many
SELECT *
FROM app.audit_events
WHERE (sqlc.narg(subject_type)::text IS NULL OR subject_type = sqlc.narg(subject_type))
  AND (sqlc.narg(subject_id)::text IS NULL OR subject_id = sqlc.narg(subject_id))
  AND (sqlc.narg(actor_user_id)::text IS NULL OR actor_user_id = sqlc.narg(actor_user_id))
  AND (sqlc.narg(after_id)::text IS NULL OR (occurred_at, id) < (sqlc.narg(after_at)::timestamptz, sqlc.narg(after_id)::text))
ORDER BY occurred_at DESC, id DESC
LIMIT sqlc.arg(page_limit);
