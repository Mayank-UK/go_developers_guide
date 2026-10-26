// Demo API handler for topic 15 grpc — same family invariant as demo.go:
// thin over the payload builder in internal/domain/demos. The console's
// whole exhibit set is fixed (four stream kinds, one miss, one deadline),
// so a parameterless GET carries the entire lesson: the builder builds a
// real in-process gRPC stack, runs the round trips, and this handler has
// nothing to do but map the reports onto the wire.
package presentation

import (
	"net/http"

	"guide/internal/domain/demos"
)

// Compile-time proof that the grpc-console handler satisfies http.Handler
// — the interface the route registration requires.
var _ http.Handler = (*GRPCConsoleHandler)(nil)

// GRPCConsoleHandler serves GET /api/demo/api/grpc-console, the grpc
// topic's inline demo payload. The builder constructs its own in-process
// server per call (bufconn, scratch repository), so the handler holds no
// dependencies and no state: constructing one allocates nothing to wire.
type GRPCConsoleHandler struct{}

// NewGRPCConsoleHandler wires the grpc-console demo endpoint.
func NewGRPCConsoleHandler() *GRPCConsoleHandler {
	return &GRPCConsoleHandler{}
}

// ServeHTTP implements http.Handler for the grpc-console payload.
func (h *GRPCConsoleHandler) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, demos.BuildGRPCConsolePayload())
}
