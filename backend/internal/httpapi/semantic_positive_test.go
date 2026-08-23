package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/magedmg/RSP-website/backend/internal/accounts"
	"github.com/magedmg/RSP-website/backend/internal/authz"
	"github.com/magedmg/RSP-website/backend/internal/mockinterviews"
	"github.com/magedmg/RSP-website/backend/internal/model"
)

func decodePage[T any](t *testing.T, body []byte) model.Page[T] {
	t.Helper()
	var page model.Page[T]
	if err := json.Unmarshal(body, &page); err != nil {
		t.Fatal(err)
	}
	return page
}

func TestSeasonWeeksOrderedRoundTripAndUnknownSeasonNotFound(t *testing.T) {
	f := newFixture()
	season := f.repository.Seasons["season"]
	f.repository.Weeks["week-two"] = model.Week{ID: "week-two", SeasonID: season.ID, Number: 2, StartAt: season.StartAt.Add(7 * 24 * time.Hour), EndAt: season.StartAt.Add(14 * 24 * time.Hour), ResourceURL: "https://rsp.test/week-two", Revision: 1}
	f.repository.Weeks["week-one"] = model.Week{ID: "week-one", SeasonID: season.ID, Number: 1, StartAt: season.StartAt, EndAt: season.StartAt.Add(7 * 24 * time.Hour), ResourceURL: "https://rsp.test/week-one", Revision: 1}

	w := request(t, f, http.MethodGet, "/api/v2/seasons/season/weeks?sort=number:asc", "student", "")
	if w.Code != http.StatusOK {
		t.Fatalf("week list: %d %s", w.Code, w.Body.String())
	}
	page := decodePage[model.Week](t, w.Body.Bytes())
	if len(page.Items) != 2 || page.Items[0].Number != 1 || page.Items[1].Number != 2 || !page.Items[0].StartAt.Equal(season.StartAt) || !page.Items[1].EndAt.Equal(season.StartAt.Add(14*24*time.Hour)) {
		t.Fatalf("ordered week round trip: %#v", page.Items)
	}
	w = request(t, f, http.MethodGet, "/api/v2/seasons/missing/weeks", "director", "")
	if w.Code != http.StatusNotFound || problemCode(t, w) != "not_found" {
		t.Fatalf("unknown season: %d %s", w.Code, w.Body.String())
	}
}

func TestMeEnrollmentRolesAndEligibleMemberDirectory(t *testing.T) {
	f := newFixture()
	f.repository.Enrollments["student-enrollment"] = model.Enrollment{ID: "student-enrollment", SeasonID: "season", UserID: "student", Role: "student", StudentLevel: "beginner", State: "active", AssignmentState: "active", Revision: 1}
	w := request(t, f, http.MethodGet, "/api/v2/me", "student", "")
	if w.Code != http.StatusOK || !bytes.Contains(w.Body.Bytes(), []byte(`"seasonId":"season"`)) || !bytes.Contains(w.Body.Bytes(), []byte(`"role":"student"`)) || !bytes.Contains(w.Body.Bytes(), []byte(`"state":"active"`)) {
		t.Fatalf("current enrollment roles: %d %s", w.Code, w.Body.String())
	}
	w = request(t, f, http.MethodGet, "/api/v2/seasons/season/members?sort=role:asc", "student", "")
	if w.Code != http.StatusOK {
		t.Fatalf("member directory: %d %s", w.Code, w.Body.String())
	}
	page := decodePage[model.Enrollment](t, w.Body.Bytes())
	if len(page.Items) != 2 || page.TotalCount != 2 {
		t.Fatalf("eligible season members: %#v", page)
	}
}

