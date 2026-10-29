package content

import (
	"errors"
	"fmt"
)

// This file is topic 39 reliability (group ship, curriculum order 39):
// keeping a distributed Go service USEFUL while its dependencies
// misbehave. The topic's spine is the failure taxonomy a remote call
// can suffer — the clean error, the slow answer, the missing answer —
// and the tool shaped for each: context deadlines for the slow and
// missing ones, bounded retry with backoff for the transient ones, a
// circuit breaker for the sustained ones, degradation and bulkheads
// for the ones that are yours. Every mechanism is taught from this
// repo's real code: the boot context and server timeouts main.go
// actually runs, the jobs worker's bounded exponential backoff, the
// redis adapter's fail-fast-then-degrade optional-service discipline,
// the worker pool as a bulkhead, the rate limiter as a shedder, and
// the ScenarioStore as a chaos toggle. The topic's demo,
// reliability:breaker, is the real thing: a mutex-guarded
// closed/open/half-open circuit breaker that PERSISTS for the process
// lifetime, protecting a downstream call which honors the shared
// scenario panel's failure injection and respects context deadlines —
// flipping the global ScenarioPanel trips and recovers the breaker
// live across requests, and every timing the payload serves was
// measured around the calls that just happened. Commercial resilience
// libraries (sony/gobreaker, resilience4j, Polly) live in comparison
// PROSE only, per the family stance; the terminal companion walks the
// whole trip-and-recover arc over curl.

