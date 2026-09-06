//go:build integration

package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/magedmg/RSP-website/backend/internal/authz"
	"github.com/magedmg/RSP-website/backend/internal/dal"
	"github.com/magedmg/RSP-website/backend/internal/mockinterviews"
	"github.com/magedmg/RSP-website/backend/internal/platform/id"
	"github.com/magedmg/RSP-website/backend/internal/practice"
	"github.com/magedmg/RSP-website/backend/internal/programme"
)

func activityTime(t *testing.T, value string) time.Time {
	t.Helper()
	v, err := time.Parse(time.RFC3339, value)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func TestActivitySeasonsFollowDatesAndParticipation(t *testing.T) {
	f := newPostgresFixture(t)
	ctx := context.Background()
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := f.pool.Exec(ctx, sql, args...); err != nil {
			t.Fatal(err)
		}
	}
	exec(`UPDATE app.seasons SET end_at='2026-02-15T23:59:59Z' WHERE id=$1`, seasonID)
	// Joining late must exclude earlier personal practice.
	exec(`UPDATE app.enrollment_participation SET started_at='2026-01-10T00:00:00Z' WHERE enrollment_id=$1`, studentMemberID)
	for _, date := range []string{"2026-01-09T12:00:00Z", "2026-01-12T12:00:00Z", "2026-02-18T12:00:00Z"} {
		if _, err := f.db.CreateAttempt(ctx, practice.AttemptRecord{ID: id.New(), UserID: studentID, ProblemID: leetcodeProblemID, Outcome: "independently_solved", Minutes: 20, AttemptedAt: activityTime(t, date)}); err != nil {
			t.Fatal(err)
		}
	}
	score := 7
	mock, err := f.db.CreateMockInterview(ctx, mockinterviews.Interview{ID: id.New(), InterviewerID: otherID, IntervieweeID: studentID, OccurredAt: activityTime(t, "2026-02-18T12:00:00Z"), DurationMinutes: 30, Rounds: []mockinterviews.Round{{ID: id.New(), Type: mockinterviews.Behavioural, Scores: mockinterviews.Scores{Behavioural: &score}}}}, otherID, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	count := func(wantAttempts, wantMocks int64) {
		t.Helper()
		_, _, attempts, err := f.db.ListAttempts(ctx, dal.AttemptQuery{UserID: studentID, SeasonID: seasonID, Year: 2026, Limit: 20})
		if err != nil {
			t.Fatal(err)
		}
		_, _, mocks, err := f.db.ListMockInterviews(ctx, dal.MockInterviewQuery{ViewerID: studentID, Visibility: dal.MockInterviewsReceived, SeasonID: seasonID, Year: 2026, Limit: 20})
		if err != nil {
			t.Fatal(err)
		}
		if attempts != wantAttempts || mocks != wantMocks {
			t.Fatalf("attempts=%d mocks=%d want=%d/%d", attempts, mocks, wantAttempts, wantMocks)
		}
	}
	count(1, 0)
	season, err := f.db.GetSeason(ctx, seasonID)
	if err != nil {
		t.Fatal(err)
	}
	season, err = f.db.UpdateSeasonDefinition(ctx, dal.UpdateSeasonDefinitionInput{SeasonID: seasonID, ActorID: otherID, ChangedAt: time.Now(), Name: season.Name, Slug: season.Slug, Location: season.Location, StartAt: season.StartAt, EndAt: activityTime(t, "2026-02-28T23:59:59Z")})
	if err != nil {
		t.Fatal(err)
	}
	count(2, 1)
	reloaded, err := f.db.GetMockInterview(ctx, mock.ID)
	if err != nil || reloaded.SeasonID == nil || *reloaded.SeasonID != seasonID {
		t.Fatalf("derived mock=%+v err=%v", reloaded, err)
	}
	// A kick changes the participation interval, not earlier records.
	if _, err := f.db.RemoveEnrollment(ctx, dal.RemoveEnrollmentInput{EnrollmentID: studentMemberID, ActorID: otherID, Reason: "left programme", ChangedAt: activityTime(t, "2026-02-10T00:00:00Z")}); err != nil {
		t.Fatal(err)
	}
	count(1, 0)
	_, _, personal, err := f.db.ListAttempts(ctx, dal.AttemptQuery{UserID: studentID, Year: 2026, Limit: 20})
	if err != nil || personal != 3 {
		t.Fatalf("personal=%d err=%v", personal, err)
	}
	// Year boundaries follow Adelaide, not UTC.
	if _, err := f.db.CreateAttempt(ctx, practice.AttemptRecord{ID: id.New(), UserID: studentID, ProblemID: leetcodeProblemID, Outcome: "independently_solved", Minutes: 20, AttemptedAt: activityTime(t, "2026-12-31T14:00:00Z")}); err != nil {
		t.Fatal(err)
	}
	_, _, nextYear, err := f.db.ListAttempts(ctx, dal.AttemptQuery{UserID: studentID, Year: 2027, Limit: 20})
	if err != nil || nextYear != 1 {
		t.Fatalf("next year=%d err=%v", nextYear, err)
	}
}

func TestMockRecordingTodayAndAfterLeaving(t *testing.T) {
	f := newPostgresFixture(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)
	body := func(at time.Time) string {
		return fmt.Sprintf(`{"interviewee":{"userId":%q},"occurredAt":%q,"durationMinutes":30,"rounds":[{"id":%q,"type":"behavioural","scores":{"behavioural":7}}]}`, studentID, at.Format(time.RFC3339), id.New())
	}
	for _, at := range []time.Time{now.Add(-48 * time.Hour), now.Add(time.Hour)} {
		res := testRequest(t, f.handler, http.MethodPost, "/api/v2/mock-interviews", "other", body(at))
		if res.Code != 400 {
			t.Fatalf("invalid time status=%d body=%s", res.Code, res.Body)
		}
	}
	if _, err := f.db.RemoveEnrollment(ctx, dal.RemoveEnrollmentInput{EnrollmentID: studentMemberID, ActorID: otherID, Reason: "left", ChangedAt: now.Add(-time.Hour)}); err != nil {
		t.Fatal(err)
	}
	res := testRequest(t, f.handler, http.MethodPost, "/api/v2/mock-interviews", "other", body(now))
	if res.Code != 201 {
		t.Fatalf("former participant status=%d body=%s", res.Code, res.Body)
	}
	var created mockInterviewResponse
	if err := json.Unmarshal(res.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.Interview.SeasonID != nil {
		t.Fatalf("post-kick mock counted in season: %+v", created.Interview)
	}
	// Feedback remains editable after closure; occurrence time cannot be moved.
	if _, err := f.pool.Exec(ctx, `UPDATE app.seasons SET status='closed',closed_at=now() WHERE id=$1`, seasonID); err != nil {
		t.Fatal(err)
	}
	update := func(at time.Time) string {
		body := mockUpdateBody(created.Interview)
		body["occurredAt"] = at
		body["durationMinutes"] = 35
		body["notes"] = "feedback"
		b, _ := json.Marshal(body)
		return string(b)
	}
	res = testRequest(t, f.handler, http.MethodPatch, "/api/v2/mock-interviews/"+created.Interview.ID, "other", update(now))
	if res.Code != 200 {
		t.Fatalf("feedback status=%d body=%s", res.Code, res.Body)
	}
	res = testRequest(t, f.handler, http.MethodPatch, "/api/v2/mock-interviews/"+created.Interview.ID, "other", update(now.Add(-time.Hour)))
	if res.Code != 400 {
		t.Fatalf("changed date status=%d body=%s", res.Code, res.Body)
	}
	// Legacy removals also carry a soft-delete marker. Preserve personal
	// access through the actual authentication resolver and /me memberships.
	if _, err := f.pool.Exec(ctx, `UPDATE app.enrollments SET deleted_at=now() WHERE id=$1`, studentMemberID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.pool.Exec(ctx, `INSERT INTO app.user_auth_links(auth_subject,user_id,provider,provider_account_id) VALUES('former-student',$1,'test','former-student')`, studentID); err != nil {
		t.Fatal(err)
	}
	handler := New(Config{DB: f.db, Authenticator: AuthenticatorFunc(func(ctx context.Context, _ string) (authz.Actor, error) {
		return f.db.ResolveAuthSubject(ctx, "former-student")
	})}).Handler()
	res = testRequest(t, handler, http.MethodGet, "/api/v2/me", "former", "")
	if res.Code != 200 || !strings.Contains(res.Body.String(), `"state":"kicked"`) {
		t.Fatalf("former me=%d %s", res.Code, res.Body)
	}
	res = testRequest(t, handler, http.MethodGet, "/api/v2/mock-interviews/participants", "former", "")
	if res.Code != 200 || !strings.Contains(res.Body.String(), studentID) {
		t.Fatalf("former participants=%d %s", res.Code, res.Body)
	}
	res = testRequest(t, handler, http.MethodPost, "/api/v2/mock-interviews", "former", strings.Replace(body(now), studentID, otherID, 1))
	if res.Code != 201 {
		t.Fatalf("legacy former recording=%d %s", res.Code, res.Body)
	}
	res = testRequest(t, handler, http.MethodGet, "/api/v2/mock-interviews", "former", "")
	if res.Code != 200 {
		t.Fatalf("former history status=%d body=%s", res.Code, res.Body)
	}
	res = testRequest(t, handler, http.MethodPost, "/api/v2/problem-attempts", "former", fmt.Sprintf(`{"problemId":%q,"outcome":"independently_solved","minutes":20,"attemptedAt":%q}`, leetcodeProblemID, now.Format(time.RFC3339)))
	if res.Code != 201 {
		t.Fatalf("former practice status=%d body=%s", res.Code, res.Body)
	}
	res = testRequest(t, handler, http.MethodGet, "/api/v2/users", "former", "")
	if res.Code != 403 {
		t.Fatalf("community directory must remain restricted, status=%d", res.Code)
	}
}

func TestActivityScopeRoleChangesAndOverlappingSeasons(t *testing.T) {
	f := newPostgresFixture(t)
	ctx := context.Background()
	scope := func(date, want string) {
		t.Helper()
		var got string
		err := f.pool.QueryRow(ctx, `SELECT COALESCE((SELECT season_id::text FROM app.activity_scope($1,$2)),'')`, studentID, activityTime(t, date)).Scan(&got)
		if err != nil || got != want {
			t.Fatalf("scope at %s=%q want=%q err=%v", date, got, want, err)
		}
	}
	if _, err := f.db.PromoteEnrollment(ctx, dal.PromoteEnrollmentInput{EnrollmentID: studentMemberID, Role: "mentor", ActorID: otherID, ChangedAt: activityTime(t, "2026-02-01T00:00:00Z")}); err != nil {
		t.Fatal(err)
	}
	scope("2026-01-31T23:59:59Z", seasonID)
	scope("2026-02-01T00:00:00Z", "")
	if _, err := f.db.UpdateEnrollmentRoleAndLevel(ctx, dal.UpdateEnrollmentRoleAndLevelInput{SeasonID: seasonID, EnrollmentID: studentMemberID, Role: "student", StudentLevel: "beginner", ActorID: otherID, ChangedAt: activityTime(t, "2026-03-01T00:00:00Z")}); err != nil {
		t.Fatal(err)
	}
	scope("2026-02-28T23:59:59Z", "")
	scope("2026-03-01T00:00:00Z", seasonID)
	if _, err := f.db.RemoveEnrollment(ctx, dal.RemoveEnrollmentInput{EnrollmentID: studentMemberID, ActorID: otherID, Reason: "left", ChangedAt: activityTime(t, "2026-04-01T00:00:00Z")}); err != nil {
		t.Fatal(err)
	}
	scope("2026-04-01T00:00:00Z", "")
	if _, err := f.pool.Exec(ctx, `UPDATE app.enrollments SET state='active',assignment_state='active',activated_at='2026-05-01',state_changed_at='2026-05-01' WHERE id=$1`, studentMemberID); err != nil {
		t.Fatal(err)
	}
	scope("2026-04-30T23:59:59Z", "")
	scope("2026-05-01T00:00:00Z", seasonID)
	nextSeason := id.New()
	if _, err := f.pool.Exec(ctx, `INSERT INTO app.seasons(id,name,slug,start_at,end_at,location) VALUES($1,'2026/27 Summer','next-summer','2026-11-01','2027-02-28','Adelaide')`, nextSeason); err != nil {
		t.Fatal(err)
	}
	if _, err := f.pool.Exec(ctx, `INSERT INTO app.enrollments(id,user_id,season_id,role,student_level,state,created_at) VALUES($1,$2,$3,'student','beginner','active','2026-11-10')`, id.New(), studentID, nextSeason); err != nil {
		t.Fatal(err)
	}
	scope("2026-11-09T23:59:59Z", seasonID)
	scope("2026-11-10T00:00:00Z", nextSeason)
	scope("2027-03-01T00:00:00Z", "")
}

func TestActivityFiltersBindCursorsAndPrivateHistory(t *testing.T) {
	f := newPostgresFixture(t)
	ctx := context.Background()
	if _, err := f.db.CreateMentorship(ctx, programme.MentorshipRecord{ID: id.New(), SeasonID: seasonID, MentorUserID: otherID, StudentUserID: studentID}, otherID, time.Now()); err != nil {
		t.Fatal(err)
	}
	if _, err := f.pool.Exec(ctx, `UPDATE app.mentorships SET created_at='2026-01-01'`); err != nil {
		t.Fatal(err)
	}
	for _, date := range []string{"2026-02-01T00:00:00Z", "2026-03-01T00:00:00Z", "2027-01-01T00:00:00Z"} {
		if _, err := f.db.CreateAttempt(ctx, practice.AttemptRecord{ID: id.New(), UserID: studentID, ProblemID: leetcodeProblemID, Outcome: "independently_solved", Minutes: 20, Notes: "private feedback", AttemptedAt: activityTime(t, date)}); err != nil {
			t.Fatal(err)
		}
	}
	res := testRequest(t, f.handler, http.MethodGet, "/api/v2/problem-attempts?userId="+studentID, "other", "")
	var history Page[practice.AttemptRecord]
	if err := json.Unmarshal(res.Body.Bytes(), &history); err != nil || res.Code != 200 {
		t.Fatalf("history status=%d err=%v body=%s", res.Code, err, res.Body)
	}
	for _, attempt := range history.Items {
		want := "private feedback"
		if attempt.AttemptedAt.Year() == 2027 {
			want = ""
		}
		if attempt.Notes != want {
			t.Fatalf("private notes at %s=%q", attempt.AttemptedAt, attempt.Notes)
		}
	}
	res = testRequest(t, f.handler, http.MethodGet, "/api/v2/problem-attempts?limit=1&year=2026&seasonId="+seasonID, "student", "")
	var first Page[practice.AttemptRecord]
	if err := json.Unmarshal(res.Body.Bytes(), &first); err != nil || res.Code != 200 || first.TotalCount != 2 {
		t.Fatalf("first=%+v status=%d err=%v", first, res.Code, err)
	}
	if first.PageInfo.NextCursor == nil {
		t.Fatal("missing next cursor")
	}
	res = testRequest(t, f.handler, http.MethodGet, "/api/v2/problem-attempts?limit=1&year=2027&seasonId="+seasonID+"&cursor="+url.QueryEscape(*first.PageInfo.NextCursor), "student", "")
	if res.Code != 400 {
		t.Fatalf("cross-year cursor accepted: %d %s", res.Code, res.Body)
	}
	for _, path := range []string{"/api/v2/problem-attempts?year=nope", "/api/v2/mock-interviews?year=1800", "/api/v2/problem-attempts?seasonId=broken"} {
		res = testRequest(t, f.handler, http.MethodGet, path, "student", "")
		if res.Code != 400 {
			t.Fatalf("invalid filter status=%d body=%s", res.Code, res.Body)
		}
	}
}
