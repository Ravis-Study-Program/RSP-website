//go:build integration

package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"reflect"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/magedmg/RSP-website/backend/internal/authz"
	"github.com/magedmg/RSP-website/backend/internal/dal"
	"github.com/magedmg/RSP-website/backend/internal/mockinterviews"
)

const (
	mockLeetCodeRoundID = "00000000-0000-7000-8000-000000000110"
	mockCustomRoundID   = "00000000-0000-7000-8000-000000000111"
	mockAddedRoundID    = "00000000-0000-7000-8000-000000000112"
)

type storedMockRoundMetadata struct {
	ID, BehaviouralID, LeetCodeID, CustomID string
	ReviewStatus, Comment                   string
	ReviewedAt                              *time.Time
	CreatedAt                               time.Time
}

func mockRoundMetadata(t *testing.T, fixture postgresFixture, interviewID string) map[string]storedMockRoundMetadata {
	t.Helper()
	queryRows, err := fixture.pool.Query(context.Background(), `SELECT r.id, r.review_status::text, COALESCE(r.interviewee_comment_html,'') AS comment,
		r.reviewed_at, r.created_at, COALESCE(b.id::text,'') AS behavioural_id,
		COALESCE(l.id::text,'') AS leet_code_id, COALESCE(c.id::text,'') AS custom_id
		FROM app.mock_interview_rounds r
		LEFT JOIN app.behavioural_mock_interview_rounds b ON b.mock_interview_round_id=r.id
		LEFT JOIN app.leetcode_mock_interview_rounds l ON l.mock_interview_round_id=r.id
		LEFT JOIN app.custom_mock_interview_rounds c ON c.mock_interview_round_id=r.id
		WHERE r.mock_interview_id=$1`, interviewID)
	if err != nil {
		t.Fatal(err)
	}
	rows, err := pgx.CollectRows(queryRows, pgx.RowToStructByName[storedMockRoundMetadata])
	if err != nil {
		t.Fatal(err)
	}
	result := make(map[string]storedMockRoundMetadata, len(rows))
	for _, row := range rows {
		result[row.ID] = row
	}
	return result
}

