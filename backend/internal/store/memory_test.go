package store

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/magedmg/RSP-website/backend/internal/accounts"
	"github.com/magedmg/RSP-website/backend/internal/mockinterviews"
	"github.com/magedmg/RSP-website/backend/internal/model"
	"github.com/magedmg/RSP-website/backend/internal/practice"
	"github.com/magedmg/RSP-website/backend/internal/programme"
)

func TestMemoryCloseReopenRestoresOnlyCloseCompletedEnrollments(t *testing.T) {
	repository := NewMemory()
	repository.Seasons["season"] = programme.SeasonRecord{ID: "season", Slug: "season", Status: "open", Revision: 1}
	repository.Enrollments["active"] = model.Enrollment{ID: "active", SeasonID: "season", UserID: "student", Role: "student", State: "active", Revision: 1}
	repository.Enrollments["kicked"] = model.Enrollment{ID: "kicked", SeasonID: "season", UserID: "former", Role: "student", State: "kicked", Revision: 2}

	closed, err := repository.CloseSeason(context.Background(), "season", 1, "coordinator", "programme completed", time.Now())
	if err != nil || closed.Status != "closed" || repository.Enrollments["active"].State != "completed" || repository.Enrollments["kicked"].State != "kicked" {
		t.Fatalf("close result=%#v active=%#v kicked=%#v err=%v", closed, repository.Enrollments["active"], repository.Enrollments["kicked"], err)
	}
	if _, err := repository.CloseSeason(context.Background(), "season", 1, "coordinator", "duplicate", time.Now()); err != ErrConflict {
		t.Fatalf("stale close returned %v", err)
	}

	reopened, err := repository.ReopenSeason(context.Background(), "season", 2, "director", "correction", time.Now())
	if err != nil || reopened.Status != "open" || repository.Enrollments["active"].State != "active" || repository.Enrollments["kicked"].State != "kicked" {
		t.Fatalf("reopen result=%#v active=%#v kicked=%#v err=%v", reopened, repository.Enrollments["active"], repository.Enrollments["kicked"], err)
	}
	if len(repository.Audits) != 2 || repository.Audits[0].Action != "season.closed" || repository.Audits[1].Action != "season.reopened" {
		t.Fatalf("transactional audits=%#v", repository.Audits)
	}
}

func TestMemoryCoordinatorMFAStateSurvivesCloseReopen(t *testing.T) {
	ctx := context.Background()
	repository := NewMemory()
	repository.Users["coordinator"] = accounts.User{ID: "coordinator", Slug: "coordinator", AccountState: "active", Revision: 1}
	repository.MFAConfigured["coordinator"] = true
	repository.Seasons["season"] = programme.SeasonRecord{ID: "season", Slug: "season", Status: "open", Revision: 1}

	enrollment, err := repository.CreateEnrollment(ctx, model.Enrollment{ID: "coordinator-enrollment", SeasonID: "season", UserID: "coordinator", Role: "coordinator", State: "active", Revision: 1}, "admin", time.Now())
	if err != nil || enrollment.AssignmentState != "active" {
		t.Fatalf("preconfigured coordinator grant=%#v err=%v", enrollment, err)
	}
	if _, err := repository.CloseSeason(ctx, "season", 1, "coordinator", "complete", time.Now()); err != nil {
		t.Fatal(err)
	}
	if closed := repository.Enrollments[enrollment.ID]; closed.State != "completed" || closed.AssignmentState != "revoked" {
		t.Fatalf("closed coordinator=%#v", closed)
	}
	if _, err := repository.ReopenSeason(ctx, "season", 2, "admin", "correction", time.Now()); err != nil {
		t.Fatal(err)
	}
	if reopened := repository.Enrollments[enrollment.ID]; reopened.State != "active" || reopened.AssignmentState != "active" {
		t.Fatalf("reopened coordinator lost MFA activation=%#v", reopened)
	}
}

