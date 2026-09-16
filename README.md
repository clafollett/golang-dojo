# 🥋 golang-dojo

A hands-on crash course in **Go concurrency and AI-infrastructure patterns** —
runnable, tested reference code plus system-design talk tracks. Built for an
engineer getting back up to speed in Go quickly and for real: the patterns a
modern backend / AI-infrastructure role actually leans on.

Read the docs, **type** the code, rehearse the design tracks out loud.

---

## What's inside

Concurrency and the patterns that sit on top of it in production AI/agent systems:

- **Worker pools** — the raw-channel `produce / work / close` shape, and the
  idiomatic `errgroup` version (bounded, cancellable, first-error wins).
- **`select` + `context`** — cancellation, deadlines, partial results.
- **Bounded fan-out to a model/tool endpoint** — the shape behind an agent loop
  calling N tools: backpressure, per-item errors, deadline propagation.
- **Multi-tenant rate limiting** — a per-tenant token bucket with caller
  attribution and tenant isolation.
- **A deadlock lab** — delete one `close()` and watch everything park.

Plus design talk tracks for the questions these patterns raise at scale.

---

## Quick start

```bash
go test ./reference/...          # all reference patterns, green
go test -race ./reference/...    # ...and clean under the race detector
go run  ./labs/deadlock          # watch a missing close() deadlock, diagnosed live
go test ./exercises/...          # RED until you implement them — that's the point
make help                        # the shortcuts
```

Prereqs: Go 1.22+ (`gopls`, `staticcheck` optional but recommended).

---

## Repo map

```
docs/
  system-design.md       # talk tracks: rate limiting, durable runtimes, agent loops, eval…
  go-survival.md         # Go syntax for the rusty (the "tax")
  concurrency-model.md   # the turnstile mental model, mapped to the code
reference/               # runnable, tested — READ and RUN these (the answers)
  workerpool/            # raw-channel worker pool: produce / work / close
  bounded/               # idiomatic errgroup version (bounded + cancel + first-error)
  selectctx/             # select + context cancellation, partial results
  fanout/                # bounded fan-out to a model endpoint
  ratelimiter/           # multi-tenant token bucket
exercises/               # TYPE-IT-COLD skeletons; tests provided (the practice)
  workerpool/            # implement Run from memory
  ratelimiter/           # implement the token bucket from memory
labs/
  deadlock/              # break it: delete close(), watch everything park
```

**reference = answers · exercises = practice · labs = experiments · docs = the course.**

---

## How to use it

1. Read [`docs/concurrency-model.md`](docs/concurrency-model.md) once — the
   turnstile model that makes Go concurrency click.
2. `go test ./reference/...` and read each package alongside its tests.
3. Type the [`exercises/`](exercises/) from memory until they're green — three
   times, no peeking. Syntax sticks through fingers, not eyes.
4. Run [`labs/deadlock`](labs/deadlock/) and trace *why* the broken version hangs.
5. Rehearse [`docs/system-design.md`](docs/system-design.md) out loud — the
   framework and the one-liners, not a memorized script.

**The one drill that matters most:** type `reference/workerpool`'s `Run` cold,
from memory, three times, green each time — and be able to explain why each
goroutine needs to be its own goroutine. Everything else is leverage on top.

---

## Practice safely — the sandbox 🧪

The committed exercises are the *prompts*; your *answers* belong in a throwaway
zone so you never fear committing half-finished practice. `make practice` copies
exercises into `_practice/`, a **gitignored nested Go module**:

```bash
make practice EX=workerpool   # copy one exercise (or bare `make practice` for all)
cd _practice && go test ./workerpool/   # fill it in, test it, repeat
make practice EX=workerpool   # re-run any time to reset to a fresh blank
```

Why a *nested module* and not just `.gitignore`? Because `.gitignore` hides files
from git, not from the Go compiler — `go test ./...` walks the filesystem, so a
plain ignored dir would still get built. A directory with its own `go.mod` is a
separate module that the root's `./...` skips entirely. Belt (nested module),
suspenders (`_` prefix), and a parachute (`.gitignore`).

---

## How this was built 🤖🤝

This repo was built by a human directing an AI pair-programmer: the human set the
goals and scope and reviewed and verified every line; the AI generated the code
and docs. Every reference package passes `go vet`, `staticcheck`, and
`go test -race`; the exercises intentionally start red. That workflow — knowing
*what* to build, steering the model, and shipping it verified and clean — is the
point as much as the Go is.

---

## License

[MIT](LICENSE) — do (almost) anything, just keep the notice. PRs and forks welcome.
