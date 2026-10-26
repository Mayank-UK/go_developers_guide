package postsdata

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"guide/internal/core"
	"guide/internal/domain/posts"
)

// The in-memory adapter's contract tests: deterministic seed order, the
// page math (last partial page, past-the-end empty page, totals under
// filtering), the sort vocabulary with id tie-breaks, full-replace
// semantics, the typed misses — and concurrency safety under -race with
// parallel hammering.

func TestSeedListsNewestFirstDeterministically(t *testing.T) {
	repo := NewInMemoryRepository()
	page, err := repo.List(context.Background(), posts.ListQuery{Page: 1, Limit: 50, Sort: posts.SortNewest})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if page.TotalItems != 6 || len(page.Items) != 6 {
		t.Fatalf("seed = %d items (total %d), want 6", len(page.Items), page.TotalItems)
	}
	if page.Items[0].ID != "p-6" || page.Items[5].ID != "p-1" {
		t.Errorf("newest-first order = %s…%s, want p-6…p-1", page.Items[0].ID, page.Items[5].ID)
	}
	for i := 1; i < len(page.Items); i++ {
		if page.Items[i-1].CreatedAt.Before(page.Items[i].CreatedAt) {
			t.Fatalf("createdAt must descend: %v before %v", page.Items[i-1].CreatedAt, page.Items[i].CreatedAt)
		}
	}
	if page.HasPrev {
		t.Error("first page has no previous")
	}
	if page.TotalPages != 1 || page.HasNext {
		t.Errorf("6 of 50 = one full page; got totalPages %d hasNext %v", page.TotalPages, page.HasNext)
	}
}

func TestListPaginatesWithPartialLastPage(t *testing.T) {
	repo := NewInMemoryRepository()
	query := posts.ListQuery{Page: 2, Limit: 4, Sort: posts.SortOldest}

	page, err := repo.List(context.Background(), query)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(page.Items) != 2 {
		t.Fatalf("page 2 of 6 by 4 carries %d items, want the partial 2", len(page.Items))
	}
	if page.TotalPages != 2 || page.TotalItems != 6 {
		t.Errorf("totals = %d items / %d pages, want 6/2", page.TotalItems, page.TotalPages)
	}
	if page.HasNext || !page.HasPrev {
		t.Errorf("neighbor flags = next %v prev %v, want false/true on the last page", page.HasNext, page.HasPrev)
	}
	if page.Items[0].ID != "p-5" || page.Items[1].ID != "p-6" {
		t.Errorf("oldest order page 2 = %s,%s, want p-5,p-6", page.Items[0].ID, page.Items[1].ID)
	}
}

func TestListPastTheEndIsAnHonestEmptyPage(t *testing.T) {
	repo := NewInMemoryRepository()
	page, err := repo.List(context.Background(), posts.ListQuery{Page: 9, Limit: 4, Sort: posts.SortNewest})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if page.Items == nil || len(page.Items) != 0 {
		t.Fatalf("past-the-end items = %#v, want an empty non-nil page", page.Items)
	}
	if page.TotalItems != 6 || page.TotalPages != 2 {
		t.Errorf("metadata must stay honest: %+v", page)
	}
	if page.HasNext {
		t.Error("no page exists past the end")
	}
}

func TestListFiltersByExactTag(t *testing.T) {
	repo := NewInMemoryRepository()
	page, err := repo.List(context.Background(), posts.ListQuery{Page: 1, Limit: 10, Tag: "api", Sort: posts.SortNewest})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if page.TotalItems != 2 {
		t.Fatalf("tag api matches %d posts, want 2 (p-4, p-5)", page.TotalItems)
	}
	for _, post := range page.Items {
		if !hasTag(post, "api") {
			t.Errorf("post %s leaked through the filter", post.ID)
		}
	}
	if _, err := repo.List(context.Background(), posts.ListQuery{Page: 1, Limit: 10, Tag: "nope", Sort: posts.SortNewest}); err != nil {
		t.Fatalf("zero-match filter is an empty page, not an error: %v", err)
	}
}

func hasTag(post posts.Post, tag string) bool {
	for _, candidate := range post.Tags {
		if candidate == tag {
			return true
		}
	}
	return false
}

func TestListSortsByTitleCaseInsensitively(t *testing.T) {
	repo := NewInMemoryRepository()
	page, err := repo.List(context.Background(), posts.ListQuery{Page: 1, Limit: 10, Sort: posts.SortTitle})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if page.Items[0].Title != "Designing JSON APIs" {
		t.Errorf("title sort starts with %q", page.Items[0].Title)
	}
	for i := 1; i < len(page.Items); i++ {
		prev := page.Items[i-1].Title
		next := page.Items[i].Title
		if lower(prev) > lower(next) {
			t.Errorf("titles out of order: %q before %q", prev, next)
		}
	}
}

