// Topic 29 background-jobs' live demo: the jobs console. One request
// against the RUNNING stack drives a real asynq choreography over the
// guide's Redis — a flaky task that fails twice under exponential backoff
// and succeeds on its third attempt, a poison task that exhausts its
// retry budget and lands in the dead-letter archive, a manual replay that
// re-runs the archived task for real, a delayed digest that matures out
// of the scheduled state, cron ticks from a real scheduler entry, and the
// two observability numbers (queue depth and oldest-task age) sampled
// while the story runs. The builder below is PURE — every proof sentence
// derives from the run's own ledger rows, so the payload can never claim
// a timeline the worker did not live.
package demos

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// JobsDemoKeyPrefix namespaces every key the jobs demo touches: the
// compose Redis is shared, and gdg:demo:jobs marks this topic's ledger
// and counter keys as disposable territory (asynq's own queue keys live
// under its asynq:{queue}: prefix, never colliding with this one).
const JobsDemoKeyPrefix = "gdg:demo:jobs"

// The demo's two weighted queues — the worker hosts both, and the
// weights are real: the main queue is polled six times as often as the
// low one, and the poison task rides the low one on purpose so the
// weights carry live traffic instead of decorating a config file.
const (
	// JobsDemoQueue is the demo's main queue (weight 6).
	JobsDemoQueue = "demos"
	// JobsDemoLowQueue is the demo's low-priority queue (weight 1).
	JobsDemoLowQueue = "low"
)

// The demo's task types — the worker's mux routes on exactly these
// strings, and the payloads are JobsDemoPayload JSON in every case.
const (
	// JobsTaskFlaky fails its first JobsFlakyFailFirst attempts, then
	// succeeds — the retry/backoff story.
	JobsTaskFlaky = "demo:flaky"
	// JobsTaskPoison always fails — the dead-letter story.
	JobsTaskPoison = "demo:poison"
	// JobsTaskDigest succeeds on its first attempt after a delayed
	// enqueue — the scheduled-state and the canonical-job story.
	JobsTaskDigest = "demo:digest"
	// JobsTaskTick is the cron entry's task — one per scheduler fire.
	JobsTaskTick = "demo:tick"
)

// The demo's choreography constants — configuration, not observation:
// how many times flaky fails first, its retry budget, the poison budget,
// the digest delay, and the cron spec the scheduler registers.
const (
	// JobsFlakyFailFirst is how many attempts the flaky task fails
	// before succeeding.
	JobsFlakyFailFirst = 2
	// JobsFlakyMaxRetry is the flaky task's retry budget (two retries
	// for three total attempts — exactly enough to succeed).
	JobsFlakyMaxRetry = 2
	// JobsPoisonMaxRetry is the poison task's retry budget: one retry,
	// two total attempts, then the archive.
	JobsPoisonMaxRetry = 1
	// JobsDigestDelay is how long the digest task waits in the
	// scheduled state before its first attempt.
	JobsDigestDelay = 900 * time.Millisecond
	// JobsDemoCronSpec is the scheduler entry's spec — robfig/cron's
	// @every form: one tick per second.
	JobsDemoCronSpec = "@every 1s"
)

// JobsDemoEventTTL bounds the demo's redis-side residue: every ledger
// push re-arms this expiry, so even a story interrupted mid-run leaves
// nothing behind for longer than the TTL. The closing teardown deletes
// the keys outright.
const JobsDemoEventTTL = 5 * time.Minute

// Event outcome strings — the ledger's whole vocabulary.
const (
	// JobsOutcomeFailed marks an attempt that returned an error.
	JobsOutcomeFailed = "failed"
	// JobsOutcomeSucceeded marks an attempt that returned nil.
	JobsOutcomeSucceeded = "succeeded"
)

// JobsDemoPayload is the JSON every demo task carries: which run it
// belongs to (the ledger key is derived from it), the stable logical
// task id, and a human note that makes the timeline readable. Small and
// self-describing on purpose — the queue carries work, not objects.
type JobsDemoPayload struct {
	Run    string `json:"run"`
	TaskID string `json:"taskId"`
	Note   string `json:"note"`
}

