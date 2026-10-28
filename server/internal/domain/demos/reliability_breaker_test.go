package demos

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"

	"guide/internal/core"
)

// The reliability breaker's machinery, pinned from both sides: the
// breaker state machine on an injected clock (threshold, cooldown,
// half-open single-probe semantics, reopen-on-failed-probe, race
// safety under real concurrency), and the console's live walks on
// bounded real runs with a tiny config — the scenario store drives
// trips, deadline conversions, and recoveries end to end. The pure
// builder's honesty is pinned too: measured durations ride the wire
// exactly as measured.

// fakeReliabilityClock is a manually-advanced clock for the breaker's
// state machine tests — deterministic cooldown math, no sleeps.
type fakeReliabilityClock struct{ now time.Time }

func (c *fakeReliabilityClock) Now() time.Time          { return c.now }
func (c *fakeReliabilityClock) Advance(d time.Duration) { c.now = c.now.Add(d) }

// reliabilityTestConfig is the tiny bounded-real-run config: every
// effect is real but fast.
func reliabilityTestConfig() ReliabilityConfig {
	return ReliabilityConfig{
		FailureThreshold: 2,
		Cooldown:         40 * time.Millisecond,
		CallTimeout:      8 * time.Millisecond,
		WalkAttempts:     4,
		RetryAttempts:    3,
		RetryBaseDelay:   2 * time.Millisecond,
		RetryJitter:      1 * time.Millisecond,
	}
}

// newReliabilityTestStore builds a store armed with one mode.
func newReliabilityTestStore(t *testing.T, mode core.FailureInjection) *core.ScenarioStore {
	t.Helper()
	store := core.NewScenarioStore()
	if _, err := store.Update(core.ScenarioPatch{InjectFailure: &mode}); err != nil {
		t.Fatalf("arm scenario %s: %v", mode, err)
	}
	return store
}

// armReliability switches the store's injection mode.
func armReliability(t *testing.T, store *core.ScenarioStore, mode core.FailureInjection) {
	t.Helper()
	if _, err := store.Update(core.ScenarioPatch{InjectFailure: &mode}); err != nil {
		t.Fatalf("switch scenario to %s: %v", mode, err)
	}
}

func TestReliabilityBreakerOpensAtThresholdThenFastFails(t *testing.T) {
	clock := &fakeReliabilityClock{now: time.Date(2026, 10, 28, 9, 0, 0, 0, time.UTC)}
	breaker := NewReliabilityBreaker(3, 2*time.Second, clock.Now)

	// Two failures below the threshold stay closed.
	for i := 0; i < 2; i++ {
		if decision := breaker.Allow(); !decision.Allowed || decision.State != ReliabilityBreakerClosed {
			t.Fatalf("attempt %d decision = %+v, want allowed closed", i+1, decision)
		}
		breaker.OnFailure()
	}
	if state := breaker.State(); state != ReliabilityBreakerClosed {
		t.Fatalf("state after 2 of 3 failures = %q, want closed", state)
	}

	// The third failure trips the breaker open.
	if decision := breaker.Allow(); !decision.Allowed {
		t.Fatalf("third call decision = %+v, want allowed", decision)
	}
	breaker.OnFailure()
	if state := breaker.State(); state != ReliabilityBreakerOpen {
		t.Fatalf("state after threshold = %q, want open", state)
	}

	// While the cooldown runs, calls fast-fail with the remaining time.
	clock.Advance(500 * time.Millisecond)
	refusal := breaker.Allow()
	if refusal.Allowed || refusal.State != ReliabilityBreakerOpen {
		t.Fatalf("open decision = %+v, want a fast-fail refusal", refusal)
	}
	if !strings.Contains(refusal.Reason, "1.5s") {
		t.Errorf("refusal reason = %q, want the remaining cooldown named", refusal.Reason)
	}

	snapshot := breaker.Snapshot(clock.Now())
	if snapshot.Counters.Allowed != 3 || snapshot.Counters.FastFailed != 1 || snapshot.Counters.Failed != 3 {
		t.Errorf("counters = %+v, want 3 allowed / 1 fast-failed / 3 failed", snapshot.Counters)
	}
	if len(snapshot.Transitions) != 1 {
		t.Fatalf("transitions = %+v, want exactly the trip", snapshot.Transitions)
	}
	trip := snapshot.Transitions[0]
	if trip.From != ReliabilityBreakerClosed || trip.To != ReliabilityBreakerOpen ||
		!strings.Contains(trip.Cause, "threshold") {
		t.Errorf("trip transition = %+v", trip)
	}
}

