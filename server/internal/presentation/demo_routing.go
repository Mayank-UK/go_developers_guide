package presentation

import (
	"net/http"

	"guide/internal/domain/demos"
)

// Demo API handler for the routing topic — thin over the pure payload
// builder in internal/domain/demos (family invariant): no input to parse,
// no state to consult, nothing to map — delegate to the builder and write
// the JSON. The builder resolves a builder-chosen probe set against the
// guide's real route table carried as data, so the endpoint is a zero-input
// GET with a deterministic payload.

// RoutingExplorerHandler serves GET /api/demo/routing/explorer, the routing
// topic's matcher demo: the route table, the probe verdicts, the labeled
// exhibits, and the verified ServeMux semantics facts. The builder is pure,
// so the handler holds no dependencies: constructing one allocates nothing
// to wire.
type RoutingExplorerHandler struct{}

// NewRoutingExplorerHandler wires the routing-explorer demo endpoint.
func NewRoutingExplorerHandler() *RoutingExplorerHandler {
	return &RoutingExplorerHandler{}
}

// ServeHTTP implements http.Handler for the routing-explorer payload.
func (h *RoutingExplorerHandler) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, demos.BuildRoutingExplorerPayload())
}
