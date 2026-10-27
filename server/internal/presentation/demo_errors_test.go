package presentation

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"guide/internal/core"
)

// The errors demo pair, exercised through the same middleware chain
// production runs behind (the family rule that every new demo endpoint
// ships with a Go test). Each test owns a fresh ScenarioStore — flips
// never leak across tests — and the latency path runs on a fake sleep
// that records the armed delay instead of waiting it out.

// fakeSleep records the delays it was asked to sleep and returns
// immediately.
type fakeSleep struct {
	slept []time.Duration
}

func (f *fakeSleep) sleep(d time.Duration) { f.slept = append(f.slept, d) }

func newErrorsDemoServer(t *testing.T, store *core.ScenarioStore) http.Handler {
	t.Helper()
	logger := newTestLogger(&strings.Builder{})
	mux := http.NewServeMux()
	mux.Handle("GET /api/demo/errors/explorer", NewErrorsExplorerHandler(store, logger))
	mux.Handle("GET /api/demo/errors/probe", NewErrorsProbeHandler(store))
	return Chain(
		RequestID,
		RequestLogger(logger),
		Recover(logger),
		EnvelopeMuxErrors(logger),
	)(mux)
}

// mustPatchInjection flips the store's failure injection and restores
// nothing — each test's store is throwaway, but the helper keeps the
// flips honest and loud.
func mustPatchInjection(t *testing.T, store *core.ScenarioStore, mode core.FailureInjection) {
	t.Helper()
	if _, err := store.Update(core.ScenarioPatch{InjectFailure: &mode}); err != nil {
		t.Fatalf("patch injectFailure=%s: %v", mode, err)
	}
}

type explorerWalkDTO struct {
	InjectFailure string `json:"injectFailure"`
	Status        int    `json:"status"`
	RequestID     string `json:"requestID"`
	Body          string `json:"body"`
	Note          string `json:"note"`
}

type explorerPayloadDTO struct {
	Kinds []struct {
		Kind       string `json:"kind"`
		HTTPStatus int    `json:"httpStatus"`
		Trigger    struct {
			Mode string `json:"mode"`
		} `json:"trigger"`
	} `json:"kinds"`
	Walk explorerWalkDTO `json:"walk"`
}

func decodeExplorer(t *testing.T, body string) explorerPayloadDTO {
	t.Helper()
	var payload explorerPayloadDTO
	if err := json.Unmarshal([]byte(body), &payload); err != nil {
		t.Fatalf("explorer payload not the demo DTO shape: %v\n%s", err, body)
	}
	return payload
}

