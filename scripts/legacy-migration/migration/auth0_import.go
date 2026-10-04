package migration

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

var (
	ErrAuth0PlanDrift       = errors.New("Auth0 import inputs do not reproduce the approved plan checksum")
	ErrAuth0Unresolved      = errors.New("Auth0 plan contains unresolved identities")
	ErrAuth0ImportApplied   = errors.New("Auth0 identities have already been imported for this run")
	ErrAuth0ImportInvariant = errors.New("Auth0 import invariant failed")
)

// Auth0ImportGroup is one Better Auth user. Several Auth0 users that were
// explicitly resolved to the same app user become one Better Auth user,
// because Better Auth allows one email and one account per provider per user.
type Auth0ImportGroup struct {
	AppUserID       string
	Email           string
	EmailVerified   bool
	GoogleAccountID string
	HasPassword     bool
	Auth0UserIDs    []string
}

// VerifyAuth0PlanBinding proves the approved plan file is intact and that the
// current inputs reproduce it exactly. CreatedAt is part of the checksum, so
// the plan is recomputed at its original timestamp.
func VerifyAuth0PlanBinding(plan Auth0ImportPlan, users []Auth0User, candidates []AppIdentityCandidate, resolutions []IdentityResolution) error {
	unsigned := plan
	unsigned.Checksum = ""
	selfChecksum, err := Checksum(unsigned)
	if err != nil {
		return err
	}
	if plan.Checksum == "" || selfChecksum != plan.Checksum {
		return fmt.Errorf("%w: plan file was modified after it was generated", ErrAuth0PlanDrift)
	}
	recomputed, err := PlanAuth0Import(users, candidates, resolutions, plan.CreatedAt, nil)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrAuth0PlanDrift, err)
	}
	if recomputed.Checksum != plan.Checksum {
		return fmt.Errorf("%w: approved %s, inputs produce %s", ErrAuth0PlanDrift, plan.Checksum, recomputed.Checksum)
	}
	return nil
}

// BuildAuth0ImportGroups turns resolved plan items into Better Auth users.
// It refuses anything this importer cannot represent faithfully.
func BuildAuth0ImportGroups(plan Auth0ImportPlan, users []Auth0User, allowUnresolved bool) ([]Auth0ImportGroup, error) {
	usersByID := make(map[string]Auth0User, len(users))
	for _, user := range users {
		usersByID[user.UserID] = user
	}
	if unresolved := plan.Reconciliation.UnresolvedAuth0UserIDs; len(unresolved) > 0 && !allowUnresolved {
		return nil, fmt.Errorf("%w: %d (pass --allow-unresolved to leave them unimported)", ErrAuth0Unresolved, len(unresolved))
	}

	groups := map[string]*Auth0ImportGroup{}
	for _, item := range plan.Items {
		if item.Status == Auth0StatusRequiresResolution {
			continue
		}
		if item.Status != Auth0StatusMatched && item.Status != Auth0StatusPasswordResetRequired {
			return nil, fmt.Errorf("Auth0 user %s has unexpected plan status %q", item.Auth0UserID, item.Status)
		}
		if item.PasswordHashCompatible {
			return nil, fmt.Errorf("Auth0 user %s carries a password hash; this importer only supports password resets", item.Auth0UserID)
		}
		user, ok := usersByID[item.Auth0UserID]
		if !ok {
			return nil, fmt.Errorf("Auth0 user %s is in the plan but not the users file", item.Auth0UserID)
		}
		email := normalizeEmail(user.Email)
		if email == "" {
			return nil, fmt.Errorf("Auth0 user %s has no email", item.Auth0UserID)
		}
		group := groups[item.ResolvedAppUserID]
		if group == nil {
			group = &Auth0ImportGroup{AppUserID: item.ResolvedAppUserID, Email: email}
			groups[item.ResolvedAppUserID] = group
		}
		if group.Email != email {
			return nil, fmt.Errorf("app user %s is resolved from Auth0 users with different emails", item.ResolvedAppUserID)
		}
		group.EmailVerified = group.EmailVerified || item.EmailVerified
		group.Auth0UserIDs = append(group.Auth0UserIDs, item.Auth0UserID)
		for _, identity := range item.Identities {
			switch identity.Provider {
			case Auth0ProviderGoogle:
				if group.GoogleAccountID != "" && group.GoogleAccountID != identity.ProviderAccountID {
					return nil, fmt.Errorf("app user %s is resolved from two different Google accounts", item.ResolvedAppUserID)
				}
				group.GoogleAccountID = identity.ProviderAccountID
			case Auth0ProviderDatabase:
				group.HasPassword = true
			default:
				return nil, fmt.Errorf("Auth0 user %s has unsupported provider %q", item.Auth0UserID, identity.Provider)
			}
		}
	}

	result := make([]Auth0ImportGroup, 0, len(groups))
	for _, group := range groups {
		sort.Strings(group.Auth0UserIDs)
		result = append(result, *group)
	}
	sort.Slice(result, func(left, right int) bool { return result[left].AppUserID < result[right].AppUserID })
	return result, nil
}

