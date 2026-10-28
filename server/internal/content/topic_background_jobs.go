package content

import (
	"errors"
	"fmt"
)

// This file is topic 29 background-jobs (group media-realtime,
// curriculum order 29): the work that happens AFTER the response is
// sent. The topic grows in three steps — a bounded goroutine pool for
// in-process fan-out (memory-fast, memory-mortal), asynq worker queues
// over the guide's Redis for durable jobs (client enqueue, weighted
// serve, the ServerRunner the composition root hosts), and the
// disciplines that make queues operable: retries with exponential
// backoff (demo jobs:console runs the whole choreography live — a flaky
// task that fails twice and succeeds, a poison task that exhausts its
// budget into the dead-letter archive, a manual replay that re-runs it
// for real, a delayed digest, per-second cron ticks, and the two
// observability numbers sampled while it all runs), the DLQ's
// poison-message honesty, cron scheduling and its drift, email as the
// canonical out-of-band job, and queue depth + oldest-job age as the
// two numbers that page. The terminal companion tours it with curl.

// backgroundJobsTopic builds topic 29.
func backgroundJobsTopic() (Topic, error) {
	var buildErr error
	collect := func(what string, err error) {
		if err != nil {
			buildErr = errors.Join(buildErr, fmt.Errorf("%s: %w", what, err))
		}
	}
	text := func(id, body string) TextBlock {
		block, err := NewTextBlock(id, body)
		collect("text "+id, err)
		return block
	}
	code := func(id, language, caption, source, snippet string) CodeBlock {
		block, err := NewCodeBlock(id, language, caption, source, snippet)
		collect("code "+id, err)
		return block
	}
	demo := func(id, title, description string) DemoBlock {
		block, err := NewDemoBlock(id, title, description)
		collect("demo "+id, err)
		return block
	}
	list := func(id string, ordered bool, items ...string) ListBlock {
		block, err := NewListBlock(id, ordered, items)
		collect("list "+id, err)
		return block
	}
	section := func(id, title, lead string, blocks ...Block) Section {
		assembled, err := NewSection(id, title, lead, blocks)
		collect("section "+id, err)
		return assembled
	}

	sections := []Section{
		// ── 1. Why background ──────────────────────────────────────
		section("why-background", "Why background: latency floors, failure isolation, retries as a first-class concern",
			"Not every unit of work belongs inside the request that caused it. Three structural reasons cover the field — and each one is a property the request path can never grow.",
			text("bg-latency",
				"The first reason is the latency floor. An HTTP request has a budget measured in milliseconds-to-seconds, and some work simply cannot fit: an SMTP round-trip to a slow relay takes seconds on a bad day, transcoding an uploaded video takes minutes, regenerating a month of report aggregates can take longer than the client's timeout, and a webhook consumer three hops away answers when it answers. Doing that work inline means the request's latency floor is the WORK's latency floor — the slowest third party in the system sets the API's p99 forever. Moving it out-of-band caps the request at the cost of recording the decision (a row, a stream entry, a queued task — milliseconds), and the work finishes on its own schedule. The trade is explicit: the caller no longer waits for the RESULT, so the result must be delivered another way — a follow-up request, an email, a realtime push, or nothing at all when the user only needs eventual consistency."),
			text("bg-isolation",
				"The second reason is failure isolation. A request that calls ten dependencies fails when any of them is having a bad day, and a flaky payment provider becomes your outage. A job system moves the flakiness somewhere survivable: the request succeeds when the job is RECORDED, and the job's own failures are the queue's business — scheduled for retry, held with their error text, never propagated to a user who already got a 201. Isolation also runs the other direction: a burst of work that would pile up as concurrent requests (each holding memory, a connection, a timeout) becomes depth in a queue, and depth is back-pressure you can see and shed deliberately instead of load you discover through a memory graph. The request path stays for work whose answer the caller needs NOW; everything else earns a queue."),
			text("bg-retries",
				"The third reason is the one teams discover last: a failed request is GONE, but a failed job is still on file. If the request fails, the caller must notice, decide, and re-send — and a browser tab that closed at the wrong moment means the work never happens. A queued task that fails is rescheduled by the broker with a growing delay, retried until its budget is spent, and finally parked in a dead-letter archive with the last error attached, where a human can read it and decide. Retry stops being error-handling code pasted at every call site and becomes a property of the transport — first-class, observable, and identical for every job in the system. That is the deepest difference between ``call it later'' and ``queue it'': the first is a hope, the second is a record with a retry policy."),
			list("bg-signals",
				false,
				"Latency floor — the work cannot finish inside the request budget (email, transcode, aggregate), so inline execution makes the API as slow as the work.",
				"Failure isolation — a flaky third party must not fail the user's request; record the job, retry it on the queue's schedule.",
				"Retries as a first-class concern — the work must survive failure without caller code, with backoff, a budget, and a dead-letter record.",
				"Burst absorption — arrivals come faster than completion for a while, and depth with back-pressure beats concurrent request pile-up.",
				"Scheduled recurrence — the work must fire on a clock (digests, cleanup, sweeps) whether or not anyone is making requests.",
				"The counter-signal — if the caller needs the result to answer, it is not a job: keep it in the request, or make the request cheap and the result asynchronous by design.",
			),
			text("bg-map",
				"This topic climbs the same ladder the ecosystem does. First the in-process answer: a bounded goroutine pool over channels — the goroutines-and-channels topic built one, and it is exactly right when the work lives and dies with the process. Then the durable answer: asynq, the Redis-backed task queue this guide runs in production posture — a client that enqueues, a worker that serves weighted queues, and the whole retry/archive/replay/observability discipline around them. Every line quoted below is the guide's own running code: the ports in ``internal/domain/jobs'', the asynq adapters in ``internal/data/jobs'', and the live console demo that choreographs all of it against the stack's Redis on every request."),
		),
		// ── 2. Goroutine pools ─────────────────────────────────────
		section("goroutine-pools", "Goroutine pools: bounded workers over channels — and when a pool is enough",
			"Go makes starting a goroutine free, which makes bounding them a decision. The pool pattern — N workers parking on one jobs channel — is the whole in-process answer, and its limits are the motivation for everything after it.",
			text("pool-shape",
				"The pattern is five moves: make a jobs channel and a results channel, start a FIXED number of worker goroutines that all receive from jobs, feed the channel from wherever work arrives, close the jobs channel when the feeding is done, and collect exactly as many results as you fed. The bound is the point: a pool of three workers processing a batch of eight jobs runs at most three transformations concurrently no matter how the arrivals bunch up, because the channel IS the queue and Go channels are FIFO and fair. Every value a worker touches travels a channel — jobs in, results out — so there is no shared counter, no mutex on the hot path, and the race detector stays quiet because the design, not the discipline, keeps the data disjoint. This guide's runtime topic ships exactly this pool as its live demo, and it remains the right answer whenever the work is in-process."),
			code("pool-worker-go", "go",
				"One pool worker: both blocking operations select-guarded on ctx, so cancellation unfailingly unparks it — waiting for work or waiting to deliver.",
				"server/internal/domain/demos/concurrency.go",
				"func poolWorker(ctx context.Context, id int, jobs <-chan poolJob, results chan<- poolResult, stats chan<- WorkerRow) {\n"+
					"	worker := fmt.Sprintf(\"worker-%d\", id)\n"+
					"	handled := 0\n"+
					"	defer func() { stats <- WorkerRow{WorkerID: worker, JobsDone: handled} }()\n"+
					"	for {\n"+
					"		select {\n"+
					"		case job, ok := <-jobs:\n"+
					"			if !ok {\n"+
					"				return\n"+
					"			}\n"+
					"			select {\n"+
					"			case results <- poolResult{JobID: job.ID, Payload: job.Payload, Output: transform(job.Payload)}:\n"+
					"				handled++\n"+
					"			case <-ctx.Done():\n"+
					"				return\n"+
					"			}\n"+
					"		case <-ctx.Done():\n"+
					"			return\n"+
					"		}\n"+
					"	}\n"+
					"}"),
			text("pool-fanout",
				"The fan-out side starts the fixed fleet before the first job exists — that parked-alive window is what makes a pool a POOL rather than a loop: the workers are capacity standing by, not cost incurred per job. The run topic's choreography starts three workers over eight jobs and reports the goroutine count before, during, and after as its own proof — three parked goroutines during the feed, zero after the join. Two details make the pattern production-grade rather than a textbook sketch. First, the feeder itself is select-guarded: if the collector goes away, feeding stops instead of blocking forever on a full channel. Second, the workers' exit is orchestrated by the CLOSE of the jobs channel — a closed channel delivers the zero value with ok=false to every receiver, which is Go's built-in broadcast: one close retires the whole fleet, and the WaitGroup joins them before anyone reads the tallies."),
			code("pool-fanout-go", "go",
				"The fleet: a fixed worker count parking on an empty jobs channel — capacity standing by, not goroutine-per-job.",
				"server/internal/domain/demos/concurrency.go",
				"	jobs := make(chan poolJob)\n"+
					"	results := make(chan poolResult)\n"+
					"	stats := make(chan WorkerRow, poolWorkers)\n"+
					"	var wg sync.WaitGroup\n"+
					"	for id := 1; id <= poolWorkers; id++ {\n"+
					"		wg.Add(1)\n"+
					"		go func() {\n"+
					"			defer wg.Done()\n"+
					"			poolWorker(ctx, id, jobs, results, stats)\n"+
					"		}()\n"+
					"	}"),
			text("pool-boundary",
				"Now the honest boundary — everything a pool is NOT. A pool is memory, not durability: kill the process mid-batch and the jobs sitting in the channel are gone, the in-flight jobs are gone, and nothing anywhere records that they were owed. A pool has no retry: a worker that returns an error has to be re-fed by whatever code noticed, which is bespoke error handling at every call site — the exact thing a queue makes first-class. A pool has no cross-process story: two server replicas each have their own channel, and work submitted to one cannot drain on the other. And a pool has no clock: nothing schedules a job to run later or every hour. When ANY of those — survive a crash, retry by policy, share across processes, run later — matters, the pool graduates into a broker-backed queue, and the rest of this topic is that graduation."),
		),
		// ── 3. asynq worker queues ─────────────────────────────────
		section("asynq-worker-queues", "asynq worker queues: client, serve, weighted queues over Redis",
			"asynq is the durable step: tasks live in Redis, survive process death, and drain through workers that may be anywhere. The guide's split keeps asynq in the data layer behind domain ports — the broker is a detail, the vocabulary is not.",
			text("aq-model",
				"The model has three participants. A CLIENT enqueues: it hands the broker a typed payload plus routing and policy options, and gets a receipt. A BROKER (Redis, with asynq's keys and Lua scripts doing the atomic moves between states) stores each task as one of five states — pending, active, scheduled, retry, archived — and every state transition is a Redis operation away from inspection. A SERVER (the worker) polls the queues, claims pending tasks, runs them through a handler mux keyed on the task's type string, and moves each outcome: success deletes the task (or retains it briefly if configured), failure schedules a retry, exhaustion archives it. Workers and clients never address each other — they meet at the queue, which is why a worker can live in a different process or a different machine, and why the demo's choreography works with the hosted worker draining whatever any request enqueues."),
			code("aq-spec-go", "go",
				"The domain's task specification: one struct carrying WHO runs it, WHAT it carries, and HOW the broker treats it — the whole broker conversation without an asynq import.",
				"server/internal/domain/jobs/jobs.go",
				"type TaskSpec struct {\n"+
					"	ID string\n"+
					"	Type string\n"+
					"	Payload []byte\n"+
					"	Queue string\n"+
					"	MaxRetry int\n"+
					"	ProcessIn time.Duration\n"+
					"	Timeout time.Duration\n"+
					"}"),
			text("aq-id",
				"Two of those fields deserve their own paragraph. The ID is CALLER-MINTED, not broker-assigned — deliberately. It is the dedup and forensics key across every retry and every archive of the task: enqueue the same id twice while the first still lives and the broker refuses the duplicate instead of silently double-sending, which is what makes ``retry the whole enqueue'' a safe recovery move after a network blip. It is also the handle an operator (or a demo) uses to find, replay, or delete the task later — a broker-assigned uuid could serve forensics but never idempotency, because the caller could never re-derive it. The Payload is opaque bytes the handler owns the format of: small, self-describing JSON in this guide. The queue carries work, not objects — if the payload needs the whole database row, the job should carry the row's ID and fetch fresh state when it runs."),
			code("aq-enqueue-go", "go",
				"The adapter maps the spec one-for-one onto broker options — id to uniqueness, queue to destination, budget to MaxRetry, delay to the scheduled state, timeout to the per-attempt bound.",
				"server/internal/data/jobs/enqueuer.go",
				"	options := []asynq.Option{\n"+
					"		asynq.TaskID(spec.ID),\n"+
					"		asynq.Queue(spec.Queue),\n"+
					"		asynq.MaxRetry(spec.MaxRetry),\n"+
					"	}\n"+
					"	if spec.ProcessIn > 0 {\n"+
					"		options = append(options, asynq.ProcessIn(spec.ProcessIn))\n"+
					"	}\n"+
					"	if spec.Timeout > 0 {\n"+
					"		options = append(options, asynq.Timeout(spec.Timeout))\n"+
					"	}\n"+
					"	task := asynq.NewTask(spec.Type, spec.Payload)\n"+
					"	info, err := e.client.EnqueueContext(ctx, task, options...)"),
			text("aq-weights",
				"Serving is where queue design becomes policy. A worker declares the queues it drains and a WEIGHT per queue, and the poller samples them proportionally: the guide's worker hosts ``demos'' at weight 6 and ``low'' at weight 1, so whenever both hold work the main queue is polled six times in seven. Weights are the production default because they are FAIR: a busy critical queue cannot starve the maintenance lanes, and a flood of low-priority scrub work cannot steal every cycle from the lane that pages you. Strict priority (``critical first, always'') exists for the rare case where starvation is the intended semantics — most systems should not intend it. The other lever is Concurrency: the worker's lane count, the direct descendant of the pool's fixed fleet — two lanes means at most two handlers run at once, whatever the queue depth behind them, and each lane is a goroutine the worker owns and reuses."),
			code("aq-serve-go", "go",
				"The hosted worker: weighted queues, a lane count, the retry policy seam, and the base context every handler context derives from — started and stopped by the composition root.",
				"server/internal/data/jobs/worker.go",
				"	r.server = asynq.NewServerFromRedisClient(r.client, asynq.Config{\n"+
					"		Concurrency:    demoWorkerConcurrency,\n"+
					"		Queues:         DemoQueueWeights,\n"+
					"		RetryDelayFunc: r.retryDelay,\n"+
					"		ShutdownTimeout: 5 * time.Second,\n"+
					"		BaseContext: func() context.Context {\n"+
					"			return r.baseCtx\n"+
					"		},\n"+
					"	})\n"+
					"	if err := r.server.Start(NewDemoMux(r.client)); err != nil {\n"+
					"		r.server = nil\n"+
					"		return fmt.Errorf(\"jobs worker start: %w\", err)\n"+
					"	}"),
			text("aq-lifecycle",
				"The ServerRunner wrapper is small but it is the part the composition root actually touches, and its shape is the family's lifecycle discipline: Start is non-blocking (the pollers run on their own goroutines), refuses a second start with a typed error instead of silently hosting two workers on the same queues, and Shutdown drains in-flight handlers inside a bounded timeout and is safe to call twice or before any start. The worker shares the process's single go-redis client — asynq is built on the same Redis the guide already runs for caching and messaging, so there is no new infrastructure, one connection pool, and one address to configure. That reuse is also the deployment story in miniature: the broker is Redis, the workers are your Go binaries, and scaling workers means running more of them against the same queues — the state machine in Redis arbitrates who claims what."),
		),
		// ── 4. Retries and backoff ─────────────────────────────────
		section("retries-and-backoff", "Retries and backoff: the ladder, the budget, timeout vs deadline",
			"Failure handling is where a queue earns its keep: the task comes back with a growing delay until the budget is spent. The live console runs the whole ladder below.",
			text("rt-ladder",
				"The ladder is exponential backoff: when an attempt fails, wait a short delay and try again, doubling the delay each failure, capped so the worst case stays bounded. The shape encodes three beliefs. Fast first retry — most failures are transient blips (a connection reset, a 503 from a restarting dependency), and they clear in the first few hundred milliseconds. Growing gaps — if the failure is NOT a blip, hammering the dependency every 500ms is how a queue becomes an attack, so the delay doubles to give the downstream time to recover. Bounded worst case — a cap keeps the ladder humane: the guide's demo policy runs 500ms, 1s, 2s, 2s… rather than asynq's production default (sidekiq's formula — n to the fourth power plus 15 seconds plus jitter — which would make a three-attempt demo take half a minute). The policy is a seam, not a constant: the server takes a function, and the demo plugs in the fast ladder so the choreography is visible in seconds."),
			code("rt-policy-go", "go",
				"The demo's bounded-exponential policy: doubling rungs, a hard cap, and a first-rung answer for any input the contract forbids — never a negative-shift panic.",
				"server/internal/data/jobs/handlers.go",
				"func DemoRetryDelay(attempt int, _ error) (delay time.Duration) {\n"+
					"	const base = 500 * time.Millisecond\n"+
					"	const capAt = 2 * time.Second\n"+
					"	if attempt < 1 {\n"+
					"		return base\n"+
					"	}\n"+
					"	shifted := base << (attempt - 1)\n"+
					"	if shifted > capAt || shifted <= 0 {\n"+
					"		return capAt\n"+
					"	}\n"+
					"	return shifted\n"+
					"}"),
			text("rt-adapter",
				"A trap hides in that policy's parameter, and it is worth meeting here because it is the kind of bug a live test catches and a review does not. The domain's vocabulary counts attempts 1-BASED — attempt 1 is the first failure — but asynq hands the delay function the RETRY COUNT so far, which is 0 on the first failure. Wire the domain policy straight onto the broker signature and the first failure computes a shift of -1, which is a run-time panic inside the worker, on every queue, the first time anything fails. The adapter's whole job is the translation: one +1, applied once, at the seam — and a defensive first rung for any input below the contract. The lesson generalizes: every broker boundary is a vocabulary boundary, and the adapter layer is where counts, ids, and errors get translated so the domain never learns the broker's dialect."),
			code("rt-adapter-go", "go",
				"The seam that keeps the policy unit-testable with two numbers and an error — and translates the broker's 0-based count onto the domain's 1-based attempts.",
				"server/internal/data/jobs/worker.go",
				"func adaptRetryDelay(policy func(int, error) time.Duration) asynq.RetryDelayFunc {\n"+
					"	return func(n int, err error, _ *asynq.Task) time.Duration {\n"+
					"		return policy(n+1, err)\n"+
					"	}\n"+
					"}"),
			text("rt-budget",
				"MaxRetry is the budget: the first execution plus MaxRetry attempts is all the ladder gets, and when it is spent the task is archived (the next section). Set it from the work's economics, not from a feeling — every retry is another delivery of the same load to the same struggling dependency, so a budget of 25 on a task that calls a rate-limited API is a self-inflicted denial of service with extra steps. Three properties of the budget are worth internalizing. It is PER TASK, declared at enqueue time, so different jobs in the same queue can carry different persistence. It survives redelivery: the broker, not the handler, counts attempts, and the count rides the task through every state. And it is spent in ATTEMPTS, not time — if you need ``keep trying for an hour'' you compose ProcessIn, MaxRetry, and the delay ladder to approximate it, which is honest: time-based budgets are a scheduler, and a task queue is not one."),
			text("rt-timeout",
				"Two time bounds guard every attempt, and conflating them is a classic outage. TIMEOUT bounds ONE attempt: the handler gets a context that dies after it, a stuck attempt fails fast, and every retry starts a fresh budget — this is the TaskSpec's Timeout, mapped onto ``asynq.Timeout''. DEADLINE bounds the WHOLE job: a receipt email that arrives three days late is worse than one that never arrives, so the scheduled state, the retry delays, and every remaining attempt must fit inside one absolute horizon. The guide's spec carries the per-attempt timeout and the process-in delay; a full deadline composes them (enqueue a deadline-check task, or compute the last viable ProcessIn). The rule of thumb: timeout protects the worker from the dependency, deadline protects the business from the worker — and a job with neither is a job that can hang forever while reporting itself busy."),
			code("rt-handler-go", "go",
				"The flaky handler: a durable attempt counter keyed by the task's caller-minted id — duplicate delivery counts itself honestly instead of restarting the ladder.",
				"server/internal/data/jobs/handlers.go",
				"		attempt, err := incrAttempt(ctx, client, payload)\n"+
					"		if err != nil {\n"+
					"			return err\n"+
					"		}\n"+
					"		if attempt <= demos.JobsFlakyFailFirst {\n"+
					"			if err := recordDemoEvent(ctx, client, payload, demos.JobsTaskFlaky, attempt, demos.JobsOutcomeFailed); err != nil {\n"+
					"				return err\n"+
					"			}\n"+
					"			return fmt.Errorf(\"flaky task %s: attempt %d of %d failed (a flaky upstream, by design)\",\n"+
					"				payload.TaskID, attempt, demos.JobsFlakyFailFirst+1)\n"+
					"		}\n"+
					"		if err := recordDemoEvent(ctx, client, payload, demos.JobsTaskFlaky, attempt, demos.JobsOutcomeSucceeded); err != nil {\n"+
					"			return err\n"+
					"		}\n"+
					"		return nil"),
			text("rt-idempotent",
				"One property the ladder FORCES onto every handler, stated once because it is the queue's law: at-least-once delivery means idempotent handlers. A worker can crash between finishing the work and reporting success — the broker re-delivers, and the task runs again. A claim can race a lease expiry — it runs again. The flaky handler shows the honest pattern: its attempt counter is an INCR on a redis key named for the task's stable id, so the second delivery of the same task counts itself as the next attempt instead of starting the ladder over; the counter IS the ``have we met before'' state. Real handlers go further — a send-email task stamps ``already sent for this event id'' before it hits SMTP, a billing task keys its writes by the event id so re-running is a no-op. Design every handler as if it will run twice, because on its worst day it will."),
			demo("jobs:console", "The jobs console: the whole choreography, live",
				"One request drives the real thing against the stack's Redis and the hosted worker: the flaky task's fail-fail-succeed ladder under backoff, the poison task's budget exhausting into the archive, a manual replay that re-runs it for real, the delayed digest maturing out of the scheduled state, per-second cron ticks, and the two observability numbers sampled while it all runs — every proof sentence derived from the run's own ledger."),
		),
		// ── 5. DLQ and replay ──────────────────────────────────────
		section("dlq-and-replay", "The dead-letter queue: archive, replay, and poison-message honesty",
			"A task that can never succeed must stop retrying and land somewhere a human can read. The archive is that somewhere, and replay is its manual re-run — real execution, not a recording.",
			text("dlq-archive",
				"When the budget is spent, the broker moves the task to the archive — asynq's RETIRED set, kept by default, queryable by queue, each entry carrying the task's payload, its attempt count, and the FINAL ERROR TEXT. That last field is the whole point: the archive is where you go at 3 a.m. to learn WHY, and a dead-letter record without its error is a paperweight. The archive is deliberately not silent and not loud: archived tasks do not retry, do not get deleted automatically (unless a retention policy says so), and do not block anything — they wait. The operational discipline is a regular sweep: list the archive, read the errors, decide. Every archived task is one of three decisions — the error is understood and the payload is fine (replay it), the payload was wrong (fix it or delete the task), or the code was wrong (fix, deploy, then replay)."),
			code("dlq-list-go", "go",
				"The operate port's archive listing: newest first, mapped onto the domain's row shape — the empty archive answers an empty slice, never nil.",
				"server/internal/data/jobs/inspector.go",
				"	entries, err := i.inspector.ListArchivedTasks(queue)\n"+
					"	if err != nil {\n"+
					"		return nil, fmt.Errorf(\"list archived tasks in queue %q: %w\", queue, err)\n"+
					"	}\n"+
					"	rows := make([]jobs.ArchivedTask, 0, len(entries))\n"+
					"	for _, entry := range entries {\n"+
					"		rows = append(rows, jobs.ArchivedTask{\n"+
					"			ID:       entry.ID,\n"+
					"			Type:     entry.Type,\n"+
					"			Queue:    entry.Queue,\n"+
					"			Retried:  entry.Retried,\n"+
					"			MaxRetry: entry.MaxRetry,\n"+
					"			LastErr:  entry.LastErr,\n"+
					"		})\n"+
					"	}\n"+
					"	return rows, nil"),
			text("dlq-replay",
				"Replay is the archive's exit back into life: one call moves the archived task to PENDING, and the worker executes it again — for real, code and all, which is exactly why it works as a deployment tool. Ship the fix, replay the backlog, the fixed code processes the old payloads; nothing was recorded to replay back. Two properties make replay safe rather than reckless. The retry counters stay where the failure left them, so a replayed task that fails again re-archives IMMEDIATELY — replay is one more attempt, not a fresh budget, and a poison message cannot ping-pong forever. And replay rides the same uniqueness as everything else: the task keeps its caller-minted id, so a replayed task is still dedupable and still findable. The demo's choreography shows the honest sequence: replay, watch the ledger grow by exactly one attempt, watch it fail the same way, watch it re-archive — then the operator deletes it, because replay is a decision and the archive is the record."),
			code("dlq-replay-go", "go",
				"The whole replay operation: one broker call, wrapped and named — archived to pending, the worker takes it from there.",
				"server/internal/data/jobs/inspector.go",
				"func (i *Inspector) Replay(ctx context.Context, queue, id string) error {\n"+
					"	if err := i.inspector.RunTask(queue, id); err != nil {\n"+
					"		return fmt.Errorf(\"replay task %q in queue %q: %w\", id, queue, err)\n"+
					"	}\n"+
					"	return nil\n"+
					"}"),
			text("dlq-poison",
				"The poison-message honesty is the section's real lesson. A poison message is a task that fails EVERY time — the payload the code cannot parse, the event id that references a deleted row, the upstream that answers 500 deterministically. The retry ladder cannot fix it (the failure is not transient), the archive cannot fix it (it can only hold it), and replay cannot fix it (it re-runs the same code over the same bytes). Pretending otherwise — unbounded retries, auto-replay loops, archive purges on a timer — just hides the problem or feeds it. The honest posture is the one the demo's poison handler embodies: fail with an error text that says what a human needs (``upstream answered 500 for attempt 2 — retrying cannot fix this''), land in the archive, and WAIT for a decision. A poison message in the archive is the system working exactly as designed: the queue kept its promise, the operator keeps theirs."),
		),
		// ── 6. Scheduling with cron ────────────────────────────────
		section("scheduling-with-cron", "Scheduling with cron: periodic tasks and the drift you inherit",
			"Periodic work is not a new kind of task — it is a clock that enqueues. A scheduler process owns the entries, fires them on their spec, and the same queues handle the results.",
			text("cron-model",
				"asynq's Scheduler is a separate participant from the worker: it holds a registry of entries, each entry a cron SPEC plus a task prototype plus enqueue options, and on every fire it ENQUEUES a new task — the worker does not know or care that a clock was involved. The spec syntax is robfig/cron's: five-field crontab (``0 3 * * *'' — 03:00 every night), the ``@every 1s'' duration form the demo's ticker uses, and the named schedules (``@daily''). Two properties of the model deserve emphasis. Each fire is a NEW task with a broker-assigned id — a caller-minted id would collide with itself every period, which is why the demo's tick payload carries a stable LOGICAL id (the run namespace) for the ledger while the broker mints fresh task ids. And the entry's task is a PROTOTYPE: the scheduler enqueues a copy per fire, so entries can be registered and unregistered at runtime without touching anything in flight."),
			code("cron-register-go", "go",
				"Registering the demo's per-second entry: spec, task prototype, and the queue it fires into — one call, one entry id to keep for the unregistration.",
				"server/internal/data/jobs/story.go",
				"	entryID, err := scheduler.Register(demos.JobsDemoCronSpec,\n"+
					"		asynq.NewTask(demos.JobsTaskTick, jobsPayload(namespace, namespace+\":tick\", \"one cron fire\")),\n"+
					"		asynq.Queue(demos.JobsDemoQueue))\n"+
					"	if err != nil {\n"+
					"		return demos.JobsConsoleRun{}, fmt.Errorf(\"register cron entry %q: %w\", demos.JobsDemoCronSpec, err)\n"+
					"	}"),
			text("cron-drift",
				"Drift is the scheduling truth nobody escapes: a cron entry means ``not before'', not ``exactly at''. The scheduler wakes on the spec's boundary and ENQUEUES — the task still waits behind queue depth and worker lanes, so a nightly 03:00 job on a busy queue runs at 03:00 plus however long the pending line is. Long-running periods drift the other way: a 24-hour ``@every 24h'' entry measures from the previous FIRE, so every execution's overrun shifts the next one later forever. The defenses are structural. For strict schedules, use five-field crontab specs (anchored to wall-clock times) rather than ``@every'' durations. For overrun safety, make the handler hold a run lock or check a last-run marker so a slow fire and a fresh fire cannot double-process one period — the same idempotency law the retry section taught, applied to time. And for ``missed while the scheduler was down'', accept the truth: a down scheduler skips entries silently, so anything that MUST catch up needs its own catch-up logic (compare a high-water mark when it runs), not a cron guarantee."),
			text("cron-lifecycle",
				"The lifecycle discipline mirrors the worker's, because a scheduler is a long-running participant too: start it once per process, stop it on shutdown, and give it its own connection. The demo's story runner owns a per-request scheduler — registered, started, and stopped INSIDE one HTTP request — and its teardown order is the teachable part: the entry is UNREGISTERED and the scheduler shut down BEFORE any keys are deleted, because ticks already enqueued may still be in flight on the worker, and a tick that lands after its ledger was deleted would re-create the key the cleanup just removed. Unregister, then stop, then give the stragglers a window to drain, then delete — the same ordering any shutdown that owns both a clock and a queue must respect."),
			code("cron-stop-go", "go",
				"The teardown order: unregister the entry (no new fires), stop the clock, THEN let the cleanup delete — a tick can never race the cleanup and resurrect a key.",
				"server/internal/data/jobs/story.go",
				"	if err := scheduler.Unregister(entryID); err != nil {\n"+
					"		return demos.JobsConsoleRun{}, fmt.Errorf(\"unregister cron entry %q: %w\", entryID, err)\n"+
					"	}\n"+
					"	scheduler.Shutdown()\n"+
					"	run.SchedulerStopped = true"),
		),
		// ── 7. Email as the canonical job ─────────────────────────
		section("email-as-a-job", "Email as the canonical job: the out-of-band send, end to end",
			"Every guide teaches background jobs with email because email has every property a job needs: slow, flaky, third-party, and needed exactly once-ish. Here is the whole pattern in one walk.",
			text("email-why",
				"An SMTP relay answers in hundreds of milliseconds on a good day and tens of seconds on a bad one; it rate-limits, it greylists (deliberately refusing the first attempt to see if you are a real sender — a retry-shaped protocol), and it is nobody's code you control. Sending mail inline in the request means every one of those properties becomes your API's latency and your endpoint's failure rate. The out-of-band shape is the fix, and it is worth writing down as a sequence because it IS the generic job pattern: the request handler commits the BUSINESS change (order confirmed) and the DECISION to send (a queued task) together — ideally atomically, the transactional-outbox pattern the messaging topic taught — then answers the client 201 immediately; the worker picks up the task, renders the email, sends it through the relay; a failure retries on the ladder (greylisting LOVES a retry ladder); exhaustion archives the task with the relay's error attached, where an operator reads it and replays it after the incident. The user's request never waited, the relay's bad day never became a 500, and the email either arrived or is on file with a reason."),
			code("email-shape-go", "go",
				"The digest handler IS the canonical shape: parse the payload, do a small bounded piece of work, succeed — the template a real send-the-receipt-email handler grows from.",
				"server/internal/data/jobs/handlers.go",
				"func digestHandler(client *goredis.Client) func(context.Context, *asynq.Task) error {\n"+
					"	return func(ctx context.Context, task *asynq.Task) error {\n"+
					"		payload, err := demos.ParseJobsDemoPayload(task.Payload())\n"+
					"		if err != nil {\n"+
					"			return fmt.Errorf(\"digest task %s: %w\", taskIDFrom(ctx), err)\n"+
					"		}\n"+
					"		return recordDemoEvent(ctx, client, payload, demos.JobsTaskDigest, retryCountFrom(ctx)+1, demos.JobsOutcomeSucceeded)\n"+
					"	}\n"+
					"}"),
			text("email-payload",
				"Design the task like an email task deserves. The payload carries the MINIMUM the handler cannot re-derive — the event id and the recipient reference — not a serialized snapshot of the whole order, because a snapshot goes stale and a re-fetch goes to the source of truth when the retry finally runs (the queue carries work, not objects). The task id is the business event's id (``email:order-1001:receipt''), which gives dedup for free: at-least-once delivery may re-run a send, and the SECOND run must find its own footprints — a ``sent'' marker keyed by the event id — and no-op instead of double-sending. That is the idempotency law wearing an email costume: the send itself is the side effect, the marker is the dedup, and the handler checks the marker BEFORE the side effect, not after. Greylisting aside, set the budget from the mail's shelf life: a password-reset mail that retries for six hours is a support ticket; three attempts over ten seconds with a clear archive error is an honest system."),
			text("email-scheduled",
				"The same shape covers scheduled mail — and shows how the sections compose. A weekly digest is a cron entry whose task prototype is the digest job (the demo literally runs this: a per-second tick enqueues into the same weighted queue everything else uses). A ``send in one hour'' confirmation mail is ProcessIn putting the task in the scheduled state, promoted by the broker's forwarder without any sleeping code. A campaign to a million addresses is one seed task whose handler enqueues the batch as many small tasks — the queue pattern recursively, keeping every unit small and retryable. The composition is the point: enqueue, retry, archive, replay, and schedule are five orthogonal mechanisms, and every mail-shaped (or report-shaped, or cleanup-shaped) problem in a system is some projection of them."),
		),
		// ── 8. The two observability numbers ──────────────────────
		section("observability-numbers", "The two observability numbers: queue depth and oldest-job age",
			"A background system is judged by two numbers, and they are not throughput. Depth is how much work is waiting; oldest-job age is how long the oldest of it has waited — together they are the queue's entire vital-sign story.",
			text("obs-two",
				"Depth (pending count) trending up means the system is falling behind: arrivals exceed completions, and every minute of it adds latency your users will eventually feel — depth is the EARLY warning, visible long before any user-facing symptom. Oldest-job age (asynq's queue latency — the age of the oldest pending task) is the symptom made honest: a depth of 5,000 on a queue that drains in a minute is noise; an oldest job aged 40 minutes means SOMETHING has waited 40 minutes, and that is a page. The two together discriminate the failure modes: depth up AND age up means workers are down or too few (nothing is draining); depth up with age FLAT means a burst is being absorbed at steady latency (probably fine, maybe scale anyway); depth flat with age up is rare and strange — usually a scheduled backlog or a stuck claim; both at zero means the queue is at rest. That is the whole diagnostic, two numbers, no dashboards required."),
			code("obs-stats-go", "go",
				"The operate port's vital signs: one broker call mapped onto the snapshot — the depth is Pending, the oldest-job age is the queue's latency.",
				"server/internal/data/jobs/inspector.go",
				"	info, err := i.inspector.GetQueueInfo(queue)\n"+
					"	if err != nil {\n"+
					"		return jobs.QueueSnapshot{}, fmt.Errorf(\"stats for queue %q: %w\", queue, err)\n"+
					"	}\n"+
					"	return jobs.QueueSnapshot{\n"+
					"		Name:             info.Queue,\n"+
					"		Size:             info.Size,\n"+
					"		Pending:          info.Pending,\n"+
					"		Active:           info.Active,\n"+
					"		Scheduled:        info.Scheduled,\n"+
					"		Retry:            info.Retry,\n"+
					"		Archived:         info.Archived,\n"+
					"		OldestPendingAge: info.Latency,\n"+
					"		Processed:        int64(info.ProcessedTotal),\n"+
					"		Failed:           int64(info.FailedTotal),\n"+
					"	}, nil"),
			text("obs-alert",
				"The alerting rules fall out of the diagnostic, and they are deliberately boring: page on oldest-job age over SLO (a job older than your promise to users is a breach in progress, whatever the depth says), and alert on sustained depth growth (a trend, not a value — five minutes of monotonic rise is a worker problem forming). Track the per-queue processed and failed totals as RATES for capacity conversations, but do not page on them: a burst of failures with a flat age is the retry ladder doing its job (the archive sweep catches the ones that exhaust). The demo's story samples both numbers across every poll while the choreography runs — peak depth, peak age, and the at-rest answer after everything drained — because the honest way to teach a metric is to watch it move through a real workload, not to define it."),
			text("obs-ceiling",
				"One number deliberately did not make the list: worker CPU or goroutine count. Those are inputs you SET (the lane count, the process count), not outcomes you page on — a queue system is interesting precisely because it decouples arrival from completion, so the signals that matter are the OUTCOME signals (depth, age) plus the failure record (archive, error rates). When depth climbs, the response levers are exactly three: more workers (more processes, more lanes), less work per job (smaller tasks, batching re-thought), or less arrival (shed, defer, rate-limit the enqueue side). Which lever is right is a business question — but WHICH lever exists is now a system property, and that is what a queue bought you: a place where load becomes a decision instead of an accident."),
		),
		// ── 9. Terminal companion ──────────────────────────────────
		section("terminal-companion", "Terminal companion: the jobs curl tour",
			"One endpoint walks the entire choreography — enqueue, backoff, archive, replay, cron, observability, teardown — and curl runs it in seconds. The story's JSON is the payload the demo panel renders.",
			text("term-intro",
				"With the stack up (server on :8100, Redis on the compose network), the whole topic is one GET away: the console demo enqueues its three tasks plus a cron entry, watches the hosted worker drain them with visible retry backoff, replays the archived poison task, samples the two observability numbers along the way, and tears down every key it created — in a few seconds, inside one request. Run the first block and read the payload top to bottom: the queues with their live weights, each task's attempt ladder (the flaky task's ``1:failed → 2:failed → 3:succeeded'' IS the backoff section, observed), the beats with the commands each step issued, and the five derived proofs. Then meet the honest degradation: the second block needs the compose Redis stopped, and the same endpoint answers the typed 503 envelope naming exactly what is unreachable — never a stale answer, never a crash. Start redis again afterwards."),
			code("term-story-bash", "bash", "", "",
				"# the whole choreography, one request: ladders, archive, replay,\n"+
					"# cron ticks, observability peaks, teardown\n"+
					"curl -s localhost:8100/api/demo/jobs/console | python3 -m json.tool | head -60\n"+
					"\n"+
					"# the attempt ladders, task by task — the flaky one is the\n"+
					"# backoff section observed: 1:failed → 2:failed → 3:succeeded\n"+
					"curl -s localhost:8100/api/demo/jobs/console | python3 -c '\n"+
					"import json,sys\n"+
					"for t in json.load(sys.stdin)[\"tasks\"]:\n"+
					"    print(t[\"id\"].rsplit(\":\",1)[-1], t[\"verdict\"])\n"+
					"    for a in t[\"attempts\"]:\n"+
					"        print(\"   attempt\", a[\"attempt\"], a[\"outcome\"])\n"+
					"'\n"+
					"\n"+
					"# the two observability numbers, at their worst and at rest\n"+
					"curl -s localhost:8100/api/demo/jobs/console | python3 -c '\n"+
					"import json,sys\n"+
					"print(json.load(sys.stdin)[\"observability\"])\n"+
					"'"),
			code("term-degrade-bash", "bash", "", "",
				"# the honest degradation: stop the compose redis (from the host,\n"+
					"# the stack dir) and the same endpoint answers the typed 503\n"+
					"docker stop go_developers_guide_redis_dev   # or: docker compose -f .docker/docker-compose.yml stop redis\n"+
					"curl -si localhost:8100/api/demo/jobs/console | head -12\n"+
					"#   HTTP/1.1 503 ...  {\"error\":{\"code\":\"service_unavailable\",\n"+
					"#   \"message\":\"... against the guide's Redis, which is not\n"+
					"#   reachable right now.\", ...}}\n"+
					"\n"+
					"# bring it back — the demo heals with the stack, no restart needed\n"+
					"docker start go_developers_guide_redis_dev"),
			text("term-coda",
				"Read the tour as the topic's proofs, re-runnable any time: the attempt ladders are the retry section with the receipts attached; the poison task's archive entry carries its final error verbatim, and the replay beat shows the ledger growing by exactly one attempt before the re-archive — replay is real execution; the cron beat's tick count is whatever the entry fired while it lived, honestly reported rather than smoothed; and the observability block is the two numbers at their worst and at rest, sampled across the same seconds the work happened. Background jobs close the media-realtime group where the realtime topic left off: realtime moved the RESPONSE off the request path, and this topic moved the WORK — the last structural decoupling an HTTP server learns before the platform topics turn to what happens when it all fails (error handling is next). Go's answer to background work is deliberately small: a pool when the process is enough, a broker-backed queue when it is not, and the same channel-shaped discipline underneath both."),
		),
	}

	if buildErr != nil {
		return Topic{}, buildErr
	}
	return NewTopic(
		"background-jobs",
		"Background jobs in Go: worker pools, asynq queues, retries, and the two numbers",
		"The work that happens after the response is sent: bounded goroutine pools for "+
			"in-process fan-out (memory-fast, memory-mortal), then asynq over the guide's "+
			"Redis for durable jobs — client enqueue with caller-minted ids, weighted "+
			"queues, a hosted ServerRunner worker (demo jobs:console runs the whole "+
			"choreography live: fail-twice-then-succeed under exponential backoff, "+
			"budget exhaustion into the dead-letter archive, a manual replay that "+
			"re-runs for real, a delayed digest, per-second cron ticks) — closed by the "+
			"operational disciplines: poison-message honesty in the DLQ, cron drift, "+
			"email as the canonical out-of-band job with idempotent handlers, and queue "+
			"depth + oldest-job age as the two numbers that page.",
		"media-realtime",
		29,
		sections,
	)
}
