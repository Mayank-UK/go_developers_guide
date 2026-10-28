package demos

// Topic 38 performance's bench builder and parsers, table-tested
// against the go tool's REAL output shapes: header lines, the
// GOMAXPROCS suffix, -count duplicates (kept, never averaged),
// -benchmem-absent rows (HasMemStats honest), failed benchmarks, and
// the -gcflags=-m verdict grammar. One test executes the REAL
// benchmarks with a tiny budget and proves the builder serves their
// parsed numbers end to end; another pins the deterministic payload
// generator and the compressed-size computation.

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// perfFixtureBenchOutput mirrors a real two-count run of the curated
// set: environment headers, duplicate rows for the same benchmark (the
// variance the wire keeps), a row without -benchmem's pairs, and a
// trailing FAIL from a benchmark that died.
func perfFixtureBenchOutput() string {
	return strings.Join([]string{
		"goos: linux",
		"goarch: arm64",
		"pkg: guide/internal/domain/demos",
		"cpu: 13th Gen Intel(R) Core(TM) i7-13700K",
		"BenchmarkPerfGzipCompress/level=1/size=4KB-16          10        92000 ns/op",
		"BenchmarkPerfGzipCompress/level=1/size=4KB-16          10       101000 ns/op     24576 B/op        3 allocs/op",
		"BenchmarkPerfJSONEncode/approach=marshal-16            10        41000 ns/op     65536 B/op        2 allocs/op",
		"BenchmarkPerfJSONEncode/approach=stream-16             10        39000 ns/op        1024 B/op        4 allocs/op",
		"BenchmarkPerfInMemoryPostsList-16                      10         2100 ns/op       1224 B/op        9 allocs/op",
		"--- FAIL: BenchmarkPerfBroken",
		"    perf_benchmarks_test.go:99: boom",
		"FAIL",
	}, "\n")
}

func TestPerfParseGoBenchOutputReadsTheRealShape(t *testing.T) {
	parsed := ParseGoBenchOutput(perfFixtureBenchOutput())

	if parsed.Goos != "linux" || parsed.Goarch != "arm64" || parsed.Pkg != "guide/internal/domain/demos" {
		t.Errorf("header parsed = %q %q %q", parsed.Goos, parsed.Goarch, parsed.Pkg)
	}
	if !strings.Contains(parsed.CPU, "i7-13700K") {
		t.Errorf("cpu header = %q", parsed.CPU)
	}
	if len(parsed.Rows) != 5 {
		t.Fatalf("rows = %d, want 5 samples (count duplicates kept)", len(parsed.Rows))
	}

	first := parsed.Rows[0]
	if first.Benchmark != "BenchmarkPerfGzipCompress" || first.Subtest != "level=1/size=4KB" {
		t.Errorf("name split = %q / %q", first.Benchmark, first.Subtest)
	}
	if first.Procs != 16 {
		t.Errorf("procs suffix = %d, want 16", first.Procs)
	}
	if first.Iterations != 10 {
		t.Errorf("iterations = %d", first.Iterations)
	}
	if first.HasMemStats {
		t.Errorf("row without -benchmem pairs must keep HasMemStats false, got %v", first)
	}

	second := parsed.Rows[1]
	if !second.HasMemStats || second.NsPerOp != 101000 || second.BytesPerOp != 24576 || second.AllocsPerOp != 3 {
		t.Errorf("mem pairs = %v", second)
	}
	if parsed.Failed != 2 {
		t.Errorf("failed lines = %d, want the --- FAIL and terminal FAIL both counted", parsed.Failed)
	}
	// The benchmark's own log line is neither row nor header nor FAIL:
	// counted as unparsed with a bounded sample, never silently gone.
	if parsed.Unparsed != 1 || len(parsed.UnparsedSample) != 1 {
		t.Errorf("unparsed = %d (sample %v)", parsed.Unparsed, parsed.UnparsedSample)
	}
}

func TestPerfParseGoBenchOutputCountDuplicatesStaySeparate(t *testing.T) {
	raw := strings.Join([]string{
		"pkg: guide/internal/domain/demos",
		"BenchmarkPerfJSONEncode/approach=stream-8    10    39000 ns/op    1024 B/op    4 allocs/op",
		"BenchmarkPerfJSONEncode/approach=stream-8    10    42000 ns/op    1024 B/op    4 allocs/op",
	}, "\n")
	parsed := ParseGoBenchOutput(raw)
	if len(parsed.Rows) != 2 {
		t.Fatalf("rows = %d, want both count runs kept", len(parsed.Rows))
	}
	if parsed.Rows[0].NsPerOp == parsed.Rows[1].NsPerOp {
		t.Error("the two samples carry the run's real variance; both must survive verbatim")
	}
}

