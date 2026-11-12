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

	"github.com/Ravis-Study-Program/RSP-website/backend/internal/authz"
	"github.com/Ravis-Study-Program/RSP-website/backend/internal/dal"
	"github.com/Ravis-Study-Program/RSP-website/backend/internal/program"
)

const (
	mondayCohortID            = "00000000-0000-7000-8000-000000000201"
	mondayCoordinatorID       = "00000000-0000-7000-8000-000000000202"
	mondayOutsiderID          = "00000000-0000-7000-8000-000000000203"
	mondayCohortMemberID      = "00000000-0000-7000-8000-000000000211"
	mondayCoordinatorMemberID = "00000000-0000-7000-8000-000000000212"
	mondayMentorshipID        = "00000000-0000-7000-8000-000000000221"
)

// mondayTestDate offsets from the last completed Sunday in Adelaide.
func mondayTestDate(offsetDays int) string {
	end, _ := time.Parse(time.DateOnly, program.CurrentMondayUpdateWeekEnd(time.Now()))
	return end.AddDate(0, 0, offsetDays-7).Format(time.DateOnly)
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
	if _, err := fixture.pool.Exec(context.Background(), `INSERT INTO app.season_weeks(season_id,week_number,start_at,end_at)
		VALUES($1,1,$2::date AT TIME ZONE 'Australia/Adelaide',($3::date + 1) AT TIME ZONE 'Australia/Adelaide' - interval '1 second')`, seasonID, mondayTestDate(-6), mondayTestDate(0)); err != nil {
		t.Fatal(err)
	}
	// The fixture's week is due on the Monday after it; pin the late lock's
	// clock to that Monday so its update can still be posted.
	pinMondayNow(t, mondayTestDate(1))
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

func createMondayUpdateFor(t *testing.T, fixture postgresFixture, actor, content string) program.MondayUpdateRecord {
	t.Helper()
	return createMondayUpdateRange(t, fixture, actor, mondayTestDate(-6), mondayTestDate(0), content)
}

func createMondayUpdateRange(t *testing.T, fixture postgresFixture, actor, startDate, endDate, content string) program.MondayUpdateRecord {
	t.Helper()
	response := testRequest(t, fixture.handler, http.MethodPost, mondayUpdatesPath(), actor,
		`{"startDate":"`+startDate+`","endDate":"`+endDate+`","contentHtml":"`+content+`","hours":12.5}`)
	if response.Code != http.StatusCreated {
		t.Fatalf("create status=%d body=%s", response.Code, response.Body)
	}
	var created program.MondayUpdateRecord
	if err := json.Unmarshal(response.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	return created
}

func listMondayUpdates(t *testing.T, fixture postgresFixture, actor, extra string) Page[program.MondayUpdateRecord] {
	t.Helper()
	path := mondayUpdatesPath() + "?startDate=" + mondayTestDate(-1) + "&endDate=" + mondayTestDate(0) + extra
	response := testRequest(t, fixture.handler, http.MethodGet, path, actor, "")
	if response.Code != http.StatusOK {
		t.Fatalf("list status=%d body=%s", response.Code, response.Body)
	}
	var page Page[program.MondayUpdateRecord]
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
	if own.Hours != 12.5 {
		t.Fatalf("hours=%v", own.Hours)
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
	if response.Code != http.StatusBadRequest || errorMessage(t, response) != "mark must be one of way_ahead, doing_good, on_track, a_bit_behind, at_risk or kicked, or empty to clear it" {
		t.Fatalf("invalid mark status=%d body=%s", response.Code, response.Body)
	}

	response = testRequest(t, fixture.handler, http.MethodPut, markPath, "mentor",
		`{"mark":"on_track","commentHtml":"<p>Steady</p><script>bad()</script>"}`)
	if response.Code != http.StatusOK {
		t.Fatalf("mark status=%d body=%s", response.Code, response.Body)
	}
	var first program.MondayUpdateMarkRecord
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
	var second program.MondayUpdateMarkRecord
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
	var page Page[program.MondayUpdateReviewRow]
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
	var firstPage Page[program.MondayUpdateReviewRow]
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
	var secondPage Page[program.MondayUpdateReviewRow]
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
			body:    `{"startDate":"` + mondayTestDate(-6) + `","endDate":"` + mondayTestDate(-7) + `","contentHtml":"<p>x</p>"}`,
			message: "updates must cover one Monday to Sunday week", status: http.StatusBadRequest,
		},
		{
			name: "a week not yet started", actor: "student", method: http.MethodPost, path: mondayUpdatesPath(),
			body:    `{"startDate":"` + mondayTestDate(8) + `","endDate":"` + mondayTestDate(14) + `","contentHtml":"<p>x</p>"}`,
			message: "updates can be written once the week has started", status: http.StatusBadRequest,
		},
		{
			name: "malformed date", actor: "student", method: http.MethodPost, path: mondayUpdatesPath(),
			body:    `{"startDate":"2026-13-01","endDate":"2026-13-02","contentHtml":"<p>x</p>"}`,
			message: "dates must use the YYYY-MM-DD format", status: http.StatusBadRequest,
		},
		{
			name: "missing hours", actor: "student", method: http.MethodPost, path: mondayUpdatesPath(),
			body:    `{"startDate":"` + mondayTestDate(-6) + `","endDate":"` + mondayTestDate(0) + `","contentHtml":"<p>x</p>"}`,
			message: "hours is required", status: http.StatusBadRequest,
		},
		{
			name: "hours above a week", actor: "student", method: http.MethodPost, path: mondayUpdatesPath(),
			body:    `{"startDate":"` + mondayTestDate(-6) + `","endDate":"` + mondayTestDate(0) + `","contentHtml":"<p>x</p>","hours":168.5}`,
			message: "hours must be between 0 and 168 in steps of 0.5", status: http.StatusBadRequest,
		},
		{
			name: "negative hours", actor: "student", method: http.MethodPost, path: mondayUpdatesPath(),
			body:    `{"startDate":"` + mondayTestDate(-6) + `","endDate":"` + mondayTestDate(0) + `","contentHtml":"<p>x</p>","hours":-0.5}`,
			message: "hours must be between 0 and 168 in steps of 0.5", status: http.StatusBadRequest,
		},
		{
			name: "hours off the half-hour step", actor: "student", method: http.MethodPost, path: mondayUpdatesPath(),
			body:    `{"startDate":"` + mondayTestDate(-6) + `","endDate":"` + mondayTestDate(0) + `","contentHtml":"<p>x</p>","hours":7.3}`,
			message: "hours must be between 0 and 168 in steps of 0.5", status: http.StatusBadRequest,
		},
		{
			name: "unknown field", actor: "student", method: http.MethodPost, path: mondayUpdatesPath(),
			body:    `{"startDate":"` + mondayTestDate(-6) + `","endDate":"` + mondayTestDate(0) + `","contentHtml":"<p>x</p>","studentId":"` + studentID + `"}`,
			message: `json: unknown field "studentId"`, status: http.StatusBadRequest,
		},
		{
			name: "mentor cannot post", actor: "mentor", method: http.MethodPost, path: mondayUpdatesPath(),
			body:    `{"startDate":"` + mondayTestDate(-6) + `","endDate":"` + mondayTestDate(0) + `","contentHtml":"<p>x</p>"}`,
			message: "Only an active student enrollment can post a Monday update in this season.", status: http.StatusForbidden,
		},
		{
			name: "unenrolled actor cannot post", actor: "outsider", method: http.MethodPost, path: mondayUpdatesPath(),
			body:    `{"startDate":"` + mondayTestDate(-6) + `","endDate":"` + mondayTestDate(0) + `","contentHtml":"<p>x</p>"}`,
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
			message: "mark must be one of way_ahead, doing_good, on_track, a_bit_behind, at_risk or kicked, or empty to clear it", status: http.StatusBadRequest,
		},
		{
			name: "mentor cannot read range activity", actor: "mentor", method: http.MethodGet,
			path:    mondayActivityQuery(mondayTestDate(-1), mondayTestDate(0)),
			message: "Only an active student enrollment can read its Monday update activity in this season.", status: http.StatusForbidden,
		},
		{
			name: "reversed activity range", actor: "student", method: http.MethodGet,
			path:    mondayActivityQuery(mondayTestDate(0), mondayTestDate(-1)),
			message: "endDate must not be before startDate", status: http.StatusBadRequest,
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
	body := `{"startDate":"` + mondayTestDate(-6) + `","endDate":"` + mondayTestDate(0) + `","contentHtml":"<p>x</p>","hours":30}`
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

func mondayActivityQuery(startDate, endDate string) string {
	return "/api/v2/seasons/" + seasonID + "/monday-update-activity?startDate=" + startDate + "&endDate=" + endDate
}

// The range activity uses Adelaide calendar days and only the caller's own rows.
func TestMondayUpdateRangeActivityBoundsAndOwnership(t *testing.T) {
	fixture := newMondayUpdateFixture(t)
	for _, statement := range []string{
		`INSERT INTO app.problems(id,title) VALUES('00000000-0000-7000-8000-000000000501','Range Problem')`,
		`INSERT INTO app.leetcode_problems(problem_id,leetcode_number,difficulty) VALUES('00000000-0000-7000-8000-000000000501',9901,'hard')`,
		// 2026-03-02 00:10 and 2026-03-08 23:50 Adelaide (UTC+10:30) are inside;
		// one minute either side of the range is outside.
		`INSERT INTO app.problem_attempts(id,user_id,problem_id,attempted_at,time_taken_minutes,outcome) VALUES
			('00000000-0000-7000-8000-000000000511','` + studentID + `','00000000-0000-7000-8000-000000000501','2026-03-01T13:40:00Z',30,'independently_solved'),
			('00000000-0000-7000-8000-000000000512','` + studentID + `','00000000-0000-7000-8000-000000000501','2026-03-08T13:20:00Z',30,'solved_with_hints'),
			('00000000-0000-7000-8000-000000000513','` + studentID + `','00000000-0000-7000-8000-000000000501','2026-03-01T13:29:00Z',30,'independently_solved'),
			('00000000-0000-7000-8000-000000000514','` + studentID + `','00000000-0000-7000-8000-000000000501','2026-03-08T13:31:00Z',30,'independently_solved'),
			('00000000-0000-7000-8000-000000000515','` + mondayCohortID + `','00000000-0000-7000-8000-000000000501','2026-03-04T03:00:00Z',30,'independently_solved')`,
		`INSERT INTO app.mock_interviews(id,interviewer_user_id,interviewee_user_id,scheduled_at,duration_minutes) VALUES
			('00000000-0000-7000-8000-000000000521','` + otherID + `','` + studentID + `','2026-03-05T08:00:00Z',60),
			('00000000-0000-7000-8000-000000000522','` + studentID + `','` + mondayCohortID + `','2026-03-05T09:00:00Z',45)`,
	} {
		if _, err := fixture.pool.Exec(context.Background(), statement); err != nil {
			t.Fatal(err)
		}
	}
	response := testRequest(t, fixture.handler, http.MethodGet, mondayActivityQuery("2026-03-02", "2026-03-08"), "student", "")
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body)
	}
	var activity program.MondayUpdateActivity
	if err := json.Unmarshal(response.Body.Bytes(), &activity); err != nil {
		t.Fatal(err)
	}
	if len(activity.Attempts) != 2 || activity.Attempts[0].ID != "00000000-0000-7000-8000-000000000511" ||
		activity.Attempts[1].ID != "00000000-0000-7000-8000-000000000512" || activity.Attempts[0].Difficulty != "hard" {
		t.Fatalf("attempts=%+v", activity.Attempts)
	}
	// Only mocks where the caller is the interviewee count, as for updates.
	if len(activity.Mocks) != 1 || activity.Mocks[0].ID != "00000000-0000-7000-8000-000000000521" || activity.Mocks[0].DurationMinutes != 60 {
		t.Fatalf("mocks=%+v", activity.Mocks)
	}

	// An empty range still returns arrays, never null.
	response = testRequest(t, fixture.handler, http.MethodGet, mondayActivityQuery("2026-05-01", "2026-05-01"), "student", "")
	if response.Code != http.StatusOK || strings.TrimSpace(response.Body.String()) != `{"attempts":[],"mocks":[]}` {
		t.Fatalf("empty status=%d body=%q", response.Code, response.Body)
	}
}

func TestMondayUpdateFixedWeeksAndConcurrentSubmission(t *testing.T) {
	fixture := newMondayUpdateFixture(t)
	body := func(start, end string) string {
		return `{"startDate":"` + start + `","endDate":"` + end + `","contentHtml":"<p>Weekly progress</p>","hours":30}`
	}
	// Completed dates outside the configured program cannot be posted.
	response := testRequest(t, fixture.handler, http.MethodPost, mondayUpdatesPath(), "student", body(mondayTestDate(-20), mondayTestDate(-14)))
	if response.Code != http.StatusBadRequest {
		t.Fatalf("unconfigured week: %d %s", response.Code, response.Body)
	}
	// A configured missed week remains available for a late submission.
	if _, err := fixture.pool.Exec(context.Background(), `INSERT INTO app.season_weeks(season_id,week_number,start_at,end_at)
  VALUES($1,2,$2::date AT TIME ZONE 'Australia/Adelaide',($3::date + 1) AT TIME ZONE 'Australia/Adelaide' - interval '1 second')`, seasonID, mondayTestDate(-20), mondayTestDate(-14)); err != nil {
		t.Fatal(err)
	}
	// Past its due date it is locked until a mentor or coordinator unlocks it.
	if response = testRequest(t, fixture.handler, http.MethodPost, mondayUpdatesPath(), "student", body(mondayTestDate(-20), mondayTestDate(-14))); response.Code != http.StatusConflict {
		t.Fatalf("locked late week: %d %s", response.Code, response.Body)
	}
	if response = testRequest(t, fixture.handler, http.MethodPost, "/api/v2/seasons/"+seasonID+"/monday-update-unlocks", "coordinator",
		`{"startDate":"`+mondayTestDate(-20)+`","endDate":"`+mondayTestDate(-14)+`","studentId":"`+studentID+`"}`); response.Code != http.StatusCreated {
		t.Fatalf("coordinator unlock: %d %s", response.Code, response.Body)
	}
	late := createMondayUpdateRange(t, fixture, "student", mondayTestDate(-20), mondayTestDate(-14), "<p>Late</p>")
	if late.StartDate != mondayTestDate(-20) {
		t.Fatalf("late=%+v", late)
	}
	// Two simultaneous clicks/tabs still produce exactly one weekly update.
	statuses := make(chan int, 2)
	start := make(chan struct{})
	for i := 0; i < 2; i++ {
		go func() {
			<-start
			statuses <- testRequest(t, fixture.handler, http.MethodPost, mondayUpdatesPath(), "student", body(mondayTestDate(-6), mondayTestDate(0))).Code
		}()
	}
	close(start)
	first, second := <-statuses, <-statuses
	if !((first == 201 && second == 409) || (first == 409 && second == 201)) {
		t.Fatalf("concurrent statuses=%d,%d", first, second)
	}
	// Deleting one's update permits a replacement without leaving a duplicate.
	own := listMondayUpdates(t, fixture, "student", "").Items[0]
	response = testRequest(t, fixture.handler, http.MethodDelete, mondayUpdatesPath()+"/"+own.ID, "student", "")
	if response.Code != 204 {
		t.Fatalf("delete=%d %s", response.Code, response.Body)
	}
	createMondayUpdateFor(t, fixture, "student", "<p>Replacement</p>")
	// Closing the season also closes late submissions.
	if _, err := fixture.pool.Exec(context.Background(), `UPDATE app.seasons SET status='closed',closed_at=now() WHERE id=$1`, seasonID); err != nil {
		t.Fatal(err)
	}
	response = testRequest(t, fixture.handler, http.MethodPost, mondayUpdatesPath(), "cohort", body(mondayTestDate(-6), mondayTestDate(0)))
	if response.Code != http.StatusConflict {
		t.Fatalf("closed=%d %s", response.Code, response.Body)
	}
}

// pinMondayNow sets the late lock's clock to noon, Adelaide, on a date.
func pinMondayNow(t *testing.T, date string) {
	t.Helper()
	adelaide, err := time.LoadLocation("Australia/Adelaide")
	if err != nil {
		t.Fatal(err)
	}
	day, err := time.ParseInLocation(time.DateOnly, date, adelaide)
	if err != nil {
		t.Fatal(err)
	}
	previous := mondayNow
	mondayNow = func() time.Time { return day.Add(12 * time.Hour) }
	t.Cleanup(func() { mondayNow = previous })
}

func TestMondayUpdateLateUnlock(t *testing.T) {
	fixture := newMondayUpdateFixture(t)
	unlocksPath := "/api/v2/seasons/" + seasonID + "/monday-update-unlocks"
	week := `"startDate":"` + mondayTestDate(-6) + `","endDate":"` + mondayTestDate(0) + `"`
	// A day past the due date, the week is locked.
	pinMondayNow(t, mondayTestDate(2))
	response := testRequest(t, fixture.handler, http.MethodPost, mondayUpdatesPath(), "cohort", `{`+week+`,"contentHtml":"","hours":10}`)
	if response.Code != http.StatusConflict {
		t.Fatalf("late post status=%d body=%s", response.Code, response.Body)
	}

	// The student asks with a reason; only their mentor or a coordinator may
	// decide, and the request is listed for the mentor.
	if response = testRequest(t, fixture.handler, http.MethodPost, unlocksPath, "cohort", `{`+week+`,"reason":"  "}`); response.Code != http.StatusBadRequest {
		t.Fatalf("blank reason status=%d body=%s", response.Code, response.Body)
	}
	response = testRequest(t, fixture.handler, http.MethodPost, unlocksPath, "cohort", `{`+week+`,"reason":"I was sick."}`)
	if response.Code != http.StatusCreated {
		t.Fatalf("request status=%d body=%s", response.Code, response.Body)
	}
	var requested program.MondayUpdateUnlock
	if err := json.Unmarshal(response.Body.Bytes(), &requested); err != nil || requested.State != "pending" || requested.Reason != "I was sick." {
		t.Fatalf("requested=%+v err=%v", requested, err)
	}
	if response = testRequest(t, fixture.handler, http.MethodPost, unlocksPath, "cohort", `{`+week+`,"reason":"Again"}`); response.Code != http.StatusConflict {
		t.Fatalf("second request status=%d body=%s", response.Code, response.Body)
	}
	response = testRequest(t, fixture.handler, http.MethodGet, unlocksPath, "mentor", "")
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), requested.ID) {
		t.Fatalf("mentor list status=%d body=%s", response.Code, response.Body)
	}
	decisionPath := unlocksPath + "/" + requested.ID + "/decision"
	if response = testRequest(t, fixture.handler, http.MethodPost, decisionPath, "student", `{"decision":"approved"}`); response.Code != http.StatusForbidden {
		t.Fatalf("stranger decision status=%d body=%s", response.Code, response.Body)
	}
	if response = testRequest(t, fixture.handler, http.MethodPost, decisionPath, "mentor", `{"decision":"approved"}`); response.Code != http.StatusOK {
		t.Fatalf("mentor decision status=%d body=%s", response.Code, response.Body)
	}

	// Unlocked, the student can post the late week and edit it.
	response = testRequest(t, fixture.handler, http.MethodPost, mondayUpdatesPath(), "cohort", `{`+week+`,"contentHtml":"","hours":10}`)
	if response.Code != http.StatusCreated {
		t.Fatalf("unlocked post status=%d body=%s", response.Code, response.Body)
	}

	// A mentor can also unlock a week directly, without a request.
	if _, err := fixture.pool.Exec(context.Background(), `UPDATE app.monday_update_unlocks SET state='declined'`); err != nil {
		t.Fatal(err)
	}
	if response = testRequest(t, fixture.handler, http.MethodPost, unlocksPath, "mentor", `{`+week+`,"studentId":"`+mondayCohortID+`"}`); response.Code != http.StatusCreated {
		t.Fatalf("direct unlock status=%d body=%s", response.Code, response.Body)
	}
	if response = testRequest(t, fixture.handler, http.MethodPost, unlocksPath, "mentor", `{`+week+`,"studentId":"`+studentID+`"}`); response.Code != http.StatusForbidden {
		t.Fatalf("unassigned unlock status=%d body=%s", response.Code, response.Body)
	}
}