type Auth0ImportSummary struct {
	AuthUsers      int `json:"authUsers"`
	GoogleAccounts int `json:"googleAccounts"`
	PasswordUsers  int `json:"passwordUsers"`
	ActiveLinks    int `json:"activeLinks"`
	PendingLinks   int `json:"pendingLinks"`
	Unresolved     int `json:"unresolved"`
}

type Auth0ImportVerification struct {
	PlanChecksum            string `json:"planChecksum"`
	ImportedAuth0Users      int    `json:"importedAuth0Users"`
	AuthUsers               int    `json:"authUsers"`
	ActiveLinks             int    `json:"activeLinks"`
	PendingLinks            int    `json:"pendingLinks"`
	ExpectedGoogleAccounts  int    `json:"expectedGoogleAccounts"`
	MissingAuthUsers        int    `json:"missingAuthUsers"`
	MissingLinks            int    `json:"missingLinks"`
	MislinkedSubjects       int    `json:"mislinkedSubjects"`
	LinkStateMismatches     int    `json:"linkStateMismatches"`
	AppUsersWithManyLinks   int    `json:"appUsersWithManyLinks"`
	MissingGoogleAccounts   int    `json:"missingGoogleAccounts"`
	SecurityVersionMismatch int    `json:"securityVersionMismatches"`
}

func (verification Auth0ImportVerification) Problems() []string {
	checks := []struct {
		name  string
		count int
	}{
		{"auth users missing", verification.MissingAuthUsers},
		{"app links missing", verification.MissingLinks},
		{"auth subjects linked to the wrong app user", verification.MislinkedSubjects},
		{"link active state differs from email verification", verification.LinkStateMismatches},
		{"app users with more than one auth link", verification.AppUsersWithManyLinks},
		{"Google accounts missing", verification.MissingGoogleAccounts},
		{"security versions differ", verification.SecurityVersionMismatch},
	}
	problems := make([]string, 0)
	for _, check := range checks {
		if check.count != 0 {
			problems = append(problems, fmt.Sprintf("%s: %d", check.name, check.count))
		}
	}
	return problems
}

// Auth0ImportTarget writes Better Auth users, Google accounts and app links in
// one serializable transaction. The connecting role needs the app and
// migration schemas plus SELECT, INSERT on auth.users and auth.accounts.
type Auth0ImportTarget struct {
	ConnectionString string
	Now              func() time.Time
	Random           io.Reader
}

func (target *Auth0ImportTarget) now() time.Time {
	if target.Now != nil {
		return target.Now().UTC()
	}
	return time.Now().UTC()
}

