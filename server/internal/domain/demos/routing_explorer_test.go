package demos

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sort"
	"strings"
	"testing"
)

// Routing-explorer builder tests. The matcher's authority is the empirical
// agreement test: every probe the payload ships — plus a battery of extra
// agreement cases the carried table cannot produce — must resolve to the
// SAME outcome, status, matched pattern, Allow set, redirect target, and
// captured path values as a real net/http.ServeMux compiled with the same
// patterns. A routing claim in the topic's prose cites these pins.

func TestRoutingExplorerPayloadIsDeterministic(t *testing.T) {
	first := BuildRoutingExplorerPayload()
	second := BuildRoutingExplorerPayload()
	if !reflect.DeepEqual(first, second) {
		t.Fatal("the pure builder must produce identical payloads on every call")
	}
}

func TestRoutingExplorerRouteTableCarriesTheWiredPatterns(t *testing.T) {
	payload := BuildRoutingExplorerPayload()
	if len(payload.Routes) != 12 {
		t.Fatalf("carried table has %d routes, want the 12 wired at authoring time", len(payload.Routes))
	}
	wildcards := 0
	for _, route := range payload.Routes {
		if route.Method == "" || route.Pattern == "" || route.Handler == "" {
			t.Errorf("incomplete real route entry: %+v", route)
		}
		if route.Kind == routeKindSingle {
			wildcards++
			if route.Pattern != "/api/content/{topicId}" || route.Handler != "presentation.NewContentTopicHandler" {
				t.Errorf("wildcard entry drifted: %+v", route)
			}
		}
	}
	if wildcards != 1 {
		t.Errorf("carried table has %d wildcard patterns, want exactly the topicId one", wildcards)
	}
	if payload.Routes[0].Pattern != "/api/health" || payload.Routes[0].Kind != routeKindExact {
		t.Errorf("first route drifted: %+v", payload.Routes[0])
	}
}

func TestRoutingExplorerProbeOutcomesCoverEveryVerdict(t *testing.T) {
	payload := BuildRoutingExplorerPayload()
	want := []struct {
		path    string
		outcome string
		status  int
		pattern string // matchedPattern when non-empty
	}{
		{"/api/content", outcomeMatched, 200, "GET /api/content"},
		{"/api/content/routing", outcomeMatched, 200, "GET /api/content/{topicId}"},
		{"/api/content/routing", outcomeMatched, 200, "GET /api/content/{topicId}"}, // HEAD via GET
		{"/api/content/routing?verbose=1&tag=go&tag=routing", outcomeMatched, 200, "GET /api/content/{topicId}"},
		{"/api/content/routing", outcomeMethod, 405, ""},
		{"/api/demo/scenario", outcomeMethod, 405, ""},
		{"/api/content/a/b", outcomeNotFound, 404, ""},
		{"/api/content/", outcomeNotFound, 404, ""},
		{"/api/content/abc%2Fdef", outcomeMatched, 200, "GET /api/content/{topicId}"},
	}
	if len(payload.Probes) != len(want) {
		t.Fatalf("probe count = %d, want %d", len(payload.Probes), len(want))
	}
	for i, probe := range payload.Probes {
		w := want[i]
		if probe.Path != w.path || probe.Outcome != w.outcome || probe.Status != w.status {
			t.Errorf("probe %d (%s %s) = %s/%d, want %s/%d", i, probe.Method, probe.Path, probe.Outcome, probe.Status, w.outcome, w.status)
		}
		if probe.MatchedPattern != w.pattern {
			t.Errorf("probe %d matched %q, want %q", i, probe.MatchedPattern, w.pattern)
		}
		if probe.Reasoning == "" {
			t.Errorf("probe %d has no reasoning sentence", i)
		}
	}
}

func TestRoutingExplorerAllowSetsIncludeImpliedHead(t *testing.T) {
	payload := BuildRoutingExplorerPayload()
	if got := payload.Probes[4].Allow; !reflect.DeepEqual(got, []string{"GET", "HEAD"}) {
		t.Errorf("Allow for a GET-only path = %v, want [GET HEAD]", got)
	}
	if got := payload.Probes[5].Allow; !reflect.DeepEqual(got, []string{"GET", "HEAD", "PATCH"}) {
		t.Errorf("Allow across two patterns = %v, want [GET HEAD PATCH]", got)
	}
}

func TestRoutingExplorerEscapedPathCaptureUnescapes(t *testing.T) {
	payload := BuildRoutingExplorerPayload()
	probe := payload.Probes[8]
	if len(probe.PathValues) != 1 {
		t.Fatalf("escaped-path probe captured %d values, want 1: %+v", len(probe.PathValues), probe.PathValues)
	}
	value := probe.PathValues[0]
	if value.Name != "topicId" || value.Source != "path" || !reflect.DeepEqual(value.Values, []string{"abc/def"}) {
		t.Errorf("capture drifted: %+v — want topicId=[abc/def] from the path", value)
	}
}

