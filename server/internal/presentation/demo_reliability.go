// Demo API handler for topic 39 reliability — the breaker. The
// handler is deliberately the thinnest in the guide: it owns NO
// policy and NO narration. One process-lifetime console (built at the
// composition root over the shared ScenarioStore) carries the real
// circuit breaker; each request walks it — a bounded series of
// downstream calls whose failures the scenario panel injects — and a
// bounded retry-with-backoff series, then the pure builder in
// internal/domain/demos shapes the observed facts into the payload.
// The breaker state PERSISTS across requests, which is the demo's
// whole point: PATCH /api/demo/scenario to error and reload until the
// threshold trips; stay under latency and watch the per-call deadline
// convert stalls into deadline exceeded; set none and reload after
// the cooldown to watch the half-open probe close the circuit.
package presentation

import (
	"log/slog"
	"net/http"
	"time"

	"guide/internal/domain/demos"
)

// Compile-time proof the handler satisfies http.Handler.
var _ http.Handler = (*ReliabilityBreakerHandler)(nil)

// ReliabilityBreakerHandler serves GET /api/demo/reliability/breaker,
// the reliability topic's demo payload: the persistent breaker's
// snapshot (state, age, counters, transitions), this request's walk
// timeline with measured elapsed values, and the retry-with-backoff
// series — everything derived from the calls that just happened.
type ReliabilityBreakerHandler struct {
	logger  *slog.Logger
	console *demos.ReliabilityConsole
	now     func() time.Time
}

// NewReliabilityBreakerHandler wires the handler over the
// process-lifetime console. The console is shared state by design —
// the same injection main.go gives every request of this route.
func NewReliabilityBreakerHandler(console *demos.ReliabilityConsole, logger *slog.Logger) *ReliabilityBreakerHandler {
	return &ReliabilityBreakerHandler{logger: logger, console: console, now: time.Now}
}

// ServeHTTP implements http.Handler: walk the console under the
// current scenario, run the retry series beside it, snapshot the
// breaker AFTER both so the payload's state is the one the walk left
// behind, and shape the payload from those facts alone.
func (h *ReliabilityBreakerHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	walk := h.console.Walk(r.Context())
	retry := h.console.RetrySeries(r.Context())
	writeJSON(w, http.StatusOK, demos.BuildReliabilityBreakerPayload(demos.ReliabilityBreakerFacts{
		Scenario: h.console.ScenarioState(),
		Breaker:  h.console.BreakerObservation(h.now()),
		Walk:     walk,
		Retry:    retry,
	}))
}
