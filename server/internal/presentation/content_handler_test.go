package presentation

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"guide/internal/content"
	"guide/internal/core"
)

func doRequest(server http.Handler, method, path, body string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	var req *http.Request
	if body == "" {
		req = httptest.NewRequest(method, path, nil)
	} else {
		req = httptest.NewRequest(method, path, strings.NewReader(body))
	}
	server.ServeHTTP(rec, req)
	return rec
}

// fixtureBlocks builds one text + one bash code block with id prefixes —
// block ids must stay unique across a whole topic, so every section gets
// its own prefix.
func fixtureBlocks(t *testing.T, prefix string) []content.Block {
	t.Helper()
	text, err := content.NewTextBlock(prefix+"-t1", "Prose introduces the block.")
	if err != nil {
		t.Fatalf("fixture text: %v", err)
	}
	code, err := content.NewCodeBlock(prefix+"-c1", "bash", "", "", "curl localhost:8100/api/health")
	if err != nil {
		t.Fatalf("fixture code: %v", err)
	}
	return []content.Block{text, code}
}

func mustContentRegistry(t *testing.T) content.Registry {
	t.Helper()
	richBlocks := append(fixtureBlocks(t, "w"),
		mustDemoBlock(t, "runtime:async"),
		mustListBlock(t, "w-l1"),
	)
	rich, err := content.NewSection("work", "How it works", "One framing sentence.", richBlocks)
	if err != nil {
		t.Fatalf("rich section: %v", err)
	}
	wrap, err := content.NewSection("wrap", "Wrap-up", "", fixtureBlocks(t, "p"))
	if err != nil {
		t.Fatalf("wrap section: %v", err)
	}

	first, err := content.NewTopic("go-language", "Go language", "The exemplar topic", "foundations", 1, []content.Section{rich, wrap})
	if err != nil {
		t.Fatalf("first topic: %v", err)
	}
	second, err := content.NewTopic("go-runtime", "Go runtime", "", "foundations", 2, []content.Section{wrap})
	if err != nil {
		t.Fatalf("second topic: %v", err)
	}
	group, err := content.NewTopicGroup("foundations", "Foundations", 1)
	if err != nil {
		t.Fatalf("group: %v", err)
	}
	registry, err := content.NewRegistry([]content.Topic{second, first}, []content.TopicGroup{group})
	if err != nil {
		t.Fatalf("registry: %v", err)
	}
	return registry
}

func mustDemoBlock(t *testing.T, id string) content.DemoBlock {
	t.Helper()
	block, err := content.NewDemoBlock(id, "Async explorer", "Watch scheduling live.")
	if err != nil {
		t.Fatalf("demo block: %v", err)
	}
	return block
}

func mustListBlock(t *testing.T, id string) content.ListBlock {
	t.Helper()
	block, err := content.NewListBlock(id, false, []string{
		"Buffered channels — block when full.",
		"Select — wait on many operations.",
		"Context — carry cancellation.",
	})
	if err != nil {
		t.Fatalf("list block: %v", err)
	}
	return block
}

