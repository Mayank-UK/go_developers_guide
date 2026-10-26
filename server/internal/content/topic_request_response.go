package content

import (
	"errors"
	"fmt"
)

// This file is topic 09 request-response (group "request-handling",
// order 9): the two things every handler moves — the request body in,
// and the status/headers/body (or stream) out. Every code block quotes
// a real repo file: the scenario PATCH handler's production decoder
// discipline, the shared writeJSON success writer, the middleware
// chain's flush forwarding and Allow preservation, and this topic's own
// demo builders (the primary sources, per family convention). The two
// live demos (reqres:inspector, reqres:methods) sit INLINE in the
// sections that teach them, and the terminal companion closes the page.

// requestResponseTopic builds topic 09 — reading bodies with json.Decoder
// discipline, writing responses (status, headers, cookies), flushing and
// SSE basics, content negotiation, and redirects. The two live demos are
// performed by real builders in internal/domain/demos and pinned by their
// own tests.
func requestResponseTopic() (Topic, error) {
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
		// ── 1. json.Decoder: streaming bodies ──────────────────────────
		section("json-decoder-streaming-bodies", "json.Decoder: streaming bodies",
			"The request body is hostile input arriving as a stream — the decoder's contract decides what survives contact with a Go struct.",
			text("dec-pitch",
				"Every handler in this guide answers the same two questions: what did the caller send, and what should we send back. "+
					"Topic 04 covered the handler's shape and the mux that routes to it; this topic goes one layer deeper into the "+
					"payload itself — the request body as a stream to decode with discipline, and the response as a status line, a "+
					"header set, and a body written in the right order. The request side comes first because it is where untrusted "+
					"input enters: everything the wire can carry — truncated JSON, unknown fields, wrong-typed fields, two documents "+
					"jammed into one body — arrives here, and what the decoder does with it is the difference between a typed struct "+
					"you can trust and a bag of maybe."),
			text("dec-parsed-for-you",
				"net/http parses the request line, the query, and the headers before any handler code runs — ``r.Method``, "+
					"``r.URL.RawQuery``, and ``r.Header`` arrive as already-typed data. The body is the deliberate exception: it "+
					"comes through as ``r.Body``, an ``io.Reader`` the handler consumes exactly once. Go offers two decoding "+
					"postures over that reader. ``json.Unmarshal`` wants a ``[]byte``, which means slurping the whole body into "+
					"memory first — fine for small, trusted payloads, careless at a public boundary. ``json.NewDecoder`` wraps the "+
					"reader itself and streams: the decoder pulls bytes as it parses, one JSON value at a time, and that streaming "+
					"posture is the one this guide's handlers use on every body they accept."),
			text("dec-discipline-pitch",
				"The decoder alone is not yet a discipline; the discipline is the small set of rules wrapped around it. This "+
					"topic's request-side demo builder owns it in its smallest form — the exact function every exhibit body and "+
					"every live body passes through — and it is worth reading line by line, because each line is one rule: stream "+
					"the body, reject unknown fields, stop at the first offense, and require the stream to hold exactly one JSON "+
					"value:"),
			code("dec-discipline", "go",
				"The demo's decoder discipline: stream, reject the unknown, first error, one value.",
				"server/internal/domain/demos/reqres_inspector.go",
				"decoder := json.NewDecoder(bytes.NewReader(body))\n"+
					"decoder.DisallowUnknownFields()\n"+
					"var value reqresBody\n"+
					"if err := decoder.Decode(&value); err != nil {\n"+
					"    return reqresVerdict{\n"+
					"        Value:         value,\n"+
					"        Err:           err,\n"+
					"        UnexpectedEOF: errors.Is(err, io.ErrUnexpectedEOF),\n"+
					"    }\n"+
					"}\n"+
					"var trailing any\n"+
					"if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {\n"+
					"    return reqresVerdict{Value: value, Trailing: true}\n"+
					"}\n"+
					"return reqresVerdict{Value: value}"),
			text("dec-disallow-unknown-fields",
				"``DisallowUnknownFields`` is the strictest of those rules and a deliberate choice. With it, a body carrying a "+
					"key the struct does not know is rejected instead of silently dropped — a typo'd ``titel`` fails loudly at "+
					"the boundary instead of leaving the real field at its zero value to surface three layers later. The cost is "+
					"honest too: strictly versioned APIs reject clients that send newer fields, so endpoints that must tolerate "+
					"forward-compatible clients switch the rule OFF and ignore unknown keys knowingly. This guide's own scenario "+
					"PATCH handler takes the tolerant stance; the demo takes the strict one — both postures are on the table with "+
					"their trade-offs visible, which is exactly what a teaching server is for."),
			text("dec-first-error",
				"First-error reporting is the rule that surprises people arriving from frameworks that collect every validation "+
					"failure into one report. ``encoding/json`` stops at the FIRST offense — one error value, verbatim — and "+
					"what the decoder already assigned stays assigned. The demo's exhibits make this observable. One body carries "+
					"an unknown field BEFORE a wrong-typed field, and the verdict names ``urgent``, never the type error: "+
					"decoding stopped. That body's decoded state still shows the earlier field set — ``title`` landed before the "+
					"offense. Truncation behaves differently in a way worth knowing cold: the decoder scans one COMPLETE JSON "+
					"value before any field is assigned, so a body cut off mid-array decodes NOTHING — not a partial object, a "+
					"zero value and ``unexpected EOF``, checked with ``errors.Is`` against ``io.ErrUnexpectedEOF`` rather than "+
					"by string matching."),
			text("dec-cap-pitch",
				"Streaming does not mean unbounded. A public endpoint that reads a body needs a size ceiling decided BEFORE the "+
					"first byte is read, or a hostile client can make the process allocate until it dies. The production pattern "+
					"in this repo is the scenario PATCH handler: ``http.MaxBytesReader`` wraps the request body so the read itself "+
					"fails past the limit, and the same one-value discipline sits behind the cap — decode into a raw message, "+
					"then insist the stream is exhausted:"),
			code("dec-maxbytes", "go",
				"The scenario PATCH handler's real discipline: a capped stream and exactly one JSON value.",
				"server/internal/presentation/scenario.go",
				"decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxScenarioBodyBytes))\n"+
					"var raw json.RawMessage\n"+
					"if err := decoder.Decode(&raw); err != nil {\n"+
					"    return invalid(err.Error())\n"+
					"}\n"+
					"// Exactly one JSON value: trailing content (a second object, stray\n"+
					"// tokens) means the body is not one patch document.\n"+
					"var trailing any\n"+
					"if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {\n"+
					"    return invalid(\"body must contain exactly one JSON object\")\n"+
					"}"),
			text("dec-handler-pitch",
				"The demo's own endpoint takes the reporting stance instead of the rejecting one — its job is to show the "+
					"caller what arrived, including a body that blew past the cap. The handler reads through an "+
					"``io.LimitReader`` parked one byte past the limit: a body within the cap arrives whole, a larger one "+
					"arrives truncated at exactly the cap with a flag set, and the pure builder renders the verdict on the "+
					"bytes that made it. Facts out of the request, verdicts out of the builder, one JSON write — the thin "+
					"handler shape the family invariant asks for:"),
			code("dec-facts", "go",
				"The demo handler: cap first, then facts into the pure builder.",
				"server/internal/presentation/demo_request_response.go",
				"capped, err := io.ReadAll(io.LimitReader(r.Body, demos.MaxReqresBodyBytes+1))\n"+
					"if err != nil {\n"+
					"    // Partial bytes are the honest remainder of a vanished peer.\n"+
					"    capped = capped[:min(len(capped), demos.MaxReqresBodyBytes)]\n"+
					"}\n"+
					"exceeded := len(capped) > demos.MaxReqresBodyBytes\n"+
					"if exceeded {\n"+
					"    capped = capped[:demos.MaxReqresBodyBytes]\n"+
					"}\n"+
					"writeJSON(w, http.StatusOK, demos.BuildReqresInspectorPayload(demos.ReqresInspectorFacts{\n"+
					"    Method:            r.Method,\n"+
					"    Path:              r.URL.Path,\n"+
					"    ContentType:       r.Header.Get(\"Content-Type\"),\n"+
					"    Body:              capped,\n"+
					"    BodyLimitExceeded: exceeded,\n"+
					"}))"),
			list("dec-rules", false,
				"Stream, don't slurp — json.NewDecoder over the body reader parses as it reads; the handler never holds bytes it did not need.",
				"Reject or consciously ignore — DisallowUnknownFields turns unknown keys into rejections; tolerating them is a versioning decision, never an accident.",
				"First error only — decoding stops at the first offense; report it verbatim and let the error envelope carry it (topic 10 builds the 422 shape).",
				"One value per body — after a successful Decode, a second Decode must return io.EOF; trailing content is two documents, not one.",
				"Cap before you read — MaxBytesReader or a LimitReader bounds the memory a body can cost before the first byte moves."),
			demo("reqres:inspector", "The body decoder discipline — four bodies, four verdicts",
				"Performed on every request: the live GET's own facts (method, path, content type, body size against the "+
					"4096-byte cap) beside the decoder discipline's deterministic verdicts over four pinned exhibit bodies — "+
					"valid, unknown-field-before-a-type-error, truncated mid-array, wrong-typed field — each with the error "+
					"verbatim, its concrete Go type (*json.UnmarshalTypeError, *errors.errorString), the offending field, and "+
					"the decoded state, partial state included. A live beat applies the same function to whatever body THIS "+
					"request carried; send one and the server judges your bytes with the same honesty as the exhibits."),
			text("dec-read-demo",
				"Read the demo in four beats: the request facts, this very GET's method, path, content type, and body size "+
					"against the cap the handler enforces; the discipline stated as rules you can render beside the verdicts; "+
					"the four exhibit bodies with their deterministic outcomes — accepted, unknown-field-first-error, truncated-"+
					"with-nothing-decoded, wrong-typed with the field named; and the live beat, which usually reports outcome "+
					"``none`` because a plain GET carries no body — absence stated, never guessed around. The terminal "+
					"companion shows how to put a body ON the GET and watch the live row switch to a real verdict."),
			text("dec-coda",
				"What the decoder rejects becomes an error value, and error values travel: the scenario handler converts its "+
					"decode failures into the 422 validation_failed envelope, and topic 10 makes that mapping a whole topic. "+
					"The request side of this topic ends here — from this point on everything is the server's own voice: the "+
					"status line, the headers, the body, and the exact order they must be written in."),
		),
		// ── 2. Writing responses: status, headers, body ────────────────
		section("writing-responses", "Writing responses: status, headers, body",
			"One interface, three methods, and a fixed order — the status line is a one-way door.",
			text("resp-writer",
				"The response side is one interface with three methods, and the ORDER around them is the whole lesson. "+
					"``http.ResponseWriter`` hands the handler a header map (``Header()``), a status line (``WriteHeader``), "+
					"and the body stream (``Write``). The rules are fixed by net/http: headers must be set before the first "+
					"byte goes out; ``WriteHeader`` takes effect exactly once — the first call wins, later calls are silently "+
					"dropped; and calling ``Write`` before ``WriteHeader`` implies a 200. This repo's handlers never touch "+
					"that raw triple directly for JSON — one shared writer does the three steps in the same order every "+
					"time:"),
			code("resp-writejson", "go",
				"Every JSON response in this guide: one content type, one status, one encoded body.",
				"server/internal/presentation/content.go",
				"func writeJSON(w http.ResponseWriter, status int, v any) {\n"+
					"    w.Header().Set(\"Content-Type\", \"application/json\")\n"+
					"    w.WriteHeader(status)\n"+
					"    // An Encode failure means the peer vanished mid-write; there is no\n"+
					"    // recovery action, and the status line already went out.\n"+
					"    _ = json.NewEncoder(w).Encode(v)\n"+
					"}"),
			text("resp-implicit-200",
				"The implicit 200 is convenient and quietly dangerous: a handler that forgets ``WriteHeader`` still answers "+
					"200, so an error path that forgets its status lies to the client with a success code. The family "+
					"discipline is to write the status explicitly on every path — the success writer and the central error "+
					"writer both do — and to treat the status line as a one-way door: after the first byte there is no taking "+
					"an error back, which is why ``writeJSON`` ignores the encoder's failure with a comment saying exactly "+
					"that. The peer is gone; the log already knows; nothing else is true."),
			text("resp-exhibit-pitch",
				"Status codes are claims, and the claim set worth making is small. The response-side demo's exhibit handler "+
					"is a complete multi-method resource in one switch — real executable code, quoted here with the duller "+
					"cases elided — where every status is chosen for what it asserts: creation, replacement, acceptance-for-"+
					"later, head-only parity, capability, and refusal:"),
			code("resp-switch", "go",
				"The multi-method exhibit: one switch, one deliberate status per method.",
				"server/internal/domain/demos/reqres_methods.go",
				"func ServeReqresExhibit(w http.ResponseWriter, r *http.Request) {\n"+
					"    w.Header().Set(\"X-Reqres-Method\", r.Method)\n"+
					"    switch r.Method {\n"+
					"    case http.MethodGet:\n"+
					"        // … headers and the session-style cookie, quoted in the next section\n"+
					"        w.WriteHeader(http.StatusOK)\n"+
					"        fmt.Fprintf(w, `{\"resource\":\"methods-exhibit\",\"state\":\"steady\"}`)\n"+
					"    case http.MethodPost:\n"+
					"        w.Header().Set(\"Content-Type\", \"application/json\")\n"+
					"        w.Header().Set(\"Location\", ReqresExhibitPath+\"/17\")\n"+
					"        w.WriteHeader(http.StatusCreated)\n"+
					"        fmt.Fprintf(w, `{\"id\":17,\"state\":\"created\"}`)\n"+
					"    // … PUT, PATCH, DELETE, HEAD, OPTIONS: same shape, one status each\n"+
					"    default:\n"+
					"        w.Header().Set(\"Allow\", ReqresAllowValue)\n"+
					"        w.WriteHeader(http.StatusMethodNotAllowed)\n"+
					"        fmt.Fprintf(w, `{\"error\":%q,\"served\":%q}`, r.Method, ReqresAllowValue)\n"+
					"    }\n"+
					"}"),
			list("resp-statuses", false,
				"200 OK — the request succeeded and this body is the answer; the default that should still be written explicitly.",
				"201 Created — a new resource exists; the Location header is not decoration, it is half the contract.",
				"204 No Content — success with nothing to say; a body here is a protocol violation, so none is written.",
				"202 Accepted — the request is valid and scheduled, not done; the honest status for work that happens after the reply.",
				"405 Method Not Allowed — the resource exists, the method does not; Allow states what does, truthfully."),
			text("resp-observed",
				"A status claim is only as good as its verification, which is why the demo's payload does not merely claim. "+
					"The exhibit runs over ``httptest.NewRecorder`` in-process for all eight methods — seven served plus a "+
					"real-but-unserved TRACE — and every row the payload carries is read back off the recorder: the status, "+
					"the phrase, the selected headers, the exact bytes. The pair of rules the observation catches and prose "+
					"cannot: a 204 must carry no body, and a 405 must carry ``Allow``. Both fail the demo's own tests if "+
					"regressed, which is what 'observed, not claimed' buys."),
			text("resp-coda",
				"One more detail the exhibit leans on: the status PHRASE is the standard library's to render — "+
					"``http.StatusText`` — never hand-typed into a payload. Statuses, headers, and their order are now set; "+
					"the next section is about the headers themselves, including the one header that is really a small "+
					"attribute grammar: Set-Cookie."),
		),
		// ── 3. Headers and cookies ─────────────────────────────────────
		section("headers-and-cookies", "Headers and cookies",
			"A canonicalized map you must fill before the status line — and the one header with its own helper.",
			text("hdr-map",
				"``w.Header()`` returns an ``http.Header`` — a map of canonical names to value slices — and the "+
					"canonicalization matters: ``Set`` normalizes the key (``content-type`` and ``CONTENT-TYPE`` both land "+
					"on ``Content-Type``), ``Add`` appends a second value where ``Set`` would replace, and the map must be "+
					"filled before ``WriteHeader`` freezes it into the response — anything set after the first byte has "+
					"left goes nowhere. The exhibit's GET case shows the full order: a content type, a cache directive, "+
					"and a cookie planted through the one header that gets a dedicated helper because its value is not a "+
					"value but a small grammar:"),
			code("hdr-cookie", "go",
				"The GET case: headers in order, then a cookie through its dedicated helper.",
				"server/internal/domain/demos/reqres_methods.go",
				"case http.MethodGet:\n"+
					"    w.Header().Set(\"Content-Type\", \"application/json\")\n"+
					"    w.Header().Set(\"Cache-Control\", \"no-store\")\n"+
					"    http.SetCookie(w, &http.Cookie{\n"+
					"        Name:     \"reqres_visit\",\n"+
					"        Value:    \"1\",\n"+
					"        Path:     \"/api/demo/reqres\",\n"+
					"        HttpOnly: true,\n"+
					"        SameSite: http.SameSiteLaxMode,\n"+
					"    })\n"+
					"    w.WriteHeader(http.StatusOK)"),
			text("hdr-cookie-anatomy",
				"``http.SetCookie`` serializes the ``http.Cookie`` struct into that grammar — name=value followed by "+
					"attributes that change what the browser does with it — and the struct mirrors the attributes one-to-"+
					"one. ``Path`` scopes where the cookie rides back, ``HttpOnly`` hides it from page JavaScript (the XSS "+
					"damper), ``Secure`` confines it to TLS, and ``SameSite`` decides whether cross-site requests carry it "+
					"at all — ``Lax``, the exhibit's choice, allows top-level navigations and blocks the cross-site POST "+
					"that CSRF needs. Inbound cookies arrive pre-parsed on the other side: ``r.Cookie(name)`` returns one "+
					"``*http.Cookie``, ``r.Cookies()`` returns all of them — no header string surgery anywhere in Go."),
			list("hdr-cookie-attrs", false,
				"Path — the URL prefix the cookie rides back on; the exhibit scopes its cookie to /api/demo/reqres so it never leaks to other demos.",
				"HttpOnly — the value is invisible to document.cookie; script cannot exfiltrate what script cannot read.",
				"Secure — the browser sends the cookie over TLS only; plain HTTP drops it silently.",
				"SameSite — Lax (top-level navigations), Strict (same-site only), or None (cross-site, requires Secure); this is the CSRF dial.",
				"Max-Age / Expires — persistence; absent means a session cookie that dies with the browser session.",
				"Domain — which hosts see it; defaults to the exact origin, and widening it is usually a mistake."),
			text("hdr-allow-pitch",
				"Some headers are the protocol's own metadata rather than the application's, and the load-bearing one for "+
					"this topic is ``Allow``: the 405 response's statement of what the resource DOES serve. RFC 9110 "+
					"requires it, and this repo's mux-error envelope goes out of its way to keep it — when the stdlib "+
					"mux answers 405 in plain text, the interceptor buffers that body and replaces it with the shared "+
					"error envelope while the real writer's headers, Allow above all, ride through untouched:"),
			code("hdr-allow", "go",
				"The 405's Allow header survives envelope rewriting — headers ride the real writer's map.",
				"server/internal/presentation/middleware.go",
				"canned := slices.Contains(muxCannedBodies[i.status], i.buf.String())\n"+
					"if !canned {\n"+
					"    i.w.WriteHeader(i.status)\n"+
					"    _, _ = i.w.Write(i.buf.Bytes())\n"+
					"    return\n"+
					"}\n"+
					"if i.status == http.StatusNotFound {\n"+
					"    core.WriteError(i.w, r, core.NotFound(\"Not Found\"), logger)\n"+
					"    return\n"+
					"}\n"+
					"core.WriteErrorStatus(i.w, r, i.status, core.New(core.KindForHTTPStatus(i.status), \"Method Not Allowed\"), logger)"),
			text("hdr-coda",
				"Cookies as planted here are stateless bookkeeping; the moment a cookie carries a session id with a store "+
					"behind it, authentication begins — topic 23 builds that whole loop (mint, store, rotate, revoke) on "+
					"exactly these primitives. For this topic, the exhibit's ``reqres_visit`` cookie is enough to watch "+
					"``Set-Cookie`` cross the wire and come back on the next request, which is the entire round trip in "+
					"one observable header."),
		),
		// ── 4. http.Flusher and SSE basics ─────────────────────────────
		section("flushing-and-sse", "http.Flusher and SSE basics",
			"Written is not sent — flushing is what moves bytes, and Server-Sent Events is the convention built on it.",
			text("sse-why",
				"Everything so far assumed the response is written once, at the end. Streaming flips that: a server-sent-"+
					"events response holds the connection open and pushes events as they happen — live logs, progress, "+
					"ticks. HTTP has no push primitive, so SSE is a convention over a long-lived body: the content type "+
					"``text/event-stream``, a body of newline-framed event blocks, and a client (``EventSource`` in the "+
					"browser, ``curl -N`` in the terminal) that renders each block as it lands. The one server-side "+
					"requirement none of that description mentions: the bytes must actually MOVE when an event is "+
					"finished, which is what flushing is for."),
			text("sse-buffering",
				"By default net/http buffers response writes and streams them as chunked encoding when the handler "+
					"finishes — efficient for whole responses, fatal for streaming, because an event sitting in a buffer "+
					"has not been sent. ``http.Flusher`` is the escape hatch: ``Flush`` pushes everything written so far "+
					"onto the wire. The wrinkle this repo already solved is that flushing must survive WRAPPING — a "+
					"middleware that embeds ``http.ResponseWriter`` hides the underlying writer's Flush from a type "+
					"assertion, so the chain's status recorder forwards it explicitly (and implements ``Unwrap`` for "+
					"``http.ResponseController``, the modern way to reach through):"),
			code("sse-chain-flush", "go",
				"Flushing through the chain: the logging recorder forwards Flush (and Unwrap) to the real writer.",
				"server/internal/presentation/middleware.go",
				"func (r *statusRecorder) Flush() {\n"+
					"    if flusher, ok := r.ResponseWriter.(http.Flusher); ok {\n"+
					"        flusher.Flush()\n"+
					"    }\n"+
					"}"),
			text("sse-beat-pitch",
				"The demo's beat writer is the whole pattern in fifteen lines — set the event-stream content type, then "+
					"loop: write one event block, flush, repeat. A bounded, named event count like every fan-out in this "+
					"guide; a guarded flush that degrades to unflushed-but-written bytes if the writer cannot flush; and "+
					"no timing values anywhere, which is why the demo's payload is byte-for-byte deterministic:"),
			code("sse-beat", "go",
				"The SSE beat: one event block, one flush, three times — synchronously.",
				"server/internal/domain/demos/reqres_methods.go",
				"w.Header().Set(\"Content-Type\", \"text/event-stream\")\n"+
					"w.Header().Set(\"Cache-Control\", \"no-store\")\n"+
					"w.WriteHeader(http.StatusOK)\n"+
					"for sequence := 1; sequence <= reqresSSEEventCount; sequence++ {\n"+
					"    fmt.Fprintf(w, \"event: beat\\ndata: {\\\"sequence\\\":%d,\\\"state\\\":\\\"tick\\\"}\\n\\n\", sequence)\n"+
					"    if flusher, ok := w.(http.Flusher); ok {\n"+
					"        flusher.Flush()\n"+
					"    }\n"+
					"}"),
			text("sse-format",
				"The wire format is almost aggressively simple, which is why it has outlived fancier attempts. An event "+
					"is a block of ``field: value`` lines — ``event:`` names it, ``data:`` carries the payload, ``id:`` "+
					"would set the resume cursor — terminated by a BLANK line; that blank line is the delimiter a client "+
					"parser dispatches on, and forgetting it is the classic 'my events never arrive' bug. The beat writes "+
					"``event: beat`` and a JSON ``data:`` payload per block. Two more facts the format carries by "+
					"omission: no ``Content-Length`` is possible — the body has no known end, so the transport chunks it "+
					"— and the connection is strictly one-way; a client that needs to talk back mid-stream is a client "+
					"that needs WebSockets, which is topic 28's material."),
			list("sse-rules", false,
				"Content type first — text/event-stream must be on the response before the first event block; a client cannot switch interpretation mid-body.",
				"Blank line per event — the parser dispatches on the empty line; without it the bytes are an unfinished event.",
				"Flush per event — written is not sent; Flush is what moves a finished event to the client while the handler keeps going.",
				"No Content-Length — an event stream has no known length; the transport chunks it and the client reads until the connection closes.",
				"Guard the flush — assert http.Flusher with comma-ok and degrade if absent, the same forwarding this repo's middleware chain does."),
			demo("reqres:methods", "Every method's answer, observed — plus a streaming beat",
				"The multi-method exhibit runs for real, in-process: eight methods drive "+
					"ServeReqresExhibit over a fresh httptest recorder each, and the payload reports what the recorder "+
					"OBSERVED — status, status phrase, selected headers (Location on the 201, Set-Cookie on the GET, Allow "+
					"on the OPTIONS and the truthful 405), a body-shape rule, and the exact bytes. Beside the rows, the "+
					"SSE beat streams three events with a flush each; the payload reports the events parsed structurally "+
					"and the flush count, observed through a counting wrapper — no timing values, byte-for-byte "+
					"deterministic."),
			text("sse-read-demo",
				"The demo's payload is the observed report, and its honesty is the design: the exhibit handler is real "+
					"code a route could serve unchanged, the recorder is the standard in-process observation tool, and "+
					"every row is read back rather than asserted. The flush count itself is observed the same way — the "+
					"beat runs over a counting wrapper whose entire job is to shadow the recorder's ``Flush`` and count, "+
					"so 'three events, three flushes' is a measurement, not a hope:"),
			code("sse-count", "go",
				"Counting flushes by shadowing: the wrapper's Flush counts, then forwards.",
				"server/internal/domain/demos/reqres_methods.go",
				"type reqresCountingRecorder struct {\n"+
					"    *httptest.ResponseRecorder\n"+
					"    flushes int\n"+
					"}\n"+
					"\n"+
					"func (c *reqresCountingRecorder) Flush() {\n"+
					"    c.flushes++\n"+
					"    c.ResponseRecorder.Flush()\n"+
					"}"),
			text("sse-coda",
				"One boundary worth drawing now: this beat is synchronous and in-process — real event streams add "+
					"reconnection (the ``Last-Event-ID`` cursor), heartbeats against proxy idle timeouts, and per-client "+
					"fan-out with all the concurrency that implies. Those are topic 28's material; the flush-per-event "+
					"core you just read is the part that never changes, which is why it is the part taught here."),
		),
		// ── 5. Content negotiation ─────────────────────────────────────
		section("content-negotiation", "Content negotiation",
			"The client states what it can consume; the server picks from what it produces — or says no honestly.",
			text("neg-what",
				"Content negotiation is the protocol's oldest embedded service: the client declares what it can consume, "+
					"the server answers from what it can produce. The client half is the ``Accept`` family — ``Accept`` "+
					"for media types, ``Accept-Language`` for languages (topic 27 turns that one into a real choice), "+
					"``Accept-Encoding`` for compression — each carrying a weighted list where ``application/json;q=0.9`` "+
					"means 'acceptable, nine tenths as good as my top choice' and ``*/*`` means 'anything'. The server "+
					"half is a decision with three honest outcomes: serve the best match, serve a default and label it "+
					"truthfully, or refuse with 406."),
			text("neg-family-pitch",
				"This repo's headers-echo demo classifies the whole family on the way in, and its classification table "+
					"doubles as the inventory worth memorizing — five headers whose entire job is stating what the caller "+
					"will accept, with ``Content-Type`` in the family as the outbound half of the same contract:"),
			code("neg-family", "go",
				"The negotiation family, recognized: the headers-echo demo's real classification table.",
				"server/internal/domain/demos/http_headers.go",
				"var negotiationHeaders = map[string]bool{\n"+
					"    \"Accept\":          true,\n"+
					"    \"Accept-Charset\":  true,\n"+
					"    \"Accept-Encoding\": true,\n"+
					"    \"Accept-Language\": true,\n"+
					"    \"Content-Type\":    true,\n"+
					"}"),
			text("neg-stance",
				"The stance this guide takes is the pragmatic one most JSON APIs take: serve ``application/json`` always, "+
					"name it explicitly on every response — ``writeJSON`` does, one line, one place — and treat ``Accept`` "+
					"as information rather than obligation. A client demanding XML alone from a JSON API has the wrong "+
					"address; whether that deserves a 406 is a question the RFC permits and pragmatism mostly declines. "+
					"The negotiated things that DO matter day-to-day are encoding and language: an intermediary or the "+
					"server may compress when ``Accept-Encoding`` allows it, and topic 27 picks a locale from "+
					"``Accept-Language`` with exactly the weighted-list machinery this section describes."),
			list("neg-ladder", true,
				"Parse the Accept family — weighted values, wildcards, and order; net/http hands you the raw header, the weighing is your code.",
				"Match against what you produce — a real media-type list with q-values applied, not a string equality check on the whole header.",
				"Serve the best match and label it — the response's Content-Type is the OUTCOME of the negotiation, stated out loud.",
				"Fail loudly or default honestly — 406 when refusal is the contract, a documented default when it is not; silent mismatching is the one forbidden move."),
			text("neg-coda",
				"Negotiation's center of gravity in this guide sits at other layers — encoding at the transport, language "+
					"at topic 27 — but the decision SHAPE is identical everywhere: parse the weights, match against real "+
					"capability, label the answer, and refuse honestly when refusal is the contract. Having seen it once "+
					"at the media-type layer, where it began, the same skeleton reads naturally everywhere else."),
		),
		// ── 6. Redirects ───────────────────────────────────────────────
		section("redirects", "Redirects",
			"A 3xx plus a Location — five statuses because history broke one of them.",
			text("red-what",
				"A redirect is a 3xx status plus a ``Location`` header: what you want is not here, it is THERE. It is "+
					"the response's way of correcting the client without doing the work twice, and its five statuses "+
					"exist because HTTP history broke one of them and had to add three more. The original pair — 301 "+
					"Moved Permanently and 302 Found — specified that the client repeat the same request at the new "+
					"location; browsers quietly reinterpreted them as 'repeat it as a GET', which silently converted "+
					"POSTs into bodyless GETs; 303, 307, and 308 were then added to say precisely what the old codes had "+
					"come to mean ambiguously."),
			list("red-codes", false,
				"301 Moved Permanently — forever moved; caches and bookmarks should update; the method may be rewritten to GET by historical accident.",
				"302 Found — temporarily elsewhere; do not update bookmarks; the same historical method rewriting applies.",
				"303 See Other — get the answer THERE, as a GET: the explicit, safe redirect after a POST (the pattern topic 23's login will use).",
				"307 Temporary Redirect — temporarily elsewhere AND repeat the request verbatim: method and body preserved.",
				"308 Permanent Redirect — forever moved with method and body preserved; 301 with the contract actually kept."),
			text("red-go",
				"In Go the mechanism is one function: ``http.Redirect(w, r, url, code)`` — it sets ``Location``, writes "+
					"the status, and (for GET/HEAD) adds a tiny HTML body with a link, so a human whose client does not "+
					"follow redirects still lands somewhere. Two disciplines matter more than the call itself. "+
					"``Location`` must be a path or an absolute URI, never a string pasted together from user input — an "+
					"open redirect that survives a login flow phishes with your domain's credibility. And the status is "+
					"a contract, not a vibe: after accepting a form POST, 303; re-issuing a PUT unchanged, 307; moving "+
					"forever while preserving everything, 308."),
			text("red-honest",
				"This server routes no redirects today, and the guide will not pretend otherwise: the first real 303 "+
					"lands with topic 23's login flow, and the deploy phase adds the 301 that moves plain HTTP to HTTPS "+
					"at the edge. What you can verify NOW is the client half — ``curl -L`` follows redirects, ``-i`` "+
					"shows each hop's status and Location — and the family rule that a redirect is still a full "+
					"response: it owns a status line, headers, and (sometimes) a body, and every rule this topic taught "+
					"about writing those applies to it unchanged."),
		),
		// ── 7. Terminal companion ──────────────────────────────────────
		section("terminal-companion", "Terminal companion: the curl tour",
			"The request-response surfaces of this guide, read from the shell.",
			text("term-pitch",
				"With the stack up (server on :8100), these commands walk this topic's surfaces: the page itself, both "+
					"live demos — including a body carried ON a GET, which the inspector decodes with the same "+
					"discipline its exhibits faced — the live 422 the scenario handler's decoder produces, and the "+
					"bounded-watch discipline for event streams. ``python3 -m json.tool`` keeps the tour dependency-"+
					"free; every payload below is deterministic, so repeated calls return identical shapes worth "+
					"diffing:"),
			code("term-topic", "bash", "", "",
				"# this topic, in full: seven sections, both demos inline\n"+
					"curl -s localhost:8100/api/content/request-response | python3 -m json.tool"),
			code("term-inspector", "bash", "", "",
				"# the inspector: this request's facts + the discipline's four verdicts\n"+
					"curl -s localhost:8100/api/demo/reqres/inspector | python3 -m json.tool"),
			code("term-inspector-body", "bash", "", "",
				"# a GET may legally carry a body — the live beat decodes YOUR bytes\n"+
					"# with the same decoder the exhibits faced; break the body and watch it lose\n"+
					"curl -s localhost:8100/api/demo/reqres/inspector \\\n"+
					"  -H 'Content-Type: application/json' \\\n"+
					"  -d '{\"title\":\"hello from curl\",\"tags\":[\"terminal\"]}' | python3 -m json.tool"),
			code("term-methods", "bash", "", "",
				"# the response side: eight observed method rows (status/headers/body)\n"+
					"# and the SSE beat — three events, three flushes, no timing values\n"+
					"curl -s localhost:8100/api/demo/reqres/methods | python3 -m json.tool"),
			code("term-405", "bash", "", "",
				"# a truthful 405: the mux's own Allow header, envelope body intact\n"+
					"curl -s -i -X POST localhost:8100/api/demo/reqres/methods"),
			code("term-422", "bash", "", "",
				"# the decoder's verdict on a hostile body, live: wrong-typed field → 422\n"+
					"curl -s -i -X PATCH localhost:8100/api/demo/scenario \\\n"+
					"  -H 'Content-Type: application/json' -d '{\"dataMode\":42}'"),
			code("term-sse", "bash", "", "",
				"# the stream-watch discipline: -N disables curl's buffering, --max-time\n"+
					"# bounds the watch at three seconds — never Ctrl-C a stream by hand\n"+
					"curl -N --max-time 3 -s -o /dev/null \\\n"+
					"  -w '%{http_code} %{size_download} bytes, %{time_total}s\\n' \\\n"+
					"  localhost:8100/api/demo/reqres/methods"),
			text("term-coda",
				"Two commands reward a second look. The inspector with a body shows the server's decoder judging the "+
					"very request you sent — change the body to a wrong-typed title and the live row reports the same "+
					"``*json.UnmarshalTypeError`` the exhibit pinned, on your bytes. And the scenario 422 closes the "+
					"loop this topic opened: a rejected body becomes a typed error, the central writer gives it the "+
					"envelope, and the ``x-request-id`` on that response ties it to its log line — the full path from "+
					"hostile input to honest answer, observable in one command. Next stop: topic 10 makes the "+
					"validation layer a subject in its own right."),
		),
	}

	if buildErr != nil {
		return Topic{}, buildErr
	}
	return NewTopic(
		"request-response",
		"Request and response bodies",
		"The request body in and the response out — json.Decoder discipline, status, "+
			"headers, cookies, flushing with SSE, negotiation, and redirects.",
		"request-handling",
		9,
		sections,
	)
}
