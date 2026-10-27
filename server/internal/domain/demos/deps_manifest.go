// Topic 31 dependency-management's live demo: the deps manifest. The
// panel puts the module's two truths side by side, both derived at
// runtime and never hardcoded: the MANIFEST (server/go.mod, parsed live
// — module and go directives, the direct requires with their MVS-pinned
// versions, the indirect count, the tool directives) and the BINARY
// (runtime/debug.ReadBuildInfo of the process serving the response —
// the modules it actually links, a strict subset of the manifest that
// excludes test-only requires like testcontainers-go). Around that
// core: the tools/deps.go anchor story (blank imports behind the
// `tools` build tag are why `go mod tidy` keeps pins it cannot see code
// for), the Docker cache-volume story with the repo's real volume
// names, and a govulncheck section that runs the real tool against the
// source tree when it is available beside the process (the dev
// container) and degrades to a clearly-labeled example when it is not
// (the distroless prod image ships no sources and no toolchain). The
// builder below is PURE: facts in, payload out — every number comes
// from the inputs, so nothing here can drift from the module it
// describes. The go-http-fundamentals app-info demo (topic 04) already
// serves one slice of this story (the linked-module count); this demo
// goes deeper and cross-references it instead of duplicating it.
package demos

import (
	"runtime/debug"
	"sort"
	"strings"
	"time"
)

// DepsManifestPath is the demo area's endpoint — registered on the real
// mux by the composition root, so a plain curl exercises the payload
// this builder shapes.
const DepsManifestPath = "/api/demo/deps/manifest"

// DepsGoModName and DepsToolsName are the repo-relative labels the
// payload names its sources by — the paths the handler reads (relative
// to the module directory) and the panel displays.
const (
	DepsGoModName  = "server/go.mod"
	DepsToolsName  = "server/tools/deps.go"
	DepsToolsTag   = "tools"
	DepsAnchorNote = "blank imports behind the tools build tag"
)

// DepsGovulncheckCommand is the exact invocation the panel names and
// the default handler runner executes against the module directory.
const DepsGovulncheckCommand = "govulncheck ./..."

// DepsGovulncheckTimeout bounds the live scan the handler runs in the
// background: generous enough for a cold vulnerability-DB download,
// bounded enough that a wedged tool never pins a goroutine forever.
const DepsGovulncheckTimeout = 90 * time.Second

// Vulncheck observation states — what happened to the one scan this
// process runs (started on the first request, cached for the process's
// lifetime after that).
const (
	// DepsVulncheckStateRunning names a scan that is in flight.
	DepsVulncheckStateRunning = "running"
	// DepsVulncheckStateDone names a scan that finished, findings or
	// not — the exit code and summary carry the verdict.
	DepsVulncheckStateDone = "done"
	// DepsVulncheckStateUnavailable names a scan that could not run at
	// all — missing tool, missing source tree, or timeout.
	DepsVulncheckStateUnavailable = "unavailable"
)

// Panel modes — running (in flight), live (the real tool's real
// output), fixtures (an example, clearly labeled, because the real tool
// could not run here).
const (
	DepsVulncheckModeRunning  = "running"
	DepsVulncheckModeLive     = "live"
	DepsVulncheckModeFixtures = "fixtures"
)

// DepsVulncheckObservation is the handler's observation of the one
// govulncheck attempt this process makes: the state, and for a finished
// scan the exit code, elapsed time, and the tool's own closing summary
// lines (verbatim — the panel never paraphrases a verdict it did not
// observe), and for an unavailable scan the honest reason.
type DepsVulncheckObservation struct {
	State    string
	ExitCode int
	Elapsed  time.Duration
	Summary  string
	Reason   string
}

// DepsRequireRow is one require as the payload shows it: the module
// path, the version MVS selected for it (read from the live go.mod),
// and whether the serving binary actually links it — the manifest-vs-
// binary distinction this demo exists to teach.
type DepsRequireRow struct {
	Path    string `json:"path"`
	Version string `json:"version"`
	Linked  bool   `json:"linked"`
}

// DepsGoModPanel is the module's manifest, parsed from the live go.mod
// beside the process. Available is false when the source tree is not
// there (the prod binary's honest case) and the note says so.
type DepsGoModPanel struct {
	Available      bool             `json:"available"`
	File           string           `json:"file"`
	Module         string           `json:"module"`
	GoDirective    string           `json:"goDirective"`
	DirectRequires []DepsRequireRow `json:"directRequires"`
	IndirectCount  int              `json:"indirectCount"`
	ToolDirectives []string         `json:"toolDirectives"`
	Note           string           `json:"note"`
}

