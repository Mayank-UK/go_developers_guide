package presentation

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"guide/internal/core"
	"guide/internal/domain/demos"
)

// The caching topic's demo endpoint, exercised through the same middleware
// chain production runs behind. The runner port is faked here — the real
// adapter's choreography has its own deterministic tests in
// internal/data/cachinglab — so what these tests pin is the endpoint's
// HTTP-caching behavior itself: the deterministic payload, the strong ETag
// over its bytes, the 304 revalidation round trip, and the honest 500 when
// the choreography cannot run.

// fakeCachingLabRunner stands in for the cachinglab-backed runner: a canned
// run, or the failure its field carries.
type fakeCachingLabRunner struct {
	run demos.CachingLabRun
	err error
}

func (f fakeCachingLabRunner) Run(_ context.Context) (demos.CachingLabRun, error) {
	return f.run, f.err
}

// cachingLabHandlerFixture mirrors a real run's shape closely enough to pin
// the handler's mapping: two beats, the herd, the swap, and the stats.
func cachingLabHandlerFixture() demos.CachingLabRun {
	return demos.CachingLabRun{
		TTLSeconds: int(demos.CachingLabTTL.Seconds()),
		Beats: []demos.CachingLabBeat{
			{
				ID: "cold-read", Title: "The first read misses",
				Script: []string{`GetByID("p-1")`}, Outcome: demos.CachingOutcomeMiss,
				ObservedValue: "Why Go for backends", Hits: 0, Misses: 1, StoreReads: 1,
			},
			{
				ID: "warm-read", Title: "The second read hits",
				Script: []string{`GetByID("p-1")`}, Outcome: demos.CachingOutcomeHit,
				ObservedValue: "Why Go for backends", Hits: 1, Misses: 1, StoreReads: 1,
			},
		},
		Herd: demos.CachingLabHerd{
			ID: demos.CachingLabHerdID, Readers: demos.CachingLabHerdReaders,
			StoreReads: 1, SameAnswer: true, Answer: "ServeMux patterns in Go 1.22",
		},
		Swap: demos.CachingLabSwap{
			LiveOnlyID: demos.CachingLabLiveOnlyID, LiveOnlyTitle: demos.CachingLabLiveOnlyTitle,
			NotFoundID: demos.CachingLabLiveOnlyID, NotFoundSurfaced: true,
			FakeTitle: demos.CachingLabEditedTitle, LiveTitle: "Why Go for backends",
			BackTitle: demos.CachingLabEditedTitle,
		},
		Stats: demos.CachingLabStats{
			Hits: 13, Misses: 10, Expiries: 2, Invalidations: 2,
			StoreReads: 10, Entries: 5,
		},
	}
}

func newCachingDemoServer(runner demos.CachingLabRunner, logger *slog.Logger) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("GET /api/demo/caching/lab", NewCachingLabHandler(runner, logger))
	return Chain(
		RequestID,
		RequestLogger(logger),
		Recover(logger),
		EnvelopeMuxErrors(logger),
	)(mux)
}

// doCachingRequest runs one prebuilt request (headers included) through the
// chain — the conditional 304 cases need If-None-Match set by hand.
func doCachingRequest(server http.Handler, req *http.Request) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	return rec
}

func TestCachingLabDemoServesTheShapedRunWithAValidator(t *testing.T) {
	var buf strings.Builder
	server := newCachingDemoServer(fakeCachingLabRunner{run: cachingLabHandlerFixture()}, newTestLogger(&buf))

	rec := doRequest(server, http.MethodGet, "/api/demo/caching/lab", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("content type = %q, want application/json", ct)
	}
	etag := rec.Header().Get("ETag")
	if etag == "" || !strings.HasPrefix(etag, `"`) || !strings.HasSuffix(etag, `"`) {
		t.Errorf("etag = %q, want a quoted strong validator", etag)
	}
	if cc := rec.Header().Get("Cache-Control"); cc != "no-cache" {
		t.Errorf("cache-control = %q, want no-cache (revalidate, do not go dark)", cc)
	}
	if rec.Header().Get(core.RequestIDHeader) == "" {
		t.Error("every response must carry x-request-id")
	}

	decoded := struct {
		TTLSeconds int `json:"ttlSeconds"`
		Beats      []struct {
			ID      string `json:"id"`
			Outcome string `json:"outcome"`
			Detail  string `json:"detail"`
		} `json:"beats"`
		Herd struct {
			Readers    int    `json:"readers"`
			StoreReads uint64 `json:"storeReads"`
			Proof      string `json:"proof"`
		} `json:"herd"`
		Reasoning string `json:"reasoning"`
	}{}
	if err := json.Unmarshal(rec.Body.Bytes(), &decoded); err != nil {
		t.Fatalf("body not the demo DTO shape: %v\n%s", err, rec.Body.String())
	}
	if decoded.TTLSeconds != 5 {
		t.Errorf("ttlSeconds = %d, want 5", decoded.TTLSeconds)
	}
	if len(decoded.Beats) != 2 || decoded.Beats[0].Outcome != "miss" || decoded.Beats[1].Outcome != "hit" {
		t.Errorf("beats drifted: %+v", decoded.Beats)
	}
	if decoded.Herd.Readers != 12 || decoded.Herd.StoreReads != 1 {
		t.Errorf("herd drifted: %+v", decoded.Herd)
	}
	if !strings.Contains(decoded.Reasoning, "ETag") {
		t.Errorf("reasoning must close the ETag loop: %q", decoded.Reasoning)
	}

	// Determinism is the validator's premise: the same run must produce the
	// same bytes and therefore the same ETag.
	again := doRequest(server, http.MethodGet, "/api/demo/caching/lab", "")
	if other := again.Header().Get("ETag"); other != etag {
		t.Errorf("etag drifted between identical runs: %q vs %q", etag, other)
	}
}