func TestReliabilityBreakerHalfOpenProbeClosesOnSuccess(t *testing.T) {
	clock := &fakeReliabilityClock{now: time.Date(2026, 10, 28, 9, 0, 0, 0, time.UTC)}
	breaker := NewReliabilityBreaker(2, 2*time.Second, clock.Now)

	breaker.Allow()
	breaker.OnFailure() // 1 of 2: stays closed.
	breaker.Allow()
	breaker.OnFailure() // 2 of 2: trips open.
	if breaker.State() != ReliabilityBreakerOpen {
		t.Fatal("breaker must be open before the cooldown test")
	}

	// Exactly at cooldown expiry the next caller becomes the probe.
	clock.Advance(2 * time.Second)
	probe := breaker.Allow()
	if !probe.Allowed || probe.State != ReliabilityBreakerHalfOpen {
		t.Fatalf("post-cooldown decision = %+v, want the half-open probe", probe)
	}
	if state := breaker.State(); state != ReliabilityBreakerHalfOpen {
		t.Fatalf("state mid-probe = %q, want half-open", state)
	}

	// A successful probe closes the circuit and resets the streak.
	breaker.OnSuccess()
	if state := breaker.State(); state != ReliabilityBreakerClosed {
		t.Fatalf("state after successful probe = %q, want closed", state)
	}
	snapshot := breaker.Snapshot(clock.Now())
	if len(snapshot.Transitions) != 3 {
		t.Fatalf("transitions = %+v, want closed→open, open→half-open, half-open→closed", snapshot.Transitions)
	}
	last := snapshot.Transitions[len(snapshot.Transitions)-1]
	if last.From != ReliabilityBreakerHalfOpen || last.To != ReliabilityBreakerClosed {
		t.Errorf("recovery transition = %+v", last)
	}
	// And the next failure needs the FULL streak again, not one.
	breaker.OnFailure()
	if state := breaker.State(); state != ReliabilityBreakerClosed {
		t.Errorf("state after one post-recovery failure = %q, want closed (streak was reset)", state)
	}
}

func TestReliabilityBreakerHalfOpenProbeFailureReopens(t *testing.T) {
	clock := &fakeReliabilityClock{now: time.Date(2026, 10, 28, 9, 0, 0, 0, time.UTC)}
	breaker := NewReliabilityBreaker(1, time.Second, clock.Now)

	breaker.Allow()
	breaker.OnFailure()
	clock.Advance(time.Second)
	if probe := breaker.Allow(); !probe.Allowed {
		t.Fatal("the post-cooldown call must be the probe")
	}
	breaker.OnFailure() // the probe fails: reopen for a FRESH cooldown.

	if state := breaker.State(); state != ReliabilityBreakerOpen {
		t.Fatalf("state after failed probe = %q, want open", state)
	}
	// The fresh cooldown must not admit another probe early.
	clock.Advance(900 * time.Millisecond)
	if decision := breaker.Allow(); decision.Allowed {
		t.Fatal("decision inside the fresh cooldown must fast-fail")
	}
	clock.Advance(101 * time.Millisecond)
	if decision := breaker.Allow(); !decision.Allowed || decision.State != ReliabilityBreakerHalfOpen {
		t.Fatalf("decision after the fresh cooldown = %+v, want the next probe", decision)
	}
}

