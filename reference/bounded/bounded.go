// Package bounded is the idiomatic version of the worker pool using errgroup.
//
// One shape answers most concurrency needs. Reach for this first in real code;
// fall back to the raw channels in reference/workerpool when errgroup is off the
// table (or when you want to show the primitive underneath).
//
// What errgroup buys you over the raw version:
//   - g.SetLimit(n)  → bounded concurrency with no manual semaphore
//   - errgroup.WithContext → cancels the shared ctx on the FIRST error, so
//     siblings stop early instead of finishing doomed work
//   - g.Wait()       → returns that first non-nil error
//
// Index-per-worker (results[i] = v) means each goroutine writes a distinct slot,
// so there is no shared write and no mutex — and results come back IN INPUT ORDER.
package bounded

import (
	"context"
	"fmt"

	"golang.org/x/sync/errgroup"
)

// FetchFunc is one unit of work. It takes ctx so it can honor cancellation and
// deadlines — every blocking call in idiomatic Go takes a context.
type FetchFunc func(ctx context.Context, id string) (string, error)

// Process fans ids out to at most `workers` concurrent fetches. It returns
// results in input order, or the first error encountered (which also cancels
// the rest via the shared context).
func Process(ctx context.Context, ids []string, workers int, fetch FetchFunc) ([]string, error) {
	g, ctx := errgroup.WithContext(ctx) // cancels ctx on first error
	g.SetLimit(workers)                 // the backpressure knob

	results := make([]string, len(ids)) // one slot per id: no mutex needed
	for i, id := range ids {
		// Go 1.22+ gives each iteration its own i, id — no `i, id := i, id`
		// shadowing needed. Knowing THAT it changed scores points.
		g.Go(func() error {
			// Honor cancellation before doing expensive work.
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}
			v, err := fetch(ctx, id)
			if err != nil {
				return fmt.Errorf("fetch %s: %w", id, err) // wrap, don't swallow
			}
			results[i] = v
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return nil, err
	}
	return results, nil
}
