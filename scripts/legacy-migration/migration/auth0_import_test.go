package migration

import (
	"bytes"
	"errors"
	"regexp"
	"strings"
	"testing"
)

func importFixture(t *testing.T) ([]Auth0User, []AppIdentityCandidate, []IdentityResolution) {
	t.Helper()
	users := []Auth0User{
		{UserID: "google-oauth2|g1", Email: "both@example.test", EmailVerified: true, Identities: []Auth0Identity{{Provider: "google-oauth2", ProviderAccountID: "g1"}}},
		{UserID: "auth0|p1", Email: "BOTH@example.test", EmailVerified: false, Identities: []Auth0Identity{{Provider: "auth0", ProviderAccountID: "p1"}}},
		{UserID: "auth0|p2", Email: "solo@example.test", EmailVerified: true, Identities: []Auth0Identity{{Provider: "auth0", ProviderAccountID: "p2"}}},
		{UserID: "auth0|stray", Email: "stray@example.test", EmailVerified: true, Identities: []Auth0Identity{{Provider: "auth0", ProviderAccountID: "stray"}}},
	}
	candidates := []AppIdentityCandidate{
		{AppUserID: "app-both", Email: "both@example.test"},
		{AppUserID: "app-solo", Email: "solo@example.test"},
	}
	resolutions := []IdentityResolution{
		{Auth0UserID: "google-oauth2|g1", AppUserID: "app-both"},
		{Auth0UserID: "auth0|p1", AppUserID: "app-both"},
		{Auth0UserID: "auth0|p2", AppUserID: "app-solo"},
	}
	return users, candidates, resolutions
}

