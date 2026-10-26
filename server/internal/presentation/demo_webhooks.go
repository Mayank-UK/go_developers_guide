// Demo API handler for topic 14 webhooks — same family invariant as
// demo.go: thin over the pure payload builder in internal/domain/demos.
// The simulator's whole input set is fixed exhibit material (a signed
// delivery, two endpoint scripts, one receiver clock), so a parameterless
// GET carries the entire lesson: real HMAC verdicts, a computed backoff
// ladder, and the receiving side's fast 2xx.
package presentation

import (
	"net/http"

	"guide/internal/domain/demos"
)

// Compile-time proof that the webhook simulator's handler satisfies
// http.Handler — the interface the route registration requires.
var _ http.Handler = (*WebhookSimulatorHandler)(nil)

// WebhookSimulatorHandler serves GET /api/demo/api/webhook-simulator, the
// webhooks topic's inline demo. The builder is a pure function over fixed
// exhibit facts — no wall clock, no sleeping, no goroutines — so the
// handler holds no dependencies: constructing one allocates nothing to
// wire.
type WebhookSimulatorHandler struct{}

// NewWebhookSimulatorHandler wires the webhook-simulator demo endpoint.
func NewWebhookSimulatorHandler() *WebhookSimulatorHandler {
	return &WebhookSimulatorHandler{}
}

// ServeHTTP implements http.Handler for the webhook-simulator payload.
func (h *WebhookSimulatorHandler) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, demos.BuildWebhookSimulatorPayload())
}
