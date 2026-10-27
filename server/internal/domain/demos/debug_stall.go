// Topic 35 debugging's live demo: the stall. The demo's machinery is a
// REAL wedge: a bounded worker set where every worker finishes tiny work
// except one, which blocks forever on a channel receive nobody serves —
// then the process captures its OWN goroutine profile
// (runtime/pprof.Lookup("goroutine"), the same profile
// /internal/debug/pprof/goroutine serves since topic 33), parses it, and
// extracts the wedged goroutine's stack: its id, its "chan receive"
// state, its frames mapped to source lines, and the frames of a healthy
// goroutine from the SAME profile for contrast. Nothing in the payload
// is invented — the profile text comes from the runtime at capture
// time — and the wedge is released deterministically before the run
// returns, so a request can never wedge on its own demo.
package demos

import (
	"runtime/pprof"
	"strconv"
	"strings"
	"time"
)

// DebugStallPath is the demo area's endpoint — registered on the real
// mux by the composition root.
const DebugStallPath = "/api/demo/debug/stall"

// DebugStallWorkers is the size of each run's worker set: four workers
// that complete tiny bounded work, plus the one designated wedge.
const DebugStallWorkers = 5

// DebugStallSettle bounds every wait the run makes: healthy workers
// settling, the scheduler parking the wedge onto its channel receive,
// and the wedge exiting after release. Each phase completes in
// microseconds; the bound exists so a pathological machine degrades the
// payload's honesty, never the request's liveness.
const DebugStallSettle = 2 * time.Second

// GoroutineFrame is one stack frame from a goroutine profile: the
// function as the runtime prints it (package-qualified, with receiver
// and arguments) and the file:line the frame is executing — the exact
// location a reader would jump to in an editor or hand to delve.
type GoroutineFrame struct {
	Function string `json:"function"`
	Location string `json:"location"`
}

// GoroutineSnapshot is one parsed goroutine from the runtime's own
// profile text: its id, its scheduler state (the bracket in the
// header — "chan receive", "runnable", "IO wait", …), the optional
// how-long marker the runtime appends to the state, its frames in
// stack order, and — when present — the created-by frame that names
// where the goroutine was spawned.
type GoroutineSnapshot struct {
	ID        int              `json:"id"`
	State     string           `json:"state"`
	Wait      string           `json:"wait,omitempty"`
	Frames    []GoroutineFrame `json:"frames"`
	CreatedBy string           `json:"createdBy,omitempty"`
	CreatedAt string           `json:"createdAt,omitempty"`
}

// Matches reports whether any frame of the snapshot mentions marker —
// the finder's predicate: a marker like "debugStallWedge" identifies
// the wedged goroutine by its own stack, without trusting ids or
// profile ordering.
func (s GoroutineSnapshot) Matches(marker string) bool {
	if marker == "" {
		return false
	}
	for _, frame := range s.Frames {
		if strings.Contains(frame.Function, marker) {
			return true
		}
	}
	return false
}

// goroutineHeader splits a profile header line like
//
//	goroutine 42 [chan receive, 5 minutes]:
//
// into id 42, state "chan receive", and wait "5 minutes". The wait is
// whatever follows the last comma when it reads as a duration; states
// themselves may carry commas ("select, sync.Mutex.Lock"), so the
// split only happens when the tail starts with a digit.
func goroutineHeader(line string) (id int, state, wait string, ok bool) {
	if !strings.HasPrefix(line, "goroutine ") || !strings.HasSuffix(line, "]:") {
		return 0, "", "", false
	}
	body := strings.TrimSuffix(strings.TrimPrefix(line, "goroutine "), "]:")
	fields := strings.SplitN(body, " ", 3)
	if len(fields) < 2 || !strings.HasPrefix(fields[1], "[") {
		return 0, "", "", false
	}
	parsed, err := strconv.Atoi(fields[0])
	if err != nil {
		return 0, "", "", false
	}
	if len(fields) == 3 {
		state = strings.TrimPrefix(fields[1], "[") + " " + fields[2]
	} else {
		state = strings.TrimPrefix(fields[1], "[")
		state = strings.TrimSuffix(state, "]")
	}
	if comma := strings.LastIndex(state, ", "); comma >= 0 && looksLikeDuration(strings.TrimSpace(state[comma+1:])) {
		wait = strings.TrimSpace(state[comma+1:])
		state = state[:comma]
	}
	return parsed, state, wait, true
}

