package presentation

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"guide/internal/core"
)

// The request-response topic's two demo endpoints, exercised through the
// same middleware chain production runs behind (the family rule that
// every new demo endpoint ships with a Go test). Routes live in this
// test's own mux — the main.go wiring lands with the wave's shared
// wiring, not here.

// newReqresDemoServer builds the demo routes behind the production chain.
func newReqresDemoServer() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("GET /api/demo/reqres/inspector", NewReqresInspectorHandler())
	mux.Handle("GET /api/demo/reqres/methods", NewReqresMethodsHandler())
	var buf strings.Builder
	logger := newTestLogger(&buf)
	return Chain(
		RequestID,
		RequestLogger(logger),
		Recover(logger),
		EnvelopeMuxErrors(logger),
	)(mux)
}

// doReqresRequest sends one request with an optional Content-Type and
// body (the inspector's whole point is a GET that carries a body).
func doReqresRequest(server http.Handler, method, path, contentType, body string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	var req *http.Request
	if body == "" {
		req = httptest.NewRequest(method, path, nil)
	} else {
		req = httptest.NewRequest(method, path, strings.NewReader(body))
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	server.ServeHTTP(rec, req)
	return rec
}

func TestReqresInspectorDemoServesThePayload(t *testing.T) {
	server := newReqresDemoServer()

	rec := doReqresRequest(server, http.MethodGet, "/api/demo/reqres/inspector", "", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("content type = %q, want application/json", ct)
	}
	if rec.Header().Get(core.RequestIDHeader) == "" {
		t.Error("every response must carry x-request-id")
	}

	// Spot-check the beats the topic's prose promises; the builder's own
	// tests pin the deep shape and the verdicts.
	decoded := struct {
		Request struct {
			Method    string `json:"method"`
			Path      string `json:"path"`
			BodyBytes int    `json:"bodyBytes"`
			BodyLimit int    `json:"bodyLimit"`
		} `json:"request"`
		Discipline struct {
			UnknownFields string `json:"unknownFields"`
			Trailing      string `json:"trailing"`
		} `json:"discipline"`
		Exhibits []struct {
			Label   string `json:"label"`
			Outcome string `json:"outcome"`
		} `json:"exhibits"`
		LiveBody struct {
			Sent    bool   `json:"sent"`
			Outcome string `json:"outcome"`
		} `json:"liveBody"`
	}{}
	if err := json.Unmarshal(rec.Body.Bytes(), &decoded); err != nil {
		t.Fatalf("body not the demo DTO shape: %v\n%s", err, rec.Body.String())
	}
	if decoded.Request.Method != "GET" || decoded.Request.Path != "/api/demo/reqres/inspector" {
		t.Errorf("request facts drifted: %+v", decoded.Request)
	}
	if decoded.Request.BodyBytes != 0 || decoded.Request.BodyLimit != 4096 {
		t.Errorf("body facts drifted: %+v", decoded.Request)
	}
	if decoded.Discipline.UnknownFields == "" || decoded.Discipline.Trailing == "" {
		t.Errorf("discipline facts drifted: %+v", decoded.Discipline)
	}
	if len(decoded.Exhibits) != 4 || decoded.Exhibits[0].Outcome != "accepted" ||
		decoded.Exhibits[1].Outcome != "rejected" {
		t.Errorf("exhibits drifted: %+v", decoded.Exhibits)
	}
	if decoded.LiveBody.Sent || decoded.LiveBody.Outcome != "none" {
		t.Errorf("body-less GET must report liveBody outcome none: %+v", decoded.LiveBody)
	}
}

func TestReqresInspectorDemoDecodesTheBodyThisGetCarried(t *testing.T) {
	server := newReqresDemoServer()

	// A GET may legally carry a body; the inspector runs the same decoder
	// discipline over it as over the exhibits.
	rec := doReqresRequest(server, http.MethodGet, "/api/demo/reqres/inspector",
		"application/json", `{"title":"hello from the test","tags":["live"]}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	decoded := struct {
		Request struct {
			ContentType string `json:"contentType"`
			BodyBytes   int    `json:"bodyBytes"`
		} `json:"request"`
		LiveBody struct {
			Sent    bool   `json:"sent"`
			Outcome string `json:"outcome"`
			Decoded string `json:"decoded"`
		} `json:"liveBody"`
	}{}
	if err := json.Unmarshal(rec.Body.Bytes(), &decoded); err != nil {
		t.Fatalf("body not the demo DTO shape: %v\n%s", err, rec.Body.String())
	}
	if decoded.Request.ContentType != "application/json" || decoded.Request.BodyBytes == 0 {
		t.Errorf("request facts drifted: %+v", decoded.Request)
	}
	if !decoded.LiveBody.Sent || decoded.LiveBody.Outcome != "accepted" ||
		decoded.LiveBody.Decoded != `title="hello from the test" tags=["live"]` {
		t.Errorf("live body beat drifted: %+v", decoded.LiveBody)
	}
}

func TestReqresInspectorDemoEnforcesTheBodyCap(t *testing.T) {
	server := newReqresDemoServer()

	// One byte past the cap: the payload reports exactly the cap's worth
	// of bytes with the exceeded flag set — and the truncated body's
	// honest verdict (rejected, unexpected EOF).
	oversized := `{"title":"` + strings.Repeat("x", 5000) + `"}`
	rec := doReqresRequest(server, http.MethodGet, "/api/demo/reqres/inspector", "", oversized)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (reported, not rejected)", rec.Code)
	}

	decoded := struct {
		Request struct {
			BodyBytes         int  `json:"bodyBytes"`
			BodyLimit         int  `json:"bodyLimit"`
			BodyLimitExceeded bool `json:"bodyLimitExceeded"`
		} `json:"request"`
		LiveBody struct {
			Outcome       string `json:"outcome"`
			UnexpectedEOF bool   `json:"unexpectedEOF"`
		} `json:"liveBody"`
	}{}
	if err := json.Unmarshal(rec.Body.Bytes(), &decoded); err != nil {
		t.Fatalf("body not the demo DTO shape: %v\n%s", err, rec.Body.String())
	}
	if decoded.Request.BodyBytes != decoded.Request.BodyLimit || !decoded.Request.BodyLimitExceeded {
		t.Errorf("cap facts drifted: %+v", decoded.Request)
	}
	if decoded.LiveBody.Outcome != "rejected" || !decoded.LiveBody.UnexpectedEOF {
		t.Errorf("truncated live body must be rejected as unexpected EOF: %+v", decoded.LiveBody)
	}
}

func TestReqresMethodsDemoServesThePayload(t *testing.T) {
	server := newReqresDemoServer()

	rec := doReqresRequest(server, http.MethodGet, "/api/demo/reqres/methods", "", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if rec.Header().Get(core.RequestIDHeader) == "" {
		t.Error("every response must carry x-request-id")
	}

	decoded := struct {
		Resource string `json:"resource"`
		Allow    string `json:"allow"`
		Methods  []struct {
			Method     string `json:"method"`
			Status     int    `json:"status"`
			StatusText string `json:"statusText"`
			Body       string `json:"body"`
		} `json:"methods"`
		SSE struct {
			ContentType string `json:"contentType"`
			Events      []struct {
				Name string `json:"name"`
				Data string `json:"data"`
			} `json:"events"`
			Flushes int `json:"flushes"`
		} `json:"sse"`
	}{}
	if err := json.Unmarshal(rec.Body.Bytes(), &decoded); err != nil {
		t.Fatalf("body not the demo DTO shape: %v\n%s", err, rec.Body.String())
	}
	if len(decoded.Methods) != 8 {
		t.Fatalf("method rows = %d, want 8", len(decoded.Methods))
	}
	wantStatus := map[string]int{
		"GET": 200, "POST": 201, "PUT": 200, "PATCH": 204,
		"DELETE": 202, "HEAD": 200, "OPTIONS": 204, "TRACE": 405,
	}
	for _, row := range decoded.Methods {
		if row.Status != wantStatus[row.Method] {
			t.Errorf("%s status = %d, want %d", row.Method, row.Status, wantStatus[row.Method])
		}
		if row.StatusText == "" {
			t.Errorf("%s row lacks its status phrase", row.Method)
		}
	}
	if decoded.Allow != "GET, HEAD, OPTIONS, POST, PUT, PATCH, DELETE" {
		t.Errorf("allow = %q", decoded.Allow)
	}
	if len(decoded.SSE.Events) != 3 || decoded.SSE.Flushes != 3 || decoded.SSE.ContentType != "text/event-stream" {
		t.Errorf("SSE beat drifted: %+v", decoded.SSE)
	}
	if decoded.SSE.Events[2].Data != `{"sequence":3,"state":"tick"}` {
		t.Errorf("last SSE event = %+v", decoded.SSE.Events[2])
	}
}

func TestReqresDemoEndpointsRejectNonGet(t *testing.T) {
	server := newReqresDemoServer()

	for _, path := range []string{"/api/demo/reqres/inspector", "/api/demo/reqres/methods"} {
		rec := doReqresRequest(server, http.MethodPost, path, "", "")
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("POST %s status = %d, want 405", path, rec.Code)
		}
		if !strings.Contains(rec.Body.String(), `"code":"not_found"`) {
			t.Errorf("POST %s body is not the envelope: %s", path, rec.Body.String())
		}
		if allow := rec.Header().Get("Allow"); !strings.Contains(allow, "GET") {
			t.Errorf("POST %s lost the Allow header: %q", path, allow)
		}
	}
}
