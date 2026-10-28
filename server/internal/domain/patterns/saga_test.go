package patterns

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"testing"
)

// The saga machinery tested directly: forward-until-failure with reverse
// partial compensation, cancellation that triggers an undo pass which
// survives the dead context, idempotent re-compensation recorded as
// no-ops, compensation failures that never strand earlier undos, the
// typed trigger error, one-shot Run semantics, the empty and nil-step
// edge cases, and concurrent drives staying race-clean. No clock and no
// randomness participate anywhere — every outcome here is a pure
// function of the steps' own behavior.

// sagaWorld is a minimal observable outside world: every successful
// action appends one line, so the test can assert the ORDER effects
// actually happened in — forward order forward, undo order reverse.
type sagaWorld struct {
	log []string
}

func (w *sagaWorld) do(line string) error {
	w.log = append(w.log, line)
	return nil
}

// errCarrierRefused is a step's own sentinel error, used to prove the
// trigger wraps the action's error reachably through errors.Is.
var errCarrierRefused = errors.New("carrier: no pickup window this week")

// newOrderSaga builds the classic order-fulfillment saga over a fresh
// world: reserve inventory, charge the card, book shipment, then the
// non-compensatable confirmation last. failAt injects a failure by step
// index (-1 = every step succeeds; the failing action refuses BEFORE
// producing its effect); cancelAt makes the step's forward action
// cancel sagaCtx after doing its work (-1 = never).
func newOrderSaga(failAt, cancelAt int) (*SagaRunner, *sagaWorld, context.Context, context.CancelFunc) {
	world := &sagaWorld{}
	sagaCtx, cancel := context.WithCancel(context.Background())
	step := func(index int, name, forwardLine, undoLine string, hasUndo bool) SagaStep {
		forward := func(ctx context.Context) error {
			if index == failAt {
				return errCarrierRefused
			}
			if err := world.do(forwardLine); err != nil {
				return err
			}
			if index == cancelAt {
				cancel()
			}
			return nil
		}
		compensate := SagaAction(nil)
		if hasUndo {
			compensate = func(ctx context.Context) error {
				// A compensating action that honored a cancelled context
				// would record it — the undo pass must not let that
				// happen (the WithoutCancel proof).
				if err := ctx.Err(); err != nil {
					return fmt.Errorf("undo saw a dead context: %w", err)
				}
				return world.do(undoLine)
			}
		}
		return SagaStep{Name: name, Forward: forward, Compensate: compensate}
	}
	steps := []SagaStep{
		step(0, "reserve-inventory", "inventory: reserved 3 units", "inventory: released 3 units", true),
		step(1, "charge-card", "payments: charged 4200 cents", "payments: refunded 4200 cents", true),
		step(2, "book-shipment", "shipping: booked pickup", "shipping: cancelled pickup", true),
		step(3, "send-confirmation", "email: confirmation queued", "", false),
	}
	return NewSagaRunner(steps...), world, sagaCtx, cancel
}

// ledgerOutcomes renders a ledger's outcome sequence for exact-order
// assertions.
func ledgerOutcomes(rows []SagaStepResult) []string {
	out := make([]string, 0, len(rows))
	for _, row := range rows {
		out = append(out, fmt.Sprintf("%d:%s/%s", row.Index, row.Action, row.Outcome))
	}
	return out
}

func TestSagaRunCompletesWhenEveryStepCommits(t *testing.T) {
	runner, world, ctx, cancel := newOrderSaga(-1, -1)
	defer cancel()

	outcome := runner.Run(ctx)

	if outcome.Status != SagaStatusCompleted {
		t.Fatalf("status = %q, want completed", outcome.Status)
	}
	if outcome.Err != nil {
		t.Fatalf("a completed saga carries no trigger error, got %v", outcome.Err)
	}
	want := []string{
		"0:forward/executed", "1:forward/executed", "2:forward/executed", "3:forward/executed",
	}
	if got := ledgerOutcomes(outcome.Steps); !reflect.DeepEqual(got, want) {
		t.Errorf("ledger = %v, want %v", got, want)
	}
	if outcome.StepsExecuted != 4 || outcome.StepsCompensated != 0 {
		t.Errorf("counts = %d executed, %d compensated; want 4, 0",
			outcome.StepsExecuted, outcome.StepsCompensated)
	}
	if len(world.log) != 4 {
		t.Errorf("world log = %v, want exactly the four forward effects", world.log)
	}
}

