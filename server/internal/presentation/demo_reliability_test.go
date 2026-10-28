package presentation

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"guide/internal/core"
	"guide/internal/domain/demos"
)

// The reliability breaker endpoint through the production middleware
// chain, driven against the REAL console machinery on a tiny config —
// the scenario store's modes trip the breaker, convert latency into
// deadline exceeded, and recover it across requests, exactly the arc
// the terminal tour walks with the default config.

// reliabilityDemoTestConfig keeps every effect real but fast: the
// cooldown outlasts a request but not a nap.
func reliabilityDemoTestConfig() demos.ReliabilityConfig {
	return demos.ReliabilityConfig{
		FailureThreshold: 2,
		Cooldown:         60 * time.Millisecond,
		CallTimeout:      8 * time.Millisecond,
		WalkAttempts:     4,
		RetryAttempts:    3,
		RetryBaseDelay:   2 * time.Millisecond,
		RetryJitter:      1 * time.Millisecond,
	}
}

func newReliabilityBreakerServer(t *testing.T) (http.Handler, *core.ScenarioStore) {
	t.Helper()
	store := core.NewScenarioStore()
	console, err := demos.NewReliabilityConsole(reliabilityDemoTestConfig(), store)
	if err != nil {
		t.Fatalf("console: %v", err)
	}
	logger := newTestLogger(&strings.Builder{})
	mux := http.NewServeMux()
	mux.Handle("GET "+demos.ReliabilityBreakerPath, NewReliabilityBreakerHandler(console, logger))
	return Chain(
		RequestID,
		RequestLogger(logger),
		Recover(logger),
		EnvelopeMuxErrors(logger),
	)(mux), store
}

