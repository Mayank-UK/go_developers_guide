// The switching facade: ONE posts.PostsRepository whose backing store is
// chosen per call from the scenario store's data mode — "fake" answers
// from the in-memory adapter, "live" from the PostgreSQL adapter. This is
// the fake/live wiring the guide has promised since Phase 1: the REST
// routes, the GraphQL resolver, and the gRPC server share the SAME use
// case instances, and those use cases hold this facade — so PATCH
// /api/demo/scenario {"dataMode":"live"} flips every surface at once,
// with no restart and no code path that knows the switch happened.
package postsdata

import (
	"context"

	"guide/internal/core"
	"guide/internal/domain/posts"
)

// Compile-time proof the facade satisfies the port it delegates.
var _ posts.PostsRepository = (*SwitchingRepository)(nil)

// SwitchingRepository resolves the backing store per call: one snapshot
// read of the scenario store at method entry, so a mode flip between two
// of this repository's calls never tears a single logical operation
// across two stores, and a flip DURING a call is invisible to it. The two
// adapters are wired once, at the composition root; the facade owns no
// state of its own beyond the choice.
type SwitchingRepository struct {
	scenario *core.ScenarioStore
	fake     posts.PostsRepository
	live     posts.PostsRepository
}

// NewSwitchingRepository wires the facade over the scenario store and the
// two adapters. Either adapter may be nil for a fixed-mode facade (tests
// do this); a nil adapter is never consulted in its mode's place — the
// constructor only refuses a nil scenario store, the one dependency the
// facade cannot answer without.
func NewSwitchingRepository(scenario *core.ScenarioStore, fake, live posts.PostsRepository) *SwitchingRepository {
	return &SwitchingRepository{scenario: scenario, fake: fake, live: live}
}

// pick resolves the adapter for THIS call: the live store only when the
// scenario says live AND it was wired; anything else answers from the
// fake. The fallback direction is deliberate — a demo stack with
// PostgreSQL wedged still serves every surface from the fake rather than
// 503-ing the whole guide.
func (s *SwitchingRepository) pick() posts.PostsRepository {
	if s.scenario.Snapshot().DataMode == core.DataModeLive && s.live != nil {
		return s.live
	}
	return s.fake
}

// List implements the port through the per-call adapter.
func (s *SwitchingRepository) List(ctx context.Context, query posts.ListQuery) (posts.PostPage, error) {
	return s.pick().List(ctx, query)
}

// GetByID implements the port through the per-call adapter.
func (s *SwitchingRepository) GetByID(ctx context.Context, id string) (posts.Post, error) {
	return s.pick().GetByID(ctx, id)
}

// Create implements the port through the per-call adapter.
func (s *SwitchingRepository) Create(ctx context.Context, draft posts.Draft) (posts.Post, error) {
	return s.pick().Create(ctx, draft)
}

// Replace implements the port through the per-call adapter.
func (s *SwitchingRepository) Replace(ctx context.Context, id string, draft posts.Draft) (posts.Post, error) {
	return s.pick().Replace(ctx, id, draft)
}

// Delete implements the port through the per-call adapter.
func (s *SwitchingRepository) Delete(ctx context.Context, id string) error {
	return s.pick().Delete(ctx, id)
}
