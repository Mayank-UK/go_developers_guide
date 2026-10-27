package presentation

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"guide/internal/core"
	postsdata "guide/internal/data/posts"
	"guide/internal/domain/demos"
	"guide/internal/domain/posts"
)

// The architecture tracer's endpoint, exercised through the same
// middleware chain production runs behind (the family rule that every
// new demo endpoint ships with a Go test). The pipeline is wired EXACTLY
// as the composition root wires it — leaf recorders inside the
// switching facade, the domain recorder around the facade, the use case
// over that — except the live leaf is a test-owned stub over the PORT,
// so the live-mode flip needs no PostgreSQL to prove the leaf swap.

// archStubLiveRepo stands in for the PostgreSQL adapter in live mode:
// same port, fixed one-item page, labeled by the same hop meta the real
// wiring carries.
type archStubLiveRepo struct{}

func (archStubLiveRepo) List(_ context.Context, _ posts.ListQuery) (posts.PostPage, error) {
	return posts.PostPage{Items: []posts.Post{{ID: "p-1", Title: "Live"}}, Page: 1, Limit: 3, TotalItems: 1}, nil
}
func (archStubLiveRepo) GetByID(_ context.Context, _ string) (posts.Post, error) {
	return posts.Post{}, core.NotFound("no such post.")
}
func (archStubLiveRepo) Create(_ context.Context, _ posts.Draft) (posts.Post, error) {
	return posts.Post{}, nil
}
func (archStubLiveRepo) Replace(_ context.Context, _ string, _ posts.Draft) (posts.Post, error) {
	return posts.Post{}, nil
}
func (archStubLiveRepo) Delete(_ context.Context, _ string) error { return nil }

// newArchTracerServer builds the tracer endpoint with main.go's wiring
// shape over a fresh scenario store (caller-owned, so tests can flip
// the mode) and a paced-clock sink — durations stay deterministic
// multiples of the pacer step.
func newArchTracerServer(t *testing.T, store *core.ScenarioStore) http.Handler {
	t.Helper()
	logger := newTestLogger(&strings.Builder{})
	sink := demos.NewArchTraceSink(archTestPacer())
	fake := demos.NewArchTraceRecorder(postsdata.NewInMemoryRepository(), demos.ArchDataInMemoryHop, sink)
	live := demos.NewArchTraceRecorder(archStubLiveRepo{}, demos.ArchDataPostgresHop, sink)
	facade := postsdata.NewSwitchingRepository(store, fake, live)
	list := posts.NewListPostsUseCase(demos.NewArchTraceRecorder(facade, demos.ArchDomainHop, sink))

	mux := http.NewServeMux()
	mux.Handle("GET "+demos.ArchTracerPath, NewArchTracerHandler(store, list, sink, logger))
	return Chain(
		RequestID,
		RequestLogger(logger),
		Recover(logger),
		EnvelopeMuxErrors(logger),
	)(mux)
}

// archTestPacer advances one millisecond per clock read — the same
// pacer shape the builder's own tests use, so every hop duration here
// is an exact multiple of 1ms and the timeline is byte-stable.
func archTestPacer() func() time.Time {
	step := 0
	return func() time.Time {
		step++
		return time.Unix(0, 0).Add(time.Duration(step) * time.Millisecond)
	}
}

type archTracerDTO struct {
	Trace struct {
		Request  string `json:"request"`
		DataMode string `json:"dataMode"`
		Leaf     string `json:"leaf"`
		FlipNote string `json:"flipNote"`
		Hops     []struct {
			Order      int     `json:"order"`
			Layer      string  `json:"layer"`
			File       string  `json:"file"`
			Symbol     string  `json:"symbol"`
			DurationMs float64 `json:"durationMs"`
		} `json:"hops"`
		Status    int    `json:"status"`
		Items     int    `json:"items"`
		RequestID string `json:"requestId"`
	} `json:"trace"`
	Violation struct {
		Label   string `json:"label"`
		Snippet string `json:"snippet"`
	} `json:"violation"`
}

func decodeArchTracer(t *testing.T, body string) archTracerDTO {
	t.Helper()
	var payload archTracerDTO
	if err := json.Unmarshal([]byte(body), &payload); err != nil {
		t.Fatalf("tracer payload not the demo DTO shape: %v\n%s", err, body)
	}
	return payload
}