func TestReliabilityBreakerHalfOpenAdmitsOneProbeAtATime(t *testing.T) {
	clock := &fakeReliabilityClock{now: time.Date(2026, 10, 28, 9, 0, 0, 0, time.UTC)}
	breaker := NewReliabilityBreaker(1, time.Second, clock.Now)

	breaker.Allow()
	breaker.OnFailure()
	clock.Advance(time.Second)

	first := breaker.Allow()
	if !first.Allowed {
		t.Fatal("the first post-cooldown call must be the probe")
	}
	second := breaker.Allow()
	if second.Allowed || second.State != ReliabilityBreakerHalfOpen {
		t.Fatalf("second decision while the probe is in flight = %+v, want a refusal", second)
	}
	if !strings.Contains(second.Reason, "probe is already in flight") {
		t.Errorf("refusal reason = %q", second.Reason)
	}
}

// TestReliabilityBreakerConcurrentAccessIsRaceClean hammers the shared
// breaker from many goroutines at once — the payload's claim that the
// breaker is genuinely shared state must survive the race detector.
func TestReliabilityBreakerConcurrentAccessIsRaceClean(t *testing.T) {
	breaker := NewReliabilityBreaker(5, 5*time.Millisecond, time.Now)
	var wg sync.WaitGroup
	for worker := 0; worker < 16; worker++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 50; i++ {
				if breaker.Allow().Allowed {
					if i%2 == 0 {
						breaker.OnSuccess()
					} else {
						breaker.OnFailure()
					}
				}
				_ = breaker.Snapshot(time.Now())
			}
		}()
	}
	wg.Wait()

	snapshot := breaker.Snapshot(time.Now())
	if snapshot.Counters.Allowed+snapshot.Counters.FastFailed != 16*50 {
		t.Errorf("counters = %+v, want every one of the %d decisions counted", snapshot.Counters, 16*50)
	}
}

func TestReliabilityBreakerConfigRejectsInvalidValues(t *testing.T) {
	store := core.NewScenarioStore()
	cases := []struct {
		name string
		cfg  ReliabilityConfig
		want string
	}{
		{"zero threshold", ReliabilityConfig{FailureThreshold: 0, Cooldown: time.Second,
			CallTimeout: time.Second, WalkAttempts: 4, RetryAttempts: 3,
			RetryBaseDelay: time.Millisecond}, "failure threshold"},
		{"zero cooldown", ReliabilityConfig{FailureThreshold: 3, Cooldown: 0,
			CallTimeout: time.Second, WalkAttempts: 4, RetryAttempts: 3,
			RetryBaseDelay: time.Millisecond}, "cooldown"},
		{"zero call timeout", ReliabilityConfig{FailureThreshold: 3, Cooldown: time.Second,
			CallTimeout: 0, WalkAttempts: 4, RetryAttempts: 3,
			RetryBaseDelay: time.Millisecond}, "call timeout"},
		{"no walk attempts", ReliabilityConfig{FailureThreshold: 3, Cooldown: time.Second,
			CallTimeout: time.Second, WalkAttempts: 0, RetryAttempts: 3,
			RetryBaseDelay: time.Millisecond}, "walk attempts"},
		{"too many walk attempts", ReliabilityConfig{FailureThreshold: 3, Cooldown: time.Second,
			CallTimeout: time.Second, WalkAttempts: 9, RetryAttempts: 3,
			RetryBaseDelay: time.Millisecond}, "walk attempts"},
		{"zero base delay", ReliabilityConfig{FailureThreshold: 3, Cooldown: time.Second,
			CallTimeout: time.Second, WalkAttempts: 4, RetryAttempts: 3,
			RetryBaseDelay: 0}, "base delay"},
	}
	for _, tc := range cases {
		if _, err := NewReliabilityConsole(tc.cfg, store); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: err = %v, want it to name %q", tc.name, err, tc.want)
		}
	}
	if _, err := NewReliabilityConsole(reliabilityTestConfig(), nil); err == nil || !strings.Contains(err.Error(), "scenario store") {
		t.Errorf("nil store: err = %v, want the nil-store refusal", err)
	}
	if _, err := NewReliabilityConsole(reliabilityTestConfig(), store); err != nil {
		t.Fatalf("valid config must build: %v", err)
	}
}

