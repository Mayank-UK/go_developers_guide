// The cache-aside decorator: a posts.PostsRepository that wraps ANY other
// repository (the switching facade in production, throwaway stores in the
// caching lab) and answers reads from a bounded in-memory cache with a TTL,
// refilling on miss and invalidating on every successful write. It is the
// topic 21 implementation quoted in the guide — real wiring, not an exhibit:
// cmd/guide/main.go wraps the production posts path with it.
package postsdata

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	"guide/internal/domain/posts"
)

// Compile-time proof the decorator satisfies the port it wraps.
var _ posts.PostsRepository = (*CachingRepository)(nil)

// Cache ceilings: the cache is memory, so it must be BOUNDED. maxPosts and
// maxPages cap each entry map; a store past the cap drops one arbitrary
// entry (map iteration order) before inserting — crude next to an LRU, but
// honest, O(1), and enough for a cache whose entries all die by TTL anyway.
const (
	maxPosts = 1024
	maxPages = 256
)

// CacheStats is the observable scoreboard: every counter is a fact about
// what the decorator did, reported as a snapshot copy under the lock.
type CacheStats struct {
	Hits          uint64 // reads served from a fresh entry
	Misses        uint64 // reads that led a refill (cold or expired)
	Expiries      uint64 // entries dropped because their TTL had passed
	Invalidations uint64 // entries dropped because a write superseded them
	Coalesced     uint64 // readers that waited on an in-flight refill instead of racing it
	Underlying    uint64 // reads delegated to the wrapped repository (leaders only)
}

// CachingRepository is the cache-aside decorator over the posts port. Reads
// consult the cache first; a miss delegates to the wrapped repository and
// stores the result with a TTL; every successful write invalidates the
// written id and ALL cached pages. Two staleness defenses sit under the same
// mutex: an epoch counter bumped by writes (an in-flight refill started
// before a write never stores its soon-to-be-stale value, and a hit whose
// entry predates the write is dropped) and a partition salt (a func() string
// folded into every key, so two different backing stores — the scenario
// switch's fake and live modes — can never read each other's entries). A
// per-key in-flight map coalesces concurrent misses into one underlying
// read: the hand-rolled singleflight pattern, without taking a dependency.
type CachingRepository struct {
	inner     posts.PostsRepository
	ttl       time.Duration
	partition func() string
	now       func() time.Time

	mu       sync.Mutex
	posts    map[string]cachedPost
	pages    map[string]cachedPage
	inFlight map[string]*fillSlot
	epoch    uint64
	stats    CacheStats
}

// cachedPost is one stored GetByID result.
type cachedPost struct {
	post      posts.Post
	expiresAt time.Time
	epoch     uint64
}

// cachedPage is one stored List result.
type cachedPage struct {
	page      posts.PostPage
	expiresAt time.Time
	epoch     uint64
}

// fillSlot marks one in-flight refill: the leader stores it under the lock,
// fetches WITHOUT the lock, then closes done so waiters re-check the cache.
type fillSlot struct {
	done chan struct{}
}

// NewCachingRepository wraps inner with the cache-aside decorator. The ttl
// bounds staleness for writes the process cannot see (a psql INSERT lands in
// PostgreSQL without passing through this decorator — the entry simply
// expires); a ttl of zero or less disables storage while keeping the method
// set, making the decorator a transparent pass-through. The partition
// function salts every key with the identity of the backing store — the
// composition root passes the scenario store's data mode, so a runtime
// fake/live flip starts on a cold keyspace instead of serving one store's
// rows to the other. Pass nil for a single-store cache.
func NewCachingRepository(inner posts.PostsRepository, ttl time.Duration, partition func() string, now func() time.Time) *CachingRepository {
	if partition == nil {
		partition = func() string { return "" }
	}
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	return &CachingRepository{
		inner:     inner,
		ttl:       ttl,
		partition: partition,
		now:       now,
		posts:     make(map[string]cachedPost),
		pages:     make(map[string]cachedPage),
		inFlight:  make(map[string]*fillSlot),
	}
}

// Stats returns a copy of the counters under the lock.
func (c *CachingRepository) Stats() CacheStats {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.stats
}

// Entries returns how many entries the maps hold right now — including
// expired-but-not-yet-touched ones, because eviction is lazy (on read) and
// bounded by the size caps. An honest number for a demo, not a promise.
func (c *CachingRepository) Entries() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.posts) + len(c.pages)
}

