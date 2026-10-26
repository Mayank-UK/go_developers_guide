// Topic 16 sql-schema's second live demo: the UUID lab. The guide's
// primary keys are UUIDv7 values, and this file is the real generator the
// server would mint them with — hand-rolled on crypto/rand with the bit
// layout spelled out, deliberately NOT a dependency: 128 bits, four fields,
// one formatUUID. google/uuid earns its keep in many codebases; here the
// point is that v7 is simple enough to own, and owning it makes the layout
// teachable. The payload proves the properties that matter: lexical order
// equals generation order (so a btree index gets time locality), no
// collisions at volume, and the trade-off against sequential ids in the
// comparison table.
package demos

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"io"
	"sort"
	"sync"
	"time"
)

// UUIDv7 sizes and masks — RFC 9562 §5.7's layout, in constants.
const (
	// uuidCanonicalLength is the canonical 8-4-4-4-12 form: 32 hex digits
	// + 4 hyphens.
	uuidCanonicalLength = 36
	// seqMask masks rand_a: 12 bits this guide uses as an
	// intra-millisecond counter (RFC 9562's "counter method" monotonicity).
	seqMask = 0x0FFF
	// seqMax is the last 12-bit counter value.
	seqMax = 0x0FFF
	// version7 is the nibble that names the version in every UUID.
	version7 = 0x7
	// variantRFC4122 is the two bits 10 that mark an RFC 4122/9562 UUID.
	variantRFC4122 = 0b10
	// seqSpinLimit bounds the wait for the next millisecond when the
	// counter overflows: a live clock crosses the boundary almost
	// immediately; a stuck one must fail, not hang.
	seqSpinLimit = 1 << 13
)

// uuidLabBatchSize is the ordered batch the demo shows, and
// uuidLabCollisionVolume is the collision check's size — fixed counts so
// the payload's shape is stable while the values it carries vary.
const (
	uuidLabBatchSize       = 8
	uuidLabCollisionVolume = 10000
	uuidLabRandomnessBits  = 74 // 12 counter bits seeded per ms + 62 rand_b bits
)

// uuidv7 lays out one UUIDv7 from its parts: the 48-bit Unix millisecond
// timestamp, the 12-bit counter (rand_a), and 62 random bits (rand_b —
// eight bytes, of whose first byte only the low 6 bits are used). Every
// field lands in RFC 9562's position: version nibble 0b0111 in bits 48-51,
// variant 0b10 in bits 64-65.
func uuidv7(unixMs uint64, seq uint16, randB [8]byte) [16]byte {
	var u [16]byte
	u[0] = byte(unixMs >> 40)
	u[1] = byte(unixMs >> 32)
	u[2] = byte(unixMs >> 24)
	u[3] = byte(unixMs >> 16)
	u[4] = byte(unixMs >> 8)
	u[5] = byte(unixMs)
	u[6] = version7<<4 | byte(seq>>8)&0x0F
	u[7] = byte(seq)
	u[8] = variantRFC4122<<6 | randB[0]&0x3F
	copy(u[9:], randB[1:])
	return u
}

// formatUUID renders the canonical 36-character 8-4-4-4-12 form.
func formatUUID(u [16]byte) string {
	const hexDigits = "0123456789abcdef"
	var out [uuidCanonicalLength]byte
	pos := 0
	for i, b := range u {
		if i == 4 || i == 6 || i == 8 || i == 10 {
			out[pos] = '-'
			pos++
		}
		out[pos] = hexDigits[b>>4]
		out[pos+1] = hexDigits[b&0x0F]
		pos += 2
	}
	return string(out[:])
}

// parseUUID reads the canonical form back into bytes — the round-trip the
// demo's anatomy dissection (and the tests) rely on.
func parseUUID(s string) ([16]byte, error) {
	var u [16]byte
	if len(s) != uuidCanonicalLength {
		return u, fmt.Errorf("uuid %q: length %d, want %d", s, len(s), uuidCanonicalLength)
	}
	for _, pos := range [4]int{8, 13, 18, 23} {
		if s[pos] != '-' {
			return u, fmt.Errorf("uuid %q: expected '-' at position %d", s, pos)
		}
	}
	hexIdx := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '-' {
			continue
		}
		value, ok := hexNibble(s[i])
		if !ok {
			return u, fmt.Errorf("uuid %q: %q is not a hex digit", s, string(s[i]))
		}
		if hexIdx%2 == 0 {
			u[hexIdx/2] = value << 4
		} else {
			u[hexIdx/2] |= value
		}
		hexIdx++
	}
	return u, nil
}

