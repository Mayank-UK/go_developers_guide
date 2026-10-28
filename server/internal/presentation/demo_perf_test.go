package presentation

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"guide/internal/core"
	"guide/internal/domain/demos"
)

// The performance bench endpoint through the production middleware
// chain. The handler tests drive the state machine with an injected
// runner (canned output in the go tool's real shapes); one test
// executes the REAL runner against the module tree beside the test
// binary — both commands, end to end — and one proves the bounded
// runner actually kills a runaway process group.

// perfBenchFixtureOutput is a captured-shape bench run: headers, the
// curve with duplicate samples, and both encode approaches.
func perfBenchFixtureOutput() string {
	return strings.Join([]string{
		"goos: linux",
		"goarch: arm64",
		"pkg: guide/internal/domain/demos",
		"cpu: 13th Gen Intel(R) Core(TM) i7-13700K",
		"BenchmarkPerfGzipCompress/level=1/size=4KB-16     10    93000 ns/op    24576 B/op    3 allocs/op",
		"BenchmarkPerfGzipCompress/level=1/size=4KB-16     10   101000 ns/op    24576 B/op    3 allocs/op",
		"BenchmarkPerfGzipCompress/level=9/size=4KB-16     10   240000 ns/op    24576 B/op    3 allocs/op",
		"BenchmarkPerfJSONEncode/approach=marshal-16       10    41000 ns/op    65536 B/op    2 allocs/op",
		"BenchmarkPerfJSONEncode/approach=stream-16        10    39000 ns/op     1024 B/op    4 allocs/op",
		"BenchmarkPerfInMemoryPostsList-16                 10     2100 ns/op     1224 B/op    9 allocs/op",
		"PASS",
		"ok  \tguide/internal/domain/demos\t2.480s",
	}, "\n")
}

// perfBenchFixtureEscape is a captured-shape -gcflags=-m build: the
// compiler's own verdict grammar over the posts wire-shaping
// (import-path-relative sites, as the real build prints them).
func perfBenchFixtureEscape() string {
	return strings.Join([]string{
		"# guide/internal/presentation",
		"internal/presentation/posts.go:50:6: can inline newPostResponse",
		"internal/presentation/posts.go:78:40: inlining call to newPostResponse",
		"internal/presentation/posts.go:55:20: append escapes to heap",
		"internal/presentation/posts.go:76:15: make([]postResponse, 0, len(page.Items)) escapes to heap",
		"internal/presentation/content.go:71:21: inlining call to json.NewEncoder",
		"internal/presentation/middleware.go:168:39: map[int][]string{...} escapes to heap",
	}, "\n")
}

func newPerfBenchServer(t *testing.T, runBench func(context.Context) demos.PerfBenchOutcome, autostart bool) (http.Handler, *PerfBenchHandler) {
	t.Helper()
	logger := newTestLogger(&strings.Builder{})
	handler := NewPerfBenchHandler(testingModuleDir(t), logger)
	handler.runBench = runBench
	if autostart {
		handler.Start()
	}
	mux := http.NewServeMux()
	mux.Handle("GET "+demos.PerfBenchPath, handler)
	return Chain(
		RequestID,
		RequestLogger(logger),
		Recover(logger),
		EnvelopeMuxErrors(logger),
	)(mux), handler
}

