//go:build integration

package migration

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/testcontainers/testcontainers-go"
	postgrescontainer "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

// auth0ImportDatabase returns a superuser DSN for an empty database. It is a
// variable so a local harness can point the test at an existing server.
var auth0ImportDatabase = func(t *testing.T, ctx context.Context) string {
	t.Helper()
	container, err := postgrescontainer.Run(ctx, "postgres:18.6-alpine3.24",
		postgrescontainer.WithDatabase("rsp"),
		postgrescontainer.WithUsername("rsp"),
		postgrescontainer.WithPassword("rsp-auth0-import-integration-password"),
		testcontainers.WithWaitStrategy(wait.ForLog("database system is ready to accept connections").WithOccurrence(2).WithStartupTimeout(90*time.Second)),
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = testcontainers.TerminateContainer(container) })
	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	return dsn
}

const auth0ImportRunID = "01900000-0000-7000-8000-000000000001"

type auth0ImportSeedUser struct {
	id, legacyID, email, name, state string
}

var auth0ImportSeed = []auth0ImportSeedUser{
	{"01900000-0000-7000-8000-0000000000a1", "legacy-both", "both@example.test", "Both Methods", "active"},
	{"01900000-0000-7000-8000-0000000000a2", "legacy-google", "google@example.test", "Google Only", "active"},
	{"01900000-0000-7000-8000-0000000000a3", "legacy-password", "password@example.test", "Password Only", "suspended"},
	{"01900000-0000-7000-8000-0000000000a4", "legacy-unverified", "unverified@example.test", "Not Verified", "active"},
	// Real deleted members have no email; one is kept here so the state guard
	// is exercised on its own rather than masked by the email guard.
	{"01900000-0000-7000-8000-0000000000a5", "legacy-deleted", "deleted@example.test", "Deleted member", "deleted"},
}

