package auth

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// TokenIssuer is the iss claim every minted token carries — and the one
// verification demands back.
const TokenIssuer = "guide.demo"

// TokenAlgorithm is the one algorithm this server speaks: HS256,
// symmetric signing (one secret signs and verifies; RS256 splits the
// pair — a comparison the topic makes in prose).
const TokenAlgorithm = "HS256"

// Token outcome labels — the closed set verification can end in. The
// order of the checks matters and is the teaching: a token whose header
// says "none" is an alg-none rejection even before any signature exists
// to check, and a forged token fails as a FORGERY even if its claims
// would also have expired.
const (
	TokenValid        = "valid"
	TokenExpired      = "expired"
	TokenNotYetValid  = "not-yet-valid"
	TokenBadSignature = "bad-signature"
	TokenAlgNone      = "alg-none-rejected"
	TokenWrongAlg     = "wrong-alg"
	TokenMalformed    = "malformed"
)

// TokenService mints and verifies HS256 JWTs with the STANDARD LIBRARY
// ONLY — crypto/hmac, crypto/sha256, encoding/base64, encoding/json —
// because a JWT is three base64url parts and an HMAC, and seeing that
// plainly beats importing a dependency for it (the guide names the
// third-party options in prose; none is on the module graph).
type TokenService struct {
	secret []byte
}

// NewTokenService wires the engine over its signing secret.
func NewTokenService(secret []byte) *TokenService {
	return &TokenService{secret: secret}
}

// MintTokenRequest is one mint: the subject, its scopes, the lease, and
// the two seams the demos drive — SignWith ("real" or "forged": the
// lab's attacker key) and Now (zero means the real clock; an explicit
// time bakes expired or not-yet-valid claims without sleeping).
type MintTokenRequest struct {
	Subject   string
	Scopes    []string
	TTL       time.Duration
	NotBefore time.Duration
	SignWith  string
	Now       time.Time
}

// MintedToken carries the token plus its decoded anatomy — the header
// and claims as maps, so a panel can show all three parts honestly.
type MintedToken struct {
	Token     string
	Header    map[string]string
	Payload   map[string]any
	ExpiresAt time.Time
}

// Mint builds the claim set a production token carries (sub, iat, exp,
// nbf, iss, scopes, jti), signs it HS256, and answers token + anatomy.
// Timestamps are seconds since the epoch — the JWT register's rule.
func (t *TokenService) Mint(req MintTokenRequest) (MintedToken, error) {
	now := req.Now
	if now.IsZero() {
		now = time.Now()
	}
	ttl := req.TTL
	if ttl == 0 {
		ttl = AccessTokenTTL
	}
	issuedAt := now.Unix()
	scopes := req.Scopes
	if scopes == nil {
		scopes = []string{}
	}
	jti, err := randomToken(16)
	if err != nil {
		return MintedToken{}, err
	}
	header := map[string]string{"alg": TokenAlgorithm, "typ": "JWT"}
	payload := map[string]any{
		"sub":    req.Subject,
		"iat":    issuedAt,
		"exp":    issuedAt + int64(ttl.Seconds()),
		"nbf":    issuedAt + int64(req.NotBefore.Seconds()),
		"iss":    TokenIssuer,
		"scopes": scopes,
		"jti":    jti,
	}
	secret := t.secret
	if req.SignWith == "forged" {
		secret = []byte("attacker-secret-that-the-server-does-not-know")
	}
	token, err := t.sign(header, payload, secret)
	if err != nil {
		return MintedToken{}, err
	}
	return MintedToken{
		Token:     token,
		Header:    header,
		Payload:   payload,
		ExpiresAt: time.Unix(payload["exp"].(int64), 0).UTC(),
	}, nil
}

// TokenVerdict is one verification's answer: the classified outcome,
// the identity it resolves to when valid, and a reason sentence for
// every rejection — none of which ever includes the secret.
type TokenVerdict struct {
	Outcome   string
	Subject   string
	Scopes    []string
	ExpiresAt time.Time
	Reason    string
}

