package content

import (
	"errors"
	"fmt"
)

// This file is topic 16 sql-schema (group data, curriculum order 16), the
// first topic of the data phase: the guide stops being an API over memory
// and becomes an API over a real PostgreSQL. Everything taught here is
// REAL in this repo — the migrations are the server's own (applied at
// boot), the schema-explorer demo introspects the live catalog, and the
// UUID lab runs the guide's own hand-rolled UUIDv7 generator. Topic 17
// swaps the in-memory posts repository for a pgx one over exactly this
// schema; nothing in this topic is scaffolding.

// sqlSchemaTopic builds topic 16 — designing the relational schema that
// carries the guide's own data: relational thinking, PostgreSQL types and
// constraints, keys and foreign keys with explicit ON DELETE behavior,
// the normalization call this guide makes for tags (and when to reverse
// it), indexes as per-query decisions, goose's migration history embedded
// in the binary, UUIDv7 vs sequential ids, and the audit-logging pattern.
func sqlSchemaTopic() (Topic, error) {
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
		// ── 1. Relational thinking ───────────────────────────────────
		section("relational-thinking", "Relational thinking",
			"A relational schema is a set of FACTS about things, each fact stated once — not a dump of the application's structs.",
			text("rel-pitch",
				"Everything in the data phase rests on one mental model, so start with it. A relational database holds "+
					"RELATIONS — named, typed tables whose rows are facts of the same shape: every row of ``posts`` is one "+
					"post, with the same columns, in no guaranteed order. The orderlessness is not a quirk to fight; it is "+
					"the point. A table is a SET of rows, and sets have no order until a query imposes one — which is why "+
					"``ORDER BY`` exists and why any code that relies on insertion order in PostgreSQL is already wrong. "+
					"What a schema designer does is decide which FACTS exist and where each one lives, so that every fact "+
					"is stated exactly once and everything else can be derived."),
			text("rel-once",
				"The \"stated once\" discipline is called normalization, and its payoff is that contradictions become "+
					"impossible. If a post's title lived in both the ``posts`` table and a ``recent_posts`` table, the two "+
					"could drift apart the moment an update touched only one. If instead ``recent_posts`` DERIVES from "+
					"``posts`` (a query, a view, or a carefully maintained cache that admits it is a cache), there is one "+
					"truth and copies that know they are copies. This guide's domain layer has been quietly rehearsing "+
					"this: the in-memory ``Post`` entity is one fact about a post; the wire response is a derived "+
					"representation of it. PostgreSQL makes the same distinction with tables and query results."),
			text("rel-null",
				"Two pieces of vocabulary carry most of the weight in schema conversations. ``NULL`` is not the empty "+
					"string, not zero, and not \"no value yet\" — it is the SQL marker for \"this fact does not apply to "+
					"this row\", and it demands an explicit decision on every column you declare: does this attribute make "+
					"sense for every possible row? A post always has a creation instant, so ``created_at`` is NOT NULL; "+
					"an audit row may outlive the post it describes, so its ``post_id`` is nullable on purpose. And a "+
					"KEY is the column (or columns) whose value identifies a row within the set — the hook every other "+
					"fact about that row hangs from. The rest of this topic is those two ideas, applied with Postgres"+
					"SQL's tools."),
		),
		// ── 2. PostgreSQL types and constraints ──────────────────────
		section("types-and-constraints", "PostgreSQL types and constraints",
			"Types say what a value IS; constraints say what the database refuses — both are load-bearing, and both are the guide's own migration SQL.",
			text("tc-pitch",
				"PostgreSQL has one of the richest type systems of any production database, and the schema below uses a "+
					"deliberately small, opinionated slice of it. This is the guide's actual first migration — the file "+
					"the server applies to its own database at boot — not an illustration:"),
			code("tc-posts-sql", "sql",
				"Migration 1's posts table, verbatim from the guide's migrations.",
				"server/internal/data/db/migrations/00001_init.sql",
				"CREATE TABLE posts (\n"+
					"    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),\n"+
					"    title       text NOT NULL CHECK (length(title) BETWEEN 1 AND 200),\n"+
					"    body        text NOT NULL,\n"+
					"    created_at  timestamptz NOT NULL DEFAULT now(),\n"+
					"    updated_at  timestamptz NOT NULL DEFAULT now()\n"+
					");"),
			text("tc-types",
				"Every type here is chosen against an alternative, and the alternatives teach the choice. ``text`` over "+
					"``varchar(200)``: PostgreSQL stores them identically, and a length limit that belongs to the DOMAIN "+
					"(titles are 1–200 runes per the validation topic) belongs in a CHECK constraint, where it can evolve "+
					"without a table rewrite. ``uuid`` over ``text`` holding uuid-shaped strings: the database knows the "+
					"value's width, its canonical form, and can generate one via ``gen_random_uuid()`` — and a string "+
					"column would happily store ``\"hello\"`` as a post id. ``timestamptz`` over ``timestamp`` without "+
					"qualification is not a preference but a rule: an instant on the timeline is always a moment with a "+
					"zone attached; naive timestamps invite the classic bug where server-local midnight means three "+
					"different instants across DST boundaries."),
			text("tc-constraints",
				"The constraints are where the database stops being a dumb store and becomes an enforcer of facts. "+
					"``NOT NULL`` says the fact always applies. ``CHECK`` says values outside a range are refused — "+
					"note the empty title is illegal HERE too, one layer below the API's 422s: the validation topic's "+
					"rules run at the boundary for honest error messages, and the schema runs them again for the row "+
					"that arrives by any other path (a DBA's hotfix, a migration backfill, another service). "+
					"``DEFAULT gen_random_uuid()`` is a safety net, not the app's id source — the ids section makes "+
					"that distinction precise. And ``PRIMARY KEY`` is three declarations in one: the identifying "+
					"column, a uniqueness constraint, and an implicit index."),
			list("tc-type-choices", false,
				"text — unbounded strings; length rules that are DOMAIN rules live in CHECK, not in the type.",
				"uuid — 128-bit identifiers, natively typed, generatable in-database; never a text column pretending.",
				"timestamptz — every instant carries its zone; timestamp without one is a bug with a schema entry.",
				"boolean / integer / numeric — exact where money is involved (numeric), fast counters where it is not.",
				"text[] and jsonb — legitimate types (the tags section visits text[] as the denormalized alternative).",
				"tsvector — full-text search's inverted index type, which topic 17 turns into a demo."),
			text("tc-defaults",
				"``DEFAULT now()`` deserves one more sentence because defaults are often misread as app logic. A "+
					"default fires when an INSERT omits the column — nothing more. The application in topic 17 will "+
					"set ``created_at`` itself (the domain owns the draft's arrival instant), and the default stays as "+
					"the same kind of safety net as ``gen_random_uuid()``: rows inserted outside the app still get "+
					"sane values. What a default must never be used for is business meaning — \"created_at is null "+
					"means unconfirmed\" is a fact expressed through absence, and facts deserve columns."),
		),
		// ── 3. Primary and foreign keys ──────────────────────────────
		section("keys-and-relationships", "Primary and foreign keys: identity and edges",
			"Primary keys name rows; foreign keys state relationships as enforceable facts — with ON DELETE behavior chosen in the open.",
			text("keys-pitch",
				"Identity and relationship are the two load-bearing structures of a relational schema, and this guide's "+
					"migration states both for the posts domain. The many-to-many relationship between posts and tags "+
					"gets the classic three-table treatment — ``tags`` for the vocabulary, ``post_tags`` for the edges — "+
					"and every decision in it is visible in the SQL:"),
			code("keys-tags-sql", "sql",
				"The tag representation: a natural-keyed vocabulary plus a composite-keyed edge table.",
				"server/internal/data/db/migrations/00001_init.sql",
				"CREATE TABLE tags (\n"+
					"    name        text PRIMARY KEY CHECK (name <> '')\n"+
					");\n"+
					"\n"+
					"CREATE TABLE post_tags (\n"+
					"    post_id     uuid NOT NULL REFERENCES posts(id) ON DELETE CASCADE,\n"+
					"    tag_name    text NOT NULL REFERENCES tags(name) ON DELETE CASCADE,\n"+
					"    PRIMARY KEY (post_id, tag_name)\n"+
					");"),
			text("keys-pk",
				"The primary key of ``post_tags`` is doing three jobs at once, and that economy is why composite keys "+
					"earn their place on edge tables. It identifies the row (a post-tag pair IS the row's identity — "+
					"there is no separate ``post_tags_id`` because a surrogate id would identify nothing the pair does "+
					"not already identify); it enforces uniqueness (a post cannot carry the same tag twice — the "+
					"database refuses what the domain's validation would have to chase); and because a primary key is "+
					"backed by an index, it serves the forward lookup (\"this post's tags\") for free. The natural "+
					"key on ``tags`` makes the same argument one level up: a tag IS its name — there is nothing else "+
					"to identify — so ``name`` as PRIMARY KEY states that fact directly, where a surrogate ``id`` "+
					"would add a join without adding meaning."),
			text("keys-fk",
				"The foreign keys state the edges as facts the database enforces: a ``post_tags`` row cannot reference "+
					"a post that does not exist, nor a tag outside the vocabulary. That enforcement is what makes "+
					"referential integrity the database's job instead of the application's — every writer, present and "+
					"future, inherits it. But an FK states more than existence; it must answer a question applications "+
					"answer badly and inconsistently: WHAT HAPPENS TO THE EDGE WHEN ITS ENDPOINT DISAPPEARS? "+
					"PostgreSQL makes you choose per relationship, and the guide's choices are:"),
			list("keys-on-delete", false,
				"ON DELETE CASCADE (post_tags → posts, post_tags → tags) — the edge dies with its endpoint: delete a post and its tag edges go with it; no orphans, no cleanup job.",
				"ON DELETE SET NULL (audit_log → posts, next section's SQL) — the edge is severed but the row survives: history outlives its subject.",
				"ON DELETE RESTRICT — the delete is refused until the children are gone: use when children have independent right to exist.",
				"NO ACTION — the default, checked at statement end; effectively RESTRICT unless deferred, which matters inside multi-statement transactions.",
				"Undeclared — not a choice the guide makes: without the FK, integrity becomes folklore every writer must remember."),
			text("keys-honest",
				"The honest summary of key design is a pair of questions you can ask of any column. For a primary key: "+
					"does this value identify the row FOREVER, across every table that will ever reference it? For a "+
					"foreign key: when the referenced row dies, what is the truth about this row — does it die too "+
					"(CASCADE), lose a limb (SET NULL), or block the death (RESTRICT)? Answering these in the schema "+
					"means the answers are enforced once; answering them in application code means answering them "+
					"again in every writer, and getting at least one wrong."),
		),
		// ── 4. Normalization → denormalization ───────────────────────
		section("normalization-to-denormalization", "Normalization, then deliberate denormalization",
			"Normalize by default so every fact lives once; denormalize only with a named query you have measured.",
			text("norm-pitch",
				"Normalization has a reputation for academic ceremony, but its working definition fits in one sentence: "+
					"every non-key column depends on the key, the whole key, and nothing but the key. Run that test "+
					"against a mistake and it earns its keep immediately. Imagine ``author_name`` on every ``posts`` "+
					"row: it depends on the AUTHOR, not the post (transitive dependency — the author's name would "+
					"change in a thousand rows or, worse, in nine hundred and ninety-nine of them). Or ``post_title`` "+
					"on ``post_tags``: the title depends on the post, not on the post-tag PAIR, so the edge table "+
					"would smuggle a second copy of a fact posts already owns. The guide's schema passes the test "+
					"because each table states one kind of fact about one kind of thing."),
			text("norm-when",
				"The classical normal forms (1NF: atomic values, no repeating groups; 2NF: no partial dependency on "+
					"part of a composite key; 3NF: no transitive dependencies) are worth naming once and then using "+
					"as a checklist, not a religion. In practice the checklist collapses into two moves: pull repeating "+
					"groups into their own rows (a ``tag1``/``tag2``/``tag3`` column trio is the canonical 1NF "+
					"violation — and yes, it ships in real systems), and pull facts that describe something other "+
					"than the key into a table keyed by that something. Both moves trade a join for correctness. "+
					"Which raises the question every normalized schema eventually faces: when is the join too "+
					"expensive, and what then?"),
			text("norm-denorm",
				"Denormalization is not the opposite of good design; it is a measured, reversible bet: ADD a derived "+
					"copy of a fact that normalization would derive on the fly, to make a specific hot query cheap. "+
					"The tags representation is the guide's live example of the decision, because PostgreSQL offers "+
					"the denormalized alternative natively — ``posts.tags text[]`` with a GIN index would answer "+
					"\"posts with tag go\" without any join and without a separate vocabulary table. The guide "+
					"deliberately chose the normalized pair table, and the trade is worth stating exactly:"),
			list("norm-tags-trade", false,
				"Normalized (chosen) — one row per distinct tag, edges in post_tags: tag statistics, renames-by-edge-rewrite, and referential integrity are all real queries; every tagged read pays one join.",
				"Denormalized text[] — the tag set rides the post row: reads are one table, but no FK can check membership, and \"which posts have tag X\" needs the GIN index to avoid scanning everything.",
				"The deciding question — is a tag a RELATIONSHIP the product will query both ways (counts, co-occurrence, vocabulary management), or a LABEL that only decorates reads?",
				"The escape hatch — PostgreSQL's arrays and jsonb mean denormalizing later is a migration, not a rewrite; starting normalized keeps every option open.",
				"Other denormalizations worth naming — cached counters (``comment_count``) and read-model tables, both of which must ADMIT they are copies and define their invalidation."),
			text("norm-stance",
				"The stance this guide teaches: normalize until a real query hurts, then denormalize that one fact, in "+
					"the open, with its invalidation story written down — and never denormalize preemptively in the "+
					"name of performance you have not measured. Every denormalized fact is a small bet against "+
					"future writes being cheap; like all bets it should be placed with evidence."),
		),
		// ── 5. Indexes ───────────────────────────────────────────────
		section("indexes", "Indexes: when, what, which kind",
			"An index is a per-query data structure paid for on every write — the guide's are all in the migration, and the demo shows the live catalog's own account of them.",
			text("ix-pitch",
				"An index is the answer to \"this query scans too many rows\", and the first honest thing to say about "+
					"it is what it costs: every index on a table must be maintained on every INSERT and UPDATE that "+
					"touches its columns. Indexes are not free speed; they are a trade of write work (and disk, and "+
					"planner complexity) for read speed on a QUERY YOU CAN NAME. The guide's migration names its "+
					"queries in comments and creates exactly the indexes they justify — no index exists here \"just in "+
					"case\":"),
			code("ix-sql", "sql",
				"Migration 1's explicit indexes, rationale comments included — per-query decisions, not decoration.",
				"server/internal/data/db/migrations/00001_init.sql",
				"CREATE INDEX posts_created_at_idx   ON posts (created_at DESC);\n"+
					"CREATE INDEX post_tags_tag_name_idx ON post_tags (tag_name);"),
			text("ix-why-these",
				"``posts_created_at_idx`` exists because the posts API's default listing is newest-first — "+
					"``ORDER BY created_at DESC LIMIT 10`` — and without an index that is a full scan plus a sort of "+
					"every post, every page view. The index is DECLARED DESCENDING to match the query's direction, so "+
					"the first page is the index's first entries. ``post_tags_tag_name_idx`` exists because of a "+
					"detail that surprises everyone once: PostgreSQL creates an index for every primary key and "+
					"unique constraint, but NEVER for foreign key columns. The composite PK ``(post_id, tag_name)`` "+
					"covers lookups BY post; the reverse query — ``?tag=go``, which posts carry this tag — would "+
					"scan the whole edge table without an explicit index on ``tag_name``."),
			text("ix-kinds",
				"The kind of index matters as much as its columns, because the structure decides which queries can "+
					"use it at all. PostgreSQL's default is B-tree — ordered, so it serves equality, ranges, and "+
					"ORDER BY; it is the right answer for almost every scalar column and the only thing the guide's "+
					"current schema needs. The moment the denormalized ``text[]`` alternative from the tags section "+
					"enters the picture, GIN arrives with it: a B-tree cannot answer \"does this array contain go\", "+
					"but a GIN (Generalized Inverted iNdex) indexes each element to its containing rows — the same "+
					"structure PostgreSQL uses for ``tsvector`` full-text search in topic 17. The other kinds earn "+
					"one sentence each so the vocabulary is complete when you meet them:"),
			list("ix-kinds-list", false,
				"B-tree — ordered equality/range/sorting; the default, and live in the guide's schema from day one.",
				"GIN — inverted index for containment: array members, jsonb paths, full-text tokens — live here since topic 17's FTS expression index.",
				"BRIN — block-range summaries for huge, naturally-ordered tables (timestamp columns of append-only logs).",
				"Hash — plain equality only, rarely better than B-tree in practice.",
				"GiST/SP-GiST — geometric and nearest-neighbor searches; the geography workhorses of PostGIS.",
				"Partial — any B-tree with a WHERE clause: index only the hot subset (e.g. undeleted rows).",
				"Covering (INCLUDE) — carry extra columns so index-only scans skip the heap fetch."),
			text("ix-when",
				"When to add one, in order of decreasing confidence: a PRIMARY KEY or UNIQUE constraint needs it and "+
					"gets it automatically; a foreign key column used in reverse lookups (like ``tag_name``) is the "+
					"next clearest case; then the measured hot path — a query in your slow log or your EXPLAIN "+
					"output whose plan says Seq Scan over a growing table. And when NOT to: small tables (a seq scan "+
					"of 200 rows beats an index hop), write-heavy tables whose extra index buys no named query, and "+
					"low-selectivity columns where the index matches nearly every row anyway. Unused indexes are not "+
					"neutral — they tax every write — and PostgreSQL keeps the receipts: ``pg_stat_user_indexes`` "+
					"counts scans per index, which is how you find the ones to drop."),
			text("ix-demo-intro",
				"Everything this section claims, the live database can confirm — and that is exactly what the demo "+
					"below does. It does not render the migration FILES (this page has already quoted those); it "+
					"reads PostgreSQL's own catalog — ``pg_class`` for tables, ``format_type`` for column types, "+
					"``pg_get_constraintdef`` and ``pg_get_indexdef`` for the database's own rendering of every "+
					"constraint and index, plus goose's version ledger — and shapes the result into the payload you "+
					"see. If the schema and this page ever disagree, the demo shows the schema's side. When the "+
					"database is unreachable it degrades honestly to a 503 service_unavailable envelope rather than "+
					"pretending:"),
			demo("db:schema-explorer", "The schema explorer",
				"The live PostgreSQL's account of itself: every public table with its columns and real types, every "+
					"constraint definition (ON DELETE behavior included), every index with its access method and "+
					"columns, goose's migration ledger, and the curated rationale notes — introspected from "+
					"pg_catalog on every load, served as the catalog states it."),
			text("ix-demo-read",
				"Read it against the SQL above. The PRIMARY KEY rows appear among indexes — that is not the demo "+
					"being generous, it is how PostgreSQL implements them. ``post_tags_tag_name_idx`` sits beside "+
					"the composite PK, visibly separate — the reverse-lookup index that no automatic rule would have "+
					"created. And the migration history table (``goose_db_version``, baseline row included) appears "+
					"among the tables because it IS one — the explorer hides nothing, which is precisely the "+
					"teaching: the catalog is the truth, and files are just one way of having written it."),
		),
		// ── 6. Migration history ─────────────────────────────────────
		section("migration-history", "Migration history with goose",
			"The schema is a sequence of numbered, versioned SQL files compiled into the binary and applied at boot — never a dump, never a wiki page.",
			text("mig-pitch",
				"A schema designed but not VERSIONED is a schema that drifts: the production database slowly stops "+
					"matching what the code expects, and the fix is archaeology. The guide's decided tooling (from "+
					"the stack decisions) is goose: SQL-first migration files, numbered in order, tracked by a "+
					"version table INSIDE the database, so the database itself knows which version it is at. The "+
					"whole story fits one sentence per piece: migrations are plain ``.sql`` files in "+
					"``internal/data/db/migrations``; each carries an Up and a Down section; the server embeds the "+
					"directory into the binary; at boot it applies everything not yet applied; and re-running "+
					"against an up-to-date database applies nothing — the version table makes it a no-op."),
			code("mig-embed-go", "go",
				"The embed: the SQL rides inside the compiled server — one artifact to deploy, no migrations directory to forget.",
				"server/internal/data/db/embed.go",
				"//go:embed migrations/*.sql\n"+
					"var migrationsFS embed.FS"),
			text("mig-runner",
				"The runner is deliberately small — goose does the discovery, versioning, and transaction-per-"+
					"migration work; the guide's code adapts the shared pgx pool to goose's database/sql-based "+
					"runner and asks for everything unapplied. Note what it does NOT do: no silent retries, no "+
					"partial tolerance. A failed migration fails the boot, because a server running against a "+
					"schema it does not expect is worse than a server not running:"),
			code("mig-migrate-go", "go",
				"The whole migration runner: pool → database/sql adapter → goose provider → Up.",
				"server/internal/data/db/db.go",
				"func Migrate(ctx context.Context, pool *pgxpool.Pool) (int, error) {\n"+
					"    sqlDB := stdlib.OpenDBFromPool(pool)\n"+
					"    defer sqlDB.Close()\n"+
					"\n"+
					"    provider, err := goose.NewProvider(goose.DialectPostgres, sqlDB, Migrations())\n"+
					"    if err != nil {\n"+
					"        return 0, fmt.Errorf(\"build migration provider: %w\", err)\n"+
					"    }\n"+
					"    applied, err := provider.Up(ctx)\n"+
					"    if err != nil {\n"+
					"        return 0, fmt.Errorf(\"apply migrations: %w\", err)\n"+
					"    }\n"+
					"    return len(applied), nil\n"+
					"}"),
			text("mig-boot",
				"At the composition root, migration is a boot step with the same standing as binding a port — "+
					"PostgreSQL is part of this stack, not an optional service, so an unreachable database or a "+
					"failed migration stops the process before it serves anything. The log line records how many "+
					"migrations THIS boot applied, which makes deploys legible: a fresh environment prints the "+
					"whole set, an existing one prints zero:"),
			code("mig-main-go", "go",
				"Boot wiring in main.go: dial, migrate, fail fast — the pool then serves the whole process.",
				"server/cmd/guide/main.go",
				"pool, err := db.NewPool(bootCtx, cfg.DatabaseURL)\n"+
					"if err != nil {\n"+
					"    logger.Error(\"database unreachable\", \"error\", err)\n"+
					"    return fmt.Errorf(\"database: %w\", err)\n"+
					"}\n"+
					"defer pool.Close()\n"+
					"applied, err := db.Migrate(bootCtx, pool)\n"+
					"// …the migration failure path returns non-zero exactly like the dial failure above\n"+
					"logger.Info(\"database migrations applied\", \"applied\", applied)"),
			text("mig-dsn",
				"The database location is typed configuration like every other environment fact — "+
					"``GDG_DATABASE_URL``, with a default that points at the dev stack's PostgreSQL by its compose "+
					"service name (port 5432 inside the network; the host sees it published on 5433). Unset or "+
					"empty falls back to the default; an unreachable or unparseable DSN is a boot failure at the "+
					"dial, which is exactly where the driver's diagnostics live:"),
			code("mig-config-go", "go",
				"The config entry and its dev-stack default.",
				"server/internal/core/config.go",
				"const (\n"+
					"    // defaultDatabaseURL points at the dev stack's PostgreSQL by its\n"+
					"    // compose-network service name (host port 5433 maps to 5432 inside\n"+
					"    // the network) with the compose-declared credentials.\n"+
					"    defaultDatabaseURL = \"postgres://postgres:guide@postgres:5432/guide?sslmode=disable\"\n"+
					")"),
			text("mig-file-anatomy",
				"A migration file's anatomy is a contract goose parses: the ``-- +goose Up`` marker starts the "+
					"forward section, ``-- +goose Down`` the rewind, and the filename's number IS the version. One "+
					"rule the trigger migration teaches the hard way: goose splits SQL on semicolons, so a "+
					"dollar-quoted function body — whose semicolons are PL/pgSQL's, not statement separators — "+
					"must be wrapped in ``-- +goose StatementBegin`` / ``StatementEnd``. The guide's first "+
					"migration hits that immediately, because it creates the ``set_updated_at`` trigger function:"),
			code("mig-trigger-sql", "sql",
				"Dollar-quoted bodies need explicit statement boundaries — goose's splitter does not read minds.",
				"server/internal/data/db/migrations/00001_init.sql",
				"-- +goose StatementBegin\n"+
					"CREATE FUNCTION set_updated_at() RETURNS trigger AS $$\n"+
					"BEGIN\n"+
					"    NEW.updated_at := clock_timestamp();\n"+
					"    RETURN NEW;\n"+
					"END;\n"+
					"$$ LANGUAGE plpgsql;\n"+
					"-- +goose StatementEnd"),
			text("mig-updated-at",
				"That trigger is a design decision this guide made on purpose and will defend: ``updated_at`` is "+
					"stamped BY THE DATABASE, on every UPDATE, no matter who issues it — the app, a DBA's hotfix, a "+
					"backfill script. The alternative is app-side stamping (every repository update sets the "+
					"column), which works until one UPDATE forgets, and then the column lies. The cost of the "+
					"trigger is a function call per row update; the cost of app-side is a silent correctness bug "+
					"in year two. Note the split of responsibility it implies: ``created_at`` defaults in the "+
					"database but is set by the app (the domain owns \"when this draft arrived\"), while "+
					"``updated_at`` belongs entirely to the database — the tests prove the trigger by updating a "+
					"row through raw SQL, with no Go code anywhere near it."),
			text("mig-discipline",
				"The discipline that keeps a goose history healthy is short: migrations are append-only once "+
					"shipped (a mistake is a NEW migration that repairs it, never an edit to an applied file — "+
					"the version table remembers the old checksum of your intent); each is small enough to review "+
					"in one glance; Down sections are best-effort rewinds for local experiments, not a promise "+
					"that production can run backwards; and the same binary that serves traffic runs the "+
					"migrations, which is what makes the single-artifact deploy story real. The goose CLI is "+
					"pinned as a go tool for exactly this workflow:"),
			code("mig-mod", "mod",
				"The goose CLI ships as a pinned go tool — no global install, no version drift.",
				"server/go.mod",
				"tool (\n"+
					"    github.com/99designs/gqlgen\n"+
					"    github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen\n"+
					"    github.com/pressly/goose/v3/cmd/goose\n"+
					"    // …plus sqlc and the protobuf/gRPC generators\n"+
					")"),
		),
		// ── 7. Identifiers ───────────────────────────────────────────
		section("identifiers", "Choosing ids: UUIDv7 vs sequential",
			"bigserial is cheap and enumerable; UUIDv4 is unguessable and index-hostile; UUIDv7 is the deliberate middle — and this guide generates it itself.",
			text("id-pitch",
				"Identifier choice is the first schema decision that reaches OUTSIDE the database — into URLs, logs, "+
					"API contracts, and other services — so it deserves its own section rather than a DEFAULT line. "+
					"The classic trade has three corners. Sequential ids (``bigserial``) are 8 bytes, perfectly "+
					"ordered, and leak information: ``/api/posts/1``, ``/api/posts/2`` invites enumeration, and row "+
					"counts leak through the ids themselves. Random UUIDs (v4, what ``gen_random_uuid()`` mints) "+
					"are unguessable and globally unique — and scatter inserts uniformly across the B-tree, so "+
					"every insert touches a random page and a hot large index thrashes the cache. UUIDv7 is the "+
					"2024-standardized middle: a 48-bit millisecond timestamp leads the value, so ids are roughly "+
					"time-ordered (inserts localize in the index like a sequence's), while 74 random bits follow "+
					"(unguessable, coordination-free)."),
			text("id-hand-rolled",
				"The guide could have added google/uuid for this; it deliberately did not, because UUIDv7 is simple "+
					"enough to own, and owning the layout makes it teachable: 128 bits, four fields, one formatting "+
					"function. The core of the real generator from the demo's payload builder — this is the code "+
					"that mints every id in this page's evidence:"),
			code("id-generator-go", "go",
				"The layout function: every field lands in RFC 9562's position.",
				"server/internal/domain/demos/uuid_lab.go",
				"func uuidv7(unixMs uint64, seq uint16, randB [8]byte) [16]byte {\n"+
					"    var u [16]byte\n"+
					"    u[0] = byte(unixMs >> 40)\n"+
					"    // …bytes 1–4 shift the timestamp down the same way, big-endian\n"+
					"    u[5] = byte(unixMs)\n"+
					"    u[6] = version7<<4 | byte(seq>>8)&0x0F\n"+
					"    u[7] = byte(seq)\n"+
					"    u[8] = variantRFC4122<<6 | randB[0]&0x3F\n"+
					"    copy(u[9:], randB[1:])\n"+
					"    return u\n"+
					"}"),
			text("id-monotonic",
				"One subtlety separates a toy from a production v7: ids minted within the SAME millisecond. If "+
					"``rand_a`` were pure randomness, same-millisecond ids would sort randomly, and the \"lexical "+
					"order equals time order\" claim would wobble at exactly the rate your service mints ids. "+
					"RFC 9562's counter-method fixes it: the guide's generator seeds ``rand_a`` randomly once per "+
					"millisecond, then increments it per id under a mutex — so within a millisecond ordering holds "+
					"by counter, and across milliseconds it holds by timestamp. On counter exhaustion it waits for "+
					"the next millisecond rather than wrapping (4096 ids per millisecond is four million per "+
					"second; the guard is for the stuck-clock case, and it errors loudly):"),
			code("id-reserve-go", "go",
				"The monotonicity contract in one place: new millisecond seeds the counter, same millisecond increments it.",
				"server/internal/domain/demos/uuid_lab.go",
				"unixMs := g.now().UnixMilli()\n"+
					"switch {\n"+
					"case unixMs > g.lastMS:\n"+
					"    var seed [2]byte\n"+
					"    // …io.ReadFull seeds the counter from the generator's injected randomness\n"+
					"    g.lastMS = unixMs\n"+
					"    g.seq = 1 + binary.BigEndian.Uint16(seed[:])%seqMax\n"+
					"    return uint64(unixMs), g.seq, nil\n"+
					"case g.seq < seqMax:\n"+
					"    g.seq++\n"+
					"    return uint64(unixMs), g.seq, nil\n"+
					"default:\n"+
					"    // …counter exhausted: spin to the next millisecond or fail on a stuck clock\n"+
					"}"),
			text("id-demo-intro",
				"The demo below runs that generator live, and it does not just CLAIM the properties — it measures "+
					"them in front of you: a batch minted in order and its lexically sorted copy (they must match "+
					"element for element), a ten-thousand-id collision check (distinct count must equal minted "+
					"count), the field-by-field anatomy of one fresh value, and the three-strategy comparison "+
					"table the decision actually turns on. The VALUES change per request — they are fresh "+
					"randomness — the properties never do:"),
			demo("db:uuid", "The UUIDv7 lab",
				"A hand-rolled RFC 9562 UUIDv7 generator in action: layout anatomy of a fresh value, a batch whose "+
					"lexical sort equals its mint order (the btree-locality argument, proven not asserted), a "+
					"10,000-id collision check, and the bigserial / uuidv7 / uuidv4 trade-off table with "+
					"PostgreSQL's own gen_random_uuid() in the third row."),
			text("id-stance",
				"The guide's recorded stance, so later topics can lean on it: primary keys are ``uuid`` columns "+
					"carrying UUIDv7 values minted APP-SIDE (the domain knows the creation instant and can build "+
					"aggregates before any row exists), with ``DEFAULT gen_random_uuid()`` as the in-database "+
					"safety net for rows inserted outside the app — a v4, and that difference is fine because "+
					"safety-net rows are rare by construction. Sequential ids remain the right call where "+
					"enumerability is harmless and locality matters most — internal join tables, queue positions, "+
					"sort orders — which is exactly why the vocabulary is worth carrying even when you always "+
					"choose the same corner."),
		),
		// ── 8. The audit-logging pattern ─────────────────────────────
		section("audit-logging", "The audit-logging pattern",
			"An append-only table written inside the same transaction as the change it describes — history as a fact the database enforces.",
			text("audit-pitch",
				"\"Who changed this, and when?\" is a requirement that arrives late and is expensive to retrofit, "+
					"so the guide builds it now, as a pattern: one audit table per audited subject, one append-"+
					"only row per state change, written by the SAME transaction that makes the change. The schema "+
					"is the guide's second migration — small enough to read whole:"),
			code("audit-sql", "sql",
				"Migration 2 in full: the audit table, its SET NULL foreign key, and its history index.",
				"server/internal/data/db/migrations/00002_audit_log.sql",
				"CREATE TABLE audit_log (\n"+
					"    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),\n"+
					"    post_id     uuid REFERENCES posts(id) ON DELETE SET NULL,\n"+
					"    action      text NOT NULL CHECK (action IN ('create', 'update', 'delete')),\n"+
					"    actor       text NOT NULL,\n"+
					"    summary     text NOT NULL,\n"+
					"    changed_at  timestamptz NOT NULL DEFAULT now()\n"+
					");\n"+
					"\n"+
					"CREATE INDEX audit_log_post_changed_idx ON audit_log (post_id, changed_at DESC);"),
			text("audit-decisions",
				"Three decisions in that table generalize to every audit schema you will write. First, the FK is "+
					"``ON DELETE SET NULL`` deliberately: the audit trail must OUTLIVE its subject — CASCADE would "+
					"rewrite history on every delete, and RESTRICT would make audited rows undeletable, both wrong "+
					"for a log whose whole job is remembering. The pre-delete facts ride in ``summary``. Second, "+
					"``action`` is a CHECK-constrained vocabulary, not a free string — unbounded enum drift is how "+
					"audit tables become unqueryable. Third, the composite index ``(post_id, changed_at DESC)`` "+
					"exists for THE audit query — \"this record's history, newest first\" — an index decision "+
					"made by the same rule as every other in the indexes section: name the query, then index for it."),
			text("audit-in-tx",
				"The pattern's second half lives in application code (topic 17 builds it as a demo over the posts "+
					"repository): the audit append rides INSIDE the changing transaction. Not after — inside. If "+
					"the change and its audit row commit together, the log is exactly true; if the audit write "+
					"happened after commit and the process died between, history silently loses the change it "+
					"exists to record, and the two-table write becomes a distributed-systems problem instead of a "+
					"database feature. The same transaction is also what makes the pattern cheap: no extra "+
					"round-trip protocol, no outbox, no reconciliation job — one ``BEGIN … INSERT post … INSERT "+
					"audit_log … COMMIT``, atomic by construction. The rule generalizes past audits to every "+
					"derived write that must not drift: same transaction, or admit it is eventually consistent."),
			text("audit-tradeoffs",
				"Honest accounting of what the pattern costs, because audit logging is not free and pretending "+
					"otherwise is how it gets disabled in year two. Every audited write writes twice (the row and "+
					"its audit row) — on the guide's scale that is noise; on a hot path it is a real doubling, and "+
					"the mitigation is auditing EVENTS not ROW STATES (one audit row per logical change, not per "+
					"column diff), which also keeps the log readable. ``summary`` is denormalized prose by "+
					"design — it snapshots what the actor did in their words — so it is a copy that admits it is "+
					"a copy, per the normalization section's rule. And access to the audit table is read-mostly "+
					"and append-only: no UPDATE grants, no DELETE grants, reads through a dedicated path. An "+
					"audit log an attacker with a stolen credential can edit is a liability, not a control."),
		),
		// ── 9. Terminal companion ────────────────────────────────────
		section("terminal-companion", "Terminal companion: the curl tour",
			"The schema from the shell: the live introspection payload, the uuid lab's evidence, the boot's own migration log line, and goose's ledger.",
			text("term-pitch",
				"With the stack up (server on :8100, PostgreSQL migrated at boot), this tour reads the topic's two "+
					"demos from the shell and checks the migration story's moving parts. You will see the catalog "+
					"introspection payload (tables, constraints with their ON DELETE clauses, indexes with their "+
					"access methods), the uuid lab's sorted batch and collision count, the boot log line recording "+
					"what THIS process applied, and goose's own version table — the same rows the schema explorer "+
					"serves, read straight from the database. ``python3 -m json.tool`` pretty-prints; the last "+
					"command runs inside the dev container, where the pinned go tool lives:"),
			code("term-explorer", "bash", "", "",
				"# the live schema as the catalog states it: tables, columns, constraints, indexes\n"+
					"curl -s localhost:8100/api/demo/db/schema-explorer | python3 -m json.tool | head -60"),
			code("term-explorer-grep", "bash", "", "",
				"# the ON DELETE decisions, straight from pg_get_constraintdef\n"+
					"curl -s localhost:8100/api/demo/db/schema-explorer \\\n"+
					"  | python3 -c 'import json,sys; d=json.load(sys.stdin); [print(c[\"name\"], \"—\", c[\"definition\"]) for c in d[\"constraints\"]]'"),
			code("term-uuid", "bash", "", "",
				"# fresh UUIDv7 evidence: anatomy, the ordered batch, the collision check\n"+
					"curl -s localhost:8100/api/demo/db/uuid | python3 -m json.tool | head -50"),
			code("term-uuid-sorted", "bash", "", "",
				"# the headline property: mint order and lexical sort agree, member for member\n"+
					"curl -s localhost:8100/api/demo/db/uuid \\\n"+
					"  | python3 -c 'import json,sys; d=json.load(sys.stdin); b=d[\"batch\"]; print(\"sorted == minted:\", b[\"sortingMatchesGenerationOrder\"])'"),
			code("term-boot-log", "bash", "", "",
				"# the boot's own account of the migration run (first boot on a fresh database\n"+
					"# prints applied: 2; every later boot prints applied: 0 — the version table at work)\n"+
					"grep 'database migrations applied' server.log | tail -1"),
			code("term-goose-status", "bash", "", "",
				"# goose's own ledger, read with the pinned go tool from the server module —\n"+
					"# same rows the explorer serves, from the CLI side (dev container):\n"+
					"cd server && go tool goose postgres \\\n"+
					"  'postgres://postgres:guide@postgres:5432/guide?sslmode=disable' \\\n"+
					"  -dir internal/data/db/migrations status"),
			code("term-ledger", "bash", "", "",
				"# and the ledger itself, straight from psql — version 0 is goose's baseline row\n"+
					"docker exec go_developers_guide_postgres_dev \\\n"+
					"  psql -U postgres -d guide -c 'SELECT version_id, is_applied FROM goose_db_version ORDER BY id;'"),
			text("term-coda",
				"Notice what the tour proves without asking you to trust prose: every constraint definition came "+
					"from the database's own mouth, the sorted batch was minted seconds ago and still ordered, and "+
					"the version ledger's rows match the files on disk one migration to one row. That is the "+
					"standard the data phase holds itself to from here on — the next topic swaps the in-memory "+
					"posts repository for a pgx one over exactly this schema, behind the same port the API has "+
					"used since topic 11, and nothing above changes when it does. The schema is the contract; "+
					"the catalog is its proof."),
		),
	}

	if buildErr != nil {
		return Topic{}, buildErr
	}
	return NewTopic(
		"sql-schema",
		"SQL schema design",
		"The guide's own PostgreSQL schema, designed in the open: types and constraints, keys with explicit ON "+
			"DELETE behavior, the tags normalization call, indexes as per-query decisions, goose migrations "+
			"embedded in the binary, UUIDv7 vs sequential ids, and the audit-logging pattern.",
		"data",
		16,
		sections,
	)
}
