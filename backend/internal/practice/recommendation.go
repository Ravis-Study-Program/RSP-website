package practice

import (
	"errors"
	"fmt"
	"sort"
	"time"
)

type Difficulty string

const (
	Easy   Difficulty = "easy"
	Medium Difficulty = "medium"
	Hard   Difficulty = "hard"
)

type Outcome string

const (
	Independent Outcome = "independently_solved"
	WithHints   Outcome = "solved_with_hints"
	NotSolved   Outcome = "not_solved"
	Unknown     Outcome = "unknown"
)

type Level string

const (
	Novice       Level = "novice"
	Beginner     Level = "beginner"
	Intermediate Level = "intermediate"
	Advanced     Level = "advanced"
	NonStudent   Level = "non_student"
)

type Goals map[Difficulty]int

func DefaultGoals() Goals { return Goals{Easy: 20, Medium: 35, Hard: 50} }

type Problem struct {
	ID         string     `json:"id"`
	Number     int        `json:"number"`
	Title      string     `json:"title"`
	Link       string     `json:"link"`
	Difficulty Difficulty `json:"difficulty"`
	Categories []string   `json:"categories"`
	Premium    bool       `json:"premium"`
	Revision   int64      `json:"revision"`
}
type Attempt struct {
	ProblemID   string
	Difficulty  Difficulty
	Categories  []string
	Outcome     Outcome
	Confidence  *int
	Minutes     int
	AttemptedAt time.Time
	Migrated    bool
}
type Dismissal struct {
	ProblemID   string    `json:"problemId"`
	DismissedAt time.Time `json:"dismissedAt"`
}
type ProblemHistory struct {
	LastAttemptedAt time.Time
	Weak            bool
}
type Criteria struct {
	Difficulty Difficulty
	Category   string
}
type Recommendation struct {
	ID          string     `json:"id"`
	UserID      string     `json:"userId"`
	Problem     Problem    `json:"problem"`
	Difficulty  Difficulty `json:"difficulty"`
	Category    string     `json:"category,omitempty"`
	Rationale   string     `json:"rationale"`
	RuleVersion string     `json:"ruleVersion"`
	CreatedAt   time.Time  `json:"createdAt"`
	Revision    int64      `json:"revision"`
	DismissedAt *time.Time `json:"dismissedAt,omitempty"`
	FulfilledAt *time.Time `json:"fulfilledAt,omitempty"`
}
type Request struct {
	UserID           string
	Level            Level
	PremiumOptIn     bool
	Problems         []Problem
	Attempts         []Attempt
	QualityAttempts  []Attempt
	ProblemHistory   map[string]ProblemHistory
	CategoryExposure map[string]int
	Dismissals       []Dismissal
	Active           *Recommendation
	Goals            Goals
	Now              time.Time
}

var ErrNoCandidate = errors.New("no suitable recommendation")

