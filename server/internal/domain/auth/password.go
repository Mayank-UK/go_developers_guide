package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/argon2"
)

// HashParams is argon2id's cost, in the open: memory in KiB, iterations
// (time cost), parallelism (lanes), and the salt and derived-key
// lengths. The parameters are STAMPED INSIDE every hash this package
// mints, so a verify re-derives with the parameters the hash itself
// carries — old hashes stay verifiable after parameter upgrades.
type HashParams struct {
	MemoryKiB   uint32
	Iterations  uint32
	Parallelism uint8
	SaltBytes   int
	KeyBytes    int
}

// DefaultHashParams is the OWASP-recommended argon2id profile this guide
// hashes with: 19 MiB of memory, 2 iterations, 1 lane — the floor of the
// cheat-sheet's current guidance, deliberately, because a teaching demo
// verifies several times per request and production dials the same knobs
// up (64 MiB / t=3 is the next step) without changing any code.
var DefaultHashParams = HashParams{
	MemoryKiB:   19456,
	Iterations:  2,
	Parallelism: 1,
	SaltBytes:   16,
	KeyBytes:    32,
}

// LegacyHashParams is 2015-era cost (8 MiB, one pass): verifies still
// PASS against hashes minted at this cost, but NeedsRehash fires — the
// rehash-on-login signal the hashing demo teaches.
var LegacyHashParams = HashParams{
	MemoryKiB:   8192,
	Iterations:  1,
	Parallelism: 2,
	SaltBytes:   16,
	KeyBytes:    32,
}

// phcPrefix and argon2idVersion pin the encoded format: a PHC string
// $argon2id$v=19$m=<KiB>,t=<iters>,p=<lanes>$<salt b64url>$<key b64url>.
const (
	phcPrefix       = "$argon2id$v="
	argon2idVersion = 19
)

// Hasher mints and verifies argon2id password hashes. It is stateless
// apart from its parameters — safe to share across requests.
type Hasher struct {
	Params HashParams
}

// NewHasher returns a hasher at the guide's default parameters.
func NewHasher() *Hasher { return &Hasher{Params: DefaultHashParams} }

// Parameters renders the active cost profile for panels and prose:
// "argon2id · m=19456 KiB · t=2 · p=1".
func (h *Hasher) Parameters() string {
	return fmt.Sprintf("argon2id · m=%d KiB · t=%d · p=%d",
		h.Params.MemoryKiB, h.Params.Iterations, h.Params.Parallelism)
}

// Hash derives a fresh salt and mints the encoded hash. The same
// password never hashes to the same string — the salt guarantees it —
// and the cost parameters travel inside the string, not beside it.
func (h *Hasher) Hash(password string) (string, error) {
	salt := make([]byte, h.Params.SaltBytes)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("read password salt entropy: %w", err)
	}
	key := argon2.IDKey([]byte(password), salt,
		h.Params.Iterations, h.Params.MemoryKiB, h.Params.Parallelism, uint32(h.Params.KeyBytes))
	return encodePHC(h.Params, salt, key), nil
}

// HashWithSalt hashes with an EXPLICIT salt — the deterministic seam the
// demo fixtures (the decoy hash, the legacy-cost hash) build on. Callers
// never need it for real credentials; Hash mints the salt.
func (h *Hasher) HashWithSalt(password string, salt []byte) string {
	key := argon2.IDKey([]byte(password), salt,
		h.Params.Iterations, h.Params.MemoryKiB, h.Params.Parallelism, uint32(h.Params.KeyBytes))
	return encodePHC(h.Params, salt, key)
}

// Verify re-derives the key from the candidate password using the
// parameters the stored hash itself carries, then compares the two keys
// in constant time. A mismatch still burns the full derivation — match
// and mismatch cost the same by construction, and a malformed stored
// hash is an error, not a failed match: stale rows deserve a loud
// failure, not a silent reject.
func (h *Hasher) Verify(password, encoded string) error {
	params, salt, want, err := decodePHC(encoded)
	if err != nil {
		return fmt.Errorf("decode stored hash: %w", err)
	}
	got := argon2.IDKey([]byte(password), salt,
		params.Iterations, params.MemoryKiB, params.Parallelism, uint32(len(want)))
	if subtle.ConstantTimeCompare(got, want) != 1 {
		return ErrInvalidCredentials
	}
	return nil
}

