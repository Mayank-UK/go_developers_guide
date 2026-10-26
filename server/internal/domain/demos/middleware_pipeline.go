package demos

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"time"

	"guide/internal/core"
)

// The middleware topic's exhibit chain: REAL net/http middleware, teaching
// grade. The production chain (presentation.Chain over RequestID,
// RequestLogger, Recover, EnvelopeMuxErrors) is what the topic's prose
// centers on; this file is the runnable miniature the live demo performs —
// the same layer signature, the same ordering semantics, small enough to
// read top to bottom. The middleware topic quotes this file as real code.
//
// The builder runs the exhibit chain over httptest.NewRecorder requests
// IN-PROCESS (no sockets) and reports what happened: the before/after
// execution order, the request id's propagation across layers, the
// recovered-panic beat (a terminal handler that panics deliberately), and
// an http.TimeoutHandler run against a slow terminal handler. Everything
// recorded is deterministic: sequence numbers come from append order, the
// traced request carries its own inbound id, and the timeout beat records
// SHAPE (budget vs sleep vs observed status), never wall-clock timings.

// Trace phases: a layer's code before it calls the next layer, the
// terminal handler itself, and a layer's code after the inner stack
// returned (or panicked through).
const (
	phaseBefore  = "before"
	phaseHandler = "handler"
	phaseAfter   = "after"
)

// The demo run's facts — one inbound request id (so the payload is
// deterministic), and the timeout beat's fixed arithmetic (the handler
// sleeps four times the budget, so the timer always wins, comfortably).
const (
	pipelineDemoRequestID   = "middleware-demo-42"
	pipelineTimeoutBudget   = 30 * time.Millisecond
	pipelineSlowHandlerWait = 120 * time.Millisecond
	// pipelineSlowJoinWait bounds the wait for the abandoned slow handler
	// to finish its sleep after TimeoutHandler has already answered —
	// bounded waits only, no goroutine left hanging past the builder.
	pipelineSlowJoinWait = 5 * time.Second
)

// ChainLayer is one row of the exhibit chain, outermost first: position 0
// sees the request before every other layer.
type ChainLayer struct {
	Name     string `json:"name"`
	Position int    `json:"position"`
	Role     string `json:"role"`
	Mirrors  string `json:"mirrors"`
}

// TraceEvent is one beat of the traced run; Sequence is the chronological
// truth (append order), and RequestIDSeen is the id visible in the request
// context at that moment — "" is data: the beat fired before the request-id
// layer had assigned anything.
type TraceEvent struct {
	Layer         string `json:"layer"`
	Phase         string `json:"phase"`
	Sequence      int    `json:"sequence"`
	RequestIDSeen string `json:"requestIDSeen"`
	Note          string `json:"note"`
}

// RequestIDBeat is the propagation proof: one id in, the same id at every
// layer that looks, and the same id echoed on the response header.
type RequestIDBeat struct {
	HeaderName       string   `json:"headerName"`
	Inbound          string   `json:"inbound"`
	SeenAtLayers     []string `json:"seenAtLayers"`
	ResponseHeader   string   `json:"responseHeader"`
	SameIDEverywhere bool     `json:"sameIDEverywhere"`
}

// PanicRecoveryBeat is the recovered-panic run's report: the deliberate
// bug, the 500 envelope the recover layer wrote, the error log line it
// recorded (captured from a real slog handler, its time= field removed —
// the rest verbatim), and the panic run's own trace — which proves the
// unwind order survives a panic: recover's after-beat fires first, then
// the logger's, then request-id's.
type PanicRecoveryBeat struct {
	Path          string       `json:"path"`
	Cause         string       `json:"cause"`
	Status        int          `json:"status"`
	Body          string       `json:"body"`
	LoggedLine    string       `json:"loggedLine"`
	RequestIDKept bool         `json:"requestIDKept"`
	Trace         []TraceEvent `json:"trace"`
}

// MiddlewareTimeoutBeat is the http.TimeoutHandler run's report: the
// budget, the slow handler's sleep, the wrapper's own 503 — and the
// bounded join that proves the abandoned handler goroutine finished
// instead of leaking.
type MiddlewareTimeoutBeat struct {
	Wrapper             string `json:"wrapper"`
	BudgetMs            int    `json:"budgetMs"`
	HandlerSleepMs      int    `json:"handlerSleepMs"`
	ObservedStatus      int    `json:"observedStatus"`
	Body                string `json:"body"`
	SlowHandlerFinished bool   `json:"slowHandlerFinished"`
	Shape               string `json:"shape"`
}

