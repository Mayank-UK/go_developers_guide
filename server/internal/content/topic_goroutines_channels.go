package content

import (
	"errors"
	"fmt"
)

// This file is topic 06 goroutines-and-channels (group "framework", order
// 6), a GO-NEW topic with no fastapi counterpart: Go's concurrency IS its
// framework. The split against topic 02 go-runtime is deliberate — 02
// teaches the machine underneath (the G-M-P scheduler, the memory model,
// the garbage collector); this topic teaches the PROGRAMMING model: the
// idioms a Go engineer types every day. Every code block quotes a real
// repo file — this topic's own demo builders are the primary source —
// the live demos sit INLINE in the sections that teach them, and the
// topic closes with the terminal companion.

// goroutinesChannelsTopic builds topic 06 — the concurrency idioms this
// server itself runs on: goroutines and WaitGroup, channels and their
// discipline, select and timeouts, context cancellation propagation,
// errgroup by comparison, and the race detector plus goroutine-leak
// hygiene as the enforcement story. The two live demos (conc:context,
// conc:race) are performed by real builders in internal/domain/demos and
// pinned by their own tests.
func goroutinesChannelsTopic() (Topic, error) {
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
		// ── 1. The concurrency model ────────────────────────────────────
		section("concurrency-model", "The concurrency model",
			"Go's concurrency is its framework — three language primitives instead of a runtime bolted on top.",
			text("cm-pitch",
				"Go was designed inside Google around a specific theory of concurrent programs: Tony "+
					"Hoare's Communicating Sequential Processes. In CSP the unit of execution and the unit "+
					"of communication are separate ideas, and synchronization lives in the communication "+
					"itself — which is why Go's famous slogan reads ``do not communicate by sharing "+
					"memory; instead, share memory by communicating``. The language spells this as three "+
					"primitives: ``go`` starts a goroutine (an independently scheduled function), "+
					"``chan`` is a typed conduit goroutines send values through, and ``select`` waits on "+
					"several channel operations at once. Everything else in Go's concurrency story — "+
					"``sync.WaitGroup``, ``sync.Mutex``, ``context.Context`` — is standard-library "+
					"structure built on those foundations, not language machinery."),
			text("cm-scheduler-split",
				"One division of labor up front, because it shapes everything below: this topic teaches "+
					"the PROGRAMMING model — the idioms you type. Topic 02 go-runtime teaches the machine "+
					"underneath: the G-M-P scheduler that multiplexes goroutines onto OS threads, why a "+
					"goroutine's stack starts around two kilobytes and grows, and the memory model that "+
					"defines what a concurrent program may observe. The practical consequences you need "+
					"here are two: goroutines are cheap enough that idiomatic Go starts them freely — "+
					"``net/http`` itself runs one goroutine per connection, so every request this server "+
					"serves is already concurrent — and cheap enough that the engineering question is "+
					"rarely ``can I afford another goroutine`` but ``who joins it, and through what``."),
			text("cm-already-running",
				"This server uses the model from its first line of startup: the composition root "+
					"launches the HTTP server on its own goroutine and hands the main goroutine a "+
					"channel to hear about its fate — one send, one owner, no shared flag. The buffer "+
					"of one is deliberate and is examined in the channels section; the shape to notice "+
					"now is that ``ListenAndServe`` blocks forever, so it MUST run somewhere that is "+
					"not the main flow, and its result must come back over a channel because a "+
					"goroutine call returns no value:"),
			code("cm-serve-goroutine", "go",
				"The server's own goroutine: launch, listen, report over a channel of one.",
				"server/cmd/guide/main.go",
				"serveErr := make(chan error, 1)\n"+
					"go func() {\n"+
					"    logger.Info(\"server listening\", \"addr\", srv.Addr, \"env\", cfg.Env)\n"+
					"    serveErr <- srv.ListenAndServe()\n"+
					"}()"),
			text("cm-two-families",
				"The toolset splits into two families, and mature Go code uses both without ideology. "+
					"The communication family — channels, ``select``, and by extension ``context`` — "+
					"shines when the problem is handing work and results between goroutines: pipelines, "+
					"fan-out and fan-in, ownership transfer. The synchronization family — ``sync.Mutex``, "+
					"``sync.RWMutex``, ``sync.WaitGroup`` — shines when the problem is shared state "+
					"sitting still: a counter, a cache, this server's scenario toggle, guarded in place. "+
					"The race-detector section at the end of this topic shows the same total computed "+
					"both ways, live; neither is the ``right`` one in the abstract, and the wrong answer "+
					"is the third option — touching shared state with no synchronization at all."),
			text("cm-map",
				"The topic runs in dependency order: goroutines and how to join them with a WaitGroup; "+
					"channels in full — unbuffered rendezvous, buffered queues, closing, direction "+
					"types; ``select`` and the two timeout idioms; ``context.Context`` and "+
					"cancellation propagation, which earns this topic's first live demo; ``errgroup`` "+
					"and what it adds over a bare WaitGroup; and finally the race detector and "+
					"goroutine-leak hygiene — the discipline that makes all of the above trustworthy — "+
					"with the second live demo. A terminal companion closes the page."),
		),
		// ── 2. Goroutines and WaitGroup ────────────────────────────────
		section("goroutines-and-waitgroup", "Goroutines and sync.WaitGroup",
			"Starting concurrent work is one keyword; finishing it is an engineering decision.",
			text("gor-go-statement",
				"``go f(args)`` evaluates and runs the function in a new goroutine; control returns to "+
					"the caller immediately. That is the entire keyword — no handle, no id, no result. "+
					"The caller of ``go`` learns NOTHING about completion unless the code arranges to be "+
					"told, which is why ``go`` is never the interesting part of a concurrent design: the "+
					"interesting part is always the rendezvous — the channel, WaitGroup, or context that "+
					"joins the work back. A goroutine launched with no rendezvous is fire-and-forget, "+
					"and that is legitimate in exactly one place in this server (the daemon-ish serve "+
					"loop above, whose lifetime IS the process) and a latent bug almost everywhere "+
					"else, because an unjoined goroutine's writes can land after the function that "+
					"launched it has returned and its data has been reused."),
			text("gor-waitgroup-pitch",
				"``sync.WaitGroup`` is the standard answer to ``launch N, join N``: a counter you add "+
					"to before each launch, each goroutine decrements as it leaves, and ``Wait`` blocks "+
					"until the counter hits zero. The race-detector demo's mutex run is the shape in "+
					"miniature — two hundred goroutines each increment a shared counter twenty-five "+
					"times under a mutex, and the total is only readable after ``Wait`` proves every "+
					"goroutine finished its critical sections:"),
			code("gor-waitgroup-shape", "go",
				"Add before every launch, Done in a defer, Wait before reading the result.",
				"server/internal/domain/demos/race_detector.go",
				"wg.Add(goroutines)\n"+
					"for i := 0; i < goroutines; i++ {\n"+
					"    go func() {\n"+
					"        defer wg.Done()\n"+
					"        for j := 0; j < increments; j++ {\n"+
					"            mu.Lock()\n"+
					"            counter++\n"+
					"            mu.Unlock()\n"+
					"        }\n"+
					"    }()\n"+
					"}\n"+
					"wg.Wait()\n"+
					"return counter"),
			text("gor-discipline",
				"The WaitGroup contract is small, and every clause exists because breaking it is "+
					"either a deadlock or a leak:"),
			list("gor-waitgroup-rules", false,
				"Add before the go — the counter must already be incremented when the goroutine "+
					"starts, or a fast goroutine can Done before its Add and drive the counter negative "+
					"(a panic) while Wait returns early.",
				"Defer the Done — ``defer wg.Done()`` as the goroutine's first line survives every "+
					"return path, including panics recovered upstream; a manual Done before a "+
					"conditional return will eventually be forgotten.",
				"Never copy a used WaitGroup — its internals are a mutex and a counter; copying "+
					"splits them. Pass it by pointer, the way the cancellation demo passes each "+
					"generation's group into its workers.",
				"Wait has no deadline — a goroutine that never finishes makes Wait block forever; "+
					"when that is possible the goroutine must also honor a context (this topic's "+
					"fifth section) so the wait has an exit.",
				"Zero is a valid state — Wait on an empty group returns immediately, which is what "+
					"makes \"collect results, then Wait\" refactorings safe."),
			text("gor-closures",
				"Goroutine bodies are usually closures, and closures capture their environment — "+
					"which for years made loop variables the classic Go bug: every closure capturing "+
					"the same ``worker`` variable shared all iterations' values. Go 1.22 settled this "+
					"by giving each iteration of a ``for`` loop its own variables, so capture just "+
					"works now. The cancellation demo still passes its per-worker descriptor as an "+
					"argument — the explicit form that reads correctly in every Go version and makes "+
					"the goroutine's inputs visible in its signature instead of buried in the "+
					"closure's free variables:"),
			code("gor-explicit-args", "go",
				"Per-iteration state as arguments: visible in the signature, safe in any Go.",
				"server/internal/domain/demos/context_cancellation.go",
				"go func(worker ctxWorkerDesc, wg *sync.WaitGroup) {\n"+
					"    defer wg.Done()\n"+
					"    ctxRunWorker(ctxByWorker[worker.ID], works[worker.ID], acks, observed, worker.ID)\n"+
					"}(worker, wg)"),
			text("gor-bounded",
				"The last launching rule is arithmetic, not syntax: never fan out unbounded. A "+
					"goroutine is cheap, not free — each holds a stack and whatever it captured — and "+
					"a loop that spawns one goroutine per request, per row, or per file under load "+
					"multiplies memory exactly when the system is busiest. The idiomatic caps are a "+
					"fixed pool of workers consuming a jobs channel (the shape topic 02's demo "+
					"choreographs) or a bounded semaphore; the errgroup section covers the "+
					"``SetLimit`` form of the same idea. Every fan-out in this topic's demos is a "+
					"fixed, named constant — the bound should be a number a reader can find."),
		),
		// ── 3. Channels ────────────────────────────────────────────────
		section("channels", "Channels: buffered, unbuffered, closed",
			"A typed conduit whose blocking behavior IS the synchronization.",
			text("ch-unbuffered",
				"``make(chan T)`` creates an unbuffered channel, and its semantics are the whole "+
					"idea: a send blocks until another goroutine is ready to receive at that same "+
					"moment, and the receive completes only when a value is handed over. The send and "+
					"the receive are a rendezvous — synchronization is not added to the data transfer, "+
					"it IS the data transfer. That property deletes an entire class of code other "+
					"models need: no condition flags, no ``isReady`` booleans, no locks around the "+
					"handoff. When the cancellation demo's orchestrator sends a work item to a "+
					"worker over an unbuffered channel, the send returning already proves a worker "+
					"took it — and the demo adds a per-item ack channel so the orchestrator also "+
					"knows the item was processed before the choreography moves on:"),
			code("ch-handoff", "go",
				"Unbuffered send + ack: the work phase is deterministic because receipt is proven.",
				"server/internal/domain/demos/context_cancellation.go",
				"for i := 1; i <= ctxWorkItemsPerWorker; i++ {\n"+
					"    works[worker.ID] <- worker.ID + \"-\" + string(rune('0'+i))\n"+
					"    <-acks\n"+
					"}"),
			text("ch-buffered",
				"``make(chan T, n)`` creates a buffered channel — a queue of capacity n. Sends "+
					"complete immediately while there is room and block only when the buffer is "+
					"full; receives drain the queue and block only when it is empty. Buffering "+
					"trades synchronization for decoupling: producer and consumer no longer meet "+
					"per value. The race demo's channel-fold run uses a buffer sized exactly to "+
					"the goroutine count, which is a real idiom with a real guarantee — every "+
					"goroutine's single send has a slot waiting, so no sender can block even if "+
					"the fold has not started reading yet:"),
			code("ch-buffered-tallies", "go",
				"Capacity equals sender count: every one of the 200 sends has a slot.",
				"server/internal/domain/demos/race_detector.go",
				"tallies := make(chan int, goroutines)\n"+
					"var wg sync.WaitGroup\n"+
					"wg.Add(goroutines)\n"+
					"for i := 0; i < goroutines; i++ {\n"+
					"    go func() {\n"+
					"        defer wg.Done()\n"+
					"        local := 0\n"+
					"        for j := 0; j < increments; j++ {\n"+
					"            local++\n"+
					"        }\n"+
					"        tallies <- local\n"+
					"    }()\n"+
					"}"),
			text("ch-closing",
				"``close`` marks a channel finished. On the receive side the rule is that buffered "+
					"values drain first, then every subsequent receive returns the zero value with "+
					"the comma-ok form reporting ``false`` — ``v, ok := <-ch`` distinguishes a real "+
					"zero from end-of-stream — and ``for v := range ch`` loops until the channel is "+
					"closed and drained, making range the natural shape for consumer loops. On the "+
					"discipline side the rules are strict because violations are run-time panics: "+
					"only the SENDER closes (closing is a sender's broadcast that no more values "+
					"will come; a receiver closing under a live sender is a race by definition), "+
					"closing twice panics, and sending on a closed channel panics. The canonical "+
					"close-and-fold from the race demo — join every sender, close once, then range — "+
					"gets the ordering right by construction:"),
			code("ch-close-fold", "go",
				"Wait for all senders, close once, then range the drained tallies.",
				"server/internal/domain/demos/race_detector.go",
				"wg.Wait()\n"+
					"close(tallies)\n"+
					"total := 0\n"+
					"for tally := range tallies {\n"+
					"    total += tally\n"+
					"}\n"+
					"return total"),
			text("ch-close-as-broadcast",
				"Closing is also Go's broadcast primitive: one ``close`` wakes EVERY receiver, "+
					"which a send can never do. That is the mechanism under ``context`` "+
					"cancellation — cancelling a context closes its ``Done()`` channel and every "+
					"goroutine selecting on it observes the close at once (the fifth section runs "+
					"exactly that choreography), and it is the mechanism behind the classic "+
					"one-way shutdown signal ``done := make(chan struct{})`` closed by the owner "+
					"when it is time for everyone to leave. The empty struct carries no data and "+
					"occupies zero bytes — only the close event matters."),
			text("ch-directions-and-nil",
				"Channel types carry direction: ``<-chan T`` is receive-only, ``chan<- T`` is "+
					"send-only, and converting a bidirectional channel to a directional type is "+
					"implicit at assignment or parameter passing. The cancellation demo's worker "+
					"signature uses both directions to make the goroutine's powers visible in its "+
					"type — this worker may receive work and send acks and observations, and the "+
					"compiler enforces that it can never close or send on the wrong conduit. The "+
					"nil channel completes the picture: a send to or receive from a nil channel "+
					"blocks FOREVER, which sounds like a trap until ``select`` turns it into a "+
					"tool — a nil channel in a select case disables that case, letting code switch "+
					"arms on and off by nil-ing a channel variable:"),
			code("ch-directional-signature", "go",
				"Direction types in a real worker signature: powers visible in the types.",
				"server/internal/domain/demos/context_cancellation.go",
				"func ctxRunWorker(ctx context.Context, work <-chan string, acks chan<- string, observed chan<- ctxObservation, id string) {"),
			text("ch-coda",
				"One design rule gathers all of the above: a channel has exactly one owner at a "+
					"time — the goroutine allowed to send on it (and therefore to close it) — and "+
					"ownership either stays put or transfers with the values. Almost every channel "+
					"bug, from double-close panics to goroutine leaks, is ultimately an ownership "+
					"bug; the hygiene section at the end of this topic returns to the leak half of "+
					"that sentence with a live demonstration."),
		),
		// ── 4. select and timeouts ─────────────────────────────────────
		section("select-and-timeouts", "select and timeouts",
			"One construct that waits on several channels — and the two ways to bound the wait.",
			text("sel-pitch",
				"``select`` is the control-flow twin of ``switch``, but its cases are channel "+
					"operations and its semantics are those of a race run fairly: select BLOCKS "+
					"until at least one case can proceed, and when several can proceed at the same "+
					"instant it picks among them uniformly at random — a deliberate design that "+
					"prevents one always-ready case from starving the others. An empty ``select`` "+
					"blocks forever (a legitimate park), and a ``default`` case runs when nothing "+
					"else is ready, turning select into a non-blocking probe. The composition root "+
					"is the canonical real use in this repo — the main goroutine waits on exactly "+
					"two futures, the server's error channel and the signal context's done:"),
			code("sel-main", "go",
				"The server's fate decided by select: a serve failure versus a signal context.",
				"server/cmd/guide/main.go",
				"select {\n"+
					"case err := <-serveErr:\n"+
					"    // Shutdown is only ever called in the signal path below, so a value\n"+
					"    // here is a genuine startup/serve failure (e.g. port already bound).\n"+
					"    return fmt.Errorf(\"serve: %w\", err)\n"+
					"case <-ctx.Done():\n"+
					"}"),
			text("sel-for-select",
				"Almost every worker goroutine in Go is the same six lines: a ``for`` loop "+
					"wrapping a ``select`` whose arms are ``do more work`` and ``stop``. The "+
					"loop shape matters as much as the select — on the stop arm the worker "+
					"reports and RETURNS, because observing cancellation while continuing to "+
					"loop would make the cancellation decorative. The cancellation demo's "+
					"worker is the textbook instance, down to reporting the cause it observed:"),
			code("sel-worker-loop", "go",
				"The standard worker: for { select { work… / done → report, return } }.",
				"server/internal/domain/demos/context_cancellation.go",
				"for {\n"+
					"    select {\n"+
					"    case <-ctx.Done():\n"+
					"        observed <- ctxObservation{workerID: id, cause: ctx.Err().Error()}\n"+
					"        return\n"+
					"    case item := <-work:\n"+
					"        acks <- item\n"+
					"    }\n"+
					"}"),
			text("sel-timeouts",
				"Bounding a wait comes in two strengths. The light one is ``time.After`` — "+
					"a select case that fires after a duration; perfect for ``reply or give ``"+
					"``up`` races and nothing more, because a fired timer tells the code nothing "+
					"WHY it fired and carries no cause. The strong one is a context deadline — "+
					"a budget attached to a unit of work that propagates to everything derived "+
					"from it and reports ``context.DeadlineExceeded`` as a checkable error. The "+
					"cancellation demo's deadline beat is the strong form in miniature: a "+
					"25-millisecond budget, nobody cancelling, a select with a work arm that "+
					"never fires — so the deadline closes ``Done()`` on its own and the worker "+
					"reads the cause off ``ctx.Err()``:"),
			code("sel-deadline-beat", "go",
				"A deadline nobody cancels: the select's Done arm wins on the timer's authority.",
				"server/internal/domain/demos/context_cancellation.go",
				"select {\n"+
					"case <-ctx.Done():\n"+
					"    result <- ctx.Err().Error()\n"+
					"case <-work:\n"+
					"    // Unreachable: no item is ever sent — the deadline wins.\n"+
					"}"),
			text("sel-default",
				"The ``default`` arm converts blocking into asking. Its most load-bearing use is "+
					"the guarded send: a send that must never park. If the reader is gone and the "+
					"channel is unbuffered, a plain send blocks forever and its goroutine leaks; "+
					"wrapped in ``select`` with a ``default``, the send is attempted once and "+
					"abandoned — the goroutine still returns, which is the difference between a "+
					"clean shutdown and a slow memory leak. The race demo runs exactly this "+
					"contrast live in its hygiene beat:"),
			code("sel-guarded-send", "go",
				"The guarded send: attempted once, abandoned rather than parked forever.",
				"server/internal/domain/demos/race_detector.go",
				"select {\n"+
					"case unguardedWouldBlock <- 1: // no reader exists: never taken\n"+
					"default: // abandoned, not parked — the goroutine still returns\n"+
					"}"),
			text("sel-coda",
				"With ``select`` in hand the concurrency toolkit is technically complete — start "+
					"work, hand values over, wait on several futures, bound every wait. What is "+
					"missing is a way to make one cancellation decision reach a whole tree of "+
					"goroutines without wiring a ``done`` channel to each by hand. That is "+
					"precisely the job of the standard library's context package."),
		),
		// ── 5. context.Context ─────────────────────────────────────────
		section("context-cancellation", "context.Context: cancellation propagation",
			"One tree of deadlines and cancel signals that every blocking call can honor — this section's demo runs it live.",
			text("ctx-pitch",
				"``context.Context`` is two deadlines and a bag of values, carried explicitly "+
					"through call chains. Concretely it is an interface with a ``Done()`` channel "+
					"that closes when the work is cancelled or out of time, an ``Err()`` saying "+
					"which of the two happened, a ``Deadline()`` reporting the budget, and "+
					"``Value`` for request-scoped data. The conventions around it are as fixed as "+
					"syntax: it is the FIRST parameter, named ``ctx``; it travels by parameter "+
					"and is never stored in a struct (a context belongs to a call stack, not to "+
					"an object's lifetime); and every function that can block takes one. This "+
					"server follows all three from the inside out — even the request-id "+
					"middleware's value lookup is a context function in the canonical shape, "+
					"with the unexported empty-struct key no other package can collide with:"),
			code("ctx-first-param", "go",
				"The convention in three lines: ctx first, values by unexported key.",
				"server/internal/core/requestid.go",
				"type requestIDKey struct{}\n"+
					"\n"+
					"func WithRequestID(ctx context.Context, id string) context.Context {\n"+
					"    return context.WithValue(ctx, requestIDKey{}, id)\n"+
					"}"),
			text("ctx-tree",
				"The power move is that contexts form a TREE. ``context.WithCancel(parent)`` and "+
					"``context.WithTimeout(parent, d)`` derive a child context carrying the "+
					"parent's values plus a new cancel condition; values flow down the tree, and "+
					"cancellation flows down it too — cancel the parent and every descendant's "+
					"``Done()`` closes, including descendants the cancelled code will never see. "+
					"One call can therefore retire a whole subsystem's worth of goroutines, which "+
					"is why the idiom for every ``With*`` is the same line of shape: derive it, "+
					"``defer`` its cancel, and let the function's exit — or the explicit cancel "+
					"site — release the whole subtree. The cancellation demo builds its tree "+
					"explicitly so the propagation can be observed:"),
			code("ctx-derivation", "go",
				"Derive, defer, repeat: the demo's tree of one parent and three descendants.",
				"server/internal/domain/demos/context_cancellation.go",
				"parent, cancelParent := context.WithTimeout(context.Background(), ctxCancelBudgetMs*time.Millisecond)\n"+
					"defer cancelParent()\n"+
					"\n"+
					"// One derivation per tree edge, each with its own deferred cancel —\n"+
					"// cancelling the parent propagates, but every With* still owes its own\n"+
					"// release (go vet's lostcancel check enforces the habit).\n"+
					"alphaCtx, cancelAlpha := context.WithCancel(parent)\n"+
					"defer cancelAlpha()"),
			text("ctx-causes",
				"A closed ``Done()`` always comes with a cause, and the cause is checked, never "+
					"string-matched: ``errors.Is(ctx.Err(), context.Canceled)`` is true when a "+
					"cancel call won; ``errors.Is(ctx.Err(), context.DeadlineExceeded)`` when the "+
					"budget ran out. Those are different failures with different responses — a "+
					"cancelled handler typically returns quietly while an exceeded deadline "+
					"usually surfaces as a 503 — and both are sentinel errors that compose with "+
					"the wrapping taxonomy of topic 30. The demo's deadline beat asserts the "+
					"check itself, the way production code must:"),
			code("ctx-errors-is", "go",
				"The cause check production code writes, run by the demo's deadline beat.",
				"server/internal/domain/demos/context_cancellation.go",
				"DeadlineExceeded: errors.Is(ctx.Err(), context.DeadlineExceeded),"),
			text("ctx-shutdown-real",
				"The production tie-in is graceful shutdown. When SIGTERM arrives, this server "+
					"must stop accepting connections, let in-flight requests finish, and hard-stop "+
					"if they do not — a parent deadline around the whole drain. The composition "+
					"root does it with one derived context and the http.Server's own "+
					"context-aware ``Shutdown``; requests that observe the deadline report it "+
					"through their own contexts, which is the whole system cooperating through "+
					"one tree:"),
			code("ctx-graceful-shutdown", "go",
				"The real graceful shutdown: a bounded drain context around srv.Shutdown.",
				"server/cmd/guide/main.go",
				"shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)\n"+
					"defer cancel()\n"+
					"if err := srv.Shutdown(shutdownCtx); err != nil {\n"+
					"    return fmt.Errorf(\"graceful shutdown: %w\", err)\n"+
					"}"),
			demo("conc:context", "Cancellation propagation — the choreography, live",
				"Performed on every request: three workers derived through a real context tree "+
					"each complete an acknowledged work phase, the parent is explicitly cancelled "+
					"before its armed 400ms deadline, and the payload records the propagation as "+
					"structure — which derivation level observed Done(), in what recorded order, "+
					"each with its cause — then a second beat lets a 25ms deadline fire on its "+
					"own (context.DeadlineExceeded, verified with errors.Is), and a ledger closes "+
					"the books: every goroutine joined, every context cancelled."),
			text("ctx-beats-intro",
				"Read the demo in five beats:"),
			list("ctx-beats", false,
				"Choreography — the configuration as facts: the 400ms armed budget, two work "+
					"items per worker, the explicit-cancel trigger, and the derivation tree "+
					"(parent to alpha and beta, alpha to gamma).",
				"Work phase — each worker's items processed before the cancel, proven by the "+
					"unbuffered send-plus-ack handshake, so the counts are deterministic.",
				"Propagation — one row per worker: generation, derived-from, the position at "+
					"which the choreography recorded its observation, and the cause it read off "+
					"Err() — context canceled for all three, because gamma is cancelled through "+
					"alpha, not directly.",
				"Deadline beat — the 25ms context nobody cancels: outcome deadline exceeded, "+
					"cause context deadline exceeded, errors.Is true.",
				"Ledger — four goroutines launched and joined, five contexts created and "+
					"cancelled: the leak-free books a cancellation choreography must balance."),
			text("ctx-handler-thin",
				"The endpoint behind the demo is deliberately the least interesting code in the "+
					"topic — a handler that adds nothing, over a pure builder that owns its own "+
					"contexts. The demo's choreography must not be coupled to any one request's "+
					"lifetime, which is why the builder takes no context from the handler and "+
					"the handler holds no dependencies at all:"),
			code("ctx-thin-handler", "go",
				"The demo endpoint: pure builder, one status, one JSON write.",
				"server/internal/presentation/demo_goroutines.go",
				"func (h *ContextCancellationHandler) ServeHTTP(w http.ResponseWriter, _ *http.Request) {\n"+
					"    writeJSON(w, http.StatusOK, demos.BuildContextCancellationPayload())\n"+
					"}"),
			text("ctx-coda",
				"The rules that make context trustworthy in production, each of which the demo "+
					"embodies: pass ``ctx`` as the first parameter of anything that can block; "+
					"derive with ``WithCancel``/``WithTimeout`` and ``defer`` the cancel on the "+
					"same line — ``go vet``'s lostcancel check fails builds that drop it; honor "+
					"``Done()`` in every loop that waits; check causes with ``errors.Is``, never "+
					"by string; and give every outbound call a deadline, because an unbounded "+
					"wait is a leak with extra steps. Topics 37 and 39 build entire shutdown and "+
					"resilience strategies on exactly this discipline."),
		),
		// ── 6. errgroup ────────────────────────────────────────────────
		section("errgroup", "errgroup: WaitGroup plus error semantics",
			"What golang.org/x/sync/errgroup adds, taught by comparison — this guide stays stdlib by design.",
			text("eg-gap",
				"A bare WaitGroup has one blind spot that bites the moment goroutines can FAIL: "+
					"it counts completions and returns nothing. ``Wait`` cannot tell the caller "+
					"that seven of two hundred workers finished with a database error. The "+
					"hand-rolled stdlib fix is a known shape — a results-or-errors channel "+
					"buffered to the goroutine count, one send per goroutine, and a collector "+
					"that decides whether the first error aborts the batch — and this repo's "+
					"race demo's channel fold is the happy-path half of exactly that pattern. "+
					"It works, it is explicit, and at two or three goroutines it is the code a "+
					"reader most wants to find."),
			text("eg-what-it-adds",
				"``golang.org/x/sync/errgroup`` packages that shape into three additions. "+
					"``group.Go(func() error)`` replaces the Add/defer-Done ceremony — it manages "+
					"the WaitGroup internally. ``group.Wait()`` returns the FIRST non-nil error "+
					"(nil only when every goroutine succeeded), replacing the manual results "+
					"channel. And ``errgroup.WithContext(ctx)`` derives a context the group "+
					"cancels the moment any member returns an error — the abort-on-first-failure "+
					"behavior that is genuinely tedious to write by hand, because every member "+
					"must select on the group context or the abort never reaches the goroutines "+
					"still running. A fourth helper, ``group.SetLimit(n)``, bounds concurrency by "+
					"blocking ``Go`` above n in-flight members — the bounded fan-out rule from "+
					"the goroutines section, as a one-liner."),
			text("eg-why-not-here",
				"This guide deliberately does not take the dependency — its go.mod is the honest "+
					"exhibit, stdlib-only from bootstrap, and every concurrency idiom in these "+
					"topics is therefore one you can read straight out of the standard library's "+
					"documentation. That is a teaching choice, not a judgment: errgroup is "+
					"well-maintained, small, and idiomatic in production services:"),
			code("eg-gomod", "mod",
				"The whole manifest: no golang.org/x/sync, by design.",
				"server/go.mod",
				"module guide\n"+
					"\n"+
					"go 1.27.3"),
			text("eg-when",
				"The decision rule in practice: reach for errgroup when a FAN-OUT of interchangeable "+
					"units should die together — parallel upstream fetches where one failure "+
					"invalidates the response, batch processing where the first error should stop "+
					"the batch, retries with bounded parallelism — and especially when the units "+
					"already take contexts, because WithContext then composes cancellation for "+
					"free. Stay with plain channels and WaitGroup when the structure is a "+
					"pipeline rather than a fan-out (different stages, different channels — an "+
					"errgroup models sameness, pipelines model difference), when the goroutines "+
					"cannot fail, or when there are so few of them that the hand-rolled shape is "+
					"shorter than the import. Topic 38 revisits the package on performance "+
					"grounds, where its SetLimit doubles as the standard semaphore."),
		),
		// ── 7. The race detector and hygiene ───────────────────────────
		section("race-detector", "The race detector and goroutine-leak hygiene",
			"Unsynchronized shared state is undefined behavior — the toolchain catches it, and two live fixes prove it.",
			text("race-def",
				"A data race is two goroutines accessing the same memory location with at least "+
					"one write and no happens-before ordering between them — and in Go, as in C "+
					"and Rust, a racy program is not merely wrong sometimes, it is UNDEFINED: the "+
					"compiler and runtime owe it nothing. The trap is that the racy code looks "+
					"like the simplest possible thing. ``counter++`` is not one operation but "+
					"three — load, add, store — and two goroutines interleaving those steps lose "+
					"updates silently. The pinned exhibit below is this topic's demonstration "+
					"code for exactly that bug, kept as DATA in the race demo's builder: two "+
					"hundred goroutines, twenty-five increments each, no synchronization. It is "+
					"never compiled into a call in this package — the demo runs only the FIXED "+
					"versions, so the guide's own race-detector-verified test suite stays green:"),
			code("race-racy-code", "go",
				"The exhibit, pinned as data in the demo builder: the unsynchronized counter.",
				"server/internal/domain/demos/race_detector.go",
				"const racyCounterCode = `package raceexhibit\n"+
					"\n"+
					"import \"sync\"\n"+
					"\n"+
					"func RacyTotal(goroutines, increments int) int {\n"+
					"    counter := 0\n"+
					"    var wg sync.WaitGroup\n"+
					"    for i := 0; i < goroutines; i++ {\n"+
					"        wg.Add(1)\n"+
					"        go func() {\n"+
					"            defer wg.Done()\n"+
					"            for j := 0; j < increments; j++ {\n"+
					"                counter++\n"+
					"            }\n"+
					"        }()\n"+
					"    }\n"+
					"    wg.Wait()\n"+
					"    return counter\n"+
					"}`"),
			text("race-detector-tool",
				"The race detector is the toolchain's answer: a dynamic analyzer (ThreadSanitizer "+
					"under the hood) built into the go command, enabled by the ``-race`` flag, that "+
					"instruments loads and stores and reports any ordered-but-unhappened pair. Two "+
					"properties govern how you use it. It is DYNAMIC — it only sees the "+
					"interleavings the run actually executes, so a clean run does not prove "+
					"race-freedom; a report, however, is always a real bug with two stack traces "+
					"attached. And it is cheap enough to be habit: this guide's standard test "+
					"invocation runs ``go test -race`` on every verification, which is precisely "+
					"how the exhibit's verdict was captured — one documented run, quoted verbatim "+
					"into the demo's payload as a pinned constant:"),
			code("race-verdict", "go",
				"The toolchain's own words, pinned: one WARNING block and the FAIL tail.",
				"server/internal/domain/demos/race_detector.go",
				"const raceVerdict = `==================\n"+
					"WARNING: DATA RACE\n"+
					"Read at 0x00c0000141f8 by goroutine 17:\n"+
					"  raceexhibit.RacyTotal.func1()\n"+
					"      /tmp/race-test/racy.go:13 +0x94\n"+
					"// …\n"+
					"--- FAIL: TestRacyTotal (0.00s)\n"+
					"    racy_test.go:7: racy total = 3900, want 5000\n"+
					"    testing.go:1867: race detected during execution of test\n"+
					"FAIL`"),
			text("race-verdict-read",
				"Learn to read the verdict once and it pays forever. The header pair — ``Read at`` "+
					"an address ``by goroutine N`` and ``Previous write at`` the same address ``by "+
					"goroutine M`` — names the two racing accesses and their goroutines; the "+
					"stacks under each (elided here to the exhibit's frames) show the exact line "+
					"both came through, the same ``counter++`` line. And the tail is the "+
					"practical lesson in one line of arithmetic: the run's total came back 3900, "+
					"not 5000 — eleven hundred increments vanished between racing read-modify-"+
					"writes. A data race is not a theoretical smear on an otherwise correct "+
					"answer; the answer itself is already wrong."),
			text("race-fix-mutex",
				"Fix one: put the shared variable under a mutex so the read-modify-write becomes "+
					"a critical section. This is production code, not an exhibit — the scenario "+
					"store guards the server's one piece of mutable shared state with an "+
					"``sync.RWMutex``, readers taking the read lock in parallel and every write "+
					"taking the exclusive lock; the pointer receivers exist precisely because the "+
					"mutex must never be copied (topic 01's rule, enforced here by the same "+
					"choice). The race demo's live mutex run is the same fix at two hundred "+
					"goroutines, and its total lands on exactly 5000, every call:"),
			code("race-mutex-production", "go",
				"Production synchronization: the scenario store's RWMutex read path.",
				"server/internal/core/scenario.go",
				"func (s *ScenarioStore) Snapshot() ScenarioState {\n"+
					"    s.mu.RLock()\n"+
					"    defer s.mu.RUnlock()\n"+
					"    return s.current\n"+
					"}"),
			text("race-fix-channel",
				"Fix two: remove the sharing. Each goroutine owns a local counter that nobody "+
					"else can touch — unsynchronized and race-free BECAUSE it is unshared — and "+
					"ownership transfers with the one tally each sends over the buffered "+
					"channel; a single goroutine folds the results after the join. That is the "+
					"``share memory by communicating`` slogan as an actual architecture, and the "+
					"race demo runs it live beside the mutex run so the payload shows both fixes "+
					"landing on exactly 5000. Choosing between them is a design question, not a "+
					"style one: the mutex wins when the state must stay shared and central "+
					"(a store, a cache, a registry); the channel wins when work and results are "+
					"naturally flowing and ownership can move with them."),
			text("race-leak-def",
				"The second discipline is goroutine-leak hygiene, and it deserves equal billing "+
					"because leaks are the bug the race detector CANNOT see — a leaked goroutine "+
					"breaks no happens-before rule, it simply never proceeds again. A leak is any "+
					"goroutine blocked forever on something that will never come: the shapes are "+
					"few and all of them appear in real code:"),
			list("race-leak-shapes", false,
				"Blocked send — an unbuffered (or full) channel send whose reader has returned; "+
					"the fix is the guarded send from the select section, or a buffer sized to "+
					"the sender count.",
				"Blocked receive — waiting on a channel nothing will ever send or close; the fix "+
					"is closing on the producer side, or selecting on a context's Done.",
				"Abandoned wait — a WaitGroup whose Wait outlives a goroutine stuck in either of "+
					"the first two shapes; the fix is upstream, plus a context so the wait can end.",
				"Forgotten timer/ticker — a ``time.Ticker`` never stopped pins its goroutine; the "+
					"fix is ``defer ticker.Stop()`` wherever one is created.",
				"Unbounded spawn — not blocked but growing: fan-out without a cap; the fix is a "+
					"worker pool or SetLimit from the errgroup section."),
			demo("conc:race", "The race detector — pinned verdict, live fixes",
				"The racy counter is taught, not run: its exact code and the race detector's own "+
					"verdict (captured from a documented go test -race run, including the lost-"+
					"update arithmetic — 3900, not 5000) are pinned as data. Beside them, both "+
					"fixes run live and land on exactly 5000: a mutex-guarded total over 200 "+
					"goroutines and a channel-fold where each goroutine owns an unshared local. A "+
					"hygiene beat runs the leaked-vs-done contrast for real — a delivered send "+
					"versus a guarded send abandoned by select+default — with both goroutines "+
					"joined before the payload exists."),
			text("race-coda",
				"Read the demo as the topic's enforcement story in miniature: the pinned verdict "+
					"is what the toolchain says about unsynchronized sharing; the two exact "+
					"totals are what synchronization buys; the hygiene rows are what happens "+
					"when channel discipline slips. Everything this topic taught — joins for "+
					"every launch, one owner per channel, contexts that give every wait an exit, "+
					"bounded fan-out — exists to keep both halves of that story true in "+
					"production code."),
		),
		// ── 8. Terminal companion ──────────────────────────────────────
		section("terminal-companion", "Terminal companion: the curl tour",
			"The concurrency surfaces of this guide, read from the shell.",
			text("term-pitch",
				"With the stack up (server on :8100), these commands walk the exact surfaces this "+
					"topic lives in — the topic itself, then both live demos, then the shared "+
					"scenario toggle. ``python3 -m json.tool`` keeps the tour dependency-free; "+
					"every payload below is deterministic, so repeated calls return identical "+
					"shapes worth diffing:"),
			code("term-topic", "bash", "", "",
				"# this topic, in full: sections → text/code/list/demo blocks\n"+
					"curl -s localhost:8100/api/content/goroutines-and-channels | python3 -m json.tool"),
			code("term-context-demo", "bash", "", "",
				"# the cancellation choreography: tree, work phase, propagation order,\n"+
					"# deadline beat, ledger — same shape every call\n"+
					"curl -s localhost:8100/api/demo/conc/context | python3 -m json.tool"),
			code("term-race-demo", "bash", "", "",
				"# the pinned verdict beside two live exact totals, plus hygiene rows\n"+
					"curl -s localhost:8100/api/demo/conc/race | python3 -m json.tool"),
			code("term-index", "bash", "", "",
				"# the curriculum index: this topic sits in group \"framework\", order 6\n"+
					"curl -s localhost:8100/api/content | python3 -m json.tool"),
			code("term-scenario", "bash", "", "",
				"# the runtime toggle every demo reads (fake/live data, failure injection)\n"+
					"curl -s localhost:8100/api/demo/scenario"),
			code("term-404", "bash", "", "",
				"# unknown topic → the shared error envelope + x-request-id\n"+
					"curl -s -i localhost:8100/api/content/nope"),
			text("term-coda",
				"Both demos reward a second look with ``-s`` piped through a diff: the context "+
					"payload's propagation rows and the race payload's observed totals are pinned "+
					"by tests on the server side, so what you diff is the live choreography "+
					"against its own contract. For the runtime view underneath these idioms — "+
					"the scheduler juggling the very goroutines these demos launch — topic 02's "+
					"demo and page are the next stop; for the resilience patterns built on "+
					"context deadlines, fast-forward to topic 39."),
		),
	}

	if buildErr != nil {
		return Topic{}, buildErr
	}
	return NewTopic(
		"goroutines-and-channels",
		"Goroutines and channels",
		"The concurrency programming model — goroutines, channels, select, context "+
			"cancellation, and the race-detector discipline that keeps them honest.",
		"framework",
		6,
		sections,
	)
}
