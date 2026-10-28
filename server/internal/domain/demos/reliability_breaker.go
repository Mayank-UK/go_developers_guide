// Topic 39 reliability's live demo: the breaker. The machinery is
// REAL, not narrated: a mutex-guarded circuit breaker with
// closed/open/half-open states, a consecutive-failure threshold, a
// cooldown window, and single-probe half-open semantics — protecting a
// downstream call that honors the SHARED ScenarioStore's injectFailure
// enum and respects context deadlines. The breaker PERSISTS for the
// process lifetime (the handler owns one console), so flipping the
// global scenario panel trips and recovers it live across requests:
// injectFailure=error fails the dependency until the threshold opens
// the circuit, injectFailure=latency stalls until the per-call
// deadline converts the stall into context.DeadlineExceeded (latency
// IS a failure — the timeout is what catches it), and injectFailure
// =none lets the half-open probe succeed and close the circuit again.
// Every timing on the wire was MEASURED during the request's own walk:
// attempt durations and backoff waits come from time.Since around the
// real calls, never from a configured number dressed up as a result.
// The retry half is equally real: a bounded exponential-backoff series
// with jitter, each wait interruptible by the request context, its
// timeline served row by row with measured elapsed values.
package demos

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"sync"
	"time"

	"guide/internal/core"
)

// ReliabilityBreakerPath is the demo area's endpoint — registered on
// the real mux by the composition root.
const ReliabilityBreakerPath = "/api/demo/reliability/breaker"

// The breaker's three states, on the wire exactly as the teaching
// names them.
const (
	// ReliabilityBreakerClosed is the healthy state: calls flow.
	ReliabilityBreakerClosed = "closed"
	// ReliabilityBreakerOpen is the tripped state: calls fast-fail
	// without touching the dependency until the cooldown elapses.
	ReliabilityBreakerOpen = "open"
	// ReliabilityBreakerHalfOpen is the probing state after cooldown:
	// exactly one call at a time may through, as the probe.
	ReliabilityBreakerHalfOpen = "half-open"
)

// ErrReliabilityInjected is the downstream's failure under the
// scenario's error injection — the stand-in for a real dependency
// answering "no" (a 5xx, a reset connection, a refused write).
var ErrReliabilityInjected = errors.New(
	"reliability demo: the scenario's error injection armed the dependency to fail",
)

// reliabilityLatencyOvershoot is how much longer the stalled
// dependency would wait BEYOND the per-call deadline before answering:
// the deadline always fires first, which is the entire point — the
// overshoot only exists so an unbounded caller could never hang.
const reliabilityLatencyOvershoot = 500 * time.Millisecond

// maxReliabilityTransitions bounds the transition ring the payload
// serves: the most recent state changes, not an unbounded log.
const maxReliabilityTransitions = 12

// ReliabilityConfig is the console's whole tuning surface. Every field
// is a real constant on the wire (the payload serves it as the config
// panel) — deliberately small so a terminal tour can watch a full
// trip-and-recover cycle in seconds.
type ReliabilityConfig struct {
	// FailureThreshold is how many consecutive failures open the
	// breaker from closed.
	FailureThreshold int
	// Cooldown is how long an open breaker fast-fails before allowing
	// a half-open probe.
	Cooldown time.Duration
	// CallTimeout is the per-call deadline the walk and the retry
	// series apply around every downstream call.
	CallTimeout time.Duration
	// WalkAttempts is how many downstream calls one request drives
	// through the breaker.
	WalkAttempts int
	// RetryAttempts bounds the retry-with-backoff series.
	RetryAttempts int
	// RetryBaseDelay is the first backoff rung; each subsequent rung
	// doubles, up to the attempt bound.
	RetryBaseDelay time.Duration
	// RetryJitter is the exclusive upper bound of random jitter added
	// to every backoff rung — de-synchronization for simultaneous
	// retriers.
	RetryJitter time.Duration
}

// DefaultReliabilityConfig is what the composition root boots with:
// a threshold of three, a two-second cooldown, a per-call deadline
// far inside the server's write timeout, a four-attempt walk, and a
// three-attempt retry series whose base delay keeps a worst-case
// request inside a comfortable second even under latency injection.
func DefaultReliabilityConfig() ReliabilityConfig {
	return ReliabilityConfig{
		FailureThreshold: 3,
		Cooldown:         2 * time.Second,
		CallTimeout:      120 * time.Millisecond,
		WalkAttempts:     4,
		RetryAttempts:    3,
		RetryBaseDelay:   40 * time.Millisecond,
		RetryJitter:      20 * time.Millisecond,
	}
}

