// Topic 38 performance's live demo: the bench. The payload's numbers
// are all PARSED, never promised: a real, bounded `go test -bench` run
// over this package's own benchmark functions (compression across
// gzip levels × payload sizes, JSON buffered-marshal versus streaming
// encode, and the in-memory posts repository's List — the fake mode's
// actual serving hot path) executes ONCE at process boot in the
// background (the handler owns that single-flight, exactly like
// topic 31's govulncheck scan and topic 34's curated go-test run), and
// a second bounded `go build -gcflags=-m` over the presentation
// package supplies the escape-analysis verdicts verbatim from the
// compiler. Every sample carries its run configuration — the exact
// command, -benchtime, -count, wall-clock bounds, and age — and the
// count runs are served side by side, never averaged into one number:
// the variance between them is part of the teaching. If the toolchain
// or the source tree is missing, the handler answers the typed 503
// rather than rounding anything into lies.
package demos

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"guide/internal/domain/posts"
)

// PerfBenchPath is the demo area's endpoint — registered on the real
// mux by the composition root.
const PerfBenchPath = "/api/demo/perf/bench"

// PerfBenchTimeout bounds the boot bench run: compiling this package's
// test binary plus 10x-iteration samples of every curated benchmark,
// twice (count=2), with a wide margin for a cold build cache. The
// handler's timeout kills the whole process group, so a wedged run
// cannot leak compiler or test-binary children.
const PerfBenchTimeout = 2 * time.Minute

// PerfEscapeTimeout bounds the escape-analysis build: a gcflags change
// recompiles the presentation package even on a warm cache, so the
// bound covers one fresh compile plus its diagnostics.
const PerfEscapeTimeout = 2 * time.Minute

// The curated bench run's configuration, served beside every number —
// a number without its config is an anecdote, and the payload refuses
// to ship anecdotes.
const (
	// PerfBenchBenchtime is the -benchtime each sample runs: 10x means
	// exactly ten iterations per benchmark — small enough for a boot
	// run, honest enough to show alongside -count=2's two samples.
	PerfBenchBenchtime = "10x"
	// PerfBenchCount is -count: two samples per benchmark so the run's
	// own variance is visible on the wire.
	PerfBenchCount = 2
)

// PerfBenchStates are the run's lifecycle values on the wire.
const (
	PerfBenchStateRunning     = "running"
	PerfBenchStateDone        = "done"
	PerfBenchStateUnavailable = "unavailable"
)

// PerfGzipLevels are the compression levels the curve measures: the
// fast end (1), the default (6), and the maximum (9) — the three
// points between which every real gzip decision lives.
var PerfGzipLevels = []int{1, 6, 9}

// PerfPayloadSize names one deterministic payload size on the curve.
type PerfPayloadSize struct {
	Label       string
	TargetBytes int
}

// PerfPayloadSizes are the three payload sizes: a small API reply, a
// fat page, and a pathological whole-document dump — the curve's other
// axis.
var PerfPayloadSizes = []PerfPayloadSize{
	{Label: "4KB", TargetBytes: 4 << 10},
	{Label: "64KB", TargetBytes: 64 << 10},
	{Label: "512KB", TargetBytes: 512 << 10},
}

// perfFiller is the deterministic sentence the payload generator
// repeats — prose-like JSON body text, compressible the way real API
// payloads are, and ASCII-only so byte slicing can never cut a rune.
const perfFiller = "measure first, tune second, and let the numbers carry the argument. "

// perfPostsPerPage is the fixed page shape the generator emits.
const perfPostsPerPage = 32

// perfBodyText builds a deterministic body of exactly length bytes by
// repeating the filler sentence and slicing — ASCII-only text, so the
// slice boundary is safe.
func perfBodyText(length int) string {
	var builder strings.Builder
	builder.Grow(length)
	for builder.Len() < length {
		builder.WriteString(perfFiller)
	}
	return builder.String()[:length]
}