// ExportCandidates lists importable app users with their legacy UserId from
// the data migration's provenance. Deleted users are excluded on purpose.
func (target *Auth0ImportTarget) ExportCandidates(ctx context.Context, runID string) ([]AppIdentityCandidate, error) {
	conn, err := connectMigration(ctx, target.ConnectionString)
	if err != nil {
		return nil, fmt.Errorf("connect target: %w", err)
	}
	defer conn.Close(context.Background())
	if err := requireVerifiedRun(ctx, conn, runID); err != nil {
		return nil, err
	}
	rows, err := conn.Query(ctx, `
SELECT u.id::text, coalesce(c.email, ''), coalesce(p.source_id, '')
  FROM app.users u
  LEFT JOIN app.user_contacts c ON c.user_id = u.id
  LEFT JOIN migration.row_provenance p
    ON p.run_id = $1 AND p.source_table = 'User'
   AND p.target_table = 'users' AND p.target_id = u.id::text
 WHERE u.account_state IN ('active', 'suspended')
 ORDER BY u.id`, runID)
	if err != nil {
		return nil, err
	}
	candidates, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (AppIdentityCandidate, error) {
		var candidate AppIdentityCandidate
		err := row.Scan(&candidate.AppUserID, &candidate.Email, &candidate.LegacyUserID)
		return candidate, err
	})
	if err != nil {
		return nil, err
	}
	return candidates, nil
}

func requireVerifiedRun(ctx context.Context, query interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, runID string) error {
	var state string
	err := query.QueryRow(ctx, `SELECT state::text FROM migration.runs WHERE id = $1`, runID).Scan(&state)
	if errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("%w: %s", ErrRunNotFound, runID)
	}
	if err != nil {
		return err
	}
	if state != "verified" {
		return fmt.Errorf("data migration run %s is %s; identities import only onto a verified run", runID, state)
	}
	return nil
}

// Apply imports every group and records bookkeeping for every plan item. It
// verifies the result inside the same transaction, so a failed invariant
// leaves nothing behind.
func (target *Auth0ImportTarget) Apply(ctx context.Context, runID string, plan Auth0ImportPlan, groups []Auth0ImportGroup) (Auth0ImportSummary, error) {
	conn, err := connectMigration(ctx, target.ConnectionString)
	if err != nil {
		return Auth0ImportSummary{}, fmt.Errorf("connect target: %w", err)
	}
	defer conn.Close(context.Background())
	tx, err := conn.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return Auth0ImportSummary{}, err
	}
	defer tx.Rollback(context.Background())

	if _, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock($1)", AdvisoryLockKey); err != nil {
		return Auth0ImportSummary{}, err
	}
	var bookkeepingReady, authReady bool
	if err := tx.QueryRow(ctx, `SELECT to_regclass('migration.auth0_imports') IS NOT NULL, to_regclass('auth.users') IS NOT NULL`).Scan(&bookkeepingReady, &authReady); err != nil {
		return Auth0ImportSummary{}, err
	}
	if !bookkeepingReady {
		return Auth0ImportSummary{}, errors.New("migration.auth0_imports is missing; apply scripts/legacy-migration/auth0-import-schema.sql first")
	}
	if !authReady {
		return Auth0ImportSummary{}, errors.New("auth.users is missing; apply deploy/postgres/migrate-auth.sql first")
	}
	if err := requireVerifiedRun(ctx, tx, runID); err != nil {
		return Auth0ImportSummary{}, err
	}
	var applied bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM migration.auth0_imports WHERE run_id = $1 OR plan_checksum = $2)`, runID, plan.Checksum).Scan(&applied); err != nil {
		return Auth0ImportSummary{}, err
	}
	if applied {
		return Auth0ImportSummary{}, ErrAuth0ImportApplied
	}

	now := target.now()
	summary := Auth0ImportSummary{Unresolved: len(plan.Reconciliation.UnresolvedAuth0UserIDs)}
	authUserByAuth0 := map[string]string{}
	for _, group := range groups {
		authUserID, err := target.importGroup(ctx, tx, group, now)
		if err != nil {
			return Auth0ImportSummary{}, fmt.Errorf("app user %s: %w", group.AppUserID, err)
		}
		for _, auth0UserID := range group.Auth0UserIDs {
			authUserByAuth0[auth0UserID] = authUserID
		}
		summary.AuthUsers++
		if group.GoogleAccountID != "" {
			summary.GoogleAccounts++
		}
		if group.HasPassword {
			summary.PasswordUsers++
		}
		if group.EmailVerified {
			summary.ActiveLinks++
		} else {
			summary.PendingLinks++
		}
	}

	if _, err := tx.Exec(ctx, `
