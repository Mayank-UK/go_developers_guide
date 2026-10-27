package demos

import (
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"testing"
	"time"
)

// The deps manifest payload builder, pinned to the family rules: every
// number derives from the facts (go.mod parsed live, build info read
// from whatever binary runs the builder, the govulncheck observation
// mapped — never paraphrased), unavailable sources degrade honestly,
// and every collection stays non-nil so the wire renders [] not null.

const depsFixtureGoMod = `module example.com/fake

go 1.27.3

require (
	example.com/direct-one v1.2.3
	example.com/direct-two v2.0.0+incompatible
)

require (
	example.com/transitive-a v0.1.0 // indirect
	example.com/transitive-b v0.2.0 // indirect
)

require example.com/single-line v3.1.0

tool (
	example.com/tool-one
	example.com/tool-two
)

replace (
	example.com/replaced => ../local
)
`

const depsFixtureTools = `//go:build tools

package tools

import (
	_ "example.com/direct-one"
	_ "example.com/tool-two"
	_ "example.com/not-a-require"
)
`

// depsFixtureBuildInfo shapes a minimal build info the way the go tool
// stamps one: main module plus the deps the fake binary links.
func depsFixtureBuildInfo() *debug.BuildInfo {
	return &debug.BuildInfo{
		GoVersion: "go1.27.3",
		Main:      debug.Module{Path: "example.com/fake", Version: "(devel)"},
		Deps: []*debug.Module{
			{Path: "example.com/direct-one", Version: "v1.2.3"},
			{Path: "example.com/transitive-a", Version: "v0.1.0"},
		},
	}
}

func TestDepsManifestParsesTheGoModHonesty(t *testing.T) {
	modulePath, goDirective, direct, indirect, tools := parseGoMod(depsFixtureGoMod)
	if modulePath != "example.com/fake" {
		t.Errorf("module = %q, want example.com/fake", modulePath)
	}
	if goDirective != "1.27.3" {
		t.Errorf("go directive = %q, want 1.27.3", goDirective)
	}
	if len(direct) != 3 {
		t.Errorf("direct requires = %v, want 3 (two block + one single-line)", direct)
	}
	if len(indirect) != 2 {
		t.Errorf("indirect requires = %v, want 2", indirect)
	}
	if got := direct[0]; got.Path != "example.com/direct-one" || got.Version != "v1.2.3" {
		t.Errorf("first direct require = %+v, want direct-one v1.2.3 (declared order)", got)
	}
	if got := direct[2]; got.Path != "example.com/single-line" {
		t.Errorf("single-line require = %+v, want example.com/single-line", got)
	}
	wantTools := []string{"example.com/tool-one", "example.com/tool-two"}
	if strings.Join(tools, ",") != strings.Join(wantTools, ",") {
		t.Errorf("tool directives = %v, want %v", tools, wantTools)
	}
}

func TestDepsManifestResolvesPackagePathsOntoModules(t *testing.T) {
	requires := map[string]string{
		"golang.org/x/crypto": "v0.58.0",
		"golang.org/x/time":   "v0.16.0",
	}
	for importPath, wantModule := range map[string]string{
		"golang.org/x/crypto/argon2":            "golang.org/x/crypto",
		"golang.org/x/time/rate":                "golang.org/x/time",
		"golang.org/x/crypto":                   "golang.org/x/crypto",
		"example.com/no-such-module/pkg/thing":  "example.com/no-such-module/pkg/thing",
		"golang.org/x/cryptography-toolkit/sub": "golang.org/x/cryptography-toolkit/sub",
	} {
		if got := depsResolveModulePath(importPath, requires); got != wantModule {
			t.Errorf("resolve(%q) = %q, want %q", importPath, got, wantModule)
		}
	}
}

