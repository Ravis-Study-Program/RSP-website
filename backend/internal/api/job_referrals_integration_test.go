//go:build integration

package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/magedmg/RSP-website/backend/internal/jobs"
)

func readReferrals(t *testing.T, h http.Handler, path, token string) Page[jobs.Referral] {
	t.Helper()
	res := jobCall(t, h, "GET", path, token, nil, http.StatusOK)
	if res.Header().Get("Cache-Control") != "private, no-store" {
		t.Fatal("referrals may be cached publicly")
	}
	var page Page[jobs.Referral]
	if err := json.Unmarshal(res.Body.Bytes(), &page); err != nil {
		t.Fatal(err)
	}
	return page
}

func TestJobsReferralsRequireSignInAndOwnerWrites(t *testing.T) {
	f, h := jobFixture(t)
	position := newJob(t, h, "referrals", "Sydney")
	input := jobs.ReferralInput{CompanyName: " Acme ", Notes: "Interns only. GPA 6+. Message me your resume."}
	for _, method := range []string{"GET", "POST"} {
		jobCall(t, h, method, "/jobs/referrals", "", input, http.StatusUnauthorized)
		for _, token := range []string{"unverified", "suspended"} {
			jobCall(t, h, method, "/jobs/referrals", token, input, http.StatusForbidden)
		}
	}
	created := jobCall(t, h, "POST", "/jobs/referrals", "member", input, http.StatusCreated)
	var referral jobs.Referral
	if err := json.Unmarshal(created.Body.Bytes(), &referral); err != nil {
		t.Fatal(err)
	}
	if referral.Referrer.ID != studentID || referral.Referrer.Name == "" || referral.Referrer.Slug == "" || referral.Company.ID != position.Company.ID || referral.Company.Category != "big_tech" || referral.Notes != input.Notes {
		t.Fatalf("wrong referral identity or company: %#v", referral)
	}
	path := "/jobs/referrals/" + referral.ID
	input.CompanyName = "acme"
	jobCall(t, h, "POST", "/jobs/referrals", "member", input, http.StatusConflict)
	// Another member can offer referrals at the same company.
	jobCall(t, h, "POST", "/jobs/referrals", "mentor", input, http.StatusCreated)
	for _, token := range []string{"member", "mentor", "coordinator", "director", "system_admin"} {
		page := readReferrals(t, h, "/jobs/referrals", token)
		if len(page.Items) != 2 || page.TotalCount != 2 || page.Items[0].Notes != input.Notes {
			t.Fatalf("members cannot read offers: %#v", page)
		}
	}
	input.Notes = "Graduate roles only."
	for _, token := range []string{"mentor", "coordinator", "director", "system_admin"} {
		jobCall(t, h, "PATCH", path, token, input, http.StatusNotFound)
		jobCall(t, h, "DELETE", path, token, nil, http.StatusNotFound)
	}
	jobCall(t, h, "PATCH", path, "member", input, http.StatusOK)
	if readReferrals(t, h, "/jobs/referrals", "mentor").Items[0].Notes != input.Notes {
		t.Fatal("edited notes were not saved")
	}
	for _, invalid := range []jobs.ReferralInput{{CompanyName: " "}, {CompanyName: strings.Repeat("a", 121)}, {CompanyName: "Acme", Notes: strings.Repeat("n", 10001)}} {
		jobCall(t, h, "PATCH", path, "member", invalid, http.StatusBadRequest)
	}
	jobCall(t, h, "PATCH", "/jobs/referrals/not-an-id", "member", input, http.StatusBadRequest)
	jobCall(t, h, "POST", "/jobs/referrals", "member", jobs.ReferralInput{CompanyName: "New Company", Notes: ""}, http.StatusCreated)
	first := readReferrals(t, h, "/jobs/referrals?limit=1", "member")
	if len(first.Items) != 1 || first.TotalCount != 3 || !first.PageInfo.HasMore {
		t.Fatalf("bad referral page: %#v", first)
	}
	second := readReferrals(t, h, "/jobs/referrals?limit=1&cursor="+*first.PageInfo.NextCursor, "member")
	if len(second.Items) != 1 || second.Items[0].ID == first.Items[0].ID {
		t.Fatal("referral pagination repeated a record")
	}
	jobCall(t, h, "GET", "/jobs/referrals?cursor="+*first.PageInfo.NextCursor, "mentor", nil, http.StatusBadRequest)
	// Unavailable accounts' offers disappear immediately.
	if _, err := f.pool.Exec(context.Background(), `UPDATE app.users SET account_state='suspended' WHERE id=$1`, studentID); err != nil {
		t.Fatal(err)
	}
	if page := readReferrals(t, h, "/jobs/referrals", "mentor"); len(page.Items) != 1 || page.TotalCount != 1 || page.Items[0].Referrer.ID != otherID {
		t.Fatal("suspended referrer remained visible")
	}
	if _, err := f.pool.Exec(context.Background(), `UPDATE app.users SET account_state='active' WHERE id=$1`, studentID); err != nil {
		t.Fatal(err)
	}
	jobCall(t, h, "DELETE", path, "member", nil, http.StatusNoContent)
	if page := readReferrals(t, h, "/jobs/referrals", "member"); len(page.Items) != 2 {
		t.Fatal("stop referring did not remove the listing")
	}
}
