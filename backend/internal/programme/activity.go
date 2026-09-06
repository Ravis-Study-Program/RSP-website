package programme

import "time"

// ActivitySummary counts records, not participants, in one selected scope.
type ActivitySummary struct {
	AttemptCount       int64      `json:"attemptCount"`
	MockInterviewCount int64      `json:"mockInterviewCount"`
	MocksReceived      int64      `json:"mocksReceived"`
	MocksConducted     int64      `json:"mocksConducted"`
	LastActivityAt     *time.Time `json:"lastActivityAt"`
}

type SeasonMemberSummary struct {
	ID        string          `json:"id"`
	Slug      string          `json:"slug"`
	Name      string          `json:"name"`
	AvatarURL *string         `json:"avatarUrl"`
	Activity  ActivitySummary `json:"activity"`
}
