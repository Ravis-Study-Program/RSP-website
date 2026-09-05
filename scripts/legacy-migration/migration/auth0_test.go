package migration

import "testing"

func TestAuth0PlanNeverSilentlyMergesByEmail(t *testing.T) {
	users := []Auth0User{{
		UserID: "auth0|one", Email: "person@example.test", EmailVerified: true,
		PasswordHash: "$2b$fixture", Identities: []Auth0Identity{{Provider: "auth0", ProviderAccountID: "database-1"}},
	}}
	candidates := []AppIdentityCandidate{{AppUserID: "app-1", Email: "PERSON@example.test"}}
	plan, err := PlanAuth0Import(users, candidates, nil, fixedNow(), func(hash string) bool { return hash == "$2b$fixture" })
	if err != nil {
		t.Fatal(err)
	}

	item := plan.Items[0]
	if item.Status != Auth0StatusRequiresResolution || item.ResolvedAppUserID != "" {
		t.Fatalf("email was silently merged: %+v", item)
	}
	if !item.PasswordHashCompatible || item.RequiresPasswordReset {
		t.Fatalf("compatible password handling = %+v", item)
	}
	resolved, err := PlanAuth0Import(users, candidates, []IdentityResolution{{Auth0UserID: "auth0|one", AppUserID: "app-1"}}, fixedNow(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if resolved.Items[0].Status != Auth0StatusPasswordResetRequired || !resolved.Items[0].RequiresPasswordReset {
		t.Fatalf("explicit resolution plan = %+v", resolved.Items[0])
	}
}

func TestAuth0PlanReconcilesSocialAndPasswordIdentities(t *testing.T) {
	users := []Auth0User{
		{UserID: "auth0|google", Email: "google@example.test", EmailVerified: true, Identities: []Auth0Identity{{Provider: "google-oauth2", ProviderAccountID: "google-1"}}},
		{UserID: "auth0|password", Email: "password@example.test", EmailVerified: true, PasswordHash: "unsupported", Identities: []Auth0Identity{{Provider: "auth0", ProviderAccountID: "database-1"}}},
		{UserID: "auth0|ambiguous", Email: "shared@example.test", EmailVerified: true, Identities: []Auth0Identity{{Provider: "google-oauth2", ProviderAccountID: "google-2"}}},
	}
	candidates := []AppIdentityCandidate{
		{AppUserID: "app-google", Email: "google@example.test"},
		{AppUserID: "app-password", Email: "password@example.test"},
		{AppUserID: "app-shared-a", Email: "shared@example.test"},
		{AppUserID: "app-shared-b", Email: "shared@example.test"},
	}
	resolutions := []IdentityResolution{
		{Auth0UserID: "auth0|google", AppUserID: "app-google"},
		{Auth0UserID: "auth0|password", AppUserID: "app-password"},
	}
	plan, err := PlanAuth0Import(users, candidates, resolutions, fixedNow(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := plan.Reconciliation.StatusCounts[Auth0StatusMatched]; got != 1 {
		t.Fatalf("matched count = %d", got)
	}
	if got := plan.Reconciliation.StatusCounts[Auth0StatusPasswordResetRequired]; got != 1 {
		t.Fatalf("password-reset count = %d", got)
	}
	if got := plan.Reconciliation.StatusCounts[Auth0StatusRequiresResolution]; got != 1 {
		t.Fatalf("unresolved count = %d", got)
	}
	if plan.Reconciliation.StatusCounts[Auth0StatusImported] != 0 || plan.Reconciliation.ProviderIdentityCount != 3 {
		t.Fatalf("reconciliation = %+v", plan.Reconciliation)
	}
	if len(plan.Reconciliation.UnresolvedAuth0UserIDs) != 1 || plan.Reconciliation.UnresolvedAuth0UserIDs[0] != "auth0|ambiguous" {
		t.Fatalf("unresolved ids = %v", plan.Reconciliation.UnresolvedAuth0UserIDs)
	}
	if plan.Items[0].Status != Auth0StatusRequiresResolution || len(plan.Items[0].CandidateAppUserIDs) != 2 {
		t.Fatalf("ambiguous item = %+v", plan.Items[0])
	}
}

func TestAuth0PlanRejectsDuplicateProviderAndStaleResolution(t *testing.T) {
	if _, err := PlanAuth0Import([]Auth0User{{UserID: "auth0|missing-provider"}}, nil, nil, fixedNow(), nil); err == nil {
		t.Fatal("Auth0 user without a provider identity was accepted")
	}

	users := []Auth0User{
		{UserID: "auth0|one", Identities: []Auth0Identity{{Provider: "google-oauth2", ProviderAccountID: "shared"}}},
		{UserID: "auth0|two", Identities: []Auth0Identity{{Provider: "google-oauth2", ProviderAccountID: "shared"}}},
	}
	if _, err := PlanAuth0Import(users, nil, nil, fixedNow(), nil); err == nil {
		t.Fatal("duplicate provider account was accepted")
	}

	users = users[:1]
	candidates := []AppIdentityCandidate{{AppUserID: "app-1"}}
	resolutions := []IdentityResolution{{Auth0UserID: "auth0|missing", AppUserID: "app-1"}}
	if _, err := PlanAuth0Import(users, candidates, resolutions, fixedNow(), nil); err == nil {
		t.Fatal("resolution for an unknown Auth0 user was accepted")
	}
}
