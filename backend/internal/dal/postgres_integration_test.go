//go:build integration

package dal

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/testcontainers/testcontainers-go"
	postgrescontainer "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/magedmg/RSP-website/backend/internal/accounts"
	"github.com/magedmg/RSP-website/backend/internal/authz"
	"github.com/magedmg/RSP-website/backend/internal/leetcode"
	"github.com/magedmg/RSP-website/backend/internal/mockinterviews"
	"github.com/magedmg/RSP-website/backend/internal/platform/audit"
	"github.com/magedmg/RSP-website/backend/internal/practice"
	"github.com/magedmg/RSP-website/backend/internal/programme"
)

func TestPostgres18MigrationsAndDAL(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	container, err := postgrescontainer.Run(ctx, "postgres:18.6-alpine3.24",
		postgrescontainer.WithDatabase("rsp"),
		postgrescontainer.WithUsername("rsp"),
		postgrescontainer.WithPassword("rsp-00000000-0000-7000-8000-000000000022"),
		testcontainers.WithWaitStrategy(wait.ForLog("database system is ready to accept connections").WithOccurrence(2).WithStartupTimeout(90*time.Second)),
	)
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() { _ = testcontainers.TerminateContainer(container) })

	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}

	defer db.Close()
	if err := goose.SetDialect("postgres"); err != nil {
		t.Fatal(err)
	}

	migrations, err := filepath.Abs("../../../db/migrations")
	if err != nil {
		t.Fatal(err)
	}
	if err := goose.Up(db, migrations); err != nil {
		t.Fatal(err)
	}

	repository, err := Open(ctx, dsn+"&options=-c%20role%3Drsp_app%20-c%20search_path%3Dapp")
	if err != nil {
		t.Fatal(err)
	}

	defer repository.Close()
	createdAt := time.Now().UTC()
	if err := repository.ApplyIdentityEvent(ctx, accounts.IdentityEvent{EventID: "00000000-0000-7000-8000-000000000010", Type: "auth_user_created", AuthUserID: "auth-unverified", Email: "unverified@rsp.local", EmailVerified: false, SecurityVersion: 1, OccurredAt: createdAt}); err != nil {
		t.Fatalf("unverified auth user creation: %v", err)
	}
	if _, err := repository.ResolveAuthSubject(ctx, "auth-unverified"); err != ErrNotFound {
		t.Fatalf("unverified inactive link resolved: %v", err)
	}
	if err := repository.ApplyIdentityEvent(ctx, accounts.IdentityEvent{EventID: "00000000-0000-7000-8000-000000000006", Type: "email_verified", AuthUserID: "auth-unverified", SecurityVersion: 2, OccurredAt: createdAt.Add(time.Second)}); err != nil {
		t.Fatalf("email verification activation: %v", err)
	}
	if actor, err := repository.ResolveAuthSubject(ctx, "auth-unverified"); err != nil || !actor.EmailVerified || actor.SecurityVersion != 2 {
		t.Fatalf("verified auth link actor=%#v err=%v", actor, err)
	}

	_, err = repository.pool.Exec(context.Background(), `INSERT INTO app.users(id,account_state) VALUES('00000000-0000-7000-8000-000000000033','active')`)
	if err != nil {
		t.Fatal(err)
	}
	for _, query := range []string{
		`INSERT INTO app.user_profiles(user_id,slug,display_name) VALUES('00000000-0000-7000-8000-000000000033','00000000-0000-7000-8000-000000000033','Integration User')`,
		`INSERT INTO app.user_preferences(user_id) VALUES('00000000-0000-7000-8000-000000000033')`,
		`INSERT INTO app.user_contacts(user_id,email) VALUES('00000000-0000-7000-8000-000000000033','integration@rsp.local')`,
		`INSERT INTO app.user_security(user_id) VALUES('00000000-0000-7000-8000-000000000033')`,
	} {
		if _, err := repository.pool.Exec(context.Background(), query); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := repository.pool.Exec(context.Background(), `INSERT INTO app.user_auth_links(auth_subject,user_id,provider,provider_account_id) VALUES('auth-integration','00000000-0000-7000-8000-000000000033','better_auth','auth-integration')`); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.pool.Exec(context.Background(), `INSERT INTO app.global_role_assignments(id,user_id,role,state) VALUES('00000000-0000-7000-8000-000000000032','00000000-0000-7000-8000-000000000033','director','pending_mfa')`); err != nil {
		t.Fatal(err)
	}
	if err := repository.ApplyIdentityEvent(ctx, accounts.IdentityEvent{EventID: "00000000-0000-7000-8000-000000000014", Type: "mfa_configured", AuthUserID: "auth-integration", SecurityVersion: 2, OccurredAt: time.Now().UTC()}); err != nil {
		t.Fatal(err)
	}

	actor, err := repository.ResolveAuthSubject(ctx, "auth-integration")
	if err != nil || !actor.HasGlobalRole("director") || actor.SecurityVersion != 2 {
		t.Fatalf("MFA role activation: actor=%#v err=%v", actor, err)
	}

	var activatedAudits int
	if err := repository.pool.QueryRow(context.Background(), `SELECT count(*) FROM app.audit_events WHERE action='global_role.activated' AND subject_id='00000000-0000-7000-8000-000000000032'`).Scan(&activatedAudits); err != nil || activatedAudits != 1 {
		t.Fatalf("activation audit count=%d err=%v", activatedAudits, err)
	}
	if err := repository.QueueLeetCodeSync(ctx, actor.UserID, "integration-request"); err != nil {
		t.Fatalf("queue LeetCode sync: %v", err)
	}
	var storedRequestID string
	if err := repository.pool.QueryRow(context.Background(), `SELECT request_id FROM app.audit_events WHERE action='leetcode.sync_requested' ORDER BY occurred_at DESC LIMIT 1`).Scan(&storedRequestID); err != nil || storedRequestID != "integration-request" {
		t.Fatalf("queued sync request id=%q err=%v", storedRequestID, err)
	}
	t.Run("LeetCode worker manual and scheduled runs", func(t *testing.T) {
		testLeetCodeWorkerRuns(t, ctx, repository, actor.UserID)
	})

	t.Run("worker sessions exclude concurrent syncs", func(t *testing.T) {
		first, err := repository.OpenWorkerSession(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer first.Close()
		second, err := repository.OpenWorkerSession(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer second.Close()
		const lockKey = 7277500199
		if locked, err := first.TryLock(ctx, lockKey); err != nil || !locked {
			t.Fatalf("first lock=%v error=%v", locked, err)
		}
		defer first.Unlock(context.Background(), lockKey)
		if locked, err := second.TryLock(ctx, lockKey); err != nil || locked {
			t.Fatalf("concurrent lock=%v error=%v", locked, err)
		}
		if err := first.Unlock(ctx, lockKey); err != nil {
			t.Fatal(err)
		}
		if locked, err := second.TryLock(ctx, lockKey); err != nil || !locked {
			t.Fatalf("released lock=%v error=%v", locked, err)
		}
		defer second.Unlock(context.Background(), lockKey)
	})
	sink, err := repository.OpenWorkerSession(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer sink.Close()

	problem := leetcode.Problem{Number: 999, Title: "Integration Problem", Slug: "integration-problem", Difficulty: "medium", Categories: []string{"Graphs", "Graphs"}}
	if err := sink.Upsert(ctx, problem); err != nil {
		t.Fatalf("create LeetCode catalogue item: %v", err)
	}
	problem.Title = "Updated Integration Problem"
	problem.Categories = []string{"Dynamic Programming"}
	if err := sink.Upsert(ctx, problem); err != nil {
		t.Fatalf("update LeetCode catalogue item: %v", err)
	}
	var categoryCount int
	if err := repository.pool.QueryRow(context.Background(), `SELECT count(*) FROM app.leetcode_problem_category_mappings m JOIN app.leetcode_problems l ON l.id=m.leetcode_problem_id WHERE l.leetcode_number=999`).Scan(&categoryCount); err != nil || categoryCount != 1 {
		t.Fatalf("LeetCode category count=%d err=%v", categoryCount, err)
	}
	if _, err := repository.pool.Exec(context.Background(), `UPDATE app.problems p SET deleted_at=now() FROM app.leetcode_problems l WHERE l.problem_id=p.id AND l.leetcode_number=999`); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.pool.Exec(context.Background(), `UPDATE app.leetcode_problems SET deleted_at=now() WHERE leetcode_number=999`); err != nil {
		t.Fatal(err)
	}
	if snapshot, err := repository.ObservabilitySnapshot(ctx); err != nil {
		t.Fatalf("observability snapshot=%#v err=%v", snapshot, err)
	}

	deadline := time.Now().UTC().Add(30 * 24 * time.Hour)
	if err := repository.ApplyIdentityEvent(ctx, accounts.IdentityEvent{EventID: "00000000-0000-7000-8000-000000000008", Type: "deletion_requested", AuthUserID: "auth-integration", SecurityVersion: 3, OccurredAt: time.Now().UTC(), RecoveryDeadline: &deadline}); err != nil {
		t.Fatal(err)
	}
	if err := repository.ApplyIdentityEvent(ctx, accounts.IdentityEvent{EventID: "00000000-0000-7000-8000-000000000007", Type: "deletion_cancelled", AuthUserID: "auth-integration", SecurityVersion: 4, OccurredAt: time.Now().UTC()}); err != nil {
		t.Fatal(err)
	}
	if err := repository.ApplyIdentityEvent(ctx, accounts.IdentityEvent{EventID: "00000000-0000-7000-8000-000000000001", Type: "deletion_requested", AuthUserID: "auth-integration", SecurityVersion: 3, OccurredAt: time.Now().UTC(), RecoveryDeadline: &deadline}); err != nil {
		t.Fatal(err)
	}

	actor, err = repository.ResolveAuthSubject(ctx, "auth-integration")
	if err != nil || actor.AccountState != "active" || actor.SecurityVersion != 4 {
		t.Fatalf("stale lifecycle event applied: actor=%#v err=%v", actor, err)
	}

	season, err := repository.CreateSeason(ctx, programme.SeasonRecord{ID: "00000000-0000-7000-8000-000000000028", Slug: "00000000-0000-7000-8000-000000000028", Name: "Integration Season", Status: "open", StartAt: time.Now().UTC(), EndAt: time.Now().UTC().Add(24 * time.Hour)}, "00000000-0000-7000-8000-000000000033", time.Now())
	if err != nil {
		t.Fatalf("create season: %#v %v", season, err)
	}

	settings, err := repository.GetPracticeSettings(ctx, "00000000-0000-7000-8000-000000000033")
	if err != nil || settings.GoalsEnabled || settings.EasyMinutes != 10 || settings.MediumMinutes != 20 || settings.HardMinutes != 45 {
		t.Fatalf("default practice settings=%#v err=%v", settings, err)
	}

	settings, err = repository.EnablePracticeGoals(ctx, "00000000-0000-7000-8000-000000000033", "00000000-0000-7000-8000-000000000033", season.ID, time.Now().UTC())
	if err != nil || !settings.GoalsEnabled {
		t.Fatalf("enable practice goals=%#v err=%v", settings, err)
	}

	settings, err = repository.UpdatePracticeSettings(ctx, "00000000-0000-7000-8000-000000000033", 25, 40, 60, "00000000-0000-7000-8000-000000000033", time.Now().UTC())
	if err != nil || settings.HardMinutes != 60 {
		t.Fatalf("update practice settings=%#v err=%v", settings, err)
	}
	if _, err := repository.pool.Exec(context.Background(), `INSERT INTO app.users(id,account_state) VALUES
		('00000000-0000-7000-8000-000000000027','active'),
		('00000000-0000-7000-8000-000000000017','active'),
		('00000000-0000-7000-8000-000000000023','active'),
		('00000000-0000-7000-8000-000000000026','active')`); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.pool.Exec(context.Background(), `INSERT INTO app.user_profiles(user_id,slug,display_name) VALUES
		('00000000-0000-7000-8000-000000000027','00000000-0000-7000-8000-000000000027','Integration Mentor'),
		('00000000-0000-7000-8000-000000000017','00000000-0000-7000-8000-000000000017','Integration Coordinator'),
		('00000000-0000-7000-8000-000000000023','00000000-0000-7000-8000-000000000023','Integration Promoted'),
		('00000000-0000-7000-8000-000000000026','00000000-0000-7000-8000-000000000026','Integration Kicked')`); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.pool.Exec(context.Background(), `INSERT INTO app.user_preferences(user_id) VALUES
		('00000000-0000-7000-8000-000000000027'),('00000000-0000-7000-8000-000000000017'),
		('00000000-0000-7000-8000-000000000023'),('00000000-0000-7000-8000-000000000026')`); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.pool.Exec(context.Background(), `INSERT INTO app.user_contacts(user_id,email) VALUES
		('00000000-0000-7000-8000-000000000027','mentor@rsp.local'),
		('00000000-0000-7000-8000-000000000017','coordinator@rsp.local'),
		('00000000-0000-7000-8000-000000000023','promoted@rsp.local'),
		('00000000-0000-7000-8000-000000000026','kicked@rsp.local')`); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.pool.Exec(context.Background(), `INSERT INTO app.user_security(user_id,mfa_configured) VALUES
		('00000000-0000-7000-8000-000000000027',false),('00000000-0000-7000-8000-000000000017',true),
		('00000000-0000-7000-8000-000000000023',true),('00000000-0000-7000-8000-000000000026',false)`); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.pool.Exec(context.Background(), `INSERT INTO app.global_role_assignments(id,user_id,role,state,activated_at) VALUES('00000000-0000-7000-8000-000000000018','00000000-0000-7000-8000-000000000026','director','active',now())`); err != nil {
		t.Fatal(err)
	}

	var assignmentState string
	var assignmentActivatedAt *time.Time
	if err := repository.pool.QueryRow(context.Background(), `SELECT state::text,activated_at FROM app.global_role_assignments WHERE id='00000000-0000-7000-8000-000000000018'`).Scan(&assignmentState, &assignmentActivatedAt); err != nil || assignmentState != "pending_mfa" || assignmentActivatedAt != nil {
		t.Fatalf("non-MFA global role state=%q activatedAt=%v err=%v", assignmentState, assignmentActivatedAt, err)
	}
	if _, err := repository.pool.Exec(context.Background(), `INSERT INTO app.global_role_assignments(id,user_id,role,state) VALUES('00000000-0000-7000-8000-000000000004','00000000-0000-7000-8000-000000000017','system_admin','pending_mfa')`); err != nil {
		t.Fatal(err)
	}
	if err := repository.pool.QueryRow(context.Background(), `SELECT state::text,activated_at FROM app.global_role_assignments WHERE id='00000000-0000-7000-8000-000000000004'`).Scan(&assignmentState, &assignmentActivatedAt); err != nil || assignmentState != "active" || assignmentActivatedAt == nil {
		t.Fatalf("preconfigured-MFA global role state=%q activatedAt=%v err=%v", assignmentState, assignmentActivatedAt, err)
	}
	if _, err := repository.CreateEnrollment(ctx, programme.EnrollmentRecord{ID: "00000000-0000-7000-8000-000000000009", SeasonID: season.ID, UserID: "00000000-0000-7000-8000-000000000033", Role: "student", State: "active"}, "00000000-0000-7000-8000-000000000033", time.Now()); err != nil {
		t.Fatal(err)
	}
	if users, _, _, err := repository.ListUsers(ctx, UserQuery{Limit: 25, Direction: "forward"}); err != nil || len(users) == 0 {
		t.Fatalf("empty-cursor user list=%#v err=%v", users, err)
	}
	if users, _, _, err := repository.ListAdminUsers(ctx, AdminUserQuery{Limit: 25, Direction: "forward"}); err != nil || len(users) == 0 {
		t.Fatalf("empty-cursor admin user list=%#v err=%v", users, err)
	}
	if seasons, _, _, err := repository.ListSeasons(ctx, SeasonQuery{Limit: 25, Direction: "forward"}); err != nil || len(seasons) == 0 {
		t.Fatalf("empty-cursor season list=%#v err=%v", seasons, err)
	}
	if _, _, _, err := repository.ListEnrollmentCandidates(ctx, EnrollmentCandidateQuery{SeasonID: season.ID, Limit: 25, Direction: "forward"}); err != nil {
		t.Fatalf("empty-cursor enrollment candidate list: %v", err)
	}
	if _, err := repository.CreateEnrollment(ctx, programme.EnrollmentRecord{ID: "00000000-0000-7000-8000-000000000012", SeasonID: season.ID, UserID: "00000000-0000-7000-8000-000000000027", Role: "mentor", State: "active"}, "00000000-0000-7000-8000-000000000033", time.Now()); err != nil {
		t.Fatal(err)
	}

	coordinatorEnrollment, err := repository.CreateEnrollment(ctx, programme.EnrollmentRecord{ID: "00000000-0000-7000-8000-000000000002", SeasonID: season.ID, UserID: "00000000-0000-7000-8000-000000000017", Role: "coordinator", State: "active"}, "00000000-0000-7000-8000-000000000033", time.Now())
	if err != nil || coordinatorEnrollment.AssignmentState != "active" {
		t.Fatalf("preconfigured coordinator create=%#v err=%v", coordinatorEnrollment, err)
	}
	if _, err := repository.CreateEnrollment(ctx, programme.EnrollmentRecord{ID: "00000000-0000-7000-8000-000000000005", SeasonID: season.ID, UserID: "00000000-0000-7000-8000-000000000023", Role: "student", State: "active"}, "00000000-0000-7000-8000-000000000033", time.Now()); err != nil {
		t.Fatal(err)
	}

	promotedEnrollment, err := repository.UpdateEnrollment(ctx, "00000000-0000-7000-8000-000000000005", "coordinator", "", "", "00000000-0000-7000-8000-000000000033", time.Now())
	if err != nil || promotedEnrollment.AssignmentState != "active" {
		t.Fatalf("preconfigured coordinator promotion=%#v err=%v", promotedEnrollment, err)
	}
	if _, err := repository.pool.Exec(context.Background(), `INSERT INTO app.enrollments(id,user_id,season_id,role,student_level,state,assignment_state,activated_at) VALUES('00000000-0000-7000-8000-000000000011','00000000-0000-7000-8000-000000000026',$1,'student','beginner','kicked','revoked',NULL)`, season.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.CreateWeek(ctx, programme.WeekRecord{ID: "00000000-0000-7000-8000-000000000034", SeasonID: season.ID, Number: 1, StartAt: season.StartAt, EndAt: season.EndAt, ResourceURL: "https://rsp.local/week"}, "00000000-0000-7000-8000-000000000033", time.Now()); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.CreateMentorship(ctx, programme.MentorshipRecord{ID: "00000000-0000-7000-8000-000000000020", SeasonID: season.ID, MentorUserID: "00000000-0000-7000-8000-000000000027", StudentUserID: "00000000-0000-7000-8000-000000000033"}, "00000000-0000-7000-8000-000000000033", time.Now()); err != nil {
		t.Fatal(err)
	}
	if assigned, err := repository.IsMentorAssigned(ctx, season.ID, "00000000-0000-7000-8000-000000000027", "00000000-0000-7000-8000-000000000033"); err != nil || !assigned {
		t.Fatalf("mentorship assigned=%v err=%v", assigned, err)
	}

	closed, err := repository.CloseSeason(ctx, season.ID, "00000000-0000-7000-8000-000000000033", "complete", time.Now().UTC())
	if err != nil || closed.Status != "closed" {
		t.Fatalf("close=%#v err=%v", closed, err)
	}

	studentEnrollment, err := repository.GetEnrollment(ctx, "00000000-0000-7000-8000-000000000009")
	if err != nil || studentEnrollment.State != "completed" {
		t.Fatalf("completed enrollment=%#v err=%v", studentEnrollment, err)
	}

	reopened, err := repository.ReopenSeason(ctx, season.ID, "00000000-0000-7000-8000-000000000033", "correction", time.Now().UTC())
	if err != nil || reopened.Status != "open" {
		t.Fatalf("reopen=%#v err=%v", reopened, err)
	}

	studentEnrollment, err = repository.GetEnrollment(ctx, "00000000-0000-7000-8000-000000000009")
	if err != nil || studentEnrollment.State != "active" {
		t.Fatalf("restored enrollment=%#v err=%v", studentEnrollment, err)
	}

	coordinatorEnrollment, err = repository.GetEnrollment(ctx, "00000000-0000-7000-8000-000000000002")
	if err != nil || coordinatorEnrollment.State != "active" || coordinatorEnrollment.AssignmentState != "active" {
		t.Fatalf("restored coordinator enrollment=%#v err=%v", coordinatorEnrollment, err)
	}

	invalidScore := 7
	invalidMock := mockinterviews.Interview{ID: "00000000-0000-7000-8000-000000000003", InterviewerID: "00000000-0000-7000-8000-000000000027", IntervieweeID: "00000000-0000-7000-8000-000000000026", SeasonID: &season.ID, OccurredAt: season.StartAt.Add(time.Hour), DurationMinutes: 60, Rounds: []mockinterviews.Round{{ID: "00000000-0000-7000-8000-000000000016", Type: mockinterviews.Behavioural, Scores: mockinterviews.Scores{Behavioural: &invalidScore}}}}
	if _, err := repository.CreateMockInterview(ctx, invalidMock, "00000000-0000-7000-8000-000000000027", time.Now().UTC()); err == nil {
		t.Fatal("kicked-only mock participant passed PostgreSQL scope validation")
	}
	if _, err := repository.pool.Exec(context.Background(), `INSERT INTO app.problems(id,title,url) VALUES('00000000-0000-7000-8000-000000000019','Two Sum','https://leetcode.com/problems/two-sum/')`); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.pool.Exec(context.Background(), `INSERT INTO app.leetcode_problems(id,problem_id,leetcode_number,difficulty,is_premium) VALUES('00000000-0000-7000-8000-000000000025','00000000-0000-7000-8000-000000000019',1,'easy',false)`); err != nil {
		t.Fatal(err)
	}
	if problems, _, _, err := repository.ListProblems(ctx, ProblemQuery{Limit: 25, Direction: "forward"}); err != nil || len(problems) != 1 {
		t.Fatalf("empty-cursor problem list=%#v err=%v", problems, err)
	}

	_, err = repository.CreateAttempt(ctx, practice.AttemptRecord{ID: "00000000-0000-7000-8000-000000000024", UserID: "00000000-0000-7000-8000-000000000033", ProblemID: "00000000-0000-7000-8000-000000000025", Outcome: "independently_solved", Confidence: intPointer(5), Minutes: 12, AttemptedAt: time.Now().UTC()})
	if err != nil {
		t.Fatal(err)
	}
	if attempts, _, _, err := repository.ListAttempts(ctx, AttemptQuery{UserID: "00000000-0000-7000-8000-000000000033", Limit: 25, Direction: "forward"}); err != nil || len(attempts) != 1 {
		t.Fatalf("empty-cursor attempt list=%#v err=%v", attempts, err)
	}

	score := 7
	interview := mockinterviews.Interview{ID: "00000000-0000-7000-8000-000000000031", InterviewerID: "00000000-0000-7000-8000-000000000027", IntervieweeID: "00000000-0000-7000-8000-000000000033", SeasonID: &season.ID, OccurredAt: time.Now().UTC(), DurationMinutes: 60, Notes: "notes", Rounds: []mockinterviews.Round{{ID: "00000000-0000-7000-8000-000000000030", Type: mockinterviews.Behavioural, Scores: mockinterviews.Scores{Behavioural: &score}}}}
	if _, err := repository.CreateMockInterview(ctx, interview, "00000000-0000-7000-8000-000000000027", time.Now().UTC()); err != nil {
		t.Fatal(err)
	}

	loadedInterview, err := repository.GetMockInterview(ctx, interview.ID)
	if err != nil || len(loadedInterview.Rounds) != 1 {
		t.Fatalf("mock=%#v err=%v", loadedInterview, err)
	}
	if interviews, _, _, err := repository.ListMockInterviews(ctx, authz.Actor{UserID: "00000000-0000-7000-8000-000000000027", Enrollments: []authz.Enrollment{{SeasonID: season.ID, Role: authz.Mentor, State: authz.Active}}}, MockInterviewQuery{Mode: "all", Limit: 25, SortBy: "id:asc", Direction: "forward"}); err != nil || len(interviews) != 1 {
		t.Fatalf("relationship-scoped mock list=%#v err=%v", interviews, err)
	}

	loadedInterview.Notes = "updated"
	if _, err := repository.UpdateMockInterview(ctx, loadedInterview, "00000000-0000-7000-8000-000000000027", time.Now().UTC()); err != nil {
		t.Fatal(err)
	}

	var versionCount int
	if err := repository.pool.QueryRow(context.Background(), `SELECT count(*) FROM app.mock_interview_versions WHERE mock_interview_id='00000000-0000-7000-8000-000000000031'`).Scan(&versionCount); err != nil || versionCount != 2 {
		t.Fatalf("mock versions=%d err=%v", versionCount, err)
	}

	updatedSeason, err := repository.UpdateSeason(ctx, season.ID, func(value *programme.SeasonRecord) error {
		value.Location = "Updated Adelaide"
		return nil
	}, "00000000-0000-7000-8000-000000000033", time.Now().UTC())
	if err != nil || updatedSeason.Location != "Updated Adelaide" {
		t.Fatalf("enum-backed season update=%#v err=%v", updatedSeason, err)
	}

	auditEvent := audit.Event{ID: "00000000-0000-7000-8000-000000000029", Action: "integration.test", SubjectType: "season", SubjectID: season.ID, Data: map[string]any{}, OccurredAt: time.Now().UTC()}
	if err := repository.AppendAudit(ctx, auditEvent); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.pool.Exec(context.Background(), `UPDATE app.audit_events SET action='tampered' WHERE id='00000000-0000-7000-8000-000000000029'`); err == nil {
		t.Fatal("append-only audit update unexpectedly succeeded")
	}
	if err := goose.DownTo(db, migrations, 0); err != nil {
		t.Fatal(err)
	}
	if err := goose.Up(db, migrations); err != nil {
		t.Fatal(err)
	}
}

func intPointer(v int) *int { return &v }