func TestMondayMentorReport(t *testing.T) {
	fixture := newMondayUpdateFixture(t)
	reportsPath := "/api/v2/seasons/" + seasonID + "/monday-mentor-reports"
	week := `"startDate":"` + mondayTestDate(-6) + `","endDate":"` + mondayTestDate(0) + `"`
	query := reportsPath + "?startDate=" + mondayTestDate(-6) + "&endDate=" + mondayTestDate(0)
	read := func(actor string) []program.MondayMentorReport {
		t.Helper()
		response := testRequest(t, fixture.handler, http.MethodGet, query, actor, "")
		if response.Code != http.StatusOK {
			t.Fatalf("%s reports status=%d body=%s", actor, response.Code, response.Body)
		}
		var page struct{ Items []program.MondayMentorReport }
		if err := json.Unmarshal(response.Body.Bytes(), &page); err != nil {
			t.Fatal(err)
		}
		return page.Items
	}

	// The mentor's week lists their mentee, unmarked.
	reports := read("mentor")
	// With no targets of their own, a student is held to the program's.
	if targets := reports[0].Students[0].Targets; targets != (program.MondayWeeklyTargets{Hours: 30}) {
		t.Fatalf("targets=%+v", targets)
	}
	if len(reports) != 1 || len(reports[0].Students) != 1 || reports[0].Students[0].Student.ID != mondayCohortID || reports[0].Students[0].Mark != nil {
		t.Fatalf("mentor reports=%+v", reports)
	}

	// The mentee posted nothing; the mentor still marks the week.
	markPath := "/api/v2/seasons/" + seasonID + "/monday-week-marks"
	if response := testRequest(t, fixture.handler, http.MethodPut, markPath, "mentor", `{`+week+`,"studentId":"`+studentID+`","mark":"at_risk"}`); response.Code != http.StatusForbidden {
		t.Fatalf("unassigned mark status=%d body=%s", response.Code, response.Body)
	}
	if response := testRequest(t, fixture.handler, http.MethodPut, markPath, "mentor", `{`+week+`,"studentId":"`+mondayCohortID+`","mark":"at_risk","commentHtml":"<p>No update again.</p>"}`); response.Code != http.StatusOK {
		t.Fatalf("week mark status=%d body=%s", response.Code, response.Body)
	}

	// The coordinator sees the week with the mentor's mark and comment.
	reports = read("coordinator")
	if len(reports) != 1 {
		t.Fatalf("coordinator reports=%+v", reports)
	}
	student := reports[0].Students[0]
	if student.Mark == nil || *student.Mark != "at_risk" || student.CommentHTML != "<p>No update again.</p>" || student.UpdateID != nil {
		t.Fatalf("report student=%+v", student)
	}
	if response := testRequest(t, fixture.handler, http.MethodGet, query, "student", ""); response.Code != http.StatusForbidden {
		t.Fatalf("student reports status=%d body=%s", response.Code, response.Body)
	}
}

