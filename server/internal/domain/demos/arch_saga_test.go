package demos

import (
	"encoding/json"
	"strings"
	"testing"

	"guide/internal/domain/patterns"
)

// The saga demo's builder, tested directly: the three drives observe
// what they claim to observe (full commit, reverse compensation after
// an injected failure, compensation that survives a cancelled context),
// the replay pass carries the machinery's own idempotency record, the
// served vocabularies match the real constants in internal/domain/
// patterns, and the whole payload is a deterministic function of the
// fixed script — no wall clock, no randomness, no invented numbers.

// archSagaRunPanel finds one observed run by id.
func archSagaRunPanel(t *testing.T, payload ArchSagaPayload, id string) ArchSagaRunPanel {
	t.Helper()
	for _, run := range payload.Runs {
		if run.ID == id {
			return run
		}
	}
	t.Fatalf("run %q missing from payload; runs = %+v", id, payload.Runs)
	return ArchSagaRunPanel{}
}

// archSagaOutcomeSeq renders a ledger's step/phase/outcome sequence for
// exact-order pins.
func archSagaOutcomeSeq(rows []ArchSagaLedgerRow) []string {
	out := make([]string, 0, len(rows))
	for _, row := range rows {
		out = append(out, row.Step+" "+row.Phase+"/"+row.Outcome)
	}
	return out
}

// archSagaCountValue finds one counted observation by exact label.
func archSagaCountValue(t *testing.T, run ArchSagaRunPanel, label string) int {
	t.Helper()
	for _, row := range run.Counts {
		if row.Label == label {
			return row.Value
		}
	}
	t.Fatalf("count %q missing from run %q; counts = %+v", label, run.ID, run.Counts)
	return 0
}

func TestArchSagaPayloadIsDeterministic(t *testing.T) {
	first, err := json.Marshal(BuildArchSagaPayload())
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	second, err := json.Marshal(BuildArchSagaPayload())
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if string(first) != string(second) {
		t.Fatal("the fixed script must serve byte-identical payloads (no clock, no randomness)")
	}
}

func TestArchSagaSuccessRunObservesTheFullCommit(t *testing.T) {
	payload := BuildArchSagaPayload()
	run := archSagaRunPanel(t, payload, "success")

	if run.Status != string(patterns.SagaStatusCompleted) {
		t.Fatalf("status = %q, want completed", run.Status)
	}
	if run.Trigger != "" {
		t.Errorf("a completed run carries no trigger, got %q", run.Trigger)
	}
	want := []string{
		"0 · reserve-inventory forward/executed",
		"1 · charge-card forward/executed",
		"2 · book-shipment forward/executed",
		"3 · send-confirmation forward/executed",
	}
	if got := archSagaOutcomeSeq(run.Ledger); !equalStrings(got, want) {
		t.Fatalf("ledger = %v, want %v", got, want)
	}
	// Every executed row's detail is the world line the action appended.
	if run.Ledger[0].Detail != "inventory: reserved 3× gopher-plush for order o-1001" {
		t.Errorf("row 0 detail = %q, want the observed world line", run.Ledger[0].Detail)
	}
	if len(run.WorldLog) != 4 {
		t.Errorf("world log = %v, want exactly the four forward effects", run.WorldLog)
	}
	if got := archSagaCountValue(t, run, "steps executed"); got != 4 {
		t.Errorf("steps executed = %d, want 4", got)
	}
	if _, has := countLabel(run, "steps compensated"); has {
		t.Error("a completed run must not carry a compensated count")
	}
}

