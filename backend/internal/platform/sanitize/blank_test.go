package sanitize

import "testing"

func TestIsBlank(t *testing.T) {
	for _, blank := range []string{"", "<p></p>", "<p> </p><p>&nbsp;</p>", "<h1></h1><hr><ul><li><p></p></li></ul>", "<p><br></p>", " \n\t"} {
		if !IsBlank(blank) {
			t.Errorf("IsBlank(%q) = false, want true", blank)
		}
	}
	for _, content := range []string{"<p>Read chapter 3</p>", "<h2>x</h2>", "<p><a href=\"https://example.com\">link</a></p>", "plain"} {
		if IsBlank(content) {
			t.Errorf("IsBlank(%q) = true, want false", content)
		}
	}
}