func TestSagaRunIsDeterministicFromItsSteps(t *testing.T) {
	firstRunner, _, firstCtx, cancelFirst := newOrderSaga(2, -1)
	secondRunner, _, secondCtx, cancelSecond := newOrderSaga(2, -1)
	defer cancelFirst()
	defer cancelSecond()

	first := firstRunner.Run(firstCtx)
	second := secondRunner.Run(secondCtx)

	if first.Status != second.Status || !reflect.DeepEqual(first.Steps, second.Steps) {
		t.Error("same steps must produce identical ledgers")
	}
	if first.StepsExecuted != second.StepsExecuted || first.StepsCompensated != second.StepsCompensated {
		t.Error("counts drifted between identical runs")
	}
}

func TestSagaFailureCompensatesInReverseAndSkipsTheRest(t *testing.T) {
	runner, world, ctx, cancel := newOrderSaga(2, -1) // book-shipment fails
	defer cancel()

	outcome := runner.Run(ctx)

	if outcome.Status != SagaStatusCompensated {
		t.Fatalf("status = %q, want compensated", outcome.Status)
	}
	want := []string{
		"0:forward/executed", "1:forward/executed",
		"2:forward/failed", "3:forward/skipped",
		"1:compensate/compensated", "0:compensate/compensated",
	}
	if got := ledgerOutcomes(outcome.Steps); !reflect.DeepEqual(got, want) {
		t.Fatalf("ledger = %v, want %v", got, want)
	}
	// The world observed the undo in REVERSE order; the failed step
	// refused before producing any effect (no booking line) and was
	// never undone, and the skipped step never ran at all.
	wantLog := []string{
		"inventory: reserved 3 units",
		"payments: charged 4200 cents",
		"payments: refunded 4200 cents",
		"inventory: released 3 units",
	}
	if !reflect.DeepEqual(world.log, wantLog) {
		t.Errorf("world log = %v, want %v", world.log, wantLog)
	}
	if outcome.StepsExecuted != 2 || outcome.StepsCompensated != 2 {
		t.Errorf("counts = %d executed, %d compensated; want 2, 2",
			outcome.StepsExecuted, outcome.StepsCompensated)
	}
}

func TestSagaTriggerErrorCarriesStepIdentity(t *testing.T) {
	runner, _, ctx, cancel := newOrderSaga(2, -1)
	defer cancel()

	outcome := runner.Run(ctx)

	var stepErr *SagaStepError
	if !errors.As(outcome.Err, &stepErr) {
		t.Fatalf("trigger = %v, want a *SagaStepError", outcome.Err)
	}
	if stepErr.Index != 2 || stepErr.StepName != "book-shipment" {
		t.Errorf("trigger identity = %d (%s), want 2 (book-shipment)", stepErr.Index, stepErr.StepName)
	}
	// The action's own sentinel stays reachable through the wrapper —
	// callers test for THEIR errors, not the saga's strings.
	if !errors.Is(outcome.Err, errCarrierRefused) {
		t.Errorf("errors.Is(trigger, errCarrierRefused) = false, want the action's error wrapped")
	}
	var stepErrAgain *SagaStepError
	if !errors.As(fmt.Errorf("wrapped: %w", outcome.Err), &stepErrAgain) {
		t.Error("errors.As must find the step error through wrapping")
	}
}

func TestSagaCancellationMidRunTriggersCompensationThatSurvives(t *testing.T) {
	runner, world, ctx, cancel := newOrderSaga(-1, 1) // charge-card cancels after charging
	defer cancel()

	outcome := runner.Run(ctx)

	if outcome.Status != SagaStatusCancelled {
		t.Fatalf("status = %q, want cancelled", outcome.Status)
	}
	if !errors.Is(outcome.Err, context.Canceled) {
		t.Errorf("trigger = %v, want context.Canceled reachable", outcome.Err)
	}
	want := []string{
		"0:forward/executed", "1:forward/executed",
		"2:forward/skipped", "3:forward/skipped",
		"1:compensate/compensated", "0:compensate/compensated",
	}
	if got := ledgerOutcomes(outcome.Steps); !reflect.DeepEqual(got, want) {
		t.Fatalf("ledger = %v, want %v", got, want)
	}
	// The undo pass ran although the saga's context was dead: both
	// compensations succeeded — a compensating action that saw the
	// cancellation would have failed with "undo saw a dead context".
	wantLog := []string{
		"inventory: reserved 3 units",
		"payments: charged 4200 cents",
		"payments: refunded 4200 cents",
		"inventory: released 3 units",
	}
	if !reflect.DeepEqual(world.log, wantLog) {
		t.Errorf("world log = %v, want the charge undone after the cancellation: %v", world.log, wantLog)
	}
}

