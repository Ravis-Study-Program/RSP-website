package accounts

// User is the account read model exposed by account-related adapters.
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

// UserSeasonRole describes a user's role in a season.
type UserSeasonRole struct {
	SeasonID   string `json:"seasonId"`
	SeasonSlug string `json:"seasonSlug"`
	Role       string `json:"role"`
	State      string `json:"state"`
}

// EnrollmentCandidate is a user eligible for season enrollment.
type EnrollmentCandidate struct {
	ID        string  `json:"id"`
	Slug      string  `json:"slug"`
	Name      string  `json:"name"`
	AvatarURL *string `json:"avatarUrl,omitempty"`
	Revision  int64   `json:"revision"`
}
