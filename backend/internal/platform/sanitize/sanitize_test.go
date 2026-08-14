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

func TestTiptapAllowlistPreservesSupportedFormattingAndBoundsAttributes(t *testing.T) {
	got := New().String(`<h4 style="text-align: center" onclick="bad()">Heading</h4><p data-text-align="justify"><mark data-color="#ffcc00" style="background-color: #ffcc00">mark</mark><sub>sub</sub><sup>sup</sup><span>text</span><a href="mailto:hello@example.com" target="_blank" rel="opener">mail</a></p><p style="text-align: expression(alert(1))">bad align</p>`)
	for _, want := range []string{"<h4 style=\"text-align: center\">", `data-text-align="justify"`, "<mark", "<sub>", "<sup>", "<span>", "mailto:hello@example.com"} {
		if !strings.Contains(got, want) {
			t.Fatalf("lost supported %q in %s", want, got)
		}
	}
	for _, bad := range []string{"onclick", "expression", `rel="opener"`} {
		if strings.Contains(strings.ToLower(got), bad) {
			t.Fatalf("unsafe %q in %s", bad, got)
		}
	}
}
