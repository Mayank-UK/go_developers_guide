// Demo API handlers for topic 30 error-handling — the family's
// thin-handler invariant with the topic's own twist: the PROBE is the
// teaching. It honors the scenario store's failure injection for real
// — under "error" it panics deliberately so the production recovery
// layer converts the panic into the generic 500 envelope (logged with
// the stack and the request id), under "latency" it sleeps out the
// injection before answering, and under "none" it answers the healthy
// row. The EXPLORER walks that probe through the same middleware chain
// the composition root serves (RequestID → RequestLogger → Recover →
// EnvelopeMuxErrors), in-process, on every request — so the payload's
// walk row reports what the probe ACTUALLY answered under the current
// scenario, and the request id threads the explorer, the probe's
// response header, and the envelope the walk observed.
package presentation

import (
	"bytes"
	"log/slog"
	"net/http"
	"time"

	"guide/internal/core"
	"guide/internal/domain/demos"
)

// Compile-time proof both handlers satisfy http.Handler.
var (
	_ http.Handler = (*ErrorsProbeHandler)(nil)
	_ http.Handler = (*ErrorsExplorerHandler)(nil)
)

// ErrorsProbeHandler serves GET /api/demo/errors/probe — the
// scenario-honoring probe. The store is injected (the same instance
// PATCH /api/demo/scenario writes), and the sleep is a field so tests
// run the latency path without wall-clock waiting.
type ErrorsProbeHandler struct {
	store *core.ScenarioStore
	sleep func(time.Duration)
}

// NewErrorsProbeHandler wires the probe over the app's scenario store.
func NewErrorsProbeHandler(store *core.ScenarioStore) *ErrorsProbeHandler {
	return &ErrorsProbeHandler{store: store, sleep: time.Sleep}
}

// ServeHTTP implements http.Handler: one snapshot read decides the
// probe's behavior. The error path panics — deliberately, with the
// demo's sentinel — because a panic recovered by middleware is exactly
// the failure shape the topic teaches; the Recover layer upstream
// answers the 500 envelope so this handler never writes one itself.
func (h *ErrorsProbeHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	state := h.store.Snapshot()
	switch state.InjectFailure {
	case core.FailureError:
		panic(demos.ErrErrorsProbeInjected)
	case core.FailureLatency:
		h.sleep(demos.ErrorsProbeLatency)
	case core.FailureNone:
	}
	writeJSON(w, http.StatusOK, demos.BuildErrorsProbeAnswer(state))
}

// ErrorsExplorerHandler serves GET /api/demo/errors/explorer, the
// error-handling topic's demo payload: the taxonomy table (statuses
// and messages from core's own tables) plus a LIVE walk of the probe
// under the scenario's current injection mode. The probe runs behind
// the same middleware chain production serves, so the panic→recover→
// envelope conversion and the request-id thread the payload narrates
// are executions that just happened, not descriptions.
type ErrorsExplorerHandler struct {
	store  *core.ScenarioStore
	logger *slog.Logger
	probe  *ErrorsProbeHandler
	walk   http.Handler
	now    func() time.Time
}

// NewErrorsExplorerHandler wires the explorer over the app's scenario
// store and process logger, mounting the probe on a private mux behind
// the production middleware order for the per-request walk.
func NewErrorsExplorerHandler(store *core.ScenarioStore, logger *slog.Logger) *ErrorsExplorerHandler {
	probe := NewErrorsProbeHandler(store)
	probeMux := http.NewServeMux()
	probeMux.Handle("GET "+demos.ErrorsProbePath, probe)
	return &ErrorsExplorerHandler{
		store:  store,
		logger: logger,
		probe:  probe,
		walk:   Chain(RequestID, RequestLogger(logger), Recover(logger), EnvelopeMuxErrors(logger))(probeMux),
		now:    time.Now,
	}
}

// ServeHTTP implements http.Handler: walk the probe under the current
// scenario, then shape the payload from what it answered.
func (h *ErrorsExplorerHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	requestID := core.RequestIDFromContext(r.Context())
	started := h.now()

	probeReq, err := http.NewRequestWithContext(r.Context(), http.MethodGet, demos.ErrorsProbePath, nil)
	if err != nil {
		// Unreachable: a fixed method and path cannot fail to build.
		core.WriteError(w, r, core.Wrap(err, core.KindInternal,
			"The errors explorer could not build its probe request."), h.logger)
		return
	}
	// Thread the explorer's own request id into the walk as the INBOUND
	// header: the RequestID layer adopts it, the envelope stamps it, and
	// the payload can prove one id served the whole thread.
	probeReq.Header.Set(core.RequestIDHeader, requestID)

	recorder := newBufferedResponseRecorder()
	h.walk.ServeHTTP(recorder, probeReq)

	walk := demos.ErrorsWalk{
		InjectFailure: h.store.Snapshot().InjectFailure,
		Status:        recorder.status,
		Elapsed:       h.now().Sub(started),
		RequestID:     requestID,
		Body:          recorder.body.String(),
	}
	writeJSON(w, http.StatusOK, demos.BuildErrorsExplorerPayload(walk))
}

// bufferedResponseRecorder is the walk's private in-process writer: it
// captures the status and buffers the body a real connection would
// have received, with no socket and no dependency on net/http/httptest
// in production code. First-write-wins on the status, exactly like the
// real ResponseWriter contract; Flush is a no-op because nothing
// streams inside a walk.
type bufferedResponseRecorder struct {
	header http.Header
	body   bytes.Buffer
	status int
	wrote  bool
}

func newBufferedResponseRecorder() *bufferedResponseRecorder {
	return &bufferedResponseRecorder{header: make(http.Header)}
}

func (b *bufferedResponseRecorder) Header() http.Header { return b.header }

func (b *bufferedResponseRecorder) Write(p []byte) (int, error) {
	if !b.wrote {
		b.status = http.StatusOK
		b.wrote = true
	}
	return b.body.Write(p)
}

func (b *bufferedResponseRecorder) WriteHeader(code int) {
	if b.wrote {
		return
	}
	b.status = code
	b.wrote = true
}

func (b *bufferedResponseRecorder) Flush() {}
