package demos

import (
	"strings"
	"testing"
)

// The files:workbench payload builder: every sentence derives from the
// run's own rows — byte counts, header values, and refusal verdicts
// appear only as the run reported them, and the five beats are pinned
// in execution order.

func TestFilesWorkbenchPayloadDerivesFromTheRun(t *testing.T) {
	run := FilesWorkbenchRun{
		Upload: FilesUploadRow{
			Name: "workbench.bin", Bytes: 4 << 20, LimitBytes: 8 << 20,
			WallMs: "12.3 ms", AllocBytes: 131_072, Stored: true,
		},
		OverLimit: FilesOverLimitRow{
			Name: "toolarge.bin", AttemptedBytes: 1_572_864, LimitBytes: 1 << 20,
			Refused: true, LeftoverTempFiles: 0,
		},
		Range: FilesRangeRow{
			Object: "workbench.bin", Size: 4 << 20,
			Header: "bytes=1048576-1048639", ContentRange: "bytes 1048576-1048639/4194304",
			SliceHex: "43 0e 5d c4 8a 11 58 d1", SliceMatched: true,
			SuffixHeader: "bytes=-64", SuffixRange: "bytes 4194240-4194303/4194304", SuffixMatched: true,
			BeyondHeader: "bytes=9999999-", BeyondContentRange: "bytes */4194304", BeyondRefused416: true,
			IfRangeHeader: `"stale-never-matches"`, IfRangeServedFull: true, FullBytesServed: 4 << 20,
		},
		Jail: FilesJailRow{
			Root: "/tmp/guide-files-story-0",
			Attempts: []FilesTraversalRow{
				{Attempt: "../escape.txt", Refused: true},
				{Attempt: "/etc/passwd", Refused: true},
				{Attempt: "..\\windows.txt", Refused: true},
				{Attempt: "nested/dir.txt", Refused: true},
				{Attempt: "nul\x00.txt", Refused: true},
				{Attempt: "..", Refused: true},
			},
			SymlinkAttempt:   "leak.txt → (symlink outside the root)",
			SymlinkRefused:   true,
			CleanStillServed: true,
		},
		Thumb: FilesThumbRow{
			SourceLabel: "in-process PNG fixture (decoded as png)", SourceW: 320, SourceH: 200,
			MaxDim: 64, ThumbW: 64, ThumbH: 40,
			PngBytes: 2100, JpegBytes: 1500, JpegDecodedW: 64, JpegDecodedH: 40,
			BombLabel: "30000×30000", BombBytes: 53, BombRefused: true,
		},
	}
	payload := BuildFilesWorkbenchPayload(run)

	if payload.Mechanism != "files-workbench" {
		t.Errorf("mechanism = %q", payload.Mechanism)
	}
	if len(payload.Beats) != 5 {
		t.Fatalf("beats = %d, want 5 (upload, bound, range, jail, thumb)", len(payload.Beats))
	}
	wants := []struct{ beat, fragment string }{
		{"the streamed upload", "4 MiB of deterministic bytes"},
		{"the size bound refuses mid-stream", "ErrTooLarge"},
		{"the ranged download", "bytes 1048576-1048639/4194304"},
		{"the traversal sweep", "EvalSymlinks re-proving"},
		{"the thumbnail", "24000000 pixels"},
	}
	for i, want := range wants {
		if payload.Beats[i].Beat != want.beat {
			t.Errorf("beat %d = %q, want %q", i, payload.Beats[i].Beat, want.beat)
		}
		if !strings.Contains(payload.Beats[i].Detail, want.fragment) {
			t.Errorf("beat %q must derive %q from the run: %s", want.beat, want.fragment, payload.Beats[i].Detail)
		}
	}
	verdicts := []string{FilesVerdictOK, FilesVerdictRejected, FilesVerdictOK, FilesVerdictRejected, FilesVerdictOK}
	for i, verdict := range verdicts {
		if payload.Beats[i].Verdict != verdict {
			t.Errorf("beat %d verdict = %q, want %q", i, payload.Beats[i].Verdict, verdict)
		}
	}
	if !strings.Contains(payload.Beats[0].Detail, "never existed in memory") {
		t.Errorf("upload beat must state the streaming thesis: %s", payload.Beats[0].Detail)
	}
	if !strings.Contains(payload.Beats[3].Facts[0].Value, "../escape.txt→REFUSED") {
		t.Errorf("sweep fact must render every attempt with its verdict: %s", payload.Beats[3].Facts[0].Value)
	}
	if !strings.Contains(payload.Beats[4].Facts[2].Value, "30000×30000") {
		t.Errorf("thumb facts must name the bomb: %s", payload.Beats[4].Facts[2].Value)
	}
	for _, proof := range payload.Proofs {
		if proof.Label == "" || proof.Value == "" {
			t.Errorf("empty proof row: %+v", proof)
		}
	}
	if !strings.Contains(payload.Reasoning, "backpressure") {
		t.Errorf("reasoning must close on backpressure: %s", payload.Reasoning)
	}
}

func TestFilesWorkbenchPayloadDriftSurfacesAsHonestSentences(t *testing.T) {
	// A drifted run (a matched slice that did NOT match, an escape that
	// was NOT refused) must render sentences that SAY so — never a
	// silent contradiction with the facts beside them.
	run := FilesWorkbenchRun{
		Upload:    FilesUploadRow{Name: "x.bin", Bytes: 10, LimitBytes: 20, Stored: false},
		OverLimit: FilesOverLimitRow{AttemptedBytes: 30, LimitBytes: 20, Refused: false, LeftoverTempFiles: 2},
		Range: FilesRangeRow{
			SliceHex: "00", SliceMatched: false, SuffixMatched: false,
			BeyondContentRange: "bytes */10", BeyondRefused416: false,
			IfRangeServedFull: false,
		},
		Jail: FilesJailRow{
			Attempts:         []FilesTraversalRow{{Attempt: "../escape", Refused: false}},
			SymlinkRefused:   false,
			CleanStillServed: false,
		},
		Thumb: FilesThumbRow{BombRefused: false, BombLabel: "30000×30000", BombBytes: 53},
	}
	payload := BuildFilesWorkbenchPayload(run)
	if !strings.Contains(payload.Beats[2].Detail, "matched: false") {
		t.Errorf("range beat must state the drifted slice verdict: %s", payload.Beats[2].Detail)
	}
	if !strings.Contains(payload.Beats[3].Detail, "0 of 1 attempts refused") {
		t.Errorf("jail beat must count refusals honestly: %s", payload.Beats[3].Detail)
	}
	if !strings.Contains(payload.Beats[3].Detail, "refused: false") {
		t.Errorf("jail beat must state the drifted symlink verdict: %s", payload.Beats[3].Detail)
	}
	if !strings.Contains(payload.Beats[4].Detail, "refused: false") {
		t.Errorf("thumb beat must state the drifted bomb verdict: %s", payload.Beats[4].Detail)
	}
}

func TestFilesWorkbenchPayloadNeverMarshalsNilSlices(t *testing.T) {
	payload := BuildFilesWorkbenchPayload(FilesWorkbenchRun{
		Jail: FilesJailRow{Attempts: []FilesTraversalRow{{Attempt: "x", Refused: true}}},
	})
	for i, beat := range payload.Beats {
		if beat.Facts == nil {
			t.Errorf("beat %d (%q) marshaled nil facts", i, beat.Beat)
		}
	}
	if payload.Proofs == nil {
		t.Error("proofs marshaled nil")
	}
}
