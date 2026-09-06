//go:build integration

package api

import (
	"context"
	"github.com/magedmg/RSP-website/backend/internal/authadmin"
	"github.com/magedmg/RSP-website/backend/internal/authz"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestAdminProfileCorrectionKeepsEmailUntilVerified(t *testing.T) {
	f := newPostgresFixture(t)
	at := time.Now()
	requested := ""
	if _, err := f.pool.Exec(context.Background(), `INSERT INTO app.user_auth_links(auth_subject,user_id,provider,provider_account_id) VALUES('profile-auth',$1,'test','profile-auth')`, studentID); err != nil {
		t.Fatal(err)
	}
	handler := New(Config{DB: f.db, Authenticator: AuthenticatorFunc(func(_ context.Context, token string) (authz.Actor, error) {
		actor := authz.Actor{UserID: otherID, EmailVerified: true, AccountState: authz.AccountActive, MFAAt: &at}
		if token == "admin" {
			actor.GlobalRoles = map[authz.GlobalRole]bool{authz.SystemAdmin: true}
		}
		return actor, nil
	}), RequestEmailChange: func(_ context.Context, input authadmin.EmailChangeInput) error {
		if input.AuthUserID != "profile-auth" {
			t.Fatalf("wrong identity=%s", input.AuthUserID)
		}
		requested = input.Email
		return nil
	}}).Handler()
	path := "/api/v2/admin/users/" + studentID + "/profile"
	body := `{"name":"Corrected Name","slug":"corrected-name","avatarUrl":"https://example.test/avatar.png","discordId":"private-discord"}`
	for _, method := range []string{http.MethodGet, http.MethodPatch} {
		res := testRequest(t, handler, method, path, "mentor", body)
		if res.Code != 403 {
			t.Fatalf("non-admin status=%d", res.Code)
		}
	}
	res := testRequest(t, handler, http.MethodPatch, path, "admin", body)
	if res.Code != 200 || !strings.Contains(res.Body.String(), "private-discord") {
		t.Fatalf("edit=%d %s", res.Code, res.Body)
	}
	res = testRequest(t, handler, http.MethodPost, "/api/v2/admin/users/"+studentID+"/email-change", "admin", `{"email":"new@example.test"}`)
	if res.Code != 202 || requested != "new@example.test" {
		t.Fatalf("email request=%d %s", res.Code, res.Body)
	}
	res = testRequest(t, handler, http.MethodGet, path, "admin", "")
	if res.Code != 200 || !strings.Contains(res.Body.String(), "private@example.com") || strings.Contains(res.Body.String(), "new@example.test") {
		t.Fatalf("email prematurely changed=%d %s", res.Code, res.Body)
	}
	res = testRequest(t, f.handler, http.MethodGet, "/api/v2/users/"+studentID, "other", "")
	if strings.Contains(res.Body.String(), "private-discord") {
		t.Fatalf("contact leaked=%s", res.Body)
	}
	res = testRequest(t, handler, http.MethodPatch, path, "admin", strings.Replace(body, "corrected-name", "other", 1))
	if res.Code != 409 {
		t.Fatalf("duplicate slug=%d %s", res.Code, res.Body)
	}
	res = testRequest(t, handler, http.MethodPatch, path, "admin", strings.Replace(body, "https://example.test/avatar.png", "javascript:alert(1)", 1))
	if res.Code != 400 {
		t.Fatalf("invalid URL=%d", res.Code)
	}
}