// reliabilityTopic builds topic 39.
func reliabilityTopic() (Topic, error) {
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
		// ── 1. Distributed failure ───────────────────────────────
		section("distributed-failure", "Distributed failure: the network is the weather",
			"A local call either returns or panics — a remote call has a third option: neither. This section names the failure modes distributed systems actually suffer and the tool shaped for each, the map the rest of the topic walks.",
			text("rel-df-partial-failure",
				"THE DEFINING FACT of a distributed backend is PARTIAL FAILURE. A function call inside one process shares the caller's fate: it returns, or the process dies with it, and nothing in between exists. A call across the network has no such contract — the database can be up while its disk is full, the dependency can answer in an hour instead of an instant, the connection can be half-open after a NAT timeout with NEITHER side knowing. The moment your handler dials anything else — PostgreSQL, Redis, a payment API — its correctness stops being a property of your code alone and becomes a property of the WEATHER between two machines. Topic 33's observability made that weather visible; this topic is the engineering that keeps serving through it. The distributed-systems canon calls this the network being unreliable; the practitioner's version is blunter: given enough calls, every dependency fails sometimes, and the only variable you control is what your service does in that moment."),
			text("rel-df-three-shapes",
				"REMOTE FAILURES COME IN THREE SHAPES, and each shape wants a different tool — which is why this topic has sections instead of one trick. The first shape is the CLEAN ERROR: the call returns, but with bad news — a refused connection, a reset, a 5xx, a DNS miss. Go models this beautifully because errors are values (topic 30): the failure arrives as an ordinary return, and your normal error handling is already the right shape. The second shape is LATENCY: the call eventually answers, but far too late — and from the caller's chair, a slow success and a hang are the same event until a DEADLINE turns them into an error. The third shape is the SUSTAINED OUTAGE: not one failed call but a pattern of them, where every further attempt spends your user's patience and your goroutines' lifetimes on a dependency that is not coming back soon. Timeouts convert shapes two and three into shape one; retries handle shape one when it is transient; the circuit breaker refuses to keep spending on shape three at all."),
			list("rel-df-tool-map",
				false,
				"Clean error — the dependency answered with bad news: handle it like any error value; retry only when the operation is idempotent and the failure plausibly transient.",
				"Latency — the answer is late or never: bound every call with a context deadline so the stall becomes context.DeadlineExceeded inside your budget, not after your user gave up.",
				"Sustained outage — failure is the pattern, not the event: open a circuit breaker so calls fail fast without touching the dependency, and probe on a cooldown until it heals.",
				"Your own overload — the failure is on your side of the wire: degrade honestly, shed load early, and keep the compartments isolated so one hot dependency cannot sink the whole process.",
			),
			text("rel-df-stance",
				"THE STANCE THIS TOPIC TAKES is the one the whole guide takes: mechanisms over magic, and every mechanism from real code. Go's standard library already ships the load-bearing pieces — ``context`` for deadlines and cancellation (topic 02 taught the select mechanics; this topic spends them on remote calls), the ``time`` package for backoff arithmetic, ``sync`` for the shared state a breaker is — so the family stance holds: commercial resilience libraries are taught by comparison in prose, not added as dependencies, and the circuit breaker you will meet below is a real one this repo runs, small enough to read in one sitting and honest enough to serve only what it measured. Reliability is also where earlier topics pay out: the error taxonomy (topic 30) gives every injected failure one envelope, the metrics registry (topic 33) gives every fast-fail a counter, and the scenario store (topics 30 and 36) gives the failure itself a switch you can flip live."),
		),
		// ── 2. Timeouts and deadlines ────────────────────────────
		section("timeouts-deadlines", "Timeouts: the budget ledger and context deadlines",
			"An unbounded call is an outage you scheduled. This section teaches the deadline as a budget — the request's total spend divided across its calls — and quotes the timeouts this server actually boots with.",
			text("rel-to-budget",
				"EVERY REMOTE CALL NEEDS A BUDGET, and in Go the budget is the ``context.Context`` that is already your function's first parameter (the convention every blocking function in this repo follows). ``context.WithTimeout`` derives a child context whose ``Done()`` channel closes when the budget is spent; the call carries that context, and every participant — your dial, the query, the remote service when it speaks gRPC — races its work against the deadline. The BUDGET LEDGER mental model is what keeps the arithmetic sane: a request that must answer inside its own deadline cannot spend more than that deadline TOTAL across all of its calls, so parents cap children (a child's deadline can only be tighter than its parent's — passing a long child deadline under a short parent deadline still dies with the parent), and parallel calls share the remaining clock rather than multiplying it. When the deadline fires, the error is ``context.DeadlineExceeded`` — a value like any other, wrapped with ``%w`` up the stack, matched with ``errors.Is``, and mapped by the taxonomy onto the envelope a client sees. This server's own composition root opens exactly one boot-scoped budget before anything dials:"),
			code("rel-to-boot-ctx", "go",
				"The composition root's first reliability act: dials and migrations get a bounded window, so a wedged database fails the boot deterministically.",
				"server/cmd/guide/main.go",
				"bootCtx, bootCancel := context.WithTimeout(context.Background(), 30*time.Second)\n"+
					"defer bootCancel()"),
			text("rel-to-two-layers",
				"TIMEOUTS LIVE AT TWO ALTITUDES, and a production service needs both. The SERVER layer bounds the connection itself — how long a client may take to send a request, how long the handler may take to answer, how long an idle keep-alive connection may linger — and Go's ``http.Server`` carries them as configuration fields. The CALL layer bounds each piece of work the handler does — a query, a cache fetch, an upstream request — with derived contexts. The two compose: a handler racing a write timeout that also faces a database stall wants the database call to lose ITS deadline first, so the error names the database and not the connection reset. The server-level bounds are also abuse defense: a slowloris client that drips bytes is defeated by a bounded read, which is why the fields below are not optional decoration but part of this process's security posture (topic 25's threat-model habit applied to time itself):"),
			code("rel-to-server-timeouts", "go",
				"The server this guide runs: every connection dimension bounded — slow clients included.",
				"server/cmd/guide/main.go",
				"srv := &http.Server{\n"+
					"	Addr:    fmt.Sprintf(\":%d\", cfg.Port),\n"+
					"	Handler: handler,\n"+
					"	// Bounded reads/writes defeat slow-client abuse while leaving ample\n"+
					"	// headroom for the guide's demos.\n"+
					"	ReadHeaderTimeout: 5 * time.Second,\n"+
					"	ReadTimeout:       10 * time.Second,\n"+
					"	WriteTimeout:      15 * time.Second,\n"+
					"	IdleTimeout:       60 * time.Second,\n"+
					"}"),
			text("rel-to-deadline-mechanics",
				"THE MECHANICS WERE TAUGHT in the concurrency topics; reliability is where they earn their keep. The pattern every deadline-respecting call reduces to is a ``select`` with a work arm and a ``ctx.Done()`` arm — and when no work ever arrives, the deadline wins on its own, which is exactly the shape the go-runtime topic's demo pinned: a goroutine parked on a channel that never sends, unparked by the deadline, the cause checked with ``errors.Is`` the way production code must check it. That beat, quoted below, is the WHOLE latency story in miniature: the dependency stalled, no error was coming from its side, and the deadline converted the silence into ``context.DeadlineExceeded``. One discipline makes the pattern safe everywhere: always ``defer cancel()`` so the child's resources are released even on the success path, and always treat the deadline error as DATA — it feeds the retry decision (next section), the breaker's failure count (the section after), and the metrics that tell you the budget was too small (topic 33's request-duration histogram). gRPC carries this across the process boundary for free: a deadline set on the client's context travels the wire and cancels work in the SERVER too (topic 15's console demonstrates it over real RPCs)."),
			code("rel-to-deadline-beat", "go",
				"The runtime topic's deadline beat, quoted: the select where a deadline converts silence into an error.",
				"server/internal/domain/demos/context_cancellation.go",
				"func ctxDeadlineBeat() DeadlineOutcome {\n"+
					"	ctx, cancel := context.WithTimeout(context.Background(), ctxDeadlineBeatMs*time.Millisecond)\n"+
					"	defer cancel()\n"+
					"	// …\n"+
					"	work := make(chan string)\n"+
					"	select {\n"+
					"	case <-ctx.Done():\n"+
					"		result <- ctx.Err().Error()\n"+
					"	case <-work:\n"+
					"		// Unreachable: no item is ever sent — the deadline wins.\n"+
					"	}\n"+
					"	// …\n"+
					"	cause := <-result\n"+
					"	return DeadlineOutcome{\n"+
					"		// …\n"+
					"		DeadlineExceeded: errors.Is(ctx.Err(), context.DeadlineExceeded),\n"+
					"	}\n"+
					"}"),
		),
		// ── 3. Retries and backoff ───────────────────────────────
		section("retries-backoff", "Retries: earn the second attempt",
			"A retry is a bet that the failure was transient — and like all bets it needs odds, a stake limit, and an exit. This section teaches when to retry, the backoff arithmetic that makes retries safe together, and quotes two real policies from this repo.",
			text("rel-re-when",
				"THE PRECONDITION IS IDEMPOTENCY, taught in the HTTP topic and non-negotiable here: retrying a GET is free because a GET twice is a GET once; retrying a POST that already reached its destination duplicates the write, and now your retry MANUFACTURED the failure it was guarding against. The HTTP verb's safety contract (topic 03) is the cheap version of the rule; the general version is the idempotency-key discipline the webhooks topic (topic 14) teaches senders and the messaging topic (topic 22) teaches consumers — attach an identity to the operation so the receiver can deduplicate the redelivery. With that precondition met, the bet still needs odds: retry the failures that are plausibly transient (a refused connection, a timeout, a 503 with ``Retry-After``), never the ones that are certainly yours (a 400 validation envelope will fail identically forever — retrying it just burns the budget the deadlines section gave you). The error taxonomy earns its keep here: ``errors.Is`` against the deadline and cancellation sentinels distinguishes ``the dependency was slow`` from ``the caller gave up``, which are different bets."),
			text("rel-re-backoff-why",
				"THE ARITHMETIC IS BACKOFF, and it exists because naive retry is a retry STORM in waiting. When a dependency blips, every in-flight caller of yours fails at once; if they all retry immediately, the dependency's first healthy second receives the entire failed cohort again as a spike — the recovery you hoped for becomes the outage you caused. Backoff spaces the attempts: each failure earns a longer wait, so the cohort spreads out and the dependency meets a drizzle instead of a wave. The two refinements that make it production-grade are the CAP (without one, exponential growth makes late attempts effectively never — the series must converge to an answer inside a human's patience) and JITTER, a random smear across each rung so simultaneous retriers do not lockstep into synchronized waves forever. This repo runs exactly this policy in two places, both real: the jobs worker's delay function (whose comment records the production shape it stands in for) and the reliability demo's retry series, whose waits you will watch on the wire in this topic's demo:"),
			code("rel-re-jobs-delay", "go",
				"The jobs topic's real policy: exponential rungs, a hard cap, and the 1-based attempt discipline (the salvage note in its comment is a taught lesson — see topic 29).",
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
			code("rel-re-demo-backoff", "go",
				"This topic's demo series: the same shape with an explicit jitter term, and waits as cancellable as the calls they separate.",
				"server/internal/domain/demos/reliability_breaker.go",
				"func reliabilityBackoff(cfg ReliabilityConfig, attempt int) time.Duration {\n"+
					"	shift := attempt - 1\n"+
					"	if shift > 4 {\n"+
					"		shift = 4 // attempts are bounded small; the cap is defense in depth\n"+
					"	}\n"+
					"	delay := cfg.RetryBaseDelay << shift\n"+
					"	if cfg.RetryJitter > 0 {\n"+
					"		delay += time.Duration(rand.Int64N(int64(cfg.RetryJitter) + 1))\n"+
					"	}\n"+
					"	return delay\n"+
					"}\n"+
					"\n"+
					"// reliabilityWait sleeps out d unless the context ends first — the\n"+
					"// backoff waits are as cancellable as the calls they separate.\n"+
					"func reliabilityWait(ctx context.Context, d time.Duration) error {\n"+
					"	if d <= 0 {\n"+
					"		return nil\n"+
					"	}\n"+
					"	timer := time.NewTimer(d)\n"+
					"	defer timer.Stop()\n"+
					"	select {\n"+
					"	case <-ctx.Done():\n"+
					"		return ctx.Err()\n"+
					"	case <-timer.C:\n"+
					"		return nil\n"+
					"	}\n"+
					"}"),
			list("rel-re-discipline",
				false,
				"Idempotent only — retry operations the receiver can deduplicate; a retried non-idempotent write is a self-inflicted outage.",
				"Transient failures only — deadlines, resets, and 503s are bets; 4xx envelopes are certainties that will fail identically.",
				"Bounded attempts — the series ends with the error surfaced for the caller's handling; an unbounded retry is an outage wearing a loop.",
				"Backoff with a cap and jitter — space the cohort, converge the waits, and smear the retriers off each other's clocks.",
				"Cancellable waits — every backoff sleeps on the request context, so a caller who left stops funding the retries.",
				"A budget, not a policy — retries spend the deadline ledger's remaining clock; the retry series must fit inside the request's budget or it is the timeout's job to end it.",
			),
		),
		// ── 4. The circuit breaker [demo reliability:breaker] ─────
		section("circuit-breaker", "The circuit breaker [demo reliability:breaker]",
			"Retry asks whether one failure was transient; the breaker asks whether the dependency deserves the next call at all. This section builds the real three-state breaker this repo runs — and its live demo, driven by the global scenario panel.",
			text("rel-cb-idea",
				"THE BREAKER'S JOB is to stop spending on a dependency that keeps failing. Electrical circuits have had the idea for a century: too many faults, the circuit OPENS, everything downstream stops drawing, and a human resets it later. The software version is a small state machine standing in front of a remote call, owned by the process and shared by every request that makes the call — which is precisely what makes it different from every mechanism so far: a timeout protects one call, a retry policy protects one operation, but a breaker is SHARED STATE ABOUT A DEPENDENCY, a memory the process keeps between requests. In the CLOSED state calls flow and failures accumulate toward a threshold; when the streak crosses the threshold the breaker OPENS, and for a cooldown window every call fails fast — refused in microseconds, without a socket, without a goroutine's wait, without spending the caller's deadline ledger on a lost cause. After the cooldown the breaker goes HALF-OPEN and lets exactly ONE call through as a probe: success closes the circuit and traffic resumes; failure re-opens it for a fresh cooldown. The economics are the point — under outage, fail-fast is not pessimism, it is the CHEAPEST honest answer you can give."),
			code("rel-cb-allow", "go",
				"The real gate: mutex-guarded because every request shares it; the open case fast-fails with the remaining cooldown named.",
				"server/internal/domain/demos/reliability_breaker.go",
				"func (b *ReliabilityBreaker) Allow() ReliabilityDecision {\n"+
					"	b.mu.Lock()\n"+
					"	defer b.mu.Unlock()\n"+
					"\n"+
					"	switch b.state {\n"+
					"	case ReliabilityBreakerClosed:\n"+
					"		b.counters.Allowed++\n"+
					"		return ReliabilityDecision{\n"+
					"			Allowed: true,\n"+
					"			State:   ReliabilityBreakerClosed,\n"+
					"			Reason:  \"closed — calls flow to the dependency\",\n"+
					"		}\n"+
					"	case ReliabilityBreakerOpen:\n"+
					"		elapsed := b.now().Sub(b.openedAt)\n"+
					"		if remaining := b.cooldown - elapsed; remaining > 0 {\n"+
					"			b.counters.FastFailed++\n"+
					"			return ReliabilityDecision{\n"+
					"				Allowed: false,\n"+
					"				State:   ReliabilityBreakerOpen,\n"+
					"				Reason: fmt.Sprintf(\"open — fast-fail; %s of the %s cooldown remains\",\n"+
					"					remaining.Round(time.Millisecond), b.cooldown),\n"+
					"			}\n"+
					"		}\n"+
					"		b.recordTransition(ReliabilityBreakerHalfOpen, \"cooldown elapsed — probing the dependency\")\n"+
					"		// …\n"+
					"	}\n"+
					"}"),
			text("rel-cb-transitions",
				"THE TRANSITIONS CARRY THE SEMANTICS, and each one is a decision worth reading twice. OPENING is triggered by a streak — consecutive failures, not a ratio; a streak is simpler to reason about and behaves correctly from the first data point. The PROBE is single-flight by construction: while one half-open probe is in flight, every other caller still fast-fails, because letting the whole cohort probe at once is just the closed state wearing a costume. A SUCCESSFUL probe CLOSES the circuit and resets the streak — but a FAILED probe re-opens it for a full FRESH cooldown, because one good call is evidence the dependency might be back, while one bad call after a cooldown is evidence it still is not, and the breaker stays cautious. Notice what the breaker does NOT do: it never decides the call failed — it only decides whether to ATTEMPT the call. The attempt's outcome is still the timeout's and the error taxonomy's business; the breaker just refuses to keep buying tickets to a show that keeps being cancelled:"),
			code("rel-cb-record", "go",
				"Recording outcomes: success resets the streak and a successful probe recovers; failure counts toward the threshold, and a failed probe re-opens.",
				"server/internal/domain/demos/reliability_breaker.go",
				"func (b *ReliabilityBreaker) OnSuccess() {\n"+
					"	b.mu.Lock()\n"+
					"	defer b.mu.Unlock()\n"+
					"\n"+
					"	b.counters.Succeeded++\n"+
					"	b.consecutiveFails = 0\n"+
					"	b.probeInFlight = false\n"+
					"	if b.state == ReliabilityBreakerHalfOpen {\n"+
					"		b.recordTransition(ReliabilityBreakerClosed, \"probe succeeded — recovered\")\n"+
					"	}\n"+
					"}\n"+
					"\n"+
					"// OnFailure records a failed call: counters tick, the streak grows,\n"+
					"// and the threshold opens the breaker from closed. A FAILED half-open\n"+
					"// probe re-opens it for a full fresh cooldown — one bad probe is\n"+
					"// evidence, not a verdict, so the circuit stays cautious.\n"+
					"func (b *ReliabilityBreaker) OnFailure() {\n"+
					"	b.mu.Lock()\n"+
					"	defer b.mu.Unlock()\n"+
					"\n"+
					"	b.counters.Failed++\n"+
					"	b.consecutiveFails++\n"+
					"	b.probeInFlight = false\n"+
					"	switch {\n"+
					"	case b.state == ReliabilityBreakerHalfOpen:\n"+
					"		b.recordTransition(ReliabilityBreakerOpen, \"probe failed — reopened for a fresh cooldown\")\n"+
					"	case b.state == ReliabilityBreakerClosed && b.consecutiveFails >= b.threshold:\n"+
					"		b.recordTransition(ReliabilityBreakerOpen, fmt.Sprintf(\n"+
					"			\"%d consecutive failures — threshold reached\", b.consecutiveFails))\n"+
					"	}\n"+
					"}"),
			text("rel-cb-console",
				"THE DEMO WIRES A CONSOLE around the breaker: a process-lifetime object holding the shared breaker, the scenario store the dependency honors, and the config that tunes both. Each request to the demo's endpoint WALKS the console — a bounded series of downstream calls through the persistent breaker — and runs a bounded retry-with-backoff series beside it; the handler is then three lines of observation, exactly the thin-handler invariant every topic has held. The dependency itself is the scenario store's puppet (the chaos section below reads its code): under ``error`` injection it fails cleanly, under ``latency`` it stalls until the per-call deadline converts the stall into ``context.DeadlineExceeded``, and under ``none`` it answers. Because the breaker PERSISTS between requests, the trip and the recovery are processes you watch across RELOADS — and because every elapsed figure in the payload was measured around the calls that just happened, the timeline is an observation, not an animation:"),
			code("rel-cb-handler", "go",
				"The whole handler: walk, retry, snapshot AFTER both, shape — the state the payload serves is the one the walk left behind.",
				"server/internal/presentation/demo_reliability.go",
				"func (h *ReliabilityBreakerHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {\n"+
					"	walk := h.console.Walk(r.Context())\n"+
					"	retry := h.console.RetrySeries(r.Context())\n"+
					"	writeJSON(w, http.StatusOK, demos.BuildReliabilityBreakerPayload(demos.ReliabilityBreakerFacts{\n"+
					"		Scenario: h.console.ScenarioState(),\n"+
					"		Breaker:  h.console.BreakerObservation(h.now()),\n"+
					"		Walk:     walk,\n"+
					"		Retry:    retry,\n"+
					"	}))\n"+
					"}"),
			demo("reliability:breaker", "The reliability breaker: a real circuit breaker you can trip and recover live",
				"GET /api/demo/reliability/breaker walks a bounded series of downstream calls through a REAL circuit breaker that persists for the process lifetime — closed, open, half-open, single-probe semantics, a consecutive-failure threshold, and a cooldown — protecting a dependency that honors the shared scenario panel's injectFailure enum and respects context deadlines. PATCH /api/demo/scenario to error and reload until the threshold trips the circuit open; reload again to watch calls fast-fail with the remaining cooldown named; switch to latency and watch the per-call deadline convert the stall into deadline exceeded — the failure shape only a timeout catches; switch to none, outlast the cooldown, and watch the half-open probe close the circuit. The payload carries the breaker's state, age, counters, and transition ring, the walk's attempt-by-attempt timeline, and a retry-with-backoff series whose waits are measured — every timing parsed from the calls that just happened, with the config panel stating the policy the rows actually ran."),
			text("rel-cb-what-to-watch",
				"WHAT TO WATCH IN THE DEMO is the division of labor this whole section built. With nothing injected, every attempt reaches the dependency and the circuit stays closed. Under error injection, the streak crosses the threshold MID-WALK: the rows after the trip say ``fast-fail`` and carry the breaker's own reason instead of a dependency error — those calls never spent a deadline. Under latency injection the called rows say ``deadline exceeded`` with an elapsed figure at the per-call budget, which trips the breaker exactly like errors do — the breaker does not care WHICH shape of failure, only that failing is the pattern. And on recovery, the first row after the cooldown shows ``half-open`` as its state-before and ``closed`` as its state-after: one probe, one decision, traffic resumed. The retry panel below the walk makes the contrast explicit: retry alone keeps paying the backoff rungs into a dead dependency; the breaker decides whether the payment starts at all. In production the two compose — retries INSIDE a closed breaker, the breaker refusing before the retry loop can spend — and the observability topic's counters (fast-fails per dependency) are how you notice a breaker doing its job."),
		),
		// ── 5. Degradation ───────────────────────────────────────
		section("degradation", "Graceful degradation: serve something honest",
			"When a dependency is down, the question is not how to fix it from the request path — it is what the request can still honestly do without it. This section teaches the optional-service discipline this repo practices on every non-essential store.",
			text("rel-deg-optional",
				"THE GUIDE'S OWN DISCIPLINE is the teaching material. PostgreSQL is part of this stack — the composition root dials it at boot and fails the process when it is unreachable, because without it nothing serves. Redis and MongoDB are OPTIONAL services: the boot does not require them, and every demo that uses one degrades to an honest typed answer when its store is gone — never a crash, never a fabricated stale answer dressed as fresh. The pattern, visible across the data adapters, has three moves. FAIL FAST: command timeouts bound every call to the optional store, so an unreachable address is discovered inside a bounded wait instead of the TCP stack's patience. DEGRADE HONESTLY: the adapter returns an ordinary error, and the handler maps it onto the taxonomy's ``service_unavailable`` — one envelope, a machine-readable code, a request id that joins the log line. CONTAIN THE BLAST RADIUS: the failed demo answers alone; the process, the other routes, and the health endpoint keep serving. The redis adapter's constants are the fail-fast half in its plainest form:"),
			code("rel-deg-timeouts", "go",
				"The optional-service discipline, first move: when the address points at nothing, the first command fails inside these bounds.",
				"server/internal/data/redis/playground.go",
				"const (\n"+
					"	dialTimeout  = 2 * time.Second\n"+
					"	readTimeout  = 2 * time.Second\n"+
					"	writeTimeout = 2 * time.Second\n"+
					"\n"+
					"	// pubsubWait bounds how long the subscriber waits for the published\n"+
					"	// message: a healthy store delivers in milliseconds, so this is only\n"+
					"	// ever the honest failure path for a store that stopped answering.\n"+
					"	pubsubWait = 2 * time.Second\n"+
					")"),
			text("rel-deg-ladder",
				"THE FULL LADDER of what a handler can do without a dependency runs from best to most honest. SERVE FROM CACHE: the caching topic's aside pattern means a dead origin can still answer warm entries — slightly stale beats unavailable, IF you say so. SERVE A FALLBACK: a default recommendation, an empty result page with a notice, a queue-later acknowledgment — the shape the messaging topic's outbox gives you when the side effect cannot happen now but must not be lost. DEGRADE THE FEATURE: the endpoint answers the typed 503 naming exactly what is unreachable, and the CLIENT decides what that means — the errors topic's table walks this exact envelope for the jobs console with Redis stopped. REFUSE EARLY: readiness (topic 33's ``/api/ready``) reports the missing dependency so the orchestrator stops sending NEW traffic while in-flight requests finish — that is graceful degradation at the process boundary, and the deployment topic's rollout machinery consumes it. What is NOT on the ladder is pretending: no stale answer labeled fresh, no swallowed error logged nowhere, no catch-all that turns a dead dependency into a wrong one."),
			code("rel-deg-fail-fast", "go",
				"Second move in one line of real code: the walk aborts with the command's error, wrapped — the handler maps it to the typed 503.",
				"server/internal/data/redis/playground.go",
				"stringRow, err := r.runString(ctx, key(\"greeting\"))\n"+
					"if err != nil {\n"+
					"	return demos.RedisPlaygroundRun{}, err\n"+
					"}"),
		),
		// ── 6. Bulkheads and shedding ────────────────────────────
		section("bulkheads-shedding", "Bulkheads and load shedding: fail the least",
			"A ship with compartments sinks by the compartment, not by the hull. This section teaches isolation between dependencies and refusal before collapse — and quotes the two real mechanisms this repo already runs.",
			text("rel-bh-bulkhead",
				"A BULKHEAD IS A BOUNDARY THAT LIMITS BLAST RADIUS. The naval version keeps one hull breach from flooding the ship; the service version keeps one slow or hot dependency from consuming the goroutines, connections, and memory that every OTHER dependency and endpoint need. The mechanism is almost embarrassingly simple in Go: a bounded pool of workers IS a bulkhead. When the concurrency topic's worker pool ran its fan-out, that pool had a fixed worker count — meaning at most that many jobs were ever in flight, and a flood of work queued in a channel instead of spawning an unbounded goroutine herd. The same shape isolates dependencies: a dedicated pool (or a semaphore — a buffered channel of tokens) per downstream bounds how much of YOUR process that downstream can tie up, so a pathological dependency saturates its own compartment and the rest of the service keeps its headroom. The pools you already met are the same lesson: the pgx pool's bounded connections (the performance topic quoted its constructor) is a bulkhead the database enforces; the worker pool below is one you enforce:"),
			code("rel-bh-worker", "go",
				"The concurrency topic's real worker: both blocking operations select-guarded on ctx — a bounded worker is a bulkhead with a cancellation story.",
				"server/internal/domain/demos/concurrency.go",
				"for {\n"+
					"	select {\n"+
					"	case job, ok := <-jobs:\n"+
					"		if !ok {\n"+
					"			return\n"+
					"		}\n"+
					"		select {\n"+
					"		case results <- poolResult{JobID: job.ID, Payload: job.Payload, Output: transform(job.Payload)}:\n"+
					"			handled++\n"+
					"		case <-ctx.Done():\n"+
					"			return\n"+
					"		}\n"+
					"	case <-ctx.Done():\n"+
					"		return\n"+
					"	}\n"+
					"}"),
			text("rel-bh-shedding",
				"SHEDDING IS THE COMPLEMENT: when the compartment is full or the process is over budget, the honest move is to REFUSE work at the boundary instead of queueing it into a collapse. An unbounded queue is not backpressure — it is a slower outage with more memory pressure; real backpressure says no early, says it cheaply, and tells the caller when to come back. The security topic's rate limiter is exactly this posture with a per-client budget, and its refusal is the model shed: the typed 429 envelope, ``Retry-After`` in the header, a machine-readable detail in the body — the caller can act on it instead of guessing. Shedding composes with the whole topic: a shed request is a clean error (no dependency was touched, no deadline spent), retries treat it with ``Retry-After`` respect rather than immediate re-spending, and the breaker counts the overload signal it never sees because the shed happened first. The order of defenses in a real handler is therefore timeout INSIDE retry INSIDE the breaker, with the shed at the very edge — each layer answering the question the previous one cannot:"),
			code("rel-bh-shed", "go",
				"The security topic's limiter middleware as a shedder: refuse early, name the retry window, answer one typed envelope.",
				"server/internal/presentation/security_ratelimit.go",
				"decision := limiter.Allow(ClientKey(r))\n"+
					"if decision.Allowed {\n"+
					"	next.ServeHTTP(w, r)\n"+
					"	return\n"+
					"}\n"+
					"retrySeconds := int(math.Ceil(decision.RetryAfter.Seconds()))\n"+
					"w.Header().Set(RetryAfterHeader, fmt.Sprintf(\"%d\", retrySeconds))\n"+
					"refusal := core.RateLimited(fmt.Sprintf(\n"+
					"	\"rate limit exceeded — the sustained allowance refills continuously; retry after %s\",\n"+
					"	decision.RetryAfterText()))"),
		),
		// ── 7. Chaos ─────────────────────────────────────────────
		section("chaos", "Chaos engineering: rehearse the failure",
			"You cannot verify a recovery path you have never run. This section teaches deliberate failure injection — and the guide's scenario store IS one, wired into every demo and into this topic's breaker.",
			text("rel-cha-why",
				"CHAOS ENGINEERING is the experimental method applied to the reliability claims above: a timeout you have never watched fire is a hypothesis, and so is a breaker you have never watched trip. The discipline's shape is the scientific method's: define the STEADY STATE you claim (the demo endpoint answers inside its budget), form the HYPOTHESIS that a specific failure will not break it (Redis dies, the dependency stalls), INJECT exactly that failure, and observe — with the abort conditions agreed in advance, because a chaos experiment that takes production down is just an outage with paperwork. The practice began with Netflix's Chaos Monkey (randomly killing production instances in office hours, when engineers are awake to respond) and matured into game days and fault-injection platforms; the family stance keeps the tools in prose and the METHOD in the repo, because the method is what transfers."),
			text("rel-cha-store",
				"THIS REPO RUNS A CHAOS TOGGLE ALREADY — you have been using it since the errors topic. The scenario store's ``injectFailure`` enum is a fault-injection control plane in three values: ``none`` is the steady state, ``error`` converts the demo dependencies into clean failures, and ``latency`` converts them into stalls that only a deadline resolves — the two remote failure shapes from this topic's first section, on a switch, live, at runtime. Its PATCH endpoint validates the enum (a bogus value answers the 422 envelope with the expected values in details — the errors topic uses exactly that as its validation trigger), and every demo that honors it becomes a chaos experiment you can run from the terminal. The enum itself is two lines worth quoting, because it is the whole idea in miniature — failure as a first-class, typed, reversible state:"),
			code("rel-cha-enum", "go",
				"The chaos toggle as typed state: three legal modes, validated on every PATCH, read live by every honoring demo.",
				"server/internal/core/scenario.go",
				"type FailureInjection string\n"+
					"\n"+
					"// The three legal failure-injection modes.\n"+
					"const (\n"+
					"	FailureNone    FailureInjection = \"none\"\n"+
					"	FailureError   FailureInjection = \"error\"\n"+
					"	FailureLatency FailureInjection = \"latency\"\n"+
					")"),
			text("rel-cha-dependent",
				"THE BREAKER'S DEPENDENCY IS THE STORE'S newest customer, and its code is the chaos method in one function: the scenario's CURRENT value is read at call time (not cached at boot — a flip must take effect on the very next call), each mode maps to one failure shape, and the latency arm honors the CALLER'S deadline rather than any clock of its own — the stall ends when the budget ends, wrapped with ``%w`` so the cause survives the wrapping for ``errors.Is`` to find. Run the experiment yourself with this section's terminal tour: arm error, watch the breaker trip; arm latency, watch deadlines do the tripping; arm none, watch the probe recover. Then read the run again as an experiment report — steady state, injection, observation, recovery — because that report shape (what did we believe, what did we break, what actually happened) is the entire practice, and the observability topic's logs and metrics are where the production-scale version of the same report gets written."),
			code("rel-cha-downstream", "go",
				"Failure injection honored at call time: clean error, deadline-bounded stall, or health — the three shapes on one switch.",
				"server/internal/domain/demos/reliability_breaker.go",
				"func (c *ReliabilityConsole) callDownstream(ctx context.Context) error {\n"+
					"	switch c.store.Snapshot().InjectFailure {\n"+
					"	case core.FailureError:\n"+
					"		return ErrReliabilityInjected\n"+
					"	case core.FailureLatency:\n"+
					"		timer := time.NewTimer(c.cfg.CallTimeout + reliabilityLatencyOvershoot)\n"+
					"		defer timer.Stop()\n"+
					"		select {\n"+
					"		case <-ctx.Done():\n"+
					"			return fmt.Errorf(\"dependency stalled past its deadline: %w\", ctx.Err())\n"+
					"		case <-timer.C:\n"+
					"			return nil\n"+
					"		}\n"+
					"	default:\n"+
					"		return nil\n"+
					"	}\n"+
					"}"),
		),
		// ── 8. Races ─────────────────────────────────────────────
		section("races", "Races: shared state under failure",
			"Failure paths are cold code that suddenly runs hot — and half of them touch the shared state you thought was safe. This section is the concurrency hygiene list applied to reliability machinery.",
			text("rel-race-hot-cold",
				"THE RACE DETECTOR (topics 06 and 34) is a reliability tool, not just a concurrency tool, because FAILURE PATHS ARE COLD CODE THAT RUNS HOT ALL AT ONCE. In the happy path, the breaker's counters tick one request at a time; the moment a dependency dies, every concurrent request hits the failure path simultaneously — the same streak counter, the same transition recording, the same map of limiter buckets — and code that was race-free by ACCIDENT (one caller at a time) becomes racy by DESIGN (a cohort of failures in the same microsecond). The rule this repo follows is the Go-idiomatic one: shared mutable state gets a mutex, full stop; the scenario store guards its state with an ``RWMutex`` (reads take the read lock, PATCHes take the write lock, copies cross the boundary so callers can never mutate under the store), and the breaker takes the plain mutex on every decision and record — the quotes in this topic showed both, lock and defer first lines. The alternative tools each have their lane: atomics for single counters with no multi-field invariants, channels when ownership can move instead of share, and ``sync/singleflight`` (the standard library's duplicate-suppression primitive, x/sync — comparison prose here, per the family stance) when a cohort of identical calls should collapse into one real one, which is caching-lab-adjacent and exactly what a stampede wants."),
			code("rel-race-snapshot", "go",
				"The store every demo reads under failure: read-locked, copy-out — no caller can race a PATCH through a handed-out snapshot.",
				"server/internal/core/scenario.go",
				"func (s *ScenarioStore) Snapshot() ScenarioState {\n"+
					"	s.mu.RLock()\n"+
					"	defer s.mu.RUnlock()\n"+
					"	return s.current\n"+
					"}"),
			text("rel-race-hygiene",
				"THE HYGIENE LIST for reliability machinery is short because the language carries most of it. Run the suite under ``-race`` always (this guide's standard invocation does, and the breaker's tests hammer it from many goroutines for exactly that reason). Keep critical sections SMALL — the breaker's lock covers counter arithmetic and a state check, never a network call; holding a lock across I/O serializes every request through the mutex and quietly undoes the concurrency you built. Beware check-then-act on shared state — the half-open probe's single-flight is correct because the check and the reserve happen under ONE lock acquisition, not two; the difference between those designs is the difference between one probe and a thundering herd in a trench coat. And prefer IMMUTABLE SNAPSHOTS at the boundary, the store's own pattern: copy the state under the lock, release, hand the copy out — readers never hold the lock while they work, so the writer is never waiting on them. Goroutine leaks are the quiet sibling of races on this list: every goroutine a failure path starts must have an exit that the failure actually reaches — the deadline-armed select and the cancellable backoff wait quoted earlier are the two shapes that guarantee it, because the context that bounds the call also bounds everything the call spawned."),
		),
		// ── 9. Terminal companion ─────────────────────────────────
		section("terminal-companion", "Terminal companion: the trip-and-recover tour",
			"The topic over a terminal: curl the breaker under each scenario mode and watch the whole arc — healthy service, threshold trip, fast-fail standoff, deadline conversion, and the half-open recovery — every state change observed live.",
			text("term-rel-intro",
				"With the stack up (server on the port the environment runs it, and ``curl`` plus ``python3`` on the path), the tour runs the arc in order. First THE HEALTHY BASELINE — the demo endpoint with nothing injected, where every walk attempt succeeds and the breaker reports closed. Then THE TRIP — arm the error injection through the real scenario endpoint and reload until the threshold opens the circuit; the walk rows after the trip answer ``fast-fail`` with the remaining cooldown named, and the transition ring carries the cause the breaker itself recorded. Then THE DEADLINE CONVERSION — arm latency instead, and watch called attempts end in ``deadline exceeded`` with measured elapsed figures at the per-call budget, tripping the breaker exactly like errors do. Then THE RECOVERY — disarm, wait out the cooldown the config panel names, and reload: the first row is the half-open probe, its success closes the circuit, and the counters tell the whole story since boot. Every block's ``python3`` filter prints only the fields the tour is about, but the raw payloads are the full contract the client renders:"),
			code("term-rel-baseline-bash", "bash", "", "",
				"# the topic in the index: order 39 in the ship group\n"+
					"curl -s localhost:8100/api/content | python3 -c \\\n"+
					"  'import json,sys; t=[x for x in json.load(sys.stdin)[\"topics\"] if x[\"id\"]==\"reliability\"][0]; \\\n"+
					"   print(t[\"order\"], t[\"group\"], t[\"sectionCount\"], \"sections\")'\n"+
					"\n"+
					"# the baseline: nothing injected, breaker closed, all attempts succeed\n"+
					"curl -s localhost:8100/api/demo/reliability/breaker | python3 -c \\\n"+
					"  'import json,sys; p=json.load(sys.stdin); b=p[\"breaker\"]; \\\n"+
					"   print(\"state:\", b[\"state\"], \"| counters:\", b[\"counters\"]); \\\n"+
					"   [print(\" \", r[\"index\"], r[\"action\"], r[\"outcome\"], \\\n"+
					"     str(round(r[\"elapsedMs\"],2))+\"ms\") for r in p[\"walk\"][\"rows\"]]'\n"+
					"\n"+
					"# the full topic detail: sections, blocks, the inline demo block\n"+
					"curl -s localhost:8100/api/content/reliability | python3 -c \\\n"+
					"  'import json,sys; d=json.load(sys.stdin); \\\n"+
					"   print(d[\"title\"]); \\\n"+
					"   [print(\" \", s[\"id\"], len(s[\"blocks\"]), \"blocks\") for s in d[\"sections\"]]'\n"+
					"\n"+
					"# an unknown topic answers the 404 envelope, request id and all\n"+
					"curl -s -i localhost:8100/api/content/reliability-neither | head -1"),
			code("term-rel-trip-bash", "bash", "", "",
				"# the trip: arm error injection on the REAL scenario endpoint\n"+
					"curl -s -X PATCH localhost:8100/api/demo/scenario \\\n"+
					"  -H 'Content-Type: application/json' -d '{\"injectFailure\": \"error\"}'\n"+
					"\n"+
					"# reload: the threshold trips mid-walk, later attempts fast-fail\n"+
					"curl -s localhost:8100/api/demo/reliability/breaker | python3 -c \\\n"+
					"  'import json,sys; p=json.load(sys.stdin); b=p[\"breaker\"]; \\\n"+
					"   print(\"state:\", b[\"state\"], \"| age:\", round(b[\"stateAgeMs\"]), \"ms\"); \\\n"+
					"   [print(\" \", r[\"index\"], r[\"action\"], r[\"outcome\"], \\\n"+
					"     (r.get(\"detail\") or \"\")[:60]) for r in p[\"walk\"][\"rows\"]]; \\\n"+
					"   [print(\" transition:\", t[\"from\"], \"->\", t[\"to\"], \"-\", t[\"cause\"]) \\\n"+
					"    for t in b[\"transitions\"][-2:]]'\n"+
					"\n"+
					"# reload again inside the cooldown: every attempt refused, none spent\n"+
					"curl -s localhost:8100/api/demo/reliability/breaker | python3 -c \\\n"+
					"  'import json,sys; p=json.load(sys.stdin); \\\n"+
					"   print(\"state:\", p[\"breaker\"][\"state\"], \"| fastFailed:\", \\\n"+
					"     p[\"breaker\"][\"counters\"][\"fastFailed\"]); \\\n"+
					"   print(p[\"walk\"][\"note\"])'"),
			code("term-rel-recover-bash", "bash", "", "",
				"# the deadline conversion: stalls become deadline exceeded\n"+
					"curl -s -X PATCH localhost:8100/api/demo/scenario \\\n"+
					"  -H 'Content-Type: application/json' -d '{\"injectFailure\": \"latency\"}'\n"+
					"curl -s localhost:8100/api/demo/reliability/breaker | python3 -c \\\n"+
					"  'import json,sys; p=json.load(sys.stdin); \\\n"+
					"   [print(\" \", r[\"index\"], r[\"outcome\"], \\\n"+
					"     str(round(r[\"elapsedMs\"],1))+\"ms\") for r in p[\"walk\"][\"rows\"]]; \\\n"+
					"   rr=p[\"retry\"]; \\\n"+
					"   [print(\" retry\", r[\"attempt\"], r[\"outcome\"], \\\n"+
					"     \"wait\", round(r[\"backoffMs\"],1), \"ms\") for r in rr[\"rows\"]]; \\\n"+
					"   print(\" retry total:\", round(rr[\"totalElapsedMs\"]), \"ms\")'\n"+
					"\n"+
					"# the recovery: disarm, outlast the cooldown, watch the probe close\n"+
					"curl -s -X PATCH localhost:8100/api/demo/scenario \\\n"+
					"  -H 'Content-Type: application/json' -d '{\"injectFailure\": \"none\"}'\n"+
					"sleep 3   # the config panel names the cooldown; outlast it\n"+
					"curl -s localhost:8100/api/demo/reliability/breaker | python3 -c \\\n"+
					"  'import json,sys; p=json.load(sys.stdin); b=p[\"breaker\"]; \\\n"+
					"   print(\"state:\", b[\"state\"], \"| counters:\", b[\"counters\"]); \\\n"+
					"   [print(\" \", r[\"index\"], r[\"stateBefore\"], \"->\", r[\"stateAfter\"], \\\n"+
					"     r[\"outcome\"]) for r in p[\"walk\"][\"rows\"]]; \\\n"+
					"   print(p[\"walk\"][\"note\"])'\n"+
					"\n"+
					"# a bogus injection mode answers the 422 with the expected values\n"+
					"curl -s -X PATCH localhost:8100/api/demo/scenario \\\n"+
					"  -H 'Content-Type: application/json' -d '{\"injectFailure\": \"bogus\"}'"),
			text("term-rel-coda",
				"Read the tour as the topic's argument, executed. The baseline block is the STEADY STATE the chaos section said every experiment starts from. The trip block is the BREAKER EARNING ITS KEEP live: threshold reached mid-walk, refusals carrying the remaining cooldown, the transition ring recording causes you can quote in a postmortem. The recovery block is the whole DEGRADATION ARC in one payload — half-open probe, successful close, counters that still remember the outage — and its sleep is the tour's only patience: the cooldown the config panel names is the cooldown the sleep outlasts, no magic. The deadline block is the TIMEOUT SECTION over the wire: measured elapsed figures at the budget, backoff waits between retry attempts, and a breaker that tripped on stalls without ever seeing an error from the dependency's side. Reliability closes the platform arc the curriculum ordered it: observable, debuggable, measurable — and now SURVIVABLE; what remains is scaling, the same disciplines under multiplication."),
		),
	}

	if buildErr != nil {
		return Topic{}, buildErr
	}
	return NewTopic(
		"reliability",
		"Reliability in Go: survive your dependencies",
		"Keeping a distributed Go service useful while its dependencies misbehave — taught from this repo's real machinery: three failure shapes — the clean error, the late answer, the missing one — and the map that pairs each with its tool.",
		"ship",
		39,
		sections,
	)
}
