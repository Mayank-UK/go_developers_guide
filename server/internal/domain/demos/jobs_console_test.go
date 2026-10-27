package demos

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// The jobs console builder's tests: the payload's every proof sentence
// derives from the run's own ledger rows — a fixture run pins each
// derivation, and a hostile run (nothing succeeded, nothing archived)
// pins that the builder reports absence instead of claiming success.

func jobsConsoleFixtureRun() JobsConsoleRun {
	return JobsConsoleRun{
		Namespace: "gdg:demo:jobs:0123456789ab",
		Queues: []JobsQueueRow{
			{Name: JobsDemoQueue, Weight: 6},
			{Name: JobsDemoLowQueue, Weight: 1},
		},
		FlakyID:  "gdg:demo:jobs:0123456789ab:flaky",
		PoisonID: "gdg:demo:jobs:0123456789ab:poison",
		DigestID: "gdg:demo:jobs:0123456789ab:digest",
		CronSpec: JobsDemoCronSpec,
		Events: []JobsDemoEvent{
			{TaskID: "gdg:demo:jobs:0123456789ab:flaky", Type: JobsTaskFlaky, Attempt: 1, Outcome: JobsOutcomeFailed, At: time.Unix(1000, 0)},
			{TaskID: "gdg:demo:jobs:0123456789ab:poison", Type: JobsTaskPoison, Attempt: 1, Outcome: JobsOutcomeFailed, At: time.Unix(1000, 0)},
			{TaskID: "gdg:demo:jobs:0123456789ab:digest", Type: JobsTaskDigest, Attempt: 1, Outcome: JobsOutcomeSucceeded, At: time.Unix(1001, 0)},
			{TaskID: "gdg:demo:jobs:0123456789ab:flaky", Type: JobsTaskFlaky, Attempt: 2, Outcome: JobsOutcomeFailed, At: time.Unix(1002, 0)},
			{TaskID: "gdg:demo:jobs:0123456789ab:poison", Type: JobsTaskPoison, Attempt: 2, Outcome: JobsOutcomeFailed, At: time.Unix(1002, 0)},
			{TaskID: "tick", Type: JobsTaskTick, Attempt: 1, Outcome: JobsOutcomeSucceeded, At: time.Unix(1003, 0)},
			{TaskID: "gdg:demo:jobs:0123456789ab:flaky", Type: JobsTaskFlaky, Attempt: 3, Outcome: JobsOutcomeSucceeded, At: time.Unix(1004, 0)},
			{TaskID: "tick", Type: JobsTaskTick, Attempt: 1, Outcome: JobsOutcomeSucceeded, At: time.Unix(1004, 0)},
			{TaskID: "gdg:demo:jobs:0123456789ab:poison", Type: JobsTaskPoison, Attempt: 2, Outcome: JobsOutcomeFailed, At: time.Unix(1006, 0)},
		},
		CronTicks:        2,
		PoisonArchived:   true,
		PoisonLastErr:    "digest upstream refused: 503",
		PoisonAttempts:   2,
		ReplayAttempts:   1,
		PoisonReArchived: true,
		PoisonDeleted:    true,
		SchedulerStopped: true,
		LedgerDeleted:    true,
		PeakDepth:        3,
		PeakOldestAge:    640 * time.Millisecond,
		FinalDepth:       0,
		FinalOldestAge:   0,
		Elapsed:          4 * time.Second,
	}
}

func TestJobsConsolePayloadDerivesEveryProof(t *testing.T) {
	payload := BuildJobsConsolePayload(jobsConsoleFixtureRun())

	if payload.Namespace != "gdg:demo:jobs:0123456789ab" {
		t.Errorf("namespace = %q", payload.Namespace)
	}
	if !strings.Contains(payload.Proofs.Retry, "failed 2 attempt(s)") ||
		!strings.Contains(payload.Proofs.Retry, "succeeded on attempt 3") {
		t.Errorf("retry proof must derive the ladder: %q", payload.Proofs.Retry)
	}
	if !strings.Contains(payload.Proofs.Archive, "all 2 of its budgeted attempts") ||
		!strings.Contains(payload.Proofs.Archive, "digest upstream refused: 503") {
		t.Errorf("archive proof must derive budget + last error: %q", payload.Proofs.Archive)
	}
	if !strings.Contains(payload.Proofs.Replay, "grew by 1 attempt") {
		t.Errorf("replay proof must derive the re-execution: %q", payload.Proofs.Replay)
	}
	if !strings.Contains(payload.Proofs.Cron, "2 tick task(s)") {
		t.Errorf("cron proof must derive ticks: %q", payload.Proofs.Cron)
	}
	if !strings.Contains(payload.Proofs.Observability, "peaked at 3") ||
		!strings.Contains(payload.Proofs.Observability, "answer 0 and 0 ms") {
		t.Errorf("observability proof must derive peak and rest: %q", payload.Proofs.Observability)
	}
}

