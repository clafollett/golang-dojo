package ratelimiter

import (
	"sync"
	"testing"
	"time"
)

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
	l := New(10, 5, clk.Now)

	for i := range 5 {
		if !l.Allow("acme") {
			t.Fatalf("burst request %d should have been allowed", i+1)
		}
	}
	if l.Allow("acme") {
		t.Fatal("request beyond burst should be denied")
	}
}

func TestAllow_RefillsOverTime(t *testing.T) {
	clk := &fakeClock{t: time.Unix(0, 0)}
	l := New(10, 5, clk.Now)

	for range 5 {
		l.Allow("acme")
	}
	if l.Allow("acme") {
		t.Fatal("should be empty right after draining burst")
	}

	clk.Advance(200 * time.Millisecond) // 0.2s * 10/sec = 2 tokens
	if !l.Allow("acme") {
		t.Fatal("token 1 should have refilled")
	}
	if !l.Allow("acme") {
		t.Fatal("token 2 should have refilled")
	}
	if l.Allow("acme") {
		t.Fatal("only two tokens should have refilled")
	}
}

func TestAllow_TenantIsolation(t *testing.T) {
	clk := &fakeClock{t: time.Unix(0, 0)}
	l := New(1, 3, clk.Now)

	for range 3 {
		l.Allow("acme")
	}
	if l.Allow("acme") {
		t.Fatal("acme should be throttled")
	}
	for i := range 3 {
		if !l.Allow("globex") {
			t.Fatalf("globex request %d must not be affected by acme", i+1)
		}
	}
}

func TestAllow_RaceSafe(t *testing.T) {
	l := New(1000, 1000, nil)

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