func TestArchTracerWalksTheLayersInFakeMode(t *testing.T) {
	store := core.NewScenarioStore()
	server := newArchTracerServer(t, store)

	rec := doRequest(server, http.MethodGet, demos.ArchTracerPath, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if rec.Header().Get(core.RequestIDHeader) == "" {
		t.Error("every response must carry x-request-id")
	}

	payload := decodeArchTracer(t, rec.Body.String())
	trace := payload.Trace
	if trace.DataMode != "fake" || trace.Status != 200 || trace.Items != 3 {
		t.Errorf("fake-mode trace facts drifted: %+v", trace)
	}
	if trace.Request != "GET /api/posts?limit=3&sort=newest" {
		t.Errorf("marker request drifted: %q", trace.Request)
	}
	if len(trace.Hops) != 4 {
		t.Fatalf("timeline hops = %d, want 4 (middleware, handler, domain, data): %+v", len(trace.Hops), trace.Hops)
	}
	wants := []struct{ layer, file, symbol string }{
		{"middleware", "server/internal/presentation/middleware.go", "Chain(RequestID → RequestLogger → Recover → EnvelopeMuxErrors)"},
		{"presentation", "server/internal/presentation/posts.go", "PostsListHandler.ServeHTTP"},
		{"domain", "server/internal/domain/posts/usecases.go", "ListPostsUseCase.Execute"},
		{"data", "server/internal/data/posts/inmemory.go", "InMemoryRepository.List"},
	}
	for i, want := range wants {
		hop := trace.Hops[i]
		if hop.Order != i+1 || hop.Layer != want.layer || hop.File != want.file || hop.Symbol != want.symbol {
			t.Errorf("hop %d = %+v, want %+v", i, hop, want)
		}
		if hop.DurationMs <= 0 {
			t.Errorf("hop %d duration = %v, want a positive paced measurement", i, hop.DurationMs)
		}
	}
	// Inclusive nesting: each layer's duration is at least its child's.
	for i := 1; i < len(trace.Hops); i++ {
		if trace.Hops[i].DurationMs > trace.Hops[i-1].DurationMs {
			t.Errorf("hop %d (%v) exceeds its parent (%v) — inclusive timing broken",
				i, trace.Hops[i].DurationMs, trace.Hops[i-1].DurationMs)
		}
	}
	if !strings.Contains(trace.Leaf, "InMemoryRepository.List") {
		t.Errorf("fake-mode leaf = %q, want the in-memory hop", trace.Leaf)
	}
	if !strings.Contains(trace.FlipNote, "/api/demo/scenario") {
		t.Errorf("flip note must name the scenario endpoint: %q", trace.FlipNote)
	}
	if payload.Violation.Label == "" || !strings.Contains(payload.Violation.Snippet, "h.pool.Query") {
		t.Errorf("violation panel drifted: %+v", payload.Violation)
	}
}

// TestArchTracerLeafSwapsWithTheScenario pins the live half of the
// teaching: flip the store, and the SAME trace shape returns with the
// PostgreSQL hop as the observed leaf — mode and leaf agreeing.
func TestArchTracerLeafSwapsWithTheScenario(t *testing.T) {
	store := core.NewScenarioStore()
	server := newArchTracerServer(t, store)

	fake := decodeArchTracer(t, doRequest(server, http.MethodGet, demos.ArchTracerPath, "").Body.String())
	if fake.Trace.DataMode != "fake" || !strings.Contains(fake.Trace.Leaf, "InMemoryRepository.List") {
		t.Fatalf("baseline trace = %+v, want fake mode on the in-memory leaf", fake.Trace)
	}

	if _, err := store.Update(core.ScenarioPatch{
		DataMode: &[]core.DataMode{core.DataModeLive}[0],
	}); err != nil {
		t.Fatalf("flip scenario to live: %v", err)
	}
	defer func() {
		// Restore the store's default for the rest of the suite.
		_, _ = store.Update(core.ScenarioPatch{
			DataMode: &[]core.DataMode{core.DataModeFake}[0],
		})
	}()

	live := decodeArchTracer(t, doRequest(server, http.MethodGet, demos.ArchTracerPath, "").Body.String())
	if live.Trace.DataMode != "live" {
		t.Errorf("live trace dataMode = %q", live.Trace.DataMode)
	}
	if len(live.Trace.Hops) != 4 || live.Trace.Hops[3].Layer != "data" {
		t.Fatalf("live timeline shape drifted: %+v", live.Trace.Hops)
	}
	if live.Trace.Hops[3].Symbol != "PostgresRepository.List" ||
		live.Trace.Hops[3].File != "server/internal/data/posts/pg.go" {
		t.Errorf("live data hop = %+v, want the pg meta", live.Trace.Hops[3])
	}
	if !strings.Contains(live.Trace.Leaf, "PostgresRepository.List") {
		t.Errorf("live leaf = %q, want the observed pg hop", live.Trace.Leaf)
	}
}

func TestArchTracerRejectsNonGet(t *testing.T) {
	server := newArchTracerServer(t, core.NewScenarioStore())

	rec := doRequest(server, http.MethodPost, demos.ArchTracerPath, "")
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST status = %d, want 405", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"code":"not_found"`) {
		t.Errorf("POST body must be the envelope: %s", rec.Body.String())
	}
	if allow := rec.Header().Get("Allow"); !strings.Contains(allow, "GET") {
		t.Errorf("POST Allow header lost: %q", allow)
	}
}
