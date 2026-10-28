package presentation

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"guide/internal/core"
	"guide/internal/domain/demos"
)

// The saga endpoint through the production middleware chain, driven
// against the REAL builder. The payload is a pure function of the fixed
// script, so the byte-stable claim is checked live, every run's shape
// is pinned through the wire, and the method discipline (GET only)
// keeps the endpoint honest.

// newArchSagaServer builds the real handler over the production chain.
func newArchSagaServer(t *testing.T) http.Handler {
	t.Helper()
	logger := newTestLogger(&strings.Builder{})
	mux := http.NewServeMux()
	mux.Handle("GET "+demos.ArchSagaPath, NewArchSagaHandler(logger))
	return Chain(
		RequestID,
		RequestLogger(logger),
		Recover(logger),
		EnvelopeMuxErrors(logger),
	)(mux)
}

func TestArchSagaDemoServesTheThreeObservedRuns(t *testing.T) {
	server := newArchSagaServer(t)
	rec := doRequest(server, http.MethodGet, demos.ArchSagaPath, "")
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

	if payload["mode"] == "" || payload["honestyNote"] == "" {
		t.Error("the payload must carry its mode and honesty note")
	}
	if payload["source"] != "server/internal/domain/patterns/saga.go" {
		t.Errorf("source = %v, want the real machinery's path", payload["source"])
	}

	runs, ok := payload["runs"].([]any)
	if !ok || len(runs) != 3 {
		t.Fatalf("runs = %v, want the success/failure/cancellation trio", payload["runs"])
	}
	wantStatus := map[string]string{"success": "completed", "failure": "compensated", "cancellation": "cancelled"}
	for _, run := range runs {
		panel, ok := run.(map[string]any)
		if !ok {
			t.Fatalf("run panel not an object: %v", run)
		}
		id, _ := panel["id"].(string)
		if panel["status"] != wantStatus[id] {
			t.Errorf("run %v status = %v, want %v", id, panel["status"], wantStatus[id])
		}
		ledger, _ := panel["ledger"].([]any)
		if len(ledger) < 4 {
			t.Errorf("run %v ledger too thin to teach: %v rows", id, len(ledger))
		}
		worldLog, _ := panel["worldLog"].([]any)
		if len(worldLog) == 0 {
			t.Errorf("run %v carries no observed world lines", id)
		}
		counts, _ := panel["counts"].([]any)
		if len(counts) == 0 {
			t.Errorf("run %v carries no counted observations", id)
		}
	}

	// The failure run alone carries the idempotent replay.
	failure := runs[1].(map[string]any)
	if replay, _ := failure["replay"].([]any); len(replay) != 2 {
		t.Errorf("failure run replay = %v, want the two already-compensated no-ops", failure["replay"])
	}
	for _, other := range []int{0, 2} {
		if panel := runs[other].(map[string]any); panel["replay"] != nil {
			t.Errorf("run %v must not carry a replay", panel["id"])
		}
	}

	steps, ok := payload["steps"].([]any)
	if !ok || len(steps) != 4 {
		t.Fatalf("steps = %v, want the four scripted steps", payload["steps"])
	}
	surface, ok := payload["surface"].([]any)
	if !ok || len(surface) < 8 {
		t.Fatalf("surface = %v, want the machinery's exported ids", payload["surface"])
	}
	semantics, ok := payload["semantics"].([]any)
	if !ok || len(semantics) < 5 {
		t.Fatalf("semantics = %v, want the observed rules", payload["semantics"])
	}
}

func TestArchSagaDemoServesIdenticalBodies(t *testing.T) {
	server := newArchSagaServer(t)
	first := doRequest(server, http.MethodGet, demos.ArchSagaPath, "")
	second := doRequest(server, http.MethodGet, demos.ArchSagaPath, "")
	if first.Body.String() != second.Body.String() {
		t.Error("two GETs must serve byte-identical bodies (fixed script, no clock)")
	}
}

func TestArchSagaDemoRejectsNonGet(t *testing.T) {
	server := newArchSagaServer(t)
	rec := doRequest(server, http.MethodPost, demos.ArchSagaPath, "")
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST status = %d, want 405", rec.Code)
	}
	if rec.Header().Get("Allow") == "" {
		t.Error("405 must carry the Allow header")
	}
}