func TestAuth0ImportAgainstPostgres(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	adminDSN := auth0ImportDatabase(t, ctx)
	migrationDSN := provisionAuth0ImportTarget(t, ctx, adminDSN)

	target := &Auth0ImportTarget{ConnectionString: migrationDSN, Now: fixedNow}
	candidates, err := target.ExportCandidates(ctx, auth0ImportRunID)
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) != 4 {
		t.Fatalf("deleted users must not be candidates: %+v", candidates)
	}
	for _, candidate := range candidates {
		if candidate.LegacyUserID == "" {
			t.Fatalf("candidate lost its legacy id: %+v", candidate)
		}
	}

	users := []Auth0User{
		{UserID: "auth0|both", Email: "both@example.test", EmailVerified: true, LegacyUserID: "legacy-both",
			Identities: []Auth0Identity{{Provider: "auth0", ProviderAccountID: "both"}, {Provider: "google-oauth2", ProviderAccountID: "111"}}},
		{UserID: "google-oauth2|222", Email: "Google@Example.test", EmailVerified: true, LegacyUserID: "legacy-google",
			Identities: []Auth0Identity{{Provider: "google-oauth2", ProviderAccountID: "222"}}},
		{UserID: "auth0|password", Email: "password@example.test", EmailVerified: true, LegacyUserID: "legacy-password",
			Identities: []Auth0Identity{{Provider: "auth0", ProviderAccountID: "password"}}},
		{UserID: "auth0|unverified", Email: "unverified@example.test", LegacyUserID: "legacy-unverified",
			Identities: []Auth0Identity{{Provider: "auth0", ProviderAccountID: "unverified"}}},
		{UserID: "auth0|deleted", Email: "deleted@example.test", EmailVerified: true, LegacyUserID: "legacy-deleted",
			Identities: []Auth0Identity{{Provider: "auth0", ProviderAccountID: "deleted"}}},
	}
	proposal := ProposeAuth0Resolutions(users, candidates)
	if len(proposal.Resolutions) != 3 || proposal.Counts[ProposalEmailUnverified] != 1 || proposal.Counts[ProposalLegacyUserNotFound] != 1 {
		t.Fatalf("proposal = %+v", proposal)
	}
	// A reviewer approves the drafts and resolves the unverified member by hand.
	resolutions := append(proposal.Resolutions, IdentityResolution{Auth0UserID: "auth0|unverified", AppUserID: auth0ImportSeed[3].id})

	// Each guard fails the whole import and leaves no rows behind.
	expectRejected := func(name string, users []Auth0User, candidates []AppIdentityCandidate, resolutions []IdentityResolution, want string) {
		t.Helper()
		plan, err := PlanAuth0Import(users, candidates, resolutions, fixedNow(), nil)
		if err != nil {
			t.Fatal(err)
		}
		groups, err := BuildAuth0ImportGroups(plan, users, true)
		if err != nil {
			t.Fatal(err)
		}
		_, err = target.Apply(ctx, auth0ImportRunID, plan, groups)
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Fatalf("%s: apply error = %v, want %q", name, err, want)
		}
		assertAuth0ImportRowCounts(t, ctx, adminDSN, 0, 0, 0, 0)
	}
	expectRejected("deleted member",
		users,
		append(append([]AppIdentityCandidate(nil), candidates...), AppIdentityCandidate{AppUserID: auth0ImportSeed[4].id}),
		append(append([]IdentityResolution(nil), resolutions...), IdentityResolution{Auth0UserID: "auth0|deleted", AppUserID: auth0ImportSeed[4].id}),
		"cannot receive a sign-in identity")
	renamed := append([]Auth0User(nil), users...)
	renamed[3].Email = "someone-else@example.test"
	expectRejected("email mismatch", renamed, candidates, resolutions, "does not match the app user's contact email")

	// A member who already signed up on the new site, for example during a
	// rehearsal, must not end up with a second identity.
	seedDB := openAuth0ImportDB(t, adminDSN)
	if _, err := seedDB.ExecContext(ctx, `INSERT INTO app.user_auth_links (auth_subject, user_id, provider, provider_account_id) VALUES ('rehearsal-subject', $1, 'better_auth', 'rehearsal-subject')`, auth0ImportSeed[0].id); err != nil {
		t.Fatal(err)
	}
	plan, err := PlanAuth0Import(users, candidates, resolutions, fixedNow(), nil)
	if err != nil {
		t.Fatal(err)
	}
	groups, err := BuildAuth0ImportGroups(plan, users, true)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := target.Apply(ctx, auth0ImportRunID, plan, groups); err == nil || !strings.Contains(err.Error(), "already has an auth link") {
		t.Fatalf("pre-linked member: apply error = %v", err)
	}
	if _, err := seedDB.ExecContext(ctx, `DELETE FROM app.user_auth_links WHERE auth_subject = 'rehearsal-subject'`); err != nil {
		t.Fatal(err)
	}
	assertAuth0ImportRowCounts(t, ctx, adminDSN, 0, 0, 0, 0)

	if err := VerifyAuth0PlanBinding(plan, users, candidates, resolutions); err != nil {
		t.Fatal(err)
	}
	if _, err := BuildAuth0ImportGroups(plan, users, false); !errors.Is(err, ErrAuth0Unresolved) {
		t.Fatalf("unresolved deleted member was not gated: %v", err)
	}
	summary, err := target.Apply(ctx, auth0ImportRunID, plan, groups)
	if err != nil {
		t.Fatal(err)
	}
	if summary != (Auth0ImportSummary{AuthUsers: 4, GoogleAccounts: 2, PasswordUsers: 3, ActiveLinks: 3, PendingLinks: 1, Unresolved: 1}) {
		t.Fatalf("summary = %+v", summary)
	}
	assertAuth0ImportRowCounts(t, ctx, adminDSN, 4, 2, 4, 5)

	db := openAuth0ImportDB(t, adminDSN)
	var credentialRows int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM auth.accounts WHERE provider_id = 'credential'`).Scan(&credentialRows); err != nil {
		t.Fatal(err)
	}
	if credentialRows != 0 {
		t.Fatalf("password users must reset before a credential exists; found %d", credentialRows)
	}
	var googleUser, googleState string
	var googleVersion int
	if err := db.QueryRowContext(ctx, `
