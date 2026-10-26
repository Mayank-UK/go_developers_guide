// Demo API handlers for topic 10 validation — same family invariant as
// demo.go: thin over the pure payload builder in internal/domain/demos,
// nothing to parse, nothing to map. The arena's whole input set is
// builder-chosen static exhibit material, so a GET with no parameters
// carries the entire lesson; the 422 half of the story stays live on the
// scenario PATCH route the topic's terminal companion exercises.
package presentation

import (
	"net/http"

	"guide/internal/domain/demos"
)

// Compile-time proof that the validation arena's handler satisfies
// http.Handler — the interface the route registration below it requires.
var _ http.Handler = (*ValidationArenaHandler)(nil)

// ValidationArenaHandler serves GET /api/demo/validation/arena, the
// validation topic's inline demo. The builder is a pure function whose
// outcomes come from the real decode discipline, the real hand-rolled
// rules, and the repo's own error-envelope writer, so the handler holds
// no dependencies: constructing one allocates nothing to wire.
type ValidationArenaHandler struct{}

// NewValidationArenaHandler wires the validation-arena demo endpoint.
func NewValidationArenaHandler() *ValidationArenaHandler {
	return &ValidationArenaHandler{}
}

// ServeHTTP implements http.Handler for the validation-arena payload.
func (h *ValidationArenaHandler) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, demos.BuildValidationArenaPayload())
}