func lower(s string) string {
	out := []rune(s)
	for i, r := range out {
		if r >= 'A' && r <= 'Z' {
			out[i] = r + ('a' - 'A')
		}
	}
	return string(out)
}

func TestCreateAssignsIdentityAndStampsBothTimestamps(t *testing.T) {
	stamp := time.Date(2026, 2, 1, 10, 0, 0, 0, time.UTC)
	repo := newRepository(func() time.Time { return stamp })
	created, err := repo.Create(context.Background(), posts.Draft{Title: "New", Body: "Body", Tags: []string{"x"}})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if created.ID != "p-7" {
		t.Errorf("first created id = %q, want p-7 (after six seeds)", created.ID)
	}
	if !created.CreatedAt.Equal(stamp) || !created.UpdatedAt.Equal(stamp) {
		t.Errorf("stamps = %v/%v, want the injected clock on both", created.CreatedAt, created.UpdatedAt)
	}
	stored, err := repo.GetByID(context.Background(), "p-7")
	if err != nil || stored.Title != "New" {
		t.Errorf("created post must be readable back: %+v err %v", stored, err)
	}
}

func TestReplacePreservesCreatedAtAndMovesUpdatedAt(t *testing.T) {
	later := time.Date(2026, 3, 1, 8, 30, 0, 0, time.UTC)
	repo := newRepository(func() time.Time { return later })
	replaced, err := repo.Replace(context.Background(), "p-1", posts.Draft{Title: "Rewritten", Body: "New body", Tags: []string{"edited"}})
	if err != nil {
		t.Fatalf("replace: %v", err)
	}
	if replaced.Title != "Rewritten" || replaced.Tags[0] != "edited" {
		t.Errorf("replaced = %+v", replaced)
	}
	if !replaced.CreatedAt.Equal(seedCreatedAt[0]) {
		t.Errorf("createdAt must be preserved, got %v", replaced.CreatedAt)
	}
	if !replaced.UpdatedAt.Equal(later) {
		t.Errorf("updatedAt must move to now, got %v", replaced.UpdatedAt)
	}
}

func TestMissesAreTypedNotFound(t *testing.T) {
	repo := NewInMemoryRepository()
	if _, err := repo.GetByID(context.Background(), "p-999"); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("get miss = %v, want the typed NotFound sentinel match", err)
	}
	if _, err := repo.Replace(context.Background(), "p-999", posts.Draft{Title: "t", Body: "b"}); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("replace miss = %v, want the typed NotFound sentinel match", err)
	}
	if err := repo.Delete(context.Background(), "p-999"); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("delete miss = %v, want the typed NotFound sentinel match", err)
	}
}

func TestDeleteRemovesThePostForReal(t *testing.T) {
	repo := NewInMemoryRepository()
	if err := repo.Delete(context.Background(), "p-2"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := repo.GetByID(context.Background(), "p-2"); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("deleted post must miss afterwards, got %v", err)
	}
	if err := repo.Delete(context.Background(), "p-2"); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("strict stance: deleting again must 404, got %v", err)
	}
	page, err := repo.List(context.Background(), posts.ListQuery{Page: 1, Limit: 10, Sort: posts.SortNewest})
	if err != nil || page.TotalItems != 5 {
		t.Errorf("total after delete = %d err %v, want 5", page.TotalItems, err)
	}
}

func TestReturnedPostsShareNoStateWithTheStore(t *testing.T) {
	repo := NewInMemoryRepository()
	post, err := repo.GetByID(context.Background(), "p-1")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	post.Tags[0] = "mutated"
	fresh, err := repo.GetByID(context.Background(), "p-1")
	if err != nil {
		t.Fatalf("get again: %v", err)
	}
	if fresh.Tags[0] == "mutated" {
		t.Error("handed-out posts must be copies; store was mutated through the API")
	}
}

func TestConcurrentAccessIsRaceClean(t *testing.T) {
	repo := NewInMemoryRepository()
	var wg sync.WaitGroup
	for worker := 0; worker < 8; worker++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			ctx := context.Background()
			for round := 0; round < 25; round++ {
				_, _ = repo.List(ctx, posts.ListQuery{Page: 1, Limit: 5, Tag: "go", Sort: posts.SortTitle})
				created, _ := repo.Create(ctx, posts.Draft{Title: "concurrent", Body: "hammering", Tags: []string{"race"}})
				_, _ = repo.Replace(ctx, created.ID, posts.Draft{Title: "replaced", Body: "still hammering", Tags: []string{"race"}})
				_, _ = repo.GetByID(ctx, "p-1")
				_ = repo.Delete(ctx, created.ID)
			}
		}(worker)
	}
	wg.Wait()
	// The seeds survive the storm untouched; created posts cleaned up.
	page, err := repo.List(context.Background(), posts.ListQuery{Page: 1, Limit: 50, Sort: posts.SortNewest})
	if err != nil || page.TotalItems != 6 {
		t.Errorf("after the storm: %d posts err %v, want the 6 seeds", page.TotalItems, err)
	}
}