SELECT a.email, a.account_state, a.security_version
  FROM auth.accounts acc JOIN auth.users a ON a.id = acc.user_id
 WHERE acc.provider_id = 'google' AND acc.account_id = '222'`).Scan(&googleUser, &googleState, &googleVersion); err != nil {
		t.Fatal(err)
	}
	if googleUser != "google@example.test" || googleState != "active" || googleVersion != 1 {
		t.Fatalf("Google user = %s %s %d", googleUser, googleState, googleVersion)
	}
	var suspendedState string
	if err := db.QueryRowContext(ctx, `SELECT account_state FROM auth.users WHERE email = 'password@example.test'`).Scan(&suspendedState); err != nil {
		t.Fatal(err)
	}
	if suspendedState != "suspended" {
		t.Fatalf("suspended app user imported as %s", suspendedState)
	}
	var pendingActive bool
	var pendingRevoked sql.NullTime
	if err := db.QueryRowContext(ctx, `SELECT active, revoked_at FROM app.user_auth_links WHERE user_id = $1`, auth0ImportSeed[3].id).Scan(&pendingActive, &pendingRevoked); err != nil {
		t.Fatal(err)
	}
	if pendingActive || !pendingRevoked.Valid {
		t.Fatalf("unverified link active=%v revoked=%v", pendingActive, pendingRevoked)
	}
	var unresolvedStatus string
	if err := db.QueryRowContext(ctx, `SELECT status FROM migration.auth0_identity_imports WHERE auth0_user_id = 'auth0|deleted'`).Scan(&unresolvedStatus); err != nil {
		t.Fatal(err)
	}
	if unresolvedStatus != Auth0StatusRequiresResolution {
		t.Fatalf("unresolved bookkeeping status = %s", unresolvedStatus)
	}

	verification, err := target.Verify(ctx, auth0ImportRunID)
	if err != nil {
		t.Fatal(err)
	}
	if problems := verification.Problems(); len(problems) > 0 || verification.AuthUsers != 4 || verification.ImportedAuth0Users != 4 || verification.ExpectedGoogleAccounts != 2 {
		t.Fatalf("verification = %+v problems=%v", verification, problems)
	}

	if _, err := target.Apply(ctx, auth0ImportRunID, plan, groups); !errors.Is(err, ErrAuth0ImportApplied) {
		t.Fatalf("second apply = %v", err)
	}
	assertAuth0ImportRowCounts(t, ctx, adminDSN, 4, 2, 4, 5)

	// Verification catches a broken link after the fact.
	if _, err := db.ExecContext(ctx, `UPDATE app.user_auth_links SET active = false, revoked_at = now() WHERE user_id = $1`, auth0ImportSeed[1].id); err != nil {
		t.Fatal(err)
	}
	verification, err = target.Verify(ctx, auth0ImportRunID)
	if err != nil {
		t.Fatal(err)
	}
	if verification.LinkStateMismatches != 1 {
		t.Fatalf("tampered link was not detected: %+v", verification)
	}
}

func openAuth0ImportDB(t *testing.T, dsn string) *sql.DB {
	t.Helper()
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

// provisionAuth0ImportTarget builds the production role layout: app and
// migration schemas owned by rsp_migration, auth owned by rsp_auth, plus the
// temporary grants documented in docs/auth-cutover.md.
func provisionAuth0ImportTarget(t *testing.T, ctx context.Context, adminDSN string) string {
	t.Helper()
	admin := openAuth0ImportDB(t, adminDSN)
	for _, role := range []string{"rsp_migration", "rsp_app", "rsp_auth"} {
		if _, err := admin.ExecContext(ctx, "CREATE ROLE "+pgx.Identifier{role}.Sanitize()+" NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION"); err != nil {
			t.Fatal(err)
		}
	}
	for _, statement := range []string{
		`DO $$ BEGIN EXECUTE format('GRANT CREATE ON DATABASE %I TO rsp_migration', current_database()); END $$`,
		`GRANT USAGE, CREATE ON SCHEMA public TO rsp_migration`,
		`CREATE SCHEMA auth AUTHORIZATION rsp_auth`,
	} {
		if _, err := admin.ExecContext(ctx, statement); err != nil {
			t.Fatal(err)
		}
	}

	migrationDSN := adminDSN + "&options=-c%20role%3Drsp_migration"
	migrationDB := openAuth0ImportDB(t, migrationDSN)
	if err := goose.SetDialect("postgres"); err != nil {
		t.Fatal(err)
	}
	migrations, err := filepath.Abs("../../../db/migrations")
	if err != nil {
		t.Fatal(err)
	}
	if err := goose.Up(migrationDB, migrations); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"../schema.sql", "../auth0-import-schema.sql"} {
		contents, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := migrationDB.ExecContext(ctx, string(contents)); err != nil {
			t.Fatalf("apply %s: %v", path, err)
		}
	}
	authSchema, err := os.ReadFile("../../../apps/auth/migrations/better-auth.sql")
	if err != nil {
		t.Fatal(err)
	}
	authDB := openAuth0ImportDB(t, adminDSN+"&options=-c%20role%3Drsp_auth")
	if _, err := authDB.ExecContext(ctx, string(authSchema)); err != nil {
		t.Fatalf("apply Better Auth schema: %v", err)
	}
	if _, err := admin.ExecContext(ctx, `