// NeedsRehash answers whether a verified hash was minted at parameters
// weaker than the hasher's current ones — the rehash-on-login signal.
func (h *Hasher) NeedsRehash(encoded string) bool {
	params, _, _, err := decodePHC(encoded)
	if err != nil {
		return true
	}
	return params.MemoryKiB != h.Params.MemoryKiB ||
		params.Iterations != h.Params.Iterations ||
		params.Parallelism != h.Params.Parallelism
}

// decoySalt is a fixed, public salt: the decoy hash is not a secret, it
// is WORK — the uniform-cost verification an unknown username burns so
// "no such user" and "wrong password" are indistinguishable by timing.
var decoySalt = []byte("gdg-decoy-salt-0")

// DecoyHash is a valid argon2id hash of a throwaway string, minted once
// at package init at the default cost. Login paths verify against it
// whenever the username is unknown, so every rejection costs one full
// argon2 run.
var DecoyHash = func() string {
	return NewHasher().HashWithSalt("decoy-password-for-unknown-users", decoySalt)
}()

// VerifyUniform is the timing-uniform credential check every login path
// uses: an empty stored hash still burns one full verification against
// the decoy, and the answer is false either way — callers learn nothing
// about which half failed, and neither does a stopwatch.
func (h *Hasher) VerifyUniform(password, encoded string) bool {
	if encoded == "" {
		encoded = DecoyHash
	}
	return h.Verify(password, encoded) == nil
}

func encodePHC(params HashParams, salt, key []byte) string {
	var b strings.Builder
	b.WriteString(phcPrefix)
	b.WriteString(strconv.Itoa(argon2idVersion))
	b.WriteString(fmt.Sprintf("$m=%d,t=%d,p=%d$", params.MemoryKiB, params.Iterations, params.Parallelism))
	b.WriteString(base64.RawURLEncoding.EncodeToString(salt))
	b.WriteByte('$')
	b.WriteString(base64.RawURLEncoding.EncodeToString(key))
	return b.String()
}

func decodePHC(encoded string) (HashParams, []byte, []byte, error) {
	parts := strings.Split(encoded, "$")
	// "", "argon2id", "v=19", "m=…,t=…,p=…", salt, key — six fields.
	if len(parts) != 6 || parts[1] != "argon2id" {
		return HashParams{}, nil, nil, fmt.Errorf("not an argon2id PHC string: %q", encoded)
	}
	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil || version != argon2idVersion {
		return HashParams{}, nil, nil, fmt.Errorf("unsupported argon2 version in %q", parts[2])
	}
	params := HashParams{}
	seen := 0
	for _, field := range strings.Split(parts[3], ",") {
		name, value, ok := strings.Cut(field, "=")
		if !ok {
			return HashParams{}, nil, nil, fmt.Errorf("malformed parameter %q", field)
		}
		parsed, err := strconv.ParseUint(value, 10, 32)
		if err != nil {
			return HashParams{}, nil, nil, fmt.Errorf("malformed parameter %q: %w", field, err)
		}
		switch name {
		case "m":
			params.MemoryKiB = uint32(parsed)
		case "t":
			params.Iterations = uint32(parsed)
		case "p":
			params.Parallelism = uint8(parsed)
		default:
			return HashParams{}, nil, nil, fmt.Errorf("unknown parameter %q", field)
		}
		seen++
	}
	if seen != 3 {
		return HashParams{}, nil, nil, fmt.Errorf("parameter field %q needs m, t and p", parts[3])
	}
	salt, err := base64.RawURLEncoding.DecodeString(parts[4])
	if err != nil {
		return HashParams{}, nil, nil, fmt.Errorf("decode salt: %w", err)
	}
	key, err := base64.RawURLEncoding.DecodeString(parts[5])
	if err != nil {
		return HashParams{}, nil, nil, fmt.Errorf("decode key: %w", err)
	}
	return params, salt, key, nil
}

// Elapsed renders a duration in milliseconds with one decimal — the
// honest unit the hashing story reports its measurements in.
func Elapsed(d time.Duration) string {
	return fmt.Sprintf("%.1f ms", float64(d.Microseconds())/1000)
}