func TestArchSagaFailureRunCompensatesInReverse(t *testing.T) {
	payload := BuildArchSagaPayload()
	run := archSagaRunPanel(t, payload, "failure")

	if run.Status != string(patterns.SagaStatusCompensated) {
		t.Fatalf("status = %q, want compensated", run.Status)
	}
	want := []string{
		"0 · reserve-inventory forward/executed",
		"1 · charge-card forward/executed",
		"2 · book-shipment forward/failed",
		"3 · send-confirmation forward/skipped",
		"1 · charge-card compensate/compensated",
		"0 · reserve-inventory compensate/compensated",
	}
	if got := archSagaOutcomeSeq(run.Ledger); !equalStrings(got, want) {
		t.Fatalf("ledger = %v, want %v", got, want)
	}
	// The failed row carries the action's own refusal; the undo rows
	// carry the world's refund and release lines, reverse order.
	if !strings.Contains(run.Ledger[2].Detail, "carrier: no ground pickup window") {
		t.Errorf("failed row detail = %q, want the carrier refusal", run.Ledger[2].Detail)
	}
	if run.Ledger[4].Detail != "payments: refunded 4200 cents to tok_visa_4242 for order o-1001" {
		t.Errorf("first undo detail = %q, want the observed refund line", run.Ledger[4].Detail)
	}
	if run.Ledger[5].Detail != "inventory: released 3× gopher-plush for order o-1001" {
		t.Errorf("second undo detail = %q, want the observed release line", run.Ledger[5].Detail)
	}
	// The trigger names the failing step and wraps the carrier error.
	if !strings.Contains(run.Trigger, "book-shipment") || !strings.Contains(run.Trigger, "carrier") {
		t.Errorf("trigger = %q, want the failing step's identity and error", run.Trigger)
	}
	for label, want := range map[string]int{
		"steps executed": 2, "steps failed": 1, "steps skipped": 1, "steps compensated": 2,
	} {
		if got := archSagaCountValue(t, run, label); got != want {
			t.Errorf("count %q = %d, want %d", label, got, want)
		}
	}
}

func TestArchSagaFailureRunReplayRecordsIdempotentNoOps(t *testing.T) {
	payload := BuildArchSagaPayload()
	run := archSagaRunPanel(t, payload, "failure")

	want := []string{
		"1 · charge-card compensate/already-compensated",
		"0 · reserve-inventory compensate/already-compensated",
	}
	if got := archSagaOutcomeSeq(run.Replay); !equalStrings(got, want) {
		t.Fatalf("replay = %v, want %v", got, want)
	}
	if run.Replay[0].Seq != len(run.Ledger)+1 {
		t.Errorf("replay seq starts at %d, want %d (continues the run's ledger)",
			run.Replay[0].Seq, len(run.Ledger)+1)
	}
	// The world never saw a second refund: exactly one refund line
	// exists across the whole run — the replay called nothing.
	refunds := 0
	for _, line := range run.WorldLog {
		if strings.Contains(line, "refunded") {
			refunds++
		}
	}
	if refunds != 1 {
		t.Errorf("world log holds %d refund lines, want exactly 1: %v", refunds, run.WorldLog)
	}
	if got := archSagaCountValue(t, run, "already-compensated no-ops on the replay"); got != 2 {
		t.Errorf("replay no-ops = %d, want 2", got)
	}
}

func TestArchSagaCancellationRunUndoesOnADeadContext(t *testing.T) {
	payload := BuildArchSagaPayload()
	run := archSagaRunPanel(t, payload, "cancellation")

	if run.Status != string(patterns.SagaStatusCancelled) {
		t.Fatalf("status = %q, want cancelled", run.Status)
	}
	if !strings.Contains(run.Trigger, "context canceled") {
		t.Errorf("trigger = %q, want the context's own error", run.Trigger)
	}
	want := []string{
		"0 · reserve-inventory forward/executed",
		"1 · charge-card forward/executed",
		"2 · book-shipment forward/skipped",
		"3 · send-confirmation forward/skipped",
		"1 · charge-card compensate/compensated",
		"0 · reserve-inventory compensate/compensated",
	}
	if got := archSagaOutcomeSeq(run.Ledger); !equalStrings(got, want) {
		t.Fatalf("ledger = %v, want %v", got, want)
	}
	// The compensating actions check their ctx (they fail with "undo saw
	// a dead context" on a cancelled one) — both undos compensated, so
	// the machinery really handed them a cancellation-stripped context.
	if got := archSagaCountValue(t, run, "steps compensated"); got != 2 {
		t.Errorf("steps compensated = %d, want 2 on the dead context (WithoutCancel)", got)
	}
	if _, has := countLabel(run, "already-compensated no-ops on the replay"); has {
		t.Error("the cancellation run carries no replay — only the failure run replays")
	}
}

