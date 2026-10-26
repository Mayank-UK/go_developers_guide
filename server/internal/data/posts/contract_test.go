package postsdata_test

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"guide/internal/core"
	"guide/internal/data/db"
	postsdata "guide/internal/data/posts"
	"guide/internal/domain/posts"
)

// THE SHARED BEHAVIOR TABLE — the port's contract, run against BOTH
// adapters: the in-memory fake and the PostgreSQL repository on a
// throwaway database. A rule the fake honors and the live store forgot
// (or vice versa) fails HERE first, which is what makes the runtime
// fake/live switch honest: both stores answer through the same table
// before either is wired behind the facade.
//
// The PostgreSQL runs follow the data-layer test discipline: every test
// draws a DEDICATED throwaway database through an admin connection to the
// `postgres` maintenance database (never the dev `guide` database),
// migrates it, seeds it, and drops it in t.Cleanup. GDG_PG_ADMIN_DSN
// overrides the admin DSN (e.g. host port 5433).

// defaultAdminDSN points at the compose stack's PostgreSQL through its
// in-network service name, at the `postgres` maintenance database — NOT
// the dev `guide` database, which these tests must never touch.
const defaultAdminDSN = "postgres://postgres:guide@postgres:5432/postgres?sslmode=disable"

// createThrowawayPool creates a fresh database on the live server and
// connects a pool to it, unmigrated and unseeded — callers own the state
// they want to test against. The drop is scheduled here.
func createThrowawayPool(t *testing.T, adminDSN string) *pgxpool.Pool {
	t.Helper()

	parsed, err := url.Parse(adminDSN)
	if err != nil {
		t.Fatalf("parse admin DSN: %v", err)
	}
	if strings.TrimPrefix(parsed.Path, "/") == "guide" {
		t.Fatalf("admin DSN must not target the dev database (got %q) — tests create their own throwaway databases", parsed.Path)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	admin, err := pgxpool.New(ctx, adminDSN)
	if err != nil {
		t.Fatalf("admin connect: %v (is the dev stack's PostgreSQL up?)", err)
	}
	if err := admin.Ping(ctx); err != nil {
		t.Fatalf("admin ping: %v (is the dev stack's PostgreSQL up?)", err)
	}

	name := fmt.Sprintf("gdg_test_%d_%d", os.Getpid(), time.Now().UnixNano())
	if _, err := admin.Exec(ctx, "CREATE DATABASE "+name); err != nil {
		t.Fatalf("create throwaway database %s: %v", name, err)
	}

	parsed.Path = "/" + name
	pool, err := db.NewPool(ctx, parsed.String())
	if err != nil {
		t.Fatalf("connect throwaway database %s: %v", name, err)
	}

	t.Cleanup(func() {
		pool.Close()
		dropCtx, dropCancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer dropCancel()
		// WITH (FORCE): no lingering connection can veto the drop.
		if _, err := admin.Exec(dropCtx, fmt.Sprintf("DROP DATABASE %s WITH (FORCE)", name)); err != nil {
			t.Errorf("drop throwaway database %s: %v", name, err)
		}
		admin.Close()
	})
	return pool
}

// newThrowawayStore migrates and seeds a fresh throwaway database and
// returns the live repository over it — the state the behavior table's
// live runs answer from, identical to a freshly booted server.
func newThrowawayStore(t *testing.T) *postsdata.PostgresRepository {
	t.Helper()

	adminDSN := os.Getenv("GDG_PG_ADMIN_DSN")
	if adminDSN == "" {
		adminDSN = defaultAdminDSN
	}
	pool := createThrowawayPool(t, adminDSN)
	ctx := context.Background()
	if _, err := db.Migrate(ctx, pool); err != nil {
		t.Fatalf("migrate throwaway database: %v", err)
	}
	if _, err := postsdata.Seed(ctx, pool); err != nil {
		t.Fatalf("seed throwaway database: %v", err)
	}
	return postsdata.NewPostgresRepository(pool)
}

// newBothStores is the table's driver: every behavior case runs against
// the fake and, when the stack's PostgreSQL is reachable, the live store.
func newBothStores(t *testing.T) map[string]posts.PostsRepository {
	t.Helper()
	stores := map[string]posts.PostsRepository{"fake": postsdata.NewInMemoryRepository()}
	if !testing.Short() {
		stores["live"] = newThrowawayStore(t)
	}
	return stores
}

// TestBothStoresServeTheSameSeed verifies the seed parity first: the same
// six posts, the same newest-first order, the same honest first page.
func TestBothStoresServeTheSameSeed(t *testing.T) {
	for name, store := range newBothStores(t) {
		t.Run(name, func(t *testing.T) {
			page, err := store.List(context.Background(), posts.ListQuery{Page: 1, Limit: 50, Sort: posts.SortNewest})
			if err != nil {
				t.Fatalf("list: %v", err)
			}
			if page.TotalItems != 6 || len(page.Items) != 6 {
				t.Fatalf("seed = %d items (total %d), want 6", len(page.Items), page.TotalItems)
			}
			wantTitles := []string{
				"Structured logging with slog",
				"Pagination that tells the truth",
				"Designing JSON APIs",
				"Errors are values",
				"ServeMux patterns in Go 1.22",
				"Why Go for backends",
			}
			for i, want := range wantTitles {
				if page.Items[i].Title != want {
					t.Errorf("newest-first[%d] = %q, want %q", i, page.Items[i].Title, want)
				}
			}
			for i := 1; i < len(page.Items); i++ {
				if page.Items[i-1].CreatedAt.Before(page.Items[i].CreatedAt) {
					t.Errorf("createdAt must descend: %v before %v", page.Items[i-1].CreatedAt, page.Items[i].CreatedAt)
				}
			}
			if page.TotalPages != 1 || page.HasNext || page.HasPrev {
				t.Errorf("6 of 50 = one page; got %+v", page)
			}
		})
	}
}

// TestBothStoresPaginateIdentically pins the page math edges: the partial
// last page, the past-the-end empty page, and honest metadata everywhere.
func TestBothStoresPaginateIdentically(t *testing.T) {
	for name, store := range newBothStores(t) {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()

			page, err := store.List(ctx, posts.ListQuery{Page: 2, Limit: 4, Sort: posts.SortOldest})
			if err != nil {
				t.Fatalf("list page 2: %v", err)
			}
			if len(page.Items) != 2 || page.TotalPages != 2 || page.TotalItems != 6 {
				t.Fatalf("partial last page = %d items / %d pages / %d total, want 2/2/6", len(page.Items), page.TotalPages, page.TotalItems)
			}
			if page.HasNext || !page.HasPrev {
				t.Errorf("neighbor flags on the last page = next %v prev %v, want false/true", page.HasNext, page.HasPrev)
			}
			if page.Items[0].Title != "Pagination that tells the truth" || page.Items[1].Title != "Structured logging with slog" {
				t.Errorf("oldest order page 2 = %q,%q", page.Items[0].Title, page.Items[1].Title)
			}

			empty, err := store.List(ctx, posts.ListQuery{Page: 9, Limit: 4, Sort: posts.SortNewest})
			if err != nil {
				t.Fatalf("list past the end: %v", err)
			}
			if empty.Items == nil || len(empty.Items) != 0 {
				t.Fatalf("past-the-end items = %#v, want an empty non-nil page", empty.Items)
			}
			if empty.TotalItems != 6 || empty.TotalPages != 2 || empty.HasNext {
				t.Errorf("past-the-end metadata must stay honest: %+v", empty)
			}
		})
	}
}

