//go:build integration

package dal

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/magedmg/RSP-website/backend/internal/platform/id"
	"github.com/magedmg/RSP-website/backend/internal/programme"
)

func mondayFixtureID(number int) string {
	return fmt.Sprintf("20000000-0000-7000-8000-%012d", number)
}

// newMondayUpdateFixture seeds one season with two students, two mentors, one
// mentorship and activity that straddles the reviewed range boundaries.
func newMondayUpdateFixture(t *testing.T) *Store {
	t.Helper()
	db := newPostgresFixture(t)
	exec := func(query string, args ...any) {
		t.Helper()
		if _, err := db.pool.Exec(context.Background(), query, args...); err != nil {
			t.Fatal(err)
		}
	}
	for _, user := range []struct {
		number int
		name   string
	}{{1, "Ada Lovelace"}, {2, "Ben Bell"}, {3, "Mia Mentor"}, {4, "Zoe Mentor"}} {
		exec(`INSERT INTO app.users(id) VALUES($1)`, mondayFixtureID(user.number))
		exec(`INSERT INTO app.user_profiles(user_id,slug,display_name,avatar_url) VALUES($1,$2,$3,$4)`,
			mondayFixtureID(user.number), fmt.Sprintf("monday-%d", user.number), user.name,
			fmt.Sprintf("https://avatars.test/%d.png", user.number))
	}
	exec(`INSERT INTO app.seasons(id,slug,name,start_at,end_at)
		VALUES($1,'monday-season','Monday season','2026-01-01','2026-12-31')`, mondayFixtureID(11))
	exec(`INSERT INTO app.enrollments(id,user_id,season_id,role,student_level,state) VALUES
		($1,$2,$5,'student','beginner','active'),
		($3,$4,$5,'student','beginner','active'),
		($6,$7,$5,'mentor','not_applicable','active'),
		($8,$9,$5,'mentor','not_applicable','active')`,
		mondayFixtureID(21), mondayFixtureID(1),
		mondayFixtureID(22), mondayFixtureID(2),
		mondayFixtureID(11),
		mondayFixtureID(23), mondayFixtureID(3),
		mondayFixtureID(24), mondayFixtureID(4))
	// Only Ada is mentored, so the mentor scope must exclude Ben.
	exec(`INSERT INTO app.mentorships(id,season_id,mentor_enrollment_id,student_enrollment_id) VALUES($1,$2,$3,$4)`,
		mondayFixtureID(31), mondayFixtureID(11), mondayFixtureID(23), mondayFixtureID(21))
	for index, difficulty := range []string{"easy", "medium", "hard"} {
		number := index + 1
		exec(`INSERT INTO app.problems(id,title,url) VALUES($1,$2,$3)`,
			mondayFixtureID(40+number), fmt.Sprintf("Monday problem %d", number), fmt.Sprintf("https://example.test/problem-%d", number))
		exec(`INSERT INTO app.leetcode_problems(id,problem_id,leetcode_number,difficulty,is_premium) VALUES($1,$2,$3,$4,false)`,
			mondayFixtureID(50+number), mondayFixtureID(40+number), 88000+number, difficulty)
	}
	// Adelaide is UTC+10:30 in March, so these instants share the written
	// calendar date; the last two bracket the end-date boundary.
	for _, attempt := range []struct {
		number      int
		problem     int
		attemptedAt string
		outcome     string
	}{
		{61, 1, "2026-02-28T10:00:00Z", "independently_solved"},
		{62, 2, "2026-03-05T10:00:00Z", "solved_with_hints"},
		{63, 3, "2026-03-08T13:00:00Z", "independently_solved"},
		{64, 3, "2026-03-08T14:00:00Z", "independently_solved"},
		{65, 2, "2026-03-05T10:00:00Z", "not_solved"},
		{68, 1, "2026-03-03T10:00:00Z", "independently_solved"},
	} {
		exec(`INSERT INTO app.problem_attempts(id,user_id,problem_id,attempted_at,time_taken_minutes,outcome)
			VALUES($1,$2,$3,$4,20,$5)`, mondayFixtureID(attempt.number), mondayFixtureID(1),
			mondayFixtureID(40+attempt.problem), attempt.attemptedAt, attempt.outcome)
	}
	// Deleted inside the range and unknown outcomes must never appear or count.
	exec(`INSERT INTO app.problem_attempts(id,user_id,problem_id,attempted_at,time_taken_minutes,outcome,deleted_at)
		VALUES($1,$2,$3,'2026-03-05T10:00:00Z',20,'independently_solved',now())`, mondayFixtureID(66), mondayFixtureID(1), mondayFixtureID(41))
	exec(`INSERT INTO app.problem_attempts(id,user_id,problem_id,attempted_at,time_taken_minutes,outcome)
		VALUES($1,$2,$3,'2026-03-05T11:00:00Z',20,'unknown')`, mondayFixtureID(67), mondayFixtureID(1), mondayFixtureID(41))
	// Ben's solved attempt falls inside the reviewed range but in none of his
	// own updates, so it must not count.
	exec(`INSERT INTO app.problem_attempts(id,user_id,problem_id,attempted_at,time_taken_minutes,outcome)
		VALUES($1,$2,$3,'2026-03-05T10:00:00Z',20,'independently_solved')`, mondayFixtureID(69), mondayFixtureID(2), mondayFixtureID(41))

	exec(`INSERT INTO app.mock_interviews(id,interviewer_user_id,interviewee_user_id,scheduled_at,duration_minutes) VALUES
		($1,$2,$3,'2026-03-05T10:00:00Z',60),
		($4,$2,$3,'2026-03-03T10:00:00Z',45)`, mondayFixtureID(71), mondayFixtureID(3), mondayFixtureID(1), mondayFixtureID(72))
	exec(`INSERT INTO app.mock_interviews(id,interviewer_user_id,interviewee_user_id,scheduled_at,duration_minutes,deleted_at)
		VALUES($1,$2,$3,'2026-03-05T10:00:00Z',60,now())`, mondayFixtureID(73), mondayFixtureID(3), mondayFixtureID(1))
	exec(`INSERT INTO app.mock_interviews(id,interviewer_user_id,interviewee_user_id,scheduled_at,duration_minutes)
		VALUES($1,$2,$3,'2026-03-05T10:00:00Z',60)`, mondayFixtureID(74), mondayFixtureID(3), mondayFixtureID(2))

	for _, update := range []struct {
		number  int
		student int
		start   string
		end     string
		content string
	}{
		{81, 1, "2026-02-27", "2026-03-03", "<p>Week one</p>"},
		{82, 1, "2026-03-28", "2026-04-03", "<p>Later</p>"},
		{83, 1, "2026-02-01", "2026-02-28", "<p>Earlier</p>"},
		{84, 1, "2026-03-02", "2026-03-08", "<p>Week two</p>"},
		{86, 2, "2026-01-05", "2026-01-11", "<p>Ben week</p>"},
	} {
		exec(`INSERT INTO app.monday_updates(id,season_id,student_user_id,start_date,end_date,content_html,hours)
			VALUES($1,$2,$3,$4::date,$5::date,$6,7.5)`, mondayFixtureID(update.number), mondayFixtureID(11),
			mondayFixtureID(update.student), update.start, update.end, update.content)
	}
	exec(`INSERT INTO app.monday_updates(id,season_id,student_user_id,start_date,end_date,content_html,hours,deleted_at)
		VALUES($1,$2,$3,'2026-03-03','2026-03-04','<p>Removed</p>',0,now())`, mondayFixtureID(85), mondayFixtureID(11), mondayFixtureID(1))
	exec(`INSERT INTO app.monday_update_marks(id,update_id,mentor_user_id,mark,comment_html)
		VALUES($1,$2,$3,'on_track','<p>Keep going</p>')`, mondayFixtureID(91), mondayFixtureID(81), mondayFixtureID(3))
	return db
}

