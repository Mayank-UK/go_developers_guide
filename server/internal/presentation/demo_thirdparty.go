// Demo API handler for topic 42 third-party — the adapter-surface
// hub. The handler is as thin as the guide's shape demands: it owns no
// policy and no narration. Each request snapshots the SHARED
// ScenarioStore (the same runtime toggle every fake/live seam reads),
// hands the pure builder the snapshot plus the process's real
// credential resolver (os.LookupEnv — the live skeletons gate on it
// and refuse honestly when the environment holds no vendor key), and
// writes the payload. The builder drives fresh deterministic sandboxes
// through fixed scripts, so the endpoint is stateless: no process-
// lifetime console, nothing to start or drain.
package presentation

import (
	"log/slog"
	"net/http"
	"os"

	"guide/internal/core"
	"guide/internal/domain/demos"
)

// Compile-time proof the handler satisfies http.Handler.
var _ http.Handler = (*ThirdPartyHubHandler)(nil)

// ThirdPartyHubHandler serves GET /api/demo/thirdparty/hub, the
// third-party topic's demo payload: the DI switch's position, the
// three vendor ports with their sandbox runs and live refusals, and
// the vendor-error taxonomy map.
type ThirdPartyHubHandler struct {
	store       *core.ScenarioStore
	credentials func(name string) (string, bool)
	logger      *slog.Logger
}

// NewThirdPartyHubHandler wires the handler over the shared scenario
// store. The credential resolver is os.LookupEnv — the live skeletons
// read only the PRESENCE of the vendor key env vars, never values,
// and no vendor call is ever made.
func NewThirdPartyHubHandler(store *core.ScenarioStore, logger *slog.Logger) *ThirdPartyHubHandler {
	return &ThirdPartyHubHandler{store: store, credentials: os.LookupEnv, logger: logger}
}

// ServeHTTP implements http.Handler: snapshot the scenario, build the
// payload from the observed sandbox runs and live refusals, write it.
func (h *ThirdPartyHubHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	facts := demos.ThirdPartyHubFacts{Scenario: h.store.Snapshot(), Credentials: h.credentials}
	writeJSON(w, http.StatusOK, demos.BuildThirdPartyHubPayload(facts))
}
