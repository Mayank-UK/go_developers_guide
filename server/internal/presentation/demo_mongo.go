// Demo API handler for topic 19 mongodb — the family's thin-handler
// shape over the domain's runner port: run the console, map any failure
// to the honest typed service_unavailable envelope (MongoDB is an
// OPTIONAL service — an unreachable store is a 503, never a panic,
// never a crashed process), write the builder's payload. The
// mongo-driver-backed runner is wired at the composition root, over the
// real store when MongoDB is up and over the unavailable fallback when
// it is not.
package presentation

import (
	"log/slog"
	"net/http"

	"guide/internal/domain/demos"
)

// Compile-time proof the handler satisfies http.Handler.
var _ http.Handler = (*DBMongoConsoleHandler)(nil)

// DBMongoConsoleHandler serves GET /api/demo/db/mongo-console, the
// mongodb topic's scripted console: the idempotent seed, the
// application-layer refusal, the query and projection beats, the
// explain pair, the TTL spec, and the BSON type survey.
type DBMongoConsoleHandler struct {
	runner demos.MongoConsoleRunner
	logger *slog.Logger
}

// NewDBMongoConsoleHandler wires the mongo-console demo endpoint over
// its runner port.
func NewDBMongoConsoleHandler(runner demos.MongoConsoleRunner, logger *slog.Logger) *DBMongoConsoleHandler {
	return &DBMongoConsoleHandler{runner: runner, logger: logger}
}

// ServeHTTP implements http.Handler for the mongo-console payload.
func (h *DBMongoConsoleHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	observation, err := h.runner.RunConsole(r.Context())
	if err != nil {
		dbDemoUnavailable(w, r, err,
			"The mongo console runs its beats against the guide's live MongoDB, which is not reachable right now.", h.logger)
		return
	}
	writeJSON(w, http.StatusOK, demos.BuildMongoConsolePayload(observation))
}
