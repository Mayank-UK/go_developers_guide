package presentation

import (
	"encoding/json"
	"net/http"
	"runtime"
	"strings"
	"testing"
	"time"

	"guide/internal/core"
	"guide/internal/domain/demos"
)

// The stall endpoint through the production middleware chain. Tests
// drive the handler with injected runs (the parsed shapes below mirror
// the real runtime profile the builder produces), and one test runs
// the REAL stall twice through the chain — the demo's repeatability
// and its no-leak release proven end to end.

// debugStallFixtureRun mirrors one real run: five workers, four
// healthy, one wedged and found by its marker, with the request's own
// goroutine as the contrast.
func debugStallFixtureRun() demos.DebugStallRun {
	return demos.DebugStallRun{
		Workers:       demos.DebugStallWorkers,
		HealthyDone:   demos.DebugStallWorkers - 1,
		CaptureMs:     0.31,
		CaptureTries:  1,
		GoroutineSeen: 9,
		Stalled: &demos.GoroutineSnapshot{
			ID:    42,
			State: "chan receive",
			Wait:  "1 minutes",
			Frames: []demos.GoroutineFrame{
				{Function: "guide/internal/domain/demos.debugStallWedge(0x0)",
					Location: "server/internal/domain/demos/debug_stall.go:118"},
			},
			CreatedBy: "guide/internal/domain/demos.RunDebugStall",
			CreatedAt: "server/internal/domain/demos/debug_stall.go:157",
		},
		Contrast: &demos.GoroutineSnapshot{
			ID:    1,
			State: "running",
			Frames: []demos.GoroutineFrame{
				{Function: "guide/internal/domain/demos.RunDebugStall(0x0)",
					Location: "server/internal/domain/demos/debug_stall.go:201"},
			},
		},
		AllExited: true,
	}
}

func newDebugStallServer(runStall func() demos.DebugStallRun) http.Handler {
	logger := newTestLogger(&strings.Builder{})
	handler := NewDebugStallHandler(logger)
	handler.runStall = runStall
	mux := http.NewServeMux()
	mux.Handle("GET "+demos.DebugStallPath, handler)
	return Chain(
		RequestID,
		RequestLogger(logger),
		Recover(logger),
		EnvelopeMuxErrors(logger),
	)(mux)
}