func TestMondayKickedStudentWeek(t *testing.T) {
	fixture := newMondayUpdateFixture(t)
	reportQuery := func(start, end string) string {
		return "/api/v2/seasons/" + seasonID + "/monday-mentor-reports?startDate=" + start + "&endDate=" + end
	}
	read := func(actor, start, end string) []program.MondayMentorReport {
		t.Helper()
		response := testRequest(t, fixture.handler, http.MethodGet, reportQuery(start, end), actor, "")
		if response.Code != http.StatusOK {
			t.Fatalf("%s reports status=%d body=%s", actor, response.Code, response.Body)
		}
		var page struct{ Items []program.MondayMentorReport }
		if err := json.Unmarshal(response.Body.Bytes(), &page); err != nil {
			t.Fatal(err)
		}
		return page.Items
	}
	// The mentee is kicked midway through the week.
	adelaide, _ := time.LoadLocation("Australia/Adelaide")
	kickDay, _ := time.ParseInLocation(time.DateOnly, mondayTestDate(-3), adelaide)
	if _, err := fixture.db.RemoveEnrollment(context.Background(), dal.RemoveEnrollmentInput{
		EnrollmentID: mondayCohortMemberID, Reason: "Stopped attending.", ActorID: mondayCoordinatorID, ChangedAt: kickDay.Add(12 * time.Hour),
	}); err != nil {
		t.Fatal(err)
	}

	// Their mentor still sees them that week, flagged, and marks it "kicked".
	reports := read("mentor", mondayTestDate(-6), mondayTestDate(0))
	if len(reports) != 1 || len(reports[0].Students) != 1 || reports[0].Students[0].KickedAt == nil {
		t.Fatalf("kicked week reports=%+v", reports)
	}
	if response := testRequest(t, fixture.handler, http.MethodPut, "/api/v2/seasons/"+seasonID+"/monday-week-marks", "mentor",
		`{"startDate":"`+mondayTestDate(-6)+`","endDate":"`+mondayTestDate(0)+`","studentId":"`+mondayCohortID+`","mark":"kicked","commentHtml":"<p>Stopped attending.</p>"}`); response.Code != http.StatusOK {
		t.Fatalf("kicked mark status=%d body=%s", response.Code, response.Body)
	}
	reports = read("coordinator", mondayTestDate(-6), mondayTestDate(0))
	if mark := reports[0].Students[0].Mark; mark == nil || *mark != "kicked" {
		t.Fatalf("coordinator reports=%+v", reports)
	}

	// An earlier week also reads as kicked: they will not post it now.
	before := read("mentor", mondayTestDate(-13), mondayTestDate(-7))
	if len(before) != 1 || before[0].Students[0].KickedAt == nil {
		t.Fatalf("week before reports=%+v", before)
	}
}

