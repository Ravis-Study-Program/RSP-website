package api

import (
	"net/mail"
	"strings"
)

// validEmailAddress reports whether email is a plain address such as
// name@example.com: one that parses, has no display name, and whose domain
// has at least two dot-separated parts (so name@mail is refused).
func validEmailAddress(email string) bool {
	if len(email) > 254 {
		return false
	}
	address, err := mail.ParseAddress(email)
	if err != nil || address.Address != email {
		return false
	}
	_, domain, found := strings.Cut(email, "@")
	if !found || !strings.Contains(domain, ".") {
		return false
	}
	for _, label := range strings.Split(domain, ".") {
		if label == "" {
			return false
		}
	}
	return true
}
