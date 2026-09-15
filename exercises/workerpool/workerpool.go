// Package workerpool is a TYPE-IT-COLD exercise.
//
// Goal: implement Run from memory until `go test ./exercises/workerpool/` is
// green three times in a row, no peeking. The answer lives in
// reference/workerpool if you get stuck — but earn the peek.
//
// Say the three roles before you type: PRODUCE, WORK, CLOSE.
package workerpool

// Result pairs an input id with the work's output (or error).
type Result struct {
	ID    string
	Value string
	Err   error
}

// FetchFunc is one unit of work.
type FetchFunc func(id string) (string, error)

// Run fans ids out to `workers` goroutines and collects every Result.
//
// TODO: implement.
//  1. WORK:    start `workers` goroutines, each `for id := range jobs { out <- ... }`
//  2. PRODUCE: a goroutine that feeds jobs then close(jobs)
//  3. CLOSE:   a goroutine that wg.Wait() then close(out)
//  4. DRAIN:   `for r := range out { ... }` on the main goroutine
func Run(ids []string, workers int, fetch FetchFunc) []Result {
	// Delete this line and build it from memory.
	panic("not implemented — type it cold, then `go test ./exercises/workerpool/`")
}