// TestReliabilityWalkTripsUnderInjectedError runs the real console
// against the armed scenario store: threshold failures open the
// breaker mid-walk and the remaining attempts fast-fail.
func TestReliabilityWalkTripsUnderInjectedError(t *testing.T) {
	cfg := reliabilityTestConfig()
	console, err := NewReliabilityConsole(cfg, newReliabilityTestStore(t, core.FailureError))
	if err != nil {
		t.Fatalf("console: %v", err)
	}

	walk := console.Walk(context.Background())
	if len(walk.Rows) != cfg.WalkAttempts {
		t.Fatalf("rows = %d, want %d", len(walk.Rows), cfg.WalkAttempts)
	}
	for i, row := range walk.Rows[:cfg.FailureThreshold] {
		if row.Action != reliabilityActionCall || row.Outcome != reliabilityOutcomeInjected {
			t.Errorf("row %d = %+v, want a called attempt failing with the injected error", i+1, row)
		}
		if row.Detail == "" || !strings.Contains(row.Detail, ErrReliabilityInjected.Error()) {
			t.Errorf("row %d must carry the real error message, got %q", i+1, row.Detail)
		}
	}
	for i, row := range walk.Rows[cfg.FailureThreshold:] {
		if row.Action != reliabilityActionFastFail || row.Outcome != reliabilityOutcomeRefused {
			t.Errorf("row %d = %+v, want a fast-fail refusal after the trip", cfg.FailureThreshold+i+1, row)
		}
	}
	if state := console.BreakerSnapshot(time.Now()).State; state != ReliabilityBreakerOpen {
		t.Errorf("state after the walk = %q, want open", state)
	}
	if !strings.Contains(DescribeReliabilityWalk(walk), "threshold tripped") {
		t.Errorf("walk note = %q", DescribeReliabilityWalk(walk))
	}
}

// TestReliabilityWalkTreatsLatencyAsDeadlineExceeded proves the
// teaching's sharpest edge: a stalled dependency becomes an error ONLY
// because the per-call deadline converts it — rows carry the deadline
// outcome with elapsed durations measured around the real wait.
func TestReliabilityWalkTreatsLatencyAsDeadlineExceeded(t *testing.T) {
	cfg := reliabilityTestConfig()
	console, err := NewReliabilityConsole(cfg, newReliabilityTestStore(t, core.FailureLatency))
	if err != nil {
		t.Fatalf("console: %v", err)
	}

	walk := console.Walk(context.Background())
	for i, row := range walk.Rows[:cfg.FailureThreshold] {
		if row.Outcome != reliabilityOutcomeDeadline {
			t.Errorf("row %d outcome = %q (%s), want deadline exceeded", i+1, row.Outcome, row.Detail)
		}
		if row.Elapsed < cfg.CallTimeout {
			t.Errorf("row %d elapsed = %s, want at least the per-call deadline %s", i+1, row.Elapsed, cfg.CallTimeout)
		}
	}
	if state := console.BreakerSnapshot(time.Now()).State; state != ReliabilityBreakerOpen {
		t.Errorf("latency must trip the breaker exactly like errors: state = %q", state)
	}
}