// ── the circuit breaker ─────────────────────────────────────────────

// ReliabilityTransition is one observed state change: when, from
// which state, to which, and the cause the breaker itself recorded.
type ReliabilityTransition struct {
	At    time.Time `json:"at"`
	From  string    `json:"from"`
	To    string    `json:"to"`
	Cause string    `json:"cause"`
}

// ReliabilityBreakerCounters are the process-lifetime totals the
// payload serves beside the state.
type ReliabilityBreakerCounters struct {
	Allowed    int `json:"allowed"`
	FastFailed int `json:"fastFailed"`
	Succeeded  int `json:"succeeded"`
	Failed     int `json:"failed"`
}

// ReliabilityBreakerSnapshot is the breaker's observed state at one
// instant: current state, how long it has held it, the counters, and
// the recent transition ring (newest last).
type ReliabilityBreakerSnapshot struct {
	State         string
	StateAge      time.Duration
	Counters      ReliabilityBreakerCounters
	Transitions   []ReliabilityTransition
	ProbeInFlight bool
}

// ReliabilityDecision is Allow's answer: whether the call may proceed,
// the state the breaker was in, and — for a refusal — the human-readable
// reason (which refusal kind, and how much cooldown remains).
type ReliabilityDecision struct {
	Allowed bool
	State   string
	Reason  string
}

// ReliabilityBreaker is a real circuit breaker: closed while calls
// succeed, opened by FailureThreshold consecutive failures, half-open
// after Cooldown with single-probe semantics. All state is
// mutex-guarded because the breaker is genuinely shared — every
// request of the demo's handler walks the SAME instance, exactly the
// way every request of a real service shares its breaker. The clock is
// injected so the state machine's tests are deterministic; elapsed
// timings elsewhere on the wire are always wall-clock measurements.
type ReliabilityBreaker struct {
	threshold int
	cooldown  time.Duration

	mu               sync.Mutex
	state            string
	consecutiveFails int
	openedAt         time.Time
	stateChangedAt   time.Time
	probeInFlight    bool
	counters         ReliabilityBreakerCounters
	transitions      []ReliabilityTransition

	now func() time.Time
}

// NewReliabilityBreaker builds a closed breaker over a clock.
func NewReliabilityBreaker(threshold int, cooldown time.Duration, now func() time.Time) *ReliabilityBreaker {
	return &ReliabilityBreaker{
		threshold:      threshold,
		cooldown:       cooldown,
		state:          ReliabilityBreakerClosed,
		stateChangedAt: now(),
		now:            now,
	}
}

// Allow decides whether one call may reach the dependency. In closed,
// always. In open, never — until the cooldown elapses, at which point
// THIS caller becomes the half-open probe. In half-open, only one
// caller at a time: a probe already in flight means everyone else
// still fast-fails.
func (b *ReliabilityBreaker) Allow() ReliabilityDecision {
	b.mu.Lock()
	defer b.mu.Unlock()

	switch b.state {
	case ReliabilityBreakerClosed:
		b.counters.Allowed++
		return ReliabilityDecision{
			Allowed: true,
			State:   ReliabilityBreakerClosed,
			Reason:  "closed — calls flow to the dependency",
		}
	case ReliabilityBreakerOpen:
		elapsed := b.now().Sub(b.openedAt)
		if remaining := b.cooldown - elapsed; remaining > 0 {
			b.counters.FastFailed++
			return ReliabilityDecision{
				Allowed: false,
				State:   ReliabilityBreakerOpen,
				Reason: fmt.Sprintf("open — fast-fail; %s of the %s cooldown remains",
					remaining.Round(time.Millisecond), b.cooldown),
			}
		}
		b.recordTransition(ReliabilityBreakerHalfOpen, "cooldown elapsed — probing the dependency")
		b.probeInFlight = true
		b.counters.Allowed++
		return ReliabilityDecision{
			Allowed: true,
			State:   ReliabilityBreakerHalfOpen,
			Reason:  "half-open — this call is the probe",
		}
	default: // half-open, no probe in flight
		if b.probeInFlight {
			b.counters.FastFailed++
			return ReliabilityDecision{
				Allowed: false,
				State:   ReliabilityBreakerHalfOpen,
				Reason:  "half-open — a probe is already in flight",
			}
		}
		b.probeInFlight = true
		b.counters.Allowed++
		return ReliabilityDecision{
			Allowed: true,
			State:   ReliabilityBreakerHalfOpen,
			Reason:  "half-open — this call is the probe",
		}
	}
}