// PerfPostsPage builds the deterministic benchmark payload for one
// size: a real posts.PostPage of 32 posts whose bodies pad the marshaled
// document to the size's target. The page is the same domain type
// GET /api/posts serves, so the encode benchmarks measure the actual
// shape the API marshals — and the fixed dates keep every number
// reproducible.
func PerfPostsPage(size PerfPayloadSize) posts.PostPage {
	// Each post's fixed JSON furniture (ids, title, tags, timestamps)
	// costs roughly 180 bytes; the body carries the rest.
	bodyLength := size.TargetBytes/perfPostsPerPage - 180
	if bodyLength < 64 {
		bodyLength = 64
	}
	created := time.Date(2026, 2, 1, 8, 0, 0, 0, time.UTC)
	items := make([]posts.Post, 0, perfPostsPerPage)
	for i := 0; i < perfPostsPerPage; i++ {
		items = append(items, posts.Post{
			ID:        fmt.Sprintf("perf-%03d", i+1),
			Title:     fmt.Sprintf("Performance note %03d: allocations, compression, pooling", i+1),
			Body:      perfBodyText(bodyLength),
			Tags:      []string{"go", "performance", fmt.Sprintf("note-%d", i%8)},
			CreatedAt: created,
			UpdatedAt: created,
		})
	}
	return posts.PostPage{
		Items:      items,
		Page:       1,
		Limit:      perfPostsPerPage,
		TotalItems: perfPostsPerPage,
		TotalPages: 1,
	}
}

// PerfPayloadJSON marshals the deterministic page for one size — the
// exact bytes the compression benchmark and the compressed-size column
// share, so ns/op and the ratio are always about the same document.
func PerfPayloadJSON(size PerfPayloadSize) ([]byte, error) {
	raw, err := json.Marshal(PerfPostsPage(size))
	if err != nil {
		return nil, fmt.Errorf("perf: build %s payload: %w", size.Label, err)
	}
	return raw, nil
}

// PerfGzipBytes compresses payload at one gzip level — the single
// compression path the benchmarks, the compressed-size column, and any
// reader following along all share. Stdlib compress/gzip only; a level
// outside the package's range fails from gzip.NewWriterLevel itself.
func PerfGzipBytes(payload []byte, level int) ([]byte, error) {
	var buffer bytes.Buffer
	writer, err := gzip.NewWriterLevel(&buffer, level)
	if err != nil {
		return nil, fmt.Errorf("perf: gzip level %d: %w", level, err)
	}
	if _, err := writer.Write(payload); err != nil {
		return nil, fmt.Errorf("perf: gzip write: %w", err)
	}
	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("perf: gzip close: %w", err)
	}
	return buffer.Bytes(), nil
}

// ── the go test -bench output parser ────────────────────────────────

// PerfBenchRow is one measured line of `go test -bench` output: the
// benchmark's full name split into its top-level function and subtest
// path, the GOMAXPROCS the suffix carried, the iteration count, and
// the metric pairs the tool printed (ns/op always; B/op and allocs/op
// when -benchmem was on, which HasMemStats states honestly).
type PerfBenchRow struct {
	Name        string
	Benchmark   string
	Subtest     string
	Procs       int
	Iterations  uint64
	NsPerOp     float64
	BytesPerOp  float64
	AllocsPerOp float64
	HasMemStats bool
}

// PerfBenchParse is the reduced output: the environment header lines
// (goos/goarch/pkg/cpu) plus every benchmark row in print order — with
// -count=N, the same name appears N times and each occurrence is kept
// as its own row (samples are never averaged). Failure lines from a
// panicking benchmark are counted and sampled, and non-benchmark lines
// that are neither header nor row land in Unparsed, never silently
// dropped.
type PerfBenchParse struct {
	Goos           string
	Goarch         string
	Pkg            string
	CPU            string
	Rows           []PerfBenchRow
	Failed         int
	FailedSample   []string
	Unparsed       int
	UnparsedSample []string
}

// Bound on the sampled failure/unparsed lines: enough to diagnose, not
// enough to bloat a payload over one pathological run.
const perfSampleKeep = 4

