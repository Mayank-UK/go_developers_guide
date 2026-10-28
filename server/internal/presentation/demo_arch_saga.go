// Demo API handler for topic 43 advanced-architecture — the saga
// exhibit (demo block id arch:saga). The handler is as thin as the
// guide's shape demands: it owns no policy and no narration. The
// builder in internal/domain/demos drives three fresh REAL saga runs
// from internal/domain/patterns through the fixed order-fulfillment
// script on every request (success, injected failure with its
// idempotent re-compensation replay, and mid-saga cancellation whose
// undo pass survives the dead context), so the endpoint is stateless —
// no process-lifetime console, nothing to start or drain, and two GETs
// serve byte-identical bodies because the script is fixed and no clock
// or randomness participates.
package presentation

import (
	"log/slog"
	"net/http"

	"guide/internal/domain/demos"
)

// Compile-time proof the handler satisfies http.Handler.
var _ http.Handler = (*ArchSagaHandler)(nil)

// ArchSagaHandler serves GET /api/demo/arch/saga, the
// advanced-architecture topic's saga demo payload: the fixed script's
// contract, the three observed runs with their ledgers and counted
// totals, the observed semantics, and the machinery's exported surface.
type ArchSagaHandler struct {
	logger *slog.Logger
}

// NewArchSagaHandler wires the handler over the process logger.
func NewArchSagaHandler(logger *slog.Logger) *ArchSagaHandler {
	return &ArchSagaHandler{logger: logger}
}

// ServeHTTP implements http.Handler: build the payload from fresh runs
// of the fixed script, write it.
func (h *ArchSagaHandler) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, demos.BuildArchSagaPayload())
}
