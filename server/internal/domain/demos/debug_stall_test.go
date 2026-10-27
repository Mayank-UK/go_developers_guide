package demos

import (
	"runtime"
	"runtime/pprof"
	"strings"
	"testing"
	"time"
)

// The stall machinery from both sides: the parser against the
// runtime's real profile grammar (headers with durations, multi-frame
// stacks, created-by pairs, noise), and the run itself end to end —
// the wedge appears on the process's own profile, is found by its
// marker, and is released with every worker's exit observed.

func TestDebugStallParserReadsTheRuntimesGrammar(t *testing.T) {
	raw := strings.Join([]string{
		"goroutine 1 [running]:",
		"guide/internal/domain/demos.RunDebugStall(0x40?)",
		"\tserver/internal/domain/demos/debug_stall.go:201 +0x8f",
		"runtime/pprof.(*Profile).WriteTo(0x9?)",
		"\t/usr/local/go/src/runtime/pprof/pprof.go:356 +0x3d",
		"",
		"goroutine 42 [chan receive, 5 minutes]:",
		"guide/internal/domain/demos.debugStallWedge(0x0)",
		"\tserver/internal/domain/demos/debug_stall.go:118 +0x1d",
		"created by guide/internal/domain/demos.RunDebugStall",
		"\tserver/internal/domain/demos/debug_stall.go:157 +0x1a5",
	}, "\n")

	snapshots := ParseGoroutineProfile(raw)
	if len(snapshots) != 2 {
		t.Fatalf("parsed %d snapshots, want 2:\n%v", len(snapshots), snapshots)
	}

	running := snapshots[0]
	if running.ID != 1 || running.State != "running" || running.Wait != "" {
		t.Errorf("running header = id %d state %q wait %q", running.ID, running.State, running.Wait)
	}
	if len(running.Frames) != 2 {
		t.Fatalf("running frames = %d, want 2", len(running.Frames))
	}
	if want := "guide/internal/domain/demos.RunDebugStall(0x40?)"; running.Frames[0].Function != want {
		t.Errorf("frame function = %q, want %q", running.Frames[0].Function, want)
	}
	if want := "server/internal/domain/demos/debug_stall.go:201"; running.Frames[0].Location != want {
		t.Errorf("frame location = %q, want %q (pc offset stripped)", running.Frames[0].Location, want)
	}

	wedged := snapshots[1]
	if wedged.ID != 42 || wedged.State != "chan receive" || wedged.Wait != "5 minutes" {
		t.Errorf("wedged header = id %d state %q wait %q", wedged.ID, wedged.State, wedged.Wait)
	}
	if wedged.CreatedBy != "guide/internal/domain/demos.RunDebugStall" {
		t.Errorf("createdBy = %q", wedged.CreatedBy)
	}
	if want := "server/internal/domain/demos/debug_stall.go:157"; wedged.CreatedAt != want {
		t.Errorf("createdAt = %q, want %q", wedged.CreatedAt, want)
	}
	if !wedged.Matches("debugStallWedge") {
		t.Error("the wedged snapshot must match its own wedge marker")
	}
	if wedged.Matches("") || running.Matches("debugStallWedge") {
		t.Error("Matches must reject empty markers and non-matching stacks")
	}
}

