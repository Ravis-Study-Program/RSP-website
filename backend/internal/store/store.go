package store

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/magedmg/RSP-website/backend/internal/accounts"
	"github.com/magedmg/RSP-website/backend/internal/authz"
	"github.com/magedmg/RSP-website/backend/internal/mockinterviews"
	"github.com/magedmg/RSP-website/backend/internal/platform/audit"
	"github.com/magedmg/RSP-website/backend/internal/practice"
	"github.com/magedmg/RSP-website/backend/internal/programme"
)

var (
	// ErrNotFound is a public value used by the backend.
	ErrNotFound = errors.New("not found")
	// ErrConflict is a public value used by the backend.
	ErrConflict = errors.New("conflict")
	// ErrDuplicate is a public value used by the backend.
	ErrDuplicate = errors.New("duplicate")
)

// MockVersion is a persistence record for an interview snapshot.
type MockVersion struct {
	InterviewID     string
	Revision        int64
	ActorID, Reason string
	SavedAt         time.Time
	Snapshot        json.RawMessage
}

// Repository defines a backend interface.
type Repository interface {
	accounts.Repository
	practice.Repository
	programme.Repository
	ListMockInterviews(context.Context, authz.Actor, string, string, int, string, string) ([]mockinterviews.Interview, bool, int64, error)
	GetMockParticipant(context.Context, string) (mockinterviews.Participant, error)
	GetMockInterview(context.Context, string) (mockinterviews.Interview, error)
	CreateMockInterview(context.Context, mockinterviews.Interview, string, time.Time) (mockinterviews.Interview, error)
	UpdateMockInterview(context.Context, mockinterviews.Interview, string, string, time.Time) (mockinterviews.Interview, error)
	AppendAudit(context.Context, audit.Event) error
}
