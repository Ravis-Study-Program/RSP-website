package migration

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

type Auth0Identity struct {
	Provider          string `json:"provider"`
	ProviderAccountID string `json:"providerAccountId"`
}

type Auth0User struct {
	UserID        string          `json:"userId"`
	Email         string          `json:"email,omitempty"`
	EmailVerified bool            `json:"emailVerified"`
	PasswordHash  string          `json:"passwordHash,omitempty"`
	Identities    []Auth0Identity `json:"identities"`
}

type AppIdentityCandidate struct {
	AppUserID string `json:"appUserId"`
	Email     string `json:"email,omitempty"`
}

type IdentityResolution struct {
	Auth0UserID string `json:"auth0UserId"`
	AppUserID   string `json:"appUserId"`
}

type Auth0ImportItem struct {
	Auth0UserID            string          `json:"auth0UserId"`
	CandidateAppUserIDs    []string        `json:"candidateAppUserIds,omitempty"`
	ResolvedAppUserID      string          `json:"resolvedAppUserId,omitempty"`
	Status                 string          `json:"status"`
	EmailVerified          bool            `json:"emailVerified"`
	PasswordHashCompatible bool            `json:"passwordHashCompatible"`
	RequiresPasswordReset  bool            `json:"requiresPasswordReset"`
	Identities             []Auth0Identity `json:"identities"`
	Detail                 string          `json:"detail"`
}

type Auth0ImportPlan struct {
	Version   int               `json:"version"`
	CreatedAt time.Time         `json:"createdAt"`
	Items     []Auth0ImportItem `json:"items"`
	Checksum  string            `json:"checksum"`
}

type PasswordCompatibility func(hash string) bool

// PlanAuth0Import never links identities by email automatically. Even one
// verified email candidate requires an explicit checksum-reviewed resolution.
func PlanAuth0Import(users []Auth0User, candidates []AppIdentityCandidate, resolutions []IdentityResolution, now time.Time, compatible PasswordCompatibility) (Auth0ImportPlan, error) {
	if compatible == nil {
		compatible = func(string) bool { return false }
	}
	candidatesByEmail := map[string][]string{}
	knownCandidates := map[string]bool{}
	for _, candidate := range candidates {
		if candidate.AppUserID == "" || knownCandidates[candidate.AppUserID] {
			return Auth0ImportPlan{}, fmt.Errorf("missing or duplicate app user id %q", candidate.AppUserID)
		}
		knownCandidates[candidate.AppUserID] = true
		email := strings.ToLower(strings.TrimSpace(candidate.Email))
		if email != "" {
			candidatesByEmail[email] = append(candidatesByEmail[email], candidate.AppUserID)
		}
	}
	resolvedByAuth0 := map[string]string{}
	for _, resolution := range resolutions {
		if resolution.Auth0UserID == "" || resolution.AppUserID == "" {
			return Auth0ImportPlan{}, fmt.Errorf("identity resolution requires both ids")
		}
		if !knownCandidates[resolution.AppUserID] {
			return Auth0ImportPlan{}, fmt.Errorf("identity resolution references unknown app user %s", resolution.AppUserID)
		}
		if _, duplicate := resolvedByAuth0[resolution.Auth0UserID]; duplicate {
			return Auth0ImportPlan{}, fmt.Errorf("duplicate identity resolution for %s", resolution.Auth0UserID)
		}
		resolvedByAuth0[resolution.Auth0UserID] = resolution.AppUserID
	}
	plan := Auth0ImportPlan{Version: 1, CreatedAt: now.UTC()}
	seenAuth0 := map[string]bool{}
	for _, user := range users {
		if user.UserID == "" || seenAuth0[user.UserID] {
			return Auth0ImportPlan{}, fmt.Errorf("missing or duplicate Auth0 user id %q", user.UserID)
		}
		seenAuth0[user.UserID] = true
		item := Auth0ImportItem{Auth0UserID: user.UserID, EmailVerified: user.EmailVerified, Identities: append([]Auth0Identity(nil), user.Identities...)}
		sort.Slice(item.Identities, func(left, right int) bool {
			return item.Identities[left].Provider+"\x00"+item.Identities[left].ProviderAccountID < item.Identities[right].Provider+"\x00"+item.Identities[right].ProviderAccountID
		})
		if user.EmailVerified {
			item.CandidateAppUserIDs = sortedStrings(candidatesByEmail[strings.ToLower(strings.TrimSpace(user.Email))])
		}
		item.PasswordHashCompatible = user.PasswordHash != "" && compatible(user.PasswordHash)
		item.RequiresPasswordReset = user.PasswordHash == "" || !item.PasswordHashCompatible
		if appUserID := resolvedByAuth0[user.UserID]; appUserID != "" {
			item.ResolvedAppUserID = appUserID
			item.Status = "ready"
			item.Detail = "explicit identity resolution supplied"
		} else if len(item.CandidateAppUserIDs) > 0 {
			item.Status = "requires_resolution"
			item.Detail = "email candidates are advisory and are never merged automatically"
		} else {
			item.Status = "unmatched"
			item.Detail = "no explicit app identity mapping"
		}
		plan.Items = append(plan.Items, item)
	}
	sort.Slice(plan.Items, func(left, right int) bool { return plan.Items[left].Auth0UserID < plan.Items[right].Auth0UserID })
	checksumPlan := plan
	checksumPlan.Checksum = ""
	checksum, err := Checksum(checksumPlan)
	if err != nil {
		return Auth0ImportPlan{}, err
	}
	plan.Checksum = checksum
	return plan, nil
}
