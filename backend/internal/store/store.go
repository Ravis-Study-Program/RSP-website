package store

import "github.com/magedmg/RSP-website/backend/internal/platform/repository"

var (
	// ErrNotFound is a public value used by the backend.
	ErrNotFound = repository.ErrNotFound
	// ErrConflict is a public value used by the backend.
	ErrConflict = repository.ErrConflict
	// ErrDuplicate is a public value used by the backend.
	ErrDuplicate = repository.ErrDuplicate
)
