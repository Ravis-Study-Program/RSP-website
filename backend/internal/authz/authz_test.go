package authz

import (
	"testing"
	"time"
)

func activeActor(id string) Actor {
	return Actor{
		UserID:        id,
		EmailVerified: true,
		AccountState:  AccountActive,
		GlobalRoles:   map[GlobalRole]bool{},
	}
}

func TestPermissionMatrix(t *testing.T) {
	now := time.Now().UTC()
	recent := now.Add(-time.Minute)
	cases := []struct {
		name                      string
		actor                     Actor
		eligible, manage, private bool
	}{
		{"anonymous", Actor{}, false, false, false},
		{"unverified", Actor{UserID: "u", AccountState: AccountActive}, false, false, false},
		{"nonmember", activeActor("u"), false, false, false},
		{"student", func() Actor { a := activeActor("u"); a.Enrollments = []Enrollment{{"s", Student, Active}}; return a }(), true, false, false},
		{"mentor", func() Actor { a := activeActor("u"); a.Enrollments = []Enrollment{{"s", Mentor, Active}}; return a }(), true, false, true},
		{"coordinator", func() Actor {
			a := activeActor("u")
			a.Enrollments = []Enrollment{{"s", Coordinator, Active}}
			return a
		}(), true, true, true},
		{"graduate", func() Actor { a := activeActor("u"); a.Enrollments = []Enrollment{{"s", Student, Completed}}; return a }(), true, false, false},
		{"director", func() Actor { a := activeActor("u"); a.GlobalRoles[Director] = true; a.MFAAt = &recent; return a }(), false, true, true},
		{"system admin", func() Actor { a := activeActor("u"); a.GlobalRoles[SystemAdmin] = true; a.MFAAt = &recent; return a }(), false, true, true},
		{"kicked", func() Actor { a := activeActor("u"); a.Enrollments = []Enrollment{{"s", Student, Kicked}}; return a }(), false, false, false},
		{"suspended", Actor{
			UserID:        "u",
			EmailVerified: true,
			AccountState:  Suspended,
		}, false, false, false},
		{"deleted", Actor{
			UserID:        "u",
			EmailVerified: true,
			AccountState:  Deleted,
		}, false, false, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.actor.CanAccessMemberDirectory(); got != tc.eligible {
				t.Fatalf("eligible=%v", got)
			}
			if got := tc.actor.IsSeasonAdmin("s"); got != tc.manage {
				t.Fatalf("manage=%v", got)
			}
			if got := tc.actor.CanViewMemberPrivateData("target", MemberRelationship{
				SeasonID:       "s",
				TargetEnrolled: true,
				AssignedMentor: true,
			}); got != tc.private {
				t.Fatalf("private=%v", got)
			}
		})
	}
}

func TestAlumniIsDerivedOnlyFromCompletedStudent(t *testing.T) {
	for _, e := range []Enrollment{{"s", Student, Active}, {"s", Student, Kicked}, {"s", Mentor, Completed}} {
		a := activeActor("u")
		a.Enrollments = []Enrollment{e}
		if a.IsStudentAlumnus() {
			t.Fatalf("unexpected alumni for %#v", e)
		}
	}
	a := activeActor("u")
	a.Enrollments = []Enrollment{{"s", Student, Completed}}
	if !a.IsStudentAlumnus() {
		t.Fatal("completed student was not alumni")
	}
}

func TestSeasonAdminIsSeasonSpecificAndIndependentOfMFA(t *testing.T) {
	actor := activeActor("coordinator")
	actor.Enrollments = []Enrollment{{
		SeasonID: "managed",
		Role:     Coordinator,
		State:    Active,
	}, {
		SeasonID: "historical",
		Role:     Coordinator,
		State:    Completed,
	}}
	if !actor.IsSeasonAdmin("managed") || actor.IsSeasonAdmin("another") || actor.IsSeasonAdmin("historical") {
		t.Fatal("season admin role scope was not respected")
	}
	if actor.HasRecentMFA(time.Now()) {
		t.Fatal("fixture unexpectedly has MFA")
	}
	actor.AccountState = Suspended
	if actor.IsSeasonAdmin("managed") {
		t.Fatal("suspended coordinator may administer season")
	}
}
