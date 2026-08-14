package leetcode

import (
	"context"
	"errors"
	"net/url"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/magedmg/RSP-website/backend/internal/platform/id"
)

type beginner interface {
	Begin(context.Context) (pgx.Tx, error)
}

// PostgresSink applies one catalogue item atomically, including its canonical
// problem record and exact category membership. The worker's advisory lock
// serialises full catalogue runs; the database constraints protect individual
// rows if an operator retries a run.
type PostgresSink struct{ DB beginner }

func (s PostgresSink) Upsert(ctx context.Context, problem Problem) error {
	if s.DB == nil {
		return errors.New("PostgreSQL sink is not configured")
	}
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var problemID, leetcodeID string
	err = tx.QueryRow(ctx, `
SELECT p.id, l.id
FROM app.leetcode_problems AS l
JOIN app.problems AS p ON p.id = l.problem_id
WHERE l.leetcode_number = $1
FOR UPDATE OF p, l`, problem.Number).Scan(&problemID, &leetcodeID)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		problemID, leetcodeID = id.New(), id.New()
		if _, err = tx.Exec(ctx, `INSERT INTO app.problems(id,title,url,revision) VALUES($1,$2,$3,1)`, problemID, problem.Title, problemURL(problem.Slug)); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `INSERT INTO app.leetcode_problems(id,problem_id,leetcode_number,difficulty,is_premium,revision) VALUES($1,$2,$3,$4,$5,1)`, leetcodeID, problemID, problem.Number, problem.Difficulty, problem.Premium); err != nil {
			return err
		}
	case err != nil:
		return err
	default:
		if _, err = tx.Exec(ctx, `UPDATE app.problems SET title=$2,url=$3,deleted_at=NULL,revision=revision+1 WHERE id=$1`, problemID, problem.Title, problemURL(problem.Slug)); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `UPDATE app.leetcode_problems SET difficulty=$2,is_premium=$3,deleted_at=NULL,revision=revision+1 WHERE id=$1`, leetcodeID, problem.Difficulty, problem.Premium); err != nil {
			return err
		}
	}

	if _, err = tx.Exec(ctx, `DELETE FROM app.leetcode_problem_category_mappings WHERE leetcode_problem_id=$1`, leetcodeID); err != nil {
		return err
	}
	seen := map[string]bool{}
	for _, displayName := range problem.Categories {
		displayName = strings.TrimSpace(displayName)
		normalized := normalizeCategory(displayName)
		if normalized == "" || seen[normalized] {
			continue
		}
		seen[normalized] = true
		categoryID := id.New()
		if _, err = tx.Exec(ctx, `INSERT INTO app.leetcode_problem_categories(id,name,normalized_name,revision) VALUES($1,$2,$3,1) ON CONFLICT DO NOTHING`, categoryID, displayName, normalized); err != nil {
			return err
		}
		if err = tx.QueryRow(ctx, `SELECT id FROM app.leetcode_problem_categories WHERE lower(normalized_name)=lower($1) AND deleted_at IS NULL`, normalized).Scan(&categoryID); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `INSERT INTO app.leetcode_problem_category_mappings(leetcode_problem_id,category_id) VALUES($1,$2) ON CONFLICT DO NOTHING`, leetcodeID, categoryID); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func normalizeCategory(value string) string {
	return strings.Join(strings.Fields(strings.ToLower(value)), " ")
}

func problemURL(slug string) string {
	return "https://leetcode.com/problems/" + url.PathEscape(strings.TrimSpace(slug)) + "/"
}
