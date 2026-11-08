package program

import (
	"errors"
	"math"
	"time"
	_ "time/tzdata"
)

// programTimezone is the boundary zone for program weeks, including its
// daylight-saving rules.
var programTimezone = func() *time.Location {
	zone, err := time.LoadLocation("Australia/Adelaide")
	if err != nil {
		panic(err)
	}
	return zone
}()

// MondayUpdateMark is one mentor's verdict on one Monday update.
type MondayUpdateMark string

const (
	MarkWayAhead   MondayUpdateMark = "way_ahead"
	MarkDoingGood  MondayUpdateMark = "doing_good"
	MarkOnTrack    MondayUpdateMark = "on_track"
	MarkABitBehind MondayUpdateMark = "a_bit_behind"
	MarkAtRisk     MondayUpdateMark = "at_risk"
	// MarkKicked is the mentor's mark for the week a student was kicked.
	MarkKicked MondayUpdateMark = "kicked"
)

// ValidMondayUpdateMark reports whether a client-supplied mark is one of the five.
func ValidMondayUpdateMark(value string) bool {
	switch MondayUpdateMark(value) {
	case MarkWayAhead, MarkDoingGood, MarkOnTrack, MarkABitBehind, MarkAtRisk, MarkKicked:
		return true
	default:
		return false
	}
}

// PersonSummary is the compact person shape Monday update responses carry.
type PersonSummary struct {
	ID        string  `json:"id"`
	Slug      string  `json:"slug"`
	Name      string  `json:"name"`
	AvatarURL *string `json:"avatarUrl,omitempty"`
}

// MondayUpdateAttempt is one problem attempt inside an update's own date range.
type MondayUpdateAttempt struct {
	ID          string    `json:"id"`
	ProblemID   string    `json:"problemId"`
	Problem     string    `json:"problem"`
	ProblemURL  string    `json:"problemUrl"`
	Difficulty  string    `json:"difficulty"`
	Outcome     string    `json:"outcome"`
	Minutes     int       `json:"minutes"`
	AttemptedAt time.Time `json:"attemptedAt"`
}

// MondayUpdateMock is one mock interview inside an update's own date range.
type MondayUpdateMock struct {
	ID              string    `json:"id"`
	OccurredAt      time.Time `json:"occurredAt"`
	DurationMinutes int       `json:"durationMinutes"`
}

// MondayUpdateActivity is the activity one date range includes. It is derived
// from the range on every read and is never stored with the update.
type MondayUpdateActivity struct {
	Attempts []MondayUpdateAttempt `json:"attempts"`
	Mocks    []MondayUpdateMock    `json:"mocks"`
}

// MondayUpdateMarkRecord is one mentor's mark and comment on one update.
type MondayUpdateMarkRecord struct {
	ID          string        `json:"id"`
	UpdateID    string        `json:"-"`
	Mentor      PersonSummary `json:"mentor"`
	Mark        string        `json:"mark"`
	CommentHTML string        `json:"commentHtml"`
	UpdatedAt   time.Time     `json:"updatedAt"`
}

// MondayUpdateRecord is one submitted Monday update.
type MondayUpdateRecord struct {
	ID          string                   `json:"id"`
	SeasonID    string                   `json:"seasonId"`
	Student     PersonSummary            `json:"student"`
	StartDate   string                   `json:"startDate"`
	EndDate     string                   `json:"endDate"`
	ContentHTML string                   `json:"contentHtml"`
	Hours       float64                  `json:"hours"`
	CreatedAt   time.Time                `json:"createdAt"`
	EditedAt    *time.Time               `json:"editedAt"`
	Marks       []MondayUpdateMarkRecord `json:"marks"`
	Activity    MondayUpdateActivity     `json:"activity"`
}

// MondayUpdateDifficultyCounts counts solved problems by difficulty.
type MondayUpdateDifficultyCounts struct {
	Easy   int64 `json:"easy"`
	Medium int64 `json:"medium"`
	Hard   int64 `json:"hard"`
}

// MondayUpdateReviewUpdate is one update inside a reviewer row.
type MondayUpdateReviewUpdate struct {
	ID          string                   `json:"id"`
	StartDate   string                   `json:"startDate"`
	EndDate     string                   `json:"endDate"`
	ContentHTML string                   `json:"contentHtml"`
	Hours       float64                  `json:"hours"`
	CreatedAt   time.Time                `json:"createdAt"`
	EditedAt    *time.Time               `json:"editedAt"`
	Marks       []MondayUpdateMarkRecord `json:"marks"`
}

// MondayUpdateReviewRow summarises one student enrolled in the season.
type MondayUpdateReviewRow struct {
	Student        PersonSummary                `json:"student"`
	LeetCodeCounts MondayUpdateDifficultyCounts `json:"leetcodeCounts"`
	MockCount      int64                        `json:"mockCount"`
	UpdateCount    int64                        `json:"updateCount"`
	Updates        []MondayUpdateReviewUpdate   `json:"updates"`
}

