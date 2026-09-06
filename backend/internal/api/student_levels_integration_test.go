//go:build integration

package api

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/magedmg/RSP-website/backend/internal/authz"
	"github.com/magedmg/RSP-website/backend/internal/platform/id"
	"github.com/magedmg/RSP-website/backend/internal/practice"
	"github.com/magedmg/RSP-website/backend/internal/programme"
)

func TestAnySeasonMentorCanSetStudentLevel(t *testing.T) {
	fixture := newPostgresFixture(t)
	ctx := context.Background()
	path := "/api/v2/seasons/" + seasonID + "/members/" + studentMemberID + "/student-level"
	// No mentorship exists: the season mentor can still set all four levels.
	for _, level := range []string{"novice", "beginner", "intermediate", "advanced"} {
		response := testRequest(t, fixture.handler, http.MethodPatch, path, "other", `{"studentLevel":"`+level+`"}`)
		if response.Code != http.StatusOK {
			t.Fatalf("level=%s status=%d body=%s", level, response.Code, response.Body)
		}
		var member programme.EnrollmentRecord
		if err := json.Unmarshal(response.Body.Bytes(), &member); err != nil {
			t.Fatal(err)
		}
		if member.Role != "student" || member.State != "active" || member.StudentLevel != level {
			t.Fatalf("member=%+v", member)
		}
	}
	// An existing mentor assignment must survive a level change too.
	if _, err := fixture.db.CreateMentorship(ctx, programme.MentorshipRecord{
		ID: id.New(), SeasonID: seasonID, MentorUserID: otherID, StudentUserID: studentID,
	}, otherID, time.Now()); err != nil {
		t.Fatal(err)
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
}

func TestStudentLevelRestrictions(t *testing.T) {
	for _, test := range []struct {
		name, token, memberID, body, setup string
		status                             int
	}{
		{"student cannot promote", "student", studentMemberID, `{"studentLevel":"advanced"}`, "", http.StatusForbidden},
		{"invalid level", "other", studentMemberID, `{"studentLevel":"expert"}`, "", http.StatusBadRequest},
		{"role cannot be injected", "other", studentMemberID, `{"studentLevel":"advanced","role":"mentor"}`, "", http.StatusBadRequest},
		{"mentor target", "other", otherMemberID, `{"studentLevel":"advanced"}`, "", http.StatusConflict},
		{"inactive student", "other", studentMemberID, `{"studentLevel":"advanced"}`, `UPDATE app.enrollments SET state='withdrawn',assignment_state='revoked',activated_at=NULL WHERE id='` + studentMemberID + `'`, http.StatusConflict},
		{"closed season", "other", studentMemberID, `{"studentLevel":"advanced"}`, `UPDATE app.seasons SET status='closed',closed_at=now() WHERE id='` + seasonID + `'`, http.StatusConflict},
		{"member in another season", "other", studentMemberID, `{"studentLevel":"advanced"}`, `INSERT INTO app.seasons(id,slug,name,status,start_at,end_at,location) SELECT '00000000-0000-7000-8000-000000000999','different','Different',status,start_at,end_at,location FROM app.seasons; UPDATE app.enrollments SET season_id='00000000-0000-7000-8000-000000000999' WHERE id='` + studentMemberID + `'`, http.StatusNotFound},
	} {
		t.Run(test.name, func(t *testing.T) {
			fixture := newPostgresFixture(t)
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

func TestPracticeGoalsAreFixed(t *testing.T) {
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
		if response.Code != http.StatusOK || settings != practice.FixedSettings() {
			t.Fatalf("status=%d settings=%+v", response.Code, settings)
		}
	}
	response := testRequest(t, fixture.handler, http.MethodPatch, "/api/v2/me/practice-settings", "student", `{"easyMinutes":25,"mediumMinutes":40,"hardMinutes":60}`)
	if response.Code != http.StatusMethodNotAllowed {
		t.Fatalf("custom goals status=%d body=%s", response.Code, response.Body)
	}
}
