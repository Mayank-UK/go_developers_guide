// Topic 06 goroutines-and-channels — the conc:context demo: a REAL
// context-cancellation choreography, performed on every request. A parent
// context carries an armed deadline (the budget), three workers derived
// from it do real work over channels, then the parent is cancelled and the
// payload records the propagation as structure — which derivation level
// observed Done(), in what recorded order, and for which cause — never as
// timings. A separate beat lets a deadline fire on its own so the payload
// can show both ways Done() closes: explicit cancellation and expiry.
// Counts, ids, causes, and the recorded order are deterministic by
// construction: the work phase is acknowledged item by item before the
// cancel, and observation is recorded behind per-generation barriers with
// ids sorted inside a generation (within one generation the scheduler
// picks freely — the guarantee taught is that EVERY descendant observes
// Done(), not who noticed first).
package demos

import (
	"context"
	"errors"
	"sort"
	"sync"
	"time"
)

// The choreography's fixed configuration — not observation. The parent's
// deadline is armed at 400ms but the explicit cancel always lands long
// before it (the builder returns in milliseconds), so the trigger and the
// ledger never vary; the deadline beat really waits out its 25ms because
// nothing cancels it.
const (
	ctxCancelBudgetMs     = 400
	ctxDeadlineBeatMs     = 25
	ctxWorkItemsPerWorker = 2
)

// ctxTreeEdges is the propagation tree, fixed: parent → alpha, parent →
// beta, alpha → gamma. Cancelling the parent must reach all three —
// including gamma, which was never derived from the parent DIRECTLY but
// from a child of it.
var ctxTreeEdges = []TreeEdge{
	{From: "parent", To: "alpha"},
	{From: "parent", To: "beta"},
	{From: "alpha", To: "gamma"},
}

// ctxWorkerDesc describes one worker's place in the tree.
type ctxWorkerDesc struct {
	ID          string
	Generation  int
	DerivedFrom string
}

// ctxWorkers is the roster: two children of the parent, one grandchild.
var ctxWorkers = []ctxWorkerDesc{
	{ID: "alpha", Generation: 1, DerivedFrom: "parent"},
	{ID: "beta", Generation: 1, DerivedFrom: "parent"},
	{ID: "gamma", Generation: 2, DerivedFrom: "alpha"},
}

// TreeEdge is one derivation edge in the context tree: To was derived from
// From, so cancelling From eventually closes To's Done channel too.
type TreeEdge struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// ChoreographyFacts states the choreography's configuration: the armed
// deadline budget, the work each worker completes before the cancel, what
// triggered cancellation, the derivation tree, and how the recorded order
// was produced.
type ChoreographyFacts struct {
	DeadlineBudgetMs   int        `json:"deadlineBudgetMs"`
	WorkItemsPerWorker int        `json:"workItemsPerWorker"`
	Trigger            string     `json:"trigger"`
	Tree               []TreeEdge `json:"tree"`
	Ordering           string     `json:"ordering"`
}

// WorkRow is one worker's completed work before cancellation landed.
type WorkRow struct {
	WorkerID       string `json:"workerId"`
	Generation     int    `json:"generation"`
	ItemsProcessed int    `json:"itemsProcessed"`
}

// PropagationRow is one worker's observation of cancellation: the position
// at which the choreography RECORDED it, and the cause it read off
// ctx.Err() when Done() fired.
type PropagationRow struct {
	WorkerID         string `json:"workerId"`
	Generation       int    `json:"generation"`
	DerivedFrom      string `json:"derivedFrom"`
	ObservedPosition int    `json:"observedPosition"`
	Cause            string `json:"cause"`
}

// DeadlineOutcome is the beat where nobody cancels: the deadline fires on
// its own and the worker's select takes the Done arm with the deadline's
// cause, verified with errors.Is the way production code checks it.
type DeadlineOutcome struct {
	BudgetMs         int    `json:"budgetMs"`
	CancelCalled     bool   `json:"cancelCalled"`
	Outcome          string `json:"outcome"`
	Cause            string `json:"cause"`
	DeadlineExceeded bool   `json:"deadlineExceeded"`
}

