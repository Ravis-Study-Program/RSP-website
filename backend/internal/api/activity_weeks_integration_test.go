//go:build integration

package api

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/magedmg/RSP-website/backend/internal/authz"
	"github.com/magedmg/RSP-website/backend/internal/mockinterviews"
	"github.com/magedmg/RSP-website/backend/internal/platform/id"
	"github.com/magedmg/RSP-website/backend/internal/programme"
)

func TestMockWeekSelectionRecalculatesDatesAndBindsCursors(t *testing.T) {
	f := newPostgresFixture(t)
	ctx := context.Background()
	weekOne, weekTwo := id.New(), id.New()
	if _, err := f.pool.Exec(ctx, `INSERT INTO app.season_weeks(id,season_id,week_number,start_at,end_at) VALUES
		($1,$3,1,'2026-01-01','2026-01-07T23:59:59Z'),
		($2,$3,2,'2026-01-08','2026-01-14T23:59:59Z')`, weekOne, weekTwo, seasonID); err != nil {
		t.Fatal(err)
	}
	for _, at := range []string{"2026-01-02T12:00:00Z", "2026-01-03T12:00:00Z", "2026-01-09T12:00:00Z"} {
		score := 8
		interview := mockinterviews.Interview{
			ID: id.New(), InterviewerID: otherID, IntervieweeID: studentID,
			OccurredAt: activityTime(t, at), DurationMinutes: 30,
			Rounds: []mockinterviews.Round{{ID: id.New(), Type: mockinterviews.LeetCode, ProblemID: leetcodeProblemID,
				Scores: mockinterviews.Scores{ConfirmQuestions: &score, AlgorithmDesign: &score, ComplexityAnalysis: &score, Coding: &score, Testing: &score}}},
		}
		if _, err := f.db.CreateMockInterview(ctx, interview, otherID, time.Now()); err != nil {
			t.Fatal(err)
		}
	}
	read := func(path string, count int) Page[mockinterviews.Interview] {
		t.Helper()
		response := testRequest(t, f.handler, http.MethodGet, path, "student", "")
		var result Page[mockinterviews.Interview]
		if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil || response.Code != 200 || result.TotalCount != int64(count) {
			t.Fatalf("mocks=%d %s error=%v", response.Code, response.Body, err)
		}
		return result
	}
	base := "/api/v2/mock-interviews?seasonId=" + seasonID
	path := base + "&weekId=" + weekOne + "&limit=1&passed=true&interviewerId=" + otherID
	first := read(path, 2)
	if len(first.Items) != 1 || first.Items[0].WeekID == nil || *first.Items[0].WeekID != weekOne || first.PageInfo.NextCursor == nil {
		t.Fatalf("first week page=%+v", first)
	}
	second := read(path+"&cursor="+*first.PageInfo.NextCursor, 2)
	if len(second.Items) != 1 || second.Items[0].ID == first.Items[0].ID {
		t.Fatalf("next week page=%+v", second)
	}
	read(base+"&weekId="+weekOne+"&weekId="+weekTwo, 3)
	read(base+"&weekId="+weekOne+"&passed=false", 0)
	for _, invalid := range []string{
		path + "&weekId=" + weekTwo + "&cursor=" + *first.PageInfo.NextCursor,
		"/api/v2/mock-interviews?weekId=" + weekOne,
		base + "&weekId=invalid",
	} {
		if response := testRequest(t, f.handler, http.MethodGet, invalid, "student", ""); response.Code != 400 {
			t.Fatalf("invalid week selection=%d %s", response.Code, response.Body)
		}
	}
	if _, err := f.pool.Exec(ctx, `UPDATE app.season_weeks SET end_at='2026-01-02T23:59:59Z' WHERE id=$1`, weekOne); err != nil {
		t.Fatal(err)
	}
	if _, err := f.pool.Exec(ctx, `UPDATE app.season_weeks SET start_at='2026-01-03T00:00:00Z' WHERE id=$1`, weekTwo); err != nil {
		t.Fatal(err)
	}
	read(base+"&weekId="+weekOne, 1)
	for _, interview := range read(base+"&weekId="+weekTwo, 2).Items {
		if interview.WeekID == nil || *interview.WeekID != weekTwo {
			t.Fatalf("week did not follow edited dates: %+v", interview)
		}
	}
}

func TestFormerMembersCanReadOwnWeekDatesWithoutResourceLinks(t *testing.T) {
	f := newPostgresFixture(t)
	if _, err := f.pool.Exec(context.Background(), `INSERT INTO app.season_weeks(id,season_id,week_number,start_at,end_at,resource_url)
		VALUES($1,$2,1,'2026-01-01','2026-01-07','https://example.test/private-week')`, id.New(), seasonID); err != nil {
		t.Fatal(err)
	}
	for _, state := range []authz.EnrollmentState{authz.Active, authz.Kicked, authz.Withdrawn} {
		t.Run(string(state), func(t *testing.T) {
			actor := authz.Actor{UserID: studentID, EmailVerified: true, AccountState: authz.AccountActive,
				Enrollments: []authz.Enrollment{{SeasonID: seasonID, Role: authz.Student, State: state}}}
			handler := New(Config{DB: f.db, CursorSecret: []byte("0123456789abcdef"), Authenticator: AuthenticatorFunc(func(context.Context, string) (authz.Actor, error) {
				return actor, nil
			})}).Handler()
			response := testRequest(t, handler, http.MethodGet, "/api/v2/seasons/"+seasonID+"/weeks", "student", "")
			var result Page[programme.WeekRecord]
			if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil || response.Code != 200 || len(result.Items) != 1 || result.Items[0].StartAt.IsZero() {
				t.Fatalf("week metadata=%d %s error=%v", response.Code, response.Body, err)
			}
			if (result.Items[0].ResourceURL != "") != (state == authz.Active) {
				t.Fatalf("resource visibility for %s: %+v", state, result.Items[0])
			}
			if response := testRequest(t, handler, http.MethodGet, "/api/v2/seasons/"+id.New()+"/weeks", "student", ""); state != authz.Active && response.Code != 403 {
				t.Fatalf("unrelated season weeks=%d %s", response.Code, response.Body)
			}
		})
	}
}
