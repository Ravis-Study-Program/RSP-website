package program

import (
	"testing"
	"time"
)

func TestMondayUpdateStartedWeek(t *testing.T) {
	for _, tc := range []struct {
		name, start, end, now string
		valid                 bool
	}{
		// A week opens at Adelaide midnight on its Monday.
		{"before the week starts", "2026-10-05", "2026-10-11", "2026-10-04T13:29:59Z", false},
		{"Adelaide Monday midnight", "2026-10-05", "2026-10-11", "2026-10-04T13:30:00Z", true},
		{"current week", "2026-10-12", "2026-10-18", "2026-10-17T04:00:00Z", true},
		{"ended week", "2026-09-21", "2026-09-27", "2026-10-11T13:30:00Z", true},
		{"next week", "2026-10-19", "2026-10-25", "2026-10-17T04:00:00Z", false},
		{"arbitrary range", "2026-10-06", "2026-10-11", "2026-10-12T04:00:00Z", false},
		{"two weeks", "2026-09-28", "2026-10-11", "2026-10-12T04:00:00Z", false},
		{"DST end before midnight", "2026-04-06", "2026-04-12", "2026-04-05T14:29:59Z", false},
		{"DST end midnight", "2026-04-06", "2026-04-12", "2026-04-05T14:30:00Z", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			now, err := time.Parse(time.RFC3339, tc.now)
			if err != nil {
				t.Fatal(err)
			}
			err = ValidateMondayUpdateDates(tc.start, tc.end, now)
			if (err == nil) != tc.valid {
				t.Fatalf("valid=%v, error=%v", tc.valid, err)
			}
		})
	}
}
