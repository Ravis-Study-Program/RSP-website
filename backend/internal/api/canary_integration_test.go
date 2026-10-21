//go:build integration

package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/magedmg/RSP-website/backend/internal/authz"
	"github.com/magedmg/RSP-website/backend/internal/dal"
)

func newCanaryHandler(t *testing.T, fixture postgresFixture, canary bool) http.Handler {
	t.Helper()
	if err := fixture.db.Seed(context.Background(), dal.SeedOptions{}); err != nil {
		t.Fatal(err)
	}
	authenticator := AuthenticatorFunc(func(_ context.Context, token string) (authz.Actor, error) {
		if token != "student" {
			return authz.Actor{}, errors.New("missing test actor")
		}
		return authz.Actor{UserID: studentID, EmailVerified: true, AccountState: authz.AccountActive}, nil
	})
	return New(Config{DB: fixture.db, Authenticator: authenticator, PublicOrigin: "https://rsp.test", Canary: canary}).Handler()
}

func putCanaryAccess(t *testing.T, handler http.Handler, body string) dal.CanaryAccess {
	t.Helper()
	response := testRequest(t, handler, http.MethodPut, "/api/v2/canary/access", "student", body)
	if response.Code != http.StatusOK {
		t.Fatalf("body=%s status=%d response=%s", body, response.Code, response.Body)
	}
	var access dal.CanaryAccess
	if err := json.Unmarshal(response.Body.Bytes(), &access); err != nil {
		t.Fatal(err)
	}
	return access
}

func TestCanaryRoutesAreAbsentOutsideCanary(t *testing.T) {
	fixture := newPostgresFixture(t)
	handler := newCanaryHandler(t, fixture, false)
	for _, method := range []string{http.MethodGet, http.MethodPut} {
		response := testRequest(t, handler, method, "/api/v2/canary/access", "student", `{"globalRoles":["system_admin"],"seasonRole":"coordinator"}`)
		if response.Code != http.StatusNotFound {
			t.Fatalf("%s status=%d body=%s", method, response.Code, response.Body)
		}
	}
}

func TestCanaryAccessSelfService(t *testing.T) {
	fixture := newPostgresFixture(t)
	ctx := context.Background()
	handler := newCanaryHandler(t, fixture, true)
	activeMentorships := func(slug string) int {
		t.Helper()
		var count int
		if err := fixture.pool.QueryRow(ctx, `
SELECT count(*) FROM app.mentorships m
JOIN app.enrollments me ON me.id=m.mentor_enrollment_id
JOIN app.enrollments se ON se.id=m.student_enrollment_id
JOIN app.user_profiles p ON p.user_id IN (me.user_id,se.user_id) AND p.slug=$2
WHERE m.ended_at IS NULL AND m.deleted_at IS NULL AND $1 IN (me.user_id,se.user_id)`, studentID, slug).Scan(&count); err != nil {
			t.Fatal(err)
		}
		return count
	}

	// Without TOTP, privileged roles are granted but held in pending_mfa.
	access := putCanaryAccess(t, handler, `{"globalRoles":["director"],"seasonRole":"coordinator"}`)
	if len(access.GlobalRoles) != 1 || access.GlobalRoles[0] != (dal.CanaryRoleState{Role: "director", State: "pending_mfa"}) {
		t.Fatalf("global roles=%+v", access.GlobalRoles)
	}
	if access.SeasonRole == nil || *access.SeasonRole != (dal.CanaryRoleState{Role: "coordinator", State: "pending_mfa"}) || access.MFAConfigured {
		t.Fatalf("access=%+v", access)
	}
	if activeMentorships("dev-student") != 1 {
		t.Fatal("coordinator was not linked to the seeded student")
	}

	// With TOTP configured the same request shape activates immediately.
	if _, err := fixture.pool.Exec(ctx, `UPDATE app.user_security SET mfa_configured=true WHERE user_id=$1`, studentID); err != nil {
		t.Fatal(err)
	}
	access = putCanaryAccess(t, handler, `{"globalRoles":["system_admin","system_admin"],"seasonRole":"student"}`)
	if len(access.GlobalRoles) != 1 || access.GlobalRoles[0] != (dal.CanaryRoleState{Role: "system_admin", State: "active"}) {
		t.Fatalf("global roles=%+v", access.GlobalRoles)
	}
	if *access.SeasonRole != (dal.CanaryRoleState{Role: "student", State: "active"}) {
		t.Fatalf("season role=%+v", access.SeasonRole)
	}
	if activeMentorships("dev-student") != 0 || activeMentorships("dev-mentor") != 1 {
		t.Fatal("mentorships were not moved to the student role")
	}

	access = putCanaryAccess(t, handler, `{"globalRoles":[],"seasonRole":"removed"}`)
	if len(access.GlobalRoles) != 0 || *access.SeasonRole != (dal.CanaryRoleState{Role: "removed", State: "kicked"}) {
		t.Fatalf("access=%+v", access)
	}
	if activeMentorships("dev-mentor") != 0 {
		t.Fatal("removal left an active mentorship")
	}

	access = putCanaryAccess(t, handler, `{"globalRoles":["director"],"seasonRole":"mentor"}`)
	if *access.SeasonRole != (dal.CanaryRoleState{Role: "mentor", State: "active"}) || access.GlobalRoles[0] != (dal.CanaryRoleState{Role: "director", State: "active"}) {
		t.Fatalf("rejoin access=%+v", access)
	}

	var audits, removals int
	if err := fixture.pool.QueryRow(ctx, `SELECT count(*) FROM app.audit_events WHERE action='canary.access_changed' AND actor_user_id=$1`, studentID).Scan(&audits); err != nil {
		t.Fatal(err)
	}
	if err := fixture.pool.QueryRow(ctx, `SELECT count(*) FROM app.enrollment_removal_events WHERE subject_user_id=$1`, studentID).Scan(&removals); err != nil {
		t.Fatal(err)
	}
	if audits != 4 || removals != 1 {
		t.Fatalf("audits=%d removals=%d", audits, removals)
	}

	response := testRequest(t, handler, http.MethodGet, "/api/v2/canary/access", "student", "")
	if response.Code != http.StatusOK {
		t.Fatalf("get status=%d body=%s", response.Code, response.Body)
	}
}

func TestCanaryAccessRejectsInvalidRoles(t *testing.T) {
	fixture := newPostgresFixture(t)
	handler := newCanaryHandler(t, fixture, true)
	for _, body := range []string{
		`{"globalRoles":["owner"],"seasonRole":"student"}`,
		`{"globalRoles":[],"seasonRole":"director"}`,
		`{"globalRoles":[],"seasonRole":"student","userId":"` + otherID + `"}`,
	} {
		if response := testRequest(t, handler, http.MethodPut, "/api/v2/canary/access", "student", body); response.Code != http.StatusBadRequest {
			t.Fatalf("body=%s status=%d", body, response.Code)
		}
	}
}
