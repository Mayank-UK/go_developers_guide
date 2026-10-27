// Demo API handler for topic 34 testing — the runner. The handler owns
// the ONE real execution: a bounded `go test -json` run over the
// curated set, kicked once at process boot (composition root calls
// Start) and cached for the process lifetime, exactly the single-flight
// shape topic 31's govulncheck scan proved out — the request path never
// waits on a compiler. If the toolchain or the source tree is missing,
// or the run exceeds its timeout, the endpoint answers the typed 503;
// the suite's shape is counted from the tree on every request, and the
// pure builder in internal/domain/demos does all the shaping.
package presentation

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"

	"guide/internal/core"
	"guide/internal/domain/demos"
)

// Compile-time proof the handler satisfies http.Handler.
var _ http.Handler = (*TestingRunnerHandler)(nil)

// TestingRunnerHandler serves GET /api/demo/testing/runner, the testing
// topic's demo payload: the boot-cached run's parsed timeline plus the
// suite's counted shape.
type TestingRunnerHandler struct {
	logger    *slog.Logger
	moduleDir string
	// runTests performs the one run this process makes; a field so
	// tests drive the state machine without executing the toolchain.
	runTests func(ctx context.Context) demos.TestingRunOutcome

	mu      sync.Mutex
	started bool
	outcome *demos.TestingRunOutcome
}

// NewTestingRunnerHandler wires the runner over the module directory
// holding go.mod (empty when the source tree is not beside the process
// — the demo then degrades honestly, like the dependency manifest's
// govulncheck scan).
func NewTestingRunnerHandler(moduleDir string, logger *slog.Logger) *TestingRunnerHandler {
	handler := &TestingRunnerHandler{
		logger:    logger,
		moduleDir: moduleDir,
	}
	handler.runTests = testingDefaultRunner(moduleDir, logger)
	return handler
}

// Start kicks the single background run at boot. Idempotent; the first
// request triggers it too if boot never did.
func (h *TestingRunnerHandler) Start() {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.started {
		return
	}
	h.started = true
	go func() {
		outcome := h.runTests(context.Background())
		h.mu.Lock()
		defer h.mu.Unlock()
		h.outcome = &outcome
	}()
}

// state snapshots the run: the cached outcome when the background run
// finished, an honest "running" placeholder until then.
func (h *TestingRunnerHandler) state() demos.TestingRunOutcome {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.outcome != nil {
		return *h.outcome
	}
	if !h.started {
		h.started = true
		go func() {
			outcome := h.runTests(context.Background())
			h.mu.Lock()
			defer h.mu.Unlock()
			h.outcome = &outcome
		}()
	}
	return demos.TestingRunOutcome{State: demos.TestingRunStateRunning}
}

// ServeHTTP implements http.Handler: serve the cached run's parsed
// timeline plus the freshly counted suite shape. A toolchain hiccup
// degrades to the typed 503 — never a fabricated green timeline.
func (h *TestingRunnerHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	outcome := h.state()
	if outcome.State == demos.TestingRunStateUnavailable {
		err := core.New(core.KindServiceUnavailable,
			"The testing runner could not execute its curated run: "+outcome.Reason)
		core.WriteError(w, r, err, h.logger)
		return
	}

	shape := demos.TestingSuiteShape{
		TableExamples: []demos.TestingTableExampleRow{},
		Provenance: "the suite's shape could not be counted: the source tree is " +
			"not beside this process (the prod image ships only the binary)",
	}
	if h.moduleDir != "" {
		shape = demos.CountSuiteShape(os.DirFS(h.moduleDir))
	}

	writeJSON(w, http.StatusOK, demos.BuildTestingRunnerPayload(demos.TestingRunnerFacts{
		Now:   time.Now(),
		Run:   outcome,
		Shape: shape,
	}))
}

// testingDefaultRunner performs the real run: locate the go binary,
// execute the curated command in the module directory under a bounded
// context whose expiry kills the WHOLE process group (go test's child
// test binaries included), and fold the outcome.
func testingDefaultRunner(moduleDir string, logger *slog.Logger) func(context.Context) demos.TestingRunOutcome {
	return func(ctx context.Context) demos.TestingRunOutcome {
		if moduleDir == "" {
			return demos.TestingRunOutcome{
				State:  demos.TestingRunStateUnavailable,
				Reason: "the module's source tree is not beside this process",
			}
		}
		goBinary, err := exec.LookPath("go")
		if err != nil {
			return demos.TestingRunOutcome{
				State:  demos.TestingRunStateUnavailable,
				Reason: "the go toolchain is not on PATH beside this process",
			}
		}

		argv := []string{"test", "-json", "-count=1", "-run", demos.TestingRunFocus}
		argv = append(argv, demos.TestingRunPackages...)
		started := time.Now()
		runCtx, cancel := context.WithTimeout(ctx, demos.TestingRunTimeout)
		defer cancel()

		stdout, exitCode, runErr := runBounded(runCtx, moduleDir, goBinary, argv...)
		outcome := demos.TestingRunOutcome{
			State:      demos.TestingRunStateDone,
			Command:    demos.TestingRunCommand("go"),
			StartedAt:  started,
			FinishedAt: time.Now(),
			ExitCode:   exitCode,
			Stdout:     stdout,
		}

		switch {
		case runCtx.Err() == context.DeadlineExceeded:
			outcome.State = demos.TestingRunStateUnavailable
			outcome.Reason = "the curated run exceeded its bounded timeout and the whole process group was killed"
		case runErr != nil && strings.TrimSpace(stdout) == "":
			// The run never produced a single event: a start failure, not
			// a test result. (A failing test produces events AND exits
			// non-zero — that stays a done run.)
			logger.Error("testing runner could not execute", "error", runErr.Error())
			outcome.State = demos.TestingRunStateUnavailable
			outcome.Reason = "go test could not be executed: " + runErr.Error()
		}
		return outcome
	}
}

// runBounded executes one command with its own process group and a
// context kill that takes the group down — `go test` spawns the test
// binaries as children, and killing only the parent would strand them.
func runBounded(ctx context.Context, dir, name string, args ...string) (string, int, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		if cmd.Process != nil {
			// Negative pid addresses the process GROUP: the go tool and
			// every test binary it spawned die together.
			_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		}
		return cmd.Process.Kill()
	}
	raw, err := cmd.Output()
	exitCode := 0
	if exitErr, ok := err.(*exec.ExitError); ok {
		// A non-zero exit is a RESULT (tests failed), not an execution
		// error to hide.
		exitCode = exitErr.ExitCode()
		err = nil
	}
	return string(raw), exitCode, err
}
