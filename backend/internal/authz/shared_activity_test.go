package authz

import "testing"

func TestSharedActivityDoesNotGrantContactOrWriteAccess(t *testing.T) {
	for _, role := range []SeasonRole{Student, Mentor, Coordinator} {
		actor := activeActor("viewer")
		actor.Enrollments = []Enrollment{{"season", role, Active}}
		if !actor.CanReadSharedActivity() || !actor.CanViewSeason("another") {
			t.Fatalf("%s cannot browse shared history", role)
		}
		if actor.CanViewMemberPrivateData("other", MemberRelationship{}) {
			t.Fatalf("%s gained unrelated contact access", role)
		}
		actor.Enrollments[0].State = Kicked
		if actor.CanReadSharedActivity() || !actor.CanRecordActivity() {
			t.Fatal("removed members must retain personal activity only")
		}
		actor.AccountState = Suspended
		if actor.CanReadSharedActivity() || actor.CanRecordActivity() {
			t.Fatal("suspended member has product access")
		}
	}
}