// looksLikeDuration reports whether s reads like the runtime's
// duration suffixes ("5 minutes", "30 seconds", "1 hours") — a digit
// run followed by a word.
func looksLikeDuration(s string) bool {
	if s == "" || s[0] < '0' || s[0] > '9' {
		return false
	}
	_, _, found := strings.Cut(s, " ")
	return found
}

// ParseGoroutineProfile reduces the runtime's goroutine profile text
// (the format `pprof.Lookup("goroutine").WriteTo(w, 2)` produces, the
// same text /internal/debug/pprof/goroutine?debug=2 serves) into one
// snapshot per goroutine. The grammar, as observed from the runtime:
// a header line, then frame pairs — a function line at column zero
// followed by a tab-indented file:line — optionally closed by a
// "created by" pair naming the spawner. Anything else (blank lines,
// the runtime's notes like "goroutine running on other thread; stack
// unavailable") is skipped, never guessed at.
func ParseGoroutineProfile(raw string) []GoroutineSnapshot {
	snapshots := []GoroutineSnapshot{}
	current := -1 // index into snapshots; -1 before the first header
	locationNext := false

	for _, line := range strings.Split(raw, "\n") {
		switch {
		case strings.TrimSpace(line) == "":
			// Blank separators carry no frame.
		case strings.HasPrefix(line, "\t"):
			location := strings.TrimSpace(line)
			// Strip the pc offset: "file.go:42 +0x1d" → "file.go:42".
			if plus := strings.Index(location, " +0x"); plus >= 0 {
				location = location[:plus]
			}
			if current < 0 {
				continue // a location with no owning goroutine header
			}
			if locationNext && len(snapshots[current].Frames) > 0 {
				snapshots[current].Frames[len(snapshots[current].Frames)-1].Location = location
				locationNext = false
				continue
			}
			snapshots[current].CreatedAt = location
		case strings.HasPrefix(line, "created by "):
			if current >= 0 {
				snapshots[current].CreatedBy = strings.TrimPrefix(line, "created by ")
			}
		default:
			if id, state, wait, isHeader := goroutineHeader(line); isHeader {
				snapshots = append(snapshots, GoroutineSnapshot{
					ID:    id,
					State: state,
					Wait:  wait,
				})
				current = len(snapshots) - 1
				continue
			}
			if current < 0 || strings.HasPrefix(line, "goroutine ") {
				// Noise before the first header, or a runtime note that
				// starts like a header but is not one ("goroutine running
				// on other thread; stack unavailable") — skipped, never
				// mistaken for a frame.
				continue
			}
			// A function line: its argument list stays, exactly as the
			// runtime prints it and `go tool pprof` would show it.
			snapshots[current].Frames = append(snapshots[current].Frames, GoroutineFrame{
				Function: line,
			})
			locationNext = true
		}
	}
	return snapshots
}

// DebugStallBlocker is the injectable wedge: it receives the release
// channel and must block until release closes. The production blocker
// is debugStallWedge below; tests inject their own to make the stall's
// lifecycle fully deterministic.
type DebugStallBlocker func(release <-chan struct{})

// debugStallWedge is the production wedge: a bare channel receive on a
// channel nobody sends to. Its name is deliberately distinctive — the
// finder locates the wedged goroutine in the captured profile by
// matching "debugStallWedge" on its own stack, so the function's name
// IS the marker. Rename it and the demo's finder must move with it.
func debugStallWedge(release <-chan struct{}) {
	<-release
}

