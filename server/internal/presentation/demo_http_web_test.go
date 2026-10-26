package presentation

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"guide/internal/core"
)

// The http-web topic's demo endpoint, exercised through the same
// middleware chain production runs behind (a local test mux carrying this
// route — main.go's wiring is the composition root's business). The family
// rule: every new demo endpoint ships with a Go test.

// newHTTPWebTestServer builds the production-shaped chain over a mux that
// carries ONLY this topic's demo route.
func newHTTPWebTestServer() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("GET /api/demo/http/headers", NewHTTPHeadersHandler())
	var buf strings.Builder
	return Chain(
		RequestID,
		RequestLogger(newTestLogger(&buf)),
		Recover(newTestLogger(&buf)),
		EnvelopeMuxErrors(newTestLogger(&buf)),
	)(mux)
}

// doHeaderRequest runs one request through the server, attaching headers
// the demo echo is supposed to see and classify.
func doHeaderRequest(server http.Handler, method, path string, headers map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, nil)
	for name, value := range headers {
		req.Header.Set(name, value)
	}
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	return rec
}

func TestHTTPHeadersDemoEchoesAndClassifies(t *testing.T) {
	server := newHTTPWebTestServer()

	rec := doHeaderRequest(server, http.MethodGet, "/api/demo/http/headers?verbose=1", map[string]string{
		"Accept":             "application/json",
		"X-Custom":           "teaching-value",
		"Authorization":      "Bearer very-secret",
		core.RequestIDHeader: "client-42",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("content type = %q, want application/json", ct)
	}
	if got := rec.Header().Get(core.RequestIDHeader); got != "client-42" {
		t.Errorf("x-request-id = %q, want the honored inbound client-42", got)
	}

	decoded := struct {
		Method      string `json:"method"`
		Path        string `json:"path"`
		Query       string `json:"query"`
		Proto       string `json:"proto"`
		RequestID   string `json:"requestID"`
		ContentType string `json:"contentType"`
		Headers     []struct {
			Name  string `json:"name"`
			Value string `json:"value"`
			Scope string `json:"scope"`
			Role  string `json:"role"`
			Note  string `json:"note"`
		} `json:"headers"`
		HopByHop []string `json:"hopByHop"`
	}{}
	if err := json.Unmarshal(rec.Body.Bytes(), &decoded); err != nil {
		t.Fatalf("body not the demo DTO shape: %v\n%s", err, rec.Body.String())
	}
	if decoded.Method != http.MethodGet || decoded.Path != "/api/demo/http/headers" ||
		decoded.Query != "verbose=1" || decoded.Proto != "HTTP/1.1" {
		t.Errorf("echoed facts drifted: %s %s %q %s", decoded.Method, decoded.Path, decoded.Query, decoded.Proto)
	}
	if decoded.RequestID != "client-42" {
		t.Errorf("payload requestID = %q, want the honored client-42", decoded.RequestID)
	}
	// A body-less GET carries no Content-Type; the echo must OMIT the fact,
	// not guess a default.
	if decoded.ContentType != "" || strings.Contains(rec.Body.String(), `"contentType"`) {
		t.Errorf("contentType = %q, want the absent fact omitted", decoded.ContentType)
	}

	byName := map[string]struct{ value, scope, role string }{}
	for _, h := range decoded.Headers {
		byName[h.Name] = struct{ value, scope, role string }{h.Value, h.Scope, h.Role}
	}
	if accept, ok := byName["Accept"]; !ok || accept.role != "negotiation" {
		t.Errorf("Accept classified %+v, want negotiation", accept)
	}
	if custom, ok := byName["X-Custom"]; !ok || custom.role != "extension" {
		t.Errorf("X-Custom classified %+v, want extension", custom)
	}
	if auth, ok := byName["Authorization"]; !ok || auth.role != "security" || auth.value != "[value withheld]" {
		t.Errorf("Authorization classified %+v, want security with withheld value", auth)
	}
	if len(decoded.HopByHop) != 8 {
		t.Errorf("hop-by-hop reference = %v, want RFC 9110 §7.6.1 + keep-alive", decoded.HopByHop)
	}
}

func TestHTTPHeadersDemoRejectsNonGet(t *testing.T) {
	server := newHTTPWebTestServer()

	rec := doHeaderRequest(server, http.MethodPost, "/api/demo/http/headers", nil)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"code":"not_found"`) {
		t.Errorf("405 body is not the envelope: %s", rec.Body.String())
	}
	if allow := rec.Header().Get("Allow"); !strings.Contains(allow, "GET") {
		t.Errorf("Allow header lost: %q", allow)
	}
}