INSERT INTO migration.auth0_imports (
  run_id, plan_checksum, auth_user_count, google_account_count,
  password_reset_count, unresolved_count, applied_at
) VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		runID, plan.Checksum, summary.AuthUsers, summary.GoogleAccounts,
		summary.PasswordUsers, summary.Unresolved, now); err != nil {
		return Auth0ImportSummary{}, fmt.Errorf("record Auth0 import: %w", err)
	}
	for _, item := range plan.Items {
		if err := recordAuth0Item(ctx, tx, runID, item, authUserByAuth0[item.Auth0UserID]); err != nil {
			return Auth0ImportSummary{}, err
		}
	}

	verification, err := verifyAuth0Import(ctx, tx, runID)
	if err != nil {
		return Auth0ImportSummary{}, err
	}
	if problems := verification.Problems(); len(problems) > 0 {
		return Auth0ImportSummary{}, fmt.Errorf("%w: %s", ErrAuth0ImportInvariant, strings.Join(problems, "; "))
	}
	if verification.AuthUsers != summary.AuthUsers || verification.ExpectedGoogleAccounts != summary.GoogleAccounts {
		return Auth0ImportSummary{}, fmt.Errorf("%w: verified %d auth users and %d Google accounts, wrote %d and %d",
			ErrAuth0ImportInvariant, verification.AuthUsers, verification.ExpectedGoogleAccounts, summary.AuthUsers, summary.GoogleAccounts)
	}
	if err := tx.Commit(ctx); err != nil {
		return Auth0ImportSummary{}, err
	}
	return summary, nil
}

