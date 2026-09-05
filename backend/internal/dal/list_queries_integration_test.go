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

	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/testcontainers/testcontainers-go"
	postgrescontainer "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/magedmg/RSP-website/backend/internal/accounts"
	"github.com/magedmg/RSP-website/backend/internal/practice"
	"github.com/magedmg/RSP-website/backend/internal/programme"
)

func TestStoreListQueries(t *testing.T) {
	db := newListQueryFixture(t)
	ctx := context.Background()

	t.Run("directory filters and cursor directions", func(t *testing.T) {
		id := func(user accounts.User) string { return user.ID }
		expectListPage(t, []int{1, 2}, true, 4, id)(db.ListUsers(ctx, UserQuery{Limit: 2, Direction: "forward"}))
		expectListPage(t, []int{3, 4}, false, 4, id)(db.ListUsers(ctx, UserQuery{Boundary: queryFixtureID(2), Limit: 2, Direction: "forward"}))
		expectListPage(t, []int{2, 3}, true, 4, id)(db.ListUsers(ctx, UserQuery{Boundary: queryFixtureID(4), Limit: 2, Direction: "backward"}))
		expectListPage(t, []int{4}, false, 1, id)(db.ListUsers(ctx, UserQuery{Limit: 2, Direction: "forward", Search: "  dELTa  ", SeasonRole: "student"}))
		expectListPage(t, []int{2}, false, 1, id)(db.ListUsers(ctx, UserQuery{Limit: 2, Direction: "forward", SeasonRole: "mentor", GlobalRole: "director"}))
		expectListPage(t, nil, false, 0, id)(db.ListUsers(ctx, UserQuery{Limit: 2, Direction: "forward", SeasonRole: "student", GlobalRole: "director"}))
	})

	t.Run("admin filters retain pending roles and private email search", func(t *testing.T) {
		id := func(user accounts.User) string { return user.ID }
		expectListPage(t, []int{7}, false, 1, id)(db.ListAdminUsers(ctx, AdminUserQuery{Limit: 2, Direction: "forward", AccountState: "suspended", GlobalRole: "director"}))
		expectListPage(t, []int{7}, false, 1, id)(db.ListAdminUsers(ctx, AdminUserQuery{Limit: 2, Direction: "backward", Search: " secret-golf@ "}))
		expectListPage(t, []int{2}, true, 2, id)(db.ListAdminUsers(ctx, AdminUserQuery{Limit: 1, Direction: "forward", GlobalRole: "director"}))
		expectListPage(t, []int{7}, false, 2, id)(db.ListAdminUsers(ctx, AdminUserQuery{Boundary: queryFixtureID(2), Limit: 1, Direction: "forward", GlobalRole: "director"}))
		expectListPage(t, []int{2}, false, 2, id)(db.ListAdminUsers(ctx, AdminUserQuery{Boundary: queryFixtureID(7), Limit: 1, Direction: "backward", GlobalRole: "director"}))
	})

	t.Run("candidates exclude existing enrollments and unverified users", func(t *testing.T) {
		id := func(user accounts.EnrollmentCandidate) string { return user.ID }
		expectListPage(t, []int{9}, true, 2, id)(db.ListEnrollmentCandidates(ctx, EnrollmentCandidateQuery{SeasonID: queryFixtureID(101), Limit: 1, Direction: "forward"}))
		expectListPage(t, []int{11}, false, 2, id)(db.ListEnrollmentCandidates(ctx, EnrollmentCandidateQuery{SeasonID: queryFixtureID(101), Boundary: queryFixtureID(9), Limit: 1, Direction: "forward"}))
		expectListPage(t, []int{9}, false, 2, id)(db.ListEnrollmentCandidates(ctx, EnrollmentCandidateQuery{SeasonID: queryFixtureID(101), Boundary: queryFixtureID(11), Limit: 1, Direction: "backward"}))
		expectListPage(t, []int{11}, false, 1, id)(db.ListEnrollmentCandidates(ctx, EnrollmentCandidateQuery{SeasonID: queryFixtureID(101), Search: " KILO ", Limit: 2, Direction: "forward"}))
	})

	t.Run("season status and backwards order", func(t *testing.T) {
		id := func(season programme.SeasonRecord) string { return season.ID }
		expectListPage(t, []int{101, 102}, true, 3, id)(db.ListSeasons(ctx, SeasonQuery{Limit: 2, Direction: "forward"}))
		expectListPage(t, []int{101, 102}, false, 3, id)(db.ListSeasons(ctx, SeasonQuery{Boundary: queryFixtureID(103), Limit: 2, Direction: "backward"}))
		expectListPage(t, []int{103}, false, 1, id)(db.ListSeasons(ctx, SeasonQuery{Limit: 2, Direction: "forward", Status: "closed"}))
	})

	t.Run("weeks sort by number rather than id", func(t *testing.T) {
		id := func(week programme.WeekRecord) string { return week.ID }
		expectListPage(t, []int{302, 301}, true, 3, id)(db.ListWeeks(ctx, WeekQuery{SeasonID: queryFixtureID(101), Limit: 2, SortBy: "number:asc", Direction: "forward"}))
		expectListPage(t, []int{303}, false, 3, id)(db.ListWeeks(ctx, WeekQuery{SeasonID: queryFixtureID(101), Boundary: queryFixtureID(301), Limit: 2, SortBy: "number:asc", Direction: "forward"}))
		expectListPage(t, []int{302, 301}, false, 3, id)(db.ListWeeks(ctx, WeekQuery{SeasonID: queryFixtureID(101), Boundary: queryFixtureID(303), Limit: 2, SortBy: "number:asc", Direction: "backward"}))
		expectListPage(t, []int{301, 302}, true, 3, id)(db.ListWeeks(ctx, WeekQuery{SeasonID: queryFixtureID(101), Limit: 2, SortBy: "id:asc", Direction: "forward"}))
	})

	t.Run("enrollment state filters and role ordering", func(t *testing.T) {
		id := func(enrollment programme.EnrollmentRecord) string { return enrollment.ID }
		expectListPage(t, []int{202, 201}, true, 6, id)(db.ListEnrollments(ctx, EnrollmentQuery{SeasonID: queryFixtureID(101), Limit: 2, SortBy: "role:asc", Direction: "forward"}))
		expectListPage(t, []int{202, 201}, false, 6, id)(db.ListEnrollments(ctx, EnrollmentQuery{SeasonID: queryFixtureID(101), Boundary: queryFixtureID(203), Limit: 2, SortBy: "role:asc", Direction: "backward"}))
		expectListPage(t, []int{204}, false, 1, id)(db.ListEnrollments(ctx, EnrollmentQuery{SeasonID: queryFixtureID(101), Limit: 10, Role: "student", State: "completed", SortBy: "role:asc", Direction: "forward"}))
		expectListPage(t, []int{205}, false, 1, id)(db.ListEnrollments(ctx, EnrollmentQuery{SeasonID: queryFixtureID(101), Limit: 10, Role: "mentor", State: "completed", SortBy: "role:asc", Direction: "forward", IncludeInactive: true}))
		expectListPage(t, nil, false, 0, id)(db.ListEnrollments(ctx, EnrollmentQuery{SeasonID: queryFixtureID(101), Limit: 10, Role: "mentor", State: "completed", SortBy: "role:asc", Direction: "forward"}))
		expectListPage(t, []int{206}, false, 1, id)(db.ListEnrollments(ctx, EnrollmentQuery{SeasonID: queryFixtureID(101), Limit: 10, State: "kicked", SortBy: "id:asc", Direction: "forward", IncludeInactive: true}))
	})

	t.Run("mentorship filters share count and page bindings", func(t *testing.T) {
		id := func(mentorship programme.MentorshipRecord) string { return mentorship.ID }
		expectListPage(t, []int{402}, true, 2, id)(db.ListMentorships(ctx, MentorshipQuery{SeasonID: queryFixtureID(101), Limit: 1, SortBy: "student:asc", Direction: "forward"}))
		expectListPage(t, []int{401}, false, 2, id)(db.ListMentorships(ctx, MentorshipQuery{SeasonID: queryFixtureID(101), Boundary: queryFixtureID(402), Limit: 1, SortBy: "student:asc", Direction: "forward", MentorUserID: queryFixtureID(2)}))
		expectListPage(t, []int{402}, false, 2, id)(db.ListMentorships(ctx, MentorshipQuery{SeasonID: queryFixtureID(101), Boundary: queryFixtureID(401), Limit: 1, SortBy: "student:asc", Direction: "backward"}))
		expectListPage(t, []int{401}, false, 1, id)(db.ListMentorships(ctx, MentorshipQuery{SeasonID: queryFixtureID(101), Limit: 2, SortBy: "id:asc", Direction: "forward", MentorUserID: queryFixtureID(2), StudentUserID: queryFixtureID(3)}))
		expectListPage(t, nil, false, 0, id)(db.ListMentorships(ctx, MentorshipQuery{SeasonID: queryFixtureID(101), Limit: 2, SortBy: "id:asc", Direction: "forward", MentorUserID: queryFixtureID(1)}))
	})

	t.Run("problem category difficulty and nullable premium", func(t *testing.T) {
		id := func(problem practice.ProblemRecord) string { return problem.ID }
		premium, free := true, false
		expectListPage(t, []int{601, 602}, true, 3, id)(db.ListProblems(ctx, ProblemQuery{Limit: 2, Direction: "forward"}))
		expectListPage(t, []int{601, 602}, false, 3, id)(db.ListProblems(ctx, ProblemQuery{Boundary: queryFixtureID(603), Limit: 2, Direction: "backward"}))
		expectListPage(t, []int{603}, false, 3, id)(db.ListProblems(ctx, ProblemQuery{Boundary: queryFixtureID(602), Limit: 2, Direction: "forward"}))
		expectListPage(t, []int{601}, false, 1, id)(db.ListProblems(ctx, ProblemQuery{Limit: 2, Difficulty: "easy", Category: "ArRaY", Premium: &free, Direction: "forward"}))
		expectListPage(t, []int{602}, false, 1, id)(db.ListProblems(ctx, ProblemQuery{Limit: 2, Difficulty: "easy", Category: "array", Premium: &premium, Direction: "forward"}))
		expectListPage(t, nil, false, 0, id)(db.ListProblems(ctx, ProblemQuery{Limit: 2, Difficulty: "hard", Category: "array", Direction: "forward"}))
	})

	t.Run("attempt user outcome difficulty and backwards pages", func(t *testing.T) {
		id := func(attempt practice.AttemptRecord) string { return attempt.ID }
		expectListPage(t, []int{701, 702}, true, 3, id)(db.ListAttempts(ctx, AttemptQuery{UserID: queryFixtureID(1), Limit: 2, Direction: "forward"}))
		expectListPage(t, []int{701, 702}, false, 3, id)(db.ListAttempts(ctx, AttemptQuery{UserID: queryFixtureID(1), Boundary: queryFixtureID(703), Limit: 2, Direction: "backward"}))
		expectListPage(t, []int{703}, false, 3, id)(db.ListAttempts(ctx, AttemptQuery{UserID: queryFixtureID(1), Boundary: queryFixtureID(702), Limit: 2, Direction: "forward"}))
		expectListPage(t, []int{702}, false, 1, id)(db.ListAttempts(ctx, AttemptQuery{UserID: queryFixtureID(1), Limit: 2, Outcome: "not_solved", Difficulty: "easy", Direction: "forward"}))
		expectListPage(t, nil, false, 0, id)(db.ListAttempts(ctx, AttemptQuery{UserID: queryFixtureID(1), Limit: 2, Outcome: "not_solved", Difficulty: "hard", Direction: "forward"}))
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

func newPostgresFixture(t *testing.T) *Store {
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
	t.Cleanup(db.Close)
	if err := goose.SetDialect("postgres"); err != nil {
		t.Fatal(err)
	}
	migrations, err := filepath.Abs("../../../db/migrations")
	if err != nil {
		t.Fatal(err)
	}
	migrationDB := stdlib.OpenDBFromPool(db.pool)
	defer migrationDB.Close()
	if err := goose.UpContext(ctx, migrationDB, migrations); err != nil {
		t.Fatal(err)
	}
	// The container is exclusive to this test; committed fixture rows are isolated.
	db.Close()
	db, err = Open(ctx, databaseURL+"&options=-c%20search_path%3Dapp")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(db.Close)
	return db
}

// A separate database keeps unfiltered count assertions independent of HTTP
// integration fixtures, which truncate their shared TEST_DATABASE_URL tables.
func newListQueryFixture(t *testing.T) *Store {
	t.Helper()
	db := newPostgresFixture(t)
	tx := db.pool
	exec := func(query string, args ...any) {
		t.Helper()
		if _, err := tx.Exec(context.Background(), query, args...); err != nil {
			t.Fatal(err)
		}
	}
	for index, name := range []string{"Alpha", "Bravo", "Charlie", "Delta", "Echo", "Foxtrot", "Golf", "Hotel", "India", "Juliet", "Kilo"} {
		number := index + 1
		exec(`INSERT INTO app.users(id,slug,display_name,email,mfa_configured,is_test)
			VALUES($1,$2,$3,$4,$5,$6)`, queryFixtureID(number), "list-query-"+strings.ToLower(name), name, "secret-"+strings.ToLower(name)+"@example.test", number == 2, number == 8)
		if number != 10 {
			exec(`INSERT INTO app.user_auth_links(auth_subject,user_id,provider,provider_account_id)
				VALUES($1,$2,'fixture',$3)`, "list-query-"+name, queryFixtureID(number), "list-query-"+name)
		}
	}
	exec(`UPDATE app.users SET account_state='suspended',suspended_at=now() WHERE id=$1`, queryFixtureID(7))
	for _, number := range []int{2, 7} {
		exec(`INSERT INTO app.global_role_assignments(user_id,role) VALUES($1,'director')`, queryFixtureID(number))
	}
	for _, number := range []int{101, 102, 103} {
		exec(`INSERT INTO app.seasons(id,slug,name,start_at,end_at)
			VALUES($1,$2,$3,'2026-01-01','2026-12-31')`, queryFixtureID(number), fmt.Sprintf("list-query-season-%d", number), "Query season")
	}
	exec(`UPDATE app.seasons SET status='closed',closed_at=now() WHERE id=$1`, queryFixtureID(103))
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
			VALUES($1,$2,$3,$4,$5,$6::app.enrollment_state,
				CASE WHEN $7='active' THEN 'active'::app.assignment_state ELSE 'revoked'::app.assignment_state END,
				CASE WHEN $8='active' THEN now() ELSE NULL END)`, queryFixtureID(200+number), queryFixtureID(number), queryFixtureID(101), role, level, state, state, state)
	}
	for index, number := range []int{2, 1, 3} {
		exec(`INSERT INTO app.season_weeks(id,season_id,week_number,start_at,end_at)
			VALUES($1,$2,$3,'2026-02-01','2026-02-07')`, queryFixtureID(301+index), queryFixtureID(101), number)
	}
	for index, student := range []int{3, 1} {
		exec(`INSERT INTO app.mentorships(id,season_id,mentor_enrollment_id,student_enrollment_id)
			VALUES($1,$2,$3,$4)`, queryFixtureID(401+index), queryFixtureID(101), queryFixtureID(202), queryFixtureID(200+student))
	}
	exec(`INSERT INTO app.leetcode_problem_categories(id,name,normalized_name) VALUES($1,'Array','array')`, queryFixtureID(801))
	for index := 1; index <= 3; index++ {
		difficulty := "easy"
		if index == 3 {
			difficulty = "hard"
		}
		exec(`INSERT INTO app.problems(id,title) VALUES($1,$2)`, queryFixtureID(500+index), fmt.Sprintf("Query problem %d", index))
		exec(`INSERT INTO app.leetcode_problems(id,problem_id,leetcode_number,difficulty,is_premium)
			VALUES($1,$2,$3,$4,$5)`, queryFixtureID(600+index), queryFixtureID(500+index), 99000+index, difficulty, index == 2)
		if index < 3 {
			exec(`INSERT INTO app.leetcode_problem_category_mappings(leetcode_problem_id,category_id) VALUES($1,$2)`, queryFixtureID(600+index), queryFixtureID(801))
		}
		outcome := "independently_solved"
		if index == 2 {
			outcome = "not_solved"
		}
		exec(`INSERT INTO app.problem_attempts(id,user_id,problem_id,attempted_at,time_taken_minutes,outcome)
			VALUES($1,$2,$3,now(),20,$4)`, queryFixtureID(700+index), queryFixtureID(1), queryFixtureID(500+index), outcome)
	}
	exec(`INSERT INTO app.problem_attempts(id,user_id,problem_id,attempted_at,time_taken_minutes,outcome)
		VALUES($1,$2,$3,now(),20,'not_solved')`, queryFixtureID(704), queryFixtureID(2), queryFixtureID(501))
	return db
}