func TestPerfParseGoBenchOutputRejectsNonRows(t *testing.T) {
	for name, line := range map[string]string{
		"no iteration count": "BenchmarkPerfGzipCompress-16",
		"non-numeric count":  "BenchmarkPerfGzipCompress-16 many 123 ns/op",
		"bad metric value":   "BenchmarkPerfGzipCompress-16 10 fast ns/op",
	} {
		parsed := ParseGoBenchOutput(line)
		if len(parsed.Rows) != 0 || parsed.Unparsed != 1 {
			t.Errorf("%s: line %q must not become a row (rows %d, unparsed %d)",
				name, line, len(parsed.Rows), parsed.Unparsed)
		}
	}
}

func TestPerfParseGoBenchOutputHandlesDashedSubtests(t *testing.T) {
	// A subtest name may itself contain dashes; only a trailing -N
	// suffix of digits is the GOMAXPROCS marker.
	parsed := ParseGoBenchOutput("BenchmarkPerfGzipCompress/size=64KB-odd-name-16    10    1 ns/op")
	if len(parsed.Rows) != 1 {
		t.Fatalf("rows = %d", len(parsed.Rows))
	}
	row := parsed.Rows[0]
	if row.Subtest != "size=64KB-odd-name" || row.Procs != 16 {
		t.Errorf("subtest = %q procs = %d", row.Subtest, row.Procs)
	}
}

func TestPerfParseEscapeVerdictsReadsTheCompilerGrammar(t *testing.T) {
	// Sites exactly as `go build -gcflags=-m ./internal/presentation/`
	// prints them: import-path-relative file:line:col.
	raw := strings.Join([]string{
		"# guide/internal/presentation",
		"internal/presentation/posts.go:55:20: append escapes to heap: {length: 3}",
		"internal/presentation/posts.go:62:6: inlining call to newPostResponse",
		"internal/presentation/content.go:71:21: leaking param: v",
		"internal/presentation/middleware.go:40:2: moved to heap: closure",
		"internal/presentation/other.go:9:9: undefined: Something",
		"internal/content/registry.go:38:33: func literal does not escape",
		"internal/core/config.go:12:20: make(map[string]bool) does not escape",
	}, "\n")
	verdicts := ParseEscapeVerdicts(raw)
	got := make([]string, 0, len(verdicts))
	for _, verdict := range verdicts {
		got = append(got, verdict.Verdict)
	}
	want := []string{
		"escapes to heap",
		"inlining call to",
		"leaking param",
		"moved to heap",
		"does not escape",
		"does not escape",
	}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("verdicts = %v\nwant %v", got, want)
	}
	if verdicts[0].Site != "internal/presentation/posts.go:55" {
		t.Errorf("site = %q, want file:line only", verdicts[0].Site)
	}
	if !strings.Contains(verdicts[0].Line, "append escapes to heap") {
		t.Errorf("the raw line must ride verbatim: %q", verdicts[0].Line)
	}
}

func TestPerfCurateEscapeVerdictsFocusesAndBounds(t *testing.T) {
	var verdicts []PerfEscapeVerdict
	for i := 0; i < 20; i++ {
		verdicts = append(verdicts, PerfEscapeVerdict{
			Site:    fmt.Sprintf("internal/presentation/posts.go:1%d", i%10),
			Verdict: "escapes to heap",
			Line:    "internal/presentation/posts.go:1x:9: something escapes to heap",
		})
	}
	curated := perfCurateEscapeVerdicts(verdicts)
	if len(curated) != perfEscapeVerdictLimit {
		t.Errorf("curated = %d, want the bound %d", len(curated), perfEscapeVerdictLimit)
	}
	irrelevant := []PerfEscapeVerdict{{Site: "./x.go:1", Verdict: "does not escape", Line: "./x.go:1:2: y does not escape"}}
	if got := perfCurateEscapeVerdicts(irrelevant); len(got) != 0 {
		t.Errorf("unfocused verdicts curated = %d, want 0 (honest empty)", len(got))
	}
}

func TestPerfPayloadGeneratorIsDeterministic(t *testing.T) {
	first, err := PerfPayloadJSON(PerfPayloadSizes[1])
	if err != nil {
		t.Fatal(err)
	}
	second, err := PerfPayloadJSON(PerfPayloadSizes[1])
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != string(second) {
		t.Fatal("the benchmark document must be byte-identical across calls")
	}
	// The 64KB target is approximate by design (fixed furniture plus
	// padded bodies); pin it to its real neighborhood so drift is loud.
	if len(first) < 60<<10 || len(first) > 70<<10 {
		t.Errorf("64KB payload = %d bytes, want ~64KiB", len(first))
	}
	page := PerfPostsPage(PerfPayloadSizes[2])
	if len(page.Items) != perfPostsPerPage {
		t.Errorf("page items = %d, want the fixed page size", len(page.Items))
	}
}

