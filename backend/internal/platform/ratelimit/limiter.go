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

type Limiter struct {
	mu      sync.Mutex
	window  time.Duration
	limits  map[Class]int
	buckets map[string]bucket
	now     func() time.Time
}

func New() *Limiter {
	return &Limiter{
		window:  time.Minute,
		limits:  map[Class]int{Read: 120, Write: 20, Sensitive: 5},
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
