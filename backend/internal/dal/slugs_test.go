package dal

import (
	"strings"
	"testing"
)

func TestNameSlug(t *testing.T) {
	for name, want := range map[string]string{
		"Maddie Frew":              "maddie-frew",
		"  Maddie   Frew  ":        "maddie-frew",
		"Zoë O'Brien-Smith":        "zoe-o-brien-smith",
		"José García":              "jose-garcia",
		"Al":                       "al",
		"MM":                       "mm",
		"J":                        "member-j",
		"李小龍":                      "member",
		"":                         "member",
		"Agent 007":                "agent-007",
		strings.Repeat("abc ", 30): strings.TrimRight(strings.Repeat("abc-", 11), "-"),
	} {
		if got := NameSlug(name); got != want {
			t.Errorf("NameSlug(%q) = %q, want %q", name, got, want)
		}
		if got := NameSlug(name); len(got) > maxSlugBase {
			t.Errorf("NameSlug(%q) is %d long", name, len(got))
		}
	}
}