// TestBothStoresFilterAndSortIdentically pins the tag filter's exact
// match and the title sort's case-insensitive order.
func TestBothStoresFilterAndSortIdentically(t *testing.T) {
	for name, store := range newBothStores(t) {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()

			filtered, err := store.List(ctx, posts.ListQuery{Page: 1, Limit: 10, Tag: "api", Sort: posts.SortNewest})
			if err != nil {
				t.Fatalf("list tag=api: %v", err)
			}
			if filtered.TotalItems != 2 {
				t.Fatalf("tag api matches %d posts, want 2", filtered.TotalItems)
			}
			for _, post := range filtered.Items {
				if !hasTag(post, "api") {
					t.Errorf("post %s leaked through the filter", post.ID)
				}
			}

			none, err := store.List(ctx, posts.ListQuery{Page: 1, Limit: 10, Tag: "nope", Sort: posts.SortNewest})
			if err != nil || none.TotalItems != 0 || none.Items == nil {
				t.Errorf("zero-match filter must be an honest empty page, got %+v err %v", none, err)
			}

			titled, err := store.List(ctx, posts.ListQuery{Page: 1, Limit: 10, Sort: posts.SortTitle})
			if err != nil {
				t.Fatalf("list sort=title: %v", err)
			}
			if titled.Items[0].Title != "Designing JSON APIs" {
				t.Errorf("title sort starts with %q", titled.Items[0].Title)
			}
			for i := 1; i < len(titled.Items); i++ {
				if strings.ToLower(titled.Items[i-1].Title) > strings.ToLower(titled.Items[i].Title) {
					t.Errorf("titles out of order: %q before %q", titled.Items[i-1].Title, titled.Items[i].Title)
				}
			}
		})
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

