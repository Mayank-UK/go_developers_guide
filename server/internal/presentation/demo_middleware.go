package presentation

import (
	"net/http"

	"guide/internal/domain/demos"
)

// Demo API handler for the middleware topic — thin over the pure payload
// builder in internal/domain/demos (family invariant): no input to parse,
// no state to consult, nothing to map — delegate to the builder and write
// the JSON. The builder runs the exhibit chain in-process over
// httptest.NewRecorder requests and owns all of its own timing internally,
// so the request's context is deliberately NOT threaded in: a demo
// endpoint's choreography must not be coupled to any one request's
// lifetime.

// MiddlewarePipelineHandler serves GET /api/demo/middleware/pipeline, the
// middleware topic's inline demo: the exhibit chain's before/after trace,
// the request id's propagation across layers, the recovered-panic beat,
// and the http.TimeoutHandler beat. The handler holds no dependencies:
// constructing one allocates nothing to wire.
type MiddlewarePipelineHandler struct{}

// NewMiddlewarePipelineHandler wires the middleware-pipeline demo endpoint.
func NewMiddlewarePipelineHandler() *MiddlewarePipelineHandler {
	return &MiddlewarePipelineHandler{}
}

// ServeHTTP implements http.Handler for the middleware-pipeline payload.
func (h *MiddlewarePipelineHandler) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, demos.BuildMiddlewarePipelinePayload())
}
