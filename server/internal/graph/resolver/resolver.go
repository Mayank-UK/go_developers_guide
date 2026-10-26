// Package resolver holds the GraphQL surface's hand-written root: the
// dependency object every generated resolver method hangs off. gqlgen
// generates the stubs beside this file (posts.resolvers.go) and never
// overwrites what humans implement there; this file is the part people
// own — the use cases the graph delegates to, injected at the composition
// root exactly like an HTTP handler's dependencies.
package resolver

import (
	"guide/internal/domain/posts"
)

// Resolver is the root dependency object. It holds USE CASES, not storage:
// the graph is one more presentation of the same domain the REST handlers
// serve, so it asks the domain's verbs and never the repository — the
// dependency rule (presentation → domain ← data) holds graph-side too.
type Resolver struct {
	list    *posts.ListPostsUseCase
	get     *posts.GetPostUseCase
	create  *posts.CreatePostUseCase
	replace *posts.ReplacePostUseCase
	delete  *posts.DeletePostUseCase
}

// NewResolver wires the five posts use cases the schema's root fields
// delegate to. Pass the SAME instances the REST handlers share and one
// store answers both surfaces; pass fresh ones over a scratch repository
// and the graph owns a private world (how the playground demo executes
// mutations without touching the live /api/posts substrate).
func NewResolver(
	list *posts.ListPostsUseCase,
	get *posts.GetPostUseCase,
	create *posts.CreatePostUseCase,
	replace *posts.ReplacePostUseCase,
	delete *posts.DeletePostUseCase,
) *Resolver {
	return &Resolver{
		list:    list,
		get:     get,
		create:  create,
		replace: replace,
		delete:  delete,
	}
}
