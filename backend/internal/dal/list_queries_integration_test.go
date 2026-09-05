//go:build integration

package dal

import (
	"context"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/magedmg/RSP-website/backend/internal/accounts"
	"github.com/magedmg/RSP-website/backend/internal/practice"
	"github.com/magedmg/RSP-website/backend/internal/programme"
	"github.com/pressly/goose/v3"
	"github.com/testcontainers/testcontainers-go"
	postgrescontainer "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

func TestStoreListQueries(t *testing.T) {
	db := newListQueryFixture(t)
	ctx := context.Background()

	t.Run("directory filters and cursor directions", func(t *testing.T) {
		id := func(user accounts.User) string { return user.ID }
		expectListPage(t, []int{1, 2}, true, 4, id)(db.ListUsers(ctx, "", 2, "forward", "", "", ""))
		expectListPage(t, []int{3, 4}, false, 4, id)(db.ListUsers(ctx, queryFixtureID(2), 2, "forward", "", "", ""))
		expectListPage(t, []int{2, 3}, true, 4, id)(db.ListUsers(ctx, queryFixtureID(4), 2, "backward", "", "", ""))
		expectListPage(t, []int{4}, false, 1, id)(db.ListUsers(ctx, "", 2, "forward", "  dELTa  ", "student", ""))
		expectListPage(t, []int{2}, false, 1, id)(db.ListUsers(ctx, "", 2, "forward", "", "mentor", "director"))
		expectListPage(t, nil, false, 0, id)(db.ListUsers(ctx, "", 2, "forward", "", "student", "director"))
	})

	t.Run("admin filters retain pending roles and private email search", func(t *testing.T) {
		id := func(user accounts.User) string { return user.ID }
		expectListPage(t, []int{7}, false, 1, id)(db.ListAdminUsers(ctx, "", 2, "forward", "", "suspended", "director"))
		expectListPage(t, []int{7}, false, 1, id)(db.ListAdminUsers(ctx, "", 2, "backward", " secret-golf@ ", "", ""))
		expectListPage(t, []int{2}, true, 2, id)(db.ListAdminUsers(ctx, "", 1, "forward", "", "", "director"))
		expectListPage(t, []int{7}, false, 2, id)(db.ListAdminUsers(ctx, queryFixtureID(2), 1, "forward", "", "", "director"))
		expectListPage(t, []int{2}, false, 2, id)(db.ListAdminUsers(ctx, queryFixtureID(7), 1, "backward", "", "", "director"))
	})

	t.Run("candidates exclude existing enrollments and unverified users", func(t *testing.T) {
		id := func(user accounts.EnrollmentCandidate) string { return user.ID }
		expectListPage(t, []int{9}, true, 2, id)(db.ListEnrollmentCandidates(ctx, queryFixtureID(101), "", "", 1, "forward"))
		expectListPage(t, []int{11}, false, 2, id)(db.ListEnrollmentCandidates(ctx, queryFixtureID(101), "", queryFixtureID(9), 1, "forward"))
		expectListPage(t, []int{9}, false, 2, id)(db.ListEnrollmentCandidates(ctx, queryFixtureID(101), "", queryFixtureID(11), 1, "backward"))
		expectListPage(t, []int{11}, false, 1, id)(db.ListEnrollmentCandidates(ctx, queryFixtureID(101), " KILO ", "", 2, "forward"))
	})

	t.Run("season status and backwards order", func(t *testing.T) {
		id := func(season programme.SeasonRecord) string { return season.ID }
		expectListPage(t, []int{101, 102}, true, 3, id)(db.ListSeasons(ctx, "", 2, "forward", ""))
		expectListPage(t, []int{101, 102}, false, 3, id)(db.ListSeasons(ctx, queryFixtureID(103), 2, "backward", ""))
		expectListPage(t, []int{103}, false, 1, id)(db.ListSeasons(ctx, "", 2, "forward", "closed"))
	})

	t.Run("weeks sort by number rather than id", func(t *testing.T) {
		id := func(week programme.WeekRecord) string { return week.ID }
		expectListPage(t, []int{302, 301}, true, 3, id)(db.ListWeeks(ctx, queryFixtureID(101), "", 2, "number:asc", "forward"))
		expectListPage(t, []int{303}, false, 3, id)(db.ListWeeks(ctx, queryFixtureID(101), queryFixtureID(301), 2, "number:asc", "forward"))
		expectListPage(t, []int{302, 301}, false, 3, id)(db.ListWeeks(ctx, queryFixtureID(101), queryFixtureID(303), 2, "number:asc", "backward"))
		expectListPage(t, []int{301, 302}, true, 3, id)(db.ListWeeks(ctx, queryFixtureID(101), "", 2, "id:asc", "forward"))
	})

	t.Run("enrollment state filters and role ordering", func(t *testing.T) {
		id := func(enrollment programme.EnrollmentRecord) string { return enrollment.ID }
		expectListPage(t, []int{202, 201}, true, 6, id)(db.ListEnrollments(ctx, queryFixtureID(101), "", 2, "", "", "role:asc", "forward", false))
		expectListPage(t, []int{202, 201}, false, 6, id)(db.ListEnrollments(ctx, queryFixtureID(101), queryFixtureID(203), 2, "", "", "role:asc", "backward", false))
		expectListPage(t, []int{204}, false, 1, id)(db.ListEnrollments(ctx, queryFixtureID(101), "", 10, "student", "completed", "role:asc", "forward", false))
		expectListPage(t, []int{205}, false, 1, id)(db.ListEnrollments(ctx, queryFixtureID(101), "", 10, "mentor", "completed", "role:asc", "forward", true))
		expectListPage(t, nil, false, 0, id)(db.ListEnrollments(ctx, queryFixtureID(101), "", 10, "mentor", "completed", "role:asc", "forward", false))
		expectListPage(t, []int{206}, false, 1, id)(db.ListEnrollments(ctx, queryFixtureID(101), "", 10, "", "kicked", "id:asc", "forward", true))
	})

	t.Run("mentorship filters share count and page bindings", func(t *testing.T) {
		id := func(mentorship programme.MentorshipRecord) string { return mentorship.ID }
		expectListPage(t, []int{402}, true, 2, id)(db.ListMentorships(ctx, queryFixtureID(101), "", 1, "student:asc", "forward", "", ""))
		expectListPage(t, []int{401}, false, 2, id)(db.ListMentorships(ctx, queryFixtureID(101), queryFixtureID(402), 1, "student:asc", "forward", queryFixtureID(2), ""))
		expectListPage(t, []int{402}, false, 2, id)(db.ListMentorships(ctx, queryFixtureID(101), queryFixtureID(401), 1, "student:asc", "backward", "", ""))
		expectListPage(t, []int{401}, false, 1, id)(db.ListMentorships(ctx, queryFixtureID(101), "", 2, "id:asc", "forward", queryFixtureID(2), queryFixtureID(3)))
		expectListPage(t, nil, false, 0, id)(db.ListMentorships(ctx, queryFixtureID(101), "", 2, "id:asc", "forward", queryFixtureID(1), ""))
	})

	t.Run("problem category difficulty and nullable premium", func(t *testing.T) {
		id := func(problem practice.ProblemRecord) string { return problem.ID }
		premium, free := true, false
		expectListPage(t, []int{601, 602}, true, 3, id)(db.ListProblems(ctx, "", 2, "", "", nil, "forward"))
		expectListPage(t, []int{601, 602}, false, 3, id)(db.ListProblems(ctx, queryFixtureID(603), 2, "", "", nil, "backward"))
		expectListPage(t, []int{603}, false, 3, id)(db.ListProblems(ctx, queryFixtureID(602), 2, "", "", nil, "forward"))
		expectListPage(t, []int{601}, false, 1, id)(db.ListProblems(ctx, "", 2, "easy", "ArRaY", &free, "forward"))
		expectListPage(t, []int{602}, false, 1, id)(db.ListProblems(ctx, "", 2, "easy", "array", &premium, "forward"))
		expectListPage(t, nil, false, 0, id)(db.ListProblems(ctx, "", 2, "hard", "array", nil, "forward"))
	})

	t.Run("attempt user outcome difficulty and backwards pages", func(t *testing.T) {
		id := func(attempt practice.AttemptRecord) string { return attempt.ID }
		expectListPage(t, []int{701, 702}, true, 3, id)(db.ListAttempts(ctx, queryFixtureID(1), "", 2, "", "", "forward"))
		expectListPage(t, []int{701, 702}, false, 3, id)(db.ListAttempts(ctx, queryFixtureID(1), queryFixtureID(703), 2, "", "", "backward"))
		expectListPage(t, []int{703}, false, 3, id)(db.ListAttempts(ctx, queryFixtureID(1), queryFixtureID(702), 2, "", "", "forward"))
		expectListPage(t, []int{702}, false, 1, id)(db.ListAttempts(ctx, queryFixtureID(1), "", 2, "not_solved", "easy", "forward"))
		expectListPage(t, nil, false, 0, id)(db.ListAttempts(ctx, queryFixtureID(1), "", 2, "not_solved", "hard", "forward"))
	})
}

func expectListPage[T any](t *testing.T, wantNumbers []int, wantMore bool, wantTotal int64, id func(T) string) func([]T, bool, int64, error) {
	t.Helper()
	return func(items []T, more bool, total int64, err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
		var got, want []string
		for _, item := range items {
			got = append(got, id(item))
		}
		for _, number := range wantNumbers {
			want = append(want, queryFixtureID(number))
		}
		if !slices.Equal(got, want) || more != wantMore || total != wantTotal {
			t.Fatalf("ids=%v more=%v total=%d; want ids=%v more=%v total=%d", got, more, total, want, wantMore, wantTotal)
		}
	}
}

func queryFixtureID(number int) string {
	return fmt.Sprintf("10000000-0000-7000-8000-%012d", number)
}

// A separate database keeps unfiltered count assertions independent of HTTP
// integration fixtures, which truncate their shared TEST_DATABASE_URL tables.
func newListQueryFixture(t *testing.T) *Store {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	t.Cleanup(cancel)
	container, err := postgrescontainer.Run(ctx, "postgres:18.6-alpine3.24",
		postgrescontainer.WithDatabase("rsp_lists"),
		postgrescontainer.WithUsername("rsp"),
		postgrescontainer.WithPassword("rsp-list-query-tests"),
		testcontainers.WithWaitStrategy(wait.ForLog("database system is ready to accept connections").WithOccurrence(2).WithStartupTimeout(90*time.Second)),
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = testcontainers.TerminateContainer(container) })
	databaseURL, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	db, err := Open(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := goose.SetDialect("postgres"); err != nil {
		t.Fatal(err)
	}
	migrations, err := filepath.Abs("../../../db/migrations")
	if err != nil {
		t.Fatal(err)
	}
	if err := goose.UpContext(ctx, db.SQL, migrations); err != nil {
		t.Fatal(err)
	}
	tx := db.DB.Begin()
	if tx.Error != nil {
		t.Fatal(tx.Error)
	}
	db.DB = tx
	t.Cleanup(func() { _ = tx.Rollback().Error })
	if err := tx.Exec("SET LOCAL search_path = app").Error; err != nil {
		t.Fatal(err)
	}
	exec := func(query string, args ...any) {
		t.Helper()
		if err := tx.Exec(query, args...).Error; err != nil {
			t.Fatal(err)
		}
	}
	for index, name := range []string{"Alpha", "Bravo", "Charlie", "Delta", "Echo", "Foxtrot", "Golf", "Hotel", "India", "Juliet", "Kilo"} {
		number := index + 1
		exec(`INSERT INTO app.users(id,slug,display_name,email,mfa_configured,is_test)
			VALUES(?,?,?,?,?,?)`, queryFixtureID(number), "list-query-"+strings.ToLower(name), name, "secret-"+strings.ToLower(name)+"@example.test", number == 2, number == 8)
		if number != 10 {
			exec(`INSERT INTO app.user_auth_links(auth_subject,user_id,provider,provider_account_id)
				VALUES(?,?,'fixture',?)`, "list-query-"+name, queryFixtureID(number), "list-query-"+name)
		}
	}
	exec(`UPDATE app.users SET account_state='suspended',suspended_at=now() WHERE id=?`, queryFixtureID(7))
	for _, number := range []int{2, 7} {
		exec(`INSERT INTO app.global_role_assignments(user_id,role) VALUES(?,'director')`, queryFixtureID(number))
	}
	for _, number := range []int{101, 102, 103} {
		exec(`INSERT INTO app.seasons(id,slug,name,start_at,end_at)
			VALUES(?,?,?,'2026-01-01','2026-12-31')`, queryFixtureID(number), fmt.Sprintf("list-query-season-%d", number), "Query season")
	}
	exec(`UPDATE app.seasons SET status='closed',closed_at=now() WHERE id=?`, queryFixtureID(103))
	for number := 1; number <= 8; number++ {
		role, level, state := "student", "beginner", "active"
		if number == 2 || number == 5 {
			role, level = "mentor", "not_applicable"
		}
		if number == 4 || number == 5 {
			state = "completed"
		} else if number == 6 {
			state = "kicked"
		}
		exec(`INSERT INTO app.enrollments(id,user_id,season_id,role,student_level,state,assignment_state,activated_at)
			VALUES(?,?,?,?,?,?::app.enrollment_state,
				CASE WHEN ?='active' THEN 'active'::app.assignment_state ELSE 'revoked'::app.assignment_state END,
				CASE WHEN ?='active' THEN now() ELSE NULL END)`, queryFixtureID(200+number), queryFixtureID(number), queryFixtureID(101), role, level, state, state, state)
	}
	for index, number := range []int{2, 1, 3} {
		exec(`INSERT INTO app.season_weeks(id,season_id,week_number,start_at,end_at)
			VALUES(?,?,?,'2026-02-01','2026-02-07')`, queryFixtureID(301+index), queryFixtureID(101), number)
	}
	for index, student := range []int{3, 1} {
		exec(`INSERT INTO app.mentorships(id,season_id,mentor_enrollment_id,student_enrollment_id)
			VALUES(?,?,?,?)`, queryFixtureID(401+index), queryFixtureID(101), queryFixtureID(202), queryFixtureID(200+student))
	}
	exec(`INSERT INTO app.leetcode_problem_categories(id,name,normalized_name) VALUES(?,'Array','array')`, queryFixtureID(801))
	for index := 1; index <= 3; index++ {
		difficulty := "easy"
		if index == 3 {
			difficulty = "hard"
		}
		exec(`INSERT INTO app.problems(id,title) VALUES(?,?)`, queryFixtureID(500+index), fmt.Sprintf("Query problem %d", index))
		exec(`INSERT INTO app.leetcode_problems(id,problem_id,leetcode_number,difficulty,is_premium)
			VALUES(?,?,?,?,?)`, queryFixtureID(600+index), queryFixtureID(500+index), 99000+index, difficulty, index == 2)
		if index < 3 {
			exec(`INSERT INTO app.leetcode_problem_category_mappings(leetcode_problem_id,category_id) VALUES(?,?)`, queryFixtureID(600+index), queryFixtureID(801))
		}
		outcome := "independently_solved"
		if index == 2 {
			outcome = "not_solved"
		}
		exec(`INSERT INTO app.problem_attempts(id,user_id,problem_id,attempted_at,time_taken_minutes,outcome)
			VALUES(?,?,?,now(),20,?)`, queryFixtureID(700+index), queryFixtureID(1), queryFixtureID(500+index), outcome)
	}
	exec(`INSERT INTO app.problem_attempts(id,user_id,problem_id,attempted_at,time_taken_minutes,outcome)
		VALUES(?,?,?,now(),20,'not_solved')`, queryFixtureID(704), queryFixtureID(2), queryFixtureID(501))
	return db
}
