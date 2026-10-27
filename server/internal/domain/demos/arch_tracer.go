// Topic 32 architecture's live demo: the request-flow tracer. One REAL
// request — the posts list, the same surface the api-design topic built —
// walked through the actual layers with recorders at the seams: the
// production middleware chain times the whole call, a wrapper times the
// thin handler, and repository-port recorders time the domain's use-case
// delegation and the data adapter the switching facade picked. Every hop
// is an execution that just happened, labeled by the real file and symbol
// that ran, with the duration measured at that seam — inclusive, because
// each layer's clock runs while the layers beneath it work. The scenario
// store chooses the leaf per call (fake answers from the in-memory
// adapter, live from the PostgreSQL adapter), so PATCH /api/demo/scenario
// and a reload show the SAME trace shape with a different leaf hop. The
// counterexample panel completes the lesson: what a dependency-rule
// violation WOULD look like — a handler importing the data layer —
// carried as a string constant, clearly labeled, never compiled and
// never wired. The builder below is PURE: facts in, payload out.
package demos

import (
	"context"
	"strings"
	"sync"
	"time"

	"guide/internal/core"
	"guide/internal/domain/posts"
)

// ArchTracerPath is the demo area's endpoint — registered on the real mux
// by the composition root, so a plain curl exercises the trace this
// builder shapes.
const ArchTracerPath = "/api/demo/arch/tracer"

// ArchTracerScenarioPath names the scenario store's real endpoints — the
// runtime toggle that flips the trace's leaf adapter.
const ArchTracerScenarioPath = "/api/demo/scenario"

// ArchTracerRequestLine is the marker request the handler traces — the
// posts list with an explicit page shape, so every hop has real work to
// do (query parsing, normalization, paging, sorting, JSON shaping).
const ArchTracerRequestLine = "GET /api/posts?limit=3&sort=newest"

// Layer names for the timeline — the four stops on the dependency arc.
const (
	ArchLayerMiddleware   = "middleware"
	ArchLayerPresentation = "presentation"
	ArchLayerDomain       = "domain"
	ArchLayerData         = "data"
)

// ArchHopMeta is the static descriptor of one traced seam: which layer
// it belongs to, the real file and symbol that runs there, and the note
// the timeline prints. The duration is measured at runtime and appended
// by the sink — the meta never carries a number.
type ArchHopMeta struct {
	Layer  string
	File   string
	Symbol string
	Note   string
}

// The four seams the traced wiring instruments (main.go wires these
// exact metas onto the recorders). Files are repo-root-relative, the
// same convention the content snippets use.
var (
	// ArchMiddlewareHop describes the production middleware chain the
	// traced request enters first — the same order the composition root
	// serves: request id, access log, panic recovery, envelope for the
	// router's own errors.
	ArchMiddlewareHop = ArchHopMeta{
		Layer:  ArchLayerMiddleware,
		File:   "server/internal/presentation/middleware.go",
		Symbol: "Chain(RequestID → RequestLogger → Recover → EnvelopeMuxErrors)",
		Note: "the documented order: the request id is stamped outermost so recovery, " +
			"logging, and every error envelope share it; this hop's clock is the whole " +
			"traced call, middleware included",
	}
	// ArchHandlerHop describes the thin handler — the topic's parse /
	// validate / delegate / map discipline in one type.
	ArchHandlerHop = ArchHopMeta{
		Layer:  ArchLayerPresentation,
		File:   "server/internal/presentation/posts.go",
		Symbol: "PostsListHandler.ServeHTTP",
		Note: "parses the query string, normalizes and validates it through the domain's " +
			"own NormalizeListQuery, calls exactly ONE use case, and maps the page onto " +
			"JSON — no rules of its own, no knowledge that storage exists",
	}
	// ArchDomainHop describes the use case — recorded at the repository
	// port boundary it delegates over.
	ArchDomainHop = ArchHopMeta{
		Layer:  ArchLayerDomain,
		File:   "server/internal/domain/posts/usecases.go",
		Symbol: "ListPostsUseCase.Execute",
		Note: "the use case's Execute is a one-line delegation over the port it owns; the " +
			"recorder sits at that boundary, so this hop's cost is the domain's call as " +
			"it left the use case — inclusive of everything the port did beneath it",
	}
	// ArchDataInMemoryHop describes the fake leaf — the in-memory adapter
	// the switching facade picks while the scenario says fake.
	ArchDataInMemoryHop = ArchHopMeta{
		Layer:  ArchLayerData,
		File:   "server/internal/data/posts/inmemory.go",
		Symbol: "InMemoryRepository.List",
		Note: "the switching facade picked the in-memory adapter from the scenario " +
			"snapshot — the leaf that actually answered, observed by the recorder " +
			"sitting between the facade and this store",
	}
	// ArchDataPostgresHop describes the live leaf — the pgx adapter the
	// switching facade picks while the scenario says live.
	ArchDataPostgresHop = ArchHopMeta{
		Layer:  ArchLayerData,
		File:   "server/internal/data/posts/pg.go",
		Symbol: "PostgresRepository.List",
		Note: "the switching facade picked the PostgreSQL adapter from the scenario " +
			"snapshot — the leaf that actually answered: one transaction holding the " +
			"totals and the page on a single snapshot, all failures typed 503",
	}
)

