// Topic 13 graphql's live demo: the playground companion payload — the
// graph as DATA. The builder executes REAL sample operations IN-PROCESS
// through the same execution path POST /api/graphql serves (graph.Execute
// over the generated executor), on a SCRATCH repository the caller
// injects: a paginated query, a read, a full create→update→delete
// mutation round-trip, the two error shapes (not_found and
// validation_failed, extensions codes and all), an introspection query,
// and the N+1-versus-dataloader experiment with its port-call counts.
// Deterministic by construction: fixed seeds, fixed ids, no timestamps in
// any selected field.
package demos

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/99designs/gqlgen/graphql"
	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vektah/gqlparser/v2/gqlerror"

	"guide/internal/domain/posts"
	"guide/internal/graph"
	"guide/internal/graph/generated"
	"guide/internal/graph/loader"
	"guide/internal/graph/resolver"
)

// GraphQLEndpointMeta is the playground's transport card: where the real
// endpoint lives and what one request body looks like.
type GraphQLEndpointMeta struct {
	Path        string `json:"path"`
	Method      string `json:"method"`
	RequestBody string `json:"requestBody"`
	LiveNote    string `json:"liveNote"`
}

// GraphQLFieldCard is one field of a schema type, with its GraphQL type
// rendered the way the schema language writes it.
type GraphQLFieldCard struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

// GraphQLTypeCard is one named type from the real parsed schema — the
// card the demo renders beside the introspection story.
type GraphQLTypeCard struct {
	Name   string             `json:"name"`
	Kind   string             `json:"kind"`
	Fields []GraphQLFieldCard `json:"fields"`
}

// GraphQLSchemaCard summarizes the schema as served by the endpoint: the
// root operation signatures and the domain types they exchange.
type GraphQLSchemaCard struct {
	Operations []string          `json:"operations"`
	Types      []GraphQLTypeCard `json:"types"`
}

// GraphQLSample is one selectable playground operation: the query text as
// a string, ready to POST as {"query": …} once the endpoint is wired (and
// runnable through the demo console's RUN button).
type GraphQLSample struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Kind  string `json:"kind"`
	Query string `json:"query"`
	Note  string `json:"note"`
}

// GraphQLErrorView is one errors[] entry the surface can answer — message
// plus the taxonomy-carrying extensions.
type GraphQLErrorView struct {
	Message    string         `json:"message"`
	Extensions map[string]any `json:"extensions,omitempty"`
}

// GraphQLRunResult is one sample operation EXECUTED in-process against
// the scratch world: the data document verbatim, the errors list ([] when
// clean), and the notes that keep the run honest about determinism.
type GraphQLRunResult struct {
	ID     string             `json:"id"`
	Label  string             `json:"label"`
	Query  string             `json:"query"`
	Data   json.RawMessage    `json:"data"`
	Errors []GraphQLErrorView `json:"errors"`
	Notes  []string           `json:"notes"`
}

// GraphQLLoaderExperiment is the N+1 story as numbers: the same page of
// seed posts hydrated two ways over the same instrumented port.
type GraphQLLoaderExperiment struct {
	PostCount       int               `json:"postCount"`
	LoadsIssued     int               `json:"loadsIssued"`
	DuplicateLoads  int               `json:"duplicateLoads"`
	UniqueIDs       int               `json:"uniqueIds"`
	NaivePortCalls  int               `json:"naivePortCalls"`
	NaiveBreakdown  loader.PortCounts `json:"naiveBreakdown"`
	LoaderPortCalls int               `json:"loaderPortCalls"`
	LoaderBreakdown loader.PortCounts `json:"loaderBreakdown"`
	AllResolved     bool              `json:"allResolved"`
	Verdict         string            `json:"verdict"`
}

// GraphQLPlaygroundPayload is the whole GET /api/demo/api/graphql-playground
// body. Every collection is non-nil so the wire renders [] — never null.
type GraphQLPlaygroundPayload struct {
	Endpoint GraphQLEndpointMeta     `json:"endpoint"`
	Schema   GraphQLSchemaCard       `json:"schema"`
	Samples  []GraphQLSample         `json:"samples"`
	Runs     []GraphQLRunResult      `json:"runs"`
	Loader   GraphQLLoaderExperiment `json:"loader"`
}

// sampleSpec is the single source of truth for one playground operation:
// the Samples list hands the query to the client, the Runs section
// executes the very same text in-process — they cannot drift.
type sampleSpec struct {
	id    string
	label string
	kind  string
	query string
	note  string
	run   []string // determinism notes for the executed result
}

