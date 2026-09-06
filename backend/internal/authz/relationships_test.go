package authz

import "testing"

func TestHistoricalPrivateAccessRequiresTheRightRelationship(t *testing.T) {
	cases := []struct {
		name         string
		role         SeasonRole
		state        EnrollmentState
		relationship MemberRelationship
		want         bool
	}{
		{"active coordinator with shared season", Coordinator, Active, MemberRelationship{SeasonID: "s", TargetEnrolled: true}, true},
		{"completed coordinator with shared season", Coordinator, Completed, MemberRelationship{SeasonID: "s", TargetEnrolled: true}, true},
		{"coordinator without shared season", Coordinator, Completed, MemberRelationship{SeasonID: "s"}, false},
		{"coordinator in another season", Coordinator, Active, MemberRelationship{SeasonID: "other", TargetEnrolled: true}, false},
		{"completed assigned mentor", Mentor, Completed, MemberRelationship{SeasonID: "s", AssignedMentor: true}, true},
		{"unassigned mentor", Mentor, Active, MemberRelationship{SeasonID: "s", TargetEnrolled: true}, false},
		{"kicked mentor", Mentor, Kicked, MemberRelationship{SeasonID: "s", AssignedMentor: true}, false},
		{"withdrawn coordinator", Coordinator, Withdrawn, MemberRelationship{SeasonID: "s", TargetEnrolled: true}, false},
		{"student cannot gain mentor access", Student, Active, MemberRelationship{
			SeasonID:       "s",
			TargetEnrolled: true,
			AssignedMentor: true,
		}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			actor := activeActor("viewer")
			actor.Enrollments = []Enrollment{{
				SeasonID: "s",
				Role:     tc.role,
				State:    tc.state,
			}}
			if got := actor.CanViewMemberPrivateData("target", tc.relationship); got != tc.want {
				t.Fatalf("private access=%v, want %v", got, tc.want)
			}
		})
	}
	actor := activeActor("viewer")
	if !actor.CanViewMemberPrivateData("viewer", MemberRelationship{}) {
		t.Fatal("member cannot view their own private data")
	}
	actor.AccountState = Suspended
	if actor.CanViewMemberPrivateData("viewer", MemberRelationship{}) {
		t.Fatal("suspended account can view private data")
	}
}

func TestSeasonAccessUsesCurrentAffiliation(t *testing.T) {
	cases := []struct {
		name        string
		enrollments []Enrollment
		want        bool
	}{
		{"active member", []Enrollment{{"s", Mentor, Active}}, true},
		{"alumnus", []Enrollment{{"s", Student, Completed}}, true},
		{"completed mentor only", []Enrollment{{"s", Mentor, Completed}}, true},
		{"completed coordinator with another active enrollment", []Enrollment{{"s", Coordinator, Completed}, {"other", Mentor, Active}}, true},
		{"completed mentor with alumni affiliation", []Enrollment{{"s", Mentor, Completed}, {"other", Student, Completed}}, true},
		{"kicked from requested season with alumni access", []Enrollment{{"s", Student, Kicked}, {"other", Student, Completed}}, true},
		{"shared season browsing", []Enrollment{{"other", Mentor, Active}}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			actor := activeActor("viewer")
			actor.Enrollments = tc.enrollments
			if got := actor.CanViewSeason("s"); got != tc.want {
				t.Fatalf("season access=%v, want %v", got, tc.want)
			}
		})
	}
}
