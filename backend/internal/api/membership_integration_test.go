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
	"github.com/magedmg/RSP-website/backend/internal/programme"
)

func TestSeasonMemberRoleChangesAndRemoval(t *testing.T) {
	for _, scenario := range []struct{ name, method, suffix, body, role, level, state, audit string }{
		{"edit role", http.MethodPatch, "", `{"role":"mentor"}`, "mentor", "not_applicable", "active", "enrollment.updated"},
		{"promote", http.MethodPost, "/promote", `{"role":"coordinator"}`, "coordinator", "not_applicable", "active", "enrollment.promoted"},
		{"remove", http.MethodPost, "/remove", `{"reason":"  no longer participating  "}`, "student", "beginner", "kicked", "enrollment.removed"},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			fixture := newPostgresFixture(t)
			now := time.Now().UTC()
			if _, err := fixture.db.CreateMentorship(context.Background(), programme.MentorshipRecord{
				ID:            id.New(),
				SeasonID:      seasonID,
				MentorUserID:  otherID,
				StudentUserID: studentID,
			}, otherID, now); err != nil {
				t.Fatal(err)
			}
			handler := New(Config{DB: fixture.db, Authenticator: AuthenticatorFunc(func(context.Context, string) (authz.Actor, error) {
				return authz.Actor{
					UserID:        otherID,
					EmailVerified: true,
					AccountState:  authz.AccountActive,
					GlobalRoles:   map[authz.GlobalRole]bool{authz.Director: true},
					MFAAt:         &now,
				}, nil
			})}).Handler()
			response := testRequest(t, handler, scenario.method, "/api/v2/seasons/"+seasonID+"/members/"+studentMemberID+scenario.suffix, "admin", scenario.body)
			if response.Code != http.StatusOK {
				t.Fatalf("status=%d body=%s", response.Code, response.Body)
			}
			var member programme.EnrollmentRecord
			if err := json.Unmarshal(response.Body.Bytes(), &member); err != nil {
				t.Fatal(err)
			}
			if member.Role != scenario.role || member.StudentLevel != scenario.level || member.State != scenario.state {
				t.Fatalf("member=%+v", member)
			}
			stored, err := fixture.db.GetEnrollment(context.Background(), studentMemberID)
			if err != nil || stored.Role != member.Role || stored.State != member.State {
				t.Fatalf("stored=%+v err=%v", stored, err)
			}
			var activeLinks, events int
			if err := fixture.pool.QueryRow(context.Background(), `SELECT count(*) FROM app.mentorships WHERE ended_at IS NULL`).Scan(&activeLinks); err != nil {
				t.Fatal(err)
			}
			if err := fixture.pool.QueryRow(context.Background(), `SELECT count(*) FROM app.audit_events WHERE action=$1 AND subject_id=$2`, scenario.audit, studentMemberID).Scan(&events); err != nil {
				t.Fatal(err)
			}
			if activeLinks != 0 || events != 1 {
				t.Fatalf("active mentorships=%d audit events=%d", activeLinks, events)
			}
			if scenario.state == "kicked" && (stored.RemovalReason == nil || *stored.RemovalReason != "no longer participating") {
				t.Fatalf("reason=%v", stored.RemovalReason)
			}
			if scenario.role == "coordinator" && stored.AssignmentState != "pending_mfa" {
				t.Fatalf("coordinator state=%s", stored.AssignmentState)
			}
		})
	}
}
