package dal

import (
	"context"
	"fmt"
	"math"
	"math/rand/v2"
	"time"

	"github.com/jackc/pgx/v5"
)

type seedProblem struct {
	number           int
	title, slug, dif string
}

// seedProblems are real LeetCode problems the development activity draws from,
// inserted only when the catalogue sync has not already provided them.
var seedProblems = []seedProblem{
	{1, "Two Sum", "two-sum", "easy"},
	{20, "Valid Parentheses", "valid-parentheses", "easy"},
	{21, "Merge Two Sorted Lists", "merge-two-sorted-lists", "easy"},
	{121, "Best Time to Buy and Sell Stock", "best-time-to-buy-and-sell-stock", "easy"},
	{125, "Valid Palindrome", "valid-palindrome", "easy"},
	{141, "Linked List Cycle", "linked-list-cycle", "easy"},
	{217, "Contains Duplicate", "contains-duplicate", "easy"},
	{226, "Invert Binary Tree", "invert-binary-tree", "easy"},
	{242, "Valid Anagram", "valid-anagram", "easy"},
	{704, "Binary Search", "binary-search", "easy"},
	{3, "Longest Substring Without Repeating Characters", "longest-substring-without-repeating-characters", "medium"},
	{11, "Container With Most Water", "container-with-most-water", "medium"},
	{15, "3Sum", "3sum", "medium"},
	{33, "Search in Rotated Sorted Array", "search-in-rotated-sorted-array", "medium"},
	{39, "Combination Sum", "combination-sum", "medium"},
	{49, "Group Anagrams", "group-anagrams", "medium"},
	{56, "Merge Intervals", "merge-intervals", "medium"},
	{79, "Word Search", "word-search", "medium"},
	{98, "Validate Binary Search Tree", "validate-binary-search-tree", "medium"},
	{102, "Binary Tree Level Order Traversal", "binary-tree-level-order-traversal", "medium"},
	{128, "Longest Consecutive Sequence", "longest-consecutive-sequence", "medium"},
	{198, "House Robber", "house-robber", "medium"},
	{200, "Number of Islands", "number-of-islands", "medium"},
	{207, "Course Schedule", "course-schedule", "medium"},
	{236, "Lowest Common Ancestor of a Binary Tree", "lowest-common-ancestor-of-a-binary-tree", "medium"},
	{238, "Product of Array Except Self", "product-of-array-except-self", "medium"},
	{300, "Longest Increasing Subsequence", "longest-increasing-subsequence", "medium"},
	{322, "Coin Change", "coin-change", "medium"},
	{347, "Top K Frequent Elements", "top-k-frequent-elements", "medium"},
	{435, "Non-overlapping Intervals", "non-overlapping-intervals", "medium"},
	{560, "Subarray Sum Equals K", "subarray-sum-equals-k", "medium"},
	{621, "Task Scheduler", "task-scheduler", "medium"},
	{4, "Median of Two Sorted Arrays", "median-of-two-sorted-arrays", "hard"},
	{23, "Merge k Sorted Lists", "merge-k-sorted-lists", "hard"},
	{42, "Trapping Rain Water", "trapping-rain-water", "hard"},
	{51, "N-Queens", "n-queens", "hard"},
	{76, "Minimum Window Substring", "minimum-window-substring", "hard"},
	{295, "Find Median from Data Stream", "find-median-from-data-stream", "hard"},
	{297, "Serialize and Deserialize Binary Tree", "serialize-and-deserialize-binary-tree", "hard"},
	{312, "Burst Balloons", "burst-balloons", "hard"},
}

var seedAttemptNotes = []string{
	"Needed a hint for the invariant, then it clicked.",
	"Brute force first, then optimised with a hash map.",
	"Off-by-one on the window bounds cost me ten minutes.",
	"Redo this one next week without notes.",
	"Clean first attempt; talked through complexity out loud.",
	"Forgot the empty-input case until testing.",
}

var seedMockNotes = []string{
	"Clarified constraints well. Jumped into code before confirming the approach.",
	"Strong communication. Complexity analysis needs to be stated unprompted.",
	"Good recovery after a wrong first idea. Test with edge cases earlier.",
	"Solid STAR structure; quantify the impact in the result.",
	"Rushed the coding phase. Slow down and narrate the invariant.",
}

var seedMockReflections = []string{
	"Nervous at the start but settled once I wrote out examples.",
	"I should have asked about input size before choosing an approach.",
	"Felt much better than last time; the practice is paying off.",
	"Ran out of time on testing. Need to leave five minutes at the end.",
}

type seedPoolProblem struct {
	problemID, leetcodeID, difficulty string
}

