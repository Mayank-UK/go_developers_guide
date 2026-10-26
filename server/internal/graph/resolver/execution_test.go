package resolver_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/99designs/gqlgen/graphql"

	"guide/internal/core"
	postsmemory "guide/internal/data/posts"
	"guide/internal/domain/posts"
	"guide/internal/graph"
	"guide/internal/graph/generated"
	"guide/internal/graph/resolver"
)

// The graph's execution contract, exercised in-process through the SAME
// path POST /api/graphql serves (graph.Execute over the generated
// executor over a scratch repository): the page envelope mirrors the REST
// wire field-for-field, misses and rule violations arrive as GraphQL
// errors whose extensions carry the taxonomy codes, and the mutation
// round-trip works end to end without touching any shared state.

// scratchSchema builds a one-test GraphQL surface over a fresh in-memory
// repository — the same private-world shape the playground demo uses.
func scratchSchema() graphql.ExecutableSchema {
	repo := postsmemory.NewRepository()
	return generated.NewExecutableSchema(generated.Config{
		Resolvers: resolver.NewResolver(
			posts.NewListPostsUseCase(repo),
			posts.NewGetPostUseCase(repo),
			posts.NewCreatePostUseCase(repo),
			posts.NewReplacePostUseCase(repo),
			posts.NewDeletePostUseCase(repo),
		),
	})
}

// run executes one operation and decodes the response into a generic map,
// the shape every assertion below reads.
func run(t *testing.T, query string) (data map[string]any, errors []map[string]any) {
	t.Helper()
	response := graph.Execute(context.Background(), scratchSchema(), graph.Request{Query: query})
	var decoded struct {
		Data   map[string]any   `json:"data"`
		Errors []map[string]any `json:"errors"`
	}
	if err := json.Unmarshal(mustJSON(t, response), &decoded); err != nil {
		t.Fatalf("response is not the {data,errors} document: %v", err)
	}
	return decoded.Data, decoded.Errors
}

// mustJSON marshals the response the way writeJSON will.
func mustJSON(t *testing.T, response *graphql.Response) []byte {
	t.Helper()
	raw, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("marshal response: %v", err)
	}
	return raw
}

func TestPostsQueryMirrorsTheRestPageEnvelope(t *testing.T) {
	data, errs := run(t, `query { posts(limit: 2, sort: "oldest") {
		items { id title tags createdAt }
		page limit totalItems totalPages hasNextPage hasPrevPage
	} }`)
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	page, _ := data["posts"].(map[string]any)
	if page == nil {
		t.Fatalf("posts missing from data: %v", data)
	}
	if page["page"].(float64) != 1 || page["limit"].(float64) != 2 {
		t.Errorf("page/limit = %v/%v, want 1/2", page["page"], page["limit"])
	}
	if page["totalItems"].(float64) != 6 || page["totalPages"].(float64) != 3 {
		t.Errorf("totals = %v/%v, want 6/3", page["totalItems"], page["totalPages"])
	}
	if page["hasNextPage"] != true || page["hasPrevPage"] != false {
		t.Errorf("neighbor flags = %v/%v, want true/false", page["hasNextPage"], page["hasPrevPage"])
	}
	items, _ := page["items"].([]any)
	if len(items) != 2 {
		t.Fatalf("items = %d, want 2", len(items))
	}
	first, _ := items[0].(map[string]any)
	if first["id"] != "p-1" {
		t.Errorf("oldest-first first item = %v, want p-1", first["id"])
	}
	// camelCase keys, exactly the REST wire's vocabulary.
	raw := string(mustJSON(t, graph.Execute(context.Background(), scratchSchema(), graph.Request{
		Query: `query { posts(limit: 1) { items { id createdAt } totalItems hasNextPage } }`,
	})))
	for _, key := range []string{`"totalItems"`, `"hasNextPage"`, `"createdAt"`, `"items"`} {
		if !strings.Contains(raw, key) {
			t.Errorf("wire lacks the REST envelope key %s: %s", key, raw)
		}
	}
}

