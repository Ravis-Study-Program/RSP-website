//go:build integration

package api

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/magedmg/RSP-website/backend/internal/mockinterviews"
	"github.com/magedmg/RSP-website/backend/internal/platform/id"
	"github.com/magedmg/RSP-website/backend/internal/practice"
)

func TestActivitySelectionsCombineGroupsAndBindCursors(t *testing.T) {
	f := newPostgresFixture(t)
	ctx := context.Background()
	weekID, categoryID := id.New(), id.New()
	for _, q := range []struct {
		sql  string
		args []any
	}{
		{`INSERT INTO app.season_weeks(id,season_id,week_number,start_at,end_at) VALUES($1,$2,1,'2026-01-01','2026-01-07')`, []any{weekID, seasonID}},
		{`INSERT INTO app.leetcode_problem_categories(id,name,normalized_name) VALUES($1,'Arrays','arrays')`, []any{categoryID}},
		{`INSERT INTO app.leetcode_problem_category_mappings(leetcode_problem_id,category_id) VALUES($1,$2)`, []any{leetcodeProblemID, categoryID}},
	} {
		if _, err := f.pool.Exec(ctx, q.sql, q.args...); err != nil {
			t.Fatal(err)
		}
	}
	for _, date := range []string{"2026-01-02T12:00:00Z", "2026-01-03T12:00:00Z", "2026-02-02T12:00:00Z"} {
		if _, err := f.db.CreateAttempt(ctx, practice.AttemptRecord{ID: id.New(), UserID: studentID, ProblemID: leetcodeProblemID, Outcome: "independently_solved", Minutes: 20, AttemptedAt: activityTime(t, date)}); err != nil {
			t.Fatal(err)
		}
	}
	path := "/api/v2/problem-attempts?seasonId=" + seasonID + "&weekId=" + weekID + "&difficulty=easy&difficulty=hard&category=Arrays&category=Trees&limit=1"
	response := testRequest(t, f.handler, http.MethodGet, path, "student", "")
	var page struct {
		TotalCount int                      `json:"totalCount"`
		Items      []practice.AttemptRecord `json:"items"`
		PageInfo   struct {
			NextCursor string `json:"nextCursor"`
		} `json:"pageInfo"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &page); err != nil || response.Code != 200 || page.TotalCount != 2 || len(page.Items) != 1 {
		t.Fatalf("selection=%d %s error=%v", response.Code, response.Body, err)
	}
	response = testRequest(t, f.handler, http.MethodGet, path+"&cursor="+page.PageInfo.NextCursor, "student", "")
	if response.Code != 200 {
		t.Fatalf("next page=%d %s", response.Code, response.Body)
	}
	response = testRequest(t, f.handler, http.MethodGet, path+"&category=Graphs&cursor="+page.PageInfo.NextCursor, "student", "")
	if response.Code != 400 {
		t.Fatalf("changed cursor filters=%d %s", response.Code, response.Body)
	}
	response = testRequest(t, f.handler, http.MethodGet, "/api/v2/problem-attempts?difficulty=medium&category=Arrays", "student", "")
	if err := json.Unmarshal(response.Body.Bytes(), &page); err != nil || response.Code != 200 || page.TotalCount != 0 {
		t.Fatalf("AND groups=%d %s", response.Code, response.Body)
	}
	for _, scores := range [][2]int{{8, 7}, {9, 4}} {
		scoreA, scoreB := scores[0], scores[1]
		if _, err := f.db.CreateMockInterview(ctx, mockinterviews.Interview{ID: id.New(), InterviewerID: otherID, IntervieweeID: studentID, OccurredAt: time.Now(), DurationMinutes: 30, Rounds: []mockinterviews.Round{{ID: id.New(), Type: mockinterviews.LeetCode, ProblemID: leetcodeProblemID, Scores: mockinterviews.Scores{ConfirmQuestions: &scoreA, AlgorithmDesign: &scoreA, ComplexityAnalysis: &scoreA, Coding: &scoreA, Testing: &scoreB}}}}, otherID, time.Now()); err != nil {
			t.Fatal(err)
		}
	}
	for _, passed := range []string{"true", "false"} {
		response = testRequest(t, f.handler, http.MethodGet, "/api/v2/mock-interviews?passed="+passed+"&interviewerId="+otherID, "student", "")
		var result struct {
			TotalCount int `json:"totalCount"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil || response.Code != 200 || result.TotalCount != 1 {
			t.Fatalf("result=%d %s error=%v", response.Code, response.Body, err)
		}
	}
	for _, path := range []string{"/api/v2/problem-attempts?difficulty=extreme", "/api/v2/problem-attempts?weekId=" + weekID, "/api/v2/mock-interviews?passed=maybe", "/api/v2/mock-interviews?interviewerId=bad"} {
		if response := testRequest(t, f.handler, http.MethodGet, path, "student", ""); response.Code != 400 {
			t.Fatalf("invalid selection=%d %s", response.Code, response.Body)
		}
	}
}
