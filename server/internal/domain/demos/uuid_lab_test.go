package demos

import (
	"bytes"
	"errors"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"
)

// The UUIDv7 lab: these tests pin the generator's BIT LAYOUT against
// RFC 9562 (version nibble, variant bits, 48-bit timestamp), the canonical
// 36-character form, the two monotonicity guarantees (across and within
// milliseconds), collision-freedom at volume, and concurrency safety.

// fakeClock serves a frozen instant, then advances one millisecond per
// tick() — deterministic time for layout and ordering tests.
type fakeClock struct {
	now time.Time
	mu  sync.Mutex
}

func (c *fakeClock) tick() { c.mu.Lock(); c.now = c.now.Add(time.Millisecond); c.mu.Unlock() }

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func TestDBUUIDLabLayoutMatchesRFC9562(t *testing.T) {
	t.Parallel()

	// A fully deterministic value: ms 0x010203040506 (a known instant),
	// counter 0xABC, rand_b bytes 0xDE.. repeating.
	var randB [8]byte
	for i := range randB {
		randB[i] = byte(0xD0 + i)
	}
	value := formatUUID(uuidv7(0x010203040506, 0xABC, randB))

	if len(value) != 36 {
		t.Fatalf("length = %d, want 36", len(value))
	}
	// 48-bit timestamp in the first 12 hex digits: 010203040506.
	if got := value[0:8] + value[9:13]; got != "010203040506" {
		t.Errorf("timestamp hex = %q, want 010203040506", got)
	}
	// Third group starts with the version nibble 7, then the 12-bit
	// counter: 7ABC.
	if got := value[14:18]; got != "7abc" {
		t.Errorf("version+counter group = %q, want 7abc", got)
	}
	// Fourth group starts with variant bits 10 followed by the top 6 bits
	// of rand_b[0]: 0x80 | (0xD0 & 0x3F) = 0x90, then rand_b[1] = 0xD1.
	if got := value[19:23]; got != "90d1" {
		t.Errorf("variant+rand group = %q, want 90d1", got)
	}
	// Remaining 56 rand bits: d2 d3 d4 d5 d6 d7.
	if got := value[24:36]; got != "d2d3d4d5d6d7" {
		t.Errorf("rand_b tail = %q, want d2d3d4d5d6d7", got)
	}
	if value[8] != '-' || value[13] != '-' || value[18] != '-' || value[23] != '-' || strings.Count(value, "-") != 4 {
		t.Errorf("canonical form must hyphenate at 8-13-18-23 exactly: %q", value)
	}
}

func TestDBUUIDLabParseRoundTrips(t *testing.T) {
	t.Parallel()

	value := formatUUID(uuidv7(0x0192AC150211, 0x123, [8]byte{0xAA, 1, 2, 3, 4, 5, 6, 7}))
	parsed, err := parseUUID(value)
	if err != nil {
		t.Fatalf("parseUUID(%q): %v", value, err)
	}
	if formatUUID(parsed) != value {
		t.Errorf("round trip changed the value: %q → %q", value, formatUUID(parsed))
	}

	for _, bad := range []string{
		"",                                       // too short
		"0192ac150211-7abc-81ed-f1f2f3f4f5f6f7",  // hyphen misplaced
		"0192ac15-0211-7abc-81ed-f1f2f3f4f5f6f",  // too long
		"0192ac15-0211-7abc-81ed-f1f2f3f4f5f6gg", // not hex
	} {
		if _, err := parseUUID(bad); err == nil {
			t.Errorf("parseUUID(%q) must fail", bad)
		}
	}
}

func TestDBUUIDLabGeneratorIsMonotonicWithinAndAcrossMilliseconds(t *testing.T) {
	t.Parallel()

	clock := &fakeClock{now: time.UnixMilli(1_700_000_000_000)}
	gen := newUUIDv7Generator(bytes.NewReader(make([]byte, 1<<20)), clock.Now)

	var batch []string
	for i := 0; i < 5; i++ {
		value, err := gen.Next()
		if err != nil {
			t.Fatalf("Next within one millisecond: %v", err)
		}
		batch = append(batch, value)
	}
	for i := 1; i < len(batch); i++ {
		if batch[i-1] >= batch[i] {
			t.Fatalf("same-millisecond ids must increase lexically: %q then %q", batch[i-1], batch[i])
		}
	}

	// Advance the clock: the next id must be strictly greater even though
	// the fresh counter seed is random (the timestamp dominates).
	clock.tick()
	next, err := gen.Next()
	if err != nil {
		t.Fatalf("Next across milliseconds: %v", err)
	}
	if next <= batch[len(batch)-1] {
		t.Fatalf("new-millisecond id %q must exceed the previous millisecond's %q", next, batch[len(batch)-1])
	}
}