func mockRequest(t *testing.T, handler http.Handler, method, path, actor string, body any, expectedStatus int) mockinterviews.Interview {
	t.Helper()
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	response := testRequest(t, handler, method, path, actor, string(raw))
	if response.Code != expectedStatus {
		t.Fatalf("%s %s: status=%d body=%s", method, path, response.Code, response.Body.String())
	}
	if expectedStatus != http.StatusOK && expectedStatus != http.StatusCreated {
		return mockinterviews.Interview{}
	}
	var result struct {
		Interview mockinterviews.Interview `json:"interview"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	return result.Interview
}

func createMockForWrites(t *testing.T, fixture postgresFixture) mockinterviews.Interview {
	t.Helper()
	return mockRequest(t, fixture.handler, http.MethodPost, "/api/v2/mock-interviews", "student", map[string]any{
		"interviewee": map[string]string{"userId": otherID}, "seasonId": seasonID,
		"occurredAt": "2026-09-01T00:00:00Z", "durationMinutes": 60, "notes": "original",
		"rounds": []map[string]any{
			{"id": mockRoundID, "type": "behavioural", "scores": map[string]int{"behavioural": 7}},
			{"id": mockLeetCodeRoundID, "type": "leetcode", "problemId": leetcodeProblemID,
				"scores": map[string]int{"confirmQuestions": 7, "algorithmDesign": 8, "complexityAnalysis": 7, "coding": 8, "testing": 7}},
			{"id": mockCustomRoundID, "type": "custom", "content": "Design a cache", "scores": map[string]int{"custom": 8}},
		},
	}, http.StatusCreated)
}

func mockUpdateBody(interview mockinterviews.Interview) map[string]any {
	rounds := make([]mockRoundRequest, len(interview.Rounds))
	for i, round := range interview.Rounds {
		rounds[i] = mockRoundRequest{ID: round.ID, Type: round.Type, ProblemID: round.ProblemID,
			Content: round.Content, Link: round.Link, Scores: round.Scores}
	}
	return map[string]any{"occurredAt": interview.OccurredAt,
		"durationMinutes": interview.DurationMinutes, "notes": interview.Notes, "rounds": rounds}
}

func mockDirectorHandler(fixture postgresFixture) http.Handler {
	return New(Config{
		DB: fixture.db, PublicOrigin: "https://rsp.test", CursorSecret: []byte("0123456789abcdef"),
		Authenticator: AuthenticatorFunc(func(*http.Request) (authz.Actor, error) {
			now := time.Now().UTC()
			return authz.Actor{UserID: studentID, EmailVerified: true, AccountState: authz.AccountActive,
				GlobalRoles: map[authz.GlobalRole]bool{authz.Director: true}, MFAAt: &now}, nil
		}),
	}).Handler()
}

func mockVersions(t *testing.T, fixture postgresFixture, interviewID string) []string {
	t.Helper()
	var snapshots []string
	snapshots = readColumn[string](t, fixture.pool, `SELECT snapshot::text FROM app.mock_interview_versions
		WHERE mock_interview_id=$1 ORDER BY version`, interviewID)
	return snapshots
}

func TestMockWritesPreserveRoundMetadataAndHistory(t *testing.T) {
	fixture := newPostgresFixture(t)
	interview := createMockForWrites(t, fixture)
	path := "/api/v2/mock-interviews/" + interview.ID
	originalVersions := mockVersions(t, fixture, interview.ID)
	metadata := mockRoundMetadata(t, fixture, interview.ID)
	if len(metadata) != 3 || metadata[mockRoundID].BehaviouralID == "" || metadata[mockLeetCodeRoundID].LeetCodeID == "" || metadata[mockCustomRoundID].CustomID == "" {
		t.Fatalf("missing round subtype metadata: %#v", metadata)
	}

	interview = mockRequest(t, fixture.handler, http.MethodPatch, path+"/rounds/"+mockRoundID+"/review", "other",
		map[string]any{"reviewed": true, "comment": "First review"}, http.StatusOK)
	firstReview := mockRoundMetadata(t, fixture, interview.ID)[mockRoundID]
	if firstReview.ReviewedAt == nil || firstReview.ReviewStatus != "reviewed" {
		t.Fatalf("review timestamp was not recorded: %#v", firstReview)
	}
	interview = mockRequest(t, fixture.handler, http.MethodPatch, path+"/rounds/"+mockLeetCodeRoundID+"/review", "other",
		map[string]any{"reviewed": true, "comment": "Second review"}, http.StatusOK)
	metadata = mockRoundMetadata(t, fixture, interview.ID)
	if !reflect.DeepEqual(firstReview, metadata[mockRoundID]) {
		t.Fatal("reviewing another round rewrote the first round's metadata")
	}

	var roundUpdatedBefore []time.Time
	roundUpdatedBefore = readColumn[time.Time](t, fixture.pool, `SELECT updated_at FROM app.mock_interview_rounds WHERE mock_interview_id=$1 ORDER BY id`, interview.ID)
	interview.Notes = "Edited notes"
	interview = mockRequest(t, fixture.handler, http.MethodPatch, path, "student", mockUpdateBody(interview), http.StatusOK)
	var roundUpdatedAfter []time.Time
	roundUpdatedAfter = readColumn[time.Time](t, fixture.pool, `SELECT updated_at FROM app.mock_interview_rounds WHERE mock_interview_id=$1 ORDER BY id`, interview.ID)
	if !reflect.DeepEqual(roundUpdatedBefore, roundUpdatedAfter) {
		t.Fatal("notes-only edit touched unrelated round rows")
	}
	interview.Rounds[0], interview.Rounds[2] = interview.Rounds[2], interview.Rounds[0]
	interview = mockRequest(t, fixture.handler, http.MethodPatch, path, "student", mockUpdateBody(interview), http.StatusOK)
	if got := mockRoundMetadata(t, fixture, interview.ID); !reflect.DeepEqual(metadata, got) {
		t.Fatalf("interviewer edit/reorder rewrote round metadata: before=%#v after=%#v", metadata, got)
	}
	if interview.Notes != "Edited notes" || interview.Rounds[0].ID != mockCustomRoundID || interview.Rounds[2].ID != mockRoundID {
		t.Fatalf("content or reorder not saved: %#v", interview)
	}

	// A reason that resembles an old command must remain ordinary audit text.
	interview = mockRequest(t, mockDirectorHandler(fixture), http.MethodPost, path+"/identity-correction", "student",
		map[string]any{"interviewerId": otherID, "intervieweeId": studentID,
			"seasonId": seasonID, "reason": "soft deleted"}, http.StatusOK)
	if interview.InterviewerID != otherID || interview.IntervieweeID != studentID {
		t.Fatalf("identities not corrected: %#v", interview)
	}
	if got := mockRoundMetadata(t, fixture, interview.ID); !reflect.DeepEqual(metadata, got) {
		t.Fatal("identity correction rewrote round metadata")
	}
	beforeDelete := mockVersions(t, fixture, interview.ID)
	if len(beforeDelete) != 6 || beforeDelete[0] != originalVersions[0] {
		t.Fatalf("prior history changed or version missing: %#v", beforeDelete)
	}
	mockRequest(t, fixture.handler, http.MethodDelete, path, "other", nil, http.StatusNoContent)
	if got := mockRoundMetadata(t, fixture, interview.ID); !reflect.DeepEqual(metadata, got) {
		t.Fatal("soft deletion rewrote or removed rounds")
	}
	versions := mockVersions(t, fixture, interview.ID)
	if len(versions) != 7 || !reflect.DeepEqual(beforeDelete, versions[:6]) {
		t.Fatal("deletion did not append one immutable version")
	}
	if _, err := fixture.db.GetMockInterview(context.Background(), interview.ID); !errors.Is(err, dal.ErrNotFound) {
		t.Fatalf("deleted interview still visible: %v", err)
	}
	var actions []string
	actions = readColumn[string](t, fixture.pool, `SELECT action FROM app.audit_events WHERE subject_id=$1 ORDER BY occurred_at,id`, interview.ID)
	wantActions := []string{"mock_interview.created", "mock_interview.reviewed", "mock_interview.reviewed", "mock_interview.updated", "mock_interview.updated", "mock_interview.identities_corrected", "mock_interview.deleted"}
	if !reflect.DeepEqual(actions, wantActions) {
		t.Fatalf("audit actions=%v", actions)
	}
	if _, err := fixture.pool.Exec(context.Background(), `UPDATE app.mock_interview_versions SET reason='rewritten' WHERE mock_interview_id=$1 AND version=1`, interview.ID); err == nil {
		t.Fatal("history is not immutable")
	}
}

func TestMockRoundEditsOnlyReplaceChangedSubtypes(t *testing.T) {
	fixture := newPostgresFixture(t)
	interview := createMockForWrites(t, fixture)
	path := "/api/v2/mock-interviews/" + interview.ID
	before := mockRoundMetadata(t, fixture, interview.ID)
	score := 9
	interview.Rounds[0].Scores.Behavioural = &score
	interview.Rounds[1] = mockinterviews.Round{ID: mockLeetCodeRoundID, Type: mockinterviews.Custom,
		Content: "Changed round kind", Scores: mockinterviews.Scores{Custom: &score}}
	interview.Rounds[2] = mockinterviews.Round{ID: mockAddedRoundID, Type: mockinterviews.Behavioural,
		Scores: mockinterviews.Scores{Behavioural: &score}}
	interview = mockRequest(t, fixture.handler, http.MethodPatch, path, "student", mockUpdateBody(interview), http.StatusOK)
	after := mockRoundMetadata(t, fixture, interview.ID)
	if len(after) != 3 || !reflect.DeepEqual(before[mockRoundID], after[mockRoundID]) {
		t.Fatal("editing a score replaced its parent or subtype")
	}
	if _, exists := after[mockCustomRoundID]; exists {
		t.Fatal("removed round still exists")
	}
	if after[mockLeetCodeRoundID].LeetCodeID != "" || after[mockLeetCodeRoundID].CustomID == "" || !after[mockLeetCodeRoundID].CreatedAt.Equal(before[mockLeetCodeRoundID].CreatedAt) {
		t.Fatal("changing kind did not replace only the subtype")
	}
	if after[mockAddedRoundID].BehaviouralID == "" || after[mockAddedRoundID].ReviewedAt != nil || *interview.Rounds[0].Scores.Behavioural != 9 {
		t.Fatal("added round or score edit not saved")
	}
}

func TestMockWriteGuardsLeaveDataAndHistoryUnchanged(t *testing.T) {
	fixture := newPostgresFixture(t)
	interview := createMockForWrites(t, fixture)
	path := "/api/v2/mock-interviews/" + interview.ID
	before := mockRoundMetadata(t, fixture, interview.ID)
	versions := mockVersions(t, fixture, interview.ID)
	mockRequest(t, fixture.handler, http.MethodPatch, path, "other", mockUpdateBody(interview), http.StatusNotFound)
	mockRequest(t, fixture.handler, http.MethodPatch, path+"/rounds/"+mockRoundID+"/review", "student",
		map[string]any{"reviewed": true}, http.StatusNotFound)
	mockRequest(t, fixture.handler, http.MethodPost, path+"/identity-correction", "other",
		map[string]any{"interviewerId": otherID, "intervieweeId": studentID, "reason": "swap"}, http.StatusForbidden)

	// An invalid subtype rolls back the preceding interview/round changes.
	invalid := interview
	invalid.Rounds = append([]mockinterviews.Round(nil), interview.Rounds...)
	invalid.Rounds[1].ProblemID = "00000000-0000-7000-8000-000000000999"
	invalid.Notes = "must roll back"
	mockRequest(t, fixture.handler, http.MethodPatch, path, "student", mockUpdateBody(invalid), http.StatusConflict)
	loaded, err := fixture.db.GetMockInterview(context.Background(), interview.ID)
	if err != nil || loaded.Notes != interview.Notes {
		t.Fatalf("failed write was not rolled back: interview=%#v error=%v", loaded, err)
	}

	// The reviewer remains eligible, but the interviewer no longer belongs to
	// the linked season. Reviews must still run the database scope validation.
	if _, err := fixture.pool.Exec(context.Background(), `UPDATE app.enrollments SET deleted_at=now() WHERE id=$1`, studentMemberID); err != nil {
		t.Fatal(err)
	}
	mockRequest(t, fixture.handler, http.MethodPatch, path+"/rounds/"+mockRoundID+"/review", "other",
		map[string]any{"reviewed": true}, http.StatusConflict)
	if _, err := fixture.pool.Exec(context.Background(), `UPDATE app.enrollments SET deleted_at=NULL WHERE id=$1`, studentMemberID); err != nil {
		t.Fatal(err)
	}

	if _, err := fixture.pool.Exec(context.Background(), `UPDATE app.seasons SET status='closed', closed_at=now() WHERE id=$1`, seasonID); err != nil {
		t.Fatal(err)
	}
	mockRequest(t, fixture.handler, http.MethodPatch, path, "student", mockUpdateBody(interview), http.StatusConflict)
	mockRequest(t, fixture.handler, http.MethodDelete, path, "student", nil, http.StatusConflict)
	mockRequest(t, fixture.handler, http.MethodPatch, path+"/rounds/"+mockRoundID+"/review", "other",
		map[string]any{"reviewed": true}, http.StatusConflict)

	// Direct storage writes also recheck season state after HTTP preflight.
	if _, err := fixture.db.UpdateMockInterview(context.Background(), interview, studentID, time.Now()); !errors.Is(err, dal.ErrConflict) {
		t.Fatalf("closed-season storage write=%v", err)
	}
	if got := mockRoundMetadata(t, fixture, interview.ID); !reflect.DeepEqual(before, got) {
		t.Fatal("rejected write changed round metadata")
	}
	if got := mockVersions(t, fixture, interview.ID); !reflect.DeepEqual(versions, got) {
		t.Fatal("rejected write changed history")
	}

	// Privileged corrections can still unlink records from a closed season.
	mockRequest(t, mockDirectorHandler(fixture), http.MethodPost, path+"/identity-correction", "student",
		map[string]any{"interviewerId": studentID, "intervieweeId": otherID, "reason": "unlink historical record"}, http.StatusOK)
	if got := mockRoundMetadata(t, fixture, interview.ID); !reflect.DeepEqual(before, got) {
		t.Fatal("unlinking a historical record rewrote rounds")
	}
}

func TestConcurrentMockEditsAppendHistory(t *testing.T) {
	fixture := newPostgresFixture(t)
	interview := createMockForWrites(t, fixture)
	start := make(chan struct{})
	results := make(chan error, 2)
	for _, notes := range []string{"First edit", "Second edit"} {
		go func(notes string) {
			candidate := interview
			candidate.Notes = notes
			<-start
			_, err := fixture.db.UpdateMockInterview(context.Background(), candidate, studentID, time.Now())
			results <- err
		}(notes)
	}
	close(start)
	first, second := <-results, <-results
	if first != nil || second != nil {
		t.Fatalf("concurrent writes failed: %v, %v", first, second)
	}
	if versions := mockVersions(t, fixture, interview.ID); len(versions) != 3 {
		t.Fatalf("concurrent writes produced %d versions", len(versions))
	}
}

func readColumn[T any](t *testing.T, pool *pgxpool.Pool, query string, args ...any) []T {
	t.Helper()
	rows, err := pool.Query(context.Background(), query, args...)
	if err != nil {
		t.Fatal(err)
	}
	values, err := pgx.CollectRows(rows, pgx.RowTo[T])
	if err != nil {
		t.Fatal(err)
	}
	return values
}
