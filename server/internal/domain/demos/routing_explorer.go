package demos

import (
	"fmt"
	"net/url"
	"path"
	"sort"
	"strings"
)

// Routing-explorer demo payload: match (method, path) probes against the
// guide's real route table, carried as data.
//
// ``GET /api/demo/routing/explorer`` serves this payload. The demo replays
// what net/http's ServeMux does for every inbound request since Go 1.22 —
// take a method and a path, resolve them against the declared patterns, and
// report the outcome: the matched pattern with its captured path values and
// parsed query parameters, a 405 with the Allow header the mux computes, a
// 307 trailing-slash redirect, or a 404.
//
// The declared table is the exact route set server/cmd/guide/main.go wires
// (patterns and handler constructors, carried as data at this topic's
// authoring time). The matching itself is an honest simulation of ServeMux
// semantics for that table, every rule of which was verified empirically
// against a real ServeMux before it was encoded here:
//
//   - A GET pattern also serves HEAD; a pattern with no method serves every
//     method.
//   - Matching happens on the ESCAPED path, segment by segment, so %2F
//     inside a segment never splits it; path values are reported unescaped.
//   - The most specific matching pattern wins: a literal segment beats a
//     wildcard at the same position; unresolvable overlaps panic at
//     registration, so they cannot exist in a booted table.
//   - A path that matches no pattern's path but shares one with other
//     methods gets 405 with a SORTED Allow that includes HEAD for GET
//     patterns ("GET, HEAD", "GET, HEAD, PATCH").
//   - A subtree root requested without its trailing slash gets a 307
//     Temporary Redirect (method-preserving, query preserved) to the
//     slashed form; there is NO reverse redirect — an exact pattern does
//     not absorb "/api/content/".
//   - Requests needing sanitization (repeated slashes, "." or ".."
//     segments) are redirected 307 to the clean path before matching.
//
// The builder is pure: no inputs, no I/O, no mutable state — the probe set
// is builder-chosen exhibit data, so the endpoint stays a zero-input GET
// with a deterministic payload.
//
// Deliberate subset: this matcher supports what the carried table and
// exhibits need — exact literals, single-segment {name} wildcards,
// {name...} trailing wildcards, and trailing-slash subtrees, with optional
// methods. Host patterns, "{$}", and the bare "/" catch-all are rejected at
// compile time; none appears in the guide's table.

// Routing outcome vocabulary — the four things a router can say.
const (
	outcomeMatched   = "matched"
	outcomeMethod    = "method_not_allowed"
	outcomeRedirect  = "redirect"
	outcomeNotFound  = "not_found"
	routeKindExact   = "exact"
	routeKindSingle  = "wildcard"
	routeKindSubtree = "subtree"
	routeKindMulti   = "trailing-wildcard"
)

// RoutingPattern is one declared route: the registration line's method,
// pattern, and handler constructor, plus the matcher's classification of
// the pattern's shape.
type RoutingPattern struct {
	Method  string `json:"method"`
	Pattern string `json:"pattern"`
	Kind    string `json:"kind"`
	// Handler is the constructor main.go wires to the pattern; exhibit-only
	// patterns (hypothetical literals used to demonstrate precedence) omit it.
	Handler string `json:"handler,omitempty"`
}

// RoutingValue is one parameter the router would bind for the handler:
// a captured path segment or a parsed query parameter. Path values carry
// one element; query values carry every occurrence, in arrival order.
type RoutingValue struct {
	Name   string   `json:"name"`
	Values []string `json:"values"`
	Source string   `json:"source"` // "path" | "query"
}

// RoutingProbe is one simulated request line and the matcher's verdict on
// it. Absent facts are omitted, never null.
type RoutingProbe struct {
	Method         string         `json:"method"`
	Path           string         `json:"path"`
	Outcome        string         `json:"outcome"`
	Status         int            `json:"status"`
	MatchedPattern string         `json:"matchedPattern,omitempty"`
	PathValues     []RoutingValue `json:"pathValues,omitempty"`
	QueryValues    []RoutingValue `json:"queryValues,omitempty"`
	Allow          []string       `json:"allow,omitempty"`
	RedirectTarget string         `json:"redirectTarget,omitempty"`
	Reasoning      string         `json:"reasoning"`
}

