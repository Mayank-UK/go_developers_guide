package demos

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

// The choreography runs for real on every build; these tests pin the facts
// the go-runtime topic's prose claims, so a change that breaks the
// choreography's honesty fails here first, next to the claim. Deterministic
// beats (setup, fan-in rows, the cancellation ledger, the rigged races)
// must be identical across builds; observed facts (per-worker tallies,
// runtime numbers) are only sanity-checked — their whole point is that a
// real scheduler picks its own split.

func buildConcurrency(t *testing.T) ConcurrencyPayload {
	t.Helper()
	return BuildConcurrencyPayload(context.Background())
}

func TestSetupMatchesTheDocumentedChoreography(t *testing.T) {
	setup := buildConcurrency(t).Setup
	if setup.Jobs != poolJobs || setup.Workers != poolWorkers || setup.CancelAfter != cancelAfter {
		t.Errorf("setup = %+v, want %d jobs / %d workers / cancel after %d", setup, poolJobs, poolWorkers, cancelAfter)
	}
	if len(poolPayloads) != poolJobs {
		t.Errorf("poolPayloads has %d entries, want the configured %d", len(poolPayloads), poolJobs)
	}
	if strings.TrimSpace(setup.Strategy) == "" {
		t.Error("setup strategy must describe the choreography")
	}
}

func TestFanOutCollectsEveryJobDeterministically(t *testing.T) {
	rows := buildConcurrency(t).FanOut
	if len(rows) != poolJobs {
		t.Fatalf("fan-out rows = %d, want %d", len(rows), poolJobs)
	}
	for i, row := range rows {
		wantID := fmt.Sprintf("job-%02d", i+1)
		if row.JobID != wantID {
			t.Errorf("row %d id = %q, want %q (sorted, zero-padded)", i, row.JobID, wantID)
		}
		if row.Payload != poolPayloads[i] {
			t.Errorf("row %d payload = %q, want %q", i, row.Payload, poolPayloads[i])
		}
		if want := transform(row.Payload); row.Output != want {
			t.Errorf("row %s output = %q, want the pure transform %q", row.JobID, row.Output, want)
		}
		if !strings.Contains(row.Output, "#") || row.Output == row.Payload {
			t.Errorf("row %s output %q does not look transformed", row.JobID, row.Output)
		}
	}
}

func TestWorkerCountsAccountForEveryJob(t *testing.T) {
	rows := buildConcurrency(t).WorkerCounts
	if len(rows) != poolWorkers {
		t.Fatalf("worker rows = %d, want %d", len(rows), poolWorkers)
	}
	total := 0
	for i, row := range rows {
		wantID := fmt.Sprintf("worker-%d", i+1)
		if row.WorkerID != wantID {
			t.Errorf("worker row %d id = %q, want %q (sorted)", i, row.WorkerID, wantID)
		}
		if row.JobsDone < 0 {
			t.Errorf("worker %s handled %d jobs, negative is impossible", row.WorkerID, row.JobsDone)
		}
		total += row.JobsDone
	}
	if total != poolJobs {
		t.Errorf("per-worker tally sums to %d, want every one of the %d jobs", total, poolJobs)
	}
}

func TestConcurrencyCancellationLedgerBalances(t *testing.T) {
	beat := buildConcurrency(t).Cancellation
	if beat.JobsSubmitted != poolJobs {
		t.Errorf("submitted = %d, want %d", beat.JobsSubmitted, poolJobs)
	}
	if beat.ResultsCollected != cancelAfter {
		t.Errorf("collected = %d, want the configured trigger %d", beat.ResultsCollected, cancelAfter)
	}
	if beat.JobsAbandoned != beat.JobsSubmitted-beat.ResultsCollected {
		t.Errorf("abandoned = %d, want submitted−collected = %d", beat.JobsAbandoned, beat.JobsSubmitted-beat.ResultsCollected)
	}
	if beat.WorkersStopped != poolWorkers {
		t.Errorf("workers stopped = %d, want all %d joined", beat.WorkersStopped, poolWorkers)
	}
	if !strings.Contains(beat.Outcome, "abandoned") || !strings.Contains(beat.Outcome, "workers returned") {
		t.Errorf("outcome does not tell the story: %q", beat.Outcome)
	}
}

func TestTimeoutRacesLandAsRigged(t *testing.T) {
	beats := buildConcurrency(t).Timeouts
	if len(beats) != 2 {
		t.Fatalf("timeout beats = %d, want 2", len(beats))
	}
	prompt, late := beats[0], beats[1]
	if prompt.ProducerDelayMs != 0 || prompt.Winner != "reply" || prompt.Received != "pong" {
		t.Errorf("prompt beat = %+v, want delay 0 / reply wins / pong", prompt)
	}
	if late.ProducerDelayMs != int(lateReplyDelay.Milliseconds()) || late.TimeoutMs != int(replyTimeout.Milliseconds()) {
		t.Errorf("late beat delays = %+v, want %dms producer vs %dms timer",
			late, lateReplyDelay.Milliseconds(), replyTimeout.Milliseconds())
	}
	if late.Winner != "timer" || late.Received != "" {
		t.Errorf("late beat = %+v, want timer wins with empty received (data, not absence)", late)
	}
}

func TestRuntimeFactsBracketThePool(t *testing.T) {
	facts := buildConcurrency(t).Runtime
	if facts.GOMAXPROCS < 1 {
		t.Errorf("GOMAXPROCS = %d, want at least 1", facts.GOMAXPROCS)
	}
	if facts.GoroutinesDuring <= facts.GoroutinesBefore {
		t.Errorf("goroutines during = %d, want parked workers to raise it above before = %d",
			facts.GoroutinesDuring, facts.GoroutinesBefore)
	}
	if facts.GoroutinesAfter >= facts.GoroutinesDuring {
		t.Errorf("goroutines after = %d, want joined workers to drop it below during = %d",
			facts.GoroutinesAfter, facts.GoroutinesDuring)
	}
}

func TestDeterministicBeatsAreStableAcrossBuilds(t *testing.T) {
	first, second := buildConcurrency(t), buildConcurrency(t)
	if !reflect.DeepEqual(first.Setup, second.Setup) {
		t.Error("setup drifted between builds")
	}
	if !reflect.DeepEqual(first.FanOut, second.FanOut) {
		t.Error("fan-out rows drifted between builds; the transform or ordering broke")
	}
	if !reflect.DeepEqual(first.Cancellation, second.Cancellation) {
		t.Error("cancellation ledger drifted between builds")
	}
	if !reflect.DeepEqual(first.Timeouts, second.Timeouts) {
		t.Error("timeout races drifted between builds; the rigging broke")
	}
	// WorkerCounts and Runtime are observed facts by design — only their
	// shape is pinned, by the tests above.
}
