package demos

import (
	"encoding/json"
	"strings"
	"testing"
)

// The caching lab's payload builder — pure shaping and deriving, pinned the
// same way every demos builder is pinned: rows pass through, proofs derive
// from the run's own numbers (never assume the happy shape — the builder
// must say so when a census disagrees), nil slices marshal as [], and the
// serialized payload is byte-stable for identical runs, which is the ETag
// story's premise.

func cachingLabFixtureRun() CachingLabRun {
	return CachingLabRun{
		TTLSeconds: int(CachingLabTTL.Seconds()),
		Beats: []CachingLabBeat{
			{
				ID: "cold-read", Title: "The first read misses",
				Script:        []string{`GetByID("p-1")`},
				Outcome:       CachingOutcomeMiss,
				ObservedValue: "Why Go for backends",
				Hits:          0, Misses: 1, StoreReads: 1,
			},
			{
				ID: "warm-read", Title: "The second read hits",
				Script:        []string{`GetByID("p-1")`},
				Outcome:       CachingOutcomeHit,
				ObservedValue: "Why Go for backends",
				Hits:          1, Misses: 1, StoreReads: 1,
			},
			{
				ID: "write-invalidation", Title: "A write retires id and pages",
				Script: []string{
					`Replace("p-1", draft)`,
					`GetByID("p-1")`,
					`List(page 1)`,
				},
				Outcome:       CachingOutcomeInvalidated,
				ObservedValue: CachingLabEditedTitle,
				Hits:          1, Misses: 3, StoreReads: 4,
			},
			{
				ID: "live-only-miss", Title: "The typed miss surfaces",
				Script:        []string{`GetByID("p-7") // fake mode`},
				Outcome:       CachingOutcomeNotFound,
				ObservedValue: "",
				Hits:          9, Misses: 10, StoreReads: 10,
			},
		},
		Herd: CachingLabHerd{
			ID: CachingLabHerdID, Readers: CachingLabHerdReaders,
			StoreReads: 1, SameAnswer: true, Answer: "ServeMux patterns in Go 1.22",
		},
		Swap: CachingLabSwap{
			LiveOnlyID: CachingLabLiveOnlyID, LiveOnlyTitle: CachingLabLiveOnlyTitle,
			NotFoundID: CachingLabLiveOnlyID, NotFoundSurfaced: true,
			FakeTitle: CachingLabEditedTitle, LiveTitle: "Why Go for backends",
			BackTitle: CachingLabEditedTitle,
		},
		Stats: CachingLabStats{
			Hits: 13, Misses: 10, Expiries: 2, Invalidations: 2,
			StoreReads: 10, Entries: 5,
		},
	}
}

func TestCachingLabBuilderPassesBeatsThroughAndDerivesDetails(t *testing.T) {
	payload := BuildCachingLabPayload(cachingLabFixtureRun())

	if len(payload.Beats) != 4 {
		t.Fatalf("beats = %d, want 4", len(payload.Beats))
	}
	cold := payload.Beats[0]
	if cold.ID != "cold-read" || cold.Outcome != string(CachingOutcomeMiss) {
		t.Errorf("cold beat = %+v", cold)
	}
	if !strings.Contains(cold.Detail, "the store answered") {
		t.Errorf("miss detail must say the store answered: %q", cold.Detail)
	}
	warm := payload.Beats[1]
	if !strings.Contains(warm.Detail, "from memory") {
		t.Errorf("hit detail must say it was served from memory: %q", warm.Detail)
	}
	invalidated := payload.Beats[2]
	if !strings.Contains(invalidated.Detail, "retired") {
		t.Errorf("invalidation detail must mention the retire: %q", invalidated.Detail)
	}
	notFound := payload.Beats[3]
	if !strings.Contains(notFound.Detail, "never cached") {
		t.Errorf("not-found detail must say errors are never cached: %q", notFound.Detail)
	}
}

func TestCachingLabBuilderDerivesHerdAndStatsProofsFromTheCensus(t *testing.T) {
	payload := BuildCachingLabPayload(cachingLabFixtureRun())

	if !strings.Contains(payload.Herd.Proof, "exactly 1 store read") {
		t.Errorf("herd proof must derive the one-read census: %q", payload.Herd.Proof)
	}
	if !strings.Contains(payload.Stats.Proof, "equal by construction") {
		t.Errorf("stats proof must derive the misses==store-reads invariant: %q", payload.Stats.Proof)
	}
	if !strings.Contains(payload.Swap.Proof, "salted into every key") {
		t.Errorf("swap proof must name the partition salt: %q", payload.Swap.Proof)
	}
	if !strings.Contains(payload.Reasoning, "ETag") {
		t.Errorf("reasoning must close the ETag loop: %q", payload.Reasoning)
	}
}

func TestCachingLabBuilderReportsAHonestCensusWhenThePatternFails(t *testing.T) {
	run := cachingLabFixtureRun()
	run.Herd.StoreReads = 7
	run.Herd.SameAnswer = false
	run.Swap.NotFoundSurfaced = false
	run.Stats.Misses = 11 // no longer equals StoreReads (10)

	payload := BuildCachingLabPayload(run)
	if strings.Contains(payload.Herd.Proof, "exactly 1 store read") {
		t.Errorf("a failed herd must not read as coalesced: %q", payload.Herd.Proof)
	}
	if payload.Swap.Proof != "The typed miss did not surface — the census reports the gap rather than papering over it." {
		t.Errorf("swap proof must report the gap: %q", payload.Swap.Proof)
	}
	if !strings.Contains(payload.Stats.Proof, "a defect to find") {
		t.Errorf("unequal scoreboard must read as a defect: %q", payload.Stats.Proof)
	}
}

func TestCachingLabPayloadIsByteStableAndNeverNull(t *testing.T) {
	run := cachingLabFixtureRun()
	first, err := json.Marshal(BuildCachingLabPayload(run))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	second, err := json.Marshal(BuildCachingLabPayload(cachingLabFixtureRun()))
	if err != nil {
		t.Fatalf("marshal again: %v", err)
	}
	if string(first) != string(second) {
		t.Error("identical runs must serialize identically — the ETag premise")
	}

	empty := BuildCachingLabPayload(CachingLabRun{TTLSeconds: 5})
	encoded, err := json.Marshal(empty)
	if err != nil {
		t.Fatalf("marshal empty: %v", err)
	}
	if !strings.Contains(string(encoded), `"beats":[]`) {
		t.Errorf("empty beats must marshal as [], never null: %s", encoded)
	}
	if strings.Contains(string(encoded), ":null") {
		t.Errorf("no field may marshal as null: %s", encoded)
	}
}
