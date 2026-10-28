package demos

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"guide/internal/domain/patterns"
)

// The event-store demo's machinery, tested directly: the fixed script
// observes what it claims to observe (every count, version, conflict
// field, and state comparison comes from the real run), the conflict
// the safety act narrates is the real typed error, the whole payload
// is a deterministic function of nothing at all (fixed inputs, fixed
// exhibit clock), and no invented number or timing measurement reaches
// the wire.

// archEventstoreCountValue finds one counted observation by exact label.
func archEventstoreCountValue(t *testing.T, panel ArchEventstorePanel, label string) int {
	t.Helper()
	for _, count := range panel.Counts {
		if count.Label == label {
			return count.Value
		}
	}
	t.Fatalf("count %q missing from panel %q; counts = %+v", label, panel.ID, panel.Counts)
	return 0
}

// archEventstorePanel finds one panel by id.
func archEventstorePanel(t *testing.T, payload ArchEventstorePayload, id string) ArchEventstorePanel {
	t.Helper()
	for _, panel := range payload.Panels {
		if panel.ID == id {
			return panel
		}
	}
	t.Fatalf("panel %q missing; panels = %+v", id, payload.Panels)
	return ArchEventstorePanel{}
}

func TestArchEventstorePayloadIsDeterministic(t *testing.T) {
	first, err := json.Marshal(BuildArchEventstorePayload())
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	second, err := json.Marshal(BuildArchEventstorePayload())
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if string(first) != string(second) {
		t.Fatal("the exhibit is pure over fixed inputs — two builds must serve byte-identical payloads")
	}
}

func TestArchEventstoreLifecycleActObservesThePinnedChoreography(t *testing.T) {
	panel := archEventstorePanel(t, BuildArchEventstorePayload(), "lifecycle")
	for label, want := range map[string]int{
		"appends accepted": 3,
		"events stored":    3,
	} {
		if got := archEventstoreCountValue(t, panel, label); got != want {
			t.Errorf("lifecycle count %q = %d, want %d", label, got, want)
		}
	}
	for i, row := range panel.Rows {
		if row.Outcome != "appended" {
			t.Errorf("lifecycle row %d outcome = %q, want appended", i, row.Outcome)
		}
		if want := int64(i + 1); row.Version != want {
			t.Errorf("lifecycle row %d version = %d, want the dense %d", i, row.Version, want)
		}
	}
}

func TestArchEventstoreSafetyActObservesThePinnedChoreography(t *testing.T) {
	panel := archEventstorePanel(t, BuildArchEventstorePayload(), "safety")
	for label, want := range map[string]int{
		"appends accepted":                                     2,
		"events stored":                                        2,
		"streams loaded by the second writer":                  1,
		"commands refused by the aggregate (no event written)": 2,
		"optimistic conflicts (typed) observed":                1,
		"events landed by stale appends":                       0,
		"recovery appends accepted after reload":               1,
	} {
		if got := archEventstoreCountValue(t, panel, label); got != want {
			t.Errorf("safety count %q = %d, want %d", label, got, want)
		}
	}

	var conflictRow *ArchEventstoreRow
	for i := range panel.Rows {
		if panel.Rows[i].Outcome == "refused-conflict" {
			conflictRow = &panel.Rows[i]
		}
	}
	if conflictRow == nil {
		t.Fatal("the safety act must carry the observed conflict row")
	}
	if conflictRow.ExpectedVersion != 3 || conflictRow.ActualVersion != 4 || conflictRow.Version != 4 {
		t.Errorf("conflict row = expected %d / actual %d / after %d, want 3 / 4 / 4",
			conflictRow.ExpectedVersion, conflictRow.ActualVersion, conflictRow.Version)
	}
	// The stale append landed nothing: the act's last row (the recovery)
	// sits at version 5 — 3 lifecycle events + the pick + the recovery.
	last := panel.Rows[len(panel.Rows)-1]
	if last.Outcome != "appended-after-reload" || last.Version != 5 {
		t.Errorf("safety act ends with %+v, want the recovery append at version 5", last)
	}
	// The over-reserve refusal never moved the stream.
	for _, row := range panel.Rows {
		if row.Outcome == "refused-by-aggregate" && row.Call == "reserve (decide only)" && row.Version != 4 {
			t.Errorf("the refused reserve left version %d, want the untouched 4", row.Version)
		}
	}
}