func TestMentorMenteeFilteringAndUnassignedStudents(t *testing.T) {
	f := newFixture()
	f.repository.Users["mentor"] = model.User{ID: "mentor", Slug: "mentor", Name: "Mentor", AccountState: "active", Timezone: "Australia/Adelaide", Revision: 1}
	f.repository.Users["unassigned"] = model.User{ID: "unassigned", Slug: "unassigned", Name: "Unassigned", AccountState: "active", Timezone: "Australia/Adelaide", Revision: 1}
	f.repository.Enrollments["mentor-enrollment"] = model.Enrollment{ID: "mentor-enrollment", SeasonID: "season", UserID: "mentor", Role: "mentor", State: "active", AssignmentState: "active", Revision: 1}
	f.repository.Enrollments["unassigned-enrollment"] = model.Enrollment{ID: "unassigned-enrollment", SeasonID: "season", UserID: "unassigned", Role: "student", StudentLevel: "beginner", State: "active", AssignmentState: "active", Revision: 1}
	f.repository.Mentorships["assigned"] = model.Mentorship{ID: "assigned", SeasonID: "season", MentorUserID: "mentor", StudentUserID: "other", Revision: 1}
	f.actors["mentor"] = authz.Actor{UserID: "mentor", EmailVerified: true, AccountState: authz.AccountActive, GlobalRoles: map[authz.GlobalRole]bool{}, Enrollments: []authz.Enrollment{{SeasonID: "season", Role: authz.Mentor, State: authz.Active}}}

	w := request(t, f, http.MethodGet, "/api/v2/seasons/season/mentorships?mentorUserId=mentor", "mentor", "")
	page := decodePage[model.Mentorship](t, w.Body.Bytes())
	if w.Code != http.StatusOK || len(page.Items) != 1 || page.Items[0].StudentUserID != "other" {
		t.Fatalf("mentor assignments: %d %#v", w.Code, page)
	}
	w = request(t, f, http.MethodGet, "/api/v2/seasons/season/mentorships?studentUserId=unassigned", "mentor", "")
	page = decodePage[model.Mentorship](t, w.Body.Bytes())
	if w.Code != http.StatusOK || len(page.Items) != 0 || page.TotalCount != 0 {
		t.Fatalf("unassigned relation: %d %#v", w.Code, page)
	}
	w = request(t, f, http.MethodGet, "/api/v2/seasons/season/members?role=student", "mentor", "")
	members := decodePage[model.Enrollment](t, w.Body.Bytes())
	if w.Code != http.StatusOK || len(members.Items) != 2 {
		t.Fatalf("mentor team candidates: %d %#v", w.Code, members)
	}
}

func TestStudentPromotionAndRemovalPersistReasonAndActorAudit(t *testing.T) {
	f := newFixture()
	f.repository.Users["removed"] = model.User{ID: "removed", Slug: "removed", Name: "Removed", AccountState: "active", Revision: 1}
	f.repository.Enrollments["removed-enrollment"] = model.Enrollment{ID: "removed-enrollment", SeasonID: "season", UserID: "removed", Role: "student", StudentLevel: "beginner", State: "active", AssignmentState: "active", Revision: 1}

	w := request(t, f, http.MethodPost, "/api/v2/seasons/season/members/other-enrollment/promote", "coordinator", `{"role":"mentor","reason":"graduated","revision":1}`)
	if w.Code != http.StatusOK || f.repository.Enrollments["other-enrollment"].Role != "mentor" {
		t.Fatalf("student promotion: %d %s", w.Code, w.Body.String())
	}
	w = request(t, f, http.MethodPost, "/api/v2/seasons/season/members/removed-enrollment/remove", "coordinator", `{"reason":"attendance policy","revision":1}`)
	removed := f.repository.Enrollments["removed-enrollment"]
	if w.Code != http.StatusOK || removed.State != "kicked" || removed.RemovalReason == nil || *removed.RemovalReason != "attendance policy" {
		t.Fatalf("reasoned removal: %d %#v %s", w.Code, removed, w.Body.String())
	}
	found := false
	for _, audit := range f.repository.Audits {
		if audit.Action == "enrollment.removed" && audit.SubjectID == "removed-enrollment" && audit.ActorID != nil && *audit.ActorID == "student" && audit.Data["reason"] == "attendance policy" {
			found = true
		}
	}
	if !found {
		t.Fatalf("removal reason/actor audit missing: %#v", f.repository.Audits)
	}
}

