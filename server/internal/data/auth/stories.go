package auth

// The seven auth demo stories, half one: sessions, API keys, and the
// three-mechanism comparison. Each runner walks the REAL engines over
// the shared console's state, times what matters with time.Since, and
// reports raw rows — the payload builders in internal/domain/demos
// derive every sentence from these.

import (
	"context"
	"errors"
	"fmt"
	"time"

	domauth "guide/internal/domain/auth"
	"guide/internal/domain/demos"
)

// preview renders a value's first n characters for display.
func preview(value string, n int) string {
	if len(value) <= n {
		return value
	}
	return value[:n] + "…"
}

// RunAuthSessionsStory walks the cookie lifecycle: login (sealed
// cookie), inspect, a one-character tamper, uniform rejections, and the
// logout that deletes the row — ending with the pre-logout value dead.
func (c *Console) RunAuthSessionsStory(ctx context.Context) (demos.AuthSessionsRun, error) {
	now := clock()

	login, err := c.Sessions.Login(ctx, "ada", "correct-horse", now)
	if err != nil {
		return demos.AuthSessionsRun{}, fmt.Errorf("sessions story login: %w", err)
	}
	sealed := login.Cookie.Value
	cookie := login.Cookie

	row, err := c.Sessions.Inspect(ctx, sealed, clock())
	if err != nil {
		return demos.AuthSessionsRun{}, fmt.Errorf("sessions story inspect: %w", err)
	}

	// One flipped character in the sealed value (kept in the base64url
	// alphabet): the seal must fail before the store is consulted.
	tampered := sealed
	if tampered[len(tampered)-1] == 'A' {
		tampered = tampered[:len(tampered)-1] + "B"
	} else {
		tampered = tampered[:len(tampered)-1] + "A"
	}
	_, tamperErr := c.Sessions.Inspect(ctx, tampered, clock())

	// Uniform rejections: wrong password for a known user, and any
	// password for an unknown one — both timed.
	wrongStart := time.Now()
	_, wrongErr := c.Sessions.Login(ctx, "ada", "wrong-horse", clock())
	wrongMs := domauth.Elapsed(time.Since(wrongStart))
	unknownStart := time.Now()
	_, unknownErr := c.Sessions.Login(ctx, "ghost", "whatever", clock())
	unknownMs := domauth.Elapsed(time.Since(unknownStart))

	revoked, _, err := c.Sessions.Logout(ctx, sealed)
	if err != nil {
		return demos.AuthSessionsRun{}, fmt.Errorf("sessions story logout: %w", err)
	}
	_, postLogoutErr := c.Sessions.Inspect(ctx, sealed, clock())

	return demos.AuthSessionsRun{
		Username: login.Identity.Username,
		Cookie: demos.AuthCookieSnapshot{
			Name:          cookie.Name,
			ValuePreview:  preview(sealed, 12),
			MaxAgeSeconds: cookie.MaxAgeSeconds,
			Path:          cookie.Path,
			HTTPOnly:      cookie.HTTPOnly,
			SameSite:      cookie.SameSite,
			Secure:        cookie.Secure,
		},
		CookieAnatomy:     c.Sessions.CookieAnatomy(sealed),
		InspectedUser:     row.Username,
		InspectedScopes:   row.Scopes,
		TamperedRefused:   errors.Is(tamperErr, domauth.ErrNotAuthenticated),
		WrongPasswordMs:   wrongMs,
		UnknownUserMs:     unknownMs,
		UniformRefusals:   errors.Is(wrongErr, domauth.ErrInvalidCredentials) && errors.Is(unknownErr, domauth.ErrInvalidCredentials),
		Revoked:           revoked,
		PostLogoutRefused: errors.Is(postLogoutErr, domauth.ErrNotAuthenticated),
	}, nil
}

// RunAuthAPIKeysStory walks the key lifecycle: generate (raw once),
// roster (hash previews), a good call, two hostile presentations, and
// the revocation that kills every copy.
func (c *Console) RunAuthAPIKeysStory(ctx context.Context) (demos.AuthAPIKeysRun, error) {
	now := clock()

	created, err := c.Keys.Generate(ctx, "ci pipeline", []string{"posts:read"}, now)
	if err != nil {
		return demos.AuthAPIKeysRun{}, fmt.Errorf("keys story generate: %w", err)
	}
	roster, err := c.Keys.List(ctx)
	if err != nil {
		return demos.AuthAPIKeysRun{}, fmt.Errorf("keys story roster: %w", err)
	}
	previews := make([]string, 0, len(roster))
	for _, key := range roster {
		previews = append(previews, "sha256:"+preview(key.KeySHA256, 12))
	}

	record, err := c.Keys.Verify(ctx, created.FullKey)
	if err != nil {
		return demos.AuthAPIKeysRun{}, fmt.Errorf("keys story verify: %w", err)
	}
	_, wrongErr := c.Keys.Verify(ctx, created.Prefix+".totally-wrong-secret")
	_, malformedErr := c.Keys.Verify(ctx, "no-separator-here")

	revoked, err := c.Keys.Revoke(ctx, created.Prefix)
	if err != nil {
		return demos.AuthAPIKeysRun{}, fmt.Errorf("keys story revoke: %w", err)
	}
	_, postRevokeErr := c.Keys.Verify(ctx, created.FullKey)

	return demos.AuthAPIKeysRun{
		Prefix:             created.Prefix,
		Name:               created.Name,
		Scopes:             created.Scopes,
		FullKey:            created.FullKey,
		SHA256Preview:      preview(created.SHA256, 12),
		RosterCount:        len(roster),
		RosterPreviews:     previews,
		VerifiedPrefix:     record.Prefix,
		VerifiedName:       record.Name,
		WrongSecretRefused: errors.Is(wrongErr, domauth.ErrNotAuthenticated),
		MalformedRefused:   errors.Is(malformedErr, domauth.ErrNotAuthenticated),
		Revoked:            revoked,
		PostRevokeRefused:  errors.Is(postRevokeErr, domauth.ErrNotAuthenticated),
	}, nil
}

