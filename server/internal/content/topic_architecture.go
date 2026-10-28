package content

import (
	"errors"
	"fmt"
)

// This file is topic 32 architecture (group platform, curriculum order
// 32): the whole-guide synthesis topic — the case study is THIS REPO,
// and every line quoted in every section below is a line this server
// compiles and runs today. Why architecture first (the dependency rule
// as a cost control, not ceremony: the price of a violated rule is the
// NEXT handler, paid in duplicated rules and store dialects). Then the
// layer map (a tour of internal/ — core, content, domain/<area>,
// data/<area>, presentation — read straight off the composition root's
// own import block). Then the dependency rule itself (presentation →
// domain ← data, ports owned by the domain, adapters proving
// satisfaction at compile time, DTOs never leaking past a repository,
// and the marked teaching-exceptions the guide carries — one per
// stack-subject exhibit, each saying so in its header). Thin
// handlers next (parse/validate → one use case → map — the posts
// handler against a demo handler, same discipline at two sizes). The
// composition root (main.go's constructor injection — Go's DI, no
// framework, wiring blocks like auth and the guarded authz routes
// quoted verbatim). Errors across layers (typed values up, one central
// writer down, the %w thread that keeps errors.Is honest end to end).
// The client's mirror (core/domain/data/ui with container.ts as its
// own composition root — the same rule in TypeScript). And the
// request-flow tracer (demo arch:tracer): one real posts request
// through middleware, handler, use case, and the adapter the scenario
// picked, every hop measured where it ran, plus the counterexample
// panel showing what a violation WOULD look like — carried as data,
// never compiled. The terminal companion tours it all with curl.