func TestPracticeAttemptDurationAndSanitizedRichNotesRoundTrip(t *testing.T) {
	f := newFixture()
	create := `{"problemId":"problem","outcome":"independently_solved","confidence":5,"minutes":31,"notes":"<p><strong>kept</strong><script>alert(1)</script></p>","attemptedAt":"2026-08-13T00:00:00Z"}`
	w := request(t, f, http.MethodPost, "/api/v2/problem-attempts", "student", create)
	if w.Code != http.StatusCreated {
		t.Fatalf("create attempt: %d %s", w.Code, w.Body.String())
	}
	var created model.Attempt
	if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.Minutes != 31 || !bytes.Contains([]byte(created.Notes), []byte("<strong>kept</strong>")) || bytes.Contains([]byte(created.Notes), []byte("script")) {
		t.Fatalf("created attempt round trip: %#v", created)
	}
	update := `{"problemId":"problem","outcome":"solved_with_hints","confidence":3,"minutes":42,"notes":"<p><u>safe</u><img src=x onerror=alert(1)></p>","attemptedAt":"2026-08-13T01:00:00Z","revision":1}`
	w = request(t, f, http.MethodPatch, "/api/v2/problem-attempts/"+created.ID, "student", update)
	var updated model.Attempt
	if err := json.Unmarshal(w.Body.Bytes(), &updated); err != nil {
		t.Fatal(err)
	}
	if w.Code != http.StatusOK || updated.Minutes != 42 || !bytes.Contains([]byte(updated.Notes), []byte("<u>safe</u>")) || bytes.Contains([]byte(updated.Notes), []byte("img")) {
		t.Fatalf("updated attempt round trip: %d %#v", w.Code, updated)
	}
	w = request(t, f, http.MethodGet, "/api/v2/problem-attempts", "student", "")
	page := decodePage[model.Attempt](t, w.Body.Bytes())
	if w.Code != http.StatusOK || len(page.Items) < 1 || page.Items[0].ID != created.ID {
		t.Fatalf("listed attempt: %d %#v", w.Code, page.Items)
	}
	w = request(t, f, http.MethodDelete, "/api/v2/problem-attempts/"+created.ID+"?revision=2", "student", "")
	if w.Code != http.StatusNoContent {
		t.Fatalf("delete attempt: %d %s", w.Code, w.Body.String())
	}
}

func TestProblemCatalogueDifficultyCategoryAndPremiumFilters(t *testing.T) {
	f := newFixture()
	f.repository.Problems["medium-free"] = model.Problem{ID: "medium-free", Number: 2, Title: "Free Graph", Difficulty: "medium", Categories: []string{"graphs"}, Premium: false, Revision: 1}
	f.repository.Problems["medium-premium"] = model.Problem{ID: "medium-premium", Number: 3, Title: "Premium Graph", Difficulty: "medium", Categories: []string{"graphs"}, Premium: true, Revision: 1}

	w := request(t, f, http.MethodGet, "/api/v2/leetcode-problems?difficulty=medium&category=graphs&premium=false", "student", "")
	page := decodePage[model.Problem](t, w.Body.Bytes())
	if w.Code != http.StatusOK || len(page.Items) != 1 || page.Items[0].ID != "medium-free" {
		t.Fatalf("free medium graph filter: %d %#v", w.Code, page.Items)
	}
	w = request(t, f, http.MethodGet, "/api/v2/leetcode-problems?difficulty=medium&category=graphs&premium=true", "student", "")
	page = decodePage[model.Problem](t, w.Body.Bytes())
	if w.Code != http.StatusOK || len(page.Items) != 1 || page.Items[0].ID != "medium-premium" {
		t.Fatalf("premium medium graph filter: %d %#v", w.Code, page.Items)
	}
}

