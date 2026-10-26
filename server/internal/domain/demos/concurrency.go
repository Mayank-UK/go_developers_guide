// Topic 02 go-runtime's live demo: a REAL worker-pool fan-out/fan-in
// choreography — goroutines, channels, select, and context cancellation —
// performed on every request. The go-runtime topic quotes these very
// functions; the payload below is their honest transcript. Counts, ids,
// and outcomes are deterministic by construction (the transform is pure,
// the cancellation trigger is a counted result, the timeout races are
// rigged by configuration); only the per-worker tally and the runtime
// facts are observed, because a real scheduler distributes real work its
// own way.
package demos

import (
	"context"
	"fmt"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
)

// The choreography's fixed shape — configuration, not observation. The
// cancellation beat always abandons the last three jobs (8 submitted − 5
// collected), and the late reply always loses its race (200ms ≫ 30ms), so
// the payload's counts and outcomes never vary between calls.
const (
	poolJobs       = 8
	poolWorkers    = 3
	cancelAfter    = 5
	replyTimeout   = 30 * time.Millisecond
	lateReplyDelay = 200 * time.Millisecond
)

// poolPayloads is the fixed batch of workloads the pool processes — NATO
// letters, in order, every run.
var poolPayloads = []string{"alpha", "bravo", "charlie", "delta", "echo", "foxtrot", "golf", "hotel"}

// poolJob is one unit of work traveling the jobs channel.
type poolJob struct {
	ID      string
	Payload string
}

// poolResult is one completed job traveling the results channel; it echoes
// the payload so the fan-in can report each job's full journey.
type poolResult struct {
	JobID   string
	Payload string
	Output  string
}

// transform is the deterministic work each job performs: uppercase the
// payload and fold its bytes into a sum. Same input, same output, every
// run — which is what makes the fan-in transcript stable.
func transform(payload string) string {
	sum := 0
	for _, b := range []byte(payload) {
		sum += int(b)
	}
	return fmt.Sprintf("%s#%d", strings.ToUpper(payload), sum)
}

// JobRow is one job's journey through the pool, exactly as the fan-in
// collected it (sorted by job id — completion order is scheduler
// business, not content).
type JobRow struct {
	JobID   string `json:"jobId"`
	Payload string `json:"payload"`
	Output  string `json:"output"`
}

// WorkerRow is the observed per-worker tally — the one fan-out fact that
// genuinely varies run to run, because real workers race for real jobs.
type WorkerRow struct {
	WorkerID string `json:"workerId"`
	JobsDone int    `json:"jobsDone"`
}

// PoolSetup states the choreography's configuration.
type PoolSetup struct {
	Jobs        int    `json:"jobs"`
	Workers     int    `json:"workers"`
	CancelAfter int    `json:"cancelAfter"`
	Strategy    string `json:"strategy"`
}

// CancelBeat is the context-cancellation run: the collector cancels after
// a counted number of results, and the ledger must always balance —
// collected + abandoned == submitted.
type CancelBeat struct {
	JobsSubmitted    int    `json:"jobsSubmitted"`
	ResultsCollected int    `json:"resultsCollected"`
	JobsAbandoned    int    `json:"jobsAbandoned"`
	WorkersStopped   int    `json:"workersStopped"`
	Outcome          string `json:"outcome"`
}

// TimeoutBeat is one select race between a reply and a timer. Received is
// "" when the timer wins — empty is data there, not absence.
type TimeoutBeat struct {
	Scenario        string `json:"scenario"`
	ProducerDelayMs int    `json:"producerDelayMs"`
	TimeoutMs       int    `json:"timeoutMs"`
	Winner          string `json:"winner"`
	Received        string `json:"received"`
}

// RuntimeFacts carries what the runtime reports about itself — observed,
// not asserted: GOMAXPROCS follows the machine (or, since Go 1.25, the
// container's CPU quota), and the goroutine counts bracket the pool's
// life from parked workers to joined ones.
type RuntimeFacts struct {
	GOMAXPROCS       int `json:"gomaxprocs"`
	GoroutinesBefore int `json:"goroutinesBefore"`
	GoroutinesDuring int `json:"goroutinesDuring"`
	GoroutinesAfter  int `json:"goroutinesAfter"`
}

// ConcurrencyPayload is the wire shape of GET /api/demo/runtime/concurrency
// (camelCase JSON) — the go-runtime topic's inline demo.
type ConcurrencyPayload struct {
	Setup        PoolSetup     `json:"setup"`
	FanOut       []JobRow      `json:"fanOut"`
	WorkerCounts []WorkerRow   `json:"workerCounts"`
	Cancellation CancelBeat    `json:"cancellation"`
	Timeouts     []TimeoutBeat `json:"timeouts"`
	Runtime      RuntimeFacts  `json:"runtime"`
}

// poolWorker is one goroutine of the pool. Every value it touches travels
// a channel — jobs in, results and its own tally out — and both blocking
// operations are select-guarded on ctx, so cancellation unfailingly
// unparks it whether it is waiting for work or waiting to deliver.
func poolWorker(ctx context.Context, id int, jobs <-chan poolJob, results chan<- poolResult, stats chan<- WorkerRow) {
	worker := fmt.Sprintf("worker-%d", id)
	handled := 0
	defer func() { stats <- WorkerRow{WorkerID: worker, JobsDone: handled} }()
	for {
		select {
		case job, ok := <-jobs:
			if !ok {
				return
			}
			select {
			case results <- poolResult{JobID: job.ID, Payload: job.Payload, Output: transform(job.Payload)}:
				handled++
			case <-ctx.Done():
				return
			}
		case <-ctx.Done():
			return
		}
	}
}

