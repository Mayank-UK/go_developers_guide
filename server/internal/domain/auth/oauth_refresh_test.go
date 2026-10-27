package auth

import (
	"errors"
	"testing"
	"time"
)

// ── OAuth2 provider ─────────────────────────────────────────────────────

func TestOAuthPkcePairMatchesS256(t *testing.T) {
	_, _, _, _, provider, _, _ := newTestEngines()
	pair, err := provider.NewPkcePair()
	if err != nil {
		t.Fatalf("pkce pair: %v", err)
	}
	if len(pair.Verifier) != 64 { // 48 bytes → 64 base64url chars, inside 43–128
		t.Errorf("verifier length = %d, want 64", len(pair.Verifier))
	}
	if pair.Challenge != S256Challenge(pair.Verifier) || len(pair.Challenge) != 43 {
		t.Errorf("challenge = %q, want the 43-char S256 of the verifier", pair.Challenge)
	}
}

func TestOAuthAuthorizeRefusesUnregisteredClientAndRedirect(t *testing.T) {
	ctx := t.Context()
	_, _, _, _, provider, _, _ := newTestEngines()
	pair, _ := provider.NewPkcePair()

	if _, err := provider.Authorize(ctx, AuthorizeRequest{
		ClientID: "evil-cli", RedirectURI: OAuthRedirectURI, Challenge: pair.Challenge, State: "s",
	}, testNow); !errors.Is(err, ErrInvalidGrant) {
		t.Errorf("unknown client = %v, want ErrInvalidGrant", err)
	}
	if _, err := provider.Authorize(ctx, AuthorizeRequest{
		ClientID: OAuthClientID, RedirectURI: "https://attacker.example/callback", Challenge: pair.Challenge, State: "s",
	}, testNow); !errors.Is(err, ErrInvalidGrant) {
		t.Errorf("unregistered redirect = %v, want ErrInvalidGrant", err)
	}
	if _, err := provider.Authorize(ctx, AuthorizeRequest{
		ClientID: OAuthClientID, RedirectURI: OAuthRedirectURI, Challenge: "short", State: "s",
	}, testNow); !errors.Is(err, ErrInvalidGrant) {
		t.Errorf("bad challenge = %v, want ErrInvalidGrant", err)
	}
}

func TestOAuthFullHappyGrantEndsInVerifiedAccessToken(t *testing.T) {
	ctx := t.Context()
	_, _, _, tokens, provider, _, _ := newTestEngines()
	pair, _ := provider.NewPkcePair()

	authorized, err := provider.Authorize(ctx, AuthorizeRequest{
		ClientID: OAuthClientID, RedirectURI: OAuthRedirectURI, Challenge: pair.Challenge, State: "xyz",
	}, testNow)
	if err != nil {
		t.Fatalf("authorize: %v", err)
	}
	if authorized.State != "xyz" || authorized.RedirectTo == "" {
		t.Errorf("authorize result = %+v", authorized)
	}
	exchanged, err := provider.Exchange(ctx, ExchangeRequest{
		Code: authorized.Code, Verifier: pair.Verifier, RedirectURI: OAuthRedirectURI,
	}, testNow)
	if err != nil {
		t.Fatalf("exchange: %v", err)
	}
	if exchanged.TokenType != "Bearer" || exchanged.Username != "ada" {
		t.Errorf("exchange result = %+v", exchanged)
	}
	// The access token is a real JWT: the resource server verifies it
	// statelessly.
	if verdict := tokens.Verify(exchanged.AccessToken, testNow); verdict.Outcome != TokenValid {
		t.Errorf("exchanged token verdict = %+v", verdict)
	}
}

func TestOAuthWrongVerifierBurnsTheCode(t *testing.T) {
	ctx := t.Context()
	_, _, _, _, provider, _, _ := newTestEngines()
	pair, _ := provider.NewPkcePair()
	authorized, _ := provider.Authorize(ctx, AuthorizeRequest{
		ClientID: OAuthClientID, RedirectURI: OAuthRedirectURI, Challenge: pair.Challenge, State: "s",
	}, testNow)

	wrong := ExchangeRequest{Code: authorized.Code, Verifier: pair.Verifier[:40] + "AAAAA", RedirectURI: OAuthRedirectURI}
	if _, err := provider.Exchange(ctx, wrong, testNow); !errors.Is(err, ErrInvalidGrant) {
		t.Fatalf("wrong verifier = %v, want ErrInvalidGrant", err)
	}
	// The code is spent even though the exchange failed — the rightful
	// client gets exactly one more refusal, not a second chance.
	if _, err := provider.Exchange(ctx, ExchangeRequest{
		Code: authorized.Code, Verifier: pair.Verifier, RedirectURI: OAuthRedirectURI,
	}, testNow); !errors.Is(err, ErrInvalidGrant) {
		t.Errorf("code replay after wrong verifier = %v, want ErrInvalidGrant (consumed on first presentation)", err)
	}
}

func TestOAuthCodeReplayAfterSuccessRefused(t *testing.T) {
	ctx := t.Context()
	_, _, _, _, provider, _, _ := newTestEngines()
	pair, _ := provider.NewPkcePair()
	authorized, _ := provider.Authorize(ctx, AuthorizeRequest{
		ClientID: OAuthClientID, RedirectURI: OAuthRedirectURI, Challenge: pair.Challenge, State: "s",
	}, testNow)
	good := ExchangeRequest{Code: authorized.Code, Verifier: pair.Verifier, RedirectURI: OAuthRedirectURI}
	if _, err := provider.Exchange(ctx, good, testNow); err != nil {
		t.Fatalf("first exchange: %v", err)
	}
	if _, err := provider.Exchange(ctx, good, testNow); !errors.Is(err, ErrInvalidGrant) {
		t.Errorf("replay = %v, want ErrInvalidGrant", err)
	}
}

