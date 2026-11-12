//go:build integration

package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Ravis-Study-Program/RSP-website/backend/internal/authadmin"
	"github.com/Ravis-Study-Program/RSP-website/backend/internal/authz"
	"github.com/Ravis-Study-Program/RSP-website/backend/internal/platform/id"
	"github.com/Ravis-Study-Program/RSP-website/backend/internal/platform/ratelimit"
	"github.com/Ravis-Study-Program/RSP-website/backend/internal/program"
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

func TestInvitationBatchInvitesAndChangesExistingRoles(t *testing.T) {
	f := newPostgresFixture(t)
	ctx := context.Background()
	season := id.New()
	now := time.Now()
	if _, err := f.pool.Exec(ctx, `INSERT INTO app.seasons(id,slug,name,start_at,end_at) VALUES($1,'import-season','Import season',now(),now()+interval '1 year')`, season); err != nil {
		t.Fatal(err)
	}
	// Other mentors in the season; Student is already a student in it.
	if _, err := f.pool.Exec(ctx, `INSERT INTO app.enrollments(id,user_id,season_id,role,student_level,state) VALUES($1,$2,$3,'mentor','not_applicable','active'),($4,$5,$3,'student','beginner','active')`, id.New(), otherID, season, id.New(), studentID); err != nil {
		t.Fatal(err)
	}
	var invites []authadmin.InvitationEmailInput
	var roleChanges []authadmin.RoleChangeEmailInput
	admin := authz.Actor{UserID: otherID, EmailVerified: true, AccountState: authz.AccountActive, GlobalRoles: map[authz.GlobalRole]bool{authz.SystemAdmin: true}, MFAAt: &now}
	handler := New(Config{DB: f.db, PublicOrigin: "https://rsp.test", Authenticator: AuthenticatorFunc(func(context.Context, string) (authz.Actor, error) {
		return admin, nil
	}), SendInvitation: func(_ context.Context, email authadmin.InvitationEmailInput) error {
		invites = append(invites, email)
		return nil
	}, SendRoleChange: func(_ context.Context, email authadmin.RoleChangeEmailInput) error {
		roleChanges = append(roleChanges, email)
		return nil
	}}).Handler()

	rows := `{"invitations":[
		{"name":"New Student","email":"new@example.com","role":"student","mentor":"other"},
		{"name":"Student","email":"PRIVATE@example.com","role":"mentor"},
		{"name":"Other","email":"other@example.com","role":"mentor"},
		{"name":"Lost","email":"lost@example.com","role":"student","mentor":"Nobody"},
		{"name":"Bad","email":"not-an-email","role":"student"}
	]}`
	send := func(query string) []invitationBatchResult {
		t.Helper()
		res := testRequest(t, handler, http.MethodPost, "/api/v2/seasons/"+season+"/invitations/batch"+query, "admin", rows)
		if res.Code == http.StatusTooManyRequests {
			t.Fatalf("batch%s was rate limited", query)
		}
		if res.Code != 200 {
			t.Fatalf("batch%s=%d %s", query, res.Code, res.Body)
		}
		var body struct {
			Results []invitationBatchResult `json:"results"`
		}
		if err := json.Unmarshal(res.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		return body.Results
	}
	expect := func(results []invitationBatchResult, outcomes ...string) {
		t.Helper()
		for i, outcome := range outcomes {
			if results[i].Row != i || results[i].Outcome != outcome {
				t.Fatalf("row %d = %+v, want %s", i, results[i], outcome)
			}
		}
	}

	// A dry run says what would happen, and who each email matched, without
	// changing anything or sending email.
	preview := send("/preview")
	expect(preview, "invite", "update", "unchanged", "failed", "failed")
	if preview[0].Person != "new" || preview[1].Person != "member" || len(preview[1].Changes) != 1 || preview[1].Changes[0] != (importChange{"role", "student", "mentor"}) {
		t.Fatalf("dry run should describe the people and changes: %+v", preview)
	}
	if len(invites) != 0 || len(roleChanges) != 0 {
		t.Fatalf("a dry run sent email: %+v %+v", invites, roleChanges)
	}

	// With failing rows in it, nothing is applied: every row is reported as a
	// preview and no email goes out.
	rejected := send("")
	expect(rejected, "invite", "update", "unchanged", "failed", "failed")
	if !strings.Contains(rejected[3].Error, "No mentor called") {
		t.Fatalf("unknown mentor error = %q", rejected[3].Error)
	}
	if len(invites) != 0 || len(roleChanges) != 0 {
		t.Fatalf("a rejected import sent email: %+v %+v", invites, roleChanges)
	}
	var stillStudent string
	if err := f.pool.QueryRow(ctx, `SELECT role::text FROM app.enrollments WHERE season_id=$1 AND user_id=$2`, season, studentID).Scan(&stillStudent); err != nil || stillStudent != "student" {
		t.Fatalf("a rejected import changed a member: %s %v", stillStudent, err)
	}

	rows = `{"invitations":[
		{"name":"New Student","email":"new@example.com","role":"student","mentor":"other"},
		{"name":"Student","email":"PRIVATE@example.com","role":"mentor"},
		{"name":"Other","email":"other@example.com","role":"mentor"}
	]}`
	results := send("")
	expect(results, "invited", "updated", "unchanged")
	body := struct{ Results []invitationBatchResult }{results}
	if m := body.Results[0].Invitation.MentorUserID; m == nil || *m != otherID {
		t.Fatalf("mentor named by name should be matched: %+v", body.Results[0].Invitation)
	}
	if len(invites) != 1 || invites[0].To != "new@example.com" {
		t.Fatalf("invitation emails = %+v", invites)
	}
	if len(roleChanges) != 0 {
		t.Fatalf("a student becoming a mentor should not be emailed: %+v", roleChanges)
	}
	var role, level string
	if err := f.pool.QueryRow(ctx, `SELECT role::text, student_level::text FROM app.enrollments WHERE season_id=$1 AND user_id=$2`, season, studentID).Scan(&role, &level); err != nil || role != "mentor" || level != "not_applicable" {
		t.Fatalf("existing member's role = %s %s %v", role, level, err)
	}

	// One row can change a member's role, mentor and name together.
	rows = `{"invitations":[{"name":"Renamed Student","email":"private@example.com","role":"student","mentor":"other@example.com","slug":"renamed"}]}`
	changed := send("")
	expect(changed, "updated")
	fields := []string{}
	for _, change := range changed[0].Changes {
		fields = append(fields, change.Field)
	}
	if strings.Join(fields, ",") != "role,mentor,name,slug" {
		t.Fatalf("changes = %+v", changed[0].Changes)
	}
	var name, slug, mentor string
	if err := f.pool.QueryRow(ctx, `SELECT p.display_name, p.slug, mentor.user_id FROM app.user_profiles p
		JOIN app.enrollments student ON student.user_id=p.user_id AND student.season_id=$1
		JOIN app.mentorships m ON m.student_enrollment_id=student.id AND m.ended_at IS NULL
		JOIN app.enrollments mentor ON mentor.id=m.mentor_enrollment_id
		WHERE p.user_id=$2`, season, studentID).Scan(&name, &slug, &mentor); err != nil || name != "Renamed Student" || slug != "renamed" || mentor != otherID {
		t.Fatalf("member after update = %q @%s mentor %s, %v", name, slug, mentor, err)
	}

	// A student can name, as their mentor, someone the same sheet makes a
	// mentor further down; the dry run already sees it.
	rows = `{"invitations":[
		{"name":"Kid","email":"kid@example.com","role":"student","mentor":"private@example.com"},
		{"name":"Renamed Student","email":"private@example.com","role":"mentor"}
	]}`
	expect(send("/preview"), "invite", "update")
	sent := send("")
	expect(sent, "invited", "updated")
	if m := sent[0].Invitation.MentorUserID; m == nil || *m != studentID {
		t.Fatalf("student should be invited with the newly made mentor: %+v", sent[0].Invitation)
	}

	// The dry run sees a mentor renamed earlier in the sheet: two mentors
	// would share the name, so naming it is ambiguous, as it is when sent.
	rows = `{"invitations":[
		{"name":"Kid Two","email":"kid2@example.com","role":"student","mentor":"Shared Name"},
		{"name":"Shared Name","email":"other@example.com","role":"mentor"},
		{"name":"Shared Name","email":"private@example.com","role":"mentor"}
	]}`
	ambiguous := send("/preview")
	expect(ambiguous, "failed", "update", "update")
	if !strings.Contains(ambiguous[0].Error, "More than one mentor") {
		t.Fatalf("dry run should find the shared name ambiguous: %+v", ambiguous[0])
	}
	// Sent, the failing row cancels the other two.
	expect(send(""), "failed", "update", "update")
}

func TestSeasonImportCreatesSeasonWithWeeks(t *testing.T) {
	f := newPostgresFixture(t)
	now := time.Now()
	admin := authz.Actor{UserID: otherID, EmailVerified: true, AccountState: authz.AccountActive, GlobalRoles: map[authz.GlobalRole]bool{authz.SystemAdmin: true}, MFAAt: &now}
	handler := New(Config{DB: f.db, PublicOrigin: "https://rsp.test", RateLimits: ratelimit.Limits{Sensitive: 100}, Authenticator: AuthenticatorFunc(func(context.Context, string) (authz.Actor, error) {
		return admin, nil
	})}).Handler()
	post := func(body string) *httptest.ResponseRecorder {
		t.Helper()
		return testRequest(t, handler, http.MethodPost, "/api/v2/seasons/import", "admin", body)
	}
	season := `"name":"Imported 2027","slug":"imported-2027","location":"Adelaide","imageUrl":"","resourcesUrl":""`

	// Overlapping or repeated weeks are refused, and nothing is created.
	if res := post(`{` + season + `,"weeks":[
		{"number":1,"startAt":"2027-02-01T00:00:00Z","endAt":"2027-02-07T23:59:59Z","resourcesHtml":""},
		{"number":2,"startAt":"2027-02-07T00:00:00Z","endAt":"2027-02-14T23:59:59Z","resourcesHtml":""}]}`); res.Code != 400 || !strings.Contains(res.Body.String(), "overlap") {
		t.Fatalf("overlap=%d %s", res.Code, res.Body)
	}
	if res := post(`{` + season + `,"weeks":[
		{"number":1,"startAt":"2027-02-08T00:00:00Z","endAt":"2027-02-14T23:59:59Z","resourcesHtml":""},
		{"number":2,"startAt":"2027-02-01T00:00:00Z","endAt":"2027-02-07T23:59:59Z","resourcesHtml":""}]}`); res.Code != 400 {
		t.Fatalf("out of order=%d %s", res.Code, res.Body)
	}
	if res := post(`{` + season + `,"weeks":[
		{"number":1,"startAt":"2027-02-01T00:00:00Z","endAt":"2027-02-07T23:59:59Z","resourcesHtml":""},
		{"number":1,"startAt":"2027-02-08T00:00:00Z","endAt":"2027-02-14T23:59:59Z","resourcesHtml":""}]}`); res.Code != 400 {
		t.Fatalf("repeated week=%d %s", res.Code, res.Body)
	}

	res := post(`{` + season + `,"weeks":[
		{"number":2,"startAt":"2027-02-08T00:00:00Z","endAt":"2027-02-14T23:59:59Z","resourcesHtml":"<p></p>"},
		{"number":1,"startAt":"2027-02-01T00:00:00Z","endAt":"2027-02-07T23:59:59Z","resourcesHtml":"<p>Arrays</p><script>alert(1)</script>"}]}`)
	if res.Code != 201 {
		t.Fatalf("import=%d %s", res.Code, res.Body)
	}
	var created program.SeasonRecord
	_ = json.Unmarshal(res.Body.Bytes(), &created)
	if !created.StartAt.Equal(time.Date(2027, 2, 1, 0, 0, 0, 0, time.UTC)) || !created.EndAt.Equal(time.Date(2027, 2, 14, 23, 59, 59, 0, time.UTC)) || created.Status != "open" {
		t.Fatalf("season dates should span its weeks: %+v", created)
	}
	rows, err := f.pool.Query(context.Background(), `SELECT week_number, resources_html FROM app.season_weeks WHERE season_id=$1 ORDER BY week_number`, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var got []string
	for rows.Next() {
		var number int
		var html string
		if err := rows.Scan(&number, &html); err != nil {
			t.Fatal(err)
		}
		got = append(got, fmt.Sprintf("%d:%s", number, html))
	}
	if strings.Join(got, "|") != "1:<p>Arrays</p>|2:" {
		t.Fatalf("weeks = %v; resources should be sanitised and blank ones stored empty", got)
	}
	// The same slug updates the season: weeks match by number, a blank
	// resources page keeps the old one, and the season widens to fit.
	week := func(n int, start, end, html string) string {
		return fmt.Sprintf(`{"number":%d,"startAt":"%sT00:00:00Z","endAt":"%sT23:59:59Z","resourcesHtml":%q}`, n, start, end, html)
	}
	update := `"name":"Renamed 2027","slug":"imported-2027","location":"","imageUrl":"","resourcesUrl":""`
	res = post(`{` + update + `,"weeks":[` + week(1, "2027-02-01", "2027-02-07", "") + `,` + week(3, "2027-02-15", "2027-02-21", "<p>New</p>") + `]}`)
	if res.Code != 200 {
		t.Fatalf("update=%d %s", res.Code, res.Body)
	}
	var updated program.SeasonRecord
	_ = json.Unmarshal(res.Body.Bytes(), &updated)
	if updated.ID != created.ID || updated.Name != "Renamed 2027" || updated.Location != "Adelaide" || !updated.EndAt.Equal(time.Date(2027, 2, 21, 23, 59, 59, 0, time.UTC)) {
		t.Fatalf("update should keep the season, rename it and widen its dates: %+v", updated)
	}
	rows2, err := f.pool.Query(context.Background(), `SELECT week_number, resources_html FROM app.season_weeks WHERE season_id=$1 AND deleted_at IS NULL ORDER BY week_number`, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows2.Close()
	got = nil
	for rows2.Next() {
		var number int
		var html string
		if err := rows2.Scan(&number, &html); err != nil {
			t.Fatal(err)
		}
		got = append(got, fmt.Sprintf("%d:%s", number, html))
	}
	if strings.Join(got, "|") != "1:<p>Arrays</p>|2:|3:<p>New</p>" {
		t.Fatalf("weeks = %v", got)
	}
	// A week that would overlap one the import leaves alone is refused.
	if res := post(`{` + update + `,"weeks":[` + week(3, "2027-02-10", "2027-02-16", "") + `]}`); res.Code != 409 {
		t.Fatalf("overlap with an existing week=%d %s", res.Code, res.Body)
	}
	// At most 52 weeks.
	var many []string
	for n := 1; n <= 53; n++ {
		many = append(many, week(n, "2030-01-01", "2030-01-02", ""))
	}
	if res := post(`{` + update + `,"weeks":[` + strings.Join(many, ",") + `]}`); res.Code != 400 {
		t.Fatalf("53 weeks=%d %s", res.Code, res.Body)
	}
	// A closed season cannot be updated.
	if _, err := f.pool.Exec(context.Background(), `UPDATE app.seasons SET status='closed', closed_at=now() WHERE id=$1`, created.ID); err != nil {
		t.Fatal(err)
	}
	if res := post(`{` + update + `,"weeks":[` + week(1, "2027-02-01", "2027-02-07", "") + `]}`); res.Code != 409 {
		t.Fatalf("closed season=%d %s", res.Code, res.Body)
	}
}

func TestInvitationUsernameAppliesToNewAccounts(t *testing.T) {
	f := newPostgresFixture(t)
	ctx := context.Background()
	season := id.New()
	now := time.Now()
	if _, err := f.pool.Exec(ctx, `INSERT INTO app.seasons(id,slug,name,start_at,end_at) VALUES($1,'slug-season','Slug season',now(),now()+interval '1 year')`, season); err != nil {
		t.Fatal(err)
	}
	var emails []authadmin.InvitationEmailInput
	admin := authz.Actor{UserID: otherID, EmailVerified: true, AccountState: authz.AccountActive, GlobalRoles: map[authz.GlobalRole]bool{authz.SystemAdmin: true}, MFAAt: &now}
	handler := New(Config{DB: f.db, PublicOrigin: "https://rsp.test", Authenticator: AuthenticatorFunc(func(_ context.Context, token string) (authz.Actor, error) {
		v := admin
		if token == "student" {
			v.UserID, v.GlobalRoles = studentID, nil
		}
		return v, nil
	}), SendInvitation: func(_ context.Context, email authadmin.InvitationEmailInput) error {
		emails = append(emails, email)
		return nil
	}}).Handler()
	res := testRequest(t, handler, http.MethodPost, "/api/v2/seasons/"+season+"/invitations/batch", "admin", `{"invitations":[
		{"name":"Student","email":"private@example.com","role":"student","slug":"Chosen-Name"},
		{"name":"Taken","email":"taken@example.com","role":"student","slug":"other"},
		{"name":"Bad","email":"bad@example.com","role":"student","slug":"no spaces"}
	]}`)
	var body struct {
		Results []invitationBatchResult `json:"results"`
	}
	_ = json.Unmarshal(res.Body.Bytes(), &body)
	// The taken and malformed usernames fail, so the import saves nothing.
	if res.Code != 200 || body.Results[0].Outcome != "invite" || len(emails) != 0 ||
		body.Results[1].Outcome != "failed" || body.Results[2].Outcome != "failed" {
		t.Fatalf("batch=%d %s", res.Code, res.Body)
	}
	res = testRequest(t, handler, http.MethodPost, "/api/v2/seasons/"+season+"/invitations/batch", "admin", `{"invitations":[
		{"name":"Student","email":"private@example.com","role":"student","slug":"Chosen-Name"}
	]}`)
	body.Results = nil
	_ = json.Unmarshal(res.Body.Bytes(), &body)
	if res.Code != 200 || len(body.Results) != 1 || body.Results[0].Outcome != "invited" || *body.Results[0].Invitation.ProfileSlug != "chosen-name" {
		t.Fatalf("batch=%d %s", res.Code, res.Body)
	}
	// The fixture's student account predates the invitation, so its username
	// stays; an account made after it takes the invited username.
	accept := func() {
		t.Helper()
		u, _ := url.Parse(emails[0].URL)
		if res := testRequest(t, handler, http.MethodPost, "/api/v2/invitations/accept", "student", fmt.Sprintf(`{"token":%q}`, strings.TrimPrefix(u.Fragment, "token="))); res.Code != 200 {
			t.Fatalf("accept=%d %s", res.Code, res.Body)
		}
	}
	if _, err := f.pool.Exec(ctx, `UPDATE app.users SET created_at=now()+interval '1 minute' WHERE id=$1`, studentID); err != nil {
		t.Fatal(err)
	}
	accept()
	var slug string
	if err := f.pool.QueryRow(ctx, `SELECT slug FROM app.user_profiles WHERE user_id=$1`, studentID).Scan(&slug); err != nil || slug != "chosen-name" {
		t.Fatalf("slug after accepting=%q %v", slug, err)
	}
}

func TestInvitationBatchStudentCanNameMentorInvitedInTheSameBatch(t *testing.T) {
	f := newPostgresFixture(t)
	ctx := context.Background()
	season := id.New()
	now := time.Now()
	if _, err := f.pool.Exec(ctx, `INSERT INTO app.seasons(id,slug,name,start_at,end_at) VALUES($1,'together-season','Together season',now(),now()+interval '1 year')`, season); err != nil {
		t.Fatal(err)
	}
	var emails []authadmin.InvitationEmailInput
	admin := authz.Actor{UserID: otherID, EmailVerified: true, AccountState: authz.AccountActive, GlobalRoles: map[authz.GlobalRole]bool{authz.SystemAdmin: true}, MFAAt: &now}
	handler := New(Config{DB: f.db, PublicOrigin: "https://rsp.test", Authenticator: AuthenticatorFunc(func(_ context.Context, token string) (authz.Actor, error) {
		v := admin
		if token == "student" {
			v.UserID, v.GlobalRoles = studentID, nil
		}
		return v, nil
	}), SendInvitation: func(_ context.Context, email authadmin.InvitationEmailInput) error {
		emails = append(emails, email)
		return nil
	}}).Handler()
	// The student comes first in the sheet, but names the mentor invited below.
	rows := `{"invitations":[
		{"name":"Student","email":"private@example.com","role":"student","mentor":"other@example.com"},
		{"name":"Mentor","email":"other@example.com","role":"mentor"}
	]}`
	post := func(path string) []invitationBatchResult {
		t.Helper()
		res := testRequest(t, handler, http.MethodPost, "/api/v2/seasons/"+season+"/invitations/batch"+path, "admin", rows)
		var body struct {
			Results []invitationBatchResult `json:"results"`
		}
		_ = json.Unmarshal(res.Body.Bytes(), &body)
		if res.Code != 200 || len(body.Results) != 2 || body.Results[0].Outcome == "failed" || body.Results[1].Outcome == "failed" {
			t.Fatalf("batch%s=%d %s", path, res.Code, res.Body)
		}
		return body.Results
	}
	// By full name too, and a name two people share is an error.
	byName := rows
	rows = `{"invitations":[
		{"name":"Student","email":"private@example.com","role":"student","mentor":"The Mentor"},
		{"name":"The Mentor","email":"other@example.com","role":"mentor"}
	]}`
	post("/preview")
	rows = `{"invitations":[
		{"name":"Kid","email":"kid@example.com","role":"student","mentor":"Twin"},
		{"name":"Twin","email":"twin1@example.com","role":"mentor"},
		{"name":"Twin","email":"twin2@example.com","role":"mentor"}
	]}`
	res := testRequest(t, handler, http.MethodPost, "/api/v2/seasons/"+season+"/invitations/batch/preview", "admin", rows)
	if !strings.Contains(res.Body.String(), "More than one mentor") {
		t.Fatalf("ambiguous name should be an error: %s", res.Body)
	}
	// Several mentors invited together, named by slug or by full name.
	rows = `{"invitations":[
		{"name":"Maged","email":"maged@example.com","role":"student","mentor":"madz1"},
		{"name":"Student 1","email":"s1@example.com","role":"student","mentor":"Maddie Mentor 2"},
		{"name":"Student 2","email":"s2@example.com","role":"student","mentor":"Maddie Mentor 2"},
		{"name":"Maddie Mentor 1","slug":"madz1","email":"mentor1@example.com","role":"mentor"},
		{"name":"Maddie Mentor 2","email":"mentor2@example.com","role":"mentor"},
		{"name":"MM","email":"mm@example.com","role":"mentor"}
	]}`
	res = testRequest(t, handler, http.MethodPost, "/api/v2/seasons/"+season+"/invitations/batch/preview", "admin", rows)
	if strings.Contains(res.Body.String(), `"failed"`) {
		t.Fatalf("mentors named by slug or name should resolve: %s", res.Body)
	}
	// A mentor listed on several rows is still one mentor.
	rows = `{"invitations":[
		{"name":"Kid","email":"kid@example.com","role":"student","mentor":"Maddie Mentor 2"},
		{"name":"Maddie Mentor 2","email":"mentor2@example.com","role":"mentor"},
		{"name":"Maddie Mentor 2","email":"mentor2@example.com","role":"mentor"},
		{"name":"Maddie Mentor 2","email":"mentor2@example.com","role":"mentor"}
	]}`
	res = testRequest(t, handler, http.MethodPost, "/api/v2/seasons/"+season+"/invitations/batch/preview", "admin", rows)
	if strings.Contains(res.Body.String(), "More than one") || strings.Contains(res.Body.String(), `"failed"`) {
		t.Fatalf("a repeated mentor row should not be ambiguous: %s", res.Body)
	}
	rows = byName
	post("/preview")
	post("")
	// Each accepts with the token emailed to them; the student first, so the
	// mentorship waits for the mentor.
	accept := func(token, to string) {
		t.Helper()
		for _, email := range emails {
			if email.To != to {
				continue
			}
			u, _ := url.Parse(email.URL)
			if res := testRequest(t, handler, http.MethodPost, "/api/v2/invitations/accept", token, fmt.Sprintf(`{"token":%q}`, strings.TrimPrefix(u.Fragment, "token="))); res.Code != 200 {
				t.Fatalf("accept %s=%d %s", to, res.Code, res.Body)
			}
			return
		}
		t.Fatalf("no invitation email for %s", to)
	}
	mentorships := func() int {
		var n int
		if err := f.pool.QueryRow(ctx, `SELECT count(*) FROM app.mentorships m JOIN app.enrollments mentor ON mentor.id=m.mentor_enrollment_id
			JOIN app.enrollments student ON student.id=m.student_enrollment_id
			WHERE m.season_id=$1 AND mentor.user_id=$2 AND student.user_id=$3 AND m.ended_at IS NULL`, season, otherID, studentID).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	accept("student", "private@example.com")
	if mentorships() != 0 {
		t.Fatal("mentorship made before the mentor accepted")
	}
	accept("admin", "other@example.com")
	if mentorships() != 1 {
		t.Fatal("mentorship not made once the mentor accepted")
	}
}

func TestInvitationBatchIsAllOrNothing(t *testing.T) {
	f := newPostgresFixture(t)
	ctx := context.Background()
	season := id.New()
	now := time.Now()
	if _, err := f.pool.Exec(ctx, `INSERT INTO app.seasons(id,slug,name,start_at,end_at) VALUES($1,'atomic-season','Atomic season',now(),now()+interval '1 year')`, season); err != nil {
		t.Fatal(err)
	}
	var emails []authadmin.InvitationEmailInput
	admin := authz.Actor{UserID: otherID, EmailVerified: true, AccountState: authz.AccountActive, GlobalRoles: map[authz.GlobalRole]bool{authz.SystemAdmin: true}, MFAAt: &now}
	handler := New(Config{DB: f.db, PublicOrigin: "https://rsp.test", Authenticator: AuthenticatorFunc(func(_ context.Context, _ string) (authz.Actor, error) {
		return admin, nil
	}), SendInvitation: func(_ context.Context, email authadmin.InvitationEmailInput) error {
		emails = append(emails, email)
		return nil
	}}).Handler()
	count := func() int {
		var n int
		if err := f.pool.QueryRow(ctx, `SELECT count(*) FROM app.season_invitations WHERE season_id=$1`, season).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	post := func(rows string) (bool, []invitationBatchResult) {
		t.Helper()
		res := testRequest(t, handler, http.MethodPost, "/api/v2/seasons/"+season+"/invitations/batch", "admin", rows)
		var body struct {
			Applied bool                    `json:"applied"`
			Results []invitationBatchResult `json:"results"`
		}
		_ = json.Unmarshal(res.Body.Bytes(), &body)
		if res.Code != 200 {
			t.Fatalf("batch=%d %s", res.Code, res.Body)
		}
		return body.Applied, body.Results
	}
	// One good row and one bad one: nothing is saved and nothing is emailed.
	applied, results := post(`{"invitations":[
		{"name":"Good","email":"good@example.com","role":"student"},
		{"name":"Bad","email":"bad@example.com","role":"teacher"}
	]}`)
	if applied || count() != 0 || len(emails) != 0 || results[0].Outcome != "invite" || results[1].Outcome != "failed" {
		t.Fatalf("rejected import applied=%v invitations=%d emails=%d results=%+v", applied, count(), len(emails), results)
	}
	// Without the bad row, both are saved and emailed.
	applied, _ = post(`{"invitations":[
		{"name":"Good","email":"good@example.com","role":"student"},
		{"name":"Also good","email":"also@example.com","role":"mentor"}
	]}`)
	if !applied || count() != 2 || len(emails) != 2 {
		t.Fatalf("import applied=%v invitations=%d emails=%d", applied, count(), len(emails))
	}
}