func TestMondayKickedStudentKeepsPostedWeek(t *testing.T) {
	fixture := newMondayUpdateFixture(t)
	// The mentee posts the week, then is kicked after it.
	createMondayUpdateRange(t, fixture, "cohort", mondayTestDate(-6), mondayTestDate(0), "<p>Done</p>")
	if _, err := fixture.db.RemoveEnrollment(context.Background(), dal.RemoveEnrollmentInput{
		EnrollmentID: mondayCohortMemberID, Reason: "Stopped attending.", ActorID: mondayCoordinatorID, ChangedAt: time.Now(),
	}); err != nil {
		t.Fatal(err)
	}

	// Their mentor and the coordinator still see the posted update's figures.
	for _, actor := range []string{"mentor", "coordinator"} {
		response := testRequest(t, fixture.handler, http.MethodGet,
			"/api/v2/seasons/"+seasonID+"/monday-mentor-reports?startDate="+mondayTestDate(-6)+"&endDate="+mondayTestDate(0), actor, "")
		var page struct{ Items []program.MondayMentorReport }
		if err := json.Unmarshal(response.Body.Bytes(), &page); err != nil || response.Code != http.StatusOK {
			t.Fatalf("%s status=%d err=%v", actor, response.Code, err)
		}
		if len(page.Items) != 1 || len(page.Items[0].Students) != 1 {
			t.Fatalf("%s reports=%+v", actor, page.Items)
		}
		student := page.Items[0].Students[0]
		if student.UpdateID == nil || student.Hours == nil || *student.Hours != 12.5 || student.KickedAt == nil {
			t.Fatalf("%s kicked student=%+v", actor, student)
		}
	}
	if updates := listMondayUpdates(t, fixture, "mentor", ""); len(updates.Items) != 1 {
		t.Fatalf("mentor updates=%+v", updates.Items)
	}
}

