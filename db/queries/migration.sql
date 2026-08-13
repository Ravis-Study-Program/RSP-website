-- name: CreateMigrationRun :one
INSERT INTO migration.runs (
  id, manifest_checksum, source_schema_fingerprint, source_snapshot_at, state
) VALUES (
  sqlc.arg(id), sqlc.arg(manifest_checksum), sqlc.arg(source_schema_fingerprint),
  sqlc.arg(source_snapshot_at), 'planned'
)
RETURNING *;

-- name: AcquireLegacyMigrationLock :one
SELECT pg_advisory_xact_lock(593215744214775351::bigint);

-- name: MarkMigrationApplying :one
UPDATE migration.runs
SET state = 'applying', started_at = sqlc.arg(started_at)
WHERE id = sqlc.arg(id) AND state = 'planned'
RETURNING *;

-- name: MarkMigrationApplied :one
UPDATE migration.runs
SET state = 'applied', finished_at = sqlc.arg(finished_at), error_summary = NULL
WHERE id = sqlc.arg(id) AND state = 'applying'
RETURNING *;

-- name: MarkMigrationVerified :one
UPDATE migration.runs
SET state = 'verified', finished_at = sqlc.arg(finished_at)
WHERE id = sqlc.arg(id) AND state = 'applied'
RETURNING *;

-- name: MarkMigrationRolledBack :one
UPDATE migration.runs
SET state = 'rolled_back', rolled_back_at = sqlc.arg(rolled_back_at)
WHERE id = sqlc.arg(id) AND state IN ('applied', 'verified')
RETURNING *;

-- name: RecordSourceTable :exec
INSERT INTO migration.source_tables (
  run_id, source_table, source_count, imported_count,
  source_ids_checksum, transformed_checksum
) VALUES (
  sqlc.arg(run_id), sqlc.arg(source_table), sqlc.arg(source_count),
  sqlc.arg(imported_count), sqlc.arg(source_ids_checksum),
  sqlc.arg(transformed_checksum)
);

-- name: RecordRowProvenance :exec
INSERT INTO migration.row_provenance (
  run_id, source_table, source_id, target_table, target_id,
  source_checksum, transformed_checksum, transformed_data, duplicate_group
) VALUES (
  sqlc.arg(run_id), sqlc.arg(source_table), sqlc.arg(source_id),
  sqlc.arg(target_table), sqlc.arg(target_id), sqlc.arg(source_checksum),
  sqlc.arg(transformed_checksum), sqlc.arg(transformed_data),
  sqlc.narg(duplicate_group)
);

-- name: GetMigrationRun :one
SELECT * FROM migration.runs WHERE id = $1;

-- name: ListMigrationTableResults :many
SELECT * FROM migration.source_tables WHERE run_id = $1 ORDER BY source_table;
