// Demo API handler for topic 36 environments — the env inspector. Thin
// by the family rule: the handler owns nothing but the JSON write. It
// receives the REAL config and the REAL scenario store at wiring time
// (constructor injection, no globals); every request derives the
// payload fresh from the live cfg — whose values were validated at
// boot and are immutable since — the current environment (for
// attribution, re-read through the same os.Getenv the boot used), and
// the store's CURRENT snapshot, so a scenario flip is reflected by the
// very next reload. Sensitive slots are redacted inside the domain
// builder BEFORE the payload exists, so no code path here can serve a
// credential.
package presentation

import (
	"log/slog"
	"net/http"
	"os"

	"guide/internal/core"
	"guide/internal/domain/demos"
)

// Compile-time proof the handler satisfies http.Handler.
var _ http.Handler = (*EnvInspectorHandler)(nil)

// EnvInspectorHandler serves GET /api/demo/env/inspector, the
// environments topic's demo payload: this process's resolved
// configuration with per-variable attribution, the twelve-factor
// scorecard as data, and the live flag panel.
type EnvInspectorHandler struct {
	cfg    core.Config
	store  *core.ScenarioStore
	logger *slog.Logger
	// getenv resolves variable presence for attribution; a field so
	// tests inject deterministic environments without mutating the
	// process environment. The value itself always comes from cfg —
	// what the process booted with.
	getenv func(string) string
}

// NewEnvInspectorHandler wires the inspector over the real config, the
// real scenario store, and the process environment.
func NewEnvInspectorHandler(cfg core.Config, store *core.ScenarioStore, logger *slog.Logger) *EnvInspectorHandler {
	return &EnvInspectorHandler{cfg: cfg, store: store, logger: logger, getenv: os.Getenv}
}

// ServeHTTP implements http.Handler: one derived payload, served.
func (h *EnvInspectorHandler) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, demos.BuildEnvInspectorPayload(h.cfg, h.getenv, h.store.Snapshot()))
}
