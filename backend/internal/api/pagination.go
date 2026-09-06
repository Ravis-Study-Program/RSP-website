package api

import (
	"errors"
	"strconv"

	"github.com/magedmg/RSP-website/backend/internal/platform/cursor"
)

// PageInfo is the cursor metadata returned by collection endpoints.
type PageInfo struct {
	NextCursor     *string `json:"nextCursor"`
	PreviousCursor *string `json:"previousCursor"`
	HasMore        bool    `json:"hasMore"`
}

// Page is the common HTTP collection response.
type Page[T any] struct {
	Items      []T      `json:"items"`
	PageInfo   PageInfo `json:"pageInfo"`
	TotalCount int64    `json:"totalCount"`
}

func parseSort(value, fallback string, allowed ...string) (string, error) {
	if value == "" {
		value = fallback
	}
	for _, candidate := range allowed {
		if value == candidate {
			return value, nil
		}
	}
	return "", errors.New("unsupported sort")
}

func parsePageDirection(direction string) (string, error) {
	if direction == "" {
		return "forward", nil
	}
	if direction != "forward" && direction != "backward" {
		return "", errors.New("unsupported direction")
	}
	return direction, nil
}

func paginateOrdered[T any](secret []byte, binding, direction, boundary string, limit int, items []T, identity func(T) string) ([]T, PageInfo, error) {
	boundaryIndex := -1
	if boundary != "" {
		for i := range items {
			if identity(items[i]) == boundary {
				boundaryIndex = i
				break
			}
		}
		if boundaryIndex < 0 {
			return nil, PageInfo{}, cursor.ErrInvalid
		}
	}

	start, end := 0, len(items)
	if direction == "forward" {
		if boundaryIndex >= 0 {
			start = boundaryIndex + 1
		}
		end = min(start+limit, len(items))
	} else {
		if boundaryIndex >= 0 {
			end = boundaryIndex
		}
		start = max(0, end-limit)
	}

	page := items[start:end]
	info := PageInfo{HasMore: (direction == "forward" && end < len(items)) || (direction == "backward" && start > 0)}
	if len(page) > 0 && end < len(items) {
		encoded, encodeErr := cursor.Encode(secret, identity(page[len(page)-1]), binding)
		if encodeErr != nil {
			return nil, PageInfo{}, encodeErr
		}
		info.NextCursor = &encoded
	}
	if len(page) > 0 && start > 0 {
		encoded, encodeErr := cursor.Encode(secret, identity(page[0]), binding)
		if encodeErr != nil {
			return nil, PageInfo{}, encodeErr
		}
		info.PreviousCursor = &encoded
	}
	return page, info, nil
}

func pageInfoForKeyset[T any](secret []byte, binding, direction, boundary string, items []T, more bool, identity func(T) string) PageInfo {
	info := PageInfo{HasMore: more}
	if len(items) == 0 {
		return info
	}

	first, last := identity(items[0]), identity(items[len(items)-1])
	if direction == "backward" {
		if more {
			encoded, _ := cursor.Encode(secret, first, binding)
			info.PreviousCursor = &encoded
		}
		if boundary != "" {
			encoded, _ := cursor.Encode(secret, last, binding)
			info.NextCursor = &encoded
		}
		return info
	}

	if boundary != "" {
		encoded, _ := cursor.Encode(secret, first, binding)
		info.PreviousCursor = &encoded
	}
	if more {
		encoded, _ := cursor.Encode(secret, last, binding)
		info.NextCursor = &encoded
	}
	return info
}

func parsePagination(rawLimit, encodedCursor, binding string, secret []byte) (int, string, error) {
	limit := 25
	if raw := rawLimit; raw != "" {
		parsedLimit, err := strconv.Atoi(raw)
		if err != nil || parsedLimit < 1 || parsedLimit > 100 {
			return 0, "", cursor.ErrInvalid
		}

		limit = parsedLimit
	}
	encoded := encodedCursor
	if encoded == "" {
		return limit, "", nil
	}
	after, err := cursor.Decode(secret, encoded, binding)
	return limit, after, err
}
