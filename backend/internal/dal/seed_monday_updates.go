package dal

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/magedmg/RSP-website/backend/internal/programme"
	"github.com/magedmg/RSP-website/backend/internal/worker"
)

// seedCohortSlugPrefix marks the extra development students so the Monday
// update fixture is created once and skipped on every later run.
const seedCohortSlugPrefix = "dev-cohort-"

type seedCohortStudent struct {
	slug, name, level string
	mentor            string // "dev" (the seeded mentor), "second" or "" for none
	persona           seedPersona
	// lengths are the day spans of consecutive updates, newest first; 0 skips
	// one week without an update.
	lengths []int
}

type seedPersona struct {
	trend  []programme.MondayUpdateMark // cycles over a student's updates
	hours  []float64                    // cycles over a student's updates
	topics []string
	style  int // 0 detailed, 1 bullet list, 2 terse
}

var seedTopicPool = map[string][]string{
	"arrays":    {"two pointers on sorted arrays", "prefix sums", "sliding window with a frequency map", "in-place array rotation"},
	"hashing":   {"hash map counting patterns", "grouping anagrams", "longest consecutive sequence", "subarray sum equals k"},
	"trees":     {"BST validation", "level-order traversal", "lowest common ancestor", "serialising a binary tree"},
	"graphs":    {"BFS on grids", "topological sort with Kahn's algorithm", "union-find with path compression", "Dijkstra on weighted edges"},
	"dp":        {"1D DP like house robber", "knapsack-style subset sums", "longest common subsequence", "interval DP on burst balloons"},
	"intervals": {"merging intervals", "meeting rooms with a min-heap", "insert interval edge cases", "non-overlapping intervals greedy"},
	"heaps":     {"top-k with a heap", "merging k sorted lists", "median of a data stream", "task scheduler"},
	"backtrack": {"permutations and subsets", "N-Queens pruning", "word search on a board", "combination sum"},
	"system":    {"rate limiter design", "URL shortener design", "designing a news feed", "consistent hashing"},
	"behaviour": {"STAR stories for conflict", "my 'tell me about yourself' pitch", "a failure story with a clear lesson", "leadership examples from uni projects"},
}

var seedWins = []string{
	"Explained my approach out loud before coding every time, which made debugging much faster.",
	"Finally understood why the invariant holds instead of memorising the template.",
	"Solved two mediums without hints, both inside 25 minutes.",
	"My mock interviewer said my complexity analysis was clear and well structured.",
	"Started writing test cases before running code and caught three off-by-one bugs.",
	"Re-did last week's failed problem from scratch and got it first try.",
	"Kept a mistakes log, which is already showing patterns in what I miss.",
}

var seedStruggles = []string{
	"Still freezing when the first idea doesn't work; I need a better fallback routine.",
	"Edge cases with empty input keep catching me out.",
	"Ran out of time on the hard problem and only got the brute force working.",
	"Uni assignments took over midweek so I lost two days of practice.",
	"I understand the solutions when I read them but struggle to come up with them cold.",
	"My recursion base cases are sloppy and cost me time in the mock.",
	"Got sick on Wednesday, so this stretch is lighter than planned.",
}

var seedNext = []string{
	"Book another mock and focus on talking through trade-offs.",
	"Do one timed medium every day before lectures.",
	"Revisit the problems I needed hints for and redo them without notes.",
	"Move on to the next topic in the roadmap and keep a daily streak.",
	"Ask my mentor to review my last two submissions.",
	"Practise writing clean code first, then optimise.",
}

var seedMarkComments = map[programme.MondayUpdateMark][]string{
	programme.MarkWayAhead:   {"Outstanding consistency. Start mixing in harder problems and timed mocks.", "You're well ahead of the plan; keep the reflection quality this high."},
	programme.MarkDoingGood:  {"Strong week. Push one more mock before the end of the month.", "Good progress and honest reflection. Keep the mistakes log going."},
	programme.MarkOnTrack:    {"Steady progress. Try to narrate the invariant before you start typing.", "On pace. Next step is solving mediums without peeking at hints."},
	programme.MarkABitBehind: {"A lighter stretch. Let's plan smaller daily goals so uni doesn't swallow the week.", "Fewer problems than planned. Pick two topics and go deep rather than wide."},
	programme.MarkAtRisk:     {"Let's catch up this week; book a 1:1 so we can reset the plan.", "Very little activity recently. Please reach out so we can adjust your schedule."},
}