func TestCachingLabDemoAnswers304ForAReplayedValidator(t *testing.T) {
	var buf strings.Builder
	server := newCachingDemoServer(fakeCachingLabRunner{run: cachingLabHandlerFixture()}, newTestLogger(&buf))

	first := doRequest(server, http.MethodGet, "/api/demo/caching/lab", "")
	etag := first.Header().Get("ETag")
	if etag == "" {
		t.Fatal("first response must carry the validator")
	}

	req, err := http.NewRequest(http.MethodGet, "/api/demo/caching/lab", nil)
	if err != nil {
		t.Fatalf("build conditional request: %v", err)
	}
	req.Header.Set("If-None-Match", etag)
	rec := doCachingRequest(server, req)
	if rec.Code != http.StatusNotModified {
		t.Fatalf("status = %d, want 304", rec.Code)
	}
	if body := rec.Body.String(); body != "" {
		t.Errorf("304 must carry no body, got %q", body)
	}
	if replayed := rec.Header().Get("ETag"); replayed != etag {
		t.Errorf("304 etag = %q, want the same validator %q", replayed, etag)
	}
	if rid := rec.Header().Get(core.RequestIDHeader); rid == "" {
		t.Error("304 must still carry x-request-id")
	}

	// The header's list grammar: a validator among others still matches.
	req, err = http.NewRequest(http.MethodGet, "/api/demo/caching/lab", nil)
	if err != nil {
		t.Fatalf("build list conditional: %v", err)
	}
	req.Header.Set("If-None-Match", `"deadbeef", `+etag)
	rec = doCachingRequest(server, req)
	if rec.Code != http.StatusNotModified {
		t.Errorf("list If-None-Match status = %d, want 304", rec.Code)
	}

	// And a non-matching validator is an honest full 200.
	req.Header.Set("If-None-Match", `"deadbeef"`)
	rec = doCachingRequest(server, req)
	if rec.Code != http.StatusOK {
		t.Errorf("stale validator status = %d, want 200 with the full body", rec.Code)
	}
	if rec.Body.Len() == 0 {
		t.Error("a 200 must carry the body back")
	}
}

func TestCachingLabDemoReportsAnInternalErrorWhenTheChoreographyFails(t *testing.T) {
	var buf strings.Builder
	server := newCachingDemoServer(
		fakeCachingLabRunner{err: errors.New("the lab's clock jammed")},
		newTestLogger(&buf),
	)

	rec := doRequest(server, http.MethodGet, "/api/demo/caching/lab", "")
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `"code":"internal"`) {
		t.Errorf("500 body is not the internal envelope: %s", body)
	}
	if !strings.Contains(body, "could not run its choreography") {
		t.Errorf("500 body must say what is wrong in plain words: %s", body)
	}
}

func TestCachingLabDemoRejectsNonGet(t *testing.T) {
	var buf strings.Builder
	server := newCachingDemoServer(fakeCachingLabRunner{run: cachingLabHandlerFixture()}, newTestLogger(&buf))

	rec := doRequest(server, http.MethodPost, "/api/demo/caching/lab", "")
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST status = %d, want 405", rec.Code)
	}
	if allow := rec.Header().Get("Allow"); !strings.Contains(allow, "GET") {
		t.Errorf("Allow header lost: %q", allow)
	}
}
