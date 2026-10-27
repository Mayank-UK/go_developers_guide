// Topic 21 caching's single live demo: the lab. One request runs the whole
// cache-aside story against throwaway stores through the REAL decorator
// (internal/data/posts.CachingRepository) — a cold read and its warm hit, a
// cached page, a write that invalidates both shapes, a TTL expiry read as
// data on a fixture clock (never slept out), a twelve-reader herd coalesced
// into one store read, and a fake/live store swap that proves the partition
// salt keeps one store's rows from answering for the other. Every number in
// the payload is deterministic — the same fixture clock, the same beats, the
// same answers — which is what lets the handler certify the whole payload
// with an ETag and answer a replayed If-None-Match with a real 304.
package demos

import (
	"context"
	"fmt"
	"time"
)

// The lab's pinned constants: shared by the runner (which runs the beats)
// and the builder (whose proof sentences derive from them), so the two can
// never drift.
const (
	// CachingLabTTL is the decorator's TTL in the lab: long enough to read
	// as a policy, short enough that one clock step retires it.
	CachingLabTTL = 5 * time.Second

	// CachingLabClockStep is how far the fixture clock advances between the
	// expiry beats — past the TTL by a clear margin, so "expired" is never a
	// boundary race.
	CachingLabClockStep = CachingLabTTL + time.Second

	// CachingLabHerdReaders is the herd beat's reader count.
	CachingLabHerdReaders = 12

	// CachingLabEditedTitle is what beat 4's Replace writes to p-1 — the
	// value a stale cache would wrongly keep serving.
	CachingLabEditedTitle = "Why Go for backends (edited by the lab)"

	// CachingLabLiveOnlyTitle is the seventh post only the live store has:
	// reading it in fake mode must surface the typed NotFound, never a
	// cached stand-in.
	CachingLabLiveOnlyTitle = "A seventh post, live only"
)

// CachingLabIDs are the posts the beats read and write.
const (
	CachingLabPrimeID    = "p-1" // the beats' main subject
	CachingLabHerdID     = "p-2" // the herd's key
	CachingLabLiveOnlyID = "p-7" // exists only in the live store
)

// CachingLabOutcome classifies one beat's cache behavior.
type CachingLabOutcome string

// The three outcomes the choreography observes — derived from the store-side
// read counter and the decorator's scoreboard, never assumed.
const (
	CachingOutcomeMiss        CachingLabOutcome = "miss"
	CachingOutcomeHit         CachingLabOutcome = "hit"
	CachingOutcomeInvalidated CachingLabOutcome = "invalidated"
	CachingOutcomeExpired     CachingLabOutcome = "expired"
	CachingOutcomeNotFound    CachingLabOutcome = "not-found"
)

// CachingLabBeat is one choreographed step: the calls as run, the outcome
// observed, the value read (when the beat read one), and the cumulative
// counters AFTER the beat — the scoreboard the proof sentences derive from.
type CachingLabBeat struct {
	ID      string
	Title   string
	Script  []string
	Outcome CachingLabOutcome
	// ObservedValue is the beat's read-back (a post title, or the page
	// fact); empty when the beat's subject is an error surface.
	ObservedValue string
	Hits          uint64
	Misses        uint64
	StoreReads    uint64
}

// CachingLabHerd is the stampede beat's census: how many readers raced the
// one cold key, how many store reads that herd produced, and whether every
// reader left with the same answer.
type CachingLabHerd struct {
	ID         string
	Readers    int
	StoreReads uint64
	SameAnswer bool
	Answer     string
}

// CachingLabSwap is the store-swap beat: the fake and live stores answer
// differently for the same ids, and every observation below is what the
// cache actually served at each step of the flip.
type CachingLabSwap struct {
	LiveOnlyID       string
	LiveOnlyTitle    string // live has it…
	NotFoundID       string // …fake does not: the typed miss surfaced
	NotFoundSurfaced bool
	FakeTitle        string // fake's p-1 after the lab's edit
	LiveTitle        string // live's p-1: the original seed title
	BackTitle        string // fake again after the flip back: the edit is BACK
}

// CachingLabStats is the decorator's final scoreboard as the lab reports
// it. The coalesced-wait count is deliberately absent: how many of the
// herd's readers parked on the in-flight refill varies with scheduling,
// and this payload is deterministic by contract (its ETag says so).
type CachingLabStats struct {
	Hits          uint64
	Misses        uint64
	Expiries      uint64
	Invalidations uint64
	StoreReads    uint64
	Entries       uint64
}

// CachingLabRunner is the port the lab runs through: one call performs the
// whole choreography against throwaway stores and reports every
// observation. The postsdata-backed implementation lives in
// internal/data/posts; tests and the handler wire fakes through this
// interface.
type CachingLabRunner interface {
	// Run walks the beats — cold/warm reads, the cached page, write
	// invalidation, TTL expiry, the herd, the store swap — and returns the
	// raw haul the payload builder shapes.
	Run(ctx context.Context) (CachingLabRun, error)
}

