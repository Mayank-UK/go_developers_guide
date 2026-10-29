package content

import (
	"errors"
	"fmt"
)

// This file is topic 38 performance (group ship, curriculum order 38):
// making a Go service fast the honest way — MEASURE FIRST, then earn
// every change with a number that carries its configuration. The
// topic's instruments are this repo's own: real Benchmark functions
// under internal/domain/demos/ that any reader can run with
// `go test -bench`, a bench runner that executes them ONCE at process
// boot under a hard process-group-killing timeout, and an escape-
// analysis build whose compiler verdicts the demo serves VERBATIM.
// No number in this topic's payload is invented: the compression
// curve (gzip levels 1/6/9 across 4KB/64KB/512KB documents), the
// buffered-marshal versus streaming-encode comparison, and the
// in-memory repository's defensive-copy tax are all parsed from the
// go tool's own output, with the command, -benchtime, -count, and
// wall-clock age beside them. benchstat — the A/B discipline's judge
// — is taught as terminal prose and deliberately NOT installed (the
// family stance adds no dependency to teach a tool the reader meets
// in their own tree). Allocation teaching quotes the presentation
// package's real wire-shaping (whose append escapes the compiler
// names), GC tuning teaches the GOGC/GOMEMLIMIT trade-offs with a
// mostly-don't stance, and connection pooling quotes the pgx pool
// and go-redis client this very server boots with. The topic's demo,
// perf:bench, is inline in the compression section; the terminal
// companion runs the real benchmark command, the benchstat ritual,
// and the escape-analysis build against this module.

