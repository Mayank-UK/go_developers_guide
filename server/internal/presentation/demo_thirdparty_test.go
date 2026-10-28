package presentation

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"guide/internal/core"
	"guide/internal/domain/demos"
)

// The third-party hub endpoint through the production middleware
// chain, driven against the REAL builder and the REAL scenario store.
// The payload is deterministic per scenario state, so the byte-stable
// claim is checked live, and flipping the shared dataMode flips the
// wiring rows — the DI switch observed through the wire.

// newThirdPartyHubServer builds the real handler over a fresh store.
func newThirdPartyHubServer(t *testing.T) (http.Handler, *core.ScenarioStore) {
	t.Helper()
	store := core.NewScenarioStore()
	logger := newTestLogger(&strings.Builder{})
	mux := http.NewServeMux()
	mux.Handle("GET "+demos.ThirdPartyHubPath, NewThirdPartyHubHandler(store, logger))
	return Chain(
		RequestID,
		RequestLogger(logger),
		Recover(logger),
		EnvelopeMuxErrors(logger),
	)(mux), store
}

// thirdPartyHubDecode fetches and decodes the hub payload.
func thirdPartyHubDecode(t *testing.T, server http.Handler) map[string]any {
	t.Helper()
	rec := doRequest(server, http.MethodGet, demos.ThirdPartyHubPath, "")
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

func TestThirdPartyHubDemoServesTheAdapterSurface(t *testing.T) {
	server, _ := newThirdPartyHubServer(t)
	payload := thirdPartyHubDecode(t, server)

	if payload["mode"] == "" || payload["honestyNote"] == "" {
		t.Error("the payload must carry its mode and honesty note")
	}
	wiring, ok := payload["wiring"].(map[string]any)
	if !ok {
		t.Fatalf("wiring panel missing: %v", payload["wiring"])
	}
	if wiring["current"] != "fake" {
		t.Errorf("wiring.current = %v, want fake at the store's default", wiring["current"])
	}
	rows, ok := wiring["rows"].([]any)
	if !ok || len(rows) != 3 {
		t.Fatalf("wiring rows = %v, want the three vendor categories", wiring["rows"])
	}
	for _, row := range rows {
		wiringRow, ok := row.(map[string]any)
		if !ok {
			t.Fatalf("wiring row not an object: %v", row)
		}
		if wiringRow["active"] != "fakeAdapter" {
			t.Errorf("wiring row %v active = %v, want fakeAdapter", wiringRow["category"], wiringRow["active"])
		}
	}

	vendors, ok := payload["vendors"].([]any)
	if !ok || len(vendors) != 3 {
		t.Fatalf("vendors = %v, want the mailer/payments/storage trio", payload["vendors"])
	}
	for _, vendor := range vendors {
		panel, ok := vendor.(map[string]any)
		if !ok {
			t.Fatalf("vendor panel not an object: %v", vendor)
		}
		port := panel["port"].(map[string]any)
		if len(port["methods"].([]any)) < 1 {
			t.Errorf("vendor %v port carries no methods", panel["id"])
		}
		if len(port["implementations"].([]any)) != 2 {
			t.Errorf("vendor %v must list exactly two asserted implementations", panel["id"])
		}
		sandbox := panel["sandbox"].(map[string]any)
		if len(sandbox["rows"].([]any)) < 5 {
			t.Errorf("vendor %v sandbox run too thin to teach", panel["id"])
		}
		counts, _ := sandbox["counts"].([]any)
		if len(counts) == 0 {
			t.Errorf("vendor %v sandbox run carries no counted observations", panel["id"])
		}
		live := panel["live"].(map[string]any)
		outcome := live["outcome"].(map[string]any)
		if outcome["outcome"] != "refused-honestly" {
			t.Errorf("vendor %v live outcome = %v, want refused-honestly", panel["id"], outcome["outcome"])
		}
	}

	taxonomy, ok := payload["taxonomyMap"].([]any)
	if !ok || len(taxonomy) < 10 {
		t.Fatalf("taxonomy map = %v, want the full classification table", payload["taxonomyMap"])
	}
}

func TestThirdPartyHubDemoServesIdenticalBodiesPerScenarioState(t *testing.T) {
	server, _ := newThirdPartyHubServer(t)
	first := doRequest(server, http.MethodGet, demos.ThirdPartyHubPath, "")
	second := doRequest(server, http.MethodGet, demos.ThirdPartyHubPath, "")
	if first.Body.String() != second.Body.String() {
		t.Error("two GETs in the same scenario state must serve byte-identical bodies")
	}
}

func TestThirdPartyHubDemoWiringFlipsWithTheSharedScenario(t *testing.T) {
	server, store := newThirdPartyHubServer(t)
	if payload := thirdPartyHubDecode(t, server); payload["wiring"].(map[string]any)["current"] != "fake" {
		t.Fatalf("default state = %v, want fake", payload["wiring"])
	}

	live := core.DataModeLive
	if _, err := store.Update(core.ScenarioPatch{DataMode: &live}); err != nil {
		t.Fatalf("flip to live: %v", err)
	}
	payload := thirdPartyHubDecode(t, server)
	wiring := payload["wiring"].(map[string]any)
	if wiring["current"] != "live" {
		t.Fatalf("after PATCH, wiring.current = %v, want live", wiring["current"])
	}
	for _, row := range wiring["rows"].([]any) {
		wiringRow := row.(map[string]any)
		if wiringRow["active"] != "liveAdapter" {
			t.Errorf("wiring row %v active = %v, want liveAdapter after the flip",
				wiringRow["category"], wiringRow["active"])
		}
		if !strings.Contains(wiringRow["activeOutcome"].(string), "credentials_absent") {
			t.Errorf("wiring row %v should probe the live skeleton's honest refusal, got %v",
				wiringRow["category"], wiringRow["activeOutcome"])
		}
	}

	// The sandbox runs stay byte-stable across the flip — only the
	// wiring panel observes the scenario.
	fake := core.DataModeFake
	if _, err := store.Update(core.ScenarioPatch{DataMode: &fake}); err != nil {
		t.Fatalf("flip back: %v", err)
	}
}

func TestThirdPartyHubDemoRejectsNonGet(t *testing.T) {
	server, _ := newThirdPartyHubServer(t)
	rec := doRequest(server, http.MethodPost, demos.ThirdPartyHubPath, "")
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST status = %d, want 405", rec.Code)
	}
	if rec.Header().Get("Allow") == "" {
		t.Error("405 must carry the Allow header")
	}
}

// TestThirdPartyHubDemoNeverLeaksCredentialValues plants sentinel
// values in the resolver and proves they appear nowhere in the
// serialized payload — the hub carries env var NAMES, never values.
func TestThirdPartyHubDemoNeverLeaksCredentialValues(t *testing.T) {
	store := core.NewScenarioStore()
	logger := newTestLogger(&strings.Builder{})
	handler := NewThirdPartyHubHandler(store, logger)
	handler.credentials = func(name string) (string, bool) {
		return "sk_sentinel_DO_NOT_LEAK_" + name, true
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, demos.ThirdPartyHubPath, nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "sk_sentinel_DO_NOT_LEAK") {
		t.Fatal("a credential VALUE crossed the wire — the hub may only ever name the env var")
	}
}
