//go:build integration

package api

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/Ravis-Study-Program/RSP-website/backend/internal/authz"
	"github.com/Ravis-Study-Program/RSP-website/backend/internal/platform/id"
	"github.com/Ravis-Study-Program/RSP-website/backend/internal/practice"
	"github.com/Ravis-Study-Program/RSP-website/backend/internal/program"
)

// assignMentor makes the fixture's mentor (other) the student's mentor.
func assignMentor(t *testing.T, fixture postgresFixture) {
	t.Helper()
	if _, err := fixture.db.CreateMentorship(context.Background(), program.MentorshipRecord{
		ID: id.New(), SeasonID: seasonID, MentorUserID: otherID, StudentUserID: studentID,
	}, otherID, time.Now()); err != nil {
		t.Fatal(err)
	}
}

func TestOnlyTheCurrentMentorSetsStudentLevel(t *testing.T) {
	fixture := newPostgresFixture(t)
	ctx := context.Background()
	path := "/api/v2/seasons/" + seasonID + "/members/" + studentMemberID + "/student-level"
	// A season mentor who is not the student's mentor cannot.
	if response := testRequest(t, fixture.handler, http.MethodPatch, path, "other", `{"studentLevel":"advanced"}`); response.Code != http.StatusForbidden {
		t.Fatalf("unassigned mentor status=%d body=%s", response.Code, response.Body)
	}
	// Their current mentor can set all four levels, and the mentorship
	// survives each change.
	assignMentor(t, fixture)
	for _, level := range []string{"novice", "beginner", "intermediate", "advanced"} {
		response := testRequest(t, fixture.handler, http.MethodPatch, path, "other", `{"studentLevel":"`+level+`"}`)
		if response.Code != http.StatusOK {
			t.Fatalf("level=%s status=%d body=%s", level, response.Code, response.Body)
		}
		var member program.EnrollmentRecord
		if err := json.Unmarshal(response.Body.Bytes(), &member); err != nil {
			t.Fatal(err)
		}
		if member.Role != "student" || member.State != "active" || member.StudentLevel != level {
			t.Fatalf("member=%+v", member)
		}
	}
	response := testRequest(t, fixture.handler, http.MethodPatch, path, "other", `{"studentLevel":"intermediate"}`)
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body)
	}
	var links, events int
	if err := fixture.pool.QueryRow(ctx, `SELECT count(*) FROM app.mentorships WHERE ended_at IS NULL`).Scan(&links); err != nil {
		t.Fatal(err)
	}
	if err := fixture.pool.QueryRow(ctx, `SELECT count(*) FROM app.audit_events WHERE action='enrollment.student_level_updated' AND subject_id=$1`, studentMemberID).Scan(&events); err != nil {
		t.Fatal(err)
	}
	if links != 1 || events != 5 {
		t.Fatalf("active mentorships=%d audit events=%d", links, events)
	}
	stored, err := fixture.db.GetEnrollment(ctx, studentMemberID)
	if err != nil || stored.StudentLevel != "intermediate" || stored.Role != "student" {
		t.Fatalf("stored=%+v err=%v", stored, err)
	}
	// Once the mentorship ends, their past mentor cannot.
	if _, err := fixture.pool.Exec(ctx, `UPDATE app.mentorships SET ended_at=now()`); err != nil {
		t.Fatal(err)
	}
	if response := testRequest(t, fixture.handler, http.MethodPatch, path, "other", `{"studentLevel":"advanced"}`); response.Code != http.StatusForbidden {
		t.Fatalf("past mentor status=%d body=%s", response.Code, response.Body)
	}
}