func TestMemoryIdentityReceiptsRejectAlteredReplayAndPseudonymizeProfile(t *testing.T) {
	ctx := context.Background()
	repository := NewMemory()
	created := accounts.IdentityEvent{EventID: "event-created", Type: "auth_user_created", AuthUserID: "auth-subject", Email: "member@example.com", EmailVerified: true, SecurityVersion: 1, OccurredAt: time.Now().UTC()}
	if err := repository.ApplyIdentityEvent(ctx, created); err != nil {
		t.Fatal(err)
	}

	actor, err := repository.ResolveAuthSubject(ctx, created.AuthUserID)
	if err != nil {
		t.Fatal(err)
	}

	oldSlug := repository.Users[actor.UserID].Slug
	if err := repository.ApplyIdentityEvent(ctx, created); err != nil {
		t.Fatalf("exact replay was not idempotent: %v", err)
	}

	altered := created
	altered.Email = "attacker@example.com"
	if err := repository.ApplyIdentityEvent(ctx, altered); !errors.Is(err, ErrConflict) {
		t.Fatalf("altered event-id replay returned %v", err)
	}
	if err := repository.ApplyIdentityEvent(ctx, accounts.IdentityEvent{EventID: "event-pseudonymized", Type: "auth_pseudonymized", AuthUserID: created.AuthUserID, SecurityVersion: 2, OccurredAt: time.Now().UTC()}); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.GetUser(ctx, oldSlug); !errors.Is(err, ErrNotFound) {
		t.Fatalf("old profile slug still resolves: %v", err)
	}

	user, err := repository.GetUser(ctx, actor.UserID)
	if err != nil || user.Slug != "deleted-"+actor.UserID || user.Name != "Deleted member" || user.Email != "" || user.AvatarURL != nil || user.Timezone != "UTC" || user.TimezoneConfigured || user.AccountState != "deleted" {
		t.Fatalf("pseudonymized user=%#v err=%v", user, err)
	}
	if len(repository.Audits) != 1 || repository.Audits[0].Action != "account.pseudonymized" {
		t.Fatalf("pseudonymization audit=%#v", repository.Audits)
	}
}

func TestMemoryPracticeMutationsRequireCurrentIndependentRevision(t *testing.T) {
	ctx := context.Background()
	repository := NewMemory()
	repository.Users["student"] = accounts.User{ID: "student", AccountState: "active", Revision: 1}
	repository.Problems["problem"] = model.Problem{ID: "problem", Number: 1, Title: "Problem", Link: "https://rsp.test/problem", Difficulty: "easy", Categories: []string{"arrays"}, Revision: 1}
	settings, err := repository.EnablePracticeGoals(ctx, "student", 1, "mentor", "season", time.Now())
	if err != nil || settings.Revision != 2 {
		t.Fatalf("enabled settings=%#v err=%v", settings, err)
	}
	if _, err := repository.EnablePracticeGoals(ctx, "student", 1, "mentor", "season", time.Now()); !errors.Is(err, ErrConflict) {
		t.Fatalf("stale settings mutation returned %v", err)
	}

	recommendation, err := repository.SaveRecommendation(ctx, practice.Recommendation{ID: "recommendation", UserID: "student", Problem: practice.Problem{ID: "problem"}, CreatedAt: time.Now()})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repository.DismissRecommendation(ctx, "student", recommendation.Revision+1, "later", time.Now(), "dismissal-stale"); !errors.Is(err, ErrConflict) {
		t.Fatalf("stale dismissal returned %v", err)
	}
	if _, err := repository.DismissRecommendation(ctx, "student", recommendation.Revision, "later", time.Now(), "dismissal"); err != nil {
		t.Fatal(err)
	}
}

