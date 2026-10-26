package content

import (
	"errors"
	"fmt"
)

// This file is topic 17 databases-sql (group data, curriculum order 17),
// the guide's largest topic: everything between "a schema exists" and "a
// query builder earns its keep" — connecting and pooling, the DAO
// discipline with parameterized SQL, the fake/live repository swap this
// topic makes REAL for every API surface, transactions and savepoints,
// SQL by hand (CTEs, window functions), reading EXPLAIN plans, full-text
// search, isolation and locking, and audit rows that ride the changing
// transaction. Six demos run it all live; the terminal companion flips
// the whole API between the in-memory fake and PostgreSQL with one PATCH.

// databasesSQLTopic builds topic 17.
func databasesSQLTopic() (Topic, error) {
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
		// ── 1. Connecting: database/sql and the pgx pool ────────────
		section("connecting-and-pools", "Connecting: database/sql and the pgx pool",
			"The pool is the process-wide owner of database connections — sized once, health-checked continuously, and shared by everything.",
			text("pool-pitch",
				"Go ships ``database/sql``: a standard interface over SQL drivers, built entirely around a "+
					"connection pool it manages for you. The pgx v5 driver this guide pins speaks that interface "+
					"AND offers its own native API, and the choice between them is worth making explicitly. "+
					"``database/sql`` buys portability (swap drivers, keep code) and ecosystem compatibility "+
					"(goose's migration runner rides it); pgx's native API buys PostgreSQL-specific power — "+
					"typed parameters, ``RETURNING`` scans, batch pipelining, and a pool with per-knob control. "+
					"This guide uses both, deliberately: goose runs through the ``database/sql`` adapter OVER "+
					"the same pgx pool (topic 16 showed that adapter — ``stdlib.OpenDBFromPool`` — one pool, "+
					"two interfaces, zero double pooling), while the application's repositories use the native "+
					"pool directly. What you must never do is open a connection per request: a connection is a "+
					"TCP socket, an authenticated session, and a backend process on the database side — "+
					"expensive to build, miserable to churn."),
			code("pool-lifecycle-go", "go",
				"The pool's whole lifecycle in the guide's own dialer: construct, prove with a ping, fail fast.",
				"server/internal/data/db/db.go",
				"func NewPool(ctx context.Context, dsn string) (*pgxpool.Pool, error) {\n"+
					"    pool, err := pgxpool.New(ctx, dsn)\n"+
					"    if err != nil {\n"+
					"        return nil, fmt.Errorf(\"parse database url: %w\", err)\n"+
					"    }\n"+
					"    pingCtx, cancel := context.WithTimeout(ctx, connectTimeout)\n"+
					"    defer cancel()\n"+
					"    if err := pool.Ping(pingCtx); err != nil {\n"+
					"        pool.Close()\n"+
					"        return nil, fmt.Errorf(\"ping database: %w\", err)\n"+
					"    }\n"+
					"    return pool, nil\n"+
					"}"),
			text("pool-sizing",
				"Sizing is the question everyone asks first and the pool answers with three numbers. "+
					"``MaxConns`` (default: the larger of 4 and your CPU count) caps total connections — "+
					"remember PostgreSQL's own ``max_connections`` is shared by EVERY process talking to it, "+
					"so twenty app replicas times a hundred pool slots is a outage plan, not a config. "+
					"``MinConns`` keeps warm connections so bursts don't pay the dial-and-authenticate tax. "+
					"``MaxConnLifetime`` retires connections before proxies, load balancers, or the database "+
					"itself silently drops long-lived ones. The guide runs the defaults — the pool is shared "+
					"by the posts store and every demo, and the guide's concurrency is a demo audience, not a "+
					"load test — and the honest sizing rule is: start with defaults, measure saturation "+
					"(``pool.Stat()`` reports acquires, waits, and idle time), and change numbers only with a "+
					"measurement in hand."),
			text("pool-health",
				"Health is continuous, not a boot-time checkbox. Acquired connections get checked: pgx "+
					"validates a connection when it has been idle past ``MaxConnIdleTime`` and transparently "+
					"re-dials when a healthy-looking connection turns out to be dead (``healthCheckPeriod`` "+
					"governs the background sweep). What the application owes in return is discipline, and "+
					"it is one rule: whatever you acquire, release. ``pool.Query``/``QueryRow``/``Exec`` "+
					"manage the round trip for you; the explicit ``Acquire`` (the race demo uses it for two "+
					"SIMULTANEOUS transactions) must be balanced with ``Release``, best through ``defer`` — "+
					"and every ``rows`` you open must be closed, because a lingering ``rows`` pins its "+
					"connection out of the pool until garbage collection happens to notice."),
			list("pool-laws", false,
				"One pool per process — every data package receives it from the composition root, never a DSN, never its own dialing.",
				"Fail fast at boot — an unreachable database is a boot failure in seconds, not a first-request surprise.",
				"Size from measurements — pool.Stat()'s waits and saturation, not from folklore about max_connections.",
				"Release what you acquire — defer Release, close rows, and let the pool — not the GC — own connection lifetimes.",
				"Contexts all the way down — every pool call takes the caller's ctx; a canceled request must free its connection immediately."),
		),
		// ── 2. The DAO: parameterized SQL or nothing ────────────────
		section("dao-and-parameterized-sql", "The DAO: parameterized SQL or nothing",
			"The repository IS the DAO: one adapter owning every statement for one resource — and every value crosses the wire as a parameter, never as text.",
			text("dao-pitch",
				"The DAO discipline — data access object, one place that owns the SQL for one resource — is "+
					"what this guide's layering has been building toward since topic 11: the port declares "+
					"WHAT (list, get, create, replace, delete), the adapter owns HOW (the statements). The "+
					"PostgreSQL adapter below is that HOW, and its one non-negotiable law is visible in "+
					"every statement it runs: values cross as PARAMETERS. In pgx that means ``$1``, ``$2`` "+
					"placeholders with the values passed alongside, which the driver sends to PostgreSQL "+
					"through the extended query protocol — the SQL text and the data travel as separate "+
					"messages, and the server parses the statement without ever seeing the values in it."),
			text("dao-injection",
				"Why that law is absolute: the moment a value is concatenated into SQL text, the input "+
					"controls the GRAMMAR. The classic exhibit — a title of ``x' OR '1'='1`` — turns "+
					"``WHERE title = '...'`` into a tautology that matches every row; the nastier one — "+
					"``'; DROP TABLE posts; --`` — tries to end the statement and start another. Neither "+
					"is clever; both are decades old; both still work on code that builds SQL with "+
					"``fmt.Sprintf``. Parameterization ends the entire bug class by construction: the "+
					"attack string arrives as a VALUE the planner compares against titles, matching none, "+
					"and the DROP payload compares as a very weird title that nobody stored. The demo "+
					"below runs both attacks against the live posts table and reports exactly that:"),
			code("dao-runner-go", "go",
				"The demo's runner: each attack executes as $1 — the statement text never contains the input.",
				"server/internal/data/db/sql_demos.go",
				"attempts := make([]demos.SQLSafetyAttempt, 0, len(attacks))\n"+
					"for _, attack := range attacks {\n"+
					"    var matched int\n"+
					"    if err := r.pool.QueryRow(ctx, lookupSQL, attack).Scan(&matched); err != nil {\n"+
					"        return demos.SQLSafetyObservation{}, fmt.Errorf(\"run the parameterized lookup: %w\", err)\n"+
					"    }\n"+
					"    // …the total count afterwards proves the table the DROP payload wanted is intact\n"+
					"}"),
			demo("db:sql-safety", "Parameterized vs concatenated",
				"Two canonical injection attacks — the OR tautology and the DROP payload — executed against "+
					"the live posts table as $1 parameters: zero rows matched, six posts still standing. "+
					"Beside each live outcome sits the anti-pattern exhibit: the exact SQL string "+
					"concatenation WOULD have produced, displayed, never run."),
			text("dao-read",
				"Read the demo's two halves against each other. The parameterized column is what the "+
					"database actually did moments ago: the hostile string went in as a bind value, "+
					"matched no title, and left the table fully populated. The ``assembledSql`` column is "+
					"the museum piece — what ``fmt.Sprintf`` would have shipped to the server — shown so "+
					"the failure mode is visible without ever being executed. One nuance worth carrying: "+
					"pgx would also have refused the DROP payload even concatenated, because the extended "+
					"protocol runs ONE statement per call — but that is a fortunate property of the "+
					"transport, not a security strategy. Parameterize because the grammar must never be "+
					"for sale, and let the protocol's single-statement rule be a seatbelt you never test."),
			text("dao-shape",
				"Beyond safety, the adapter earns its keep by owning the statement SHAPES the port "+
					"promises. The list below is the whole read path: the filter is an exact tag match "+
					"through the edge table (a subquery, so the totals count POSTS and not edges), the "+
					"sort comes from a fixed map over the closed vocabulary — never interpolated from "+
					"user input — and the tag aggregation rides ``array_agg`` so one round trip carries "+
					"the post AND its tags:"),
			code("dao-list-go", "go",
				"The DAO's read shape: parameterized filter, fixed ORDER BY, tags aggregated in the same round trip.",
				"server/internal/data/posts/pg.go",
				"rows, err := tx.Query(ctx, fmt.Sprintf(`\n"+
					"    SELECT p.id, p.title, p.body, p.created_at, p.updated_at,\n"+
					"           COALESCE(array_agg(pt.tag_name ORDER BY pt.tag_name)\n"+
					"                    FILTER (WHERE pt.tag_name IS NOT NULL), '{}') AS tags\n"+
					"    FROM posts p\n"+
					"    LEFT JOIN post_tags pt ON pt.post_id = p.id\n"+
					"    WHERE ($1 = '' OR p.id IN (SELECT post_id FROM post_tags WHERE tag_name = $1))\n"+
					"    GROUP BY p.id, p.title, p.body, p.created_at, p.updated_at\n"+
					"    ORDER BY %s\n"+
					"    LIMIT $2 OFFSET $3`, order),\n"+
					"    query.Tag, query.Limit, (query.Page-1)*query.Limit)"),
			text("dao-orderby",
				"That ``fmt.Sprintf`` deserves its own sentence, because it looks like the bug this "+
					"section just finished condemning. The ORDER BY clause cannot be a ``$n`` parameter — "+
					"PostgreSQL does not accept identifiers or keywords as bind values — so it is "+
					"interpolated from ``listOrderBy``, a FIXED map from the validated sort vocabulary "+
					"to constant strings. User input never reaches the Sprintf; the vocabulary was "+
					"closed before the repository was consulted. This is the honest pattern for every "+
					"dynamic-SQL corner: interpolate only from your own constants, and let everything "+
					"that ever touched the request travel as a parameter."),
		),
		// ── 3. One port, two stores ─────────────────────────────────
		section("one-port-two-stores", "One port, two stores: the fake/live switch, for real",
			"The posts API now answers from the in-memory fake or PostgreSQL — chosen per call from the scenario toggle, flipped at runtime, identical on the wire.",
			text("swap-pitch",
				"Every API topic since 11 has promised the same thing: the posts routes depend on a PORT, "+
					"the in-memory adapter is a stand-in, and a PostgreSQL adapter will slot in \"behind "+
					"the same port\" without the domain or handlers learning which store answered. This "+
					"topic pays that promise. The adapter is ``internal/data/posts/pg.go`` over exactly "+
					"the schema topic 16 designed: ``posts``, ``tags``, ``post_tags``, and the "+
					"``updated_at`` trigger — the adapter sets ``created_at`` itself (the domain owns the "+
					"draft's arrival instant) and lets the TRIGGER stamp ``updated_at`` on every UPDATE, "+
					"reading both back through ``RETURNING`` so the caller gets what the database states, "+
					"not what the app assumes:"),
			code("swap-create-go", "go",
				"The live adapter's Create: mint the UUIDv7, insert, attach tag edges — one transaction, stamps returned by the database.",
				"server/internal/data/posts/pg.go",
				"func (r *PostgresRepository) Create(ctx context.Context, draft posts.Draft) (posts.Post, error) {\n"+
					"    id, err := r.newID()\n"+
					"    // …parseUUID validates the minted id into its 16 bytes\n"+
					"    tx, err := r.pool.Begin(ctx)\n"+
					"    if err != nil {\n"+
					"        return posts.Post{}, unavailable(err)\n"+
					"    }\n"+
					"    defer tx.Rollback(ctx)\n"+
					"\n"+
					"    tags := uniqueTags(draft.Tags)\n"+
					"    var createdAt, updatedAt time.Time\n"+
					"    if err := tx.QueryRow(ctx, `\n"+
					"        INSERT INTO posts (id, title, body, created_at, updated_at)\n"+
					"        VALUES ($1, $2, $3, $4, $4)\n"+
					"        RETURNING created_at, updated_at`,\n"+
					"        key, draft.Title, draft.Body, now).Scan(&createdAt, &updatedAt); err != nil {\n"+
					"        return posts.Post{}, unavailable(err)\n"+
					"    }\n"+
					"    if err := attachTags(ctx, tx, key, tags); err != nil {\n"+
					"        return posts.Post{}, unavailable(err)\n"+
					"    }\n"+
					"    if err := tx.Commit(ctx); err != nil {\n"+
					"        return posts.Post{}, unavailable(err)\n"+
					"    }\n"+
					"    // …the stored post, stamped exactly as the database states it\n"+
					"}"),
			text("swap-seed",
				"The seed makes the two stores serve the SAME starting world. It is deliberately NOT a "+
					"migration — migrations carry structure and are append-only forever; the seed carries "+
					"content and must be re-runnable every boot, which ``ON CONFLICT DO NOTHING`` makes "+
					"it: a fresh database inserts the six guide posts (ids are fixed UUIDv7 literals "+
					"carrying the posts' January 2026 creation instants), every later boot inserts "+
					"nothing. The whole seed rides one transaction, so a half-seeded database is never "+
					"observable — the same all-or-nothing rule every write in this adapter follows."),
			text("swap-facade",
				"The switch itself is the smallest interesting object in the topic: a facade over the "+
					"port that consults the scenario store's data mode PER CALL. Not per request, not "+
					"per boot — per call: one snapshot read at method entry decides which adapter "+
					"answers, so a flip lands strictly BETWEEN two calls and no logical operation ever "+
					"tears across two stores. The fallback direction is deliberate too: only live mode "+
					"AND a wired live adapter reaches PostgreSQL; anything else answers from the fake, "+
					"so a demo stack with a wedged database still serves every surface honestly from memory:"),
			code("swap-pick-go", "go",
				"The whole switch: one snapshot read, one choice, per call.",
				"server/internal/data/posts/switching.go",
				"func (s *SwitchingRepository) pick() posts.PostsRepository {\n"+
					"    if s.scenario.Snapshot().DataMode == core.DataModeLive && s.live != nil {\n"+
					"        return s.live\n"+
					"    }\n"+
					"    return s.fake\n"+
					"}"),
			code("swap-main-go", "go",
				"The composition root: one facade behind five use cases — REST, GraphQL, and gRPC share them all.",
				"server/cmd/guide/main.go",
				"postsRepo := postsdata.NewSwitchingRepository(\n"+
					"    scenarioStore,\n"+
					"    postsdata.NewInMemoryRepository(),\n"+
					"    postsdata.NewPostgresRepository(pool),\n"+
					")"),
			text("swap-flip",
				"Now read that wiring and feel its weight: the REST routes, ``POST /api/graphql``, and "+
					"the gRPC server were all constructed over the SAME five use-case instances, and "+
					"those instances hold this facade — so ``PATCH /api/demo/scenario`` with "+
					"``{\"dataMode\":\"live\"}`` flips every surface at once, mid-traffic, no restart. "+
					"The terminal companion walks the full round trip; the shape of it is: flip live, "+
					"``POST /api/posts`` (the id comes back a UUIDv7 minted app-side), read it back, "+
					"flip fake — the post is GONE, because the fake store never saw it — flip live "+
					"again and it is BACK, because PostgreSQL had it all along. Two stores, one wire "+
					"contract, zero code paths that know the switch happened. The api-design topic's "+
					"posts workbench plays the same game through the browser: leave it in fake mode "+
					"and its CRUD answers from memory; flip live in the scenario panel and the very "+
					"next mutation writes PostgreSQL — and a GraphQL ``createPost`` mutation lands in "+
					"the same place, because the resolver holds the same use case the REST handler does."),
			text("swap-parity",
				"What makes the switch honest rather than a party trick is the shared behavior table: "+
					"one test file drives BOTH adapters through the same cases — the seed and its "+
					"newest-first order, the page math's partial and past-the-end pages, the exact tag "+
					"filter, the strict-404 delete (including ids that are not even canonical UUIDs: "+
					"a miss is a miss), PUT's preserve-createdAt/move-updatedAt contract, the sorted "+
					"tag set both stores keep, and the validation that lives in the shared use case "+
					"and never reaches either store. The PostgreSQL runs execute against throwaway "+
					"databases the tests create and drop themselves — never the dev database. A rule "+
					"the fake honors and the live store forgot (or vice versa) fails there first, "+
					"which is what lets the runtime flip be a demo instead of a liability."),
		),
		// ── 4. Transactions ─────────────────────────────────────────
		section("transactions", "Transactions: ACID in practice",
			"BEGIN to COMMIT is one all-or-nothing unit — and savepoints let you unwind part of it without throwing the unit away.",
			text("tx-pitch",
				"ACID is four words — atomicity, consistency, isolation, durability — and three of them "+
					"are mostly the database's day job. Consistency is your constraints enforced (topic "+
					"16 wrote them); durability is the WAL's promise (committed means survived); "+
					"isolation gets its own section below. ATOMICITY is the one your code wields: "+
					"everything between ``BEGIN`` and ``COMMIT`` happens together or not at all, and "+
					"the write path above already leaned on it twice — a post and its tag edges commit "+
					"as one unit, the seed's six posts as one unit. In pgx the shape is always the "+
					"same three moves: ``Begin``, ``defer Rollback``, work, ``Commit``. The deferred "+
					"rollback is not pessimism; it is the error path's cleanup — a committed "+
					"transaction's rollback is a harmless no-op, but an abandoned transaction without "+
					"one pins a connection and holds locks until the pool reclaims it."),
			text("tx-savepoint",
				"Savepoints are the refinement everyone meets eventually: a BOOKMARK inside the "+
					"transaction you can unwind to without ending it. ``SAVEPOINT sp1`` marks the "+
					"spot; ``ROLLBACK TO SAVEPOINT sp1`` undoes everything after it while keeping the "+
					"transaction open and its earlier work intact. The classic use is a multi-step "+
					"operation that can retry ONE step — parse a batch, savepoint per item, roll back "+
					"the failing item's rows, keep the rest — and the caution is that a savepoint "+
					"undoes ROWS, not session state or sequence consumption. The demo runs the whole "+
					"story as data: one transaction, two scratch inserts, a savepoint between them, "+
					"and the post count observed at every step — 6, 7, 7, 8, then 7 when the savepoint "+
					"unwinds the second insert, then 6 after the final ROLLBACK confirms the database "+
					"never saw any of it:"),
			code("tx-story-go", "go",
				"The demo's choreography: every step records the count the transaction itself observes.",
				"server/internal/data/db/sql_demos.go",
				"steps = append(steps, demos.TxStoryRow{Action: \"begin\", SQL: \"BEGIN\", Count: count(tx)})\n"+
					"// …the first scratch insert lands, then:\n"+
					"if _, err := tx.Exec(ctx, \"SAVEPOINT sp1\"); err != nil {\n"+
					"    return demos.TxStoryObservation{}, fmt.Errorf(\"savepoint: %w\", err)\n"+
					"}\n"+
					"steps = append(steps, demos.TxStoryRow{Action: \"savepoint sp1\", SQL: \"SAVEPOINT sp1\", Count: count(tx)})\n"+
					"// …the second scratch insert, then the unwind and the final ROLLBACK"),
			demo("db:migrations", "The transaction story, with goose's ledger",
				"A live BEGIN / INSERT / SAVEPOINT / INSERT / ROLLBACK-TO-SAVEPOINT / ROLLBACK dance on "+
					"the guide's own posts table, narrated by the row count observed inside the "+
					"transaction at every step — beside goose_db_version, the migration ledger that "+
					"makes re-running the schema's history a no-op. The story rolls back entirely; "+
					"the running demo leaves nothing behind."),
			text("tx-discipline",
				"The demo also closes the loop the migration-history section of topic 16 opened: the "+
					"ledger it serves alongside the story is the same ``goose_db_version`` table the "+
					"schema explorer shows — version 0's baseline row plus one row per applied "+
					"migration — and goose runs EACH migration inside its own transaction, which is "+
					"why a failed migration leaves the database exactly where it was. Transactions "+
					"are not only for your writes; they are the mechanism the schema's own history "+
					"rides on. Keep units short, keep them focused (a transaction is a unit of WORK, "+
					"not a unit of user think-time — never hold one open across an external call), "+
					"and let the deferred rollback do its quiet job on every error path."),
		),
		// ── 5. Query builders and ORMs, by comparison ───────────────
		section("builders-and-orms", "Query builders and ORMs, by comparison",
			"This guide writes SQL by hand — but the alternatives are real tools with real trade-offs, and topic 18 builds with two of them.",
			text("orm-pitch",
				"Hand-written SQL is this topic's medium, and the sections around this one make the "+
					"case for it: the statement is visible, the planner is reachable, and nothing "+
					"stands between the code and what the database actually receives. The honest "+
					"counter-case is that SQL strings are untyped at the call site — a rename in the "+
					"schema is a runtime discovery — and that the same CRUD shapes repeat. The Go "+
					"ecosystem answers along a spectrum, and the guide's stance is a spectrum "+
					"stance too: stdlib-shaped code with hand-written SQL as the backbone, plus two "+
					"deliberate points on the spectrum built properly in topic 18."),
			list("orm-spectrum", false,
				"Hand-written SQL (this topic) — full control, full visibility; the cost is typing rows by hand and owning every statement's correctness.",
				"Query builders (squirrel-style) — compose SQL in Go types, gaining refactorability; the SQL is still yours to know, just spelled in method calls.",
				"sqlc (topic 18) — write SQL, GENERATE the typed Go: queries checked against the schema at compile time; the deliberate midpoint this guide adopts.",
				"GORM (topic 18) — a full ORM: structs become tables, associations become methods; maximum speed-to-CRUD, at the price of SQL you must still understand when it misbehaves.",
				"The failure mode to avoid — an ORM used as a reason not to learn SQL: the N+1 query, the accidental full-table load, and the unmappable plan all wait behind that door."),
			text("orm-n1",
				"One comparison deserves its own paragraph because it will find you in every camp: "+
					"the N+1 query. List posts, then fetch each post's tags in a loop, and six posts "+
					"cost seven round trips — invisible at demo scale, catastrophic at real scale, "+
					"and IDENTICAL in symptoms whether the loop is hand-written or ORM-generated "+
					"(ORMs merely make it a default). The fix is structural, not library-specific: "+
					"fetch the relation for the whole page in one query — the ``array_agg`` read in "+
					"the DAO section does exactly this — or batch by id set. Topic 18 replays this "+
					"exact lesson with sqlc and GORM side by side, on this same posts schema, so "+
					"the comparison is code you can run rather than an opinion to trust."),
		),
		// ── 6. SQL by hand: CTEs and windows ────────────────────────
		section("sql-by-hand", "SQL by hand: CTEs and window functions",
			"Two constructs do most of the heavy lifting beyond plain SELECTs: named pipelines (CTEs) and per-row analytics over ordered sets (windows).",
			text("cte-pitch",
				"A common table expression — ``WITH name AS (…)`` — names an intermediate result so "+
					"the next stage reads as its own explanation. The(tag-totals) exhibit below is "+
					"the pattern at its smallest: the CTE names \"the tag counts over the seeded "+
					"posts\", and the final SELECT merely orders them. At real sizes the same shape "+
					"replaces the alternative — subqueries nested until neither author nor reader "+
					"can name what any layer means. CTEs in PostgreSQL since v12 are inlined like "+
					"subqueries (the old materialization fence now requires ``MATERIALIZED``), so "+
					"readability no longer costs a plan; the recursive variant (``WITH RECURSIVE``) "+
					"walks trees and generates series — worth knowing exists before you need it."),
			text("window-pitch",
				"A window function computes ACROSS rows while KEEPING them — the mental model that "+
					"unlocks them is \"aggregate behavior, per-row output``. ``RANK() OVER (ORDER BY "+
					"length(body) DESC)`` attaches a standing to every post without collapsing the "+
					"table the way ``GROUP BY`` would; ``LAG(created_at) OVER (ORDER BY "+
					"created_at)`` hands each row its predecessor's timestamp, turning a timeline "+
					"into the DAYS BETWEEN posts — a fact no column stores. The ``OVER`` clause is "+
					"the whole trick: it defines the window (all rows, or per partition), the order "+
					"within it, and sometimes a frame; the function then runs over that window while "+
					"every row survives to the output."),
			code("lab-cte-sql", "sql",
				"Exhibit A: the CTE chain, pinned to the seeded ids so demo traffic cannot perturb it.",
				"server/internal/domain/demos/sql_lab.go",
				"WITH tagged AS (\n"+
					"    SELECT pt.tag_name AS tag, count(*) AS posts\n"+
					"    FROM post_tags pt\n"+
					"    WHERE pt.post_id = ANY($1::uuid[])\n"+
					"    GROUP BY pt.tag_name\n"+
					")\n"+
					"SELECT tag, posts FROM tagged ORDER BY tag"),
			code("lab-window-sql", "sql",
				"Exhibit C: LAG turns the timeline into gaps-in-days; exhibit B ranks by body length the same way.",
				"server/internal/domain/demos/sql_lab.go",
				"SELECT title,\n"+
					"       to_char(created_at, 'YYYY-MM-DD') AS created,\n"+
					"       (created_at::date - LAG(created_at::date) OVER (ORDER BY created_at)) AS days_since_previous\n"+
					"FROM posts\n"+
					"WHERE id = ANY($1::uuid[])\n"+
					"ORDER BY created_at"),
			demo("db:sql-lab", "The SQL lab",
				"Three exhibits over the six seeded posts, each served with the exact SQL that ran and "+
					"the rows it returned: the tag-totals CTE chain, RANK() over body length (ties "+
					"share a rank, every post survives), and LAG()-computed days between posts on the "+
					"seed's January 2026 timeline — 2, 3, 2, 3, 3 days, read straight off the dates."),
			text("lab-read",
				"Read the exhibit rows against the SQL and two habits form. First: the window "+
					"function's output column (``days_since_previous``) is NULL exactly once — the "+
					"oldest row has no predecessor — and the payload renders that null as an "+
					"explicit dash rather than letting an absent value masquerade as zero. Second: "+
					"the exhibits are pinned to the seeded ids by ``$1``, the same parameterization "+
					"law from the DAO section — even a read-only demo sends its (constant!) ids as "+
					"a parameter, because the shape of a demo is the shape its readers copy."),
		),
		// ── 7. Reading plans: EXPLAIN ───────────────────────────────
		section("reading-plans", "Reading plans: EXPLAIN and EXPLAIN ANALYZE",
			"The plan is the database's account of how it answered you — reading it is the difference between guessing and knowing why a query is slow.",
			text("plan-pitch",
				"``EXPLAIN`` in front of any query returns the PLAN PostgreSQL chose: a tree of nodes "+
					"(scans, joins, sorts) with cost estimates; ``EXPLAIN ANALYZE`` additionally RUNS "+
					"the query and reports actual times and row counts beside the estimates. The "+
					"search demo carries a real plan (plain ``EXPLAIN`` — the estimate-only form keeps "+
					"the payload deterministic); the terminal companion runs the ``ANALYZE`` form live "+
					"so you can watch estimates meet reality. Reading a plan is a small vocabulary:"),
			list("plan-vocabulary", false,
				"Seq Scan — every row examined; correct for small tables, the red flag on large ones.",
				"Index Scan / Index Only Scan — the btree (or gin) is walked; 'only' means the heap was never touched.",
				"Bitmap Heap Scan — the index built a bitmap of matching pages, then the heap is read once; the middle ground for selective queries.",
				"Nested Loop / Hash Join / Merge Join — the join strategies; hash builds a table in memory, merge needs sorted inputs, nested loop wins small sides.",
				"Sort — an ORDER BY with no index to serve it; the node the DESC index from topic 16 exists to eliminate.",
				"rows / cost — the planner's estimates; with ANALYZE, 'actual' columns beside them tell you where the planner guessed wrong (the signature of stale statistics)."),
			text("plan-honesty",
				"The most important habit in plan reading is respecting what the planner knows. On "+
					"six rows it will usually choose a Seq Scan over the FTS index — and it is RIGHT: "+
					"an index hop that saves nothing is not optimization, it is ceremony. The plan "+
					"changes its mind as tables grow and statistics sharpen, which is why the topic "+
					"teaches reading plans rather than memorizing one: run ``EXPLAIN ANALYZE`` on the "+
					"slow thing, find the node doing more work than the question warrants, and aim "+
					"the fix at that node — an index for the filter, a covering index for the "+
					"projection, a rewritten predicate the planner can actually use. The search "+
					"section's plan is the worked example: read it before moving on."),
		),
		// ── 8. Full-text search ─────────────────────────────────────
		section("full-text-search", "Full-text search with tsvector",
			"Stemmed inverted indexes make 'find me posts about logging errors' a question the INDEX answers — not a LIKE scan.",
			text("fts-pitch",
				"``LIKE '%log%'`` matches substrings and understands nothing: it misses \"logging\", "+
					"matches \"blog\", and reads every row to do it. PostgreSQL's full-text search "+
					"runs the other direction: ``to_tsvector`` reduces a document to its stemmed "+
					"lexemes (an inverted index structure — 'log' points at the rows containing it), "+
					"``to_tsquery`` reduces the QUERY the same way, and ``@@`` asks which vectors "+
					"match. Stemming is dictionary-based ('logging' and 'logs' both become 'log'; "+
					"stop words vanish), so the match is at the level of MEANING-units rather than "+
					"byte sequences. The query side has three entry points worth naming: "+
					"``plainto_tsquery`` ANDs every word (strict), ``websearch_to_tsquery`` accepts "+
					"the way people type (quoted phrases, ``or``, minus-exclusions — the demo's "+
					"choice), and raw ``to_tsquery`` takes the operators yourself (``& | !``) for "+
					"power users."),
			code("fts-migration-sql", "sql",
				"Migration 3: the search index as a real schema decision — a GIN over the computed tsvector.",
				"server/internal/data/db/migrations/00003_fts_index.sql",
				"-- The tsvector values are DERIVED, so the index is an EXPRESSION index —\n"+
					"-- PostgreSQL indexes the computed tsvector and the query matches when it\n"+
					"-- computes the SAME expression.\n"+
					"CREATE INDEX posts_search_idx ON posts\n"+
					"    USING gin (to_tsvector('english', title || ' ' || body));"),
			text("fts-index",
				"That migration is where the schema earns its search: without it, ``@@`` computes "+
					"the tsvector of every row per query; with it, the GIN index answers directly. "+
					"Two details carry real weight. The index is an EXPRESSION index — it indexes "+
					"the computed ``to_tsvector(title || ' ' || body)``, so every query must "+
					"compute the identical expression to use it (the demo's SQL does, verbatim). "+
					"And the dictionary is PINNED ('english' in both index and query) because an "+
					"immutable expression requires a fixed configuration — a real constraint you "+
					"meet the day you need multi-language search, where the answer is one index "+
					"per language or a language column in the expression. The concatenation also "+
					"flattens title and body into one unweighted vector — honest for a demo, and "+
					"the production refinement (``setweight`` on the title's vector) is exactly "+
					"the kind of upgrade tsvector's composable design makes a migration, not a "+
					"rewrite."),
			code("fts-query-sql", "sql",
				"The demo's statement: websearch syntax in, ranked rows out.",
				"server/internal/domain/demos/sql_search.go",
				"SELECT title,\n"+
					"       ts_rank(to_tsvector('english', title || ' ' || body),\n"+
					"               websearch_to_tsquery('english', $1)) AS rank\n"+
					"FROM posts\n"+
					"WHERE to_tsvector('english', title || ' ' || body)\n"+
					"      @@ websearch_to_tsquery('english', $1)\n"+
					"ORDER BY rank DESC, title"),
			demo("db:search", "Full-text search, ranked",
				"The fixed query \"logging or errors\" run live: websearch_to_tsquery stems it into "+
					"'log' | 'error', three seeded posts match (the slog post, the errors post, and "+
					"the pagination post whose body says \"not an error\"), ts_rank orders them by "+
					"match weight — and the payload carries the EXPLAIN plan PostgreSQL chose plus "+
					"the GIN index's own pg_get_indexdef rendering."),
			text("fts-read",
				"Read the ranking against the stems and it stops being magic: the errors post carries "+
					"'error' in both title and body (heavier weight), the slog post's 'log' sits in "+
					"its title, the pagination post scrapes one body mention — ts_rank turns those "+
					"facts into an order. The plan beside them is the plans section's lesson applied: "+
					"six rows, so the planner likely walked them — and the terminal companion's "+
					"``EXPLAIN ANALYZE`` on the same statement shows the honest numbers live. Rank "+
					"functions are tunable normalization choices (``ts_rank`` vs ``ts_rank_cd`` "+
					"cover-density; weight arrays boost fields), and the discipline is the one this "+
					"guide keeps applying: ship the simple version, read its plan, tune with a "+
					"complaint in hand."),
		),
		// ── 9. Isolation, locking, races ────────────────────────────
		section("isolation-and-locking", "Isolation, locking, and the races between transactions",
			"Two transactions, one row: what each can see of the other's uncommitted work is the isolation level — and the row lock is the serialization point.",
			text("iso-pitch",
				"Isolation answers a deceptively simple question: when two transactions overlap, what "+
					"does each SEE of the other's uncommitted writes? PostgreSQL's MVCC makes reads "+
					"non-blocking — readers never wait on writers, they see a consistent SNAPSHOT — "+
					"and the isolation level decides how often that snapshot refreshes. The default "+
					"READ COMMITTED takes a fresh snapshot per STATEMENT: you can never read dirty "+
					"(uncommitted) data, but two reads in one transaction can disagree if another "+
					"transaction committed between them — the nonrepeatable read. REPEATABLE READ "+
					"holds one snapshot per TRANSACTION (anomalies gone, but concurrent writers "+
					"may need retrying). SERIALIZABLE goes further and rejects combinations of "+
					"transactions whose INTERLEAVING could not have run serially — correctness by "+
					"aborted optimism, with the application contractually on the hook to RETRY."),
			text("iso-locking",
				"Locking is the writer-side half. Every UPDATE takes a row lock; a second writer "+
					"targeting the same row WAITS — that is serialization at the row, and it is "+
					"exactly what ``SELECT … FOR UPDATE`` lets you take EARLY, when you are about "+
					"to base a decision on rows you will change (read, decide, write, with nobody "+
					"changing the ground under you between read and write). The orchestration "+
					"problem is what makes races feel unbeatable in tests — so the demo cheats "+
					"honestly: two connections from the pool, every handoff signaled by a channel "+
					"or CONFIRMED through ``pg_stat_activity`` (the reader's wait observed in the "+
					"database's own activity view), never by sleeping and hoping. The observed "+
					"sequence is therefore deterministic: the reader sees the OLD title before the "+
					"commit, the NEW one after it, and the FOR UPDATE waits for the lock's holder "+
					"to let go:"),
			code("race-runner-go", "go",
				"The choreography's core: handoffs by signal and wait-state, not sleeps.",
				"server/internal/data/db/sql_demos.go",
				"locked := make(chan lockResult, 1)\n"+
					"go func() {\n"+
					"    var title string\n"+
					"    err := reader.QueryRow(ctx, `SELECT title FROM posts WHERE id::text = $1 FOR UPDATE`, scratch).Scan(&title)\n"+
					"    locked <- lockResult{title: title, err: err}\n"+
					"}()\n"+
					"\n"+
					"waiting := false\n"+
					"for range 2000 {\n"+
					"    var blocked int\n"+
					"    if err := r.pool.QueryRow(ctx,\n"+
					"        `SELECT count(*) FROM pg_stat_activity WHERE pid = $1 AND wait_event_type = 'Lock'`, readerPID,\n"+
					"    ).Scan(&blocked); err == nil && blocked == 1 {\n"+
					"        waiting = true\n"+
					"        break\n"+
					"    }\n"+
					"    // …the holder commits; the waiting read proceeds and reports what it saw\n"+
					"}"),
			demo("db:race", "Two transactions, choreographed",
				"A writer and a reader on two live connections, stepped through the phases: the "+
					"uncommitted UPDATE is invisible (read committed sees the old row), the COMMIT "+
					"makes it visible to the very next statement, and the FOR UPDATE exhibit shows "+
					"the reader genuinely WAITING on the row lock until the holder commits. The "+
					"serializable comparison rides along as the counterfactual — which transaction "+
					"it would have rejected, and why the app retries."),
			text("iso-read",
				"The payload's counterfactual is the part to sit with: under SERIALIZABLE, this "+
					"exact choreography cannot both succeed — the database would abort one "+
					"transaction with a serialization failure (SQLSTATE 40001), and the APPLICATION "+
					"would retry it. That retry loop is the price of serializable and the reason "+
					"read committed plus explicit locks is the default posture: locks you take "+
					"deliberately, at the smallest scope that makes the decision safe, instead of "+
					"aborts you handle everywhere. The lost-update bug this section quietly "+
					"prevents — read a value, decide, write, discover another writer moved it — is "+
					"the same bug the race detector catches in memory (topic 6); FOR UPDATE is "+
					"its database form, and \"take the lock before you read what you'll base the "+
					"write on\" is the whole rule."),
		),
		// ── 10. Audit in the transaction ────────────────────────────
		section("audit-in-transaction", "Audit rows in the transaction",
			"The audit append rides the changing transaction — so the history and the change commit or roll back together, by construction.",
			text("audit-pitch",
				"Topic 16 built the ``audit_log`` table and stated the pattern's second half as a "+
					"promise: the audit row is written INSIDE the transaction that makes the change. "+
					"Not after — inside. The failure mode \"after\" buys you is silent: the change "+
					"commits, the process dies before the audit write, and the log is now missing "+
					"the exact event it exists to record — a two-table write turned into a "+
					"distributed-systems problem. Inside, atomicity makes correctness free: one "+
					"``BEGIN … change … audit … COMMIT`` and there is no state where one landed "+
					"without the other. The demo runs both proofs as data: a commit phase whose "+
					"post AND audit rows appear together, and a rollback phase whose doomed title "+
					"change takes its audit row with it — the audit count refusing to grow is the "+
					"whole proof:"),
			code("audit-runner-go", "go",
				"The doomed unit: change, audit, roll back — both writes vanish together.",
				"server/internal/data/db/sql_demos.go",
				"doomed, err := r.pool.Begin(ctx)\n"+
					"// …UPDATE the title, INSERT the audit row that must never exist\n"+
					"if err := doomed.QueryRow(ctx,\n"+
					"    `SELECT (SELECT title FROM posts WHERE id::text = $1),\n"+
					"            (SELECT count(*) FROM audit_log WHERE post_id::text = $1)`, scratch,\n"+
					").Scan(&insideTitle, &insideRows); err != nil {\n"+
					"    return demos.AuditObservation{}, fmt.Errorf(\"observe inside the doomed transaction: %w\", err)\n"+
					"}\n"+
					"if err := doomed.Rollback(ctx); err != nil {\n"+
					"    return demos.AuditObservation{}, fmt.Errorf(\"roll both writes back: %w\", err)\n"+
					"}"),
			demo("db:audit", "Commit and roll back, together",
				"A scratch post and its audit rows: a create-plus-two-audit-rows unit that COMMITS "+
					"and is read back by the outside world, then an update-plus-audit unit that "+
					"ROLLS BACK — title restored, audit count unmoved, the rolled-back history "+
					"entry provably gone. Both verdicts are derived from the observed world, not "+
					"asserted; the demo cleans up its scratch rows afterwards."),
			text("audit-read",
				"Two honest footnotes complete the pattern. The demo deletes its scratch rows in a "+
					"final cleanup transaction — an affordance demos earn and production audit "+
					"trails do not: append-only means no UPDATE or DELETE grants, reads through a "+
					"dedicated path, because a log an attacker can edit is a liability wearing a "+
					"control's clothes. And the summary rows generalize past audits to every "+
					"derived write that must not drift — outboxes (topic 22), denormalized "+
					"counters, read-model updates: same transaction, or admit out loud that it is "+
					"eventually consistent. The database cannot enforce that honesty; the pattern "+
					"can, and this demo is the pattern's proof."),
		),
		// ── 11. Terminal companion ──────────────────────────────────
		section("terminal-companion", "Terminal companion: the curl and psql tour",
			"The topic from the shell: all six demos, the fake/live flip round-tripped by curl, and EXPLAIN ANALYZE live in psql.",
			text("term-pitch",
				"With the stack up (server on :8100, PostgreSQL migrated and seeded at boot), this "+
					"tour runs the topic's six demos from the shell, then walks the promise the "+
					"whole topic exists to keep — the dataMode flip: create a post in live mode, "+
					"watch it disappear under the fake, watch it come back. The last block runs "+
					"INSIDE the dev container's psql for the live plan; everything else is curl "+
					"and ``python3 -m json.tool``:"),
			code("term-demos", "bash", "", "",
				"# all six demos, straight from the shell\n"+
					"for d in sql-safety migrations sql-lab search race audit; do\n"+
					"  curl -s localhost:8100/api/demo/db/$d | python3 -m json.tool | head -20\n"+
					"done"),
			code("term-flip-live", "bash", "", "",
				"# the flip: live mode on, then a post lands in PostgreSQL (the id is a UUIDv7)\n"+
					"curl -s -X PATCH localhost:8100/api/demo/scenario \\\n"+
					"  -H 'content-type: application/json' -d '{\"dataMode\":\"live\"}'\n"+
					"curl -s -X POST localhost:8100/api/posts \\\n"+
					"  -H 'content-type: application/json' \\\n"+
					"  -d '{\"title\":\"Landed in PostgreSQL\",\"body\":\"The facade chose live for this call.\",\"tags\":[\"live\"]}'"),
			code("term-flip-fake", "bash", "", "",
				"# flip back: the fake store never saw the post — the strict miss is the truth\n"+
					"curl -s -X PATCH localhost:8100/api/demo/scenario \\\n"+
					"  -H 'content-type: application/json' -d '{\"dataMode\":\"fake\"}'\n"+
					"curl -s localhost:8100/api/posts?limit=50 \\\n"+
					"  | python3 -c 'import json,sys; print(\"titles:\", [p[\"title\"] for p in json.load(sys.stdin)[\"items\"] if \"Landed\" in p[\"title\"]])'"),
			code("term-flip-back", "bash", "", "",
				"# and back to live: the post was there all along — two stores, one wire contract\n"+
					"curl -s -X PATCH localhost:8100/api/demo/scenario \\\n"+
					"  -H 'content-type: application/json' -d '{\"dataMode\":\"live\"}'\n"+
					"curl -s 'localhost:8100/api/posts?tag=live' | python3 -m json.tool | head -15"),
			code("term-psql-explain", "bash", "", "",
				"# EXPLAIN ANALYZE, live in psql (inside the dev container's postgres):\n"+
					"# estimates beside actuals — watch the seq scan be RIGHT on six rows\n"+
					"docker exec go_developers_guide_postgres_dev psql -U postgres -d guide -c \\\n"+
					"  \"EXPLAIN ANALYZE SELECT title FROM posts WHERE to_tsvector('english', title || ' ' || body) @@ websearch_to_tsquery('english', 'logging or errors');\""),
			code("term-psql-audit", "bash", "", "",
				"# the audit table the demo cleans up after itself — empty between demos, by design\n"+
					"docker exec go_developers_guide_postgres_dev psql -U postgres -d guide -c \\\n"+
					"  'SELECT count(*) AS audit_rows FROM audit_log;'"),
			text("term-coda",
				"Notice what the tour never asks you to trust: the flip round-trip is proven by the "+
					"post's absence and return, not by a log line; the plan is the database's own "+
					"output with actual timings; the demos' no-residue discipline is checked by a "+
					"count. That is the data phase's standing deal with you — every claim in this "+
					"topic ran, against this schema, before you read it. The next topic takes the "+
					"same schema and asks what code GENERATION can add on top: sqlc's typed queries "+
					"and GORM's ORM, compared honestly against the hand-written SQL you just "+
					"watched work."),
		),
	}

	if buildErr != nil {
		return Topic{}, buildErr
	}
	return NewTopic(
		"databases-sql",
		"Databases and SQL from Go",
		"pgx v5 pooling, the DAO discipline with parameterized SQL, the fake/live repository switch "+
			"wired for real behind every API surface, transactions and savepoints, CTEs and window "+
			"functions, EXPLAIN plans, full-text search with tsvector, isolation and row locks, and "+
			"audit rows that ride the changing transaction — six live demos over the topic 16 schema.",
		"data",
		17,
		sections,
	)
}
