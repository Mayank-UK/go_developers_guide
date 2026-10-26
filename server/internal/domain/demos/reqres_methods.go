// Request-methods demo payload: the response side of topic 09, made
// observable — every status/header/body claim a handler makes, verified
// by a recorder instead of asserted by prose.
//
// GET /api/demo/reqres/methods serves this payload. The exhibit is REAL
// executable code — ServeReqresExhibit answers eight methods (GET, POST,
// PUT, PATCH, DELETE, HEAD, OPTIONS, and a truthful 405 for anything
// else) and writeReqresSSEBeat streams a small event-source beat with a
// Flush per event. The builder runs each one over an in-process recorder
// and reports what the recorder OBSERVED: the status, the headers, the
// exact body, and — for the beat — the parsed events and the flush count.
// No timing values anywhere: the loop is synchronous and the payload is
// byte-for-byte deterministic.
package demos

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
)

// ReqresExhibitPath is the fictional resource the exhibit models — a
// single note-ish object the methods operate on. It never routes
// anywhere; it exists so Location and the payload's resource identity
// have one honest source.
const ReqresExhibitPath = "/api/demo/reqres/methods/exhibit"

// ReqresAllowValue is the exhibit's capability statement: the methods it
// serves, in teaching order (safe reads first, then the writers), used
// verbatim for the OPTIONS response and every 405.
const ReqresAllowValue = "GET, HEAD, OPTIONS, POST, PUT, PATCH, DELETE"

// reqresSSEEventCount is the number of events the SSE beat streams — a
// fixed, named bound like every fan-out in this guide.
const reqresSSEEventCount = 3

// ServeReqresExhibit is the multi-method exhibit handler — pure teaching
// code in production shape: one switch over the method, headers set
// before WriteHeader, each status chosen for what it claims. It is
// exercised by the builder over recorders and directly by tests; wiring
// it to a route would work unchanged.
func ServeReqresExhibit(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-Reqres-Method", r.Method)
	switch r.Method {
	case http.MethodGet:
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		http.SetCookie(w, &http.Cookie{
			Name:     "reqres_visit",
			Value:    "1",
			Path:     "/api/demo/reqres",
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
		})
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"resource":"methods-exhibit","state":"steady"}`)
	case http.MethodPost:
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Location", ReqresExhibitPath+"/17")
		w.WriteHeader(http.StatusCreated)
		fmt.Fprintf(w, `{"id":17,"state":"created"}`)
	case http.MethodPut:
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"id":17,"state":"replaced"}`)
	case http.MethodPatch:
		// 204: the headers are the whole answer — a body on a 204 is a
		// protocol violation, so none is written.
		w.WriteHeader(http.StatusNoContent)
	case http.MethodDelete:
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		fmt.Fprintf(w, `{"id":17,"state":"deletion scheduled"}`)
	case http.MethodHead:
		// HEAD answers exactly what GET would — headers, cookie, status —
		// and nothing else: the body is the only thing HEAD omits.
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		http.SetCookie(w, &http.Cookie{
			Name:     "reqres_visit",
			Value:    "1",
			Path:     "/api/demo/reqres",
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
		})
		w.WriteHeader(http.StatusOK)
	case http.MethodOptions:
		w.Header().Set("Allow", ReqresAllowValue)
		w.WriteHeader(http.StatusNoContent)
	default:
		// A real HTTP method this resource does not serve: 405 (not 501),
		// with Allow saying what IS served.
		w.Header().Set("Allow", ReqresAllowValue)
		w.WriteHeader(http.StatusMethodNotAllowed)
		fmt.Fprintf(w, `{"error":%q,"served":%q}`, r.Method, ReqresAllowValue)
	}
}

// writeReqresSSEBeat streams the beat: one Server-Sent-Event block per
// iteration, each followed by a Flush so it reaches the client as it is
// produced. The flush is guarded — a writer that cannot flush still gets
// the bytes, the same degradation the middleware chain's recorder uses.
func writeReqresSSEBeat(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	for sequence := 1; sequence <= reqresSSEEventCount; sequence++ {
		fmt.Fprintf(w, "event: beat\ndata: {\"sequence\":%d,\"state\":\"tick\"}\n\n", sequence)
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
	}
}

// reqresCountingRecorder wraps the recorder so the beat's payload can
// report the flush COUNT, not just that flushing happened — the recorder
// implements http.Flusher, and the wrapper shadows its Flush to count.
type reqresCountingRecorder struct {
	*httptest.ResponseRecorder
	flushes int
}

// Flush counts, then forwards to the real recorder.
func (c *reqresCountingRecorder) Flush() {
	c.flushes++
	c.ResponseRecorder.Flush()
}

// ReqresObservedHeader is one response header exactly as the recorder saw
// it after the exhibit ran — observed, not claimed.
type ReqresObservedHeader struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// ReqresMethodRow is one method's observed outcome: status (+ phrase),
// the selected headers, a one-line body shape, and the exact body bytes.
type ReqresMethodRow struct {
	Method     string                 `json:"method"`
	Status     int                    `json:"status"`
	StatusText string                 `json:"statusText"`
	Headers    []ReqresObservedHeader `json:"headers"`
	BodyShape  string                 `json:"bodyShape"`
	Body       string                 `json:"body"`
}

