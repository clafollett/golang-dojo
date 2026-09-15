// Package ratelimiter is a runnable multi-tenant rate limiter — the core of any
// shared tool layer: rate limiting with per-caller attribution and isolation.
//
// It is a per-tenant token bucket. Each tenant gets its own bucket so a noisy
// caller cannot starve the others (tenant isolation). Tokens refill continuously
// at `rate` per second up to `burst`; a request costs one token.
//
// Design notes worth saying out loud when you defend the design:
//   - Token bucket (not fixed window) because it absorbs bursts up to `burst`
//     while holding the long-run average at `rate` — no thundering herd at the
//     top of each window, which a fixed window suffers from.
//   - The clock is injected (`now`) so behavior is testable without sleeping and
//     so you could swap in a monotonic or distributed clock later.
//   - This is in-process. Say the scaling boundary yourself: for a multi-replica
//     service you push this to a shared store (Redis token bucket / sliding log,
//     or an envoy/gateway limiter) so the limit is global, not per-pod. Naming
//     that boundary unprompted is the principal-level signal.
package ratelimiter

import (
	"math"
	"sync"
	"time"
)

// Limiter hands out tokens per tenant. The zero value is not usable; call New.
type Limiter struct {
	rate  float64 // tokens added per second
	burst float64 // max tokens a bucket can hold (the burst ceiling)
	now   func() time.Time

	mu      sync.Mutex // guards buckets and every bucket's fields
	buckets map[string]*bucket
}

type bucket struct {
	tokens float64
	last   time.Time
}

// New builds a Limiter allowing `rate` requests/sec sustained, with bursts up to
// `burst`. Pass nil for `now` to use time.Now.
func New(rate, burst float64, now func() time.Time) *Limiter {
	if now == nil {
		now = time.Now
	}
	return &Limiter{
		rate:    rate,
		burst:   burst,
		now:     now,
		buckets: make(map[string]*bucket),
	}
}

// Allow reports whether `tenant` may make one request now, consuming a token if
// so. The tenant string is the caller attribution: whatever identifies the
// billed principal (API key, org id, JWT sub).
func (l *Limiter) Allow(tenant string) bool {
	return l.AllowN(tenant, 1)
}

// AllowN consumes n tokens for tenant if available. n lets a single expensive
// tool call cost more than one token (weight by cost, not just count).
func (l *Limiter) AllowN(tenant string, n float64) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	b, ok := l.buckets[tenant]
	if !ok {
		// First request from a new tenant starts with a full burst allowance.
		b = &bucket{tokens: l.burst, last: now}
		l.buckets[tenant] = b
	}

	// Refill: add the tokens accrued since we last looked, capped at burst.
	elapsed := now.Sub(b.last).Seconds()
	if elapsed > 0 {
		b.tokens = math.Min(l.burst, b.tokens+elapsed*l.rate)
		b.last = now
	}

	if b.tokens >= n {
		b.tokens -= n
		return true
	}
	return false
}

// Tokens returns the current token count for a tenant (after refill). Useful for
// an X-RateLimit-Remaining header or a metric; also makes tests readable.
func (l *Limiter) Tokens(tenant string) float64 {
	l.mu.Lock()
	defer l.mu.Unlock()

	b, ok := l.buckets[tenant]
	if !ok {
		return l.burst
	}
	elapsed := l.now().Sub(b.last).Seconds()
	if elapsed > 0 {
		b.tokens = math.Min(l.burst, b.tokens+elapsed*l.rate)
		b.last = l.now()
	}
	return b.tokens
}
