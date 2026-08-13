package accounts

import (
	"testing"
	"time"
)

type revoker struct{ called bool }

func (r *revoker) RevokeAll(string) error { r.called = true; return nil }
func TestDeletionGraceLifecycle(t *testing.T) {
	now := time.Now().UTC()
	a := Account{ID: "01901234-abcdef", Name: "Ada", Email: "a@example.com", Slug: "ada", State: Active, Revision: 1}
	r := &revoker{}
	if err := RequestDeletion(&a, "secret", now, r); err != nil || !r.called || a.State != DeletionPending {
		t.Fatal("request failed")
	}
	if err := CancelDeletion(&a, "wrong", now.Add(time.Hour)); err != ErrRecoveryToken {
		t.Fatalf("wrong token: %v", err)
	}
	if err := CancelDeletion(&a, "secret", now.Add(time.Hour)); err != nil || a.State != Active {
		t.Fatal("cancel failed")
	}
	_ = RequestDeletion(&a, "next", now, r)
	if Pseudonymize(&a, now.Add(29*24*time.Hour)) {
		t.Fatal("deleted early")
	}
	if !Pseudonymize(&a, now.Add(31*24*time.Hour)) || a.Email != "" || a.State != Deleted {
		t.Fatal("not pseudonymized")
	}
}
