// Demo API handlers for topic 13 graphql — TWO endpoints in one file: the
// topic's inline demo payload (GET /api/demo/api/graphql-playground) and
// the REAL GraphQL HTTP endpoint (POST /api/graphql) the demo's RUN
// button and the terminal companion drive. The endpoint is hand-written
// on purpose — parse the {query, variables?} JSON document, execute it
// through the same in-process path the demo uses, write the {data,
// errors} response — so the transport decisions it makes (which failures
// are 400s, which ride a 200) are code this guide can teach, not a
// library's default.
package presentation

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/99designs/gqlgen/graphql"

	"guide/internal/domain/demos"
	"guide/internal/domain/posts"
	"guide/internal/graph"
	"guide/internal/graph/generated"
	"guide/internal/graph/resolver"
)

// maxGraphQLBodyBytes bounds POST /api/graphql bodies: any query and
// variable set this schema deserves fits far under 128 KiB, so anything
// larger is broken or hostile — refusal, not memory pressure.
const maxGraphQLBodyBytes = 128 << 10

// Compile-time proofs that both handlers satisfy http.Handler.
var (
	_ http.Handler = (*GraphQLPlaygroundHandler)(nil)
	_ http.Handler = (*GraphQLHandler)(nil)
)

// GraphQLPlaygroundHandler serves GET /api/demo/api/graphql-playground,
// the graphql topic's inline demo payload: the executed sample runs, the
// schema card, the selectable queries, and the loader experiment's
// counts. The scratch repository factory is injected so the mutations the
// payload demonstrates land on a private world, never on the live
// /api/posts substrate.
type GraphQLPlaygroundHandler struct {
	newScratch func() posts.PostsRepository
}

// NewGraphQLPlaygroundHandler wires the playground payload endpoint. The
// composition root passes the in-memory adapter's constructor (one
// closure, because Go has no function covariance).
func NewGraphQLPlaygroundHandler(newScratch func() posts.PostsRepository) *GraphQLPlaygroundHandler {
	return &GraphQLPlaygroundHandler{newScratch: newScratch}
}

// ServeHTTP implements http.Handler for the playground payload.
func (h *GraphQLPlaygroundHandler) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, demos.BuildGraphQLPlaygroundPayload(h.newScratch))
}

// GraphQLHandler serves POST /api/graphql: the real endpoint over the
// generated executor. The handler owns exactly the HTTP-shaped decisions
// — bounded read, one JSON document, the 400-versus-200 line — and hands
// every operation to graph.Execute, the same in-process path the
// playground demo runs through.
type GraphQLHandler struct {
	schema graphql.ExecutableSchema
	logger *slog.Logger
}

// NewGraphQLHandler wires the endpoint over the posts use cases — the
// same instances the REST handlers share when the composition root passes
// the same constructors' products, so one store answers both surfaces.
func NewGraphQLHandler(
	list *posts.ListPostsUseCase,
	get *posts.GetPostUseCase,
	create *posts.CreatePostUseCase,
	replace *posts.ReplacePostUseCase,
	delete *posts.DeletePostUseCase,
	logger *slog.Logger,
) *GraphQLHandler {
	return &GraphQLHandler{
		schema: generated.NewExecutableSchema(generated.Config{
			Resolvers: resolver.NewResolver(list, get, create, replace, delete),
		}),
		logger: logger,
	}
}

// ServeHTTP implements http.Handler for POST /api/graphql.
//
// The status-line stance this endpoint teaches: transport-level failures
// (unreadable body, not one JSON object, no query) are HTTP failures and
// answer 400 with a GraphQL-shaped errors body; everything that happens
// DURING execution — unknown fields, rule violations, resolver misses —
// rides a 200 with the {data, errors} document, because GraphQL owns one
// status and spends it honestly.
func (h *GraphQLHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxGraphQLBodyBytes))
	if err != nil {
		h.reject(w, r, http.StatusBadRequest, "could not read request body")
		return
	}

	// The request body IS the library's own params shape: {query,
	// operationName?, variables?} decode straight into RawParams.
	var params graphql.RawParams
	if err := json.Unmarshal(body, &params); err != nil {
		h.reject(w, r, http.StatusBadRequest,
			`request body must be one JSON object: {"query":"…","variables":{…}}`)
		return
	}
	if strings.TrimSpace(params.Query) == "" {
		h.reject(w, r, http.StatusBadRequest, "query is required")
		return
	}
	params.Headers = r.Header

	response := graph.Execute(r.Context(), h.schema, graph.Request{
		Query:         params.Query,
		OperationName: params.OperationName,
		Variables:     params.Variables,
	})
	writeJSON(w, http.StatusOK, response)
}

// reject answers a transport-level failure: the truthful 400 plus the
// GraphQL error shape, so even a rejected request parses in every client.
func (h *GraphQLHandler) reject(w http.ResponseWriter, r *http.Request, status int, message string) {
	h.logger.Warn("graphql request rejected",
		"error", message, "method", r.Method, "path", r.URL.Path)
	writeJSON(w, status, graphql.ErrorResponse(r.Context(), "%s", message))
}
