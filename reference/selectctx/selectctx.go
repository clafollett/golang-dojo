// Package selectctx drills the two things that turn a rusty dev into someone who
// "gets" Go concurrency: select and context.
//
// select waits on multiple channels at once and takes the first ready one. If
// several are ready it picks RANDOMLY — deliberately, so you cannot starve a case.
//
// context is a channel wearing a nice coat. ctx.Done() returns a channel that is
// CLOSED on deadline or cancel(). Closed channel = EOF = every receiver wakes at
// once. Cancellation is a broadcast via close: one cancel() unblocks a thousand
// goroutines instantly.
package selectctx

import (
	"context"
	"sync"
)

// Result carries a fetched value or the error that produced it.
type Result struct {
	ID    string
	Value string
	Err   error
}

// FetchFunc is one unit of cancellable work.
type FetchFunc func(ctx context.Context, id string) (string, error)

// Worker consumes ids until jobs is closed OR ctx is cancelled, whichever comes
// first. This is the pattern to reach for when a worker must bail mid-stream on a
// timeout — the raw range-loop worker in reference/workerpool cannot do that.
//
// Note the `, ok` on the receive: range handled channel-close for you, but inside
// a select you check it yourself. That is the one real tax for the upgrade.
func Worker(ctx context.Context, jobs <-chan string, out chan<- Result, fetch FetchFunc) {
	for {
		select {
		case id, ok := <-jobs:
			if !ok {
				return // jobs closed: no more work, clean exit.
			}
			v, err := fetch(ctx, id)
			// The send can block too. Guard it so a cancel during the send does
			// not strand this goroutine forever (a classic goroutine leak).
			select {
			case out <- Result{ID: id, Value: v, Err: err}:
			case <-ctx.Done():
				return
			}
		case <-ctx.Done():
			return // timeout or cancel: bail immediately.
		}
	}
}

// Drain runs `workers` cancellable workers over ids and returns whatever
// completed before ctx fired. Results that did not finish are simply absent —
// that is the deliberate partial-result contract. Same PRODUCE / WORK / CLOSE
// skeleton as reference/workerpool, but every blocking op is guarded by ctx.
func Drain(ctx context.Context, ids []string, workers int, fetch FetchFunc) []Result {
	jobs := make(chan string)
	out := make(chan Result)
	var wg sync.WaitGroup

	// WORK
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			Worker(ctx, jobs, out, fetch)
		}()
	}

	// PRODUCE — stop feeding the instant ctx is done.
	go func() {
		defer close(jobs)
		for _, id := range ids {
			select {
			case jobs <- id:
			case <-ctx.Done():
				return
			}
		}
	}()

	// CLOSE
	go func() {
		wg.Wait()
		close(out)
	}()

	// DRAIN
	var results []Result
	for r := range out {
		results = append(results, r)
	}
	return results
}
