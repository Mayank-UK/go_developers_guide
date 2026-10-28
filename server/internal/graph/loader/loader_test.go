package loader

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"guide/internal/core"
	postsdata "guide/internal/data/posts"
	"guide/internal/domain/posts"
)

// The loader's claims, as data: concurrent Loads share ONE batch fetch,
// duplicate ids collapse, misses arrive as the typed taxonomy error, and
// the counting wrapper turns the N+1 question into a number — the naive
// pattern spends one List plus one GetByID per item (N+1 port calls)
// where the loader spends a single List.

// TestConcurrentLoadsShareOneBatchFetch pins the headline: nine Loads
// (six unique ids plus three duplicates) issued the way sibling field
// resolvers issue them — concurrently — cross the batch seam exactly
// once, every Load returns the right post, and duplicates receive the
// same answer twice.
func TestConcurrentLoadsShareOneBatchFetch(t *testing.T) {
	repo := postsdata.NewInMemoryRepository()
	var fetches atomic.Int64
	loader := NewPostsLoader(func(ctx context.Context, ids []string) (map[string]posts.Post, error) {
		fetches.Add(1)
		return PortBatch(repo)(ctx, ids)
	})

	ids := []string{"p-1", "p-2", "p-3", "p-4", "p-5", "p-6", "p-2", "p-4", "p-6"}
	results := make([]posts.Post, len(ids))
	errs := make([]error, len(ids))
	var wg sync.WaitGroup
	for i, id := range ids {
		wg.Add(1)
		go func(i int, id string) {
			defer wg.Done()
			results[i], errs[i] = loader.Load(context.Background(), id)
		}(i, id)
	}
	wg.Wait()

	if got := fetches.Load(); got != 1 {
		t.Fatalf("batch fetches = %d, want exactly 1 for %d concurrent loads", got, len(ids))
	}
	for i, id := range ids {
		if errs[i] != nil {
			t.Fatalf("Load(%q) errored: %v", id, errs[i])
		}
		if results[i].ID != id {
			t.Errorf("Load(%q) returned post %q", id, results[i].ID)
		}
	}
	// Duplicates must receive the identical answer (same id, same title).
	if results[1].ID != results[6].ID || results[1].Title != results[6].Title {
		t.Errorf("duplicate ids resolved differently: %q vs %q", results[1].Title, results[6].Title)
	}
}

// TestNaiveVersusLoaderPortCalls is the experiment the demo reports: the
// same page of six posts hydrated two ways over the same instrumented
// port. Naive — one List then one GetByID per item — costs 7 port calls;
// the loader costs 1 for the whole batch. Behind a database, each call is
// a round trip.
func TestNaiveVersusLoaderPortCalls(t *testing.T) {
	naiveRepo := NewCountingRepository(postsdata.NewInMemoryRepository())
	naivePage, err := naiveRepo.List(context.Background(), posts.ListQuery{Page: 1, Limit: posts.MaxPageLimit, Sort: posts.SortOldest})
	if err != nil {
		t.Fatalf("naive setup: %v", err)
	}
	ids := make([]string, 0, len(naivePage.Items))
	for _, post := range naivePage.Items {
		ids = append(ids, post.ID)
	}
	for _, id := range ids {
		if _, err := naiveRepo.GetByID(context.Background(), id); err != nil {
			t.Fatalf("naive GetByID(%q): %v", id, err)
		}
	}
	naiveCounts := naiveRepo.Snapshot()

	loaderRepo := NewCountingRepository(postsdata.NewInMemoryRepository())
	loader := NewPostsLoader(PortBatch(loaderRepo))
	var wg sync.WaitGroup
	for _, id := range ids {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			if _, err := loader.Load(context.Background(), id); err != nil {
				t.Errorf("Load(%q): %v", id, err)
			}
		}(id)
	}
	wg.Wait()
	loaderCounts := loaderRepo.Snapshot()

	if len(ids) != 6 {
		t.Fatalf("seed page = %d posts, want the 6 deterministic seeds", len(ids))
	}
	if naiveCounts.List != 1 || naiveCounts.GetByID != 6 || naiveCounts.Total() != 7 {
		t.Errorf("naive counts = %+v (total %d), want 1 list + 6 getById = 7", naiveCounts, naiveCounts.Total())
	}
	if loaderCounts.List != 1 || loaderCounts.Total() != 1 {
		t.Errorf("loader counts = %+v (total %d), want the single batched list", loaderCounts, loaderCounts.Total())
	}
}