func TestSagaCancellationBeforeRunSkipsEverything(t *testing.T) {
	runner, world, _, cancel := newOrderSaga(-1, -1)
	defer cancel()
	ctx, cancelRun := context.WithCancel(context.Background())
	cancelRun()

	outcome := runner.Run(ctx)

	if outcome.Status != SagaStatusCancelled {
		t.Fatalf("status = %q, want cancelled", outcome.Status)
	}
	want := []string{
		"0:forward/skipped", "1:forward/skipped", "2:forward/skipped", "3:forward/skipped",
	}
	if got := ledgerOutcomes(outcome.Steps); !reflect.DeepEqual(got, want) {
		t.Errorf("ledger = %v, want %v", got, want)
	}
	if outcome.StepsExecuted != 0 || outcome.StepsCompensated != 0 {
		t.Errorf("counts = %d, %d; want 0, 0 (nothing executed, nothing to undo)",
			outcome.StepsExecuted, outcome.StepsCompensated)
	}
	if len(world.log) != 0 {
		t.Errorf("world log = %v, want no effects at all", world.log)
	}
}

func TestSagaCompensateIsIdempotentAndRecordsNoOps(t *testing.T) {
	runner, world, ctx, cancel := newOrderSaga(2, -1)
	defer cancel()
	outcome := runner.Run(ctx)
	undoCalls := len(world.log)

	replay := runner.Compensate(context.Background())

	if len(replay) != 2 {
		t.Fatalf("replay rows = %v, want one row per compensated step", ledgerOutcomes(replay))
	}
	for _, row := range replay {
		if row.Outcome != SagaStepAlreadyCompensated {
			t.Errorf("replay row %d = %q, want already-compensated", row.Index, row.Outcome)
		}
	}
	if len(world.log) != undoCalls {
		t.Errorf("re-compensation called actions again: log grew from %d to %d lines",
			undoCalls, len(world.log))
	}
	// The no-op rows joined the same ledger — the replay shows the same
	// steps, reverse order, recorded as already-compensated.
	total := append(ledgerOutcomes(outcome.Steps), ledgerOutcomes(replay)...)
	if tail := total[len(total)-2:]; !reflect.DeepEqual(tail, []string{
		"1:compensate/already-compensated", "0:compensate/already-compensated",
	}) {
		t.Errorf("replay rows = %v, want reverse-order already-compensated no-ops", tail)
	}
}

func TestSagaCompensateAfterCompletionUndoesIt(t *testing.T) {
	runner, world, ctx, cancel := newOrderSaga(-1, -1)
	defer cancel()
	runner.Run(ctx)

	rows := runner.Compensate(context.Background())

	// The walk runs in reverse over every EXECUTED step, so it starts
	// at the confirmation — recorded as nothing-to-undo — before the
	// three real compensations.
	want := []string{
		"3:compensate/nothing-to-undo",
		"2:compensate/compensated", "1:compensate/compensated", "0:compensate/compensated",
	}
	if got := ledgerOutcomes(rows); !reflect.DeepEqual(got, want) {
		t.Errorf("compensate-after-complete rows = %v, want %v", got, want)
	}
	// The confirmation step executed but has nothing to undo — recorded,
	// its compensating action never called.
	foundNoUndo := false
	for _, row := range rows {
		if row.Index == 3 && row.Outcome == SagaStepNothingToUndo {
			foundNoUndo = true
		}
	}
	if !foundNoUndo {
		t.Errorf("rows = %v, want a nothing-to-undo row for the non-compensatable step", ledgerOutcomes(rows))
	}
	// The confirmation line appears exactly once — from its forward run.
	// The undo pass never invents an effect for it.
	emails := 0
	for _, entry := range world.log {
		if entry == "email: confirmation queued" {
			emails++
		}
	}
	if emails != 1 {
		t.Errorf("the confirmation appears %d times in %v, want exactly once", emails, world.log)
	}
	// A later snapshot answers the grown ledger WITH matching counts —
	// the compensation rows are counted beside the rows that carry them.
	final := runner.Run(ctx)
	if final.StepsCompensated != 3 {
		t.Errorf("final snapshot StepsCompensated = %d, want 3 (recounted from its own rows)",
			final.StepsCompensated)
	}
	if final.StepsExecuted != 4 || len(final.Steps) != 8 {
		t.Errorf("final snapshot = %d executed, %d rows; want 4 executed, 8 rows",
			final.StepsExecuted, len(final.Steps))
	}
}

