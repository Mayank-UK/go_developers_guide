package postsdata

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"guide/internal/core"
	"guide/internal/domain/posts"
)

// The cache-aside decorator's behavior table. Everything runs against the
// real in-memory repository as the wrapped store (a counting recorder in
// front of it witnesses every delegation), with a stepping clock so TTL
// expiry is asserted, never slept. The concurrency cases run under -race in
// the standard suite.

// cachingStepClock is a fixed clock the test advances by hand: determinism
// where wall-clock sleep would only prove patience.
type cachingStepClock struct {
	mu sync.Mutex
	t  time.Time
}

func newCachingStepClock() *cachingStepClock {
	return &cachingStepClock{t: time.Date(2026, 10, 21, 9, 0, 0, 0, time.UTC)}
}

func (c *cachingStepClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *cachingStepClock) Advance(d time.Duration) time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = c.t.Add(d)
	return c.t
}

// cachingCountingRepo witnesses reads delegated to the wrapped store — the
// store-side truth the decorator's own scoreboard is checked against.
type cachingCountingRepo struct {
	posts.PostsRepository
	reads atomic.Int64
}

func (c *cachingCountingRepo) GetByID(ctx context.Context, id string) (posts.Post, error) {
	c.reads.Add(1)
	return c.PostsRepository.GetByID(ctx, id)
}

func (c *cachingCountingRepo) List(ctx context.Context, query posts.ListQuery) (posts.PostPage, error) {
	c.reads.Add(1)
	return c.PostsRepository.List(ctx, query)
}

// newCachingLabStore builds the standard fixture: the seeded in-memory
// repository on the stepping clock, wrapped by the counting witness.
func newCachingLabStore(clock *cachingStepClock) *cachingCountingRepo {
	return &cachingCountingRepo{PostsRepository: newRepository(clock.Now)}
}

func TestCachingFirstReadMissesThenSecondHits(t *testing.T) {
	clock := newCachingStepClock()
	store := newCachingLabStore(clock)
	cache := NewCachingRepository(store, 5*time.Second, nil, clock.Now)

	first, err := cache.GetByID(context.Background(), "p-1")
	if err != nil {
		t.Fatalf("first read: %v", err)
	}
	if store.reads.Load() != 1 {
		t.Errorf("first read must delegate, store reads = %d", store.reads.Load())
	}

	second, err := cache.GetByID(context.Background(), "p-1")
	if err != nil {
		t.Fatalf("second read: %v", err)
	}
	if second.Title != first.Title {
		t.Errorf("hit drifted from the miss: %q vs %q", second.Title, first.Title)
	}
	if store.reads.Load() != 1 {
		t.Errorf("second read must be a hit, store reads = %d", store.reads.Load())
	}
	stats := cache.Stats()
	if stats.Hits != 1 || stats.Misses != 1 || stats.Underlying != 1 {
		t.Errorf("stats = %+v, want hits 1 / misses 1 / underlying 1", stats)
	}

	// Handed-out values are copies: mutating the returned post's tags cannot
	// corrupt the cached entry behind the next hit.
	second.Tags[0] = "mutated-by-caller"
	third, err := cache.GetByID(context.Background(), "p-1")
	if err != nil {
		t.Fatalf("third read: %v", err)
	}
	for _, tag := range third.Tags {
		if tag == "mutated-by-caller" {
			t.Error("caller mutation leaked into the cached entry")
		}
	}
}

