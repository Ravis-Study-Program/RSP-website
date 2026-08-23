package accounts

import (
	"testing"
	"time"
)

func TestDeletionGraceLifecycle(t *testing.T) {
	now := time.Now().UTC()
	a := Account{ID: "01901234-abcdef", Name: "Ada", Email: "a@example.com", Slug: "ada", State: Active, Revision: 1}
	requested, err := RequestDeletion(a, "secret", now)
	if err != nil || len(requested.Effects) != 1 || requested.Account.State != DeletionPending || a.State != Active {
		t.Fatal("request failed")
	}
	if _, err := CancelDeletion(requested.Account, "wrong", now.Add(time.Hour)); err != ErrRecoveryToken {
		t.Fatalf("wrong token: %v", err)
	}
	a, err = CancelDeletion(requested.Account, "secret", now.Add(time.Hour))
	if err != nil || a.State != Active {
		t.Fatal("cancel failed")
	}

	requested, err = RequestDeletion(a, "next", now)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := Pseudonymize(requested.Account, now.Add(29*24*time.Hour)); ok {
		t.Fatal("deleted early")
	}
	a, ok := Pseudonymize(requested.Account, now.Add(31*24*time.Hour))
	if !ok || a.Email != "" || a.State != Deleted {
		t.Fatal("not pseudonymized")
	}
}
