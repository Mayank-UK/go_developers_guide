// Demo API handler for topic 40 scaling — the statelessness round.
// The handler is deliberately thin: it owns no policy and no
// narration. Each request builds a fresh fleet (three in-process
// replicas behind a real round-robin balancer with an affinity table,
// plus the shared store standing in for externalized state), runs the
// deterministic round through it, and hands the observed facts to the
// pure builder in internal/domain/demos, which shapes the payload.
// Nothing on the wire is invented: per-replica served counts, session
// hit/miss outcomes, swap before-counts, and the accumulated shared
// counter are all read off the machinery that produced them.
package presentation

import (
	"log/slog"
	"net/http"

	"guide/internal/core"
	"guide/internal/domain/demos"
)

// Compile-time proof the handler satisfies http.Handler.
var _ http.Handler = (*ScalingStatelessHandler)(nil)

// ScalingStatelessHandler serves GET /api/demo/scaling/stateless, the
// scaling topic's demo payload: the measured distribution, the
// thirteen-step session round (local placement breaking on hop and
// swap, sticky routing's lease, the shared placement surviving
// everything), the counter beat, and the verdicts derived from those
// measured numbers alone.
type ScalingStatelessHandler struct {
	logger *slog.Logger
}

// NewScalingStatelessHandler wires the handler. It holds no state of
// its own — the round is rebuilt per request, which is what makes the
// endpoint deterministic (two GETs serve byte-identical payloads).
func NewScalingStatelessHandler(logger *slog.Logger) *ScalingStatelessHandler {
	return &ScalingStatelessHandler{logger: logger}
}

// ServeHTTP implements http.Handler: build the round from the default
// config, run it, shape the payload from the observed facts.
func (h *ScalingStatelessHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	round, err := demos.NewScalingStatelessRound(demos.DefaultScalingStatelessConfig())
	if err != nil {
		// Unreachable with the default config — a construction failure
		// here is a programmer bug, surfaced honestly rather than
		// swallowed.
		h.logger.Error("scaling round construction failed", "error", err)
		core.WriteError(w, r, core.Wrap(err, core.KindInternal,
			"The scaling demo could not build its fleet."), h.logger)
		return
	}
	writeJSON(w, http.StatusOK, demos.BuildScalingStatelessPayload(round.Run()))
}
