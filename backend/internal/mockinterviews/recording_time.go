package mockinterviews

import (
	"errors"
	"time"
	_ "time/tzdata"
)

var programmeTimezone = func() *time.Location {
	zone, err := time.LoadLocation("Australia/Adelaide")
	if err != nil {
		panic(err)
	}
	return zone
}()

func ValidateRecordingTime(occurredAt, now time.Time) error {
	if occurredAt.IsZero() || occurredAt.After(now) || occurredAt.In(programmeTimezone).Format("2006-01-02") != now.In(programmeTimezone).Format("2006-01-02") {
		return errors.New("mock interviews must be recorded on the day they happen in Australia/Adelaide time; future times are not allowed")
	}
	return nil
}
