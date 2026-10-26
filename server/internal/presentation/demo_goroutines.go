package presentation

import (
	"net/http"

	"guide/internal/domain/demos"
)

// Demo API handlers for the goroutines-and-channels topic — thin over the
// pure payload builders in internal/domain/demos (family invariant): no
// input to parse, no state to consult, nothing to map — delegate to the
// builder and write the JSON. Both builders are parameterless and own
// their contexts internally, so the request's context is deliberately NOT
// threaded in: a demo endpoint's choreography must not be coupled to any
// one request's lifetime.

// ContextCancellationHandler serves GET /api/demo/conc/context, the
// goroutines-and-channels topic's cancellation-propagation demo. The
// builder runs a real parent-cancel choreography (work phase, tree
// propagation, deadline beat) deterministically, so the handler holds no
// dependencies: constructing one allocates nothing to wire.
type ContextCancellationHandler struct{}

// NewContextCancellationHandler wires the context-cancellation demo endpoint.
func NewContextCancellationHandler() *ContextCancellationHandler {
	return &ContextCancellationHandler{}
}

// ServeHTTP implements http.Handler for the context-cancellation payload.
func (h *ContextCancellationHandler) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, demos.BuildContextCancellationPayload())
}

// RaceDetectorHandler serves GET /api/demo/conc/race, the topic's
// race-detector demo: the two synchronized totals run live; the racy
// exhibit is pinned data that never executes.
type RaceDetectorHandler struct{}

// NewRaceDetectorHandler wires the race-detector demo endpoint.
func NewRaceDetectorHandler() *RaceDetectorHandler {
	return &RaceDetectorHandler{}
}

// ServeHTTP implements http.Handler for the race-detector payload.
func (h *RaceDetectorHandler) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, demos.BuildRaceDetectorPayload())
}
