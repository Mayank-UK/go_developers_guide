// Package graph holds the GraphQL surface's hand-written halves beside the
// codegen output (generated/, model/, resolver/): the error mapping from
// the app's typed taxonomy into GraphQL errors with extensions codes, and
// the in-process execution helper the playground demo and the HTTP
// endpoint share. The generated executor never imports this package — it
// sits above it, and the resolvers call in.
package graph

import (
	"errors"

	"github.com/vektah/gqlparser/v2/gqlerror"

	"guide/internal/core"
)

// MapError translates any error a resolver returns into a GraphQL error
// whose extensions carry the taxonomy: code is the SAME closed vocabulary
// the REST error envelope serves (not_found, validation_failed, and the
// six other buckets), so one client-side switch handles both surfaces.
// Typed *core.Error values keep their message and their structured
// details (the validation issue list rides extensions.details verbatim);
// anything untyped degrades to the internal bucket — never leaking an
// implementation detail onto the graph.
func MapError(err error) *gqlerror.Error {
	var typed *core.Error
	if !errors.As(err, &typed) {
		return &gqlerror.Error{
			Message:    "Internal error.",
			Extensions: map[string]any{"code": string(core.KindInternal)},
		}
	}
	extensions := map[string]any{"code": string(typed.Kind)}
	if typed.Details != nil {
		extensions["details"] = typed.Details
	}
	return &gqlerror.Error{
		Message:    typed.Message,
		Extensions: extensions,
	}
}
