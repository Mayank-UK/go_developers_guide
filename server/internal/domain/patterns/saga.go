// The saga is the second distributed-application pattern this package
// carries: the machinery behind "a business transaction that spans
// services which cannot share a database transaction". Where the event
// store records what happened, the saga DOORS the doing — it drives an
// ordered list of steps forward until one fails or the context dies,
// then undoes the steps that already ran, in reverse order, one
// compensating action per forward action. Every fact about a run lives
// in an append-only ledger of per-step outcomes, so a caller can answer
// the two questions a distributed transaction must answer at any
// moment: what happened, and what was undone about it.
//
// The design decisions worth quoting, each enforced below:
//
//   - Compensation is REVERSE and PARTIAL. Only steps whose forward
//     action SUCCEEDED are compensated — the step that failed never
//     ran to completion, so it has nothing to undo.
//
//   - Cancellation stops the forward work, never the cleanup it
//     triggered. The undo pass runs on context.WithoutCancel of the
//     caller's context: values (request ids, trace ids) survive, the
//     cancellation does not.
//
//   - Compensation is IDEMPOTENT at the runner. A compensating action
//     is called at most once per execution; a re-compensation attempt
//     is recorded as an already-compensated no-op, never a second call.
//
//   - One compensation failure does not stop the undos above it. Every
//     executed step gets its chance; failures are recorded, not raised.
//
//   - A runner is a state machine, not a loop. Run drives the saga
//     once; later calls answer the recorded outcome. Compensate is the
//     recovery door into the same ledger.
package patterns

import (
	"context"
	"fmt"
	"sync"
)

// SagaAction is the one shape every step's two halves share: a function
// that does bounded work under a context and answers an error. Forward
// actions advance the business transaction; compensating actions undo
// exactly their own step's effect — nothing more.
type SagaAction func(ctx context.Context) error

// SagaStep is one ordered unit of saga work: a human-readable Name for
// the ledger, the Forward action that advances the transaction, and the
// Compensate action that undoes the forward's effect. A step whose work
// cannot be undone (an email sent, a webhook delivered — a fact the
// outside world already holds) declares no Compensate action; such
// steps belong LAST, after everything that can still be rolled back.
//
// A nil Forward action is a programmer bug and panics with the step's
// name the moment the runner reaches it; a nil Compensate action is a
// legitimate "nothing to undo", recorded as such in the ledger.
type SagaStep struct {
	Name       string
	Forward    SagaAction
	Compensate SagaAction
}

// SagaPhase labels which half of the step a ledger row belongs to.
type SagaPhase string

const (
	// SagaPhaseForward marks rows recorded while the saga advanced.
	SagaPhaseForward SagaPhase = "forward"
	// SagaPhaseCompensate marks rows recorded while the saga undid work.
	SagaPhaseCompensate SagaPhase = "compensate"
)

// SagaStepOutcome is the taxonomy of things that can happen to one step
// — the vocabulary of the ledger. Every row's outcome is decided by
// what the machinery observed, never asserted ahead of the run.
type SagaStepOutcome string

const (
	// SagaStepExecuted: the forward action ran and succeeded.
	SagaStepExecuted SagaStepOutcome = "executed"
	// SagaStepFailed: the forward action ran and returned an error.
	SagaStepFailed SagaStepOutcome = "failed"
	// SagaStepSkipped: the forward action never ran — an earlier step
	// failed or the context was cancelled before this step's turn.
	SagaStepSkipped SagaStepOutcome = "skipped"
	// SagaStepCompensated: the compensating action ran and succeeded.
	SagaStepCompensated SagaStepOutcome = "compensated"
	// SagaStepNothingToUndo: the step executed but declared no
	// compensating action — there is nothing to undo by design.
	SagaStepNothingToUndo SagaStepOutcome = "nothing-to-undo"
	// SagaStepCompensationFailed: the compensating action ran and
	// returned an error; recorded, and the undos above it still ran.
	SagaStepCompensationFailed SagaStepOutcome = "compensation-failed"
	// SagaStepAlreadyCompensated: a re-compensation attempt met a step
	// the runner had already undone — a recorded no-op, the runner
	// never calls a compensating action twice.
	SagaStepAlreadyCompensated SagaStepOutcome = "already-compensated"
)