func (target *Auth0ImportTarget) importGroup(ctx context.Context, tx pgx.Tx, group Auth0ImportGroup, now time.Time) (string, error) {
	var state, contactEmail, displayName string
	var securityVersion int64
	err := tx.QueryRow(ctx, `
SELECT u.account_state::text, coalesce(c.email, ''), p.display_name, s.security_version
  FROM app.users u
  JOIN app.user_profiles p ON p.user_id = u.id
  JOIN app.user_security s ON s.user_id = u.id
  LEFT JOIN app.user_contacts c ON c.user_id = u.id
 WHERE u.id = $1
   FOR UPDATE OF u`, group.AppUserID).Scan(&state, &contactEmail, &displayName, &securityVersion)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", errors.New("app user does not exist")
	}
	if err != nil {
		return "", err
	}
	if state != "active" && state != "suspended" {
		return "", fmt.Errorf("app user is %s and cannot receive a sign-in identity", state)
	}
	if normalizeEmail(contactEmail) != group.Email {
		return "", errors.New("Auth0 email does not match the app user's contact email")
	}
	var linked, emailTaken bool
	if err := tx.QueryRow(ctx, `
SELECT EXISTS (SELECT 1 FROM app.user_auth_links WHERE user_id = $1),
       EXISTS (SELECT 1 FROM auth.users WHERE lower(email) = $2)`,
		group.AppUserID, group.Email).Scan(&linked, &emailTaken); err != nil {
		return "", err
	}
	if linked {
		return "", errors.New("app user already has an auth link")
	}
	if emailTaken {
		return "", errors.New("an auth user with this email already exists")
	}

	authUserID, err := betterAuthID(target.Random)
	if err != nil {
		return "", err
	}
	if _, err := tx.Exec(ctx, `
INSERT INTO auth.users (
  id, name, email, email_verified, image, created_at, updated_at,
  two_factor_enabled, account_state, security_version
) VALUES ($1, $2, $3, $4, NULL, $5, $5, false, $6, $7)`,
		authUserID, displayName, group.Email, group.EmailVerified, now, state, securityVersion); err != nil {
		return "", fmt.Errorf("insert auth user: %w", err)
	}
	if group.GoogleAccountID != "" {
		accountID, err := betterAuthID(target.Random)
		if err != nil {
			return "", err
		}
		// Imported Google rows hold no tokens. Better Auth finds the user by
		// (provider_id, account_id) and stores fresh tokens on first sign-in.
		if _, err := tx.Exec(ctx, `
INSERT INTO auth.accounts (id, account_id, provider_id, user_id, created_at, updated_at)
VALUES ($1, $2, 'google', $3, $4, $4)`, accountID, group.GoogleAccountID, authUserID, now); err != nil {
			return "", fmt.Errorf("insert Google account: %w", err)
		}
	}
	// Mirrors createIdentityUser in backend/internal/dal/identity.go: one
	// better_auth subject per app user, inactive until the email is verified.
	var revokedAt *time.Time
	if !group.EmailVerified {
		revokedAt = &now
	}
	if _, err := tx.Exec(ctx, `
INSERT INTO app.user_auth_links (auth_subject, user_id, provider, provider_account_id, active, linked_at, revoked_at)
VALUES ($1, $2, 'better_auth', $1, $3, $4, $5)`,
		authUserID, group.AppUserID, group.EmailVerified, now, revokedAt); err != nil {
		return "", fmt.Errorf("insert app link: %w", err)
	}
	return authUserID, nil
}

