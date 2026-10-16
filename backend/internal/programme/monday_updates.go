package programme

import (
	"errors"
	"time"
	_ "time/tzdata"
)

// programmeTimezone is the boundary zone for programme weeks, including its
// daylight-saving rules.
var programmeTimezone = func() *time.Location {
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
)

// ValidMondayUpdateMark reports whether a client-supplied mark is one of the five.
func ValidMondayUpdateMark(value string) bool {
	switch MondayUpdateMark(value) {
	case MarkWayAhead, MarkDoingGood, MarkOnTrack, MarkABitBehind, MarkAtRisk:
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
	CreatedAt   time.Time                `json:"createdAt"`
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
	CreatedAt   time.Time                `json:"createdAt"`
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

// ValidateMondayUpdateDates rejects a reversed range and a start date later than
// the end of the current Australia/Adelaide week.
func ValidateMondayUpdateDates(startDate, endDate string, now time.Time) error {
	start, err := ParseMondayUpdateDate(startDate)
	if err != nil {
		return err
	}
	end, err := ParseMondayUpdateDate(endDate)
	if err != nil {
		return err
	}
	if end.Before(start) {
		return errors.New("endDate must not be before startDate")
	}
	if start.Format(time.DateOnly) > CurrentMondayUpdateWeekEnd(now) {
		return errors.New("startDate cannot be after the current Australia/Adelaide week")
	}
	return nil
}

// CurrentMondayUpdateWeekEnd is the Sunday of the Monday-anchored
// Australia/Adelaide week containing now, as a calendar date.
func CurrentMondayUpdateWeekEnd(now time.Time) string {
	local := now.In(programmeTimezone)
	weekday := (int(local.Weekday()) + 6) % 7
	return time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, time.UTC).
		AddDate(0, 0, 6-weekday).Format(time.DateOnly)
}
