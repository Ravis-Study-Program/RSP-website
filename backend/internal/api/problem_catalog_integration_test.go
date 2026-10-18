//go:build integration

package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/magedmg/RSP-website/backend/internal/platform/id"
)

func TestProblemSearchMatchesTitleOrNumberOnly(t *testing.T) {
	f := newPostgresFixture(t)
	search := func(q string) []string {
		t.Helper()
		res := testRequest(t, f.handler, http.MethodGet, "/api/v2/leetcode-problems?limit=20&q="+url.QueryEscape(q), "student", "")
		var page struct {
			Items []struct {
				Title string `json:"title"`
			} `json:"items"`
		}
		if err := json.Unmarshal(res.Body.Bytes(), &page); err != nil || res.Code != http.StatusOK {
			t.Fatalf("search %q status=%d body=%s err=%v", q, res.Code, res.Body, err)
		}
		titles := []string{}
		for _, item := range page.Items {
			titles = append(titles, item.Title)
		}
		return titles
	}
	for _, q := range []string{"two", "SUM", "1"} {
		if got := search(q); len(got) != 1 || got[0] != "Two Sum" {
			t.Fatalf("search %q = %v", q, got)
		}
	}
	// Number matches are exact, and LIKE wildcards are literal.
	for _, q := range []string{"11", "%", "_", "Two_Sum"} {
		if got := search(q); len(got) != 0 {
			t.Fatalf("search %q = %v, want none", q, got)
		}
	}
	res := testRequest(t, f.handler, http.MethodGet, "/api/v2/leetcode-problems?q="+strings.Repeat("a", 101), "student", "")
	if res.Code != http.StatusBadRequest {
		t.Fatalf("long search status=%d", res.Code)
	}
}

func TestProblemCategoriesListOnlyLiveMappedCategories(t *testing.T) {
	f := newPostgresFixture(t)
	ctx := context.Background()
	mapped, unmapped, deleted := id.New(), id.New(), id.New()
	for _, statement := range []struct {
		sql  string
		args []any
	}{
		{`INSERT INTO app.leetcode_problem_categories(id,name,normalized_name) VALUES($1,'Arrays','arrays')`, []any{mapped}},
		{`INSERT INTO app.leetcode_problem_categories(id,name,normalized_name) VALUES($1,'Graphs','graphs')`, []any{unmapped}},
		{`INSERT INTO app.leetcode_problem_categories(id,name,normalized_name,deleted_at) VALUES($1,'Heaps','heaps',now())`, []any{deleted}},
		{`INSERT INTO app.leetcode_problem_category_mappings(leetcode_problem_id,category_id) VALUES($1,$2),($1,$3)`, []any{leetcodeProblemID, mapped, deleted}},
	} {
		if _, err := f.pool.Exec(ctx, statement.sql, statement.args...); err != nil {
			t.Fatal(err)
		}
	}
	res := testRequest(t, f.handler, http.MethodGet, "/api/v2/leetcode-categories", "student", "")
	if res.Code != http.StatusOK || strings.TrimSpace(res.Body.String()) != `{"items":["arrays"]}` {
		t.Fatalf("categories status=%d body=%s", res.Code, res.Body)
	}
}

func TestAttemptsAndMockRoundsEmbedTheirProblem(t *testing.T) {
	f := newPostgresFixture(t)
	type embedded struct {
		Problem *struct {
			ID    string `json:"id"`
			Title string `json:"title"`
		} `json:"problem"`
	}
	assertTwoSum := func(label string, got embedded) {
		t.Helper()
		if got.Problem == nil || got.Problem.ID != leetcodeProblemID || got.Problem.Title != "Two Sum" {
			t.Fatalf("%s problem=%+v", label, got.Problem)
		}
	}

	body := `{"problemId":"` + leetcodeProblemID + `","outcome":"independently_solved","minutes":25,"attemptedAt":"2026-09-01T00:00:00Z"}`
	res := testRequest(t, f.handler, http.MethodPost, "/api/v2/problem-attempts", "student", body)
	var created struct {
		ID string `json:"id"`
		embedded
	}
	if err := json.Unmarshal(res.Body.Bytes(), &created); err != nil || res.Code != http.StatusCreated {
		t.Fatalf("create attempt status=%d body=%s", res.Code, res.Body)
	}
	assertTwoSum("created attempt", created.embedded)
	res = testRequest(t, f.handler, http.MethodPatch, "/api/v2/problem-attempts/"+created.ID, "student", body)
	var updated embedded
	if err := json.Unmarshal(res.Body.Bytes(), &updated); err != nil || res.Code != http.StatusOK {
		t.Fatalf("update attempt status=%d body=%s", res.Code, res.Body)
	}
	assertTwoSum("updated attempt", updated)
	res = testRequest(t, f.handler, http.MethodGet, "/api/v2/problem-attempts", "student", "")
	var attempts struct{ Items []embedded }
	if err := json.Unmarshal(res.Body.Bytes(), &attempts); err != nil || len(attempts.Items) != 1 {
		t.Fatalf("list attempts status=%d body=%s", res.Code, res.Body)
	}
	assertTwoSum("listed attempt", attempts.Items[0])

	mock := `{"interviewee":{"userId":"` + otherID + `"},"occurredAt":"` + time.Now().UTC().Truncate(time.Second).Format(time.RFC3339) + `","durationMinutes":45,"rounds":[` +
		`{"id":"` + id.New() + `","type":"leetcode","problemId":"` + leetcodeProblemID + `","scores":{"confirmQuestions":7,"algorithmDesign":8,"complexityAnalysis":7,"coding":8,"testing":7}},` +
		`{"id":"` + id.New() + `","type":"behavioural","scores":{"behavioural":6}}]}`
	res = testRequest(t, f.handler, http.MethodPost, "/api/v2/mock-interviews", "student", mock)
	var result struct {
		Interview struct {
			ID     string     `json:"id"`
			Rounds []embedded `json:"rounds"`
		} `json:"interview"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &result); err != nil || res.Code != http.StatusCreated || len(result.Interview.Rounds) != 2 {
		t.Fatalf("create mock status=%d body=%s", res.Code, res.Body)
	}
	assertTwoSum("created round", result.Interview.Rounds[0])
	if result.Interview.Rounds[1].Problem != nil {
		t.Fatalf("behavioural round has a problem: %+v", result.Interview.Rounds[1].Problem)
	}
	res = testRequest(t, f.handler, http.MethodGet, "/api/v2/mock-interviews?mode=given", "student", "")
	var mocks struct {
		Items []struct {
			Rounds []embedded `json:"rounds"`
		}
	}
	if err := json.Unmarshal(res.Body.Bytes(), &mocks); err != nil || len(mocks.Items) != 1 {
		t.Fatalf("list mocks status=%d body=%s", res.Code, res.Body)
	}
	assertTwoSum("listed round", mocks.Items[0].Rounds[0])

	// Catalogue data is response-only; immutable version snapshots never copy it.
	var snapshotHasProblem bool
	if err := f.pool.QueryRow(context.Background(), `SELECT bool_or(round ? 'problem') FROM app.mock_interview_versions v,
		jsonb_array_elements(v.snapshot->'rounds') round WHERE v.mock_interview_id=$1`, result.Interview.ID).Scan(&snapshotHasProblem); err != nil || snapshotHasProblem {
		t.Fatalf("snapshot includes problem=%v err=%v", snapshotHasProblem, err)
	}
}
