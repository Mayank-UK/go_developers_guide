// Package auth adapters: the in-memory implementations of the auth
// domain's store ports. Every adapter is mutex-guarded (concurrent
// requests share the console) and honest about its scope — state lives
// in the process, so a restart wipes it and two replicas never see the
// same sessions. Production swaps these for Redis (session rows and
// refresh families with TTL eviction) or PostgreSQL (a users table for
// the directory, key records) behind the SAME ports; the guide teaches
// that trade where the demo makes it.
package auth

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	domauth "guide/internal/domain/auth"
)

// Directory is the in-memory user directory: usernames to rows,
// read-only after construction (the console seeds it once).
type Directory struct {
	users map[string]domauth.User
}

// NewDirectory builds a directory over pre-hashed user rows.
func NewDirectory(users map[string]domauth.User) *Directory {
	return &Directory{users: users}
}

// FindUser resolves a username to its credential row.
func (d *Directory) FindUser(_ context.Context, username string) (domauth.User, bool, error) {
	user, ok := d.users[username]
	return user, ok, nil
}

// SessionMemory is the in-memory SessionStore: one map, one mutex, TTL
// checked by the ENGINE (the adapter never invents policy).
type SessionMemory struct {
	mu       sync.Mutex
	sessions map[string]domauth.StoredSession
}

// NewSessionMemory returns an empty session store.
func NewSessionMemory() *SessionMemory {
	return &SessionMemory{sessions: make(map[string]domauth.StoredSession)}
}

// CreateSession records the row.
func (s *SessionMemory) CreateSession(_ context.Context, session domauth.StoredSession) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[session.ID] = session
	return nil
}

// FindSession resolves an id to its live row.
func (s *SessionMemory) FindSession(_ context.Context, id string) (domauth.StoredSession, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	session, ok := s.sessions[id]
	return session, ok, nil
}

// DeleteSession removes the row — logout. Answers whether a row died.
func (s *SessionMemory) DeleteSession(_ context.Context, id string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.sessions[id]; !ok {
		return false, nil
	}
	delete(s.sessions, id)
	return true, nil
}

// APIKeyMemory is the in-memory ApiKeyStore.
type APIKeyMemory struct {
	mu   sync.Mutex
	keys map[string]domauth.StoredAPIKey
}

// NewAPIKeyMemory returns an empty key store.
func NewAPIKeyMemory() *APIKeyMemory {
	return &APIKeyMemory{keys: make(map[string]domauth.StoredAPIKey)}
}

// SaveKey records the key's row (hash, never raw).
func (s *APIKeyMemory) SaveKey(_ context.Context, key domauth.StoredAPIKey) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.keys[key.Prefix] = key
	return nil
}

// FindKeyByPrefix resolves a prefix to its record.
func (s *APIKeyMemory) FindKeyByPrefix(_ context.Context, prefix string) (domauth.StoredAPIKey, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key, ok := s.keys[prefix]
	return key, ok, nil
}

// ListKeys answers the roster, oldest first.
func (s *APIKeyMemory) ListKeys(_ context.Context) ([]domauth.StoredAPIKey, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]domauth.StoredAPIKey, 0, len(s.keys))
	for _, key := range s.keys {
		out = append(out, key)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })
	return out, nil
}

// RevokeKey deletes the record by prefix.
func (s *APIKeyMemory) RevokeKey(_ context.Context, prefix string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.keys[prefix]; !ok {
		return false, nil
	}
	delete(s.keys, prefix)
	return true, nil
}

// RefreshMemory is the in-memory RefreshStore: families and their
// token chains, positions preserved.
type RefreshMemory struct {
	mu       sync.Mutex
	families map[string]domauth.StoredRefreshFamily
	tokens   map[string]domauth.StoredRefreshToken // keyed by SHA-256
	// order preserves creation order for ListFamilies without a clock.
	order []string
}

