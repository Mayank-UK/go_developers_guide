// Topic 34 testing's live demo: the runner. The payload's spine is a
// REAL, bounded `go test -json` run over a small curated set of this
// repo's own test packages — one pure-logic package, one httptest
// package, one testcontainers package's ungated test — executed once at
// process boot in the background (the handler owns that single-flight,
// exactly like topic 31's govulncheck scan) so no request ever waits on
// a compiler. Everything served here is PARSED, never promised: the
// timeline's packages, tests, subtests, statuses, and durations come
// from the go tool's own JSON events; the suite-shape counts (test
// files, test packages, test functions, t.Parallel calls) are counted
// from the source tree on disk at request time; and the table-test
// examples are located by symbol, their presence verified against the
// very files the topic quotes. If the toolchain hiccups, the handler
// answers the typed 503 rather than inventing a green timeline.
package demos

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"regexp"
	"strings"
	"time"
)

// TestingRunnerPath is the demo area's endpoint — registered on the real
// mux by the composition root.
const TestingRunnerPath = "/api/demo/testing/runner"

// TestingRunTimeout bounds the curated run: compile time for three test
// binaries plus their execution, with a wide margin for a cold build
// cache. The handler's timeout kills the whole process group, so a
// wedged run cannot leak test binaries.
const TestingRunTimeout = 2 * time.Minute

// TestingRunFocus is the -run pattern naming the curated tests: two
// pure-logic tests (typed-error paths in the posts domain), two
// httptest tests (the wire contract plus a table of hostile bodies),
// and one test from the package that also holds the gated live-PG
// suite (its embedded-migrations pin, which needs no database).
const TestingRunFocus = "TestDraftValidateCollectsEveryIssue|" +
	"TestNormalizeListQueryCollectsEveryViolation|" +
	"TestPostsListServesTheSeedPage|" +
	"TestPostsCreateRefusesBodiesAtTheBoundary|" +
	"TestMigrationsEmbedTheExpectedFileSet"

// TestingRunPackages is the curated package set: pure logic, handlers
// through httptest, and the migrations embed — three tiers of the
// pyramid, one command.
var TestingRunPackages = []string{
	"./internal/domain/posts",
	"./internal/presentation",
	"./internal/data/db",
}

// TestingRunStates are the run's lifecycle values on the wire.
const (
	TestingRunStateRunning     = "running"
	TestingRunStateDone        = "done"
	TestingRunStateUnavailable = "unavailable"
)

// TestingTableExample locates one real table-driven test by symbol: the
// file path is repo-root-relative, and the shape counter verifies the
// symbol is still declared there — an example whose file moved is
// reported missing, never silently kept.
type TestingTableExample struct {
	Package string
	File    string
	Symbol  string
	Note    string
}

// TestingTableExamples is the curated trio the payload locates: the
// parallel config table, the hostile-bodies boundary table, and the
// trace-context edge table — all quoted in the topic's own sections.
var TestingTableExamples = []TestingTableExample{
	{
		Package: "guide/internal/core",
		File:    "internal/core/config_parsing_test.go",
		Symbol:  "TestLoadConfig",
		Note:    "the parallel exemplar: t.Parallel at BOTH levels — the table's subtests fan out across the test binary's P goroutines",
	},
	{
		Package: "guide/internal/presentation",
		File:    "internal/presentation/posts_test.go",
		Symbol:  "TestPostsCreateRefusesBodiesAtTheBoundary",
		Note:    "hostile bodies at the wire boundary: five cases under t.Run, every refusal pinned against the 422 envelope",
	},
	{
		Package: "guide/internal/domain/obs",
		File:    "internal/domain/obs/tracecontext_test.go",
		Symbol:  "TestParseTraceparentRejectsMalformedHeaders",
		Note:    "the spec's hostile edge table: one row per malformed traceparent, each rejection matched on the parser's own error text",
	},
}

