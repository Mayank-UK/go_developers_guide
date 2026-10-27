package jobs

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/hibiken/asynq"
	goredis "github.com/redis/go-redis/v9"

	"guide/internal/domain/demos"
	"guide/internal/domain/jobs"
)

// Compile-time proof the story runner satisfies the demo port.
var _ demos.JobsConsoleRunner = (*ConsoleStoryRunner)(nil)

// The story's timing budget — configuration, not observation. The poll
// cadence is fast enough to catch short-lived queue states without
// hammering the broker; the deadline bounds the whole walk (retries
// mature in ~0.5–2s, the forwarder promotes scheduled tasks on ~1s
// polls, cron fires per second, so 12s is generous); the replay gets its
// own shorter window (one more low-queue attempt).
const (
	storyPollEvery = 150 * time.Millisecond
	storyDeadline  = 12 * time.Second
	replayDeadline = 6 * time.Second
	minCronTicks   = 2
)

// ErrWorkerNotDraining is the domain's honest worker-down sentinel —
// aliased here (same value, errors.Is works through every wrap) so the
// adapter's call sites read in package terms while the presentation
// layer stays ignorant of this package: handlers never import the data
// layer, and the sentinel is part of the PORT's contract.
var ErrWorkerNotDraining = demos.ErrJobsWorkerNotDraining

// ConsoleStoryRunner choreographs the jobs:console demo: it enqueues the
// story's tasks through the domain's Enqueuer port, drives a real cron
// entry on a per-run scheduler, watches the run's ledger fill as the
// hosted worker processes (and retries, and archives) the tasks, replays
// the archived poison task, samples the two observability numbers while
// everything runs, and tears down every key and entry it created. The
// cron clock is stopped BEFORE the teardown deletes anything, so no tick
// can race the cleanup and re-create a deleted key.
type ConsoleStoryRunner struct {
	client *goredis.Client
	// enqueuer is the domain's produce port (the adapter behind it is
	// this package's Enqueuer). inspector is held CONCRETE: the teardown
	// purge needs the broker's task listings to reach broker-assigned
	// cron tick ids, a listing the domain port deliberately does not
	// carry — satisfaction of the port is proven on the type itself.
	enqueuer  jobs.Enqueuer
	inspector *Inspector
}

// NewConsoleStoryRunner wires the story over the shared go-redis client
// (the ledger lives beside the broker) and the asynq-backed ports.
func NewConsoleStoryRunner(client *goredis.Client) *ConsoleStoryRunner {
	return &ConsoleStoryRunner{
		client:    client,
		enqueuer:  NewEnqueuer(client),
		inspector: NewInspector(client),
	}
}