func TestErrorsExplorerServesTheTableAndAWalk(t *testing.T) {
	store := core.NewScenarioStore()
	server := newErrorsDemoServer(t, store)

	rec := doRequest(server, http.MethodGet, "/api/demo/errors/explorer", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if rec.Header().Get(core.RequestIDHeader) == "" {
		t.Error("every response must carry x-request-id")
	}

	payload := decodeExplorer(t, rec.Body.String())
	if len(payload.Kinds) != 8 {
		t.Errorf("kinds = %d rows, want the taxonomy's 8", len(payload.Kinds))
	}
	if payload.Walk.InjectFailure != "none" || payload.Walk.Status != 200 {
		t.Errorf("healthy walk = %+v, want none/200", payload.Walk)
	}
	if !strings.Contains(payload.Walk.Body, `"status":"healthy"`) {
		t.Errorf("healthy walk body must be the probe's row: %q", payload.Walk.Body)
	}
	if !strings.Contains(payload.Walk.Note, "nothing injected") {
		t.Errorf("healthy walk note must derive the observation: %q", payload.Walk.Note)
	}
	if payload.Walk.RequestID == "" || payload.Walk.RequestID != rec.Header().Get(core.RequestIDHeader) {
		t.Errorf(
			"walk request id %q must equal the explorer's response id %q — the thread the demo proves",
			payload.Walk.RequestID, rec.Header().Get(core.RequestIDHeader),
		)
	}
}

// TestErrorsExplorerWalksTheRecoveredPanic pins the demo's headline
// beat: with error injection armed, the walk observes the probe's
// deliberate panic converted into the generic 500 envelope, and ONE
// request id threads the explorer response, the probe's envelope, and
// the walk row.
func TestErrorsExplorerWalksTheRecoveredPanic(t *testing.T) {
	store := core.NewScenarioStore()
	mustPatchInjection(t, store, core.FailureError)
	server := newErrorsDemoServer(t, store)

	rec := doRequest(server, http.MethodGet, "/api/demo/errors/explorer", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("explorer status = %d, want 200 (only the probe degrades)", rec.Code)
	}

	payload := decodeExplorer(t, rec.Body.String())
	if payload.Walk.InjectFailure != "error" || payload.Walk.Status != 500 {
		t.Fatalf("error-mode walk = %+v, want error/500", payload.Walk)
	}
	if !strings.Contains(payload.Walk.Body, `"code":"internal"`) {
		t.Errorf("walk body must be the recovered 500 envelope: %q", payload.Walk.Body)
	}
	if !strings.Contains(payload.Walk.Body, `"requestId":"`+payload.Walk.RequestID+`"`) {
		t.Errorf("the envelope's requestId must equal the walk's id %q: %q", payload.Walk.RequestID, payload.Walk.Body)
	}
	if !strings.Contains(payload.Walk.Note, "error injection honored") {
		t.Errorf("error-mode walk note must derive the observation: %q", payload.Walk.Note)
	}
}

// TestErrorsExplorerWalksTheLatencyInjection runs the latency path on
// a fake sleep (no wall-clock wait): the walk still reports the armed
// mode and the slept delay, honestly measured end to end.
func TestErrorsExplorerWalksTheLatencyInjection(t *testing.T) {
	store := core.NewScenarioStore()
	mustPatchInjection(t, store, core.FailureLatency)
	logger := newTestLogger(&strings.Builder{})
	explorer := NewErrorsExplorerHandler(store, logger)
	sleeper := &fakeSleep{}
	explorer.probe.sleep = sleeper.sleep

	rec := doRequest(explorer, http.MethodGet, "/api/demo/errors/explorer", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if len(sleeper.slept) != 1 || sleeper.slept[0] != 1500*time.Millisecond {
		t.Fatalf("fake sleep = %v, want exactly one 1500ms injection", sleeper.slept)
	}

	payload := decodeExplorer(t, rec.Body.String())
	if payload.Walk.InjectFailure != "latency" || payload.Walk.Status != 200 {
		t.Fatalf("latency-mode walk = %+v, want latency/200", payload.Walk)
	}
	if !strings.Contains(payload.Walk.Note, "slept out its 1500 ms injection") {
		t.Errorf("latency walk note must derive the observation: %q", payload.Walk.Note)
	}
}

func TestErrorsProbeAnswersHealthyByDefault(t *testing.T) {
	store := core.NewScenarioStore()
	server := newErrorsDemoServer(t, store)

	rec := doRequest(server, http.MethodGet, "/api/demo/errors/probe", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{`"status":"healthy"`, `"injectFailure":"none"`, `"detail":`} {
		if !strings.Contains(body, want) {
			t.Errorf("probe body missing %q: %s", want, body)
		}
	}
}

// TestErrorsProbeUnderErrorInjectionIsTheRecovered500 pins the probe
// itself: armed with error injection, a direct call through the
// production chain answers the generic 500 internal envelope with the
// request id stamped — the client sees no stack, the log sees all of it.
func TestErrorsProbeUnderErrorInjectionIsTheRecovered500(t *testing.T) {
	store := core.NewScenarioStore()
	mustPatchInjection(t, store, core.FailureError)
	server := newErrorsDemoServer(t, store)

	rec := doRequest(server, http.MethodGet, "/api/demo/errors/probe", "")
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	decoded := struct {
		Error struct {
			Code      string `json:"code"`
			Message   string `json:"message"`
			RequestID string `json:"requestId"`
		} `json:"error"`
	}{}
	if err := json.Unmarshal(rec.Body.Bytes(), &decoded); err != nil {
		t.Fatalf("not an envelope: %s", rec.Body.String())
	}
	if decoded.Error.Code != "internal" || decoded.Error.Message != "Internal server error." {
		t.Errorf("envelope = %+v, want the generic internal one", decoded.Error)
	}
	if decoded.Error.RequestID == "" || decoded.Error.RequestID != rec.Header().Get(core.RequestIDHeader) {
		t.Errorf("request id %q must match the response header %q", decoded.Error.RequestID, rec.Header().Get(core.RequestIDHeader))
	}
}

func TestErrorsProbeUnderLatencyInjectionSleptTheDelay(t *testing.T) {
	store := core.NewScenarioStore()
	mustPatchInjection(t, store, core.FailureLatency)
	probe := NewErrorsProbeHandler(store)
	sleeper := &fakeSleep{}
	probe.sleep = sleeper.sleep

	rec := doRequest(probe, http.MethodGet, "/api/demo/errors/probe", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 after the sleep", rec.Code)
	}
	if len(sleeper.slept) != 1 || sleeper.slept[0] != 1500*time.Millisecond {
		t.Errorf("fake sleep = %v, want one 1500ms injection", sleeper.slept)
	}
	if !strings.Contains(rec.Body.String(), `"injectFailure":"latency"`) {
		t.Errorf("body must mirror the served mode: %s", rec.Body.String())
	}
}

func TestErrorsDemoRejectsNonGet(t *testing.T) {
	store := core.NewScenarioStore()
	server := newErrorsDemoServer(t, store)

	for _, path := range []string{"/api/demo/errors/explorer", "/api/demo/errors/probe"} {
		rec := doRequest(server, http.MethodPost, path, "")
		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("POST %s status = %d, want 405", path, rec.Code)
		}
		if !strings.Contains(rec.Body.String(), `"code":"not_found"`) {
			t.Errorf("POST %s body must be the envelope: %s", path, rec.Body.String())
		}
		if allow := rec.Header().Get("Allow"); !strings.Contains(allow, "GET") {
			t.Errorf("POST %s Allow header lost: %q", path, allow)
		}
	}
}
