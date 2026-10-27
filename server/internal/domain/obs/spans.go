package obs

import (
	"context"
	"sync"
	"time"
)

// Span is one node of a recorded span tree: the trace it belongs to, its
// own id, its parent's id (the remote parent for the root span, a local
// span id beneath that), the name and layer it was opened with, and the
// inclusive start/end window — inclusive exactly the way the architecture
// tracer's hops are: a span's clock runs while everything it called works.
type Span struct {
	TraceID  string
	SpanID   string
	ParentID string
	Name     string
	Layer    string
	Start    time.Time
	EndedAt  time.Time
	recorder *SpanRecorder
}

// SpanRecorder collects finished spans for one traced execution. It is
// mutex-guarded (concurrent seam recording is race-detector clean) and
// holds an injectable clock plus an injectable id minter so tests pace
// and pin the tree exactly. Spans land in COMPLETION order (the
// innermost returns first); consumers sort by Start for the pre-order
// tree.
type SpanRecorder struct {
	mu    sync.Mutex
	spans []Span
	now   func() time.Time
	newID func() string
}

// NewSpanRecorder builds a recorder over the given clock and span-id
// minters (nil means time.Now / NewSpanID — the production defaults).
func NewSpanRecorder(now func() time.Time) *SpanRecorder {
	if now == nil {
		now = time.Now
	}
	return &SpanRecorder{now: now, newID: NewSpanID}
}

// Start opens a span under the ambient context: the trace id comes from
// the propagated TraceContext (a fresh root when none is riding the
// context — Start never refuses to record), the parent is the CURRENT
// span on the context if one is open, and the returned context carries
// the new span as current so nested recorders parent themselves
// automatically. That is the mechanics of a distributed tracer in
// miniature: context in, context out, one parent pointer.
func (r *SpanRecorder) Start(ctx context.Context, name, layer string) (context.Context, *Span) {
	traceID := NewTraceID()
	parentID := ""
	if tc, ok := TraceContextFromContext(ctx); ok {
		traceID = tc.TraceID
		// The FIRST span under an inbound context parents on the REMOTE
		// caller's span id — that is what grafts the local tree onto the
		// distributed one.
		parentID = tc.ParentID
	}
	if current, ok := currentSpanID(ctx); ok {
		parentID = current
	}
	span := &Span{
		TraceID:  traceID,
		SpanID:   r.newID(),
		ParentID: parentID,
		Name:     name,
		Layer:    layer,
		Start:    r.now(),
		recorder: r,
	}
	return withCurrentSpanID(ctx, span.SpanID), span
}

// End stamps the span's end time and records it into the sink. Safe to
// call exactly once per Start — the recorder keeps the first stamp.
func (s *Span) End() {
	if s == nil || s.recorder == nil {
		return
	}
	s.recorder.mu.Lock()
	defer s.recorder.mu.Unlock()
	s.EndedAt = s.recorder.now()
	s.recorder.spans = append(s.recorder.spans, *s)
}

// Spans returns a copy of the finished spans in completion order — always
// non-nil, so an empty recorder reads as [] on the wire.
func (r *SpanRecorder) Spans() []Span {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append(make([]Span, 0, len(r.spans)), r.spans...)
}

// Reset clears the recorded spans so a recorder wired once at the
// composition root serves each traced execution a fresh tree.
func (r *SpanRecorder) Reset() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.spans = nil
}

// currentSpanKey is the unexported key for the open span id.
type currentSpanKey struct{}

func withCurrentSpanID(ctx context.Context, spanID string) context.Context {
	return context.WithValue(ctx, currentSpanKey{}, spanID)
}

func currentSpanID(ctx context.Context) (string, bool) {
	if ctx == nil {
		return "", false
	}
	id, ok := ctx.Value(currentSpanKey{}).(string)
	return id, ok
}
