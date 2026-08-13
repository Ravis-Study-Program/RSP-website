package model

import "time"

type User struct {
	ID           string   `json:"id"`
	Slug         string   `json:"slug"`
	Name         string   `json:"name"`
	AvatarURL    *string  `json:"avatarUrl,omitempty"`
	Timezone     string   `json:"timezone"`
	Email        string   `json:"email,omitempty"`
	AccountState string   `json:"accountState,omitempty"`
	GlobalRoles  []string `json:"globalRoles"`
	Revision     int64    `json:"revision"`
}
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
type Week struct {
	ID          string    `json:"id"`
	SeasonID    string    `json:"seasonId"`
	Number      int       `json:"number"`
	StartAt     time.Time `json:"startAt"`
	EndAt       time.Time `json:"endAt"`
	ResourceURL string    `json:"resourceUrl"`
	Revision    int64     `json:"revision"`
}
type Enrollment struct {
	ID            string  `json:"id"`
	SeasonID      string  `json:"seasonId"`
	UserID        string  `json:"userId"`
	Role          string  `json:"role"`
	State         string  `json:"state"`
	RemovalReason *string `json:"removalReason,omitempty"`
	Revision      int64   `json:"revision"`
}
type Mentorship struct {
	ID            string `json:"id"`
	SeasonID      string `json:"seasonId"`
	MentorUserID  string `json:"mentorUserId"`
	StudentUserID string `json:"studentUserId"`
	Revision      int64  `json:"revision"`
}
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
}
type AuditEvent struct {
	ID          string         `json:"id"`
	ActorID     *string        `json:"actorId,omitempty"`
	Action      string         `json:"action"`
	SubjectType string         `json:"subjectType"`
	SubjectID   string         `json:"subjectId"`
	Data        map[string]any `json:"data"`
	OccurredAt  time.Time      `json:"occurredAt"`
}

type PageInfo struct {
	NextCursor     *string `json:"nextCursor"`
	PreviousCursor *string `json:"previousCursor"`
	HasMore        bool    `json:"hasMore"`
}
type Page[T any] struct {
	Items      []T      `json:"items"`
	PageInfo   PageInfo `json:"pageInfo"`
	TotalCount int64    `json:"totalCount"`
}
