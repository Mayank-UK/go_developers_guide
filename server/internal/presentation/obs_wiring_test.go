package presentation

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"guide/internal/core"
	"guide/internal/domain/obs"
)

// The observability edge's own tests: the trace middleware's three
// inbound cases, the metrics middleware's bounded labels (with the
// hostile-raw-path proof), the /metrics handler's exposition, the
// readiness endpoint's honest 503, and pprof's dedicated mount.

// obsTestPinger is the readiness stub: answers after delay with err.
type obsTestPinger struct {
	delay time.Duration
	err   error
}

func (p obsTestPinger) Ping(ctx context.Context) error {
	select {
	case <-time.After(p.delay):
		return p.err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func TestTraceContextAdoptsValidInboundHeaders(t *testing.T) {
	logger := newTestLogger(&strings.Builder{})
	var seen obs.TraceContext
	var headerEcho string
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var ok bool
		seen, ok = obs.TraceContextFromContext(r.Context())
		if !ok {
			t.Error("inner handler must see the trace context")
		}
		headerEcho = w.Header().Get(obs.TraceparentHeader)
	})
	server := TraceContext(logger)(inner)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	req.Header.Set(obs.TraceparentHeader, "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01")
	server.ServeHTTP(rec, req)

	if seen.TraceID != "4bf92f3577b34da6a3ce929d0e0e4736" || seen.ParentID != "00f067aa0ba902b7" {
		t.Errorf("inbound context not adopted: %+v", seen)
	}
	// The response echo must keep the trace id and mint a fresh span id.
	parts := strings.Split(headerEcho, "-")
	if len(parts) != 4 || parts[1] != "4bf92f3577b34da6a3ce929d0e0e4736" {
		t.Errorf("response echo = %q, want the same trace id", headerEcho)
	}
	if len(parts) == 4 && (parts[2] == "00f067aa0ba902b7" || len(parts[2]) != 16) {
		t.Errorf("response echo parent = %q, want a fresh 16-hex span id", parts[2])
	}
	if rec.Header().Get(obs.TraceparentHeader) == "" {
		t.Error("the echo must be on the wire, not only the writer")
	}
}

func TestTraceContextRejectsMalformedAndMintsRoots(t *testing.T) {
	logger := newTestLogger(&strings.Builder{})
	var innerTraceID, innerParent string
	inner := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		if tc, ok := obs.TraceContextFromContext(r.Context()); ok {
			innerTraceID, innerParent = tc.TraceID, tc.ParentID
		}
	})
	server := TraceContext(logger)(inner)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	req.Header.Set(obs.TraceparentHeader, "00-00000000000000000000000000000000-00f067aa0ba902b7-01")
	server.ServeHTTP(rec, req)

	if innerTraceID == "00000000000000000000000000000000" || innerTraceID == "" {
		t.Errorf("malformed header must NOT be adopted; trace id = %q", innerTraceID)
	}
	if innerParent == "00f067aa0ba902b7" {
		t.Errorf("fresh root must not inherit the hostile parent; got %q", innerParent)
	}
	// A request with no header at all: the server is the root.
	rec2 := httptest.NewRecorder()
	server.ServeHTTP(rec2, httptest.NewRequest(http.MethodGet, "/api/health", nil))
	if innerTraceID == "" {
		t.Error("untraced requests must still run under a minted root")
	}
}

func newObsMetricsServer(t *testing.T) (http.Handler, *obs.MetricsRegistry, *obs.Counter, *obs.Histogram) {
	t.Helper()
	registry := obs.NewMetricsRegistry()
	requests, durations := RegisterHTTPMetrics(registry)
	mux := http.NewServeMux()
	mux.Handle("GET /api/posts/{id}", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"ok": "yes"})
	}))
	mux.Handle("GET /api/health", NewHealthHandler("test"))
	handler := Chain(RequestID, RequestLogger(newTestLogger(&strings.Builder{})))(mux)
	wrapped := MetricsMiddleware(requests, durations, mux)(handler)
	return wrapped, registry, requests, durations
}

// TestMetricsMiddlewareBoundsLabelCardinality is the hostile-path
// proof: thousands of distinct raw paths (and unroutable probes) must
// collapse onto the route PATTERN (or the single "unmatched" label) —
// the series count stays bounded by the route table.
func TestMetricsMiddlewareBoundsLabelCardinality(t *testing.T) {
	server, _, requests, _ := newObsMetricsServer(t)
	paths := []string{"/api/posts/p-1", "/api/posts/p-2", "/api/posts/hostile-a=b", "/api/posts/a,b", "/api/posts/%20%20"}
	for i := 0; i < 50; i++ {
		paths = append(paths, "/api/posts/raw-"+strings.Repeat("x", i+1))
	}
	for _, path := range paths {
		server.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, path, nil))
	}
	for i := 0; i < 5; i++ {
		server.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/nowhere-"+string(rune('a'+i)), nil))
	}
	if got := requests.Value("GET", "/api/posts/{id}", "2xx"); got != uint64(len(paths)) {
		t.Errorf("raw paths must collapse onto the pattern: %d hits counted, want %d", got, len(paths))
	}
	if got := requests.Value("GET", "unmatched", "4xx"); got != 5 {
		t.Errorf("unroutable probes must collapse onto unmatched: %d", got)
	}
}