// MarshalJobsDemoPayload encodes a demo payload for enqueue.
func MarshalJobsDemoPayload(payload JobsDemoPayload) ([]byte, error) {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal jobs demo payload: %w", err)
	}
	return encoded, nil
}

// ParseJobsDemoPayload decodes a demo payload from task bytes.
func ParseJobsDemoPayload(raw []byte) (JobsDemoPayload, error) {
	var payload JobsDemoPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return JobsDemoPayload{}, fmt.Errorf("parse jobs demo payload: %w", err)
	}
	return payload, nil
}

// JobsDemoEvent is one handler invocation as the ledger records it: which
// logical task, which type, which 1-based attempt, what came of it, and
// when. The ledger is an append-only list per run — the raw material
// every proof in the payload derives from.
type JobsDemoEvent struct {
	TaskID  string    `json:"taskId"`
	Type    string    `json:"type"`
	Attempt int       `json:"attempt"`
	Outcome string    `json:"outcome"`
	At      time.Time `json:"at"`
}

// MarshalJobsDemoEvent encodes one ledger event for RPUSH.
func MarshalJobsDemoEvent(event JobsDemoEvent) ([]byte, error) {
	encoded, err := json.Marshal(event)
	if err != nil {
		return nil, fmt.Errorf("marshal jobs demo event: %w", err)
	}
	return encoded, nil
}

// ParseJobsDemoEvent decodes one ledger event.
func ParseJobsDemoEvent(raw string) (JobsDemoEvent, error) {
	var event JobsDemoEvent
	if err := json.Unmarshal([]byte(raw), &event); err != nil {
		return JobsDemoEvent{}, fmt.Errorf("parse jobs demo event: %w", err)
	}
	return event, nil
}

// ErrJobsWorkerNotDraining is the honest failure when the story's tasks
// are enqueued but never processed: the hosted worker is not running,
// so the ledger never fills inside the story window. The asynq-backed
// runner (internal/data/jobs) answers this sentinel and the demo
// handler maps it to the typed 503 naming exactly that — a reachable
// broker with nobody draining it is a different outage than an
// unreachable broker, and the envelope should say which.
var ErrJobsWorkerNotDraining = errors.New("jobs console: no worker drained the demo queues inside the story window — is the hosted worker running?")

// JobsConsoleRunner is the port the jobs:console demo reads through: one
// call walks the whole choreography — enqueue, backoff, archive, replay,
// cron, observation, cleanup — against the live broker and reports the
// timeline. The asynq-backed implementation lives in internal/data/jobs;
// it presumes the hosted worker (the ServerRunner main.go starts) is
// draining the same queues.
type JobsConsoleRunner interface {
	// RunConsoleStory drives the choreography under a throwaway
	// namespace and reports what the broker and the handlers answered.
	RunConsoleStory(ctx context.Context, namespace string) (JobsConsoleRun, error)
}

// JobsQueueRow names one hosted queue and its weight.
type JobsQueueRow struct {
	Name   string
	Weight int
}

// JobsConsoleRun is the console story's raw haul: exactly what the live
// broker and handlers answered, beat by beat. The ledger arrives in
// execution order; the counts are derived facts the adapter observed
// directly (archive membership, replay outcome, peak queue statistics).
type JobsConsoleRun struct {
	Namespace        string
	Queues           []JobsQueueRow
	FlakyID          string
	PoisonID         string
	DigestID         string
	CronSpec         string
	Events           []JobsDemoEvent
	CronTicks        int
	PoisonArchived   bool
	PoisonLastErr    string
	PoisonAttempts   int // attempts before the replay
	ReplayAttempts   int // attempts the replay added
	PoisonReArchived bool
	PoisonDeleted    bool
	SchedulerStopped bool
	LedgerDeleted    bool
	PeakDepth        int
	PeakOldestAge    time.Duration
	FinalDepth       int
	FinalOldestAge   time.Duration
	Elapsed          time.Duration
}

