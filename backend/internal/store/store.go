package store

import (
	"context"
	"errors"

	"github.com/magedmg/RSP-website/backend/internal/authz"
	"github.com/magedmg/RSP-website/backend/internal/model"
)

var (
	ErrNotFound  = errors.New("not found")
	ErrConflict  = errors.New("conflict")
	ErrDuplicate = errors.New("duplicate")
)

type Repository interface {
	ResolveAuthSubject(context.Context, string) (authz.Actor, error)
	GetUser(context.Context, string) (model.User, error)
	ListUsers(context.Context, string, int) ([]model.User, bool, int64, error)
	UpdateUser(context.Context, string, int64, func(*model.User) error) (model.User, error)
	GetSeason(context.Context, string) (model.Season, error)
	ListSeasons(context.Context, string, int) ([]model.Season, bool, int64, error)
	CreateSeason(context.Context, model.Season) (model.Season, error)
	UpdateSeason(context.Context, string, int64, func(*model.Season) error) (model.Season, error)
	ListProblems(context.Context, string, int) ([]model.Problem, bool, int64, error)
	GetAttempt(context.Context, string) (model.Attempt, error)
	ListAttempts(context.Context, string, string, int) ([]model.Attempt, bool, int64, error)
	CreateAttempt(context.Context, model.Attempt) (model.Attempt, error)
	UpdateAttempt(context.Context, string, string, int64, func(*model.Attempt) error) (model.Attempt, error)
	DeleteAttempt(context.Context, string, string, int64) error
	AppendAudit(context.Context, model.AuditEvent) error
}
