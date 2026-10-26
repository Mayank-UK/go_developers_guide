package postsdata

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"guide/internal/core"
	"guide/internal/domain/posts"
)

// The PostgreSQL adapter's package-internal pins — the pieces that touch
// unexported seams: the uuid helpers' contract, the minter's
// monotonicity, and the honest degradation when the store is unreachable
// (a closed pool answers instantly, so the 503 path is proven without
// waiting on a network timeout). The database-backed behavior lives in
// the external postsdata_test package (contract_test.go +
// postgres_seed_test.go), which may import internal/data/db freely.

func TestParseUUIDAcceptsOnlyCanonicalForm(t *testing.T) {
	t.Parallel()
	good := []string{
		"019b8d70-3620-7101-82b6-a17c9d4e0f01",
		"00000000-0000-0000-0000-000000000000",
		"019B8D70-3620-7101-82B6-A17C9D4E0F01", // upper case parses too
	}
	for _, candidate := range good {
		if _, ok := parseUUID(candidate); !ok {
			t.Errorf("parseUUID(%q) rejected a canonical form", candidate)
		}
	}
	bad := []string{
		"",
		"p-1",
		"019b8d703620710182b6a17c9d4e0f01",     // no dashes
		"019b8d70-3620-7101-82b6-a17c9d4e0f0",  // 35 chars
		"019b8d70+3620-7101-82b6-a17c9d4e0f01", // wrong separator
		"g19b8d70-3620-7101-82b6-a17c9d4e0f01", // non-hex
	}
	for _, candidate := range bad {
		if _, ok := parseUUID(candidate); ok {
			t.Errorf("parseUUID(%q) accepted a malformed id", candidate)
		}
	}
}

func TestFormatUUIDRoundTripsThroughParse(t *testing.T) {
	t.Parallel()
	canonical := "019b8d70-3620-7101-82b6-a17c9d4e0f01"
	key, ok := parseUUID(canonical)
	if !ok {
		t.Fatalf("parse failed for %q", canonical)
	}
	if got := formatUUID(key); got != canonical {
		t.Errorf("format(parse(%q)) = %q", canonical, got)
	}
}

func TestMinterProducesMonotonicCanonicalV7s(t *testing.T) {
	t.Parallel()
	frozen := time.UnixMilli(1767604500000) // one stuck millisecond
	minter := &uuidMinter{
		random: bytes.NewReader(make([]byte, 1<<16)),
		now:    func() time.Time { return frozen },
		lastMS: -1,
	}
	previous := ""
	for i := 0; i < 32; i++ {
		value, err := minter.next()
		if err != nil {
			t.Fatalf("mint %d: %v", i, err)
		}
		if len(value) != 36 || value[14] != '7' {
			t.Fatalf("minted %q is not a canonical version-7 uuid", value)
		}
		if value <= previous {
			t.Fatalf("same-millisecond ids must be monotonic: %q after %q", value, previous)
		}
		previous = value
	}

	// Counter exhaustion inside a stuck millisecond must fail loudly
	// instead of wrapping around: a minter that waits only 4096 ids per
	// millisecond on a frozen clock has hit a programmer bug.
	exhausted := &uuidMinter{
		random: bytes.NewReader(make([]byte, 1<<16)),
		now:    func() time.Time { return frozen },
		lastMS: frozen.UnixMilli(),
		seq:    uuidSeqMax,
	}
	if _, err := exhausted.next(); err == nil {
		t.Fatal("counter exhaustion on a non-advancing clock must error loudly")
	}
}

func TestLiveStoreDegradesToTypedUnavailableWhenDown(t *testing.T) {
	t.Parallel()
	// A closed pool answers every query with an instant acquire failure —
	// the deterministic stand-in for "the database is gone", no timeout
	// waits. The adapter must map it to the typed service_unavailable
	// error the central writer turns into the honest 503 envelope.
	pool, err := pgxpool.New(context.Background(), "postgres://postgres:guide@127.0.0.1:1/guide?sslmode=disable")
	if err != nil {
		t.Fatalf("build pool: %v", err)
	}
	pool.Close()

	repo := NewPostgresRepository(pool)
	ctx := context.Background()
	if _, err := repo.List(ctx, posts.ListQuery{Page: 1, Limit: 10, Sort: posts.SortNewest}); !errors.Is(err, core.ErrServiceUnavailable) {
		t.Errorf("list against a down store = %v, want the typed service_unavailable", err)
	}
	if _, err := repo.GetByID(ctx, SeedPostIDs[0]); !errors.Is(err, core.ErrServiceUnavailable) {
		t.Errorf("get against a down store = %v, want the typed service_unavailable", err)
	}
	if _, err := repo.Create(ctx, posts.Draft{Title: "t", Body: "b"}); !errors.Is(err, core.ErrServiceUnavailable) {
		t.Errorf("create against a down store = %v, want the typed service_unavailable", err)
	}
}