// hexNibble decodes one ASCII hex digit.
func hexNibble(c byte) (byte, bool) {
	switch {
	case c >= '0' && c <= '9':
		return c - '0', true
	case c >= 'a' && c <= 'f':
		return c - 'a' + 10, true
	case c >= 'A' && c <= 'F':
		return c - 'A' + 10, true
	default:
		return 0, false
	}
}

// UUIDv7Generator mints UUIDv7 values with two monotonicity guarantees:
// ids from DIFFERENT milliseconds order by time (the timestamp leads), and
// ids from the SAME millisecond order by their 12-bit counter — seeded
// randomly per millisecond, incremented per id, and on overflow the
// generator waits for the next millisecond rather than wrap (wrapping
// would break the ordering the whole scheme exists for). The mutex makes
// those guarantees hold across goroutines; randomness and the clock are
// injected, so tests can pin the bit layout without sleeping.
type UUIDv7Generator struct {
	mu     sync.Mutex
	random io.Reader
	now    func() time.Time
	lastMS int64
	seq    uint16
}

// NewUUIDv7Generator builds the production generator: crypto/rand for the
// 74 random bits, the wall clock for timestamps.
func NewUUIDv7Generator() *UUIDv7Generator {
	return newUUIDv7Generator(rand.Reader, time.Now)
}

func newUUIDv7Generator(random io.Reader, now func() time.Time) *UUIDv7Generator {
	return &UUIDv7Generator{random: random, now: now, lastMS: -1}
}

// Next mints one canonical UUIDv7 string.
func (g *UUIDv7Generator) Next() (string, error) {
	unixMs, seq, err := g.reserve()
	if err != nil {
		return "", err
	}
	var randB [8]byte
	if _, err := io.ReadFull(g.random, randB[:]); err != nil {
		return "", fmt.Errorf("uuidv7 randomness: %w", err)
	}
	return formatUUID(uuidv7(unixMs, seq, randB)), nil
}

// reserve picks the (millisecond, counter) pair under the lock — the whole
// monotonicity contract in one place. The zero counter is reserved as the
// "not yet seeded" state: a fresh millisecond seeds randomly in [1, seqMax].
func (g *UUIDv7Generator) reserve() (uint64, uint16, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	for {
		unixMs := g.now().UnixMilli()
		switch {
		case unixMs > g.lastMS:
			var seed [2]byte
			if _, err := io.ReadFull(g.random, seed[:]); err != nil {
				return 0, 0, fmt.Errorf("uuidv7 counter seed: %w", err)
			}
			// Random start in [1, seqMax]: 0 stays reserved as "not seeded",
			// and the top of the range leaves room for increments before
			// the overflow wait.
			g.lastMS = unixMs
			g.seq = 1 + binary.BigEndian.Uint16(seed[:])%seqMax
			return uint64(unixMs), g.seq, nil
		case g.seq < seqMax:
			g.seq++
			return uint64(unixMs), g.seq, nil
		default:
			// Counter exhausted in this millisecond: spin until the clock
			// crosses into the next one, or fail loudly on a stuck clock.
			for i := 0; i < seqSpinLimit && g.now().UnixMilli() == g.lastMS; i++ {
				time.Sleep(50 * time.Microsecond)
			}
			if g.now().UnixMilli() == g.lastMS {
				return 0, 0, fmt.Errorf("uuidv7: clock stuck at millisecond %d with counter exhausted", g.lastMS)
			}
		}
	}
}

// UUIDLayoutRow names one field of the v7 layout — the anatomy the demo
// teaches field by field.
type UUIDLayoutRow struct {
	Field    string `json:"field"`
	Bits     int    `json:"bits"`
	Contents string `json:"contents"`
}

