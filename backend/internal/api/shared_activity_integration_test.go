//go:build integration

package api

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/magedmg/RSP-website/backend/internal/authz"
)

func TestSharedFeedbackRetainsOwnershipAndContactProtection(t *testing.T) {
	f := newPostgresFixture(t)
	ctx := context.Background()
	const readerID = "00000000-0000-7000-8000-000000000120"
	if _, err := f.pool.Exec(ctx, `INSERT INTO app.users(id) VALUES ($1);
`, readerID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.pool.Exec(ctx, `INSERT INTO app.enrollments(user_id,season_id,role,student_level,state)
	 VALUES ($1,$2,'student','novice','active')`, readerID, seasonID); err != nil {
		t.Fatal(err)
	}
	actor := authz.Actor{UserID: readerID, EmailVerified: true, AccountState: authz.AccountActive,
		Enrollments: []authz.Enrollment{{SeasonID: seasonID, Role: authz.Student, State: authz.Active}}}
	handler := New(Config{DB: f.db, CursorSecret: []byte("0123456789abcdef"),
		Authenticator: AuthenticatorFunc(func(context.Context, string) (authz.Actor, error) { return actor, nil }),
	}).Handler()
	if _, err := f.pool.Exec(ctx, `UPDATE app.problem_attempts SET notes_html='Shared practice feedback',confidence=4 WHERE id=$1`, foreignAttemptID); err != nil {
		t.Fatal(err)
	}
	mock := createMockForWrites(t, f)
	for _, path := range []string{"/api/v2/problem-attempts?userId=" + otherID, "/api/v2/mock-interviews?mode=all"} {
		res := testRequest(t, handler, http.MethodGet, path, "reader", "")
		if res.Code != http.StatusOK || (strings.Contains(path, "problem-attempts") && !strings.Contains(res.Body.String(), "Shared practice feedback")) || (strings.Contains(path, "mock-interviews") && !strings.Contains(res.Body.String(), mock.ID)) {
			t.Fatalf("shared read %s: %d %s", path, res.Code, res.Body)
		}
	}
	res := testRequest(t, handler, http.MethodGet, "/api/v2/users/"+otherID, "reader", "")
	if res.Code != http.StatusOK || strings.Contains(res.Body.String(), "other@example.com") {
		t.Fatalf("contact disclosure: %d %s", res.Code, res.Body)
	}
	res = testRequest(t, handler, http.MethodDelete, "/api/v2/mock-interviews/"+mock.ID, "reader", "")
	if res.Code != http.StatusNotFound {
		t.Fatalf("shared reader can delete mock: %d %s", res.Code, res.Body)
	}
	actor.Enrollments[0].State = authz.Kicked
	res = testRequest(t, handler, http.MethodGet, "/api/v2/problem-attempts?userId="+otherID, "reader", "")
	if res.Code != http.StatusNotFound {
		t.Fatalf("removed reader has shared access: %d %s", res.Code, res.Body)
	}
}
