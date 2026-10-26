package presentation

import (
	"net/http"

	"guide/internal/domain/demos"
)

// Demo API handlers — thin over the pure payload builders in
// internal/domain/demos (family invariant): no input to parse, no state to
// consult, nothing to map — delegate to the builder and write the JSON.

// RuntimeTypesHandler serves GET /api/demo/go/runtime-types, the
// go-language topic's inline demo. The builder is a pure function over
// real guide objects, so the handler holds no dependencies: constructing
// one allocates nothing to wire.
type RuntimeTypesHandler struct{}

// NewRuntimeTypesHandler wires the runtime-types demo endpoint.
func NewRuntimeTypesHandler() *RuntimeTypesHandler {
	return &RuntimeTypesHandler{}
}

// ServeHTTP implements http.Handler for the runtime-types demo payload.
func (h *RuntimeTypesHandler) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, demos.BuildRuntimeTypesPayload())
}