// CancellationLedger is the hygiene accounting: every goroutine launched
// was joined before the payload was built, and every context created was
// cancelled — the leak-free books a cancellation choreography must balance.
type CancellationLedger struct {
	GoroutinesLaunched int    `json:"goroutinesLaunched"`
	GoroutinesJoined   int    `json:"goroutinesJoined"`
	ContextsCreated    int    `json:"contextsCreated"`
	ContextsCancelled  int    `json:"contextsCancelled"`
	Outcome            string `json:"outcome"`
}

// ContextCancellationPayload is the wire shape of GET /api/demo/conc/context
// (camelCase JSON) — the goroutines-and-channels topic's cancellation demo.
type ContextCancellationPayload struct {
	Choreography ChoreographyFacts  `json:"choreography"`
	WorkPhase    []WorkRow          `json:"workPhase"`
	Propagation  []PropagationRow   `json:"propagation"`
	DeadlineBeat DeadlineOutcome    `json:"deadlineBeat"`
	Ledger       CancellationLedger `json:"ledger"`
}

// ctxObservation is one worker's cancellation report traveling the
// observation channel.
type ctxObservation struct {
	workerID string
	cause    string
}

// ctxRunWorker is the demo's whole worker loop: real work over the work
// channel (one ack per item, so the orchestrator knows the item landed),
// then the cancellation race — select on ctx.Done() versus more work. When
// Done() fires the worker reports its cause and RETURNS: observing
// cancellation means leaving the loop, not flag-checking inside it.
func ctxRunWorker(ctx context.Context, work <-chan string, acks chan<- string, observed chan<- ctxObservation, id string) {
	for {
		select {
		case <-ctx.Done():
			observed <- ctxObservation{workerID: id, cause: ctx.Err().Error()}
			return
		case item := <-work:
			acks <- item
		}
	}
}

// ctxWorkPhase hands each worker its items over an UNBUFFERED channel and
// waits for the per-item ack before sending the next — which is why the
// counts are deterministic: by the time the parent is cancelled, every
// worker has processed exactly its share and is parked back in the select.
func ctxWorkPhase(works map[string]chan string, acks chan string) []WorkRow {
	rows := make([]WorkRow, 0, len(ctxWorkers))
	for _, worker := range ctxWorkers {
		for i := 1; i <= ctxWorkItemsPerWorker; i++ {
			works[worker.ID] <- worker.ID + "-" + string(rune('0'+i))
			<-acks
		}
		rows = append(rows, WorkRow{
			WorkerID:       worker.ID,
			Generation:     worker.Generation,
			ItemsProcessed: ctxWorkItemsPerWorker,
		})
	}
	return rows
}

// ctxCollectObservations turns the buffered observation reports into the
// recorded order: generation barriers first (the caller already waited on
// both WaitGroups), ids sorted within a generation, positions assigned in
// that order. Each row keeps ITS OWN observation's worker id and cause —
// the sort decides who is LISTED first, never who reported first.
func ctxCollectObservations(observed <-chan ctxObservation, byID map[string]ctxWorkerDesc) []PropagationRow {
	reports := make(map[string]ctxObservation, len(ctxWorkers))
	for range ctxWorkers {
		obs := <-observed
		reports[obs.workerID] = obs
	}
	rows := make([]PropagationRow, 0, len(ctxWorkers))
	position := 1
	for _, group := range [][]string{{"alpha", "beta"}, {"gamma"}} {
		sorted := append([]string(nil), group...)
		sort.Strings(sorted)
		for _, id := range sorted {
			desc := byID[id]
			rows = append(rows, PropagationRow{
				WorkerID:         reports[id].workerID,
				Generation:       desc.Generation,
				DerivedFrom:      desc.DerivedFrom,
				ObservedPosition: position,
				Cause:            reports[id].cause,
			})
			position++
		}
	}
	return rows
}