// GetByID implements the port: warm hit, coalesced miss, or leader refill.
// Errors pass through untouched — the typed core.NotFound of a real miss is
// the caller's to map, and negative results are deliberately never cached.
func (c *CachingRepository) GetByID(ctx context.Context, id string) (posts.Post, error) {
	key := c.postKey(id)
	for round := 0; round < 3; round++ {
		c.mu.Lock()
		if entry, ok := c.posts[key]; ok {
			if c.fresh(entry.expiresAt, entry.epoch) {
				c.stats.Hits++
				c.mu.Unlock()
				return copyPost(entry.post), nil
			}
			c.dropStale(key, entry.expiresAt)
		}
		if pending, joined := c.joinFill(key); joined {
			c.mu.Unlock()
			if err := waitFill(ctx, pending); err != nil {
				return posts.Post{}, err
			}
			continue // the leader finished; re-check the cache from the top
		}
		lead := c.leadFill(key)
		c.mu.Unlock()

		post, err := c.inner.GetByID(ctx, id)

		c.mu.Lock()
		c.settleFill(key)
		if err == nil && c.ttl > 0 && c.epoch == lead.epoch {
			c.storePostLocked(key, post)
		}
		c.mu.Unlock()
		close(lead.slot.done)
		return post, err
	}
	// Three rounds of churn (writes and refills kept racing): fall back to
	// one uncached read — slower, never wrong, and honestly counted.
	c.countPassthrough()
	return c.inner.GetByID(ctx, id)
}

// List implements the port with the same engine over the page cache. Page
// keys are the canonical query, so every distinct (page, limit, tag, sort)
// combination caches independently — and every write flushes them all.
func (c *CachingRepository) List(ctx context.Context, query posts.ListQuery) (posts.PostPage, error) {
	key := c.pageKey(query)
	for round := 0; round < 3; round++ {
		c.mu.Lock()
		if entry, ok := c.pages[key]; ok {
			if c.fresh(entry.expiresAt, entry.epoch) {
				c.stats.Hits++
				c.mu.Unlock()
				return copyPage(entry.page), nil
			}
			c.dropStale(key, entry.expiresAt)
		}
		if pending, joined := c.joinFill(key); joined {
			c.mu.Unlock()
			if err := waitFill(ctx, pending); err != nil {
				return posts.PostPage{}, err
			}
			continue
		}
		lead := c.leadFill(key)
		c.mu.Unlock()

		page, err := c.inner.List(ctx, query)

		c.mu.Lock()
		c.settleFill(key)
		if err == nil && c.ttl > 0 && c.epoch == lead.epoch {
			c.storePageLocked(key, page)
		}
		c.mu.Unlock()
		close(lead.slot.done)
		return page, err
	}
	c.countPassthrough()
	return c.inner.List(ctx, query)
}

// Create implements the port: delegate, then invalidate on success. The new
// id's key is dropped (a store may hand out an id a reader already missed
// on) and every cached page goes — membership, totals, and ordering all
// moved. A failed create changes nothing and flushes nothing.
func (c *CachingRepository) Create(ctx context.Context, draft posts.Draft) (posts.Post, error) {
	post, err := c.inner.Create(ctx, draft)
	if err == nil {
		c.invalidateWrite(post.ID)
	}
	return post, err
}

// Replace implements the port: delegate, then invalidate on success — the
// replaced id and every page that might order or count it.
func (c *CachingRepository) Replace(ctx context.Context, id string, draft posts.Draft) (posts.Post, error) {
	post, err := c.inner.Replace(ctx, id, draft)
	if err == nil {
		c.invalidateWrite(post.ID)
	}
	return post, err
}

// Delete implements the port: delegate, then invalidate on success — a
// deleted post must not answer the next read from memory.
func (c *CachingRepository) Delete(ctx context.Context, id string) error {
	err := c.inner.Delete(ctx, id)
	if err == nil {
		c.invalidateWrite(id)
	}
	return err
}

// fresh reports whether an entry is still usable: inside its TTL and stored
// by a refill that started before the latest write.
func (c *CachingRepository) fresh(expiresAt time.Time, entryEpoch uint64) bool {
	return c.ttl > 0 && c.now().Before(expiresAt) && entryEpoch == c.epoch
}

// dropStale removes one unusable entry and counts WHY it left. Must be
// called with the lock held and only for entries that exist.
func (c *CachingRepository) dropStale(key string, expiresAt time.Time) {
	delete(c.posts, key)
	delete(c.pages, key)
	if c.now().Before(expiresAt) {
		c.stats.Invalidations++ // still fresh — a write's epoch moved past it
	} else {
		c.stats.Expiries++
	}
}