// architectureTopic builds topic 32.
func architectureTopic() (Topic, error) {
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
		// ── 1. Why architecture ────────────────────────────────────
		section("why-architecture", "Why architecture: the dependency rule is a cost control",
			"Architecture is not diagrams; it is who is allowed to know about whom. Get that one question wrong and every future feature pays for it — get it right and the costs stay flat.",
			text("arch-why-cost",
				"Everything in this topic reduces to one question with a measurable price: WHEN A LAYER CHANGES, HOW MANY OTHER FILES MUST CHANGE WITH IT? A handler that owns its own SQL makes every schema migration a handler hunt; a domain that imports a driver makes every store swap a rewrite; an error formatted five different ways makes every client integration a negotiation. The discipline this guide runs on — the dependency rule, thin handlers, one composition root, one error taxonomy — is not ceremony layered on top of working code: it is the practice of keeping the change-multiplication factor of every decision at one. The proof is the guide's own history, told elsewhere in these pages: the databases topic swapped the posts store from an in-memory fake to live PostgreSQL by writing ONE adapter behind an interface the domain already owned, and the scenario endpoint flips between them per request with no code path any the wiser; the GraphQL and gRPC topics mounted two entirely new transports over the SAME five use cases without touching them. Those were cheap because the seams already existed. Architecture is where you buy your seams, and you buy them before you need them because retrofitting costs the full price of every handler that grew around the missing one."),
			text("arch-why-go-posture",
				"Go's posture toward architecture is distinctive and worth naming before the tour. The language ships no classes, no inheritance, and no framework-shaped ceremony — but it ships INTERFACES that are satisfied implicitly, small packages that compose by import, and a compile step that makes the import graph a first-class fact. The idiomatic Go answer to ``where does the business logic live`` is therefore not a framework's annotation-scanned container: it is plain packages under ``internal/`` (unimportable outside the module — the compiler enforces the boundary), ports defined as interfaces the DOMAIN owns, adapters that prove satisfaction with a one-line compile-time assertion, and a ``main`` that wires it all by hand. Nothing in the standard library forces this shape — which is exactly why the dependency rule has to be a TEAM convention rather than a framework feature, and why this topic spends its evidence on the repo's real files rather than on a framework's documentation."),
			text("arch-why-case-study",
				"The case study is deliberate: this guide's server is a real mid-sized Go service — 30+ topics of working surface, five transports (REST, GraphQL, gRPC, WebSocket, SSE), three databases plus Redis, background jobs, and a content engine serving the very page you are reading — and it holds the whole discipline in a directory tree you can ``ls``. Every quotation below is a live line: the port, the adapter proof, the thin handler, the wiring blocks, the error writer. Where the curriculum needed machinery that did not exist yet, the request-flow tracer at the end of this topic BUILT it real — recorders at the seams of an actual request — rather than drawing a diagram of one. Read this topic with the repo open beside it; the architecture is the files."),
		),
		// ── 2. The layer map ───────────────────────────────────────
		section("layer-map", "The layer map: a tour of internal/",
			"The server's whole architecture is visible in one import block and five directory prefixes. Nothing is hidden behind a framework — the tree IS the architecture.",
			text("arch-map-tree",
				"The server lives under ``server/internal/``, and Go's ``internal`` rule does real work here: no other module can import these packages, so the server's boundaries are compiler-enforced at the module edge and convention-enforced within it. Five prefixes carry the layers, and one file ties them together. Read them as a responsibility list, because that is what a layer is — a named set of reasons to change:"),
			list("arch-map-layers",
				false,
				"internal/core — the shared bedrock: typed env config, the error taxonomy and its one envelope writer, slog JSON logging, request-id middleware, and the ScenarioStore the demos flip; depended on by EVERY layer, depending on nothing internal.",
				"internal/content — the teaching engine: typed topics → sections → blocks with startup validation; a pure data domain that knows nothing about HTTP beyond its handlers.",
				"internal/domain/<area> — the business: entities, validation rules, PORTS (interfaces the domain owns), and one-verb use cases; posts, auth, authorization, security, realtime, demos — each importing only core and each other.",
				"internal/data/<area> — the adapters: pgx, Redis, MongoDB, files, messaging implementations of the domain's ports; the ONLY layer that knows a store's dialect, including the posts switching facade.",
				"internal/presentation — the edge: thin handlers, middleware, the WebSocket hub; parses and maps, owns no rules, imports domain (never data).",
				"cmd/guide — the composition root: one main.go that loads config, builds adapters, injects them into use cases and handlers, and serves; the only file allowed to know every layer.",
			),
			code("arch-map-imports", "go",
				"The composition root's own import block — the whole layer map in twenty lines: stdlib first, then this module's packages in dependency order.",
				"server/cmd/guide/main.go",
				"import (\n"+
					"	// …fourteen stdlib imports first: context, fmt, log/slog, net, http…\n"+
					"	\"guide/internal/content\"\n"+
					"	\"guide/internal/core\"\n"+
					"	\"guide/internal/data/auth\"\n"+
					"	// …the data layer's adapters, one import per area, aliased where names clash\n"+
					"	\"guide/internal/domain/demos\"\n"+
					"	\"guide/internal/domain/posts\"\n"+
					"	\"guide/internal/domain/realtime\"\n"+
					"	\"guide/internal/grpcapi\"\n"+
					"	\"guide/internal/presentation\"\n"+
					")"),
			text("arch-map-reading",
				"That import block is an ARCHITECTURE DOCUMENT that cannot go stale, because the compiler reads it on every build. ``main`` is the one place allowed to see every layer — the data adapters (aliased where two areas would collide: ``authzdata``, ``postsdata``), the domains it wires them into, the grpcapi surface, and the presentation handlers it hands everything to. Now compare what each OTHER layer's files import: a domain file imports core and maybe a sibling domain, never data; a presentation file imports core and domains, never data; a data file imports core and the domain whose port it implements, never presentation. The convention is checkable in one command — ``grep -rn 'guide/internal/data' internal/presentation/`` returns only test files today — and that one-liner is the enforcement mechanism: the rule lives in the import graph, where ``go list -deps`` and any reviewer can see it. This is the Go-specific gift to architecture: dependencies are DECLARED per file, visible, greppable, and impossible to acquire by accident the way a framework's classpath magic acquires them."),
		),
		// ── 3. The dependency rule ─────────────────────────────────
		section("dependency-rule", "The dependency rule: presentation → domain ← data",
			"One arrow diagram, enforced by convention and proven by the compiler: both edges point INTO the domain, the domain owns the ports, and the adapters kneel.",
			text("arch-rule-shape",
				"The rule itself is one sentence: SOURCE CODE DEPENDENCIES POINT ONLY TOWARD THE DOMAIN. Presentation (handlers, middleware, transports) may import the domain; data (adapters) may import the domain; NOTHING may import presentation; the domain imports neither. The consequence is that the domain sits at the bottom of the import graph owning every seam: it declares, as INTERFACES, everything it needs from the outside world — storage, clocks, id minters, remote services — and the outer layers bring the implementations. This inversion is the whole trick. The posts domain does not know PostgreSQL exists; it knows a ``PostsRepository`` exists because IT DECLARED THAT TYPE, with five methods speaking the app's error taxonomy. When the databases topic added the live store, the domain did not change by one character — a new adapter knelt at the port it had always owned:"),
			code("arch-rule-port", "go",
				"The port, whole: the domain's own interface — ctx-first, typed errors, no storage vocabulary in sight.",
				"server/internal/domain/posts/repository.go",
				"// PostsRepository is the storage port for the posts resource. All methods\n"+
					"// take ctx first (anything that can block takes context) and return typed\n"+
					"// core errors: core.NotFound for an unknown id, so one mapping at the\n"+
					"// handler produces the 404 envelope for every adapter.\n"+
					"type PostsRepository interface {\n"+
					"	// List returns one page of posts matching the query, sorted and\n"+
					"	// filtered by the repository, with the total match count for the page\n"+
					"	// metadata.\n"+
					"	List(ctx context.Context, query ListQuery) (PostPage, error)\n"+
					"	// GetByID returns one post or core.NotFound.\n"+
					"	GetByID(ctx context.Context, id string) (Post, error)\n"+
					"	// Create stores a draft and returns the stored post — the adapter\n"+
					"	// assigns the ID and stamps CreatedAt/UpdatedAt.\n"+
					"	Create(ctx context.Context, draft Draft) (Post, error)\n"+
					"	// …Replace and Delete close the port the same way\n"+
					"}"),
			text("arch-rule-adapter-proof",
				"On the data side, satisfaction is a compile-time FACT, asserted in the adapter's own source — not a runtime surprise, not a framework scan. This one line is the dependency rule's handshake made mechanical: if the port grows a method, every adapter fails to compile until it kneels; if an adapter drifts from the contract, the compiler says so at build time, before any request can find out. The struct it attaches to shows the pattern the tracer demo will later exploit: the adapter holds the POOL plus injected seams for time and ids — never a global clock, never its own connection share — so tests pace it and the composition root owns every choice:"),
			code("arch-rule-adapter", "go",
				"The adapter's kneel: compile-time proof over the port, and a struct of injected seams — pool, clock, id minter.",
				"server/internal/data/posts/pg.go",
				"// Compile-time proof the PostgreSQL adapter satisfies the port.\n"+
					"var _ posts.PostsRepository = (*PostgresRepository)(nil)\n"+
					"\n"+
					"type PostgresRepository struct {\n"+
					"	pool  *pgxpool.Pool\n"+
					"	now   func() time.Time\n"+
					"	newID func() (string, error)\n"+
					"}"),
			text("arch-rule-dtos-and-exception",
				"Two corollaries complete the rule. FIRST, DTOs NEVER LEAK PAST A REPOSITORY: the wire shapes (``postResponse``, its camelCase tags, its materialized ``[]`` for empty tags) are defined and mapped in the presentation layer, and the data layer's rows become DOMAIN values before they cross back over the port — which is why ``PostPage`` speaks of pages and totals, never of SQL rows or BSON documents. A DTO leaking inward would weld the domain to one transport's spelling. SECOND, the marked exceptions: a guide whose SUBJECT is whole stacks occasionally needs an exhibit that constructs one in-process — the gRPC console demo builds a real gRPC server over a fresh in-memory repository, and because that file lives in ``domain/demos`` while touching a data adapter, its header carries an explicit TEACHING-EXCEPTION comment saying so (the sibling exhibits that import their own subject stacks — the GraphQL engine, the content registry's types — carry the same marker for the same reason). That is the honest way to break a rule you teach: loudly, in writing, exactly once for the data rule — the violation panel of this topic's demo greps the production tree to show the count is zero everywhere else. What the rule FORBIDS outright is the silent version: a presentation handler importing ``guide/internal/data`` for convenience, which the next section shows at its cheapest and the demo panel shows at its worst."),
		),
		// ── 4. Thin handlers ───────────────────────────────────────
		section("thin-handlers", "Thin handlers: parse, validate, delegate, map",
			"A handler is a translation layer, not a logic layer. The posts handler and a demo handler show the same four-move discipline at two very different sizes.",
			text("arch-thin-moves",
				"Every handler in this repo — REST, demo, health — runs the same four moves and nothing else: PARSE the request (query string, path values, body), VALIDATE through the domain's own rules (never rules of the handler's own invention), DELEGATE to exactly one use case, and MAP the result onto a status and body. The discipline pays twice. Downstream, the mapping half means every error path is one line — ``core.WriteError`` — so the envelope contract holds by construction; upstream, the parsing half means every rule lives in the domain where the GraphQL and gRPC transports can reuse it verbatim (and they do — three transports, one ``NormalizeListQuery``). Read the posts list handler and count what is ABSENT: no branch on storage kind, no paging math, no sort logic, no JSON string building — the handler is a corridor, and corridors are cheap to read, cheap to test, and impossible to break in interesting ways:"),
			code("arch-thin-posts", "go",
				"The whole list handler: parse, normalize through the domain, one use case, one mapping. Nothing else.",
				"server/internal/presentation/posts.go",
				"func (h *PostsListHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {\n"+
					"	query := r.URL.Query()\n"+
					"	normalized, err := posts.NormalizeListQuery(\n"+
					"		query.Get(\"page\"),\n"+
					"		query.Get(\"limit\"),\n"+
					"		strings.TrimSpace(query.Get(\"tag\")),\n"+
					"		query.Get(\"sort\"),\n"+
					"	)\n"+
					"	if err != nil {\n"+
					"		writePostsError(w, r, err, h.logger)\n"+
					"		return\n"+
					"	}\n"+
					"	page, err := h.list.Execute(r.Context(), normalized)\n"+
					"	if err != nil {\n"+
					"		writePostsError(w, r, err, h.logger)\n"+
					"		return\n"+
					"	}\n"+
					"	writeJSON(w, http.StatusOK, newPostPageResponse(page))\n"+
					"}"),
			text("arch-thin-demo-handler",
				"The same discipline at demo size — the guide's 40+ demo endpoints each back one PURE payload builder in ``internal/domain/demos`` with a handler that holds no state, parses nothing, and writes the builder's answer. The handler exists at all only because the route table speaks HTTP; everything worth testing lives behind it in the builder, where tests feed facts and pin payloads without a socket. This is the thin-handler rule's sharpest form: when a handler could plausibly be ``func(w, r) { writeJSON(w, 200, build()) }``, write exactly that and move every line of interest inward — the demo endpoints prove the pattern is not just for CRUD:"),
			code("arch-thin-demo", "go",
				"A demo handler, whole: the builder is pure, the handler is a corridor, and no demo endpoint in the guide owns a rule.",
				"server/internal/presentation/demo.go",
				"type RuntimeTypesHandler struct{}\n"+
					"\n"+
					"// NewRuntimeTypesHandler wires the runtime-types demo endpoint.\n"+
					"func NewRuntimeTypesHandler() *RuntimeTypesHandler {\n"+
					"	return &RuntimeTypesHandler{}\n"+
					"}\n"+
					"\n"+
					"// ServeHTTP implements http.Handler for the runtime-types demo payload.\n"+
					"func (h *RuntimeTypesHandler) ServeHTTP(w http.ResponseWriter, _ *http.Request) {\n"+
					"	writeJSON(w, http.StatusOK, demos.BuildRuntimeTypesPayload())\n"+
					"}"),
			text("arch-thin-smell",
				"The smell test is one question: COULD THIS HANDLER LINE MOVE TO THE DOMAIN WITHOUT LOSING ANYTHING? If a line parses HTTP specifics (headers, path values, body bytes), it stays; if it decides anything — a bound, a vocabulary, a sort order, a price, a permission — it moves. The guide's history has the negative case on record too: the counterexample panel in this topic's demo shows a handler that kept EVERYTHING — SQL, pool handling, paging, a raw 500 with a text body — and the costs named there (bypassed rules, a broken envelope, a store dialect per handler) are not hypothetical style complaints; they are the exact bills this repo never had to pay because the handlers stayed corridors."),
		),
		// ── 5. The composition root ────────────────────────────────
		section("composition-root", "The composition root: main.go is Go's DI",
			"No framework, no container, no annotations — one function that builds the world in dependency order and injects it constructor by constructor. Boring by design, debuggable by right.",
			text("arch-root-what",
				"Dependency injection in Go is a ``main`` function. The composition root — ``cmd/guide/main.go``'s ``run`` — loads typed config, dials the pool, runs migrations, builds every adapter, hands each one to the constructor of the thing that needs it, registers the finished handlers on the mux, and serves. Nothing anywhere else in the tree constructs a dependency for itself: handlers receive use cases, use cases receive ports, adapters receive pools and clocks. The payoff is that every WIRING QUESTION has one answer location — which adapter backs the posts port today, which logger the middleware chain uses, what the authz guard wraps — and every swap is a one-line edit at that location with the compiler walking you through the consequences. The authz block below is the pattern at full strength: a data-layer console is built once, handed to a story handler, and then WRAPPED — identity middleware outside permission middleware outside the guarded handler — pure constructor composition, no framework ever involved:"),
			code("arch-root-authz", "go",
				"Wiring as composition: the console is injected, and the guarded route is three constructors nested — identity outside permission outside handler.",
				"server/cmd/guide/main.go",
				"	authzConsole := authzdata.NewConsole()\n"+
					"	mux.Handle(\"GET /api/demo/authz/matrix\", presentation.NewAuthzMatrixHandler(authzConsole, logger))\n"+
					"	mux.Handle(\"GET /api/demo/authz/guarded\",\n"+
					"		presentation.RequireIdentity(authConsole.Resolver, logger)(\n"+
					"			presentation.RequirePermission(authzConsole, domauthz.PermPostsWrite, logger)(\n"+
					"				presentation.NewAuthzGuardedHandler(domauthz.PermPostsWrite, logger))))"),
			text("arch-root-why-no-framework",
				"Why no wire/dig/fx? Because the problems those solve — graph reflection, lifecycle breadth, provider sets — are problems this graph does not have: it is shallow (four layers), explicit (every edge visible in one file), and STARTS-THEN-STOPS within one function whose failure path is a returned error, not a container panic at inject time. A framework would move the wiring from readable code into configuration, and the whole point of a composition root is that wiring is READABLE CODE — greppable, debuggable, diff-able line by line. The guide does carry the honest counterweight: at hundreds of services with shared lifecycles, a framework's registration model starts earning its keep, and the dependency-management topic's choosing bar (stdlib first; adopt only at demonstrated cost) applies to DI frameworks exactly as it applied to routers. At this scale the constructor is the container, and ``go vet`` is the graph validator."),
			text("arch-root-fakelive",
				"The composition root is also where the fake/live teaching switch lives — the piece this topic's demo leans on. The posts wiring (quoted in full by the api-design topic) builds the SWITCHING FACADE over two adapters and the shared ScenarioStore; the tracer demo below adds one more composition of the same parts, with recorders between the layers. That is the quiet superpower of constructor injection at the root: instrumentation, decoration, and mode-switching are all the SAME operation — wrap the injected value at the one place it is handed over — with no code anywhere downstream able to tell the difference. It is the decorator pattern bought at the price of one constructor parameter, and the tracer exists to prove the seams are real by timing them."),
		),
		// ── 6. Errors across layers ────────────────────────────────
		section("errors-across-layers", "Errors across layers: typed values up, one envelope down",
			"Every layer speaks the same error taxonomy; one central writer turns it into the one wire envelope; and the %w thread keeps the original cause alive all the way down to the log line.",
			text("arch-errors-contract",
				"The error architecture is a contract with two directions. UP the stack, failures travel as VALUES in the ``core`` taxonomy: the domain's use cases return ``core.ValidationFailed`` with structured details, the adapters return ``core.NotFound`` for a miss and ``core.ServiceUnavailable`` when a store is down — never booleans to interpret, never panics to catch, never strings to parse. The typed error carries its KIND (the failure bucket that maps to a status), its MESSAGE (served verbatim), optional DETAILS, and optionally a CAUSE — and the cause is where the second direction begins: wrapping with ``%w`` semantics (``core.Wrap`` builds the typed error AROUND an inner one) keeps the chain inspectable, so ``errors.Is`` and ``errors.As`` work through every layer the failure crossed. When the databases topic's adapter wraps a pgx error in a typed 503, the pgx error is still THERE — logged, inspectable, unforgotten:"),
			code("arch-errors-typed", "go",
				"The taxonomy's core type and its wrap constructor: kind + message + details + a cause that survives.",
				"server/internal/core/errors.go",
				"type Error struct {\n"+
					"	Kind    Kind\n"+
					"	Message string\n"+
					"	Details any\n"+
					"	cause   error\n"+
					"}\n"+
					"\n"+
					"// Wrap builds a typed error that carries cause (%w-style) for the\n"+
					"// error's own bookkeeping while presenting message to callers.\n"+
					"func Wrap(cause error, kind Kind, message string) *Error {\n"+
					"	return &Error{Kind: kind, Message: message, cause: cause}\n"+
					"}"),
			text("arch-errors-writer",
				"DOWN the stack, exactly one function writes error responses — ``core.WriteError`` — and every handler's every failure path is one call to it. The writer is where the taxonomy meets HTTP: it maps the kind to its status, stamps the request id from the context, logs the failure at the right level (warn for expected kinds, error for taxonomy misses), and emits the family envelope ``{error:{code,message,requestId,details?}}``. Because the mapping lives in ONE place, it lives CORRECTLY in one place: a 404 is not_found on every route, an unknown kind degrades to a generic 500 with the real error logged but never served, and a client integrating against this API parses one shape for every failure it will ever meet. The handler's side of the bargain is the thinnest possible — this is the whole read-one handler, error path included:"),
			code("arch-errors-handler", "go",
				"The handler's entire error handling: hand the value to the central writer. It maps, logs, stamps, and envelopes.",
				"server/internal/presentation/posts.go",
				"func (h *PostDetailHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {\n"+
					"	post, err := h.get.Execute(r.Context(), r.PathValue(\"id\"))\n"+
					"	if err != nil {\n"+
					"		core.WriteError(w, r, err, h.logger)\n"+
					"		return\n"+
					"	}\n"+
					"	writeJSON(w, http.StatusOK, newPostResponse(post))\n"+
					"}"),
			text("arch-errors-why-it-holds",
				"What makes the contract durable is that no layer can cheaply break it. A handler that tried to format its own error would have to import statuses and envelope shapes by hand — visible drift in review; an adapter that returned a raw driver error would fail the taxonomy the moment WriteError met it (and the failure mode is the SAFE one: a generic 500 with the truth in the log, never a leaked internal). The error-handling topic walks this whole machine with a live failure-injection probe; architecture's claim on it is narrower and older: the taxonomy is a LAYER ARTIFACT — it exists so that the domain can name failures without knowing HTTP, the edge can map them without knowing stores, and neither ever speaks the other's dialect. That is the dependency rule applied to failures, and it is why the same five use cases serve REST, GraphQL, and gRPC with three different transport error encodings and zero duplicated error logic."),
		),
		// ── 7. The client's mirror ─────────────────────────────────
		section("clients-mirror", "The client's mirror: the same rule in TypeScript",
			"The React shell that renders this guide runs the architecture's exact mirror — core/domain/data/ui, ports at the domain edge, one composition root — which is the strongest proof the rule is about layers, not about Go.",
			text("arch-mirror-map",
				"The client is a presentation-only shell by family rule — it never teaches React, it renders the server's content — and its internals mirror the server deliberately: ``src/core`` (config, the Result type, the error pipeline, the container), ``src/domain`` (models, repository PORTS as interfaces, one-purpose use cases), ``src/data`` (DTO validation, fetch-based API adapters, repository implementations), and ``src/ui`` (views and components that resolve use cases and never import data). The dependency arrows are identical: ui → domain ← data, with DTOs (the wire's snake_case vs the domain's spelling) converted in the data layer and never leaking inward. The port below is the client half of a seam this guide has crossed a hundred times — every demo payload you have loaded walked through one of these:"),
			code("arch-mirror-port", "ts",
				"A client port: the domain owns the interface, the Result type carries failure as a value, and the view never sees fetch.",
				"client/src/domain/demo/deps-manifest-repository.ts",
				"export interface DepsManifestRepository {\n"+
					"  get(): Promise<Result<DepsManifestDemoData>>;\n"+
					"}"),
			text("arch-mirror-container",
				"The mirror's composition root is ``container.ts`` — a hand-rolled service locator keyed by symbol tokens, playing the role ``main.go`` plays server-side. Its layer rules are enforced by construction and stated in its own header: ONLY the container imports data adapters; views resolve through the container and never import data; data never imports the container. The posts block below is the mirror of the server's posts wiring quoted by the api-design topic — one API adapter, one repository over it, four verb use cases — and the guide's demo pipelines land the same way, one register triple per demo, which is why every demo view is a lazy chunk that resolves its own use case with no prop-drilling and no module-level singleton:"),
			code("arch-mirror-wiring", "ts",
				"The client's composition root wiring the posts pipeline — the same graph main.go builds, in TypeScript.",
				"client/src/core/container.ts",
				"    this.register(ContainerTokens.PostsApi, new PostsApi(config));\n"+
					"    this.register(\n"+
					"      ContainerTokens.PostsRepository,\n"+
					"      new PostsRepositoryImpl(this.postsApi),\n"+
					"    );\n"+
					"    this.register(\n"+
					"      ContainerTokens.ListPostsUseCase,\n"+
					"      new ListPostsUseCase(this.postsRepository),\n"+
					"    );\n"+
					"    // …three more verb use cases over the same repository"),
			text("arch-mirror-why",
				"Why mirror at all, when the client is small? Because the rule's benefits are size-independent: the error pipeline lives once (every failure — network, non-2xx, malformed payload — becomes a typed Failure the shared ErrorPanel renders), the demo pipelines are interchangeable (a view swap never touches transport), and the composition root makes the wiring auditable in one file. The family's other guides — express, springboot, fastapi — run this same client shape against their servers, so the mirror is also a PORTABILITY result: the layer rule, not the language, is what made the client clonable across four backends. When the request-flow tracer below draws its timeline, the GET that fetched the payload walked BOTH graphs — the server's middleware → handler → use case → adapter, and the client's api → repository → use case → view — and the same discipline carried it end to end."),
		),
		// ── 8. Request-flow tracer [demo] ─────────────────────────
		section("request-flow-tracer", "The request-flow tracer: one request, four layers, measured",
			"The topic's demo walks a REAL posts request through the actual chain with recorders at the seams — every hop a measurement taken where the code ran, and a counterexample panel for the rule's negative space.",
			text("arch-tracer-how",
				"Diagrams assert; the tracer MEASURES. One marker request — ``GET /api/posts?limit=3&sort=newest`` — executes for real inside the demo handler: the genuine ``PostsListHandler`` over a ``ListPostsUseCase`` over the SWITCHING repository, mounted on a private mux behind the production middleware chain, with recorders slipped between the layers exactly the way the composition root slips adapters. Each recorder is a decorator over the ``posts.PostsRepository`` PORT (plus one timing wrapper at the handler boundary): it notes the clock at entry, delegates unchanged — arguments, results, and errors pass through untouched — and records the seam's layer, file, symbol, and inclusive duration at exit. Because the recorders wrap the two adapters INSIDE the switching facade, the timeline's data hop names the store that ACTUALLY answered under the scenario's current mode — flip ``dataMode`` through the scenario endpoint and reload, and the same trace re-runs with a different leaf while every layer above the port proceeds unaltered, which is the dependency rule demonstrating itself:"),
			code("arch-tracer-recorder", "go",
				"One seam, recorded: the decorator times the port call and passes everything through — the traced pipeline behaves exactly like the unwired one.",
				"server/internal/domain/demos/arch_tracer.go",
				"// List implements the port through the wrapped repository, recorded.\n"+
					"func (rec *ArchTraceRecorder) List(ctx context.Context, query posts.ListQuery) (posts.PostPage, error) {\n"+
					"	started := rec.sink.Now()\n"+
					"	page, err := rec.inner.List(ctx, query)\n"+
					"	rec.sink.Record(rec.meta, started)\n"+
					"	return page, err\n"+
					"}"),
			text("arch-tracer-wiring",
				"The wiring is the composition root doing what composition roots are for — instrumenting by decorating at the handover points. Note what the tracer's wiring does NOT contain: no change to the handler, the use case, the facade, or either adapter; the recorders exist only in the ``main`` function's wiring lines, wrapped around the same constructors the production routes use. This is the same operation as the fake/live switch (wrap the injected value at the one place it is handed over), and it is why instrumentation, mode-switching, and decoration are all ONE capability in a wired-at-the-root codebase:"),
			code("arch-tracer-main", "go",
				"The tracer's composition: leaf recorders inside the facade, the domain recorder around it, the use case over that — production constructors, decorated.",
				"server/cmd/guide/main.go",
				"	archSink := demos.NewArchTraceSink(time.Now)\n"+
					"	archFake := demos.NewArchTraceRecorder(postsdata.NewInMemoryRepository(), demos.ArchDataInMemoryHop, archSink)\n"+
					"	archLive := demos.NewArchTraceRecorder(postsdata.NewPostgresRepository(pool), demos.ArchDataPostgresHop, archSink)\n"+
					"	archRepo := postsdata.NewSwitchingRepository(scenarioStore, archFake, archLive)\n"+
					"	archList := posts.NewListPostsUseCase(demos.NewArchTraceRecorder(archRepo, demos.ArchDomainHop, archSink))\n"+
					"	mux.Handle(\"GET /api/demo/arch/tracer\",\n"+
					"		presentation.NewArchTracerHandler(scenarioStore, archList, archSink, logger))"),
			text("arch-tracer-counterexample",
				"The demo's second panel is the rule's negative space: what a VIOLATION would look like. The snippet below lives as a STRING CONSTANT inside the tracer's builder — data the payload carries, never code the module compiles — and the topic quotes it with the builder file as its snippet source precisely so the quotation discipline holds even for anti-code. Read it as a cost inventory: the handler owns SQL and a pool (two data-layer jobs), paging and sort logic (domain jobs), a raw ``http.Error`` 500 with a text body (an envelope the client's error pipeline cannot parse), and — the part that never shows up in a diff — it establishes a second store dialect that every FUTURE handler must either duplicate or import around. The production tree carries none of this: the guide's single data-layer exemption is the marked gRPC exhibit named in the dependency-rule section, and the violation panel says so from the repo's actual state, not from aspiration:"),
			code("arch-tracer-violation", "go",
				"The counterexample, quoted from the string constant that carries it — a handler importing the data layer. This is what the rule forbids; it is not code this repo builds.",
				"server/internal/domain/demos/arch_tracer.go",
				"func (h *BadPostsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {\n"+
					"	tag := r.URL.Query().Get(\"tag\")\n"+
					"	rows, err := h.pool.Query(r.Context(),\n"+
					"		\"SELECT id, title, body FROM posts p WHERE EXISTS (\"+\n"+
					"			\"SELECT 1 FROM post_tags t WHERE t.post_id = p.id AND t.tag_name = $1)\", tag)\n"+
					"	if err != nil {\n"+
					"		http.Error(w, \"database unavailable\", 500)\n"+
					"		return\n"+
					"	}\n"+
					"	defer rows.Close()\n"+
					"	// …the row scanning, the paging math, the sort, and the JSON shaping\n"+
					"	// all inline in the handler — every rule the domain owns, bypassed,\n"+
					"	// and a second store dialect for every handler that follows\n"+
					"}"),
			demo("arch:tracer", "The architecture tracer: one request through the layers, measured",
				"A real posts-list request walked through middleware → thin handler → use case → the adapter the scenario picked, every hop labeled by the file that ran it and timed at its seam (durations are inclusive — the narrowing numbers down the timeline are the thin-delegation evidence). The panel names the leaf the facade chose and how to flip it (PATCH /api/demo/scenario, then reload: same shape, different leaf). Beside the timeline, the rule-violation counterexample — a handler importing the data layer, carried as clearly-labeled non-compiling data — with the honest state of this repo's import graph."),
			text("arch-tracer-reading",
				"How to read the timeline the panel draws: the four rows NEST. The middleware row's clock covers the whole call; the handler row covers parsing, normalization, delegation, and mapping; the domain row is measured at the port boundary the use case delegates over; the data row is the adapter the facade picked. The near-equality of the bottom two rows is not a rendering artifact — it is the thin-delegation discipline showing up as a MEASUREMENT (the use case adds nothing to the store's cost, by design). And the request id threading the panel's facts is the errors topic's middleware order made visible: one id stamped outermost, adopted by the traced request, reported by the payload — the same thread every log line of the walk carried."),
		),
		// ── 9. Terminal companion ──────────────────────────────────
		section("terminal-companion", "Terminal companion: the architecture tour",
			"The tracer over curl in both scenario modes, the layer map over ls and the import graph over grep — the architecture read from the dev container's shell.",
			text("term-arch-intro",
				"With the stack up (server on :8100), start where the demo starts: one GET returns the traced timeline — four hops, outermost first, each carrying its layer, file, symbol, and inclusive duration, plus the leaf the scenario picked and the counterexample panel. The second block flips the scenario to live and re-runs the SAME trace: the shape holds, the data hop re-labels to the PostgreSQL adapter, and the layers above the port proceed unaltered — the dependency rule, executed twice. The third block reads the architecture straight off the tree: the layer directories under internal/, the composition root's size (every wiring decision in one readable file), and the import-graph greps that make the dependency rule checkable — including the one that counts production presentation files importing the data layer, whose honest answer is zero."),
			code("term-arch-trace-bash", "bash", "", "",
				"# the traced timeline: one request, four layers, every hop measured\n"+
					"curl -s localhost:8100/api/demo/arch/tracer | python3 -m json.tool | head -40\n"+
					"curl -s localhost:8100/api/demo/arch/tracer | python3 -c \\\n"+
					"  'import json,sys; p=json.load(sys.stdin)[\"trace\"]; \\\n"+
					"   [print(h[\"order\"], h[\"layer\"], h[\"symbol\"], str(h[\"durationMs\"])+\"ms\") for h in p[\"hops\"]]'\n"+
					"\n"+
					"# flip the leaf and re-run the SAME trace — same shape, different data hop\n"+
					"curl -s -X PATCH localhost:8100/api/demo/scenario \\\n"+
					"  -H 'content-type: application/json' -d '{\"dataMode\":\"live\"}'\n"+
					"curl -s localhost:8100/api/demo/arch/tracer | python3 -c \\\n"+
					"  'import json,sys; p=json.load(sys.stdin)[\"trace\"]; print(p[\"dataMode\"], \"->\", p[\"leaf\"])'\n"+
					"curl -s -X PATCH localhost:8100/api/demo/scenario \\\n"+
					"  -H 'content-type: application/json' -d '{\"dataMode\":\"fake\"}'   # restore"),
			code("term-arch-graph-bash", "bash", "", "",
				"# the layer map, straight off the tree\n"+
					"ls internal/            # core content domain data grpcapi presentation\n"+
					"ls internal/domain/     # one package per business area\n"+
					"wc -l cmd/guide/main.go # the composition root — every wiring decision, one file\n"+
					"\n"+
					"# the dependency rule, checkable in two greps\n"+
					"grep -rn 'guide/internal/data' internal/presentation/*.go | grep -v _test\n"+
					"#   (no output: zero production handlers import the data layer)\n"+
					"grep -rnE '^[[:space:]]*([[:alnum:]]+[[:space:]]+)?\"guide/internal/presentation\"' internal/domain/ internal/data/\n"+
					"#   (no output: no IMPORT line points into the edge — anchored to the import\n"+
					"#    shape, because the runner demos name the package in string literals)\n"+
					"grep -rln 'TEACHING-EXCEPTION' internal/domain/demos/   # the marked exhibits, one header each"),
			text("term-arch-coda",
				"Read the tour as the topic's thesis in shell form. The tracer GET is the layer map EXECUTED — files and symbols you just ``ls``-ed, appearing as hops with durations attached, flipping their leaf on a scenario PATCH without any layer above noticing. The greps are the dependency rule's enforcement story told honestly: no framework police, just an import graph that is trivially inspectable, a convention stated in one sentence, and a review culture that keeps the count at zero — with every marked exemption greppable by name. That is Go architecture at this scale: ``internal/`` plus five prefixes, one rule with two arrows, a ``main`` that wires the world in dependency order, one error taxonomy with one writer, a client that mirrors the whole shape — and, when you want to prove any of it, a composition root that lets you wrap a recorder around the seams and watch the rule run."),
		),
	}

	if buildErr != nil {
		return Topic{}, buildErr
	}
	return NewTopic(
		"architecture",
		"Architecture in Go: layers, the dependency rule, and one composition root",
		"The whole-guide synthesis, taught from the repo that is running "+
			"it: why the dependency rule is a cost control (the price of a "+
			"violated seam is the next handler), the layer map of internal/ "+
			"read off the composition root's own imports, presentation → "+
			"domain ← data with ports the domain owns and adapters that "+
			"prove satisfaction at compile time, thin handlers as corridors "+
			"(parse, validate, delegate, map), main.go's constructor "+
			"injection as Go's dependency injection, the one error taxonomy "+
			"flowing up as typed values and down as one envelope with the "+
			"%w thread intact, and the client's core/domain/data/ui mirror "+
			"with container.ts as its own composition root — proven live by "+
			"the request-flow tracer (demo arch:tracer), which walks a real "+
			"posts request through middleware, handler, use case, and the "+
			"adapter the scenario picked, measuring every hop at its seam, "+
			"beside a clearly-labeled counterexample of the violation the "+
			"rule exists to prevent.",
		"platform",
		32,
		sections,
	)
}
