//go:build integration

package api

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/magedmg/RSP-website/backend/internal/platform/id"
	"github.com/magedmg/RSP-website/backend/internal/practice"
)

func TestSuggestionsExcludeLifetimeAttemptsAndRespectSelections(t *testing.T) {
	f := newPostgresFixture(t)
	check := func(query string, want bool) {
		t.Helper()
		res := testRequest(t, f.handler, http.MethodGet, "/api/v2/me/problem-suggestion"+query, "student", "")
		var result struct {
			Problem *practice.ProblemRecord `json:"problem"`
		}
		if err := json.Unmarshal(res.Body.Bytes(), &result); err != nil || res.Code != 200 || (result.Problem != nil) != want {
			t.Fatalf("suggestion=%d %s err=%v", res.Code, res.Body, err)
		}
	}
	check("?difficulty=easy", true)
	check("?difficulty=hard", false)
	check("?category=Trees", false)
	// This attempt predates the student's season; the suggestion still excludes it.
	if _, err := f.db.CreateAttempt(context.Background(), practice.AttemptRecord{ID: id.New(), UserID: studentID, ProblemID: leetcodeProblemID, Outcome: "not_solved", Minutes: 10, AttemptedAt: activityTime(t, "2025-01-01T00:00:00Z")}); err != nil {
		t.Fatal(err)
	}
	check("?difficulty=easy", false)
	var count int
	if err := f.pool.QueryRow(context.Background(), `SELECT count(*) FROM app.problem_attempts WHERE user_id=$1`, studentID).Scan(&count); err != nil || count != 1 {
		t.Fatalf("suggesting should not record attempts: %d %v", count, err)
	}
}
