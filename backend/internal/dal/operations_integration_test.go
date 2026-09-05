//go:build integration

package dal

import (
	"context"
	"testing"
	"time"

	"github.com/magedmg/RSP-website/backend/internal/accounts"
	"github.com/magedmg/RSP-website/backend/internal/platform/id"
)

func TestStoreOperations(t *testing.T) {
	db := newPostgresFixture(t)
	ctx := context.Background()

	snapshot, err := db.ObservabilitySnapshot(ctx)
	if err != nil || snapshot.WorkerRuns["success"] != 0 {
		t.Fatalf("empty database snapshot=%#v error=%v", snapshot, err)
	}

	if err := db.ApplyIdentityEvent(ctx, accounts.IdentityEvent{
		EventID: id.New(), Type: "auth_user_created", AuthUserID: "bootstrap-user",
		Email: "admin@example.test", EmailVerified: true, SecurityVersion: 1, OccurredAt: time.Now().UTC(),
	}); err != nil {
		t.Fatal(err)
	}
	if err := db.BootstrapAdmin(ctx, "bootstrap-user", "wrong@example.test"); err == nil {
		t.Fatal("bootstrap accepted an email that does not belong to the linked user")
	}
	if err := db.BootstrapAdmin(ctx, "bootstrap-user", " Admin@Example.Test "); err != nil {
		t.Fatal(err)
	}
	var state, action, auditUserID, userID string
	if err := db.pool.QueryRow(ctx, `SELECT g.state::text,a.action,a.data->>'userId',g.user_id
  FROM app.global_role_assignments g JOIN app.audit_events a ON a.subject_id=g.id::text
  WHERE g.role='system_admin'`).Scan(&state, &action, &auditUserID, &userID); err != nil {
		t.Fatal(err)
	}
	if state != "pending_mfa" || action != "system_admin.bootstrap_pending_mfa" || auditUserID != userID {
		t.Fatalf("bootstrap role=%s action=%s audit user=%s user=%s", state, action, auditUserID, userID)
	}
	if err := db.BootstrapAdmin(ctx, "bootstrap-user", "admin@example.test"); err == nil {
		t.Fatal("bootstrap created a second System Admin")
	}

	// The student is inserted before the missing mentor is detected; both must roll back.
	if err := db.Seed(ctx, SeedOptions{MentorEmail: "missing@example.test"}); err == nil {
		t.Fatal("seed accepted a missing linked mentor")
	}
	var partialUsers int
	if err := db.pool.QueryRow(ctx, `SELECT count(*) FROM app.users u JOIN app.user_profiles p ON p.user_id=u.id WHERE p.slug='dev-student'`).Scan(&partialUsers); err != nil || partialUsers != 0 {
		t.Fatalf("failed seed left %d users: %v", partialUsers, err)
	}
	if err := db.Seed(ctx, SeedOptions{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Seed(ctx, SeedOptions{}); err == nil {
		t.Fatal("duplicate seed accepted without allow-existing")
	}
	if err := db.Seed(ctx, SeedOptions{AllowExisting: true}); err != nil {
		t.Fatal(err)
	}
	var seasons int
	if err := db.pool.QueryRow(ctx, `SELECT count(*) FROM app.seasons WHERE slug='dev-season'`).Scan(&seasons); err != nil || seasons != 1 {
		t.Fatalf("seed seasons=%d error=%v", seasons, err)
	}
}
