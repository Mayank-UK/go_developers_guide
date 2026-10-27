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

// The one registered client — a PUBLIC client: no secret on the
// machine, so PKCE is required, not optional.
const (
	OAuthClientID   = "guide-cli"
	OAuthClientName = "Guide CLI (public client)"
)

// OAuthRedirectURI is the exact-match redirect the client registered.
// An unregistered redirect_uri is REFUSED, never redirected — an open
// redirect with a code attached is a credential leak.
const OAuthRedirectURI = "https://client.example/callback"

// OAuthClientScopes are the scopes this client may ask consent for.
var OAuthClientScopes = []string{"posts:read", "posts:write"}

// PKCE length bounds from RFC 7636: the verifier is 43–128 base64url
// characters; the guide mints 48 bytes → 64 characters, inside the
// window.
const (
	PKCEVerifierMin = 43
	PKCEVerifierMax = 128
)

// OAuthProvider is the guide's teaching authorization server: the
// authorization-code grant with PKCE (S256), honest about the one
// simulated step (the resource owner's consent) and real about
// everything else — registered clients, exact-match redirects,
// one-time codes stored hashed and consumed on FIRST presentation,
// constant-time challenge comparison. No external provider is called:
// every party (client, authorization server, resource server) lives in
// this process, and the topic names which hat each beat wears.
type OAuthProvider struct {
	codes  AuthorizationCodeStore
	tokens *TokenService
}

// NewOAuthProvider wires the provider over its ports.
func NewOAuthProvider(codes AuthorizationCodeStore, tokens *TokenService) *OAuthProvider {
	return &OAuthProvider{codes: codes, tokens: tokens}
}

// PkcePair is what a client generates locally before the dance begins:
// the verifier (kept secret on the client) and its S256 challenge (the
// only half that travels the authorize request).
type PkcePair struct {
	Verifier  string
	Challenge string
}

// NewPkcePair mints a verifier/challenge pair — RFC 7636 §4.2:
// challenge = base64url(SHA-256(verifier)), padding stripped.
func (p *OAuthProvider) NewPkcePair() (PkcePair, error) {
	verifier, err := randomToken(48)
	if err != nil {
		return PkcePair{}, err
	}
	return PkcePair{Verifier: verifier, Challenge: S256Challenge(verifier)}, nil
}

// S256Challenge derives the challenge from a verifier: one SHA-256,
// base64url-encoded, padding stripped — the one-way trip PKCE stands on.
func S256Challenge(verifier string) string {
	digest := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(digest[:])
}

// AuthorizeRequest is the front-channel ask: who the client is, where
// it registered its redirect, its PKCE challenge, and the opaque state
// the client demands back unchanged.
type AuthorizeRequest struct {
	ClientID      string
	RedirectURI   string
	Challenge     string
	State         string
	ResourceOwner string // the (simulated) user consenting — "ada" in every demo run
}

// AuthorizeResult is the authorize answer: the redirect URL a real
// client would follow, code and state riding the query string.
type AuthorizeResult struct {
	RedirectTo     string
	Code           string
	State          string
	ConsentingUser string
	ExpiresAt      time.Time
}

// Authorize validates the client, the URI, and the challenge, then
// issues the ONE-TIME code bound to all three. The consent is the
// demo's one simulation: the named resource owner grants the requested
// scopes, exactly what a real consent screen records.
func (p *OAuthProvider) Authorize(ctx context.Context, req AuthorizeRequest, now time.Time) (AuthorizeResult, error) {
	if req.ClientID != OAuthClientID {
		return AuthorizeResult{}, fmt.Errorf("%w: unknown client %q — only %q is registered",
			ErrInvalidGrant, req.ClientID, OAuthClientID)
	}
	if req.RedirectURI != OAuthRedirectURI {
		return AuthorizeResult{}, fmt.Errorf("%w: redirect_uri is not registered for this client — refusing rather than redirecting a code somewhere new",
			ErrInvalidGrant)
	}
	if len(req.Challenge) != 43 { // an S256 digest is exactly 43 base64url chars
		return AuthorizeResult{}, fmt.Errorf("%w: code_challenge must be an S256 digest (43 base64url characters)",
			ErrInvalidGrant)
	}
	owner := req.ResourceOwner
	if owner == "" {
		owner = "ada"
	}
	code, err := randomToken(32)
	if err != nil {
		return AuthorizeResult{}, err
	}
	expiresAt := now.Add(AuthorizationCodeTTL)
	if err := p.codes.IssueCode(ctx, StoredAuthorizationCode{
		SHA256:        SHA256Hex(code),
		ClientID:      req.ClientID,
		RedirectURI:   req.RedirectURI,
		CodeChallenge: req.Challenge,
		Username:      owner,
		Scopes:        OAuthClientScopes,
		IssuedAt:      now,
		ExpiresAt:     expiresAt,
	}); err != nil {
		return AuthorizeResult{}, fmt.Errorf("issue authorization code: %w", err)
	}
	separator := "?"
	if strings.Contains(req.RedirectURI, "?") {
		separator = "&"
	}
	return AuthorizeResult{
		RedirectTo:     fmt.Sprintf("%s%scode=%s&state=%s", req.RedirectURI, separator, code, req.State),
		Code:           code,
		State:          req.State,
		ConsentingUser: owner,
		ExpiresAt:      expiresAt,
	}, nil
}

