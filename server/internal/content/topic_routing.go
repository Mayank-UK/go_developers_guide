package content

import (
	"errors"
	"fmt"
)

// This file is topic 07 routing (group "request-handling", order 7): the
// ServeMux pattern router end to end, at production depth. Topic 04
// introduced patterns as part of the net/http fundamentals; this topic
// owns the subject: the full pattern grammar, wildcards and PathValue,
// query parsing, matching precedence with the 404/405/307 semantics this
// server really serves, sub-router mounting with http.StripPrefix, and
// the flat table at scale. The live demo (routing:explorer) sits INLINE
// in the matching section; every must-source block quotes a real repo
// file — the composition root's route table, the error-envelope
// middleware, and this topic's own matcher builder and test wiring; the
// topic closes with the terminal companion.

// routingTopic builds topic 07 — routing as net/http actually does it:
// one ServeMux, one pattern table, method-aware and wildcard-capable,
// with failure semantics the middleware chain preserves. The demo's
// matcher is verified against a real ServeMux by its own tests, so every
// claim this topic makes about 404, 405, Allow, and 307 is pinned
// executable fact, not recollection.
func routingTopic() (Topic, error) {
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
		// ── 1. The route table ─────────────────────────────────────────
		section("route-table", "The route table: methods and paths",
			"One mux, one table in the composition root — the API's table of contents, enforced by the router.",
			text("rt-pitch",
				"Routing in Go is ``http.ServeMux``, and since Go 1.22 its ``Handle`` method takes pattern "+
					"strings that carry the method and the path shape in one declaration: ``GET "+
					"/api/content/{topicId}``. Topic 04 introduced that idea in a paragraph; this topic "+
					"is the full treatment, because routing at production depth is where the honest "+
					"questions live. Which pattern wins when two match? What exactly comes back when "+
					"none does — or when the path is right and the method is wrong? Where does the "+
					"query string fit, and where does it NOT fit? How do independently grown routers "+
					"compose under one prefix? Every one of those questions has a precise answer in "+
					"the modern ServeMux, and this server is small enough that the whole answer is "+
					"visible in one place — the composition root's route table:"),
			code("rt-table-code", "go",
				"The whole router, verbatim: every pattern this server has, wired to its handler.",
				"server/cmd/guide/main.go",
				"mux := http.NewServeMux()\n"+
					"mux.Handle(\"GET /api/health\", presentation.NewHealthHandler(cfg.Env))\n"+
					"mux.Handle(\"GET /api/content\", presentation.NewContentListHandler(registry))\n"+
					"mux.Handle(\"GET /api/content/{topicId}\", presentation.NewContentTopicHandler(registry, logger))\n"+
					"mux.Handle(\"GET /api/demo/scenario\", presentation.NewScenarioGetHandler(scenarioStore))\n"+
					"mux.Handle(\"PATCH /api/demo/scenario\", presentation.NewScenarioPatchHandler(scenarioStore, logger))\n"+
					"// …\n"+
					"mux.Handle(\"GET /api/demo/iface/matrix\", presentation.NewInterfaceMatrixHandler())\n"+
					"mux.Handle(\"GET /api/demo/conc/context\", presentation.NewContextCancellationHandler())\n"+
					"mux.Handle(\"GET /api/demo/conc/race\", presentation.NewRaceDetectorHandler())"),
			text("rt-reading",
				"Read the table as a contract registry: one line per route, a method, a path shape, and an "+
					"``http.Handler`` value. Twelve lines at this topic's authoring time, growing by one "+
					"line per landed topic — including the line this topic adds for its own demo. The "+
					"method prefix is load-bearing: because ``GET /api/content`` is registered and nothing "+
					"else matches that path, a ``POST /api/content`` is rejected by the router itself with "+
					"405 and an ``Allow`` header before any handler code runs. The pattern grammar those "+
					"lines speak is small enough to state in full:"),
			list("rt-grammar", false,
				"Method — an optional leading verb (``GET``, ``PATCH``…); a GET pattern also serves HEAD; a "+
					"pattern with no method matches every method.",
				"Host — an optional host before the path (``example.com/``) scopes matching to that Host "+
					"header; this repo registers none.",
				"Literal segment — plain path text, matched case-sensitively, byte for byte.",
				"Wildcard {name} — matches exactly one non-empty path segment and captures it for "+
					"``r.PathValue``; wildcards must be full segments, never partial text like ``/b_{id}``.",
				"Trailing wildcard {name...} — matches the rest of the path including slashes; only "+
					"valid as the final segment.",
				"Trailing slash — ``/assets/`` is a subtree: an anonymous ``...`` matching everything "+
					"under the prefix.",
				"End anchor {$} — matches only the end of the path, so ``/{$}`` matches exactly the root "+
					"while ``/`` alone matches everything."),
			text("rt-boot-failures",
				"The router fails LOUDLY, at boot, and that is a feature worth internalizing early. A "+
					"syntactically invalid pattern — ``/b_{bucket}``, a stray brace, an empty segment — "+
					"panics inside the ``Handle`` call itself, so the composition root dies at startup "+
					"with the pattern named. A pattern that conflicts with one already registered panics "+
					"too, and the message is surgical: registering ``GET /b/{y}`` beside ``GET /{x}/a`` "+
					"produces ``both match some paths, like \"/b/a\". But neither is more specific than "+
					"the other`` — the exact colliding request, spelled out. A router that refuses to "+
					"boot over an ambiguous table is telling you the truth early: shadowed routes in "+
					"other ecosystems are among the longest-lived production bugs, and Go's stance is "+
					"that they should be impossible to deploy."),
			text("rt-escaping",
				"One rule about escaping does more work than its size suggests: matching runs on the "+
					"ESCAPED path, segment by segment. A request for ``/api/content/abc%2Fdef`` is one "+
					"wildcard's worth of segment — ``%2F`` stays inside the segment and never splits it — "+
					"and ``r.PathValue`` then reports the value UNESCAPED, ``abc/def``. Both halves are "+
					"pinned live by this topic's demo. The router also sanitizes before matching: "+
					"repeated slashes and ``.``/``..`` segments are redirected to the cleaned path "+
					"(307, coming below), while escaped lookalikes such as ``%2e`` are left alone — "+
					"only literal separators and dot segments are cleaned. Hostnames have their port "+
					"stripped. None of this is configurable; it is the mux being a well-raised member "+
					"of a server stack."),
		),
		// ── 2. Wildcards ───────────────────────────────────────────────
		section("wildcards", "Wildcards: {id}, {name...}, and r.PathValue",
			"Capturing path data is the router's only job beyond dispatch — three forms, one accessor.",
			text("wc-single",
				"The single-segment wildcard is the workhorse: ``{topicId}`` in ``GET "+
					"/api/content/{topicId}`` matches exactly one non-empty path segment and captures "+
					"it under its name. The handler reads the capture with ``r.PathValue(\"topicId\")`` — "+
					"no map lookups, no parsing, no arguments-threading through middleware; the value is "+
					"simply there, on the request the router handed over. The content detail handler is "+
					"the complete production example, and the shape is worth pausing on: the handler "+
					"reads its capture, asks the validated registry, and either writes the payload or "+
					"delegates to the central error writer. Routing found the request its handler; what "+
					"the capture MEANS is entirely the domain's business:"),
			code("wc-pathvalue-code", "go",
				"The capture arrives on the request; the handler reads it once and delegates.",
				"server/internal/presentation/content.go",
				"topicID := r.PathValue(\"topicId\")\n"+
					"topic, ok := h.registry.TopicByID(topicID)\n"+
					"if !ok {\n"+
					"    core.WriteError(w, r, core.Newf(core.KindNotFound, \"Topic %q not found\", topicID), h.logger)\n"+
					"    return\n"+
					"}"),
			text("wc-multiple-and-multi",
				"Wildcards compose: a pattern may carry several, ``GET /b/{bucket}/o/{object}`` capturing "+
					"two, each under its own name, and a trailing ``{name...}`` swallows the REST of the "+
					"path — slashes included — which is why it is only legal as the final segment. The "+
					"multi form differs from a subtree in one important way: it has a name, so "+
					"``GET /files/{path...}`` hands the handler ``path = \"a/b/c\"`` where ``GET "+
					"/files/`` hands it nothing and lets it read ``r.URL.Path``. A multi wildcard can "+
					"even capture the empty remainder — ``/files/`` matches with ``path = \"\"`` — but "+
					"the slashless ``/files`` does not match at all; it becomes a redirect candidate, "+
					"which the matching section demonstrates. Names must be valid Go identifiers, and "+
					"a name used twice in one pattern is a registration-time panic: the accessor takes "+
					"one key, so the grammar refuses the ambiguity before it can cost anyone an "+
					"afternoon."),
			text("wc-end-anchor",
				"The end anchor earns its keep the moment a subtree exists. ``/assets/`` matches the "+
					"whole subtree — including ``/assets/`` itself; ``/assets/{$}`` matches ONLY "+
					"``/assets/`` and lets some other pattern own the deeper paths. The same anchor "+
					"distinguishes ``/`` (every request — the catch-all of last resort) from ``/{$}`` "+
					"(exactly the root). This repo's table registers none of those, and that is a "+
					"decision to copy: a catch-all turns every typo into a 200 from an unintended "+
					"handler, while an unmatched path falls through to the mux's own 404 — which this "+
					"server's innermost middleware rewrites into the shared error envelope, so even "+
					"the router's failures speak the API's language."),
			text("wc-matcher",
				"How does a teaching guide demonstrate all this without shipping a second router to "+
					"production? The same way every demo in this guide earns its keep: a pure builder "+
					"in ``internal/domain/demos`` carries the guide's REAL route table as data and "+
					"resolves builder-chosen probes against it with a mini-matcher that implements "+
					"exactly the rules this section stated — and the package's tests run every probe "+
					"against a REAL ``ServeMux`` first and assert the two agree: outcome, status, "+
					"matched pattern, captures, Allow, redirect target. The single-wildcard rule reads "+
					"the same in the matcher as it does in the mux:"),
			code("wc-matcher-code", "go",
				"The demo matcher's wildcard rule — the same semantics the mux applies, quoted from the builder.",
				"server/internal/domain/demos/routing_explorer.go",
				"// A single-segment wildcard matches one non-empty segment: it ends\n"+
					"// at the next literal slash, never at an escaped %2F.\n"+
					"if got == \"\" {\n"+
					"    return nil, false\n"+
					"}\n"+
					"captures[seg.name] = got"),
			text("wc-coda",
				"Notice what wildcards did NOT bring with them: no typed captures. ``PathValue`` "+
					"returns a string, always — converting ``{id}`` to an integer is handler-side "+
					"work, with handler-side error handling, and topic 10's validation chapter owns "+
					"that boundary honestly (a ``{id}`` that cannot parse is a 404 about the resource, "+
					"not a 500 about the code). Some third-party routers offer typed params; the "+
					"stdlib's position is that one conversion at the edge, next to its error handling, "+
					"beats a parameter subsystem in the router. The at-scale section returns to that "+
					"trade."),
		),
		// ── 3. Query parsing ───────────────────────────────────────────
		section("query-parsing", "Query parsing: the part after the question mark",
			"The query string never routes — it is handler input, parsed by the handler, on purpose.",
			text("q-not-routing",
				"The first fact is a non-fact, stated firmly: ServeMux patterns never see the query "+
					"string. ``GET /api/content`` matches ``/api/content?verbose=1`` exactly as well "+
					"as the bare path — patterns are method-and-path contracts, full stop. That is a "+
					"deliberate split of concerns: the path names the resource (routing's business), "+
					"the query parameterizes the representation (the handler's business). Other "+
					"ecosystems blur this with optional-query route matching; Go's refusal to is why "+
					"a route table stays readable as a table of contents and why ``?debug=1`` can "+
					"never change which handler runs — a property you come to appreciate the first "+
					"time a query parameter silently reroutes a request elsewhere."),
			text("q-parse",
				"Parsing is one call: ``r.URL.Query()`` returns a ``url.Values`` — a "+
					"``map[string][]string`` — with keys and values percent-decoded. The rules that "+
					"surround that map are the ones that matter in production. ``Get`` returns the "+
					"FIRST occurrence of a key, and the map keeps every occurrence, so ``?tag=go&tag=routing`` "+
					"is readable as one value or both, explicitly. Blank values are data: ``?verbose`` "+
					"parses to an empty string, not a missing key. Each ``Query()`` call re-parses and "+
					"allocates, so a handler that consults the query repeatedly parses it once into a "+
					"local. And the sharp edge: ``Query()`` swallows malformed-escape errors, "+
					"returning whatever parsed — hostile input discipline says validate what you use "+
					"(topic 10), because the parser already decided not to fail for you. This "+
					"topic's demo does the parse the way a handler would, keeping every occurrence "+
					"in first-appearance order:"),
			code("q-parse-code", "go",
				"The demo's query binding: url.Values semantics, arrival order kept.",
				"server/internal/domain/demos/routing_explorer.go",
				"parsed, err := url.ParseQuery(rawQuery)\n"+
					"if err != nil {\n"+
					"    panic(fmt.Sprintf(\"routing explorer: probe query %q is not a valid query string: %v\", rawQuery, err))\n"+
					"}\n"+
					"// …\n"+
					"for _, key := range order {\n"+
					"    values = append(values, RoutingValue{Name: key, Values: parsed[key], Source: \"query\"})\n"+
					"}"),
			text("q-first-vs-all",
				"The first-occurrence rule deserves its own sentence because it differs across the "+
					"family: a dict-style parser keeps the LAST occurrence of a duplicated key, and "+
					"handlers ported between the two silently disagree about ``?page=1&page=2``. Go's "+
					"answer is to keep ALL of them and let ``Get`` define \"first\" explicitly — the "+
					"demo's query probe carries ``?verbose=1&tag=go&tag=routing`` and reports "+
					"``tag = [go, routing]``, both occurrences, in the order they arrived. When a "+
					"parameter is single-valued by contract, validate that: rejecting a duplicate "+
					"is one ``len`` check, and it is cheaper than debugging which one won."),
			text("q-coda",
				"Where the query belongs in API design — filtering, pagination, sorting as "+
					"representation parameters rather than path structure — is topic 11's "+
					"resource-design chapter. The mechanical split this section pinned is the one "+
					"routing needs: path values are typed-by-position and guaranteed present by the "+
					"pattern; query values are optional-by-nature, stringly, and entirely the "+
					"handler's to interpret. Both ride the same request; only one of them chose the "+
					"handler."),
		),
		// ── 4. Matching: precedence, 404, 405, 307 ──────────────────────
		section("matching", "Matching: precedence, 404, 405, and the 307s",
			"Which pattern wins, and exactly what the router says when none does — this section's demo runs it live.",
			text("m-precedence",
				"When several patterns match one request, the most specific wins — and \"specific\" "+
					"is a precise term, not a heuristic. The toolchain's own documentation states the "+
					"rule:"),
			code("m-doc-quote", "text",
				"net/http's own words (go doc net/http.ServeMux, Precedence section).",
				"",
				"If two or more patterns match a request, then the most specific pattern\n"+
					"takes precedence. A pattern P1 is more specific than P2 if P1 matches a\n"+
					"strict subset of P2’s requests; that is, if P2 matches all the requests of\n"+
					"P1 and more. If neither is more specific, then the patterns conflict.\n"+
					"There is one exception to this rule, for backwards compatibility: if two\n"+
					"patterns would otherwise conflict and one has a host while the other does\n"+
					"not, then the pattern with the host takes precedence. If a pattern passed\n"+
					"to ServeMux.Handle or ServeMux.HandleFunc conflicts with another pattern\n"+
					"that is already registered, those functions panic."),
			text("m-precedence-read",
				"In practice the subset rule reduces to segment arithmetic. A literal segment beats "+
					"a wildcard at the same position, so ``GET /api/content/new`` and ``GET "+
					"/api/content/{topicId}`` coexist, the literal taking exactly its own path and "+
					"the wildcard everything else. A longer literal prefix beats a shorter one: "+
					"``/images/thumbnails/`` is more specific than ``/images/``, so a thumbnails "+
					"handler and a subtree handler compose without ambiguity. And when neither "+
					"pattern's requests are a subset of the other's — ``/{x}/a`` versus ``/b/{y}``, "+
					"both matching ``/b/a`` — the patterns conflict and registration panics, which "+
					"is the boot-time guarantee from the route-table section doing its job. There "+
					"is no registration-order tiebreak anywhere in the modern mux; order never "+
					"matters, only specificity."),
			demo("routing:explorer", "The route table under test — match, 405, 404, 307",
				"Performed on every request: this topic's matcher carries the guide's REAL route table "+
					"(patterns and handler constructors, carried as data) and resolves nine probes "+
					"against it — the exact-literal match, the wildcard capture, HEAD riding a GET "+
					"pattern, a query that never routes, the 405 with its mux-computed Allow (GET "+
					"expanded with the HEAD it implies; two patterns on one path unioned), the "+
					"single-segment 404, the trailing-slash 404, and the escaped-path capture "+
					"(topicId=abc/def). Two labeled exhibits add what the real table cannot show: a "+
					"literal beside the wildcard (precedence) and a subtree with its 307 redirect. "+
					"Every verdict is verified against a real ServeMux by the builder's tests."),
			text("m-beats-intro",
				"Read the demo in four beats:"),
			list("m-beats", false,
				"Routes — the composition root's table as data: method, pattern, shape "+
					"(exact/wildcard/subtree), and the handler constructor each line wires.",
				"Probes — nine (method, path) pairs resolved live by the mini-matcher, each with its "+
					"verdict, bound path and query values, and a reasoning sentence stating the rule "+
					"that decided it.",
				"Exhibits — the labeled what-ifs: a literal sibling proving precedence, a subtree "+
					"proving the 307; exhibit-only patterns are marked by their missing handler.",
				"Semantics — the verified rules as checkable facts: GET serves HEAD, 307 subtree "+
					"redirects, no reverse-slash redirect, escaped-path matching, unescaped "+
					"PathValue."),
			text("m-405",
				"The 405 is the router's most informative failure, and its details are worth quoting "+
					"exactly. When a path matches some patterns' paths but not their methods, the "+
					"mux answers 405 with an ``Allow`` header computed from the PATTERNS — every "+
					"method the path actually declares, GET patterns expanded with the HEAD they "+
					"imply, sorted. ``POST /api/content/routing`` gets ``Allow: GET, HEAD`` even "+
					"though only GET was registered; ``DELETE /api/demo/scenario`` — a path shared "+
					"by the GET and PATCH scenario patterns — gets ``Allow: GET, HEAD, PATCH``. The "+
					"demo's matcher computes the same sets from the same rule, quoted from its "+
					"source:"),
			code("m-allow-code", "go",
				"Allow as the mux computes it: patterns unioned, GET expanded with HEAD, sorted.",
				"server/internal/domain/demos/routing_explorer.go",
				"switch route.method {\n"+
					"case \"\":\n"+
					"    // Unreachable behind a 405: a method-less pattern that\n"+
					"    // path-matches would have matched the request outright.\n"+
					"    return nil\n"+
					"case \"GET\":\n"+
					"    seen[\"GET\"] = true\n"+
					"    seen[\"HEAD\"] = true\n"+
					"default:\n"+
					"    seen[route.method] = true\n"+
					"}"),
			text("m-envelope",
				"This server wraps the mux's failures in the shared error envelope, and the 405 is "+
					"the case that must be handled most carefully: the status must stay 405 (it is "+
					"the truth) and the ``Allow`` header must survive (it is the useful part), while "+
					"the plain-text canned body becomes the taxonomy envelope — bucketed "+
					"``not_found``, the closest client-error kind the family contract defines. The "+
					"innermost middleware does exactly this rewrite, and the 404 branch beside it "+
					"takes the same shape:"),
			code("m-envelope-code", "go",
				"The mux's canned failures become the envelope; the 405 keeps status and Allow.",
				"server/internal/presentation/middleware.go",
				"if i.status == http.StatusNotFound {\n"+
					"    core.WriteError(i.w, r, core.NotFound(\"Not Found\"), logger)\n"+
					"    return\n"+
					"}\n"+
					"core.WriteErrorStatus(i.w, r, i.status, core.New(core.KindForHTTPStatus(i.status), \"Method Not Allowed\"), logger)"),
			text("m-redirect",
				"The 307s are the router's third verdict, and their logic is asymmetric on purpose. "+
					"A SUBTREE root requested without its slash — ``/assets`` when ``/assets/`` is "+
					"registered — answers ``307 Temporary Redirect`` to the slashed form: the "+
					"method is preserved (a POST subtree redirects as a POST), the query string "+
					"rides along, and the browser or curl re-issues the request unchanged at the "+
					"new target. But the reverse is nowhere: an exact pattern does not absorb "+
					"``/api/content/`` — that request is a plain 404, because a trailing slash on "+
					"an exact route is a client mistake worth surfacing, not a resource worth "+
					"canonicalizing. The same 307 handles sanitization: ``//api/content`` and "+
					"``/x/./y`` redirect to their cleaned forms before matching ever runs. The "+
					"matcher implements this order verbatim — sanitize first, then the "+
					"method-aware slash toggle:"),
			code("m-redirect-code", "go",
				"The matcher's resolution order: sanitize, match, method-aware slash toggle, then 405/404.",
				"server/internal/domain/demos/routing_explorer.go",
				"routePath := sanitizePath(rawPath)\n"+
					"if routePath != rawPath {\n"+
					"    probe.Outcome = outcomeRedirect\n"+
					"    probe.Status = 307\n"+
					"    probe.RedirectTarget = routePath + querySuffix\n"+
					"    return probe\n"+
					"}\n"+
					"// …\n"+
					"if routePath != \"/\" && !strings.HasSuffix(routePath, \"/\") {\n"+
					"    slashed, _ := findBest(routes, spec.method, escapeSegments(routePath+\"/\"))\n"+
					"    if slashed != nil {\n"+
					"        probe.Outcome = outcomeRedirect\n"+
					"        probe.Status = 307\n"+
					"        probe.RedirectTarget = routePath + \"/\" + querySuffix\n"+
					"        return probe\n"+
					"    }\n"+
					"}"),
			text("m-head-options",
				"Two method footnotes complete the semantics. HEAD rides GET automatically — the "+
					"mux serves a HEAD request with the GET pattern's handler and the server drops "+
					"the body on the wire, so health checks and precondition probes need no "+
					"separately declared route (contrast stacks where a GET-only route answers "+
					"HEAD with 405). OPTIONS gets nothing automatic: no implicit OPTIONS "+
					"enumeration, no automatic CORS preflight — a preflight is answered by "+
					"middleware this guide adds in topic 25, deliberately, not by the router "+
					"guessing. The one method rule left is the mounting section's opening move: "+
					"a pattern with NO method matches every method, and that is exactly how a "+
					"mounted area router says ``everything under this prefix is mine``."),
		),
		// ── 5. Mounting sub-routers ────────────────────────────────────
		section("mounting", "Mounting sub-routers: http.StripPrefix",
			"When areas own their routers: compose muxes under a prefix, stdlib-only.",
			text("mt-why",
				"A flat table in one composition root is the honest default, and this server keeps "+
					"it — but the moment routers are owned by different areas, versions, or teams, "+
					"the question becomes how to compose them. The stdlib's answer predates "+
					"patterns and still fits them perfectly: each area builds its OWN "+
					"``http.ServeMux`` with LOCAL patterns (short paths, no prefix), and the root "+
					"mounts it under the area's prefix. What the 1.22 pattern language changed is "+
					"how rarely you NEED this — methods and wildcards removed most of the pressure "+
					"that drove teams to per-area routers — not the idiom itself, which remains "+
					"the composition tool for versioned APIs (``/api/v1/``), health-and-metrics "+
					"ports, and genuinely independent subsystems."),
			text("mt-stripprefix",
				"The mount is one function: ``http.StripPrefix`` returns a handler that removes the "+
					"prefix from ``r.URL.Path`` before delegating, so the area mux sees ``/explorer`` "+
					"when the world asked for ``/api/demo/routing/explorer``. Two details make or "+
					"break a mount. The strip string must EQUAL the mount pattern's path — a "+
					"mismatch leaves a leading slash or a segment behind, and the area's patterns "+
					"silently stop matching. And the mount pattern itself is method-less with a "+
					"trailing slash — ``/api/demo/routing/`` — so every method and every sub-path "+
					"flows through to the area, whose own method-aware patterns then produce the "+
					"truthful 405s. The routing demo's test wiring is the repo's one real mount, "+
					"built exactly this way:"),
			code("mt-mount-code", "go",
				"A real mount from this repo's tests: area-owned patterns, prefix stripped, chain outside.",
				"server/internal/presentation/demo_routing_test.go",
				"area := http.NewServeMux()\n"+
					"area.Handle(\"GET /explorer\", NewRoutingExplorerHandler())\n"+
					"\n"+
					"mux := http.NewServeMux()\n"+
					"mux.Handle(\"/api/demo/routing/\", http.StripPrefix(\"/api/demo/routing\", area))"),
			text("mt-hygiene",
				"Mounting rules that keep the composition honest: one owner per prefix — a path "+
					"reachable through two mounts is a routing bug wearing a costume; middleware "+
					"wraps the MOUNT, not the area (chain outside the ``StripPrefix`` so request "+
					"ids, logging, and recovery still see the FULL original path in ``r.URL.Path`` "+
					"— the demo's test server chains around the outer mux exactly so); and the "+
					"redirect from the mounting section becomes provable — requesting the mount "+
					"root without its slash, ``/api/demo/routing``, answers the subtree 307 to "+
					"``/api/demo/routing/`` with the method preserved. The terminal companion "+
					"probes that live."),
			text("mt-when",
				"The decision rule, stated once: keep the flat table while every line still reads "+
					"like a table of contents — a dozen lines in the composition root IS "+
					"documentation. Introduce a mount when an area genuinely owns its routing "+
					"(its own release cadence, its own 404 semantics, a versioned surface), not "+
					"merely because the table got longer; length is what prefixes and wildcards "+
					"are for. And when you find yourself mounting more than two or three areas "+
					"with shared per-area middleware, that is the signal the at-scale section "+
					"addresses directly below."),
		),
		// ── 6. Routing at scale ────────────────────────────────────────
		section("routing-at-scale", "Routing at scale",
			"How far the flat table really goes, and what chi adds when it stops going.",
			text("sc-flat",
				"The flat table travels further than its reputation. Matching is a walk over a "+
					"trie keyed by segments — cost proportional to the path's length, not the "+
					"table's — and the boot-time conflict check means a growing table cannot grow "+
					"ambiguity. Hundreds of routes in one composition root is workable, with one "+
					"structural caveat this guide hit honestly while building the demo: "+
					"``ServeMux`` exposes NO way to enumerate a registered table — there is no "+
					"``Patterns()`` method (as of the Go this server pins), so a running mux "+
					"cannot tell you what it routes. That is precisely why this topic's demo "+
					"carries the table as DATA beside the mux, and why the family discipline is "+
					"to keep the real registration lines in one visible place — main.go — where "+
					"reading them never requires reflection against a running process."),
			text("sc-structure",
				"When the table needs structure, the stdlib's tools are the ones this topic "+
					"already taught. Per-area routers with ``StripPrefix`` mounts are the "+
					"decomposition unit — areas own patterns, the root owns prefixes. Per-subtree "+
					"middleware is the same shape one level up: mount a ``Chain(...)``-wrapped "+
					"handler instead of the bare area mux, and the whole subtree shares the "+
					"chain while the rest of the table stays bare — the mounting section's test "+
					"wiring is exactly this composition. What the stdlib genuinely does not have "+
					"is route METADATA: named routes for metrics labels, URL builders that "+
					"reverse a pattern into a path, introspection for docs. On stdlib you get "+
					"those by wrapping handlers (a decorating handler that counts by pattern "+
					"string, as this server's access log already does with method and path) — "+
					"or by adopting a router whose table IS data."),
			text("sc-chi",
				"``chi`` remains the smallest honest step away, and the routing-shaped reasons "+
					"are concrete: ``r.Route(\"/api\", ...)`` groups with inline sub-routers "+
					"(mounting, first-class), middleware scoped per group without manual "+
					"``StripPrefix`` wiring, ``r.URLParam`` access to wildcards, and a route "+
					"table you can walk for metrics and docs. Because chi handlers and middleware "+
					"keep the ``net/http`` shapes, migration between it and the stdlib mux stays "+
					"mechanical — the types never changed, only the table's ergonomics. ``gin`` "+
					"and ``echo`` revisit topic 04's trade at routing scale: their context types "+
					"bundle params, binding, and rendering into one ergonomic value, and their "+
					"routers gain URL builders and route naming — at the price of leaving the "+
					"portable handler shape behind. One Go-specific fact tilts many teams back "+
					"to stdlib-shaped handlers: spec-first codegen (topic 12's oapi-codegen) "+
					"EMITS net/http handlers, so a stdlib-shaped table composes with generated "+
					"servers for free."),
			list("sc-decision", false,
				"Flat table — the default: one mux in the composition root, patterns as the "+
					"table of contents; add a line per route and let boot-time checks police it.",
				"Mount — when an area owns its routing: area mux with local patterns behind "+
					"StripPrefix, one owner per prefix, middleware chained outside the mount.",
				"chi — when the table needs to be data: groups, per-subtree middleware, route "+
					"names, and introspection, all still net/http-shaped underneath.",
				"Framework context — when ergonomics dominate: gin/echo bundle params, binding, "+
					"and rendering; accept the lock-in consciously, having read topic 04's "+
					"comparison.",
				"This guide — stdlib by design: the backbone every Go engineer can read from the "+
					"standard library's own documentation, frameworks taught at the moment they "+
					"earn their keep."),
		),
		// ── 7. Terminal companion ──────────────────────────────────────
		section("terminal-companion", "Terminal companion: the curl tour",
			"The router's four verdicts, provoked from the shell.",
			text("term-pitch",
				"With the stack up (server on :8100), these commands walk the exact surfaces this "+
					"topic taught: the wildcard route in action, the demo's own payload, and all "+
					"three failure verdicts — the 405 with its truthful Allow, the 404 that a "+
					"trailing slash earns, and the 307 a subtree root gets. ``-i`` is the tour's "+
					"most important flag: the headers ARE the routing story. The explorer's own "+
					"mount lands with this topic's wiring, so its 307 is live from day one:"),
			code("term-topic", "bash", "", "",
				"# this topic, in full — routed by the wildcard you are about to probe\n"+
					"curl -s localhost:8100/api/content/routing | python3 -m json.tool"),
			code("term-demo", "bash", "", "",
				"# the demo: the real route table as data, nine probes, two exhibits, semantics facts\n"+
					"curl -s localhost:8100/api/demo/routing/explorer | python3 -m json.tool"),
			code("term-405", "bash", "", "",
				"# wrong method on the wildcard path -> 405, Allow: GET, HEAD, in the error envelope\n"+
					"curl -s -i -X POST localhost:8100/api/content/routing"),
			code("term-404", "bash", "", "",
				"# the trailing slash the exact pattern refuses -> plain 404 in the same envelope\n"+
					"curl -s -i localhost:8100/api/content/"),
			code("term-307", "bash", "", "",
				"# the explorer's mount root without its slash -> 307, Location: /api/demo/routing/\n"+
					"curl -s -i localhost:8100/api/demo/routing"),
			code("term-index", "bash", "", "",
				"# the curriculum index: routing sits in group \"request-handling\", order 7\n"+
					"curl -s localhost:8100/api/content | python3 -m json.tool"),
			text("term-coda",
				"Run the 405 and the 404 back to back and read the whole middleware story in two "+
					"responses: the same envelope shape, the same x-request-id discipline, one "+
					"verdict from the pattern's method and one from its path shape. Then diff the "+
					"demo payload between two calls — it is deterministic by contract, so any "+
					"drift is a bug, and the builder's tests would already know. Next stop in the "+
					"request-handling group: topic 08 wraps this router in the middleware chain "+
					"and shows the pipeline running live."),
		),
	}

	if buildErr != nil {
		return Topic{}, buildErr
	}
	return NewTopic(
		"routing",
		"Routing",
		"ServeMux patterns end to end — wildcards and PathValue, query parsing, precedence "+
			"with the real 404/405/307 semantics, mounting sub-routers, and the flat table at scale.",
		"request-handling",
		7,
		sections,
	)
}
