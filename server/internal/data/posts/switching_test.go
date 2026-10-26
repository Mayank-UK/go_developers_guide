package postsdata

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"guide/internal/core"
	"guide/internal/domain/posts"
)

// The switching facade's own pins: per-call resolution (a flip lands
// BETWEEN calls, never inside one), the honest degradation story (live
// mode against a down store surfaces the typed 503; fake mode keeps
// serving), and race-clean operation while flips and calls interleave.

// callCounter is a mutex-guarded method-name tally — race-clean under the
// facade's per-call delegation.
type callCounter struct {
	mu    sync.Mutex
	calls map[string]int
}

func (c *callCounter) add(method string) {
	c.mu.Lock()
	c.calls[method]++
	c.mu.Unlock()
}

func (c *callCounter) of(method string) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.calls[method]
}

func newCallCounter() *callCounter { return &callCounter{calls: make(map[string]int)} }

// countingRepository records which adapter answered, for delegation pins.
type countingRepository struct {
	label string
	calls *callCounter
}

func (c *countingRepository) List(ctx context.Context, query posts.ListQuery) (posts.PostPage, error) {
	c.calls.add("List")
	return posts.PostPage{Items: []posts.Post{{ID: c.label}}}, nil
}

func (c *countingRepository) GetByID(ctx context.Context, id string) (posts.Post, error) {
	c.calls.add("GetByID")
	return posts.Post{ID: c.label}, nil
}

func (c *countingRepository) Create(ctx context.Context, draft posts.Draft) (posts.Post, error) {
	c.calls.add("Create")
	return posts.Post{ID: c.label}, nil
}

func (c *countingRepository) Replace(ctx context.Context, id string, draft posts.Draft) (posts.Post, error) {
	c.calls.add("Replace")
	return posts.Post{ID: c.label}, nil
}

func (c *countingRepository) Delete(ctx context.Context, id string) error {
	c.calls.add("Delete")
	return nil
}

func TestFacadeAnswersFromFakeByDefaultAndLiveAfterTheFlip(t *testing.T) {
	t.Parallel()

	fakeCalls, liveCalls := newCallCounter(), newCallCounter()
	fake := &countingRepository{label: "fake", calls: fakeCalls}
	live := &countingRepository{label: "live", calls: liveCalls}
	scenario := core.NewScenarioStore()
	facade := NewSwitchingRepository(scenario, fake, live)

	ctx := context.Background()
	if post, err := facade.GetByID(ctx, "any"); err != nil || post.ID != "fake" {
		t.Fatalf("default mode must answer from the fake, got %q err %v", post.ID, err)
	}

	if _, err := scenario.Update(core.ScenarioPatch{DataMode: ptr(core.DataModeLive)}); err != nil {
		t.Fatalf("flip to live: %v", err)
	}
	for range 5 {
		if post, err := facade.GetByID(ctx, "any"); err != nil || post.ID != "live" {
			t.Fatalf("live mode must answer from the live store, got %q err %v", post.ID, err)
		}
	}

	if _, err := scenario.Update(core.ScenarioPatch{DataMode: ptr(core.DataModeFake)}); err != nil {
		t.Fatalf("flip back to fake: %v", err)
	}
	if post, err := facade.GetByID(ctx, "any"); err != nil || post.ID != "fake" {
		t.Fatalf("fake mode must answer from the fake again, got %q err %v", post.ID, err)
	}

	if fakeCalls.of("GetByID") != 2 || liveCalls.of("GetByID") != 5 {
		t.Errorf("delegation counts drifted: fake %d, live %d — want 2 and 5", fakeCalls.of("GetByID"), liveCalls.of("GetByID"))
	}
}