// DepsBuildPanel is the serving binary's own birth certificate, exactly
// as runtime/debug.ReadBuildInfo reports it — including WHICH binary
// that is: the process answering this request (the go-run server in the
// dev container, the built binary in prod, a test binary under go
// test), each stamping slightly different facts.
type DepsBuildPanel struct {
	Available   bool   `json:"available"`
	ModulePath  string `json:"modulePath"`
	MainVersion string `json:"mainVersion"`
	GoVersion   string `json:"goVersion"`
	LinkedCount int    `json:"linkedCount"`
	Note        string `json:"note"`
}

// DepsAnchorPanel is the tools/deps.go story: the blank-import anchors
// that keep `go mod tidy` honest about pins no server package imports
// yet, each paired with its live require version and whether the
// serving binary links it.
type DepsAnchorPanel struct {
	Available bool             `json:"available"`
	File      string           `json:"file"`
	BuildTag  string           `json:"buildTag"`
	Anchors   []DepsRequireRow `json:"anchors"`
	Note      string           `json:"note"`
}

// DepsCacheVolumeRow is one named cache volume the dev compose stack
// mounts — the reason docker-exec verification runs warm.
type DepsCacheVolumeRow struct {
	Name        string `json:"name"`
	Mount       string `json:"mount"`
	WhatItHolds string `json:"whatItHolds"`
	WhyWarm     string `json:"whyWarm"`
}

// DepsCachePanel is the Docker cache story: the named volumes, why they
// are external (pre-created, shared with the stack-down fallback), and
// the manifests-first layer ordering that keeps the prod dependency
// layer cacheable.
type DepsCachePanel struct {
	Volumes        []DepsCacheVolumeRow `json:"volumes"`
	LayerOrderNote string               `json:"layerOrderNote"`
	ExternalNote   string               `json:"externalNote"`
}

// DepsVulncheckPanel is the govulncheck section: mode live when the
// real tool ran (exit code, elapsed, the tool's own summary verbatim),
// mode fixtures when it could not (a documented example, labeled as
// such), mode running while the background scan is in flight.
type DepsVulncheckPanel struct {
	Mode      string `json:"mode"`
	Command   string `json:"command"`
	ExitCode  int    `json:"exitCode"`
	ElapsedMs int64  `json:"elapsedMs"`
	Summary   string `json:"summary,omitempty"`
	Fixture   string `json:"fixture,omitempty"`
	Note      string `json:"note"`
}

// DepsManifestPayload is the wire shape of GET /api/demo/deps/manifest.
// Every collection is non-nil so the wire renders [] — never null.
type DepsManifestPayload struct {
	GoMod     DepsGoModPanel     `json:"goMod"`
	Build     DepsBuildPanel     `json:"build"`
	Anchors   DepsAnchorPanel    `json:"anchors"`
	Cache     DepsCachePanel     `json:"cache"`
	Vulncheck DepsVulncheckPanel `json:"vulncheck"`
	Reasoning string             `json:"reasoning"`
}

// DepsManifestFacts is everything the payload derives from, observed by
// the handler: the raw bytes of the module's go.mod and of tools/deps.go
// (empty strings when the source tree is not available beside the
// process — the prod binary's case), the serving process's build info,
// and the current govulncheck observation. The builder is pure over
// these facts.
type DepsManifestFacts struct {
	GoModContent string
	ToolsContent string
	BuildInfo    *debug.BuildInfo
	Vulncheck    DepsVulncheckObservation
}

// depsVulncheckFixture is the documented output shape govulncheck
// prints for a finding — abridged from the tool's own published example
// (the GO-2021-0113 / golang.org/x/text report its docs walk through).
// It is NEVER served as a scan of this repo: the panel carries it only
// in fixtures mode, and the note says exactly that.
const depsVulncheckFixture = `Vulnerability #1: GO-2021-0113
    Due to improper index calculation, an incorrectly formatted
    language tag can cause Parse to panic, due to an out of bounds read.
  More info: https://pkg.go.dev/vuln/GO-2021-0113
  Module: golang.org/x/text
    Found in: golang.org/x/text@v0.3.0
    Fixed in: golang.org/x/text@v0.3.8

=== Symbol Results ===

    Your code is affected by 1 vulnerability...

Your code is affected by 1 vulnerability from 1 module.
`

