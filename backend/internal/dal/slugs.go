package dal

import (
	"context"
	"fmt"
	"strings"
	"unicode"

	"github.com/jackc/pgx/v5"
	"golang.org/x/text/unicode/norm"
)

// maxSlugBase leaves room for a "-NN" suffix within the 50-character limit.
const maxSlugBase = 44

// NameSlug turns a name into the start of a profile slug: lower-case letters
// and digits joined by single hyphens, accents dropped ("Zoë Frew" becomes
// "zoe-frew"). A name with no usable letters gives "member", and a very short
// one is prefixed so the slug is at least three characters.
func NameSlug(name string) string {
	var b strings.Builder
	hyphen := false
	for _, r := range norm.NFKD.String(strings.ToLower(name)) {
		switch {
		case unicode.Is(unicode.Mn, r):
			// An accent split off its letter by NFKD.
		case r >= 'a' && r <= 'z' || r >= '0' && r <= '9':
			if hyphen && b.Len() > 0 {
				b.WriteByte('-')
			}
			hyphen = false
			b.WriteRune(r)
		default:
			hyphen = true
		}
	}
	slug := b.String()
	if len(slug) > maxSlugBase {
		slug = strings.TrimRight(slug[:maxSlugBase], "-")
	}
	switch {
	case slug == "":
		return "member"
	case len(slug) < 3:
		return "member-" + slug
	}
	return slug
}

type rowQuerier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

// freeSlug returns base if no other profile uses it, or else base-2, base-3
// and so on, whichever is first free. exceptUserID's own slug does not count.
func freeSlug(ctx context.Context, q rowQuerier, base, exceptUserID string) (string, error) {
	rows, err := q.Query(ctx, `SELECT lower(slug) FROM app.user_profiles
 WHERE (lower(slug)=$1 OR lower(slug) ~ ('^' || $1 || '-[0-9]+$')) AND user_id::text <> $2`, base, exceptUserID)
	if err != nil {
		return "", err
	}
	defer rows.Close()
	taken := map[string]bool{}
	for rows.Next() {
		var slug string
		if err := rows.Scan(&slug); err != nil {
			return "", err
		}
		taken[slug] = true
	}
	if err := rows.Err(); err != nil {
		return "", err
	}
	if !taken[base] {
		return base, nil
	}
	for n := 2; ; n++ {
		if candidate := fmt.Sprintf("%s-%d", base, n); !taken[candidate] {
			return candidate, nil
		}
	}
}