// TestBothStoresCreateIdentically pins create semantics: identity is
// minted (an opaque unique id — the fake's p-N, the live store's canonical
// UUIDv7), both stamps start equal, and tags land as the sorted set.
func TestBothStoresCreateIdentically(t *testing.T) {
	for name, store := range newBothStores(t) {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			created, err := store.Create(ctx, posts.Draft{
				Title: "New", Body: "Body",
				Tags: []string{"go", "go", "http"}, // duplicates collapse to the set
			})
			if err != nil {
				t.Fatalf("create: %v", err)
			}
			if created.ID == "" {
				t.Fatal("create must mint an id")
			}
			if !created.CreatedAt.Equal(created.UpdatedAt) {
				t.Errorf("a fresh post's stamps start equal, got %v/%v", created.CreatedAt, created.UpdatedAt)
			}
			if strings.Join(created.Tags, ",") != "go,http" {
				t.Errorf("tags = %v, want the sorted set [go http]", created.Tags)
			}

			stored, err := store.GetByID(ctx, created.ID)
			if err != nil || stored.Title != "New" {
				t.Fatalf("created post must read back: %+v err %v", stored, err)
			}
			if strings.Join(stored.Tags, ",") != "go,http" {
				t.Errorf("stored tags = %v, want [go http]", stored.Tags)
			}

			// Handed-out posts are copies: mutating one never touches the store.
			stored.Tags[0] = "mutated"
			fresh, err := store.GetByID(ctx, created.ID)
			if err != nil {
				t.Fatalf("get again: %v", err)
			}
			if fresh.Tags[0] == "mutated" {
				t.Error("handed-out posts must be copies; the store was mutated through the API")
			}
		})
	}
}

// TestBothStoresReplaceIdentically pins PUT semantics: full replacement
// (the draft IS the next state — old tags die), CreatedAt preserved,
// UpdatedAt moved forward.
func TestBothStoresReplaceIdentically(t *testing.T) {
	for name, store := range newBothStores(t) {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			page, err := store.List(ctx, posts.ListQuery{Page: 1, Limit: 1, Sort: posts.SortOldest})
			if err != nil || len(page.Items) != 1 {
				t.Fatalf("oldest post: %+v err %v", page, err)
			}
			victim := page.Items[0]

			replaced, err := store.Replace(ctx, victim.ID, posts.Draft{
				Title: "Rewritten", Body: "New body", Tags: []string{"edited"},
			})
			if err != nil {
				t.Fatalf("replace: %v", err)
			}
			if replaced.Title != "Rewritten" || strings.Join(replaced.Tags, ",") != "edited" {
				t.Errorf("replaced = %+v", replaced)
			}
			if !replaced.CreatedAt.Equal(victim.CreatedAt) {
				t.Errorf("createdAt must be preserved: %v -> %v", victim.CreatedAt, replaced.CreatedAt)
			}
			if !replaced.UpdatedAt.After(victim.UpdatedAt) {
				t.Errorf("updatedAt must move forward: %v -> %v", victim.UpdatedAt, replaced.UpdatedAt)
			}
			// The old tag edges are gone — replacement is whole, not merged.
			after, err := store.List(ctx, posts.ListQuery{Page: 1, Limit: 10, Tag: "go", Sort: posts.SortNewest})
			if err != nil {
				t.Fatalf("list after replace: %v", err)
			}
			for _, post := range after.Items {
				if post.ID == victim.ID && hasTag(post, "architecture") {
					t.Error("the victim's old tags must not survive replacement")
				}
			}
		})
	}
}

