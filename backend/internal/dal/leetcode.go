package dal

import (
	"context"
	"errors"
	"net/url"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/magedmg/RSP-website/backend/internal/leetcode"
	"github.com/magedmg/RSP-website/backend/internal/platform/id"
)

// Upsert keeps each catalogue item and its category membership in one transaction.
func (s *WorkerSession) Upsert(ctx context.Context, problem leetcode.Problem) error {
	tx, err := s.conn.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())
	var problemID, leetcodeID string
	err = tx.QueryRow(ctx, `SELECT p.id,l.id FROM app.leetcode_problems l JOIN app.problems p ON p.id=l.problem_id
  WHERE l.leetcode_number=$1 FOR UPDATE OF l,p`, problem.Number).Scan(&problemID, &leetcodeID)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	link := problemURL(problem.Slug)
	if leetcodeID == "" {
		problemID, leetcodeID = id.New(), id.New()
		if _, err := tx.Exec(ctx, `INSERT INTO app.problems(id,title,url) VALUES($1,$2,$3)`, problemID, problem.Title, link); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO app.leetcode_problems(id,problem_id,leetcode_number,difficulty,is_premium) VALUES($1,$2,$3,$4,$5)`, leetcodeID, problemID, problem.Number, problem.Difficulty, problem.Premium); err != nil {
			return err
		}
	} else {
		if _, err := tx.Exec(ctx, `UPDATE app.problems SET title=$2,url=$3,deleted_at=NULL WHERE id=$1`, problemID, problem.Title, link); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE app.leetcode_problems SET difficulty=$2,is_premium=$3,deleted_at=NULL WHERE id=$1`, leetcodeID, problem.Difficulty, problem.Premium); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(ctx, `DELETE FROM app.leetcode_problem_category_mappings WHERE leetcode_problem_id=$1`, leetcodeID); err != nil {
		return err
	}
	seen := map[string]bool{}
	for _, displayName := range problem.Categories {
		displayName = strings.TrimSpace(displayName)
		normalized := strings.Join(strings.Fields(strings.ToLower(displayName)), " ")
		if normalized == "" || seen[normalized] {
			continue
		}
		seen[normalized] = true
		categoryID := id.New()
		if _, err := tx.Exec(ctx, `INSERT INTO app.leetcode_problem_categories(id,name,normalized_name) VALUES($1,$2,$3) ON CONFLICT DO NOTHING`, categoryID, displayName, normalized); err != nil {
			return err
		}
		if err := tx.QueryRow(ctx, `SELECT id FROM app.leetcode_problem_categories WHERE lower(normalized_name)=lower($1) AND deleted_at IS NULL`, normalized).Scan(&categoryID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO app.leetcode_problem_category_mappings(leetcode_problem_id,category_id) VALUES($1,$2) ON CONFLICT DO NOTHING`, leetcodeID, categoryID); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func problemURL(slug string) string {
	return "https://leetcode.com/problems/" + url.PathEscape(strings.TrimSpace(slug)) + "/"
}
