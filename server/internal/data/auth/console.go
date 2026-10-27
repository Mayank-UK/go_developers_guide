package auth

import (
	"crypto/rand"
	"fmt"
	"time"

	domauth "guide/internal/domain/auth"
)

// Console is the topic's whole substrate, wired once at the composition
// root and shared by the seven demo endpoints and the auth middleware:
// a seeded two-user directory (argon2id hashes minted at construction),
// in-memory stores for sessions, keys, refresh families, and OAuth
// codes, and every engine over them. Secrets (the JWT signing key, the
// cookie seal key) are minted from crypto/rand at boot — per-process,
// fine for a teaching substrate; production loads them from environment
// or a secret store, a discipline topic 25 takes up.
type Console struct {
	Directory *Directory
	Sessions  *domauth.SessionService
	Keys      *domauth.ApiKeyService
	Tokens    *domauth.TokenService
	OAuth     *domauth.OAuthProvider
	Refresh   *domauth.RefreshRotationService
	Resolver  *domauth.Resolver
}

// seeded users: two identities every story shares, with real argon2id
// credential hashes minted once at console construction.
var seededUsers = []struct {
	username string
	display  string
	password string
	scopes   []string
}{
	{"ada", "Ada", "correct-horse", []string{"posts:read", "posts:write"}},
	{"linus", "Linus", "tr0ub4dor", []string{"posts:read"}},
}

// NewConsole wires the whole auth substrate. It hashes the seeded
// users' passwords at default argon2id cost (a handful of milliseconds
// each, once per process).
func NewConsole() (*Console, error) {
	hasher := domauth.NewHasher()
	users := make(map[string]domauth.User, len(seededUsers))
	for _, seed := range seededUsers {
		hash, err := hasher.Hash(seed.password)
		if err != nil {
			return nil, fmt.Errorf("hash seed credential for %s: %w", seed.username, err)
		}
		users[seed.username] = domauth.User{
			Username:     seed.username,
			DisplayName:  seed.display,
			PasswordHash: hash,
			Scopes:       seed.scopes,
		}
	}
	jwtSecret := make([]byte, 32)
	if _, err := rand.Read(jwtSecret); err != nil {
		return nil, fmt.Errorf("mint jwt secret: %w", err)
	}
	cookieSecret := make([]byte, 32)
	if _, err := rand.Read(cookieSecret); err != nil {
		return nil, fmt.Errorf("mint cookie seal secret: %w", err)
	}

	directory := NewDirectory(users)
	sessionStore := NewSessionMemory()
	keyStore := NewAPIKeyMemory()
	refreshStore := NewRefreshMemory()
	codeStore := NewCodeMemory()

	tokens := domauth.NewTokenService(jwtSecret)
	sessions := domauth.NewSessionService(directory, sessionStore, hasher, cookieSecret)
	keys := domauth.NewApiKeyService(keyStore)
	console := &Console{
		Directory: directory,
		Sessions:  sessions,
		Keys:      keys,
		Tokens:    tokens,
		OAuth:     domauth.NewOAuthProvider(codeStore, tokens),
		Refresh:   domauth.NewRefreshRotationService(refreshStore, directory, hasher, tokens),
		Resolver:  domauth.NewResolver(sessions, keys, tokens),
	}
	return console, nil
}

// clock is the stories' time seam: one place, real time in production.
func clock() time.Time { return time.Now().UTC() }
