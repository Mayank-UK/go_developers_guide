package presentation

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"testing"

	"guide/api"
	"guide/internal/apispec"
)

// The spec-drift test — the api-documentation topic's centerpiece. The
// OpenAPI document at server/api/openapi.yaml CLAIMS a wire shape for
// every posts route; these tests drive the REAL handlers (the same
// constructors main.go wires, behind the same middleware chain, via
// newPostsServer below — the production wiring minus only the route
// registration itself) and validate each response's JSON against the
// spec's schemas with the hand-rolled walker. A spec that lies about the
// wire — a renamed key, a missing required field, a wrong type, a value
// outside an enum, a timestamp that is not RFC 3339 — FAILS here.

// newDriftServer is newPostsServer (posts_test.go) by another name for
// readability at the call sites below: the production-shaped stack the
// drift cases exercise.
func newDriftServer(logger *slog.Logger) http.Handler {
	return newPostsServer(logger)
}

// driftWalker builds the walker from the EMBEDDED spec — the same bytes
// the route serves, so the test can never validate against a stale copy.
func driftWalker(t *testing.T) *apispec.Walker {
	t.Helper()
	walker, err := apispec.NewWalker(api.OpenAPIYAML)
	if err != nil {
		t.Fatalf("the committed spec must parse: %v", err)
	}
	return walker
}

// decodeWireJSON decodes a response body into generic JSON values — the
// shape the walker walks.
func decodeWireJSON(t *testing.T, body string) any {
	t.Helper()
	var value any
	if err := json.Unmarshal([]byte(body), &value); err != nil {
		t.Fatalf("body is not JSON: %v\n%s", err, body)
	}
	return value
}

// assertConforms fails the test with every violation when the live body
// disagrees with the named schema.
func assertConforms(t *testing.T, walker *apispec.Walker, body string, schema string) {
	t.Helper()
	if violations := walker.Validate(decodeWireJSON(t, body), schema); len(violations) > 0 {
		t.Errorf("%s conformance violations against the spec:\n  %s",
			schema, strings.Join(violations, "\n  "))
	}
}

func TestSpecDriftListResponseMatchesPostPage(t *testing.T) {
	var buf strings.Builder
	server := newDriftServer(newTestLogger(&buf))
	walker := driftWalker(t)

	rec := doRequest(server, http.MethodGet, "/api/posts?page=1&limit=3&tag=go&sort=oldest", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	assertConforms(t, walker, rec.Body.String(), "PostPage")

	// The honest empty page — items [] and true totals — must conform too.
	rec = doRequest(server, http.MethodGet, "/api/posts?page=9&limit=4", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("empty page status = %d, want 200", rec.Code)
	}
	assertConforms(t, walker, rec.Body.String(), "PostPage")
}

func TestSpecDriftSinglePostMatchesPost(t *testing.T) {
	var buf strings.Builder
	server := newDriftServer(newTestLogger(&buf))
	walker := driftWalker(t)

	rec := doRequest(server, http.MethodGet, "/api/posts/p-1", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	assertConforms(t, walker, rec.Body.String(), "Post")
}

func TestSpecDriftCreateAnswersAPostThatConforms(t *testing.T) {
	var buf strings.Builder
	server := newDriftServer(newTestLogger(&buf))
	walker := driftWalker(t)

	rec := doRequest(server, http.MethodPost, "/api/posts",
		`{"title":"Drift check","body":"Created by the spec-drift tour.","tags":["spec","drift"]}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201", rec.Code)
	}
	if location := rec.Header().Get("Location"); location == "" {
		t.Error("201 must carry the Location header the spec documents")
	}
	assertConforms(t, walker, rec.Body.String(), "Post")
}

func TestSpecDriftMissWearsThe404Envelope(t *testing.T) {
	var buf strings.Builder
	server := newDriftServer(newTestLogger(&buf))
	walker := driftWalker(t)

	rec := doRequest(server, http.MethodGet, "/api/posts/p-404", "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
	assertConforms(t, walker, rec.Body.String(), "ErrorEnvelope")
}

func TestSpecDriftValidation422CarriesIssueDetails(t *testing.T) {
	var buf strings.Builder
	server := newDriftServer(newTestLogger(&buf))
	walker := driftWalker(t)

	// Broken DRAFT: the details array of ValidationIssue — oneOf branch 1.
	rec := doRequest(server, http.MethodPost, "/api/posts", `{"title":"","body":""}`)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422", rec.Code)
	}
	assertConforms(t, walker, rec.Body.String(), "ErrorEnvelope")

	// Broken QUERY: the same envelope on the list route.
	rec = doRequest(server, http.MethodGet, "/api/posts?page=zero&sort=bogus", "")
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("list 422 status = %d, want 422", rec.Code)
	}
	assertConforms(t, walker, rec.Body.String(), "ErrorEnvelope")
}

func TestSpecDriftDecode422CarriesTheMapBranch(t *testing.T) {
	var buf strings.Builder
	server := newDriftServer(newTestLogger(&buf))
	walker := driftWalker(t)

	// A strict-decode refusal: details is the object keyed by body —
	// oneOf branch 2.
	rec := doRequest(server, http.MethodPost, "/api/posts", `{"title":"t","body":"b","priority":1}`)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422", rec.Code)
	}
	assertConforms(t, walker, rec.Body.String(), "ErrorEnvelope")
}

func TestSpecDriftCanaryFlagsALyingSpec(t *testing.T) {
	// The harness must BITE: take the real spec, lie about one property
	// name (createdAt → created_at), and the live post response must be
	// refused — proving a green run above means "wire matches document",
	// not "walker approves everything".
	var buf strings.Builder
	server := newDriftServer(newTestLogger(&buf))

	lying := strings.Replace(string(api.OpenAPIYAML), "createdAt:", "created_at:", 1)
	if lying == string(api.OpenAPIYAML) {
		t.Fatal("canary setup failed to tamper the spec text")
	}
	walker, err := apispec.NewWalker([]byte(lying))
	if err != nil {
		t.Fatalf("tampered spec must still parse: %v", err)
	}

	rec := doRequest(server, http.MethodGet, "/api/posts/p-1", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	violations := walker.Validate(decodeWireJSON(t, rec.Body.String()), "Post")
	if len(violations) == 0 {
		t.Fatal("a spec that renamed createdAt must be flagged — the walker approves everything")
	}
	if !strings.Contains(strings.Join(violations, "\n"), "createdAt") {
		t.Errorf("violations must name the honest key createdAt:\n%s", strings.Join(violations, "\n"))
	}
}
