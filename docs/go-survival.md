# Go Survival Kit — syntax for the rusty

You know how to program. This is just the Go dialect: enough to read Go code and
write a snippet without fumbling. Skim it, then *type* the exercises — syntax
sticks through fingers, not eyes.

---

## The shapes you'll actually use

### Declarations
```go
x := 42                 // short declare + infer (inside functions only)
var y int               // zero value (0). strings "", bools false, pointers nil
const Max = 100
var buf []byte          // nil slice — usable: append works on nil
```

### Functions — multiple returns, error last
```go
func fetch(id string) (string, error) {   // (value, error) is THE Go idiom
    if id == "" {
        return "", fmt.Errorf("empty id")  // error is a value, not an exception
    }
    return "data", nil
}

v, err := fetch("x")
if err != nil {          // check immediately, every time. This is 40% of Go code.
    return err           // or wrap: fmt.Errorf("fetching: %w", err)
}
```

### Structs & methods
```go
type Server struct {
    Name string          // Capital = exported (public). lowercase = package-private.
    port int
}

func (s *Server) Start() error { ... }   // pointer receiver: can mutate, no copy
func (s Server) Name2() string { ... }   // value receiver: gets a copy

s := &Server{Name: "api", port: 8080}    // &T{...} = pointer to a new struct
```

### Interfaces — implicit, small
```go
type Completer interface {
    Complete(ctx context.Context, prompt string) (string, error)
}
// Any type with that method SATISFIES it — no "implements" keyword.
// Idiom: accept interfaces, return structs. Keep interfaces 1–3 methods.
```

### Slices & maps
```go
s := []int{1, 2, 3}
s = append(s, 4)              // append RETURNS a new slice header — reassign it
for i, v := range s { ... }   // i index, v value (copy)

m := make(map[string]int)    // MUST make() before writing, or panic on nil map
m["k"] = 1
v, ok := m["missing"]        // ok=false, v=zero. The comma-ok idiom.
delete(m, "k")
```

### Errors — wrap and inspect
```go
err := fmt.Errorf("load %s: %w", id, cause)  // %w wraps (preserves the chain)
errors.Is(err, io.EOF)                        // is this (or a wrapped) EOF?
errors.As(err, &myErr)                        // unwrap to a concrete type
```

### defer — cleanup that runs on the way out
```go
f, err := os.Open(path)
if err != nil { return err }
defer f.Close()              // runs when the function returns, LIFO order

ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
defer cancel()               // ALWAYS pair a cancel with a defer. Leaks otherwise.
```

---

## Concurrency quick-reference

```go
go doWork()                  // launch a goroutine: "start and move on", not "run now"

ch := make(chan int)         // unbuffered: send blocks until a receiver meets it
ch := make(chan int, 100)    // buffered: send blocks only when full
ch <- 5                      // send
v := <-ch                    // receive
v, ok := <-ch                // ok=false means channel is closed & drained
close(ch)                    // EOF: unblocks all receivers, ends `range ch`
for v := range ch { ... }    // receive until closed

select {                     // wait on multiple channels; first ready wins (random if tie)
case v := <-ch:   ...
case <-ctx.Done(): return ctx.Err()
default:          ...        // non-blocking: taken when nothing else is ready
}

var wg sync.WaitGroup        // wait for N goroutines
wg.Add(1); go func(){ defer wg.Done(); ... }(); wg.Wait()

var mu sync.Mutex            // guard shared mutable state
mu.Lock(); defer mu.Unlock()
```

Mental model (full version in [concurrency-model.md](concurrency-model.md)):
**channel = turnstile, not a container** · **`range ch` = blocking read** ·
**`close` = EOF** · **`go` = launch and move on**.

---

## Gotchas a reviewer might poke (name them before they do)

| Trap | What happens | Fix |
| - | - | - |
| Write to a `nil` map | panic | `make(map[k]v)` first |
| `append` result ignored | silent data loss | always `s = append(s, ...)` |
| Missing `defer cancel()` | timer/context leak | pair every `WithTimeout`/`WithCancel` with `defer cancel()` |
| Unbuffered send, no receiver | **deadlock** | buffer, or drain on another goroutine |
| `defer` inside a loop | piles up until function returns | move to a helper, or don't defer in the loop |
| Nil pointer in non-nil interface | `err != nil` is *true* unexpectedly | return `nil` explicitly, not a typed nil |
| Shared slice backing array | aliasing bug via `append` | `slices.Clone` or 3-index slice `a[low:high:max]` |

**Loop variable capture:** fixed in **Go 1.22** — each iteration gets its own
`i`, `v`, so `go func(){ use(v) }()` is safe now. Older code has `v := v` to
copy. *Knowing it changed* is the point score, not memorizing the workaround.

---

## Modern Go you can show off (1.22+)
```go
for range 5 { ... }          // range over an int (1.22)
for i := range 5 { ... }     // i = 0..4
var wg sync.WaitGroup
wg.Go(func() { ... })        // 1.25: Add(1)+Done() in one call (nice-to-know)
min(a, b); max(a, b)         // builtins (1.21)
slices.Sort(s); maps.Keys(m) // stdlib generics (1.21)
```

Use the classic `wg.Add(1)` / `defer wg.Done()` in interviews — it's what every
codebase and whiteboard expects. Mention `wg.Go` exists as a "1.25 tidied this up."

---

## Running things
```
go run ./labs/deadlock          # run a main package
go test ./...                   # all tests
go test -race ./reference/...   # with the race detector (do this for concurrency)
go vet ./...                    # cheap static checks
gofmt -w .                      # format (non-negotiable in Go culture)
```