func TestArchEventstoreReplayEvidenceProvesTheRebuild(t *testing.T) {
	payload := BuildArchEventstorePayload()
	evidence := payload.Replay
	if evidence.FoldedEvents != 5 {
		t.Errorf("foldedEvents = %d, want the 5 stored events", evidence.FoldedEvents)
	}
	if len(evidence.AppliedInOrder) != 5 || !strings.HasPrefix(evidence.AppliedInOrder[0], "v1 inventory.stocked") {
		t.Errorf("appliedInOrder = %v, want five entries folding from v1 stocked", evidence.AppliedInOrder)
	}
	state, naive := evidence.State, evidence.NaiveExpected
	if state.OnHand != 30 || state.Reserved != 0 || state.Available != 30 || len(state.Reservations) != 0 {
		t.Errorf("rebuilt state = %+v, want onHand 30 / reserved 0 / available 30 / none open", state)
	}
	// The state carries a map, so it is compared part by part.
	if state.OnHand != naive.OnHand || state.Reserved != naive.Reserved ||
		state.Available != naive.Available || len(state.Reservations) != len(naive.Reservations) {
		t.Errorf("rebuilt state %+v disagrees with the naive ledger %+v", state, naive)
	}
	if evidence.RebuildsRun != 2 || !evidence.ByteIdentical {
		t.Errorf("rebuild evidence = %d runs / byteIdentical=%t, want 2 / true", evidence.RebuildsRun, evidence.ByteIdentical)
	}
	if evidence.AbsentStreamID != archEventstoreAbsentStream || evidence.AbsentVersion != 0 {
		t.Errorf("absent evidence = %q at v%d, want %q at v0", evidence.AbsentStreamID, evidence.AbsentVersion, archEventstoreAbsentStream)
	}
	if evidence.AbsentState.OnHand != 0 || evidence.AbsentState.Reserved != 0 || len(evidence.AbsentState.Reservations) != 0 {
		t.Errorf("absent state = %+v, want the untouched zero state", evidence.AbsentState)
	}
}

// TestArchEventstoreConflictIsTypedAndLeavesTheStreamUnchanged drives
// the machinery directly beside the script: the typed conflict the
// safety act narrates is the real error the store answers with, and a
// refused append leaves the stream exactly where it was.
func TestArchEventstoreConflictIsTypedAndLeavesTheStreamUnchanged(t *testing.T) {
	store := patterns.NewEventStore()
	ctx := context.Background()
	if _, err := store.Append(ctx, "solo", patterns.EventStreamVersionEmpty,
		patterns.EventAppend{Type: "inventory.stocked", Payload: []byte(`{"qty":5}`), RecordedAt: archEventstoreEventClock(1)}); err != nil {
		t.Fatalf("create: %v", err)
	}
	_, err := store.Append(ctx, "solo", patterns.EventStreamVersionEmpty,
		patterns.EventAppend{Type: "inventory.reserved", Payload: []byte(`{"qty":1}`), RecordedAt: archEventstoreEventClock(2)})
	var conflict *patterns.EventStoreConflict
	if !errors.As(err, &conflict) {
		t.Fatalf("stale create: %v, want the typed *patterns.EventStoreConflict", err)
	}
	if conflict.ExpectedVersion != 0 || conflict.ActualVersion != 1 {
		t.Errorf("conflict = expected %d / actual %d, want 0 / 1", conflict.ExpectedVersion, conflict.ActualVersion)
	}
	records, err := store.Stream(ctx, "solo")
	if err != nil {
		t.Fatalf("stream: %v", err)
	}
	if len(records) != 1 || records[0].Version != 1 {
		t.Errorf("after the refused append the stream = %d records, want exactly the original one", len(records))
	}
}

// TestArchEventstoreInventsNoNumbers pins the honesty claim: two runs
// serve identical counts and evidence, and no timing measurement field
// exists anywhere on the wire (occurrence times are fixed data inside
// strings, never measured durations).
func TestArchEventstoreInventsNoNumbers(t *testing.T) {
	first := BuildArchEventstorePayload()
	second := BuildArchEventstorePayload()
	for i, panel := range first.Panels {
		for j, got := range panel.Counts {
			want := second.Panels[i].Counts[j]
			if got != want {
				t.Errorf("count drifted between runs: %+v vs %+v", got, want)
			}
		}
	}
	if first.Replay.FoldedEvents != second.Replay.FoldedEvents ||
		!reflect.DeepEqual(first.Replay.State, second.Replay.State) {
		t.Error("the replay evidence drifted between two runs of the same fixed exhibit")
	}
	encoded, err := json.Marshal(first)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	for _, timingField := range []string{"elapsedMs", "durationMs", "totalMs", "latencyMs", "\"at\":", "tookMs"} {
		if strings.Contains(string(encoded), timingField) {
			t.Errorf("the payload must not carry timing fields, found %q", timingField)
		}
	}
}
