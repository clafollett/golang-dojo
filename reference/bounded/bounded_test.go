package bounded

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestProcess_ReturnsInputOrder(t *testing.T) {
	ids := []string{"a", "b", "c", "d"}
	fetch := func(_ context.Context, id string) (string, error) {
		// Sleep inversely to position so later ids finish first — if the result
		// order still matches input order, index-per-worker is doing its job.
		time.Sleep(time.Duration(len(id)) * time.Millisecond)
		return strings.ToUpper(id), nil
	}

	got, err := Process(context.Background(), ids, 4, fetch)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"A", "B", "C", "D"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("position %d: got %q, want %q (full: %v)", i, got[i], want[i], got)
		}
	}
}

func TestProcess_FirstErrorWins(t *testing.T) {
	ids := []string{"ok1", "boom", "ok2"}
	fetch := func(_ context.Context, id string) (string, error) {
		if id == "boom" {
			return "", errors.New("kaboom")
		}
		return id, nil
	}

	_, err := Process(context.Background(), ids, 3, fetch)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if !strings.Contains(err.Error(), "fetch boom") {
		t.Errorf("error not wrapped with id context: %v", err)
	}
}

// TestProcess_CancelStopsSiblings proves the shared ctx cancels remaining work
// after the first failure: far fewer than all fetches actually start.
func TestProcess_CancelStopsSiblings(t *testing.T) {
	const n = 100
	ids := make([]string, n)
	for i := range ids {
		ids[i] = fmt.Sprintf("id-%d", i)
	}

	var started atomic.Int64
	fetch := func(ctx context.Context, id string) (string, error) {
		started.Add(1)
		if id == "id-0" {
			return "", errors.New("early failure")
		}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(10 * time.Millisecond):
			return id, nil
		}
	}

	_, err := Process(context.Background(), ids, 4, fetch)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if s := started.Load(); s == n {
		t.Errorf("all %d fetches started; cancellation did not stop siblings", n)
	}
}

func TestProcess_HonorsCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already dead on arrival

	_, err := Process(ctx, []string{"a", "b"}, 2, func(_ context.Context, id string) (string, error) {
		return id, nil
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("got %v, want context.Canceled", err)
	}
}
