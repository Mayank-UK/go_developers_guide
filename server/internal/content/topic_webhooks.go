package content

import (
	"errors"
	"fmt"
)

// This file is topic 14 webhooks (group apis, curriculum order 14),
// authored on the exemplar pattern: prose is the default block, every
// must-source code block quotes the REAL simulator this guide ships
// (internal/domain/demos/webhook_simulator.go — the HMAC signing, the
// verification gate, and the backoff schedule below are live code the
// demo runs, not illustrations), the live demo sits INLINE in the
// section that walks the retry ladder, and the topic closes with the
// bash terminal companion.

// webhooksTopic builds topic 14 — the reversed API: what changes when
// YOUR server becomes the caller's target, how to design event payloads
// receivers can trust and process, HMAC signing with timestamp+nonce
// discipline, the retry ladder with exponential backoff and its
// dead-letter exit, the receiving side's front door (verify, answer 2xx
// fast, process async), and the local dev story — plus the delivery
// simulator that runs all of it for real.
func webhooksTopic() (Topic, error) {
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
		// ── 1. Webhooks are reversed APIs ──────────────────────────────
		section("reversed-apis", "Webhooks are reversed APIs",
			"The control flow you spent eleven topics building flips direction: now somebody else's server calls yours.",
			text("rev-pitch",
				"Every API so far in this guide had the same shape: a client sends a request, "+
					"your server answers, the conversation ends when the response lands. A webhook "+
					"inverts that. Some event happens on ANOTHER system — an invoice gets paid, a "+
					"repository gets pushed to, a card gets declined — and that system makes an HTTP "+
					"request TO a URL you registered, carrying a body describing the event. You are "+
					"no longer the server waiting for work; you are the endpoint receiving work you "+
					"did not ask for at this moment. All the disciplines you know still apply, but "+
					"the roles swap: the sender owns reliability, and you — the receiver — own "+
					"latency, verification, and idempotency."),
			text("rev-inversion",
				"The inversion is deeper than who sends the request. When you expose an API, YOU "+
					"decide the contract and clients adapt; you see every request in your logs; you "+
					"rate-limit abuse at your door. When you CONSUME or EMIT webhooks, the contract "+
					"is negotiated with each counterparty, the interesting failures happen on "+
					"someone else's network, and your debugging starts from a body you cannot "+
					"reproduce by curl-ing your own endpoint. A webhook system is therefore two "+
					"programs wearing each other's clothes: a sender that behaves like a stubborn "+
					"client (retries, backoff, giving up into a dead-letter queue) and a receiver "+
					"that behaves like a very patient server (verify first, answer fast, process "+
					"later). This topic builds both halves, in Go, from the standard library."),
			text("rev-semantics",
				"The delivery contract is almost always at-least-once: the sender retries until it "+
					"hears 2xx, and anything that can go wrong in a distributed system — timeouts "+
					"after your handler succeeded, connection resets mid-write, two workers "+
					"processing one queue — means the same event WILL sometimes arrive twice. "+
					"Exactly-once delivery is a marketing phrase; at-least-once plus idempotent "+
					"receivers is an engineering plan. Write that on the whiteboard before you "+
					"write a line of code, because every design decision downstream follows from "+
					"it: payloads carry stable event ids, receivers dedupe, senders keep a ledger "+
					"of every attempt, and nothing on either side may assume the counterpart "+
					"remembers anything about the previous delivery."),
			text("rev-concerns",
				"Splitting the problem the way this topic does keeps each concern small enough to "+
					"be correct. The sender's half is a delivery engine: design payloads that age "+
					"well, sign them so receivers can prove origin, walk a retry ladder that is "+
					"polite to a struggling endpoint, and park the hopeless cases somewhere a human "+
					"can reach them. The receiver's half is a front door: verify the signature "+
					"BEFORE trusting anything, answer 2xx before doing slow work, and process the "+
					"event asynchronously with dedupe. The guide's live demo — the webhook "+
					"simulator — runs both halves as real code; this tour sets up the decisions "+
					"each half has to make:"),
			list("rev-concern-map", false,
				"Payload design — the event envelope: stable id, type, version, occurred time, and a data object the receiver can evolve with.",
				"Signing — HMAC-SHA256 over the exact bytes, with timestamp and nonce, so a receiver can reject forgeries and replays without trusting the network.",
				"Delivery — the retry ladder: exponential backoff between attempts, a maximum attempt count, and a dead-letter queue for the deliveries that never succeed.",
				"Receiving — the front door: verify first, answer 2xx immediately, then process async and dedupe by event id.",
				"Local development — tunnels and fixtures: how to receive real webhooks on a laptop and test without waiting for a real payment to fail."),
			text("rev-go",
				"Go's standard library covers the whole surface without a single dependency: "+
					"``crypto/hmac`` and ``crypto/sha256`` for signing, ``net/http`` for both the "+
					"outbound delivery POST and the inbound receiving handler, ``time`` for the "+
					"backoff arithmetic and the replay window. What the stdlib deliberately does "+
					"NOT give you is a webhook framework — no signature-header convention, no "+
					"retry loop, no dead-letter queue — because those are policy, not plumbing. "+
					"This topic's position is the guide's usual one: build the loop once by hand "+
					"so you know exactly what a managed service or a queue library is doing for "+
					"you later. The simulator this topic quotes and runs is that hand-built loop, "+
					"complete enough to steal for production."),
			text("rev-where",
				"One framing note before the details, because it places webhooks in the rest of "+
					"the guide. A webhook is not a messaging system — there is no broker, no "+
					"consumer groups, no ordering across events; it is HTTP POSTs with a delivery "+
					"policy bolted on. When you need fan-out to many independent subscribers, "+
					"ordered streams, or durable queues with pending-and-claim semantics, topic 22 "+
					"(messaging) is the honest answer. Webhooks win when the counterparty is "+
					"another company's HTTP endpoint, when you want zero infrastructure between "+
					"the event and its consumer, or when you ARE the consumer of someone else's "+
					"product. The two worlds compose: a common production shape is a webhook "+
					"receiver that verifies and enqueues, and a messaging consumer that does the "+
					"actual work."),
		),
		// ── 2. Designing the payloads and events ───────────────────────
		section("designing-payloads", "Designing the payloads and events",
			"The body is the API now — design it like one, because you cannot version a POST the way you version a URL.",
			text("pay-envelope",
				"An event payload needs an envelope that separates WHAT HAPPENED from the data "+
					"about it. The bare minimum set — and the set the whole ecosystem has "+
					"converged on, from Stripe to GitHub — is an id that identifies THIS "+
					"OCCURRENCE (not the object), a type naming the event (``invoice.paid``), a "+
					"schema version for the payload's shape, the time the event occurred in the "+
					"sender's domain, and a ``data`` object carrying the facts. The guide's "+
					"simulator ships exactly this envelope as its signed body — one JSON object, "+
					"readable at a glance:"),
			code("pay-envelope-body", "go",
				"The simulator's canonical signed body — the exact bytes the HMAC covers.",
				"server/internal/domain/demos/webhook_simulator.go",
				"webhookBody = `{\"eventId\":\"evt_0142\",\"type\":\"invoice.paid\",\"version\":\"1\",` +\n"+
					"    `\"occurredAt\":\"2026-10-26T08:59:59Z\",\"data\":{\"invoiceId\":\"in_88\",\"amountCents\":42000,\"currency\":\"USD\"}}`"),
			text("pay-id-read",
				"Read that body as a contract. ``eventId`` is globally unique and stable — it is "+
					"the receiver's dedupe key when (not if) the same delivery arrives twice. "+
					"``type`` plus ``version`` let the receiver route and evolve: a handler for "+
					"``invoice.paid`` version 1 keeps working while version 2 adds a field "+
					"somewhere else. ``occurredAt`` is the BUSINESS time — when the payment "+
					"happened — which is different from delivery time and from the signing "+
					"timestamp the signature header carries; conflating those three is a classic "+
					"first design bug, because retries make delivery time a many-valued thing "+
					"while the business time is one fact. And ``amountCents`` as an integer with "+
					"an explicit ``currency`` is the money-in-cents discipline every payment "+
					"API beats into its users: floats in money are a rounding-error lawsuit."),
			text("pay-granularity",
				"Granularity — what deserves its own event — is the design tension with no "+
					"perfect answer, only good defaults. Emit NARROW events (one business fact "+
					"each: ``invoice.paid``, not ``invoice.changed``) because narrow events give "+
					"receivers the choice of what to subscribe to, and a receiver that only "+
					"cares about payments should not have to parse everything to find out this "+
					"was a refund. Emit COMPLETE events (carry the resulting state, not just "+
					"the delta) because forcing every receiver to reconstruct state from a "+
					"changelog couples them to your full history. And emit PAST-tense events "+
					"(``invoice.paid``) rather than commands (``pay-invoice``) — a webhook "+
					"announces a fact that already happened; it is not an RPC the receiver is "+
					"expected to make happen. Receivers may reject, but they may not veto."),
			text("pay-idempotency",
				"Every field choice funnels toward idempotency. At-least-once delivery means the "+
					"receiver WILL see ``evt_0142`` twice in some failure mode — the sender "+
					"timed out reading your 200, retried, and now there are two POSTs with one "+
					"business fact. The payload makes survival possible: the receiver records "+
					"``eventId`` in an insert-only table (or a unique-key store) BEFORE doing "+
					"the work, so the second arrival is recognized and skipped as a no-op. This "+
					"is why ``eventId`` must be assigned by the EVENT's owner at creation time, "+
					"not regenerated per delivery attempt — the retry carries the SAME id, or "+
					"the dedupe never fires. The receiving section shows the exact sequence; "+
					"here the design rule: no event id, no event."),
			text("pay-evolution",
				"Versioning a webhook is versioning a promise you cannot recall: old bodies are "+
					"already sitting in dead-letter queues, receiver code you do not control, "+
					"and integration docs. The working rules are additive-only evolution (new "+
					"fields are fine — receivers must ignore what they do not know; removing or "+
					"renaming fields is a new version), explicit ``version`` in the envelope so "+
					"the shape change is discoverable instead of surprising, and a live "+
					"compatibility test that posts the current payload shape at your own "+
					"receiver before shipping. When a breaking change is genuinely needed, "+
					"publish the new version alongside the old, announce a sunset with dates, "+
					"and meter how many deliveries still go to the old shape — the same "+
					"discipline topic 11 (api-design) applies to URL versioning, moved into "+
					"the body. The checklist that keeps all of this honest is short:"),
			list("pay-checklist", false,
				"Envelope — eventId, type, version, occurredAt, data: five fields every event carries, before any domain specifics.",
				"Stable identity — eventId assigned at event creation and reused across retries; it is the receiver's dedupe key.",
				"Narrow events — one business fact per event, past tense; subscriptions stay cheap and routing stays simple.",
				"Complete data — carry the resulting state, not instructions; receivers never replay your history to know what happened.",
				"Additive evolution — new fields only inside a version; breaking changes ship as a new version beside the old.",
				"Money and time — integer minor units with explicit currency; RFC 3339 UTC timestamps with full precision."),
		),
		// ── 3. HMAC signing ────────────────────────────────────────────
		section("hmac-signing", "HMAC signing: proving who sent it",
			"The body arrives over a network anyone can POST to — a shared secret, hashed over the exact bytes, is the proof of origin.",
			text("sign-why",
				"Your receiving URL is a plain HTTPS endpoint, and anyone who learns it can POST "+
					"a fake ``invoice.paid`` to it. TLS authenticates the PIPE, not the SENDER — "+
					"it proves the bytes were not modified in flight, but says nothing about who "+
					"wrote them. The standard answer is an HMAC: both sides share a secret key "+
					"(exchanged once, out of band, ideally per-endpoint), the sender computes a "+
					"keyed hash over the delivery and attaches it, and the receiver recomputes "+
					"the same hash over what actually arrived. A forger without the secret can "+
					"produce bodies, but not bodies whose hash matches — and a middleman who "+
					"alters one digit of the body breaks the hash deterministically. HMAC is "+
					"symmetric, cheap, and needs no certificate ceremony, which is why it "+
					"dominates webhook signing from Stripe's ``Stripe-Signature`` to GitHub's "+
					"``X-Hub-Signature-256``."),
			text("sign-scheme",
				"The scheme this guide builds — and the simulator runs for real — signs a "+
					"CANONICAL MESSAGE built from three facts: the signing timestamp, a nonce, "+
					"and the raw body bytes, joined with dots: ``{timestamp}.{nonce}.{body}``. "+
					"The result rides in one header, ``X-Guide-Signature``, as "+
					"``t=…,n=…,v1=…`` with the signature hex-encoded. Every piece earns its "+
					"place: the timestamp turns the signature into a perishable artifact (an "+
					"intercepted delivery cannot be replayed next week — the receiver's clock "+
					"check kills it), the nonce distinguishes two deliveries signed in the same "+
					"second over the same body (different message, different signature), and "+
					"signing the RAW bytes means what you see is exactly what was hashed — no "+
					"canonicalization step for an attacker to exploit. The sender side is four "+
					"lines of real Go:"),
			code("sign-sign-delivery", "go",
				"The sender's whole signing step: HMAC-SHA256 over the canonical message.",
				"server/internal/domain/demos/webhook_simulator.go",
				"func signDelivery(secret, timestamp, nonce string, body []byte) string {\n"+
					"    mac := hmac.New(sha256.New, []byte(secret))\n"+
					"    fmt.Fprintf(mac, \"%s.%s.%s\", timestamp, nonce, body)\n"+
					"    return hex.EncodeToString(mac.Sum(nil))\n"+
					"}"),
			text("sign-read",
				"Three details in those four lines are worth slowing down for. ``hmac.New`` "+
					"takes the hash constructor ``sha256.New`` itself — not an instance — "+
					"because HMAC precomputes the key-padded inner and outer states, something "+
					"a hand-rolled ``sha256(secret + body)`` gets wrong in both security and "+
					"ego cost; never build an HMAC from a raw hash. Writing the message with "+
					"``fmt.Fprintf`` into the ``hash.Hash`` works because it satisfies "+
					"``io.Writer``, and avoids building an intermediate string for large "+
					"bodies. And ``hex.EncodeToString`` (not base64) matches the ecosystem's "+
					"lowercase-hex convention — a mismatch HERE is invisible until the "+
					"receiver's comparison fails on two encodings of the SAME bytes, which is "+
					"among the most annoying integration bugs in the field."),
			text("sign-verify-order",
				"The receiver's gate is where the discipline lives, and the ORDER of its checks "+
					"is the teaching point. The clock check runs FIRST: parse the presented "+
					"timestamp and reject anything outside the tolerance window before doing "+
					"any cryptography, so a replayed delivery from last week dies cheaply. Only "+
					"then does the gate rebuild the expected signature from the parts that "+
					"arrived — the timestamp and nonce from the header, the raw body bytes — "+
					"and compare. The simulator's gate, exactly as the demo runs it:"),
			code("sign-verify-delivery", "go",
				"The receiver's gate: clock first, then a constant-time HMAC comparison.",
				"server/internal/domain/demos/webhook_simulator.go",
				"func verifyDelivery(secret string, parts signatureParts, receivedAt time.Time, body []byte) error {\n"+
					"    issuedAt, err := time.Parse(time.RFC3339, parts.Timestamp)\n"+
					"    if err != nil {\n"+
					"        return fmt.Errorf(\"unparseable timestamp %q: %w\", parts.Timestamp, err)\n"+
					"    }\n"+
					"    // Absolute delta: far-past AND far-future timestamps are stale.\n"+
					"    if delta := receivedAt.Sub(issuedAt); delta < -webhookTolerance || delta > webhookTolerance {\n"+
					"        return errStaleTimestamp\n"+
					"    }\n"+
					"    expected := signDelivery(secret, parts.Timestamp, parts.Nonce, body)\n"+
					"    if !hmac.Equal([]byte(expected), []byte(parts.Signature)) {\n"+
					"        return errSignatureMismatch\n"+
					"    }\n"+
					"    return nil\n"+
					"}"),
			text("sign-constant-time",
				"The comparison is ``hmac.Equal``, not ``==``, and the difference is a timing "+
					"attack, not style. A byte-slice ``==`` returns false at the FIRST "+
					"differing byte, so its runtime leaks how many leading bytes matched; an "+
					"attacker who can measure response times can forge a signature one byte at "+
					"a time, each guess measurably slower to reject. ``hmac.Equal`` (and its "+
					"``crypto/subtle`` cousin ``ConstantTimeCompare``) always walks every "+
					"byte, leaking nothing. Go makes the safe path the easy path here — but "+
					"only if you know the unsafe path exists. The demo's tampered case proves "+
					"the discipline matters: the body's amount rewritten from 42000 to 99000 "+
					"cents after signing fails the gate, because the rebuilt HMAC is over the "+
					"bytes that ARRIVED."),
			text("sign-window",
				"The tolerance window — five minutes in the simulator — is a business decision "+
					"wearing a security hat. Too tight, and honest deliveries die on ordinary "+
					"clock skew between two servers (NTP is good, not perfect); too loose, and "+
					"a replay has that long to matter. Note the check uses the ABSOLUTE delta: "+
					"far-future timestamps are as suspicious as far-past ones, or the attacker "+
					"signs a delivery dated tomorrow and lets your clock catch up to it. The "+
					"timestamp discipline composes with the retry ladder in a subtle way: the "+
					"timestamp is the SIGNING time, not the send time — a delivery retried "+
					"eight seconds later carries its original signing timestamp, still well "+
					"inside the window, while a delivery retried after an hours-long outage "+
					"SHOULD be re-signed, which is one more reason the sender keeps the "+
					"original event and rebuilds deliveries from it rather than replaying "+
					"captured bytes."),
			text("sign-secrets",
				"The secret is the whole ballgame, so its lifecycle is part of the design. "+
					"Keys arrive by environment or a secrets manager — ``WEBHOOK_SIGNING_SECRET`` "+
					"in this guide's twelve-factor world (topic 36) — never in source, never in "+
					"logs, never in the payload; the simulator's exhibit key says ``demo-only`` "+
					"out loud precisely so nobody copies the pattern. Give every receiving "+
					"endpoint its own key so one leak is one rotation, not a fleet-wide "+
					"incident. Rotation is a versioned-header problem: sign with ``v1=`` today, "+
					"add ``v2=`` alongside during migration, retire ``v1`` when the meter says "+
					"zero — the same additive-only evolution the payload section preached. And "+
					"log a signature VERDICT (verified, mismatch, stale) with the request id, "+
					"never the signature bytes or key material: a rejection you can audit "+
					"beats a secret you can leak."),
		),
		// ── 4. Retries, backoff, and the DLQ ───────────────────────────
		section("retries-and-dlq", "Retries, exponential backoff, and the DLQ",
			"The sender owns delivery now — a polite ladder for struggling endpoints, and a back door for the hopeless ones.",
			text("retry-why",
				"When your server answered a request in topic 09, reliability was the client's "+
					"problem. As a webhook SENDER it is yours: the receiver is redeploying, "+
					"rate-limiting, or lying under load, and your job is to deliver anyway — "+
					"without hammering. The answer is a retry ladder: try, and on failure wait "+
					"a growing amount and try again, up to a maximum number of attempts. What "+
					"counts as retryable matters as much as the schedule — a 500, 502, 503, or "+
					"504 says \"not right now\", so retry; a 410 or 422 says \"this endpoint is "+
					"done with you\", so stop immediately; and a 429's ``Retry-After`` header "+
					"overrides your schedule when present, because the receiver is TELLING you "+
					"the polite interval. Retrying a 4xx is the classic sender bug: it cannot "+
					"succeed, and it converts your delivery engine into their DoS engine."),
			text("retry-backoff",
				"Exponential backoff is arithmetic with a purpose: each retry waits roughly "+
					"twice as long as the last, giving a recovering endpoint exponentially "+
					"more breathing room while bounding your own total effort. The simulator's "+
					"knobs are the standard set — five attempts maximum, one-second base, "+
					"thirty-second cap — and the schedule function is pure data, the same "+
					"function the live demo's ledger rows come from:"),
			code("retry-schedule", "go",
				"The ladder's knobs and its schedule: computed waits, capped, attempt one free.",
				"server/internal/domain/demos/webhook_simulator.go",
				"const (\n"+
					"    webhookMaxAttempts  = 5\n"+
					"    webhookBaseDelay    = 1 * time.Second\n"+
					"    webhookMaxDelay     = 30 * time.Second\n"+
					"    webhookDelayPattern = \"base × 2^(attempt-1), capped at max\"\n"+
					")\n"+
					"\n"+
					"// backoffDelay is the wait BEFORE the given attempt (1-based): base ×\n"+
					"// 2^(attempt-1), capped at max. Attempt 1 waits nothing — the schedule\n"+
					"// spaces out the RETRIES, not the first try. The result is data: callers\n"+
					"// decide how to wait (or, in this simulation, just report it).\n"+
					"func backoffDelay(attempt int, base, max time.Duration) time.Duration {\n"+
					"    if attempt <= 1 {\n"+
					"        return 0\n"+
					"    }\n"+
					"    delay := base\n"+
					"    for i := 2; i < attempt; i++ {\n"+
					"        delay *= 2\n"+
					"        if delay >= max {\n"+
					"            return max\n"+
					"        }\n"+
					"    }\n"+
					"    return delay\n"+
					"}"),
			text("retry-read",
				"Read the schedule against a concrete ladder, because the shape is the lesson. "+
					"Attempt 1 waits nothing — the first try is not a retry. After it fails, "+
					"the sender waits 1s; after the second failure, 2s; then 4s; then 8s. So a "+
					"delivery that succeeds on attempt 3 cost 1+2 = 3 seconds of patience, and "+
					"one that exhausts all five attempts spent 15 seconds trying — the demo's "+
					"two ledgers show exactly these totals, computed by exactly this function, "+
					"with no sleeping anywhere: the waits are DATA the simulation records, not "+
					"time it burns. The ``max`` cap is the piece hand-rolled ladders forget: "+
					"without it, doubling reaches absurdity (attempt 15 would \"wait\" 4+ "+
					"hours), the cap bends the curve to a ceiling your operations team can "+
					"reason about. In a real sender this function feeds a timer or a queue's "+
					"visibility timeout; in the demo it feeds a table — same arithmetic, "+
					"different consumer."),
			text("retry-jitter",
				"Pure exponential backoff has a failure mode the demo deliberately does not "+
					"model, and honesty about that is worth a paragraph: synchronization. If "+
					"you retry 500 failing deliveries with the same schedule, they all retry "+
					"at t+1s, t+3s, t+7s — a synchronized thundering herd that beats the "+
					"recovering endpoint back down, so everything retries harder, together, "+
					"forever. Production ladders add JITTER: randomize each wait within a "+
					"band (full jitter picks uniformly in [0, delay]; equal jitter splits the "+
					"difference), decorrelating the herd into a drizzle. The simulator keeps "+
					"the schedule exact — deterministic demos teach better than noisy ones — "+
					"and says so in its config's ``delayPattern``. When you lift this code "+
					"into production, add the jitter at the WAIT site, never in the schedule "+
					"function itself, so the arithmetic stays testable and the noise stays "+
					"where it belongs."),
			text("retry-dlq",
				"Some deliveries never succeed: the endpoint was decommissioned with its "+
					"subscription still live, or the receiver has a bug that turns every "+
					"attempt into a 500. When the ladder exhausts, the delivery must go "+
					"somewhere better than a log line — the DEAD-LETTER QUEUE, a durable "+
					"parking place (a table, a dedicated queue, a bucket) holding the full "+
					"delivery: the event body, the signature material, the endpoint, and the "+
					"ledger of every attempt with its status and wait. The record exists for "+
					"two operational motions. REPLAY: fix the receiver, replay the delivery, "+
					"the event lands late but lands — at-least-once strikes again, so the "+
					"receiver's dedupe must survive a replayed delivery from hours ago. "+
					"REAP: discover the endpoint that has been dead-lettering for a week and "+
					"deactivate its subscription. A DLQ nobody reads is just a slower way of "+
					"dropping data; the demo's second ledger ends in a replayable "+
					"``webhook-dlq`` record precisely so the story ends in operations, not "+
					"silence."),
			demo("api:webhook-simulator", "Webhook delivery simulator",
				"One signed delivery, four verification verdicts (honest, tampered, stale, wrong key), two endpoints walked through the retry ladder — one recovers on attempt 3, one dead-letters after 5 — and the receiving side's fast 2xx, all computed for real by the guide's own HMAC and backoff code."),
			text("retry-demo-read",
				"The simulator above is not a mock-up of these mechanisms; it IS them. The "+
					"signing section's four verdicts come from running the real gate over the "+
					"real HMAC; the two ledgers come from walking the real ``backoffDelay`` "+
					"over two endpoint scripts; the receiver beat answers 200 through the same "+
					"front door the receiving section quotes. Watch the two ledgers side by "+
					"side: same ladder, different worlds — ``dlv_0207`` burns 3 seconds and "+
					"recovers on attempt 3 (a blip in an otherwise healthy integration), "+
					"while ``dlv_0208`` spends 15 seconds across five attempts and lands in "+
					"the DLQ with its replay door open. That contrast is the sender's whole "+
					"story in one screen: the ladder absorbs ordinary failure quietly and "+
					"escalates genuine failure visibly."),
		),
		// ── 5. Receiving webhooks ──────────────────────────────────────
		section("receiving", "Receiving: verify, answer 2xx, process async",
			"You are now somebody's integration target — the front door has three jobs, in one order, and none of them is business logic.",
			text("recv-verify-first",
				"A receiving handler's first act is to NOT trust the request, and its tool is "+
					"the gate from the signing section. Everything about the inbound delivery "+
					"is hostile until verified: the body may be forged, the signature may be "+
					"replayed from yesterday, the header may be malformed. Verification runs "+
					"over the RAW body bytes — read the stream once with ``io.ReadAll`` and "+
					"hash those exact bytes; parsing the JSON first and re-marshaling it "+
					"changes key order and whitespace, changing the bytes, breaking the HMAC "+
					"for a perfectly honest delivery. The header parse is deliberately "+
					"unforgiving — any malformed or incomplete ``t=…,n=…,v1=…`` line is a "+
					"refusal, because guessing defaults in a security gate is how a missing "+
					"signature becomes an accepted delivery:"),
			code("recv-parse-header", "go",
				"Header parsing that refuses to guess: three known fields, all required.",
				"server/internal/domain/demos/webhook_simulator.go",
				"func parseSignatureHeader(header string) (signatureParts, error) {\n"+
					"    var parts signatureParts\n"+
					"    seen := 0\n"+
					"    for _, field := range strings.Split(header, \",\") {\n"+
					"        key, value, found := strings.Cut(strings.TrimSpace(field), \"=\")\n"+
					"        if !found {\n"+
					"            return signatureParts{}, fmt.Errorf(\"malformed signature field %q\", field)\n"+
					"        }\n"+
					"        switch key {\n"+
					"        case \"t\":\n"+
					"            parts.Timestamp = value\n"+
					"        case \"n\":\n"+
					"            parts.Nonce = value\n"+
					"        case \"v1\":\n"+
					"            parts.Signature = value\n"+
					"        default:\n"+
					"            return signatureParts{}, fmt.Errorf(\"unknown signature field %q\", key)\n"+
					"        }\n"+
					"        seen++\n"+
					"    }\n"+
					"    if seen != 3 || parts.Timestamp == \"\" || parts.Nonce == \"\" || parts.Signature == \"\" {\n"+
					"        return signatureParts{}, fmt.Errorf(\"incomplete signature header %q\", header)\n"+
					"    }\n"+
					"    return parts, nil\n"+
					"}"),
			text("recv-2xx-fast",
				"The second act is the one teams get wrong: answer 2xx IMMEDIATELY after "+
					"verification, before any processing. The sender's retry clock is ticking — "+
					"from the sender's side, a handler that spends nine seconds updating the "+
					"database and then returns 200 is indistinguishable from one that hung, "+
					"and a timeout mid-processing triggers a retry that makes the receiver's "+
					"own work the source of duplicate load. So the front door answers "+
					"``200 {\"received\":true}`` — meaning received and VERIFIED, not done — and "+
					"refuses with 401 when the gate says no. Refusals deserve the same "+
					"speed: verify-and-reject is microseconds of arithmetic, and a fast 401 "+
					"tells a misconfigured sender the truth while there is still a developer "+
					"watching. What the answer must never be is slow, and what it must never "+
					"do is lie with a 200 for an unverified body: senders stop retrying on "+
					"2xx, so a premature 200 is data loss with a receipt."),
			code("recv-front-door", "go",
				"The whole front door: parse, verify, answer — nothing else.",
				"server/internal/domain/demos/webhook_simulator.go",
				"func receiveDelivery(secret, headerLine string, receivedAt time.Time, rawBody []byte) receptionOutcome {\n"+
					"    parts, err := parseSignatureHeader(headerLine)\n"+
					"    if err == nil {\n"+
					"        err = verifyDelivery(secret, parts, receivedAt, rawBody)\n"+
					"    }\n"+
					"    if err == nil {\n"+
					"        return receptionOutcome{status: httpStatusOK, body: `{\"received\":true}`}\n"+
					"    }\n"+
					"    return receptionOutcome{status: httpStatusUnauthorized, body: `{\"received\":false}`, reason: err.Error()}\n"+
					"}"),
			text("recv-async",
				"The third act happens somewhere else: processing, asynchronously. The handler "+
					"hands the VERIFIED event — body bytes, event id, headers worth keeping — "+
					"to a worker through a channel, a queue, or this guide's topic-29 "+
					"background-jobs machinery, and forgets it. The split pays three times. "+
					"The sender gets its fast 2xx no matter how heavy processing is. "+
					"Processing failures become RETRYABLE jobs instead of webhook redelivery "+
					"storms — your worker's own backoff, your own DLQ, at-least-once on both "+
					"sides of the wall. And the handler stays thin, exactly like every "+
					"handler in this guide: parse, verify, hand off, answer — the family "+
					"invariant (no business logic in handlers) with a security gate bolted "+
					"to the front. Note the demo's receiver beat numbers the steps in this "+
					"exact order: read raw, verify, answer, hand off, dedupe — steps three "+
					"through five all happen after the answer."),
			text("recv-dedupe",
				"Async processing is where at-least-once finally comes home to roost, because "+
					"now TWO layers may redeliver: the sender (it never saw your 200) and "+
					"your own job queue (the worker died mid-task). The dedupe key is the "+
					"payload's ``eventId``, and the pattern is claim-before-work: insert the "+
					"event id into an insert-only table inside a transaction, and if the "+
					"insert conflicts, someone already processed (or is processing) this "+
					"event — skip. Doing the insert BEFORE the work, transactionally, is "+
					"what makes it safe: claim after work loses the race where two workers "+
					"process the same event in parallel, and claim without a transaction "+
					"loses it to a crash between claim and work. The second arrival of "+
					"``evt_0142`` becomes a logged no-op instead of a double-charged "+
					"invoice, and your on-call rotation stays quiet. Every front door in "+
					"this topic compresses to one ordered discipline:"),
			list("recv-front-door-steps", true,
				"Read the raw body — io.ReadAll once; no parsing before the signature has been checked over the exact bytes.",
				"Parse the signature header — three known fields, all required; anything malformed is an immediate refusal.",
				"Verify the gate — timestamp window first, then the constant-time HMAC comparison; rejection is a 401, fast.",
				"Answer 2xx immediately — received-and-verified, not processed; the sender's retry clock stops here.",
				"Hand off to a worker — channel or queue with its own retry and DLQ; the handler is done and stays thin.",
				"Dedupe by eventId — transactional claim-before-work, so duplicates from either side become logged no-ops."),
			text("recv-failure-manners",
				"One last receiving rule about FAILURE MANNERS, because your status codes are "+
					"someone else's retry policy. Answer 2xx for anything you have taken "+
					"ownership of — verified and enqueued is done, even if processing later "+
					"fails. Answer 401 for verification failures and mean it (never 500: the "+
					"sender would retry a signature that can never verify). Reserve 500 for "+
					"the rare \"I could not even take ownership\" — enqueue failed — knowing "+
					"each such answer triggers the sender's ladder. And keep an operational "+
					"log of verdicts keyed by request id, so when a sender's support team "+
					"asks \"why did you reject delivery dlv_0208 at 09:07\", your answer is a "+
					"log line, not an archaeology project. Receiving well is mostly having "+
					"the story of every delivery you can tell back."),
		),
		// ── 6. The local dev story ─────────────────────────────────────
		section("local-dev", "The local dev story",
			"Nobody can develop a receiver against a webhook that cannot reach their laptop — the local loop is a first-class design problem.",
			text("dev-problem",
				"The wall hits every webhook developer on day one: the sender (a payment "+
					"provider, a git host) must POST to a URL, and your dev server is "+
					"``localhost:8100`` behind a router that would laugh at an inbound "+
					"connection. Waiting for staging to test every receiver change is a "+
					"feedback loop measured in deploys, which is where webhook projects go "+
					"to die. The local dev story is therefore part of the receiver's design: "+
					"get real deliveries onto your machine, and get deterministic "+
					"repetitions of them without depending on the sender's mood. Every "+
					"tool below is the same trick at different price points — a bridge "+
					"from a public URL to your localhost:"),
			list("dev-tools", false,
				"Tunnel services — ngrok, Cloudflare Tunnel, localtunnel: a public HTTPS URL that forwards to your localhost; point the sender's webhook config at it.",
				"Browsers-as-repeaters — webhook.site, RequestBin: a disposable URL that logs everything POSTed at it; perfect for reading what a provider ACTUALLY sends.",
				"Replay harnesses — Stripe's CLI, GitHub's webhook redelivery: the provider re-POSTs a past delivery on demand, so the same event hits your receiver every iteration.",
				"Hand-rolled fixtures — a curl script or a Go test posting a saved body with a computed HMAC: the guide's simulator, in miniature, with zero third-party dependency."),
			text("dev-fixtures",
				"Fixtures are the quiet winner of that list, because they work offline, in "+
					"CI, and at 3am. Save one real delivery per event type — body bytes, "+
					"headers, and the verdict your gate produced — as a test fixture; then "+
					"a receiver regression is a ``go test`` away instead of a "+
					"redeploy-and-pray. This guide's own simulator is that idea carried to "+
					"its logical end: fixed clocks, fixed keys, fixed endpoint scripts, so "+
					"the entire webhook pipeline — signing, verification, the retry "+
					"ladder, the DLQ — runs as a deterministic unit test. The same "+
					"discipline extends to YOUR sender: a test that walks "+
					"``backoffDelay`` and asserts the schedule (0s, 1s, 2s, 4s, 8s, then "+
					"the cap) needs no HTTP server, no sleeping, and no luck. Determinism "+
					"is what makes webhook code testable; wall clocks and real networks "+
					"are what make it untestable — choose accordingly."),
			text("dev-verification-loop",
				"The full local loop, assembled: run the tunnel, point the provider's "+
					"webhook settings at it, and let ONE real delivery land in a file; "+
					"capture it with ``webhook.site`` first if you want to read the "+
					"provider's exact header conventions before writing any code. Turn "+
					"that capture into a fixture, write the gate test against it (verified "+
					"must pass, every hostile variant must fail), then build the handler "+
					"around the gate until the fixture round-trips. From then on the "+
					"provider's redelivery button is your integration test and the fixture "+
					"is your unit test, and the tunnel only comes back when a NEW event "+
					"type lands. Notice what this loop buys beyond convenience: every "+
					"verification decision you make — tolerance window, header format, "+
					"dedupe key — is pinned by a test that fails loudly when someone "+
					"changes it, which is exactly the property webhook integrations lose "+
					"when their only test environment is production."),
		),
		// ── 7. Terminal companion ──────────────────────────────────────
		section("terminal-companion", "Terminal companion: the curl tour",
			"The webhook machinery, observed from the shell — verdicts, ledgers, and the receiver's fast answer.",
			text("term-pitch",
				"With the stack up (server on :8100), these commands walk the simulator's "+
					"whole story. The demo payload is deterministic — computed HMACs over "+
					"fixed clocks, a backoff ladder of pure arithmetic — so repeated calls "+
					"are worth diffing, and every number below is stable: the four verdicts, "+
					"the 0/1/2s ladder that recovers, the 0/1/2/4/8s ladder that dead-letters, "+
					"and the receiving side's immediate 200. ``python3 -m json.tool`` keeps "+
					"the tour dependency-free:"),
			code("term-simulator", "bash", "", "",
				"# the simulator in full: config knobs, four signature verdicts, two retry\n"+
					"# ledgers, the receiver's front door\n"+
					"curl -s localhost:8100/api/demo/api/webhook-simulator | python3 -m json.tool"),
			code("term-verdicts", "bash", "", "",
				"# just the verification verdicts — one verified, three refusals with reasons\n"+
					"curl -s localhost:8100/api/demo/api/webhook-simulator | python3 -c '\n"+
					"import json,sys; print(json.dumps(json.load(sys.stdin)[\"signing\"][\"cases\"], indent=2))'"),
			code("term-ladder", "bash", "", "",
				"# the retry ladder's two endings: delivered on attempt 3, dead-lettered after 5\n"+
					"curl -s localhost:8100/api/demo/api/webhook-simulator | python3 -c '\n"+
					"import json,sys; [print(d[\"deliveryId\"], d[\"outcome\"], d[\"totalWaitSeconds\"], \"s total\") for d in json.load(sys.stdin)[\"deliveries\"]]'"),
			code("term-method", "bash", "", "",
				"# the endpoint is a GET: a POST draws the router's truthful 405 + Allow header\n"+
					"curl -s -i -X POST localhost:8100/api/demo/api/webhook-simulator"),
			code("term-topic", "bash", "", "",
				"# this topic in full: sections → text/code/list/demo blocks\n"+
					"curl -s localhost:8100/api/content/webhooks | python3 -m json.tool"),
			text("term-coda",
				"The verdicts tour is the one to linger on: each refusal carries the gate's "+
					"own reason — the mismatch for the tampered body and the wrong key, the "+
					"stale-window rejection for the replay — and each reason maps to a check "+
					"in ``verifyDelivery`` you read three sections ago. For the sender's "+
					"neighbors in the curriculum, topic 11 (api-design) owns the request-"+
					"response contract this topic reversed, and topic 22 (messaging) owns the "+
					"brokered world webhooks deliberately are not; topic 29 (background-jobs) "+
					"is where the receiver's async half goes to be built properly."),
		),
	}

	return NewTopic(
		"webhooks",
		"Webhooks",
		"Reversed APIs: signing with HMAC, retrying with backoff, dead-lettering with dignity — on both ends of the wire.",
		"apis",
		14,
		sections,
	)
}
