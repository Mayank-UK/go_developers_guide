// Command guide is the Go Developer's Guide API server. Its composition
// root lives here: load typed environment config, build the slog JSON
// logger, wire the HTTP routes, and serve until SIGINT/SIGTERM triggers a
// graceful shutdown. Dependencies are injected by construction — no DI
// framework, no globals.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"guide/internal/content"
	"guide/internal/core"
	"guide/internal/data/auth"
	authzdata "guide/internal/data/authorization"
	"guide/internal/data/cachinglab"
	"guide/internal/data/db"
	filesdata "guide/internal/data/files"
	datai18n "guide/internal/data/i18n"
	jobsdata "guide/internal/data/jobs"
	"guide/internal/data/messaging"
	"guide/internal/data/mongodb"
	obsdata "guide/internal/data/obs"
	"guide/internal/data/ormprobe"
	postsdata "guide/internal/data/posts"
	"guide/internal/data/redis"
	"guide/internal/data/security"
	domauthz "guide/internal/domain/authorization"
	"guide/internal/domain/demos"
	domobs "guide/internal/domain/obs"
	"guide/internal/domain/posts"
	"guide/internal/domain/realtime"
	"guide/internal/grpcapi"
	"guide/internal/presentation"
)

// shutdownTimeout bounds graceful shutdown: in-flight requests get this
// long to finish before the process gives up and exits non-zero.
const shutdownTimeout = 10 * time.Second

func main() {
	if err := run(); err != nil {
		// The configured logger may not exist yet (config load failure),
		// so fatal boot errors land on a minimal stderr JSON logger.
		slog.New(slog.NewJSONHandler(os.Stderr, nil)).Error("fatal", "error", err)
		os.Exit(1)
	}
}