func Select(in Request) (Recommendation, error) {
	if in.Active != nil && in.Active.DismissedAt == nil && in.Active.FulfilledAt == nil {
		return *in.Active, nil
	}
	in.Goals = normalizedGoals(in.Goals)
	now := in.Now.UTC()
	criteria := criteriaFor(in)
	difficulty, category := criteria.Difficulty, criteria.Category
	seen, lastAttempt := map[string]bool{}, map[string]time.Time{}
	previouslyWeak := map[string]bool{}
	if in.ProblemHistory != nil {
		for problemID, history := range in.ProblemHistory {
			seen[problemID] = true
			lastAttempt[problemID] = history.LastAttemptedAt
			previouslyWeak[problemID] = history.Weak
		}
	} else {
		for _, a := range in.Attempts {
			seen[a.ProblemID] = true
			if a.AttemptedAt.After(lastAttempt[a.ProblemID]) {
				lastAttempt[a.ProblemID] = a.AttemptedAt
			}
			if !a.Migrated && (a.Outcome == NotSolved || a.Outcome == WithHints || (a.Confidence != nil && *a.Confidence < 4) || a.Minutes > in.Goals[a.Difficulty]) {
				previouslyWeak[a.ProblemID] = true
			}
		}
	}
	dismissed := map[string]bool{}
	for _, d := range in.Dismissals {
		if now.Sub(d.DismissedAt.UTC()) < 30*24*time.Hour {
			dismissed[d.ProblemID] = true
		}
	}
	candidates := filter(in.Problems, difficulty, category, in.PremiumOptIn, dismissed)
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].ID < candidates[j].ID })
	var chosen *Problem
	for i := range candidates {
		if !seen[candidates[i].ID] {
			chosen = &candidates[i]
			break
		}
	}
	if chosen == nil {
		for i := range candidates {
			last := lastAttempt[candidates[i].ID]
			if previouslyWeak[candidates[i].ID] && !last.IsZero() && now.Sub(last) >= 90*24*time.Hour {
				chosen = &candidates[i]
				break
			}
		}
	}
	if chosen == nil {
		return Recommendation{}, ErrNoCandidate
	}
	rationale := fmt.Sprintf("We chose a %s problem", difficulty)
	if category != "" {
		rationale += fmt.Sprintf(" in %s because recent attempts show that category needs practice", category)
	} else {
		rationale += " to match your current practice level"
	}
	if difficulty != baseDifficulty(in.Level) {
		rationale += fmt.Sprintf("; recent outcomes adjusted this from %s", baseDifficulty(in.Level))
	}
	return Recommendation{ID: "recommendation-" + in.UserID + "-" + chosen.ID, UserID: in.UserID, Problem: *chosen, Difficulty: difficulty, Category: category, Rationale: rationale, RuleVersion: "v1", CreatedAt: now}, nil
}

func CriteriaFor(in Request) Criteria {
	in.Goals = normalizedGoals(in.Goals)
	return criteriaFor(in)
}

func criteriaFor(in Request) Criteria {
	difficulty := baseDifficulty(in.Level)
	known := knownNewest(in.Attempts, 20)
	if in.QualityAttempts != nil {
		known = knownNewest(in.QualityAttempts, 20)
	}
	lastFive := known
	if len(lastFive) > 5 {
		lastFive = lastFive[:5]
	}
	independentStrong, withinGoal, failures, weak := 0, 0, 0, 0
	for _, a := range lastFive {
		goal := in.Goals[a.Difficulty]
		strong := a.Outcome == Independent && a.Confidence != nil && *a.Confidence >= 4
		if strong {
			independentStrong++
			if a.Minutes <= goal {
				withinGoal++
			}
		}
		if a.Outcome == NotSolved {
			failures++
		}
		if a.Outcome == WithHints || (a.Confidence != nil && *a.Confidence < 4) || a.Minutes > goal {
			weak++
		}
	}
	if independentStrong >= 4 && withinGoal >= 3 {
		difficulty = raise(difficulty)
	} else if failures >= 2 || weak >= 3 {
		difficulty = lower(difficulty)
	}
	category := weakestCategory(known, in.Attempts, in.Goals)
	if in.CategoryExposure != nil {
		category = weakestCategoryFromExposure(known, in.CategoryExposure, in.Goals)
	}
	return Criteria{Difficulty: difficulty, Category: category}
}

func normalizedGoals(goals Goals) Goals {
	defaults := DefaultGoals()
	normalized := Goals{}
	for _, difficulty := range []Difficulty{Easy, Medium, Hard} {
		normalized[difficulty] = goals[difficulty]
		if normalized[difficulty] <= 0 {
			normalized[difficulty] = defaults[difficulty]
		}
	}
	return normalized
}

func baseDifficulty(l Level) Difficulty {
	switch l {
	case Novice, Beginner:
		return Easy
	case Advanced:
		return Hard
	default:
		return Medium
	}
}
func raise(d Difficulty) Difficulty {
	if d == Easy {
		return Medium
	}
	if d == Medium {
		return Hard
	}
	return Hard
}
func lower(d Difficulty) Difficulty {
	if d == Hard {
		return Medium
	}
	if d == Medium {
		return Easy
	}
	return Easy
}
func knownNewest(all []Attempt, limit int) []Attempt {
	out := make([]Attempt, 0, len(all))
	for _, a := range all {
		if a.Outcome != Unknown && !a.Migrated {
			out = append(out, a)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].AttemptedAt.After(out[j].AttemptedAt) })
	if len(out) > limit {
		return out[:limit]
	}
	return out
}

