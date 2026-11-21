//go:build integration

package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/Ravis-Study-Program/RSP-website/backend/internal/jobs"
)

const testReferralResumeURL = "https://drive.google.com/file/d/sample-resume/view?resourcekey=sample-key"

func saveTestReferralProfile(t *testing.T, h http.Handler, token, reason, resumeURL string, want int) {
	t.Helper()
	jobCall(t, h, "PUT", "/jobs/referrals/profile", token, jobs.ReferralProfile{Reason: reason, ResumeURL: resumeURL}, want)
}

func TestJobsReferralMatchingScopesProfilesAndResumeLinks(t *testing.T) {
	f, h := jobFixture(t)
	p := newJob(t, h, "referral", "Sydney")
	other := jobs.PositionInput{CompanyName: "Other Company", Category: "other", Title: "Other company's role", JobType: "intern", ApplicationURL: "https://jobs.example.com/other", Locations: []string{"Sydney"}}
	var otherPosition jobs.Position
	otherResponse := jobCall(t, h, "POST", "/jobs/positions", "member", other, http.StatusCreated)
	if err := json.Unmarshal(otherResponse.Body.Bytes(), &otherPosition); err != nil {
		t.Fatal(err)
	}

	const base = "/jobs/referrals"
	queue := base + "/companies/" + p.Company.ID + "/candidates"
	for _, path := range []string{base, base + "/counts", queue} {
		jobCall(t, h, "GET", path, "", nil, http.StatusUnauthorized)
		jobCall(t, h, "GET", path, "member", nil, http.StatusForbidden)
	}
	jobCall(t, h, "GET", base, "alumnus", nil, http.StatusOK)
	jobCall(t, h, "PUT", base+"/requests/"+p.ID, "mentor", nil, http.StatusForbidden)
	jobCall(t, h, "PUT", base+"/requests/"+p.ID, "student", nil, http.StatusConflict)
	offer := base + "/offers/" + p.Company.ID
	for _, token := range []string{"student", "mentor", "member"} {
		jobCall(t, h, "PUT", offer, token, nil, http.StatusForbidden)
		jobCall(t, h, "DELETE", offer, token, nil, http.StatusForbidden)
		jobCall(t, h, "GET", queue, token, nil, http.StatusForbidden)
	}
	for _, badLink := range []string{"", "http://drive.google.com/file/d/sample/view", "https://drive.google.com.example.org/file/d/sample/view", "https://drive.google.com/drive/folders/sample"} {
		saveTestReferralProfile(t, h, "student", "A reusable reason", badLink, http.StatusBadRequest)
	}
	saveTestReferralProfile(t, h, "student", "A reusable reason", " "+testReferralResumeURL+" ", http.StatusNoContent)
	assertReferralProfile := func(token, resumeURL string) {
		t.Helper()
		res := jobCall(t, h, "GET", base, token, nil, http.StatusOK)
		var board jobs.ReferralBoard
		if err := json.Unmarshal(res.Body.Bytes(), &board); err != nil || board.Profile.ResumeURL != resumeURL {
			t.Fatalf("unexpected own referral profile: %s, %v", res.Body, err)
		}
	}
	assertReferralProfile("student", testReferralResumeURL)
	assertReferralProfile("alumnus", "")
	jobCall(t, h, "PUT", base+"/requests/"+p.ID, "student", nil, http.StatusNoContent)
	jobCall(t, h, "PUT", base+"/requests/"+otherPosition.ID, "student", nil, http.StatusNoContent)

	if _, err := f.pool.Exec(context.Background(), `UPDATE app.enrollments SET role='student', state='completed', student_level='beginner',
 assignment_state='revoked', activated_at=NULL WHERE user_id=$1`, otherID); err != nil {
		t.Fatal(err)
	}
	// A student may have opted in before referring became graduate-only.
	if _, err := f.pool.Exec(context.Background(), `INSERT INTO app.job_referrals(user_id,company_id,matching_enabled) VALUES($1,$2,true)`, studentID, p.Company.ID); err != nil {
		t.Fatal(err)
	}
	jobCall(t, h, "GET", queue, "student", nil, http.StatusForbidden)
	assertEmptyQueue := func(path string) {
		t.Helper()
		res := jobCall(t, h, "GET", path, "alumnus", nil, http.StatusOK)
		var page Page[jobs.ReferralCandidate]
		if err := json.Unmarshal(res.Body.Bytes(), &page); err != nil || page.TotalCount != 0 || len(page.Items) != 0 {
			t.Fatalf("unexpected candidate link access: %s, %v", res.Body, err)
		}
	}
	assertEmptyQueue(queue)
	jobCall(t, h, "PUT", offer, "alumnus", nil, http.StatusNoContent)
	counts := jobCall(t, h, "GET", base+"/counts", "student", nil, http.StatusOK)
	var countResult struct{ Items []jobs.ReferralCount }
	if err := json.Unmarshal(counts.Body.Bytes(), &countResult); err != nil || len(countResult.Items) != 1 || countResult.Items[0] != (jobs.ReferralCount{CompanyID: p.Company.ID, Count: 1}) {
		t.Fatalf("unexpected referrer counts: %s, %v", counts.Body, err)
	}
	candidates := jobCall(t, h, "GET", queue, "alumnus", nil, http.StatusOK)
	var candidateResult Page[jobs.ReferralCandidate]
	if err := json.Unmarshal(candidates.Body.Bytes(), &candidateResult); err != nil || candidateResult.TotalCount != 1 || len(candidateResult.Items) != 1 {
		t.Fatalf("unexpected candidate list: %s, %v", candidates.Body, err)
	}
	candidate := candidateResult.Items[0]
	if candidate.ID != studentID || candidate.Name != "Student" || candidate.Reason != "A reusable reason" || candidate.ResumeURL != testReferralResumeURL || len(candidate.Positions) != 1 || candidate.Positions[0].ID != p.ID {
		t.Fatalf("candidate profile leaked another company or lost data: %+v", candidate)
	}
	if candidates.Header().Get("Cache-Control") != "private, no-store" {
		t.Fatal("candidate profiles must not be cached")
	}
	assertEmptyQueue(base + "/companies/" + otherPosition.Company.ID + "/candidates")
	jobCall(t, h, "DELETE", offer, "alumnus", nil, http.StatusNoContent)
	assertEmptyQueue(queue)
	jobCall(t, h, "PUT", offer, "alumnus", nil, http.StatusNoContent)
	jobCall(t, h, "DELETE", base+"/requests/"+p.ID, "student", nil, http.StatusNoContent)
	assertEmptyQueue(queue)
	jobCall(t, h, "DELETE", base+"/profile", "student", nil, http.StatusNoContent)
	assertReferralProfile("student", "")
}