// ArchTraceHop is one stop on the timeline as the wire renders it: the
// seam's layer, file, and symbol, the teaching note, and the inclusive
// duration measured at that seam. Order is 1-based, outermost first.
type ArchTraceHop struct {
	Order      int     `json:"order"`
	Layer      string  `json:"layer"`
	File       string  `json:"file"`
	Symbol     string  `json:"symbol"`
	Note       string  `json:"note"`
	DurationMs float64 `json:"durationMs"`
}

// ArchTraceSink collects the hops one traced call records. It is
// mutex-guarded (concurrent port calls record race-detector clean) and
// holds an injectable clock: production passes time.Now, tests pass a
// pacer so durations pin exactly. Hops append in COMPLETION order —
// the innermost seam returns first — and the builder reverses them
// into the outermost-first timeline.
type ArchTraceSink struct {
	mu   sync.Mutex
	hops []ArchTraceHop
	now  func() time.Time
}

// NewArchTraceSink builds a sink over the given clock (nil means
// time.Now — the production default).
func NewArchTraceSink(now func() time.Time) *ArchTraceSink {
	if now == nil {
		now = time.Now
	}
	return &ArchTraceSink{now: now}
}

// Now reads the sink's clock — the seam wrappers use it so test pacing
// and production wall time flow through one hole.
func (s *ArchTraceSink) Now() time.Time {
	return s.now()
}

// Record appends one completed hop: the seam's static meta plus the
// inclusive duration from the recorded start to now (microsecond
// precision — sub-millisecond hops render as fractions, not zeros).
func (s *ArchTraceSink) Record(meta ArchHopMeta, started time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.hops = append(s.hops, ArchTraceHop{
		Layer:      meta.Layer,
		File:       meta.File,
		Symbol:     meta.Symbol,
		Note:       meta.Note,
		DurationMs: float64(s.now().Sub(started).Microseconds()) / 1000,
	})
}

// Reset clears the recorded hops — the handler calls it before each
// trace so a sink wired once at the composition root serves every
// request a fresh timeline.
func (s *ArchTraceSink) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.hops = nil
}

// Hops returns a copy of the recorded hops in completion order — always
// non-nil, so an empty sink reads as [] on the wire.
func (s *ArchTraceSink) Hops() []ArchTraceHop {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append(make([]ArchTraceHop, 0, len(s.hops)), s.hops...)
}

// Compile-time proof the recorder satisfies the port it decorates.
var _ posts.PostsRepository = (*ArchTraceRecorder)(nil)

// ArchTraceRecorder decorates one posts.PostsRepository seam: every port
// call is timed and recorded into the sink, then delegated unchanged —
// arguments, results, and errors pass through untouched, so the traced
// pipeline behaves exactly like the unwired one. The recorder knows the
// PORT only (never an adapter type), which is what keeps it inside the
// domain: the composition root passes the adapters as interface values.
type ArchTraceRecorder struct {
	inner posts.PostsRepository
	meta  ArchHopMeta
	sink  *ArchTraceSink
}

// NewArchTraceRecorder wires a recorder over the port seam.
func NewArchTraceRecorder(inner posts.PostsRepository, meta ArchHopMeta, sink *ArchTraceSink) *ArchTraceRecorder {
	return &ArchTraceRecorder{inner: inner, meta: meta, sink: sink}
}

