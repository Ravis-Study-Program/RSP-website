package authz

import "testing"

func TestCanSetPracticeGoals(t *testing.T) {
	active := func(role SeasonRole, state EnrollmentState) Actor {
		return Actor{UserID: "actor", EmailVerified: true, AccountState: AccountActive,
			Enrollments: []Enrollment{{SeasonID: "season", Role: role, State: state}}}
	}
	director := Actor{UserID: "director", EmailVerified: true, AccountState: AccountActive, GlobalRoles: map[GlobalRole]bool{Director: true}}
	systemAdmin := Actor{UserID: "admin", EmailVerified: true, AccountState: AccountActive, GlobalRoles: map[GlobalRole]bool{SystemAdmin: true}}
	suspended := director
	suspended.AccountState = Suspended
	for _, test := range []struct {
		name               string
		actor              Actor
		assigned, inactive bool
		student, programme bool
	}{
		{"assigned mentor", active(Mentor, Active), true, false, true, false},
		{"unassigned mentor", active(Mentor, Active), false, false, false, false},
		{"former mentor", active(Mentor, Completed), true, false, false, false},
		{"coordinator", active(Coordinator, Active), false, false, true, false},
		{"student", active(Student, Active), false, false, false, false},
		{"inactive student", active(Coordinator, Active), false, true, false, false},
		{"director", director, false, false, true, true},
		{"system admin", systemAdmin, false, false, true, true},
		{"suspended director", suspended, false, false, false, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := test.actor.CanSetStudentPracticeGoals("season", test.assigned, !test.inactive); got != test.student {
				t.Fatalf("CanSetStudentPracticeGoals=%v want %v", got, test.student)
			}
			if got := test.actor.CanSetProgrammePracticeGoals(); got != test.programme {
				t.Fatalf("CanSetProgrammePracticeGoals=%v want %v", got, test.programme)
			}
		})
	}
}