func TestMockInterviewAllThreeSubtypesDateDurationNotesAndExpandedRoundTrip(t *testing.T) {
	f := newFixture()
	f.repository.Enrollments["student-member"] = model.Enrollment{ID: "student-member", SeasonID: "season", UserID: "student", Role: "student", State: "active", Revision: 1}
	body := `{"interviewee":{"userId":"other"},"occurredAt":"2026-08-13T00:00:00Z","durationMinutes":75,"notes":"<p><strong>expanded</strong><script>bad()</script></p>","rounds":[{"id":"behavioural","type":"behavioural","scores":{"behavioural":7}},{"id":"leetcode","type":"leetcode","problemId":"problem","scores":{"confirmQuestions":7,"algorithmDesign":7,"complexityAnalysis":7,"coding":7,"testing":7}},{"id":"custom","type":"custom","content":"<p>System design</p>","link":"https://rsp.test/custom","scores":{"custom":7}}]}`
	w := request(t, f, http.MethodPost, "/api/v2/mock-interviews", "student", body)
	if w.Code != http.StatusCreated {
		t.Fatalf("create three-subtype mock: %d %s", w.Code, w.Body.String())
	}
	var result struct {
		Interview mockinterviews.Interview `json:"interview"`
		Passed    bool                     `json:"passed"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Interview.InterviewerID != "student" || result.Interview.IntervieweeID != "other" || result.Interview.DurationMinutes != 75 || len(result.Interview.Rounds) != 3 || !result.Passed || bytes.Contains([]byte(result.Interview.Notes), []byte("script")) || len(f.repository.MockVersions) != 1 {
		t.Fatalf("expanded mock result=%#v versions=%#v", result, f.repository.MockVersions)
	}
}

func TestMockReceivedGivenAllExcludeUnrelatedPrivateRecords(t *testing.T) {
	f := newFixture()
	for _, userID := range []string{"student", "other", "third", "fourth"} {
		if _, ok := f.repository.Users[userID]; !ok {
			f.repository.Users[userID] = model.User{ID: userID, Slug: userID, Name: userID, AccountState: "active", Revision: 1}
		}
		f.repository.Enrollments[userID+"-member"] = model.Enrollment{ID: userID + "-member", SeasonID: "season", UserID: userID, Role: "student", State: "active", Revision: 1}
	}
	score := 7
	makeInterview := func(id, interviewer, interviewee string) mockinterviews.Interview {
		return mockinterviews.Interview{ID: id, InterviewerID: interviewer, IntervieweeID: interviewee, OccurredAt: time.Now().UTC(), DurationMinutes: 60, Rounds: []mockinterviews.Round{{ID: id + "-round", Type: mockinterviews.Behavioural, Scores: mockinterviews.Scores{Behavioural: &score}}}, Revision: 1}
	}
	f.repository.Mocks["received"] = makeInterview("received", "other", "student")
	f.repository.Mocks["given"] = makeInterview("given", "student", "other")
	f.repository.Mocks["unrelated"] = makeInterview("unrelated", "third", "fourth")

	for mode, want := range map[string][]string{"received": {"received"}, "given": {"given"}, "all": {"given", "received"}} {
		w := request(t, f, http.MethodGet, "/api/v2/mock-interviews?mode="+mode+"&sort=id:asc", "student", "")
		page := decodePage[mockinterviews.Interview](t, w.Body.Bytes())
		if w.Code != http.StatusOK || len(page.Items) != len(want) {
			t.Fatalf("mode %s status=%d items=%#v", mode, w.Code, page.Items)
		}
		for index, id := range want {
			if page.Items[index].ID != id {
				t.Fatalf("mode %s items=%#v", mode, page.Items)
			}
		}
	}
}

func TestMockSuccessfulDeleteIsAbsentFromActiveLists(t *testing.T) {
	f := newFixture()
	f.repository.Enrollments["student-member"] = model.Enrollment{ID: "student-member", SeasonID: "season", UserID: "student", Role: "student", State: "active", Revision: 1}
	score := 7
	f.repository.Mocks["delete-me"] = mockinterviews.Interview{ID: "delete-me", InterviewerID: "student", IntervieweeID: "other", OccurredAt: time.Now().UTC(), DurationMinutes: 60, Rounds: []mockinterviews.Round{{ID: "round", Type: mockinterviews.Behavioural, Scores: mockinterviews.Scores{Behavioural: &score}}}, Revision: 1}
	w := request(t, f, http.MethodDelete, "/api/v2/mock-interviews/delete-me?revision=1", "student", "")
	if w.Code != http.StatusNoContent {
		t.Fatalf("delete mock: %d %s", w.Code, w.Body.String())
	}
	w = request(t, f, http.MethodGet, "/api/v2/mock-interviews?mode=given", "student", "")
	page := decodePage[mockinterviews.Interview](t, w.Body.Bytes())
	if w.Code != http.StatusOK || len(page.Items) != 0 || page.TotalCount != 0 {
		t.Fatalf("deleted mock remained active: %d %#v", w.Code, page)
	}
}

func TestAdminActiveNonmemberAndGlobalRoleRevokeAuditReason(t *testing.T) {
	f := newFixture()
	f.repository.Users["active-nonmember"] = model.User{ID: "active-nonmember", Slug: "active-nonmember", Name: "Active Nonmember", Email: "new@example.com", AccountState: "active", Timezone: "Australia/Adelaide", Revision: 1}
	f.repository.GlobalRoleAssignments["other-director"] = accounts.GlobalRoleAssignment{ID: "other-director", UserID: "other", Role: "director", State: "active", Revision: 1}

	w := request(t, f, http.MethodGet, "/api/v2/admin/users?query=Active%20Nonmember", "admin", "")
	page := decodePage[model.User](t, w.Body.Bytes())
	if w.Code != http.StatusOK || len(page.Items) != 1 || page.Items[0].ID != "active-nonmember" {
		t.Fatalf("active nonmember admin list: %d %#v", w.Code, page.Items)
	}
	w = request(t, f, http.MethodDelete, "/api/v2/admin/users/other/global-roles/director?revision=1&reason=programme-ended", "admin", "")
	if w.Code != http.StatusOK || f.repository.GlobalRoleAssignments["other-director"].State != "revoked" {
		t.Fatalf("role revoke: %d %s", w.Code, w.Body.String())
	}
	found := false
	for _, audit := range f.repository.Audits {
		if audit.Action == "global_role.revoked" && audit.ActorID != nil && *audit.ActorID == "student" && audit.Data["reason"] == "programme-ended" {
			found = true
		}
	}
	if !found {
		t.Fatalf("role revoke audit missing: %#v", f.repository.Audits)
	}
}

func TestAdminEnrollmentPatchReasonedRemovalMentorshipPatchAndDelete(t *testing.T) {
	f := newFixture()
	for _, userID := range []string{"mentor-a", "mentor-b", "student-a", "student-b"} {
		f.repository.Users[userID] = model.User{ID: userID, Slug: userID, Name: userID, AccountState: "active", Revision: 1}
	}
	f.repository.Enrollments["mentor-a-enrollment"] = model.Enrollment{ID: "mentor-a-enrollment", SeasonID: "season", UserID: "mentor-a", Role: "mentor", State: "active", AssignmentState: "active", Revision: 1}
	f.repository.Enrollments["mentor-b-enrollment"] = model.Enrollment{ID: "mentor-b-enrollment", SeasonID: "season", UserID: "mentor-b", Role: "mentor", State: "active", AssignmentState: "active", Revision: 1}
	f.repository.Enrollments["student-a-enrollment"] = model.Enrollment{ID: "student-a-enrollment", SeasonID: "season", UserID: "student-a", Role: "student", StudentLevel: "beginner", State: "active", AssignmentState: "active", Revision: 1}
	f.repository.Enrollments["student-b-enrollment"] = model.Enrollment{ID: "student-b-enrollment", SeasonID: "season", UserID: "student-b", Role: "student", StudentLevel: "beginner", State: "active", AssignmentState: "active", Revision: 1}
	f.repository.Mentorships["mentorship"] = model.Mentorship{ID: "mentorship", SeasonID: "season", MentorUserID: "mentor-a", StudentUserID: "student-a", Revision: 1}

	w := request(t, f, http.MethodPatch, "/api/v2/seasons/season/members/student-a-enrollment", "coordinator", `{"role":"student","studentLevel":"advanced","revision":1}`)
	if w.Code != http.StatusOK || f.repository.Enrollments["student-a-enrollment"].StudentLevel != "advanced" {
		t.Fatalf("enrollment patch: %d %s", w.Code, w.Body.String())
	}
	w = request(t, f, http.MethodPost, "/api/v2/seasons/season/members/student-b-enrollment/remove", "coordinator", `{"reason":"withdrawal requested","revision":1}`)
	if w.Code != http.StatusOK || f.repository.Enrollments["student-b-enrollment"].State != "kicked" {
		t.Fatalf("enrollment removal: %d %s", w.Code, w.Body.String())
	}
	w = request(t, f, http.MethodPatch, "/api/v2/seasons/season/mentorships/mentorship", "coordinator", `{"mentorUserId":"mentor-b","studentUserId":"student-a","revision":1}`)
	if w.Code != http.StatusOK || f.repository.Mentorships["mentorship"].MentorUserID != "mentor-b" || f.repository.Mentorships["mentorship"].Revision != 2 {
		t.Fatalf("mentorship patch: %d %s", w.Code, w.Body.String())
	}
	w = request(t, f, http.MethodDelete, "/api/v2/seasons/season/mentorships/mentorship?revision=2", "coordinator", "")
	if w.Code != http.StatusNoContent {
		t.Fatalf("mentorship delete: %d %s", w.Code, w.Body.String())
	}
	if _, exists := f.repository.Mentorships["mentorship"]; exists {
		t.Fatal("deleted mentorship remains active")
	}
}
