package presentation

import (
	"net/http"
	"strings"
	"testing"

	"guide/internal/content"
	"guide/internal/core"
)

func newScenarioServer(t *testing.T) (http.Handler, *core.ScenarioStore) {
	t.Helper()
	var buf strings.Builder
	registry, err := content.NewRegistry(nil, nil)
	if err != nil {
		t.Fatalf("empty registry: %v", err)
	}
	store := core.NewScenarioStore()
	return newAPIServer(registry, store, newTestLogger(&buf)), store
}

func TestScenarioGetServesDefaults(t *testing.T) {
	server, _ := newScenarioServer(t)
	rec := doRequest(server, http.MethodGet, "/api/demo/scenario", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if got, want := strings.TrimSuffix(rec.Body.String(), "\n"), `{"dataMode":"fake","injectFailure":"none"}`; got != want {
		t.Errorf("snapshot = %s, want %s", got, want)
	}
}

func TestScenarioPatchAppliesPartialUpdates(t *testing.T) {
	server, _ := newScenarioServer(t)

	rec := doRequest(server, http.MethodPatch, "/api/demo/scenario", `{"dataMode":"live"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("patch dataMode status = %d body %s", rec.Code, rec.Body.String())
	}
	if got, want := strings.TrimSuffix(rec.Body.String(), "\n"), `{"dataMode":"live","injectFailure":"none"}`; got != want {
		t.Errorf("patch answer = %s, want %s", got, want)
	}

	rec = doRequest(server, http.MethodPatch, "/api/demo/scenario", `{"injectFailure":"latency"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("patch injectFailure status = %d", rec.Code)
	}
	if got, want := strings.TrimSuffix(rec.Body.String(), "\n"), `{"dataMode":"live","injectFailure":"latency"}`; got != want {
		t.Errorf("partial patch must keep dataMode: %s, want %s", got, want)
	}

	// The GET reflects the same shared store the PATCH wrote.
	rec = doRequest(server, http.MethodGet, "/api/demo/scenario", "")
	if got := rec.Body.String(); got != `{"dataMode":"live","injectFailure":"latency"}`+"\n" {
		t.Errorf("GET after PATCH = %s", got)
	}

	// An empty object is a valid no-op, and unknown keys are ignored.
	for _, body := range []string{`{}`, `{"dataMode":"live","extraKey":"ignored"}`} {
		rec = doRequest(server, http.MethodPatch, "/api/demo/scenario", body)
		if rec.Code != http.StatusOK {
			t.Errorf("PATCH %s status = %d body %s", body, rec.Code, rec.Body.String())
		}
	}
}

func TestScenarioPatchRejectsInvalidBodies(t *testing.T) {
	server, store := newScenarioServer(t)

	// Establish a known state the invalid patches must not disturb.
	rec := doRequest(server, http.MethodPatch, "/api/demo/scenario", `{"dataMode":"live"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("setup patch status = %d", rec.Code)
	}

	garbage := []string{
		`{oops`,                        // not JSON
		`null`,                         // must be an object, not null
		`{"dataMode":42}`,              // wrong type
		`{"dataMode":true}`,            // wrong type
		`{"injectFailure":true}`,       // wrong type — the enum is a string
		`{"injectFailure":"bogus"}`,    // unknown enum value
		`{"dataMode":"bogus"}`,         // unknown enum value
		`{"dataMode":"live"} {"x":1}`,  // trailing second document
		`{"dataMode":"live"} trailing`, // trailing garbage
		`{"dataMode":"` + strings.Repeat("x", 8192) + `"}`, // over the size bound
		``, // empty body
	}
	for _, body := range garbage {
		t.Run(body[:min(len(body), 24)], func(t *testing.T) {
			rec := doRequest(server, http.MethodPatch, "/api/demo/scenario", body)
			if rec.Code != http.StatusUnprocessableEntity {
				t.Fatalf("status = %d body %s, want 422", rec.Code, rec.Body.String())
			}
			if !strings.Contains(rec.Body.String(), `"code":"validation_failed"`) {
				t.Errorf("body is not the validation envelope: %s", rec.Body.String())
			}
			if rec.Header().Get(core.RequestIDHeader) == "" {
				t.Error("422 must still carry x-request-id")
			}
		})
	}

	if state := store.Snapshot(); state.DataMode != core.DataModeLive || state.InjectFailure != core.FailureNone {
		t.Errorf("invalid patches mutated the store: %+v", state)
	}
}

func TestScenarioRoutingFailuresRideTheEnvelope(t *testing.T) {
	server, _ := newScenarioServer(t)

	rec := doRequest(server, http.MethodPost, "/api/demo/scenario", "")
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST status = %d, want 405", rec.Code)
	}
	if body := rec.Body.String(); !strings.Contains(body, `"code":"not_found"`) {
		t.Errorf("405 body is not the envelope: %s", body)
	}
}
