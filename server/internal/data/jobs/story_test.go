package jobs

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/hibiken/asynq"
	goredis "github.com/redis/go-redis/v9"

	"guide/internal/domain/demos"
	"guide/internal/domain/jobs"
)

// The jobs adapters' tests, in two layers. The pure layer (no broker):
// the demo retry policy's ladder. The live layer walks the FULL console
// story against the running stack Redis — a hosted ServerRunner drains
// the same weighted queues the story enqueues into, and the
// choreography's own guarantees are pinned beat by beat: the retry
// ladder, the archive, the replay's real re-execution, the cron cadence,
// the two observability numbers, and the no-residue teardown. Redis is
// an optional service: when it is unreachable the live tests SKIP
// honestly rather than fail.

// jobsTestRedisAddr resolves the compose Redis from inside the dev
// container, with an escape hatch for other environments.
func jobsTestRedisAddr() string {
	if addr := os.Getenv("GDG_REDIS_ADDR"); addr != "" {
		return addr
	}
	return "redis:6379"
}

// newJobsTestClient dials the stack Redis and skips the test honestly
// when it does not answer — the optional-service discipline applied to
// the suite itself.
func newJobsTestClient(t *testing.T) *goredis.Client {
	t.Helper()
	client := goredis.NewClient(&goredis.Options{
		Addr:         jobsTestRedisAddr(),
		DialTimeout:  2 * time.Second,
		ReadTimeout:  2 * time.Second,
		WriteTimeout: 2 * time.Second,
	})
	pingCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := client.Ping(pingCtx).Err(); err != nil {
		client.Close()
		t.Skipf("guide redis unreachable at %s — live jobs story skipped honestly: %v", jobsTestRedisAddr(), err)
	}
	t.Cleanup(func() { client.Close() })
	return client
}

// jobsTestNamespace mints a throwaway run namespace under the demo
// prefix (unique per test, so parallel stories never share ledgers).
func jobsTestNamespace(t *testing.T) string {
	t.Helper()
	namespace, err := demos.NewJobsDemoNamespace()
	if err != nil {
		t.Fatalf("mint jobs demo namespace: %v", err)
	}
	return namespace
}

// clearJobsTestResidue deletes every key under the demo prefix — the
// defensive belt that runs AFTER the story's own teardown claims,
// whatever happened.
func clearJobsTestResidue(t *testing.T, client *goredis.Client) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var keys []string
	iterator := client.Scan(ctx, 0, demos.JobsDemoKeyPrefix+":*", 100).Iterator()
	for iterator.Next(ctx) {
		keys = append(keys, iterator.Val())
	}
	if err := iterator.Err(); err != nil {
		t.Logf("defensive scan %s: %v", demos.JobsDemoKeyPrefix, err)
		return
	}
	if len(keys) > 0 {
		if err := client.Del(ctx, keys...).Err(); err != nil {
			t.Logf("defensive clear %s: %v", demos.JobsDemoKeyPrefix, err)
		}
	}
}

func TestJobsDemoRetryDelayIsBoundedExponential(t *testing.T) {
	// The policy's whole shape, with no broker in sight: 500ms, 1s, 2s,
	// then the 2s cap — doubling growth, bounded worst case, whatever
	// the error says.
	sentinel := errors.New("upstream answered 503")
	for attempt, want := range map[int]time.Duration{
		1: 500 * time.Millisecond,
		2: 1 * time.Second,
		3: 2 * time.Second,
		4: 2 * time.Second,
		9: 2 * time.Second,
	} {
		if got := DemoRetryDelay(attempt, sentinel); got != want {
			t.Errorf("DemoRetryDelay(%d) = %s, want %s", attempt, got, want)
		}
	}
	if got := DemoRetryDelay(0, sentinel); got != 500*time.Millisecond {
		// attempt 0 is out of the 1-based contract (the adapter
		// translates the broker's 0-based count); the policy still owes
		// the FIRST rung, never a negative-shift panic.
		t.Errorf("DemoRetryDelay(0) = %s, want the 500ms first rung", got)
	}
	if got := DemoRetryDelay(99, sentinel); got != 2*time.Second {
		t.Errorf("DemoRetryDelay(99) = %s, want the 2s cap (overflow answers the cap, not zero)", got)
	}
}

