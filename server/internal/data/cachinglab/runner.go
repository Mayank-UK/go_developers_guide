// The caching lab's choreography driver: the data side of the topic 21
// demo. It lives in its own package because domain/demos already imports
// data/posts (the gRPC console exhibit), so postsdata cannot import the
// demos types back — an adapter may compose another adapter, and this one
// composes the real pieces: two throwaway posts stores, a LOCAL scenario
// store + switching facade (the same fake/live machinery production uses,
// on disposable state), and the REAL CachingRepository. One Run walks the
// beats — cold read, warm hit, cached page, write invalidation, TTL expiry
// on a stepped clock, the twelve-reader herd, the store swap — and every
// number it reports is deterministic: fixed beats, fixed answers, so the
// payload (and the endpoint's ETag) is byte-stable.
package cachinglab

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"guide/internal/core"
	postsdata "guide/internal/data/posts"
	"guide/internal/domain/demos"
	"guide/internal/domain/posts"
)

// Compile-time proof the runner satisfies the domain's port.
var _ demos.CachingLabRunner = (*Runner)(nil)

// labClock is the fixture clock the DECORATOR reads: fixed at a pinned
// instant, advanced only by the choreography's explicit steps. The stores
// underneath may stamp wall-clock timestamps — no timestamp ever reaches
// the payload — but cache TTLs must expire deterministically, so the cache
// reads THIS clock.
type labClock struct {
	mu sync.Mutex
	t  time.Time
}

// newLabClock pins the lab's start instant.
func newLabClock() *labClock {
	return &labClock{t: time.Date(2026, 10, 21, 9, 0, 0, 0, time.UTC)}
}

