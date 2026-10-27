// Topic 33 observability's second live demo: the W3C trace-context
// round-trip. The handler walks one REAL posts request through a
// pipeline wrapped with the production TraceContext middleware and
// span recorders at the seams; the inbound traceparent (yours, when
// the request carries one; minted, when it does not) is accepted,
// child spans land across handler → use case → adapter, and this
// builder renders the tree plus the malformed-header table. The
// ObsSpanningRepository below is the port decorator that records those
// spans — the same decoration move the architecture tracer timed with,
// proving instrumentation and measurement are one capability. The
// builder is PURE: spans in, tree out; the malformed table is parsed
// FOR REAL here from the hostile-header constants.
package demos

import (
	"context"
	"sort"
	"time"

	"guide/internal/domain/obs"
	"guide/internal/domain/posts"
)

// ObsTracePath is the demo area's endpoint — registered on the real mux
// by the composition root. Curl it WITH your own traceparent header to
// watch the walk adopt it.
const ObsTracePath = "/api/demo/obs/trace"

// ObsTraceRequestLine is the marker request the demo walks through the
// traced pipeline.
const ObsTraceRequestLine = "GET /api/posts?limit=2&sort=newest"

// ObsMalformedHeaders is the hostile-header table: every entry is
// parsed for real by the builder, and the reported problem is the
// parser's actual error, never a caption written by hand.
var ObsMalformedHeaders = []string{
	"00-00000000000000000000000000000000-00f067aa0ba902b7-01",
	"00-4bf92f3577b34da6a3ce929d0e0e4736-0000000000000000-01",
	"00-4bf92f3577b34da6-00f067aa0ba902b7-01",
	"ff-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01",
	"00-4bf92f3577b34da6a3ce929d0e0e473g-00f067aa0ba902b7-01",
	"00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-0x",
	"00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01-oops",
}

// Compile-time proof the spanning repository satisfies the port it
// decorates.
var _ posts.PostsRepository = (*ObsSpanningRepository)(nil)

// ObsSpanningRepository decorates one posts repository seam: every port
// call opens a span under the ambient context (the trace id rides the
// W3C-propagated context; the parent is whatever span is currently
// open), delegates unchanged, and ends the span — arguments, results,
// and errors pass through untouched.
type ObsSpanningRepository struct {
	inner    posts.PostsRepository
	layer    string
	symbol   string
	recorder *obs.SpanRecorder
}

// NewObsSpanningRepository wires a spanning recorder over the port
// seam; symbol is the seam's real name ("InMemoryRepository",
// "PostsRepository") and layer its architecture layer.
func NewObsSpanningRepository(inner posts.PostsRepository, layer, symbol string, recorder *obs.SpanRecorder) *ObsSpanningRepository {
	return &ObsSpanningRepository{inner: inner, layer: layer, symbol: symbol, recorder: recorder}
}

// List implements the port through the wrapped repository, spanned.
func (rec *ObsSpanningRepository) List(ctx context.Context, query posts.ListQuery) (posts.PostPage, error) {
	ctx, span := rec.recorder.Start(ctx, rec.symbol+".List", rec.layer)
	defer span.End()
	return rec.inner.List(ctx, query)
}

// GetByID implements the port through the wrapped repository, spanned.
func (rec *ObsSpanningRepository) GetByID(ctx context.Context, id string) (posts.Post, error) {
	ctx, span := rec.recorder.Start(ctx, rec.symbol+".GetByID", rec.layer)
	defer span.End()
	return rec.inner.GetByID(ctx, id)
}

// Create implements the port through the wrapped repository, spanned.
func (rec *ObsSpanningRepository) Create(ctx context.Context, draft posts.Draft) (posts.Post, error) {
	ctx, span := rec.recorder.Start(ctx, rec.symbol+".Create", rec.layer)
	defer span.End()
	return rec.inner.Create(ctx, draft)
}