// UUIDAnatomy dissects one freshly minted value: its string form, the
// timestamp buried in its first 48 bits, the version and variant nibbles,
// and the canonical length (a stable 36 for any value).
type UUIDAnatomy struct {
	Value   string `json:"value"`
	UnixMs  int64  `json:"unixMs"`
	Version int    `json:"version"`
	Variant string `json:"variant"`
	Length  int    `json:"length"`
}

// UUIDBatch is the sortedness proof: a batch minted in generation order,
// the same batch sorted lexically, and whether the two agree. With this
// generator they always do — lexical order IS time order.
type UUIDBatch struct {
	Count                         int      `json:"count"`
	GeneratedInOrder              []string `json:"generatedInOrder"`
	SortedLexically               []string `json:"sortedLexically"`
	SortingMatchesGenerationOrder bool     `json:"sortingMatchesGenerationOrder"`
}

// UUIDCollisionCheck mints a volume of ids and counts distinct values —
// the property "collision-free" is empirical, so the demo shows the
// experiment, not just the claim.
type UUIDCollisionCheck struct {
	Generated      int  `json:"generated"`
	Distinct       int  `json:"distinct"`
	CollisionFree  bool `json:"collisionFree"`
	RandomnessBits int  `json:"randomnessBits"`
}

// UUIDStrategyRow compares id strategies on the axes that decide them.
type UUIDStrategyRow struct {
	Strategy        string `json:"strategy"`
	Storage         string `json:"storage"`
	IndexLocality   string `json:"indexLocality"`
	URLGuessability string `json:"urlGuessability"`
	UniquenessScope string `json:"uniquenessScope"`
	Notes           string `json:"notes"`
}

// UUIDGeneratorInfo names what minted the batch.
type UUIDGeneratorInfo struct {
	Name         string `json:"name"`
	RandomSource string `json:"randomSource"`
	Description  string `json:"description"`
}

// UUIDLabPayload is the wire shape of GET /api/demo/db/uuid (camelCase
// JSON). Counts, layout, and comparison rows are deterministic; the uuid
// VALUES (anatomy.value, batch ids) vary by necessity — they are fresh
// randomness — so consumers assert their PROPERTIES (sortedness, canonical
// form, collision counts), never their literals.
type UUIDLabPayload struct {
	Generator  UUIDGeneratorInfo  `json:"generator"`
	Layout     []UUIDLayoutRow    `json:"layout"`
	Anatomy    UUIDAnatomy        `json:"anatomy"`
	Batch      UUIDBatch          `json:"batch"`
	Collision  UUIDCollisionCheck `json:"collision"`
	Comparison []UUIDStrategyRow  `json:"comparison"`
}

// uuidLabLayout is the fixed field-by-field layout table.
var uuidLabLayout = []UUIDLayoutRow{
	{Field: "unix_ts_ms", Bits: 48, Contents: "Unix epoch milliseconds, big-endian — the sort key that makes v7 time-ordered"},
	{Field: "ver", Bits: 4, Contents: "0b0111 — version 7, so parsers know what the other fields mean"},
	{Field: "rand_a", Bits: 12, Contents: "this guide: a per-millisecond counter — same-millisecond ids stay ordered"},
	{Field: "var", Bits: 2, Contents: "0b10 — the RFC 4122/9562 variant marker"},
	{Field: "rand_b", Bits: 62, Contents: "crypto/rand — uniqueness within a millisecond-counter slot"},
}

