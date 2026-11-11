// Package sanitize sanitizes user-provided HTML.
package sanitize

import (
	"html"
	"regexp"
	"strings"

	"github.com/microcosm-cc/bluemonday"
)

type HTML struct{ policy *bluemonday.Policy }

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

func (h *HTML) String(value string) string { return h.policy.Sanitize(value) }

var textOnly = bluemonday.StrictPolicy()

// IsBlank reports whether rich text has no visible words: only tags (such as
// an empty paragraph or a divider), spaces and non-breaking spaces.
func IsBlank(value string) bool {
	text := html.UnescapeString(textOnly.Sanitize(value))
	return strings.TrimSpace(strings.ReplaceAll(text, "\u00a0", " ")) == ""
}
