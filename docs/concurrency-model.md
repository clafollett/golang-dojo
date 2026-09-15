# The Concurrency Mental Model

The one model that unlocks Go concurrency. Read it once, then *run the code* it
points to.

---

## Four sentences that explain everything

1. **A channel is a turnstile, not a container.** Nothing is stored in an
   unbuffered channel — a sender and a receiver *meet*, hand off, and both walk
   on. Early arrival waits.
2. **`for v := range ch` is a blocking read, not an iteration.** The goroutine
   hits that line, finds nothing, and **parks**. Zero CPU. A send wakes exactly
   one parked receiver.
3. **`close(ch)` is EOF.** It ends every `range`, and wakes every receiver still
   waiting. A closed channel is the only thing that ends a range loop.
4. **`go f()` means "launch and move on," not "run now."** Source order ≠
   execution order. Five goroutines can all be parked at the door within
   microseconds while main keeps going.

Deadlock, then, is just: *everyone is waiting at a turnstile and nobody's coming.*

---

## The three goroutine roles (say them out loud)

**PRODUCE · WORK · CLOSE.**

- **Produce:** feed the jobs channel, then `close(jobs)`.
- **Work:** `for id := range jobs` — pull until EOF.
- **Close:** `wg.Wait()` then `close(out)` — so the drain loop can end.

Each on its own goroutine, because doing any of them inline on `main` blocks main
before it can drain → deadlock. → See it fail live: `go run ./labs/deadlock`.

---

## Timeline (2 workers, unbuffered channels)

| | t0 launch | t1 | t2 | t3 close |
| - | - | - | - | - |
| **Producer** | parked | sends id | sends id | closes jobs |
| **Worker A** | parked | fetch | sends out | exits |
| **Worker B** | parked | parked | fetch | exits |
| **Main** | parked | parked | gets result | loop ends |

Three readings:
- **t0 is all parked.** "`range jobs` runs before jobs has anything" is fine —
  running means parked at the turnstile, costing nothing.
- **Worker B idles through t0–t1.** Nobody assigned it work; A got there first.
  That's free load balancing — the scheduler picks, you don't.
- **Main is the deadlock lesson.** If main were also the producer, it'd be stuck
  feeding jobs at t0 and never reach the drain loop — so at t2 Worker A has
  nowhere to send. Everything gray forever.

---

## Map: concept → runnable code

| Concept | Package | Run it |
| - | - | - |
| Raw worker pool (produce/work/close) | [`reference/workerpool`](../reference/workerpool/) | `go test ./reference/workerpool/` |
| Idiomatic bounded pool (errgroup) | [`reference/bounded`](../reference/bounded/) | `go test ./reference/bounded/` |
| select + context cancellation | [`reference/selectctx`](../reference/selectctx/) | `go test ./reference/selectctx/` |
| Bounded fan-out to a model endpoint | [`reference/fanout`](../reference/fanout/) | `go test ./reference/fanout/` |
| What a missing `close` costs you | [`labs/deadlock`](../labs/deadlock/) | `go run ./labs/deadlock` |

---

## select + context in one breath

`select` waits on several channels; first ready wins; ties break **randomly** (so
you can't starve a case). `context` is a channel wearing a coat: `ctx.Done()`
returns a channel that is **closed** on timeout/cancel — closed = EOF = every
receiver wakes at once. One `cancel()` broadcasts to a thousand goroutines.

```go
select {
case job := <-jobs:      // whichever turnstile is ready first
    process(job)
case <-ctx.Done():       // cancellation/timeout arrives here
    return ctx.Err()
}
```

Inside a `select` you check close yourself with `, ok` (range did it for you):
```go
case id, ok := <-jobs:
    if !ok { return }    // jobs closed → done
```

---

## Say these out loud
- *"Parked, not spinning."* (a blocked goroutine costs nothing)
- *"Close is EOF."*
- *"The scheduler picks the worker, I don't."*
- *"I bound at N for backpressure, not a thundering herd."*
- *"Deadlock"* — before they say it.
