// Package auth is the guide's authentication domain area (topic 23):
// the vocabulary every credential mechanism shares (users, identities,
// the store ports sessions, API keys, refresh families, and OAuth codes
// live behind), and the engines built on it — argon2id password hashing,
// the session login with its sealed cookie, hashed-at-rest API keys,
// stdlib-only HS256 JWTs, the OAuth2 code grant with PKCE, and refresh
// rotation with reuse detection. Engines depend on the store ports and a
// clock seam, never on a storage technology; the in-memory adapters live
// in internal/data/auth (a Redis- or pgx-backed adapter would slot into
// the same ports without touching this package).
package auth

import (
	"context"
	"errors"
	"time"
)

// The typed sentinel errors every engine answers with. Handlers classify
// them onto the wire taxonomy (401 unauthorized); nothing here leaks
// WHICH half of a credential pair failed — that honesty is the point.
var (
	// ErrInvalidCredentials is the one answer every login failure gives:
	// unknown user, wrong password, same envelope, same shape.
	ErrInvalidCredentials = errors.New("invalid credentials")
	// ErrNotAuthenticated covers every failed proof presentation (bad
	// signature, unknown handle, expired row, tampered value).
	ErrNotAuthenticated = errors.New("not authenticated")
	// ErrInvalidGrant is the OAuth2 token-endpoint refusal for a code or
	// verifier that does not check out.
	ErrInvalidGrant = errors.New("invalid_grant")
	// ErrInvalidRefresh is the refresh-endpoint refusal for an unknown,
	// expired, or family-revoked refresh token.
	ErrInvalidRefresh = errors.New("invalid refresh token")
)

// TTL constants the engines share — one place to read the leases.
const (
	// SessionTTL is a session row's lifetime and its cookie's Max-Age.
	SessionTTL = 30 * time.Minute
	// AccessTokenTTL is the short lease a JWT carries: brief enough that
	// revocation-by-expiry is a reasonable approximation.
	AccessTokenTTL = 15 * time.Minute
	// RefreshTTL is the long lease a refresh family runs on.
	RefreshTTL = 24 * time.Hour
	// AuthorizationCodeTTL is the one-time code's tiny window.
	AuthorizationCodeTTL = 60 * time.Second
)

// Identity is what every mechanism resolves to and what topic 24's
// authorization consumes: WHO is calling, what they may address (scopes
// are vocabulary, not verdicts), and WHICH mechanism carried the proof.
type Identity struct {
	Username    string
	DisplayName string
	Scopes      []string
	// Via names the carrier: "session", "api-key", or "jwt".
	Via string
}

// User is one directory row: the username, its display form, the argon2id
// hash of the password, and the scopes a login of this user carries.
type User struct {
	Username     string
	DisplayName  string
	PasswordHash string
	Scopes       []string
}

// UserDirectory resolves a username to its credential row. The port is
// the seam: the demo console backs it with two seeded users, production
// would back it with the users table behind pgx.
type UserDirectory interface {
	FindUser(ctx context.Context, username string) (User, bool, error)
}

// StoredSession is the server-side row a login creates: the random id
// (never the password, never the claims — just WHO), the owner, and the
// expiry that lets the store evict.
type StoredSession struct {
	ID          string
	Username    string
	DisplayName string
	Scopes      []string
	IssuedAt    time.Time
	ExpiresAt   time.Time
}

// SessionStore is the session port: create on login, resolve on every
// authenticated request, delete on logout. Revocation is DELETION — the
// moment the row is gone, every copy of the cookie value is dead.
type SessionStore interface {
	CreateSession(ctx context.Context, session StoredSession) error
	FindSession(ctx context.Context, id string) (StoredSession, bool, error)
	DeleteSession(ctx context.Context, id string) (bool, error)
}

// StoredAPIKey is one key record: the greppable prefix, its name and
// scopes, and the SHA-256 of the full key — the raw key is never stored.
type StoredAPIKey struct {
	Prefix    string
	Name      string
	Scopes    []string
	KeySHA256 string
	CreatedAt time.Time
}

// ApiKeyStore is the key port: save at generation, find by prefix at
// verification, list for rosters, delete on revocation.
type ApiKeyStore interface {
	SaveKey(ctx context.Context, key StoredAPIKey) error
	FindKeyByPrefix(ctx context.Context, prefix string) (StoredAPIKey, bool, error)
	ListKeys(ctx context.Context) ([]StoredAPIKey, error)
	RevokeKey(ctx context.Context, prefix string) (bool, error)
}

// StoredRefreshToken is one link in a rotation family: the SHA-256 of the
// raw token (the raw value rides exactly one response), the family it
// belongs to, its position in the chain, and its status — active until
// exchanged, used after, revoked with the whole family on reuse.
type StoredRefreshToken struct {
	SHA256   string
	FamilyID string
	Position int
	Status   string // "active" | "used" | "revoked"
	IssuedAt time.Time
	UsedAt   time.Time
}

// StoredRefreshFamily groups a login's refresh chain: every token minted
// after that login belongs to it, and a reuse event revokes ALL of them.
type StoredRefreshFamily struct {
	ID       string
	Username string
	Status   string // "active" | "revoked"
}

// RefreshStore is the rotation port: start a family at login, find the
// token a presentation hashes to, mark it used, append the next link,
// revoke the family (and count the dead), and list for panels.
type RefreshStore interface {
	StartFamily(ctx context.Context, family StoredRefreshFamily, first StoredRefreshToken) error
	FindRefreshToken(ctx context.Context, tokenSHA256 string) (StoredRefreshToken, bool, error)
	MarkRefreshUsed(ctx context.Context, tokenSHA256 string, usedAt time.Time) error
	AddRefreshToken(ctx context.Context, token StoredRefreshToken) error
	RevokeFamily(ctx context.Context, familyID string) (int, error)
	FindFamily(ctx context.Context, familyID string) (StoredRefreshFamily, bool, error)
	ListFamilies(ctx context.Context) ([]StoredRefreshFamily, []StoredRefreshToken, error)
}

// StoredAuthorizationCode is the OAuth2 one-time code, stored as its
// SHA-256 and bound to the client, the exact redirect URI, and the PKCE
// challenge it was issued under.
type StoredAuthorizationCode struct {
	SHA256        string
	ClientID      string
	RedirectURI   string
	CodeChallenge string
	Username      string
	Scopes        []string
	IssuedAt      time.Time
	ExpiresAt     time.Time
}

// AuthorizationCodeStore is the code port. TakeCode is CONSUME-FIRST:
// the first presentation of a hash removes it, successful or not, so a
// stolen code buys exactly one guess.
type AuthorizationCodeStore interface {
	IssueCode(ctx context.Context, code StoredAuthorizationCode) error
	TakeCode(ctx context.Context, codeSHA256 string) (StoredAuthorizationCode, bool, error)
}
