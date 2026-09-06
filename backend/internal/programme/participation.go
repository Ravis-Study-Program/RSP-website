package programme

import "time"

type ParticipationPeriod struct {
	Role      string     `json:"role"`
	StartedAt time.Time  `json:"startedAt"`
	EndedAt   *time.Time `json:"endedAt"`
}

type Participation struct {
	EnrollmentRecord
	SeasonName       string                `json:"seasonName"`
	StartAt          time.Time             `json:"startAt"`
	EndAt            time.Time             `json:"endAt"`
	LastStudentLevel string                `json:"lastStudentLevel"`
	Periods          []ParticipationPeriod `json:"periods"`
}
