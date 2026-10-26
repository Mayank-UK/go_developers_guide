// Demo API handler for topic 20 redis — the family's thin-handler
// invariant with the optional-service twist: the playground runs REAL
// commands against the guide's RUNNING Redis, so the handler holds the
// domain's runner port, mints a throwaway key namespace per request, and
// degrades honestly when the store is unreachable (a typed
// service_unavailable through the central error writer — never a panic,
// never a 500, never a stale answer).
package presentation

import (
	"log/slog"
	"net/http"

	"guide/internal/core"
	"guide/internal/domain/demos"
)

// Compile-time proof the handler satisfies http.Handler.
var _ http.Handler = (*RedisPlaygroundHandler)(nil)

// RedisPlaygroundHandler serves GET /api/demo/redis/playground, the
// redis topic's single demo. It depends on the domain's
// RedisPlaygroundRunner port — the go-redis-backed adapter is wired at
// the composition root from the configured address — and maps any store
// failure to the 503 envelope: Redis is an OPTIONAL service in this
// stack, so its absence is an honest service_unavailable, not an
// internal error.
type RedisPlaygroundHandler struct {
	runner demos.RedisPlaygroundRunner
	logger *slog.Logger
}

// NewRedisPlaygroundHandler wires the playground demo endpoint over its
// runner port.
func NewRedisPlaygroundHandler(runner demos.RedisPlaygroundRunner, logger *slog.Logger) *RedisPlaygroundHandler {
	return &RedisPlaygroundHandler{runner: runner, logger: logger}
}

// ServeHTTP implements http.Handler for the playground payload: mint a
// throwaway namespace (concurrent requests never share keys), walk the
// beats, shape the run, write the JSON.
func (h *RedisPlaygroundHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	namespace, err := demos.NewRedisDemoNamespace()
	if err != nil {
		// Minting needs crypto/rand; a failing entropy source is a genuine
		// internal error — not the client's fault and not the store's.
		core.WriteError(w, r, core.Wrap(err, core.KindInternal,
			"The redis playground could not mint a throwaway key namespace."), h.logger)
		return
	}
	run, err := h.runner.Run(r.Context(), namespace)
	if err != nil {
		core.WriteError(w, r, core.Wrapf(err, core.KindServiceUnavailable,
			"The redis playground runs real commands against the guide's Redis, which is not reachable right now."), h.logger)
		return
	}
	writeJSON(w, http.StatusOK, demos.BuildRedisPlaygroundPayload(run))
}
