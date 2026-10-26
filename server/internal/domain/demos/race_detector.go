// Topic 06 goroutines-and-channels — the conc:race demo: the race
// detector as a habit, taught by contrast. The two CORRECT
// synchronizations run live on every request — a mutex-guarded counter
// and a channel-fold (share memory by communicating) — and the RACY
// variant is pinned as data: its exact code and the race detector's own
// verdict, captured from a documented `go test -race` run of that code in
// /tmp/race-test (2026-10-26, go1.27.3). The racy function NEVER executes
// here — no builder or test in this package may link or call it — so the
// guide's own `-race` verification stays green by construction. A hygiene
// beat runs the leaked-vs-done contrast live, without leaking anything.
package demos

import (
	"fmt"
	"sync"
)

// The synchronized runs' fixed configuration: enough goroutines and
// increments that a lost update would be certain if the synchronization
// were removed, and the total still lands exactly on 5000.
const (
	raceGoroutines     = 200
	raceIncrementsEach = 25
	raceExpectedTotal  = raceGoroutines * raceIncrementsEach
)

// racyCounterCode is the exhibit the pinned verdict was captured from,
// verbatim — including the missing synchronization, which is the entire
// point. It is DATA served on the wire, never code this package runs.
const racyCounterCode = `package raceexhibit

import "sync"

func RacyTotal(goroutines, increments int) int {
	counter := 0
	var wg sync.WaitGroup
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < increments; j++ {
				counter++
			}
		}()
	}
	wg.Wait()
	return counter
}`

// raceVerdict is the race detector's response to racyCounterCode, from one
// documented `go test -race` run (go1.27.3): the first WARNING block and
// the FAIL tail, verbatim; only the second WARNING block is elided (the
// detector typically reports several). Addresses, goroutine ids, and the
// lost-update count vary run to run — the SHAPE is what the toolchain
// guarantees. Note the tail's arithmetic: the same run read 3900, not
// 5000 — 1100 increments vanished between the racing read-modify-writes.
const raceVerdict = `==================
WARNING: DATA RACE
Read at 0x00c0000141f8 by goroutine 17:
  raceexhibit.RacyTotal.func1()
      /tmp/race-test/racy.go:13 +0x94

Previous write at 0x00c0000141f8 by goroutine 10:
  raceexhibit.RacyTotal.func1()
      /tmp/race-test/racy.go:13 +0xa4

Goroutine 17 (running) created at:
  raceexhibit.RacyTotal()
      /tmp/race-test/racy.go:10 +0x74
  raceexhibit.TestRacyTotal()
      /tmp/race-test/racy_test.go:6 +0x2c
  testing.tRunner()
      /usr/local/go/src/testing/testing.go:2195 +0x164
  testing.(*T).Run.gowrap1()
      /usr/local/go/src/testing/testing.go:2260 +0x34
… second WARNING block elided …
==================
--- FAIL: TestRacyTotal (0.00s)
    racy_test.go:7: racy total = 3900, want 5000
    testing.go:1867: race detected during execution of test
FAIL`

// RaceExhibit is the pinned proof: the racy code, the command whose
// output the verdict came from, the verdict itself, what the detector
// actually checks, and the two fixes this demo runs live.
type RaceExhibit struct {
	Code        string `json:"code"`
	Command     string `json:"command"`
	Verdict     string `json:"verdict"`
	Explanation string `json:"explanation"`
	Fix         string `json:"fix"`
}

// SynchronizedRun is one live, race-free total: the strategy, the shape,
// and the exact arithmetic — observed must equal expected, every call.
type SynchronizedRun struct {
	Strategy       string `json:"strategy"`
	Goroutines     int    `json:"goroutines"`
	IncrementsEach int    `json:"incrementsEach"`
	Expected       int    `json:"expected"`
	Observed       int    `json:"observed"`
	Exact          bool   `json:"exact"`
}

// HygieneRow is one live leaked-vs-done case: what the goroutine's last
// channel write looked like, whether a reader was there, and how it
// exited. Both cases are run for real — without leaking anything.
type HygieneRow struct {
	Case          string `json:"case"`
	ReaderPresent bool   `json:"readerPresent"`
	Outcome       string `json:"outcome"`
}

// RaceDetectorPayload is the wire shape of GET /api/demo/conc/race
// (camelCase JSON) — the goroutines-and-channels topic's race-and-hygiene
// demo.
type RaceDetectorPayload struct {
	Race    RaceExhibit     `json:"race"`
	Mutex   SynchronizedRun `json:"mutex"`
	Channel SynchronizedRun `json:"channel"`
	Hygiene []HygieneRow    `json:"hygiene"`
}