func TestDepsManifestPairsFactsIntoPanels(t *testing.T) {
	payload := BuildDepsManifestPayload(DepsManifestFacts{
		GoModContent: depsFixtureGoMod,
		ToolsContent: depsFixtureTools,
		BuildInfo:    depsFixtureBuildInfo(),
		Vulncheck: DepsVulncheckObservation{
			State:    DepsVulncheckStateDone,
			ExitCode: 0,
			Elapsed:  3*time.Second + 281*time.Millisecond,
			Summary:  "No vulnerabilities found.",
		},
	})

	if !payload.GoMod.Available || payload.GoMod.Module != "example.com/fake" {
		t.Errorf("goMod panel = %+v, want the parsed manifest", payload.GoMod)
	}
	if len(payload.GoMod.DirectRequires) != 3 || payload.GoMod.IndirectCount != 2 {
		t.Errorf("goMod counts = %d direct/%d indirect, want 3/2",
			len(payload.GoMod.DirectRequires), payload.GoMod.IndirectCount)
	}
	// Linked is the manifest-vs-binary lesson: direct-one links (the
	// fake binary imports it), direct-two and single-line do not.
	linkedByPath := map[string]bool{}
	for _, row := range payload.GoMod.DirectRequires {
		linkedByPath[row.Path] = row.Linked
	}
	if !linkedByPath["example.com/direct-one"] {
		t.Error("direct-one must be linked: the fixture binary imports it")
	}
	if linkedByPath["example.com/direct-two"] || linkedByPath["example.com/single-line"] {
		t.Error("direct-two/single-line are not in the fixture build info — Linked must be false")
	}

	if !payload.Build.Available || payload.Build.ModulePath != "example.com/fake" ||
		payload.Build.LinkedCount != 2 {
		t.Errorf("build panel = %+v, want the fixture build info (2 linked)", payload.Build)
	}
	if !strings.Contains(payload.Build.Note, "process serving this response") {
		t.Errorf("build note must say WHICH binary's info it is: %q", payload.Build.Note)
	}

	// Anchors pair each blank import with its live require version and
	// link status; sorted for determinism.
	if !payload.Anchors.Available || len(payload.Anchors.Anchors) != 3 {
		t.Fatalf("anchors = %+v, want the 3 fixture blank imports", payload.Anchors)
	}
	anchorsByPath := map[string]DepsRequireRow{}
	for _, row := range payload.Anchors.Anchors {
		anchorsByPath[row.Path] = row
	}
	if row := anchorsByPath["example.com/direct-one"]; row.Version != "v1.2.3" || !row.Linked {
		t.Errorf("anchor direct-one = %+v, want version from go.mod and linked", row)
	}
	if row := anchorsByPath["example.com/tool-two"]; row.Version != "" || row.Linked {
		t.Errorf("anchor tool-two = %+v, want no require version and unlinked (a tool, not a require)", row)
	}
	if row := anchorsByPath["example.com/not-a-require"]; row.Version != "" {
		t.Errorf("anchor without a require must carry an empty version, not an invented one: %+v", row)
	}

	if len(payload.Cache.Volumes) != 3 {
		t.Errorf("cache volumes = %d rows, want the 3 named volumes (gdg-go, gdg-gocache, gdg-npm)",
			len(payload.Cache.Volumes))
	}
	for _, name := range []string{"gdg-go", "gdg-gocache", "gdg-npm"} {
		found := false
		for _, volume := range payload.Cache.Volumes {
			if volume.Name == name {
				found = true
			}
		}
		if !found {
			t.Errorf("cache panel missing the real volume %q", name)
		}
	}

	vuln := payload.Vulncheck
	if vuln.Mode != DepsVulncheckModeLive || vuln.ExitCode != 0 || vuln.ElapsedMs != 3281 {
		t.Errorf("vulncheck panel = %+v, want live/0/3281ms", vuln)
	}
	if !strings.Contains(vuln.Summary, "No vulnerabilities found.") {
		t.Errorf("live summary must be the tool's own words verbatim: %q", vuln.Summary)
	}
}

func TestDepsManifestDegradesHonestlyWithoutSources(t *testing.T) {
	payload := BuildDepsManifestPayload(DepsManifestFacts{
		Vulncheck: DepsVulncheckObservation{
			State:  DepsVulncheckStateUnavailable,
			Reason: "govulncheck: not found in PATH",
		},
	})

	if payload.GoMod.Available || payload.Anchors.Available || payload.Build.Available {
		t.Errorf("empty facts must degrade every source panel: %+v", payload)
	}
	if !strings.Contains(payload.GoMod.Note, "not available beside this process") {
		t.Errorf("goMod degradation note must say why it is empty: %q", payload.GoMod.Note)
	}
	// Non-nil empties: the wire renders [] never null.
	if payload.GoMod.DirectRequires == nil || payload.GoMod.ToolDirectives == nil ||
		payload.Anchors.Anchors == nil || payload.Cache.Volumes == nil {
		t.Error("collections must be non-nil so the wire renders [] not null")
	}

	vuln := payload.Vulncheck
	if vuln.Mode != DepsVulncheckModeFixtures {
		t.Errorf("unavailable scan must map to fixtures mode, got %q", vuln.Mode)
	}
	if !strings.Contains(vuln.Note, "govulncheck: not found in PATH") {
		t.Errorf("fixtures note must carry the honest reason: %q", vuln.Note)
	}
	if !strings.Contains(vuln.Note, "not a scan of this repo") {
		t.Errorf("fixtures note must say the example is not a scan of this repo: %q", vuln.Note)
	}
	if !strings.Contains(vuln.Fixture, "GO-2021-0113") {
		t.Errorf("fixture must carry the documented example output: %q", vuln.Fixture)
	}
}

func TestDepsManifestVulncheckPanelMapsEveryState(t *testing.T) {
	running := BuildDepsVulncheckPanel(DepsVulncheckObservation{State: DepsVulncheckStateRunning})
	if running.Mode != DepsVulncheckModeRunning || running.Summary != "" || running.Fixture != "" {
		t.Errorf("running panel = %+v, want mode running with no verdict", running)
	}
	if !strings.Contains(running.Note, "reload") {
		t.Errorf("running note must invite the reload: %q", running.Note)
	}

	findings := BuildDepsVulncheckPanel(DepsVulncheckObservation{
		State:    DepsVulncheckStateDone,
		ExitCode: 3,
		Elapsed:  5 * time.Second,
		Summary:  "Your code is affected by 1 vulnerability from 1 module.",
	})
	if findings.Mode != DepsVulncheckModeLive || findings.ExitCode != 3 || findings.Fixture != "" {
		t.Errorf("findings panel = %+v, want live mode carrying the tool's verdict", findings)
	}

	noReason := BuildDepsVulncheckPanel(DepsVulncheckObservation{State: DepsVulncheckStateUnavailable})
	if !strings.Contains(noReason.Note, "no reason was recorded") {
		t.Errorf("an unavailable scan with no reason must say so honestly: %q", noReason.Note)
	}
}

