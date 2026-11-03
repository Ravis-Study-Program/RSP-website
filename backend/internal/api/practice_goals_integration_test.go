//go:build integration

package api

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/magedmg/RSP-website/backend/internal/authz"
	"github.com/magedmg/RSP-website/backend/internal/practice"
)

func decodePracticeSettings(t *testing.T, body []byte) practice.PracticeSettings {
	t.Helper()
	var settings practice.PracticeSettings
	if err := json.Unmarshal(body, &settings); err != nil {
		t.Fatal(err)
	}
	return settings
}

func TestDirectorsSetProgrammePracticeGoals(t *testing.T) {
	fixture := newPostgresFixture(t)
	response := testRequest(t, fixture.handler, http.MethodGet, "/api/v2/practice-goals", "student", "")
	var goals practice.Goals
	if err := json.Unmarshal(response.Body.Bytes(), &goals); err != nil || response.Code != http.StatusOK || goals != practice.DefaultGoals() {
		t.Fatalf("default goals status=%d body=%s", response.Code, response.Body)
	}

	edited := `{"easyMinutes":12,"mediumMinutes":28,"hardMinutes":45}`
	for _, token := range []string{"student", "other"} {
		if response := testRequest(t, fixture.handler, http.MethodPut, "/api/v2/practice-goals", token, edited); response.Code != http.StatusForbidden {
			t.Fatalf("%s programme goals status=%d body=%s", token, response.Code, response.Body)
		}
	}

	director := New(Config{
		DB: fixture.db,
		Authenticator: AuthenticatorFunc(func(context.Context, string) (authz.Actor, error) {
			return authz.Actor{UserID: otherID, EmailVerified: true, AccountState: authz.AccountActive, GlobalRoles: map[authz.GlobalRole]bool{authz.Director: true}}, nil
		}),
		CursorSecret: []byte("0123456789abcdef"),
	}).Handler()
	for _, invalid := range []string{
		`{"easyMinutes":0,"mediumMinutes":28,"hardMinutes":45}`,
		`{"easyMinutes":12,"mediumMinutes":28,"hardMinutes":181}`,
		`{"easyMinutes":12,"mediumMinutes":28}`,
		`{"easyMinutes":12,"mediumMinutes":28,"hardMinutes":45,"goalsEnabled":false}`,
	} {
		if response := testRequest(t, director, http.MethodPut, "/api/v2/practice-goals", "director", invalid); response.Code != http.StatusBadRequest {
			t.Fatalf("invalid goals %s status=%d body=%s", invalid, response.Code, response.Body)
		}
	}
	if response := testRequest(t, director, http.MethodPut, "/api/v2/practice-goals", "director", edited); response.Code != http.StatusOK {
		t.Fatalf("director programme goals status=%d body=%s", response.Code, response.Body)
	}

	response = testRequest(t, fixture.handler, http.MethodGet, "/api/v2/me/practice-settings", "student", "")
	want := practice.PracticeSettings{GoalsEnabled: true, Goals: practice.Goals{EasyMinutes: 12, MediumMinutes: 28, HardMinutes: 45}, Source: practice.GoalSourceProgramme}
	if settings := decodePracticeSettings(t, response.Body.Bytes()); response.Code != http.StatusOK || settings != want {
		t.Fatalf("student settings status=%d settings=%+v", response.Code, settings)
	}
}

