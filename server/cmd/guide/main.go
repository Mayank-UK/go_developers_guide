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
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"guide/internal/content"
	"guide/internal/core"
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
	cfg, err := core.LoadConfig(os.Getenv)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	logger := core.NewLogger(cfg)

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
	mux.Handle("GET /api/demo/conc/context", presentation.NewContextCancellationHandler())
	mux.Handle("GET /api/demo/conc/race", presentation.NewRaceDetectorHandler())

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
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("graceful shutdown: %w", err)
	}
	logger.Info("server stopped")
	return nil
}