// RoutingExhibit is a labeled pair of extra patterns the real table cannot
// demonstrate (the guide declares no overlapping or subtree routes), with
// probes resolved against them. Each exhibit's outcome is verified against
// a real ServeMux in this package's tests, exactly like the main probes.
type RoutingExhibit struct {
	ID       string           `json:"id"`
	Title    string           `json:"title"`
	Note     string           `json:"note"`
	Patterns []RoutingPattern `json:"patterns"`
	Probes   []RoutingProbe   `json:"probes"`
}

// RoutingSemantics pins the ServeMux rules the simulator implements as
// checkable facts, so the client can render them without hardcoding.
type RoutingSemantics struct {
	GetAlsoServesHead     bool `json:"getAlsoServesHead"`
	MethodlessMatchesAll  bool `json:"methodlessMatchesAll"`
	SubtreeRedirectStatus int  `json:"subtreeRedirectStatus"`
	// ReverseSlashRedirect is false: Go 1.22+ ServeMux redirects a subtree
	// root WITHOUT its slash to the slashed form, but never strips a slash —
	// "/api/content/" is a 404 beside the exact "/api/content".
	ReverseSlashRedirect bool `json:"reverseSlashRedirect"`
	MatchesEscapedPath   bool `json:"matchesEscapedPath"`
	PathValueIsUnescaped bool `json:"pathValueIsUnescaped"`
}

// RoutingExplorerPayload is the wire shape of GET /api/demo/routing/explorer
// (camelCase JSON) — the routing topic's inline demo.
type RoutingExplorerPayload struct {
	Routes    []RoutingPattern `json:"routes"`
	Probes    []RoutingProbe   `json:"probes"`
	Exhibits  []RoutingExhibit `json:"exhibits"`
	Semantics RoutingSemantics `json:"semantics"`
}

// ── the mini-matcher ─────────────────────────────────────────────────────

// routeSegment is one compiled pattern segment: a literal, a single-segment
// wildcard, or a trailing multi-segment wildcard.
type routeSegment struct {
	literal string
	name    string // wildcard name; empty for literals
	multi   bool   // {name...}: the pattern's last segment
}

// compiledRoute is a pattern ready to match: an optional method, the path's
// segments, and whether a trailing slash makes it a subtree.
type compiledRoute struct {
	method   string
	raw      string
	segments []routeSegment
	subtree  bool
}

// compileRoute parses "[METHOD ]/PATH" into a matcher-ready form. It
// implements exactly the subset this demo carries (see the package comment)
// and rejects everything else with an error naming the pattern.
func compileRoute(pattern string) (compiledRoute, error) {
	method, rest := "", pattern
	if idx := strings.IndexAny(pattern, " \t"); idx >= 0 {
		method, rest = pattern[:idx], strings.TrimLeft(pattern[idx+1:], " \t")
	}
	if !strings.HasPrefix(rest, "/") {
		return compiledRoute{}, fmt.Errorf("route pattern %q: host patterns and bare paths are outside this matcher's subset", pattern)
	}
	if strings.Contains(rest, "{$}") {
		return compiledRoute{}, fmt.Errorf("route pattern %q: {$} end anchors are outside this matcher's subset", pattern)
	}
	body := rest[1:]
	if body == "" {
		return compiledRoute{}, fmt.Errorf("route pattern %q: the bare catch-all is outside this matcher's subset", pattern)
	}
	subtree := strings.HasSuffix(body, "/")
	body = strings.TrimSuffix(body, "/")

	compiled := compiledRoute{method: method, raw: pattern, subtree: subtree}
	for i, raw := range strings.Split(body, "/") {
		if raw == "" {
			return compiledRoute{}, fmt.Errorf("route pattern %q: empty segment", pattern)
		}
		if !strings.HasPrefix(raw, "{") {
			if strings.ContainsAny(raw, "{}") {
				return compiledRoute{}, fmt.Errorf("route pattern %q: wildcards must be full segments (%q)", pattern, raw)
			}
			compiled.segments = append(compiled.segments, routeSegment{literal: raw})
			continue
		}
		if !strings.HasSuffix(raw, "}") {
			return compiledRoute{}, fmt.Errorf("route pattern %q: malformed wildcard segment %q", pattern, raw)
		}
		inner := raw[1 : len(raw)-1]
		multi := strings.HasSuffix(inner, "...")
		if multi {
			inner = strings.TrimSuffix(inner, "...")
			if i != len(strings.Split(body, "/"))-1 {
				return compiledRoute{}, fmt.Errorf("route pattern %q: a {...} wildcard must be the last segment", pattern)
			}
		}
		if inner == "" {
			return compiledRoute{}, fmt.Errorf("route pattern %q: nameless wildcard segment %q", pattern, raw)
		}
		compiled.segments = append(compiled.segments, routeSegment{name: inner, multi: multi})
	}
	return compiled, nil
}

