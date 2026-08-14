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

const (
	Auth0StatusMatched               = "matched"
	Auth0StatusRequiresResolution    = "requires_resolution"
	Auth0StatusPasswordResetRequired = "password_reset_required"
	Auth0StatusImported              = "imported"
)

type Auth0Reconciliation struct {
	Auth0UserCount          int             `json:"auth0UserCount"`
	AppCandidateCount       int             `json:"appCandidateCount"`
	ExplicitResolutionCount int             `json:"explicitResolutionCount"`
	ProviderIdentityCount   int             `json:"providerIdentityCount"`
	ProviderAccounts        []Auth0Identity `json:"providerAccounts"`
	StatusCounts            map[string]int  `json:"statusCounts"`
	UnresolvedAuth0UserIDs  []string        `json:"unresolvedAuth0UserIds"`
}

type Auth0ImportPlan struct {
	Version        int                 `json:"version"`
	CreatedAt      time.Time           `json:"createdAt"`
	Items          []Auth0ImportItem   `json:"items"`
	Reconciliation Auth0Reconciliation `json:"reconciliation"`
	Checksum       string              `json:"checksum"`
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
	seenAuth0 := map[string]bool{}
	seenProviderAccounts := map[string]string{}
	providerAccounts := make([]Auth0Identity, 0)
	for _, user := range users {
		if user.UserID == "" || seenAuth0[user.UserID] {
			return Auth0ImportPlan{}, fmt.Errorf("missing or duplicate Auth0 user id %q", user.UserID)
		}
		seenAuth0[user.UserID] = true
		if len(user.Identities) == 0 {
			return Auth0ImportPlan{}, fmt.Errorf("Auth0 user %s has no provider identities", user.UserID)
		}
		for _, identity := range user.Identities {
			if strings.TrimSpace(identity.Provider) == "" || strings.TrimSpace(identity.ProviderAccountID) == "" {
				return Auth0ImportPlan{}, fmt.Errorf("Auth0 user %s has an identity with a missing provider key", user.UserID)
			}
			key := identity.Provider + "\x00" + identity.ProviderAccountID
			if previous := seenProviderAccounts[key]; previous != "" {
				return Auth0ImportPlan{}, fmt.Errorf("provider account %s/%s is duplicated by Auth0 users %s and %s", identity.Provider, identity.ProviderAccountID, previous, user.UserID)
			}
			seenProviderAccounts[key] = user.UserID
			providerAccounts = append(providerAccounts, identity)
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
		if !seenAuth0[resolution.Auth0UserID] {
			return Auth0ImportPlan{}, fmt.Errorf("identity resolution references unknown Auth0 user %s", resolution.Auth0UserID)
		}
		if _, duplicate := resolvedByAuth0[resolution.Auth0UserID]; duplicate {
			return Auth0ImportPlan{}, fmt.Errorf("duplicate identity resolution for %s", resolution.Auth0UserID)
		}
		resolvedByAuth0[resolution.Auth0UserID] = resolution.AppUserID
	}
	plan := Auth0ImportPlan{Version: 1, CreatedAt: now.UTC()}
	for _, user := range users {
		item := Auth0ImportItem{Auth0UserID: user.UserID, EmailVerified: user.EmailVerified, Identities: append([]Auth0Identity(nil), user.Identities...)}
		sort.Slice(item.Identities, func(left, right int) bool {
			return item.Identities[left].Provider+"\x00"+item.Identities[left].ProviderAccountID < item.Identities[right].Provider+"\x00"+item.Identities[right].ProviderAccountID
		})
		if user.EmailVerified {
			item.CandidateAppUserIDs = sortedStrings(candidatesByEmail[strings.ToLower(strings.TrimSpace(user.Email))])
		}
		hasPasswordIdentity := false
		for _, identity := range item.Identities {
			if identity.Provider == "auth0" {
				hasPasswordIdentity = true
				break
			}
		}
		item.PasswordHashCompatible = hasPasswordIdentity && user.PasswordHash != "" && compatible(user.PasswordHash)
		item.RequiresPasswordReset = hasPasswordIdentity && !item.PasswordHashCompatible
		if appUserID := resolvedByAuth0[user.UserID]; appUserID != "" {
			item.ResolvedAppUserID = appUserID
			if item.RequiresPasswordReset {
				item.Status = Auth0StatusPasswordResetRequired
				item.Detail = "explicit identity resolution supplied; database password requires a verified reset"
			} else {
				item.Status = Auth0StatusMatched
				item.Detail = "explicit identity resolution supplied"
			}
		} else if len(item.CandidateAppUserIDs) > 0 {
			item.Status = Auth0StatusRequiresResolution
			item.Detail = "email candidates are advisory and are never merged automatically"
		} else {
			item.Status = Auth0StatusRequiresResolution
			item.Detail = "no explicit app identity mapping"
		}
		plan.Items = append(plan.Items, item)
	}
	sort.Slice(plan.Items, func(left, right int) bool { return plan.Items[left].Auth0UserID < plan.Items[right].Auth0UserID })
	sort.Slice(providerAccounts, func(left, right int) bool {
		return providerAccounts[left].Provider+"\x00"+providerAccounts[left].ProviderAccountID < providerAccounts[right].Provider+"\x00"+providerAccounts[right].ProviderAccountID
	})
	statusCounts := map[string]int{
		Auth0StatusMatched:               0,
		Auth0StatusRequiresResolution:    0,
		Auth0StatusPasswordResetRequired: 0,
		Auth0StatusImported:              0,
	}
	unresolved := make([]string, 0)
	for _, item := range plan.Items {
		statusCounts[item.Status]++
		if item.Status == Auth0StatusRequiresResolution {
			unresolved = append(unresolved, item.Auth0UserID)
		}
	}
	plan.Reconciliation = Auth0Reconciliation{
		Auth0UserCount:          len(users),
		AppCandidateCount:       len(candidates),
		ExplicitResolutionCount: len(resolutions),
		ProviderIdentityCount:   len(providerAccounts),
		ProviderAccounts:        providerAccounts,
		StatusCounts:            statusCounts,
		UnresolvedAuth0UserIDs:  unresolved,
	}
	checksumPlan := plan
	checksumPlan.Checksum = ""
	checksum, err := Checksum(checksumPlan)
	if err != nil {
		return Auth0ImportPlan{}, err
	}
	plan.Checksum = checksum
	return plan, nil
}
