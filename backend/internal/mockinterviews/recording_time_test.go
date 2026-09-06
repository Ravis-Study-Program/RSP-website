package mockinterviews

import (
	"testing"
	"time"
)

func TestRecordingTimeUsesAdelaideDay(t *testing.T) {
	for _, date := range []string{"2026-01-10T00:10:00+10:30", "2026-07-10T00:10:00+09:30", "2026-10-04T03:10:00+10:30", "2026-04-05T03:10:00+09:30"} {
		now, err := time.Parse(time.RFC3339, date)
		if err != nil {
			t.Fatal(err)
		}
		local := now.In(programmeTimezone)
		midnight := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, programmeTimezone)
		for _, test := range []struct {
			name  string
			at    time.Time
			valid bool
		}{
			{"now", now, true}, {"midnight", midnight, true}, {"previous day", midnight.Add(-time.Second), false}, {"future", now.Add(time.Minute), false}, {"missing", time.Time{}, false},
		} {
			if got := ValidateRecordingTime(test.at, now) == nil; got != test.valid {
				t.Errorf("%s %s valid=%v", date, test.name, got)
			}
		}
	}
}