func TestAssignedMentorsSetStudentPracticeGoals(t *testing.T) {
	fixture := newPostgresFixture(t)
	ctx := context.Background()
	path := "/api/v2/seasons/" + seasonID + "/members/" + studentMemberID + "/practice-goals"
	override := `{"easyMinutes":15,"mediumMinutes":30,"hardMinutes":50}`
	programmeSettings := practice.PracticeSettings{GoalsEnabled: true, Goals: practice.DefaultGoals(), Source: practice.GoalSourceProgramme}

	// Legacy personal goals stay historical data and never apply.
	if _, err := fixture.pool.Exec(ctx, `INSERT INTO app.practice_goals(user_id,enabled,easy_minutes,medium_minutes,hard_minutes) VALUES($1,false,10,20,45)`, studentID); err != nil {
		t.Fatal(err)
	}
	response := testRequest(t, fixture.handler, http.MethodGet, "/api/v2/me/practice-settings", "student", "")
	if settings := decodePracticeSettings(t, response.Body.Bytes()); response.Code != http.StatusOK || settings != programmeSettings {
		t.Fatalf("legacy settings status=%d settings=%+v", response.Code, settings)
	}
	if response := testRequest(t, fixture.handler, http.MethodPatch, "/api/v2/me/practice-settings", "student", override); response.Code != http.StatusMethodNotAllowed {
		t.Fatalf("personal goals status=%d body=%s", response.Code, response.Body)
	}

	if response := testRequest(t, fixture.handler, http.MethodPut, path, "other", override); response.Code != http.StatusForbidden {
		t.Fatalf("unassigned mentor status=%d body=%s", response.Code, response.Body)
	}
	if response := testRequest(t, fixture.handler, http.MethodPut, path, "student", override); response.Code != http.StatusForbidden {
		t.Fatalf("student self-service status=%d body=%s", response.Code, response.Body)
	}
	if _, err := fixture.pool.Exec(ctx, `INSERT INTO app.mentorships(id,season_id,mentor_enrollment_id,student_enrollment_id)
		VALUES ('00000000-0000-7000-8000-000000000110',$1,$2,$3)`, seasonID, otherMemberID, studentMemberID); err != nil {
		t.Fatal(err)
	}
	mentorPath := "/api/v2/seasons/" + seasonID + "/members/" + otherMemberID + "/practice-goals"
	if response := testRequest(t, fixture.handler, http.MethodPut, mentorPath, "other", override); response.Code != http.StatusNotFound {
		t.Fatalf("mentor enrollment status=%d body=%s", response.Code, response.Body)
	}
	if response := testRequest(t, fixture.handler, http.MethodPut, path, "other", `{"easyMinutes":15,"mediumMinutes":30,"hardMinutes":0}`); response.Code != http.StatusBadRequest {
		t.Fatalf("invalid override status=%d body=%s", response.Code, response.Body)
	}

	response = testRequest(t, fixture.handler, http.MethodPut, path, "other", override)
	want := practice.PracticeSettings{GoalsEnabled: true, Goals: practice.Goals{EasyMinutes: 15, MediumMinutes: 30, HardMinutes: 50}, Source: practice.GoalSourceStudent}
	if settings := decodePracticeSettings(t, response.Body.Bytes()); response.Code != http.StatusOK || settings != want {
		t.Fatalf("override status=%d settings=%+v", response.Code, settings)
	}
	for _, read := range []struct{ path, token string }{
		{"/api/v2/me/practice-settings", "student"},
		{"/api/v2/users/" + studentID + "/practice-settings", "other"},
	} {
		response := testRequest(t, fixture.handler, http.MethodGet, read.path, read.token, "")
		if settings := decodePracticeSettings(t, response.Body.Bytes()); response.Code != http.StatusOK || settings != want {
			t.Fatalf("%s read status=%d settings=%+v", read.token, response.Code, settings)
		}
	}

	for range 2 {
		response = testRequest(t, fixture.handler, http.MethodDelete, path, "other", "")
		if settings := decodePracticeSettings(t, response.Body.Bytes()); response.Code != http.StatusOK || settings != programmeSettings {
			t.Fatalf("clear status=%d settings=%+v", response.Code, settings)
		}
	}
	var audits int
	if err := fixture.pool.QueryRow(ctx, `SELECT count(*) FROM app.audit_events WHERE subject_id=$1 AND action LIKE 'practice_goals.student_%'`, studentID).Scan(&audits); err != nil || audits != 2 {
		t.Fatalf("override audits=%d err=%v", audits, err)
	}

	if _, err := fixture.pool.Exec(ctx, `UPDATE app.seasons SET status='closed',closed_at=now() WHERE id=$1`, seasonID); err != nil {
		t.Fatal(err)
	}
	if response := testRequest(t, fixture.handler, http.MethodPut, path, "other", override); response.Code != http.StatusConflict {
		t.Fatalf("closed season status=%d body=%s", response.Code, response.Body)
	}
}
