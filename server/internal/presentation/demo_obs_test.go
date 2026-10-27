package presentation

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"guide/internal/core"
	postsdata "guide/internal/data/posts"
	"guide/internal/domain/demos"
	"guide/internal/domain/obs"
	"guide/internal/domain/posts"
)

// The two observability demo endpoints, exercised through the same
// middleware chain production runs behind. The viewer replay is wired
// over a fresh in-memory repository (scenario-independent); the trace
// walk is wired EXACTLY as the composition root wires it — spanning
// recorders at the leaf and port seams, the use case over those — with
// a paced clock so durations are deterministic multiples of 1ms.

func newObsDemosServer(t *testing.T) http.Handler {
	t.Helper()
	logger := newTestLogger(&strings.Builder{})

	viewRepo := postsdata.NewInMemoryRepository()
	viewList := posts.NewListPostsUseCase(viewRepo)
	viewGet := posts.NewGetPostUseCase(viewRepo)

	recorder := obs.NewSpanRecorder(obsTestPacer())
	leaf := demos.NewObsSpanningRepository(postsdata.NewInMemoryRepository(), "data", "InMemoryRepository", recorder)
	port := demos.NewObsSpanningRepository(leaf, "domain", "PostsRepository", recorder)
	traceList := posts.NewListPostsUseCase(port)

	mux := http.NewServeMux()
	mux.Handle("GET "+demos.ObsViewerPath, NewObsViewerHandler(viewList, viewGet))
	mux.Handle("GET "+demos.ObsTracePath, NewObsTraceHandler(traceList, recorder, logger))
	return Chain(
		RequestID,
		RequestLogger(logger),
		Recover(logger),
		EnvelopeMuxErrors(logger),
	)(mux)
}

// obsTestPacer advances one millisecond per clock read.
func obsTestPacer() func() time.Time {
	step := 0
	return func() time.Time {
		step++
		return time.Unix(0, 0).Add(time.Duration(step) * time.Millisecond)
	}
}