func TestCachingListPagesCacheIndependentlyPerQuery(t *testing.T) {
	clock := newCachingStepClock()
	store := newCachingLabStore(clock)
	cache := NewCachingRepository(store, 5*time.Second, nil, clock.Now)
	ctx := context.Background()

	newest := posts.ListQuery{Page: 1, Limit: 3, Sort: posts.SortNewest}
	titled := posts.ListQuery{Page: 1, Limit: 3, Sort: posts.SortTitle}
	filtered := posts.ListQuery{Page: 1, Limit: 3, Tag: "go", Sort: posts.SortNewest}

	for _, query := range []posts.ListQuery{newest, titled, filtered} {
		if _, err := cache.List(ctx, query); err != nil {
			t.Fatalf("first list %+v: %v", query, err)
		}
		if _, err := cache.List(ctx, query); err != nil {
			t.Fatalf("second list %+v: %v", query, err)
		}
	}
	if got := store.reads.Load(); got != 3 {
		t.Errorf("store reads = %d, want 3 (one per distinct query)", got)
	}
	if stats := cache.Stats(); stats.Hits != 3 || stats.Misses != 3 {
		t.Errorf("stats = %+v, want hits 3 / misses 3", stats)
	}

	// A page result handed out is a copy: mutating its items cannot poison
	// the cached page the next hit serves.
	page, err := cache.List(ctx, newest)
	if err != nil {
		t.Fatalf("hit list: %v", err)
	}
	page.Items[0].Title = "mutated-by-caller"
	again, err := cache.List(ctx, newest)
	if err != nil {
		t.Fatalf("second hit list: %v", err)
	}
	if again.Items[0].Title == "mutated-by-caller" {
		t.Error("caller mutation leaked into the cached page")
	}
}

func TestCachingWritesInvalidateTheIdAndEveryPage(t *testing.T) {
	clock := newCachingStepClock()
	store := newCachingLabStore(clock)
	cache := NewCachingRepository(store, 5*time.Second, nil, clock.Now)
	ctx := context.Background()

	query := posts.ListQuery{Page: 1, Limit: 10, Sort: posts.SortNewest}
	if _, err := cache.GetByID(ctx, "p-1"); err != nil {
		t.Fatalf("warm the id: %v", err)
	}
	if _, err := cache.List(ctx, query); err != nil {
		t.Fatalf("warm the page: %v", err)
	}

	edited := posts.Draft{Title: "Errors are values (edited)", Body: "replaced", Tags: []string{"go"}}
	if _, err := cache.Replace(ctx, "p-1", edited); err != nil {
		t.Fatalf("replace: %v", err)
	}

	after, err := cache.GetByID(ctx, "p-1")
	if err != nil {
		t.Fatalf("read after replace: %v", err)
	}
	if after.Title != edited.Title {
		t.Errorf("stale hit after a write: %q, want %q", after.Title, edited.Title)
	}
	page, err := cache.List(ctx, query)
	if err != nil {
		t.Fatalf("list after replace: %v", err)
	}
	found := false
	for _, post := range page.Items {
		if post.ID == "p-1" && post.Title == edited.Title {
			found = true
		}
	}
	if !found {
		t.Error("cached page survived the write that changed its ordering input")
	}
	stats := cache.Stats()
	if stats.Invalidations < 2 {
		t.Errorf("invalidations = %d, want at least 2 (the id entry and the page)", stats.Invalidations)
	}
}

func TestCachingDeleteSurfacesTypedNotFoundAndCreateRefills(t *testing.T) {
	clock := newCachingStepClock()
	store := newCachingLabStore(clock)
	cache := NewCachingRepository(store, 5*time.Second, nil, clock.Now)
	ctx := context.Background()

	if _, err := cache.GetByID(ctx, "p-3"); err != nil {
		t.Fatalf("warm: %v", err)
	}
	if err := cache.Delete(ctx, "p-3"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	_, err := cache.GetByID(ctx, "p-3")
	if !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("deleted post must surface the typed miss, got %v", err)
	}
	// The miss is never cached: a later Create of the same shape answers live.
	created, err := cache.Create(ctx, posts.Draft{Title: "A fresh post", Body: "new", Tags: []string{"go"}})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	stored, err := cache.GetByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("read the created post: %v", err)
	}
	if stored.Title != "A fresh post" {
		t.Errorf("created post read back as %q", stored.Title)
	}
}