// OnSuccess records a successful call: counters tick, the failure
// streak resets, and a successful half-open probe CLOSES the breaker —
// the recovery path.
func (b *ReliabilityBreaker) OnSuccess() {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.counters.Succeeded++
	b.consecutiveFails = 0
	b.probeInFlight = false
	if b.state == ReliabilityBreakerHalfOpen {
		b.recordTransition(ReliabilityBreakerClosed, "probe succeeded — recovered")
	}
}

// OnFailure records a failed call: counters tick, the streak grows,
// and the threshold opens the breaker from closed. A FAILED half-open
// probe re-opens it for a full fresh cooldown — one bad probe is
// evidence, not a verdict, so the circuit stays cautious.
func (b *ReliabilityBreaker) OnFailure() {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.counters.Failed++
	b.consecutiveFails++
	b.probeInFlight = false
	switch {
	case b.state == ReliabilityBreakerHalfOpen:
		b.recordTransition(ReliabilityBreakerOpen, "probe failed — reopened for a fresh cooldown")
	case b.state == ReliabilityBreakerClosed && b.consecutiveFails >= b.threshold:
		b.recordTransition(ReliabilityBreakerOpen, fmt.Sprintf(
			"%d consecutive failures — threshold reached", b.consecutiveFails))
	}
}

// State reports the current state (a convenience read for row shaping;
// Snapshot is the full observation).
func (b *ReliabilityBreaker) State() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.state
}

// Snapshot observes the breaker at now: state, how long it has held
// it, the counters, the probe flag, and a copy of the recent
// transition ring.
func (b *ReliabilityBreaker) Snapshot(now time.Time) ReliabilityBreakerSnapshot {
	b.mu.Lock()
	defer b.mu.Unlock()

	transitions := append([]ReliabilityTransition(nil), b.transitions...)
	return ReliabilityBreakerSnapshot{
		State:         b.state,
		StateAge:      now.Sub(b.stateChangedAt),
		Counters:      b.counters,
		Transitions:   transitions,
		ProbeInFlight: b.probeInFlight,
	}
}

// Reset returns the breaker to its constructed state: closed, zero
// counters, no failure streak, no probe in flight, and an EMPTY
// transition history — the shared demo reset (POST /api/demo/reset)
// calls this so a reader can replay the trip-and-recover arc from its
// beginning. The reset is deliberately not itself a transition: the
// breaker observes nothing, exactly as at construction. Idempotent by
// construction — resetting a fresh breaker changes nothing observable.
func (b *ReliabilityBreaker) Reset() {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.state = ReliabilityBreakerClosed
	b.consecutiveFails = 0
	b.probeInFlight = false
	b.counters = ReliabilityBreakerCounters{}
	b.transitions = nil
	b.stateChangedAt = b.now()
}

// recordTransition moves the breaker to next and rings the change.
// The caller holds the mutex; when next is open, the cooldown clock
// restarts here.
func (b *ReliabilityBreaker) recordTransition(next string, cause string) {
	b.transitions = append(b.transitions, ReliabilityTransition{
		At:    b.now(),
		From:  b.state,
		To:    next,
		Cause: cause,
	})
	if len(b.transitions) > maxReliabilityTransitions {
		b.transitions = b.transitions[len(b.transitions)-maxReliabilityTransitions:]
	}
	if next == ReliabilityBreakerOpen {
		b.openedAt = b.now()
		b.consecutiveFails = 0
	}
	b.state = next
	b.stateChangedAt = b.now()
}

// ── the console: dependency + walk + retry series ───────────────────

// The walk row's action vocabulary: a real downstream call, or a
// refusal the breaker answered without one.
const (
	reliabilityActionCall     = "call"
	reliabilityActionFastFail = "fast-fail"
)

// The outcome vocabulary, derived from observed errors only.
const (
	reliabilityOutcomeSuccess   = "success"
	reliabilityOutcomeInjected  = "injected error"
	reliabilityOutcomeDeadline  = "deadline exceeded"
	reliabilityOutcomeCancelled = "cancelled"
	reliabilityOutcomeRefused   = "refused"
)

// ReliabilityAttemptRow is one attempt of a request's walk through the
// breaker: the state before, whether the dependency was actually
// called or the call refused, the outcome label derived from the real
// error, the measured elapsed time, and the state after the breaker
// digested the result.
type ReliabilityAttemptRow struct {
	Index       int
	StateBefore string
	Action      string
	Outcome     string
	Detail      string
	Elapsed     time.Duration
	StateAfter  string
}

