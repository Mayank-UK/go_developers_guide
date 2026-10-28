package presentation

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"guide/internal/core"
	"guide/internal/domain/demos"
)

// The event-store endpoint through the production middleware chain,
// driven against the REAL builder. The exhibit is deterministic with no
// inputs, so the byte-stable claim is checked live, and the payload's
// structure (three panels, observed rows with versions, counted totals,
// the replay evidence block, the store surface) is asserted from the
// wire — the same facts the client slice decodes.

// newArchEventstoreServer builds the real handler behind the chain.
func newArchEventstoreServer(t *testing.T) http.Handler {
	t.Helper()
	logger := newTestLogger(&strings.Builder{})
	mux := http.NewServeMux()
	mux.Handle("GET "+demos.ArchEventstorePath, NewArchEventstoreHandler(logger))
	return Chain(
		RequestID,
		RequestLogger(logger),
		Recover(logger),
		EnvelopeMuxErrors(logger),
	)(mux)
}

// archEventstoreDecode fetches and decodes the exhibit payload.
func archEventstoreDecode(t *testing.T, server http.Handler) map[string]any {
	t.Helper()
	rec := doRequest(server, http.MethodGet, demos.ArchEventstorePath, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if rec.Header().Get(core.RequestIDHeader) == "" {
		t.Error("every response must carry x-request-id")
	}
	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("payload not JSON: %v\n%s", err, rec.Body.String())
	}
	return payload
}

func TestArchEventstoreDemoServesTheThreeActLifecycle(t *testing.T) {
	server := newArchEventstoreServer(t)
	payload := archEventstoreDecode(t, server)

	if payload["mode"] == "" || payload["honestyNote"] == "" {
		t.Error("the payload must carry its mode and honesty note")
	}
	if payload["streamId"] != "inventory-tee-medium" {
		t.Errorf("streamId = %v, want the exhibit's inventory stream", payload["streamId"])
	}

	surface, ok := payload["store"].([]any)
	if !ok || len(surface) < 5 {
		t.Fatalf("store surface = %v, want the real API's served lines", payload["store"])
	}

	panels, ok := payload["panels"].([]any)
	if !ok || len(panels) != 3 {
		t.Fatalf("panels = %v, want the lifecycle/safety/replay trio", payload["panels"])
	}
	wantIDs := []string{"lifecycle", "safety", "replay"}
	for i, want := range wantIDs {
		panel, ok := panels[i].(map[string]any)
		if !ok {
			t.Fatalf("panel %d not an object: %v", i, panels[i])
		}
		if panel["id"] != want {
			t.Errorf("panel %d id = %v, want %q", i, panel["id"], want)
		}
		rows, ok := panel["rows"].([]any)
		if !ok || len(rows) < 3 {
			t.Errorf("panel %v carries %v rows, want a full act (>= 3)", want, panel["rows"])
			continue
		}
		for _, row := range rows {
			runRow, ok := row.(map[string]any)
			if !ok {
				t.Fatalf("row not an object: %v", row)
			}
			if _, ok := runRow["version"].(float64); !ok {
				t.Errorf("panel %v row %v lacks its observed version", want, runRow["step"])
			}
		}
		counts, _ := panel["counts"].([]any)
		if len(counts) == 0 {
			t.Errorf("panel %v carries no counted observations", want)
		}
	}
}

func TestArchEventstoreDemoServesTheConflictAndReplayEvidence(t *testing.T) {
	server := newArchEventstoreServer(t)
	payload := archEventstoreDecode(t, server)

	var conflictRow map[string]any
	for _, panel := range payload["panels"].([]any) {
		for _, row := range panel.(map[string]any)["rows"].([]any) {
			runRow := row.(map[string]any)
			if runRow["outcome"] == "refused-conflict" {
				conflictRow = runRow
			}
		}
	}
	if conflictRow == nil {
		t.Fatal("the payload must carry the observed conflict row")
	}
	if conflictRow["expectedVersion"] != float64(3) || conflictRow["actualVersion"] != float64(4) {
		t.Errorf("conflict row = expected %v / actual %v, want 3 / 4",
			conflictRow["expectedVersion"], conflictRow["actualVersion"])
	}

	replay, ok := payload["replay"].(map[string]any)
	if !ok {
		t.Fatalf("replay evidence missing: %v", payload["replay"])
	}
	if replay["foldedEvents"] != float64(5) {
		t.Errorf("foldedEvents = %v, want 5", replay["foldedEvents"])
	}
	if replay["byteIdentical"] != true || replay["rebuildsRun"] != float64(2) {
		t.Errorf("rebuild evidence = %v runs / identical=%v, want 2 / true",
			replay["rebuildsRun"], replay["byteIdentical"])
	}
	if replay["absentVersion"] != float64(0) {
		t.Errorf("absentVersion = %v, want the honest 0", replay["absentVersion"])
	}
	applied, _ := replay["appliedInOrder"].([]any)
	if len(applied) != 5 {
		t.Errorf("appliedInOrder = %v entries, want the five folded events", len(applied))
	}
}

func TestArchEventstoreDemoServesIdenticalBodies(t *testing.T) {
	server := newArchEventstoreServer(t)
	first := doRequest(server, http.MethodGet, demos.ArchEventstorePath, "")
	second := doRequest(server, http.MethodGet, demos.ArchEventstorePath, "")
	if first.Body.String() != second.Body.String() {
		t.Error("the exhibit is pure over fixed inputs — two GETs must serve byte-identical bodies")
	}
}

func TestArchEventstoreDemoRejectsNonGet(t *testing.T) {
	server := newArchEventstoreServer(t)
	rec := doRequest(server, http.MethodPost, demos.ArchEventstorePath, "")
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST status = %d, want 405", rec.Code)
	}
	if rec.Header().Get("Allow") == "" {
		t.Error("405 must carry the Allow header")
	}
}
