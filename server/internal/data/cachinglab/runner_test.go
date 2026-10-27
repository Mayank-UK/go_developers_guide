package cachinglab

import (
	"context"
	"encoding/json"
	"testing"

	"guide/internal/domain/demos"
)

// The caching lab's choreography, end to end: two Runs must answer
// IDENTICALLY (the ETag premise), every beat's outcome must be the one the
// teaching claims, and the census invariants must hold — misses equal store
// reads, the herd cost exactly one read, the swap never leaked one store's
// row to the other, and the typed miss surfaced untouched.

func TestCachingLabRunProducesTheExpectedChoreography(t *testing.T) {
	run, err := New().Run(context.Background())
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	wantOutcomes := map[string]demos.CachingLabOutcome{
		"cold-read":          demos.CachingOutcomeMiss,
		"warm-read":          demos.CachingOutcomeHit,
		"page-fill":          demos.CachingOutcomeMiss,
		"write-invalidation": demos.CachingOutcomeInvalidated,
		"ttl-expiry":         demos.CachingOutcomeExpired,
	}
	if len(run.Beats) != len(wantOutcomes) {
		t.Fatalf("beats = %d, want %d", len(run.Beats), len(wantOutcomes))
	}
	for _, beat := range run.Beats {
		want, ok := wantOutcomes[beat.ID]
		if !ok {
			t.Fatalf("unexpected beat id %q", beat.ID)
		}
		if beat.Outcome != want {
			t.Errorf("beat %q outcome = %q, want %q", beat.ID, beat.Outcome, want)
		}
	}

	if run.Beats[0].ObservedValue != "Why Go for backends" {
		t.Errorf("cold read = %q, want the seed title", run.Beats[0].ObservedValue)
	}
	if run.Beats[3].ObservedValue != demos.CachingLabEditedTitle {
		t.Errorf("post-edit read = %q, want %q", run.Beats[3].ObservedValue, demos.CachingLabEditedTitle)
	}
	if run.Beats[4].ObservedValue != demos.CachingLabEditedTitle {
		t.Errorf("post-TTL read = %q, want the edit to survive its own expiry", run.Beats[4].ObservedValue)
	}
}

func TestCachingLabCensusInvariantsHold(t *testing.T) {
	run, err := New().Run(context.Background())
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	if run.Herd.Readers != demos.CachingLabHerdReaders || run.Herd.StoreReads != 1 || !run.Herd.SameAnswer {
		t.Errorf("herd census = %+v, want %d readers / 1 store read / same answer",
			run.Herd, demos.CachingLabHerdReaders)
	}
	if run.Herd.Answer != "ServeMux patterns in Go 1.22" {
		t.Errorf("herd answer = %q, want the p-2 seed title", run.Herd.Answer)
	}

	if !run.Swap.NotFoundSurfaced {
		t.Error("the fake store's typed miss must surface for the live-only post")
	}
	if run.Swap.FakeTitle == run.Swap.LiveTitle {
		t.Errorf("stores did not diverge: %q", run.Swap.FakeTitle)
	}
	if run.Swap.FakeTitle != demos.CachingLabEditedTitle || run.Swap.LiveTitle != "Why Go for backends" {
		t.Errorf("swap titles drifted: fake %q, live %q", run.Swap.FakeTitle, run.Swap.LiveTitle)
	}
	if run.Swap.BackTitle != demos.CachingLabEditedTitle {
		t.Errorf("flip back served %q, want the fake store's edited title", run.Swap.BackTitle)
	}

	if run.Stats.Misses != run.Stats.StoreReads {
		t.Errorf("census broke: %d misses vs %d store reads", run.Stats.Misses, run.Stats.StoreReads)
	}
	if run.Stats.Hits == 0 || run.Stats.Expiries == 0 || run.Stats.Invalidations == 0 || run.Stats.Entries == 0 {
		t.Errorf("scoreboard left a lifecycle stage unproven: %+v", run.Stats)
	}
}

func TestCachingLabTwoRunsSerializeIdentically(t *testing.T) {
	ctx := context.Background()
	first, err := New().Run(ctx)
	if err != nil {
		t.Fatalf("first run: %v", err)
	}
	second, err := New().Run(ctx)
	if err != nil {
		t.Fatalf("second run: %v", err)
	}

	firstJSON, err := json.Marshal(first)
	if err != nil {
		t.Fatalf("marshal first: %v", err)
	}
	secondJSON, err := json.Marshal(second)
	if err != nil {
		t.Fatalf("marshal second: %v", err)
	}
	if string(firstJSON) != string(secondJSON) {
		t.Errorf("two runs differ — the ETag premise is broken:\n%s\n%s", firstJSON, secondJSON)
	}
}
