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
func TestInterviewerCannotWriteIntervieweeReview(t *testing.T) {
	s := Service{}
	in := validCreate()
	in.Rounds[0].Reviewed = true
	in.Rounds[0].IntervieweeComment = "forged on create"
	m, err := s.Create("mentor", in, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if m.Rounds[0].Reviewed || m.Rounds[0].IntervieweeComment != "" {
		t.Fatalf("create trusted interviewer review: %#v", m.Rounds[0])
	}
	if err := s.Review(&m, "student", "r1", "real review", true, 1, time.Now()); err != nil {
		t.Fatal(err)
	}
	updateRounds := append([]Round(nil), m.Rounds...)
	updateRounds[0].Reviewed = false
	updateRounds[0].IntervieweeComment = "forged update"
	if err := s.Update(&m, "mentor", UpdateInput{ExpectedRevision: 2, OccurredAt: m.OccurredAt, DurationMinutes: 70, Rounds: updateRounds}, time.Now()); err != nil {
		t.Fatal(err)
	}
	if !m.Rounds[0].Reviewed || m.Rounds[0].IntervieweeComment != "real review" {
		t.Fatalf("update overwrote interviewee review: %#v", m.Rounds[0])
	}
}
func TestParticipantEligibility(t *testing.T) {
	for _, p := range []Participant{{UserID: "none"}, {UserID: "former", FormerMember: true}, {UserID: "s", ActiveMember: true, Suspended: true}, {UserID: "d", Alumni: true, Deleted: true}, {UserID: "t", ActiveMember: true, Test: true}, {UserID: "k", ActiveMember: true, KickedOnly: true}} {
		in := validCreate()
		in.Interviewee = p
		if _, err := new(Service).Create("mentor", in, time.Now()); !errors.Is(err, ErrForbidden) {
			t.Fatalf("participant allowed: %#v", p)
		}
	}
	former := Participant{UserID: "former", FormerMember: true}
	if former.Eligible() || !former.ProgrammeAccessEligible() {
		t.Fatalf("former member target/caller eligibility was not separated: %#v", former)
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