// RunConsoleStory walks the choreography under namespace and reports the
// timeline. The first enqueue failure aborts with the command's error
// wrapped (%w) for the handler's honest 503; after the tasks are in,
// failures to observe terminal states surface as ErrWorkerNotDraining
// (with what was observed joined in) — an enqueue the broker accepted is
// durable, so the only missing ingredient is a worker.
func (r *ConsoleStoryRunner) RunConsoleStory(ctx context.Context, namespace string) (demos.JobsConsoleRun, error) {
	startedAt := time.Now()
	if ctx == nil {
		return demos.JobsConsoleRun{}, errors.New("jobs console: context must not be nil")
	}

	flakyID := namespace + ":flaky"
	poisonID := namespace + ":poison"
	digestID := namespace + ":digest"

	// The enqueue beat: flaky on the main queue, poison on the low one,
	// digest delayed into the scheduled state.
	if _, err := r.enqueuer.Enqueue(ctx, jobs.TaskSpec{
		ID: flakyID, Type: demos.JobsTaskFlaky,
		Payload: jobsPayload(namespace, flakyID, "the receipt email that fails twice, then lands"),
		Queue:   demos.JobsDemoQueue, MaxRetry: demos.JobsFlakyMaxRetry, Timeout: 5 * time.Second,
	}); err != nil {
		return demos.JobsConsoleRun{}, err
	}
	if _, err := r.enqueuer.Enqueue(ctx, jobs.TaskSpec{
		ID: poisonID, Type: demos.JobsTaskPoison,
		Payload: jobsPayload(namespace, poisonID, "the upstream that answers 500 forever"),
		Queue:   demos.JobsDemoLowQueue, MaxRetry: demos.JobsPoisonMaxRetry, Timeout: 5 * time.Second,
	}); err != nil {
		return demos.JobsConsoleRun{}, err
	}
	if _, err := r.enqueuer.Enqueue(ctx, jobs.TaskSpec{
		ID: digestID, Type: demos.JobsTaskDigest,
		Payload: jobsPayload(namespace, digestID, "the daily digest, out of band"),
		Queue:   demos.JobsDemoQueue, MaxRetry: 0, ProcessIn: demos.JobsDigestDelay, Timeout: 5 * time.Second,
	}); err != nil {
		return demos.JobsConsoleRun{}, err
	}

	// From here the broker HOLDS work under this namespace. Every exit
	// path below runs the best-effort teardown: the success walk performs
	// the same deletes authoritatively (idempotent no-ops by then), and
	// an aborted run — a canceled caller, a missing worker — still
	// leaves no pending or archived task behind in the shared broker.
	defer r.bestEffortTeardown(context.WithoutCancel(ctx), namespace, flakyID, poisonID, digestID)

	// The cron beat: a per-run scheduler entry firing one tick per
	// second. The scheduler owns a DEDICATED connection (asynq closes
	// the client it was built from when it shuts down), so the story
	// never closes the process's shared pool out from under the app.
	scheduler := newStoryScheduler(r.client.Options().Addr)
	entryID, err := scheduler.Register(demos.JobsDemoCronSpec,
		asynq.NewTask(demos.JobsTaskTick, jobsPayload(namespace, namespace+":tick", "one cron fire")),
		asynq.Queue(demos.JobsDemoQueue))
	if err != nil {
		return demos.JobsConsoleRun{}, fmt.Errorf("register cron entry %q: %w", demos.JobsDemoCronSpec, err)
	}
	if err := scheduler.Start(); err != nil {
		return demos.JobsConsoleRun{}, fmt.Errorf("start the cron scheduler: %w", err)
	}
	// Safety net for the error paths below; on the success path the
	// ordered shutdown below runs first (asynq's Shutdown is idempotent,
	// so the deferred second call is a no-op).
	defer scheduler.Shutdown()

	// The observation beat: watch the ledger fill and the queues drain,
	// sampling depth and oldest-age along the way.
	run, observeErr := r.observe(ctx, namespace, flakyID, poisonID, digestID)
	if observeErr != nil {
		return demos.JobsConsoleRun{}, observeErr
	}

	// Stop the clock BEFORE any teardown: the entry is unregistered (no
	// further fires can be scheduled), the scheduler shut down, and ticks
	// already enqueued get the replay beat's full window to drain before
	// the ledger keys are deleted — no tick can race the cleanup and
	// resurrect a key.
	if err := scheduler.Unregister(entryID); err != nil {
		return demos.JobsConsoleRun{}, fmt.Errorf("unregister cron entry %q: %w", entryID, err)
	}
	scheduler.Shutdown()
	run.SchedulerStopped = true

	// The replay beat: the poison task is archived now — run it again
	// for real, wait for the re-attempt and the re-archive, then delete
	// it from the archive.
	if err := r.inspector.Replay(ctx, demos.JobsDemoLowQueue, poisonID); err != nil {
		return demos.JobsConsoleRun{}, err
	}
	replay, err := r.waitForReplay(ctx, namespace, poisonID, run.PoisonAttempts)
	if err != nil {
		return demos.JobsConsoleRun{}, err
	}
	run.Events = replay.events
	run.ReplayAttempts = replay.replayAttempts
	run.PoisonReArchived = replay.reArchived
	run.PoisonDeleted = replay.deleted

	// The final observability sample: everything at rest.
	final := r.snapshotStats(ctx)
	run.FinalDepth = final.depth
	run.FinalOldestAge = final.oldest

	// Teardown: the archived task first (the record the archive holds),
	// then the ledger and counter keys.
	if err := r.client.Del(ctx,
		eventsKey(namespace),
		attemptKey(demos.JobsDemoPayload{Run: namespace, TaskID: flakyID}),
		attemptKey(demos.JobsDemoPayload{Run: namespace, TaskID: poisonID}),
		attemptKey(demos.JobsDemoPayload{Run: namespace, TaskID: digestID}),
	).Err(); err != nil {
		return demos.JobsConsoleRun{}, fmt.Errorf("DEL %s keys: %w", namespace, err)
	}
	run.LedgerDeleted = true

	run.Namespace = namespace
	run.Queues = queueRows()
	run.FlakyID = flakyID
	run.PoisonID = poisonID
	run.DigestID = digestID
	run.CronSpec = demos.JobsDemoCronSpec
	run.Elapsed = time.Since(startedAt)
	return run, nil
}

