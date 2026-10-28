package content

import (
	"errors"
	"fmt"
)

// This file is topic 43 advanced-architecture (group capstones, curriculum
// order 43 — the FINAL topic): the architecture capstone. Topic 32 taught
// the layer map of one process; this topic teaches what happens when the
// boundary moves past the process — consistency across it, the vocabulary
// that keeps a growing domain speakable, why the monolith wins first, what
// actually travels between services, and the two patterns that make
// distributed state honest: the saga (a business transaction as an ordered
// list of compensable steps) and event sourcing (state as a derivation
// from an append-only ledger of facts), both carried by REAL machinery in
// internal/domain/patterns — written to be quoted here, served live by
// the topic's two demos. Tracing across the boundary closes the loop:
// once work spans processes, correlation is a design decision.

// advancedArchitectureTopic builds topic 43.
func advancedArchitectureTopic() (Topic, error) {
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
		// ── 1. The consistency map ───────────────────────────────
		section("consistency-map", "The consistency map",
			"What a transaction boundary actually buys you, what evaporates the moment a second service enters, and the honest spectrum between strong and eventual.",
			text("ax-cm-pitch",
				"Everything in this topic rests on one map. Inside a single database transaction, consistency is a property the DATABASE enforces for you: the topic 17 race exhibit showed the machinery directly — one connection's uncommitted update is invisible to another connection, its commit makes it visible at once, and a row lock blocks a second writer until the first lets go. That exhibit ran against real PostgreSQL with the isolation level read straight from the server, and its choreography is worth quoting because it is the baseline every distributed pattern in this topic is measured against:"),
			code("ax-cm-race-quote", "go",
				"Topic 17's baseline: what one database, one transaction boundary, buys you — visibility and exclusion, enforced by the engine.",
				"server/internal/data/db/sql_demos.go",
				"// RunRace choreographs the deterministic two-transaction dance: the\n"+
					"// writer's uncommitted update is invisible to the reader, its commit\n"+
					"// makes it visible, and the FOR UPDATE exhibit blocks the reader until\n"+
					"// the writer lets go — every handoff signaled by channels or confirmed\n"+
					"// by pg_stat_activity polling, never by sleeping and hoping.\n"+
					"func (r *SQLDemoRunner) RunRace(ctx context.Context) (demos.RaceObservation, error) {\n"+
					"\tctx, cancel := context.WithTimeout(ctx, 20*time.Second)\n"+
					"\tdefer cancel()\n"+
					"\n"+
					"\tobservation := demos.RaceObservation{}\n"+
					"\tif err := r.pool.QueryRow(ctx, \"SHOW transaction_isolation\").Scan(&observation.Isolation); err != nil {\n"+
					"\t\treturn demos.RaceObservation{}, fmt.Errorf(\"read the isolation level: %w\", err)\n"+
					"\t}"),
			text("ax-cm-boundary",
				"The moment a business transaction spans two services — reserve inventory in one, charge the card in another — that guarantee is gone, and no engineering buys it back. The reason is physics wearing a protocol costume: the two services hold their state in different memory, reached over a network that can fail between any two messages, and there is no lock that spans both. A commit in service A followed by a commit in service B is TWO transactions; the network can eat the second call, the second service can refuse it, or the first process can die between them. Distributed-systems literature compresses this into CAP: under a network partition — which is not an exotic event but Tuesday — a system must choose between consistency and availability, and the honest engineering answer is to make that choice PER OPERATION, with the consequence written down. What replaces the all-or-nothing transaction is a spectrum of weaker promises, and the craft is knowing which one each interaction actually needs:"),
			list("ax-cm-spectrum",
				false,
				"Linearizable / serializable — every operation appears to happen in one total order, as if a single machine processed them: what a single PostgreSQL transaction gives its participants, and what spanning services can only approximate at enormous coordination cost.",
				"Read committed — no transaction sees another's uncommitted work, but reads are not repeatable: PostgreSQL's default, the baseline the topic 17 exhibit reads off the server.",
				"Read-your-writes — a user who just wrote a value sees their own write back on the next read, though other users may lag: the promise most user-facing caches actually need (the caching topic's invalidation discipline exists to keep it).",
				"Monotonic reads — once a reader has seen a version, they never see an older one: the property sticky sessions and snapshot tokens protect when replicas each hold a slightly stale copy.",
				"Eventual consistency — replicas converge given time and no new writes, with no bound on when: the cheapest promise, only safe where the business tolerates a stale window (topic 21 drew that line per cache).",
				"Compensated consistency — the multi-service business transaction either completes everywhere or is actively UNDONE to a business-equivalent state: the saga's promise, weaker than atomicity, strong enough for orders and payments, taught with real machinery two sections down.",
			),
			text("ax-cm-reading",
				"Read the spectrum as a budget, not a ranking. Every step to the right buys availability and latency and spends guarantee, and the failure mode of this whole subject is paying for strong consistency where the business never needed it, or — worse — ASSUMING strong consistency in code that runs on an eventually-consistent substrate. The tests of the choice are concrete: can a customer see a checkout total that predates their own coupon? Can an inventory count go negative if two orders race? Can a refund land after its charge was already reversed? When the answer to any of these is 'that must never happen', you have found the operation that genuinely needs a strong tool — a transaction, a lock, an idempotency key, or a saga — and everything else can live cheaply further down the spectrum. This topic's two patterns are the strong tools for the one case the spectrum cannot otherwise cover: a business transaction that spans boundaries."),
		),
		// ── 2. DDD vocabulary ────────────────────────────────────
		section("ddd-vocabulary", "DDD vocabulary",
			"The words that keep a growing domain speakable — and this repo's real shapes for each one: ports, entities, and the patterns package's aggregates-in-motion.",
			text("ax-ddd-pitch",
				"Domain-driven design is mostly a vocabulary discipline wearing a methodology's clothes, and the vocabulary earns its keep the day a second engineer joins. Its core observation: software rots first at the SEAMS between what the business means and what the code says, so the words in the code must be the words the business uses — the 'ubiquitous language' — and the boundaries in the code must be boundaries in the business — the 'bounded contexts'. This guide has been doing DDD quietly since topic 4 without naming it: `internal/domain/` IS the bounded context of the guide's own domain, its packages (posts, files, events, patterns) are sub-domains, and the ports — interfaces the domain owns, adapters the data layer implements — are DDD repositories in Go's native shape. Topic 5 taught the interface half (accept interfaces, return structs); here is the whole vocabulary, each term with the place it already lives in this repo:"),
			list("ax-ddd-terms",
				false,
				"Entity — an object with identity that outlives its field values: a Post is the same post whether its title changes; identity is the id, equality is the id — this repo's Post struct.",
				"Value object — an object without identity, equal when its fields are equal: a ListQuery, a RatePolicy, an EventAppend — safe to copy, never mutated in place.",
				"Aggregate — a cluster of entities and values that commit together under one root, the consistency boundary: an order and its line items behind the Order root; a stream and its events behind the stream id in this topic's event store.",
				"Domain event — an immutable statement that something business-meaningful HAPPENED, named in past tense: `inventory.reserved`, `order.placed` — the atom of this topic's second half.",
				"Repository — the port that speaks the domain's vocabulary while an adapter speaks storage's: the interface the domain owns (topic 32's dependency rule, arrow reversed).",
				"Bounded context — a boundary inside which a term means exactly one thing: 'post' in the content context and 'post' in the HTTP middleware log are different words that happen to share spelling.",
				"Anti-corruption layer — a translation boundary that keeps a FOREIGN model (a vendor's, a legacy system's) from leaking into yours: the third-party topic's adapters are exactly this, converting vendor-shaped errors into the taxonomy's eight kinds.",
				"Ubiquitous language — the discipline of one vocabulary across conversation, code, and wire: when the demo's ledger says `already-compensated`, that is not a UI label, it is the machinery's own constant.",
			),
			text("ax-ddd-port-pitch",
				"The repository term deserves its real shape, because it is where DDD and Go's composition culture agree most cleanly. A DDD repository is not a query builder and not a table gateway — it is a COLLECTION of aggregates that happens to persist, speaking the domain's nouns and verbs, returning typed domain errors instead of storage errors. The guide's posts port is the canonical local example, and every clause of its doc comment is a DDD claim:"),
			code("ax-ddd-port-quote", "go",
				"A repository as Go writes it: the domain owns the interface, the adapter owns the dialect — typed errors so the mapping happens once.",
				"server/internal/domain/posts/repository.go",
				"// PostsRepository is the storage port for the posts resource. All methods\n"+
					"// take ctx first (anything that can block takes context) and return typed\n"+
					"// core errors: core.NotFound for an unknown id, so one mapping at the\n"+
					"// handler produces the 404 envelope for every adapter.\n"+
					"type PostsRepository interface {\n"+
					"\t// List returns one page of posts matching the query, sorted and\n"+
					"\t// filtered by the repository, with the total match count for the page\n"+
					"\t// metadata.\n"+
					"\tList(ctx context.Context, query ListQuery) (PostPage, error)\n"+
					"\t// GetByID returns one post or core.NotFound.\n"+
					"\tGetByID(ctx context.Context, id string) (Post, error)\n"+
					"\t// Create stores a draft and returns the stored post — the adapter\n"+
					"\t// assigns the ID and stamps CreatedAt/UpdatedAt.\n"+
					"\tCreate(ctx context.Context, draft Draft) (Post, error)\n"+
					"// …\n"+
					"}"),
			text("ax-ddd-coda",
				"What DDD is NOT, stated once because the failure is common: it is not a folder structure, not a requirement to build entities for everything, and not a license for a Rich Domain Model bureaucracy in a language whose structs favor plain data. The test of the vocabulary is operational — when a new engineer can read `SagaStepCompensationFailed` and know both what the machinery did and what the business means, the language is doing its job; when a 'manager service' and an 'orchestrator' and a 'coordinator' all exist and nobody can say which one owns the refund, it is not. This topic's patterns package was written under that discipline on purpose: every identifier a reader meets there — step, forward, compensate, ledger, outcome — is a term the demo panels use unchanged, so the vocabulary a reader learns here is literally the one the wire speaks. The same discipline names the bounded context this topic guards: `patterns`, the guide's distributed-transactions context, dependency-free by design so nothing leaks in from either direction."),
		),
		// ── 3. Monolith first ────────────────────────────────────
		section("monolith-first", "Monolith first",
			"The strongest architectural position this guide can take, having been built as one: one process, hard module boundaries, extraction as a LATER decision with evidence.",
			text("ax-mf-pitch",
				"This whole guide — 43 topics, 62 demos, a content engine, a GraphQL surface, a gRPC listener, workers, websockets — is ONE Go binary, and that is an architectural position, not an accident. The argument is Martin Fowler's old one and it has aged well: a monolith with hard internal boundaries gives you the decoupling benefits people chase into microservices while keeping the debugging story of a single process — one stack trace from the HTTP handler to the SQL, one `go test ./...`, one deployment, zero serialization at the seams. Go sharpens the argument further: the toolchain is built for it (internal/ visibility, one static binary, fast cross-compilation), and the language's package system gives you compile-time enforcement of module boundaries that in other ecosystems only a separate REPO can enforce. The failure mode the guide's structure pre-empts is the distributed monolith — services split apart while still sharing a database and a deploy cadence, paying both models' costs and collecting neither's benefits."),
			text("ax-mf-boundaries-pitch",
				"What makes 'monolith' a strategy rather than an excuse is that the internal boundaries are REAL. The dependency rule (topic 32: presentation → domain ← data) is enforced by the compiler — handlers physically cannot import the data layer, because the domain's ports sit between them. The composition root is the one place wiring happens, so the seams a future extraction would cut are already cut and already visible in one file. The posts surface is the canonical exhibit — one repository behind five use cases behind five thin handlers, with the repository itself a switching facade over a fake and a live adapter:"),
			code("ax-mf-wiring-quote", "go",
				"The monolith's tell: every seam a future extraction would cut is already cut, in one visible place.",
				"server/cmd/guide/main.go",
				"\tpostsRepo := postsdata.NewSwitchingRepository(\n"+
					"\t\tscenarioStore,\n"+
					"\t\tpostsdata.NewInMemoryRepository(),\n"+
					"\t\tpostsdata.NewPostgresRepository(pool),\n"+
					"\t)\n"+
					"\tpostsList := posts.NewListPostsUseCase(postsRepo)\n"+
					"\tpostsCreate := posts.NewCreatePostUseCase(postsRepo)\n"+
					"\tpostsGet := posts.NewGetPostUseCase(postsRepo)\n"+
					"\tpostsReplace := posts.NewReplacePostUseCase(postsRepo)\n"+
					"\tpostsDelete := posts.NewDeletePostUseCase(postsRepo)"),
			text("ax-mf-wiring-reading",
				"Read that wiring as an extraction plan wearing a dependency-injection costume. If the posts domain ever left the process, the line `postsdata.NewPostgresRepository(pool)` becomes a network client behind the SAME interface, the five use-case constructors do not change at all, and the handlers do not change at all — the seams already hold everything that would move. That is the actual test of a modular monolith: not that you never extract, but that extraction is a DEPLOYMENT change, not a redesign. The guide even rehearses the shape at smaller scale: the optional stores (mongo, redis) sit behind ports that degrade to typed 503s when absent, and the scenario store flips adapters at runtime — a service boundary you can toggle with a PATCH request, which is what 'boundary' should mean before it costs you a network hop."),
			text("ax-mf-when",
				"When IS extraction right? The honest list is short and none of its items are 'the team got big'. A service boundary earns its network hop when: a piece needs a different SCALING axis (a CPU-bound image pipeline should not share a process with a latency-bound API); a piece needs a different FAILURE domain (a job worker that wedges must not take request serving with it — the jobs topic's worker lives in-process but behind its own lifecycle for exactly this rehearsal); a piece needs a different RELEASE cadence (data-hungry ML scoring changing weekly beside a frozen checkout); or an ORGANIZATIONAL boundary demands it (two teams, one contract). Notice every item is about what the boundary BUYS — and the moment you extract, everything in the rest of this topic becomes your bill: consistency across the new boundary (§1), the outbox and idempotency (§4), sagas (§5), event-carried state (§7), tracing (§8). Build the monolith whose seams make that bill predictable, and pay it only when a line item justifies it."),
		),
		// ── 4. Between services ──────────────────────────────────
		section("between-services", "Between services",
			"What actually crosses a process boundary — bounded calls, the dual-write problem, the transactional outbox, and the idempotent consumer that makes at-least-once survivable.",
			text("ax-bs-pitch",
				"Extract one service and your method calls become messages, with everything that implies: the callee may be slow (so every call needs the deadline discipline of topic 39 — a context with a budget, derived from the caller's own), absent (so every failure needs the typed taxonomy of topic 30 — errors as values, classified, never swallowed), or DUPLICATE (so every effect needs an idempotency story). The last one is the sleeper. Network retry semantics are at-least-once as soon as anything between the parties retries — your client library, your load balancer, your own backoff loop — because a timeout does not mean the request failed; it means you do not KNOW. The guide's messaging topic (22) built this whole story on real Redis streams; this section recaps the two pieces the architecture capstone stands on, then spends the rest of the topic on what they cannot cover."),
			text("ax-bs-dualwrite-pitch",
				"The first piece is the dual-write problem, and it is worth seeing precisely because it looks trivial. 'Commit the order row, then publish the order-placed event' is two writes to two systems. If the process dies between them, the row is committed and the event never lands: every downstream consumer is now silently wrong. If you publish first and commit second, the mirror failure fires. No retry ladder fixes this — the window is between the writes, not inside them. The fix is the transactional outbox (topic 22's msg:outbox demo): write the business row AND the event row in ONE database transaction, then a separate relay publishes committed outbox rows and marks them. The write side of that machinery, quoted from the messaging domain:"),
			code("ax-bs-outbox-quote", "go",
				"The dual-write fix: the event commits with the state it describes, or neither does — the relay becomes the only thing that can duplicate, and the consumer handles that.",
				"server/internal/domain/messaging/outbox.go",
				"type EventRecord struct {\n"+
					"\tID      string\n"+
					"\tRunID   string\n"+
					"\tKind    string\n"+
					"\tPayload string\n"+
					"}\n"+
					"// …\n"+
					"// OutboxRecorder is the write side of the outbox: record the business\n"+
					"// event AND its outbox row in one PostgreSQL transaction. If either\n"+
					"// write fails, BOTH roll back — there is no committed state change whose\n"+
					"// event never became publishable, and no event for a change that never\n"+
					"// happened.\n"+
					"type OutboxRecorder interface {\n"+
					"\t// RecordEventWithOutbox commits event and message together, or\n"+
					"\t// neither.\n"+
					"\tRecordEventWithOutbox(ctx context.Context, event EventRecord, message OutboxMessage) error\n"+
					"}"),
			text("ax-bs-idempotent-pitch",
				"The second piece follows from the first like a shadow. The relay can crash after publishing but before marking, so on restart it publishes again — the pipeline is at-least-once BY DESIGN, and the contract moves to the consumer: dedupe on a stable message id before taking any side effect. The id must be minted by the WRITER, not the store, which is exactly the kind of sentence that sounds like folklore until you read the reason sitting in the type's own comment:"),
			code("ax-bs-message-quote", "go",
				"Why the id is app-minted: dedupe is only possible if every redelivery of one logical message carries the SAME id — a store-assigned id would make every duplicate look unique.",
				"server/internal/domain/messaging/outbox.go",
				"// OutboxMessage is one outbox row as the writer hands it over: WHAT to\n"+
					"// publish (payload) and WHERE (stream). The ID is app-minted (a UUID)\n"+
					"// rather than store-assigned: consumers dedup on it, so it must be\n"+
					"// identical across every redelivery of the same logical message — a\n"+
					"// store-assigned per-append id would make every duplicate look unique\n"+
					"// and defeat dedup. The created-at/published-at stamps stay inside the\n"+
					"// adapter; the write side never reads them.\n"+
					"type OutboxMessage struct {\n"+
					"\tID      string\n"+
					"\tRunID   string\n"+
					"\tStream  string\n"+
					"\tPayload string\n"+
					"}"),
			text("ax-bs-coda",
				"What the outbox and the idempotent consumer together still do NOT give you is a business transaction with a BEGIN and an END. They make single effects durable and safely repeatable; they cannot express 'reserve inventory, charge the card, book shipment, and if the shipment refuses, give the money and the inventory back'. That is not a message problem — it is a distributed TRANSACTION problem, and it needs a shape whose end is either completion or active undo. That shape is the saga, and it is the next section — carried by real machinery this repo built for it, not by a diagram."),
		),
		// ── 5. Sagas ─────────────────────────────────────────────
		section("sagas", "Sagas: the distributed transaction as compensation",
			"This section carries the topic's first demo — the real compensation machinery, driven live through three observed runs.",
			text("ax-sg-pitch",
				"A saga is a business transaction written as an ordered list of steps, each carrying its own antidote: a forward action that advances the transaction and a compensating action that undoes exactly that step. There is no distributed lock and no two-phase commit — the coordination is explicit, in your code, and the consistency guarantee is the compensated kind from the consistency map: either every step completes, or every COMPLETED step is actively undone, in reverse order, leaving the world in a business-equivalent state (inventory released, card refunded, pickup cancelled — not 'as if nothing happened', but as if the ORDER never happened). The trade is honest: a saga gives up isolation entirely (intermediate states are visible; other transactions can observe a reserved-then-released inventory) in exchange for availability and an end to every step. For money-and-inventory flows, that trade is the industry answer — it is how airline bookings and hotel-and-flight packages have worked since the pattern was named at 1987 database conferences for long-running transactions that could not hold locks for hours."),
			text("ax-sg-step-pitch",
				"The machinery lives in this repo — `internal/domain/patterns/saga.go`, written to be quoted, tested with the same table-driven discipline as everything else (topic 34). The design starts from one signature, shared by both halves of every step:"),
			code("ax-sg-step-quote", "go",
				"The whole pattern in two types: a function shape both halves share, and one ordered unit of work carrying its own antidote.",
				"server/internal/domain/patterns/saga.go",
				"// SagaAction is the one shape every step's two halves share: a function\n"+
					"// that does bounded work under a context and answers an error. Forward\n"+
					"// actions advance the business transaction; compensating actions undo\n"+
					"// exactly their own step's effect — nothing more.\n"+
					"type SagaAction func(ctx context.Context) error\n"+
					"\n"+
					"// SagaStep is one ordered unit of saga work: a human-readable Name for\n"+
					"// the ledger, the Forward action that advances the transaction, and the\n"+
					"// Compensate action that undoes the forward's effect. A step whose work\n"+
					"// cannot be undone (an email sent, a webhook delivered — a fact the\n"+
					"// outside world already holds) declares no Compensate action; such\n"+
					"// steps belong LAST, after everything that can still be rolled back.\n"+
					"//\n"+
					"// A nil Forward action is a programmer bug and panics with the step's\n"+
					"// name the moment the runner reaches it; a nil Compensate action is a\n"+
					"// legitimate \"nothing to undo\", recorded as such in the ledger.\n"+
					"type SagaStep struct {\n"+
					"\tName       string\n"+
					"\tForward    SagaAction\n"+
					"\tCompensate SagaAction\n"+
					"}"),
			text("ax-sg-step-reading",
				"Every clause of that comment is a design decision worth naming aloud. Compensation is PER-STEP and minimal — a compensate action undoes its own step and nothing else, because composites drift. Steps that cannot be undone go LAST — an email is a fact the outside world already holds, so nothing may be allowed to fail after one is sent; ordering the script so externalized facts trail every compensable effect is a saga-design rule, not a nicety. And the two nils are treated differently on purpose: a nil Forward is a programmer bug (a step that does no work is not a step) and panics with the step's name, while a nil Compensate is legitimate domain vocabulary — recorded in the ledger as nothing-to-undo, visible as data. That distinction is the guide's whole panic policy (topic 30) in one type: panics for programmer bugs, recorded values for domain facts."),
			text("ax-sg-run-pitch",
				"The runner is a state machine, not a loop: Run drives the saga once, forward through the steps until one fails or the context dies, then — for either stop — an undo pass walks the executed prefix in reverse. The forward half, quoted from the real Run:"),
			code("ax-sg-run-quote", "go",
				"The forward pass: in order until a stop, then every remaining step is recorded as skipped — the ledger shows what never ran.",
				"server/internal/domain/patterns/saga.go",
				"\tfor i, step := range r.steps {\n"+
					"\t\tif stopReason != stopNone {\n"+
					"\t\t\t// Past the stop: every remaining step is skipped, one row\n"+
					"\t\t\t// each — the ledger shows what never ran.\n"+
					"\t\t\tr.recordLocked(i, step.Name, SagaPhaseForward, SagaStepSkipped, \"\")\n"+
					"\t\t\tcontinue\n"+
					"\t\t}\n"+
					"\t\tif err := ctx.Err(); err != nil {\n"+
					"\t\t\tstopReason = stopCancelled\n"+
					"\t\t\ttrigger = &SagaStepError{Index: i, StepName: step.Name, Err: err}\n"+
					"\t\t\tr.recordLocked(i, step.Name, SagaPhaseForward, SagaStepSkipped, \"\")\n"+
					"\t\t\tcontinue\n"+
					"\t\t}\n"+
					"// …\n"+
					"\t\tif err := step.Forward(ctx); err != nil {\n"+
					"\t\t\tstopReason = stopFailed\n"+
					"\t\t\ttrigger = &SagaStepError{Index: i, StepName: step.Name, Err: err}\n"+
					"\t\t\tr.recordLocked(i, step.Name, SagaPhaseForward, SagaStepFailed, err.Error())\n"+
					"\t\t\tcontinue\n"+
					"\t\t}\n"+
					"\t\tr.executed[i] = true\n"+
					"\t\tr.recordLocked(i, step.Name, SagaPhaseForward, SagaStepExecuted, \"\")\n"+
					"\t}"),
			text("ax-sg-undo-pitch",
				"The undo pass is where the two decisions live that separate a real saga runner from a textbook's. Compensation runs on `context.WithoutCancel` — the cancellation that stopped the forward work must not stop the CLEANUP it triggered, while the context's values (request ids, trace ids — topic 33's propagation) still flow to the compensating actions. And the walk visits only the executed prefix, in reverse, because the step that failed never completed and so has nothing to undo. One locking decision rides along: the mutex guards the LEDGER, never the user-supplied callbacks — each step is claimed under the lock, the compensating action runs with the lock released (an undo may take as long as its work takes, or re-enter the runner's public doors), and the outcome is recorded back under it:"),
			code("ax-sg-undo-quote", "go",
				"The undo pass: reverse over the executed prefix, on a cancellation-stripped context — claims and short lock holds mean a user callback never blocks readers, and a step with no antidote is recorded, never skipped silently.",
				"server/internal/domain/patterns/saga.go",
				"func (r *SagaRunner) undoPass(ctx context.Context) []SagaStepResult {\n"+
					"\tundoCtx := context.WithoutCancel(ctx)\n"+
					"\trecorded := []SagaStepResult{}\n"+
					"\tfor i := len(r.steps) - 1; i >= 0; i-- {\n"+
					"\t\tstep := r.steps[i]\n"+
					"\t\tr.mu.Lock()\n"+
					"\t\tif !r.executed[i] {\n"+
					"\t\t\tr.mu.Unlock()\n"+
					"\t\t\tcontinue // never executed (failed, skipped, or never reached): nothing to undo\n"+
					"\t\t}\n"+
					"// …\n"+
					"\t\tif step.Compensate == nil {\n"+
					"\t\t\tr.compensated[i] = true\n"+
					"\t\t\trow := r.recordLocked(i, step.Name, SagaPhaseCompensate, SagaStepNothingToUndo,\n"+
					"\t\t\t\t\"the step declared no compensating action — an externalized fact, placed last on purpose\")\n"+
					"\t\t\tr.mu.Unlock()\n"+
					"\t\t\trecorded = append(recorded, row)\n"+
					"\t\t\tcontinue\n"+
					"\t\t}\n"+
					"// …\n"+
					"\t\terr := step.Compensate(undoCtx)\n"+
					"\n"+
					"\t\tr.mu.Lock()\n"+
					"\t\tr.undoing[i] = false\n"+
					"// …\n"+
					"\t\tr.compensated[i] = true\n"+
					"\t\trow := r.recordLocked(i, step.Name, SagaPhaseCompensate, SagaStepCompensated, \"\")\n"+
					"\t\tr.mu.Unlock()\n"+
					"\t\trecorded = append(recorded, row)\n"+
					"\t}\n"+
					"\treturn recorded\n"+
					"}"),
			text("ax-sg-idempotent-pitch",
				"Because retries are the normal condition of distributed work, the runner's second guarantee is idempotent compensation: a compensating action is called AT MOST ONCE per execution, and a re-compensation attempt — an operator replaying the recovery door after a crash, say — is recorded as an already-compensated no-op, never a second call. The recovery door is a public method into the same ledger, with the whole contract in its comment:"),
			code("ax-sg-compensate-quote", "go",
				"Idempotency as data: replaying the recovery door records no-ops and calls nothing — the runner undoes each step at most once.",
				"server/internal/domain/patterns/saga.go",
				"// Compensate is the recovery door into the same ledger: it compensates\n"+
					"// every step that executed and has not been undone yet, in reverse\n"+
					"// order, answering the rows THIS pass recorded. After a failed run's\n"+
					"// automatic undo pass every executed step is already compensated, so a\n"+
					"// retried Compensate observes one already-compensated no-op per step\n"+
					"// and calls nothing — the idempotency guarantee, visible as data. Like\n"+
					"// the automatic pass it runs on context.WithoutCancel of ctx, so an\n"+
					"// operator may safely compensate with the very context that died.\n"+
					"// Compensating a saga whose Run never happened records nothing.\n"+
					"func (r *SagaRunner) Compensate(ctx context.Context) []SagaStepResult {\n"+
					"\treturn r.undoPass(ctx)\n"+
					"}"),
			text("ax-sg-error-pitch",
				"And when a run stops, WHAT stopped it travels as a typed error, not a string: the trigger wraps the underlying failure with the failing step's identity, so a caller can branch on errors.Is for its own sentinels (and for context.Canceled / DeadlineExceeded, topic 6's cancellation vocabulary) and errors.As for the step's index and name — the same errors-as-values discipline topic 30 built the envelope around, now spanning a whole transaction:"),
			code("ax-sg-error-quote", "go",
				"The typed trigger: which step stopped the saga, wrapping why — inspectable with the standard errors tools.",
				"server/internal/domain/patterns/saga.go",
				"// SagaStepError is the typed trigger error a failed or cancelled run\n"+
					"// carries: the identity of the step where progress stopped, wrapping\n"+
					"// the underlying error. For a failed step it wraps the forward action's\n"+
					"// error; for a cancelled run it wraps the context's error at the step\n"+
					"// where cancellation was observed. Callers use errors.As to learn which\n"+
					"// step stopped the saga and errors.Is to test for their own sentinel\n"+
					"// errors (and for context.Canceled / context.DeadlineExceeded).\n"+
					"type SagaStepError struct {\n"+
					"\tIndex    int\n"+
					"\tStepName string\n"+
					"\tErr      error\n"+
					"}"),
			text("ax-sg-demo-pitch",
				"The demo drives this exact machinery through the fixed order-fulfillment script every panel shows: reserve inventory (undo: release), charge the card (undo: refund), book shipment (undo: cancel pickup), send the confirmation (no undo — an externalized fact, deliberately last). Three runs are observed per request: the success run, a failure run whose shipment step refuses deterministically by index, and a cancellation run whose payer deadline fires INSIDE the charge step — after the charge lands, so the undo pass has real work to do on a dead context, proving the WithoutCancel rule live. The failure run then replays Compensate once more so the payload carries the machinery's own idempotency record. The injection is three lines of closure code over the fixed script — deterministic by construction, no sleeps, no clock:"),
			code("ax-sg-script-quote", "go",
				"How failure and cancellation enter the exhibit: by step index, inside the compiled closures — never by timing.",
				"server/internal/domain/demos/arch_saga.go",
				"\tfor index, scripted := range archSagaScript {\n"+
					"\t\tforward := func(ctx context.Context) error {\n"+
					"\t\t\tif index == failAt {\n"+
					"\t\t\t\treturn errArchSagaCarrierRefused\n"+
					"\t\t\t}\n"+
					"\t\t\tworld.log = append(world.log, scripted.does)\n"+
					"\t\t\tif index == cancelAt {\n"+
					"\t\t\t\tcancel() // the payer service's deadline, injected mid-step\n"+
					"\t\t\t}\n"+
					"\t\t\treturn nil\n"+
					"\t\t}\n"+
					"// …\n"+
					"\t}"),
			demo("arch:saga", "The saga compensation ledger",
				"GET /api/demo/arch/saga serves the REAL patterns.SagaRunner from internal/domain/patterns driven through three observed runs of the fixed "+
					"order-fulfillment script · the SUCCESS panel — four executed rows, zero compensate rows, the confirmation the world's last effect · the FAILURE panel — "+
					"book-shipment refuses by deterministic index, the refund lands before the release (reverse order over the executed prefix), step 3 recorded skipped, and the "+
					"REPLAY pass driving Compensate a second time: two already-compensated no-ops, the machinery's idempotency guarantee visible as data · the CANCELLATION panel — "+
					"the payer's deadline fires inside step 1, and the refund and release still succeed because the undo pass runs on context.WithoutCancel (values kept, cancellation "+
					"stripped) while the compensating actions check their ctx on purpose · every ledger row, world line, and count was recorded by the run that served it — no clock, no "+
					"randomness, so two GETs serve byte-identical bodies"),
			text("ax-sg-demo-reading",
				"How to read the panels. In the failure run's ledger, read the ORDER of the compensate rows against the world log: the refund (step 1's undo) is recorded before the release (step 0's undo) — reverse over the executed prefix — while the failed shipment row carries the carrier's refusal text and the confirmation row says skipped, never reached. In the cancellation run, the charge's undo is the exhibit's thesis: the compensating actions in the script deliberately check their context and fail on a dead one, and the refunds still land — because the machinery handed them one with the cancellation stripped. Then compare the replay's rows to the undo pass they repeat: same steps, already-compensated, no world lines between them, because nothing ran. The endpoint itself is the layering lesson at its thinnest — the handler is three lines and owns nothing:"),
			code("ax-sg-handler-quote", "go",
				"The whole endpoint: build from fresh runs of the real machinery, write. No policy in the presentation layer.",
				"server/internal/presentation/demo_arch_saga.go",
				"func (h *ArchSagaHandler) ServeHTTP(w http.ResponseWriter, _ *http.Request) {\n"+
					"\twriteJSON(w, http.StatusOK, demos.BuildArchSagaPayload())\n"+
					"}"),
			text("ax-sg-wiring-pitch",
				"The route is one line at the composition root beside its sibling — and the client side is the same mirror every topic has built: a lazy demo-registry entry binding the demo block's id to the view, and the health-pipeline shape (API → repository → use case) registered under the demo's own token in the client's container. Both halves quoted:"),
			code("ax-sg-route-quote", "go",
				"One route at the root: the saga's endpoint registered beside its event-store sibling.",
				"server/cmd/guide/main.go",
				"\tmux.Handle(\"GET \"+demos.ArchSagaPath, presentation.NewArchSagaHandler(logger))"),
			code("ax-sg-client-quote", "ts",
				"The client mirror, part one: the demo-registry's lazy entry — the demo block id starts resolving through DemoHost.",
				"client/src/ui/demos/demo-registry.tsx",
				"const ArchSagaDemo = lazy(() => import('./arch/saga_demo'));\n"+
					"const ArchEventstoreDemo = lazy(() => import('./arch/eventstore_demo'));\n"+
					"// …\n"+
					"  'arch:saga': ArchSagaDemo,\n"+
					"  'arch:eventstore': ArchEventstoreDemo,\n"+
					"};"),
			code("ax-sg-container-quote", "ts",
				"The client mirror, part two: the saga pipeline in the container — API, repository impl over the port, use case under its token.",
				"client/src/core/container.ts",
				"    this.register(ContainerTokens.ArchSagaApi, new ArchSagaApi(config));\n"+
					"    this.register(\n"+
					"      ContainerTokens.ArchSagaRepository,\n"+
					"      new ArchSagaRepositoryImpl(this.archSagaApi),\n"+
					"    );\n"+
					"    this.register(\n"+
					"      ContainerTokens.GetArchSagaDemoUseCase,\n"+
					"      new GetArchSagaDemoUseCase(this.archSagaRepository),\n"+
					"    );"),
			text("ax-sg-coda",
				"Two production notes the demo's honesty forces. First, orchestration vs choreography: this runner is an ORCHESTRATOR — one object owns the order and drives the steps — while the choreographed alternative lets each service react to the previous one's events, with the order living implicitly in who listens to whom. Orchestration is easier to observe (the ledger exists), choreography is easier to extend (new listeners attach without the orchestrator's knowledge); the trade is exactly visibility versus coupling, and the observability section will have more to say about why this repo picked the ledger. Second, compensation is not free even when it succeeds: a refunded charge is not an uncharged card — fees, customer emails, audit trails all survive — which is why step ORDER is a business decision. The saga makes undo possible; keeping the script short enough that undo is rare is still on you."),
		),
		// ── 6. CQRS ──────────────────────────────────────────────
		section("cqrs", "CQRS: two models, one truth",
			"Command Query Responsibility Segregation without the ceremony — the reads and writes your API already separates, and the event-sourced version the next section builds.",
			text("ax-cq-pitch",
				"CQRS is the least understood three-letter pattern because it is usually taught backwards, starting from its most extreme form. The actual idea is modest and almost boring: the model you use to VALIDATE AND COMMIT a change does not have to be the model you use to ANSWER a query. A write wants strict invariants and a narrow shape; a read wants denormalized convenience — everything the screen needs in one fetch. Forcing one model to serve both means each side pays for the other's needs: writes navigate read conveniences, reads reconstruct aggregates the UI immediately flattens again. Greg Young's formulation, which the industry shorthand buries: it is a pattern about SEPARATION, not about event stores, message buses, or having two databases — you can do CQRS with one table and two views, and most systems should."),
			text("ax-cq-already-pitch",
				"This guide has been doing the modest version since the api-design topic: five use cases over one repository — list and get on the query side, create, replace, and delete on the command side — each validating exactly what its verb needs and nothing else. The query side's whole rule set is pagination and sorting; the command side's is the draft's shape and length rules. Quoted side by side, the separation reads as design:"),
			code("ax-cq-query-quote", "go",
				"The query half: normalize, paginate, answer — no mutation rules because nothing mutates.",
				"server/internal/domain/posts/usecases.go",
				"// ListPostsUseCase returns one honest page of posts for a normalized query.\n"+
					"type ListPostsUseCase struct {\n"+
					"\trepository PostsRepository\n"+
					"}\n"+
					"// …\n"+
					"// Execute lists through the port; the repository owns filtering, sorting,\n"+
					"// and the page math's raw material.\n"+
					"func (uc *ListPostsUseCase) Execute(ctx context.Context, query ListQuery) (PostPage, error) {\n"+
					"\treturn uc.repository.List(ctx, query)\n"+
					"}"),
			code("ax-cq-command-quote", "go",
				"The command half: normalize, validate every issue into one 422, commit only when clean — the write model's discipline.",
				"server/internal/domain/posts/usecases.go",
				"// Execute normalizes the draft (so rules judge canonical text), validates\n"+
					"// it (collecting every issue into one 422), and stores it only when clean.\n"+
					"func (uc *CreatePostUseCase) Execute(ctx context.Context, draft Draft) (Post, error) {\n"+
					"\tnormalized := draft.NormalizeDraft()\n"+
					"\tif issues := normalized.ValidateDraft(); len(issues) > 0 {\n"+
					"\t\treturn Post{}, core.ValidationFailed(\"Invalid post body.\", issues)\n"+
					"\t}\n"+
					"\treturn uc.repository.Create(ctx, normalized)\n"+
					"}"),
			text("ax-cq-reading",
				"The full pattern is one more move: the read side stops sharing the write side's STORAGE at all. Commands commit through the write model; a projection builds read-shaped views from what was committed — and the projection may lag, cache, rebuild, and multiply per screen without the write model ever knowing. The moment reads and writes split storage, each side scales on its own axis (topic 40's statelessness argument, now with two levers instead of one), and the consistency between the two models becomes EXPLICITLY eventual — the read model converges, it is never guaranteed current. That trade is the whole decision: screens that tolerate a beat of staleness (catalog pages, dashboards, activity feeds) thrive; screens that must read their own write back immediately (the checkout total) need the read-your-writes guarantee engineered in, usually by routing that one read at the write model."),
			text("ax-cq-bridge",
				"The natural bridge to this topic's second half: event sourcing is one particularly honest way to feed the read side. If every state change is an immutable event, then ANY read model is a fold over the events — the write model is the append path, every projection is EventFold with a different apply function, and rebuilding a read model is replaying history. The next section builds exactly that, with real machinery: the EventProjection type IS the CQRS boundary — state beside the version it provably reflects. But keep the ordering straight: CQRS does not require event sourcing (a plain table plus updatable views is CQRS), and event sourcing does not require CQRS — they compose because each is good at what the other needs. What neither excuses is skipping the decision: know which screens read which model, and know the staleness budget of every one."),
		),
		// ── 7. Event sourcing ────────────────────────────────────
		section("event-sourcing", "Event sourcing: state as a derivation",
			"This section carries the topic's second demo — the real event store, one inventory stream driven through the full command path, a typed conflict, and rebuild proofs.",
			text("ax-es-pitch",
				"Event sourcing inverts the usual storage question. Instead of storing the CURRENT state and overwriting it on every change, you store every CHANGE — an append-only ledger of immutable, business-named facts — and derive the state by replaying them in order. `inventory.stocked 50`, `inventory.reserved 20 for order-A` — never `SET available = 30`. Three properties fall out of the inversion, and they are the pattern's whole argument: the state at ANY past moment is reconstructable (fold to version N); the audit trail is not a feature bolted on but the primary record itself; and concurrency becomes checkable, because the append names the version it believes in and the store refuses liars. The costs are equally structural: reading requires folding (so projections and snapshots stop it being O(history) per read), deleting data is a legal impossibility that must be designed around (GDPR's right-to-erasure lands as crypto-shredding or contract terms), and schema evolution means old events must forever fold correctly — version the payloads, evolve the apply function."),
			text("ax-es-machinery-pitch",
				"The machinery is real and in this repo — `internal/domain/patterns/eventstore.go`, stdlib-only by design because the pattern, not the storage engine, is the lesson; its doc comment names the swap explicitly (the in-memory map for a PostgreSQL table, and every guarantee survives). The entry shape is where the discipline starts, because what a caller may NOT construct is the interesting part: no version (the store assigns them; a caller-chosen version would let history be forged) and no store clock — event time is CALLER-OWNED data, which is what makes replays deterministic:"),
			code("ax-es-append-shape-quote", "go",
				"The only shape a caller may build: what happened, its payload, when it happened — the store owns where it lands in history.",
				"server/internal/domain/patterns/eventstore.go",
				"// EventAppend is one event on its way IN — the only shape a caller may\n"+
					"// construct. It deliberately carries no version (the store assigns\n"+
					"// versions; a caller-chosen version would let history be forged), no\n"+
					"// stream id (the Append call carries it), and a byte payload that the\n"+
					"// store immediately freezes into a string. RecordedAt is supplied by the\n"+
					"// CALLER, never read from a clock inside the store: event time is\n"+
					"// data about the fact, the store stays deterministic, and replays\n"+
					"// reproduce byte-for-byte without mocking time.\n"+
					"type EventAppend struct {\n"+
					"\t// Type names what happened, e.g. \"inventory.reserved\". Non-empty.\n"+
					"\tType string\n"+
					"\t// Payload is the fact's body — conventionally JSON. May be empty\n"+
					"\t// for marker events that carry no data beyond their type.\n"+
					"\tPayload []byte\n"+
					"\t// RecordedAt is when the fact occurred, supplied by the caller.\n"+
					"\t// Must be non-zero; the store never reads the wall clock.\n"+
					"\tRecordedAt time.Time\n"+
					"}"),
			text("ax-es-concurrency-pitch",
				"The heart of the store is optimistic concurrency — the same idea the databases topic showed as row versioning, now as the ENTIRE locking story. Append takes the version the caller believes the stream is at; if the stream moved on, nothing lands and the caller gets a typed conflict carrying both numbers. The check-and-append is one unit under one lock, so a batch either lands whole with consecutive versions or not at all — the transaction the store CAN honestly offer, because it never leaves the process:"),
			code("ax-es-conflict-quote", "go",
				"The version check IS the lock: believe in a stale version and nothing lands — with both numbers in the typed error.",
				"server/internal/domain/patterns/eventstore.go",
				"\tcurrent := int64(len(s.streams[streamID]))\n"+
					"\tif expectedVersion != EventStreamVersionAny && expectedVersion != current {\n"+
					"\t\treturn nil, &EventStoreConflict{\n"+
					"\t\t\tStreamID:        streamID,\n"+
					"\t\t\tExpectedVersion: expectedVersion,\n"+
					"\t\t\tActualVersion:   current,\n"+
					"\t\t}\n"+
					"\t}\n"+
					"\n"+
					"\tstored := make([]EventRecord, 0, len(events))\n"+
					"\tfor i, event := range events {\n"+
					"\t\tstored = append(stored, EventRecord{\n"+
					"\t\t\tStreamID:   streamID,\n"+
					"\t\t\tVersion:    current + int64(i) + 1,\n"+
					"\t\t\tType:       event.Type,\n"+
					"\t\t\tPayload:    string(event.Payload), // frozen: the []byte can no longer be edited through any alias\n"+
					"\t\t\tRecordedAt: event.RecordedAt,\n"+
					"\t\t})\n"+
					"\t}\n"+
					"\ts.streams[streamID] = append(s.streams[streamID], stored...)\n"+
					"\treturn stored, nil"),
			text("ax-es-recovery-pitch",
				"Note the teaching move in the error itself — the message does not stop at what failed, it names the recovery loop, because in event sourcing the recovery is not exceptional boilerplate, it is the NORMAL write path under contention: reload, revalidate the intent against fresh history, retry with the fresh version:"),
			code("ax-es-error-quote", "go",
				"An error message that teaches the recovery — the loop is the write path, not a fallback.",
				"server/internal/domain/patterns/eventstore.go",
				"// Error implements the error interface. The message teaches the\n"+
					"// recovery, not just the failure.\n"+
					"func (e *EventStoreConflict) Error() string {\n"+
					"\treturn fmt.Sprintf(\n"+
					"\t\t\"event store conflict on stream %q: expected version %d, actual %d — the stream gained %d event(s) this writer has not seen; reload, revalidate, retry\",\n"+
					"\t\te.StreamID, e.ExpectedVersion, e.ActualVersion, e.ActualVersion-e.ExpectedVersion)\n"+
					"}"),
			text("ax-es-command-path-pitch",
				"Those pieces assemble into the command path — the loop every event-sourced aggregate runs: rebuild the state by replay, DECIDE the command against that state (the aggregate's rules are ordinary domain logic; what makes them event-sourced is where the input came from), append the decision naming the version it was made against. This is CQRS from the previous section with the boundary drawn exactly: the fold is the read model, the append is the write model, the version ties them together. The demo drives one inventory stream through it; the driver, quoted:"),
			code("ax-es-command-quote", "go",
				"The command path: project (replay) → decide → append with the projection's version — the whole pattern in one function.",
				"server/internal/domain/demos/arch_eventstore.go",
				"\tprojection, err := patterns.EventProject(ctx, store, archEventstoreStreamID, archEventstoreNewFold(), archEventstoreApply)\n"+
					"\tif err != nil {\n"+
					"\t\tpanic(\"arch eventstore: the command path's projection failed: \" + err.Error())\n"+
					"\t}\n"+
					"\tdecision := archEventstoreDecide(kind, qty, ref, projection.State)\n"+
					"\tif decision.refusal != \"\" {\n"+
					"\t\treturn archEventstoreCommandResult{version: projection.Version, refusal: decision.refusal}\n"+
					"\t}\n"+
					"\trecords, err := store.Append(ctx, archEventstoreStreamID, projection.Version, patterns.EventAppend{\n"+
					"\t\tType:       decision.eventType,\n"+
					"\t\tPayload:    []byte(decision.payload),\n"+
					"\t\tRecordedAt: archEventstoreEventClock(step),\n"+
					"\t})\n"+
					"\tvar conflict *patterns.EventStoreConflict\n"+
					"\tif errors.As(err, &conflict) {\n"+
					"\t\treturn archEventstoreCommandResult{version: conflict.ActualVersion, eventType: decision.eventType, conflict: conflict}\n"+
					"\t}"),
			text("ax-es-fold-pitch",
				"The rebuild is a pure generic function — free, not a method, because Go methods cannot introduce their own type parameters, and replay is a property of the EVENTS, not of any store. Determinism is the contract that makes it all trustworthy: the same events folded by the same apply must always build the same state, or projections become disposable and the ledger meaningless. The store's answer pairs the state with the version it provably reflects — the CQRS boundary as a type:"),
			code("ax-es-project-quote", "go",
				"The rebuild: state beside the version it provably reflects — the next append's expectedVersion, closing the loop safely.",
				"server/internal/domain/patterns/eventstore.go",
				"type EventProjection[S any] struct {\n"+
					"\tState   S\n"+
					"\tVersion int64\n"+
					"}\n"+
					"// …\n"+
					"func EventProject[S any](ctx context.Context, store *EventStore, streamID string, initial S, apply func(S, EventRecord) S) (EventProjection[S], error) {\n"+
					"\tevents, err := store.Stream(ctx, streamID)\n"+
					"\tif err != nil {\n"+
					"\t\tvar zero EventProjection[S]\n"+
					"\t\treturn zero, fmt.Errorf(\"event store: projection of %q: %w\", streamID, err)\n"+
					"\t}\n"+
					"\tif apply == nil {\n"+
					"\t\tvar zero EventProjection[S]\n"+
					"\t\treturn zero, fmt.Errorf(\"event store: projection of %q needs an apply fold\", streamID)\n"+
					"\t}\n"+
					"\tstate := EventFold(events, initial, apply)\n"+
					"\treturn EventProjection[S]{State: state, Version: int64(len(events))}, nil\n"+
					"}"),
			text("ax-es-stream-pitch",
				"One more store decision earns its quote because it is the vocabulary lesson: a stream that never received an event answers an EMPTY slice — not an error, not nil. In event sourcing there is no 'does not exist' state, only 'no events yet'; an absent stream and an empty stream fold to the same initial state, and the store answers both identically. The zero-event fold is the base case that makes replay total:"),
			code("ax-es-stream-quote", "go",
				"No 'missing' state — absent and empty fold alike; the copies make history unreachable for mutation.",
				"server/internal/domain/patterns/eventstore.go",
				"// Stream answers the stream's records in version order, as fresh\n"+
					"// copies. A stream that never received an event answers an EMPTY slice,\n"+
					"// not an error and not nil: in event sourcing there is no \"does not\n"+
					"// exist\" state, only \"no events yet\" — an absent stream and an empty\n"+
					"// stream fold to the same initial state, so the store answers both the\n"+
					"// same way. The copies matter: mutating the returned slice or its\n"+
					"// records cannot reach the stored history.\n"+
					"func (s *EventStore) Stream(ctx context.Context, streamID string) ([]EventRecord, error) {\n"+
					"// …\n"+
					"\tsource := s.streams[streamID]\n"+
					"\tif len(source) == 0 {\n"+
					"\t\treturn []EventRecord{}, nil\n"+
					"\t}\n"+
					"\tcopies := make([]EventRecord, len(source))\n"+
					"\tcopy(copies, source)\n"+
					"\treturn copies, nil\n"+
					"\t}"),
			text("ax-es-demo-pitch",
				"The demo puts the whole discipline on the wire in three acts over one inventory item's stream. Act one — the lifecycle: stock the item, reserve for two orders, every command through the full rebuild-decide-append path, the state never stored anywhere. Act two — the collision: a second writer loads the stream at version 3, writer one picks order-A's reservation (v4), the second writer's stale append naming expected=3 meets the typed conflict — and the recovery runs BOTH halves: the original intent revalidated against fresh history is REFUSED by the aggregate's own rules (the reservation it wanted to release was already consumed — the conflict saved the ledger), then a genuinely valid intent lands at the new head. Act three — the proofs: the stream rebuilt from nothing twice with encoding/json comparing the bytes, the result cross-checked against a naive ledger computed by plain arithmetic that never touched the fold, and a stream that never received an event folding to the zero state at version 0:"),
			demo("arch:eventstore", "The event store, end to end",
				"GET /api/demo/arch/eventstore serves the REAL patterns.EventStore driven through one inventory stream's complete lifecycle · ACT ONE — the command path: stock, reserve, reserve; every append naming the "+
					"projection's version, every state line a DERIVATION from replay, nothing stored · ACT TWO — the conflict: a second writer's stale append refused by the typed *EventStoreConflict (expected 3, actual 4, nothing landed), "+
					"the recovery revalidating the stale intent against reloaded history and REFUSING it — then the valid intent landing at the new head · ACT THREE — the proofs: two byte-identical rebuilds from the zero state, an independently "+
					"computed naive ledger agreeing exactly, and the absent stream folding to the zero state at version 0 · every version, count, and state observed from the real store at a fixed exhibit clock — two GETs serve byte-identical bodies"),
			text("ax-es-demo-reading",
				"Read act two twice, because it carries the pattern's most counterintuitive lesson. The conflict is not a NUISANCE to be retried blindly — the recovery's first half revalidates the original intent against fresh history and the aggregate refuses it, which is the system telling you the decision you made is no longer valid in the world that exists. Accepting the stale append would have released a reservation the pick already consumed: optimistic concurrency did not just prevent a lost update, it prevented a WRONG BUSINESS ACTION. The typed error carries both versions precisely so this loop is code, not heroics — errors.As on the conflict, decide again, append again. The handler below the demo is the same three lines as the saga's — the machinery lives behind the port and the pure builder, and the composition root registers it beside its sibling:"),
			code("ax-es-handler-quote", "go",
				"The sibling endpoint, equally thin — a fresh exhibit per request, deterministic by construction.",
				"server/internal/presentation/demo_arch_eventstore.go",
				"func (h *ArchEventstoreHandler) ServeHTTP(w http.ResponseWriter, _ *http.Request) {\n"+
					"\twriteJSON(w, http.StatusOK, demos.BuildArchEventstorePayload())\n"+
					"}"),
			text("ax-es-coda",
				"Where event sourcing genuinely earns its costs, and where it does not. It earns them where the EVENT LOG IS THE BUSINESS — payments and ledgers (the audit trail is the product), inventory and logistics (every movement is a fact), collaborative domains (the conflict loop is the workflow), and any system whose past states must answer questions nobody thought to ask yet (rebuild yesterday's exposure by folding to yesterday's version). It does not earn them where state is disposable working data — a shopping cart's row can be UPDATEd because nobody audits the fourth keystroke of an address form; a session's metadata; a cache. The guide's own line is the honest one to copy: the posts surface stores state (its questions are always about the present), while the patterns package stores facts (its questions are about history) — both in one binary, chosen per aggregate, which is the real advanced architecture: not a system built on one pattern, but one where each boundary pays for the pattern it actually needs."),
		),
		// ── 8. Tracing across boundaries ─────────────────────────
		section("tracing-across-boundaries", "Tracing across boundaries",
			"The third leg: once work spans processes, correlation is a design decision — trace ids, span trees, and the saga ledger as a domain-level trace.",
			text("ax-tr-pitch",
				"Every pattern so far moved WORK across a boundary; this one moves UNDERSTANDING across it. The moment two services cooperate on one user request, the question 'what happened, and where did the time go' stops having an answer any single log can give: service A's log has the request id, service B's has its own, and the network time between them lives in neither. Topic 33 built the whole answer out of two standards — structured logs correlated by request id, and distributed tracing correlated by trace id — and topic 32's arch:tracer demo walked one request through the layers of THIS process. Across boundaries the mechanics stay exactly the same; only the grafting point matters: the FIRST span a server opens under an inbound tracecontext parents on the REMOTE caller's span id, which is what welds one process's local tree onto the distributed one. The guide's miniature tracer states it in the code a reader meets first:"),
			code("ax-tr-span-quote", "go",
				"The graft point of a distributed trace: the first local span parents on the remote caller — context in, context out, one parent pointer.",
				"server/internal/domain/obs/spans.go",
				"func (r *SpanRecorder) Start(ctx context.Context, name, layer string) (context.Context, *Span) {\n"+
					"\ttraceID := NewTraceID()\n"+
					"\tparentID := \"\"\n"+
					"\tif tc, ok := TraceContextFromContext(ctx); ok {\n"+
					"\t\ttraceID = tc.TraceID\n"+
					"\t\t// The FIRST span under an inbound context parents on the REMOTE\n"+
					"\t\t// caller's span id — that is what grafts the local tree onto the\n"+
					"\t\t// distributed one.\n"+
					"\t\tparentID = tc.ParentID\n"+
					"\t}\n"+
					"\tif current, ok := currentSpanID(ctx); ok {\n"+
					"\t\tparentID = current\n"+
					"\t}\n"+
					"\tspan := &Span{\n"+
					"\t\tTraceID:  traceID,\n"+
					"\t\tSpanID:   r.newID(),\n"+
					"\t\tParentID: parentID,\n"+
					"\t\tName:     name,\n"+
					"\t\tLayer:    layer,\n"+
					"\t\tStart:    r.now(),\n"+
					"\t\trecorder: r,\n"+
					"\t}\n"+
					"\treturn withCurrentSpanID(ctx, span.SpanID), span\n"+
					"}"),
			text("ax-tr-context-pitch",
				"The id that travels is the W3C tracecontext header the observability topic implemented by hand — trace id, parent id, flags — and the outbound half is one derivation: the next hop's context is a child of this one, same trace, fresh span id, flags carried. Two lines of protocol vocabulary that make a ten-service chain one queryable tree:"),
			code("ax-tr-child-quote", "go",
				"The outbound half: same trace, fresh span — the header that connects a response to the work it caused.",
				"server/internal/domain/obs/tracecontext.go",
				"// Child derives the context for the next span under this one: same trace,\n"+
					"// fresh span id, flags carried. The server's outbound header carries the\n"+
					"// child so the caller can connect the response to the work it caused.\n"+
					"func (tc TraceContext) Child() TraceContext {\n"+
					"\treturn TraceContext{\n"+
					"\t\tVersion:    \"00\",\n"+
					"\t\tTraceID:    tc.TraceID,\n"+
					"\t\tParentID:   NewSpanID(),\n"+
					"\t\tFlags:      tc.Flags,\n"+
					"\t\tTracestate: tc.Tracestate,\n"+
					"\t}\n"+
					"}"),
			text("ax-tr-saga-bridge",
				"The bridge to this topic's machinery: a saga's ledger IS a trace, one level up the stack. The span tree correlates MECHANICAL work (which function in which process, how long); the ledger correlates BUSINESS work (which step of which transaction, what outcome, what it triggered and what undid it) — and crucially, the ledger survives the run: after the failure, the rows say forever that step 2 refused, steps 0 and 1 were compensated, step 3 never ran. Distributed systems fail in compositions, and the incident question is always 'which composition': a trace answers where the time went, a ledger answers what the business did about it, and the two correlate because the saga's compensating actions run on a context whose VALUES — request id, trace id — survived the cancellation that stopped the forward work (the WithoutCancel decision from the saga section, paying out exactly here). Propagate the ids everywhere: it is the cheapest observability that exists, and nothing else works without it."),
			text("ax-tr-coda",
				"This is also where the guide's error discipline (topic 30) crosses the boundary with you. A failure in service B must arrive at service A as a TYPED, classified error — never a string to parse, never a swallowed 500 — because the saga's SagaStepError and the store's EventStoreConflict are only useful if what they wrap is inspectable; errors.Is across a network means mapping to codes at the edge and back to types at the seam, which is precisely what the third-party topic's adapter classification did for vendor errors. Trace ids correlate the logs, typed errors carry the semantics, the ledger and the span tree carry the two histories. That is the full toolkit for work that spans boundaries — and it closes the curriculum: consistency chosen per operation, a vocabulary the wire shares, the monolith whose seams are ready, messages made durable and safe, transactions that compensate, state that derives from facts, and understanding that follows the work everywhere it goes."),
		),
		// ── 9. Terminal companion ────────────────────────────────
		section("terminal-companion", "Terminal companion: the capstone over curl",
			"The final tour: both demos' determinism proven live, the compensation ledger read, the conflict numbers read, and the curriculum's shape closed.",
			text("ax-terminal-pitch",
				"The last tour of the guide walks this topic's two endpoints on the running stack. Both are stateless and clock-free — the saga's runs are pure functions of a fixed script, the event store's exhibit runs at a fixed exhibit clock — so the determinism claim is checkable with a diff against the LIVE endpoints, and it should print nothing but the echo. Then the ledger, the conflict, and the shape of the whole curriculum:"),
			code("ax-terminal-tour-1", "bash", "", "",
				"# stop 1 — the saga: three observed runs, reverse compensation, idempotent replay\n"+
					"curl -s localhost:8100/api/demo/arch/saga | grep -o '\"status\":\"[a-z]*\"'\n"+
					"curl -s localhost:8100/api/demo/arch/saga | grep -o '\"outcome\":\"[a-z-]*\"' | sort | uniq -c\n"+
					"# the failure run's undo order — refund before release, newest first:\n"+
					"curl -s localhost:8100/api/demo/arch/saga | grep -o '\"detail\":\"[^\"]*' | head -n 6\n"+
					"\n"+
					"# determinism, proven against the live endpoint — expect the echo, nothing else:\n"+
					"diff <(curl -s localhost:8100/api/demo/arch/saga) \\\n"+
					"     <(curl -s localhost:8100/api/demo/arch/saga) && echo deterministic"),
			code("ax-terminal-tour-2", "bash", "", "",
				"# stop 2 — the event store: the conflict's both numbers, and the recovery\n"+
					"curl -s localhost:8100/api/demo/arch/eventstore | grep -o '\"outcome\":\"[a-z-]*\"' | sort | uniq -c\n"+
					"curl -s localhost:8100/api/demo/arch/eventstore | grep -o '\"expectedVersion\":[0-9]*'\n"+
					"curl -s localhost:8100/api/demo/arch/eventstore | grep -o '\"byteIdentical\":[a-z]*'\n"+
					"diff <(curl -s localhost:8100/api/demo/arch/eventstore) \\\n"+
					"     <(curl -s localhost:8100/api/demo/arch/eventstore) && echo deterministic"),
			code("ax-terminal-tour-3", "bash", "", "",
				"# stop 3 — this topic's shape through the content API\n"+
					"curl -s localhost:8100/api/content | grep -o '\"id\":\"advanced-architecture\"[^}]*'\n"+
					"curl -s localhost:8100/api/content/advanced-architecture | grep -o '\"id\":\"[a-z-]*\"'\n"+
					"curl -s localhost:8100/api/content/advanced-architecture | grep -c '\"kind\":\"demo\"'\n"+
					"# the counts the runs earned — both demos, side by side:\n"+
					"curl -s localhost:8100/api/demo/arch/saga | grep -o '\"label\":\"[^\"]*\",\"value\":[0-9]*'\n"+
					"curl -s localhost:8100/api/demo/arch/eventstore | grep -o '\"label\":\"[^\"]*\",\"value\":[0-9]*' | head -n 5"),
			code("ax-terminal-tour-4", "bash", "", "",
				"# stop 4 — through the client's dev proxy, as the browser reaches it,\n"+
					"# and the honest 404 as the curriculum closes it\n"+
					"curl -s localhost:5175/api/demo/arch/saga | grep -o '\"status\":\"[a-z]*\"'\n"+
					"curl -s localhost:5175/api/demo/arch/eventstore | grep -o '\"byteIdentical\":[a-z]*'\n"+
					"curl -s -i localhost:8100/api/content/advanced-architecture-nope | head -n 6"),
			text("ax-terminal-coda",
				"Stops 1–2 are this topic's thesis in payloads: compensation recorded in reverse with the replay proving idempotency as data, and a typed conflict whose both numbers — expected and actual — sit on the wire beside a recovery that refused the stale intent before accepting the valid one. Stop 3 reads the topic's own shape — eight teaching sections, two demos inline in the sections that teach them. Stop 4 closes the family loop the way every topic has: the demo through the client's proxy, and the unknown topic answered by the one error envelope this guide has served since its first route. Forty-three topics, one binary, every quote on this page from code that runs — that is the guide, and the terminal just proved it one last time."),
		),
	}

	return Topic{
		ID:    "advanced-architecture",
		Title: "Advanced architecture",
		Subtitle: "The architecture capstone: what happens when the boundary moves past the process — the consistency map that chooses " +
			"the right guarantee per operation · the DDD vocabulary that keeps a growing domain speakable, every term mapped to a real " +
			"shape in this repo · the monolith-first position this guide IS, with seams a future extraction would cut already cut at the " +
			"composition root · what actually travels between services (bounded calls, the transactional outbox, the app-minted " +
			"idempotency id) · the SAGA as a distributed transaction: forward-until-failure, reverse compensation on " +
			"context.WithoutCancel, idempotent undo, a typed step error — real machinery in internal/domain/patterns, driven live " +
			"[demo arch:saga] · CQRS as separation, not ceremony — the five use cases you already have, and the event-sourced boundary " +
			"· EVENT SOURCING: state as a derivation from an append-only ledger, optimistic concurrency as the version check, " +
			"rebuild-decide-append as the command path — real store, one stream's whole lifecycle including the conflict and its " +
			"recovery [demo arch:eventstore] · and tracing across the boundary: the graft point, the child context, and the saga " +
			"ledger as a domain-level trace.",
		Group:    "capstones",
		Order:    43,
		Sections: sections,
	}, buildErr
}