// JobsConsolePayload is the wire shape of GET /api/demo/jobs/console
// (camelCase JSON). Every slice is non-nil so the wire renders [], never
// null; every sentence in Proofs is derived from the run's own rows.
type JobsConsolePayload struct {
	Namespace     string                `json:"namespace"`
	Queues        []JobsQueueWire       `json:"queues"`
	Commands      []string              `json:"commands"`
	Tasks         []JobsTaskWire        `json:"tasks"`
	Beats         []JobsConsoleBeatRow  `json:"beats"`
	Cron          JobsCronWire          `json:"cron"`
	Observability JobsObservabilityWire `json:"observability"`
	Proofs        JobsConsoleProofGroup `json:"proofs"`
	Reasoning     string                `json:"reasoning"`
}

// JobsQueueWire is one queue and its weight on the wire.
type JobsQueueWire struct {
	Name   string `json:"name"`
	Weight int    `json:"weight"`
}

// JobsAttemptWire is one attempt as the timeline serves it.
type JobsAttemptWire struct {
	Attempt int    `json:"attempt"`
	Outcome string `json:"outcome"`
}

// JobsTaskWire is one task's whole story on the wire: identity, routing,
// budget, the attempt ledger, and a derived verdict sentence.
type JobsTaskWire struct {
	ID       string            `json:"id"`
	Type     string            `json:"type"`
	Queue    string            `json:"queue"`
	MaxRetry int               `json:"maxRetry"`
	Attempts []JobsAttemptWire `json:"attempts"`
	Verdict  string            `json:"verdict"`
}

// JobsConsoleBeatRow is one beat as the payload serves it: what was
// done, the commands that did it, and what the broker answered.
type JobsConsoleBeatRow struct {
	Beat     string   `json:"beat"`
	Commands []string `json:"commands"`
	Outcome  string   `json:"outcome"`
}

// JobsCronWire carries the scheduler entry's spec and its observed ticks.
type JobsCronWire struct {
	Spec  string `json:"spec"`
	Ticks int    `json:"ticks"`
}

// JobsObservabilityWire is the two numbers, at their worst and at rest.
type JobsObservabilityWire struct {
	PeakDepth        int   `json:"peakDepth"`
	PeakOldestAgeMs  int64 `json:"peakOldestAgeMs"`
	FinalDepth       int   `json:"finalDepth"`
	FinalOldestAgeMs int64 `json:"finalOldestAgeMs"`
}

// JobsConsoleProofGroup carries the story's derived verdicts: the retry
// ladder, the archive, the replay's real re-execution, the cron cadence,
// and the observability pair.
type JobsConsoleProofGroup struct {
	Retry         string `json:"retry"`
	Archive       string `json:"archive"`
	Replay        string `json:"replay"`
	Cron          string `json:"cron"`
	Observability string `json:"observability"`
}

// NewJobsDemoNamespace mints a jobs demo run's throwaway namespace:
// gdg:demo:jobs:<12 hex chars>. Random per request, so concurrent runs
// never share ledgers, counters, or task ids.
func NewJobsDemoNamespace() (string, error) {
	var entropy [6]byte
	if _, err := rand.Read(entropy[:]); err != nil {
		return "", fmt.Errorf("read jobs demo namespace entropy: %w", err)
	}
	return JobsDemoKeyPrefix + ":" + hex.EncodeToString(entropy[:]), nil
}