// TestingPackageResult is one package's parsed outcome: the go tool's
// terminal verdict plus test-level counts and the package's own
// elapsed time, all from the JSON events.
type TestingPackageResult struct {
	Package    string  `json:"package"`
	Outcome    string  `json:"outcome"`
	Passed     int     `json:"passed"`
	Failed     int     `json:"failed"`
	Skipped    int     `json:"skipped"`
	DurationMs float64 `json:"durationMs"`
}

// TestingTestResult is one test (or subtest — Depth says which) from
// the run's timeline, in event order. Output is the bounded tail of the
// go tool's captured lines for FAILED tests only; passes carry none.
type TestingTestResult struct {
	Package    string  `json:"package"`
	Test       string  `json:"test"`
	Status     string  `json:"status"`
	Depth      int     `json:"depth"`
	DurationMs float64 `json:"durationMs"`
	Output     string  `json:"output,omitempty"`
}

// TestingRunEvents is the parsed event stream: package rows in
// first-seen order, test rows in first-run order, and the count (plus a
// bounded sample) of lines that were not JSON events.
type TestingRunEvents struct {
	Packages       []TestingPackageResult
	Tests          []TestingTestResult
	Unparsed       int
	UnparsedSample []string
}

// testEvent is one decoded line of `go test -json` output — the fields
// the parser needs from the go tool's event schema (everything else,
// like Time and OutputType, is deliberately ignored).
type testEvent struct {
	Action  string  `json:"Action"`
	Package string  `json:"Package"`
	Test    string  `json:"Test"`
	Elapsed float64 `json:"Elapsed"`
	Output  string  `json:"Output"`
}

// Per-test output is bounded: only failed tests keep any, and only the
// tail — the excerpt a human reads first when a timeline row is red.
const (
	testingOutputKeepPerTest = 8
	testingExcerptLines      = 4
)

// ParseGoTestEvents reduces a raw `go test -json` stream to the rows
// the payload serves. The rules, all observed from the real tool: a
// package appears with Action "start"; terminal package events
// (pass/fail/skip, no Test field) carry the package's elapsed seconds;
// test-level events carry Test names whose slashes denote subtests (the
// parent's own terminal event arrives AFTER its children); output
// lines ride alongside the test they belong to; and packages run in
// PARALLEL, so events from different packages interleave freely. Lines
// that are not JSON events (a build failure's diagnostics, a wedged
// tool's prose) are counted and sampled, never dropped silently.
func ParseGoTestEvents(raw string) TestingRunEvents {
	parsed := TestingRunEvents{}
	packageIndex := make(map[string]int)
	testIndex := make(map[string]int)

	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimRight(line, "\r")
		if strings.TrimSpace(line) == "" {
			continue
		}
		var event testEvent
		if err := json.Unmarshal([]byte(line), &event); err != nil || event.Package == "" {
			parsed.Unparsed++
			if len(parsed.UnparsedSample) < 4 {
				parsed.UnparsedSample = append(parsed.UnparsedSample, truncateForDisplay(line, 160))
			}
			continue
		}

		if event.Test == "" {
			applyPackageEvent(&parsed, packageIndex, event)
			continue
		}
		applyTestEvent(&parsed, testIndex, packageIndex, event)
	}
	return parsed
}

// applyPackageEvent folds a package-level event: "start" reserves the
// row (parallel packages interleave, so the row must exist before its
// terminal event), and pass/fail/skip stamps the verdict, converting
// the event's elapsed seconds to milliseconds.
func applyPackageEvent(parsed *TestingRunEvents, index map[string]int, event testEvent) {
	row, seen := index[event.Package]
	if !seen {
		parsed.Packages = append(parsed.Packages, TestingPackageResult{Package: event.Package})
		row = len(parsed.Packages) - 1
		index[event.Package] = row
	}
	switch event.Action {
	case "pass", "fail", "skip":
		parsed.Packages[row].Outcome = event.Action
		parsed.Packages[row].DurationMs = event.Elapsed * 1000
	}
}