// NewRefreshMemory returns an empty refresh store.
func NewRefreshMemory() *RefreshMemory {
	return &RefreshMemory{families: make(map[string]domauth.StoredRefreshFamily), tokens: make(map[string]domauth.StoredRefreshToken)}
}

// StartFamily records the family and its first link.
func (s *RefreshMemory) StartFamily(_ context.Context, family domauth.StoredRefreshFamily, first domauth.StoredRefreshToken) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.families[family.ID] = family
	s.tokens[first.SHA256] = first
	s.order = append(s.order, family.ID)
	return nil
}

// FindRefreshToken resolves a token hash to its row.
func (s *RefreshMemory) FindRefreshToken(_ context.Context, sha string) (domauth.StoredRefreshToken, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	token, ok := s.tokens[sha]
	return token, ok, nil
}

// MarkRefreshUsed flips a token to used.
func (s *RefreshMemory) MarkRefreshUsed(_ context.Context, sha string, usedAt time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	token := s.tokens[sha]
	token.Status = "used"
	token.UsedAt = usedAt
	s.tokens[sha] = token
	return nil
}

// AddRefreshToken appends the next link.
func (s *RefreshMemory) AddRefreshToken(_ context.Context, token domauth.StoredRefreshToken) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tokens[token.SHA256] = token
	return nil
}

// RevokeFamily kills every token in the family and answers the count.
func (s *RefreshMemory) RevokeFamily(_ context.Context, familyID string) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.families[familyID]; !ok {
		return 0, fmt.Errorf("unknown refresh family %q", familyID)
	}
	family := s.families[familyID]
	family.Status = "revoked"
	s.families[familyID] = family
	killed := 0
	for sha, token := range s.tokens {
		if token.FamilyID == familyID {
			token.Status = "revoked"
			s.tokens[sha] = token
			killed++
		}
	}
	return killed, nil
}

// FindFamily resolves a family id to its row.
func (s *RefreshMemory) FindFamily(_ context.Context, familyID string) (domauth.StoredRefreshFamily, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	family, ok := s.families[familyID]
	return family, ok, nil
}

// ListFamilies answers every family with its tokens (families in
// creation order, tokens sorted by position) — the panel roster.
func (s *RefreshMemory) ListFamilies(_ context.Context) ([]domauth.StoredRefreshFamily, []domauth.StoredRefreshToken, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	families := make([]domauth.StoredRefreshFamily, 0, len(s.order))
	tokens := make([]domauth.StoredRefreshToken, 0)
	for _, id := range s.order {
		families = append(families, s.families[id])
	}
	for _, token := range s.tokens {
		tokens = append(tokens, token)
	}
	sort.SliceStable(tokens, func(i, j int) bool {
		if tokens[i].FamilyID != tokens[j].FamilyID {
			return tokens[i].IssuedAt.Before(tokens[j].IssuedAt)
		}
		return tokens[i].Position < tokens[j].Position
	})
	return families, tokens, nil
}

// CodeMemory is the in-memory AuthorizationCodeStore. TakeCode is
// consume-first: the first presentation of a hash removes it, exactly
// the one-time semantics the OAuth engine demands.
type CodeMemory struct {
	mu    sync.Mutex
	codes map[string]domauth.StoredAuthorizationCode
}

// NewCodeMemory returns an empty code store.
func NewCodeMemory() *CodeMemory {
	return &CodeMemory{codes: make(map[string]domauth.StoredAuthorizationCode)}
}

// IssueCode records the code row (hash-keyed).
func (s *CodeMemory) IssueCode(_ context.Context, code domauth.StoredAuthorizationCode) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.codes[code.SHA256] = code
	return nil
}

// TakeCode removes and answers the code under a hash — one attempt.
func (s *CodeMemory) TakeCode(_ context.Context, sha string) (domauth.StoredAuthorizationCode, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	code, ok := s.codes[sha]
	if ok {
		delete(s.codes, sha)
	}
	return code, ok, nil
}
