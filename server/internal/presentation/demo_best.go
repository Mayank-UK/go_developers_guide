// Demo API handler for topic 41 best-practices — the self-audit
// checklist. The handler owns the ONE real audit: every probe, tool
// execution, and file read happens in a single background pass at
// process boot (the composition root calls Start; the first request
// triggers it too if boot never did), bounded overall so a wedged tool
// cannot stall the panel forever, and cached for the process lifetime
// — the single-flight shape the testing runner and perf bench proved
// out. The probes ride a PRIVATE mux mounting the REAL health,
// scenario, and posts handlers behind the REAL production middleware
// chain (the errors-explorer technique), observed in-process — no
// socket, no staged responses. The pure builder in internal/domain/
// demos does all the judging and shaping. Nothing here turns a broken
// check into a 500: a check that cannot run becomes an honest fail or
// manual row, and the endpoint always serves the panel.
package presentation

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"guide/internal/core"
	"guide/internal/domain/demos"
	"guide/internal/domain/posts"
)

// Compile-time proof the handler satisfies http.Handler.
var _ http.Handler = (*BestChecklistHandler)(nil)

// The audit's bounds: gofmt is fast, the ratchet suite compiles one
// package, and the -race build is the slow one — each check owns its
// own deadline, and the whole pass owns an umbrella over them all, so
// a wedged toolchain degrades its row to an honest timeout reason
// instead of pinning boot work forever.
const (
	bestAuditTimeout   = 4 * time.Minute
	bestGofmtTimeout   = 30 * time.Second
	bestRatchetTimeout = 90 * time.Second
	bestRaceTimeout    = 3 * time.Minute
)

// The real test functions the exec rows run — named here so a rename
// in the content or demos package changes the command (and its row)
// visibly instead of silently passing nothing.
const (
	bestRatchetRunFocus = "TestProductionRegistryRatchetIsZero|TestMissingSourceFileIsAViolation"
	bestRaceRunFocus    = "TestRaceDetectorMutexRunIsExact|TestRaceDetectorChannelRunIsExact"
	bestAuditRequestID  = "best-checklist-probe"
)

// BestChecklistHandler serves GET /api/demo/best/checklist, the
// best-practices topic's self-audit payload: the boot-computed verdict
// rows, the not-dos, and the summary — served from the process-
// lifetime snapshot.
type BestChecklistHandler struct {
	probe     http.Handler
	moduleDir string
	repoDir   string
	cfg       core.Config
	// runAudit performs the one audit this process makes; a field so
	// tests drive the cache without executing the toolchain.
	runAudit func(ctx context.Context) demos.BestChecklistFacts

	mu      sync.Mutex
	started bool
	panel   *demos.BestChecklistPanel
}

// NewBestChecklistHandler wires the self-audit over the app's real
// pieces: the typed config, the shared scenario store, and the posts
// use cases the probes exercise — mounted at construction on a private
// mux behind the production middleware chain, so the audit observes
// the same handlers the live server serves. moduleDir is the module
// directory when the source tree sits beside the process (empty in the
// distroless prod image — exec and file rows then degrade honestly).
func NewBestChecklistHandler(
	cfg core.Config,
	store *core.ScenarioStore,
	postsGet *posts.GetPostUseCase,
	postsCreate *posts.CreatePostUseCase,
	moduleDir string,
	logger *slog.Logger,
) *BestChecklistHandler {
	probeMux := http.NewServeMux()
	probeMux.Handle("GET /api/health", NewHealthHandler(cfg.Env))
	probeMux.Handle("GET /api/demo/scenario", NewScenarioGetHandler(store))
	probeMux.Handle("PATCH /api/demo/scenario", NewScenarioPatchHandler(store, logger))
	probeMux.Handle("GET /api/posts/{id}", NewPostDetailHandler(postsGet, logger))
	probeMux.Handle("POST /api/posts", NewPostsCreateHandler(postsCreate, logger))

	handler := &BestChecklistHandler{
		probe:     Chain(RequestID, RequestLogger(logger), Recover(logger), EnvelopeMuxErrors(logger))(probeMux),
		moduleDir: moduleDir,
		repoDir:   bestResolveRepoDir(moduleDir),
		cfg:       cfg,
	}
	handler.runAudit = handler.defaultAudit
	return handler
}

