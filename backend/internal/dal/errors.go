package dal

import (
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

var (
	ErrNotFound               = errors.New("not found")
	ErrConflict               = errors.New("conflict")
	ErrDuplicate              = errors.New("duplicate")
	ErrMondayUpdateWeek       = errors.New("updates must match a configured program week")
	ErrMondayUpdateSubmitted  = errors.New("you have already submitted an update for this week")
	ErrMondayUpdateUnlockOpen = errors.New("this week already has an unlock request or an unlock")
	ErrInvalidMentor          = errors.New("choose a mentor who is mentoring in this season")
)

func noRows(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	return err
}

func mapDatabaseError(err error) error {
	if isUnique(err) {
		return ErrDuplicate
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && (pgErr.Code == "23503" || pgErr.Code == "23514" || pgErr.Code == "23502") {
		return ErrConflict
	}
	return noRows(err)
}

func isUnique(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