// ReliabilityWalk is one request's drive through the persistent
// breaker: the scenario mode that was armed and the attempt rows.
type ReliabilityWalk struct {
	Mode core.FailureInjection
	Rows []ReliabilityAttemptRow
}

// ReliabilityRetryRow is one attempt of the retry-with-backoff series:
// outcome, the real error when there was one, the measured attempt
// duration, and the measured backoff wait AFTER this attempt (zero on
// the last row or when the series ended early).
type ReliabilityRetryRow struct {
	Attempt int
	Outcome string
	Detail  string
	Elapsed time.Duration
	Backoff time.Duration
}

// ReliabilityRetryRun is the whole series: the armed mode, the rows,
// whether the series succeeded, and the measured total elapsed time.
type ReliabilityRetryRun struct {
	Mode         core.FailureInjection
	Rows         []ReliabilityRetryRow
	Succeeded    bool
	TotalElapsed time.Duration
}

// ReliabilityConsole owns the process-lifetime machinery the demo's
// handler walks: the persistent breaker, the shared scenario store the
// dependency honors, and the config that tunes both. One console per
// process, built at the composition root.
type ReliabilityConsole struct {
	cfg     ReliabilityConfig
	store   *core.ScenarioStore
	breaker *ReliabilityBreaker
}

// NewReliabilityConsole validates the config (every dimension
// positive, attempt counts inside honest bounds) and builds the
// console around the shared scenario store.
func NewReliabilityConsole(cfg ReliabilityConfig, store *core.ScenarioStore) (*ReliabilityConsole, error) {
	if cfg.FailureThreshold < 1 {
		return nil, fmt.Errorf("reliability console: failure threshold %d must be at least 1", cfg.FailureThreshold)
	}
	if cfg.Cooldown <= 0 {
		return nil, fmt.Errorf("reliability console: cooldown %s must be positive", cfg.Cooldown)
	}
	if cfg.CallTimeout <= 0 {
		return nil, fmt.Errorf("reliability console: call timeout %s must be positive", cfg.CallTimeout)
	}
	if cfg.WalkAttempts < 1 || cfg.WalkAttempts > 8 {
		return nil, fmt.Errorf("reliability console: walk attempts %d must be 1-8", cfg.WalkAttempts)
	}
	if cfg.RetryAttempts < 1 || cfg.RetryAttempts > 8 {
		return nil, fmt.Errorf("reliability console: retry attempts %d must be 1-8", cfg.RetryAttempts)
	}
	if cfg.RetryBaseDelay <= 0 {
		return nil, fmt.Errorf("reliability console: retry base delay %s must be positive", cfg.RetryBaseDelay)
	}
	if cfg.RetryJitter < 0 {
		return nil, fmt.Errorf("reliability console: retry jitter %s must not be negative", cfg.RetryJitter)
	}
	if store == nil {
		return nil, fmt.Errorf("reliability console: scenario store must not be nil")
	}
	return &ReliabilityConsole{
		cfg:     cfg,
		store:   store,
		breaker: NewReliabilityBreaker(cfg.FailureThreshold, cfg.Cooldown, time.Now),
	}, nil
}

// Config exposes the console's tuning (the handler serves it as the
// payload's config panel).
func (c *ReliabilityConsole) Config() ReliabilityConfig { return c.cfg }

// ScenarioState exposes the scenario the dependency currently honors.
func (c *ReliabilityConsole) ScenarioState() core.ScenarioState { return c.store.Snapshot() }

// BreakerSnapshot observes the persistent breaker at now.
func (c *ReliabilityConsole) BreakerSnapshot(now time.Time) ReliabilityBreakerSnapshot {
	return c.breaker.Snapshot(now)
}

// BreakerObservation pairs the breaker snapshot with the config it
// runs under — the shape the pure payload builder consumes.
func (c *ReliabilityConsole) BreakerObservation(now time.Time) ReliabilityBreakerSnapshotWithConfig {
	return ReliabilityBreakerSnapshotWithConfig{
		ReliabilityBreakerSnapshot: c.breaker.Snapshot(now),
		Config:                     c.cfg,
	}
}

// Reset returns the console's process-lifetime machinery — the
// persistent breaker — to its constructed state, under the breaker's
// own mutex (a reset racing a walk interleaves safely: every field
// access is guarded). The config and the shared scenario store are
// wiring, not residue — they stay exactly as the composition root
// built them.
func (c *ReliabilityConsole) Reset() {
	c.breaker.Reset()
}

