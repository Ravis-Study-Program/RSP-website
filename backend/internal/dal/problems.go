package dal

import (
	"context"
	"encoding/json"

	"github.com/jackc/pgx/v5"

	"github.com/magedmg/RSP-website/backend/internal/practice"
)

func scanProblem(row pgx.Row) (practice.ProblemRecord, error) {
	var v practice.ProblemRecord
	var categories []byte
	if err := row.Scan(&v.ID, &v.Number, &v.Title, &v.Link, &v.Difficulty, &v.Premium, &categories); err != nil {
		return v, noRows(err)
	}
	if err := json.Unmarshal(categories, &v.Categories); err != nil {
		return v, err
	}
	return v, nil
}

const problemQuery = `SELECT l.id,l.leetcode_number,p.title,COALESCE(p.url,''),l.difficulty::text,l.is_premium,
	COALESCE(to_jsonb(array_agg(c.normalized_name) FILTER (WHERE c.id IS NOT NULL)),'[]'::jsonb)
	FROM app.leetcode_problems l
	JOIN app.problems p ON p.id=l.problem_id
	LEFT JOIN app.leetcode_problem_category_mappings m ON m.leetcode_problem_id=l.id
	LEFT JOIN app.leetcode_problem_categories c ON c.id=m.category_id
	WHERE l.deleted_at IS NULL AND p.deleted_at IS NULL`

type ProblemQuery struct {
	Boundary   string
	Limit      int
	Difficulty string
	Category   string
	Premium    *bool
	Direction  string
}

func (p *Store) ListProblems(ctx context.Context, q ProblemQuery) ([]practice.ProblemRecord, bool, int64, error) {
	const filters = `(@difficulty = '' OR l.difficulty::text = @difficulty)
		AND (@category = '' OR EXISTS (
			SELECT 1 FROM app.leetcode_problem_category_mappings fm
			JOIN app.leetcode_problem_categories fc ON fc.id = fm.category_id
			WHERE fm.leetcode_problem_id = l.id AND fc.deleted_at IS NULL
			AND lower(fc.normalized_name) = lower(@category)
		))
		AND (CAST(@premium AS boolean) IS NULL OR l.is_premium = @premium)`
	args := pgx.NamedArgs{
		"difficulty": q.Difficulty,
		"category":   q.Category,
		"premium":    q.Premium,
	}
	var total int64
	err := p.pool.QueryRow(ctx, `SELECT count(*) FROM app.leetcode_problems l
		JOIN app.problems p ON p.id = l.problem_id
		WHERE l.deleted_at IS NULL AND p.deleted_at IS NULL AND `+filters, args).Scan(&total)
	if err != nil {
		return nil, false, 0, err
	}

	comparison, order := `l.id > NULLIF(@boundary, '')::uuid`, `ASC`
	if q.Direction == "backward" {
		comparison, order = `l.id < NULLIF(@boundary, '')::uuid`, `DESC`
	}
	if q.Boundary == "" {
		comparison = `TRUE`
	}
	args["boundary"] = q.Boundary
	args["limit"] = q.Limit + 1
	rows, err := p.pool.Query(ctx, problemQuery+`
		AND `+comparison+` AND `+filters+`
		GROUP BY l.id,p.id
		ORDER BY l.id `+order+` LIMIT @limit`, args)
	if err != nil {
		return nil, false, 0, err
	}

	defer rows.Close()
	out := []practice.ProblemRecord{}
	for rows.Next() {
		v, err := scanProblem(rows)
		if err != nil {
			return nil, false, 0, err
		}

		out = append(out, v)
	}
	out, more := finishPage(out, q.Limit, q.Direction)
	return out, more, total, rows.Err()
}
