package auth

// The seven auth demo stories, half two: the JWT verdict lab, the OAuth
// code+PKCE grant, the argon2id hashing lab, and refresh rotation with
// reuse detection. Same discipline as half one: real engines, honest
// timings, raw rows for the builders to derive from.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	domauth "guide/internal/domain/auth"
	"guide/internal/domain/demos"
)

// RunAuthJWTStory mints one token and attacks it six ways, each
// classified by the real verifier: the honest verify, a forged
// signature (minted with the attacker's key), a tampered payload under
// the original signature, the alg=none attack, an expired lease, and a
// not-yet-valid lease.
func (c *Console) RunAuthJWTStory(_ context.Context) (demos.AuthJWTRun, error) {
	now := clock()

	minted, err := c.Tokens.Mint(domauth.MintTokenRequest{
		Subject: "ada", Scopes: []string{"posts:read", "posts:write"}, Now: now,
	})
	if err != nil {
		return demos.AuthJWTRun{}, fmt.Errorf("jwt story mint: %w", err)
	}
	forged, err := c.Tokens.Mint(domauth.MintTokenRequest{
		Subject: "admin", Scopes: []string{"posts:write"}, SignWith: "forged", Now: now,
	})
	if err != nil {
		return demos.AuthJWTRun{}, fmt.Errorf("jwt story forge: %w", err)
	}
	claims := map[string]any{
		"sub": "admin", "iat": now.Unix(), "exp": now.Add(time.Hour).Unix(), "nbf": now.Unix(),
		"iss": domauth.TokenIssuer, "scopes": []string{"posts:write"}, "jti": "tampered",
	}
	tampered, err := c.Tokens.TamperPayload(minted.Token, claims)
	if err != nil {
		return demos.AuthJWTRun{}, fmt.Errorf("jwt story tamper: %w", err)
	}
	unsigned, err := domauth.StripSignatureForAlgNone(minted.Token)
	if err != nil {
		return demos.AuthJWTRun{}, fmt.Errorf("jwt story alg-none: %w", err)
	}
	expired, err := c.Tokens.Mint(domauth.MintTokenRequest{Subject: "ada", TTL: -time.Minute, Now: now})
	if err != nil {
		return demos.AuthJWTRun{}, fmt.Errorf("jwt story expire: %w", err)
	}
	future, err := c.Tokens.Mint(domauth.MintTokenRequest{Subject: "ada", NotBefore: time.Hour, Now: now})
	if err != nil {
		return demos.AuthJWTRun{}, fmt.Errorf("jwt story nbf: %w", err)
	}

	verifyAt := clock()
	cases := []struct {
		name  string
		token string
	}{
		{"honest verify", minted.Token},
		{"forged signature", forged.Token},
		{"tampered payload (claims edited, signature kept)", tampered},
		{"alg=none attack", unsigned},
		{"expired token", expired.Token},
		{"not-yet-valid token", future.Token},
	}
	verdicts := make([]demos.AuthJWTVerdictRow, 0, len(cases))
	for _, tc := range cases {
		verdict := c.Tokens.Verify(tc.token, verifyAt)
		verdicts = append(verdicts, demos.AuthJWTVerdictRow{
			Case:         tc.name,
			TokenPreview: preview(tc.token, 24),
			Outcome:      verdict.Outcome,
			Reason:       verdict.Reason,
		})
	}

	headerJSON, _ := json.Marshal(minted.Header)
	payloadJSON, _ := json.Marshal(minted.Payload)
	parts := strings.Split(minted.Token, ".")

	return demos.AuthJWTRun{
		Subject:        "ada",
		TokenPreview:   preview(minted.Token, 36),
		HeaderJSON:     string(headerJSON),
		PayloadJSON:    string(payloadJSON),
		SignatureChars: len(parts[2]),
		ExpiresAt:      minted.ExpiresAt.Format(time.RFC3339),
		Verdicts:       verdicts,
	}, nil
}

