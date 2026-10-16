//go:build integration

package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/magedmg/RSP-website/backend/internal/authz"
	"github.com/magedmg/RSP-website/backend/internal/dal"
	"github.com/magedmg/RSP-website/backend/internal/programme"
)

const (
	mondayCohortID            = "00000000-0000-7000-8000-000000000201"
	mondayCoordinatorID       = "00000000-0000-7000-8000-000000000202"
	mondayOutsiderID          = "00000000-0000-7000-8000-000000000203"
	mondayCohortMemberID      = "00000000-0000-7000-8000-000000000211"
	mondayCoordinatorMemberID = "00000000-0000-7000-8000-000000000212"
	mondayMentorshipID        = "00000000-0000-7000-8000-000000000221"
)

// mondayTestDate returns a calendar date the API accepts as a Monday update
// boundary. Adelaide is ahead of UTC, so a UTC date is never after the current
// Australia/Adelaide week.
func mondayTestDate(offsetDays int) string {
	return time.Now().UTC().AddDate(0, 0, offsetDays).Format(time.DateOnly)
}

// newMondayUpdateFixture seeds the season with a mentored second student, a
// coordinator and an outsider, then authenticates them all.
func newMondayUpdateFixture(t *testing.T) postgresFixture {
	t.Helper()
	fixture := newPostgresFixture(t)
	for _, statement := range []string{
		`INSERT INTO app.users(id,account_state) VALUES
			('` + mondayCohortID + `','active'),
			('` + mondayCoordinatorID + `','active'),
			('` + mondayOutsiderID + `','active')`,
		`INSERT INTO app.user_profiles(user_id,slug,display_name) VALUES
			('` + mondayCohortID + `','cohort','Cohort Student'),
			('` + mondayCoordinatorID + `','coordinator','Coordinator'),
			('` + mondayOutsiderID + `','outsider','Outsider')`,
		`INSERT INTO app.user_security(user_id,mfa_configured) VALUES
			('` + mondayCohortID + `',false),
			('` + mondayCoordinatorID + `',true),
			('` + mondayOutsiderID + `',false)`,
		`INSERT INTO app.enrollments(id,user_id,season_id,role,student_level,state) VALUES
			('` + mondayCohortMemberID + `','` + mondayCohortID + `','` + seasonID + `','student','beginner','active'),
			('` + mondayCoordinatorMemberID + `','` + mondayCoordinatorID + `','` + seasonID + `','coordinator','not_applicable','active')`,
		`INSERT INTO app.mentorships(id,season_id,mentor_enrollment_id,student_enrollment_id) VALUES
			('` + mondayMentorshipID + `','` + seasonID + `','` + otherMemberID + `','` + mondayCohortMemberID + `')`,
	} {
		if _, err := fixture.pool.Exec(context.Background(), statement); err != nil {
			t.Fatal(err)
		}
	}
	fixture.handler = mondayUpdateHandler(fixture.db, map[string]authz.Actor{
		"student": mondayActor(studentID, authz.Enrollment{SeasonID: seasonID, Role: authz.Student, State: authz.Active}),
		"cohort":  mondayActor(mondayCohortID, authz.Enrollment{SeasonID: seasonID, Role: authz.Student, State: authz.Active}),
		"mentor":  mondayActor(otherID, authz.Enrollment{SeasonID: seasonID, Role: authz.Mentor, State: authz.Active}),
		"coordinator": mondayActor(mondayCoordinatorID, authz.Enrollment{
			SeasonID: seasonID, Role: authz.Coordinator, State: authz.Active,
		}),
		"outsider": mondayActor(mondayOutsiderID),
	})
	return fixture
}

func mondayActor(userID string, enrollments ...authz.Enrollment) authz.Actor {
	return authz.Actor{UserID: userID, EmailVerified: true, AccountState: authz.AccountActive, Enrollments: enrollments}
}

func mondayUpdateHandler(db *dal.Store, actors map[string]authz.Actor) http.Handler {
	return New(Config{
		DB: db,
		Authenticator: AuthenticatorFunc(func(_ context.Context, token string) (authz.Actor, error) {
			actor, ok := actors[token]
			if !ok {
				return authz.Actor{}, errors.New("missing test actor")
			}
			return actor, nil
		}),
		PublicOrigin: "https://rsp.test",
		CursorSecret: []byte("0123456789abcdef"),
	}).Handler()
}

