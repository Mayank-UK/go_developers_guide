package content

import (
	"errors"
	"fmt"
)

// Topic 05 interfaces-and-composition — the GO-NEW topic with no family
// counterpart. It teaches the discipline this whole server is built on:
// composition over inheritance, small interfaces, embedding as the
// inheritance replacement, accept-interfaces/return-structs, type switches,
// and generics restraint. Every code block quotes a real repo file (the
// snippet ratchet verifies it); the live demo — the satisfaction matrix —
// sits INLINE in the section that teaches satisfaction and typing; the
// terminal companion closes the topic.

// interfacesCompositionTopic builds topic 05. Prose is the default block;
// one paragraph per text block; lists only where the content is genuinely
// parallel; the builder follows the exemplar's errors.Join closure pattern
// so the block list below reads as pure content.
func interfacesCompositionTopic() (Topic, error) {
	var buildErr error
	collect := func(what string, err error) {
		if err != nil {
			buildErr = errors.Join(buildErr, fmt.Errorf("%s: %w", what, err))
		}
	}
	text := func(id, body string) TextBlock {
		block, err := NewTextBlock(id, body)
		collect("text "+id, err)
		return block
	}
	code := func(id, language, caption, source, snippet string) CodeBlock {
		block, err := NewCodeBlock(id, language, caption, source, snippet)
		collect("code "+id, err)
		return block
	}
	demo := func(id, title, description string) DemoBlock {
		block, err := NewDemoBlock(id, title, description)
		collect("demo "+id, err)
		return block
	}
	list := func(id string, ordered bool, items ...string) ListBlock {
		block, err := NewListBlock(id, ordered, items)
		collect("list "+id, err)
		return block
	}
	section := func(id, title, lead string, blocks ...Block) Section {
		assembled, err := NewSection(id, title, lead, blocks)
		collect("section "+id, err)
		return assembled
	}

	sections := []Section{
		// ── 1. Composition over inheritance ─────────────────────────────
		section("composition-over-inheritance", "Composition over inheritance",
			"Why Go deleted inheritance outright, and what this server does instead.",
			text("comp-pitch",
				"Go has no inheritance. No classes, no subclassing, no ``extends``, no "+
					"method overriding through a super chain, no protected fields visible only "+
					"to descendants — the entire apparatus is absent from the language spec. "+
					"That is not an omission waiting to be filled by a framework; it is a "+
					"deliberate refusal, made on the evidence of large C++ and Java codebases "+
					"where deep type hierarchies coupled modules that never needed to know "+
					"about each other. Go's replacement runs on two axes that this topic "+
					"walks in order: BEHAVIOR composes through interfaces and function "+
					"values — small capability contracts satisfied implicitly — and STATE "+
					"composes through embedding, promotion of fields and methods from "+
					"nested types. Neither axis has a taxonomy. The question a Go design "+
					"asks is never ``what IS this thing`` but ``what can this thing DO`` — "+
					"and the whole architecture of this guide server is the answer."),
			text("comp-middleware-intro",
				"Composition in the miniature: the middleware chain. Cross-cutting behavior "+
					"— request ids, access logging, panic recovery, error envelopes — is not a "+
					"base class handlers inherit from, nor an interceptor registry a framework "+
					"reflects over. It is a defined FUNCTION type that wraps one handler in "+
					"another, and a ``Chain`` function that folds any number of them "+
					"innermost-first. Every layer is an ordinary value; the order is an "+
					"argument list you can read in one place; nothing is virtual:"),
			code("comp-middleware-code", "go",
				"The whole middleware pattern: one function type, one fold.",
				"server/internal/presentation/middleware.go",
				"type Middleware func(http.Handler) http.Handler\n"+
					"// …\n"+
					"func Chain(middleware ...Middleware) Middleware {\n"+
					"    return func(final http.Handler) http.Handler {\n"+
					"        for i := len(middleware) - 1; i >= 0; i-- {\n"+
					"            final = middleware[i](final)\n"+
					"        }\n"+
					"        return final\n"+
					"    }\n"+
					"}"),
			text("comp-chain-read",
				"Read ``Chain`` closely — it is function composition with the loop running "+
					"BACKWARDS so the FIRST argument ends up the OUTERMOST layer: "+
					"``Chain(A, B, C)(handler)`` serves as ``A(B(C(handler)))``. The sibling "+
					"guides build the identical stack — express middleware arrays, spring "+
					"filter chains — but each of those needs a framework contract: a base "+
					"class to extend, an interface to implement, a registration mechanism "+
					"the runtime walks. Here the contract is ``http.Handler`` itself (one "+
					"method, next section) and the mechanism is a ``for`` loop. That is the "+
					"composition-over-inheritance trade in one screen: a little more typing "+
					"at the fold, and no framework owns your call order."),
			code("comp-root-code", "go",
				"The composition root: constructor injection, one route table, no DI framework.",
				"server/cmd/guide/main.go",
				"mux := http.NewServeMux()\n"+
					"mux.Handle(\"GET /api/health\", presentation.NewHealthHandler(cfg.Env))\n"+
					"mux.Handle(\"GET /api/content\", presentation.NewContentListHandler(registry))\n"+
					"mux.Handle(\"GET /api/content/{topicId}\", presentation.NewContentTopicHandler(registry, logger))\n"+
					"// …\n"+
					"mux.Handle(\"GET /api/demo/go/runtime-types\", presentation.NewRuntimeTypesHandler())\n"+
					"mux.Handle(\"GET /api/demo/runtime/concurrency\", presentation.NewConcurrencyHandler())"),
			text("comp-root-read",
				"The same choice scales up to the whole application. This server has no DI "+
					"framework and no service locator: ``cmd/guide/main.go`` is the single "+
					"composition root where every constructor is called with exactly what it "+
					"needs — config into the logger, registry into the content handlers, the "+
					"scenario store into its two handlers — and every route is one "+
					"``mux.Handle`` line. Notice what the mux accepts: ``NewHealthHandler`` "+
					"and ``NewContentListHandler`` return entirely different concrete types, "+
					"and the SAME registration call takes them all, because the one thing "+
					"they share is a method — ``ServeHTTP``. Polymorphism here is a "+
					"consequence of construction, not a hierarchy anyone declared."),
			text("comp-what-it-buys",
				"What the refusal buys, concretely. No fragile base class: changing "+
					"``statusRecorder`` in the next section cannot break code that embeds it "+
					"elsewhere, because nothing overrides across a boundary. No deep-coupled "+
					"taxonomies: every piece — middleware, handlers, stores, registries — is "+
					"constructible and testable in isolation with plain function calls and "+
					"plain fakes. Dependencies stay explicit: if a handler needs the scenario "+
					"store, it says so in its constructor, and there is no annotation-driven "+
					"magic to reverse-engineer. The Go community's shorthand for all of this "+
					"is ``composition over inheritance`` — four tools, no class tree:"),
			list("comp-tools", false,
				"Struct embedding — an unnamed field whose methods and fields are promoted "+
					"to the outer type; wrappers and mixins without subclassing.",
				"Interface embedding — an interface declared inside another interface (or a "+
					"struct); the whole method set arrives as one unit.",
				"Function values — first-class functions as swappable behavior; the "+
					"middleware type above IS one.",
				"Type parameters — generics, for algorithms whose shape is identical across "+
					"types; the last section draws their exact lane."),
		),
		// ── 2. Small interfaces ─────────────────────────────────────────
		section("small-interfaces", "Small interfaces: the io.Reader canon",
			"The method set as a unit of design — and why one method is usually enough.",
			text("small-pitch",
				"An interface is a METHOD SET — the list of signatures a value must carry "+
					"to be usable as that interface — and its size is a design decision "+
					"with consequences in every direction. Go's proverbs put it flatly: "+
					"``The bigger the interface, the weaker the abstraction.`` A one-method "+
					"interface is a single capability any type can grow; a ten-method "+
					"interface is a contract so heavy that only types designed against it "+
					"can ever sign. The canonical example is ``io.Reader`` — one method, "+
					"``Read([]byte) (int, error)`` — and through that single seam flows "+
					"half the standard library: files, network connections, buffers, "+
					"hashes, compression readers, ``os.Stdin``. None of those types knows "+
					"the others exist; the interface is the entire agreement. This "+
					"server's demo module carries the canon as one column of its matrix, "+
					"defined from the real reflect type rather than a stored string:"),
			code("small-reader-column", "go",
				"The io.Reader column of the satisfaction matrix, built from the live type.",
				"server/internal/domain/demos/interface_matrix.go",
				"{\n"+
					"    name: \"io.Reader\",\n"+
					"    typ:  reflect.TypeOf((*io.Reader)(nil)).Elem(),\n"+
					"    pkg:  \"io\",\n"+
					"    note: \"The canon: Read([]byte) (int, error) carries half the stdlib on its back.\",\n"+
					"},"),
			text("small-pointer-trick",
				"That ``reflect.TypeOf((*io.Reader)(nil)).Elem()`` line is an idiom worth "+
					"stopping on, because it answers a question every reflect user hits: "+
					"how do you get the ``reflect.Type`` OF AN INTERFACE? ``TypeOf`` takes "+
					"``any``, and passing a nil interface value yields a nil type — there "+
					"is no concrete value to describe. The trick manufactures one: "+
					"``(*io.Reader)(nil)`` is a TYPED nil — a non-nil interface holding a "+
					"nil POINTER — so ``TypeOf`` sees the pointer type, and ``.Elem()`` "+
					"steps from pointer to pointee, which is the interface type itself. "+
					"The same pattern opens any interface for inspection: method lists, "+
					"signatures, embeddeds — which is exactly how the demo renders every "+
					"column's method signature from the real type."),
			text("small-writejson-intro",
				"Small interfaces earn their keep at the boundaries between packages, "+
					"where satisfaction is IMPLICIT and therefore free. The repo's shared "+
					"success writer is one line of proof. Its parameter ``w`` is an "+
					"``http.ResponseWriter`` — a three-method interface from ``net/http`` "+
					"that never mentions ``io`` anywhere in its declaration. But "+
					"``json.NewEncoder`` demands an ``io.Writer``, one method, "+
					"``Write([]byte) (int, error)`` — and the call compiles, because "+
					"``ResponseWriter`` carries that method. Nobody wrote an adapter; the "+
					"method set did the adapting:"),
			code("small-writejson-code", "go",
				"w crosses the io.Writer boundary without an adapter in sight.",
				"server/internal/presentation/content.go",
				"func writeJSON(w http.ResponseWriter, status int, v any) {\n"+
					"    // …\n"+
					"    _ = json.NewEncoder(w).Encode(v)\n"+
					"}"),
			text("small-consumer",
				"The sharpest form of the small-interface discipline: define the interface "+
					"where it is CONSUMED, at the exact width of the need — even zero "+
					"names wide. The recovery middleware needs to know one thing about the "+
					"response writer it was handed: whether anything has been written yet "+
					"(an error envelope is only deliverable on a clean response). Rather "+
					"than depend on the concrete recorder type — which lives two files "+
					"away in the same package — it asserts an ANONYMOUS one-method "+
					"interface, spelled inline at the point of use, and proceeds only if "+
					"the writer happens to satisfy it:"),
			code("small-recover-code", "go",
				"A consumer-defined interface with no name: one method, declared at the point of use.",
				"server/internal/presentation/middleware.go",
				"if writer, ok := w.(interface{ Written() bool }); ok && writer.Written() {\n"+
					"    return\n"+
					"}"),
			text("small-pantheon",
				"Consumer-side definition inverts the usual object-oriented habit of "+
					"declaring interfaces next to the types that implement them. In Go the "+
					"implementer often never hears about the interface: ``*core.Error`` "+
					"was written for this guide's error taxonomy, and it satisfies "+
					"``error`` because it has the method — no import required, since "+
					"``error`` is builtin. The standard library's pantheon is almost "+
					"entirely one- and two-method contracts, and the guide's own demo "+
					"crosses five of them against real repo types:"),
			list("small-canons", false,
				"error — Error() string; the builtin everything returns, the whole "+
					"errors-as-values mechanism of topic 01 and topic 30.",
				"fmt.Stringer — String() string; one method fmt's entire formatting world "+
					"keys off, from %v to log lines.",
				"io.Reader / io.Writer — one method each; every byte that moves through a "+
					"Go program moves through one of them.",
				"http.Handler — ServeHTTP(ResponseWriter, *Request); the entire HTTP layer "+
					"of this server is this single signature.",
				"content.Block — blockKind() string; this repo's own one-method seal, "+
					"unexported so only package content can implement it.",
			),
		),
		// ── 3. Embedding ────────────────────────────────────────────────
		section("embedding", "Embedding: structs and interfaces",
			"Promotion in both directions — structs embedding interfaces, interfaces embedding interfaces.",
			text("embed-pitch",
				"Topic 01 met struct embedding as the field-promotion mechanic. This "+
					"section adds the half that makes it Go's inheritance replacement in "+
					"practice: embedding an INTERFACE inside a struct. The embedded "+
					"interface contributes its entire method set — every signature, "+
					"promoted — so the outer type satisfies the interface with ZERO "+
					"methods of its own, while an unnamed field holds whatever concrete "+
					"implementer was injected. That is the wrapping pattern: hold the "+
					"thing, forward to it for free, interpose only where you choose to. "+
					"The access-log recorder in this server's middleware is the "+
					"production instance, and its two bookkeeping fields ride along in "+
					"the same declaration:"),
			code("embed-recorder-code", "go",
				"Embedding an interface: the recorder IS a ResponseWriter, plus two counters.",
				"server/internal/presentation/middleware.go",
				"type statusRecorder struct {\n"+
					"    http.ResponseWriter\n"+
					"    status int\n"+
					"    wrote  bool\n"+
					"}"),
			text("embed-recorder-read",
				"Because the field is embedded, handlers below the logger cannot tell the "+
					"recorder from the real writer: ``Header``, ``Write``, ``WriteHeader`` "+
					"all arrive by promotion, so the recorder satisfies "+
					"``http.ResponseWriter`` without declaring it. Where the recorder "+
					"needs to interpose — capturing the status code, marking the response "+
					"started — it simply declares a method with the SAME NAME; the outer "+
					"method shadows the promoted one. This is Go's answer to overriding, "+
					"with one honest difference: there is no ``super`` and no dynamic "+
					"dispatch back out. The promoted original remains reachable by naming "+
					"the embedded field explicitly — the recorder's own ``WriteHeader`` "+
					"does exactly that when it calls ``r.ResponseWriter.WriteHeader`` "+
					"after recording. Shadowing is a namespace fact, never a virtual "+
					"call."),
			text("embed-flush-read",
				"One capability deserves special handling because it is NOT part of "+
					"``http.ResponseWriter`` at all: flushing. ``http.Flusher`` is a "+
					"separate one-method interface the underlying writer may or may not "+
					"satisfy — optional capability, expressed as an optional interface "+
					"rather than a flag. The recorder re-exposes flushing by asking its "+
					"inner writer with a comma-ok assertion — present, delegate; absent, "+
					"no-op. The type switch section below returns to the assertion "+
					"mechanics; here the design point is the composition: embed the "+
					"whole, forward the extras conditionally, never assume."),
			text("embed-named-intro",
				"The same package holds the deliberate counter-pattern, and the pair is "+
					"worth seeing side by side. The mux-error interceptor also wraps a "+
					"response writer — it buffers 404/405 bodies to decide whether to "+
					"replace them with the error envelope — but its writer field is "+
					"NAMED, not embedded, so nothing is promoted: every method it "+
					"forwards, it forwards by hand, and the delegation is visible on "+
					"every line. The choice is deliberate: the interceptor never passes "+
					"itself off as a full writer to the code above it — it intercepts "+
					"and finishes the response itself — so an explicit ``i.w`` hop "+
					"documents every crossing:"),
			code("embed-named-code", "go",
				"The named-field wrapper: no promotion, every forwarded method spelled out.",
				"server/internal/presentation/middleware.go",
				"type muxErrorInterceptor struct {\n"+
					"    w         http.ResponseWriter\n"+
					"    buffering bool\n"+
					"    decided   bool\n"+
					"    status    int\n"+
					"    buf       bytes.Buffer\n"+
					"}\n"+
					"\n"+
					"func (i *muxErrorInterceptor) Header() http.Header { return i.w.Header() }"),
			text("embed-promotion-intro",
				"The satisfaction matrix demo carries two promotion exhibits as real "+
					"repo types, one per direction of the trick — a struct embedding a "+
					"concrete POINTER, and a struct embedding an interface — plus the "+
					"third form taught below: interfaces embedding interfaces. Neither "+
					"exhibit declares a single method, and both satisfy their target "+
					"interface, facts the matrix computes live and the compiler proves "+
					"at build time:"),
			code("embed-promotion-code", "go",
				"Satisfaction by promotion: zero declared methods, two satisfied interfaces.",
				"server/internal/domain/demos/interface_matrix.go",
				"type taggedReader struct {\n"+
					"    *strings.Reader\n"+
					"    tag string\n"+
					"}\n"+
					"// …\n"+
					"type delegatingHandler struct {\n"+
					"    http.Handler\n"+
					"}"),
			text("embed-honesty",
				"What embedding is NOT, stated once more because the inheritance reflex "+
					"runs deep: it is not subtype polymorphism. Promotion is static — "+
					"the outer type's method set is fixed at compile time from its own "+
					"declarations plus everything embedded; an inner value never calls "+
					"back out to a shadowing outer method. The embedded field is also "+
					"just a field: a ``delegatingHandler`` built with no inner handler "+
					"assigned still satisfies ``http.Handler`` — satisfaction is a "+
					"compile-time fact about method sets — but calling its promoted "+
					"``ServeHTTP`` would panic on the nil embedded interface. The "+
					"compiler guarantees the SHAPE of the promise; the constructor "+
					"guarantees the VALUE behind it. Interfaces embed interfaces too: "+
					"``io.ReadWriter`` is nothing but ``Reader`` embedded in "+
					"``Writer`` — two one-method sides composing a two-method whole — "+
					"and the matrix's embedding face shows ``*bytes.Buffer`` satisfying "+
					"the composed whole with the same methods that satisfied each half."),
		),
		// ── 4. Accept interfaces, return structs ────────────────────────
		section("accept-interfaces-return-structs", "Accept interfaces, return structs",
			"The guideline that keeps Go APIs loose on the way in and rich on the way out.",
			text("air-pitch",
				"One guideline compresses most of Go API design: functions should ACCEPT "+
					"interfaces and RETURN concrete structs. The two halves have the same "+
					"reasoning underneath — maximize the caller's freedom. A parameter is "+
					"a DEMAND: every narrower demand widens the set of arguments that "+
					"work, so parameters want the smallest method set that suffices — "+
					"often one method, sometimes the anonymous inline kind the recovery "+
					"middleware used. A result is a PROMISE: returning an interface "+
					"forces every caller into the narrow view forever, while returning "+
					"the concrete type lets callers keep every method and decide for "+
					"themselves when to erase. This repo's registry constructor is the "+
					"production shape — plain slice parameters in, validated concrete "+
					"struct plus error out:"),
			code("air-newregistry-code", "go",
				"Slices in, struct out: the registry constructor's honest signature.",
				"server/internal/content/registry.go",
				"func NewRegistry(topics []Topic, groups []TopicGroup) (Registry, error) {\n"+
					"    if err := validateGroups(groups); err != nil {\n"+
					"        return Registry{}, err\n"+
					"    }\n"+
					"    // …\n"+
					"    return Registry{topics: sortedTopics, groups: sortedGroups, topicsByID: byID}, nil\n"+
					"}"),
			text("air-returns",
				"The returns side in practice: every constructor in this server hands back "+
					"a concrete type — ``Registry``, ``*ScenarioStore``, and the handler "+
					"below — even though every one of those types immediately satisfies "+
					"some useful interface (``Registry`` carries three value-receiver "+
					"methods into any interface that wants them; every handler satisfies "+
					"``http.Handler`` the moment it is passed to the mux). Callers keep "+
					"the full API and erase only when erasing pays:"),
			code("air-topic-handler-code", "go",
				"A concrete return that satisfies http.Handler the moment it needs to.",
				"server/internal/presentation/content.go",
				"type ContentTopicHandler struct {\n"+
					"    registry content.Registry\n"+
					"    logger   *slog.Logger\n"+
					"}\n"+
					"\n"+
					"// NewContentTopicHandler wires the validated content registry and the\n"+
					"// process logger (the central error writer logs with it).\n"+
					"func NewContentTopicHandler(registry content.Registry, logger *slog.Logger) *ContentTopicHandler {\n"+
					"    return &ContentTopicHandler{registry: registry, logger: logger}\n"+
					"}"),
			text("air-accept",
				"The accepts side is more subtle, and this codebase is honest about it: "+
					"most constructors here accept STRUCTS, not interfaces, because there "+
					"is exactly one production implementation and inventing a port for a "+
					"population of one is ceremony, not abstraction. The interfaces appear "+
					"at the TRUE polymorphism points, and they are unmistakable: "+
					"``mux.Handle`` takes any ``http.Handler``; ``json.NewEncoder`` takes "+
					"any ``io.Writer``; and the most-accepted interface in the entire "+
					"repo is ``context.Context`` — first parameter of anything that can "+
					"block, four methods, satisfied only by values the ``context`` "+
					"package itself constructs. User code never implements it; it "+
					"propagates it — cancellation and deadlines flowing down call chains "+
					"through a type designed to be passed, not subclassed."),
			text("air-exceptions",
				"The honest exceptions prove the guideline rather than break it. "+
					"``Chain`` accepts ``http.Handler`` and RETURNS ``http.Handler`` — an "+
					"interface result — because its entire job is polymorphism: it "+
					"composes handlers, so its output must be exactly what its inputs "+
					"are. ``writeJSON`` accepts ``v any`` — the empty interface, zero "+
					"methods — because encoding/json does its own reflection and the "+
					"writer genuinely cannot know more. Both are the pattern applied "+
					"with judgment rather than a rule applied blindly; the guideline "+
					"serves the design, never the reverse. Collected as four working "+
					"habits:"),
			list("air-rules", false,
				"Accept the narrowest type — the smallest method set that the body actually "+
					"uses; an anonymous one-method interface at the point of use is fair game.",
				"Return concrete structs — hand callers the whole API; let THEM choose the "+
					"interface moment.",
				"Define interfaces consumer-side — where the value is USED, at the width of "+
					"the need; implementers need not know.",
				"Reserve interface results for genuinely polymorphic jobs — composition "+
					"functions and true ports, not default habits.",
			),
		),
		// ── 5. Type switches and assertions ─────────────────────────────
		section("type-switches-assertions", "Type switches and assertions",
			"Opening the interface box: assertions, comma-ok, and the switch that binds each case.",
			text("ts-pitch",
				"An interface value is a BOX: a (dynamic type, concrete value) pair. The "+
					"static type says what you may ASK of the value; the dynamic type "+
					"says what the value IS. Assertions open the box. The single form "+
					"``x.(T)`` either yields the value as ``T`` or panics — reserve it "+
					"for when a miss means the program is wrong. The comma-ok form "+
					"``v, ok := x.(T)`` reports failure as ``false`` and is the everyday "+
					"choice. The type switch scales the assertion to many cases at once, "+
					"binding the value at its CONCRETE type inside each branch — and the "+
					"content model's block dispatcher is the canonical real one, running "+
					"on every block of every served topic:"),
			code("ts-blockidof-code", "go",
				"The registry's type switch: each block kind resolved by its concrete type.",
				"server/internal/content/model.go",
				"func blockIDOf(block Block) string {\n"+
					"    switch b := block.(type) {\n"+
					"    case TextBlock:\n"+
					"        return b.ID\n"+
					"    case CodeBlock:\n"+
					"        return b.ID\n"+
					"    case DemoBlock:\n"+
					"        return b.ID\n"+
					"    case ListBlock:\n"+
					"        return b.ID\n"+
					"    default:\n"+
					"        return \"\"\n"+
					"    }\n"+
					"}"),
			text("ts-blockidof-read",
				"Every case binds ``b`` at a different concrete type, so ``b.ID`` is "+
					"ordinary field access per branch — no casting, no reflection. The "+
					"sealed hierarchy from the small-interfaces section is what keeps "+
					"this switch exhaustive-able: ``Block`` demands an unexported "+
					"method, so the four block types are the complete population, and "+
					"the registry turns a ``default`` miss into a validation error at "+
					"startup rather than a silent skip at request time. A loud default "+
					"is the difference between a type switch you can reason about and "+
					"one that quietly eats the future."),
			text("ts-order-intro",
				"Case ORDER is semantics, not style: cases run top to bottom and the "+
					"FIRST match wins — which matters the moment interface cases appear "+
					"alongside concrete ones, because a value can satisfy several. The "+
					"demo's ``classify`` switch interleaves them deliberately: two "+
					"concrete cases first, then ``io.Reader``, then ``fmt.Stringer``, "+
					"then default. Hand it a ``*bytes.Buffer`` — which satisfies BOTH "+
					"interface cases — and it lands in ``io.Reader`` purely by order; "+
					"the matrix's type-switch face pins that verdict as data:"),
			code("ts-classify-code", "go",
				"Concrete cases first, interface cases after, default loud — order decides.",
				"server/internal/domain/demos/interface_matrix.go",
				"func classify(v any) string {\n"+
					"    switch x := v.(type) {\n"+
					"    case content.TextBlock:\n"+
					"        return \"text block \" + x.ID\n"+
					"    case *core.Error:\n"+
					"        return fmt.Sprintf(\"error %s: %s\", x.Kind, x.Message)\n"+
					"    case io.Reader:\n"+
					"        return fmt.Sprintf(\"io.Reader case — dynamic type %T\", x)\n"+
					"    case fmt.Stringer:\n"+
					"        return \"fmt.Stringer case — \" + x.String()\n"+
					"    default:\n"+
					"        return fmt.Sprintf(\"unhandled %T\", x)\n"+
					"    }\n"+
					"}"),
			text("ts-commaok",
				"The comma-ok assertion in the wild is usually about OPTIONAL "+
					"capability: asking a value whether it happens to carry more than "+
					"its static type promised. The recorder's flush, quoted in the "+
					"embedding section, asks its inner writer for ``http.Flusher`` and "+
					"delegates only on a hit. The same shape reads request ids out of "+
					"context (a ``string`` that may be absent), and the errors machinery "+
					"of topic 30 generalizes it: ``errors.As`` walks a whole wrap chain "+
					"looking for a concrete type, succeeding only if some layer of the "+
					"error is the one you asked for — a type assertion with a memory. "+
					"Reach for the PANICKING single form only when the miss is a "+
					"programmer bug; the comma-ok form is the one production code "+
					"deserves:"),
			code("ts-flush-code", "go",
				"Comma-ok as optional capability: flush only if the inner writer can.",
				"server/internal/presentation/middleware.go",
				"if flusher, ok := r.ResponseWriter.(http.Flusher); ok {\n"+
					"    flusher.Flush()\n"+
					"}"),
		),
		// ── 6. The satisfaction matrix (inline demo) ────────────────────
		section("satisfaction-matrix", "The satisfaction matrix",
			"This topic's demo lives here — every verdict computed from real method sets.",
			text("mx-pitch",
				"Everything this topic has claimed is a compile-time fact about method "+
					"sets — so let the running server compute them. The demo endpoint "+
					"crosses five interfaces (``fmt.Stringer``, ``error``, "+
					"``io.Reader``, ``http.Handler``, and this repo's own sealed "+
					"``content.Block``) against eleven real types from this codebase "+
					"and the standard library, and every ✓/✗ in the served matrix is "+
					"computed live with reflect's ``Implements`` against the ACTUAL "+
					"method sets — nothing authored, nothing hand-checked. The same "+
					"facts are pinned a second way, at build time, by the ``var _`` "+
					"idiom: blank-identifier assignments the compiler must verify and "+
					"the binary never carries:"),
			code("mx-varproofs-code", "go",
				"Compile-time proofs: each line pins one TRUE cell of the matrix.",
				"server/internal/domain/demos/interface_matrix.go",
				"var (\n"+
					"    _ fmt.Stringer  = time.Duration(0)\n"+
					"    _ error         = (*core.Error)(nil)\n"+
					"    _ io.Reader     = (*bytes.Buffer)(nil)\n"+
					"    _ io.ReadWriter = (*bytes.Buffer)(nil)\n"+
					"    _ io.Reader     = (*strings.Reader)(nil)\n"+
					"    _ http.Handler  = http.HandlerFunc(nil)\n"+
					"    _ content.Block = content.TextBlock{}\n"+
					"    _ content.Block = content.CodeBlock{}\n"+
					"    _ io.Reader     = taggedReader{}\n"+
					"    _ http.Handler  = delegatingHandler{}\n"+
					")"),
			text("mx-varproofs-read",
				"``var _ Iface = Type{}`` is the cheapest possible regression test: a "+
					"package-level assignment to the blank identifier, costing zero "+
					"memory and zero runtime, that fails the BUILD the moment the "+
					"type stops satisfying the interface — a renamed method, a flipped "+
					"receiver, a new required capability. The demo pairs the two "+
					"witnesses on purpose: the ``var _`` block holds the facts where "+
					"the compiler checks them, reflect computes the same facts where "+
					"the API serves them, and the module's tests cross-check that the "+
					"two never drift apart."),
			code("mx-row-code", "go",
				"One matrix row: type, kind, exported method set, one live verdict per column.",
				"server/internal/domain/demos/interface_matrix.go",
				"func satisfactionRow(expression string, value any, cols []matrixColumn) SatisfactionRow {\n"+
					"    typ := reflect.TypeOf(value)\n"+
					"    methods := make([]string, 0, typ.NumMethod())\n"+
					"    for i := 0; i < typ.NumMethod(); i++ {\n"+
					"        methods = append(methods, typ.Method(i).Name)\n"+
					"    }\n"+
					"    sort.Strings(methods)\n"+
					"    cells := make([]MatrixCell, 0, len(cols))\n"+
					"    for _, col := range cols {\n"+
					"        cells = append(cells, MatrixCell{Iface: col.name, Satisfied: typ.Implements(col.typ)})\n"+
					"    }"),
			text("mx-rows-read",
				"Two rows repay close reading. The error row appears TWICE — as "+
					"``*core.Error`` and as ``core.Error`` — and only the pointer row "+
					"satisfies ``error``, because every method on the type takes a "+
					"pointer receiver, so the VALUE's method set is empty: the single "+
					"most common interface bug in Go, rendered as a side-by-side "+
					"contrast. And the ``content.TextBlock`` row satisfies "+
					"``content.Block`` while showing an EMPTY method list — reflect "+
					"can only list EXPORTED methods, ``blockKind`` is unexported, and "+
					"satisfaction still holds because ``Implements`` checks the full "+
					"method set. The seal that guards the hierarchy is the same fact "+
					"that blinds the listing; the juxtaposition is the lesson. Run "+
					"the demo and read it in five faces:"),
			demo("iface:matrix", "The satisfaction matrix — computed, not authored",
				"Five interfaces crossed with eleven real types, every verdict computed "+
					"live from actual method sets via reflect: the pointer-vs-value error "+
					"pair, the sealed Block satisfied while listing zero exported methods, "+
					"three embedding exhibits satisfied purely by promotion, a type switch "+
					"whose case order decides which of two matching cases wins, and the "+
					"ten var _ compile proofs that pin the same cells at build time."),
			list("mx-beats", false,
				"Interfaces — the five columns with their single methods rendered from the "+
					"real reflect types, from fmt.Stringer to the repo's own content.Block.",
				"Rows — eleven expressions, each with type, kind, exported method set, and "+
					"one verdict per column; find the *core.Error/core.Error pair.",
				"Embeddings — taggedReader, delegatingHandler, and the composed "+
					"io.ReadWriter: satisfaction by promotion, verified by the same check.",
				"Type switch — six inputs through classify, including the *bytes.Buffer "+
					"row that matches two cases and settles by order.",
				"Compile proofs — the var _ lines as data, each naming the matrix cell it "+
					"pins, cross-checked by the module's tests."),
			text("mx-coda",
				"The payload is pure — no inputs, no I/O, same call same bytes — so the "+
					"matrix is a snapshot of THIS codebase at THIS commit: if a refactor "+
					"changes any method set, the demo's own tests fail next to the claim "+
					"the topic makes. That is the honest version of a satisfaction "+
					"table: not documentation that can rot, but a computation that can "+
					"only drift loudly."),
		),
		// ── 7. Generics done right ──────────────────────────────────────
		section("generics-done-right", "Generics: done right, and when not",
			"Type parameters: the right tool for a narrow set of jobs.",
			text("gen2-pitch",
				"Topic 01 met the mechanics — type parameters, constraints, inference. "+
					"This section draws the LANE. Generics pay when the SHAPE of an "+
					"algorithm or container is identical across types and the only "+
					"alternative is copy-paste per type or erasing to ``any`` and losing "+
					"the checker: filters, maps, sets, sorting, worker pools over any "+
					"payload. The guide's teaching exhibit holds the canonical instance "+
					"— a filter whose body never mentions a concrete type yet type-checks "+
					"at every call site, with the predicate injected as a plain function "+
					"value (behavior composing with generics, the topic's two threads in "+
					"one signature):"),
			code("gen2-filter-code", "go",
				"The canonical payoff: one body, every element type, checked at every call.",
				"server/internal/domain/teaching/generics.go",
				"func Filter[T any](in []T, keep func(T) bool) []T {\n"+
					"    out := make([]T, 0, len(in))\n"+
					"    for _, value := range in {\n"+
					"        if keep(value) {\n"+
					"            out = append(out, value)\n"+
					"        }\n"+
					"    }\n"+
					"    return out\n"+
					"}"),
			text("gen2-stdlib",
				"Before writing any generic of your own, the standard library — Go 1.21 "+
					"grew the ``slices``, ``maps``, and ``cmp`` packages precisely so "+
					"most code never needs to. ``cmp.Ordered`` is the ready-made "+
					"constraint for anything comparable; the exhibit's ``Max`` is one "+
					"line of body over it and exists to teach exactly this: reach for "+
					"the stdlib's constraint vocabulary first. The census of THIS "+
					"server's production code is the honest footnote: it CALLS generic "+
					"stdlib on real duty and defines zero production generics of its "+
					"own — every ``[T any]`` in the module lives in the teaching "+
					"package, quarantined there because the topics quote it:"),
			code("gen2-max-code", "go",
				"cmp.Ordered: the stdlib's constraint, before you write your own.",
				"server/internal/domain/teaching/generics.go",
				"func Max[T cmp.Ordered](a, b T) T {\n"+
					"    if a > b {\n"+
					"        return a\n"+
					"    }\n"+
					"    return b\n"+
					"}"),
			code("gen2-contains-code", "go",
				"Generic stdlib on production duty: one call, invisible inference.",
				"server/internal/presentation/middleware.go",
				"canned := slices.Contains(muxCannedBodies[i.status], i.buf.String())\n"+
					"if !canned {\n"+
					"    i.w.WriteHeader(i.status)\n"+
					"    _, _ = i.w.Write(i.buf.Bytes())\n"+
					"    return\n"+
					"}"),
			text("gen2-when-not",
				"And when NOT to reach for type parameters — the restraint matters more "+
					"than the power. First: if an interface with one method does the "+
					"job, prefer the interface. Satisfaction handles heterogeneous "+
					"VALUES — one slice of ``Block`` holding four concrete types, the "+
					"type switch waiting underneath — while type parameters handle "+
					"homogeneous SHAPES, one element type flowing through one body. "+
					"Choosing generics for the heterogeneous case buys you nothing but "+
					"a type switch you must write anyway. Second: never wrap a concrete "+
					"function nobody duplicated — a generic shell around one caller is "+
					"complexity on credit. Third: start concrete and abstract at the "+
					"second or third duplication, when the invariant shape is actually "+
					"visible. The stdlib itself followed exactly this path — "+
					"``sort.Slice`` with its interface-and-callback design ran for a "+
					"decade before ``slices.Sort`` replaced it with the generic that "+
					"the whole ecosystem's experience had justified."),
			text("gen2-coda",
				"The working heuristic, in one line: reach for an interface first, the "+
					"stdlib's generics second, and your own type parameter third — and "+
					"only when the shape is proven by duplication. This server is the "+
					"census in miniature: dozens of small interfaces doing real work on "+
					"every request path, one generic stdlib call in the middleware, and "+
					"a teaching package that exists because this topic needed an honest "+
					"exhibit — not because the production code was hurting."),
		),
		// ── 8. Terminal companion ───────────────────────────────────────
		section("terminal-companion", "Terminal companion: the curl tour",
			"The guide is its own API — read this topic from the shell.",
			text("term-pitch",
				"With the stack up (server on :8100), these commands walk the exact "+
					"surface this topic lives in — the content that teaches it, the demo "+
					"endpoint that computes it, and the envelope every failure rides. "+
					"``python3 -m json.tool`` keeps the tour dependency-free; the "+
					"one-liner near the end distills the matrix to its verdicts:"),
			code("term-topic", "bash", "", "",
				"# this topic, in full: sections → text/code/list/demo blocks\n"+
					"curl -s localhost:8100/api/content/interfaces-and-composition | python3 -m json.tool"),
			code("term-demo", "bash", "", "",
				"# the satisfaction matrix — every verdict computed live\n"+
					"curl -s localhost:8100/api/demo/iface/matrix | python3 -m json.tool"),
			code("term-cells", "bash", "", "",
				"# no jq required: each row's SATISFIED columns only\n"+
					"python3 -c \"import json,urllib.request as u; d=json.load(u.urlopen('http://localhost:8100/api/demo/iface/matrix')); [print(r['expression'], '→', ', '.join(c['iface'] for c in r['cells'] if c['satisfied']) or 'none') for r in d['rows']]\""),
			code("term-405", "bash", "", "",
				"# the demo route is method-guarded: POST draws the 405 envelope + Allow\n"+
					"curl -s -i -X POST localhost:8100/api/demo/iface/matrix"),
			code("term-404", "bash", "", "",
				"# unknown topic → the shared error envelope + x-request-id\n"+
					"curl -s -i localhost:8100/api/content/nope"),
			text("term-coda",
				"Every failure you can provoke comes back in one envelope — "+
					"{error: {code, message, requestId}} — with an x-request-id header "+
					"matching the server logs; the taxonomy behind it gets its deep dive "+
					"in topic 30. The matrix endpoint is deliberately pure: curl it "+
					"twice and the bytes match — a satisfaction table you can diff."),
		),
	}

	if buildErr != nil {
		return Topic{}, buildErr
	}
	return NewTopic(
		"interfaces-and-composition",
		"Interfaces and composition",
		"Go's answer to polymorphism: small interfaces satisfied implicitly, "+
			"embedding as the inheritance replacement, and the discipline of "+
			"accepting interfaces while returning structs.",
		"framework",
		5,
		sections,
	)
}
