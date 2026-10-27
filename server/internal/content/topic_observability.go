package content

import (
	"errors"
	"fmt"
)

// This file is topic 33 observability (group platform, curriculum order
// 33): the discipline of answering three questions about a running
// service — what happened, where, and why right now — using three
// signals that are one system (logs, metrics, traces, joined by shared
// ids). Everything quoted below is a line this server compiles and
// serves today, and the topic's landing built the real thing: a
// /api/ready readiness endpoint pinging PostgreSQL and Redis inside
// bounded timeouts, a /metrics endpoint speaking the Prometheus text
// exposition format from a hand-rolled stdlib registry whose labels
// are route PATTERNS (bounded by construction), pprof mounted on a
// dedicated /internal/ prefix off the public root, and W3C
// trace-context propagation — traceparent parsed and validated to the
// spec's own edge cases, echoed outbound, rejected honestly when
// malformed — feeding a span-tree recorder the demo renders. The two
// demos run against the running server: the slog viewer (one real log
// line dissected field by field, the level matrix computed by really
// gating statements, a request replay proving the id correlation) and
// the trace round-trip (your traceparent accepted, child spans
// recorded across handler → use case → adapter, the hostile-header
// table parsed for real). No new dependencies: slog, net/http/pprof,
// and the W3C header are all the protocol you need; the OpenTelemetry
// SDK and the Prometheus client appear as honest comparison prose,
// not imports.