// newStoryScheduler builds the story's dedicated scheduler connection —
// short timeouts so an unreachable Redis fails fast instead of hanging
// the demo request.
func newStoryScheduler(addr string) *asynq.Scheduler {
	return asynq.NewScheduler(asynq.RedisClientOpt{
		Addr:         addr,
		DialTimeout:  2 * time.Second,
		ReadTimeout:  2 * time.Second,
		WriteTimeout: 2 * time.Second,
	}, nil)
}

// observe polls the ledger and the queues until every terminal condition
// holds — flaky succeeded, poison archived, digest processed, enough
// cron ticks — or the deadline turns the absence into
// ErrWorkerNotDraining. Peak depth and oldest-age are tracked across
// every sample, so the payload's observability pair is a real maximum,
// not a terminal snapshot.
func (r *ConsoleStoryRunner) observe(ctx context.Context, namespace, flakyID, poisonID, digestID string) (demos.JobsConsoleRun, error) {
	run := demos.JobsConsoleRun{}
	flakyDone, digestSeen := false, false
	deadline := time.Now().Add(storyDeadline)
	for time.Now().Before(deadline) {
		if err := ctx.Err(); err != nil {
			return run, fmt.Errorf("jobs console: caller canceled: %w", err)
		}
		events, err := r.readEvents(ctx, namespace)
		if err != nil {
			return run, err
		}
		run.Events = events
		run.CronTicks = countTicks(events)
		run.PoisonAttempts = countAttempts(events, poisonID)
		flakyDone = flakySucceeded(events, flakyID)
		digestSeen = digestDone(events, digestID)

		sample := r.snapshotStats(ctx)
		if sample.depth > run.PeakDepth {
			run.PeakDepth = sample.depth
		}
		if sample.oldest > run.PeakOldestAge {
			run.PeakOldestAge = sample.oldest
		}

		if flakyDone {
			archived, lastErr, err := r.poisonArchive(ctx, poisonID)
			if err != nil {
				return run, err
			}
			run.PoisonArchived = archived
			run.PoisonLastErr = lastErr
			if run.PoisonArchived && digestSeen && run.CronTicks >= minCronTicks {
				return run, nil
			}
		}
		time.Sleep(storyPollEvery)
	}
	return run, fmt.Errorf("%w (flaky succeeded=%v poison archived=%v digest seen=%v ticks=%d)",
		ErrWorkerNotDraining, flakyDone, run.PoisonArchived, digestSeen, run.CronTicks)
}

// replayState is the replay beat's outcome.
type replayState struct {
	events         []demos.JobsDemoEvent
	replayAttempts int
	reArchived     bool
	deleted        bool
}