// List implements the port through the wrapped repository, recorded.
func (rec *ArchTraceRecorder) List(ctx context.Context, query posts.ListQuery) (posts.PostPage, error) {
	started := rec.sink.Now()
	page, err := rec.inner.List(ctx, query)
	rec.sink.Record(rec.meta, started)
	return page, err
}

// GetByID implements the port through the wrapped repository, recorded.
func (rec *ArchTraceRecorder) GetByID(ctx context.Context, id string) (posts.Post, error) {
	started := rec.sink.Now()
	post, err := rec.inner.GetByID(ctx, id)
	rec.sink.Record(rec.meta, started)
	return post, err
}

// Create implements the port through the wrapped repository, recorded.
func (rec *ArchTraceRecorder) Create(ctx context.Context, draft posts.Draft) (posts.Post, error) {
	started := rec.sink.Now()
	post, err := rec.inner.Create(ctx, draft)
	rec.sink.Record(rec.meta, started)
	return post, err
}

// Replace implements the port through the wrapped repository, recorded.
func (rec *ArchTraceRecorder) Replace(ctx context.Context, id string, draft posts.Draft) (posts.Post, error) {
	started := rec.sink.Now()
	post, err := rec.inner.Replace(ctx, id, draft)
	rec.sink.Record(rec.meta, started)
	return post, err
}

// Delete implements the port through the wrapped repository, recorded.
func (rec *ArchTraceRecorder) Delete(ctx context.Context, id string) error {
	started := rec.sink.Now()
	err := rec.inner.Delete(ctx, id)
	rec.sink.Record(rec.meta, started)
	return err
}

// ArchTracerFacts is everything the payload derives from, observed by
// the handler: the scenario's data mode at trace time, the hops the
// seams recorded, and what the traced request answered — status, item
// count, wall time, and the request id threaded through the walk.
type ArchTracerFacts struct {
	DataMode  core.DataMode
	Hops      []ArchTraceHop
	Status    int
	Items     int
	Elapsed   time.Duration
	RequestID string
}

// ArchTracePanel is the timeline the client draws: the marker request,
// the mode and the leaf that answered, how to flip it, the hops
// outermost-first, and what the traced call answered.
type ArchTracePanel struct {
	Request   string         `json:"request"`
	DataMode  string         `json:"dataMode"`
	Leaf      string         `json:"leaf"`
	FlipNote  string         `json:"flipNote"`
	Hops      []ArchTraceHop `json:"hops"`
	Status    int            `json:"status"`
	Items     int            `json:"items"`
	ElapsedMs float64        `json:"elapsedMs"`
	RequestID string         `json:"requestId"`
	Note      string         `json:"note"`
}

// ArchViolationPanel is the counterexample: the dependency-rule
// violation as a clearly-labeled non-compiling exhibit — what a
// handler importing the data layer would look like, why it is wrong,
// and what the repo's honest state is.
type ArchViolationPanel struct {
	Label    string `json:"label"`
	Snippet  string `json:"snippet"`
	Why      string `json:"why"`
	CaughtBy string `json:"caughtBy"`
	Honest   string `json:"honest"`
}

// ArchTracerPayload is the wire shape of GET /api/demo/arch/tracer.
// Collections are non-nil so the wire renders [] never null.
type ArchTracerPayload struct {
	Trace     ArchTracePanel     `json:"trace"`
	Violation ArchViolationPanel `json:"violation"`
	Reasoning string             `json:"reasoning"`
}

// ArchViolationSnippet is the counterexample the panel shows: what a
// dependency-rule violation WOULD look like — a handler reaching past
// the domain straight into the data layer, SQL and all. It is DATA: a
// string constant in this builder, quoted by the topic with this file
// as its snippet source, never compiled, never wired, and not
// importable as code by anything. The raw string opens on its own line
// so every exhibit line is a real line of this file — that is what
// lets the topic quote the counterexample under the same snippet
// discipline as real code.
const ArchViolationSnippet = `
func (h *BadPostsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	tag := r.URL.Query().Get("tag")
	rows, err := h.pool.Query(r.Context(),
		"SELECT id, title, body FROM posts p WHERE EXISTS ("+
			"SELECT 1 FROM post_tags t WHERE t.post_id = p.id AND t.tag_name = $1)", tag)
	if err != nil {
		http.Error(w, "database unavailable", 500)
		return
	}
	defer rows.Close()
	// …the row scanning, the paging math, the sort, and the JSON shaping
	// all inline in the handler — every rule the domain owns, bypassed,
	// and a second store dialect for every handler that follows
}`