func TestJobsReferralRequestLimitAndPositionLifecycle(t *testing.T) {
	_, h := jobFixture(t)
	saveTestReferralProfile(t, h, "student", "A reusable reason", testReferralResumeURL, http.StatusNoContent)
	var positions []jobs.Position
	for i := range jobs.MaxReferralRequests + 1 {
		positions = append(positions, newJob(t, h, fmt.Sprintf("referral-%d", i), "Sydney"))
	}
	const base = "/jobs/referrals"
	responses := make(chan int, len(positions))
	for _, p := range positions {
		go func() { responses <- testRequest(t, h, "PUT", "/api/v2"+base+"/requests/"+p.ID, "student", "").Code }()
	}
	var accepted, rejected int
	for range positions {
		switch code := <-responses; code {
		case http.StatusNoContent:
			accepted++
		case http.StatusConflict:
			rejected++
		default:
			t.Fatalf("unexpected referral status %d", code)
		}
	}
	if accepted != jobs.MaxReferralRequests || rejected != 1 {
		t.Fatalf("request limit was not enforced: %d accepted, %d rejected", accepted, rejected)
	}
	board := jobCall(t, h, "GET", base, "student", nil, http.StatusOK)
	var v jobs.ReferralBoard
	if err := json.Unmarshal(board.Body.Bytes(), &v); err != nil || len(v.Requests) != jobs.MaxReferralRequests {
		t.Fatalf("unexpected referral board: %s, %v", board.Body, err)
	}
	selected := map[string]bool{}
	for _, id := range v.Requests {
		selected[id] = true
	}
	var toClose, next jobs.Position
	for _, p := range positions {
		if selected[p.ID] {
			toClose = p
		} else {
			next = p
		}
	}
	input := jobs.PositionInput{CompanyName: toClose.Company.Name, Category: toClose.Company.Category, Title: toClose.Title, JobType: toClose.JobType, ApplicationURL: toClose.ApplicationURL, Locations: toClose.Locations, Status: "closed"}
	jobCall(t, h, "PATCH", "/jobs/positions/"+toClose.ID, "coordinator", input, http.StatusOK)
	jobCall(t, h, "PUT", base+"/requests/"+toClose.ID, "student", nil, http.StatusConflict)
	jobCall(t, h, "PUT", base+"/requests/"+next.ID, "student", nil, http.StatusNoContent)
	input.Status = "open"
	jobCall(t, h, "PATCH", "/jobs/positions/"+toClose.ID, "coordinator", input, http.StatusOK)
	jobCall(t, h, "PUT", base+"/requests/"+toClose.ID, "student", nil, http.StatusConflict)
}
