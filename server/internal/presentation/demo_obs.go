// Demo API handlers for topic 33 observability. The VIEWER owns the
// replay: it builds the production middleware chain over the real
// posts handlers with a buffer-backed slog logger, walks a success and
// a 404 with one shared request id, and hands the captured JSON lines
// to the pure builder. The TRACE owns the walk: it accepts the
// request's OWN traceparent header (minting one when absent), runs the
// marker request through the production chain plus the trace
// middleware and the recorder-wired use case the composition root
// injected, and hands the recorded span tree to the pure builder.
// Neither handler owns a rule — executions in, facts out, builders
// shape.
package presentation

import (
	"bytes"
	"log/slog"
	"net/http"
	"strings"
	"sync"

	"guide/internal/core"
	"guide/internal/domain/demos"
	"guide/internal/domain/obs"
	"guide/internal/domain/posts"
)

// Compile-time proof both handlers satisfy http.Handler.
var (
	_ http.Handler = (*ObsViewerHandler)(nil)
	_ http.Handler = (*ObsTraceHandler)(nil)
)

// ObsViewerHandler serves GET /api/demo/obs/viewer, the slog JSON
// anatomy demo: a real request replayed through the production chain
// with every log line it produced captured verbatim. The use cases are
// injected (over a scenario-independent in-memory repository the
// composition root owns) so the replay's facts hold under every
// scenario mode.
type ObsViewerHandler struct {
	list *posts.ListPostsUseCase
	get  *posts.GetPostUseCase
}

// NewObsViewerHandler wires the viewer over the posts list and get use
// cases. No logger is injected: the replay builds its own buffer-backed
// one, and the handlers it mounts receive THAT logger so every line the
// walk can produce — access lines and the 404's warn alike — lands in
// the buffer the payload dissects.
func NewObsViewerHandler(list *posts.ListPostsUseCase, get *posts.GetPostUseCase) *ObsViewerHandler {
	return &ObsViewerHandler{list: list, get: get}
}

// ServeHTTP implements http.Handler: replay two requests through the
// real chain, then shape the payload from the captured lines.
func (h *ObsViewerHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	requestID := core.RequestIDFromContext(r.Context())

	// The replayed pipeline is the production shape: the real posts
	// handlers behind the documented middleware order — only the LOGGER
	// differs (buffer, debug level, so every line the walk can produce
	// is captured).
	var buffer bytes.Buffer
	replayLogger := slog.New(slog.NewJSONHandler(&buffer, &slog.HandlerOptions{Level: slog.LevelDebug}))
	mux := http.NewServeMux()
	mux.Handle("GET /api/posts", NewPostsListHandler(h.list, replayLogger))
	mux.Handle("GET /api/posts/{id}", NewPostDetailHandler(h.get, replayLogger))

	chain := Chain(
		RequestID,
		RequestLogger(replayLogger),
		Recover(replayLogger),
		EnvelopeMuxErrors(replayLogger),
	)(mux)

	// Two walks, one shared id: the 200 the happy path answers and the
	// 404 the error path answers — the 404's warn line and both access
	// lines will carry this id, which is the correlation the panel
	// teaches.
	statusOK := h.obsWalk(chain, requestID, "/api/posts?limit=2&sort=newest")
	statusMiss := h.obsWalk(chain, requestID, "/api/posts/obs-viewer-missing")

	facts := demos.ObsViewerFacts{
		AnatomyLine: "",
		ReplayRequests: []demos.ObsViewerReplayedRequest{
			{Label: "the 200", Request: "GET /api/posts?limit=2&sort=newest", RequestID: requestID, Status: statusOK},
			{Label: "the 404", Request: "GET /api/posts/obs-viewer-missing", RequestID: requestID, Status: statusMiss},
		},
	}
	for _, line := range strings.Split(strings.TrimSuffix(buffer.String(), "\n"), "\n") {
		if line == "" {
			continue
		}
		facts.ReplayLines = append(facts.ReplayLines, demos.ObsViewerCapturedLine{Source: replaySource(line), Raw: line})
		if facts.AnatomyLine == "" && strings.Contains(line, "\"http_request\"") {
			facts.AnatomyLine = line
		}
	}
	if facts.AnatomyLine == "" && len(facts.ReplayLines) > 0 {
		// The access lines are the anatomy's subject; if none appeared,
		// dissect the first captured line honestly instead of inventing one.
		facts.AnatomyLine = facts.ReplayLines[0].Raw
	}
	writeJSON(w, http.StatusOK, demos.BuildObsViewerPayload(facts))
}

// obsWalk fires one request through the chain and returns its status.
// The fixed method and path cannot fail to build; the error is honored
// with a typed 500 for the impossible case anyway.
func (h *ObsViewerHandler) obsWalk(chain http.Handler, requestID, target string) int {
	marker, err := http.NewRequest(http.MethodGet, target, nil)
	if err != nil {
		// Unreachable: a fixed method and path cannot fail to build.
		return http.StatusInternalServerError
	}
	marker.Header.Set(core.RequestIDHeader, requestID)
	recorder := newBufferedResponseRecorder()
	chain.ServeHTTP(recorder, marker)
	return recorder.status
}

