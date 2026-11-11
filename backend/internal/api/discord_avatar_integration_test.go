//go:build integration

package api

import (
	"context"
	"fmt"
	"net/http"
	"testing"
)

func TestDiscordAvatarSyncAndWriteProtection(t *testing.T) {
	f := newPostgresFixture(t)
	const token = "discord-avatar-test-service-token-0000"
	t.Setenv("IDENTITY_SERVICE_TOKEN", token)
	_, err := f.pool.Exec(context.Background(), `INSERT INTO app.user_auth_links(auth_subject,user_id,provider,provider_account_id) VALUES('discord-test',$1,'better_auth','discord-test')`, studentID)
	if err != nil {
		t.Fatal(err)
	}
	avatar := `https://cdn.discordapp.com/avatars/123456789/0123456789abcdef0123456789abcdef.png?size=256`
	sync := func(id, timestamp, value string, status int) {
		t.Helper()
		body := fmt.Sprintf(`{"eventId":%q,"type":"discord_avatar_updated","authUserId":"discord-test","securityVersion":1,"occurredAt":%q,"avatarUrl":%s}`, id, timestamp, value)
		res := testRequest(t, f.handler, http.MethodPost, "/internal/auth/lifecycle-events", token, body)
		if res.Code != status {
			t.Fatalf("sync=%d %s", res.Code, res.Body)
		}
	}
	sync("00000000-0000-7000-8000-000000000901", "2026-11-08T01:00:00Z", fmt.Sprintf("%q", avatar), 204)
	var actual *string
	check := func(want *string) {
		t.Helper()
		if err := f.pool.QueryRow(context.Background(), `SELECT avatar_url FROM app.user_profiles WHERE user_id=$1`, studentID).Scan(&actual); err != nil {
			t.Fatal(err)
		}
		if (actual == nil) != (want == nil) || (actual != nil && *actual != *want) {
			t.Fatalf("unexpected avatar: %v", actual)
		}
	}
	check(&avatar)
	// An ordinary profile edit preserves the synchronized avatar.
	res := testRequest(t, f.handler, http.MethodPatch, "/api/v2/me", "student", `{"name":"Changed Name","timezone":"Australia/Adelaide"}`)
	if res.Code != 200 {
		t.Fatalf("profile edit=%d %s", res.Code, res.Body)
	}
	check(&avatar)
	// Even a Discord CDN URL cannot be supplied by a browser.
	res = testRequest(t, f.handler, http.MethodPatch, "/api/v2/me", "student", fmt.Sprintf(`{"name":"Changed Name","timezone":"Australia/Adelaide","avatarUrl":%q}`, avatar))
	if res.Code != 400 {
		t.Fatalf("arbitrary avatar write=%d", res.Code)
	}
	sync("00000000-0000-7000-8000-000000000902", "2026-11-08T02:00:00Z", "null", 204)
	check(nil)
	// Retried/delayed login events must not resurrect a disconnected avatar.
	sync("00000000-0000-7000-8000-000000000903", "2026-11-08T01:30:00Z", fmt.Sprintf("%q", avatar), 204)
	check(nil)
	sync("00000000-0000-7000-8000-000000000904", "2026-11-08T03:00:00Z", `"https://example.com/arbitrary.png"`, 400)
	check(nil)
	// A later connection can set a photo again; replaying the old disconnect
	// still cannot clear it.
	sync("00000000-0000-7000-8000-000000000905", "2026-11-08T04:00:00Z", fmt.Sprintf("%q", avatar), 204)
	check(&avatar)
	sync("00000000-0000-7000-8000-000000000902", "2026-11-08T02:00:00Z", "null", 204)
	check(&avatar)
}