func recordAuth0Item(ctx context.Context, tx pgx.Tx, runID string, item Auth0ImportItem, authUserID string) error {
	identities, err := json.Marshal(item.Identities)
	if err != nil {
		return err
	}
	status, detail := item.Status, item.Detail
	var appUserID, provider, providerAccountID, authUser any
	if authUserID != "" {
		status, appUserID, authUser = Auth0StatusImported, item.ResolvedAppUserID, authUserID
		if item.RequiresPasswordReset {
			detail = "imported; database password requires a verified reset"
		} else {
			detail = "imported"
		}
	}
	if len(item.Identities) == 1 {
		provider, providerAccountID = item.Identities[0].Provider, item.Identities[0].ProviderAccountID
	}
	if _, err := tx.Exec(ctx, `
INSERT INTO migration.auth0_identity_imports (
  run_id, auth0_user_id, app_user_id, provider, provider_account_id,
  email_verified, password_hash_compatible, status, detail, auth_user_id, identities
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
		runID, item.Auth0UserID, appUserID, provider, providerAccountID,
		item.EmailVerified, item.PasswordHashCompatible, status, detail, authUser, identities); err != nil {
		return fmt.Errorf("record Auth0 user %s: %w", item.Auth0UserID, err)
	}
	return nil
}

// Verify re-checks a committed import. Run it straight after apply; later
// sign-in activity legitimately changes verification and security state.
func (target *Auth0ImportTarget) Verify(ctx context.Context, runID string) (Auth0ImportVerification, error) {
	conn, err := connectMigration(ctx, target.ConnectionString)
	if err != nil {
		return Auth0ImportVerification{}, fmt.Errorf("connect target: %w", err)
	}
	defer conn.Close(context.Background())
	tx, err := conn.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return Auth0ImportVerification{}, err
	}
	defer tx.Rollback(context.Background())
	return verifyAuth0Import(ctx, tx, runID)
}

func verifyAuth0Import(ctx context.Context, tx pgx.Tx, runID string) (Auth0ImportVerification, error) {
	var verification Auth0ImportVerification
	err := tx.QueryRow(ctx, `SELECT plan_checksum FROM migration.auth0_imports WHERE run_id = $1`, runID).Scan(&verification.PlanChecksum)
	if errors.Is(err, pgx.ErrNoRows) {
		return verification, fmt.Errorf("no Auth0 import recorded for run %s", runID)
	}
	if err != nil {
		return verification, err
	}
	err = tx.QueryRow(ctx, `
WITH imported AS (
  SELECT DISTINCT auth_user_id, app_user_id
    FROM migration.auth0_identity_imports
   WHERE run_id = $1 AND status = 'imported'
)
SELECT
  (SELECT count(*) FROM migration.auth0_identity_imports WHERE run_id = $1 AND status = 'imported'),
  count(*),
  count(*) FILTER (WHERE l.active),
  count(*) FILTER (WHERE NOT l.active),
  count(*) FILTER (WHERE a.id IS NULL),
  count(*) FILTER (WHERE l.auth_subject IS NULL),
  count(*) FILTER (WHERE l.user_id IS DISTINCT FROM i.app_user_id),
  count(*) FILTER (WHERE l.active IS DISTINCT FROM a.email_verified),
  count(*) FILTER (WHERE a.security_version IS DISTINCT FROM s.security_version::integer),
  (SELECT count(*) FROM (
     SELECT l2.user_id FROM app.user_auth_links l2
      WHERE l2.user_id IN (SELECT app_user_id FROM imported)
      GROUP BY l2.user_id HAVING count(*) > 1) many)
  FROM imported i
  LEFT JOIN auth.users a ON a.id = i.auth_user_id
  LEFT JOIN app.user_auth_links l ON l.auth_subject = i.auth_user_id
  LEFT JOIN app.user_security s ON s.user_id = i.app_user_id`, runID).Scan(
		&verification.ImportedAuth0Users, &verification.AuthUsers,
		&verification.ActiveLinks, &verification.PendingLinks,
		&verification.MissingAuthUsers, &verification.MissingLinks,
		&verification.MislinkedSubjects, &verification.LinkStateMismatches,
		&verification.SecurityVersionMismatch, &verification.AppUsersWithManyLinks)
	if err != nil {
		return verification, err
	}
	err = tx.QueryRow(ctx, `
WITH expected AS (
  SELECT DISTINCT i.auth_user_id, identity->>'providerAccountId' AS google_id
    FROM migration.auth0_identity_imports i
   CROSS JOIN LATERAL jsonb_array_elements(i.identities) identity
   WHERE i.run_id = $1 AND i.status = 'imported'
     AND identity->>'provider' = 'google-oauth2'
)
SELECT count(*), count(*) FILTER (WHERE a.id IS NULL)
  FROM expected e
  LEFT JOIN auth.accounts a
    ON a.provider_id = 'google' AND a.account_id = e.google_id AND a.user_id = e.auth_user_id`, runID).Scan(
		&verification.ExpectedGoogleAccounts, &verification.MissingGoogleAccounts)
	return verification, err
}

const betterAuthIDAlphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

// betterAuthID matches Better Auth's default generator: 32 characters from
// [a-zA-Z0-9], drawn without modulo bias.
func betterAuthID(source io.Reader) (string, error) {
	if source == nil {
		source = rand.Reader
	}
	const size = 32
	const limit = 256 - 256%len(betterAuthIDAlphabet)
	output := make([]byte, 0, size)
	buffer := make([]byte, size)
	for len(output) < size {
		if _, err := io.ReadFull(source, buffer); err != nil {
			return "", fmt.Errorf("generate auth id: %w", err)
		}
		for _, value := range buffer {
			if int(value) < limit && len(output) < size {
				output = append(output, betterAuthIDAlphabet[int(value)%len(betterAuthIDAlphabet)])
			}
		}
	}
	return string(output), nil
}
