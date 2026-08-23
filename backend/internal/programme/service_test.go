package programme

import (
	"testing"
	"time"
)

func TestCloseReopenRestoresOnlyCloseCompletions(t *testing.T) {
	s := Season{ID: "s1", Status: "open", Enrollments: []Enrollment{{ID: "active", State: "active"}, {ID: "kicked", State: "kicked"}, {ID: "old", State: "completed"}}}
	e, err := Close(&s, "u1", "ended", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if s.Enrollments[0].State != "completed" || s.Enrollments[1].State != "kicked" {
		t.Fatal("bad close")
	}
	if err := Reopen(&s, e, Director); err != nil {
		t.Fatal(err)
	}
	if s.Enrollments[0].State != "active" || s.Enrollments[1].State != "kicked" || s.Enrollments[2].State != "completed" {
		t.Fatal("bad reopen")
	}
}