// BuildDepsManifestPayload shapes the panel (pure) from observed facts:
// go.mod parsed live, build info read from the serving binary, anchors
// cross-referenced against both, the static cache story, and the
// govulncheck observation mapped onto its panel — never a number the
// inputs did not carry.
func BuildDepsManifestPayload(facts DepsManifestFacts) DepsManifestPayload {
	linked := linkedModuleSet(facts.BuildInfo)

	modulePath, goDirective, direct, indirect, tools := parseGoMod(facts.GoModContent)
	goModPanel := DepsGoModPanel{
		Available:      facts.GoModContent != "",
		File:           DepsGoModName,
		Module:         modulePath,
		GoDirective:    goDirective,
		DirectRequires: markLinked(direct, linked),
		IndirectCount:  len(indirect),
		ToolDirectives: tools,
	}
	if !goModPanel.Available {
		goModPanel.Note = "the source tree is not available beside this process (the " +
			"distroless prod image ships only the binary) — the manifest panel derives " +
			"from go.mod, so it stays empty here rather than guessing; the build-info " +
			"panel beside it still reports the binary's live facts"
	}

	buildPanel := DepsBuildPanel{Note: ""}
	if facts.BuildInfo != nil {
		buildPanel.Available = true
		buildPanel.ModulePath = facts.BuildInfo.Main.Path
		buildPanel.MainVersion = facts.BuildInfo.Main.Version
		buildPanel.GoVersion = facts.BuildInfo.GoVersion
		buildPanel.LinkedCount = len(facts.BuildInfo.Deps)
		buildPanel.Note = "read at runtime from runtime/debug.ReadBuildInfo of the " +
			"process serving this response — the go-run dev server, the built prod " +
			"binary, or a test binary under go test, each stamping its own facts; " +
			"the linked count is the modules THIS binary imports transitively, a " +
			"subset of the manifest that excludes test-only requires"
	} else {
		buildPanel.Note = "no build info stamped into this process — the go tool " +
			"always stamps one, so this is the honest empty case, not a failure"
	}

	anchorPanel := buildDepsAnchorPanel(facts.ToolsContent, direct, indirect, linked)

	return DepsManifestPayload{
		GoMod:     goModPanel,
		Build:     buildPanel,
		Anchors:   anchorPanel,
		Cache:     buildDepsCachePanel(),
		Vulncheck: BuildDepsVulncheckPanel(facts.Vulncheck),
		Reasoning: "One module, two honest views: go.mod states what the build may use " +
			"(direct requires plus the indirect graph MVS selected), the build info " +
			"states what this binary actually linked — and the difference between the " +
			"two is the lesson (test-only requires, tool anchors, and the unlinked " +
			"long tail of the graph). The go-http app-info demo serves the linked " +
			"count as its dependencyCount; this panel derives both sides and names " +
			"the distinction. govulncheck closes the loop: not what you require, but " +
			"what your code can actually REACH",
	}
}

// BuildDepsVulncheckPanel maps one observation onto the panel: a done
// scan is live (the tool's exit code, elapsed, and closing summary
// verbatim — the same output `govulncheck ./...` prints in a terminal),
// an unavailable scan degrades to the documented example with the
// reason stated, and an in-flight scan says so and invites the reload.
func BuildDepsVulncheckPanel(obs DepsVulncheckObservation) DepsVulncheckPanel {
	panel := DepsVulncheckPanel{Command: DepsGovulncheckCommand}
	switch obs.State {
	case DepsVulncheckStateDone:
		panel.Mode = DepsVulncheckModeLive
		panel.ExitCode = obs.ExitCode
		panel.ElapsedMs = obs.Elapsed.Milliseconds()
		panel.Summary = obs.Summary
		panel.Note = "the real tool ran just now, against this module's source tree, " +
			"vulnerability DB and all — the summary below is its own closing output, " +
			"passed through verbatim (exit 0 = no vulnerability your code reaches; " +
			"the tool exits 3 when reachable findings exist); the scan runs once per " +
			"process and the result is cached, so reloads serve the observation"
	case DepsVulncheckStateRunning:
		panel.Mode = DepsVulncheckModeRunning
		panel.Note = "the scan is running in the background right now (bounded by a " +
			"timeout, started by the first request this process served) — reload in " +
			"a moment for the live verdict"
	default:
		panel.Mode = DepsVulncheckModeFixtures
		panel.Fixture = depsVulncheckFixture
		panel.Note = "the tool could not run here — " + firstSentenceOrHonest(obs.Reason) +
			" — so this panel teaches the tool's output shape from its documented " +
			"example instead: an EXAMPLE, clearly not a scan of this repo"
	}
	return panel
}