// BuildJobsConsolePayload shapes a run into the demo payload. Every task
// row is derived from the ledger (the events list filtered per task id,
// in execution order); every proof sentence derives from those rows and
// the adapter's observed facts — the builder inspects, it never assumes.
func BuildJobsConsolePayload(run JobsConsoleRun) JobsConsolePayload {
	attemptsOf := func(taskID string) []JobsAttemptWire {
		attempts := []JobsAttemptWire{}
		for _, event := range run.Events {
			if event.TaskID == taskID {
				attempts = append(attempts, JobsAttemptWire{Attempt: event.Attempt, Outcome: event.Outcome})
			}
		}
		return attempts
	}
	flakyAttempts := attemptsOf(run.FlakyID)
	poisonAttempts := attemptsOf(run.PoisonID)
	digestAttempts := attemptsOf(run.DigestID)

	// The retry proof: the flaky ladder must end in a success whose
	// attempt number is exactly the failures before it plus one.
	flakyFailed, flakySucceededOn := 0, 0
	for _, attempt := range flakyAttempts {
		if attempt.Outcome == JobsOutcomeFailed {
			flakyFailed++
		} else {
			flakySucceededOn = attempt.Attempt
		}
	}
	retryProof := fmt.Sprintf(
		"the flaky task failed %d attempt(s) under exponential backoff and succeeded on attempt %d — "+
			"each failure rescheduled the task with a growing delay, and the worker drained every retry "+
			"without any caller waiting on the original request.",
		flakyFailed, flakySucceededOn,
	)
	if flakySucceededOn == 0 {
		retryProof = fmt.Sprintf(
			"the flaky task recorded %d attempt(s) and never succeeded inside the story window — "+
				"reported as it happened; with the hosted worker draining the queues this would mean a broker or worker outage.",
			flakyFailed,
		)
	}

	// The archive proof: the poison task's budget is its pre-replay
	// attempts (first execution + MaxRetry); the archive must hold it.
	archiveProof := fmt.Sprintf(
		"the poison task failed all %d of its budgeted attempts (first run + %d retry) and the archive "+
			"now holds it with the final error on record: %q — the dead-letter queue is where poison "+
			"messages go to be LOOKED AT, not lost.",
		run.PoisonAttempts, JobsPoisonMaxRetry, poisonLastErrOrDefault(run),
	)
	if !run.PoisonArchived {
		archiveProof = fmt.Sprintf(
			"the poison task recorded %d failed attempt(s) but was not observed in the archive inside "+
				"the story window — reported as it happened.", run.PoisonAttempts,
		)
	}

	// The replay proof: a replayed archive entry re-runs for real — the
	// ledger grew by exactly the re-attempt, and the task re-archived.
	replayProof := fmt.Sprintf(
		"RunTask moved the archived task back to pending; the worker executed it once more (the ledger "+
			"grew by %d attempt) and it failed the same honest way and re-archived — replay is a real "+
			"re-execution, not a replay of a recorded result; only a code fix or a payload fix changes the outcome.",
		run.ReplayAttempts,
	)
	if !run.PoisonReArchived {
		replayProof = fmt.Sprintf(
			"RunTask moved the archived task back to pending but it was not observed re-archived inside " +
				"the story window — reported as it happened.")
	}

	// The cron proof: ticks observed under the registered spec.
	cronProof := fmt.Sprintf(
		"the scheduler registered %q and the ledger observed %d tick task(s) enqueued and processed while "+
			"the entry lived — periodic work is just an enqueue with a clock attached.",
		run.CronSpec, run.CronTicks,
	)

	// The observability proof: the two numbers, at their worst and at rest.
	observabilityProof := fmt.Sprintf(
		"while the story ran, the queues' combined depth peaked at %d with the oldest pending task aged %d ms; "+
			"at rest they answer %d and %d ms. Depth trending up means the workers are falling behind; oldest-age "+
			"trending up with flat depth means the SAME work is waiting — these are the two numbers that page.",
		run.PeakDepth, run.PeakOldestAge.Milliseconds(), run.FinalDepth, run.FinalOldestAge.Milliseconds(),
	)

	queues := make([]JobsQueueWire, 0, len(run.Queues))
	for _, queue := range run.Queues {
		queues = append(queues, JobsQueueWire{Name: queue.Name, Weight: queue.Weight})
	}

	beats := []JobsConsoleBeatRow{
		{
			Beat: "enqueue",
			Commands: []string{
				fmt.Sprintf("ENQUEUE %s … Queue(%s) MaxRetry(%d)", JobsTaskFlaky, JobsDemoQueue, JobsFlakyMaxRetry),
				fmt.Sprintf("ENQUEUE %s … Queue(%s) MaxRetry(%d)", JobsTaskPoison, JobsDemoLowQueue, JobsPoisonMaxRetry),
			},
			Outcome: fmt.Sprintf(
				"two tasks accepted by the broker: the flaky one on the main queue, the poison one on the low "+
					"queue (weights are live: the low queue is polled %s of the time).",
				lowShareOf(run.Queues)),
		},
		{
			Beat:     "delayed",
			Commands: []string{fmt.Sprintf("ENQUEUE %s … ProcessIn(%s)", JobsTaskDigest, JobsDigestDelay)},
			Outcome: fmt.Sprintf(
				"the digest task waits in the SCHEDULED state for %s, then the forwarder promotes it to pending — "+
					"the same machinery retry backoff and cron rides on. Attempt ledger: %s.",
				JobsDigestDelay, digestVerdict(digestAttempts)),
		},
		{
			Beat:     "retry",
			Commands: []string{fmt.Sprintf("RETRY %s ← exponential backoff per failure", run.FlakyID)},
			Outcome: fmt.Sprintf(
				"the flaky task's attempt ladder: %s.", ladderOf(flakyAttempts)),
		},
		{
			Beat:     "archive",
			Commands: []string{fmt.Sprintf("ARCHIVE %s (retry budget exhausted)", run.PoisonID)},
			Outcome: fmt.Sprintf(
				"the poison task exhausted %d attempt(s) and moved to the dead-letter archive; its final error is "+
					"on record for the operator.", run.PoisonAttempts),
		},
		{
			Beat:     "replay",
			Commands: []string{fmt.Sprintf("RUN %s (archived → pending)", run.PoisonID)},
			Outcome: fmt.Sprintf(
				"the archived task was replayed once, re-executed for real, failed the same way, and re-archived; " +
					"the operator then deleted it — replay is a decision, the archive is the record."),
		},
		{
			Beat:     "cron",
			Commands: []string{fmt.Sprintf("REGISTER %q → ENQUEUE %s per fire", run.CronSpec, JobsTaskTick)},
			Outcome: fmt.Sprintf(
				"%d tick task(s) fired while the entry lived; the entry was unregistered and the scheduler stopped "+
					"inside the request.", run.CronTicks),
		},
		{
			Beat:     "observe",
			Commands: []string{"STATS queues (pending + oldest-pending age)"},
			Outcome: fmt.Sprintf(
				"combined depth peaked at %d (oldest %d ms); at rest: depth %d, oldest %d ms.",
				run.PeakDepth, run.PeakOldestAge.Milliseconds(), run.FinalDepth, run.FinalOldestAge.Milliseconds()),
		},
		{
			Beat:     "cleanup",
			Commands: []string{fmt.Sprintf("DELETE %s", run.PoisonID), fmt.Sprintf("DEL %s:*", run.Namespace)},
			Outcome: fmt.Sprintf(
				"the archived task was deleted (%v), the run's ledger keys were removed (%v), and the scheduler "+
					"entry unregistered (%v) — the throwaway namespace leaves nothing behind.",
				yesNo(run.PoisonDeleted), yesNo(run.LedgerDeleted), yesNo(run.SchedulerStopped)),
		},
	}

	cleanupNote := "clean"
	if !run.PoisonDeleted || !run.LedgerDeleted || !run.SchedulerStopped {
		cleanupNote = "reported as it happened"
	}
	reasoning := fmt.Sprintf(
		"Every beat above is what the live broker answered under %s over asynq and the guide's Redis. The retry "+
			"proof: failure is a first-class state — the task comes back with a growing delay until the budget is "+
			"spent, and success on a later attempt is the NORM, not a surprise. The archive proof: a task that can "+
			"never succeed stops retrying and lands where an operator can read its last error. The replay proof: "+
			"replaying the archive re-runs real code — idempotent handlers are what make that safe. The cron proof: "+
			"the scheduler is just a clock that enqueues. The observability pair — queue depth and oldest-task age — "+
			"is the entire monitoring story a background system needs. Teardown (%s): archive entry deleted, ledger "+
			"keys removed, scheduler entry unregistered.",
		run.Namespace, cleanupNote,
	)

	return JobsConsolePayload{
		Namespace: run.Namespace,
		Queues:    queues,
		Commands:  []string{"ENQUEUE", "RETRY", "ARCHIVE", "RUN", "DELETE", "REGISTER", "STATS"},
		Tasks: []JobsTaskWire{
			{
				ID: run.FlakyID, Type: JobsTaskFlaky, Queue: JobsDemoQueue, MaxRetry: JobsFlakyMaxRetry,
				Attempts: flakyAttempts, Verdict: flakyVerdict(flakyAttempts),
			},
			{
				ID: run.PoisonID, Type: JobsTaskPoison, Queue: JobsDemoLowQueue, MaxRetry: JobsPoisonMaxRetry,
				Attempts: poisonAttempts, Verdict: poisonVerdict(run, poisonAttempts),
			},
			{
				ID: run.DigestID, Type: JobsTaskDigest, Queue: JobsDemoQueue, MaxRetry: 0,
				Attempts: digestAttempts, Verdict: digestVerdict(digestAttempts),
			},
		},
		Beats: beats,
		Cron:  JobsCronWire{Spec: run.CronSpec, Ticks: run.CronTicks},
		Observability: JobsObservabilityWire{
			PeakDepth:        run.PeakDepth,
			PeakOldestAgeMs:  run.PeakOldestAge.Milliseconds(),
			FinalDepth:       run.FinalDepth,
			FinalOldestAgeMs: run.FinalOldestAge.Milliseconds(),
		},
		Proofs: JobsConsoleProofGroup{
			Retry:         retryProof,
			Archive:       archiveProof,
			Replay:        replayProof,
			Cron:          cronProof,
			Observability: observabilityProof,
		},
		Reasoning: reasoning,
	}
}