// applyTestEvent folds a test-level event: "run" reserves the timeline
// row in first-seen order, terminal events stamp status and duration
// (and, for TOP-LEVEL tests, bump their package's counters — subtests
// belong to their parent's row, not the package totals), and output
// lines accumulate as a bounded tail on their row.
func applyTestEvent(parsed *TestingRunEvents, testIndex map[string]int, packageIndex map[string]int, event testEvent) {
	key := event.Package + "\x00" + event.Test
	row, seen := testIndex[key]
	if !seen {
		parsed.Tests = append(parsed.Tests, TestingTestResult{
			Package: event.Package,
			Test:    event.Test,
			Status:  "run",
			Depth:   strings.Count(event.Test, "/"),
		})
		row = len(parsed.Tests) - 1
		testIndex[key] = row
	}
	switch event.Action {
	case "pass", "fail", "skip":
		alreadyTerminal := parsed.Tests[row].Status == event.Action
		parsed.Tests[row].Status = event.Action
		parsed.Tests[row].DurationMs = event.Elapsed * 1000
		// A duplicated terminal event overwrites but must not
		// double-count into the package totals.
		if !alreadyTerminal && !strings.Contains(event.Test, "/") {
			pkgRow, pkgSeen := packageIndex[event.Package]
			if !pkgSeen {
				// Defensive: test events always follow their package's
				// "start" in practice, but the counters must not depend
				// on that ordering.
				parsed.Packages = append(parsed.Packages, TestingPackageResult{Package: event.Package})
				pkgRow = len(parsed.Packages) - 1
				packageIndex[event.Package] = pkgRow
			}
			switch event.Action {
			case "pass":
				parsed.Packages[pkgRow].Passed++
			case "fail":
				parsed.Packages[pkgRow].Failed++
			case "skip":
				parsed.Packages[pkgRow].Skipped++
			}
		}
	case "output":
		if trimmed := strings.TrimRight(event.Output, "\n"); trimmed != "" {
			current := parsed.Tests[row].Output
			lines := strings.Split(strings.TrimRight(current, "\n"), "\n")
			if current == "" {
				lines = nil
			}
			lines = append(lines, trimmed)
			if overflow := len(lines) - testingOutputKeepPerTest; overflow > 0 {
				lines = lines[overflow:]
			}
			parsed.Tests[row].Output = strings.Join(lines, "\n")
		}
	}
}

// truncateForDisplay bounds a sampled line so one pathological line
// cannot bloat the payload.
func truncateForDisplay(line string, max int) string {
	if len(line) <= max {
		return line
	}
	return line[:max] + "…"
}

// testingFuncPattern matches a top-level test declaration.
var testingFuncPattern = regexp.MustCompile(`(?m)^func (Test[A-Za-z0-9_]+)\(`)

// testingParallelPattern matches a t.Parallel() call on its own line.
var testingParallelPattern = regexp.MustCompile(`(?m)^\s*t\.Parallel\(\)`)

// TestingTableExampleRow is one curated table-test example with its
// presence verified against the tree at count time.
type TestingTableExampleRow struct {
	Package string `json:"package"`
	File    string `json:"file"`
	Symbol  string `json:"symbol"`
	Note    string `json:"note"`
	Present bool   `json:"present"`
}

// TestingSuiteShape is the suite's counted shape: files, packages,
// functions, parallel calls — all parsed from disk — plus the curated
// table-test examples with presence verified.
type TestingSuiteShape struct {
	TestFiles     int                      `json:"testFiles"`
	TestPackages  int                      `json:"testPackages"`
	TestFunctions int                      `json:"testFunctions"`
	ParallelCalls int                      `json:"parallelCalls"`
	TableExamples []TestingTableExampleRow `json:"tableExamples"`
	Provenance    string                   `json:"provenance"`
}

