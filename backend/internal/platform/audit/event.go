// Package audit defines append-only audit records shared by feature adapters.
package audit

import "time"

// Event records an auditable application action.
type Event struct {
	ID          string
	ActorID     *string
	Action      string
	SubjectType string
	SubjectID   string
	Data        map[string]any
	OccurredAt  time.Time
}