func TestMondayMarkCarriesOverAndClears(t *testing.T) {
	fixture := newMondayUpdateFixture(t)
	week := `"startDate":"` + mondayTestDate(-6) + `","endDate":"` + mondayTestDate(0) + `"`
	markPath := "/api/v2/seasons/" + seasonID + "/monday-week-marks"
	reportQuery := "/api/v2/seasons/" + seasonID + "/monday-mentor-reports?startDate=" + mondayTestDate(-6) + "&endDate=" + mondayTestDate(0)
	markOf := func() *string {
		t.Helper()
		response := testRequest(t, fixture.handler, http.MethodGet, reportQuery, "mentor", "")
		var page struct{ Items []program.MondayMentorReport }
		if err := json.Unmarshal(response.Body.Bytes(), &page); err != nil || len(page.Items) != 1 {
			t.Fatalf("reports status=%d body=%s", response.Code, response.Body)
		}
		return page.Items[0].Students[0].Mark
	}
	// The mentor marks the week before the student posts; posting keeps it.
	if response := testRequest(t, fixture.handler, http.MethodPut, markPath, "mentor", `{`+week+`,"studentId":"`+mondayCohortID+`","mark":"at_risk","commentHtml":"<p>Late again.</p>"}`); response.Code != http.StatusOK {
		t.Fatalf("week mark status=%d body=%s", response.Code, response.Body)
	}
	created := createMondayUpdateRange(t, fixture, "cohort", mondayTestDate(-6), mondayTestDate(0), "<p>Done</p>")
	if mark := markOf(); mark == nil || *mark != "at_risk" {
		t.Fatalf("mark after posting=%v", mark)
	}
	// Editing the update keeps the mark too.
	if response := testRequest(t, fixture.handler, http.MethodPatch, mondayUpdatesPath()+"/"+created.ID, "cohort", `{"contentHtml":"<p>More</p>","hours":20}`); response.Code != http.StatusOK {
		t.Fatalf("edit status=%d body=%s", response.Code, response.Body)
	}
	if mark := markOf(); mark == nil || *mark != "at_risk" {
		t.Fatalf("mark after editing=%v", mark)
	}
	// "Not marked" clears it.
	if response := testRequest(t, fixture.handler, http.MethodPut, mondayUpdatesPath()+"/"+created.ID+"/mark", "mentor", `{"mark":""}`); response.Code != http.StatusNoContent {
		t.Fatalf("clear status=%d body=%s", response.Code, response.Body)
	}
	if mark := markOf(); mark != nil {
		t.Fatalf("mark after clearing=%v", *mark)
	}
}
