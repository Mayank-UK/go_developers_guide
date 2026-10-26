package content

import (
	"errors"
	"fmt"
)

// This file is topic 19 mongodb (group data, curriculum order 19): the
// document way of keeping state — when a document model fits, what the
// optional-service discipline costs and buys, the v2 Go driver,
// validation that lives in the application because the database will not
// do it, queries and projection, indexes and TTL collections, the
// port-and-adapter layering unchanged from the SQL topics, an honest
// SQL-vs-Mongo comparison, and the wider NoSQL survey — one console demo
// running every claim live against the stack's MongoDB.

// mongodbTopic builds topic 19.
func mongodbTopic() (Topic, error) {
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
		// ── 1. Document thinking ───────────────────────────────────
		section("document-thinking", "Document thinking: when a document model fits",
			"A document is a self-describing value tree — and some resources simply ARE trees, with the whole answer in one read.",
			text("doc-pitch",
				"PostgreSQL keeps your data in tables because relations between FLAT rows are its whole "+
					"vocabulary. MongoDB keeps your data in documents — BSON value trees — because some "+
					"resources are shaped like trees in the wild: an event with a tag LIST, a product with "+
					"per-locale name MAPS, a user with nested preferences. The guide's teaching case is an "+
					"event log: deploys, alerts, scaling decisions, backups. Look at the SQL shape first — "+
					"tags want their OWN table plus a join, because a text column cannot hold a set; the "+
					"containment query (\"events tagged prod\") becomes an EXISTS through an edge table. The "+
					"document shape keeps what the domain already knows: tags are ONE field holding the "+
					"whole set, and containment is one filter clause against the array. Nothing about this "+
					"is magic — it is the same normalization decision topic 16 taught, taken in the other "+
					"direction on purpose: EMBED what you always read together, RELATE what lives its own "+
					"life. The event log embeds cleanly because an event is written once, read whole, and "+
					"never joins to anything on its hot path."),
			code("doc-mapping-go", "go",
				"The adapter's private BSON mapping — one struct, struct tags, and the tag array is just a field.",
				"server/internal/data/mongodb/events.go",
				"type eventDocument struct {\n"+
					"    ID         bson.ObjectID `bson:\"_id,omitempty\"`\n"+
					"    Key        string        `bson:\"evtKey\"`\n"+
					"    Type       string        `bson:\"type\"`\n"+
					"    Severity   string        `bson:\"severity\"`\n"+
					"    Source     string        `bson:\"source\"`\n"+
					"    Message    string        `bson:\"message\"`\n"+
					"    Tags       []string      `bson:\"tags,omitempty\"`\n"+
					"    OccurredAt time.Time     `bson:\"occurredAt\"`\n"+
					"}"),
			text("doc-flexibility",
				"The second half of document thinking is honest about FLEXIBILITY, because it is the "+
					"half that bites. A collection has no schema the server will enforce: documents in "+
					"the same collection can differ in fields entirely, and the database will not stop "+
					"you. That freedom is real value while a shape is EXPLORING — new field, no "+
					"migration, old documents simply lack it — and real risk once a shape is LOAD-"+
					"BEARING, because nothing but discipline keeps ten writers agreeing on what an "+
					"event looks like. The guide's answer is the same boundary-first stance every "+
					"topic keeps: the APPLICATION owns the schema. The document struct above is the "+
					"contract; validation (next sections) is the enforcement; the store is a place to "+
					"keep agreed shapes, not a place to discover them. Schema-on-write versus "+
					"schema-on-read is the textbook framing — Go structs with struct tags ARE a "+
					"schema-on-write discipline, just enforced in your process instead of the "+
					"database's."),
			text("doc-when",
				"So when does the document model FIT? The guide's own checklist, earned across the "+
					"family guides: the aggregate is read and written WHOLE (an event is exactly "+
					"this); the shape is nested or set-valued in ways SQL must shred into tables; "+
					"writers are few enough that one shared contract is realistic; and the access "+
					"pattern is lookup-by-key or filtered scans, not arbitrary joins across many "+
					"shapes. When any of those inverts — many-shaped ad-hoc joins, strong multi-row "+
					"invariants, reporting over normalized data — the relational camp wins, and the "+
					"SQL-vs-Mongo section says so plainly. Both engines are in this guide's stack on "+
					"purpose; the engineering is choosing per problem, not pledging per tribe."),
		),
		// ── 2. The optional-service discipline ─────────────────────
		section("optional-service-discipline", "The optional-service discipline: Mongo down, app up",
			"MongoDB is an optional service in this guide: when it is unreachable, everything else keeps working and its endpoints answer the typed 503 — never a crash.",
			text("optional-pitch",
				"Topic 17 made PostgreSQL a REQUIRED service: the guide's system of record, wired at "+
					"boot with fail-fast — no database, no start, because every surface needs it. "+
					"MongoDB earns the opposite treatment, and the distinction is an architecture "+
					"decision, not a mood: an optional service is one the product survives WITHOUT. "+
					"The demo endpoints that use Mongo become honestly unavailable; every other "+
					"topic, route, and page keeps serving. That posture has three moving parts, and "+
					"the whole guide's error taxonomy exists to make the third one cheap: connect "+
					"with a bounded timeout, map EVERY driver failure to the typed "+
					"``service_unavailable`` error, and wire a fallback adapter that keeps serving "+
					"that same typed error when the database was already down at boot. A crashed "+
					"process serves nobody; a degraded process tells the truth."),
			code("optional-connect-go", "go",
				"The dialer: prove with a ping, refuse with the typed 503 — the boot never panics over Mongo.",
				"server/internal/data/mongodb/client.go",
				"func Connect(ctx context.Context, uri string) (*mongo.Client, error) {\n"+
					"    client, err := mongo.Connect(options.Client().ApplyURI(uri).SetConnectTimeout(connectTimeout))\n"+
					"    if err != nil {\n"+
					"        return nil, unavailable(err)\n"+
					"    }\n"+
					"    pingCtx, cancel := context.WithTimeout(ctx, connectTimeout)\n"+
					"    defer cancel()\n"+
					"    if err := client.Ping(pingCtx, readpref.Primary()); err != nil {\n"+
					"        _ = client.Disconnect(context.Background())\n"+
					"        return nil, unavailable(err)\n"+
					"    }\n"+
					"    return client, nil\n"+
					"}"),
			code("optional-unavailable-go", "go",
				"The one failure mapping the whole package applies — typed, cause-carrying, handler-ready.",
				"server/internal/data/mongodb/client.go",
				"func unavailable(cause error) error {\n"+
					"    return core.Wrap(cause, core.KindServiceUnavailable, \"MongoDB is not reachable right now.\")\n"+
					"}"),
			text("optional-fallback",
				"The fallback is where the posture becomes visible in code. When ``Connect`` fails at "+
					"boot, the composition root does not exit — it logs the failure and wires the "+
					"demo endpoint over an ``UnavailableConsoleRunner``: a same-port adapter whose "+
					"every call returns the typed error carrying the boot-time cause. The handler "+
					"cannot tell the difference; the central error writer turns either path into "+
					"the one family envelope — ``{\"error\":{\"code\":\"service_unavailable\",…}}`` "+
					"with status 503 and a request id. The tests prove the round trip from BOTH "+
					"sides: a constructed bad URI refused at ``Connect``, and a handler whose "+
					"runner fails serving the honest envelope through the real middleware chain. "+
					"This pattern — optional service, typed degradation, fallback behind the same "+
					"port — is one of the most portable ideas in the guide: it is exactly how "+
					"topic 20 treats Redis and how any third-party dependency in topic 42 should "+
					"be treated."),
			code("optional-fallback-go", "go",
				"The never-connected fallback: same port, typed refusal, process still up.",
				"server/internal/data/mongodb/console.go",
				"func (r *UnavailableConsoleRunner) RunConsole(context.Context) (demos.MongoConsoleObservation, error) {\n"+
					"    return demos.MongoConsoleObservation{}, unavailable(r.cause)\n"+
					"}"),
		),
		// ── 3. The Go driver ───────────────────────────────────────
		section("the-go-driver", "The Go driver: mongo-driver v2",
			"v2 is the current driver line — import go.mongodb.org/mongo-driver/v2 — and v1 is frozen: new code takes v2.",
			text("driver-v2",
				"MongoDB's official Go driver is ``go.mongodb.org/mongo-driver`` — and since 2024 "+
					"the V2 line, imported as ``go.mongodb.org/mongo-driver/v2/mongo``, is where "+
					"development happens; v1 is frozen at maintenance. The guide pins v2.9.2 and "+
					"never imports the v1 path — a rule worth stating because the internet is full "+
					"of v1 snippets, and mixing the two in one module is a confusing day of your "+
					"life. The v2 changes you FEEL: the old ``primitive`` package folded into "+
					"``bson`` (``bson.ObjectID``, ``bson.NewObjectID`` — the types you will touch "+
					"most); ``mongo.Connect`` takes options directly and returns the client pair; "+
					"and options are builders — ``options.Client().ApplyURI(uri)``, "+
					"``options.Find().SetSort(…)``, chained, typed, discoverable. The mental "+
					"model that survives every version: a ``Client`` owns the topology, "+
					"``Database`` and ``Collection`` are lightweight handles (no I/O to make "+
					"them), and every operation takes ``context.Context`` first — the same "+
					"cancellation contract the whole guide already lives by."),
			code("driver-pin-mod", "mod",
				"The pin — one line in go.mod, the v2 import path, the current line.",
				"server/go.mod",
				"go.mongodb.org/mongo-driver/v2 v2.9.2"),
			text("driver-ordered",
				"Two BSON document types matter more than everything else combined: ``bson.D`` — "+
					"an ORDERED document, key-value pairs in sequence — and ``bson.M`` — an "+
					"unordered map. Filters and sort specifications are ORDER-SENSITIVE "+
					"artifacts: ``{type: 1, occurredAt: -1}`` is a different index than "+
					"``{occurredAt: -1, type: 1}``, so the guide builds every filter, sort, and "+
					"key pattern with ``bson.D`` and reaches for ``bson.M`` almost never. "+
					"Documents themselves ride Go structs with ``bson`` struct tags — the same "+
					"discipline as ``json`` tags, one mapping per adapter, private to the data "+
					"layer. The stack ships MongoDB 8 with no credentials (dev-compose "+
					"posture — production turns auth on, the same slide the deployment topic "+
					"makes for PostgreSQL), reachable in-network at ``mongo:27017`` and from "+
					"the host at port 27018."),
			code("driver-compose-yaml", "yaml",
				"The stack's own MongoDB — one service in the compose file the whole guide runs on.",
				".docker/docker-compose.yml",
				"  mongo:\n"+
					"    image: mongo:8\n"+
					"    container_name: go_developers_guide_mongo_dev\n"+
					"    ports:\n"+
					"      - 27018:27017\n"+
					"    volumes:\n"+
					"      - gdg-mongo-data:/data/db\n"+
					"    healthcheck:\n"+
					"      test: [\"CMD\", \"mongosh\", \"--quiet\", \"--eval\", \"db.runCommand({ ping: 1 }).ok\"]"),
		),
		// ── 4. Validation lives in the application ─────────────────
		section("validation-in-the-app", "Validation lives in the application layer",
			"The document store will not judge your data — no NOT NULL, no CHECK, no enums — so the domain does, before the driver is ever consulted.",
			text("validation-pitch",
				"PostgreSQL quietly validated every write topics 16 and 17 made: ``NOT NULL`` "+
					"refused the missing, ``CHECK`` refused the absurd, the unique index refused "+
					"the duplicate. A MongoDB collection enforces exactly one of those things "+
					"uniqueness, via an index — and none of the others. That is not a flaw; it is "+
					"the trade the flexibility bought. But it MOVES the work: every rule the "+
					"database no longer runs is now yours, and if you do not write it, your "+
					"collection drifts into shapes no reader can trust. The guide's events domain "+
					"carries the full boundary the posts domain taught — closed vocabularies for "+
					"``type`` and ``severity``, a kebab-case grammar for keys, length bounds, tag "+
					"counting after normalization — and collects every issue into one typed 422, "+
					"exactly as the request-handling topics established:"),
			code("validation-switch-go", "go",
				"Closed vocabulary, judged in the domain — the CHECK constraint the store will not run.",
				"server/internal/domain/events/event.go",
				"switch d.Type {\n"+
					"case TypeDeploy, TypeAlert, TypeScaling, TypeBackup:\n"+
					"default:\n"+
					"    add(\"type\", fmt.Sprintf(\"one of %q, %q, %q, or %q\", TypeDeploy, TypeAlert, TypeScaling, TypeBackup), renderGot(d.Type))\n"+
					"}"),
			text("validation-gate",
				"The gate is the use case — the same RecordEvent shape the console demo runs and "+
					"the tests pin. Normalize the draft (trim, lowercase vocabularies, dedupe "+
					"tags), validate (ALL issues, one pass), and only then consult the store. "+
					"The zero-IO claim is not a hope; the events package's test wires the use "+
					"case over a store whose every method fails the test the moment it is "+
					"touched — an invalid draft provably never reaches the driver, and the "+
					"console demo replays that proof live: its refusal beat shows the attempted "+
					"event, every rule it broke, and the note that MongoDB never saw it. Where "+
					"the rule genuinely belongs to the SHAPE of storage rather than the meaning "+
					"of the domain — uniqueness — it rides the one enforcement the store does "+
					"offer: the unique index the adapter ensures at construction."),
			code("validation-usecase-go", "go",
				"The gate: judge first, store second — the driver is never consulted on the error path.",
				"server/internal/domain/events/usecases.go",
				"func (uc *RecordEventUseCase) Execute(ctx context.Context, draft Draft) (Event, error) {\n"+
					"    normalized, err := validNormalizedDraft(draft)\n"+
					"    if err != nil {\n"+
					"        return Event{}, err\n"+
					"    }\n"+
					"    return uc.store.Upsert(ctx, storeDraft(normalized))\n"+
					"}"),
		),
		// ── 5. Queries and projection ──────────────────────────────
		section("queries-and-projection", "Queries, projection, and the console",
			"Filter, sort, limit, containment, count — and projection, the document store's answer to SELECT-ing some columns — all live in the demo.",
			text("queries-pitch",
				"Reading MongoDB from Go is building the same documents you would type in mongosh "+
					"and handing them to ``Find``. The adapter's read path assembles a filter "+
					"from the NORMALIZED domain query — type equality when set, tag membership "+
					"when set — and the driver turns the array field match into containment "+
					"without a join in sight. Sort rides the same ``bson.D`` grammar (``-1`` is "+
					"descending), and the tie-break habit from the SQL topics comes along: sort "+
					"by ``occurredAt`` DESC then ``evtKey`` ASC, so equal instants never leave "+
					"the order to chance — the property every deterministic test in this "+
					"package leans on:"),
			code("queries-filter-go", "go",
				"The filter builder: validated domain values become an ordered BSON document.",
				"server/internal/data/mongodb/events.go",
				"func filterFor(query events.Query) bson.D {\n"+
					"    filter := bson.D{}\n"+
					"    if query.Type != \"\" {\n"+
					"        filter = append(filter, bson.E{Key: \"type\", Value: query.Type})\n"+
					"    }\n"+
					"    if query.Tag != \"\" {\n"+
					"        filter = append(filter, bson.E{Key: \"tags\", Value: query.Tag})\n"+
					"    }\n"+
					"    return filter\n"+
					"}"),
			code("queries-find-go", "go",
				"The read: sort and limit as builder options, the cursor decoded into domain events.",
				"server/internal/data/mongodb/events.go",
				"cursor, err := s.collection.Find(\n"+
					"    ctx,\n"+
					"    filterFor(query),\n"+
					"    options.Find().\n"+
					"        SetSort(bson.D{{Key: \"occurredAt\", Value: -1}, {Key: \"evtKey\", Value: 1}}).\n"+
					"        SetLimit(int64(query.Limit)),\n"+
					")"),
			text("queries-projection",
				"Projection is the beat that surprises SQL people least and everyone else most: "+
					"``find`` takes a SECOND document saying which fields come back. The demo's "+
					"projection beat asks for three fields and excludes ``_id`` explicitly — "+
					"``_id`` returns by DEFAULT, so a true partial read says so in the "+
					"projection, and the documents that cross the wire are visibly smaller: "+
					"three fields, no object ids, no messages. At demo scale it is a parlor "+
					"trick; at real scale it is the difference between shipping 40 bytes and "+
					"4 kilobytes per document on a hot list path — the same covering-argument "+
					"topic 17 made for index-only scans, wearing document clothes. Runs of "+
					"the whole script land in the demo below: every command shown is the one "+
					"that actually ran, and every document shown is what actually came back."),
			code("queries-projection-go", "go",
				"The projection beat: a whitelist of fields, _id excluded — the partial read.",
				"server/internal/data/mongodb/console.go",
				"cursor, err := r.store.collection.Find(\n"+
					"    ctx,\n"+
					"    bson.D{{Key: \"type\", Value: events.TypeDeploy}},\n"+
					"    options.Find().\n"+
					"        SetProjection(bson.D{\n"+
					"            {Key: \"evtKey\", Value: 1}, {Key: \"severity\", Value: 1}, {Key: \"occurredAt\", Value: 1},\n"+
					"            {Key: \"_id\", Value: 0},\n"+
					"        }).\n"+
					"        SetSort(bson.D{{Key: \"occurredAt\", Value: -1}}),\n"+
					")"),
			demo("db:mongo-console", "The mongo console",
				"One scripted run against the live MongoDB: the idempotent keyed seed (re-runs "+
					"match, never duplicate), the application-layer refusal (an invalid event "+
					"rejected with zero I/O), the query beats — deploy filter with sort and "+
					"limit, prod-tag containment, count, and the three-field projection — the "+
					"explain pair (COLLSCAN where no index exists, IXSCAN on the hinted "+
					"compound index), the TTL index's own spec as listIndexes states it, and "+
					"the BSON type survey of one stored document (objectId, date, array)."),
			text("queries-read",
				"Read the demo's rows against the sections behind it: the seed's ``upserted`` "+
					"count is 8 on the first run and 0 forever after — idempotency you can "+
					"watch; the refusal beat names every rule the broken event violated while "+
					"MongoDB never saw it; the projection rows carry exactly three fields. The "+
					"explain pair deserves its last sentence: the COLLSCAN row is the planner "+
					"being HONEST (there is no index on severity, so it reads every document), "+
					"and the IXSCAN row is HINTED — on eight documents the planner may "+
					"legitimately prefer a scan, exactly as PostgreSQL's planner preferred a "+
					"Seq Scan on six posts in topic 17, so the hint pins the plan the lesson "+
					"is about instead of gambling on selectivity estimates. At scale you drop "+
					"the hint and let the planner earn its keep."),
		),
		// ── 6. Indexes and TTL collections ──────────────────────────
		section("indexes-and-ttl", "Indexes and TTL collections",
			"Indexes are how a document store stays a database: the unique key upserts address, the compound index serves the read path, and a TTL index is retention as a first-class concept.",
			text("indexes-pitch",
				"Every index in the guide's events collection is a DECISION, ensured "+
					"idempotently at adapter construction — ``CreateOne`` calls that are no-ops "+
					"when the identical index already exists, safe at every boot, loud (names "+
					"pinned) when options drift. Three decisions, three jobs: the UNIQUE index "+
					"on ``evtKey`` is the business key — the address every upsert targets, and "+
					"the only constraint MongoDB will enforce for you (a duplicate insert is "+
					"refused by the SERVER, which the adapter tests prove with a real "+
					"duplicate-key error); the COMPOUND ``{type: 1, occurredAt: -1}`` serves "+
					"the default read — type equality plus newest-first — as one btree walk; "+
					"and the TTL index on ``occurredAt`` is retention policy declared as data:"),
			code("indexes-ensure-go", "go",
				"Three indexes, ensured at construction: the unique key, the query path, the TTL window.",
				"server/internal/data/mongodb/events.go",
				"{ttlIndexName, mongo.IndexModel{\n"+
					"    Keys:    bson.D{{Key: \"occurredAt\", Value: 1}},\n"+
					"    Options: options.Index().SetName(ttlIndexName).SetExpireAfterSeconds(TTLSeconds),\n"+
					"}},"),
			text("indexes-ttl",
				"A TTL index says: when ``now`` minus the indexed date exceeds "+
					"``expireAfterSeconds``, the document dies — deleted by a background "+
					"sweeper MongoDB runs roughly every sixty seconds. That is the entire "+
					"mechanism, and it replaces a whole class of cron jobs: session stores, "+
					"rate-limit windows, verification codes, event retention. The guide's "+
					"window is deliberately TEN YEARS — long enough that nothing vanishes "+
					"mid-lesson; the demo asserts and shows the SPEC (the key pattern and the "+
					"window exactly as ``listIndexes`` states them), not an eviction. The "+
					"production habits worth carrying: choose the window from a retention "+
					"requirement, not a vibe; know the sweeper's cadence means expiry is "+
					"eventual, not exact (a TTL'd document can outlive its deadline by up to "+
					"about a minute); and remember the indexed field must be a BSON DATE for "+
					"the fixed-window form — the event's ``occurredAt`` is exactly that. Two "+
					"finer points ride the same idea: a per-document expiry uses a date FIELD "+
					"named in the index with ``expireAfterSeconds: 0`` (each document states "+
					"its own death), and a TTL index is a normal index too — the query path "+
					"can use it even where nothing expires."),
			list("indexes-laws", false,
				"Ensure at construction — indexes are schema, created idempotently at boot, never assumed to exist.",
				"Pin the names — an index that drifts its options should fail loudly, not silently twin itself.",
				"Index the read path, not the write — every index taxes every insert; the compound key mirrors the actual query shape.",
				"Unique is the one enforced constraint — the business key belongs behind it, and the server does the refusing.",
				"TTL is retention as data — a date field, a window, a sweeper; no cron, no cleanup endpoint."),
		),
		// ── 7. Layering: the port and its adapter ───────────────────
		section("layering-port-and-adapter", "Layering: the same port, a new adapter",
			"The dependency arrow does not care which database sits at the end of it — the events domain declares a port, the data layer implements it, the handler never learns the driver's name.",
			text("layering-pitch",
				"Everything the request-handling and architecture topics established about ports "+
					"and adapters arrives here UNCHANGED, and that is the lesson: the domain "+
					"declares ``events.Store`` — upsert keyed by business key, recent by "+
					"normalized query, count — with ``context.Context`` first and typed errors "+
					"out. The MongoDB adapter satisfies it; so would a PostgreSQL adapter, an "+
					"in-memory fake, or a file. The handler holds the PORT; the composition "+
					"root chooses the implementation; the driver's types (``bson.D``, "+
					"``mongo.Client``, the document struct) never cross out of "+
					"``internal/data/mongodb`` — the mapping is the adapter's private "+
					"business, the DTOs-never-leak rule from topic 11 wearing a new database. "+
					"The compile-time proof sits in the adapter (``var _ events.Store = "+
					"(*EventStore)(nil)``) so the contract is checked by the compiler, not by "+
					"review:"),
			code("layering-port-go", "go",
				"The port, whole: three methods, context first, typed errors out.",
				"server/internal/domain/events/repository.go",
				"type Store interface {\n"+
					"    // Upsert writes the event keyed by Key and returns the stored form —\n"+
					"    // including the store-assigned ID, exactly as the database states it.\n"+
					"    Upsert(ctx context.Context, event Event) (Event, error)\n"+
					"    // Recent returns events matching the query, newest first, capped at\n"+
					"    // the query's limit. The result is never nil: no match renders [].\n"+
					"    Recent(ctx context.Context, query Query) ([]Event, error)\n"+
					"    // Count returns how many events match the query's filters.\n"+
					"    Count(ctx context.Context, query Query) (int64, error)\n"+
					"}"),
			code("layering-adapter-go", "go",
				"The adapter's constructor: one handle, indexes ensured, typed refusal when the store is down.",
				"server/internal/data/mongodb/events.go",
				"func NewEventStore(ctx context.Context, client *mongo.Client, database string) (*EventStore, error) {\n"+
					"    collection := client.Database(database).Collection(EventsCollection)\n"+
					"    if err := ensureEventIndexes(ctx, collection); err != nil {\n"+
					"        return nil, err\n"+
					"    }\n"+
					"    return &EventStore{collection: collection}, nil\n"+
					"}"),
			text("layering-upsert",
				"The adapter's one interesting WRITE is the idempotent upsert. ``ReplaceOne`` "+
					"with ``SetUpsert(true)`` against the unique ``evtKey`` means: replace the "+
					"document at this key, insert it if absent — the at-least-once-world's "+
					"favorite verb, and the reason the demo's seed can replay forever. The "+
					"replacement deliberately carries NO ``_id``: ``_id`` is immutable, so a "+
					"replacement that set one could never update an existing document — the "+
					"server assigns it once on the insert path, and every later replacement "+
					"leaves it alone. Then the adapter READS BACK, the same "+
					"return-what-the-database-states discipline the pgx adapter's "+
					"``RETURNING`` clauses keep — the caller receives the stored form, id "+
					"and all, never an assumption:"),
			code("layering-upsert-go", "go",
				"Idempotent by construction: replace-or-insert at the business key, then read back.",
				"server/internal/data/mongodb/events.go",
				"if _, err := s.collection.ReplaceOne(\n"+
					"    ctx,\n"+
					"    bson.D{{Key: \"evtKey\", Value: event.Key}},\n"+
					"    eventDocument{\n"+
					"        Key:        event.Key,\n"+
					"        // …the remaining fields ride the same replacement document\n"+
					"        OccurredAt: event.OccurredAt,\n"+
					"    },\n"+
					"    options.Replace().SetUpsert(true),\n"+
					"); err != nil {\n"+
					"    return events.Event{}, unavailable(err)\n"+
					"}\n"+
					"return s.FindByKey(ctx, event.Key)"),
		),
		// ── 8. SQL vs Mongo, honestly ───────────────────────────────
		section("sql-vs-mongo", "SQL vs Mongo, honestly",
			"The guide runs both on purpose — the comparison is engineering, not identity.",
			text("sql-stance",
				"PostgreSQL is this guide's system of record and Mongo is a tool it reaches "+
					"for when shapes say so — but the HONEST comparison starts by retiring "+
					"the 2009-era caricatures. Modern MongoDB HAS multi-document ACID "+
					"transactions (a session, a callback, real isolation); modern PostgreSQL "+
					"HAS JSONB (indexable documents inside a relational row). What actually "+
					"differs are the defaults and the center of gravity: PostgreSQL's "+
					"defaults push you toward normalized data, server-enforced integrity, "+
					"and the join-rich query language that reporting lives on; MongoDB's "+
					"push you toward whole-aggregate documents, denormalization without "+
					"guilt, and horizontal sharding baked in from day one. Transactions "+
					"EXIST in Mongo but they are the exception you design AROUND (keep the "+
					"aggregate in one document and you rarely need one); JSONB EXISTS in "+
					"PostgreSQL but a document-laden row is still a row, and the type system "+
					"around it stays relational."),
			list("sql-vs-mongo-calls", false,
				"Strong invariants across many rows — relational: constraints, FKs, and CHECKs are enforced by the engine that keeps them honest.",
				"Arbitrary joins and reporting — relational: the query language is the product; documents approximate joins with $lookup and pay for it.",
				"One aggregate, read and written whole — document: one find, one write, no assembly; the event log is the shape case.",
				"Set-valued and nested fields — document: arrays and subdocuments are native; SQL shreds them into tables to keep its own rules.",
				"Shape still exploring — document: add a field, no migration; pay the discipline tax in the application layer instead.",
				"Retention as policy — document (TTL): a date and a window replace a cleanup job; PostgreSQL reaches the same end with partition drops."),
			text("sql-verdict",
				"The verdict this guide acts on: START relational when the domain is a web "+
					"product's core — resources, relations, money — because the engine's "+
					"constraints are teammates you will miss the day an invoice goes wrong. "+
					"Reach for documents when the shape is aggregate-shaped by NATURE "+
					"(events, content, per-user blobs), when writes are append-mostly and "+
					"reads are by key or filtered scan, and when the operational story — "+
					"scales-out sharding, TTL retention, flexible fields — matches the "+
					"workload's grain. And notice what the choice does NOT change: the port "+
					"sits in the domain either way, the adapter swaps behind it, the handler "+
					"and the error envelope never learn which store answered. The decision "+
					"is which ADAPTER you wire at the composition root — reversible by "+
					"construction, which is the only honest way to hold any database "+
					"opinion."),
		),
		// ── 9. The NoSQL survey ─────────────────────────────────────
		section("nosql-survey", "The wider NoSQL survey",
			"Document stores are one camp in a wider movement — one paragraph each for the families you will actually meet.",
			text("survey-kv",
				"Key-value stores — Redis is the family's famous member, and topic 20 builds "+
					"on it properly — treat the database as a giant typed map: GET a key, SET "+
					"a key, expire a key. The value's MEANING lives entirely in the "+
					"application; the store's promises are speed, atomic operations on single "+
					"keys, and (for the memory-native designs) optional persistence that "+
					"trades durability nuance for latency. Caches, sessions, rate counters, "+
					"leaderboards, job queues: state that is looked up by exact key and "+
					"wanted NOW. Redis layers useful structures (lists, sets, sorted sets, "+
					"streams) on the same idea, which is why it keeps escaping the "+
					"\"just a cache\" box."),
			text("survey-wide",
				"Wide-column stores — Cassandra and its derivatives like ScyllaDB, and "+
					"cloud kin like DynamoDB and Bigtable — model rows keyed and clustered "+
					"for WRITE throughput across many machines. The design coin is the "+
					"PARTITION KEY: choose it around your queries, because a query that does "+
					"not know its partition is a scan across the ring. There is no JOIN, "+
					"schema is flexible-but-tuned, and consistency is tunable per write — "+
					"the store of record for telemetry, messaging metadata, and "+
					"write-heavy time series at scales where a single relational primary "+
					"would fall over. The engineering is query-first data modeling, the "+
					"exact OPPOSITE of the normalize-then-query reflex."),
			text("survey-graph",
				"Graph databases — Neo4j is the reference point — make the RELATIONSHIP the "+
					"first-class citizen: nodes and typed edges stored and traversed natively, "+
					"so \"friends of friends who bought this\" is a two-hop walk instead of a "+
					"self-joining SQL query the planner hates. The moment your domain's "+
					"questions are about PATHS and connectivity — social graphs, fraud "+
					"rings, dependency trees, knowledge graphs — the model stops being an "+
					"exotic choice and becomes the natural one. PostgreSQL's recursive CTEs "+
					"(topic 17's vocabulary) can walk trees; they do not enjoy walking "+
					"neighborhoods."),
			text("survey-search",
				"Search engines — Elasticsearch and the OpenSearch lineage — are document "+
					"stores inverted for RELEVANCE: an index of analyzed terms, scoring "+
					"engines, and query shapes shaped by linguistics (stemming, synonyms, "+
					"fuzzy matching) rather than exact equality. They answer \"best "+
					"matches for these words\" over millions of documents in "+
					"milliseconds, and they are almost always a SECOND store fed FROM "+
					"your system of record — the same data, re-shaped for a different "+
					"question. PostgreSQL's tsvector (topic 17 built it) covers the small "+
					"end of this spectrum remarkably well; the dedicated engine earns its "+
					"keep at scale, under faceting, and once ranking becomes the product."),
			text("survey-verdict",
				"The survey's takeaway is not a scoreboard; it is a recognition reflex. Each "+
					"family is an answer to a specific access pattern: key-value for exact "+
					"key at lowest latency, wide-column for write-heavy scale-out by "+
					"partition, graph for paths, search for ranked text, document for "+
					"whole aggregates with flexible shape. The engineering question is "+
					"never \"which database is best\" — it is \"which ACCESS PATTERN "+
					"dominates this problem\", asked per bounded context, with the port-"+
					"and-adapter layering keeping every answer swappable when the pattern "+
					"changes its mind."),
		),
		// ── 10. Terminal companion ──────────────────────────────────
		section("terminal-companion", "Terminal companion: the curl and mongosh tour",
			"The topic from the shell: the console endpoint by curl, then mongosh inside the stack's own MongoDB, then the optional-service discipline proven live.",
			text("term-pitch",
				"With the stack up (server on :8100, MongoDB in-network at ``mongo:27017``, "+
					"host port 27018), this tour runs the console demo from the shell, opens "+
					"``mongosh`` INSIDE the container to poke the same collection by hand — "+
					"the demo's database is ``guide_demos`` — and finishes with the "+
					"headline: stop MongoDB, watch the endpoint answer the typed 503 while "+
					"every other route keeps serving, start it again. The last block is the "+
					"optional-service discipline as a shell experiment; run it whenever you "+
					"want proof the process never crashed:"),
			code("term-console", "bash", "", "",
				"# the whole scripted console, straight from the shell\n"+
					"curl -s localhost:8100/api/demo/db/mongo-console | python3 -m json.tool | head -30"),
			code("term-mongosh", "bash", "", "",
				"# mongosh inside the stack's MongoDB container (the image ships it)\n"+
					"docker exec -it go_developers_guide_mongo_dev mongosh guide_demos\n"+
					"\n"+
					"# inside mongosh: the demo's own collection, by hand\n"+
					"#   db.events.countDocuments({})\n"+
					"#   db.events.find({type: \"deploy\"}).sort({occurredAt: -1}).limit(2)\n"+
					"#   db.events.find({tags: \"prod\"}).limit(3)\n"+
					"#   db.events.getIndexes()\n"+
					"#   db.events.find({severity: \"critical\"}).explain(\"executionStats\").executionStats"),
			code("term-degrade", "bash", "", "",
				"# the optional-service discipline, proven live: Mongo down, app up\n"+
					"docker stop go_developers_guide_mongo_dev\n"+
					"curl -s -o /dev/null -w '%{http_code}\\n' localhost:8100/api/demo/db/mongo-console\n"+
					"#   503 — and the envelope says so, typed:\n"+
					"curl -s localhost:8100/api/demo/db/mongo-console | python3 -m json.tool\n"+
					"curl -s -o /dev/null -w 'content still serves: %{http_code}\\n' localhost:8100/api/content\n"+
					"docker start go_developers_guide_mongo_dev   # and the 503 becomes 200 again"),
			text("term-coda",
				"Notice the symmetry with the tours before it: every claim in this topic ran "+
					"somewhere you can re-run — the payload is the driver's own answers, "+
					"mongosh shows the same collection the console touched, and the "+
					"degradation experiment is the optional-service discipline executed "+
					"live. The next topic keeps the exact same optional posture for a "+
					"completely different animal: Redis, the in-memory data structure "+
					"store — key-value at its core, the same typed degradation when it "+
					"is down, and the same port-and-adapter spine underneath."),
		),
	}

	if buildErr != nil {
		return Topic{}, buildErr
	}
	return NewTopic(
		"mongodb",
		"MongoDB from Go",
		"Document thinking and when it fits, the optional-service discipline (Mongo down → "+
			"typed 503, never a crash), mongo-driver v2, validation that lives in the application "+
			"layer, queries with filter/sort/limit and projection, indexes and TTL collections, "+
			"the port-and-adapter layering unchanged, SQL vs Mongo honestly, and the wider NoSQL "+
			"survey — one console demo running every beat live against the stack's MongoDB.",
		"data",
		19,
		sections,
	)
}