// TestReliabilityWalkRecoversAfterCooldown walks the full arc live:
// trip under error, outlast the cooldown, and watch the half-open
// probe close the circuit once the scenario is disarmed.
func TestReliabilityWalkRecoversAfterCooldown(t *testing.T) {
	cfg := reliabilityTestConfig()
	store := newReliabilityTestStore(t, core.FailureError)
	console, err := NewReliabilityConsole(cfg, store)
	if err != nil {
		t.Fatalf("console: %v", err)
	}

	console.Walk(context.Background()) // trips open.
	if state := console.BreakerSnapshot(time.Now()).State; state != ReliabilityBreakerOpen {
		t.Fatal("precondition: breaker must be open after the first walk")
	}

	armReliability(t, store, core.FailureNone)
	time.Sleep(cfg.Cooldown + 10*time.Millisecond) // outlast the cooldown for real

	recovered := console.Walk(context.Background())
	first := recovered.Rows[0]
	if first.StateBefore != ReliabilityBreakerHalfOpen || first.Outcome != reliabilityOutcomeSuccess {
		t.Errorf("recovery probe row = %+v, want a half-open probe that succeeds", first)
	}
	for i, row := range recovered.Rows[1:] {
		if row.StateBefore != ReliabilityBreakerClosed || row.Outcome != reliabilityOutcomeSuccess {
			t.Errorf("post-recovery row %d = %+v, want closed-and-succeeding", i+2, row)
		}
	}
	if state := console.BreakerSnapshot(time.Now()).State; state != ReliabilityBreakerClosed {
		t.Errorf("state after recovery = %q, want closed", state)
	}
	if !strings.Contains(DescribeReliabilityWalk(recovered), "recovery observed") {
		t.Errorf("walk note = %q", DescribeReliabilityWalk(recovered))
	}
}

// TestReliabilityRetryBackoffIsBoundedAndJittered pins the series'
// shape under error injection: every attempt fails, the waits grow by
// the doubling rung, jitter stays inside its bound, and the series is
// BOUNDED — it ends with the error surfaced, never looping forever.
func TestReliabilityRetryBackoffIsBoundedAndJittered(t *testing.T) {
	cfg := reliabilityTestConfig()
	console, err := NewReliabilityConsole(cfg, newReliabilityTestStore(t, core.FailureError))
	if err != nil {
		t.Fatalf("console: %v", err)
	}

	run := console.RetrySeries(context.Background())
	if run.Succeeded {
		t.Fatal("series must not succeed under error injection")
	}
	if len(run.Rows) != cfg.RetryAttempts {
		t.Fatalf("rows = %d, want all %d attempts", len(run.Rows), cfg.RetryAttempts)
	}
	for i, row := range run.Rows {
		if row.Outcome != reliabilityOutcomeInjected {
			t.Errorf("row %d outcome = %q, want the injected error", i+1, row.Outcome)
		}
		rung := cfg.RetryBaseDelay << i
		if row.Backoff == 0 && i < len(run.Rows)-1 {
			t.Errorf("row %d must carry its measured backoff", i+1)
		}
		if row.Backoff > 0 {
			max := rung + cfg.RetryJitter + 2*time.Millisecond
			if row.Backoff < rung-2*time.Millisecond || row.Backoff > max {
				t.Errorf("row %d backoff = %s, want rung %s + jitter inside [%s, %s]",
					i+1, row.Backoff, rung, rung, max)
			}
		}
	}
	if last := run.Rows[len(run.Rows)-1]; last.Backoff != 0 {
		t.Errorf("last row backoff = %s, want no wait after the final attempt", last.Backoff)
	}
	var waits time.Duration
	for _, row := range run.Rows {
		waits += row.Backoff
	}
	if run.TotalElapsed < waits {
		t.Errorf("total %s < summed waits %s — the total must be measured end to end", run.TotalElapsed, waits)
	}
	if !strings.Contains(DescribeReliabilityRetry(run), "exhausted") {
		t.Errorf("retry note = %q", DescribeReliabilityRetry(run))
	}
}

// TestReliabilityRetrySucceedsFirstAttemptUnderNone pins the healthy
// path: one attempt, no backoff waits, the series reports success.
func TestReliabilityRetrySucceedsFirstAttemptUnderNone(t *testing.T) {
	cfg := reliabilityTestConfig()
	console, err := NewReliabilityConsole(cfg, newReliabilityTestStore(t, core.FailureNone))
	if err != nil {
		t.Fatalf("console: %v", err)
	}

	run := console.RetrySeries(context.Background())
	if !run.Succeeded || len(run.Rows) != 1 {
		t.Fatalf("run = %+v, want first-attempt success", run)
	}
	if run.Rows[0].Backoff != 0 || run.Rows[0].Outcome != reliabilityOutcomeSuccess {
		t.Errorf("first row = %+v, want a clean success with no wait", run.Rows[0])
	}
	if !strings.Contains(DescribeReliabilityRetry(run), "succeeded on attempt 1") {
		t.Errorf("retry note = %q", DescribeReliabilityRetry(run))
	}
}

