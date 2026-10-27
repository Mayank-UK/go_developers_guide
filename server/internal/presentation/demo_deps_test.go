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

// The deps manifest demo handler, exercised through the same
// middleware chain production runs behind (the family rule that every
// new demo endpoint ships with a Go test). The govulncheck runner is
// injected as a fake so the state machine — running, then the cached
// done/unavailable observation — is tested deterministically, without
// executing the real tool.

type depsManifestDTO struct {
	GoMod struct {
		Available      bool   `json:"available"`
		Module         string `json:"module"`
		GoDirective    string `json:"goDirective"`
		IndirectCount  int    `json:"indirectCount"`
		DirectRequires []struct {
			Path    string `json:"path"`
			Version string `json:"version"`
			Linked  bool   `json:"linked"`
		} `json:"directRequires"`
		ToolDirectives []string `json:"toolDirectives"`
		Note           string   `json:"note"`
	} `json:"goMod"`
	Build struct {
		Available   bool   `json:"available"`
		ModulePath  string `json:"modulePath"`
		MainVersion string `json:"mainVersion"`
		GoVersion   string `json:"goVersion"`
		LinkedCount int    `json:"linkedCount"`
		Note        string `json:"note"`
	} `json:"build"`
	Anchors struct {
		Available bool   `json:"available"`
		BuildTag  string `json:"buildTag"`
		Anchors   []struct {
			Path    string `json:"path"`
			Version string `json:"version"`
			Linked  bool   `json:"linked"`
		} `json:"anchors"`
	} `json:"anchors"`
	Cache struct {
		Volumes []struct {
			Name  string `json:"name"`
			Mount string `json:"mount"`
		} `json:"volumes"`
	} `json:"cache"`
	Vulncheck struct {
		Mode      string `json:"mode"`
		Command   string `json:"command"`
		ExitCode  int    `json:"exitCode"`
		ElapsedMs int64  `json:"elapsedMs"`
		Summary   string `json:"summary"`
		Fixture   string `json:"fixture"`
		Note      string `json:"note"`
	} `json:"vulncheck"`
	Reasoning string `json:"reasoning"`
}

func decodeDepsManifest(t *testing.T, body string) depsManifestDTO {
	t.Helper()
	var payload depsManifestDTO
	if err := json.Unmarshal([]byte(body), &payload); err != nil {
		t.Fatalf("manifest payload not the demo DTO shape: %v\n%s", err, body)
	}
	return payload
}

// writeDepsModuleDir materializes a throwaway module directory with the
// go.mod and tools/deps.go the happy-path tests read.
func writeDepsModuleDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	goMod := "module example.com/handlertest\n\ngo 1.27.3\n\nrequire (\n" +
		"\texample.com/dep-one v1.0.0\n\texample.com/dep-two v2.0.0\n)\n\n" +
		"require example.com/shadowed v0.0.0 // indirect\n"
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(goMod), 0o644); err != nil {
		t.Fatalf("write fixture go.mod: %v", err)
	}
	tools := "//go:build tools\n\npackage tools\n\nimport (\n\t_ \"example.com/dep-one\"\n)\n"
	toolsDir := filepath.Join(dir, "tools")
	if err := os.MkdirAll(toolsDir, 0o755); err != nil {
		t.Fatalf("mkdir tools: %v", err)
	}
	if err := os.WriteFile(filepath.Join(toolsDir, "deps.go"), []byte(tools), 0o644); err != nil {
		t.Fatalf("write fixture deps.go: %v", err)
	}
	return dir
}

func newDepsDemoServer(t *testing.T, moduleDir string) (http.Handler, *DepsManifestHandler) {
	t.Helper()
	logger := newTestLogger(&strings.Builder{})
	handler := NewDepsManifestHandler(moduleDir, logger)
	mux := http.NewServeMux()
	mux.Handle("GET "+demos.DepsManifestPath, handler)
	return Chain(
		RequestID,
		RequestLogger(logger),
		Recover(logger),
		EnvelopeMuxErrors(logger),
	)(mux), handler
}

// seedDepsVulncheck installs an already-completed observation — the
// exact state the handler caches after any finished scan — so the
// deterministic tests never race the background goroutine.
func seedDepsVulncheck(t *testing.T, handler *DepsManifestHandler, observation demos.DepsVulncheckObservation) {
	t.Helper()
	handler.mu.Lock()
	defer handler.mu.Unlock()
	handler.started = true
	handler.result = &observation
}

