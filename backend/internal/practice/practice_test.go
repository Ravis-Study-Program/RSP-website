package practice

import "testing"

func TestOutcomesReadAsSolvedOrUnsolved(t *testing.T) {
	for stored, public := range map[string]string{
		"independently_solved": Solved,
		"unknown":              Solved,
		"solved_with_hints":    Unsolved,
		"not_solved":           Unsolved,
	} {
		if got := PublicOutcome(stored); got != public {
			t.Errorf("PublicOutcome(%q) = %q, want %q", stored, got, public)
		}
	}
	for public, stored := range map[string]Outcome{Solved: Independent, Unsolved: NotSolved} {
		if got, ok := StoredOutcome(public); !ok || got != stored {
			t.Errorf("StoredOutcome(%q) = %q, %v; want %q", public, got, ok, stored)
		}
	}
	if _, ok := StoredOutcome("independently_solved"); ok {
		t.Error("StoredOutcome accepted a stored outcome name")
	}
}
