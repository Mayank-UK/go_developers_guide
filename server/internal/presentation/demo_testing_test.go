package presentation

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"guide/internal/core"
	"guide/internal/domain/demos"
)

// The testing runner endpoint through the production middleware chain.
// The handler tests drive the state machine with an injected runner
// (canned event streams captured from the real tool's shape); one test
// executes the REAL runner against the module tree beside the test
// binary, and one proves the bounded runner actually kills a runaway
// process group.

// testingFixtureEvents is a captured-shape stream: two packages, one
// table test with subtests, one skipped test, one package verdict each.
func testingFixtureEvents() string {
	return strings.Join([]string{
		`{"Action":"start","Package":"guide/internal/domain/posts"}`,
		`{"Action":"run","Package":"guide/internal/domain/posts","Test":"TestDraftValidateCollectsEveryIssue"}`,
		`{"Action":"pass","Package":"guide/internal/domain/posts","Test":"TestDraftValidateCollectsEveryIssue","Elapsed":0.004}`,
		`{"Action":"pass","Package":"guide/internal/domain/posts","Elapsed":0.008}`,
		`{"Action":"start","Package":"guide/internal/presentation"}`,
		`{"Action":"run","Package":"guide/internal/presentation","Test":"TestPostsCreateRefusesBodiesAtTheBoundary"}`,
		`{"Action":"run","Package":"guide/internal/presentation","Test":"TestPostsCreateRefusesBodiesAtTheBoundary/null_body"}`,
		`{"Action":"output","Package":"guide/internal/presentation","Test":"TestPostsCreateRefusesBodiesAtTheBoundary/null_body","Output":"--- PASS: TestPostsCreateRefusesBodiesAtTheBoundary/null_body (0.00s)\n"}`,
		`{"Action":"pass","Package":"guide/internal/presentation","Test":"TestPostsCreateRefusesBodiesAtTheBoundary/null_body","Elapsed":0}`,
		`{"Action":"pass","Package":"guide/internal/presentation","Test":"TestPostsCreateRefusesBodiesAtTheBoundary","Elapsed":0.001}`,
		`{"Action":"skip","Package":"guide/internal/data/db","Test":"TestTestcontainersSmoke","Elapsed":0}`,
		`{"Action":"pass","Package":"guide/internal/data/db","Elapsed":0.005}`,
		`{"Action":"pass","Package":"guide/internal/presentation","Elapsed":0.27}`,
	}, "\n")
}

func newTestingRunnerServer(t *testing.T, runTests func(context.Context) demos.TestingRunOutcome, autostart bool) (http.Handler, *TestingRunnerHandler) {
	t.Helper()
	logger := newTestLogger(&strings.Builder{})
	handler := NewTestingRunnerHandler(testingModuleDir(t), logger)
	handler.runTests = runTests
	if autostart {
		handler.Start()
	}
	mux := http.NewServeMux()
	mux.Handle("GET "+demos.TestingRunnerPath, handler)
	return Chain(
		RequestID,
		RequestLogger(logger),
		Recover(logger),
		EnvelopeMuxErrors(logger),
	)(mux), handler
}

// testingModuleDir locates the module root beside the test binary
// (internal/presentation → server/); empty when the tree is absent.
func testingModuleDir(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		return ""
	}
	for dir := wd; ; dir = filepath.Dir(dir) {
		if _, statErr := os.Stat(filepath.Join(dir, "go.mod")); statErr == nil {
			return dir
		}
		if parent := filepath.Dir(dir); parent == dir {
			return ""
		}
	}
}