// sampleSpecs are the playground's operations, in teaching order: read
// the page, read one, write the round-trip, meet the two error shapes,
// then introspect.
func sampleSpecs() []sampleSpec {
	return []sampleSpec{
		{
			id: "list-first-page", label: "List the first page (paginated)", kind: "query",
			query: `query { posts(page: 1, limit: 2, sort: "oldest") {
  items { id title }
  page limit totalItems totalPages hasNextPage hasPrevPage
} }`,
			note: "the page envelope, field-for-field the REST wire's shape",
			run:  []string{"runs against the deterministic six-post seed; oldest-first order is pinned by fixed January 2026 stamps"},
		},
		{
			id: "read-one", label: "Read one post by id", kind: "query",
			query: `query { post(id: "p-3") { id title tags } }`,
			note:  "nullable field: a miss answers null plus an errors entry",
		},
		{
			id: "create-post", label: "Create a post (mutation)", kind: "mutation",
			query: `mutation { createPost(input: {
  title: "GraphQL wrote this"
  body: "One endpoint, one document, the fields you asked for."
  tags: ["graphql", "demo"]
}) { id title tags } }`,
			note: "the first scratch-world create is p-7, like the REST tour's",
			run:  []string{"the repository stamps createdAt/updatedAt live, so the selection pins the deterministic fields"},
		},
		{
			id: "update-post", label: "Full-replace that post (mutation)", kind: "mutation",
			query: `mutation { updatePost(id: "p-7", input: {
  title: "Edited through the graph"
  body: "The input is the whole next state — old tags are gone, not merged."
  tags: ["edited"]
}) { id title tags } }`,
			note: "PUT semantics: omitting a field removes it",
		},
		{
			id: "delete-post", label: "Delete it (mutation, strict stance)", kind: "mutation",
			query: `mutation { deletePost(id: "p-7") }`,
			note:  "answers the removed id; deleting a vanished post is a not_found error",
		},
		{
			id: "error-not-found", label: "A miss, the GraphQL way", kind: "error",
			query: `query { post(id: "p-999") { id title } }`,
			note:  "200 with {data: {post: null}} and an errors entry — extensions.code is the same not_found the REST 404 envelope serves",
		},
		{
			id: "error-validation", label: "A rule violation, details and all", kind: "error",
			query: `query { posts(limit: 500) { totalItems } }`,
			note:  "the same rules the REST list enforces; extensions.details carries the issue list the 422 serves",
		},
		{
			id: "introspection", label: "Ask the schema about itself", kind: "introspection",
			query: `query { __schema { queryType { name } mutationType { name } } }`,
			note:  "how every GraphQL tool discovers the contract at runtime",
		},
	}
}

// BuildGraphQLPlaygroundPayload assembles the whole demo. The scratch
// repository factory is INJECTED (the composition root passes the
// in-memory adapter's constructor): the builder never imports storage,
// and the mutations it demonstrates land on a private world, never on the
// live /api/posts substrate.
func BuildGraphQLPlaygroundPayload(newScratch func() posts.PostsRepository) GraphQLPlaygroundPayload {
	schema := scratchSchema(newScratch)
	specs := sampleSpecs()

	samples := make([]GraphQLSample, 0, len(specs))
	runs := make([]GraphQLRunResult, 0, len(specs))
	for _, spec := range specs {
		samples = append(samples, GraphQLSample{
			ID: spec.id, Label: spec.label, Kind: spec.kind, Query: spec.query, Note: spec.note,
		})
		response := graph.Execute(context.Background(), schema, graph.Request{Query: spec.query})
		run := GraphQLRunResult{
			ID:     spec.id,
			Label:  spec.label,
			Query:  spec.query,
			Data:   response.Data,
			Errors: errorViews(response.Errors),
			Notes:  append([]string(nil), spec.run...),
		}
		if run.Notes == nil {
			run.Notes = []string{}
		}
		runs = append(runs, run)
	}

	return GraphQLPlaygroundPayload{
		Endpoint: GraphQLEndpointMeta{
			Path:        "/api/graphql",
			Method:      "POST",
			RequestBody: `{"query":"{ posts(limit: 2) { items { id title } } }"}`,
			LiveNote: "the route is wired at the composition root (cmd/guide/main.go, POST /api/graphql); " +
				"these runs execute the SAME generated executor in-process over a scratch repository, " +
				"so the payload is deterministic while the RUN button and the curl tour hit the live endpoint",
		},
		Schema:  schemaCard(schema),
		Samples: samples,
		Runs:    runs,
		Loader:  loaderExperiment(newScratch),
	}
}