// fanOutFanIn performs the complete run: three workers park on an empty
// jobs channel (that parked-alive window is the goroutine count the
// payload reports), a feeder streams the eight jobs in, and the fan-in
// collects exactly eight results before joining the workers.
func fanOutFanIn(ctx context.Context) (rows []JobRow, counts []WorkerRow, facts RuntimeFacts) {
	facts.GOMAXPROCS = runtime.GOMAXPROCS(0)
	facts.GoroutinesBefore = runtime.NumGoroutine()

	jobs := make(chan poolJob)
	results := make(chan poolResult)
	stats := make(chan WorkerRow, poolWorkers)
	var wg sync.WaitGroup
	for id := 1; id <= poolWorkers; id++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			poolWorker(ctx, id, jobs, results, stats)
		}()
	}
	facts.GoroutinesDuring = runtime.NumGoroutine()

	go func() {
		defer close(jobs)
		for i, payload := range poolPayloads {
			select {
			case jobs <- poolJob{ID: fmt.Sprintf("job-%02d", i+1), Payload: payload}:
			case <-ctx.Done():
				return
			}
		}
	}()

	rows = make([]JobRow, 0, poolJobs)
collect:
	for len(rows) < poolJobs {
		select {
		case r := <-results:
			rows = append(rows, JobRow{JobID: r.JobID, Payload: r.Payload, Output: r.Output})
		case <-ctx.Done():
			// A cancelled caller: stop collecting. The joins below still
			// hold, so nothing leaks.
			break collect
		}
	}
	wg.Wait()
	sort.Slice(rows, func(i, j int) bool { return rows[i].JobID < rows[j].JobID })

	counts = make([]WorkerRow, 0, poolWorkers)
	for i := 0; i < poolWorkers; i++ {
		counts = append(counts, <-stats)
	}
	sort.Slice(counts, func(i, j int) bool { return counts[i].WorkerID < counts[j].WorkerID })
	facts.GoroutinesAfter = runtime.NumGoroutine()
	return rows, counts, facts
}

// cancellationBeat replays the pool with the escape hatch armed: all
// eight jobs are submitted up front (a buffered channel), and the
// collector cancels the context the moment the fifth result lands.
// Workers stop draining, in-flight sends abort through their select's ctx
// arm, and the ledger must balance: collected + abandoned == submitted.
func cancellationBeat(ctx context.Context) CancelBeat {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	jobs := make(chan poolJob, poolJobs)
	for i, payload := range poolPayloads {
		jobs <- poolJob{ID: fmt.Sprintf("job-%02d", i+1), Payload: payload}
	}
	// Deliberately NOT closed: cancellation, not closure, ends this run.

	results := make(chan poolResult)
	stats := make(chan WorkerRow, poolWorkers)
	var wg sync.WaitGroup
	for id := 1; id <= poolWorkers; id++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			poolWorker(ctx, id, jobs, results, stats)
		}()
	}

	collected := 0
collect:
	for collected < cancelAfter {
		select {
		case <-results:
			collected++
		case <-ctx.Done():
			break collect
		}
	}
	cancel() // the counted result IS the trigger

	wg.Wait()
	stopped := 0
	for i := 0; i < poolWorkers; i++ {
		<-stats
		stopped++
	}
	return CancelBeat{
		JobsSubmitted:    poolJobs,
		ResultsCollected: collected,
		JobsAbandoned:    poolJobs - collected,
		WorkersStopped:   stopped,
		Outcome: fmt.Sprintf(
			"context cancelled after %d of %d results; %d jobs abandoned, %d workers returned",
			collected, poolJobs, poolJobs-collected, stopped,
		),
	}
}

// raceReplyVsTimeout runs one select race between a reply channel and a
// timer. With zero delay the reply sits in the buffer BEFORE the select
// starts — a ready case wins instantly, which is the deterministic point
// being demonstrated; with a real delay the producer goroutine loses by
// configuration (its sleep dwarfs the timer), never by luck.
func raceReplyVsTimeout(delay, timeout time.Duration, scenario string) TimeoutBeat {
	reply := make(chan string, 1) // buffered: the late producer never leaks, timer or not
	if delay == 0 {
		reply <- "pong"
	} else {
		go func() {
			timer := time.NewTimer(delay)
			defer timer.Stop()
			<-timer.C
			reply <- "pong"
		}()
	}

	beat := TimeoutBeat{
		Scenario:        scenario,
		ProducerDelayMs: int(delay.Milliseconds()),
		TimeoutMs:       int(timeout.Milliseconds()),
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case received := <-reply:
		beat.Winner, beat.Received = "reply", received
	case <-timer.C:
		beat.Winner, beat.Received = "timer", ""
	}
	return beat
}

// BuildConcurrencyPayload assembles the whole demo payload by RUNNING the
// choreography: one complete fan-out/fan-in pass, one mid-stream
// cancellation, two rigged select races, and the runtime's own facts. It
// blocks for roughly one reply timeout — bounded work, with ctx honored
// throughout the pool runs.
func BuildConcurrencyPayload(ctx context.Context) ConcurrencyPayload {
	if ctx == nil {
		ctx = context.Background()
	}
	rows, counts, facts := fanOutFanIn(ctx)
	return ConcurrencyPayload{
		Setup: PoolSetup{
			Jobs:        poolJobs,
			Workers:     poolWorkers,
			CancelAfter: cancelAfter,
			Strategy:    "worker pool: fan-out over one jobs channel, fan-in over one results channel, cancellation via select",
		},
		FanOut:       rows,
		WorkerCounts: counts,
		Cancellation: cancellationBeat(ctx),
		Timeouts: []TimeoutBeat{
			raceReplyVsTimeout(0, replyTimeout, "reply already waiting"),
			raceReplyVsTimeout(lateReplyDelay, replyTimeout, "reply arrives late"),
		},
		Runtime: facts,
	}
}
