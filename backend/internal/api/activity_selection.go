package api

import (
	"fmt"
	"net/url"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5/pgtype"
)

// Multi-value selections have a canonical order so cursors bind to the filter
// meaning rather than the order in which checkboxes were selected.
func selection(values url.Values, key string, validate func(string) bool) ([]string, error) {
	if len(values[key]) > 100 {
		return nil, fmt.Errorf("too many %s filters", key)
	}
	seen := map[string]bool{}
	for _, value := range values[key] {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if len(value) > 100 || validate != nil && !validate(value) {
			return nil, fmt.Errorf("invalid %s filter", key)
		}
		seen[value] = true
	}
	items := make([]string, 0, len(seen))
	for value := range seen {
		items = append(items, value)
	}
	sort.Strings(items)
	return items, nil
}

func uuidSelection(value string) bool { var id pgtype.UUID; return id.Scan(value) == nil }
func difficultySelection(value string) bool {
	return value == "easy" || value == "medium" || value == "hard"
}
