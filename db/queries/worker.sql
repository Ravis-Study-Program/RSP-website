-- name: AcquireLeetcodeSyncLock :one
SELECT pg_try_advisory_xact_lock(691230184719023041::bigint) AS acquired;

-- name: GetLastSuccessfulLeetcodeSync :one
SELECT *
FROM app.leetcode_sync_runs
WHERE succeeded
ORDER BY finished_at DESC, id DESC
LIMIT 1;

-- name: StartLeetcodeSync :one
INSERT INTO app.leetcode_sync_runs (
  id, trigger_kind, requested_by_user_id, started_at
) VALUES (
  sqlc.arg(id), sqlc.arg(trigger_kind), sqlc.narg(requested_by_user_id),
  sqlc.arg(started_at)
)
RETURNING *;

-- name: FinishLeetcodeSync :one
UPDATE app.leetcode_sync_runs
SET finished_at = sqlc.arg(finished_at),
    succeeded = sqlc.arg(succeeded),
    fetched_count = sqlc.arg(fetched_count),
    changed_count = sqlc.arg(changed_count),
    error_summary = sqlc.narg(error_summary)
WHERE id = sqlc.arg(id) AND finished_at IS NULL
RETURNING *;

-- name: UpsertLeetcodeProblem :one
INSERT INTO app.leetcode_problems (
  id, problem_id, leetcode_number, difficulty, is_premium
) VALUES (
  sqlc.arg(id), sqlc.arg(problem_id), sqlc.arg(leetcode_number),
  sqlc.arg(difficulty), sqlc.arg(is_premium)
)
ON CONFLICT (leetcode_number) DO UPDATE
SET difficulty = EXCLUDED.difficulty,
    is_premium = EXCLUDED.is_premium,
    deleted_at = NULL,
    revision = app.leetcode_problems.revision + 1
RETURNING *;