// SagaStatus is a saga run's terminal state. Completed means every
// forward action succeeded. Compensated means a step failed and the
// reverse undo pass ran. Cancelled means the context died mid-run and
// the undo pass ran for everything that had executed.
type SagaStatus string

const (
	// SagaStatusCompleted: all steps executed; nothing was undone.
	SagaStatusCompleted SagaStatus = "completed"
	// SagaStatusCompensated: a step failed; executed steps were undone
	// in reverse.
	SagaStatusCompensated SagaStatus = "compensated"
	// SagaStatusCancelled: the context was cancelled mid-run; executed
	// steps were undone in reverse.
	SagaStatusCancelled SagaStatus = "cancelled"
)

// SagaStepResult is one row of the outcomes ledger: which step (by
// index and name), which phase, what happened to it, and the honest
// error text when there was one. Rows appear in recording order — the
// order the machinery actually observed them, not step order.
type SagaStepResult struct {
	Index   int
	Name    string
	Action  SagaPhase
	Outcome SagaStepOutcome
	Err     string
}

// SagaOutcome is the ledger a run answers: the terminal status, every
// recorded row, the typed trigger error (nil when the saga completed),
// and the two counts callers reach for first. The counts are counted
// from the rows above — StepsExecuted counts executed forward actions;
// StepsCompensated counts successful compensations only, so no-ops and
// compensation failures never inflate it.
type SagaOutcome struct {
	Status           SagaStatus
	Steps            []SagaStepResult
	Err              error
	StepsExecuted    int
	StepsCompensated int
}

// SagaStepError is the typed trigger error a failed or cancelled run
// carries: the identity of the step where progress stopped, wrapping
// the underlying error. For a failed step it wraps the forward action's
// error; for a cancelled run it wraps the context's error at the step
// where cancellation was observed. Callers use errors.As to learn which
// step stopped the saga and errors.Is to test for their own sentinel
// errors (and for context.Canceled / context.DeadlineExceeded).
type SagaStepError struct {
	Index    int
	StepName string
	Err      error
}

// Error implements the error interface.
func (e *SagaStepError) Error() string {
	return fmt.Sprintf("saga stopped at step %d (%s): %s", e.Index, e.StepName, e.Err)
}

// Unwrap keeps the underlying error reachable through errors.Is and
// errors.As.
func (e *SagaStepError) Unwrap() error { return e.Err }

// SagaRunner orchestrates one saga instance over an ordered list of
// steps. It is safe for concurrent use: the ledger is the runner's
// single source of truth, and the mutex guards the ledger and the
// per-step state maps ONLY — it is never held across a user-supplied
// compensating action, so readers and re-entrant calls are never
// blocked for a callback's duration, and concurrent undo passes
// coordinate through per-step claims (a compensating action is called
// by at most one pass at a time). A Compensate that arrives while a
// Run is still driving the forward pass waits at the ledger lock, then
// undoes whatever the run left executed. The forward pass itself runs
// under the lock on purpose — drives are sequential by design — so a
// Forward action must not re-enter the runner (that would deadlock);
// the compensation path is the re-entrant-safe one. The zero value is
// not usable; build one with NewSagaRunner.
type SagaRunner struct {
	mu          sync.Mutex
	steps       []SagaStep
	ran         bool
	outcome     SagaOutcome
	executed    map[int]bool
	compensated map[int]bool
	undoing     map[int]bool
}

// NewSagaRunner builds a runner over steps in the order given — order
// is the contract: forward runs left to right, compensation runs right
// to left over the prefix that executed.
func NewSagaRunner(steps ...SagaStep) *SagaRunner {
	return &SagaRunner{
		steps:       steps,
		executed:    make(map[int]bool),
		compensated: make(map[int]bool),
		undoing:     make(map[int]bool),
	}
}