func TestPostMissCarriesTheNotFoundExtensionCode(t *testing.T) {
	data, errs := run(t, `query { post(id: "p-999") { id title } }`)
	if len(errs) != 1 {
		t.Fatalf("errors = %v, want the one not_found entry", errs)
	}
	first := errs[0]
	if first["extensions"].(map[string]any)["code"] != string(core.KindNotFound) {
		t.Errorf("extensions.code = %v, want %q", first["extensions"], core.KindNotFound)
	}
	// The nullable field answers null beside the error — data still parses.
	if post := data["post"]; post != nil {
		t.Errorf("post = %v, want null beside the error", post)
	}
}

func TestRuleViolationsCarryValidationFailedWithDetails(t *testing.T) {
	_, errs := run(t, `query { posts(limit: 500) { totalItems } }`)
	if len(errs) != 1 {
		t.Fatalf("errors = %v, want the one validation_failed entry", errs)
	}
	first := errs[0]
	extensions, _ := first["extensions"].(map[string]any)
	if extensions["code"] != string(core.KindValidationFailed) {
		t.Errorf("extensions.code = %v, want %q", extensions, core.KindValidationFailed)
	}
	details, _ := extensions["details"].([]any)
	if len(details) == 0 {
		t.Errorf("extensions.details = %v, want the collected issue list riding along", details)
	}
}

func TestMutationRoundTripOnTheScratchWorld(t *testing.T) {
	schema := scratchSchema() // one private world for the whole story
	execute := func(query string) (map[string]any, []map[string]any) {
		var decoded struct {
			Data   map[string]any   `json:"data"`
			Errors []map[string]any `json:"errors"`
		}
		raw := mustJSON(t, graph.Execute(context.Background(), schema, graph.Request{Query: query}))
		if err := json.Unmarshal(raw, &decoded); err != nil {
			t.Fatalf("decode: %v", err)
		}
		return decoded.Data, decoded.Errors
	}

	data, errs := execute(`mutation { createPost(input: {
		title: "GraphQL wrote this" body: "Through one endpoint." tags: ["graphql", "demo"]
	}) { id title tags } }`)
	if len(errs) != 0 {
		t.Fatalf("create errors: %v", errs)
	}
	created, _ := data["createPost"].(map[string]any)
	if created["id"] != "p-7" || created["title"] != "GraphQL wrote this" {
		t.Fatalf("created = %v, want p-7 titled by the mutation", created)
	}

	data, errs = execute(`mutation { updatePost(id: "p-7", input: {
		title: "Edited through the graph" body: "Full replacement." tags: ["edited"]
	}) { id title tags } }`)
	if len(errs) != 0 {
		t.Fatalf("update errors: %v", errs)
	}
	updated, _ := data["updatePost"].(map[string]any)
	if updated["title"] != "Edited through the graph" {
		t.Fatalf("updated = %v, want the replaced title", updated)
	}

	data, errs = execute(`mutation { deletePost(id: "p-7") }`)
	if len(errs) != 0 {
		t.Fatalf("delete errors: %v", errs)
	}
	if data["deletePost"] != "p-7" {
		t.Fatalf("deletePost = %v, want the removed id echoed back", data["deletePost"])
	}

	_, errs = execute(`query { post(id: "p-7") { id } }`)
	if len(errs) != 1 || errs[0]["extensions"].(map[string]any)["code"] != string(core.KindNotFound) {
		t.Fatalf("post after delete = %v, want the strict not_found miss", errs)
	}
}

func TestIntrospectionAnswersThroughTheExecutionPath(t *testing.T) {
	data, errs := run(t, `query { __schema { queryType { name } mutationType { name } } }`)
	if len(errs) != 0 {
		t.Fatalf("introspection errors: %v", errs)
	}
	schema, _ := data["__schema"].(map[string]any)
	if schema["queryType"].(map[string]any)["name"] != "Query" ||
		schema["mutationType"].(map[string]any)["name"] != "Mutation" {
		t.Errorf("__schema = %v, want Query/Mutation root names", schema)
	}
}
