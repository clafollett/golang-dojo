# System Design Talk Tracks — AI Infrastructure

The patterns behind production agent/LLM systems, with a talk track for each. The
goal isn't to memorize scripts — it's to internalize the **framework** and the
**one-liners**, then think out loud. Each track maps to runnable code in
[`reference/`](../reference/).

---

## The universal framework (run this on ANY prompt)

Say the steps as you go — narrating the method is half the signal.

1. **Clarify & scope.** Restate the problem, ask 2–3 sharp questions, state
   assumptions. (Requests/sec? Tenants? Latency budget? Consistency needs?)
2. **Define the API / contract** before internals. Resource-based, versioned.
3. **Happy path** end-to-end, then **data model** (what's stored, where, keyed how).
4. **Scale the bottleneck** — where does it break at 100×, and what's the knob.
5. **Failure modes** — retries, idempotency, timeouts, partial failure, recovery.
6. **Observability** — metrics/traces/logs; how you'd know it's broken at 3am.

> Golden move: **name the tradeoff before you're asked.** "I'd pick X over Y here
> because we care more about A than B — if that assumption's wrong, I'd flip it."

---

## §1 — Multi-tenant tool gateway with rate limiting

**Prompt shape:** "Direct enterprise customers call our shared tools/APIs. Design
the layer that isolates tenants, rate-limits them, and attributes usage."

### Clarify
- Per-tenant limits, or per-tenant *and* per-tool? Weighted by cost/tokens?
- Hard limit (reject) or soft (queue/degrade)? SLA on rejection?
- Single region or global? (Decides in-process vs shared-store limiter.)

### Core design
```
client → [gateway] → auth (who is the tenant?)  ── caller attribution
                   → rate limit check (per-tenant token bucket)
                   → tenant-scoped context (isolation: no shared mutable state)
                   → dispatch to tool
                   → emit usage event (attribution + billing + observability)
```

- **Caller attribution:** identity comes from the API key / JWT `sub` / org id.
  Everything downstream is tagged with it — logs, metrics, rate-limit key,
  usage records. "Attribution is a label you attach at the edge and never lose."
- **Rate limiting:** per-tenant **token bucket** — absorbs bursts up to `burst`,
  holds long-run average at `rate`. Beats a fixed window (no thundering herd at
  window rollover). Cost-weight expensive tools with `AllowN(tenant, cost)`.
  → runnable: [`reference/ratelimiter`](../reference/ratelimiter/ratelimiter.go)
- **Tenant isolation:** no shared mutable state across tenants; per-tenant quotas
  so a noisy neighbor can't starve others; ideally per-tenant concurrency caps too.

### The scaling boundary (say this unprompted — it's the senior signal)
> "An in-process token bucket is per-replica. Behind a load balancer that means
> the effective limit is `rate × replicas`, which drifts. For a *global* limit I'd
> move the counter to a shared store — Redis token bucket or a sliding-log — or
> push limiting to the gateway/envoy layer. In-process is the right first cut and
> a fine fallback; I'd name the consistency-vs-latency tradeoff explicitly."

### Failure modes
- Limiter store down → **fail open or fail closed?** State the choice: fail open
  protects availability, fail closed protects the downstream. Depends on whether
  the tool is expensive/dangerous.
- Return `429` with `Retry-After` and `X-RateLimit-Remaining` so callers back off.

---

## §2 — Durable agent runtime

**Prompt shape:** "An agent task runs 20 minutes across many tool calls and LLM
turns. The process crashes at minute 12. Design it so the task survives."

### The key insight
An in-memory goroutine loop dies with the process. **Durable execution** persists
*progress*, not just data — so on restart the workflow resumes from the last
completed step instead of redoing everything (or losing it).

### Core design
- **A workflow engine** (Temporal, durable step functions, or equivalent). The
  workflow's state is checkpointed; each **activity** (a tool call, an LLM call)
  is retried independently with backoff.
- **Activities must be idempotent** — at-least-once delivery means an activity can
  run twice. Key writes by a stable request id so a replay doesn't double-charge
  or double-send. "At-least-once + idempotent = effectively-once."
- **Task lifecycle:** pending → running → (paused/waiting-on-external) → done/failed.
  Long waits (human approval, a slow tool) are just the workflow parked durably —
  no goroutine held hostage.
- **Failure recovery:** deterministic replay rebuilds in-memory state from the
  event history; non-deterministic bits (LLM output, clocks, randomness) go
  *through activities* so they're recorded, not recomputed.

### One-liners
> "I'd model each agent task as a durable workflow; tool/LLM calls are idempotent
> activities with per-activity retry policies. Crash recovery is replay from the
> event log, so a mid-task crash costs one in-flight step, not the whole run."

> "LLM calls go through activities, not inline, because replay must be
> deterministic — anything non-deterministic has to be recorded once and replayed,
> never re-rolled."

---

## §3 — Agent orchestration loop (fan-out + context)

**Prompt shape:** "Walk me through the loop that runs an agent turn: it may call
several tools, some in parallel, with a deadline."

### Core design
```
loop per turn:
  build context (system prompt + memory + tool schemas)   ── isolated per task
  call LLM → get tool calls
  fan out tool calls, BOUNDED, honoring the turn deadline  ── backpressure
  collect results (one error per tool ≠ whole turn dies)
  append to context, decide: another turn or finish
```

- **Bounded fan-out** to the model/tool endpoints — this is literally
  [`reference/fanout`](../reference/fanout/fanout.go). Bound = backpressure against
  the downstream and your rate limit / token budget.
- **Context management across turns:** what carries forward (running summary,
  tool results, memory) vs. what you drop to control token cost. Mention
  **prefix caching** — a stable prefix (system prompt + tools) is cached so
  repeated turns pay less latency and cost.
- **Isolated context per task, shared memory where intended** — one task's scratch
  state doesn't bleed into another's; shared memory is explicit and scoped.
- **Deadline:** one `context.Context` with a timeout threaded through every call;
  cancel propagates to all in-flight tool calls at once (close = broadcast).

### One-liner
> "I bound the tool fan-out at N because downstream is the model endpoint — I want
> backpressure, not a thundering herd — and I thread one context deadline through
> the whole turn so a timeout cancels every in-flight call at once."

---

## §4 — Multi-provider LLM abstraction

**Prompt shape:** "Support Anthropic and OpenAI (and swap freely)."

- **Interface at the seam:** define your own `Completer`/`ModelClient` interface;
  write one adapter per provider. Your agent loop depends on the interface, never
  a vendor SDK. (Exactly the injected `ModelClient` in `reference/fanout`.)
- Normalize the divergent bits: tool-call schema, streaming format, token
  accounting, error/retry taxonomy, stop reasons.
- Cross-cutting concerns (retries, rate limit, metrics, prefix caching) live in a
  **decorator** around the interface, not duplicated per provider.

> "Provider choice becomes a config-time decision, not a code change. The blast
> radius of a new provider is one adapter + a golden-scenario run."

---

## §5 — Eval-driven development for LLM systems

**Prompt shape:** "How do you test something non-deterministic?"

- **Golden scenarios:** curated input → expected behavior/outcome (not exact
  string match). Assert on structure, tool-call correctness, and graded quality.
- **Automated eval in CI as a quality gate** — a regression in answer quality
  fails the pipeline like a unit test would. Eval is a parallel engineering
  workstream, not an afterthought.
- **Regression detection:** track eval scores over time; alert on drift when a
  prompt, model version, or tool changes.
- **Grading:** deterministic checks where possible; LLM-as-judge for the fuzzy
  parts, with the judge itself pinned and eval'd.
- **Observability feeds eval:** a replay/observability layer captures real traffic
  → becomes tomorrow's golden scenarios (a supervised learning loop).

---

## §6 — Progressive tool discovery

**Prompt shape:** "The agent has 500 tools. Don't dump 500 schemas into context."

- **Deferred loading:** load a tool's full schema only when it's a candidate.
- **Search / semantic filtering:** embed tool descriptions; retrieve the top-k
  relevant to the current task instead of listing all.
- **Categorization / namespacing:** group tools; the model picks a category, then
  a tool — two cheap hops beat one enormous prompt.
- Tie it back to cost: "every tool schema in context is tokens on every turn;
  progressive discovery keeps the prefix small, which also helps prefix caching."

*(This is how Claude Code's own deferred-tool loading works — a shipping example
of the pattern in the wild.)*

---

## Vocabulary bank — drop these naturally

**Backpressure · bounded concurrency · thundering herd · idempotency ·
at-least-once vs exactly-once · effectively-once · caller attribution · tenant
isolation · noisy neighbor · fail open / fail closed · graceful shutdown ·
context deadline · circuit breaker · retry with jittered backoff · durable
execution · deterministic replay · prefix caching · golden scenarios · quality
gate · blast radius · resource-based API · deferred loading.**

Each one is a flag that says "I've operated this at scale." Use them where they're
true — never as filler.