GRANT USAGE ON SCHEMA auth TO rsp_migration;
GRANT SELECT, INSERT ON auth.users, auth.accounts TO rsp_migration`); err != nil {
		t.Fatal(err)
	}

	if _, err := migrationDB.ExecContext(ctx, `
INSERT INTO migration.runs (id, manifest_checksum, source_schema_fingerprint, source_snapshot_at, state, started_at, finished_at)
VALUES ($1, 'manifest-checksum', 'fingerprint', now(), 'verified', now(), now())`, auth0ImportRunID); err != nil {
		t.Fatal(err)
	}
	for index, user := range auth0ImportSeed {
		var email any
		if user.email != "" {
			email = user.email
		}
		var deletedAt any
		if user.state == "deleted" {
			deletedAt = time.Now().UTC()
		}
		for _, statement := range []struct {
			query string
			args  []any
		}{
			{`INSERT INTO app.users (id, account_state, deleted_at) VALUES ($1, $2, $3)`, []any{user.id, user.state, deletedAt}},
			{`INSERT INTO app.user_profiles (user_id, slug, display_name) VALUES ($1, $2, $3)`, []any{user.id, "member-" + string(rune('a'+index)), user.name}},
			{`INSERT INTO app.user_preferences (user_id) VALUES ($1)`, []any{user.id}},
			{`INSERT INTO app.user_contacts (user_id, email) VALUES ($1, $2)`, []any{user.id, email}},
			{`INSERT INTO app.user_security (user_id) VALUES ($1)`, []any{user.id}},
			{`INSERT INTO migration.row_provenance (run_id, source_table, source_id, target_table, target_id, source_checksum, transformed_checksum, transformed_data)
			  VALUES ($1, 'User', $2, 'users', $3, 'source', 'transformed', '{}'::jsonb)`, []any{auth0ImportRunID, user.legacyID, user.id}},
		} {
			if _, err := migrationDB.ExecContext(ctx, statement.query, statement.args...); err != nil {
				t.Fatalf("seed %s: %v", user.legacyID, err)
			}
		}
	}
	return migrationDSN
}

func assertAuth0ImportRowCounts(t *testing.T, ctx context.Context, adminDSN string, authUsers, accounts, links, bookkeeping int) {
	t.Helper()
	db := openAuth0ImportDB(t, adminDSN)
	var gotUsers, gotAccounts, gotLinks, gotBookkeeping int
	if err := db.QueryRowContext(ctx, `
SELECT (SELECT count(*) FROM auth.users),
       (SELECT count(*) FROM auth.accounts),
       (SELECT count(*) FROM app.user_auth_links),
       (SELECT count(*) FROM migration.auth0_identity_imports)`).Scan(&gotUsers, &gotAccounts, &gotLinks, &gotBookkeeping); err != nil {
		t.Fatal(err)
	}
	if gotUsers != authUsers || gotAccounts != accounts || gotLinks != links || gotBookkeeping != bookkeeping {
		t.Fatalf("rows auth.users=%d auth.accounts=%d links=%d bookkeeping=%d, want %d %d %d %d",
			gotUsers, gotAccounts, gotLinks, gotBookkeeping, authUsers, accounts, links, bookkeeping)
	}
}