// Verify classifies a presented token. The check order IS the lesson:
// split and decode first (malformed), then the header's alg claim
// (alg=none and any non-HS256 are refused BEFORE a signature is even
// recomputed — the algorithm-confusion attack dies at the door), then
// the signature (constant-time hmac.Equal), then the required claims,
// then the time windows, then the issuer.
func (t *TokenService) Verify(token string, now time.Time) TokenVerdict {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return malformedVerdict("not three dots-joined parts — wrong shape or garbage")
	}
	var header struct {
		Alg string `json:"alg"`
	}
	if err := decodePart(parts[0], &header); err != nil {
		return malformedVerdict("the header does not decode as JSON")
	}
	if header.Alg == "none" {
		return TokenVerdict{Outcome: TokenAlgNone,
			Reason: "the header declares alg=none — an UNSIGNED token. A server that trusts the header's algorithm instead of insisting on its own would accept this; this one refuses before any signature exists to check"}
	}
	if header.Alg != TokenAlgorithm {
		return TokenVerdict{Outcome: TokenWrongAlg,
			Reason: fmt.Sprintf("the header declares alg=%s; this server verifies %s only — the algorithm is the SERVER's choice, never the token's", header.Alg, TokenAlgorithm)}
	}
	got, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return malformedVerdict("the signature does not decode as base64url")
	}
	want := t.signature(parts[0], parts[1])
	if !hmac.Equal(got, want) {
		return TokenVerdict{Outcome: TokenBadSignature,
			Reason: "the signature does not match the server's secret over this exact header and payload — a forged or tampered token"}
	}
	var claims struct {
		Subject string   `json:"sub"`
		Issued  int64    `json:"iat"`
		Expires int64    `json:"exp"`
		NotBef  int64    `json:"nbf"`
		Issuer  string   `json:"iss"`
		Scopes  []string `json:"scopes"`
	}
	if err := decodePart(parts[1], &claims); err != nil {
		return malformedVerdict("the payload does not decode as JSON")
	}
	if claims.Subject == "" || claims.Expires == 0 || claims.Issued == 0 || claims.NotBef == 0 || claims.Issuer == "" {
		return malformedVerdict("a required claim (sub, exp, iat, nbf, iss) is missing")
	}
	moment := now.Unix()
	if moment >= claims.Expires {
		return TokenVerdict{Outcome: TokenExpired,
			Reason: "the exp claim is in the past — the token's lease ran out"}
	}
	if moment < claims.NotBef {
		return TokenVerdict{Outcome: TokenNotYetValid,
			Reason: "the nbf claim is in the future — not valid YET"}
	}
	if claims.Issuer != TokenIssuer {
		return TokenVerdict{Outcome: TokenBadSignature,
			Reason: fmt.Sprintf("the iss claim is %q; this server only accepts %q", claims.Issuer, TokenIssuer)}
	}
	scopes := claims.Scopes
	if scopes == nil {
		scopes = []string{}
	}
	return TokenVerdict{
		Outcome:   TokenValid,
		Subject:   claims.Subject,
		Scopes:    scopes,
		ExpiresAt: time.Unix(claims.Expires, 0).UTC(),
		Reason: fmt.Sprintf("signature verified against the server secret and every claim holds: %s, exp in the future, nbf in the past — nothing was consulted but the token itself",
			claims.Subject),
	}
}

// Require is the protected-endpoint check: a valid verdict resolves to
// the identity, every other outcome is one typed unauthorized naming
// its classification — safe to tell a token-holder, never the secret.
func (t *TokenService) Require(token string, now time.Time) (Identity, error) {
	verdict := t.Verify(token, now)
	if verdict.Outcome != TokenValid {
		return Identity{}, fmt.Errorf("%w: invalid token — %s", ErrNotAuthenticated, verdict.Outcome)
	}
	return Identity{Username: verdict.Subject, DisplayName: verdict.Subject, Scopes: verdict.Scopes, Via: "jwt"}, nil
}

// Login is the token flow's server half: credentials in, one bearer
// token out — the same timing-uniform credential check the session
// engine uses.
func (t *TokenService) Login(ctx context.Context, directory UserDirectory, passwords *Hasher, username, password string, now time.Time) (MintedToken, Identity, error) {
	user, _, err := directory.FindUser(ctx, username)
	if err != nil {
		return MintedToken{}, Identity{}, fmt.Errorf("find user %q: %w", username, err)
	}
	// Empty hash on an unknown user burns the decoy — see SessionService.
	if !passwords.VerifyUniform(password, user.PasswordHash) {
		return MintedToken{}, Identity{}, ErrInvalidCredentials
	}
	minted, err := t.Mint(MintTokenRequest{Subject: user.Username, Scopes: user.Scopes, Now: now})
	if err != nil {
		return MintedToken{}, Identity{}, err
	}
	return minted, Identity{
		Username:    user.Username,
		DisplayName: user.DisplayName,
		Scopes:      user.Scopes,
		Via:         "jwt",
	}, nil
}

// TamperPayload rebuilds a token with an EDITED payload under the
// original signature — the demo's tampering attack, kept in the engine
// so the story builder cannot drift from the wire format.
func (t *TokenService) TamperPayload(token string, payload map[string]any) (string, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return "", fmt.Errorf("tamper: token is not three parts")
	}
	encoded, err := encodePart(payload)
	if err != nil {
		return "", err
	}
	return parts[0] + "." + encoded + "." + parts[2], nil
}

// StripSignatureForAlgNone rebuilds a token with alg=none and an EMPTY
// signature — the classic unsigned-token attack, byte-accurate.
func StripSignatureForAlgNone(token string) (string, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return "", fmt.Errorf("alg=none attack: token is not three parts")
	}
	header, err := encodePart(map[string]string{"alg": "none", "typ": "JWT"})
	if err != nil {
		return "", err
	}
	return header + "." + parts[1] + ".", nil
}

func (t *TokenService) sign(header map[string]string, payload map[string]any, secret []byte) (string, error) {
	encodedHeader, err := encodePart(header)
	if err != nil {
		return "", fmt.Errorf("encode header: %w", err)
	}
	encodedPayload, err := encodePart(payload)
	if err != nil {
		return "", fmt.Errorf("encode payload: %w", err)
	}
	signingInput := encodedHeader + "." + encodedPayload
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(signingInput))
	return signingInput + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), nil
}

func (t *TokenService) signature(encodedHeader, encodedPayload string) []byte {
	mac := hmac.New(sha256.New, t.secret)
	mac.Write([]byte(encodedHeader + "." + encodedPayload))
	return mac.Sum(nil)
}

func encodePart(v any) (string, error) {
	encoded, err := json.Marshal(v)
	if err != nil {
		return "", fmt.Errorf("marshal JWT part: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(encoded), nil
}

func decodePart(encoded string, v any) error {
	raw, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return fmt.Errorf("decode base64url: %w", err)
	}
	if err := json.Unmarshal(raw, v); err != nil {
		return fmt.Errorf("decode JSON: %w", err)
	}
	return nil
}

func malformedVerdict(reason string) TokenVerdict {
	return TokenVerdict{Outcome: TokenMalformed, Reason: reason}
}