var seedCohort = []seedCohortStudent{
	{"amelia-chen", "Amelia Chen", "intermediate", "dev", seedPersona{
		trend: []programme.MondayUpdateMark{programme.MarkDoingGood, programme.MarkWayAhead, programme.MarkDoingGood},
		hours: []float64{32, 28.5, 35, 30}, topics: []string{"hashing", "trees", "graphs", "dp"}}, []int{7, 7, 7, 7, 7, 7, 7, 7}},
	{"marcus-oyelaran", "Marcus Oyelaran", "beginner", "dev", seedPersona{
		trend: []programme.MondayUpdateMark{programme.MarkABitBehind, programme.MarkOnTrack, programme.MarkAtRisk},
		hours: []float64{12, 8.5, 15, 4}, topics: []string{"arrays", "hashing", "behaviour"}, style: 2}, []int{3, 0, 4, 7, 0, 14, 7}},
	{"zara-ahmed", "Zara Ahmed", "advanced", "dev", seedPersona{
		trend: []programme.MondayUpdateMark{programme.MarkWayAhead},
		hours: []float64{42, 38, 45.5, 40}, topics: []string{"graphs", "dp", "system", "heaps"}, style: 1}, []int{7, 3, 4, 7, 7, 7, 7, 7, 7}},
	{"liam-nguyen", "Liam Nguyen", "novice", "dev", seedPersona{
		trend: []programme.MondayUpdateMark{programme.MarkOnTrack, programme.MarkABitBehind},
		hours: []float64{18, 20, 16.5}, topics: []string{"arrays", "hashing", "intervals"}}, []int{14, 14, 7, 10}},
	{"sofia-rossi", "Sofia Rossi", "intermediate", "dev", seedPersona{
		trend: []programme.MondayUpdateMark{programme.MarkOnTrack, programme.MarkDoingGood},
		hours: []float64{25, 22, 30, 27.5}, topics: []string{"intervals", "heaps", "backtrack", "behaviour"}, style: 1}, []int{1, 6, 7, 0, 7, 5, 2, 7}},
	{"noah-williams", "Noah Williams", "beginner", "second", seedPersona{
		trend: []programme.MondayUpdateMark{programme.MarkAtRisk, programme.MarkABitBehind},
		hours: []float64{3, 6.5, 0, 10}, topics: []string{"arrays", "behaviour"}, style: 2}, []int{0, 7, 0, 0, 21}},
	{"priyanka-das", "Priyanka Das", "advanced", "second", seedPersona{
		trend: []programme.MondayUpdateMark{programme.MarkDoingGood, programme.MarkWayAhead},
		hours: []float64{36, 40, 33.5}, topics: []string{"system", "graphs", "dp"}}, []int{7, 7, 7, 7, 7, 7, 7, 7, 7}},
	{"ethan-park", "Ethan Park", "intermediate", "second", seedPersona{
		trend: []programme.MondayUpdateMark{programme.MarkOnTrack},
		hours: []float64{20, 24, 19.5, 26}, topics: []string{"trees", "backtrack", "heaps"}, style: 1}, []int{4, 3, 7, 7, 2, 5, 7}},
	{"hannah-kowalski", "Hannah Kowalski", "novice", "second", seedPersona{
		trend: []programme.MondayUpdateMark{programme.MarkABitBehind, programme.MarkOnTrack},
		hours: []float64{14, 11, 16}, topics: []string{"arrays", "hashing", "trees"}}, []int{7, 7, 0, 7, 7}},
	{"diego-martinez", "Diego Martínez", "intermediate", "", seedPersona{
		trend: []programme.MondayUpdateMark{programme.MarkOnTrack},
		hours: []float64{22, 18.5, 27}, topics: []string{"intervals", "dp", "behaviour"}, style: 2}, []int{10, 4, 7, 7}},
	{"aisha-bello", "Aisha Bello", "beginner", "", seedPersona{
		trend: []programme.MondayUpdateMark{programme.MarkOnTrack},
		hours: []float64{15, 17.5, 12}, topics: []string{"arrays", "hashing"}, style: 1}, []int{7, 7, 7}},
	{"oliver-smith", "Oliver Smith", "advanced", "", seedPersona{
		trend: []programme.MondayUpdateMark{programme.MarkDoingGood},
		hours: []float64{50, 45, 55.5}, topics: []string{"system", "graphs"}}, []int{2, 5, 7, 7, 7, 7, 7, 7, 7, 7}},
}

// devStudentSeed gives the seeded (log-in-able) student their own history.
var devStudentSeed = seedCohortStudent{persona: seedPersona{
	trend: []programme.MondayUpdateMark{programme.MarkOnTrack, programme.MarkDoingGood, programme.MarkABitBehind},
	hours: []float64{30, 26, 33.5, 21}, topics: []string{"arrays", "hashing", "trees", "behaviour"}}, lengths: []int{7, 7, 3, 4, 7, 0, 7}}

// SeedMondayUpdatesResult reports which parts of the fixture a run created.
type SeedMondayUpdatesResult struct {
	Cohort, Activity bool
}

