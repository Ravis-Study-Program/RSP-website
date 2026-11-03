package practice

import "time"

// Outcome is the result of a problem attempt.
type Outcome string

const (
	Independent Outcome = "independently_solved"
	WithHints   Outcome = "solved_with_hints"
	NotSolved   Outcome = "not_solved"
)

// SolvedOutcomes are the attempt outcomes that count as solving a problem.
// Not solved and unknown outcomes never count.
func SolvedOutcomes() []Outcome { return []Outcome{Independent, WithHints} }

// Goals are the Easy, Medium and Hard time goals, in minutes.
type Goals struct {
	EasyMinutes   int `json:"easyMinutes"`
	MediumMinutes int `json:"mediumMinutes"`
	HardMinutes   int `json:"hardMinutes"`
}

// MaxGoalMinutes matches the database range for every goal.
const MaxGoalMinutes = 180

// DefaultGoals apply until a Director or System Admin saves programme goals.
func DefaultGoals() Goals { return Goals{EasyMinutes: 10, MediumMinutes: 25, HardMinutes: 40} }

// Valid reports whether every goal is a whole number of minutes from 1 to 180.
func (g Goals) Valid() bool {
	for _, minutes := range []int{g.EasyMinutes, g.MediumMinutes, g.HardMinutes} {
		if minutes < 1 || minutes > MaxGoalMinutes {
			return false
		}
	}
	return true
}

const (
	// GoalSourceProgramme means the member uses the programme goals.
	GoalSourceProgramme = "programme"
	// GoalSourceStudent means a mentor or administrator set goals for this student.
	GoalSourceStudent = "student"
)

// PracticeSettings describes the time goals that apply to one member.
type PracticeSettings struct {
	GoalsEnabled bool `json:"goalsEnabled"`
	Goals
	Source string `json:"source"`
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
	Problem     *ProblemRecord `json:"problem,omitempty"`
	Outcome     string         `json:"outcome"`
	Confidence  *int           `json:"confidence,omitempty"`
	Minutes     int            `json:"minutes"`
	Notes       string         `json:"notes"`
	AttemptedAt time.Time      `json:"attemptedAt"`
	SeasonID    *string        `json:"seasonId,omitempty"`
	WeekID      *string        `json:"weekId,omitempty"`
	DeletedAt   *time.Time     `json:"-"`
}