func TestCachingTTLExpiryTurnsHitsBackIntoMisses(t *testing.T) {
	clock := newCachingStepClock()
	store := newCachingLabStore(clock)
	cache := NewCachingRepository(store, 5*time.Second, nil, clock.Now)
	ctx := context.Background()

	if _, err := cache.GetByID(ctx, "p-1"); err != nil {
		t.Fatalf("warm: %v", err)
	}
	clock.Advance(5*time.Second + time.Nanosecond)
	again, err := cache.GetByID(ctx, "p-1")
	if err != nil {
		t.Fatalf("read after the deadline: %v", err)
	}
	if again.Title != "Why Go for backends" {
		t.Errorf("expired refill drifted: %q", again.Title)
	}
	if got := store.reads.Load(); got != 2 {
		t.Errorf("store reads = %d, want 2 (miss, expiry miss)", got)
	}
	if stats := cache.Stats(); stats.Expiries != 1 {
		t.Errorf("expiries = %d, want 1", stats.Expiries)
	}

	// The refill re-arms the TTL on the stepped clock.
	clock.Advance(2 * time.Second)
	if _, err := cache.GetByID(ctx, "p-1"); err != nil {
		t.Fatalf("hit inside the new window: %v", err)
	}
	if got := store.reads.Load(); got != 2 {
		t.Errorf("store reads inside the new window = %d, want 2", got)
	}
}

func TestCachingPartitionSaltKeepsTwoStoresFromSharingEntries(t *testing.T) {
	clock := newCachingStepClock()
	scenario := core.NewScenarioStore()
	fakeStore := newRepository(clock.Now)
	liveStore := newRepository(clock.Now)
	switching := NewSwitchingRepository(scenario, fakeStore, liveStore)
	mode := func() string { return string(scenario.Snapshot().DataMode) }
	cache := NewCachingRepository(switching, 5*time.Second, mode, clock.Now)
	ctx := context.Background()

	// The two stores answer differently for the same id: give live's p-1 its
	// own title before any caching read happens.
	if _, err := liveStore.Replace(ctx, "p-1", posts.Draft{
		Title: "Errors are values (live store)", Body: "distinct", Tags: []string{"go"},
	}); err != nil {
		t.Fatalf("diverge live's p-1: %v", err)
	}

	fromFake, err := cache.GetByID(ctx, "p-1")
	if err != nil {
		t.Fatalf("fake read: %v", err)
	}
	live := core.DataModeLive
	if _, err := scenario.Update(core.ScenarioPatch{DataMode: &live}); err != nil {
		t.Fatalf("flip live: %v", err)
	}
	fromLive, err := cache.GetByID(ctx, "p-1")
	if err != nil {
		t.Fatalf("live read: %v", err)
	}
	if fromFake.Title == fromLive.Title {
		t.Fatalf("partition salt failed: both modes served %q", fromFake.Title)
	}
	if fromLive.Title != "Errors are values (live store)" {
		t.Errorf("live read = %q", fromLive.Title)
	}

	// Flip back: the fake partition's own entry is still valid (no write
	// happened through the decorator) and serves the FAKE title — one
	// store's rows never answer for the other.
	fake := core.DataModeFake
	if _, err := scenario.Update(core.ScenarioPatch{DataMode: &fake}); err != nil {
		t.Fatalf("flip fake: %v", err)
	}
	back, err := cache.GetByID(ctx, "p-1")
	if err != nil {
		t.Fatalf("fake read again: %v", err)
	}
	if back.Title != fromFake.Title {
		t.Errorf("flip back served %q, want the fake store's %q", back.Title, fromFake.Title)
	}
}

