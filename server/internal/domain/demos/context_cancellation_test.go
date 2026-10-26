package demos

import (
	"reflect"
	"testing"
)

// The choreography is REAL concurrency performed on every call; these
// tests pin the facts the goroutines-and-channels topic's prose claims:
// the deterministic shape (counts, ids, causes, recorded order) and the
// hygiene ledger — so a refactor that breaks the choreography fails HERE
// first, next to the claim.

func TestContextCancellationChoreographyMatchesItsConfiguration(t *testing.T) {
	setup := BuildContextCancellationPayload().Choreography
	if setup.DeadlineBudgetMs != ctxCancelBudgetMs || setup.WorkItemsPerWorker != ctxWorkItemsPerWorker {
		t.Errorf("setup drifted from the constants: %+v", setup)
	}
	if len(setup.Tree) != 3 {
		t.Errorf("tree = %d edges, want 3", len(setup.Tree))
	}
	for _, edge := range setup.Tree {
		if edge.From == "" || edge.To == "" {
			t.Errorf("blank tree edge: %+v", edge)
		}
	}
	if setup.Trigger == "" || setup.Ordering == "" {
		t.Errorf("setup must state its trigger and ordering rule: %+v", setup)
	}
}

func TestContextCancellationWorkPhaseCompletesBeforeCancel(t *testing.T) {
	rows := BuildContextCancellationPayload().WorkPhase
	if len(rows) != len(ctxWorkers) {
		t.Fatalf("work phase = %d rows, want %d", len(rows), len(ctxWorkers))
	}
	for _, row := range rows {
		if row.ItemsProcessed != ctxWorkItemsPerWorker {
			t.Errorf("worker %q processed %d items, want %d — the ack handshake broke",
				row.WorkerID, row.ItemsProcessed, ctxWorkItemsPerWorker)
		}
		if row.Generation < 1 || row.Generation > 2 {
			t.Errorf("worker %q generation = %d, want 1 or 2", row.WorkerID, row.Generation)
		}
	}
}

func TestContextCancellationPropagationReachesEveryDescendant(t *testing.T) {
	rows := BuildContextCancellationPayload().Propagation
	if len(rows) != len(ctxWorkers) {
		t.Fatalf("propagation = %d rows, want %d", len(rows), len(ctxWorkers))
	}
	want := []PropagationRow{
		{WorkerID: "alpha", Generation: 1, DerivedFrom: "parent", ObservedPosition: 1, Cause: "context canceled"},
		{WorkerID: "beta", Generation: 1, DerivedFrom: "parent", ObservedPosition: 2, Cause: "context canceled"},
		{WorkerID: "gamma", Generation: 2, DerivedFrom: "alpha", ObservedPosition: 3, Cause: "context canceled"},
	}
	for i, row := range rows {
		if row != want[i] {
			t.Errorf("propagation row %d = %+v, want %+v", i, row, want[i])
		}
	}
}

func TestContextCancellationDeadlineBeatFiresOnItsOwn(t *testing.T) {
	beat := BuildContextCancellationPayload().DeadlineBeat
	if beat.BudgetMs != ctxDeadlineBeatMs || beat.CancelCalled {
		t.Errorf("deadline beat must be the never-cancelled one: %+v", beat)
	}
	if beat.Outcome != "deadline exceeded" || beat.Cause != "context deadline exceeded" {
		t.Errorf("deadline beat = %+v, want the deadline's own cause", beat)
	}
	if !beat.DeadlineExceeded {
		t.Error("errors.Is(ctx.Err(), context.DeadlineExceeded) must hold in the deadline beat")
	}
}

func TestContextCancellationLedgerBalances(t *testing.T) {
	ledger := BuildContextCancellationPayload().Ledger
	if ledger.GoroutinesLaunched != 4 || ledger.GoroutinesJoined != 4 {
		t.Errorf("goroutine ledger = %+v, want 4 launched and 4 joined", ledger)
	}
	if ledger.ContextsCreated != 5 || ledger.ContextsCancelled != 5 {
		t.Errorf("context ledger = %+v, want 5 created and 5 cancelled", ledger)
	}
	if ledger.Outcome != "no goroutines leaked, every context cancelled" {
		t.Errorf("ledger outcome drifted: %q", ledger.Outcome)
	}
}

func TestContextCancellationPayloadIsPure(t *testing.T) {
	first, second := BuildContextCancellationPayload(), BuildContextCancellationPayload()
	if !reflect.DeepEqual(first, second) {
		t.Error("builder must be shape-deterministic: two calls returned different payloads")
	}
}
