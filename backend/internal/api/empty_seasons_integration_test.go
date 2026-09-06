//go:build integration

package api

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/magedmg/RSP-website/backend/internal/authz"
	"github.com/magedmg/RSP-website/backend/internal/dal"
	"github.com/magedmg/RSP-website/backend/internal/platform/id"
	"github.com/magedmg/RSP-website/backend/internal/programme"
)

func TestEmptySeasonDeletionPreservesHistoryAndSerializesEnrollment(t *testing.T) {
	f := newPostgresFixture(t)
	ctx := context.Background()
	at := time.Now()
	newSeason := func() string {
		t.Helper()
		season := id.New()
		if _, err := f.pool.Exec(ctx, `INSERT INTO app.seasons(id,slug,name,start_at,end_at) VALUES($1,$2,'Empty season','2026-01-01','2026-12-31')`, season, season); err != nil {
			t.Fatal(err)
		}
		return season
	}
	handler := New(Config{DB: f.db, Authenticator: AuthenticatorFunc(func(_ context.Context, token string) (authz.Actor, error) {
		actor := authz.Actor{UserID: otherID, EmailVerified: true, AccountState: authz.AccountActive}
		if token != "student" {
			actor.GlobalRoles = map[authz.GlobalRole]bool{authz.Director: true}
		}
		if token == "admin" {
			actor.MFAAt = &at
		}
		return actor, nil
	})}).Handler()
	for _, token := range []string{"student", "without-mfa"} {
		if res := testRequest(t, handler, http.MethodDelete, "/api/v2/seasons/"+seasonID, token, ""); res.Code != 403 {
			t.Fatalf("unauthorized deletion=%d", res.Code)
		}
	}
	if err := f.db.DeleteEmptySeason(ctx, seasonID, otherID, at); !errors.Is(err, dal.ErrConflict) {
		t.Fatalf("populated season deletion=%v", err)
	}
	if _, err := f.db.RemoveEnrollment(ctx, dal.RemoveEnrollmentInput{EnrollmentID: studentMemberID, ActorID: otherID, Reason: "left", ChangedAt: at}); err != nil {
		t.Fatal(err)
	}
	if err := f.db.DeleteEmptySeason(ctx, seasonID, otherID, at); !errors.Is(err, dal.ErrConflict) {
		t.Fatalf("historical season deletion=%v", err)
	}
	season := newSeason()
	token, hash, err := invitationToken()
	if err != nil || token == "" {
		t.Fatal(err)
	}
	invite, err := f.db.CreateInvitation(ctx, programme.Invitation{ID: id.New(), SeasonID: season, Name: "Invitee", Email: "invitee@example.test", Role: "student", DeliveryID: id.New(), ExpiresAt: at.Add(time.Hour)}, hash, otherID, at)
	if err != nil {
		t.Fatal(err)
	}
	if err = f.db.DeleteEmptySeason(ctx, season, otherID, at); !errors.Is(err, dal.ErrConflict) {
		t.Fatalf("pending invitation deletion=%v", err)
	}
	if _, err = f.db.ChangeInvitation(ctx, season, invite.ID, "", "", otherID, true, at); err != nil {
		t.Fatal(err)
	}
	if _, err = f.db.CreateWeek(ctx, programme.WeekRecord{ID: id.New(), SeasonID: season, Number: 1, StartAt: activityTime(t, "2026-01-01T00:00:00Z"), EndAt: activityTime(t, "2026-01-07T00:00:00Z")}, otherID, at); err != nil {
		t.Fatal(err)
	}
	if res := testRequest(t, handler, http.MethodDelete, "/api/v2/seasons/"+season, "admin", ""); res.Code != 204 {
		t.Fatalf("delete=%d %s", res.Code, res.Body)
	}
	if _, err = f.db.GetSeason(ctx, season); !errors.Is(err, dal.ErrNotFound) {
		t.Fatalf("deleted season visible=%v", err)
	}
	var weeks, audits int
	if err = f.pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM app.season_weeks WHERE season_id=$1 AND deleted_at IS NULL),(SELECT count(*) FROM app.audit_events WHERE subject_id=$1::text AND action='season.deleted')`, season).Scan(&weeks, &audits); err != nil || weeks != 0 || audits != 1 {
		t.Fatalf("weeks=%d audit=%d %v", weeks, audits, err)
	}
	// Either enrollment wins and deletion refuses, or deletion wins and enrollment refuses.
	for range 3 {
		raceSeason := newSeason()
		var wg sync.WaitGroup
		errs := make(chan error, 2)
		wg.Add(2)
		go func() { defer wg.Done(); errs <- f.db.DeleteEmptySeason(ctx, raceSeason, otherID, at) }()
		go func() {
			defer wg.Done()
			_, err := f.db.CreateEnrollment(ctx, programme.EnrollmentRecord{ID: id.New(), SeasonID: raceSeason, UserID: otherID, Role: "student", State: "active"}, otherID, at)
			errs <- err
		}()
		wg.Wait()
		close(errs)
		successes := 0
		for err := range errs {
			if err == nil {
				successes++
			} else if !errors.Is(err, dal.ErrConflict) && !errors.Is(err, dal.ErrNotFound) {
				t.Fatal(err)
			}
		}
		if successes != 1 {
			t.Fatalf("concurrent successful mutations=%d", successes)
		}
	}
}