// uuidLabComparison is the fixed strategy table: the axes the ids topic
// argues over, verdicts included.
var uuidLabComparison = []UUIDStrategyRow{
	{
		Strategy:        "bigserial",
		Storage:         "8 bytes",
		IndexLocality:   "perfect — every insert appends to the btree's right edge",
		URLGuessability: "enumerable — /posts/1, /posts/2, … invites scraping",
		UniquenessScope: "one table's sequence",
		Notes:           "cheap and ordered, but ids leak volume and pace; merging two databases then means renumbering",
	},
	{
		Strategy:        "uuidv7 (this generator)",
		Storage:         "16 bytes",
		IndexLocality:   "good — time-ordered like a sequence, only coarser",
		URLGuessability: "unguessable — 74 random bits per millisecond slot",
		UniquenessScope: "global — any node mints ids without coordination",
		Notes:           "the default here: minted app-side, sortable, safe to expose; costs the wider key and its index fanout",
	},
	{
		Strategy:        "uuidv4 (gen_random_uuid())",
		Storage:         "16 bytes",
		IndexLocality:   "worst — random inserts scatter across every index page",
		URLGuessability: "unguessable",
		UniquenessScope: "global",
		Notes:           "PostgreSQL's built-in DEFAULT; fine for rows the database mints, hostile to large hot indexes — v7 exists to fix exactly this",
	},
}

// BuildUUIDLabPayload mints the demo's evidence live: one dissected id, an
// ordered batch with its sorted copy, a collision check at volume, and the
// strategy comparison around them. The generator is a parameter so the
// handler wires the production one and tests wire deterministic fakes.
func BuildUUIDLabPayload(generator *UUIDv7Generator) (UUIDLabPayload, error) {
	generated := make([]string, 0, uuidLabBatchSize)
	for i := 0; i < uuidLabBatchSize; i++ {
		value, err := generator.Next()
		if err != nil {
			return UUIDLabPayload{}, fmt.Errorf("mint batch member %d: %w", i, err)
		}
		generated = append(generated, value)
	}
	sorted := append([]string(nil), generated...)
	sort.Strings(sorted)
	matches := true
	for i := range generated {
		if generated[i] != sorted[i] {
			matches = false
			break
		}
	}

	anatomy, err := uuidAnatomy(generated[0])
	if err != nil {
		return UUIDLabPayload{}, err
	}

	distinct := make(map[string]struct{}, uuidLabCollisionVolume)
	for i := 0; i < uuidLabCollisionVolume; i++ {
		value, err := generator.Next()
		if err != nil {
			return UUIDLabPayload{}, fmt.Errorf("mint collision-check member %d: %w", i, err)
		}
		distinct[value] = struct{}{}
	}

	return UUIDLabPayload{
		Generator: UUIDGeneratorInfo{
			Name:         "UUIDv7, hand-rolled (RFC 9562 §5.7)",
			RandomSource: "crypto/rand — 12-bit counter seed + 62 random bits per id",
			Description:  "48-bit Unix milliseconds + version 7 + monotonic 12-bit counter + variant 10 + 62 random bits; no uuid library, no new dependency",
		},
		Layout:  append([]UUIDLayoutRow(nil), uuidLabLayout...),
		Anatomy: anatomy,
		Batch: UUIDBatch{
			Count:                         len(generated),
			GeneratedInOrder:              generated,
			SortedLexically:               sorted,
			SortingMatchesGenerationOrder: matches,
		},
		Collision: UUIDCollisionCheck{
			Generated:      uuidLabCollisionVolume,
			Distinct:       len(distinct),
			CollisionFree:  len(distinct) == uuidLabCollisionVolume,
			RandomnessBits: uuidLabRandomnessBits,
		},
		Comparison: append([]UUIDStrategyRow(nil), uuidLabComparison...),
	}, nil
}

// uuidAnatomy dissects one canonical value into the fields the layout
// table names.
func uuidAnatomy(value string) (UUIDAnatomy, error) {
	bits, err := parseUUID(value)
	if err != nil {
		return UUIDAnatomy{}, err
	}
	variant := "RFC 4122/9562 (0b10)"
	if bits[8]>>6 != variantRFC4122 {
		variant = fmt.Sprintf("unexpected variant bits %02b", bits[8]>>6)
	}
	unixMs := uint64(bits[0])<<40 | uint64(bits[1])<<32 | uint64(bits[2])<<24 |
		uint64(bits[3])<<16 | uint64(bits[4])<<8 | uint64(bits[5])
	return UUIDAnatomy{
		Value:   value,
		UnixMs:  int64(unixMs),
		Version: int(bits[6] >> 4),
		Variant: variant,
		Length:  len(value),
	}, nil
}
