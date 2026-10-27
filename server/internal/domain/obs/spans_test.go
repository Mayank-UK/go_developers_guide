package obs

import (
	"context"
	"testing"
	"time"
)

// A paced clock makes span windows exact multiples of the step, so the
// nesting assertions below are arithmetic, not timing.
func pacedClock(step time.Duration) func() time.Time {
	tick := 0
	return func() time.Time {
		tick++
		return time.Unix(0, 0).Add(time.Duration(tick) * step)
	}
}

func TestSpanRecorderNestsByContext(t *testing.T) {
	rec := NewSpanRecorder(pacedClock(time.Millisecond))
	root := NewRoot()
	ctx := WithTraceContext(context.Background(), root)

	ctx1, span1 := rec.Start(ctx, "http.request", "middleware")
	ctx2, span2 := rec.Start(ctx1, "PostsListHandler.ServeHTTP", "presentation")
	ctx3, span3 := rec.Start(ctx2, "PostsRepository.List", "domain")
	_, span4 := rec.Start(ctx3, "InMemoryRepository.List", "data")

	if span1.ParentID != root.ParentID {
		t.Errorf("root span parent = %q, want the inbound remote parent %q", span1.ParentID, root.ParentID)
	}
	if span2.ParentID != span1.SpanID || span3.ParentID != span2.SpanID || span4.ParentID != span3.SpanID {
		t.Errorf("parent chain broke: %q %q %q", span2.ParentID, span3.ParentID, span4.ParentID)
	}
	for _, span := range []*Span{span1, span2, span3, span4} {
		if span.TraceID != root.TraceID {
			t.Errorf("span %q left the trace: %q", span.Name, span.TraceID)
		}
	}

	// End innermost-first, mirroring real completion order.
	span4.End()
	span3.End()
	span2.End()
	span1.End()

	spans := rec.Spans()
	if len(spans) != 4 {
		t.Fatalf("recorded %d spans, want 4", len(spans))
	}
	if spans[0].Name != "InMemoryRepository.List" {
		t.Errorf("completion order broken: %+v", spans)
	}
	// Inclusive windows: each outer span's end is later than its child's.
	if !spans[3].EndedAt.After(spans[2].EndedAt) || !spans[2].EndedAt.After(spans[1].EndedAt) {
		t.Errorf("inclusive nesting broken: %+v", spans)
	}
}

func TestSpanRecorderMintsRootWhenUntraced(t *testing.T) {
	rec := NewSpanRecorder(pacedClock(time.Millisecond))
	_, span := rec.Start(context.Background(), "untraced", "middleware")
	span.End()
	if span.TraceID == "" || span.ParentID != "" {
		t.Errorf("untraced start must mint a root with no parent: %+v", span)
	}
	if len(span.TraceID) != 32 {
		t.Errorf("minted trace id must be 32 hex digits: %q", span.TraceID)
	}
}

func TestSpanRecorderResetServesFreshTrees(t *testing.T) {
	rec := NewSpanRecorder(pacedClock(time.Millisecond))
	_, first := rec.Start(context.Background(), "one", "middleware")
	first.End()
	if len(rec.Spans()) != 1 {
		t.Fatalf("first tree must hold one span")
	}
	rec.Reset()
	if spans := rec.Spans(); len(spans) != 0 {
		t.Errorf("after reset, spans = %+v, want none", spans)
	}
}

func TestSpanRecorderConcurrentStartsAreRaceClean(t *testing.T) {
	rec := NewSpanRecorder(time.Now)
	done := make(chan struct{})
	for worker := 0; worker < 8; worker++ {
		go func(worker int) {
			defer func() { done <- struct{}{} }()
			for i := 0; i < 50; i++ {
				_, span := rec.Start(context.Background(), "concurrent", "test")
				span.End()
			}
		}(worker)
	}
	for worker := 0; worker < 8; worker++ {
		<-done
	}
	if got := len(rec.Spans()); got != 400 {
		t.Errorf("recorded %d spans, want 400", got)
	}
}
