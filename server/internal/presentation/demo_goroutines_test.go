package presentation

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"guide/internal/core"
)

// The goroutines-and-channels topic's two demo endpoints, exercised
// through the same middleware chain production runs behind (the family
// rule that every new demo endpoint ships with a Go test). Routes live in
// this test's own mux — the main.go wiring lands with the wave's shared
// wiring, not here.

// newConcDemoServer builds the demo routes behind the production chain.
func newConcDemoServer() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("GET /api/demo/conc/context", NewContextCancellationHandler())
	mux.Handle("GET /api/demo/conc/race", NewRaceDetectorHandler())
	var buf strings.Builder
	logger := newTestLogger(&buf)
	return Chain(
		RequestID,
		RequestLogger(logger),
		Recover(logger),
		EnvelopeMuxErrors(logger),
	)(mux)
}

func TestContextCancellationDemoServesThePayload(t *testing.T) {
	server := newConcDemoServer()

	rec := doRequest(server, http.MethodGet, "/api/demo/conc/context", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("content type = %q, want application/json", ct)
	}
	if rec.Header().Get(core.RequestIDHeader) == "" {
		t.Error("every response must carry x-request-id")
	}

	// Spot-check the beats the topic's prose promises; the payload
	// builder's own tests pin the deep shape.
	decoded := struct {
		Choreography struct {
			DeadlineBudgetMs int    `json:"deadlineBudgetMs"`
			Trigger          string `json:"trigger"`
		} `json:"choreography"`
		Propagation []struct {
			WorkerID         string `json:"workerId"`
			ObservedPosition int    `json:"observedPosition"`
			Cause            string `json:"cause"`
		} `json:"propagation"`
		DeadlineBeat struct {
			Cause            string `json:"cause"`
			DeadlineExceeded bool   `json:"deadlineExceeded"`
		} `json:"deadlineBeat"`
		Ledger struct {
			Outcome string `json:"outcome"`
		} `json:"ledger"`
	}{}
	if err := json.Unmarshal(rec.Body.Bytes(), &decoded); err != nil {
		t.Fatalf("body not the demo DTO shape: %v\n%s", err, rec.Body.String())
	}
	if decoded.Choreography.DeadlineBudgetMs != 400 || decoded.Choreography.Trigger == "" {
		t.Errorf("choreography drifted: %+v", decoded.Choreography)
	}
	if len(decoded.Propagation) != 3 || decoded.Propagation[0].WorkerID != "alpha" ||
		decoded.Propagation[2].ObservedPosition != 3 {
		t.Errorf("propagation drifted: %+v", decoded.Propagation)
	}
	for _, row := range decoded.Propagation {
		if row.Cause != "context canceled" {
			t.Errorf("propagation cause = %q, want context canceled", row.Cause)
		}
	}
	if decoded.DeadlineBeat.Cause != "context deadline exceeded" || !decoded.DeadlineBeat.DeadlineExceeded {
		t.Errorf("deadline beat drifted: %+v", decoded.DeadlineBeat)
	}
	if decoded.Ledger.Outcome == "" {
		t.Error("ledger must state its leak-free outcome")
	}
}

func TestRaceDetectorDemoServesThePayload(t *testing.T) {
	server := newConcDemoServer()

	rec := doRequest(server, http.MethodGet, "/api/demo/conc/race", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if rec.Header().Get(core.RequestIDHeader) == "" {
		t.Error("every response must carry x-request-id")
	}

	decoded := struct {
		Race struct {
			Command string `json:"command"`
			Verdict string `json:"verdict"`
		} `json:"race"`
		Mutex struct {
			Observed int  `json:"observed"`
			Exact    bool `json:"exact"`
		} `json:"mutex"`
		Channel struct {
			Observed int  `json:"observed"`
			Exact    bool `json:"exact"`
		} `json:"channel"`
		Hygiene []struct {
			ReaderPresent bool   `json:"readerPresent"`
			Outcome       string `json:"outcome"`
		} `json:"hygiene"`
	}{}
	if err := json.Unmarshal(rec.Body.Bytes(), &decoded); err != nil {
		t.Fatalf("body not the demo DTO shape: %v\n%s", err, rec.Body.String())
	}
	if decoded.Mutex.Observed != 5000 || !decoded.Mutex.Exact {
		t.Errorf("mutex run = %+v, want exactly 5000", decoded.Mutex)
	}
	if decoded.Channel.Observed != 5000 || !decoded.Channel.Exact {
		t.Errorf("channel run = %+v, want exactly 5000", decoded.Channel)
	}
	if decoded.Race.Command != "go test -race -count=1 ." ||
		!strings.Contains(decoded.Race.Verdict, "WARNING: DATA RACE") {
		t.Errorf("pinned exhibit drifted: %+v", decoded.Race)
	}
	if len(decoded.Hygiene) != 2 || decoded.Hygiene[0].ReaderPresent == decoded.Hygiene[1].ReaderPresent {
		t.Errorf("hygiene contrast drifted: %+v", decoded.Hygiene)
	}
}

func TestConcDemoEndpointsRejectNonGet(t *testing.T) {
	server := newConcDemoServer()

	for _, path := range []string{"/api/demo/conc/context", "/api/demo/conc/race"} {
		rec := doRequest(server, http.MethodPost, path, "")
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("POST %s status = %d, want 405", path, rec.Code)
		}
		if !strings.Contains(rec.Body.String(), `"code":"not_found"`) {
			t.Errorf("POST %s body is not the envelope: %s", path, rec.Body.String())
		}
		if allow := rec.Header().Get("Allow"); !strings.Contains(allow, "GET") {
			t.Errorf("POST %s lost the Allow header: %q", path, allow)
		}
	}
}
