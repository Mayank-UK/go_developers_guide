package demos

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"guide/internal/core"
	"guide/internal/domain/posts"
)

// The architecture tracer's builder + recorder, tested with a paced
// clock so every duration pins exactly, and a fake repository over the
// PORT (the recorder must not care which adapter sits beneath it — the
// same property that lets the composition root wire it around the
// switching facade's two leaves).

// archFakeRepo is a deterministic fake over the port: List answers a
// fixed one-item page, GetByID answers a typed miss, and the write
// verbs delegate to flags the tests can observe.
type archFakeRepo struct {
	listed  int
	deleted bool
	failAll error
}

func (f *archFakeRepo) List(_ context.Context, _ posts.ListQuery) (posts.PostPage, error) {
	f.listed++
	if f.failAll != nil {
		return posts.PostPage{}, f.failAll
	}
	return posts.PostPage{Items: []posts.Post{{ID: "p-1", Title: "Seeded"}}, Page: 1, Limit: 3, TotalItems: 1}, nil
}

func (f *archFakeRepo) GetByID(_ context.Context, _ string) (posts.Post, error) {
	return posts.Post{}, core.NotFound("no such post.")
}

func (f *archFakeRepo) Create(_ context.Context, draft posts.Draft) (posts.Post, error) {
	return posts.Post{ID: "p-7", Title: draft.Title}, nil
}

func (f *archFakeRepo) Replace(_ context.Context, _ string, draft posts.Draft) (posts.Post, error) {
	return posts.Post{ID: "p-1", Title: draft.Title}, nil
}

func (f *archFakeRepo) Delete(_ context.Context, _ string) error {
	f.deleted = true
	return nil
}

// archPacedClock advances one exact millisecond per read, so a seam
// spanning k clock reads measures exactly k milliseconds.
func archPacedClock() func() time.Time {
	step := 0
	return func() time.Time {
		step++
		return time.Unix(0, 0).Add(time.Duration(step) * time.Millisecond)
	}
}

func TestArchTraceRecorderRecordsEveryPortCall(t *testing.T) {
	sink := NewArchTraceSink(archPacedClock())
	fake := &archFakeRepo{}
	recorder := NewArchTraceRecorder(fake, ArchDataInMemoryHop, sink)

	// One List call spans two clock reads (start + record): exactly 1ms.
	page, err := recorder.List(context.Background(), posts.ListQuery{Page: 1, Limit: 3})
	if err != nil || len(page.Items) != 1 || page.Items[0].ID != "p-1" {
		t.Fatalf("List must delegate unchanged: page=%+v err=%v", page, err)
	}
	hops := sink.Hops()
	if len(hops) != 1 {
		t.Fatalf("recorded %d hops, want 1", len(hops))
	}
	if hops[0].Layer != ArchLayerData || hops[0].File != "server/internal/data/posts/inmemory.go" ||
		hops[0].Symbol != "InMemoryRepository.List" {
		t.Errorf("hop meta drifted: %+v", hops[0])
	}
	if hops[0].DurationMs != 1 {
		t.Errorf("paced duration = %v, want exactly 1ms", hops[0].DurationMs)
	}

	// The write verbs record through the same seam, values still pass.
	if _, err := recorder.Create(context.Background(), posts.Draft{Title: "T", Body: "B"}); err != nil {
		t.Fatalf("Create delegation broke: %v", err)
	}
	if err := recorder.Delete(context.Background(), "p-1"); err != nil {
		t.Fatalf("Delete delegation broke: %v", err)
	}
	if !fake.deleted || len(sink.Hops()) != 3 {
		t.Errorf("delegation flags/hop count drifted: deleted=%v hops=%d", fake.deleted, len(sink.Hops()))
	}

	sink.Reset()
	if got := sink.Hops(); len(got) != 0 || got == nil {
		t.Errorf("Reset must clear the hops for the next trace, got %+v", got)
	}
}

func TestArchTraceRecorderPassesErrorsThroughUntouched(t *testing.T) {
	sink := NewArchTraceSink(archPacedClock())
	typed := core.ServiceUnavailable("live store unreachable.")
	recorder := NewArchTraceRecorder(&archFakeRepo{failAll: typed}, ArchDataPostgresHop, sink)

	_, err := recorder.List(context.Background(), posts.ListQuery{})
	if !errors.Is(err, core.ErrServiceUnavailable) {
		t.Fatalf("error must pass through the recorder wrapped in %%w-reachable form: %v", err)
	}
	hops := sink.Hops()
	if len(hops) != 1 || hops[0].DurationMs != 1 {
		t.Errorf("failing calls record too: %+v", hops)
	}
}