func TestDepsManifestServesTheParsedLiveSources(t *testing.T) {
	moduleDir := writeDepsModuleDir(t)
	server, handler := newDepsDemoServer(t, moduleDir)
	seedDepsVulncheck(t, handler, demos.DepsVulncheckObservation{
		State:    demos.DepsVulncheckStateDone,
		ExitCode: 0,
		Elapsed:  3 * time.Second,
		Summary:  "No vulnerabilities found.",
	})

	rec := doRequest(server, http.MethodGet, demos.DepsManifestPath, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if rec.Header().Get(core.RequestIDHeader) == "" {
		t.Error("every response must carry x-request-id")
	}

	payload := decodeDepsManifest(t, rec.Body.String())
	if !payload.GoMod.Available || payload.GoMod.Module != "example.com/handlertest" ||
		payload.GoMod.GoDirective != "1.27.3" {
		t.Errorf("goMod panel = %+v, want the parsed fixture manifest", payload.GoMod)
	}
	if len(payload.GoMod.DirectRequires) != 2 || payload.GoMod.IndirectCount != 1 {
		t.Errorf("goMod counts = %+v, want 2 direct / 1 indirect", payload.GoMod)
	}
	if payload.Vulncheck.Mode != "live" || payload.Vulncheck.ExitCode != 0 ||
		!strings.Contains(payload.Vulncheck.Summary, "No vulnerabilities found.") {
		t.Errorf("vulncheck panel = %+v, want the live observation passed through", payload.Vulncheck)
	}
	if !payload.Anchors.Available || len(payload.Anchors.Anchors) != 1 ||
		payload.Anchors.Anchors[0].Path != "example.com/dep-one" ||
		payload.Anchors.Anchors[0].Version != "v1.0.0" {
		t.Errorf("anchors = %+v, want the fixture anchor paired with its require", payload.Anchors)
	}
	// The build panel reports the TEST binary's own info: the guide
	// module, honestly labeled.
	if !payload.Build.Available || payload.Build.ModulePath != "guide" {
		t.Errorf("build panel = %+v, want the serving test binary's guide module", payload.Build)
	}
	if !strings.Contains(payload.Build.Note, "process serving this response") {
		t.Errorf("build note must name which binary's info it carries: %q", payload.Build.Note)
	}
	if len(payload.Cache.Volumes) != 3 {
		t.Errorf("cache volumes = %d, want the three named volumes", len(payload.Cache.Volumes))
	}
}

// TestDepsManifestVulncheckRunsOnceAndCaches pins the state machine:
// the first request reports the in-flight scan and starts exactly one,
// later requests serve the cached observation forever after.
func TestDepsManifestVulncheckRunsOnceAndCaches(t *testing.T) {
	moduleDir := writeDepsModuleDir(t)
	server, handler := newDepsDemoServer(t, moduleDir)
	release := make(chan struct{})
	var calls atomic.Int32
	handler.runVulncheck = func(context.Context) demos.DepsVulncheckObservation {
		calls.Add(1)
		<-release
		return demos.DepsVulncheckObservation{
			State:    demos.DepsVulncheckStateDone,
			ExitCode: 3,
			Elapsed:  time.Second,
			Summary:  "Your code is affected by 1 vulnerability from 1 module.",
		}
	}

	first := decodeDepsManifest(t, doRequest(server, http.MethodGet, demos.DepsManifestPath, "").Body.String())
	if first.Vulncheck.Mode != "running" {
		t.Fatalf("first payload vulncheck mode = %q, want running", first.Vulncheck.Mode)
	}
	// The scan starts in a goroutine, so its first instruction may land
	// a beat after the response is written — poll for the call rather
	// than asserting it synchronously (the fake blocks on release, so
	// the count can only be 0 or 1 here, never more).
	startDeadline := time.Now().Add(2 * time.Second)
	for calls.Load() != 1 {
		if time.Now().After(startDeadline) {
			t.Fatalf("scan calls = %d after first request, want exactly 1", calls.Load())
		}
		time.Sleep(5 * time.Millisecond)
	}

	close(release)
	deadline := time.Now().Add(2 * time.Second)
	for {
		second := decodeDepsManifest(t, doRequest(server, http.MethodGet, demos.DepsManifestPath, "").Body.String())
		if second.Vulncheck.Mode == "live" {
			if second.Vulncheck.ExitCode != 3 || !strings.Contains(second.Vulncheck.Summary, "1 vulnerability") {
				t.Errorf("cached observation = %+v, want exit 3 with the tool's verdict", second.Vulncheck)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("scan never landed: mode stayed %q", second.Vulncheck.Mode)
		}
		time.Sleep(10 * time.Millisecond)
	}

	third := decodeDepsManifest(t, doRequest(server, http.MethodGet, demos.DepsManifestPath, "").Body.String())
	if third.Vulncheck.Mode != "live" || calls.Load() != 1 {
		t.Errorf("third request mode/calls = %q/%d, want the cached live result with no re-run",
			third.Vulncheck.Mode, calls.Load())
	}
}

// TestDepsManifestDegradesWithoutSourcesOrTool pins the honest
// degradation: no module directory and an unavailable scan leave the
// source panels empty (not guessed) and the vulncheck panel teaching
// from the clearly-labeled example.
func TestDepsManifestDegradesWithoutSourcesOrTool(t *testing.T) {
	server, handler := newDepsDemoServer(t, "")
	seedDepsVulncheck(t, handler, demos.DepsVulncheckObservation{
		State:  demos.DepsVulncheckStateUnavailable,
		Reason: "govulncheck is not installed here (go install golang.org/x/vuln/cmd/govulncheck@latest)",
	})

	rec := doRequest(server, http.MethodGet, demos.DepsManifestPath, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (degradation is data, not failure)", rec.Code)
	}
	payload := decodeDepsManifest(t, rec.Body.String())
	if payload.GoMod.Available || payload.Anchors.Available {
		t.Errorf("source panels = %+v %+v, want both unavailable", payload.GoMod, payload.Anchors)
	}
	if payload.GoMod.DirectRequires == nil || payload.Anchors.Anchors == nil {
		t.Error("degraded collections must still marshal as [] never null")
	}
	if payload.Vulncheck.Mode != "fixtures" || payload.Vulncheck.Fixture == "" {
		t.Errorf("vulncheck = %+v, want fixtures mode carrying the example", payload.Vulncheck)
	}
	if !strings.Contains(payload.Vulncheck.Note, "not installed here") ||
		!strings.Contains(payload.Vulncheck.Note, "not a scan of this repo") {
		t.Errorf("fixtures note must carry the reason and the label: %q", payload.Vulncheck.Note)
	}
}

// TestDepsVulncheckDefaultRunnerDegradesWithoutAModuleDir exercises
// the real runner's guard without executing any tool: no source tree
// beside the process means the honest unavailable observation.
func TestDepsVulncheckDefaultRunnerDegradesWithoutAModuleDir(t *testing.T) {
	runner := depsDefaultVulncheckRunner("", newTestLogger(&strings.Builder{}))
	observation := runner(context.Background())
	if observation.State != demos.DepsVulncheckStateUnavailable {
		t.Fatalf("state = %q, want unavailable", observation.State)
	}
	if !strings.Contains(observation.Reason, "source tree") {
		t.Errorf("reason = %q, want the no-source-tree honesty", observation.Reason)
	}
}

// TestDepsVulncheckSummaryCarriesTheToolsOwnWords pins the extraction:
// the closing summary block survives verbatim, long reports are capped
// to a readable tail, and a stdout-less failure falls back to stderr.
func TestDepsVulncheckSummaryCarriesTheToolsOwnWords(t *testing.T) {
	report := "Vulnerability #1: GO-2021-0113\n    details\n\n" +
		"=== Symbol Results ===\n\nNo vulnerabilities found.\n\n" +
		"Your code is affected by 0 vulnerabilities.\n" +
		"This scan also found 0 vulnerabilities in packages you import and 1\n" +
		"vulnerability in modules you require, but your code doesn't appear to call these\n" +
		"vulnerabilities.\n"
	summary := depsVulncheckSummary(report, "")
	for _, want := range []string{
		"No vulnerabilities found.",
		"Your code is affected by 0 vulnerabilities.",
		"vulnerability in modules you require",
	} {
		if !strings.Contains(summary, want) {
			t.Errorf("summary %q lost the tool's own line %q", summary, want)
		}
	}
	if strings.Contains(summary, "GO-2021-0113") {
		t.Errorf("summary %q should carry only the closing block, not the detail tables", summary)
	}

	empty := depsVulncheckSummary("", "the tool failed to load")
	if empty != "the tool failed to load" {
		t.Errorf("stderr fallback = %q, want the tool's failure line", empty)
	}
}

func TestDepsManifestRejectsNonGet(t *testing.T) {
	server, handler := newDepsDemoServer(t, "")
	seedDepsVulncheck(t, handler, demos.DepsVulncheckObservation{
		State:  demos.DepsVulncheckStateUnavailable,
		Reason: "not under test",
	})

	rec := doRequest(server, http.MethodPost, demos.DepsManifestPath, "")
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST status = %d, want 405", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"code":"not_found"`) {
		t.Errorf("POST body must be the envelope: %s", rec.Body.String())
	}
	if allow := rec.Header().Get("Allow"); !strings.Contains(allow, "GET") {
		t.Errorf("POST Allow header lost: %q", allow)
	}
}