// TestDepsManifestDerivesFromTheLiveModule pins the demo's headline
// honesty against the REAL repo artifacts: the live go.mod parses with
// the guide module's stable facts, the real tools/deps.go anchors all
// resolve to requires, and the test binary's own build info carries
// the guide module with a non-empty linked set.
func TestDepsManifestDerivesFromTheLiveModule(t *testing.T) {
	serverDir := depsFindServerDir(t)
	raw, err := os.ReadFile(filepath.Join(serverDir, "go.mod"))
	if err != nil {
		t.Fatalf("read the live go.mod: %v", err)
	}
	modulePath, goDirective, direct, indirect, tools := parseGoMod(string(raw))
	if modulePath != "guide" {
		t.Errorf("live module = %q, want guide", modulePath)
	}
	if goDirective == "" {
		t.Error("live go directive must parse non-empty")
	}
	// Headline pins: the phases that landed these deps are done, so the
	// requires are stable facts of the module — not a drifting count.
	directPaths := map[string]bool{}
	for _, row := range direct {
		directPaths[row.Path] = true
		if row.Version == "" {
			t.Errorf("direct require %q carries no version", row.Path)
		}
	}
	for _, pinned := range []string{
		"github.com/jackc/pgx/v5",
		"github.com/redis/go-redis/v9",
		"github.com/hibiken/asynq",
		"github.com/coder/websocket",
		"google.golang.org/grpc",
		"github.com/testcontainers/testcontainers-go",
	} {
		if !directPaths[pinned] {
			t.Errorf("live manifest lost the pinned direct require %q", pinned)
		}
	}
	if len(indirect) <= len(direct) {
		t.Errorf("live indirect count (%d) must dwarf the direct count (%d) — the transitive story",
			len(indirect), len(direct))
	}
	if len(tools) == 0 {
		t.Error("live tool directives must parse (the module pins codegen tools)")
	}

	toolsRaw, err := os.ReadFile(filepath.Join(serverDir, "tools", "deps.go"))
	if err != nil {
		t.Fatalf("read the live tools/deps.go: %v", err)
	}
	anchors := parseBlankImports(string(toolsRaw))
	if len(anchors) == 0 {
		t.Fatal("live anchors must parse")
	}
	requires := map[string]bool{}
	versions := map[string]string{}
	for _, row := range append(append([]DepsRequireRow(nil), direct...), indirect...) {
		requires[row.Path] = true
		versions[row.Path] = row.Version
	}
	for _, anchor := range anchors {
		// Anchors name package paths; requires name modules — resolve
		// the way the builder (and the go command) do.
		resolved := depsResolveModulePath(anchor, versions)
		if !requires[resolved] {
			t.Errorf("anchor %q resolves to no require — tidy drift", anchor)
		}
	}

	// The binary running this test IS a guide-module binary: its build
	// info must carry the guide module, and every Linked flag in the
	// manifest panel must agree with that build info's own dep set —
	// the manifest-vs-binary invariant, pinned against live truth.
	buildInfo, ok := debug.ReadBuildInfo()
	if !ok || buildInfo == nil {
		t.Fatal("test binary must carry build info")
	}
	payload := BuildDepsManifestPayload(DepsManifestFacts{
		GoModContent: string(raw),
		ToolsContent: string(toolsRaw),
		BuildInfo:    buildInfo,
		Vulncheck:    DepsVulncheckObservation{State: DepsVulncheckStateRunning},
	})
	if payload.Build.ModulePath != "guide" {
		t.Errorf("build module path = %q, want guide (the main module stamps every go-built binary)", payload.Build.ModulePath)
	}
	if payload.Build.LinkedCount != len(buildInfo.Deps) {
		t.Errorf("linked count = %d, want the live len(buildInfo.Deps) = %d",
			payload.Build.LinkedCount, len(buildInfo.Deps))
	}
	liveLinked := map[string]bool{}
	for _, dep := range buildInfo.Deps {
		liveLinked[dep.Path] = true
	}
	for _, row := range payload.GoMod.DirectRequires {
		if row.Linked != liveLinked[row.Path] {
			t.Errorf("require %q Linked=%v, want %v (membership in this binary's build info)",
				row.Path, row.Linked, liveLinked[row.Path])
		}
	}
}

// depsFindServerDir walks up from the test's working directory to the
// directory holding go.mod — the server module, where go.mod and
// tools/deps.go live.
func depsFindServerDir(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getcwd: %v", err)
	}
	for {
		if _, statErr := os.Stat(filepath.Join(dir, "go.mod")); statErr == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("no go.mod found above the test's working directory")
		}
		dir = parent
	}
}
