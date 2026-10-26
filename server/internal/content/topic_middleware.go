package content

import (
	"errors"
	"fmt"
)

// This file is topic 08 middleware (group "request-handling", order 8),
// the canon family's filters-and-interceptors topic adapted to Go's
// answer: there IS no middleware concept in the standard library — only a
// function signature the ecosystem agreed on, ``func(http.Handler)
// http.Handler``, and the discipline to compose it deliberately. The
// topic's centerpiece is THIS REPO'S OWN CHAIN: every load-bearing code
// block quotes the real wiring in server/internal/presentation/middleware.go
// and server/cmd/guide/main.go, and the live demo performs a real
// teaching-grade chain from internal/domain/demos/middleware_pipeline.go.
// chi is taught by comparison only — no dependency, and the go.mod block
// proves it.

// middlewareTopic builds topic 08 — the middleware chain and its ordering
// (with the pipeline demo INLINE), the request-id and logging pattern,
// panic recovery, http.TimeoutHandler and the stdlib's limits, and the
// hand-rolled-versus-chi decision, closing with the terminal companion.
func middlewareTopic() (Topic, error) {
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
		// ── 1. The chain and its ordering ─────────────────────────────
		section("the-chain-and-its-ordering", "The middleware chain and its ordering",
			"Go has no middleware concept — it has one function signature, and the order you compose it in IS your architecture.",
			text("chain-no-concept",
				"Other frameworks ship middleware as a first-class concept: Express has ``app.use``, Spring has "+
					"``HandlerInterceptor`` and servlet filters, FastAPI stacks its ASGI middleware. Go's "+
					"``net/http`` ships none of that — and does not need to, because it already has the one "+
					"thing those mechanisms reduce to: an interface with a single method, ``http.Handler``, "+
					"and first-class functions. A middleware in Go is just a function that takes the next "+
					"handler and returns a handler that wraps it — code that runs before it calls down, the "+
					"call, and code that runs after the inner stack returns. That signature is the ecosystem's "+
					"universal convention: this server's own type is one line, and every third-party router "+
					"worth using — chi above all — accepts exactly this shape, which is why hand-rolled "+
					"middleware and ecosystem middleware interoperate freely."),
			code("chain-type", "go",
				"The whole idea, one type and one combinator — the server's real definitions.",
				"server/internal/presentation/middleware.go",
				"// Middleware wraps one handler with cross-cutting behavior.\n"+
					"type Middleware func(http.Handler) http.Handler\n"+
					"\n"+
					"// Chain composes middleware so the FIRST argument is the OUTERMOST layer:\n"+
					"// Chain(A, B, C)(handler) serves as A(B(C(handler))). The composition root\n"+
					"// wires the documented order:\n"+
					"//\n"+
					"//\tRequestID(RequestLogger(Recover(EnvelopeMuxErrors(mux))))\n"+
					"//\n"+
					"// Request-id outermost so the logging and recovery layers (and every error\n"+
					"// envelope they write) can stamp and log the same id. The request logger\n"+
					"// sits OUTSIDE recovery so a recovered panic is still logged as a finished\n"+
					"// request with its true 500 status. Recovery converts panics to the 500\n"+
					"// envelope via the central writer; the mux-error wrapper innermost turns\n"+
					"// the stdlib ServeMux's plain-text 404/405 replies into the same envelope.\n"+
					"func Chain(middleware ...Middleware) Middleware {\n"+
					"\treturn func(final http.Handler) http.Handler {\n"+
					"\t\tfor i := len(middleware) - 1; i >= 0; i-- {\n"+
					"\t\t\tfinal = middleware[i](final)\n"+
					"\t\t}\n"+
					"\t\treturn final\n"+
					"\t}\n"+
					"}"),
			text("chain-fold",
				"Read ``Chain`` as pure function composition. ``Chain(A, B, C)(handler)`` builds "+
					"``A(B(C(handler)))`` by folding from the END of the list: wrap the terminal handler "+
					"with C, wrap that with B, wrap that with A — so the FIRST argument ends up the "+
					"OUTERMOST layer, the first code to run on the way in and the last on the way out. "+
					"The consequence is the single most important sentence in this topic: WRAP ORDER IS "+
					"REQUEST ORDER, AND RESPONSE ORDER IS ITS REVERSE. Each layer's before-code runs "+
					"outermost-first, exactly one handler runs in the middle, and each layer's after-code "+
					"— everything below ``next.ServeHTTP(w, r)`` — runs innermost-first as the call stack "+
					"unwinds. If a layer panics or an inner layer already wrote the response, the "+
					"outer layers' after-code still runs; that is what makes recovery and completion "+
					"logging possible at all."),
			text("chain-real-wiring",
				"This server composes four layers at its composition root — the one place the "+
					"whole stack is visible, deliberately spelled out in ``main.go`` next to the "+
					"call that builds it. Request ids outermost, access logging just inside, "+
					"panic recovery inside that, and the mux-error envelope wrapper innermost, "+
					"hugging the router. Every request this guide serves — including the demo "+
					"payload this page is about to fetch — flows through exactly this stack:"),
			code("chain-main-wiring", "go",
				"The production wiring: order documented in a comment, enforced by the call.",
				"server/cmd/guide/main.go",
				"// The documented order: request-id outermost (recovery, logging, and\n"+
					"// every error envelope share the id), access logging outside recovery\n"+
					"// (recovered panics log as finished 500s), recovery converting panics\n"+
					"// to the envelope, and the mux-error wrapper innermost so even the\n"+
					"// router's own 404/405 replies ride the same envelope.\n"+
					"handler := presentation.Chain(\n"+
					"\tpresentation.RequestID,\n"+
					"\tpresentation.RequestLogger(logger),\n"+
					"\tpresentation.Recover(logger),\n"+
					"\tpresentation.EnvelopeMuxErrors(logger),\n"+
					")(mux)"),
			text("chain-why-this-order",
				"Each position is a decision, not a preference. Request-id must be OUTERMMOST "+
					"because two layers beneath it need the id: the logger stamps every access line "+
					"with it, and every error envelope — including the 500 a recovered panic "+
					"produces — carries it to the client. The logger must sit OUTSIDE recovery, or "+
					"a recovered panic would never be logged as a finished request: the access log "+
					"is the ledger that says ``this request happened and ended in a 500`` with its "+
					"true status and duration. Recovery must sit OUTSIDE the mux (and your "+
					"handlers), because its whole job is to catch what they throw. And the "+
					"mux-error wrapper is INNERMOST because its subject — the stdlib ServeMux's "+
					"canned plain-text 404 and 405 bodies — is produced by the router itself; "+
					"outside the router there is nothing left to intercept. Swap any two "+
					"neighbors and a concrete guarantee dies: logging inside recovery loses 500s "+
					"from the ledger; request-id inside logging makes every log line cite an "+
					"empty id; the mux wrapper outside recovery would re-envelope a panic it "+
					"never gets to see."),
			demo("middleware:pipeline", "The pipeline, live — order, ids, recovery, timeout",
				"A real teaching-grade chain — request-id → request-logger → recover → terminal "+
					"handler, same signature and same ordering semantics as the production stack — "+
					"run in-process over httptest recordors. The payload reports the execution order "+
					"as a numbered trace (before-beats descend, the handler runs once in the middle, "+
					"after-beats unwind in reverse), the request id's propagation across every layer "+
					"and onto the response header, a deliberately panicking terminal handler "+
					"converted to the 500 envelope with the panic logged, and an "+
					"http.TimeoutHandler run whose slow handler sleeps past its budget — 503 out, "+
					"abandoned goroutine joined, no leak."),
			text("chain-demo-read",
				"Read the demo in four beats. The ``chain`` rows name each layer, its position, "+
					"and the production function it mirrors. The ``trace`` is the onion made "+
					"literal: sequence numbers 0–6, request-id's before-beat at 0 carrying an "+
					"empty ``requestIDSeen`` — the id does not exist yet, assigning it IS that "+
					"layer's job — the handler beat at 3, and the unwind 4–6 in strict reverse. "+
					"The ``requestID`` beat proves propagation: one id in, the same id observed "+
					"by every layer past the first, the same id echoed on the response header. "+
					"The ``panicRecovery`` beat runs the same chain at a terminal handler that "+
					"panics on purpose — watch the unwind order SURVIVE the panic: recover's "+
					"after-beat fires first (the 500 envelope and its error log), then the "+
					"logger still logs the finished 500, then request-id still stamps. The "+
					"``timeoutBeat`` is the fourth section's story in miniature."),
			list("chain-ordering-rules", false,
				"Wrap order is request order — the first argument to the combinator is the first "+
					"code to run; if you must debug a chain, print the wrap, not the intention.",
				"Response order is the reverse — after-code unwinds innermost-first; anything an "+
					"outer layer must know about the response (status, duration, size) belongs in "+
					"its after-code.",
				"Dependents sit inside their dependencies — the logger needs the id, so it runs "+
					"inside request-id; the envelope needs recovery's output, so the mux wrapper "+
					"hugs the router inside it.",
				"Observers sit outside transformers — logging outside recovery sees the true "+
					"final status even when the response was synthesized by a recover block.",
				"One handler, exactly once — nothing in a correct chain calls ``next`` twice or "+
					"zero times; a layer either delegates exactly once or takes full ownership "+
					"of the response."),
			text("chain-coda",
				"A mental model worth keeping for the rest of the topic: a middleware chain is a "+
					"nested function-call tree that the type system lets you build at runtime. "+
					"There is no framework magic to lean on and none to fight — which is also why "+
					"the next two sections, the request-id/logging pair and panic recovery, are "+
					"just ordinary Go functions with unusually many readers. The demo you just "+
					"read runs a four-layer miniature of exactly this stack; the sections that "+
					"follow open each production layer in turn."),
		),
		// ── 2. The request-id and logging middleware pattern ──────────
		section("request-id-and-logging", "The request-id and logging middleware pattern",
			"The two workhorse layers: one id per request, and one structured line per request — in that order.",
			text("rid-why",
				"Distributed systems fail in ways that are only visible across boundaries: a client "+
					"reports a 500, the server logs show four candidates, and the database log shows "+
					"nine queries from all of them. The request id is the oldest and cheapest fix in "+
					"the book: assign every request exactly one id, attach it to every log line and "+
					"every error response it produces, and echo it back so a user-reported problem "+
					"names its own paper trail. The contract this server implements is threefold: "+
					"honor a client-supplied ``x-request-id`` when present (so a caller's trace and "+
					"the server's join), mint one when absent, and never trust the inbound value — "+
					"it is trimmed, capped at a length bound, and treated as an opaque string, "+
					"because a hostile header must not be able to bloat every log line. The whole "+
					"layer is small enough to read in one sitting:"),
			code("rid-middleware", "go",
				"The production RequestID: honor or mint, stamp the header, propagate the context.",
				"server/internal/presentation/middleware.go",
				"// RequestID assigns every request exactly one id: the inbound\n"+
					"// x-request-id header when the client supplies one (trimmed, capped at\n"+
					"// MaxInboundRequestIDLength so hostile headers cannot bloat logs), else a\n"+
					"// freshly minted one. The id is echoed on the response header, propagated\n"+
					"// through the request context for handlers, logging, and error envelopes,\n"+
					"// and set on the response before the inner stack runs so even a\n"+
					"// mid-response failure keeps it.\n"+
					"func RequestID(next http.Handler) http.Handler {\n"+
					"\treturn http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {\n"+
					"\t\tid := strings.TrimSpace(r.Header.Get(core.RequestIDHeader))\n"+
					"\t\tif id == \"\" {\n"+
					"\t\t\tid = core.NewRequestID()\n"+
					"\t\t}\n"+
					"\t\tif len(id) > core.MaxInboundRequestIDLength {\n"+
					"\t\t\tid = id[:core.MaxInboundRequestIDLength]\n"+
					"\t\t}\n"+
					"\t\tw.Header().Set(core.RequestIDHeader, id)\n"+
					"\t\tnext.ServeHTTP(w, r.WithContext(core.WithRequestID(r.Context(), id)))\n"+
					"\t})\n"+
					"}"),
			text("rid-two-moves",
				"Two moves in that function carry all the weight. First, ``w.Header().Set`` runs "+
					"BEFORE ``next`` — headers cannot be modified once the first byte of the body "+
					"is written, so stamping early means even a mid-response crash leaves the id "+
					"on whatever the client received. Second, the id travels inward as request "+
					"CONTEXT, on a derived request — ``r.WithContext`` — because that is the only "+
					"per-request channel Go offers that reaches every handler without touching "+
					"handler signatures. The context pair that makes it work lives in the core "+
					"package, and its key type is the detail worth memorizing: an UNEXPORTED "+
					"empty struct, so no other package can collide with, forge, or sift out the "+
					"value — the same encapsulation the goroutines topic's ``ctx`` conventions "+
					"rest on:"),
			code("rid-context", "go",
				"The context pair every layer and every error envelope reads the id through.",
				"server/internal/core/requestid.go",
				"// requestIDKey is the unexported context key type: no other package can\n"+
					"// collide with or extract the value by accident.\n"+
					"type requestIDKey struct{}\n"+
					"\n"+
					"// WithRequestID returns a context carrying the request id. The\n"+
					"// presentation middleware calls this once per request; WriteError and any\n"+
					"// handler read it back with RequestIDFromContext.\n"+
					"func WithRequestID(ctx context.Context, id string) context.Context {\n"+
					"\treturn context.WithValue(ctx, requestIDKey{}, id)\n"+
					"}\n"+
					"\n"+
					"// RequestIDFromContext returns the ambient request id, or \"\" outside a\n"+
					"// request (or when the middleware did not run).\n"+
					"func RequestIDFromContext(ctx context.Context) string {\n"+
					"\tif ctx == nil {\n"+
					"\t\treturn \"\"\n"+
					"\t}\n"+
					"\tid, _ := ctx.Value(requestIDKey{}).(string)\n"+
					"\treturn id\n"+
					"}"),
			text("log-problem",
				"The access-log layer answers a question the ``http.ResponseWriter`` interface "+
					"makes surprisingly hard: what status did we actually send? A handler calls "+
					"``WriteHeader(418)`` or just ``Write`` and the interface records nothing you "+
					"can read back — by the time the logging layer's after-code runs, the status "+
					"is already on the wire and gone from view. The standard fix is the second "+
					"oldest idiom in net/http: wrap the writer in a small recording decorator "+
					"that notes the status as it passes through. This server's version also "+
					"forwards ``Flush`` and ``Unwrap`` so streaming responses and "+
					"``http.ResponseController`` keep working through the chain — details the "+
					"realtime topic will be glad of — and exposes ``Written`` so the recovery "+
					"layer can tell whether an envelope is still deliverable:"),
			code("log-recorder", "go",
				"The status recorder: the writer decorator that makes completion logging possible.",
				"server/internal/presentation/middleware.go",
				"// statusRecorder captures the response status for the access log. It\n"+
					"// forwards Flush and Unwrap so http.ResponseController (and classic\n"+
					"// type-assertion flushing, e.g. for later SSE demos) keeps working through\n"+
					"// the chain.\n"+
					"type statusRecorder struct {\n"+
					"\thttp.ResponseWriter\n"+
					"\tstatus int\n"+
					"\twrote  bool\n"+
					"}\n"+
					"\n"+
					"func (r *statusRecorder) WriteHeader(code int) {\n"+
					"\tif !r.wrote {\n"+
					"\t\tr.status = code\n"+
					"\t\tr.wrote = true\n"+
					"\t}\n"+
					"\tr.ResponseWriter.WriteHeader(code)\n"+
					"}\n"+
					"\n"+
					"func (r *statusRecorder) Write(b []byte) (int, error) {\n"+
					"\tr.wrote = true\n"+
					"\treturn r.ResponseWriter.Write(b)\n"+
					"}\n"+
					"\n"+
					"// Written reports whether anything (headers or body) has gone out yet —\n"+
					"// the recovery layer uses it to know whether an envelope is still\n"+
					"// deliverable.\n"+
					"func (r *statusRecorder) Written() bool { return r.wrote }"),
			text("log-logger",
				"With the recorder in hand the logger itself is one deferred function: start the "+
					"clock before delegating, and in the defer — which runs whether the inner "+
					"stack returns, panics through, or gets recovered by the layer INSIDE it — "+
					"emit one structured ``slog`` line. Field names mirror the fastapi guide's "+
					"access log so the family's tooling reads both servers identically, and the "+
					"duration deliberately covers everything beneath this layer: inner "+
					"middleware, routing, the handler, serialization. The line you are reading "+
					"this page through was written by exactly this function:"),
			code("log-requestlogger", "go",
				"The production RequestLogger: one structured line per request, at completion.",
				"server/internal/presentation/middleware.go",
				"// RequestLogger writes one structured \"http_request\" line per request at\n"+
					"// completion: method, path, status, duration_ms, request_id. The log field\n"+
					"// names mirror the fastapi guide's access log so the family's\n"+
					"// observability tooling reads both identically; duration covers everything\n"+
					"// beneath this layer (inner middleware, routing, handler, serialization).\n"+
					"func RequestLogger(logger *slog.Logger) Middleware {\n"+
					"\treturn func(next http.Handler) http.Handler {\n"+
					"\t\treturn http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {\n"+
					"\t\t\trec := newStatusRecorder(w)\n"+
					"\t\t\tstarted := time.Now()\n"+
					"\t\t\tdefer func() {\n"+
					"\t\t\t\tdurationMs := float64(time.Since(started).Microseconds()) / 1000\n"+
					"\t\t\t\tlogger.Info(\"http_request\",\n"+
					"\t\t\t\t\t\"method\", r.Method,\n"+
					"\t\t\t\t\t\"path\", r.URL.Path,\n"+
					"\t\t\t\t\t\"status\", rec.status,\n"+
					"\t\t\t\t\t\"duration_ms\", math.Round(durationMs*10)/10,\n"+
					"\t\t\t\t\t\"request_id\", core.RequestIDFromContext(r.Context()),\n"+
					"\t\t\t\t)\n"+
					"\t\t\t}()\n"+
					"\t\t\tnext.ServeHTTP(rec, r)\n"+
					"\t\t})\n"+
					"\t}\n"+
					"}"),
			text("log-exhibit",
				"The live demo's pipeline runs a teaching-grade miniature of this pair — same "+
					"signature, same ordering semantics, recording beats instead of writing "+
					"log lines, so the ORDER becomes the observable. The exhibit's request-id "+
					"layer is recognizably the production one with the tracing added; note that "+
					"it propagates through the REAL core context helpers, which is why the demo's "+
					"panic envelope comes back carrying the same id the trace shows at every "+
					"layer:"),
			code("log-exhibit-requestid", "go",
				"The demo's exhibit request-id layer — the production shape, instrumented.",
				"server/internal/domain/demos/middleware_pipeline.go",
				"func exhibitRequestID(events *[]TraceEvent) func(http.Handler) http.Handler {\n"+
					"\treturn func(next http.Handler) http.Handler {\n"+
					"\t\treturn http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {\n"+
					"\t\t\trecord(events, \"request-id\", phaseBefore, core.RequestIDFromContext(r.Context()),\n"+
					"\t\t\t\t\"no id in the context yet — assigning one is this layer's job\")\n"+
					"\t\t\tid := strings.TrimSpace(r.Header.Get(core.RequestIDHeader))\n"+
					"\t\t\tif id == \"\" {\n"+
					"\t\t\t\tid = core.NewRequestID()\n"+
					"\t\t\t}\n"+
					"\t\t\tw.Header().Set(core.RequestIDHeader, id)\n"+
					"\t\t\tnext.ServeHTTP(w, r.WithContext(core.WithRequestID(r.Context(), id)))\n"+
					"\t\t\t// The after-beat reports the id directly: this frame's r is the\n"+
					"\t\t\t// ORIGINAL request, which never carried the derived context.\n"+
					"\t\t\trecord(events, \"request-id\", phaseAfter, id,\n"+
					"\t\t\t\t\"the response header was stamped before the inner stack ran, so even the 500 envelope carries the id\")\n"+
					"\t\t})\n"+
					"\t}\n"+
					"}"),
			text("log-coda",
				"Production hardening beyond the teaching shape, each already visible in the "+
					"quoted code: the recorder is the seam where future layers can observe "+
					"bytes written; the deferred emit means a panic unwinding THROUGH the "+
					"logger still logs (the recover layer is inside it, so by the time the "+
					"unwind reaches the defer, the status is a resolute 500); and the logger "+
					"takes its ``*slog.Logger`` as a constructor argument rather than a global, "+
					"so tests capture its output by injection — exactly how the demo's panic "+
					"beat captured its ``panic recovered`` line. Topic 33 observability builds "+
					"a whole logging chapter on this layer; here it is enough that the pattern "+
					"is: derive the request, wrap the writer, defer the emit."),
		),
		// ── 3. Panic recovery middleware ──────────────────────────────
		section("panic-recovery", "Panic recovery middleware",
			"Panics are for programmer bugs; the chain's job is that a bug in one request cannot blank the response.",
			text("panic-stance",
				"Go's position on panics is stricter than most languages': a panic means the "+
					"program is in a state its author did not reason about — a nil map write, "+
					"an index past the end, an impossible type assertion — and the only honest "+
					"responses are fix the bug or die loudly. It is emphatically not control "+
					"flow and not an error channel; errors are values (topic 30's whole "+
					"premise). But an HTTP server is a long-lived process serving thousands of "+
					"unrelated requests concurrently, and ``net/http`` already handles the "+
					"process half: each request runs on its own goroutine, and the server's "+
					"connection loop recovers handler panics it did not ask for — logging "+
					"``http: panic serving`` with a stack and closing that one connection. "+
					"What the stdlib does NOT give the client is an answer: the connection "+
					"just dies mid-response, so the caller sees a reset where a JSON envelope "+
					"should be. Recovery middleware exists to finish that sentence."),
			text("panic-how",
				"The mechanism is the language's own: a ``defer`` armed before the inner stack "+
					"runs, checking ``recover()`` after. A panic unwinding through the inner "+
					"layers stops at that defer; the layer then converts it into the same "+
					"central-writer envelope every other error uses — a 500 with the generic "+
					"internal message, internals never leaked — and logs the real panic value "+
					"plus a stack trace at error level, with the request id from the layer "+
					"above. This is the production function, and every clause in it is load-"+
					"bearing:"),
			code("panic-recover-func", "go",
				"The production Recover: convert the panic, log the truth, never leak internals.",
				"server/internal/presentation/middleware.go",
				"// Recover converts panics into the 500 internal error envelope via the\n"+
					"// central writer and logs them at error with the stack — panics are for\n"+
					"// programmer bugs only, never control flow. http.ErrAbortHandler is\n"+
					"// re-raised untouched: it is net/http's own sentinel for \"the connection\n"+
					"// is gone\", where writing an envelope is impossible and wrong. If the\n"+
					"// response has already started, no envelope can be delivered; the panic is\n"+
					"// still logged with the stack so nothing disappears.\n"+
					"func Recover(logger *slog.Logger) Middleware {\n"+
					"\treturn func(next http.Handler) http.Handler {\n"+
					"\t\treturn http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {\n"+
					"\t\t\tdefer func() {\n"+
					"\t\t\t\tif recovered := recover(); recovered != nil {\n"+
					"\t\t\t\t\tif recovered == http.ErrAbortHandler {\n"+
					"\t\t\t\t\t\tpanic(recovered)\n"+
					"\t\t\t\t\t}\n"+
					"\t\t\t\t\trequestID := core.RequestIDFromContext(r.Context())\n"+
					"\t\t\t\t\tlogger.Error(\"panic recovered\",\n"+
					"\t\t\t\t\t\t\"request_id\", requestID,\n"+
					"\t\t\t\t\t\t\"method\", r.Method,\n"+
					"\t\t\t\t\t\t\"path\", r.URL.Path,\n"+
					"\t\t\t\t\t\t\"panic\", recovered,\n"+
					"\t\t\t\t\t\t\"stack\", string(debug.Stack()),\n"+
					"\t\t\t\t\t)\n"+
					"\t\t\t\t\tif writer, ok := w.(interface{ Written() bool }); ok && writer.Written() {\n"+
					"\t\t\t\t\t\treturn\n"+
					"\t\t\t\t\t}\n"+
					"\t\t\t\t\tcore.WriteError(w, r, core.Internal(\"Internal server error.\"), logger)\n"+
					"\t\t\t\t}\n"+
					"\t\t\t}()\n"+
					"\t\t\tnext.ServeHTTP(w, r)\n"+
					"\t\t})\n"+
					"\t}\n"+
					"}"),
			text("panic-edges",
				"The two edge clauses are where naive versions go wrong. ``http.ErrAbortHandler`` "+
					"is net/http's OWN sentinel panic — the transport's way of saying ``this "+
					"connection is gone, stop touching it`` — and recovering it breaks the "+
					"server's cleanup, so it is re-raised untouched. And a response whose bytes "+
					"have already gone out CANNOT be replaced: HTTP has no unsend. When the "+
					"recorder from the logging layer reports ``Written``, the layer does the "+
					"only remaining right thing — log the panic with its stack and let the "+
					"truncated body stand. Note the quiet dependency: that ``Written`` probe "+
					"works because recovery is composed INSIDE the logger, whose recorder wraps "+
					"the writer the recover block sees. Ordering strikes again."),
			text("panic-demo-beat",
				"The live demo's panic beat runs exactly this shape against a terminal handler "+
					"that panics on purpose after recording its beat — a simulated nil-map "+
					"write. The payload then shows the whole sentence the middleware writes: "+
					"the 500 envelope (``{\"error\":{\"code\":\"internal\",…}}`` with the SAME "+
					"request id the trace shows at every layer), the captured ``panic "+
					"recovered`` log line with ``request_id`` and the panic value, and the "+
					"unwind order surviving — recover's after-beat, then the access log's, "+
					"then request-id's. The exhibit version is the production one with the "+
					"edges trimmed for readability, and its terminal handler is two lines:"),
			code("panic-exhibit", "go",
				"The demo's recover exhibit and the deliberate bug it exists to catch.",
				"server/internal/domain/demos/middleware_pipeline.go",
				"func exhibitTerminalPanic(events *[]TraceEvent) http.Handler {\n"+
					"\treturn http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {\n"+
					"\t\trecord(events, \"terminal handler\", phaseHandler, core.RequestIDFromContext(r.Context()),\n"+
					"\t\t\t\"the deliberate bug: a write to a nil map mid-handler\")\n"+
					"\t\tpanic(\"assignment to entry in nil map\")\n"+
					"\t})\n"+
					"}"),
			list("panic-policy", false,
				"Panics are for bugs — impossible states, invariant violations, programmer "+
					"errors; anything a caller can provoke with input is an ERROR VALUE, not a "+
					"panic.",
				"Recover at the chain — one recovery layer per stack, composed outside the "+
					"handlers; recover scattered through business code hides bugs instead of "+
					"reporting them.",
				"Log the stack — the panic value alone is a riddle; ``runtime/debug.Stack`` in "+
					"the error log is what makes the report actionable at 3 a.m.",
				"Answer generically — the client gets the taxonomy's internal envelope and "+
					"nothing else; a stack trace in a response body is a vulnerability report "+
					"you filed against yourself.",
				"Re-raise ErrAbortHandler — the transport's sentinel panic is not yours; "+
					"swallowing it corrupts connection cleanup.",
				"Respect committed bytes — once ``Write`` has run, log and stand down; a second "+
					"``WriteHeader`` cannot unsend what the client already received."),
			text("panic-coda",
				"Notice what recovery does NOT do: retry, degrade, or decide. It converts one "+
					"request's catastrophic failure into an honest 500 and lets the process "+
					"keep serving the next request — the isolation net, not a resilience "+
					"mechanism. Retries, circuit breaking, and graceful degradation are "+
					"deliberate policies that live in topic 39, built on error values and "+
					"contexts, not on the panic path. The discipline that keeps this layer "+
					"boring is the same one the goroutines topic drilled: every channel "+
					"owned, every goroutine joined — panics in middleware are almost always "+
					"a bug somewhere else wearing a middleware costume."),
		),
		// ── 4. http.TimeoutHandler and the stdlib's limits ────────────
		section("timeouts", "http.TimeoutHandler and the stdlib's limits",
			"A slow handler is a stuck goroutine with a client attached — the stdlib has two answers, and neither is complete.",
			text("to-why",
				"Every in-flight request holds a goroutine, a connection, and a client's "+
					"patience. A handler that blocks forever — a database call with no "+
					"deadline, a lock that never comes free, a sleep wired to the wrong "+
					"condition — holds all three indefinitely, and enough of them take the "+
					"process down by exhaustion. The first line of defense is the server "+
					"level: ``http.Server``'s read and write timeouts bound the SOCKET, "+
					"defeating slowloris-style clients and answers that dribble out forever. "+
					"This server sets all four, and the composition root is where they live:"),
			code("to-server-timeouts", "go",
				"Server-level bounds: the socket's deadlines, set once at the composition root.",
				"server/cmd/guide/main.go",
				"srv := &http.Server{\n"+
					"\tAddr:    fmt.Sprintf(\":%d\", cfg.Port),\n"+
					"\tHandler: handler,\n"+
					"\t// Bounded reads/writes defeat slow-client abuse while leaving ample\n"+
					"\t// headroom for the guide's demos.\n"+
					"\tReadHeaderTimeout: 5 * time.Second,\n"+
					"\tReadTimeout:       10 * time.Second,\n"+
					"\tWriteTimeout:      15 * time.Second,\n"+
					"\tIdleTimeout:       60 * time.Second,\n"+
					"}"),
			text("to-server-limits",
				"But socket deadlines are the wrong tool for handler deadlines, in both "+
					"directions. ``WriteTimeout`` fails WRITES — it does not stop a handler "+
					"that is stuck before its first write, which keeps holding its goroutine "+
					"with the socket healthy; and it is a blunt instrument for legitimate "+
					"slow responses, because it cannot tell a streaming answer from a stuck "+
					"one. What you usually want is per-route: THIS subtree gets 500 "+
					"milliseconds, that one may take thirty seconds. The standard library's "+
					"answer is ``http.TimeoutHandler`` — a wrapper, notably NOT a "+
					"``func(http.Handler) http.Handler`` middleware but a constructor "+
					"returning one — that races the wrapped handler against a budget and "+
					"answers 503 with its own canned body when the budget loses:"),
			code("to-timeout-beat", "go",
				"The demo's timeout beat: the real wrapper, a really slow handler, a bounded join.",
				"server/internal/domain/demos/middleware_pipeline.go",
				"func exhibitSlowHandler(done chan<- struct{}) http.Handler {\n"+
					"\treturn http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {\n"+
					"\t\tselect {\n"+
					"\t\tcase <-time.After(pipelineSlowHandlerWait):\n"+
					"\t\t\t// The wrapper never got a timely response; anything written now\n"+
					"\t\t\t// lands on an abandoned buffer and is discarded.\n"+
					"\t\tcase <-r.Context().Done():\n"+
					"\t\t}\n"+
					"\t\tw.Header().Set(\"Content-Type\", \"application/json\")\n"+
					"\t\t_, _ = w.Write([]byte(`{\"slow\":\"finally\"}`))\n"+
					"\t\tclose(done)\n"+
					"\t})\n"+
					"}\n"+
					"\n"+
					"// …\n"+
					"\twrapped := http.TimeoutHandler(exhibitSlowHandler(done), pipelineTimeoutBudget, \"\")"),
			text("to-mechanics",
				"The mechanics are worth knowing precisely, because they differ from every "+
					"wrapper above it. ``TimeoutHandler`` runs the wrapped handler on its OWN "+
					"goroutine and races it against the budget; on timeout the parent writes "+
					"a 503 and the wrapper's message body, and — the half most code forgets — "+
					"it derives a timeout context and passes it to the handler, so a handler "+
					"that SELECTS on ``r.Context().Done()`` observes the expiry and can stop "+
					"early. In this Go version the constructor takes the message body as its "+
					"third argument; the empty string selects the classic canned page the "+
					"demo's payload records verbatim. The live demo runs exactly this race — "+
					"a 30-millisecond budget against a 120-millisecond sleep — and its payload "+
					"reports the SHAPE, never the timings: wrapper's status 503, the canned "+
					"body, and a bounded join proving the abandoned handler goroutine "+
					"FINISHED (observing cancellation, closing its done channel) rather than "+
					"leaking past the request."),
			text("to-stdlib-limits",
				"Now the honest ledger. ``TimeoutHandler`` has three real limits. It does not "+
					"support the ``Hijacker`` or ``Flusher`` interfaces — streaming responses "+
					"(SSE, WebSockets) cannot pass through it, which relegates it to JSON-ish "+
					"endpoints. It buffers the whole successful response in memory before "+
					"releasing it, so a large payload doubles its footprint under the wrapper. "+
					"And it does not KILL the slow handler — the goroutine runs to its own "+
					"end; the wrapper only stops waiting. That last limit is why the real "+
					"answer for anything nontrivial is the one the goroutines topic built: "+
					"context deadlines INSIDE the handler, honored at every blocking call, so "+
					"work actually stops — with ``TimeoutHandler`` as the belt to the "+
					"handler's suspenders, and the server-level timeouts as the outer fence."),
			list("to-toolbox", false,
				"Server timeouts — the outer fence: socket-level bounds against slow "+
					"clients; always set them, but never mistake them for handler policy.",
				"TimeoutHandler — the per-route belt: race the handler, answer 503 yourself, "+
					"cancel the context; JSON endpoints only (no streaming, buffered body).",
				"Context deadlines — the real mechanism: derived per unit of work, honored at "+
					"every blocking call, checkable with ``errors.Is``; the only tool that "+
					"actually stops work.",
				"Client-side timeouts — the caller's half: an http.Client without a timeout is "+
					"the same stuck goroutine, mirrored; topic 39 makes this symmetric."),
			text("to-coda",
				"The composition this guide actually runs is the layered one: socket fences "+
					"in ``main.go``, context discipline in every handler that can block, and "+
					"``TimeoutHandler`` held in reserve for routes that need a hard, simple "+
					"cap. When you reach for it, reach knowing its shape — the demo's "+
					"timeout beat is the reference: budget, sleep, 503, canned body, "+
					"goroutine joined. Nothing about timeouts is hard; everything about them "+
					"is discipline, and the discipline is: every wait gets a bound, every "+
					"bound gets an owner, every owner gets a log line."),
		),
		// ── 5. Hand-rolled versus chi's middleware stack ──────────────
		section("hand-rolled-vs-chi", "Hand-rolled versus chi's middleware stack",
			"chi's greatest trick is that it barely has a middleware system — it standardized on the stdlib's shape.",
			text("chi-what",
				"go-chi/chi is the standard library-shaped router: a ``chi.Router`` IS an "+
					"``http.Handler``, mounts straight into ``http.ServeMux`` (or replaces it "+
					"entirely), speaks the same 1.22-style method-and-path patterns, and — the "+
					"part that matters here — standardized on exactly the middleware signature "+
					"this topic hand-rolled: ``func(http.Handler) http.Handler``. That is not "+
					"a coincidence or a concession; it is the bet that the ecosystem's common "+
					"denominator is already right. Every layer this topic quoted would run "+
					"unchanged under chi, and every middleware in chi's standard suite would "+
					"run unchanged under this server's ``Chain`` — interop is the signature, "+
					"not a feature list."),
			text("chi-api",
				"What chi adds around that signature is ergonomics and scoping. "+
					"``r.Use(middleware.RequestID, middleware.Logger, middleware.Recoverer)`` "+
					"appends layers to a ROUTER rather than composing them at one composition "+
					"root — the stack travels with the subtree. ``r.With(extra)`` derives an "+
					"inline stack for one route or group, so an admin group can carry "+
					"authentication without a second server. And the standard middleware "+
					"suite ships the usual suspects prebuilt — ``RequestID``, ``Logger``, "+
					"``Recoverer``, ``RealIP``, ``Throttle``, ``Timeout`` — each honoring "+
					"chi's URL-parameter context so log lines can cite the ROUTE, not just "+
					"the path. Under gin or echo the story differs: their middleware take "+
					"framework-specific contexts (``*gin.Context``, ``echo.Context``) and "+
					"run through the engine's own chain — richer hooks (abort semantics, "+
					"per-request mutable state), but you leave the stdlib signature at the door."),
			text("chi-what-you-lose-or-gain",
				"The honest comparison is not features but OWNERSHIP. Hand-rolled, as this "+
					"guide does it, you own every line: the envelope semantics, the exact log "+
					"fields, the ``ErrAbortHandler`` clause, the canned-body interception "+
					"trick in ``EnvelopeMuxErrors`` — none of which any framework's Recoverer "+
					"will replicate, because they encode THIS family's contracts. The cost "+
					"is that you wrote them and you test them (this repo's middleware tests "+
					"pin each edge the prose claims). With chi you trade that ownership for "+
					"batteries and years of hardening, and you keep most of the control — "+
					"chi's suite is small enough to read, and its pieces compose under your "+
					"own ``Chain`` if you want them to. The go.mod is the guide's own "+
					"verdict, deliberate and visible:"),
			code("chi-gomod", "mod",
				"The whole manifest: no router, no middleware suite — stdlib is the backbone.",
				"server/go.mod",
				"module guide\n"+
					"\n"+
					"go 1.27.3"),
			list("chi-decision", false,
				"Take chi when — the route table is large and params-heavy, you want "+
					"route-scoped middleware groups, and you would rather configure hardened "+
					"middleware than own it.",
				"Stay stdlib when — the routing needs are modest (1.22 patterns cover them), "+
					"the error/logging contracts are bespoke, and every dependency in the "+
					"supply chain is a review surface you would rather not grow.",
				"Either way, keep the signature — ``func(http.Handler) http.Handler`` is the "+
					"portable asset: layers you write under stdlib run under chi, and chi's "+
					"layers run under your combinator.",
				"Migrate late, not never — wrapping the mux in chi later is a "+
					"composition-root change, not a rewrite, precisely because both sides "+
					"speak http.Handler.",
				"Avoid the framework gravity — gin/echo middleware ecosystems only pay if you "+
					"accept their contexts end to end; bolting them onto a stdlib stack buys "+
					"two of everything."),
			text("chi-coda",
				"This guide's stance generalizes beyond routers: the standard library is the "+
					"backbone, third-party libraries are adopted where they earn their keep, "+
					"and the seam that makes the policy safe is the one this topic has been "+
					"quoting all along — small interfaces (``http.Handler``), plain function "+
					"types (``Middleware``), composition at one visible root. When topic 42 "+
					"surveys the third-party landscape, middleware is the case study that "+
					"argues you can swap an ecosystem in and out around a stable, stdlib-"+
					"shaped core."),
		),
		// ── 6. Terminal companion ─────────────────────────────────────
		section("terminal-companion", "Terminal companion: the curl tour",
			"The middleware surfaces of this guide, observed from the shell.",
			text("term-pitch",
				"With the stack up (server on :8100), these commands watch the chain work on "+
					"real responses. Two of them demonstrate middleware BEHAVIOR directly: "+
					"supplying your own ``x-request-id`` and reading it echoed back, and "+
					"watching the demo payload's id agree with the very response carrying it. "+
					"``python3 -m json.tool`` keeps the tour dependency-free; the demo "+
					"payload is deterministic, so repeated calls are worth diffing:"),
			code("term-pipeline-demo", "bash", "", "",
				"# the pipeline demo: chain rows, the numbered onion trace, the id's\n"+
					"# propagation, the recovered-panic beat, the timeout beat\n"+
					"curl -s localhost:8100/api/demo/middleware/pipeline | python3 -m json.tool"),
			code("term-own-request-id", "bash", "", "",
				"# supply your own request id — watch the same id come back on the header\n"+
					"curl -s -i -H 'x-request-id: terminal-tour-08' localhost:8100/api/health"),
			code("term-demo-request-id", "bash", "", "",
				"# the demo's turn: the payload's trace cites its inbound id; -i shows the\n"+
					"# production chain stamping a (freshly minted) one on THIS response\n"+
					"curl -s -i localhost:8100/api/demo/middleware/pipeline | head -n 20"),
			code("term-404-envelope", "bash", "", "",
				"# the innermost layer's work: the mux's plain 404, rewritten into the\n"+
					"# shared error envelope — with x-request-id riding on top\n"+
					"curl -s -i localhost:8100/api/nope"),
			code("term-405-envelope", "bash", "", "",
				"# method mismatch: truthful 405 status, Allow header preserved,\n"+
					"# envelope body — three layers cooperating on one reply\n"+
					"curl -s -i -X POST localhost:8100/api/demo/middleware/pipeline"),
			code("term-topic", "bash", "", "",
				"# this topic in full: sections → text/code/list/demo blocks\n"+
					"curl -s localhost:8100/api/content/middleware | python3 -m json.tool"),
			text("term-coda",
				"The 404 and 405 tours are the ones to linger on: every layer of this topic "+
					"is visible in one reply — the request id stamped outermost, the access "+
					"log line the server just wrote, recovery standing by unused, and the "+
					"mux-error wrapper rewriting the router's plain text into the envelope. "+
					"For what the chain's layers DO with routing underneath, topic 07's "+
					"page and its routing demo are the pairing; for the logging layer's full "+
					"story, fast-forward to topic 33 observability."),
		),
	}

	if buildErr != nil {
		return Topic{}, buildErr
	}
	return NewTopic(
		"middleware",
		"Middleware",
		"The chain and its ordering, request ids and access logging, panic "+
			"recovery, timeouts, and the hand-rolled-versus-chi decision.",
		"request-handling",
		8,
		sections,
	)
}
