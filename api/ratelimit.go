package api

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// rateLimiter is a token bucket per key.
//
// Written rather than pulled in from golang.org/x/time/rate: the per-key map
// and its eviction are the bulk of this either way, and a direct dependency for
// the remaining few lines of bucket arithmetic is out of proportion for a
// project that otherwise avoids them.
//
// Buckets for idle keys are swept lazily, on the same lock as allow, so the map
// cannot grow without bound from an attacker cycling keys and there is no
// background goroutine to own and shut down.
type rateLimiter struct {
	mu        sync.Mutex
	buckets   map[string]*tokenBucket
	perSecond float64
	burst     float64
	idleTTL   time.Duration
	lastSweep time.Time

	// now is injectable so tests can advance time instead of sleeping.
	now func() time.Time
}

type tokenBucket struct {
	tokens   float64
	lastFill time.Time
}

// newRateLimiter allows burst requests immediately, then one more every
// 1/perSecond. Keys untouched for idleTTL are forgotten, which also restores a
// key's full burst.
func newRateLimiter(perSecond, burst float64, idleTTL time.Duration) *rateLimiter {
	return &rateLimiter{
		buckets:   make(map[string]*tokenBucket),
		perSecond: perSecond,
		burst:     burst,
		idleTTL:   idleTTL,
		now:       time.Now,
	}
}

// allow consumes a token for key, reporting whether one was available.
func (l *rateLimiter) allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	l.sweepLocked(now)

	b, ok := l.buckets[key]
	if !ok {
		// Start full and fall through to the same consume path rather than
		// admitting unconditionally, so an unseen key is still subject to the
		// burst — with burst 0 that means denied.
		b = &tokenBucket{tokens: l.burst, lastFill: now}
		l.buckets[key] = b
	}

	b.tokens += now.Sub(b.lastFill).Seconds() * l.perSecond
	if b.tokens > l.burst {
		b.tokens = l.burst
	}
	b.lastFill = now

	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

// sweepLocked drops buckets that have sat idle long enough to have refilled
// completely, since those are indistinguishable from absent ones. Runs at most
// once per idleTTL. Caller holds the lock.
func (l *rateLimiter) sweepLocked(now time.Time) {
	if now.Sub(l.lastSweep) < l.idleTTL {
		return
	}
	l.lastSweep = now
	for k, b := range l.buckets {
		if now.Sub(b.lastFill) >= l.idleTTL {
			delete(l.buckets, k)
		}
	}
}

// clientIP returns the address to rate-limit a request by.
//
// Deliberately reads RemoteAddr and not X-Forwarded-For: that header is
// attacker-controlled unless a trusted proxy is known to overwrite it, and
// honouring it blindly would let one client mint unlimited identities and
// defeat the limit entirely. Behind a reverse proxy this collapses every client
// onto the proxy's address, so the per-email limit is what still protects an
// individual mailbox there.
func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// limitKeyForEmail normalises an address so case variants share one bucket.
func limitKeyForEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}