// ExchangeRequest is the back-channel presentation: the one-time code,
// the verifier that hashed to the challenge, and the redirect the code
// was bound to.
type ExchangeRequest struct {
	Code        string
	Verifier    string
	RedirectURI string
}

// ExchangeResult is the token answer on success.
type ExchangeResult struct {
	AccessToken string
	TokenType   string
	ExpiresIn   time.Duration
	Scope       string
	Username    string
}

// Exchange is the back door's twin: the client presents the ONE-TIME
// code plus the verifier that hashed to the challenge. The code is
// consumed FIRST — even a failed exchange spends it — so an attacker
// who steals the code and guesses verifiers gets exactly one guess
// before the code is ash. The challenge comparison is constant-time.
func (p *OAuthProvider) Exchange(ctx context.Context, req ExchangeRequest, now time.Time) (ExchangeResult, error) {
	stored, found, err := p.codes.TakeCode(ctx, SHA256Hex(req.Code))
	if err != nil {
		return ExchangeResult{}, fmt.Errorf("take authorization code: %w", err)
	}
	if !found {
		return ExchangeResult{}, fmt.Errorf("%w: the code is unknown, already used, or expired (one attempt: codes are consumed on first presentation)",
			ErrInvalidGrant)
	}
	if !now.Before(stored.ExpiresAt) {
		return ExchangeResult{}, fmt.Errorf("%w: the code's %s window has closed", ErrInvalidGrant, AuthorizationCodeTTL)
	}
	if len(req.Verifier) < PKCEVerifierMin || len(req.Verifier) > PKCEVerifierMax || !isBase64URL(req.Verifier) {
		return ExchangeResult{}, fmt.Errorf("%w: the verifier must be 43–128 base64url characters", ErrInvalidGrant)
	}
	if req.RedirectURI != stored.RedirectURI {
		return ExchangeResult{}, fmt.Errorf("%w: redirect_uri does not match the authorize request", ErrInvalidGrant)
	}
	if !hmac.Equal([]byte(S256Challenge(req.Verifier)), []byte(stored.CodeChallenge)) {
		return ExchangeResult{}, fmt.Errorf("%w: S256(verifier) does not match the code_challenge — wrong verifier, and the code is spent",
			ErrInvalidGrant)
	}
	minted, err := p.tokens.Mint(MintTokenRequest{
		Subject: stored.Username,
		Scopes:  stored.Scopes,
		TTL:     AccessTokenTTL,
		Now:     now,
	})
	if err != nil {
		return ExchangeResult{}, err
	}
	return ExchangeResult{
		AccessToken: minted.Token,
		TokenType:   "Bearer",
		ExpiresIn:   AccessTokenTTL,
		Scope:       strings.Join(stored.Scopes, " "),
		Username:    stored.Username,
	}, nil
}

// isBase64URL checks RFC 7636 §4.1's alphabet: unreserved ASCII only.
func isBase64URL(value string) bool {
	for _, c := range value {
		switch {
		case c >= 'A' && c <= 'Z', c >= 'a' && c <= 'z', c >= '0' && c <= '9', c == '-', c == '_':
		default:
			return false
		}
	}
	return len(value) > 0
}