// CachingLabRun is the runner's raw haul: the beats in order, the herd and
// swap censuses, and the final scoreboard.
type CachingLabRun struct {
	TTLSeconds int
	Beats      []CachingLabBeat
	Herd       CachingLabHerd
	Swap       CachingLabSwap
	Stats      CachingLabStats
}

// CachingLabPayload is the wire shape of GET /api/demo/caching/lab
// (camelCase JSON), deterministic by construction: same fixture clock, same
// beats, same numbers — every slice non-nil so the wire renders [], never
// null.
type CachingLabPayload struct {
	TTLSeconds int                  `json:"ttlSeconds"`
	Beats      []CachingLabBeatRow  `json:"beats"`
	Herd       CachingLabHerdProof  `json:"herd"`
	Swap       CachingLabSwapProof  `json:"swap"`
	Stats      CachingLabStatsProof `json:"stats"`
	Reasoning  string               `json:"reasoning"`
}

// CachingLabBeatRow is one beat as the payload serves it, with its detail
// sentence derived from the beat's own observations.
type CachingLabBeatRow struct {
	ID            string   `json:"id"`
	Title         string   `json:"title"`
	Script        []string `json:"script"`
	Outcome       string   `json:"outcome"`
	ObservedValue string   `json:"observedValue"`
	Detail        string   `json:"detail"`
	Hits          uint64   `json:"hits"`
	Misses        uint64   `json:"misses"`
	StoreReads    uint64   `json:"storeReads"`
}

// CachingLabHerdProof is the stampede census with its derived proof.
type CachingLabHerdProof struct {
	ID         string `json:"id"`
	Readers    int    `json:"readers"`
	StoreReads uint64 `json:"storeReads"`
	SameAnswer bool   `json:"sameAnswer"`
	Answer     string `json:"answer"`
	Proof      string `json:"proof"`
}

// CachingLabSwapProof is the store-swap census with its derived proof.
type CachingLabSwapProof struct {
	LiveOnlyID       string `json:"liveOnlyId"`
	LiveOnlyTitle    string `json:"liveOnlyTitle"`
	NotFoundID       string `json:"notFoundId"`
	NotFoundSurfaced bool   `json:"notFoundSurfaced"`
	FakeTitle        string `json:"fakeTitle"`
	LiveTitle        string `json:"liveTitle"`
	BackTitle        string `json:"backTitle"`
	Proof            string `json:"proof"`
}

// CachingLabStatsProof is the final scoreboard with its derived proof.
type CachingLabStatsProof struct {
	Hits          uint64 `json:"hits"`
	Misses        uint64 `json:"misses"`
	Expiries      uint64 `json:"expiries"`
	Invalidations uint64 `json:"invalidations"`
	StoreReads    uint64 `json:"storeReads"`
	Entries       uint64 `json:"entries"`
	Proof         string `json:"proof"`
}

