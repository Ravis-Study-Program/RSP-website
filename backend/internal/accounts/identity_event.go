package accounts

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"time"
)

// IdentityEvent is the durable event contract between the identity service
// and the account lifecycle.
type IdentityEvent struct {
	EventID, Type, AuthUserID, Email, Reason, AccountState, ActorUserID string
	EmailVerified                                                       bool
	SecurityVersion                                                     int64
	OccurredAt                                                          time.Time
	RecoveryDeadline                                                    *time.Time
}

// IdentityEventHash returns the stable replay-detection hash for an event.
func IdentityEventHash(event IdentityEvent) string {
	recoveryDeadline := ""
	if event.RecoveryDeadline != nil {
		recoveryDeadline = event.RecoveryDeadline.UTC().Format(time.RFC3339Nano)
	}
	payload, _ := json.Marshal(struct {
		Type, AuthUserID, Email, Reason, AccountState, ActorUserID, OccurredAt, RecoveryDeadline string
		EmailVerified                                                                            bool
		SecurityVersion                                                                          int64
	}{event.Type, event.AuthUserID, event.Email, event.Reason, event.AccountState, event.ActorUserID, event.OccurredAt.UTC().Format(time.RFC3339Nano), recoveryDeadline, event.EmailVerified, event.SecurityVersion})
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}
