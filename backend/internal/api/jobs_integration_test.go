//go:build integration

package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Ravis-Study-Program/RSP-website/backend/internal/accounts"
	"github.com/Ravis-Study-Program/RSP-website/backend/internal/authz"
	"github.com/Ravis-Study-Program/RSP-website/backend/internal/jobs"
	"github.com/Ravis-Study-Program/RSP-website/backend/internal/platform/ratelimit"
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
		case "student", "alumnus":
			actor.Enrollments = []authz.Enrollment{{SeasonID: seasonID, Role: authz.Student, State: authz.Active}}
			if token == "alumnus" {
				actor.UserID = otherID
				actor.Enrollments[0].State = authz.Completed
			}
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

func TestJobsSharedProgressUpdatesWithoutExposingPrivateNotes(t *testing.T) {
	_, h := jobFixture(t)
	position := newJob(t, h, "shared-progress", "Sydney")
	positionPath := "/jobs/positions/" + position.ID + "/progress"
	application := applyJob(t, h, "member", position.ID)
	privateApp := applyJob(t, h, "mentor", position.ID)
	appPath := "/me/job-applications/" + application
	date := "2026-11-02"
	privateInitial := readApplications(t, h, "mentor")[0].Stages[0]
	private := false
	jobCall(t, h, "PUT", "/me/job-applications/"+privateApp, "mentor", jobs.ApplicationEdit{
		Status: "active", Date: date, ShareProgress: &private,
		Stages: []jobs.StageEdit{
			{ID: privateInitial.ID, StageInput: jobs.StageInput{Title: privateInitial.Title, Date: privateInitial.Date}},
		},
	}, http.StatusNoContent)
	jobCall(t, h, "POST", "/me/job-applications/"+privateApp+"/stages", "mentor", jobs.StageInput{
		Title: "Unshared round", Date: date, Notes: "Unshared notes",
	}, http.StatusCreated)
	readProgress := func() jobs.PositionProgress {
		t.Helper()
		response := jobCall(t, h, "GET", positionPath, "", nil, http.StatusOK)
		for _, secret := range []string{application, privateApp, studentID, otherID, date, "Private prep", "Unshared round"} {
			if strings.Contains(response.Body.String(), secret) {
				t.Fatalf("unexpected private value in shared progress: %s", response.Body)
			}
		}
		var progress jobs.PositionProgress
		if err := json.Unmarshal(response.Body.Bytes(), &progress); err != nil {
			t.Fatal(err)
		}
		return progress
	}
	if got := readProgress(); got.Total != 1 || len(got.Stages) != 1 || got.Stages[0].Current != 1 {
		t.Fatalf("a new application should share its Applied count by default: %+v", got)
	}
	initial := readApplications(t, h, "member")[0].Stages[0]
	input := jobs.ApplicationEdit{
		Status: "active", Date: date,
		Stages: []jobs.StageEdit{
			{ID: initial.ID, StageInput: jobs.StageInput{Title: initial.Title, Date: initial.Date}},
			{StageInput: jobs.StageInput{Title: "Recruiter screen", Date: date, Notes: "Private prep"}},
		},
	}
	jobCall(t, h, "PUT", appPath, "member", input, http.StatusNoContent)
	if got := readProgress(); got.Total != 1 || len(got.Stages) != 2 || got.Stages[1].Title != "Recruiter screen" ||
		got.Stages[1].Current != 1 || !slices.Contains(got.Suggestions, "Recruiter screen") {
		t.Fatalf("unexpected progress after adding the recruiter screen: %+v", got)
	}
	technical := jobs.StageInput{Title: "Technical (LeetCode style)", Date: date}
	jobCall(t, h, "POST", appPath+"/stages", "member", technical, http.StatusCreated)
	if got := readProgress(); len(got.Stages) != 3 || len(got.Transitions) != 2 ||
		got.Stages[1].Current != 0 || got.Stages[2].Current != 1 {
		t.Fatalf("reached round did not update progress: %+v", got)
	}
	stages := readApplications(t, h, "member")[0].Stages
	input.Stages[1].ID = stages[1].ID
	input.Stages = append(input.Stages, jobs.StageEdit{ID: stages[2].ID, StageInput: technical})
	input.ShareProgress = &private
	jobCall(t, h, "PUT", appPath, "member", input, http.StatusNoContent)
	if got := readProgress(); got.Total != 0 || len(got.Suggestions) != 0 {
		t.Fatalf("unsharing left progress visible: %+v", got)
	}
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
	f, h := jobFixture(t)
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
	const legacyNote = "private application note"
	if _, err := f.pool.Exec(context.Background(), `UPDATE app.job_application_stages SET notes=$1 WHERE id=$2`, legacyNote, initial.ID); err != nil {
		t.Fatal(err)
	}
	note := jobs.StageInput{Title: "Applied", Date: "2026-11-03", Notes: "private application preferences"}
	updated := jobCall(t, h, "PATCH", "/me/job-applications/"+appID+"/stages/"+initial.ID, "member", note, http.StatusOK)
	var updatedStage jobs.Stage
	if err := json.Unmarshal(updated.Body.Bytes(), &updatedStage); err != nil {
		t.Fatal(err)
	}
	if updatedStage.Title != "Applied" || updatedStage.Date != note.Date || updatedStage.Notes != note.Notes || !updatedStage.Initial {
		t.Fatalf("Applied date and notes were not updated without changing the stage: %+v", updatedStage)
	}
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
	keptNotes, mergedNotes := strings.Repeat("A", 10000), strings.Repeat("B", 10000)
	for _, app := range readApplications(t, h, "member") {
		date, notes := "2026-11-01", mergedNotes
		if app.ID == a {
			date, notes = "2026-11-02", keptNotes
		}
		jobCall(t, h, "PATCH", "/me/job-applications/"+app.ID+"/stages/"+app.Stages[0].ID, "member", jobs.StageInput{Title: "Applied", Date: date, Notes: notes}, http.StatusOK)
	}
	jobCall(t, h, "POST", "/me/job-applications/"+a+"/stages", "member", jobs.StageInput{Title: "Assessment", Date: "2026-11-06", Notes: "target note"}, http.StatusCreated)
	jobCall(t, h, "POST", "/me/job-applications/"+b+"/stages", "member", jobs.StageInput{Title: "Final panel", Date: "2026-11-06", Notes: "source note"}, http.StatusCreated)
	if _, err := f.pool.Exec(context.Background(), `UPDATE app.job_application_stages SET sort_order=1 WHERE application_id=$1 AND title='Assessment'`, a); err != nil {
		t.Fatal(err)
	}
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
	stages := merged[0].Stages
	if len(merged[0].Position.Locations) != 2 || stages[0].Title != "Applied" || stages[0].Date != "2026-11-02" || stages[2].Title != "Rejected" ||
		stages[3].Date != "2026-11-06" || stages[4].Date != "2026-11-06" || stages[3].ID >= stages[4].ID {
		t.Fatalf("bad merged application: %#v", merged[0])
	}
	if stages[0].Notes != keptNotes || stages[1].Title != "Applied (merged listing)" || stages[1].Notes != mergedNotes {
		t.Fatal("merge lost Applied notes")
	}
	progress := jobCall(t, h, "GET", "/jobs/positions/"+target.ID+"/progress", "", nil, http.StatusOK)
	for _, text := range []string{keptNotes, mergedNotes, "target note", "source note", "Applied (merged listing)"} {
		if strings.Contains(progress.Body.String(), text) {
			t.Fatal("shared progress exposed private history")
		}
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
	ctx := context.Background()
	if _, err := f.pool.Exec(ctx, `INSERT INTO app.job_referrals(user_id,company_id,matching_enabled) VALUES($1,$2,true)`, studentID, p.Company.ID); err != nil {
		t.Fatal(err)
	}
	saveTestReferralProfile(t, h, "student", "Test candidate profile", testReferralResumeURL, http.StatusNoContent)
	jobCall(t, h, "PUT", "/jobs/referrals/requests/"+p.ID, "student", nil, http.StatusNoContent)
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
	for _, table := range []string{"job_applications", "job_application_stages", "job_referrals", "job_referral_profiles", "job_referral_requests"} {
		if err := f.pool.QueryRow(ctx, `SELECT count(*) FROM app.`+table).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != 0 {
			t.Fatalf("private rows remained in %s", table)
		}
	}
	jobCall(t, h, "GET", "/jobs/positions/"+p.ID, "", nil, http.StatusOK)
}

func TestJobsEditApplicationIsAtomicAndPrivate(t *testing.T) {
	_, h := jobFixture(t)
	position := newJob(t, h, "editor", "Sydney")
	id := applyJob(t, h, "member", position.ID)
	path := "/me/job-applications/" + id
	for _, title := range []string{"Phone Screen", "Technical Interview"} {
		jobCall(t, h, "POST", path+"/stages", "member", jobs.StageInput{Title: title, Date: "2026-11-02", Notes: "Private"}, http.StatusCreated)
	}
	before := readApplications(t, h, "member")[0]
	input := jobs.ApplicationEdit{Status: "offer", Date: "2026-11-03"}
	for _, i := range []int{0, 2, 1} {
		stage := before.Stages[i]
		input.Stages = append(input.Stages, jobs.StageEdit{ID: stage.ID, StageInput: jobs.StageInput{Title: stage.Title, Date: stage.Date, Notes: stage.Notes}})
	}
	input.Stages[0].Date = "2026-11-02"
	input.Stages[0].Notes = "Preferred the Sydney office"
	jobCall(t, h, "PUT", path, "", input, http.StatusUnauthorized)
	jobCall(t, h, "PUT", path, "mentor", input, http.StatusNotFound)
	invalid := input
	invalid.Stages = append([]jobs.StageEdit(nil), input.Stages...)
	invalid.Stages[1].ID = "11111111-1111-4111-8111-111111111111"
	jobCall(t, h, "PUT", path, "member", invalid, http.StatusConflict)
	unchanged := readApplications(t, h, "member")[0]
	if unchanged.Status != "active" || unchanged.Stages[0].Notes != before.Stages[0].Notes || unchanged.Stages[1].Title != "Phone Screen" {
		t.Fatal("invalid edit partially saved")
	}
	jobCall(t, h, "PUT", path, "member", input, http.StatusNoContent)
	saved := readApplications(t, h, "member")[0]
	if saved.Stages[0].Title != "Applied" || saved.Stages[0].Date != input.Stages[0].Date || saved.Stages[0].Notes != input.Stages[0].Notes {
		t.Fatalf("bulk edit lost the Applied date or notes: %+v", saved.Stages[0])
	}
	if saved.Status != "offer" || len(saved.Stages) != 4 || saved.Stages[1].Title != "Technical Interview" || saved.Stages[2].Title != "Phone Screen" || saved.Stages[3].Title != "Offer" {
		t.Fatalf("order/status not saved: %#v", saved)
	}
	// Removing the initial stage is never permitted.
	invalid = input
	invalid.Stages = input.Stages[1:]
	jobCall(t, h, "PUT", path, "member", invalid, http.StatusBadRequest)
	// Delete one ordinary stage and append a new one in a single save.
	input.Stages = append(input.Stages[:2], jobs.StageEdit{StageInput: jobs.StageInput{Title: "Team Match", Date: "2026-11-04", Notes: "Still private"}})
	input.Stages[0].Notes = ""
	jobCall(t, h, "PUT", path, "member", input, http.StatusNoContent)
	saved = readApplications(t, h, "member")[0]
	if len(saved.Stages) != 3 || saved.Stages[0].Notes != "" || saved.Stages[2].Title != "Team Match" || saved.Stages[2].Notes != "Still private" {
		t.Fatalf("stage replacement failed: %#v", saved)
	}
}
