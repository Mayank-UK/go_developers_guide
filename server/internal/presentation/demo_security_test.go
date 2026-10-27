package presentation

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"golang.org/x/time/rate"

	"guide/internal/core"
	domauth "guide/internal/domain/auth"
	"guide/internal/domain/demos"
	domsecurity "guide/internal/domain/security"
)

// The topic 25 endpoints and middleware, exercised through the same
// middleware chain shape production runs behind. The story runners are
// faked here — the REAL console stories have their own suite in
// internal/data/security — and the two middlewares are tested against
// real limiters and the real header policy, so the 429 envelope shape
// and the header set are pinned without any store.

// ── fakes ───────────────────────────────────────────────────────────────

type fakeSecurityLabRunner struct {
	err error
}

func (f fakeSecurityLabRunner) RunSecurityLabStory(context.Context) (demos.SecurityLabRun, error) {
	return demos.SecurityLabRun{
		Headers: []demos.SecurityHeaderRow{{Header: "X-Frame-Options", Value: "DENY", Why: "why"}},
		Policy:  "10/s sustained, burst 5",
		Requests: []demos.SecurityRateRequestRow{
			{N: 1, Allowed: true}, {N: 6, Allowed: false, RetryAfter: "90ms"},
		},
		AllowedCount: 5, DeniedCount: 3, FirstRetryAfter: "90ms",
		RecoveryWaitMs: "95.0 ms", SteadyWaitMs: "99.0 ms", WaitServed: true,
		OtherClientAllowed: true, Buckets: 2, RoutePolicy: "2/s sustained, burst 5",
	}, f.err
}

type fakeSecurityPIIRunner struct {
	err error
}

func (f fakeSecurityPIIRunner) RunSecurityPIIStory(context.Context) (demos.SecurityPIIRun, error) {
	return demos.SecurityPIIRun{
		SubjectID: "sub_ada",
		Classification: []demos.SecurityClassifiedRow{
			{Name: "email", Sensitivity: "pii", Value: "ada@example.com"},
		},
		MaskedView: []demos.SecurityClassifiedRow{
			{Name: "email", Sensitivity: "pii", Value: "a***********m"},
		},
		LogBefore:    []demos.SecurityLogRow{{Line: "subject=sub_ada action=read"}},
		ErasedFields: []string{"email"},
		ViewAfterErase: []demos.SecurityClassifiedRow{
			{Name: "email", Sensitivity: "pii", Value: "[erased per subject request]"},
		},
		LogAfter:            []demos.SecurityLogRow{{Line: "subject=sub_ada action=erase"}},
		PIIAbsentEverywhere: true,
	}, f.err
}

// ── the two story endpoints ─────────────────────────────────────────────

func newSecurityDemoServer(t *testing.T, lab demos.SecurityLabRunner, pii demos.SecurityPIIRunner) http.Handler {
	t.Helper()
	logger := newTestLogger(&strings.Builder{})
	mux := http.NewServeMux()
	mux.Handle("GET /api/demo/security/lab", NewSecurityLabHandler(lab, logger))
	mux.Handle("GET /api/demo/security/pii", NewSecurityPIIHandler(pii, logger))
	return Chain(RequestID, RequestLogger(logger), Recover(logger), EnvelopeMuxErrors(logger))(mux)
}

func TestSecurityStoryEndpointsServeShapedPayloads(t *testing.T) {
	server := newSecurityDemoServer(t, fakeSecurityLabRunner{}, fakeSecurityPIIRunner{})
	for _, tc := range []struct {
		path      string
		mechanism string
		beats     int
	}{
		{"/api/demo/security/lab", "security-lab", 6},
		{"/api/demo/security/pii", "security-pii", 6},
	} {
		rec := doRequest(server, http.MethodGet, tc.path, "")
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: status = %d: %s", tc.path, rec.Code, rec.Body.String())
		}
		decoded := struct {
			Mechanism string `json:"mechanism"`
			Beats     []struct {
				Beat    string `json:"beat"`
				Verdict string `json:"verdict"`
				Facts   []any  `json:"facts"`
			} `json:"beats"`
			Proofs    []any  `json:"proofs"`
			Reasoning string `json:"reasoning"`
		}{}
		if err := json.Unmarshal(rec.Body.Bytes(), &decoded); err != nil {
			t.Fatalf("%s: body not the story DTO shape: %v", tc.path, err)
		}
		if decoded.Mechanism != tc.mechanism {
			t.Errorf("%s: mechanism = %q", tc.path, decoded.Mechanism)
		}
		if len(decoded.Beats) != tc.beats {
			t.Errorf("%s: beats = %d, want %d", tc.path, len(decoded.Beats), tc.beats)
		}
		if decoded.Beats[0].Facts == nil {
			t.Errorf("%s: first beat facts marshaled null", tc.path)
		}
		if len(decoded.Proofs) == 0 || decoded.Reasoning == "" {
			t.Errorf("%s: payload lacks proofs/reasoning", tc.path)
		}
	}
}