// bestResolveRepoDir locates the repo root above the module directory
// — present in the dev container (the marker directory proves it),
// empty in the prod image, where the one repo-root row degrades to an
// honest manual verdict.
func bestResolveRepoDir(moduleDir string) string {
	if moduleDir == "" {
		return ""
	}
	candidate := filepath.Dir(moduleDir)
	if _, err := os.Stat(filepath.Join(candidate, ".docker")); err == nil {
		return candidate
	}
	return ""
}

// Start kicks the single background audit at boot. Idempotent; the
// first request triggers it too if boot never did.
func (h *BestChecklistHandler) Start() {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.started {
		return
	}
	h.started = true
	go h.compute()
}

// compute runs the audit and caches the shaped panel. Runs on the
// handler's own goroutines (boot or first request), never on a request
// path.
func (h *BestChecklistHandler) compute() {
	ctx, cancel := context.WithTimeout(context.Background(), bestAuditTimeout)
	defer cancel()
	facts := h.runAudit(ctx)
	panel := demos.BuildBestChecklistPanel(facts)
	h.mu.Lock()
	defer h.mu.Unlock()
	h.panel = &panel
}

// snapshot returns the cached panel, or nil while the audit runs.
func (h *BestChecklistHandler) snapshot() *demos.BestChecklistPanel {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.panel != nil {
		return h.panel
	}
	if !h.started {
		h.started = true
		go h.compute()
	}
	return nil
}

// ServeHTTP implements http.Handler: serve the boot snapshot; until it
// exists, an honest computing placeholder whose shape matches the
// finished panel (empty collections marshal as [], never null).
func (h *BestChecklistHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if panel := h.snapshot(); panel != nil {
		writeJSON(w, http.StatusOK, *panel)
		return
	}
	writeJSON(w, http.StatusOK, demos.BestChecklistPanel{
		Mode: "computing: the boot audit is still executing — the panel you want is one " +
			"reload away (probes, tool runs, and file reads are bounded; nothing waits on " +
			"this request)",
		Areas:    []demos.BestChecklistAreaRow{},
		NotDones: []demos.BestNotDoneRow{},
	})
}

// defaultAudit collects the raw facts: the five live probes through the
// private production chain, the three bounded tool executions, the file
// reads under the module and repo directories, and this process's own
// resolved config. Collection failures become data (a reason string),
// never panics and never a hung call.
func (h *BestChecklistHandler) defaultAudit(ctx context.Context) demos.BestChecklistFacts {
	facts := demos.BestChecklistFacts{
		Now:     time.Now(),
		Gofmt:   h.runTool(ctx, bestGofmtTimeout, "gofmt", "gofmt", "-l", "."),
		Ratchet: h.runGoTest(ctx, bestRatchetTimeout, bestRatchetRunFocus, "./internal/content"),
		Race:    h.bestRunRace(ctx),
		Probes:  h.runProbes(ctx),
	}
	facts.GoMod = bestReadFile(h.moduleDir, filepath.Join("go.mod"), "server/go.mod")
	facts.MainGo = bestReadFile(h.moduleDir, filepath.Join("cmd", "guide", "main.go"), "server/cmd/guide/main.go")
	facts.PasswordGo = bestReadFile(h.moduleDir,
		filepath.Join("internal", "domain", "auth", "password.go"),
		"server/internal/domain/auth/password.go")
	facts.AuthStories = bestReadFile(h.moduleDir,
		filepath.Join("internal", "data", "auth", "stories_tokens.go"),
		"server/internal/data/auth/stories_tokens.go")
	facts.Compose = bestReadFile(h.repoDir, filepath.Join(".docker", "docker-compose.yml"), ".docker/docker-compose.yml")

	if h.moduleDir != "" {
		facts.MigrationsFS = os.DirFS(filepath.Join(h.moduleDir, "internal", "data", "db", "migrations"))
	}
	facts.Config = demos.BestConfigFacts{Port: h.cfg.Port, GRPCAddr: h.cfg.GRPCAddr, Env: h.cfg.Env}
	return facts
}