func TestMetricsMiddlewareCountsStatusClassesAndDurations(t *testing.T) {
	server, _, requests, durations := newObsMetricsServer(t)
	server.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/health", nil))
	server.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/health", nil))
	server.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/api/health", nil))

	if got := requests.Value("GET", "/api/health", "2xx"); got != 2 {
		t.Errorf("GET 2xx count = %d, want 2", got)
	}
	// A method mismatch answers 405 and the mux's own Handler() lookup
	// exposes no pattern for it — the router refused the request, so the
	// honest bounded label is "unmatched" (class 4xx), never a raw path.
	if got := requests.Value("POST", "unmatched", "4xx"); got != 1 {
		t.Errorf("POST 405 must class as 4xx under unmatched: %d", got)
	}
	// The histogram observed both GETs with the pattern label.
	if got := durations.ObservedCount("GET", "/api/health"); got != 2 {
		t.Errorf("duration observations = %d, want 2", got)
	}
}

func TestMetricsHandlerServesTheTextExpositionFormat(t *testing.T) {
	registry := obs.NewMetricsRegistry()
	requests, _ := RegisterHTTPMetrics(registry)
	requests.Inc("GET", "/api/content", "2xx")

	rec := doRequest(NewMetricsHandler(registry, newTestLogger(&strings.Builder{})), http.MethodGet, "/metrics", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/plain; version=0.0.4") {
		t.Errorf("content type = %q", ct)
	}
	body := rec.Body.String()
	for _, want := range []string{
		"# HELP guide_http_requests_total",
		"# TYPE guide_http_requests_total counter",
		`guide_http_requests_total{method="GET",path="/api/content",status="2xx"} 1`,
		"# TYPE guide_http_request_duration_seconds histogram",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("exposition missing %q:\n%s", want, body)
		}
	}
}

func TestReadinessServesReadyAndNotReadyHonestly(t *testing.T) {
	logger := newTestLogger(&strings.Builder{})

	up := NewReadinessHandler([]obs.DependencyCheck{
		{Name: "postgres", Pinger: obsTestPinger{delay: time.Millisecond}},
		{Name: "redis", Pinger: obsTestPinger{delay: time.Millisecond}},
	}, logger)
	rec := doRequest(up, http.MethodGet, "/api/ready", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("ready status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var ready struct {
		Status string `json:"status"`
		Checks []struct {
			Name string `json:"name"`
			OK   bool   `json:"ok"`
		} `json:"checks"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &ready); err != nil {
		t.Fatalf("ready body: %v", err)
	}
	if ready.Status != "ready" || len(ready.Checks) != 2 || !ready.Checks[0].OK || !ready.Checks[1].OK {
		t.Errorf("ready body drifted: %+v", ready)
	}

	cause := errors.New("connection refused")
	down := NewReadinessHandler([]obs.DependencyCheck{
		{Name: "postgres", Pinger: obsTestPinger{delay: time.Millisecond}},
		{Name: "redis", Pinger: obsTestPinger{err: cause, delay: time.Millisecond}},
	}, logger)
	// The production chain shape: request-id outermost, so the 503 rides
	// the standard envelope complete with its x-request-id.
	downChain := Chain(RequestID, RequestLogger(logger))(down)
	recDown := doRequest(downChain, http.MethodGet, "/api/ready", "")
	if recDown.Code != http.StatusServiceUnavailable {
		t.Fatalf("not-ready status = %d, want 503", recDown.Code)
	}
	var envelope struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
			Details struct {
				Checks []struct {
					Name  string `json:"name"`
					OK    bool   `json:"ok"`
					Error string `json:"error"`
				} `json:"checks"`
			} `json:"details"`
		} `json:"error"`
	}
	if err := json.Unmarshal(recDown.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("not-ready body: %v\n%s", err, recDown.Body.String())
	}
	if envelope.Error.Code != "service_unavailable" {
		t.Errorf("code = %q", envelope.Error.Code)
	}
	if !strings.Contains(envelope.Error.Message, "redis") {
		t.Errorf("message must name the down dependency: %q", envelope.Error.Message)
	}
	if len(envelope.Error.Details.Checks) != 2 {
		t.Fatalf("details must carry every row: %+v", envelope.Error.Details)
	}
	for _, check := range envelope.Error.Details.Checks {
		if check.Name == "redis" && (check.OK || !strings.Contains(check.Error, "connection refused")) {
			t.Errorf("redis row must name the cause: %+v", check)
		}
		if check.Name == "postgres" && !check.OK {
			t.Errorf("postgres row must still report up: %+v", check)
		}
	}
	if recDown.Header().Get(core.RequestIDHeader) == "" {
		t.Error("the 503 must ride the standard envelope incl. x-request-id")
	}
}

func TestPprofMountsUnderTheInternalPrefix(t *testing.T) {
	mux := http.NewServeMux()
	mux.Handle("/internal/", NewInternalHandler())
	mux.Handle("GET /api/health", NewHealthHandler("test"))
	server := Chain(RequestID, RequestLogger(newTestLogger(&strings.Builder{})))(mux)

	rec := doRequest(server, http.MethodGet, "/internal/debug/pprof/", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("pprof index status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "goroutine") {
		t.Errorf("pprof index must list the profiles: %s", rec.Body.String())
	}
	// The heap profile is a real binary payload under the same prefix.
	heap := doRequest(server, http.MethodGet, "/internal/debug/pprof/heap", "")
	if heap.Code != http.StatusOK || heap.Body.Len() == 0 {
		t.Errorf("heap profile = %d bytes, status %d", heap.Body.Len(), heap.Code)
	}
	// And the mount is NOT on the public mux root: the default-mux path
	// the package would have registered must not exist here.
	stray := doRequest(server, http.MethodGet, "/debug/pprof/", "")
	if stray.Code != http.StatusNotFound {
		t.Errorf("/debug/pprof/ status = %d, want the 404 envelope (dedicated path only)", stray.Code)
	}
}