// Replace implements the port through the wrapped repository, spanned.
func (rec *ObsSpanningRepository) Replace(ctx context.Context, id string, draft posts.Draft) (posts.Post, error) {
	ctx, span := rec.recorder.Start(ctx, rec.symbol+".Replace", rec.layer)
	defer span.End()
	return rec.inner.Replace(ctx, id, draft)
}

// Delete implements the port through the wrapped repository, spanned.
func (rec *ObsSpanningRepository) Delete(ctx context.Context, id string) error {
	ctx, span := rec.recorder.Start(ctx, rec.symbol+".Delete", rec.layer)
	defer span.End()
	return rec.inner.Delete(ctx, id)
}

// ObsTraceFacts is everything the payload derives from, observed by the
// handler: the inbound header (empty string when the request carried
// none), the tracestate as accepted, the context actually accepted for
// the walk (parsed or minted), the walk's answer, and the recorded
// spans in completion order.
type ObsTraceFacts struct {
	InboundHeader string
	InboundState  string
	Accepted      obs.TraceContext
	Status        int
	Spans         []obs.Span
}

// ObsSpanRow is one node of the rendered tree: pre-order (outermost
// first), depth-derived, with offsets relative to the walk's first
// span — the shape the client indents.
type ObsSpanRow struct {
	Name          string  `json:"name"`
	Layer         string  `json:"layer"`
	SpanID        string  `json:"spanId"`
	ParentID      string  `json:"parentId"`
	TraceID       string  `json:"traceId"`
	Depth         int     `json:"depth"`
	StartOffsetMs float64 `json:"startOffsetMs"`
	DurationMs    float64 `json:"durationMs"`
	Note          string  `json:"note"`
}

// ObsTraceAccepted is the acceptance panel: what arrived, what parsed,
// what the response echoed.
type ObsTraceAccepted struct {
	Inbound       string `json:"inbound"`
	InboundOrigin string `json:"inboundOrigin"`
	Version       string `json:"version"`
	TraceID       string `json:"traceId"`
	ParentID      string `json:"parentId"`
	Flags         string `json:"flags"`
	Sampled       bool   `json:"sampled"`
	Tracestate    string `json:"tracestate,omitempty"`
	ResponseEcho  string `json:"responseEcho"`
	Note          string `json:"note"`
}

// ObsMalformedRow is one hostile header, its real rejection reason,
// and the policy that handled it.
type ObsMalformedRow struct {
	Header    string `json:"header"`
	Problem   string `json:"problem"`
	HandledAs string `json:"handledAs"`
}

// ObsTracePayload is the wire shape of GET /api/demo/obs/trace.
type ObsTracePayload struct {
	Accepted  ObsTraceAccepted  `json:"accepted"`
	Tree      []ObsSpanRow      `json:"tree"`
	Malformed []ObsMalformedRow `json:"malformed"`
	SDK       string            `json:"sdk"`
	Reasoning string            `json:"reasoning"`
}