// TestBothStoresMissIdentically pins the strict-404 stance for every id
// shape: an id that never existed, an id that was deleted, and (live-side
// especially) an id that is not even a canonical uuid — all typed
// NotFound, never a driver error.
func TestBothStoresMissIdentically(t *testing.T) {
	for name, store := range newBothStores(t) {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			for _, missing := range []string{"does-not-exist", "p-999", "019b8d70-3620-7101-82b6-a17c9d4e0f99"} {
				if _, err := store.GetByID(ctx, missing); !errors.Is(err, core.ErrNotFound) {
					t.Errorf("get %q = %v, want the typed NotFound", missing, err)
				}
				if _, err := store.Replace(ctx, missing, posts.Draft{Title: "t", Body: "b"}); !errors.Is(err, core.ErrNotFound) {
					t.Errorf("replace %q = %v, want the typed NotFound", missing, err)
				}
				if err := store.Delete(ctx, missing); !errors.Is(err, core.ErrNotFound) {
					t.Errorf("delete %q = %v, want the typed NotFound", missing, err)
				}
			}

			page, err := store.List(ctx, posts.ListQuery{Page: 1, Limit: 1, Sort: posts.SortNewest})
			if err != nil || len(page.Items) != 1 {
				t.Fatalf("newest post: %+v err %v", page, err)
			}
			doomed := page.Items[0].ID
			if err := store.Delete(ctx, doomed); err != nil {
				t.Fatalf("delete: %v", err)
			}
			if _, err := store.GetByID(ctx, doomed); !errors.Is(err, core.ErrNotFound) {
				t.Errorf("deleted post must miss afterwards, got %v", err)
			}
			if err := store.Delete(ctx, doomed); !errors.Is(err, core.ErrNotFound) {
				t.Errorf("strict stance: deleting again must miss, got %v", err)
			}
			after, err := store.List(ctx, posts.ListQuery{Page: 1, Limit: 50, Sort: posts.SortNewest})
			if err != nil || after.TotalItems != 5 {
				t.Errorf("total after delete = %d err %v, want 5", after.TotalItems, err)
			}
		})
	}
}

// TestBothStoresValidateThroughTheSameUseCase pins what the mode switch
// actually promises: validation lives in the shared use case, so a bad
// draft is a 422 before EITHER store is consulted — the stores never
// diverge on validation because they are never asked.
func TestBothStoresValidateThroughTheSameUseCase(t *testing.T) {
	for name, store := range newBothStores(t) {
		t.Run(name, func(t *testing.T) {
			create := posts.NewCreatePostUseCase(store)
			_, err := create.Execute(context.Background(), posts.Draft{Title: "", Body: "", Tags: nil})
			if err == nil || !errors.Is(err, core.ErrValidationFailed) {
				t.Fatalf("invalid draft = %v, want the shared 422", err)
			}
			page, err := store.List(context.Background(), posts.ListQuery{Page: 1, Limit: 50, Sort: posts.SortNewest})
			if err != nil || page.TotalItems != 6 {
				t.Errorf("a rejected draft must leave the store untouched: %d err %v", page.TotalItems, err)
			}
		})
	}
}

// TestBothStoresAreRaceClean hammers both stores under the race detector —
// the fake's mutex and the live store's pool + transactions must both
// answer concurrent reads and writes without a torn state.
func TestBothStoresAreRaceClean(t *testing.T) {
	for name, store := range newBothStores(t) {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			var wg sync.WaitGroup
			for worker := 0; worker < 4; worker++ {
				wg.Add(1)
				go func(worker int) {
					defer wg.Done()
					for round := 0; round < 5; round++ {
						_, _ = store.List(ctx, posts.ListQuery{Page: 1, Limit: 5, Tag: "go", Sort: posts.SortTitle})
						created, _ := store.Create(ctx, posts.Draft{Title: "concurrent", Body: "hammering", Tags: []string{"race"}})
						_, _ = store.Replace(ctx, created.ID, posts.Draft{Title: "replaced", Body: "still hammering", Tags: []string{"race"}})
						_, _ = store.GetByID(ctx, created.ID)
						_ = store.Delete(ctx, created.ID)
					}
				}(worker)
			}
			wg.Wait()
			page, err := store.List(ctx, posts.ListQuery{Page: 1, Limit: 50, Sort: posts.SortNewest})
			if err != nil || page.TotalItems != 6 {
				t.Errorf("after the storm: %d posts err %v, want the 6 seeds", page.TotalItems, err)
			}
		})
	}
}
