package leetcode

import (
	"context"
	"errors"
	"net/url"
	"strings"

	"github.com/magedmg/RSP-website/backend/internal/platform/dbtable"
	"github.com/magedmg/RSP-website/backend/internal/platform/id"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// PostgresSink applies one catalogue item atomically, including its canonical
// problem record and exact category membership.
type PostgresSink struct{ DB *gorm.DB }

func (s PostgresSink) Upsert(ctx context.Context, problem Problem) error {
	if s.DB == nil {
		return errors.New("PostgreSQL sink is not configured")
	}
	return s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var stored struct {
			ProblemID         string
			LeetcodeProblemID string
		}
		if err := tx.Raw(`SELECT p.id AS problem_id,l.id AS leetcode_problem_id
			FROM app.leetcode_problems l JOIN app.problems p ON p.id=l.problem_id
			WHERE l.leetcode_number=? FOR UPDATE OF l,p`, problem.Number).Scan(&stored).Error; err != nil {
			return err
		}

		problemID, leetcodeID := stored.ProblemID, stored.LeetcodeProblemID
		link := problemURL(problem.Slug)
		if leetcodeID == "" {
			problemID, leetcodeID = id.New(), id.New()
			if err := tx.Table(dbtable.Problems).Create(map[string]any{
				"id": problemID, "title": problem.Title, "url": link, "revision": 1,
			}).Error; err != nil {
				return err
			}
			if err := tx.Table(dbtable.LeetcodeProblems).Create(map[string]any{
				"id": leetcodeID, "problem_id": problemID, "leetcode_number": problem.Number,
				"difficulty": problem.Difficulty, "is_premium": problem.Premium, "revision": 1,
			}).Error; err != nil {
				return err
			}
		} else {
			if err := tx.Table(dbtable.Problems).Where("id = ?", problemID).Updates(map[string]any{
				"title": problem.Title, "url": link, "deleted_at": nil,
				"revision": gorm.Expr("revision + 1"),
			}).Error; err != nil {
				return err
			}
			if err := tx.Table(dbtable.LeetcodeProblems).Where("id = ?", leetcodeID).Updates(map[string]any{
				"difficulty": problem.Difficulty, "is_premium": problem.Premium,
				"deleted_at": nil, "revision": gorm.Expr("revision + 1"),
			}).Error; err != nil {
				return err
			}
		}

		if err := tx.Table(dbtable.LeetcodeCategoryMappings).
			Where("leetcode_problem_id = ?", leetcodeID).Delete(&struct{}{}).Error; err != nil {
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
			if err := tx.Table(dbtable.LeetcodeCategories).Clauses(clause.OnConflict{DoNothing: true}).Create(map[string]any{
				"id": categoryID, "name": displayName, "normalized_name": normalized, "revision": 1,
			}).Error; err != nil {
				return err
			}
			if err := tx.Table(dbtable.LeetcodeCategories).Select("id").
				Where("lower(normalized_name) = lower(?) AND deleted_at IS NULL", normalized).
				Row().Scan(&categoryID); err != nil {
				return err
			}
			if err := tx.Table(dbtable.LeetcodeCategoryMappings).Clauses(clause.OnConflict{DoNothing: true}).Create(map[string]any{
				"leetcode_problem_id": leetcodeID, "category_id": categoryID,
			}).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func normalizeCategory(value string) string {
	return strings.Join(strings.Fields(strings.ToLower(value)), " ")
}

func problemURL(slug string) string {
	return "https://leetcode.com/problems/" + url.PathEscape(strings.TrimSpace(slug)) + "/"
}
