//go:build integration

package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/magedmg/RSP-website/backend/internal/dal"
	"github.com/magedmg/RSP-website/backend/internal/mockinterviews"
	"github.com/magedmg/RSP-website/backend/internal/platform/id"
)

type mockReadQueryCounter struct{ statements []string }

func (c *mockReadQueryCounter) TraceQueryStart(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	c.statements = append(c.statements, data.SQL)
	return ctx
}
func (*mockReadQueryCounter) TraceQueryEnd(context.Context, *pgx.Conn, pgx.TraceQueryEndData) {}
func countedMockRepository(t *testing.T, fixture postgresFixture) (*dal.Store, *mockReadQueryCounter) {
	t.Helper()
	counter := &mockReadQueryCounter{}
	config := fixture.pool.Config().Copy()
	config.ConnConfig.Tracer = counter
	pool, err := pgxpool.NewWithConfig(context.Background(), config)
	if err != nil {
		t.Fatal(err)
	}
	store := dal.New(pool)
	t.Cleanup(store.Close)
	return store, counter
}

func TestMockListUsesBoundedReadsAndPreservesPageValues(t *testing.T) {
	fixture := newPostgresFixture(t)
	ctx := context.Background()
	base := createMockForWrites(t, fixture)
	interviews := []mockinterviews.Interview{base}
	for index := 1; index < 4; index++ {
		interview := base
		interview.ID = id.New()
		interview.OccurredAt = base.OccurredAt.Add(time.Duration(index/2) * time.Hour)
		interview.Rounds = append([]mockinterviews.Round(nil), base.Rounds...)
		for round := range interview.Rounds {
			interview.Rounds[round].ID = id.New()
		}
		created, err := fixture.db.CreateMockInterview(ctx, interview, studentID, time.Now())
		if err != nil {
			t.Fatal(err)
		}
		interviews = append(interviews, created)
	}
	// Include interviewee-owned values in the equality checks against detail reads.
	base.Rounds[0].Reviewed = true
	base.Rounds[0].IntervieweeComment = "Thanks for the interview"
	if _, err := fixture.db.ReviewMockInterviewRound(ctx, dal.ReviewMockInterviewRoundInput{
		Interview: base,
		RoundID:   base.Rounds[0].ID,
		ActorID:   otherID,
		ChangedAt: time.Now(),
	}); err != nil {
		t.Fatal(err)
	}

	repository, queries := countedMockRepository(t, fixture)

	for _, limit := range []int{1, 4} {
		queries.statements = nil
		items, more, total, err := repository.ListMockInterviews(ctx, dal.MockInterviewQuery{ViewerID: studentID,
			Visibility: dal.MockInterviewsGiven,
			Limit:      limit,
			SortBy:     "occurredAt:desc",
			Direction:  "forward",
		})
		if err != nil || total != 4 || len(items) != limit || more != (limit < 4) {
			t.Fatalf("limit=%d items=%d more=%v total=%d error=%v", limit, len(items), more, total, err)
		}
		if len(queries.statements) != 3 {
			t.Fatalf("limit=%d used %d queries instead of count/page/rounds: %v", limit, len(queries.statements), queries.statements)
		}
		for _, item := range items {
			stored, err := fixture.db.GetMockInterview(ctx, item.ID)
			if err != nil || !reflect.DeepEqual(stored, item) {
				t.Fatalf("batch/detail mismatch: batch=%#v detail=%#v error=%v", item, stored, err)
			}
		}
	}

	sort.Slice(interviews, func(i, j int) bool {
		if interviews[i].OccurredAt.Equal(interviews[j].OccurredAt) {
			return interviews[i].ID > interviews[j].ID
		}
		return interviews[i].OccurredAt.After(interviews[j].OccurredAt)
	})
	first, more, total, err := repository.ListMockInterviews(ctx, dal.MockInterviewQuery{ViewerID: studentID,
		Visibility: dal.MockInterviewsGiven,
		Limit:      2,
		SortBy:     "occurredAt:desc",
		Direction:  "forward",
	})
	if err != nil || !more || total != 4 || len(first) != 2 || first[0].ID != interviews[0].ID || first[1].ID != interviews[1].ID {
		t.Fatalf("first page=%#v more=%v total=%d error=%v", first, more, total, err)
	}
	second, more, _, err := repository.ListMockInterviews(ctx, dal.MockInterviewQuery{ViewerID: studentID,
		Visibility: dal.MockInterviewsGiven,
		Boundary:   first[1].ID,
		Limit:      2,
		SortBy:     "occurredAt:desc",
		Direction:  "forward",
	})
	if err != nil || more || len(second) != 2 || second[0].ID != interviews[2].ID || second[1].ID != interviews[3].ID {
		t.Fatalf("second page=%#v more=%v error=%v", second, more, err)
	}
	previous, more, _, err := repository.ListMockInterviews(ctx, dal.MockInterviewQuery{ViewerID: studentID,
		Visibility: dal.MockInterviewsGiven,
		Boundary:   second[0].ID,
		Limit:      2,
		SortBy:     "occurredAt:desc",
		Direction:  "backward",
	})
	if err != nil || more || !reflect.DeepEqual(first, previous) {
		t.Fatalf("backward page=%#v more=%v error=%v", previous, more, err)
	}
	queries.statements = nil
	items, more, total, err := repository.ListMockInterviews(ctx, dal.MockInterviewQuery{ViewerID: studentID,
		Visibility: dal.MockInterviewsReceived,
		Limit:      4,
		SortBy:     "id:asc",
		Direction:  "forward",
	})
	if err != nil || more || total != 0 || len(items) != 0 || len(queries.statements) != 2 {
		t.Fatalf("empty received page=%#v more=%v total=%d queries=%d error=%v", items, more, total, len(queries.statements), err)
	}
}

