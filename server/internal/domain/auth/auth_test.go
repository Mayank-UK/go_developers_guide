package auth

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

// The auth area's shared test scaffolding: an in-memory directory with
// two seeded users, throwaway stores for every port, and a fixed clock
// seam. The REAL adapters live in internal/data/auth; these fakes are
// deliberately minimal — just enough surface for the engine tests
// below, no concurrency claims, no production use.

type fakeDirectory struct {
	users map[string]User
}

func (d fakeDirectory) FindUser(_ context.Context, username string) (User, bool, error) {
	user, ok := d.users[username]
	return user, ok, nil
}

func newTestDirectory() fakeDirectory {
	hasher := NewHasher()
	return fakeDirectory{users: map[string]User{
		"ada": {
			Username:     "ada",
			DisplayName:  "Ada",
			PasswordHash: hasher.HashWithSalt("correct-horse", []byte("ada-salt--------")),
			Scopes:       []string{"posts:read", "posts:write"},
		},
		"linus": {
			Username:     "linus",
			DisplayName:  "Linus",
			PasswordHash: hasher.HashWithSalt("tr0ub4dor", []byte("linus-salt------")),
			Scopes:       []string{"posts:read"},
		},
	}}
}

type fakeSessionStore struct {
	mu       sync.Mutex
	sessions map[string]StoredSession
}

func (s *fakeSessionStore) CreateSession(_ context.Context, session StoredSession) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[session.ID] = session
	return nil
}

func (s *fakeSessionStore) FindSession(_ context.Context, id string) (StoredSession, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	session, ok := s.sessions[id]
	return session, ok, nil
}

func (s *fakeSessionStore) DeleteSession(_ context.Context, id string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.sessions[id]; !ok {
		return false, nil
	}
	delete(s.sessions, id)
	return true, nil
}

type fakeAPIKeyStore struct {
	mu   sync.Mutex
	keys map[string]StoredAPIKey
}

func (s *fakeAPIKeyStore) SaveKey(_ context.Context, key StoredAPIKey) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.keys[key.Prefix] = key
	return nil
}

func (s *fakeAPIKeyStore) FindKeyByPrefix(_ context.Context, prefix string) (StoredAPIKey, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key, ok := s.keys[prefix]
	return key, ok, nil
}

func (s *fakeAPIKeyStore) ListKeys(_ context.Context) ([]StoredAPIKey, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]StoredAPIKey, 0, len(s.keys))
	for _, key := range s.keys {
		out = append(out, key)
	}
	return out, nil
}

func (s *fakeAPIKeyStore) RevokeKey(_ context.Context, prefix string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.keys[prefix]; !ok {
		return false, nil
	}
	delete(s.keys, prefix)
	return true, nil
}

type fakeRefreshStore struct {
	mu       sync.Mutex
	families map[string]StoredRefreshFamily
	tokens   map[string]StoredRefreshToken
}

func (s *fakeRefreshStore) StartFamily(_ context.Context, family StoredRefreshFamily, first StoredRefreshToken) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.families[family.ID] = family
	s.tokens[first.SHA256] = first
	return nil
}

func (s *fakeRefreshStore) FindRefreshToken(_ context.Context, sha string) (StoredRefreshToken, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	token, ok := s.tokens[sha]
	return token, ok, nil
}

func (s *fakeRefreshStore) MarkRefreshUsed(_ context.Context, sha string, usedAt time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	token := s.tokens[sha]
	token.Status = "used"
	token.UsedAt = usedAt
	s.tokens[sha] = token
	return nil
}

func (s *fakeRefreshStore) AddRefreshToken(_ context.Context, token StoredRefreshToken) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tokens[token.SHA256] = token
	return nil
}

func (s *fakeRefreshStore) RevokeFamily(_ context.Context, familyID string) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	killed := 0
	family := s.families[familyID]
	family.Status = "revoked"
	s.families[familyID] = family
	for sha, token := range s.tokens {
		if token.FamilyID == familyID {
			token.Status = "revoked"
			s.tokens[sha] = token
			killed++
		}
	}
	return killed, nil
}

func (s *fakeRefreshStore) FindFamily(_ context.Context, familyID string) (StoredRefreshFamily, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	family, ok := s.families[familyID]
	return family, ok, nil
}

func (s *fakeRefreshStore) ListFamilies(_ context.Context) ([]StoredRefreshFamily, []StoredRefreshToken, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	families := make([]StoredRefreshFamily, 0, len(s.families))
	for _, family := range s.families {
		families = append(families, family)
	}
	tokens := make([]StoredRefreshToken, 0, len(s.tokens))
	for _, token := range s.tokens {
		tokens = append(tokens, token)
	}
	return families, tokens, nil
}

type fakeCodeStore struct {
	mu    sync.Mutex
	codes map[string]StoredAuthorizationCode
}

func (s *fakeCodeStore) IssueCode(_ context.Context, code StoredAuthorizationCode) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.codes[code.SHA256] = code
	return nil
}

func (s *fakeCodeStore) TakeCode(_ context.Context, sha string) (StoredAuthorizationCode, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	code, ok := s.codes[sha]
	if ok {
		delete(s.codes, sha)
	}
	return code, ok, nil
}

// newTestEngines wires every engine over the fakes and a fixed secret
// set, at the test-friendly clock moment.
func newTestEngines() (fakeDirectory, *SessionService, *ApiKeyService, *TokenService, *OAuthProvider, *RefreshRotationService, *Resolver) {
	directory := newTestDirectory()
	sessions := NewSessionService(directory, &fakeSessionStore{sessions: map[string]StoredSession{}},
		NewHasher(), []byte("session-secret"))
	keys := NewApiKeyService(&fakeAPIKeyStore{keys: map[string]StoredAPIKey{}})
	tokens := NewTokenService([]byte("jwt-secret"))
	provider := NewOAuthProvider(&fakeCodeStore{codes: map[string]StoredAuthorizationCode{}}, tokens)
	refresh := NewRefreshRotationService(&fakeRefreshStore{families: map[string]StoredRefreshFamily{}, tokens: map[string]StoredRefreshToken{}},
		directory, NewHasher(), tokens)
	resolver := NewResolver(sessions, keys, tokens)
	return directory, sessions, keys, tokens, provider, refresh, resolver
}

var testNow = time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)

// fmtErrors renders a wrapped-error chain for assertions.
func fmtErrors(err error) string {
	if err == nil {
		return "<nil>"
	}
	return fmt.Sprintf("%v | is-invalid-credentials=%v is-not-authenticated=%v is-invalid-grant=%v is-invalid-refresh=%v",
		err, errors.Is(err, ErrInvalidCredentials), errors.Is(err, ErrNotAuthenticated),
		errors.Is(err, ErrInvalidGrant), errors.Is(err, ErrInvalidRefresh))
}