// Now returns the fixture's current instant.
func (c *labClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

// Advance steps the fixture clock — the only way time moves in the lab.
func (c *labClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = c.t.Add(d)
}

// readCountingRepository is the store-side witness: it counts every read
// delegated to the wrapped repository, independently of the decorator's
// own scoreboard — two counters that must agree is the lab's honesty check.
type readCountingRepository struct {
	posts.PostsRepository
	reads atomic.Int64
}

// GetByID implements the port, counting the delegation.
func (r *readCountingRepository) GetByID(ctx context.Context, id string) (posts.Post, error) {
	r.reads.Add(1)
	return r.PostsRepository.GetByID(ctx, id)
}

// List implements the port, counting the delegation.
func (r *readCountingRepository) List(ctx context.Context, query posts.ListQuery) (posts.PostPage, error) {
	r.reads.Add(1)
	return r.PostsRepository.List(ctx, query)
}

// Runner drives the topic 21 demo against throwaway stores. It owns no
// state between calls: every Run starts from the same fixture, so two Runs
// answer identically — the premise the endpoint's ETag certifies.
type Runner struct{}

// New wires the runner (stateless by design).
func New() *Runner { return &Runner{} }

// Run performs the whole choreography and reports the raw haul.
func (r *Runner) Run(ctx context.Context) (demos.CachingLabRun, error) {
	clock := newLabClock()
	fakeStore := postsdata.NewInMemoryRepository()
	liveStore := postsdata.NewInMemoryRepository()
	if _, err := liveStore.Create(ctx, posts.Draft{
		Title: demos.CachingLabLiveOnlyTitle,
		Body:  "The live store keeps one post the fake never sees — the swap beat's wedge.",
		Tags:  []string{"caching", "live"},
	}); err != nil {
		return demos.CachingLabRun{}, fmt.Errorf("seed the live-only post: %w", err)
	}

	// The same fake/live machinery production uses — a scenario store and
	// the switching facade — on a LOCAL store pair the lab owns completely.
	scenario := core.NewScenarioStore()
	switching := postsdata.NewSwitchingRepository(scenario, fakeStore, liveStore)
	store := &readCountingRepository{PostsRepository: switching}
	partition := func() string { return string(scenario.Snapshot().DataMode) }
	cache := postsdata.NewCachingRepository(store, demos.CachingLabTTL, partition, clock.Now)

	var beats []demos.CachingLabBeat
	beat := func(before, after postsdata.CacheStats, readsBefore int64, id, title string, script []string, observed string) demos.CachingLabBeat {
		outcome := demos.CachingOutcomeMiss
		switch {
		case store.reads.Load() == readsBefore:
			outcome = demos.CachingOutcomeHit
		case after.Expiries > before.Expiries:
			outcome = demos.CachingOutcomeExpired
		case after.Invalidations > before.Invalidations:
			outcome = demos.CachingOutcomeInvalidated
		}
		return demos.CachingLabBeat{
			ID: id, Title: title, Script: script, Outcome: outcome, ObservedValue: observed,
			Hits: after.Hits, Misses: after.Misses, StoreReads: uint64(store.reads.Load()),
		}
	}
	step := func() (postsdata.CacheStats, int64) { return cache.Stats(), store.reads.Load() }

	// ── Beats 1–2: the cold read and its warm hit ─────────────────────
	before, readsBefore := step()
	prime, err := cache.GetByID(ctx, demos.CachingLabPrimeID)
	if err != nil {
		return demos.CachingLabRun{}, fmt.Errorf("cold read %s: %w", demos.CachingLabPrimeID, err)
	}
	after := cache.Stats()
	beats = append(beats, beat(before, after, readsBefore,
		"cold-read", "The first read misses",
		[]string{`GetByID("` + demos.CachingLabPrimeID + `")`}, prime.Title))

	before, readsBefore = step()
	primeAgain, err := cache.GetByID(ctx, demos.CachingLabPrimeID)
	if err != nil {
		return demos.CachingLabRun{}, fmt.Errorf("warm read %s: %w", demos.CachingLabPrimeID, err)
	}
	after = cache.Stats()
	beats = append(beats, beat(before, after, readsBefore,
		"warm-read", "The second read hits",
		[]string{`GetByID("` + demos.CachingLabPrimeID + `")`}, primeAgain.Title))

	// ── Beat 3: the page fills too, per canonical query ───────────────
	query := posts.ListQuery{Page: 1, Limit: 3, Sort: posts.SortNewest}
	before, readsBefore = step()
	page, err := cache.List(ctx, query)
	if err != nil {
		return demos.CachingLabRun{}, fmt.Errorf("page fill: %w", err)
	}
	if _, err := cache.List(ctx, query); err != nil {
		return demos.CachingLabRun{}, fmt.Errorf("page hit: %w", err)
	}
	after = cache.Stats()
	beats = append(beats, beat(before, after, readsBefore,
		"page-fill", "Pages cache per query — the second pass never asks",
		[]string{`List(page=1, limit=3, sort=newest)`, `List(page=1, limit=3, sort=newest)`},
		fmt.Sprintf("%d of %d posts, newest first", len(page.Items), page.TotalItems)))

	// ── Beat 4: a write retires the id and every page ─────────────────
	before, readsBefore = step()
	if _, err := cache.Replace(ctx, demos.CachingLabPrimeID, posts.Draft{
		Title: demos.CachingLabEditedTitle,
		Body:  "The lab's edit — a stale cache would keep serving the old title.",
		Tags:  []string{"go", "architecture"},
	}); err != nil {
		return demos.CachingLabRun{}, fmt.Errorf("edit %s: %w", demos.CachingLabPrimeID, err)
	}
	edited, err := cache.GetByID(ctx, demos.CachingLabPrimeID)
	if err != nil {
		return demos.CachingLabRun{}, fmt.Errorf("read after edit: %w", err)
	}
	if _, err := cache.List(ctx, query); err != nil {
		return demos.CachingLabRun{}, fmt.Errorf("list after edit: %w", err)
	}
	after = cache.Stats()
	beats = append(beats, beat(before, after, readsBefore,
		"write-invalidation", "The write retires the id and every cached page",
		[]string{
			`Replace("` + demos.CachingLabPrimeID + `", edited draft)`,
			`GetByID("` + demos.CachingLabPrimeID + `")`,
			`List(page=1, limit=3, sort=newest)`,
		}, edited.Title))

	// ── Beat 5: the TTL expires — read as data, never slept ───────────
	before, readsBefore = step()
	clock.Advance(demos.CachingLabClockStep)
	afterTTL, err := cache.GetByID(ctx, demos.CachingLabPrimeID)
	if err != nil {
		return demos.CachingLabRun{}, fmt.Errorf("read past the TTL: %w", err)
	}
	after = cache.Stats()
	beats = append(beats, beat(before, after, readsBefore,
		"ttl-expiry", "The fixture clock steps past the TTL",
		[]string{
			fmt.Sprintf("clock.Advance(%s)", demos.CachingLabClockStep),
			`GetByID("` + demos.CachingLabPrimeID + `")`,
		}, afterTTL.Title))

	// ── The herd: twelve readers, one store read ──────────────────────
	clock.Advance(demos.CachingLabClockStep)
	herdReadsBefore := store.reads.Load()
	titles := make([]string, demos.CachingLabHerdReaders)
	herdErrs := make([]error, demos.CachingLabHerdReaders)
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := range demos.CachingLabHerdReaders {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			post, err := cache.GetByID(ctx, demos.CachingLabHerdID)
			titles[i], herdErrs[i] = post.Title, err
		}()
	}
	close(start)
	wg.Wait()
	for i, err := range herdErrs {
		if err != nil {
			return demos.CachingLabRun{}, fmt.Errorf("herd reader %d: %w", i, err)
		}
	}
	sameAnswer := true
	for _, title := range titles {
		sameAnswer = sameAnswer && title == titles[0]
	}
	herd := demos.CachingLabHerd{
		ID:         demos.CachingLabHerdID,
		Readers:    demos.CachingLabHerdReaders,
		StoreReads: uint64(store.reads.Load() - herdReadsBefore),
		SameAnswer: sameAnswer,
		Answer:     titles[0],
	}

	// ── The swap: two stores, one cache, zero leaks ───────────────────
	swap := demos.CachingLabSwap{NotFoundID: demos.CachingLabLiveOnlyID}
	live := core.DataModeLive
	if _, err := scenario.Update(core.ScenarioPatch{DataMode: &live}); err != nil {
		return demos.CachingLabRun{}, fmt.Errorf("flip live: %w", err)
	}
	liveOnly, err := cache.GetByID(ctx, demos.CachingLabLiveOnlyID)
	if err != nil {
		return demos.CachingLabRun{}, fmt.Errorf("read the live-only post: %w", err)
	}
	swap.LiveOnlyID, swap.LiveOnlyTitle = demos.CachingLabLiveOnlyID, liveOnly.Title
	livePrime, err := cache.GetByID(ctx, demos.CachingLabPrimeID)
	if err != nil {
		return demos.CachingLabRun{}, fmt.Errorf("read p-1 in live: %w", err)
	}
	swap.LiveTitle = livePrime.Title

	fake := core.DataModeFake
	if _, err := scenario.Update(core.ScenarioPatch{DataMode: &fake}); err != nil {
		return demos.CachingLabRun{}, fmt.Errorf("flip fake: %w", err)
	}
	_, err = cache.GetByID(ctx, demos.CachingLabLiveOnlyID)
	swap.NotFoundSurfaced = errors.Is(err, core.ErrNotFound)
	if err != nil && !swap.NotFoundSurfaced {
		return demos.CachingLabRun{}, fmt.Errorf("read the live-only post in fake: %w", err)
	}
	backPrime, err := cache.GetByID(ctx, demos.CachingLabPrimeID)
	if err != nil {
		return demos.CachingLabRun{}, fmt.Errorf("read p-1 back in fake: %w", err)
	}
	swap.FakeTitle, swap.BackTitle = edited.Title, backPrime.Title

	final := cache.Stats()
	return demos.CachingLabRun{
		TTLSeconds: int(demos.CachingLabTTL.Seconds()),
		Beats:      beats,
		Herd:       herd,
		Swap:       swap,
		Stats: demos.CachingLabStats{
			Hits:          final.Hits,
			Misses:        final.Misses,
			Expiries:      final.Expiries,
			Invalidations: final.Invalidations,
			StoreReads:    uint64(store.reads.Load()),
			Entries:       uint64(cache.Entries()),
		},
	}, nil
}
