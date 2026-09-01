package practice

import (
	"context"
	"time"
)

// Repository defines the persistence port required by practice features.
type Repository interface {
	GetPracticeSettings(context.Context, string) (PracticeSettings, error)
	UpdatePracticeSettings(context.Context, string, int64, int, int, int, string, time.Time) (PracticeSettings, error)
	EnablePracticeGoals(context.Context, string, int64, string, string, time.Time) (PracticeSettings, error)
	ListProblems(context.Context, string, int, string, string, *bool, string) ([]ProblemRecord, bool, int64, error)
	GetAttempt(context.Context, string) (AttemptRecord, error)
	ListAttempts(context.Context, string, string, int, string, string, string) ([]AttemptRecord, bool, int64, error)
	CreateAttempt(context.Context, AttemptRecord) (AttemptRecord, error)
	UpdateAttempt(context.Context, string, string, int64, func(*AttemptRecord) error) (AttemptRecord, error)
	DeleteAttempt(context.Context, string, string, int64) error
}
