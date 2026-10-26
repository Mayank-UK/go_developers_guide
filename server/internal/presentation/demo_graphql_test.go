package presentation

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"testing"

	"guide/internal/core"
	postsmemory "guide/internal/data/posts"
	"guide/internal/domain/posts"
)

// The graphql topic's two demo endpoints, exercised through the same
// middleware chain production runs behind (the family rule that every new
// demo endpoint ships with a Go test). The routes live in a test mux
// here; main.go's production wiring is the composition root's call.

// newGraphQLDemoServer builds the topic's whole surface over one
// repository: the playground payload route plus the real POST /api/graphql
// endpoint sharing the store with a scratch payload world.
func newGraphQLDemoServer(logger *slog.Logger) http.Handler {
	repo := postsmemory.NewRepository()
	mux := http.NewServeMux()
	mux.Handle("GET /api/demo/api/graphql-playground",
		NewGraphQLPlaygroundHandler(func() posts.PostsRepository {
			return postsmemory.NewRepository()
		}))
	mux.Handle("POST /api/graphql", NewGraphQLHandler(
		posts.NewListPostsUseCase(repo),
		posts.NewGetPostUseCase(repo),
		posts.NewCreatePostUseCase(repo),
		posts.NewReplacePostUseCase(repo),
		posts.NewDeletePostUseCase(repo),
		logger,
	))
	return Chain(
		RequestID,
		RequestLogger(logger),
		Recover(logger),
		EnvelopeMuxErrors(logger),
	)(mux)
}

// graphResponse is the endpoint's decoded answer for the assertions.
type graphResponse struct {
	Errors []struct {
		Message    string         `json:"message"`
		Extensions map[string]any `json:"extensions"`
	} `json:"errors"`
	Data map[string]any `json:"data"`
}

func postGraphQLErrorFree(t *testing.T, server http.Handler, query string) graphResponse {
	t.Helper()
	rec := doRequest(server, http.MethodPost, "/api/graphql",
		`{"query":`+jsonString(t, query)+`}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var decoded graphResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &decoded); err != nil {
		t.Fatalf("body is not the GraphQL response document: %v\n%s", err, rec.Body.String())
	}
	return decoded
}

func TestGraphQLPlaygroundDemoServesThePayload(t *testing.T) {
	var buf strings.Builder
	server := newGraphQLDemoServer(newTestLogger(&buf))

	rec := doRequest(server, http.MethodGet, "/api/demo/api/graphql-playground", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("content type = %q, want application/json", ct)
	}
	if rec.Header().Get(core.RequestIDHeader) == "" {
		t.Error("every response must carry x-request-id")
	}

	decoded := struct {
		Endpoint struct {
			Path   string `json:"path"`
			Method string `json:"method"`
		} `json:"endpoint"`
		Runs   []struct{ ID string } `json:"runs"`
		Loader struct {
			NaivePortCalls  int `json:"naivePortCalls"`
			LoaderPortCalls int `json:"loaderPortCalls"`
		} `json:"loader"`
	}{}
	if err := json.Unmarshal(rec.Body.Bytes(), &decoded); err != nil {
		t.Fatalf("body not the demo DTO shape: %v\n%s", err, rec.Body.String())
	}
	if decoded.Endpoint.Path != "/api/graphql" || decoded.Endpoint.Method != "POST" {
		t.Errorf("endpoint card = %+v, want POST /api/graphql", decoded.Endpoint)
	}
	if len(decoded.Runs) != 8 {
		t.Errorf("runs = %d, want the eight executed samples", len(decoded.Runs))
	}
	if decoded.Loader.NaivePortCalls != 7 || decoded.Loader.LoaderPortCalls != 1 {
		t.Errorf("loader counts = %+v, want naive 7 vs loader 1", decoded.Loader)
	}
}

func TestGraphQLPlaygroundDemoRejectsNonGet(t *testing.T) {
	var buf strings.Builder
	server := newGraphQLDemoServer(newTestLogger(&buf))

	rec := doRequest(server, http.MethodPost, "/api/demo/api/graphql-playground", "")
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"code":"not_found"`) {
		t.Errorf("405 body is not the envelope: %s", rec.Body.String())
	}
}

func TestGraphQLEndpointServesQueriesOverTheRealPostsWorld(t *testing.T) {
	var buf strings.Builder
	server := newGraphQLDemoServer(newTestLogger(&buf))

	decoded := postGraphQLErrorFree(t, server, `query { posts(tag: "go", limit: 3) {
		items { id title }
		totalItems totalPages hasNextPage
	} }`)
	if len(decoded.Errors) != 0 {
		t.Fatalf("unexpected errors: %v", decoded.Errors)
	}
	page, _ := decoded.Data["posts"].(map[string]any)
	if page == nil || page["totalItems"].(float64) != 4 || page["totalPages"].(float64) != 2 {
		t.Errorf("filtered page = %v, want 4 go-tagged posts over 2 pages of 3", decoded.Data)
	}
	if page["hasNextPage"] != true {
		t.Errorf("hasNextPage = %v, want true", page["hasNextPage"])
	}
}

