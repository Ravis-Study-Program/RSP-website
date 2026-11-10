package dal

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/magedmg/RSP-website/backend/internal/platform/id"
	"github.com/magedmg/RSP-website/backend/internal/program"
	"github.com/magedmg/RSP-website/backend/internal/worker"
)

// seedCohortSlugPrefix marks the extra development students so the Monday
// update fixture is created once and skipped on every later run.
const seedCohortSlugPrefix = "dev-cohort-"

type seedCohortStudent struct {
	slug, name, level string
	mentor            string // "dev" (the seeded mentor), "second" or "" for none
	persona           seedPersona
	// posted marks, newest first, each ended program week the student posted
	// an update for ('x') or skipped ('-'). Weeks past its end have none.
	posted string
}

type seedPersona struct {
	trend  []program.MondayUpdateMark // cycles over a student's updates
	hours  []float64                  // cycles over a student's updates
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

var seedMarkComments = map[program.MondayUpdateMark][]string{
	program.MarkWayAhead:   {"Outstanding consistency. Start mixing in harder problems and timed mocks.", "You're well ahead of the plan; keep the reflection quality this high."},
	program.MarkDoingGood:  {"Strong week. Push one more mock before the end of the month.", "Good progress and honest reflection. Keep the mistakes log going."},
	program.MarkOnTrack:    {"Steady progress. Try to narrate the invariant before you start typing.", "On pace. Next step is solving mediums without peeking at hints."},
	program.MarkABitBehind: {"A lighter stretch. Let's plan smaller daily goals so uni doesn't swallow the week.", "Fewer problems than planned. Pick two topics and go deep rather than wide."},
	program.MarkAtRisk:     {"Let's catch up this week; book a 1:1 so we can reset the plan.", "Very little activity recently. Please reach out so we can adjust your schedule."},
}

var seedCohort = []seedCohortStudent{
	{"amelia-chen", "Amelia Chen", "intermediate", "dev", seedPersona{
		trend: []program.MondayUpdateMark{program.MarkDoingGood, program.MarkWayAhead, program.MarkDoingGood},
		hours: []float64{32, 28.5, 35, 30}, topics: []string{"hashing", "trees", "graphs", "dp"}}, "xxxxxxxx"},
	{"marcus-oyelaran", "Marcus Oyelaran", "beginner", "dev", seedPersona{
		trend: []program.MondayUpdateMark{program.MarkABitBehind, program.MarkOnTrack, program.MarkAtRisk},
		hours: []float64{12, 8.5, 15, 4}, topics: []string{"arrays", "hashing", "behaviour"}, style: 2}, "x-xx-xx"},
	{"zara-ahmed", "Zara Ahmed", "advanced", "dev", seedPersona{
		trend: []program.MondayUpdateMark{program.MarkWayAhead},
		hours: []float64{42, 38, 45.5, 40}, topics: []string{"graphs", "dp", "system", "heaps"}, style: 1}, "xxxxxxxxx"},
	{"liam-nguyen", "Liam Nguyen", "novice", "dev", seedPersona{
		trend: []program.MondayUpdateMark{program.MarkOnTrack, program.MarkABitBehind},
		hours: []float64{18, 20, 16.5}, topics: []string{"arrays", "hashing", "intervals"}}, "xxxx"},
	{"sofia-rossi", "Sofia Rossi", "intermediate", "dev", seedPersona{
		trend: []program.MondayUpdateMark{program.MarkOnTrack, program.MarkDoingGood},
		hours: []float64{25, 22, 30, 27.5}, topics: []string{"intervals", "heaps", "backtrack", "behaviour"}, style: 1}, "xxx-xxxx"},
	{"noah-williams", "Noah Williams", "beginner", "second", seedPersona{
		trend: []program.MondayUpdateMark{program.MarkAtRisk, program.MarkABitBehind},
		hours: []float64{3, 6.5, 0, 10}, topics: []string{"arrays", "behaviour"}, style: 2}, "-x--x"},
	{"priyanka-das", "Priyanka Das", "advanced", "second", seedPersona{
		trend: []program.MondayUpdateMark{program.MarkDoingGood, program.MarkWayAhead},
		hours: []float64{36, 40, 33.5}, topics: []string{"system", "graphs", "dp"}}, "xxxxxxxxx"},
	{"ethan-park", "Ethan Park", "intermediate", "second", seedPersona{
		trend: []program.MondayUpdateMark{program.MarkOnTrack},
		hours: []float64{20, 24, 19.5, 26}, topics: []string{"trees", "backtrack", "heaps"}, style: 1}, "xxxxxxx"},
	{"hannah-kowalski", "Hannah Kowalski", "novice", "second", seedPersona{
		trend: []program.MondayUpdateMark{program.MarkABitBehind, program.MarkOnTrack},
		hours: []float64{14, 11, 16}, topics: []string{"arrays", "hashing", "trees"}}, "xx-xx"},
	{"diego-martinez", "Diego Martínez", "intermediate", "", seedPersona{
		trend: []program.MondayUpdateMark{program.MarkOnTrack},
		hours: []float64{22, 18.5, 27}, topics: []string{"intervals", "dp", "behaviour"}, style: 2}, "xxxx"},
	{"aisha-bello", "Aisha Bello", "beginner", "", seedPersona{
		trend: []program.MondayUpdateMark{program.MarkOnTrack},
		hours: []float64{15, 17.5, 12}, topics: []string{"arrays", "hashing"}, style: 1}, "xxx"},
	{"oliver-smith", "Oliver Smith", "advanced", "", seedPersona{
		trend: []program.MondayUpdateMark{program.MarkDoingGood},
		hours: []float64{50, 45, 55.5}, topics: []string{"system", "graphs"}}, "xxxxxxxxxx"},
}

// devStudentSeed gives the seeded (log-in-able) student their own history,
// with the latest week still to write.
var devStudentSeed = seedCohortStudent{persona: seedPersona{
	trend: []program.MondayUpdateMark{program.MarkOnTrack, program.MarkDoingGood, program.MarkABitBehind},
	hours: []float64{30, 26, 33.5, 21}, topics: []string{"arrays", "hashing", "trees", "behaviour"}}, posted: "-xx-xx"}

// seedMondayUpdates adds a development cohort to dev-season (extra students,
// a second mentor, mentorships and Monday updates for the season's ended
// program weeks with mentor marks) plus LeetCode attempts and mock interviews inside those
// ranges, so mentor and reviewer screens have realistic volume. Seed runs it;
// each part is created once and a later run leaves an existing part untouched.
func (s *Store) seedMondayUpdates(ctx context.Context, now time.Time, adminMentorEmail string) error {
	tx, err := s.conn().Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())
	// The worker may sync the problem catalogue concurrently.
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, worker.LeetCodeAdvisoryLock); err != nil {
		return err
	}

	var seasonID string
	if err := tx.QueryRow(ctx, `SELECT id FROM app.seasons WHERE slug='dev-season'`).Scan(&seasonID); err != nil {
		return fmt.Errorf("dev-season is missing: %w", noRows(err))
	}
	var cohortExists, activityExists bool
	if err := tx.QueryRow(ctx, `SELECT
		EXISTS(SELECT 1 FROM app.user_profiles WHERE slug LIKE $1),
		EXISTS(SELECT 1 FROM app.problem_attempts a JOIN app.user_profiles p ON p.user_id=a.user_id WHERE p.slug LIKE $1)
		OR EXISTS(SELECT 1 FROM app.mock_interviews m JOIN app.user_profiles p ON p.user_id=m.interviewee_user_id WHERE p.slug LIKE $1)`,
		seedCohortSlugPrefix+"%").Scan(&cohortExists, &activityExists); err != nil {
		return err
	}
	if !cohortExists {
		if err := seedMondayCohort(ctx, tx, seasonID, now); err != nil {
			return err
		}
	}
	if !activityExists {
		if err := seedMondayActivity(ctx, tx, seasonID, now); err != nil {
			return err
		}
	}
	if err := seedKickedStudent(ctx, tx, seasonID, now); err != nil {
		return err
	}
	if err := seedAdminMentor(ctx, tx, seasonID, adminMentorEmail); err != nil {
		return err
	}
	// Seeded members are enrolled today but have activity from earlier in the
	// season; start their participation with the season so that activity
	// counts towards it.
	if _, err := tx.Exec(ctx, `UPDATE app.enrollment_participation p SET started_at=s.start_at
		FROM app.enrollments e JOIN app.seasons s ON s.id=e.season_id
		WHERE p.enrollment_id=e.id AND s.id=$1 AND p.started_at > s.start_at
		  AND (p.ended_at IS NULL OR p.ended_at >= s.start_at)`, seasonID); err != nil {
		return fmt.Errorf("seed participation start: %w", err)
	}
	return tx.Commit(ctx)
}

