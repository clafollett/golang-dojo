// Package ratelimiter is a TYPE-IT-COLD exercise — a multi-tenant token bucket.
//
// Goal: implement a per-tenant token bucket until
// `go test ./exercises/ratelimiter/` is green. Reference lives in
// reference/ratelimiter.
//
// The shape to hold in your head:
//   - map[tenant]*bucket, guarded by one mutex
//   - each bucket: tokens float64, last time.Time
//   - on Allow: refill by elapsed*rate (cap at burst), then spend 1 if available
//   - inject the clock so tests do not sleep
package ratelimiter

import (
	"sync"
	"time"
)

// Limiter hands out tokens per tenant. Build it with New.
type Limiter struct {
	// TODO: fields — rate, burst float64; now func() time.Time;
	//       mu sync.Mutex; buckets map[string]*bucket
	_ struct{} // placeholder so the struct compiles before you fill it in
}

// New builds a Limiter allowing `rate` req/sec sustained with bursts up to `burst`.
// Pass nil for `now` to default to time.Now.
func New(rate, burst float64, now func() time.Time) *Limiter {
	_ = sync.Mutex{} // keep imports live until you implement; delete when done
	_ = time.Now
	panic("not implemented — build the token bucket from memory")
}

// Allow reports whether tenant may make one request now, consuming a token if so.
func (l *Limiter) Allow(tenant string) bool {
	panic("not implemented")
}
