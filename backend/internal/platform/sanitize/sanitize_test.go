package sanitize

import (
	"strings"
	"testing"
)

func TestTiptapAllowlistRemovesStoredXSS(t *testing.T) {
	got := New().String(`<p onclick="steal()">Hello <strong>world</strong><script>alert(1)</script><img src=x onerror=alert(2)><a href="javascript:alert(3)">bad</a><a href="https://example.com">safe</a></p>`)
	for _, bad := range []string{"onclick", "script", "img", "javascript:"} {
		if strings.Contains(strings.ToLower(got), bad) {
			t.Fatalf("unsafe %q in %s", bad, got)
		}
	}
	for _, ok := range []string{"<p>", "<strong>", "https://example.com"} {
		if !strings.Contains(got, ok) {
			t.Fatalf("lost %q in %s", ok, got)
		}
	}
}