// runProbes drives the five real rounds through the private chain. Any
// round that breaks becomes its recording's ErrText — the row judges
// it; the audit never aborts because its own probe failed.
func (h *BestChecklistHandler) runProbes(ctx context.Context) map[string]demos.BestProbeRecording {
	plan := []struct {
		label  string
		method string
		path   string
		body   string
	}{
		{demos.BestProbeHealth, http.MethodGet, "/api/health", ""},
		{demos.BestProbeScenarioGet, http.MethodGet, "/api/demo/scenario", ""},
		{demos.BestProbeScenarioPatchBad, http.MethodPatch, "/api/demo/scenario", `{"dataMode":"bogus"}`},
		{demos.BestProbePostsNotFound, http.MethodGet, "/api/posts/best-checklist-nope", ""},
		{demos.BestProbePostsInvalidPost, http.MethodPost, "/api/posts", "{}"},
	}
	rounds := make(map[string]demos.BestProbeRecording, len(plan))
	for _, step := range plan {
		rounds[step.label] = h.runProbe(ctx, step.label, step.method, step.path, step.body)
	}
	return rounds
}

// runProbe drives ONE real request through the private chain and
// records what the recorder observed — status, the x-request-id the
// response carried, and the body verbatim.
func (h *BestChecklistHandler) runProbe(ctx context.Context, label, method, path, body string) demos.BestProbeRecording {
	recording := demos.BestProbeRecording{Label: label}
	var payload io.Reader
	if body != "" {
		payload = strings.NewReader(body)
	}
	request, err := http.NewRequestWithContext(ctx, method, path, payload)
	if err != nil {
		recording.ErrText = "build request: " + err.Error()
		return recording
	}
	request.Header.Set(core.RequestIDHeader, bestAuditRequestID)

	recorder := newBufferedResponseRecorder()
	h.probe.ServeHTTP(recorder, request)
	recording.Status = recorder.status
	recording.HeaderRequestID = recorder.header.Get(core.RequestIDHeader)
	recording.Body = recorder.body.String()
	return recording
}

// runTool executes one bounded command in the module directory,
// reporting an honest not-ran outcome when the tree or tool is absent.
func (h *BestChecklistHandler) runTool(ctx context.Context, timeout time.Duration, label, name string, args ...string) demos.BestExecOutcome {
	command := strings.Join(append([]string{name}, args...), " ")
	outcome := demos.BestExecOutcome{Label: label, Command: command}
	if h.moduleDir == "" {
		outcome.ErrText = "the module's source tree is not beside this process"
		return outcome
	}
	binary, err := exec.LookPath(name)
	if err != nil {
		outcome.ErrText = name + " is not on PATH beside this process"
		return outcome
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	output, exitCode, runErr := runBounded(runCtx, h.moduleDir, binary, args...)
	switch {
	case runCtx.Err() == context.DeadlineExceeded:
		outcome.ErrText = "the check exceeded its bounded timeout and the whole process group was killed"
	default:
		outcome.Ran = true
		outcome.ExitCode = exitCode
		outcome.Output = output
		if runErr != nil {
			outcome.ErrText = "execution failed: " + runErr.Error()
		}
	}
	return outcome
}

// runGoTest renders and runs one bounded `go test` round with the -run
// focus the row names.
func (h *BestChecklistHandler) runGoTest(ctx context.Context, timeout time.Duration, focus, pkg string) demos.BestExecOutcome {
	return h.runTool(ctx, timeout, "go-test", "go", "test", "-count=1", "-run", focus, pkg)
}

// bestRunRace is the race round: the -race flag rides the same bounded
// runner, with the command the row serves quoting the flag.
func (h *BestChecklistHandler) bestRunRace(ctx context.Context) demos.BestExecOutcome {
	outcome := h.runTool(ctx, bestRaceTimeout, "go-test-race", "go",
		"test", "-race", "-count=1", "-run", bestRaceRunFocus, "./internal/domain/demos")
	outcome.Command = "go test -race -count=1 -run '" + bestRaceRunFocus + "' ./internal/domain/demos"
	return outcome
}

// bestReadFile reads one file under a root, reporting the repo-root-
// relative path the evidence prints; an absent root or file reads as
// empty content and the row degrades honestly.
func bestReadFile(root string, relUnder string, relPath string) demos.BestFileRead {
	if root == "" {
		return demos.BestFileRead{RelPath: relPath}
	}
	raw, err := os.ReadFile(filepath.Join(root, relUnder))
	if err != nil {
		return demos.BestFileRead{RelPath: relPath}
	}
	return demos.BestFileRead{RelPath: relPath, Content: string(raw)}
}