func mondayUpdatesPath() string { return "/api/v2/seasons/" + seasonID + "/monday-updates" }

func mondayReviewPath() string { return "/api/v2/seasons/" + seasonID + "/monday-update-review" }

func mondayReviewQuery(extra string) string {
	return mondayReviewPath() + "?startDate=" + mondayTestDate(-1) + "&endDate=" + mondayTestDate(0) + extra
}

func createMondayUpdateFor(t *testing.T, fixture postgresFixture, actor, content string) programme.MondayUpdateRecord {
	t.Helper()
	return createMondayUpdateRange(t, fixture, actor, mondayTestDate(-1), mondayTestDate(0), content)
}

func createMondayUpdateRange(t *testing.T, fixture postgresFixture, actor, startDate, endDate, content string) programme.MondayUpdateRecord {
	t.Helper()
	response := testRequest(t, fixture.handler, http.MethodPost, mondayUpdatesPath(), actor,
		`{"startDate":"`+startDate+`","endDate":"`+endDate+`","contentHtml":"`+content+`"}`)
	if response.Code != http.StatusCreated {
		t.Fatalf("create status=%d body=%s", response.Code, response.Body)
	}
	var created programme.MondayUpdateRecord
	if err := json.Unmarshal(response.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	return created
}

func listMondayUpdates(t *testing.T, fixture postgresFixture, actor, extra string) Page[programme.MondayUpdateRecord] {
	t.Helper()
	path := mondayUpdatesPath() + "?startDate=" + mondayTestDate(-1) + "&endDate=" + mondayTestDate(0) + extra
	response := testRequest(t, fixture.handler, http.MethodGet, path, actor, "")
	if response.Code != http.StatusOK {
		t.Fatalf("list status=%d body=%s", response.Code, response.Body)
	}
	var page Page[programme.MondayUpdateRecord]
	if err := json.Unmarshal(response.Body.Bytes(), &page); err != nil {
		t.Fatal(err)
	}
	return page
}

func TestMondayUpdateStudentScopeHidesMarks(t *testing.T) {
	fixture := newMondayUpdateFixture(t)
	own := createMondayUpdateFor(t, fixture, "student", `<p>My week</p><script>bad()</script>`)
	if own.Student.ID != studentID || own.Student.Name != "Student" || own.SeasonID != seasonID {
		t.Fatalf("created=%+v", own)
	}
	if !strings.Contains(own.ContentHTML, "<p>My week</p>") || strings.Contains(own.ContentHTML, "bad()") {
		t.Fatalf("content=%q", own.ContentHTML)
	}
	if own.Marks == nil || len(own.Marks) != 0 || own.Activity.Attempts == nil || own.Activity.Mocks == nil {
		t.Fatalf("created=%+v", own)
	}
	mentee := createMondayUpdateFor(t, fixture, "cohort", "<p>Cohort week</p>")

	// A mark exists, yet a student response never carries marks or comments.
	if _, err := fixture.pool.Exec(context.Background(), `INSERT INTO app.monday_update_marks(update_id,mentor_user_id,mark,comment_html)
		VALUES($1,$2,'on_track','<p>private mentor comment</p>')`, own.ID, otherID); err != nil {
		t.Fatal(err)
	}
	page := listMondayUpdates(t, fixture, "student", "")
	if page.TotalCount != 1 || len(page.Items) != 1 || page.Items[0].ID != own.ID || page.Items[0].Student.ID != studentID {
		t.Fatalf("page=%+v", page)
	}
	if len(page.Items[0].Marks) != 0 {
		t.Fatalf("student marks=%+v", page.Items[0].Marks)
	}
	// The student selector can never widen a student's own scope.
	page = listMondayUpdates(t, fixture, "student", "&studentId="+mondayCohortID)
	if page.TotalCount != 1 || len(page.Items) != 1 || page.Items[0].ID != own.ID {
		t.Fatalf("narrowed page=%+v", page)
	}
	cohortPage := listMondayUpdates(t, fixture, "cohort", "")
	if cohortPage.TotalCount != 1 || len(cohortPage.Items) != 1 || cohortPage.Items[0].ID != mentee.ID {
		t.Fatalf("cohort page=%+v", cohortPage)
	}

	// Deleting someone else's update is refused and leaves it in place.
	response := testRequest(t, fixture.handler, http.MethodDelete, mondayUpdatesPath()+"/"+mentee.ID, "student", "")
	if response.Code != http.StatusForbidden || errorMessage(t, response) != "Only the author can delete a Monday update." {
		t.Fatalf("foreign delete status=%d body=%s", response.Code, response.Body)
	}
	if page := listMondayUpdates(t, fixture, "cohort", ""); page.TotalCount != 1 {
		t.Fatalf("mentee update disappeared: %+v", page)
	}
	response = testRequest(t, fixture.handler, http.MethodDelete, mondayUpdatesPath()+"/"+own.ID, "student", "")
	if response.Code != http.StatusNoContent || response.Body.Len() != 0 {
		t.Fatalf("delete status=%d body=%s", response.Code, response.Body)
	}
	if page := listMondayUpdates(t, fixture, "student", ""); page.TotalCount != 0 || len(page.Items) != 0 {
		t.Fatalf("page after delete=%+v", page)
	}
	response = testRequest(t, fixture.handler, http.MethodDelete, mondayUpdatesPath()+"/"+own.ID, "student", "")
	if response.Code != http.StatusNotFound {
		t.Fatalf("repeated delete status=%d body=%s", response.Code, response.Body)
	}
}

func TestMondayUpdateMentorScopeAndMarkUpsert(t *testing.T) {
	fixture := newMondayUpdateFixture(t)
	own := createMondayUpdateFor(t, fixture, "student", "<p>Student week</p>")
	mentee := createMondayUpdateFor(t, fixture, "cohort", "<p>Cohort week</p>")
	markPath := mondayUpdatesPath() + "/" + mentee.ID + "/mark"

	// A mentor may not mark a student they do not mentor, even in their season.
	response := testRequest(t, fixture.handler, http.MethodPut, mondayUpdatesPath()+"/"+own.ID+"/mark", "mentor",
		`{"mark":"on_track","commentHtml":"<p>Not mine</p>"}`)
	if response.Code != http.StatusForbidden || errorMessage(t, response) != "Only a mentor of this student can mark this update." {
		t.Fatalf("unmentored mark status=%d body=%s", response.Code, response.Body)
	}
	response = testRequest(t, fixture.handler, http.MethodPut, markPath, "mentor", `{"mark":"excelling","commentHtml":""}`)
	if response.Code != http.StatusBadRequest || errorMessage(t, response) != "mark must be one of way_ahead, doing_good, on_track, a_bit_behind or at_risk" {
		t.Fatalf("invalid mark status=%d body=%s", response.Code, response.Body)
	}

	response = testRequest(t, fixture.handler, http.MethodPut, markPath, "mentor",
		`{"mark":"on_track","commentHtml":"<p>Steady</p><script>bad()</script>"}`)
	if response.Code != http.StatusOK {
		t.Fatalf("mark status=%d body=%s", response.Code, response.Body)
	}
	var first programme.MondayUpdateMarkRecord
	if err := json.Unmarshal(response.Body.Bytes(), &first); err != nil {
		t.Fatal(err)
	}
	if first.ID == "" || first.UpdateID != "" || first.Mentor.ID != otherID || first.Mentor.Name != "Other" || first.Mark != "on_track" {
		t.Fatalf("first mark=%+v", first)
	}
	if !strings.Contains(first.CommentHTML, "<p>Steady</p>") || strings.Contains(first.CommentHTML, "bad()") {
		t.Fatalf("comment=%q", first.CommentHTML)
	}

	// The second mark replaces the mentor's previous verdict on the same row.
	response = testRequest(t, fixture.handler, http.MethodPut, markPath, "mentor", `{"mark":"a_bit_behind","commentHtml":"<p>Slower week</p>"}`)
	if response.Code != http.StatusOK {
		t.Fatalf("second mark status=%d body=%s", response.Code, response.Body)
	}
	var second programme.MondayUpdateMarkRecord
	if err := json.Unmarshal(response.Body.Bytes(), &second); err != nil {
		t.Fatal(err)
	}
	if second.ID != first.ID || second.Mark != "a_bit_behind" || second.CommentHTML != "<p>Slower week</p>" {
		t.Fatalf("second mark=%+v first=%+v", second, first)
	}
	var rows int
	if err := fixture.pool.QueryRow(context.Background(),
		`SELECT count(*) FROM app.monday_update_marks WHERE update_id=$1 AND mentor_user_id=$2`, mentee.ID, otherID).Scan(&rows); err != nil || rows != 1 {
		t.Fatalf("mark rows=%d error=%v", rows, err)
	}

	// The mentor sees only their mentees, with marks, and cannot narrow to a
	// student they do not mentor.
	page := listMondayUpdates(t, fixture, "mentor", "")
	if page.TotalCount != 1 || len(page.Items) != 1 || page.Items[0].ID != mentee.ID {
		t.Fatalf("mentor page=%+v", page)
	}
	if len(page.Items[0].Marks) != 1 || page.Items[0].Marks[0].Mark != "a_bit_behind" || page.Items[0].Marks[0].Mentor.ID != otherID {
		t.Fatalf("mentor marks=%+v", page.Items[0].Marks)
	}
	narrowed := listMondayUpdates(t, fixture, "mentor", "&studentId="+studentID)
	if narrowed.TotalCount != 0 || len(narrowed.Items) != 0 {
		t.Fatalf("mentor narrowed page=%+v", narrowed)
	}
}

func TestMondayUpdateReviewAuthorizationAndRows(t *testing.T) {
	fixture := newMondayUpdateFixture(t)
	own := createMondayUpdateFor(t, fixture, "student", "<p>Student week</p>")
	mentee := createMondayUpdateFor(t, fixture, "cohort", "<p>Cohort week</p>")
	if response := testRequest(t, fixture.handler, http.MethodPut, mondayUpdatesPath()+"/"+mentee.ID+"/mark", "mentor",
		`{"mark":"doing_good","commentHtml":"<p>Good</p>"}`); response.Code != http.StatusOK {
		t.Fatalf("mark status=%d body=%s", response.Code, response.Body)
	}

	// Only season administrators may read the review.
	for _, actor := range []string{"student", "cohort", "mentor", "outsider"} {
		response := testRequest(t, fixture.handler, http.MethodGet, mondayReviewQuery(""), actor, "")
		if response.Code != http.StatusForbidden || errorMessage(t, response) != "Season administrator access is required." {
			t.Fatalf("actor=%s status=%d body=%s", actor, response.Code, response.Body)
		}
	}

	response := testRequest(t, fixture.handler, http.MethodGet, mondayReviewQuery(""), "coordinator", "")
	if response.Code != http.StatusOK {
		t.Fatalf("review status=%d body=%s", response.Code, response.Body)
	}
	var page Page[programme.MondayUpdateReviewRow]
	if err := json.Unmarshal(response.Body.Bytes(), &page); err != nil {
		t.Fatal(err)
	}
	// One row per enrolled student, ordered by display name, including the
	// student whose marks are hidden from them.
	if page.TotalCount != 2 || len(page.Items) != 2 {
		t.Fatalf("review page=%+v", page)
	}
	if page.Items[0].Student.Name != "Cohort Student" || page.Items[1].Student.Name != "Student" {
		t.Fatalf("review order=%q,%q", page.Items[0].Student.Name, page.Items[1].Student.Name)
	}
	cohort, student := page.Items[0], page.Items[1]
	if cohort.UpdateCount != 1 || len(cohort.Updates) != 1 || cohort.Updates[0].ID != mentee.ID ||
		len(cohort.Updates[0].Marks) != 1 || cohort.Updates[0].Marks[0].Mark != "doing_good" {
		t.Fatalf("cohort row=%+v", cohort)
	}
	if student.UpdateCount != 1 || len(student.Updates) != 1 || student.Updates[0].ID != own.ID || len(student.Updates[0].Marks) != 0 {
		t.Fatalf("student row=%+v", student)
	}

	// The range is required and the cursor binds to it.
	response = testRequest(t, fixture.handler, http.MethodGet, mondayReviewPath(), "coordinator", "")
	if response.Code != http.StatusBadRequest || errorMessage(t, response) != "dates must use the YYYY-MM-DD format" {
		t.Fatalf("missing range status=%d body=%s", response.Code, response.Body)
	}
	response = testRequest(t, fixture.handler, http.MethodGet, mondayReviewQuery("&limit=1"), "coordinator", "")
	if response.Code != http.StatusOK {
		t.Fatalf("first page status=%d body=%s", response.Code, response.Body)
	}
	var firstPage Page[programme.MondayUpdateReviewRow]
	if err := json.Unmarshal(response.Body.Bytes(), &firstPage); err != nil {
		t.Fatal(err)
	}
	if len(firstPage.Items) != 1 || firstPage.Items[0].Student.ID != mondayCohortID || !firstPage.PageInfo.HasMore || firstPage.PageInfo.NextCursor == nil {
		t.Fatalf("first page=%+v", firstPage)
	}
	response = testRequest(t, fixture.handler, http.MethodGet, mondayReviewQuery("&limit=1&cursor="+*firstPage.PageInfo.NextCursor), "coordinator", "")
	if response.Code != http.StatusOK {
		t.Fatalf("second page status=%d body=%s", response.Code, response.Body)
	}
	var secondPage Page[programme.MondayUpdateReviewRow]
	if err := json.Unmarshal(response.Body.Bytes(), &secondPage); err != nil {
		t.Fatal(err)
	}
	if len(secondPage.Items) != 1 || secondPage.Items[0].Student.ID != studentID || secondPage.PageInfo.HasMore {
		t.Fatalf("second page=%+v", secondPage)
	}
	response = testRequest(t, fixture.handler, http.MethodGet, mondayReviewPath()+"?startDate="+mondayTestDate(-2)+"&endDate="+mondayTestDate(-1)+
		"&limit=1&cursor="+*firstPage.PageInfo.NextCursor, "coordinator", "")
	if response.Code != http.StatusBadRequest {
		t.Fatalf("mismatched cursor status=%d body=%s", response.Code, response.Body)
	}
}

func TestMondayUpdateValidationAndEnrollment(t *testing.T) {
	for _, test := range []struct {
		name    string
		actor   string
		method  string
		path    string
		body    string
		message string
		status  int
	}{
		{
			name: "reversed range", actor: "student", method: http.MethodPost, path: mondayUpdatesPath(),
			body:    `{"startDate":"` + mondayTestDate(0) + `","endDate":"` + mondayTestDate(-1) + `","contentHtml":"<p>x</p>"}`,
			message: "endDate must not be before startDate", status: http.StatusBadRequest,
		},
		{
			name: "start after the current week", actor: "student", method: http.MethodPost, path: mondayUpdatesPath(),
			body:    `{"startDate":"` + mondayTestDate(30) + `","endDate":"` + mondayTestDate(30) + `","contentHtml":"<p>x</p>"}`,
			message: "startDate cannot be after the current Australia/Adelaide week", status: http.StatusBadRequest,
		},
		{
			name: "malformed date", actor: "student", method: http.MethodPost, path: mondayUpdatesPath(),
			body:    `{"startDate":"2026-13-01","endDate":"2026-13-02","contentHtml":"<p>x</p>"}`,
			message: "dates must use the YYYY-MM-DD format", status: http.StatusBadRequest,
		},
		{
			name: "whitespace content", actor: "student", method: http.MethodPost, path: mondayUpdatesPath(),
			body:    `{"startDate":"` + mondayTestDate(0) + `","endDate":"` + mondayTestDate(0) + `","contentHtml":"  "}`,
			message: "contentHtml is required", status: http.StatusBadRequest,
		},
		{
			name: "unknown field", actor: "student", method: http.MethodPost, path: mondayUpdatesPath(),
			body:    `{"startDate":"` + mondayTestDate(0) + `","endDate":"` + mondayTestDate(0) + `","contentHtml":"<p>x</p>","studentId":"` + studentID + `"}`,
			message: `json: unknown field "studentId"`, status: http.StatusBadRequest,
		},
		{
			name: "mentor cannot post", actor: "mentor", method: http.MethodPost, path: mondayUpdatesPath(),
			body:    `{"startDate":"` + mondayTestDate(0) + `","endDate":"` + mondayTestDate(0) + `","contentHtml":"<p>x</p>"}`,
			message: "Only an active student enrollment can post a Monday update in this season.", status: http.StatusForbidden,
		},
		{
			name: "unenrolled actor cannot post", actor: "outsider", method: http.MethodPost, path: mondayUpdatesPath(),
			body:    `{"startDate":"` + mondayTestDate(0) + `","endDate":"` + mondayTestDate(0) + `","contentHtml":"<p>x</p>"}`,
			message: "Only an active student enrollment can post a Monday update in this season.", status: http.StatusForbidden,
		},
		{
			name: "unenrolled actor cannot list", actor: "outsider", method: http.MethodGet,
			path:    mondayUpdatesPath() + "?startDate=" + mondayTestDate(-1) + "&endDate=" + mondayTestDate(0),
			message: "The current account cannot access this season's Monday updates.", status: http.StatusForbidden,
		},
		{
			name: "missing range", actor: "student", method: http.MethodGet, path: mondayUpdatesPath(),
			message: "dates must use the YYYY-MM-DD format", status: http.StatusBadRequest,
		},
		{
			name: "malformed range", actor: "student", method: http.MethodGet,
			path:    mondayUpdatesPath() + "?startDate=2026-13-01&endDate=2026-01-01",
			message: "dates must use the YYYY-MM-DD format", status: http.StatusBadRequest,
		},
		{
			name: "invalid student filter", actor: "student", method: http.MethodGet,
			path:    mondayUpdatesPath() + "?startDate=" + mondayTestDate(-1) + "&endDate=" + mondayTestDate(0) + "&studentId=not-a-uuid",
			message: "invalid studentId filter", status: http.StatusBadRequest,
		},
		{
			name: "invalid mark", actor: "mentor", method: http.MethodPut, path: mondayUpdatesPath() + "/" + "00000000-0000-7000-8000-000000000299" + "/mark",
			body:    `{"mark":"nope","commentHtml":""}`,
			message: "mark must be one of way_ahead, doing_good, on_track, a_bit_behind or at_risk", status: http.StatusBadRequest,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			fixture := newMondayUpdateFixture(t)
			response := testRequest(t, fixture.handler, test.method, test.path, test.actor, test.body)
			if response.Code != test.status || errorMessage(t, response) != test.message {
				t.Fatalf("status=%d body=%s", response.Code, response.Body)
			}
		})
	}
}