// performanceTopic builds topic 38.
func performanceTopic() (Topic, error) {
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
		// ── 1. Measure first ─────────────────────────────────────
		section("measure-first", "Measure first: no number, no tuning",
			"Go's toolchain ships the measurement instruments with the language — the testing package's benchmark framework, -benchmem, -count, and the benchstat comparison discipline. This section is the rule the whole topic obeys: a claim without a number is an opinion, and a number without its run configuration is an anecdote.",
			text("perf-mf-the-rule",
				"NO NUMBER, NO TUNING is the discipline this entire topic is built to enforce, and it is worth stating as a rule before any instrument appears. Performance intuition is famously wrong — in every language, but especially in Go, where the runtime's escape analysis, inlining, and garbage collector quietly transform code between what you wrote and what executes — so the only trustworthy statement about ``faster`` is one shaped like ``this benchmark, run with this -benchtime and this -count, on this machine, moved from that number to this one``. Every weaker claim (``it feels snappier``, ``everyone knows level 9 is too slow``, ``obviously the streaming encoder allocates less``) is a hypothesis wearing a conclusion's clothes. The rule has a practical corollary that shapes how you work: you MEASURE TO FIND THE HOT SPOT before you change anything (the profile leads, the editor follows), you MEASURE TO PROVE THE CHANGE after (the same benchmark, the same configuration, before and after), and you RECORD THE CONFIGURATION with every number you keep, because a benchmark result detached from its -benchtime, -count, and machine is a number you cannot ever compare against."),
			text("perf-mf-the-instrument",
				"THE INSTRUMENT IS ALREADY IN THE MODULE. Go's ``testing`` package doubles as the benchmark framework: a function named ``BenchmarkXxx(b *testing.B)`` with a loop over ``b.N`` is a complete benchmark, ``b.Run`` carves sub-benchmarks whose names become the comparison keys, ``b.ReportAllocs`` turns on per-op allocation reporting, and the ``go test -bench`` flag executes the whole thing with the same tooling that runs the suite. This guide's performance demo is built on three REAL benchmark functions that live in the demos package and measure this server's own code paths — the one below is the JSON encode A/B: buffered ``json.Marshal`` (one whole-document allocation handed to the caller) against the streaming ``json.NewEncoder(w).Encode`` that the presentation package's ``writeJSON`` uses on every response it serves:"),
			code("perf-mf-benchmark-fn", "go",
				"A real benchmark from this repo: two approaches, one page, b.ReportAllocs on both arms.",
				"server/internal/domain/demos/perf_benchmarks_test.go",
				"func BenchmarkPerfJSONEncode(b *testing.B) {\n"+
					"	page := PerfPostsPage(PerfPayloadSizes[1])\n"+
					"	b.Run(\"approach=marshal\", func(b *testing.B) {\n"+
					"		b.ReportAllocs()\n"+
					"		for i := 0; i < b.N; i++ {\n"+
					"			raw, err := json.Marshal(page)\n"+
					"			if err != nil {\n"+
					"				b.Fatal(err)\n"+
					"			}\n"+
					"			if len(raw) == 0 {\n"+
					"				b.Fatal(\"empty marshal output\")\n"+
					"			}\n"+
					"		}\n"+
					"	})\n"+
					"	b.Run(\"approach=stream\", func(b *testing.B) {\n"+
					"		b.ReportAllocs()\n"+
					"		for i := 0; i < b.N; i++ {\n"+
					"			if err := json.NewEncoder(io.Discard).Encode(page); err != nil {\n"+
					"				b.Fatal(err)\n"+
					"			}\n"+
					"		}\n"+
					"	})\n"+
					"}"),
			code("perf-mf-writejson", "go",
				"What the streaming arm measures in production: the handlers' shared success writer, encoding straight to the ResponseWriter.",
				"server/internal/presentation/content.go",
				"func writeJSON(w http.ResponseWriter, status int, v any) {\n"+
					"	w.Header().Set(\"Content-Type\", \"application/json\")\n"+
					"	w.WriteHeader(status)\n"+
					"	_ = json.NewEncoder(w).Encode(v)\n"+
					"}"),
			text("perf-mf-reading-output",
				"READING THE OUTPUT is a skill the format rewards. ``go test -bench`` prints one line per benchmark run: the name with its GOMAXPROCS suffix, the iteration count the framework chose, then metric pairs — ``ns/op`` always, and ``B/op`` plus ``allocs/op`` when allocations were reported. The iteration count is the part beginners skip and veterans read first: the framework ramps ``b.N`` until the sample is long enough to time, so a benchmark that reports ten iterations is a different KIND of measurement from one reporting a million, with proportionally different noise. Elisions matter too — ``-run='^$'`` skips the unit tests so a benchmark run does not pay for them, and ``-benchmem`` is the flag that makes the allocation story visible at all. The demo at this topic's heart parses exactly this format, and its parser is table-tested against the real shapes: count duplicates kept side by side, a row without -benchmem reporting its absence honestly, a dashed subtest name that is not a GOMAXPROCS suffix."),
			text("perf-mf-variance",
				"HONEST VARIANCE TALK is what separates measurement from theater. Run the same benchmark twice on an idle machine and the two ``ns/op`` values differ — scheduler migration, cache state, thermal throttling, and background work all tax the sample — so a SINGLE run establishes nothing except a ballpark. The standard instruments are repetition and comparison: ``-count=N`` runs each benchmark N times and keeps every sample (Go's tooling does not average them for you, and neither should you before looking at the spread), and benchstat — the ``golang.org/x/perf`` command — turns two saved runs into a statistical judgment: per-metric mean, the delta between old and new with a confidence statement, and a geomean line for the whole suite. This repo deliberately does NOT install benchstat (the family stance adds no dependency to teach a tool the reader runs in their own tree); the terminal companion shows the exact ritual — save two runs, point benchstat at both — and the demo below serves its ``-count=2`` samples SIDE BY SIDE, because the distance between them IS the noise floor every claim must survive."),
			list("perf-mf-benchstat-ritual",
				false,
				"Baseline — run the benchmark on the unchanged code with -count=10 and redirect the output to a file; this file is the \"old\" the comparison judges.",
				"Candidate — apply the change and run the IDENTICAL command (-bench, -benchtime, -count, package) to a second file; any drift in configuration quietly invalidates the comparison.",
				"Compare — benchstat old.txt new.txt reports per-benchmark means, the delta with a significance verdict, and the suite geomean; read the delta AND the verdict together.",
				"Judge — a change is real when the delta clears the noise (benchstat marks insignificant rows), not when one sample moved; insignificant deltas are \"no change\", not \"slightly better\".",
				"Keep — the surviving number goes into the commit message or PR description WITH its command, so the next reader can reproduce the claim.",
			),
		),
		// ── 2. Allocations + escape analysis ─────────────────────
		section("allocations-escape", "Allocations and escape analysis: the Go tax",
			"Every heap allocation is a tax the garbage collector collects later. The compiler's escape analysis decides who pays — and it prints its verdicts if you ask. This section reads the compiler's own output over this repo's real wire-shaping code.",
			text("perf-ae-the-tax",
				"ALLOCATIONS ARE THE GO TAX, and understanding the levy changes how you read Go code. Values that live on a goroutine's STACK are nearly free: the stack is bumped and unbumped with the call, no bookkeeping, no cleanup. Values that land on the HEAP are the taxed ones — each costs a fast-but-real allocation, and collectively they set the garbage collector's WORKLOAD: GC effort is proportional not to how long your program runs but to how much it allocates, so an allocation-heavy hot path shows up as GC CPU stolen from your request handlers. The compiler's ESCAPE ANALYSIS is the auditor that decides: if a value can be proven to die within the frame that created it (never stored somewhere that outlives the frame, never returned, never captured past the call), it stays on the stack; the moment the proof fails — the value is returned, stored in an interface that escapes, captured by a closure that outlives the frame, or made larger than the stack can risk — it ``escapes to heap`` and the tax applies. The whole game of Go allocation tuning is therefore not ``avoid new()`` but ``give the compiler proofs it can use``."),
			text("perf-ae-ask-the-compiler",
				"ASK THE COMPILER DIRECTLY — it answers in its own words, and those words are the section's vocabulary. ``go build -gcflags=-m ./internal/presentation/`` recompiles that package with the compiler's optimization diagnostics ON, printing one verdict line per decision: ``escapes to heap`` (the tax applies), ``does not escape`` (stack, free), ``leaking param`` (the parameter's lifetime is tied to something the caller cannot see — often a RESULT, meaning the value escapes only when the caller lets it), ``moved to heap`` (a local the proof failed for), and ``inlining call to`` (a call flattened away entirely — inlining is performance teaching too, because an inlined call's arguments frequently STOP escaping). The sites print import-path-relative (``internal/presentation/posts.go:55:20``), the columns name the exact expression, and the messages are stable enough to grep. The demo at this topic's end serves a curated slice of this very build's output VERBATIM — the payload never paraphrases a compiler."),
			code("perf-ae-tags-copy", "go",
				"The repo's own hot-path escape: shaping a post for the wire materializes the tags slice — append's result escapes because it is returned.",
				"server/internal/presentation/posts.go",
				"func newPostResponse(post posts.Post) postResponse {\n"+
					"	return postResponse{\n"+
					"		ID:        post.ID,\n"+
					"		Title:     post.Title,\n"+
					"		Body:      post.Body,\n"+
					"		Tags:      append([]string(nil), post.Tags...),\n"+
					"		CreatedAt: post.CreatedAt,\n"+
					"		UpdatedAt: post.UpdatedAt,\n"+
					"	}\n"+
					"}"),
			text("perf-ae-reading-the-verdict",
				"READ THAT FUNCTION AGAINST THE COMPILER'S VERDICT and the escape-analysis lesson lands. The struct itself does NOT escape — it is small, and the caller (``writeJSON``'s encoder loop) can keep it local — but ``append([]string(nil), post.Tags...)`` at line 55 is named by the build exactly: ``append escapes to heap``, because the fresh slice is STORED INTO the returned struct, and a returned value might outlive the frame, so the proof fails and the slice is heap-allocated once per post per response. The neighboring verdicts tell the rest of the story: ``leaking param content: post`` on the function's own signature (the parameter's CONTENTS flow into the result — accurate, and benign here), and ``make([]postResponse, 0, len(page.Items)) escapes to heap`` in the page shaper above it, because the items slice rides the response out. These are not defects: they are the COST of the guide's defensive-copy discipline (callers can never mutate shared state through a wire copy), and the point of reading verdicts is to know what your disciplines COST, not to reflexively delete them."),
			code("perf-ae-defensive-copy", "go",
				"The same tax on the storage side: the in-memory repository copies every post it returns — the discipline the benchmark measures.",
				"server/internal/data/posts/inmemory.go",
				"func copyPost(post posts.Post) posts.Post {\n"+
					"	post.Tags = append([]string(nil), post.Tags...)\n"+
					"	return post\n"+
					"}"),
			text("perf-ae-common-triggers",
				"THE COMMON TRIGGERS are worth naming as a checklist, because they account for nearly every avoidable heap escape in review. RETURNING a pointer or slice to a local (the proof cannot see the caller). STORING into an interface — the error and envelope types this guide loves are interface-shaped, and an interface's value escapes unless the compiler can devirtualize it. CAPTURING a local by a closure that outlives the frame — every middleware ``func(next http.Handler) http.Handler`` closure holds its captured state on the heap by design. SENDING a pointer on a channel (the receiver is another goroutine — no proof possible). And SLICES that GROW: an ``append`` that can exceed capacity reallocates, so the pre-sized ``make([]T, 0, n)`` with a known ``n`` is both an allocation-count win and an escape-analysis hint. The repo's own triggers are the friendly kind: the tags copy above exists BECAUSE sharing would be worse, and the demo's hot-path benchmark exists to put a number on exactly that trade."),
			text("perf-ae-allocsperrun",
				"``testing.AllocsPerRun`` IS THE ASSERTION INSTRUMENT — the piece that turns allocation knowledge into a TEST that fails when a refactor silently doubles the tax. It runs the function a set number of times, counts the heap allocations per call with the GC quieted around the measurement, and returns the average as a float you compare against a bound. The guide's own suite uses it on the demo's compression helper — pinning that a gzip call allocates a bounded MACHINE CONSTANT (the flate machinery's fixed buffers), not something proportional to the payload — and the pattern transfers directly to handler-shaped code: an ``AllocsPerRun`` bound on a request's wire-shaping path fails the suite the day someone adds a per-field allocation, which is the review comment you never have to write again:"),
			code("perf-ae-allocs-test", "go",
				"The suite's own allocation pin: a bounded machine constant, asserted with the instrument the topic teaches.",
				"server/internal/domain/demos/perf_bench_test.go",
				"	allocs := testing.AllocsPerRun(100, func() {\n"+
					"		if _, err := PerfGzipBytes(payload, 6); err != nil {\n"+
					"			t.Fatal(err)\n"+
					"		}\n"+
					"	})\n"+
					"	// …\n"+
					"	if allocs > 32 {\n"+
					"		t.Errorf(\"allocs per gzip call = %.0f, want a bounded machine constant (< 32)\", allocs)\n"+
					"	}"),
			text("perf-ae-when-to-care",
				"WHEN TO CARE is the judgment that keeps this section from becoming cargo cult. Allocations on a COLD path — boot code, config parsing, the once-per-process paths — are free for all practical purposes; chase them and you are optimizing a cost you pay once. Allocations on a HOT path — per-request wire shaping, per-message marshaling, the inside of the loop your profiles keep lighting up — are the real subject, and even there the order of operations is fixed: PROFILE FIRST (the debugging topic's pprof endpoints and the allocs profile specifically), find the line the profile names, THEN read its escape verdicts, THEN decide whether the fix is a proof the compiler can use (pre-sizing, value receivers, avoiding an interface hop) or a discipline whose cost you now know and accept. The benchmark below is the model: it does not argue that the in-memory repository's defensive copies are bad — it MEASURES them, with ``-benchmem``, so the discipline's price is a number on the wire instead of a feeling in a review."),
		),
		// ── 3. GC tuning ─────────────────────────────────────────
		section("gc-tuning", "GC tuning: GOGC, GOMEMLIMIT, and mostly don't",
			"The runtime exposes two knobs — GOGC (how hard the heap may grow before collection) and GOMEMLIMIT (a soft memory ceiling). They are real and worth understanding; the honest default is to touch neither until the numbers demand it.",
			text("perf-gc-two-knobs",
				"TWO KNOBS GOVERN THE COLLECTOR, and both predate your code and outlive your tuning sessions. ``GOGC`` (environment variable, or ``debug.SetGCPercent``) sets the heap growth target: the default of 100 means the GC triggers when the heap has grown by its size after the last collection — a live heap of that size doubles before the next cycle. Raise it and collections become less frequent (less GC CPU stolen per second) while peak heap grows; lower it and memory stays tighter at the price of more cycles. ``GOMEMLIMIT`` (or ``debug.SetMemoryLimit``) is the younger and usually better knob: a SOFT ceiling on total runtime memory — heap, stacks, runtime structures — that the collector treats as a goal to stay under, running harder as it approaches rather than crashing or swapping. Crucially it is soft: exceeding it does not kill the process, it makes the GC work harder, and only a genuinely live-set-above-the-limit situation becomes a thrash the runtime reports loudly. The two compose: a common production posture is GOMEMLIMIT set near the container's memory reservation with GOGC left at its default, so the heap behaves normally until the ceiling approaches."),
			text("perf-gc-mostly-dont",
				"THE MOSTLY-DON'T STANCE deserves its reasoning, not just its assertion. The garbage collector's cost scales with ALLOCATION RATE and pointer count, so the highest-leverage lever is almost always REDUCING ALLOCATIONS (the previous section's whole subject) — a codebase that halves its per-request heap traffic halves its GC work at every GOGC setting simultaneously, while knob-turning only trades the same work against memory. The classic knob-turn failure modes are symmetric: ``GOGC=off`` in a long-lived server is a leak by configuration (fine for a short batch job that exits, catastrophic for a daemon), and ``GOMEMLIMIT`` set above what the container actually grants invites the OOM killer — the limit only defends memory the runtime can see. And the diagnostic order is fixed before any knob turns: read ``/metrics`` (the observability topic's registry exports GC counters) and the runtime's own profiles (the debugging topic's pprof endpoints expose ``GODEBUG=gctrace=1`` output shapes and the alloc profile) — a GC problem you have not PROFILED is a guess wearing a knob."),
			list("perf-gc-when-to-turn",
				false,
				"Batch jobs — GOGC=off for a process with a bounded lifetime and a bounded input: zero GC cycles until exit, all throughput deferred to the end; the background-jobs topic's one-shot workers are the shape.",
				"Memory-capped containers — GOMEMLIMIT slightly under the reservation (leave headroom for stacks and the runtime): the collector leans in as the ceiling approaches instead of letting the OOM killer decide.",
				"GC CPU theft visible in profiles — the CPU profile's GC entries or gctrace cadence name the collector as a top consumer: fix the allocation rate first, then re-measure before touching GOGC.",
				"Steady-state services — leave both defaults: GOGC at its default with no ceiling is the behavior the vast majority of services, including this guide's, never need to change.",
			),
		),
		// ── 4. Connection pooling ────────────────────────────────
		section("connection-pooling", "Connection pooling: the wiring this server boots",
			"A pool is amortization made structural: pay the handshake once, bound the concurrency, and let every request borrow. The pgx pool and the go-redis client this guide boots with, quoted — because the pool you run is the pool you should learn.",
			text("perf-cp-why-pools",
				"WHY POOLS EXIST is a handshake-cost argument with a concurrency-safety cap on top. Every PostgreSQL connection pays TCP setup, TLS when configured, and the startup authentication exchange before the first query — milliseconds of latency that a per-request ``connect/query/close`` pattern re-pays on EVERY request, which is both slow and a fine way to exhaust the server's connection slots under load. A POOL pays that cost a bounded number of times and then LENDS the warmed connections out: acquire, use, release, with waiting callers queued rather than thundering. The concurrency bound is the quieter half of the value: a pool of a fixed size IS backpressure made physical — when every connection is busy, the next caller waits in line (visible in metrics!) instead of piling a thousand concurrent queries onto a database that serves fifty well. The same argument repeats for Redis clients, HTTP clients with their ``Transport`` connection pools, and gRPC channels: the guide's composition root builds each ONCE and injects it everywhere, and that once-ness is the performance teaching as much as the pooling itself."),
			code("perf-cp-pgx-pool", "go",
				"The process's ONE pgx pool, built at boot with a fail-fast ping — every query, migration, and readiness check borrows from it.",
				"server/internal/data/db/db.go",
				"// NewPool dials PostgreSQL with the configured DSN and proves the\n"+
					"// connection with a ping before returning the pool. The pool is the\n"+
					"// process-wide connection owner: acquire/release discipline, health\n"+
					"// checking, and max-connection bounds live here once, not in every caller.\n"+
					"// A DSN that parses but cannot be reached fails here — fail-fast at boot\n"+
					"// beats first-request surprises.\n"+
					"func NewPool(ctx context.Context, dsn string) (*pgxpool.Pool, error) {\n"+
					"	pool, err := pgxpool.New(ctx, dsn)\n"+
					"	if err != nil {\n"+
					"		return nil, fmt.Errorf(\"parse database url: %w\", err)\n"+
					"	}\n"+
					"	pingCtx, cancel := context.WithTimeout(ctx, connectTimeout)\n"+
					"	defer cancel()\n"+
					"	if err := pool.Ping(pingCtx); err != nil {\n"+
					"		pool.Close()\n"+
					"		return nil, fmt.Errorf(\"ping database: %w\", err)\n"+
					"	}\n"+
					"	return pool, nil\n"+
					"}"),
			text("perf-cp-pgx-knobs",
				"THE PGX POOL'S KNOBS are DSN parameters the constructor above accepts implicitly, and knowing their names is knowing what you can tune without code. ``pool_max_conns`` bounds the pool (pgx's default is the greater of four or the CPU count — a per-process default that assumes one pool per database per process); ``pool_min_conns`` keeps warm connections on hand so a burst does not pay handshakes; ``pool_max_conn_lifetime`` retires connections before server-side timeouts and failovers can strand them; ``pool_max_conn_idle_time`` reaps the unused; and ``pool_health_check_period`` sets how often idle connections are proven alive. The guide runs the DEFAULTS deliberately: a single-process server against a single PostgreSQL rarely saturates the default pool, and a pool you have not measured saturating is a knob you have not earned. The ``Ping`` in the quoted constructor is the OTHER half of the story — fail-fast at boot (inside a bounded ten-second timeout) instead of a first-request surprise, which is the reliability topic's stance arriving through a performance-shaped constructor."),
			text("perf-cp-redis",
				"THE REDIS CLIENT FOLLOWS the same once-at-the-root pattern with its own pooling story: go-redis maintains a connection pool INTERNALLY (its own acquire/release discipline over commands), so the performance rule reduces to CONSTRUCTION DISCIPLINE — build one ``Client`` per process and share it, never one per request (each client would field its own pool, its own goroutines, and its own dial storm). The guide's factory quotes below for its second teaching point: the TIMEOUT POLICY lives at construction (dial, read, write each bounded at two seconds), so an unreachable Redis fails FAST inside every caller instead of hanging a request on the TCP stack's defaults — latency bounds chosen once, in one place, applying everywhere:"),
			code("perf-cp-redis-client", "go",
				"The process's ONE go-redis client: lazy construction, bounded timeouts — the factory every Redis surface shares.",
				"server/internal/data/redis/playground.go",
				"func NewClient(addr string) *goredis.Client {\n"+
					"	return goredis.NewClient(&goredis.Options{\n"+
					"		Addr:         addr,\n"+
					"		DialTimeout:  dialTimeout,\n"+
					"		ReadTimeout:  readTimeout,\n"+
					"		WriteTimeout: writeTimeout,\n"+
					"	})\n"+
					"}"),
			text("perf-cp-coda",
				"THE ANTI-PATTERN SECTION writes itself once the pattern is visible: a client constructed per request (pool churn, dial storms, goroutine leaks), a pool shared by string globally instead of injected at the root (test impossibility), or knobs tuned before saturation is OBSERVED in metrics (the observability topic's registry carries the counters that would tell you). This server's composition root holds exactly one pgx pool and one go-redis client, injected into every adapter that needs them — and the same once-at-the-root rule covers the HTTP ``Transport`` and the gRPC channel the platform topics wired. When a future load test shows pool waits, the fix is a DSN parameter away, measured before and after with this topic's instruments; until then the pools run their defaults, which is the performance stance this guide teaches: STRUCTURE FIRST, MEASUREMENT SECOND, KNOBS LAST."),
		),
		// ── 5. The compression benchmark [demo perf:bench] ────────
		section("compression-bench", "The compression trade-off, measured [demo perf:bench]",
			"Response compression is the classic measured trade-off: CPU now versus bytes on the wire. This section's benchmark sweeps gzip levels 1, 6, and 9 across 4KB, 64KB, and 512KB documents — and the demo runs it for real, serving the parsed numbers with their run configuration.",
			text("perf-cb-the-tradeoff",
				"COMPRESSION IS THE CLEANEST measured trade-off in a backend, because both currencies are priced in units everyone agrees on. On one side: CPU time in the request path (the gzip deflater's cycles come out of the same core budget as your handlers). On the other: bytes on the wire (paid in latency for slow clients, in egress bills, in mobile users' patience). The stdlib's ``compress/gzip`` exposes the whole dial — level 1 is the fast, shallow end; level 6 is the default the library ships; level 9 is the slow, deepest setting — and the honest engineering answer to ``what level should we use`` is never a number but a CURVE: for each payload size, what does each level cost in ns/op and allocations, and what does it save in bytes? That curve is what this topic's benchmark measures and what the demo serves — three levels against three deterministic documents (a small API reply, a fat page, a whole-document dump), with the compressed SIZES computed from the very bytes the benchmark compressed so the ratio column and the ns/op column are always about the same document."),
			code("perf-cb-gzip-helper", "go",
				"The single compression path everything shares: the benchmarks, the size column, and a reader following along.",
				"server/internal/domain/demos/perf_bench.go",
				"func PerfGzipBytes(payload []byte, level int) ([]byte, error) {\n"+
					"	var buffer bytes.Buffer\n"+
					"	writer, err := gzip.NewWriterLevel(&buffer, level)\n"+
					"	if err != nil {\n"+
					"		return nil, fmt.Errorf(\"perf: gzip level %d: %w\", level, err)\n"+
					"	}\n"+
					"	if _, err := writer.Write(payload); err != nil {\n"+
					"		return nil, fmt.Errorf(\"perf: gzip write: %w\", err)\n"+
					"	}\n"+
					"	if err := writer.Close(); err != nil {\n"+
					"		return nil, fmt.Errorf(\"perf: gzip close: %w\", err)\n"+
					"	}\n"+
					"	return buffer.Bytes(), nil\n"+
					"}"),
			code("perf-cb-benchmark", "go",
				"The curve as a real benchmark: levels × sizes as b.Run subtests, allocations reported on every cell.",
				"server/internal/domain/demos/perf_benchmarks_test.go",
				"func BenchmarkPerfGzipCompress(b *testing.B) {\n"+
					"	for _, size := range PerfPayloadSizes {\n"+
					"		payload, err := PerfPayloadJSON(size)\n"+
					"		if err != nil {\n"+
					"			b.Fatal(err)\n"+
					"		}\n"+
					"		for _, level := range PerfGzipLevels {\n"+
					"			b.Run(fmt.Sprintf(\"level=%d/size=%s\", level, size.Label), func(b *testing.B) {\n"+
					"				b.ReportAllocs()\n"+
					"				for i := 0; i < b.N; i++ {\n"+
					"					compressed, err := PerfGzipBytes(payload, level)\n"+
					"					if err != nil {\n"+
					"						b.Fatal(err)\n"+
					"					}\n"+
					"					if len(compressed) == 0 {\n"+
					"						b.Fatal(\"empty gzip output\")\n"+
					"					}\n"+
					"				}\n"+
					"			})\n"+
					"		}\n"+
					"	}\n"+
					"}"),
			text("perf-cb-what-the-curve-says",
				"WHAT THE CURVE SAYS — as directions the suite PINS, not numbers this page invents — is the shape of every compression decision you will ever make. The SIZE axis dominates the savings story: the same level shrinks a repetitive 512KB document proportionally more of its bulk than a 4KB one, because the deflater's window has more to work with — so compression pays RENT on large, repetitive payloads (JSON lists, CSV dumps, API pages) and barely covers its booking fee on tiny ones (a level-1 pass over a small document can spend more bytes of CPU than it saves of wire, which is why real servers skip compressing small responses). The LEVEL axis is the CPU knob: on this repo's deterministic documents the suite asserts the direction that makes the trade real — level 1's output is strictly LARGER than level 9's on the same document — and the benchmark's ``ns/op`` samples show what that extra shrinkage costs in time across the matrix. The exact numbers for every cell, with both ``-count`` samples side by side, live in the demo below — parsed from a real run, never typed into this page."),
			demo("perf:bench", "The performance bench: real benchmarks, run at boot, served with their config",
				"GET /api/demo/perf/bench executes this repo's own curated benchmarks ONCE at process boot in the background (single-flight, bounded by a hard timeout that kills the whole process group) and serves the parsed results: the gzip curve across levels 1/6/9 and 4KB/64KB/512KB documents with both count samples per cell and the compressed sizes computed from the same deterministic bytes, the JSON buffered-marshal versus streaming-encode A/B, and the in-memory posts repository's List — the fake mode's REAL serving path — with its per-call allocation tax from -benchmem. Beside the numbers rides the escape-analysis panel: a real `go build -gcflags=-m` over the presentation package, its verdicts served verbatim. Every figure carries the exact command, -benchtime, -count, wall-clock duration, and the age of the run; the two samples per benchmark sit side by side because the distance between them is the noise floor. When the toolchain or the source tree is absent, the endpoint degrades to the honest typed 503 rather than rounding anything into lies."),
			code("perf-cb-boot-run", "go",
				"Single-flight at boot: the run executes once per process, and the first request triggers it if boot never did.",
				"server/internal/presentation/demo_perf.go",
				"func (h *PerfBenchHandler) Start() {\n"+
					"	h.mu.Lock()\n"+
					"	defer h.mu.Unlock()\n"+
					"	if h.started {\n"+
					"		return\n"+
					"	}\n"+
					"	h.started = true\n"+
					"	go func() {\n"+
					"		outcome := h.runBench(context.Background())\n"+
					"		h.mu.Lock()\n"+
					"		defer h.mu.Unlock()\n"+
					"		h.outcome = &outcome\n"+
					"	}()\n"+
					"}"),
			code("perf-cb-bounded-run", "go",
				"The bound with teeth: the run gets its own process group, and timeout expiry kills the group — the go tool and every child it spawned.",
				"server/internal/presentation/demo_perf.go",
				"	cmd := exec.CommandContext(ctx, name, args...)\n"+
					"	cmd.Dir = dir\n"+
					"	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}\n"+
					"	cmd.Cancel = func() error {\n"+
					"		if cmd.Process != nil {\n"+
					"			// Negative pid addresses the process GROUP: the go tool and\n"+
					"			// every child it spawned die together.\n"+
					"			_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)\n"+
					"		}\n"+
					"		return cmd.Process.Kill()\n"+
					"	}"),
			text("perf-cb-provenance",
				"EVERY NUMBER CARRIES ITS PROVENANCE, which is the difference between this demo and a chart someone drew. The boot run's configuration is a fact on the wire: the payload names the exact command (``go test -bench=^BenchmarkPerf -benchtime=10x -count=2 -benchmem -run='^$'`` over the demos package), the environment headers the tool itself printed (goos, goarch, cpu), the wall-clock start, finish, and duration, and the AGE of the numbers — reload and the age moves while no number does, because the run happened once at boot and its samples are cached for the process lifetime. The small ``-benchtime=10x`` (exactly ten iterations per sample) and ``-count=2`` are honest about being a BOOT budget, not a publication-grade measurement: they exist so the page loads with real parsed numbers, and the terminal companion hands you the reader's invocation with real budgets (one-second benchtime, ten counts) to settle the same questions properly. The suite pins the whole chain — the parser against the tool's real output shapes, the builder against fixtures, and one end-to-end test that executes the actual runner against the actual module and refuses anything but parsed rows."),
		),
		// ── 6. The checklist ─────────────────────────────────────
		section("pre-production-checklist", "The pre-production performance pass",
			"The five-step pass this guide runs before calling a service performant — budgets written down, profiles read, metrics watched, benchmarks compared, and traffic rehearsed. Each step names the topic that built its instrument.",
			text("perf-cl-what",
				"THE PASS IS SHORT because every instrument in it already exists in the topics before this one — performance is where the platform work pays out, not a new subsystem to build. You write the BUDGET down first (an unwritten budget is a wish: a p95 target per endpoint class, an allocation ceiling per request, a resident-memory ceiling the deployment must fit inside). You PROFILE before you optimize, using the debugging topic's pprof endpoints mounted on ``/internal/`` — the CPU profile names the hot function, the allocs profile names the allocation tax, and the escape-analysis build from this topic explains the lines both name. You WATCH the running verdict on the observability topic's ``/metrics`` — the request-duration histogram and GC counters are the numbers that move (or do not) when your change ships. You BENCH every optimization through the A/B ritual this topic taught. And you REHEARSE real traffic against the composed stack before production sees it. The list block states each step as the pass runs it:"),
			list("perf-cl-the-pass",
				false,
				"Budgets — write them down before tuning: p95 latency per endpoint class, allocations per request, and a resident-memory ceiling; a budget you cannot recite is a wish.",
				"Profile — capture pprof profiles from the real binary under load (the debugging topic's /internal/ endpoints), and read the top of the profile before changing a line.",
				"Watch — /metrics and the request-duration histogram (the observability topic's registry) are the running verdict; a change that does not move them did not happen.",
				"Bench — A/B every optimization with go test -bench -count=10 and benchstat; a single run is an anecdote, and the delta must survive the noise floor.",
				"Load — rehearse production traffic against the composed stack before production: saturation behavior, pool waits, and GOMEMLIMIT's degradation observed rather than assumed.",
			),
			text("perf-cl-coda",
				"PERFORMANCE CLOSES THE PLATFORM TRIAD the way the curriculum ordered it: observable (topic 33) so the numbers exist, debuggable (topic 35) so the causes are reachable, and now MEASURABLE (this topic) so the fixes are provable. The stance worth keeping is the one the topic opened with, now fully instrumented: no number, no tuning — and the numbers are cheap to get, because the benchmark framework ships with the toolchain, the profiles ship with the runtime, the metrics ship with the registry this guide built, and the A/B judge is one ``go install`` away in the reader's own tree. What remains is the discipline to run them in order — budget, profile, watch, bench, load — and the honesty to report what they say even when the answer is ``the defaults were fine``, which is, measured-ly, the most common correct answer in Go."),
		),
		// ── 7. Terminal companion ────────────────────────────────
		section("terminal-companion", "Terminal companion: the measurement tour",
			"The topic over a terminal: run the repo's real benchmarks with real budgets, judge an A/B with benchstat, read the compiler's escape verdicts, and curl the demo's boot-cached run — every number next to the command that produced it.",
			text("term-perf-intro",
				"With the stack up (server on :8100, run from the repo root), the tour runs in the topic's order. First THE BENCHMARK ITSELF — the reader's own invocation with publication-grade budgets (one-second benchtime, ten counts, allocations on), which is the run the demo's boot budget only approximates; read the iteration counts before the ns/op, and the B/op and allocs/op columns as the allocation section taught. Then THE BENCHSTAT RITUAL — two saved runs, one per branch, judged statistically; benchstat is prose-only here by the family stance (no dependency installed for teaching), so the block shows the exact commands to run where it IS installed. Third, THE COMPILER'S VERDICTS — the escape-analysis build over the presentation package, grepped to the verdict grammar, which is exactly the output the demo curates. And finally THE DEMO — the boot-cached run served with its command, configuration, and age:"),
			code("term-perf-bench-bash", "bash", "", "",
				"# the curated set with real budgets — the reader's own run\n"+
					"cd server && go test -bench='^BenchmarkPerf' -benchtime=1s -count=10 \\\n"+
					"  -benchmem -run='^$' ./internal/domain/demos/\n"+
					"\n"+
					"# one cell of the curve, isolated by subtest name\n"+
					"go test -bench='^BenchmarkPerfGzipCompress/level=1/size=4KB$' \\\n"+
					"  -benchmem -run='^$' ./internal/domain/demos/\n"+
					"\n"+
					"# the boot-cached run the demo serves: state, command, config, age,\n"+
					"# then the curve — level, size, ratio, and both count samples\n"+
					"curl -s localhost:8100/api/demo/perf/bench | python3 -c \\\n"+
					"  'import json,sys; p=json.load(sys.stdin); r=p[\"run\"]; \\\n"+
					"   print(r[\"state\"], \"-\", r[\"command\"]); \\\n"+
					"   print(\"benchtime\", r[\"benchtime\"], \"count\", r[\"count\"], \\\n"+
					"         \"age\", round(r[\"ageSeconds\"]), \"s\"); \\\n"+
					"   [print(c[\"level\"], c[\"size\"].rjust(5), c[\"ratio\"].rjust(6), \\\n"+
					"         [round(s[\"nsPerOp\"]) for s in c[\"samples\"]]) \\\n"+
					"    for c in p[\"compression\"]]'"),
			code("term-perf-benchstat-bash", "bash", "", "",
				"# benchstat is the A/B judge — NOT installed in this repo (the\n"+
					"# family stance adds no dependency to teach it); run it in yours:\n"+
					"go install golang.org/x/perf/cmd/benchstat@latest\n"+
					"\n"+
					"# the ritual: baseline saved on main, candidate on your branch,\n"+
					"# IDENTICAL commands, ten samples each\n"+
					"git stash && go test -bench='^BenchmarkPerfJSONEncode' -benchtime=1s \\\n"+
					"  -count=10 -run='^$' ./internal/domain/demos/ > /tmp/old.txt && git stash pop\n"+
					"go test -bench='^BenchmarkPerfJSONEncode' -benchtime=1s \\\n"+
					"  -count=10 -run='^$' ./internal/domain/demos/ > /tmp/new.txt\n"+
					"benchstat /tmp/old.txt /tmp/new.txt"),
			code("term-perf-escape-bash", "bash", "", "",
				"# the compiler's allocation verdicts, verbatim — the same build\n"+
					"# the demo's escape panel curates\n"+
					"go build -gcflags=-m ./internal/presentation/ 2>&1 | \\\n"+
					"  grep -E 'escapes to heap|leaking param|moved to heap' | head\n"+
					"\n"+
					"# the curated verdicts off the wire, with the panel's command\n"+
					"curl -s localhost:8100/api/demo/perf/bench | python3 -c \\\n"+
					"  'import json,sys; e=json.load(sys.stdin)[\"escape\"]; \\\n"+
					"   print(e[\"state\"], \"-\", e[\"command\"]); \\\n"+
					"   [print(v[\"site\"].ljust(44), v[\"verdict\"]) for v in e[\"verdicts\"]]'"),
			text("term-perf-coda",
				"Read the tour as the topic's argument in terminal form. The first block is MEASUREMENT, not demonstration: those are the real benchmark functions this module ships, run with the budgets a decision deserves, and every column of their output is one this topic taught you to read. The benchstat block is the A/B DISCIPLINE — the identical commands, the saved baselines, the statistical judgment that separates a real delta from noise — with the tool left in the reader's tree where the family stance puts it. The escape block is the COMPILER UNDER OATH: the same verdict lines the demo serves, grepped from a build you ran yourself. And the curl blocks are PROVENANCE ON THE WIRE: numbers that name the command that produced them, the configuration it ran with, and the age of the run — the demo's whole honesty contract, checked out over HTTP. Performance sits in the ship group between deployment and the reliability-and-scaling pair; everything from here applies these instruments to failure and to load, and the capstones close the curriculum."),
		),
	}

	if buildErr != nil {
		return Topic{}, buildErr
	}
	return NewTopic(
		"performance",
		"Performance in Go: measure, then earn the tuning",
		"Making a Go service fast the honest way — no number, no tuning — taught from this repo's real instruments: benchmarks as the judge, allocations as the Go tax, profiles as the map.",
		"ship",
		38,
		sections,
	)
}
