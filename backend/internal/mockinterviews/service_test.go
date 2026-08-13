package mockinterviews

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func score(v int) *int { return &v }
func validCreate() CreateInput {
	return CreateInput{Interviewee: Participant{UserID: "student", ActiveMember: true}, OccurredAt: time.Now(), DurationMinutes: 60, Notes: "<script>x</script><b>ok</b>", Rounds: []Round{{ID: "r1", Type: Behavioural, Scores: Scores{Behavioural: score(7)}}}}
}
func TestOwnershipVersionsReviewAndPass(t *testing.T) {
	s := Service{Sanitize: func(v string) string { return strings.ReplaceAll(v, "<script>x</script>", "") }}
	m, err := s.Create("mentor", validCreate(), time.Now())
	if err != nil || m.InterviewerID != "mentor" || len(s.Versions) != 1 || !Passed(m) {
		t.Fatal("create failed")
	}
	if err := s.Update(&m, "student", UpdateInput{ExpectedRevision: 1}, time.Now()); !errors.Is(err, ErrForbidden) {
		t.Fatalf("ownership: %v", err)
	}
	if err := s.Review(&m, "student", "r1", "thanks", true, 1, time.Now()); err != nil || len(s.Versions) != 2 {
		t.Fatalf("review: %v", err)
	}
	if err := s.Delete(&m, "mentor", 1, time.Now()); !errors.Is(err, ErrConflict) {
		t.Fatalf("stale edit: %v", err)
	}
}
func TestParticipantEligibility(t *testing.T) {
	for _, p := range []Participant{{UserID: "none"}, {UserID: "s", ActiveMember: true, Suspended: true}, {UserID: "d", Alumni: true, Deleted: true}, {UserID: "t", ActiveMember: true, Test: true}, {UserID: "k", ActiveMember: true, KickedOnly: true}} {
		in := validCreate()
		in.Interviewee = p
		if _, err := new(Service).Create("mentor", in, time.Now()); !errors.Is(err, ErrForbidden) {
			t.Fatalf("participant allowed: %#v", p)
		}
	}
}
func TestAllApplicableScoresMustPass(t *testing.T) {
	m := Interview{Rounds: []Round{{Scores: Scores{ConfirmQuestions: score(8), Coding: score(4)}}}}
	if Passed(m) {
		t.Fatal("failed score passed")
	}
	m.Rounds[0].Scores.Coding = score(5)
	if !Passed(m) {
		t.Fatal("passing scores failed")
	}
}
