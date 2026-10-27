// Demo API handler for topic 27 internationalization — the negotiator
// story endpoint: one request's Accept-Language header walked from raw
// text to a served message (parsed candidates, the fallback chain, the
// same message per catalog, plurals, formats, timezones, and the two
// honest edges). The handler is deliberately the thinnest in the guide:
// the header it negotiates with is the request's own, read verbatim and
// handed to the story runner — the demo's whole point is that every
// beat re-derives from what THIS request actually sent.
package presentation

import (
	"log/slog"
	"net/http"

	"guide/internal/core"
	"guide/internal/domain/demos"
)

// Compile-time proof the handler satisfies http.Handler.
var _ http.Handler = (*I18nNegotiatorHandler)(nil)

// I18nNegotiatorHandler serves GET /api/demo/i18n/negotiator: the i18n
// story derived from the request's own Accept-Language header. A request
// that carries none runs the RFC 9110 §12.5.4 fixture instead (the
// console substitutes it and the story says which ran).
type I18nNegotiatorHandler struct {
	runner demos.I18nNegotiatorRunner
	logger *slog.Logger
}

// NewI18nNegotiatorHandler wires the negotiator story endpoint: the
// runner port (the data-layer console implements it over the embedded
// catalogs) and the process logger.
func NewI18nNegotiatorHandler(runner demos.I18nNegotiatorRunner, logger *slog.Logger) *I18nNegotiatorHandler {
	return &I18nNegotiatorHandler{runner: runner, logger: logger}
}

// ServeHTTP implements http.Handler for the negotiator story.
func (h *I18nNegotiatorHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// The request's OWN header, verbatim — the one thing this demo must
	// never do is negotiate with anything other than what actually came
	// in on the wire.
	header := r.Header.Get("Accept-Language")
	source := "request"
	if header == "" {
		source = "fixture" // the console substitutes the RFC 9110 fixture
	}
	run, err := h.runner.RunNegotiatorStory(r.Context(), header, source)
	if err != nil {
		core.WriteError(w, r, core.Wrap(err, core.KindInternal,
			"The i18n negotiator demo could not run its story against the catalogs."), h.logger)
		return
	}
	writeJSON(w, http.StatusOK, demos.BuildI18nNegotiatorPayload(run))
}
