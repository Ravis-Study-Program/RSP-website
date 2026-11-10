//go:build integration

package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/magedmg/RSP-website/backend/internal/accounts"
	"github.com/magedmg/RSP-website/backend/internal/authz"
	"github.com/magedmg/RSP-website/backend/internal/jobs"
	"github.com/magedmg/RSP-website/backend/internal/platform/ratelimit"
)

func jobFixture(t *testing.T) (postgresFixture, http.Handler) {
	f := newPostgresFixture(t)
	now := time.Now()
	h := New(Config{DB: f.db, CursorSecret: []byte("jobs-integration-cursor-secret"), RateLimits: ratelimit.Limits{Read: 1000, Write: 1000}, Authenticator: AuthenticatorFunc(func(_ context.Context, token string) (authz.Actor, error) {
		actor := authz.Actor{UserID: studentID, EmailVerified: true, AccountState: authz.AccountActive}
		switch token {
		case "unverified":
			actor.EmailVerified = false
		case "suspended":
			actor.AccountState = authz.Suspended
		case "mentor":
			actor.UserID = otherID
			actor.Enrollments = []authz.Enrollment{{SeasonID: seasonID, Role: authz.Mentor, State: authz.Active}}
		case "coordinator", "stale-coordinator", "former-coordinator":
			actor.UserID = otherID
			actor.Enrollments = []authz.Enrollment{{SeasonID: seasonID, Role: authz.Coordinator, State: authz.Active}}
			actor.MFAAt = &now
			if token == "stale-coordinator" {
				actor.MFAAt = nil
			}
			if token == "former-coordinator" {
				actor.Enrollments[0].State = authz.Completed
			}
		case "director", "system_admin":
			actor.UserID = otherID
			actor.GlobalRoles = map[authz.GlobalRole]bool{authz.GlobalRole(token): true}
			actor.MFAAt = &now
		case "member": // Deliberately no enrollment: verified accounts can contribute.
		default:
			return actor, fmt.Errorf("unknown actor")
		}
		return actor, nil
	})}).Handler()
	return f, h
}

func jobCall(t *testing.T, h http.Handler, method, path, token string, input any, want int) *httptest.ResponseRecorder {
	t.Helper()
	body := ""
	if input != nil {
		encoded, err := json.Marshal(input)
		if err != nil {
			t.Fatal(err)
		}
		body = string(encoded)
	}
	res := testRequest(t, h, method, "/api/v2"+path, token, body)
	if res.Code != want {
		t.Fatalf("%s %s (%s): got %d want %d: %s", method, path, token, res.Code, want, res.Body.String())
	}
	return res
}

func newJob(t *testing.T, h http.Handler, suffix, location string) jobs.Position {
	t.Helper()
	input := jobs.PositionInput{CompanyName: "Acme", Category: "big_tech", Title: "Software Engineer " + suffix, JobType: "grad", ApplicationURL: "https://jobs.example.com/" + suffix, Locations: []string{location}}
	res := jobCall(t, h, "POST", "/jobs/positions", "member", input, http.StatusCreated)
	var p jobs.Position
	if err := json.Unmarshal(res.Body.Bytes(), &p); err != nil {
		t.Fatal(err)
	}
	return p
}

func applyJob(t *testing.T, h http.Handler, token, positionID string) string {
	t.Helper()
	res := jobCall(t, h, "POST", "/me/job-applications", token, map[string]string{"positionId": positionID, "date": "2026-11-01"}, http.StatusOK)
	var v struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &v); err != nil {
		t.Fatal(err)
	}
	return v.ID
}

func readApplications(t *testing.T, h http.Handler, token string) []jobs.Application {
	t.Helper()
	res := jobCall(t, h, "GET", "/me/job-applications", token, nil, http.StatusOK)
	var page Page[jobs.Application]
	if err := json.Unmarshal(res.Body.Bytes(), &page); err != nil {
		t.Fatal(err)
	}
	return page.Items
}