func seedMondayCohort(ctx context.Context, tx pgx.Tx, seasonID string, now time.Time) error {
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
		return err
	}
	devMentorID, devMentorEnrollment, err := enrollmentFor("dev-mentor", "mentor")
	if err != nil {
		return err
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
		return fmt.Errorf("seed second mentor: %w", err)
	}

	rng := rand.New(rand.NewPCG(2026, 10))
	// Updates are posted against the season's program weeks that have ended,
	// newest first, as their Monday-to-Sunday dates.
	type seedWeek struct{ start, end time.Time }
	var weeks []seedWeek
	rows, err := tx.Query(ctx, `SELECT (start_at AT TIME ZONE 'Australia/Adelaide')::date,(end_at AT TIME ZONE 'Australia/Adelaide')::date
		FROM app.season_weeks WHERE season_id=$1 AND deleted_at IS NULL AND end_at < $2
		ORDER BY start_at DESC`, seasonID, now)
	if err != nil {
		return err
	}
	for rows.Next() {
		var w seedWeek
		if err := rows.Scan(&w.start, &w.end); err != nil {
			rows.Close()
			return err
		}
		weeks = append(weeks, w)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	insert := func(studentID, mentorID string, st seedCohortStudent) error {
		for i, week := range weeks {
			if i >= len(st.posted) || st.posted[i] != 'x' {
				continue
			}
			hours := st.persona.hours[i%len(st.persona.hours)]
			created := week.end.Add(33 * time.Hour) // Monday morning after the week, Adelaide-ish
			if created.After(now) {
				created = now.Add(-time.Duration(rng.IntN(48)+1) * time.Hour)
			}
			var updateID string
			if err := tx.QueryRow(ctx, `INSERT INTO app.monday_updates(season_id,student_user_id,start_date,end_date,content_html,hours,created_at)
				VALUES($1,$2,$3::date,$4::date,$5,$6,$7) RETURNING id`, seasonID, studentID, week.start.Format(time.DateOnly), week.end.Format(time.DateOnly),
				seedUpdateHTML(rng, st.persona, i, 7, hours), hours, created).Scan(&updateID); err != nil {
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
		}
		return nil
	}

	if err := insert(devStudentID, devMentorID, devStudentSeed); err != nil {
		return err
	}
	for _, st := range seedCohort {
		studentID, enrollmentID, err := enrol(seedCohortSlugPrefix+st.slug, st.name, "student", st.level)
		if err != nil {
			return fmt.Errorf("seed cohort student %s: %w", st.slug, err)
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
				return fmt.Errorf("seed cohort mentorship: %w", err)
			}
		}
		if err := insert(studentID, mentorID, st); err != nil {
			return err
		}
	}
	return nil
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

// seedKickedStudent adds a student, mentored by the seeded mentor, who posted
// every week before Week 6 and was kicked partway through it, so the kicked
// mark can be tried out. It runs once, even on an existing fixture.
func seedKickedStudent(ctx context.Context, tx pgx.Tx, seasonID string, now time.Time) error {
	slug := seedCohortSlugPrefix + "tomas-reyes"
	var exists bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM app.user_profiles WHERE slug=$1)`, slug).Scan(&exists); err != nil || exists {
		return err
	}
	var mentorID, mentorEnrollmentID string
	if err := tx.QueryRow(ctx, `SELECT e.user_id,e.id FROM app.enrollments e JOIN app.user_profiles p ON p.user_id=e.user_id
		WHERE p.slug='dev-mentor' AND e.season_id=$1 AND e.role='mentor' AND e.state='active'`, seasonID).Scan(&mentorID, &mentorEnrollmentID); err != nil {
		return fmt.Errorf("seed kicked student: dev-mentor is missing: %w", noRows(err))
	}
	// Week 6, or the latest ended week when Week 6 has not ended yet.
	var kickStart time.Time
	var kickNumber int
	if err := tx.QueryRow(ctx, `SELECT start_at,week_number FROM app.season_weeks
		WHERE season_id=$1 AND deleted_at IS NULL AND end_at < $2
		ORDER BY (week_number=6) DESC,week_number DESC LIMIT 1`, seasonID, now).Scan(&kickStart, &kickNumber); err != nil {
		return fmt.Errorf("seed kicked student: no ended program week: %w", noRows(err))
	}

	studentID, err := resolveSeedUser(ctx, tx, seedUser{FallbackEmail: slug + "@rsp.local", Slug: slug, DisplayName: "Tomás Reyes"})
	if err != nil {
		return err
	}
	var enrollmentID string
	if err := tx.QueryRow(ctx, `INSERT INTO app.enrollments(user_id,season_id,role,student_level,state) VALUES($1,$2,'student','beginner','active') RETURNING id`,
		studentID, seasonID).Scan(&enrollmentID); err != nil {
		return fmt.Errorf("seed kicked student enrollment: %w", err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO app.mentorships(season_id,mentor_enrollment_id,student_enrollment_id) VALUES($1,$2,$3)`,
		seasonID, mentorEnrollmentID, enrollmentID); err != nil {
		return fmt.Errorf("seed kicked student mentorship: %w", err)
	}
	// One update for each week before the week they were kicked.
	if _, err := tx.Exec(ctx, `INSERT INTO app.monday_updates(season_id,student_user_id,start_date,end_date,content_html,hours,created_at)
		SELECT $1,$2,(start_at AT TIME ZONE 'Australia/Adelaide')::date,(end_at AT TIME ZONE 'Australia/Adelaide')::date,
			'<p>Kept going with arrays and hashing; struggled to find time this week.</p>',12,end_at + interval '33 hours'
		FROM app.season_weeks WHERE season_id=$1 AND deleted_at IS NULL AND week_number < $3`, seasonID, studentID, kickNumber); err != nil {
		return fmt.Errorf("seed kicked student updates: %w", err)
	}
	// Kicked on the Wednesday of that week.
	kickedAt := kickStart.Add(2*24*time.Hour + 12*time.Hour)
	if kickedAt.After(now) {
		kickedAt = now.Add(-time.Hour)
	}
	if _, err := tx.Exec(ctx, `UPDATE app.enrollments SET state='kicked',state_changed_at=$2,assignment_state='revoked',activated_at=NULL WHERE id=$1`,
		enrollmentID, kickedAt); err != nil {
		return fmt.Errorf("seed kicked student removal: %w", err)
	}
	if _, err := tx.Exec(ctx, `UPDATE app.mentorships SET ended_at=$2 WHERE student_enrollment_id=$1 AND ended_at IS NULL`, enrollmentID, kickedAt); err != nil {
		return fmt.Errorf("seed kicked student mentorship end: %w", err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO app.enrollment_removal_events(id,enrollment_id,season_id,subject_user_id,actor_user_id,resulting_state,reason,occurred_at)
		VALUES($1,$2,$3,$4,$5,'kicked','Stopped attending sessions and mock interviews.',$6)`,
		id.New(), enrollmentID, seasonID, studentID, mentorID, kickedAt); err != nil {
		return fmt.Errorf("seed kicked student removal event: %w", err)
	}
	return nil
}

// adminMentorStudents are the fake students given to the seeded administrator
// who also mentors in dev-season, so one account can try both roles.
var adminMentorStudents = []struct{ slug, name, level string }{
	{"dev-admin-mentee-1", "Imani Okafor", "novice"},
	{"dev-admin-mentee-2", "Lucas Meyer", "beginner"},
	{"dev-admin-mentee-3", "Hana Sato", "intermediate"},
}

// seedAdminMentor makes sure dev-season has one account that is both a System
// Admin and a mentor, with a few fake students. email names an existing
// verified account to use; empty creates one. Each part is added only when it
// is missing, so a later run leaves them untouched.
func seedAdminMentor(ctx context.Context, tx pgx.Tx, seasonID, email string) error {
	var adminID string
	err := tx.QueryRow(ctx, `SELECT user_id FROM app.user_profiles WHERE slug='dev-admin-mentor'`).Scan(&adminID)
	if errors.Is(err, pgx.ErrNoRows) {
		adminID, err = resolveSeedUser(ctx, tx, seedUser{FallbackEmail: "admin-mentor@rsp.local", Email: email, Slug: "dev-admin-mentor", DisplayName: "Dev Admin Mentor"})
	}
	if err != nil {
		return fmt.Errorf("seed admin mentor: %w", err)
	}
	// The role starts pending; a trigger activates it when MFA is configured.
	if _, err := tx.Exec(ctx, `INSERT INTO app.global_role_assignments(user_id,role) VALUES($1,'system_admin') ON CONFLICT (user_id,role) DO NOTHING`, adminID); err != nil {
		return fmt.Errorf("seed admin mentor role: %w", err)
	}
	var mentorEnrollment string
	err = tx.QueryRow(ctx, `SELECT id FROM app.enrollments WHERE user_id=$1 AND season_id=$2`, adminID, seasonID).Scan(&mentorEnrollment)
	if errors.Is(err, pgx.ErrNoRows) {
		err = tx.QueryRow(ctx, `INSERT INTO app.enrollments(user_id,season_id,role,state) VALUES($1,$2,'mentor','active') RETURNING id`,
			adminID, seasonID).Scan(&mentorEnrollment)
	}
	if err != nil {
		return fmt.Errorf("seed admin mentor enrollment: %w", err)
	}
	var hasMentees bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM app.mentorships WHERE mentor_enrollment_id=$1)`, mentorEnrollment).Scan(&hasMentees); err != nil {
		return err
	}
	if hasMentees {
		return nil
	}
	for _, student := range adminMentorStudents {
		userID, err := resolveSeedUser(ctx, tx, seedUser{FallbackEmail: student.slug + "@rsp.local", Slug: student.slug, DisplayName: student.name})
		if err != nil {
			return fmt.Errorf("seed admin mentor student %s: %w", student.slug, err)
		}
		var enrollmentID string
		if err := tx.QueryRow(ctx, `INSERT INTO app.enrollments(user_id,season_id,role,student_level,state) VALUES($1,$2,'student',$3,'active') RETURNING id`,
			userID, seasonID, student.level).Scan(&enrollmentID); err != nil {
			return fmt.Errorf("seed admin mentor student %s: %w", student.slug, err)
		}
		if _, err := tx.Exec(ctx, `INSERT INTO app.mentorships(season_id,mentor_enrollment_id,student_enrollment_id) VALUES($1,$2,$3)`,
			seasonID, mentorEnrollment, enrollmentID); err != nil {
			return fmt.Errorf("seed admin mentor mentorship: %w", err)
		}
	}
	return nil
}
