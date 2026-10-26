package presentation

import (
	"bytes"
	"log/slog"
	"math"
	"net/http"
	"runtime/debug"
	"slices"
	"strings"
	"time"

	"guide/internal/core"
)

// Middleware wraps one handler with cross-cutting behavior.
type Middleware func(http.Handler) http.Handler

// Chain composes middleware so the FIRST argument is the OUTERMOST layer:
// Chain(A, B, C)(handler) serves as A(B(C(handler))). The composition root
// wires the documented order:
//
//	RequestID(RequestLogger(Recover(EnvelopeMuxErrors(mux))))
//
// Request-id outermost so the logging and recovery layers (and every error
// envelope they write) can stamp and log the same id. The request logger
// sits OUTSIDE recovery so a recovered panic is still logged as a finished
// request with its true 500 status. Recovery converts panics to the 500
// envelope via the central writer; the mux-error wrapper innermost turns
// the stdlib ServeMux's plain-text 404/405 replies into the same envelope.
func Chain(middleware ...Middleware) Middleware {
	return func(final http.Handler) http.Handler {
		for i := len(middleware) - 1; i >= 0; i-- {
			final = middleware[i](final)
		}
		return final
	}
}

// RequestID assigns every request exactly one id: the inbound
// x-request-id header when the client supplies one (trimmed, capped at
// MaxInboundRequestIDLength so hostile headers cannot bloat logs), else a
// freshly minted one. The id is echoed on the response header, propagated
// through the request context for handlers, logging, and error envelopes,
// and set on the response before the inner stack runs so even a
// mid-response failure keeps it.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := strings.TrimSpace(r.Header.Get(core.RequestIDHeader))
		if id == "" {
			id = core.NewRequestID()
		}
		if len(id) > core.MaxInboundRequestIDLength {
			id = id[:core.MaxInboundRequestIDLength]
		}
		w.Header().Set(core.RequestIDHeader, id)
		next.ServeHTTP(w, r.WithContext(core.WithRequestID(r.Context(), id)))
	})
}

// statusRecorder captures the response status for the access log. It
// forwards Flush and Unwrap so http.ResponseController (and classic
// type-assertion flushing, e.g. for later SSE demos) keeps working through
// the chain.
type statusRecorder struct {
	http.ResponseWriter
	status int
	wrote  bool
}

func newStatusRecorder(w http.ResponseWriter) *statusRecorder {
	return &statusRecorder{ResponseWriter: w, status: http.StatusOK}
}

func (r *statusRecorder) WriteHeader(code int) {
	if !r.wrote {
		r.status = code
		r.wrote = true
	}
	r.ResponseWriter.WriteHeader(code)
}

func (r *statusRecorder) Write(b []byte) (int, error) {
	r.wrote = true
	return r.ResponseWriter.Write(b)
}

// Written reports whether anything (headers or body) has gone out yet —
// the recovery layer uses it to know whether an envelope is still
// deliverable.
func (r *statusRecorder) Written() bool { return r.wrote }

func (r *statusRecorder) Flush() {
	if flusher, ok := r.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

// Unwrap supports http.ResponseController.
func (r *statusRecorder) Unwrap() http.ResponseWriter { return r.ResponseWriter }

// RequestLogger writes one structured "http_request" line per request at
// completion: method, path, status, duration_ms, request_id. The log field
// names mirror the fastapi guide's access log so the family's
// observability tooling reads both identically; duration covers everything
// beneath this layer (inner middleware, routing, handler, serialization).
func RequestLogger(logger *slog.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rec := newStatusRecorder(w)
			started := time.Now()
			defer func() {
				durationMs := float64(time.Since(started).Microseconds()) / 1000
				logger.Info("http_request",
					"method", r.Method,
					"path", r.URL.Path,
					"status", rec.status,
					"duration_ms", math.Round(durationMs*10)/10,
					"request_id", core.RequestIDFromContext(r.Context()),
				)
			}()
			next.ServeHTTP(rec, r)
		})
	}
}

