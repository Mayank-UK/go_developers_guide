package loader

import (
	"context"
	"sync"

	"guide/internal/domain/posts"
)

// PortCounts is the instrument's reading: how many calls hit each port
// method. Total is the number that matters for the N+1 story — every call
// is a round trip once a database sits behind the port.
type PortCounts struct {
	List    int `json:"list"`
	GetByID int `json:"getById"`
	Create  int `json:"create"`
	Replace int `json:"replace"`
	Delete  int `json:"delete"`
}

// Total sums the counters — the port-call count the experiment reports.
func (p PortCounts) Total() int {
	return p.List + p.GetByID + p.Create + p.Replace + p.Delete
}

// CountingRepository wraps any PostsRepository and counts every port call
// that crosses it — the instrument that turns "is this an N+1?" from vibes
// into data. Wrap the real adapter, run the workload, read the snapshot:
// the naive pattern shows one List plus one GetByID per item (N+1 calls);
// the loader path shows one List for the batch, full stop.
type CountingRepository struct {
	inner posts.PostsRepository

	mu     sync.Mutex
	counts PortCounts
}

// Compile-time proof the wrapper still satisfies the port it wraps — the
// whole point: instrumentation you can inject anywhere the port fits.
var _ posts.PostsRepository = (*CountingRepository)(nil)

// NewCountingRepository wraps a repository with call counting.
func NewCountingRepository(inner posts.PostsRepository) *CountingRepository {
	return &CountingRepository{inner: inner}
}

// Snapshot returns the counters' current reading, race-safe.
func (c *CountingRepository) Snapshot() PortCounts {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.counts
}

// List counts and delegates.
func (c *CountingRepository) List(ctx context.Context, query posts.ListQuery) (posts.PostPage, error) {
	c.bump(func(counts *PortCounts) { counts.List++ })
	return c.inner.List(ctx, query)
}

// GetByID counts and delegates — the call the naive N+1 pattern repeats
// once per item.
func (c *CountingRepository) GetByID(ctx context.Context, id string) (posts.Post, error) {
	c.bump(func(counts *PortCounts) { counts.GetByID++ })
	return c.inner.GetByID(ctx, id)
}

// Create counts and delegates.
func (c *CountingRepository) Create(ctx context.Context, draft posts.Draft) (posts.Post, error) {
	c.bump(func(counts *PortCounts) { counts.Create++ })
	return c.inner.Create(ctx, draft)
}

// Replace counts and delegates.
func (c *CountingRepository) Replace(ctx context.Context, id string, draft posts.Draft) (posts.Post, error) {
	c.bump(func(counts *PortCounts) { counts.Replace++ })
	return c.inner.Replace(ctx, id, draft)
}

// Delete counts and delegates.
func (c *CountingRepository) Delete(ctx context.Context, id string) error {
	c.bump(func(counts *PortCounts) { counts.Delete++ })
	return c.inner.Delete(ctx, id)
}

// bump applies one counter mutation under the lock.
func (c *CountingRepository) bump(apply func(*PortCounts)) {
	c.mu.Lock()
	apply(&c.counts)
	c.mu.Unlock()
}