// CountSuiteShape walks a module tree (repo-root-relative paths) and
// counts the suite for real: every *_test.go file, the distinct
// packages that hold them, the test functions declared inside, and the
// t.Parallel calls — then verifies each curated table-test example is
// still declared where the topic says it is. Nothing here is recorded
// by hand; an empty tree counts as zero, honestly.
func CountSuiteShape(fsys fs.FS) TestingSuiteShape {
	shape := TestingSuiteShape{
		TableExamples: []TestingTableExampleRow{},
		Provenance: "every count above was parsed from the source tree beside the " +
			"process at request time (files walked, declarations matched); the " +
			"table-test examples are a curated trio whose presence is re-verified " +
			"against the same tree — no number in this panel was typed by hand",
	}
	packages := make(map[string]bool)
	_ = fs.WalkDir(fsys, ".", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return nil // a partial tree still counts what it can see
		}
		if entry.IsDir() || !strings.HasSuffix(path, "_test.go") {
			return nil
		}
		shape.TestFiles++
		dir := "."
		if slash := strings.LastIndex(path, "/"); slash >= 0 {
			dir = path[:slash]
		}
		packages[dir] = true

		raw, readErr := fs.ReadFile(fsys, path)
		if readErr != nil {
			return nil
		}
		source := string(raw)
		shape.TestFunctions += len(testingFuncPattern.FindAllString(source, -1))
		shape.ParallelCalls += len(testingParallelPattern.FindAllString(source, -1))
		return nil
	})
	shape.TestPackages = len(packages)

	for _, example := range TestingTableExamples {
		present := false
		if raw, err := fs.ReadFile(fsys, example.File); err == nil {
			present = strings.Contains(string(raw), "func "+example.Symbol+"(")
		}
		shape.TableExamples = append(shape.TableExamples, TestingTableExampleRow{
			Package: example.Package,
			File:    example.File,
			Symbol:  example.Symbol,
			Note:    example.Note,
			Present: present,
		})
	}
	return shape
}

// TestingRunOutcome is the handler's observation of one run attempt:
// lifecycle state, the exact command, wall-clock bounds, exit code, and
// the raw stdout the parser reduces. The handler fills it; the builder
// shapes it.
type TestingRunOutcome struct {
	State      string
	Reason     string
	Command    string
	StartedAt  time.Time
	FinishedAt time.Time
	ExitCode   int
	Stdout     string
}

// TestingRunPanel is the run's headline: state, command, timing, exit
// code, and the totals derived from the parsed events.
type TestingRunPanel struct {
	State        string  `json:"state"`
	Reason       string  `json:"reason,omitempty"`
	Command      string  `json:"command"`
	StartedAt    string  `json:"startedAt,omitempty"`
	FinishedAt   string  `json:"finishedAt,omitempty"`
	DurationMs   float64 `json:"durationMs"`
	AgeSeconds   float64 `json:"ageSeconds"`
	ExitCode     int     `json:"exitCode"`
	TotalPassed  int     `json:"totalPassed"`
	TotalFailed  int     `json:"totalFailed"`
	TotalSkipped int     `json:"totalSkipped"`
}

// TestingRunnerFacts is everything the payload derives from: the run's
// outcome (with the parser doing the reduction), the counted shape, and
// the clock the age is measured against.
type TestingRunnerFacts struct {
	Now   time.Time
	Run   TestingRunOutcome
	Shape TestingSuiteShape
}

// TestingRunnerPayload is the wire shape of GET /api/demo/testing/runner.
type TestingRunnerPayload struct {
	Mode      string                 `json:"mode"`
	Run       TestingRunPanel        `json:"run"`
	Packages  []TestingPackageResult `json:"packages"`
	Tests     []TestingTestResult    `json:"tests"`
	Shape     TestingSuiteShape      `json:"shape"`
	Reasoning string                 `json:"reasoning"`
}

