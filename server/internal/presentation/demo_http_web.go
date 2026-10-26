package presentation

import (
	"net/http"

	"guide/internal/core"
	"guide/internal/domain/demos"
)

// http-web topic demo handlers. Unlike the zero-input demo endpoints, the
// headers echo's payload is a function of the REQUEST ITSELF: the handler's
// only job is to extract the facts net/http already parsed — method, path,
// raw query, negotiated protocol, the request-id middleware's id, and the
// header map — and hand them to the pure builder (family invariant: thin
// handler, pure domain function, one JSON write).

// HTTPHeadersHandler serves GET /api/demo/http/headers, the http-web
// topic's inline demo: an echo of what the transport actually delivered,
// classified (end-to-end vs hop-by-hop, negotiation/CORS/security/identity
// roles) with credentials detected but never echoed. The handler holds no
// dependencies: constructing one allocates nothing to wire.
type HTTPHeadersHandler struct{}

// NewHTTPHeadersHandler wires the headers echo demo endpoint.
func NewHTTPHeadersHandler() *HTTPHeadersHandler {
	return &HTTPHeadersHandler{}
}

// ServeHTTP implements http.Handler for the headers echo. Facts out of the
// request, classification into the payload, one writeJSON — nothing else.
func (h *HTTPHeadersHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, demos.BuildHTTPHeadersEcho(demos.HTTPRequestFacts{
		Method:    r.Method,
		Path:      r.URL.Path,
		Query:     r.URL.RawQuery,
		Proto:     r.Proto,
		RequestID: core.RequestIDFromContext(r.Context()),
		Header:    r.Header,
	}))
}