// awaitPerfOutcome polls the handler until its background run lands.
func awaitPerfOutcome(t *testing.T, handler *PerfBenchHandler) demos.PerfBenchOutcome {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for {
		handler.mu.Lock()
		outcome := handler.outcome
		handler.mu.Unlock()
		if outcome != nil || time.Now().After(deadline) {
			if outcome == nil {
				t.Fatal("the background run never landed within its wait bound")
			}
			return *outcome
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func perfDoneOutcome() demos.PerfBenchOutcome {
	finished := time.Now().Add(-time.Minute)
	return demos.PerfBenchOutcome{
		State: demos.PerfBenchStateDone,
		Bench: demos.PerfCommandOutcome{
			State:      demos.PerfBenchStateDone,
			Command:    demos.PerfBenchCommand("go"),
			StartedAt:  finished.Add(-3 * time.Second),
			FinishedAt: finished,
			ExitCode:   0,
			Output:     perfBenchFixtureOutput(),
		},
		Escape: demos.PerfCommandOutcome{
			State:   demos.PerfBenchStateDone,
			Command: demos.PerfEscapeCommand("go"),
			Output:  perfBenchFixtureEscape(),
		},
	}
}

func TestPerfBenchServesParsedSamplesWithRunConfigAndAge(t *testing.T) {
	server, handler := newPerfBenchServer(t, func(context.Context) demos.PerfBenchOutcome {
		return perfDoneOutcome()
	}, true)
	awaitPerfOutcome(t, handler)

	rec := doRequest(server, http.MethodGet, demos.PerfBenchPath, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if rec.Header().Get(core.RequestIDHeader) == "" {
		t.Error("every response must carry x-request-id")
	}

	var payload struct {
		Mode string `json:"mode"`
		Run  struct {
			State     string  `json:"state"`
			Command   string  `json:"command"`
			Benchtime string  `json:"benchtime"`
			Count     int     `json:"count"`
			Goos      string  `json:"goos"`
			CPU       string  `json:"cpu"`
			Pkg       string  `json:"pkg"`
			Failed    int     `json:"failedBenchmarks"`
			AgeSec    float64 `json:"ageSeconds"`
		} `json:"run"`
		Compression []struct {
			Level   int    `json:"level"`
			Size    string `json:"size"`
			Ratio   string `json:"ratio"`
			Samples []struct {
				NsPerOp     float64 `json:"nsPerOp"`
				AllocsPerOp float64 `json:"allocsPerOp"`
				HasMemStats bool    `json:"hasMemStats"`
			} `json:"samples"`
		} `json:"compression"`
		Encoding []struct {
			Approach string `json:"approach"`
			Samples  []struct {
				BytesPerOp float64 `json:"bytesPerOp"`
			} `json:"samples"`
		} `json:"encoding"`
		HotPath struct {
			Benchmark string `json:"benchmark"`
		} `json:"hotPath"`
		Escape struct {
			Command  string `json:"command"`
			State    string `json:"state"`
			Verdicts []struct {
				Site    string `json:"site"`
				Verdict string `json:"verdict"`
			} `json:"verdicts"`
		} `json:"escape"`
		Checklist []string `json:"checklist"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("payload not the demo DTO shape: %v\n%s", err, rec.Body.String())
	}

	if payload.Run.State != "done" || payload.Run.Command == "" {
		t.Errorf("run panel = %+v", payload.Run)
	}
	if payload.Run.Benchtime != "10x" || payload.Run.Count != 2 {
		t.Errorf("run config must ride the panel: %+v", payload.Run)
	}
	if payload.Run.Goos != "linux" || payload.Run.Pkg != "guide/internal/domain/demos" {
		t.Errorf("environment parsed = %+v", payload.Run)
	}
	if payload.Run.AgeSec < 55 {
		t.Errorf("age = %vs, want the run's real wall-clock age (~60s)", payload.Run.AgeSec)
	}
	if len(payload.Compression) != 9 {
		t.Fatalf("compression cells = %d, want 9 (3 levels x 3 sizes)", len(payload.Compression))
	}
	if len(payload.Compression[0].Samples) != 2 {
		t.Errorf("first cell samples = %d, want both count runs (never averaged)", len(payload.Compression[0].Samples))
	}
	if payload.Compression[0].Samples[0].NsPerOp == payload.Compression[0].Samples[1].NsPerOp {
		t.Error("the two samples must carry their real difference")
	}
	if payload.Compression[0].Ratio == "" {
		t.Error("the ratio column is computed from the real document")
	}
	if len(payload.Encoding) != 2 || payload.Encoding[0].Approach == payload.Encoding[1].Approach {
		t.Errorf("encoding rows = %+v", payload.Encoding)
	}
	if payload.HotPath.Benchmark != "BenchmarkPerfInMemoryPostsList" {
		t.Errorf("hot path = %+v", payload.HotPath)
	}
	if payload.Escape.State != "done" || len(payload.Escape.Verdicts) == 0 {
		t.Errorf("escape panel = %+v", payload.Escape)
	}
	if len(payload.Checklist) < 5 {
		t.Errorf("checklist = %d items", len(payload.Checklist))
	}
	if !strings.Contains(payload.Mode, "boot-cached") {
		t.Errorf("mode must name the shipped mode: %q", payload.Mode)
	}
}

func TestPerfBenchReportsRunningUntilTheBootRunLands(t *testing.T) {
	release := make(chan struct{})
	server, handler := newPerfBenchServer(t, func(ctx context.Context) demos.PerfBenchOutcome {
		<-release
		return perfDoneOutcome()
	}, false)

	rec := doRequest(server, http.MethodGet, demos.PerfBenchPath, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 while the boot run is in flight", rec.Code)
	}
	var payload struct {
		Run struct {
			State string `json:"state"`
		} `json:"run"`
		Compression []struct {
			Samples []json.RawMessage `json:"samples"`
		} `json:"compression"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if payload.Run.State != "running" || len(payload.Compression) == 0 || len(payload.Compression[0].Samples) != 0 {
		t.Errorf("in-flight state = %+v, want running with honestly empty samples", payload.Run)
	}

	close(release)
	awaitPerfOutcome(t, handler)
	rec = doRequest(server, http.MethodGet, demos.PerfBenchPath, "")
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode after landing: %v", err)
	}
	if payload.Run.State != "done" || len(payload.Compression[0].Samples) == 0 {
		t.Errorf("landed state = %+v, want the parsed samples", payload.Run)
	}
}

func TestPerfBenchDegradesToTheTypedUnavailable(t *testing.T) {
	server, handler := newPerfBenchServer(t, func(context.Context) demos.PerfBenchOutcome {
		return demos.PerfBenchOutcome{
			State:  demos.PerfBenchStateUnavailable,
			Reason: "the go toolchain is not on PATH beside this process",
		}
	}, true)
	awaitPerfOutcome(t, handler)

	rec := doRequest(server, http.MethodGet, demos.PerfBenchPath, "")
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want the typed 503", rec.Code)
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
	if envelope.Error.Code != "service_unavailable" || !strings.Contains(rec.Body.String(), "go toolchain") {
		t.Errorf("envelope = %+v, want service_unavailable naming the real reason", envelope)
	}
}

func TestPerfBenchRejectsNonGet(t *testing.T) {
	server, _ := newPerfBenchServer(t, func(context.Context) demos.PerfBenchOutcome {
		return perfDoneOutcome()
	}, true)
	rec := doRequest(server, http.MethodPost, demos.PerfBenchPath, "")
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST status = %d, want 405", rec.Code)
	}
}

// TestPerfBenchRealRunParsesRealOutput executes the actual runner
// (real go toolchain, real module tree, real bench output, real
// compiler verdicts) — the end-to-end proof that the served numbers
// come from commands that ran.
func TestPerfBenchRealRunParsesRealOutput(t *testing.T) {
	moduleDir := testingModuleDir(t)
	if moduleDir == "" {
		t.Skip("module tree not beside this binary: the real runner needs the source tree")
	}
	logger := newTestLogger(&strings.Builder{})
	handler := NewPerfBenchHandler(moduleDir, logger)

	outcome := handler.runBench(context.Background())
	if outcome.State != demos.PerfBenchStateDone {
		t.Fatalf("real run state = %q (%q)", outcome.State, outcome.Reason)
	}
	if outcome.Bench.ExitCode != 0 {
		t.Fatalf("bench exit = %d:\n%s", outcome.Bench.ExitCode, outcome.Bench.Output)
	}
	parsed := demos.ParseGoBenchOutput(outcome.Bench.Output)
	if parsed.Goos == "" || parsed.Pkg != "guide/internal/domain/demos" {
		t.Errorf("environment = %+v from real output:\n%s", parsed, outcome.Bench.Output)
	}
	// The curve's nine cells all carry both count samples.
	curveCells := 0
	for _, row := range parsed.Rows {
		if row.Benchmark == "BenchmarkPerfGzipCompress" && row.HasMemStats {
			curveCells++
		}
	}
	if curveCells != 2*len(demos.PerfGzipLevels)*len(demos.PerfPayloadSizes) {
		t.Errorf("curve samples = %d, want both count runs of all nine cells:\n%s",
			curveCells, outcome.Bench.Output)
	}

	// The escape build's verdicts include the posts wire-shaping focus.
	if outcome.Escape.ExitCode != 0 {
		t.Fatalf("escape build exit = %d:\n%s", outcome.Escape.ExitCode, outcome.Escape.Output)
	}
	verdicts := demos.ParseEscapeVerdicts(outcome.Escape.Output)
	if len(verdicts) == 0 {
		t.Fatalf("no verdicts parsed from the real -m build:\n%s", outcome.Escape.Output)
	}
	focused := 0
	for _, verdict := range verdicts {
		if strings.Contains(verdict.Line, "posts.go") {
			focused++
		}
	}
	if focused == 0 {
		t.Errorf("no ./posts.go verdicts in the real build output:\n%s", outcome.Escape.Output)
	}
}

// TestRunBoundedCombinedKillsARunawayProcessGroup proves the combined
// runner's timeout actually kills: a command that spawns a child and
// sleeps far past the bound is torn down — parent and group — well
// before its own lifetime.
func TestRunBoundedCombinedKillsARunawayProcessGroup(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()

	started := time.Now()
	_, _, _ = runBoundedCombined(ctx, t.TempDir(), "sh", "-c", "sleep 30 & wait")
	elapsed := time.Since(started)

	if elapsed > 5*time.Second {
		t.Fatalf("bounded run took %v — the timeout must actually kill the group", elapsed)
	}
	if ctx.Err() == nil {
		t.Error("the context must have expired for the kill path to be the one exercised")
	}
}

// TestPerfBenchHandlerStartIsIdempotent pins the single-flight: a
// double Start (boot wiring plus a racing first request) still runs
// the bench exactly once.
func TestPerfBenchHandlerStartIsIdempotent(t *testing.T) {
	logger := newTestLogger(&strings.Builder{})
	handler := NewPerfBenchHandler("", logger)

	var calls atomic.Int64
	handler.runBench = func(context.Context) demos.PerfBenchOutcome {
		calls.Add(1)
		select {} // park: the state machine must not depend on the run returning
	}
	handler.Start()
	handler.Start()
	time.Sleep(50 * time.Millisecond)

	if got := calls.Load(); got != 1 {
		t.Fatalf("Start ran the bench %d times, want exactly one flight", got)
	}
	if outcome := handler.state(); outcome.State != demos.PerfBenchStateRunning {
		t.Errorf("state = %q, want running while the single flight is parked", outcome.State)
	}
}