// reliabilityDecode fetches and decodes the demo payload.
func reliabilityDecode(t *testing.T, server http.Handler) map[string]any {
	t.Helper()
	rec := doRequest(server, http.MethodGet, demos.ReliabilityBreakerPath, "")
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

// reliabilityRows digs the walk rows out of the decoded payload.
func reliabilityRows(t *testing.T, payload map[string]any) []map[string]any {
	t.Helper()
	walk, ok := payload["walk"].(map[string]any)
	if !ok {
		t.Fatalf("walk panel missing: %v", payload["walk"])
	}
	raw, ok := walk["rows"].([]any)
	if !ok {
		t.Fatalf("walk rows missing: %v", walk["rows"])
	}
	rows := make([]map[string]any, 0, len(raw))
	for _, item := range raw {
		row, ok := item.(map[string]any)
		if !ok {
			t.Fatalf("walk row not an object: %v", item)
		}
		rows = append(rows, row)
	}
	return rows
}

func armReliabilityDemo(t *testing.T, store *core.ScenarioStore, mode core.FailureInjection) {
	t.Helper()
	if _, err := store.Update(core.ScenarioPatch{InjectFailure: &mode}); err != nil {
		t.Fatalf("arm %s: %v", mode, err)
	}
}

func TestReliabilityBreakerDemoServesClosedStateUnderNone(t *testing.T) {
	server, _ := newReliabilityBreakerServer(t)
	payload := reliabilityDecode(t, server)

	breaker := payload["breaker"].(map[string]any)
	if breaker["state"] != demos.ReliabilityBreakerClosed {
		t.Errorf("state = %v, want closed under no injection", breaker["state"])
	}
	config := breaker["config"].(map[string]any)
	if config["failureThreshold"] != float64(2) || config["halfOpenProbes"] != float64(1) {
		t.Errorf("config panel = %v", config)
	}
	if age, ok := breaker["stateAgeMs"].(float64); !ok || age < 0 {
		t.Errorf("stateAgeMs = %v, want a measured non-negative age", breaker["stateAgeMs"])
	}

	rows := reliabilityRows(t, payload)
	if len(rows) != 4 {
		t.Fatalf("rows = %d, want the configured walk attempts", len(rows))
	}
	for _, row := range rows {
		if row["action"] != "call" || row["outcome"] != "success" {
			t.Errorf("row = %v, want a called-and-succeeded attempt", row)
		}
	}
	walk := payload["walk"].(map[string]any)
	if !strings.Contains(walk["note"].(string), "stays closed") {
		t.Errorf("walk note = %v", walk["note"])
	}
	scenario := payload["scenario"].(map[string]any)
	if scenario["mode"] != "none" || scenario["patchEndpoint"] != "/api/demo/scenario" {
		t.Errorf("scenario panel = %v", scenario)
	}
}

func TestReliabilityBreakerDemoTripsAndRecoversAcrossRequests(t *testing.T) {
	server, store := newReliabilityBreakerServer(t)
	armReliabilityDemo(t, store, core.FailureError)

	// The trip: threshold failures open the breaker mid-walk and the
	// remaining attempts are refused outright.
	trip := reliabilityDecode(t, server)
	tripRows := reliabilityRows(t, trip)
	if got := tripRows[0]["outcome"]; got != "injected error" {
		t.Fatalf("first row outcome = %v, want the injected error", got)
	}
	breaker := trip["breaker"].(map[string]any)
	if breaker["state"] != demos.ReliabilityBreakerOpen {
		t.Fatalf("state after the trip = %v, want open", breaker["state"])
	}
	transitions := breaker["transitions"].([]any)
	last := transitions[len(transitions)-1].(map[string]any)
	if last["to"] != demos.ReliabilityBreakerOpen || !strings.Contains(last["cause"].(string), "threshold") {
		t.Errorf("trip transition = %v", last)
	}

	// Inside the cooldown: every attempt fast-fails without a call.
	refused := reliabilityDecode(t, server)
	for _, row := range reliabilityRows(t, refused) {
		if row["action"] != "fast-fail" {
			t.Errorf("row inside cooldown = %v, want a fast-fail", row)
		}
	}

	// Recovery: disarm, outlast the cooldown, and the half-open probe
	// closes the circuit — the arc persists across requests.
	armReliabilityDemo(t, store, core.FailureNone)
	time.Sleep(reliabilityDemoTestConfig().Cooldown + 20*time.Millisecond)
	recovered := reliabilityDecode(t, server)
	recoveredRows := reliabilityRows(t, recovered)
	if recoveredRows[0]["stateBefore"] != demos.ReliabilityBreakerHalfOpen ||
		recoveredRows[0]["outcome"] != "success" {
		t.Errorf("recovery probe row = %v", recoveredRows[0])
	}
	final := recovered["breaker"].(map[string]any)
	if final["state"] != demos.ReliabilityBreakerClosed {
		t.Errorf("state after recovery = %v, want closed", final["state"])
	}
	recoveredTransitions := final["transitions"].([]any)
	closing := recoveredTransitions[len(recoveredTransitions)-1].(map[string]any)
	if closing["from"] != demos.ReliabilityBreakerHalfOpen || closing["to"] != demos.ReliabilityBreakerClosed {
		t.Errorf("recovery transition = %v", closing)
	}
}

func TestReliabilityBreakerDemoConvertsLatencyIntoDeadlineExceeded(t *testing.T) {
	server, store := newReliabilityBreakerServer(t)
	armReliabilityDemo(t, store, core.FailureLatency)

	payload := reliabilityDecode(t, server)
	rows := reliabilityRows(t, payload)
	for i := 0; i < 2; i++ { // the threshold rung: called attempts.
		if rows[i]["outcome"] != "deadline exceeded" {
			t.Errorf("row %d outcome = %v, want deadline exceeded", i+1, rows[i]["outcome"])
		}
		if elapsed, ok := rows[i]["elapsedMs"].(float64); !ok || elapsed < 8 {
			t.Errorf("row %d elapsedMs = %v, want at least the per-call deadline", i+1, rows[i]["elapsedMs"])
		}
	}

	// The retry series ran the same stalled dependency: its rows carry
	// the deadline outcome and measured waits too.
	retry := payload["retry"].(map[string]any)
	retryRows := retry["rows"].([]any)
	if len(retryRows) != 3 {
		t.Fatalf("retry rows = %d, want all three attempts", len(retryRows))
	}
	for _, item := range retryRows {
		row := item.(map[string]any)
		if row["outcome"] != "deadline exceeded" {
			t.Errorf("retry row = %v, want deadline exceeded", row)
		}
	}
	if total, ok := retry["totalElapsedMs"].(float64); !ok || total <= 0 {
		t.Errorf("totalElapsedMs = %v, want a measured positive total", retry["totalElapsedMs"])
	}
}

func TestReliabilityBreakerDemoRejectsNonGet(t *testing.T) {
	server, _ := newReliabilityBreakerServer(t)
	rec := doRequest(server, http.MethodPost, demos.ReliabilityBreakerPath, "")
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST status = %d, want 405", rec.Code)
	}
}
