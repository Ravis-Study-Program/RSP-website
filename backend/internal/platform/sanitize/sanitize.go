// Package sanitize sanitizes user-provided HTML.
package sanitize

import (
	"regexp"

	"github.com/microcosm-cc/bluemonday"
)

// HTML represents a backend data structure.
type HTML struct{ policy *bluemonday.Policy }

// New creates a new value.
func New() *HTML {
	p := bluemonday.NewPolicy()
	p.AllowElements("p", "br", "strong", "b", "em", "i", "u", "s", "strike", "mark", "span", "sub", "sup", "code", "pre", "blockquote", "ul", "ol", "li", "h1", "h2", "h3", "h4", "hr", "a")
	p.AllowAttrs("href", "title").OnElements("a")
	p.AllowAttrs("target").Matching(regexp.MustCompile(`^_blank$`)).OnElements("a")
	p.AllowAttrs("data-type").Matching(bluemonday.SpaceSeparatedTokens).OnElements("p", "blockquote")
	p.AllowAttrs("data-text-align").Matching(regexp.MustCompile(`^(left|center|right|justify)$`)).OnElements("p", "h1", "h2", "h3", "h4", "blockquote")
	p.AllowStyles("text-align").Matching(regexp.MustCompile(`^(left|center|right|justify)$`)).OnElements("p", "h1", "h2", "h3", "h4", "blockquote")
	p.AllowAttrs("data-color").Matching(regexp.MustCompile(`^#[0-9a-fA-F]{3,8}$`)).OnElements("mark")
	p.AllowStyles("background-color").Matching(regexp.MustCompile(`^#[0-9a-fA-F]{3,8}$`)).OnElements("mark")
	p.AllowURLSchemes("http", "https", "mailto")
	p.RequireNoFollowOnLinks(true)
	p.RequireNoReferrerOnLinks(true)
	p.AddTargetBlankToFullyQualifiedLinks(true)
	return &HTML{policy: p}
}

// String performs the operation.
func (h *HTML) String(value string) string { return h.policy.Sanitize(value) }