func TestMondayUpdateRequiresActiveStudentEnrollment(t *testing.T) {
	fixture := newMondayUpdateFixture(t)
	body := `{"startDate":"` + mondayTestDate(0) + `","endDate":"` + mondayTestDate(0) + `","contentHtml":"<p>x</p>"}`
	for _, state := range []authz.EnrollmentState{authz.Completed, authz.Withdrawn} {
		handler := mondayUpdateHandler(fixture.db, map[string]authz.Actor{
			"student": mondayActor(studentID, authz.Enrollment{SeasonID: seasonID, Role: authz.Student, State: state}),
		})
		response := testRequest(t, handler, http.MethodPost, mondayUpdatesPath(), "student", body)
		if response.Code != http.StatusForbidden || errorMessage(t, response) != "Only an active student enrollment can post a Monday update in this season." {
			t.Fatalf("state=%s status=%d body=%s", state, response.Code, response.Body)
		}
		response = testRequest(t, handler, http.MethodGet, mondayUpdatesPath()+"?startDate="+mondayTestDate(-1)+"&endDate="+mondayTestDate(0), "student", "")
		if response.Code != http.StatusForbidden || errorMessage(t, response) != "The current account cannot access this season's Monday updates." {
			t.Fatalf("state=%s list status=%d body=%s", state, response.Code, response.Body)
		}
	}
	// The same account with an active enrollment may post.
	active := mondayUpdateHandler(fixture.db, map[string]authz.Actor{
		"student": mondayActor(studentID, authz.Enrollment{SeasonID: seasonID, Role: authz.Student, State: authz.Active}),
	})
	if response := testRequest(t, active, http.MethodPost, mondayUpdatesPath(), "student", body); response.Code != http.StatusCreated {
		t.Fatalf("active status=%d body=%s", response.Code, response.Body)
	}
}
