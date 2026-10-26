package content

import (
	"errors"
	"fmt"
)

// This file is topic 02 go-runtime, the second topic to copy the fixed
// per-topic authoring pattern (topic 01 go-language is the exemplar): one
// builder in this package, registered only in NewProductionRegistry; prose
// is the default block; every must-source code block quotes a real repo
// file; the live demo sits INLINE in the section that teaches it (here,
// channels-and-select quotes and serves the real worker-pool choreography
// in internal/domain/demos/concurrency.go); the topic closes with the
// terminal companion.

// goRuntimeTopic builds topic 02 — the runtime under the language: the
// G-M-P scheduler, goroutines against OS threads, channels and select
// (with the topic's live worker-pool demo), the memory model and escape
// analysis, the garbage collector, the toolchain, GOMAXPROCS in
// containers, and the static-binary model the whole guide ships as.
func goRuntimeTopic() (Topic, error) {
	// Authoring helpers: constructor errors are collected (not swallowed)
	// and surfaced once, so the block list below reads as pure content.
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
		// ── 1. The scheduler: G, M, and P ────────────────────────────────
		section("scheduler", "The scheduler: G, M, and P",
			"Every ``go`` statement hands work to a runtime scheduler most Go servers never think about — this section makes you think about it once, properly.",
			text("sched-pitch",
				"Topic 01 taught the language; underneath it runs a second program, the Go runtime — a "+
					"few hundred kilobytes of runtime machinery linked into every binary that owns "+
					"scheduling, garbage collection, and the growth of stacks. Its centerpiece is the "+
					"goroutine scheduler, usually described by its three letters. When you write "+
					"``go doWork()``, the compiler does not spawn anything the operating system can "+
					"see; it allocates a lightweight descriptor and queues it. The runtime then "+
					"multiplexes any number of these onto a small number of kernel threads, which "+
					"is why the cost model of a goroutine is closer to ``make`` on a struct than to "+
					"a system call. The three moving parts:"),
			list("sched-gmp-list", false,
				"G — a goroutine: the descriptor plus a small, growable stack. It is the unit of "+
					"scheduling, and there can be millions; it only runs while some M carries it.",
				"M — a machine: an operating system thread, the only thing the kernel actually "+
					"schedules. Ms block in system calls, park, and get reused; user code never "+
					"touches them directly.",
				"P — a processor: a scheduling context that holds a local run queue of runnable "+
					"Gs. An M must acquire a P to execute Go code, and the number of Ps is exactly "+
					"``GOMAXPROCS`` — the knob this topic returns to in the containers section."),
			text("sched-anatomy",
				"The design goal hiding in that shape: keep every P busy without central "+
					"coordination. Each P schedules from its own local queue first, checks the "+
					"global queue periodically, and steals half of a neighbor's queue when it runs "+
					"dry — work stealing, so imbalance corrects itself instead of piling up behind "+
					"a lock. Two runtime mechanisms do the quiet heavy lifting. Blocking system "+
					"calls hand off: when an M enters a blocking syscall it detaches from its P, "+
					"and the P — with its whole queue of runnable work — is immediately handed to "+
					"another M, so one slow ``read`` on a socket never strands a CPU's worth of "+
					"work. Network I/O, meanwhile, does not burn threads at all: the netpoller "+
					"(epoll on Linux, kqueue on macOS) parks goroutines without an M attached and "+
					"wakes them when their file descriptor is ready. And since Go 1.14 long-"+
					"running tight loops no longer starve the scheduler either — a background "+
					"observer thread preempts them with a signal. The composition root of this "+
					"very server is a client of all of that in eight lines:"),
			code("sched-serve-goroutine", "go",
				"The server's own startup goroutine: one go statement, one buffered channel.",
				"server/cmd/guide/main.go",
				"serveErr := make(chan error, 1)\n"+
					"go func() {\n"+
					"    logger.Info(\"server listening\", \"addr\", srv.Addr, \"env\", cfg.Env)\n"+
					"    serveErr <- srv.ListenAndServe()\n"+
					"}()"),
			text("sched-select-walk",
				"Read it as a scheduler story: ``go func() { … }`` creates a G that starts "+
					"listening on :8100; the main goroutine does NOT wait for it — it moves on "+
					"to a ``select`` that can wait on two channels at once. The buffer of one on "+
					"``serveErr`` matters: if startup fails fast, the server goroutine can drop "+
					"its error into the channel and exit even while the main goroutine is still "+
					"reaching the select — an unbuffered channel here would be a rendezvous the "+
					"main goroutine might arrive at too late to matter. Then the wait itself:"),
			code("sched-select", "go",
				"Two wait arms: the serve error, or the shutdown signal arriving via context.",
				"server/cmd/guide/main.go",
				"select {\n"+
					"case err := <-serveErr:\n"+
					"    // Shutdown is only ever called in the signal path below, so a value\n"+
					"    // here is a genuine startup/serve failure (e.g. port already bound).\n"+
					"    return fmt.Errorf(\"serve: %w\", err)\n"+
					"case <-ctx.Done():\n"+
					"}"),
			text("sched-coda",
				"This is the everyday shape of Go concurrency: not thread pools you configure, "+
					"but goroutines you start and channels you wait on, with the scheduler "+
					"deciding which M carries which G at microsecond granularity — switches "+
					"between goroutines cost a few dozen instructions, versus a kernel thread "+
					"context switch measured in microseconds. The rest of this topic unpacks "+
					"that bargain: what goroutines really cost (next section), how to talk "+
					"between them (channels and select), what the memory model guarantees "+
					"(memory model), and what reclaims the memory they leave behind (GC)."),
		),
		// ── 2. Goroutines vs OS threads ──────────────────────────────────
		section("goroutines-vs-threads", "Goroutines vs OS threads",
			"Why one goroutine per request is normal and one thread per request is a design failure.",
			text("gvt-pitch",
				"The goroutine/thread comparison is best made as a bill of materials. An OS "+
					"thread brings a fixed stack — commonly between one and eight megabytes, "+
					"reserved up front whether used or not — and is created by the kernel "+
					"through a system call, with a scheduler in the kernel that must be entered "+
					"to switch between threads. A goroutine starts with a stack of around two "+
					"kilobytes (the runtime tunes the exact initial size since Go 1.19) that "+
					"grows and shrinks on demand, is created by ordinary user-space code "+
					"allocation, and is switched by the runtime without the kernel's "+
					"involvement. The practical arithmetic: a few thousand threads exhaust "+
					"memory or kernel limits on many systems; a few million goroutines is a "+
					"normal Tuesday. The dimensions of the difference:"),
			list("gvt-costs-list", false,
				"Stack — a thread reserves megabytes it may never use; a goroutine starts "+
					"near two kilobytes and grows only if the recursion actually deepens.",
				"Creation — a thread is a system call plus kernel bookkeeping; a goroutine is "+
					"an allocation and a queue push, cheap enough to start one per request "+
					"without measuring.",
				"Scheduling — threads are scheduled by the kernel in the general case; "+
					"goroutines are scheduled by the runtime in user space, with switches "+
					"measured in nanoseconds and the kernel only involved when an M truly "+
					"blocks.",
				"Parallelism — the number of goroutines says nothing about parallelism: "+
					"``GOMAXPROCS`` fixes how many run simultaneously; a million waiting "+
					"goroutines cost memory, not contention."),
			text("gvt-sharing",
				"Cheap goroutines do not repeal the laws of shared memory. Two goroutines "+
					"writing the same variable with no synchronization are a data race — "+
					"undefined behavior, not a debatable style point — no matter how nice the "+
					"scheduler is. Go gives you exactly two disciplined answers. The first is "+
					"ownership: don't share the value, hand it over — a channel send transfers "+
					"the data to exactly one receiver, and the next section builds a whole "+
					"choreography on that. The second is a lock around genuinely shared "+
					"state, and this server carries the canonical small example: the scenario "+
					"store every demo reads and PATCHes, concurrently, from many request "+
					"goroutines:"),
			code("gvt-mutex", "go",
				"Shared state done right: one RWMutex, lock and defer unlock, copy-in/copy-out.",
				"server/internal/core/scenario.go",
				"func (s *ScenarioStore) Update(patch ScenarioPatch) (ScenarioState, error) {\n"+
					"    s.mu.Lock()\n"+
					"    defer s.mu.Unlock()\n"+
					"    // …\n"+
					"    next := s.current\n"+
					"    if patch.DataMode != nil {\n"+
					"        next.DataMode = *patch.DataMode\n"+
					"    }"),
			text("gvt-mutex-vs-channels",
				"Note the shape: the lock is held for the whole operation and released by "+
					"``defer``; the state is copied out, merged, and written back, so no "+
					"caller ever holds a reference into guarded memory. The rule of thumb "+
					"this repo follows — and the Go proverb behind it — is simple to state "+
					"and worth internalizing: use a mutex to protect a small amount of state "+
					"many goroutines read and write (a counter, a cache, this store); use "+
					"channels to transfer ownership of data through a pipeline (jobs to "+
					"workers, results to a collector). ``Share memory by communicating; "+
					"don't communicate by sharing memory`` is the slogan, but read it as a "+
					"tiebreaker, not a law — the standard library itself is full of mutexes "+
					"guarding maps, including the one above. When in doubt, remember the "+
					"net/http server under this API spawns one goroutine per request and "+
					"thinks nothing of it: concurrency is the default posture of a Go "+
					"server, and the only real question per shared value is lock or handoff."),
		),
		// ── 3. Channels and select (inline demo) ─────────────────────────
		section("channels-and-select", "Channels and select",
			"Goroutines are cheap; channels are how they cooperate — and this section's demo runs a real worker pool live.",
			text("ch-pitch",
				"A channel is a typed conduit: ``make(chan T)`` creates one, ``ch <- v`` sends "+
					"into it, ``v := <-ch`` receives from it, and ``close(ch)`` retires it. "+
					"The two flavors behave differently enough to be the first thing you "+
					"decide. An unbuffered channel is a rendezvous: a send blocks until a "+
					"receiver takes the value, so completion of the send PROVES the handoff "+
					"happened. A buffered channel (``make(chan T, n)``) is a queue of "+
					"capacity n: sends complete immediately while there is room and block "+
					"only when full. Closing is a broadcast: every subsequent receive drains "+
					"remaining values, then returns the zero value with ``ok == false`` — the "+
					"signal loops over ``range ch`` wait for — and a send on a closed channel "+
					"panics, which is why the sender always closes, never the receiver. One "+
					"special case is worth knowing early:"),
			list("ch-kinds-list", false,
				"Unbuffered — send and receive synchronize (a rendezvous); use it when the "+
					"handoff itself is the contract.",
				"Buffered — a bounded queue; use it to absorb bursts or to let a producer "+
					"finish and exit, like the demo's stats channel of capacity three.",
				"Closed — receives drain then report ok=false; range loops end on it; one "+
					"close turns N waiters into N finishers.",
				"Nil — a channel that is never ready; deliberately setting a channel to nil "+
					"disables that arm of a select, a trick topic 06 revisits."),
			text("ch-worker-intro",
				"``select`` is the control-flow twin of channels: it waits on several channel "+
					"operations at once and runs whichever case is ready — with one "+
					"goroutine per arm of the system, select is how each one says ``whatever "+
					"happens first: new work, shutdown, or timeout``. The live demo at the "+
					"end of this section is a complete worker pool built from exactly these "+
					"pieces, and this topic quotes its real code (``server/internal/demos`` "+
					"package — the payload builder the endpoint serves). The worker first — "+
					"read it as a loop that can end two ways, and where every value it "+
					"touches arrives or leaves through a channel:"),
			code("ch-worker", "go",
				"One pool worker: select on work vs shutdown, on send vs shutdown, tally via stats.",
				"server/internal/domain/demos/concurrency.go",
				"func poolWorker(ctx context.Context, id int, jobs <-chan poolJob, results chan<- poolResult, stats chan<- WorkerRow) {\n"+
					"    worker := fmt.Sprintf(\"worker-%d\", id)\n"+
					"    handled := 0\n"+
					"    defer func() { stats <- WorkerRow{WorkerID: worker, JobsDone: handled} }()\n"+
					"    for {\n"+
					"        select {\n"+
					"        case job, ok := <-jobs:\n"+
					"            if !ok {\n"+
					"                return\n"+
					"            }\n"+
					"            select {\n"+
					"            case results <- poolResult{JobID: job.ID, Payload: job.Payload, Output: transform(job.Payload)}:\n"+
					"                handled++\n"+
					"            case <-ctx.Done():\n"+
					"                return\n"+
					"            }\n"+
					"        case <-ctx.Done():\n"+
					"            return\n"+
					"        }\n"+
					"    }\n"+
					"}"),
			text("ch-worker-walk",
				"Four details repay attention. The receive uses the two-value form — ``job, "+
					"ok := <-jobs`` — because a closed jobs channel (``ok`` false) is how "+
					"workers learn there is no more work, while ``ctx.Done()`` firing is how "+
					"they learn to abandon work: two different endings, both honored. The "+
					"send is guarded by its own select: if no collector is receiving and "+
					"cancellation arrives, the worker drops the in-flight result and exits "+
					"instead of blocking forever — the difference between a pool that shuts "+
					"down and one that leaks. The parameters use direction: ``jobs <-chan "+
					"poolJob`` (receive-only) and ``results chan<- poolResult`` (send-only) "+
					"make the misuse of a channel a compile error. And the deferred stats "+
					"send reports the worker's tally on every exit path — defer runs on "+
					"return however the return happened."),
			code("ch-fanout", "go",
				"Fan-out: three workers park on the empty channel — the goroutine-count window the payload reports.",
				"server/internal/domain/demos/concurrency.go",
				"jobs := make(chan poolJob)\n"+
					"results := make(chan poolResult)\n"+
					"stats := make(chan WorkerRow, poolWorkers)\n"+
					"var wg sync.WaitGroup\n"+
					"for id := 1; id <= poolWorkers; id++ {\n"+
					"    wg.Add(1)\n"+
					"    go func() {\n"+
					"        defer wg.Done()\n"+
					"        poolWorker(ctx, id, jobs, results, stats)\n"+
					"    }()\n"+
					"}\n"+
					"facts.GoroutinesDuring = runtime.NumGoroutine()"),
			text("ch-fanout-walk",
				"Fanning out is starting N identical consumers of one channel — here three "+
					"workers launched before any job exists, parked on the empty ``jobs`` "+
					"channel, which is precisely why the line after the loop can count them: "+
					"``runtime.NumGoroutine()`` is taken while all three are provably alive "+
					"and waiting, the runtime fact the demo reports as the pool's window. A "+
					"``sync.WaitGroup`` tracks their lifetime (``Add`` before the ``go``, "+
					"``Done`` deferred inside, ``Wait`` later to join). Meanwhile a feeder "+
					"goroutine streams the eight jobs in, and the fan-in collector receives "+
					"exactly eight results:"),
			code("ch-fanin", "go",
				"Feeder and fan-in: the feeder closes to say no-more-jobs; the collector counts and joins.",
				"server/internal/domain/demos/concurrency.go",
				"go func() {\n"+
					"    defer close(jobs)\n"+
					"    for i, payload := range poolPayloads {\n"+
					"        select {\n"+
					"        case jobs <- poolJob{ID: fmt.Sprintf(\"job-%02d\", i+1), Payload: payload}:\n"+
					"        case <-ctx.Done():\n"+
					"            return\n"+
					"        }\n"+
					"    }\n"+
					"}()\n"+
					"\n"+
					"rows = make([]JobRow, 0, poolJobs)\n"+
					"collect:\n"+
					"for len(rows) < poolJobs {\n"+
					"    select {\n"+
					"    case r := <-results:\n"+
					"        rows = append(rows, JobRow{JobID: r.JobID, Payload: r.Payload, Output: r.Output})\n"+
					"    case <-ctx.Done():\n"+
					"        // A cancelled caller: stop collecting. The joins below still\n"+
					"        // hold, so nothing leaks.\n"+
					"        break collect\n"+
					"    }\n"+
					"}\n"+
					"wg.Wait()\n"+
					"sort.Slice(rows, func(i, j int) bool { return rows[i].JobID < rows[j].JobID })"),
			text("ch-fanin-walk",
				"The feeder ``defer close(jobs)`` — closure is the no-more-work broadcast, and "+
					"deferred so even a cancelled feed closes the channel. The collector "+
					"loop is bounded by a count (exactly eight results), guarded by select "+
					"on cancellation, and uses a labeled ``break collect`` because a plain "+
					"``break`` inside select would break the select, not the loop — a "+
					"classic first-week Go trap. The ``sort.Slice`` at the end is honesty "+
					"about channels: results arrive in completion order, which is scheduler "+
					"weather; the payload reports content order, sorted by job id. "+
					"Cancellation gets its own beat, with the twist that this time the "+
					"pool is cancelled mid-stream:"),
			code("ch-cancel", "go",
				"The cancellation run: all jobs buffered up front, the counted result is the trigger.",
				"server/internal/domain/demos/concurrency.go",
				"jobs := make(chan poolJob, poolJobs)\n"+
					"for i, payload := range poolPayloads {\n"+
					"    jobs <- poolJob{ID: fmt.Sprintf(\"job-%02d\", i+1), Payload: payload}\n"+
					"}\n"+
					"// Deliberately NOT closed: cancellation, not closure, ends this run.\n"+
					"\n"+
					"collected := 0\n"+
					"collect:\n"+
					"for collected < cancelAfter {\n"+
					"    select {\n"+
					"    case <-results:\n"+
					"        collected++\n"+
					"    case <-ctx.Done():\n"+
					"        break collect\n"+
					"    }\n"+
					"}\n"+
					"cancel() // the counted result IS the trigger"),
			text("ch-cancel-walk",
				"Context is cancellation plumbing: ``context.WithCancel`` returns a derived "+
					"context and a ``cancel`` function, and every select arm watching "+
					"``ctx.Done()`` — the workers', the feeder's — unblocks the moment "+
					"``cancel()`` runs. Here the jobs channel is buffered and pre-filled, "+
					"and the collector cancels after the FIFTH result: from that instant "+
					"workers stop draining jobs, in-flight sends abort through their "+
					"select's ctx arm, and the ledger must balance — eight submitted, five "+
					"collected, three abandoned, three workers returned — which is exactly "+
					"what the demo asserts. The same rig races a reply against a timer:"),
			code("ch-race", "go",
				"select with a timer: the reply already waiting wins instantly; the late one loses by configuration.",
				"server/internal/domain/demos/concurrency.go",
				"reply := make(chan string, 1) // buffered: the late producer never leaks, timer or not\n"+
					"if delay == 0 {\n"+
					"    reply <- \"pong\"\n"+
					"} else {\n"+
					"    // …\n"+
					"}\n"+
					"\n"+
					"timer := time.NewTimer(timeout)\n"+
					"defer timer.Stop()\n"+
					"select {\n"+
					"case received := <-reply:\n"+
					"    beat.Winner, beat.Received = \"reply\", received\n"+
					"case <-timer.C:\n"+
					"    beat.Winner, beat.Received = \"timer\", \"\"\n"+
					"}"),
			text("ch-race-walk",
				"Two races, both honest. In the first, the reply sits in the buffer BEFORE "+
					"the select starts, and a select with an instantly-ready case takes it "+
					"immediately — the reply wins deterministically, which is the lesson: "+
					"readiness, not declaration order, decides. In the second, a producer "+
					"goroutine sleeps 200 milliseconds against a 30-millisecond timer, so "+
					"the timer wins by configuration rather than luck; the reply channel "+
					"is buffered precisely so the late producer can always complete its "+
					"send and exit — no goroutine leaks out of a lost race. The ``defer "+
					"timer.Stop()`` is the hygiene worth copying: timers are resources. "+
					"When several cases are ready at once select picks uniformly at "+
					"random — fairness by lottery, which is why you never rely on case "+
					"order. The whole choreography is served live by one thin handler:"),
			code("ch-handler", "go",
				"The demo endpoint: parse nothing, run the choreography, write the payload.",
				"server/internal/presentation/demo.go",
				"func (h *ConcurrencyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {\n"+
					"    writeJSON(w, http.StatusOK, demos.BuildConcurrencyPayload(r.Context()))\n"+
					"}"),
			demo("runtime:concurrency", "Concurrency, live — the worker pool",
				"Runs the real choreography on every request: three goroutines fan out over "+
					"eight jobs and fan results back in through channels; a second run "+
					"cancels mid-stream after five results; two select races pit a reply "+
					"against a timer. Counts, ids, and outcomes are deterministic; the "+
					"per-worker tally and the goroutine counts are what the runtime "+
					"actually did, this run."),
			text("ch-beats-intro",
				"Read the demo in six beats:"),
			list("ch-beats-list", false,
				"Setup — the configuration: eight jobs, three workers, cancel after five; "+
					"identical on every call.",
				"Fan-out — each job's journey: id, payload, and the deterministic transform "+
					"output, sorted by job id because completion order is scheduler weather.",
				"Worker tallies — the one varying fact: how many of the eight jobs each "+
					"worker won, reported per worker and summing to eight.",
				"Cancellation — the ledger: eight submitted, five collected, three "+
					"abandoned, three workers returned; collected + abandoned must always "+
					"equal submitted.",
				"Timeouts — the two rigged races: an already-waiting reply beats the "+
					"select's timer instantly; a 200ms reply loses to a 30ms timer by "+
					"construction.",
				"Runtime facts — what the runtime reports about itself: GOMAXPROCS and the "+
					"goroutine count before, during (three parked workers), and after the "+
					"pool."),
			text("ch-coda",
				"Topic 06 replays this toolbox at production scale — errgroup, timeouts as "+
					"context deadlines, and the leak-hunting habits the race detector "+
					"enforces. What this section adds to the scheduler story is the "+
					"cooperation layer: goroutines make concurrency cheap, channels make it "+
					"correct, select makes it responsive. What neither makes it is SAFE "+
					"from memory races by mere existence — that guarantee has a written "+
					"contract, and it is next."),
		),
		// ── 4. The memory model and escape analysis ──────────────────────
		section("memory-model-and-escape", "The memory model and escape analysis",
			"What the runtime promises about who sees which write — and where your values actually live.",
			text("mm-pitch",
				"The Go memory model is a short, deliberately minimal document: it defines "+
					"happens-before edges — when one goroutine's write is guaranteed visible "+
					"to another goroutine's read — and declares everything else a data race, "+
					"and racy programs are simply invalid, with no obligation on the "+
					"compiler to behave. The guarantees you actually use come from the "+
					"standard library's synchronization primitives, and they are precise. "+
					"A send on a channel happens before the corresponding receive "+
					"completes — the demo's fan-in may read everything a worker wrote "+
					"before sending. For a buffered channel of capacity C, the k-th "+
					"receive happens before the (k+C)-th send completes — the pool's stats "+
					"channel (capacity three, exactly three sends) can never block, and "+
					"the guarantee is what makes that reasoning sound. A call to "+
					"``Unlock`` happens before any subsequent ``Lock`` returns — the "+
					"scenario store's guarded state is visible to the next reader in full. "+
					"And ``wg.Done()`` happens before ``Wait()`` returns — the collector "+
					"may trust every worker's writes after the join:"),
			code("mm-happens", "go",
				"The pool's joins and tallies, read as happens-before edges.",
				"server/internal/domain/demos/concurrency.go",
				"wg.Wait()\n"+
					"sort.Slice(rows, func(i, j int) bool { return rows[i].JobID < rows[j].JobID })\n"+
					"\n"+
					"counts = make([]WorkerRow, 0, poolWorkers)\n"+
					"for i := 0; i < poolWorkers; i++ {\n"+
					"    counts = append(counts, <-stats)\n"+
					"}"),
			text("mm-edges",
				"Why the drain loop after ``wg.Wait()`` is not a race: each worker's "+
					"deferred stats send runs before the worker returns — and the "+
					"launcher goroutine's own deferred ``wg.Done()`` runs only after "+
					"``poolWorker`` returns, so every stats send happens before its "+
					"Done; Done happens before ``Wait`` returns; the receives into "+
					"``counts`` therefore observe completed sends. That chain of "+
					"custody — send before Done, Done before Wait, Wait before the "+
					"read — IS the memory model in daily use, and the reasoning "+
					"transfers to every primitive in ``sync``. When you skip the edges "+
					"entirely — two unsynchronized accesses to one variable — you have "+
					"a data race: the race detector (``go test -race``, on every suite "+
					"this repo runs) catches it with happens-before tracking at "+
					"runtime, and it never reports a race that is not there. Escape "+
					"analysis is the memory model's quiet cousin on the allocation "+
					"side — the compiler deciding, for every value, stack or heap:"),
			text("mm-escape-intro",
				"Every function's locals CAN live on its stack frame — free allocation, "+
					"free reclamation, gone when the frame returns. A value must move to "+
					"the heap (a real allocation the GC must later reclaim) when its "+
					"lifetime might outlive the frame: a pointer returned to the caller, "+
					"a value sent into the outside world through an interface or a "+
					"channel whose receiver the compiler cannot see, a variable captured "+
					"by a goroutine that may outlive the call. Escape analysis is "+
					"conservative and provable: taking an address does NOT force a heap "+
					"trip if the pointer provably dies with the frame — the opposite of "+
					"the C stack-vs-malloc folklore. The compiler will tell you its "+
					"decisions for this very repo:"),
			code("mm-escape-cmd", "bash", "", "",
				"# ask the compiler where this repo's values live (real output, go 1.27)\n"+
					"cd server && go build -gcflags=-m ./internal/content/ 2>&1 | grep -m2 'escapes to heap'\n"+
					"internal/content/model.go:40:42: map[string]bool{...} escapes to heap\n"+
					"internal/content/snippet.go:53:44: map[string]commentStyle{...} escapes to heap\n"+
					"# and the other side: function values that provably stay on the stack\n"+
					"go build -gcflags=-m ./internal/content/ 2>&1 | grep -m2 'func literal does not escape'\n"+
					"internal/content/registry.go:38:33: func literal does not escape\n"+
					"internal/content/registry.go:42:33: func literal does not escape"),
			text("mm-escape-why",
				"Read those four lines like a memory-model auditor. The package-level maps "+
					"escape because package variables live forever — the frame is the whole "+
					"program, so the heap it is. The sort comparators do NOT escape: a "+
					"function value passed to ``sort.SliceStable`` and called before it "+
					"returns can stay on the stack — no allocation per comparison, which "+
					"matters in a hot loop. Why an author cares: heap allocations are the "+
					"currency the next section's garbage collector spends its CPU "+
					"collecting, so the cheapest allocation is the one the compiler "+
					"proves unnecessary. The idioms that follow — ``make`` with capacity "+
					"hints (the registry's ``make([]TopicSummary, 0, len(r.topics))``), "+
					"``strings.Builder`` instead of string concatenation, reusing "+
					"buffers — are all allocation-avoidance by another name, and they "+
					"only make sense once you know the compiler is your ally that "+
					"already eliminates what it can prove."),
		),
		// ── 5. The garbage collector ─────────────────────────────────────
		section("garbage-collector", "The garbage collector: tricolor and the pacer",
			"A concurrent mark-sweep collector tuned by two environment variables — and how to watch it work.",
			text("gc-pitch",
				"Go's GC is a concurrent, tri-color mark-and-sweep collector: concurrent "+
					"means your goroutines keep running while most of the collection "+
					"happens (the stop-the-world pauses are the tiny bookends — starting "+
					"and finishing the mark — typically well under a millisecond); "+
					"tri-color is the bookkeeping that makes marking incremental; sweep "+
					"means memory is simply returned to the allocator's free lists — Go "+
					"never compacts, never moves objects, which is why pointers are "+
					"stable and the runtime needs no read barriers on ordinary loads. "+
					"The three colors are the states every object moves through during a "+
					"mark cycle:"),
			list("gc-colors-list", false,
				"White — not yet visited; at the end of marking, everything still white is "+
					"garbage and gets swept.",
				"Grey — known reachable but not yet scanned; the work queue of the mark "+
					"phase, drained by collector and mutator alike.",
				"Black — scanned and live; everything it points to has been discovered, so "+
					"it is safe for the cycle."),
			text("gc-pacer",
				"The subtle part is running a mark WHILE the program mutates the graph. The "+
					"tricolor invariant — no black object may point to a white one without "+
					"the write barrier noticing — is maintained by a hybrid write barrier "+
					"(since Go 1.8): every pointer overwrite that could break the invariant "+
					"shades a survivor grey instead. When allocating goroutines outpace the "+
					"collectors, the pacer makes them pay their own bill through mark "+
					"assists — allocation itself slows until the mark catches up. WHEN to "+
					"collect is the pacer's other job, and it is exposed to you as exactly "+
					"two knobs. ``GOGC`` (default 100) is the heap-growth trigger: with "+
					"100, the next cycle aims to run when the heap doubles from its live "+
					"size after the last one — double the live heap in GC work, trade half "+
					"the collections for it. ``GOMEMLIMIT`` (Go 1.19) is a soft ceiling on "+
					"total runtime memory — the pacer runs harder as the limit approaches, "+
					"and in a container with a hard memory limit it is the difference "+
					"between degrading and being OOM-killed. Under-provisioned services "+
					"feel GC in latency; the knobs are how you answer."),
			code("gc-trace", "bash", "", "",
				"# watch real cycles: one line per GC, from the runtime itself\n"+
					"# (columns: cycle number, time, % of CPU in GC, then phase timings\n"+
					"# and the heap sizes before/after — run it against any Go binary)\n"+
					"GODEBUG=gctrace=1 ./guide"),
			text("gc-coda",
				"The practical posture this guide takes: write allocation-aware code "+
					"(the escape-analysis section is where that starts), leave ``GOGC`` "+
					"alone unless a workload says otherwise, and set ``GOMEMLIMIT`` to "+
					"roughly 90 percent of the container's memory limit in every "+
					"deployment — a rule topic 37 turns into real configuration. Deep "+
					"GC-tuning with benchmarks and ``benchstat`` is topic 38's subject; "+
					"observability of GC in production metrics is topic 33's. What this "+
					"section leaves you with is the mental model: mark runs concurrently "+
					"under a write barrier, the pacer turns two environment variables "+
					"into a collection schedule, and pauses are bookkeeping, not "+
					"philosophy."),
		),
		// ── 6. The toolchain ─────────────────────────────────────────────
		section("toolchain", "The toolchain: build, test, vet, gofmt, go mod",
			"One statically-linked ``go`` binary is the whole development environment — this repo's verification protocol is just its subcommands.",
			text("tool-pitch",
				"Go ships its entire toolchain as one binary: the compiler, the test "+
					"runner, the formatter, the dependency manager, the profiler hooks — "+
					"subcommands of ``go``, versioned together, no plugins, no Makefile "+
					"required to paper over gaps. This repo is dogfood for that claim: "+
					"its entire verification pipeline (the one every topic in this guide "+
					"is checked against) is a one-line chain of stock subcommands run "+
					"inside the dev container. The five you will use daily:"),
			list("tool-commands-list", false,
				"go build — compile packages and their dependencies into a binary (or just "+
					"type-check, with go vet as the lint pass); content-addressed caching "+
					"makes the second build free.",
				"go test — run tests with the built-in framework; add -race for the race "+
					"detector, -count=1 to bypass the result cache, -run to scope a suite.",
				"go vet — the suspicious-construct checker: printf mismatches, locks "+
					"copied by value, unreachable code — cheap enough to run always, "+
					"this repo runs it on every verification pass.",
				"gofmt — canonical formatting, no configuration, no arguments; "+
					"gofmt -l . lists files that deviate (the empty output is the gate).",
				"go mod — module graph management: tidy the manifest, explain why a "+
					"dependency exists, graph the whole tree; MVS picks one version per "+
					"module deterministically."),
			code("tool-dockerfile", "dockerfile",
				"This repo's toolchain, pinned: one official image, one version, everywhere.",
				".docker/Dockerfile.dev",
				"FROM golang:1.27.3-bookworm\n"+
					"# …\n"+
					"RUN npm install -g npm@11\n"+
					"# …\n"+
					"RUN ln -s /usr/local/go/bin/go /usr/local/bin/go \\\n"+
					"    && ln -s /usr/local/go/bin/gofmt /usr/local/bin/gofmt \\\n"+
					"    && printf 'export PATH=\"$PATH\":/usr/local/go/bin:/go/bin\\n' > /etc/profile.d/go-toolchain.sh"),
			text("tool-mod",
				"The Dockerfile pins the exact Go version by pinning the image — the "+
					"toolchain IS the environment, which is the whole trick: nothing to "+
					"install, nothing to drift, every machine that runs the container "+
					"running byte-identical tooling. The symlink lines solve a real "+
					"trap this repo hit (a login shell resetting PATH and losing ``go``) "+
					"— and they document the non-negotiable: gofmt's output is part of "+
					"the definition of Go code, so formatting is never a debate. go.mod "+
					"and go.sum carry the dependency story: the manifest names the "+
					"module (``guide``) and its requirements, the sum file pins their "+
					"hashes, and ``go mod tidy`` keeps both honest. This server "+
					"currently needs no third-party code at all — the stdlib-only "+
					"manifest from topic 01 is the honest starting point — and "+
					"dependency management as a full discipline (MVS, upgrades, "+
					"govulncheck) is topic 31. The verification pipeline that ties "+
					"the toolchain together is one command, and it is the same one "+
					"this guide runs after every change:"),
			code("tool-verify", "bash", "", "",
				"# the whole server-side gate for this repo (Protocol v2, warm caches)\n"+
					"docker exec go_developers_guide_dev bash -lc \\\n"+
					"  'cd /workspaces/go_developers_guide/server && gofmt -l . && go build ./... && go vet ./... && go test -race -count=1 ./...'"),
		),
		// ── 7. GOMAXPROCS in containers ──────────────────────────────────
		section("gomaxprocs-in-containers", "GOMAXPROCS in containers",
			"The knob that decides how many things run at once — and why containers changed its default answer.",
			text("gx-pitch",
				"``GOMAXPROCS`` sets the number of Ps — scheduler contexts — and therefore "+
					"the maximum number of goroutines executing Go code simultaneously. "+
					"It caps PARALLELISM, not concurrency: a million goroutines run fine "+
					"with ``GOMAXPROCS=1``; they just take turns. For years the default "+
					"was the host's CPU count (``runtime.NumCPU()``), which was exactly "+
					"right on a machine and exactly wrong in a container: the runtime "+
					"would see a 64-core node, arm 64 Ps, and start 64 concurrent threads "+
					"— while the container's cgroup CPU quota allowed, say, 2 cores' "+
					"worth of runtime. The result was CPU throttling: bursts of runnable "+
					"work exhausting the quota early in every period, GC mark assists "+
					"compounding the burst, and tail latency spikes that looked like "+
					"mystery until someone read the cgroup file. The classic fixes were "+
					"manual — set ``GOMAXPROCS`` by hand, or import the automaxprocs "+
					"package to read the quota for you."),
			text("gx-modern",
				"Since Go 1.25 the runtime is container-aware by default: it reads the "+
					"cgroup CPU quota and derives ``GOMAXPROCS`` from it (rounded up, "+
					"with a floor of two), re-evaluating periodically as quotas change. "+
					"This guide pins Go 1.27, so its containers get the correct behavior "+
					"with no code and no configuration — and the demo at the channels "+
					"section reports the live value in its runtime facts, so you can "+
					"see what the runtime decided, from inside. The dev stack this repo "+
					"runs in deliberately sets no CPU limits — the toolchain container "+
					"below is built for warm caches and unlimited build parallelism — "+
					"but the resources it does pin are pure toolchain discipline:"),
			code("gx-compose", "yaml",
				"The dev stack's Go volumes: the toolchain's caches are data, not image layers.",
				".docker/docker-compose.yml",
				"volumes:\n"+
					"  # Named Go caches: module cache + build cache ride volumes so every\n"+
					"  # docker-exec verification run stays warm (never the image, never the\n"+
					"  # bind-mounted workspace).\n"+
					"  - gdg-go:/go/pkg/mod\n"+
					"  - gdg-gocache:/root/.cache/go-build\n"+
					"  # Named npm cache at the same mount point the fastapi guide uses;\n"+
					"  # shared with client_dev below.\n"+
					"  - gdg-npm:/root/.npm"),
			text("gx-coda",
				"What to actually check in production: that ``GOMAXPROCS`` matches the "+
					"quota (the demo's runtime-facts beat answers for this process), "+
					"that a CPU quota exists at all (an unlimited quota with a memory "+
					"limit is its own incident), and that throttling metrics — not CPU "+
					"usage — are what your alerts watch. Deployment configuration for "+
					"CPU and memory limits, and where GOMEMLIMIT meets them, is topic "+
					"37's ground; performance forensics when it goes wrong is topic 38's."),
		),
		// ── 8. The static-binary model ───────────────────────────────────
		section("static-binary", "The static-binary model",
			"What ``go build`` actually produces — and why deploying Go is copying a file.",
			text("bin-pitch",
				"``go build`` produces one self-contained executable. No interpreter to "+
					"install, no runtime to version-match, no site-packages or "+
					"node_modules to sync: the binary links the runtime, the stdlib "+
					"packages you use, and your code into a single file measured in "+
					"megabytes, and that file IS the deployment. The composition root "+
					"this server runs from is worth reading as the whole program in "+
					"miniature — every dependency it has is either the standard library "+
					"or this repo, which is precisely why the output can be static:"),
			code("bin-server", "go",
				"The entire server, dependency-wise: stdlib only, one binary out.",
				"server/cmd/guide/main.go",
				"srv := &http.Server{\n"+
					"    Addr:    fmt.Sprintf(\":%d\", cfg.Port),\n"+
					"    Handler: handler,\n"+
					"    // Bounded reads/writes defeat slow-client abuse while leaving ample\n"+
					"    // headroom for the guide's demos.\n"+
					"    ReadHeaderTimeout: 5 * time.Second,\n"+
					"    ReadTimeout:       10 * time.Second,\n"+
					"    WriteTimeout:      15 * time.Second,\n"+
					"    IdleTimeout:       60 * time.Second,\n"+
					"}"),
			code("bin-build", "bash", "", "",
				"# build it, then ask the file what it is — no runtime to install\n"+
					"cd server && go build -o /tmp/guide-p32 ./cmd/guide\n"+
					"file /tmp/guide-p32"),
			text("bin-tradeoffs",
				"Two qualifiers make the model precise. First, static is the default "+
					"only when cgo is out of the picture: with ``CGO_ENABLED=0`` (this "+
					"repo's deployment posture, topic 37) the binary is fully static — "+
					"the net package falls back to the pure-Go resolver, and the image "+
					"can be distroless or even ``FROM scratch``. With cgo enabled the "+
					"binary links glibc dynamically and needs a matching libc at rest. "+
					"Second, size: the binary carries symbol tables and DWARF debug "+
					"information; ``-ldflags \"-s -w\"`` strips both for production "+
					"images at the cost of harder stack traces, and ``-ldflags \"-X "+
					"main.version=…\"`` bakes version metadata in — the standard release "+
					"recipe. Cross-compilation is the quiet superpower of the same "+
					"model: ``GOOS=linux GOARCH=arm64 go build`` on a macOS laptop "+
					"produces a Linux ARM binary, no toolchain matrix, because the "+
					"compiler ships every target in the one ``go`` binary."),
			text("bin-coda",
				"Operationally, the static binary is why Go deployment stories are short: "+
					"a container image becomes a binary plus TLS certificates plus "+
					"nothing; rollback is keeping the previous file; the runtime"+
					" — scheduler, GC, memory model, everything this topic taught — "+
					"travels inside the artifact it explains. Topics 37 and 38 take "+
					"the model to Kubernetes and to its performance limits; here it "+
					"closes the loop the scheduler section opened: the runtime you "+
					"just learned to reason about is the only process your "+
					"deployment contains."),
		),
		// ── 9. Terminal companion ────────────────────────────────────────
		section("terminal-companion", "Terminal companion: the curl tour",
			"The runtime you just met, inspected from the shell — this topic's endpoints, live.",
			text("term-pitch",
				"With the stack up (server on :8100), these commands walk the surface this "+
					"topic lives in: the curriculum index where go-runtime sits second in "+
					"foundations, this topic's full payload, and the concurrency demo "+
					"endpoint that runs the worker pool on every call. Piping to "+
					"``python3 -m json.tool`` keeps the tour dependency-free:"),
			code("term-list", "bash", "", "",
				"# the curriculum index: go-runtime is order 2 in the foundations group\n"+
					"curl -s localhost:8100/api/content | python3 -m json.tool | head -30"),
			code("term-detail", "bash", "", "",
				"# this topic in full: scheduler → goroutines → channels (+ inline demo) →\n"+
					"# memory model → GC → toolchain → GOMAXPROCS → static binary → this tour\n"+
					"curl -s localhost:8100/api/content/go-runtime | python3 -m json.tool | head -40"),
			code("term-demo", "bash", "", "",
				"# the worker pool, live: run it twice and compare — deterministic beats\n"+
					"# (setup, fan-out rows, cancellation ledger, rigged races) never move;\n"+
					"# worker tallies and goroutine counts are what the runtime did\n"+
					"curl -s localhost:8100/api/demo/runtime/concurrency | python3 -m json.tool"),
			code("term-404", "bash", "", "",
				"# unknown topic → the shared error envelope + x-request-id\n"+
					"curl -s -i localhost:8100/api/content/nope"),
			code("term-scenario", "bash", "", "",
				"# the runtime toggle every demo reads (fake/live data, failure injection)\n"+
					"curl -s localhost:8100/api/demo/scenario"),
			text("term-coda",
				"One more runtime fact hiding in plain sight: every curl above is one "+
					"connection served by one goroutine the net/http server spawned for "+
					"it, multiplexed by the scheduler onto a thread count you never "+
					"specified — the topic's first section, running under your terminal. "+
					"Topic 03 moves up the stack to what those goroutines are serving: "+
					"HTTP itself."),
		),
	}

	if buildErr != nil {
		return Topic{}, buildErr
	}
	return NewTopic(
		"go-runtime",
		"The Go runtime",
		"The runtime under the language — scheduler, goroutines, channels and select, "+
			"memory model, GC, toolchain, and the static binary — with a live worker-pool demo.",
		"foundations",
		2,
		sections,
	)
}
