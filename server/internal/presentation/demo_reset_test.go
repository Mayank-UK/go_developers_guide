package presentation

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"guide/internal/core"
	"guide/internal/domain/demos"
)

// The shared demo reset endpoint through the production middleware
// chain, driven against the REAL stateful machinery it serves — the
// scenario store and the process-lifetime reliability console — with
// the same participant wiring the composition root registers. The
// stateful observables must actually move: a tripped breaker comes
// back closed with an empty history, and a mutated scenario comes
// back to fake/none.

// newDemoResetServer builds the reset endpoint beside the two demo
// surfaces it resets, so one test can trip real state through real
// routes and watch the reset return it.
func newDemoResetServer(t *testing.T) (http.Handler, *core.ScenarioStore) {
	t.Helper()
	store := core.NewScenarioStore()
	console, err := demos.NewReliabilityConsole(reliabilityDemoTestConfig(), store)
	if err != nil {
		t.Fatalf("console: %v", err)
	}
	logger := newTestLogger(&strings.Builder{})

	fakeMode, noFailure := core.DataModeFake, core.FailureNone
	resetter, err := demos.NewDemoResetter(
		demos.DemoResetParticipant{
			Area:  "scenario",
			Label: "Scenario store",
			Action: func() (string, error) {
				state, err := store.Update(core.ScenarioPatch{
					DataMode:      &fakeMode,
					InjectFailure: &noFailure,
				})
				if err != nil {
					return "", err
				}
				return "scenario returned to dataMode=" + string(state.DataMode) +
					", injectFailure=" + string(state.InjectFailure) + " — the constructed baseline", nil
			},
		},
		demos.DemoResetParticipant{
			Area:  "reliability",
			Label: "Process-lifetime breaker",
			Action: func() (string, error) {
				console.Reset()
				snapshot := console.BreakerObservation(time.Now())
				return "breaker back to " + snapshot.State +
					" — counters zeroed, transition history cleared", nil
			},
		},
	)
	if err != nil {
		t.Fatalf("resetter: %v", err)
	}

	mux := http.NewServeMux()
	mux.Handle("GET /api/demo/scenario", NewScenarioGetHandler(store))
	mux.Handle("PATCH /api/demo/scenario", NewScenarioPatchHandler(store, logger))
	mux.Handle("GET "+demos.ReliabilityBreakerPath, NewReliabilityBreakerHandler(console, logger))
	mux.Handle("POST "+demos.DemoResetPath, NewDemoResetHandler(resetter, logger))
	return Chain(
		RequestID,
		RequestLogger(logger),
		Recover(logger),
		EnvelopeMuxErrors(logger),
	)(mux), store
}

// demoResetDecodeBody decodes a JSON response body into a map.
func demoResetDecodeBody(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body is not JSON: %v\n%s", err, rec.Body.String())
	}
	return body
}

func TestDemoResetReturnsStatefulDemosToConstructedState(t *testing.T) {
	server, _ := newDemoResetServer(t)

	// Dirty both stateful areas through their real routes: the
	// scenario flips to live + error injection, and one breaker walk
	// under error injection trips the persistent breaker open.
	patch := doRequest(server, http.MethodPatch, "/api/demo/scenario",
		`{"dataMode":"live","injectFailure":"error"}`)
	if patch.Code != http.StatusOK {
		t.Fatalf("scenario patch status = %d: %s", patch.Code, patch.Body.String())
	}
	trip := doRequest(server, http.MethodGet, demos.ReliabilityBreakerPath, "")
	tripBody := demoResetDecodeBody(t, trip)
	breaker, _ := tripBody["breaker"].(map[string]any)
	if breaker["state"] != demos.ReliabilityBreakerOpen {
		t.Fatalf("pre-reset breaker state = %v, want open (the test must trip it first)", breaker["state"])
	}

	// One POST resets every registered area and answers with the
	// honest report.
	rec := doRequest(server, http.MethodPost, demos.DemoResetPath, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("reset status = %d: %s", rec.Code, rec.Body.String())
	}
	if rec.Header().Get(core.RequestIDHeader) == "" {
		t.Error("every response must carry x-request-id")
	}
	body := demoResetDecodeBody(t, rec)
	rows, _ := body["reset"].([]any)
	if len(rows) != 2 {
		t.Fatalf("reset rows = %d, want the two registered areas", len(rows))
	}
	byArea := map[string]map[string]any{}
	for _, row := range rows {
		rowMap, _ := row.(map[string]any)
		byArea[rowMap["area"].(string)] = rowMap
	}
	for _, area := range []string{"scenario", "reliability"} {
		if byArea[area] == nil {
			t.Fatalf("no reset row for area %q", area)
		}
		if byArea[area]["outcome"] != demos.DemoOutcomeReset {
			t.Errorf("area %q outcome = %v, want reset (detail: %v)",
				area, byArea[area]["outcome"], byArea[area]["detail"])
		}
	}
	notReset, _ := body["notReset"].([]any)
	if len(notReset) == 0 {
		t.Error("the report must name the areas deliberately not reset")
	}

	// The scenario is observably back at the constructed baseline.
	scenario := demoResetDecodeBody(t, doRequest(server, http.MethodGet, "/api/demo/scenario", ""))
	if scenario["dataMode"] != string(core.DataModeFake) || scenario["injectFailure"] != string(core.FailureNone) {
		t.Errorf("post-reset scenario = %v, want fake/none", scenario)
	}

	// The breaker is observably fresh: closed, empty history, and the
	// verification walk's own attempts all succeed under none.
	after := demoResetDecodeBody(t, doRequest(server, http.MethodGet, demos.ReliabilityBreakerPath, ""))
	breaker, _ = after["breaker"].(map[string]any)
	if breaker["state"] != demos.ReliabilityBreakerClosed {
		t.Errorf("post-reset breaker state = %v, want closed", breaker["state"])
	}
	if transitions, _ := breaker["transitions"].([]any); len(transitions) != 0 {
		t.Errorf("post-reset transitions = %d rows, want the history cleared", len(transitions))
	}
	walk, _ := after["walk"].(map[string]any)
	if walk["mode"] != string(core.FailureNone) {
		t.Errorf("post-reset walk mode = %v, want none", walk["mode"])
	}

	// Idempotent: a second reset answers the same honest 200.
	again := doRequest(server, http.MethodPost, demos.DemoResetPath, "")
	if again.Code != http.StatusOK {
		t.Errorf("second reset status = %d, want 200 (the sweep is idempotent)", again.Code)
	}
}