func TestDebugStallParserSkipsNoiseAndHandlesEdgeShapes(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want func(t *testing.T, snapshots []GoroutineSnapshot)
	}{
		{
			name: "empty input parses to zero snapshots",
			raw:  "",
			want: func(t *testing.T, snapshots []GoroutineSnapshot) {
				if len(snapshots) != 0 {
					t.Errorf("snapshots = %d, want 0", len(snapshots))
				}
			},
		},
		{
			name: "runtime notes are skipped without stealing frames",
			raw: strings.Join([]string{
				"goroutine 7 [IO wait]:",
				"internal/poll.(*pollDesc).wait(0x1?)",
				"\t/usr/local/go/src/internal/poll/fd_posix.go:12 +0x8",
				"goroutine running on other thread; stack unavailable",
				"",
			}, "\n"),
			want: func(t *testing.T, snapshots []GoroutineSnapshot) {
				if len(snapshots) != 1 {
					t.Fatalf("snapshots = %d, want 1 (note skipped)", len(snapshots))
				}
				if len(snapshots[0].Frames) != 1 || snapshots[0].State != "IO wait" {
					t.Errorf("snapshot = %+v, want one IO-wait frame", snapshots[0])
				}
			},
		},
		{
			name: "location before any header is dropped",
			raw: strings.Join([]string{
				"\tstray.go:9 +0x1",
				"goroutine 3 [runnable]:",
				"main.work(0x0)",
				"\tmain.go:10 +0x2",
			}, "\n"),
			want: func(t *testing.T, snapshots []GoroutineSnapshot) {
				if len(snapshots) != 1 || snapshots[0].ID != 3 {
					t.Fatalf("snapshots = %+v, want the id-3 header only", snapshots)
				}
				if snapshots[0].CreatedAt != "" {
					t.Errorf("stray location leaked into createdAt: %q", snapshots[0].CreatedAt)
				}
			},
		},
		{
			name: "states with commas but no duration stay whole",
			raw:  "goroutine 9 [select, sync.Mutex.Lock]:\nmain.wait(0x0)\n\tmain.go:1 +0x1",
			want: func(t *testing.T, snapshots []GoroutineSnapshot) {
				if len(snapshots) != 1 {
					t.Fatalf("snapshots = %d, want 1", len(snapshots))
				}
				if snapshots[0].State != "select, sync.Mutex.Lock" || snapshots[0].Wait != "" {
					t.Errorf("state = %q wait = %q — a non-duration comma must not split", snapshots[0].State, snapshots[0].Wait)
				}
			},
		},
		{
			name: "created by location rides the tab after the spawner",
			raw: strings.Join([]string{
				"goroutine 12 [semacquire]:",
				"sync.runtime_SemacquireMutex(0x0, 0x0, 0x1)",
				"\t/usr/local/go/src/runtime/sema.go:95 +0x25",
				"created by main.spawn",
				"\tmain.go:44 +0x5c",
			}, "\n"),
			want: func(t *testing.T, snapshots []GoroutineSnapshot) {
				if len(snapshots) != 1 {
					t.Fatalf("snapshots = %d, want 1", len(snapshots))
				}
				got := snapshots[0]
				if got.CreatedBy != "main.spawn" || got.CreatedAt != "main.go:44" {
					t.Errorf("created = %q @ %q, want main.spawn @ main.go:44", got.CreatedBy, got.CreatedAt)
				}
				if len(got.Frames) != 1 || got.Frames[0].Location != "/usr/local/go/src/runtime/sema.go:95" {
					t.Errorf("frames = %+v — the created-by location must not overwrite the frame's", got.Frames)
				}
			},
		},
		{
			name: "a non-numeric id is not a header",
			raw:  "goroutine gp [running]:\nmain.f()\n\tmain.go:1 +0x1",
			want: func(t *testing.T, snapshots []GoroutineSnapshot) {
				if len(snapshots) != 0 {
					t.Errorf("snapshots = %d, want 0", len(snapshots))
				}
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tc.want(t, ParseGoroutineProfile(tc.raw))
		})
	}
}

// TestDebugStallRunWedgesFindsAndReleases proves the whole lifecycle
// with the production blocker: four workers finish, the wedge lands on
// the process's own goroutine profile, the finder names it by its
// stack, the request's own goroutine serves as the contrast, and the
// release lets every worker exit.
func TestDebugStallRunWedgesFindsAndReleases(t *testing.T) {
	run := RunDebugStall(nil, "")

	if run.Workers != DebugStallWorkers || run.HealthyDone != DebugStallWorkers-1 {
		t.Errorf("workload = %d workers, %d healthy done — want %d and %d",
			run.Workers, run.HealthyDone, DebugStallWorkers, DebugStallWorkers-1)
	}
	if !run.AllExited {
		t.Error("every worker's exit must be observed after release")
	}
	if run.CaptureTries < 1 {
		t.Error("capture must actually run")
	}
	if run.GoroutineSeen < 2 {
		t.Errorf("goroutines seen = %d — the capture must observe a living process", run.GoroutineSeen)
	}
	if run.Stalled == nil {
		t.Fatal("the wedged goroutine must be found on its own profile")
	}
	if run.Stalled.State != "chan receive" {
		t.Errorf("stalled state = %q, want chan receive", run.Stalled.State)
	}
	foundWedge := false
	for _, frame := range run.Stalled.Frames {
		if strings.Contains(frame.Function, "debugStallWedge") && strings.Contains(frame.Location, "debug_stall.go") {
			foundWedge = true
		}
		if frame.Location == "" {
			t.Errorf("frame %q has no source location", frame.Function)
		}
	}
	if !foundWedge {
		t.Errorf("stalled frames = %+v — the wedge frame with its source line must be present", run.Stalled.Frames)
	}
	if !strings.Contains(run.Stalled.CreatedBy, "RunDebugStall") {
		t.Errorf("createdBy = %q — the runtime names the spawner", run.Stalled.CreatedBy)
	}
	if run.Contrast == nil {
		t.Fatal("the request's own goroutine must serve as the contrast")
	}
	if run.Contrast.ID == run.Stalled.ID {
		t.Errorf("contrast id %d equals stalled id — the finder must exclude the wedge", run.Contrast.ID)
	}
	if !run.Contrast.Matches("RunDebugStall") {
		t.Errorf("contrast state %q must be the capturing goroutine", run.Contrast.State)
	}
}