type mondayQueryCounter struct{ statements []string }

func (c *mondayQueryCounter) TraceQueryStart(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	c.statements = append(c.statements, data.SQL)
	return ctx
}

func (*mondayQueryCounter) TraceQueryEnd(context.Context, *pgx.Conn, pgx.TraceQueryEndData) {}

func countedMondayStore(t *testing.T, db *Store) (*Store, *mondayQueryCounter) {
	t.Helper()
	counter := &mondayQueryCounter{}
	config := db.pool.Config().Copy()
	config.ConnConfig.Tracer = counter
	pool, err := pgxpool.NewWithConfig(context.Background(), config)
	if err != nil {
		t.Fatal(err)
	}
	store := New(pool)
	t.Cleanup(store.Close)
	return store, counter
}

func mondayRecordIDs(items []programme.MondayUpdateRecord) []string {
	ids := make([]string, len(items))
	for i := range items {
		ids[i] = items[i].ID
	}
	return ids
}

func mondayAttemptIDs(attempts []programme.MondayUpdateAttempt) []string {
	ids := make([]string, len(attempts))
	for i := range attempts {
		ids[i] = attempts[i].ID
	}
	return ids
}

func mondayMockIDs(mocks []programme.MondayUpdateMock) []string {
	ids := make([]string, len(mocks))
	for i := range mocks {
		ids[i] = mocks[i].ID
	}
	return ids
}