func TestSecurityStoryEndpointMapsRunnerFailureToInternal(t *testing.T) {
	server := newSecurityDemoServer(t, fakeSecurityLabRunner{err: errors.New("boom")}, fakeSecurityPIIRunner{})
	rec := doRequest(server, http.MethodGet, "/api/demo/security/lab", "")
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	var envelope struct {
		Error struct {
			Code      string `json:"code"`
			RequestID string `json:"requestId"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("envelope: %v", err)
	}
	if envelope.Error.Code != string(core.KindInternal) || envelope.Error.RequestID == "" {
		t.Errorf("envelope = %+v", envelope.Error)
	}
}

// ── the security-headers middleware ─────────────────────────────────────

func TestSecurityHeadersMiddlewareStampsThePolicyOnEveryResponse(t *testing.T) {
	logger := newTestLogger(&strings.Builder{})
	failing := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		core.WriteError(w, r, core.NotFound("nope"), logger)
	})
	wrapped := SecurityHeaders(domsecurity.APISecurityHeaders)(failing)
	rec := httptest.NewRecorder()
	wrapped.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/demo/security/lab", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want the handler's own 404", rec.Code)
	}
	for _, rule := range domsecurity.APISecurityHeaders {
		if got := rec.Header().Get(rule.Header); got != rule.Value {
			t.Errorf("%s = %q, want %q even on the error path", rule.Header, got, rule.Value)
		}
	}
}

func TestSecurityHeadersMiddlewareIsIdempotentWhenTwiceApplied(t *testing.T) {
	ok := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	wrapped := SecurityHeaders(domsecurity.APISecurityHeaders)(SecurityHeaders(domsecurity.APISecurityHeaders)(ok))
	rec := httptest.NewRecorder()
	wrapped.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if values := rec.Header().Values("Content-Security-Policy"); len(values) != 1 {
		t.Errorf("double-wrapped CSP = %v, want exactly one value", values)
	}
}

// ── the rate-limit middleware ───────────────────────────────────────────

func newRateLimitedTarget(t *testing.T, policy domsecurity.RatePolicy) (http.Handler, *domsecurity.ClientRateLimiter) {
	t.Helper()
	logger := newTestLogger(&strings.Builder{})
	limiter, err := domsecurity.NewClientRateLimiter(policy)
	if err != nil {
		t.Fatalf("limiter: %v", err)
	}
	target := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	return Chain(RequestID, RateLimit(limiter, logger))(target), limiter
}

func TestRateLimitMiddlewareRefusesWithTheTypedEnvelope(t *testing.T) {
	server, _ := newRateLimitedTarget(t, domsecurity.RatePolicy{Rate: rate.Limit(1), Burst: 2})
	var last *httptest.ResponseRecorder
	for i := 0; i < 3; i++ {
		req := httptest.NewRequest(http.MethodGet, "/api/demo/security/lab", nil)
		req.RemoteAddr = "192.0.2.10:4444"
		last = httptest.NewRecorder()
		server.ServeHTTP(last, req)
	}
	if last.Code != http.StatusTooManyRequests {
		t.Fatalf("third request status = %d, want 429", last.Code)
	}
	var envelope struct {
		Error struct {
			Code      string         `json:"code"`
			Message   string         `json:"message"`
			RequestID string         `json:"requestId"`
			Details   map[string]any `json:"details"`
		} `json:"error"`
	}
	if err := json.Unmarshal(last.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("envelope: %v", err)
	}
	if envelope.Error.Code != string(core.KindRateLimited) {
		t.Errorf("code = %q, want rate_limited", envelope.Error.Code)
	}
	if envelope.Error.RequestID == "" {
		t.Error("the 429 must carry the request id")
	}
	if last.Header().Get(core.RequestIDHeader) == "" {
		t.Error("the 429 must stamp the x-request-id response header")
	}
	if retry := last.Header().Get(RetryAfterHeader); retry == "" || retry == "0" {
		t.Errorf("Retry-After = %q, want a positive ceil-seconds hint", retry)
	}
	if ms, ok := envelope.Error.Details["retryAfterMs"].(float64); !ok || ms <= 0 {
		t.Errorf("details.retryAfterMs = %v, want a positive number", envelope.Error.Details["retryAfterMs"])
	}
	if strings.Contains(envelope.Error.Message, "192.0.2.10") {
		t.Errorf("the refusal must not echo the client key back: %s", envelope.Error.Message)
	}
}

func TestRateLimitMiddlewareKeysPerClient(t *testing.T) {
	server, _ := newRateLimitedTarget(t, domsecurity.RatePolicy{Rate: rate.Limit(1), Burst: 1})
	get := func(remote string) int {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = remote
		rec := httptest.NewRecorder()
		server.ServeHTTP(rec, req)
		return rec.Code
	}
	if get("192.0.2.10:1") != http.StatusOK || get("192.0.2.10:2") != http.StatusTooManyRequests {
		t.Error("one address must spend its own bucket (200 then 429)")
	}
	if get("198.51.100.7:3") != http.StatusOK {
		t.Error("a different address must have its own bucket")
	}
}

func TestRateLimitMiddlewareKeysIdentityOverAddress(t *testing.T) {
	logger := newTestLogger(&strings.Builder{})
	limiter, err := domsecurity.NewClientRateLimiter(domsecurity.RatePolicy{Rate: rate.Limit(1), Burst: 1})
	if err != nil {
		t.Fatalf("limiter: %v", err)
	}
	// Chain order under test: identity resolved by an OUTER layer, the
	// rate limiter inside it — so both requests are one user.
	inner := RateLimit(limiter, logger)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	outer := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		inner.ServeHTTP(w, r.WithContext(WithIdentity(r.Context(), domauth.Identity{Username: "ada"})))
	})
	first := httptest.NewRequest(http.MethodGet, "/", nil)
	first.RemoteAddr = "192.0.2.10:1"
	rec := httptest.NewRecorder()
	outer.ServeHTTP(rec, first)
	if rec.Code != http.StatusOK {
		t.Fatalf("first call status = %d, want 200", rec.Code)
	}
	second := httptest.NewRequest(http.MethodGet, "/", nil)
	second.RemoteAddr = "198.51.100.7:2" // same user, NEW address
	rec = httptest.NewRecorder()
	outer.ServeHTTP(rec, second)
	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("same identity from a second address must share one bucket (got %d)", rec.Code)
	}
}

func TestClientKeyTrustsOnlyTransportAndIdentity(t *testing.T) {
	withIdentity := httptest.NewRequest(http.MethodGet, "/", nil)
	withIdentity.RemoteAddr = "192.0.2.10:4444"
	withIdentity.Header.Set("X-Forwarded-For", "203.0.113.9")
	withIdentity = withIdentity.WithContext(WithIdentity(withIdentity.Context(), domauth.Identity{Username: "ada"}))
	if got := ClientKey(withIdentity); got != "user:ada" {
		t.Errorf("identity key = %q, want user:ada", got)
	}

	anonymous := httptest.NewRequest(http.MethodGet, "/", nil)
	anonymous.RemoteAddr = "192.0.2.10:4444"
	anonymous.Header.Set("X-Forwarded-For", "203.0.113.9")
	if got := ClientKey(anonymous); got != "ip:192.0.2.10" {
		t.Errorf("anonymous key = %q, want the remote host, port stripped, forward header IGNORED", got)
	}

	portless := httptest.NewRequest(http.MethodGet, "/", nil)
	portless.RemoteAddr = "unix" // no host:port to split
	if got := ClientKey(portless); got != "ip:unix" {
		t.Errorf("portless key = %q, want the address verbatim", got)
	}
}
