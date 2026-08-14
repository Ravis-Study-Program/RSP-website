package httpapi

import (
	"testing"

	"github.com/magedmg/RSP-website/backend/internal/authn"
	"github.com/magedmg/RSP-website/backend/internal/authz"
)

func TestClaimsMustMatchCurrentAccountStateAndSecurityVersion(t *testing.T) {
	actor := authz.Actor{AccountState: authz.AccountActive, SecurityVersion: 4}
	for name, claims := range map[string]authn.Claims{
		"current":       {AccountState: "active", SecurityVersion: 4},
		"stale version": {AccountState: "active", SecurityVersion: 3},
		"wrong state":   {AccountState: "suspended", SecurityVersion: 4},
		"missing":       {AccountState: "active"},
	} {
		want := name == "current"
		if got := claimsMatchActor(claims, actor); got != want {
			t.Fatalf("%s: got %v, want %v", name, got, want)
		}
	}
}
