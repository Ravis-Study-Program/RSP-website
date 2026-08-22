//go:build integration

package store

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/magedmg/RSP-website/backend/internal/authz"
	"github.com/magedmg/RSP-website/backend/internal/mockinterviews"
	"github.com/magedmg/RSP-website/backend/internal/model"
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
		postgrescontainer.WithPassword("rsp-integration-password"),
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
	if err := repository.ApplyIdentityEvent(ctx, IdentityEvent{EventID: "integration-unverified-created", Type: "auth_user_created", AuthUserID: "auth-unverified", Email: "unverified@rsp.local", EmailVerified: false, SecurityVersion: 1, OccurredAt: createdAt}); err != nil {
		t.Fatalf("unverified auth user creation: %v", err)
	}
	if _, err := repository.ResolveAuthSubject(ctx, "auth-unverified"); err != ErrNotFound {
		t.Fatalf("unverified inactive link resolved: %v", err)
	}
	if err := repository.ApplyIdentityEvent(ctx, IdentityEvent{EventID: "integration-unverified-verified", Type: "email_verified", AuthUserID: "auth-unverified", SecurityVersion: 2, OccurredAt: createdAt.Add(time.Second)}); err != nil {
		t.Fatalf("email verification activation: %v", err)
	}
	if actor, err := repository.ResolveAuthSubject(ctx, "auth-unverified"); err != nil || !actor.EmailVerified || actor.SecurityVersion != 2 {
		t.Fatalf("verified auth link actor=%#v err=%v", actor, err)
	}
	_, err = repository.Pool.Exec(ctx, `INSERT INTO app.users(id,slug,display_name,email,account_state,timezone,revision) VALUES('integration-user','integration-user','Integration User','integration@rsp.local','active','Australia/Adelaide',1)`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool.Exec(ctx, `INSERT INTO app.user_auth_links(auth_subject,user_id,provider,provider_account_id) VALUES('auth-integration','integration-user','better_auth','auth-integration')`); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool.Exec(ctx, `INSERT INTO app.global_role_assignments(id,user_id,role,state) VALUES('integration-role','integration-user','director','pending_mfa')`); err != nil {
		t.Fatal(err)
	}
	if err := repository.ApplyIdentityEvent(ctx, IdentityEvent{EventID: "integration-mfa-configured", Type: "mfa_configured", AuthUserID: "auth-integration", SecurityVersion: 2, OccurredAt: time.Now().UTC()}); err != nil {
		t.Fatal(err)
	}
	actor, err := repository.ResolveAuthSubject(ctx, "auth-integration")
	if err != nil || !actor.IsGlobal("director") || actor.SecurityVersion != 2 {
		t.Fatalf("MFA role activation: actor=%#v err=%v", actor, err)
	}
	var activatedAudits int
	if err := repository.Pool.QueryRow(ctx, `SELECT count(*) FROM app.audit_events WHERE action='global_role.activated' AND subject_id='integration-role'`).Scan(&activatedAudits); err != nil || activatedAudits != 1 {
		t.Fatalf("activation audit count=%d err=%v", activatedAudits, err)
	}
	deadline := time.Now().UTC().Add(30 * 24 * time.Hour)
	if err := repository.ApplyIdentityEvent(ctx, IdentityEvent{EventID: "integration-deletion-requested", Type: "deletion_requested", AuthUserID: "auth-integration", SecurityVersion: 3, OccurredAt: time.Now().UTC(), RecoveryDeadline: &deadline}); err != nil {
		t.Fatal(err)
	}
	if err := repository.ApplyIdentityEvent(ctx, IdentityEvent{EventID: "integration-deletion-cancelled", Type: "deletion_cancelled", AuthUserID: "auth-integration", SecurityVersion: 4, OccurredAt: time.Now().UTC()}); err != nil {
		t.Fatal(err)
	}
	if err := repository.ApplyIdentityEvent(ctx, IdentityEvent{EventID: "integration-stale-deletion-requested", Type: "deletion_requested", AuthUserID: "auth-integration", SecurityVersion: 3, OccurredAt: time.Now().UTC(), RecoveryDeadline: &deadline}); err != nil {
		t.Fatal(err)
	}
	actor, err = repository.ResolveAuthSubject(ctx, "auth-integration")
	if err != nil || actor.AccountState != "active" || actor.SecurityVersion != 4 {
		t.Fatalf("stale lifecycle event applied: actor=%#v err=%v", actor, err)
	}
	season, err := repository.CreateSeason(ctx, model.Season{ID: "integration-season", Slug: "integration-season", Name: "Integration Season", Status: "open", StartAt: time.Now().UTC(), EndAt: time.Now().UTC().Add(24 * time.Hour), Revision: 1}, "integration-user", time.Now())
	if err != nil || season.Revision != 1 {
		t.Fatalf("create season: %#v %v", season, err)
	}
	if _, err := repository.UpdateSeason(ctx, season.ID, 99, func(*model.Season) error { return nil }, "integration-user", time.Now()); err != ErrConflict {
		t.Fatalf("stale revision returned %v", err)
	}
	settings, err := repository.GetPracticeSettings(ctx, "integration-user")
	if err != nil || settings.GoalsEnabled || settings.EasyMinutes != 20 || settings.Revision != 1 {
		t.Fatalf("default practice settings=%#v err=%v", settings, err)
	}
	if _, err := repository.EnablePracticeGoals(ctx, "integration-user", 99, "integration-user", season.ID, time.Now().UTC()); err != ErrConflict {
		t.Fatalf("missing-row stale goal enable returned %v", err)
	}
	settings, err = repository.EnablePracticeGoals(ctx, "integration-user", settings.Revision, "integration-user", season.ID, time.Now().UTC())
	if err != nil || !settings.GoalsEnabled {
		t.Fatalf("enable practice goals=%#v err=%v", settings, err)
	}
	settings, err = repository.UpdatePracticeSettings(ctx, "integration-user", settings.Revision, true, 25, 40, 60, "integration-user", time.Now().UTC())
	if err != nil || !settings.PremiumOptIn || settings.HardMinutes != 60 {
		t.Fatalf("update practice settings=%#v err=%v", settings, err)
	}
	if _, err := repository.UpdatePracticeSettings(ctx, "integration-user", 1, false, 20, 35, 50, "integration-user", time.Now().UTC()); err != ErrConflict {
		t.Fatalf("stale practice settings returned %v", err)
	}
	if _, err := repository.Pool.Exec(ctx, `INSERT INTO app.users(id,slug,display_name,email,account_state,timezone,revision) VALUES('integration-mentor','integration-mentor','Integration Mentor','mentor@rsp.local','active','Australia/Adelaide',1)`); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool.Exec(ctx, `INSERT INTO app.users(id,slug,display_name,email,account_state,timezone,mfa_configured,revision) VALUES
		('integration-coordinator','integration-coordinator','Integration Coordinator','coordinator@rsp.local','active','Australia/Adelaide',true,1),
		('integration-promoted','integration-promoted','Integration Promoted','promoted@rsp.local','active','Australia/Adelaide',true,1),
		('integration-kicked','integration-kicked','Integration Kicked','kicked@rsp.local','active','Australia/Adelaide',false,1)`); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool.Exec(ctx, `INSERT INTO app.global_role_assignments(id,user_id,role,state,activated_at) VALUES('integration-kicked-role','integration-kicked','director','active',now())`); err != nil {
		t.Fatal(err)
	}
	var assignmentState string
	var assignmentActivatedAt *time.Time
	if err := repository.Pool.QueryRow(ctx, `SELECT state::text,activated_at FROM app.global_role_assignments WHERE id='integration-kicked-role'`).Scan(&assignmentState, &assignmentActivatedAt); err != nil || assignmentState != "pending_mfa" || assignmentActivatedAt != nil {
		t.Fatalf("non-MFA global role state=%q activatedAt=%v err=%v", assignmentState, assignmentActivatedAt, err)
	}
	if _, err := repository.Pool.Exec(ctx, `INSERT INTO app.global_role_assignments(id,user_id,role,state) VALUES('integration-preconfigured-admin','integration-coordinator','system_admin','pending_mfa')`); err != nil {
		t.Fatal(err)
	}
	if err := repository.Pool.QueryRow(ctx, `SELECT state::text,activated_at FROM app.global_role_assignments WHERE id='integration-preconfigured-admin'`).Scan(&assignmentState, &assignmentActivatedAt); err != nil || assignmentState != "active" || assignmentActivatedAt == nil {
		t.Fatalf("preconfigured-MFA global role state=%q activatedAt=%v err=%v", assignmentState, assignmentActivatedAt, err)
	}
	if _, err := repository.CreateEnrollment(ctx, model.Enrollment{ID: "integration-student-enrollment", SeasonID: season.ID, UserID: "integration-user", Role: "student", State: "active", Revision: 1}, "integration-user", time.Now()); err != nil {
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
	if _, err := repository.CreateEnrollment(ctx, model.Enrollment{ID: "integration-mentor-enrollment", SeasonID: season.ID, UserID: "integration-mentor", Role: "mentor", State: "active", Revision: 1}, "integration-user", time.Now()); err != nil {
		t.Fatal(err)
	}
	coordinatorEnrollment, err := repository.CreateEnrollment(ctx, model.Enrollment{ID: "integration-coordinator-enrollment", SeasonID: season.ID, UserID: "integration-coordinator", Role: "coordinator", State: "active", Revision: 1}, "integration-user", time.Now())
	if err != nil || coordinatorEnrollment.AssignmentState != "active" {
		t.Fatalf("preconfigured coordinator create=%#v err=%v", coordinatorEnrollment, err)
	}
	if _, err := repository.CreateEnrollment(ctx, model.Enrollment{ID: "integration-promoted-enrollment", SeasonID: season.ID, UserID: "integration-promoted", Role: "student", State: "active", Revision: 1}, "integration-user", time.Now()); err != nil {
		t.Fatal(err)
	}
	promotedEnrollment, err := repository.UpdateEnrollment(ctx, "integration-promoted-enrollment", 1, "coordinator", "", "", "integration-user", time.Now())
	if err != nil || promotedEnrollment.AssignmentState != "active" {
		t.Fatalf("preconfigured coordinator promotion=%#v err=%v", promotedEnrollment, err)
	}
	if _, err := repository.Pool.Exec(ctx, `INSERT INTO app.enrollments(id,user_id,season_id,role,student_level,state,assignment_state,activated_at,revision) VALUES('integration-kicked-enrollment','integration-kicked',$1,'student','beginner','kicked','revoked',NULL,1)`, season.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.CreateWeek(ctx, model.Week{ID: "integration-week", SeasonID: season.ID, Number: 1, StartAt: season.StartAt, EndAt: season.EndAt, ResourceURL: "https://rsp.local/week", Revision: 1}, "integration-user", time.Now()); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.CreateMentorship(ctx, model.Mentorship{ID: "integration-mentorship", SeasonID: season.ID, MentorUserID: "integration-mentor", StudentUserID: "integration-user", Revision: 1}, "integration-user", time.Now()); err != nil {
		t.Fatal(err)
	}
	if assigned, err := repository.IsMentorAssigned(ctx, season.ID, "integration-mentor", "integration-user"); err != nil || !assigned {
		t.Fatalf("mentorship assigned=%v err=%v", assigned, err)
	}
	closed, err := repository.CloseSeason(ctx, season.ID, 1, "integration-user", "complete", time.Now().UTC())
	if err != nil || closed.Status != "closed" {
		t.Fatalf("close=%#v err=%v", closed, err)
	}
	studentEnrollment, err := repository.GetEnrollment(ctx, "integration-student-enrollment")
	if err != nil || studentEnrollment.State != "completed" {
		t.Fatalf("completed enrollment=%#v err=%v", studentEnrollment, err)
	}
	reopened, err := repository.ReopenSeason(ctx, season.ID, 2, "integration-user", "correction", time.Now().UTC())
	if err != nil || reopened.Status != "open" {
		t.Fatalf("reopen=%#v err=%v", reopened, err)
	}
	studentEnrollment, err = repository.GetEnrollment(ctx, "integration-student-enrollment")
	if err != nil || studentEnrollment.State != "active" {
		t.Fatalf("restored enrollment=%#v err=%v", studentEnrollment, err)
	}
	coordinatorEnrollment, err = repository.GetEnrollment(ctx, "integration-coordinator-enrollment")
	if err != nil || coordinatorEnrollment.State != "active" || coordinatorEnrollment.AssignmentState != "active" {
		t.Fatalf("restored coordinator enrollment=%#v err=%v", coordinatorEnrollment, err)
	}

	invalidScore := 7
	invalidMock := mockinterviews.Interview{ID: "integration-invalid-kicked-mock", InterviewerID: "integration-mentor", IntervieweeID: "integration-kicked", SeasonID: &season.ID, OccurredAt: season.StartAt.Add(time.Hour), DurationMinutes: 60, Rounds: []mockinterviews.Round{{ID: "integration-invalid-round", Type: mockinterviews.Behavioural, Scores: mockinterviews.Scores{Behavioural: &invalidScore}}}, Revision: 1}
	if _, err := repository.CreateMockInterview(ctx, invalidMock, "integration-mentor", time.Now().UTC()); err == nil {
		t.Fatal("kicked-only mock participant passed PostgreSQL scope validation")
	}

	if _, err := repository.Pool.Exec(ctx, `INSERT INTO app.problems(id,title,url) VALUES('integration-problem-row','Two Sum','https://leetcode.com/problems/two-sum/')`); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool.Exec(ctx, `INSERT INTO app.leetcode_problems(id,problem_id,leetcode_number,difficulty,is_premium) VALUES('integration-problem','integration-problem-row',1,'easy',false)`); err != nil {
		t.Fatal(err)
	}
	if problems, _, _, err := repository.ListProblems(ctx, "", 25, "", "", nil, "forward"); err != nil || len(problems) != 1 {
		t.Fatalf("empty-cursor problem list=%#v err=%v", problems, err)
	}
	candidates, history, err := repository.RecommendationCandidates(ctx, "integration-user", practice.Criteria{Difficulty: practice.Easy}, false, practice.DefaultGoals(), time.Now().UTC())
	if err != nil || len(candidates) != 1 || candidates[0].ID != "integration-problem" || len(history) != 0 {
		t.Fatalf("database-side recommendation candidate=%#v history=%#v err=%v", candidates, history, err)
	}
	recommendation := practice.Recommendation{ID: "integration-recommendation", UserID: "integration-user", Problem: practice.Problem{ID: "integration-problem", Number: 1, Title: "Two Sum", Link: "https://leetcode.com/problems/two-sum/", Difficulty: practice.Easy, Revision: 1}, Difficulty: practice.Easy, Rationale: "Practice arrays", RuleVersion: "v1", CreatedAt: time.Now().UTC()}
	if _, err := repository.SaveRecommendation(ctx, recommendation); err != nil {
		t.Fatal(err)
	}
	active, err := repository.GetActiveRecommendation(ctx, "integration-user")
	if err != nil || active == nil || active.ID != recommendation.ID || active.Problem.Number != 1 || active.Problem.Revision != 1 {
		t.Fatalf("active recommendation=%#v err=%v", active, err)
	}
	_, fulfilled, err := repository.CreateAttempt(ctx, model.Attempt{ID: "integration-attempt", UserID: "integration-user", ProblemID: "integration-problem", Outcome: "independently_solved", Confidence: intPointer(5), Minutes: 12, AttemptedAt: time.Now().UTC(), Revision: 1})
	if err != nil {
		t.Fatal(err)
	}
	if !fulfilled {
		t.Fatalf("recommendation fulfilled=%v err=%v", fulfilled, err)
	}
	if attempts, _, _, err := repository.ListAttempts(ctx, "integration-user", "", 25, "", "", "forward"); err != nil || len(attempts) != 1 {
		t.Fatalf("empty-cursor attempt list=%#v err=%v", attempts, err)
	}
	if snapshot, err := repository.RecommendationSnapshot(ctx, "integration-user", practice.DefaultGoals()); err != nil || len(snapshot.QualityAttempts) != 1 {
		t.Fatalf("runtime-role recommendation snapshot=%#v err=%v", snapshot, err)
	}
	recommendation.ID = "integration-recommendation-2"
	if _, err := repository.SaveRecommendation(ctx, recommendation); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.DismissRecommendation(ctx, "integration-user", 1, "not now", time.Now().UTC(), "integration-dismissal"); err != nil {
		t.Fatal(err)
	}
	if dismissals, err := repository.ListRecommendationDismissals(ctx, "integration-user"); err != nil || len(dismissals) != 1 {
		t.Fatalf("dismissals=%#v err=%v", dismissals, err)
	}

	score := 7
	interview := mockinterviews.Interview{ID: "integration-mock", InterviewerID: "integration-mentor", IntervieweeID: "integration-user", SeasonID: &season.ID, OccurredAt: time.Now().UTC(), DurationMinutes: 60, Notes: "notes", Rounds: []mockinterviews.Round{{ID: "integration-round", Type: mockinterviews.Behavioural, Scores: mockinterviews.Scores{Behavioural: &score}}}, Revision: 1}
	if _, err := repository.CreateMockInterview(ctx, interview, "integration-mentor", time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	loadedInterview, err := repository.GetMockInterview(ctx, interview.ID)
	if err != nil || len(loadedInterview.Rounds) != 1 {
		t.Fatalf("mock=%#v err=%v", loadedInterview, err)
	}
	if interviews, _, _, err := repository.ListMockInterviews(ctx, authz.Actor{UserID: "integration-mentor", Enrollments: []authz.Enrollment{{SeasonID: season.ID, Role: authz.Mentor, State: authz.Active}}}, "all", "", 25, "id:asc", "forward"); err != nil || len(interviews) != 1 {
		t.Fatalf("relationship-scoped mock list=%#v err=%v", interviews, err)
	}
	loadedInterview.Notes = "updated"
	loadedInterview.Revision++
	if _, err := repository.UpdateMockInterview(ctx, loadedInterview, "integration-mentor", "updated", time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	var versionCount int
	if err := repository.Pool.QueryRow(ctx, `SELECT count(*) FROM app.mock_interview_versions WHERE mock_interview_id='integration-mock'`).Scan(&versionCount); err != nil || versionCount != 2 {
		t.Fatalf("mock versions=%d err=%v", versionCount, err)
	}
	currentSeason, err := repository.GetSeason(ctx, season.ID)
	if err != nil {
		t.Fatal(err)
	}
	updatedSeason, err := repository.UpdateSeason(ctx, season.ID, currentSeason.Revision, func(value *model.Season) error {
		value.Location = "Updated Adelaide"
		return nil
	}, "integration-user", time.Now().UTC())
	if err != nil || updatedSeason.Location != "Updated Adelaide" || updatedSeason.Revision != currentSeason.Revision+1 {
		t.Fatalf("enum-backed season update=%#v err=%v", updatedSeason, err)
	}

	audit := model.AuditEvent{ID: "integration-audit", Action: "integration.test", SubjectType: "season", SubjectID: season.ID, Data: map[string]any{}, OccurredAt: time.Now().UTC()}
	if err := repository.AppendAudit(ctx, audit); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool.Exec(ctx, `UPDATE app.audit_events SET action='tampered' WHERE id='integration-audit'`); err == nil {
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