// Run drives the saga once: forward through the steps in order until
// one fails or the context is cancelled, then — for a failure or a
// cancellation — the undo pass compensates every executed step in
// reverse order. The undo pass always runs on context.WithoutCancel of
// ctx: the cancellation that stopped the forward work must not stop the
// cleanup it triggered, while the context's values still flow to the
// compensating actions.
//
// Cancellation is observed between steps and inside well-behaved
// actions: a context that dies after the last forward action succeeded
// does not retroactively un-complete finished work.
//
// A saga runs once. Calling Run again answers the recorded outcome
// without re-executing anything — the runner is a state machine whose
// terminal states are completed, compensated, or cancelled.
func (r *SagaRunner) Run(ctx context.Context) SagaOutcome {
	r.mu.Lock()
	if r.ran {
		snapshot := r.snapshotLocked()
		r.mu.Unlock()
		return snapshot
	}
	r.ran = true

	stopReason, trigger := r.forwardLocked(ctx)
	switch stopReason {
	case stopNone:
		r.outcome.Status = SagaStatusCompleted
	case stopFailed:
		r.outcome.Status = SagaStatusCompensated
	default:
		r.outcome.Status = SagaStatusCancelled
	}
	r.outcome.Err = trigger
	r.mu.Unlock()

	if stopReason != stopNone {
		// The undo pass runs WITHOUT the ledger lock held: compensating
		// actions are user-supplied and may block arbitrarily long or
		// re-enter the runner — the lock guards the ledger, never the
		// callbacks.
		r.undoPass(ctx)
	}
	return r.finalize()
}

// finalize answers the run's snapshot under the ledger lock — after the
// undo pass (when one ran) has finished recording its rows, so Status,
// Err, and the ledger are read as one consistent state.
func (r *SagaRunner) finalize() SagaOutcome {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.snapshotLocked()
}

// stopReason values track why the forward pass ended — they distinguish
// the two terminal statuses that both end in compensation.
const (
	stopNone = iota
	stopFailed
	stopCancelled
)

// forwardLocked drives the forward pass (the caller holds r.mu): steps
// in order until one fails or the context is cancelled, then one
// skipped row per remaining step — the ledger shows what never ran.
// The caller keeps the lock for the whole drive on purpose: drives are
// sequential by design, and the undo pass that may follow is where the
// lock discipline changes (see undoPass).
func (r *SagaRunner) forwardLocked(ctx context.Context) (int, error) {
	stopReason := stopNone
	var trigger error

	for i, step := range r.steps {
		if stopReason != stopNone {
			// Past the stop: every remaining step is skipped, one row
			// each — the ledger shows what never ran.
			r.recordLocked(i, step.Name, SagaPhaseForward, SagaStepSkipped, "")
			continue
		}
		if err := ctx.Err(); err != nil {
			stopReason = stopCancelled
			trigger = &SagaStepError{Index: i, StepName: step.Name, Err: err}
			r.recordLocked(i, step.Name, SagaPhaseForward, SagaStepSkipped, "")
			continue
		}
		if step.Forward == nil {
			panic(fmt.Sprintf("saga: step %d (%s) has no Forward action — a step must do work", i, step.Name))
		}
		if err := step.Forward(ctx); err != nil {
			stopReason = stopFailed
			trigger = &SagaStepError{Index: i, StepName: step.Name, Err: err}
			r.recordLocked(i, step.Name, SagaPhaseForward, SagaStepFailed, err.Error())
			continue
		}
		r.executed[i] = true
		r.recordLocked(i, step.Name, SagaPhaseForward, SagaStepExecuted, "")
	}

	return stopReason, trigger
}

// Compensate is the recovery door into the same ledger: it compensates
// every step that executed and has not been undone yet, in reverse
// order, answering the rows THIS pass recorded. After a failed run's
// automatic undo pass every executed step is already compensated, so a
// retried Compensate observes one already-compensated no-op per step
// and calls nothing — the idempotency guarantee, visible as data. Like
// the automatic pass it runs on context.WithoutCancel of ctx, so an
// operator may safely compensate with the very context that died.
// Compensating a saga whose Run never happened records nothing.
func (r *SagaRunner) Compensate(ctx context.Context) []SagaStepResult {
	return r.undoPass(ctx)
}