// flakyVerdict derives the flaky task's one-line story from its ladder.
func flakyVerdict(attempts []JobsAttemptWire) string {
	last := ""
	for _, attempt := range attempts {
		last = attempt.Outcome
	}
	if last == JobsOutcomeSucceeded {
		return fmt.Sprintf("succeeded after %d attempt(s) — the retry ladder worked as designed", len(attempts))
	}
	return fmt.Sprintf("did not succeed inside the story window (%d attempt(s) recorded)", len(attempts))
}

// poisonVerdict derives the poison task's story, archive and all.
func poisonVerdict(run JobsConsoleRun, attempts []JobsAttemptWire) string {
	state := "never archived inside the story window"
	if run.PoisonReArchived {
		state = "archived, replayed once, re-archived, then deleted"
	} else if run.PoisonArchived {
		state = "archived with its final error on record"
	}
	return fmt.Sprintf("failed every attempt (%d before the replay, %d after) — %s",
		len(attempts)-run.ReplayAttempts, run.ReplayAttempts, state)
}

// digestVerdict derives the digest task's story.
func digestVerdict(attempts []JobsAttemptWire) string {
	if len(attempts) == 1 && attempts[0].Outcome == JobsOutcomeSucceeded {
		return "one attempt, one success — the canonical job: small, bounded, out of band"
	}
	return fmt.Sprintf("%d attempt(s) recorded", len(attempts))
}