func TestMemoryRecommendationSnapshotUsesNewestQualityAndEntireCatalogue(t *testing.T) {
	repository := NewMemory()
	now := time.Now().UTC()
	for index := 0; index < 10001; index++ {
		identifier := fmt.Sprintf("%05d", index)
		repository.Problems[identifier] = model.Problem{ID: identifier, Difficulty: "medium"}
		repository.Attempts[identifier] = model.Attempt{ID: identifier, UserID: "student", ProblemID: identifier, Outcome: "unknown", AttemptedAt: now.Add(time.Duration(index) * time.Second), Migrated: true}
	}
	newest := repository.Attempts["00000"]
	newest.Outcome, newest.Migrated, newest.Minutes, newest.AttemptedAt = "independently_solved", false, 10, now.Add(20000*time.Second)
	repository.Attempts[newest.ID] = newest
	repository.Attempts["zzzzz-old-quality"] = model.Attempt{ID: "zzzzz-old-quality", UserID: "student", ProblemID: "00001", Outcome: "independently_solved", Minutes: 10, AttemptedAt: now.Add(-time.Hour)}
	repository.Problems["zzzzz-unseen"] = model.Problem{ID: "zzzzz-unseen", Number: 10002, Difficulty: "medium"}

	snapshot, err := repository.RecommendationSnapshot(context.Background(), "student", practice.DefaultGoals())
	if err != nil || len(snapshot.Problems) != 2 || len(snapshot.QualityAttempts) != 2 || snapshot.QualityAttempts[0].ID != "00000" {
		t.Fatalf("snapshot problems=%d quality=%#v err=%v", len(snapshot.Problems), snapshot.QualityAttempts, err)
	}

	qualityProblems := map[string]model.Problem{}
	for _, problem := range snapshot.Problems {
		qualityProblems[problem.ID] = problem
	}
	qualityAttempts := make([]practice.Attempt, 0, len(snapshot.QualityAttempts))
	for _, attempt := range snapshot.QualityAttempts {
		problem := qualityProblems[attempt.ProblemID]
		qualityAttempts = append(qualityAttempts, practice.Attempt{ProblemID: attempt.ProblemID, Difficulty: practice.Difficulty(problem.Difficulty), Outcome: practice.Outcome(attempt.Outcome), Minutes: attempt.Minutes, AttemptedAt: attempt.AttemptedAt})
	}
	criteria := practice.CriteriaFor(practice.Request{Level: practice.NonStudent, QualityAttempts: qualityAttempts, CategoryExposure: snapshot.CategoryExposure, Goals: practice.DefaultGoals()})
	candidates, history, err := repository.RecommendationCandidates(context.Background(), "student", criteria, false, practice.DefaultGoals(), now.Add(30000*time.Second))
	if err != nil || len(candidates) != 1 || candidates[0].ID != "zzzzz-unseen" {
		t.Fatalf("candidate beyond previous 10k cap candidates=%#v err=%v", candidates, err)
	}

	problems := make([]practice.Problem, 0, len(candidates))
	for _, problem := range candidates {
		problems = append(problems, practice.Problem{ID: problem.ID, Difficulty: practice.Difficulty(problem.Difficulty)})
	}
	selected, err := practice.Select(practice.Request{UserID: "student", Level: practice.NonStudent, Problems: problems, QualityAttempts: qualityAttempts, ProblemHistory: history, CategoryExposure: snapshot.CategoryExposure, Goals: practice.DefaultGoals(), Now: now.Add(30000 * time.Second)})
	if err != nil || selected.Problem.ID != "zzzzz-unseen" {
		t.Fatalf("candidate beyond previous 10k cap result=%#v err=%v", selected, err)
	}
}

