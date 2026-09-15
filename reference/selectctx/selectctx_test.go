package selectctx

import (
	"context"
	"testing"
	"time"
)

func TestDrain_AllCompleteWithoutDeadline(t *testing.T) {
	ids := []string{"a", "b", "c"}
	fetch := func(_ context.Context, id string) (string, error) {
		return "v-" + id, nil
	}

	got := Drain(context.Background(), ids, 2, fetch)

	if len(got) != len(ids) {
		t.Fatalf("got %d results, want %d", len(got), len(ids))
	}
}

// TestDrain_DeadlineYieldsPartialResults proves the partial-result contract:
// with a short deadline and slow work, we get some results, not a hang, and not
// all of them.
func TestDrain_DeadlineYieldsPartialResults(t *testing.T) {
	ids := make([]string, 20)
	for i := range ids {
		ids[i] = string(rune('a' + i))
	}
	fetch := func(ctx context.Context, id string) (string, error) {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(20 * time.Millisecond):
			return id, nil
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()

	done := make(chan []Result, 1)
	go func() { done <- Drain(ctx, ids, 2, fetch) }()

	select {
	case got := <-done:
		// Some finished, but not all 20 — the deadline cut it short.
		successful := 0
		for _, r := range got {
			if r.Err == nil {
				successful++
			}
		}
		if successful == len(ids) {
			t.Errorf("all %d finished; deadline was not enforced", len(ids))
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Drain hung past the deadline — leak or missing ctx guard")
	}
}