// ParseGoBenchOutput reduces a raw `go test -bench` stream to its rows.
// The format, as the tool prints it: `goos:`/`goarch:`/`pkg:`/`cpu:`
// header lines; then one line per sample, `Name-PROCS  iters  value
// unit  [value unit …]` with tab-separated columns whose units are
// ns/op, B/op, and allocs/op (whitespace-split, so alignment never
// matters); `--- FAIL: BenchmarkX` and terminal `FAIL` lines when a
// benchmark dies. Metric pairs after the iteration count are consumed
// generically — a future unit is ignored rather than misparsed, and a
// row without -benchmem keeps HasMemStats false so the payload can
// render "—" instead of a lying zero.
func ParseGoBenchOutput(raw string) PerfBenchParse {
	parsed := PerfBenchParse{}
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimRight(line, "\r")
		trimmed := strings.TrimSpace(line)
		switch {
		case trimmed == "":
			continue
		case strings.HasPrefix(trimmed, "goos:"):
			parsed.Goos = strings.TrimSpace(strings.TrimPrefix(trimmed, "goos:"))
		case strings.HasPrefix(trimmed, "goarch:"):
			parsed.Goarch = strings.TrimSpace(strings.TrimPrefix(trimmed, "goarch:"))
		case strings.HasPrefix(trimmed, "pkg:"):
			parsed.Pkg = strings.TrimSpace(strings.TrimPrefix(trimmed, "pkg:"))
		case strings.HasPrefix(trimmed, "cpu:"):
			parsed.CPU = strings.TrimSpace(strings.TrimPrefix(trimmed, "cpu:"))
		case strings.HasPrefix(trimmed, "Benchmark"):
			if row, ok := perfParseBenchRow(trimmed); ok {
				parsed.Rows = append(parsed.Rows, row)
				continue
			}
			parsed.Unparsed++
			perfSample(&parsed.UnparsedSample, trimmed)
		case trimmed == "FAIL" || strings.HasPrefix(trimmed, "--- FAIL") || strings.HasPrefix(trimmed, "FAIL\t"):
			parsed.Failed++
			perfSample(&parsed.FailedSample, trimmed)
		default:
			parsed.Unparsed++
			perfSample(&parsed.UnparsedSample, trimmed)
		}
	}
	return parsed
}

// perfParseBenchRow parses one sample line. The shape is strict where
// it must be — a Benchmark-prefixed first token, then an integer
// iteration count, then value/unit pairs — and lenient where the tool
// has options: the GOMAXPROCS suffix (-N) is stripped only when the
// text after the last dash is all digits, and the subtest path after
// the first slash is kept verbatim (subtest names may themselves carry
// dashes).
func perfParseBenchRow(line string) (PerfBenchRow, bool) {
	fields := strings.Fields(line)
	if len(fields) < 2 {
		return PerfBenchRow{}, false
	}
	fullName := fields[0]
	row := PerfBenchRow{Procs: 1, Name: fullName}
	if dash := strings.LastIndex(fullName, "-"); dash > 0 {
		if procs, err := strconv.Atoi(fullName[dash+1:]); err == nil && procs > 0 {
			row.Procs = procs
			row.Name = fullName[:dash]
		}
	}
	if slash := strings.Index(row.Name, "/"); slash >= 0 {
		row.Benchmark = row.Name[:slash]
		row.Subtest = row.Name[slash+1:]
	} else {
		row.Benchmark = row.Name
	}
	iterations, err := strconv.ParseUint(fields[1], 10, 64)
	if err != nil {
		return PerfBenchRow{}, false
	}
	row.Iterations = iterations

	for i := 2; i+1 < len(fields); i += 2 {
		value, valueErr := strconv.ParseFloat(fields[i], 64)
		if valueErr != nil {
			return PerfBenchRow{}, false
		}
		switch fields[i+1] {
		case "ns/op":
			row.NsPerOp = value
		case "B/op":
			row.BytesPerOp = value
			row.HasMemStats = true
		case "allocs/op":
			row.AllocsPerOp = value
			row.HasMemStats = true
		}
	}
	return row, true
}

