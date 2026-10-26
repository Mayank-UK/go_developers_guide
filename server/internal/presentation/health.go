// Package presentation is the HTTP edge: middleware (request id, access
// logging, panic recovery, envelope-shaped routing errors) and thin
// handlers that translate between the HTTP wire and the rest of the
// server.
package presentation

import (
	"encoding/json"
	"net/http"
)

// healthResponse is the family health wire shape — field names and values
// mirror the fastapi guide's GET /api/health exactly so family clients and
// probes stay interchangeable across guides.
type healthResponse struct {
	Status string `json:"status"`
	Env    string `json:"env"`
}

// HealthHandler serves GET /api/health, the family liveness probe. It is
// deliberately dependency-free (no DB/redis ping): it answers "is the
// process up", not "are its dependencies up" — readiness splits arrive
// with the observability topic.
type HealthHandler struct {
	env string
}

// NewHealthHandler returns a handler that reports the environment the
// server booted in.
func NewHealthHandler(env string) *HealthHandler {
	return &HealthHandler{env: env}
}

// ServeHTTP implements http.Handler for the health probe.
func (h *HealthHandler) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	// An Encode failure means the peer vanished mid-write; there is no
	// recovery action, and the error already surfaces to the client.
	_ = json.NewEncoder(w).Encode(healthResponse{Status: "ok", Env: h.env})
}
