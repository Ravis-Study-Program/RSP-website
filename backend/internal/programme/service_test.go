package programme

import (
	"testing"
	"time"
)

func TestCloseReopenRestoresOnlyCloseCompletions(t *testing.T) {
	s := Season{ID: "s1", Status: "open", Enrollments: []Enrollment{{ID: "active", State: "active"}, {ID: "kicked", State: "kicked"}, {ID: "old", State: "completed"}}}
	closed, err := Close(s, "u1", "ended", "close-s1", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if s.Status != "open" || s.Enrollments[0].State != "active" {
		t.Fatal("close mutated its input")
	}
	if closed.Season.Enrollments[0].State != "completed" || closed.Season.Enrollments[1].State != "kicked" {
		t.Fatal("bad close")
	}
	reopened, err := Reopen(closed.Season, *closed.CloseEvent, Director)
	if err != nil {
		t.Fatal(err)
	}
	if reopened.Enrollments[0].State != "active" || reopened.Enrollments[1].State != "kicked" || reopened.Enrollments[2].State != "completed" {
		t.Fatal("bad reopen")
	}
}
