package accounts

import (
	"context"
	"time"

	"github.com/magedmg/RSP-website/backend/internal/authz"
)

// Repository defines the persistence port required by account features.
type Repository interface {
	ApplyIdentityEvent(context.Context, IdentityEvent) error
	ResolveAuthSubject(context.Context, string) (authz.Actor, error)
	GetUser(context.Context, string) (User, error)
	SuggestUserSlug(context.Context) (string, error)
	ListUsers(context.Context, string, int, string, string, string, string) ([]User, bool, int64, error)
	ListAdminUsers(context.Context, string, int, string, string, string, string) ([]User, bool, int64, error)
	ListEnrollmentCandidates(context.Context, string, string, string, int, string) ([]EnrollmentCandidate, bool, int64, error)
	UpdateUser(context.Context, string, int64, func(*User) error, string, time.Time) (User, error)
	ResolveAuthSubjectForUser(context.Context, string) (string, error)
	GrantGlobalRole(context.Context, string, string, bool, string, string, time.Time) (GlobalRoleAssignment, error)
	ListGlobalRoles(context.Context, string) ([]GlobalRoleAssignment, error)
	RevokeGlobalRole(context.Context, string, string, int64, string, string, time.Time) (GlobalRoleAssignment, error)
}