// seedMondayActivity adds LeetCode attempts and mock interviews inside the date
// range of every development Monday update, scaled by the reported hours and
// shaped by the update's mentor mark.
func seedMondayActivity(ctx context.Context, tx pgx.Tx, seasonID string, now time.Time) error {
	adelaide, err := time.LoadLocation("Australia/Adelaide")
	if err != nil {
		return err
	}
	for _, p := range seedProblems {
		if _, err := tx.Exec(ctx, `WITH problem AS (
			INSERT INTO app.problems(title,url) SELECT $2,$3
			WHERE NOT EXISTS (SELECT 1 FROM app.leetcode_problems WHERE leetcode_number=$1) RETURNING id)
			INSERT INTO app.leetcode_problems(problem_id,leetcode_number,difficulty,is_premium)
			SELECT id,$1,$4::app.problem_difficulty,false FROM problem`,
			p.number, p.title, "https://leetcode.com/problems/"+p.slug+"/", p.dif); err != nil {
			return fmt.Errorf("seed problem %d: %w", p.number, err)
		}
	}
	pool := map[string][]seedPoolProblem{}
	rows, err := tx.Query(ctx, `SELECT l.problem_id,l.id,l.difficulty::text FROM app.leetcode_problems l
		JOIN app.problems p ON p.id=l.problem_id
		WHERE l.deleted_at IS NULL AND p.deleted_at IS NULL AND NOT l.is_premium ORDER BY l.leetcode_number`)
	if err != nil {
		return err
	}
	for rows.Next() {
		var item seedPoolProblem
		if err := rows.Scan(&item.problemID, &item.leetcodeID, &item.difficulty); err != nil {
			rows.Close()
			return err
		}
		pool[item.difficulty] = append(pool[item.difficulty], item)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	var seasonStart, seasonEnd time.Time
	var coordinatorID string
	if err := tx.QueryRow(ctx, `SELECT s.start_at,s.end_at,
		COALESCE((SELECT e.user_id::text FROM app.enrollments e WHERE e.season_id=s.id AND e.role='coordinator' AND e.state='active' AND e.deleted_at IS NULL LIMIT 1),'')
		FROM app.seasons s WHERE s.id=$1`, seasonID).Scan(&seasonStart, &seasonEnd, &coordinatorID); err != nil {
		return err
	}

	type seedUpdate struct {
		studentID, enrollmentID, mentorID, mark string
		start, end                              time.Time
		hours                                   float64
	}
	var updates []seedUpdate
	rows, err = tx.Query(ctx, `SELECT u.student_user_id,e.id,
		COALESCE((SELECT me.user_id::text FROM app.mentorships m JOIN app.enrollments me ON me.id=m.mentor_enrollment_id
			WHERE m.student_enrollment_id=e.id AND m.ended_at IS NULL AND m.deleted_at IS NULL LIMIT 1),''),
		COALESCE((SELECT k.mark::text FROM app.monday_update_marks k WHERE k.update_id=u.id LIMIT 1),'on_track'),
		u.start_date,u.end_date,u.hours::float8
		FROM app.monday_updates u
		JOIN app.user_profiles p ON p.user_id=u.student_user_id
		JOIN app.enrollments e ON e.user_id=u.student_user_id AND e.season_id=u.season_id AND e.role='student'
		WHERE u.season_id=$1 AND u.deleted_at IS NULL AND (p.slug LIKE $2 OR p.slug='dev-student')
		ORDER BY u.student_user_id,u.start_date`, seasonID, seedCohortSlugPrefix+"%")
	if err != nil {
		return err
	}
	for rows.Next() {
		var u seedUpdate
		if err := rows.Scan(&u.studentID, &u.enrollmentID, &u.mentorID, &u.mark, &u.start, &u.end, &u.hours); err != nil {
			rows.Close()
			return err
		}
		updates = append(updates, u)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	skillByMark := map[string]float64{"way_ahead": 0.85, "doing_good": 0.7, "on_track": 0.55, "a_bit_behind": 0.4, "at_risk": 0.25}
	rng := rand.New(rand.NewPCG(2026, 11))
	// at returns a local Adelaide instant on one day of the range, or false when
	// that instant is still in the future.
	at := func(u seedUpdate, fromHour, toHour int) (time.Time, bool) {
		days := int(u.end.Sub(u.start).Hours()/24) + 1
		day := u.start.AddDate(0, 0, rng.IntN(days))
		instant := time.Date(day.Year(), day.Month(), day.Day(), fromHour+rng.IntN(toHour-fromHour), rng.IntN(60), 0, 0, adelaide)
		return instant, instant.Before(now)
	}
	pick := func(skill float64) seedPoolProblem {
		roll := rng.Float64()
		difficulty := "medium"
		switch {
		case roll < 0.35-skill*0.2:
			difficulty = "easy"
		case roll > 0.75+0.15*(1-skill):
			difficulty = "hard"
		}
		if len(pool[difficulty]) == 0 {
			for _, items := range pool {
				if len(items) > 0 {
					return items[rng.IntN(len(items))]
				}
			}
		}
		items := pool[difficulty]
		return items[rng.IntN(len(items))]
	}
	score := func(skill float64) int {
		return max(0, min(10, int(math.Round(skill*10+rng.NormFloat64()*1.5))))
	}

	for _, u := range updates {
		skill := skillByMark[u.mark]
		attempts := int(math.Round(u.hours / 2.5 * (0.6 + rng.Float64()*0.6)))
		for range min(attempts, 30) {
			when, ok := at(u, 8, 23)
			if !ok {
				continue
			}
			problem := pick(skill)
			roll := rng.Float64()
			outcome, confidence := "not_solved", any(1+rng.IntN(2))
			switch {
			case roll < 0.05:
				outcome, confidence = "unknown", nil
			case roll < 0.05+skill*0.7:
				outcome, confidence = "independently_solved", 4+rng.IntN(2)
			case roll < 0.05+skill*0.7+0.25:
				outcome, confidence = "solved_with_hints", 2+rng.IntN(3)
			}
			base := map[string]int{"easy": 8, "medium": 18, "hard": 35}[problem.difficulty]
			minutes := base + rng.IntN(base*2)
			var notes any
			if rng.IntN(3) == 0 {
				notes = "<p>" + seedAttemptNotes[rng.IntN(len(seedAttemptNotes))] + "</p>"
			}
			if _, err := tx.Exec(ctx, `INSERT INTO app.problem_attempts(user_id,problem_id,enrollment_id,attempted_at,time_taken_minutes,outcome,confidence,notes_html)
				VALUES($1,$2,$3,$4,$5,$6::app.attempt_outcome,$7,$8)`, u.studentID, problem.problemID, u.enrollmentID, when, minutes, outcome, confidence, notes); err != nil {
				return fmt.Errorf("seed attempt: %w", err)
			}
		}

		mocks := 0
		switch {
		case u.hours >= 25:
			mocks = 1 + rng.IntN(2)
		case u.hours >= 10:
			mocks = rng.IntN(2)
		}
		interviewer := u.mentorID
		if interviewer == "" || rng.IntN(4) == 0 {
			interviewer = coordinatorID
		}
		if interviewer == "" || interviewer == u.studentID {
			continue
		}
		for range mocks {
			when, ok := at(u, 17, 21)
			if !ok {
				continue
			}
			duration := []int{45, 60, 60, 90}[rng.IntN(4)]
			var season any
			if !when.Before(seasonStart) && !when.After(seasonEnd) {
				season = seasonID
			}
			var mockID string
			if err := tx.QueryRow(ctx, `INSERT INTO app.mock_interviews(interviewer_user_id,interviewee_user_id,season_id,scheduled_at,duration_minutes,interviewer_notes_html)
				VALUES($1,$2,$3,$4,$5,$6) RETURNING id`, interviewer, u.studentID, season, when, duration,
				"<p>"+seedMockNotes[rng.IntN(len(seedMockNotes))]+"</p>").Scan(&mockID); err != nil {
				return fmt.Errorf("seed mock interview: %w", err)
			}
			kinds := []string{"leetcode"}
			if duration >= 60 && rng.IntN(2) == 0 {
				kinds = append(kinds, "behavioural")
			}
			for position, kind := range kinds {
				reviewedAt := when.Add(time.Duration(duration)*time.Minute + time.Duration(1+rng.IntN(30))*time.Hour)
				status, comment := "pending", any(nil)
				var reviewed any
				if reviewedAt.Before(now) && rng.IntN(5) > 0 {
					status, reviewed = "reviewed", reviewedAt
					comment = "<p>" + seedMockReflections[rng.IntN(len(seedMockReflections))] + "</p>"
				}
				var roundID string
				if err := tx.QueryRow(ctx, `INSERT INTO app.mock_interview_rounds(mock_interview_id,position,kind,review_status,interviewee_comment_html,reviewed_at)
					VALUES($1,$2,$3::app.mock_round_kind,$4::app.review_status,$5,$6) RETURNING id`,
					mockID, position+1, kind, status, comment, reviewed).Scan(&roundID); err != nil {
					return fmt.Errorf("seed mock round: %w", err)
				}
				if kind == "behavioural" {
					_, err = tx.Exec(ctx, `INSERT INTO app.behavioural_mock_interview_rounds(mock_interview_round_id,behavioural_score) VALUES($1,$2)`, roundID, score(skill))
				} else {
					problem := pick(skill)
					_, err = tx.Exec(ctx, `INSERT INTO app.leetcode_mock_interview_rounds(mock_interview_round_id,leetcode_problem_id,
						clarify_question_score,algorithm_design_score,complexity_analysis_score,coding_score,testing_score)
						VALUES($1,$2,$3,$4,$5,$6,$7)`, roundID, problem.leetcodeID, score(skill), score(skill), score(skill), score(skill), score(skill))
				}
				if err != nil {
					return fmt.Errorf("seed mock round detail: %w", err)
				}
			}
		}
	}
	return nil
}
