package programme

import "time"

type Invitation struct {
	ID         string     `json:"id"`
	SeasonID   string     `json:"seasonId"`
	SeasonName string     `json:"seasonName"`
	SeasonSlug string     `json:"seasonSlug"`
	Name       string     `json:"name"`
	Email      string     `json:"email"`
	Role       string     `json:"role"`
	Status     string     `json:"status"`
	ExpiresAt  time.Time  `json:"expiresAt"`
	SentAt     *time.Time `json:"sentAt"`
	DeliveryID string     `json:"-"`
}
