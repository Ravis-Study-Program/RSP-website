//go:build integration

package api

import (
	"context"
	"github.com/magedmg/RSP-website/backend/internal/authz"
	"github.com/magedmg/RSP-website/backend/internal/platform/id"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestStaffRemovalRetainsHistory(t *testing.T) {
	for _, role := range []string{"mentor", "coordinator"} {
		t.Run(role, func(t *testing.T) {
			f := newPostgresFixture(t)
			ctx := context.Background()
			at := time.Now()
			if _, err := f.pool.Exec(ctx, `UPDATE app.enrollments SET role=$2 WHERE id=$1`, otherMemberID, role); err != nil {
				t.Fatal(err)
			}
			if role == "mentor" {
				if _, err := f.pool.Exec(ctx, `INSERT INTO app.mentorships(id,season_id,mentor_enrollment_id,student_enrollment_id) VALUES($1,$2,$3,$4)`, id.New(), seasonID, otherMemberID, studentMemberID); err != nil {
					t.Fatal(err)
				}
			}
			handler := New(Config{DB: f.db, Authenticator: AuthenticatorFunc(func(_ context.Context, token string) (authz.Actor, error) {
				actor := authz.Actor{UserID: studentID, EmailVerified: true, AccountState: authz.AccountActive, MFAAt: &at}
				if token == "admin" {
					actor.GlobalRoles = map[authz.GlobalRole]bool{authz.SystemAdmin: true}
				}
				return actor, nil
			})}).Handler()
			path := "/api/v2/seasons/" + seasonID + "/members/" + otherMemberID + "/remove"
			if res := testRequest(t, handler, http.MethodPost, path, "student", `{"reason":"No longer volunteering"}`); res.Code != 403 {
				t.Fatalf("unauthorized=%d", res.Code)
			}
			res := testRequest(t, handler, http.MethodPost, path, "admin", `{"reason":"No longer volunteering"}`)
			if res.Code != 200 || !strings.Contains(res.Body.String(), `"withdrawn"`) || !strings.Contains(res.Body.String(), `"revoked"`) {
				t.Fatalf("remove=%d %s", res.Code, res.Body)
			}
			var activeAssignments, history, attempts int
			if err := f.pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM app.mentorships WHERE ended_at IS NULL),(SELECT count(*) FROM app.enrollment_participation WHERE enrollment_id=$1 AND ended_at IS NOT NULL),(SELECT count(*) FROM app.problem_attempts WHERE user_id=$2 AND deleted_at IS NULL)`, otherMemberID, otherID).Scan(&activeAssignments, &history, &attempts); err != nil || activeAssignments != 0 || history < 1 || attempts != 1 {
				t.Fatalf("retained history=%d/%d/%d %v", activeAssignments, history, attempts, err)
			}
		})
	}
}