func TestContentListServesEmptyRegistryShape(t *testing.T) {
	var buf strings.Builder
	registry, err := content.NewRegistry(nil, nil)
	if err != nil {
		t.Fatalf("empty registry: %v", err)
	}
	server := newAPIServer(registry, nil, newTestLogger(&buf))

	rec := doRequest(server, http.MethodGet, "/api/content", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	got := strings.TrimSuffix(rec.Body.String(), "\n")
	want := `{"version":1,"groups":[],"topics":[]}`
	if got != want {
		t.Errorf("empty index =\n got %s\nwant %s", got, want)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("content type = %q", ct)
	}
	if rec.Header().Get(core.RequestIDHeader) == "" {
		t.Error("every response must carry x-request-id")
	}
}

func TestContentListServesProductionGroups(t *testing.T) {
	var buf strings.Builder
	registry, err := content.NewProductionRegistry()
	if err != nil {
		t.Fatalf("production registry: %v", err)
	}
	server := newAPIServer(registry, nil, newTestLogger(&buf))

	rec := doRequest(server, http.MethodGet, "/api/content", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	decoded := struct {
		Version int `json:"version"`
		Groups  []struct {
			ID    string `json:"id"`
			Title string `json:"title"`
			Order int    `json:"order"`
		} `json:"groups"`
		Topics []json.RawMessage `json:"topics"`
	}{}
	if err := json.Unmarshal(rec.Body.Bytes(), &decoded); err != nil {
		t.Fatalf("body not the client DTO shape: %v\n%s", err, rec.Body.String())
	}
	if decoded.Version != 1 || len(decoded.Groups) != 10 || decoded.Groups[0].ID != "foundations" || decoded.Groups[9].ID != "capstones" {
		t.Errorf("production groups drifted: %+v", decoded.Groups)
	}
	if len(decoded.Topics) != 23 {
		t.Errorf(
			"content index serves %d topics, want 23 (go-language, go-runtime, http-web, go-http-fundamentals, interfaces-and-composition, goroutines-and-channels, routing, middleware, request-response, validation, api-design, api-documentation, graphql, webhooks, grpc, sql-schema, databases-sql, sqlc-gorm, mongodb, redis, caching, messaging, authentication)",
			len(decoded.Topics),
		)
	}
}

func TestContentListServesSortedSummariesWithSectionCounts(t *testing.T) {
	var buf strings.Builder
	server := newAPIServer(mustContentRegistry(t), nil, newTestLogger(&buf))

	rec := doRequest(server, http.MethodGet, "/api/content", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	body := rec.Body.String()
	if pos := strings.Index(body, `"id":"go-language"`); pos == -1 || strings.LastIndex(body, `"id":"go-runtime"`) < pos {
		t.Errorf("topics not served in order order: %s", body)
	}
	if !strings.Contains(body, `"sectionCount":2`) || !strings.Contains(body, `"sectionCount":1`) {
		t.Errorf("sectionCount missing from summaries: %s", body)
	}
	// A topic without a subtitle omits the key entirely (the client's
	// optionalString reads absence as undefined).
	if strings.Contains(body, `"subtitle":""`) || strings.Contains(body, `"subtitle":null`) {
		t.Errorf("empty subtitle serialized: %s", body)
	}
}

func TestContentTopicDetailWireGolden(t *testing.T) {
	var buf strings.Builder
	server := newAPIServer(mustContentRegistry(t), nil, newTestLogger(&buf))

	rec := doRequest(server, http.MethodGet, "/api/content/go-language", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	got := strings.TrimSuffix(rec.Body.String(), "\n")
	want := `{"version":1,"id":"go-language","title":"Go language","subtitle":"The exemplar topic",` +
		`"group":"foundations","order":1,"sections":[` +
		`{"id":"work","title":"How it works","lead":"One framing sentence.","blocks":[` +
		`{"kind":"text","id":"w-t1","text":"Prose introduces the block."},` +
		`{"kind":"code","id":"w-c1","language":"bash","snippet":"curl localhost:8100/api/health"},` +
		`{"kind":"demo","id":"runtime:async","title":"Async explorer","description":"Watch scheduling live."},` +
		`{"kind":"list","id":"w-l1","ordered":false,"items":[` +
		`"Buffered channels — block when full.",` +
		`"Select — wait on many operations.",` +
		`"Context — carry cancellation."]}]},` +
		`{"id":"wrap","title":"Wrap-up","blocks":[` +
		`{"kind":"text","id":"p-t1","text":"Prose introduces the block."},` +
		`{"kind":"code","id":"p-c1","language":"bash","snippet":"curl localhost:8100/api/health"}]}]}`
	if got != want {
		t.Errorf("topic detail =\n got %s\nwant %s", got, want)
	}
}

func TestContentTopicUnknownIDIs404Envelope(t *testing.T) {
	var buf strings.Builder
	server := newAPIServer(mustContentRegistry(t), nil, newTestLogger(&buf))

	rec := doRequest(server, http.MethodGet, "/api/content/nope", "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
	decoded := struct {
		Error struct {
			Code      string `json:"code"`
			Message   string `json:"message"`
			RequestID string `json:"requestId"`
		} `json:"error"`
	}{}
	if err := json.Unmarshal(rec.Body.Bytes(), &decoded); err != nil {
		t.Fatalf("not an envelope: %s", rec.Body.String())
	}
	if decoded.Error.Code != "not_found" || decoded.Error.Message != `Topic "nope" not found` {
		t.Errorf("envelope = %+v", decoded.Error)
	}
	if decoded.Error.RequestID == "" || rec.Header().Get(core.RequestIDHeader) != decoded.Error.RequestID {
		t.Errorf("request id %q must match the response header %q", decoded.Error.RequestID, rec.Header().Get(core.RequestIDHeader))
	}
}

func TestContentTopicEscapedIDIs404(t *testing.T) {
	// The client's repository spec stubs this exact URL; the server must
	// treat the decoded id as one unknown topic id, not a routing error.
	var buf strings.Builder
	server := newAPIServer(mustContentRegistry(t), nil, newTestLogger(&buf))
	rec := doRequest(server, http.MethodGet, "/api/content/weird%20id%2Fslash", "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"code":"not_found"`) {
		t.Errorf("escaped-id miss must be the envelope: %s", rec.Body.String())
	}
}

func TestRoutingFailuresRideTheEnvelope(t *testing.T) {
	var buf strings.Builder
	server := newAPIServer(mustContentRegistry(t), nil, newTestLogger(&buf))

	rec := doRequest(server, http.MethodPost, "/api/content", "")
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST /api/content status = %d, want 405", rec.Code)
	}
	if body := rec.Body.String(); !strings.Contains(body, `"code":"not_found"`) {
		t.Errorf("405 body is not the envelope: %s", body)
	}
	if allow := rec.Header().Get("Allow"); !strings.Contains(allow, "GET") {
		t.Errorf("Allow header lost: %q", allow)
	}

	rec = doRequest(server, http.MethodGet, "/api/ghost", "")
	if rec.Code != http.StatusNotFound || !strings.Contains(rec.Body.String(), `"code":"not_found"`) {
		t.Errorf("unknown path = %d %s, want the 404 envelope", rec.Code, rec.Body.String())
	}

	// The happy path still works through the same chain.
	rec = doRequest(server, http.MethodGet, "/api/health", "")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"status":"ok"`) {
		t.Errorf("health through the chain = %d %s", rec.Code, rec.Body.String())
	}
}