// TestDebugStallInjectableBlockerIsHonoredAndCleanedUp drives the run
// with an injected blocker — determinism proof: the wedge blocks where
// the injection says, is FOUND by the injected marker (no 2s retry
// burn), and release still returns every worker.
func TestDebugStallInjectableBlockerIsHonoredAndCleanedUp(t *testing.T) {
	entered := make(chan struct{})
	run := RunDebugStall(func(release <-chan struct{}) {
		close(entered)
		<-release
	}, "IsHonoredAndCleanedUp")

	select {
	case <-entered:
	default:
		t.Error("the injected blocker must actually run")
	}
	if run.Stalled == nil {
		t.Fatal("the finder must locate an injected blocker by its marker")
	}
	if !run.AllExited {
		t.Error("release must free an injected blocker's workers too")
	}
	if run.CaptureTries > 20 {
		t.Errorf("capture tries = %d — a marker-honoring finder settles on the first captures", run.CaptureTries)
	}
}

// TestDebugStallLeaksNoGoroutines is the leak proof, two layers: the
// precise one reads the process's own goroutine profile after the run
// and asserts NO goroutine carries the wedge frame (sibling-test noise
// cannot mask this), and the coarse one retries until the total count
// settles back to its baseline.
func TestDebugStallLeaksNoGoroutines(t *testing.T) {
	before := runtime.NumGoroutine()

	RunDebugStall(nil, "")

	// Precise: the wedge frame must exist on no goroutine's stack.
	var profile strings.Builder
	if looked := pprof.Lookup("goroutine"); looked != nil {
		_ = looked.WriteTo(&profile, 2)
	}
	for _, snapshot := range ParseGoroutineProfile(profile.String()) {
		if snapshot.Matches("debugStallWedge") {
			t.Errorf("goroutine %d still carries the wedge frame after release:\n%+v", snapshot.ID, snapshot.Frames)
		}
	}

	// Coarse: the total settles back to baseline, retry-until-stable.
	deadline := time.Now().Add(3 * time.Second)
	for runtime.NumGoroutine() > before && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if after := runtime.NumGoroutine(); after > before {
		t.Errorf("goroutines %d → %d after the run — the wedge leaked", before, after)
	}
}

func TestBuildDebugStallPayloadShapesHonestPanels(t *testing.T) {
	run := DebugStallRun{
		Workers:       DebugStallWorkers,
		HealthyDone:   DebugStallWorkers - 1,
		CaptureMs:     0.4,
		CaptureTries:  1,
		GoroutineSeen: 9,
		Stalled: &GoroutineSnapshot{
			ID:    42,
			State: "chan receive",
			Wait:  "1 minutes",
			Frames: []GoroutineFrame{
				{Function: "guide/internal/domain/demos.debugStallWedge(0x0)", Location: "server/internal/domain/demos/debug_stall.go:118"},
			},
			CreatedBy: "guide/internal/domain/demos.RunDebugStall",
			CreatedAt: "server/internal/domain/demos/debug_stall.go:157",
		},
		Contrast: &GoroutineSnapshot{
			ID:    1,
			State: "running",
			Frames: []GoroutineFrame{
				{Function: "guide/internal/domain/demos.RunDebugStall(0x0)", Location: "server/internal/domain/demos/debug_stall.go:201"},
			},
		},
		AllExited: true,
	}

	payload := BuildDebugStallPayload(run)
	if payload.Workload.Workers != DebugStallWorkers || payload.Workload.HealthyDone != 4 {
		t.Errorf("workload = %+v", payload.Workload)
	}
	if payload.Capture.GoroutinesSeen != 9 || payload.Capture.CaptureTries != 1 {
		t.Errorf("capture = %+v", payload.Capture)
	}
	if payload.Stalled.ID != 42 || payload.Stalled.State != "chan receive" || payload.Stalled.Wait != "1 minutes" {
		t.Errorf("stalled = %+v", payload.Stalled)
	}
	if len(payload.Stalled.Frames) != 1 || payload.Stalled.Frames[0].Location != "server/internal/domain/demos/debug_stall.go:118" {
		t.Errorf("stalled frames = %+v", payload.Stalled.Frames)
	}
	if payload.Contrast.ID != 1 || payload.Contrast.State != "running" {
		t.Errorf("contrast = %+v", payload.Contrast)
	}
	if len(payload.Endpoints) != len(DebugStallEndpoints) {
		t.Errorf("endpoints = %d, want the live surface", len(payload.Endpoints))
	}
	if !strings.Contains(payload.Capture.Curl, "/internal/debug/pprof/goroutine") {
		t.Errorf("capture curl must point at the live endpoint: %q", payload.Capture.Curl)
	}

	// The honest empty state: a run that failed to find its wedge says
	// so instead of inventing a stack.
	missing := BuildDebugStallPayload(DebugStallRun{Workers: DebugStallWorkers})
	if missing.Stalled.State != "(not found)" || missing.Stalled.Frames == nil {
		t.Errorf("missing stall = %+v — must carry the honest marker and a non-nil frames slice", missing.Stalled)
	}
}
