package auth

import (
	"testing"
	"time"
)

func TestLimiterLocksAndExpires(t *testing.T) {
	limiter := NewLimiter(2, time.Minute)
	now := time.Now()
	limiter.Fail("user|ip", now)
	if limiter.Locked("user|ip", now) {
		t.Fatal("locked too early")
	}
	limiter.Fail("user|ip", now)
	if !limiter.Locked("user|ip", now) {
		t.Fatal("expected lock")
	}
	if limiter.Locked("user|ip", now.Add(2*time.Minute)) {
		t.Fatal("expected lock to expire")
	}
}