// BuildObsTracePayload shapes the panels (pure) from the walk's facts.
func BuildObsTracePayload(facts ObsTraceFacts) ObsTracePayload {
	origin := "minted by the server for this walk (the request carried no traceparent — curl this endpoint with your own header to watch it adopted)"
	if facts.InboundHeader != "" {
		origin = "the request's own header, adopted verbatim"
	}
	accepted := ObsTraceAccepted{
		Inbound:       facts.InboundHeader,
		InboundOrigin: origin,
		Version:       facts.Accepted.Version,
		TraceID:       facts.Accepted.TraceID,
		ParentID:      facts.Accepted.ParentID,
		Flags:         facts.Accepted.Flags,
		Sampled:       facts.Accepted.Sampled(),
		Tracestate:    facts.InboundState,
		ResponseEcho:  facts.Accepted.Child().String(),
		Note: "the response to the traced walk carries traceparent " + facts.Accepted.Child().String() +
			" — same trace, the server's own span id — which is how the caller grafts this work onto its trace",
	}

	tree := buildObsTree(facts)
	malformed := buildObsMalformedTable()

	return ObsTracePayload{
		Accepted:  accepted,
		Tree:      tree,
		Malformed: malformed,
		SDK: "what the OpenTelemetry SDK adds on top of this skeleton: a spec-complete W3C " +
			"parser, real samplers (this walk honors the sampled bit as a hint only), the " +
			"resource/service model, span attributes and events, exporters (OTLP over gRPC), " +
			"and propagation of baggage. What it does NOT change: the header, the context, " +
			"the parent-pointer mechanics rendered above — those are the protocol, and the " +
			"protocol is what you just watched work",
		Reasoning: "One header became one tree: the inbound traceparent was accepted (or " +
			"minted), the walk's spans grafted onto its parent id, and every layer between " +
			"the router and the in-memory store recorded itself with the context it was " +
			"handed — no global registry, no thread-local, just one value flowing down the " +
			"call chain and parent pointers flowing back up. The malformed table is the " +
			"other half of the contract: every hostile header was parsed for real, every " +
			"rejection is the parser's own reason, and none of them failed the request",
	}
}

// buildObsTree renders the recorded spans as a pre-order, depth-annotated
// tree with offsets from the walk's start.
func buildObsTree(facts ObsTraceFacts) []ObsSpanRow {
	spans := append([]obs.Span(nil), facts.Spans...)
	sort.SliceStable(spans, func(i, j int) bool { return spans[i].Start.Before(spans[j].Start) })

	depths := make(map[string]int, len(spans))
	rows := make([]ObsSpanRow, 0, len(spans))
	var zero time.Time
	if len(spans) > 0 {
		zero = spans[0].Start
	}
	byID := make(map[string]bool, len(spans))
	for _, span := range spans {
		byID[span.SpanID] = true
	}
	for _, span := range spans {
		depth := 0
		if span.ParentID != "" && byID[span.ParentID] {
			depth = depths[span.ParentID] + 1
		} else if span.ParentID != "" {
			// The remote parent: depth 0 is the graft point itself.
			depth = 0
		}
		depths[span.SpanID] = depth
		rows = append(rows, ObsSpanRow{
			Name:          span.Name,
			Layer:         span.Layer,
			SpanID:        span.SpanID,
			ParentID:      span.ParentID,
			TraceID:       span.TraceID,
			Depth:         depth,
			StartOffsetMs: float64(span.Start.Sub(zero).Microseconds()) / 1000,
			DurationMs:    float64(span.EndedAt.Sub(span.Start).Microseconds()) / 1000,
			Note:          obsSpanNote(span, depth),
		})
	}
	return rows
}

// obsSpanNote explains one row's teaching point.
func obsSpanNote(span obs.Span, depth int) string {
	if depth == 0 {
		return "the walk's root: its parent id is the INBOUND remote span — the graft point where this server's tree joins yours"
	}
	switch span.Layer {
	case "presentation":
		return "the thin handler's span — opened at the handler boundary, inclusive of everything it delegated to"
	case "domain":
		return "recorded at the repository PORT boundary — the use case's delegation, the domain's only seam over storage"
	default:
		return "the adapter's span — the leaf that actually answered, nested inside every layer above it"
	}
}

// buildObsMalformedTable parses every hostile header for real.
func buildObsMalformedTable() []ObsMalformedRow {
	rows := make([]ObsMalformedRow, 0, len(ObsMalformedHeaders))
	for _, header := range ObsMalformedHeaders {
		_, err := obs.ParseTraceparent(header)
		problem := "accepted (this entry belongs in the OTHER panel)"
		if err != nil {
			problem = err.Error()
		}
		rows = append(rows, ObsMalformedRow{
			Header:    header,
			Problem:   problem,
			HandledAs: "rejected by the parser; the middleware proceeds under a fresh root and logs the reason at debug — a broken header never fails the request carrying it",
		})
	}
	return rows
}