// RunAuthOAuthStory runs the code+PKCE grant end to end: the client's
// local PKCE pair, the authorize redirect (consent simulated, code
// one-time), the exchange, the stateless verdict on the access token, a
// code replay, and a wrong-verifier exchange that BURNS a fresh code.
func (c *Console) RunAuthOAuthStory(ctx context.Context) (demos.AuthOAuthRun, error) {
	now := clock()

	pair, err := c.OAuth.NewPkcePair()
	if err != nil {
		return demos.AuthOAuthRun{}, fmt.Errorf("oauth story pkce: %w", err)
	}
	authorized, err := c.OAuth.Authorize(ctx, domauth.AuthorizeRequest{
		ClientID:    domauth.OAuthClientID,
		RedirectURI: domauth.OAuthRedirectURI,
		Challenge:   pair.Challenge,
		State:       "xyz-anti-csrf",
	}, now)
	if err != nil {
		return demos.AuthOAuthRun{}, fmt.Errorf("oauth story authorize: %w", err)
	}
	exchanged, err := c.OAuth.Exchange(ctx, domauth.ExchangeRequest{
		Code: authorized.Code, Verifier: pair.Verifier, RedirectURI: domauth.OAuthRedirectURI,
	}, clock())
	if err != nil {
		return demos.AuthOAuthRun{}, fmt.Errorf("oauth story exchange: %w", err)
	}
	accessOutcome := c.Tokens.Verify(exchanged.AccessToken, clock()).Outcome

	// Replay the spent code.
	_, replayErr := c.OAuth.Exchange(ctx, domauth.ExchangeRequest{
		Code: authorized.Code, Verifier: pair.Verifier, RedirectURI: domauth.OAuthRedirectURI,
	}, clock())

	// Fresh code, WRONG verifier — then prove the code burned by
	// trying again with the RIGHT one.
	fresh, err := c.OAuth.Authorize(ctx, domauth.AuthorizeRequest{
		ClientID:    domauth.OAuthClientID,
		RedirectURI: domauth.OAuthRedirectURI,
		Challenge:   pair.Challenge,
		State:       "second-round",
	}, clock())
	if err != nil {
		return demos.AuthOAuthRun{}, fmt.Errorf("oauth story re-authorize: %w", err)
	}
	wrong := pair.Verifier[:40] + "AAAAA"
	_, wrongErr := c.OAuth.Exchange(ctx, domauth.ExchangeRequest{
		Code: fresh.Code, Verifier: wrong, RedirectURI: domauth.OAuthRedirectURI,
	}, clock())
	_, burnedErr := c.OAuth.Exchange(ctx, domauth.ExchangeRequest{
		Code: fresh.Code, Verifier: pair.Verifier, RedirectURI: domauth.OAuthRedirectURI,
	}, clock())

	reasonOf := func(err error) string {
		if err == nil {
			return "ACCEPTED — reported as it happened; this would be an engine defect"
		}
		return err.Error()
	}

	return demos.AuthOAuthRun{
		VerifierPreview:      preview(pair.Verifier, 8),
		ChallengePreview:     preview(pair.Challenge, 12),
		State:                authorized.State,
		RedirectTo:           authorized.RedirectTo,
		CodePreview:          preview(authorized.Code, 8),
		AccessTokenPreview:   preview(exchanged.AccessToken, 24),
		Scope:                exchanged.Scope,
		Username:             exchanged.Username,
		AccessTokenOutcome:   accessOutcome,
		ReplayRefused:        reasonOf(replayErr),
		WrongVerifierRefused: reasonOf(wrongErr),
		BurnedCodeProof:      reasonOf(burnedErr),
	}, nil
}

