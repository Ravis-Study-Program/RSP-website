// Package ratelimit provides in-process request rate limiting.
package ratelimit

import (
	"sync"
	"time"
)

type Class string

const (
	Read      Class = "read"
	Write     Class = "write"
	Sensitive Class = "sensitive"
)

type Result struct {
	Allowed          bool
	Limit, Remaining int
	RetryAfter       time.Duration
}

type bucket struct {
	started time.Time
	used    int
}

// Limits are the per-account requests allowed per minute for each class.
type Limits struct {
	Read, Write, Sensitive int
}

// DefaultLimits are the production limits.
func DefaultLimits() Limits {
	return Limits{Read: 120, Write: 20, Sensitive: 5}
}

type Limiter struct {
	mu      sync.Mutex
	window  time.Duration
	limits  map[Class]int
	buckets map[string]bucket
	now     func() time.Time
}

// New returns a limiter using DefaultLimits.
func New() *Limiter {
	return NewWithLimits(DefaultLimits())
}

// NewWithLimits returns a limiter with explicit per-minute limits. A zero
// field falls back to its default so partial configuration stays safe.
func NewWithLimits(configured Limits) *Limiter {
	defaults := DefaultLimits()
	pick := func(value, fallback int) int {
		if value > 0 {
			return value
		}
		return fallback
	}
	return &Limiter{
		window: time.Minute,
		limits: map[Class]int{
			Read:      pick(configured.Read, defaults.Read),
			Write:     pick(configured.Write, defaults.Write),
			Sensitive: pick(configured.Sensitive, defaults.Sensitive),
		},
		buckets: map[string]bucket{},
		now:     time.Now,
	}
}

func (l *Limiter) Allow(account string, class Class) Result {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now().UTC()
	key := account + "\x00" + string(class)
	b := l.buckets[key]
	if b.started.IsZero() || now.Sub(b.started) >= l.window {
		b = bucket{started: now}
	}
	limit := l.limits[class]
	if b.used >= limit {
		return Result{
			Allowed:    false,
			Limit:      limit,
			Remaining:  0,
			RetryAfter: b.started.Add(l.window).Sub(now),
		}
	}
	b.used++
	l.buckets[key] = b
	return Result{
		Allowed:   true,
		Limit:     limit,
		Remaining: limit - b.used,
	}
}