// mutexTotal is fix one: the shared counter guarded by a sync.Mutex, the
// same shape the ScenarioStore uses for its state. Every increment is a
// critical section — Lock, mutate, Unlock — and the WaitGroup proves all
// the work finished before the total is read.
func mutexTotal(goroutines, increments int) int {
	var (
		mu      sync.Mutex
		counter int
		wg      sync.WaitGroup
	)
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < increments; j++ {
				mu.Lock()
				counter++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	return counter
}

// channelTotal is fix two: no shared variable at all. Each goroutine owns
// a local counter — unsynchronized, because nobody else can touch it —
// and sends exactly one tally over a buffered channel; the fold happens
// in ONE goroutine after the join. Share memory by communicating.
func channelTotal(goroutines, increments int) int {
	tallies := make(chan int, goroutines)
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			local := 0
			for j := 0; j < increments; j++ {
				local++
			}
			tallies <- local
		}()
	}
	wg.Wait()
	close(tallies)
	total := 0
	for tally := range tallies {
		total += tally
	}
	return total
}

// raceHygieneRuns runs the leaked-vs-done contrast live. The done case:
// an unbuffered send with a reader waiting — handoff, both sides return.
// The would-leak case: the reader is gone, so the send is guarded by a
// select with a default arm and is abandoned instead of blocking; a plain
// send there would park the goroutine forever — the leak. Both goroutines
// are joined before the rows exist, which is the point of the rows.
func raceHygieneRuns() []HygieneRow {
	var wg sync.WaitGroup
	wg.Add(2)

	delivered := make(chan string)
	go func() {
		defer wg.Done()
		delivered <- "done" // reader is waiting below: handoff, return
	}()

	unguardedWouldBlock := make(chan int)
	go func() {
		defer wg.Done()
		select {
		case unguardedWouldBlock <- 1: // no reader exists: never taken
		default: // abandoned, not parked — the goroutine still returns
		}
	}()

	result := <-delivered
	wg.Wait()
	return []HygieneRow{
		{
			Case:          "unbuffered send with a waiting reader",
			ReaderPresent: true,
			Outcome:       fmt.Sprintf("delivered %q — goroutine returned", result),
		},
		{
			Case:          "unbuffered send after the reader is gone, guarded by select+default",
			ReaderPresent: false,
			Outcome:       "send abandoned by the default arm — goroutine returned (a plain send would block forever: the leak)",
		},
	}
}

// BuildRaceDetectorPayload runs the two synchronized totals live and pairs
// them with the pinned racy exhibit. Same call, same payload — the live
// totals are exact by construction, and nothing racy ever executes.
func BuildRaceDetectorPayload() RaceDetectorPayload {
	mutexObserved := mutexTotal(raceGoroutines, raceIncrementsEach)
	channelObserved := channelTotal(raceGoroutines, raceIncrementsEach)
	return RaceDetectorPayload{
		Race: RaceExhibit{
			Code:    racyCounterCode,
			Command: "go test -race -count=1 .",
			Verdict: raceVerdict,
			Explanation: "the race detector instruments loads and stores and flags any pair with no " +
				"happens-before edge between them — counter++ is a load, an add, and a store, so two " +
				"goroutines between the same pair race; it is dynamic: it only fires on accesses the " +
				"run actually performs, which is why the habit is running tests with -race always",
			Fix: "synchronize: guard the shared variable with a mutex (the live mutex run), or remove " +
				"the sharing and pass ownership over a channel (the live channel run)",
		},
		Mutex: SynchronizedRun{
			Strategy:       "sync.Mutex — every increment a critical section",
			Goroutines:     raceGoroutines,
			IncrementsEach: raceIncrementsEach,
			Expected:       raceExpectedTotal,
			Observed:       mutexObserved,
			Exact:          mutexObserved == raceExpectedTotal,
		},
		Channel: SynchronizedRun{
			Strategy:       "channel fold — local tallies, one owner folds after the join",
			Goroutines:     raceGoroutines,
			IncrementsEach: raceIncrementsEach,
			Expected:       raceExpectedTotal,
			Observed:       channelObserved,
			Exact:          channelObserved == raceExpectedTotal,
		},
		Hygiene: raceHygieneRuns(),
	}
}
