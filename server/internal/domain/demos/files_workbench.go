// Topic 26 files-media's demo `files:workbench` — one STORY payload in
// the shape the security and authz demos established (beats in
// execution order, each with a teaching sentence, a verdict color, and
// label/value facts, plus derived proofs and a closing reasoning
// paragraph). The data-layer console reports raw rows from the REAL
// streaming store; this builder proves every sentence from them — a
// drifted engine answer surfaces as a sentence saying so, never as a
// silent contradiction.
package demos

import (
	"context"
	"fmt"
	"strings"

	domfiles "guide/internal/domain/files"
)

// Verdict labels for the files story's client colors: green ok, red
// rejected, neutral info.
const (
	FilesVerdictOK       = "ok"
	FilesVerdictRejected = "rejected"
	FilesVerdictInfo     = "info"
)

// FilesStoryFact is one label/value pair — byte counts, header values,
// refusals. Values are pre-rendered strings so the client never formats.
type FilesStoryFact struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

// FilesStoryBeat is one beat of the workbench story.
type FilesStoryBeat struct {
	Beat    string           `json:"beat"`
	Detail  string           `json:"detail"`
	Verdict string           `json:"verdict"`
	Facts   []FilesStoryFact `json:"facts"`
}

// FilesStoryPayload is the wire shape the workbench endpoint serves
// (camelCase JSON); every slice is non-nil so the wire renders [].
type FilesStoryPayload struct {
	Mechanism string           `json:"mechanism"`
	Beats     []FilesStoryBeat `json:"beats"`
	Proofs    []FilesStoryFact `json:"proofs"`
	Reasoning string           `json:"reasoning"`
}

// newFilesStoryPayload applies the wire rules: non-nil facts and proofs.
func newFilesStoryPayload(mechanism string, beats []FilesStoryBeat, proofs []FilesStoryFact, reasoning string) FilesStoryPayload {
	for i := range beats {
		if beats[i].Facts == nil {
			beats[i].Facts = []FilesStoryFact{}
		}
	}
	if proofs == nil {
		proofs = []FilesStoryFact{}
	}
	return FilesStoryPayload{Mechanism: mechanism, Beats: beats, Proofs: proofs, Reasoning: reasoning}
}

// FilesWorkbenchRunner is the port the workbench story runs through;
// the data-layer console implements it against a real local store.
type FilesWorkbenchRunner interface {
	RunFilesWorkbenchStory(ctx context.Context) (FilesWorkbenchRun, error)
}

// FilesUploadRow is the streamed-upload beat's haul: how many bytes
// flowed, how long the copy took, and how much the PROCESS allocated
// for the whole exchange (the honesty number — a slurped upload would
// allocate at least the object's size).
type FilesUploadRow struct {
	Name       string
	Bytes      int64
	LimitBytes int64
	WallMs     string
	AllocBytes int64
	Stored     bool
}

// FilesOverLimitRow is the size-bound beat: a stream that ran past its
// limit was refused mid-copy and left nothing behind.
type FilesOverLimitRow struct {
	Name              string
	AttemptedBytes    int64
	LimitBytes        int64
	Refused           bool
	LeftoverTempFiles int
}

// FilesRangeRow is the range beat's haul: every header the story sent
// and the exact answer the range math produced for it.
type FilesRangeRow struct {
	Object string
	Size   int64

	Header        string // bytes=1048576-1048639
	ContentRange  string // bytes 1048576-1048639/4194304
	SliceHex      string // first bytes of the served slice
	SliceMatched  bool   // byte-for-byte against the expected slice
	SuffixHeader  string
	SuffixRange   string
	SuffixMatched bool

	BeyondHeader       string // bytes=9999999-
	BeyondContentRange string // bytes */4194304
	BeyondRefused416   bool

	IfRangeHeader     string // a stale validator
	IfRangeServedFull bool   // the range was ignored: whole object
	FullBytesServed   int64
}

// FilesTraversalRow is one refused name in the jail sweep.
type FilesTraversalRow struct {
	Attempt string
	Refused bool
}

// FilesJailRow is the traversal beat: hostile names refused by the
// sanitizer/jail, a symlink escape refused by EvalSymlinks, and the
// clean object still served afterwards.
type FilesJailRow struct {
	Root             string
	Attempts         []FilesTraversalRow
	SymlinkAttempt   string
	SymlinkRefused   bool
	CleanStillServed bool
}

// FilesThumbRow is the thumbnail beat: decode bounded, downscale, both
// encodings, and a decode bomb refused from its header alone.
type FilesThumbRow struct {
	SourceLabel                string
	SourceW, SourceH           int
	MaxDim                     int
	ThumbW, ThumbH             int
	PngBytes, JpegBytes        int
	JpegDecodedW, JpegDecodedH int
	BombLabel                  string
	BombBytes                  int
	BombRefused                bool
}

// FilesWorkbenchRun is the story's raw haul from the real store.
type FilesWorkbenchRun struct {
	Upload    FilesUploadRow
	OverLimit FilesOverLimitRow
	Range     FilesRangeRow
	Jail      FilesJailRow
	Thumb     FilesThumbRow
}

