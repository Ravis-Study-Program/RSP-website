// Package seasons models season lifecycle rules.
package seasons

import (
	"errors"
	"time"
)

var (
	// ErrClosed is a public value used by the backend.
	ErrClosed = errors.New("season is closed")
	// ErrForbidden is a public value used by the backend.
	ErrForbidden = errors.New("forbidden")
)

// GlobalRole is a backend domain type.
type GlobalRole string

const (
	// Director is a public value used by the backend.
	Director GlobalRole = "director"
	// SystemAdmin is a public value used by the backend.
	SystemAdmin GlobalRole = "system_admin"
)

// Enrollment represents a backend data structure.
type Enrollment struct {
	ID, State          string
	CompletedByCloseID *string
}

// Season represents a backend data structure.
type Season struct {
	ID, Status  string
	Revision    int64
	Enrollments []Enrollment
}

// CloseEvent represents a backend data structure.
type CloseEvent struct {
	ID, ActorID, Reason string
	ClosedAt            time.Time
}

// Close closes a value.
func Close(s *Season, actorID, reason string, now time.Time) (CloseEvent, error) {
	if s.Status != "open" {
		return CloseEvent{}, ErrClosed
	}
	e := CloseEvent{"close-" + s.ID, actorID, reason, now.UTC()}
	for i := range s.Enrollments {
		if s.Enrollments[i].State == "active" {
			s.Enrollments[i].State = "completed"
			s.Enrollments[i].CompletedByCloseID = &e.ID
		}
	}
	s.Status = "closed"
	s.Revision++
	return e, nil
}

// Reopen reopens a value.
func Reopen(s *Season, e CloseEvent, role GlobalRole) error {
	if role != Director && role != SystemAdmin {
		return ErrForbidden
	}
	for i := range s.Enrollments {
		if s.Enrollments[i].CompletedByCloseID != nil && *s.Enrollments[i].CompletedByCloseID == e.ID {
			s.Enrollments[i].State = "active"
			s.Enrollments[i].CompletedByCloseID = nil
		}
	}
	s.Status = "open"
	s.Revision++
	return nil
}
