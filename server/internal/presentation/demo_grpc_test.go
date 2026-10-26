package presentation

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"testing"

	"guide/internal/core"
)

// The grpc topic's demo endpoint, exercised through the same middleware
// chain production runs behind (the family rule that every new demo
// endpoint ships with a Go test). The route lives in a test mux here;
// main.go's production wiring is the composition root's call.

func newGRPCConsoleServer(logger *slog.Logger) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("GET /api/demo/api/grpc-console", NewGRPCConsoleHandler())
	return Chain(
		RequestID,
		RequestLogger(logger),
		Recover(logger),
		EnvelopeMuxErrors(logger),
	)(mux)
}

func TestGRPCConsoleDemoServesThePayload(t *testing.T) {
	var buf strings.Builder
	server := newGRPCConsoleServer(newTestLogger(&buf))

	rec := doRequest(server, http.MethodGet, "/api/demo/api/grpc-console", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("content type = %q, want application/json", ct)
	}
	if rec.Header().Get(core.RequestIDHeader) == "" {
		t.Error("every response must carry x-request-id")
	}

	// Spot-check the beats the topic's prose promises; the payload
	// builder's own tests pin the deep shape and the determinism.
	decoded := struct {
		Transport struct {
			Mode      string `json:"mode"`
			Service   string `json:"service"`
			ProtoFile string `json:"protoFile"`
		} `json:"transport"`
		Calls []struct {
			Kind         string `json:"kind"`
			Method       string `json:"method"`
			StatusCode   string `json:"statusCode"`
			StatusNumber uint32 `json:"statusNumber"`
			RoundTrips   int    `json:"roundTrips"`
			Results      []struct {
				Label  string `json:"label"`
				Detail string `json:"detail"`
				OK     bool   `json:"ok"`
			} `json:"results"`
		} `json:"calls"`
	}{}
	if err := json.Unmarshal(rec.Body.Bytes(), &decoded); err != nil {
		t.Fatalf("body not the demo DTO shape: %v\n%s", err, rec.Body.String())
	}
	if decoded.Transport.Service != "guide.posts.v1.PostsService" ||
		decoded.Transport.ProtoFile != "server/api/proto/posts.proto" {
		t.Errorf("transport facts drifted: %+v", decoded.Transport)
	}
	if len(decoded.Calls) != 6 {
		t.Fatalf("calls = %d, want 6 exhibit calls", len(decoded.Calls))
	}
	kinds := map[string]bool{}
	statuses := map[string]bool{}
	for _, call := range decoded.Calls {
		kinds[call.Kind] = true
		statuses[call.StatusCode] = true
		if len(call.Results) == 0 {
			t.Errorf("call %s %q has no result rows", call.Kind, call.Method)
		}
	}
	for _, want := range []string{"unary", "server-stream", "client-stream", "bidi"} {
		if !kinds[want] {
			t.Errorf("kind %s missing from the wire payload", want)
		}
	}
	for _, want := range []string{"OK", "NotFound", "DeadlineExceeded"} {
		if !statuses[want] {
			t.Errorf("status %s never observed on the wire", want)
		}
	}
}

func TestGRPCConsoleDemoRejectsNonGet(t *testing.T) {
	var buf strings.Builder
	server := newGRPCConsoleServer(newTestLogger(&buf))

	rec := doRequest(server, http.MethodPost, "/api/demo/api/grpc-console", "")
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