// run is the composition root: config → logger → content registry →
// routes + middleware → serve → shut down. Operational failures return an
// error for main to exit non-zero — panics are reserved for programmer
// bugs, which this code has none of.
func run() error {
	// Boot-scoped context: dials and migrations get a bounded window so a
	// wedged database fails the boot deterministically instead of hanging
	// it. Once serving starts, request lifetimes belong to the signal
	// context below.
	bootCtx, bootCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer bootCancel()

	cfg, err := core.LoadConfig(os.Getenv)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	logger := core.NewLogger(cfg)

	// PostgreSQL is part of the stack, not an optional service: dial it,
	// run the embedded goose migrations to the latest version, fail the
	// boot if either step fails. The pool then serves the whole process —
	// the introspection demo now, the posts repository from topic 17 on.
	pool, err := db.NewPool(bootCtx, cfg.DatabaseURL)
	if err != nil {
		logger.Error("database unreachable", "error", err)
		return fmt.Errorf("database: %w", err)
	}
	defer pool.Close()
	applied, err := db.Migrate(bootCtx, pool)
	if err != nil {
		logger.Error("database migrations failed", "error", err)
		return fmt.Errorf("migrate: %w", err)
	}
	logger.Info("database migrations applied", "applied", applied)

	// The seed is data, not structure — deliberately NOT a migration: it
	// must be re-runnable on every boot, and ON CONFLICT DO NOTHING makes
	// it one (a fresh database inserts the six guide posts, every later
	// boot inserts nothing). The topic 17 repository reads this seed in
	// live mode; the in-memory fake serves the same collection from code.
	seeded, err := postsdata.Seed(bootCtx, pool)
	if err != nil {
		logger.Error("posts seed failed", "error", err)
		return fmt.Errorf("seed: %w", err)
	}
	logger.Info("posts seed applied", "inserted", seeded)

	// The content registry validates every registered topic at startup and
	// fails the process here — broken content never reaches a request.
	registry, err := content.NewProductionRegistry()
	if err != nil {
		logger.Error("content registry invalid", "error", err)
		return fmt.Errorf("content registry: %w", err)
	}

	mux := http.NewServeMux()
	mux.Handle("GET /api/health", presentation.NewHealthHandler(cfg.Env))
	mux.Handle("GET /api/content", presentation.NewContentListHandler(registry))
	mux.Handle("GET /api/content/{topicId}", presentation.NewContentTopicHandler(registry, logger))
	scenarioStore := core.NewScenarioStore()
	mux.Handle("GET /api/demo/scenario", presentation.NewScenarioGetHandler(scenarioStore))
	mux.Handle("PATCH /api/demo/scenario", presentation.NewScenarioPatchHandler(scenarioStore, logger))
	mux.Handle("GET /api/demo/go/runtime-types", presentation.NewRuntimeTypesHandler())
	mux.Handle("GET /api/demo/runtime/concurrency", presentation.NewConcurrencyHandler())
	mux.Handle("GET /api/demo/http/headers", presentation.NewHTTPHeadersHandler())
	mux.Handle("GET /api/demo/gohttp/app-info", presentation.NewAppInfoHandler(cfg))
	mux.Handle("GET /api/demo/iface/matrix", presentation.NewInterfaceMatrixHandler())
	mux.Handle("GET /api/demo/conc/context", presentation.NewContextCancellationHandler())
	mux.Handle("GET /api/demo/conc/race", presentation.NewRaceDetectorHandler())
	mux.Handle("GET /api/demo/routing/explorer", presentation.NewRoutingExplorerHandler())
	mux.Handle("GET /api/demo/middleware/pipeline", presentation.NewMiddlewarePipelineHandler())
	mux.Handle("GET /api/demo/reqres/inspector", presentation.NewReqresInspectorHandler())
	mux.Handle("GET /api/demo/reqres/methods", presentation.NewReqresMethodsHandler())
	mux.Handle("GET /api/demo/validation/arena", presentation.NewValidationArenaHandler())
	mux.Handle("GET /api/demo/api/posts-workbench", presentation.NewPostsWorkbenchHandler())
	mux.Handle("GET /api/demo/api/webhook-simulator", presentation.NewWebhookSimulatorHandler())
	mux.Handle("GET /api/demo/api/docs-link", presentation.NewAPIDocsLinkHandler(logger))
	mux.Handle("GET /api/openapi.yaml", presentation.NewOpenAPISpecHandler())
	mux.Handle("GET /api/demo/api/grpc-console", presentation.NewGRPCConsoleHandler())
	mux.Handle("GET /api/demo/db/schema-explorer", presentation.NewDBSchemaExplorerHandler(db.NewIntrospector(pool), logger))
	mux.Handle("GET /api/demo/db/uuid", presentation.NewDBUUIDHandler(logger))
	// The databases topic's six demos all run against the live pool
	// through one runner: safety, transactions, SQL-by-hand, search,
	// isolation, and the in-transaction audit — each degrading to the
	// typed 503 when the database is unreachable.
	sqlDemos := db.NewSQLDemoRunner(pool)
	mux.Handle("GET /api/demo/db/sql-safety", presentation.NewDBSQLSafetyHandler(sqlDemos, logger))
	mux.Handle("GET /api/demo/db/migrations", presentation.NewDBMigrationsHandler(sqlDemos, logger))
	mux.Handle("GET /api/demo/db/sql-lab", presentation.NewDBSQLLabHandler(sqlDemos, logger))
	mux.Handle("GET /api/demo/db/search", presentation.NewDBSearchHandler(sqlDemos, logger))
	mux.Handle("GET /api/demo/db/race", presentation.NewDBRaceHandler(sqlDemos, logger))
	mux.Handle("GET /api/demo/db/audit", presentation.NewDBAuditHandler(sqlDemos, logger))

	// The sqlc-gorm topic's probe runs BOTH query stacks against the same
	// PostgreSQL the server already migrated — same DSN, measured live.
	mux.Handle(
		"GET /api/demo/db/orm-probe",
		presentation.NewDBORMProbeHandler(ormprobe.NewRunner(cfg.DatabaseURL), logger),
	)

	// MongoDB is an OPTIONAL service (the topic's own discipline): connect
	// inside the boot window; ANY failure — dial or event-store construction
	// — degrades the console to the honest unavailable runner instead of
	// failing the boot.
	mongoClient, err := mongodb.Connect(bootCtx, cfg.MongoURI)
	var mongoRunner demos.MongoConsoleRunner
	if err != nil {
		logger.Warn("mongo unavailable; console demo degrades", "error", err)
		mongoRunner = mongodb.NewUnavailableConsoleRunner(err)
	} else {
		defer mongoClient.Disconnect(context.Background())
		store, storeErr := mongodb.NewEventStore(bootCtx, mongoClient, "guide_demos")
		if storeErr != nil {
			logger.Warn("mongo event store unavailable; console demo degrades", "error", storeErr)
			mongoRunner = mongodb.NewUnavailableConsoleRunner(storeErr)
		} else {
			mongoRunner = mongodb.NewConsoleRunner(store)
		}
	}
	mux.Handle("GET /api/demo/db/mongo-console", presentation.NewDBMongoConsoleHandler(mongoRunner, logger))

	// Redis is wired LAZY — no boot ping at all: the client dials on first
	// use, a down Redis never blocks boot, and every failure maps to the
	// playground's typed 503 at request time.
	redisClient := redis.NewClient(cfg.RedisAddr)
	defer redisClient.Close()
	mux.Handle(
		"GET /api/demo/redis/playground",
		presentation.NewRedisPlaygroundHandler(redis.NewPlaygroundRunner(redisClient), logger),
	)

	// Topic 21's caching lab: the REAL cache-aside decorator on throwaway
	// stores — cold/warm beats, write invalidation, TTL expiry on a fixture
	// clock, the coalesced herd, and the fake/live store swap — on a payload
	// deterministic enough to carry a strong ETag and answer a replayed
	// If-None-Match with an honest 304.
	mux.Handle("GET /api/demo/caching/lab", presentation.NewCachingLabHandler(cachinglab.New(), logger))

	// Topic 22's messaging pair, both on the live stack the server already
	// holds: the streams demo runs a real consumer-group choreography over
	// Redis Streams, the outbox demo writes the outbox row and the business
	// change in ONE PostgreSQL transaction and relays it into the stream —
	// each degrading to the typed 503 when its backend is unreachable.
	mux.Handle(
		"GET /api/demo/msg/stream",
		presentation.NewMessagingStreamHandler(messaging.NewStreamStoryRunner(redisClient), logger),
	)
	mux.Handle(
		"GET /api/demo/msg/outbox",
		presentation.NewMessagingOutboxHandler(messaging.NewOutboxStoryRunner(pool, redisClient), logger),
	)

	// Topic 23's authentication console: the shared substrate the seven
	// auth demos walk — seeded users, in-memory stores, every engine —
	// plus the auth middleware's guarded proof route. The console is
	// process-local by design (the topic teaches the Redis trade-off),
	// so nothing here can fail a boot or degrade another endpoint.
	authConsole, err := auth.NewConsole()
	if err != nil {
		return fmt.Errorf("auth console: %w", err)
	}
	mux.Handle("GET /api/demo/auth/sessions", presentation.NewAuthSessionsHandler(authConsole, logger))
	mux.Handle("GET /api/demo/auth/api-keys", presentation.NewAuthAPIKeysHandler(authConsole, logger))
	mux.Handle("GET /api/demo/auth/compare", presentation.NewAuthCompareHandler(authConsole, logger))
	mux.Handle("GET /api/demo/auth/jwt", presentation.NewAuthJWTHandler(authConsole, logger))
	mux.Handle("GET /api/demo/auth/oauth", presentation.NewAuthOAuthHandler(authConsole, logger))
	mux.Handle("GET /api/demo/auth/hashing", presentation.NewAuthHashingHandler(authConsole, logger))
	mux.Handle("GET /api/demo/auth/refresh", presentation.NewAuthRefreshHandler(authConsole, logger))
	mux.Handle("GET /api/demo/auth/whoami",
		presentation.RequireIdentity(authConsole.Resolver, logger)(presentation.NewAuthWhoamiHandler(logger)))

	// Topic 24's authorization pair: the matrix demo tells the whole
	// roles × scopes × ownership story unauthenticated, and two guarded
	// proof routes enforce it FOR REAL on top of topic 23's identity
	// middleware — RequirePermission for a flat RBAC check,
	// RequireOwnership for the object-level one.
	authzConsole := authzdata.NewConsole()
	mux.Handle("GET /api/demo/authz/matrix", presentation.NewAuthzMatrixHandler(authzConsole, logger))
	mux.Handle("GET /api/demo/authz/guarded",
		presentation.RequireIdentity(authConsole.Resolver, logger)(
			presentation.RequirePermission(authzConsole, domauthz.PermPostsWrite, logger)(
				presentation.NewAuthzGuardedHandler(domauthz.PermPostsWrite, logger))))
	mux.Handle("GET /api/demo/authz/guarded/post/{id}",
		presentation.RequireIdentity(authConsole.Resolver, logger)(
			presentation.RequireOwnership(authzConsole, authzConsole, domauthz.PermPostsWrite, logger)(
				presentation.NewAuthzGuardedPostHandler(domauthz.PermPostsWrite, authzConsole, logger))))

	// Topic 25's security pair, wrapped in their own teaching: the
	// headers middleware sits OUTSIDE the rate limiter so a 429 reply
	// still carries the full security-header set, and one shared limiter
	// (2/s sustained, burst 5, keyed per client) makes the demo's 429
	// boundary reachable by a plain curl loop.
	securityConsole, err := security.NewConsole()
	if err != nil {
		return fmt.Errorf("security console: %w", err)
	}
	mux.Handle("GET /api/demo/security/lab",
		presentation.Chain(
			presentation.SecurityHeaders(securityConsole.Headers),
			presentation.RateLimit(securityConsole.RouteLimiter, logger),
		)(presentation.NewSecurityLabHandler(securityConsole, logger)))
	mux.Handle("GET /api/demo/security/pii",
		presentation.Chain(
			presentation.SecurityHeaders(securityConsole.Headers),
			presentation.RateLimit(securityConsole.RouteLimiter, logger),
		)(presentation.NewSecurityPIIHandler(securityConsole, logger)))

	// Topic 26's files workbench: real streaming routes over the local
	// store — a bounded multipart upload, a Content-Disposition download
	// that honors ranges, a stdlib thumbnail transform, and the story
	// payload that narrates the same machinery.
	filesConsole, err := filesdata.NewConsole()
	if err != nil {
		return fmt.Errorf("files console: %w", err)
	}
	mux.Handle("POST /api/demo/files/upload",
		presentation.NewFilesUploadHandler(filesConsole.Store, filesConsole.UploadLimit, logger))
	mux.Handle("GET /api/demo/files/download/{name...}",
		presentation.NewFilesDownloadHandler(filesConsole.Store, logger))
	mux.Handle("GET /api/demo/files/thumb/{name...}",
		presentation.NewFilesThumbHandler(filesConsole.Store, logger))
	mux.Handle("GET /api/demo/files/workbench",
		presentation.NewFilesWorkbenchHandler(filesConsole, logger))

	// Topic 27's negotiator reads the request's ACTUAL Accept-Language
	// header and answers with the parsed candidates, the matched catalog,
	// and the same message per locale.
	datai18nConsole, err := datai18n.NewConsole()
	if err != nil {
		return fmt.Errorf("i18n console: %w", err)
	}
	mux.Handle("GET /api/demo/i18n/negotiator",
		presentation.NewI18nNegotiatorHandler(datai18nConsole, logger))

	// Topic 28's realtime pair: story payloads describe the mechanics,
	// while the stream and room routes ARE the mechanics — one hub, one
	// ticker feed broadcasting into the "ticker" room, SSE with
	// Last-Event-ID replay, and a WebSocket chat room with a
	// slow-consumer policy and honest close codes.
	rtHub, err := realtime.NewHub(realtime.DefaultHubPolicy())
	if err != nil {
		return fmt.Errorf("realtime hub: %w", err)
	}
	rtFeed, err := realtime.NewTickerFeed(rtHub, "ticker", 64, realtime.FeedOptions{Interval: time.Second})
	if err != nil {
		return fmt.Errorf("realtime ticker feed: %w", err)
	}
	mux.Handle("GET /api/demo/realtime/ticker", presentation.NewRealtimeTickerStoryHandler())
	mux.Handle("GET /api/demo/realtime/chat", presentation.NewRealtimeChatStoryHandler())
	mux.Handle("GET /api/demo/realtime/ticker/stream",
		presentation.NewRealtimeTickerStreamHandler(rtFeed, 3000, 15*time.Second, logger))
	mux.Handle("GET /api/demo/realtime/chat/ws",
		presentation.NewRealtimeChatHandler(rtHub, "chat", realtime.DefaultChatPolicy(),
			[]string{"localhost:5175", "127.0.0.1:5175"}, logger))

	// Topic 29's background jobs: the console demo choreographs real
	// enqueue → retry/backoff → archive/replay cycles against the hosted
	// asynq worker, which shares the process's redis client and rides the
	// same boot context and graceful shutdown as everything else.
	jobsWorker := jobsdata.NewServerRunner(redisClient)
	mux.Handle("GET /api/demo/jobs/console",
		presentation.NewJobsConsoleHandler(jobsdata.NewConsoleStoryRunner(redisClient), logger))

	// Topic 30's error-handling pair: the explorer narrates the taxonomy
	// and walks the scenario-honoring probe live through the production
	// middleware chain, and the probe IS the teaching — it answers
	// healthy, or the deliberate panic the recovery layer converts into
	// the 500 envelope, or the latency injection slept out before 200.
	mux.Handle("GET /api/demo/errors/explorer",
		presentation.NewErrorsExplorerHandler(scenarioStore, logger))
	mux.Handle("GET /api/demo/errors/probe",
		presentation.NewErrorsProbeHandler(scenarioStore))

	// Topic 31's dependency manifest: the module's own go.mod parsed
	// live beside the serving binary's build info, the tools/deps.go
	// anchors, the Docker cache volumes, and a govulncheck scan that
	// runs for real when the source tree sits beside the process (the
	// dev container: `go run` from server/) and degrades honestly when
	// it does not (the distroless prod image ships no sources).
	depsModuleDir := ""
	if wd, wdErr := os.Getwd(); wdErr == nil {
		if _, statErr := os.Stat(filepath.Join(wd, "go.mod")); statErr == nil {
			depsModuleDir = wd
		}
	}
	mux.Handle("GET /api/demo/deps/manifest",
		presentation.NewDepsManifestHandler(depsModuleDir, logger))

	// Topic 32's architecture tracer: the SAME chain the posts API serves
	// — middleware → thin handler → use case → port → adapter — with
	// recorders at the seams, walked live on every request. The leaf
	// recorders sit INSIDE the switching facade (one per adapter), so the
	// timeline's data hop is the store that actually answered under the
	// scenario's current mode.
	archSink := demos.NewArchTraceSink(time.Now)
	archFake := demos.NewArchTraceRecorder(postsdata.NewInMemoryRepository(), demos.ArchDataInMemoryHop, archSink)
	archLive := demos.NewArchTraceRecorder(postsdata.NewPostgresRepository(pool), demos.ArchDataPostgresHop, archSink)
	archRepo := postsdata.NewSwitchingRepository(scenarioStore, archFake, archLive)
	archList := posts.NewListPostsUseCase(demos.NewArchTraceRecorder(archRepo, demos.ArchDomainHop, archSink))
	mux.Handle("GET /api/demo/arch/tracer",
		presentation.NewArchTracerHandler(scenarioStore, archList, archSink, logger))

	// Topic 33's observability wiring — the production edge the topic
	// teaches, all stdlib: /metrics serves the registry the middleware
	// below feeds, readiness pings the stack's REAL dependencies inside
	// bounded timeouts, and pprof rides a dedicated /internal/ path off
	// the public root (one prefix for ops to fence, the runtime's own
	// profiles inside).
	metricsRegistry := domobs.NewMetricsRegistry()
	httpRequests, httpDurations := presentation.RegisterHTTPMetrics(metricsRegistry)
	mux.Handle("GET /metrics", presentation.NewMetricsHandler(metricsRegistry, logger))
	mux.Handle("GET /api/ready", presentation.NewReadinessHandler([]domobs.DependencyCheck{
		{Name: "postgres", Pinger: obsdata.NewPgxPinger(pool), Timeout: time.Second},
		{Name: "redis", Pinger: obsdata.NewGoRedisPinger(redisClient), Timeout: time.Second},
	}, logger))
	mux.Handle("/internal/", presentation.NewInternalHandler())

	// The slog viewer demo replays the REAL posts handlers over a fresh
	// in-memory repository — scenario-independent by design, so the
	// replayed 200 and 404 hold under every mode.
	obsViewRepo := postsdata.NewInMemoryRepository()
	mux.Handle("GET /api/demo/obs/viewer",
		presentation.NewObsViewerHandler(
			posts.NewListPostsUseCase(obsViewRepo),
			posts.NewGetPostUseCase(obsViewRepo)))

	// The trace-context demo walks the same posts list with spanning
	// recorders at the seams — the leaf adapter inside, the domain's
	// port boundary outside — under whatever traceparent the caller
	// actually sent.
	obsSpans := domobs.NewSpanRecorder(time.Now)
	obsLeaf := demos.NewObsSpanningRepository(postsdata.NewInMemoryRepository(), "data", "InMemoryRepository", obsSpans)
	obsPort := demos.NewObsSpanningRepository(obsLeaf, "domain", "PostsRepository", obsSpans)
	mux.Handle("GET /api/demo/obs/trace",
		presentation.NewObsTraceHandler(posts.NewListPostsUseCase(obsPort), obsSpans, logger))

	// Topic 34's testing runner: a real, bounded `go test -json` run over
	// a curated set of this repo's own test packages, executed ONCE at
	// boot in the background (single-flight like the dependency topic's
	// govulncheck scan) so no request waits on a compiler; the endpoint
	// serves the cached parsed timeline with its wall-clock age plus the
	// suite's shape counted live from the source tree, and degrades to
	// the typed 503 when the toolchain or tree is absent.
	testingRunner := presentation.NewTestingRunnerHandler(depsModuleDir, logger)
	testingRunner.Start()
	mux.Handle("GET /api/demo/testing/runner", testingRunner)

	// Topic 35's stall: one real wedge per request — a bounded worker
	// set with one goroutine stuck on a channel receive — captured from
	// this process's own goroutine profile (the same profile the
	// /internal/ endpoints serve), parsed into the stuck stack with a
	// healthy contrast, and RELEASED before the response returns; every
	// internal wait is bounded, so the demo can never wedge its own
	// request.
	mux.Handle("GET /api/demo/debug/stall", presentation.NewDebugStallHandler(logger))

	// Topic 36's env inspector: THIS process's resolved config, derived
	// at request time from the live cfg — every variable with its
	// environment-vs-default attribution and its redacted-when-sensitive
	// value — beside the twelve-factor scorecard as data and the
	// ScenarioStore's CURRENT state as the live flag panel. The handler
	// receives the real cfg + store at wiring time; credentials never
	// reach the wire.
	mux.Handle("GET /api/demo/env/inspector",
		presentation.NewEnvInspectorHandler(cfg, scenarioStore, logger))

	// Topic 37's deployment stack viewer: the repo's REAL deployment
	// artifacts parsed at request time. The composition root resolves
	// the deploy/ directory when the repo sits beside the process (the
	// dev container); the distroless prod image ships no repo, so the
	// handler then serves the honest typed 503 instead of guessing.
	deployDir := ""
	if wd, wdErr := os.Getwd(); wdErr == nil {
		deployDir = demos.LocateDeployDir(wd)
	}
	mux.Handle("GET /api/demo/deploy/stack",
		presentation.NewDeployStackHandler(deployDir, logger))

	// Topic 38's performance bench: this repo's own curated benchmarks
	// (gzip levels × payload sizes, JSON buffered-marshal versus
	// streaming encode, the in-memory repository's List) executed ONCE
	// at boot in the background — single-flight like the testing
	// runner, bounded by a hard timeout that kills the whole process
	// group — beside a real `go build -gcflags=-m` over the
	// presentation package whose escape verdicts ride the payload
	// verbatim. Every number is parsed from the run with its command,
	// benchtime, count, and age; a missing toolchain or tree degrades
	// to the typed 503.
	perfBench := presentation.NewPerfBenchHandler(depsModuleDir, logger)
	perfBench.Start()
	mux.Handle("GET /api/demo/perf/bench", perfBench)

	// Topic 39's reliability breaker: a REAL process-lifetime circuit
	// breaker (closed/open/half-open, failure threshold, cooldown,
	// single-probe semantics) protecting a downstream call that honors
	// the shared scenario store's failure injection and respects
	// context deadlines. The breaker PERSISTS across requests, so
	// PATCHing /api/demo/scenario trips it under error injection,
	// converts stalls into deadline exceeded under latency, and lets
	// the half-open probe close it once disarmed after the cooldown —
	// with every served timing measured around the calls that just
	// happened. The console build is deterministic, so a failure here
	// is a programmer bug and fails boot like every other wiring.
	reliabilityConsole, err := demos.NewReliabilityConsole(demos.DefaultReliabilityConfig(), scenarioStore)
	if err != nil {
		return fmt.Errorf("reliability console: %w", err)
	}
	mux.Handle("GET /api/demo/reliability/breaker",
		presentation.NewReliabilityBreakerHandler(reliabilityConsole, logger))

	// The shared demo reset (guide infrastructure, not a curriculum
	// topic): one POST returns every stateful demo area to its
	// constructed state — the scenario store back to fake/none through
	// its own update API, and the process-lifetime breaker above back
	// to closed with zero counters and an empty history. The report is
	// honest per-area rows plus the areas deliberately never touched
	// (the store-backed substrates — destructive resets are out of
	// scope). Curl-only by the family's design: no client panel rides
	// it; the terminal companions own the affordance.
	fakeMode, noFailure := core.DataModeFake, core.FailureNone
	demoResetter, err := demos.NewDemoResetter(
		demos.DemoResetParticipant{
			Area:  "scenario",
			Label: "Scenario store",
			Action: func() (string, error) {
				state, err := scenarioStore.Update(core.ScenarioPatch{
					DataMode:      &fakeMode,
					InjectFailure: &noFailure,
				})
				if err != nil {
					return "", err
				}
				return fmt.Sprintf("scenario returned to dataMode=%s, injectFailure=%s — the constructed baseline",
					state.DataMode, state.InjectFailure), nil
			},
		},
		demos.DemoResetParticipant{
			Area:  "reliability",
			Label: "Process-lifetime breaker",
			Action: func() (string, error) {
				reliabilityConsole.Reset()
				snapshot := reliabilityConsole.BreakerObservation(time.Now())
				return fmt.Sprintf("breaker back to %s — counters zeroed, transition history cleared",
					snapshot.State), nil
			},
		},
	)
	if err != nil {
		return fmt.Errorf("demo resetter: %w", err)
	}
	mux.Handle("POST "+demos.DemoResetPath,
		presentation.NewDemoResetHandler(demoResetter, logger))

	// Topic 40's scaling round: REAL fleet machinery — three in-process
	// replicas behind a round-robin balancer with an affinity table,
	// plus the shared store standing for externalized state. Each
	// request rebuilds the fleet and walks one session through both
	// placements: replica-local memory breaks at the first cross-replica
	// hop and again at the first rolling-update swap; the shared store
	// survives everything. Every count on the wire (per-replica served
	// totals, swap before-counts, the accumulated counter) is measured
	// by the machinery itself, and the round reads no clock and no
	// random source — the endpoint is deterministic by construction.
	mux.Handle("GET /api/demo/scaling/stateless",
		presentation.NewScalingStatelessHandler(logger))

	// The posts resource — the api-design topic's real API surface, and
	// the substrate later topics reuse (authorization guards its ids,
	// webhooks fire on its writes, the docs topics document its routes).
	// One repository behind five use cases behind five thin handlers — and
	// since the databases topic that one repository is the SWITCHING
	// FACADE: fake mode answers from the seeded in-memory adapter, live
	// mode from the pgx adapter over the migrated + seeded PostgreSQL, and
	// PATCH /api/demo/scenario {"dataMode":…} flips every surface at once
	// — REST, GraphQL, and gRPC share these five use-case instances.
	postsRepo := postsdata.NewSwitchingRepository(
		scenarioStore,
		postsdata.NewInMemoryRepository(),
		postsdata.NewPostgresRepository(pool),
	)
	postsList := posts.NewListPostsUseCase(postsRepo)
	postsCreate := posts.NewCreatePostUseCase(postsRepo)
	postsGet := posts.NewGetPostUseCase(postsRepo)
	postsReplace := posts.NewReplacePostUseCase(postsRepo)
	postsDelete := posts.NewDeletePostUseCase(postsRepo)
	mux.Handle("GET /api/posts", presentation.NewPostsListHandler(postsList, logger))
	mux.Handle("POST /api/posts", presentation.NewPostsCreateHandler(postsCreate, logger))
	mux.Handle("GET /api/posts/{id}", presentation.NewPostDetailHandler(postsGet, logger))
	mux.Handle("PUT /api/posts/{id}", presentation.NewPostReplaceHandler(postsReplace, logger))
	mux.Handle("DELETE /api/posts/{id}", presentation.NewPostDeleteHandler(postsDelete, logger))

	// Topic 41's best-practices checklist: a REAL self-audit computed
	// once at boot (single-flight like the testing runner and perf
	// bench) — live probes through a private mux mounting the SAME
	// health, scenario, and posts handlers behind the SAME production
	// chain, bounded tool executions (gofmt -l, the snippet-ratchet
	// suite, a -race run), and file reads under the module and repo
	// directories. Every request serves the snapshot; a check that
	// cannot run becomes an honest fail/manual row, never a 500.
	bestChecklist := presentation.NewBestChecklistHandler(
		cfg, scenarioStore, postsGet, postsCreate, depsModuleDir, logger)
	bestChecklist.Start()
	mux.Handle("GET /api/demo/best/checklist", bestChecklist)

	// Topic 42's third-party hub: three vendor ports (mailer, payments,
	// object storage), each with a deterministic sandbox and a
	// credential-gated live skeleton, switched fake/live by the SHARED
	// scenario store — the same runtime swap the posts surface uses. The
	// handler is stateless: every request drives fresh sandboxes through
	// fixed scripts at a fixed exhibit clock (byte-identical bodies per
	// scenario state), resolves vendor credentials by env var NAME only,
	// and makes no vendor network call.
	mux.Handle("GET /api/demo/thirdparty/hub",
		presentation.NewThirdPartyHubHandler(scenarioStore, logger))

	// Topic 43's architecture capstone, first half: the saga — the REAL
	// compensation machinery from internal/domain/patterns driven through
	// three observed runs of a fixed order-fulfillment script (success,
	// injected failure with its idempotent re-compensation replay, and a
	// mid-saga cancellation whose undo pass survives the dead context).
	// Stateless by construction — no clock, no randomness — so two GETs
	// serve byte-identical bodies.
	mux.Handle("GET "+demos.ArchSagaPath, presentation.NewArchSagaHandler(logger))

	// Topic 43's architecture capstone, second half: the event store —
	// one inventory stream driven through the command path event
	// sourcing prescribes (rebuild by replay, decide against the
	// projection, append naming the version the decision was made
	// against), a typed optimistic-concurrency conflict with its
	// recovery, and deterministic rebuild proofs at a fixed exhibit
	// clock. Stateless like the saga half: byte-identical per GET.
	mux.Handle("GET "+demos.ArchEventstorePath, presentation.NewArchEventstoreHandler(logger))

	// GraphQL rides the SAME posts use-case instances as the REST routes —
	// one domain, two surfaces, one data-mode switch. The playground's
	// scratch store stays separate and fake by design (a func, because the
	// closure must construct the interface, not return it covariantly).
	mux.Handle(
		"GET /api/demo/api/graphql-playground",
		presentation.NewGraphQLPlaygroundHandler(func() posts.PostsRepository {
			return postsdata.NewInMemoryRepository()
		}),
	)
	mux.Handle(
		"POST /api/graphql",
		presentation.NewGraphQLHandler(postsList, postsGet, postsCreate, postsReplace, postsDelete, logger),
	)

	// The gRPC surface — the grpc topic's real server: the posts use cases
	// (minus delete, the proto's four methods) over HTTP/2 on a second
	// listener. A bind failure here fails startup like an HTTP one would.
	grpcListener, err := net.Listen("tcp", cfg.GRPCAddr)
	if err != nil {
		return fmt.Errorf("grpc listen %s: %w", cfg.GRPCAddr, err)
	}
	grpcServer := grpcapi.NewGRPCServer(postsList, postsGet, postsCreate, postsReplace)

	// The documented order: request-id outermost (recovery, logging, and
	// every error envelope share the id), access logging outside recovery
	// (recovered panics log as finished 500s), recovery converting panics
	// to the envelope, and the mux-error wrapper innermost so even the
	// router's own 404/405 replies ride the same envelope.
	handler := presentation.Chain(
		presentation.RequestID,
		presentation.RequestLogger(logger),
		presentation.Recover(logger),
		presentation.EnvelopeMuxErrors(logger),
	)(mux)

	// Topic 33's observability edge wraps the finished chain from the
	// OUTSIDE — no line inside the documented order above changes: the
	// trace-context middleware propagates W3C traceparent end to end
	// (accepted inbound, echoed outbound, malformed rejected to a fresh
	// root), and the metrics middleware counts every request onto the
	// bounded route-pattern labels /metrics serves.
	handler = presentation.TraceContext(logger)(handler)
	handler = presentation.MetricsMiddleware(httpRequests, httpDurations, mux)(handler)

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.Port),
		Handler: handler,
		// Bounded reads/writes defeat slow-client abuse while leaving ample
		// headroom for the guide's demos.
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	// SIGINT/SIGTERM cancel ctx, which triggers the graceful shutdown path.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// The ticker feed broadcasts until the boot context's cancellation —
	// the same signal path that drains the HTTP server below.
	go rtFeed.Run(ctx)

	// The jobs worker serves the demos queue until the same cancellation;
	// Start blocks until the broker answers, so a dead redis fails boot
	// honestly instead of stranding the console demo.
	if err := jobsWorker.Start(ctx); err != nil {
		return fmt.Errorf("jobs worker start: %w", err)
	}

	serveErr := make(chan error, 1)
	go func() {
		logger.Info("server listening", "addr", srv.Addr, "env", cfg.Env)
		serveErr <- srv.ListenAndServe()
	}()
	// Serve returns nil after GracefulStop, so only genuine failures land
	// on serveErr — the signal path's stop is not an error.
	go func() {
		logger.Info("grpc server listening", "addr", grpcListener.Addr().String())
		if err := grpcServer.Serve(grpcListener); err != nil {
			serveErr <- fmt.Errorf("grpc serve: %w", err)
		}
	}()

	select {
	case err := <-serveErr:
		// Shutdown is only ever called in the signal path below, so a value
		// here is a genuine startup/serve failure (e.g. port already bound).
		return fmt.Errorf("serve: %w", err)
	case <-ctx.Done():
	}

	// Restore default signal behavior: a second Ctrl-C force-exits instead
	// of waiting on the shutdown timeout again.
	stop()

	logger.Info("shutting down")
	// Both surfaces drain in-flight work: gRPC first (its RPCs are short),
	// then HTTP inside its bounded timeout.
	grpcServer.GracefulStop()
	logger.Info("grpc server stopped")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("graceful shutdown: %w", err)
	}
	logger.Info("server stopped")

	// The worker drains last: in-flight demo stories finish, then the
	// asynq server and its redis client stand down (idempotent).
	jobsWorker.Shutdown()
	logger.Info("jobs worker stopped")
	return nil
}