func TestSagaCompensationFailureDoesNotStopEarlierUndos(t *testing.T) {
	world := &sagaWorld{}
	steps := []SagaStep{
		{Name: "first", Forward: func(context.Context) error { return world.do("first done") },
			Compensate: func(context.Context) error { return world.do("first undone") }},
		{Name: "second", Forward: func(context.Context) error { return world.do("second done") },
			Compensate: func(context.Context) error {
				world.do("second undo attempted")
				return errors.New("refund gateway unreachable")
			}},
		{Name: "third", Forward: func(context.Context) error {
			return errors.New("third failed")
		}, Compensate: func(context.Context) error { return world.do("third undone") }},
	}
	runner := NewSagaRunner(steps...)

	outcome := runner.Run(context.Background())

	want := []string{
		"0:forward/executed", "1:forward/executed", "2:forward/failed",
		"1:compensate/compensation-failed", "0:compensate/compensated",
	}
	if got := ledgerOutcomes(outcome.Steps); !reflect.DeepEqual(got, want) {
		t.Fatalf("ledger = %v, want %v", got, want)
	}
	if outcome.StepsCompensated != 1 {
		t.Errorf("StepsCompensated = %d, want 1 — a failed undo must not count as compensated",
			outcome.StepsCompensated)
	}
	failedRow := outcome.Steps[3]
	if failedRow.Err != "refund gateway unreachable" {
		t.Errorf("compensation-failed row err = %q, want the undo's own error text", failedRow.Err)
	}
	if world.log[len(world.log)-1] != "first undone" {
		t.Errorf("the undo above the failure must still run, log = %v", world.log)
	}
}

func TestSagaRunWithNoSteps(t *testing.T) {
	for name, runner := range map[string]*SagaRunner{
		"no steps":    NewSagaRunner(),
		"empty slice": NewSagaRunner([]SagaStep{}...),
	} {
		outcome := runner.Run(context.Background())
		if outcome.Status != SagaStatusCompleted {
			t.Errorf("%s: status = %q, want completed (vacuously)", name, outcome.Status)
		}
		if len(outcome.Steps) != 0 {
			t.Errorf("%s: ledger = %v, want empty", name, outcome.Steps)
		}
	}
}

func TestSagaRunWithNoStepsAndADeadContextStillCompletes(t *testing.T) {
	// Cancellation is observed at step boundaries; a saga with no steps
	// has no boundary to observe it at — the run completes vacuously
	// with an empty ledger.
	runner := NewSagaRunner()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	outcome := runner.Run(ctx)

	if outcome.Status != SagaStatusCompleted || len(outcome.Steps) != 0 {
		t.Errorf("outcome = %q with %d rows; want completed with 0 rows", outcome.Status, len(outcome.Steps))
	}
}

func TestSagaNilForwardPanicsWithTheStepName(t *testing.T) {
	runner := NewSagaRunner(SagaStep{Name: "the-nameless-step", Forward: nil})

	defer func() {
		recovered := recover()
		if recovered == nil {
			t.Error("a nil Forward action must panic — it is a programmer bug")
		} else if text := fmt.Sprint(recovered); !strings.Contains(text, "the-nameless-step") {
			t.Errorf("panic %q must name the offending step", text)
		}
	}()
	runner.Run(context.Background())
}

func TestSagaRunIsOneShot(t *testing.T) {
	runner, world, ctx, cancel := newOrderSaga(2, -1)
	defer cancel()
	first := runner.Run(ctx)
	worldLen := len(world.log)

	second := runner.Run(ctx)

	if second.Status != first.Status || !reflect.DeepEqual(second.Steps, first.Steps) {
		t.Error("a second Run must answer the recorded outcome, not re-execute")
	}
	if len(world.log) != worldLen {
		t.Errorf("the world observed %d new effects on the second Run", len(world.log)-worldLen)
	}
}

func TestSagaCompensateBeforeRunRecordsNothing(t *testing.T) {
	runner, _, _, cancel := newOrderSaga(-1, -1)
	defer cancel()

	if rows := runner.Compensate(context.Background()); len(rows) != 0 {
		t.Errorf("rows = %v, want none — nothing has executed", ledgerOutcomes(rows))
	}
}

func TestSagaConcurrentDrivesStayRaceClean(t *testing.T) {
	runner, world, ctx, cancel := newOrderSaga(2, -1)
	defer cancel()

	var wg sync.WaitGroup
	for worker := 0; worker < 8; worker++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			runner.Run(ctx)
			runner.Compensate(context.Background())
			runner.Compensate(context.Background())
		}()
	}
	wg.Wait()

	// One worker's Run drove the saga (book-shipment refused after the
	// charge); every other drive answered the recorded outcome. The
	// world must show each effect exactly once — the charge once, its
	// refund once, no matter how many compensations were attempted
	// concurrently.
	count := func(line string) int {
		seen := 0
		for _, entry := range world.log {
			if entry == line {
				seen++
			}
		}
		return seen
	}
	if charges := count("payments: charged 4200 cents"); charges != 1 {
		t.Errorf("the charge ran %d times under 8 concurrent drives, want exactly 1", charges)
	}
	if refunds := count("payments: refunded 4200 cents"); refunds != 1 {
		t.Errorf("the refund ran %d times under concurrent re-compensations, want exactly 1 (idempotent)", refunds)
	}
}