func TestMondayUpdateListSelectionAndActivity(t *testing.T) {
	db := newMondayUpdateFixture(t)
	ctx := context.Background()
	reviewed := MondayUpdateQuery{SeasonID: mondayFixtureID(11), StartDate: "2026-03-02", EndDate: "2026-03-08", Limit: 10, IncludeMarks: true}

	items, more, total, err := db.ListMondayUpdates(ctx, reviewed)
	if err != nil {
		t.Fatal(err)
	}
	// Only the two updates overlapping 2026-03-02..2026-03-08 are selected, and
	// the soft-deleted and fully outside ranges stay out. Newest id first.
	if more || total != 2 {
		t.Fatalf("more=%v total=%d", more, total)
	}
	if ids := mondayRecordIDs(items); !slices.Equal(ids, []string{mondayFixtureID(84), mondayFixtureID(81)}) {
		t.Fatalf("ids=%v", ids)
	}
	byID := make(map[string]programme.MondayUpdateRecord, len(items))
	for _, item := range items {
		byID[item.ID] = item
	}
	partialOverlap := byID[mondayFixtureID(81)]
	if partialOverlap.StartDate != "2026-02-27" || partialOverlap.EndDate != "2026-03-03" || partialOverlap.ContentHTML != "<p>Week one</p>" {
		t.Fatalf("partial overlap=%+v", partialOverlap)
	}
	if partialOverlap.Student.ID != mondayFixtureID(1) || partialOverlap.Student.Slug != "monday-1" || partialOverlap.Student.Name != "Ada Lovelace" {
		t.Fatalf("student=%+v", partialOverlap.Student)
	}
	if partialOverlap.Student.AvatarURL == nil || *partialOverlap.Student.AvatarURL != "https://avatars.test/1.png" {
		t.Fatalf("avatar=%v", partialOverlap.Student.AvatarURL)
	}

	// Activity comes from each update's own dates, not the selected range: the
	// 2026-02-28 attempt belongs to the partly overlapping update only, and the
	// 2026-03-03 attempt belongs to both overlapping updates.
	selected := byID[mondayFixtureID(84)]
	if got := mondayAttemptIDs(partialOverlap.Activity.Attempts); !slices.Equal(got, []string{mondayFixtureID(61), mondayFixtureID(68)}) {
		t.Fatalf("partial-overlap attempts=%v", got)
	}
	if got := mondayMockIDs(partialOverlap.Activity.Mocks); !slices.Equal(got, []string{mondayFixtureID(72)}) {
		t.Fatalf("partial-overlap mocks=%v", got)
	}
	if got := mondayAttemptIDs(selected.Activity.Attempts); !slices.Equal(got, []string{mondayFixtureID(68), mondayFixtureID(62), mondayFixtureID(65), mondayFixtureID(67), mondayFixtureID(63)}) {
		t.Fatalf("selected attempts=%v", got)
	}
	if got := mondayMockIDs(selected.Activity.Mocks); !slices.Equal(got, []string{mondayFixtureID(72), mondayFixtureID(71)}) {
		t.Fatalf("selected mocks=%v", got)
	}
	if len(partialOverlap.Marks) != 1 || partialOverlap.Marks[0].ID != mondayFixtureID(91) ||
		partialOverlap.Marks[0].Mentor.ID != mondayFixtureID(3) || partialOverlap.Marks[0].Mentor.Name != "Mia Mentor" ||
		partialOverlap.Marks[0].Mark != "on_track" || partialOverlap.Marks[0].CommentHTML != "<p>Keep going</p>" || partialOverlap.Marks[0].UpdateID != mondayFixtureID(81) {
		t.Fatalf("marks=%+v", partialOverlap.Marks)
	}
	if len(selected.Marks) != 0 {
		t.Fatalf("selected marks=%+v", selected.Marks)
	}

	detail, err := db.GetMondayUpdateDetail(ctx, mondayFixtureID(11), mondayFixtureID(84))
	if err != nil || !reflect.DeepEqual(detail, selected) {
		t.Fatalf("detail=%+v error=%v", detail, err)
	}
	row, err := db.GetMondayUpdate(ctx, mondayFixtureID(11), mondayFixtureID(84))
	if err != nil || !reflect.DeepEqual(row.Marks, []programme.MondayUpdateMarkRecord{}) || len(row.Activity.Attempts) != 0 || len(row.Activity.Mocks) != 0 {
		t.Fatalf("row=%+v error=%v", row, err)
	}
	if _, err := db.GetMondayUpdate(ctx, mondayFixtureID(84), mondayFixtureID(84)); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-season read error=%v", err)
	}

	// Without marks the batch read still returns an empty, non-nil slice.
	noMarks, _, _, err := db.ListMondayUpdates(ctx, MondayUpdateQuery{SeasonID: mondayFixtureID(11), StartDate: "2026-03-02", EndDate: "2026-03-08", Limit: 10})
	if err != nil || len(noMarks) != 2 || noMarks[0].Marks == nil || len(noMarks[0].Marks) != 0 {
		t.Fatalf("noMarks=%+v error=%v", noMarks, err)
	}

	// A wider range returns Ben's update too; the student filter and the mentor
	// scope narrow it back to Ada's updates only.
	wider := MondayUpdateQuery{SeasonID: mondayFixtureID(11), StartDate: "2026-01-01", EndDate: "2026-12-31", Limit: 10}
	all, _, allTotal, err := db.ListMondayUpdates(ctx, wider)
	if err != nil || allTotal != 5 {
		t.Fatalf("all=%v total=%d error=%v", mondayRecordIDs(all), allTotal, err)
	}
	if ids := mondayRecordIDs(all); !slices.Equal(ids, []string{mondayFixtureID(86), mondayFixtureID(84), mondayFixtureID(83), mondayFixtureID(82), mondayFixtureID(81)}) {
		t.Fatalf("all ids=%v", ids)
	}
	ben, _, benTotal, err := db.ListMondayUpdates(ctx, MondayUpdateQuery{
		SeasonID: mondayFixtureID(11), StartDate: "2026-01-01", EndDate: "2026-12-31", StudentIDs: []string{mondayFixtureID(2)}, Limit: 10,
	})
	if err != nil || benTotal != 1 || !slices.Equal(mondayRecordIDs(ben), []string{mondayFixtureID(86)}) {
		t.Fatalf("ben=%v total=%d error=%v", mondayRecordIDs(ben), benTotal, err)
	}
	mentored, _, mentoredTotal, err := db.ListMondayUpdates(ctx, MondayUpdateQuery{
		SeasonID: mondayFixtureID(11), StartDate: "2026-01-01", EndDate: "2026-12-31", MentorUserID: mondayFixtureID(3), Limit: 10,
	})
	if err != nil || mentoredTotal != 4 {
		t.Fatalf("mentored=%v total=%d error=%v", mondayRecordIDs(mentored), mentoredTotal, err)
	}
	if slices.Contains(mondayRecordIDs(mentored), mondayFixtureID(86)) {
		t.Fatalf("mentor scope leaked an unmentored student: %v", mondayRecordIDs(mentored))
	}
	unmatched := MondayUpdateQuery{
		SeasonID: mondayFixtureID(11), StartDate: "2026-01-01", EndDate: "2026-12-31",
		MentorUserID: mondayFixtureID(4), StudentIDs: []string{mondayFixtureID(2)}, Limit: 10,
	}
	if _, _, unmatchedTotal, err := db.ListMondayUpdates(ctx, unmatched); err != nil || unmatchedTotal != 0 {
		t.Fatalf("unmatched total=%d error=%v", unmatchedTotal, err)
	}

	// Keyset paging walks the newest-first order without repeating rows.
	firstPage, firstMore, pageTotal, err := db.ListMondayUpdates(ctx, MondayUpdateQuery{
		SeasonID: mondayFixtureID(11), StartDate: "2026-01-01", EndDate: "2026-12-31", Limit: 2, IncludeMarks: true,
	})
	if err != nil || !firstMore || pageTotal != 5 {
		t.Fatalf("first page=%v more=%v total=%d error=%v", mondayRecordIDs(firstPage), firstMore, pageTotal, err)
	}
	if ids := mondayRecordIDs(firstPage); !slices.Equal(ids, []string{mondayFixtureID(86), mondayFixtureID(84)}) {
		t.Fatalf("first page ids=%v", ids)
	}
	secondPage, secondMore, _, err := db.ListMondayUpdates(ctx, MondayUpdateQuery{
		SeasonID: mondayFixtureID(11), StartDate: "2026-01-01", EndDate: "2026-12-31",
		Boundary: mondayFixtureID(83), Limit: 1, IncludeMarks: true,
	})
	if err != nil || !secondMore {
		t.Fatalf("second page=%v more=%v error=%v", mondayRecordIDs(secondPage), secondMore, err)
	}
	if ids := mondayRecordIDs(secondPage); !slices.Equal(ids, []string{mondayFixtureID(82)}) {
		t.Fatalf("second page ids=%v", ids)
	}
	lastPage, lastMore, _, err := db.ListMondayUpdates(ctx, MondayUpdateQuery{
		SeasonID: mondayFixtureID(11), StartDate: "2026-01-01", EndDate: "2026-12-31",
		Boundary: mondayFixtureID(82), Limit: 1, IncludeMarks: true,
	})
	if err != nil || lastMore || !slices.Equal(mondayRecordIDs(lastPage), []string{mondayFixtureID(81)}) {
		t.Fatalf("last page=%v more=%v error=%v", mondayRecordIDs(lastPage), lastMore, err)
	}

	// Hydration is batched: count, page, marks, attempts and mocks.
	store, counter := countedMondayStore(t, db)
	if _, _, _, err := store.ListMondayUpdates(ctx, reviewed); err != nil {
		t.Fatal(err)
	}
	if len(counter.statements) != 5 {
		t.Fatalf("list used %d statements: %v", len(counter.statements), counter.statements)
	}
}

