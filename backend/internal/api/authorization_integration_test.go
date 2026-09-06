//go:build integration

package api

import (
	"context"
	"net/http"
	"testing"

	"github.com/magedmg/RSP-website/backend/internal/authz"
)

func TestPrivateSettingsUseHistoricalRelationshipPolicy(t *testing.T) {
	fixture := newPostgresFixture(t)
	actor := authz.Actor{
		UserID:        otherID,
		EmailVerified: true,
		AccountState:  authz.AccountActive,
		Enrollments:   []authz.Enrollment{{SeasonID: seasonID, Role: authz.Coordinator, State: authz.Completed}},
	}
	handler := New(Config{
		DB:            fixture.db,
		Authenticator: AuthenticatorFunc(func(context.Context, string) (authz.Actor, error) { return actor, nil }),
		CursorSecret:  []byte("0123456789abcdef"),
	}).Handler()
	request := func(want int) {
		t.Helper()
		response := testRequest(t, handler, http.MethodGet, "/api/v2/users/"+studentID+"/practice-settings", "actor", "")
		if response.Code != want {
			t.Fatalf("private settings status=%d, want %d: %s", response.Code, want, response.Body.String())
		}
	}
	request(http.StatusOK)
	if _, err := fixture.pool.Exec(context.Background(), `UPDATE app.enrollments SET state='withdrawn',assignment_state='revoked',activated_at=NULL WHERE id=$1`, studentMemberID); err != nil {
		t.Fatal(err)
	}
	request(http.StatusNotFound)
	if _, err := fixture.pool.Exec(context.Background(), `UPDATE app.enrollments SET state='active',assignment_state='active',activated_at=now() WHERE id=$1`, studentMemberID); err != nil {
		t.Fatal(err)
	}

	actor.Enrollments[0].SeasonID = "another-season"
	request(http.StatusNotFound)
	actor.Enrollments[0].SeasonID = seasonID
	actor.Enrollments[0].State = authz.Withdrawn
	request(http.StatusNotFound)

	actor.Enrollments[0] = authz.Enrollment{
		SeasonID: seasonID,
		Role:     authz.Mentor,
		State:    authz.Completed,
	}
	request(http.StatusNotFound)
	if _, err := fixture.pool.Exec(context.Background(), `INSERT INTO app.mentorships(id,season_id,mentor_enrollment_id,student_enrollment_id)
		VALUES ('00000000-0000-7000-8000-000000000110',$1,$2,$3)`, seasonID, otherMemberID, studentMemberID); err != nil {
		t.Fatal(err)
	}
	request(http.StatusOK)
	if _, err := fixture.pool.Exec(context.Background(), `UPDATE app.mentorships SET ended_at=now() WHERE season_id=$1`, seasonID); err != nil {
		t.Fatal(err)
	}
	request(http.StatusNotFound)
}