func TestStudentLevelRestrictions(t *testing.T) {
	for _, test := range []struct {
		name, token, memberID, body, setup string
		status                             int
	}{
		{"student cannot promote", "student", studentMemberID, `{"studentLevel":"advanced"}`, "", http.StatusForbidden},
		{"invalid level", "other", studentMemberID, `{"studentLevel":"expert"}`, "", http.StatusBadRequest},
		{"role cannot be injected", "other", studentMemberID, `{"studentLevel":"advanced","role":"mentor"}`, "", http.StatusBadRequest},
		{"not their student", "other", otherMemberID, `{"studentLevel":"advanced"}`, "", http.StatusForbidden},
		{"inactive student", "other", studentMemberID, `{"studentLevel":"advanced"}`, `UPDATE app.enrollments SET state='withdrawn',assignment_state='revoked',activated_at=NULL WHERE id='` + studentMemberID + `'`, http.StatusConflict},
		{"closed season", "other", studentMemberID, `{"studentLevel":"advanced"}`, `UPDATE app.seasons SET status='closed',closed_at=now() WHERE id='` + seasonID + `'`, http.StatusConflict},
		{"member in another season", "other", studentMemberID, `{"studentLevel":"advanced"}`, `INSERT INTO app.seasons(id,slug,name,status,start_at,end_at,location) SELECT '00000000-0000-7000-8000-000000000999','different','Different',status,start_at,end_at,location FROM app.seasons; UPDATE app.enrollments SET season_id='00000000-0000-7000-8000-000000000999' WHERE id='` + studentMemberID + `'`, http.StatusNotFound},
	} {
		t.Run(test.name, func(t *testing.T) {
			fixture := newPostgresFixture(t)
			assignMentor(t, fixture)
			if test.setup != "" {
				if _, err := fixture.pool.Exec(context.Background(), test.setup); err != nil {
					t.Fatal(err)
				}
			}
			response := testRequest(t, fixture.handler, http.MethodPatch, "/api/v2/seasons/"+seasonID+"/members/"+test.memberID+"/student-level", test.token, test.body)
			if response.Code != test.status {
				t.Fatalf("status=%d want=%d body=%s", response.Code, test.status, response.Body)
			}
		})
	}
	for _, state := range []authz.Enrollment{{SeasonID: "different", Role: authz.Mentor, State: authz.Active}, {SeasonID: seasonID, Role: authz.Mentor, State: authz.Completed}} {
		fixture := newPostgresFixture(t)
		handler := New(Config{DB: fixture.db, Authenticator: AuthenticatorFunc(func(context.Context, string) (authz.Actor, error) {
			return authz.Actor{UserID: otherID, EmailVerified: true, AccountState: authz.AccountActive, Enrollments: []authz.Enrollment{state}}, nil
		})}).Handler()
		response := testRequest(t, handler, http.MethodPatch, "/api/v2/seasons/"+seasonID+"/members/"+studentMemberID+"/student-level", "mentor", `{"studentLevel":"advanced"}`)
		if response.Code != http.StatusForbidden {
			t.Fatalf("membership=%+v status=%d", state, response.Code)
		}
	}
}

func TestPracticeGoalsApplyToEveryone(t *testing.T) {
	fixture := newPostgresFixture(t)
	for _, legacy := range []bool{false, true} {
		if legacy {
			if _, err := fixture.pool.Exec(context.Background(), `INSERT INTO app.practice_goals(user_id,enabled,easy_minutes,medium_minutes,hard_minutes) VALUES($1,false,10,20,45)`, studentID); err != nil {
				t.Fatal(err)
			}
		}
		response := testRequest(t, fixture.handler, http.MethodGet, "/api/v2/me/practice-settings", "student", "")
		var settings practice.PracticeSettings
		if err := json.Unmarshal(response.Body.Bytes(), &settings); err != nil {
			t.Fatal(err)
		}
		if response.Code != http.StatusOK || settings != practice.DefaultSettings() {
			t.Fatalf("status=%d settings=%+v", response.Code, settings)
		}
	}
	response := testRequest(t, fixture.handler, http.MethodPatch, "/api/v2/me/practice-settings", "student", `{"easyMinutes":25,"mediumMinutes":40,"hardMinutes":60}`)
	if response.Code != http.StatusMethodNotAllowed {
		t.Fatalf("custom goals status=%d body=%s", response.Code, response.Body)
	}
	// Only coordinators and administrators change the program's goals.
	for _, actor := range []string{"student", "other"} {
		response = testRequest(t, fixture.handler, http.MethodPut, "/api/v2/practice-settings", actor, `{"easyMinutes":15,"mediumMinutes":30,"hardMinutes":45,"weeklyHours":30}`)
		if response.Code != http.StatusForbidden {
			t.Fatalf("%s goals status=%d body=%s", actor, response.Code, response.Body)
		}
	}
}

func TestMentorSetsStudentRequirements(t *testing.T) {
	fixture := newPostgresFixture(t)
	path := "/api/v2/seasons/" + seasonID + "/members/" + studentMemberID + "/requirements"
	// The student cannot set their own; out-of-range values are refused.
	if response := testRequest(t, fixture.handler, http.MethodPut, path, "student", `{"easyMinutes":15}`); response.Code != http.StatusForbidden {
		t.Fatalf("student status=%d body=%s", response.Code, response.Body)
	}
	// Only the student's current mentor sets them, not any season mentor.
	if response := testRequest(t, fixture.handler, http.MethodGet, path, "other", ""); response.Code != http.StatusForbidden {
		t.Fatalf("unassigned get status=%d body=%s", response.Code, response.Body)
	}
	if response := testRequest(t, fixture.handler, http.MethodPut, path, "other", `{"easyMinutes":15}`); response.Code != http.StatusForbidden {
		t.Fatalf("unassigned put status=%d body=%s", response.Code, response.Body)
	}
	assignMentor(t, fixture)
	if response := testRequest(t, fixture.handler, http.MethodPut, path, "other", `{"weeklyHours":200}`); response.Code != http.StatusBadRequest {
		t.Fatalf("invalid status=%d body=%s", response.Code, response.Body)
	}
	// Their mentor sets an easy goal and weekly hours; the rest follow the
	// program's defaults.
	response := testRequest(t, fixture.handler, http.MethodPut, path, "other", `{"easyMinutes":15,"weeklyHours":20}`)
	if response.Code != http.StatusOK {
		t.Fatalf("put status=%d body=%s", response.Code, response.Body)
	}
	response = testRequest(t, fixture.handler, http.MethodGet, path, "other", "")
	var own practice.StudentRequirements
	if err := json.Unmarshal(response.Body.Bytes(), &own); err != nil || own.EasyMinutes == nil || *own.EasyMinutes != 15 || own.MediumMinutes != nil || own.WeeklyHours == nil || *own.WeeklyHours != 20 {
		t.Fatalf("requirements=%+v err=%v body=%s", own, err, response.Body)
	}
	// The student's settings now carry them.
	response = testRequest(t, fixture.handler, http.MethodGet, "/api/v2/me/practice-settings", "student", "")
	var settings practice.PracticeSettings
	if err := json.Unmarshal(response.Body.Bytes(), &settings); err != nil {
		t.Fatal(err)
	}
	if settings.EasyMinutes != 15 || settings.MediumMinutes != 25 || settings.WeeklyHours != 20 {
		t.Fatalf("student settings=%+v", settings)
	}
}