// awaitRunOutcome polls the handler until its background run lands.
func awaitRunOutcome(t *testing.T, handler *TestingRunnerHandler) demos.TestingRunOutcome {
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

func TestTestingRunnerServesTheParsedTimelineAndCountedShape(t *testing.T) {
	finished := time.Now().Add(-time.Minute)
	server, handler := newTestingRunnerServer(t, func(context.Context) demos.TestingRunOutcome {
		return demos.TestingRunOutcome{
			State:      demos.TestingRunStateDone,
			Command:    demos.TestingRunCommand("go"),
			StartedAt:  finished.Add(-2 * time.Second),
			FinishedAt: finished,
			ExitCode:   0,
			Stdout:     testingFixtureEvents(),
		}
	}, true)
	awaitRunOutcome(t, handler)

	rec := doRequest(server, http.MethodGet, demos.TestingRunnerPath, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if rec.Header().Get(core.RequestIDHeader) == "" {
		t.Error("every response must carry x-request-id")
	}

	var payload struct {
		Mode string `json:"mode"`
		Run  struct {
			State        string  `json:"state"`
			Command      string  `json:"command"`
			ExitCode     int     `json:"exitCode"`
			TotalPassed  int     `json:"totalPassed"`
			TotalSkipped int     `json:"totalSkipped"`
			AgeSeconds   float64 `json:"ageSeconds"`
		} `json:"run"`
		Packages []struct {
			Package    string  `json:"package"`
			Outcome    string  `json:"outcome"`
			Passed     int     `json:"passed"`
			Skipped    int     `json:"skipped"`
			DurationMs float64 `json:"durationMs"`
		} `json:"packages"`
		Tests []struct {
			Test   string `json:"test"`
			Status string `json:"status"`
			Depth  int    `json:"depth"`
		} `json:"tests"`
		Shape struct {
			TestFiles     int `json:"testFiles"`
			TestPackages  int `json:"testPackages"`
			TestFunctions int `json:"testFunctions"`
			TableExamples []struct {
				Symbol  string `json:"symbol"`
				Present bool   `json:"present"`
			} `json:"tableExamples"`
		} `json:"shape"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("payload not the demo DTO shape: %v\n%s", err, rec.Body.String())
	}

	if payload.Run.State != "done" || payload.Run.ExitCode != 0 {
		t.Errorf("run = %+v", payload.Run)
	}
	if payload.Run.TotalPassed != 2 || payload.Run.TotalSkipped != 1 {
		t.Errorf("totals = passed %d skipped %d, want 2 and 1 (parsed, not promised)", payload.Run.TotalPassed, payload.Run.TotalSkipped)
	}
	if payload.Run.AgeSeconds < 55 {
		t.Errorf("age = %vs, want the run's real wall-clock age (~60s)", payload.Run.AgeSeconds)
	}
	if len(payload.Packages) != 3 {
		t.Fatalf("packages = %d, want 3", len(payload.Packages))
	}
	if payload.Packages[2].Outcome != "pass" || payload.Packages[2].Skipped != 1 {
		t.Errorf("db row = %+v, want pass with the gated smoke test counted skipped", payload.Packages[2])
	}
	subtests := 0
	for _, test := range payload.Tests {
		if test.Depth == 1 {
			subtests++
		}
	}
	if subtests != 1 {
		t.Errorf("subtests = %d, want 1 on the timeline", subtests)
	}
	if payload.Shape.TestPackages < 20 || payload.Shape.TestFiles < 60 {
		t.Errorf("shape counted %d packages / %d files — the real tree walks from the module root", payload.Shape.TestPackages, payload.Shape.TestFiles)
	}
	for _, example := range payload.Shape.TableExamples {
		if !example.Present {
			t.Errorf("table example %s must verify against the real tree", example.Symbol)
		}
	}
	if !strings.Contains(payload.Mode, "boot-cached") {
		t.Errorf("mode must name the shipped mode: %q", payload.Mode)
	}
}

func TestTestingRunnerReportsRunningUntilTheBootRunLands(t *testing.T) {
	release := make(chan struct{})
	server, handler := newTestingRunnerServer(t, func(ctx context.Context) demos.TestingRunOutcome {
		<-release
		return demos.TestingRunOutcome{State: demos.TestingRunStateDone, Stdout: testingFixtureEvents()}
	}, false)

	rec := doRequest(server, http.MethodGet, demos.TestingRunnerPath, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 while the boot run is in flight", rec.Code)
	}
	var payload struct {
		Run struct {
			State string `json:"state"`
		} `json:"run"`
		Tests []json.RawMessage `json:"tests"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if payload.Run.State != "running" || len(payload.Tests) != 0 {
		t.Errorf("in-flight state = %+v with %d tests, want running and an honest empty timeline", payload.Run, len(payload.Tests))
	}

	// Land the run, then the very next request serves the timeline.
	close(release)
	awaitRunOutcome(t, handler)
	rec = doRequest(server, http.MethodGet, demos.TestingRunnerPath, "")
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode after landing: %v", err)
	}
	if payload.Run.State != "done" || len(payload.Tests) == 0 {
		t.Errorf("landed state = %+v with %d tests, want the parsed timeline", payload.Run, len(payload.Tests))
	}
}

func TestTestingRunnerDegradesToTheTypedUnavailable(t *testing.T) {
	server, handler := newTestingRunnerServer(t, func(context.Context) demos.TestingRunOutcome {
		return demos.TestingRunOutcome{
			State:  demos.TestingRunStateUnavailable,
			Reason: "the go toolchain is not on PATH beside this process",
		}
	}, true)
	awaitRunOutcome(t, handler)

	rec := doRequest(server, http.MethodGet, demos.TestingRunnerPath, "")
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
		t.Errorf("envelope = %+v, want service_unavailable naming the real reason", envelope.Error)
	}
}

func TestTestingRunnerRejectsNonGet(t *testing.T) {
	server, _ := newTestingRunnerServer(t, func(context.Context) demos.TestingRunOutcome {
		return demos.TestingRunOutcome{State: demos.TestingRunStateDone, Stdout: testingFixtureEvents()}
	}, true)
	rec := doRequest(server, http.MethodPost, demos.TestingRunnerPath, "")
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST status = %d, want 405", rec.Code)
	}
}