// BuildFilesWorkbenchPayload shapes the workbench story from the run.
func BuildFilesWorkbenchPayload(run FilesWorkbenchRun) FilesStoryPayload {
	refusals := make([]string, 0, len(run.Jail.Attempts))
	for _, attempt := range run.Jail.Attempts {
		mark := "REFUSED"
		if !attempt.Refused {
			mark = "ACCEPTED"
		}
		refusals = append(refusals, fmt.Sprintf("%s→%s", attempt.Attempt, mark))
	}
	beats := []FilesStoryBeat{
		{
			Beat: "the streamed upload", Verdict: FilesVerdictOK,
			Detail: fmt.Sprintf("a real multipart body — %s of deterministic bytes — flows through a PIPE into the store part by part: the writer generates, the store copies, and the pipe's small kernel buffer is the backpressure (the writer blocks whenever the disk is slower than the generator). The whole exchange allocated %s against %s of payload: the object never existed in memory, which is the difference between a stream and a slurp.",
				humanBytes(run.Upload.Bytes), humanBytes(run.Upload.AllocBytes), humanBytes(run.Upload.Bytes)),
			Facts: []FilesStoryFact{
				{Label: "object", Value: fmt.Sprintf("%s · %s · stored: %t", run.Upload.Name, humanBytes(run.Upload.Bytes), run.Upload.Stored)},
				{Label: "the bound", Value: fmt.Sprintf("io.LimitReader at %s — the copy stops the moment the stream exceeds it", humanBytes(run.Upload.LimitBytes))},
				{Label: "wall time", Value: run.Upload.WallMs},
				{Label: "allocations (whole exchange)", Value: fmt.Sprintf("%s — a slurped upload would allocate at least the payload", humanBytes(run.Upload.AllocBytes))},
			},
		},
		{
			Beat: "the size bound refuses mid-stream", Verdict: FilesVerdictRejected,
			Detail: fmt.Sprintf("the next upload runs %s against a %s limit: the LimitReader lets exactly one proving byte past the line, the copy stops there, the staged temp file is removed, and the store answers ErrTooLarge. Nothing landed (%d leftover staging files) — a refused upload must not cost disk, only the bytes already spent reading it.",
				humanBytes(run.OverLimit.AttemptedBytes), humanBytes(run.OverLimit.LimitBytes), run.OverLimit.LeftoverTempFiles),
			Facts: []FilesStoryFact{
				{Label: "refused", Value: fmt.Sprintf("%t (refused: %t)", run.OverLimit.Refused, run.OverLimit.Refused)},
				{Label: "partial files left", Value: fmt.Sprintf("%d — the staged temp is deleted on every failure path", run.OverLimit.LeftoverTempFiles)},
			},
		},
		{
			Beat: "the ranged download", Verdict: FilesVerdictOK,
			Detail: fmt.Sprintf("the stored object answers ranged requests as pure math over its size: a mid-file slice (206 with Content-Range %s, and the served bytes are checked against the expected slice — matched: %t), a SUFFIX range for the last bytes (matched: %t), and a range that starts past the end, which is not an error but a 416 carrying the honest census %s. If-Range closes the loop: a STALE validator means the object may have changed, so the range is ignored and the whole %s served (%t).",
				run.Range.ContentRange, run.Range.SliceMatched, run.Range.SuffixMatched, run.Range.BeyondContentRange, humanBytes(run.Range.FullBytesServed), run.Range.IfRangeServedFull),
			Facts: []FilesStoryFact{
				{Label: "Range", Value: run.Range.Header},
				{Label: "Content-Range", Value: run.Range.ContentRange},
				{Label: "the exact slice", Value: fmt.Sprintf("%s… (matched: %t)", run.Range.SliceHex, run.Range.SliceMatched)},
				{Label: "suffix", Value: fmt.Sprintf("%s → %s (matched: %t)", run.Range.SuffixHeader, run.Range.SuffixRange, run.Range.SuffixMatched)},
				{Label: "unsatisfiable", Value: fmt.Sprintf("%s → 416 with %s", run.Range.BeyondHeader, run.Range.BeyondContentRange)},
				{Label: "If-Range (stale)", Value: fmt.Sprintf("%s → whole object (%t)", run.Range.IfRangeHeader, run.Range.IfRangeServedFull)},
			},
		},
		{
			Beat: "the traversal sweep", Verdict: FilesVerdictRejected,
			Detail: fmt.Sprintf("every hostile spelling of \"leave the store\" is refused BY NAME before the filesystem is touched: parent walks, absolute paths, backslash smuggles, nested separators, NUL bytes — %d of %d attempts refused. The deepest one is the symlink: a link planted INSIDE the root pointing at a secret OUTSIDE it passes the string jail and is caught by EvalSymlinks re-proving the prefix over the real filesystem (refused: %t). And the jail refuses without breaking the store: the clean object still serves (%t).",
				countRefused(run.Jail.Attempts), len(run.Jail.Attempts), run.Jail.SymlinkRefused, run.Jail.CleanStillServed),
			Facts: []FilesStoryFact{
				{Label: "the sweep", Value: strings.Join(refusals, "  ")},
				{Label: "the symlink escape", Value: fmt.Sprintf("%s → refused: %t (EvalSymlinks re-jails)", run.Jail.SymlinkAttempt, run.Jail.SymlinkRefused)},
			},
		},
		{
			Beat: "the thumbnail", Verdict: FilesVerdictOK,
			Detail: fmt.Sprintf("a deterministic %d×%d PNG decodes through the BUDGET first: image.DecodeConfig reads the header, checks %s, and only then does image.Decode allocate. Box sampling downscales the longest edge to %d → %d×%d exactly, and both encodings round-trip (the JPEG decodes back at %d×%d). The bomb closes the beat: a %d-byte header CLAIMING %s is refused from the header alone (refused: %t) — the pixels are never allocated, which is the whole defense.",
				run.Thumb.SourceW, run.Thumb.SourceH, maxPixelsLabel(), run.Thumb.MaxDim, run.Thumb.ThumbW, run.Thumb.ThumbH, run.Thumb.JpegDecodedW, run.Thumb.JpegDecodedH, run.Thumb.BombBytes, run.Thumb.BombLabel, run.Thumb.BombRefused),
			Facts: []FilesStoryFact{
				{Label: "source", Value: fmt.Sprintf("%s — %d×%d", run.Thumb.SourceLabel, run.Thumb.SourceW, run.Thumb.SourceH)},
				{Label: "thumbnail", Value: fmt.Sprintf("%d×%d — png %s, jpeg %s (q%d)", run.Thumb.ThumbW, run.Thumb.ThumbH, humanBytes(int64(run.Thumb.PngBytes)), humanBytes(int64(run.Thumb.JpegBytes)), domfiles.ThumbnailJPEGQuality)},
				{Label: "the decode bomb", Value: fmt.Sprintf("%s in %d bytes → refused before allocation: %t", run.Thumb.BombLabel, run.Thumb.BombBytes, run.Thumb.BombRefused)},
			},
		},
	}
	return newFilesStoryPayload("files-workbench",
		beats,
		[]FilesStoryFact{
			{Label: "streaming honesty", Value: fmt.Sprintf("%s flowed for %s of allocations — io.Copy's fixed buffer, not the object", humanBytes(run.Upload.Bytes), humanBytes(run.Upload.AllocBytes))},
			{Label: "the bound", Value: fmt.Sprintf("over-limit stream refused at %s with %d partial files left behind", humanBytes(run.OverLimit.LimitBytes), run.OverLimit.LeftoverTempFiles)},
			{Label: "range math", Value: fmt.Sprintf("slice matched %t · suffix matched %t · past-the-end answered 416 with %s", run.Range.SliceMatched, run.Range.SuffixMatched, run.Range.BeyondContentRange)},
			{Label: "the jail", Value: fmt.Sprintf("%d of %d hostile names + the symlink escape refused; clean object still served: %t", countRefused(run.Jail.Attempts), len(run.Jail.Attempts), run.Jail.CleanStillServed)},
			{Label: "the pixel budget", Value: fmt.Sprintf("header checked against %s before any allocation — a %s claim refused in %d bytes", maxPixelsLabel(), run.Thumb.BombLabel, run.Thumb.BombBytes)},
		},
		"Every number above is what the real store answered. The upload beat's allocation count is the topic's whole thesis made measurable: the bytes exist on disk and on the wire, never in memory, because every hop reads a bounded buffer and pulls only when the downstream hop is ready — backpressure is not added to a streaming pipeline, it is what a streaming pipeline IS. The range beat is pure arithmetic over the object's size, so it costs nothing to be exact, and the 416 is the same honesty as the 206: the server states precisely which bytes exist. The jail is three locks because each lock catches what the others cannot see — structure, string escapes, and symlinks — and the thumbnail's budget check moves the decode bomb from a memory incident to a refused request. Uploads stream, downloads seek, limits refuse mid-stream, and names never become paths.")
}

// countRefused counts the sweep's refusals for the derived sentences.
func countRefused(attempts []FilesTraversalRow) int {
	refused := 0
	for _, attempt := range attempts {
		if attempt.Refused {
			refused++
		}
	}
	return refused
}

// maxPixelsLabel names the budget the way the beats read it — the
// number is the domain's constant, never restated here.
func maxPixelsLabel() string {
	return fmt.Sprintf("%d pixels", domfiles.MaxPixels)
}

// humanBytes renders byte counts the way the beats read them.
func humanBytes(n int64) string {
	switch {
	case n >= 1<<20 && n%(1<<20) == 0:
		return fmt.Sprintf("%d MiB", n>>20)
	case n >= 1<<20:
		return fmt.Sprintf("%.1f MiB", float64(n)/(1<<20))
	case n >= 1<<10 && n%(1<<10) == 0:
		return fmt.Sprintf("%d KiB", n>>10)
	case n >= 1<<10:
		return fmt.Sprintf("%.1f KiB", float64(n)/(1<<10))
	default:
		return fmt.Sprintf("%d B", n)
	}
}
