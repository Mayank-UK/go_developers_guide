package demos

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"
)

// osDirFSOrSkip hands back a real-tree FS rooted at the module root
// (walking up from the test's working directory until go.mod appears),
// skipping when the tree is not beside the binary (a compiled prod
// image).
func osDirFSOrSkip(t *testing.T) fs.FS {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Skipf("working directory unavailable: %v", err)
	}
	for dir := wd; ; dir = filepath.Dir(dir) {
		if _, statErr := os.Stat(filepath.Join(dir, "go.mod")); statErr == nil {
			return os.DirFS(dir)
		}
		if filepath.Dir(dir) == dir {
			t.Skipf("module tree not beside this binary (walked up from %s)", wd)
		}
	}
}

// The testing runner's pure halves, table-tested: the go-test-json
// parser (interleaved packages, subtests, skips, build failures,
// non-JSON noise, output bounding) and the suite-shape counter (a
// synthetic tree counted for real). The builder is proven against
// events captured from actual runs — every number in the payload must
// be derivable, none typed.

func TestParseGoTestEventsReducesARealShapedStream(t *testing.T) {
	t.Parallel()

	// Captured shape from the curated run (fields trimmed to the parser's
	// concern): two packages interleaved, one table test with subtests,
	// the parent's verdict arriving after its children.
	raw := strings.Join([]string{
		`{"Action":"start","Package":"guide/internal/domain/posts"}`,
		`{"Action":"run","Package":"guide/internal/domain/posts","Test":"TestDraftValidateCollectsEveryIssue"}`,
		`{"Action":"output","Package":"guide/internal/domain/posts","Test":"TestDraftValidateCollectsEveryIssue","Output":"=== RUN   TestDraftValidateCollectsEveryIssue\n"}`,
		`{"Action":"start","Package":"guide/internal/presentation"}`,
		`{"Action":"output","Package":"guide/internal/domain/posts","Test":"TestDraftValidateCollectsEveryIssue","Output":"--- PASS: TestDraftValidateCollectsEveryIssue (0.00s)\n"}`,
		`{"Action":"pass","Package":"guide/internal/domain/posts","Test":"TestDraftValidateCollectsEveryIssue","Elapsed":0}`,
		`{"Action":"output","Package":"guide/internal/domain/posts","Output":"PASS\n"}`,
		`{"Action":"output","Package":"guide/internal/domain/posts","Output":"ok  \tguide/internal/domain/posts\t0.007s\n"}`,
		`{"Action":"pass","Package":"guide/internal/domain/posts","Elapsed":0.007}`,
		`{"Action":"run","Package":"guide/internal/presentation","Test":"TestPostsCreateRefusesBodiesAtTheBoundary"}`,
		`{"Action":"run","Package":"guide/internal/presentation","Test":"TestPostsCreateRefusesBodiesAtTheBoundary/empty_required_fields"}`,
		`{"Action":"pass","Package":"guide/internal/presentation","Test":"TestPostsCreateRefusesBodiesAtTheBoundary/empty_required_fields","Elapsed":0}`,
		`{"Action":"run","Package":"guide/internal/presentation","Test":"TestPostsCreateRefusesBodiesAtTheBoundary/null_body"}`,
		`{"Action":"pass","Package":"guide/internal/presentation","Test":"TestPostsCreateRefusesBodiesAtTheBoundary/null_body","Elapsed":0.001}`,
		`{"Action":"pass","Package":"guide/internal/presentation","Test":"TestPostsCreateRefusesBodiesAtTheBoundary","Elapsed":0.002}`,
		`{"Action":"pass","Package":"guide/internal/presentation","Elapsed":0.31}`,
	}, "\n")

	events := ParseGoTestEvents(raw)

	if len(events.Packages) != 2 {
		t.Fatalf("packages = %d, want 2", len(events.Packages))
	}
	posts := events.Packages[0]
	if posts.Package != "guide/internal/domain/posts" || posts.Outcome != "pass" ||
		posts.Passed != 1 || posts.Failed != 0 || posts.DurationMs != 7 {
		t.Errorf("posts row = %+v, want pass / 1 passed / 7ms", posts)
	}
	presentation := events.Packages[1]
	if presentation.Outcome != "pass" || presentation.Passed != 1 ||
		presentation.DurationMs != 310 {
		t.Errorf("presentation row = %+v, want pass / 1 top-level passed / 310ms", presentation)
	}

	if len(events.Tests) != 4 {
		t.Fatalf("tests = %d, want 4 (one domain test, one parent, two subtests)", len(events.Tests))
	}
	if events.Tests[2].Depth != 1 || events.Tests[2].Test != "TestPostsCreateRefusesBodiesAtTheBoundary/empty_required_fields" {
		t.Errorf("subtest row = %+v, want depth 1 with the slashed name", events.Tests[2])
	}
	if events.Tests[1].Status != "pass" || events.Tests[1].DurationMs != 2 {
		t.Errorf("parent verdict = %+v, want pass at 2ms arriving after its children", events.Tests[1])
	}
	if events.Unparsed != 0 {
		t.Errorf("unparsed = %d, want 0", events.Unparsed)
	}
}