func TestJobsPublicBoardAndPrivateOwnership(t *testing.T) {
	_, h := jobFixture(t)
	p := newJob(t, h, "one", "Sydney")
	if p.CreatedAt.IsZero() {
		t.Fatal("position must expose its added date")
	}
	public := jobCall(t, h, "GET", "/jobs/positions", "", nil, http.StatusOK)
	if !strings.Contains(public.Body.String(), p.Title) || strings.Contains(public.Body.String(), studentID) {
		t.Fatalf("public response: %s", public.Body)
	}
	jobCall(t, h, "POST", "/jobs/positions", "", jobs.PositionInput{}, http.StatusUnauthorized)
	for _, token := range []string{"unverified", "suspended"} {
		jobCall(t, h, "POST", "/jobs/positions", token, jobs.PositionInput{}, http.StatusForbidden)
	}
	input := jobs.PositionInput{CompanyName: p.Company.Name, Category: p.Company.Category, Title: p.Title, JobType: p.JobType, ApplicationURL: p.ApplicationURL, Locations: p.Locations, Status: "closed"}
	for _, token := range []string{"member", "mentor", "former-coordinator", "stale-coordinator"} {
		jobCall(t, h, "PATCH", "/jobs/positions/"+p.ID, token, input, http.StatusForbidden)
	}
	for _, token := range []string{"coordinator", "director", "system_admin"} {
		res := jobCall(t, h, "PATCH", "/jobs/positions/"+p.ID, token, input, http.StatusOK)
		var updated jobs.Position
		if err := json.Unmarshal(res.Body.Bytes(), &updated); err != nil {
			t.Fatal(err)
		}
		if !updated.CreatedAt.Equal(p.CreatedAt) {
			t.Fatal("editing or closing a position changed its added date")
		}
	}
	appID := applyJob(t, h, "member", p.ID)
	if again := applyJob(t, h, "member", p.ID); again != appID {
		t.Fatal("Applied created duplicate application")
	}
	apps := readApplications(t, h, "member")
	if !apps[0].Position.CreatedAt.Equal(p.CreatedAt) {
		t.Fatal("application must retain the position's added date")
	}
	initial := apps[0].Stages[0]
	note := jobs.StageInput{Title: "Applied", Date: "2026-11-01", Notes: "private application note"}
	jobCall(t, h, "PATCH", "/me/job-applications/"+appID+"/stages/"+initial.ID, "member", note, http.StatusOK)
	custom := jobs.StageInput{Title: "System design with team", Date: "2026-11-05", Notes: "private interview note"}
	jobCall(t, h, "POST", "/me/job-applications/"+appID+"/stages", "member", custom, http.StatusCreated)
	for _, token := range []string{"mentor", "coordinator", "director", "system_admin"} {
		if len(readApplications(t, h, token)) != 0 {
			t.Fatalf("%s read someone else's application", token)
		}
		jobCall(t, h, "POST", "/me/job-applications/"+appID+"/stages", token, custom, http.StatusNotFound)
		jobCall(t, h, "PATCH", "/me/job-applications/"+appID+"/stages/"+initial.ID, token, note, http.StatusNotFound)
		jobCall(t, h, "DELETE", "/me/job-applications/"+appID, token, nil, http.StatusNotFound)
	}
	note.Title = "Changed initial"
	jobCall(t, h, "PATCH", "/me/job-applications/"+appID+"/stages/"+initial.ID, "member", note, http.StatusConflict)
	jobCall(t, h, "DELETE", "/me/job-applications/"+appID+"/stages/"+initial.ID, "member", nil, http.StatusConflict)
	public = jobCall(t, h, "GET", "/jobs/positions/"+p.ID, "", nil, http.StatusOK)
	if strings.Contains(public.Body.String(), "private") || strings.Contains(public.Body.String(), appID) {
		t.Fatalf("private data leaked: %s", public.Body)
	}
	input.ApplicationURL += "?utm_source=test"
	jobCall(t, h, "POST", "/jobs/positions", "member", input, http.StatusConflict)
	jobCall(t, h, "GET", "/jobs/positions/not-an-id", "", nil, http.StatusBadRequest)
	jobCall(t, h, "DELETE", "/me/job-applications/"+appID, "member", nil, http.StatusNoContent)
	if len(readApplications(t, h, "member")) != 0 {
		t.Fatal("application was not removed")
	}
}

