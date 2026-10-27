package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

// randomToken mints n bytes of crypto/rand entropy as base64url — the
// shared mint behind session ids, key secrets, refresh handles, and
// OAuth codes. Everything this area treats as "unguessable" comes from
// here, and only here.
func randomToken(n int) (string, error) {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("read token entropy: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// APIKeyPrefix is the readable half's namespace: keys read
// gdg_<8 hex>.<secret>, so a key is greppable in logs and revocable by
// a short, human-typeable handle.
const APIKeyPrefix = "gdg"

// APIKeySeparator splits a presented key's prefix from its secret.
const APIKeySeparator = "."

// ApiKeyService implements the three rules a key system lives or dies
// by. GENERATION mints a two-part key — a short readable prefix and a
// 256-bit secret — and stores only the SHA-256 of the full key, so a
// leaked store does not leak working keys. VERIFICATION splits at the
// separator, fetches the record by prefix, re-hashes the presentation,
// and compares the two hashes in constant time. REVOCATION is a
// prefix-keyed delete: instant death for every copy of the key.
type ApiKeyService struct {
	store ApiKeyStore
}

// NewApiKeyService wires the engine over its store port.
func NewApiKeyService(store ApiKeyStore) *ApiKeyService {
	return &ApiKeyService{store: store}
}

// ApiKeyCreation is the ONE response the raw key ever appears in.
type ApiKeyCreation struct {
	Prefix  string
	FullKey string
	SHA256  string
	Name    string
	Scopes  []string
}

// Generate mints prefix + secret, stores the hash, and answers the raw
// key exactly once — everything after this moment carries the prefix
// alone, and even the prefix is display, never proof.
func (s *ApiKeyService) Generate(ctx context.Context, name string, scopes []string, now time.Time) (ApiKeyCreation, error) {
	prefixEntropy := make([]byte, 4)
	if _, err := rand.Read(prefixEntropy); err != nil {
		return ApiKeyCreation{}, fmt.Errorf("read key prefix entropy: %w", err)
	}
	prefix := APIKeyPrefix + "_" + hex.EncodeToString(prefixEntropy)
	secret, err := randomToken(32)
	if err != nil {
		return ApiKeyCreation{}, err
	}
	fullKey := prefix + APIKeySeparator + secret
	keySHA := SHA256Hex(fullKey)
	if err := s.store.SaveKey(ctx, StoredAPIKey{
		Prefix:    prefix,
		Name:      name,
		Scopes:    scopes,
		KeySHA256: keySHA,
		CreatedAt: now,
	}); err != nil {
		return ApiKeyCreation{}, fmt.Errorf("save api key: %w", err)
	}
	return ApiKeyCreation{Prefix: prefix, FullKey: fullKey, SHA256: keySHA, Name: name, Scopes: scopes}, nil
}

// dummyKeySHA is a comparison target that never matches — malformed
// presentations burn one constant-time comparison against it so every
// rejection costs the same shape of work.
var dummyKeySHA = SHA256Hex("gdg_00000000.dead-key-that-never-existed")

// Verify is the protected-call check: prefix lookup plus a
// constant-time hash compare. A malformed presentation (no separator)
// burns the dummy comparison before refusing.
func (s *ApiKeyService) Verify(ctx context.Context, presented string) (StoredAPIKey, error) {
	prefix, _, ok := strings.Cut(presented, APIKeySeparator)
	if !ok {
		_ = subtle.ConstantTimeCompare([]byte(SHA256Hex(presented)), []byte(dummyKeySHA))
		return StoredAPIKey{}, ErrNotAuthenticated
	}
	record, found, err := s.store.FindKeyByPrefix(ctx, prefix)
	if err != nil {
		return StoredAPIKey{}, fmt.Errorf("find key by prefix: %w", err)
	}
	if !found || subtle.ConstantTimeCompare([]byte(SHA256Hex(presented)), []byte(record.KeySHA256)) != 1 {
		return StoredAPIKey{}, ErrNotAuthenticated
	}
	return record, nil
}

// List answers the roster, oldest first — hash previews are
// display-only.
func (s *ApiKeyService) List(ctx context.Context) ([]StoredAPIKey, error) {
	keys, err := s.store.ListKeys(ctx)
	if err != nil {
		return nil, fmt.Errorf("list api keys: %w", err)
	}
	return keys, nil
}

// Revoke deletes the record by prefix. The next lookup finds nothing —
// revocation is immediate, and every copy of the key in the world dies
// with the record.
func (s *ApiKeyService) Revoke(ctx context.Context, prefix string) (bool, error) {
	revoked, err := s.store.RevokeKey(ctx, prefix)
	if err != nil {
		return false, fmt.Errorf("revoke api key %s: %w", prefix, err)
	}
	return revoked, nil
}

// SHA256Hex is the storage hash: plain SHA-256, hex-encoded. Argon2 is
// for passwords (low entropy, must be slow); a 256-bit random key
// needs no stretching — one SHA-256 makes the grind 2^256 wide.
func SHA256Hex(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}
