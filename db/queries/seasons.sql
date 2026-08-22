-- name: ListSeasons :many
SELECT id, slug, name, status, start_at, end_at, location, image_url,
       resources_url, closed_at, revision, created_at, updated_at
FROM app.seasons
WHERE deleted_at IS NULL
  AND (sqlc.narg(status)::app.season_status IS NULL OR status = sqlc.narg(status))
  AND (
    sqlc.narg(after_id)::uuid IS NULL
    OR (start_at, id) < (sqlc.narg(after_start_at)::timestamptz, sqlc.narg(after_id)::uuid)
  )
ORDER BY start_at DESC, id DESC
LIMIT sqlc.arg(page_limit);

-- name: GetSeasonByID :one
SELECT id, slug, name, status, start_at, end_at, location, image_url,
       resources_url, closed_at, revision, created_at, updated_at
FROM app.seasons
WHERE id = $1 AND deleted_at IS NULL;

-- name: GetSeasonBySlug :one
SELECT id, slug, name, status, start_at, end_at, location, image_url,
       resources_url, closed_at, revision, created_at, updated_at
FROM app.seasons
WHERE lower(slug) = lower($1) AND deleted_at IS NULL;

-- name: CreateSeason :one
INSERT INTO app.seasons (
  id, slug, name, status, start_at, end_at, location, image_url, resources_url
) VALUES (
  sqlc.arg(id), sqlc.arg(slug), sqlc.arg(name), 'open', sqlc.arg(start_at),
  sqlc.arg(end_at), sqlc.arg(location), sqlc.arg(image_url), sqlc.arg(resources_url)
)
RETURNING *;

-- name: UpdateSeason :one
UPDATE app.seasons
SET slug = sqlc.arg(slug),
    name = sqlc.arg(name),
    start_at = sqlc.arg(start_at),
    end_at = sqlc.arg(end_at),
    location = sqlc.arg(location),
    image_url = sqlc.arg(image_url),
    resources_url = sqlc.arg(resources_url),
    revision = revision + 1
WHERE id = sqlc.arg(id)
  AND revision = sqlc.arg(revision)
  AND deleted_at IS NULL
RETURNING *;

-- name: CreateSeasonCloseEvent :one
INSERT INTO app.season_close_events (
  id, season_id, closed_by_user_id, close_reason, closed_at, migration_run_id
) VALUES (
  sqlc.arg(id), sqlc.arg(season_id), sqlc.narg(actor_user_id),
  sqlc.arg(reason), sqlc.arg(closed_at), sqlc.narg(migration_run_id)
)
RETURNING *;

-- name: CompleteActiveEnrollmentsForClose :many
UPDATE app.enrollments
SET state = 'completed',
    completed_by_close_id = sqlc.arg(close_event_id),
    state_changed_at = sqlc.arg(closed_at),
    assignment_state = 'revoked',
    activated_at = NULL,
    revision = revision + 1
WHERE season_id = sqlc.arg(season_id)
  AND state = 'active'
  AND deleted_at IS NULL
RETURNING id;

-- name: MarkSeasonClosed :one
UPDATE app.seasons
SET status = 'closed',
    closed_at = sqlc.arg(closed_at),
    revision = revision + 1
WHERE id = sqlc.arg(id)
  AND status = 'open'
  AND revision = sqlc.arg(revision)
  AND deleted_at IS NULL
RETURNING *;

-- name: ReopenSeasonCloseEvent :one
UPDATE app.season_close_events
SET reopened_by_user_id = sqlc.arg(actor_user_id),
    reopen_reason = sqlc.arg(reason),
    reopened_at = sqlc.arg(reopened_at)
WHERE id = sqlc.arg(close_event_id)
  AND reopened_at IS NULL
RETURNING *;

-- name: RestoreEnrollmentsForReopen :many
UPDATE app.enrollments
SET state = 'active',
    completed_by_close_id = NULL,
    state_changed_at = sqlc.arg(reopened_at),
    assignment_state = CASE WHEN role='coordinator' THEN 'pending_mfa' ELSE 'active' END,
    activated_at = CASE WHEN role='coordinator' THEN NULL ELSE sqlc.arg(reopened_at) END,
    revision = revision + 1
WHERE completed_by_close_id = sqlc.arg(close_event_id)
  AND state = 'completed'
  AND deleted_at IS NULL
RETURNING id;

-- name: MarkSeasonOpen :one
UPDATE app.seasons
SET status = 'open', closed_at = NULL, revision = revision + 1
WHERE id = sqlc.arg(id)
  AND status = 'closed'
  AND revision = sqlc.arg(revision)
  AND deleted_at IS NULL
RETURNING *;

-- name: ListSeasonWeeks :many
SELECT *
FROM app.season_weeks
WHERE season_id = $1 AND deleted_at IS NULL
ORDER BY week_number ASC, id ASC;

-- name: UpsertSeasonWeek :one
INSERT INTO app.season_weeks (
  id, season_id, week_number, start_at, end_at
) VALUES (
  sqlc.arg(id), sqlc.arg(season_id), sqlc.arg(week_number),
  sqlc.arg(start_at), sqlc.arg(end_at)
)
ON CONFLICT (season_id, week_number) DO UPDATE
SET start_at = EXCLUDED.start_at,
    end_at = EXCLUDED.end_at,
    revision = app.season_weeks.revision + 1
WHERE app.season_weeks.revision = sqlc.arg(expected_revision)
RETURNING *;
