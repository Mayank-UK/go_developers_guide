// Demo API handlers for topic 17 databases-sql — six routes under
// /api/demo/db/, each the family's thin-handler shape over one domain
// port: call the runner, map any failure to the honest typed
// service_unavailable envelope (every one of these demos' truth is "what
// the live database just did", so an unreachable database is a 503,
// never a stale answer and never a panic), and write the builder's
// payload. The pgx-backed runner is wired at the composition root.
package presentation

import (
	"log/slog"
	"net/http"

	"guide/internal/core"
	"guide/internal/domain/demos"
)

// Compile-time proofs all six handlers satisfy http.Handler.
var (
	_ http.Handler = (*DBSQLSafetyHandler)(nil)
	_ http.Handler = (*DBMigrationsHandler)(nil)
	_ http.Handler = (*DBSQLLabHandler)(nil)
	_ http.Handler = (*DBSearchHandler)(nil)
	_ http.Handler = (*DBRaceHandler)(nil)
	_ http.Handler = (*DBAuditHandler)(nil)
)

// dbDemoUnavailable is the one failure mapping all six handlers share.
func dbDemoUnavailable(w http.ResponseWriter, r *http.Request, err error, message string, logger *slog.Logger) {
	core.WriteError(w, r, core.Wrap(err, core.KindServiceUnavailable, message), logger)
}

// DBSQLSafetyHandler serves GET /api/demo/db/sql-safety: the
// parameterized-versus-concatenated exhibit, run live against posts.
type DBSQLSafetyHandler struct {
	runner demos.SQLSafetyRunner
	logger *slog.Logger
}

// NewDBSQLSafetyHandler wires the sql-safety demo endpoint.
func NewDBSQLSafetyHandler(runner demos.SQLSafetyRunner, logger *slog.Logger) *DBSQLSafetyHandler {
	return &DBSQLSafetyHandler{runner: runner, logger: logger}
}

// ServeHTTP implements http.Handler for the sql-safety payload.
func (h *DBSQLSafetyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	observation, err := h.runner.ObserveSQLSafety(r.Context(), demos.SQLSafetyAttackInputs())
	if err != nil {
		dbDemoUnavailable(w, r, err,
			"The sql-safety demo runs its parameterized lookups against the guide's live PostgreSQL, which is not reachable right now.", h.logger)
		return
	}
	writeJSON(w, http.StatusOK, demos.BuildSQLSafetyPayload(observation))
}

// DBMigrationsHandler serves GET /api/demo/db/migrations: goose's ledger
// beside the live savepoint/rollback transaction story.
type DBMigrationsHandler struct {
	runner demos.SQLTxRunner
	logger *slog.Logger
}

// NewDBMigrationsHandler wires the migrations/transactions demo endpoint.
func NewDBMigrationsHandler(runner demos.SQLTxRunner, logger *slog.Logger) *DBMigrationsHandler {
	return &DBMigrationsHandler{runner: runner, logger: logger}
}

// ServeHTTP implements http.Handler for the transactions story payload.
func (h *DBMigrationsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	observation, err := h.runner.RunTxStory(r.Context())
	if err != nil {
		dbDemoUnavailable(w, r, err,
			"The transactions demo runs its savepoint story inside the guide's live PostgreSQL, which is not reachable right now.", h.logger)
		return
	}
	writeJSON(w, http.StatusOK, demos.BuildSQLMigrationsPayload(observation))
}

// DBSQLLabHandler serves GET /api/demo/db/sql-lab: the CTE and window
// function exhibits over the seeded posts.
type DBSQLLabHandler struct {
	runner demos.SQLLabRunner
	logger *slog.Logger
}

// NewDBSQLLabHandler wires the sql-lab demo endpoint.
func NewDBSQLLabHandler(runner demos.SQLLabRunner, logger *slog.Logger) *DBSQLLabHandler {
	return &DBSQLLabHandler{runner: runner, logger: logger}
}

// ServeHTTP implements http.Handler for the sql-lab payload.
func (h *DBSQLLabHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	observation, err := h.runner.RunSQLLab(r.Context())
	if err != nil {
		dbDemoUnavailable(w, r, err,
			"The sql-lab demo runs its CTE and window exhibits against the guide's live PostgreSQL, which is not reachable right now.", h.logger)
		return
	}
	writeJSON(w, http.StatusOK, demos.BuildSQLLabPayload(observation))
}

// DBSearchHandler serves GET /api/demo/db/search: the tsvector/tsquery
// match with its ranked rows, plan, and GIN index.
type DBSearchHandler struct {
	runner demos.SQLSearchRunner
	logger *slog.Logger
}

// NewDBSearchHandler wires the search demo endpoint.
func NewDBSearchHandler(runner demos.SQLSearchRunner, logger *slog.Logger) *DBSearchHandler {
	return &DBSearchHandler{runner: runner, logger: logger}
}

// ServeHTTP implements http.Handler for the search payload.
func (h *DBSearchHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	observation, err := h.runner.RunSearch(r.Context())
	if err != nil {
		dbDemoUnavailable(w, r, err,
			"The search demo runs its tsvector query against the guide's live PostgreSQL, which is not reachable right now.", h.logger)
		return
	}
	writeJSON(w, http.StatusOK, demos.BuildSQLSearchPayload(observation))
}

// DBRaceHandler serves GET /api/demo/db/race: the orchestrated
// two-transaction isolation story.
type DBRaceHandler struct {
	runner demos.SQLRaceRunner
	logger *slog.Logger
}

// NewDBRaceHandler wires the race demo endpoint.
func NewDBRaceHandler(runner demos.SQLRaceRunner, logger *slog.Logger) *DBRaceHandler {
	return &DBRaceHandler{runner: runner, logger: logger}
}

// ServeHTTP implements http.Handler for the race payload.
func (h *DBRaceHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	observation, err := h.runner.RunRace(r.Context())
	if err != nil {
		dbDemoUnavailable(w, r, err,
			"The isolation demo choreographs two transactions on the guide's live PostgreSQL, which is not reachable right now.", h.logger)
		return
	}
	writeJSON(w, http.StatusOK, demos.BuildSQLRacePayload(observation))
}

// DBAuditHandler serves GET /api/demo/db/audit: the audit rows riding
// the changing transaction — commit and rollback, proven together.
type DBAuditHandler struct {
	runner demos.SQLAuditRunner
	logger *slog.Logger
}

// NewDBAuditHandler wires the audit demo endpoint.
func NewDBAuditHandler(runner demos.SQLAuditRunner, logger *slog.Logger) *DBAuditHandler {
	return &DBAuditHandler{runner: runner, logger: logger}
}

// ServeHTTP implements http.Handler for the audit payload.
func (h *DBAuditHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	observation, err := h.runner.RunAuditStory(r.Context())
	if err != nil {
		dbDemoUnavailable(w, r, err,
			"The audit demo runs its commit/rollback story inside the guide's live PostgreSQL, which is not reachable right now.", h.logger)
		return
	}
	writeJSON(w, http.StatusOK, demos.BuildAuditPayload(observation))
}