// buildDepsAnchorPanel pairs the blank-import anchors with their live
// require versions and link status.
func buildDepsAnchorPanel(toolsContent string, direct, indirect []DepsRequireRow, linked map[string]bool) DepsAnchorPanel {
	panel := DepsAnchorPanel{
		Available: toolsContent != "",
		File:      DepsToolsName,
		BuildTag:  DepsToolsTag,
		Anchors:   make([]DepsRequireRow, 0),
	}
	if !panel.Available {
		panel.Note = "tools/deps.go is not available beside this process — the anchor " +
			"story derives from the real file, so it stays empty here rather than " +
			"reciting a list that could drift"
		return panel
	}
	versions := make(map[string]string, len(direct)+len(indirect))
	for _, row := range append(append([]DepsRequireRow(nil), direct...), indirect...) {
		versions[row.Path] = row.Version
	}
	for _, path := range parseBlankImports(toolsContent) {
		// An anchor names a PACKAGE path; the require names its MODULE —
		// resolve the longest require that prefixes the anchor, exactly
		// how the go command maps imports onto modules.
		modulePath := depsResolveModulePath(path, versions)
		panel.Anchors = append(panel.Anchors, DepsRequireRow{
			Path:    path,
			Version: versions[modulePath],
			Linked:  linked[modulePath],
		})
	}
	panel.Note = "each anchor is a blank import behind the `tools` build tag: `go mod " +
		"tidy` scans imports across build configurations, so the anchor keeps the " +
		"require in go.mod even while no server package imports it yet — the pin " +
		"survives every tidy until real code absorbs it. Each row names the anchor's " +
		"PACKAGE path with its module's pinned version, and linked=true once the " +
		"serving binary imports that module (the test-only anchors stay unlinked " +
		"in the server binary by design)"
	return panel
}

// depsResolveModulePath maps an import path onto the require whose
// module path prefixes it — the longest (most specific) match wins,
// mirroring how the go command walks import paths up to a module root.
// An unmatched path resolves to itself, so the caller's version lookup
// honestly finds nothing.
func depsResolveModulePath(importPath string, requires map[string]string) string {
	best := ""
	for modulePath := range requires {
		if importPath == modulePath {
			return modulePath
		}
		if strings.HasPrefix(importPath, modulePath+"/") && len(modulePath) > len(best) {
			best = modulePath
		}
	}
	if best != "" {
		return best
	}
	return importPath
}

// buildDepsCachePanel states the repo's real cache wiring — the named
// volumes the dev compose stack mounts and the prod layer ordering.
func buildDepsCachePanel() DepsCachePanel {
	return DepsCachePanel{
		Volumes: []DepsCacheVolumeRow{
			{
				Name:  "gdg-go",
				Mount: "/go/pkg/mod",
				WhatItHolds: "the module cache: every downloaded module, unpacked and " +
					"read-only, content-addressed by path and version",
				WhyWarm: "a warm module cache means builds and govulncheck never " +
					"re-download a module the graph already required",
			},
			{
				Name:  "gdg-gocache",
				Mount: "/root/.cache/go-build",
				WhatItHolds: "the build cache: compiled package objects keyed by a " +
					"hash of sources, flags, and toolchain",
				WhyWarm: "an unchanged package recompiles to a cache hit, which is why " +
					"the repo's docker-exec verification loop stays seconds-fast",
			},
			{
				Name:  "gdg-npm",
				Mount: "/root/.npm",
				WhatItHolds: "the npm cache for the client half of the repo — the same " +
					"pattern carried across toolchains",
				WhyWarm: "client installs skip the network for anything the cache " +
					"already holds",
			},
		},
		LayerOrderNote: "the prod Dockerfile copies go.mod and go.sum FIRST and runs " +
			"`go mod download` as its own layer, so source churn invalidates only the " +
			"compile layer — the dependency layer rebuilds exactly when the manifests " +
			"change, which is what layer caching is for",
		ExternalNote: "the volumes are declared external and pre-created (`docker " +
			"volume create gdg-go gdg-gocache gdg-npm`): without external: true, " +
			"compose would prefix and create EMPTY twins, silently forcing full " +
			"dependency re-downloads",
	}
}

// linkedModuleSet indexes a build info's linked modules for the
// manifest-vs-binary cross-reference (nil-safe: no build info links
// nothing).
func linkedModuleSet(buildInfo *debug.BuildInfo) map[string]bool {
	linked := make(map[string]bool)
	if buildInfo == nil {
		return linked
	}
	for _, dep := range buildInfo.Deps {
		linked[dep.Path] = true
	}
	return linked
}

// markLinked stamps each require row with whether the serving binary
// links it, preserving the manifest's declared order.
func markLinked(rows []DepsRequireRow, linked map[string]bool) []DepsRequireRow {
	stamped := make([]DepsRequireRow, 0, len(rows))
	for _, row := range rows {
		row.Linked = linked[row.Path]
		stamped = append(stamped, row)
	}
	return stamped
}

