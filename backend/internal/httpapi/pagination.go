package httpapi

import (
	"errors"
	"net/http"

	"github.com/magedmg/RSP-website/backend/internal/model"
	"github.com/magedmg/RSP-website/backend/internal/platform/cursor"
)

func requestedSort(r *http.Request, fallback string, allowed ...string) (string, error) {
	value := r.URL.Query().Get("sort")
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

func requestedDirection(r *http.Request) (string, error) {
	direction := r.URL.Query().Get("direction")
	if direction == "" {
		return "forward", nil
	}
	if direction != "forward" && direction != "backward" {
		return "", errors.New("unsupported direction")
	}
	return direction, nil
}

func paginateOrdered[T any](a *API, r *http.Request, binding string, items []T, identity func(T) string) ([]T, model.PageInfo, error) {
	direction, directionErr := requestedDirection(r)
	if directionErr != nil {
		return nil, model.PageInfo{}, cursor.ErrInvalid
	}
	limit, boundary, err := a.page(r, binding)
	if err != nil {
		return nil, model.PageInfo{}, err
	}

	boundaryIndex := -1
	if boundary != "" {
		for i := range items {
			if identity(items[i]) == boundary {
				boundaryIndex = i
				break
			}
		}
		if boundaryIndex < 0 {
			return nil, model.PageInfo{}, cursor.ErrInvalid
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
	info := model.PageInfo{HasMore: (direction == "forward" && end < len(items)) || (direction == "backward" && start > 0)}
	if len(page) > 0 && end < len(items) {
		encoded, encodeErr := cursor.Encode(a.cursorSecret, identity(page[len(page)-1]), binding)
		if encodeErr != nil {
			return nil, model.PageInfo{}, encodeErr
		}
		info.NextCursor = &encoded
	}
	if len(page) > 0 && start > 0 {
		encoded, encodeErr := cursor.Encode(a.cursorSecret, identity(page[0]), binding)
		if encodeErr != nil {
			return nil, model.PageInfo{}, encodeErr
		}
		info.PreviousCursor = &encoded
	}
	return page, info, nil
}

func pageInfoForKeyset[T any](a *API, binding, direction, boundary string, items []T, more bool, identity func(T) string) model.PageInfo {
	info := model.PageInfo{HasMore: more}
	if len(items) == 0 {
		return info
	}

	first, last := identity(items[0]), identity(items[len(items)-1])
	if direction == "backward" {
		if more {
			encoded, _ := cursor.Encode(a.cursorSecret, first, binding)
			info.PreviousCursor = &encoded
		}
		if boundary != "" {
			encoded, _ := cursor.Encode(a.cursorSecret, last, binding)
			info.NextCursor = &encoded
		}
		return info
	}

	if boundary != "" {
		encoded, _ := cursor.Encode(a.cursorSecret, first, binding)
		info.PreviousCursor = &encoded
	}
	if more {
		encoded, _ := cursor.Encode(a.cursorSecret, last, binding)
		info.NextCursor = &encoded
	}
	return info
}