func TestCachingWriteInOnePartitionRetiresTheOtherPartitionsStaleEntry(t *testing.T) {
	clock := newCachingStepClock()
	scenario := core.NewScenarioStore()
	fakeStore := newRepository(clock.Now)
	liveStore := newRepository(clock.Now)
	switching := NewSwitchingRepository(scenario, fakeStore, liveStore)
	mode := func() string { return string(scenario.Snapshot().DataMode) }
	cache := NewCachingRepository(switching, 5*time.Second, mode, clock.Now)
	ctx := context.Background()

	if _, err := cache.GetByID(ctx, "p-1"); err != nil { // warms the FAKE partition
		t.Fatalf("warm fake: %v", err)
	}
	// Diverge the fake store CACHE-BLINDLY (an out-of-band write, the way a
	// psql session would): if the warm entry is wrongly served, the read
	// answers the OLD title; a correct retire-and-refill answers this one.
	if _, err := fakeStore.Replace(ctx, "p-1", posts.Draft{
		Title: "Why Go for backends (edited out of band)", Body: "edited", Tags: []string{"go"},
	}); err != nil {
		t.Fatalf("out-of-band fake edit: %v", err)
	}
	live := core.DataModeLive
	if _, err := scenario.Update(core.ScenarioPatch{DataMode: &live}); err != nil {
		t.Fatalf("flip live: %v", err)
	}
	// A write through the decorator in the live partition bumps the GLOBAL
	// epoch: the fake entry cannot survive it, because the decorator cannot
	// prove a fake read is unaffected by a write it never sees the
	// store-side effects of.
	if _, err := cache.Replace(ctx, "p-1", posts.Draft{
		Title: "Why Go for backends (live edit)", Body: "edited live", Tags: []string{"go"},
	}); err != nil {
		t.Fatalf("live replace: %v", err)
	}
	fake := core.DataModeFake
	if _, err := scenario.Update(core.ScenarioPatch{DataMode: &fake}); err != nil {
		t.Fatalf("flip fake: %v", err)
	}
	after, err := cache.GetByID(ctx, "p-1")
	if err != nil {
		t.Fatalf("fake read after the live write: %v", err)
	}
	if after.Title != "Why Go for backends (edited out of band)" {
		t.Errorf("stale entry survived a write in the other partition: %q", after.Title)
	}
	if stats := cache.Stats(); stats.Invalidations == 0 {
		t.Error("the retired cross-partition entry must count as an invalidation")
	}
}

func TestCachingZeroTTLIsAPassthrough(t *testing.T) {
	clock := newCachingStepClock()
	store := newCachingLabStore(clock)
	cache := NewCachingRepository(store, 0, nil, clock.Now)
	ctx := context.Background()

	for range 3 {
		if _, err := cache.GetByID(ctx, "p-1"); err != nil {
			t.Fatalf("read: %v", err)
		}
	}
	if got := store.reads.Load(); got != 3 {
		t.Errorf("store reads = %d, want 3 (nothing stored at ttl 0)", got)
	}
	if cache.Entries() != 0 {
		t.Errorf("entries = %d, want 0", cache.Entries())
	}
}

// gatedStore hands the coalescing tests a store whose GetByID blocks until
// the test releases it — the deterministic way to hold a refill in flight
// while other readers pile onto the same key.
type gatedStore struct {
	posts.PostsRepository
	gate    chan struct{}
	spyRead *atomic.Int64
}

func (g *gatedStore) GetByID(ctx context.Context, id string) (posts.Post, error) {
	g.spyRead.Add(1)
	select {
	case <-g.gate:
		return g.PostsRepository.GetByID(ctx, id)
	case <-ctx.Done():
		return posts.Post{}, ctx.Err()
	}
}

func TestCachingConcurrentMissesCoalesceIntoOneStoreRead(t *testing.T) {
	clock := newCachingStepClock()
	inner := newRepository(clock.Now)
	spy := &atomic.Int64{}
	gated := &gatedStore{PostsRepository: inner, gate: make(chan struct{}), spyRead: spy}
	cache := NewCachingRepository(gated, 5*time.Second, nil, clock.Now)

	const readers = 8
	results := make([]posts.Post, readers)
	errs := make([]error, readers)
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := range readers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			results[i], errs[i] = cache.GetByID(context.Background(), "p-2")
		}()
	}
	close(start)

	// Every reader is either parked on the fill slot or about to be; wait
	// until the coalesced counter stops moving before letting the store go.
	deadline := time.Now().Add(5 * time.Second)
	for {
		if cache.Stats().Coalesced == readers-1 || time.Now().After(deadline) {
			break
		}
		time.Sleep(time.Millisecond)
	}
	close(gated.gate)
	wg.Wait()

	if got := spy.Load(); got != 1 {
		t.Errorf("store reads = %d, want exactly 1 for %d concurrent readers", got, readers)
	}
	for i := range readers {
		if errs[i] != nil {
			t.Fatalf("reader %d failed: %v", i, errs[i])
		}
		if results[i].Title != results[0].Title {
			t.Errorf("reader %d got %q, reader 0 got %q", i, results[i].Title, results[0].Title)
		}
	}
	if stats := cache.Stats(); stats.Coalesced != readers-1 {
		t.Errorf("coalesced = %d, want %d (deadline may have clipped the herd)", stats.Coalesced, readers-1)
	}
}