// escapeSegments splits an escaped request path on literal slashes only —
// %2F stays inside its segment, which is the whole point of matching on the
// escaped path.
func escapeSegments(escaped string) []string {
	if escaped == "/" {
		return nil
	}
	return strings.Split(strings.TrimPrefix(escaped, "/"), "/")
}

// methodMatches applies the method rules: a method-less pattern serves
// every method; a GET pattern also serves HEAD; otherwise exact equality.
func methodMatches(patternMethod, requestMethod string) bool {
	if patternMethod == "" {
		return true
	}
	if patternMethod == requestMethod {
		return true
	}
	return patternMethod == "GET" && requestMethod == "HEAD"
}

// matchPath resolves a compiled pattern's path against the request's
// ESCAPED path segments, reporting the captured values (still escaped —
// the caller unescapes for display, like Request.PathValue does).
func (c compiledRoute) matchPath(segments []string) (map[string]string, bool) {
	lastMulti := len(c.segments) > 0 && c.segments[len(c.segments)-1].multi
	switch {
	case c.subtree:
		// A subtree pattern ("/assets/") needs the request to carry its
		// separating slash — strictly more segments than the literal
		// prefix — so "/assets" misses and becomes a redirect candidate.
		if len(segments) <= len(c.segments) {
			return nil, false
		}
	case lastMulti:
		// A trailing {...} consumes the remainder INCLUDING the empty one
		// ("/files/" captures ""), but its separating slash is still
		// required: "/files" misses.
		if len(segments) < len(c.segments) {
			return nil, false
		}
	default:
		if len(segments) != len(c.segments) {
			return nil, false
		}
	}

	captures := map[string]string{}
	for i, seg := range c.segments {
		got := segments[i]
		if seg.multi {
			captures[seg.name] = strings.Join(segments[i:], "/")
			break
		}
		if seg.literal != "" {
			if got != seg.literal {
				return nil, false
			}
			continue
		}
		// A single-segment wildcard matches one non-empty segment: it ends
		// at the next literal slash, never at an escaped %2F.
		if got == "" {
			return nil, false
		}
		captures[seg.name] = got
	}
	return captures, true
}

// segmentRank orders segment kinds by specificity: a literal segment is
// more specific than a wildcard, and a wildcard more than a subtree's
// anonymous continuation.
func segmentRank(seg routeSegment, exhausted bool) int {
	if exhausted {
		return 0
	}
	if seg.literal != "" {
		return 2
	}
	if seg.multi {
		return 0
	}
	return 1
}

// moreSpecific reports whether pattern a beats pattern b when both match a
// request: the FIRST differing position decides, literal over wildcard,
// wildcard over subtree continuation. This is the strict-subset rule from
// the ServeMux docs ("P1 is more specific than P2 if P1 matches a strict
// subset of P2's requests") reduced to a segment comparison.
func moreSpecific(a, b compiledRoute) bool {
	longest := len(a.segments)
	if len(b.segments) > longest {
		longest = len(b.segments)
	}
	for i := 0; i < longest; i++ {
		var as, bs routeSegment
		aExhausted, bExhausted := i >= len(a.segments), i >= len(b.segments)
		if !aExhausted {
			as = a.segments[i]
		}
		if !bExhausted {
			bs = b.segments[i]
		}
		ar, br := segmentRank(as, aExhausted), segmentRank(bs, bExhausted)
		if ar != br {
			return ar > br
		}
	}
	return false
}

