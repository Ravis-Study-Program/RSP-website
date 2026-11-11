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
	body := `{"name":"Corrected Name","slug":"corrected-name","discordId":"private-discord"}`
	for _, method := range []string{http.MethodGet, http.MethodPatch} {
		res := testRequest(t, handler, method, path, "mentor", body)
		if res.Code != 403 {
			t.Fatalf("non-admin status=%d", res.Code)
		}
	}
	// The person's own Discord link survives an administrator's edit, which
	// cannot change it.
	if _, err := f.pool.Exec(context.Background(), `UPDATE app.user_contacts SET discord_id='private-discord' WHERE user_id=$1`, studentID); err != nil {
		t.Fatal(err)
	}
	res := testRequest(t, handler, http.MethodPatch, path, "admin", strings.Replace(body, "private-discord", "admin-chosen", 1))
	if res.Code != 200 || !strings.Contains(res.Body.String(), "private-discord") || strings.Contains(res.Body.String(), "admin-chosen") {
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
	res = testRequest(t, handler, http.MethodPatch, path, "admin", strings.Replace(body, `"name":`, `"avatarUrl":"https://cdn.discordapp.com/avatars/123/abc.png","name":`, 1))
	if res.Code != 400 {
		t.Fatalf("invalid URL=%d", res.Code)
	}
}

func TestSystemAdminDeletesAccountWithReason(t *testing.T) {
	f := newPostgresFixture(t)
	at := time.Now()
	if _, err := f.pool.Exec(context.Background(), `INSERT INTO app.user_auth_links(auth_subject,user_id,provider,provider_account_id) VALUES('deleted-auth',$1,'test','deleted-auth')`, studentID); err != nil {
		t.Fatal(err)
	}
	var erased []string
	handler := New(Config{DB: f.db, PublicOrigin: "https://rsp.test", Authenticator: AuthenticatorFunc(func(_ context.Context, token string) (authz.Actor, error) {
		actor := authz.Actor{UserID: otherID, EmailVerified: true, AccountState: authz.AccountActive, MFAAt: &at}
		if token == "admin" {
			actor.GlobalRoles = map[authz.GlobalRole]bool{authz.SystemAdmin: true}
		}
		return actor, nil
	}), EraseAccount: func(_ context.Context, authUserID string) error {
		erased = append(erased, authUserID)
		return nil
	}}).Handler()
	path := "/api/v2/admin/users/" + studentID + "/deletion"
	if res := testRequest(t, handler, http.MethodPost, path, "mentor", `{"reason":"Left the program"}`); res.Code != 403 {
		t.Fatalf("non-admin=%d", res.Code)
	}
	if res := testRequest(t, handler, http.MethodPost, path, "admin", `{"reason":" "}`); res.Code != 400 {
		t.Fatalf("blank reason=%d", res.Code)
	}
	if res := testRequest(t, handler, http.MethodPost, "/api/v2/admin/users/"+otherID+"/deletion", "admin", `{"reason":"Mine"}`); res.Code != 409 {
		t.Fatalf("own account=%d", res.Code)
	}
	if len(erased) != 0 {
		t.Fatalf("refused requests reached the auth service: %v", erased)
	}
	if res := testRequest(t, handler, http.MethodPost, path, "admin", `{"reason":"Left the program"}`); res.Code != 204 {
		t.Fatalf("delete=%d %s", res.Code, res.Body)
	}
	if len(erased) != 1 || erased[0] != "deleted-auth" {
		t.Fatalf("erased = %v", erased)
	}
	var reason string
	if err := f.pool.QueryRow(context.Background(), `SELECT data->>'reason' FROM app.audit_events WHERE action='user.deleted' AND subject_id=$1`, studentID).Scan(&reason); err != nil || reason != "Left the program" {
		t.Fatalf("audit reason=%q %v", reason, err)
	}
}
