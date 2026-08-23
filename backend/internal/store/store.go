package store

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/magedmg/RSP-website/backend/internal/accounts"
	"github.com/magedmg/RSP-website/backend/internal/authz"
	"github.com/magedmg/RSP-website/backend/internal/mockinterviews"
	"github.com/magedmg/RSP-website/backend/internal/model"
	"github.com/magedmg/RSP-website/backend/internal/practice"
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

// ObservabilitySnapshot contains only bounded, aggregate operational data.
// Keeping this separate from Repository lets non-PostgreSQL stores remain
// lightweight while the API metrics endpoint can expose durable worker and
// migration state when it is available.
type ObservabilitySnapshot struct {
	DBPoolAcquiredConnections int32
	DBPoolIdleConnections     int32
	WorkerRuns                map[string]uint64
	MigrationState            string
}

// RecommendationSnapshot represents a backend data structure.
type RecommendationSnapshot struct {
	QualityAttempts  []model.Attempt
	Problems         []model.Problem
	ProblemHistory   map[string]practice.ProblemHistory
	CategoryExposure map[string]int
}

// ObservabilitySource defines a backend interface.
type ObservabilitySource interface {
	ObservabilitySnapshot(context.Context) (ObservabilitySnapshot, error)
}

// Repository defines a backend interface.
type Repository interface {
	ApplyIdentityEvent(context.Context, accounts.IdentityEvent) error
	ResolveAuthSubject(context.Context, string) (authz.Actor, error)
	GetUser(context.Context, string) (model.User, error)
	SuggestUserSlug(context.Context) (string, error)
	ListUsers(context.Context, string, int, string, string, string, string) ([]model.User, bool, int64, error)
	ListAdminUsers(context.Context, string, int, string, string, string, string) ([]model.User, bool, int64, error)
	ListEnrollmentCandidates(context.Context, string, string, string, int, string) ([]model.EnrollmentCandidate, bool, int64, error)
	UpdateUser(context.Context, string, int64, func(*model.User) error, string, time.Time) (model.User, error)
	ResolveAuthSubjectForUser(context.Context, string) (string, error)
	GrantGlobalRole(context.Context, string, string, bool, string, string, time.Time) (accounts.GlobalRoleAssignment, error)
	ListGlobalRoles(context.Context, string) ([]accounts.GlobalRoleAssignment, error)
	RevokeGlobalRole(context.Context, string, string, int64, string, string, time.Time) (accounts.GlobalRoleAssignment, error)
	GetPracticeSettings(context.Context, string) (model.PracticeSettings, error)
	UpdatePracticeSettings(context.Context, string, int64, bool, int, int, int, string, time.Time) (model.PracticeSettings, error)
	EnablePracticeGoals(context.Context, string, int64, string, string, time.Time) (model.PracticeSettings, error)
	GetSeason(context.Context, string) (model.Season, error)
	ListSeasons(context.Context, string, int, string, string) ([]model.Season, bool, int64, error)
	CreateSeason(context.Context, model.Season, string, time.Time) (model.Season, error)
	UpdateSeason(context.Context, string, int64, func(*model.Season) error, string, time.Time) (model.Season, error)
	CloseSeason(context.Context, string, int64, string, string, time.Time) (model.Season, error)
	ReopenSeason(context.Context, string, int64, string, string, time.Time) (model.Season, error)
	ListWeeks(context.Context, string, string, int, string, string) ([]model.Week, bool, int64, error)
	CreateWeek(context.Context, model.Week, string, time.Time) (model.Week, error)
	UpdateWeek(context.Context, string, string, int64, model.Week, string, time.Time) (model.Week, error)
	DeleteWeek(context.Context, string, string, int64, string, time.Time) error
	ListEnrollments(context.Context, string, string, int, string, string, string, string, bool) ([]model.Enrollment, bool, int64, error)
	ListEnrollmentsForUser(context.Context, string) ([]model.Enrollment, error)
	GetEnrollment(context.Context, string) (model.Enrollment, error)
	CreateEnrollment(context.Context, model.Enrollment, string, time.Time) (model.Enrollment, error)
	UpdateEnrollmentDetails(context.Context, string, string, int64, string, string, string, time.Time) (model.Enrollment, error)
	UpdateEnrollment(context.Context, string, int64, string, string, string, string, time.Time) (model.Enrollment, error)
	ListMentorships(context.Context, string, string, int, string, string, string, string) ([]model.Mentorship, bool, int64, error)
	CreateMentorship(context.Context, model.Mentorship, string, time.Time) (model.Mentorship, error)
	UpdateMentorship(context.Context, string, string, int64, string, string, string, time.Time) (model.Mentorship, error)
	DeleteMentorship(context.Context, string, string, int64, string, time.Time) error
	IsMentorAssigned(context.Context, string, string, string) (bool, error)
	ListProblems(context.Context, string, int, string, string, *bool, string) ([]model.Problem, bool, int64, error)
	GetAttempt(context.Context, string) (model.Attempt, error)
	ListAttempts(context.Context, string, string, int, string, string, string) ([]model.Attempt, bool, int64, error)
	RecommendationSnapshot(context.Context, string, practice.Goals) (RecommendationSnapshot, error)
	RecommendationCandidates(context.Context, string, practice.Criteria, bool, practice.Goals, time.Time) ([]model.Problem, map[string]practice.ProblemHistory, error)
	CreateAttempt(context.Context, model.Attempt) (model.Attempt, bool, error)
	UpdateAttempt(context.Context, string, string, int64, func(*model.Attempt) error) (model.Attempt, error)
	DeleteAttempt(context.Context, string, string, int64) error
	GetActiveRecommendation(context.Context, string) (*practice.Recommendation, error)
	SaveRecommendation(context.Context, practice.Recommendation) (practice.Recommendation, error)
	ListRecommendationDismissals(context.Context, string) ([]practice.Dismissal, error)
	DismissRecommendation(context.Context, string, int64, string, time.Time, string) (practice.Recommendation, error)
	FulfillRecommendation(context.Context, string, model.Attempt, time.Time) (bool, error)
	ListMockInterviews(context.Context, authz.Actor, string, string, int, string, string) ([]mockinterviews.Interview, bool, int64, error)
	GetMockParticipant(context.Context, string) (mockinterviews.Participant, error)
	GetMockInterview(context.Context, string) (mockinterviews.Interview, error)
	CreateMockInterview(context.Context, mockinterviews.Interview, string, time.Time) (mockinterviews.Interview, error)
	UpdateMockInterview(context.Context, mockinterviews.Interview, string, string, time.Time) (mockinterviews.Interview, error)
	AppendAudit(context.Context, model.AuditEvent) error
}
