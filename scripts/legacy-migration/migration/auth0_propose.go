package migration

import (
	"sort"
	"strings"
)

// Reasons a proposal was not drafted. Each one is a manual-resolution item.
const (
	ProposalBlocked             = "blocked"
	ProposalEmailUnverified     = "email_unverified"
	ProposalNoLegacyUserID      = "no_legacy_user_id"
	ProposalLegacyUserNotFound  = "legacy_user_not_found"
	ProposalEmailMismatch       = "email_mismatch"
	ProposalDuplicateLegacyUser = "duplicate_legacy_user_candidate"
)

type Auth0ProposalGap struct {
	Auth0UserID         string   `json:"auth0UserId"`
	Reason              string   `json:"reason"`
	CandidateAppUserIDs []string `json:"candidateAppUserIds,omitempty"`
}

type Auth0Proposal struct {
	Resolutions []IdentityResolution `json:"resolutions"`
	Gaps        []Auth0ProposalGap   `json:"gaps"`
	Counts      map[string]int       `json:"counts"`
}

// ProposeAuth0Resolutions drafts resolutions for review. A draft needs two
// independent signals that agree: the legacy server's app_metadata.userId
// binding, and a verified Auth0 email equal to that legacy user's email. The
// output is an input for human review, not an approved mapping; only the
// resolutions file passed to plan and apply is authoritative.
func ProposeAuth0Resolutions(users []Auth0User, candidates []AppIdentityCandidate) Auth0Proposal {
	byLegacy := map[string][]AppIdentityCandidate{}
	byEmail := map[string][]string{}
	for _, candidate := range candidates {
		if candidate.LegacyUserID != "" {
			byLegacy[candidate.LegacyUserID] = append(byLegacy[candidate.LegacyUserID], candidate)
		}
		if email := normalizeEmail(candidate.Email); email != "" {
			byEmail[email] = append(byEmail[email], candidate.AppUserID)
		}
	}

	proposal := Auth0Proposal{
		Resolutions: make([]IdentityResolution, 0),
		Gaps:        make([]Auth0ProposalGap, 0),
		Counts:      map[string]int{"proposed": 0},
	}
	gap := func(user Auth0User, reason string) {
		proposal.Gaps = append(proposal.Gaps, Auth0ProposalGap{
			Auth0UserID:         user.UserID,
			Reason:              reason,
			CandidateAppUserIDs: sortedStrings(byEmail[normalizeEmail(user.Email)]),
		})
		proposal.Counts[reason]++
	}
	for _, user := range users {
		switch {
		case user.Blocked:
			gap(user, ProposalBlocked)
			continue
		case !user.EmailVerified:
			gap(user, ProposalEmailUnverified)
			continue
		case user.LegacyUserID == "":
			gap(user, ProposalNoLegacyUserID)
			continue
		}
		matches := byLegacy[user.LegacyUserID]
		switch {
		case len(matches) == 0:
			gap(user, ProposalLegacyUserNotFound)
		case len(matches) > 1:
			gap(user, ProposalDuplicateLegacyUser)
		case normalizeEmail(matches[0].Email) == "" || normalizeEmail(matches[0].Email) != normalizeEmail(user.Email):
			gap(user, ProposalEmailMismatch)
		default:
			proposal.Resolutions = append(proposal.Resolutions, IdentityResolution{Auth0UserID: user.UserID, AppUserID: matches[0].AppUserID})
			proposal.Counts["proposed"]++
		}
	}
	sort.Slice(proposal.Resolutions, func(left, right int) bool {
		return proposal.Resolutions[left].Auth0UserID < proposal.Resolutions[right].Auth0UserID
	})
	sort.Slice(proposal.Gaps, func(left, right int) bool { return proposal.Gaps[left].Auth0UserID < proposal.Gaps[right].Auth0UserID })
	return proposal
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}
