package seasons

import (
	"errors"
	"time"
)

var (
	ErrClosed    = errors.New("season is closed")
	ErrForbidden = errors.New("forbidden")
)

type GlobalRole string

const (
	Director    GlobalRole = "director"
	SystemAdmin GlobalRole = "system_admin"
)

type Enrollment struct {
	ID, State          string
	CompletedByCloseID *string
}
type Season struct {
	ID, Status  string
	Revision    int64
	Enrollments []Enrollment
}
type CloseEvent struct {
	ID, ActorID, Reason string
	ClosedAt            time.Time
}

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