func TestMondayUpdateReviewCountsAndDedup(t *testing.T) {
	db := newMondayUpdateFixture(t)
	ctx := context.Background()
	reviewed := MondayUpdateReviewQuery{SeasonID: mondayFixtureID(11), StartDate: "2026-03-02", EndDate: "2026-03-08", Limit: 10}

	items, more, total, err := db.ListMondayUpdateReview(ctx, reviewed)
	if err != nil {
		t.Fatal(err)
	}
	if more || total != 2 || len(items) != 2 {
		t.Fatalf("more=%v total=%d items=%d", more, total, len(items))
	}
	// Every enrolled student appears, ordered by display name.
	if items[0].Student.ID != mondayFixtureID(1) || items[0].Student.Name != "Ada Lovelace" ||
		items[1].Student.ID != mondayFixtureID(2) || items[1].Student.Name != "Ben Bell" {
		t.Fatalf("items=%+v", []programme.PersonSummary{items[0].Student, items[1].Student})
	}
	ada := items[0]
	// Solved-only, and deduplicated across Ada's two overlapping updates: the
	// 2026-03-03 attempt falls inside both, so easy would be 3 without DISTINCT.
	if ada.LeetCodeCounts != (programme.MondayUpdateDifficultyCounts{Easy: 2, Medium: 1, Hard: 1}) {
		t.Fatalf("counts=%+v", ada.LeetCodeCounts)
	}
	if ada.MockCount != 2 || ada.UpdateCount != 2 {
		t.Fatalf("mockCount=%d updateCount=%d", ada.MockCount, ada.UpdateCount)
	}
	if len(ada.Updates) != 2 || ada.Updates[0].ID != mondayFixtureID(84) || ada.Updates[1].ID != mondayFixtureID(81) {
		t.Fatalf("updates=%+v", ada.Updates)
	}
	if ada.Updates[0].StartDate != "2026-03-02" || ada.Updates[0].EndDate != "2026-03-08" || ada.Updates[0].ContentHTML != "<p>Week two</p>" {
		t.Fatalf("update=%+v", ada.Updates[0])
	}
	if len(ada.Updates[0].Marks) != 0 {
		t.Fatalf("update marks=%+v", ada.Updates[0].Marks)
	}
	if len(ada.Updates[1].Marks) != 1 || ada.Updates[1].Marks[0].Mark != "on_track" ||
		ada.Updates[1].Marks[0].Mentor.ID != mondayFixtureID(3) || ada.Updates[1].Marks[0].Mentor.Name != "Mia Mentor" {
		t.Fatalf("update marks=%+v", ada.Updates[1].Marks)
	}

	// A student with no update in the range still gets a row, with no activity
	// counted even though they were active inside the selected range.
	ben := items[1]
	if ben.UpdateCount != 0 || ben.MockCount != 0 || ben.LeetCodeCounts != (programme.MondayUpdateDifficultyCounts{}) {
		t.Fatalf("ben=%+v", ben)
	}
	if ben.Updates == nil || len(ben.Updates) != 0 {
		t.Fatalf("ben updates=%+v", ben.Updates)
	}

	firstPage, firstMore, pageTotal, err := db.ListMondayUpdateReview(ctx, MondayUpdateReviewQuery{
		SeasonID: mondayFixtureID(11), StartDate: "2026-03-02", EndDate: "2026-03-08", Limit: 1,
	})
	if err != nil || !firstMore || pageTotal != 2 || len(firstPage) != 1 || firstPage[0].Student.ID != mondayFixtureID(1) {
		t.Fatalf("first page=%+v more=%v total=%d error=%v", firstPage, firstMore, pageTotal, err)
	}
	secondPage, secondMore, _, err := db.ListMondayUpdateReview(ctx, MondayUpdateReviewQuery{
		SeasonID: mondayFixtureID(11), StartDate: "2026-03-02", EndDate: "2026-03-08", Boundary: mondayFixtureID(1), Limit: 1,
	})
	if err != nil || secondMore || len(secondPage) != 1 || secondPage[0].Student.ID != mondayFixtureID(2) || len(secondPage[0].Updates) != 0 {
		t.Fatalf("second page=%+v more=%v error=%v", secondPage, secondMore, err)
	}

	// The review page hydrates everything in batched queries, never per row.
	store, counter := countedMondayStore(t, db)
	if _, _, _, err := store.ListMondayUpdateReview(ctx, reviewed); err != nil {
		t.Fatal(err)
	}
	if len(counter.statements) != 6 {
		t.Fatalf("review used %d statements: %v", len(counter.statements), counter.statements)
	}
}

