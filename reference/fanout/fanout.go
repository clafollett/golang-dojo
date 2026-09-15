// Package fanout is the AI-infra fan-out pattern: send a batch of requests to a
// model / tool endpoint, cap how many are in flight, honor a deadline, and return
// one result per request — one bad call does not sink the batch.
//
// This is the concrete answer to "how would you call the model endpoint for N
// items?" The bound is not an optimization; it is a contract with the downstream:
// backpressure, not a thundering herd. You size it to what the endpoint (and your
// rate limit / token budget) can absorb.
//
// Two ways to bound concurrency in Go:
//   - counting semaphore (a buffered channel), shown here — the primitive
//   - errgroup.SetLimit(n), shown in reference/bounded — the same idea, less code
//
// Reach for errgroup in real code. Know the semaphore so you can explain what
// errgroup is doing under the hood, and so you are not stuck if it is banned.
package fanout

import (
	"context"
	"sync"
	"time"
)

// Request is one unit of work sent downstream (an LLM completion, an MCP tool call).
type Request struct {
	ID     string
	Prompt string
}

// Response carries the outcome of one Request, including its own error so a
// single failure is isolated to its item.
type Response struct {
	ID      string
	Output  string
	Latency time.Duration
	Err     error
}

// ModelClient is the downstream you are protecting with the concurrency bound.
// Injecting an interface keeps tests fast and lets you swap providers — exactly
// what a multi-provider LLM abstraction layer needs.
type ModelClient interface {
	Complete(ctx context.Context, req Request) (string, error)
}

// FanOut sends every request to the client with at most maxConcurrent in flight,
// honoring ctx. Results come back in input order; each carries its own error.
//
// If ctx is already done (or trips mid-loop), remaining requests are marked with
// ctx.Err() instead of being dispatched — fail fast, do not pile work onto a
// deadline that has already blown.
func FanOut(ctx context.Context, client ModelClient, reqs []Request, maxConcurrent int) []Response {
	responses := make([]Response, len(reqs))  // one slot per request: no mutex
	sem := make(chan struct{}, maxConcurrent) // counting semaphore = the bound
	var wg sync.WaitGroup

	for i, req := range reqs {
		// Acquire a slot. A full semaphore blocks here — THAT is the backpressure.
		// Race it against ctx so we do not wait on a slot after the deadline blows.
		select {
		case <-ctx.Done():
			responses[i] = Response{ID: req.ID, Err: ctx.Err()}
			continue
		case sem <- struct{}{}:
		}

		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() { <-sem }() // release the slot

			start := time.Now()
			out, err := client.Complete(ctx, req)
			responses[i] = Response{
				ID:      req.ID,
				Output:  out,
				Latency: time.Since(start),
				Err:     err,
			}
		}()
	}

	wg.Wait()
	return responses
}
