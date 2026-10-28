package presentation

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"guide/internal/core"
	"guide/internal/domain/demos"
)

// The scaling statelessness endpoint through the production middleware
// chain: the payload must carry the measured distribution, the
// thirteen-step session round, the counter beat, and the verdicts —
// and two GETs must serve byte-identical bodies, because the round
// reads no clock and no random source.

// newScalingStatelessServer builds the demo route under the standard
// test chain.
func newScalingStatelessServer(t *testing.T) http.Handler {
	t.Helper()
	logger := newTestLogger(&strings.Builder{})
	mux := http.NewServeMux()
	mux.Handle("GET "+demos.ScalingStatelessPath, NewScalingStatelessHandler(logger))
	return Chain(
		RequestID,
		RequestLogger(logger),
		Recover(logger),
		EnvelopeMuxErrors(logger),
	)(mux)
}

// scalingStatelessDecode fetches and decodes the demo payload.
func scalingStatelessDecode(t *testing.T, server http.Handler) map[string]any {
	t.Helper()
	rec := doRequest(server, http.MethodGet, demos.ScalingStatelessPath, "")
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

func TestScalingStatelessDemoServesTheFullRound(t *testing.T) {
	payload := scalingStatelessDecode(t, newScalingStatelessServer(t))

	config := payload["config"].(map[string]any)
	if config["replicas"] != float64(3) || config["distributionRequests"] != float64(12) {
		t.Errorf("config panel = %v", config)
	}
	if config["localToken"] != "s-local-ada" || config["sharedToken"] != "s-shared-ada" {
		t.Errorf("session tokens = %v / %v", config["localToken"], config["sharedToken"])
	}

	distribution := payload["distribution"].(map[string]any)
	if distribution["totalServed"] != float64(12) {
		t.Errorf("totalServed = %v, want 12", distribution["totalServed"])
	}
	rows := distribution["rows"].([]any)
	if len(rows) != 3 {
		t.Fatalf("distribution rows = %d, want 3", len(rows))
	}
	for _, item := range rows {
		row := item.(map[string]any)
		if row["served"] != float64(4) {
			t.Errorf("row %v served %v, want the measured 4", row["replicaId"], row["served"])
		}
	}

	steps := payload["steps"].([]any)
	if len(steps) != 13 {
		t.Fatalf("steps = %d, want 13", len(steps))
	}
	first := steps[0].(map[string]any)
	if first["phase"] != "local-memory" || first["outcome"] != "stored" {
		t.Errorf("first step = %v/%v", first["phase"], first["outcome"])
	}
	postSwap := steps[5].(map[string]any)
	if postSwap["outcome"] != "miss" || postSwap["route"] != "sticky" {
		t.Errorf("post-swap sticky step = %+v", postSwap)
	}
	finalStep := steps[12].(map[string]any)
	if finalStep["phase"] != "counter" {
		t.Errorf("final step phase = %v, want counter", finalStep["phase"])
	}

	counter := payload["counter"].(map[string]any)
	if counter["total"] != float64(6) {
		t.Errorf("counter total = %v, want the measured 6", counter["total"])
	}

	verdicts := payload["verdicts"].([]any)
	if len(verdicts) != 5 {
		t.Errorf("verdicts = %d, want 5", len(verdicts))
	}
	honesty, _ := payload["honesty"].(string)
	if !strings.Contains(honesty, "deterministic") {
		t.Errorf("honesty note must state the determinism: %.80s…", honesty)
	}
}

func TestScalingStatelessDemoIsDeterministicAcrossRequests(t *testing.T) {
	server := newScalingStatelessServer(t)
	first := doRequest(server, http.MethodGet, demos.ScalingStatelessPath, "")
	second := doRequest(server, http.MethodGet, demos.ScalingStatelessPath, "")
	if first.Body.String() != second.Body.String() {
		t.Error("two GETs must serve byte-identical payloads — the round is deterministic")
	}
}

func TestScalingStatelessDemoRejectsNonGet(t *testing.T) {
	server := newScalingStatelessServer(t)
	rec := doRequest(server, http.MethodPost, demos.ScalingStatelessPath, "")
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST status = %d, want 405", rec.Code)
	}
}
