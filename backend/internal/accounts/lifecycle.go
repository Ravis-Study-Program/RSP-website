package accounts

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"time"
)

var (
	ErrInvalidState  = errors.New("invalid account state")
	ErrRecoveryToken = errors.New("invalid recovery token")
)

type State string

const (
	Active          State = "active"
	Suspended       State = "suspended"
	DeletionPending State = "deletion_pending"
	Deleted         State = "deleted"
)

type Account struct {
	ID, Name, Email, Slug, RecoveryTokenHash string
	State                                    State
	DeletionRequestedAt                      *time.Time
	DeleteAfter                              *time.Time
	Revision                                 int64
}

type SessionRevoker interface{ RevokeAll(userID string) error }

func RequestDeletion(a *Account, rawToken string, now time.Time, r SessionRevoker) error {
	if a.State != Active {
		return ErrInvalidState
	}
	if err := r.RevokeAll(a.ID); err != nil {
		return err
	}

	requested := now.UTC()
	after := requested.Add(30 * 24 * time.Hour)
	sum := sha256.Sum256([]byte(rawToken))
	a.RecoveryTokenHash = hex.EncodeToString(sum[:])
	a.DeletionRequestedAt = &requested
	a.DeleteAfter = &after
	a.State = DeletionPending
	a.Revision++
	return nil
}

func CancelDeletion(a *Account, rawToken string, now time.Time) error {
	if a.State != DeletionPending || a.DeleteAfter == nil || !now.UTC().Before(*a.DeleteAfter) {
		return ErrInvalidState
	}
	sum := sha256.Sum256([]byte(rawToken))
	if a.RecoveryTokenHash != hex.EncodeToString(sum[:]) {
		return ErrRecoveryToken
	}
	a.State = Active
	a.RecoveryTokenHash = ""
	a.DeletionRequestedAt = nil
	a.DeleteAfter = nil
	a.Revision++
	return nil
}

func Pseudonymize(a *Account, now time.Time) bool {
	if a.State != DeletionPending || a.DeleteAfter == nil || now.UTC().Before(*a.DeleteAfter) {
		return false
	}
	suffix := a.ID
	if len(suffix) > 8 {
		suffix = suffix[len(suffix)-8:]
	}
	a.Name = "Deleted member"
	a.Email = ""
	a.Slug = "deleted-" + suffix
	a.RecoveryTokenHash = ""
	a.State = Deleted
	a.Revision++
	return true
}