func TestAuth0ImportGroupsMergeIdentitiesPerAppUser(t *testing.T) {
	users, candidates, resolutions := importFixture(t)
	plan, err := PlanAuth0Import(users, candidates, resolutions, fixedNow(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := BuildAuth0ImportGroups(plan, users, false); !errors.Is(err, ErrAuth0Unresolved) {
		t.Fatalf("unresolved identities were not gated: %v", err)
	}
	groups, err := BuildAuth0ImportGroups(plan, users, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(groups) != 2 {
		t.Fatalf("groups = %+v", groups)
	}
	both := groups[0]
	if both.AppUserID != "app-both" || both.Email != "both@example.test" || !both.EmailVerified ||
		both.GoogleAccountID != "g1" || !both.HasPassword || strings.Join(both.Auth0UserIDs, ",") != "auth0|p1,google-oauth2|g1" {
		t.Fatalf("merged group = %+v", both)
	}
	if solo := groups[1]; solo.GoogleAccountID != "" || !solo.HasPassword || !solo.EmailVerified {
		t.Fatalf("password-only group = %+v", solo)
	}
}

func TestAuth0ImportGroupsRejectUnrepresentableIdentities(t *testing.T) {
	users := []Auth0User{
		{UserID: "google-oauth2|a", Email: "x@example.test", EmailVerified: true, Identities: []Auth0Identity{{Provider: "google-oauth2", ProviderAccountID: "a"}}},
		{UserID: "google-oauth2|b", Email: "x@example.test", EmailVerified: true, Identities: []Auth0Identity{{Provider: "google-oauth2", ProviderAccountID: "b"}}},
		{UserID: "auth0|y", Email: "y@example.test", EmailVerified: true, Identities: []Auth0Identity{{Provider: "auth0", ProviderAccountID: "y"}}},
	}
	candidates := []AppIdentityCandidate{{AppUserID: "app-x"}, {AppUserID: "app-y"}}

	twoGoogle := []IdentityResolution{{Auth0UserID: "google-oauth2|a", AppUserID: "app-x"}, {Auth0UserID: "google-oauth2|b", AppUserID: "app-x"}}
	plan, err := PlanAuth0Import(users[:2], candidates, twoGoogle, fixedNow(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := BuildAuth0ImportGroups(plan, users, true); err == nil || !strings.Contains(err.Error(), "two different Google accounts") {
		t.Fatalf("two Google accounts on one app user = %v", err)
	}

	differentEmails := []IdentityResolution{{Auth0UserID: "google-oauth2|a", AppUserID: "app-x"}, {Auth0UserID: "auth0|y", AppUserID: "app-x"}}
	plan, err = PlanAuth0Import([]Auth0User{users[0], users[2]}, candidates, differentEmails, fixedNow(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := BuildAuth0ImportGroups(plan, users, true); err == nil || !strings.Contains(err.Error(), "different emails") {
		t.Fatalf("different emails on one app user = %v", err)
	}

	hashed := []Auth0User{{UserID: "auth0|y", Email: "y@example.test", EmailVerified: true, PasswordHash: "$2b$10$fixture", Identities: []Auth0Identity{{Provider: "auth0", ProviderAccountID: "y"}}}}
	plan, err = PlanAuth0Import(hashed, candidates, []IdentityResolution{{Auth0UserID: "auth0|y", AppUserID: "app-y"}}, fixedNow(), func(string) bool { return true })
	if err != nil {
		t.Fatal(err)
	}
	if _, err := BuildAuth0ImportGroups(plan, hashed, true); err == nil || !strings.Contains(err.Error(), "only supports password resets") {
		t.Fatalf("compatible hash = %v", err)
	}
}

func TestAuth0PlanBindingDetectsEditedPlanAndDriftedInputs(t *testing.T) {
	users, candidates, resolutions := importFixture(t)
	plan, err := PlanAuth0Import(users, candidates, resolutions, fixedNow(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := VerifyAuth0PlanBinding(plan, users, candidates, resolutions); err != nil {
		t.Fatalf("intact plan rejected: %v", err)
	}

	edited := plan
	edited.Items = append([]Auth0ImportItem(nil), plan.Items...)
	edited.Items[0].ResolvedAppUserID = "app-solo"
	if err := VerifyAuth0PlanBinding(edited, users, candidates, resolutions); !errors.Is(err, ErrAuth0PlanDrift) {
		t.Fatalf("edited plan accepted: %v", err)
	}

	drifted := append([]IdentityResolution(nil), resolutions...)
	drifted = append(drifted, IdentityResolution{Auth0UserID: "auth0|stray", AppUserID: "app-solo"})
	if err := VerifyAuth0PlanBinding(plan, users, candidates, drifted); !errors.Is(err, ErrAuth0PlanDrift) {
		t.Fatalf("drifted resolutions accepted: %v", err)
	}
}

func TestAuth0PlanNeverImportsBlockedUsers(t *testing.T) {
	users := []Auth0User{{UserID: "auth0|b", Email: "b@example.test", EmailVerified: true, Blocked: true, Identities: []Auth0Identity{{Provider: "auth0", ProviderAccountID: "b"}}}}
	candidates := []AppIdentityCandidate{{AppUserID: "app-b", Email: "b@example.test"}}
	if _, err := PlanAuth0Import(users, candidates, []IdentityResolution{{Auth0UserID: "auth0|b", AppUserID: "app-b"}}, fixedNow(), nil); err == nil {
		t.Fatal("resolution for a blocked Auth0 user was accepted")
	}
	plan, err := PlanAuth0Import(users, candidates, nil, fixedNow(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if item := plan.Items[0]; item.Status != Auth0StatusRequiresResolution || !item.Blocked || len(item.CandidateAppUserIDs) != 1 {
		t.Fatalf("blocked item = %+v", item)
	}
}

func TestBetterAuthIDMatchesDefaultGeneratorShape(t *testing.T) {
	pattern := regexp.MustCompile(`^[a-zA-Z0-9]{32}$`)
	for range 50 {
		id, err := betterAuthID(nil)
		if err != nil {
			t.Fatal(err)
		}
		if !pattern.MatchString(id) {
			t.Fatalf("id %q does not match Better Auth's default shape", id)
		}
	}
	// Bytes at or above the rejection limit must be skipped, not wrapped.
	source := bytes.NewReader(append(bytes.Repeat([]byte{255}, 32), bytes.Repeat([]byte{0}, 32)...))
	id, err := betterAuthID(source)
	if err != nil {
		t.Fatal(err)
	}
	if id != strings.Repeat("a", 32) {
		t.Fatalf("biased id = %q", id)
	}
}