// observabilityTopic builds topic 33.
func observabilityTopic() (Topic, error) {
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
		// ── 1. Three questions ─────────────────────────────────────
		section("three-questions", "Three questions: logs, metrics, traces as one system",
			"Observability is not a tool you install; it is the property of being able to answer three questions about production without shipping new code to do it.",
			text("obs-questions-framing",
				"When something is wrong in production you are always asking one of three questions. WHAT HAPPENED — the narrative: which request, what did it touch, what did it say when it failed. WHERE — the shape: which endpoint, which dependency, how often, since when. WHY NOW — the causality: what this request did that the healthy ones did not, which layer the milliseconds died in. The three signals map onto those questions, and the mapping explains their shapes better than any feature list: LOGS are rows of structured events (unbounded in variety, cheap to write, expensive to search — they answer WHAT); METRICS are pre-aggregated numbers (bounded in variety, trivially cheap to store and query over time — they answer WHERE); TRACES are causal trees of spans (one per request, carrying ids that cross process boundaries — they answer WHY NOW, the question neither of the others can). Treat them as ONE SYSTEM rather than three purchases, because that is how they are consumed at 3 a.m.: the metric says the error rate on /api/posts spiked, the trace shows the spike's requests dying inside the PostgreSQL span, and the log line — joined by the request id the trace and the log both carry — says what the database actually said. This guide runs all three, stdlib-only, on the server you are reading:"),
			list("obs-questions-signals",
				false,
				"Logs — one structured JSON line per event from Go's standard log/slog: machine-parseable, correlated by request id, gated by a level dial; the section after next dissects a real one.",
				"Metrics — a /metrics endpoint speaking the Prometheus text exposition format by hand: request counters and duration histograms whose labels are route patterns, bounded by the route table itself.",
				"Traces — W3C trace-context propagation (the traceparent header) plus a stdlib span recorder: the same protocol OpenTelemetry speaks, implemented small enough to see every moving part.",
				"The joins — the request id on every log line and every error envelope, the trace id on every span: the two keys that turn three silos into one investigation.",
			),
			text("obs-questions-cost",
				"The stdlib-first discipline this topic follows is the same one the JWT section of the authentication topic used for HS256: implement the PROTOCOL, and let the libraries stay comparison prose. The Prometheus text exposition format is plain text by design — ``# HELP``, ``# TYPE``, one sample per line — and implementing it by hand teaches more than importing a client hides. The W3C trace-context header is four dash-separated hex fields with a spec full of deliberate edge cases (all-zero ids are invalid, version ``ff`` is reserved, version 00 must carry exactly four fields) — parsing it correctly IS the curriculum, and the parser below is table-tested against every one of those edges. Where a real SDK genuinely adds machinery — samplers, exporters, the OTLP transport, baggage — the prose says so in so many words. What no SDK adds is the hard part anyway: emitting the ids consistently, bounding the label cardinality, and keeping the signals cheap enough to leave on in production."),
		),
		// ── 2. slog JSON [demo obs:viewer] ─────────────────────────
		section("slog-json", "slog JSON: the log line is data",
			"The server you are reading writes one JSON object per log line to stdout — Go's standard structured logger, configured in four lines and dissected field by field in the demo below.",
			text("obs-slog-what",
				"Since Go 1.21 the standard library ships ``log/slog`` — a structured logger with first-class levels, attribute-based fields, and pluggable handlers. Two decisions in this server's four-line constructor shape everything downstream. FIRST, the handler is ``slog.NewJSONHandler``: every line is a JSON object, which means every line is PARSEABLE — a pipeline (or a ``python3 -c`` one-liner in the terminal tour) can read fields out of it without regexes, where a free-text line like ``GET /api/posts 200 3.2ms`` forces every consumer to re-parse prose. SECOND, the sink is stdout: the container-native choice — the process writes, the platform (Docker's json-file driver, Kubernetes, any collector) ships, and the application never opens a log file, never rotates one, and never blocks on disk. The level comes from typed config, read once at boot from ``GDG_LOG_LEVEL`` — the same fail-fast environment discipline every other knob follows:"),
			code("obs-slog-logger", "go",
				"The whole logger, quoted from core: JSON on stdout, gated at the configured level.",
				"server/internal/core/logging.go",
				"func NewLogger(cfg Config) *slog.Logger {\n"+
					"	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{\n"+
					"		Level: cfg.LogLevel,\n"+
					"	}))\n"+
					"}"),
			text("obs-slog-anatomy",
				"A real line makes the abstractions concrete — this is the shape every request this server serves produces (times and durations from a live capture; your values will differ, the SHAPE never will): ``{\"time\":\"2026-10-27T10:15:02.412Z\",\"level\":\"INFO\",\"msg\":\"http_request\",\"method\":\"GET\",\"path\":\"/api/posts\",\"status\":200,\"duration_ms\":1.8,\"request_id\":\"8f0c9f2a\"}``. The demo below dissects exactly such a line FIELD BY FIELD — what wrote each key, what a consumer does with it — and adds two panels the anatomy needs to become a system: the LEVEL MATRIX (which configurations let which severities through — computed by really gating statements through a handler configured at each level, not restated from documentation) and the REPLAY (two real requests walked through the production middleware chain in-process with a buffer logger — a 200 and a 404 sharing one request id, the 404's warn line and both access lines captured verbatim, which is the correlation contract that makes thousands of lines one story):"),
			demo("obs:viewer", "The slog viewer: one JSON line, dissected live",
				"A real http_request line's fields anatomized key by key (time, level, msg, method, path, status, duration_ms, request_id — each with what wrote it and what joins on it); the level matrix computed by REALLY firing each severity through a handler gated at each GDG_LOG_LEVEL; and a request replay — a 200 and a 404 walked through the production middleware chain in-process with a buffer logger — proving the request-id correlation: one id threading the failure's warn line, both access lines, and the 404's error envelope."),
			text("obs-slog-reading",
				"Read the panel the way a log consumer does. The ANATOMY row order is the reading order: timestamp (when), level (how loud), msg (which event — the filter key), then the event's own fields. The LEVEL matrix is asymmetric by design — turning the dial down buys quiet and blindness, turning it up buys detail and cost — and the honest row is the first one: this server currently logs NOTHING at debug, because debug is the investigation dial you turn up when something is wrong, not a default you pay for always. The REPLAY is the topic's thesis performed: the 404's ``error response`` warn line arrives BEFORE its access line (the central error writer logs on the way out, the access logger logs at request completion), both carry the same id, and that same id rides the 404 envelope the client received — three artifacts of one failure, joinable from any one of them."),
		),
		// ── 3. Request logging ─────────────────────────────────────
		section("request-logging", "Request logging: the middleware that writes the ledger",
			"One structured line per request, written at completion by the same middleware chain every request already flows through — the log is a property of the pipeline, not a call scattered through handlers.",
			text("obs-reqlog-shape",
				"The request logger is middleware (topic 08's discipline applied to observability), and its position in the chain is load-bearing: it sits OUTSIDE panic recovery, so a recovered panic is still logged as a finished request with its true 500 status — the ledger records that the request happened and how it ended, which is a different question than what it was doing when it broke. It wraps the real ``ResponseWriter`` in a status recorder, starts a clock, and writes exactly one line in a ``defer`` when the stack beneath returns. The field names mirror the family's other guides (``duration_ms``, ``request_id``) so one pipeline reads them all identically — a small convention with a real payoff when your grep works across four codebases:"),
			code("obs-reqlog-middleware", "go",
				"The whole access logger: status captured through a recorder, one line per request at completion, duration inclusive of everything beneath.",
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
			text("obs-reqlog-correlation",
				"Note what the logger does NOT do: it does not generate the request id — it READS one from the context, stamped outermost by the request-id middleware, so the same id serves the access line, every error envelope, the warn the central error writer logs, and the response header the client can quote back in a bug report. One id per request, threaded through everything, is the cheapest observability decision in the whole system and the one everything else depends on. The duration is INCLUSIVE — the clock starts above the inner middleware, routing, handler, and serialization, so the number is what the client felt, not what the handler alone cost; when you need the handler-alone number, that is what the trace spans (this topic's second demo) and the pprof profiles (two sections down) decompose it into. The logs-versus-metrics split falls out of cost: this line is written for EVERY request because logs are the narrative of record, but answering ``how often`` and ``how slow, over time`` by counting log lines is paying search prices for numbers — which is what metrics are for."),
		),
		// ── 4. Levels ──────────────────────────────────────────────
		section("levels", "Levels: when a line earns its severity",
			"Four severities, one gate, and an economic rule: a level is a promise about who reads the line and what it costs to have written it.",
			text("obs-levels-economics",
				"A log level is not a mood; it is a COST decision made per line, summed per process. Every line written costs a little (formatting, I/O) and costs again forever (storage, and the attention of whoever greps it later), so the dial exists to spend that cost only where it pays. The working rule this server follows: a line's level is set by what a reader must be able to DO with it — and the four levels, in the order the gate lets them through:"),
			list("obs-levels-earns",
				false,
				"Debug — the investigation dial: detail you would never pay for by default but want one syscall deep when chasing a specific failure; this server writes nothing at debug today, and that absence is deliberate (turn the dial up in config when a live puzzle deserves it).",
				"Info — the heartbeat: events that describe normal operation and must be cheap and complete — every request's access line, boot events (listening, migrations applied, seeds); the default level, because the ledger must be whole.",
				"Warn — expected failures: something went wrong for one request or one dependency, the process is fine and the request was answered honestly — every 4xx error response the central writer logs, an optional service degrading (mongo unavailable, boot continues).",
				"Error — process-shaped trouble: recovered panics with their stacks, taxonomy misses an operator must see, boot-stopping failures; a line here should sting.",
			),
			text("obs-levels-mechanics",
				"The mechanics are one field in one constructor: the handler's ``Level`` option, fed from typed config read at boot — the matrix in the demo above COMPUTED the truth table by constructing a handler at each of the four settings and firing one statement per severity through it (info-level config silences debug; error-level config silences everything but error; the relationship is a total order, which is why the table is a triangle). Two production notes the table implies. First, the dial is per PROCESS and read ONCE — changing it means a restart or a platform-level config rollout, so levels are for steady-state policy, not for reaching into a running system (that is what dynamic debug surfaces — a debug endpoint, a config service — are for, and this guide deliberately does not build one). Second, the level never changes the line's CONTENT — a warn demoted to debug is the same JSON with a different level field — because the content is what the pipeline indexes; the level is only the gate it passed."),
		),
		// ── 5. Health: liveness vs readiness ───────────────────────
		section("health-readiness", "Health: liveness vs readiness",
			"Two probes with opposite jobs: liveness says restart me, readiness says stop routing to me — conflating them turns a dependency outage into a restart storm.",
			text("obs-health-liveness",
				"The guide has served a health endpoint since the skeleton phase, and its deliberate emptiness is the teaching: it touches NOTHING — no database ping, no redis ping, no dependency of any kind — because it answers exactly one question, IS THE PROCESS ABLE TO SERVE AT ALL. That is LIVENESS, and its consumer is the platform's restart logic: if this endpoint fails, the right move is a restart (the process is wedged — deadlocked, out of memory, gone). Read its comment's promise: dependency checks are explicitly deferred to a readiness endpoint, because a liveness probe that fails when PostgreSQL is down tells the platform to restart every instance simultaneously — solving nothing (PostgreSQL is still down) while dropping every in-flight request and every warm cache. Liveness is cheap, dependency-free, and almost always 200:"),
			code("obs-health-handler", "go",
				"The liveness probe, whole: process-up, dependency-free by design — readiness is a different endpoint with a different job.",
				"server/internal/presentation/health.go",
				"// HealthHandler serves GET /api/health, the family liveness probe. It is\n"+
					"// deliberately dependency-free (no DB/redis ping): it answers \"is the\n"+
					"// process up\", not \"are its dependencies up\" — readiness splits arrive\n"+
					"// with the observability topic.\n"+
					"type HealthHandler struct {\n"+
					"	env string\n"+
					"}\n"+
					"\n"+
					"// NewHealthHandler returns a handler that reports the environment the\n"+
					"// server booted in.\n"+
					"func NewHealthHandler(env string) *HealthHandler {\n"+
					"	return &HealthHandler{env: env}\n"+
					"}\n"+
					"\n"+
					"// ServeHTTP implements http.Handler for the health probe.\n"+
					"func (h *HealthHandler) ServeHTTP(w http.ResponseWriter, _ *http.Request) {\n"+
					"	w.Header().Set(\"Content-Type\", \"application/json\")\n"+
					"	// An Encode failure means the peer vanished mid-write; there is no\n"+
					"	// recovery action, and the error already surfaces to the client.\n"+
					"	_ = json.NewEncoder(w).Encode(healthResponse{Status: \"ok\", Env: h.env})\n"+
					"}"),
			text("obs-health-readiness",
				"READINESS is the other half: CAN THIS INSTANCE SERVE ITS TRAFFIC RIGHT NOW — which depends on its dependencies answering. Its consumer is the load balancer (or the scheduler's endpoints controller): a failing readiness probe means STOP ROUTING HERE, temporarily, without killing anything — the instance stays up, finishes in-flight work, heals into the pool when the dependency returns. This topic lands the real one: ``GET /api/ready`` pings the process's ACTUAL PostgreSQL pool and Redis client through a domain-owned port, each inside its own bounded timeout, and answers with per-dependency rows. The mapping discipline is the error-handling topic's, unchanged: ready is a plain 200 body; not-ready is the standard 503 ``service_unavailable`` envelope with every dependency's row riding the details slot — an honest 503 that names which dependency and why, never a bare one:"),
			code("obs-ready-handler", "go",
				"The readiness mapping: 200 with every row when ready, the standard 503 envelope naming the down dependencies when not.",
				"server/internal/presentation/readiness.go",
				"func (h *ReadinessHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {\n"+
					"	report := obs.CheckDependencies(r.Context(), h.checks)\n"+
					"	rows := make([]readinessCheckRow, 0, len(report.Checks))\n"+
					"	var down []string\n"+
					"	for _, check := range report.Checks {\n"+
					"		rows = append(rows, readinessCheckRow{\n"+
					"			Name:      check.Name,\n"+
					"			OK:        check.OK,\n"+
					"			LatencyMs: float64(check.Latency.Microseconds()) / 1000,\n"+
					"			Error:     check.Error,\n"+
					"		})\n"+
					"		if !check.OK {\n"+
					"			down = append(down, check.Name)\n"+
					"		}\n"+
					"	}\n"+
					"	if report.Ready {\n"+
					"		writeJSON(w, http.StatusOK, readinessBody{Status: \"ready\", Checks: rows})\n"+
					"		return\n"+
					"	}\n"+
					"	err := core.New(core.KindServiceUnavailable, \"Service not ready: \"+strings.Join(down, \", \")+\" unavailable.\")\n"+
					"	err.Details = readinessDetails{Checks: rows}\n"+
					"	core.WriteError(w, r, err, h.logger)\n"+
					"}"),
			text("obs-health-bounds",
				"The bound is the quiet half of the design. A readiness probe has a deadline of its own (the prober's), and a probe that waits on a wedged dependency fails the WHOLE deadline without learning anything — so every check runs inside its own timeout, and the failure row names the bound it exceeded. The pingers are data-layer adapters over the process's EXISTING pool and client (no new connections, no second pool), satisfying a port the domain owns — the dependency rule applied to probes:"),
			code("obs-ready-bound", "go",
				"One bounded check: the port's adapter answers inside its own timeout or reports down with the bound named.",
				"server/internal/domain/obs/readiness.go",
				"func checkDependency(ctx context.Context, check DependencyCheck) DependencyStatus {\n"+
					"	if check.Pinger == nil {\n"+
					"		return DependencyStatus{\n"+
					"			Name:  check.Name,\n"+
					"			OK:    false,\n"+
					"			Error: \"no pinger wired (composition-root bug)\",\n"+
					"		}\n"+
					"	}\n"+
					"	timeout := check.Timeout\n"+
					"	if timeout <= 0 {\n"+
					"		timeout = DefaultDependencyTimeout\n"+
					"	}\n"+
					"	pingCtx, cancel := context.WithTimeout(ctx, timeout)\n"+
					"	defer cancel()\n"+
					"\n"+
					"	started := time.Now()\n"+
					"	err := check.Pinger.Ping(pingCtx)\n"+
					"	latency := time.Since(started)\n"+
					"	// …the error row carries the cause and the bound; success the latency\n"+
					"}"),
		),
		// ── 6. pprof in production ─────────────────────────────────
		section("pprof-in-production", "pprof in production: the runtime's own instruments",
			"The Go runtime ships a profiler that answers 'where did the CPU go' and 'what is holding memory' at the process level — net/http/pprof exposes it over HTTP, and the only real decision is where to mount it.",
			text("obs-pprof-what",
				"``net/http/pprof`` is the standard library's window onto the runtime's profiling machinery: CPU profiles (where cycles went), heap profiles (what is allocated and who holds it), goroutine dumps (what every goroutine is doing right now), mutex and block profiles (what is contending and what is waiting). It is the tool the debugging topic walks in depth from the operator's side; this section's job is the SERVER-SIDE wiring decision, which has one famous trap. The package's ``init`` registers its handlers on ``http.DefaultServeMux`` as an import side effect — which matters exactly never here, because this server (like any serious one) builds its OWN ``ServeMux`` and never serves the default one; the registrations sit inert until a mux the composition root owns mounts them deliberately. The deliberate mount is where the policy lives: a DEDICATED INTERNAL PREFIX, not the public root:"),
			code("obs-pprof-mount", "go",
				"The internal surface: the runtime's profiles behind one fence-able prefix, StripPrefix bridging to the package's own paths.",
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
			text("obs-pprof-why-prefix",
				"Why a dedicated prefix instead of the conventional root-level ``/debug/pprof``? Because the mount point is the ACCESS POLICY, and one prefix — ``/internal/`` — gives operations exactly one string to enforce at whatever layer already has an opinion: an ingress rule that drops it from public routing, a network policy that admits only the ops subnet, or a bind to a loopback-only listener when the profiles should never leave the host. A profiler endpoint is real surface area (profiles cost CPU to capture; a heap dump can contain request data in memory), so its exposure must be a DECISION, and the prefix is the decision made greppable. What rides inside:"),
			list("obs-pprof-profiles",
				false,
				"Index (/internal/debug/pprof/) — the profile directory; bare it serves a heap snapshot, and every link below is reachable from it.",
				"profile?seconds=N — a CPU profile over N seconds of live traffic: which functions the cycles went to, read with `go tool pprof`.",
				"heap — allocation profile: what is live, what allocated it, the input to every 'memory grows until restart' investigation.",
				"goroutine?debug=1 — every goroutine's stack, now: the input to every leak and deadlock investigation.",
				"block and mutex — contention profiles (enabled by rate): where goroutines wait on locks, which locks they wait ON.",
				"trace?seconds=N — the execution tracer's window (scheduler, GC, syscalls) — the debugging topic's deep-dive tool.",
			),
			text("obs-pprof-cost",
				"The cost model is what makes this safe to run in production rather than a dev-only luxury: CAPTURING a profile costs (the CPU profile samples at a fixed rate while it runs; a heap dump stops the world briefly), but MERELY MOUNTING the routes costs nothing measurable — no goroutines spin, no sampling runs, until someone requests a profile. That asymmetry is the whole argument for mounting by default and fencing with policy: the moment you need a profile is precisely the moment you cannot wait for a redeploy. The dev container leaves the prefix reachable (the terminal tour curls it); a production deployment would fence it — the composition root makes the same mount either way, and the fence lives outside the process."),
		),
		// ── 7. Prometheus metrics ──────────────────────────────────
		section("prometheus-metrics", "Prometheus metrics: /metrics by hand, labels by route table",
			"A metrics endpoint is a text format and a registry — both small enough to build honestly in stdlib, which is exactly what this server does: counters and histograms, exposition rendered by hand, and a label discipline that bounds cardinality by construction.",
			text("obs-metrics-model",
				"Prometheus is a pull model: the server EXPOSES current metric values at one endpoint; a scraper (Prometheus itself, or anything that speaks the format) GETs it on an interval and does the storing, graphing, and alerting. The exposition format is deliberately, almost embarrassingly, plain — a ``# HELP`` comment, a ``# TYPE`` line, then one sample per series: ``guide_http_requests_total{method=\"GET\",path=\"/api/content\",status=\"2xx\"} 42``. That plainness is why the stdlib-first move is honest here rather than ascetic: a registry holding counters and histograms, plus a renderer that emits those lines with the three defined escapes, is a small amount of exact code — and writing it teaches the data model (names, label sets, series identity, cumulative buckets) that using a client library well ALSO requires you to know. The registry is mutex-guarded, render order is deterministic (families in registration order, series sorted), and the histogram buckets are the canonical seconds set the client library itself ships:"),
			code("obs-metrics-render", "go",
				"The label renderer: the format's three escapes, applied when the sample line is built — the whole 'protocol' surface of a metrics endpoint.",
				"server/internal/domain/obs/metrics.go",
				"func formatLabels(labelNames, values []string, extra ...string) string {\n"+
					"	if len(labelNames) == 0 && len(extra) == 0 {\n"+
					"		return \"\"\n"+
					"	}\n"+
					"	parts := make([]string, 0, len(labelNames)+len(extra)/2)\n"+
					"	for i, name := range labelNames {\n"+
					"		parts = append(parts, name+`=\"`+escapeLabelValue(values[i])+`\"`)\n"+
					"	}\n"+
					"	for i := 0; i+1 < len(extra); i += 2 {\n"+
					"		parts = append(parts, extra[i]+`=\"`+escapeLabelValue(extra[i+1])+`\"`)\n"+
					"	}\n"+
					"	return \"{\" + strings.Join(parts, \",\") + \"}\"\n"+
					"}"),
			text("obs-metrics-cardinality",
				"The decision that makes or breaks a metrics system is LABEL CARDINALITY, and it is where hand-rolling sharpens the lesson. A series is the metric name PLUS its label values; a time-series database pays per series, forever — so a label on USER ID or raw PATH is an outage with a delay (a million users is a million series). This server's rule, enforced in the middleware rather than hoped for in review: label values come only from CLOSED VOCABULARIES — the HTTP method, the status CLASS (2xx…5xx, because for aggregation 404 and 422 are one story), and the ROUTE PATTERN the mux matched, never the raw path. The middleware asks the router itself what pattern the request will match, so ten thousand distinct post ids are all ``/api/posts/{id}`` — one series — and an unroutable probe lands on the single ``unmatched`` label:"),
			code("obs-metrics-middleware", "go",
				"The instrumentation edge: every request counted and timed onto route-pattern labels — the mux's own match is the label source.",
				"server/internal/presentation/metrics.go",
				"func MetricsMiddleware(\n"+
					"	requests *obs.Counter,\n"+
					"	durations *obs.Histogram,\n"+
					"	mux *http.ServeMux,\n"+
					") Middleware {\n"+
					"	return func(next http.Handler) http.Handler {\n"+
					"		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {\n"+
					"			rec := newStatusRecorder(w)\n"+
					"			started := time.Now()\n"+
					"			next.ServeHTTP(rec, r)\n"+
					"\n"+
					"			pattern := routePatternFor(mux, r)\n"+
					"			durations.Observe(time.Since(started).Seconds(), r.Method, pattern)\n"+
					"			requests.Inc(r.Method, pattern, statusClass(rec.status))\n"+
					"		})\n"+
					"	}\n"+
					"}\n"+
					"\n"+
					"func routePatternFor(mux *http.ServeMux, r *http.Request) string {\n"+
					"	_, pattern := mux.Handler(r)\n"+
					"	if pattern == \"\" {\n"+
					"		return unmatchedPatternLabel\n"+
					"	}\n"+
					"	if _, path, found := strings.Cut(pattern, \" \"); found {\n"+
					"		return path\n"+
					"	}\n"+
					"	return pattern\n"+
					"}"),
			text("obs-metrics-wiring",
				"The composition root registers the two families once (``guide_http_requests_total`` by method/pATTERN/status-class, ``guide_http_request_duration_seconds`` as a histogram by method/pATTERN), mounts ``GET /metrics`` to serve the registry rendered live, and wraps the finished middleware chain from the OUTSIDE with the instrumentation edge — no line inside the documented request-id/logging/recovery order changes, which is the additive-wrapping discipline this whole topic repeats: observability is added by decorating seams, never by editing them. The wrap sits outermost so it sees the final status of everything, including requests the inner stack fails to route honestly; the /metrics scrape itself is counted too (scraping is traffic), which the alerting section's math simply divides out. The terminal tour scrapes the endpoint with plain curl — the format's plainness is its own demo."),
		),
		// ── 8. Trace context [demo obs:trace] ──────────────────────
		section("trace-context", "W3C trace context: propagation and the span tree",
			"OpenTelemetry is an SDK; distributed tracing is a HEADER — four hex fields the W3C trace-context spec defines, which this server parses, validates, propagates, and echoes for real, feeding a stdlib span recorder the demo renders.",
			text("obs-trace-protocol",
				"The insight that makes this section teachable without a dependency is that the load-bearing part of distributed tracing is a PROTOCOL, not a toolkit: the ``traceparent`` header, whose value is ``VERSION-TRACEID-PARENTID-FLAGS`` — the version (``00`` today, ``ff`` reserved forever), the trace id both sides share (32 hex digits, all-zeros invalid by spec), the sender's span id (16 hex digits, all-zeros invalid), and two hex digits of flags (bit 0: sampled). Every W3C-conformant system — OpenTelemetry's SDKs, most vendors' agents — speaks exactly this header, so a server that parses and propagates it correctly is TRACE-COMPATIBLE with that entire ecosystem before it records a single span of its own. The parser below enforces the spec's own edge table — and its error RETURNS are the other half of the contract: a malformed header is reported honestly and never adopted, but never fails the request either (the caller's telemetry bug is not the caller's outage):"),
			code("obs-trace-parser", "go",
				"The spec's edge table as code: widths, reserved version, all-zero ids, four-fields-exactly for version 00 — every rejection an error naming its rule.",
				"server/internal/domain/obs/tracecontext.go",
				"func ParseTraceparent(header string) (TraceContext, error) {\n"+
					"	value := strings.TrimSpace(header)\n"+
					"	if value == \"\" {\n"+
					"		return TraceContext{}, fmt.Errorf(\"traceparent: empty header\")\n"+
					"	}\n"+
					"	parts := strings.Split(value, \"-\")\n"+
					"	if len(parts) < 4 {\n"+
					"		return TraceContext{}, fmt.Errorf(\n"+
					"			\"traceparent %q: want version-traceid-parentid-flags, got %d field(s)\", value, len(parts))\n"+
					"	}\n"+
					"\n"+
					"	version, err := parseHexField(value, parts[0], 2, \"version\")\n"+
					"	if err != nil {\n"+
					"		return TraceContext{}, err\n"+
					"	}\n"+
					"	if version == \"ff\" {\n"+
					"		return TraceContext{}, fmt.Errorf(\"traceparent %q: version ff is reserved\", value)\n"+
					"	}\n"+
					"	if version == \"00\" && len(parts) != 4 {\n"+
					"		return TraceContext{}, fmt.Errorf(\n"+
					"			\"traceparent %q: version 00 must carry exactly four fields, got %d\", value, len(parts))\n"+
					"	}\n"+
					"\n"+
					"	traceID, err := parseHexField(value, parts[1], 32, \"trace-id\")\n"+
					"	// …same shape for parent-id (16) and flags (2), plus the\n"+
					"	// all-zeros refusals the spec reserves\n"+
					"	if traceID == strings.Repeat(\"0\", 32) {\n"+
					"		return TraceContext{}, fmt.Errorf(\"traceparent %q: trace-id must not be all zeros\", value)\n"+
					"	}\n"+
					"	// …\n"+
					"	return TraceContext{Version: version, TraceID: traceID, ParentID: parentID, Flags: flags}, nil\n"+
					"}"),
			text("obs-trace-middleware",
				"Propagation is one middleware, added to the production chain by WRAPPING it from the outside — the same additive discipline the metrics edge uses, and deliberately the same shape the request-id middleware has had since the skeleton phase: parse a header at the edge, store it in the request context, echo a response header, and let everything beneath read it for free. Three behaviors fall out of the code: a VALID inbound header is adopted verbatim (context + outbound echo carrying the server's own fresh span id, so the caller can graft this request's work onto its trace); a MALFORMED one is rejected with a debug log naming header and reason while the request proceeds under a fresh root; and NO header means this server is the trace's root. The companion ``tracestate`` list rides along validated (order preserved, members counted, the teaching subset of the ABNF):"),
			code("obs-trace-mw", "go",
				"The propagation edge: adopt, reject honestly, or mint — and always echo the server's own span back to the caller.",
				"server/internal/presentation/trace.go",
				"		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {\n"+
					"			tc := obs.NewRoot()\n"+
					"			inbound := strings.TrimSpace(r.Header.Get(obs.TraceparentHeader))\n"+
					"			if inbound != \"\" {\n"+
					"				parsed, err := obs.ParseTraceparent(inbound)\n"+
					"				if err != nil {\n"+
					"					// Honest degradation, visible at the debug dial: the\n"+
					"					// header is named, the reason is named, the request\n"+
					"					// lives under a fresh root.\n"+
					"					logger.Debug(\"traceparent rejected; starting a fresh root\",\n"+
					"						\"header\", inbound,\n"+
					"						\"reason\", err.Error(),\n"+
					"						\"request_id\", requestIDOf(r),\n"+
					"					)\n"+
					"				} else {\n"+
					"					// …tracestate parsed alongside when present\n"+
					"					tc = parsed\n"+
					"				}\n"+
					"			}\n"+
					"			// The response carries the server span's context: same trace,\n"+
					"			// fresh span id — set before the inner stack runs so even a\n"+
					"			// mid-response failure keeps it, exactly like the request id.\n"+
					"			w.Header().Set(obs.TraceparentHeader, tc.Child().String())\n"+
					"			if len(tc.Tracestate) > 0 {\n"+
					"				w.Header().Set(obs.TracestateHeader, obs.FormatTracestate(tc.Tracestate))\n"+
					"			}\n"+
					"			next.ServeHTTP(w, r.WithContext(obs.WithTraceContext(r.Context(), tc)))\n"+
					"		})"),
			text("obs-trace-recorder",
				"A span TREE needs a recorder, and the recorder's mechanics are the tracer's whole trick in miniature: opening a span reads the trace id from the propagated context and the parent from the CURRENT span on that context, and returns a context carrying the new span as current — so nested instrumentation parents itself correctly with no global registry and no thread-local (this is, structurally, what every tracer's context propagation does; Go's explicit context makes the plumbing visible). The first span under an inbound context parents on the REMOTE caller's span id — that single pointer is what grafts a local tree onto a distributed one:"),
			code("obs-trace-start", "go",
				"The tracer's mechanics: context in, context out, one parent pointer — and the first span grafts onto the inbound remote parent.",
				"server/internal/domain/obs/spans.go",
				"func (r *SpanRecorder) Start(ctx context.Context, name, layer string) (context.Context, *Span) {\n"+
					"	traceID := NewTraceID()\n"+
					"	parentID := \"\"\n"+
					"	if tc, ok := TraceContextFromContext(ctx); ok {\n"+
					"		traceID = tc.TraceID\n"+
					"		// The FIRST span under an inbound context parents on the REMOTE\n"+
					"		// caller's span id — that is what grafts the local tree onto the\n"+
					"		// distributed one.\n"+
					"		parentID = tc.ParentID\n"+
					"	}\n"+
					"	if current, ok := currentSpanID(ctx); ok {\n"+
					"		parentID = current\n"+
					"	}\n"+
					"	span := &Span{\n"+
					"		TraceID:  traceID,\n"+
					"		SpanID:   r.newID(),\n"+
					"		ParentID: parentID,\n"+
					"		Name:     name,\n"+
					"		Layer:    layer,\n"+
					"		Start:    r.now(),\n"+
					"		recorder: r,\n"+
					"	}\n"+
					"	// …the returned context carries the span as current\n"+
					"	return withCurrentSpanID(ctx, span.SpanID), span\n"+
					"}"),
			text("obs-trace-seams",
				"Where spans open is the architecture topic's lesson replayed with a different instrument: at the SEAMS, by decoration, never by editing the layers. The decorator below implements the domain's ``PostsRepository`` port and records every call through it — the same wrapping move the architecture tracer TIMED with — and the composition root stacks two of them (the adapter's leaf, the domain's port boundary) under the real thin handler, so one request yields the four-node tree the demo renders. Note what the decorator does NOT do: it never touches arguments, results, or errors — instrumentation that changes behavior is a bug wearing a cape:"),
			code("obs-trace-decorator", "go",
				"One seam, spanned: the port decorator opens a span from the ambient context, delegates unchanged, ends the span.",
				"server/internal/domain/demos/obs_trace.go",
				"func (rec *ObsSpanningRepository) List(ctx context.Context, query posts.ListQuery) (posts.PostPage, error) {\n"+
					"	ctx, span := rec.recorder.Start(ctx, rec.symbol+\".List\", rec.layer)\n"+
					"	defer span.End()\n"+
					"	return rec.inner.List(ctx, query)\n"+
					"}"),
			demo("obs:trace", "The trace round-trip: your header in, a span tree out",
				"A W3C trace-context round-trip against the real traced pipeline: the demo request's OWN traceparent is accepted (minted and labeled honestly when you send none — curl the endpoint with your own header and watch the walk adopt it), then one real posts request runs through the production chain plus the trace middleware and spanning recorders — child spans recorded across middleware → thin handler → use case/port → adapter, rendered as a pre-order tree with the graft point (the root's parent is YOUR span id) and inclusive durations per node. Beside it, the hostile-header table: every malformed traceparent from the spec's edge cases parsed FOR REAL, each row carrying the parser's actual rejection reason and the never-fails-the-request policy that handled it."),
			text("obs-trace-sdk",
				"The honest SDK comparison, in full: the OpenTelemetry SDK adds a spec-complete parser and a maintained one (this subset passes the edge table but is not the whole ABNF, particularly tracestate's vendor grammar), real SAMPLERS (this implementation treats the sampled flag as a hint it records regardless — head sampling policies are where production tracing costs are controlled), the resource model (service.name, version, environment on every export), span attributes and events beyond name-and-layer, automated instrumentation for databases and HTTP clients you do not control, and EXPORTERS — OTLP over gRPC to a collector, which is how spans leave the process instead of rendering in a demo panel. What it does not change: the header on the wire, the context threading, the parent-pointer tree — the protocol you just watched work is the same one the SDK speaks, which is precisely why understanding it at this size is the right preparation for adopting the SDK when export volume justifies one."),
		),
		// ── 9. Alerting ────────────────────────────────────────────
		section("alerting", "Alerting: what pages a human",
			"Signals are for machines; alerts are the tiny, high-cost subset that wakes a person — and the discipline is keeping that subset tiny on purpose.",
			text("obs-alerting-what",
				"Every signal this topic has built — logs, metrics, traces, health, profiles — is consumed by machines first: pipelines index, scrapers store, dashboards draw. An ALERT is the one artifact aimed at a human, and its economics are brutal: a page at 3 a.m. costs an engineer's sleep tonight and a fraction of the team's trust every time it is wrong, so the bar is not ``interesting`` but ``a human must act NOW or something gets worse``. The reliable way over that bar is to alert on SYMPTOMS users feel, expressed as bounded queries over the metrics you already expose — and to let dashboards carry everything merely interesting. This stack has earned exactly a handful:"),
			list("obs-alerting-set",
				false,
				"Error rate — the fraction of non-5xx→5xx responses on guide_http_requests_total over a window: the one number that says users are being hurt right now (computed from the counter by status class — the metric this topic mounted).",
				"Queue depth — background-jobs topic's pending count past a threshold: work is arriving faster than it drains, and the retry ladder is filling (the topic's first pager number).",
				"Dead-letter count — jobs that exhausted their retries: work is being LOST, not delayed (the topic's second pager number; one dead job is a ticket, a stream of them is a page).",
				"Readiness flapping — /api/ready failing for more than a probe interval or two: the load balancer is shedding this instance and the dependency it names in details is the cause to chase.",
			),
			text("obs-alerting-fatigue",
				"ALERT FATIGUE is the failure mode every rule above is defending against, and it is a one-way door: a team paged weekly on non-actionable alerts stops reading pages, and the page that mattered is absorbed into the noise it deserved to outgrow. The classic causes are all variations of alerting on CAUSES instead of symptoms (CPU is high — users may not be; alert on latency and error rate, let CPU be a dashboard annotation that explains them), alerting on absolutes instead of bounded windows (spikes are normal; sustained shifts are news), and alerting on everything a system CAN report instead of everything a human must. The test this guide applies: for each alert, name the ACTION its page triggers — if the honest answer is ``look at it in the morning``, it is a dashboard, not a page. Correlated ids close the loop the other way: the page names the symptom's window and route, the traces in that window narrow it to a layer, the log lines joined by request id hand you the narrative — three signals, one system, one well-rested engineer."),
		),
		// ── 10. Process nets ───────────────────────────────────────
		section("process-nets", "Process nets: bounded, recovered, drained",
			"Observability assumes the process survives long enough to be observed — the nets that keep it alive were wired in the composition root from the start, and this topic is where they get named as observability's substrate.",
			text("obs-nets-why",
				"Every instrument this topic mounted rides on four nets the error-handling topic installed and the composition root has carried since the skeleton: the RECOVER net (programmer-bug panics become 500 envelopes — logged with stack and request id — never crashes), the TIMEOUT net (every read, write, and dial bounded, so a slow or hostile client cannot hold a goroutine forever), the BOOT net (config, database, migrations, and content validation all fail BEFORE the port opens — a broken process never serves one request it cannot serve honestly), and the SHUTDOWN net (SIGINT/SIGTERM drain in-flight work: gRPC first, HTTP inside a bounded window, the background-jobs worker last). The timeout net, quoted again here because it is the one observability itself depends on — a wedged request must release its goroutine for the metrics to stay honest and the pprof goroutine dump to stay readable:"),
			code("obs-nets-timeouts", "go",
				"The timeout net, unchanged since the skeleton: nothing waits forever, so the instruments stay readable.",
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
			text("obs-nets-observability",
				"Two of this topic's own additions are nets in the same sense. The readiness probe's BOUNDED PINGS are a net for the prober: a wedged dependency cannot wedge the probe that watches it — the check fails fast and names its bound. And the trace middleware's reject-to-fresh-root policy is a net for propagation: one malformed header from one caller degrades that request's tracing, not the request, not the process, not anyone else's trace. The general shape is worth naming because it recurs through this whole guide: a NET is a bound that turns a pathological case into an honest, observable failure — a timeout instead of a hang, an envelope instead of a crash, a fresh root instead of a corrupted trace. The topic's systems then make those honest failures VISIBLE: the timeout becomes a 5xx the error-rate alert can see, the recovered panic a logged stack with a request id, the degraded trace a debug line. Bounded first, then observable — that ordering is the whole reliability story in two words."),
		),
		// ── 11. Terminal companion ─────────────────────────────────
		section("terminal-companion", "Terminal companion: the observability tour",
			"The whole topic over curl: a log line dissected from the demo payload, health vs readiness vs metrics side by side, a real traceparent round-trip against the running server, and pprof's index reached through its dedicated prefix.",
			text("term-obs-intro",
				"With the stack up (server on :8100), the tour runs in the order the topic taught. First the viewer's payload — the same dissection the panel draws, read raw — pulling the anatomy line, the level matrix, and the replay's correlated ids out with python3, the tour's jq. Second the three GET-only probes every platform expects: liveness (always ok when the process lives), readiness (per-dependency rows, honestly 503 when something is down), and the metrics scrape (the exposition text itself, then the two families grep-ed out of it). Third the round-trip: mint a valid traceparent, send it to the REAL server (any route — propagation is middleware, it wraps everything), and read the response's echo — same trace id, the server's own span id; then send YOUR header to the trace demo and watch the tree adopt it, graft point first. Last, pprof through its fence-able prefix."),
			code("term-obs-logs-bash", "bash", "", "",
				"# the viewer: one real log line, dissected; the level matrix; the replay\n"+
					"curl -s localhost:8100/api/demo/obs/viewer | python3 -c \\\n"+
					"  'import json,sys; p=json.load(sys.stdin); print(p[\"anatomy\"][\"line\"]); \\\n"+
					"   [print(f[\"key\"], \"=\", f[\"value\"]) for f in p[\"anatomy\"][\"fields\"]]'\n"+
					"curl -s localhost:8100/api/demo/obs/viewer | python3 -c \\\n"+
					"  'import json,sys; p=json.load(sys.stdin); \\\n"+
					"   [print(r[\"level\"], \"survives at:\", \",\".join(r[\"survivesAt\"])) for r in p[\"levels\"][\"rows\"]]'\n"+
					"curl -s localhost:8100/api/demo/obs/viewer | python3 -c \\\n"+
					"  'import json,sys; p=json.load(sys.stdin); \\\n"+
					"   [print(l[\"source\"], l[\"level\"], l[\"requestId\"]) for l in p[\"replay\"][\"lines\"]]'\n"+
					"\n"+
					"# liveness vs readiness vs metrics — three probes, three jobs\n"+
					"curl -s localhost:8100/api/health\n"+
					"curl -s localhost:8100/api/ready | python3 -m json.tool\n"+
					"curl -s localhost:8100/metrics | head -8\n"+
					"curl -s localhost:8100/metrics | grep '^guide_http_requests_total'\n"+
					"curl -s localhost:8100/metrics | grep '_bucket' | head -4"),
			code("term-obs-trace-bash", "bash", "", "",
				"# the traceparent round-trip, against the REAL server: any route echoes it\n"+
					"TRACE='00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01'\n"+
					"curl -s -D- -o /dev/null -H \"traceparent: $TRACE\" localhost:8100/api/health \\\n"+
					"  | grep -i traceparent\n"+
					"#   → same trace id, the server's own span id — the graft point\n"+
					"\n"+
					"# your header into the trace demo: the tree adopts it\n"+
					"curl -s -H \"traceparent: $TRACE\" localhost:8100/api/demo/obs/trace | python3 -c \\\n"+
					"  'import json,sys; p=json.load(sys.stdin); a=p[\"accepted\"]; \\\n"+
					"   print(\"accepted trace:\", a[\"traceId\"], \"grafted on:\", a[\"parentId\"]); \\\n"+
					"   [print(\"  \"*s[\"depth\"] + s[\"name\"], str(s[\"durationMs\"])+\"ms\") for s in p[\"tree\"]]'\n"+
					"\n"+
					"# the hostile half: malformed headers, rejected honestly\n"+
					"curl -s localhost:8100/api/demo/obs/trace | python3 -c \\\n"+
					"  'import json,sys; p=json.load(sys.stdin); \\\n"+
					"   [print(m[\"header\"][:47]+\"…\", \"→\", m[\"problem\"][:60]) for m in p[\"malformed\"]]'\n"+
					"\n"+
					"# pprof behind its dedicated prefix — the runtime's own instruments\n"+
					"curl -s localhost:8100/internal/debug/pprof/ | head -12\n"+
					"curl -s 'localhost:8100/internal/debug/pprof/goroutine?debug=1' | head -6\n"+
					"curl -s localhost:8100/debug/pprof/ -o /dev/null -w '%{http_code}\\n'  # 404: not on the root"),
			text("term-obs-coda",
				"Read the tour as one continuous investigation, because that is the topic's argument. The viewer GET is the LOG half: a line you can read row by row, a gate you can recompute, an id you can join. The three probes are the PLATFORM half: liveness answering restart-me, readiness answering route-around-me with its dependencies named, and /metrics answering both questions as text any scraper speaks. The traceparent round-trip is the DISTRIBUTED half — your header accepted by the real middleware, echoed with the server's span, and expanded into a four-node tree whose root parent is the id you sent; the malformed table the parser's own errors, verbatim. And the pprof lines are the DEPTH half: the same prefix a production ingress would fence, serving the profiles that turn ``slow`` into a flame graph. Nothing in the tour needed a new dependency — a logger, a mux, a header, and a profiler: the observability stack was in the standard library all along, waiting to be wired."),
		),
	}

	if buildErr != nil {
		return Topic{}, buildErr
	}
	return NewTopic(
		"observability",
		"Observability in Go: logs, metrics, traces, and the probes between",
		"The three questions (what happened, where, why now) answered by "+
			"three signals run as one system on the server you are reading "+
			"— stdlib-only, by protocol. slog JSON logs (the logger this "+
			"server runs, one line dissected field by field; the level "+
			"dial's cost economics) and the request-logging middleware that "+
			"writes the ledger, proven live by the slog viewer demo (a real "+
			"replayed request's lines, the level matrix computed by really "+
			"gating each severity, and the request-id correlation joining a "+
			"404's warn to its access line and envelope). Health's two "+
			"halves: liveness (the dependency-free probe that must never "+
			"ping anything) vs readiness (the real /api/ready — PostgreSQL "+
			"and Redis pinged through domain-owned ports inside bounded "+
			"timeouts, honest 503s naming the down dependency). pprof in "+
			"production, mounted on a dedicated /internal/ prefix off the "+
			"public root. Prometheus metrics by hand: the text exposition "+
			"format rendered from a stdlib registry, request counters and "+
			"duration histograms whose labels are route PATTERNS — "+
			"cardinality bounded by the route table itself. W3C trace "+
			"context: traceparent parsed to the spec's own edge cases, "+
			"propagated by additive middleware, rejected honestly when "+
			"malformed, feeding a span-tree recorder — the trace demo's "+
			"round-trip (your header in, child spans across handler → use "+
			"case → adapter, the tree with its graft point) — with the "+
			"OpenTelemetry SDK compared honestly. Alerting's bar (what "+
			"pages a human: error rate, queue depth, dead letters, "+
			"readiness — and alert fatigue as the one-way door), and the "+
			"process nets (bounded timeouts, recovery, graceful drain) the "+
			"instruments stand on.",
		"platform",
		33,
		sections,
	)
}