func TestJobsServerRunnerRefusesDoubleStartAndToleratesStopFirst(t *testing.T) {
	client := newJobsTestClient(t)

	runner := NewServerRunner(client)
	// Shutdown before Start is a no-op, never a panic — the composition
	// root may tear down a boot that never got its Start.
	runner.Shutdown()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := runner.Start(ctx); err != nil {
		t.Fatalf("first start must succeed: %v", err)
	}
	if err := runner.Start(ctx); !errors.Is(err, ErrWorkerAlreadyStarted) {
		t.Errorf("second start = %v, want ErrWorkerAlreadyStarted", err)
	}
	runner.Shutdown()
	// Idempotent: the second stop is a no-op, and a restart after a full
	// stop is legal again (one boot → one long run → one shutdown).
	runner.Shutdown()
	if err := runner.Start(ctx); err != nil {
		t.Fatalf("restart after a full stop must succeed: %v", err)
	}
	runner.Shutdown()
}

func TestJobsEnqueuerRefusesDuplicateIDsAndInvalidSpecs(t *testing.T) {
	client := newJobsTestClient(t)
	enqueuer := NewEnqueuer(client)
	t.Cleanup(func() { enqueuer.Close() })
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	namespace := jobsTestNamespace(t)
	spec := jobs.TaskSpec{
		ID: namespace + ":dup", Type: demos.JobsTaskDigest,
		Payload: []byte(`{"run":"` + namespace + `","taskId":"` + namespace + `:dup","note":"uniqueness probe"}`),
		Queue:   demos.JobsDemoQueue, MaxRetry: 0,
	}
	if _, err := enqueuer.Enqueue(ctx, spec); err != nil {
		t.Fatalf("first enqueue: %v", err)
	}
	inspector := NewInspector(client)
	t.Cleanup(func() {
		inspector.DeleteTask(context.Background(), spec.Queue, spec.ID)
		inspector.Close()
	})

	// The caller-minted id IS the uniqueness key: a second live task
	// with the same id is a broker refusal, not a silent double-send.
	if _, err := enqueuer.Enqueue(ctx, spec); err == nil {
		t.Error("duplicate id must be refused while the original still lives")
	} else if !strings.Contains(strings.ToLower(err.Error()), "id") {
		t.Errorf("refusal should name the id conflict: %v", err)
	}

	// Validation happens before the broker is touched at all.
	blank := spec
	blank.ID = ""
	if _, err := enqueuer.Enqueue(ctx, blank); err == nil || !strings.Contains(err.Error(), "id must not be blank") {
		t.Errorf("blank spec refused before the broker: %v", err)
	}
}

func TestJobsInspectorNamesAnUnknownQueue(t *testing.T) {
	client := newJobsTestClient(t)
	inspector := NewInspector(client)
	t.Cleanup(func() { inspector.Close() })
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if _, err := inspector.Stats(ctx, "gdg-no-such-queue"); err == nil {
		t.Error("stats for an unknown queue must be an error naming the queue")
	} else if !strings.Contains(err.Error(), "gdg-no-such-queue") {
		t.Errorf("error must name the queue: %v", err)
	}
	if archived, err := inspector.Archived(ctx, "gdg-no-such-queue"); err == nil {
		// An unknown queue is an honest broker error naming the queue; an
		// EMPTY archive on a known queue is the case that must answer
		// the empty non-nil slice (pinned by the story test).
		t.Errorf("archived listing for an unknown queue = %v, want the named error", archived)
	} else if !strings.Contains(err.Error(), "gdg-no-such-queue") {
		t.Errorf("error must name the queue: %v", err)
	}
}

