package presentation

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"guide/internal/content"
	"guide/internal/core"
)

// The go-language topic's demo endpoint, exercised through the same
// middleware chain production runs behind — the family rule that every
// new demo endpoint ships with a Go test.

func TestRuntimeTypesDemoServesThePayload(t *testing.T) {
	var buf strings.Builder
	server := newAPIServer(content.Registry{}, core.NewScenarioStore(), newTestLogger(&buf))

	rec := doRequest(server, http.MethodGet, "/api/demo/go/runtime-types", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("content type = %q, want application/json", ct)
	}
	if rec.Header().Get(core.RequestIDHeader) == "" {
		t.Error("every response must carry x-request-id")
	}

	// Spot-check the beats the topic's prose promises, not the whole body:
	// the payload builder's own tests pin the deep shape.
	decoded := struct {
		Inspections []struct {
			Expression string `json:"expression"`
			Kind       string `json:"kind"`
			PkgPath    string `json:"pkgPath"`
		} `json:"inspections"`
		Fields []struct {
			Name    string `json:"name"`
			JSONTag string `json:"jsonTag"`
		} `json:"fields"`
		Assertions []struct {
			DynamicType string `json:"dynamicType"`
			OK          bool   `json:"ok"`
		} `json:"assertions"`
		Enforcement struct {
			Function        string `json:"function"`
			CompilerVerdict string `json:"compilerVerdict"`
		} `json:"enforcement"`
		Boundary struct {
			Outcome   string `json:"outcome"`
			ErrorType string `json:"errorType"`
		} `json:"boundary"`
	}{}
	if err := json.Unmarshal(rec.Body.Bytes(), &decoded); err != nil {
		t.Fatalf("body not the demo DTO shape: %v\n%s", err, rec.Body.String())
	}
	if len(decoded.Inspections) != 9 || decoded.Inspections[0].PkgPath != "guide/internal/content" {
		t.Errorf("inspections drifted: %+v", decoded.Inspections)
	}
	if len(decoded.Fields) != 4 || decoded.Fields[0].JSONTag != "id" {
		t.Errorf("field walk drifted: %+v", decoded.Fields)
	}
	if len(decoded.Assertions) != 5 || decoded.Assertions[4].OK {
		t.Errorf("assertions drifted: %+v", decoded.Assertions)
	}
	if decoded.Enforcement.Function != "repeat" || decoded.Enforcement.CompilerVerdict == "" {
		t.Errorf("enforcement beat drifted: %+v", decoded.Enforcement)
	}
	if decoded.Boundary.Outcome != "rejected" || decoded.Boundary.ErrorType != "*json.UnmarshalTypeError" {
		t.Errorf("boundary beat drifted: %+v", decoded.Boundary)
	}
}

func TestRuntimeTypesDemoRejectsNonGet(t *testing.T) {
	var buf strings.Builder
	server := newAPIServer(content.Registry{}, nil, newTestLogger(&buf))

	rec := doRequest(server, http.MethodPost, "/api/demo/go/runtime-types", "")
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"code":"not_found"`) {
		t.Errorf("405 body is not the envelope: %s", rec.Body.String())
	}
	if allow := rec.Header().Get("Allow"); !strings.Contains(allow, "GET") {
		t.Errorf("Allow header lost: %q", allow)
	}
}

func TestConcurrencyDemoServesThePayload(t *testing.T) {
	var buf strings.Builder
	server := newAPIServer(content.Registry{}, core.NewScenarioStore(), newTestLogger(&buf))

	rec := doRequest(server, http.MethodGet, "/api/demo/runtime/concurrency", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("content type = %q, want application/json", ct)
	}
	if rec.Header().Get(core.RequestIDHeader) == "" {
		t.Error("every response must carry x-request-id")
	}

	// Spot-check the beats the topic's prose promises; the builder's own
	// tests pin the deep shape and the determinism.
	decoded := struct {
		Setup struct {
			Jobs        int `json:"jobs"`
			Workers     int `json:"workers"`
			CancelAfter int `json:"cancelAfter"`
		} `json:"setup"`
		FanOut []struct {
			JobID  string `json:"jobId"`
			Output string `json:"output"`
		} `json:"fanOut"`
		WorkerCounts []struct {
			WorkerID string `json:"workerId"`
		} `json:"workerCounts"`
		Cancellation struct {
			JobsSubmitted    int `json:"jobsSubmitted"`
			ResultsCollected int `json:"resultsCollected"`
			JobsAbandoned    int `json:"jobsAbandoned"`
			WorkersStopped   int `json:"workersStopped"`
		} `json:"cancellation"`
		Timeouts []struct {
			Winner   string `json:"winner"`
			Received string `json:"received"`
		} `json:"timeouts"`
		Runtime struct {
			GOMAXPROCS int `json:"gomaxprocs"`
		} `json:"runtime"`
	}{}
	if err := json.Unmarshal(rec.Body.Bytes(), &decoded); err != nil {
		t.Fatalf("body not the demo DTO shape: %v\n%s", err, rec.Body.String())
	}
	if decoded.Setup.Jobs != 8 || decoded.Setup.Workers != 3 || decoded.Setup.CancelAfter != 5 {
		t.Errorf("setup drifted: %+v", decoded.Setup)
	}
	if len(decoded.FanOut) != 8 || decoded.FanOut[0].JobID != "job-01" || decoded.FanOut[0].Output == "" {
		t.Errorf("fan-out drifted: %+v", decoded.FanOut)
	}
	if len(decoded.WorkerCounts) != 3 {
		t.Errorf("worker counts drifted: %+v", decoded.WorkerCounts)
	}
	if decoded.Cancellation.JobsAbandoned != 3 || decoded.Cancellation.ResultsCollected != 5 ||
		decoded.Cancellation.WorkersStopped != 3 || decoded.Cancellation.JobsSubmitted != 8 {
		t.Errorf("cancellation ledger drifted: %+v", decoded.Cancellation)
	}
	if len(decoded.Timeouts) != 2 || decoded.Timeouts[1].Winner != "timer" || decoded.Timeouts[0].Winner != "reply" {
		t.Errorf("timeout races drifted: %+v", decoded.Timeouts)
	}
	if decoded.Runtime.GOMAXPROCS < 1 {
		t.Errorf("runtime facts drifted: %+v", decoded.Runtime)
	}
}

func TestConcurrencyDemoRejectsNonGet(t *testing.T) {
	var buf strings.Builder
	server := newAPIServer(content.Registry{}, nil, newTestLogger(&buf))

	rec := doRequest(server, http.MethodPost, "/api/demo/runtime/concurrency", "")
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"code":"not_found"`) {
		t.Errorf("405 body is not the envelope: %s", rec.Body.String())
	}
	if allow := rec.Header().Get("Allow"); !strings.Contains(allow, "GET") {
		t.Errorf("Allow header lost: %q", allow)
	}
}