// DebugStallRun is everything one stall run observed, honestly: the
// worker counts, the raw profile text exactly as captured, the
// capture's cost and attempt count, the parsed snapshot of the wedged
// goroutine (nil only if the finder failed — surfaced, never hidden),
// the contrast snapshot from the same capture, and whether every
// worker's exit was observed within the bound.
type DebugStallRun struct {
	Workers       int
	HealthyDone   int
	CaptureRaw    string
	CaptureMs     float64
	CaptureTries  int
	GoroutineSeen int
	Stalled       *GoroutineSnapshot
	Contrast      *GoroutineSnapshot
	AllExited     bool
}

// RunDebugStall choreographs the whole demo for real. The blocker is
// the wedge (nil means the production debugStallWedge); the marker
// names a function on the blocker's stack and is what the finder
// locates the wedged goroutine by — pass the injection's function
// name, and the production default when the blocker is nil. The
// lifecycle: start the worker set (four healthy, one wedged through
// the blocker) → rendezvous with every worker once, so all are
// provably scheduled → wait bounded for the healthy workers to finish
// → capture this process's own goroutine profile, retrying briefly
// until the wedge is visible on it (a just-scheduled goroutine can be
// runnable for a scheduling instant; the retry makes "the wedge is on
// the profile" a guarantee, not a hope) → parse and find the stalled
// goroutine by its marker plus a healthy contrast from the same
// capture → release the wedge and wait bounded for every worker's
// exit. The release runs before any return past it, so no failure
// path leaks the wedged goroutine.
func RunDebugStall(blocker DebugStallBlocker, marker string) DebugStallRun {
	if blocker == nil {
		blocker = debugStallWedge
		marker = "debugStallWedge"
	}
	run := DebugStallRun{Workers: DebugStallWorkers}

	release := make(chan struct{})
	// started is buffered to the full worker set so a worker can signal
	// entry without waiting to be received — even a pathological run
	// that stops draining early strands nobody on the send.
	started := make(chan struct{}, DebugStallWorkers)
	done := make(chan struct{}, DebugStallWorkers)

	for worker := 0; worker < DebugStallWorkers; worker++ {
		go func(wedged bool) {
			defer func() { done <- struct{}{} }()
			started <- struct{}{}
			if wedged {
				blocker(release)
				return
			}
			debugStallWork()
		}(worker == DebugStallWorkers-1)
	}

	// Rendezvous with every worker once — the wedge signals entry
	// before it blocks, so the capture below sees a live wedge.
	settleBound := time.NewTimer(DebugStallSettle)
rendezvous:
	for signals := 0; signals < DebugStallWorkers; signals++ {
		select {
		case <-started:
		case <-settleBound.C:
			break rendezvous // unreachable in practice; the bound protects liveness
		}
	}

	// Healthy workers complete in microseconds; drain bounded.
	for remaining := DebugStallWorkers - 1; remaining > 0; {
		select {
		case <-done:
			remaining--
			run.HealthyDone++
		case <-time.After(DebugStallSettle):
			remaining = 0 // surfaced honestly below as a shortfall
		}
	}

	// Capture our own goroutine profile until the wedge is visible on
	// it — normally the first capture, retried only for the
	// scheduler's settling instant.
	deadline := time.Now().Add(DebugStallSettle)
	for {
		var raw strings.Builder
		captureStarted := time.Now()
		if profile := pprof.Lookup("goroutine"); profile != nil {
			_ = profile.WriteTo(&raw, 2)
		}
		run.CaptureTries++
		run.CaptureMs = float64(time.Since(captureStarted).Microseconds()) / 1000
		run.CaptureRaw = raw.String()

		snapshots := ParseGoroutineProfile(run.CaptureRaw)
		run.GoroutineSeen = len(snapshots)
		for i := range snapshots {
			if run.Stalled == nil && snapshots[i].Matches(marker) {
				wedged := snapshots[i]
				run.Stalled = &wedged
				break
			}
		}
		if run.Stalled != nil {
			// The contrast is the REQUEST's own goroutine from the same
			// capture — runnable, executing the capture itself. The
			// wedged worker's stack also carries "RunDebugStall" (its
			// closure), so the finder excludes the stalled id.
			for i := range snapshots {
				if snapshots[i].ID != run.Stalled.ID && snapshots[i].Matches("RunDebugStall") {
					working := snapshots[i]
					run.Contrast = &working
					break
				}
			}
			break
		}
		if time.Now().After(deadline) {
			break
		}
		time.Sleep(2 * time.Millisecond)
	}

	// Release the wedge deterministically and wait bounded until every
	// worker's exit has been observed (healthy exits are usually
	// already counted above; the wedge's lands here). AllExited states
	// what was actually observed — never a promise the bound bailed on.
	close(release)
	exited := run.HealthyDone
	drainBound := time.NewTimer(DebugStallSettle)
drain:
	for exited < run.Workers {
		select {
		case <-done:
			exited++
		case <-drainBound.C:
			break drain
		}
	}
	run.AllExited = exited == run.Workers
	return run
}