// perfSample appends one bounded diagnostic sample.
func perfSample(sample *[]string, line string) {
	if len(*sample) < perfSampleKeep {
		*sample = append(*sample, truncateForDisplay(line, 160))
	}
}

// ── the escape-analysis output parser ───────────────────────────────

// PerfEscapeVerdict is one compiler decision served verbatim: the
// file:line site, the verdict phrase (escapes to heap / does not
// escape / leaking param / inlining call to …), and the raw line the
// compiler printed — the payload never paraphrases a compiler.
type PerfEscapeVerdict struct {
	Site    string `json:"site"`
	Verdict string `json:"verdict"`
	Line    string `json:"line"`
}

// ParseEscapeVerdicts reduces `go build -gcflags=-m` output to its
// verdict lines: each carries a `file:line:col: message` prefix, and
// only the allocation-relevant messages are kept (escape decisions,
// leaking parameters, moved-to-heap, and inlining notes — inlining is
// performance teaching too). Everything else the build prints is not a
// verdict and is dropped here.
func ParseEscapeVerdicts(raw string) []PerfEscapeVerdict {
	var verdicts []PerfEscapeVerdict
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimRight(line, "\r")
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		site, message, ok := perfSplitDiagnostic(trimmed)
		if !ok {
			continue
		}
		verdict := perfVerdictPhrase(message)
		if verdict == "" {
			continue
		}
		verdicts = append(verdicts, PerfEscapeVerdict{
			Site:    site,
			Verdict: verdict,
			Line:    trimmed,
		})
	}
	return verdicts
}

// perfSplitDiagnostic splits one `file:line:col: message` compiler
// line, tolerating the -m output's occasional two-space padding. The
// site keeps file:line (the column is noise for teaching).
func perfSplitDiagnostic(line string) (site, message string, ok bool) {
	first := strings.IndexByte(line, ':')
	if first <= 0 {
		return "", "", false
	}
	second := strings.IndexByte(line[first+1:], ':')
	if second < 0 {
		return "", "", false
	}
	third := strings.IndexByte(line[first+1+second+1:], ':')
	if third < 0 {
		return "", "", false
	}
	site = line[:first+1+second]
	message = strings.TrimSpace(line[first+1+second+1+third+1:])
	if site == "" || message == "" {
		return "", "", false
	}
	return site, message, true
}

// perfVerdictPhrase extracts the teaching phrase from one diagnostic
// message, "" when the line is not an allocation-relevant decision.
func perfVerdictPhrase(message string) string {
	for _, phrase := range []string{
		"escapes to heap",
		"does not escape",
		"leaking param",
		"moved to heap",
		"inlining call to",
	} {
		if strings.Contains(message, phrase) {
			return phrase
		}
	}
	return ""
}

// perfEscapeFocus names the verdict lines the payload's allocs table
// curates — matched against the output as `go build -gcflags=-m
// ./internal/presentation/` actually prints it (import-path-relative
// sites). The posts wire-shaping lines (whose copies and slices the
// benchmark measures) and the handler chain's own encode path. A
// verdict matching none of these still parsed fine — it is simply not
// part of the curated table.
var perfEscapeFocus = []string{
	"presentation/posts.go",
	"presentation/content.go:67",
	"presentation/content.go:71",
	"newPostResponse",
	"newPostPageResponse",
}

// perfEscapeVerdictLimit bounds the curated table: enough for the
// story, never a dump.
const perfEscapeVerdictLimit = 8

// perfCurateEscapeVerdicts keeps the focus lines in print order, up to
// the limit — an empty result is honest (the build said nothing about
// the focus lines), never filled in from memory.
func perfCurateEscapeVerdicts(verdicts []PerfEscapeVerdict) []PerfEscapeVerdict {
	curated := []PerfEscapeVerdict{}
	for _, verdict := range verdicts {
		if len(curated) >= perfEscapeVerdictLimit {
			break
		}
		for _, focus := range perfEscapeFocus {
			if strings.Contains(verdict.Line, focus) {
				curated = append(curated, verdict)
				break
			}
		}
	}
	return curated
}

