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
	ID, State            string
	AssignmentState      string
	CloseAssignmentState string
	CompletedByCloseID   *string
	Revision             int64
}

// Season represents a backend data structure.
type Season struct {
	ID, Status  string
	Revision    int64
	Enrollments []Enrollment
}

// SeasonRecord is the persisted and HTTP-facing season representation.
type SeasonRecord struct {
	ID           string    `json:"id"`
	Slug         string    `json:"slug"`
	Name         string    `json:"name"`
	Status       string    `json:"status"`
	StartAt      time.Time `json:"startAt"`
	EndAt        time.Time `json:"endAt"`
	Location     string    `json:"location"`
	ImageURL     string    `json:"imageUrl"`
	ResourcesURL string    `json:"resourcesUrl"`
	Revision     int64     `json:"revision"`
}

// MentorshipRecord assigns a mentor to a student within a season.
type MentorshipRecord struct {
	ID            string `json:"id"`
	SeasonID      string `json:"seasonId"`
	MentorUserID  string `json:"mentorUserId"`
	StudentUserID string `json:"studentUserId"`
	Revision      int64  `json:"revision"`
}

// WeekRecord is a scheduled week within a season.
type WeekRecord struct {
	ID          string    `json:"id"`
	SeasonID    string    `json:"seasonId"`
	Number      int       `json:"number"`
	StartAt     time.Time `json:"startAt"`
	EndAt       time.Time `json:"endAt"`
	ResourceURL string    `json:"resourceUrl"`
	Revision    int64     `json:"revision"`
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
func Close(s Season, actorID, reason, eventID string, now time.Time) (Transition, error) {
	if s.Status != "open" {
		return Transition{}, ErrClosed
	}
	if eventID == "" {
		eventID = "close-" + s.ID
	}
	e := CloseEvent{eventID, actorID, reason, now.UTC()}
	closed := cloneSeason(s)
	for i := range closed.Enrollments {
		if closed.Enrollments[i].State == "active" {
			closed.Enrollments[i].State = "completed"
			closed.Enrollments[i].CloseAssignmentState = closed.Enrollments[i].AssignmentState
			if closed.Enrollments[i].AssignmentState != "" {
				closed.Enrollments[i].AssignmentState = "revoked"
			}
			closeID := e.ID
			closed.Enrollments[i].CompletedByCloseID = &closeID
			closed.Enrollments[i].Revision++
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
			if reopened.Enrollments[i].CloseAssignmentState != "" {
				reopened.Enrollments[i].AssignmentState = reopened.Enrollments[i].CloseAssignmentState
			}
			reopened.Enrollments[i].CloseAssignmentState = ""
			reopened.Enrollments[i].CompletedByCloseID = nil
			reopened.Enrollments[i].Revision++
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
