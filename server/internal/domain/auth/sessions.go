package auth

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"strings"
	"time"
)

// SessionCookieName is the cookie the session mechanism rides.
const SessionCookieName = "gdg_session"

// SessionCookiePath scopes the cookie to the site's own paths.
const SessionCookiePath = "/"

// SessionIDBytes is the session id's entropy: 32 bytes → 256 bits of
// unguessable handle, carried base64url.
const SessionIDBytes = 32

// CookieInstruction is a handler-ready Set-Cookie recipe: the name, the
// value, the Max-Age, and the three flags that make a session cookie
// honest. The HANDLER writes it with net/http's http.Cookie (which owns
// the actual header syntax); the domain owns the semantics.
type CookieInstruction struct {
	Name          string
	Value         string
	MaxAgeSeconds int
	Path          string
	HTTPOnly      bool
	SameSite      string // "lax" in this guide; the browser spellings map at the edge
	Secure        bool
}

// SessionService is the server-side session pattern whole: login
// verifies credentials (timing-uniform for unknown users), mints a
// random session id, records the row in the store, and answers the
// cookie — the VALUE is the id sealed with an HMAC signature, so a
// forged or edited cookie fails the seal before it ever reaches the
// store. Inspect resolves a presented cookie to the live row; logout
// deletes the row and expires the browser's copy.
type SessionService struct {
	directory UserDirectory
	store     SessionStore
	passwords *Hasher
	secret    []byte
}

// NewSessionService wires the engine over its ports. secret is the
// HMAC key that seals cookie values — distinct from every other secret
// the auth area holds.
func NewSessionService(directory UserDirectory, store SessionStore, passwords *Hasher, secret []byte) *SessionService {
	return &SessionService{directory: directory, store: store, passwords: passwords, secret: secret}
}

// SessionLoginResult is what a successful login answers: the identity
// established, the row's bookkeeping, and the cookie to set.
type SessionLoginResult struct {
	Identity  Identity
	SessionID string
	Cookie    CookieInstruction
	IssuedAt  time.Time
	ExpiresAt time.Time
}

// Login verifies the credential pair and creates the server session.
// An unknown username burns the decoy hash first, so its rejection
// costs the same as a wrong password — the 401 is indistinguishable by
// message or by stopwatch.
func (s *SessionService) Login(ctx context.Context, username, password string, now time.Time) (SessionLoginResult, error) {
	user, _, err := s.directory.FindUser(ctx, username)
	if err != nil {
		return SessionLoginResult{}, fmt.Errorf("find user %q: %w", username, err)
	}
	// VerifyUniform with the ZERO row's empty hash burns the DECOY for
	// unknown users — one full argon2 run either way, so the 401 is
	// indistinguishable by stopwatch. (A `!ok ||` short-circuit would
	// SKIP the burn and leak the username's existence by timing.)
	if !s.passwords.VerifyUniform(password, user.PasswordHash) {
		return SessionLoginResult{}, ErrInvalidCredentials
	}

	id, err := randomToken(SessionIDBytes)
	if err != nil {
		return SessionLoginResult{}, err
	}
	expiresAt := now.Add(SessionTTL)
	if err := s.store.CreateSession(ctx, StoredSession{
		ID:          id,
		Username:    user.Username,
		DisplayName: user.DisplayName,
		Scopes:      user.Scopes,
		IssuedAt:    now,
		ExpiresAt:   expiresAt,
	}); err != nil {
		return SessionLoginResult{}, fmt.Errorf("create session: %w", err)
	}
	return SessionLoginResult{
		Identity: Identity{
			Username:    user.Username,
			DisplayName: user.DisplayName,
			Scopes:      user.Scopes,
			Via:         "session",
		},
		SessionID: id,
		Cookie: CookieInstruction{
			Name:          SessionCookieName,
			Value:         s.seal(id),
			MaxAgeSeconds: int(SessionTTL.Seconds()),
			Path:          SessionCookiePath,
			HTTPOnly:      true,
			SameSite:      "lax",
			Secure:        false, // dev stack is plain http; production flips this unconditionally
		},
		IssuedAt:  now,
		ExpiresAt: expiresAt,
	}, nil
}

// Inspect resolves a presented cookie value to the live session row —
// or ErrNotAuthenticated for a missing cookie, a broken seal, an
// unknown id, or an expired row. Every failure is the SAME error: the
// response teaches the caller nothing about which half failed.
func (s *SessionService) Inspect(ctx context.Context, cookieValue string, now time.Time) (StoredSession, error) {
	id, ok := s.unseal(cookieValue)
	if !ok {
		return StoredSession{}, ErrNotAuthenticated
	}
	session, found, err := s.store.FindSession(ctx, id)
	if err != nil {
		return StoredSession{}, fmt.Errorf("find session: %w", err)
	}
	if !found {
		return StoredSession{}, ErrNotAuthenticated
	}
	if !now.Before(session.ExpiresAt) {
		_, _ = s.store.DeleteSession(ctx, session.ID)
		return StoredSession{}, ErrNotAuthenticated
	}
	return session, nil
}

// Logout deletes the server row and answers the clearing cookie: the
// same name and flags, empty value, Max-Age 0. Idempotent by design —
// logging out twice is not an error.
func (s *SessionService) Logout(ctx context.Context, cookieValue string) (bool, CookieInstruction, error) {
	clear := CookieInstruction{
		Name:          SessionCookieName,
		Value:         "",
		MaxAgeSeconds: 0,
		Path:          SessionCookiePath,
		HTTPOnly:      true,
		SameSite:      "lax",
		Secure:        false,
	}
	id, ok := s.unseal(cookieValue)
	if !ok {
		return false, clear, nil
	}
	revoked, err := s.store.DeleteSession(ctx, id)
	if err != nil {
		return false, clear, fmt.Errorf("delete session: %w", err)
	}
	return revoked, clear, nil
}

// CookieAnatomy explains one sealed value's shape for teaching panels:
// "id-half… . <n signature chars>" — the id opaque, the signature
// tamper-evident.
func (s *SessionService) CookieAnatomy(sealed string) string {
	cut := strings.LastIndex(sealed, ".")
	if cut <= 0 || cut == len(sealed)-1 {
		return "not a sealed value"
	}
	return fmt.Sprintf("%s… . <%d signature chars> — the opaque session id, sealed with an HMAC-SHA256 signature; edit one character and the seal is broken",
		sealed[:min(cut, 8)], len(sealed)-cut-1)
}

// seal signs the id: value = "<id>.<base64url(hmac(secret, id))>". The
// id is base64url (alphabet without dots), so the LAST dot in a sealed
// value is always the seal's.
func (s *SessionService) seal(id string) string {
	mac := hmac.New(sha256.New, s.secret)
	mac.Write([]byte(id))
	return id + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

// unseal verifies the signature in constant time (hmac.Equal) and
// answers the id — false on ANY tamper.
func (s *SessionService) unseal(value string) (string, bool) {
	cut := strings.LastIndex(value, ".")
	if cut <= 0 || cut == len(value)-1 {
		return "", false
	}
	id, signature := value[:cut], value[cut+1:]
	mac := hmac.New(sha256.New, s.secret)
	mac.Write([]byte(id))
	want := mac.Sum(nil)
	got, err := base64.RawURLEncoding.DecodeString(signature)
	if err != nil || !hmac.Equal(got, want) {
		return "", false
	}
	return id, true
}