func TestDebugStallServesTheCapturedAnatomy(t *testing.T) {
	server := newDebugStallServer(debugStallFixtureRun)

	rec := doRequest(server, http.MethodGet, demos.DebugStallPath, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if rec.Header().Get(core.RequestIDHeader) == "" {
		t.Error("every response must carry x-request-id")
	}

	var payload struct {
		Mode     string `json:"mode"`
		Workload struct {
			Workers     int    `json:"workers"`
			HealthyDone int    `json:"healthyDone"`
			Wedge       string `json:"wedge"`
		} `json:"workload"`
		Capture struct {
			Profile        string  `json:"profile"`
			GoroutinesSeen int     `json:"goroutinesSeen"`
			CaptureMs      float64 `json:"captureMs"`
			CaptureTries   int     `json:"captureTries"`
			Curl           string  `json:"curl"`
		} `json:"capture"`
		Stalled struct {
			ID     int    `json:"id"`
			State  string `json:"state"`
			Wait   string `json:"wait"`
			Frames []struct {
				Function string `json:"function"`
				Location string `json:"location"`
			} `json:"frames"`
			CreatedBy string `json:"createdBy"`
		} `json:"stalled"`
		Contrast struct {
			ID    int    `json:"id"`
			State string `json:"state"`
		} `json:"contrast"`
		Endpoints []struct {
			Path    string `json:"path"`
			Profile string `json:"profile"`
		} `json:"endpoints"`
		Reasoning string `json:"reasoning"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("payload not the demo DTO shape: %v\n%s", err, rec.Body.String())
	}

	if payload.Workload.Workers != demos.DebugStallWorkers || payload.Workload.HealthyDone != 4 {
		t.Errorf("workload = %+v", payload.Workload)
	}
	if !strings.Contains(payload.Workload.Wedge, "channel receive") {
		t.Errorf("wedge description = %q", payload.Workload.Wedge)
	}
	if payload.Capture.Profile != "goroutine" || payload.Capture.GoroutinesSeen != 9 {
		t.Errorf("capture = %+v", payload.Capture)
	}
	if !strings.Contains(payload.Capture.Curl, "/internal/debug/pprof/goroutine") {
		t.Errorf("capture curl must point at the live endpoint: %q", payload.Capture.Curl)
	}
	if payload.Stalled.ID != 42 || payload.Stalled.State != "chan receive" || payload.Stalled.Wait != "1 minutes" {
		t.Errorf("stalled = %+v", payload.Stalled)
	}
	if len(payload.Stalled.Frames) != 1 || payload.Stalled.Frames[0].Location == "" {
		t.Errorf("stalled frames = %+v — every frame must carry its source line", payload.Stalled.Frames)
	}
	if !strings.Contains(payload.Stalled.CreatedBy, "RunDebugStall") {
		t.Errorf("createdBy = %q", payload.Stalled.CreatedBy)
	}
	if payload.Contrast.ID != 1 || payload.Contrast.State != "running" {
		t.Errorf("contrast = %+v", payload.Contrast)
	}
	if len(payload.Endpoints) == 0 {
		t.Error("the payload must point at the live pprof endpoints")
	}
	for _, endpoint := range payload.Endpoints {
		if !strings.HasPrefix(endpoint.Path, "/internal/debug/pprof/") {
			t.Errorf("endpoint %q must sit under the internal prefix", endpoint.Path)
		}
	}
	if !strings.Contains(payload.Mode, "runtime-derived") {
		t.Errorf("mode must state the provenance: %q", payload.Mode)
	}
}

// TestDebugStallRunsForRealThroughTheChain executes the production
// runner twice through the chain — repeatability, honest stacks, and
// the release proven by the process's own count settling.
func TestDebugStallRunsForRealThroughTheChain(t *testing.T) {
	before := runtime.NumGoroutine()
	logger := newTestLogger(&strings.Builder{})
	handler := NewDebugStallHandler(logger)
	mux := http.NewServeMux()
	mux.Handle("GET "+demos.DebugStallPath, handler)
	server := Chain(RequestID, EnvelopeMuxErrors(logger))(mux)

	for attempt := 1; attempt <= 2; attempt++ {
		rec := doRequest(server, http.MethodGet, demos.DebugStallPath, "")
		if rec.Code != http.StatusOK {
			t.Fatalf("attempt %d: status = %d, want 200: %s", attempt, rec.Code, rec.Body.String())
		}
		var payload struct {
			Workload struct {
				HealthyDone int `json:"healthyDone"`
				Workers     int `json:"workers"`
			} `json:"workload"`
			Stalled struct {
				State  string `json:"state"`
				Frames []struct {
					Function string `json:"function"`
					Location string `json:"location"`
				} `json:"frames"`
			} `json:"stalled"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
			t.Fatalf("attempt %d: decode: %v", attempt, err)
		}
		if payload.Workload.HealthyDone != payload.Workload.Workers-1 {
			t.Errorf("attempt %d: healthy done = %d of %d", attempt, payload.Workload.HealthyDone, payload.Workload.Workers)
		}
		if payload.Stalled.State != "chan receive" {
			t.Errorf("attempt %d: stalled state = %q, want chan receive", attempt, payload.Stalled.State)
		}
		foundWedge := false
		for _, frame := range payload.Stalled.Frames {
			if strings.Contains(frame.Function, "debugStallWedge") && strings.Contains(frame.Location, "debug_stall.go:") {
				foundWedge = true
			}
		}
		if !foundWedge {
			t.Errorf("attempt %d: stalled frames = %+v — the wedge frame with its source line must be served", attempt, payload.Stalled.Frames)
		}
	}

	// The release is proven: no wedge frame on any goroutine, and the
	// count settles back to baseline.
	deadline := time.Now().Add(3 * time.Second)
	for runtime.NumGoroutine() > before && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if after := runtime.NumGoroutine(); after > before {
		t.Errorf("goroutines %d → %d after two stalls — the wedges leaked", before, after)
	}
}

// TestDebugStallRefusesToInventAStack serves a run whose wedge was
// never found: the honest typed 500, never a fabricated stack.
func TestDebugStallRefusesToInventAStack(t *testing.T) {
	server := newDebugStallServer(func() demos.DebugStallRun {
		run := debugStallFixtureRun()
		run.Stalled = nil
		return run
	})

	rec := doRequest(server, http.MethodGet, demos.DebugStallPath, "")
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want the typed 500", rec.Code)
	}
	var envelope struct {
		Error struct {
			Code      string `json:"code"`
			RequestID string `json:"requestId"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("body not the error envelope: %v\n%s", err, rec.Body.String())
	}
	if envelope.Error.Code != "internal" || !strings.Contains(rec.Body.String(), "no stack was invented") {
		t.Errorf("envelope = %+v, want internal naming the honest refusal", envelope.Error)
	}
}

func TestDebugStallRejectsNonGet(t *testing.T) {
	server := newDebugStallServer(debugStallFixtureRun)
	rec := doRequest(server, http.MethodPost, demos.DebugStallPath, "")
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST status = %d, want 405", rec.Code)
	}
}
