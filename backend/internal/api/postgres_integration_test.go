//go:build integration

package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/magedmg/RSP-website/backend/internal/authadmin"
	"github.com/magedmg/RSP-website/backend/internal/authz"
	"github.com/magedmg/RSP-website/backend/internal/dal"
)

const (
	studentID         = "00000000-0000-7000-8000-000000000101"
	otherID           = "00000000-0000-7000-8000-000000000102"
	seasonID          = "00000000-0000-7000-8000-000000000103"
	studentMemberID   = "00000000-0000-7000-8000-000000000104"
	otherMemberID     = "00000000-0000-7000-8000-000000000105"
	problemID         = "00000000-0000-7000-8000-000000000106"
	leetcodeProblemID = "00000000-0000-7000-8000-000000000107"
	mockRoundID       = "00000000-0000-7000-8000-000000000108"
	foreignAttemptID  = "00000000-0000-7000-8000-000000000109"
)

type postgresFixture struct {
	db      *dal.Store
	pool    *pgxpool.Pool
	handler http.Handler
}

func newPostgresFixture(t *testing.T) postgresFixture {
	t.Helper()
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is required for PostgreSQL integration tests")
	}
	separator := "?"
	if strings.Contains(databaseURL, "?") {
		separator = "&"
	}
	config, err := pgxpool.ParseConfig(databaseURL + separator + "options=-c%20search_path%3Dapp")
	if err != nil {
		t.Fatal(err)
	}
	config.ConnConfig.RuntimeParams["timezone"] = "UTC"
	pool, err := pgxpool.NewWithConfig(context.Background(), config)
	if err != nil {
		t.Fatal(err)
	}
	db := dal.New(pool)
	t.Cleanup(db.Close)
	truncateAppTables(t, pool)
	t.Cleanup(func() { truncateAppTables(t, pool) })
	seedPostgresFixture(t, pool)

	actors := map[string]authz.Actor{
		"student": {
			UserID:        studentID,
			EmailVerified: true,
			AccountState:  authz.AccountActive,
			Enrollments:   []authz.Enrollment{{SeasonID: seasonID, Role: authz.Student, State: authz.Active}},
		},
		"other": {
			UserID:        otherID,
			EmailVerified: true,
			AccountState:  authz.AccountActive,
			Enrollments:   []authz.Enrollment{{SeasonID: seasonID, Role: authz.Mentor, State: authz.Active}},
		},
	}
	authenticator := AuthenticatorFunc(func(_ context.Context, token string) (authz.Actor, error) {
		actor, ok := actors[token]
		if !ok {
			return authz.Actor{}, errors.New("missing test actor")
		}
		return actor, nil
	})
	api := New(Config{
		DB:                db,
		Authenticator:     authenticator,
		PublicOrigin:      "https://rsp.test",
		CursorSecret:      []byte("0123456789abcdef"),
		Ready:             func() error { return db.Ping(context.Background()) },
		QueueLeetCodeSync: func(context.Context, string, string) error { return nil },
		SetAccountState:   func(context.Context, authadmin.SetAccountStateInput) error { return nil },
		GetMFAConfigured:  func(context.Context, string) (bool, error) { return false, nil },
	})
	return postgresFixture{
		db:      db,
		pool:    pool,
		handler: api.Handler(),
	}
}

func truncateAppTables(t *testing.T, db *pgxpool.Pool) {
	t.Helper()
	rows, err := db.Query(context.Background(), `SELECT format('%I.%I', schemaname, tablename)
		FROM pg_tables WHERE schemaname = 'app' ORDER BY tablename`)
	if err != nil {
		t.Fatal(err)
	}
	tables, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		t.Fatal(err)
	}
	if len(tables) == 0 {
		t.Fatal("app schema is not migrated")
	}
	if _, err := db.Exec(context.Background(), "TRUNCATE TABLE "+strings.Join(tables, ", ")+" RESTART IDENTITY CASCADE"); err != nil {
		t.Fatal(err)
	}
}

func seedPostgresFixture(t *testing.T, db *pgxpool.Pool) {
	t.Helper()
	statements := []string{
		`INSERT INTO app.users(id,account_state) VALUES
			('` + studentID + `','active'),
			('` + otherID + `','active')`,
		`INSERT INTO app.user_profiles(user_id,slug,display_name) VALUES
			('` + studentID + `','student','Student'),
			('` + otherID + `','other','Other')`,
		`INSERT INTO app.user_preferences(user_id,timezone) VALUES
			('` + studentID + `','Australia/Adelaide'),
			('` + otherID + `','Australia/Adelaide')`,
		`INSERT INTO app.user_contacts(user_id,email) VALUES
			('` + studentID + `','private@example.com'),
			('` + otherID + `','other@example.com')`,
		`INSERT INTO app.user_security(user_id) VALUES
			('` + studentID + `'),
			('` + otherID + `')`,
		`INSERT INTO app.seasons(id,slug,name,status,start_at,end_at,location) VALUES
			('` + seasonID + `','season','Season','open','2026-01-01T00:00:00Z','2026-12-31T23:59:59Z','Adelaide')`,
		`INSERT INTO app.enrollments(id,user_id,season_id,role,student_level,state,created_at) VALUES
			('` + studentMemberID + `','` + studentID + `','` + seasonID + `','student','beginner','active','2026-01-01T00:00:00Z'),
			('` + otherMemberID + `','` + otherID + `','` + seasonID + `','mentor','not_applicable','active','2026-01-01T00:00:00Z')`,
		`INSERT INTO app.problems(id,title,url) VALUES
			('` + problemID + `','Two Sum','https://leetcode.com/problems/two-sum/')`,
		`INSERT INTO app.leetcode_problems(id,problem_id,leetcode_number,difficulty,is_premium) VALUES
			('` + leetcodeProblemID + `','` + problemID + `',1,'easy',false)`,
		`INSERT INTO app.problem_attempts(id,user_id,problem_id,attempted_at,time_taken_minutes,outcome) VALUES
			('` + foreignAttemptID + `','` + otherID + `','` + problemID + `','2026-09-01T00:00:00Z',20,'independently_solved')`,
	}
	for _, statement := range statements {
		if _, err := db.Exec(context.Background(), statement); err != nil {
			t.Fatal(err)
		}
	}
}

