// Demo API handlers for topic 12 api-documentation — two GETs, both thin:
// the docs-link endpoint serves the payload built (by parsing the real
// spec file) in internal/domain/demos, and the spec endpoint serves the
// embedded openapi.yaml VERBATIM with a YAML content type, so any
// OpenAPI-aware tool can consume the contract from the running server.
// The routes go live at the composition root (cmd/guide/main.go):
//
//	GET /api/demo/api/docs-link   the topic's demo payload
//	GET /api/openapi.yaml         the spec, byte for byte
package presentation

import (
	"log/slog"
	"net/http"

	"guide/api"
	"guide/internal/core"
	"guide/internal/domain/demos"
)

// Compile-time proofs that both handlers satisfy http.Handler — the
// interface the route registration below requires.
var (
	_ http.Handler = (*APIDocsLinkHandler)(nil)
	_ http.Handler = (*OpenAPISpecHandler)(nil)
)

// APIDocsLinkHandler serves GET /api/demo/api/docs-link, the
// api-documentation topic's demo payload: the spec's facts parsed from
// the file itself, the operations and schemas inventories, the
// docs-serving story, the drift guarantees, and the raw spec text.
type APIDocsLinkHandler struct {
	logger *slog.Logger
}

// NewAPIDocsLinkHandler wires the process logger; the builder itself is
// a pure function over the embedded spec.
func NewAPIDocsLinkHandler(logger *slog.Logger) *APIDocsLinkHandler {
	return &APIDocsLinkHandler{logger: logger}
}

// ServeHTTP implements http.Handler for the docs-link payload. The only
// failure the builder can report is an unparseable spec — a contract
// break that deserves the honest 500 envelope, never a half-rendered
// card.
func (h *APIDocsLinkHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	payload, err := demos.BuildAPIDocsLinkPayload()
	if err != nil {
		core.WriteError(w, r, core.Internal("The API documentation demo is unavailable."), h.logger)
		return
	}
	writeJSON(w, http.StatusOK, payload)
}

// OpenAPISpecHandler serves GET /api/openapi.yaml: the embedded spec,
// byte for byte, as application/yaml — the machine-readable contract at a
// stable URL. No transformation, no JSON conversion: what the repository
// holds is what the route answers, which is precisely the property that
// makes the document trustworthy to tools and drift tests alike.
type OpenAPISpecHandler struct{}

// NewOpenAPISpecHandler wires the spec-serving endpoint.
func NewOpenAPISpecHandler() *OpenAPISpecHandler {
	return &OpenAPISpecHandler{}
}

// ServeHTTP implements http.Handler: serve the spec verbatim.
func (h *OpenAPISpecHandler) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/yaml")
	w.Header().Set("Cache-Control", "no-cache")
	_, _ = w.Write(api.OpenAPIYAML)
}