func TestDBUUIDLabGeneratorFailsOnStuckClockAtCounterOverflow(t *testing.T) {
	t.Parallel()

	// Frozen clock: the 12-bit counter exhausts after at most 4095 more
	// ids, and the stuck-clock guard must return an error instead of
	// hanging forever.
	clock := &fakeClock{now: time.UnixMilli(42)}
	gen := newUUIDv7Generator(bytes.NewReader(make([]byte, 1<<20)), clock.Now)

	var lastErr error
	for i := 0; i < seqMax+3; i++ {
		if _, err := gen.Next(); err != nil {
			lastErr = err
			break
		}
	}
	if lastErr == nil {
		t.Fatal("exhausting the counter under a frozen clock must surface the stuck-clock error")
	}
	if !strings.Contains(lastErr.Error(), "clock stuck") {
		t.Errorf("error = %v, want the stuck-clock diagnosis", lastErr)
	}
}

func TestDBUUIDLabGeneratorPropagatesRandomnessFailures(t *testing.T) {
	t.Parallel()

	gen := newUUIDv7Generator(errReader{}, time.Now)
	if _, err := gen.Next(); err == nil {
		t.Fatal("a failing entropy source must surface as an error, never as a zero-value uuid")
	}
}

// errReader always fails — the entropy source's outage path.
type errReader struct{}

func (errReader) Read([]byte) (int, error) { return 0, errors.New("entropy depleted") }

func TestDBUUIDLabPayloadProvesSortednessAndCollisionFreedom(t *testing.T) {
	t.Parallel()

	payload, err := BuildUUIDLabPayload(NewUUIDv7Generator())
	if err != nil {
		t.Fatalf("BuildUUIDLabPayload: %v", err)
	}

	if payload.Batch.Count != 8 || len(payload.Batch.GeneratedInOrder) != 8 {
		t.Fatalf("batch = %+v, want 8 members", payload.Batch)
	}
	if !payload.Batch.SortingMatchesGenerationOrder {
		t.Error("lexical order must equal generation order — the demo's headline property")
	}
	sortedCopy := append([]string(nil), payload.Batch.GeneratedInOrder...)
	sort.Strings(sortedCopy)
	if strings.Join(sortedCopy, ",") != strings.Join(payload.Batch.SortedLexically, ",") {
		t.Error("sortedLexically must be the sorted copy of generatedInOrder")
	}

	if payload.Collision.Generated != uuidLabCollisionVolume || payload.Collision.Distinct != payload.Collision.Generated ||
		!payload.Collision.CollisionFree {
		t.Errorf("collision check = %+v, want 10000 distinct of 10000", payload.Collision)
	}

	if payload.Anatomy.Version != 7 || payload.Anatomy.Length != 36 || payload.Anatomy.Variant == "" {
		t.Errorf("anatomy = %+v, want version 7, length 36, named variant", payload.Anatomy)
	}
	if payload.Anatomy.UnixMs <= 0 {
		t.Errorf("anatomy unixMs = %d, want the embedded timestamp", payload.Anatomy.UnixMs)
	}
	if payload.Anatomy.Value != payload.Batch.GeneratedInOrder[0] {
		t.Errorf("anatomy must dissect the batch's first value: %q vs %q", payload.Anatomy.Value, payload.Batch.GeneratedInOrder[0])
	}

	if len(payload.Layout) != 5 || payload.Layout[0].Field != "unix_ts_ms" || payload.Layout[0].Bits != 48 {
		t.Errorf("layout = %+v, want the five RFC fields starting with 48-bit unix_ts_ms", payload.Layout)
	}
	if len(payload.Comparison) != 3 || payload.Comparison[0].Strategy != "bigserial" ||
		payload.Comparison[2].Strategy != "uuidv4 (gen_random_uuid())" {
		t.Errorf("comparison = %+v, want bigserial / uuidv7 / uuidv4 rows", payload.Comparison)
	}
}

func TestDBUUIDLabGeneratorIsConcurrencySafeAndOrdered(t *testing.T) {
	t.Parallel()

	// 8 goroutines × 64 ids over a real clock: -race watches the shared
	// lastMS/seq, and the union must still be free of duplicates.
	gen := NewUUIDv7Generator()
	const goroutines, each = 8, 64

	var mu sync.Mutex
	seen := make(map[string]bool)
	var wg sync.WaitGroup
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < each; i++ {
				value, err := gen.Next()
				if err != nil {
					t.Errorf("Next: %v", err)
					return
				}
				mu.Lock()
				seen[value] = true
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	if len(seen) != goroutines*each {
		t.Errorf("distinct ids = %d, want %d — concurrent minting duplicated values", len(seen), goroutines*each)
	}
}

func TestDBUUIDLabCanonicalFormHoldsAtVolume(t *testing.T) {
	t.Parallel()

	// Every production-minted value (real clock, real crypto/rand) must
	// parse back cleanly — the round trip at volume, not just once.
	gen := NewUUIDv7Generator()
	for i := 0; i < 1000; i++ {
		value, err := gen.Next()
		if err != nil {
			t.Fatalf("Next: %v", err)
		}
		bits, err := parseUUID(value)
		if err != nil {
			t.Fatalf("value %d %q does not parse: %v", i, value, err)
		}
		if version := bits[6] >> 4; version != 7 {
			t.Fatalf("value %d %q carries version %d, want 7", i, value, version)
		}
		if variant := bits[8] >> 6; variant != 0b10 {
			t.Fatalf("value %d %q carries variant %02b, want 10", i, value, variant)
		}
	}
}
