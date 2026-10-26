// Demo API handlers for topic 16 sql-schema — the family's thin-handler
// invariant with one new twist: the schema-explorer demo reads a LIVE
// PostgreSQL, so its handler holds the domain's introspector port and
// degrades honestly when the database is unreachable (a typed
// service_unavailable through the central error writer — never a panic,
// never a 500). The uuid lab's handler is the classic shape: no inputs, a
// builder, write the JSON.
package presentation

import (
	"log/slog"
	"net/http"

	"guide/internal/core"
	"guide/internal/domain/demos"
)

// Compile-time proofs both handlers satisfy http.Handler.
var (
	_ http.Handler = (*DBSchemaExplorerHandler)(nil)
	_ http.Handler = (*DBUUIDHandler)(nil)
)

// DBSchemaExplorerHandler serves GET /api/demo/db/schema-explorer, the
// sql-schema topic's introspection demo. It depends on the domain's
// SchemaIntrospector port — the pgx-backed adapter is wired at the
// composition root — and maps any introspection failure to the 503
// envelope: this endpoint's truth is "what the live database says", so an
// unreachable database is an honest service_unavailable, never a stale
// answer.
type DBSchemaExplorerHandler struct {
	introspector demos.SchemaIntrospector
	logger       *slog.Logger
}

// NewDBSchemaExplorerHandler wires the schema-explorer demo endpoint over
// its introspector port.
func NewDBSchemaExplorerHandler(introspector demos.SchemaIntrospector, logger *slog.Logger) *DBSchemaExplorerHandler {
	return &DBSchemaExplorerHandler{introspector: introspector, logger: logger}
}

// ServeHTTP implements http.Handler for the schema-explorer payload.
func (h *DBSchemaExplorerHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	snapshot, err := h.introspector.Introspect(r.Context())
	if err != nil {
		core.WriteError(w, r, core.Wrapf(err, core.KindServiceUnavailable,
			"The schema explorer reads the guide's live PostgreSQL, which is not reachable right now."), h.logger)
		return
	}
	writeJSON(w, http.StatusOK, demos.BuildSchemaExplorerPayload(snapshot))
}

// DBUUIDHandler serves GET /api/demo/db/uuid, the ids section's UUIDv7
// lab. The handler owns the production generator (crypto/rand + wall
// clock) and mints a fresh batch per request — the payload's counts and
// properties are stable, its uuid values are live randomness.
type DBUUIDHandler struct {
	generator *demos.UUIDv7Generator
	logger    *slog.Logger
}

// NewDBUUIDHandler wires the uuid lab demo endpoint.
func NewDBUUIDHandler(logger *slog.Logger) *DBUUIDHandler {
	return &DBUUIDHandler{generator: demos.NewUUIDv7Generator(), logger: logger}
}

// ServeHTTP implements http.Handler for the uuid lab payload.
func (h *DBUUIDHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	payload, err := demos.BuildUUIDLabPayload(h.generator)
	if err != nil {
		// Minting needs crypto/rand and a moving clock; both failing is a
		// genuine internal error — the entropy source being down is not
		// the client's fault and not the database's.
		core.WriteError(w, r, core.Wrapf(err, core.KindInternal, "The uuid lab could not mint fresh values."), h.logger)
		return
	}
	writeJSON(w, http.StatusOK, payload)
}