// debugStallWork is the healthy workers' tiny bounded workload — a
// fixed-cost loop that completes in microseconds, holds no locks, and
// touches no channels beyond its own; it exists so the worker set is
// real goroutines doing real (trivial) work rather than no-ops.
func debugStallWork() {
	acc := 0
	for i := 0; i < 1000; i++ {
		acc += i % 7
	}
	_ = acc
}

// DebugStallEndpoint describes one of the live pprof endpoints the
// topic's wiring has served under /internal/ since topic 33 — the
// pointer the payload gives the reader for their own exploration.
type DebugStallEndpoint struct {
	Path    string `json:"path"`
	Profile string `json:"profile"`
	Note    string `json:"note"`
}

// DebugStallEndpoints is the live surface as the composition root
// mounts it: pprof.Index serves every named profile under
// /debug/pprof/, and the four explicit handlers (cmdline, profile,
// symbol, trace) round out the set.
var DebugStallEndpoints = []DebugStallEndpoint{
	{
		Path:    "/internal/debug/pprof/goroutine?debug=2",
		Profile: "goroutine",
		Note: "every live goroutine with its full stack — the exact text " +
			"this demo captured from its own process, served for yours",
	},
	{
		Path:    "/internal/debug/pprof/profile?seconds=2",
		Profile: "cpu",
		Note: "a CPU profile sampled over the next N seconds — the " +
			"endpoint blocks for the window, then serves the profile",
	},
	{
		Path:    "/internal/debug/pprof/heap",
		Profile: "heap",
		Note: "a heap snapshot: live allocations by size and site, the " +
			"inuse view `go tool pprof -top` reads by default",
	},
	{
		Path:    "/internal/debug/pprof/trace?seconds=1",
		Profile: "execution trace",
		Note: "the execution tracer's capture — scheduling, GC pauses, " +
			"goroutine lifetimes, the dimensions pprof cannot show",
	},
}

// DebugStallStackRow is the wire view of one goroutine's anatomy: its
// id and state, the frames in stack order (function + source
// location), and the created-by provenance when the runtime reports
// it. Frames is always non-nil — the client's DTO rejects null arrays.
type DebugStallStackRow struct {
	ID        int              `json:"id"`
	State     string           `json:"state"`
	Wait      string           `json:"wait,omitempty"`
	Frames    []GoroutineFrame `json:"frames"`
	CreatedBy string           `json:"createdBy,omitempty"`
	CreatedAt string           `json:"createdAt,omitempty"`
}

