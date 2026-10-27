package content

import (
	"errors"
	"fmt"
)

// This file is topic 35 debugging (group platform, curriculum order 35):
// how to find out why a Go system is misbehaving, taught from this
// repo's own machinery. The mindset section sets the method (read the
// error, hypothesize, bisect); print discipline states honestly when
// fmt.Println is right and when the observation must become slog
// structure (the request logger quoted in full); delve is taught in
// prose and terminal as the real interactive debugger — and stated
// plainly as NOT wired as a demo, because a breakpoint prompt cannot
// ride a GET. The pprof section quotes topic 33's actual /internal/
// mount, teaches CPU vs heap vs goroutine profiles, and carries the
// topic's one demo (debug:stall): a real wedge whose anatomy is
// captured from the process's own goroutine profile by the demo's
// runtime machinery, parsed by a table-tested parser, and served with
// a healthy contrast — released before the request returns. The
// execution tracer covers what aggregates cannot show; production
// debugging puts topic 33's observability loop FIRST and pprof behind
// the internal fence; the process nets close with the real timeout,
// recovery, and shutdown wiring this server runs.

// debuggingTopic builds topic 35.
func debuggingTopic() (Topic, error) {
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
		// ── 1. The mindset ────────────────────────────────────────
		section("mindset", "The debugging mindset: read the error, hypothesize, bisect",
			"Debugging is not intuition applied faster — it is the scientific method applied to a running system: read what the system already told you, form one falsible hypothesis, then bisect the system until the hypothesis is the only thing left standing.",
			text("dbg-mindset-read",
				"READ THE ERROR BEFORE REACHING FOR A TOOL — that sentence orders this whole topic, because a surprising share of debugging sessions end at line one. Go's error discipline pays off exactly here: errors are VALUES that carry context, wrapped with ``%w`` so the chain survives, and the chain is a compressed narrative of what was attempted and which layer refused. When this guide's seed step fails, the log line does not say ``seed failed`` — it says ``seed: insert guide posts: pq: duplicate key value violates unique constraint`` — three layers of story in one line: WHO was working (seed), WHAT it did (insert guide posts), WHY the engine refused (the constraint, by name). The error-handling topic built that discipline (typed taxonomy, ``errors.Is``/``errors.As`` for decisions, wrapping for narrative); debugging is where the narrative gets read. The anti-pattern is symmetric and common: swallowing the error entirely, or logging it context-free (``log.Error(err)`` with no wrapping) so the chain is destroyed exactly when it was most needed. Read slowly, unwrap deliberately, and only then pick a tool."),
			text("dbg-mindset-hypothesis",
				"FORM A HYPOTHESIS before touching anything — debugging without one is not debugging but random-walk editing, and random-walk editing SHIPS the bugs it displaces. A hypothesis is a falsifiable prediction: ``the caching decorator serves the stale page after invalidation because the cache key omits the sort order`` names a mechanism, not just a symptom — and everything in the rest of this topic is an experiment that confirms or kills exactly one such prediction. A print checks a VALUE; a breakpoint checks a MOMENT; a profile checks a DISTRIBUTION; a trace checks a TIMELINE. Each probe gets a prediction written down BEFORE it runs (``if the key is the problem, the miss here will show the unsorted key``), because the point of predicting is that an unexpected result is information: a probe that surprises you has just eliminated a hypothesis and usually suggested the next one. One hypothesis at a time, one probe per hypothesis — the discipline feels slow and is the fastest thing in this topic."),
			text("dbg-mindset-bisect",
				"BISECT THE SYSTEM: halve the space where the bug can hide, relentlessly, until only its neighborhood is left. Two axes do almost all the work. TIME: the repo's history is a binary tree of behavior — a failing test plus ``git bisect`` lands on the exact commit that introduced the change, in log₂(steps) probes, which is why the testing topic's command IS the contract (a suite that runs anywhere makes bisect possible anywhere). SPACE: the architecture topic's layer map (client → middleware → handler → use case → adapter → store) is a bisection instrument — probe the MIDDLE seam first, and the answer tells you which half to keep; this repo even built the probe for you (the arch tracer demo records the first layer where reality diverges from expectation). Concurrency gets the same move from this topic's demo: a system of goroutines is not debuggable as a whole, but a goroutine profile cuts it to the ONE stuck worker — bisection applied to the scheduler's own records."),
			list("dbg-mindset-loop",
				false,
				"Reproduce — a bug you cannot trigger on demand cannot be verified fixed; the smallest command that triggers it is the ticket's real content, and everything downstream depends on it.",
				"Reduce — strip every input, layer, and interleaving that does not matter; the smallest repro does not just make iterating faster, it narrows where the bug can physically hide.",
				"Hypothesize — one falsible prediction at a time, written down before the probe runs, so an unexpected result is information instead of noise.",
				"Bisect — halve the remaining space, in time (git bisect over commits) or in layers (probe the middle seam first), until only the bug's neighborhood is left.",
				"Verify — the fix lands with the test that failed before it and passes after; a fix you cannot prove is a guess you have renamed.",
			),
		),
		// ── 2. Print discipline ───────────────────────────────────
		section("print-discipline", "Print debugging: honest limits, and the print that stays",
			"The print statement is the oldest debugger and still the fastest answer for a small repro — its failure mode is scale, and the cure is knowing exactly when a print has stopped being temporary.",
			text("dbg-print-dies",
				"WHY ``fmt.Println`` DIES AT SCALE: five failures converge, and each one is harmless alone. NO CORRELATION — a busy handler runs on one goroutine among hundreds, and prints from all of them interleave on stdout; the value you just read may answer a different request's question. NO LEVELS — a print cannot be turned down in production without a redeploy, so the same line is either absent when you need it or omnipresent when you do not. NO STRUCTURE — a flat line cannot be filtered by field, counted by group, or joined across services; every consumer has to parse prose. NO COST CONTROL — a print on a hot path is synchronous I/O on a hot path, and the debug observability you added is now the latency you were chasing. NO LIFECYCLE — prints accrete; the codebase that reached for ten of them in one incident still has nine next quarter. At one request a second none of this bites; at thousands concurrent, all five bite at once."),
			text("dbg-print-discipline",
				"THE DISCIPLINED TEMPORARY PRINT IS NOT A SIN, and pretending otherwise is theater, not discipline. For a local repro — the loop narrowed, the input reduced, the hypothesis pointing at one expression — the fastest instrument in existence is still ``fmt.Println(\"here\", got)``, and this repo's own tests were debugged with exactly that tool. The discipline is a FORK, not a ban: temporary means it leaves with the branch — if the observation should outlive the session, it is either an assertion in disguise (make it a test, where the suite keeps it true forever) or an observation the system should keep making (make it slog, where it joins levels, structure, and correlation). The fork's judgment call is what separates the two: a print that answered THIS hypothesis is temporary; a print that watches a boundary the system crosses forever is the seed of a log line. When the print stays, it becomes what this server's request logger already is — quoted here in full, because it is the print that stayed, done right:"),
			code("dbg-print-requestlogger", "go",
				"The print that stayed: one stable message key, leveled, with attributes — filterable, joinable, and correlated by request id.",
				"server/internal/presentation/middleware.go",
				"func RequestLogger(logger *slog.Logger) Middleware {\n"+
					"	return func(next http.Handler) http.Handler {\n"+
					"		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {\n"+
					"			rec := newStatusRecorder(w)\n"+
					"			started := time.Now()\n"+
					"			defer func() {\n"+
					"				durationMs := float64(time.Since(started).Microseconds()) / 1000\n"+
					"				logger.Info(\"http_request\",\n"+
					"					\"method\", r.Method,\n"+
					"					\"path\", r.URL.Path,\n"+
					"					\"status\", rec.status,\n"+
					"					\"duration_ms\", math.Round(durationMs*10)/10,\n"+
					"					\"request_id\", core.RequestIDFromContext(r.Context()),\n"+
					"				)\n"+
					"			}()\n"+
					"			next.ServeHTTP(rec, r)\n"+
					"		})\n"+
					"	}\n"+
					"}"),
			text("dbg-print-stayed",
				"Read that quote as the observability topic taught it and this topic uses it: the message key ``http_request`` is STABLE (grep-able forever), the level is a real level (``Info``, turned down or up by config, never by redeploy), and the payload is ATTRIBUTES, not prose — method, path, status, a rounded duration, and the request id that threads every log line of one request into one story. That last field is the debugging payoff: when something goes wrong in production, you do not read logs, you read A REQUEST — grep the id the error envelope handed back and watch that request's whole journey, handler by handler, in order. The printing habit, upgraded by exactly this shape, is what the production-debugging section later in this topic reaches for FIRST. Between the temporary print and the structured line sits the rest of this topic's toolbox — starting with the one tool that stops time itself."),
		),
		// ── 3. delve ──────────────────────────────────────────────
		section("delve", "delve: the real debugger",
			"Go's real debugger is delve — built for Go's runtime semantics, goroutines included. This section teaches it honestly in prose and terminal: a debugger session is interactive, and it is the one tool in this topic deliberately NOT wired as an HTTP demo.",
			text("dbg-dlv-what",
				"WHAT DELVE IS, and why the gap it fills exists: the go toolchain ships build, test, profile, and trace — but no debugger command, because generic debuggers barely speak Go. ``gdb`` sees threads and memory, while Go's concurrency lives in GOROUTINES scheduled onto those threads, its values are interfaces and slices and channels with runtime layouts, and its stacks GROW by moving — a generic debugger answers questions about the machine when the questions you have are about the runtime's own abstractions. delve is the debugger that closes that gap: it launches or attaches with full runtime awareness, lists goroutines and switches between them, prints Go values with their real structure, steps by SOURCE LINE, and evaluates expressions against live frames. It installs like any Go tool (the dependency topic's whole story): one ``go install``, one binary in GOBIN — and one honest note: this guide's devcontainer does NOT ship it, deliberately (topic 31's manifest discipline; a debugger is a per-machine tool, not a service dependency), so the install line below is the entire setup story:"),
			code("dbg-dlv-install", "bash", "", "",
				"# delve installs like any Go tool (topic 31's story) — one binary in GOBIN\n"+
					"go install github.com/go-delve/delve/cmd/dlv@latest\n"+
					"dlv version"),
			text("dbg-dlv-session",
				"THE SESSION IS FOUR VERBS — ``break``, ``continue``, ``print``, ``stack`` — plus one Go-specific fifth, ``goroutines``, and everything else is ornament. The shape of a session against this very repo: ``dlv debug ./cmd/guide`` compiles the server and stops before main; set a FUNCTION breakpoint (``break guide/internal/presentation.NewPostsListHandler`` — function names beat line numbers, which drift with every edit); ``continue`` runs until it fires — the request that reaches it is the first one you curl; now ``print`` the handler's parsed inputs and ``stack`` to see the middleware chain as frames, the same chain the architecture topic taught as a diagram; ``goroutines`` lists the rest of the process — every parked connection handler, the redis client, the jobs worker — and ``goroutine N`` switches your context to any of them before ``stack`` again. The variants matter for where you are: ``dlv exec ./guide`` debugs a built binary, ``dlv attach <pid>`` attaches to an already-RUNNING process (the production-shaped option, where ptrace permissions allow), and ``dlv test`` is the sleeper — the testing topic's table-driven loop under a debugger is delightful: break inside the loop body once and every table row walks through your breakpoint, ``tt`` inspectable case by case."),
			code("dbg-dlv-session-bash", "bash", "", "",
				"# the session, against this repo (server/ is the module root)\n"+
					"dlv debug ./cmd/guide\n"+
					"(dlv) break guide/internal/presentation.NewPostsListHandler\n"+
					"(dlv) continue          # then, from another terminal: curl localhost:8100/api/posts\n"+
					"(dlv) print query       # the handler's parsed, validated input\n"+
					"(dlv) stack             # the middleware chain, as frames\n"+
					"(dlv) goroutines        # every goroutine the server is running\n"+
					"(dlv) exit\n"+
					"\n"+
					"# debugging a test: the table loop, one row at a time\n"+
					"dlv test ./internal/domain/posts\n"+
					"(dlv) break guide/internal/posts.(*Draft).Validate\n"+
					"(dlv) continue"),
			text("dbg-dlv-honest",
				"THE HONEST BOUNDARY: delve is NOT wired as a demo in this guide, and the reason is structural, not lazy. This guide's demos ride ``GET`` requests — bounded, read-only, curl-able payloads a browser renders — and a debugger session is the OPPOSITE shape: an interactive two-way terminal whose whole value is that the next command depends on what the last one showed. No honest HTTP wrapper exists for a breakpoint prompt, and a recorded session would be a lecture wearing a demo's clothes. What this guide wires instead is the debugger's non-interactive cousins, in the sections that follow: the goroutine profile answers ``what is everyone doing`` without stopping the process, the CPU and heap profiles answer ``where does the cost go``, and the execution tracer answers ``when did it happen`` — the post-mortem instruments that ALSO work in production, where a debugger session is usually impossible anyway. For completeness: delve ships a headless mode (``dlv --headless --listen=:2345 --api-version=2``) that IDEs drive over its API — the VS Code Go extension is that engine with a different front-end — same debugger, same verbs, one more window."),
		),
		// ── 4. pprof profiles [demo debug:stall] ──────────────────
		section("pprof-profiles", "pprof: the records the runtime kept for you",
			"The Go runtime has been keeping records the whole time — CPU samples, heap snapshots, and every goroutine's stack — and pprof serves them. Since topic 33 this server mounts them for real under /internal/, and this topic's demo stages a genuine wedge and reads its anatomy out of the process's own profile.",
			text("dbg-prof-mounted",
				"THE MOUNT IS REAL — and the wiring is worth quoting rather than paraphrasing, because its shape teaches two non-obvious things. First, the SIDE EFFECT: importing ``net/http/pprof`` REGISTERS its handlers on ``http.DefaultServeMux`` as a package-level consequence of the import — a classic Go gotcha, because a library that imports it silently exposes profiles on any server that serves the default mux. This server never serves the default mux, so those registrations stay INERT until a mux the composition root owns mounts them deliberately — here, under the dedicated ``/internal/`` prefix. Second, the FENCE: one prefix gives ops one string to filter (``nothing under /internal/ crosses the edge``) whether the boundary is an ingress rule, a firewall, or a loopback-only bind. Inside the fence sit the runtime's own instruments: the index, a CPU profile sampled over N seconds, the execution trace, symbol and cmdline — and, through pprof.Index's own routing, every NAMED profile: ``heap``, ``goroutine``, ``block``, ``mutex``, ``allocs``, ``threadcreate``:"),
			code("dbg-prof-mount", "go",
				"Topic 33's real mount: default-mux side effect neutralized, profiles fenced behind /internal/.",
				"server/internal/presentation/pprof.go",
				"func NewInternalHandler() http.Handler {\n"+
					"	internal := http.NewServeMux()\n"+
					"	internal.HandleFunc(\"/debug/pprof/\", pprof.Index)\n"+
					"	internal.HandleFunc(\"/debug/pprof/cmdline\", pprof.Cmdline)\n"+
					"	internal.HandleFunc(\"/debug/pprof/profile\", pprof.Profile)\n"+
					"	internal.HandleFunc(\"/debug/pprof/symbol\", pprof.Symbol)\n"+
					"	internal.HandleFunc(\"/debug/pprof/trace\", pprof.Trace)\n"+
					"	return http.StripPrefix(\"/internal\", internal)\n"+
					"}"),
			text("dbg-prof-cpu-heap",
				"CPU AND HEAP ANSWER DIFFERENT QUESTIONS, and picking the wrong one wastes the whole session. The CPU profile is SAMPLED STACK TRACES of where the process burns time — the profiler interrupts execution thousands of times a second and records where it was; read it as a ranking (``go tool pprof -top``: ``flat`` is time in the function's own frames, ``cum`` includes its callees) or as a FLAME GRAPH — the same samples drawn as widths, one bar per stack, wide where the time went; the concept to internalize is that a flame graph is not a different measurement, only a different projection of the identical samples. The HEAP profile is where memory LIVES: ``inuse_space`` (bytes alive right now — the OOM and growth view) versus ``alloc_space`` (bytes allocated over the window — the allocation-PRESSURE view, because every allocated byte is future GC work). The debugging move is a decision table: SLOW under load → CPU; GROWING or OOM or GC thrash → heap; and when the symptom is neither cost but STUCKNESS — the third profile is the one: the GOROUTINE profile is a census of every goroutine and its full stack, nearly free to take, and it is the single most under-used debugging instrument Go ships. (``block`` and ``mutex`` quantify lock waits, but only after ``runtime.SetBlockProfileRate``/``SetMutexProfileFraction`` turn their collection on — honest footnote, since an endpoint can serve an empty profile politely.)"),
			text("dbg-prof-stall",
				"THE DEMO'S MACHINERY IS A REAL WEDGE, built the way this guide builds everything: implementation first, then it quotes itself. The substrate stages a bounded worker set — four workers doing tiny bounded work, and ONE worker that blocks forever on a channel receive nobody serves. The wedge function is deliberately tiny and deliberately NAMED, because its name is how the demo finds the wedged goroutine again:"),
			code("dbg-prof-wedge", "go",
				"The production wedge: a bare channel receive — and a function name that doubles as the search marker.",
				"server/internal/domain/demos/debug_stall.go",
				"// debugStallWedge is the production wedge: a bare channel receive on a\n"+
					"// channel nobody sends to. Its name is deliberately distinctive — the\n"+
					"// finder locates the wedged goroutine in the captured profile by\n"+
					"// matching \"debugStallWedge\" on its own stack, so the function's name\n"+
					"// IS the marker. Rename it and the demo's finder must move with it.\n"+
					"func debugStallWedge(release <-chan struct{}) {\n"+
					"	<-release\n"+
					"}"),
			text("dbg-prof-capture",
				"THE CAPTURE IS THE POINT: while the wedge is live, the process takes its OWN goroutine profile — the same profile the ``/internal/`` endpoint serves, but from code, via ``runtime/pprof`` — and reads the wedged goroutine's anatomy out of it. The capture is honest about one scheduling truth: a goroutine that has merely been CREATED is not yet ON its channel receive — it is runnable for an instant — so the demo captures, checks whether the wedge is visible, and retries within a bounded window until the runtime's records match the ground truth it just created. Normally the first capture settles it:"),
			code("dbg-prof-capture-loop", "go",
				"The demo's engine: the profile the endpoint serves, captured from code — debug=2 is the full-text format with every stack.",
				"server/internal/domain/demos/debug_stall.go",
				"		var raw strings.Builder\n"+
					"		captureStarted := time.Now()\n"+
					"		if profile := pprof.Lookup(\"goroutine\"); profile != nil {\n"+
					"			_ = profile.WriteTo(&raw, 2)\n"+
					"		}\n"+
					"		run.CaptureTries++\n"+
					"		run.CaptureMs = float64(time.Since(captureStarted).Microseconds()) / 1000\n"+
					"		run.CaptureRaw = raw.String()"),
			text("dbg-prof-parser",
				"THE PARSER is where ``why is this slow`` becomes a readable answer, and its subject is the runtime's own grammar, observed rather than invented: each goroutine is a header line — ``goroutine 42 [chan receive, 5 minutes]:`` — followed by frame pairs, a function line and a tab-indented ``file:line``, optionally closed by a ``created by`` pair naming the spawner. The header's bracket deserves attention because it is DIAGNOSIS IN MINIATURE: ``chan receive`` names the blocking operation, and the trailing duration says how long it has been stuck — the runtime has been timestamping your bug for you. The tricky parse detail the code below handles: states themselves carry commas (``select, sync.Mutex.Lock``), so the trailing ``5 minutes`` is split off ONLY when it reads as a duration — and the suite table-tests exactly that edge, along with multi-frame stacks, created-by pairs, noise lines, and headers that are not headers:"),
			code("dbg-prof-parser-header", "go",
				"Reading the runtime's grammar: header recognized by shape, ids parsed, everything suspicious rejected as noise.",
				"server/internal/domain/demos/debug_stall.go",
				"func goroutineHeader(line string) (id int, state, wait string, ok bool) {\n"+
					"	if !strings.HasPrefix(line, \"goroutine \") || !strings.HasSuffix(line, \"]:\") {\n"+
					"		return 0, \"\", \"\", false\n"+
					"	}\n"+
					"	body := strings.TrimSuffix(strings.TrimPrefix(line, \"goroutine \"), \"]:\")\n"+
					"	fields := strings.SplitN(body, \" \", 3)\n"+
					"	if len(fields) < 2 || !strings.HasPrefix(fields[1], \"[\") {\n"+
					"		return 0, \"\", \"\", false\n"+
					"	}\n"+
					"	// …state assembled from the bracket; a trailing \", 5 minutes\" splits\n"+
					"	// into the wait — but only when it reads as a duration, because\n"+
					"	// states themselves may carry commas"),
			demo("debug:stall", "The stall: a real wedge, captured from this process's own profile",
				"Each GET stages a fresh worker set — five goroutines, four of which complete tiny work and one of which blocks forever on a channel receive. While the wedge is live, the server captures its OWN goroutine profile (runtime/pprof.Lookup, the same profile /internal/debug/pprof/goroutine serves), parses it, and serves the anatomy: the wedged goroutine's id, its `chan receive` state and wait, its frames mapped to source lines, and the created-by chain that spawned it — beside the CONTRAST row, the request's own goroutine from the same capture, runnable and executing the capture itself. The wedge is released deterministically before the response returns (the suite proves no goroutine leaks), every internal wait is bounded (the demo can never wedge its own request), and a run that somehow lost its wedge serves a typed 500 that says so — no stack is ever invented. Reload to stage another; the pointer panel lists the live pprof endpoints for your own exploration."),
			text("dbg-prof-honest",
				"THE HANDLER'S REFUSAL is the demo's last teaching, and it is the same discipline the whole guide holds: runtimes are observed, never simulated. If a run cannot find its own wedge on the profile it just captured, the honest response is not a best-effort stack — it is the typed 500 below, naming the failure. The pair of this refusal with the demo's release proof is the complete honesty loop: the demo asserts what it can PROVE (the wedge was found on the runtime's own records) and refuses what it cannot (an invented stack), and the leak test in the suite pins the third leg — that every worker, wedged ones included, has exited by the time the response is served:"),
			code("dbg-prof-handler", "go",
				"The honest refusal: a lost wedge is a broken demo, and the typed 500 says so.",
				"server/internal/presentation/demo_debug.go",
				"func (h *DebugStallHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {\n"+
					"	run := h.runStall()\n"+
					"	if run.Stalled == nil {\n"+
					"		h.logger.Error(\"debug stall run lost its own wedge\",\n"+
					"			\"request_id\", core.RequestIDFromContext(r.Context()),\n"+
					"			\"goroutines_seen\", run.GoroutineSeen,\n"+
					"			\"capture_tries\", run.CaptureTries,\n"+
					"		)\n"+
					"		core.WriteError(w, r, core.Internal(\n"+
					"			\"The stall demo could not find its wedged goroutine on the \"+\n"+
					"				\"process's own profile; no stack was invented to cover for it.\"),\n"+
					"			h.logger)\n"+
					"		return\n"+
					"	}\n"+
					"	writeJSON(w, http.StatusOK, demos.BuildDebugStallPayload(run))\n"+
					"}"),
		),
		// ── 5. The execution tracer ───────────────────────────────
		section("execution-tracer", "The execution tracer: the timeline pprof cannot show",
			"pprof answers WHERE; the execution tracer answers WHEN — a millisecond-resolution timeline of scheduling, garbage collection, and goroutine lifetimes that no aggregate can represent, and it is mounted on the same /internal/ fence.",
			text("dbg-trace-what",
				"WHAT THE TRACE SHOWS THAT PPROF CANNOT: the difference is aggregates versus TIMELINE. A CPU profile sums samples over its window — a four-millisecond GC stop-the-world pause, a goroutine starved for thirty milliseconds behind a runaway worker, a lock convoy that forms and dissolves within one second: all of these can be INVISIBLE in a profile whose averages they hide inside, and every one of them is a bright, obvious stripe on a trace. The execution tracer records events — goroutine created, runnable, running, blocked, unblocked; syscalls; network-poller wakeups; every phase of every GC cycle — with timestamps, and the viewer turns them into the views that answer latency questions directly: the Goroutine analysis (per-goroutine time in each state), the Scheduler latency profile (how long goroutines WAITED for a processor — the starvation view), GC event timelines, and user-defined TASK spans for regions you label yourself. The classic tracer diagnosis, stated as a shape: ``the request took 40 milliseconds of wall time and 2 milliseconds of CPU`` — the missing 38 is not compute, it is WAITING, and the trace is the only instrument that shows where it went."),
			text("dbg-trace-capture",
				"HOW TO CAPTURE — three doors, same format. From a RUNNING server: the same ``/internal/`` mount serves ``/debug/pprof/trace?seconds=N`` — the endpoint blocks for the window, then serves the binary trace file. From CODE: the ``runtime/trace`` package — ``trace.Start(w)`` and ``trace.Stop()`` bracket a region, the shape you reach for when a test or a specific code path is the region of interest. From TESTS: ``go test -trace=out`` wraps a whole run. Reading it is one command — ``go tool trace trace.out`` — which serves a local web UI with the views above; on a headless box (this guide's devcontainer), the same tool extracts profile-shaped answers without a browser: ``go tool trace -pprof=SYSCALL`` prints the syscall-time profile from the trace, and ``SCHED``, ``NET``, and ``SYNC`` are its siblings. Two cautions from practice: traces are BIGGER than profiles (seconds, not minutes, on your first window), and a trace taken on an idle system records nothing — generate load while the window is open:"),
			code("dbg-trace-inline", "bash", "", "",
				"# capture a second of the running server's life (load it while it runs)\n"+
					"curl -s -o /tmp/gdg-trace.out 'localhost:8100/internal/debug/pprof/trace?seconds=1'\n"+
					"go tool trace /tmp/gdg-trace.out            # the viewer, where a browser exists\n"+
					"go tool trace -pprof=SCHED /tmp/gdg-trace.out  # headless: scheduler wait, as pprof"),
			text("dbg-trace-when",
				"WHEN TO REACH FOR IT — the tracer is the second instrument, not the first, because its answers cost more to read. The triggers that justify it: LATENCY WITH IDLE CPU (pprof says nothing is hot, yet responses are slow — the time is going to scheduling, locks, or GC, and only the timeline says which); THROUGHPUT CLIFFS (fine at one load, collapsing at another — processor starvation and queueing show as scheduler stripes); GC SUSPICION (the trace shows every pause, assist, and sweep as first-class events with durations); and STARVATION (a low-priority goroutine repeatedly waiting behind bursts — invisible to every aggregate, obvious as repeated short-runs-long-waits on one row). The pairing discipline: PROFILE FIRST, TRACE SECOND — the profile says where the cost is, and only if the cost does not explain the symptom does the timeline get asked WHEN it happened. In this repo's terms: the caching herd demo's coalesced window, the realtime hub's broadcast under slow consumers, the jobs worker's retry backoff — each is a timing story the trace could verify end to end."),
		),
		// ── 6. Production debugging ───────────────────────────────
		section("production-debugging", "Debugging production: the observability loop first",
			"In production the observability loop from topic 33 is the FIRST debugging tool — logs correlated by request id, metrics with rates, a readiness probe — with the profiles behind the /internal/ fence for when the loop says the answer is inside the process.",
			text("dbg-prod-loop",
				"THE LOOP FIRST, because production debugging inverts the local order of operations: locally you repro and instrument; in production the instrumentation is ALREADY RUNNING, and discipline means reading it before touching anything. Topic 33 built the loop for exactly this moment — three questions, three instruments: WHAT BROKE is the logs (slog JSON, every line carrying the request id that threads one request's whole story together — the print-discipline section's quote, doing its job at 3am); HOW OFTEN AND HOW SLOWLY is the metrics (request counters by route-pattern and status, duration histograms whose buckets show the tail, not just the mean); IS IT ALIVE is the split probe — liveness (``/api/health``, the process answers) versus READINESS (``/api/ready``, the process's real dependencies answer within bounded timeouts). The wiring is one composition-root block, quoted:"),
			code("dbg-prod-wiring", "go",
				"The loop, wired: metrics fed by the middleware, readiness pinging real dependencies, profiles fenced.",
				"server/cmd/guide/main.go",
				"	metricsRegistry := domobs.NewMetricsRegistry()\n"+
					"	httpRequests, httpDurations := presentation.RegisterHTTPMetrics(metricsRegistry)\n"+
					"	mux.Handle(\"GET /metrics\", presentation.NewMetricsHandler(metricsRegistry, logger))\n"+
					"	mux.Handle(\"GET /api/ready\", presentation.NewReadinessHandler([]domobs.DependencyCheck{\n"+
					"		{Name: \"postgres\", Pinger: obsdata.NewPgxPinger(pool), Timeout: time.Second},\n"+
					"		{Name: \"redis\", Pinger: obsdata.NewGoRedisPinger(redisClient), Timeout: time.Second},\n"+
					"	}, logger))\n"+
					"	mux.Handle(\"/internal/\", presentation.NewInternalHandler())"),
			text("dbg-prod-pprof",
				"PPROF IN PRODUCTION, WITH CARE — the profiles you met in this topic's demo are one fence away in prod, and the care is three-sided. ACCESS: the ``/internal/`` prefix is the whole boundary story — one string for an ingress rule, a firewall, or a loopback-only bind; never expose the public root. COST: the goroutine dump is nearly free (this topic's demo takes it on the request path); a CPU profile costs its sampling window plus small overhead; a HEAP snapshot can be large; the TRACE is the most expensive instrument on the menu — short windows, and only when the timeline question is already formed. PAYOFF: production profiles beat staging profiles, because REAL traffic shapes them — the flame graph of the staging bench is a hypothesis; the flame graph under Tuesday's peak IS the answer. The sequencing that experienced operators converge on: loop first (logs and metrics localize the WHEN and WHICH), then goroutine census (is something STUCK — this takes seconds), and only then the sampling instruments."),
			text("dbg-prod-inject",
				"REHEARSED FAILURE is debugging you can practice before the incident, and this guide ships a rehearsal switch. Topic 30's scenario store exposes ``injectFailure: error | latency`` — PATCH it, and the next probe through the production middleware chain really panics (recovered into the 500 envelope, logged with stack and request id) or really drags (the injected sleep, visible afterwards in the duration histograms on ``/metrics``). The debugging skill this builds is RECOGNITION: the team that has watched the system refuse and drag ON PURPOSE knows exactly what those signals look like at 3am — which log lines appear, which metric moves, what the envelope carries — and recognition converts an unknown incident into a known one in minutes. That is the honest summary of this entire section: production debugging is mostly reading instruments that were installed BEFORE the failure, by people who practiced reading them — and the terminal companion at this topic's end runs the whole sequence against the live stack."),
		),
		// ── 7. Process nets ───────────────────────────────────────
		section("process-nets", "Process nets: the nets that make bugs diagnosable",
			"The nets are what turn a crash into a diagnosable event instead of a silent gap: bounded timeouts on everything that can block, panic recovery that logs with a stack, and a graceful shutdown that drains in-flight work — all real in this server, quoted.",
			text("dbg-nets-timeouts",
				"BOUNDED EVERYTHING — the rule this repo applies to anything that can block: give it a bound, so the failure mode is an EVENT rather than an absence. The distinction is everything for debugging: a request that times out at ten seconds FIRED A SIGNAL (which timeout, which path — diagnosable, correlated, greppable), while a request that hangs forever only generates silence and a growing connection table. The server-level shape is the standard one — bounded header, read, write, and idle windows that defeat slow-client abuse — but the same rule cascades all the way down: every dial and migration runs inside the boot's bounded context, every demo in this guide bounds its own waits, and this topic's stall demo bounds its settle and drain waits so that a demo ABOUT wedging can never wedge its own request. The composition root's server block, quoted, is the outermost net:"),
			code("dbg-nets-server", "go",
				"The outermost net: bounded reads, writes, and idles — a slow client cannot wedge a handler forever.",
				"server/cmd/guide/main.go",
				"	srv := &http.Server{\n"+
					"		Addr:    fmt.Sprintf(\":%d\", cfg.Port),\n"+
					"		Handler: handler,\n"+
					"		// Bounded reads/writes defeat slow-client abuse while leaving ample\n"+
					"		// headroom for the guide's demos.\n"+
					"		ReadHeaderTimeout: 5 * time.Second,\n"+
					"		ReadTimeout:       10 * time.Second,\n"+
					"		WriteTimeout:      15 * time.Second,\n"+
					"		IdleTimeout:       60 * time.Second,\n"+
					"	}"),
			text("dbg-nets-recover",
				"THE RECOVER NET converts the one failure Go would otherwise let take the process's request with it — a panic — into the diagnosable shape every other failure already has: a logged event with a request id, a stack, and the 500 envelope on the wire. The policy from the error-handling topic holds (panics are for PROGRAMMER bugs only, never control flow), and the middleware implements the two refinements that policy needs in production: ``http.ErrAbortHandler`` is re-raised untouched (it is net/http's own sentinel for a dead connection — writing an envelope is both impossible and wrong), and a panic after the response STARTED is logged but not double-written. The stack in that log line is the debugging payload — the same ``debug.Stack()`` the panic printed in your terminal during development, now delivered into the logs where production debugging actually reads it:"),
			code("dbg-nets-recover-code", "go",
				"The recover net: a panic becomes a request-id-correlated log entry with a stack, then the envelope.",
				"server/internal/presentation/middleware.go",
				"func Recover(logger *slog.Logger) Middleware {\n"+
					"	return func(next http.Handler) http.Handler {\n"+
					"		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {\n"+
					"			defer func() {\n"+
					"				if recovered := recover(); recovered != nil {\n"+
					"					if recovered == http.ErrAbortHandler {\n"+
					"						panic(recovered)\n"+
					"					}\n"+
					"					requestID := core.RequestIDFromContext(r.Context())\n"+
					"					logger.Error(\"panic recovered\",\n"+
					"						\"request_id\", requestID,\n"+
					"						\"method\", r.Method,\n"+
					"						\"path\", r.URL.Path,\n"+
					"						\"panic\", recovered,\n"+
					"						\"stack\", string(debug.Stack()),\n"+
					"					)\n"+
					"					// …the envelope write follows, unless the response already started\n"+
					"				}\n"+
					"			}()\n"+
					"			next.ServeHTTP(w, r)\n"+
					"		})\n"+
					"	}\n"+
					"}"),
			text("dbg-nets-shutdown",
				"GRACEFUL SHUTDOWN is the last net, and it is a debugging net twice over. Operationally: SIGINT/SIGTERM cancel a context, both serving surfaces drain in-flight work inside bounded timeouts (gRPC first — its RPCs are short — then HTTP), and a process that drains is a process you can RESTART without losing evidence or requests, which is what makes every other production debugging move affordable (capture the goroutine dump, then restart, knowing nothing in flight was destroyed mid-story). Forensically, Go ships one more deliberate instrument here, and knowing it exists is the point: ``SIGQUIT`` makes a Go process dump EVERY goroutine's stack to stderr before it exits — the destructive button for a wedged process. It dies, but it tells you what everyone was doing; run it in a terminal you can scroll, or redirect first, because the dump is exactly the goroutine-profile text this topic's demo parsed. Its gentler cousin ``GOTRACEBACK`` tunes how much a CRASH dumps (``none`` → ``single`` → ``all`` → ``system`` → ``crash``, the default ``single`` showing only the panicking goroutine and ``all`` every goroutine) — and the composition root's own shutdown, quoted, is the shape the whole paragraph rides on:"),
			code("dbg-nets-shutdown-code", "go",
				"The last net: both surfaces drain in-flight work inside bounded timeouts before the process exits.",
				"server/cmd/guide/main.go",
				"	// Both surfaces drain in-flight work: gRPC first (its RPCs are short),\n"+
					"	// then HTTP inside its bounded timeout.\n"+
					"	grpcServer.GracefulStop()\n"+
					"	logger.Info(\"grpc server stopped\")\n"+
					"	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)\n"+
					"	defer cancel()\n"+
					"	if err := srv.Shutdown(shutdownCtx); err != nil {\n"+
					"		return fmt.Errorf(\"graceful shutdown: %w\", err)\n"+
					"	}"),
		),
		// ── 8. Terminal companion ─────────────────────────────────
		section("terminal-companion", "Terminal companion: the debugging tour",
			"The whole topic over a terminal: the stall demo's parsed anatomy, the live goroutine census, a real CPU profile read with go tool pprof -top, a heap snapshot, an execution-trace capture, and the observability loop that always goes first.",
			text("term-dbg-intro",
				"With the stack up (server on :8100), the tour runs in the order the topic taught. First the DEMO — the parsed anatomy of a real wedge, read raw: the capture's provenance (profile name, goroutines seen, tries), the stalled goroutine's id, state, wait, frames WITH their source lines, and the created-by chain; then the CONTRAST row, the request's own goroutine from the same capture. Then the same instrument live and unfiltered: the goroutine CENSUS (debug=1 groups identical stacks with counts — the fastest stuck-goroutine triage in Go) and the full dump's goroutine count. Then the instruments the demo only points at: a real CPU PROFILE for a few seconds — read with ``go tool pprof -top``, the ranked answer — and a HEAP snapshot in the inuse view. Then one second of TRACE, and the loop that goes first everywhere: health, readiness, the metric rates. Every command is honest about what it costs; none of them invented a number."),
			code("term-dbg-stall-bash", "bash", "", "",
				"# the stall: parsed anatomy of a real wedge, then the contrast row\n"+
					"curl -s localhost:8100/api/demo/debug/stall | python3 -c \\\n"+
					"  'import json,sys; p=json.load(sys.stdin); c=p[\"capture\"]; \\\n"+
					"   print(c[\"profile\"], \"profile ·\", c[\"goroutinesSeen\"], \"goroutines ·\", \\\n"+
					"   c[\"captureTries\"], \"capture try ·\", str(c[\"captureMs\"])+\"ms\"); \\\n"+
					"   s=p[\"stalled\"]; print(\"stalled\", s[\"id\"], \"[\", s[\"state\"], \"]\", s.get(\"wait\",\"\")); \\\n"+
					"   [print(\"   \", f[\"function\"], \"—\", f[\"location\"]) for f in s[\"frames\"]]; \\\n"+
					"   print(\"created by:\", s[\"createdBy\"]); \\\n"+
					"   k=p[\"contrast\"]; print(\"contrast\", k[\"id\"], \"[\", k[\"state\"], \"] — the request itself\")'\n"+
					"\n"+
					"# the same instrument, live: the census, then the full count\n"+
					"curl -s 'localhost:8100/internal/debug/pprof/goroutine?debug=1' | head -6\n"+
					"curl -s 'localhost:8100/internal/debug/pprof/goroutine?debug=2' | grep -c '^goroutine '\n"+
					"\n"+
					"# the wedge is released before the response: reload and the count is stable\n"+
					"curl -s localhost:8100/api/demo/debug/stall > /dev/null && \\\n"+
					"  curl -s 'localhost:8100/internal/debug/pprof/goroutine?debug=2' | grep -c '^goroutine '"),
			code("term-dbg-profiles-bash", "bash", "", "",
				"# CPU for three seconds (load the server while it samples), then the ranked answer\n"+
					"curl -s -o /tmp/gdg-cpu.pprof 'localhost:8100/internal/debug/pprof/profile?seconds=3'\n"+
					"go tool pprof -top /tmp/gdg-cpu.pprof | head -12\n"+
					"# equivalent one-liner — pprof fetches the URL itself:\n"+
					"# go tool pprof -top 'localhost:8100/internal/debug/pprof/profile?seconds=3'\n"+
					"\n"+
					"# heap: where the live bytes live (inuse view; alloc view = GC pressure)\n"+
					"curl -s -o /tmp/gdg-heap.pprof localhost:8100/internal/debug/pprof/heap\n"+
					"go tool pprof -top -sample_index=inuse_space /tmp/gdg-heap.pprof | head -8\n"+
					"\n"+
					"# one second of timeline — scheduling, GC, goroutine lifetimes\n"+
					"curl -s -o /tmp/gdg-trace.out 'localhost:8100/internal/debug/pprof/trace?seconds=1'\n"+
					"go tool trace -pprof=SCHED /tmp/gdg-trace.out   # headless: scheduler wait, as pprof\n"+
					"# go tool trace /tmp/gdg-trace.out              # the full viewer, with a browser\n"+
					"\n"+
					"# the loop first, always: health, readiness, the rates\n"+
					"curl -s localhost:8100/api/health; echo\n"+
					"curl -s localhost:8100/api/ready | python3 -m json.tool\n"+
					"curl -s localhost:8100/metrics | grep '^guide_http_requests_total' | head -4"),
			text("term-dbg-coda",
				"Read the tour as the topic's whole argument in terminal form. The stall block is PARSED PROVENANCE — a payload whose every field was captured from the runtime while the wedge was live, with the contrast row proving the instrument sees healthy goroutines too — and the reload-plus-count pair is the RELEASE PROOF made runnable: the count before and after is the same number, because the demo releases what it wedges. The census lines are the instrument every stuck-goroutine incident actually starts with, in the field, before anyone opens a profiler UI. The CPU block is this topic's core read — ``-top`` ranking flat and cumulative, the same samples a flame graph would draw as widths; the heap block is the other half of the decision table (slow → CPU, growing → heap); the trace block is the WHEN instrument, extracted headless the way a container session needs it. And the last block is the discipline the production section argued: the loop reads FIRST, before any of the heavier instruments are asked anything. Mindset → prints → delve → profiles → trace → the loop → the nets: every section of this topic was one of those instruments, and the tour runs them all in two terminals' worth of commands."),
		),
	}

	if buildErr != nil {
		return Topic{}, buildErr
	}
	return NewTopic(
		"debugging",
		"Debugging in Go: from print to profile",
		"How a Go system tells you why it is misbehaving, taught from "+
			"this repo's own machinery: the mindset (read the error — Go's "+
			"wrapped error chains are a compressed narrative; form ONE "+
			"falsifiable hypothesis before any probe; bisect the space — "+
			"git bisect in time, the layer map's middle seam in space). "+
			"Print discipline stated honestly: fmt.Println dies at scale "+
			"(no correlation, levels, structure, cost control, or "+
			"lifecycle) but the disciplined temporary print is not a sin — "+
			"and when the print stays, it becomes the request logger's "+
			"slog shape, quoted with request ids that thread one request "+
			"into one story. delve as the real debugger — installed per "+
			"machine (deliberately NOT in the manifest), session verbs "+
			"(break/continue/print/stack/goroutines), exec vs attach vs "+
			"dlv test driving a table loop one row at a time — taught in "+
			"prose and terminal, honestly not wired as a demo because a "+
			"breakpoint prompt cannot ride a GET. pprof: topic 33's REAL "+
			"/internal/ mount quoted (default-mux side effect neutralized, "+
			"one prefix to fence), CPU vs heap as a decision table (slow → "+
			"CPU, growing → heap, stuck → the under-used goroutine census), "+
			"flame graph as a projection of the same samples — and the "+
			"topic's demo, debug:stall: a REAL wedge (one worker of five "+
			"blocking forever on a channel nobody serves) whose anatomy is "+
			"captured from the process's OWN goroutine profile via "+
			"runtime/pprof, parsed by a table-tested reader of the "+
			"runtime's grammar (header states with durations, multi-frame "+
			"stacks, created-by pairs), served beside the request's own "+
			"goroutine as contrast — released deterministically before the "+
			"response returns, leak-proven, bounded so the demo can never "+
			"wedge its own request, and refusing to invent a stack if the "+
			"wedge is lost. The execution tracer: what aggregates cannot "+
			"show (GC pauses, starvation, wait-vs-compute), three capture "+
			"doors (endpoint, runtime/trace, go test -trace), go tool "+
			"trace's viewer and headless -pprof extraction. Production: "+
			"the observability loop FIRST (logs by request id, metric "+
			"rates, readiness — wiring quoted), pprof in prod with care "+
			"(access fence, per-instrument cost, real traffic's payoff), "+
			"and rehearsed failure via topic 30's injectFailure as "+
			"recognition training. The process nets that turn crashes "+
			"into diagnosable events: bounded timeouts everywhere (server "+
			"read/write windows quoted — an expiry is a signal, a hang is "+
			"only an absence), the recover middleware logging stack + "+
			"request id into the envelope's 500, graceful shutdown that "+
			"drains both surfaces, and the forensic pair SIGQUIT / "+
			"GOTRACEBACK. Terminal companion: the stall's parsed anatomy, "+
			"the live goroutine census, a real go tool pprof -top on a "+
			"captured CPU profile, the heap's inuse view, one second of "+
			"trace, and the loop that always reads first.",
		"platform",
		35,
		sections,
	)
}
