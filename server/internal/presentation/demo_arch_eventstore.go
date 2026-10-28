// Demo API handler for topic 43 advanced-architecture — the
// event-store exhibit (demo block id arch:eventstore). The handler is
// as thin as the guide's shape demands: it owns no policy and no
// narration. The builder in internal/domain/demos drives a fresh REAL
// event store from internal/domain/patterns through a fixed script on
// every request, so the endpoint is stateless — no process-lifetime
// console, nothing to start or drain, and two GETs serve byte-identical
// bodies because the exhibit runs at a fixed clock with fixed inputs.
package presentation

import (
	"log/slog"
	"net/http"

	"guide/internal/domain/demos"
)

// Compile-time proof the handler satisfies http.Handler.
var _ http.Handler = (*ArchEventstoreHandler)(nil)

// ArchEventstoreHandler serves GET /api/demo/arch/eventstore, the
// advanced-architecture topic's event-sourcing demo payload: the
// stream's three-act lifecycle (the command path, the optimistic
// conflict and its recovery, the deterministic rebuild proofs) and the
// store's API surface.
type ArchEventstoreHandler struct {
	logger *slog.Logger
}

// NewArchEventstoreHandler wires the handler over the process logger.
func NewArchEventstoreHandler(logger *slog.Logger) *ArchEventstoreHandler {
	return &ArchEventstoreHandler{logger: logger}
}

// ServeHTTP implements http.Handler: build the payload from a fresh
// run of the fixed exhibit, write it.
func (h *ArchEventstoreHandler) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, demos.BuildArchEventstorePayload())
}
