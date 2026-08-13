-- name: ListSeasons :many
SELECT id, slug, name, status, revision, created_at
FROM app.seasons
WHERE ($1::text IS NULL OR id > $1)
ORDER BY id ASC
LIMIT $2;

-- name: UpdateSeasonRevision :execrows
UPDATE app.seasons SET name = $2, slug = $3, revision = revision + 1
WHERE id = $1 AND revision = $4;