// joinFill registers a waiter on an in-flight refill of key, if one exists.
// Must be called with the lock held.
func (c *CachingRepository) joinFill(key string) (*fillSlot, bool) {
	if slot, ok := c.inFlight[key]; ok {
		c.stats.Coalesced++
		return slot, true
	}
	return nil, false
}

// leadFill registers THIS caller as the refill's leader and counts the
// underlying read it is about to make. Must be called with the lock held.
func (c *CachingRepository) leadFill(key string) *leadToken {
	slot := &fillSlot{done: make(chan struct{})}
	c.inFlight[key] = slot
	c.stats.Misses++
	c.stats.Underlying++
	return &leadToken{slot: slot, epoch: c.epoch}
}

// settleFill removes the leader's slot. Must be called with the lock held.
func (c *CachingRepository) settleFill(key string) {
	delete(c.inFlight, key)
}

// countPassthrough books the fallback read that bypassed the cache after
// three raced rounds: still a miss, still one underlying read — the
// scoreboard never lies to make the cache look better than it was.
func (c *CachingRepository) countPassthrough() {
	c.mu.Lock()
	c.stats.Misses++
	c.stats.Underlying++
	c.mu.Unlock()
}

// waitFill parks a waiter until the leader finishes or the caller's context
// gives up. The channel close is the memory barrier: everything the leader
// wrote before close is visible after the wait returns.
func waitFill(ctx context.Context, slot *fillSlot) error {
	select {
	case <-slot.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// leadToken hands the leader the slot it owns and the epoch its refill
// started under, so the store step can refuse to cache across a write.
type leadToken struct {
	slot  *fillSlot
	epoch uint64
}

// storePostLocked stores a refill result under the size cap. Must be called
// with the lock held and only on the leader's success path.
func (c *CachingRepository) storePostLocked(key string, post posts.Post) {
	if len(c.posts) >= maxPosts {
		for victim := range c.posts {
			delete(c.posts, victim)
			break
		}
	}
	c.posts[key] = cachedPost{post: copyPost(post), expiresAt: c.now().Add(c.ttl), epoch: c.epoch}
}

// storePageLocked stores a page refill under the size cap. Must be called
// with the lock held and only on the leader's success path.
func (c *CachingRepository) storePageLocked(key string, page posts.PostPage) {
	if len(c.pages) >= maxPages {
		for victim := range c.pages {
			delete(c.pages, victim)
			break
		}
	}
	c.pages[key] = cachedPage{page: copyPage(page), expiresAt: c.now().Add(c.ttl), epoch: c.epoch}
}

// invalidateWrite runs after every successful write: bump the epoch (every
// entry stored before this write fails its freshness check from now on,
// wherever it lives), drop the written id's entry, and clear every cached
// page. Counting the dropped entries keeps the scoreboard honest.
func (c *CachingRepository) invalidateWrite(id string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.epoch++
	dropped := len(c.pages)
	c.pages = make(map[string]cachedPage, maxPages)
	key := c.postKey(id)
	if _, ok := c.posts[key]; ok {
		delete(c.posts, key)
		dropped++
	}
	c.stats.Invalidations += uint64(dropped)
}

// postKey builds the GetByID cache key: kind, partition, and the id quoted —
// the quote makes key injection impossible no matter what an id contains.
func (c *CachingRepository) postKey(id string) string {
	return "get\x1f" + c.partition() + "\x1f" + strconv.Quote(id)
}

// pageKey builds the List cache key from the canonical query: distinct
// pages cache independently, and the quoting keeps tags from colliding keys.
func (c *CachingRepository) pageKey(query posts.ListQuery) string {
	return fmt.Sprintf("list\x1f%s\x1fpage=%d;limit=%d;tag=%q;sort=%q",
		c.partition(), query.Page, query.Limit, query.Tag, query.Sort)
}

// copyPage deep-copies a page so a caller mutating the handed-out slice can
// never corrupt the cached entry — the same discipline the in-memory
// repository applies to its store.
func copyPage(page posts.PostPage) posts.PostPage {
	copied := page
	copied.Items = make([]posts.Post, 0, len(page.Items))
	for _, post := range page.Items {
		copied.Items = append(copied.Items, copyPost(post))
	}
	return copied
}