func TestObsViewerReplaysRealLinesAndDissectsThem(t *testing.T) {
	server := newObsDemosServer(t)
	rec := doRequest(server, http.MethodGet, demos.ObsViewerPath, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if rec.Header().Get(core.RequestIDHeader) == "" {
		t.Error("every response must carry x-request-id")
	}

	var payload struct {
		Anatomy struct {
			Line   string `json:"line"`
			Fields []struct {
				Key   string `json:"key"`
				Value string `json:"value"`
				Note  string `json:"note"`
			} `json:"fields"`
		} `json:"anatomy"`
		Levels struct {
			Rows []struct {
				Level      string   `json:"level"`
				SurvivesAt []string `json:"survivesAt"`
			} `json:"rows"`
		} `json:"levels"`
		Replay struct {
			Requests []struct {
				Label  string `json:"label"`
				Status int    `json:"status"`
			} `json:"requests"`
			Lines []struct {
				Source    string `json:"source"`
				Level     string `json:"level"`
				RequestID string `json:"requestId"`
			} `json:"lines"`
			Note string `json:"note"`
		} `json:"replay"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("viewer payload not the demo DTO shape: %v\n%s", err, rec.Body.String())
	}

	// The anatomy line is a REAL http_request line with the full field set.
	if !strings.Contains(payload.Anatomy.Line, `"msg":"http_request"`) {
		t.Errorf("anatomy line must be an access line: %s", payload.Anatomy.Line)
	}
	keys := make([]string, 0)
	for _, field := range payload.Anatomy.Fields {
		keys = append(keys, field.Key)
		if field.Note == "" {
			t.Errorf("field %q must carry its note", field.Key)
		}
	}
	for _, want := range []string{"time", "level", "msg", "method", "path", "status", "duration_ms", "request_id"} {
		if !strings.Contains(strings.Join(keys, ","), want) {
			t.Errorf("anatomy must dissect %q: %v", want, keys)
		}
	}

	// The level matrix is computed: four rows, monotone survival.
	if len(payload.Levels.Rows) != 4 {
		t.Errorf("level rows = %d, want 4", len(payload.Levels.Rows))
	}

	// The replay: a 200 and a 404, at least three captured lines, every
	// line sharing the OUTER request's id.
	statuses := map[int]bool{}
	for _, request := range payload.Replay.Requests {
		statuses[request.Status] = true
	}
	if !statuses[200] || !statuses[404] {
		t.Errorf("replay must walk a 200 and a 404: %+v", payload.Replay.Requests)
	}
	if len(payload.Replay.Lines) < 3 {
		t.Fatalf("replay lines = %d, want >= 3 (two access + one warn)", len(payload.Replay.Lines))
	}
	outerID := rec.Header().Get(core.RequestIDHeader)
	warnSeen := false
	for _, line := range payload.Replay.Lines {
		if line.RequestID != outerID {
			t.Errorf("line from %s carried id %q, want the outer id %q", line.Source, line.RequestID, outerID)
		}
		if line.Level == "WARN" {
			warnSeen = true
		}
	}
	if !warnSeen {
		t.Error("the 404's warn line must be captured")
	}
}

func TestObsTraceAcceptsTheCallersOwnHeaderAndGrowsTheTree(t *testing.T) {
	server := newObsDemosServer(t)

	// The round trip starts with the CALLER's header: the demo request
	// itself carries a traceparent the walk must adopt.
	req, err := http.NewRequest(http.MethodGet, demos.ObsTracePath, nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set(obs.TraceparentHeader, "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01")
	req.Header.Set(obs.TracestateHeader, "go=guide;vendor=demo")

	rec := serveObsRequest(server, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}

	var payload struct {
		Accepted struct {
			Inbound       string `json:"inbound"`
			InboundOrigin string `json:"inboundOrigin"`
			TraceID       string `json:"traceId"`
			ParentID      string `json:"parentId"`
			Tracestate    string `json:"tracestate"`
			ResponseEcho  string `json:"responseEcho"`
		} `json:"accepted"`
		Tree []struct {
			Name       string  `json:"name"`
			Layer      string  `json:"layer"`
			Depth      int     `json:"depth"`
			ParentID   string  `json:"parentId"`
			TraceID    string  `json:"traceId"`
			DurationMs float64 `json:"durationMs"`
		} `json:"tree"`
		Malformed []struct {
			Header  string `json:"header"`
			Problem string `json:"problem"`
		} `json:"malformed"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("trace payload not the demo DTO shape: %v\n%s", err, rec.Body.String())
	}

	if payload.Accepted.TraceID != "4bf92f3577b34da6a3ce929d0e0e4736" {
		t.Errorf("accepted trace id = %q, want the caller's", payload.Accepted.TraceID)
	}
	if !strings.Contains(payload.Accepted.InboundOrigin, "adopted verbatim") {
		t.Errorf("origin must say the caller's header was adopted: %q", payload.Accepted.InboundOrigin)
	}
	if payload.Accepted.Tracestate != "go=guide;vendor=demo" {
		t.Errorf("tracestate round trip drifted: %q", payload.Accepted.Tracestate)
	}
	if !strings.HasPrefix(payload.Accepted.ResponseEcho, "00-4bf92f3577b34da6a3ce929d0e0e4736-") {
		t.Errorf("response echo = %q, want the same trace id", payload.Accepted.ResponseEcho)
	}

	if len(payload.Tree) != 4 {
		t.Fatalf("tree = %d nodes, want 4: %+v", len(payload.Tree), payload.Tree)
	}
	want := []struct{ name, layer string }{
		{"GET /api/posts (traced walk)", "middleware"},
		{"PostsListHandler.ServeHTTP", "presentation"},
		{"PostsRepository.List", "domain"},
		{"InMemoryRepository.List", "data"},
	}
	for i, w := range want {
		if payload.Tree[i].Name != w.name || payload.Tree[i].Layer != w.layer {
			t.Errorf("tree[%d] = %+v, want %q/%q", i, payload.Tree[i], w.name, w.layer)
		}
		if payload.Tree[i].DurationMs <= 0 {
			t.Errorf("paced node %q must have a positive duration", w.name)
		}
	}
	if payload.Tree[0].ParentID != "00f067aa0ba902b7" {
		t.Errorf("root parent = %q, want the caller's span id (the graft point)", payload.Tree[0].ParentID)
	}
	for i, node := range payload.Tree {
		if node.TraceID != "4bf92f3577b34da6a3ce929d0e0e4736" {
			t.Errorf("node %d left the caller's trace", i)
		}
	}

	if len(payload.Malformed) < 5 {
		t.Errorf("malformed table = %d rows, want the hostile set", len(payload.Malformed))
	}
}

func TestObsTraceMintsARootWhenTheCallerSendsNothing(t *testing.T) {
	server := newObsDemosServer(t)
	rec := doRequest(server, http.MethodGet, demos.ObsTracePath, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var payload struct {
		Accepted struct {
			Inbound       string `json:"inbound"`
			InboundOrigin string `json:"inboundOrigin"`
		} `json:"accepted"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if payload.Accepted.Inbound != "" || !strings.Contains(payload.Accepted.InboundOrigin, "minted") {
		t.Errorf("headerless call must report the mint: %+v", payload.Accepted)
	}
}

func TestObsTraceRejectsAMalformedCallerHeaderHonestly(t *testing.T) {
	server := newObsDemosServer(t)
	req, _ := http.NewRequest(http.MethodGet, demos.ObsTracePath, nil)
	req.Header.Set(obs.TraceparentHeader, "00-00000000000000000000000000000000-00f067aa0ba902b7-01")
	rec := serveObsRequest(server, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("malformed header must NOT fail the request; status = %d", rec.Code)
	}
	var payload struct {
		Accepted struct {
			TraceID string `json:"traceId"`
		} `json:"accepted"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if payload.Accepted.TraceID == "00000000000000000000000000000000" {
		t.Error("malformed header must not be adopted")
	}
}

func TestObsDemosRejectNonGet(t *testing.T) {
	server := newObsDemosServer(t)
	for _, path := range []string{demos.ObsViewerPath, demos.ObsTracePath} {
		rec := doRequest(server, http.MethodPost, path, "")
		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("POST %s status = %d, want 405", path, rec.Code)
		}
		if !strings.Contains(rec.Body.String(), `"code":"not_found"`) {
			t.Errorf("POST %s body must be the envelope: %s", path, rec.Body.String())
		}
	}
}

// serveObsRequest fires a prepared request (with its inbound headers)
// at the server and captures the response.
func serveObsRequest(server http.Handler, req *http.Request) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	return rec
}
