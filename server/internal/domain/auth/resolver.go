package auth

import (
	"context"
	"fmt"
	"time"
)

// CredentialResolver resolves any of the topic's three carriers — a
// session cookie, an x-api-key header, or a Bearer token — to the one
// Identity every protected handler consumes. This is the port the auth
// middleware depends on: presentation stays ignorant of which engine
// answered, and topic 24's authorization layers on the same Identity.
type CredentialResolver interface {
	ResolveSessionCookie(ctx context.Context, cookieValue string, now time.Time) (Identity, error)
	ResolveAPIKey(ctx context.Context, presented string, now time.Time) (Identity, error)
	ResolveBearerToken(ctx context.Context, token string, now time.Time) (Identity, error)
}

// Resolver is the concrete composition of the three engines behind the
// resolver port: sessions unseal and look up, keys hash and compare,
// tokens verify statelessly.
type Resolver struct {
	sessions *SessionService
	keys     *ApiKeyService
	tokens   *TokenService
}

// NewResolver wires the concrete resolver.
func NewResolver(sessions *SessionService, keys *ApiKeyService, tokens *TokenService) *Resolver {
	return &Resolver{sessions: sessions, keys: keys, tokens: tokens}
}

// ResolveSessionCookie resolves a sealed cookie value to its identity.
func (r *Resolver) ResolveSessionCookie(ctx context.Context, cookieValue string, now time.Time) (Identity, error) {
	session, err := r.sessions.Inspect(ctx, cookieValue, now)
	if err != nil {
		return Identity{}, err
	}
	return Identity{
		Username:    session.Username,
		DisplayName: session.DisplayName,
		Scopes:      session.Scopes,
		Via:         "session",
	}, nil
}

// ResolveAPIKey resolves a presented key to its record's identity.
func (r *Resolver) ResolveAPIKey(ctx context.Context, presented string, _ time.Time) (Identity, error) {
	record, err := r.keys.Verify(ctx, presented)
	if err != nil {
		return Identity{}, err
	}
	return Identity{
		Username:    record.Prefix,
		DisplayName: record.Name,
		Scopes:      record.Scopes,
		Via:         "api-key",
	}, nil
}

// ResolveBearerToken resolves a JWT to its subject's identity.
func (r *Resolver) ResolveBearerToken(_ context.Context, token string, now time.Time) (Identity, error) {
	identity, err := r.tokens.Require(token, now)
	if err != nil {
		return Identity{}, fmt.Errorf("bearer token rejected: %w", err)
	}
	return identity, nil
}