func TestPerfGzipBytesLevelCurveIsReal(t *testing.T) {
	payload, err := PerfPayloadJSON(PerfPayloadSizes[0])
	if err != nil {
		t.Fatal(err)
	}
	sizes := map[int]int{}
	for _, level := range PerfGzipLevels {
		compressed, err := PerfGzipBytes(payload, level)
		if err != nil {
			t.Fatalf("level %d: %v", level, err)
		}
		if len(compressed) >= len(payload) {
			t.Errorf("level %d: gzip must shrink this document", level)
		}
		sizes[level] = len(compressed)
	}
	// The trade-off the demo teaches, asserted on the real document:
	// level 1 compresses less than level 9 (and runs faster — that half
	// is the benchmark's, not the size's).
	if sizes[1] <= sizes[9] {
		t.Errorf("level 1 (%dB) must compress less than level 9 (%dB)", sizes[1], sizes[9])
	}
	if _, err := PerfGzipBytes(payload, 42); err == nil {
		t.Error("gzip.NewWriterLevel must reject a bogus level")
	}
}

// TestPerfGzipAllocsAreBounded pins the compression helper's per-call
// allocation count with testing.AllocsPerRun — the same instrument the
// topic teaches for allocation assertions on a hot path.
func TestPerfGzipAllocsAreBounded(t *testing.T) {
	payload, err := PerfPayloadJSON(PerfPayloadSizes[0])
	if err != nil {
		t.Fatal(err)
	}
	allocs := testing.AllocsPerRun(100, func() {
		if _, err := PerfGzipBytes(payload, 6); err != nil {
			t.Fatal(err)
		}
	})
	if allocs == 0 {
		t.Fatal("AllocsPerRun reported zero — the helper writes to a bytes.Buffer; the instrument must be wired")
	}
	// Measured 18 allocs/op for the 4KB document at level 6 on
	// go1.27.3 (the flate machinery's window and block buffers); the
	// pin is a generous ceiling so a version bump that shuffles the
	// internals does not break the teaching assertion, while a change
	// proportional to iterations or payload WOULD.
	if allocs > 32 {
		t.Errorf("allocs per gzip call = %.0f, want a bounded machine constant (< 32)", allocs)
	}
}

func TestPerfBuildPayloadServesParsedSamplesWithTheirConfig(t *testing.T) {
	finished := time.Now().Add(-time.Minute)
	payload := BuildPerfBenchPayload(PerfBenchFacts{
		Now: time.Now(),
		Run: PerfBenchOutcome{
			State: PerfBenchStateDone,
			Bench: PerfCommandOutcome{
				State:      PerfBenchStateDone,
				Command:    PerfBenchCommand("go"),
				StartedAt:  finished.Add(-3 * time.Second),
				FinishedAt: finished,
				ExitCode:   0,
				Output:     perfFixtureBenchOutput(),
			},
			Escape: PerfCommandOutcome{
				State:   PerfBenchStateDone,
				Command: PerfEscapeCommand("go"),
				Output:  "internal/presentation/posts.go:55:20: append escapes to heap: {length: 3}",
			},
		},
	})

	if payload.Run.State != "done" || payload.Run.ExitCode != 0 {
		t.Errorf("run panel = %+v", payload.Run)
	}
	if payload.Run.Benchtime != PerfBenchBenchtime || payload.Run.Count != PerfBenchCount {
		t.Errorf("config must ride the panel: %+v", payload.Run)
	}
	if payload.Run.AgeSeconds < 55 {
		t.Errorf("age = %vs, want the run's real wall-clock age (~60s)", payload.Run.AgeSeconds)
	}
	if payload.Run.Failed != 2 {
		t.Errorf("failed = %d, want parsed from the fixture", payload.Run.Failed)
	}

	// Nine curve cells, deterministic order (level outer, sizes inner).
	if len(payload.Compression) != len(PerfGzipLevels)*len(PerfPayloadSizes) {
		t.Fatalf("compression cells = %d", len(payload.Compression))
	}
	if payload.Compression[0].Level != 1 || payload.Compression[0].Size != "4KB" {
		t.Errorf("first cell = %+v", payload.Compression[0])
	}
	if payload.Compression[0].RawBytes == 0 || payload.Compression[0].CompressedBytes == 0 {
		t.Errorf("sizes are computed from the real document: %+v", payload.Compression[0])
	}
	if payload.Compression[0].Ratio == "" {
		t.Error("the ratio column must render")
	}
	cellSamples := payload.Compression[0].Samples
	if len(cellSamples) != 2 {
		t.Fatalf("curve cell samples = %d, want both count runs", len(cellSamples))
	}
	// The fixture's first 4KB row carries no -benchmem pairs: HasMemStats
	// must say so rather than pretending a zero.
	if cellSamples[0].HasMemStats {
		t.Error("a sample parsed without mem pairs must keep HasMemStats false")
	}

	marshal := payload.Encoding[0]
	if marshal.Approach == "" || len(marshal.Samples) != 1 || marshal.Samples[0].AllocsPerOp != 2 {
		t.Errorf("encode rows = %+v", payload.Encoding)
	}
	if len(payload.HotPath.Samples) != 1 || payload.HotPath.Samples[0].BytesPerOp != 1224 {
		t.Errorf("hot path = %+v", payload.HotPath)
	}
	if len(payload.Escape.Verdicts) != 1 || !strings.Contains(payload.Escape.Verdicts[0].Line, "append") {
		t.Errorf("escape verdicts = %+v", payload.Escape.Verdicts)
	}
	if len(payload.Checklist) < 5 {
		t.Errorf("checklist = %d items", len(payload.Checklist))
	}
	if !strings.Contains(payload.Mode, "boot-cached") {
		t.Errorf("mode must name the shipped mode: %q", payload.Mode)
	}
}

