// Demo API handler for topic 18 sqlc-gorm: the ORM probe. Same shape as
// the schema explorer's handler — a live-PostgreSQL demo behind the
// domain's port, degrading honestly through the central error writer when
// the database is unreachable. The route is exactly
// GET /api/demo/db/orm-probe.
package presentation

import (
	"log/slog"
	"net/http"

	"guide/internal/core"
	"guide/internal/domain/demos"
)

// Compile-time proof the handler satisfies http.Handler.
var _ http.Handler = (*DBORMProbeHandler)(nil)

// DBORMProbeHandler serves GET /api/demo/db/orm-probe, the sqlc-gorm
// topic's measured exhibit. It depends on the domain's ORMProbeRunner
// port — the pgx/sqlc/GORM implementation in internal/data/ormprobe is
// wired at the composition root over the configured GDG_DATABASE_URL —
// and maps any probe failure to the 503 envelope: this endpoint's truth
// is "what the live database just did", so an unreachable database is an
// honest service_unavailable, never a stale answer.
type DBORMProbeHandler struct {
	runner demos.ORMProbeRunner
	logger *slog.Logger
}

// NewDBORMProbeHandler wires the orm-probe demo endpoint over its port.
func NewDBORMProbeHandler(runner demos.ORMProbeRunner, logger *slog.Logger) *DBORMProbeHandler {
	return &DBORMProbeHandler{runner: runner, logger: logger}
}

// ServeHTTP implements http.Handler for the orm-probe payload.
func (h *DBORMProbeHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	observation, err := h.runner.Probe(r.Context())
	if err != nil {
		core.WriteError(w, r, core.Wrapf(err, core.KindServiceUnavailable,
			"The ORM probe runs against the guide's live PostgreSQL, which is not reachable right now."), h.logger)
		return
	}
	writeJSON(w, http.StatusOK, demos.BuildORMProbePayload(observation))
}
