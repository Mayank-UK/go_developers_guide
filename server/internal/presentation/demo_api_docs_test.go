package presentation

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"testing"

	"guide/api"
	"guide/internal/core"
)

// The api-documentation topic's two endpoints, exercised through the same
// middleware chain production runs behind (the family rule that every new
// demo endpoint ships with a Go test). The routes live in a test mux
// here; main.go's production wiring is the composition root's call.

func newAPIDocsServer(logger *slog.Logger) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("GET /api/demo/api/docs-link", NewAPIDocsLinkHandler(logger))
	mux.Handle("GET /api/openapi.yaml", NewOpenAPISpecHandler())
	return Chain(
		RequestID,
		RequestLogger(logger),
		Recover(logger),
		EnvelopeMuxErrors(logger),
	)(mux)
}

func TestAPIDocsLinkDemoServesSpecFacts(t *testing.T) {
	var buf strings.Builder
	server := newAPIDocsServer(newTestLogger(&buf))

	rec := doRequest(server, http.MethodGet, "/api/demo/api/docs-link", "")
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
		Spec struct {
			OpenAPIVersion string `json:"openapiVersion"`
			PathCount      int    `json:"pathCount"`
			OperationCount int    `json:"operationCount"`
			SchemaCount    int    `json:"schemaCount"`
		} `json:"spec"`
		Serving struct {
			SpecURL string `json:"specURL"`
		} `json:"serving"`
		SpecText string `json:"specText"`
	}{}
	if err := json.Unmarshal(rec.Body.Bytes(), &decoded); err != nil {
		t.Fatalf("body not the demo DTO shape: %v\n%s", err, rec.Body.String())
	}
	if decoded.Spec.OpenAPIVersion != "3.1.0" {
		t.Errorf("openapiVersion = %q, want 3.1.0", decoded.Spec.OpenAPIVersion)
	}
	if decoded.Spec.PathCount != 2 || decoded.Spec.OperationCount != 5 || decoded.Spec.SchemaCount != 6 {
		t.Errorf("counts = %d/%d/%d, want 2 paths, 5 operations, 6 schemas",
			decoded.Spec.PathCount, decoded.Spec.OperationCount, decoded.Spec.SchemaCount)
	}
	if decoded.Serving.SpecURL != "/api/openapi.yaml" {
		t.Errorf("serving.specURL = %q, want /api/openapi.yaml", decoded.Serving.SpecURL)
	}
	if !strings.Contains(decoded.SpecText, "operationId: listPosts") {
		t.Error("payload must carry the verbatim spec text")
	}
}

func TestOpenAPISpecRouteServesTheFileVerbatim(t *testing.T) {
	var buf strings.Builder
	server := newAPIDocsServer(newTestLogger(&buf))

	rec := doRequest(server, http.MethodGet, "/api/openapi.yaml", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/yaml" {
		t.Errorf("content type = %q, want application/yaml", ct)
	}
	if rec.Header().Get(core.RequestIDHeader) == "" {
		t.Error("every response must carry x-request-id")
	}
	// VERBATIM is the contract: the exact embedded bytes, no
	// transformation.
	if !bytes.Equal(rec.Body.Bytes(), api.OpenAPIYAML) {
		t.Error("spec route must serve the embedded file byte for byte")
	}
	if !strings.Contains(rec.Body.String(), "openapi: 3.1.0") {
		t.Error("the served spec must open with its version line")
	}
}

func TestAPIDocsEndpointsRejectNonGet(t *testing.T) {
	var buf strings.Builder
	server := newAPIDocsServer(newTestLogger(&buf))

	for _, path := range []string{"/api/demo/api/docs-link", "/api/openapi.yaml"} {
		rec := doRequest(server, http.MethodPost, path, "")
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("POST %s = %d, want 405", path, rec.Code)
		}
		if !strings.Contains(rec.Body.String(), `"code":"not_found"`) {
			t.Errorf("405 body is not the envelope: %s", rec.Body.String())
		}
		if allow := rec.Header().Get("Allow"); !strings.Contains(allow, "GET") {
			t.Errorf("Allow header lost: %q", allow)
		}
	}
}