func TestOAuthExpiredCodeRefused(t *testing.T) {
	ctx := t.Context()
	_, _, _, _, provider, _, _ := newTestEngines()
	pair, _ := provider.NewPkcePair()
	authorized, _ := provider.Authorize(ctx, AuthorizeRequest{
		ClientID: OAuthClientID, RedirectURI: OAuthRedirectURI, Challenge: pair.Challenge, State: "s",
	}, testNow)
	_, err := provider.Exchange(ctx, ExchangeRequest{
		Code: authorized.Code, Verifier: pair.Verifier, RedirectURI: OAuthRedirectURI,
	}, testNow.Add(AuthorizationCodeTTL+time.Second))
	if !errors.Is(err, ErrInvalidGrant) {
		t.Errorf("expired code = %v, want ErrInvalidGrant", err)
	}
}

// ── Refresh rotation ────────────────────────────────────────────────────

func TestRefreshRotationWalksPositionsAndDetectsReuse(t *testing.T) {
	ctx := t.Context()
	_, _, _, _, _, refresh, _ := newTestEngines()
	login, err := refresh.Login(ctx, "ada", "correct-horse", testNow)
	if err != nil {
		t.Fatalf("login: %v", err)
	}

	first, err := refresh.Exchange(ctx, login.RefreshToken, testNow)
	if err != nil || first.Outcome != RefreshRotated || first.RotatedFrom != 1 {
		t.Fatalf("first exchange: %+v %v", first, err)
	}
	second, err := refresh.Exchange(ctx, first.RefreshToken, testNow)
	if err != nil || second.Outcome != RefreshRotated || second.RotatedFrom != 2 {
		t.Fatalf("second exchange: %+v %v", second, err)
	}

	// REUSE: present the already-spent position-2 token.
	reuse, err := refresh.Exchange(ctx, first.RefreshToken, testNow)
	if err != nil {
		t.Fatalf("reuse exchange errored instead of alarming: %v", err)
	}
	if reuse.Outcome != RefreshReuseDetected || reuse.FamilyRevoked != 3 {
		t.Errorf("reuse = %+v, want reuse-detected with 3 tokens killed", reuse)
	}

	// The alarm works: even the NEWEST token is dead with the family.
	if _, err := refresh.Exchange(ctx, second.RefreshToken, testNow); !errors.Is(err, ErrInvalidRefresh) {
		t.Errorf("post-revocation exchange = %v, want ErrInvalidRefresh", err)
	}
}

func TestRefreshLoginRejectsBadCredentialsAndExchangeRejectsUnknowns(t *testing.T) {
	ctx := t.Context()
	_, _, _, _, _, refresh, _ := newTestEngines()
	if _, err := refresh.Login(ctx, "ghost", "x", testNow); !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("unknown user = %v", err)
	}
	if _, err := refresh.Login(ctx, "ada", "wrong", testNow); !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("wrong password = %v", err)
	}
	if _, err := refresh.Exchange(ctx, "rt_unknown", testNow); !errors.Is(err, ErrInvalidRefresh) {
		t.Errorf("unknown token = %v", err)
	}
}

func TestRefreshLeaseExpiryRefused(t *testing.T) {
	ctx := t.Context()
	_, _, _, _, _, refresh, _ := newTestEngines()
	login, err := refresh.Login(ctx, "ada", "correct-horse", testNow)
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	if _, err := refresh.Exchange(ctx, login.RefreshToken, testNow.Add(RefreshTTL+time.Minute)); !errors.Is(err, ErrInvalidRefresh) {
		t.Errorf("expired lease = %v, want ErrInvalidRefresh", err)
	}
}

// ── Resolver (the middleware's seam) ────────────────────────────────────

func TestResolverResolvesAllThreeCarriers(t *testing.T) {
	ctx := t.Context()
	_, sessions, keys, tokens, _, _, resolver := newTestEngines()

	login, _ := sessions.Login(ctx, "ada", "correct-horse", testNow)
	viaSession, err := resolver.ResolveSessionCookie(ctx, login.Cookie.Value, testNow)
	if err != nil || viaSession.Via != "session" || viaSession.Username != "ada" {
		t.Errorf("session resolve = %+v %v", viaSession, err)
	}
	created, _ := keys.Generate(ctx, "k", nil, testNow)
	viaKey, err := resolver.ResolveAPIKey(ctx, created.FullKey, testNow)
	if err != nil || viaKey.Via != "api-key" || viaKey.Username != created.Prefix {
		t.Errorf("key resolve = %+v %v", viaKey, err)
	}
	minted, _ := tokens.Mint(MintTokenRequest{Subject: "ada", Now: testNow})
	viaToken, err := resolver.ResolveBearerToken(ctx, minted.Token, testNow)
	if err != nil || viaToken.Via != "jwt" || viaToken.Username != "ada" {
		t.Errorf("token resolve = %+v %v", viaToken, err)
	}

	if _, err := resolver.ResolveSessionCookie(ctx, "junk", testNow); !errors.Is(err, ErrNotAuthenticated) {
		t.Errorf("junk cookie = %v", err)
	}
	if _, err := resolver.ResolveAPIKey(ctx, "junk", testNow); !errors.Is(err, ErrNotAuthenticated) {
		t.Errorf("junk key = %v", err)
	}
	if _, err := resolver.ResolveBearerToken(ctx, "junk", testNow); !errors.Is(err, ErrNotAuthenticated) {
		t.Errorf("junk token = %v", err)
	}
}
