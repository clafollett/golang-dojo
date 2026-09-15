package fanout

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"
)

// fakeClient is a deterministic stand-in for a real model endpoint. It records
// peak concurrency so a test can assert the bound is actually enforced.
type fakeClient struct {
	delay    time.Duration
	failOn   map[string]bool
	inFlight atomic.Int64
	peak     atomic.Int64
}

func (c *fakeClient) Complete(ctx context.Context, req Request) (string, error) {
	n := c.inFlight.Add(1)
	for {
		p := c.peak.Load()
		if n <= p || c.peak.CompareAndSwap(p, n) {
			break
		}
	}
	defer c.inFlight.Add(-1)

	if c.failOn[req.ID] {
		return "", fmt.Errorf("model refused %s", req.ID)
	}
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case <-time.After(c.delay):
		return "echo:" + req.Prompt, nil
	}
}

func makeReqs(n int) []Request {
	reqs := make([]Request, n)
	for i := range reqs {
		reqs[i] = Request{ID: fmt.Sprintf("r%d", i), Prompt: fmt.Sprintf("p%d", i)}
	}
	return reqs
}

func TestFanOut_AllSucceedInInputOrder(t *testing.T) {
	c := &fakeClient{delay: time.Millisecond}
	reqs := makeReqs(6)

	got := FanOut(context.Background(), c, reqs, 3)

	if len(got) != len(reqs) {
		t.Fatalf("got %d responses, want %d", len(got), len(reqs))
	}
	for i, r := range got {
		if r.ID != reqs[i].ID {
			t.Errorf("position %d: id %q, want %q (order not preserved)", i, r.ID, reqs[i].ID)
		}
		if r.Err != nil {
			t.Errorf("%s: unexpected error %v", r.ID, r.Err)
		}
		if r.Output != "echo:"+reqs[i].Prompt {
			t.Errorf("%s: output %q", r.ID, r.Output)
		}
	}
}

func TestFanOut_EnforcesBound(t *testing.T) {
	const bound = 4
	c := &fakeClient{delay: 3 * time.Millisecond}

	FanOut(context.Background(), c, makeReqs(40), bound)

	if peak := c.peak.Load(); peak > bound {
		t.Fatalf("peak concurrency %d exceeded bound %d", peak, bound)
	}
}

func TestFanOut_IsolatesPerItemErrors(t *testing.T) {
	c := &fakeClient{delay: time.Millisecond, failOn: map[string]bool{"r2": true}}
	reqs := makeReqs(5)

	got := FanOut(context.Background(), c, reqs, 5)

	for _, r := range got {
		if r.ID == "r2" {
			if r.Err == nil {
				t.Errorf("expected r2 to carry an error")
			}
		} else if r.Err != nil {
			t.Errorf("%s should have succeeded, got %v", r.ID, r.Err)
		}
	}
}

func TestFanOut_HonorsDeadline(t *testing.T) {
	c := &fakeClient{delay: 50 * time.Millisecond}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	got := FanOut(ctx, c, makeReqs(20), 2)

	deadlineErrors := 0
	for _, r := range got {
		if errors.Is(r.Err, context.DeadlineExceeded) {
			deadlineErrors++
		}
	}
	if deadlineErrors == 0 {
		t.Fatal("expected some responses to carry context.DeadlineExceeded")
	}
}