func TestMentorChangesTheirStudentsNameAndUsername(t *testing.T) {
	fixture := newPostgresFixture(t)
	ctx := context.Background()
	path := "/api/v2/seasons/" + seasonID + "/members/" + studentMemberID + "/profile"
	stored := func(userID string) (name, slug string) {
		t.Helper()
		if err := fixture.pool.QueryRow(ctx, `SELECT display_name,slug FROM app.user_profiles WHERE user_id=$1`, userID).Scan(&name, &slug); err != nil {
			t.Fatal(err)
		}
		return name, slug
	}
	_, studentSlug := stored(studentID)
	_, mentorSlug := stored(otherID)

	// The student can change neither themselves, here or from their profile.
	if response := testRequest(t, fixture.handler, http.MethodPatch, path, "student", `{"name":"Self Named","slug":"self-named"}`); response.Code != http.StatusForbidden {
		t.Fatalf("student status=%d body=%s", response.Code, response.Body)
	}
	if response := testRequest(t, fixture.handler, http.MethodPatch, "/api/v2/me", "student", `{"name":"Student","slug":"self-named","timezone":"Australia/Adelaide"}`); response.Code != http.StatusForbidden {
		t.Fatalf("own username status=%d body=%s", response.Code, response.Body)
	}
	// Only their current mentor, not any mentor in the season.
	if response := testRequest(t, fixture.handler, http.MethodPatch, path, "other", `{"name":"Renamed Student","slug":"renamed-student"}`); response.Code != http.StatusForbidden {
		t.Fatalf("unassigned status=%d body=%s", response.Code, response.Body)
	}
	assignMentor(t, fixture)
	for body, want := range map[string]int{
		`{"name":"","slug":"renamed-student"}`:                   http.StatusBadRequest,
		`{"name":"Renamed Student","slug":"Not A Username"}`:     http.StatusBadRequest,
		`{"name":"Renamed Student","slug":"` + mentorSlug + `"}`: http.StatusConflict,
	} {
		if response := testRequest(t, fixture.handler, http.MethodPatch, path, "other", body); response.Code != want {
			t.Fatalf("%s status=%d want %d body=%s", body, response.Code, want, response.Body)
		}
	}
	if name, slug := stored(studentID); name != "Student" || slug != studentSlug {
		t.Fatalf("refused changes saved: %q %q", name, slug)
	}

	response := testRequest(t, fixture.handler, http.MethodPatch, path, "other", `{"name":" Renamed Student ","slug":"Renamed-Student"}`)
	if response.Code != http.StatusOK {
		t.Fatalf("mentor status=%d body=%s", response.Code, response.Body)
	}
	var saved struct{ Name, Slug string }
	if err := json.Unmarshal(response.Body.Bytes(), &saved); err != nil || saved.Name != "Renamed Student" || saved.Slug != "renamed-student" {
		t.Fatalf("saved=%+v err=%v", saved, err)
	}
	if name, slug := stored(studentID); name != "Renamed Student" || slug != "renamed-student" {
		t.Fatalf("stored %q %q", name, slug)
	}
	var audits int
	if err := fixture.pool.QueryRow(ctx, `SELECT count(*) FROM app.audit_events WHERE action='user.profile_corrected' AND subject_id=$1 AND actor_user_id=$2`, studentID, otherID).Scan(&audits); err != nil || audits != 1 {
		t.Fatalf("audits=%d err=%v", audits, err)
	}
	// A mentor cannot use it on themselves.
	if response := testRequest(t, fixture.handler, http.MethodPatch, "/api/v2/seasons/"+seasonID+"/members/"+otherMemberID+"/profile", "other", `{"name":"Other","slug":"other-renamed"}`); response.Code != http.StatusForbidden {
		t.Fatalf("mentor enrollment status=%d body=%s", response.Code, response.Body)
	}
}
