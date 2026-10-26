package presentation

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"testing"

	"guide/internal/core"
)

// The api-design topic's demo endpoint, exercised through the same
// middleware chain production runs behind (the family rule that every new
// demo endpoint ships with a Go test). The route lives in a test mux
// here; main.go's production wiring is the composition root's call.

func newPostsWorkbenchServer(logger *slog.Logger) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("GET /api/demo/api/posts-workbench", NewPostsWorkbenchHandler())
	return Chain(
		RequestID,
		RequestLogger(logger),
		Recover(logger),
		EnvelopeMuxErrors(logger),
	)(mux)
}

func TestPostsWorkbenchDemoServesTheContractCard(t *testing.T) {
	var buf strings.Builder
	server := newPostsWorkbenchServer(newTestLogger(&buf))

	rec := doRequest(server, http.MethodGet, "/api/demo/api/posts-workbench", "")
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
		Routes []struct {
			Method string `json:"method"`
			Path   string `json:"path"`
		} `json:"routes"`
		Pagination struct {
			DefaultLimit int `json:"defaultLimit"`
			MaxLimit     int `json:"maxLimit"`
		} `json:"pagination"`
		DeleteSemantics struct {
			OnMiss int `json:"onMiss"`
		} `json:"deleteSemantics"`
	}{}
	if err := json.Unmarshal(rec.Body.Bytes(), &decoded); err != nil {
		t.Fatalf("body not the demo DTO shape: %v\n%s", err, rec.Body.String())
	}
	if len(decoded.Routes) != 5 || decoded.Routes[0].Method != "GET" || decoded.Routes[0].Path != "/api/posts" {
		t.Errorf("route table drifted: %+v", decoded.Routes)
	}
	if decoded.Pagination.DefaultLimit != 10 || decoded.Pagination.MaxLimit != 50 {
		t.Errorf("pagination drifted: %+v", decoded.Pagination)
	}
	if decoded.DeleteSemantics.OnMiss != http.StatusNotFound {
		t.Errorf("delete stance drifted: %+v", decoded.DeleteSemantics)
	}
}

func TestPostsWorkbenchDemoRejectsNonGet(t *testing.T) {
	var buf strings.Builder
	server := newPostsWorkbenchServer(newTestLogger(&buf))

	rec := doRequest(server, http.MethodPost, "/api/demo/api/posts-workbench", "")
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
