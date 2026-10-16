package migration

import (
	"bytes"
	"compress/gzip"
	"strings"
	"testing"
)

const exportNDJSON = `{"user_id":"google-oauth2|1001","email":"Google@Example.test","email_verified":true,"identities":[{"provider":"google-oauth2","user_id":"1001","connection":"google-oauth2"}],"app_metadata":{"userId":"legacy-google","isAdmin":false}}
{"user_id":"auth0|abc","email":"password@example.test","email_verified":true,"identities":[{"provider":"auth0","user_id":"abc","connection":"Username-Password-Authentication"},{"provider":"google-oauth2","user_id":2002,"connection":"google-oauth2"}],"app_metadata":{"userId":"Unknown User"}}
{"user_id":"auth0|blocked","email":"blocked@example.test","email_verified":false,"blocked":true}
`

func TestNormalizeAuth0ExportReadsGzipNDJSON(t *testing.T) {
	var compressed bytes.Buffer
	writer := gzip.NewWriter(&compressed)
	if _, err := writer.Write([]byte(exportNDJSON)); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	users, err := NormalizeAuth0Export(&compressed)
	if err != nil {
		t.Fatal(err)
	}
	if len(users) != 3 || users[0].UserID != "auth0|abc" || users[2].UserID != "google-oauth2|1001" {
		t.Fatalf("users are not sorted by Auth0 id: %+v", users)
	}
	linked := users[0]
	if linked.LegacyUserID != "" {
		t.Fatalf("legacy placeholder survived normalization: %q", linked.LegacyUserID)
	}
	if len(linked.Identities) != 2 || linked.Identities[1] != (Auth0Identity{Provider: "google-oauth2", ProviderAccountID: "2002"}) {
		t.Fatalf("linked identities = %+v", linked.Identities)
	}
	blocked := users[1]
	if !blocked.Blocked || len(blocked.Identities) != 1 || blocked.Identities[0] != (Auth0Identity{Provider: "auth0", ProviderAccountID: "blocked"}) {
		t.Fatalf("primary identity was not derived for a record without identities: %+v", blocked)
	}
	if users[2].LegacyUserID != "legacy-google" || users[2].Email != "Google@Example.test" {
		t.Fatalf("google user = %+v", users[2])
	}
}

func TestNormalizeAuth0ExportAcceptsArray(t *testing.T) {
	users, err := NormalizeAuth0Export(strings.NewReader(` [{"user_id":"auth0|one","email":"one@example.test"}]`))
	if err != nil {
		t.Fatal(err)
	}
	if len(users) != 1 || users[0].Identities[0].Provider != "auth0" {
		t.Fatalf("users = %+v", users)
	}
}

func TestNormalizeAuth0ExportRejectsUnsafeInput(t *testing.T) {
	cases := map[string]string{
		"empty":                ``,
		"unsupported provider": `{"user_id":"github|9","identities":[{"provider":"github","user_id":9}]}`,
		"missing primary":      `{"user_id":"auth0|one","identities":[{"provider":"google-oauth2","user_id":"5"}]}`,
		"malformed user id":    `{"user_id":"no-separator"}`,
		"duplicate user":       `{"user_id":"auth0|one"}` + "\n" + `{"user_id":"auth0|one"}`,
		"empty identity id":    `{"user_id":"auth0|one","identities":[{"provider":"auth0","user_id":""}]}`,
	}
	for name, input := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := NormalizeAuth0Export(strings.NewReader(input)); err == nil {
				t.Fatal("unsafe export was accepted")
			}
		})
	}
}