// TestClosedBatchFiresOnCountNotClock pins the deterministic mode: with the
// batch boundary set by COUNT, slowly-issued loads — gaps far wider than
// the coalescing window — still cross the seam exactly once. The window
// may retire a quiet dispatcher, but in closed-batch mode it can never
// fire a batch early; only the count does. The staggering is the proof:
// under window dispatch these gaps would cost six fetches, so the count
// landing on 1 either way makes the assertion bimodal, not flaky.
func TestClosedBatchFiresOnCountNotClock(t *testing.T) {
	repo := postsdata.NewInMemoryRepository()
	var fetches atomic.Int64
	closedLoader := NewPostsLoader(func(ctx context.Context, ids []string) (map[string]posts.Post, error) {
		fetches.Add(1)
		return PortBatch(repo)(ctx, ids)
	}, WithClosedBatch(6))

	ids := []string{"p-1", "p-2", "p-3", "p-4", "p-5", "p-6"}
	var wg sync.WaitGroup
	for i, id := range ids {
		wg.Add(1)
		go func(i int, id string) {
			defer wg.Done()
			// Each load lands well after the previous one's window closed.
			time.Sleep(time.Duration(i) * 25 * time.Millisecond)
			if _, err := closedLoader.Load(context.Background(), id); err != nil {
				t.Errorf("Load(%q): %v", id, err)
			}
		}(i, id)
	}
	wg.Wait()

	if got := fetches.Load(); got != 1 {
		t.Fatalf("batch fetches = %d, want exactly 1: a closed batch fires on the count, never on the clock", got)
	}
}

// TestSequentialLoadsGetTheirOwnBatches pins the semantics rather than
// the performance: a caller that WAITS for each Load before issuing the
// next cannot be batched (it is not concurrent), so each gets its own
// fetch — sequential callers pay one call each, concurrent ones share.
func TestSequentialLoadsGetTheirOwnBatches(t *testing.T) {
	repo := postsdata.NewInMemoryRepository()
	var fetches atomic.Int64
	loader := NewPostsLoader(func(ctx context.Context, ids []string) (map[string]posts.Post, error) {
		fetches.Add(1)
		return PortBatch(repo)(ctx, ids)
	})

	first, err := loader.Load(context.Background(), "p-1")
	if err != nil || first.ID != "p-1" {
		t.Fatalf("first Load = %q, %v", first.ID, err)
	}
	second, err := loader.Load(context.Background(), "p-2")
	if err != nil || second.ID != "p-2" {
		t.Fatalf("second Load = %q, %v", second.ID, err)
	}
	if got := fetches.Load(); got != 2 {
		t.Errorf("batch fetches = %d, want 2 (sequential Loads cannot share a window)", got)
	}
}

// TestAbsentIDReportsTheTypedMiss pins the error contract: an id the
// batch did not find arrives as core.NotFound — the same typed taxonomy
// error a point read serves, ready for extensions.code not_found.
func TestAbsentIDReportsTheTypedMiss(t *testing.T) {
	loader := NewPostsLoader(PortBatch(postsdata.NewInMemoryRepository()))
	_, err := loader.Load(context.Background(), "p-999")
	if err == nil {
		t.Fatal("absent id must error")
	}
	if !errors.Is(err, core.ErrNotFound) {
		t.Errorf("absent id error = %v, want the typed not_found taxonomy error", err)
	}
}

// TestCountingRepositoryCountsEveryPortMethod proves the instrument
// itself: one call through each method moves exactly its own counter.
func TestCountingRepositoryCountsEveryPortMethod(t *testing.T) {
	counting := NewCountingRepository(postsdata.NewInMemoryRepository())
	ctx := context.Background()

	if _, err := counting.List(ctx, posts.ListQuery{Page: 1, Limit: 2}); err != nil {
		t.Fatalf("List: %v", err)
	}
	if _, err := counting.GetByID(ctx, "p-1"); err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if _, err := counting.Create(ctx, posts.Draft{Title: "t", Body: "b"}); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if _, err := counting.Replace(ctx, "p-1", posts.Draft{Title: "t2", Body: "b2"}); err != nil {
		t.Fatalf("Replace: %v", err)
	}
	if err := counting.Delete(ctx, "p-1"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	got := counting.Snapshot()
	want := PortCounts{List: 1, GetByID: 1, Create: 1, Replace: 1, Delete: 1}
	if got != want || got.Total() != 5 {
		t.Errorf("counts = %+v (total %d), want %+v", got, got.Total(), want)
	}
}