func TestMemoryMentoringRecommendationAndMockStateSurvivesAPIRestart(t *testing.T) {
	ctx := context.Background()
	repository := NewMemory()
	seasonStart := time.Now().UTC()
	repository.Users["mentor"] = accounts.User{ID: "mentor", AccountState: "active"}
	repository.Users["student"] = accounts.User{ID: "student", AccountState: "active"}
	repository.Problems["problem"] = model.Problem{ID: "problem", Number: 1, Title: "Problem", Link: "https://rsp.test/problem", Difficulty: "easy", Categories: []string{"arrays"}, Revision: 1}
	repository.Seasons["season"] = programme.SeasonRecord{ID: "season", Slug: "season", Status: "open", StartAt: seasonStart, EndAt: seasonStart.Add(14 * 24 * time.Hour), Revision: 1}
	for _, enrollment := range []model.Enrollment{
		{ID: "mentor-enrollment", SeasonID: "season", UserID: "mentor", Role: "mentor", State: "active", Revision: 1},
		{ID: "student-enrollment", SeasonID: "season", UserID: "student", Role: "student", State: "active", StudentLevel: "beginner", Revision: 1},
	} {
		if _, err := repository.CreateEnrollment(ctx, enrollment, "coordinator", time.Now()); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := repository.CreateWeek(ctx, model.Week{ID: "week", SeasonID: "season", Number: 1, StartAt: seasonStart, EndAt: seasonStart.Add(7 * 24 * time.Hour), Revision: 1}, "coordinator", time.Now()); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.CreateMentorship(ctx, programme.MentorshipRecord{ID: "mentorship", SeasonID: "season", MentorUserID: "mentor", StudentUserID: "student", Revision: 1}, "coordinator", time.Now()); err != nil {
		t.Fatal(err)
	}

	assigned, err := repository.IsMentorAssigned(ctx, "season", "mentor", "student")
	if err != nil || !assigned {
		t.Fatalf("assigned=%v err=%v", assigned, err)
	}

	recommendation := practice.Recommendation{ID: "recommendation", UserID: "student", Problem: practice.Problem{ID: "problem", Difficulty: practice.Easy}, Difficulty: practice.Easy, Rationale: "practice", RuleVersion: "v1", CreatedAt: time.Now().UTC()}
	if _, err := repository.SaveRecommendation(ctx, recommendation); err != nil {
		t.Fatal(err)
	}

	attempt := model.Attempt{ID: "attempt", UserID: "student", ProblemID: "problem", Revision: 1}
	fulfilled, err := repository.FulfillRecommendation(ctx, "student", attempt, time.Now())
	if err != nil || !fulfilled {
		t.Fatalf("fulfilled=%v err=%v", fulfilled, err)
	}
	if active, err := repository.GetActiveRecommendation(ctx, "student"); err != nil || active != nil {
		t.Fatalf("fulfilled recommendation remained active: %#v %v", active, err)
	}

	score := 7
	interview := mockinterviews.Interview{ID: "mock", InterviewerID: "mentor", IntervieweeID: "student", OccurredAt: time.Now().UTC(), DurationMinutes: 60, Rounds: []mockinterviews.Round{{ID: "round", Type: mockinterviews.Behavioural, Scores: mockinterviews.Scores{Behavioural: &score}}}, Revision: 1}
	if _, err := repository.CreateMockInterview(ctx, interview, "mentor", time.Now()); err != nil {
		t.Fatal(err)
	}

	interview.Notes = "updated"
	interview.Revision = 2
	if _, err := repository.UpdateMockInterview(ctx, interview, "mentor", "updated", time.Now()); err != nil {
		t.Fatal(err)
	}

	loaded, err := repository.GetMockInterview(ctx, "mock")
	if err != nil || loaded.Notes != "updated" || len(repository.MockVersions) != 2 {
		t.Fatalf("loaded=%#v versions=%d err=%v", loaded, len(repository.MockVersions), err)
	}
}

func TestMemoryMockEligibilityIsDerivedFromStoredRelationships(t *testing.T) {
	repository := NewMemory()
	repository.Users["eligible"] = accounts.User{ID: "eligible", AccountState: "active"}
	repository.Users["kicked"] = accounts.User{ID: "kicked", AccountState: "active"}
	repository.Users["test"] = accounts.User{ID: "test", AccountState: "active", IsTest: true}
	repository.Enrollments["eligible"] = model.Enrollment{ID: "eligible", UserID: "eligible", Role: "student", State: "completed"}
	repository.Enrollments["kicked"] = model.Enrollment{ID: "kicked", UserID: "kicked", Role: "student", State: "kicked"}
	repository.Enrollments["test"] = model.Enrollment{ID: "test", UserID: "test", Role: "mentor", State: "active"}

	for userID, want := range map[string]bool{"eligible": true, "kicked": false, "test": false} {
		participant, err := repository.GetMockParticipant(context.Background(), userID)
		if err != nil || participant.Eligible() != want {
			t.Fatalf("participant %s=%#v err=%v want eligible=%v", userID, participant, err, want)
		}
	}
}