// stackRowFrom converts a parsed snapshot to its wire row; a nil
// snapshot keeps the zero row (the builder marks it honestly).
func stackRowFrom(snapshot *GoroutineSnapshot) DebugStallStackRow {
	row := DebugStallStackRow{Frames: []GoroutineFrame{}}
	if snapshot == nil {
		return row
	}
	row.ID = snapshot.ID
	row.State = snapshot.State
	row.Wait = snapshot.Wait
	row.CreatedBy = snapshot.CreatedBy
	row.CreatedAt = snapshot.CreatedAt
	row.Frames = append(row.Frames, snapshot.Frames...)
	return row
}

// DebugStallPayload is the wire shape of GET /api/demo/debug/stall.
type DebugStallPayload struct {
	Mode      string               `json:"mode"`
	Workload  DebugStallWorkload   `json:"workload"`
	Capture   DebugStallCaptureRow `json:"capture"`
	Stalled   DebugStallStackRow   `json:"stalled"`
	Contrast  DebugStallStackRow   `json:"contrast"`
	Endpoints []DebugStallEndpoint `json:"endpoints"`
	Reasoning string               `json:"reasoning"`
}

// DebugStallWorkload states the choreography's facts: the worker set
// size, how many completed, and what the wedge actually is.
type DebugStallWorkload struct {
	Workers     int    `json:"workers"`
	HealthyDone int    `json:"healthyDone"`
	Wedge       string `json:"wedge"`
}

// DebugStallCaptureRow states the capture's provenance: the profile
// name, the parsed goroutine count, the capture cost, how many capture
// attempts the scheduler's settling needed, and the exact curl that
// reads the same profile for any process.
type DebugStallCaptureRow struct {
	Profile        string  `json:"profile"`
	GoroutinesSeen int     `json:"goroutinesSeen"`
	CaptureMs      float64 `json:"captureMs"`
	CaptureTries   int     `json:"captureTries"`
	Curl           string  `json:"curl"`
}

// BuildDebugStallPayload shapes the wire payload (pure): the workload
// facts, the capture's provenance, the stalled stack and its healthy
// contrast as captured, and the pointer to the live endpoints. A
// missing snapshot is surfaced as an empty-state row with the honest
// marker "(not found)" — the payload never fabricates a stack.
func BuildDebugStallPayload(run DebugStallRun) DebugStallPayload {
	stalled := stackRowFrom(run.Stalled)
	if run.Stalled == nil {
		stalled.State = "(not found)"
	}
	contrast := stackRowFrom(run.Contrast)
	if run.Contrast == nil {
		contrast.State = "(not found)"
	}

	return DebugStallPayload{
		Mode: "runtime-derived: the stalled stack below was captured from " +
			"THIS process's own goroutine profile while the wedge was " +
			"live, then parsed — no stack in this payload was typed by " +
			"hand, and the wedge was released before the request returned",
		Workload: DebugStallWorkload{
			Workers:     run.Workers,
			HealthyDone: run.HealthyDone,
			Wedge: "one worker of the set blocks forever on a channel " +
				"receive nobody serves (debugStallWedge's <-release) — the " +
				"other workers complete their tiny work and exit",
		},
		Capture: DebugStallCaptureRow{
			Profile:        "goroutine",
			GoroutinesSeen: run.GoroutineSeen,
			CaptureMs:      run.CaptureMs,
			CaptureTries:   run.CaptureTries,
			Curl:           "curl -s 'localhost:8100/internal/debug/pprof/goroutine?debug=2'",
		},
		Stalled:  stalled,
		Contrast: contrast,
		Endpoints: append([]DebugStallEndpoint(nil),
			DebugStallEndpoints...),
		Reasoning: "The wedge was real: one goroutine sat in a channel " +
			"receive while this response was being built, and the runtime's " +
			"own profile names it — id, state, frames, and the " +
			"created-by chain that spawned it. The contrast row is the " +
			"request's own goroutine from the SAME capture: runnable, " +
			"executing the capture itself — a working goroutine and a " +
			"wedged one differ in exactly the way these two rows do. The " +
			"endpoints below serve the same profiles live for any process " +
			"you are debugging",
	}
}