// testingShapeProvenanceDefault labels a shape whose counter produced
// no note (a zero-value shape) so the panel never claims nothing.
const testingShapeProvenanceDefault = "the suite's shape was not counted for " +
	"this payload (a zero-value shape); when the source tree is reachable " +
	"every count is parsed from it at request time"

// BuildTestingRunnerPayload shapes the panels (pure): the run panel with
// totals derived from the events, the package and test timelines with
// failure excerpts kept and pass noise dropped, and the shape panel as
// counted. The mode line states honestly WHEN the run happened — a
// cached boot run served with its wall-clock age.
func BuildTestingRunnerPayload(facts TestingRunnerFacts) TestingRunnerPayload {
	events := ParseGoTestEvents(facts.Run.Stdout)

	panel := TestingRunPanel{
		State:    facts.Run.State,
		Reason:   facts.Run.Reason,
		Command:  facts.Run.Command,
		ExitCode: facts.Run.ExitCode,
	}
	if !facts.Run.StartedAt.IsZero() {
		panel.StartedAt = facts.Run.StartedAt.UTC().Format(time.RFC3339)
	}
	if !facts.Run.FinishedAt.IsZero() {
		panel.FinishedAt = facts.Run.FinishedAt.UTC().Format(time.RFC3339)
		panel.DurationMs = float64(facts.Run.FinishedAt.Sub(facts.Run.StartedAt).Microseconds()) / 1000
		panel.AgeSeconds = facts.Now.Sub(facts.Run.FinishedAt).Seconds()
	}

	tests := make([]TestingTestResult, 0, len(events.Tests))
	for _, test := range events.Tests {
		row := TestingTestResult{
			Package:    test.Package,
			Test:       test.Test,
			Status:     test.Status,
			Depth:      test.Depth,
			DurationMs: test.DurationMs,
		}
		if test.Status == "fail" && test.Output != "" {
			row.Output = excerptTail(test.Output, testingExcerptLines)
		}
		tests = append(tests, row)
	}

	for _, pkg := range events.Packages {
		panel.TotalPassed += pkg.Passed
		panel.TotalFailed += pkg.Failed
		panel.TotalSkipped += pkg.Skipped
	}

	shape := facts.Shape
	if strings.TrimSpace(shape.Provenance) == "" {
		shape.Provenance = testingShapeProvenanceDefault
	}

	return TestingRunnerPayload{
		Mode: "boot-cached: the curated run executes ONCE at process boot in the " +
			"background (bounded by a hard timeout that kills the whole process " +
			"group); every request serves the cached result with its wall-clock " +
			"age — no test ever executes on the request path",
		Run:      panel,
		Packages: events.Packages,
		Tests:    tests,
		Shape:    shape,
		Reasoning: "Every row above was parsed from the go tool's own -json events " +
			"or counted from the source tree — statuses, durations, subtests, and " +
			"totals included; nothing was typed by hand. The timeline's three " +
			"packages are three tiers of the pyramid in one command: pure domain " +
			"logic (typed-error paths over a fake repository), handlers through " +
			"httptest (the wire contract against the real chain), and the " +
			"migrations embed from the package that also holds the gated live-PG " +
			"suite. Reload the panel and the ages move but the numbers do not: " +
			"the run is a fact about boot, served with its provenance",
	}
}

// excerptTail keeps the LAST n lines of a bounded output buffer — the
// assertion lines a failing test prints last are the ones worth reading.
func excerptTail(output string, n int) string {
	lines := strings.Split(output, "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}

// TestingRunCommand renders the exact command the handler executes (and
// the payload serves verbatim) — quoting the focus pattern so the
// string a reader copies is the string the tool ran.
func TestingRunCommand(goBinary string) string {
	return fmt.Sprintf("%s test -json -count=1 -run '%s' %s",
		goBinary, TestingRunFocus, strings.Join(TestingRunPackages, " "))
}
