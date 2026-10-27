// Demo API handlers for topic 29 background-jobs — the family's
// thin-handler invariant with the optional-service twist the console
// carries: the story runs a REAL choreography against the guide's
// RUNNING Redis (enqueue, retry backoff, archive, replay, cron,
// teardown) through the hosted worker the composition root starts, so
// the handler holds the domain's runner port, mints a throwaway
// namespace per request, and degrades honestly when the store is
// unreachable or no worker drained the queues (a typed
// service_unavailable through the central error writer — never a
// panic, never a 500, never a stale answer).
package presentation

import (
	"errors"
	"log/slog"
	"net/http"

	"guide/internal/core"
	"guide/internal/domain/demos"
)

// Compile-time proof the handler satisfies http.Handler.
var _ http.Handler = (*JobsConsoleHandler)(nil)

// JobsConsoleHandler serves GET /api/demo/jobs/console, the
// background-jobs console demo. It depends on the domain's
// JobsConsoleRunner port — the asynq-backed story runner over the
// guide's Redis is wired at the composition root, which also starts the
// ServerRunner worker draining the demo queues. Redis is an OPTIONAL
// service in this stack, so its absence is an honest
// service_unavailable; a reachable Redis whose queues never drained
// (the hosted worker not running) is the same honest 503 with its own
// message naming exactly that.
type JobsConsoleHandler struct {
	runner demos.JobsConsoleRunner
	logger *slog.Logger
}

// NewJobsConsoleHandler wires the console demo endpoint over its
// runner port.
func NewJobsConsoleHandler(runner demos.JobsConsoleRunner, logger *slog.Logger) *JobsConsoleHandler {
	return &JobsConsoleHandler{runner: runner, logger: logger}
}

// ServeHTTP implements http.Handler for the console story payload:
// mint a throwaway namespace (concurrent requests never share ledgers,
// counters, or task ids), walk the choreography, shape the run, write
// the JSON.
func (h *JobsConsoleHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	namespace, err := demos.NewJobsDemoNamespace()
	if err != nil {
		// Minting needs crypto/rand; a failing entropy source is a genuine
		// internal error — not the client's fault and not the store's.
		core.WriteError(w, r, core.Wrap(err, core.KindInternal,
			"The jobs console demo could not mint a throwaway namespace."), h.logger)
		return
	}
	run, err := h.runner.RunConsoleStory(r.Context(), namespace)
	if err != nil {
		if errors.Is(err, demos.ErrJobsWorkerNotDraining) {
			core.WriteError(w, r, core.Wrapf(err, core.KindServiceUnavailable,
				"The jobs console demo enqueued its tasks, but no worker drained the demo queues "+
					"inside the story window — the hosted worker is not running."), h.logger)
			return
		}
		core.WriteError(w, r, core.Wrapf(err, core.KindServiceUnavailable,
			"The jobs console demo runs a real choreography against the guide's Redis, "+
				"which is not reachable right now."), h.logger)
		return
	}
	writeJSON(w, http.StatusOK, demos.BuildJobsConsolePayload(run))
}
