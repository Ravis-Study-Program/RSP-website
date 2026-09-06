//go:build integration

package api

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/magedmg/RSP-website/backend/internal/dal"
	"github.com/magedmg/RSP-website/backend/internal/platform/id"
	"github.com/magedmg/RSP-website/backend/internal/practice"
	"github.com/magedmg/RSP-website/backend/internal/programme"
)

func TestSeasonSummariesFollowDateChangesAndRetainDepartedMembers(t *testing.T) {
	f := newPostgresFixture(t)
	ctx := context.Background()
	if _, err := f.pool.Exec(ctx, `UPDATE app.seasons SET end_at='2026-02-28T23:59:59Z' WHERE id=$1`, seasonID); err != nil {
		t.Fatal(err)
	}
	for _, date := range []string{"2026-02-01T00:00:00Z", "2026-03-01T00:00:00Z"} {
		if _, err := f.db.CreateAttempt(ctx, practice.AttemptRecord{ID: id.New(), UserID: studentID, ProblemID: leetcodeProblemID, Outcome: "independently_solved", Minutes: 20, AttemptedAt: activityTime(t, date)}); err != nil {
			t.Fatal(err)
		}
	}
	read := func(want int64) {
		t.Helper()
		for _, path := range []string{"/api/v2/users/" + studentID + "/activity-summary?seasonId=" + seasonID, "/api/v2/seasons/" + seasonID + "/activity-summary"} {
			res := testRequest(t, f.handler, http.MethodGet, path, "other", "")
			var summary programme.ActivitySummary
			if err := json.Unmarshal(res.Body.Bytes(), &summary); err != nil || res.Code != 200 || summary.AttemptCount != want || summary.LastActivityAt == nil {
				t.Fatalf("summary %s: %d %s %v", path, res.Code, res.Body, err)
			}
		}
	}
	read(1)
	if _, err := f.pool.Exec(ctx, `UPDATE app.seasons SET end_at='2026-03-31T23:59:59Z' WHERE id=$1`, seasonID); err != nil {
		t.Fatal(err)
	}
	read(2)
	if _, err := f.db.RemoveEnrollment(ctx, dal.RemoveEnrollmentInput{EnrollmentID: studentMemberID, ActorID: otherID, Reason: "left", ChangedAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	read(2)
	res := testRequest(t, f.handler, http.MethodGet, "/api/v2/seasons/"+seasonID+"/members", "other", "")
	var members Page[programme.EnrollmentRecord]
	if err := json.Unmarshal(res.Body.Bytes(), &members); err != nil || res.Code != 200 {
		t.Fatalf("members: %d %s %v", res.Code, res.Body, err)
	}
	for _, member := range members.Items {
		if member.UserID == studentID {
			if member.State != "kicked" || member.Member == nil || member.Member.Activity.AttemptCount != 2 || member.RemovalReason != nil {
				t.Fatalf("incorrect historical roster: %+v", member)
			}
			return
		}
	}
	t.Fatal("departed student disappeared from the season")
}
