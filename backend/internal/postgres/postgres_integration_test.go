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
	"github.com/magedmg/RSP-website/backend/internal/leetcode"
	"github.com/magedmg/RSP-website/backend/internal/mockinterviews"
	"github.com/magedmg/RSP-website/backend/internal/platform/audit"
	"github.com/magedmg/RSP-website/backend/internal/practice"
	"github.com/magedmg/RSP-website/backend/internal/programme"
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

	err = repository.DB.WithContext(ctx).Exec(`INSERT INTO app.users(id,slug,display_name,email,account_state,timezone,revision) VALUES('00000000-0000-7000-8000-000000000033','00000000-0000-7000-8000-000000000033','Integration User','integration@rsp.local','active','Australia/Adelaide',1)`).Error
	if err != nil {
		t.Fatal(err)
	}
	if err := repository.DB.WithContext(ctx).Exec(`INSERT INTO app.user_auth_links(auth_subject,user_id,provider,provider_account_id) VALUES('auth-integration','00000000-0000-7000-8000-000000000033','better_auth','auth-integration')`).Error; err != nil {
		t.Fatal(err)
	}
	if err := repository.DB.WithContext(ctx).Exec(`INSERT INTO app.global_role_assignments(id,user_id,role,state) VALUES('00000000-0000-7000-8000-000000000032','00000000-0000-7000-8000-000000000033','director','pending_mfa')`).Error; err != nil {
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
	if err := repository.DB.WithContext(ctx).Raw(`SELECT count(*) FROM app.audit_events WHERE action='global_role.activated' AND subject_id='00000000-0000-7000-8000-000000000032'`).Row().Scan(&activatedAudits); err != nil || activatedAudits != 1 {
		t.Fatalf("activation audit count=%d err=%v", activatedAudits, err)
	}
	if err := repository.QueueLeetCodeSync(ctx, actor.UserID, "integration-request"); err != nil {
		t.Fatalf("queue LeetCode sync: %v", err)
	}
	var storedRequestID string
	if err := repository.DB.WithContext(ctx).Raw(`SELECT request_id FROM app.audit_events WHERE action='leetcode.sync_requested' ORDER BY occurred_at DESC LIMIT 1`).Row().Scan(&storedRequestID); err != nil || storedRequestID != "integration-request" {
		t.Fatalf("queued sync request id=%q err=%v", storedRequestID, err)
	}

	pinnedDB, closePinned, err := repository.PinnedConnection(ctx)
	if err != nil {
		t.Fatal(err)
	}
	state := WorkerState{DB: pinnedDB}
	locked, err := state.TryLock(ctx, 7277500199)
	if err != nil || !locked {
		_ = closePinned()
		t.Fatalf("worker advisory lock=%v err=%v", locked, err)
	}
	if err := state.Unlock(ctx, 7277500199); err != nil {
		_ = closePinned()
		t.Fatal(err)
	}
	if err := closePinned(); err != nil {
		t.Fatal(err)
	}

	sink := leetcode.PostgresSink{DB: repository.DB}
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
	if err := repository.DB.WithContext(ctx).Raw(`SELECT count(*) FROM app.leetcode_problem_category_mappings m JOIN app.leetcode_problems l ON l.id=m.leetcode_problem_id WHERE l.leetcode_number=999`).Row().Scan(&categoryCount); err != nil || categoryCount != 1 {
		t.Fatalf("LeetCode category count=%d err=%v", categoryCount, err)
	}
	if err := repository.DB.WithContext(ctx).Exec(`UPDATE app.problems p SET deleted_at=now() FROM app.leetcode_problems l WHERE l.problem_id=p.id AND l.leetcode_number=999`).Error; err != nil {
		t.Fatal(err)
	}
	if err := repository.DB.WithContext(ctx).Exec(`UPDATE app.leetcode_problems SET deleted_at=now() WHERE leetcode_number=999`).Error; err != nil {
		t.Fatal(err)
	}
	if snapshot, err := repository.ObservabilitySnapshot(ctx); err != nil || snapshot.MigrationState != "none" {
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

	season, err := repository.CreateSeason(ctx, programme.SeasonRecord{ID: "00000000-0000-7000-8000-000000000028", Slug: "00000000-0000-7000-8000-000000000028", Name: "Integration Season", Status: "open", StartAt: time.Now().UTC(), EndAt: time.Now().UTC().Add(24 * time.Hour), Revision: 1}, "00000000-0000-7000-8000-000000000033", time.Now())
	if err != nil || season.Revision != 1 {
		t.Fatalf("create season: %#v %v", season, err)
	}
	if _, err := repository.UpdateSeason(ctx, season.ID, 99, func(*programme.SeasonRecord) error { return nil }, "00000000-0000-7000-8000-000000000033", time.Now()); err != ErrConflict {
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

	settings, err = repository.UpdatePracticeSettings(ctx, "00000000-0000-7000-8000-000000000033", settings.Revision, 25, 40, 60, "00000000-0000-7000-8000-000000000033", time.Now().UTC())
	if err != nil || settings.HardMinutes != 60 {
		t.Fatalf("update practice settings=%#v err=%v", settings, err)
	}
	if _, err := repository.UpdatePracticeSettings(ctx, "00000000-0000-7000-8000-000000000033", 1, 20, 35, 50, "00000000-0000-7000-8000-000000000033", time.Now().UTC()); err != ErrConflict {
		t.Fatalf("stale practice settings returned %v", err)
	}
	if err := repository.DB.WithContext(ctx).Exec(`INSERT INTO app.users(id,slug,display_name,email,account_state,timezone,revision) VALUES('00000000-0000-7000-8000-000000000027','00000000-0000-7000-8000-000000000027','Integration Mentor','mentor@rsp.local','active','Australia/Adelaide',1)`).Error; err != nil {
		t.Fatal(err)
	}
	if err := repository.DB.WithContext(ctx).Exec(`INSERT INTO app.users(id,slug,display_name,email,account_state,timezone,mfa_configured,revision) VALUES
		('00000000-0000-7000-8000-000000000017','00000000-0000-7000-8000-000000000017','Integration Coordinator','coordinator@rsp.local','active','Australia/Adelaide',true,1),
		('00000000-0000-7000-8000-000000000023','00000000-0000-7000-8000-000000000023','Integration Promoted','promoted@rsp.local','active','Australia/Adelaide',true,1),
		('00000000-0000-7000-8000-000000000026','00000000-0000-7000-8000-000000000026','Integration Kicked','kicked@rsp.local','active','Australia/Adelaide',false,1)`).Error; err != nil {
		t.Fatal(err)
	}
	if err := repository.DB.WithContext(ctx).Exec(`INSERT INTO app.global_role_assignments(id,user_id,role,state,activated_at) VALUES('00000000-0000-7000-8000-000000000018','00000000-0000-7000-8000-000000000026','director','active',now())`).Error; err != nil {
		t.Fatal(err)
	}

	var assignmentState string
	var assignmentActivatedAt *time.Time
	if err := repository.DB.WithContext(ctx).Raw(`SELECT state::text,activated_at FROM app.global_role_assignments WHERE id='00000000-0000-7000-8000-000000000018'`).Row().Scan(&assignmentState, &assignmentActivatedAt); err != nil || assignmentState != "pending_mfa" || assignmentActivatedAt != nil {
		t.Fatalf("non-MFA global role state=%q activatedAt=%v err=%v", assignmentState, assignmentActivatedAt, err)
	}
	if err := repository.DB.WithContext(ctx).Exec(`INSERT INTO app.global_role_assignments(id,user_id,role,state) VALUES('00000000-0000-7000-8000-000000000004','00000000-0000-7000-8000-000000000017','system_admin','pending_mfa')`).Error; err != nil {
		t.Fatal(err)
	}
	if err := repository.DB.WithContext(ctx).Raw(`SELECT state::text,activated_at FROM app.global_role_assignments WHERE id='00000000-0000-7000-8000-000000000004'`).Row().Scan(&assignmentState, &assignmentActivatedAt); err != nil || assignmentState != "active" || assignmentActivatedAt == nil {
		t.Fatalf("preconfigured-MFA global role state=%q activatedAt=%v err=%v", assignmentState, assignmentActivatedAt, err)
	}
	if _, err := repository.CreateEnrollment(ctx, programme.EnrollmentRecord{ID: "00000000-0000-7000-8000-000000000009", SeasonID: season.ID, UserID: "00000000-0000-7000-8000-000000000033", Role: "student", State: "active", Revision: 1}, "00000000-0000-7000-8000-000000000033", time.Now()); err != nil {
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
	if _, err := repository.CreateEnrollment(ctx, programme.EnrollmentRecord{ID: "00000000-0000-7000-8000-000000000012", SeasonID: season.ID, UserID: "00000000-0000-7000-8000-000000000027", Role: "mentor", State: "active", Revision: 1}, "00000000-0000-7000-8000-000000000033", time.Now()); err != nil {
		t.Fatal(err)
	}

	coordinatorEnrollment, err := repository.CreateEnrollment(ctx, programme.EnrollmentRecord{ID: "00000000-0000-7000-8000-000000000002", SeasonID: season.ID, UserID: "00000000-0000-7000-8000-000000000017", Role: "coordinator", State: "active", Revision: 1}, "00000000-0000-7000-8000-000000000033", time.Now())
	if err != nil || coordinatorEnrollment.AssignmentState != "active" {
		t.Fatalf("preconfigured coordinator create=%#v err=%v", coordinatorEnrollment, err)
	}
	if _, err := repository.CreateEnrollment(ctx, programme.EnrollmentRecord{ID: "00000000-0000-7000-8000-000000000005", SeasonID: season.ID, UserID: "00000000-0000-7000-8000-000000000023", Role: "student", State: "active", Revision: 1}, "00000000-0000-7000-8000-000000000033", time.Now()); err != nil {
		t.Fatal(err)
	}

	promotedEnrollment, err := repository.UpdateEnrollment(ctx, "00000000-0000-7000-8000-000000000005", 1, "coordinator", "", "", "00000000-0000-7000-8000-000000000033", time.Now())
	if err != nil || promotedEnrollment.AssignmentState != "active" {
		t.Fatalf("preconfigured coordinator promotion=%#v err=%v", promotedEnrollment, err)
	}
	if err := repository.DB.WithContext(ctx).Exec(`INSERT INTO app.enrollments(id,user_id,season_id,role,student_level,state,assignment_state,activated_at,revision) VALUES('00000000-0000-7000-8000-000000000011','00000000-0000-7000-8000-000000000026',$1,'student','beginner','kicked','revoked',NULL,1)`, season.ID).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := repository.CreateWeek(ctx, programme.WeekRecord{ID: "00000000-0000-7000-8000-000000000034", SeasonID: season.ID, Number: 1, StartAt: season.StartAt, EndAt: season.EndAt, ResourceURL: "https://rsp.local/week", Revision: 1}, "00000000-0000-7000-8000-000000000033", time.Now()); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.CreateMentorship(ctx, programme.MentorshipRecord{ID: "00000000-0000-7000-8000-000000000020", SeasonID: season.ID, MentorUserID: "00000000-0000-7000-8000-000000000027", StudentUserID: "00000000-0000-7000-8000-000000000033", Revision: 1}, "00000000-0000-7000-8000-000000000033", time.Now()); err != nil {
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
	if err := repository.DB.WithContext(ctx).Exec(`INSERT INTO app.problems(id,title,url) VALUES('00000000-0000-7000-8000-000000000019','Two Sum','https://leetcode.com/problems/two-sum/')`).Error; err != nil {
		t.Fatal(err)
	}
	if err := repository.DB.WithContext(ctx).Exec(`INSERT INTO app.leetcode_problems(id,problem_id,leetcode_number,difficulty,is_premium) VALUES('00000000-0000-7000-8000-000000000025','00000000-0000-7000-8000-000000000019',1,'easy',false)`).Error; err != nil {
		t.Fatal(err)
	}
	if problems, _, _, err := repository.ListProblems(ctx, "", 25, "", "", nil, "forward"); err != nil || len(problems) != 1 {
		t.Fatalf("empty-cursor problem list=%#v err=%v", problems, err)
	}

	_, err = repository.CreateAttempt(ctx, practice.AttemptRecord{ID: "00000000-0000-7000-8000-000000000024", UserID: "00000000-0000-7000-8000-000000000033", ProblemID: "00000000-0000-7000-8000-000000000025", Outcome: "independently_solved", Confidence: intPointer(5), Minutes: 12, AttemptedAt: time.Now().UTC(), Revision: 1})
	if err != nil {
		t.Fatal(err)
	}
	if attempts, _, _, err := repository.ListAttempts(ctx, "00000000-0000-7000-8000-000000000033", "", 25, "", "", "forward"); err != nil || len(attempts) != 1 {
		t.Fatalf("empty-cursor attempt list=%#v err=%v", attempts, err)
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
	if err := repository.DB.WithContext(ctx).Raw(`SELECT count(*) FROM app.mock_interview_versions WHERE mock_interview_id='00000000-0000-7000-8000-000000000031'`).Row().Scan(&versionCount); err != nil || versionCount != 2 {
		t.Fatalf("mock versions=%d err=%v", versionCount, err)
	}

	currentSeason, err := repository.GetSeason(ctx, season.ID)
	if err != nil {
		t.Fatal(err)
	}

	updatedSeason, err := repository.UpdateSeason(ctx, season.ID, currentSeason.Revision, func(value *programme.SeasonRecord) error {
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
	if err := repository.DB.WithContext(ctx).Exec(`UPDATE app.audit_events SET action='tampered' WHERE id='00000000-0000-7000-8000-000000000029'`).Error; err == nil {
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
