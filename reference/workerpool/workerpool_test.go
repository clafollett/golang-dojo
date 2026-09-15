package workerpool

import (
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"
)

func TestRun_CollectsEveryResult(t *testing.T) {
	ids := []string{"a", "b", "c", "d", "e"}
	fetch := func(id string) (string, error) { return "v-" + id, nil }

	got := Run(ids, 3, fetch)

	if len(got) != len(ids) {
		t.Fatalf("got %d results, want %d", len(got), len(ids))
	}
	// Order is nondeterministic, so compare as a set.
	seen := make(map[string]string, len(got))
	for _, r := range got {
		if r.Err != nil {
			t.Errorf("unexpected error for %s: %v", r.ID, r.Err)
		}
		seen[r.ID] = r.Value
	}
	for _, id := range ids {
		if want := "v-" + id; seen[id] != want {
			t.Errorf("id %s: got %q, want %q", id, seen[id], want)
		}
	}
}

func TestRun_PropagatesErrors(t *testing.T) {
	ids := []string{"ok", "boom"}
	fetch := func(id string) (string, error) {
		if id == "boom" {
			return "", errors.New("kaboom")
		}
		return "v-" + id, nil
	}

	got := Run(ids, 2, fetch)

	var boom Result
	for _, r := range got {
		if r.ID == "boom" {
			boom = r
		}
	}
	if boom.Err == nil {
		t.Fatalf("expected error for id=boom, got nil")
	}
}

// TestRun_RespectsBound proves `workers` actually caps concurrency: the observed
// peak of simultaneously-running fetches never exceeds the bound.
func TestRun_RespectsBound(t *testing.T) {
	const bound = 4
	ids := make([]string, 50)
	for i := range ids {
		ids[i] = fmt.Sprintf("id-%d", i)
	}

	var inFlight, peak atomic.Int64
	fetch := func(id string) (string, error) {
		n := inFlight.Add(1)
		for { // lock-free max
			p := peak.Load()
			if n <= p || peak.CompareAndSwap(p, n) {
				break
			}
		}
		time.Sleep(2 * time.Millisecond)
		inFlight.Add(-1)
		return id, nil
	}

	Run(ids, bound, fetch)

	if got := peak.Load(); got > bound {
		t.Fatalf("peak concurrency %d exceeded bound %d", got, bound)
	}
}