// sanitizePath is net/http's cleanPath: fold repeated slashes, resolve "."
// and "..", preserve a trailing slash. A request whose path changes under
// it is redirected before matching ever happens.
func sanitizePath(p string) string {
	if p == "" {
		return "/"
	}
	if p[0] != '/' {
		p = "/" + p
	}
	trailing := len(p) > 1 && strings.HasSuffix(p, "/")
	cleaned := path.Clean(p)
	if trailing && !strings.HasSuffix(cleaned, "/") {
		cleaned += "/"
	}
	return cleaned
}

// probeSpec is one builder-chosen probe: a request line plus the authored
// reasoning sentence that explains the verdict.
type probeSpec struct {
	method    string
	path      string
	reasoning string
}

// simulate resolves one (method, path) against the compiled routes with
// ServeMux's resolution order: match; else the method-aware trailing-slash
// toggle; else 405 with the computed Allow; else 404. Sanitization comes
// first, exactly as the mux sanitizes before matching.
func simulate(routes []compiledRoute, spec probeSpec) RoutingProbe {
	rawPath, rawQuery, _ := strings.Cut(spec.path, "?")
	querySuffix := ""
	if rawQuery != "" {
		querySuffix = "?" + rawQuery
	}
	probe := RoutingProbe{Method: spec.method, Path: spec.path, Reasoning: spec.reasoning}

	routePath := sanitizePath(rawPath)
	if routePath != rawPath {
		probe.Outcome = outcomeRedirect
		probe.Status = 307
		probe.RedirectTarget = routePath + querySuffix
		return probe
	}
	segments := escapeSegments(routePath)

	best, captures := findBest(routes, spec.method, segments)
	if best != nil {
		probe.Outcome = outcomeMatched
		probe.Status = 200
		probe.MatchedPattern = best.raw
		if len(captures) > 0 {
			probe.PathValues = boundPathValues(captures)
		}
		if rawQuery != "" {
			probe.QueryValues = parseQueryValues(rawQuery)
		}
		return probe
	}

	// Subtree roots redirect to their slashed form — with the METHOD, so a
	// POST whose subtree pattern exists redirects too (and one whose does
	// not falls through to 405/404, never a lying redirect).
	if routePath != "/" && !strings.HasSuffix(routePath, "/") {
		slashed, _ := findBest(routes, spec.method, escapeSegments(routePath+"/"))
		if slashed != nil {
			probe.Outcome = outcomeRedirect
			probe.Status = 307
			probe.RedirectTarget = routePath + "/" + querySuffix
			return probe
		}
	}

	if allow := allowedMethods(routes, routePath, segments); len(allow) > 0 {
		probe.Outcome = outcomeMethod
		probe.Status = 405
		probe.Allow = allow
		return probe
	}
	probe.Outcome = outcomeNotFound
	probe.Status = 404
	return probe
}

// findBest returns the most specific pattern whose path AND method match,
// with its captures. Ties are impossible in a registered table (ServeMux
// panics on unresolvable overlaps at Handle time).
func findBest(routes []compiledRoute, method string, segments []string) (*compiledRoute, map[string]string) {
	var best *compiledRoute
	var bestCaptures map[string]string
	for i := range routes {
		route := routes[i]
		if !methodMatches(route.method, method) {
			continue
		}
		captures, ok := route.matchPath(segments)
		if !ok {
			continue
		}
		if best == nil || moreSpecific(route, *best) {
			best = &routes[i]
			bestCaptures = captures
		}
	}
	return best, bestCaptures
}

// allowedMethods unions the methods of every pattern whose PATH matches,
// expanding GET patterns with HEAD exactly as the mux's Allow header does,
// sorted — "GET, HEAD", "GET, HEAD, PATCH". Like the real mux's
// matchingMethods, a path that does not end in a slash also consults the
// SLASHED form, so a subtree's methods count for its slashless root.
func allowedMethods(routes []compiledRoute, routePath string, segments []string) []string {
	forms := [][]string{segments}
	if !strings.HasSuffix(routePath, "/") {
		forms = append(forms, escapeSegments(routePath+"/"))
	}
	seen := map[string]bool{}
	for _, form := range forms {
		for _, route := range routes {
			if _, ok := route.matchPath(form); !ok {
				continue
			}
			switch route.method {
			case "":
				// Unreachable behind a 405: a method-less pattern that
				// path-matches would have matched the request outright.
				return nil
			case "GET":
				seen["GET"] = true
				seen["HEAD"] = true
			default:
				seen[route.method] = true
			}
		}
	}
	allow := make([]string, 0, len(seen))
	for method := range seen {
		allow = append(allow, method)
	}
	sort.Strings(allow)
	return allow
}

