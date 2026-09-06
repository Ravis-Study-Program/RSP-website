//go:build integration

package api

import (
	"context"
	"encoding/json"
	"github.com/magedmg/RSP-website/backend/internal/authz"
	"github.com/magedmg/RSP-website/backend/internal/dal"
	"github.com/magedmg/RSP-website/backend/internal/platform/id"
	"github.com/magedmg/RSP-website/backend/internal/practice"
	"net/http"
	"testing"
	"time"
)

func TestEnrollmentCorrectionRetainsDatesAndActivityOwners(t *testing.T) {
	f := newPostgresFixture(t)
	ctx := context.Background()
	targetSeason := id.New()
	at := time.Now()
	for _, q := range []struct {
		sql  string
		args []any
	}{
		{`INSERT INTO app.seasons(id,slug,name,start_at,end_at) VALUES($1,'correct-season','Correct season','2026-01-01','2026-12-31')`, []any{targetSeason}},
		{`INSERT INTO app.user_auth_links(auth_subject,user_id,provider,provider_account_id) VALUES('correct-other',$1,'test','correct-other'),('correct-student',$2,'test','correct-student')`, []any{otherID, studentID}},
		{`INSERT INTO app.mentorships(id,season_id,mentor_enrollment_id,student_enrollment_id) VALUES($1,$2,$3,$4)`, []any{id.New(), seasonID, otherMemberID, studentMemberID}},
	} {
		if _, err := f.pool.Exec(ctx, q.sql, q.args...); err != nil {
			t.Fatal(err)
		}
	}
	attempt, err := f.db.CreateAttempt(ctx, practice.AttemptRecord{ID: id.New(), UserID: studentID, ProblemID: leetcodeProblemID, Outcome: "not_solved", Minutes: 10, AttemptedAt: activityTime(t, "2026-02-01T12:00:00Z")})
	if err != nil {
		t.Fatal(err)
	}
	var before time.Time
	if err = f.pool.QueryRow(ctx, `SELECT started_at FROM app.enrollment_participation WHERE enrollment_id=$1`, studentMemberID).Scan(&before); err != nil {
		t.Fatal(err)
	}
	handler := New(Config{DB: f.db, Authenticator: AuthenticatorFunc(func(_ context.Context, token string) (authz.Actor, error) {
		actor := authz.Actor{UserID: otherID, EmailVerified: true, AccountState: authz.AccountActive, MFAAt: &at}
		if token == "admin" {
			actor.GlobalRoles = map[authz.GlobalRole]bool{authz.SystemAdmin: true}
		}
		return actor, nil
	})}).Handler()
	correct := func(user, season, actor string) int {
		body, _ := json.Marshal(map[string]string{"userId": user, "seasonId": season})
		return testRequest(t, handler, http.MethodPost, "/api/v2/seasons/"+seasonID+"/members/"+studentMemberID+"/correction", actor, string(body)).Code
	}
	if status := correct(otherID, seasonID, "admin"); status != 409 {
		t.Fatalf("duplicate=%d", status)
	}
	if status := correct(studentID, targetSeason, "mentor"); status != 403 {
		t.Fatalf("unauthorized=%d", status)
	}
	if _, err = f.pool.Exec(ctx, `UPDATE app.seasons SET status='closed',closed_at=now() WHERE id=$1`, targetSeason); err != nil {
		t.Fatal(err)
	}
	if status := correct(studentID, targetSeason, "admin"); status != 409 {
		t.Fatalf("closed target=%d", status)
	}
	if _, err = f.pool.Exec(ctx, `UPDATE app.seasons SET status='open',closed_at=NULL WHERE id=$1`, targetSeason); err != nil {
		t.Fatal(err)
	}
	if status := correct(studentID, targetSeason, "admin"); status != 200 {
		t.Fatalf("correct season=%d", status)
	}
	_, _, count, err := f.db.ListAttempts(ctx, dal.AttemptQuery{UserID: studentID, SeasonID: targetSeason, Limit: 10})
	if err != nil || count != 1 {
		t.Fatalf("recalculated scope=%d %v", count, err)
	}
	var after time.Time
	var assignments int
	if err = f.pool.QueryRow(ctx, `SELECT started_at,(SELECT count(*) FROM app.mentorships WHERE deleted_at IS NULL) FROM app.enrollment_participation WHERE enrollment_id=$1`, studentMemberID).Scan(&after, &assignments); err != nil || !before.Equal(after) || assignments != 0 {
		t.Fatalf("history=%v/%v assignments=%d error=%v", before, after, assignments, err)
	}
	if _, err = f.db.CorrectEnrollment(ctx, dal.CorrectEnrollmentInput{EnrollmentID: studentMemberID, SourceSeasonID: targetSeason, SeasonID: targetSeason, UserID: otherID, ActorID: otherID, ChangedAt: at}); err != nil {
		t.Fatal(err)
	}
	var owner string
	if err = f.pool.QueryRow(ctx, `SELECT user_id FROM app.problem_attempts WHERE id=$1`, attempt.ID).Scan(&owner); err != nil || owner != studentID {
		t.Fatalf("owner changed=%s %v", owner, err)
	}
	_, _, count, err = f.db.ListAttempts(ctx, dal.AttemptQuery{UserID: otherID, SeasonID: targetSeason, Limit: 10})
	if err != nil || count != 1 {
		t.Fatalf("new member own pre-existing history=%d %v", count, err)
	}
	// The new owner's own fixture attempt falls into the retained participation interval.
}