func TestRoutingExplorerQueryValuesKeepEveryOccurrence(t *testing.T) {
	payload := BuildRoutingExplorerPayload()
	probe := payload.Probes[3]
	if len(probe.QueryValues) != 2 {
		t.Fatalf("query probe carries %d bound query values, want 2: %+v", len(probe.QueryValues), probe)
	}
	verbose, tag := probe.QueryValues[0], probe.QueryValues[1]
	if verbose.Name != "verbose" || !reflect.DeepEqual(verbose.Values, []string{"1"}) || verbose.Source != "query" {
		t.Errorf("verbose drifted: %+v", verbose)
	}
	if tag.Name != "tag" || !reflect.DeepEqual(tag.Values, []string{"go", "routing"}) {
		t.Errorf("tag must keep BOTH occurrences in arrival order: %+v", tag)
	}
}

func TestRoutingExplorerExhibitsResolveAgainstTheirOwnPatterns(t *testing.T) {
	payload := BuildRoutingExplorerPayload()
	if len(payload.Exhibits) != 2 {
		t.Fatalf("exhibit count = %d, want 2", len(payload.Exhibits))
	}
	precedence := payload.Exhibits[0]
	if precedence.ID != "precedence" || len(precedence.Probes) != 1 {
		t.Fatalf("precedence exhibit drifted: %+v", precedence)
	}
	probe := precedence.Probes[0]
	if probe.Outcome != outcomeMatched || probe.MatchedPattern != "GET /api/content/new" {
		t.Errorf("precedence verdict = %s on %q, want the LITERAL to win", probe.Outcome, probe.MatchedPattern)
	}

	subtree := payload.Exhibits[1]
	if subtree.ID != "subtree" || len(subtree.Probes) != 2 {
		t.Fatalf("subtree exhibit drifted: %+v", subtree)
	}
	root, deep := subtree.Probes[0], subtree.Probes[1]
	if root.Outcome != outcomeRedirect || root.Status != 307 || root.RedirectTarget != "/assets/" {
		t.Errorf("subtree root probe = %s/%d → %q, want redirect/307 → /assets/", root.Outcome, root.Status, root.RedirectTarget)
	}
	if deep.Outcome != outcomeMatched || deep.MatchedPattern != "GET /assets/" || len(deep.PathValues) != 0 {
		t.Errorf("deep subtree probe drifted: %+v", deep)
	}
}

func TestRoutingExplorerSemanticsFacts(t *testing.T) {
	s := BuildRoutingExplorerPayload().Semantics
	if !s.GetAlsoServesHead || !s.MethodlessMatchesAll || s.SubtreeRedirectStatus != 307 {
		t.Errorf("semantics drifted: %+v", s)
	}
	if s.ReverseSlashRedirect || !s.MatchesEscapedPath || !s.PathValueIsUnescaped {
		t.Errorf("semantics drifted: %+v", s)
	}
}

// ── the empirical agreement test ──────────────────────────────────────────

// captureReport renders what a real handler saw: the matched pattern's
// wildcard names in declaration order with their PathValue values —
// including a multi wildcard that captured the empty remainder.
func captureReport(r *http.Request) string {
	if r.Pattern == "" {
		return ""
	}
	compiled, err := compileRoute(r.Pattern)
	if err != nil {
		return ""
	}
	var parts []string
	for _, seg := range compiled.segments {
		if seg.name != "" {
			parts = append(parts, seg.name+"="+r.PathValue(seg.name))
		}
	}
	return strings.Join(parts, ";")
}

// statusFor maps the matcher's outcome vocabulary onto the status the real
// mux must produce for the same request.
func statusFor(outcome string) int {
	switch outcome {
	case outcomeMatched:
		return 200
	case outcomeMethod:
		return 405
	case outcomeRedirect:
		return 307
	default:
		return 404
	}
}