// boundPathValues unescapes each capture (matching runs on the escaped
// path; PathValue returns the unescaped value) and orders them by the
// pattern's wildcard names. An invalid escape cannot occur in this demo's
// constants — net/http rejects such request lines during URL parsing, 400,
// before the router ever runs — so the raw segment is the honest fallback.
func boundPathValues(captures map[string]string) []RoutingValue {
	names := make([]string, 0, len(captures))
	for name := range captures {
		names = append(names, name)
	}
	sort.Strings(names)
	values := make([]RoutingValue, 0, len(names))
	for _, name := range names {
		raw := captures[name]
		unescaped, err := url.PathUnescape(raw)
		if err != nil {
			unescaped = raw
		}
		values = append(values, RoutingValue{Name: name, Values: []string{unescaped}, Source: "path"})
	}
	return values
}

// parseQueryValues parses a raw query string the way a handler's
// r.URL.Query does — url.Values semantics: every occurrence kept,
// first-appearance key order (maps do not iterate deterministically), blank
// values are data. The demo's query constants are valid by construction;
// a malformed escape here is a programmer bug in the probe table, and the
// panic names it.
func parseQueryValues(rawQuery string) []RoutingValue {
	parsed, err := url.ParseQuery(rawQuery)
	if err != nil {
		panic(fmt.Sprintf("routing explorer: probe query %q is not a valid query string: %v", rawQuery, err))
	}
	var order []string
	seen := map[string]bool{}
	for _, pair := range strings.Split(rawQuery, "&") {
		key, _, _ := strings.Cut(pair, "=")
		if unescaped, keyErr := url.QueryUnescape(key); keyErr == nil {
			key = unescaped
		}
		if !seen[key] {
			seen[key] = true
			order = append(order, key)
		}
	}
	values := make([]RoutingValue, 0, len(order))
	for _, key := range order {
		values = append(values, RoutingValue{Name: key, Values: parsed[key], Source: "query"})
	}
	return values
}

// ── the carried table and the exhibit data ───────────────────────────────

// guideRouteTable is the route set server/cmd/guide/main.go wires, carried
// as data: the registration line's method, pattern, and handler
// constructor, verbatim. ServeMux exposes no API to enumerate a registered
// table (verified: no Patterns method as of Go 1.27), so the table is
// carried here — the snapshot this topic landed with.
var guideRouteTable = []RoutingPattern{
	{Method: "GET", Pattern: "/api/health", Handler: "presentation.NewHealthHandler"},
	{Method: "GET", Pattern: "/api/content", Handler: "presentation.NewContentListHandler"},
	{Method: "GET", Pattern: "/api/content/{topicId}", Handler: "presentation.NewContentTopicHandler"},
	{Method: "GET", Pattern: "/api/demo/scenario", Handler: "presentation.NewScenarioGetHandler"},
	{Method: "PATCH", Pattern: "/api/demo/scenario", Handler: "presentation.NewScenarioPatchHandler"},
	{Method: "GET", Pattern: "/api/demo/go/runtime-types", Handler: "presentation.NewRuntimeTypesHandler"},
	{Method: "GET", Pattern: "/api/demo/runtime/concurrency", Handler: "presentation.NewConcurrencyHandler"},
	{Method: "GET", Pattern: "/api/demo/http/headers", Handler: "presentation.NewHTTPHeadersHandler"},
	{Method: "GET", Pattern: "/api/demo/gohttp/app-info", Handler: "presentation.NewAppInfoHandler"},
	{Method: "GET", Pattern: "/api/demo/iface/matrix", Handler: "presentation.NewInterfaceMatrixHandler"},
	{Method: "GET", Pattern: "/api/demo/conc/context", Handler: "presentation.NewContextCancellationHandler"},
	{Method: "GET", Pattern: "/api/demo/conc/race", Handler: "presentation.NewRaceDetectorHandler"},
}

// classifyKind derives a pattern's shape so the wire's kind column can
// never drift from the pattern string itself.
func classifyKind(pattern string) string {
	switch {
	case strings.HasSuffix(pattern, "/"):
		return routeKindSubtree
	case strings.Contains(pattern, "...}"):
		return routeKindMulti
	case strings.Contains(pattern, "{"):
		return routeKindSingle
	default:
		return routeKindExact
	}
}