// TestReliabilityRetryStopsWhenContextEnds proves the waits are as
// cancellable as the calls: a context that is already done ends the
// series without sleeping out its backoff.
func TestReliabilityRetryStopsWhenContextEnds(t *testing.T) {
	cfg := reliabilityTestConfig()
	cfg.RetryBaseDelay = 50 * time.Millisecond
	console, err := NewReliabilityConsole(cfg, newReliabilityTestStore(t, core.FailureError))
	if err != nil {
		t.Fatalf("console: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	started := time.Now()
	run := console.RetrySeries(ctx)
	if time.Since(started) > cfg.RetryBaseDelay {
		t.Fatalf("series took %s with a done context — the waits must abort immediately", time.Since(started))
	}
	if run.Succeeded {
		t.Error("series must not report success under injection")
	}
	if len(run.Rows) > 1 {
		t.Errorf("rows = %d, want the series to stop at the first cancelled wait", len(run.Rows))
	}
}

// TestReliabilityBreakerPayloadCarriesOnlyMeasuredTimings is the
// demo's honesty pin: durations recorded by the machinery ride the
// wire EXACTLY as measured (microsecond-precision floats), never
// recomputed, rounded into new numbers, or invented — the same
// discipline as the performance topic's no-invented-numbers rule.
func TestReliabilityBreakerPayloadCarriesOnlyMeasuredTimings(t *testing.T) {
	at := time.Date(2026, 10, 28, 9, 30, 0, 0, time.UTC)
	facts := ReliabilityBreakerFacts{
		Scenario: core.ScenarioState{DataMode: core.DataModeFake, InjectFailure: core.FailureLatency},
		Breaker: ReliabilityBreakerSnapshotWithConfig{
			ReliabilityBreakerSnapshot: ReliabilityBreakerSnapshot{
				State:    ReliabilityBreakerOpen,
				StateAge: 1234 * time.Millisecond,
				Counters: ReliabilityBreakerCounters{Allowed: 7, FastFailed: 2, Succeeded: 4, Failed: 3},
				Transitions: []ReliabilityTransition{{
					At:    at,
					From:  ReliabilityBreakerClosed,
					To:    ReliabilityBreakerOpen,
					Cause: "3 consecutive failures — threshold reached",
				}},
			},
			Config: DefaultReliabilityConfig(),
		},
		Walk: ReliabilityWalk{
			Mode: core.FailureLatency,
			Rows: []ReliabilityAttemptRow{
				{Index: 1, StateBefore: ReliabilityBreakerClosed, Action: reliabilityActionCall,
					Outcome: reliabilityOutcomeDeadline, Detail: "deadline exceeded",
					Elapsed: 120450 * time.Microsecond, StateAfter: ReliabilityBreakerClosed},
				{Index: 2, StateBefore: ReliabilityBreakerOpen, Action: reliabilityActionFastFail,
					Outcome: reliabilityOutcomeRefused, Detail: "open — fast-fail",
					Elapsed: 3 * time.Microsecond, StateAfter: ReliabilityBreakerOpen},
			},
		},
		Retry: ReliabilityRetryRun{
			Mode: core.FailureLatency,
			Rows: []ReliabilityRetryRow{
				{Attempt: 1, Outcome: reliabilityOutcomeDeadline, Detail: "deadline exceeded",
					Elapsed: 120100 * time.Microsecond, Backoff: 41250 * time.Microsecond},
				{Attempt: 2, Outcome: reliabilityOutcomeDeadline, Detail: "deadline exceeded",
					Elapsed: 120300 * time.Microsecond, Backoff: 0},
			},
			TotalElapsed: 281800 * time.Microsecond,
		},
	}

	payload := BuildReliabilityBreakerPayload(facts)

	if payload.Breaker.State != ReliabilityBreakerOpen || payload.Breaker.StateAgeMs != 1234 {
		t.Errorf("breaker panel state/age = %q/%v, want open/1234", payload.Breaker.State, payload.Breaker.StateAgeMs)
	}
	if payload.Breaker.Config.CooldownMs != 2000 || payload.Breaker.Config.CallTimeoutMs != 120 ||
		payload.Breaker.Config.RetryBaseMs != 40 || payload.Breaker.Config.RetryJitterMs != 20 ||
		payload.Breaker.Config.FailureThreshold != 3 || payload.Breaker.Config.HalfOpenProbes != 1 {
		t.Errorf("config panel = %+v, want the real default constants", payload.Breaker.Config)
	}
	if len(payload.Breaker.Transitions) != 1 || !payload.Breaker.Transitions[0].At.Equal(at) {
		t.Errorf("transitions = %+v", payload.Breaker.Transitions)
	}

	if len(payload.Walk.Rows) != 2 {
		t.Fatalf("walk rows = %d", len(payload.Walk.Rows))
	}
	if payload.Walk.Rows[0].ElapsedMs != 120.45 || payload.Walk.Rows[1].ElapsedMs != 0.003 {
		t.Errorf("walk elapsed = %v/%v, want the measured 120.45/0.003 exactly",
			payload.Walk.Rows[0].ElapsedMs, payload.Walk.Rows[1].ElapsedMs)
	}
	if payload.Retry.Rows[0].BackoffMs != 41.25 || payload.Retry.Rows[1].BackoffMs != 0 {
		t.Errorf("retry backoffs = %v/%v, want the measured 41.25/0 exactly",
			payload.Retry.Rows[0].BackoffMs, payload.Retry.Rows[1].BackoffMs)
	}
	if payload.Retry.TotalElapsedMs != 281.8 {
		t.Errorf("retry total = %v, want 281.8", payload.Retry.TotalElapsedMs)
	}
	if payload.Scenario.Mode != string(core.FailureLatency) || len(payload.Scenario.Modes) != 3 {
		t.Errorf("scenario panel = %+v", payload.Scenario)
	}
	if payload.Walk.Note == "" || payload.Retry.Note == "" || payload.Reasoning == "" {
		t.Error("every panel must carry its derived note")
	}

	// The wire is camelCase end to end — the smoke caught this once:
	// the counters struct without tags serialized "Allowed"/"FastFailed"
	// and the client's DTO rightly refused the payload.
	encoded, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	for _, key := range []string{
		`"allowed":7`, `"fastFailed":2`, `"succeeded":4`, `"failed":3`,
		`"stateAgeMs":1234`, `"elapsedMs":120.45`, `"backoffMs":41.25`, `"probeInFlight":false`,
	} {
		if !strings.Contains(string(encoded), key) {
			t.Errorf("wire body lacks the camelCase key %s", key)
		}
	}
}

// TestReliabilityBreakerDescribeWalkCoversTheHonestFallback pins the
// note derivation's default arm: a shape outside the named arcs still
// gets a note built from its own observed counts.
func TestReliabilityBreakerDescribeWalkCoversTheHonestFallback(t *testing.T) {
	walk := ReliabilityWalk{
		Mode: core.FailureLatency,
		Rows: []ReliabilityAttemptRow{
			{Index: 1, StateBefore: ReliabilityBreakerHalfOpen, Action: reliabilityActionCall,
				Outcome: reliabilityOutcomeDeadline, StateAfter: ReliabilityBreakerOpen},
			{Index: 2, StateBefore: ReliabilityBreakerOpen, Action: reliabilityActionFastFail,
				Outcome: reliabilityOutcomeRefused, StateAfter: ReliabilityBreakerOpen},
		},
	}
	note := DescribeReliabilityWalk(walk)
	if !strings.Contains(note, "1 call(s) reached the dependency, 1 refused") {
		t.Errorf("fallback note = %q, want the observed counts", note)
	}
}
