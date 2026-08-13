package migration

import "testing"

func TestAuth0PlanNeverSilentlyMergesByEmail(t *testing.T) {
	users := []Auth0User{{
		UserID: "auth0|one", Email: "person@example.test", EmailVerified: true,
		PasswordHash: "$2b$fixture", Identities: []Auth0Identity{{Provider: "google-oauth2", ProviderAccountID: "google-1"}},
	}}
	candidates := []AppIdentityCandidate{{AppUserID: "app-1", Email: "PERSON@example.test"}}
	plan, err := PlanAuth0Import(users, candidates, nil, fixedNow(), func(hash string) bool { return hash == "$2b$fixture" })
	if err != nil {
		t.Fatal(err)
	}
	item := plan.Items[0]
	if item.Status != "requires_resolution" || item.ResolvedAppUserID != "" {
		t.Fatalf("email was silently merged: %+v", item)
	}
	if !item.PasswordHashCompatible || item.RequiresPasswordReset {
		t.Fatalf("compatible password handling = %+v", item)
	}
	resolved, err := PlanAuth0Import(users, candidates, []IdentityResolution{{Auth0UserID: "auth0|one", AppUserID: "app-1"}}, fixedNow(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if resolved.Items[0].Status != "ready" || !resolved.Items[0].RequiresPasswordReset {
		t.Fatalf("explicit resolution plan = %+v", resolved.Items[0])
	}
}