// mustCompileRoute compiles a pattern from the static tables; a failure is
// a programmer bug in this file and deserves the loud failure.
func mustCompileRoute(methodAndPattern string) compiledRoute {
	compiled, err := compileRoute(methodAndPattern)
	if err != nil {
		panic(fmt.Sprintf("routing explorer: %v", err))
	}
	return compiled
}

// compileTable turns wire patterns into matcher routes (one entry per
// pattern, order preserved — registration order never matters for outcome,
// only specificity does).
func compileTable(table []RoutingPattern) []compiledRoute {
	routes := make([]compiledRoute, 0, len(table))
	for _, route := range table {
		routes = append(routes, mustCompileRoute(route.Method+" "+route.Pattern))
	}
	return routes
}

// guideProbes is the builder-chosen probe set: every outcome the real
// table can produce, including the escapes and the two probes that prove
// what the table does NOT do (no multi-segment wildcard, no free trailing
// slash).
var guideProbes = []probeSpec{
	{method: "GET", path: "/api/content",
		reasoning: "Matched the exact pattern GET /api/content: every segment is literal, so the router " +
			"captured nothing — the handler runs with no path values to read, and the wildcard sibling " +
			"never applied because {topicId} demands a third segment."},
	{method: "GET", path: "/api/content/routing",
		reasoning: "Matched GET /api/content/{topicId}: the literal prefix consumed /api/content and the " +
			"wildcard captured the third segment — the handler reads it with r.PathValue(\"topicId\") and " +
			"the registry never learns how the id arrived."},
	{method: "HEAD", path: "/api/content/routing",
		reasoning: "Matched the GET pattern under a HEAD request: a GET pattern also serves HEAD, so the " +
			"same handler answers with the body dropped — Go needs no separately declared HEAD route."},
	{method: "GET", path: "/api/content/routing?verbose=1&tag=go&tag=routing",
		reasoning: "Matched the wildcard pattern with the query string playing no part in matching: patterns " +
			"see the path only, and the handler parses the query itself — verbose arrives once, tag twice, " +
			"and a query parser that keeps every occurrence (url.Values) hands the handler both."},
	{method: "POST", path: "/api/content/routing",
		reasoning: "The path matches GET /api/content/{topicId} but the method does not, and no other " +
			"pattern wants this path — 405, with the mux-computed Allow header listing GET expanded with " +
			"the HEAD it implies, sorted."},
	{method: "DELETE", path: "/api/demo/scenario",
		reasoning: "Two patterns share this path — GET and PATCH /api/demo/scenario — and DELETE matches " +
			"neither: 405 with Allow uniting both patterns and expanding GET's implied HEAD: GET, HEAD, PATCH."},
	{method: "GET", path: "/api/content/a/b",
		reasoning: "No match: {topicId} is a SINGLE-segment wildcard ending at the next slash, and this path " +
			"has two trailing segments. A pattern that should own the rest would declare {name...} — the " +
			"table does not, so the mux answers 404."},
	{method: "GET", path: "/api/content/",
		reasoning: "No match, and no redirect: an exact pattern does not absorb a trailing slash — there is " +
			"no reverse-slash redirect in Go's pattern router. Only subtree roots (patterns ending in /) get " +
			"the 307 treatment, and this table registers none, so the trailing slash is a plain 404."},
	{method: "GET", path: "/api/content/abc%2Fdef",
		reasoning: "Matched with topicId=abc/def: matching runs on the ESCAPED path, so %2F stays inside the " +
			"segment instead of splitting it — one wildcard captured abc%2Fdef whole, and PathValue " +
			"reports it unescaped."},
}

// exhibitSpec is an exhibit's authored content: its wire facts plus the
// probe specs that resolve against its patterns at build time.
type exhibitSpec struct {
	id       string
	title    string
	note     string
	patterns []RoutingPattern
	probes   []probeSpec
}

