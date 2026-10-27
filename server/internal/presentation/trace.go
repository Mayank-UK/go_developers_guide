package presentation

import (
	"log/slog"
	"net/http"
	"strings"

	"guide/internal/core"
	"guide/internal/domain/obs"
)

// TraceContext is the W3C trace-context propagation middleware — the
// additive observability edge, composed OUTSIDE the existing chain the
// way the composition root wraps the finished handler: it never edits a
// line of the request-id/recovery stack, it wraps them. One inbound
// header decides the request's trace: a VALID traceparent is adopted
// (context + outbound echo), a MALFORMED one is rejected honestly (a
// debug log naming the reason) and the request proceeds under a fresh
// root — a broken header from a caller is never that caller's request
// failed — and NO header means the server is the trace's root. The
// outbound traceparent carries the server's own span id, so the caller
// can graft the response onto the exact work it caused. The companion
// tracestate list rides along validated whenever present.
func TraceContext(logger *slog.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tc := obs.NewRoot()
			inbound := strings.TrimSpace(r.Header.Get(obs.TraceparentHeader))
			if inbound != "" {
				parsed, err := obs.ParseTraceparent(inbound)
				if err != nil {
					// Honest degradation, visible at the debug dial: the
					// header is named, the reason is named, the request
					// lives under a fresh root.
					logger.Debug("traceparent rejected; starting a fresh root",
						"header", inbound,
						"reason", err.Error(),
						"request_id", requestIDOf(r),
					)
				} else {
					if state := strings.TrimSpace(r.Header.Get(obs.TracestateHeader)); state != "" {
						if members, stateErr := obs.ParseTracestate(state); stateErr == nil {
							parsed.Tracestate = members
						} else {
							logger.Debug("tracestate rejected; propagating without it",
								"reason", stateErr.Error(),
								"request_id", requestIDOf(r),
							)
						}
					}
					tc = parsed
				}
			}
			// The response carries the server span's context: same trace,
			// fresh span id — set before the inner stack runs so even a
			// mid-response failure keeps it, exactly like the request id.
			w.Header().Set(obs.TraceparentHeader, tc.Child().String())
			if len(tc.Tracestate) > 0 {
				w.Header().Set(obs.TracestateHeader, obs.FormatTracestate(tc.Tracestate))
			}
			next.ServeHTTP(w, r.WithContext(obs.WithTraceContext(r.Context(), tc)))
		})
	}
}

// requestIDOf is a nil-safe read of the ambient request id for the
// middleware's debug lines (the trace middleware sits OUTSIDE the
// request-id layer in some wirings, so the id may not exist yet).
func requestIDOf(r *http.Request) string {
	return core.RequestIDFromContext(r.Context())
}
