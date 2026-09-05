package migration

import (
	"regexp"

	"github.com/microcosm-cc/bluemonday"
)

var legacyHTMLPolicy = func() *bluemonday.Policy {
	policy := bluemonday.NewPolicy()
	policy.AllowElements("p", "br", "strong", "b", "em", "i", "u", "s", "strike", "mark", "span", "sub", "sup", "code", "pre", "blockquote", "ul", "ol", "li", "h1", "h2", "h3", "h4", "hr", "a")
	policy.AllowAttrs("href", "title").OnElements("a")
	policy.AllowAttrs("target").Matching(regexp.MustCompile(`^_blank$`)).OnElements("a")
	policy.AllowAttrs("data-type").Matching(bluemonday.SpaceSeparatedTokens).OnElements("p", "blockquote")
	policy.AllowAttrs("data-text-align").Matching(regexp.MustCompile(`^(left|center|right|justify)$`)).OnElements("p", "h1", "h2", "h3", "h4", "blockquote")
	policy.AllowStyles("text-align").Matching(regexp.MustCompile(`^(left|center|right|justify)$`)).OnElements("p", "h1", "h2", "h3", "h4", "blockquote")
	policy.AllowAttrs("data-color").Matching(regexp.MustCompile(`^#[0-9a-fA-F]{3,8}$`)).OnElements("mark")
	policy.AllowStyles("background-color").Matching(regexp.MustCompile(`^#[0-9a-fA-F]{3,8}$`)).OnElements("mark")
	policy.AllowURLSchemes("http", "https", "mailto")
	policy.RequireNoFollowOnLinks(true)
	policy.RequireNoReferrerOnLinks(true)
	policy.AddTargetBlankToFullyQualifiedLinks(true)
	return policy
}()

// sanitizeLegacyHTML is deliberately conservative. It strips active content
// while retaining the legacy formatting markup needed by the import.
func sanitizeLegacyHTML(input string) (string, bool) {
	result := legacyHTMLPolicy.Sanitize(input)
	return result, result != input
}