// ParseMondayUpdateDate validates one calendar date with no time component.
func ParseMondayUpdateDate(value string) (time.Time, error) {
	parsed, err := time.Parse(time.DateOnly, value)
	if err != nil {
		return time.Time{}, errors.New("dates must use the YYYY-MM-DD format")
	}
	return parsed, nil
}

// ValidateMondayUpdateDates accepts a Monday–Sunday week in Adelaide once it
// has started, so the current week can be written as it goes.
func ValidateMondayUpdateDates(startDate, endDate string, now time.Time) error {
	start, err := ParseMondayUpdateDate(startDate)
	if err != nil {
		return err
	}
	end, err := ParseMondayUpdateDate(endDate)
	if err != nil {
		return err
	}
	if start.Weekday() != time.Monday || !end.Equal(start.AddDate(0, 0, 6)) {
		return errors.New("updates must cover one Monday to Sunday week")
	}
	if startDate > now.In(programTimezone).Format(time.DateOnly) {
		return errors.New("updates can be written once the week has started")
	}
	return nil
}

// CurrentMondayUpdateWeekEnd is the Sunday of the Monday-anchored
// Australia/Adelaide week containing now, as a calendar date.
func CurrentMondayUpdateWeekEnd(now time.Time) string {
	local := now.In(programTimezone)
	weekday := (int(local.Weekday()) + 6) % 7
	return time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, time.UTC).
		AddDate(0, 0, 6-weekday).Format(time.DateOnly)
}

// ValidateMondayUpdateHours rejects reported hours outside 0–168 or not on a
// half-hour step.
func ValidateMondayUpdateHours(hours float64) error {
	if math.IsNaN(hours) || hours < 0 || hours > 168 || hours*2 != math.Trunc(hours*2) {
		return errors.New("hours must be between 0 and 168 in steps of 0.5")
	}
	return nil
}

// MondayUpdateUnlock is a student's request to post or change a week's
// update after its due date, and their mentor's decision on it.
type MondayUpdateUnlock struct {
	ID          string         `json:"id"`
	SeasonID    string         `json:"seasonId"`
	Student     PersonSummary  `json:"student"`
	StartDate   string         `json:"startDate"`
	EndDate     string         `json:"endDate"`
	Reason      string         `json:"reason"`
	State       string         `json:"state"`
	RequestedAt time.Time      `json:"requestedAt"`
	DecidedAt   *time.Time     `json:"decidedAt"`
	DecidedBy   *PersonSummary `json:"decidedBy"`
}

// MaxUnlockReasonLength is the longest reason a student can give.
const MaxUnlockReasonLength = 1000

// MondayUpdatePastDue reports whether a week's update is past its due date:
// the end of the Monday after the week, in Australia/Adelaide.
func MondayUpdatePastDue(endDate string, now time.Time) bool {
	end, err := ParseMondayUpdateDate(endDate)
	if err != nil {
		return false
	}
	due := end.AddDate(0, 0, 1).Format(time.DateOnly)
	return now.In(programTimezone).Format(time.DateOnly) > due
}

// MondayWeekMark is a mentor's mark on a student's week with no update.
type MondayWeekMark struct {
	SeasonID    string    `json:"seasonId"`
	StudentID   string    `json:"studentId"`
	MentorID    string    `json:"mentorId"`
	StartDate   string    `json:"startDate"`
	EndDate     string    `json:"endDate"`
	Mark        string    `json:"mark"`
	CommentHTML string    `json:"commentHtml"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// MondayMentorReportStudent is one mentee's week in a mentor's report: what
// they posted and did, and the mentor's mark and comment. The student's own
// writing is never part of it.
type MondayMentorReportStudent struct {
	Student      PersonSummary `json:"student"`
	StudentLevel string        `json:"studentLevel"`
	// KickedAt is when the student was kicked: during this week or after it.
	KickedAt *time.Time `json:"kickedAt"`
	// Targets are the student's weekly targets: their own where set,
	// otherwise the program's.
	Targets        MondayWeeklyTargets          `json:"targets"`
	UpdateID       *string                      `json:"updateId"`
	PostedAt       *time.Time                   `json:"postedAt"`
	EditedAt       *time.Time                   `json:"editedAt"`
	Hours          *float64                     `json:"hours"`
	LeetCodeCounts MondayUpdateDifficultyCounts `json:"leetcodeCounts"`
	MockCount      int64                        `json:"mockCount"`
	Mark           *string                      `json:"mark"`
	CommentHTML    string                       `json:"commentHtml"`
}

// MondayMentorReport is a mentor's week: every current mentee and the
// mentor's mark on each.
type MondayMentorReport struct {
	Mentor    PersonSummary               `json:"mentor"`
	StartDate string                      `json:"startDate"`
	EndDate   string                      `json:"endDate"`
	Students  []MondayMentorReportStudent `json:"students"`
}

// MondayWeeklyTargets are what a student is expected to do in a week. The
// weekly LeetCode counts are fixed in the web app for now.
type MondayWeeklyTargets struct {
	Hours int `json:"hours"`
}
