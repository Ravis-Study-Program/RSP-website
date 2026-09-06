package dal

import (
	"context"
	"github.com/jackc/pgx/v5"
	"github.com/magedmg/RSP-website/backend/internal/practice"
)

// SuggestProblem excludes the member's complete practice history, regardless of
// the season or year currently being viewed.
func (p *Store) SuggestProblem(ctx context.Context, userID string, difficulties, categories []string) (practice.ProblemRecord, error) {
	return scanProblem(p.pool.QueryRow(ctx, problemQuery+`
 AND (COALESCE(cardinality(@difficulties::text[]),0)=0 OR l.difficulty::text=ANY(@difficulties::text[]))
 AND (COALESCE(cardinality(@categories::text[]),0)=0 OR EXISTS (
  SELECT 1 FROM app.leetcode_problem_category_mappings mapping
  JOIN app.leetcode_problem_categories category ON category.id=mapping.category_id
  WHERE mapping.leetcode_problem_id=l.id AND category.deleted_at IS NULL
   AND lower(category.normalized_name) IN (SELECT lower(unnest(@categories::text[])))))
 AND NOT EXISTS (SELECT 1 FROM app.problem_attempts attempt
  WHERE attempt.user_id=@userID AND attempt.problem_id=p.id AND attempt.deleted_at IS NULL)
 GROUP BY l.id,p.id ORDER BY random() LIMIT 1`, pgx.NamedArgs{"userID": userID, "difficulties": difficulties, "categories": categories}))
}
