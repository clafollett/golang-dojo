// Command deadlock is the "break it and watch it turn gray" lab.
//
// Run it:  go run ./labs/deadlock
//
// It runs the SAME worker pool twice: once with close(jobs), once without. A
// watchdog turns the classic hang into a printed diagnosis instead of freezing
// your terminal, so you can see exactly what a missing close costs you.
//
// The chain of failure when close(jobs) is removed:
//  1. workers drain the fed jobs, then park in `for id := range jobs` — a range
//     over a channel only ends on close, and close never comes.
//  2. parked workers never return, so wg never reaches 0.
//  3. the closer goroutine sits in wg.Wait() forever, so close(out) never runs.
//  4. main receives its N results, then parks in `for r := range out` waiting
//     for a close that will never happen.
//
// close is EOF. Delete it and every downstream goroutine leaks, parked, forever.
package main

import (
	"fmt"
	"sync"
	"time"
)

func main() {
	ids := []string{"a", "b", "c", "d", "e"}

	fmt.Println("=== FIXED: close(jobs) present ===")
	report(runWithWatchdog(func() []string { return runFixed(ids) }))

	fmt.Println("\n=== BROKEN: close(jobs) removed ===")
	report(runWithWatchdog(func() []string { return runBroken(ids) }))

	fmt.Println("\nSay it out loud: range ends on close; close is EOF; no close = parked forever.")
}

// runWithWatchdog runs fn in a goroutine and gives it 2 seconds. If it does not
// finish, we call it deadlocked. (The leaked goroutines die when the process
// exits — in real code that leak is a slow memory bleed and a paged on-call.)
func runWithWatchdog(fn func() []string) []string {
	done := make(chan []string, 1)
	go func() { done <- fn() }()
	select {
	case r := <-done:
		return r
	case <-time.After(2 * time.Second):
		return nil // deadlocked
	}
}

func report(r []string) {
	if r == nil {
		fmt.Println("  -> DEADLOCK: watchdog fired; workers + closer + main all parked.")
		return
	}
	fmt.Printf("  -> completed with %d results: %v\n", len(r), r)
}

func runFixed(ids []string) []string {
	jobs := make(chan string)
	out := make(chan string)
	var wg sync.WaitGroup

	for range 3 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for id := range jobs {
				out <- "v-" + id
			}
		}()
	}
	go func() {
		for _, id := range ids {
			jobs <- id
		}
		close(jobs) // the one line that lets the workers exit.
	}()
	go func() {
		wg.Wait()
		close(out)
	}()

	var results []string
	for r := range out {
		results = append(results, r)
	}
	return results
}

func runBroken(ids []string) []string {
	jobs := make(chan string)
	out := make(chan string)
	var wg sync.WaitGroup

	for range 3 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for id := range jobs { // <- parks here forever once jobs is drained
				out <- "v-" + id
			}
		}()
	}
	go func() {
		for _, id := range ids {
			jobs <- id
		}
		// close(jobs)  <-- THE BUG: this line is deleted. Everything downstream leaks.
	}()
	go func() {
		wg.Wait() // never returns: workers never exit their range loop
		close(out)
	}()

	var results []string
	for r := range out { // receives 5, then parks forever: out is never closed
		results = append(results, r)
	}
	return results
}