// exhibitProbes resolve against their exhibit's patterns only, isolating
// behavior the real table cannot show because it declares no overlapping
// or subtree routes.
var exhibitSpecs = []exhibitSpec{
	{
		id:    "precedence",
		title: "Literal beats wildcard — the one overlap the table cannot show",
		note: "The guide declares no literal beside its wildcard, so this exhibit adds one hypothetical " +
			"registration (marked by its missing handler) beside the real GET /api/content/{topicId}. " +
			"Both patterns path-match /api/content/new; the literal is more specific — it matches a strict " +
			"subset of the wildcard's requests — so it takes the request without ambiguity. An overlap " +
			"neither pattern wins panics at registration instead; this pair resolves cleanly.",
		patterns: []RoutingPattern{
			{Method: "GET", Pattern: "/api/content/new", Kind: routeKindExact},
			{Method: "GET", Pattern: "/api/content/{topicId}", Kind: routeKindSingle, Handler: "presentation.NewContentTopicHandler"},
		},
		probes: []probeSpec{
			{method: "GET", path: "/api/content/new",
				reasoning: "Both patterns path-match — /api/content/{topicId} would happily capture new — but " +
					"the exact literal GET /api/content/new wins: most-specific-wins, literal over wildcard " +
					"at the deciding position, verified against a real ServeMux."},
		},
	},
	{
		id:    "subtree",
		title: "The trailing-slash subtree and its 307",
		note: "A pattern ending in / (an anonymous ... wildcard) matches everything under the prefix. " +
			"Requesting the subtree root WITHOUT the slash answers 307 Temporary Redirect — method- and " +
			"query-preserving — to the slashed form. Registering the slashless path separately overrides " +
			"the redirect; the {$} end anchor pins an exact root instead. Neither is wired here: the " +
			"exhibit carries the pattern as data, and this package's tests verify both outcomes against " +
			"a real ServeMux.",
		patterns: []RoutingPattern{
			{Method: "GET", Pattern: "/assets/", Kind: routeKindSubtree},
		},
		probes: []probeSpec{
			{method: "GET", path: "/assets",
				reasoning: "The subtree root without its slash: no pattern matches /assets as written, but " +
					"GET /assets/ exists — 307 to /assets/, the method preserved, so the retried GET (or " +
					"POST, when a POST subtree exists) lands on the real handler."},
			{method: "GET", path: "/assets/css/app.css",
				reasoning: "Matched the subtree pattern: the literal prefix /assets/ consumed its segments " +
					"and the anonymous trailing wildcard swallowed the rest — css/app.css never needed " +
					"per-file routes."},
		},
	},
}

// BuildRoutingExplorerPayload assembles the whole demo payload: the real
// route table as data, the probe set resolved by the mini-matcher, the two
// labeled exhibits, and the semantics facts the matcher was verified
// against. Pure: same call, same payload, no inputs, no I/O.
func BuildRoutingExplorerPayload() RoutingExplorerPayload {
	// The kind column is derived from the pattern string itself, so a
	// pattern edit can never leave a stale classification behind.
	table := make([]RoutingPattern, len(guideRouteTable))
	copy(table, guideRouteTable)
	for i := range table {
		table[i].Kind = classifyKind(table[i].Pattern)
	}

	routes := compileTable(table)
	probes := make([]RoutingProbe, 0, len(guideProbes))
	for _, spec := range guideProbes {
		probes = append(probes, simulate(routes, spec))
	}

	exhibits := make([]RoutingExhibit, 0, len(exhibitSpecs))
	for _, spec := range exhibitSpecs {
		exhibitRoutes := compileTable(spec.patterns)
		resolved := make([]RoutingProbe, 0, len(spec.probes))
		for _, probeSpec := range spec.probes {
			resolved = append(resolved, simulate(exhibitRoutes, probeSpec))
		}
		exhibits = append(exhibits, RoutingExhibit{
			ID:       spec.id,
			Title:    spec.title,
			Note:     spec.note,
			Patterns: spec.patterns,
			Probes:   resolved,
		})
	}

	return RoutingExplorerPayload{
		Routes:   table,
		Probes:   probes,
		Exhibits: exhibits,
		Semantics: RoutingSemantics{
			GetAlsoServesHead:     true,
			MethodlessMatchesAll:  true,
			SubtreeRedirectStatus: 307,
			ReverseSlashRedirect:  false,
			MatchesEscapedPath:    true,
			PathValueIsUnescaped:  true,
		},
	}
}