// snapshotStore freezes its GetByID answer at call time and blocks before
// returning it — the deterministic way to prove an in-flight refill that
// straddled a write never caches its pre-write snapshot.
type snapshotStore struct {
	posts.PostsRepository
	gate    chan struct{}
	spyRead *atomic.Int64
}

func (g *snapshotStore) GetByID(ctx context.Context, id string) (posts.Post, error) {
	post, err := g.PostsRepository.GetByID(ctx, id) // snapshot at call time
	g.spyRead.Add(1)
	select {
	case <-g.gate:
		return post, err
	case <-ctx.Done():
		return posts.Post{}, ctx.Err()
	}
}

func TestCachingRefillStraddlingAWriteIsNotCached(t *testing.T) {
	clock := newCachingStepClock()
	inner := newRepository(clock.Now)
	spy := &atomic.Int64{}
	snapshot := &snapshotStore{PostsRepository: inner, gate: make(chan struct{}), spyRead: spy}
	cache := NewCachingRepository(snapshot, 5*time.Second, nil, clock.Now)
	ctx := context.Background()

	type outcome struct {
		post posts.Post
		err  error
	}
	refill := make(chan outcome, 1)
	go func() {
		post, err := cache.GetByID(ctx, "p-1") // snapshots the pre-write title
		refill <- outcome{post: post, err: err}
	}()

	// Wait until the refill is provably in flight, then write through the
	// decorator: the epoch moves while the refill holds its stale snapshot.
	deadline := time.Now().Add(5 * time.Second)
	for snapshot.spyRead.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if _, err := cache.Replace(ctx, "p-1", posts.Draft{
		Title: "Errors are values (edited mid-flight)", Body: "edited", Tags: []string{"go"},
	}); err != nil {
		t.Fatalf("replace under the in-flight refill: %v", err)
	}
	close(snapshot.gate)

	first := <-refill
	if first.err != nil {
		t.Fatalf("in-flight refill failed: %v", first.err)
	}
	if first.post.Title == "Errors are values (edited mid-flight)" {
		t.Fatal("the refill's snapshot was taken AFTER the write — test setup is wrong")
	}

	// The straddled refill must not have cached: the next read goes back to
	// the store and sees the post-write truth.
	after, err := cache.GetByID(ctx, "p-1")
	if err != nil {
		t.Fatalf("read after the straddle: %v", err)
	}
	if after.Title != "Errors are values (edited mid-flight)" {
		t.Errorf("stale snapshot was cached across the write: %q", after.Title)
	}
}

func TestCachingHammerAcrossMixedTrafficIsRaceClean(t *testing.T) {
	clock := newCachingStepClock()
	store := newCachingLabStore(clock)
	cache := NewCachingRepository(store, 5*time.Second, nil, clock.Now)
	ctx := context.Background()

	var wg sync.WaitGroup
	for worker := range 6 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range 40 {
				switch (worker + i) % 4 {
				case 0:
					_, _ = cache.GetByID(ctx, "p-1")
				case 1:
					_, _ = cache.List(ctx, posts.ListQuery{Page: 1 + i%3, Limit: 3, Sort: posts.SortNewest})
				case 2:
					_, _ = cache.Replace(ctx, "p-2", posts.Draft{Title: "hammered", Body: "x", Tags: []string{"go"}})
				default:
					_ = cache.Stats()
				}
			}
		}()
	}
	wg.Wait()

	// After the storm the cache must still be CORRECT: the next read of the
	// hammered post returns the last written state.
	final, err := cache.GetByID(ctx, "p-2")
	if err != nil {
		t.Fatalf("final read: %v", err)
	}
	if final.Title != "hammered" {
		t.Errorf("final read = %q, want the last written state", final.Title)
	}
}
