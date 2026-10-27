package presentation

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"testing"

	"guide/internal/core"
)

// The go-http-fundamentals topic's demo endpoint, exercised through the
// same middleware chain production runs behind. The route lives in THIS
// test's mux because main.go wiring lands with the master's wave close —
// the handler contract is pinned here regardless.

func newAppInfoServer(logger *slog.Logger) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("GET /api/demo/gohttp/app-info", NewAppInfoHandler(core.Config{
		Port:     8100,
		Env:      "dev",
		LogLevel: slog.LevelInfo,
	}))
	return Chain(
		RequestID,
		RequestLogger(logger),
		Recover(logger),
		EnvelopeMuxErrors(logger),
	)(mux)
}

func TestAppInfoDemoServesThePayload(t *testing.T) {
	var logBuf strings.Builder
	server := newAppInfoServer(newTestLogger(&logBuf))

	rec := doRequest(server, http.MethodGet, "/api/demo/gohttp/app-info", "")
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
		Module struct {
			Path            string `json:"path"`
			Version         string `json:"version"`
			GoVersion       string `json:"goVersion"`
			DependencyCount int    `json:"dependencyCount"`
		} `json:"module"`
		Runtime struct {
			NumCPU     int `json:"numCPU"`
			GoMaxProcs int `json:"goMaxProcs"`
		} `json:"runtime"`
		Environment struct {
			Port     int    `json:"port"`
			Env      string `json:"env"`
			LogLevel string `json:"logLevel"`
		} `json:"environment"`
		BuildSettings []struct {
			Key   string `json:"key"`
			Value string `json:"value"`
		} `json:"buildSettings"`
	}{}
	if err := json.Unmarshal(rec.Body.Bytes(), &decoded); err != nil {
		t.Fatalf("body not the demo DTO shape: %v\n%s", err, rec.Body.String())
	}
	// 12 = the deliberate-direct-requires count since the Phase 5 deps
	// landed (grpc-go, gqlgen, pgx, …) — derived live, pinned here.
	// The count is binary-scoped: this package's test binary links the
	// auth engines (argon2 rides along) and the security stack (x/time
	// rides along), so it sees two more modules than the demos package's
	// test binary — whose pinned 16 the go-http-fundamentals topic quotes
	// verbatim. Both pins are live facts about their own binaries; the
	// served payload describes the SERVER binary, which links the full
	// graph.
	if decoded.Module.Path != "guide" || decoded.Module.DependencyCount != 18 {
		t.Errorf("module drifted: %+v", decoded.Module)
	}
	if decoded.Module.Version == "" || !strings.HasPrefix(decoded.Module.GoVersion, "go1.") {
		t.Errorf("module honesty drifted: %+v", decoded.Module)
	}
	if decoded.Runtime.NumCPU <= 0 || decoded.Runtime.GoMaxProcs <= 0 {
		t.Errorf("runtime drifted: %+v", decoded.Runtime)
	}
	if decoded.Environment.Port != 8100 || decoded.Environment.Env != "dev" ||
		decoded.Environment.LogLevel != "INFO" {
		t.Errorf("environment drifted: %+v", decoded.Environment)
	}
	if decoded.BuildSettings == nil {
		t.Error("buildSettings must marshal as [] — never null")
	}
	for i := 1; i < len(decoded.BuildSettings); i++ {
		if decoded.BuildSettings[i-1].Key > decoded.BuildSettings[i].Key {
			t.Errorf("build settings not sorted: %q before %q",
				decoded.BuildSettings[i-1].Key, decoded.BuildSettings[i].Key)
		}
	}
	if !strings.Contains(logBuf.String(), "http_request") {
		t.Error("the access log must record the demo request")
	}
}

func TestAppInfoDemoRejectsNonGet(t *testing.T) {
	var logBuf strings.Builder
	server := newAppInfoServer(newTestLogger(&logBuf))

	rec := doRequest(server, http.MethodPost, "/api/demo/gohttp/app-info", "")
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