// BuildArchTracerPayload shapes the panel (pure) from observed facts:
// the sink recorded the hops in completion order (innermost seam
// returns first), so the timeline is the reverse — outermost first,
// numbered 1..n. The leaf label derives from whichever data hop ran,
// never from the mode string alone: mode and leaf are two observations
// that agree, and the note says so.
func BuildArchTracerPayload(facts ArchTracerFacts) ArchTracerPayload {
	ordered := make([]ArchTraceHop, 0, len(facts.Hops))
	for i := len(facts.Hops) - 1; i >= 0; i-- {
		hop := facts.Hops[i]
		hop.Order = len(facts.Hops) - i
		ordered = append(ordered, hop)
	}

	leaf := "no data hop recorded — the traced wiring always has one, so this is the honest-empty case"
	for _, hop := range ordered {
		if hop.Layer == ArchLayerData {
			leaf = hop.Symbol + " — " + hop.File
			break
		}
	}

	mode := string(facts.DataMode)
	flipNote := "PATCH " + ArchTracerScenarioPath + ` {"dataMode":"live"} flips every posts ` +
		"surface at once (REST, GraphQL, gRPC, and this tracer share the switching " +
		"facade) — reload the tracer and the SAME request walks the same shape with " +
		"a different leaf hop"

	return ArchTracerPayload{
		Trace: ArchTracePanel{
			Request:   ArchTracerRequestLine,
			DataMode:  mode,
			Leaf:      leaf,
			FlipNote:  flipNote,
			Hops:      ordered,
			Status:    facts.Status,
			Items:     facts.Items,
			ElapsedMs: float64(facts.Elapsed.Microseconds()) / 1000,
			RequestID: facts.RequestID,
			Note: "every hop above is an execution that just happened inside this process: " +
				"the traced request is the real posts list served by the real handler over " +
				"the real use case and switching repository, with recorders at the seams. " +
				"Durations are inclusive — each layer's clock runs while the layers beneath " +
				"it work — which is why the numbers narrow as they descend: the near-equality " +
				"of the domain and data hops is the thin-delegation evidence, visible as a " +
				"measurement. The leaf and the dataMode are two independent observations " +
				"that agree: the recorder under the facade saw which adapter answered",
		},
		Violation: ArchViolationPanel{
			Label:   "counterexample — not code this repo compiles",
			Snippet: strings.TrimSpace(ArchViolationSnippet),
			Why: "the handler owns SQL, a pool, paging math, and a raw 500 with a string " +
				"body — four jobs the architecture gives to four places. The domain's rules " +
				"are bypassed (no normalization, no typed errors, no validation), the " +
				"envelope contract is broken (http.Error writes plain text the client's " +
				"error pipeline cannot parse), and every new posts handler must re-learn " +
				"the store's dialect: the cost of the violation is the NEXT handler",
			CaughtBy: "the import graph: a presentation file importing guide/internal/data is " +
				"one grep away (`grep -rn 'guide/internal/data' internal/presentation/` — " +
				"zero production hits today), review enforces the rule, and the guide's " +
				"one deliberate exemption is marked in the file that uses it",
			Honest: "this repo's actual state: no production presentation file imports the " +
				"data layer — the only marked teaching-exception in the guide is " +
				"internal/domain/demos/grpc_console.go, a domain exhibit that constructs " +
				"an in-process gRPC stack and says so in its header. Tests import adapters " +
				"freely (posts_test.go wires the in-memory fake): the rule bounds " +
				"production code paths, not test wiring",
		},
		Reasoning: "One request, four layers, every hop measured where it ran: middleware → " +
			"thin handler → use case → the adapter the scenario picked. The timeline is the " +
			"dependency rule executed — presentation and data never touch, the domain sits " +
			"between them owning the port, and the counterexample panel shows what the " +
			"rule costs when it breaks. Flip the scenario and the same trace re-runs with a " +
			"different leaf: the layers above the port never learn the store changed",
	}
}