func TestMondayUpdateLifecycle(t *testing.T) {
	db := newMondayUpdateFixture(t)
	ctx := context.Background()
	actor := mondayFixtureID(1)
	if _, err := db.pool.Exec(ctx, `INSERT INTO app.problem_attempts(id,user_id,problem_id,attempted_at,time_taken_minutes,outcome)
		VALUES($1,$2,$3,'2026-03-11T10:00:00Z',20,'independently_solved')`, mondayFixtureID(70), actor, mondayFixtureID(41)); err != nil {
		t.Fatal(err)
	}

	created, err := db.CreateMondayUpdate(ctx, programme.MondayUpdateRecord{
		ID:          id.New(),
		SeasonID:    mondayFixtureID(11),
		Student:     programme.PersonSummary{ID: actor},
		StartDate:   "2026-03-10",
		EndDate:     "2026-03-12",
		ContentHTML: "<p>Fresh</p>",
	}, actor)
	if err != nil {
		t.Fatal(err)
	}
	if created.StartDate != "2026-03-10" || created.EndDate != "2026-03-12" || created.ContentHTML != "<p>Fresh</p>" ||
		created.SeasonID != mondayFixtureID(11) || created.Student.Name != "Ada Lovelace" || created.Student.Slug != "monday-1" {
		t.Fatalf("created=%+v", created)
	}
	// The returned detail carries the activity of its own dates only.
	if got := mondayAttemptIDs(created.Activity.Attempts); !slices.Equal(got, []string{mondayFixtureID(70)}) || len(created.Activity.Mocks) != 0 {
		t.Fatalf("activity=%+v", created.Activity)
	}
	if created.Marks == nil || len(created.Marks) != 0 {
		t.Fatalf("marks=%+v", created.Marks)
	}

	mark := programme.MondayUpdateMarkRecord{
		ID:          id.New(),
		UpdateID:    mondayFixtureID(84),
		Mentor:      programme.PersonSummary{ID: mondayFixtureID(3)},
		Mark:        "doing_good",
		CommentHTML: "<p>Nice</p>",
	}
	first, err := db.UpsertMondayUpdateMark(ctx, mark, mondayFixtureID(3))
	if err != nil {
		t.Fatal(err)
	}
	if first.UpdateID != mondayFixtureID(84) || first.Mark != "doing_good" || first.CommentHTML != "<p>Nice</p>" ||
		first.Mentor.ID != mondayFixtureID(3) || first.Mentor.Name != "Mia Mentor" || first.Mentor.Slug != "monday-3" {
		t.Fatalf("first mark=%+v", first)
	}
	// The same mentor's second mark replaces the row instead of adding one.
	mark.Mark, mark.CommentHTML = "at_risk", "<p>Watch this</p>"
	second, err := db.UpsertMondayUpdateMark(ctx, mark, mondayFixtureID(3))
	if err != nil {
		t.Fatal(err)
	}
	if second.ID != first.ID || second.Mark != "at_risk" || second.CommentHTML != "<p>Watch this</p>" {
		t.Fatalf("second mark=%+v first=%+v", second, first)
	}
	third, err := db.UpsertMondayUpdateMark(ctx, programme.MondayUpdateMarkRecord{
		ID:       id.New(),
		UpdateID: mondayFixtureID(84),
		Mentor:   programme.PersonSummary{ID: mondayFixtureID(4)},
		Mark:     "way_ahead",
	}, mondayFixtureID(4))
	if err != nil {
		t.Fatal(err)
	}
	if third.ID == first.ID || third.CommentHTML != "" || third.Mentor.ID != mondayFixtureID(4) {
		t.Fatalf("third mark=%+v", third)
	}
	var markRows int
	if err := db.pool.QueryRow(ctx, `SELECT count(*) FROM app.monday_update_marks WHERE update_id=$1`, mondayFixtureID(84)).Scan(&markRows); err != nil || markRows != 2 {
		t.Fatalf("mark rows=%d error=%v", markRows, err)
	}
	var blankComments int
	if err := db.pool.QueryRow(ctx, `SELECT count(*) FROM app.monday_update_marks WHERE comment_html IS NULL`).Scan(&blankComments); err != nil || blankComments != 1 {
		t.Fatalf("null comments=%d error=%v", blankComments, err)
	}
	detail, err := db.GetMondayUpdateDetail(ctx, mondayFixtureID(11), mondayFixtureID(84))
	if err != nil || len(detail.Marks) != 2 || detail.Marks[0].Mark != "at_risk" || detail.Marks[1].Mark != "way_ahead" {
		t.Fatalf("detail marks=%+v error=%v", detail.Marks, err)
	}

	// Deletion is soft, author-scoped and audited.
	if err := db.DeleteMondayUpdate(ctx, mondayFixtureID(11), mondayFixtureID(84), mondayFixtureID(2), actor); !errors.Is(err, ErrNotFound) {
		t.Fatalf("wrong author error=%v", err)
	}
	if err := db.DeleteMondayUpdate(ctx, mondayFixtureID(11), mondayFixtureID(82), actor, actor); err != nil {
		t.Fatal(err)
	}
	if err := db.DeleteMondayUpdate(ctx, mondayFixtureID(11), mondayFixtureID(82), actor, actor); !errors.Is(err, ErrNotFound) {
		t.Fatalf("second delete error=%v", err)
	}
	if _, err := db.GetMondayUpdate(ctx, mondayFixtureID(11), mondayFixtureID(82)); !errors.Is(err, ErrNotFound) {
		t.Fatalf("deleted read error=%v", err)
	}
	var deleted bool
	if err := db.pool.QueryRow(ctx, `SELECT deleted_at IS NOT NULL FROM app.monday_updates WHERE id=$1`, mondayFixtureID(82)).Scan(&deleted); err != nil || !deleted {
		t.Fatalf("stored deleted=%v error=%v", deleted, err)
	}

	for action, want := range map[string]int{"monday_update.created": 1, "monday_update.deleted": 1, "monday_update.mark_saved": 3} {
		var events int
		if err := db.pool.QueryRow(ctx, `SELECT count(*) FROM app.audit_events WHERE action=$1`, action).Scan(&events); err != nil || events != want {
			t.Fatalf("action=%s events=%d error=%v", action, events, err)
		}
	}
	var repeated int
	if err := db.pool.QueryRow(ctx, `SELECT count(*) FROM app.audit_events WHERE action='monday_update.mark_saved' AND subject_id=$1`, first.ID).Scan(&repeated); err != nil || repeated != 2 {
		t.Fatalf("mark audit rows for the first mark=%d error=%v", repeated, err)
	}
	var markSubjects int
	if err := db.pool.QueryRow(ctx, `SELECT count(DISTINCT subject_id) FROM app.audit_events WHERE action='monday_update.mark_saved'`).Scan(&markSubjects); err != nil || markSubjects != 2 {
		t.Fatalf("mark audit subjects=%d error=%v", markSubjects, err)
	}
}