func TestGraphQLEndpointServesTheMutationRoundTrip(t *testing.T) {
	var buf strings.Builder
	server := newGraphQLDemoServer(newTestLogger(&buf))

	created := postGraphQLErrorFree(t, server, `mutation { createPost(input: {
		title: "From the endpoint test" body: "Round trip." tags: ["test"]
	}) { id title tags } }`)
	if len(created.Errors) != 0 {
		t.Fatalf("create errors: %v", created.Errors)
	}
	post, _ := created.Data["createPost"].(map[string]any)
	if post["id"] != "p-7" {
		t.Errorf("created = %v, want p-7 on this server's fresh world", post)
	}

	deleted := postGraphQLErrorFree(t, server, `mutation { deletePost(id: "p-7") }`)
	if deleted.Data["deletePost"] != "p-7" {
		t.Errorf("deletePost = %v, want p-7 echoed back", deleted.Data)
	}
}

func TestGraphQLEndpointAnswersErrorsWithExtensionCodes(t *testing.T) {
	var buf strings.Builder
	server := newGraphQLDemoServer(newTestLogger(&buf))

	miss := postGraphQLErrorFree(t, server, `query { post(id: "p-999") { id } }`)
	if len(miss.Errors) != 1 || miss.Errors[0].Extensions["code"] != "not_found" {
		t.Fatalf("miss errors = %+v, want one not_found entry", miss.Errors)
	}
	if miss.Data["post"] != nil {
		t.Errorf("miss data.post = %v, want null beside the error", miss.Data["post"])
	}

	violation := postGraphQLErrorFree(t, server, `query { posts(sort: "sideways") { totalItems } }`)
	if len(violation.Errors) != 1 || violation.Errors[0].Extensions["code"] != "validation_failed" {
		t.Fatalf("violation errors = %+v, want one validation_failed entry", violation.Errors)
	}

	unknown := postGraphQLErrorFree(t, server, `query { nosuchfield }`)
	if len(unknown.Errors) == 0 {
		t.Error("unknown field must surface as an errors entry")
	}
}

func TestGraphQLEndpointRejectsBrokenTransportsWith400(t *testing.T) {
	var buf strings.Builder
	server := newGraphQLDemoServer(newTestLogger(&buf))

	rec := doRequest(server, http.MethodPost, "/api/graphql", `{"query": `)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("bad JSON status = %d, want 400", rec.Code)
	}
	var decoded graphResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &decoded); err != nil {
		t.Fatalf("400 body is not the GraphQL error shape: %v\n%s", err, rec.Body.String())
	}
	if len(decoded.Errors) != 1 || decoded.Errors[0].Message == "" {
		t.Errorf("400 errors = %+v, want one shaped message", decoded.Errors)
	}

	rec = doRequest(server, http.MethodPost, "/api/graphql", `{"variables":{}}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("missing query status = %d, want 400", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "query is required") {
		t.Errorf("missing query body = %s", rec.Body.String())
	}
}

func TestGraphQLEndpointServesIntrospection(t *testing.T) {
	var buf strings.Builder
	server := newGraphQLDemoServer(newTestLogger(&buf))

	decoded := postGraphQLErrorFree(t, server,
		`query { __schema { queryType { name } mutationType { name } } }`)
	if len(decoded.Errors) != 0 {
		t.Fatalf("introspection errors: %v", decoded.Errors)
	}
	schema, _ := decoded.Data["__schema"].(map[string]any)
	if schema == nil || schema["queryType"].(map[string]any)["name"] != "Query" {
		t.Errorf("__schema = %v, want the Query root", decoded.Data)
	}
}

func TestGraphQLEndpointRejectsNonPost(t *testing.T) {
	var buf strings.Builder
	server := newGraphQLDemoServer(newTestLogger(&buf))

	rec := doRequest(server, http.MethodGet, "/api/graphql", "")
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", rec.Code)
	}
	if allow := rec.Header().Get("Allow"); !strings.Contains(allow, "POST") {
		t.Errorf("Allow header lost: %q", allow)
	}
}

// jsonString encodes a Go string as a JSON string literal.
func jsonString(t *testing.T, value string) string {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal query: %v", err)
	}
	return string(raw)
}
