// Demo API handlers for topic 05 interfaces-and-composition — same family
// invariant as demo.go: thin over the pure payload builders in
// internal/domain/demos, nothing to parse, nothing to map.
package presentation

import (
	"net/http"

	"guide/internal/domain/demos"
)

// Compile-time proof that the repo's own handlers satisfy http.Handler —
// no type in this package ever declares "implements". This block is the
// matrix demo's http.Handler column held at build time: the interface
// matrix endpoint below serves the same fact computed live.
var (
	_ http.Handler = (*HealthHandler)(nil)
	_ http.Handler = (*ContentListHandler)(nil)
	_ http.Handler = (*ContentTopicHandler)(nil)
	_ http.Handler = (*ScenarioGetHandler)(nil)
	_ http.Handler = (*ScenarioPatchHandler)(nil)
	_ http.Handler = (*RuntimeTypesHandler)(nil)
	_ http.Handler = (*ConcurrencyHandler)(nil)
	_ http.Handler = (*InterfaceMatrixHandler)(nil)
)

// InterfaceMatrixHandler serves GET /api/demo/iface/matrix, the
// interfaces-and-composition topic's inline demo. The builder computes
// every satisfaction verdict from real method sets, so the handler holds
// no dependencies: constructing one allocates nothing to wire.
type InterfaceMatrixHandler struct{}

// NewInterfaceMatrixHandler wires the interface-matrix demo endpoint.
func NewInterfaceMatrixHandler() *InterfaceMatrixHandler {
	return &InterfaceMatrixHandler{}
}

// ServeHTTP implements http.Handler for the interface-matrix payload.
func (h *InterfaceMatrixHandler) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, demos.BuildInterfaceMatrixPayload())
}