// SeedMondayUpdates adds a development cohort to dev-season — extra students,
// a second mentor, mentorships and many Monday updates over varied date ranges
// with mentor marks — plus LeetCode attempts and mock interviews inside those
// ranges, so mentor and reviewer screens have realistic volume. Each part is
// created once; a later run leaves an existing part untouched.
func (s *Store) SeedMondayUpdates(ctx context.Context, now time.Time) (SeedMondayUpdatesResult, error) {
	var result SeedMondayUpdatesResult
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return result, err
	}
	defer tx.Rollback(context.Background())
	// The worker may sync the problem catalogue concurrently.
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, worker.LeetCodeAdvisoryLock); err != nil {
		return result, err
	}

	var seasonID string
	if err := tx.QueryRow(ctx, `SELECT id FROM app.seasons WHERE slug='dev-season'`).Scan(&seasonID); err != nil {
		return result, fmt.Errorf("dev-season is missing; run the base seed first: %w", noRows(err))
	}
	var cohortExists, activityExists bool
	if err := tx.QueryRow(ctx, `SELECT
		EXISTS(SELECT 1 FROM app.user_profiles WHERE slug LIKE $1),
		EXISTS(SELECT 1 FROM app.problem_attempts a JOIN app.user_profiles p ON p.user_id=a.user_id WHERE p.slug LIKE $1)
		OR EXISTS(SELECT 1 FROM app.mock_interviews m JOIN app.user_profiles p ON p.user_id=m.interviewee_user_id WHERE p.slug LIKE $1)`,
		seedCohortSlugPrefix+"%").Scan(&cohortExists, &activityExists); err != nil {
		return result, err
	}
	if !cohortExists {
		if _, err := seedMondayCohort(ctx, tx, seasonID, now); err != nil {
			return result, err
		}
		result.Cohort = true
	}
	if !activityExists {
		if err := seedMondayActivity(ctx, tx, seasonID, now); err != nil {
			return result, err
		}
		result.Activity = true
	}
	return result, tx.Commit(ctx)
}

func seedMondayCohort(ctx context.Context, tx pgx.Tx, seasonID string, now time.Time) (bool, error) {
	enrollmentFor := func(slug, role string) (userID, enrollmentID string, err error) {
		err = tx.QueryRow(ctx, `SELECT e.user_id,e.id FROM app.enrollments e JOIN app.user_profiles p ON p.user_id=e.user_id
			WHERE p.slug=$1 AND e.season_id=$2 AND e.role=$3 AND e.state='active'`, slug, seasonID, role).Scan(&userID, &enrollmentID)
		if errors.Is(err, pgx.ErrNoRows) {
			err = fmt.Errorf("seeded %s %s is missing from dev-season", role, slug)
		}
		return userID, enrollmentID, err
	}
	devStudentID, _, err := enrollmentFor("dev-student", "student")
	if err != nil {
		return false, err
	}
	devMentorID, devMentorEnrollment, err := enrollmentFor("dev-mentor", "mentor")
	if err != nil {
		return false, err
	}

	enrol := func(slug, name, role, level string) (string, string, error) {
		userID, err := resolveSeedUser(ctx, tx, seedUser{FallbackEmail: slug + "@rsp.local", Slug: slug, DisplayName: name})
		if err != nil {
			return "", "", err
		}
		var enrollmentID string
		err = tx.QueryRow(ctx, `INSERT INTO app.enrollments(user_id,season_id,role,student_level,state) VALUES($1,$2,$3,$4,'active') RETURNING id`,
			userID, seasonID, role, level).Scan(&enrollmentID)
		return userID, enrollmentID, err
	}
	secondMentorID, secondMentorEnrollment, err := enrol(seedCohortSlugPrefix+"priya-raman", "Priya Raman", "mentor", "not_applicable")
	if err != nil {
		return false, fmt.Errorf("seed second mentor: %w", err)
	}

	rng := rand.New(rand.NewPCG(2026, 10))
	weekEnd, err := time.Parse(time.DateOnly, programme.CurrentMondayUpdateWeekEnd(now))
	if err != nil {
		return false, err
	}
	insert := func(studentID, mentorID string, st seedCohortStudent) error {
		end := weekEnd
		for i, length := range st.lengths {
			if length == 0 {
				end = end.AddDate(0, 0, -7)
				continue
			}
			start := end.AddDate(0, 0, -(length - 1))
			hours := st.persona.hours[i%len(st.persona.hours)]
			created := end.Add(33 * time.Hour) // Monday morning after the range, Adelaide-ish
			if created.After(now) {
				created = now.Add(-time.Duration(rng.IntN(48)+1) * time.Hour)
			}
			var updateID string
			if err := tx.QueryRow(ctx, `INSERT INTO app.monday_updates(season_id,student_user_id,start_date,end_date,content_html,hours,created_at)
				VALUES($1,$2,$3::date,$4::date,$5,$6,$7) RETURNING id`, seasonID, studentID, start.Format(time.DateOnly), end.Format(time.DateOnly),
				seedUpdateHTML(rng, st.persona, i, length, hours), hours, created).Scan(&updateID); err != nil {
				return fmt.Errorf("seed monday update: %w", err)
			}
			// Mentors mark most finished updates; the newest is often still waiting.
			if mentorID != "" && created.Before(now.Add(-24*time.Hour)) && rng.IntN(10) < 8 {
				mark := st.persona.trend[i%len(st.persona.trend)]
				comments := seedMarkComments[mark]
				markedAt := created.Add(time.Duration(rng.IntN(40)+2) * time.Hour)
				if markedAt.After(now) {
					markedAt = now
				}
				if _, err := tx.Exec(ctx, `INSERT INTO app.monday_update_marks(update_id,mentor_user_id,mark,comment_html,created_at,updated_at)
					VALUES($1,$2,$3,$4,$5,$5)`, updateID, mentorID, string(mark), "<p>"+comments[rng.IntN(len(comments))]+"</p>", markedAt); err != nil {
					return fmt.Errorf("seed monday update mark: %w", err)
				}
			}
			end = start.AddDate(0, 0, -1)
		}
		return nil
	}

	if err := insert(devStudentID, devMentorID, devStudentSeed); err != nil {
		return false, err
	}
	for _, st := range seedCohort {
		studentID, enrollmentID, err := enrol(seedCohortSlugPrefix+st.slug, st.name, "student", st.level)
		if err != nil {
			return false, fmt.Errorf("seed cohort student %s: %w", st.slug, err)
		}
		mentorID, mentorEnrollment := "", ""
		switch st.mentor {
		case "dev":
			mentorID, mentorEnrollment = devMentorID, devMentorEnrollment
		case "second":
			mentorID, mentorEnrollment = secondMentorID, secondMentorEnrollment
		}
		if mentorEnrollment != "" {
			if _, err := tx.Exec(ctx, `INSERT INTO app.mentorships(season_id,mentor_enrollment_id,student_enrollment_id) VALUES($1,$2,$3)`,
				seasonID, mentorEnrollment, enrollmentID); err != nil {
				return false, fmt.Errorf("seed cohort mentorship: %w", err)
			}
		}
		if err := insert(studentID, mentorID, st); err != nil {
			return false, err
		}
	}
	return true, nil
}

