package content

import (
	"errors"
	"fmt"
)

// This file is topic 30 error-handling (group platform, curriculum
// order 30): what happens when things go wrong, taught from the code
// this server already runs. Go's stance first — errors are values,
// panics are for programmer bugs — then the machinery every layer
// shares: wrapping with %w and matching with errors.Is/As, sentinel
// versus typed errors and when each earns its keep, the eight-kind
// taxonomy and the ONE error envelope the central writer serves, the
// panic policy the recover middleware enforces, failure injection
// against the live scenario store (demo errors:explorer walks the
// probe that honors it — error injection panics into the recovered
// 500, latency injection sleeps), what a failure leaves in the slog
// JSON log (one request id threading client, envelope, and lines), and
// the process nets the composition root already runs (timeouts,
// bounded shutdown). The terminal companion tours all of it with curl.

// errorHandlingTopic builds topic 30.
func errorHandlingTopic() (Topic, error) {
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
		// ── 1. Errors are values ───────────────────────────────────
		section("errors-are-values", "Errors are values: Go's stance, and what it costs you",
			"Go has no exceptions. A failure is an ordinary value the failing function returns, the caller inspects, and the language treats like any other data — the consequence is a discipline, not a mechanism.",
			text("eh-stance",
				"The go-language topic introduced the shape; this is the platform-level truth. In Go, an error IS a value — any type implementing one method, ``Error() string`` — and a function that can fail says so in its signature: ``(T, error)``. Nothing unwinds. Nothing interrupts control flow. The failure travels the same path as the data, which means every caller along the way makes an explicit, visible decision: handle it, wrap it with context, or return it upward. That is the cost people mean when they say Go error handling is verbose — the language refuses to make the failure path invisible — and it is also the whole benefit: a reader can find every place a failure is considered by reading the code, not by imagining which frame might throw. This server's ``run()`` composition root returns errors all the way up to ``main``, which logs one fatal JSON line and exits non-zero — boot failures are values too."),
			text("eh-contract",
				"The convention the standard library sets and this codebase follows: the error is the LAST return value; ``nil`` means success; a non-nil error means the other values are unusable and must not be touched. Constructors and use cases return ``(T, error)``; adapters wrap the library errors they translate (``fmt.Errorf`` with ``%w``, next section) so the boundary's dialect never leaks inward; and nothing is ever swallowed — a function that ignores an error it cannot handle is a bug, full stop. The typed error this server standardizes on carries more than a string: a failure KIND (one of eight, mapped to an HTTP status), a client-safe MESSAGE, structured DETAILS for the cases that deserve them (which fields failed validation), and an optional wrapped CAUSE preserved for logs and matching:"),
			code("eh-typed-error-go", "go",
				"The typed application error — kind, message, details, cause — and the three methods that make it a first-class citizen of errors.Is/As and %w chaining.",
				"server/internal/core/errors.go",
				"type Error struct {\n"+
					"	Kind    Kind\n"+
					"	Message string\n"+
					"	Details any\n"+
					"	cause   error\n"+
					"}\n"+
					"\n"+
					"// Error implements the error interface: \"[kind] message\".\n"+
					"func (e *Error) Error() string {\n"+
					"	return fmt.Sprintf(\"[%s] %s\", e.Kind, e.Message)\n"+
					"}\n"+
					"\n"+
					"// Unwrap returns the wrapped cause, if any (%w-compatible).\n"+
					"func (e *Error) Unwrap() error { return e.cause }\n"+
					"\n"+
					"// Is makes errors.Is compare by kind: any error whose chain holds an *Error\n"+
					"// matches the sentinel for its kind, so callers can test\n"+
					"// errors.Is(err, core.ErrNotFound) without knowing where the error was\n"+
					"// wrapped.\n"+
					"func (e *Error) Is(target error) bool {\n"+
					"	targetErr, ok := target.(*Error)\n"+
					"	return ok && targetErr.Kind == e.Kind\n"+
					"}"),
			text("eh-where-panics",
				"Panics exist, and the stance on them is strict: panics are for PROGRAMMER bugs — the invariant violation that means the code is wrong, not that the world went sideways. A nil map written by a constructor that promised initialization, a type assertion the layering guaranteed, an index the loop bound promised — those may panic, because a recover-and-continue would only hide a broken program. The world going sideways (a database refusing a connection, a user submitting garbage, a store being down) is EXPECTED at runtime and must come back as an error value, because only a value lets the caller choose: retry, degrade, or fail the request honestly. The boundary between the two halves of that sentence is the judgment this topic keeps returning to, and the recovery middleware (later) is the safety net for the days the judgment is wrong."),
			list("eh-decide-list",
				false,
				"Expected condition — a missing row, a bad body, an unreachable store: return an error value; the caller decides retry, degrade, or fail.",
				"Programmer bug — a broken invariant, an impossible state: panic is honest; the net catches it and the fix is a code change.",
				"Boundary translation — library errors crossing into your domain: wrap with %w and your own context; never leak the dialect inward.",
				"Never swallowed — an error nobody handles is a bug; log-and-continue is a decision you must be able to defend.",
				"Client-safe by construction — the message you return is a sentence a client reads; internals ride the cause, not the message.",
			),
		),
		// ── 2. Wrapping and matching ──────────────────────────────
		section("wrapping-and-matching", "Wrapping with %w and matching with errors.Is and errors.As",
			"Errors travel up many frames; the questions callers ask at the top are 'is this the one I care about' and 'what exactly is under it'. Wrapping and matching are the two halves of that conversation.",
			text("eh-wrap-why",
				"An error that just says ``connection refused`` answers nothing a caller needs: WHICH dependency, from WHICH call, while doing WHAT. Wrapping adds that context without destroying the original — ``fmt.Errorf(``\"get post %s: %w\"``, id, err) keeps the cause reachable so the whole chain reads like a story when logged, while the outer message says the sentence a human needs first. The verb is ``%w`` and not ``%s`` on purpose: ``%s`` flattens the cause into a string (the chain is GONE), while ``%w`` wraps it so ``errors.Is`` and ``errors.As`` can walk it. The rule this codebase holds: wrap at LAYER boundaries, where context is added — an adapter wrapping a pgx error, a use case wrapping a repository error — and not at every frame, where it only adds noise."),
			code("eh-wrapf-go", "go",
				"The typed constructors: kind + message + cause in one call — everything a boundary wants to say when it translates a library failure.",
				"server/internal/core/errors.go",
				"// Wrap builds a typed error that carries cause (%w-style) for the\n"+
					"// error's own bookkeeping while presenting message to callers.\n"+
					"func Wrap(cause error, kind Kind, message string) *Error {\n"+
					"	return &Error{Kind: kind, Message: message, cause: cause}\n"+
					"}\n"+
					"\n"+
					"// Wrapf is Wrap with a formatted message.\n"+
					"func Wrapf(cause error, kind Kind, format string, args ...any) *Error {\n"+
					"	return &Error{Kind: kind, Message: fmt.Sprintf(format, args...), cause: cause}\n"+
					"}"),
			text("eh-is",
				"``errors.Is`` answers identity: does this error's chain contain THAT error? The chain is the sequence of ``Unwrap`` calls out of the box, and a type can sharpen the comparison by implementing ``Is(error) bool`` — which is exactly what the typed error above does: any ``*core.Error`` in the chain matches the SENTINEL for its kind, wherever it was wrapped. A handler can therefore ask ``errors.Is(err, core.ErrNotFound)`` without knowing whether the miss came from the in-memory repository or the PostgreSQL adapter, and the answer is true for both — the kind is the contract, the raise site is an implementation detail. The standard library uses the same shape: ``errors.Is(err, context.Canceled)`` and ``errors.Is(err, io.EOF)`` work through every well-behaved wrapper in the ecosystem, which is why %w discipline matters beyond your own code."),
			text("eh-as",
				"``errors.As`` answers type: does this chain hold a value of type T, and can I have it? Where Is asks ``is it that one'', As asks ``is there one of those inside'' — and hands it over so the caller can read its fields. The central error writer uses exactly this: it needs to know whether the error it was handed is a ``*core.Error`` WITH a known kind (then the response carries that kind's status, code, message, and details) — and ``errors.As(err, &appErr)`` finds the typed error even if three layers of %w sit above it. The asymmetry with Is is deliberate: Is is for control flow (should we retry, should we 404), As is for data extraction (what status, what details). Match on identity when you can; reach for the type only when you need what it carries."),
			code("eh-writer-as-go", "go",
				"The central writer's first move: errors.As finds the typed error under any depth of wrapping, and the known-kinds table decides whether the response is its own or the generic 500.",
				"server/internal/core/envelope.go",
				"	var appErr *Error\n"+
					"	if errors.As(err, &appErr) && knownKinds[appErr.Kind] {\n"+
					"		status := HTTPStatusFor(appErr.Kind)\n"+
					"		if logger != nil {\n"+
					"			logger.Warn(\"error response\",\n"+
					"				\"kind\", string(appErr.Kind),\n"+
					"				\"message\", appErr.Message,\n"+
					"				\"status\", status,\n"+
					"				\"request_id\", requestID,\n"+
					"				\"error\", err.Error(),\n"+
					"			)\n"+
					"		}\n"+
					"		writeEnvelope(w, status, appErr.Kind, appErr.Message, requestID, appErr.Details)\n"+
					"		return\n"+
					"	}"),
		),
		// ── 3. Sentinel versus typed ───────────────────────────────
		section("sentinel-versus-typed", "Sentinel errors versus typed errors: when each",
			"Both styles say 'this specific failure happened'. The difference is how much they carry and how far they travel — and a codebase needs both, each in its place.",
			text("eh-sentinel-what",
				"A SENTINEL is a package-level error value callers compare against: ``io.EOF``, ``context.Canceled``, ``http.ErrAbortHandler``, and in this server the eight kind sentinels below. Sentinels shine when the failure is a NAME — a fact callers branch on — and nothing more: the reader of ``errors.Is(err, io.EOF)`` learns everything needed in one line. They are cheap to declare, cheap to match, and they travel across packages without dragging types along. Their limit is the same property: a sentinel carries no payload. If the failure needs to say WHICH row, WHAT constraint, or WHICH fields failed validation, a bare sentinel cannot — you either wrap it (the wrapped context reaches logs, not Is-comparers) or you graduate to a typed error."),
			code("eh-sentinels-go", "go",
				"The taxonomy's eight sentinels — one per kind, matched by errors.Is through the typed error's Is method, their messages never served.",
				"server/internal/core/errors.go",
				"// Package-level kind sentinels for errors.Is comparisons. Their messages\n"+
					"// are never served — the concrete error's message is.\n"+
					"var (\n"+
					"	ErrValidationFailed   = &Error{Kind: KindValidationFailed, Message: \"Validation failed.\"}\n"+
					"	ErrUnauthorized       = &Error{Kind: KindUnauthorized, Message: \"Authentication required.\"}\n"+
					"	ErrForbidden          = &Error{Kind: KindForbidden, Message: \"Access forbidden.\"}\n"+
					"	ErrNotFound           = &Error{Kind: KindNotFound, Message: \"Resource not found.\"}\n"+
					"	ErrConflict           = &Error{Kind: KindConflict, Message: \"Resource conflict.\"}\n"+
					"	ErrRateLimited        = &Error{Kind: KindRateLimited, Message: \"Too many requests.\"}\n"+
					"	ErrServiceUnavailable = &Error{Kind: KindServiceUnavailable, Message: \"Service unavailable.\"}\n"+
					"	ErrInternal           = &Error{Kind: KindInternal, Message: \"Internal server error.\"}\n"+
					")"),
			text("eh-typed-what",
				"A TYPED error is a struct implementing ``error`` whose fields carry the failure's specifics. This server's ``*core.Error`` is the working example: kind, message, details, and a wrapped cause — the HTTP status falls out of the kind, the client-ready message rides its own field, and the cause keeps the pgx/redis/asynq original for the log. Typed errors earn their keep the moment a failure needs STRUCTURE: a validation error naming its fields (the 422's details object), a rate-limit refusal carrying Retry-After, an unavailable dependency naming which store. The constructors make the intent readable at the raise site — ``core.NotFound(``\"post not found\"``)`` says exactly what it means — and the central writer needs only one ``errors.As`` to translate any of them onto the wire:"),
			code("eh-constructors-go", "go",
				"Kind-named constructors — the raise sites read like sentences, and every one maps onto the envelope without handler-side branching.",
				"server/internal/core/errors.go",
				"// New builds a typed error with an explicit kind.\n"+
					"func New(kind Kind, message string) *Error {\n"+
					"	return &Error{Kind: kind, Message: message}\n"+
					"}\n"+
					"\n"+
					"// …\n"+
					"\n"+
					"// ValidationFailed builds the 422 kind with structured details naming what\n"+
					"// failed (details may be nil).\n"+
					"func ValidationFailed(message string, details any) *Error {\n"+
					"	return &Error{Kind: KindValidationFailed, Message: message, Details: details}\n"+
					"}\n"+
					"\n"+
					"// NotFound builds the 404 kind.\n"+
					"func NotFound(message string) *Error { return &Error{Kind: KindNotFound, Message: message} }"),
			list("eh-choosing-list",
				false,
				"Sentinel for a name — the failure is a fact callers branch on (io.EOF, ErrNotFound) and carries no payload.",
				"Typed for structure — the failure must say which row, what fields, what status; the fields ARE the API.",
				"One taxonomy end-to-end — this server has exactly one typed error and eight sentinels for it; no second vocabulary exists.",
				"Sentinels for matching, types for rendering — Is branches on the name; As extracts what the response must carry.",
				"Never a string — error strings are for logs; a bare string compared with == is a sentinel with the type system disabled.",
			),
		),
		// ── 4. One taxonomy, one envelope ─────────────────────────
		section("one-taxonomy-one-envelope", "One taxonomy, one envelope: the eight kinds and the central writer",
			"A codebase with per-handler status picking grows a dialect per author. The fix is structural: a closed set of failure kinds, one kind→status table, and ONE writer every failure passes through.",
			text("eh-taxonomy",
				"The taxonomy is eight kinds — the buckets every failure in this app falls into, no more. Each kind names a wire ``code`` (the string clients switch on) and maps to exactly one HTTP status, in ONE table in one file, so a not_found can never answer 400 on Tuesday and 404 on Friday. Handlers never pick statuses; they return typed errors whose kinds already chose. Two statuses sit OUTSIDE the table's names but not outside the discipline: a 405 method mismatch keeps its truthful status and takes the CLOSEST kind bucket (not_found) — the family rule that the status must tell the truth even when the taxonomy would prefer a tidier bucket — and unknown kinds degrade to 500 exactly like unknown errors do:"),
			code("eh-kinds-go", "go",
				"The eight kinds — the whole vocabulary, declared once.",
				"server/internal/core/errors.go",
				"const (\n"+
					"	KindValidationFailed   Kind = \"validation_failed\"   // 422 (family convention; the express sibling uses 400)\n"+
					"	KindUnauthorized       Kind = \"unauthorized\"        // 401\n"+
					"	KindForbidden          Kind = \"forbidden\"           // 403\n"+
					"	KindNotFound           Kind = \"not_found\"           // 404\n"+
					"	KindConflict           Kind = \"conflict\"            // 409\n"+
					"	KindRateLimited        Kind = \"rate_limited\"        // 429\n"+
					"	KindServiceUnavailable Kind = \"service_unavailable\" // 503\n"+
					"	KindInternal           Kind = \"internal\"            // 500\n"+
					")"),
			code("eh-status-for-go", "go",
				"The mapping and its degradation: one table, one lookup, unknown kinds take 500 like everything else unrecognized.",
				"server/internal/core/errors.go",
				"// HTTPStatusFor maps a failure kind to its HTTP status. Unknown kinds take\n"+
					"// 500 — the same degradation WriteError applies.\n"+
					"func HTTPStatusFor(kind Kind) int {\n"+
					"	if status, ok := httpStatusByKind[kind]; ok {\n"+
					"		return status\n"+
					"	}\n"+
					"	return http.StatusInternalServerError\n"+
					"}"),
			text("eh-envelope",
				"Whatever the kind, the RESPONSE is one shape — the family envelope: ``{\"error\": {code, message, requestId, details?}}`` with the ``x-request-id`` header stamped. One shape means the React client writes its error pipeline exactly once (it did, in Phase 2), and a curl tour can teach all eight kinds as differences of CONTENT, not of form. The writer is central — ``core.WriteError`` — and it is the only code in the server that writes an error body: handlers call it, the recovery layer calls it, the mux-error wrapper calls it. Its degradation is the deepest rule in the file: an error that is NOT a known-kind ``*core.Error`` is a TAXONOMY MISS — the client gets the generic 500 (no internals leak, ever), and the underlying error is logged at error level with the request id, so nothing is ever swallowed, just not shown:"),
			code("eh-write-error-go", "go",
				"The one central error-response writer: known kinds get their own status, code, message, and details at warn; everything else is the taxonomy-miss path — generic 500 out, full error to the log.",
				"server/internal/core/envelope.go",
				"func WriteError(w http.ResponseWriter, r *http.Request, err error, logger *slog.Logger) {\n"+
					"	requestID := RequestIDFromContext(r.Context())\n"+
					"\n"+
					"	// …\n"+
					"\n"+
					"	if logger != nil {\n"+
					"		logger.Error(\"unhandled error\",\n"+
					"			\"request_id\", requestID,\n"+
					"			\"error\", err.Error(),\n"+
					"		)\n"+
					"	}\n"+
					"	writeEnvelope(w, http.StatusInternalServerError, KindInternal, genericInternalMessage, requestID, nil)\n"+
					"}"),
			code("eh-write-envelope-go", "go",
				"The envelope's anatomy, byte for byte: headers, status, and the one body shape every failure answers with.",
				"server/internal/core/envelope.go",
				"// writeEnvelope stamps the shared headers and writes the JSON envelope.\n"+
					"func writeEnvelope(w http.ResponseWriter, status int, kind Kind, message, requestID string, details any) {\n"+
					"	w.Header().Set(\"Content-Type\", \"application/json\")\n"+
					"	w.Header().Set(RequestIDHeader, requestID)\n"+
					"	w.WriteHeader(status)\n"+
					"	body := errorEnvelope{Error: errorBody{\n"+
					"		Code:      string(kind),\n"+
					"		Message:   message,\n"+
					"		RequestID: requestID,\n"+
					"		Details:   details,\n"+
					"	}}\n"+
					"	// An Encode failure means the peer vanished mid-write; there is no\n"+
					"	// recovery action, and the status line already went out.\n"+
					"	_ = json.NewEncoder(w).Encode(body)\n"+
					"}"),
		),
		// ── 5. The panic policy ───────────────────────────────────
		section("the-panic-policy", "The panic policy: programmer bugs only, and the net that catches them",
			"Panics are not error handling — they are the assert that failed. The policy says who may panic, who may not, and what the process does when it happens anyway.",
			text("eh-policy",
				"The policy in one paragraph: DOMAIN and DATA code never panics for world-side failures — unreachable stores, bad input, missing rows are error values — but MAY panic for invariant violations that mean the program itself is wrong (a constructor handed an impossible combination, a slice the caller guaranteed sorted). PRESENTATION code never panics on purpose at all; its job is to catch. And one special value crosses the boundary untouched: ``http.ErrAbortHandler``, net/http's own sentinel for ``the connection is gone'' — re-raised, not recovered, because writing an envelope on a dead connection is both impossible and wrong. When a panic does escape a handler, the recovery middleware — third in the production chain — converts it into the same 500 envelope every other failure uses, and logs the whole stack at error with the request id:"),
			code("eh-recover-go", "go",
				"The recovery layer: panics to the central writer, ErrAbortHandler re-raised untouched, and a response that already started is never double-written.",
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
					"					if writer, ok := w.(interface{ Written() bool }); ok && writer.Written() {\n"+
					"						return\n"+
					"					}\n"+
					"					core.WriteError(w, r, core.Internal(\"Internal server error.\"), logger)\n"+
					"				}\n"+
					"			}()\n"+
					"			next.ServeHTTP(w, r)\n"+
					"		})\n"+
					"	}\n"+
					"}"),
			text("eh-ordering",
				"The chain's ORDER is what makes the net trustworthy, and the composition comment states it as a contract: request-id OUTERMOST, so the logging and recovery layers (and every envelope they write) can stamp and log the same id; the request logger OUTSIDE recovery, so a recovered panic is still logged as a finished request with its true 500 status — a panic that vanished between the two would be an outage with no access-log trace; recovery next; and the mux-error wrapper INNERMOST, converting the stdlib router's own plain-text 404/405 replies into the same envelope so even the framework's failures ride the family shape. One detail in the recover body deserves its own sentence: the ``Written()`` check. If a handler panicked MID-RESPONSE — headers and half a body already on the wire — no envelope can be delivered (the status line is spent), so the layer logs and returns instead of writing on top; honest degradation to the very last byte."),
			code("eh-chain-go", "go",
				"The documented order — the comment IS the contract, and the composition root wires exactly this.",
				"server/internal/presentation/middleware.go",
				"// Chain composes middleware so the FIRST argument is the OUTERMOST layer:\n"+
					"// Chain(A, B, C)(handler) serves as A(B(C(handler))). The composition root\n"+
					"// wires the documented order:\n"+
					"//\n"+
					"//	RequestID(RequestLogger(Recover(EnvelopeMuxErrors(mux))))\n"+
					"//\n"+
					"// Request-id outermost so the logging and recovery layers (and every error\n"+
					"// envelope they write) can stamp and log the same id. The request logger\n"+
					"// sits OUTSIDE recovery so a recovered panic is still logged as a finished\n"+
					"// request with its true 500 status. Recovery converts panics to the 500\n"+
					"// envelope via the central writer; the mux-error wrapper innermost turns\n"+
					"// the stdlib ServeMux's plain-text 404/405 replies into the same envelope.\n"+
					"func Chain(middleware ...Middleware) Middleware {\n"+
					"	return func(final http.Handler) http.Handler {\n"+
					"		for i := len(middleware) - 1; i >= 0; i-- {\n"+
					"			final = middleware[i](final)\n"+
					"		}\n"+
					"		return final\n"+
					"	}\n"+
					"}"),
		),
		// ── 6. Failure injection ──────────────────────────────────
		section("failure-injection", "Failure injection: the scenario store's error/latency split, honored live",
			"You cannot teach failure handling on a stack that never fails. The scenario store's injectFailure toggle is the honest fix — and this topic's probe is the endpoint that honors it for real.",
			text("eh-injection",
				"Since Phase 2 the store has carried a three-value enum, not a bool, and the comment on it names this topic: the teaching DISTINGUISHES error injection (a dependency failing hard — this topic) from latency injection (a dependency going slow — the performance topic). PATCH ``/api/demo/scenario`` flips it at runtime; every demo that wants to can read it. Invalid values never reach the store — the merged-patch guard refuses them with a validation_failed 422 whose details carry the value sent and the values expected, which is itself the taxonomy's first kind firing live:"),
			code("eh-injection-go", "go",
				"The three-value enum (the comment names this topic and the performance one) and the store-side guard that refuses unknown values with the structured 422.",
				"server/internal/core/scenario.go",
				"// FailureInjection arms the demos' failure path. The family wire contract\n"+
					"// (fastapi verbatim) is the three-value enum, not a bool: the teaching\n"+
					"// distinguishes error injection (topics 30) from latency injection\n"+
					"// (topic 39), and the client sends these exact strings.\n"+
					"type FailureInjection string\n"+
					"\n"+
					"// The three legal failure-injection modes.\n"+
					"const (\n"+
					"	FailureNone    FailureInjection = \"none\"\n"+
					"	FailureError   FailureInjection = \"error\"\n"+
					"	FailureLatency FailureInjection = \"latency\"\n"+
					"}\n"+
					"\n"+
					"// …\n"+
					"\n"+
					"	if patch.InjectFailure != nil && !patch.InjectFailure.Valid() {\n"+
					"		return s.current, ValidationFailed(\n"+
					"			\"Invalid scenario patch: unknown failure injection.\",\n"+
					"			map[string]any{\n"+
					"				\"injectFailure\": string(*patch.InjectFailure),\n"+
					"				\"expected\":      []string{string(FailureNone), string(FailureError), string(FailureLatency)},\n"+
					"			},\n"+
					"		)\n"+
					"	}"),
			text("eh-probe",
				"The probe is this demo area's own guarded endpoint, and honoring the toggle is its entire behavior. Under ``error`` it does the Go-honest equivalent of an unhandled failure: it PANICS — deliberately, with a documented sentinel — and lets the production recovery layer do its job, converting the panic into the generic 500 envelope while the stack trace goes to the log and the request id threads both. Under ``latency`` it sleeps out a fixed, bounded delay and then answers healthy — because a slow dependency moves no status code, yet spends the caller's budget all the same, and that difference (the taxonomy catches one, the stopwatch catches the other) is the enum's whole reason for existing. Under ``none`` it answers instantly:"),
			code("eh-probe-go", "go",
				"The probe's whole decision: one snapshot read, a deliberate panic, a bounded sleep, or the healthy row.",
				"server/internal/presentation/demo_errors.go",
				"func (h *ErrorsProbeHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {\n"+
					"	state := h.store.Snapshot()\n"+
					"	switch state.InjectFailure {\n"+
					"	case core.FailureError:\n"+
					"		panic(demos.ErrErrorsProbeInjected)\n"+
					"	case core.FailureLatency:\n"+
					"		h.sleep(demos.ErrorsProbeLatency)\n"+
					"	case core.FailureNone:\n"+
					"	}\n"+
					"	writeJSON(w, http.StatusOK, demos.BuildErrorsProbeAnswer(state))\n"+
					"}"),
			demo("errors:explorer", "The errors explorer: eight kinds, one walk of the probe, live",
				"The taxonomy table (statuses and messages derived from core's own tables, every trigger curl-able or honestly a recipe), the scenario toggle's three modes, and a LIVE walk: the explorer runs its own probe through the production middleware chain under whatever injection is armed RIGHT NOW and reports the status, the body, and the request id that threaded them — flip the scenario and reload to watch the same endpoint answer differently."),
			text("eh-walk",
				"The explorer does not DESCRIBE the probe's behavior — it EXECUTES it. Each request mounts the probe on a private mux behind the same four layers the composition root serves, threads the explorer's own request id in as the inbound header, and walks it; whatever comes back (the healthy row, the recovered 500 envelope, or the post-sleep 200) becomes the payload's walk row, with the note DERIVED from what actually answered rather than what the mode promised. The panel's table is derived the same way — statuses read through ``core.HTTPStatusFor`` and default messages from the sentinels, never restated — so the teaching surface cannot drift from the machinery it describes:"),
			code("eh-walk-go", "go",
				"The walk: the explorer's request id goes in as the inbound header, the probe runs behind the production chain, and the observed facts become the payload.",
				"server/internal/presentation/demo_errors.go",
				"	// Thread the explorer's own request id into the walk as the INBOUND\n"+
					"	// header: the RequestID layer adopts it, the envelope stamps it, and\n"+
					"	// the payload can prove one id served the whole thread.\n"+
					"	probeReq.Header.Set(core.RequestIDHeader, requestID)\n"+
					"\n"+
					"	recorder := newBufferedResponseRecorder()\n"+
					"	h.walk.ServeHTTP(recorder, probeReq)\n"+
					"\n"+
					"	walk := demos.ErrorsWalk{\n"+
					"		InjectFailure: h.store.Snapshot().InjectFailure,\n"+
					"		Status:        recorder.status,\n"+
					"		Elapsed:       h.now().Sub(started),\n"+
					"		RequestID:     requestID,\n"+
					"		Body:          recorder.body.String(),\n"+
					"	}"),
		),
		// ── 7. What logs on failure ───────────────────────────────
		section("what-logs-on-failure", "What a failure leaves in the log: three lines and one request id",
			"The envelope serves the client; the log serves the engineer at 3 a.m. The design goal is that one id — the request id — retrieves everything a failure produced, in order, from one stream.",
			text("eh-three-lines",
				"Every failing request leaves a small, predictable set of slog JSON lines on stdout, and every one of them carries ``request_id``. An ordinary typed failure writes TWO: the ``error response`` warn line from the central writer (kind, message, status, the full error chain) and the ``http_request`` info line the access logger writes at completion (method, path, status, duration_ms). A recovered panic writes THREE, and the ordering the chain guarantees makes them readable as a story: the ``panic recovered`` error line with the recovered value and the full stack, the same ``http_request`` line (the logger sits outside recovery, so the panic is still a finished request with its true 500), and nothing from the writer if the response had already started. A taxonomy miss — an error outside the eight kinds — writes ``unhandled error`` at error level instead of the warn, with the generic 500 still served. Grep the id, read the whole failure."),
			code("eh-access-log-go", "go",
				"The completion line every request writes — panics included, because the logger wraps recovery.",
				"server/internal/presentation/middleware.go",
				"			defer func() {\n"+
					"				durationMs := float64(time.Since(started).Microseconds()) / 1000\n"+
					"				logger.Info(\"http_request\",\n"+
					"					\"method\", r.Method,\n"+
					"					\"path\", r.URL.Path,\n"+
					"					\"status\", rec.status,\n"+
					"					\"duration_ms\", math.Round(durationMs*10)/10,\n"+
					"					\"request_id\", core.RequestIDFromContext(r.Context()),\n"+
					"				)\n"+
					"			}()"),
			text("eh-why-json",
				"The lines are JSON because they are DATA — the observability topic builds on exactly this stream. slog's structured fields mean the log never needs parsing prose to answer ``which path, which status, whose id''; a filter on ``request_id`` is a map lookup, not a regex. The handler is the stdlib's own — one JSON handler over stdout, gated at the configured level — which keeps the sink container-native (``docker logs`` reads it) and the dependency list empty:"),
			code("eh-logger-go", "go",
				"The process logger: stdlib slog, JSON on stdout, level from the typed config.",
				"server/internal/core/logging.go",
				"// NewLogger returns the process logger: JSON records on stdout, gated at\n"+
					"// the configured level. JSON keeps every log line machine-parseable for\n"+
					"// the observability tooling later in the guide, and stdout is the\n"+
					"// container-native sink.\n"+
					"func NewLogger(cfg Config) *slog.Logger {\n"+
					"	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{\n"+
					"		Level: cfg.LogLevel,\n"+
					"	}))\n"+
					"}"),
			text("eh-id-thread",
				"The id itself is born in the outermost layer: the request-id middleware adopts a client-supplied ``x-request-id`` when there is one (trimmed and length-capped, so hostile headers cannot bloat every line) or mints a fresh 16-byte hex id, echoes it on the response, and pushes it into the request CONTEXT where the logger, the recovery layer, and the error writer all read the same value back. That context read is the thread's last link — ``WriteError`` pulls the ambient id at write time, so the envelope's ``requestId``, the response header, and every log line for the request are the same string by construction, not by convention. A client that logs the id it received can hand support ONE value that retrieves the server's whole account of the failure:"),
			code("eh-rid-ctx-go", "go",
				"The context read every failure site shares — the id in the envelope is the id in the log because both come from here.",
				"server/internal/core/requestid.go",
				"// RequestIDFromContext returns the ambient request id, or \"\" outside a\n"+
					"// request (or when the middleware did not run).\n"+
					"func RequestIDFromContext(ctx context.Context) string {\n"+
					"	if ctx == nil {\n"+
					"		return \"\"\n"+
					"	}\n"+
					"	id, _ := ctx.Value(requestIDKey{}).(string)\n"+
					"	return id\n"+
					"}"),
		),
		// ── 8. Process nets ───────────────────────────────────────
		section("process-nets", "Process nets: recovery, timeouts, and the bounded shutdown",
			"Per-request handling is half the story. A server also needs NETS the whole process lives under — bounds that turn pathological cases into honest failures instead of hangs.",
			text("eh-nets",
				"Four nets hold this process, and every one is already wired in the composition root. The RECOVER net converts programmer-bug panics into 500 envelopes (the section before). The TIMEOUT net bounds the server itself: reads and writes are capped so a slow or hostile client cannot hold a connection — and therefore a goroutine — forever; the numbers are deliberately generous for the guide's demos but finite, because every unbounded wait in a server is an outage waiting for traffic. The SHUTDOWN net drains instead of dying: SIGINT/SIGTERM cancel one context, gRPC drains first (its RPCs are short), HTTP follows inside a bounded window, and the background-jobs worker drains last. And the BOOT net fails fast: config, database dial, migrations, and content validation all happen before the port opens, so a broken process never serves a single request it cannot serve honestly."),
			code("eh-timeouts-go", "go",
				"The timeout net — every read and write bounded; nothing waits forever.",
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
			code("eh-shutdown-go", "go",
				"The shutdown net — both surfaces drain in order, HTTP inside its bounded window, the worker last.",
				"server/cmd/guide/main.go",
				"	logger.Info(\"shutting down\")\n"+
					"	// Both surfaces drain in-flight work: gRPC first (its RPCs are short),\n"+
					"	// then HTTP inside its bounded timeout.\n"+
					"	grpcServer.GracefulStop()\n"+
					"	logger.Info(\"grpc server stopped\")\n"+
					"	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)\n"+
					"	defer cancel()\n"+
					"	if err := srv.Shutdown(shutdownCtx); err != nil {\n"+
					"		return fmt.Errorf(\"graceful shutdown: %w\", err)\n"+
					"	}\n"+
					"	logger.Info(\"server stopped\")\n"+
					"\n"+
					"	// The worker drains last: in-flight demo stories finish, then the\n"+
					"	// asynq server and its redis client stand down (idempotent).\n"+
					"	jobsWorker.Shutdown()"),
			text("eh-net-philosophy",
				"The philosophy under all four nets is the same sentence this topic opened with, applied to the process: a failure is a VALUE, so make it a bounded, observable one. A panic becomes a logged 500, not a corpse. A slow client becomes a timed-out connection, not a leaked goroutine. A SIGTERM becomes a drained shutdown with a deadline, not a half-written response. And an un-bootable process becomes a non-zero exit with one fatal JSON line, not a listener serving lies. Nothing here handles a failure by making it disappear — every net's job is to convert the unbearable shape (hang, crash, leak, zombie) into the bearable one this topic spent its sections teaching: a typed, logged, request-id-carrying error value that somebody can act on."),
		),
		// ── 9. Terminal companion ─────────────────────────────────
		section("terminal-companion", "Terminal companion: the error-handling curl tour",
			"Every row of the explorer's table is curl-able (three honestly are not, and say so) — and the failure-injection round trip is three commands: arm it, call the probe, disarm it.",
			text("term-intro",
				"With the stack up (server on :8100), start with the explorer itself: one GET answers the taxonomy table with every trigger, the scenario panel, and a walk row reporting what the probe answered under the CURRENT injection (nothing, by default). Then run the injection round trip in the second block — arm error injection with one PATCH, call the probe and watch the generic 500 envelope arrive with the SAME request id in the header and the body, disarm, and call again for the healthy 200. The third block fires the live table rows for real: the 422 with its structured details, the uniform 401, the plain 404, and the rate limiter's 429 after the burst drains. Leave the scenario at ``none`` when you finish — the next reader of any demo deserves a healthy stack."),
			code("term-injection-bash", "bash", "", "",
				"# the taxonomy table + the live walk, one request\n"+
					"curl -s localhost:8100/api/demo/errors/explorer | python3 -m json.tool | head -50\n"+
					"\n"+
					"# the injection round trip: arm, fail, disarm, heal\n"+
					"curl -s -X PATCH localhost:8100/api/demo/scenario \\\n"+
					"  -H 'content-type: application/json' -d '{\"injectFailure\": \"error\"}'\n"+
					"curl -si localhost:8100/api/demo/errors/probe | head -12\n"+
					"#   HTTP/1.1 500 ...  {\"error\":{\"code\":\"internal\",\n"+
					"#   \"message\":\"Internal server error.\",\"requestId\":\"…\"}}\n"+
					"#   — and the server log holds the panic-recovered line with the SAME id\n"+
					"curl -s -X PATCH localhost:8100/api/demo/scenario \\\n"+
					"  -H 'content-type: application/json' -d '{\"injectFailure\": \"none\"}'\n"+
					"curl -si localhost:8100/api/demo/errors/probe | head -6   # 200, healthy\n"+
					"\n"+
					"# latency injection: no status moves, the stopwatch catches it\n"+
					"curl -s -X PATCH localhost:8100/api/demo/scenario \\\n"+
					"  -H 'content-type: application/json' -d '{\"injectFailure\": \"latency\"}'\n"+
					"time curl -s localhost:8100/api/demo/errors/probe          # ~1.5s, still 200\n"+
					"curl -s -X PATCH localhost:8100/api/demo/scenario \\\n"+
					"  -H 'content-type: application/json' -d '{\"injectFailure\": \"none\"}'"),
			code("term-kinds-bash", "bash", "", "",
				"# validation_failed — the store's own guard, details carry expected\n"+
					"curl -s -X PATCH localhost:8100/api/demo/scenario \\\n"+
					"  -H 'content-type: application/json' -d '{\"injectFailure\": \"bogus\"}' | python3 -m json.tool\n"+
					"\n"+
					"# unauthorized — the guarded whoami with no credential at all\n"+
					"curl -si localhost:8100/api/demo/auth/whoami | head -8\n"+
					"\n"+
					"# not_found — a well-formed id that names nothing\n"+
					"curl -si localhost:8100/api/posts/no-such-post | head -8\n"+
					"\n"+
					"# rate_limited — a tight loop spends the burst (5 at a 10/s refill);\n"+
					"# the boundary lands mid-loop and Retry-After rides the 429\n"+
					"for i in $(seq 1 12); do\n"+
					"  curl -s -o /dev/null -w '%{http_code} ' localhost:8100/api/demo/security/lab\n"+
					"done   # 200 200 200 200 200 200 200 429 429 … — where the 429s start shifts\n"+
					"       # with loop speed (refill runs while you hammer); it self-heals in a second"),
			text("term-coda",
				"Read the tour as the topic in miniature. The injection round trip is the panic policy and the recovery net proven on a live port: the probe's deliberate panic never reaches the client as a crash — it arrives as the same envelope every other failure uses, with a request id that retrieves the panic-recovered line and its stack from the log. The kinds block is the taxonomy's promise kept: four different failures, four different statuses, ONE body shape, zero handler-side status picking anywhere in the server. And the latency leg is the enum's reason: the response is ``healthy'' in every way except the second and a half it cost — the failure mode the taxonomy cannot name and the stopwatch must. Error handling in Go is not a section of code; it is the shape this whole server returns its world in — values, wrapped and matched, mapped onto one envelope, caught by nets, and never once swallowed."),
		),
	}

	if buildErr != nil {
		return Topic{}, buildErr
	}
	return NewTopic(
		"error-handling",
		"Error handling in Go: values, wrapping, one envelope, and the nets underneath",
		"Go's stance — errors are values, panics are for programmer bugs — made operational: "+
			"wrapping with %w and errors.Is/As, one taxonomy mapped to one envelope by the "+
			"central writer, and the process nets of recover, timeouts, and fail-fast boot.",
		"platform",
		30,
		sections,
	)
}
