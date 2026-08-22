-- name: ListLeetcodeProblems :many
SELECT lp.*, p.title, p.url,
       COALESCE(array_agg(c.normalized_name ORDER BY c.normalized_name)
         FILTER (WHERE c.id IS NOT NULL), ARRAY[]::text[]) AS categories
FROM app.leetcode_problems AS lp
JOIN app.problems AS p ON p.id = lp.problem_id
LEFT JOIN app.leetcode_problem_category_mappings AS pcm ON pcm.leetcode_problem_id = lp.id
LEFT JOIN app.leetcode_problem_categories AS c ON c.id = pcm.category_id AND c.deleted_at IS NULL
WHERE lp.deleted_at IS NULL
  AND p.deleted_at IS NULL
  AND (sqlc.narg(difficulty)::app.problem_difficulty IS NULL OR lp.difficulty = sqlc.narg(difficulty))
  AND (sqlc.narg(is_premium)::boolean IS NULL OR lp.is_premium = sqlc.narg(is_premium))
  AND (sqlc.narg(after_id)::uuid IS NULL OR (lp.leetcode_number, lp.id) > (sqlc.narg(after_number)::integer, sqlc.narg(after_id)::uuid))
GROUP BY lp.id, p.id
ORDER BY lp.leetcode_number ASC, lp.id ASC
LIMIT sqlc.arg(page_limit);

-- name: CreateProblemAttempt :one
INSERT INTO app.problem_attempts (
  id, user_id, problem_id, enrollment_id, season_week_id, attempted_at,
  time_taken_minutes, outcome, confidence, notes_html
) VALUES (
  sqlc.arg(id), sqlc.arg(user_id), sqlc.arg(problem_id),
  sqlc.narg(enrollment_id), sqlc.narg(season_week_id), sqlc.arg(attempted_at),
  sqlc.arg(time_taken_minutes), sqlc.arg(outcome), sqlc.narg(confidence),
  sqlc.narg(notes_html)
)
RETURNING *;

-- name: UpdateProblemAttempt :one
UPDATE app.problem_attempts
SET attempted_at = sqlc.arg(attempted_at),
    time_taken_minutes = sqlc.arg(time_taken_minutes),
    outcome = sqlc.arg(outcome),
    confidence = sqlc.narg(confidence),
    notes_html = sqlc.narg(notes_html),
    revision = revision + 1
WHERE id = sqlc.arg(id)
  AND user_id = sqlc.arg(user_id)
  AND revision = sqlc.arg(revision)
  AND deleted_at IS NULL
RETURNING *;

-- name: SoftDeleteProblemAttempt :one
UPDATE app.problem_attempts
SET deleted_at = sqlc.arg(deleted_at), revision = revision + 1
WHERE id = sqlc.arg(id)
  AND user_id = sqlc.arg(user_id)
  AND revision = sqlc.arg(revision)
  AND deleted_at IS NULL
RETURNING revision;

-- name: ListProblemAttemptsForUser :many
SELECT pa.*, p.title, p.url, lp.leetcode_number, lp.difficulty, lp.is_premium
FROM app.problem_attempts AS pa
JOIN app.problems AS p ON p.id = pa.problem_id
LEFT JOIN app.leetcode_problems AS lp ON lp.problem_id = p.id
WHERE pa.user_id = sqlc.arg(user_id)
  AND pa.deleted_at IS NULL
  AND (sqlc.narg(outcome)::app.attempt_outcome IS NULL OR pa.outcome = sqlc.narg(outcome))
  AND (sqlc.narg(after_id)::uuid IS NULL OR (pa.attempted_at, pa.id) < (sqlc.narg(after_at)::timestamptz, sqlc.narg(after_id)::uuid))
ORDER BY pa.attempted_at DESC, pa.id DESC
LIMIT sqlc.arg(page_limit);

-- name: GetActiveRecommendation :one
SELECT r.*, lp.problem_id, lp.leetcode_number, lp.is_premium,
       p.title, p.url, c.name AS category_name
FROM app.recommendations AS r
JOIN app.leetcode_problems AS lp ON lp.id = r.leetcode_problem_id
JOIN app.problems AS p ON p.id = lp.problem_id
LEFT JOIN app.leetcode_problem_categories AS c ON c.id = r.category_id
WHERE r.user_id = $1 AND r.state = 'active' AND r.deleted_at IS NULL;

-- name: CreateRecommendation :one
INSERT INTO app.recommendations (
  id, user_id, leetcode_problem_id, category_id, difficulty,
  rationale, rule_version, generated_at
) VALUES (
  sqlc.arg(id), sqlc.arg(user_id), sqlc.arg(leetcode_problem_id),
  sqlc.narg(category_id), sqlc.arg(difficulty), sqlc.arg(rationale),
  sqlc.arg(rule_version), sqlc.arg(generated_at)
)
RETURNING *;

-- name: DismissRecommendation :one
UPDATE app.recommendations
SET state = 'dismissed', state_changed_at = sqlc.arg(dismissed_at), revision = revision + 1
WHERE id = sqlc.arg(id)
  AND user_id = sqlc.arg(user_id)
  AND state = 'active'
  AND revision = sqlc.arg(revision)
RETURNING *;

-- name: RecordRecommendationDismissal :one
INSERT INTO app.recommendation_dismissals (
  id, recommendation_id, user_id, leetcode_problem_id, reason,
  dismissed_at, excluded_until
) VALUES (
  sqlc.arg(id), sqlc.arg(recommendation_id), sqlc.arg(user_id),
  sqlc.arg(leetcode_problem_id), sqlc.narg(reason), sqlc.arg(dismissed_at),
  sqlc.arg(excluded_until)
)
RETURNING *;