// TestJobsConsoleStoryWalksEveryBeat is the live choreography: a hosted
// worker drains the same weighted queues the story enqueues into, and
// every guarantee the demo claims is asserted against what the broker
// and the handlers actually answered.
func TestJobsConsoleStoryWalksEveryBeat(t *testing.T) {
	client := newJobsTestClient(t)
	namespace := jobsTestNamespace(t)
	t.Cleanup(func() { clearJobsTestResidue(t, client) })

	worker := NewServerRunner(client)
	if err := worker.Start(context.Background()); err != nil {
		t.Fatalf("host the demo worker: %v", err)
	}
	t.Cleanup(worker.Shutdown)

	runner := NewConsoleStoryRunner(client)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	run, err := runner.RunConsoleStory(ctx, namespace)
	if err != nil {
		t.Fatalf("the console story must walk against the running worker: %v", err)
	}

	if run.Namespace != namespace {
		t.Errorf("namespace = %q, want %q", run.Namespace, namespace)
	}
	if len(run.Queues) != 2 || run.Queues[0].Name != demos.JobsDemoQueue || run.Queues[1].Weight != 1 {
		t.Errorf("queue rows = %+v, want demos(6) then low(1)", run.Queues)
	}

	// The retry ladder: fail, fail, succeed — attempts numbered 1, 2, 3.
	flaky := attemptsOfRun(t, run, run.FlakyID)
	if len(flaky) != 3 {
		t.Fatalf("flaky attempts = %d, want 3 (the budget is exactly enough): %+v", len(flaky), flaky)
	}
	for i, want := range []string{demos.JobsOutcomeFailed, demos.JobsOutcomeFailed, demos.JobsOutcomeSucceeded} {
		if flaky[i].Attempt != i+1 || flaky[i].Outcome != want {
			t.Errorf("flaky attempt %d = %+v, want %d/%s", i, flaky[i], i+1, want)
		}
	}

	// The archive: the poison task spent its whole budget, the archive
	// held it with the error on record, and it is GONE after the story.
	if !run.PoisonArchived || run.PoisonAttempts != demos.JobsPoisonMaxRetry+1 {
		t.Errorf("poison archived=%v attempts=%d, want true/%d", run.PoisonArchived, run.PoisonAttempts, demos.JobsPoisonMaxRetry+1)
	}
	if !strings.Contains(run.PoisonLastErr, "retrying cannot fix this") {
		t.Errorf("the archive's final error must be on record: %q", run.PoisonLastErr)
	}

	// The replay: a real re-execution (ledger grew by one), re-archived,
	// then deleted by the operator.
	if run.ReplayAttempts != 1 || !run.PoisonReArchived || !run.PoisonDeleted {
		t.Errorf("replay = %d/%v/%v, want 1/true/true", run.ReplayAttempts, run.PoisonReArchived, run.PoisonDeleted)
	}

	// The scheduled digest and the cron cadence.
	if digest := attemptsOfRun(t, run, run.DigestID); len(digest) != 1 || digest[0].Outcome != demos.JobsOutcomeSucceeded {
		t.Errorf("digest attempts = %+v, want exactly one success after the delay", digest)
	}
	if run.CronTicks < 2 {
		t.Errorf("cron ticks = %d, want at least 2 while the entry lived", run.CronTicks)
	}
	if !run.SchedulerStopped || !run.LedgerDeleted {
		t.Errorf("teardown flags = stopped:%v ledger-deleted:%v", run.SchedulerStopped, run.LedgerDeleted)
	}

	// The observability pair: peaks observed, and rest answered.
	if run.PeakDepth < 1 {
		t.Errorf("peak depth = %d, want at least one task pending at some sample", run.PeakDepth)
	}
	if run.FinalDepth != 0 {
		t.Errorf("final depth = %d, want 0 at rest", run.FinalDepth)
	}

	// The no-residue belt, re-derived: no task under this namespace in
	// any queue state, and no ledger or counter keys left behind (a
	// straggler tick may briefly re-create the ledger key — the belt
	// polls for its departure instead of asserting a single instant).
	inspector := NewInspector(client)
	t.Cleanup(func() { inspector.Close() })
	assertTasksGone(t, inspector, run)
	assertNamespaceKeysDrained(t, client, namespace)
}

