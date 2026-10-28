// Demo API handler for topic 38 performance — the bench. The handler
// owns the ONE real execution, exactly the single-flight shape the
// testing runner proved out: two bounded commands kicked once at
// process boot (composition root calls Start) and cached for the
// process lifetime, so no request ever waits on a compiler — a real
// `go test -bench` run over this repo's own benchmark functions and a
// real `go build -gcflags=-m` over the presentation package whose
// output the escape panel serves verbatim. A missing toolchain or
// source tree, or a run that exceeds its timeout, degrades to the
// typed 503; the pure builder in internal/domain/demos does all the
// shaping.
package presentation

import (
	"context"
	"log/slog"
	"net/http"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"guide/internal/core"
	"guide/internal/domain/demos"
)

// Compile-time proof the handler satisfies http.Handler.
var _ http.Handler = (*PerfBenchHandler)(nil)

// PerfBenchHandler serves GET /api/demo/perf/bench, the performance
// topic's demo payload: the boot-cached benchmark samples with their
// run configuration, plus the compiler's escape-analysis verdicts.
type PerfBenchHandler struct {
	logger    *slog.Logger
	moduleDir string
	// runBench performs the one run this process makes; a field so
	// tests drive the state machine without executing the toolchain.
	runBench func(ctx context.Context) demos.PerfBenchOutcome

	mu      sync.Mutex
	started bool
	outcome *demos.PerfBenchOutcome
}

// NewPerfBenchHandler wires the bench over the module directory
// holding go.mod (empty when the source tree is not beside the process
// — the demo then degrades honestly, like the testing runner and the
// dependency manifest's govulncheck scan).
func NewPerfBenchHandler(moduleDir string, logger *slog.Logger) *PerfBenchHandler {
	handler := &PerfBenchHandler{
		logger:    logger,
		moduleDir: moduleDir,
	}
	handler.runBench = perfDefaultRunner(moduleDir, logger)
	return handler
}

// Start kicks the single background run at boot. Idempotent; the first
// request triggers it too if boot never did.
func (h *PerfBenchHandler) Start() {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.started {
		return
	}
	h.started = true
	go func() {
		outcome := h.runBench(context.Background())
		h.mu.Lock()
		defer h.mu.Unlock()
		h.outcome = &outcome
	}()
}

// state snapshots the run: the cached outcome when the background run
// finished, an honest "running" placeholder until then.
func (h *PerfBenchHandler) state() demos.PerfBenchOutcome {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.outcome != nil {
		return *h.outcome
	}
	if !h.started {
		h.started = true
		go func() {
			outcome := h.runBench(context.Background())
			h.mu.Lock()
			defer h.mu.Unlock()
			h.outcome = &outcome
		}()
	}
	return demos.PerfBenchOutcome{State: demos.PerfBenchStateRunning}
}

// ServeHTTP implements http.Handler: serve the cached run's parsed
// samples with their configuration and age. A toolchain hiccup
// degrades to the typed 503 — never a fabricated curve.
func (h *PerfBenchHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	outcome := h.state()
	if outcome.State == demos.PerfBenchStateUnavailable {
		err := core.New(core.KindServiceUnavailable,
			"The performance bench could not execute its curated run: "+outcome.Reason)
		core.WriteError(w, r, err, h.logger)
		return
	}
	writeJSON(w, http.StatusOK, demos.BuildPerfBenchPayload(demos.PerfBenchFacts{
		Now: time.Now(),
		Run: outcome,
	}))
}

// perfDefaultRunner performs the real run: locate the go binary, then
// execute the two bounded commands in the module directory — the
// curated benchmark run and the escape-analysis build — each under its
// own context whose expiry kills the WHOLE process group, and fold the
// outcomes. The overall state follows the bench run; the escape build
// failing degrades its own panel, not the measured numbers.
func perfDefaultRunner(moduleDir string, logger *slog.Logger) func(context.Context) demos.PerfBenchOutcome {
	return func(ctx context.Context) demos.PerfBenchOutcome {
		if moduleDir == "" {
			return demos.PerfBenchOutcome{
				State:  demos.PerfBenchStateUnavailable,
				Reason: "the module's source tree is not beside this process",
			}
		}
		goBinary, err := exec.LookPath("go")
		if err != nil {
			return demos.PerfBenchOutcome{
				State:  demos.PerfBenchStateUnavailable,
				Reason: "the go toolchain is not on PATH beside this process",
			}
		}

		outcome := demos.PerfBenchOutcome{}
		outcome.Bench = perfRunBoundedCommand(ctx, moduleDir, goBinary, demos.PerfBenchTimeout,
			demos.PerfBenchCommand("go"),
			"test",
			"-bench=^BenchmarkPerf",
			"-benchtime="+demos.PerfBenchBenchtime,
			"-count="+strconv.Itoa(demos.PerfBenchCount),
			"-benchmem",
			"-run=^$",
			"./internal/domain/demos/")
		outcome.Escape = perfRunBoundedCommand(ctx, moduleDir, goBinary, demos.PerfEscapeTimeout,
			demos.PerfEscapeCommand("go"),
			"build",
			"-gcflags=-m",
			"./internal/presentation/")

		if outcome.Bench.State == demos.PerfBenchStateUnavailable {
			outcome.State = demos.PerfBenchStateUnavailable
			outcome.Reason = outcome.Bench.Reason
			logger.Error("perf bench run degraded", "reason", outcome.Reason)
		} else {
			outcome.State = demos.PerfBenchStateDone
		}
		return outcome
	}
}

// perfRunBoundedCommand executes one command with its own process
// group and a timeout kill that takes the group down (`go test`
// spawns test binaries; `go build` spawns compiler workers), folding
// the combined output — the go tool prints bench rows to stdout and
// compiler diagnostics to stderr, and both are data here.
func perfRunBoundedCommand(ctx context.Context, dir, goBinary string, timeout time.Duration,
	command string, args ...string,
) demos.PerfCommandOutcome {
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	started := time.Now()
	combined, exitCode, runErr := runBoundedCombined(runCtx, dir, goBinary, args...)
	outcome := demos.PerfCommandOutcome{
		State:      demos.PerfBenchStateDone,
		Command:    command,
		StartedAt:  started,
		FinishedAt: time.Now(),
		ExitCode:   exitCode,
		Output:     combined,
	}
	switch {
	case runCtx.Err() == context.DeadlineExceeded:
		outcome.State = demos.PerfBenchStateUnavailable
		outcome.Reason = "the command exceeded its bounded timeout and the whole process group was killed"
	case runErr != nil && strings.TrimSpace(combined) == "":
		outcome.State = demos.PerfBenchStateUnavailable
		outcome.Reason = "the command could not be executed: " + runErr.Error()
	}
	return outcome
}

// runBoundedCombined executes one command with its own process group,
// returning the combined stdout+stderr — runBounded's sibling for
// tools whose interesting output spans both streams.
func runBoundedCombined(ctx context.Context, dir, name string, args ...string) (string, int, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		if cmd.Process != nil {
			// Negative pid addresses the process GROUP: the go tool and
			// every child it spawned die together.
			_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		}
		return cmd.Process.Kill()
	}
	raw, err := cmd.CombinedOutput()
	exitCode := 0
	if exitErr, ok := err.(*exec.ExitError); ok {
		// A non-zero exit is a RESULT (a failed compile, a failing
		// benchmark), not an execution error to hide.
		exitCode = exitErr.ExitCode()
		err = nil
	}
	return string(raw), exitCode, err
}