type categoryStats struct {
	score, successes, exposure int
	latest                     time.Time
}

func weakestCategory(known, all []Attempt, goals Goals) string {
	stats := map[string]*categoryStats{}
	qualityKeys := map[string]int{}
	for i, a := range known {
		qualityKeys[attemptKey(a)]++
		weight := 20 - i
		for _, c := range a.Categories {
			s := stats[c]
			if s == nil {
				s = &categoryStats{}
				stats[c] = s
			}
			s.exposure++
			if a.AttemptedAt.After(s.latest) {
				s.latest = a.AttemptedAt
			}
			if a.Outcome == NotSolved {
				s.score += 4 * weight
			}
			if a.Outcome == WithHints {
				s.score += 3 * weight
			}
			if a.Confidence != nil && *a.Confidence < 4 {
				s.score += 2 * weight
			}
			if a.Minutes > goals[a.Difficulty] {
				s.score += weight
			}
			if a.Outcome == Independent {
				s.successes++
			}
		}
	}
	for _, a := range all {
		key := attemptKey(a)
		if qualityKeys[key] > 0 {
			qualityKeys[key]--
			continue
		}
		for _, c := range a.Categories {
			s := stats[c]
			if s == nil {
				s = &categoryStats{}
				stats[c] = s
			}
			s.exposure++
		}
	}
	names := make([]string, 0, len(stats))
	for n, s := range stats {
		if s.score > 0 {
			names = append(names, n)
		}
	}
	sort.Slice(names, func(i, j int) bool {
		a, b := stats[names[i]], stats[names[j]]
		if a.score != b.score {
			return a.score > b.score
		}
		if a.successes != b.successes {
			return a.successes < b.successes
		}
		if a.exposure != b.exposure {
			return a.exposure < b.exposure
		}
		return names[i] < names[j]
	})
	if len(names) == 0 {
		return ""
	}
	return names[0]
}

func weakestCategoryFromExposure(known []Attempt, exposure map[string]int, goals Goals) string {
	stats := map[string]*categoryStats{}
	for category, count := range exposure {
		stats[category] = &categoryStats{exposure: count}
	}
	for index, attempt := range known {
		weight := 20 - index
		for _, category := range attempt.Categories {
			stat := stats[category]
			if stat == nil {
				stat = &categoryStats{}
				stats[category] = stat
			}
			if attempt.AttemptedAt.After(stat.latest) {
				stat.latest = attempt.AttemptedAt
			}
			if attempt.Outcome == NotSolved {
				stat.score += 4 * weight
			}
			if attempt.Outcome == WithHints {
				stat.score += 3 * weight
			}
			if attempt.Confidence != nil && *attempt.Confidence < 4 {
				stat.score += 2 * weight
			}
			if attempt.Minutes > goals[attempt.Difficulty] {
				stat.score += weight
			}
			if attempt.Outcome == Independent {
				stat.successes++
			}
		}
	}
	names := make([]string, 0, len(stats))
	for name, stat := range stats {
		if stat.score > 0 {
			names = append(names, name)
		}
	}
	sort.Slice(names, func(i, j int) bool {
		a, b := stats[names[i]], stats[names[j]]
		if a.score != b.score {
			return a.score > b.score
		}
		if a.successes != b.successes {
			return a.successes < b.successes
		}
		if a.exposure != b.exposure {
			return a.exposure < b.exposure
		}
		return names[i] < names[j]
	})
	if len(names) == 0 {
		return ""
	}
	return names[0]
}

func attemptKey(a Attempt) string {
	return a.ProblemID + "\x00" + a.AttemptedAt.UTC().Format(time.RFC3339Nano)
}
func filter(all []Problem, d Difficulty, c string, premium bool, dismissed map[string]bool) []Problem {
	out := []Problem{}
	for _, p := range all {
		if p.Difficulty != d || (!premium && p.Premium) || dismissed[p.ID] {
			continue
		}
		if c != "" && !contains(p.Categories, c) {
			continue
		}
		out = append(out, p)
	}
	return out
}
func contains(xs []string, want string) bool {
	for _, x := range xs {
		if x == want {
			return true
		}
	}
	return false
}
