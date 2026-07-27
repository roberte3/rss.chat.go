package api

import (
	"net/http/httptest"
	"testing"
	"time"
)

// newTestLimiter returns a limiter with a clock the test drives, so nothing
// here depends on wall-clock sleeps.
func newTestLimiter(perSecond, burst float64, idleTTL time.Duration) (*rateLimiter, func(time.Duration)) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	l := newRateLimiter(perSecond, burst, idleTTL)
	l.now = func() time.Time { return now }
	return l, func(d time.Duration) { now = now.Add(d) }
}

func TestRateLimiterAllowsBurstThenBlocks(t *testing.T) {
	l, _ := newTestLimiter(1.0/300.0, 3, time.Hour)

	for i := 1; i <= 3; i++ {
		if !l.allow("a@example.com") {
			t.Fatalf("request %d denied, want the first 3 allowed", i)
		}
	}
	if l.allow("a@example.com") {
		t.Error("4th request allowed, want denied once the burst is spent")
	}
}

func TestRateLimiterRefillsOverTime(t *testing.T) {
	l, advance := newTestLimiter(1.0/300.0, 3, time.Hour) // one per 5 minutes
	for i := 0; i < 3; i++ {
		l.allow("a@example.com")
	}

	advance(4 * time.Minute)
	if l.allow("a@example.com") {
		t.Error("allowed after 4 minutes, want a full 5 to earn a token")
	}

	advance(1 * time.Minute)
	if !l.allow("a@example.com") {
		t.Error("denied after 5 minutes, want one token refilled")
	}
}

func TestRateLimiterDoesNotRefillBeyondBurst(t *testing.T) {
	l, advance := newTestLimiter(1.0/300.0, 3, 24*time.Hour)
	l.allow("a@example.com")

	// Idle far longer than it takes to refill, without hitting the sweep.
	advance(12 * time.Hour)

	for i := 1; i <= 3; i++ {
		if !l.allow("a@example.com") {
			t.Fatalf("request %d denied, want burst restored", i)
		}
	}
	if l.allow("a@example.com") {
		t.Error("4th allowed; tokens accumulated past the burst ceiling")
	}
}

func TestRateLimiterKeysAreIndependent(t *testing.T) {
	l, _ := newTestLimiter(1.0/300.0, 1, time.Hour)

	if !l.allow("a@example.com") {
		t.Fatal("first key denied")
	}
	if l.allow("a@example.com") {
		t.Fatal("first key not limited")
	}
	if !l.allow("b@example.com") {
		t.Error("second key denied; one key's limit leaked onto another")
	}
}

// TestRateLimiterEvictsIdleKeys is the memory-safety property: without it, a
// sender cycling keys grows the map without bound, which is its own denial of
// service.
func TestRateLimiterEvictsIdleKeys(t *testing.T) {
	l, advance := newTestLimiter(1.0/300.0, 1, time.Hour)

	for _, k := range []string{"a", "b", "c"} {
		l.allow(k)
	}
	if got := len(l.buckets); got != 3 {
		t.Fatalf("bucket count = %d, want 3", got)
	}

	// Past the TTL, the next call sweeps the idle keys.
	advance(2 * time.Hour)
	l.allow("d")

	if _, stale := l.buckets["a"]; stale {
		t.Error("idle key retained after its TTL")
	}
	if got := len(l.buckets); got != 1 {
		t.Errorf("bucket count = %d after sweep, want only the live key", got)
	}
}

func TestLimitKeyForEmailNormalises(t *testing.T) {
	// Case and padding must share a bucket, or the limit is bypassed by
	// retyping the same address differently.
	a := limitKeyForEmail("  Alice@Example.COM ")
	b := limitKeyForEmail("alice@example.com")
	if a != b {
		t.Errorf("keys differ: %q vs %q", a, b)
	}
}

func TestClientIPStripsPort(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "203.0.113.9:54321"
	if got := clientIP(r); got != "203.0.113.9" {
		t.Errorf("clientIP = %q, want 203.0.113.9", got)
	}
}

// TestClientIPIgnoresForwardedFor pins the deliberate choice: honouring the
// header would let a caller mint a fresh identity per request and bypass the
// per-IP limit entirely.
func TestClientIPIgnoresForwardedFor(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "203.0.113.9:54321"
	r.Header.Set("X-Forwarded-For", "198.51.100.7")
	if got := clientIP(r); got != "203.0.113.9" {
		t.Errorf("clientIP = %q; X-Forwarded-For must not override RemoteAddr", got)
	}
}
