package mockinterviews

import (
	"errors"
	"time"
	_ "time/tzdata"
)

var programTimezone = func() *time.Location {
	zone, err := time.LoadLocation("Australia/Adelaide")
	if err != nil {
		panic(err)
	}
	return zone
}()

// ValidateRecordingTime accepts any time up to now. Interviews can be recorded
// after the day they happened, but never in the future.
func ValidateRecordingTime(occurredAt, now time.Time) error {
	if occurredAt.IsZero() || occurredAt.After(now) {
		return errors.New("mock interviews cannot be recorded in the future")
	}
	return nil
}
