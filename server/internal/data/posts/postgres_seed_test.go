package postsdata_test

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"guide/internal/data/db"
	postsdata "guide/internal/data/posts"
	"guide/internal/domain/posts"
)

// The PostgreSQL adapter's database-backed pins that are NOT part of the
// shared behavior table: the seed's idempotency and parity with the
// in-memory collection, the updated_at trigger firing under Replace, the
// tag vocabulary growing through the edges, and the seed's all-or-nothing
// refusal on an unmigrated database. External test package on purpose:
// these import internal/data/db (migrations + pool), which the internal
// package's tests cannot reach without an import cycle.

// newThrowawayStorePool is newThrowawayStore's pool-only twin, for the
// seed tests that need the database BEFORE any seed ran.
func newThrowawayStorePool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	adminDSN := defaultAdminDSN
	if override := os.Getenv("GDG_PG_ADMIN_DSN"); override != "" {
		adminDSN = override
	}
	return createThrowawayPool(t, adminDSN)
}

func TestSeedIsIdempotentAndServesTheGuidePosts(t *testing.T) {
	pool := newThrowawayStorePool(t)
	ctx := context.Background()
	if _, err := db.Migrate(ctx, pool); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	first, err := postsdata.Seed(ctx, pool)
	if err != nil {
		t.Fatalf("first seed: %v", err)
	}
	if first != 6 {
		t.Fatalf("first seed inserted %d posts, want 6", first)
	}
	second, err := postsdata.Seed(ctx, pool)
	if err != nil {
		t.Fatalf("second seed: %v", err)
	}
	if second != 0 {
		t.Fatalf("second seed inserted %d posts, want 0 — ON CONFLICT must make re-seeds no-ops", second)
	}

	repo := postsdata.NewPostgresRepository(pool)
	page, err := repo.List(ctx, posts.ListQuery{Page: 1, Limit: 50, Sort: posts.SortOldest})
	if err != nil {
		t.Fatalf("list seeded: %v", err)
	}
	if page.TotalItems != 6 {
		t.Fatalf("seeded total = %d, want 6", page.TotalItems)
	}
	// The same collection the fake serves, oldest first, ending at the
	// fixed UUIDv7 literal — the seed's identity is stable across boots.
	wantTitles := []string{
		"Why Go for backends",
		"ServeMux patterns in Go 1.22",
		"Errors are values",
		"Designing JSON APIs",
		"Pagination that tells the truth",
		"Structured logging with slog",
	}
	for i, want := range wantTitles {
		if page.Items[i].Title != want {
			t.Errorf("seeded post %d = %q, want %q", i, page.Items[i].Title, want)
		}
	}
	if page.Items[0].ID != "019b8d70-3620-7101-82b6-a17c9d4e0f01" {
		t.Errorf("oldest seeded id = %q, want the fixed literal", page.Items[0].ID)
	}
	if strings.Join(page.Items[0].Tags, ",") != "architecture,go" {
		t.Errorf("oldest seeded tags = %v, want [architecture go]", page.Items[0].Tags)
	}
}

func TestSeedRefusesToRunAgainstAnUnmigratedDatabase(t *testing.T) {
	// A bare throwaway pool with no tables is the state a misconfigured
	// boot would see; the seed must fail honestly (its transaction rolls
	// back) rather than half-apply.
	pool := newThrowawayStorePool(t)
	if _, err := postsdata.Seed(context.Background(), pool); err == nil {
		t.Fatal("seed against an unmigrated database must fail, not half-apply")
	}
}

func TestCreateMintsUUIDv7AndReplaceFiresTheTrigger(t *testing.T) {
	store := newThrowawayStore(t)
	ctx := context.Background()

	created, err := store.Create(ctx, posts.Draft{Title: "Trigger proof", Body: "updated_at belongs to the database", Tags: []string{"go"}})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if len(created.ID) != 36 || created.ID[14] != '7' || created.ID[19] != '8' && created.ID[19] != '9' && created.ID[19] != 'a' && created.ID[19] != 'b' {
		t.Fatalf("created id %q is not a canonical RFC 9562 UUIDv7", created.ID)
	}

	time.Sleep(5 * time.Millisecond) // make the trigger's stamp unambiguous
	replaced, err := store.Replace(ctx, created.ID, posts.Draft{Title: "Trigger proof 2", Body: "still belongs to the database", Tags: []string{"go", "database"}})
	if err != nil {
		t.Fatalf("replace: %v", err)
	}
	if !replaced.UpdatedAt.After(replaced.CreatedAt) {
		t.Errorf("the topic 16 trigger must stamp updated_at on UPDATE: %v !> %v", replaced.UpdatedAt, replaced.CreatedAt)
	}
	if strings.Join(replaced.Tags, ",") != "database,go" {
		t.Errorf("replaced tags = %v, want the sorted set [database go]", replaced.Tags)
	}

	// The vocabulary grew by the new name, and the edge answers the exact
	// reverse lookup the tag filter rides — asserted through the port, so
	// the pin is about behavior, not table shape.
	byTag, err := store.List(ctx, posts.ListQuery{Page: 1, Limit: 10, Tag: "database", Sort: posts.SortNewest})
	if err != nil {
		t.Fatalf("list tag=database: %v", err)
	}
	if byTag.TotalItems != 1 || byTag.Items[0].ID != created.ID {
		t.Errorf("tag=database after replace = %+v, want exactly the replaced post", byTag.Items)
	}
}
