package demos

import (
	"reflect"
	"strings"
	"testing"
)

// The race demo pairs LIVE synchronized totals with a PINNED racy
// exhibit; these tests pin both halves: the totals land exactly, the
// pinned verdict keeps the toolchain's shape, and the hygiene contrast
// reports both outcomes without leaking anything.

func TestRaceDetectorMutexRunIsExact(t *testing.T) {
	run := BuildRaceDetectorPayload().Mutex
	if run.Goroutines != raceGoroutines || run.IncrementsEach != raceIncrementsEach {
		t.Errorf("mutex run shape drifted: %+v", run)
	}
	if run.Expected != raceExpectedTotal || run.Observed != raceExpectedTotal || !run.Exact {
		t.Errorf("mutex total = %d (exact=%v), want exactly %d", run.Observed, run.Exact, raceExpectedTotal)
	}
}

func TestRaceDetectorChannelRunIsExact(t *testing.T) {
	run := BuildRaceDetectorPayload().Channel
	if run.Observed != raceExpectedTotal || !run.Exact {
		t.Errorf("channel total = %d (exact=%v), want exactly %d", run.Observed, run.Exact, raceExpectedTotal)
	}
}

func TestRaceDetectorExhibitPinsTheRealCodeAndVerdict(t *testing.T) {
	exhibit := BuildRaceDetectorPayload().Race
	if !strings.Contains(exhibit.Code, "func RacyTotal(goroutines, increments int) int {") ||
		!strings.Contains(exhibit.Code, "counter++") {
		t.Errorf("pinned code is not the unsynchronized counter exhibit:\n%s", exhibit.Code)
	}
	if strings.Contains(exhibit.Code, "mu.Lock") || strings.Contains(exhibit.Code, "<-") {
		t.Error("the pinned exhibit must be the RACY variant — synchronization found in it")
	}
	if exhibit.Command != "go test -race -count=1 ." {
		t.Errorf("verdict command drifted: %q", exhibit.Command)
	}
}

func TestRaceDetectorPinnedVerdictMatchesGoTestRaceShape(t *testing.T) {
	verdict := BuildRaceDetectorPayload().Race.Verdict
	for _, want := range []string{
		"==================\nWARNING: DATA RACE\n",
		"Read at 0x",
		"Previous write at 0x",
		"raceexhibit.RacyTotal.func1()",
		"/tmp/race-test/racy.go:13",
		"race detected during execution of test",
		"racy total = 3900, want 5000",
		"FAIL",
	} {
		if !strings.Contains(verdict, want) {
			t.Errorf("pinned verdict lost the `go test -race` shape marker %q:\n%s", want, verdict)
		}
	}
}

func TestRaceDetectorHygieneContrastReportsDoneAndWouldLeak(t *testing.T) {
	rows := BuildRaceDetectorPayload().Hygiene
	if len(rows) != 2 {
		t.Fatalf("hygiene = %d rows, want 2", len(rows))
	}
	if !rows[0].ReaderPresent || !strings.Contains(rows[0].Outcome, "delivered") {
		t.Errorf("done case drifted: %+v", rows[0])
	}
	if rows[1].ReaderPresent || !strings.Contains(rows[1].Outcome, "leak") {
		t.Errorf("would-leak case drifted: %+v", rows[1])
	}
}

func TestRaceDetectorPayloadIsPure(t *testing.T) {
	first, second := BuildRaceDetectorPayload(), BuildRaceDetectorPayload()
	if !reflect.DeepEqual(first, second) {
		t.Error("builder must be shape-deterministic: two calls returned different payloads")
	}
}
