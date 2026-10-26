package presentation

import (
	"net/http"

	"guide/internal/core"
	"guide/internal/domain/demos"
)

// go-http-fundamentals topic demo handler — one file per topic's demos in
// the wave authoring pattern, thin like every demo handler: delegate to the
// pure builder and write the JSON. This one holds exactly one dependency,
// the typed config, because the demo reports the environment the server
// actually booted in — the handler supplies it; the builder stays pure.

// AppInfoHandler serves GET /api/demo/gohttp/app-info, the
// go-http-fundamentals topic's inline demo: the running server introducing
// itself — module and toolchain facts from runtime/debug.ReadBuildInfo,
// scheduler-facing machine facts from the runtime package, and the port,
// environment, and log level core.LoadConfig resolved at boot.
type AppInfoHandler struct {
	config core.Config
}

// NewAppInfoHandler wires the app-info demo endpoint with the server's
// booted configuration.
func NewAppInfoHandler(config core.Config) *AppInfoHandler {
	return &AppInfoHandler{config: config}
}

// ServeHTTP implements http.Handler for the app-info demo payload. No input
// to parse and no I/O to await, so the request goes unused.
func (h *AppInfoHandler) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, demos.BuildAppInfoPayload(h.config))
}
