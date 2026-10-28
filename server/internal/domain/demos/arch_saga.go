// Topic 43 advanced-architecture's saga half: the compensation-ledger
// demo. The machinery is the REAL internal/domain/patterns saga — this
// file invents no orchestration. Every request drives three fresh
// patterns.SagaRunner instances through the SAME fixed order-fulfillment
// script (reserve inventory, charge the card, book shipment, send the
// non-compensatable confirmation) against a deterministic in-memory
// world whose log records each effect as it happens: a success run, a
// failure run (step 2 refuses — deterministic injection by index, no
// clock, no retry timers), and a cancellation run (the payer's deadline
// fires inside step 1, proving the undo pass survives the dead context
// because it runs on context.WithoutCancel). The failure run then
// replays Compensate once more, so the payload carries the machinery's
// own idempotency record: already-compensated no-ops, never a second
// call. Every ledger row, trigger, world line, and count below was
// observed from the run that served it — nothing is narrated into
// existence, and no wall-clock time or randomness appears anywhere on
// the wire.
package demos

import (
	"context"
	"fmt"
	"sort"

	"guide/internal/domain/patterns"
)

// ArchSagaPath is the demo area's endpoint — registered on the real mux
// by the composition root.
const ArchSagaPath = "/api/demo/arch/saga"

// The fixed script's knobs: failure injects at the shipment step,
// cancellation injects inside the charge step (after the charge lands —
// the effect exists, so the undo has real work to do). Step indexes are
// 0-based, matching patterns.SagaStepResult.Index and the machinery's
// trigger error.
const (
	archSagaFailureStep = 2
	archSagaCancelStep  = 1
)

// errArchSagaCarrierRefused is the shipment step's refusal — the
// outside world saying no, exactly as a carrier API would. It reaches
// the payload only through the machinery's typed trigger error.
var errArchSagaCarrierRefused = fmt.Errorf("carrier: no ground pickup window for order o-1001")

// archSagaScriptStep is one row of the fixed choreography: the step's
// name, the world line its forward action appends, and the world line
// its compensating action appends ("" — no compensating action, an
// externalized fact that belongs last).
type archSagaScriptStep struct {
	name   string
	does   string
	undoes string
}

// archSagaScript is the fixed four-step order-fulfillment choreography
// every run drives. The lines are the WORLD'S vocabulary: the steps
// panel serves them as the script's contract, and the run panels serve
// them again only where the world actually logged them.
var archSagaScript = []archSagaScriptStep{
	{name: "reserve-inventory",
		does:   "inventory: reserved 3× gopher-plush for order o-1001",
		undoes: "inventory: released 3× gopher-plush for order o-1001"},
	{name: "charge-card",
		does:   "payments: charged 4200 cents to tok_visa_4242 for order o-1001",
		undoes: "payments: refunded 4200 cents to tok_visa_4242 for order o-1001"},
	{name: "book-shipment",
		does:   "shipping: booked ground pickup for order o-1001",
		undoes: "shipping: cancelled pickup for order o-1001"},
	{name: "send-confirmation",
		does: "email: confirmation queued for ops@example.com for order o-1001"},
}

// archSagaWorld is the deterministic outside world the steps act on:
// every successful action appends exactly one line, so the world log is
// the ground truth of which effects happened and in which order.
type archSagaWorld struct {
	log []string
}

// archSagaBuildSteps compiles the fixed script into real saga steps
// over a fresh world. failAt (-1 = none) makes the step's forward
// action refuse BEFORE producing its effect; cancelAt (-1 = none) makes
// the step's forward action cancel the saga's context AFTER doing its
// work — both deterministic by construction, no sleeps anywhere.
func archSagaBuildSteps(world *archSagaWorld, cancel context.CancelFunc, failAt, cancelAt int) []patterns.SagaStep {
	steps := make([]patterns.SagaStep, 0, len(archSagaScript))
	for index, scripted := range archSagaScript {
		forward := func(ctx context.Context) error {
			if index == failAt {
				return errArchSagaCarrierRefused
			}
			world.log = append(world.log, scripted.does)
			if index == cancelAt {
				cancel() // the payer service's deadline, injected mid-step
			}
			return nil
		}
		var compensate patterns.SagaAction
		if scripted.undoes != "" {
			compensate = func(ctx context.Context) error {
				// A compensating action that honored a cancelled context
				// would fail here — the undo pass must hand it one with
				// cancellation stripped (the machinery's own rule).
				if err := ctx.Err(); err != nil {
					return fmt.Errorf("undo saw a dead context: %w", err)
				}
				world.log = append(world.log, scripted.undoes)
				return nil
			}
		}
		steps = append(steps, patterns.SagaStep{
			Name: scripted.name, Forward: forward, Compensate: compensate,
		})
	}
	return steps
}