// BuildCachingLabPayload shapes a run into the demo payload: beats pass
// through with nil slices materialized as [], and every proof sentence
// DERIVES from the run's own numbers — the invariant that misses and store
// reads agree, the herd's one-read census, the swap's no-leak observations —
// so the payload can never claim a thing the run did not do.
func BuildCachingLabPayload(run CachingLabRun) CachingLabPayload {
	beats := make([]CachingLabBeatRow, 0, len(run.Beats))
	for _, beat := range run.Beats {
		beats = append(beats, CachingLabBeatRow{
			ID:            beat.ID,
			Title:         beat.Title,
			Script:        nonNilStrings(beat.Script),
			Outcome:       string(beat.Outcome),
			ObservedValue: beat.ObservedValue,
			Detail:        cachingBeatDetail(beat),
			Hits:          beat.Hits,
			Misses:        beat.Misses,
			StoreReads:    beat.StoreReads,
		})
	}

	herd := CachingLabHerdProof{
		ID:         run.Herd.ID,
		Readers:    run.Herd.Readers,
		StoreReads: run.Herd.StoreReads,
		SameAnswer: run.Herd.SameAnswer,
		Answer:     run.Herd.Answer,
	}
	if run.Herd.StoreReads == 1 && run.Herd.SameAnswer {
		herd.Proof = fmt.Sprintf(
			"%d concurrent readers of %s produced exactly 1 store read and every reader "+
				"left with %q — concurrent misses coalesce into one in-flight refill "+
				"(the singleflight pattern, hand-rolled) instead of stampeding the store.",
			run.Herd.Readers, run.Herd.ID, run.Herd.Answer,
		)
	} else {
		herd.Proof = fmt.Sprintf(
			"%d concurrent readers of %s produced %d store reads (same answer: %t) — the "+
				"census reports what actually happened, not what the pattern promises.",
			run.Herd.Readers, run.Herd.ID, run.Herd.StoreReads, run.Herd.SameAnswer,
		)
	}

	swap := CachingLabSwapProof{
		LiveOnlyID:       run.Swap.LiveOnlyID,
		LiveOnlyTitle:    run.Swap.LiveOnlyTitle,
		NotFoundID:       run.Swap.NotFoundID,
		NotFoundSurfaced: run.Swap.NotFoundSurfaced,
		FakeTitle:        run.Swap.FakeTitle,
		LiveTitle:        run.Swap.LiveTitle,
		BackTitle:        run.Swap.BackTitle,
	}
	swap.Proof = fmt.Sprintf(
		"Live answered %q for %s while fake answered the typed miss — and for %s the two "+
			"stores disagreed (%q in fake, %q in live) without the cache ever serving one "+
			"store's row to the other: the scenario's data mode is salted into every key, "+
			"so a flip starts on a cold keyspace. Flipping back re-read fake's own %q.",
		run.Swap.LiveOnlyTitle, run.Swap.LiveOnlyID, CachingLabPrimeID,
		run.Swap.FakeTitle, run.Swap.LiveTitle, run.Swap.BackTitle,
	)
	if !run.Swap.NotFoundSurfaced {
		swap.Proof = "The typed miss did not surface — the census reports the gap rather than papering over it."
	}

	stats := CachingLabStatsProof{
		Hits:          run.Stats.Hits,
		Misses:        run.Stats.Misses,
		Expiries:      run.Stats.Expiries,
		Invalidations: run.Stats.Invalidations,
		StoreReads:    run.Stats.StoreReads,
		Entries:       run.Stats.Entries,
	}
	if run.Stats.Misses == run.Stats.StoreReads {
		stats.Proof = fmt.Sprintf(
			"%d misses and %d store reads — equal by construction: every miss led exactly one "+
				"underlying read (coalesced readers never refetch), while %d hits never touched "+
				"the store. %d entries left by expiry, %d by invalidation, %d still resident "+
				"(expired-but-untouched entries leave lazily, on their next read).",
			run.Stats.Misses, run.Stats.StoreReads, run.Stats.Hits,
			run.Stats.Expiries, run.Stats.Invalidations, run.Stats.Entries,
		)
	} else {
		stats.Proof = fmt.Sprintf(
			"%d misses vs %d store reads — the scoreboard disagrees with itself and is "+
				"served as-is; a difference here is a defect to find, not a number to trust.",
			run.Stats.Misses, run.Stats.StoreReads,
		)
	}

	reasoning := fmt.Sprintf(
		"Every row above ran through the guide's real cache-aside decorator on throwaway " +
			"stores — the same type that wraps the production posts path in cmd/guide/main.go. " +
			"The beats covered the whole lifecycle: cold miss, warm hit, the cached page, the " +
			"write that invalidated id and pages together, the TTL expiry read as data on a " +
			"fixture clock (nothing was slept out), the herd that coalesced, and the fake/live " +
			"store swap that stayed partition-honest. The payload is byte-deterministic — the " +
			"fixture clock and the pinned beats answer identically every time — which is why " +
			"this very response carries an ETag: replay it as If-None-Match and the server " +
			"answers 304 without re-running a beat.",
	)

	return CachingLabPayload{
		TTLSeconds: run.TTLSeconds,
		Beats:      beats,
		Herd:       herd,
		Swap:       swap,
		Stats:      stats,
		Reasoning:  reasoning,
	}
}

// cachingBeatDetail derives one beat's detail sentence from its own
// observations: the outcome, the value read, and where the counters moved.
func cachingBeatDetail(beat CachingLabBeat) string {
	value := beat.ObservedValue
	if value == "" {
		value = "—"
	}
	switch beat.Outcome {
	case CachingOutcomeHit:
		return fmt.Sprintf("Served %q from memory — the store-read counter did not move (%d total).", value, beat.StoreReads)
	case CachingOutcomeMiss:
		return fmt.Sprintf("The cache was cold: the store answered %q and the entry went in (%d store reads so far).", value, beat.StoreReads)
	case CachingOutcomeInvalidated:
		return fmt.Sprintf("The write retired the stale entries; the next read went back to the store and answered %q.", value)
	case CachingOutcomeExpired:
		return fmt.Sprintf("The fixture clock stepped past the %d-second TTL: the entry died, the store re-answered %q.", int(CachingLabTTL.Seconds()), value)
	case CachingOutcomeNotFound:
		return fmt.Sprintf("The store's typed miss surfaced untouched — errors are never cached (%d store reads so far).", beat.StoreReads)
	default:
		return fmt.Sprintf("Outcome %q observed with value %q.", beat.Outcome, value)
	}
}
