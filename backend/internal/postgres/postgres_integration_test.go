//go:build integration

package postgres

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/magedmg/RSP-website/backend/internal/accounts"
	"github.com/magedmg/RSP-website/backend/internal/authz"
	"github.com/magedmg/RSP-website/backend/internal/mockinterviews"
	"github.com/magedmg/RSP-website/backend/internal/model"
	"github.com/magedmg/RSP-website/backend/internal/platform/audit"
	"github.com/magedmg/RSP-website/backend/internal/practice"
	"github.com/pressly/goose/v3"
	"github.com/testcontainers/testcontainers-go"
	postgrescontainer "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

func TestPostgres18MigrationsAndRepository(t *testing.T) {
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

	_, err = repository.Pool.Exec(ctx, `INSERT INTO app.users(id,slug,display_name,email,account_state,timezone,revision) VALUES('00000000-0000-7000-8000-000000000033','00000000-0000-7000-8000-000000000033','Integration User','integration@rsp.local','active','Australia/Adelaide',1)`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool.Exec(ctx, `INSERT INTO app.user_auth_links(auth_subject,user_id,provider,provider_account_id) VALUES('auth-integration','00000000-0000-7000-8000-000000000033','better_auth','auth-integration')`); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool.Exec(ctx, `INSERT INTO app.global_role_assignments(id,user_id,role,state) VALUES('00000000-0000-7000-8000-000000000032','00000000-0000-7000-8000-000000000033','director','pending_mfa')`); err != nil {
		t.Fatal(err)
	}
	if err := repository.ApplyIdentityEvent(ctx, accounts.IdentityEvent{EventID: "00000000-0000-7000-8000-000000000014", Type: "mfa_configured", AuthUserID: "auth-integration", SecurityVersion: 2, OccurredAt: time.Now().UTC()}); err != nil {
		t.Fatal(err)
	}

	actor, err := repository.ResolveAuthSubject(ctx, "auth-integration")
	if err != nil || !actor.IsGlobal("director") || actor.SecurityVersion != 2 {
		t.Fatalf("MFA role activation: actor=%#v err=%v", actor, err)
	}

	var activatedAudits int
	if err := repository.Pool.QueryRow(ctx, `SELECT count(*) FROM app.audit_events WHERE action='global_role.activated' AND subject_id='00000000-0000-7000-8000-000000000032'`).Scan(&activatedAudits); err != nil || activatedAudits != 1 {
		t.Fatalf("activation audit count=%d err=%v", activatedAudits, err)
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

	season, err := repository.CreateSeason(ctx, model.Season{ID: "00000000-0000-7000-8000-000000000028", Slug: "00000000-0000-7000-8000-000000000028", Name: "Integration Season", Status: "open", StartAt: time.Now().UTC(), EndAt: time.Now().UTC().Add(24 * time.Hour), Revision: 1}, "00000000-0000-7000-8000-000000000033", time.Now())
	if err != nil || season.Revision != 1 {
		t.Fatalf("create season: %#v %v", season, err)
	}
	if _, err := repository.UpdateSeason(ctx, season.ID, 99, func(*model.Season) error { return nil }, "00000000-0000-7000-8000-000000000033", time.Now()); err != ErrConflict {
		t.Fatalf("stale revision returned %v", err)
	}

	settings, err := repository.GetPracticeSettings(ctx, "00000000-0000-7000-8000-000000000033")
	if err != nil || settings.GoalsEnabled || settings.EasyMinutes != 20 || settings.Revision != 1 {
		t.Fatalf("default practice settings=%#v err=%v", settings, err)
	}
	if _, err := repository.EnablePracticeGoals(ctx, "00000000-0000-7000-8000-000000000033", 99, "00000000-0000-7000-8000-000000000033", season.ID, time.Now().UTC()); err != ErrConflict {
		t.Fatalf("missing-row stale goal enable returned %v", err)
	}

	settings, err = repository.EnablePracticeGoals(ctx, "00000000-0000-7000-8000-000000000033", settings.Revision, "00000000-0000-7000-8000-000000000033", season.ID, time.Now().UTC())
	if err != nil || !settings.GoalsEnabled {
		t.Fatalf("enable practice goals=%#v err=%v", settings, err)
	}

	settings, err = repository.UpdatePracticeSettings(ctx, "00000000-0000-7000-8000-000000000033", settings.Revision, true, 25, 40, 60, "00000000-0000-7000-8000-000000000033", time.Now().UTC())
	if err != nil || !settings.PremiumOptIn || settings.HardMinutes != 60 {
		t.Fatalf("update practice settings=%#v err=%v", settings, err)
	}
	if _, err := repository.UpdatePracticeSettings(ctx, "00000000-0000-7000-8000-000000000033", 1, false, 20, 35, 50, "00000000-0000-7000-8000-000000000033", time.Now().UTC()); err != ErrConflict {
		t.Fatalf("stale practice settings returned %v", err)
	}
	if _, err := repository.Pool.Exec(ctx, `INSERT INTO app.users(id,slug,display_name,email,account_state,timezone,revision) VALUES('00000000-0000-7000-8000-000000000027','00000000-0000-7000-8000-000000000027','Integration Mentor','mentor@rsp.local','active','Australia/Adelaide',1)`); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool.Exec(ctx, `INSERT INTO app.users(id,slug,display_name,email,account_state,timezone,mfa_configured,revision) VALUES
		('00000000-0000-7000-8000-000000000017','00000000-0000-7000-8000-000000000017','Integration Coordinator','coordinator@rsp.local','active','Australia/Adelaide',true,1),
		('00000000-0000-7000-8000-000000000023','00000000-0000-7000-8000-000000000023','Integration Promoted','promoted@rsp.local','active','Australia/Adelaide',true,1),
		('00000000-0000-7000-8000-000000000026','00000000-0000-7000-8000-000000000026','Integration Kicked','kicked@rsp.local','active','Australia/Adelaide',false,1)`); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool.Exec(ctx, `INSERT INTO app.global_role_assignments(id,user_id,role,state,activated_at) VALUES('00000000-0000-7000-8000-000000000018','00000000-0000-7000-8000-000000000026','director','active',now())`); err != nil {
		t.Fatal(err)
	}

	var assignmentState string
	var assignmentActivatedAt *time.Time
	if err := repository.Pool.QueryRow(ctx, `SELECT state::text,activated_at FROM app.global_role_assignments WHERE id='00000000-0000-7000-8000-000000000018'`).Scan(&assignmentState, &assignmentActivatedAt); err != nil || assignmentState != "pending_mfa" || assignmentActivatedAt != nil {
		t.Fatalf("non-MFA global role state=%q activatedAt=%v err=%v", assignmentState, assignmentActivatedAt, err)
	}
	if _, err := repository.Pool.Exec(ctx, `INSERT INTO app.global_role_assignments(id,user_id,role,state) VALUES('00000000-0000-7000-8000-000000000004','00000000-0000-7000-8000-000000000017','system_admin','pending_mfa')`); err != nil {
		t.Fatal(err)
	}
	if err := repository.Pool.QueryRow(ctx, `SELECT state::text,activated_at FROM app.global_role_assignments WHERE id='00000000-0000-7000-8000-000000000004'`).Scan(&assignmentState, &assignmentActivatedAt); err != nil || assignmentState != "active" || assignmentActivatedAt == nil {
		t.Fatalf("preconfigured-MFA global role state=%q activatedAt=%v err=%v", assignmentState, assignmentActivatedAt, err)
	}
	if _, err := repository.CreateEnrollment(ctx, model.Enrollment{ID: "00000000-0000-7000-8000-000000000009", SeasonID: season.ID, UserID: "00000000-0000-7000-8000-000000000033", Role: "student", State: "active", Revision: 1}, "00000000-0000-7000-8000-000000000033", time.Now()); err != nil {
		t.Fatal(err)
	}
	if users, _, _, err := repository.ListUsers(ctx, "", 25, "forward", "", "", ""); err != nil || len(users) == 0 {
		t.Fatalf("empty-cursor user list=%#v err=%v", users, err)
	}
	if users, _, _, err := repository.ListAdminUsers(ctx, "", 25, "forward", "", "", ""); err != nil || len(users) == 0 {
		t.Fatalf("empty-cursor admin user list=%#v err=%v", users, err)
	}
	if seasons, _, _, err := repository.ListSeasons(ctx, "", 25, "forward", ""); err != nil || len(seasons) == 0 {
		t.Fatalf("empty-cursor season list=%#v err=%v", seasons, err)
	}
	if _, _, _, err := repository.ListEnrollmentCandidates(ctx, season.ID, "", "", 25, "forward"); err != nil {
		t.Fatalf("empty-cursor enrollment candidate list: %v", err)
	}
	if _, err := repository.CreateEnrollment(ctx, model.Enrollment{ID: "00000000-0000-7000-8000-000000000012", SeasonID: season.ID, UserID: "00000000-0000-7000-8000-000000000027", Role: "mentor", State: "active", Revision: 1}, "00000000-0000-7000-8000-000000000033", time.Now()); err != nil {
		t.Fatal(err)
	}

	coordinatorEnrollment, err := repository.CreateEnrollment(ctx, model.Enrollment{ID: "00000000-0000-7000-8000-000000000002", SeasonID: season.ID, UserID: "00000000-0000-7000-8000-000000000017", Role: "coordinator", State: "active", Revision: 1}, "00000000-0000-7000-8000-000000000033", time.Now())
	if err != nil || coordinatorEnrollment.AssignmentState != "active" {
		t.Fatalf("preconfigured coordinator create=%#v err=%v", coordinatorEnrollment, err)
	}
	if _, err := repository.CreateEnrollment(ctx, model.Enrollment{ID: "00000000-0000-7000-8000-000000000005", SeasonID: season.ID, UserID: "00000000-0000-7000-8000-000000000023", Role: "student", State: "active", Revision: 1}, "00000000-0000-7000-8000-000000000033", time.Now()); err != nil {
		t.Fatal(err)
	}

	promotedEnrollment, err := repository.UpdateEnrollment(ctx, "00000000-0000-7000-8000-000000000005", 1, "coordinator", "", "", "00000000-0000-7000-8000-000000000033", time.Now())
	if err != nil || promotedEnrollment.AssignmentState != "active" {
		t.Fatalf("preconfigured coordinator promotion=%#v err=%v", promotedEnrollment, err)
	}
	if _, err := repository.Pool.Exec(ctx, `INSERT INTO app.enrollments(id,user_id,season_id,role,student_level,state,assignment_state,activated_at,revision) VALUES('00000000-0000-7000-8000-000000000011','00000000-0000-7000-8000-000000000026',$1,'student','beginner','kicked','revoked',NULL,1)`, season.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.CreateWeek(ctx, model.Week{ID: "00000000-0000-7000-8000-000000000034", SeasonID: season.ID, Number: 1, StartAt: season.StartAt, EndAt: season.EndAt, ResourceURL: "https://rsp.local/week", Revision: 1}, "00000000-0000-7000-8000-000000000033", time.Now()); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.CreateMentorship(ctx, model.Mentorship{ID: "00000000-0000-7000-8000-000000000020", SeasonID: season.ID, MentorUserID: "00000000-0000-7000-8000-000000000027", StudentUserID: "00000000-0000-7000-8000-000000000033", Revision: 1}, "00000000-0000-7000-8000-000000000033", time.Now()); err != nil {
		t.Fatal(err)
	}
	if assigned, err := repository.IsMentorAssigned(ctx, season.ID, "00000000-0000-7000-8000-000000000027", "00000000-0000-7000-8000-000000000033"); err != nil || !assigned {
		t.Fatalf("mentorship assigned=%v err=%v", assigned, err)
	}

	closed, err := repository.CloseSeason(ctx, season.ID, 1, "00000000-0000-7000-8000-000000000033", "complete", time.Now().UTC())
	if err != nil || closed.Status != "closed" {
		t.Fatalf("close=%#v err=%v", closed, err)
	}

	studentEnrollment, err := repository.GetEnrollment(ctx, "00000000-0000-7000-8000-000000000009")
	if err != nil || studentEnrollment.State != "completed" {
		t.Fatalf("completed enrollment=%#v err=%v", studentEnrollment, err)
	}

	reopened, err := repository.ReopenSeason(ctx, season.ID, 2, "00000000-0000-7000-8000-000000000033", "correction", time.Now().UTC())
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
	invalidMock := mockinterviews.Interview{ID: "00000000-0000-7000-8000-000000000003", InterviewerID: "00000000-0000-7000-8000-000000000027", IntervieweeID: "00000000-0000-7000-8000-000000000026", SeasonID: &season.ID, OccurredAt: season.StartAt.Add(time.Hour), DurationMinutes: 60, Rounds: []mockinterviews.Round{{ID: "00000000-0000-7000-8000-000000000016", Type: mockinterviews.Behavioural, Scores: mockinterviews.Scores{Behavioural: &invalidScore}}}, Revision: 1}
	if _, err := repository.CreateMockInterview(ctx, invalidMock, "00000000-0000-7000-8000-000000000027", time.Now().UTC()); err == nil {
		t.Fatal("kicked-only mock participant passed PostgreSQL scope validation")
	}
	if _, err := repository.Pool.Exec(ctx, `INSERT INTO app.problems(id,title,url) VALUES('00000000-0000-7000-8000-000000000019','Two Sum','https://leetcode.com/problems/two-sum/')`); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool.Exec(ctx, `INSERT INTO app.leetcode_problems(id,problem_id,leetcode_number,difficulty,is_premium) VALUES('00000000-0000-7000-8000-000000000025','00000000-0000-7000-8000-000000000019',1,'easy',false)`); err != nil {
		t.Fatal(err)
	}
	if problems, _, _, err := repository.ListProblems(ctx, "", 25, "", "", nil, "forward"); err != nil || len(problems) != 1 {
		t.Fatalf("empty-cursor problem list=%#v err=%v", problems, err)
	}

	candidates, history, err := repository.RecommendationCandidates(ctx, "00000000-0000-7000-8000-000000000033", practice.Criteria{Difficulty: practice.Easy}, false, practice.DefaultGoals(), time.Now().UTC())
	if err != nil || len(candidates) != 1 || candidates[0].ID != "00000000-0000-7000-8000-000000000025" || len(history) != 0 {
		t.Fatalf("database-side recommendation candidate=%#v history=%#v err=%v", candidates, history, err)
	}

	recommendation := practice.Recommendation{ID: "00000000-0000-7000-8000-000000000015", UserID: "00000000-0000-7000-8000-000000000033", Problem: practice.Problem{ID: "00000000-0000-7000-8000-000000000025", Number: 1, Title: "Two Sum", Link: "https://leetcode.com/problems/two-sum/", Difficulty: practice.Easy, Revision: 1}, Difficulty: practice.Easy, Rationale: "Practice arrays", RuleVersion: "v1", CreatedAt: time.Now().UTC()}
	if _, err := repository.SaveRecommendation(ctx, recommendation); err != nil {
		t.Fatal(err)
	}

	active, err := repository.GetActiveRecommendation(ctx, "00000000-0000-7000-8000-000000000033")
	if err != nil || active == nil || active.ID != recommendation.ID || active.Problem.Number != 1 || active.Problem.Revision != 1 {
		t.Fatalf("active recommendation=%#v err=%v", active, err)
	}

	_, fulfilled, err := repository.CreateAttempt(ctx, model.Attempt{ID: "00000000-0000-7000-8000-000000000024", UserID: "00000000-0000-7000-8000-000000000033", ProblemID: "00000000-0000-7000-8000-000000000025", Outcome: "independently_solved", Confidence: intPointer(5), Minutes: 12, AttemptedAt: time.Now().UTC(), Revision: 1})
	if err != nil {
		t.Fatal(err)
	}
	if !fulfilled {
		t.Fatalf("recommendation fulfilled=%v err=%v", fulfilled, err)
	}
	if attempts, _, _, err := repository.ListAttempts(ctx, "00000000-0000-7000-8000-000000000033", "", 25, "", "", "forward"); err != nil || len(attempts) != 1 {
		t.Fatalf("empty-cursor attempt list=%#v err=%v", attempts, err)
	}
	if snapshot, err := repository.RecommendationSnapshot(ctx, "00000000-0000-7000-8000-000000000033", practice.DefaultGoals()); err != nil || len(snapshot.QualityAttempts) != 1 {
		t.Fatalf("runtime-role recommendation snapshot=%#v err=%v", snapshot, err)
	}

	recommendation.ID = "00000000-0000-7000-8000-000000000013"
	if _, err := repository.SaveRecommendation(ctx, recommendation); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.DismissRecommendation(ctx, "00000000-0000-7000-8000-000000000033", 1, "not now", time.Now().UTC(), "00000000-0000-7000-8000-000000000021"); err != nil {
		t.Fatal(err)
	}
	if dismissals, err := repository.ListRecommendationDismissals(ctx, "00000000-0000-7000-8000-000000000033"); err != nil || len(dismissals) != 1 {
		t.Fatalf("dismissals=%#v err=%v", dismissals, err)
	}

	score := 7
	interview := mockinterviews.Interview{ID: "00000000-0000-7000-8000-000000000031", InterviewerID: "00000000-0000-7000-8000-000000000027", IntervieweeID: "00000000-0000-7000-8000-000000000033", SeasonID: &season.ID, OccurredAt: time.Now().UTC(), DurationMinutes: 60, Notes: "notes", Rounds: []mockinterviews.Round{{ID: "00000000-0000-7000-8000-000000000030", Type: mockinterviews.Behavioural, Scores: mockinterviews.Scores{Behavioural: &score}}}, Revision: 1}
	if _, err := repository.CreateMockInterview(ctx, interview, "00000000-0000-7000-8000-000000000027", time.Now().UTC()); err != nil {
		t.Fatal(err)
	}

	loadedInterview, err := repository.GetMockInterview(ctx, interview.ID)
	if err != nil || len(loadedInterview.Rounds) != 1 {
		t.Fatalf("mock=%#v err=%v", loadedInterview, err)
	}
	if interviews, _, _, err := repository.ListMockInterviews(ctx, authz.Actor{UserID: "00000000-0000-7000-8000-000000000027", Enrollments: []authz.Enrollment{{SeasonID: season.ID, Role: authz.Mentor, State: authz.Active}}}, "all", "", 25, "id:asc", "forward"); err != nil || len(interviews) != 1 {
		t.Fatalf("relationship-scoped mock list=%#v err=%v", interviews, err)
	}

	loadedInterview.Notes = "updated"
	loadedInterview.Revision++
	if _, err := repository.UpdateMockInterview(ctx, loadedInterview, "00000000-0000-7000-8000-000000000027", "updated", time.Now().UTC()); err != nil {
		t.Fatal(err)
	}

	var versionCount int
	if err := repository.Pool.QueryRow(ctx, `SELECT count(*) FROM app.mock_interview_versions WHERE mock_interview_id='00000000-0000-7000-8000-000000000031'`).Scan(&versionCount); err != nil || versionCount != 2 {
		t.Fatalf("mock versions=%d err=%v", versionCount, err)
	}

	currentSeason, err := repository.GetSeason(ctx, season.ID)
	if err != nil {
		t.Fatal(err)
	}

	updatedSeason, err := repository.UpdateSeason(ctx, season.ID, currentSeason.Revision, func(value *model.Season) error {
		value.Location = "Updated Adelaide"
		return nil
	}, "00000000-0000-7000-8000-000000000033", time.Now().UTC())
	if err != nil || updatedSeason.Location != "Updated Adelaide" || updatedSeason.Revision != currentSeason.Revision+1 {
		t.Fatalf("enum-backed season update=%#v err=%v", updatedSeason, err)
	}

	auditEvent := audit.Event{ID: "00000000-0000-7000-8000-000000000029", Action: "integration.test", SubjectType: "season", SubjectID: season.ID, Data: map[string]any{}, OccurredAt: time.Now().UTC()}
	if err := repository.AppendAudit(ctx, auditEvent); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool.Exec(ctx, `UPDATE app.audit_events SET action='tampered' WHERE id='00000000-0000-7000-8000-000000000029'`); err == nil {
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