// ── the observed-run shapes ─────────────────────────────────────────

// ArchSagaStepRow is one step of the fixed script as served: its 0-based
// order (matching the machinery's indexes), the effect its forward
// action has on the world, and the effect its compensating action undoes
// ("" — nothing: an externalized fact, placed last on purpose).
type ArchSagaStepRow struct {
	Order   int    `json:"order"`
	Name    string `json:"name"`
	Forward string `json:"forward"`
	Undo    string `json:"undo"`
}

// ArchSagaLedgerRow is one ledger row as served: the recording sequence,
// the step (index · name), the phase, the machinery's outcome label, and
// the detail — the action's own error text for failures, the world line
// the action appended for successes, or the row's fixed honest meaning
// for skips and no-ops.
type ArchSagaLedgerRow struct {
	Seq     int    `json:"seq"`
	Step    string `json:"step"`
	Phase   string `json:"phase"`
	Outcome string `json:"outcome"`
	Detail  string `json:"detail"`
}

// ArchSagaCountRow is one counted observation — counted from the run's
// own ledger, never asserted.
type ArchSagaCountRow struct {
	Label string `json:"label"`
	Value int    `json:"value"`
}

// ArchSagaRunPanel is one observed run: the script that drove it, the
// terminal status and typed trigger, the full ledger in recording
// order, the world's log of effects in the order they happened, the
// counted totals, and — for the failure run — the idempotent
// re-compensation replay the machinery recorded.
type ArchSagaRunPanel struct {
	ID       string              `json:"id"`
	Title    string              `json:"title"`
	Lead     string              `json:"lead"`
	Script   string              `json:"script"`
	Status   string              `json:"status"`
	Trigger  string              `json:"trigger,omitempty"`
	Ledger   []ArchSagaLedgerRow `json:"ledger"`
	WorldLog []string            `json:"worldLog"`
	Counts   []ArchSagaCountRow  `json:"counts"`
	Replay   []ArchSagaLedgerRow `json:"replay,omitempty"`
	Note     string              `json:"note"`
}

// ArchSagaSurfaceRow is one exported id of the real machinery, served
// as data so the demo can point at the exact Go the topic quotes.
type ArchSagaSurfaceRow struct {
	ID   string `json:"id"`
	Kind string `json:"kind"`
	Role string `json:"role"`
}

// ArchSagaPayload is the wire shape of GET /api/demo/arch/saga
// (camelCase JSON).
type ArchSagaPayload struct {
	Mode        string               `json:"mode"`
	Source      string               `json:"source"`
	HonestyNote string               `json:"honestyNote"`
	Steps       []ArchSagaStepRow    `json:"steps"`
	Runs        []ArchSagaRunPanel   `json:"runs"`
	Semantics   []string             `json:"semantics"`
	Surface     []ArchSagaSurfaceRow `json:"surface"`
}

// archSagaRunSpec describes one drive of the script.
type archSagaRunSpec struct {
	id, title, lead, script, note string
	failAt, cancelAt              int
	withReplay                    bool
}

// BuildArchSagaPayload drives the three runs (pure over the fixed
// script): success, injected failure at the shipment step, and
// cancellation injected inside the charge step — plus the failure run's
// idempotent re-compensation replay. Same script in, same bytes out: no
// clock, no randomness, nothing invented.
func BuildArchSagaPayload() ArchSagaPayload {
	success := archSagaDriveRun(archSagaRunSpec{
		id: "success", title: "All four steps commit",
		lead:   "the happy path: every forward action runs in order, nothing is undone",
		script: "four steps, no injected failure — the saga completes",
		note: "the confirmation's line is the last effect the world ever sees: four " +
			"executed rows, zero compensate rows",
		failAt: -1, cancelAt: -1,
	})
	failure := archSagaDriveRun(archSagaRunSpec{
		id: "failure", title: "Step 2 fails: compensated in reverse",
		lead: "book-shipment refuses; the charge and the reservation are undone, newest first",
		script: "failure injected at step 2 — the carrier refusal arrives before any effect, " +
			"deterministic by index",
		note: "the replay below drives Compensate a second time over the same runner: the " +
			"machinery records two already-compensated no-ops and calls nothing — idempotency " +
			"as data",
		failAt: archSagaFailureStep, cancelAt: -1, withReplay: true,
	})
	cancellation := archSagaDriveRun(archSagaRunSpec{
		id: "cancellation", title: "Context cancelled mid-saga",
		lead: "the payer's deadline fires inside step 1; steps 2 and 3 are skipped and the undo pass still runs",
		script: "cancellation injected inside step 1 — the charge lands, then the saga's " +
			"context dies; the runner observes it before step 2",
		note: "the refund and release succeeded although the context was dead: the undo pass " +
			"runs on context.WithoutCancel — values kept, cancellation stripped — so a " +
			"compensating action that checks its ctx (these do) still succeeds",
		failAt: -1, cancelAt: archSagaCancelStep,
	})

	return ArchSagaPayload{
		Mode: "compensation ledger: the real patterns.SagaRunner driven through three observed runs " +
			"— success, injected failure, mid-saga cancellation — over the fixed order-fulfillment " +
			"script; every ledger row below was recorded by the run that served this payload",
		Source: "server/internal/domain/patterns/saga.go",
		HonestyNote: "No-invention pin: every status, trigger, ledger row, world line, and count on " +
			"this payload was observed from the real saga runs that served it — the counts were " +
			"counted from the ledgers, and the replay's no-ops were recorded by the machinery's " +
			"own idempotency check. Failure and cancellation are injected by step index in the " +
			"fixed script; no wall-clock time, randomness, or retry timer participates anywhere, " +
			"so two GETs serve byte-identical bodies",
		Steps:     archSagaStepRows(),
		Runs:      []ArchSagaRunPanel{success, failure, cancellation},
		Semantics: archSagaSemantics(),
		Surface:   archSagaSurfaceRows(),
	}
}

