package content

import (
	"errors"
	"fmt"
)

// Topic modules: one file per topic, each exporting a package-private
// builder (goLanguageTopic here) that assembles the typed model through
// the constructors in model.go. NewProductionRegistry in registry.go lists
// every builder — a topic not listed there is not served. The builders
// live IN package content (not a topics/ subpackage) because they build
// content's own types: a subpackage importing content while content
// imports it for registration would be an import cycle.
//
// This file is topic 01 go-language, the guide's EXEMPLAR topic: the
// authoring pattern later topics copy. Every code block quotes a real
// repo file (the snippet ratchet verifies it); prose is the default
// block; the live demo sits INLINE in the section that teaches it; the
// topic closes with the terminal companion.

// goLanguageTopic builds topic 01 — the language tour this server
// actually uses, self-contained: it assumes no prior Go (and no
// programming fundamentals beyond "a program runs statements in order")
// and teaches the language from variables up to generics, then closes on
// what types mean at runtime — the subject of the topic's inline demo.
func goLanguageTopic() (Topic, error) {
	// Authoring helpers: constructor errors are collected (not swallowed)
	// and surfaced once, so the block list below reads as pure content.
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
		// ── 1. Why Go on the server ─────────────────────────────────────
		section("why-go", "Why Go on the server", "",
			text("why-go-pitch",
				"Go is a compiled, statically typed, garbage-collected language born at Google "+
					"from a simple complaint: large codebases were slow to build, slow to read, and "+
					"slow to onboard engineers into. Its server-side pitch follows from that origin. "+
					"The compiler is fast enough to feel like an interpreter while still rejecting "+
					"wrong-typed code before it ever runs. Deployment is a single static binary — "+
					"no interpreter to install, no runtime dependencies to drift, a container image "+
					"measured in megabytes. And concurrency is not a library bolted on top but a "+
					"language feature: goroutines, cheap enough that an HTTP server spawns one per "+
					"request without ceremony (topic 06 is that whole story)."),
			code("why-go-module", "mod", "The whole server, dependency-wise.",
				"server/go.mod",
				"module guide\n"+
					"\n"+
					"go 1.27.3"),
			text("why-go-stdlib",
				"That manifest is the honest advertisement: this entire guide server runs on the "+
					"Go standard library and nothing else. ``net/http`` is the web server (routing, "+
					"TLS, timeouts), ``encoding/json`` the serializer, ``log/slog`` the structured "+
					"logger, ``testing`` plus ``httptest`` the test framework. The framework "+
					"question — chi, gin, echo — is taught by comparison in topic 04, not adopted, "+
					"because the stdlib's ``ServeMux`` has been a competent router since Go 1.22 "+
					"added method-and-path patterns like ``GET /api/content/{topicId}``. The "+
					"toolchain is equally uniform: ``gofmt`` ends formatting debates (there is one "+
					"way code looks), ``go vet`` catches suspect constructs, ``go test -race`` runs "+
					"the race detector as a first-class citizen, and ``go build`` produces the "+
					"binary. One language, one toolchain, zero configuration files to hold it together."),
			text("why-go-tradeoffs",
				"Where Go asks a price, stated plainly:"),
			list("why-go-costs-list", false,
				"Error handling is explicit — every call that can fail hands you an error value "+
					"you check by hand; the errors section shows why that becomes a feature.",
				"No inheritance and no overloading — composition and plain names only; the "+
					"language refuses clever taxonomies, and topic 05 turns that refusal into an "+
					"architecture.",
				"A younger ecosystem — the stdlib covers HTTP, JSON, and logging well, but niche "+
					"needs reach further than Python's PyPI or JavaScript's npm.",
				"The language still moves — generics landed in 1.18, loop-variable semantics and "+
					"ServeMux patterns in 1.22; older code in the wild reads slightly differently."),
			text("why-go-io-bound",
				"For I/O-bound API servers — a request arrives, the handler waits on PostgreSQL or "+
					"Redis, a response leaves — Go is close to the sweet spot. Goroutine stacks "+
					"start around two kilobytes and grow on demand, so ten thousand concurrent "+
					"waiting requests are routine, not an engineering project; the scheduler that "+
					"makes that cheap is topic 02. The CPU-bound caveat from every language applies "+
					"here too: sustained number crunching belongs in optimized libraries, not naive "+
					"loops — but servers spend their lives waiting, which is exactly the shape Go "+
					"was drawn around."),
			text("why-go-tour-map",
				"The rest of this topic is the language tour in the order this server uses it: "+
					"the bare-language basics — variables, types, control flow, loops, functions — "+
					"then the core data structures and strings, structs with methods and embedding, "+
					"interfaces, pointers versus values, errors as values, the package system, and "+
					"generics. Then the punchline: what those types actually ARE at runtime — the "+
					"subject of this topic's live demo — and the terminal companion that walks the "+
					"running server from the shell."),
		),
		// ── 2. Go basics: variables, types, control flow, loops, functions
		section("go-basics", "Go basics: variables, types, control flow, loops, functions",
			"The tour assumes no prior Go — every mechanic below is one this server actually uses.",
			text("basics-pitch",
				"Every Go file opens with a package clause and lives in exactly one package; "+
					"execution of a program starts at ``main()`` in package ``main`` (that is "+
					"``cmd/guide/main.go`` here). Statements end in newlines, not semicolons — the "+
					"compiler inserts them — and braces are mandatory, always on the same line. "+
					"Types come AFTER names: ``id string``, not ``string id`` — declarations read "+
					"aloud as ``id is a string``, and the habit becomes second nature within a "+
					"day. The two declaration forms matter most: ``var id string`` declares "+
					"without initializing (and a variable is NEVER uninitialized — more on zero "+
					"values below), while ``id := value`` declares AND infers in one move, the "+
					"form you will type thousands of times. Inside an ``if`` statement the short "+
					"form gains a superpower: the declared variable exists only inside the "+
					"statement's branches, which keeps the check and the use inseparable:"),
			code("basics-short-decl", "go",
				"Declare-and-scope in one stroke: raw lives only inside the if.",
				"server/internal/core/config.go",
				"if raw := strings.TrimSpace(getenv(envEnv)); raw != \"\" {\n"+
					"    cfg.Env = raw\n"+
					"}"),
			text("basics-const",
				"Constants are compile-time fixed values, declared in parenthesized groups, and "+
					"Go's constants are UNTYPED until used: ``defaultPort = 8100`` is not an int, "+
					"it is an untyped integer constant that flows into any integer-shaped context "+
					"— which is why the config can compare it or assign it without conversion "+
					"noise. Untyped constants make numeric code conspicuously clean; the day you "+
					"need the type fixed, ``const timeout time.Duration = 5 * time.Second`` pins "+
					"it. Names follow the convention you see here: lowercase for package-private "+
					"(``envPort``), and gofmt aligns the group's ``=`` signs for you:"),
			code("basics-const-block", "go",
				"Two const groups from the config: names and dev-safe defaults.",
				"server/internal/core/config.go",
				"const (\n"+
					"    envPort     = \"GDG_PORT\"\n"+
					"    envEnv      = \"GDG_ENV\"\n"+
					"    envLogLevel = \"GDG_LOG_LEVEL\"\n"+
					")\n"+
					"// …\n"+
					"const (\n"+
					"    defaultPort     = 8100\n"+
					"    defaultEnv      = \"dev\"\n"+
					"    defaultLogLevel = slog.LevelInfo\n"+
					")"),
			text("basics-types",
				"The value types you will meet constantly: ``int`` (the machine-word integer; "+
					"use ``int64``/``float64`` when size matters), ``string`` (immutable UTF-8 "+
					"bytes — never mutable, always copy-on-write), ``bool``, and the reference "+
					"trio slice, map, and pointer, which the data-structures section dissects. "+
					"Composite types are declared inline: ``[]TopicGroup`` is a slice of "+
					"``TopicGroup``, ``map[Kind]int`` a map from ``Kind`` keys to int values. A "+
					"``struct`` bundles named fields, and the convention here is worth adopting "+
					"verbatim: every exported field carries a doc comment, because the struct "+
					"declaration IS the documentation of record:"),
			code("basics-config-struct", "go",
				"A real struct: three typed fields, each documented where it lives.",
				"server/internal/core/config.go",
				"type Config struct {\n"+
					"    // Port is the TCP port the HTTP server listens on (GDG_PORT, 1-65535).\n"+
					"    Port int\n"+
					"    // Env names the deployment environment, e.g. \"dev\" or \"prod\" (GDG_ENV).\n"+
					"    Env string\n"+
					"    // LogLevel gates the slog JSON logger (GDG_LOG_LEVEL: debug, info,\n"+
					"    // warn, or error).\n"+
					"    LogLevel slog.Level\n"+
					"}"),
			text("basics-zero-values",
				"Zero values are Go's answer to initialization: every type has one, and ``var`` "+
					"gives it to you — numbers are 0, strings are ``\"\"``, bools are false, "+
					"pointers, slices, maps, and functions are nil, and a struct is the zero of "+
					"every field. There is no null-vs-uninitialized dance and no "+
					"undefined-is-not-null: declare ``var topics []Topic`` and it is an empty "+
					"topic list you can immediately range over safely. This single design choice "+
					"deletes a whole class of bugs other languages carry, and the runtime-types "+
					"demo at the end of this topic proves zero values live — reflect can confirm "+
					"any variable ``IsZero()``. The honest discipline it demands: distinguish "+
					"``nil`` (a real, safe value) from what your code does with it — ranging a "+
					"nil slice is fine, writing to a nil map panics."),
			text("basics-branch-intro",
				"Control flow is deliberately small: ``if``/``else``, ``for``, ``switch``, and "+
					"``select`` (concurrency's switch — topic 06). There is no while, no do-while, "+
					"no ternary operator (``if``/``else`` as a statement is the answer, and the "+
					"verbosity is considered a feature), and Go 1.22 finally settled the infamous "+
					"loop-variable capture semantics so closures capturing ``for`` variables just "+
					"work. The canonical real branch in this server maps an HTTP status to the "+
					"closest error kind — note the comma-ok map lookup (second value reports "+
					"presence) and the early returns instead of an else:"),
			code("basics-branch", "go",
				"A real branch: exact map hit first, then a comparison for the leftovers.",
				"server/internal/core/errors.go",
				"func KindForHTTPStatus(status int) Kind {\n"+
					"    if kind, ok := kindByHTTPStatus[status]; ok {\n"+
					"        return kind\n"+
					"    }\n"+
					"    if status < 500 {\n"+
					"        return KindNotFound\n"+
					"    }\n"+
					"    return KindInternal\n"+
					"}"),
			text("basics-loops-intro",
				"Go has exactly one loop keyword — ``for`` — wearing every costume the other "+
					"languages need four keywords for:"),
			list("basics-loop-tools", false,
				"for range — walk any slice, map, string, or channel, element by element; the "+
					"snippet matcher's scan below ranges over normalized lines.",
				"for i := 0; i < n; i++ — the classic counted loop, still first-class when the "+
					"index itself matters; the middleware chain builder runs one backwards.",
				"for cond — a while loop spelled with the same keyword; the scan below advances "+
					"an inner position while lines mismatch.",
				"break / continue — leave the loop entirely or jump to the next iteration; the "+
					"byte scanner's backslash handling uses continue.",
				"range over a map — legal and common, but iteration order is deliberately "+
					"randomized; sort keys explicitly when order matters on the wire.",
				"for i := range xs — index only, when you need positions but not elements."),
			code("basics-loops-scan", "go",
				"for range + while-for + early return in one real two-pointer scan.",
				"server/internal/content/snippet.go",
				"position := 0\n"+
					"for number, line := range snippetLines {\n"+
					"    for position < len(sourceLines) && sourceLines[position] != line {\n"+
					"        position++\n"+
					"    }\n"+
					"    // …\n"+
					"    position++\n"+
					"}"),
			code("basics-loops-chain", "go",
				"The counted loop, running backwards to wrap middleware innermost-first.",
				"server/internal/presentation/middleware.go",
				"return func(final http.Handler) http.Handler {\n"+
					"    for i := len(middleware) - 1; i >= 0; i-- {\n"+
					"        final = middleware[i](final)\n"+
					"    }\n"+
					"    return final\n"+
					"}"),
			text("basics-switch",
				"``switch`` is Go's other branching workhorse, and unlike C its cases do not "+
					"fall through — each case is a clean branch, and the ``default`` catches the "+
					"rest. Its superpower is the type switch (``switch b := block.(type)``), which "+
					"branches on an interface value's dynamic type — so central to this server "+
					"that the interfaces section quotes the real one. A plain switch compares one "+
					"expression against cases; switch with no expression is a cleaner if/else "+
					"chain; both appear throughout the repo. And ``continue`` skips to the next "+
					"iteration, here letting the scanner treat an escaped character pair as one "+
					"unit:"),
			code("basics-continue", "go",
				"continue in the byte scanner: the escaped pair is written, then skipped past.",
				"server/internal/content/snippet.go",
				"if ch == '\\\\' && next != 0 {\n"+
					"    out.WriteByte(next)\n"+
					"    i++\n"+
					"    continue\n"+
					"}"),
			text("basics-functions",
				"A function is ``func name(params) results { body }`` — and the plural "+
					"``results`` is a headline feature: Go functions return multiple values, "+
					"which is how the error convention (``value, err := call()``) is even "+
					"possible. Parameters are typed and checked at every call site; results are "+
					"typed too, so the compiler verifies what you hand back. The single most "+
					"common shape in this server — and in every idiomatic Go codebase — is the "+
					"constructor that validates as it builds: return the zero value of the "+
					"struct plus the error on a bad input, the populated struct plus nil "+
					"otherwise, and the caller branches immediately:"),
			code("basics-func", "go",
				"The repo's most common function shape: build, validate, return both.",
				"server/internal/content/model.go",
				"func NewTextBlock(id, text string) (TextBlock, error) {\n"+
					"    if err := requireNonBlank(\"text block id\", id); err != nil {\n"+
					"        return TextBlock{}, err\n"+
					"    }\n"+
					"    // …\n"+
					"    return TextBlock{Kind: KindText, ID: id, Text: text}, nil\n"+
					"}"),
			text("basics-variadic-intro",
				"Parameters can be variadic — ``args ...any`` collects any number of trailing "+
					"arguments into a slice, and ``any`` (a synonym for the empty interface "+
					"``interface{}``) accepts every type. That signature is printf's signature, "+
					"and this repo's formatted-error helper is printf in miniature — format "+
					"plus any values, one typed result:"),
			code("basics-variadic", "go",
				"Variadic + any: the printf pattern, in the typed-error factory.",
				"server/internal/core/errors.go",
				"func Newf(kind Kind, format string, args ...any) *Error {\n"+
					"    return &Error{Kind: kind, Message: fmt.Sprintf(format, args...)}\n"+
					"}"),
		),
		// ── 3. Data structures and strings ───────────────────────────────
		section("go-data-and-strings", "Data structures and strings",
			"Slices, maps, and the string/byte/rune triangle — where Go's data model lives.",
			text("data-slices",
				"A slice is Go's workhorse sequence: a window over a backing array, carrying "+
					"length (``len``) and capacity (``cap``). ``make([]T, 0, n)`` allocates with "+
					"capacity n so appends do not repeatedly reallocate — the repo does this "+
					"everywhere a loop builds a result. ``append`` grows slices functionally "+
					"(it may return a new backing array; always reassign), and the spread "+
					"operator ``xs...`` splats one slice into a call. The sharpest idiom in the "+
					"registry is the defensive copy: ``append([]T(nil), xs...)`` builds a fresh "+
					"slice so the registry's stored copy can never be mutated through the "+
					"caller's alias — exactly what an immutable-by-construction content store wants:"),
			code("data-slices-copies", "go",
				"The defensive copy idiom, plus a sort with an inline closure comparator.",
				"server/internal/content/registry.go",
				"sortedGroups := append([]TopicGroup(nil), groups...)\n"+
					"sort.SliceStable(sortedGroups, func(i, j int) bool {\n"+
					"    return sortedGroups[i].Order < sortedGroups[j].Order\n"+
					"})\n"+
					"sortedTopics := append([]Topic(nil), topics...)"),
			text("data-maps",
				"A map is Go's hash table: ``map[K]V``, made with ``make`` or a composite "+
					"literal, accessed with ``m[key]``. The two-value lookup is the idiom to "+
					"internalize — ``v, ok := m[k]`` distinguishes a zero-VALUED entry from a "+
					"missing one (``ok`` is the comma-ok form the branch above used). Deleting "+
					"is ``delete(m, k)``, a builtin that no-ops on missing keys. Maps are "+
					"unordered — the randomized iteration order is deliberate — and the registry "+
					"shows the two construction styles side by side: a literal for the fixed "+
					"kind-to-status table, then a loop-built inversion of it (a function literal "+
					"executed immediately — the ``func() … ()`` shape — because package-level "+
					"``var`` initialization can run code):"),
			code("data-maps-literal", "go",
				"A fixed table as a map literal — keys aligned, values from net/http.",
				"server/internal/core/errors.go",
				"var httpStatusByKind = map[Kind]int{\n"+
					"    KindValidationFailed:   http.StatusUnprocessableEntity,\n"+
					"    // …\n"+
					"    KindInternal:           http.StatusInternalServerError,\n"+
					"}"),
			code("data-maps-inverted", "go",
				"The same table inverted by a loop, inside an immediately-run function literal.",
				"server/internal/core/errors.go",
				"var kindByHTTPStatus = func() map[int]Kind {\n"+
					"    byStatus := make(map[int]Kind, len(httpStatusByKind))\n"+
					"    for kind, status := range httpStatusByKind {\n"+
					"        byStatus[status] = kind\n"+
					"    }\n"+
					"    return byStatus\n"+
					"}()"),
			text("data-strings-intro",
				"Strings are immutable UTF-8 byte sequences — that is the whole truth, and every "+
					"surprise in Go string handling comes from forgetting one half of it. "+
					"``len(s)`` counts BYTES, not characters; indexing ``s[i]`` yields a "+
					"``byte``; a rune-count requires ``utf8.RuneCountInString``. The standard "+
					"library's strings package carries the everyday toolkit — ``Split``, "+
					"``TrimSpace``, ``HasPrefix``, ``Contains``, ``Builder`` for assembling "+
					"without per-concat allocation — and the snippet normalizer is a compact "+
					"tour of it, splitting text into lines, trimming each, and appending the "+
					"survivors to a growing slice:"),
			code("data-strings-normalize", "go",
				"Split, trim, and build: the strings package on real duty.",
				"server/internal/content/snippet.go",
				"var normalized []string\n"+
					"inBlock := false\n"+
					"for _, raw := range strings.Split(text, \"\\n\") {\n"+
					"    raw = strings.TrimSuffix(raw, \"\\r\")\n"+
					"    code, stillInBlock := stripLine(raw, style, inBlock)\n"+
					"    inBlock = stillInBlock\n"+
					"    code = strings.TrimSpace(code)\n"+
					"    if code != \"\" {\n"+
					"        normalized = append(normalized, code)\n"+
					"    }\n"+
					"}"),
			text("data-runes",
				"The rune is Go's character: a ``rune`` is an alias for ``int32``, holding a "+
					"Unicode code point. When you need characters, you range a string — ``for "+
					"i, r := range s`` yields byte offsets and runes, decoding UTF-8 on the fly. "+
					"When you need raw bytes, you index. The comment scanner below lives on the "+
					"boundary between the two worlds, and its boundary check is the line worth "+
					"staring at: it takes the LAST BYTE of the string so far, converts it to a "+
					"rune value, and asks whether that code point is whitespace — a byte-to-rune "+
					"round trip in one expression:"),
			code("data-runes-boundary", "go",
				"byte indexing and rune conversion, one real expression.",
				"server/internal/content/snippet.go",
				"func atTokenBoundary(codeSoFar string) bool {\n"+
					"    return codeSoFar == \"\" || strings.ContainsRune(whitespace, rune(codeSoFar[len(codeSoFar)-1]))\n"+
					"}"),
			text("data-bytes-honest",
				"The byte-semantics caveat, demonstrated by this repo's own line-shortener: "+
					"``len(line) <= maxReasonLine`` compares BYTES, and ``line[:maxReasonLine]`` "+
					"slices BYTES — so a line whose 80th byte falls inside a multi-byte rune "+
					"would be cut mid-character. The function is honest for its actual input "+
					"(failure reasons are ASCII code lines), and it is quoted here precisely "+
					"because the trap is universal: every string slice, every len, every index "+
					"in Go is byte-shaped, and the fix is always the same — range for runes, "+
					"index for bytes, and know which one you are holding:"),
			code("data-strings-short", "go",
				"Byte semantics on display: len and slicing are byte-shaped.",
				"server/internal/content/snippet.go",
				"func short(line string) string {\n"+
					"    if len(line) <= maxReasonLine {\n"+
					"        return line\n"+
					"    }\n"+
					"    return line[:maxReasonLine] + \"…\"\n"+
					"}"),
			text("data-verbs-intro",
				"Formatting is ``fmt``'s job, and its verbs are worth memorizing once — they "+
					"appear in every log line, every error message, and every test failure this "+
					"guide prints:"),
			list("data-verbs", false,
				"%v / %s — default formatting and strings; the error type's \"[%s] %s\" labels "+
					"every error this server logs.",
				"%q — a Go-quoted string, quotes included; snippet-matcher failure reasons use "+
					"it so quoted code renders literally.",
				"%d — integers; the registry's list-size errors report their %d–%d bounds.",
				"%T — the value's runtime type name; the demo at this topic's end computes the "+
					"same fact the long way, through reflect.",
				"%+v — struct fields with names; the debugging habit when a bare %v hides too much.",
				"%w — wrap an error inside a formatted message so errors.Is/As still see it — "+
					"the mechanism the errors section below is built on."),
		),
		// ── 4. Structs, methods, embedding ───────────────────────────────
		section("structs-methods", "Structs, methods, and embedding",
			"Go's answer to objects: data types with functions attached, composed instead of inherited.",
			text("structs-pitch",
				"Go has no classes, and the closest equivalent is exactly two pieces: a "+
					"``struct`` that holds fields, and ``methods`` — functions with a receiver "+
					"parameter that associates them with the type. The receiver is written "+
					"before the name: ``func (e *Error) Error() string`` reads as ``the method "+
					"Error on *Error``. Methods do not live inside the struct declaration; they "+
					"can be defined in any file of the same package, and the struct stays pure "+
					"data. The server's error type is the canonical exhibit — four fields, one "+
					"of them deliberately unexported (lowercase ``cause``), and a method "+
					"satisfying the built-in ``error`` interface just by existing:"),
			code("structs-error-type", "go",
				"A real struct with an unexported field, plus its two smallest methods.",
				"server/internal/core/errors.go",
				"type Error struct {\n"+
					"    Kind    Kind\n"+
					"    Message string\n"+
					"    Details any\n"+
					"    cause   error\n"+
					"}\n"+
					"// …\n"+
					"func (e *Error) Error() string {\n"+
					"    return fmt.Sprintf(\"[%s] %s\", e.Kind, e.Message)\n"+
					"}\n"+
					"\n"+
					"func (e *Error) Unwrap() error { return e.cause }"),
			text("structs-constructors",
				"There is no constructor syntax — constructors are ordinary functions, and the "+
					"convention is a ``New``-prefixed name returning the built value plus an "+
					"error: ``NewTopic``, ``NewScenarioStore``, ``NewRegistry``. This repo's "+
					"constructors validate their inputs and refuse to build invalid objects, so "+
					"a value that EXISTS is a value that passes its own rules — the registry "+
					"re-validates anyway because Go struct literals can bypass functions, but "+
					"the shape is the habit to copy. Factory variants follow the same pattern: "+
					"``Newf`` is the formatted-error factory, ``NotFound`` and friends the "+
					"kind-specific ones."),
			text("structs-embedding-intro",
				"Embedding is Go's inheritance replacement: name a type as an anonymous field "+
					"and its fields and methods are PROMOTED — accessible on the outer type as "+
					"if declared there. The status recorder wraps ``http.ResponseWriter`` to "+
					"capture the status code for the access log; embedding hands it every "+
					"ResponseWriter method for free while it adds its own bookkeeping fields:"),
			code("structs-embedding", "go",
				"Embedding: the wrapper IS a ResponseWriter, plus two private counters.",
				"server/internal/presentation/middleware.go",
				"type statusRecorder struct {\n"+
					"    http.ResponseWriter\n"+
					"    status int\n"+
					"    wrote  bool\n"+
					"}"),
			text("structs-embedding-flush",
				"Promotion is reachable but not hidden: ``r.ResponseWriter`` names the "+
					"embedded value explicitly when you need IT rather than the promoted "+
					"method — the Flush override below does exactly that, finding the flusher "+
					"on the wrapped writer (via a comma-ok type assertion) and delegating to "+
					"it. Method overriding, Go-style, is just a method with the same name on "+
					"the outer type; there is no super keyword, you call through the embedded "+
					"field by name instead:"),
			code("structs-embedding-flush-code", "go",
				"Override-and-delegate: the outer Flush finds the inner flusher explicitly.",
				"server/internal/presentation/middleware.go",
				"func (r *statusRecorder) Flush() {\n"+
					"    if flusher, ok := r.ResponseWriter.(http.Flusher); ok {\n"+
					"        flusher.Flush()\n"+
					"    }\n"+
					"}"),
			text("structs-coda",
				"What embedding deliberately is NOT: subtype polymorphism. A ``statusRecorder`` "+
					"wraps a ResponseWriter; it does not become one in any substitution sense "+
					"beyond satisfying the same interface — and interface satisfaction is the "+
					"next section's subject, the true center of Go's type system."),
		),
		// ── 5. Interfaces ────────────────────────────────────────────────
		section("interfaces", "Interfaces: satisfaction without declaration",
			"The type system's center of gravity — and the one design choice most worth understanding deeply.",
			text("iface-pitch",
				"An interface is a method SET — a list of method signatures a value must have "+
					"to be usable as that interface. The radical part: satisfaction is "+
					"IMPLICIT. No ``implements`` clause, no declaration anywhere — if your type "+
					"has the methods, it is the interface, and the compiler proves it at the "+
					"assignment. This single rule inverts how APIs are designed: instead of "+
					"defining interfaces up front and making implementers conform, you write "+
					"small consumers-side interfaces and any existing type already satisfies "+
					"them. The content model seals its block hierarchy with the trick "+
					"explicitly: ``Block`` demands one UNEXPORTED method, so only types in "+
					"package content can ever implement it — an interface used as a gate:"),
			code("iface-block", "go",
				"Implicit satisfaction, sealing a hierarchy: no type ever declares 'implements Block'.",
				"server/internal/content/model.go",
				"type Block interface {\n"+
					"    blockKind() string\n"+
					"}\n"+
					"// …\n"+
					"func (TextBlock) blockKind() string { return KindText }\n"+
					"func (CodeBlock) blockKind() string { return KindCode }\n"+
					"func (DemoBlock) blockKind() string { return KindDemo }\n"+
					"func (ListBlock) blockKind() string { return KindList }"),
			text("iface-any",
				"The empty interface ``interface{}`` — spelled ``any`` since Go 1.18 — demands "+
					"nothing, so every value satisfies it. It is how ``fmt`` accepts anything, "+
					"and how the JSON writer accepts every response model with one signature. "+
					"Power with a price: an ``any`` carries no compile-time knowledge, so you "+
					"must reflect or type-assert to do anything specific — which is exactly "+
					"the trade the runtime-types demo at this topic's end makes visible:"),
			code("iface-any-writejson", "go",
				"One writer, every response: any is the whole contract.",
				"server/internal/presentation/content.go",
				"func writeJSON(w http.ResponseWriter, status int, v any) {\n"+
					"    w.Header().Set(\"Content-Type\", \"application/json\")\n"+
					"    w.WriteHeader(status)\n"+
					"    _ = json.NewEncoder(w).Encode(v)\n"+
					"}"),
			text("iface-handler",
				"The most-satisfied interface in Go servers is ``http.Handler`` — one method, "+
					"``ServeHTTP(ResponseWriter, *Request)`` — and the entire HTTP layer of this "+
					"guide is built on it. Every handler below implements it without saying so; "+
					"the mux accepts them BECAUSE they have the method. Note the underscore "+
					"parameter: unused parameters keep their position but surrender their name:"),
			code("iface-handler-code", "go",
				"http.Handler, satisfied implicitly by every handler in this server.",
				"server/internal/presentation/health.go",
				"func (h *HealthHandler) ServeHTTP(w http.ResponseWriter, _ *http.Request) {\n"+
					"    w.Header().Set(\"Content-Type\", \"application/json\")\n"+
					"    // …\n"+
					"    _ = json.NewEncoder(w).Encode(healthResponse{Status: \"ok\", Env: h.env})\n"+
					"}"),
			text("iface-assertions",
				"An interface value is a box: a (dynamic type, concrete value) pair. Type "+
					"assertions and type switches open the box. The two-value assertion "+
					"``v, ok := x.(T)`` reports false instead of panicking when the dynamic "+
					"type mismatches; the single-value form ``x.(T)`` panics — reserve it for "+
					"cases where the program is wrong if it fails. The request-id reader is "+
					"the repo's tidiest comma-ok: pull from context, assert to string, fall "+
					"back on absence:"),
			code("iface-assertion", "go",
				"Comma-ok assertion: missing value reports false, never panics.",
				"server/internal/core/requestid.go",
				"func RequestIDFromContext(ctx context.Context) string {\n"+
					"    if ctx == nil {\n"+
					"        return \"\"\n"+
					"    }\n"+
					"    id, _ := ctx.Value(requestIDKey{}).(string)\n"+
					"    return id\n"+
					"}"),
			text("iface-switch",
				"The type switch is the assertion's big brother — one construct, every case "+
					"binding the value at its concrete type, and the registry's block "+
					"dispatcher is the canonical real one. Four concrete block types plus a "+
					"default that turns a future unknown block into a loud error rather than "+
					"a silent skip:"),
			code("iface-switch-code", "go",
				"The real type switch: every block kind resolved by its concrete type.",
				"server/internal/content/model.go",
				"func blockIDOf(block Block) string {\n"+
					"    switch b := block.(type) {\n"+
					"    case TextBlock:\n"+
					"        return b.ID\n"+
					"    // …\n"+
					"    default:\n"+
					"        return \"\"\n"+
					"    }\n"+
					"}"),
			text("iface-coda",
				"The design rules that fall out, in the idiom Go engineers repeat to each "+
					"other: accept interfaces, return structs — parameters want the narrowest "+
					"method set that suffices (often one method), results want concrete types "+
					"so callers keep the full API. Keep interfaces small — the standard "+
					"library's ``io.Reader`` and ``io.Writer`` are one method each and carry "+
					"half the ecosystem on their backs. Topic 05 spends a whole page on this "+
					"discipline; here it is enough to have met the mechanism."),
		),
		// ── 6. Pointers vs values ────────────────────────────────────────
		section("pointers-vs-values", "Pointers vs values",
			"When data is copied, when it is shared — Go's most practical everyday decision.",
			text("ptr-pitch",
				"A pointer is a variable holding another value's address: ``*T`` is a pointer "+
					"to T, ``&x`` takes x's address, ``*p`` reads through the pointer. There "+
					"is no pointer arithmetic — pointers are capability, not math. The choice "+
					"between values and pointers runs through everything: function arguments "+
					"PASS BY VALUE in Go, always — a struct argument is copied — so "+
					"passing a pointer is how a callee mutates the caller's copy or avoids "+
					"copying something large. The compiler helps with a rule of thumb: escape "+
					"analysis decides what lives on the stack versus heap, and taking an "+
					"address does not force a heap allocation if the value never outlives the "+
					"frame. The repo's sentinels show the most common shape — package-level "+
					"pointer values, compared by identity:"),
			code("ptr-sentinels", "go",
				"& composite literals: package-level pointers used as comparison sentinels.",
				"server/internal/core/errors.go",
				"var (\n"+
					"    ErrValidationFailed   = &Error{Kind: KindValidationFailed, Message: \"Validation failed.\"}\n"+
					"    // …\n"+
					"    ErrNotFound           = &Error{Kind: KindNotFound, Message: \"Resource not found.\"}\n"+
					")"),
			text("ptr-receivers",
				"Receiver choice is the same decision in method clothing, with one hard rule "+
					"and one strong convention. The hard rule: a value receiver gets a COPY of "+
					"the receiver, so a mutex inside the struct is copied too — and "+
					"``sync.RWMutex`` must never be copied, or the lock stops locking. Any "+
					"struct guarding state therefore takes POINTER receivers for everything. "+
					"The convention: never mix — a type whose smallest method needs a pointer "+
					"takes pointers throughout, so readers never guess. The scenario store is "+
					"the textbook case:"),
			code("ptr-receivers-code", "go",
				"Pointer receivers, because the mutex must not be copied.",
				"server/internal/core/scenario.go",
				"type ScenarioStore struct {\n"+
					"    mu      sync.RWMutex\n"+
					"    current ScenarioState\n"+
					"}\n"+
					"// …\n"+
					"func (s *ScenarioStore) Snapshot() ScenarioState {\n"+
					"    s.mu.RLock()\n"+
					"    defer s.mu.RUnlock()\n"+
					"    return s.current\n"+
					"}"),
			text("ptr-value-receivers",
				"Value receivers are not a fallback — they are a statement: this method "+
					"reads, never writes, and the type is safe and cheap to copy. The "+
					"registry is exactly that: an immutable snapshot after construction, so "+
					"every method takes it by value, callers copy it freely, and the copy is "+
					"still correct because its map field — the id lookup — is a reference "+
					"type whose backing storage is shared on purpose (building it is the "+
					"quote below). Copies of the struct share the map; nothing mutates it "+
					"after boot; everyone reads. Value semantics with shared read-only "+
					"internals is a quietly powerful combination:"),
			code("ptr-value-receivers-code", "go",
				"A value-receiver type, safe to copy because its map is built once, then read-only.",
				"server/internal/content/registry.go",
				"byID := make(map[string]Topic, len(sortedTopics))\n"+
					"for _, topic := range sortedTopics {\n"+
					"    byID[topic.ID] = topic\n"+
					"}\n"+
					"return Registry{topics: sortedTopics, groups: sortedGroups, topicsByID: byID}, nil"),
			text("ptr-optional",
				"The other everyday use: pointers as OPTIONAL fields. Go structs have no "+
					"optional — a string field is always there, always \"\" when unset — so "+
					"when a patch protocol must distinguish absent from set-to-zero-value, "+
					"the idiom is a pointer field: nil means absent, non-nil means change it. "+
					"The scenario PATCH body is the real example, and its JSON tags carry the "+
					"struct-tag syntax worth learning once (backquoted tags, name plus "+
					"options, ``omitempty`` dropping empty values from the wire):"),
			code("ptr-optional-code", "go",
				"Pointer fields as optional: nil means 'not in this patch'.",
				"server/internal/core/scenario.go",
				"type ScenarioPatch struct {\n"+
					"    DataMode      *DataMode         `json:\"dataMode,omitempty\"`\n"+
					"    InjectFailure *FailureInjection `json:\"injectFailure,omitempty\"`\n"+
					"}"),
			text("ptr-nil",
				"Nil needs a paragraph of respect: a nil POINTER is safe to hold, unsafe to "+
					"dereference; a nil SLICE is safe to range and len; a nil MAP is safe to "+
					"read and panics on write; and a nil interface is distinct from an "+
					"interface holding a nil pointer — the classic trap (a typed nil in a "+
					"non-nil interface box), which is why error returns never wrap a bare nil "+
					"pointer in a typed interface. When you meet a mysterious non-nil error "+
					"that prints as nil, you have met this trap; the fix is returning nil "+
					"itself, not a typed nil."),
		),
		// ── 7. Errors as values ──────────────────────────────────────────
		section("errors-as-values", "Errors as values: a first taste",
			"No exceptions, no try/catch — failures travel as ordinary values with one interface.",
			text("errs-pitch",
				"Go's most argued and most defining choice: errors are values. The builtin "+
					"``error`` interface is one method — ``Error() string`` — and the "+
					"convention is that a function which can fail returns ``(result, error)`` "+
					"with the error last. You check it at the call site, immediately, with "+
					"``if err != nil`` — the two-line dance you will type more than any other "+
					"in Go. There is no stack unwinding and no invisible control flow: every "+
					"failure path is visible in the source, which reads as verbosity until the "+
					"first debugging session where nothing is hidden, then reads as honesty. "+
					"Topic 30 does this subject full justice; this taste covers the mechanism "+
					"the rest of the tour needs."),
			text("errs-wrap",
				"Wrapping is how errors keep their story while gaining context: "+
					"``fmt.Errorf`` with the ``%w`` verb produces an error that CONTAINS the "+
					"original and still exposes it to the matching machinery — the addition "+
					"says what the code was doing, the wrapped error says what actually broke. "+
					"The composition root wraps every boot failure exactly this way:"),
			code("errs-wrap-code", "go",
				"%w wrapping at the composition root: context added, cause preserved.",
				"server/cmd/guide/main.go",
				"cfg, err := core.LoadConfig(os.Getenv)\n"+
					"if err != nil {\n"+
					"    return fmt.Errorf(\"load config: %w\", err)\n"+
					"}"),
			text("errs-match",
				"Matching is the other half: ``errors.Is`` walks the wrap chain comparing "+
					"against sentinel values, ``errors.As`` walks it looking for a concrete "+
					"type. Both work because the error interface's implementers cooperate — "+
					"the typed error below customizes ``Is`` so ANY error of the same kind "+
					"matches the kind's sentinel, and the central error writer unwraps with "+
					"``As`` to decide the response envelope. This trio — Is, As, and "+
					"custom Unwrap/Is — is the whole matching mechanism:"),
			code("errs-is-code", "go",
				"Kind-level matching: Is overridden so sentinels compare by bucket.",
				"server/internal/core/errors.go",
				"func (e *Error) Is(target error) bool {\n"+
					"    targetErr, ok := target.(*Error)\n"+
					"    return ok && targetErr.Kind == e.Kind\n"+
					"}"),
			code("errs-as-code", "go",
				"The other side: As finds the typed error inside any wrap chain.",
				"server/internal/core/envelope.go",
				"var appErr *Error\n"+
					"if errors.As(err, &appErr) && knownKinds[appErr.Kind] {"),
			text("errs-panics",
				"And panics: they exist, they are for PROGRAMMER bugs (a broken invariant, "+
					"an impossible state), and they are never control flow. The middleware "+
					"chain recovers them into the same 500 envelope every other failure uses "+
					"so a crash never leaks a stack trace to a client — and the one deliberate "+
					"panic in the demo module (guarding an encoding/json contract that must "+
					"hold) is annotated as exactly that: a this-cannot-happen tripwire, not an "+
					"error path."),
		),
		// ── 8. Packages, imports, visibility ─────────────────────────────
		section("packages", "Packages, imports, and visibility",
			"How Go code is organized — and the two rules that make large codebases navigable.",
			text("pkg-pitch",
				"Go code is organized into packages: one directory, one package, imported by "+
					"path. The import path is the module path (``guide``, from go.mod) plus "+
					"the directory — ``guide/internal/content`` is the content package. "+
					"Imports group visually (standard library, then your own, then third "+
					"party — gofmt keeps the groups separate), and every import must be USED "+
					"or the program does not compile: unused imports are errors, not "+
					"warnings, which keeps import lists honest. The composition root's import "+
					"block is the whole architecture in fifteen lines:"),
			code("pkg-imports", "go",
				"The import block: stdlib group, then the guide's own three layers.",
				"server/cmd/guide/main.go",
				"import (\n"+
					"    \"context\"\n"+
					"    \"fmt\"\n"+
					"    \"log/slog\"\n"+
					"    \"net/http\"\n"+
					"    \"os\"\n"+
					"    \"os/signal\"\n"+
					"    \"syscall\"\n"+
					"    \"time\"\n"+
					"\n"+
					"    \"guide/internal/content\"\n"+
					"    \"guide/internal/core\"\n"+
					"    \"guide/internal/presentation\"\n"+
					")"),
			text("pkg-visibility",
				"Visibility is one character: identifiers starting with an UPPERCASE letter "+
					"are exported — visible to importing packages; lowercase is package "+
					"private. No public/private/protected keywords, no friend declarations; "+
					"the case IS the access control. This is why the Block interface in the "+
					"interfaces section could seal a hierarchy — its one method is "+
					"lowercase, so only package content can implement it. The same rule "+
					"makes context keys safe: an unexported STRUCT type as key cannot "+
					"collide with any other package's key, and the empty struct ``struct{}`` "+
					"occupies zero bytes — the tiniest possible key:"),
			code("pkg-key", "go",
				"Unexported empty struct as a context key: collision-proof and zero-sized.",
				"server/internal/core/requestid.go",
				"type requestIDKey struct{}\n"+
					"\n"+
					"func WithRequestID(ctx context.Context, id string) context.Context {\n"+
					"    return context.WithValue(ctx, requestIDKey{}, id)\n"+
					"}"),
			text("pkg-internal",
				"The ``internal`` directory name is ENFORCED privacy: anything under a path "+
					"segment named internal can be imported only by code rooted at the "+
					"directory containing it. ``guide/internal/...`` is therefore invisible "+
					"to any other module — the go command itself rejects the import. This "+
					"server uses it as the architecture: internal/core, internal/content, "+
					"internal/domain, internal/data, internal/presentation are all sealed "+
					"inside the module, so the module's public surface is exactly nothing "+
					"but its ``cmd/guide`` binary."),
			text("pkg-docs",
				"Documentation is a comment convention, checked by tooling: a comment "+
					"starting ``Package <name>`` on one file's package clause becomes the "+
					"package doc (``go doc`` renders it); a comment directly above an "+
					"exported identifier is its doc comment, and this repo requires one on "+
					"every exported id — the content package's header is the model, stating "+
					"what the package IS and what its contracts are before any code:"),
			code("pkg-doc", "go",
				"The package doc: what this package is, in prose above the clause.",
				"server/internal/content/model.go",
				"// Package content is the typed teaching model the React client renders:\n"+
					"// topic → sections → blocks, plus the registry that validates it at startup\n"+
					"// and the snippet matcher that keeps every quoted snippet honest.\n"+
					"// …\n"+
					"package content"),
		),
		// ── 9. Generics ──────────────────────────────────────────────────
		section("generics", "Generics: type parameters and constraints",
			"Write the algorithm once, keep the compiler's checks — sparingly.",
			text("gen-pitch",
				"Generics arrived in Go 1.18 with deliberately narrow ambitions: type "+
					"PARAMETERS on functions and types, constrained by interfaces, for the "+
					"cases where you would otherwise copy-paste a function per type or "+
					"degrade to ``any`` and lose checking. The syntax sits in square "+
					"brackets: ``func Sum[T Numeric](values []T) T`` declares that T is any "+
					"type satisfying the constraint, and callers' T is INFERRED from "+
					"arguments. The constraint vocabulary is the real learning: ``any`` "+
					"means unconstrained; ``comparable`` means usable as a map key; a union "+
					"like the one below enumerates allowed underlying types, with the tilde "+
					"admitting every type whose UNDERLYING type matches — so ``type Hits int`` "+
					"still qualifies:"),
			code("gen-numeric", "go",
				"A constraint with union + tilde, and the generic function that uses it.",
				"server/internal/domain/teaching/generics.go",
				"type Numeric interface {\n"+
					"    ~int | ~int64 | ~float64\n"+
					"}\n"+
					"\n"+
					"func Sum[T Numeric](values []T) T {\n"+
					"    var total T\n"+
					"    for _, value := range values {\n"+
					"        total += value\n"+
					"    }\n"+
					"    return total\n"+
					"}"),
			text("gen-two-params",
				"Multiple type parameters model transformations — Map changes element type, "+
					"so it needs two independent parameters, and its function-typed "+
					"parameter shows generics composing with first-class functions. Note "+
					"the capacity hint in ``make``: generic code keeps every ordinary habit:"),
			code("gen-map", "go",
				"Two independent type parameters, a function-typed argument, plain make.",
				"server/internal/domain/teaching/generics.go",
				"func Map[T, U any](in []T, fn func(T) U) []U {\n"+
					"    out := make([]U, 0, len(in))\n"+
					"    for _, value := range in {\n"+
					"        out = append(out, fn(value))\n"+
					"    }\n"+
					"    return out\n"+
					"}"),
			text("gen-types",
				"Types take parameters too, and methods on them use the parameters as-is — "+
					"a ``Set[T comparable]`` is a set over any hashable element type, with "+
					"methods that never repeat the parameter lists beyond the receiver. The "+
					"map-based set with ``struct{}`` values is the canonical instance: empty "+
					"struct occupies no memory, so the map's values cost nothing and only "+
					"keys exist:"),
			code("gen-set", "go",
				"A generic type with methods: Set[T comparable], the classic payoff.",
				"server/internal/domain/teaching/generics.go",
				"type Set[T comparable] map[T]struct{}\n"+
					"\n"+
					"func NewSet[T comparable](values ...T) Set[T] {\n"+
					"    set := make(Set[T], len(values))\n"+
					"    for _, value := range values {\n"+
					"        set[value] = struct{}{}\n"+
					"    }\n"+
					"    return set\n"+
					"}\n"+
					"\n"+
					"func (s Set[T]) Has(value T) bool {\n"+
					"    _, ok := s[value]\n"+
					"    return ok\n"+
					"}"),
			text("gen-stdlib",
				"Before writing your own generic anything, check the standard library — "+
					"Go 1.21 grew generic packages that cover the common ground: ``slices`` "+
					"(Contains, Sort, Equal, and friends), ``maps`` (Keys, Values), "+
					"``cmp`` (the Ordered constraint, with a ready-made Max). The mux-error "+
					"middleware already calls the generic ``slices.Contains`` on production "+
					"duty — note it is an ordinary call site; generics are invisible to "+
					"callers when inference does its job:"),
			code("gen-stdlib-code", "go",
				"Generic stdlib in production use: slices.Contains over a map[int][]string.",
				"server/internal/presentation/middleware.go",
				"canned := slices.Contains(muxCannedBodies[i.status], i.buf.String())\n"+
					"if !canned {"),
			text("gen-when-not",
				"And the restraint, because Go's own designers lead with it: reach for a "+
					"METHOD first (an interface with one method often beats a type "+
					"parameter), start concrete and abstract only on the second or third "+
					"duplicate, and never wrap a perfectly good concrete function in a "+
					"generic shell nobody needs. This repo's own exhibit package is the honest "+
					"shape of generics in a real codebase: a handful of small, tested, "+
					"quarantined-in-teaching helpers — while the production code satisfies "+
					"itself with concrete types and the stdlib's generics. Topic 05 returns "+
					"to where the line genuinely belongs."),
		),
		// ── 10. Types at runtime (inline demo) ───────────────────────────
		section("runtime-types", "Types at runtime",
			"This topic's demo lives here — beside the teaching it proves.",
			text("rt-pitch",
				"Everything above happens at compile time — so what DO types mean at runtime? "+
					"Three real things. Interface values carry their dynamic type, which is "+
					"why assertions and type switches work. Struct metadata — field names, "+
					"types, tags — is retained and readable through the ``reflect`` package, "+
					"which is how encoding/json knows that ``json:\"id\"`` names the field "+
					"your API serves. And every value knows enough about itself for ``fmt``'s "+
					"%T to print it. That runtime information exists FOR the boundaries — "+
					"serialization, fmt, and the rare genuinely-dynamic plugin point — while "+
					"the compile-time checker does the everyday enforcement. The demo module "+
					"below is the payload builder the endpoint serves, and it is real "+
					"code the topic quotes — starting with the exhibit function the "+
					"enforcement beat is built on:"),
			code("rt-repeat", "go",
				"The demo's exhibit: fully typed, checked at every call site.",
				"server/internal/domain/demos/runtime_types.go",
				"func repeat(text string, times int) string {\n"+
					"    return strings.Repeat(text, times)\n"+
					"}"),
			code("rt-typerow", "go",
				"reflect.TypeOf over any value: the runtime type facts, as data.",
				"server/internal/domain/demos/runtime_types.go",
				"func typeRow(expression string, value any) TypeRow {\n"+
					"    t := reflect.TypeOf(value)\n"+
					"    return TypeRow{\n"+
					"        Expression: expression,\n"+
					"        Type:       t.String(),\n"+
					"        Kind:       t.Kind().String(),\n"+
					"        PkgPath:    t.PkgPath(),\n"+
					"        SizeBytes:  int(t.Size()),\n"+
					"    }\n"+
					"}"),
			text("rt-enforcement",
				"The enforcement proof is the one thing this demo cannot RUN — by design. "+
					"The call ``repeat(3.0, 2)`` — a float handed to a string parameter — "+
					"does not misbehave at runtime; it never reaches runtime, because the "+
					"program containing it does not compile. The payload carries the "+
					"compiler's own verdict, verbatim from ``go build``: ``cannot use 3.0 "+
					"(untyped float constant 3) as string value in argument to repeat``. "+
					"Where a dynamically typed language needs a static analyzer to catch "+
					"that call, Go needs nothing — the toolchain that builds the program is "+
					"the checker. The counterpoint is the boundary: the same payload hands "+
					"encoding/json a number where ``content.TextBlock``'s ID declares a "+
					"string, and the decoder refuses with a typed ``*json.UnmarshalTypeError`` "+
					"— runtime type checking, exactly at the edge where outside data meets "+
					"a typed struct."),
			code("rt-switch", "go",
				"The demo's dispatch: the same type switch the model runs, over real blocks.",
				"server/internal/domain/demos/runtime_types.go",
				"func blockIDBySwitch(block content.Block) string {\n"+
					"    switch b := block.(type) {\n"+
					"    case content.TextBlock:\n"+
					"        return b.ID\n"+
					"    // …\n"+
					"    }\n"+
					"}"),
			code("rt-handler", "go",
				"The demo endpoint itself: a thin handler, a pure builder, one JSON write.",
				"server/internal/presentation/demo.go",
				"func (h *RuntimeTypesHandler) ServeHTTP(w http.ResponseWriter, _ *http.Request) {\n"+
					"    writeJSON(w, http.StatusOK, demos.BuildRuntimeTypesPayload())\n"+
					"}"),
			demo("go:runtime-types", "Types at runtime — the proof",
				"Computed live from this repo's real objects: reflect.TypeOf/Kind/PkgPath "+
					"walks, the Section struct's fields with their JSON tags, dynamic types "+
					"resolved by type switch, declared-and-never-initialized zero values, the "+
					"compiler's verbatim rejection of a wrong-typed call, and encoding/json "+
					"refusing a number where a string field is declared."),
			text("rt-beats-intro",
				"Read the demo in six beats:"),
			list("rt-beats", false,
				"Inspections — reflect.TypeOf over real guide values of nine kinds: struct, "+
					"func, slice, map, string, int, pointer among them.",
				"Fields — the Section struct walked field by field, its json tags readable at "+
					"runtime; that is where camelCase wire names live.",
				"Assertions — what one []content.Block slot holds per element, resolved by "+
					"type switch, plus one comma-ok miss that reports false instead of panicking.",
				"Zero values — six declared-and-never-initialized variables, each confirmed "+
					"zero by reflect's IsZero.",
				"Enforcement — the wrong-typed call with the compiler's verbatim verdict, "+
					"beside the typed call's real output.",
				"Boundary — encoding/json rejecting a number in a string field: the runtime "+
					"check living exactly where untyped data enters."),
			text("rt-coda",
				"Compile-time checking everywhere, runtime type information exactly where it "+
					"earns its keep — that division of labor is the Go contract this whole "+
					"server is written under, and it is the exact inverse of the dynamically "+
					"typed story this guide's siblings tell."),
		),
		// ── 11. Terminal companion ───────────────────────────────────────
		section("terminal-companion", "Terminal companion: the curl tour",
			"The guide is its own API — read it from the shell.",
			text("term-pitch",
				"With the stack up (server on :8100), these commands walk the exact surface "+
					"this topic lives in. Piping to ``python3 -m json.tool`` gets you "+
					"pretty-printed JSON with zero extra tooling — the only dependency this "+
					"tour needs is something every machine already has:"),
			code("term-list", "bash", "", "",
				"# the curriculum index: groups + topic summaries with sectionCount\n"+
					"curl -s localhost:8100/api/content | python3 -m json.tool"),
			code("term-detail", "bash", "", "",
				"# this topic, in full: sections → text/code/list/demo blocks\n"+
					"curl -s localhost:8100/api/content/go-language | python3 -m json.tool"),
			code("term-demo", "bash", "", "",
				"# the demo the runtime-types section renders — live reflect facts\n"+
					"curl -s localhost:8100/api/demo/go/runtime-types | python3 -m json.tool"),
			code("term-oneliner", "bash", "", "",
				"# no jq required: a python one-liner over the index\n"+
					"python3 -c \"import json,urllib.request as u; print([t['id'] for t in json.load(u.urlopen('http://localhost:8100/api/content'))['topics']])\""),
			code("term-scenario", "bash", "", "",
				"# the runtime toggle every demo reads (fake/live data, failure injection)\n"+
					"curl -s localhost:8100/api/demo/scenario"),
			code("term-404", "bash", "", "",
				"# unknown topic → the shared error envelope + x-request-id\n"+
					"curl -s -i localhost:8100/api/content/nope"),
			text("term-coda",
				"Every failure you can provoke comes back in one envelope — "+
					"{error: {code, message, requestId}} — with an x-request-id header matching "+
					"the server logs; the taxonomy behind it gets its deep dive in topic 30. "+
					"For the same surface rendered instead of raw, the Vite dev server proxies "+
					"/api on :5175 — but that is the client shell's story, not the language's."),
		),
	}

	if buildErr != nil {
		return Topic{}, buildErr
	}
	return NewTopic(
		"go-language",
		"The Go language",
		"The language tour this server actually uses — from variables and control flow "+
			"to what types mean at runtime.",
		"foundations",
		1,
		sections,
	)
}