// undoPass walks the executed steps in reverse, compensating each that
// has not been undone. The lock is held only around ledger reads and
// writes — NEVER across a user-supplied compensating action: an undo
// may block for as long as its work takes (or re-enter the runner's
// public doors), and readers must not queue behind it. Exactly-once
// survives the unlocked callbacks through per-step claims: a step is
// claimed under the lock before its action runs, so a concurrent pass
// (a racing Compensate, an automatic undo still in flight) skips the
// claimed step and leaves its row to the pass that owns the call. A
// failing compensation is recorded and the walk continues: every
// executed step gets its chance, and one stuck undo never strands the
// work above it.
func (r *SagaRunner) undoPass(ctx context.Context) []SagaStepResult {
	undoCtx := context.WithoutCancel(ctx)
	recorded := []SagaStepResult{}
	for i := len(r.steps) - 1; i >= 0; i-- {
		step := r.steps[i]
		r.mu.Lock()
		if !r.executed[i] {
			r.mu.Unlock()
			continue // never executed (failed, skipped, or never reached): nothing to undo
		}
		if r.compensated[i] {
			row := r.recordLocked(i, step.Name, SagaPhaseCompensate, SagaStepAlreadyCompensated,
				"compensating action not called again — the runner undoes each step at most once")
			r.mu.Unlock()
			recorded = append(recorded, row)
			continue
		}
		if step.Compensate == nil {
			r.compensated[i] = true
			row := r.recordLocked(i, step.Name, SagaPhaseCompensate, SagaStepNothingToUndo,
				"the step declared no compensating action — an externalized fact, placed last on purpose")
			r.mu.Unlock()
			recorded = append(recorded, row)
			continue
		}
		if r.undoing[i] {
			// Another pass is mid-callback on this step: it owns the
			// call and the row. A compensation that failed there stays
			// retryable by a later pass.
			r.mu.Unlock()
			continue
		}
		r.undoing[i] = true
		r.mu.Unlock()

		err := step.Compensate(undoCtx)

		r.mu.Lock()
		r.undoing[i] = false
		if err != nil {
			row := r.recordLocked(i, step.Name, SagaPhaseCompensate, SagaStepCompensationFailed, err.Error())
			r.mu.Unlock()
			recorded = append(recorded, row)
			continue
		}
		r.compensated[i] = true
		row := r.recordLocked(i, step.Name, SagaPhaseCompensate, SagaStepCompensated, "")
		r.mu.Unlock()
		recorded = append(recorded, row)
	}
	return recorded
}

// recordLocked appends one ledger row (caller holds r.mu) and answers
// the value copy that was recorded.
func (r *SagaRunner) recordLocked(index int, name string, phase SagaPhase, outcome SagaStepOutcome, errText string) SagaStepResult {
	row := SagaStepResult{Index: index, Name: name, Action: phase, Outcome: outcome, Err: errText}
	r.outcome.Steps = append(r.outcome.Steps, row)
	return row
}

// snapshotLocked answers a defensive copy of the recorded outcome with
// the two headline counts recounted from the rows it carries — the
// caller's copy cannot mutate the runner's ledger, and its counts can
// never drift from the rows beside them (a later Compensate appends
// rows, and the next snapshot counts them).
func (r *SagaRunner) snapshotLocked() SagaOutcome {
	snapshot := r.outcome
	snapshot.Steps = append([]SagaStepResult(nil), r.outcome.Steps...)
	snapshot.StepsExecuted = 0
	snapshot.StepsCompensated = 0
	for _, row := range snapshot.Steps {
		switch {
		case row.Action == SagaPhaseForward && row.Outcome == SagaStepExecuted:
			snapshot.StepsExecuted++
		case row.Action == SagaPhaseCompensate && row.Outcome == SagaStepCompensated:
			snapshot.StepsCompensated++
		}
	}
	return snapshot
}