func TestFacadeDelegatesEveryVerb(t *testing.T) {
	t.Parallel()

	fakeCalls, liveCalls := newCallCounter(), newCallCounter()
	scenario := core.NewScenarioStore()
	facade := NewSwitchingRepository(scenario, &countingRepository{label: "fake", calls: fakeCalls}, &countingRepository{label: "live", calls: liveCalls})

	ctx := context.Background()
	draft := posts.Draft{Title: "t", Body: "b"}
	if _, err := facade.List(ctx, posts.ListQuery{Page: 1, Limit: 5, Sort: posts.SortNewest}); err != nil {
		t.Fatalf("list: %v", err)
	}
	if _, err := facade.Create(ctx, draft); err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := facade.GetByID(ctx, "x"); err != nil {
		t.Fatalf("get: %v", err)
	}
	if _, err := facade.Replace(ctx, "x", draft); err != nil {
		t.Fatalf("replace: %v", err)
	}
	if err := facade.Delete(ctx, "x"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	for _, method := range []string{"List", "Create", "GetByID", "Replace", "Delete"} {
		if fakeCalls.of(method) != 1 {
			t.Errorf("fake %s calls = %d, want 1", method, fakeCalls.of(method))
		}
		if liveCalls.of(method) != 0 {
			t.Errorf("live %s calls = %d, want 0 in fake mode", method, liveCalls.of(method))
		}
	}
}

// TestFacadeFlipsAreRaceClean interleaves flips with hammering under the
// race detector: the scenario store's snapshot read per call plus the two
// independent adapters must never produce a torn answer or a detector
// report — the same guarantee the running server needs while PATCH
// scenario lands mid-traffic.
func TestFacadeFlipsAreRaceClean(t *testing.T) {
	t.Parallel()

	scenario := core.NewScenarioStore()
	facade := NewSwitchingRepository(scenario, NewInMemoryRepository(), NewInMemoryRepository())

	var wg sync.WaitGroup
	for flipper := 0; flipper < 2; flipper++ {
		wg.Add(1)
		go func(flipper int) {
			defer wg.Done()
			mode := core.DataModeFake
			if flipper%2 == 1 {
				mode = core.DataModeLive
			}
			for range 50 {
				_, _ = scenario.Update(core.ScenarioPatch{DataMode: ptr(mode)})
			}
		}(flipper)
	}
	for caller := 0; caller < 4; caller++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx := context.Background()
			for range 50 {
				page, err := facade.List(ctx, posts.ListQuery{Page: 1, Limit: 5, Sort: posts.SortNewest})
				if err != nil {
					t.Errorf("list under flips: %v", err)
					return
				}
				if len(page.Items) > 5 {
					t.Errorf("a page of limit 5 carried %d items — a torn answer", len(page.Items))
					return
				}
				created, err := facade.Create(ctx, posts.Draft{Title: "race", Body: "b", Tags: []string{"x"}})
				if err != nil {
					t.Errorf("create under flips: %v", err)
					return
				}
				// A flip between the create and the delete can strand the
				// post in the OTHER store — the facade's documented
				// per-call semantics (demos flip between requests, never
				// mid-pair); the strict miss is the honest answer, so only
				// unexpected errors fail here.
				if err := facade.Delete(ctx, created.ID); err != nil && !errors.Is(err, core.ErrNotFound) {
					t.Errorf("delete under flips: %v", err)
					return
				}
			}
		}()
	}
	wg.Wait()
}

// TestFacadeSurfacesTheTypedUnavailableInLiveModeWhenTheStoreIsDown pins
// the honest degradation: live mode against a dead store is the typed 503
// (never a 500, never a hang) — and the moment the scenario returns to
// fake, every surface answers again. This is the exact behavior the
// running server shows when PostgreSQL is stopped mid-session. A closed
// pool is the deterministic "database is gone": every query fails
// instantly, no timeout waits.
func TestFacadeSurfacesTheTypedUnavailableInLiveModeWhenTheStoreIsDown(t *testing.T) {
	t.Parallel()

	pool, err := pgxpool.New(context.Background(), "postgres://postgres:guide@127.0.0.1:1/guide?sslmode=disable")
	if err != nil {
		t.Fatalf("build pool: %v", err)
	}
	pool.Close()

	scenario := core.NewScenarioStore()
	facade := NewSwitchingRepository(scenario, NewInMemoryRepository(), NewPostgresRepository(pool))
	ctx := context.Background()

	if _, err := scenario.Update(core.ScenarioPatch{DataMode: ptr(core.DataModeLive)}); err != nil {
		t.Fatalf("flip to live: %v", err)
	}
	if _, err := facade.List(ctx, posts.ListQuery{Page: 1, Limit: 10, Sort: posts.SortNewest}); !errors.Is(err, core.ErrServiceUnavailable) {
		t.Errorf("live list against a down store = %v, want the typed service_unavailable", err)
	}

	if _, err := scenario.Update(core.ScenarioPatch{DataMode: ptr(core.DataModeFake)}); err != nil {
		t.Fatalf("flip back to fake: %v", err)
	}
	if page, err := facade.List(ctx, posts.ListQuery{Page: 1, Limit: 10, Sort: posts.SortNewest}); err != nil || page.TotalItems != 6 {
		t.Errorf("fake mode must keep serving while the live store is down, got %+v err %v", page, err)
	}
}

func ptr(mode core.DataMode) *core.DataMode { return &mode }
