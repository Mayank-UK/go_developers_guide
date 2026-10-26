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
	"syscall"
	"time"

	"guide/internal/content"
	"guide/internal/core"
	"guide/internal/data/db"
	postsdata "guide/internal/data/posts"
	"guide/internal/domain/posts"
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
	return nil
}
