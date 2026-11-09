//go:build integration

package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/magedmg/RSP-website/backend/internal/authadmin"
	"github.com/magedmg/RSP-website/backend/internal/authz"
	"github.com/magedmg/RSP-website/backend/internal/platform/id"
	"github.com/magedmg/RSP-website/backend/internal/program"
)

func TestInvitationLifecycleAndVerifiedEmail(t *testing.T) {
	f := newPostgresFixture(t)
	ctx := context.Background()
	newSeason := id.New()
	now := time.Now()
	if _, err := f.pool.Exec(ctx, `INSERT INTO app.seasons(id,slug,name,start_at,end_at) VALUES($1,'invited-season','Invited season',now(),now()+interval '1 year')`, newSeason); err != nil {
		t.Fatal(err)
	}
	var emails []authadmin.InvitationEmailInput
	actor := authz.Actor{UserID: otherID, EmailVerified: true, AccountState: authz.AccountActive, GlobalRoles: map[authz.GlobalRole]bool{authz.SystemAdmin: true}, MFAAt: &now}
	handler := New(Config{DB: f.db, PublicOrigin: "https://rsp.test", Authenticator: AuthenticatorFunc(func(_ context.Context, token string) (authz.Actor, error) {
		v := actor
		if token == "student" {
			v.UserID = studentID
			v.GlobalRoles = nil
		}
		return v, nil
	}), SendInvitation: func(_ context.Context, email authadmin.InvitationEmailInput) error {
		emails = append(emails, email)
		return nil
	}}).Handler()
	create := func() program.Invitation {
		t.Helper()
		res := testRequest(t, handler, http.MethodPost, "/api/v2/seasons/"+newSeason+"/invitations", "admin", `{"name":"New member","email":"PRIVATE@example.com","role":"coordinator"}`)
		var v program.Invitation
		if err := json.Unmarshal(res.Body.Bytes(), &v); err != nil || res.Code != 201 || v.SentAt == nil {
			t.Fatalf("create=%d %s %v", res.Code, res.Body, err)
		}
		return v
	}
	tokenBody := func(index int) string {
		u, err := url.Parse(emails[index].URL)
		if err != nil {
			t.Fatal(err)
		}
		return fmt.Sprintf(`{"token":%q}`, strings.TrimPrefix(u.Fragment, "token="))
	}
	v := create()
	var users, members int
	if err := f.pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM app.users),(SELECT count(*) FROM app.enrollments WHERE season_id=$1)`, newSeason).Scan(&users, &members); err != nil || users != 2 || members != 0 {
		t.Fatalf("inviting precreated records: users=%d members=%d %v", users, members, err)
	}
	res := testRequest(t, handler, http.MethodPost, "/api/v2/invitations/preview", "admin", tokenBody(0))
	if res.Code != 404 {
		t.Fatalf("wrong email=%d", res.Code)
	}
	res = testRequest(t, handler, http.MethodPost, "/api/v2/seasons/"+newSeason+"/invitations/"+v.ID+"/resend", "admin", "")
	if res.Code != 200 || len(emails) != 2 {
		t.Fatalf("resend=%d %s", res.Code, res.Body)
	}
	res = testRequest(t, handler, http.MethodPost, "/api/v2/invitations/accept", "student", tokenBody(0))
	if res.Code != 404 {
		t.Fatalf("old token=%d", res.Code)
	}
	res = testRequest(t, handler, http.MethodPost, "/api/v2/invitations/preview", "student", tokenBody(1))
	if res.Code != 200 {
		t.Fatalf("preview=%d %s", res.Code, res.Body)
	}
	res = testRequest(t, handler, http.MethodPost, "/api/v2/seasons/"+newSeason+"/invitations/"+v.ID+"/cancel", "admin", "")
	if res.Code != 200 {
		t.Fatalf("cancel=%d %s", res.Code, res.Body)
	}
	res = testRequest(t, handler, http.MethodPost, "/api/v2/invitations/accept", "student", tokenBody(1))
	if res.Code != 404 {
		t.Fatalf("cancelled=%d", res.Code)
	}
	v = create()
	if _, err := f.pool.Exec(ctx, `UPDATE app.season_invitations SET expires_at=now()-interval '1 second' WHERE id=$1`, v.ID); err != nil {
		t.Fatal(err)
	}
	res = testRequest(t, handler, http.MethodPost, "/api/v2/invitations/accept", "student", tokenBody(2))
	if res.Code != 404 {
		t.Fatalf("expired=%d", res.Code)
	}
	res = testRequest(t, handler, http.MethodPost, "/api/v2/seasons/"+newSeason+"/invitations/"+v.ID+"/resend", "admin", "")
	if res.Code != 200 {
		t.Fatalf("renew=%d %s", res.Code, res.Body)
	}
	// Two simultaneous acceptances must only create one membership.
	var wg sync.WaitGroup
	statuses := make(chan int, 2)
	body := tokenBody(3)
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			statuses <- testRequest(t, handler, http.MethodPost, "/api/v2/invitations/accept", "student", body).Code
		}()
	}
	wg.Wait()
	close(statuses)
	successes := 0
	for status := range statuses {
		if status == 200 {
			successes++
		} else if status != 404 {
			t.Fatalf("concurrent status=%d", status)
		}
	}
	if successes != 1 {
		t.Fatalf("accept successes=%d", successes)
	}
	var state string
	if err := f.pool.QueryRow(ctx, `SELECT assignment_state::text FROM app.enrollments WHERE season_id=$1 AND user_id=$2`, newSeason, studentID).Scan(&state); err != nil || state != "pending_mfa" {
		t.Fatalf("coordinator activation=%s %v", state, err)
	}
	res = testRequest(t, handler, http.MethodGet, "/api/v2/seasons/"+newSeason+"/invitations", "admin", "")
	if res.Code != 200 || !strings.Contains(res.Body.String(), `"accepted"`) || strings.Contains(res.Body.String(), "token") {
		t.Fatalf("history=%d %s", res.Code, res.Body)
	}
}

func TestInvitationWithMentorAssignsMentorOnAcceptance(t *testing.T) {
	f := newPostgresFixture(t)
	ctx := context.Background()
	season := id.New()
	now := time.Now()
	if _, err := f.pool.Exec(ctx, `INSERT INTO app.seasons(id,slug,name,start_at,end_at) VALUES($1,'mentored-season','Mentored season',now(),now()+interval '1 year')`, season); err != nil {
		t.Fatal(err)
	}
	if _, err := f.pool.Exec(ctx, `INSERT INTO app.enrollments(id,user_id,season_id,role,student_level,state) VALUES($1,$2,$3,'mentor','not_applicable','active')`, id.New(), otherID, season); err != nil {
		t.Fatal(err)
	}
	var emails []authadmin.InvitationEmailInput
	admin := authz.Actor{UserID: otherID, EmailVerified: true, AccountState: authz.AccountActive, GlobalRoles: map[authz.GlobalRole]bool{authz.SystemAdmin: true}, MFAAt: &now}
	handler := New(Config{DB: f.db, PublicOrigin: "https://rsp.test", Authenticator: AuthenticatorFunc(func(_ context.Context, token string) (authz.Actor, error) {
		v := admin
		if token == "student" {
			v.UserID = studentID
			v.GlobalRoles = nil
		}
		return v, nil
	}), SendInvitation: func(_ context.Context, email authadmin.InvitationEmailInput) error {
		emails = append(emails, email)
		return nil
	}}).Handler()
	invite := func(body string) (int, program.Invitation) {
		t.Helper()
		res := testRequest(t, handler, http.MethodPost, "/api/v2/seasons/"+season+"/invitations", "admin", body)
		var v program.Invitation
		_ = json.Unmarshal(res.Body.Bytes(), &v)
		return res.Code, v
	}

	// Only students are invited with a mentor, and only to someone mentoring
	// in the season.
	if code, _ := invite(`{"name":"New mentor","email":"mentor@example.com","role":"mentor","mentorUserId":"` + otherID + `"}`); code != 400 {
		t.Fatalf("mentor invited with a mentor=%d", code)
	}
	if code, _ := invite(`{"name":"New student","email":"private@example.com","role":"student","mentorUserId":"` + studentID + `"}`); code != 400 {
		t.Fatalf("non-mentor accepted as mentor=%d", code)
	}
	if code, _ := invite(`{"name":"New student","email":"private@example.com","role":"student","mentorUserId":"not-a-user"}`); code != 400 {
		t.Fatalf("malformed mentor=%d", code)
	}

	code, v := invite(`{"name":"New student","email":"private@example.com","role":"student","mentorUserId":"` + otherID + `"}`)
	if code != 201 || v.MentorUserID == nil || *v.MentorUserID != otherID {
		t.Fatalf("invite with mentor=%d %+v", code, v)
	}
	u, err := url.Parse(emails[len(emails)-1].URL)
	if err != nil {
		t.Fatal(err)
	}
	body := fmt.Sprintf(`{"token":%q}`, strings.TrimPrefix(u.Fragment, "token="))
	if res := testRequest(t, handler, http.MethodPost, "/api/v2/invitations/accept", "student", body); res.Code != 200 {
		t.Fatalf("accept=%d %s", res.Code, res.Body)
	}
	var mentor string
	if err := f.pool.QueryRow(ctx, `SELECT mentor.user_id FROM app.mentorships m
		JOIN app.enrollments mentor ON mentor.id=m.mentor_enrollment_id
		JOIN app.enrollments student ON student.id=m.student_enrollment_id
		WHERE m.season_id=$1 AND student.user_id=$2 AND m.ended_at IS NULL`, season, studentID).Scan(&mentor); err != nil || mentor != otherID {
		t.Fatalf("mentorship after acceptance=%q %v", mentor, err)
	}
	res := testRequest(t, handler, http.MethodGet, "/api/v2/seasons/"+season+"/invitations", "admin", "")
	if res.Code != 200 || !strings.Contains(res.Body.String(), `"acceptedBy":"`+studentID+`"`) {
		t.Fatalf("accepted invitation should name its account: %d %s", res.Code, res.Body)
	}
}
