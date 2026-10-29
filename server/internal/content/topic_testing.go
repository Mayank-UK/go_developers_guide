package content

import (
	"errors"
	"fmt"
)

// This file is topic 34 testing (group platform, curriculum order 34):
// how this repository verifies itself — honestly, from the inside. The
// subject of every sourced snippet below is the repo's OWN test code:
// the table-driven config table, the httptest chain builder and its
// doRequest helper, the testcontainers boot/skip gate, the stepping
// clock and throwaway databases that make time and state deterministic,
// the race-hammered switching facade, the client's three vitest layers,
// and the pins and ratchets that guard this content itself. The topic's
// one demo (testing:runner) is the discipline made executable: a real,
// bounded `go test -json` run over a curated set of the repo's own
// packages, parsed event by event — no number in its payload typed by
// hand — plus the suite's shape counted from the tree. The pyramid is
// described as this repo actually shapes it (unit-heavy, integration
// where it pays, browser E2E one deliberately thin host-run suite), the CI story is
// this family's real one (no hosted CI; the local verification command
// IS the contract), and the coverage section argues what the number
// buys and what the ratchet buys instead.

// testingTopic builds topic 34.
func testingTopic() (Topic, error) {
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
		// ── 1. The pyramid, honestly ──────────────────────────────
		section("pyramid-honestly", "The pyramid, honestly: what this repo actually runs",
			"The testing pyramid is a cost model, not a law — and the honest way to teach it is to show this repository's real distribution rather than an idealized diagram.",
			text("tst-pyramid-model",
				"The classic pyramid says: many unit tests, fewer integration tests, a thin cap of end-to-end tests. The reasoning behind the shape is COST AND LOCALITY — a unit test exercises one function or one type in milliseconds, fails pointing at the line that broke, and runs anywhere; an integration test drags a real dependency along (a database boot, a socket, seconds); an end-to-end test drives the whole assembled system through its outermost surface, which makes it the only test that proves the WIRING but also the slowest to run, the flakiest to trust, and the least helpful when it fails (something broke — somewhere). The pyramid is the equilibrium those economics push you toward, and most dogma around it is just the equilibrium restated with moral language. What matters is that each layer answers a QUESTION the cheaper layer cannot: unit tests answer ``does this logic hold``, integration tests answer ``does this logic hold against the real PostgreSQL``, and end-to-end tests answer ``does the assembled product work in a browser``."),
			text("tst-pyramid-repo",
				"So here is THIS repo's real distribution, stated as it is rather than as a diagram would like it to be. The suite is UNIT-HEAVY by design: pure domain logic (draft normalization, query boundaries, the trace-context parser) and pure payload builders (every demo in internal/domain/demos) test as plain functions with table tests, in milliseconds. INTEGRATION tests exist exactly where they pay: migrations and schema assertions run against the stack's real PostgreSQL through dedicated throwaway databases, redis and messaging stories run against the real redis, and the message-adapter layer round-trips real envelopes — while handler tests sit in between, running the real middleware chain and the real handlers over in-memory adapters through net/http/httptest, which is ``integration-shaped`` without any process boundary. And BROWSER E2E is exactly one small suite: ``e2e/`` runs host-side Playwright (seven spec files, eleven tests) against the composed stack, a boundary this topic's last sections teach honestly — what those specs pin, and why the tier deliberately stops there — rather than pretending a screenshots directory constitutes coverage. The demo at this topic's center makes the shape visible by RUNNING a slice of it."),
			list("tst-pyramid-distribution",
				false,
				"Unit (the wide base) — table tests over pure logic and payload builders: domain rules, parsers, the demo builders; milliseconds each, no I/O, run under -race on every verification pass.",
				"Handler-height (the middle) — httptest servers running the production middleware chain over in-memory adapters: the wire contract, status codes, envelopes, decode strictness — real routing and serialization without process boundaries.",
				"Integration (where it pays) — the real PostgreSQL of the dev stack through throwaway databases, real redis streams, the real go toolchain in the dependency and testing demos: each one gated to skip honestly when its dependency is absent.",
				"End-to-end (the cap) — host-run Playwright against the composed stack: one deliberately thin suite in ``e2e/`` pinning wiring facts no in-process tier can see, kept small on purpose rather than sprawled per topic.",
			),
			text("tst-pyramid-why-this-shape",
				"Two deliberate asymmetries in that distribution deserve naming, because they are decisions rather than accidents. First, this repo has NO MOCKING FRAMEWORK — no testify, no gomock: the ``fakes`` are hand-written adapters of the domain's own port interfaces (the fakeRepository the posts domain tests use, the counting recorders the caching tests use), which cost a few dozen lines once and then read like any other adapter instead of like invocation bookkeeping. Second, the suite's ratio of test code to production code is well above one, and that is normal for a codebase whose tests are also its SUBJECT: the guide teaches testing BY being a thing you can read the tests of. The next section starts where reading should start — with the table, the form almost every test in this repo takes."),
		),
		// ── 2. Table-driven tests + t.Parallel ────────────────────
		section("table-driven-tests", "Table-driven tests and t.Parallel: one test, many cases",
			"Go's idiomatic test shape is a loop over a table of cases — the test body becomes the QUESTION and the table becomes the ANSWERS — and t.Parallel spreads those answers across every core the machine has.",
			text("tst-table-shape",
				"A table-driven test inverts the usual structure: instead of one function per case, you write ONE test function whose body is a loop, and a slice of structs — the table — where every row is a case with a name, its inputs, and its expected outcome. The payoff is threefold, and all three show up in the real example below. ADDING A CASE IS ADDING A LINE — the test body never grows, so the marginal cost of the sixteenth boundary case is zero and the table becomes the natural place to accumulate every edge case a bug report teaches you. FAILURE NAMES ITS CASE — Go reports ``TestLoadConfig/limits_at_the_ceiling`` rather than ``test 14 failed``, so the red row tells you which INPUT broke before you read anything. And THE TABLE IS ITSELF DOCUMENTATION — reading the table top to bottom is reading the function's contract, inputs to expected outputs, which is why this repo's tables tend to run from the happy default to the hostile edge in a deliberate order. The canonical example in this codebase is the config loader's table:"),
			code("tst-table-config", "go",
				"The repo's own parallel exemplar: typed config parsed from a hostile world — every case one row, every row one subtest.",
				"server/internal/core/config_parsing_test.go",
				"func TestLoadConfig(t *testing.T) {\n"+
					"	t.Parallel()\n"+
					"\n"+
					"	tests := []struct {\n"+
					"		name    string\n"+
					"		env     map[string]string\n"+
					"		want    Config\n"+
					"		wantErr bool\n"+
					"	}{\n"+
					"		{\n"+
					"			name: \"defaults when unset\",\n"+
					"			env:  nil,\n"+
					"			want: Config{Port: 8100, Env: \"dev\", LogLevel: slog.LevelInfo, GRPCAddr: \":8102\", DatabaseURL: defaultDatabaseURL, MongoURI: defaultMongoURI, RedisAddr: defaultRedisAddr},\n"+
					"		},\n"+
					"		// …every boundary and hostile env row, same shape\n"+
					"	}\n"+
					"\n"+
					"	for _, tt := range tests {\n"+
					"		t.Run(tt.name, func(t *testing.T) {\n"+
					"			t.Parallel()\n"+
					"			got, err := LoadConfig(func(key string) string { return tt.env[key] })\n"+
					"\n"+
					"			if tt.wantErr {\n"+
					"				// …errors must name the offending GDG_ variable\n"+
					"				return\n"+
					"			}\n"+
					"			if err != nil {\n"+
					"				t.Fatalf(\"LoadConfig(%v) unexpected error: %v\", tt.env, err)\n"+
					"			}\n"+
					"			if got != tt.want {\n"+
					"				t.Errorf(\"LoadConfig(%v) = %+v, want %+v\", tt.env, got, tt.want)\n"+
					"			}\n"+
					"		})\n"+
					"	}\n"+
					"}"),
			text("tst-table-parallel",
				"Read the ``t.Parallel`` placement carefully, because it is two distinct declarations. The one at the TOP of the function lets the whole test run concurrently with its sibling tests in the package — the package's runner is free to schedule it while other, slower tests settle. The one INSIDE the ``t.Run`` closure is the one that matters more: each subtest marks itself parallel, and when the loop finishes, Go releases all of them onto ``GOMAXPROCS`` workers at once. The table's rows then execute CONCURRENTLY, which is both a speed gift (a 40-row table costs the wall-clock of its slowest row, not the sum) and a free probe: cases that race each other's shared state fail here first, under the race detector, before they fail in production. The standard discipline applies — a parallel test must not mutate state its siblings read — and the config table satisfies it by construction: each closure captures its own ``tt`` row and reads only that. One subtlety this repo leans on: the loop variable capture is safe without copying because each closure runs only ``for`` the subtest it was declared in, and Go 1.22's per-iteration loop variables made even the historical footgun disappear."),
			text("tst-table-where",
				"Where the tables live in this repo — located by symbol, verified live by the demo below: the config table just quoted (``TestLoadConfig``, the parallel exemplar), the posts-create boundary table (``TestPostsCreateRefusesBodiesAtTheBoundary`` — five hostile request bodies, each pinned against the 422 envelope), and the trace-context edge table (``TestParseTraceparentRejectsMalformedHeaders`` — fourteen malformed headers, each rejection matched against the parser's own error text, the table the observability topic's hostile-header panel renders). That last pair of parentheses is the table-driven idea closing a loop: the topic TAUGHT the parser's edge cases, the TABLE pins them, and the demo RUNS the package the table lives in. When a table row fails, the fix usually starts by adding the row that should have existed — tables grow toward their subject's real boundary the way the boundary itself is discovered."),
		),
		// ── 3. httptest against handlers ───────────────────────────
		section("httptest-handlers", "httptest: the real chain, exercised in-process",
			"Handler tests in this repo do not call functions — they fire HTTP requests through the production middleware chain at an in-memory server and assert on the recorded response.",
			text("tst-httptest-model",
				"``net/http/httptest`` is the standard library's answer to the question every HTTP handler test asks: how do I exercise a handler WITHOUT a port, a socket, or a process? Its two halves are exactly the two halves of a request. ``httptest.NewRequest`` builds a request from a method, a target, and an optional body — fully formed, no network. ``httptest.NewRecorder`` builds a ``ResponseWriter`` that CAPTURES everything the handler writes — status, headers, body bytes — and plays it back as a ``*httptest.ResponseRecorder`` you assert on: ``rec.Code``, ``rec.Header()``, ``rec.Body.String()``. Between them a handler runs exactly as it would serve. But the pattern this repo standardizes goes one step further than handler-plus-recorder: tests build the PRODUCTION STACK — mux with the real route patterns, the real middleware Chain around it — and fire at that, so what a test pins is not a function's return but the WIRE CONTRACT: the status, the envelope's camelCase keys, the Location header on a 201, the Allow header on a 405, the x-request-id on every failure. Here is the builder every posts-handler test starts from:"),
			code("tst-httptest-server", "go",
				"The chain builder: real routes, real middleware, in-memory adapter — the production shape minus the port.",
				"server/internal/presentation/posts_test.go",
				"func newPostsServer(logger *slog.Logger) http.Handler {\n"+
					"	repository := postsdata.NewInMemoryRepository()\n"+
					"	mux := http.NewServeMux()\n"+
					"	mux.Handle(\"GET /api/posts\", NewPostsListHandler(posts.NewListPostsUseCase(repository), logger))\n"+
					"	// …the other four routes, wired identically over the same repository\n"+
					"	return Chain(\n"+
					"		RequestID,\n"+
					"		RequestLogger(logger),\n"+
					"		Recover(logger),\n"+
					"		EnvelopeMuxErrors(logger),\n"+
					"	)(mux)\n"+
					"}"),
			code("tst-httptest-doRequest", "go",
				"The shared fire-and-capture helper every handler test file reuses.",
				"server/internal/presentation/content_handler_test.go",
				"func doRequest(server http.Handler, method, path, body string) *httptest.ResponseRecorder {\n"+
					"	rec := httptest.NewRecorder()\n"+
					"	var req *http.Request\n"+
					"	if body == \"\" {\n"+
					"		req = httptest.NewRequest(method, path, nil)\n"+
					"	} else {\n"+
					"		req = httptest.NewRequest(method, path, strings.NewReader(body))\n"+
					"	}\n"+
					"	server.ServeHTTP(rec, req)\n"+
					"	return rec\n"+
					"}"),
			text("tst-httptest-why",
				"Two properties of this shape earn their keep week after week. First, the middleware is the PRODUCTION order — RequestID outermost, then logging, then panic recovery, then the mux-error envelope — so a test that asserts a 404 carries both the error envelope AND a request id is asserting the COOPERATION of three middlewares and the handler, not any one of them; when that cooperation breaks, the failure reproduces in a millisecond test instead of a manual curl. Second, the DEPENDENCY is injected exactly as main.go injects it: the handler under test receives the same use-case constructor over the same in-memory adapter the composition root wires in fake mode, so a handler test failing means the HANDLER drifted, and a use-case failing means the domain test already caught it one layer down — this is the layering topic's dependency rule paying rent in test locality. The one honest limitation of httptest-shaped tests is that they never leave the process: no real TCP, no real client timeouts, no TLS handshake. Those properties belong to the integration tier — and to the one tier above it, whose plan this topic's later sections state plainly."),
		),
		// ── 4. testcontainers-go [demo testing:runner] ─────────────
		section("testcontainers", "testcontainers-go: real PostgreSQL, gated honestly",
			"Some contracts only a real database can verify — testcontainers-go spawns a throwaway PostgreSQL from inside the test; the gate that skips when Docker is absent is part of the pattern, not an apology.",
			text("tst-tc-why",
				"There is a class of bug no in-memory adapter can catch: a migration whose SQL does not do what its author believes, a type mapping that silently truncates, an index the planner refuses because of a shape only real ANALYZE sees, a constraint that fires differently on real PostgreSQL than on any simulation. For those, the test needs the REAL engine — and the question becomes how a test obtains one without making the suite hostage to a manually-provisioned database. testcontainers-go is the standard answer: the test itself drives the Docker daemon (through the socket the dev container mounts), asks for a container from a pinned image, WAITS until the engine is genuinely ready, uses it, and terminates it in cleanup — with a reaper (Ryuk) that collects anything a crashed run leaves behind. This repo's smoke test is the whole pattern in miniature, and its first three lines are the part worth internalizing:"),
			code("tst-tc-boot-skip", "go",
				"The boot/skip gate: the socket is probed, the absence skips, and the suite never hard-depends on what it cannot reach.",
				"server/internal/data/db/testcontainers_smoke_test.go",
				"func TestTestcontainersSmoke(t *testing.T) {\n"+
					"	if _, err := os.Stat(dockerSocket); err != nil {\n"+
					"		t.Skipf(\"docker socket unavailable at %s: testcontainers wiring not present on this machine\", dockerSocket)\n"+
					"	}\n"+
					"\n"+
					"	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)\n"+
					"	defer cancel()\n"+
					"\n"+
					"	req := testcontainers.ContainerRequest{\n"+
					"		Image:        \"postgres:17\",\n"+
					"		ExposedPorts: []string{\"5432/tcp\"},\n"+
					"		Env: map[string]string{\n"+
					"			\"POSTGRES_USER\":     \"postgres\",\n"+
					"			\"POSTGRES_PASSWORD\": \"guide\",\n"+
					"			\"POSTGRES_DB\":       \"guide\",\n"+
					"		},\n"+
					"		WaitingFor: wait.ForLog(\"database system is ready to accept connections\").\n"+
					"			WithOccurrence(2).\n"+
					"			WithStartupTimeout(testContainerStartup),\n"+
					"	}"),
			text("tst-tc-details",
				"Three details in that quote carry more weight than their line count. The SKIP is not a failure and not a silence: ``t.Skipf`` names WHAT was missing (the socket) and WHY the test declined, so a contributor running the suite on a laptop without the wiring sees an explicit skipped line instead of a false red or a mystery green — and the suite's exit status never depends on an environment it cannot assume. The IMAGE is the same pinned ``postgres:17`` the compose stack itself runs, so the sibling container costs no extra pull and tests exactly the engine version production uses — engine drift between test and prod is a whole class of non-reproducible bug this one line deletes. And the WAIT is occurrence TWO of ``database system is ready to accept connections`` because postgres logs that sentence twice — once for the init phase's temporary server, once for the real one — and a waiter satisfied by the first occurrence hands the test a connection that is about to be torn down; the flaky test that inspired that ``WithOccurrence(2)`` is now a stable, boring one. What runs BESIDE the smoke test in that package is the cheaper middle: the migrations and schema-assertion tests reuse the stack's already-running PostgreSQL through dedicated throwaway databases (the isolation pattern two sections down), so even without the Docker socket the package still verifies its SQL against the real engine."),
			text("tst-tc-runner-intro",
				"The topic's demo makes the whole discipline executable: it RUNS a curated slice of this repo's own suite — two pure-logic tests from the posts domain, two httptest tests from the presentation layer (one of them the boundary table with its subtests), and the migrations-embed pin from the testcontainers package itself — through the go tool's own ``-json`` event stream, parses every event, and serves the timeline you see: packages with pass/fail/skip counts and durations, tests and subtests in execution order with their verdicts, plus the suite's SHAPE counted from the source tree beside the process. The parser is real and table-tested against the event shapes (interleaved packages, subtests, skips, build failures, non-JSON noise); its core loop is quoted here because reading it IS the lesson — every number the panel shows arrived as an event or a counted file, and nothing else:"),
			code("tst-tc-parser", "go",
				"The demo's engine: one line in, one event folded — unparseable lines counted and sampled, never guessed at.",
				"server/internal/domain/demos/testing_runner.go",
				"		var event testEvent\n"+
					"		if err := json.Unmarshal([]byte(line), &event); err != nil || event.Package == \"\" {\n"+
					"			parsed.Unparsed++\n"+
					"			if len(parsed.UnparsedSample) < 4 {\n"+
					"				parsed.UnparsedSample = append(parsed.UnparsedSample, truncateForDisplay(line, 160))\n"+
					"			}\n"+
					"			continue\n"+
					"		}\n"+
					"\n"+
					"		if event.Test == \"\" {\n"+
					"			applyPackageEvent(&parsed, packageIndex, event)\n"+
					"			continue\n"+
					"		}\n"+
					"		applyTestEvent(&parsed, testIndex, packageIndex, event)"),
			demo("testing:runner", "The runner: this repo's tests, run for real and parsed",
				"A real `go test -json -count=1` run over a curated set of the guide's own test packages — internal/domain/posts (pure logic), internal/presentation (httptest through the real chain), internal/data/db (the testcontainers package's ungated embed pin) — executed ONCE at server boot in the background, bounded by a hard timeout that kills the whole process group, and served here with its wall-clock age. The timeline's packages, tests, subtests, statuses, and durations are all PARSED from the go tool's own events; the shape panel's counts (test files, test packages, test functions, t.Parallel calls) are counted from the source tree at request time; and the three table-test exemplars are located by symbol with their presence re-verified. If the toolchain is missing, the endpoint says so with a typed 503 — it never invents a green timeline."),
			text("tst-tc-runner-mode",
				"One design decision in the demo deserves its own paragraph because it is a testing lesson about TIME, not about tests: the run happens at BOOT, not on the request path. The same single-flight shape the dependency topic's govulncheck scan proved out applies here — the first boot kicks one bounded background run, every request serves the cached outcome with its age, and the request path never waits on a compiler. A scoped ``go test`` with a warm build cache settles in under a second, but a cold cache or a freshly-edited package can pay seconds-to-tens-of-seconds of compilation, and a demo endpoint that sometimes takes fifteen seconds is a demo people reload once and abandon. The timeout that bounds the background run is enforced for real: the runner puts the tool in its own process group and, on expiry, kills the GROUP — ``go test`` spawns the test binaries as children, and killing only the parent would strand them mid-run. Reload the panel: the ages move, the numbers do not — the run is a fact about boot, served with its provenance."),
		),
		// ── 5. The awkward bits ────────────────────────────────────
		section("awkward-bits", "The awkward bits: time, isolation, and races",
			"Three things make tests lie: clocks that do not move when you want them to, state shared between tests, and concurrency — this repo's answers to each are small, boring, and quoted.",
			text("tst-time-clock",
				"TIME is the first liar. A test that proves a TTL expired by sleeping five seconds is a test that costs five seconds FOREVER and still asserts on an accident of scheduling — did the entry expire because the clock crossed the line, or because the sleep drifted past it? This repo's answer is the INJECTABLE CLOCK, and the pattern is visible wherever time is load-bearing: the caching repository's constructor takes a ``now func() time.Time`` parameter, production wires ``time.Now``, and the test wires a stepping clock it advances BY HAND. Expiry becomes an assertion instead of a wait: advance past the TTL, assert the miss, never sleep. The clock is mutex-guarded because the concurrency tests run the same repository from several goroutines under -race — a shared test fixture is still shared state:"),
			code("tst-time-step-clock", "go",
				"The stepping clock: a fixed time the test advances deterministically, injected where production injects time.Now.",
				"server/internal/data/posts/caching_test.go",
				"type cachingStepClock struct {\n"+
					"	mu sync.Mutex\n"+
					"	t  time.Time\n"+
					"}\n"+
					"\n"+
					"func (c *cachingStepClock) Now() time.Time {\n"+
					"	c.mu.Lock()\n"+
					"	defer c.mu.Unlock()\n"+
					"	return c.t\n"+
					"}\n"+
					"\n"+
					"func (c *cachingStepClock) Advance(d time.Duration) time.Time {\n"+
					"	c.mu.Lock()\n"+
					"	defer c.mu.Unlock()\n"+
					"	c.t = c.t.Add(d)\n"+
					"	return c.t\n"+
					"}\n"+
					"// …the wiring: NewCachingRepository(store, 5*time.Second, nil, clock.Now)"),
			text("tst-isolation-db",
				"STATE is the second liar, and its worst habitat is the shared database: two test runs hitting the same tables see each other's rows, and a suite that passes alone fails in company. This repo isolates at two scales, both named by their cleanup. Against the STACK's PostgreSQL (no new container needed), every database-touching test draws a THROWAWAY DATABASE — created through an admin connection to the ``postgres`` maintenance database, named with a prefix plus the test process's pid and a nanosecond timestamp, and dropped with ``WITH (FORCE)`` in ``t.Cleanup`` so no lingering connection can veto the drop; the helper refuses an admin DSN that points at the dev database, because a test suite that drops the shared dev database is a very expensive test indeed:"),
			code("tst-isolation-throwaway", "go",
				"Run-scoped databases: born named, dropped forced — never the dev database.",
				"server/internal/data/db/db_test.go",
				"func newThrowawayPool(t *testing.T) *pgxpool.Pool {\n"+
					"	t.Helper()\n"+
					"	// …admin DSN resolved; refused when it targets the dev database\n"+
					"	name := fmt.Sprintf(\"%s%d_%d\", throwawayPrefix, os.Getpid(), time.Now().UnixNano())\n"+
					"	if _, err := admin.Exec(ctx, \"CREATE DATABASE \"+name); err != nil {\n"+
					"		t.Fatalf(\"create throwaway database %s: %v\", name, err)\n"+
					"	}\n"+
					"	// …a pool connected to the fresh database\n"+
					"	t.Cleanup(func() {\n"+
					"		pool.Close()\n"+
					"		// …DROP DATABASE … WITH (FORCE), then the admin closes\n"+
					"	})\n"+
					"	return pool\n"+
					"}"),
			text("tst-isolation-redis",
				"The same discipline scales DOWN to redis and messaging: every run of the messaging demos mints a THROWAWAY NAMESPACE — a random hex suffix on a fixed prefix — so streams, consumer groups, and keys created by one run can never collide with another's, whether the other is a concurrent request or the test suite itself. The entropy is six bytes from crypto/rand, which is overkill for namespace uniqueness and exactly right for the habit (never derive isolation from a timestamp alone — two runs in the same nanosecond are a real thing on a fast machine):"),
			code("tst-isolation-namespace", "go",
				"Run-scoping without a database engine: a random namespace per run, keys and streams prefixed into it.",
				"server/internal/domain/demos/messaging_stream.go",
				"func NewMessagingDemoNamespace() (string, error) {\n"+
					"	var entropy [6]byte\n"+
					"	if _, err := rand.Read(entropy[:]); err != nil {\n"+
					"		return \"\", fmt.Errorf(\"read messaging demo namespace entropy: %w\", err)\n"+
					"	}\n"+
					"	return MessagingDemoKeyPrefix + \":\" + hex.EncodeToString(entropy[:]), nil\n"+
					"}"),
			text("tst-race-where",
				"CONCURRENCY is the third liar, and Go's answer is structural: the race detector instruments the compiled binary so that every memory access is checked against the happens-before graph — a data race is REPORTED at the moment it occurs, with both stacks, not inferred later from corrupted state. It costs roughly two to twenty times runtime, which is why the honest policy is not ``always -race everywhere`` but ``-race wherever concurrency actually runs`` — in this repo that is the standard verification command for the packages that carry goroutines (the switching facade, the realtime hub, the caching decorator, every demo that hammers). What it catches here is exactly the bug class this architecture invites: the switching repository answers per-call from a mode that can flip mid-request, so its race test INTERLEAVES flippers with hammering callers — two goroutines PATCH the scenario store while four more list, create, and delete through the facade — and the detector proves the snapshot-per-call design never yields a torn answer:"),
			code("tst-race-hammer", "go",
				"The race test: flips and calls interleaved on purpose — run under -race, a torn read becomes a detector report.",
				"server/internal/data/posts/switching_test.go",
				"func TestFacadeFlipsAreRaceClean(t *testing.T) {\n"+
					"	t.Parallel()\n"+
					"\n"+
					"	scenario := core.NewScenarioStore()\n"+
					"	facade := NewSwitchingRepository(scenario, NewInMemoryRepository(), NewInMemoryRepository())\n"+
					"\n"+
					"	var wg sync.WaitGroup\n"+
					"	// …two flippers PATCH the data mode 50 times each, in the clear\n"+
					"	for caller := 0; caller < 4; caller++ {\n"+
					"		wg.Add(1)\n"+
					"		go func() {\n"+
					"			defer wg.Done()\n"+
					"			ctx := context.Background()\n"+
					"			for range 50 {\n"+
					"				page, err := facade.List(ctx, posts.ListQuery{Page: 1, Limit: 5, Sort: posts.SortNewest})\n"+
					"				if err != nil {\n"+
					"					t.Errorf(\"list under flips: %v\", err)\n"+
					"					return\n"+
					"				}\n"+
					"				// …a page of limit 5 carrying more than 5 items is a torn answer\n"+
					"			}\n"+
					"		}()\n"+
					"	}\n"+
					"	wg.Wait()\n"+
					"}"),
			text("tst-race-notes",
				"Two notes complete the honest picture. ``t.Errorf`` inside those goroutines is safe where ``t.Fatalf`` would not be — FailNow must run on the test's own goroutine, so the convention here is Errorf-and-return from workers (the loop's early ``return`` stops that worker's contribution) while the OUTER test Fatals on setup problems; getting this distinction wrong is a classic first concurrency-test bug. And the race detector's own boundary should be stated as plainly as the tool states it: it finds races THAT HAPPEN, not races THAT EXIST — an unsynchronized access on a path the scheduled goroutines never took is invisible to it, which is why the hammer's shape matters (flips AND reads AND writes, dozens of rounds, run repeatedly). The detector raises the floor dramatically; it does not touch the ceiling. The client's tests, next, face the same three liars — and answer them with fixtures and stubs instead of clocks and namespaces."),
		),
		// ── 6. Client testing ──────────────────────────────────────
		section("client-testing", "The client's half: vitest specs at three layers",
			"The React client mirrors the server's layering in TypeScript, and so do its tests: DTO specs validate the wire, repository specs guard the transport with a stubbed fetch, component specs render real components against fixture data.",
			text("tst-client-shape",
				"The client is a presentation-only shell, but ``presentation-only`` is not ``test-free`` — it just means the tests guard different seams. The three layers map one-to-one onto the server's tiering. DTO SPECS are the unit tier: the ``fromJson`` decoders are pure functions from unknown JSON to typed domain shapes, so their specs are tables of payloads — valid bodies that must build, and subtly-broken bodies (a missing array, a wrong-typed field) that must THROW, because the parse pipeline upgrades a throw into a typed InvalidResponse failure. REPOSITORY SPECS are the transport tier: the real repository adapter runs, but ``fetch`` is stubbed at the global boundary — one stub answers a fixture payload, another answers with a broken body, a third simulates the network being down — and the spec pins that every outcome resolves as a typed Result, never an unhandled rejection. COMPONENT SPECS are the wiring tier: the real component renders in jsdom, its use case is swapped at the container token for one returning a fixture or a failure, and the spec asserts what the user would see — the loading state first, then the panels, or the error panel with a working retry. One real example of each, quoted from the observability topic's client slice:"),
			code("tst-client-dto", "ts",
				"The DTO spec: a whitelist builder — the valid payload must build, the violated shapes must throw.",
				"client/src/data/demo/obs-trace-dto.spec.ts",
				"describe('obsTraceDtoFromJson', () => {\n"+
					"  it('builds the domain payload from a valid wire body', () => {\n"+
					"    const dto = obsTraceDtoFromJson(fullPayload);\n"+
					"\n"+
					"    expect(dto.accepted.traceId).toBe('4bf92f3577b34da6a3ce929d0e0e4736');\n"+
					"    expect(dto.tree).toHaveLength(2);\n"+
					"  });\n"+
					"  // …\n"+
					"  it('rejects a payload missing the span tree', () => {\n"+
					"    const { tree: _tree, ...withoutTree } = fullPayload;\n"+
					"\n"+
					"    expect(() => obsTraceDtoFromJson(withoutTree)).toThrow();\n"+
					"  });\n"+
					"});"),
			code("tst-client-repository", "ts",
				"The repository spec: the REAL adapter over a stubbed fetch — success, malformed, and network-down all become typed Results.",
				"client/src/data/demo/obs-trace-repository-impl.spec.ts",
				"describe('ObsTraceRepositoryImpl', () => {\n"+
					"  it('returns a Success carrying the validated domain payload', async () => {\n"+
					"    stubApiFetch([{ url: '/api/demo/obs/trace', body: demoPayload }]);\n"+
					"\n"+
					"    const result = await makeRepository().get();\n"+
					"\n"+
					"    expect(result.ok).toBe(true);\n"+
					"    // …field-level assertions on the domain payload\n"+
					"  });\n"+
					"\n"+
					"  it('guards a network failure into a Failed Result (network_error, status 0)', async () => {\n"+
					"    stubNetworkFailure();\n"+
					"    // …the failure asserts its own code and status\n"+
					"  });\n"+
					"});"),
			code("tst-client-component", "tsx",
				"The component spec: the real component, a fixture-backed use case injected at the container token.",
				"client/src/ui/demos/obs/trace_demo.spec.tsx",
				"function stubUseCase(execute: () => Promise<Result<ObsTraceDemoData>>): void {\n"+
					"  container.initialize(createAppConfig());\n"+
					"  container.register(\n"+
					"    OBS_TRACE_DEMO_USE_CASE,\n"+
					"    { execute } as unknown as GetObsTraceDemoUseCase,\n"+
					"  );\n"+
					"}\n"+
					"// …the happy spec renders, awaits the ready panel, and asserts\n"+
					"// on what the USER sees; the failure spec stubs a network_error\n"+
					"// and clicks 'Try again' to prove recovery re-runs the load"),
			text("tst-client-notes",
				"The discipline that keeps these three layers honest is the same one the server applies to httptest: test through the REAL boundary and stub at the EDGE. The DTO spec never mocks the validator, the repository spec never mocks the guards — it stubs ``fetch`` itself, the true process boundary, so everything inside (URL building, the guarded pipeline, DTO parsing, Result wrapping) runs for real on every test. The component spec stubs the USE CASE at the container token for the same reason: the component's real dependency-injection path, its loading and error states, and its retry wiring all execute; only the network is replaced by a fixture. And the fixtures are honest by construction — the trace-demo fixture carries the same span ids the server's own demo test pins, so if the wire contract drifts, the DTO spec fails against the fixture AND the server test fails against the handler, both pointing at the same seam from both sides of it."),
		),
		// ── 7. Coverage ────────────────────────────────────────────
		section("coverage", "Coverage: what the number buys, and what it doesn't",
			"`go test -cover` measures one specific thing — which lines ran — and this repo treats it as a map, while guarding correctness with pins and ratchets that fail when content drifts.",
			text("tst-coverage-buys",
				"Line coverage answers exactly one question: WHICH LINES OF PRODUCTION CODE DID SOME TEST EXECUTE. That is genuinely useful — as a MAP. A package at 40% coverage has territory no test walks, and the uncovered lines are usually more interesting than the percentage: the error paths nobody provokes, the boundary branches nobody pins. Run ``go test -cover ./...`` and read the per-package numbers as a where-to-look-next list, not a score. What the number does NOT buy is the thing teams often hope it does: it cannot tell you the assertions were MEANINGFUL, because a test that calls a function and asserts nothing covers its lines just as completely; it cannot tell you the EDGE CASES exist, because coverage counts the cases you ran, not the cases you should have; and it rounds to zero the difference between code that ran and behavior that was verified. A suite can climb to 95% while every assertion is ``if err != nil { t.Error }`` — and the bug ships anyway."),
			text("tst-coverage-pins",
				"So this repo spends its guarding budget where a percentage cannot go: on PINS that fail when the content itself drifts. Every topic's per-topic test pins its exact section order, block-id uniqueness, the inline placement of its demos, and a PROSE FLOOR — the words a topic must carry, counted with ``strings.Fields`` — because the failure mode of a teaching repo is not a crashing function but silently thinning content. And the snippet ratchet (zero unsourced must-source blocks, enforced by test) is a mutation-ish guard in the exact sense coverage is not: it does not ask ``did this code run`` but ``does every quoted line still exist in the file it claims`` — a change to any quoted source line anywhere in the repo turns a green suite red with the offending block id. Both quoted here, because they are the honest answer to ``how do you test a guide``:"),
			code("tst-coverage-floor", "go",
				"The prose floor: a topic that quietly loses a third of its words fails HERE, at build time.",
				"server/internal/content/topic_observability_test.go",
				"func TestObservabilityProseMeetsTheFamilyFloor(t *testing.T) {\n"+
					"	words := 0\n"+
					"	for _, section := range builtObservabilityTopic(t).Sections {\n"+
					"		for _, block := range section.Blocks {\n"+
					"			if text, ok := block.(TextBlock); ok {\n"+
					"				words += len(strings.Fields(text.Text))\n"+
					"			}\n"+
					"		}\n"+
					"	}\n"+
					"	if words < minObservabilityProseWords {\n"+
					"		t.Errorf(\"prose = %d words, want at least %d\", words, minObservabilityProseWords)\n"+
					"	}\n"+
					"}"),
			code("tst-coverage-ratchet", "go",
				"The ratchet: zero unsourced snippets, canary-proven — a harness that cannot bite guards nothing.",
				"server/internal/content/snippet_sources_test.go",
				"func TestCanaryHarnessFlagsExactlyTheBadBlock(t *testing.T) {\n"+
					"	// A real-file-sourced block with an invented line — flagged, and only\n"+
					"	// it: the sourced go.mod block passes, the bash terminal tour is exempt.\n"+
					"	good, err := NewCodeBlock(\"good-mod\", \"mod\", \"\", \"server/go.mod\", \"module guide\")\n"+
					"	// …\n"+
					"	bad, err := NewCodeBlock(\"bad-go\", \"go\", \"\", \"server/cmd/guide/main.go\", \"THIS_LINE_EXISTS_IN_NO_REPO_FILE = true\")\n"+
					"	// …the canary registry: exactly the bad block is flagged\n"+
					"}"),
			text("tst-coverage-canary",
				"That canary deserves one more sentence, because it is the difference between a guard and a ritual: a ratchet test that has never failed anyone proves nothing, so the suite deliberately PLANTS a violation — a snippet line that exists in no repo file — and asserts the harness flags exactly it and nothing else. The pattern generalizes to every gate in this topic: a skip that never skips, a timeout that never fires, a ratchet that never bites are all the same failure — machinery believed in but never exercised — and the testing cure is uniform: prove the guard works by feeding it a controlled failure. The demo's own parser test does this with a fake event stream that includes non-JSON noise and duplicate events, and the runner's timeout test does it with a command that genuinely runs away. Trust guards that have been seen to trip."),
		),
		// ── 8. The CI contract ─────────────────────────────────────
		section("ci-contract", "The CI contract: local verification is the only gate",
			"This family of guides runs no hosted CI by deliberate stance — the verification command is the contract, and the terminal companion's first block is that command, runnable anywhere the repo clones.",
			text("tst-ci-stance",
				"There is no ``.github/`` in this repository, and that is a decision with reasons, not an omission. The family's stance: hosted CI is a PAIR with open-source contribution flow — strangers opening pull requests need an automated neutral party to run the suite. A single-maintainer teaching repo with a devcontainer has no strangers; it has one verification environment, reproducible by construction (the container), and the honest gate is the command run IN that environment before anything lands. The failure mode the stance avoids is the badge that lies — a hosted runner with a different Docker daemon, different socket wiring, different network, silently skipping every testcontainers-gated case and reporting green. This repo's gated tests skip LOCALLY with a named reason when their dependency is absent; a hosted runner would turn those skips into a green checkmark that verified less than it appeared to. Better one honest local gate everyone runs than a distant green light with quietly reduced jurisdiction."),
			text("tst-ci-command",
				"THE CONTRACT, then, is the exact pair of commands the repo's own development ledger names as its definition of done — the server suite and the client suite, both run inside the long-lived dev containers (Protocol v2 in the ledger: docker exec on the standing containers, no ephemeral churn, image rebuilds only when Dockerfiles change). The server line is ``gofmt -l . && go build ./... && go vet ./... && go test ./...`` — formatting SILENT (a file listed here is a failure), then compilation, then vet's static analysis, then the whole suite including every gated test that can run where you are. The client line is ``npm test && npm run build`` — the vitest specs AND the production build, because a type error that tests survive but the build rejects is still a broken tree. A topic lands when both are green and its endpoint has been curled against the running stack; nothing else certifies."),
			list("tst-ci-hosted-would-run",
				false,
				"Same commands, neutral hardware — the exact docker exec pair in a runner that provisions the same devcontainer, so the gate is the contract verbatim rather than a translation of it.",
				"The docker-socket tests unsuppressed — a hosted runner WITH a Docker daemon would let the testcontainers-gated cases actually execute instead of skipping, extending the gate's jurisdiction rather than shrinking it.",
				"The race pass explicit — go test -race ./... as its own step, so concurrency reports are first-class failures with their own red X rather than folded into a generic test failure.",
				"A coverage map, not a gate — go test -cover ./... published as per-package numbers for humans to read as a where-to-look list, deliberately not a threshold that fails the build (the previous section's argument, enforced).",
			),
			text("tst-ci-coda",
				"What makes this a CONTRACT rather than a habit is that the suite itself enforces the parts of it that software can: the formatting check is a command whose failure mode is output (any filename), the pins and ratchet fail the suite when content drifts, the gated tests refuse to pass silently in environments that cannot exercise them, and this topic's demo runs the tool on demand so the reader can WATCH the machinery work. The remaining part — a human running the pair of commands before saying done — is enforced by nothing but the ledger, which is exactly as it should be: the tools verify what tools can, and the process owns the rest."),
		),
		// ── 9. Browser E2E ────────────────────────────────────────
		section("browser-e2e", "Browser E2E: the boundary the suite draws",
			"This repo's browser tier is one small host-run Playwright suite — and the honest way to teach E2E is to state precisely what it covers that no in-process spec in this repo can.",
			text("tst-e2e-plan",
				"The shape, as the repo ships it: a top-level ``e2e/`` directory holding one host-run Playwright suite that drives the EXTERNALLY MANAGED STACK — the real server in its container, the real client front door, the compose wiring between them — through a real browser. Three design choices in that sentence are load-bearing. HOST-RUN (not in-container): Playwright installs browser binaries and talks to them over CDP, and the dev containers deliberately stay lean; the suite runs on the host against the stack the compose files bring up, which also makes it the first tier that verifies the COMPOSITION itself — compose file, wiring, ports, the proxy the client uses. NO BOOT OF ITS OWN: the config has no ``webServer`` block on purpose — the suite drives the stack that is already up, so a red spec means the wiring is red, not that a boot race was lost. And THE DEVELOPER'S FRONT DOOR (overridable): the default base URL is the dev client (:5175), whose /api proxy forwards to the Go server — the same door a developer's browser uses — with ``E2E_BASE_URL`` able to point the same specs at a production front door instead."),
			text("tst-e2e-boundary",
				"What does that tier cover that nothing here can? The seams BETWEEN processes: the client's proxy actually forwarding to the server's port; a page actually loading its content and rendering a topic end to end (React suspense boundaries, route lazy-loading, the demo registry's dynamic imports — none of which execute meaningfully in jsdom); a demo actually fetched, rendered, and interacted with in a layout engine that has a real one; the theme and scenario panels actually persisting their state across reloads. What it deliberately does NOT cover is logic — every assertion of behavior belongs to the millisecond tiers below, and an E2E suite that re-asserts DTO validation through a browser is paying seconds to re-prove what a unit spec proves in microseconds. The boundary, stated as a rule: E2E owns JOURNEYS (a user opens the guide, navigates to a topic, works a demo, toggles the scenario, still sees consistent state), and the tiers below own BEHAVIOR. The suite exists — seven spec files, eleven tests, deliberately thin — and the guide's honest stance is that you can read those specs as this boundary, enforced."),
		),
		// ── 10. Terminal companion ────────────────────────────────
		section("terminal-companion", "Terminal companion: the testing tour",
			"The whole topic over a terminal: the runner's parsed timeline pulled apart field by field, the same tests run yourself with -v and -cover, the race pass on a package that deserves it, and the one-line contract.",
			text("term-tst-intro",
				"With the stack up (server on :8100), the tour runs in the order the topic taught. First the DEMO's payload — the same parsed timeline the panel draws, read raw: the mode line (boot-cached, honestly labeled), the run panel with its age and totals, the package rows with their counted verdicts, and the timeline's subtests picked out by their slashed names. Then the same curated tests run YOURSELF, verbosely, so the tool's own narration (=== RUN, --- PASS, subtests indented) is visibly the same stream the demo parses — plus -count=1 so you see a real execution rather than a cache hit, and -cover on one package so the map from the coverage section has numbers under it. Then the race pass over a package that actually carries goroutines. Last, the contract line itself — the exact server-side gate this topic argued is the CI contract, ready to paste."),
			code("term-tst-demo-bash", "bash", "", "",
				"# the runner: the boot-cached run's parsed timeline, read raw\n"+
					"curl -s localhost:8100/api/demo/testing/runner | python3 -c \\\n"+
					"  'import json,sys; p=json.load(sys.stdin); print(p[\"mode\"]); \\\n"+
					"   r=p[\"run\"]; print(r[\"state\"], \"age\", round(r[\"ageSeconds\"]), \"s, totals:\", \\\n"+
					"   r[\"totalPassed\"], \"passed /\", r[\"totalFailed\"], \"failed /\", r[\"totalSkipped\"], \"skipped\")'\n"+
					"curl -s localhost:8100/api/demo/testing/runner | python3 -c \\\n"+
					"  'import json,sys; p=json.load(sys.stdin); \\\n"+
					"   [print(pk[\"outcome\"], pk[\"package\"], str(pk[\"durationMs\"])+\"ms\") for pk in p[\"packages\"]]'\n"+
					"curl -s localhost:8100/api/demo/testing/runner | python3 -c \\\n"+
					"  'import json,sys; p=json.load(sys.stdin); \\\n"+
					"   [print(\"  \"*t[\"depth\"] + t[\"test\"], t[\"status\"]) for t in p[\"tests\"]]'\n"+
					"# the suite's shape, counted live: files, packages, functions, parallel calls\n"+
					"curl -s localhost:8100/api/demo/testing/runner | python3 -c \\\n"+
					"  'import json,sys; s=json.load(sys.stdin)[\"shape\"]; \\\n"+
					"   print(s[\"testPackages\"], \"packages /\", s[\"testFiles\"], \"files /\", \\\n"+
					"   s[\"testFunctions\"], \"functions /\", s[\"parallelCalls\"], \"t.Parallel calls\"); \\\n"+
					"   [print(e[\"symbol\"], \"—\", \"verified\" if e[\"present\"] else \"MISSING\") for e in s[\"tableExamples\"]]'\n"+
					"\n"+
					"# the toolchain hiccup, degraded honestly (prod image: no toolchain beside the binary)\n"+
					"# curl -i localhost:8100/api/demo/testing/runner  → 503 service_unavailable, reason in the envelope"),
			code("term-tst-suite-bash", "bash", "", "",
				"# the same curated tests, run yourself — the tool's own narration IS the parsed stream\n"+
					"cd server\n"+
					"go test -v -count=1 -run 'TestPostsCreateRefusesBodiesAtTheBoundary' ./internal/presentation\n"+
					"go test -v -count=1 -run 'TestDraftValidateCollectsEveryIssue' ./internal/domain/posts\n"+
					"\n"+
					"# coverage as a MAP: which lines walked, per package\n"+
					"go test -cover ./internal/domain/posts ./internal/domain/obs\n"+
					"go test -coverprofile=/tmp/cover.out ./internal/domain/obs && go tool cover -func=/tmp/cover.out | tail -3\n"+
					"\n"+
					"# the race pass where concurrency actually runs\n"+
					"go test -race -count=1 -run 'TestFacadeFlipsAreRaceClean' ./internal/data/posts\n"+
					"\n"+
					"# the CI contract, verbatim — the gate itself\n"+
					"gofmt -l . && go build ./... && go vet ./... && go test ./..."),
			text("term-tst-coda",
				"Read the tour as the topic's whole argument compressed. The runner GETs are PARSED PROVENANCE — a payload whose every number arrived as a go-tool event or a counted file, with its age served beside it and its degradation path typed and honest. The ``-v`` runs are THE SAME STREAM — the narration you read in the terminal is byte-for-byte the event source the demo's parser folds into its timeline, which is the strongest possible version of ``the demo is real``: you can diff them by eye. The cover commands are the MAP stance from the coverage section, with ``go tool cover -func`` turning the profile into the per-function where-to-look list; the race pass runs where races actually live; and the last line is the contract — formatting silent, build green, vet green, suite green, in the environment that can run all of it. The pyramid, the tables, the chains, the containers, the clocks, the namespaces, the fixtures, the pins: every section of this topic was one of those nouns, and the tour runs them all in four commands."),
		),
	}

	if buildErr != nil {
		return Topic{}, buildErr
	}
	return NewTopic(
		"testing",
		"Testing in Go: the pyramid this repo actually runs",
		"How this guide verifies itself, taught from its own test code: the pyramid described honestly — unit-heavy tables, httptest chains, integration where it pays, one thin host-run E2E suite; no dogma, just the real distribution.",
		"platform",
		34,
		sections,
	)
}
