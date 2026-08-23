package mockinterviews

import (
	"context"
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

// Repository contains only the persistence operations needed by interview
// mutations. The application layer does not depend on the concrete database.
type Repository interface {
	CreateMockInterview(context.Context, Interview, string, time.Time) (Interview, error)
	UpdateMockInterview(context.Context, Interview, string, string, time.Time) (Interview, error)
}

// Application coordinates interview transitions and persistence.
type Application struct {
	Repository Repository
	Rules      Service
	NewID      func() string
}

// Create applies the creation rules and persists the result.
func (a Application) Create(ctx context.Context, actorID string, in CreateInput, now time.Time) (Interview, error) {
	interview, err := a.Rules.Create(actorID, in, now)
	if err != nil {
		return Interview{}, err
	}
	if a.NewID != nil {
		interview.ID = a.NewID()
	}
	return a.Repository.CreateMockInterview(ctx, interview, actorID, now)
}

// Update applies an update transition and persists its version.
func (a Application) Update(ctx context.Context, current Interview, actorID string, in UpdateInput, now time.Time) (Interview, error) {
	updated, err := a.Rules.Update(current, actorID, in, now)
	if err != nil {
		return Interview{}, err
	}
	return a.Repository.UpdateMockInterview(ctx, updated, actorID, "updated", now)
}

// Delete applies a soft-delete transition and persists its version.
func (a Application) Delete(ctx context.Context, current Interview, actorID string, revision int64, now time.Time) (Interview, error) {
	updated, err := a.Rules.Delete(current, actorID, revision, now)
	if err != nil {
		return Interview{}, err
	}
	return a.Repository.UpdateMockInterview(ctx, updated, actorID, "soft deleted", now)
}

// Review applies an interviewee review and persists its version.
func (a Application) Review(ctx context.Context, current Interview, actorID, roundID, comment string, reviewed bool, revision int64, now time.Time) (Interview, error) {
	updated, err := a.Rules.Review(current, actorID, roundID, comment, reviewed, revision, now)
	if err != nil {
		return Interview{}, err
	}
	return a.Repository.UpdateMockInterview(ctx, updated, actorID, "interviewee review", now)
}

// CorrectIdentities applies an identity correction and persists its version.
func (a Application) CorrectIdentities(ctx context.Context, current Interview, actorID, interviewerID, intervieweeID string, seasonID *string, reason string, privileged bool, revision int64, now time.Time) (Interview, error) {
	updated, err := a.Rules.CorrectIdentities(current, actorID, interviewerID, intervieweeID, seasonID, reason, privileged, revision, now)
	if err != nil {
		return Interview{}, err
	}
	return a.Repository.UpdateMockInterview(ctx, updated, actorID, "identity correction: "+reason, now)
}
