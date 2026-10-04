package migration

import "testing"

func TestProposeRequiresLegacyBindingAndVerifiedEmailToAgree(t *testing.T) {
	users := []Auth0User{
		{UserID: "auth0|agree", Email: "Agree@Example.test", EmailVerified: true, LegacyUserID: "legacy-1"},
		{UserID: "auth0|mismatch", Email: "other@example.test", EmailVerified: true, LegacyUserID: "legacy-2"},
		{UserID: "auth0|unverified", Email: "unverified@example.test", LegacyUserID: "legacy-3"},
		{UserID: "auth0|no-legacy", Email: "agree@example.test", EmailVerified: true},
		{UserID: "auth0|deleted", Email: "deleted@example.test", EmailVerified: true, LegacyUserID: "legacy-gone"},
		{UserID: "auth0|blocked", Email: "agree@example.test", EmailVerified: true, LegacyUserID: "legacy-1", Blocked: true},
	}
	candidates := []AppIdentityCandidate{
		{AppUserID: "app-1", Email: "agree@example.test", LegacyUserID: "legacy-1"},
		{AppUserID: "app-2", Email: "mismatch@example.test", LegacyUserID: "legacy-2"},
		{AppUserID: "app-3", Email: "unverified@example.test", LegacyUserID: "legacy-3"},
	}
	proposal := ProposeAuth0Resolutions(users, candidates)
	if len(proposal.Resolutions) != 1 || proposal.Resolutions[0] != (IdentityResolution{Auth0UserID: "auth0|agree", AppUserID: "app-1"}) {
		t.Fatalf("resolutions = %+v", proposal.Resolutions)
	}
	want := map[string]string{
		"auth0|blocked":    ProposalBlocked,
		"auth0|deleted":    ProposalLegacyUserNotFound,
		"auth0|mismatch":   ProposalEmailMismatch,
		"auth0|no-legacy":  ProposalNoLegacyUserID,
		"auth0|unverified": ProposalEmailUnverified,
	}
	if len(proposal.Gaps) != len(want) {
		t.Fatalf("gaps = %+v", proposal.Gaps)
	}
	for _, gap := range proposal.Gaps {
		if want[gap.Auth0UserID] != gap.Reason {
			t.Fatalf("gap %s reason = %s, want %s", gap.Auth0UserID, gap.Reason, want[gap.Auth0UserID])
		}
	}
	noLegacy := proposal.Gaps[3]
	if noLegacy.Auth0UserID != "auth0|no-legacy" || len(noLegacy.CandidateAppUserIDs) != 1 || noLegacy.CandidateAppUserIDs[0] != "app-1" {
		t.Fatalf("manual gap should carry advisory email candidates: %+v", noLegacy)
	}
	if proposal.Counts["proposed"] != 1 || proposal.Counts[ProposalEmailMismatch] != 1 {
		t.Fatalf("counts = %+v", proposal.Counts)
	}
}
