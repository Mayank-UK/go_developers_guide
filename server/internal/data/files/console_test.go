package files

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"guide/internal/domain/demos"
)

// The workbench story against the REAL streaming store: one run walks
// all five beats, and every claim the demo's sentences make is asserted
// here first — the upload landed whole, the over-limit refusal left
// nothing behind, the range math matched the deterministic bytes, every
// hostile name (and the symlink) was refused while the clean object
// kept serving, and the decode bomb died at its header.

func TestFilesWorkbenchStoryIsTrueAgainstARealStore(t *testing.T) {
	console, err := NewConsole()
	if err != nil {
		t.Fatalf("console: %v", err)
	}
	run, err := console.RunFilesWorkbenchStory(context.Background())
	if err != nil {
		t.Fatalf("story: %v", err)
	}

	// Beat 1: the streamed upload landed whole, well under the limit,
	// and the exchange allocated far less than the payload — a slurp
	// would have allocated at least the object's size.
	if !run.Upload.Stored || run.Upload.Bytes != storyUploadBytes {
		t.Errorf("upload = %+v, want %d bytes stored", run.Upload, storyUploadBytes)
	}
	if run.Upload.LimitBytes != DefaultUploadLimit {
		t.Errorf("story limit = %d, want the console's %d", run.Upload.LimitBytes, DefaultUploadLimit)
	}
	if run.Upload.AllocBytes >= run.Upload.Bytes {
		t.Errorf("allocations %d for a %d-byte payload look like a slurp, not a stream",
			run.Upload.AllocBytes, run.Upload.Bytes)
	}

	// Beat 2: the over-limit stream was refused mid-copy with zero
	// partial files left behind.
	if !run.OverLimit.Refused {
		t.Error("the over-limit upload must be refused")
	}
	if run.OverLimit.LeftoverTempFiles != 0 {
		t.Errorf("%d staging files survived the refusal", run.OverLimit.LeftoverTempFiles)
	}

	// Beat 3: the range math — slice, suffix, 416, and the stale
	// If-Range falling back to the whole object.
	if !run.Range.SliceMatched || !run.Range.SuffixMatched {
		t.Errorf("range slices did not match the deterministic bytes: %+v", run.Range)
	}
	if run.Range.ContentRange != "bytes 1048576-1048639/4194304" {
		t.Errorf("content range = %q", run.Range.ContentRange)
	}
	if !run.Range.BeyondRefused416 || run.Range.BeyondContentRange != "bytes */4194304" {
		t.Errorf("past-the-end range must be the 416 census: %+v", run.Range)
	}
	if !run.Range.IfRangeServedFull || run.Range.FullBytesServed != storyUploadBytes {
		t.Errorf("a stale If-Range must serve the whole object: %+v", run.Range)
	}

	// Beat 4: the jail — every name refused, the symlink escape refused
	// by EvalSymlinks, the clean object untouched.
	for i, attempt := range run.Jail.Attempts {
		if !attempt.Refused {
			t.Errorf("traversal attempt %d (%q) was not refused", i, attempt.Attempt)
		}
	}
	if !run.Jail.SymlinkRefused {
		t.Error("the symlink escape must be refused by the EvalSymlinks re-jail")
	}
	if !run.Jail.CleanStillServed {
		t.Error("the clean object must still serve after the sweep")
	}
	if !strings.HasPrefix(run.Jail.Root, "/") || strings.Contains(run.Jail.Root, " ") {
		t.Errorf("story root = %q, want an absolute private path", run.Jail.Root)
	}

	// Beat 5: the thumbnail — aspect preserved, both encodings
	// non-empty, the JPEG round-trip, and the bomb refused from its
	// header before any pixel existed.
	if run.Thumb.ThumbW != 64 || run.Thumb.ThumbH != 40 {
		t.Errorf("thumb = %d×%d, want 64×40 (aspect preserved from 320×200)",
			run.Thumb.ThumbW, run.Thumb.ThumbH)
	}
	if run.Thumb.PngBytes == 0 || run.Thumb.JpegBytes == 0 {
		t.Errorf("encodings must be non-empty: %+v", run.Thumb)
	}
	if run.Thumb.JpegDecodedW != 64 || run.Thumb.JpegDecodedH != 40 {
		t.Errorf("jpeg round-trip = %d×%d", run.Thumb.JpegDecodedW, run.Thumb.JpegDecodedH)
	}
	if !run.Thumb.BombRefused {
		t.Error("the decode bomb must be refused from its header alone")
	}
}

func TestFilesWorkbenchStoryIsDeterministicAcrossRuns(t *testing.T) {
	console, err := NewConsole()
	if err != nil {
		t.Fatalf("console: %v", err)
	}
	first, err := console.RunFilesWorkbenchStory(context.Background())
	if err != nil {
		t.Fatalf("first run: %v", err)
	}
	second, err := console.RunFilesWorkbenchStory(context.Background())
	if err != nil {
		t.Fatalf("second run: %v", err)
	}
	// Wall time and allocation counts are machine-dependent by nature;
	// every FACT the story teaches must be identical run over run.
	first.Upload.WallMs, second.Upload.WallMs = "", ""
	first.Upload.AllocBytes, second.Upload.AllocBytes = 0, 0
	first.Jail.Root, second.Jail.Root = "", ""
	if !reflect.DeepEqual(first, second) {
		t.Errorf("two runs of the story disagree:\n%+v\n%+v", first, second)
	}
}

func TestNewConsoleWiresTheRouteStoreUnderTheDefaultRoot(t *testing.T) {
	console, err := NewConsole()
	if err != nil {
		t.Fatalf("console: %v", err)
	}
	if console.Store == nil || console.Store.Root() != DefaultRoot() {
		t.Errorf("console store root = %v, want %s", console.Store, DefaultRoot())
	}
	if console.UploadLimit != DefaultUploadLimit {
		t.Errorf("console limit = %d, want %d", console.UploadLimit, DefaultUploadLimit)
	}
	// The port proof the type declares holds at runtime too: the story
	// runs through the same interface the handler will hold.
	var runner demos.FilesWorkbenchRunner = console
	if _, err := runner.RunFilesWorkbenchStory(context.Background()); err != nil {
		t.Errorf("story through the runner port: %v", err)
	}
}
