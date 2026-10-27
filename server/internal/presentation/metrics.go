package presentation

import (
	"log/slog"
	"net/http"
	"strings"
	"time"

	"guide/internal/domain/obs"
)

// requestMetricPath and its label names are the fixed, bounded
// vocabulary the HTTP metrics carry: method, the ROUTE PATTERN that
// matched (never the raw path — raw paths carry ids and hostile strings
// straight into the label cardinality), and the status CLASS (2xx…5xx,
// never the exact status — 404 and 422 are one story: the client).
const (
	requestsMetricName    = "guide_http_requests_total"
	requestsMetricHelp    = "HTTP requests processed by the guide server, labeled by method, matched route pattern, and status class."
	durationsMetricName   = "guide_http_request_duration_seconds"
	durationsMetricHelp   = "HTTP request durations in seconds, labeled by method and matched route pattern."
	unmatchedPatternLabel = "unmatched"
)

// RegisterHTTPMetrics declares the request counter and duration
// histogram on the registry — one call at the composition root, so the
// metric families exist (typed, helped) even before the first request.
func RegisterHTTPMetrics(registry *obs.MetricsRegistry) (requests *obs.Counter, durations *obs.Histogram) {
	requests = registry.MustCounter(requestsMetricName, requestsMetricHelp,
		[]string{"method", "path", "status"})
	durations = registry.MustHistogramSeconds(durationsMetricName, durationsMetricHelp,
		[]string{"method", "path"}, obs.DefaultDurationBuckets())
	return requests, durations
}

// MetricsMiddleware instruments every request that flows through it into
// the two registered families: a counter by (method, route pattern,
// status class) and a duration histogram by (method, route pattern).
// The mux is injected so the middleware can ask the ROUTER what pattern
// the request will match — the bounded-label discipline in one move:
// /api/posts/p-1 and /api/posts/p-99 are both the pattern
// "/api/posts/{id}", so ten thousand ids cost zero new series, and an
// unroutable path lands on the single "unmatched" label instead of a
// fresh one per probe. Durations cover everything beneath this layer,
// exactly like the access log's duration_ms.
func MetricsMiddleware(
	requests *obs.Counter,
	durations *obs.Histogram,
	mux *http.ServeMux,
) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rec := newStatusRecorder(w)
			started := time.Now()
			next.ServeHTTP(rec, r)

			pattern := routePatternFor(mux, r)
			durations.Observe(time.Since(started).Seconds(), r.Method, pattern)
			requests.Inc(r.Method, pattern, statusClass(rec.status))
		})
	}
}

// routePatternFor asks the mux which registered pattern the request
// matches — the same match the router is about to make when the request
// reaches it. A method-qualified pattern ("GET /api/posts/{id}")
// renders as its path template (the method already has its own label);
// requests the router will not serve as-is — no match, or a method
// mismatch, which the mux's own lookup reports without a pattern —
// take the single "unmatched" label. Either way the label vocabulary is
// the route table plus one, never the raw path.
func routePatternFor(mux *http.ServeMux, r *http.Request) string {
	_, pattern := mux.Handler(r)
	if pattern == "" {
		return unmatchedPatternLabel
	}
	if _, path, found := strings.Cut(pattern, " "); found {
		return path
	}
	return pattern
}

// statusClass buckets the exact status into the 2xx…5xx story a dashboard
// wants; 1xx and the zero default (nothing written — a hijacked or
// aborted connection) fall to "other" rather than inventing a class.
func statusClass(status int) string {
	switch {
	case status >= 200 && status < 300:
		return "2xx"
	case status >= 300 && status < 400:
		return "3xx"
	case status >= 400 && status < 500:
		return "4xx"
	case status >= 500 && status < 600:
		return "5xx"
	default:
		return "other"
	}
}

// MetricsHandler serves GET /metrics: the registry rendered live in the
// Prometheus text exposition format. The content type is the format's
// registered one, so scrapers and `curl` alike read it unambiguous.
type MetricsHandler struct {
	registry *obs.MetricsRegistry
	logger   *slog.Logger
}

// NewMetricsHandler wires the handler over the live registry.
func NewMetricsHandler(registry *obs.MetricsRegistry, logger *slog.Logger) *MetricsHandler {
	return &MetricsHandler{registry: registry, logger: logger}
}

// ServeHTTP implements http.Handler: stream the exposition text.
func (h *MetricsHandler) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	if err := h.registry.Write(w); err != nil {
		// A write failure means the scraper vanished mid-scrape; the
		// metric families are intact and the next scrape will read them.
		h.logger.Warn("metrics scrape write failed", "error", err)
	}
}

// Compile-time proof the handler satisfies http.Handler.
var _ http.Handler = (*MetricsHandler)(nil)
