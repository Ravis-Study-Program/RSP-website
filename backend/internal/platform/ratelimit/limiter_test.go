package ratelimit

import (
	"testing"
	"time"
)

func TestClassesAndReset(t *testing.T) {
	l := New()
	now := time.Now()
	l.now = func() time.Time { return now }
	for i := 0; i < 5; i++ {
		if !l.Allow("u", Sensitive).Allowed {
			t.Fatal("limited early")
		}
	}
	got := l.Allow("u", Sensitive)
	if got.Allowed || got.RetryAfter <= 0 {
		t.Fatal("missing limit")
	}
	if !l.Allow("other", Sensitive).Allowed || !l.Allow("u", Read).Allowed {
		t.Fatal("buckets not isolated")
	}
	now = now.Add(time.Minute)
	if !l.Allow("u", Sensitive).Allowed {
		t.Fatal("did not reset")
	}
}

func TestConfiguredLimitsFallBackToDefaults(t *testing.T) {
	l := NewWithLimits(Limits{Read: 3})
	for i := 0; i < 3; i++ {
		if !l.Allow("u", Read).Allowed {
			t.Fatal("configured read limit applied too early")
		}
	}
	if l.Allow("u", Read).Allowed {
		t.Fatal("configured read limit was not enforced")
	}
	if got := l.Allow("u", Sensitive); got.Limit != DefaultLimits().Sensitive {
		t.Fatalf("unset sensitive limit = %d, want default", got.Limit)
	}
}