// ReqresSSEEvent is one parsed event block: its name and its data
// payload, verbatim.
type ReqresSSEEvent struct {
	Name string `json:"name"`
	Data string `json:"data"`
}

// ReqresSSEBeat is the streaming beat reported structurally: the content
// type, the parsed events, the flush count, and the framing rule.
type ReqresSSEBeat struct {
	ContentType string           `json:"contentType"`
	Events      []ReqresSSEEvent `json:"events"`
	Flushes     int              `json:"flushes"`
	Framing     string           `json:"framing"`
}

// ReqresMethodsPayload is the wire shape of GET /api/demo/reqres/methods
// (camelCase JSON) — the request-response topic's response-side demo.
type ReqresMethodsPayload struct {
	Resource string            `json:"resource"`
	Allow    string            `json:"allow"`
	Methods  []ReqresMethodRow `json:"methods"`
	SSE      ReqresSSEBeat     `json:"sse"`
}

// reqresHeaderSelection is the header order each row reports: the
// teaching headers first; only the ones the response actually carries.
var reqresHeaderSelection = []string{
	"Content-Type",
	"Cache-Control",
	"Set-Cookie",
	"Location",
	"Allow",
	"X-Reqres-Method",
}

// reqresObservedHeaders reads the selected headers off one finished
// response, in selection order.
func reqresObservedHeaders(header http.Header) []ReqresObservedHeader {
	observed := make([]ReqresObservedHeader, 0, len(reqresHeaderSelection))
	for _, name := range reqresHeaderSelection {
		if value := header.Get(name); value != "" {
			observed = append(observed, ReqresObservedHeader{Name: name, Value: value})
		}
	}
	return observed
}

// reqresBodyShape describes each method's body in one honest line — the
// shape rule, written where the exhibit applies it.
func reqresBodyShape(method string) string {
	switch method {
	case http.MethodGet:
		return "one JSON object — the resource's current state"
	case http.MethodPost:
		return "one JSON object — the created record; Location names where it lives"
	case http.MethodPut:
		return "one JSON object — the full replacement"
	case http.MethodPatch:
		return "no body — 204 means the headers are the whole answer"
	case http.MethodDelete:
		return "one JSON object — the acceptance note for scheduled work"
	case http.MethodHead:
		return "no body by definition — HEAD answers GET's headers and nothing else"
	case http.MethodOptions:
		return "no body — 204 again; Allow is the payload"
	default:
		return "one JSON object — the error note; Allow states what IS served"
	}
}

// reqresMethodRow drives the exhibit once with one method and shapes the
// recorder's observation into a row.
func reqresMethodRow(method string) ReqresMethodRow {
	recorder := httptest.NewRecorder()
	ServeReqresExhibit(recorder, httptest.NewRequest(method, ReqresExhibitPath, nil))
	return ReqresMethodRow{
		Method:     method,
		Status:     recorder.Code,
		StatusText: http.StatusText(recorder.Code),
		Headers:    reqresObservedHeaders(recorder.Header()),
		BodyShape:  reqresBodyShape(method),
		Body:       recorder.Body.String(),
	}
}

// reqresMethodOrder is the report's row order: the served methods in
// teaching order, then the truthful 405 for a method that exists but is
// not served (TRACE — a real HTTP verb, so 405 is the honest status).
var reqresMethodOrder = []string{
	http.MethodGet,
	http.MethodPost,
	http.MethodPut,
	http.MethodPatch,
	http.MethodDelete,
	http.MethodHead,
	http.MethodOptions,
	"TRACE",
}

// reqresSSEBeat runs the beat over the counting recorder and parses what
// arrived: one event per blank-line-terminated block, flushes counted by
// the wrapper. No timing values exist to report — the loop is
// synchronous.
func reqresSSEBeat() ReqresSSEBeat {
	recorder := &reqresCountingRecorder{ResponseRecorder: httptest.NewRecorder()}
	writeReqresSSEBeat(recorder)

	events := make([]ReqresSSEEvent, 0, reqresSSEEventCount)
	for _, block := range strings.Split(strings.TrimSuffix(recorder.Body.String(), "\n\n"), "\n\n") {
		event := ReqresSSEEvent{}
		for _, line := range strings.Split(block, "\n") {
			if name, ok := strings.CutPrefix(line, "event: "); ok {
				event.Name = name
			}
			if data, ok := strings.CutPrefix(line, "data: "); ok {
				event.Data = data
			}
		}
		events = append(events, event)
	}
	return ReqresSSEBeat{
		ContentType: recorder.Header().Get("Content-Type"),
		Events:      events,
		Flushes:     recorder.flushes,
		Framing:     "each event block ends with a blank line — the delimiter a client parser waits for",
	}
}

// BuildReqresMethodsPayload assembles the whole report. It is pure and
// zero-input: every row is the exhibit's own behavior observed through a
// fresh recorder, so the same call yields the same bytes every time.
func BuildReqresMethodsPayload() ReqresMethodsPayload {
	rows := make([]ReqresMethodRow, 0, len(reqresMethodOrder))
	for _, method := range reqresMethodOrder {
		rows = append(rows, reqresMethodRow(method))
	}
	return ReqresMethodsPayload{
		Resource: ReqresExhibitPath,
		Allow:    ReqresAllowValue,
		Methods:  rows,
		SSE:      reqresSSEBeat(),
	}
}
