package presentation

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"testing"

	"guide/internal/core"
	postsdata "guide/internal/data/posts"
	"guide/internal/domain/posts"
)

// The posts routes, exercised through the same middleware chain production
// runs behind. These tests pin the WIRE contract the api-design topic
// teaches: statuses (200/201+Location/204/404/422), the page envelope's
// camelCase keys, the error envelope on every miss, and strict decode
// refusals — against the real in-memory repository, exactly as main.go
// wires it.

// newPostsServer builds the production-shaped stack for the posts routes.
func newPostsServer(logger *slog.Logger) http.Handler {
	repository := postsdata.NewInMemoryRepository()
	mux := http.NewServeMux()
	mux.Handle("GET /api/posts", NewPostsListHandler(posts.NewListPostsUseCase(repository), logger))
	mux.Handle("POST /api/posts", NewPostsCreateHandler(posts.NewCreatePostUseCase(repository), logger))
	mux.Handle("GET /api/posts/{id}", NewPostDetailHandler(posts.NewGetPostUseCase(repository), logger))
	mux.Handle("PUT /api/posts/{id}", NewPostReplaceHandler(posts.NewReplacePostUseCase(repository), logger))
	mux.Handle("DELETE /api/posts/{id}", NewPostDeleteHandler(posts.NewDeletePostUseCase(repository), logger))
	return Chain(
		RequestID,
		RequestLogger(logger),
		Recover(logger),
		EnvelopeMuxErrors(logger),
	)(mux)
}

// pageEnvelope decodes the list response's wire shape.
type pageEnvelope struct {
	Items []struct {
		ID        string   `json:"id"`
		Title     string   `json:"title"`
		Body      string   `json:"body"`
		Tags      []string `json:"tags"`
		CreatedAt string   `json:"createdAt"`
		UpdatedAt string   `json:"updatedAt"`
	} `json:"items"`
	Page        int  `json:"page"`
	Limit       int  `json:"limit"`
	TotalItems  int  `json:"totalItems"`
	TotalPages  int  `json:"totalPages"`
	HasNextPage bool `json:"hasNextPage"`
	HasPrevPage bool `json:"hasPrevPage"`
}

func decodePage(t *testing.T, body string) pageEnvelope {
	t.Helper()
	var page pageEnvelope
	if err := json.Unmarshal([]byte(body), &page); err != nil {
		t.Fatalf("body not the page envelope: %v\n%s", err, body)
	}
	return page
}

