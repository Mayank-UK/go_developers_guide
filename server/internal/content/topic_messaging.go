package content

import (
	"errors"
	"fmt"
)

// This file is topic 22 messaging (group state-auth, curriculum order
// 22): why systems message at all (decoupling in time), the log-versus-
// queue mental model, consumer groups and ordering walked live through
// Redis Streams (demo msg:stream), the three delivery guarantees argued
// honestly, the pending ledger and XAUTOCLAIM as THE crash-recovery
// story, the transactional outbox that welds "commit the change" and
// "publish the event" into one all-or-nothing unit across PostgreSQL and
// Redis (demo msg:outbox), the idempotent consumer that makes
// at-least-once safe, and where Kafka and RabbitMQ honestly earn their
// keep over the same vocabulary. The terminal companion closes with
// redis-cli's stream commands and the curl tour.

// messagingTopic builds topic 22.
func messagingTopic() (Topic, error) {
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
		// ── 1. Why messaging ────────────────────────────────────────
		section("why-messaging", "Why systems message at all",
			"The request committed — now what? Messaging decouples the follow-on work in time, in failure, and in audience.",
			text("messaging-after-the-commit",
				"Every topic so far has answered a request inside the request: the handler called a use case, the use "+
					"case committed to PostgreSQL, the answer went back. But look closely at what that commit DID — "+
					"say it stored a paid order — and a second list of obligations appears beside it: send the "+
					"receipt, update the search index, tell the analytics pipeline, release the shipment. You CAN "+
					"do all four inside the request (and topic 29 will show why you mostly should not: latency, "+
					"partial failure, blast radius), but the moment you do them later, you have introduced "+
					"messaging — the disciplined answer to \"this work must happen, but not necessarily now, "+
					"and not necessarily by me\". The discipline has three parts: the message must SURVIVE the "+
					"gap between \"decided\" and \"done\"; the sender must not have to know who consumes; and "+
					"the system must behave when a consumer dies halfway through. Everything in this topic is "+
					"one of those three parts wearing a command name."),
			text("messaging-jobs-pitch",
				"What messaging buys you, concretely, is best seen as four jobs the request-response world has no "+
					"answer for. None of them is about speed — a message hop is slower than a function call. All "+
					"of them are about ROBUSTNESS at the boundaries where work outlives the request that caused "+
					"it:"),
			list("messaging-jobs", false,
				"Deferred work — the request commits the decision and returns; the slow follow-on (email, export, resize) happens on a worker, later.",
				"Decoupled audiences — the producer publishes one fact; N independent consumers subscribe without the producer naming any of them.",
				"Load leveling — a burst of arrivals queues up and drains at the consumers' pace instead of toppling them.",
				"Failure isolation — a consumer that crashes stops ITS work, not the producer's requests; recovery is a ledger, not an outage.",
			),
			text("messaging-in-this-stack",
				"This guide implements messaging on Redis Streams — the log structure already inside the store topic "+
					"20 introduced (its structure map pointed here on purpose). The choice is the family's "+
					"optional-service posture again: PostgreSQL stays the system of record, Redis carries the "+
					"message traffic, and if Redis is down the messaging endpoints degrade to the typed 503 "+
					"rather than failing anything else. The Go side is the same ``go-redis/v9`` client from the "+
					"redis topic (``GDG_REDIS_ADDR``, lazy dial, short timeouts), and the layering is the "+
					"guide's usual split: the ports and vocabulary live in ``internal/domain/messaging`` (a new "+
					"domain area — producer, consumer group, deduplication seam, outbox), the go-redis and pgx "+
					"implementations in ``internal/data/messaging``. Handlers depend on the ports; the "+
					"composition root wires the adapters."),
		),
		// ── 2. Logs vs queues ───────────────────────────────────────
		section("logs-vs-queues", "Logs versus queues",
			"Two models hide under the word \"message\" — delete-on-read queues and append-only logs — and every broker you will meet is one of them.",
			text("queue-model",
				"A QUEUE is the mental model most people arrive with: messages wait in order, a consumer takes "+
					"one, and taking it REMOVES it — the message is delivered to exactly one consumer and "+
					"then it is gone. RabbitMQ works this way, and so do cloud queues like SQS. The model's "+
					"virtues are simplicity and automatic cleanup: nothing accumulates, nothing is read "+
					"twice by design. Its costs are just as real: the message cannot be re-read after "+
					"processing (a bug in the consumer means the data is lost unless you built a dead-letter "+
					"queue), a second TEAM cannot independently consume the same traffic (taking the "+
					"message starves the other reader), and there is no \"replay from last Tuesday\" — "+
					"the queue is a conveyor belt, not a record."),
			text("log-model",
				"A LOG keeps the messages instead: producers append, the log retains them (bounded by retention "+
					"or trimming), and each reader tracks its OWN position in it — reading advances the "+
					"reader's cursor, not the log. Kafka made this model famous; Redis Streams is the same "+
					"idea at the scale of one store: ``XADD`` appends an entry with a store-assigned id, "+
					"entries stay in the stream, and readers consume by id. The log's virtues are the "+
					"queue's costs inverted: many independent readers can walk the same history, replay is "+
					"just \"read from an older id\", and consumers can even rewind. Its cost is that SOMEBODY "+
					"must manage retention — an untrimmed log grows forever — and that delivery bookkeeping "+
					"moves from the broker's take-semantics into explicit per-group state. That state is "+
					"exactly the consumer group this topic builds on."),
			text("pubsub-recap-and-step-up",
				"Topic 20's pub/sub is the third model, and its limits are the fastest way to see what a stream "+
					"adds. Pub/sub delivers to whoever is subscribed RIGHT NOW: no persistence, no replay, "+
					"no ledger — a subscriber that was reconnecting when the message flew past simply never "+
					"sees it. That is fine for presence pings and cache-invalidation hints, and wrong for "+
					"anything that must not be lost. A stream is pub/sub's serious sibling: the message is "+
					"STORED first and delivered second, so a consumer that arrives late — or crashes and "+
					"comes back — resumes from where its group left off. The guide's two message demos "+
					"both run on streams for exactly that reason."),
			code("group-port-go", "go",
				"The domain port the whole topic consumes through: a consumer group as six operations, in internal/domain/messaging.",
				"server/internal/domain/messaging/streams.go",
				"type GroupStore interface {\n"+
					"    // EnsureGroup creates the group reading from start (\"0-0\" to include\n"+
					"    // history, \"$\" for new entries only); an existing group is not an\n"+
					"    // error — the idempotent posture a restarting worker needs.\n"+
					"    EnsureGroup(ctx context.Context, stream, group, start string) error\n"+
					"    // ReadNew delivers up to count never-delivered entries to consumer.\n"+
					"    // A healthy empty read is an empty slice, not an error.\n"+
					"    ReadNew(ctx context.Context, stream, group, consumer string, count int) ([]Entry, error)\n"+
					"    // Ack marks ids done, removing them from the group's pending list.\n"+
					"    Ack(ctx context.Context, stream, group string, ids ...string) (int64, error)\n"+
					"    // …PendingEntries and ClaimIdle are the ledger and its recovery, next sections\n"+
					"    ClaimIdle(ctx context.Context, stream, group, consumer string, minIdle time.Duration, count int) ([]Entry, error)\n"+
					"}"),
		),
		// ── 3. Consumer groups and ordering ─────────────────────────
		section("consumer-groups-and-ordering", "Consumer groups and ordering",
			"One group over a stream is a work queue; several groups are fan-out — and inside one stream, the log's order is the law.",
			text("group-anatomy",
				"A consumer GROUP is the log's answer to \"who has seen what\". The group owns a per-stream "+
					"cursor (the last id delivered to it) plus a pending LEDGER — the list of entries "+
					"delivered but not yet acknowledged. Consumers are just names inside the group "+
					"(``consumer-a``, ``worker-3``). Reading with ``XREADGROUP … >`` asks for entries the "+
					"GROUP has never been delivered; the store assigns each such entry to exactly ONE of "+
					"the group's consumers and records it in the ledger under that consumer's name. That "+
					"one rule produces both scaling shapes at once: add consumers to ONE group and they "+
					"split the work (each entry to exactly one consumer — a queue); add a SECOND group and "+
					"it independently walks the same stream from its own cursor (every entry to every "+
					"group — fan-out). The demo below runs both shapes deliberately."),
			code("append-beat-go", "go",
				"The produce side: one XADD per event; the STORE assigns the id, because the id's ordering is the stream's ordering.",
				"server/internal/data/messaging/streams.go",
				"func (s *StreamStore) Append(ctx context.Context, stream string, values map[string]string) (string, error) {\n"+
					"    id, err := s.client.XAdd(ctx, &goredis.XAddArgs{\n"+
					"        Stream: stream,\n"+
					"        Values: stringMapToAny(values),\n"+
					"    }).Result()\n"+
					"    if err != nil {\n"+
					"        return \"\", fmt.Errorf(\"XADD %s: %w\", stream, err)\n"+
					"    }\n"+
					"    return id, nil\n"+
					"}"),
			text("ordering-law",
				"Ordering inside one stream is absolute because the log is append-only and ids are "+
					"``<milliseconds>-<sequence>`` — time-ordered by construction, and comparable as "+
					"strings. Entries are read back in insertion order, full stop. What a stream does NOT "+
					"promise is ordering ACROSS streams or partitions: Kafka's partitions and any "+
					"multi-stream design trade global order for parallelism — the discipline is \"one "+
					"stream (or partition) per entity whose order matters\", the same way topic 17 put "+
					"one row per identity. The adapter's read carries the one trap worth pinning in a "+
					"test: go-redis serializes the ``Block`` field for any value ≥ 0, and ``BLOCK 0`` "+
					"means \"block forever\" — a non-blocking poll must pass ``-1``:"),
			code("readnew-beat-go", "go",
				"The consume side: XREADGROUP with > (never-delivered), non-blocking, and redis.Nil mapped to a healthy empty slice.",
				"server/internal/data/messaging/streams.go",
				"    result, err := s.client.XReadGroup(ctx, &goredis.XReadGroupArgs{\n"+
					"        Group:    group,\n"+
					"        Consumer: consumer,\n"+
					"        Streams:  []string{stream, \">\"},\n"+
					"        Count:    int64(count),\n"+
					"        Block:    -1, // any value ≥ 0 sends BLOCK — and BLOCK 0 is FOREVER\n"+
					"        NoAck:    false,\n"+
					"    }).Result()\n"+
					"    if err != nil {\n"+
					"        if errors.Is(err, goredis.Nil) {\n"+
					"            return []messaging.Entry{}, nil\n"+
					"        }\n"+
					"        return nil, fmt.Errorf(\"XREADGROUP %s %s %s: %w\", stream, group, consumer, err)\n"+
					"    }"),
			demo("msg:stream", "The stream story: groups, ordering, recovery",
				"One request against the guide's RUNNING Redis walks the full worker lifecycle: three order "+
					"events XADDed in sequence, the group created at 0-0 so history is in scope, consumer-a "+
					"reading all three in insertion order, consumer-b reading NOTHING new (the group already "+
					"assigned every entry — the queue property), consumer-a crashing with the middle entry "+
					"un-acknowledged, XPENDING naming the stalled entry and its owner, XAUTOCLAIM moving it to "+
					"consumer-b with its values intact, the ack, and the pending ledger drained to zero — "+
					"then the teardown that leaves the store as the run found it."),
			text("stream-story-read",
				"Read the demo's beats as the two proofs they carry. The ORDERING proof: the ids consumer-a "+
					"read are exactly the ids the appends answered, in the same order — the payload derives "+
					"that sentence by comparing the two lists, so it cannot claim an order the store did "+
					"not give. The QUEUE proof: consumer-b's read answers zero entries, because the group "+
					"had already assigned all three to consumer-a; the payload derives that from the empty "+
					"list too. Both derivations live in the pure builder (``internal/domain/demos/"+
					"messaging_stream.go``), the same pattern the redis playground used — the adapter "+
					"reports, the builder proves, and a drifted store answer would surface as a sentence "+
					"saying so rather than a silent contradiction."),
		),
		// ── 4. Delivery guarantees ──────────────────────────────────
		section("delivery-guarantees", "Delivery guarantees: at-most, at-least, exactly-once",
			"Three guarantees, two of them real — and the third an effect you engineer from the second, not a promise a broker can make you.",
			text("guarantees-pitch",
				"\"What happens if a message is lost?\" and \"what happens if it arrives twice?\" are the two "+
					"questions every messaging decision reduces to, and the three classic labels are just "+
					"their answer pairs. The labels get tossed around as feature checkboxes; the honest "+
					"reading is a trade-off axis where each step away from loss moves you toward "+
					"duplicates, and the \"exactly-once\" end of the axis is not a delivery property at "+
					"all:"),
			list("guarantee-list", false,
				"At-most-once — deliver zero or one times; acknowledged before or without processing. Losing messages is possible; duplicates are not. Right for telemetry and presence, wrong for orders.",
				"At-least-once — deliver one or more times; acknowledged only after processing. Duplicates are possible; loss is not (with the pending ledger below). The workhorse guarantee.",
				"Exactly-once — the effect of processing once even under redelivery. No broker hands you this end-to-end; you BUILD it as at-least-once plus an idempotent consumer.",
			),
			text("guarantees-in-streams",
				"Redis Streams' consumer groups are at-least-once by construction: an entry leaves the group's "+
					"ledger only on ``XACK``, so a consumer that dies mid-processing leaves the entry "+
					"pending — redeliverable by claim, never lost. The two failure directions the demo "+
					"pins are the ones production hits: process-then-die-before-ack (the entry comes back — "+
					"a DUPLICATE from the consumer's point of view, the good kind, handled below), and "+
					"ack-then-die (the entry never comes back — which is why the honest order is always "+
					"process FIRST, acknowledge SECOND; acknowledging first is how teams accidentally "+
					"build at-most-once and lose work). The outbox demo later in this topic shows the same "+
					"choice at the relay boundary: publish-then-mark, accepting a possible duplicate "+
					"publish so that a crash can never LOSE an event."),
		),
		// ── 5. Pending entries and XAUTOCLAIM ───────────────────────
		section("pending-and-autoclaim", "Pending entries and XAUTOCLAIM",
			"Delivered but not acknowledged is its own state — the pending ledger — and XAUTOCLAIM is how a live worker inherits a dead one's stalled entries.",
			text("pending-ledger",
				"Every entry a group delivers goes into the group's PENDING list under the consumer that "+
					"received it, and stays there until that entry is acknowledged — seconds or hours, "+
					"however long the work takes. ``XPENDING`` in its extended form reads the ledger "+
					"entry by entry: the id, the owning consumer, how long the entry has been idle, and "+
					"how many times it has been delivered. Those four fields are a complete post-mortem "+
					"table: an entry idle for two minutes with one delivery is probably just slow work; "+
					"idle for an hour with four deliveries is a POISON entry — something crashes every "+
					"worker that touches it — and the retry count is what tells you to stop redelivering "+
					"and route it to a dead-letter stream instead."),
			text("autoclaim-loop",
				"``XAUTOCLAIM`` closes the loop: it atomically takes pending entries that have been idle for "+
					"at least a threshold away from their current owner and reassigns them to the calling "+
					"consumer, returning their ids and values so the new owner can finish the work and "+
					"acknowledge. The idle threshold is the safety mechanism — a live worker mid-flight on "+
					"a slow entry is NOT stalled, and claiming its work early would duplicate the effort; "+
					"the threshold must exceed your slowest legitimate processing time. (The demo passes "+
					"zero because its \"crash\" is staged and the recovery must happen inside one "+
					"request — the same honesty as topic 20 refusing to sleep out a TTL.) A production "+
					"worker's poll loop ends up as: read new, process, ack, then periodically claim idle "+
					"entries and process them again — idempotently, which is exactly why the next-next "+
					"section exists:"),
			code("claimidle-beat-go", "go",
				"The recovery primitive: XAUTOCLAIM reassigns idle pending entries to a live consumer, values intact.",
				"server/internal/data/messaging/streams.go",
				"    claimed, _, err := s.client.XAutoClaim(ctx, &goredis.XAutoClaimArgs{\n"+
					"        Stream:   stream,\n"+
					"        Group:    group,\n"+
					"        Consumer: consumer,\n"+
					"        MinIdle:  minIdle,\n"+
					"        Start:    \"0-0\",\n"+
					"        Count:    int64(count),\n"+
					"    }).Result()\n"+
					"    if err != nil {\n"+
					"        return nil, fmt.Errorf(\"XAUTOCLAIM %s %s %s: %w\", stream, group, consumer, err)\n"+
					"    }"),
			code("crash-beat-go", "go",
				"How the demo stages the crash: acknowledge the outer entries, deliberately leave the middle one pending — then the ledger names it.",
				"server/internal/data/messaging/story.go",
				"    var ackedByA []string\n"+
					"    for i, entry := range consumerA {\n"+
					"        if i == 1 {\n"+
					"            continue // the stalled one\n"+
					"        }\n"+
					"        if _, err := r.store.Ack(ctx, stream, group, entry.ID); err != nil {\n"+
					"            return demos.MessagingStreamRun{}, err\n"+
					"        }\n"+
					"        ackedByA = append(ackedByA, entry.ID)\n"+
					"    }\n"+
					"    stalled := consumerA[1].ID"),
		),
		// ── 6. Transactional outbox ─────────────────────────────────
		section("transactional-outbox", "The transactional outbox",
			"Committing a change in PostgreSQL and publishing an event to Redis are two systems — the outbox welds them into one all-or-nothing unit.",
			text("dual-write-problem",
				"The hardest bug in this whole topic is two innocent lines: commit the database change, then "+
					"publish the event. Two systems, no shared transaction — so the process can die "+
					"BETWEEN them, and the result is a committed order whose ``order.created`` event "+
					"nobody will ever see: the search index misses it, the analytics miss it, and nothing "+
					"errors, because from each system's own view nothing went wrong. This is the DUAL-WRITE "+
					"problem, and retrying harder does not fix it — a publish-retry after a failed commit "+
					"is the mirrored bug (an event for a change that never happened). What fixes it is "+
					"making the two writes ONE write: the transactional outbox."),
			text("outbox-idea",
				"The idea is almost embarrassingly small: instead of publishing inside the request, write the "+
					"event you INTEND to publish into an ``outbox`` table inside the SAME PostgreSQL "+
					"transaction as the business change. Both rows commit together or neither does — "+
					"topic 17's atomicity, spent on messaging. A separate RELAY process then reads "+
					"committed, not-yet-published rows, publishes each to the stream, and stamps it "+
					"published. A crash between commit and relay costs only DELAY (the row is still "+
					"there; the next relay pass publishes it). A crash between publish and stamp costs one "+
					"DUPLICATE publish on restart — the deliberately-accepted price, made safe by the "+
					"idempotent consumer in the next section. The write side, in the guide's real "+
					"adapter:"),
			code("outbox-write-beat-go", "go",
				"The write side: business row and outbox row in ONE pgx transaction — commit both or neither.",
				"server/internal/data/messaging/outbox.go",
				"func (o *Outbox) RecordEventWithOutbox(ctx context.Context, event messaging.EventRecord, message messaging.OutboxMessage) error {\n"+
					"    tx, err := o.pool.Begin(ctx)\n"+
					"    if err != nil {\n"+
					"        return fmt.Errorf(\"begin the outbox transaction: %w\", err)\n"+
					"    }\n"+
					"    defer tx.Rollback(ctx)\n"+
					"\n"+
					"    if _, err := tx.Exec(ctx, `\n"+
					"        INSERT INTO msgdemo_events (id, run_id, kind, payload)\n"+
					"        VALUES ($1, $2, $3, $4)`,\n"+
					"        event.ID, event.RunID, event.Kind, event.Payload); err != nil {\n"+
					"        return fmt.Errorf(\"insert the event row: %w\", err)\n"+
					"    }\n"+
					"    if _, err := tx.Exec(ctx, `\n"+
					"        INSERT INTO msgdemo_outbox (id, run_id, stream, payload)\n"+
					"        VALUES ($1, $2, $3, $4)`,\n"+
					"        message.ID, message.RunID, message.Stream, message.Payload); err != nil {\n"+
					"        return fmt.Errorf(\"insert the outbox row: %w\", err)\n"+
					"    }\n"+
					"    if err := tx.Commit(ctx); err != nil {\n"+
					"        return fmt.Errorf(\"commit the outbox transaction: %w\", err)\n"+
					"    }\n"+
					"    return nil\n"+
					"}"),
			code("outbox-schema-sql", "sql",
				"The outbox table itself: what to publish, where, and the published_at seam the relay stamps.",
				"server/internal/data/messaging/schema.sql",
				"CREATE TABLE IF NOT EXISTS msgdemo_outbox (\n"+
					"    id           text PRIMARY KEY,\n"+
					"    run_id       text NOT NULL,\n"+
					"    stream       text NOT NULL,\n"+
					"    payload      text NOT NULL,\n"+
					"    created_at   timestamptz NOT NULL DEFAULT now(),\n"+
					"    published_at timestamptz\n"+
					");\n"+
					"\n"+
					"CREATE INDEX IF NOT EXISTS idx_msgdemo_outbox_pending\n"+
					"    ON msgdemo_outbox (run_id, created_at)\n"+
					"    WHERE published_at IS NULL;"),
			text("relay-loop",
				"The relay is a loop any worker can run: claim a bounded batch of pending rows under ``FOR "+
					"UPDATE SKIP LOCKED`` (two concurrent relays divide the rows instead of fighting "+
					"over them — topic 17's locking lesson, applied), publish each to its stream with the "+
					"message id riding as a FIELD (so consumers can dedup), stamp it published, and "+
					"commit. The stamp rides the same transaction as the row locks, which is precisely "+
					"the at-least-once choice from the guarantees section: if the relay dies after "+
					"publishing but before commit, the row is still pending and will publish AGAIN on "+
					"restart — a duplicate, by design, defeated downstream:"),
			code("relay-beat-go", "go",
				"The read side: drain the run's pending rows oldest-first under SKIP LOCKED, XADD each, stamp published — one transaction.",
				"server/internal/data/messaging/outbox.go",
				"    rows, err := tx.Query(ctx, `\n"+
					"        SELECT id, stream, payload FROM msgdemo_outbox\n"+
					"        WHERE published_at IS NULL AND run_id = $2\n"+
					"        ORDER BY created_at, id\n"+
					"        LIMIT $1\n"+
					"        FOR UPDATE SKIP LOCKED`,\n"+
					"        limit, runID)\n"+
					"    // …scan the pending rows, then publish each and stamp it\n"+
					"    for _, row := range pending {\n"+
					"        entryID, err := o.streams.Append(ctx, row.stream, map[string]string{\n"+
					"            \"messageId\": row.id,\n"+
					"            \"payload\":   row.payload,\n"+
					"        })\n"+
					"        if err != nil {\n"+
					"            return messaging.RelayReport{}, err\n"+
					"        }\n"+
					"        if _, err := tx.Exec(ctx, `\n"+
					"            UPDATE msgdemo_outbox SET published_at = now() WHERE id = $1`,\n"+
					"            row.id); err != nil {\n"+
					"            return messaging.RelayReport{}, fmt.Errorf(\"mark outbox row %s published: %w\", row.id, err)\n"+
					"        }"),
			demo("msg:outbox", "The outbox story: atomic write, relay, idempotent consume",
				"One request walks the full pattern across BOTH live stores — PostgreSQL and Redis: two event "+
					"rows committed with their outbox rows (each pair one transaction), a third pair written "+
					"and deliberately ROLLED BACK to prove neither row survived, the relay draining the two "+
					"committed rows onto the stream (and a second pass draining NOTHING), a consumer group "+
					"processing each message exactly once by deduplicating on the message id, a forced "+
					"redelivery of an already-processed message that the dedup seam REFUSES, and the cleanup "+
					"that deletes every row and key the story created."),
			text("outbox-story-read",
				"The demo's strongest sentence is the one it derives from a count query: after the deliberate "+
					"rollback, ``SELECT count(*)`` for the rolled-back event answers ZERO — neither the "+
					"business row nor its outbox row exists, which is the whole pattern proven by absence. "+
					"The relay beat reports both passes (two published, then zero) and the consumer beat "+
					"reports the refused duplicate — one refusal, derived from the dedup key's answer, "+
					"not assumed. As with the stream story, every sentence is derived in the pure builder "+
					"from the stores' own answers; the handler's only other move is the degradation map, "+
					"identical in shape to the redis playground's:"),
			code("outbox-degrade-go", "go",
				"The handler's whole degradation: run the story, map any failure to service_unavailable, write the payload only on success.",
				"server/internal/presentation/demo_messaging.go",
				"    run, err := h.runner.RunOutboxStory(r.Context(), namespace)\n"+
					"    if err != nil {\n"+
					"        core.WriteError(w, r, core.Wrapf(err, core.KindServiceUnavailable,\n"+
					"            \"The messaging outbox demo runs a real transaction against the guide's PostgreSQL and Redis, \"+\n"+
					"                \"which are not both reachable right now.\"), h.logger)\n"+
					"        return\n"+
					"    }\n"+
					"    writeJSON(w, http.StatusOK, demos.BuildMessagingOutboxPayload(run))"),
		),
		// ── 7. Idempotent consumers ─────────────────────────────────
		section("idempotent-consumers", "Idempotent consumers",
			"At-least-once delivery means the second delivery is a WHEN, not an IF — the consumer's dedup on the message id is what makes it a no-op.",
			text("idempotency-pitch",
				"Everything this topic built — the pending ledger, XAUTOCLAIM, the relay's publish-then-mark — "+
					"deliberately risks delivering the same logical message more than once, because the "+
					"alternative is losing messages. So the receiving end must answer one question per "+
					"delivery: \"have I already processed THIS message?\" — and the answer must live "+
					"OUTSIDE the process (a restart wipes memory) and be atomic (a check-then-set of two "+
					"commands races two concurrent deliveries into both answering \"first\"). The guide's "+
					"seam is one command — ``SET NX`` — wrapped in a port whose name says what it means:"),
			code("processonce-beat-go", "go",
				"The dedup seam: one atomic SET NX answers first-time-or-repeat for both deliveries of a message.",
				"server/internal/data/messaging/streams.go",
				"func (s *StreamStore) ProcessOnce(ctx context.Context, id string) (bool, error) {\n"+
					"    first, err := s.client.SetNX(ctx, dedupKey(id), \"processed\", 0).Result()\n"+
					"    if err != nil {\n"+
					"        return false, fmt.Errorf(\"SET NX %s: %w\", dedupKey(id), err)\n"+
					"    }\n"+
					"    return first, nil\n"+
					"}"),
			text("idempotency-what-counts",
				"What you dedup ON is the design decision that makes the seam sound. The demo dedups on the "+
					"OUTBOX id — the app-minted id the producer chose — not on the stream entry id the "+
					"store assigned, because redelivery (relay restart, XAUTOCLAIM) creates a NEW stream "+
					"entry for the same logical message, and a store-assigned id would make every "+
					"redelivery look unique and defeat the whole point. That id rides with the message as "+
					"a field, stamped by the relay at publish time. In production, the dedup marks want a "+
					"TTL longer than your longest redelivery window (the guide passes zero because each "+
					"demo namespace is thrown away whole), and the same reasoning generalizes: an "+
					"idempotent API (topic 11's PUT semantics) and an idempotent consumer are the same "+
					"idea at two boundaries — a stable client-chosen key, and an apply step that is safe "+
					"to repeat."),
			code("redeliver-beat-go", "go",
				"The outbox demo's redelivery beat: the claimed-again entry hits the same seam and the duplicate is counted, then acknowledged.",
				"server/internal/data/messaging/story.go",
				"    for _, entry := range claimed {\n"+
					"        messageID := entry.Values[\"messageId\"]\n"+
					"        first, err := r.store.ProcessOnce(ctx, messageID)\n"+
					"        if err != nil {\n"+
					"            return demos.MessagingOutboxRun{}, err\n"+
					"        }\n"+
					"        if !first {\n"+
					"            duplicates++\n"+
					"        }"),
			text("idempotency-order",
				"Order of operations is the last half of the pattern: PROCESS first, MARK processed, then "+
					"ACKNOWLEDGE. Marking before processing loses messages (the mark says done; the crash "+
					"says otherwise, and the entry never returns). Acknowledging before processing builds "+
					"at-most-once by accident. The outbox story choreographs exactly this order for its "+
					"first consumer — process, dedup-mark, and only the LAST entry's ack is withheld to "+
					"stage the crash — so the redelivery beat exercises the honest path end to end: the "+
					"same message arrives twice, the work happens ONCE, the ledger drains, and the payload "+
					"says all three in derived sentences."),
		),
		// ── 8. Kafka / RabbitMQ ─────────────────────────────────────
		section("kafka-rabbitmq", "Kafka and RabbitMQ from Go",
			"The vocabulary of this topic — groups, offsets, acks, redelivery — transfers wholesale; the differences are the retention model, the ordering scope, and the operational weight.",
			text("kafka-pitch",
				"Kafka is the log model this topic taught, at Internet scale and with real teeth. A cluster "+
					"of brokers holds TOPICS split into PARTITIONS — each partition an append-only log "+
					"with its own ordering, which is exactly the \"one stream per ordered entity\" law "+
					"from earlier, made physical. Consumer groups are first-class: the group's offset "+
					"per partition is the cursor, committed after processing, and rebalancing moves "+
					"partitions between group members when workers come and go — the same pending-ledger "+
					"story with different names (and the same failure mode: a poll loop that commits "+
					"before processing is at-most-once again). Retention is TIME- or SIZE-based rather "+
					"than per-entry ack — read messages stay in the log — so replay is a native "+
					"operation and multiple teams consume one topic independently, each at their own "+
					"pace. From Go, the idiomatic client is ``franz-go`` (``github.com/twmb/franz-go``) — "+
					"context-first, used by many high-volume Go shops — with the older ``sarama`` still "+
					"common in older codebases; both speak the native protocol, no JVM required."),
			text("rabbitmq-pitch",
				"RabbitMQ is the queue model with production manners: an AMQP 0-9-1 (or newer QUIC-based) "+
					"broker where producers PUBLISH to exchanges, routes bind exchanges to queues, and "+
					"consumers take from queues with explicit ACKNOWLEDGMENT — unacked messages return to "+
					"the queue on connection loss, the same at-least-once ledger idea in wire clothing. "+
					"Its distinctive strengths are ROUTING (topic headers, per-binding rules — one "+
					"publish fanning into many queues by pattern), per-message deletion (nothing "+
					"accumulates; no retention tuning), and mature dead-lettering with TTL-based retry "+
					"lanes. Ordering is per-queue but weakened by redelivery and by competing "+
					"consumers — the queue's honesty is work distribution, not log semantics. From Go, "+
					"``amqp091-go`` (``github.com/rabbitmq/amqp091-go``) is the maintained client; the "+
					"codebase discipline is the same as this guide's: one connection, channels per "+
					"goroutine, publishes confirmed, consumes acknowledged manually."),
			list("broker-choices", false,
				"Redis Streams — you already run Redis; per-group cursors, pending ledger, XAUTOCLAIM. Ideal to tens of thousands of msgs/sec; retention by trimming; no cluster story for the log itself.",
				"Kafka — many-partition logs at high sustained throughput; long retention, native replay, ecosystem (Connect, exactly-once pairs). Heaviest to operate; ordering is per-partition only.",
				"RabbitMQ — rich routing and per-message acknowledgment; queues that empty themselves. Great task distribution; replay and multi-reader history are not its shape.",
				"Cloud buses (SQS, Pub/Sub) — the queue or log model as a service; capacity is somebody else's pager, at the price of vendor semantics and egress thinking.",
			),
			text("choosing-honestly",
				"The honest decision procedure is the one this guide models: message on infrastructure you "+
					"ALREADY run, sized to the guarantees you actually need. Redis Streams carried this "+
					"topic because the store was already there, its group semantics teach every concept "+
					"Kafka will ask you about, and the operational budget is zero new services. Graduate "+
					"when a specific pain arrives — sustained throughput beyond one Redis, retention "+
					"measured in days with independent replay, or a contract that multiple teams consume "+
					"— and then graduate to exactly the model whose vocabulary you learned here: a "+
					"log with consumer groups if you need history, a queue with routes if you need "+
					"work distribution. What does NOT change, on any of them, is the pattern you can "+
					"now build blindfolded: transactional outbox on the write side, idempotent "+
					"dedup on the read side, and the three delivery guarantees chosen with open eyes."),
		),
		// ── 9. Terminal companion ───────────────────────────────────
		section("terminal-companion", "Terminal companion: redis-cli streams and the curl tour",
			"The topic from the shell: XADD and XLEN by hand, a group's ledger under XPENDING, both demos by curl, and the streams inspected with XINFO.",
			text("term-pitch",
				"With the stack up (server on :8100, Redis in-network), this tour runs the topic's commands "+
					"by hand — every command below is ``docker exec`` against the running redis "+
					"container, the same store the demos just walked. The two XINFO stops are the best "+
					"teaching moments: ``XINFO GROUPS`` shows the group's cursor and pending count as "+
					"the STORE sees them, and ``XINFO STREAM`` shows the stream's length and first and "+
					"last entry ids — the log's own vital signs:"),
			code("term-xadd", "bash", "", "",
				"# append by hand, then watch the log grow — ids are time-ordered by construction\n"+
					"docker exec go_developers_guide_redis_dev redis-cli XADD tour:orders '*' step created\n"+
					"docker exec go_developers_guide_redis_dev redis-cli XADD tour:orders '*' step paid\n"+
					"docker exec go_developers_guide_redis_dev redis-cli XLEN tour:orders\n"+
					"docker exec go_developers_guide_redis_dev redis-cli XRANGE tour:orders - +"),
			code("term-group", "bash", "", "",
				"# a scratch group at 0-0, one read, then the LEDGER: delivered, un-acked, owner named\n"+
					"docker exec go_developers_guide_redis_dev redis-cli XGROUP CREATE tour:orders tour-workers 0\n"+
					"docker exec go_developers_guide_redis_dev redis-cli XREADGROUP GROUP tour-workers w1 COUNT 10 STREAMS tour:orders >\n"+
					"docker exec go_developers_guide_redis_dev redis-cli XPENDING tour:orders tour-workers\n"+
					"# …no XACK yet: the entries above sit pending under w1 until acknowledged\n"+
					"docker exec go_developers_guide_redis_dev redis-cli XACK tour:orders tour-workers 0-0\n"+
					"docker exec go_developers_guide_redis_dev redis-cli XGROUP DESTROY tour:orders tour-workers"),
			code("term-demos", "bash", "", "",
				"# both stories by curl — the stream lifecycle, then the outbox pattern across both stores\n"+
					"curl -s localhost:8100/api/demo/msg/stream | python3 -m json.tool | head -40\n"+
					"curl -s localhost:8100/api/demo/msg/outbox | python3 -m json.tool | head -40\n"+
					"# …and the no-residue proof: the demos' throwaway namespaces are gone\n"+
					"docker exec go_developers_guide_redis_dev redis-cli KEYS 'gdg:demo:msg:*'"),
			code("term-xinfo", "bash", "", "",
				"# the log's own vital signs — length, first/last ids, radix-tree shape\n"+
					"docker exec go_developers_guide_redis_dev redis-cli XADD tour:vitals '*' hello world\n"+
					"docker exec go_developers_guide_redis_dev redis-cli XINFO STREAM tour:vitals\n"+
					"docker exec go_developers_guide_redis_dev redis-cli DEL tour:orders tour:vitals"),
			text("term-coda",
				"Notice what the tour proves without asking you to trust it: XRANGE hands the entries back "+
					"in insertion order because the log cannot do otherwise; XPENDING shows entries "+
					"you READ but have not ACKNOWLEDGED — the ledger is real state, not a figure of "+
					"speech; the XACK of ``0-0`` acknowledges everything up to that id, the batch form "+
					"production uses; and the KEYS check at the end comes back EMPTY because both demos "+
					"clean up after themselves. That closes the first half of the state-and-auth group: "+
					"topic 21 cached reads, this topic moved work in time. Next, topic 23 starts the "+
					"auth spine — and its sessions will ride cookies whose honesty this topic's "+
					"dedup discipline anticipates: a stable identity, applied idempotently."),
		),
	}

	if buildErr != nil {
		return Topic{}, buildErr
	}
	return NewTopic(
		"messaging",
		"Messaging from Go",
		"Why systems message at all, the log-versus-queue model, consumer groups and ordering walked live "+
			"through Redis Streams (XADD, XREADGROUP, the pending ledger, XAUTOCLAIM recovery), the three "+
			"delivery guarantees argued honestly, the transactional outbox that welds a PostgreSQL commit and "+
			"its event into one all-or-nothing unit across two stores, the idempotent consumer that makes "+
			"at-least-once delivery safe, and where Kafka and RabbitMQ honestly earn their keep — with the "+
			"terminal's redis-cli and curl tour.",
		"state-auth",
		22,
		sections,
	)
}