// archSagaDriveRun drives one run of the fixed script through the REAL
// machinery and shapes the panel purely from what came back: the
// outcome's status and trigger, its ledger rows paired with the world
// lines the actions actually appended, and the counts counted from the
// rows.
func archSagaDriveRun(spec archSagaRunSpec) ArchSagaRunPanel {
	world := &archSagaWorld{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	runner := patterns.NewSagaRunner(archSagaBuildSteps(world, cancel, spec.failAt, spec.cancelAt)...)
	outcome := runner.Run(ctx)

	panel := ArchSagaRunPanel{
		ID:       spec.id,
		Title:    spec.title,
		Lead:     spec.lead,
		Script:   spec.script,
		Status:   string(outcome.Status),
		Ledger:   archSagaLedgerRows(outcome.Steps, world),
		WorldLog: append([]string(nil), world.log...),
		Note:     spec.note,
	}
	if outcome.Err != nil {
		panel.Trigger = outcome.Err.Error()
	}
	if spec.withReplay {
		panel.Replay = archSagaReplayRows(runner.Compensate(context.Background()), len(outcome.Steps))
	}
	panel.Counts = archSagaCountRows(outcome.Steps, panel.Replay)
	return panel
}

// archSagaStepRows serves the fixed script as the contract panel — the
// same lines the world logs when the actions run.
func archSagaStepRows() []ArchSagaStepRow {
	rows := make([]ArchSagaStepRow, 0, len(archSagaScript))
	for index, scripted := range archSagaScript {
		rows = append(rows, ArchSagaStepRow{
			Order: index, Name: scripted.name, Forward: scripted.does, Undo: scripted.undoes,
		})
	}
	return rows
}

// archSagaLedgerRows shapes the machinery's rows for the wire. Each
// executed or compensated row consumes the NEXT world line — the action
// appended exactly one, in this order, or the script is broken and
// panics loudly rather than mislabeling a row.
func archSagaLedgerRows(rows []patterns.SagaStepResult, world *archSagaWorld) []ArchSagaLedgerRow {
	served := make([]ArchSagaLedgerRow, 0, len(rows))
	cursor := 0
	nextWorldLine := func() string {
		if cursor >= len(world.log) {
			panic("arch saga: ledger row has no world line — the script desynced")
		}
		line := world.log[cursor]
		cursor++
		return line
	}
	for seq, row := range rows {
		detail := ""
		switch row.Outcome {
		case patterns.SagaStepExecuted, patterns.SagaStepCompensated:
			detail = nextWorldLine()
		case patterns.SagaStepFailed, patterns.SagaStepCompensationFailed:
			detail = row.Err
		case patterns.SagaStepSkipped:
			detail = "never reached — the saga stopped at an earlier step"
		case patterns.SagaStepNothingToUndo:
			detail = "no compensating action — an externalized fact; such steps go last"
		case patterns.SagaStepAlreadyCompensated:
			detail = row.Err
		}
		served = append(served, ArchSagaLedgerRow{
			Seq:   seq + 1,
			Step:  fmt.Sprintf("%d · %s", row.Index, row.Name),
			Phase: string(row.Action), Outcome: string(row.Outcome), Detail: detail,
		})
	}
	return served
}

// archSagaReplayRows shapes the replay pass's rows — seq continues from
// the run's own ledger length so both passes read as one record.
func archSagaReplayRows(rows []patterns.SagaStepResult, afterSeq int) []ArchSagaLedgerRow {
	served := make([]ArchSagaLedgerRow, 0, len(rows))
	for seq, row := range rows {
		served = append(served, ArchSagaLedgerRow{
			Seq:   afterSeq + seq + 1,
			Step:  fmt.Sprintf("%d · %s", row.Index, row.Name),
			Phase: string(row.Action), Outcome: string(row.Outcome), Detail: row.Err,
		})
	}
	return served
}

// archSagaCountRows counts the run's observations from its own rows —
// the replay's no-ops included when it ran.
func archSagaCountRows(rows []patterns.SagaStepResult, replay []ArchSagaLedgerRow) []ArchSagaCountRow {
	counts := map[string]int{}
	for _, row := range rows {
		switch {
		case row.Action == patterns.SagaPhaseForward && row.Outcome == patterns.SagaStepExecuted:
			counts["steps executed"]++
		case row.Action == patterns.SagaPhaseForward && row.Outcome == patterns.SagaStepFailed:
			counts["steps failed"]++
		case row.Action == patterns.SagaPhaseForward && row.Outcome == patterns.SagaStepSkipped:
			counts["steps skipped"]++
		case row.Action == patterns.SagaPhaseCompensate && row.Outcome == patterns.SagaStepCompensated:
			counts["steps compensated"]++
		}
	}
	for _, row := range replay {
		if row.Outcome == string(patterns.SagaStepAlreadyCompensated) {
			counts["already-compensated no-ops on the replay"]++
		}
	}
	labels := make([]string, 0, len(counts))
	for label := range counts {
		labels = append(labels, label)
	}
	sort.Strings(labels)
	served := make([]ArchSagaCountRow, 0, len(labels))
	for _, label := range labels {
		served = append(served, ArchSagaCountRow{Label: label, Value: counts[label]})
	}
	return served
}

// archSagaSemantics serves the observed rules — each one grounded in a
// panel on this payload or in the machinery's own pinned tests.
func archSagaSemantics() []string {
	return []string{
		"forward runs in order until a step fails or the context dies; every step after the " +
			"stop is skipped and recorded as such (the failure run's ledger ends with step 3 skipped)",
		"compensation runs in REVERSE over the steps that EXECUTED — the failed step never " +
			"completed, so it has nothing to undo (the refund lands before the release, in both " +
			"undo panels)",
		"cancellation stops the forward work, never the cleanup it triggered: the undo pass " +
			"runs on context.WithoutCancel, so the cancellation run's refund and release " +
			"succeeded on a dead context",
		"re-compensation is a recorded no-op: the runner calls a compensating action at most " +
			"once, and the failure run's replay shows both steps already-compensated",
		"a compensation failure never strands the undos above it — recorded as " +
			"compensation-failed, the walk continues (pinned in the machinery's own test suite)",
		"steps that cannot be undone go last: send-confirmation declares no compensating " +
			"action, so nothing can fail after an externalized fact",
	}
}

// archSagaSurfaceRows serves the machinery's exported surface as data —
// every id below compiles in internal/domain/patterns/saga.go.
func archSagaSurfaceRows() []ArchSagaSurfaceRow {
	return []ArchSagaSurfaceRow{
		{ID: "SagaStep", Kind: "type",
			Role: "one ordered unit of work: Name, Forward action, Compensate action"},
		{ID: "SagaAction", Kind: "type",
			Role: "func(ctx context.Context) error — the one signature both halves share"},
		{ID: "NewSagaRunner", Kind: "func",
			Role: "NewSagaRunner(steps ...SagaStep) *SagaRunner"},
		{ID: "(*SagaRunner).Run", Kind: "method",
			Role: "Run(ctx) SagaOutcome — forward-until-failure, then reverse compensation; one-shot"},
		{ID: "(*SagaRunner).Compensate", Kind: "method",
			Role: "Compensate(ctx) []SagaStepResult — the idempotent recovery pass"},
		{ID: "SagaOutcome", Kind: "type",
			Role: "the outcomes ledger: Status, per-step rows, the typed trigger error, counted totals"},
		{ID: "SagaStepResult", Kind: "type",
			Role: "one ledger row: Index, Name, Action (forward|compensate), Outcome, error text"},
		{ID: "SagaStepError", Kind: "type",
			Role: "the typed trigger — the failing step's identity wrapping its error (errors.Is/As-able)"},
		{ID: "SagaStatus", Kind: "vocabulary",
			Role: "completed | compensated | cancelled"},
		{ID: "SagaStepOutcome", Kind: "vocabulary",
			Role: "executed | failed | skipped | compensated | nothing-to-undo | compensation-failed | already-compensated"},
	}
}
