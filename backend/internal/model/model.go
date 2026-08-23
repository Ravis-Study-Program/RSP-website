// Package model defines backend API and persistence models.
package model

import "time"

// PracticeSettings represents a backend data structure.
type PracticeSettings struct {
	PremiumOptIn  bool  `json:"premiumOptIn"`
	GoalsEnabled  bool  `json:"goalsEnabled"`
	EasyMinutes   int   `json:"easyMinutes"`
	MediumMinutes int   `json:"mediumMinutes"`
	HardMinutes   int   `json:"hardMinutes"`
	Revision      int64 `json:"revision"`
}

// Week represents a backend data structure.
type Week struct {
	ID          string    `json:"id"`
	SeasonID    string    `json:"seasonId"`
	Number      int       `json:"number"`
	StartAt     time.Time `json:"startAt"`
	EndAt       time.Time `json:"endAt"`
	ResourceURL string    `json:"resourceUrl"`
	Revision    int64     `json:"revision"`
}

// Enrollment represents a backend data structure.
type Enrollment struct {
	ID              string  `json:"id"`
	SeasonID        string  `json:"seasonId"`
	SeasonSlug      string  `json:"seasonSlug,omitempty"`
	UserID          string  `json:"userId"`
	Role            string  `json:"role"`
	StudentLevel    string  `json:"studentLevel,omitempty"`
	State           string  `json:"state"`
	AssignmentState string  `json:"assignmentState"`
	RemovalReason   *string `json:"removalReason,omitempty"`
	Revision        int64   `json:"revision"`
}

// Problem represents a backend data structure.
type Problem struct {
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

// Attempt represents a backend data structure.
type Attempt struct {
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
	// Migrated is internal recommendation provenance. Legacy attempts still
	// count as exposure, but never as outcome-quality evidence.
	Migrated bool `json:"-"`
}