func TestParseGoTestEventsKeepsFailureOutputAsABoundedTail(t *testing.T) {
	t.Parallel()

	lines := []string{
		`{"Action":"start","Package":"p"}`,
		`{"Action":"run","Package":"p","Test":"TestRed"}`,
	}
	for i := range 12 {
		lines = append(lines, `{"Action":"output","Package":"p","Test":"TestRed","Output":"line `+
			string(rune('a'+i))+`\n"}`)
	}
	lines = append(lines,
		`{"Action":"output","Package":"p","Test":"TestRed","Output":"    red_test.go:9: assertion failed\n"}`,
		`{"Action":"fail","Package":"p","Test":"TestRed","Elapsed":0.5}`,
		`{"Action":"fail","Package":"p","Elapsed":0.5}`,
	)
	events := ParseGoTestEvents(strings.Join(lines, "\n"))

	if len(events.Tests) != 1 || events.Tests[0].Status != "fail" {
		t.Fatalf("test row = %+v, want one failed test", events.Tests)
	}
	outputLines := strings.Split(events.Tests[0].Output, "\n")
	if len(outputLines) != testingOutputKeepPerTest {
		t.Fatalf("kept output = %d lines, want the bound %d", len(outputLines), testingOutputKeepPerTest)
	}
	if outputLines[len(outputLines)-1] != "    red_test.go:9: assertion failed" {
		t.Errorf("the tail must keep the assertion line last: %q", outputLines[len(outputLines)-1])
	}
	if events.Packages[0].Failed != 1 {
		t.Errorf("package failed count = %d, want 1", events.Packages[0].Failed)
	}
}

func TestParseGoTestEventsHandlesSkipsBuildFailuresAndNoise(t *testing.T) {
	t.Parallel()

	raw := strings.Join([]string{
		// A package with no test files answers "skip".
		`{"Action":"skip","Package":"guide/pkg/empty","Elapsed":0}`,
		// A build failure: package-level output diagnostics, no test
		// events, terminal fail with no elapsed time worth reporting.
		`{"Action":"output","Package":"guide/pkg/broken","Output":"# guide/pkg/broken\n"}`,
		`{"Action":"output","Package":"guide/pkg/broken","Output":"broken.go:12:2: undefined: Missing\n"}`,
		`{"Action":"fail","Package":"guide/pkg/broken","Elapsed":0}`,
		// A skipped test (t.Skip), counted separately from failures.
		`{"Action":"start","Package":"guide/pkg/gated"}`,
		`{"Action":"output","Package":"guide/pkg/gated","Test":"TestGated","Output":"    gated_test.go:20: docker socket unavailable: skipping\n"}`,
		`{"Action":"skip","Package":"guide/pkg/gated","Test":"TestGated","Elapsed":0}`,
		`{"Action":"pass","Package":"guide/pkg/gated","Elapsed":0.01}`,
		// Non-JSON noise: counted and sampled, never fatal.
		"go: downloading go1.99.0 (map-kind noise the tool can emit)",
		"",
		`{"Action":"pause","Package":"guide/pkg/gated","Test":"TestGated"}`,
	}, "\n")

	events := ParseGoTestEvents(raw)

	if len(events.Packages) != 3 {
		t.Fatalf("packages = %d, want 3", len(events.Packages))
	}
	if events.Packages[0].Outcome != "skip" {
		t.Errorf("empty package outcome = %q, want skip", events.Packages[0].Outcome)
	}
	if events.Packages[1].Outcome != "fail" || events.Packages[1].Failed != 0 {
		t.Errorf("broken package = %+v, want fail with zero test-level failures (it never ran tests)", events.Packages[1])
	}
	if events.Packages[2].Outcome != "pass" || events.Packages[2].Skipped != 1 {
		t.Errorf("gated package = %+v, want pass with one skipped test", events.Packages[2])
	}
	if events.Unparsed != 1 || len(events.UnparsedSample) != 1 {
		t.Errorf("unparsed = %d sampled %d, want 1 and 1", events.Unparsed, len(events.UnparsedSample))
	}
}