// parseGoMod extracts the facts the manifest panel teaches from raw
// go.mod bytes: the module path, the go directive, the require entries
// split direct/indirect, and the tool directive entries. Hand-rolled on
// purpose — the stdlib ships no modfile parser, this module pins no
// third-party one for a demo, and the subset this panel needs (require
// blocks, single-line requires, the tool block, comment stripping) is
// exactly the grammar it implements. Order is preserved as written,
// which for a tidied go.mod is sorted.
func parseGoMod(content string) (modulePath, goDirective string, direct, indirect []DepsRequireRow, tools []string) {
	direct = make([]DepsRequireRow, 0)
	indirect = make([]DepsRequireRow, 0)
	tools = make([]string, 0)

	lines := strings.Split(content, "\n")
	block := "" // "", "require", "tool", or "skip" (replace/exclude/retract/godebug)
	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "//") {
			continue
		}
		code, isIndirect := stripGoModComment(line)
		if code == "" {
			continue
		}
		if block != "" {
			if code == ")" {
				block = ""
				continue
			}
			switch block {
			case "require":
				if path, version, ok := parseGoModRequire(code); ok {
					if isIndirect {
						indirect = append(indirect, DepsRequireRow{Path: path, Version: version})
					} else {
						direct = append(direct, DepsRequireRow{Path: path, Version: version})
					}
				}
			case "tool":
				if path := strings.Fields(code)[0]; path != "" {
					tools = append(tools, path)
				}
			}
			continue
		}
		switch {
		case strings.HasPrefix(code, "module "):
			modulePath = strings.TrimSpace(strings.TrimPrefix(code, "module "))
		case strings.HasPrefix(code, "go "):
			goDirective = strings.TrimSpace(strings.TrimPrefix(code, "go "))
		case code == "require (":
			block = "require"
		case code == "tool (":
			block = "tool"
		case strings.HasPrefix(code, "require "):
			if path, version, ok := parseGoModRequire(strings.TrimPrefix(code, "require ")); ok {
				if isIndirect {
					indirect = append(indirect, DepsRequireRow{Path: path, Version: version})
				} else {
					direct = append(direct, DepsRequireRow{Path: path, Version: version})
				}
			}
		case strings.HasPrefix(code, "replace ("), strings.HasPrefix(code, "exclude ("),
			strings.HasPrefix(code, "retract ("), strings.HasPrefix(code, "godebug ("):
			block = "skip"
		}
	}
	return modulePath, goDirective, direct, indirect, tools
}

// stripGoModComment splits a go.mod line into its code fragment and
// whether its trailing comment carries the `// indirect` marker — the
// flag that separates a require your code names from one only the
// module graph pulled in.
func stripGoModComment(line string) (code string, indirect bool) {
	if idx := strings.Index(line, "//"); idx >= 0 {
		comment := strings.TrimSpace(line[idx+2:])
		code = strings.TrimSpace(line[:idx])
		if comment == "indirect" || strings.HasPrefix(comment, "indirect ") {
			indirect = true
		}
		return code, indirect
	}
	return line, false
}

// parseGoModRequire splits `path version` (and tolerates a trailing
// pseudo-version or elision markers that make it unparseable — those
// rows are skipped rather than half-parsed).
func parseGoModRequire(entry string) (path, version string, ok bool) {
	fields := strings.Fields(entry)
	if len(fields) < 2 {
		return "", "", false
	}
	return fields[0], fields[1], true
}

// parseBlankImports collects the blank-import anchor paths from a
// tools file's content — lines of the form `_ "module/path"`, sorted
// for a deterministic panel.
func parseBlankImports(content string) []string {
	var paths []string
	for _, raw := range strings.Split(content, "\n") {
		line := strings.TrimSpace(raw)
		if !strings.HasPrefix(line, "_ ") {
			continue
		}
		quoted := strings.TrimSpace(strings.TrimPrefix(line, "_ "))
		if len(quoted) >= 2 && quoted[0] == '"' && quoted[len(quoted)-1] == '"' {
			if path := quoted[1 : len(quoted)-1]; path != "" {
				paths = append(paths, path)
			}
		}
	}
	sort.Strings(paths)
	return paths
}

// firstSentenceOrHonest keeps an unavailable-scan reason readable when
// the caller passed none: never an empty clause in a sentence that
// promises one.
func firstSentenceOrHonest(reason string) string {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return "no reason was recorded"
	}
	if idx := strings.Index(reason, "\n"); idx >= 0 {
		reason = reason[:idx]
	}
	return reason
}
