package api

import (
	"testing"

	"github.com/magedmg/RSP-website/backend/internal/dal"
)

func TestMatchMentor(t *testing.T) {
	mentors := []dal.SeasonMentor{
		{UserID: "a", Name: "Priya Raman", Email: "priya@example.test", Slug: "priya-raman"},
		{UserID: "b", Name: "Sam Lee", Email: "sam.one@example.test"},
		{UserID: "c", Name: "Sam Lee", Email: "sam.two@example.test"},
	}
	for named, want := range map[string]string{
		"priya@example.test":   "a",
		"PRIYA@example.test":   "a",
		"priya raman":          "a",
		"priya-raman":          "a",
		"@Priya-Raman":         "a",
		"sam.two@example.test": "c",
	} {
		if got, problem := matchMentor(mentors, named); got != want || problem != "" {
			t.Errorf("matchMentor(%q) = %q, %q; want %q", named, got, problem, want)
		}
	}
	for _, named := range []string{"Sam Lee", "Nobody"} {
		if got, problem := matchMentor(mentors, named); got != "" || problem == "" {
			t.Errorf("matchMentor(%q) = %q, %q; want a problem", named, got, problem)
		}
	}
}