func TestArchSagaStepsPanelMatchesTheFixedScript(t *testing.T) {
	payload := BuildArchSagaPayload()

	if len(payload.Steps) != 4 {
		t.Fatalf("steps panel = %+v, want the four scripted steps", payload.Steps)
	}
	for index, step := range payload.Steps {
		if step.Order != index {
			t.Errorf("step %d panel order = %d, want the machinery's 0-based index", index, step.Order)
		}
	}
	if payload.Steps[3].Name != "send-confirmation" || payload.Steps[3].Undo != "" {
		t.Errorf("the last step must be the non-compensatable confirmation, got %+v", payload.Steps[3])
	}
	if payload.Steps[0].Undo == "" {
		t.Error("reserve-inventory must declare its release as the compensating action")
	}
}

// TestArchSagaSurfaceMatchesTheRealMachinery pins the served surface
// against the actual constants: the two vocabulary rows must carry
// exactly the machinery's statuses and outcomes, and the payload must
// name the real source file.
func TestArchSagaSurfaceMatchesTheRealMachinery(t *testing.T) {
	payload := BuildArchSagaPayload()

	if payload.Source != "server/internal/domain/patterns/saga.go" {
		t.Errorf("source = %q, want the real machinery's path", payload.Source)
	}
	ids := map[string]ArchSagaSurfaceRow{}
	for _, row := range payload.Surface {
		ids[row.ID] = row
	}
	for _, want := range []string{
		"SagaStep", "SagaAction", "NewSagaRunner", "(*SagaRunner).Run", "(*SagaRunner).Compensate",
		"SagaOutcome", "SagaStepResult", "SagaStepError", "SagaStatus", "SagaStepOutcome",
	} {
		if _, ok := ids[want]; !ok {
			t.Errorf("surface row %q missing; surface = %+v", want, payload.Surface)
		}
	}
	statusRow := ids["SagaStatus"]
	for _, status := range []string{
		string(patterns.SagaStatusCompleted), string(patterns.SagaStatusCompensated),
		string(patterns.SagaStatusCancelled),
	} {
		if !strings.Contains(statusRow.Role, status) {
			t.Errorf("SagaStatus role %q must carry the real status %q", statusRow.Role, status)
		}
	}
	outcomeRow := ids["SagaStepOutcome"]
	for _, outcome := range []string{
		string(patterns.SagaStepExecuted), string(patterns.SagaStepFailed),
		string(patterns.SagaStepSkipped), string(patterns.SagaStepCompensated),
		string(patterns.SagaStepNothingToUndo), string(patterns.SagaStepCompensationFailed),
		string(patterns.SagaStepAlreadyCompensated),
	} {
		if !strings.Contains(outcomeRow.Role, outcome) {
			t.Errorf("SagaStepOutcome role %q must carry the real outcome %q", outcomeRow.Role, outcome)
		}
	}
}

// TestArchSagaInventsNoNumbers pins the honesty claim: every count on
// every run equals a recount from that run's own ledger and replay, and
// no timing field appears anywhere on the wire.
func TestArchSagaInventsNoNumbers(t *testing.T) {
	payload := BuildArchSagaPayload()

	for _, run := range payload.Runs {
		recount := map[string]int{}
		for _, row := range run.Ledger {
			switch row.Phase + "/" + row.Outcome {
			case "forward/executed":
				recount["steps executed"]++
			case "forward/failed":
				recount["steps failed"]++
			case "forward/skipped":
				recount["steps skipped"]++
			case "compensate/compensated":
				recount["steps compensated"]++
			case "compensate/already-compensated":
				recount["already-compensated no-ops on the replay"]++
			}
		}
		for _, replayRow := range run.Replay {
			if replayRow.Outcome == "already-compensated" {
				recount["already-compensated no-ops on the replay"]++
			}
		}
		for _, count := range run.Counts {
			if recount[count.Label] != count.Value {
				t.Errorf("run %q count %q = %d, recount from the ledger = %d",
					run.ID, count.Label, count.Value, recount[count.Label])
			}
		}
	}

	encoded, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	for _, timingField := range []string{"elapsedMs", "durationMs", "totalMs", "latencyMs", "\"at\":"} {
		if strings.Contains(string(encoded), timingField) {
			t.Errorf("the saga payload must not carry timing fields, found %q", timingField)
		}
	}
}

// equalStrings compares two string slices without reflect — the pinned
// sequences are short and exact.
func equalStrings(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

// countLabel says whether one counted observation exists, for the
// absence pins above.
func countLabel(run ArchSagaRunPanel, label string) (int, bool) {
	for _, row := range run.Counts {
		if row.Label == label {
			return row.Value, true
		}
	}
	return 0, false
}