// ── run outcomes and the wire payload ───────────────────────────────

// PerfCommandOutcome is the handler's observation of one bounded
// command: its rendered form, wall-clock bounds, exit code, and the
// raw combined output the parsers reduce.
type PerfCommandOutcome struct {
	State      string
	Reason     string
	Command    string
	StartedAt  time.Time
	FinishedAt time.Time
	ExitCode   int
	Output     string
}

// PerfBenchOutcome is the boot run as a whole: the overall state plus
// the two commands' outcomes. The overall state follows the BENCH run
// (the escape build degrades inside its own panel, honestly, without
// taking the measured numbers down with it).
type PerfBenchOutcome struct {
	State  string
	Reason string
	Bench  PerfCommandOutcome
	Escape PerfCommandOutcome
}

// PerfBenchFacts carries the run's outcome and the clock the age is
// measured against into the pure builder.
type PerfBenchFacts struct {
	Now time.Time
	Run PerfBenchOutcome
}

// PerfRunPanel is the headline every number answers to: state, the
// exact command, the run's configuration and timing, the environment
// the tool reported, and the parsed failure count.
type PerfRunPanel struct {
	State      string  `json:"state"`
	Reason     string  `json:"reason,omitempty"`
	Command    string  `json:"command"`
	StartedAt  string  `json:"startedAt,omitempty"`
	FinishedAt string  `json:"finishedAt,omitempty"`
	DurationMs float64 `json:"durationMs"`
	AgeSeconds float64 `json:"ageSeconds"`
	ExitCode   int     `json:"exitCode"`
	Goos       string  `json:"goos,omitempty"`
	Goarch     string  `json:"goarch,omitempty"`
	CPU        string  `json:"cpu,omitempty"`
	Pkg        string  `json:"pkg,omitempty"`
	Benchtime  string  `json:"benchtime"`
	Count      int     `json:"count"`
	Failed     int     `json:"failedBenchmarks"`
}

// PerfSampleView is ONE count-run of one benchmark — iterations and
// the tool's metric pairs, exactly as parsed. Samples ride the wire
// side by side; nothing is averaged.
type PerfSampleView struct {
	Iterations  uint64  `json:"iterations"`
	NsPerOp     float64 `json:"nsPerOp"`
	BytesPerOp  float64 `json:"bytesPerOp"`
	AllocsPerOp float64 `json:"allocsPerOp"`
	HasMemStats bool    `json:"hasMemStats"`
}

// PerfCompressionRow is one cell of the measured curve: gzip level ×
// payload size, the deterministic document's raw and compressed sizes
// (computed from the very bytes the benchmark compressed) with the
// ratio, and every parsed sample beside them.
type PerfCompressionRow struct {
	Level           int              `json:"level"`
	Size            string           `json:"size"`
	RawBytes        int              `json:"rawBytes"`
	CompressedBytes int              `json:"compressedBytes"`
	Ratio           string           `json:"ratio"`
	Samples         []PerfSampleView `json:"samples"`
}

// PerfEncodeRow is one side of the encode comparison: the approach's
// name, its subtest key, a one-line note, and the parsed samples.
type PerfEncodeRow struct {
	Approach string           `json:"approach"`
	Subtest  string           `json:"subtest"`
	Note     string           `json:"note"`
	Samples  []PerfSampleView `json:"samples"`
}

// PerfHotPathRow is the allocation-focused benchmark of the fake
// mode's real serving path: the in-memory repository's List.
type PerfHotPathRow struct {
	Benchmark string           `json:"benchmark"`
	Note      string           `json:"note"`
	Samples   []PerfSampleView `json:"samples"`
}

// PerfEscapePanel is the compiler's say: the command, its state, and
// the curated verbatim verdicts (empty is honest, with the reason).
type PerfEscapePanel struct {
	Command  string              `json:"command"`
	State    string              `json:"state"`
	Reason   string              `json:"reason,omitempty"`
	Verdicts []PerfEscapeVerdict `json:"verdicts"`
}

