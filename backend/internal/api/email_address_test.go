package api

import "testing"

func TestValidEmailAddress(t *testing.T) {
	for email, want := range map[string]bool{
		"name@example.com":  true,
		"a.b+c@mail.co.uk":  true,
		"coord-maddie@mail": false,
		"name@example.":     false,
		"name@.com":         false,
		"Name <a@b.com>":    false,
		"no-at-sign.com":    false,
		"":                  false,
	} {
		if got := validEmailAddress(email); got != want {
			t.Errorf("validEmailAddress(%q) = %v, want %v", email, got, want)
		}
	}
}
