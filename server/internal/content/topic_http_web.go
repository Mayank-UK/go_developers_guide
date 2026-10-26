package content

import (
	"errors"
	"fmt"
)

// This file is topic 03 http-web — the protocol topic, authored per the
// FIXED per-topic pattern (see topic_go_language.go, the exemplar): an
// unexported builder assembling the typed model through model.go's
// constructors, registered only in NewProductionRegistry. The topic
// teaches HTTP generically but grounds every claim in this repo's real
// net/http code; the demo (http:headers) sits INLINE in the section that
// teaches headers; the terminal companion closes.

// httpWebTopic builds topic 03 — HTTP and the web: the request journey,
// the vocabulary of methods, statuses, and headers, and the rules
// (idempotency, negotiation, cookies, CORS, TLS, versions) every Go API
// inherits from the protocol it speaks.
func httpWebTopic() (Topic, error) {
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
		// ── 1. The request journey ──────────────────────────────────────
		section("request-journey", "The request journey",
			"From bytes on a socket to a handler and back — the whole trip, in this server's real code.",
			text("journey-pitch",
				"Every page of this guide is served over the same trip. A client — curl, a browser, the React shell — resolves the host, opens a TCP connection to port 8100, and writes an HTTP request as text: a request line (method, path, version), a block of headers, and maybe a body. Go's ``net/http`` server accepts the connection, parses that text into a typed ``*http.Request``, and hands it to a handler — your code — which fills in an ``http.ResponseWriter``: status, headers, body. The server serializes that back onto the socket, and either keeps the connection open for the next request (keep-alive) or closes it. That is the entire loop HTTP runs, billions of times a second, across the planet."),
			text("journey-goroutine",
				"Two properties of Go's implementation shape everything downstream. First, the server handles each connection on its own goroutine — concurrency is the default you get for free, not a thread pool you configure (the scheduler that makes this cheap is topic 02's whole subject). Second, the journey's stations are composed, not hardcoded: ``net/http`` gives you a router and a handler interface, and the cross-cutting steps — assigning a request id, logging, recovering panics, shaping error responses — are middleware wrapped around the router. This server's composition root wires the real chain:"),
			code("journey-chain", "go",
				"The middleware chain, outermost layer first — every request passes through it.",
				"server/cmd/guide/main.go",
				"handler := presentation.Chain(\n"+
					"    presentation.RequestID,\n"+
					"    presentation.RequestLogger(logger),\n"+
					"    presentation.Recover(logger),\n"+
					"    presentation.EnvelopeMuxErrors(logger),\n"+
					")(mux)"),
			text("journey-stations-intro",
				"Read one request's life as six stations, each of which later topics examine in depth:"),
			list("journey-stations", true,
				"Connection — TCP accept (or a TLS handshake first); Go serves each connection on its own goroutine from the first byte.",
				"Parse — the request line and headers become the typed *http.Request: method, URL, header map, body reader, context.",
				"Route — ServeMux matches the method+path pattern (GET /api/content/{topicId}) and picks the handler.",
				"Middleware — request id stamped, then logging, recovery, and error-envelope shaping wrap the handler's behavior.",
				"Handle — the thin handler reads the request, calls domain code, writes status+headers+body through ResponseWriter.",
				"Respond — the server serializes the response; writeJSON's body rides the still-open connection back to the client."),
			text("journey-routing",
				"The routing station is one data structure built at boot: a ``http.ServeMux`` loaded with method-and-path patterns (the Go 1.22 syntax this guide leans on everywhere). Registration happens once in ``main`` — a handler that is not registered is unreachable, and a pattern that names a method rejects the others with 405 automatically. The mux is this server's entire routing table:"),
			code("journey-routes", "go",
				"The whole route table: method+path patterns, registered once at boot.",
				"server/cmd/guide/main.go",
				"mux := http.NewServeMux()\n"+
					"mux.Handle(\"GET /api/health\", presentation.NewHealthHandler(cfg.Env))\n"+
					"mux.Handle(\"GET /api/content\", presentation.NewContentListHandler(registry))\n"+
					"mux.Handle(\"GET /api/content/{topicId}\", presentation.NewContentTopicHandler(registry, logger))\n"+
					"// …\n"+
					"mux.Handle(\"GET /api/demo/go/runtime-types\", presentation.NewRuntimeTypesHandler())"),
			text("journey-serve",
				"The last station hides in plain sight: serving. The ``http.Server`` owns the listeners, the timeouts, and the shutdown path. This server runs ``ListenAndServe`` on a goroutine and selects against a signal context — SIGINT or SIGTERM cancels the context, ``Shutdown`` drains in-flight requests politely, and only a genuine startup failure (a port already bound, say) surfaces as an error:"),
			code("journey-serve-loop", "go",
				"Serving and shutting down: the server's accept loop lives on its own goroutine.",
				"server/cmd/guide/main.go",
				"serveErr := make(chan error, 1)\n"+
					"go func() {\n"+
					"    logger.Info(\"server listening\", \"addr\", srv.Addr, \"env\", cfg.Env)\n"+
					"    serveErr <- srv.ListenAndServe()\n"+
					"}()"),
			text("journey-coda",
				"Hold the whole picture while the rest of the topic zooms into the request's vocabulary — methods, statuses, headers — because every later station of this guide (routing in 07, middleware in 08, request bodies in 09) is one segment of this same journey, done properly."),
		),
		// ── 2. Methods, statuses, headers (inline demo) ─────────────────
		section("methods-status-headers", "Methods, statuses, and headers",
			"The three axes of every exchange — and this topic's live demo: an echo that shows you what the server actually received.",
			text("msh-axes",
				"Every HTTP exchange carries three vocabularies at once. The METHOD is the verb — what the client wants done to the target resource. The STATUS is the result — what the server actually did, in a three-digit code every client library on earth can read. The HEADERS are the metadata plane — name/value pairs riding both directions, carrying everything from the body's media type to caching instructions to correlation ids. Go models all three directly: ``r.Method`` and ``r.Header`` on the request, ``w.WriteHeader(status)`` and ``w.Header()`` on the response."),
			text("msh-methods-intro",
				"The core methods, with the contract each one promises (the safety and idempotency words get their own section next):"),
			list("msh-methods", false,
				"GET — retrieve the resource's representation; safe and idempotent by contract, and the method this server serves most.",
				"HEAD — GET without a body: same headers, same status, no payload; Go's mux answers HEAD for any registered GET pattern.",
				"POST — submit data to be processed; neither safe nor idempotent, which is why retries need care (and idempotency keys).",
				"PUT — replace the target resource wholesale; not safe, but idempotent — the same replacement twice leaves the same state.",
				"PATCH — apply a partial modification; not safe, and idempotent only if the patch itself is — this server's scenario toggle is.",
				"DELETE — remove the target; not safe, idempotent by contract (deleting a missing resource is the same end state)."),
			text("msh-method-surface",
				"This server's method surface is declared by the route patterns themselves — ServeMux matches the method as part of the pattern, so a GET-only resource is GET-only by construction, not by an if statement inside the handler. The scenario endpoint is the one place two methods meet one resource, and the pattern pair says so explicitly:"),
			code("msh-scenario-routes", "go",
				"Two methods, one resource — declared in the pattern, enforced by the mux.",
				"server/cmd/guide/main.go",
				"scenarioStore := core.NewScenarioStore()\n"+
					"mux.Handle(\"GET /api/demo/scenario\", presentation.NewScenarioGetHandler(scenarioStore))\n"+
					"mux.Handle(\"PATCH /api/demo/scenario\", presentation.NewScenarioPatchHandler(scenarioStore, logger))"),
			text("msh-status",
				"Statuses come in five classes: 1xx informational, 2xx success, 3xx redirection, 4xx the client's mistake, 5xx the server's. APIs live mostly on 200, 201, 204, 400-odd, and 500 — and the honest rule this guide follows: the status must tell the truth, even when the taxonomy would prefer a tidier bucket. The server maps its eight failure kinds onto statuses in ONE table, so a ``KindConflict`` can never drift to a 400 on Tuesday and a 409 on Friday:"),
			code("msh-status-map", "go",
				"The one kind-to-status table — the whole app's status vocabulary.",
				"server/internal/core/errors.go",
				"var httpStatusByKind = map[Kind]int{\n"+
					"    KindValidationFailed:   http.StatusUnprocessableEntity,\n"+
					"    // …\n"+
					"    KindNotFound:           http.StatusNotFound,\n"+
					"    KindConflict:           http.StatusConflict,\n"+
					"    // …\n"+
					"    KindInternal:           http.StatusInternalServerError,\n"+
					"}"),
			text("msh-405",
				"The truth-telling rule's best exhibit is the 405. A POST to a GET-only route must answer 405 Method Not Allowed — not a 404 pretending the resource is missing — and it must carry an ``Allow`` header naming what IS allowed. Go's ServeMux does both; this server's mux-error middleware then reshapes the plain-text canned body into the same JSON error envelope every other failure uses, keeping the truthful status and the Allow header while taking the closest taxonomy bucket (``not_found``) for the body's code:"),
			code("msh-405-envelope", "go",
				"404s become the envelope; 405s keep their truthful status and Allow header.",
				"server/internal/presentation/middleware.go",
				"if i.status == http.StatusNotFound {\n"+
					"    core.WriteError(i.w, r, core.NotFound(\"Not Found\"), logger)\n"+
					"    return\n"+
					"}\n"+
					"core.WriteErrorStatus(i.w, r, i.status, core.New(core.KindForHTTPStatus(i.status), \"Method Not Allowed\"), logger)"),
			text("msh-headers",
				"Headers are the protocol's metadata plane, and Go normalizes them before your code ever sees them: names arrive canonicalized (``Accept-Encoding``, never ``accept-encoding``), the ``Host`` header is promoted out of the map into ``r.Host`` because it is routing data, not metadata, and values may repeat (a slice per name, joined with commas on the wire). The classification that matters most for understanding real traffic is SCOPE: end-to-end headers (the vast majority — ``Accept``, ``Authorization``, ``User-Agent``) are meant for the final recipient and forwarded by every proxy, while a closed RFC 9110 set (``Connection``, ``Keep-Alive``, ``Transfer-Encoding``, ``Upgrade``, and friends) is hop-by-hop — each describes one transport connection and is consumed, never forwarded. The x-request-id contract this server runs shows a custom header earning its keep both directions:"),
			code("msh-request-id", "go",
				"One custom header, both directions: honored or minted, echoed and propagated.",
				"server/internal/presentation/middleware.go",
				"return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {\n"+
					"    id := strings.TrimSpace(r.Header.Get(core.RequestIDHeader))\n"+
					"    if id == \"\" {\n"+
					"        id = core.NewRequestID()\n"+
					"    }\n"+
					"    // …\n"+
					"    w.Header().Set(core.RequestIDHeader, id)\n"+
					"    next.ServeHTTP(w, r.WithContext(core.WithRequestID(r.Context(), id)))\n"+
					"})"),
			text("msh-response-headers",
				"Response headers are set BEFORE the status line is written — Go's ``ResponseWriter`` buffers headers until the first write, then sends them as one block, which is why the order in ``writeJSON`` matters: Content-Type first, status second, body last. Every success response this server serves goes through one four-line writer, so ``application/json`` is not a per-handler decision that can be forgotten; the error path has its own single writer that stamps Content-Type AND x-request-id the same way:"),
			code("msh-writejson", "go",
				"The one success writer: Content-Type, status, body — in that order.",
				"server/internal/presentation/content.go",
				"func writeJSON(w http.ResponseWriter, status int, v any) {\n"+
					"    w.Header().Set(\"Content-Type\", \"application/json\")\n"+
					"    w.WriteHeader(status)\n"+
					"    // …\n"+
					"    _ = json.NewEncoder(w).Encode(v)\n"+
					"}"),
			text("msh-demo-pitch",
				"Which brings this topic to its demo — the most honest HTTP lesson possible: an endpoint that echoes what the server ACTUALLY received. ``GET /api/demo/http/headers`` reports the request line (method, path, raw query, negotiated protocol), the request id the middleware assigned, and every inbound header classified twice — scope (end-to-end vs hop-by-hop) and role (negotiation, CORS, security, identity, extension, transport). Credentials are detected but their values never echoed, because an echo that leaks bearer tokens would be its own counter-example. The classification lives in the pure builder the handler delegates to:"),
			code("msh-classifier", "go",
				"The classifier: security outranks transport, specific sets outrank the x- prefix.",
				"server/internal/domain/demos/http_headers.go",
				"func roleOf(name string) string {\n"+
					"    switch {\n"+
					"    case securityHeaders[name]:\n"+
					"        return RoleSecurity\n"+
					"    case hopByHopHeaders[name]:\n"+
					"        return RoleTransport\n"+
					"    case negotiationHeaders[name]:\n"+
					"        return RoleNegotiation\n"+
					"    // …\n"+
					"    }\n"+
					"}"),
			code("msh-demo-handler", "go",
				"The demo endpoint: extract the facts, classify purely, write once.",
				"server/internal/presentation/demo_http_web.go",
				"func (h *HTTPHeadersHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {\n"+
					"    writeJSON(w, http.StatusOK, demos.BuildHTTPHeadersEcho(demos.HTTPRequestFacts{\n"+
					"        Method:    r.Method,\n"+
					"        Path:      r.URL.Path,\n"+
					"        // …\n"+
					"        Header:    r.Header,\n"+
					"    }))\n"+
					"}"),
			demo("http:headers", "What the server actually received — the headers echo",
				"Send this demo any headers you like: it echoes the request line, the "+
					"request id, and every inbound header with its scope (end-to-end vs "+
					"hop-by-hop) and role (negotiation, CORS, security, identity, "+
					"extension, transport), values redacted for credentials, plus the "+
					"RFC 9110 hop-by-hop reference list."),
			text("msh-beats-intro",
				"Read the demo in five beats:"),
			list("msh-beats", false,
				"Request line — the method, path, and raw query exactly as routed, plus the negotiated protocol (HTTP/1.1 from curl in dev).",
				"Scope — every header tagged end-to-end or hop-by-hop; send Connection: keep-alive and watch it land in the second bucket.",
				"Role — Accept and Content-Type are negotiation, Origin is CORS material, Authorization and Cookie are security, X-* is extension.",
				"Redaction — Authorization and Cookie report their presence with the value withheld; an echo must never become a credential leak.",
				"Reference — the sorted RFC 9110 §7.6.1 hop-by-hop set rides along, so the list a proxy may consume is one glance away."),
			text("msh-coda",
				"Headers look like plumbing until they are the product: CORS is headers, content negotiation is headers, caching (topic 21) is headers, half of security (topic 25) is headers. The demo's classification is the map you will keep re-using."),
		),
		// ── 3. Idempotency and safety ───────────────────────────────────
		section("idempotency-safety", "Idempotency and safety",
			"Two promises methods make — and why retries, caches, and queues all depend on them.",
			text("idem-definitions",
				"Two words carry precise meanings in HTTP, and conflating them causes real outages. A method is SAFE when it changes no server state — the request is read-only, so following a link, prefetching, or a crawler hitting it a thousand times has no side effects (logging and metrics don't count). A method is IDEMPOTENT when the state after N identical requests equals the state after one — the second and later requests may run, but they cannot change the outcome further. GET and HEAD are safe; PUT and DELETE are idempotent but not safe; POST is neither; PATCH is whatever the patch semantics make it."),
			list("idem-matrix", false,
				"GET — safe and idempotent: retrieve only; the whole content API rides on it.",
				"HEAD — safe and idempotent: GET's headers for free, the body withheld.",
				"PUT — idempotent, not safe: replace with the same bytes twice, end at the same state.",
				"DELETE — idempotent, not safe: the first call removes, later calls find nothing (same end state).",
				"PATCH — conditional: idempotent exactly when the patch itself is — set-dataMode-to-fake is, append-a-line is not.",
				"POST — neither: every replay may create another charge, another row, another email."),
			text("idem-why",
				"Why the distinction earns engineering respect: distributed systems retry. A timeout does not mean the request failed — it means you do not KNOW. Retrying an idempotent request is always safe; retrying a POST blind can double-charge a card, duplicate a webhook action, or send the email twice. The same logic governs caches (only safe methods may be answered from a cache without revalidation), browser prefetchers (they only fire safe requests), and message consumers (at-least-once delivery plus idempotent handlers equals effectively-once — the pattern topic 22 builds). When topics 14 and 39 add webhooks and retries, idempotency is the load-bearing wall."),
			text("idem-live",
				"This server has one live non-GET surface, and it was designed idempotent on purpose: ``PATCH /api/demo/scenario`` merges only the fields the patch sets, so sending the same patch body twice lands on the same state — and an invalid value is rejected BEFORE any mutation, so a failed replay cannot half-apply. The store's merge is deliberately copy-in, merge, copy-out:"),
			code("idem-patch-merge", "go",
				"An idempotent merge: same patch twice, same state; invalid input never lands.",
				"server/internal/core/scenario.go",
				"next := s.current\n"+
					"if patch.DataMode != nil {\n"+
					"    next.DataMode = *patch.DataMode\n"+
					"}\n"+
					"// …\n"+
					"s.current = next\n"+
					"return next, nil"),
			text("idem-coda",
				"The honest caveat: the protocol can only DEFINE idempotency; the server must DELIVER it. Nothing in net/http enforces that your PUT handler is really idempotent — that is what tests are for (the scenario store's tests replay patches on purpose). Treat method semantics as a contract you sign with every client that trusts your status line."),
		),
		// ── 4. Content negotiation ─────────────────────────────────────
		section("content-negotiation", "Content negotiation",
			"How client and server agree on a representation — and this server's deliberate JSON-only stance.",
			text("neg-pitch",
				"The same resource can have many representations — JSON, XML, HTML, a language, a compression — and HTTP's negotiation family lets the two sides choose without out-of-band knowledge. The client states preferences with ``Accept`` (media types, with quality weights like ``application/json;q=0.9``), ``Accept-Encoding`` (compression codings), and ``Accept-Language``; the server picks what it can serve, and says what it picked in the response's ``Content-Type``. A server that cannot satisfy ``Accept`` answers 406 Not Acceptable — or, in practice, most APIs serve JSON and let the client cope."),
			list("neg-family", false,
				"Accept — media types the client will consume, weighted; the demos section shows yours arriving verbatim.",
				"Accept-Encoding — compression codings (gzip, br, zstd); Go's server does NOT auto-compress responses — that is handler or middleware work.",
				"Accept-Language — preferred languages, q-weighted; topic 27's i18n demo turns this header into an actual choice.",
				"Content-Type — the other direction: the media type of the body being sent, request or response."),
			text("neg-stance",
				"This guide's stance is deliberately simple, and honest about it: one representation. Every success response is JSON via the single ``writeJSON`` helper you met above, so the response ``Content-Type`` is always ``application/json`` and 406 never happens — there is nothing to negotiate. The client shell consumes exactly that shape, and the family's API contract (camelCase, optional fields omitted) is the fixed deal. Real negotiation — serving HTML to browsers and JSON to API clients from one route — arrives with topic 09's request-response deep dive, where ``Accept`` parsing earns its keep."),
			text("neg-bodies",
				"Inbound, the mirror question is what the REQUEST body claims to be. ``Content-Type: application/json`` is the contract this server's one body-accepting endpoint expects — and its decoder is honest about the two things that can go wrong: a body that is not valid JSON, and a body that is valid JSON of the wrong shape. What Go does NOT do automatically is reject a body whose Content-Type lies; ``json.Decoder`` parses bytes regardless of the header, so strict Content-Type checking (a one-line guard) is a decision topic 09 makes explicitly:"),
			code("neg-decode", "go",
				"The request-body counterpart: bound the read, decode exactly one value.",
				"server/internal/presentation/scenario.go",
				"decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxScenarioBodyBytes))\n"+
					"var raw json.RawMessage\n"+
					"if err := decoder.Decode(&raw); err != nil {\n"+
					"    return invalid(err.Error())\n"+
					"}"),
			text("neg-coda",
				"One family rule for the road: JSON's encoding is UTF-8 by definition (RFC 8259), so ``application/json`` carries no charset parameter worth sending — ``application/json; charset=utf-8`` is noise, and Go's encoder never adds it. Negotiation matters most exactly when you outgrow one representation; until then, one honest Content-Type beats a misleading negotiation."),
		),
		// ── 5. Cookies, sessions, same-origin ──────────────────────────
		section("cookies-sessions-same-origin", "Cookies, sessions, and same-origin",
			"How the stateless protocol remembers — and the browser rule all of it hangs on.",
			text("cookie-pitch",
				"HTTP itself remembers nothing: each request stands alone, and any appearance of continuity is built on top. The browser's half of that build is the cookie. A server plants one with a ``Set-Cookie`` response header — a name, a value, and attributes — and the browser stores it and then replays it as a ``Cookie`` request header on every later request that matches. No header parsing on the client side, no per-request decision: cookies are the browser's automatic memory, which is exactly why they became both the workhorse of sessions and the attack surface CSRF and tracking protection grew up around."),
			text("cookie-attributes",
				"The attributes are the security model, and each one answers one threat:"),
			list("cookie-attrs", false,
				"HttpOnly — JavaScript cannot read the cookie; XSS may run scripts but cannot exfiltrate what it cannot see.",
				"Secure — the cookie rides HTTPS only, never a plain-http hop.",
				"SameSite — Lax/Strict withhold the cookie on cross-site requests (the CSRF defense) ; None allows it but requires Secure.",
				"Path/Domain — which requests match: scope the cookie to the smallest path that works.",
				"Expires/Max-Age — persistence versus the session cookie that dies with the browser tab."),
			text("cookie-sessions",
				"Sessions are the pattern cookies enable: the server mints an unguessable session id, plants it as an HttpOnly cookie, and keeps the real state server-side keyed by that id — so a stolen value is one revocable handle, not the state itself. Go models both halves directly: ``http.SetCookie`` takes an ``http.Cookie`` struct whose fields mirror the attributes one-to-one (Name, Value, HttpOnly, Secure, SameSite with its ``http.SameSiteLaxMode`` constants), and inbound cookies arrive pre-parsed via ``r.Cookie(name)`` and ``r.Cookies()`` — no header string surgery. Topic 23 builds the real thing: login, session store, rotation, logout."),
			text("cookie-stance",
				"This server today is deliberately cookieless — and per-request state travels a different road: the request CONTEXT. The middleware assigns the request id once and threads it through ``context.WithValue``; every layer beneath — handlers, error writer, logs — reads it back without any cookie or global. It is the same shape a session id would take through the call stack, minus the browser memory:"),
			code("cookie-context", "go",
				"State without cookies: per-request facts travel the request context.",
				"server/internal/core/requestid.go",
				"func WithRequestID(ctx context.Context, id string) context.Context {\n"+
					"    return context.WithValue(ctx, requestIDKey{}, id)\n"+
					"}\n"+
					"// …\n"+
					"func RequestIDFromContext(ctx context.Context) string {\n"+
					"    if ctx == nil {\n"+
					"        return \"\"\n"+
					"    }\n"+
					"    id, _ := ctx.Value(requestIDKey{}).(string)\n"+
					"    return id\n"+
					"}"),
			text("cookie-same-origin",
				"The other half of the section's title is the rule everything above hangs on: SAME-ORIGIN. Two URLs share an origin when their scheme, host, and port all match — ``http://localhost:5175`` and ``http://localhost:8100`` are DIFFERENT origins (the port differs), just as ``https://a.dev`` and ``https://b.dev`` differ. The browser's same-origin policy is the fundamental boundary of web security: a script from one origin cannot read a response from another, cookies are scoped by origin-matching rules, and every relaxation — CORS, next section — is an explicit, server-signed exception. Say the triple out loud once: scheme, host, port."),
		),
		// ── 6. CORS ───────────────────────────────────────────────────
		section("cors", "CORS: when origins must cross",
			"The server-controlled exception to the same-origin policy — and why this repo needs none of it.",
			text("cors-pitch",
				"The same-origin policy is strict: a page loaded from origin A cannot read responses from origin B, even though it can often SEND requests there. Since real frontends legitimately need exactly that — a shell on one origin calling an API on another — the relaxation mechanism is standardized as CORS, Cross-Origin Resource Sharing. Its shape surprises everyone once: CORS is enforced by the BROWSER, configured by the SERVER. The server declares ``Access-Control-Allow-Origin`` and friends; the browser reads the declaration and only then decides whether the page's JavaScript may see the response. curl ignores all of it — which is why your API works from the terminal and fails from the browser, the classic first debugging session."),
			text("cors-preflight",
				"The mechanism has two lanes. SIMPLE requests — GET, POST, or HEAD with no custom headers — go straight to the server, and the browser checks the response headers before exposing the body. Anything beyond that (a ``PUT``, a ``Content-Type: application/json`` body, any custom ``X-`` header like this guide's ``x-request-id``) triggers a PREFLIGHT: the browser first sends an ``OPTIONS`` request carrying ``Access-Control-Request-Method`` and ``Access-Control-Request-Headers``, and the actual request only proceeds if the server's answer allows both. Preflights are cached (``Access-Control-Max-Age``), so the tax is per-policy, not per-request. This is why the headers demo matters: send it an ``Origin`` and an ``X-Custom`` header and you are looking at exactly the material a preflight decision is made from."),
			text("cors-stance",
				"This repo runs no CORS middleware, on purpose: its origins never cross. In development the React shell is served by Vite on ``localhost:5175``, and Vite's dev proxy forwards ``/api`` to the Go server on ``8100`` — so as far as the browser can tell, every request is same-origin, no ``Origin`` header is even sent on GETs, and the question never arises. In deployment the halves ship under one origin the same way. The proxy is three lines of real config:"),
			code("cors-proxy", "ts",
				"Same-origin by construction: the dev shell proxies /api to the Go server.",
				"client/vite.config.ts",
				"server: {\n"+
					"    host: '0.0.0.0',\n"+
					"    port: 5175,\n"+
					"    strictPort: true,\n"+
					"    proxy: {\n"+
					"      '/api': { target: apiProxyTarget, ws: true },\n"+
					"    },\n"+
					"  },"),
			text("cors-coda",
				"When a genuine second origin must call this API — a partner dashboard, a hosted tool — the fix is a small, explicit middleware that answers preflights with a strict allow-list (origin, methods, headers, and ``Access-Control-Allow-Credentials`` only when the cookie story truly needs it). That middleware is topic 25 security's to build properly; the trap to avoid until then is the wildcard reflex — ``Access-Control-Allow-Origin: *`` with credentials is both rejected by browsers and a policy hole on its own."),
		),
		// ── 7. TLS and HTTPS ──────────────────────────────────────────
		section("tls-https", "TLS and HTTPS",
			"Encrypting and authenticating the transport — what the handshake does, and Go's one-call version.",
			text("tls-pitch",
				"Plain HTTP is readable and forgeable by every machine on the path — the coffee-shop router, the ISP, the proxy. TLS fixes both with one handshake: the server presents a CERTIFICATE a trusted authority has signed (proving it is the host it claims), the two sides derive shared symmetric keys from an exchange the eavesdropper cannot replay, and from then on the connection carries encrypted, integrity-checked records. ``https://`` means TLS; port 443 is its convention. Everything HTTP-level you have read so far — methods, statuses, headers — rides inside unchanged, which is why switching to HTTPS changes transport code, not API code."),
			text("tls-go",
				"Go makes the server side a one-call switch: ``srv.ListenAndServeTLS(certFile, keyFile)`` instead of ``ListenAndServe``, or an explicit ``crypto/tls.Config`` on the ``http.Server`` for control (minimum version, cipher suites, client certificates for mTLS). Two bonuses arrive automatically: Go's server negotiates HTTP/2 over any TLS connection (next section), and the same timeouts you set for plain HTTP still bound the encrypted exchange — TLS changes the wire, not the lifecycle:"),
			code("tls-server", "go",
				"Today's server: plain HTTP in dev, with the lifecycle timeouts TLS will inherit.",
				"server/cmd/guide/main.go",
				"srv := &http.Server{\n"+
					"    Addr:    fmt.Sprintf(\":%d\", cfg.Port),\n"+
					"    Handler: handler,\n"+
					"    // …\n"+
					"    ReadHeaderTimeout: 5 * time.Second,\n"+
					"    ReadTimeout:       10 * time.Second,\n"+
					"    WriteTimeout:      15 * time.Second,\n"+
					"    IdleTimeout:       60 * time.Second,\n"+
					"}"),
			text("tls-stance",
				"This guide's dev loop runs plain HTTP on localhost inside a dev container — no certificates to mint, and loopback traffic never leaves the machine. That is a development convenience, not a deployment posture: production TLS terminates in front of the binary (a reverse proxy or a load balancer holding the certificates, forwarding plain HTTP inside the network) or on the server itself via ``ListenAndServeTLS``; topic 37 wires the real deployment and its certificate rotation. For local HTTPS experiments, ``mkcert`` mints a locally-trusted CA and cert pair in one command — useful the day you need to test Secure cookies or HTTP/2 behavior in development."),
			text("tls-coda",
				"One mental model to keep: HTTPS = HTTP + a TLS session. The request journey from section one is unchanged — accept, parse, route, middleware, handle, respond — with one new station, the handshake, before the first byte of any request. When something breaks with TLS, check the new station first: certificate expiry and hostname mismatch outrank every other suspect."),
		),
		// ── 8. HTTP versions ──────────────────────────────────────────
		section("http-versions", "HTTP versions: 1.1, 2, and 3",
			"What changed across versions — and which one this server speaks to you right now.",
			text("ver-pitch",
				"HTTP/1.1 (1997) is the version that made the web work: persistent connections (keep-alive) by default so one TCP connection serves many requests, ``Host`` headers so one IP can serve many sites, and chunked transfer encoding so a response can start before its length is known. Its ceiling is head-of-line blocking: requests on one connection are answered in order, so one slow response stalls everything queued behind it. Browsers fought back with six parallel connections per host; the protocol fixed it properly instead."),
			text("ver-h2",
				"HTTP/2 (2015) kept the semantics — methods, statuses, headers, everything this topic taught — and replaced the wire format: binary frames instead of text, many parallel STREAMS multiplexed over one TCP connection (no request queue), and HPACK header compression. A slow response no longer blocks its siblings; one connection per host is enough. The negotiation trick matters for Go: over TLS, the TLS handshake's ALPN extension agrees on ``h2`` or ``http/1.1``, and Go's server speaks HTTP/2 automatically whenever TLS is on — no code change. HTTP/3 (QUIC, over UDP) goes further and fixes TCP-level blocking with streams that don't share one lossy pipe, plus faster handshakes; Go tracks it outside the standard library, and no part of this guide depends on it."),
			list("ver-comparison", false,
				"HTTP/1.1 — text on TCP; keep-alive connections; sequential per connection; what curl speaks to this dev server, and what r.Proto reports in the demo.",
				"HTTP/2 — binary frames, multiplexed streams, HPACK; negotiated via ALPN over TLS; Go serves it automatically with TLS enabled.",
				"HTTP/3 — QUIC over UDP; no TCP head-of-line blocking, faster handshakes; Go support lives outside the stdlib (x/net and quic-go)."),
			text("ver-facts",
				"The versions topic would be armchair theory without one anchor: the protocol a request ACTUALLY negotiated is a field on every request. The headers demo's facts carry it — curl over plain HTTP in dev yields ``HTTP/1.1``; a browser over TLS in production would report ``HTTP/2`` — and the struct the handler fills makes that explicit, which is why the demo belongs to this section as much as to the headers one:"),
			code("ver-facts-struct", "go",
				"r.Proto is the negotiated version — the demo reports it per request.",
				"server/internal/domain/demos/http_headers.go",
				"type HTTPRequestFacts struct {\n"+
					"    Method    string\n"+
					"    Path      string\n"+
					"    Query     string // raw query string, no leading '?'\n"+
					"    Proto     string // negotiated protocol, e.g. \"HTTP/1.1\"\n"+
					"    // …\n"+
					"    Header    http.Header\n"+
					"}"),
			text("ver-coda",
				"For a Go API author the versions stack is almost entirely absorbed: semantics are identical across 1.1/2/3, the server negotiates for you, and what you actually own are the connection-lifecycle settings — the read, write, and idle timeouts on the ``http.Server`` you saw in the TLS section. Write handlers that never assume a protocol detail, and the version becomes an operations fact instead of a code fact."),
		),
		// ── 9. Terminal companion ─────────────────────────────────────
		section("terminal-companion", "Terminal companion: the curl tour",
			"The protocol, read from the shell — headers first.",
			text("term-pitch",
				"With the stack up (server on :8100), these commands turn this topic into something you can SEE. ``curl -v`` is the best HTTP teacher available: it prints the exact request line and headers it sent and the exact status and headers the server answered — the same facts the demo classifies, verbatim. Piping to ``python3 -m json.tool`` pretty-prints JSON with zero extra tooling:"),
			code("term-verbose", "bash", "", "",
				"# the whole exchange on your terminal: request line, headers, response\n"+
					"curl -sv localhost:8100/api/health"),
			code("term-echo", "bash", "", "",
				"# the headers demo: echo + classification of what YOU just sent\n"+
					"curl -s localhost:8100/api/demo/http/headers | python3 -m json.tool"),
			code("term-custom", "bash", "", "",
				"# teach it something: negotiation, CORS material, credentials, an extension header\n"+
					"curl -s -H 'Accept: application/json' -H 'Origin: http://localhost:5175' \\\n"+
					"  -H 'Authorization: Bearer demo-token' -H 'X-Custom: hello-headers' \\\n"+
					"  localhost:8100/api/demo/http/headers | python3 -m json.tool"),
			code("term-hop-by-hop", "bash", "", "",
				"# a hop-by-hop header, classified on arrival (scope: hop-by-hop)\n"+
					"curl -s -H 'Connection: keep-alive' -H 'Upgrade: websocket' \\\n"+
					"  localhost:8100/api/demo/http/headers | python3 -m json.tool"),
			code("term-405", "bash", "", "",
				"# wrong verb: truthful 405 + Allow header + the shared error envelope\n"+
					"curl -si -X POST localhost:8100/api/demo/http/headers"),
			code("term-404", "bash", "", "",
				"# unknown path: the 404 envelope with a request id you can grep in the logs\n"+
					"curl -si localhost:8100/api/content/nope | python3 -m json.tool"),
			text("term-coda",
				"Everything on this tour is this topic in miniature: request line and headers you can read, statuses that tell the truth, one error envelope shape for every failure, and a request id stitching client, response, and server log together. The next topic drops one level lower — into Go's own ``net/http`` and ServeMux — and builds the server side of everything you just exercised."),
		),
	}

	if buildErr != nil {
		return Topic{}, buildErr
	}
	return NewTopic(
		"http-web",
		"HTTP and the web",
		"The protocol this server speaks — a request's full journey, the vocabulary of "+
			"methods, statuses, and headers, and the rules every API inherits.",
		"foundations",
		3,
		sections,
	)
}
