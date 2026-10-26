package graph

import (
	"context"

	"github.com/99designs/gqlgen/graphql"
	"github.com/99designs/gqlgen/graphql/executor"
)

// Request is one GraphQL operation ready to execute: the raw query text,
// an optional operation name (documents may declare several), and optional
// variables for the chosen operation's declarations. This is exactly the
// JSON body POST /api/graphql accepts — the endpoint decodes into a
// Request, and the playground demo builds them by hand.
type Request struct {
	Query         string
	OperationName string
	Variables     map[string]any
}

// Execute runs one operation against the schema and returns the response
// the wire would carry — the {data, errors} document — with no HTTP in
// the way. This is the single execution path BOTH transports share: the
// playground demo executes sample operations through it in-process, and
// POST /api/graphql serves its result verbatim. Parse and validation
// failures arrive as the response's errors list with data null; resolver
// failures ride the same list after execution — GraphQL has one shape for
// every failure, and it is not a status code.
func Execute(ctx context.Context, schema graphql.ExecutableSchema, request Request) *graphql.Response {
	// The executor's stats want a trace start in the context (the library's
	// HTTP server seeds it per request); ours is per operation.
	ctx = graphql.StartOperationTrace(ctx)
	ex := executor.New(schema)
	opCtx, list := ex.CreateOperationContext(ctx, &graphql.RawParams{
		Query:         request.Query,
		OperationName: request.OperationName,
		Variables:     request.Variables,
	})
	if len(list) != 0 {
		return ex.DispatchError(ctx, list)
	}
	// The bare executor ships with introspection DISABLED — the safe
	// default for a graph exposed to strangers. This graph teaches
	// introspection, so the surface opts in deliberately, per operation.
	opCtx.DisableIntrospection = false

	handler, execCtx := ex.DispatchOperation(ctx, opCtx)
	response := handler(execCtx)
	if response == nil {
		// Unreachable for queries and mutations (they yield exactly one
		// response), but a nil would silently produce an empty body —
		// better to answer a shaped error than nothing.
		response = graphql.ErrorResponse(ctx, "operation produced no response")
	}
	return response
}