func TestJobsConsolePayloadTaskRowsMatchTheLedger(t *testing.T) {
	payload := BuildJobsConsolePayload(jobsConsoleFixtureRun())

	if len(payload.Tasks) != 3 {
		t.Fatalf("tasks = %d, want 3", len(payload.Tasks))
	}
	flaky := payload.Tasks[0]
	if flaky.Type != JobsTaskFlaky || len(flaky.Attempts) != 3 {
		t.Errorf("flaky row = %+v", flaky)
	}
	if flaky.Attempts[2].Outcome != JobsOutcomeSucceeded {
		t.Errorf("flaky ladder must end in success: %+v", flaky.Attempts)
	}
	poison := payload.Tasks[1]
	if poison.Queue != JobsDemoLowQueue || poison.MaxRetry != JobsPoisonMaxRetry {
		t.Errorf("poison row must carry the low queue + budget: %+v", poison)
	}
	if len(poison.Attempts) != 3 || poison.Attempts[2].Attempt != 2 {
		t.Errorf("poison attempts must include the replay re-run (same attempt number): %+v", poison.Attempts)
	}
	digest := payload.Tasks[2]
	if !strings.Contains(digest.Verdict, "one attempt, one success") {
		t.Errorf("digest verdict = %q", digest.Verdict)
	}
}

func TestJobsConsolePayloadBeatsAndObservability(t *testing.T) {
	payload := BuildJobsConsolePayload(jobsConsoleFixtureRun())

	if len(payload.Beats) != 8 {
		t.Errorf("beats = %d, want 8", len(payload.Beats))
	}
	if payload.Cron.Spec != JobsDemoCronSpec || payload.Cron.Ticks != 2 {
		t.Errorf("cron wire = %+v", payload.Cron)
	}
	if payload.Observability.PeakDepth != 3 || payload.Observability.PeakOldestAgeMs != 640 {
		t.Errorf("observability wire = %+v", payload.Observability)
	}
	if payload.Observability.FinalDepth != 0 || payload.Observability.FinalOldestAgeMs != 0 {
		t.Errorf("final observability must be at rest: %+v", payload.Observability)
	}
	if !strings.Contains(payload.Reasoning, "failure is a first-class state") {
		t.Errorf("reasoning must carry the thesis: %q", payload.Reasoning)
	}
}

func TestJobsConsolePayloadReportsAbsenceHonestly(t *testing.T) {
	run := JobsConsoleRun{
		Namespace: "gdg:demo:jobs:ffffffffffff", FlakyID: "f", PoisonID: "p", DigestID: "d",
		CronSpec: JobsDemoCronSpec, Queues: []JobsQueueRow{{Name: JobsDemoQueue, Weight: 6}},
	}
	payload := BuildJobsConsolePayload(run)

	if !strings.Contains(payload.Proofs.Retry, "never succeeded inside the story window") {
		t.Errorf("empty ladder must be reported, not claimed: %q", payload.Proofs.Retry)
	}
	if !strings.Contains(payload.Proofs.Archive, "not observed in the archive") {
		t.Errorf("missing archive must be reported: %q", payload.Proofs.Archive)
	}
	if !strings.Contains(payload.Proofs.Replay, "not observed re-archived") {
		t.Errorf("missing re-archive must be reported: %q", payload.Proofs.Replay)
	}
	if !strings.Contains(payload.Reasoning, "reported as it happened") {
		t.Errorf("cleanup must be reported as it happened: %q", payload.Reasoning)
	}
	if payload.Tasks[0].Attempts == nil {
		t.Error("attempt slices must be non-nil so the wire renders []")
	}
}

func TestJobsConsolePayloadMarshalsOnTheWire(t *testing.T) {
	payload := BuildJobsConsolePayload(jobsConsoleFixtureRun())
	encoded, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("payload must be plain JSON: %v", err)
	}
	for _, key := range []string{"namespace", "queues", "commands", "tasks", "beats", "cron", "observability", "proofs", "reasoning"} {
		if _, ok := decoded[key]; !ok {
			t.Errorf("wire payload missing %q", key)
		}
	}
}

func TestJobsDemoPayloadRoundTrips(t *testing.T) {
	encoded, err := MarshalJobsDemoPayload(JobsDemoPayload{Run: "r-1", TaskID: "t-1", Note: "the receipt email"})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	decoded, err := ParseJobsDemoPayload(encoded)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if decoded.Run != "r-1" || decoded.TaskID != "t-1" || decoded.Note != "the receipt email" {
		t.Errorf("round trip drifted: %+v", decoded)
	}
}

func TestJobsDemoEventRoundTrips(t *testing.T) {
	at := time.Unix(1780000000, 0).UTC()
	encoded, err := MarshalJobsDemoEvent(JobsDemoEvent{TaskID: "t-1", Type: JobsTaskFlaky, Attempt: 2, Outcome: JobsOutcomeFailed, At: at})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	decoded, err := ParseJobsDemoEvent(string(encoded))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if decoded.TaskID != "t-1" || decoded.Attempt != 2 || !decoded.At.Equal(at) {
		t.Errorf("round trip drifted: %+v", decoded)
	}
}

func TestNewJobsDemoNamespaceIsThrowaway(t *testing.T) {
	first, err := NewJobsDemoNamespace()
	if err != nil {
		t.Fatalf("mint namespace: %v", err)
	}
	second, err := NewJobsDemoNamespace()
	if err != nil {
		t.Fatalf("mint namespace: %v", err)
	}
	if !strings.HasPrefix(first, JobsDemoKeyPrefix+":") || first == second {
		t.Errorf("namespaces must be prefixed and unique: %q vs %q", first, second)
	}
}