// waitForReplay watches the ledger for the re-attempt and the archive
// for the task's return. The delete happens here (not in teardown) so
// replayed-then-deleted is one honest story beat: the archive's record
// is removed only after the re-execution was observed and re-archived.
func (r *ConsoleStoryRunner) waitForReplay(ctx context.Context, namespace, poisonID string, attemptsBefore int) (replayState, error) {
	state := replayState{}
	deadline := time.Now().Add(replayDeadline)
	for time.Now().Before(deadline) {
		if err := ctx.Err(); err != nil {
			return state, fmt.Errorf("jobs console: caller canceled: %w", err)
		}
		events, err := r.readEvents(ctx, namespace)
		if err != nil {
			return state, err
		}
		state.events = events
		state.replayAttempts = countAttempts(events, poisonID) - attemptsBefore
		if state.replayAttempts >= 1 {
			archived, _, err := r.poisonArchive(ctx, poisonID)
			if err != nil {
				return state, err
			}
			if archived {
				state.reArchived = true
				if err := r.inspector.DeleteTask(ctx, demos.JobsDemoLowQueue, poisonID); err != nil {
					return state, err
				}
				state.deleted = true
				return state, nil
			}
		}
		time.Sleep(storyPollEvery)
	}
	return state, fmt.Errorf("%w (replay attempts observed: %d)", ErrWorkerNotDraining, state.replayAttempts)
}

