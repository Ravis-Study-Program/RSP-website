// Package accounts models account lifecycle operations.
package accounts

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"time"
)

var (
	// ErrInvalidState is a public value used by the backend.
	ErrInvalidState = errors.New("invalid account state")
	// ErrRecoveryToken is a public value used by the backend.
	ErrRecoveryToken = errors.New("invalid recovery token")
)

// State is a backend domain type.
type State string

const (
	// Active is a public value used by the backend.
	Active State = "active"
	// Suspended is a public value used by the backend.
	Suspended State = "suspended"
	// DeletionPending is a public value used by the backend.
	DeletionPending State = "deletion_pending"
	// Deleted is a public value used by the backend.
	Deleted State = "deleted"
)

// Account represents a backend data structure.
type Account struct {
	ID, Name, Email, Slug, RecoveryTokenHash string
	State                                    State
	DeletionRequestedAt                      *time.Time
	DeleteAfter                              *time.Time
	Revision                                 int64
}

// EffectKind identifies an imperative action required after a transition.
type EffectKind string

const (
	// RevokeSessions requests invalidation of the account's sessions.
	RevokeSessions EffectKind = "revoke_sessions"
)

// Effect describes work that belongs to the application shell.
type Effect struct {
	Kind   EffectKind
	UserID string
}

// Transition contains the new account value and shell effects.
type Transition struct {
	Account Account
	Effects []Effect
}

// RequestDeletion requests account deletion without changing the input.
func RequestDeletion(a Account, rawToken string, now time.Time) (Transition, error) {
	if a.State != Active {
		return Transition{}, ErrInvalidState
	}

	requested := now.UTC()
	after := requested.Add(30 * 24 * time.Hour)
	sum := sha256.Sum256([]byte(rawToken))
	updated := a
	updated.RecoveryTokenHash = hex.EncodeToString(sum[:])
	updated.DeletionRequestedAt = &requested
	updated.DeleteAfter = &after
	updated.State = DeletionPending
	updated.Revision++
	return Transition{Account: updated, Effects: []Effect{{Kind: RevokeSessions, UserID: a.ID}}}, nil
}

// CancelDeletion cancels deletion without changing the input.
func CancelDeletion(a Account, rawToken string, now time.Time) (Account, error) {
	if a.State != DeletionPending || a.DeleteAfter == nil || !now.UTC().Before(*a.DeleteAfter) {
		return Account{}, ErrInvalidState
	}
	sum := sha256.Sum256([]byte(rawToken))
	if a.RecoveryTokenHash != hex.EncodeToString(sum[:]) {
		return Account{}, ErrRecoveryToken
	}
	updated := a
	updated.State = Active
	updated.RecoveryTokenHash = ""
	updated.DeletionRequestedAt = nil
	updated.DeleteAfter = nil
	updated.Revision++
	return updated, nil
}

// Pseudonymize pseudonymizes an expired account without changing the input.
func Pseudonymize(a Account, now time.Time) (Account, bool) {
	if a.State != DeletionPending || a.DeleteAfter == nil || now.UTC().Before(*a.DeleteAfter) {
		return Account{}, false
	}
	suffix := a.ID
	if len(suffix) > 8 {
		suffix = suffix[len(suffix)-8:]
	}
	updated := a
	updated.Name = "Deleted member"
	updated.Email = ""
	updated.Slug = "deleted-" + suffix
	updated.RecoveryTokenHash = ""
	updated.State = Deleted
	updated.Revision++
	return updated, true
}
