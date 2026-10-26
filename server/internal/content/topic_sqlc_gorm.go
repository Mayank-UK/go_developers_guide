package content

import (
	"errors"
	"fmt"
)

// This file is topic 18 sqlc-gorm (group data, curriculum order 18), the
// Go-new topic the data phase builds its tooling story around: sqlc —
// SQL-first, queries as code, compile-time types — and GORM — a full ORM,
// with the conventions, hooks, and associations that implies. The topic's
// center of gravity is the measured N+1 exhibit: one seeded dataset, one
// logical job, three data-access styles, every SQL statement counted, and
// the numbers derived by RUNNING against a real PostgreSQL — first in the
// testcontainers tests, then live on every demo request. Transactions
// across repositories (the unit of work), lazy versus eager loading as
// shapes, and an honest choosing guide close it out.

// sqlcGormTopic builds topic 18.
func sqlcGormTopic() (Topic, error) {
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
		// ── 1. sqlc: SQL first ──────────────────────────────────────
		section("sqlc-sql-first", "sqlc: SQL first, queries as code",
			"Write the SQL, declare the schema, generate the Go — and every query is type-checked against that schema before your program ever runs.",
			text("sqlc-pitch",
				"Topic 17 wrote SQL by hand and defended the practice: the statement is visible, "+
					"the planner is reachable, nothing stands between the code and what the database "+
					"receives. The honest cost was named there too — SQL strings are untyped at the "+
					"call site, so a renamed column is a runtime discovery, and the same scan-and-map "+
					"plumbing repeats in every repository. sqlc answers exactly that cost without "+
					"giving up the SQL: you write the queries as ``.sql`` files, point sqlc at the "+
					"schema, and it generates a typed Go function per query — parameters and result "+
					"rows as structs, checked against the schema by a real PostgreSQL parser at "+
					"generation time. The SQL stays yours; the typing becomes the tool's; a column "+
					"rename turns into a compile error in every caller."),
			code("sqlc-config-yaml", "yaml",
				"The probe's whole sqlc configuration: engine, schema, queries, and the Go package to emit.",
				"server/internal/data/ormprobe/sqlc.yaml",
				"version: \"2\"\n"+
					"sql:\n"+
					"  - engine: \"postgresql\"\n"+
					"    schema: \"schema.sql\"\n"+
					"    queries: \"queries\"\n"+
					"    gen:\n"+
					"      go:\n"+
					"        package: \"ormprobe\"\n"+
					"        out: \".\"\n"+
					"        sql_package: \"pgx/v5\"\n"+
					"        overrides:\n"+
					"          - db_type: \"pg_catalog.timestamptz\"\n"+
					"            go_type:\n"+
					"              import: \"time\"\n"+
					"              type: \"Time\""),
			text("sqlc-query",
				"The query file is where the work happens, and it is just SQL with one "+
					"addition: a name annotation that becomes the Go function. The probe's single "+
					"query is deliberately the same statement topic 17's DAO teaches — the page "+
					"and its tags in one round trip, tags aggregated by ``array_agg`` — because "+
					"the entire topic that follows measures this statement against the ORM's "+
					"alternatives. One detail earns its keep: the ``::text[]`` cast. sqlc infers "+
					"Go types from the query's return types, and a ``COALESCE`` wrapped around a "+
					"``FILTER``-ed aggregate infers as untyped (``interface{}``); the explicit "+
					"cast pins the type for the planner AND for the generator, and the generated "+
					"row carries ``[]string`` instead of a mystery box:"),
			code("sqlc-query-sql", "sql",
				"One annotation, one statement, one typed function — :many rows, tags included.",
				"server/internal/data/ormprobe/queries/posts.sql",
				"-- name: ListPostsWithTags :many\n"+
					"SELECT p.id, p.title, p.body, p.created_at,\n"+
					"       (COALESCE(array_agg(pt.tag_name ORDER BY pt.tag_name)\n"+
					"                 FILTER (WHERE pt.tag_name IS NOT NULL), '{}'))::text[] AS tags\n"+
					"FROM ormprobe_posts p\n"+
					"LEFT JOIN ormprobe_post_tags pt ON pt.post_id = p.id\n"+
					"GROUP BY p.id, p.title, p.body, p.created_at\n"+
					"ORDER BY p.id;"),
			code("sqlc-generated-go", "go",
				"What generation emits: the query as a typed call, the row as a struct — committed, reviewed, compiled.",
				"server/internal/data/ormprobe/posts.sql.go",
				"const listPostsWithTags = `-- name: ListPostsWithTags :many\n"+
					"SELECT p.id, p.title, p.body, p.created_at,\n"+
					"// …the same statement the query file carries, verbatim\n"+
					"\n"+
					"type ListPostsWithTagsRow struct {\n"+
					"	ID        int64\n"+
					"	Title     string\n"+
					"	Body      string\n"+
					"	CreatedAt time.Time\n"+
					"	Tags      []string\n"+
					"}"),
			text("sqlc-gen-read",
				"Reading the generated code is the moment sqlc clicks: ``ListPostsWithTagsRow`` "+
					"is a struct the tool derived — nobody hand-maintains it — and calling "+
					"``ListPostsWithTags(ctx)`` returns ``[]ListPostsWithTagsRow`` or an error, "+
					"no strings, no ``Scan`` counts to keep honest. The generated ``New`` takes "+
					"anything implementing ``DBTX`` (a pool, a connection, a transaction), which "+
					"is the same seam pgx code uses — sqlc output rides the guide's existing "+
					"driver and pooling, not a parallel universe. The ``sql_package: pgx/v5`` "+
					"line in the config is what buys that; the one override maps "+
					"``timestamptz`` to ``time.Time`` instead of pgtype's wrapper, keeping the "+
					"row pleasant to hold."),
			text("sqlc-loop",
				"The workflow has one loop in it, and it must be a deliberate step: edit schema "+
					"or queries, regenerate, commit the output alongside the inputs. The "+
					"generated files live in the repository on purpose — reviewers see the typed "+
					"shape change in the diff, CI compiles it like any other Go, and nobody's "+
					"build secretly depends on a codegen step running first. The discipline that "+
					"keeps it honest is regen-and-diff: re-run the tool, and if the committed "+
					"output changes when you changed nothing, someone edited generated code by "+
					"hand — the terminal companion shows the exact command pair. What sqlc does "+
					"NOT do is write your SQL: the query file is still yours to design, "+
					"EXPLAIN, and tune; the tool only removes the typing tax you paid for "+
					"writing it well."),
			list("sqlc-facts", false,
				"SQL-first — the query file is ordinary PostgreSQL, tuned and explained like any hand-written statement.",
				"Compile-time types — parameters and rows become Go structs; schema drift breaks the build, not production.",
				"Committed output — generated code is reviewed and compiled like the rest; no hidden codegen step in the build.",
				"Driver-native — sql_package pgx/v5 rides the same pool and transactions the hand-written repositories use.",
				"No query builder — composing SQL in method calls is a different tool's trade; sqlc keeps the SQL literal.",
				"The tool loop — schema or query edits require regenerate + commit; regen-and-diff is the anti-drift check."),
		),
		// ── 2. GORM: what an ORM does ──────────────────────────────
		section("gorm-what-an-orm-does", "GORM: what an ORM does, and what it costs",
			"Map structs to tables and associations to methods — and pay for it in conventions you must know, not just code you don't write.",
			text("orm-pitch",
				"GORM walks the opposite direction from sqlc: start from Go structs, let the "+
					"ORM derive the SQL. A struct is a table (by naming convention, overridable), "+
					"a field is a column, a slice field with the right tag is an association, "+
					"and ``Create``, ``Find``, ``Preload`` compose the statements for you. The "+
					"productivity is real: CRUD that would take a repository of hand-written "+
					"statements becomes method calls, and the same structs serve reads and "+
					"writes. The cost is equally real, and it is not performance — it is "+
					"KNOWLEDGE: the ORM has conventions for everything (table names, "+
					"timestamp stamping, primary-key handling), and code whose SQL you never "+
					"wrote is still SQL you must be able to read when it misbehaves. This "+
					"topic maps the SAME tables sqlc reads, so every convention below has a "+
					"visible consequence:"),
			code("orm-models-go", "go",
				"The probe's GORM models: one schema, a second mapping — TableName and join columns stated, not guessed.",
				"server/internal/data/ormprobe/gorm.go",
				"type Post struct {\n"+
					"	ID        int64 `gorm:\"primaryKey\"`\n"+
					"	Title     string\n"+
					"	Body      string\n"+
					"	CreatedAt time.Time\n"+
					"	Tags      []Tag `gorm:\"many2many:ormprobe_post_tags;joinForeignKey:post_id;joinReferences:tag_name\"`\n"+
					"}"),
			text("orm-conventions",
				"Three conventions are already at work in that struct. ``TableName()`` overrides "+
					"the default naming strategy, which would point ``Post`` at a table called "+
					"``posts`` — the guide's REAL posts table; the prefix override is not "+
					"decoration, it is the difference between the probe's dataset and the "+
					"application's data. ``CreatedAt time.Time`` is special-cased by name: on "+
					"``Create``, a zero value is stamped for you — the convention replacing "+
					"topic 17's explicit ``now()`` plumbing. And ``ID int64`` with "+
					"``primaryKey`` is treated as auto-increment: inserts omit it and read it "+
					"back through ``RETURNING``. The many2many tag states the join table AND "+
					"both join columns explicitly because the edge here has a natural-keyed "+
					"side (``tags.name``, not ``tags.id``) — a place where guessing the "+
					"convention would silently produce the wrong query."),
			text("orm-hooks",
				"Hooks are the ORM's inversion of the write path: methods the model may define "+
					"that GORM calls inside the statement's lifecycle — ``BeforeCreate`` before "+
					"the INSERT runs, with the same transaction underneath. The probe registers "+
					"one that normalizes the model — trimming the title and rejecting an empty "+
					"one — and the unit-of-work exhibit observes its effect from inside the "+
					"transaction: the row read back carries the trimmed title, and a hook "+
					"error unwinds the whole unit. That is the shape to internalize: hooks "+
					"are in-statement logic, not post-commit cleanup, so they compose with "+
					"transactions for free — and they are the first place business rules "+
					"hide when you are reading someone else's ORM code:"),
			code("orm-hook-go", "go",
				"A hook: in-statement, in-transaction model logic the exhibit observes firing.",
				"server/internal/data/ormprobe/gorm.go",
				"func (p *Post) BeforeCreate(_ *gorm.DB) error {\n"+
					"	p.Title = strings.TrimSpace(p.Title)\n"+
					"	if p.Title == \"\" {\n"+
					"		return errors.New(\"ormprobe: a post needs a title\")\n"+
					"	}\n"+
					"	return nil\n"+
					"}"),
			text("orm-escape",
				"When the convention does not fit, GORM's answer is clauses — explicit "+
					"composable pieces of SQL behavior attached to a call. The tags repository "+
					"upserts with ``clause.OnConflict{DoNothing: true}``, which is exactly the "+
					"``ON CONFLICT DO NOTHING`` the raw seed spells in SQL; the same mechanism "+
					"covers locking (``clause.Locking{Strength: \"UPDATE\"}`` — topic 17's "+
					"``FOR UPDATE``) and partial writes. The escape hatch existing is the good "+
					"news; needing to know it is the cost. An ORM compresses the code you "+
					"write, not the knowledge you need — the moment it generates a query you "+
					"cannot predict, you are back to reading SQL anyway, minus the "+
					"fingerprints of having written it. The session logger is how you keep "+
					"the visibility topic 17 built: the probe counts every statement through "+
					"it, and the N+1 section leans on exactly that seam."),
		),
		// ── 3. The N+1 problem, measured ───────────────────────────
		section("the-n-plus-one-problem", "The N+1 problem, measured",
			"List the page, then fetch each row's relation in a loop: N rows cost N+1 round trips — invisible in code, unmistakable in a statement count.",
			text("n1-pitch",
				"The N+1 query problem is the bug every data-access style can ship: fetch a "+
					"list of parents, then resolve each parent's relation inside the loop — "+
					"one query for the page plus one per row, N+1 statements for one logical "+
					"job. At twelve posts it is a rounding error; at twelve thousand it is an "+
					"outage wearing a profiler's chart. ORM users meet it most often because "+
					"the loop is the library's DEFAULT (lazy accessors hide it), but hand-"+
					"written code meets it too — the shape is the loop, not the tool. The "+
					"guide refuses to teach it as an opinion, so the demo measures it: the "+
					"probe runs the same job three ways against a seeded posts+tags dataset "+
					"and COUNTS the SQL statements each one issues, through the observability "+
					"seam each library actually offers."),
			list("n1-counting-seams", false,
				"pgx pool tracer — a pgx.QueryTracer attached at pool construction records every statement crossing the connections.",
				"GORM session logger — a gorm.Logger whose Trace callback counts each statement one session issues.",
				"sqlc call sites — generated code makes the statement count a code fact: the measured phase contains exactly one call."),
			code("n1-tracer-go", "go",
				"The counting middleware: one tracer, attached before any connection exists.",
				"server/internal/data/ormprobe/probe.go",
				"func (c *statementCounter) TraceQueryStart(\n"+
					"	ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryStartData,\n"+
					") context.Context {\n"+
					"	c.mu.Lock()\n"+
					"	c.statements = append(c.statements, normalizeSQL(data.SQL))\n"+
					"	c.mu.Unlock()\n"+
					"	return ctx\n"+
					"}"),
			text("n1-lazy-loop",
				"Here is the exhibit itself, in GORM form. Go has no transparent lazy "+
					"loading — struct fields are not proxies — so lazy loading here is an "+
					"explicit association call per row, and the loop's cost is countable at "+
					"a glance once you know to look: one ``Find`` for the page, then one "+
					"query per post for its tags. The session-logger counter wraps the whole "+
					"session, which is why the measurement is honest — nothing between the "+
					"reset and the read runs but this code:"),
			code("n1-lazy-go", "go",
				"The N+1, spelled out: the page, then per-row tag fetches — 1 + 12 statements.",
				"server/internal/data/ormprobe/gorm.go",
				"var posts []Post\n"+
					"if err := session.Order(\"id\").Find(&posts).Error; err != nil {\n"+
					"	return nil, demos.ORMProbeTrialFacts{}, fmt.Errorf(\"gorm: load the posts page: %w\", err)\n"+
					"}\n"+
					"for i := range posts {\n"+
					"	if err := session.Model(&posts[i]).Association(\"Tags\").Find(&posts[i].Tags); err != nil {\n"+
					"		return nil, demos.ORMProbeTrialFacts{}, fmt.Errorf(\"gorm: load tags for post %d: %w\", posts[i].ID, err)\n"+
					"	}\n"+
					"}"),
			demo("db:orm-probe", "The ORM probe: statements counted",
				"One seeded dataset (12 posts, 5 tags, 24 edges on the guide's own "+
					"PostgreSQL), one logical job run four ways, every SQL statement counted "+
					"through its library's own seam: raw pgx 1, sqlc 1, GORM's naive loop 13 "+
					"(the N+1), GORM with Preload 3 — page, join-table IN, tags IN. Each "+
					"trial carries its observed timing band and its actual statement "+
					"receipts; the unit-of-work exhibit below it rolls a two-repository "+
					"transaction back, provably. Numbers are measured live per request."),
			text("n1-read",
				"Read the payload's statement column top to bottom and the topic's argument "+
					"makes itself: the baseline and sqlc answer the whole question in ONE "+
					"statement (the array_agg JOIN — tags ride the same round trip); the "+
					"naive loop pays 13; Preload collapses to 3 — and 3, not the folkloric "+
					"2, is itself a lesson: a many2many preload costs one query per TABLE "+
					"involved (the join rows, then the tag rows), so even the fix's count "+
					"depends on the association's shape, which is exactly why you measure "+
					"instead of recite. The ratio column is the durable comparison — 13x the "+
					"round trips — because it divides out clock noise: on a local database "+
					"every band reads sub-millisecond, and the LOOP is still wrong, since "+
					"each round trip is a network hop that production's cross-AZ latency "+
					"multiplies by 13. Statement counts scale with the dataset; durations "+
					"only tell you where you are."),
			code("n1-measured-go", "go",
				"How the probe runs the sqlc trial: reset the tracer window, one generated call, read the clock.",
				"server/internal/data/ormprobe/probe.go",
				"counter.Reset()\n"+
					"started = time.Now()\n"+
					"sqlcRows, err := New(pool).ListPostsWithTags(ctx)\n"+
					"if err != nil {\n"+
					"	return demos.ORMProbeObservation{}, fmt.Errorf(\"run the sqlc trial: %w\", err)\n"+
					"}"),
			text("n1-honest",
				"Two honesty notes the probe bakes in. First, the four trials must answer "+
					"identical rows — the payload's verdict compares ids, titles, and tag SETS "+
					"across all four runs, because a measurement between programs answering "+
					"different questions measures nothing. Tag order deliberately does not "+
					"count: the SQL aggregates alphabetically, GORM returns scan order, and "+
					"neither is more correct. Second, everything the probe does to the "+
					"database is idempotent and residue-free — the schema is ``CREATE TABLE "+
					"IF NOT EXISTS``, the seed rides ``ON CONFLICT DO NOTHING``, and the "+
					"unit-of-work exhibit rolls its writes back — so the second request "+
					"measures the same world the first one did."),
		),
		// ── 4. Lazy vs eager loading ───────────────────────────────
		section("lazy-vs-eager-loading", "Lazy and eager loading, as shapes",
			"Resolve a relation per row on demand, or for the whole page up front — the choice is a query-count decision, and Go makes you make it explicitly.",
			text("loading-pitch",
				"Lazy and eager describe WHEN a relation's rows are fetched. Lazy resolves "+
					"each relation at the moment it is touched — per parent, on demand; eager "+
					"resolves the relation for the whole result set up front, usually as one "+
					"batched query alongside the parents. In languages with field-interception "+
					"proxies (Hibernate's loaded-on-access collections), lazy loading is "+
					"TRANSPARENT — touch the field, SQL happens — which is precisely how N+1 "+
					"became folklore: the code looks innocent because the loop is invisible. "+
					"Go structs have no such machinery: a slice is populated when you populate "+
					"it, so lazy loading in GORM is the explicit association call the probe "+
					"measured, and the cost is visible at the call site. That visibility is "+
					"worth more than the convenience it replaces."),
			text("loading-eager",
				"The eager shape in GORM is ``Preload``: one declarative clause on the query, "+
					"and the relation arrives with the page. The probe's measured run shows "+
					"exactly what that costs — the page query, the join-table lookup, the "+
					"target-table lookup — and, just as important, what it does NOT cost: "+
					"extra statements per row. GORM also offers ``Joins`` mode for "+
					"single-valued relations (a ``belongs to`` flattened into the parent's "+
					"query — one statement total) and nested preloads (``Preload(\"Tags\")`` "+
					"chains through associations); the principle across all of them is the "+
					"same: declare the shape you need BEFORE the query runs, and let the "+
					"library batch it. The array_agg JOIN the baseline and sqlc run is the "+
					"hand-written extreme of the same idea — the relation never leaves the "+
					"statement at all:"),
			code("loading-eager-go", "go",
				"The fix, one clause: Preload batches the relation for the whole page.",
				"server/internal/data/ormprobe/gorm.go",
				"counter := newSessionCounter()\n"+
					"session := db.WithContext(ctx).Session(&gorm.Session{Logger: counter})\n"+
					"// …the clock starts with the query, the counter with the session\n"+
					"var posts []Post\n"+
					"if err := session.Preload(\"Tags\").Order(\"id\").Find(&posts).Error; err != nil {\n"+
					"	return nil, demos.ORMProbeTrialFacts{}, fmt.Errorf(\"gorm: preload the posts page: %w\", err)\n"+
					"}"),
			text("loading-choosing",
				"The decision rule falls out of the arithmetic rather than taste. Lists — "+
					"anything rendering a page of parents with their relations — go eager, "+
					"because per-row resolution multiplies by page size and then by "+
					"traffic. A single entity's detail view can resolve relations lazily "+
					"with a clear conscience: one parent, one or two relation queries, no "+
					"loop to multiply. When neither shape fits — huge pages, relations "+
					"needed for only some rows — batch by id set yourself: collect the "+
					"parents' ids, fetch the edges with ``WHERE post_id = ANY($1)`` in one "+
					"statement, group in memory. That is what Preload does internally; "+
					"doing it by hand is topic 17's territory, and the probe's numbers are "+
					"the argument for why any of it matters."),
		),
		// ── 5. Transactions across repositories ────────────────────
		section("transactions-across-repositories", "Transactions across repositories: the unit of work",
			"Two repositories, one transaction — the unit of work pattern that keeps a multi-resource write atomic without merging the repositories.",
			text("uow-pitch",
				"Topic 17 made every write atomic inside ONE repository — a post and its tag "+
					"edges commit together because one adapter owns both statements. The "+
					"harder requirement arrives the day one logical operation spans TWO "+
					"resources that live in different repositories (a post plus its audit "+
					"row, an order plus its inventory hold): the write must be all-or-"+
					"nothing ACROSS repositories, and merging them into one god-repository "+
					"to get there trades your layering for a transaction. The unit-of-work "+
					"pattern keeps both: one transaction object is the shared unit, and "+
					"each repository is constructed OVER it for the duration of the "+
					"operation — one resource per repository, one transaction per unit, "+
					"composition instead of consolidation. In GORM the transaction is a "+
					"``*gorm.DB`` with ``Transaction``'s callback scoping its lifetime; in "+
					"sqlc the same seam is ``Queries.WithTx`` — the generated queries run "+
					"against whatever DBTX you construct them over, a pool or a tx."),
			code("uow-repos-go", "go",
				"The repositories stay one-resource-each; the shared handle is what makes them one unit.",
				"server/internal/data/ormprobe/gorm.go",
				"type GormPostsRepository struct {\n"+
					"	db *gorm.DB\n"+
					"}\n"+
					"\n"+
					"func (r *GormPostsRepository) Create(ctx context.Context, post *Post) error {\n"+
					"	return r.db.WithContext(ctx).Create(post).Error\n"+
					"}"),
			code("uow-transaction-go", "go",
				"The unit of work: both repositories built over tx, one deliberate failure, everything unwinds.",
				"server/internal/data/ormprobe/gorm.go",
				"txErr := db.WithContext(ctx).Session(&gorm.Session{Logger: counter}).Transaction(func(tx *gorm.DB) error {\n"+
					"	posts := &GormPostsRepository{db: tx}\n"+
					"	tags := &GormTagsRepository{db: tx}\n"+
					"\n"+
					"	scratch := &Post{\n"+
					"		Title: \"  unit-of-work scratch  \",\n"+
					"		Body:  \"two repositories, one transaction, one deliberate failure\",\n"+
					"	}\n"+
					"	// …create, attach the scratch tag, read the row back inside the unit\n"+
					"	return errors.New(\"deliberate failure: prove the whole unit unwinds\")\n"+
					"})"),
			text("uow-read",
				"Watch the exhibit's steps in the demo payload and the pattern's mechanics "+
					"are all visible: the read-back INSIDE the transaction sees the scratch "+
					"post with its hook-trimmed title (in-transaction reads see the unit's "+
					"own writes — topic 17's isolation section in miniature), the deliberate "+
					"error makes ``Transaction`` roll back, and the counts before and after "+
					"prove nothing leaked — 12 posts before, 12 after, every time. The "+
					"callback shape is the quiet win: rollback is not a code path you write "+
					"per repository, it is what ``Transaction`` does whenever the closure "+
					"returns non-nil, so the error handling you already write IS the "+
					"atomicity guarantee."),
			text("uow-sequence",
				"One trap the probe hit while being built is worth its own paragraph, "+
					"because production hits it too. The seed inserts posts with EXPLICIT "+
					"ids into a ``GENERATED BY DEFAULT AS IDENTITY`` column — and explicit "+
					"ids do not advance the identity sequence. The first GORM insert, which "+
					"omits the id and asks the identity to mint one, generated 1 and "+
					"collided with the seeded row: ``duplicate key value violates unique "+
					"constraint``. The fix is the textbook companion to explicit-id "+
					"seeding — sync the sequence to the seeded maximum with ``setval`` — "+
					"and the probe runs it as part of its idempotent setup. The general "+
					"lesson outlives the demo: when two writers mint ids for the same "+
					"table through different mechanisms, the mechanisms must agree, and "+
					"the database will tell you exactly when they don't."),
		),
		// ── 6. testcontainers-go ───────────────────────────────────
		section("testcontainers-go", "testcontainers-go: real PostgreSQL in tests",
			"The only database that proves data-layer behavior is a database — testcontainers gives every test its own throwaway one.",
			text("tc-pitch",
				"A repository tested against a fake proves the fake; the behaviors this "+
					"data phase teaches — statement counts, transaction visibility, identity "+
					"sequences, GORM's generated SQL — only exist against a real PostgreSQL. "+
					"testcontainers-go is the mechanism that makes that affordable: the test "+
					"asks the Docker daemon for a container, waits for it to be genuinely "+
					"ready, and terminates it in cleanup, so every test runs against an "+
					"isolated, disposable database with the production engine's exact "+
					"semantics. The guide's wiring — the host daemon's socket mounted into "+
					"the dev container plus ``TESTCONTAINERS_HOST_OVERRIDE`` so published "+
					"ports resolve — landed with the smoke test this phase; the orm-probe's "+
					"tests are its first full consumer:"),
			code("tc-spawn-go", "go",
				"The whole spin-up: postgres:17, ready-waited on the SECOND ready log, Ryuk-reaped.",
				"server/internal/data/ormprobe/ormprobe_test.go",
				"container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{\n"+
					"	ContainerRequest: testcontainers.ContainerRequest{\n"+
					"		Image:        \"postgres:17\",\n"+
					"		ExposedPorts: []string{\"5432/tcp\"},\n"+
					"		Env: map[string]string{\n"+
					"			\"POSTGRES_USER\":     \"postgres\",\n"+
					"			\"POSTGRES_PASSWORD\": \"guide\",\n"+
					"			\"POSTGRES_DB\":       \"guide\",\n"+
					"		},\n"+
					"		WaitingFor: wait.ForLog(\"database system is ready to accept connections\").\n"+
					"			WithOccurrence(2).\n"+
					"			WithStartupTimeout(containerStartup),\n"+
					"	},\n"+
					"	Started: true,\n"+
					"})"),
			text("tc-ready",
				"Three details in that spawn carry the guide's scars. The image is the "+
					"STACK'S OWN pinned postgres:17, already on the host daemon — no pulls, "+
					"no version drift between the database you test and the one you run. "+
					"The wait strategy counts the ready log's SECOND occurrence, because "+
					"postgres logs it once for the init phase and once for the real server, "+
					"and only the second one answers queries. And termination is belt-and-"+
					"braces: the test's cleanup terminates the container, and Ryuk — the "+
					"reaper sidecar testcontainers runs — collects anything a crashed run "+
					"leaves behind, so CI never rots under orphaned databases."),
			code("tc-pinned-go", "go",
				"The proof the tests pin: measured numbers, derived by running, never guessed.",
				"server/internal/data/ormprobe/ormprobe_test.go",
				"pinned := map[string]int{\n"+
					"	demos.ORMProbeStrategyPgx:       1,  // one explicit JOIN\n"+
					"	demos.ORMProbeStrategySQLC:      1,  // one generated call site\n"+
					"	demos.ORMProbeStrategyGormLazy:  13, // 1 + 12: the N+1\n"+
					"	demos.ORMProbeStrategyGormEager: 3,  // page + join-table IN + tags IN\n"+
					"}"),
			text("tc-discipline",
				"The pinned-numbers test is this topic's whole epistemology in one file: "+
					"every constant in that map was first OBSERVED by running against the "+
					"real container, then written down — the eager count is 3 precisely "+
					"because the folkloric \"Preload costs one extra query\" turned out to "+
					"depend on the association's shape when measurement met reality. Around "+
					"it sit the discipline tests: a second full probe run must reproduce "+
					"every number (idempotency, no residue), and a deliberately unreachable "+
					"DSN must fail fast with an error the handler maps to the honest 503. "+
					"On machines without the docker socket the tests SKIP rather than fail — "+
					"the suite's green must mean \"correct\", never \"this machine happens "+
					"to have Docker\". The costs are real and bounded: a few seconds per "+
					"container (image cached), a Docker dependency in the test environment, "+
					"and enough discipline to keep such tests few and meaningful — unit "+
					"logic stays on fakes; the database-backed suite is for behavior only "+
					"a database can prove."),
		),
		// ── 7. Choosing honestly ───────────────────────────────────
		section("choosing-honestly", "Choosing honestly: pgx, sqlc, or GORM",
			"All three are the right answer somewhere — the decision rides on query complexity, team fluency, and how much SQL you can afford not to see.",
			text("choose-pitch",
				"The probe's numbers could suggest a league table; resist it. The measured "+
					"job — one page, one relation — is sqlc and raw pgx's home ground, and "+
					"the ORM still came within three statements of the fix it needed one "+
					"clause to reach. Real systems run all three KINDS of workload: hot "+
					"analytical reads where the SQL IS the product, CRUD-dense resources "+
					"where the ORM's compression pays daily, and mid-sized query surfaces "+
					"where typed generated code removes the scan plumbing without hiding "+
					"the statement. The honest chooser prices what each style costs where "+
					"the others don't: raw pgx costs you every ``Scan``; sqlc costs a "+
					"toolchain step and generated files in review; GORM costs the "+
					"conventions-and-clauses knowledge and the readiness to read SQL you "+
					"never wrote. The guide's own stance is the layered one this topic "+
					"exhibits: hand-written pgx as the backbone you must be able to fall "+
					"to, sqlc where the query surface is stable enough to generate, GORM "+
					"where a resource's shape is exactly the ORM's shape — and the "+
					"N+1-hunting measurement habits regardless of the choice."),
			list("choose-table", false,
				"Raw pgx — when the SQL is the product: CTEs, window functions, EXPLAIN-tuned reads, batch pipelines; total visibility, total responsibility for the typing.",
				"sqlc — when the query surface is stable and repetition is the pain: typed calls, drift caught at compile time, committed output; SQL-first teams keep their tuning.",
				"GORM — when CRUD density is the workload and the schema is conventional: associations, hooks, migrations compression; budget the conventions knowledge and the statement log.",
				"The measurement habit — whichever you choose: count statements on lists, EXPLAIN the slow one, and distrust any number you have not observed on your own data."),
			text("choose-close",
				"One closing observation about how these tools compose rather than "+
					"compete, because the probe embodies it: the pgx pool, the sqlc "+
					"queries, and the GORM session in the measured exhibit all ran over "+
					"ONE connection source — sqlc's ``DBTX`` is the pool, and GORM rides "+
					"pgx's ``database/sql`` adapter of the same pool. Choosing sqlc for "+
					"one repository does not evict pgx from another; adopting GORM for "+
					"one resource does not forbid a sqlc query beside it. The seam that "+
					"makes the mix safe is the one this guide has taught since topic 4: "+
					"ports at the domain, adapters per style at the data layer, one "+
					"composition root deciding which answers which resource. The next "+
					"topics keep that seam while changing the engine entirely — documents "+
					"in MongoDB, then structures in Redis — where the same repository "+
					"discipline decides what \"the data layer\" means when it is not "+
					"SQL at all."),
		),
		// ── 8. Terminal companion ──────────────────────────────────
		section("terminal-companion", "Terminal companion: the curl and generate tour",
			"The topic from the shell: run the probe, read its counts, regenerate sqlc's output and prove it didn't drift, then watch the testcontainers suite pin the same numbers.",
			text("term-pitch",
				"With the stack up (server on :8100, PostgreSQL migrated and seeded at "+
					"boot), the tour runs the probe from the shell, pulls the statement "+
					"counts out of the payload, regenerates the sqlc output and proves "+
					"the committed code did not drift, and runs the testcontainers suite "+
					"that pins the same numbers against a throwaway postgres:17. Every "+
					"claim in the topic is one of those commands away:"),
			code("term-probe", "bash", "", "",
				"# the probe, live: dataset, four trials, statement counts and receipts\n"+
					"curl -s localhost:8100/api/demo/db/orm-probe | python3 -m json.tool | head -30"),
			code("term-counts", "bash", "", "",
				"# the statement column only — the numbers this topic is made of\n"+
					"curl -s localhost:8100/api/demo/db/orm-probe \\\n"+
					"  | python3 -c 'import json,sys; d=json.load(sys.stdin); \\\n"+
					"               [print(t[\"strategy\"], t[\"statements\"], t[\"roundTripRatio\"]) for t in d[\"trials\"]]'; \\\n"+
					"  echo \"results agree:\", $(curl -s localhost:8100/api/demo/db/orm-probe | python3 -c 'import json,sys; print(json.load(sys.stdin)[\"resultsAgree\"])')"),
			code("term-sqlc", "bash", "", "",
				"# regenerate sqlc's output and prove the committed code did not drift\n"+
					"# (sqlc verify needs its cloud service; regen + git diff is the local check)\n"+
					"docker exec go_developers_guide_dev bash -lc \\\n"+
					"  'cd /workspaces/go_developers_guide/server/internal/data/ormprobe && go tool sqlc generate'\n"+
					"git diff --exit-code -- server/internal/data/ormprobe && echo 'generated code in sync'"),
			code("term-tests", "bash", "", "",
				"# the testcontainers suite: a throwaway postgres:17, the probe run against it\n"+
					"docker exec go_developers_guide_dev bash -lc \\\n"+
					"  'cd /workspaces/go_developers_guide/server && go test -race -v -run TestDBORMProbe ./internal/data/ormprobe/'"),
			text("term-coda",
				"Notice what the tour keeps proving: the demo's counts and the test's pins "+
					"are the SAME numbers, derived the same way — against a real PostgreSQL, "+
					"counted through each library's own seam. That is the data phase's "+
					"contract with you, extended one tool further: not \"the ORM is slow\" "+
					"but \"the loop costs 13 statements, here is the receipt, here is the "+
					"clause that collapses it to 3\". The next topic swaps the relational "+
					"engine for documents entirely, and the discipline travels unchanged."),
		),
	}

	if buildErr != nil {
		return Topic{}, buildErr
	}
	return NewTopic(
		"sqlc-gorm",
		"sqlc and GORM: typed queries and the ORM, measured",
		"sqlc's SQL-first code generation and GORM's full ORM, run against the same "+
			"schema — with the N+1 query problem measured live (statements counted, not "+
			"opined), lazy versus eager loading as shapes, the unit-of-work pattern for "+
			"transactions across repositories, and testcontainers-go pinning every number "+
			"against a real PostgreSQL.",
		"data",
		18,
		sections,
	)
}