// callDownstream performs one protected call. The dependency honors
// the scenario's failure injection and the CALLER'S deadline: under
// "error" it fails immediately with the demo sentinel; under
// "latency" it stalls on a select between the deadline closing
// Done() and a timer deliberately set BEYOND it, so the deadline
// always wins and the stall surfaces as context.DeadlineExceeded —
// the failure mode only a timeout converts into an error at all.
func (c *ReliabilityConsole) callDownstream(ctx context.Context) error {
	switch c.store.Snapshot().InjectFailure {
	case core.FailureError:
		return ErrReliabilityInjected
	case core.FailureLatency:
		timer := time.NewTimer(c.cfg.CallTimeout + reliabilityLatencyOvershoot)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return fmt.Errorf("dependency stalled past its deadline: %w", ctx.Err())
		case <-timer.C:
			return nil
		}
	default:
		return nil
	}
}

// Walk drives cfg.WalkAttempts downstream calls through the PERSISTENT
// breaker, recording every attempt as a row: refusals carry the
// breaker's own reason, calls carry the measured outcome. Each call
// gets its own deadline derived from the request context, so a client
// that walks away aborts the walk instead of wedging it.
func (c *ReliabilityConsole) Walk(ctx context.Context) ReliabilityWalk {
	walk := ReliabilityWalk{Mode: c.store.Snapshot().InjectFailure}
	for index := 1; index <= c.cfg.WalkAttempts; index++ {
		decision := c.breaker.Allow()
		if !decision.Allowed {
			started := time.Now()
			walk.Rows = append(walk.Rows, ReliabilityAttemptRow{
				Index:       index,
				StateBefore: decision.State,
				Action:      reliabilityActionFastFail,
				Outcome:     reliabilityOutcomeRefused,
				Detail:      decision.Reason,
				Elapsed:     time.Since(started),
				StateAfter:  c.breaker.State(),
			})
			continue
		}

		row := ReliabilityAttemptRow{
			Index:       index,
			StateBefore: decision.State,
			Action:      reliabilityActionCall,
		}
		started := time.Now()
		callCtx, cancel := context.WithTimeout(ctx, c.cfg.CallTimeout)
		err := c.callDownstream(callCtx)
		cancel()
		row.Elapsed = time.Since(started)
		if err != nil {
			c.breaker.OnFailure()
			row.Outcome, row.Detail = reliabilityOutcomeFor(err), err.Error()
		} else {
			c.breaker.OnSuccess()
			row.Outcome = reliabilityOutcomeSuccess
		}
		row.StateAfter = c.breaker.State()
		walk.Rows = append(walk.Rows, row)
	}
	return walk
}

// RetrySeries drives one bounded retry-with-backoff execution against
// the same dependency — deliberately OUTSIDE the breaker, because the
// two mechanisms answer different questions (retry asks "was that
// transient?", the breaker asks "is the dependency worth calling at
// all?"). Every attempt duration and backoff wait is measured; the
// waits are interruptible by the request context.
func (c *ReliabilityConsole) RetrySeries(ctx context.Context) ReliabilityRetryRun {
	run := ReliabilityRetryRun{Mode: c.store.Snapshot().InjectFailure}
	started := time.Now()
	for attempt := 1; attempt <= c.cfg.RetryAttempts; attempt++ {
		row := ReliabilityRetryRow{Attempt: attempt}
		callStart := time.Now()
		callCtx, cancel := context.WithTimeout(ctx, c.cfg.CallTimeout)
		err := c.callDownstream(callCtx)
		cancel()
		row.Elapsed = time.Since(callStart)

		if err == nil {
			row.Outcome = reliabilityOutcomeSuccess
			run.Rows = append(run.Rows, row)
			run.Succeeded = true
			break
		}
		row.Outcome, row.Detail = reliabilityOutcomeFor(err), err.Error()
		if attempt < c.cfg.RetryAttempts {
			wait := reliabilityBackoff(c.cfg, attempt)
			waitStart := time.Now()
			interrupted := reliabilityWait(ctx, wait) != nil
			row.Backoff = time.Since(waitStart)
			run.Rows = append(run.Rows, row)
			if interrupted {
				break
			}
			continue
		}
		run.Rows = append(run.Rows, row)
	}
	run.TotalElapsed = time.Since(started)
	return run
}