func TestParseGoTestEventsToleratesDuplicateAndUnterminalEvents(t *testing.T) {
	t.Parallel()

	raw := strings.Join([]string{
		`{"Action":"start","Package":"p"}`,
		`{"Action":"run","Package":"p","Test":"TestA"}`,
		// A duplicate terminal event overwrites, not double-counts.
		`{"Action":"pass","Package":"p","Test":"TestA","Elapsed":0.1}`,
		`{"Action":"pass","Package":"p","Test":"TestA","Elapsed":0.1}`,
		// A started-but-never-finished package (a timeout kill mid-run):
		// the row exists with its counters, outcome honestly empty.
		`{"Action":"start","Package":"q"}`,
	}, "\n")

	events := ParseGoTestEvents(raw)

	if len(events.Packages) != 2 {
		t.Fatalf("packages = %d, want 2", len(events.Packages))
	}
	if events.Packages[0].Passed != 1 {
		t.Errorf("duplicated verdict counted %d passes, want 1", events.Packages[0].Passed)
	}
	if events.Packages[1].Outcome != "" {
		t.Errorf("unfinished package outcome = %q, want empty (unfinished), never guessed", events.Packages[1].Outcome)
	}
}

func TestCountSuiteShapeCountsASyntheticTree(t *testing.T) {
	t.Parallel()

	tree := fstest.MapFS{
		"internal/core/config_parsing_test.go": &fstest.MapFile{Data: []byte(
			"package core\n\nfunc TestLoadConfig(t *testing.T) {\n\tt.Parallel()\n}\n\nfunc TestOther(t *testing.T) {\n\tt.Parallel()\n}\n")},
		"internal/core/main.go": &fstest.MapFile{Data: []byte("package core\n\nfunc NotATest() {}\n")},
		"internal/posts/post_test.go": &fstest.MapFile{Data: []byte(
			"package posts\n\nfunc TestPost(t *testing.T) {\n\tt.Run(\"edge\", func(t *testing.T) {\n\t\tt.Parallel()\n\t})\n}\n")},
		"internal/posts/nested/deep_test.go": &fstest.MapFile{Data: []byte("package deep\n\nfunc TestDeep(t *testing.T) {}\n")},
	}

	shape := CountSuiteShape(tree)

	if shape.TestFiles != 3 || shape.TestPackages != 3 {
		t.Errorf("files/packages = %d/%d, want 3/3", shape.TestFiles, shape.TestPackages)
	}
	if shape.TestFunctions != 4 {
		t.Errorf("test functions = %d, want 4", shape.TestFunctions)
	}
	if shape.ParallelCalls != 3 {
		t.Errorf("parallel calls = %d, want 3 (one per t.Parallel() line)", shape.ParallelCalls)
	}
}

func TestCountSuiteShapeVerifiesTableExamplesAgainstTheTree(t *testing.T) {
	t.Parallel()

	tree := fstest.MapFS{
		"internal/core/config_parsing_test.go": &fstest.MapFile{Data: []byte(
			"package core\n\nfunc TestLoadConfig(t *testing.T) {}\n")},
	}

	shape := CountSuiteShape(tree)

	if len(shape.TableExamples) != len(TestingTableExamples) {
		t.Fatalf("table examples = %d, want the curated %d", len(shape.TableExamples), len(TestingTableExamples))
	}
	if !shape.TableExamples[0].Present {
		t.Errorf("TestLoadConfig must be found where it is declared: %+v", shape.TableExamples[0])
	}
	for _, row := range shape.TableExamples[1:] {
		if row.Present {
			t.Errorf("example %q must be reported missing from this synthetic tree", row.Symbol)
		}
	}
}

func TestCountSuiteShapeCountsTheRealModuleAndFindsTheTrio(t *testing.T) {
	t.Parallel()

	// The handler counts the real tree beside the process; this pin runs
	// the same walk from the demos package; "../../" is the server/
	// module root. If a curated example's file moves, this fails HERE
	// first — the payload would otherwise serve a missing example.
	shape := CountSuiteShape(osDirFSOrSkip(t))

	if shape.TestPackages < 20 || shape.TestFiles < 60 {
		t.Errorf("real suite counted %d packages / %d files — implausibly small; is the walk rooted right?", shape.TestPackages, shape.TestFiles)
	}
	if shape.TestFunctions < 300 || shape.ParallelCalls < 50 {
		t.Errorf("real suite counted %d functions / %d parallel calls — implausibly small", shape.TestFunctions, shape.ParallelCalls)
	}
	for _, row := range shape.TableExamples {
		if !row.Present {
			t.Errorf("curated table example %s is not declared at %s anymore — re-curate", row.Symbol, row.File)
		}
	}
}