// MiddlewarePipelinePayload is the wire shape of GET
// /api/demo/middleware/pipeline (camelCase JSON) — the middleware topic's
// inline demo: the exhibit chain as rows, the traced run as events, and
// the three proving beats.
type MiddlewarePipelinePayload struct {
	Chain             []ChainLayer          `json:"chain"`
	Trace             []TraceEvent          `json:"trace"`
	RequestSideOrder  []string              `json:"requestSideOrder"`
	ResponseSideOrder []string              `json:"responseSideOrder"`
	RequestID         RequestIDBeat         `json:"requestID"`
	PanicRecovery     PanicRecoveryBeat     `json:"panicRecovery"`
	TimeoutBeat       MiddlewareTimeoutBeat `json:"timeoutBeat"`
	Reasoning         string                `json:"reasoning"`
}

// record appends one trace event; the slice's length IS the next sequence
// number, which is what makes the trace deterministic and testable.
func record(events *[]TraceEvent, layer, phase, requestID, note string) {
	*events = append(*events, TraceEvent{
		Layer: layer, Phase: phase, Sequence: len(*events), RequestIDSeen: requestID, Note: note,
	})
}

// pipelineChain composes layers so the FIRST argument is the OUTERMOST —
// the same fold presentation.Chain runs, in miniature: wrap the terminal
// handler, then wrap that, outermost last.
func pipelineChain(layers ...func(http.Handler) http.Handler) func(http.Handler) http.Handler {
	return func(final http.Handler) http.Handler {
		for i := len(layers) - 1; i >= 0; i-- {
			final = layers[i](final)
		}
		return final
	}
}

// exhibitRequestID is the request-id layer, teaching grade: honor a trimmed
// inbound x-request-id (else mint one), stamp the response header BEFORE
// the inner stack runs, and propagate the id through the request context —
// via the REAL core helpers, so error envelopes written anywhere beneath
// this layer read the same id production code reads.
func exhibitRequestID(events *[]TraceEvent) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			record(events, "request-id", phaseBefore, core.RequestIDFromContext(r.Context()),
				"no id in the context yet — assigning one is this layer's job")
			id := strings.TrimSpace(r.Header.Get(core.RequestIDHeader))
			if id == "" {
				id = core.NewRequestID()
			}
			w.Header().Set(core.RequestIDHeader, id)
			next.ServeHTTP(w, r.WithContext(core.WithRequestID(r.Context(), id)))
			// The after-beat reports the id directly: this frame's r is the
			// ORIGINAL request, which never carried the derived context.
			record(events, "request-id", phaseAfter, id,
				"the response header was stamped before the inner stack ran, so even the 500 envelope carries the id")
		})
	}
}

// exhibitAccessLog is the logging layer's observation half, teaching grade:
// one event entering, one event leaving. The real RequestLogger also
// captures the status and writes one structured slog line per request at
// completion; this miniature records the beats that make the ordering
// visible instead.
func exhibitAccessLog(events *[]TraceEvent) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			record(events, "request-logger", phaseBefore, core.RequestIDFromContext(r.Context()),
				"the id is already in the context — the access log can cite it on every line")
			next.ServeHTTP(w, r)
			record(events, "request-logger", phaseAfter, core.RequestIDFromContext(r.Context()),
				"the request is logged at completion — everything beneath this layer is inside the duration")
		})
	}
}

// exhibitRecover is the panic-recovery layer, teaching grade: a deferred
// recover that converts a panicking handler into the family's 500 error
// envelope (via the real central writer) and one error log line carrying
// the request id. The real layer also re-raises http.ErrAbortHandler and
// leaves committed responses alone; those edges are pinned in
// presentation/middleware_test.go.
func exhibitRecover(events *[]TraceEvent, logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			record(events, "recover", phaseBefore, core.RequestIDFromContext(r.Context()),
				"the deferred recover is armed — nothing happens unless the inner stack panics")
			defer func() {
				if recovered := recover(); recovered != nil {
					logger.Error("panic recovered",
						"request_id", core.RequestIDFromContext(r.Context()),
						"panic", recovered)
					core.WriteError(w, r, core.Internal("Internal server error."), logger)
					record(events, "recover", phaseAfter, core.RequestIDFromContext(r.Context()),
						"the panic was converted: 500 envelope written, error logged with the same id")
					return
				}
				record(events, "recover", phaseAfter, core.RequestIDFromContext(r.Context()),
					"no panic on this request — the after-beat simply unwinds")
			}()
			next.ServeHTTP(w, r)
		})
	}
}

