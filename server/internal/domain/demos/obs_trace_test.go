package demos

import (
	"context"
	"strings"
	"testing"
	"time"

	"guide/internal/core"
	"guide/internal/domain/obs"
	"guide/internal/domain/posts"
)

// The trace demo's builder tests: the spanning repository records real
// spans through the port, the tree renders pre-order with the remote
// graft point, and the malformed table carries the parser's own
// reasons.

// obsStubRepo stands in for the in-memory adapter at the leaf seam.
type obsStubRepo struct{}

func (obsStubRepo) List(_ context.Context, _ posts.ListQuery) (posts.PostPage, error) {
	return posts.PostPage{Items: []posts.Post{{ID: "p-1", Title: "One"}}, Page: 1, Limit: 2, TotalItems: 1}, nil
}
func (obsStubRepo) GetByID(_ context.Context, _ string) (posts.Post, error) {
	return posts.Post{}, core.NotFound("no such post.")
}
func (obsStubRepo) Create(_ context.Context, _ posts.Draft) (posts.Post, error) {
	return posts.Post{}, nil
}
func (obsStubRepo) Replace(_ context.Context, _ string, _ posts.Draft) (posts.Post, error) {
	return posts.Post{}, nil
}
func (obsStubRepo) Delete(_ context.Context, _ string) error { return nil }

// obsPacer advances one millisecond per clock read — exact spans.
func obsPacer() func() time.Time {
	step := 0
	return func() time.Time {
		step++
		return time.Unix(0, 0).Add(time.Duration(step) * time.Millisecond)
	}
}

func TestObsSpanningRepositoryRecordsThePortSeam(t *testing.T) {
	rec := obs.NewSpanRecorder(obsPacer())
	repo := NewObsSpanningRepository(obsStubRepo{}, "data", "StubRepository", rec)
	root := obs.NewRoot()
	ctx := obs.WithTraceContext(context.Background(), root)

	if _, err := repo.List(ctx, posts.ListQuery{Page: 1, Limit: 2}); err != nil {
		t.Fatalf("List through the decorator: %v", err)
	}
	spans := rec.Spans()
	if len(spans) != 1 {
		t.Fatalf("recorded %d spans, want 1", len(spans))
	}
	span := spans[0]
	if span.Name != "StubRepository.List" || span.Layer != "data" || span.TraceID != root.TraceID {
		t.Errorf("span drifted: %+v", span)
	}
	if span.EndedAt.Before(span.Start) {
		t.Errorf("paced span window inverted: %+v", span)
	}
}

func TestObsTraceTreeRendersPreOrderWithTheGraftPoint(t *testing.T) {
	rec := obs.NewSpanRecorder(obsPacer())
	root := obs.TraceContext{Version: "00", TraceID: "4bf92f3577b34da6a3ce929d0e0e4736", ParentID: "00f067aa0ba902b7", Flags: "01"}

	leaf := NewObsSpanningRepository(obsStubRepo{}, "data", "InMemoryRepository", rec)
	port := NewObsSpanningRepository(leaf, "domain", "PostsRepository", rec)
	list := posts.NewListPostsUseCase(port)

	// The walk shape the handler produces: root span (middleware), the
	// handler-boundary span, then the port chain under it.
	base := obs.WithTraceContext(context.Background(), root)
	ctx, rootSpan := rec.Start(base, "GET /api/posts (traced walk)", "middleware")
	func() {
		ctx, span := rec.Start(ctx, "PostsListHandler.ServeHTTP", "presentation")
		defer span.End()
		if _, err := list.Execute(ctx, posts.ListQuery{Page: 1, Limit: 2}); err != nil {
			t.Fatalf("execute: %v", err)
		}
	}()
	rootSpan.End()

	payload := BuildObsTracePayload(ObsTraceFacts{
		InboundHeader: root.String(),
		Accepted:      root,
		Status:        200,
		Spans:         rec.Spans(),
	})

	if len(payload.Tree) != 4 {
		t.Fatalf("tree = %d nodes, want 4 (middleware, presentation, domain, data): %+v", len(payload.Tree), payload.Tree)
	}
	wantShape := []struct {
		name, layer string
		depth       int
	}{
		{"GET /api/posts (traced walk)", "middleware", 0},
		{"PostsListHandler.ServeHTTP", "presentation", 1},
		{"PostsRepository.List", "domain", 2},
		{"InMemoryRepository.List", "data", 3},
	}
	for i, want := range wantShape {
		row := payload.Tree[i]
		if row.Name != want.name || row.Layer != want.layer || row.Depth != want.depth {
			t.Errorf("tree[%d] = %+v, want %q/%q depth %d", i, row, want.name, want.layer, want.depth)
		}
		if row.TraceID != root.TraceID {
			t.Errorf("node %q left the trace", row.Name)
		}
	}
	// The graft point: the root's parent is the inbound remote span.
	if payload.Tree[0].ParentID != "00f067aa0ba902b7" {
		t.Errorf("root parent = %q, want the inbound remote span", payload.Tree[0].ParentID)
	}
	// Durations nest inclusively down the tree.
	for i := 1; i < len(payload.Tree); i++ {
		if payload.Tree[i].DurationMs > payload.Tree[i-1].DurationMs {
			t.Errorf("node %d outlives its parent — inclusive timing broken", i)
		}
	}
	// The acceptance panel echoes a child with the same trace id.
	echo := payload.Accepted.ResponseEcho
	if !strings.HasPrefix(echo, "00-4bf92f3577b34da6a3ce929d0e0e4736-") {
		t.Errorf("response echo = %q, want the same trace id", echo)
	}
	if !payload.Accepted.Sampled {
		t.Error("flags 01 must report sampled")
	}
}

func TestObsTraceMalformedTableCarriesRealReasons(t *testing.T) {
	payload := BuildObsTracePayload(ObsTraceFacts{Accepted: obs.NewRoot()})
	if len(payload.Malformed) != len(ObsMalformedHeaders) {
		t.Fatalf("malformed rows = %d, want %d", len(payload.Malformed), len(ObsMalformedHeaders))
	}
	seenReasons := map[string]bool{}
	for _, row := range payload.Malformed {
		if row.Problem == "" || row.HandledAs == "" {
			t.Errorf("row must carry the parser's reason and the policy: %+v", row)
		}
		seenReasons[row.Problem] = true
	}
	// Distinct failure modes surface: zeros, widths, reserved version,
	// non-hex, extra field.
	if len(seenReasons) < 5 {
		t.Errorf("hostile headers must surface distinct real reasons: %v", seenReasons)
	}
	for _, row := range payload.Malformed {
		if !strings.Contains(row.HandledAs, "fresh root") {
			t.Errorf("policy note must state the fresh-root handling: %+v", row)
		}
	}
}

func TestObsTraceMintedOriginIsHonest(t *testing.T) {
	payload := BuildObsTracePayload(ObsTraceFacts{Accepted: obs.NewRoot()})
	if !strings.Contains(payload.Accepted.InboundOrigin, "minted by the server") {
		t.Errorf("absent header must report the mint honestly: %q", payload.Accepted.InboundOrigin)
	}
	if payload.Accepted.Inbound != "" {
		t.Errorf("inbound must be the empty string it was")
	}
	if !strings.Contains(payload.SDK, "OpenTelemetry SDK") {
		t.Errorf("the SDK comparison must be present: %q", payload.SDK)
	}
}
