package httpapi

import (
	"context"
	"time"

	"github.com/magedmg/RSP-website/backend/internal/accounts"
	"github.com/magedmg/RSP-website/backend/internal/authz"
	"github.com/magedmg/RSP-website/backend/internal/mockinterviews"
	"github.com/magedmg/RSP-website/backend/internal/platform/audit"
	"github.com/magedmg/RSP-website/backend/internal/practice"
	"github.com/magedmg/RSP-website/backend/internal/programme"
)

// Repository is the HTTP adapter's composed feature port.
// Concrete storage packages do not appear in the HTTP contract.
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
