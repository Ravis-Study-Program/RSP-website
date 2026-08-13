package authz

import (
	"testing"
	"time"
)

func activeActor(id string) Actor {
	return Actor{UserID: id, EmailVerified: true, AccountState: AccountActive, GlobalRoles: map[GlobalRole]bool{}}
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
		{"suspended", Actor{UserID: "u", EmailVerified: true, AccountState: Suspended}, false, false, false},
		{"deleted", Actor{UserID: "u", EmailVerified: true, AccountState: Deleted}, false, false, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.actor.EligibleMember(); got != tc.eligible {
				t.Fatalf("eligible=%v", got)
			}
			if got := tc.actor.CanManageSeason("s", true); got != tc.manage {
				t.Fatalf("manage=%v", got)
			}
			if got := tc.actor.CanViewPrivate("target", "s", true); got != tc.private {
				t.Fatalf("private=%v", got)
			}
		})
	}
}

func TestAlumniIsDerivedOnlyFromCompletedStudent(t *testing.T) {
	for _, e := range []Enrollment{{"s", Student, Active}, {"s", Student, Kicked}, {"s", Mentor, Completed}} {
		a := activeActor("u")
		a.Enrollments = []Enrollment{e}
		if a.Alumni() {
			t.Fatalf("unexpected alumni for %#v", e)
		}
	}
	a := activeActor("u")
	a.Enrollments = []Enrollment{{"s", Student, Completed}}
	if !a.Alumni() {
		t.Fatal("completed student was not alumni")
	}
}
