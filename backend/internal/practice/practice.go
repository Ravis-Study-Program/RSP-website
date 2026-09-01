package practice

import "time"

// Outcome is the result of a problem attempt.
type Outcome string

const (
	Independent Outcome = "independently_solved"
	WithHints   Outcome = "solved_with_hints"
	NotSolved   Outcome = "not_solved"
)

// PracticeSettings stores a user's personal time goals.
type PracticeSettings struct {
	GoalsEnabled  bool  `json:"goalsEnabled"`
	EasyMinutes   int   `json:"easyMinutes"`
	MediumMinutes int   `json:"mediumMinutes"`
	HardMinutes   int   `json:"hardMinutes"`
	Revision      int64 `json:"revision"`
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
	Revision   int64    `json:"revision"`
}

// AttemptRecord is the persisted and HTTP-facing attempt representation.
type AttemptRecord struct {
	ID          string     `json:"id"`
	UserID      string     `json:"-"`
	ProblemID   string     `json:"problemId"`
	Outcome     string     `json:"outcome"`
	Confidence  *int       `json:"confidence,omitempty"`
	Minutes     int        `json:"minutes"`
	Notes       string     `json:"notes"`
	AttemptedAt time.Time  `json:"attemptedAt"`
	SeasonID    *string    `json:"seasonId,omitempty"`
	WeekID      *string    `json:"weekId,omitempty"`
	Revision    int64      `json:"revision"`
	DeletedAt   *time.Time `json:"-"`
}
