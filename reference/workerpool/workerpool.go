// Package workerpool is the raw-channel worker pool — the shape to type cold.
//
// Three goroutine roles, say them out loud: PRODUCE, WORK, CLOSE.
//
// The mental model that unlocks it:
//   - A channel is a turnstile, not a container. Nothing is stored; a sender and
//     a receiver meet, hand off, and both walk on.
//   - `for id := range jobs` is a BLOCKING READ, not an iteration. The worker
//     parks at that line until a value arrives. Zero CPU while parked.
//   - `close(jobs)` is EOF. That — and only that — ends every worker's range loop.
package workerpool

import "sync"

// Result pairs an input id with the work's output (or the error that killed it).
type Result struct {
	ID    string
	Value string
	Err   error
}

// FetchFunc is one unit of work: an API call, a model request, a DB read.
// It is injected so tests stay fast and deterministic.
type FetchFunc func(id string) (string, error)

// Run fans ids out to `workers` goroutines and collects every Result.
//
// `workers` is your backpressure knob: the ceiling on how many units of work run
// at once. In an AI-infra setting you bound this at N because downstream is a
// model endpoint — you want backpressure, not a thundering herd.
//
// Result order is NOT input order. Whichever worker finishes first appends first.
// If you need input order, carry an index (see reference/bounded) instead.
func Run(ids []string, workers int, fetch FetchFunc) []Result {
	jobs := make(chan string) // work goes in
	out := make(chan Result)  // results come out
	var wg sync.WaitGroup

	// WORK: each worker pulls until jobs is closed, then its range loop ends.
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for id := range jobs {
				v, err := fetch(id)
				out <- Result{ID: id, Value: v, Err: err}
			}
		}()
	}

	// PRODUCE: feed the queue on its own goroutine, then close to signal EOF.
	// This MUST be its own goroutine. If main fed jobs inline, workers would
	// block sending on the unbuffered `out` (nobody is draining it yet), main
	// would block on the next `jobs <-`, and everything deadlocks.
	go func() {
		for _, id := range ids {
			jobs <- id
		}
		close(jobs) // signals workers: no more work, exit your range loop.
	}()

	// CLOSE: once every worker has returned, nobody sends on out again, so it is
	// safe to close it. wg.Wait() lives on its own goroutine so main can reach
	// the drain loop below instead of blocking here.
	go func() {
		wg.Wait()
		close(out)
	}()

	// DRAIN: main pulls until out is closed.
	var results []Result
	for r := range out {
		results = append(results, r)
	}
	return results
}