// reliabilityBackoff computes one rung: base doubled per prior
// attempt, plus bounded jitter so simultaneous retriers de-synchronize.
func reliabilityBackoff(cfg ReliabilityConfig, attempt int) time.Duration {
	shift := attempt - 1
	if shift > 4 {
		shift = 4 // attempts are bounded small; the cap is defense in depth
	}
	delay := cfg.RetryBaseDelay << shift
	if cfg.RetryJitter > 0 {
		delay += time.Duration(rand.Int64N(int64(cfg.RetryJitter) + 1))
	}
	return delay
}

// reliabilityWait sleeps out d unless the context ends first — the
// backoff waits are as cancellable as the calls they separate.
func reliabilityWait(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return nil
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// reliabilityOutcomeFor labels an observed error by identity — the
// error taxonomy's stance (topic 30) applied to a timeline row.
func reliabilityOutcomeFor(err error) (outcome string) {
	switch {
	case errors.Is(err, ErrReliabilityInjected):
		return reliabilityOutcomeInjected
	case errors.Is(err, context.DeadlineExceeded):
		return reliabilityOutcomeDeadline
	case errors.Is(err, context.Canceled):
		return reliabilityOutcomeCancelled
	default:
		return err.Error()
	}
}

// ── the pure payload builder ────────────────────────────────────────

// ReliabilityScenarioPanel is the scenario sub-panel: the runtime
// toggle the dependency honors and its three modes.
type ReliabilityScenarioPanel struct {
	Mode          string   `json:"mode"`
	GetEndpoint   string   `json:"getEndpoint"`
	PatchEndpoint string   `json:"patchEndpoint"`
	Modes         []string `json:"modes"`
	Note          string   `json:"note"`
}

// ReliabilityConfigPanel is the config as served: every knob the
// machinery actually runs with, in milliseconds.
type ReliabilityConfigPanel struct {
	FailureThreshold int     `json:"failureThreshold"`
	CooldownMs       float64 `json:"cooldownMs"`
	CallTimeoutMs    float64 `json:"callTimeoutMs"`
	WalkAttempts     int     `json:"walkAttempts"`
	HalfOpenProbes   int     `json:"halfOpenProbes"`
	RetryAttempts    int     `json:"retryAttempts"`
	RetryBaseMs      float64 `json:"retryBaseDelayMs"`
	RetryJitterMs    float64 `json:"retryJitterMs"`
}

// ReliabilityBreakerPanel is the persistent breaker's observation:
// state, measured age in the state, counters, the probe flag, and the
// recent transitions with their wall-clock times.
type ReliabilityBreakerPanel struct {
	State         string                     `json:"state"`
	StateAgeMs    float64                    `json:"stateAgeMs"`
	Config        ReliabilityConfigPanel     `json:"config"`
	Counters      ReliabilityBreakerCounters `json:"counters"`
	ProbeInFlight bool                       `json:"probeInFlight"`
	Transitions   []ReliabilityTransition    `json:"transitions"`
}

// ReliabilityAttemptView is one walk row on the wire.
type ReliabilityAttemptView struct {
	Index       int     `json:"index"`
	StateBefore string  `json:"stateBefore"`
	Action      string  `json:"action"`
	Outcome     string  `json:"outcome"`
	Detail      string  `json:"detail,omitempty"`
	ElapsedMs   float64 `json:"elapsedMs"`
	StateAfter  string  `json:"stateAfter"`
}

// ReliabilityWalkPanel is the walk as served, with its derived note.
type ReliabilityWalkPanel struct {
	Mode string                   `json:"mode"`
	Rows []ReliabilityAttemptView `json:"rows"`
	Note string                   `json:"note"`
}

// ReliabilityRetryRowView is one retry row on the wire.
type ReliabilityRetryRowView struct {
	Attempt   int     `json:"attempt"`
	Outcome   string  `json:"outcome"`
	Detail    string  `json:"detail,omitempty"`
	ElapsedMs float64 `json:"elapsedMs"`
	BackoffMs float64 `json:"backoffMs"`
}

// ReliabilityRetryPanel is the retry series as served, with its
// derived note.
type ReliabilityRetryPanel struct {
	Mode           string                    `json:"mode"`
	Succeeded      bool                      `json:"succeeded"`
	Rows           []ReliabilityRetryRowView `json:"rows"`
	TotalElapsedMs float64                   `json:"totalElapsedMs"`
	Note           string                    `json:"note"`
}

// ReliabilityBreakerPayload is the wire shape of
// GET /api/demo/reliability/breaker.
type ReliabilityBreakerPayload struct {
	Mode      string                   `json:"mode"`
	Scenario  ReliabilityScenarioPanel `json:"scenario"`
	Breaker   ReliabilityBreakerPanel  `json:"breaker"`
	Walk      ReliabilityWalkPanel     `json:"walk"`
	Retry     ReliabilityRetryPanel    `json:"retry"`
	Reasoning string                   `json:"reasoning"`
}

// milliseconds renders a duration as a float with sub-millisecond
// precision — fast-fails show their real microseconds, not a rounded
// zero.
func milliseconds(d time.Duration) float64 {
	return float64(d.Microseconds()) / 1000
}

// BuildReliabilityBreakerPayload shapes the wire payload (pure): the
// scenario panel, the breaker's snapshot with its config, the walk
// and retry timelines carrying exactly the durations that were
// measured, and notes derived from what actually happened — never from
// what the mode promised.
func BuildReliabilityBreakerPayload(facts ReliabilityBreakerFacts) ReliabilityBreakerPayload {
	config := ReliabilityConfigPanel{
		FailureThreshold: facts.Breaker.Config.FailureThreshold,
		CooldownMs:       milliseconds(facts.Breaker.Config.Cooldown),
		CallTimeoutMs:    milliseconds(facts.Breaker.Config.CallTimeout),
		WalkAttempts:     facts.Breaker.Config.WalkAttempts,
		HalfOpenProbes:   1,
		RetryAttempts:    facts.Breaker.Config.RetryAttempts,
		RetryBaseMs:      milliseconds(facts.Breaker.Config.RetryBaseDelay),
		RetryJitterMs:    milliseconds(facts.Breaker.Config.RetryJitter),
	}

	transitions := make([]ReliabilityTransition, 0, len(facts.Breaker.Transitions))
	for _, transition := range facts.Breaker.Transitions {
		transitions = append(transitions, ReliabilityTransition{
			At:    transition.At.UTC(),
			From:  transition.From,
			To:    transition.To,
			Cause: transition.Cause,
		})
	}

	walkRows := make([]ReliabilityAttemptView, 0, len(facts.Walk.Rows))
	for _, row := range facts.Walk.Rows {
		walkRows = append(walkRows, ReliabilityAttemptView{
			Index:       row.Index,
			StateBefore: row.StateBefore,
			Action:      row.Action,
			Outcome:     row.Outcome,
			Detail:      row.Detail,
			ElapsedMs:   milliseconds(row.Elapsed),
			StateAfter:  row.StateAfter,
		})
	}

	retryRows := make([]ReliabilityRetryRowView, 0, len(facts.Retry.Rows))
	for _, row := range facts.Retry.Rows {
		retryRows = append(retryRows, ReliabilityRetryRowView{
			Attempt:   row.Attempt,
			Outcome:   row.Outcome,
			Detail:    row.Detail,
			ElapsedMs: milliseconds(row.Elapsed),
			BackoffMs: milliseconds(row.Backoff),
		})
	}

	return ReliabilityBreakerPayload{
		Mode: "process-lifetime breaker: one shared circuit breaker protects " +
			"the dependency for the life of the process — every request walks " +
			"the SAME instance, so the global scenario panel (PATCH " +
			"/api/demo/scenario) trips and recovers it live across requests; " +
			"every timing below was measured during this request's own walk",
		Scenario: ReliabilityScenarioPanel{
			Mode:          string(facts.Scenario.InjectFailure),
			GetEndpoint:   "/api/demo/scenario",
			PatchEndpoint: "/api/demo/scenario",
			Modes: []string{
				string(core.FailureNone),
				string(core.FailureError),
				string(core.FailureLatency),
			},
			Note: "the runtime toggle the dependency honors on every call: " +
				"error fails it outright, latency stalls it past the per-call " +
				"deadline, none lets it answer",
		},
		Breaker: ReliabilityBreakerPanel{
			State:         facts.Breaker.State,
			StateAgeMs:    milliseconds(facts.Breaker.StateAge),
			Config:        config,
			Counters:      facts.Breaker.Counters,
			ProbeInFlight: facts.Breaker.ProbeInFlight,
			Transitions:   transitions,
		},
		Walk: ReliabilityWalkPanel{
			Mode: string(facts.Walk.Mode),
			Rows: walkRows,
			Note: DescribeReliabilityWalk(facts.Walk),
		},
		Retry: ReliabilityRetryPanel{
			Mode:           string(facts.Retry.Mode),
			Succeeded:      facts.Retry.Succeeded,
			Rows:           retryRows,
			TotalElapsedMs: milliseconds(facts.Retry.TotalElapsed),
			Note:           DescribeReliabilityRetry(facts.Retry),
		},
		Reasoning: "The breaker state, counters, and transitions persist across " +
			"requests — that is the teaching: a circuit breaker is shared " +
			"state about a dependency, not a per-request helper. PATCH the " +
			"scenario to error and reload to watch the threshold trip; keep " +
			"reloading while open to watch calls fast-fail before reaching the " +
			"dependency; set latency to watch the per-call deadline convert a " +
			"stall into deadline exceeded; set none and reload after the " +
			"cooldown to watch the half-open probe close the circuit. Every " +
			"elapsed and backoff figure was MEASURED around the real calls of " +
			"this request — the config panel states the policy, the rows state " +
			"what actually happened, and neither invents the other",
	}
}

// ReliabilityBreakerFacts carries everything the pure builder needs:
// the scenario snapshot, the breaker observation (snapshot plus its
// config), and the walk and retry runs the handler just executed.
type ReliabilityBreakerFacts struct {
	Scenario core.ScenarioState
	Breaker  ReliabilityBreakerSnapshotWithConfig
	Walk     ReliabilityWalk
	Retry    ReliabilityRetryRun
}

// ReliabilityBreakerSnapshotWithConfig pairs the breaker snapshot
// with the config the console runs — the payload serves both, because
// a state without its policy is an anecdote.
type ReliabilityBreakerSnapshotWithConfig struct {
	ReliabilityBreakerSnapshot
	Config ReliabilityConfig
}

// DescribeReliabilityWalk derives the walk's note from what its rows
// actually recorded — recovery, threshold trip, fast-fail standoff, or
// clean service — with an honest fallback for shapes the named paths
// do not cover.
func DescribeReliabilityWalk(walk ReliabilityWalk) string {
	calls, refusals, successes := 0, 0, 0
	tripAt, recoveryAt := 0, 0
	for _, row := range walk.Rows {
		switch {
		case row.Action == reliabilityActionFastFail:
			refusals++
		case row.Outcome == reliabilityOutcomeSuccess:
			calls++
			successes++
			if row.StateBefore == ReliabilityBreakerHalfOpen && recoveryAt == 0 {
				recoveryAt = row.Index
			}
		default:
			calls++
			if row.StateBefore == ReliabilityBreakerClosed &&
				row.StateAfter == ReliabilityBreakerOpen && tripAt == 0 {
				tripAt = row.Index
			}
		}
	}
	switch {
	case recoveryAt > 0 && refusals > 0:
		return fmt.Sprintf(
			"recovery observed: after the cooldown the half-open probe on attempt %d succeeded and closed the circuit, and %d refused attempt(s) before it show what the open breaker was protecting the dependency from",
			recoveryAt, refusals)
	case recoveryAt > 0:
		return fmt.Sprintf(
			"recovery observed: the half-open probe on attempt %d succeeded and the breaker closed — the dependency is trusted again",
			recoveryAt)
	case tripAt > 0 && refusals > 0:
		return fmt.Sprintf(
			"the threshold tripped on attempt %d: attempts after it were refused outright — %d call(s) reached the dependency, %d were fast-failed without touching it",
			tripAt, calls, refusals)
	case refusals > 0 && calls == 0:
		return fmt.Sprintf(
			"the breaker refused all %d attempts without touching the dependency — the cooldown window is still running; reload after it elapses to arm the half-open probe",
			refusals)
	case calls > 0 && successes == calls:
		return fmt.Sprintf(
			"every one of the %d attempts reached the dependency and succeeded — the circuit stays closed while calls answer in time",
			calls)
	default:
		return fmt.Sprintf(
			"%d call(s) reached the dependency, %d refused, %d succeeded — the walk reports what actually happened, never what the mode promised",
			calls, refusals, successes)
	}
}

// DescribeReliabilityRetry derives the retry series' note from its
// measured rows.
func DescribeReliabilityRetry(run ReliabilityRetryRun) string {
	if run.Succeeded {
		for i, row := range run.Rows {
			if row.Outcome == reliabilityOutcomeSuccess {
				return fmt.Sprintf(
					"the series succeeded on attempt %d after %d failed attempt(s) — the backoff waits between them are measured on the wire",
					row.Attempt, i)
			}
		}
	}
	failures := len(run.Rows)
	last := run.Rows[failures-1]
	return fmt.Sprintf(
		"the series exhausted its %d attempts (last outcome: %s) and surfaced the error for the caller to handle — a bounded retry never turns into an infinite loop, and the breaker decides whether the NEXT request even tries",
		failures, last.Outcome)
}
