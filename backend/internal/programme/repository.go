package programme

import (
	"context"
	"time"
)

// Repository defines the persistence port required by programme features.
type Repository interface {
	GetSeason(context.Context, string) (SeasonRecord, error)
	ListSeasons(context.Context, string, int, string, string) ([]SeasonRecord, bool, int64, error)
	CreateSeason(context.Context, SeasonRecord, string, time.Time) (SeasonRecord, error)
	UpdateSeason(context.Context, string, int64, func(*SeasonRecord) error, string, time.Time) (SeasonRecord, error)
	CloseSeason(context.Context, string, int64, string, string, time.Time) (SeasonRecord, error)
	ReopenSeason(context.Context, string, int64, string, string, time.Time) (SeasonRecord, error)
	ListWeeks(context.Context, string, string, int, string, string) ([]WeekRecord, bool, int64, error)
	CreateWeek(context.Context, WeekRecord, string, time.Time) (WeekRecord, error)
	UpdateWeek(context.Context, string, string, int64, WeekRecord, string, time.Time) (WeekRecord, error)
	DeleteWeek(context.Context, string, string, int64, string, time.Time) error
	ListEnrollments(context.Context, string, string, int, string, string, string, string, bool) ([]EnrollmentRecord, bool, int64, error)
	ListEnrollmentsForUser(context.Context, string) ([]EnrollmentRecord, error)
	GetEnrollment(context.Context, string) (EnrollmentRecord, error)
	CreateEnrollment(context.Context, EnrollmentRecord, string, time.Time) (EnrollmentRecord, error)
	UpdateEnrollmentDetails(context.Context, string, string, int64, string, string, string, time.Time) (EnrollmentRecord, error)
	UpdateEnrollment(context.Context, string, int64, string, string, string, string, time.Time) (EnrollmentRecord, error)
	ListMentorships(context.Context, string, string, int, string, string, string, string) ([]MentorshipRecord, bool, int64, error)
	CreateMentorship(context.Context, MentorshipRecord, string, time.Time) (MentorshipRecord, error)
	UpdateMentorship(context.Context, string, string, int64, string, string, string, time.Time) (MentorshipRecord, error)
	DeleteMentorship(context.Context, string, string, int64, string, time.Time) error
	IsMentorAssigned(context.Context, string, string, string) (bool, error)
}