// Recover converts panics into the 500 internal error envelope via the
// central writer and logs them at error with the stack — panics are for
// programmer bugs only, never control flow. http.ErrAbortHandler is
// re-raised untouched: it is net/http's own sentinel for "the connection
// is gone", where writing an envelope is impossible and wrong. If the
// response has already started, no envelope can be delivered; the panic is
// still logged with the stack so nothing disappears.
func Recover(logger *slog.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if recovered := recover(); recovered != nil {
					if recovered == http.ErrAbortHandler {
						panic(recovered)
					}
					requestID := core.RequestIDFromContext(r.Context())
					logger.Error("panic recovered",
						"request_id", requestID,
						"method", r.Method,
						"path", r.URL.Path,
						"panic", recovered,
						"stack", string(debug.Stack()),
					)
					if writer, ok := w.(interface{ Written() bool }); ok && writer.Written() {
						return
					}
					core.WriteError(w, r, core.Internal("Internal server error."), logger)
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

// muxCannedBodies lists the stdlib ServeMux's canned error bodies per
// status — the only strings it writes on routing failures. They are
// matched exactly (including the trailing newline) so genuine
// handler-written 404/405 bodies are never mistaken for them; both the
// older "405 method not allowed" and Go 1.27's "Method Not Allowed"
// wordings are listed, and a future wording change degrades to a
// pass-through rather than eating a real body.
var muxCannedBodies = map[int][]string{
	http.StatusNotFound:         {"404 page not found\n"},
	http.StatusMethodNotAllowed: {"405 method not allowed\n", "Method Not Allowed\n"},
}

// EnvelopeMuxErrors wraps the router and converts net/http's plain-text
// routing failures into the family error envelope, so EVERY error response
// the client can receive has the same shape. The mapping mirrors the
// fastapi guide: an unmatched path is a 404 not_found envelope
// ("Not Found"); a method mismatch keeps its truthful 405 status and Allow
// header but takes the closest taxonomy kind, not_found ("Method Not
// Allowed"). Normal responses stream through untouched — only 404/405
// responses are buffered for inspection, and a 404/405 whose body is not
// the canned mux text (e.g. the content handler's own 404 envelope) is
// replayed verbatim.
func EnvelopeMuxErrors(logger *slog.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			interceptor := &muxErrorInterceptor{w: w}
			next.ServeHTTP(interceptor, r)
			interceptor.finish(r, logger)
		})
	}
}

// muxErrorInterceptor passes everything through until a 404/405 status is
// declared; from then on it buffers the body so finish() can decide
// whether to replace the canned reply with the envelope or replay the
// handler's own bytes. Response headers ride the real writer's header map
// either way (Set on that map before WriteHeader reaches the wire), which
// is what preserves the mux's Allow header on 405s.
type muxErrorInterceptor struct {
	w         http.ResponseWriter
	buffering bool
	decided   bool
	status    int
	buf       bytes.Buffer
}

func (i *muxErrorInterceptor) Header() http.Header { return i.w.Header() }

func (i *muxErrorInterceptor) WriteHeader(code int) {
	if i.decided {
		// A second WriteHeader is superfluous by net/http's own contract;
		// drop it rather than relay the warning.
		return
	}
	i.decided = true
	if code == http.StatusNotFound || code == http.StatusMethodNotAllowed {
		i.buffering = true
		i.status = code
		return
	}
	i.w.WriteHeader(code)
}

func (i *muxErrorInterceptor) Write(b []byte) (int, error) {
	if !i.decided {
		i.WriteHeader(http.StatusOK)
	}
	if i.buffering {
		return i.buf.Write(b)
	}
	return i.w.Write(b)
}

func (i *muxErrorInterceptor) Flush() {
	if !i.buffering {
		if flusher, ok := i.w.(http.Flusher); ok {
			flusher.Flush()
		}
	}
}

// Unwrap supports http.ResponseController.
func (i *muxErrorInterceptor) Unwrap() http.ResponseWriter { return i.w }

// finish runs after the wrapped handler returns: buffered 404/405 bodies
// that are the mux's canned strings become the envelope; anything else is
// replayed with its original status.
func (i *muxErrorInterceptor) finish(r *http.Request, logger *slog.Logger) {
	if !i.buffering {
		return
	}
	canned := slices.Contains(muxCannedBodies[i.status], i.buf.String())
	if !canned {
		i.w.WriteHeader(i.status)
		_, _ = i.w.Write(i.buf.Bytes())
		return
	}
	// The headers on the real writer (Content-Type, Allow) stay; the
	// writer overwrites Content-Type and stamps the envelope body. The 405
	// keeps its truthful status and takes the closest kind bucket.
	if i.status == http.StatusNotFound {
		core.WriteError(i.w, r, core.NotFound("Not Found"), logger)
		return
	}
	core.WriteErrorStatus(i.w, r, i.status, core.New(core.KindForHTTPStatus(i.status), "Method Not Allowed"), logger)
}
