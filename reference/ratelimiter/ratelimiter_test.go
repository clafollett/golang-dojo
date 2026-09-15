package ratelimiter

import (
	"sync"
	"testing"
	"time"
)

// fakeClock lets tests advance time deterministically — no sleeping.
type fakeClock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *fakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = c.t.Add(d)
}

func TestAllow_BurstThenThrottle(t *testing.T) {
	clk := &fakeClock{t: time.Unix(0, 0)}
	l := New(10, 5, clk.Now) // 10/sec sustained, burst of 5

	// A fresh tenant may spend its full burst immediately.
	for i := range 5 {
		if !l.Allow("acme") {
			t.Fatalf("burst request %d should have been allowed", i+1)
		}
	}
	// Sixth request in the same instant is over burst → denied.
	if l.Allow("acme") {
		t.Fatal("request beyond burst should be denied")
	}
}

func TestAllow_RefillsOverTime(t *testing.T) {
	clk := &fakeClock{t: time.Unix(0, 0)}
	l := New(10, 5, clk.Now) // 10 tokens/sec

	for range 5 { // drain the burst
		l.Allow("acme")
	}
	if l.Allow("acme") {
		t.Fatal("should be empty right after draining burst")
	}

	clk.Advance(200 * time.Millisecond) // 0.2s * 10/sec = 2 tokens back
	if !l.Allow("acme") {
		t.Fatal("token 1 should have refilled")
	}
	if !l.Allow("acme") {
		t.Fatal("token 2 should have refilled")
	}
	if l.Allow("acme") {
		t.Fatal("only 2 tokens should have refilled")
	}
}

func TestAllow_TenantIsolation(t *testing.T) {
	clk := &fakeClock{t: time.Unix(0, 0)}
	l := New(1, 3, clk.Now)

	// Drain acme entirely.
	for range 3 {
		l.Allow("acme")
	}
	if l.Allow("acme") {
		t.Fatal("acme should be throttled")
	}
	// A different tenant is completely unaffected.
	for i := range 3 {
		if !l.Allow("globex") {
			t.Fatalf("globex request %d must not be affected by acme's usage", i+1)
		}
	}
}

func TestRefill_CapsAtBurst(t *testing.T) {
	clk := &fakeClock{t: time.Unix(0, 0)}
	l := New(10, 5, clk.Now)

	l.Allow("acme")            // spend one, then wait a long time
	clk.Advance(1 * time.Hour) // would accrue 36000 tokens, uncapped
	if got := l.Tokens("acme"); got != 5 {
		t.Fatalf("tokens = %v, want burst cap 5", got)
	}
}

// TestAllow_RaceSafe is the -race canary: hammer one tenant from many goroutines.
func TestAllow_RaceSafe(t *testing.T) {
	l := New(1000, 1000, nil) // real clock is fine here

	var wg sync.WaitGroup
	for range 50 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 100 {
				l.Allow("shared")
			}
		}()
	}
	wg.Wait()
}
