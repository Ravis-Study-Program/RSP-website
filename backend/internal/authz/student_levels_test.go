package authz

import "testing"

func TestCanSetStudentLevel(t *testing.T) {
	for _, test := range []struct {
		name     string
		role     SeasonRole
		state    EnrollmentState
		season   string
		account  AccountState
		verified bool
		want     bool
	}{
		{"any active mentor", Mentor, Active, "season", AccountActive, true, true},
		{"coordinator", Coordinator, Active, "season", AccountActive, true, true},
		{"student", Student, Active, "season", AccountActive, true, false},
		{"other season", Mentor, Active, "other", AccountActive, true, false},
		{"former mentor", Mentor, Completed, "season", AccountActive, true, false},
		{"removed mentor", Mentor, Kicked, "season", AccountActive, true, false},
		{"suspended", Mentor, Active, "season", Suspended, true, false},
		{"unverified", Mentor, Active, "season", AccountActive, false, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			actor := Actor{UserID: "mentor", EmailVerified: test.verified, AccountState: test.account,
				Enrollments: []Enrollment{{SeasonID: test.season, Role: test.role, State: test.state}}}
			if got := actor.CanSetStudentLevel("season"); got != test.want {
				t.Fatalf("CanSetStudentLevel=%v want %v", got, test.want)
			}
			if test.role == Mentor && actor.CanPromoteOrRemoveStudent("season", false, true) {
				t.Fatal("level permission must not allow removing an unassigned student")
			}
		})
	}
}
