package sanitize

import "github.com/microcosm-cc/bluemonday"

type HTML struct{ policy *bluemonday.Policy }

func New() *HTML {
	p := bluemonday.NewPolicy()
	p.AllowElements("p", "br", "strong", "b", "em", "i", "u", "s", "code", "pre", "blockquote", "ul", "ol", "li", "h1", "h2", "h3", "hr")
	p.AllowAttrs("href", "title").OnElements("a")
	p.AllowAttrs("target").Matching(bluemonday.SpaceSeparatedTokens).OnElements("a")
	p.AllowAttrs("data-type").Matching(bluemonday.SpaceSeparatedTokens).OnElements("p", "blockquote")
	p.AllowStandardURLs()
	p.RequireNoFollowOnLinks(true)
	p.RequireNoReferrerOnLinks(true)
	p.AddTargetBlankToFullyQualifiedLinks(true)
	return &HTML{policy: p}
}

func (h *HTML) String(value string) string { return h.policy.Sanitize(value) }