// RunAuthCompareStory runs ONE identity through all three carriers and
// times each verification honestly: a session (unseal + store lookup),
// an API key (SHA-256 + lookup + constant-time compare), and a JWT
// (pure HMAC crypto) — then revokes what can be revoked and re-verifies
// all three, the JWT's survival being the lesson.
func (c *Console) RunAuthCompareStory(ctx context.Context) (demos.AuthCompareRun, error) {
	now := clock()

	login, err := c.Sessions.Login(ctx, "ada", "correct-horse", now)
	if err != nil {
		return demos.AuthCompareRun{}, fmt.Errorf("compare story login: %w", err)
	}
	key, err := c.Keys.Generate(ctx, "compare key", []string{"posts:read"}, now)
	if err != nil {
		return demos.AuthCompareRun{}, fmt.Errorf("compare story key: %w", err)
	}
	token, err := c.Tokens.Mint(domauth.MintTokenRequest{Subject: "ada", Scopes: []string{"posts:read", "posts:write"}, Now: now})
	if err != nil {
		return demos.AuthCompareRun{}, fmt.Errorf("compare story mint: %w", err)
	}

	// Time the three verifications, honest single-shot measurements.
	sessionStart := time.Now()
	if _, err := c.Sessions.Inspect(ctx, login.Cookie.Value, clock()); err != nil {
		return demos.AuthCompareRun{}, fmt.Errorf("compare story session verify: %w", err)
	}
	sessionMs := domauth.Elapsed(time.Since(sessionStart))

	keyStart := time.Now()
	if _, err := c.Keys.Verify(ctx, key.FullKey); err != nil {
		return demos.AuthCompareRun{}, fmt.Errorf("compare story key verify: %w", err)
	}
	keyMs := domauth.Elapsed(time.Since(keyStart))

	tokenStart := time.Now()
	verdict := c.Tokens.Verify(token.Token, clock())
	tokenMs := domauth.Elapsed(time.Since(tokenStart))
	if verdict.Outcome != domauth.TokenValid {
		return demos.AuthCompareRun{}, fmt.Errorf("compare story token verify: %s", verdict.Outcome)
	}

	// Revoke the two revocable mechanisms, then re-verify all three.
	revokedSession, _, err := c.Sessions.Logout(ctx, login.Cookie.Value)
	if err != nil {
		return demos.AuthCompareRun{}, fmt.Errorf("compare story session revoke: %w", err)
	}
	revokedKey, err := c.Keys.Revoke(ctx, key.Prefix)
	if err != nil {
		return demos.AuthCompareRun{}, fmt.Errorf("compare story key revoke: %w", err)
	}
	_, sessionAfter := c.Sessions.Inspect(ctx, login.Cookie.Value, clock())
	_, keyAfter := c.Keys.Verify(ctx, key.FullKey)
	tokenAfter := c.Tokens.Verify(token.Token, clock()).Outcome

	sessionAfterText := "the cookie resolves to nothing — dead"
	if sessionAfter == nil {
		sessionAfterText = "STILL VALID — an engine defect, reported as it happened"
	}
	keyAfterText := "the key hashes to nothing — dead"
	if keyAfter == nil {
		keyAfterText = "STILL VALID — an engine defect, reported as it happened"
	}
	tokenAfterText := fmt.Sprintf("STILL VALID (%s) — and it will be until exp: a JWT has no kill switch, only a lease", tokenAfter)

	return demos.AuthCompareRun{
		Subject: "ada",
		Mechanisms: []demos.AuthCompareMechanismRow{
			{
				Mechanism:       "session cookie",
				Establish:       "one login (password → argon2 verify) mints a random id stored server-side",
				Carries:         "an opaque id sealed with an HMAC signature in an HttpOnly cookie",
				VerifyElapsedMs: sessionMs,
				Consults:        "the session store (one lookup after unsealing)",
				Revocation:      fmt.Sprintf("logout deletes the row (revoked=%t)", revokedSession),
				AfterRevoke:     sessionAfterText,
			},
			{
				Mechanism:       "api key",
				Establish:       "one generation mints prefix + 256-bit secret, storing only the SHA-256",
				Carries:         "the raw key in the x-api-key header",
				VerifyElapsedMs: keyMs,
				Consults:        "one SHA-256 + one prefix lookup + one constant-time compare",
				Revocation:      fmt.Sprintf("a prefix delete (revoked=%t)", revokedKey),
				AfterRevoke:     keyAfterText,
			},
			{
				Mechanism:       "jwt (HS256)",
				Establish:       "one mint signs the claims with the server's secret — no store row exists",
				Carries:         "the token itself (header.payload.signature) in the Authorization header",
				VerifyElapsedMs: tokenMs,
				Consults:        "nothing but the signature and the claims — stateless",
				Revocation:      "none — expiry is the only lever (that is what refresh rotation fixes)",
				AfterRevoke:     tokenAfterText,
			},
		},
	}, nil
}