func TestBuildTestingRunnerPayloadDerivesEveryNumber(t *testing.T) {
	t.Parallel()

	started := time.Date(2026, 10, 27, 9, 0, 0, 0, time.UTC)
	finished := started.Add(1500 * time.Millisecond)
	raw := strings.Join([]string{
		`{"Action":"start","Package":"p"}`,
		`{"Action":"run","Package":"p","Test":"TestA"}`,
		`{"Action":"pass","Package":"p","Test":"TestA","Elapsed":0.004}`,
		`{"Action":"pass","Package":"p","Elapsed":0.004}`,
	}, "\n")

	payload := BuildTestingRunnerPayload(TestingRunnerFacts{
		Now: finished.Add(10 * time.Second),
		Run: TestingRunOutcome{
			State:      TestingRunStateDone,
			Command:    "go test -json -count=1 -run '…' ./p",
			StartedAt:  started,
			FinishedAt: finished,
			ExitCode:   0,
			Stdout:     raw,
		},
		Shape: TestingSuiteShape{TestFiles: 1, TestPackages: 1, TestFunctions: 1,
			ParallelCalls: 0, TableExamples: []TestingTableExampleRow{}},
	})

	if payload.Run.State != "done" || payload.Run.ExitCode != 0 {
		t.Errorf("run panel = %+v", payload.Run)
	}
	if payload.Run.DurationMs != 1500 || payload.Run.AgeSeconds != 10 {
		t.Errorf("duration/age = %v/%v, want 1500ms and 10s", payload.Run.DurationMs, payload.Run.AgeSeconds)
	}
	if payload.Run.TotalPassed != 1 || payload.Run.TotalFailed != 0 || payload.Run.TotalSkipped != 0 {
		t.Errorf("totals = %+v, want 1/0/0 derived from events", payload.Run)
	}
	if len(payload.Packages) != 1 || payload.Packages[0].Passed != 1 {
		t.Errorf("package rows = %+v", payload.Packages)
	}
	if payload.Tests[0].Output != "" {
		t.Errorf("passing tests must not carry output noise: %q", payload.Tests[0].Output)
	}
	if !strings.Contains(payload.Mode, "boot-cached") {
		t.Errorf("mode must state the shipped mode honestly: %q", payload.Mode)
	}
	if !strings.Contains(payload.Shape.Provenance, "parsed from") {
		t.Errorf("shape provenance must label its provenance: %q", payload.Shape.Provenance)
	}
}

func TestBuildTestingRunnerPayloadReportsRunningAndUnavailableHonestly(t *testing.T) {
	t.Parallel()

	started := time.Date(2026, 10, 27, 9, 0, 0, 0, time.UTC)
	now := started.Add(3 * time.Second)

	running := BuildTestingRunnerPayload(TestingRunnerFacts{
		Now: now,
		Run: TestingRunOutcome{State: TestingRunStateRunning, Command: "go test …", StartedAt: started},
	})
	if running.Run.State != "running" || running.Run.DurationMs != 0 || len(running.Packages) != 0 {
		t.Errorf("running panel = %+v with %d package rows, want an honest empty timeline", running.Run, len(running.Packages))
	}

	unavailable := BuildTestingRunnerPayload(TestingRunnerFacts{
		Now: now,
		Run: TestingRunOutcome{State: TestingRunStateUnavailable, Reason: "the go toolchain is not on PATH"},
	})
	if unavailable.Run.State != "unavailable" || unavailable.Run.Reason == "" || len(unavailable.Packages) != 0 {
		t.Errorf("unavailable panel = %+v — degradation must be labeled, never faked", unavailable.Run)
	}
}

func TestTestingRunCommandIsTheExactCommand(t *testing.T) {
	t.Parallel()

	command := TestingRunCommand("go")
	for _, want := range []string{"go test -json -count=1", "-run '", TestingRunFocus} {
		if !strings.Contains(command, want) {
			t.Errorf("command %q must contain %q", command, want)
		}
	}
	for _, pkg := range TestingRunPackages {
		if !strings.Contains(command, pkg) {
			t.Errorf("command %q must name package %q", command, pkg)
		}
	}
}