func TestDemoResetDegradesAFailingAreaAndStillAnswers(t *testing.T) {
	logger := newTestLogger(&strings.Builder{})
	resetter, err := demos.NewDemoResetter(
		demos.DemoResetParticipant{Area: "healthy", Label: "Healthy", Action: func() (string, error) {
			return "reset for real", nil
		}},
		demos.DemoResetParticipant{Area: "unreachable", Label: "Unreachable", Action: func() (string, error) {
			return "", core.ServiceUnavailable("the optional store is down")
		}},
	)
	if err != nil {
		t.Fatalf("resetter: %v", err)
	}
	mux := http.NewServeMux()
	mux.Handle("POST "+demos.DemoResetPath, NewDemoResetHandler(resetter, logger))
	server := Chain(RequestID, EnvelopeMuxErrors(logger))(mux)

	rec := doRequest(server, http.MethodPost, demos.DemoResetPath, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 — a degraded area never fails the sweep: %s", rec.Code, rec.Body.String())
	}
	body := demoResetDecodeBody(t, rec)
	rows, _ := body["reset"].([]any)
	if len(rows) != 2 {
		t.Fatalf("rows = %d, want both areas reported", len(rows))
	}
	degraded, _ := rows[1].(map[string]any)
	if degraded["outcome"] != demos.DemoOutcomeDegraded {
		t.Errorf("outcome = %v, want degraded", degraded["outcome"])
	}
	if detail, _ := degraded["detail"].(string); !strings.Contains(detail, string(core.KindServiceUnavailable)) {
		t.Errorf("detail %q must name the failure kind", detail)
	}
}

func TestDemoResetPropagatesBugsAsThe500Envelope(t *testing.T) {
	logger := newTestLogger(&strings.Builder{})
	resetter, err := demos.NewDemoResetter(
		demos.DemoResetParticipant{Area: "buggy", Label: "Buggy", Action: func() (string, error) {
			return "", errors.New("a plain bug, not a degraded demo")
		}},
	)
	if err != nil {
		t.Fatalf("resetter: %v", err)
	}
	mux := http.NewServeMux()
	mux.Handle("POST "+demos.DemoResetPath, NewDemoResetHandler(resetter, logger))
	server := Chain(RequestID, EnvelopeMuxErrors(logger))(mux)

	rec := doRequest(server, http.MethodPost, demos.DemoResetPath, "")
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500 — nothing is swallowed", rec.Code)
	}
	envelope := demoResetDecodeBody(t, rec)
	failure, _ := envelope["error"].(map[string]any)
	if failure["code"] != string(core.KindInternal) {
		t.Errorf("code = %v, want internal", failure["code"])
	}
	if rec.Header().Get(core.RequestIDHeader) == "" {
		t.Error("the 500 must carry x-request-id")
	}
}

func TestDemoResetRejectsNonPost(t *testing.T) {
	server, _ := newDemoResetServer(t)
	rec := doRequest(server, http.MethodGet, demos.DemoResetPath, "")
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("GET status = %d, want 405", rec.Code)
	}
	if allow := rec.Header().Get("Allow"); !strings.Contains(allow, http.MethodPost) {
		t.Errorf("Allow header = %q, want it to name POST", allow)
	}
	if !strings.Contains(rec.Body.String(), `"error"`) {
		t.Errorf("405 body is not the envelope: %s", rec.Body.String())
	}
}
