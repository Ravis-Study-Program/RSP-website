package dal

import "slices"

func finishStorePage[T any](items []T, limit int, direction string) ([]T, bool) {
	more := len(items) > limit
	if more {
		items = items[:limit]
	}
	if direction == "backward" {
		slices.Reverse(items)
	}
	return items, more
}
