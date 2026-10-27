// Demo API handler for topic 31 dependency-management — the family's
// thin-handler invariant: parse nothing, trust the composition root's
// one parameter (the module directory, when the source tree sits
// beside the process), observe, and delegate shaping to the pure
// builder in internal/domain/demos. The one piece of real work the
// handler owns is the govulncheck attempt: a single background scan
// per process (started by the first request, bounded by a timeout,
// cached for the lifetime), so the endpoint stays fast while the
// panel still serves the real tool's real output whenever the tool
// and the source tree are both available — the dev container — and
// degrades to the builder's clearly-labeled example when they are not
// (the distroless prod image ships neither sources nor toolchain).
package presentation

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"guide/internal/domain/demos"
)

// Compile-time proof the handler satisfies http.Handler.
var _ http.Handler = (*DepsManifestHandler)(nil)

// DepsManifestHandler serves GET /api/demo/deps/manifest, the
// dependency-management topic's demo payload: the live go.mod panel,
// the serving binary's build info, the tools/deps.go anchor story, the
// Docker cache volumes, and the govulncheck observation.
type DepsManifestHandler struct {
	logger    *slog.Logger
	moduleDir string
	// runVulncheck performs the one scan this process makes; a field so
	// tests run the state machine without executing the real tool.
	runVulncheck func(ctx context.Context) demos.DepsVulncheckObservation

	mu      sync.Mutex
	started bool
	result  *demos.DepsVulncheckObservation
}

// NewDepsManifestHandler wires the manifest demo over the module
// directory holding go.mod (empty when the source tree is not beside
// the process — the demo then degrades honestly instead of guessing).
func NewDepsManifestHandler(moduleDir string, logger *slog.Logger) *DepsManifestHandler {
	handler := &DepsManifestHandler{
		logger:    logger,
		moduleDir: moduleDir,
	}
	handler.runVulncheck = depsDefaultVulncheckRunner(moduleDir, logger)
	return handler
}

// ServeHTTP implements http.Handler: snapshot the one govulncheck
// state (starting the scan if this is the first request), read the
// manifest sources, and shape the payload from those facts.
func (h *DepsManifestHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	observation := h.vulncheckState()

	facts := demos.DepsManifestFacts{
		GoModContent: depsReadFile(h.moduleDir, "go.mod"),
		ToolsContent: depsReadFile(h.moduleDir, "tools", "deps.go"),
		BuildInfo:    depsReadBuildInfo(),
		Vulncheck:    observation,
	}

	writeJSON(w, http.StatusOK, demos.BuildDepsManifestPayload(facts))
}

// vulncheckState returns the current observation and starts the single
// background scan on the first call: every later request re-reads the
// cached result, so the endpoint's cost stays constant however many
// times the panel is opened.
func (h *DepsManifestHandler) vulncheckState() demos.DepsVulncheckObservation {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.result != nil {
		return *h.result
	}
	if !h.started {
		h.started = true
		go func() {
			// The scan outlives the request that started it and is
			// bounded by the runner's own timeout — a wedged tool must
			// never pin this goroutine (or the first payload) forever.
			observation := h.runVulncheck(context.Background())
			h.mu.Lock()
			defer h.mu.Unlock()
			h.result = &observation
		}()
	}
	return demos.DepsVulncheckObservation{State: demos.DepsVulncheckStateRunning}
}

// depsReadBuildInfo reads the serving process's own build info — the
// birth certificate the go tool stamps into every binary, including
// the go-run dev server, the built prod binary, and this test binary.
func depsReadBuildInfo() *debug.BuildInfo {
	buildInfo, _ := debug.ReadBuildInfo()
	return buildInfo
}

// depsReadFile reads a file under the module directory, returning ""
// when there is no module directory or no such file — the payload's
// degradation is the empty content, handled honestly downstream.
func depsReadFile(moduleDir string, segments ...string) string {
	if moduleDir == "" {
		return ""
	}
	parts := append([]string{moduleDir}, segments...)
	raw, err := os.ReadFile(filepath.Join(parts...))
	if err != nil {
		return ""
	}
	return string(raw)
}