// readEvents answers the run's whole ledger in execution order.
func (r *ConsoleStoryRunner) readEvents(ctx context.Context, namespace string) ([]demos.JobsDemoEvent, error) {
	raw, err := r.client.LRange(ctx, eventsKey(namespace), 0, -1).Result()
	if err != nil {
		return nil, fmt.Errorf("LRANGE %s: %w", eventsKey(namespace), err)
	}
	events := make([]demos.JobsDemoEvent, 0, len(raw))
	for _, item := range raw {
		event, err := demos.ParseJobsDemoEvent(item)
		if err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	return events, nil
}

// poisonArchive answers whether the poison task sits in the low queue's
// archive, and the final error text the archive carries for it.
func (r *ConsoleStoryRunner) poisonArchive(ctx context.Context, poisonID string) (bool, string, error) {
	archived, err := r.inspector.Archived(ctx, demos.JobsDemoLowQueue)
	if err != nil {
		return false, "", err
	}
	for _, task := range archived {
		if task.ID == poisonID {
			return true, task.LastErr, nil
		}
	}
	return false, "", nil
}

// queueStats is one combined sample of the two observability numbers.
type queueStats struct {
	depth  int
	oldest time.Duration
}

// snapshotStats samples pending depth and oldest-pending age across both
// hosted queues. A failed sample answers zeros — observability must
// never break the story it decorates.
func (r *ConsoleStoryRunner) snapshotStats(ctx context.Context) queueStats {
	stats := queueStats{}
	for _, queue := range []string{demos.JobsDemoQueue, demos.JobsDemoLowQueue} {
		snapshot, err := r.inspector.Stats(ctx, queue)
		if err != nil {
			continue
		}
		stats.depth += snapshot.Pending
		if snapshot.OldestPendingAge > stats.oldest {
			stats.oldest = snapshot.OldestPendingAge
		}
	}
	return stats
}

// countTicks answers how many cron fires the ledger holds.
func countTicks(events []demos.JobsDemoEvent) int {
	ticks := 0
	for _, event := range events {
		if event.Type == demos.JobsTaskTick {
			ticks++
		}
	}
	return ticks
}

// countAttempts answers how many attempts one task recorded.
func countAttempts(events []demos.JobsDemoEvent, taskID string) int {
	attempts := 0
	for _, event := range events {
		if event.TaskID == taskID {
			attempts++
		}
	}
	return attempts
}

// flakySucceeded answers whether the flaky task's ledger rows include a
// success.
func flakySucceeded(events []demos.JobsDemoEvent, flakyID string) bool {
	for _, event := range events {
		if event.TaskID == flakyID && event.Outcome == demos.JobsOutcomeSucceeded {
			return true
		}
	}
	return false
}

// digestDone answers whether the digest task processed.
func digestDone(events []demos.JobsDemoEvent, digestID string) bool {
	for _, event := range events {
		if event.TaskID == digestID {
			return true
		}
	}
	return false
}

// queueRows answers the hosted queues as the run reports them, in a
// fixed order so the payload is stable run to run.
func queueRows() []demos.JobsQueueRow {
	rows := make([]demos.JobsQueueRow, 0, len(DemoQueueWeights))
	for _, name := range []string{demos.JobsDemoQueue, demos.JobsDemoLowQueue} {
		rows = append(rows, demos.JobsQueueRow{Name: name, Weight: DemoQueueWeights[name]})
	}
	return rows
}

// bestEffortTeardown removes whatever the aborted story left behind:
// the three named tasks in whatever state they sit (pending, retrying,
// scheduled, or archived — DeleteTask clears them all) and the run's
// ledger and attempt keys. Errors are swallowed BY DESIGN: this runs on
// the way out of a failed request whose real error must not be masked,
// and the ledger keys' TTL (re-armed on every write) bounds anything a
// concurrent straggler re-creates. The detached context outlives the
// caller's cancellation so the deletes themselves are not abandoned
// mid-flight.
func (r *ConsoleStoryRunner) bestEffortTeardown(ctx context.Context, namespace, flakyID, poisonID, digestID string) {
	cleanupCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	for _, task := range []struct{ queue, id string }{
		{demos.JobsDemoQueue, flakyID},
		{demos.JobsDemoLowQueue, poisonID},
		{demos.JobsDemoQueue, digestID},
	} {
		if err := r.inspector.DeleteTask(cleanupCtx, task.queue, task.id); err != nil {
			continue // already gone, or the broker is unreachable
		}
	}
	// Cron ticks carry broker-assigned ids, so the named deletes above
	// cannot reach them — the payload's Run field is the honest key.
	// Every task left in the demo queues that names this namespace is
	// this run's residue; anything else belongs to another run and is
	// left strictly alone.
	for _, queue := range []string{demos.JobsDemoQueue, demos.JobsDemoLowQueue} {
		var listers []func(string, ...asynq.ListOption) ([]*asynq.TaskInfo, error)
		listers = append(listers,
			r.inspector.inspector.ListPendingTasks,
			r.inspector.inspector.ListScheduledTasks,
			r.inspector.inspector.ListRetryTasks,
		)
		for _, list := range listers {
			entries, err := list(queue)
			if err != nil {
				continue // queue missing or broker unreachable
			}
			for _, entry := range entries {
				payload, err := demos.ParseJobsDemoPayload(entry.Payload)
				if err != nil || payload.Run != namespace {
					continue
				}
				r.inspector.DeleteTask(cleanupCtx, queue, entry.ID)
			}
		}
	}
	r.client.Del(cleanupCtx,
		eventsKey(namespace),
		attemptKey(demos.JobsDemoPayload{Run: namespace, TaskID: flakyID}),
		attemptKey(demos.JobsDemoPayload{Run: namespace, TaskID: poisonID}),
		attemptKey(demos.JobsDemoPayload{Run: namespace, TaskID: digestID}),
	)
}

// jobsPayload builds and encodes one demo task's payload, for the few
// call sites that construct tasks without going through a spec.
func jobsPayload(run, taskID, note string) []byte {
	encoded, err := demos.MarshalJobsDemoPayload(demos.JobsDemoPayload{Run: run, TaskID: taskID, Note: note})
	if err != nil {
		// Marshal of three strings cannot fail; this is a programmer-bug
		// panic per the guide's convention.
		panic(fmt.Sprintf("jobs demo: marshal payload: %v", err))
	}
	return encoded
}
