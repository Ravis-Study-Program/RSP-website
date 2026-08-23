// Package programme models programme lifecycle rules.
package programme

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

// Enrollment is the part of an enrollment that season transitions change.
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

// Transition is the result of a pure season state change.
type Transition struct {
	Season     Season
	CloseEvent *CloseEvent
}

// Close closes a season without changing the supplied value.
func Close(s Season, actorID, reason string, now time.Time) (Transition, error) {
	if s.Status != "open" {
		return Transition{}, ErrClosed
	}
	e := CloseEvent{"close-" + s.ID, actorID, reason, now.UTC()}
	closed := cloneSeason(s)
	for i := range closed.Enrollments {
		if closed.Enrollments[i].State == "active" {
			closed.Enrollments[i].State = "completed"
			closeID := e.ID
			closed.Enrollments[i].CompletedByCloseID = &closeID
		}
	}
	closed.Status = "closed"
	closed.Revision++
	return Transition{Season: closed, CloseEvent: &e}, nil
}

// Reopen reopens a season without changing the supplied value.
func Reopen(s Season, e CloseEvent, role GlobalRole) (Season, error) {
	if role != Director && role != SystemAdmin {
		return Season{}, ErrForbidden
	}
	reopened := cloneSeason(s)
	for i := range reopened.Enrollments {
		if reopened.Enrollments[i].CompletedByCloseID != nil && *reopened.Enrollments[i].CompletedByCloseID == e.ID {
			reopened.Enrollments[i].State = "active"
			reopened.Enrollments[i].CompletedByCloseID = nil
		}
	}
	reopened.Status = "open"
	reopened.Revision++
	return reopened, nil
}

func cloneSeason(s Season) Season {
	s.Enrollments = append([]Enrollment(nil), s.Enrollments...)
	return s
}
