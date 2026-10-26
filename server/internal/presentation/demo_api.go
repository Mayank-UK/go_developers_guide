// Demo API handler for topic 11 api-design — same family invariant as
// demo.go: thin over the pure payload builder in internal/domain/demos,
// nothing to parse, nothing to map. The workbench's dynamic half is the
// live /api/posts CRUD surface itself; this endpoint serves the static
// contract card (route table, pagination rules, versioning stance, delete
// semantics, envelope anatomy) that the topic and the view quote.
package presentation

import (
	"net/http"

	"guide/internal/domain/demos"
)

// Compile-time proof that the posts-workbench handler satisfies
// http.Handler — the interface the route registration below it requires.
var _ http.Handler = (*PostsWorkbenchHandler)(nil)

// PostsWorkbenchHandler serves GET /api/demo/api/posts-workbench, the
// api-design topic's inline demo payload. The builder is a pure function
// over the real posts vocabulary, so the handler holds no dependencies:
// constructing one allocates nothing to wire.
type PostsWorkbenchHandler struct{}

// NewPostsWorkbenchHandler wires the posts-workbench demo endpoint.
func NewPostsWorkbenchHandler() *PostsWorkbenchHandler {
	return &PostsWorkbenchHandler{}
}

// ServeHTTP implements http.Handler for the posts-workbench payload.
func (h *PostsWorkbenchHandler) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, demos.BuildPostsWorkbenchPayload())
}