// depsDefaultVulncheckRunner performs the real scan: locate the tool
// (PATH plus the conventional /go/bin and /usr/local/bin slots), run
// `govulncheck ./...` in the module directory under a bounded context,
// and fold the outcome into an observation — the tool's closing
// summary lines carried verbatim, never paraphrased.
func depsDefaultVulncheckRunner(moduleDir string, logger *slog.Logger) func(context.Context) demos.DepsVulncheckObservation {
	return func(ctx context.Context) demos.DepsVulncheckObservation {
		if moduleDir == "" {
			return demos.DepsVulncheckObservation{
				State:  demos.DepsVulncheckStateUnavailable,
				Reason: "the module's source tree is not beside this process (the prod image ships only the binary)",
			}
		}
		binary, err := depsLookPath("govulncheck")
		if err != nil {
			return demos.DepsVulncheckObservation{
				State:  demos.DepsVulncheckStateUnavailable,
				Reason: "govulncheck is not installed here (go install golang.org/x/vuln/cmd/govulncheck@latest)",
			}
		}

		started := time.Now()
		scanCtx, cancel := context.WithTimeout(ctx, demos.DepsGovulncheckTimeout)
		defer cancel()
		cmd := exec.CommandContext(scanCtx, binary, "./...")
		cmd.Dir = moduleDir
		raw, err := cmd.Output()
		elapsed := time.Since(started)

		if scanCtx.Err() == context.DeadlineExceeded {
			return demos.DepsVulncheckObservation{
				State:  demos.DepsVulncheckStateUnavailable,
				Reason: "the scan exceeded its bounded timeout and was cancelled",
			}
		}
		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				// A non-zero exit is a RESULT (govulncheck exits 3 when
				// reachable findings exist), not an error to hide.
				return demos.DepsVulncheckObservation{
					State:    demos.DepsVulncheckStateDone,
					ExitCode: exitErr.ExitCode(),
					Elapsed:  elapsed,
					Summary:  depsVulncheckSummary(string(raw), string(exitErr.Stderr)),
				}
			}
			logger.Error("govulncheck could not run", "error", err.Error())
			return demos.DepsVulncheckObservation{
				State:  demos.DepsVulncheckStateUnavailable,
				Reason: "the tool could not be executed: " + err.Error(),
			}
		}
		return demos.DepsVulncheckObservation{
			State:    demos.DepsVulncheckStateDone,
			ExitCode: 0,
			Elapsed:  elapsed,
			Summary:  depsVulncheckSummary(string(raw), ""),
		}
	}
}

// depsVulncheckSummary extracts the tool's own closing block — the
// summary section it prints after its last "===" header — so the
// payload carries the verdict in the tool's words without the detail
// tables. Output without a header degrades to the last non-empty
// lines, and a stdout-less failure falls back to stderr.
func depsVulncheckSummary(stdout, stderr string) string {
	stdout = strings.TrimRight(stdout, "\n")
	if stdout == "" {
		if stderr != "" {
			return strings.TrimSpace(stderr)
		}
		return ""
	}
	lines := strings.Split(stdout, "\n")
	lastHeader := -1
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "===") {
			lastHeader = i
		}
	}
	if lastHeader >= 0 && lastHeader+1 < len(lines) {
		if summary := strings.TrimSpace(strings.Join(lines[lastHeader+1:], "\n")); summary != "" {
			return summary
		}
	}
	var kept []string
	for i := len(lines) - 1; i >= 0 && len(kept) < 8; i-- {
		line := strings.TrimRight(lines[i], " \t")
		if line == "" {
			if len(kept) > 0 {
				break
			}
			continue
		}
		kept = append([]string{line}, kept...)
	}
	return strings.Join(kept, "\n")
}

// depsLookPath resolves the tool binary: PATH first, then the
// conventional install slots the dev toolchain image uses.
func depsLookPath(binary string) (string, error) {
	if found, err := exec.LookPath(binary); err == nil {
		return found, nil
	}
	for _, slot := range []string{"/go/bin/govulncheck", "/usr/local/bin/govulncheck"} {
		if info, err := os.Stat(slot); err == nil && !info.IsDir() {
			return slot, nil
		}
	}
	return "", os.ErrNotExist
}