func TestMockSummaryCountsTheListScopeAcrossPages(t *testing.T) {
	fixture := newPostgresFixture(t)
	ctx := context.Background()
	base := createMockForWrites(t, fixture)
	for index := 1; index < 4; index++ {
		interview := base
		interview.ID = id.New()
		if index < 3 {
			interview.OccurredAt = base.OccurredAt.Add(-48 * time.Hour)
		}
		interview.Rounds = append([]mockinterviews.Round(nil), base.Rounds...)
		for round := range interview.Rounds {
			interview.Rounds[round].ID = id.New()
		}
		if _, err := fixture.db.CreateMockInterview(ctx, interview, studentID, time.Now()); err != nil {
			t.Fatal(err)
		}
	}
	// An interview counts as reviewed only when every live round is reviewed.
	if _, err := fixture.pool.Exec(ctx, `UPDATE app.mock_interview_rounds SET review_status='reviewed', reviewed_at=now() WHERE mock_interview_id=$1`, base.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.pool.Exec(ctx, `UPDATE app.mock_interview_rounds SET review_status='reviewed', reviewed_at=now()
		WHERE id=(SELECT r.id FROM app.mock_interview_rounds r WHERE r.mock_interview_id<>$1 ORDER BY r.id LIMIT 1)`, base.ID); err != nil {
		t.Fatal(err)
	}

	adelaide, err := time.LoadLocation("Australia/Adelaide")
	if err != nil {
		t.Fatal(err)
	}
	day := func(at time.Time) string { return at.In(adelaide).Format(time.DateOnly) }
	summary := func(actor, query string) mockinterviews.Summary {
		t.Helper()
		response := testRequest(t, fixture.handler, http.MethodGet, "/api/v2/mock-interviews/summary?"+query, actor, "")
		if response.Code != http.StatusOK {
			t.Fatalf("summary %s: status=%d body=%s", query, response.Code, response.Body.String())
		}
		var result mockinterviews.Summary
		if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		return result
	}

	given := summary("student", "mode=given")
	want := mockinterviews.Summary{Total: 4, Given: 4, Reviewed: 1, Days: []mockinterviews.DayCount{
		{Date: day(base.OccurredAt.Add(-48 * time.Hour)), Count: 2},
		{Date: day(base.OccurredAt), Count: 2},
	}, Interviewers: []mockinterviews.ParticipantSummary{{ID: studentID, Slug: "student", Name: "Student"}}}
	if !reflect.DeepEqual(given, want) {
		t.Fatalf("given summary=%#v want=%#v", given, want)
	}
	if received := summary("student", "mode=received"); received.Total != 0 || len(received.Days) != 0 {
		t.Fatalf("interviewer has no received interviews: %#v", received)
	}
	if own := summary("other", "mode=all&userId="+otherID); own.Total != 4 || own.Received != 4 || own.Given != 0 {
		t.Fatalf("user-scoped summary must count that user's side: %#v", own)
	}
	// Filters are applied server-side exactly as the list applies them.
	if failed := summary("student", "mode=given&passed=false"); failed.Total != 0 {
		t.Fatalf("passed filter ignored: %#v", failed)
	}
	if bad := testRequest(t, fixture.handler, http.MethodGet, "/api/v2/mock-interviews/summary?mode=everyone", "student", ""); bad.Code != http.StatusBadRequest {
		t.Fatalf("invalid mode status=%d", bad.Code)
	}

	page := testRequest(t, fixture.handler, http.MethodGet, "/api/v2/mock-interviews?mode=given&limit=1", "student", "")
	var list struct {
		TotalCount int64 `json:"totalCount"`
	}
	if err := json.Unmarshal(page.Body.Bytes(), &list); err != nil || list.TotalCount != given.Total {
		t.Fatalf("list total=%d summary total=%d error=%v", list.TotalCount, given.Total, err)
	}
}

func TestMockParticipantSummariesBatchOnlyPublicFields(t *testing.T) {
	fixture := newPostgresFixture(t)
	repository, queries := countedMockRepository(t, fixture)
	ctx := context.Background()
	avatar := "https://rsp.test/avatar.png"
	if _, err := fixture.pool.Exec(context.Background(), `UPDATE app.user_profiles SET avatar_url=$1 WHERE user_id=$2`, avatar, studentID); err != nil {
		t.Fatal(err)
	}
	missing := "00000000-0000-7000-8000-000000000999"
	summaries, err := repository.ListMockParticipantSummaries(ctx, []string{studentID, otherID, studentID, missing})
	if err != nil || len(summaries) != 2 || len(queries.statements) != 1 {
		t.Fatalf("summaries=%#v queries=%v error=%v", summaries, queries.statements, err)
	}
	if summaries[studentID].ID != studentID || summaries[studentID].Slug != "student" || summaries[studentID].Name != "Student" || summaries[studentID].AvatarURL == nil || *summaries[studentID].AvatarURL != avatar {
		t.Fatalf("public fields not populated: %#v", summaries[studentID])
	}
	query := strings.ToLower(queries.statements[0])
	projection := strings.SplitN(query, " from ", 2)[0]
	if strings.Contains(projection, "email") || strings.Contains(projection, "account_state") || strings.Contains(projection, "*") {
		t.Fatalf("summary query selected private or unnecessary fields: %s", query)
	}

	if _, err := fixture.pool.Exec(context.Background(), `UPDATE app.users SET deleted_at=now() WHERE id=$1`, otherID); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.pool.Exec(context.Background(), `UPDATE app.users SET account_state='deleted', deleted_at=now() WHERE id=$1`, studentID); err != nil {
		t.Fatal(err)
	}
	summaries, err = repository.ListMockParticipantSummaries(ctx, []string{studentID, otherID, missing})
	if err != nil || len(summaries) != 0 {
		t.Fatalf("deleted/missing users were not omitted: %#v error=%v", summaries, err)
	}
	queries.statements = nil
	if summaries, err := repository.ListMockParticipantSummaries(ctx, nil); err != nil || len(summaries) != 0 || len(queries.statements) != 0 {
		t.Fatalf("empty lookup=%#v queries=%v error=%v", summaries, queries.statements, err)
	}

	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := repository.ListMockParticipantSummaries(cancelled, []string{studentID}); !errors.Is(err, context.Canceled) {
		t.Fatalf("database failure must be returned, not interpreted as missing users: %v", err)
	}
}