func TestPostgresBackedProfileAndPracticeFlow(t *testing.T) {
	fixture := newPostgresFixture(t)

	response := testRequest(t, fixture.handler, http.MethodGet, "/api/v2/me", "student", "")
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), studentID) {
		t.Fatalf("profile status=%d body=%s", response.Code, response.Body.String())
	}
	response = testRequest(t, fixture.handler, http.MethodGet, "/api/v2/users/"+otherID, "student", "")
	if response.Code != http.StatusOK || strings.Contains(response.Body.String(), "other@example.com") {
		t.Fatalf("public profile status=%d body=%s", response.Code, response.Body.String())
	}

	body := `{"problemId":"` + leetcodeProblemID + `","outcome":"independently_solved","confidence":5,"minutes":25,"notes":"<p><strong>kept</strong><script>removed()</script></p>","attemptedAt":"2026-09-01T00:00:00Z"}`
	response = testRequest(t, fixture.handler, http.MethodPost, "/api/v2/problem-attempts", "student", body)
	if response.Code != http.StatusCreated {
		t.Fatalf("attempt status=%d body=%s", response.Code, response.Body.String())
	}
	var attempt struct {
		ID    string `json:"id"`
		Notes string `json:"notes"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &attempt); err != nil || attempt.ID == "" || strings.Contains(attempt.Notes, "script") || !strings.Contains(attempt.Notes, "<strong>kept</strong>") {
		t.Fatalf("attempt body=%s error=%v", response.Body.String(), err)
	}
	var count int64
	if err := fixture.pool.QueryRow(context.Background(), `SELECT count(*) FROM app.problem_attempts WHERE id=$1 AND user_id=$2`, attempt.ID, studentID).Scan(&count); err != nil || count != 1 {
		t.Fatalf("stored attempts=%d error=%v", count, err)
	}
	response = testRequest(t, fixture.handler, http.MethodDelete, "/api/v2/problem-attempts/"+foreignAttemptID+"", "student", "")
	if response.Code != http.StatusNotFound {
		t.Fatalf("foreign delete status=%d body=%s", response.Code, response.Body.String())
	}
	response = testRequest(t, fixture.handler, http.MethodDelete, "/api/v2/problem-attempts/"+attempt.ID+"", "student", "")
	if response.Code != http.StatusNoContent {
		t.Fatalf("delete status=%d body=%s", response.Code, response.Body.String())
	}

	response = testRequest(t, fixture.handler, http.MethodGet, "/api/v2/metrics", "", "")
	if response.Code != http.StatusOK || response.Header().Get("Content-Type") != "text/plain; version=0.0.4; charset=utf-8" {
		t.Fatalf("metrics status=%d content-type=%q", response.Code, response.Header().Get("Content-Type"))
	}
}

func TestPostgresBackedMockInterviewSurvivesHandlerRecreation(t *testing.T) {
	fixture := newPostgresFixture(t)
	body := `{"interviewee":{"userId":"` + otherID + `"},"occurredAt":"` + time.Now().UTC().Truncate(time.Second).Format(time.RFC3339) + `","durationMinutes":60,"rounds":[{"id":"` + mockRoundID + `","type":"behavioural","scores":{"behavioural":7}}]}`

	response := testRequest(t, fixture.handler, http.MethodPost, "/api/v2/mock-interviews", "student", body)
	if response.Code != http.StatusCreated {
		t.Fatalf("create status=%d body=%s", response.Code, response.Body.String())
	}

	fixture.handler = New(Config{
		DB: fixture.db,
		Authenticator: AuthenticatorFunc(func(context.Context, string) (authz.Actor, error) {
			return authz.Actor{
				UserID:        studentID,
				EmailVerified: true,
				AccountState:  authz.AccountActive,
				Enrollments:   []authz.Enrollment{{SeasonID: seasonID, Role: authz.Student, State: authz.Active}},
			}, nil
		}),
		PublicOrigin: "https://rsp.test",
		CursorSecret: []byte("0123456789abcdef"),
	}).Handler()
	response = testRequest(t, fixture.handler, http.MethodGet, "/api/v2/mock-interviews?mode=given", "student", "")
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"totalCount":1`) {
		t.Fatalf("list status=%d body=%s", response.Code, response.Body.String())
	}
}
