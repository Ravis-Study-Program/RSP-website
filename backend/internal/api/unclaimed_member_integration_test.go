//go:build integration

package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/Ravis-Study-Program/RSP-website/backend/internal/platform/id"
)

// A member with no sign-in (the fixture's mentor, other@example.com) is
// found by the auth service and keeps their record when a sign-in is made
// for their address.
func TestUnclaimedMemberGetsTheSignInMadeForTheirEmail(t *testing.T) {
	f := newPostgresFixture(t)
	ctx := context.Background()
	const serviceToken = "unclaimed-test-service-token-00000000"
	t.Setenv("IDENTITY_SERVICE_TOKEN", serviceToken)
	// The student already has a sign-in, so is not unclaimed.
	if _, err := f.pool.Exec(ctx, `INSERT INTO app.user_auth_links(auth_subject,user_id,provider,provider_account_id) VALUES('auth-student',$1,'better_auth','auth-student')`, studentID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.pool.Exec(ctx, `UPDATE app.user_security SET security_version=3 WHERE user_id=$1`, otherID); err != nil {
		t.Fatal(err)
	}

	type lookup struct {
		Found           bool   `json:"found"`
		Name            string `json:"name"`
		SecurityVersion int64  `json:"securityVersion"`
	}
	find := func(email string) lookup {
		t.Helper()
		res := testRequest(t, f.handler, http.MethodPost, "/internal/auth/unclaimed-member", serviceToken, fmt.Sprintf(`{"email":%q}`, email))
		var v lookup
		if err := json.Unmarshal(res.Body.Bytes(), &v); err != nil || res.Code != http.StatusOK {
			t.Fatalf("lookup %s=%d %s", email, res.Code, res.Body)
		}
		return v
	}
	if res := testRequest(t, f.handler, http.MethodPost, "/internal/auth/unclaimed-member", "", `{"email":"other@example.com"}`); res.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated lookup=%d", res.Code)
	}
	if v := find(" Other@Example.com "); !v.Found || v.Name != "Other" || v.SecurityVersion != 3 {
		t.Fatalf("unclaimed lookup=%+v", v)
	}
	for _, email := range []string{"private@example.com", "nobody@example.com"} {
		if v := find(email); v.Found {
			t.Fatalf("%s found=%+v", email, v)
		}
	}
	if _, err := f.pool.Exec(ctx, `UPDATE app.users SET account_state='suspended' WHERE id=$1`, otherID); err != nil {
		t.Fatal(err)
	}
	if v := find("other@example.com"); v.Found {
		t.Fatalf("suspended member found=%+v", v)
	}
	if _, err := f.pool.Exec(ctx, `UPDATE app.users SET account_state='active' WHERE id=$1`, otherID); err != nil {
		t.Fatal(err)
	}

	var membersBefore int
	if err := f.pool.QueryRow(ctx, `SELECT count(*) FROM app.users`).Scan(&membersBefore); err != nil {
		t.Fatal(err)
	}
	event := fmt.Sprintf(`{"eventId":%q,"type":"auth_user_created","authUserId":"auth-other","email":"other@example.com","name":"Other Name","emailVerified":true,"securityVersion":3,"occurredAt":%q}`, id.New(), time.Now().UTC().Format(time.RFC3339Nano))
	if res := testRequest(t, f.handler, http.MethodPost, "/internal/auth/lifecycle-events", serviceToken, event); res.Code != http.StatusNoContent {
		t.Fatalf("created=%d %s", res.Code, res.Body)
	}

	// The sign-in is the existing mentor's: no second member, same name and
	// season, and access tokens carry their security version.
	var linkedTo string
	var active bool
	if err := f.pool.QueryRow(ctx, `SELECT user_id::text,active FROM app.user_auth_links WHERE auth_subject='auth-other'`).Scan(&linkedTo, &active); err != nil || linkedTo != otherID || !active {
		t.Fatalf("link user=%s active=%v err=%v", linkedTo, active, err)
	}
	var membersAfter, version int
	var name string
	if err := f.pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM app.users),(SELECT security_version FROM app.user_security WHERE user_id=$1),(SELECT display_name FROM app.user_profiles WHERE user_id=$1)`, otherID).Scan(&membersAfter, &version, &name); err != nil {
		t.Fatal(err)
	}
	if membersAfter != membersBefore || version != 3 || name != "Other" {
		t.Fatalf("members %d->%d version=%d name=%q", membersBefore, membersAfter, version, name)
	}
	var audits int
	if err := f.pool.QueryRow(ctx, `SELECT count(*) FROM app.audit_events WHERE action='account.sign_in_attached' AND subject_id=$1`, otherID).Scan(&audits); err != nil || audits != 1 {
		t.Fatalf("audits=%d err=%v", audits, err)
	}
	if actor, err := f.db.ResolveAuthSubject(ctx, "auth-other"); err != nil || actor.UserID != otherID || actor.SecurityVersion != 3 {
		t.Fatalf("resolved actor=%+v err=%v", actor, err)
	}
	if v := find("other@example.com"); v.Found {
		t.Fatalf("claimed member still found=%+v", v)
	}
}