// RunAuthHashingStory is the argon2id lab: the same password hashed
// twice (fresh salts), a correct verify, a wrong verify (full cost), a
// legacy-cost hash (verifies, fires the rehash signal), the uniform
// unknown-user burn, and a malformed stored hash.
func (c *Console) RunAuthHashingStory(ctx context.Context) (demos.AuthHashingRun, error) {
	hasher := domauth.NewHasher()
	password := "correct-horse"

	first, err := hasher.Hash(password)
	if err != nil {
		return demos.AuthHashingRun{}, fmt.Errorf("hashing story first hash: %w", err)
	}
	second, err := hasher.Hash(password)
	if err != nil {
		return demos.AuthHashingRun{}, fmt.Errorf("hashing story second hash: %w", err)
	}

	correctStart := time.Now()
	correctErr := hasher.Verify(password, first)
	correctMs := domauth.Elapsed(time.Since(correctStart))

	wrongStart := time.Now()
	wrongErr := hasher.Verify("wrong-horse", first)
	wrongMs := domauth.Elapsed(time.Since(wrongStart))

	legacyHasher := &domauth.Hasher{Params: domauth.LegacyHashParams}
	legacy := legacyHasher.HashWithSalt(password, []byte("legacy-salt-----"))
	legacyStart := time.Now()
	legacyErr := legacyHasher.Verify(password, legacy)
	legacyMs := domauth.Elapsed(time.Since(legacyStart))

	// The uniform decoy burn: same work as a real wrong password.
	unknownStart := time.Now()
	hasher.VerifyUniform(password, "")
	unknownMs := domauth.Elapsed(time.Since(unknownStart))

	malformedErr := hasher.Verify(password, "not-a-phc-string")

	// Touch the console's seeded directory so the story and the shared
	// substrate agree the engine is the same one the logins use.
	if _, _, err := c.Directory.FindUser(ctx, "ada"); err != nil {
		return demos.AuthHashingRun{}, fmt.Errorf("hashing story directory: %w", err)
	}

	malformed := "refused as a LOUD error, not a failed match: “" + malformedErr.Error() + "” — stale rows deserve a loud failure, not a silent reject"
	if malformedErr == nil {
		malformed = "ACCEPTED — reported as it happened; this would be an engine defect"
	}

	return demos.AuthHashingRun{
		Parameters:         hasher.Parameters(),
		FirstHashPreview:   preview(first, 30),
		SecondHashPreview:  preview(second, 30),
		Distinct:           first != second,
		CorrectMs:          correctMs,
		CorrectNeedsRehash: hasher.NeedsRehash(first),
		WrongMs:            wrongMs,
		LegacyParameters:   "argon2id · m=8192 KiB · t=1 · p=2 (2015-era cost)",
		LegacyNeedsRehash:  hasher.NeedsRehash(legacy),
		LegacyMs:           legacyMs,
		UnknownUserMs:      unknownMs,
		MalformedRefused:   malformed,
	}, func() error {
		if correctErr != nil {
			return fmt.Errorf("correct password must verify: %w", correctErr)
		}
		if !errors.Is(wrongErr, domauth.ErrInvalidCredentials) {
			return fmt.Errorf("wrong password must refuse: %w", wrongErr)
		}
		if legacyErr != nil {
			return fmt.Errorf("legacy-cost hash must still verify: %w", legacyErr)
		}
		return nil
	}()
}

// RunAuthRefreshStory walks one family through its whole life: born at
// login (position 1), rotated twice, KILLED by reuse (the spent
// position-2 token presented again), and the post-revocation refusal of
// even the newest token.
func (c *Console) RunAuthRefreshStory(ctx context.Context) (demos.AuthRefreshRun, error) {
	now := clock()

	login, err := c.Refresh.Login(ctx, "ada", "correct-horse", now)
	if err != nil {
		return demos.AuthRefreshRun{}, fmt.Errorf("refresh story login: %w", err)
	}

	first, err := c.Refresh.Exchange(ctx, login.RefreshToken, clock())
	if err != nil {
		return demos.AuthRefreshRun{}, fmt.Errorf("refresh story first exchange: %w", err)
	}
	second, err := c.Refresh.Exchange(ctx, first.RefreshToken, clock())
	if err != nil {
		return demos.AuthRefreshRun{}, fmt.Errorf("refresh story second exchange: %w", err)
	}

	// REUSE: the already-spent position-2 token (first.RotatedFrom==1 →
	// minted position 2) is presented again.
	reuse, err := c.Refresh.Exchange(ctx, first.RefreshToken, clock())
	if err != nil {
		return demos.AuthRefreshRun{}, fmt.Errorf("refresh story reuse: %w", err)
	}
	_, postReuseErr := c.Refresh.Exchange(ctx, second.RefreshToken, clock())

	postReuse := "refused — the family is revoked, only a fresh login starts a new one"
	if postReuseErr == nil {
		postReuse = "ACCEPTED — reported as it happened; this would be an engine defect"
	}

	return demos.AuthRefreshRun{
		FamilyID:          login.FamilyID,
		Username:          login.Identity.Username,
		LoginTokenPreview: preview(login.RefreshToken, 8),
		Rotations: []demos.AuthRefreshRotationRow{
			{From: first.RotatedFrom, To: first.RotatedFrom + 1},
			{From: second.RotatedFrom, To: second.RotatedFrom + 1},
		},
		ReusePosition:    reuse.RotatedFrom,
		TokensRevoked:    reuse.FamilyRevoked,
		PostReuseRefused: postReuse,
	}, nil
}
