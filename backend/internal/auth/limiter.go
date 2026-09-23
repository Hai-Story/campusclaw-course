package auth

import (
	"sync"
	"time"
)

type attempt struct {
	Failures    int
	LockedUntil time.Time
}

type Limiter struct {
	mu       sync.Mutex
	attempts map[string]attempt
	max      int
	lockFor  time.Duration
}

func NewLimiter(max int, lockFor time.Duration) *Limiter {
	return &Limiter{attempts: make(map[string]attempt), max: max, lockFor: lockFor}
}

func (l *Limiter) Locked(key string, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	state, ok := l.attempts[key]
	if !ok {
		return false
	}
	if !state.LockedUntil.IsZero() && now.Before(state.LockedUntil) {
		return true
	}
	if !state.LockedUntil.IsZero() {
		delete(l.attempts, key)
	}
	return false
}

func (l *Limiter) Fail(key string, now time.Time) {
	l.mu.Lock()
	defer l.mu.Unlock()
	state := l.attempts[key]
	state.Failures++
	if state.Failures >= l.max {
		state.LockedUntil = now.Add(l.lockFor)
	}
	l.attempts[key] = state
}

func (l *Limiter) Reset(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.attempts, key)
}
