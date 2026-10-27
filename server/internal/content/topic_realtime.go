package content

import (
	"errors"
	"fmt"
)

// This file is topic 28 realtime (group media-realtime, curriculum
// order 28): the two push protocols an HTTP server grows into —
// server-sent events over the http.Flusher every router already has
// (demo realtime:ticker streams a shared feed live, with Last-Event-ID
// resume), and WebSockets through coder/websocket (demo realtime:chat
// runs a live room: handshake origin checks, one read pump and one
// write pump per connection, bounded-buffer broadcast through a hub
// with an explicit slow-consumer eviction). The ceiling is named — the
// per-connection goroutines and buffers that make room memory bounded —
// and reconnection semantics close the topic: the browser owns the SSE
// retry, ids make resume and dedupe possible, and at-least-once is the
// honest promise. The terminal companion tours both with curl -N and
// websocat.

// realtimeTopic builds topic 28.
func realtimeTopic() (Topic, error) {
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
		// ── 1. Push models ─────────────────────────────────────────
		section("push-models", "Push models: polling, SSE, WebSockets — when each wins",
			"Every realtime design starts with the same question: who initiates the next byte? Three answers cover the field, and the cheapest one that covers your traffic is the right one.",
			text("pm-shortpoll",
				"The baseline is polling: the client asks, the server answers — even when the answer is ``nothing yet''. Short polling is trivially correct and universally compatible, but its costs are structural, not tunable. Latency is half the interval on average: an event that lands one millisecond after a poll waits the whole interval for the next ask. Cost scales with emptiness: a quiet feed still pays one HTTP exchange per tick of the timer, so an idle 10,000-client fleet at one poll per second is 10,000 requests per second serving ``204 No Content''. And the server cannot prioritize: a poll is indistinguishable from any other request, so events queue behind whatever the exchange does anyway. Polling is the right answer when freshness measured in seconds is fine and the client count is small — dashboard refresh buttons, health probes, slow config rotation."),
			text("pm-longpoll",
				"Long polling moves the wait server-side: the client asks, and the server HOLDS the request open until data exists or a timeout fires, answers, and the client immediately re-asks. Latency collapses to the event itself, and empty responses mostly disappear. What remains is the plumbing the pattern makes you own: every event still costs a full HTTP exchange (headers, TLS if any, connection churn), each held request is a goroutine and a file descriptor on the server, timeouts must be tuned below every proxy's idle cutoff, and reconnection after a timeout is client code. It is the right answer when you need push semantics through infrastructure that speaks nothing else — and it is, mechanically, the halfway house that motivates both protocols this topic teaches: if the response never has to end, you have server-sent events; if the client must talk back on the same wire, you need WebSockets."),
			text("pm-two-protocols",
				"Server-sent events keep the HTTP response open forever: the client makes one ordinary GET, the server answers ``text/event-stream'' and writes framed events as they happen, flushing each one. The stream is strictly server-to-client — but that is precisely the shape of feeds, tickers, progress updates, log tails, and notifications, which is most push traffic. Because it IS HTTP, everything you already built keeps working: routing, middleware, authentication via headers, compression decisions, metrics per route. WebSockets promote the connection with an Upgrade handshake into a bidirectional frame protocol: chats, collaborative editing, presence, and games — traffic where the client talks as much as it listens. The browser's ``EventSource`` owns SSE reconnection for you; a WebSocket client manages its own retry loop. This topic builds both, on one shared fan-out core, and shows where each wins."),
			list("pm-decision",
				false,
				"Polling — the client re-asks on a timer; simplest and universally compatible, but latency equals the interval and cost scales with emptiness.",
				"Long polling — the server holds the request open until an event; near-event latency, but one HTTP exchange per event and client-owned reconnection.",
				"Server-sent events — one never-ending HTTP response of flushed frames; server-to-client only, reconnection owned by the browser, composes with all HTTP infrastructure.",
				"WebSockets — an upgraded connection carrying frames both directions; the choice when the client must talk back on the same wire, at the cost of leaving HTTP semantics behind.",
				"The tiebreaker — pick the smallest protocol that covers the traffic: one direction is SSE's job, two directions is WebSockets' job, and neither is needed for state a user refreshes manually.",
			),
			text("pm-boundary",
				"The decision boundary deserves respect in both directions. Teams reach for WebSockets too early and inherit a second protocol: their HTTP middleware stops running, their load balancer needs upgrade support and sticky timeouts, their auth story moves from a header on every request to a token sent once after the upgrade. Teams avoid WebSockets too long and build tragic long-poll chats. This guide's media-realtime group lands the split deliberately: the ticker — server announcing a number once a second — is SSE; the chat room — every guest both listening and speaking — is a WebSocket. Both share the same hub, the same bounded buffers, and the same slow-consumer policy underneath, which is the quiet lesson of the topic: the protocol is the edge; the fan-out discipline is the same everywhere."),
		),
		// ── 2. SSE via http.Flusher ─────────────────────────────────
		section("sse-streaming", "Server-sent events: the Flushered response your mux already routes",
			"An SSE endpoint is an ordinary handler that refuses to end its response: stamp the content type, write framed events, flush every one. net/http buffers writes — http.Flusher is the delivery mechanism.",
			text("sse-what",
				"A server-sent event stream is one GET request whose response never ends. The handler sets ``Content-Type: text/event-stream``, writes ``200 OK``, and then keeps writing small text frames as events happen, for as long as the client stays connected. Nothing about the routing changes: the same ``ServeMux`` pattern, the same middleware chain, the same request id — the handler simply does not return. The one mechanism that makes it work is flushing: ``net/http`` buffers ``ResponseWriter`` writes for efficiency, and an unflushed frame sits in that buffer where no client will ever see it. Asserting the writer implements ``http.Flusher`` — every real server's writer does — and calling ``Flush`` after every frame is the difference between a stream and a very slow 500-error page."),
			code("sse-handler-go", "go",
				"The live ticker stream's opening move: refuse a writer that cannot flush, stamp the one content type plus the no-cache/no-proxy headers, then write the header before any frame.",
				"server/internal/presentation/websocket.go",
				"	flusher, ok := w.(http.Flusher)\n"+
					"	if !ok {\n"+
					"		core.WriteError(w, r, core.Wrap(errFlusherUnavailable, core.KindInternal,\n"+
					"			\"This endpoint streams; the response writer cannot flush.\"), h.logger)\n"+
					"		return\n"+
					"	}\n"+
					"	w.Header().Set(\"Content-Type\", realtime.SSEContentType)\n"+
					"	for key, value := range realtime.SSEStreamRequestHeaders {\n"+
					"		w.Header().Set(key, value)\n"+
					"	}\n"+
					"	w.WriteHeader(http.StatusOK)"),
			text("sse-frameout",
				"Every byte that reaches the stream goes through one closure: write the frame, flush, report whether the client is still there. That last part is the stream's lifecycle. A browser tab closes, a proxy times out, a laptop sleeps — the ``Request.Context()`` is canceled the moment the connection is gone, and a write fails the moment after. The handler's select loop watches three things at once: the request context (the client hung up — return, and the deferred ``Leave`` unwires the subscriber), the keepalive timer (nothing was sent for a while — write a comment frame), and the subscriber's message channel (a broadcast arrived — render it as a frame). One loop, no per-frame allocations beyond the frame itself, and the connection's cost while idle is one parked goroutine blocked on a channel receive."),
			code("sse-loop-go", "go",
				"The stream loop: context, keepalive, and the subscriber's bounded channel — every branch ends the stream honestly.",
				"server/internal/presentation/websocket.go",
				"	keepalive := time.NewTimer(h.keepaliveEvery)\n"+
					"	defer keepalive.Stop()\n"+
					"	for {\n"+
					"		select {\n"+
					"		case <-r.Context().Done():\n"+
					"			return\n"+
					"		case <-keepalive.C:\n"+
					"			if !writeFrame(realtime.FormatComment(\"keepalive\")) {\n"+
					"				return\n"+
					"			}\n"+
					"			keepalive.Reset(h.keepaliveEvery)\n"+
					"		case msg, open := <-sub.Messages():\n"+
					"			if !open {\n"+
					"				// The hub evicted this stream: its bounded buffer stayed\n"+
					"				// full past the drop budget. SSE has no close frame — the\n"+
					"				// honest ending is to stop writing; EventSource reconnects\n"+
					"				// with Last-Event-ID and resume decides what was missed.\n"+
					"				return\n"+
					"			}\n"+
					"			if !writeFrame(realtimeFrame(msg)) {\n"+
					"				return\n"+
					"			}\n"+
					"			keepalive.Reset(h.keepaliveEvery)\n"+
					"		}\n"+
					"	}"),
			text("sse-feed",
				"Behind the stream sits ONE feed for the whole process, not one per connection — that is what makes every connected client see the same sequence numbers. The domain's ``TickerFeed`` is deliberately split in two: ``TickOnce`` is the deterministic core (publish one tick into the hub room, append it to the replay log, return the message), and ``Run`` is only the wall-clock shell around it — a ``time.Ticker`` firing ``TickOnce`` until the context ends. Tests drive ``TickOnce`` directly and never wait; the composition root starts ``Run`` once in its own goroutine and cancels it on shutdown. The injectable ``Now`` keeps timestamps deterministic without touching the live cadence: the same property the files topic gave its clocks, applied to a feed."),
			code("sse-feed-go", "go",
				"The feed's two halves: the deterministic tick and the wall-clock shell.",
				"server/internal/domain/realtime/feed.go",
				"func (f *TickerFeed) TickOnce() Message {\n"+
					"	msg, _ := f.hub.Publish(f.room, \"tick\", \"feed\", \"tick\", f.opts.Now())\n"+
					"	f.log.Append(msg)\n"+
					"	return msg\n"+
					"}\n"+
					"\n"+
					"// Run ticks forever at the feed's interval until the context is\n"+
					"// canceled — the composition root starts it once per process in its own\n"+
					"// goroutine and cancels it on shutdown.\n"+
					"func (f *TickerFeed) Run(ctx context.Context) {\n"+
					"	ticker := time.NewTicker(f.opts.Interval)\n"+
					"	defer ticker.Stop()\n"+
					"	for {\n"+
					"		select {\n"+
					"		case <-ctx.Done():\n"+
					"			return\n"+
					"		case <-ticker.C:\n"+
					"			f.TickOnce()\n"+
					"		}\n"+
					"	}\n"+
					"}"),
			text("sse-landing",
				"The live demo runs all of this beside its story: the panel below is a real ``EventSource`` pointed at the guide's stream — one tick per second, each frame carrying the room's sequence number in its ``id`` field. The story beats derive every byte they show: the frame anatomy is rendered by the same ``FormatEvent`` the stream calls, the response contract is the exact header set the handler stamps, and the resume math is a real replay log fed 41 real ticks. Kill your network for a moment and watch the panel reconnect on its own — the browser retries, sends ``Last-Event-ID``, and the server catches the stream up before continuing live. No page code is involved in any of that, which is the entire point of the protocol's design."),
			demo("realtime:ticker", "The realtime ticker: frames, resume math, and the live stream",
				"The SSE story — frame anatomy byte-for-byte, the response contract, Last-Event-ID replay computed live, keepalives, and the at-least-once honesty — beside the LIVE one-tick-per-second stream this page is connected to right now."),
		),
		// ── 3. Event stream mechanics ───────────────────────────────
		section("event-stream-mechanics", "Event stream mechanics: fields, frames, keepalives, Last-Event-ID",
			"The wire format is four fields and a blank line. Mastering it means rendering it in one function, keeping the line warm, and numbering events so reconnects lose nothing.",
			text("mech-format",
				"An event frame is a set of ``field: value`` lines terminated by ONE blank line — and the blank line is not decoration, it is the delivery signal: a frame without it is not an event yet, which is exactly why a proxy that buffers half a frame delivers nothing. Four fields exist. ``data`` carries the payload — one line per field, and multi-line payloads repeat the field, the browser rejoining the lines with newlines. ``id`` names the event; the browser remembers the last one it processed, which is the entire resume mechanism. ``event`` switches the frame from a default ``message`` event to a named one so page code can subscribe per kind. ``retry`` advises the reconnect delay in milliseconds. Lines starting with a colon are comments — invisible to page JavaScript, visible to every byte-counting proxy on the path."),
			code("mech-format-go", "go",
				"The one renderer both the live stream and the story payload call — byte-exact frames from one implementation.",
				"server/internal/domain/realtime/sse.go",
				"func FormatEvent(id int64, event, data string) string {\n"+
					"	var builder strings.Builder\n"+
					"	if id >= 0 {\n"+
					"		builder.WriteString(\"id: \")\n"+
					"		builder.WriteString(strconv.FormatInt(id, 10))\n"+
					"		builder.WriteByte('\\n')\n"+
					"	}\n"+
					"	if event != \"\" {\n"+
					"		builder.WriteString(\"event: \")\n"+
					"		builder.WriteString(event)\n"+
					"		builder.WriteByte('\\n')\n"+
					"	}\n"+
					"	for _, line := range strings.Split(data, \"\\n\") {\n"+
					"		builder.WriteString(\"data: \")\n"+
					"		builder.WriteString(line)\n"+
					"		builder.WriteByte('\\n')\n"+
					"	}\n"+
					"	builder.WriteByte('\\n')\n"+
					"	return builder.String()\n"+
					"}"),
			text("mech-onerenderer",
				"The format is simple enough to hand-roll in a handler, and that is exactly the trap: two renderers drift, and a drifted frame is invisible until a browser ignores a payload it should have shown. This guide renders event frames in one place — ``realtime.FormatEvent`` — and both consumers call it: the live stream for every tick, and the ``realtime:ticker`` story payload for every frame it displays as a fact. The demo's frame facts are therefore not illustrations of the format; they ARE the format's output, byte for byte, proven by a test that renders the same message both ways and compares. When the topic's demo says a tick frame looks like ``id: 41 / event: tick / data: {...}``, that string was produced by the same function your terminal's ``curl -N`` output came from."),
			text("mech-keepalive",
				"An idle stream looks dead to the middle of the network. Proxies and load balancers time out connections that carry no bytes, NAT tables forget them, and the failure mode is ugly: the server writes the next frame into a connection the client will never receive it on. The defense is the comment frame: a line starting with a colon carries no event, so ``EventSource`` ignores it completely, but the bytes traverse the path and reset every idle timer on it. This guide's stream writes one whenever nothing else has been sent for the keepalive interval — in practice the ticker's one-per-second cadence IS the keepalive, and the comment fires only if the feed ever stalls, which is the honest arrangement: a liveness mechanism that costs nothing while the stream is healthy."),
			text("mech-laste",
				"Reconnection is where the format earns its keep. ``EventSource`` reconnects on its own — no page code, no retry library — and on every attempt it sends the ``Last-Event-ID`` request header carrying the ``id`` of the last event the page actually processed. The server's half of the contract is a replay buffer: everything it retained with a higher id is replayed to the client BEFORE the stream continues live. The guide's ``ReplayLog`` is a fixed-capacity ring of the newest messages per stream; its ``After`` method returns everything strictly newer than the given seq, oldest first, so the client sees missed frames in order and then the live edge. A client that missed more than the capacity gets the retained tail — and because every frame carries its id, the gap is visible in the ids themselves rather than hidden as silent data loss."),
			code("mech-replay-go", "go",
				"The server half of resume: everything strictly newer than the client's last processed id, oldest first, never nil.",
				"server/internal/domain/realtime/replay.go",
				"func (l *ReplayLog) After(seq int64) []Message {\n"+
					"	l.mu.Lock()\n"+
					"	defer l.mu.Unlock()\n"+
					"	match := make([]Message, 0, l.size)\n"+
					"	for i := 0; i < l.size; i++ {\n"+
					"		msg := l.entries[(l.start+i)%l.capacity]\n"+
					"		if msg.Seq > seq {\n"+
					"			match = append(match, msg)\n"+
					"		}\n"+
					"	}\n"+
					"	return match\n"+
					"}"),
			text("mech-parse",
				"Parsing the incoming header is deliberately strict and deliberately forgiving in the right places: whitespace is trimmed (browsers may pad), a real integer id within int64 is accepted, and EVERYTHING ELSE — an empty header (first connection, nothing to resume), unparsable text, a negative number — reports not-ok and means the same thing: start fresh at the live edge. The alternative, guessing a position from garbage, would replay the wrong frames or skip real ones silently. The stream handler distinguishes exactly two cases: a known id replays the tail, an unknown id replays nothing and continues live — one branch, no error response, because a reconnecting client with a lost id is not an error, it is a Tuesday."),
			code("mech-parse-go", "go",
				"Last-Event-ID in: a real id, or the honest not-ok that means start fresh.",
				"server/internal/domain/realtime/sse.go",
				"func ParseLastEventID(raw string) (int64, bool) {\n"+
					"	trimmed := strings.TrimSpace(raw)\n"+
					"	if trimmed == \"\" {\n"+
					"		return 0, false\n"+
					"	}\n"+
					"	id, err := strconv.ParseInt(trimmed, 10, 64)\n"+
					"	if err != nil || id < 0 {\n"+
					"		return 0, false\n"+
					"	}\n"+
					"	return id, true\n"+
					"}"),
		),
		// ── 4. WebSockets ───────────────────────────────────────────
		section("websockets", "WebSockets with coder/websocket: the upgrade, the frames, the endings",
			"A WebSocket starts as one HTTP request the server promotes to a bidirectional frame protocol. The library is coder/websocket — the handshake, message types, ping/pong, and close codes that make every ending legible.",
			text("ws-what",
				"The WebSocket conversation begins before any socket exists: the browser sends an ordinary HTTP GET carrying ``Upgrade: websocket``, ``Connection: Upgrade``, a random ``Sec-WebSocket-Key``, and the protocol version, and the server answers ``101 Switching Protocols`` with an accept token computed from the key — the whole exchange specified by RFC 6455 so neither side can upgrade by accident. From that byte on the connection is no longer HTTP: it carries frames — text or binary payloads with ping/pong control frames interleaved — in both directions until either side sends a close frame. Everything HTTP gave you is still available BEFORE the upgrade: the request went through your router, your middleware, your request id — and crucially your origin check, which runs at accept time when it is still a decision about an HTTP request."),
			code("ws-accept-go", "go",
				"The handshake: one call, origin checked before any upgrade — a refusal is a logged plain-HTTP ending, never a socket.",
				"server/internal/presentation/websocket.go",
				"	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{OriginPatterns: h.originPatterns})\n"+
					"	if err != nil {\n"+
					"		h.logger.Warn(\"realtime chat: handshake refused\", slog.Any(\"error\", err))\n"+
					"		return\n"+
					"	}"),
			text("ws-origin",
				"The origin check is where WebSockets re-learn everything HTTP already knew. A browser always sends ``Origin`` on an upgrade request, and a page from somewhere else connecting to your room is the same attack shape as a CSRF: a page the user trusts-ish, opening a channel to a server that trusts cookies. ``coder/websocket`` refuses cross-origin handshakes unless the origin matches a stated pattern — the request's own host is always authorized, so the same-origin demo page connects with no configuration. The guide's live room pattern-allows the dev proxy's origin explicitly because in dev the page and the API legitimately live on different ports. The rule this enforces is worth keeping after the demo: enumerate who may open sockets to you, and let the library refuse everyone else before the first frame."),
			text("ws-message",
				"Once upgraded, the conversation is frames. Two kinds matter to application code: text and binary — and a protocol should choose one and refuse the other, because agreeing on the wire shape is what makes both sides' parsing honest. This room speaks JSON text frames: one inbound shape (``{type, body}``) and one outbound shape (the broadcast ``Message``), so the read loop's switch has exactly two arms — text frames are parsed, and binary frames end the connection with the unsupported-data close code rather than being guessed at. Size is the other axis: ``SetReadLimit`` caps one inbound frame, and the library itself closes the connection with ``message too big`` when a client exceeds it — the server does not need its own check, only its own policy number."),
			code("ws-message-go", "go",
				"The one outbound shape every room broadcast takes — tick, chat, or system notice — so SSE and WS speak one Message.",
				"server/internal/domain/realtime/realtime.go",
				"type Message struct {\n"+
					"	// Seq is the per-room, hub-assigned sequence number: strictly\n"+
					"	// monotonic per room, never reused, and the id SSE clients resume\n"+
					"	// from (Last-Event-ID) and WS clients order and dedupe by.\n"+
					"	Seq int64 `json:\"seq\"`\n"+
					"	// Kind is \"tick\" (the SSE ticker feed), \"chat\" (a guest line), or\n"+
					"	// \"system\" (join/leave notices the hub's callers publish).\n"+
					"	Kind string `json:\"kind\"`\n"+
					"	// From names the publisher: a guest id for chat, \"hub\" for system\n"+
					"	// notices, \"feed\" for ticks. Empty means the server itself.\n"+
					"	From string `json:\"from,omitempty\"`\n"+
					"	// Body is the text payload. For ticks it is the tick label; for\n"+
					"	// chat, the guest's line verbatim (bounded by the read limit).\n"+
					"	Body string `json:\"body\"`\n"+
					"	// At is the server-side publication time (UTC).\n"+
					"	At time.Time `json:\"at\"`\n"+
					"}"),
			text("ws-pumps",
				"A connection lives in two goroutines, one per direction. The read pump runs on the handler's own goroutine: a loop of ``conn.Read`` calls, each frame dispatched by type, ending when the client closes, the context cancels, or a protocol refusal fires. The write pump is spawned per connection and is the connection's ONLY writer — Go channels and the library's frame writing both want single-owner access, and one writer goroutine gives you that for free. It drains the subscriber's bounded channel into text frames under a write timeout, and pings on the heartbeat: the library answers inbound pings with pongs automatically in the background, so liveness costs one call per interval and no handler code. When either pump ends, a shared per-connection context cancels the other — the connection never half-lives."),
			code("ws-read-go", "go",
				"The read pump: text frames become room broadcasts, binary frames end the connection, and input the protocol does not understand is ignored defensively.",
				"server/internal/presentation/websocket.go",
				"func (h *RealtimeChatHandler) readPump(ctx context.Context, conn *websocket.Conn, guest string) (websocket.StatusCode, string) {\n"+
					"	for {\n"+
					"		frameType, data, err := conn.Read(ctx)\n"+
					"		if err != nil {\n"+
					"			return realtimeCloseCode(err)\n"+
					"		}\n"+
					"		switch frameType {\n"+
					"		case websocket.MessageBinary:\n"+
					"			return websocket.StatusUnsupportedData, realtime.CloseUnsupportedData.Explain()\n"+
					"		case websocket.MessageText:\n"+
					"			var line chatInbound\n"+
					"			if json.Unmarshal(data, &line) != nil {\n"+
					"				continue // not the protocol: ignored, never guessed for\n"+
					"			}\n"+
					"			if line.Type == \"chat\" && strings.TrimSpace(line.Body) != \"\" {\n"+
					"				h.hub.Publish(h.room, \"chat\", guest, line.Body, time.Now())\n"+
					"			}\n"+
					"		}\n"+
					"	}\n"+
					"}"),
			text("ws-close",
				"Endings are named. A close frame carries a status code, and the codes are a vocabulary worth learning once: 1000 is the orderly goodbye, 1001 says the server is going away underneath the connection, 1003 refuses a data type the protocol did not agree to, 1009 is the library's automatic answer to an oversized frame, 1011 is the honest internal failure, and 1013 — the one this stack gives its own meaning — evicts a subscriber that stopped reading. Two codes you will see only on the CLIENT side: 1005 (close with no payload) and 1006 (abnormal closure — the TCP path died without a close frame), which a server must never send but a client library will report. The guide's domain package defines its own ``CloseCode`` set with an explanation per code, and the presentation layer maps it onto the library's — so the story's close-code table, the handler's behavior, and RFC 6455's vocabulary are one list, not three."),
			code("ws-close-go", "go",
				"The domain's close-code set — one source for the handler's endings and the story's table.",
				"server/internal/domain/realtime/realtime.go",
				"const (\n"+
					"	// CloseNormal is an orderly goodbye: the guest left the room, the\n"+
					"	// demo connection ended by request.\n"+
					"	CloseNormal CloseCode = 1000\n"+
					"	// CloseGoingAway fires when the server is shutting the connection\n"+
					"	// down as the process exits or the request context is canceled.\n"+
					"	CloseGoingAway CloseCode = 1001\n"+
					"	// CloseUnsupportedData refuses a binary frame: this chat speaks\n"+
					"	// JSON text frames only.\n"+
					"	CloseUnsupportedData CloseCode = 1003\n"+
					"	// CloseMessageTooLarge is the library's automatic answer when a\n"+
					"	// frame exceeds the read limit this stack sets (512 bytes).\n"+
					"	CloseMessageTooLarge CloseCode = 1009\n"+
					"	// CloseInternalError is the honest answer when a write fails\n"+
					"	// underneath us.\n"+
					"	CloseInternalError CloseCode = 1011\n"+
					"	// CloseSlowConsumer evicts a subscriber that stopped reading: its\n"+
					"	// bounded buffer stayed full past the drop budget, so the hub\n"+
					"	// removes it rather than let its backlog grow without bound.\n"+
					"	CloseSlowConsumer CloseCode = 1013\n"+
					")"),
			text("ws-landing",
				"The live room runs all of this beside its story: the panel below is a real ``WebSocket`` client — it joins as a guest, receives the join notices, echoes your own lines back to you (that echo is the confirmation the room served you), and shows system events as other guests come and go. The story payload's numbers are not prose: the delivered counts and the eviction it shows come from a real hub choreography the payload runs synchronously on every fetch — three members, one of whom never reads, and the hub's own verdict on what happened to them. Send a binary frame or an oversized line from a terminal client and you will meet the close codes from the table above, on the wire, exactly as documented."),
			demo("realtime:chat", "The realtime chat room: a live WebSocket conversation",
				"The WS story — the handshake and origin policy, a real broadcast choreography with a real eviction, the ceiling's numbers, and the close-code table — beside the LIVE room: join as a guest, say something, watch the fan-out."),
		),
		// ── 5. Rooms and the ceiling ────────────────────────────────
		section("rooms-and-ceiling", "Broadcast, rooms, and the ceiling",
			"The hub is the topic's core data structure: named rooms of subscribers, one non-blocking publish, bounded buffers everywhere — because a realtime server's real risk is the read that never comes.",
			text("hub-rooms",
				"Both live demos share one hub — the SSE ticker subscribes to room ``ticker``, the chat runs in room ``chat``, and rooms are just named sets of subscribers inside one mutex-guarded map. Joining wires a subscriber's bounded channel exactly once; leaving unwires it, and a room that empties is deleted along with its sequence counter, so an abandoned room costs nothing by morning. The subscriber's identity is its seat, not its person: after a connection ends, its seat is dead, and a reconnect takes a NEW seat — which is precisely why reconnect re-sync is an explicit protocol step rather than an implementation detail the hub could paper over. Room counts feed the chat's system notices (``guest-2 joined — 2 online``) through the same broadcast path everything else takes."),
			code("hub-struct-go", "go",
				"The fan-out core: rooms of subscribers behind one mutex, sized by a policy both of whose numbers must be positive.",
				"server/internal/domain/realtime/hub.go",
				"type Hub struct {\n"+
					"	mu      sync.Mutex\n"+
					"	policy  HubPolicy\n"+
					"	rooms   map[string]map[*Subscriber]struct{}\n"+
					"	nextSeq map[string]int64\n"+
					"}\n"+
					"\n"+
					"// HubPolicy sizes a hub. Both numbers must be positive: a zero buffer\n"+
					"// would drop every broadcast, a zero drop budget would evict on the\n"+
					"// first full buffer.\n"+
					"type HubPolicy struct {\n"+
					"	Buffer   int\n"+
					"	MaxDrops int\n"+
					"}"),
			text("hub-publish",
				"Publishing is one method with two invariants. First, every message gets the room's next sequence number — strictly monotonic, never reused — before any delivery happens, so ordering is assigned at the source rather than inferred from arrival. Second, delivery to each member is a NON-BLOCKING channel send inside a select with a ``default`` arm: a ready subscriber receives immediately, a full-buffered subscriber is skipped for that message, and the publisher never waits for anyone. The critical section is a handful of map operations and channel sends under the hub mutex — microseconds — so a room of thousands publishes in one pass with no locks held across I/O. The cost model this buys is the point: one publish is O(members) cheap operations, and no slow reader anywhere can stall a broadcast."),
			code("hub-publish-go", "go",
				"The fan-out loop: non-blocking send, counted drop, and eviction when the budget is spent.",
				"server/internal/domain/realtime/hub.go",
				"	for sub := range members {\n"+
					"		select {\n"+
					"		case sub.ch <- msg:\n"+
					"			stats.Delivered++\n"+
					"		default:\n"+
					"			// Full buffer: the slow-consumer policy. This subscriber\n"+
					"			// misses THIS message; enough misses in a row and it loses\n"+
					"			// its seat. No send ever blocks the publisher.\n"+
					"			sub.drops++\n"+
					"			stats.Dropped++\n"+
					"			if sub.drops >= h.policy.MaxDrops && !sub.evicted {\n"+
					"				// …marked, removed from the room, channel closed\n"+
					"				// with CloseSlowConsumer — the write pump observes\n"+
					"				// the close and ends the connection with the code…\n"+
					"			}\n"+
					"		}\n"+
					"	}"),
			text("hub-slow",
				"The slow-consumer policy is where the ceiling gets teeth. A full buffer costs the subscriber THIS message — counted, not silent — and a subscriber that misses ``MaxDrops`` publishes in a row is evicted: removed from the room, its channel closed with the slow-consumer close code, and its write pump (which drains the channel's buffered remainder first, Go delivers buffered values before the closed signal) ends the connection with code 1013 and the reason. Why evict rather than buffer more? Because an unread buffer is unbounded memory by another name: a phone in a pocket on a train generates exactly this subscriber, and a room that keeps its backlog grows without bound for everyone who ever joined. The room's cost must be bounded by ENGAGED readers — and the eviction is honest: the client learns why it lost its seat and may reconnect, at which point ids make catching up possible."),
			code("hub-evict-go", "go",
				"The write pump's eviction arm: the buffered remainder drains, then the connection ends with the hub's own code and reason.",
				"server/internal/presentation/websocket.go",
				"		case msg, open := <-sub.Messages():\n"+
					"			if !open {\n"+
					"				// The hub closed the channel: this subscriber was\n"+
					"				// evicted by the slow-consumer policy. Buffered messages\n"+
					"				// were drained first; now the connection ends with the\n"+
					"				// eviction's own code and reason.\n"+
					"				code, why := sub.Eviction()\n"+
					"				conn.Close(websocket.StatusCode(code), why)\n"+
					"				return\n"+
					"			}"),
			text("hub-ceiling",
				"Now the arithmetic a capacity plan actually needs. Per connection: two goroutines (read pump, write pump) and one buffered channel of ``Buffer`` message slots. Per room: a members set and one int64 sequence counter. Per publish: one pass of non-blocking sends. Go's runtime makes a parked goroutine cost a few kilobytes, so ten thousand idle chat connections are twenty thousand mostly-parked goroutines and a few hundred megabytes worst case — a real number, but a PLANNABLE one, and the buffers are bounded by policy, not by traffic. The numbers this room runs are one object: the ``ChatPolicy`` the handler enforces is the same struct the story payload quotes, so the live room's read limit, buffer size, drop budget, heartbeat, and write timeout cannot drift from what the topic teaches."),
			code("hub-policy-go", "go",
				"One policy object: the handler enforces it, the story quotes it, the tests pin it.",
				"server/internal/domain/realtime/realtime.go",
				"type ChatPolicy struct {\n"+
					"	// ReadLimit caps one inbound frame in bytes (SetReadLimit plus an\n"+
					"	// explicit check): the ceiling on what a client may say at once.\n"+
					"	ReadLimit int64\n"+
					"	// Buffer is each subscriber's outbound channel capacity.\n"+
					"	Buffer int\n"+
					"	// MaxDrops is how many publishes a full-buffered subscriber may\n"+
					"	// miss before the hub evicts it.\n"+
					"	MaxDrops int\n"+
					"	// Heartbeat is the interval between protocol-level pings.\n"+
					"	Heartbeat time.Duration\n"+
					"	// WriteTimeout bounds one outbound frame write.\n"+
					"	WriteTimeout time.Duration\n"+
					"}\n"+
					"\n"+
					"// DefaultChatPolicy is the policy the live chat demo runs: generous\n"+
					"// enough for humans, tight enough that the ceiling is real.\n"+
					"func DefaultChatPolicy() ChatPolicy {\n"+
					"	return ChatPolicy{\n"+
					"		ReadLimit:    512,\n"+
					"		Buffer:       16,\n"+
					"		MaxDrops:     8,\n"+
					"		Heartbeat:    15 * time.Second,\n"+
					"		WriteTimeout: 5 * time.Second,\n"+
					"	}\n"+
					"}"),
		),
		// ── 6. Reconnection semantics ───────────────────────────────
		section("reconnection-semantics", "Reconnection semantics: who owns the retry",
			"Connections drop — laptops sleep, phones change cells, proxies restart. The design question is never whether to reconnect but who owns it, and what the server promised about delivery across the gap.",
			text("rec-sse",
				"For SSE the answer is beautifully lopsided: the browser owns everything. ``EventSource`` detects the drop, waits (the ``retry`` field's advice, else its default), reconnects on its own, and sends ``Last-Event-ID`` — no page code, no retry library, no exponential backoff implementation to get wrong. The server's whole contribution is to make resume possible: number every event (the ``id`` field), retain recent history (the replay log), replay on a known id, and keep the line warm so drops are rare. This division is why SSE demos feel magical — the demo page never reconnects, yet survives a Wi-Fi blip — and it is worth internalizing as a design principle: push the retry policy to the layer that can make it universal, and keep the server's duties enumerable."),
			text("rec-ws",
				"For WebSockets nobody owns it for you: the ``WebSocket`` API fires ``close`` and stops. Reconnection is client code — reconnect with backoff, re-authenticate if the upgrade consumed a token, re-subscribe, and re-sync what was missed. The server can make each of those steps cheap, and this room does: sequence numbers are monotonic per room, so a reconnecting client knows exactly which seq it last rendered and can ask or dedupe accordingly; join notices re-establish presence for everyone; and the room's eviction policy guarantees a reconnecting backlog cannot wedge the room. The honest framing for a production system: WS reconnection is a small state machine you test, not a feature you get — which is why the earlier decision boundary leaned SSE-first, WS-when-needed."),
			text("rec-once",
				"Delivery promises deserve precision, because ``resume'' tempts people to promise exactly-once, and exactly-once across reconnects is a lie unless you build transactions into the stream. What this stack delivers is at-least-once per connection and at-most-once per id: a browser that reconnected BEFORE its page processed the last delivered frame will receive that frame again after resume, and that is correct behavior, not a bug — the server cannot know the page never ran its handler. The defense is one line of consumer code: track the highest seq handled, drop anything less or equal. With ids in every frame, duplication is harmless and gaps are visible; without ids, neither is knowable. The rule to carry out of this topic: every event carries an id, every consumer dedupes, and the promise you document is at-least-once."),
			list("rec-rules",
				true,
				"Number every event — the id field is the resume mechanism and the dedupe key; an unnumbered stream cannot reconnect honestly.",
				"Retain recent history — a bounded replay log turns a reconnect from data loss into a visible gap.",
				"Replay before live — everything newer than the client's last id first, then the live edge, so ordering survives the gap.",
				"Advise, don't command — the retry field sets reconnect pace; the browser still owns the timer.",
				"Dedupe on seq — consumers keep the highest id handled; at-least-once becomes indistinguishable from exactly-once in effect.",
				"Say what you promise — at-least-once per connection, at-most-once per id, written down where consumers can read it.",
			),
		),
		// ── 7. Terminal companion ───────────────────────────────────
		section("terminal-companion", "Terminal companion: the realtime curl tour",
			"Both protocols, from a shell: curl -N walks the stream frame by frame (and resumes with Last-Event-ID), websocat speaks the room's JSON protocol, and every close code in the table is one command away.",
			text("term-intro",
				"With the stack up (server on :8100), the SSE half needs only curl — with ONE flag that matters: ``-N`` disables curl's output buffering, the client-side twin of the server's flush, and without it the frames pool in the terminal's pipe and the demo looks broken. Run the first block and watch the stream: the retry advice, then one tick per second, each frame's id advancing by one — Ctrl-C hangs up like a browser tab closing. The second block is the reconnect story performed by hand: kill the first curl, note the last id you saw, reconnect carrying it, and watch the server replay exactly the ticks you missed before continuing live. The WS half uses ``websocat`` (``cargo install websocat`` or your package manager) — join, speak, and meet the close codes by sending what the room forbids."),
			code("term-sse-bash", "bash", "", "",
				"# the live stream: -N is curl's flush — Ctrl-C to hang up\n"+
					"curl -N localhost:8100/api/demo/realtime/ticker/stream\n"+
					"\n"+
					"# the story: frame anatomy, response contract, resume math — derived\n"+
					"curl -s localhost:8100/api/demo/realtime/ticker | python3 -m json.tool | head -30\n"+
					"\n"+
					"# reconnect by hand: reconnect carrying the last id you processed\n"+
					"curl -N -H 'Last-Event-ID: 41' localhost:8100/api/demo/realtime/ticker/stream\n"+
					"\n"+
					"# an unparsable id means start fresh — the replay is empty, live follows\n"+
					"curl -N -H 'Last-Event-ID: nonsense' localhost:8100/api/demo/realtime/ticker/stream"),
			code("term-ws-bash", "bash", "", "",
				"# the live room: join, and the hub answers with your join notice\n"+
					"websocat ws://localhost:8100/api/demo/realtime/chat/ws\n"+
					"\n"+
					"# say something — your echo back is the confirmation you were served\n"+
					"{\"type\":\"chat\",\"body\":\"hello from the terminal\"}\n"+
					"\n"+
					"# open a second websocat in another terminal and watch both sides\n"+
					"# see the same seq — one room, one sequence, two members\n"+
					"\n"+
					"# meet the close codes: a binary frame ends you with 1003\n"+
					"websocat -b ws://localhost:8100/api/demo/realtime/chat/ws\n"+
					"\n"+
					"# and an oversized line (over the 512-byte read limit) ends with 1009\n"+
					"websocat ws://localhost:8100/api/demo/realtime/chat/ws\n"+
					"{\"type\":\"chat\",\"body\":\"aaaaaaaaaaaaaaaa ... past 512 bytes ... a\"}"),
			text("term-coda",
				"Read the tour as the topic's proofs, re-runnable any time: the stream's frames byte-identical to the story's derived facts (one ``FormatEvent`` behind both), the resume replaying exactly the retained tail past your id and nothing before it, the room echoing your line to every member including you, and the close codes arriving on the wire exactly as the table promised — 1003 for binary, 1009 for oversized, 1013 waiting for a subscriber that never reads. Realtime in Go ends where it started: the protocols are thin, the discipline is in the fan-out — bounded buffers, non-blocking publishes, evictions that say why, ids in every frame — and both halves run on one hub small enough to read in a sitting. The next topic moves the same discipline off the request path entirely: work that happens after the response is sent."),
		),
	}

	if buildErr != nil {
		return Topic{}, buildErr
	}
	return NewTopic(
		"realtime",
		"Realtime Go: server-sent events and WebSockets",
		"Push over HTTP, twice: server-sent events as one never-ending Flushered response "+
			"(text/event-stream frames with id/event/data fields, comment keepalives, and "+
			"Last-Event-ID replay from a bounded log — demo realtime:ticker streams the guide's "+
			"shared feed live beside its derived story), and WebSockets via coder/websocket "+
			"(the 101 upgrade, origin-checked handshakes, text/binary message types, ping/pong "+
			"liveness, and named close codes — demo realtime:chat runs a live room). Both fan "+
			"out through one hub: named rooms, per-connection read/write pumps, bounded "+
			"per-subscriber buffers, and an explicit slow-consumer eviction (close 1013) that "+
			"keeps room memory bounded by engaged readers — closed by reconnection semantics "+
			"(the browser owns the SSE retry; ids make resume and dedupe possible; at-least-once "+
			"is the honest promise) and the terminal's curl -N and websocat tour.",
		"media-realtime",
		28,
		sections,
	)
}
