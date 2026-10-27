// Demo API handler for topic 32 architecture — the request-flow
// tracer. The handler owns exactly the execution the topic narrates and
// nothing else: it mounts the REAL posts list handler over the
// recorder-wired use case the composition root injected, walks one
// marker request through the production middleware chain in-process,
// and hands the observed facts to the pure builder in
// internal/domain/demos. The seams (middleware, handler boundary,
// repository port, leaf adapters) are timed by the injected sink, so
// every hop in the payload is a measurement taken where the code ran.
// One trace runs at a time — the mutex keeps the sink's reset-read
// window whole under concurrent demo hits; the traced request shares
// the OUTER request's id via the inbound header, so the payload proves
// one id served the whole thread.
package presentation

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"sync"

	"guide/internal/core"
	"guide/internal/domain/demos"
	"guide/internal/domain/posts"
)

// Compile-time proof the handler satisfies http.Handler.
var _ http.Handler = (*ArchTracerHandler)(nil)

// ArchTracerHandler serves GET /api/demo/arch/tracer, the architecture
// topic's demo payload: the traced posts-list timeline (middleware →
// thin handler → use case → the adapter the scenario picked) and the
// dependency-rule counterexample panel.
type ArchTracerHandler struct {
	store  *core.ScenarioStore
	list   *posts.ListPostsUseCase
	sink   *demos.ArchTraceSink
	logger *slog.Logger

	mu sync.Mutex
}

// NewArchTracerHandler wires the tracer over the scenario store, the
// recorder-wrapped posts list use case, the shared trace sink, and the
// process logger. The use case arrives pre-wired by the composition
// root because its repository chain crosses the data layer — the same
// boundary the production posts wiring crosses, in the same file.
func NewArchTracerHandler(
	store *core.ScenarioStore,
	list *posts.ListPostsUseCase,
	sink *demos.ArchTraceSink,
	logger *slog.Logger,
) *ArchTracerHandler {
	return &ArchTracerHandler{store: store, list: list, sink: sink, logger: logger}
}

// ServeHTTP implements http.Handler: reset the sink, walk the marker
// request through the production chain, then shape the payload from
// what actually ran.
func (h *ArchTracerHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.sink.Reset()
	requestID := core.RequestIDFromContext(r.Context())

	// The traced pipeline is the production shape: the real thin handler
	// on a private mux, timed at its boundary, behind the same middleware
	// order the composition root serves.
	traced := http.NewServeMux()
	traced.Handle("GET /api/posts", archTimedHandler(demos.ArchHandlerHop, h.sink)(NewPostsListHandler(h.list, h.logger)))
	chain := Chain(
		RequestID,
		RequestLogger(h.logger),
		Recover(h.logger),
		EnvelopeMuxErrors(h.logger),
	)(traced)

	marker, err := http.NewRequestWithContext(r.Context(), http.MethodGet, "/api/posts?limit=3&sort=newest", nil)
	if err != nil {
		// Unreachable: a fixed method and path cannot fail to build.
		core.WriteError(w, r, core.Wrap(err, core.KindInternal,
			"The architecture tracer could not build its marker request."), h.logger)
		return
	}
	// Thread the tracer's own request id into the walk as the INBOUND
	// header: the RequestID layer adopts it, so one id serves the tracer,
	// the traced request, and every log line between them.
	marker.Header.Set(core.RequestIDHeader, requestID)

	recorder := newBufferedResponseRecorder()
	started := h.sink.Now()
	chain.ServeHTTP(recorder, marker)
	elapsed := h.sink.Now().Sub(started)
	h.sink.Record(demos.ArchMiddlewareHop, started)

	facts := demos.ArchTracerFacts{
		DataMode:  h.store.Snapshot().DataMode,
		Hops:      h.sink.Hops(),
		Status:    recorder.status,
		Items:     archCountItems(recorder.body.String()),
		Elapsed:   elapsed,
		RequestID: requestID,
	}
	writeJSON(w, http.StatusOK, demos.BuildArchTracerPayload(facts))
}

// archTimedHandler wraps one handler boundary with the sink: the hop's
// clock starts when the layer above hands over and stops when the
// handler returns — inclusive of everything it called beneath.
func archTimedHandler(meta demos.ArchHopMeta, sink *demos.ArchTraceSink) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			started := sink.Now()
			next.ServeHTTP(w, r)
			sink.Record(meta, started)
		})
	}
}

// archCountItems reads the traced response's item count — the one fact
// about the BODY the payload reports. A non-JSON or failing body
// honestly counts zero; the status beside it says why.
func archCountItems(body string) int {
	var listed struct {
		Items []json.RawMessage `json:"items"`
	}
	if err := json.Unmarshal([]byte(body), &listed); err != nil {
		return 0
	}
	return len(listed.Items)
}