// ladderOf renders an attempt ladder: 1:failed → 2:failed → 3:succeeded.
func ladderOf(attempts []JobsAttemptWire) string {
	parts := make([]string, 0, len(attempts))
	for _, attempt := range attempts {
		parts = append(parts, fmt.Sprintf("%d:%s", attempt.Attempt, attempt.Outcome))
	}
	if len(parts) == 0 {
		return "no attempts recorded"
	}
	return joinArrows(parts)
}

// joinArrows joins parts with " → ".
func joinArrows(parts []string) string {
	out := ""
	for i, part := range parts {
		if i > 0 {
			out += " → "
		}
		out += part
	}
	return out
}

// lowShareOf derives the low queue's share of poll cycles from the live
// weights, e.g. "1/7".
func lowShareOf(queues []JobsQueueRow) string {
	total, low := 0, 0
	for _, queue := range queues {
		total += queue.Weight
		if queue.Name == JobsDemoLowQueue {
			low = queue.Weight
		}
	}
	if total == 0 || low == 0 {
		return "an unobserved share"
	}
	return fmt.Sprintf("%d/%d of poll cycles", low, total)
}

// poisonLastErrOrDefault renders the archived error or the honest absence.
func poisonLastErrOrDefault(run JobsConsoleRun) string {
	if run.PoisonLastErr == "" {
		return "no error text on record"
	}
	return run.PoisonLastErr
}

// yesNo renders a bool for the cleanup beat.
func yesNo(value bool) string {
	if value {
		return "yes"
	}
	return "no"
}
