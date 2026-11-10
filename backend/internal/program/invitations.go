package program

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
	// MentorUserID is the mentor a student is invited to; accepting creates
	// the mentorship.
	MentorUserID *string `json:"mentorUserId"`
	// AcceptedBy is the account that accepted, linking it to its enrollment.
	AcceptedBy *string `json:"acceptedBy"`
	// ProfileSlug is the username the person gets on accepting, when their
	// account was created for this invitation and the name is still free.
	ProfileSlug *string `json:"profileSlug"`
	// MentorInvitationID is the mentor's own invitation, when they are invited
	// in the same import and have no account in the season yet.
	MentorInvitationID *string `json:"-"`
	DeliveryID         string  `json:"-"`
}
