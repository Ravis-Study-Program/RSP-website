package jobs

import "testing"

func TestNormalizeURLPreservesPositionIdentifiers(t *testing.T) {
	got, err := NormalizeURL("https://www.vivcourt.com/job-listings/?gh_jid=4653802006&gh_src=ref&utm_source=mail#apply")
	if err != nil || got != "https://www.vivcourt.com/job-listings/?gh_jid=4653802006" {
		t.Fatalf("%q %v", got, err)
	}
	first, _ := NormalizeURL("https://jobs.example.com/careers?pid=123&start=0")
	second, _ := NormalizeURL("https://jobs.example.com/careers?pid=456&start=0")
	if first == second {
		t.Fatal("distinct jobs must not collapse")
	}
	for _, raw := range []string{"javascript:alert(1)", "http://example.com", "https://localhost/apply", "https://127.0.0.1/apply", "https://user:pass@example.com/job", "https://jobs.internal/job", "https://example.com:8080/job"} {
		if _, err := NormalizeURL(raw); err == nil {
			t.Errorf("accepted %q", raw)
		}
	}
}

func TestPositionAndStageValidation(t *testing.T) {
	p := PositionInput{CompanyName: " Acme ", Category: "other", Title: " Engineer ", JobType: "experienced", ApplicationURL: "https://example.com/job", Locations: []string{" Sydney ", "sydney", "Melbourne"}}
	if _, err := p.Validate(); err != nil {
		t.Fatal(err)
	}
	if p.CompanyName != "Acme" || len(p.Locations) != 2 || p.Status != "open" {
		t.Fatalf("unexpected normalized input: %#v", p)
	}
	p.JobType = "unrecognized"
	if _, err := p.Validate(); err == nil {
		t.Fatal("invalid type accepted")
	}
	s := StageInput{Title: "Technical interview", Date: "2026-02-30"}
	if s.Validate() == nil {
		t.Fatal("invalid calendar date accepted")
	}
	s.Date = "2026-02-28"
	if err := s.Validate(); err != nil {
		t.Fatal(err)
	}
}
