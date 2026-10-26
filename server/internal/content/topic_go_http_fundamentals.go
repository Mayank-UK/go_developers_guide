package content

import (
	"errors"
	"fmt"
)

// This file is topic 04 go-http-fundamentals, the framework group's first
// topic: the stdlib net/http server this very process is, from project
// setup through routing patterns, handler anatomy (with the topic's live
// demo INLINE), the middleware chain, the life of a request, error
// handling at the edge, an honest stdlib-vs-framework comparison, the
// pprof/expvar mounts, the dev workflow, and the terminal companion. Every
// must-source block quotes a real repo file; the ratchet verifies it.

// goHTTPFundamentalsTopic builds topic 04 — what the express guide teaches
// as "the framework", taught here as the standard library: net/http is the
// backbone, and chi/gin/echo appear only by comparison. The topic assumes
// topic 01's language tour and walks the exact code serving this API.
func goHTTPFundamentalsTopic() (Topic, error) {
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
		// ── 1. Setup ────────────────────────────────────────────────────
		section("setup", "Setup: one module, one binary",
			"Where other stacks start with a framework install, Go starts with one command and three lines of manifest.",
			text("setup-pitch",
				"Where the express guide begins with ``npm init`` and then installs a framework before the first route, a Go web project begins with ``go mod init guide`` and stops there. The command writes a ``go.mod`` file — the module manifest — and the module path it records (``guide`` here) becomes the root of every import path in the project: ``guide/internal/core``, ``guide/internal/content``, and so on. That manifest is the entire dependency story of this server, and it is three lines long, because the standard library IS the framework in this guide — a claim topic 01 made and this topic's demo proves live."),
			code("setup-gomod", "mod", "The whole manifest: a module path and a toolchain version. No dependencies.",
				"server/go.mod",
				"module guide\n"+
					"\n"+
					"go 1.27.3"),
			text("setup-layout",
				"The layout above the manifest follows the convention this repo uses everywhere: a ``cmd/`` directory holding one subdirectory per binary (just ``cmd/guide`` today), and the ``internal/`` tree holding everything else. ``internal`` is enforced privacy, as topic 01 covered — the go tool itself refuses imports of it from outside the module — which makes it the natural home for all server code: the module's public surface is exactly its one binary. Inside ``internal/``, five packages each own one concern, and the dependency rule between them is the architecture topic 32 teaches in full: presentation calls domain, data implements domain's interfaces, core serves them all."),
			list("setup-layers", false,
				"core — the cross-cutting plumbing: typed env config, the error taxonomy and envelope writer, the slog JSON logger, request ids, the scenario store.",
				"content — the typed teaching model this API serves: topics, sections, blocks, and the registry that validates them at startup.",
				"domain — the business shapes: entities, ports (interfaces), use cases, and the pure payload builders behind every live demo.",
				"data — repository implementations over real stores: pgx, redis, mongo, files, messaging — each behind a domain port.",
				"presentation — the HTTP edge: handlers, the middleware chain, and the wiring between the wire and the domain."),
			text("setup-config-intro",
				"Configuration is the first code a server runs, and the Go-idiomatic shape is deliberately boring: read environment variables once at boot, validate them, and hand a typed struct to everything that needs them. There is no config framework and no layered YAML — the twelve-factor environment is parsed by one function in one file, with names namespaced by a per-guide prefix so the family's stacks can co-run on one machine without colliding:"),
			code("setup-config", "go",
				"The env contract: names, dev-safe defaults, and a getenv function parameter (tests inject values; the process never mutates).",
				"server/internal/core/config.go",
				"const (\n"+
					"    envPort        = \"GDG_PORT\"\n"+
					"    envEnv         = \"GDG_ENV\"\n"+
					"    envLogLevel    = \"GDG_LOG_LEVEL\"\n"+
					")\n"+
					"// …\n"+
					"const (\n"+
					"    defaultPort     = 8100\n"+
					"    defaultEnv      = \"dev\"\n"+
					"    defaultLogLevel = slog.LevelInfo\n"+
					")\n"+
					"// …\n"+
					"func LoadConfig(getenv func(string) string) (Config, error) {\n"+
					"    cfg := Config{\n"+
					"        Port:        defaultPort,\n"+
					"        Env:         defaultEnv,\n"+
					"        LogLevel:    defaultLogLevel,\n"+
					"        GRPCAddr:    defaultGRPCAddr,\n"+
					"    }"),
			text("setup-config-values",
				"Three things in that snippet repay attention. Unset or empty variables fall back to dev-safe defaults, so the stack boots anywhere with zero configuration. Malformed values — a port that is not a number, a log level outside debug/info/warn/error — return a descriptive error naming the offending variable, and the composition root wraps it and exits non-zero: configuration problems fail at boot, never panic, never surface mid-request. And ``getenv`` is a function parameter rather than a direct ``os.Getenv`` call, which is what makes the loader testable without mutating process state — the same dependency-injection instinct the handlers use, applied to the environment itself. The resulting ``Config`` is read once and then immutable by convention; it is a small value struct, so handing it around copies are cheap and nothing mutates it."),
			text("setup-build",
				"Finally, the artifact. ``go build ./cmd/guide`` produces one static binary containing the server, the router, the content — everything. There is no ``node_modules`` equivalent, no virtualenv, no jar classpath: deployment is copying that file. ``go run ./cmd/guide`` compiles and runs in one step for the inner loop, fast enough that most Go projects do not bother with a watcher (the dev workflow section returns to this). The binary even carries its own birth certificate — the module, toolchain, and build-flag metadata that ``runtime/debug.ReadBuildInfo`` exposes — which is exactly what this topic's inline demo reads to let the server introduce itself. Cross-compiling is a flag pair, ``GOOS=linux GOARCH=arm64 go build``, and the topic 37 deployment story builds on it."),
		),
		// ── 2. Routing: ServeMux patterns ───────────────────────────────
		section("routing-patterns", "Routing: net/http and the ServeMux patterns",
			"Since Go 1.22 the standard router understands methods and wildcards — the feature gap that once justified a framework, closed.",
			text("rt-pitch",
				"``net/http`` is not a micro-framework; it is the whole server: the listener, TLS, connection management, timeouts, and the router. The router is ``http.ServeMux``, and since Go 1.22 its ``Handle`` method takes PATTERN strings that carry a method and path wildcards: ``GET /api/content/{topicId}``. Before 1.22 the stdlib mux matched only path prefixes, ignored methods entirely, and handed you the raw path to parse — which is why chi earned its place in so many stacks. The pattern language changed that, and the route table of this very server is now written entirely in it:"),
			code("rt-routes", "go",
				"The whole router: seven patterns against handlers that are plain http.Handler values.",
				"server/cmd/guide/main.go",
				"mux := http.NewServeMux()\n"+
					"mux.Handle(\"GET /api/health\", presentation.NewHealthHandler(cfg.Env))\n"+
					"mux.Handle(\"GET /api/content\", presentation.NewContentListHandler(registry))\n"+
					"mux.Handle(\"GET /api/content/{topicId}\", presentation.NewContentTopicHandler(registry, logger))\n"+
					"// …\n"+
					"mux.Handle(\"GET /api/demo/go/runtime-types\", presentation.NewRuntimeTypesHandler())"),
			text("rt-pattern-reading",
				"Read one line and you have read the contract: a method, a path shape, and a handler value. The method in the pattern is load-bearing — because ``GET /api/content`` is registered and nothing else is, a ``POST /api/content`` is rejected by the mux itself with a 405 status and an ``Allow`` header, before any handler code runs. No per-route method checks to write, no middleware to enforce verbs; the pattern is the contract and the router enforces it. The same table grows by exactly one line per demo as this guide's topics land, which is the whole point of a declarative route table: it reads like the API's table of contents."),
			list("rt-pattern-forms", false,
				"Exact match — ``GET /api/health``: method plus literal path; the most common form in this repo's table.",
				"Wildcard segment — ``GET /api/content/{topicId}``: one path element captured for the handler via ``r.PathValue``.",
				"Trailing-slash subtree — ``/assets/``: matches everything under the prefix (pre-1.22 behavior, still useful for mounts).",
				"Root catch-all — ``/``: matches anything otherwise unmatched; this repo deliberately registers none, so unknown paths 404."),
			text("rt-wildcard",
				"The captured segment reaches the handler as a string, read with ``r.PathValue(\"topicId\")`` — the name inside braces is the key. The content detail handler is the complete real example, and it is worth noticing how little it does: read the capture, look the topic up in the validated registry, and either write the payload or delegate to the central error writer with a typed not-found error. Parsing, matching, and method enforcement are all upstream of it:"),
			code("rt-pathvalue", "go",
				"From pattern to payload: PathValue reads the capture; the unknown case becomes the 404 envelope.",
				"server/internal/presentation/content.go",
				"func (h *ContentTopicHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {\n"+
					"    topicID := r.PathValue(\"topicId\")\n"+
					"    topic, ok := h.registry.TopicByID(topicID)\n"+
					"    if !ok {\n"+
					"        core.WriteError(w, r, core.Newf(core.KindNotFound, \"Topic %q not found\", topicID), h.logger)\n"+
					"        return\n"+
					"    }"),
			text("rt-precedence",
				"Matching precedence is most-specific-wins: a literal segment beats a wildcard at the same position, and a longer pattern beats a shorter one, so ``GET /api/content/{topicId}`` and a hypothetical ``GET /api/content/latest`` can coexist with the literal taking precedence for that one path. Two patterns that conflict exactly — the same method and path registered twice — panic at registration, which means a routing mistake fails the process at boot, loudly, instead of silently shadowing a route at request time. Unknown paths fall through to the mux's built-in 404; how that plain-text default becomes this API's error envelope is the middleware section's story. Topic 07 rebuilds this whole subject at production scale — sub-routers, mounting, and the 404/405 semantics under load — on exactly these primitives."),
		),
		// ── 3. Handler anatomy + inline demo ────────────────────────────
		section("handler-anatomy", "The anatomy of a handler",
			"One interface, one struct, one constructor — and this topic's live demo, built from exactly those parts.",
			text("an-pitch",
				"Every route above ends in a handler, and every handler in Go — from the health probe to the most complex API endpoint — is the same one-method interface: ``http.Handler`` with ``ServeHTTP(w http.ResponseWriter, r *http.Request)``. The ``ResponseWriter`` is where output goes: set headers, write the status once, then write the body through it as an ``io.Writer``. The ``*Request`` is where input already is: method, path, query, headers, body stream, and the request's context for cancellation. There is no base class, no lifecycle hooks, no middleware algebra — a handler is a value with a method, and that austerity is the anatomy."),
			text("an-struct",
				"The idiomatic shape puts state in a small struct, dependencies in a constructor, and the interface method after them. The health handler is the complete pattern in miniature: it holds one dependency (the environment name it reports), its ``New`` constructor injects it, and ``ServeHTTP`` writes the JSON. A handler with no dependencies would skip the struct entirely and adapt a plain function with ``http.HandlerFunc``; a handler with five dependencies is the same shape with five fields — the pattern does not grow new concepts:"),
			code("an-health", "go",
				"A whole handler: struct, constructor, interface method. This one is dependency-injected with its env.",
				"server/internal/presentation/health.go",
				"type HealthHandler struct {\n"+
					"    env string\n"+
					"}\n"+
					"// …\n"+
					"func NewHealthHandler(env string) *HealthHandler {\n"+
					"    return &HealthHandler{env: env}\n"+
					"}\n"+
					"// …\n"+
					"func (h *HealthHandler) ServeHTTP(w http.ResponseWriter, _ *http.Request) {\n"+
					"    w.Header().Set(\"Content-Type\", \"application/json\")\n"+
					"    // …\n"+
					"    _ = json.NewEncoder(w).Encode(healthResponse{Status: \"ok\", Env: h.env})\n"+
					"}"),
			text("an-writejson-intro",
				"The success path condenses into one shared helper: set the content type, write the status, encode the value. Every handler in this repo writes success through ``writeJSON`` — one place to get the header right, one signature that accepts any response model through ``any``. The deliberately swallowed ``Encode`` error is the honest edge case: if encoding fails, the peer vanished mid-response, the status line has already gone out, and there is nothing left to do but let the connection die:"),
			code("an-writejson", "go",
				"The shared success writer every handler calls.",
				"server/internal/presentation/content.go",
				"func writeJSON(w http.ResponseWriter, status int, v any) {\n"+
					"    w.Header().Set(\"Content-Type\", \"application/json\")\n"+
					"    w.WriteHeader(status)\n"+
					"    // An Encode failure means the peer vanished mid-write; there is no\n"+
					"    // recovery action, and the status line already went out.\n"+
					"    _ = json.NewEncoder(w).Encode(v)\n"+
					"}"),
			text("an-demo-intro",
				"Where the express fundamentals topic introspects its framework object, Go has no framework object to introspect — so this topic's demo introspects the next most honest thing: the running binary itself. ``runtime/debug.ReadBuildInfo`` reports the module this server was compiled from, its toolchain, its dependency list, and the build flags stamped into it; the ``runtime`` package reports the machine facts the scheduler sees; and the one builder parameter carries the environment config the process booted with. The payload builder stays pure — observed facts in, payload out — which is why the handler can be three lines and the builder testable without any HTTP at all:"),
			code("an-demo-builder", "go",
				"The demo's payload builder: ReadBuildInfo + runtime facts + booted config, shaped pure.",
				"server/internal/domain/demos/app_info.go",
				"type ModuleInfo struct {\n"+
					"    Path            string `json:\"path\"`\n"+
					"    Version         string `json:\"version\"`\n"+
					"    GoVersion       string `json:\"goVersion\"`\n"+
					"    DependencyCount int    `json:\"dependencyCount\"`\n"+
					"}\n"+
					"// …\n"+
					"func BuildAppInfoPayload(cfg core.Config) AppInfoPayload {\n"+
					"    buildInfo, _ := debug.ReadBuildInfo()\n"+
					"    return buildAppInfoPayload(buildInfo, runtime.NumCPU(), runtime.GOMAXPROCS(0), cfg)\n"+
					"}"),
			code("an-demo-handler", "go",
				"The demo endpoint: the same anatomy as the health handler, one config dependency deep.",
				"server/internal/presentation/demo_go_http.go",
				"type AppInfoHandler struct {\n"+
					"    config core.Config\n"+
					"}\n"+
					"// …\n"+
					"func NewAppInfoHandler(config core.Config) *AppInfoHandler {\n"+
					"    return &AppInfoHandler{config: config}\n"+
					"}\n"+
					"// …\n"+
					"func (h *AppInfoHandler) ServeHTTP(w http.ResponseWriter, _ *http.Request) {\n"+
					"    writeJSON(w, http.StatusOK, demos.BuildAppInfoPayload(h.config))\n"+
					"}"),
			demo("gohttp:app-info", "The running server, introduced by itself",
				"Computed live from this very process: the module and toolchain the binary was built from, "+
					"its dependency count (the manifest's deliberate direct requires, pinned against the live "+
					"build info), the CPU "+
					"facts the scheduler sees (NumCPU, GOMAXPROCS), the port/env/log-level the process "+
					"actually booted with, and the toolchain's build settings sorted by key — honest under "+
					"go run, go test, and a release build alike."),
			text("an-beats-intro",
				"The payload reads in four beats:"),
			list("an-beats", false,
				"Module — path, version, toolchain, dependency count: the go.mod facts as the running binary holds them; ``(devel)`` until a VCS-stamped release build says otherwise.",
				"Runtime — NumCPU and GOMAXPROCS, the machine as the scheduler sees it; equal here, and topic 02 covers when they are not.",
				"Environment — port, env, and log level the process actually holds: the LoadConfig result the handler injected at wiring time.",
				"Build settings — the toolchain flags stamped at build (build mode, compiler, CGO, GOARCH/GOOS), sorted by key so the shape is deterministic."),
			text("an-coda",
				"What the demo quietly proves is the point of this whole section: no framework object existed to inspect, so the handler anatomy and a pure builder composed into a real introspection endpoint — registered by one pattern line, tested without a socket, serving camelCase JSON through the shared writer. The demo's builder and handler are quoted above as real repo files, and the dev workflow section later shows the test that pins them. That is the stdlib bet: the pieces are small, but they are ALL you need."),
		),
		// ── 4. Middleware chain ─────────────────────────────────────────
		section("middleware-chain", "The middleware chain",
			"Cross-cutting behavior as nested handlers: one function type, composed by function application.",
			text("mw-pitch",
				"Request ids, access logging, panic recovery, error-envelope shaping — none of these belong in any one handler, and all of them belong on every request. The Go answer is the middleware convention, and it is not a framework feature but a function type: a middleware takes the next handler and returns a wrapper around it. Because both ends are ``http.Handler``, middleware compose by plain function application — the onion model — and any ``net/http``-shaped middleware from any library fits the same signature. This repo's own type is one alias:"),
			code("mw-type", "go",
				"The entire middleware abstraction: one function type, and Chain folding it outermost-first.",
				"server/internal/presentation/middleware.go",
				"type Middleware func(http.Handler) http.Handler\n"+
					"// …\n"+
					"func Chain(middleware ...Middleware) Middleware {\n"+
					"    return func(final http.Handler) http.Handler {\n"+
					"        for i := len(middleware) - 1; i >= 0; i-- {\n"+
					"            final = middleware[i](final)\n"+
					"        }\n"+
					"        return final\n"+
					"    }\n"+
					"}"),
			text("mw-why-functions",
				"``Chain`` composes so the FIRST argument is the OUTERMOST layer: it wraps from the inside out, folding the slice backwards so ``Chain(A, B, C)(handler)`` serves as ``A(B(C(handler)))``. Notice what makes this testable — a middleware is just a function from handler to handler, so ``httptest`` alone exercises any layer with a stub inner handler, no server required. And because the convention is a plain function type rather than an interface hierarchy, this repo's middleware and a third-party ``net/http`` middleware are interchangeable values; that portability is why the Go ecosystem standardized here rather than per-framework."),
			code("mw-compose", "go",
				"The composition root wires the documented order — one expression, the whole request pipeline.",
				"server/cmd/guide/main.go",
				"handler := presentation.Chain(\n"+
					"    presentation.RequestID,\n"+
					"    presentation.RequestLogger(logger),\n"+
					"    presentation.Recover(logger),\n"+
					"    presentation.EnvelopeMuxErrors(logger),\n"+
					")(mux)"),
			text("mw-order",
				"The order is a decision, and each position earns its place. Request-id is outermost so every layer beneath it — logging, recovery, every error envelope — can read and stamp the same id. The access logger sits OUTSIDE recovery so a recovered panic still logs as a finished request with its true 500 status. Recovery converts panics into the 500 envelope through the central writer, re-raising only ``http.ErrAbortHandler`` (net/http's own \"the connection is gone\" sentinel). And the mux-error wrapper is innermost, hugging the router so even the mux's own plain-text 404/405 replies can be rewritten into the envelope. Swap any two positions and semantics change: request-id inside the logger would log id-less lines; recovery outside the logger would hide crash durations from the access log."),
			text("mw-recorder-intro",
				"One structural wrinkle: to log the response status, the logging middleware must observe what the inner stack writes — and the handler API passes an ``http.ResponseWriter``, not an observable. The classic solution wraps the writer in a recording struct via embedding (the topic 01 mechanic): the wrapper IS a ``ResponseWriter``, every method promoted to the real one underneath, with ``WriteHeader`` intercepting the status exactly once. This repo's recorder also forwards ``Flush`` and ``Unwrap`` so ``http.ResponseController`` and streaming keep working through the chain:"),
			code("mw-recorder-code", "go",
				"Recording the status: embedding turns a wrapper into a full ResponseWriter.",
				"server/internal/presentation/middleware.go",
				"func (r *statusRecorder) WriteHeader(code int) {\n"+
					"    if !r.wrote {\n"+
					"        r.status = code\n"+
					"        r.wrote = true\n"+
					"    }\n"+
					"    r.ResponseWriter.WriteHeader(code)\n"+
					"}"),
			text("mw-requestid",
				"The request-id middleware shows a full layer end to end. It takes the inbound ``x-request-id`` header when the client supplies one (trimmed, length-capped so hostile headers cannot bloat logs) and mints a fresh id otherwise. It echoes the id on the response headers BEFORE the inner stack runs — so even a mid-response failure keeps the header — and propagates it through the request context, the request-scoped key-value channel that handlers, logging, and error envelopes all read. Context propagation is how outer layers communicate inward without touching handler signatures, and topic 06 makes contexts first-class:"),
			code("mw-requestid-code", "go",
				"Take or mint, echo on the response, propagate by context — one layer, complete.",
				"server/internal/presentation/middleware.go",
				"id := strings.TrimSpace(r.Header.Get(core.RequestIDHeader))\n"+
					"if id == \"\" {\n"+
					"    id = core.NewRequestID()\n"+
					"}\n"+
					"// …\n"+
					"w.Header().Set(core.RequestIDHeader, id)\n"+
					"next.ServeHTTP(w, r.WithContext(core.WithRequestID(r.Context(), id)))"),
		),
		// ── 5. Life of a request ────────────────────────────────────────
		section("life-of-a-request", "The life of a request",
			"From the kernel's accept() to the access log line — one request's full journey through this server.",
			text("life-arrival",
				"Follow one request. The kernel accepts a connection on :8100 and hands it to ``http.Server``, which serves every CONNECTION on its own goroutine — not one goroutine per request, though with keep-alive each connection usually carries one request at a time anyway. That is the entire concurrency model at this layer: a handler runs as a plain, blocking function on its goroutine, reads the request, writes the response, returns. There is no event loop to starve and no callback inversion; when the handler blocks on I/O, the runtime parks the goroutine and schedules others — topic 02's scheduler story, working for the web server for free. Ten thousand waiting requests are routine."),
			text("life-server-intro",
				"Before any request arrives, the server struct itself has already made operational decisions. ``http.Server`` carries the timeouts that bound hostile or broken peers: ``ReadHeaderTimeout`` defeats slowloris-style header dribbling, the read and write timeouts bound each request's stalls, and ``IdleTimeout`` reaps keep-alive connections. The zero values — the naive ``&http.Server{}`` — mean NO limit, which net/http's own documentation warns about; setting them once at the composition root is the whole defense:"),
			code("life-server-code", "go",
				"Operational policy as struct fields: the timeouts every production server must set.",
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
			text("life-through-stack",
				"Now the request itself. It descends the middleware onion in wiring order: request-id stamps and propagates; the logger installs its status recorder and arms its deferred log line; recovery arms its panic net; the envelope interceptor takes position around the router. The mux matches the pattern, and the handler runs — decode input if any, call the domain, write the response through the shared writer. Then the unwinding: the interceptor rewrites (or replays) any 404/405, recovery's defer finds nothing to catch on a good path, and the logger's defer finally fires, one structured line carrying method, path, the TRUE status the recorder observed, duration in milliseconds, and the request id:"),
			code("life-log-code", "go",
				"The deferred access log: one structured line per request, written on the way out.",
				"server/internal/presentation/middleware.go",
				"rec := newStatusRecorder(w)\n"+
					"started := time.Now()\n"+
					"defer func() {\n"+
					"    durationMs := float64(time.Since(started).Microseconds()) / 1000\n"+
					"    logger.Info(\"http_request\",\n"+
					"        \"method\", r.Method,\n"+
					"        \"path\", r.URL.Path,\n"+
					"        \"status\", rec.status,\n"+
					"        // …\n"+
					"        \"request_id\", core.RequestIDFromContext(r.Context()),\n"+
					"    )\n"+
					"}()\n"+
					"next.ServeHTTP(rec, r)"),
			text("life-shutdown",
				"The last journey is the process's own. ``signal.NotifyContext`` turns SIGINT/SIGTERM into context cancellation; the select that was blocking on the serve channel notices; and ``srv.Shutdown`` takes over — it stops accepting new connections, waits for in-flight handlers to finish up to a bounded timeout (ten seconds here), and returns an error if stragglers overstay. A second Ctrl-C force-exits instead of waiting again, because ``stop()`` restores default signal behavior first. Graceful shutdown with a deadline is a handful of stdlib lines — no process manager required — and topic 37's deployment story reuses this exact block behind containers and orchestrators:"),
			code("life-shutdown-code", "go",
				"Signal to context, context to graceful shutdown, bounded by a timeout.",
				"server/cmd/guide/main.go",
				"ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)\n"+
					"defer stop()\n"+
					"// …\n"+
					"shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)\n"+
					"defer cancel()\n"+
					"if err := srv.Shutdown(shutdownCtx); err != nil {\n"+
					"    return fmt.Errorf(\"graceful shutdown: %w\", err)\n"+
					"}"),
		),
		// ── 6. Error handling first steps ───────────────────────────────
		section("error-first-steps", "Error handling: first steps",
			"Handlers return nothing — so failures must be WRITTEN, and written in exactly one shape.",
			text("err-pitch",
				"Go handlers have no return value for errors; an unhandled error in a handler is simply silence, the worst possible outcome on the HTTP edge. The discipline this repo builds (and topic 30 completes) starts with three moves: failures travel as typed ``core.Error`` values with a Kind — the errors-as-values machinery from topic 01; ONE central writer maps any error onto the wire; and every failure response shares one envelope shape, so a client can parse errors as uniformly as successes. The envelope's body type is small enough to read in full:"),
			code("err-envelope-code", "go",
				"The one error body every failure serves: code, message, requestId, optional details.",
				"server/internal/core/envelope.go",
				"type errorBody struct {\n"+
					"    Code      string `json:\"code\"`\n"+
					"    Message   string `json:\"message\"`\n"+
					"    RequestID string `json:\"requestId\"`\n"+
					"    Details   any    `json:\"details,omitempty\"`\n"+
					"}"),
			text("err-envelope-shape",
				"On the wire that renders as ``{error: {code, message, requestId}}`` with ``details`` omitted — never null — when there is nothing structured to say. The code is a small taxonomy string (``not_found``, ``validation_failed``, ``rate_limited``...), not a bare number, so clients switch on meaning rather than mapping numerals; the request id matches the ``x-request-id`` header and the server's own logs, which is what makes a user-reported failure findable. The central writer decides all of this once, so no handler ever hand-builds an error response:"),
			code("err-handler-code", "go",
				"The central writer: unwrap, map kind to status, log with the request id, write the envelope.",
				"server/internal/core/envelope.go",
				"func WriteError(w http.ResponseWriter, r *http.Request, err error, logger *slog.Logger) {\n"+
					"    requestID := RequestIDFromContext(r.Context())\n"+
					"\n"+
					"    var appErr *Error\n"+
					"    if errors.As(err, &appErr) && knownKinds[appErr.Kind] {\n"+
					"        status := HTTPStatusFor(appErr.Kind)"),
			text("err-mux",
				"One failure source sits BELOW every handler: the router itself. An unmatched path makes the stdlib mux write a plain-text 404; a wrong method writes a plain-text 405 — both shapes nothing like the envelope. The innermost middleware solves exactly this: it buffers only 404/405 responses, compares the body against the mux's known canned strings, and rewrites matches through the same central writer — while a 404 a HANDLER wrote (like the content detail route's unknown-topic case) passes through untouched, because its body is the envelope already. The 405 keeps its truthful status and ``Allow`` header and takes the closest taxonomy bucket:"),
			code("err-mux-code", "go",
				"The router's canned failures become the same envelope — handler-written ones pass through.",
				"server/internal/presentation/middleware.go",
				"if i.status == http.StatusNotFound {\n"+
					"    core.WriteError(i.w, r, core.NotFound(\"Not Found\"), logger)\n"+
					"    return\n"+
					"}\n"+
					"core.WriteErrorStatus(i.w, r, i.status, core.New(core.KindForHTTPStatus(i.status), \"Method Not Allowed\"), logger)"),
			text("err-coda",
				"What is deliberately NOT here yet: validation errors with structured ``details`` arrays land with topic 10, failure injection and the full taxonomy walk with topic 30, and rate limiting's 429 with topic 25. First steps means exactly the invariants above — nothing swallowed, every failure in one parseable shape, a request id on every response — and those invariants are already enough to keep this API debuggable from a browser's network tab alone."),
		),
		// ── 7. stdlib vs frameworks ─────────────────────────────────────
		section("stdlib-vs-frameworks", "What the stdlib leaves out",
			"chi, gin, echo — where a framework genuinely earns its keep, and why this guide still bets on net/http.",
			text("fw-pitch",
				"An honest inventory first. ``net/http`` gives you: the HTTP/1.1 and HTTP/2 server, method-and-wildcard routing, the middleware convention, TLS, timeouts, graceful shutdown, ``httptest`` for handler tests, and pprof/expvar mounts — everything the previous five sections used. It does NOT give you: route groups or sub-routers, request binding and validation, response rendering helpers beyond do-it-yourself JSON, any ready-made middleware ecosystem, or scaffolding generators. Those gaps are real, and pretending otherwise is how stdlib projects grow their own half-tested frameworks by accident. The honest question is which gaps bite a given project:"),
			list("fw-gaps", false,
				"Route groups and mounting — sub-routers under a prefix with shared middleware; stdlib does it only by hand (topic 07 builds it explicitly).",
				"Request binding and validation — decoding JSON into structs with schema rules and helpful 422s; stdlib stops at json.Decoder (topic 10).",
				"Response conventions — renderers, content negotiation, consistent error pages; stdlib hands you the writer and the choice (topic 09).",
				"Ecosystem middleware — CORS, sessions, rate limiting, JWT as ready-made layers; on stdlib each is a deliberate build or import (topics 08, 23, 25).",
				"Scaffolding and watch tooling — generators and hot reload; stdlib's story is gofmt, go run, and a Makefile (topic 36's air)."),
			text("fw-chi",
				"``chi`` is the smallest step away from stdlib — by design it IS net/http: your handlers stay ``http.Handler``, your middleware stays ``func(http.Handler) http.Handler``, and chi adds a router with groups (``r.Route(\"/api\", ...)``), mounting, and middleware scoping. Migrating between chi and the stdlib mux is mostly mechanical because the types never changed; a team that outgrows raw ``ServeMux`` organization loses nothing by adopting it. Topic 07's routing deep-dive does exactly that comparison on this repo's own route table."),
			text("fw-gin-echo",
				"``gin`` and ``echo`` make the opposite trade: their own context type — ``gin.Context`` wrapping the request, response writer, params, binding, and rendering in one value — which buys fast, ergonomic handlers (``c.ShouldBindJSON(&req)``) at the price of leaving the ``net/http`` handler shape. Handlers written against gin.Context are not portable http.Handlers; middleware, test tooling, and ecosystem lock to the framework. Their performance reputations were earned in a pre-1.22 world and have mostly converged — for most services the deciding factor is ergonomics and ecosystem, not throughput, and both ecosystems are large and well-maintained."),
			text("fw-stance",
				"This guide's stance, stated once and kept: the stdlib is the backbone, and frameworks are taught by comparison at the moment they earn their keep — chi in topic 07, validation libraries in 10, the full third-party decision framework in 42. The reasons are structural, not ascetic: every Go framework is built ON net/http, so the concepts here transfer down to all of them; the stdlib's types are the industry's lingua franca for middleware and handlers; and the 1.22 pattern router closed the gap that originally justified most router adoptions. Choose a framework when a gap in the list above actually bites — and know exactly which one you are paying for."),
		),
		// ── 8. pprof / expvar ───────────────────────────────────────────
		section("pprof-expvar", "pprof and expvar mounts",
			"The profiler and the var registry ship with the toolchain — observability before any import.",
			text("pp-intro",
				"Two standard-library packages exist purely to be mounted: ``net/http/pprof`` and ``expvar``. Both register themselves through an import side effect — the blank import ``import _ \"net/http/pprof\"`` attaches profiling handlers to the DEFAULT mux, and expvar does the same with one JSON endpoint. The pattern is unusual for a stdlib that otherwise avoids magic, but it means any Go server can expose production-grade profiling with zero third-party code, which is a genuinely differentiator: the same profiler that debugs a Kubernetes pod debugs your laptop."),
			text("pp-pprof",
				"pprof's handlers live under ``/debug/pprof/``: a CPU profile sampled over a window (``/debug/pprof/profile?seconds=5``), heap snapshots, goroutine dumps with stack traces, block and mutex profiles, and an execution trace. The consumption side is the ``go tool pprof`` command, which reads a live URL or a saved file and answers where the time or the allocations actually went — the flame graphs and top lists topic 35's debugging chapter is built on. This guide mounts pprof explicitly with topic 33's observability work; the commands below are the shape of the tour as it lands, runnable against any Go server that exposes them:"),
			code("pp-commands", "bash", "", "",
				"# once the pprof mount lands (topic 33), against this very server:\n"+
					"go tool pprof http://localhost:8100/debug/pprof/profile?seconds=5\n"+
					"go tool pprof http://localhost:8100/debug/pprof/heap"),
			text("pp-expvar",
				"``expvar`` is the smaller sibling: a typed global registry exposed as JSON at ``/debug/vars``. Any package can publish a counter or gauge — ``expvar.Int`` for request counts, ``expvar.Func`` for computed snapshots — and the runtime's own memory statistics ship in the same document by default. It is deliberately primitive (no metrics formats, no aggregation), which is why production setups usually front it with Prometheus — topic 33's move — but for \"what does the process know about itself right now\" it is one import away."),
			text("pp-honest",
				"Two honest caveats close the section. First, the blank-import trick attaches to ``http.DefaultServeMux``, while this server builds its own mux explicitly — the observability topic shows the explicit pattern registration that keeps routing, middleware, and profiling in ONE table. Second, profiling endpoints are an information leak if exposed publicly: they disclose internals and cost real CPU. The standard discipline is mounting them on an internal port or guarding them — a decision topic 33 makes deliberately, not a default to inherit silently."),
		),
		// ── 9. Dev workflow ─────────────────────────────────────────────
		section("dev-workflow", "The dev workflow",
			"run, test, vet, fmt — the loop this repo actually uses, and where air would fit.",
			text("dev-loop",
				"The inner loop is two commands. ``go run ./cmd/guide`` compiles and starts the server — fast enough, routinely under a second for a module this size, that most Go developers restart rather than watch. ``go build ./cmd/guide`` produces the deployable binary when you want it. There is no transpile step, no bundler, no watchers required: edit, save, re-run. Two hygiene tools ride along in every loop: ``gofmt`` formats source canonically (there is exactly one way Go code looks, which ends style debates in code review), and ``go vet`` flags the suspect constructs the compiler will not — printf verb mismatches, unreachable code, shadowed returns."),
			text("dev-verify",
				"Verification is the full pipeline: ``go build ./...``, ``go vet ./...``, and ``go test -race ./...`` — the race detector is a first-class part of the standard invocation in this repo, not an occasional deep check. The family rule this guide keeps: every demo endpoint ships with a handler test through the real middleware chain, and every payload claim the prose makes is pinned by a builder test next to the code. The test guarding THIS topic's demo is a real example — it pins the module facts against the live ``ReadBuildInfo``, including the dependency count against the live manifest:"),
			code("dev-tests-code", "go",
				"The test behind the demo: prose claims, pinned as executable facts.",
				"server/internal/domain/demos/app_info_test.go",
				"func TestAppInfoModuleFactsMatchTheLiveBuildInfo(t *testing.T) {\n"+
					"    payload := BuildAppInfoPayload(appInfoTestConfig())\n"+
					"    buildInfo, ok := debug.ReadBuildInfo()\n"+
					"    if !ok {\n"+
					"        t.Fatal(\"a go-built test binary always carries build info\")\n"+
					"    }\n"+
					"// …\n"+
					"    if payload.Module.DependencyCount != len(buildInfo.Deps) || payload.Module.DependencyCount != 12 {\n"+
					"        t.Errorf(\"dependency count = %d, want 12 (the deliberate-direct-requires count, confirmed live)\", payload.Module.DependencyCount)\n"+
					"    }"),
			text("dev-devcontainer",
				"This repo's environment is a devcontainer stack: compose keeps the server on :8100 beside PostgreSQL, Redis, and MongoDB; the client's Vite dev server on :5175 proxies ``/api`` to it. Verification runs inside the long-lived containers — no ephemeral churn — with named volumes caching the Go module and build caches so an exec'd ``go test`` is warm. The docker-exec command pair is the project's definition of done, and it is exactly what produced the green output behind this page: the stack you are reading this from is the stack that tested it."),
			text("dev-air",
				"Hot reload exists — ``air`` watches sources and rebuilds+restarts on save, configured by an ``.air.toml`` — and many Go teams adopt it for handler-heavy work. The honest trade: Go compiles so fast that ``go run`` plus restart is often already the shortest path, and a watcher adds a moving part to the loop (missed events, stale processes on crash). This guide runs plain ``go run`` in the container and evaluates air properly in topic 36's environments chapter; until then the fastest reload in the family is still just Go."),
		),
		// ── 10. Terminal companion ──────────────────────────────────────
		section("terminal-companion", "Terminal companion: the curl tour",
			"This topic's own surface, walked from the shell.",
			text("term-pitch",
				"With the stack up (server on :8100), these commands exercise exactly what this topic taught: the route table's patterns, the app-info demo's payload, the request-id header every response carries, and the two router failures the error envelope absorbs. ``python3 -m json.tool`` does the pretty-printing — the tour's only dependency is something every machine already has:"),
			code("term-index", "bash", "", "",
				"# the curriculum index this router serves\n"+
					"curl -s localhost:8100/api/content | python3 -m json.tool"),
			code("term-topic", "bash", "", "",
				"# this topic, in full: sections -> text/code/list/demo blocks\n"+
					"curl -s localhost:8100/api/content/go-http-fundamentals | python3 -m json.tool"),
			code("term-demo", "bash", "", "",
				"# the demo: the running server introducing itself (ReadBuildInfo + runtime + booted config)\n"+
					"curl -s localhost:8100/api/demo/gohttp/app-info | python3 -m json.tool"),
			code("term-headers", "bash", "", "",
				"# headers on: every response carries x-request-id, echoed from the outermost middleware\n"+
					"curl -s -i localhost:8100/api/health"),
			code("term-404", "bash", "", "",
				"# unknown path -> the mux's 404, rewritten into the error envelope\n"+
					"curl -s -i localhost:8100/api/content/nope"),
			code("term-405", "bash", "", "",
				"# wrong method -> truthful 405 + Allow header, same envelope, from the pattern itself\n"+
					"curl -s -i -X POST localhost:8100/api/content"),
			text("term-coda",
				"Provoke both failures and read the bodies side by side: the same ``{error: {code, message, requestId}}`` shape, the same header, one from the router and one from a handler — the middleware chain's work, visible from the shell. Match a response's request id against the server's log stream and you have this topic's whole story in one command. Next, topic 05 widens the lens from the HTTP edge to the type system's real center: interfaces and composition."),
		),
	}

	if buildErr != nil {
		return Topic{}, buildErr
	}
	return NewTopic(
		"go-http-fundamentals",
		"HTTP fundamentals with net/http",
		"The stdlib server this API is — setup, routing patterns, handler anatomy, "+
			"the middleware chain, and an honest look at where chi, gin, and echo earn their keep.",
		"framework",
		4,
		sections,
	)
}
