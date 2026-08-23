package store

import (
	"encoding/json"
	"time"

	"github.com/magedmg/RSP-website/backend/internal/platform/repository"
)

var (
	// ErrNotFound is a public value used by the backend.
	ErrNotFound = repository.ErrNotFound
	// ErrConflict is a public value used by the backend.
	ErrConflict = repository.ErrConflict
	// ErrDuplicate is a public value used by the backend.
	ErrDuplicate = repository.ErrDuplicate
)

// MockVersion is a persistence record for an interview snapshot.
type MockVersion struct {
	InterviewID     string
	Revision        int64
	ActorID, Reason string
	SavedAt         time.Time
	Snapshot        json.RawMessage
}
