package presentation

import (
	"io"
	"net/http"

	"guide/internal/domain/demos"
)

// request-response topic demo handlers. The inspector's payload is a
// function of the REQUEST ITSELF (like the headers echo, unlike the
// zero-input builders): the handler's only job is to extract the facts —
// method, path, content type, and the body capped at a sane limit — and
// hand them to the pure builder. The methods report is zero-input: the
// exhibit runs in-process over recorders, and the handler adds only
// transport (family invariant: thin handler, pure domain function, one
// JSON write).

// ReqresInspectorHandler serves GET /api/demo/reqres/inspector, the
// request-response topic's request-side demo: the live request's facts
// beside the json.Decoder discipline's deterministic verdicts over four
// exhibit bodies and over whatever body this GET carried. The handler
// holds no dependencies: constructing one allocates nothing to wire.
type ReqresInspectorHandler struct{}

// NewReqresInspectorHandler wires the request-inspector demo endpoint.
func NewReqresInspectorHandler() *ReqresInspectorHandler {
	return &ReqresInspectorHandler{}
}

// ServeHTTP implements http.Handler for the inspector payload. Facts out
// of the request, verdicts out of the builder, one writeJSON.
//
// The body is read through an io.LimitReader of one byte past the cap:
// a body up to the cap arrives whole, a larger one arrives truncated at
// exactly the cap with the exceeded flag set — reported, never silently
// dropped. A mid-read transport error cannot occur through a LimitReader
// over a fully-buffered test stream and on the wire means the peer is
// gone; the bytes that did arrive are still worth reporting.
func (h *ReqresInspectorHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	capped, err := io.ReadAll(io.LimitReader(r.Body, demos.MaxReqresBodyBytes+1))
	if err != nil {
		// Partial bytes are the honest remainder of a vanished peer.
		capped = capped[:min(len(capped), demos.MaxReqresBodyBytes)]
	}
	exceeded := len(capped) > demos.MaxReqresBodyBytes
	if exceeded {
		capped = capped[:demos.MaxReqresBodyBytes]
	}
	writeJSON(w, http.StatusOK, demos.BuildReqresInspectorPayload(demos.ReqresInspectorFacts{
		Method:            r.Method,
		Path:              r.URL.Path,
		ContentType:       r.Header.Get("Content-Type"),
		Body:              capped,
		BodyLimitExceeded: exceeded,
	}))
}

// ReqresMethodsHandler serves GET /api/demo/reqres/methods, the topic's
// response-side demo: the multi-method exhibit (status, headers, bodies,
// Allow, a truthful 405) and the SSE beat, every row OBSERVED through an
// in-process recorder rather than claimed.
type ReqresMethodsHandler struct{}

// NewReqresMethodsHandler wires the methods-report demo endpoint.
func NewReqresMethodsHandler() *ReqresMethodsHandler {
	return &ReqresMethodsHandler{}
}

// ServeHTTP implements http.Handler for the methods report. Zero inputs,
// one pure builder, one JSON write.
func (h *ReqresMethodsHandler) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, demos.BuildReqresMethodsPayload())
}
