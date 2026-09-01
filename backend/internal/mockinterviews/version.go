package mockinterviews

import (
	"encoding/json"
	"time"
)

// Version records an immutable interview snapshot.
type Version struct {
	InterviewID     string
	Revision        int64
	ActorID, Reason string
	SavedAt         time.Time
	Snapshot        json.RawMessage
}