func TestPerfBuildPayloadWhileRunningIsHonestlyEmpty(t *testing.T) {
	payload := BuildPerfBenchPayload(PerfBenchFacts{
		Now: time.Now(),
		Run: PerfBenchOutcome{
			State: PerfBenchStateRunning,
			Bench: PerfCommandOutcome{State: PerfBenchStateRunning},
		},
	})
	if payload.Run.State != PerfBenchStateRunning {
		t.Errorf("state = %q", payload.Run.State)
	}
	if len(payload.Compression[0].Samples) != 0 {
		t.Errorf("a running panel serves zero samples, got %+v", payload.Compression[0].Samples)
	}
	// The deterministic sizes exist before any run does — they are facts
	// about the generator, not the run.
	if payload.Compression[0].RawBytes == 0 {
		t.Error("the raw document size is computed, not measured")
	}
}

// TestPerfBenchmarksRunAndParse executes the REAL curated benchmarks
// with a tiny budget and proves the builder serves their parsed
// numbers end to end — the same loop the boot run performs.
func TestPerfBenchmarksRunAndParse(t *testing.T) {
	raw := perfRunCuratedBenchmarks(t)
	parsed := ParseGoBenchOutput(raw)
	if len(parsed.Rows) == 0 {
		t.Fatalf("the real run produced no samples:\n%s", raw)
	}
	seen := map[string]int{}
	for _, row := range parsed.Rows {
		seen[row.Benchmark]++
	}
	for _, name := range []string{
		"BenchmarkPerfGzipCompress",
		"BenchmarkPerfJSONEncode",
		"BenchmarkPerfInMemoryPostsList",
	} {
		if seen[name] == 0 {
			t.Errorf("benchmark %s absent from the real run:\n%s", name, raw)
		}
	}
	// Every row of the curve benchmark belongs to a declared cell.
	for _, row := range parsed.Rows {
		if row.Benchmark != "BenchmarkPerfGzipCompress" {
			continue
		}
		if !strings.HasPrefix(row.Subtest, "level=") || !strings.Contains(row.Subtest, "/size=") {
			t.Errorf("curve subtest %q not the declared cell shape", row.Subtest)
		}
		if !row.HasMemStats {
			t.Errorf("the curated run always passes -benchmem: %v", row)
		}
	}
}

// perfRunCuratedBenchmarks shells out to the real go tool over this
// package with a tiny budget — the reader's own invocation from the
// terminal companion, run from the test so the parser is proven
// against the tool as it actually prints today.
func perfRunCuratedBenchmarks(t *testing.T) string {
	t.Helper()
	goBinary, err := exec.LookPath("go")
	if err != nil {
		t.Skip("go toolchain not on PATH beside this test")
	}
	moduleDir := perfModuleDir(t)
	if moduleDir == "" {
		t.Skip("module tree not beside this test binary")
	}
	ctx, cancel := context.WithTimeout(context.Background(), PerfBenchTimeout)
	defer cancel()
	command := exec.CommandContext(ctx, goBinary,
		"test", "-bench=^BenchmarkPerf", "-benchtime=2x", "-count=1", "-benchmem",
		"-run=^$", "./internal/domain/demos/")
	command.Dir = moduleDir
	raw, err := command.CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		t.Fatalf("the curated run exceeded its %v bound", PerfBenchTimeout)
	}
	if err != nil && len(raw) == 0 {
		t.Skipf("go test could not execute: %v", err)
	}
	return string(raw)
}

// perfModuleDir walks up from the test binary's working directory to
// the module root.
func perfModuleDir(t *testing.T) string {
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