// TestJobsConsoleStoryAbortsHonestlyWithoutAWorker pins the honest
// failure the handler maps to its typed 503: tasks enqueued, no worker
// draining, and the story reports the missing ingredient by name — and
// the best-effort teardown leaves no pending residue behind.
func TestJobsConsoleStoryAbortsHonestlyWithoutAWorker(t *testing.T) {
	client := newJobsTestClient(t)
	namespace := jobsTestNamespace(t)
	t.Cleanup(func() { clearJobsTestResidue(t, client) })

	runner := NewConsoleStoryRunner(client)
	// A caller that gives up early: the context deadline lands inside the
	// observation window, well before the 12s story deadline — no worker
	// is hosting the queues in this test, so nothing can drain them.
	ctx, cancel := context.WithTimeout(context.Background(), 1500*time.Millisecond)
	defer cancel()

	_, err := runner.RunConsoleStory(ctx, namespace)
	if err == nil {
		t.Fatal("a canceled caller must get an error, never a silent success")
	}
	if !strings.Contains(err.Error(), "canceled") && !errors.Is(err, ErrWorkerNotDraining) {
		t.Errorf("the abort must name cancellation or the missing worker: %v", err)
	}

	// The aborted run leaves no pending or scheduled task behind — the
	// three named tasks AND any cron tick this run fired (ticks carry
	// broker-assigned ids; the purge keys on the payload's run field).
	inspector := NewInspector(client)
	t.Cleanup(func() { inspector.Close() })
	for _, task := range []struct{ queue, id string }{
		{demos.JobsDemoQueue, namespace + ":flaky"},
		{demos.JobsDemoLowQueue, namespace + ":poison"},
		{demos.JobsDemoQueue, namespace + ":digest"},
	} {
		if _, err := inspector.inspector.GetTaskInfo(task.queue, task.id); !errors.Is(err, asynq.ErrTaskNotFound) {
			t.Errorf("task %s must be cleaned from queue %s, got: %v", task.id, task.queue, err)
		}
	}
	if strays := namespacedTasksIn(t, inspector, namespace); len(strays) > 0 {
		t.Errorf("the aborted run left tasks behind in the queues: %v", strays)
	}
}

// namespacedTasksIn lists every task in the demo queues' live states
// whose payload names the given run — the belt that catches residue the
// named deletes cannot reach (broker-assigned cron tick ids).
func namespacedTasksIn(t *testing.T, inspector *Inspector, namespace string) []string {
	t.Helper()
	var strays []string
	for _, queue := range []string{demos.JobsDemoQueue, demos.JobsDemoLowQueue} {
		var listers []func(string, ...asynq.ListOption) ([]*asynq.TaskInfo, error)
		listers = append(listers,
			inspector.inspector.ListPendingTasks,
			inspector.inspector.ListScheduledTasks,
			inspector.inspector.ListRetryTasks,
		)
		for _, list := range listers {
			entries, err := list(queue)
			if err != nil {
				t.Fatalf("belt listing queue %s: %v", queue, err)
			}
			for _, entry := range entries {
				payload, err := demos.ParseJobsDemoPayload(entry.Payload)
				if err == nil && payload.Run == namespace {
					strays = append(strays, queue+"/"+entry.ID)
				}
			}
		}
	}
	return strays
}

// attemptsOfRun extracts one task's ledger rows in execution order.
func attemptsOfRun(t *testing.T, run demos.JobsConsoleRun, taskID string) []demos.JobsDemoEvent {
	t.Helper()
	var rows []demos.JobsDemoEvent
	for _, event := range run.Events {
		if event.TaskID == taskID {
			rows = append(rows, event)
		}
	}
	return rows
}

// assertTasksGone verifies the story's named tasks exist in NO queue
// state — completed-and-deleted, archived-then-deleted, all gone.
func assertTasksGone(t *testing.T, inspector *Inspector, run demos.JobsConsoleRun) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for _, task := range []struct{ queue, id string }{
		{demos.JobsDemoQueue, run.FlakyID},
		{demos.JobsDemoLowQueue, run.PoisonID},
		{demos.JobsDemoQueue, run.DigestID},
	} {
		if _, err := inspector.inspector.GetTaskInfo(task.queue, task.id); err == nil {
			t.Errorf("task %s still lives in queue %s after the story's teardown", task.id, task.queue)
		}
	}
	archived, err := inspector.Archived(ctx, demos.JobsDemoLowQueue)
	if err != nil {
		t.Fatalf("final archive listing: %v", err)
	}
	for _, task := range archived {
		if strings.HasPrefix(task.ID, run.Namespace+":") {
			t.Errorf("archive still holds this run's task %q after the story", task.ID)
		}
	}
}

// assertNamespaceKeysDrained polls until every key under this run's
// namespace is gone — the teardown's own claim, re-derived against the
// store with tolerance for an in-flight straggler tick.
func assertNamespaceKeysDrained(t *testing.T, client *goredis.Client, namespace string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		var keys []string
		iterator := client.Scan(ctx, 0, namespace+"*", 100).Iterator()
		for iterator.Next(ctx) {
			keys = append(keys, iterator.Val())
		}
		scanErr := iterator.Err()
		cancel()
		if scanErr == nil && len(keys) == 0 {
			return
		}
		if time.Now().After(deadline) {
			t.Errorf("namespace %s keys still present after the story: %v (scan err: %v)", namespace, keys, scanErr)
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
}
