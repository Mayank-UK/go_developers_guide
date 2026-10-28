// The shared demo reset handler — POST /api/demo/reset, the terminal
// companion's one-call affordance (guide infrastructure, not a
// curriculum topic; no client panel rides it, by the family's design).
// Thin as always: no input to parse, ONE use-case call, the report
// onto the wire — the handler owns no policy and no narration. The
// route is registered POST-only on the mux, so the router itself
// answers every other method with the truthful 405 + Allow envelope.
package presentation

import (
	"log/slog"
	"net/http"

	"guide/internal/core"
	"guide/internal/domain/demos"
)

// Compile-time proof the handler satisfies http.Handler.
var _ http.Handler = (*DemoResetHandler)(nil)

// DemoResetHandler serves POST /api/demo/reset: every registered
// stateful demo area returned to its constructed state, answered with
// the honest per-area report — what really reset, what degraded and
// why, and the areas deliberately never touched.
type DemoResetHandler struct {
	resetter *demos.DemoResetter
	logger   *slog.Logger
}

// NewDemoResetHandler wires the reset use case the composition root
// assembled over the stateful demo areas.
func NewDemoResetHandler(resetter *demos.DemoResetter, logger *slog.Logger) *DemoResetHandler {
	return &DemoResetHandler{resetter: resetter, logger: logger}
}

// ServeHTTP implements http.Handler: run the sweep, then map the
// report — or a propagated non-taxonomy bug, as the 500 envelope —
// onto the wire.
func (h *DemoResetHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	report, err := h.resetter.Run()
	if err != nil {
		core.WriteError(w, r, err, h.logger)
		return
	}
	if h.logger != nil {
		degraded := 0
		for _, row := range report.Reset {
			if row.Outcome == demos.DemoOutcomeDegraded {
				degraded++
			}
		}
		h.logger.Info("demo reset completed",
			"areas", len(report.Reset),
			"degraded", degraded)
	}
	writeJSON(w, http.StatusOK, report)
}