// PerfBenchPayload is the wire shape of GET /api/demo/perf/bench.
type PerfBenchPayload struct {
	Mode        string               `json:"mode"`
	Run         PerfRunPanel         `json:"run"`
	Compression []PerfCompressionRow `json:"compression"`
	Encoding    []PerfEncodeRow      `json:"encoding"`
	HotPath     PerfHotPathRow       `json:"hotPath"`
	Escape      PerfEscapePanel      `json:"escape"`
	Checklist   []string             `json:"checklist"`
	Reasoning   string               `json:"reasoning"`
}

// perfChecklist is the pre-production pass the payload ships as data —
// the same list the topic teaches, each item "Lead — explanation."
var perfChecklist = []string{
	"Budgets — write the numbers down before you tune: p95 latency per endpoint, allocations per request, and a resident-memory ceiling; a budget you cannot recite is a wish.",
	"Profile — capture pprof profiles from the real binary under load (the debugging topic's /internal/pprof endpoints), and read the top of the flame before changing a line.",
	"Watch — /metrics and the request-duration histogram from the observability topic are the running verdict; a change that does not move them did not happen.",
	"Bench — A/B every optimization with go test -bench -count=10 and benchstat; a single run is an anecdote, and the delta must survive the noise.",
	"Load — rehearse production traffic before production: a load tool against the composed stack, with GOMEMLIMIT's degradation observed rather than assumed.",
}