// replaySource labels a captured line with the walk that produced it:
// access lines follow their own request; the warn a 404 writes arrives
// BEFORE that request's access line, so it is attributed to the 404 by
// its message.
func replaySource(line string) string {
	if strings.Contains(line, "\"error response\"") {
		return "the 404 (its failure line)"
	}
	if strings.Contains(line, "\"http_request\"") {
		if strings.Contains(line, "/api/posts/obs-viewer-missing") {
			return "the 404 (its access line)"
		}
		return "the 200 (its access line)"
	}
	return "the replay"
}

// ObsTraceHandler serves GET /api/demo/obs/trace, the W3C
// trace-context round-trip demo: the request's own traceparent (or a
// minted root) accepted, one posts request walked through the traced
// pipeline, the span tree rendered.
type ObsTraceHandler struct {
	list     *posts.ListPostsUseCase
	recorder *obs.SpanRecorder
	logger   *slog.Logger

	mu sync.Mutex
}

// NewObsTraceHandler wires the trace demo over the recorder-wrapped
// posts list use case (pre-wired by the composition root, like the
// architecture tracer before it) and the shared span recorder.
func NewObsTraceHandler(list *posts.ListPostsUseCase, recorder *obs.SpanRecorder, logger *slog.Logger) *ObsTraceHandler {
	return &ObsTraceHandler{list: list, recorder: recorder, logger: logger}
}

// ServeHTTP implements http.Handler: accept the inbound context, walk
// the marker request through the traced chain, shape the tree.
func (h *ObsTraceHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Accept the REQUEST's own traceparent — the round trip starts with
	// whatever the caller actually sent. Absent means mint; malformed
	// means reject-and-mint (the middleware's exact policy, applied here
	// so the payload can report which happened).
	inbound := strings.TrimSpace(r.Header.Get(obs.TraceparentHeader))
	inboundState := strings.TrimSpace(r.Header.Get(obs.TracestateHeader))
	accepted := obs.NewRoot()
	if inbound != "" {
		if parsed, err := obs.ParseTraceparent(inbound); err == nil {
			accepted = parsed
			if inboundState != "" {
				if members, stateErr := obs.ParseTracestate(inboundState); stateErr == nil {
					accepted.Tracestate = members
					inboundState = obs.FormatTracestate(members)
				} else {
					inboundState = ""
				}
			}
		}
	}

	h.recorder.Reset()

	// The traced pipeline: the real thin handler on a private mux, the
	// production middleware order, PLUS the trace middleware — the same
	// additive wrapping the composition root serves. The handler
	// boundary carries its own span, so the tree's presentation node is
	// recorded where the code ran.
	traced := http.NewServeMux()
	traced.Handle("GET /api/posts", obsSpanningHandler(h.recorder)(NewPostsListHandler(h.list, h.logger)))
	chain := Chain(
		RequestID,
		RequestLogger(h.logger),
		TraceContext(h.logger),
		Recover(h.logger),
		EnvelopeMuxErrors(h.logger),
	)(traced)

	marker, err := http.NewRequest(http.MethodGet, "/api/posts?limit=2&sort=newest", nil)
	if err != nil {
		// Unreachable: a fixed method and path cannot fail to build.
		core.WriteError(w, r, core.Wrap(err, core.KindInternal,
			"The trace demo could not build its marker request."), h.logger)
		return
	}
	marker.Header.Set(core.RequestIDHeader, core.RequestIDFromContext(r.Context()))
	// The walk's inbound header is the ACCEPTED context serialized — the
	// middleware inside the chain parses it again for real, which is the
	// round trip: one header, parsed twice, one tree.
	marker.Header.Set(obs.TraceparentHeader, accepted.String())

	// The root span runs under the walk's ambient context so its parent
	// is the inbound REMOTE span id — the graft point.
	walkCtx, root := h.recorder.Start(obs.WithTraceContext(r.Context(), accepted), "GET /api/posts (traced walk)", "middleware")
	recorder := newBufferedResponseRecorder()
	chain.ServeHTTP(recorder, marker.WithContext(walkCtx))
	root.End()

	facts := demos.ObsTraceFacts{
		InboundHeader: inbound,
		InboundState:  inboundState,
		Accepted:      accepted,
		Status:        recorder.status,
		Spans:         h.recorder.Spans(),
	}
	writeJSON(w, http.StatusOK, demos.BuildObsTracePayload(facts))
}

// obsSpanningHandler opens one span around a handler boundary — the
// presentation node of the tree, opened where the router hands over.
func obsSpanningHandler(recorder *obs.SpanRecorder) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, span := recorder.Start(r.Context(), "PostsListHandler.ServeHTTP", "presentation")
			next.ServeHTTP(w, r.WithContext(ctx))
			span.End()
		})
	}
}