func TestJobsMergePreservesBothHistoriesAndAliases(t *testing.T) {
	f, h := jobFixture(t)
	target := newJob(t, h, "target", "Sydney")
	source := newJob(t, h, "source", "Melbourne")
	a := applyJob(t, h, "member", target.ID)
	b := applyJob(t, h, "member", source.ID)
	other := applyJob(t, h, "mentor", source.ID)
	jobCall(t, h, "PATCH", "/me/job-applications/"+a, "member", map[string]string{"status": "inactive", "date": "2026-11-03"}, http.StatusNoContent)
	jobCall(t, h, "PATCH", "/me/job-applications/"+other, "mentor", map[string]string{"status": "inactive", "date": "2026-11-03"}, http.StatusNoContent)
	for _, app := range readApplications(t, h, "member") {
		jobCall(t, h, "PATCH", "/me/job-applications/"+app.ID+"/stages/"+app.Stages[0].ID, "member", jobs.StageInput{Title: "Applied", Date: "2026-11-01", Notes: strings.Repeat(app.ID[:1], 10000)}, http.StatusOK)
	}
	jobCall(t, h, "POST", "/me/job-applications/"+a+"/stages", "member", jobs.StageInput{Title: "Assessment", Date: "2026-11-02", Notes: "target note"}, http.StatusCreated)
	jobCall(t, h, "POST", "/me/job-applications/"+b+"/stages", "member", jobs.StageInput{Title: "Final panel", Date: "2026-11-06", Notes: "source note"}, http.StatusCreated)
	merge := map[string]string{"targetId": target.ID}
	jobCall(t, h, "POST", "/jobs/positions/"+source.ID+"/merge", "mentor", merge, http.StatusForbidden)
	jobCall(t, h, "POST", "/jobs/positions/"+source.ID+"/merge", "coordinator", map[string]string{"targetId": "00000000-0000-7000-8000-000000009999"}, http.StatusNotFound)
	if len(readApplications(t, h, "member")) != 2 {
		t.Fatal("failed merge changed applications")
	}
	jobCall(t, h, "POST", "/jobs/positions/"+source.ID+"/merge", "coordinator", merge, http.StatusOK)
	jobCall(t, h, "POST", "/jobs/positions/"+source.ID+"/merge", "coordinator", merge, http.StatusOK)
	merged := readApplications(t, h, "member")
	if len(merged) != 1 || merged[0].ID != a || merged[0].Status != "inactive" || len(merged[0].Stages) != 5 {
		t.Fatalf("lost history: %#v", merged)
	}
	if len(merged[0].Position.Locations) != 2 || merged[0].Stages[0].Title != "Applied" || merged[0].Stages[3].Title != "Rejected" || merged[0].Stages[4].Title != "Final panel" {
		t.Fatalf("bad merged application: %#v", merged[0])
	}
	if len(merged[0].Stages[0].Notes)+len(merged[0].Stages[1].Notes) != 20000 {
		t.Fatal("merge truncated initial notes")
	}
	moved := readApplications(t, h, "mentor")
	if len(moved) != 1 || moved[0].ID != other || moved[0].Status != "inactive" || moved[0].Position.ID != target.ID || len(moved[0].Stages) != 2 || moved[0].Stages[1].Title != "Rejected" {
		t.Fatal("another owner's application lost")
	}
	public := jobCall(t, h, "GET", "/jobs/positions/"+source.ID, "", nil, http.StatusOK)
	if !strings.Contains(public.Body.String(), target.ID) {
		t.Fatal("old position did not resolve")
	}
	if applyJob(t, h, "member", source.ID) != a {
		t.Fatal("old position created duplicate application")
	}
	duplicate := jobs.PositionInput{CompanyName: "Acme", Category: "other", Title: "Duplicate", JobType: "intern", ApplicationURL: source.ApplicationURL, Locations: []string{"Sydney"}}
	jobCall(t, h, "POST", "/jobs/positions", "member", duplicate, http.StatusConflict)
	var count int
	if err := f.pool.QueryRow(context.Background(), `SELECT count(*) FROM app.audit_events WHERE action='job_position.merged'`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("merge audit count %d", count)
	}
}

