// Package model defines backend API and persistence models.
package model

import "time"

// User represents a backend data structure.
type User struct {
	ID                 string           `json:"id"`
	Slug               string           `json:"slug"`
	Name               string           `json:"name"`
	AvatarURL          *string          `json:"avatarUrl,omitempty"`
	Timezone           string           `json:"timezone"`
	TimezoneConfigured bool             `json:"timezoneConfigured"`
	Email              string           `json:"email,omitempty"`
	AccountState       string           `json:"accountState,omitempty"`
	GlobalRoles        []string         `json:"globalRoles"`
	SeasonRoles        []UserSeasonRole `json:"seasonRoles"`
	AttemptCount       int64            `json:"attemptCount"`
	MockInterviewCount int64            `json:"mockInterviewCount"`
	IsTest             bool             `json:"-"`
	PremiumOptIn       bool             `json:"-"`
	Revision           int64            `json:"revision"`
}

// UserSeasonRole represents a backend data structure.
type UserSeasonRole struct {
	SeasonID   string `json:"seasonId"`
	SeasonSlug string `json:"seasonSlug"`
	Role       string `json:"role"`
	State      string `json:"state"`
}

// EnrollmentCandidate represents a backend data structure.
type EnrollmentCandidate struct {
	ID        string  `json:"id"`
	Slug      string  `json:"slug"`
	Name      string  `json:"name"`
	AvatarURL *string `json:"avatarUrl,omitempty"`
	Revision  int64   `json:"revision"`
}

// PracticeSettings represents a backend data structure.
type PracticeSettings struct {
	PremiumOptIn  bool  `json:"premiumOptIn"`
	GoalsEnabled  bool  `json:"goalsEnabled"`
	EasyMinutes   int   `json:"easyMinutes"`
	MediumMinutes int   `json:"mediumMinutes"`
	HardMinutes   int   `json:"hardMinutes"`
	Revision      int64 `json:"revision"`
}

// Season represents a backend data structure.
type Season struct {
	ID           string    `json:"id"`
	Slug         string    `json:"slug"`
	Name         string    `json:"name"`
	Status       string    `json:"status"`
	StartAt      time.Time `json:"startAt"`
	EndAt        time.Time `json:"endAt"`
	Location     string    `json:"location"`
	ImageURL     string    `json:"imageUrl"`
	ResourcesURL string    `json:"resourcesUrl"`
	Revision     int64     `json:"revision"`
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

// Mentorship represents a backend data structure.
type Mentorship struct {
	ID            string `json:"id"`
	SeasonID      string `json:"seasonId"`
	MentorUserID  string `json:"mentorUserId"`
	StudentUserID string `json:"studentUserId"`
	Revision      int64  `json:"revision"`
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

// AuditEvent represents a backend data structure.
type AuditEvent struct {
	ID          string         `json:"id"`
	ActorID     *string        `json:"actorId,omitempty"`
	Action      string         `json:"action"`
	SubjectType string         `json:"subjectType"`
	SubjectID   string         `json:"subjectId"`
	Data        map[string]any `json:"data"`
	OccurredAt  time.Time      `json:"occurredAt"`
}

// PageInfo represents a backend data structure.
type PageInfo struct {
	NextCursor     *string `json:"nextCursor"`
	PreviousCursor *string `json:"previousCursor"`
	HasMore        bool    `json:"hasMore"`
}

// Page represents a backend data structure.
type Page[T any] struct {
	Items      []T      `json:"items"`
	PageInfo   PageInfo `json:"pageInfo"`
	TotalCount int64    `json:"totalCount"`
}
