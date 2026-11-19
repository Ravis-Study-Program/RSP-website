//go:build integration

package api

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/Ravis-Study-Program/RSP-website/backend/internal/authz"
	"github.com/Ravis-Study-Program/RSP-website/backend/internal/dal"
	"github.com/Ravis-Study-Program/RSP-website/backend/internal/platform/id"
	"github.com/Ravis-Study-Program/RSP-website/backend/internal/program"
)

func TestSeasonDeletionRemovesTheSeasonAndEverythingInIt(t *testing.T) {
	f := newPostgresFixture(t)
	ctx := context.Background()
	at := time.Now()
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
	// A pending invitation too: the season has members, activity and one.
	token, hash, err := invitationToken()
	if err != nil || token == "" {
		t.Fatal(err)
	}
	if _, err = f.db.CreateInvitation(ctx, program.Invitation{ID: id.New(), SeasonID: seasonID, Name: "Invitee", Email: "invitee@example.test", Role: "student", DeliveryID: id.New(), ExpiresAt: at.Add(time.Hour)}, hash, otherID, at); err != nil {
		t.Fatal(err)
	}
	if res := testRequest(t, handler, http.MethodDelete, "/api/v2/seasons/"+seasonID, "admin", ""); res.Code != 204 {
		t.Fatalf("delete=%d %s", res.Code, res.Body)
	}
	if _, err = f.db.GetSeason(ctx, seasonID); !errors.Is(err, dal.ErrNotFound) {
		t.Fatalf("deleted season visible=%v", err)
	}
	var members, invites, weeks, audits int
	if err = f.pool.QueryRow(ctx, `SELECT
 (SELECT count(*) FROM app.enrollments WHERE season_id=$1 AND deleted_at IS NULL),
 (SELECT count(*) FROM app.season_invitations WHERE season_id=$1 AND accepted_at IS NULL AND cancelled_at IS NULL),
 (SELECT count(*) FROM app.season_weeks WHERE season_id=$1 AND deleted_at IS NULL),
 (SELECT count(*) FROM app.audit_events WHERE subject_id=$1::text AND action='season.deleted')`, seasonID).Scan(&members, &invites, &weeks, &audits); err != nil || members != 0 || invites != 0 || weeks != 0 || audits != 1 {
		t.Fatalf("members=%d invitations=%d weeks=%d audit=%d %v", members, invites, weeks, audits, err)
	}
	// Deleting it again finds nothing.
	if res := testRequest(t, handler, http.MethodDelete, "/api/v2/seasons/"+seasonID, "admin", ""); res.Code != 404 {
		t.Fatalf("second delete=%d", res.Code)
	}
}
