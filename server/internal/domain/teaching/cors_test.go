package teaching

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCORSMiddlewareAnswersPreflight(t *testing.T) {
	handler := CORSMiddleware(DefaultCORSConfig)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Error("a preflight must never reach the wrapped handler")
	}))
	req := httptest.NewRequest(http.MethodOptions, "/api/posts", nil)
	req.Header.Set("Origin", DefaultCORSConfig.Origin)
	req.Header.Set("Access-Control-Request-Method", http.MethodGet)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("preflight status = %d, want 204", rec.Code)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != DefaultCORSConfig.Origin {
		t.Errorf("allow-origin = %q, want the configured origin", got)
	}
	if got := rec.Header().Get("Access-Control-Allow-Methods"); got != "GET, OPTIONS" {
		t.Errorf("allow-methods = %q", got)
	}
	if got := rec.Header().Get("Access-Control-Allow-Headers"); got != "Content-Type, x-request-id" {
		t.Errorf("allow-headers = %q", got)
	}
}

func TestCORSMiddlewareStampsSimpleResponses(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := CORSMiddleware(DefaultCORSConfig)(inner)
	req := httptest.NewRequest(http.MethodGet, "/api/posts", nil)
	req.Header.Set("Origin", DefaultCORSConfig.Origin)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("simple request status = %d", rec.Code)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != DefaultCORSConfig.Origin {
		t.Errorf("simple responses must carry allow-origin, got %q", got)
	}
}

func TestCORSMiddlewareIgnoresUnrelatedOrigins(t *testing.T) {
	served := false
	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		served = true
		w.WriteHeader(http.StatusOK)
	})
	handler := CORSMiddleware(DefaultCORSConfig)(inner)
	req := httptest.NewRequest(http.MethodGet, "/api/posts", nil)
	req.Header.Set("Origin", "https://evil.example.net")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// The request still runs (the server cannot know which origin a
	// non-browser client claims), but NO CORS header vouches for the
	// evil origin, so the browser blocks the read.
	if !served {
		t.Error("the handler should still serve the request itself")
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("unrelated origin got allow-origin = %q, want none", got)
	}
}

func TestOriginMatchesIsExactNotSubstring(t *testing.T) {
	allowed := []string{"https://example.com"}
	if !originMatches(allowed, "https://example.com") {
		t.Error("exact origin must match")
	}
	for _, hostile := range []string{"https://evil-example.com", "https://example.com.evil.net"} {
		if originMatches(allowed, hostile) {
			t.Errorf("origin %q must NOT match a policy it is abusing", hostile)
		}
	}
}