// ctxDeadlineBeat arms a short deadline and deliberately never cancels:
// the worker's select has a work arm, but no work ever arrives, so the
// deadline closes Done() on its own and errors.Is confirms the cause the
// way production code must check it.
func ctxDeadlineBeat() DeadlineOutcome {
	ctx, cancel := context.WithTimeout(context.Background(), ctxDeadlineBeatMs*time.Millisecond)
	defer cancel()

	result := make(chan string, 1)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		work := make(chan string)
		select {
		case <-ctx.Done():
			result <- ctx.Err().Error()
		case <-work:
			// Unreachable: no item is ever sent — the deadline wins.
		}
	}()
	wg.Wait()
	cause := <-result
	return DeadlineOutcome{
		BudgetMs:         ctxDeadlineBeatMs,
		CancelCalled:     false,
		Outcome:          "deadline exceeded",
		Cause:            cause,
		DeadlineExceeded: errors.Is(ctx.Err(), context.DeadlineExceeded),
	}
}

// BuildContextCancellationPayload performs the choreography and returns its
// transcript. It is deterministic in shape — same configuration, same
// counts, same recorded order, same causes on every call — and leak-free by
// construction: every goroutine is joined and every context cancelled
// before the payload exists.
func BuildContextCancellationPayload() ContextCancellationPayload {
	parent, cancelParent := context.WithTimeout(context.Background(), ctxCancelBudgetMs*time.Millisecond)
	defer cancelParent()

	// One derivation per tree edge, each with its own deferred cancel —
	// cancelling the parent propagates, but every With* still owes its own
	// release (go vet's lostcancel check enforces the habit).
	alphaCtx, cancelAlpha := context.WithCancel(parent)
	defer cancelAlpha()
	betaCtx, cancelBeta := context.WithCancel(parent)
	defer cancelBeta()
	gammaCtx, cancelGamma := context.WithCancel(alphaCtx)
	defer cancelGamma()
	ctxByWorker := map[string]context.Context{
		"alpha": alphaCtx,
		"beta":  betaCtx,
		"gamma": gammaCtx,
	}

	works := make(map[string]chan string, len(ctxWorkers))
	for _, worker := range ctxWorkers {
		works[worker.ID] = make(chan string)
	}
	acks := make(chan string)
	observed := make(chan ctxObservation, len(ctxWorkers))

	var genOne, genTwo sync.WaitGroup
	byID := make(map[string]ctxWorkerDesc, len(ctxWorkers))
	for _, worker := range ctxWorkers {
		byID[worker.ID] = worker
		wg := &genOne
		if worker.Generation == 2 {
			wg = &genTwo
		}
		wg.Add(1)
		go func(worker ctxWorkerDesc, wg *sync.WaitGroup) {
			defer wg.Done()
			ctxRunWorker(ctxByWorker[worker.ID], works[worker.ID], acks, observed, worker.ID)
		}(worker, wg)
	}

	workRows := ctxWorkPhase(works, acks)

	// The trigger: an explicit cancel of the parent, long before the armed
	// 400ms deadline. Everything derived from parent — directly or through
	// alpha — must observe Done() with cause "context canceled".
	cancelParent()

	genOne.Wait()
	genTwo.Wait()
	propagationRows := ctxCollectObservations(observed, byID)
	deadlineBeat := ctxDeadlineBeat()

	return ContextCancellationPayload{
		Choreography: ChoreographyFacts{
			DeadlineBudgetMs:   ctxCancelBudgetMs,
			WorkItemsPerWorker: ctxWorkItemsPerWorker,
			Trigger:            "explicit cancel of the parent, before the armed deadline",
			Tree:               append([]TreeEdge(nil), ctxTreeEdges...),
			Ordering: "recorded behind per-generation barriers, ids sorted within a generation — " +
				"within one generation the scheduler picks; the guarantee is that every descendant observes Done()",
		},
		WorkPhase:    workRows,
		Propagation:  propagationRows,
		DeadlineBeat: deadlineBeat,
		Ledger: CancellationLedger{
			GoroutinesLaunched: len(ctxWorkers) + 1, // three workers + the deadline beat's
			GoroutinesJoined:   len(ctxWorkers) + 1,
			ContextsCreated:    5, // parent + three derivations + the deadline beat's
			ContextsCancelled:  5, // every one: the trigger plus four deferred hygiene cancels
			Outcome:            "no goroutines leaked, every context cancelled",
		},
	}
}
