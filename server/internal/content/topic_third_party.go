package content

import (
	"errors"
	"fmt"
)

// This file is topic 42 third-party (group capstones, curriculum
// order 42 — the second capstone): the dependencies capstone. Forty
// topics built things and wired the stores this repo owns; this one
// is about everything the app USES that it did not build — the
// modules in the manifest, the relay/processor/bucket surfaces every
// real application eventually faces, and the keys, quotas, and blast
// radii each integration brings. Everything quoted is this repo's
// real surface: the manifest read as a decision record, the webhook
// HMAC signer, the files Store port, the optional-service timeouts,
// the jobs topic's email-as-a-job handler, the prod compose secret
// rail — and, new in this topic, a real adapter-surface hub: three
// vendor ports (mailer, payments, object storage) each carrying two
// Go implementations (a deterministic sandbox and a credential-gated
// live skeleton) behind the same ScenarioStore switch the whole guide
// uses. Vendor SDKs this repo deliberately does not run — stripe-go,
// aws-sdk-go-v2, mail clients — live in comparison PROSE and honest
// recipes, never fabricated as live integrations.

// thirdPartyTopic builds topic 42.
func thirdPartyTopic() (Topic, error) {
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
		// ── 1. Integrating versus building ───────────────────────
		section("integrating-vs-building", "Integrating versus building",
			"This section carries the topic's demo — the third-party hub.",
			text("tp-ivb-pitch",
				"Third-party means code you did not write, running on a schedule you do not control: a module pinned in "+
					"your manifest, a service on someone else's network, an image in someone else's registry. Every feature "+
					"beyond your core domain eventually arrives as the same decision — build it or integrate it — and the "+
					"question is never whether you CAN build it (you can build anything given enough quarters) but which side "+
					"of the line the piece belongs on. Build it and you own the edge cases forever, but the behavior is exactly "+
					"yours; integrate it and the edge cases become someone else's day job, but you inherit their API shape, "+
					"their release cadence, their outages, and their invoice. Go's standard library sharpens the question "+
					"because its floor is so high: net/http, crypto/hmac, encoding/json, and the testing package cover ground "+
					"other ecosystems hand to third parties on day one, so in Go the integrate-or-build line starts further "+
					"out than almost anywhere else. This repo — a teaching guide that runs as a real stack — made the decision "+
					"for every piece it uses, and the decisions are on the record, which makes them teachable."),
			text("tp-ivb-axes-pitch",
				"Six axes decide most integrate-versus-build questions, and the honest version scores each one rather than "+
					"gesturing at all of them:"),
			list("tp-ivb-axes",
				false,
				"Build cost — the true weeks to write, debug, and harden the piece, counted against the day its edge cases arrive: timezone arithmetic, greylisting on mail sockets, idempotency corners on money.",
				"Integration cost — the adapter you write anyway, the config surface, the version pins, the upgrade treadmill: integrating is never zero engineering, it is different engineering.",
				"Blast radius — what a defect in the piece can take down when it misbehaves, and whether the failure is loud (an error) or silent (wrong answers with a 200).",
				"Operational ownership — who gets paged when it breaks: you at 3am for your code, a vendor's status page for theirs; the pager is a cost the invoice never shows.",
				"Reversibility — how expensive the swap-out is if the choice proves wrong: an interface in front of the decision is cheap to reverse, a scattered import of the SDK is not.",
				"Competence boundary — the categories where subtly-wrong is catastrophic (cryptography, money, parsing hostile input) and building is malpractice regardless of team skill.",
			),
			text("tp-ivb-manifest-pitch",
				"The bought column is literally on the record: the server's manifest. Each of its direct requires is a "+
					"decision that this repo does not want to own that problem — read as decisions, not a shopping list:"),
			code("tp-ivb-gomod-quote", "mod",
				"The bought column: direct requires, each a problem this repo declined to own.",
				"server/go.mod",
				"require (\n"+
					"\tgithub.com/hibiken/asynq v0.26.0\n"+
					"\tgithub.com/jackc/pgx/v5 v5.11.0\n"+
					"// …\n"+
					"\tgolang.org/x/crypto v0.58.0\n"+
					"\tgolang.org/x/time v0.16.0\n"+
					"// …\n"+
					")"),
			text("tp-ivb-manifest-reading",
				"Score the visible lines against the axes. asynq is operational ownership: queue semantics, retries, cron, "+
					"and worker supervision are a product, so topic 29 kept only the retry POLICY pure in-domain and let the "+
					"broker execute its verdicts. pgx is the wire-driver decision — PostgreSQL access is deep vendor surface "+
					"and the driver is bought, then BOUNDED by this repo's own timeout discipline. golang.org/x/crypto is the "+
					"competence boundary in one line: the argon2id password hashing behind the authentication topic is "+
					"cryptography, and building it would be malpractice; x/time carries the token-bucket rate limiter the "+
					"security topic uses, the same verdict at a different layer. The manifest's biggest entry is what is NOT "+
					"there: no chi, gin, or echo — the routing topic proved ServeMux patterns carry the app — and no testify, "+
					"because the standard library's table-driven tests already express the assertions. Topic 31 walked the "+
					"whole dependency story; this topic walks the vendor-shaped half the manifest cannot show."),
			text("tp-ivb-built-pitch",
				"The built column is just as deliberate, and it is not not-invented-here — each build decision names what it "+
					"declined to buy and why the build is small. The clearest example is the webhook signer the API topics "+
					"built over the standard library: crypto/hmac and crypto/sha256 ARE the standard library's exported "+
					"competency, so using them is integrating one level down. The signer is one honest function:"),
			code("tp-ivb-built-quote", "go",
				"The built column's exhibit: machine-to-machine authentication over the standard library — bought nothing, hid nothing.",
				"server/internal/domain/demos/webhook_simulator.go",
				"func signDelivery(secret, timestamp, nonce string, body []byte) string {\n"+
					"\tmac := hmac.New(sha256.New, []byte(secret))\n"+
					"\tfmt.Fprintf(mac, \"%s.%s.%s\", timestamp, nonce, body)\n"+
					"\treturn hex.EncodeToString(mac.Sum(nil))\n"+
					"}"),
			text("tp-ivb-built-reading",
				"Why build this and buy argon2? Because the competence boundary sits in a different place for each: password "+
					"hashing has no safe local implementation short of the full argon2 construction, while HMAC-SHA256 is a "+
					"single constructor call the standard library already owns. The same reasoning built the JWT engine as "+
					"stdlib HS256 (the token format is small enough that a dependency buys nothing a screen of honest code "+
					"cannot — topic 23), the rate limiter over x/time (the bucket is a few dozen lines; the POLICY is the "+
					"lesson), and the observability stack by hand (a Prometheus text exposition in one file, a W3C "+
					"tracecontext propagator in another — topic 33). Built is a teaching choice here; in your production repo "+
					"it is a cost choice scored on the same six axes, and the axes usually agree: buy the crypto constructions "+
					"nobody should hand-roll, buy the engines whose edge cases are a product, and build the seams."),
			text("tp-ivb-demo-pitch",
				"The demo puts the whole discipline in one payload, and unlike a checklist it is MACHINERY: three vendor "+
					"ports — a mailer, a payment processor, an object store — each with two real Go implementations behind "+
					"it. The deterministic sandbox adapters do real work: HMAC signing and verification, idempotency-key "+
					"replay and conflict detection, quota accounting that burns down and refuses at the ceiling, "+
					"content-addressed dedupe, and a vendor-error mapping that runs this app's real taxonomy functions. The "+
					"live adapters are credential-gated skeletons with the exact shape a stripe-go or aws-sdk-go-v2 adapter "+
					"takes: they resolve the credential's env var NAME, refuse honestly when it is absent, refuse honestly "+
					"(differently) when it is present, and never invent a vendor response. A real switch selects the wired "+
					"adapter per vendor from the shared scenario store — the same runtime swap the posts surface uses."),
			demo("thirdparty:hub", "The third-party hub",
				"GET /api/demo/thirdparty/hub serves the adapter surface, driven live on this very request · the WIRING "+
					"panel — the shared scenario store's data mode selecting sandbox or skeleton per vendor (PATCH "+
					"/api/demo/scenario to flip it and reload), with one observed probe through each ACTIVE adapter · three "+
					"VENDOR panels — the mailer (greylisting, quota burn-down to the 429, signed acceptances verified honest "+
					"and tampered), the payment processor (idempotency-key replay and same-key-different-money conflict, a "+
					"decline classified input-shaped, the signed charge.succeeded event verified fresh, tampered, and stale), "+
					"and the object store (hostile keys refused, content-addressed dedupe, byte and put budgets, not-found "+
					"gets) — each beside its live skeleton's honest refusal naming the env var and the real call site · the "+
					"TAXONOMY map — every vendor error code the sandboxes can answer with, mapped onto the app's eight kinds "+
					"with HTTP statuses derived through the real core.HTTPStatusFor. Every count was counted during the run "+
					"that served it; every signature is real HMAC-SHA256 at a fixed exhibit clock, so two GETs serve "+
					"byte-identical bodies; no vendor network call is ever made"),
			text("tp-ivb-demo-reading",
				"How to read the panel. Start with the wiring rows: they prove the switch is real, because PATCHing the "+
					"scenario's data mode to live and reloading flips every row to the skeletons and the active probes "+
					"answer the credential refusal instead of sandbox receipts. Then read each vendor's sandbox rows for "+
					"the behavior names this section taught — the mailer's greylist-then-accept is the SMTP protocol's own "+
					"retry shape, the processor's replay row answers the ORIGINAL charge id, the store's dedupe row shares "+
					"the first put's etag — and the live panel beside each for the honest refusal. The port claims are "+
					"compile-time assertions in the source, quoted next. Every failure row carries its vendor code, the "+
					"vendor's status, the classified kind, and OUR status for that kind — the 402 decline mapping to a 422 "+
					"is the adapter-owns-the-mapping lesson in one row."),
			code("tp-ivb-port-quote", "go",
				"A vendor port: one method, vendor errors in the signature — and both implementations compile against it by assertion, not convention.",
				"server/internal/domain/demos/thirdparty_hub.go",
				"type ThirdPartyMailer interface {\n"+
					"\t// Send hands one message to the relay and answers the receipt, or\n"+
					"\t// a *ThirdPartyVendorError the adapter maps onto the taxonomy.\n"+
					"\tSend(ctx context.Context, draft ThirdPartyMailDraft) (ThirdPartySendReceipt, error)\n"+
					"}\n"+
					"// …\n"+
					"var (\n"+
					"\t_ ThirdPartyMailer = (*thirdPartySandboxMailer)(nil)\n"+
					"\t_ ThirdPartyMailer = (*thirdPartyLiveMailer)(nil)\n"+
					")"),
			text("tp-ivb-handler-pitch",
				"And the endpoint that serves it all is the thinnest handler in the capstones — three lines, because the "+
					"machinery lives behind the ports and the pure builder, which is the layering lesson (topic 32) "+
					"standing still:"),
			code("tp-ivb-handler-quote", "go",
				"The hub's whole handler: snapshot the switch's source, hand the builder the facts, write the payload.",
				"server/internal/presentation/demo_thirdparty.go",
				"func (h *ThirdPartyHubHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {\n"+
					"\tfacts := demos.ThirdPartyHubFacts{Scenario: h.store.Snapshot(), Credentials: h.credentials}\n"+
					"\twriteJSON(w, http.StatusOK, demos.BuildThirdPartyHubPayload(facts))\n"+
					"}"),
			text("tp-ivb-coda",
				"The rules of thumb the hub demonstrates, in descending order of confidence: never build crypto constructions "+
					"(the boundary is absolute — buy argon2, buy TLS, sign with the standard library's hmac rather than your "+
					"own arithmetic); rarely build engines, queues, or parsers (the build cost is a product and the ecosystem "+
					"already paid it); always build the seam — the interface in front of the vendor, because nobody else can "+
					"write your domain's half of it and it is the one part that survives a vendor swap; buy the format, keep "+
					"the key and the policy; and score every decision on the six axes with numbers, not vibes. The rest of "+
					"this topic walks the vendor-shaped problems every real app meets — email, payments, object storage — "+
					"through the same decision, honestly: what this repo really has, and what the recipe would be."),
		),
		// ── 2. Vendor ports and adapters ─────────────────────────
		section("vendor-ports-and-adapters", "Vendor ports and adapters",
			"The seam itself: a port per vendor, two implementations, one switch — and one error taxonomy under all of it.",
			text("tp-vpa-pitch",
				"An integration's quality is decided at the seam, before any vendor is chosen. The shape this guide uses "+
					"everywhere — and the hub implements for real — is three pieces: a PORT, a small interface in the domain "+
					"that speaks the app's vocabulary; ADAPTERS, one or more implementations that translate to and from a "+
					"vendor's shape; and a SWITCH, the composition-root wiring that decides which adapter is live. Topic 5 "+
					"taught the interface half (accept interfaces, return structs), topic 32 the layering (presentation → "+
					"domain ← data), and topic 26's files workbench already runs this exact shape with a Store port over a "+
					"local adapter. What the hub adds is the full pair per vendor: a deterministic sandbox and a live "+
					"skeleton behind one interface, switchable at runtime."),
			text("tp-vpa-switch-pitch",
				"The switch is not new machinery — it is the same runtime swap this guide has used since the data phase, "+
					"when the posts surface learned to answer from fake in-memory data or the live stores with one PATCH. "+
					"The hub's version reads the shared scenario store's data mode and wires every vendor port the same way:"),
			code("tp-vpa-switch-quote", "go",
				"The DI switch: fake mode wires deterministic sandboxes, live mode wires credential-gated skeletons — one function, three ports.",
				"server/internal/domain/demos/thirdparty_hub.go",
				"func WireThirdParty(mode core.DataMode, credentials func(string) (string, bool)) ThirdPartyWired {\n"+
					"\tif mode == core.DataModeLive {\n"+
					"\t\treturn ThirdPartyWired{\n"+
					"\t\t\tMailer:   &thirdPartyLiveMailer{cfg: thirdPartyLiveMailerCfg, credentials: credentials},\n"+
					"\t\t\tPayments: &thirdPartyLivePayments{cfg: thirdPartyLivePaymentsCfg, credentials: credentials},\n"+
					"\t\t\tStore:    &thirdPartyLiveStore{cfg: thirdPartyLiveStoreCfg, credentials: credentials},\n"+
					"\t\t}\n"+
					"\t}\n"+
					"\treturn ThirdPartyWired{\n"+
					"\t\tMailer:   newThirdPartySandboxMailer(),\n"+
					"\t\tPayments: newThirdPartySandboxPayments(),\n"+
					"\t\tStore:    newThirdPartySandboxStore(),\n"+
					"\t}\n"+
					"}"),
			text("tp-vpa-switch-reading",
				"The scenario store it reads is the guide's single runtime toggle — a mutex-guarded snapshot every demo "+
					"endpoint shares, patchable over HTTP, validating its own enum before storing. The hub's handler "+
					"_snapshots it once per request and hands it to the pure builder with the process's real credential "+
					"resolver; nothing about the wiring is hidden in a framework's container, because Go's answer to "+
					"dependency injection is constructor arguments at the composition root (topic 32). The sandbox "+
					"adapters are deterministic by construction — fixed scripts, fixed exhibit clock — which is what makes "+
					"the payload byte-stable across reloads while the WIRING panel still moves with the scenario."),
			text("tp-vpa-errors-pitch",
				"The second half of the seam is the error vocabulary. Vendors fail in vendor terms — a 402 card decline, a "+
					"429 quota, a 451 greylist — while this app has exactly eight failure kinds and one error envelope "+
					"(topic 30). The adapter owns the translation, and the hub implements it as a table plus a fallback "+
					"that runs the REAL taxonomy function on raw vendor statuses:"),
			code("tp-vpa-classify-quote", "go",
				"The adapter's mapping: named vendor codes through the table, raw statuses through the real core.KindForHTTPStatus, everything else honestly unavailable.",
				"server/internal/domain/demos/thirdparty_hub.go",
				"func ThirdPartyClassifyVendorError(err error) (core.Kind, bool) {\n"+
					"\tvar vendorErr *ThirdPartyVendorError\n"+
					"\tif !errors.As(err, &vendorErr) {\n"+
					"\t\treturn core.KindServiceUnavailable, false\n"+
					"\t}\n"+
					"\tif row, ok := thirdPartyClassifications[vendorErr.Code]; ok {\n"+
					"\t\treturn row.Kind, true\n"+
					"\t}\n"+
					"\tif kind := core.KindForHTTPStatus(vendorErr.VendorStatus); kind != core.KindInternal {\n"+
					"\t\treturn kind, true\n"+
					"\t}\n"+
					"\treturn core.KindServiceUnavailable, false\n"+
					"}"),
			code("tp-vpa-status-quote", "go",
				"The taxonomy function the fallback leans on — one place where statuses and kinds meet.",
				"server/internal/core/errors.go",
				"func KindForHTTPStatus(status int) Kind {\n"+
					"\tif kind, ok := kindByHTTPStatus[status]; ok {\n"+
					"\t\treturn kind\n"+
					"\t}\n"+
					"\tif status < 500 {\n"+
					"\t\treturn KindNotFound\n"+
					"\t}\n"+
					"\treturn KindInternal\n"+
					"}"),
			text("tp-vpa-errors-reading",
				"Two rules make the mapping honest. First, the vendor's status is DATA, not our status: a card decline "+
					"arrives as the vendor's 402 and leaves as our validation_failed 422, because from OUR caller's side a "+
					"decline is input that will never succeed as-is — the row keeps both numbers so the translation is "+
					"visible. Second, an unclassifiable vendor failure degrades to service_unavailable, never internal: "+
					"the internal kind means WE broke, and a dependency we cannot understand is a dependency problem. The "+
					"same stance is why the optional stores degrade to typed 503s instead of hanging (topics 19, 20) — the "+
					"adapter is where vendor chaos is folded into the eight kinds the envelope knows, so no handler ever "+
					"strings-matches a vendor message."),
			text("tp-vpa-sandboxes-pitch",
				"Why sandboxes at all — why not stubs that return nil? Because a stub proves nothing: it exercises no "+
					"signing, burns no quota, races no idempotency key. The hub's sandboxes implement each vendor's "+
					"OBSESSABLE behaviors for real — greylisting per new address, a per-window send quota, idempotent "+
					"charge replay that refuses a same-key-different-amount bug, content-addressed etags, byte and put "+
					"budgets — so the adapter patterns (retry the transient, refuse the input-shaped, dedupe the replay) "+
					"are executed by Go code a test can drive, not described by prose. Determinism is a design constraint "+
					"here, exactly as in the webhook simulator: fixed exhibit clocks, counted-not-timed observations, and "+
					"scripts whose labels are DERIVED from what the adapter answered, so a refusal can never wear an "+
					"acceptance label. The live skeletons prove the other half — that the port's contract is complete "+
					"enough for a real client — by refusing honestly at the credential gate instead of faking success."),
		),
		// ── 3. Email over SMTP ───────────────────────────────────
		section("email-over-smtp", "Email, concept-first",
			"The first integration every app needs: a relay, a bounded client, and the quota/signature reality on the far side.",
			text("tp-em-pitch",
				"Email is the first integration nearly every application needs and the one most often underestimated. The "+
					"protocol is old enough to collect a pension (SMTP, 1982), but modern practice is simpler than the "+
					"protocol: you do not talk to recipient mail servers anymore — you hand every message to a relay "+
					"(Postmark, SES, Resend, or a company SMTP host), because deliverability is the provider's "+
					"competency: SPF, DKIM, and DMARC records, IP reputation, greylist and bounce handling. The "+
					"integrate-versus-build answer is therefore settled twice over — build cost (a correct SMTP client "+
					"with TLS and retry semantics) and competence boundary (deliverability) both say integrate — and the "+
					"remaining decision is which client and which discipline to wrap it in. Go's standard library ships "+
					"net/smtp, and honesty requires saying what it is: functional but frozen — no modern OAuth2 auth, no "+
					"connection pooling, unmaintained upstream — so production Go either calls the relay's HTTP API "+
					"directly (an API key and a JSON POST, often the smallest dependency of all: zero) or adopts a "+
					"maintained mail library. This repo sends no real mail and pins no mail dependency; the hub's mailer "+
					"port plus sandbox is the seam either choice plugs into."),
			text("tp-em-timeout-pitch",
				"Whichever client you choose, the discipline is already codified in this repo's optional-service "+
					"adapters: bound every phase of every outbound call, because a relay having a bad day is perfectly "+
					"capable of hanging a handshake for minutes. The redis playground states the policy in its constants:"),
			code("tp-em-timeout-quote", "go",
				"The discipline any mail adapter must copy: two-second bounds on dial, read, and write — degradation in seconds, not vendor timeouts.",
				"server/internal/data/redis/playground.go",
				"const (\n"+
					"\tdialTimeout  = 2 * time.Second\n"+
					"\treadTimeout  = 2 * time.Second\n"+
					"\twriteTimeout = 2 * time.Second\n"+
					"// …\n"+
					"\tpubsubWait = 2 * time.Second\n"+
					")"),
			text("tp-em-timeout-reading",
				"An unbounded SMTP or relay call is the topic 39 pathology verbatim: a parked request multiplying into "+
					"pool exhaustion while the vendor's TCP stack shrugs. If the send happens in a request handler it "+
					"steals the request's budget; if it happens in the worker — where it belongs, next section — it pins "+
					"a worker slot against the queue's whole concurrency. Contexts compose the bound the Go way: the "+
					"adapter's per-call deadline derives from the CALLER's context, so a cancelled request or a shutting-"+
					"down process unwinds the vendor call instead of waiting it out. The recipe below is the shape at "+
					"the mailer port's seam — every phase bounded, the connection always released, credentials from the "+
					"environment never the repo:"),
			code("tp-em-recipe", "bash", "", "",
				"# RECIPE — no mail dependency is pinned in this repo (the hub's mailer port\n"+
					"# is the seam). Two honest shapes, pick per relay: (a) the relay's HTTP API\n"+
					"# with an API key — zero dependencies: \n"+
					"req, _ := http.NewRequestWithContext(ctx, \"POST\", relayURL+\"/v1/email/send\", body)\n"+
					"req.Header.Set(\"Authorization\", \"Bearer \"+os.Getenv(\"MAIL_RELAY_API_KEY\"))\n"+
					"client := &http.Client{Timeout: 5 * time.Second} // the bound, every phase pays it\n"+
					"# (b) classic SMTP over net/smtp (frozen but functional; a maintained library\n"+
					"# earns its pin when you need pooling or OAuth2):\n"+
					"addr := net.JoinHostPort(host, \"587\")\n"+
					"d := net.Dialer{Timeout: 5 * time.Second}\n"+
					"conn, err := tls.Client(d.DialContext(ctx, \"tcp\", addr), &tls.Config{ServerName: host})\n"+
					"# … authenticate, send, and ALWAYS Close in a defer — the relay is never leaked"),
			text("tp-em-sandbox-pitch",
				"What the relay will actually DO to your send is the part no recipe teaches, and it is why the hub's "+
					"mailer sandbox implements the two behaviors every relay exhibits: greylisting — the first send to "+
					"any new address is refused transiently, a retry-shaped protocol feature that separates real senders "+
					"from fire-and-forget spam — and a per-window quota that refuses when spent. Both are in the "+
					"adapter's mapped vocabulary:"),
			code("tp-em-sandbox-quote", "go",
				"The sandbox relay's two real behaviors: a transient greylist worth retrying, a quota worth waiting out — different kinds, different retry verdicts.",
				"server/internal/domain/demos/thirdparty_hub.go",
				"\tif !m.greylisted[to] {\n"+
					"\t\tm.greylisted[to] = true\n"+
					"\t\treturn ThirdPartySendReceipt{}, thirdPartyVendorErrorf(\"greylisted\",\n"+
					"\t\t\t\"the relay greylists the first send to a new address — back off and retry (SMTP 451, by design)\", true)\n"+
					"\t}\n"+
					"\tif m.accepted >= thirdPartyMailerBudget {\n"+
					"\t\treturn ThirdPartySendReceipt{}, thirdPartyVendorErrorf(\"send_quota_exceeded\",\n"+
					"\t\t\tfmt.Sprintf(\"the window's budget of %d accepted sends is spent\", thirdPartyMailerBudget), true)\n"+
					"\t}"),
			text("tp-em-coda",
				"One more property of the email category that generalizes to every provider-shaped integration: the "+
					"provider arrives with a console, a quota, and a webhook habit. The console gives you a sandbox — "+
					"test keys, sandbox addresses, a deliverability dashboard your own logs cannot fake; the quota is a "+
					"sending limit per second or per day your burstiest moment must fit under; and the webhooks are how "+
					"you learn what happened after the handoff — deliveries, bounces, spam complaints arrive as signed "+
					"POSTs to your endpoint, which is the surface topic 14 built and the Stripe section reuses: verify "+
					"the signature, dedupe the delivery, and never trust the body until both have happened. Email is "+
					"also idempotence-hostile by nature — at-least-once infrastructure means the same send can happen "+
					"twice — which is exactly why the send belongs in the next section's queue, not in the request."),
		),
		// ── 4. Email as a job: the recap ─────────────────────────
		section("email-as-a-job-recap", "Email as a job: the recap",
			"Topic 29 already taught the pattern this repo's email story rides — recap, cross-reference, and the seam a real client fills.",
			text("tp-recap-pitch",
				"Topic 29 taught background jobs through email precisely because email has every property a job needs: "+
					"slow, flaky, third-party, and needed exactly-once-ish. This section recaps rather than re-teaches — "+
					"the four beats, the real handler, and where this topic's mailer port meets that machinery. The "+
					"beats: COMPOSE at the boundary (the request validates shape and answers 201 with a job ticket), "+
					"DEFER the send (the draft rides the queue as JSON), SEND in the worker (where seconds of relay "+
					"latency cost nothing), and RECORD the outcome (an attempt trail a support human can read). The "+
					"repo's canonical handler is quotable, and its own comment names this topic:"),
			code("tp-recap-digest-quote", "go",
				"The canonical job from topic 29: parse, one bounded piece of work, record — the exact shape a real send-the-receipt-email task grows from.",
				"server/internal/data/jobs/handlers.go",
				"// digestHandler is the canonical job shape: parse the payload, do a\n"+
					"// small bounded piece of work (here: record it), succeed on the first\n"+
					"// attempt. This is the handler a real \"send the receipt email\" task\n"+
					"// would grow from — the queue carries the DECISION out of band, and the\n"+
					"// handler's job is to finish it durably.\n"+
					"func digestHandler(client *goredis.Client) func(context.Context, *asynq.Task) error {\n"+
					"\treturn func(ctx context.Context, task *asynq.Task) error {\n"+
					"\t\tpayload, err := demos.ParseJobsDemoPayload(task.Payload())\n"+
					"\t\tif err != nil {\n"+
					"\t\t\treturn fmt.Errorf(\"digest task %s: %w\", taskIDFrom(ctx), err)\n"+
					"\t\t}\n"+
					"\t\treturn recordDemoEvent(ctx, client, payload, demos.JobsTaskDigest, retryCountFrom(ctx)+1, demos.JobsOutcomeSucceeded)\n"+
					"\t}\n"+
					"}"),
			text("tp-recap-promise",
				"That comment contains this topic's forward promise — a real send task would grow from this handler — and "+
					"this topic resolves it the honest way: the hub's mailer port is the seam, the sandbox is the "+
					"behavior model, and the recipe in the previous section is the client. Swapping the recorder for a "+
					"real relay adapter changes exactly ONE adapter behind ONE port — the pattern's whole payoff, and "+
					"the clearest demonstration in this repo of why the integrate-versus-build decision should always "+
					"leave a port at the seam. The worker side already owns the hard parts the relay will throw at it: "+
					"the retry ladder with its bounded exponential policy, the poison classification that parks "+
					"malformed drafts without spending one retry, the durable attempt counter that counts double "+
					"deliveries honestly (a redis INCR keyed by the task's stable id — the have-we-met state), and the "+
					"dead-letter archive an operator reads after the incident."),
			text("tp-recap-idempotence",
				"Two production notes from topic 29 worth repeating because they ARE the third-party reality of email. "+
					"First, sending is idempotence-hostile: at-least-once delivery means the same job can run twice, "+
					"and the mitigation is a sent-marker keyed by the business event id, checked BEFORE the side effect "+
					"— the same check-and-mark the messaging topic's Deduplicator performs atomically, and the same "+
					"discipline the payments sandbox in this topic's hub enforces with idempotency keys. Second, the "+
					"address rule belongs at BOTH ends even though it looks redundant — the boundary check gives the "+
					"user a fast 422, the worker check keeps the queue honest for every other door that can enqueue: "+
					"replays, other enqueuers, a manual broker call. Both notes are one rule wearing two costumes: the "+
					"queue is a door you do not fully control, so the contract is re-derived wherever the work runs."),
		),
		// ── 5. Stripe: payments ──────────────────────────────────
		section("stripe-payments", "Stripe: payments, the archetype",
			"Payments as the SaaS integration to learn from: tokenization, signed events, idempotency — all three already in this repo's vocabulary.",
			text("tp-st-pitch",
				"Payments are the category where integrate-versus-build has exactly one answer, and it is not close: "+
					"you never build payment processing, and you never touch a card number. PCI DSS compliance for raw "+
					"card data is a multi-year audit program; letting the provider's browser widgets tokenize the card "+
					"means the number never crosses your infrastructure at all — your API carries tokens, amounts, and "+
					"metadata. Honesty up front, as the whole topic practices: the Stripe SDK (stripe-go) is NOT a "+
					"dependency of this repo, nothing here charges anything, and the SDK lives in this section's prose "+
					"and recipe only. What this section teaches is the SHAPE, because Stripe is the best-documented "+
					"instance of a pattern this repo already owns — and recognizing a pattern you own beats memorizing "+
					"a vendor's docs. A payments integration is three surfaces:"),
			list("tp-st-surfaces",
				false,
				"The API surface — your server calls the vendor's API (create a PaymentIntent, confirm a subscription) with a secret key: the same outbound-call discipline as every adapter here — bounded, credentialed from env, behind a port.",
				"The webhook surface — the vendor POSTs events to YOUR endpoint (payment succeeded, charge refunded), signed and at-least-once: verify the signature, dedupe the delivery — exactly topic 14's receiver and topic 22's idempotent consumer.",
				"The console surface — test mode with sandbox cards, event replay, and a dashboard of every charge: the provider's dev experience becomes your debugging environment the way the scenario store is this repo's.",
			),
			text("tp-st-signature-pitch",
				"The first mapping is the deep one. Stripe's webhook signature header carries a timestamp and an "+
					"HMAC-SHA256 over the exact request body — the same design as this repo's webhook gate, down to the "+
					"two checks and their order. The receiver half, quoted from the webhooks exhibit:"),
			code("tp-st-signature-quote", "go",
				"The design Stripe's header follows: freshness first (the cheapest gate), then a constant-time MAC over the exact bytes.",
				"server/internal/domain/demos/webhook_simulator.go",
				"\tif delta := receivedAt.Sub(issuedAt); delta < -webhookTolerance || delta > webhookTolerance {\n"+
					"\t\treturn errStaleTimestamp\n"+
					"\t}\n"+
					"\texpected := signDelivery(secret, parts.Timestamp, parts.Nonce, body)\n"+
					"\tif !hmac.Equal([]byte(expected), []byte(parts.Signature)) {\n"+
					"\t\treturn errSignatureMismatch\n"+
					"\t}\n"+
					"\treturn nil"),
			text("tp-st-signature-reading",
				"Every property in that quote has a Stripe counterpart. The timestamp rides inside the signed message, so "+
					"a captured delivery cannot be re-dated — Stripe's t= value is covered by the MAC that accompanies "+
					"it, and the freshness window is the replay defense. The MAC covers the EXACT payload bytes, which is "+
					"why both receivers read the raw body before parsing: a receiver that parses first and re-serializes "+
					"verifies nothing, because JSON re-serialization is not byte-stable. And the comparison is constant-"+
					"time because naive equality returns at the first differing byte, leaking a signature one measured "+
					"response at a time. The hub's payments sandbox reuses the whole discipline for its charge.succeeded "+
					"event — signed at a fixed clock, then verified three ways on the wire: the honest event passes, the "+
					"amount-tampered copy fails the MAC, and the stale-but-validly-signed copy dies at the clock check "+
					"before any crypto runs. That third row is the one teams forget: a valid signature over an old "+
					"message is still a replay."),
			text("tp-st-idempotence-pitch",
				"The second mapping is the delivery contract. Webhook delivery is at-least-once — the same event WILL "+
					"arrive twice sometimes, and a payment event handled twice is a double fulfillment — so the handler "+
					"needs the seen-before check this repo's messaging layer already defines, while the vendor's API "+
					"takes the same idea inbound as an Idempotency-Key header on your own requests. The dedupe seam, "+
					"quoted from the messaging domain:"),
			code("tp-st-dedup-quote", "go",
				"The dedupe every webhook handler needs: one atomic check-and-mark, because a check-then-set races two deliveries.",
				"server/internal/domain/messaging/streams.go",
				"// Deduplicator is the idempotent-consumer seam: ProcessOnce answers\n"+
					"// whether id was seen for the FIRST time (true → do the work) or is a\n"+
					"// repeat (false → skip it). One atomic check-and-mark — SET NX under one\n"+
					"// implementation — because a check-then-set of two commands races two\n"+
					"// deliveries of the same message.\n"+
					"type Deduplicator interface {\n"+
					"\tProcessOnce(ctx context.Context, id string) (bool, error)\n"+
					"}"),
			text("tp-st-idempotence-reading",
				"The hub's sandbox implements the vendor's side of the same contract, and one corner most integrations "+
					"learn in production: a replay with the same key but a DIFFERENT payload is not a dedupe — it is a "+
					"caller bug the vendor refuses to paper over:"),
			code("tp-st-idem-quote", "go",
				"The vendor's side of idempotency: identical replays answer the original receipt; the bug-shaped replay is refused by name.",
				"server/internal/domain/demos/thirdparty_hub.go",
				"\tif prior, ok := p.charges[key]; ok {\n"+
					"\t\tif prior.AmountCents != req.AmountCents {\n"+
					"\t\t\treturn ThirdPartyChargeReceipt{}, thirdPartyVendorErrorf(\"idempotency_key_reused\",\n"+
					"\t\t\t\tfmt.Sprintf(\"key %q first charged %d cents; this replay wants %d — refused\",\n"+
					"\t\t\t\t\tkey, prior.AmountCents, req.AmountCents), false)\n"+
					"\t\t}\n"+
					"\t\treturn prior, nil\n"+
					"\t}"),
			code("tp-st-recipe", "bash", "", "",
				"# RECIPE — stripe-go is NOT pinned in this repo; the shape a real handler\n"+
					"# at the payments port takes (stripe-go, from the module proxy):\n"+
					"#   go get github.com/stripe/stripe-go/v82\n"+
					"stripe.Key = os.Getenv(\"STRIPE_SECRET_KEY\") // env, never the repo (§7)\n"+
					"\n"+
					"func stripeWebhook(w http.ResponseWriter, r *http.Request) {\n"+
					"\traw, _ := io.ReadAll(r.Body)            // RAW body first — the MAC covers bytes\n"+
					"\tevent, err := webhook.ConstructEvent(raw, r.Header.Get(\"Stripe-Signature\"),\n"+
					"\t\tos.Getenv(\"STRIPE_WEBHOOK_SECRET\"))  // t= + v1=, constant-time — §1's design\n"+
					"\tif err != nil {  // bad signature or stale timestamp\n"+
					"\t\tcore.WriteError(w, r, core.Unauthorized(\"stripe webhook: \"+err.Error()), logger)\n"+
					"\t\treturn\n"+
					"\t}\n"+
					"\tif seen, _ := dedupe.ProcessOnce(r.Context(), event.ID); seen {\n"+
					"\t\tw.WriteHeader(http.StatusOK)          // at-least-once ⇒ 2xx the duplicate\n"+
					"\t\treturn\n"+
					"\t}\n"+
					"\thandle(event)                          // only NOW does the money move\n"+
					"}"),
			text("tp-st-coda",
				"The order of operations in that handler is the entire contract: raw body, signature verification, dedupe, "+
					"and only then the side effect — swap any two neighbors and you have either a forged event or a "+
					"double fulfillment. The refusal on a bad signature goes out as the taxonomy's unauthorized kind, "+
					"and the keys point at the last section: a live secret key is the widest blast radius a SaaS "+
					"integration can carry — it can move real money — so test mode (sk_test) is the only mode that "+
					"should ever touch a development branch, and the hub's live skeleton naming STRIPE_SECRET_KEY is "+
					"the honest marker of where the real client would slot in."),
		),
		// ── 6. Cloud storage ─────────────────────────────────────
		section("cloud-storage", "Cloud storage behind the port",
			"Files that outlive the process: the bucket as the third classic vendor surface, one adapter away from the files topic's Store.",
			text("tp-cs-pitch",
				"Object storage is the third classic vendor surface: files that must outlive the request and the "+
					"process — uploads, exports, generated documents — stored in a bucket (S3, R2, GCS) instead of a "+
					"local disk. This repo runs one deliberately: the files topic's workbench keeps bytes on the local "+
					"disk through a Store port, and what makes that honest rather than thin is the PORT — the workbench "+
					"service never learns where bytes live, so a bucket adapter is a composition-root decision instead "+
					"of a rewrite. The files topic's port is the integrating-versus-building lesson in its purest Go "+
					"form — build the seam, buy the storage:"),
			code("tp-cs-port-quote", "go",
				"The seam: uploads arrive as bounded readers, downloads leave as seekers — the service above never learns disk from bucket.",
				"server/internal/domain/files/files.go",
				"type Store interface {\n"+
					"\t// SaveStream reads body (already an untrusted stream) under name,\n"+
					"\t// refusing with ErrTooLarge once more than limit bytes have been\n"+
					"\t// read. The adapter streams to a sibling temp file and renames, so\n"+
					"\t// a refused or crashed upload never leaves a partial object behind.\n"+
					"\tSaveStream(ctx context.Context, name string, body io.Reader, limit int64) (Object, error)\n"+
					"\t// Open returns the object's metadata plus a seeker over its bytes.\n"+
					"\t// ErrNotFound when absent; ErrInvalidName when the name refuses or\n"+
					"\t// escapes the root (including via symlink).\n"+
					"\tOpen(ctx context.Context, name string) (Object, io.ReadSeekCloser, error)\n"+
					"\t// Stat returns the metadata without opening the bytes.\n"+
					"\tStat(ctx context.Context, name string) (Object, error)\n"+
					"}"),
			text("tp-cs-port-reading",
				"Read the port as third-party hygiene. The storage name is never the user's raw filename — the layer "+
					"sanitizes and confines before any adapter sees it, so an attacker controls no part of any key in "+
					"any store. The limit rides the contract, so no adapter can be handed an unbounded stream; the "+
					"composition root wires the local adapter today, and the same three lines would wire a bucket:"),
			code("tp-cs-wiring-quote", "go",
				"The composition root today: real streaming routes over the local store — the port is where a bucket adapter would land.",
				"server/cmd/guide/main.go",
				"\t// Topic 26's files workbench: real streaming routes over the local\n"+
					"\t// store — a bounded multipart upload, a Content-Disposition download\n"+
					"\t// that honors ranges, a stdlib thumbnail transform, and the story\n"+
					"\t// payload that narrates the same machinery.\n"+
					"\tfilesConsole, err := filesdata.NewConsole()\n"+
					"\tif err != nil {\n"+
					"\t\treturn fmt.Errorf(\"files console: %w\", err)\n"+
					"\t}\n"+
					"\tmux.Handle(\"POST /api/demo/files/upload\",\n"+
					"\t\tpresentation.NewFilesUploadHandler(filesConsole.Store, filesConsole.UploadLimit, logger))"),
			text("tp-cs-sandbox-pitch",
				"The hub's object-store sandbox carries the bucket behaviors the port's adapters must survive, "+
					"and the key rules are the files topic's own allowlist posture — one component, no structure, no "+
					"traversal — now enforced at a vendor-shaped boundary:"),
			code("tp-cs-keys-quote", "go",
				"Bucket keys under the files topic's rules: dots inside a component are fine, traversal and separators never are.",
				"server/internal/domain/demos/thirdparty_hub.go",
				"func thirdPartyValidObjectKey(key string) bool {\n"+
					"\tname := strings.TrimSpace(key)\n"+
					"\tif name == \"\" || name == \".\" || name == \"..\" {\n"+
					"\t\treturn false\n"+
					"\t}\n"+
					"\tif strings.ContainsAny(name, `/\\`) || strings.ContainsRune(name, 0) || strings.Contains(name, \"..\") {\n"+
					"\t\treturn false\n"+
					"\t}\n"+
					"\treturn true\n"+
					"}"),
			code("tp-cs-recipe", "bash", "", "",
				"# RECIPE — aws-sdk-go-v2 is NOT pinned in this repo (the Store port and the hub's\n"+
					"# object-store port are the seams); the adapter shape a real bucket takes:\n"+
					"#   go get github.com/aws/aws-sdk-go-v2/config  (+ service/s3)\n"+
					"cfg, _ := config.LoadDefaultConfig(ctx)      // region + creds from the env\n"+
					"client := s3.NewFromConfig(cfg)              # … and bound via its retries/timeouts\n"+
					"_, err = client.PutObject(ctx, &s3.PutObjectInput{\n"+
					"\tBucket:      aws.String(\"guide-uploads\"),\n"+
					"\tKey:         aws.String(mintedKey),         # server-minted — never the upload's name\n"+
					"\tBody:        bytes.NewReader(buf),         # already bounded by the port's limit\n"+
					"\tContentType: aws.String(sniffedType),      # what the magic bytes said\n"+
					"})\n"+
					"# For large media, the production pattern is a PRESIGNED upload URL:\n"+
					"# the server mints a scoped, expiring credential and the browser PUTs\n"+
					"# directly to the bucket — the bytes never enter the API process at all\n"+
					"# (the same tokenization move the Stripe section made with cards)."),
			text("tp-cs-coda",
				"The cost model completes the decision. Object storage is cheap per gigabyte and metered per request "+
					"and per egress byte — the bill scales with traffic in a way a local disk does not — and the "+
					"quotas are real: request-rate limits whose burst behavior (throttling, slow-down errors) belongs "+
					"in the adapter's degradation story, exactly as the hub's store sandbox models with its put "+
					"budget. The dedupe row is the quiet money-saver: content-addressed etags mean identical bytes "+
					"are stored once and the byte budget charges nothing for repeats — real buckets dedupe the same "+
					"way at the block level. This repo integrated nothing and built the seam, because the teaching "+
					"value is the port and the bytes are throwaway; your application scores the same axes with its "+
					"own numbers and usually lands on the bucket — through the same port, changing the same wiring "+
					"lines."),
		),
		// ── 7. Keys, quotas, blast radius ────────────────────────
		section("keys-quotas-blast-radius", "Keys, quotas, and blast radius",
			"What every integration leaves behind: credentials to gate, limits to respect, and a blast radius to reason about — by name, never by value.",
			text("tp-kq-pitch",
				"Every integration you add arrives carrying keys and quotas: credentials that authenticate you to the "+
					"vendor, limits that shape how hard you may lean on them, and a blast radius — the set of things "+
					"a leaked key or an exhausted quota can damage. The rule that governs everything in this section: "+
					"name the variable, scope the value, reason about the blast — but never let the value cross a "+
					"wire you do not control, including this guide's own payloads. The hub practices the rule by "+
					"construction: its live panels carry env var NAMES (MAIL_RELAY_API_KEY, STRIPE_SECRET_KEY, "+
					"OBJECT_STORE_ACCESS_KEY) and its endpoint test plants sentinel values in the resolver to prove "+
					"they appear nowhere in the serialized payload — the builder resolves only PRESENCE, never values."),
			text("tp-kq-gate-pitch",
				"The gating discipline is small enough to quote: the adapter refuses before it pretends — an absent "+
					"credential is a typed unavailability with the env var named in the message, never a panic, and "+
					"never a fabricated success:"),
			code("tp-kq-gate-quote", "go",
				"The credential gate: presence by name, honest refusal by kind — the value is never even read.",
				"server/internal/domain/demos/thirdparty_hub.go",
				"func thirdPartyRequireCredentials(cfg ThirdPartyLiveConfig, credentials func(string) (string, bool)) error {\n"+
					"\tif credentials == nil {\n"+
					"\t\treturn thirdPartyVendorErrorf(\"credentials_absent\",\n"+
					"\t\t\t\"no credential resolver wired — nothing to call \"+cfg.VendorName+\" with\", false)\n"+
					"\t}\n"+
					"\tif _, ok := credentials(cfg.KeyEnvVar); !ok {\n"+
					"\t\treturn thirdPartyVendorErrorf(\"credentials_absent\",\n"+
					"\t\t\tcfg.KeyEnvVar+\" is not set in this environment — set it to wire the \"+cfg.VendorName+\" client\", false)\n"+
					"\t}\n"+
					"\treturn nil\n"+
					"}"),
			text("tp-kq-rail-pitch",
				"The second door is the deploy rail, and this repo's production compose states the pattern: an unset "+
					"secret is a RENDER-time failure — the stack refuses to come up, which is the cheapest possible "+
					"moment to have the conversation:"),
			code("tp-kq-rail-quote", "yaml",
				"The second door: compose's required-variable syntax makes an unset secret a deploy-time error, before any container starts.",
				"deploy/docker-compose.prod.yml",
				"      GDG_DATABASE_URL: postgres://guide:${POSTGRES_PASSWORD:?export POSTGRES_PASSWORD before compose up}@postgres:5432/guide?sslmode=disable\n"+
					"      # …\n"+
					"      POSTGRES_PASSWORD: ${POSTGRES_PASSWORD:?export POSTGRES_PASSWORD before compose up}"),
			text("tp-kq-handling",
				"Where the real values live is as deliberate as the doors demanding them: nowhere in the repository. "+
					"Typed config (topic 36) reads names like GDG_DATABASE_URL from the environment at boot with dev-"+
					"safe defaults and no secrets in source; the production compose refuses to render without the "+
					"real database password; and the same posture scales to vendor keys — one env var per purpose, "+
					"the narrowest scope the vendor allows (a test key in dev, a restricted key in prod, never one "+
					"key shared across purposes), and a rotation story you have REHEARSED, because a signing secret's "+
					"leak ends only at rotation. The discipline generalizes beyond secrets to every value that "+
					"identifies or authorizes: the hub never reads credential VALUES at all, which is the strongest "+
					"version of the rule — code that cannot read a value cannot leak it, log it, or serialize it."),
			text("tp-kq-quotas-pitch",
				"Quotas are the other half of the vendor contract, and this repo enforces its own with the same "+
					"deliberateness it demands of vendors — every ceiling is a named number someone chose, stated "+
					"with its two halves (the refill rate and the burst the client may spend at once):"),
			code("tp-kq-policy-quote", "go",
				"A quota as code: rate and burst answer different questions, and an honest limiter states both.",
				"server/internal/domain/security/ratelimit.go",
				"type RatePolicy struct {\n"+
					"\tRate  rate.Limit // sustained allowance, tokens per second (fractional allowed)\n"+
					"\tBurst int        // bucket size — the instant ceiling\n"+
					"}"),
			text("tp-kq-quotas-reading",
				"The hub's sandbox budgets are the same statement at vendor scale: three sends per window, two "+
					"processed charges (replays free — that is idempotency's dividend), three puts and sixty-four new "+
					"bytes per window, each with its refusal classified rate_limited and its retry verdict stated. A "+
					"quota without a documented behavior at the ceiling is a number, not a policy — the 429 with its "+
					"kind and retry verdict is the POLICY, and the number merely says when it fires. Vendor quotas "+
					"deserve the same respect inside your adapters: a payments processor's live-mode limits, a relay's "+
					"sends-per-second, a bucket's request rate are observable in the vendor's dashboard, not from your "+
					"repo — the honest adapter encodes them as configuration with a degradation story (queue, back "+
					"off, or shed), never as a surprise incident."),
			text("tp-kq-coda",
				"The section's discipline in one paragraph, which is also the topic's: before adding any dependency, "+
					"write its key's blast radius down in one sentence — what can a HOLDER of this value DO — then "+
					"give it the narrowest scope the vendor allows, keep it in the narrowest store that works (the "+
					"environment materialized at deploy, a secrets manager at scale — never the repo, never a log, "+
					"never a payload), and know the rotation story before you need it. The integrate-versus-build "+
					"decision opened this topic; the keys, quotas, and blast radii it leaves behind are how you live "+
					"with the decision every day after — and the hub's whole payload is the habit made executable: "+
					"names without values, counts without invention, refusals without fabrication."),
		),
		// ── 8. Terminal companion ────────────────────────────────
		section("terminal-companion", "Terminal companion: the vendor surfaces over curl",
			"Third-party from a shell: the hub payload, the wiring flip, the taxonomy map, the determinism proof, and this topic's shape.",
			text("tp-terminal-pitch",
				"The tour walks this topic's real surfaces on the running stack. The determinism claim is itself "+
					"checkable: the sandbox runs are pure functions of fixed scripts at a fixed exhibit clock, so two "+
					"GETs in the same scenario state serve byte-identical bodies — the diff below should print nothing "+
					"but the echo. The wiring flip is the demo's headline: PATCH the scenario's data mode, watch every "+
					"vendor's active adapter become the honest skeleton, and flip it back."),
			code("tp-terminal-tour-1", "bash", "", "",
				"# stop 1 — the hub: the wiring panel and the three vendor panels\n"+
					"curl -s localhost:8100/api/demo/thirdparty/hub | grep -o '\"current\":\"[a-z]*\"'\n"+
					"curl -s localhost:8100/api/demo/thirdparty/hub | grep -o '\"active\":\"[a-zA-Z]*\"' | sort | uniq -c\n"+
					"curl -s localhost:8100/api/demo/thirdparty/hub | grep -o '\"id\":\"[a-z]*\",\"title\":\"[^\"]*\"' | head -n 3\n"+
					"\n"+
					"# determinism, proven against the live endpoint — expect the echo, nothing else:\n"+
					"diff <(curl -s localhost:8100/api/demo/thirdparty/hub) \\\n"+
					"     <(curl -s localhost:8100/api/demo/thirdparty/hub) && echo deterministic"),
			code("tp-terminal-tour-2", "bash", "", "",
				"# stop 2 — the wiring flip, live: sandbox adapters become honest skeletons\n"+
					"curl -s -X PATCH localhost:8100/api/demo/scenario \\\n"+
					"  -H 'content-type: application/json' -d '{\"dataMode\":\"live\"}'\n"+
					"curl -s localhost:8100/api/demo/thirdparty/hub | grep -o '\"activeOutcome\":\"[^\"]*\"' | head -n 3\n"+
					"curl -s -X PATCH localhost:8100/api/demo/scenario \\\n"+
					"  -H 'content-type: application/json' -d '{\"dataMode\":\"fake\"}'\n"+
					"# flip it back and the probes accept again (the mailer greylists first — its own protocol)"),
			code("tp-terminal-tour-3", "bash", "", "",
				"# stop 3 — the observed behaviors: quota burn-down, idempotent replay, event verdicts\n"+
					"curl -s localhost:8100/api/demo/thirdparty/hub | grep -o '\"outcome\":\"[a-z-]*\"' | sort | uniq -c\n"+
					"curl -s localhost:8100/api/demo/thirdparty/hub | grep -A3 '\"vendorCode\":\"card_declined\"' | head -n 5\n"+
					"# the counts, counted during the run that served them:\n"+
					"curl -s localhost:8100/api/demo/thirdparty/hub | grep -o '\"label\":\"[^\"]*\",\"value\":[0-9]*' | head -n 6"),
			code("tp-terminal-tour-4", "bash", "", "",
				"# stop 4 — the taxonomy map and the honesty pins\n"+
					"curl -s localhost:8100/api/demo/thirdparty/hub | grep -o '\"vendorCode\":\"[a-z_]*\"' | sort | uniq -c\n"+
					"curl -s localhost:8100/api/demo/thirdparty/hub | grep -o '\"honestyNote\":\"[^\"]*' | head -c 220; echo\n"+
					"# credentials are named, never valued — plant a sentinel and hunt for it:\n"+
					"docker exec go_developers_guide_dev bash -c 'MAIL_RELAY_API_KEY=sk_sentinel_zzz curl -s \\\n"+
					"  localhost:8100/api/demo/thirdparty/hub' | grep -c sk_sentinel_zzz   # → 0, by construction"),
			code("tp-terminal-tour-5", "bash", "", "",
				"# stop 5 — this topic's shape, and the honest 404 as the guide closes it\n"+
					"curl -s localhost:8100/api/content | grep -o '\"id\":\"third-party\"[^}]*'\n"+
					"curl -s localhost:8100/api/content/third-party | grep -o '\"id\":\"[a-z-]*\"' | tail -n 9\n"+
					"curl -s localhost:8100/api/content/third-party | grep -c '\"kind\":\"demo\"'\n"+
					"\n"+
					"# through the client's dev proxy, as the browser reaches it:\n"+
					"curl -s localhost:5175/api/demo/thirdparty/hub | grep -o '\"current\":\"[a-z]*\"'\n"+
					"curl -s -i localhost:8100/api/content/third-party-nope | head -n 6"),
			text("tp-terminal-coda",
				"Stops 1–2 are the topic's argument in payloads: the decision (sandbox or skeleton) is one PATCH away, "+
					"and both halves answer honestly — receipts and refusals, never fabrication. Stop 3 reads the "+
					"behaviors the section taught, with every count earned during the serving run. Stop 4 is the keys "+
					"discipline made visible — a payload about vendor credentials that cannot leak one by "+
					"construction, proven with a planted sentinel. Stop 5 closes the family loop the way every topic "+
					"has: the shape through the content API, the demo through the client's proxy, and the unknown "+
					"topic answered by the one error envelope this guide has served since its first route."),
		),
	}

	return Topic{
		ID:    "third-party",
		Title: "Third-party services",
		Subtitle: "Integrating versus building — the decision this repo made for every piece it did not write, and the " +
			"vendor-shaped problems (email, payments, object storage) that force the same decision on every real " +
			"application · walked through a REAL adapter-surface hub: three vendor ports, each carrying a deterministic " +
			"sandbox (signing, idempotency keys, quota accounting, error-taxonomy mapping) and a credential-gated live " +
			"skeleton, switched fake/live by the shared scenario store exactly as the posts surface swaps its data mode · " +
			"email taught concept-first over the bounded-call discipline (net/smtp's frozen status stated honestly) · " +
			"the email-as-a-job seam from topic 29 recapped, not re-taught · Stripe as the archetype SaaS integration " +
			"mapped onto machinery this repo already owns (the HMAC webhook gate, the idempotent consumer, the " +
			"Idempotency-Key contract) · cloud storage one adapter away from the files topic's Store port · and the " +
			"keys, quotas, and blast radii every integration leaves behind — named, gated, and never leaked by value.",
		Group:    "capstones",
		Order:    42,
		Sections: sections,
	}, buildErr
}