// BuildPerfBenchPayload shapes the wire payload (pure): the run panel
// with its configuration and age, the compression curve with both
// count samples per cell and the deterministic raw/compressed sizes,
// the encode comparison, the hot-path row, the escape panel's verbatim
// verdicts, and the checklist. Row order is the generator's declared
// order (levels ascending, sizes in PerfPayloadSizes order) — not
// print order — so the wire shape is deterministic even though the
// numbers are whatever the run measured.
func BuildPerfBenchPayload(facts PerfBenchFacts) PerfBenchPayload {
	parsed := ParseGoBenchOutput(facts.Run.Bench.Output)

	panel := PerfRunPanel{
		State:     facts.Run.State,
		Reason:    facts.Run.Reason,
		Command:   facts.Run.Bench.Command,
		ExitCode:  facts.Run.Bench.ExitCode,
		Goos:      parsed.Goos,
		Goarch:    parsed.Goarch,
		CPU:       parsed.CPU,
		Pkg:       parsed.Pkg,
		Benchtime: PerfBenchBenchtime,
		Count:     PerfBenchCount,
		Failed:    parsed.Failed,
	}
	if !facts.Run.Bench.StartedAt.IsZero() {
		panel.StartedAt = facts.Run.Bench.StartedAt.UTC().Format(time.RFC3339)
	}
	if !facts.Run.Bench.FinishedAt.IsZero() {
		panel.FinishedAt = facts.Run.Bench.FinishedAt.UTC().Format(time.RFC3339)
		panel.DurationMs = float64(facts.Run.Bench.FinishedAt.Sub(facts.Run.Bench.StartedAt).Microseconds()) / 1000
		panel.AgeSeconds = facts.Now.Sub(facts.Run.Bench.FinishedAt).Seconds()
	}

	payload := PerfBenchPayload{
		Mode: "boot-cached: the curated benchmarks execute ONCE at process boot " +
			"in the background (bounded by a hard timeout that kills the whole " +
			"process group); every request serves the parsed samples with the " +
			"exact command, run configuration, and wall-clock age — the count " +
			"samples ride side by side, never averaged",
		Run: panel,
		Escape: PerfEscapePanel{
			Command:  facts.Run.Escape.Command,
			State:    facts.Run.Escape.State,
			Reason:   facts.Run.Escape.Reason,
			Verdicts: perfCurateEscapeVerdicts(ParseEscapeVerdicts(facts.Run.Escape.Output)),
		},
		Encoding: []PerfEncodeRow{
			{
				Approach: "buffered: json.Marshal",
				Subtest:  "approach=marshal",
				Note: "serializes into one whole-document allocation, then hands " +
					"the caller the buffer — the copy the caller usually immediately writes out",
				Samples: perfSamplesFor(parsed.Rows, "BenchmarkPerfJSONEncode", "approach=marshal"),
			},
			{
				Approach: "streaming: json.NewEncoder(w).Encode",
				Subtest:  "approach=stream",
				Note: "the shape writeJSON uses on every response: the encoder " +
					"writes straight to the writer, so no whole-document buffer is returned",
				Samples: perfSamplesFor(parsed.Rows, "BenchmarkPerfJSONEncode", "approach=stream"),
			},
		},
		HotPath: PerfHotPathRow{
			Benchmark: "BenchmarkPerfInMemoryPostsList",
			Note: "the fake mode's REAL serving path: List copies every post it " +
				"returns (the defensive-copy discipline), so -benchmem states the " +
				"per-request allocation tax in bytes and counts",
			Samples: perfSamplesFor(parsed.Rows, "BenchmarkPerfInMemoryPostsList", ""),
		},
		Checklist: perfChecklist,
		Reasoning: "Every number above was parsed from the go tool's own output — " +
			"the samples from `go test -bench -benchmem`, the verdicts verbatim " +
			"from `go build -gcflags=-m` — with the exact command, -benchtime, " +
			"-count, and the run's wall-clock age beside them; the two samples " +
			"per benchmark are shown side by side because the distance between " +
			"them IS the noise floor, and the compressed sizes are computed from " +
			"the same deterministic document the benchmark compressed. Reload " +
			"and the age moves but no number does: the run is a fact about boot, " +
			"served with its provenance — copy the run command in the terminal " +
			"companion to measure it yourself with real -benchtime and count",
	}

	for _, level := range PerfGzipLevels {
		for _, size := range PerfPayloadSizes {
			row := PerfCompressionRow{Level: level, Size: size.Label}
			if raw, err := PerfPayloadJSON(size); err == nil {
				row.RawBytes = len(raw)
				if compressed, gzipErr := PerfGzipBytes(raw, level); gzipErr == nil {
					row.CompressedBytes = len(compressed)
					if row.CompressedBytes > 0 {
						row.Ratio = fmt.Sprintf("%.1fx", float64(row.RawBytes)/float64(row.CompressedBytes))
					}
				}
			}
			row.Samples = perfSamplesFor(parsed.Rows, "BenchmarkPerfGzipCompress",
				fmt.Sprintf("level=%d/size=%s", level, size.Label))
			payload.Compression = append(payload.Compression, row)
		}
	}
	return payload
}

// perfSamplesFor collects the parsed samples of one benchmark subtest
// in print order — every count run, never averaged.
func perfSamplesFor(rows []PerfBenchRow, benchmark, subtest string) []PerfSampleView {
	samples := []PerfSampleView{}
	for _, row := range rows {
		if row.Benchmark != benchmark || row.Subtest != subtest {
			continue
		}
		samples = append(samples, PerfSampleView{
			Iterations:  row.Iterations,
			NsPerOp:     row.NsPerOp,
			BytesPerOp:  row.BytesPerOp,
			AllocsPerOp: row.AllocsPerOp,
			HasMemStats: row.HasMemStats,
		})
	}
	return samples
}

// PerfBenchCommand renders the exact bench command the handler
// executes (and the payload serves verbatim).
func PerfBenchCommand(goBinary string) string {
	return fmt.Sprintf(
		"%s test -bench=^BenchmarkPerf -benchtime=%s -count=%d -benchmem -run='^$' ./internal/domain/demos/",
		goBinary, PerfBenchBenchtime, PerfBenchCount)
}

// PerfEscapeCommand renders the escape-analysis command the handler
// executes: -gcflags=-m makes the compiler print its allocation
// decisions, and the pattern-less gcflags apply only to the package
// named on the command line.
func PerfEscapeCommand(goBinary string) string {
	return goBinary + " build -gcflags=-m ./internal/presentation/"
}