// exhibitTerminalOK is the happy-path terminal handler: the handler beat,
// then a minimal 200 JSON response every layer passes outward untouched.
func exhibitTerminalOK(events *[]TraceEvent) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		record(events, "terminal handler", phaseHandler, core.RequestIDFromContext(r.Context()),
			"the endpoint runs exactly once, in the middle — every layer's before-code has run, no after-code has")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	})
}

// exhibitTerminalPanic is the deliberate programmer bug: it records its
// beat and panics — the exact shape Recover exists to catch.
func exhibitTerminalPanic(events *[]TraceEvent) http.Handler {
	return http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		record(events, "terminal handler", phaseHandler, core.RequestIDFromContext(r.Context()),
			"the deliberate bug: a write to a nil map mid-handler")
		panic("assignment to entry in nil map")
	})
}

// exhibitSlowHandler sleeps past any TimeoutHandler budget, then reports
// completion on the channel (the bounded join that proves no goroutine
// leak) — and records that it observed cancellation on r.Context(), the
// half of http.TimeoutHandler most code forgets.
func exhibitSlowHandler(done chan<- struct{}) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-time.After(pipelineSlowHandlerWait):
			// The wrapper never got a timely response; anything written now
			// lands on an abandoned buffer and is discarded.
		case <-r.Context().Done():
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"slow":"finally"}`))
		close(done)
	})
}

// exhibitChainLayers is the exhibit chain's own description, outermost
// first — each row names the production layer it mirrors.
func exhibitChainLayers() []ChainLayer {
	return []ChainLayer{
		{Name: "request-id", Position: 0,
			Role:    "assigns or honors one id per request, stamps the response header, and puts the id in the request context",
			Mirrors: "presentation.RequestID"},
		{Name: "request-logger", Position: 1,
			Role:    "one structured access-log line per request at completion, status and duration included",
			Mirrors: "presentation.RequestLogger"},
		{Name: "recover", Position: 2,
			Role:    "a deferred recover that converts handler panics into the 500 error envelope plus an error log",
			Mirrors: "presentation.Recover"},
		{Name: "terminal handler", Position: 3,
			Role:    "the mux and the endpoint: routing (topic 07) and the response itself — innermost, wrapped by everything",
			Mirrors: "http.ServeMux + the demo handlers"},
	}
}

// runExhibitOKRun sends one GET with a fixed inbound request id through the
// full exhibit chain and returns its trace (the recorder's facts feed the
// request-id beat).
func runExhibitOKRun() (events []TraceEvent, rec *httptest.ResponseRecorder) {
	var logBuf strings.Builder
	logger := slog.New(slog.NewTextHandler(&logBuf, nil))
	chain := pipelineChain(
		exhibitRequestID(&events),
		exhibitAccessLog(&events),
		exhibitRecover(&events, logger),
	)(exhibitTerminalOK(&events))

	rec = httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/demo/middleware/pipeline", nil)
	req.Header.Set(core.RequestIDHeader, pipelineDemoRequestID)
	chain.ServeHTTP(rec, req)
	return events, rec
}

// runExhibitPanicRun sends the same request shape at a terminal handler
// that panics deliberately; it returns the trace, the recorder, and the
// captured panic log (the "panic recovered" line, time= field removed).
func runExhibitPanicRun() (events []TraceEvent, rec *httptest.ResponseRecorder, panicLog string) {
	var logBuf strings.Builder
	logger := slog.New(slog.NewTextHandler(&logBuf, nil))
	chain := pipelineChain(
		exhibitRequestID(&events),
		exhibitAccessLog(&events),
		exhibitRecover(&events, logger),
	)(exhibitTerminalPanic(&events))

	rec = httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/demo/middleware/pipeline", nil)
	req.Header.Set(core.RequestIDHeader, pipelineDemoRequestID)
	chain.ServeHTTP(rec, req)
	return events, rec, stripSlogTimeField(logBuf.String())
}

// stripSlogTimeField removes the leading time="…" token slog's text
// handler emits, so a captured log line is deterministic; everything after
// the time field is verbatim.
func stripSlogTimeField(line string) string {
	for _, raw := range strings.Split(line, "\n") {
		if strings.Contains(raw, `msg="panic recovered"`) {
			if i := strings.Index(raw, "level="); i >= 0 {
				return raw[i:]
			}
			return raw
		}
	}
	return ""
}

// runExhibitTimeoutRun wraps a slow terminal handler in http.TimeoutHandler
// with a budget the sleep always exceeds, and reports the wrapper's own
// answer plus the bounded join on the abandoned handler. The empty message
// argument deliberately selects the wrapper's classic canned body.
func runExhibitTimeoutRun() MiddlewareTimeoutBeat {
	done := make(chan struct{})
	wrapped := http.TimeoutHandler(exhibitSlowHandler(done), pipelineTimeoutBudget, "")
	rec := httptest.NewRecorder()
	wrapped.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/demo/middleware/pipeline", nil))

	finished := false
	select {
	case <-done:
		finished = true
	case <-time.After(pipelineSlowJoinWait):
		finished = false
	}
	return MiddlewareTimeoutBeat{
		Wrapper:             "http.TimeoutHandler",
		BudgetMs:            int(pipelineTimeoutBudget.Milliseconds()),
		HandlerSleepMs:      int(pipelineSlowHandlerWait.Milliseconds()),
		ObservedStatus:      rec.Code,
		Body:                rec.Body.String(),
		SlowHandlerFinished: finished,
		Shape:               "the wrapper races the handler against the budget, derives a timeout context so r.Context() is cancelled, and answers 503 with its own body; the abandoned handler keeps running — its writes are discarded, and this payload waited (bounded) for it to finish",
	}
}

// requestIDBeat folds the traced run's facts into the propagation proof:
// every layer after request-id saw the same non-empty id, and the response
// header echoes the inbound id.
func requestIDBeat(events []TraceEvent, rec *httptest.ResponseRecorder) RequestIDBeat {
	seen := make([]string, 0, len(events))
	same := true
	for _, event := range events {
		if event.RequestIDSeen == "" {
			if event.Phase != phaseBefore || event.Layer != "request-id" {
				same = false
			}
			continue
		}
		if event.RequestIDSeen != pipelineDemoRequestID {
			same = false
		}
		seen = append(seen, event.Layer+" ("+event.Phase+")")
	}
	return RequestIDBeat{
		HeaderName:       core.RequestIDHeader,
		Inbound:          pipelineDemoRequestID,
		SeenAtLayers:     seen,
		ResponseHeader:   rec.Header().Get(core.RequestIDHeader),
		SameIDEverywhere: same && rec.Header().Get(core.RequestIDHeader) == pipelineDemoRequestID,
	}
}

// BuildMiddlewarePipelinePayload assembles the whole demo payload by
// running the exhibit chain in-process. It is pure: no inputs, no I/O, no
// clock in the recorded facts — the same call yields the same payload.
func BuildMiddlewarePipelinePayload() MiddlewarePipelinePayload {
	events, okRec := runExhibitOKRun()
	panicEvents, panicRec, panicLog := runExhibitPanicRun()

	layers := exhibitChainLayers()
	order := make([]string, 0, len(layers))
	for _, layer := range layers {
		order = append(order, layer.Name)
	}
	reversed := make([]string, 0, len(order))
	for i := len(order) - 1; i >= 0; i-- {
		reversed = append(reversed, order[i])
	}

	return MiddlewarePipelinePayload{
		Chain:             layers,
		Trace:             events,
		RequestSideOrder:  order,
		ResponseSideOrder: reversed,
		RequestID:         requestIDBeat(events, okRec),
		PanicRecovery: PanicRecoveryBeat{
			Path:          "/api/demo/middleware/pipeline",
			Cause:         "assignment to entry in nil map",
			Status:        panicRec.Code,
			Body:          panicRec.Body.String(),
			LoggedLine:    panicLog,
			RequestIDKept: panicRec.Header().Get(core.RequestIDHeader) == pipelineDemoRequestID,
			Trace:         panicEvents,
		},
		TimeoutBeat: runExhibitTimeoutRun(),
		Reasoning: "One request, three middleware layers, one terminal handler: request-side code runs " +
			"outermost-first because each layer wraps the next, the handler runs exactly once in the middle, " +
			"and response-side code unwinds innermost-first because code after next.ServeHTTP only runs once " +
			"the inner stack has finished — even when what comes back is a recovered panic. The trace is a real " +
			"in-process run of real middleware over httptest, and the live proof rides on this very response: " +
			"its x-request-id header was stamped by the production RequestID after this payload was built — " +
			"check them agree with curl -i.",
	}
}