func TestPostsListServesTheSeedPage(t *testing.T) {
	var buf strings.Builder
	server := newPostsServer(newTestLogger(&buf))

	rec := doRequest(server, http.MethodGet, "/api/posts", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	page := decodePage(t, rec.Body.String())
	if page.Page != 1 || page.Limit != 10 || page.TotalItems != 6 || page.TotalPages != 1 {
		t.Errorf("defaults drifted: %+v", page)
	}
	if len(page.Items) != 6 || page.Items[0].ID != "p-6" {
		t.Errorf("items = %d starting %s, want 6 starting p-6 (newest first)", len(page.Items), firstID(page))
	}
	if page.Items[0].CreatedAt == "" || page.Items[0].UpdatedAt == "" {
		t.Error("timestamps must ride the wire as RFC 3339 strings")
	}
}

func firstID(page pageEnvelope) string {
	if len(page.Items) == 0 {
		return "(none)"
	}
	return page.Items[0].ID
}

func TestPostsListPaginatesAndFilters(t *testing.T) {
	var buf strings.Builder
	server := newPostsServer(newTestLogger(&buf))

	rec := doRequest(server, http.MethodGet, "/api/posts?page=2&limit=4&tag=go", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	page := decodePage(t, rec.Body.String())
	// Seeds tagged go: p-1, p-2, p-3, p-6 → 4 matches; 4 by 4 = one page,
	// so page 2 is the honest past-the-end empty page.
	if page.TotalItems != 4 || page.TotalPages != 1 || len(page.Items) != 0 {
		t.Errorf("filtered page 2 = %+v with %d items, want totals 4/1 and an honest empty page", page, len(page.Items))
	}
	if page.HasNextPage || !page.HasPrevPage {
		t.Errorf("neighbor flags past the end = next %v prev %v", page.HasNextPage, page.HasPrevPage)
	}

	rec = doRequest(server, http.MethodGet, "/api/posts?tag=go&sort=oldest&limit=2", "")
	page = decodePage(t, rec.Body.String())
	if page.Items[0].ID != "p-1" || len(page.Items) != 2 || !page.HasNextPage {
		t.Errorf("oldest+filtered first page = %s…, want p-1 with a next page", firstID(page))
	}
}

func TestPostsListRejectsBrokenQueriesWithTheEnvelope(t *testing.T) {
	var buf strings.Builder
	server := newPostsServer(newTestLogger(&buf))

	rec := doRequest(server, http.MethodGet, "/api/posts?page=zero&limit=999&sort=bogus", "")
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422", rec.Code)
	}
	var envelope struct {
		Error struct {
			Code    string `json:"code"`
			Details []struct {
				Field string `json:"field"`
				Rule  string `json:"rule"`
			} `json:"details"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("body not the error envelope: %v\n%s", err, rec.Body.String())
	}
	if envelope.Error.Code != "validation_failed" {
		t.Errorf("code = %q, want validation_failed", envelope.Error.Code)
	}
	fields := map[string]bool{}
	for _, issue := range envelope.Error.Details {
		fields[issue.Field] = true
	}
	for _, want := range []string{"page", "limit", "sort"} {
		if !fields[want] {
			t.Errorf("details must name every broken parameter; got %v", fields)
		}
	}
	if rec.Header().Get(core.RequestIDHeader) == "" {
		t.Error("every 422 must carry x-request-id")
	}
}

func TestPostsCreateAnswers201WithLocation(t *testing.T) {
	var buf strings.Builder
	server := newPostsServer(newTestLogger(&buf))

	body := `{"title":"A created post","body":"Fresh off the wire","tags":[" Go ","api"]}`
	rec := doRequest(server, http.MethodPost, "/api/posts", body)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201", rec.Code)
	}
	if location := rec.Header().Get("Location"); location != "/api/posts/p-7" {
		t.Errorf("Location = %q, want /api/posts/p-7", location)
	}
	var created struct {
		ID   string   `json:"id"`
		Tags []string `json:"tags"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("body not a post: %v", err)
	}
	if created.ID != "p-7" || len(created.Tags) != 2 || created.Tags[0] != "api" || created.Tags[1] != "go" {
		// Sorted order since the databases topic: tags are a set, and both
		// stores behind the port return the same canonical order.
		t.Errorf("created = %+v, tags normalized and set-ordered on the way in", created)
	}

	// And it is immediately readable at the Location the header promised.
	rec = doRequest(server, http.MethodGet, "/api/posts/p-7", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("get after create: %d", rec.Code)
	}
}

func TestPostsCreateRefusesBodiesAtTheBoundary(t *testing.T) {
	var buf strings.Builder
	server := newPostsServer(newTestLogger(&buf))

	cases := []struct {
		name string
		body string
		want string
	}{
		{"empty required fields", `{"title":"","body":""}`, `"field"`},
		{"null body", `null`, "body must be a JSON object"},
		{"unknown field", `{"title":"t","body":"b","priority":1}`, "body"},
		{"trailing document", `{"title":"t","body":"b"}{"title":"u","body":"v"}`, "exactly one JSON object"},
		{"broken syntax", `{"title":`, "body"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := doRequest(server, http.MethodPost, "/api/posts", tc.body)
			if rec.Code != http.StatusUnprocessableEntity {
				t.Fatalf("status = %d, want 422", rec.Code)
			}
			if !strings.Contains(rec.Body.String(), tc.want) {
				t.Errorf("body does not carry %q: %s", tc.want, rec.Body.String())
			}
		})
	}
}

func TestPostDetailAnswers404EnvelopeOnMiss(t *testing.T) {
	var buf strings.Builder
	server := newPostsServer(newTestLogger(&buf))

	rec := doRequest(server, http.MethodGet, "/api/posts/p-404", "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
	var envelope struct {
		Error struct {
			Code      string `json:"code"`
			RequestID string `json:"requestId"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("body not the error envelope: %v", err)
	}
	if envelope.Error.Code != "not_found" || envelope.Error.RequestID == "" {
		t.Errorf("envelope = %+v, code not_found + requestId", envelope.Error)
	}
}

func TestPostReplaceIsAFullSwap(t *testing.T) {
	var buf strings.Builder
	server := newPostsServer(newTestLogger(&buf))

	rec := doRequest(server, http.MethodPut, "/api/posts/p-1",
		`{"title":"Replaced wholly","body":"New next state","tags":["edited"]}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var replaced struct {
		Title string   `json:"title"`
		Tags  []string `json:"tags"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &replaced); err != nil {
		t.Fatalf("body: %v", err)
	}
	if replaced.Title != "Replaced wholly" || len(replaced.Tags) != 1 {
		t.Errorf("replaced = %+v", replaced)
	}

	// PUT is full replacement: the old tags are gone, not merged.
	rec = doRequest(server, http.MethodGet, "/api/posts/p-1", "")
	if !strings.Contains(rec.Body.String(), `"tags":["edited"]`) {
		t.Errorf("full replace must drop the old tags: %s", rec.Body.String())
	}

	// 404 on a miss, 422 on a broken replacement.
	if rec := doRequest(server, http.MethodPut, "/api/posts/p-404", `{"title":"t","body":"b"}`); rec.Code != http.StatusNotFound {
		t.Errorf("replace miss = %d, want 404", rec.Code)
	}
	if rec := doRequest(server, http.MethodPut, "/api/posts/p-1", `{"title":"","body":"b"}`); rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("broken replacement = %d, want 422", rec.Code)
	}
}

func TestPostDeleteIs204Then404(t *testing.T) {
	var buf strings.Builder
	server := newPostsServer(newTestLogger(&buf))

	rec := doRequest(server, http.MethodDelete, "/api/posts/p-3", "")
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rec.Code)
	}
	if rec.Body.Len() != 0 {
		t.Errorf("204 must carry no body, got %q", rec.Body.String())
	}
	rec = doRequest(server, http.MethodDelete, "/api/posts/p-3", "")
	if rec.Code != http.StatusNotFound || !strings.Contains(rec.Body.String(), `"code":"not_found"`) {
		t.Errorf("second delete = %d %s, want the 404 envelope (strict stance)", rec.Code, rec.Body.String())
	}
	rec = doRequest(server, http.MethodGet, "/api/posts/p-3", "")
	if rec.Code != http.StatusNotFound {
		t.Errorf("get after delete = %d, want 404", rec.Code)
	}
}

func TestPostsRoutesKeepTheirMethods(t *testing.T) {
	var buf strings.Builder
	server := newPostsServer(newTestLogger(&buf))

	rec := doRequest(server, http.MethodPatch, "/api/posts/p-1", `{}`)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405 (no PATCH route on posts)", rec.Code)
	}
	if allow := rec.Header().Get("Allow"); !strings.Contains(allow, "GET") || !strings.Contains(allow, "PUT") || !strings.Contains(allow, "DELETE") {
		t.Errorf("Allow = %q, want GET, PUT, DELETE", allow)
	}
	if !strings.Contains(rec.Body.String(), `"code":"not_found"`) {
		t.Errorf("405 must ride the shared envelope: %s", rec.Body.String())
	}
}