// agree asserts the matcher's verdict equals the real mux's for one probe:
// status, matched pattern, captured values, Allow set, and redirect target.
func agree(t *testing.T, routes []compiledRoute, mux *http.ServeMux, method, path string) {
	t.Helper()
	probe := simulate(routes, probeSpec{method: method, path: path})

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(method, path, nil))

	if rec.Code != statusFor(probe.Outcome) {
		t.Errorf("%s %s: matcher says %s (%d), real mux says %d", method, path, probe.Outcome, statusFor(probe.Outcome), rec.Code)
	}
	switch probe.Outcome {
	case outcomeMatched:
		if got := strings.TrimSpace(rec.Body.String()); got != "" {
			// Handlers report "matched-pattern|captures" for exact comparison.
			reported, captures, _ := strings.Cut(got, "|")
			if reported != probe.MatchedPattern {
				t.Errorf("%s %s: real mux matched %q, matcher says %q", method, path, reported, probe.MatchedPattern)
			}
			var parts []string
			for _, value := range probe.PathValues {
				parts = append(parts, value.Name+"="+strings.Join(value.Values, ","))
			}
			if strings.Join(parts, ";") != captures {
				t.Errorf("%s %s: captures matcher=%q real=%q", method, path, strings.Join(parts, ";"), captures)
			}
		}
	case outcomeMethod:
		var allow []string
		if header := rec.Header().Get("Allow"); header != "" {
			allow = strings.Split(header, ", ")
			sort.Strings(allow)
		}
		if !reflect.DeepEqual(probe.Allow, allow) {
			t.Errorf("%s %s: Allow matcher=%v real=%v", method, path, probe.Allow, allow)
		}
	case outcomeRedirect:
		if target := rec.Header().Get("Location"); target != probe.RedirectTarget {
			t.Errorf("%s %s: redirect target matcher=%q real=%q", method, path, probe.RedirectTarget, target)
		}
	}
}

func TestRoutingExplorerAgreesWithTheRealServeMux(t *testing.T) {
	// A real mux carrying the guide's table plus the exhibit patterns and
	// the synthetic ones the extra agreement cases need. Each handler
	// reports the matched pattern and its captures, so precedence and
	// wildcard verdicts are compared on WHAT matched, not just the status.
	mux := http.NewServeMux()
	report := func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "%s|%s", r.Pattern, captureReport(r))
	}
	for _, pattern := range append(
		patternStringsOf(BuildRoutingExplorerPayload().Routes),
		"GET /api/content/new", "GET /assets/", "POST /assets/",
		"GET /files/{path...}", "GET /b/{bucket}/o/{object}",
	) {
		mux.HandleFunc(pattern, report)
	}

	routes := append(
		compileTable(BuildRoutingExplorerPayload().Routes),
		mustCompileRoute("GET /api/content/new"),
		mustCompileRoute("GET /assets/"),
		mustCompileRoute("POST /assets/"),
		mustCompileRoute("GET /files/{path...}"),
		mustCompileRoute("GET /b/{bucket}/o/{object}"),
	)

	// Every shipped probe — main table and both exhibits — agreed again
	// through the real mux.
	for _, spec := range guideProbes {
		agree(t, routes, mux, spec.method, spec.path)
	}
	for _, exhibit := range exhibitSpecs {
		for _, spec := range exhibit.probes {
			agree(t, routes, mux, spec.method, spec.path)
		}
	}

	// Extra agreement cases the carried table cannot produce.
	for _, pair := range [][2]string{
		{"POST", "/assets"},       // the subtree redirect is method-aware
		{"HEAD", "/assets"},       // GET serves HEAD, redirect included
		{"GET", "/assets?x=1"},    // the query rides along on the redirect
		{"GET", "//api/content"},  // sanitization redirects before matching
		{"GET", "/x/./y"},         // dot segments clean up
		{"GET", "/files"},         // {...} root without its slash → 307
		{"GET", "/files/"},        // {...} captures the empty remainder
		{"GET", "/files/a/b/c"},   // {...} captures across slashes
		{"GET", "/b/x/o/y"},       // two single wildcards capture both
		{"DELETE", "/assets"},     // method gates the redirect: no lying 307
		{"PATCH", "/api/content"}, // 405 on the literal, Allow GET+HEAD
		{"GET", "/api/content/go-language"},
	} {
		agree(t, routes, mux, pair[0], pair[1])
	}
}

// patternStringsOf renders the carried table back into registration lines.
func patternStringsOf(table []RoutingPattern) []string {
	patterns := make([]string, 0, len(table))
	for _, route := range table {
		patterns = append(patterns, route.Method+" "+route.Pattern)
	}
	return patterns
}

func TestUnresolvableOverlapPanicsAtRegistration(t *testing.T) {
	// Documented ServeMux behavior the matcher's carried data relies on: a
	// table that cannot resolve an overlap refuses to boot. "GET /{x}/a"
	// and "GET /b/{y}" both match "/b/a" with neither more specific.
	defer func() {
		recovered := recover()
		if recovered == nil {
			t.Fatal("the second registration must panic")
		}
		if message := fmt.Sprint(recovered); !strings.Contains(message, "conflicts with pattern") {
			t.Errorf("panic message does not name the conflict: %v", message)
		}
	}()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{x}/a", func(http.ResponseWriter, *http.Request) {})
	mux.HandleFunc("GET /b/{y}", func(http.ResponseWriter, *http.Request) {})
}