func TestArchTracerPayloadReversesToOutermostFirstAndDerivesTheLeaf(t *testing.T) {
	// Completion order as the sink records it: data leaf, domain, handler.
	hops := []ArchTraceHop{
		{Layer: ArchLayerData, File: "server/internal/data/posts/pg.go", Symbol: "PostgresRepository.List", DurationMs: 1.2},
		{Layer: ArchLayerDomain, File: "server/internal/domain/posts/usecases.go", Symbol: "ListPostsUseCase.Execute", DurationMs: 1.4},
		{Layer: ArchLayerPresentation, File: "server/internal/presentation/posts.go", Symbol: "PostsListHandler.ServeHTTP", DurationMs: 2},
		{Layer: ArchLayerMiddleware, File: "server/internal/presentation/middleware.go", Symbol: "Chain(...)", DurationMs: 3},
	}
	payload := BuildArchTracerPayload(ArchTracerFacts{
		DataMode:  core.DataModeLive,
		Hops:      hops,
		Status:    200,
		Items:     3,
		Elapsed:   3 * time.Millisecond,
		RequestID: "rid-arch-1",
	})

	trace := payload.Trace
	if trace.DataMode != "live" || trace.Status != 200 || trace.Items != 3 || trace.RequestID != "rid-arch-1" {
		t.Errorf("trace facts drifted: %+v", trace)
	}
	if len(trace.Hops) != 4 {
		t.Fatalf("timeline hops = %d, want 4", len(trace.Hops))
	}
	wantOrder := []string{ArchLayerMiddleware, ArchLayerPresentation, ArchLayerDomain, ArchLayerData}
	for i, hop := range trace.Hops {
		if hop.Order != i+1 || hop.Layer != wantOrder[i] {
			t.Errorf("hop %d = order %d layer %q, want %d/%q", i, hop.Order, hop.Layer, i+1, wantOrder[i])
		}
	}
	if trace.Hops[0].DurationMs != 3 || trace.Hops[3].DurationMs != 1.2 {
		t.Errorf("durations must survive the reversal: %+v", trace.Hops)
	}
	// The leaf derives from the OBSERVED data hop, not the mode string.
	if trace.Leaf != "PostgresRepository.List — server/internal/data/posts/pg.go" {
		t.Errorf("leaf = %q, want the observed pg hop", trace.Leaf)
	}
	if trace.Request != ArchTracerRequestLine {
		t.Errorf("request line drifted: %q", trace.Request)
	}
	for _, want := range []string{"PATCH /api/demo/scenario", "different leaf hop"} {
		if !contains(trace.FlipNote, want) {
			t.Errorf("flip note lost %q: %q", want, trace.FlipNote)
		}
	}
}

func TestArchTracerPayloadNamesTheFakeLeafInFakeMode(t *testing.T) {
	payload := BuildArchTracerPayload(ArchTracerFacts{
		DataMode: core.DataModeFake,
		Hops: []ArchTraceHop{
			{Layer: ArchLayerData, File: "server/internal/data/posts/inmemory.go", Symbol: "InMemoryRepository.List"},
			{Layer: ArchLayerDomain, File: "server/internal/domain/posts/usecases.go", Symbol: "ListPostsUseCase.Execute"},
			{Layer: ArchLayerPresentation, File: "server/internal/presentation/posts.go", Symbol: "PostsListHandler.ServeHTTP"},
		},
		Status: 200, Items: 3, RequestID: "rid-arch-2",
	})
	if payload.Trace.DataMode != "fake" ||
		payload.Trace.Leaf != "InMemoryRepository.List — server/internal/data/posts/inmemory.go" {
		t.Errorf("fake-mode trace drifted: %+v", payload.Trace)
	}
}

func TestArchTracerPayloadHonestEmptyAndViolationPanel(t *testing.T) {
	payload := BuildArchTracerPayload(ArchTracerFacts{DataMode: core.DataModeFake})
	if payload.Trace.Hops == nil {
		t.Error("empty trace must still marshal hops as [] never null")
	}
	if payload.Trace.Leaf != "no data hop recorded — the traced wiring always has one, so this is the honest-empty case" {
		t.Errorf("empty leaf honesty drifted: %q", payload.Trace.Leaf)
	}

	violation := payload.Violation
	if violation.Label != "counterexample — not code this repo compiles" {
		t.Errorf("violation label drifted: %q", violation.Label)
	}
	if violation.Snippet != strings.TrimSpace(ArchViolationSnippet) || !contains(violation.Snippet, "h.pool.Query") {
		t.Error("the panel must carry the counterexample verbatim")
	}
	for _, want := range []string{"grpc_console.go", "grep", "envelope"} {
		if !contains(violation.Honest+violation.CaughtBy+violation.Why, want) {
			t.Errorf("violation panel lost %q", want)
		}
	}
	if payload.Reasoning == "" {
		t.Error("reasoning must never be empty")
	}
}

// contains is the test-local substring check (the package keeps stdlib
// testing only — no testify, per the stack decisions).
func contains(haystack, needle string) bool {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}