// TestTestingRunnerRealRunParsesRealEvents executes the actual runner
// (real go toolchain, real module tree, real events) — the end-to-end
// proof that the served timeline is parsed from a run that happened.
func TestTestingRunnerRealRunParsesRealEvents(t *testing.T) {
	moduleDir := testingModuleDir(t)
	if moduleDir == "" {
		t.Skip("module tree not beside this binary: the real runner needs the source tree")
	}
	logger := newTestLogger(&strings.Builder{})
	handler := NewTestingRunnerHandler(moduleDir, logger)

	outcome := handler.runTests(context.Background())
	if outcome.State != demos.TestingRunStateDone {
		t.Fatalf("real run state = %q (%q): %v", outcome.State, outcome.Reason, outcome.State)
	}
	events := demos.ParseGoTestEvents(outcome.Stdout)

	wantPackages := map[string]bool{
		"guide/internal/domain/posts": false,
		"guide/internal/presentation": false,
		"guide/internal/data/db":      false,
	}
	for _, pkg := range events.Packages {
		if _, known := wantPackages[pkg.Package]; !known {
			t.Errorf("unexpected package %q on the timeline", pkg.Package)
			continue
		}
		wantPackages[pkg.Package] = true
		if pkg.Outcome != "pass" {
			t.Errorf("package %s outcome = %q — the curated set is green in a healthy tree", pkg.Package, pkg.Outcome)
		}
	}
	for pkg, seen := range wantPackages {
		if !seen {
			t.Errorf("package %s missing from the real run's timeline", pkg)
		}
	}

	// The three named presentations/posts/domain tests must appear as
	// rows; the table test carries its subtests.
	rows := map[string]int{}
	for _, test := range events.Tests {
		rows[test.Test]++
	}
	for _, symbol := range []string{
		"TestDraftValidateCollectsEveryIssue",
		"TestNormalizeListQueryCollectsEveryViolation",
		"TestPostsListServesTheSeedPage",
		"TestPostsCreateRefusesBodiesAtTheBoundary",
		"TestMigrationsEmbedTheExpectedFileSet",
	} {
		if rows[symbol] == 0 {
			t.Errorf("curated test %s absent from the real timeline", symbol)
		}
	}
	if rows["TestPostsCreateRefusesBodiesAtTheBoundary/empty_required_fields"] == 0 {
		t.Error("the boundary table's subtests must ride the timeline (slash-named rows)")
	}
}

// TestRunBoundedKillsARunawayProcessGroup proves the timeout actually
// kills: a command that spawns a child and sleeps far past the bound is
// torn down — parent and group — well before its own lifetime.
func TestRunBoundedKillsARunawayProcessGroup(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()

	started := time.Now()
	_, _, _ = runBounded(ctx, t.TempDir(), "sh", "-c", "sleep 30 & wait")
	elapsed := time.Since(started)

	if elapsed > 5*time.Second {
		t.Fatalf("bounded run took %v — the timeout must actually kill the group", elapsed)
	}
	if ctx.Err() == nil {
		t.Error("the context must have expired for the kill path to be the one exercised")
	}
}

// TestTestingRunnerHandlerStartIsIdempotent pins the single-flight: a
// double Start (boot wiring plus a racing first request) still runs the
// suite exactly once.
func TestTestingRunnerHandlerStartIsIdempotent(t *testing.T) {
	logger := newTestLogger(&strings.Builder{})
	handler := NewTestingRunnerHandler("", logger)

	var calls atomic.Int64
	handler.runTests = func(context.Context) demos.TestingRunOutcome {
		calls.Add(1)
		select {} // park: the state machine must not depend on the run returning
	}
	handler.Start()
	handler.Start()
	time.Sleep(50 * time.Millisecond)

	if got := calls.Load(); got != 1 {
		t.Fatalf("Start ran the suite %d times, want exactly one flight", got)
	}
	if outcome := handler.state(); outcome.State != demos.TestingRunStateRunning {
		t.Errorf("state = %q, want running while the single flight is parked", outcome.State)
	}
}
