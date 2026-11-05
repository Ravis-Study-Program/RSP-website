package practice

import "time"

// Outcome is the result of a problem attempt.
type Outcome string

// Stored outcomes. The database keeps all four, but the API only speaks
// Solved and Unsolved (see PublicOutcome).
const (
	Independent Outcome = "independently_solved"
	WithHints   Outcome = "solved_with_hints"
	NotSolved   Outcome = "not_solved"
	Unknown     Outcome = "unknown"
)

// Outcomes as the API reads and writes them.
const (
	Solved   = "solved"
	Unsolved = "unsolved"
)

// SolvedOutcomes are the stored outcomes that count as solving a problem:
// independently solved, and older attempts whose outcome was never recorded.
// Solving with hints counts as unsolved.
func SolvedOutcomes() []Outcome { return []Outcome{Independent, Unknown} }

// UnsolvedOutcomes are the stored outcomes that read as Unsolved.
func UnsolvedOutcomes() []Outcome { return []Outcome{WithHints, NotSolved} }

// PublicOutcome maps a stored outcome to the API's Solved or Unsolved.
func PublicOutcome(stored string) string {
	for _, outcome := range SolvedOutcomes() {
		if stored == string(outcome) {
			return Solved
		}
	}
	return Unsolved
}

// StoredOutcome maps the API's Solved or Unsolved to the outcome saved for a
// new or edited attempt.
func StoredOutcome(public string) (Outcome, bool) {
	switch public {
	case Solved:
		return Independent, true
	case Unsolved:
		return NotSolved, true
	}
	return "", false
}

// OutcomeNames lists outcomes as strings, for SQL parameters.
func OutcomeNames(outcomes []Outcome) []string {
	names := make([]string, len(outcomes))
	for i, outcome := range outcomes {
		names[i] = string(outcome)
	}
	return names
}

// PracticeSettings describes the fixed programme time goals.
type PracticeSettings struct {
	GoalsEnabled  bool `json:"goalsEnabled"`
	EasyMinutes   int  `json:"easyMinutes"`
	MediumMinutes int  `json:"mediumMinutes"`
	HardMinutes   int  `json:"hardMinutes"`
}

// FixedSettings applies to every member, including members with legacy personal goals.
func FixedSettings() PracticeSettings {
	return PracticeSettings{GoalsEnabled: true, EasyMinutes: 20, MediumMinutes: 35, HardMinutes: 50}
}

// ProblemRecord is the persisted and HTTP-facing problem representation.
type ProblemRecord struct {
	ID         string   `json:"id"`
	Number     int      `json:"number"`
	Title      string   `json:"title"`
	Slug       string   `json:"slug"`
	Link       string   `json:"link"`
	Difficulty string   `json:"difficulty"`
	Categories []string `json:"categories"`
	Premium    bool     `json:"premium"`
}

// AttemptRecord is the persisted and HTTP-facing attempt representation.
type AttemptRecord struct {
	ID        string `json:"id"`
	UserID    string `json:"-"`
	ProblemID string `json:"problemId"`
	// Problem is attached for responses; it is not persisted with the attempt.
	Problem *ProblemRecord `json:"problem,omitempty"`
	// Outcome is Solved or Unsolved, mapped from the stored outcome on read.
	Outcome string `json:"outcome"`
	Minutes int    `json:"minutes"`
	// Reviewed is set by the attempt's owner once they have reviewed it.
	Reviewed    bool       `json:"reviewed"`
	Notes       string     `json:"notes"`
	AttemptedAt time.Time  `json:"attemptedAt"`
	SeasonID    *string    `json:"seasonId,omitempty"`
	WeekID      *string    `json:"weekId,omitempty"`
	DeletedAt   *time.Time `json:"-"`
}