// scratchSchema builds the demo's private world: a fresh repository
// behind the real use cases behind the real resolver behind the real
// generated executor — every layer production uses, none of the shared
// state.
func scratchSchema(newScratch func() posts.PostsRepository) graphql.ExecutableSchema {
	repo := newScratch()
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

// errorViews projects the executor's error list onto the wire view.
func errorViews(errors gqlerror.List) []GraphQLErrorView {
	views := make([]GraphQLErrorView, 0, len(errors))
	for _, err := range errors {
		views = append(views, GraphQLErrorView{
			Message:    err.Message,
			Extensions: err.Extensions,
		})
	}
	return views
}

// schemaCard reads the contract off the REAL parsed schema — the same
// document introspection serves — so the card cannot drift from the
// executor that answers.
func schemaCard(schema graphql.ExecutableSchema) GraphQLSchemaCard {
	types := schema.Schema().Types
	card := GraphQLSchemaCard{
		Operations: rootSignatures(types["Query"], types["Mutation"]),
		Types:      []GraphQLTypeCard{},
	}
	for _, name := range []string{"Post", "PostPage", "PostInput"} {
		definition := types[name]
		if definition == nil {
			continue
		}
		typeCard := GraphQLTypeCard{
			Name:   name,
			Kind:   kindName(definition.Kind),
			Fields: []GraphQLFieldCard{},
		}
		for _, field := range definition.Fields {
			typeCard.Fields = append(typeCard.Fields, GraphQLFieldCard{
				Name: field.Name,
				Type: field.Type.String(),
			})
		}
		card.Types = append(card.Types, typeCard)
	}
	return card
}

// rootSignatures renders the query and mutation roots as signature
// strings — the operation vocabulary in one glance.
func rootSignatures(query, mutation *ast.Definition) []string {
	signatures := make([]string, 0, 5)
	for _, root := range []*ast.Definition{query, mutation} {
		if root == nil {
			continue
		}
		prefix := strings.ToLower(root.Name)
		for _, field := range root.Fields {
			// Skip the introspection meta-fields (__schema, __type) the
			// parser grafts onto every Query root — real, but not this
			// API's operations; the introspection RUN covers them.
			if strings.HasPrefix(field.Name, "__") {
				continue
			}
			args := make([]string, 0, len(field.Arguments))
			for _, arg := range field.Arguments {
				args = append(args, arg.Name+": "+arg.Type.String())
			}
			signatures = append(signatures, fmt.Sprintf("%s %s(%s): %s",
				prefix, field.Name, strings.Join(args, ", "), field.Type.String()))
		}
	}
	return signatures
}

// kindName maps an ast kind onto the card's wire vocabulary.
func kindName(kind ast.DefinitionKind) string {
	if kind == ast.InputObject {
		return "input"
	}
	return "object"
}

// loaderExperiment runs the counting experiment: the naive N+1 hydration
// (one list, then one point read per post) against the loader path (one
// batched port call for every concurrent load, duplicates collapsed).
func loaderExperiment(newScratch func() posts.PostsRepository) GraphQLLoaderExperiment {
	// Naive: 1 List + one GetByID per item.
	naiveRepo := loader.NewCountingRepository(newScratch())
	naivePage, err := naiveRepo.List(context.Background(), posts.ListQuery{
		Page: 1, Limit: posts.MaxPageLimit, Sort: posts.SortOldest,
	})
	if err != nil {
		// Unreachable with the in-memory adapter; a scratch-world failure
		// is a programmer bug the demo should not paper over.
		panic(fmt.Sprintf("graphql playground: naive list: %v", err))
	}
	ids := make([]string, 0, len(naivePage.Items))
	for _, post := range naivePage.Items {
		ids = append(ids, post.ID)
	}
	for _, id := range ids {
		if _, err := naiveRepo.GetByID(context.Background(), id); err != nil {
			panic(fmt.Sprintf("graphql playground: naive get %q: %v", id, err))
		}
	}
	naiveCounts := naiveRepo.Snapshot()

	// Loader: the same ids, each requested twice, all concurrently — the
	// way sibling field resolvers would.
	loaderRepo := loader.NewCountingRepository(newScratch())
	loads := make([]string, 0, len(ids)*2)
	loads = append(loads, ids...)
	loads = append(loads, ids...)
	// Closed batch: this payload is pinned byte-identical across builds,
	// so the batch boundary is the COUNT (all 2N loads), not a coalescing
	// window that machine load could close mid-flight and split — the 2N
	// loads cross the port exactly once, every build, however the test
	// runner schedules them.
	batchLoader := loader.NewPostsLoader(
		loader.PortBatch(loaderRepo),
		loader.WithClosedBatch(len(loads)),
	)
	resolved := make([]bool, len(loads))
	var wg sync.WaitGroup
	for i, id := range loads {
		wg.Add(1)
		go func(i int, id string) {
			defer wg.Done()
			post, loadErr := batchLoader.Load(context.Background(), id)
			resolved[i] = loadErr == nil && post.ID == id
		}(i, id)
	}
	wg.Wait()
	loaderCounts := loaderRepo.Snapshot()

	allResolved := true
	for _, ok := range resolved {
		allResolved = allResolved && ok
	}
	duplicates := len(loads) - len(ids)
	return GraphQLLoaderExperiment{
		PostCount:       len(ids),
		LoadsIssued:     len(loads),
		DuplicateLoads:  duplicates,
		UniqueIDs:       len(ids),
		NaivePortCalls:  naiveCounts.Total(),
		NaiveBreakdown:  naiveCounts,
		LoaderPortCalls: loaderCounts.Total(),
		LoaderBreakdown: loaderCounts,
		AllResolved:     allResolved,
		Verdict: fmt.Sprintf(
			"%d concurrent loads (%d duplicates) crossed the port ONCE, batched — against %d port calls for the naive one-read-per-item hydration; behind a database every port call is a round trip",
			len(loads), duplicates, naiveCounts.Total(),
		),
	}
}
