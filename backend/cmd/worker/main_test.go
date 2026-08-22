package main

import (
	"testing"

	"github.com/magedmg/RSP-website/backend/internal/leetcode"
)

func TestResolvedSyncURL(t *testing.T) {
	if got, err := resolvedSyncURL("", "production"); err != nil || got != leetcode.DefaultURL {
		t.Fatalf("default URL = %q, %v", got, err)
	}
	if _, err := resolvedSyncURL("http://127.0.0.1:9091/catalog", "production"); err == nil {
		t.Fatal("production accepted an HTTP catalogue URL")
	}
	if got, err := resolvedSyncURL("http://127.0.0.1:9091/catalog", "test"); err != nil || got == "" {
		t.Fatalf("test stub URL = %q, %v", got, err)
	}

	for _, invalid := range []string{"file:///tmp/catalog", "https://user:secret@example.test/catalog", "https://example.test/catalog#fragment"} {
		if _, err := resolvedSyncURL(invalid, "production"); err == nil {
			t.Fatalf("accepted invalid URL %q", invalid)
		}
	}
}