// seedUpdateHTML writes one plausible student update in the persona's style.
func seedUpdateHTML(rng *rand.Rand, p seedPersona, index, days int, hours float64) string {
	topicKey := p.topics[index%len(p.topics)]
	topics := seedTopicPool[topicKey]
	firstIndex := rng.IntN(len(topics))
	first, second := topics[firstIndex], topics[(firstIndex+1+rng.IntN(len(topics)-1))%len(topics)]
	win, struggle, next := seedWins[rng.IntN(len(seedWins))], seedStruggles[rng.IntN(len(seedStruggles))], seedNext[rng.IntN(len(seedNext))]
	span := fmt.Sprintf("these %d days", days)
	switch days {
	case 1:
		span = "today"
	case 7:
		span = "this week"
	case 14:
		span = "the last two weeks"
	}
	solved := int(hours/3) + rng.IntN(3)

	var b strings.Builder
	switch p.style {
	case 1:
		fmt.Fprintf(&b, "<p><strong>Focus %s:</strong> %s and %s.</p><ul>", span, first, second)
		fmt.Fprintf(&b, "<li>Solved %d problems (%d medium).</li>", solved, solved/2)
		fmt.Fprintf(&b, "<li>✅ %s</li><li>⚠️ %s</li></ul>", win, struggle)
		fmt.Fprintf(&b, "<p><strong>Next:</strong> %s</p>", next)
	case 2:
		fmt.Fprintf(&b, "<p>Worked on %s %s. %s</p>", first, span, struggle)
		if rng.IntN(2) == 0 {
			fmt.Fprintf(&b, "<p>Next: %s</p>", strings.ToLower(next[:1])+next[1:])
		}
	default:
		fmt.Fprintf(&b, "<p>%s I spent most of my time on <strong>%s</strong>, then moved on to %s. I put in about %g hours and finished %d problems.</p>",
			strings.ToUpper(span[:1])+span[1:]+",", first, second, hours, solved)
		fmt.Fprintf(&b, "<p><strong>What went well:</strong> %s</p>", win)
		fmt.Fprintf(&b, "<p><strong>What was hard:</strong> %s</p>", struggle)
		fmt.Fprintf(&b, "<p><strong>Plan for next week:</strong> %s</p>", next)
	}
	return b.String()
}
