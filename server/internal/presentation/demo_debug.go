// Demo API handler for topic 35 debugging — the stall. Thin by the
// family rule: the handler owns nothing but the request's own stall
// run and the JSON write; the choreography (wedge, capture, parse,
// release) lives in the domain's builder, and every wait the run makes
// is bounded there — a wedged demo can never wedge the REQUEST. The
// run is also repeatable: each GET stages a fresh worker set, so
// reloading the panel is the "capture again" gesture.
package presentation

import (
	"log/slog"
	"net/http"

	"guide/internal/core"
	"guide/internal/domain/demos"
)

// Compile-time proof the handler satisfies http.Handler.
var _ http.Handler = (*DebugStallHandler)(nil)

// DebugStallHandler serves GET /api/demo/debug/stall, the debugging
// topic's demo payload: one real stall, captured from this process's
// own goroutine profile and released before the response returns.
type DebugStallHandler struct {
	logger *slog.Logger
	// runStall performs one full wedge-capture-release cycle; a field
	// so tests inject deterministic stalls without touching the
	// runtime.
	runStall func() demos.DebugStallRun
}

// NewDebugStallHandler wires the stall over the production runner.
func NewDebugStallHandler(logger *slog.Logger) *DebugStallHandler {
	handler := &DebugStallHandler{logger: logger}
	handler.runStall = func() demos.DebugStallRun {
		return demos.RunDebugStall(nil, "")
	}
	return handler
}

// ServeHTTP implements http.Handler: one bounded stall run, shaped and
// served. A run that somehow failed to find its own wedge on the
// profile is a broken demo, not a degraded feature — the typed 500
// says so instead of serving a fabricated stack.
func (h *DebugStallHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	run := h.runStall()
	if run.Stalled == nil {
		h.logger.Error("debug stall run lost its own wedge",
			"request_id", core.RequestIDFromContext(r.Context()),
			"goroutines_seen", run.GoroutineSeen,
			"capture_tries", run.CaptureTries,
		)
		core.WriteError(w, r, core.Internal(
			"The stall demo could not find its wedged goroutine on the "+
				"process's own profile; no stack was invented to cover for it."),
			h.logger)
		return
	}
	writeJSON(w, http.StatusOK, demos.BuildDebugStallPayload(run))
}