func TestJobsApplicationStatusIsPrivateAndPreservesHistory(t *testing.T) {
	f, h := jobFixture(t)
	p := newJob(t, h, "status", "Sydney")
	appID := applyJob(t, h, "member", p.ID)
	path := "/me/job-applications/" + appID
	if readApplications(t, h, "member")[0].Status != "active" {
		t.Fatal("new application must start active")
	}
	jobCall(t, h, "POST", path+"/stages", "member", jobs.StageInput{Title: "Interview", Date: "2026-11-02", Notes: "Keep my private notes"}, http.StatusCreated)
	for i, change := range []struct{ status, title string }{{"inactive", "Rejected"}, {"offer", "Offer"}, {"active", "Active"}} {
		input := map[string]string{"status": change.status, "date": "2026-11-08"}
		jobCall(t, h, "PATCH", path, "", input, http.StatusUnauthorized)
		for _, token := range []string{"unverified", "suspended"} {
			jobCall(t, h, "PATCH", path, token, input, http.StatusForbidden)
		}
		for _, token := range []string{"mentor", "coordinator", "director", "system_admin"} {
			jobCall(t, h, "PATCH", path, token, input, http.StatusNotFound)
		}
		before := readApplications(t, h, "member")[0]
		if len(before.Stages) != 2+i || before.Status == change.status {
			t.Fatalf("unauthorized request changed the application: %#v", before)
		}
		jobCall(t, h, "PATCH", path, "member", input, http.StatusNoContent)
		// Retrying the same status must not add a second stage.
		jobCall(t, h, "PATCH", path, "member", input, http.StatusNoContent)
		if applyJob(t, h, "member", p.ID) != appID {
			t.Fatal("status change created a new application")
		}
		apps := readApplications(t, h, "member")
		if len(apps) != 1 || apps[0].Status != change.status || len(apps[0].Stages) != 3+i || apps[0].Stages[1].Notes != "Keep my private notes" {
			t.Fatalf("status or history not preserved: %#v", apps)
		}
		stage := apps[0].Stages[2+i]
		if stage.Title != change.title || stage.Date != input["date"] || stage.Notes != "" || stage.Initial {
			t.Fatalf("incorrect status stage: %#v", stage)
		}
		if len(readApplications(t, h, "mentor")) != 0 {
			t.Fatal("another account can see the application")
		}
	}
	for _, status := range []string{"", "closed", "Rejected"} {
		jobCall(t, h, "PATCH", path, "member", map[string]string{"status": status, "date": "2026-11-08"}, http.StatusBadRequest)
	}
	for _, date := range []string{"", "2026-02-30", "not-a-date"} {
		jobCall(t, h, "PATCH", path, "member", map[string]string{"status": "inactive", "date": date}, http.StatusBadRequest)
	}
	jobCall(t, h, "PATCH", "/me/job-applications/not-an-id", "member", map[string]string{"status": "inactive", "date": "2026-11-08"}, http.StatusBadRequest)
	// If the stage insert fails, the status update must roll back with it.
	if err := f.db.UpdateJobApplicationStatus(context.Background(), studentID, appID, "inactive", "not-a-date"); err == nil {
		t.Fatal("invalid stage date was accepted by storage")
	}
	app := readApplications(t, h, "member")[0]
	if app.Status != "active" || len(app.Stages) != 5 {
		t.Fatal("failed status update changed the application or its history")
	}
}

func TestJobsConcurrentApplyAndDeletionLifecycle(t *testing.T) {
	f, h := jobFixture(t)
	p := newJob(t, h, "concurrent", "Sydney")
	jobCall(t, h, "POST", "/jobs/referrals", "member", jobs.ReferralInput{CompanyName: p.Company.Name, Notes: "Referral requirements"}, http.StatusCreated)
	ctx := context.Background()
	var wg sync.WaitGroup
	results := make(chan string, 8)
	failures := make(chan error, 8)
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			id, err := f.db.ApplyToJob(ctx, studentID, p.ID, "2026-11-01")
			if err != nil {
				failures <- err
			}
			results <- id
		}()
	}
	wg.Wait()
	close(results)
	close(failures)
	for err := range failures {
		t.Fatal(err)
	}
	var first string
	for id := range results {
		if first == "" {
			first = id
		} else if id != first {
			t.Fatal("concurrent duplicate applications")
		}
	}
	if _, err := f.pool.Exec(ctx, `INSERT INTO app.user_auth_links(auth_subject,user_id,provider,provider_account_id) VALUES('job-user',$1,'credential','job-user')`, studentID); err != nil {
		t.Fatal(err)
	}
	if err := f.db.ApplyIdentityEvent(ctx, accounts.IdentityEvent{EventID: "job-deletion", AuthUserID: "job-user", Type: "auth_pseudonymized", SecurityVersion: 2, OccurredAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	var count int
	for _, table := range []string{"job_applications", "job_application_stages", "job_referrals"} {
		if err := f.pool.QueryRow(ctx, `SELECT count(*) FROM app.`+table).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != 0 {
			t.Fatalf("private rows remained in %s", table)
		}
	}
	jobCall(t, h, "GET", "/jobs/positions/"+p.ID, "", nil, http.StatusOK)
}
