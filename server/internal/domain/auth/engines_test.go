package auth

import (
	"errors"
	"strings"
	"testing"
	"time"
)

// ── Password engine ─────────────────────────────────────────────────────

func TestPasswordHashRoundTripAndFormat(t *testing.T) {
	hasher := NewHasher()
	encoded, err := hasher.Hash("correct-horse")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	if !strings.HasPrefix(encoded, "$argon2id$v=19$m=19456,t=2,p=1$") {
		t.Errorf("hash %q lacks the stamped parameter prefix", encoded)
	}
	if err := hasher.Verify("correct-horse", encoded); err != nil {
		t.Errorf("verify correct password: %v", err)
	}
	if err := hasher.Verify("wrong-horse", encoded); !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("verify wrong password = %v, want ErrInvalidCredentials", err)
	}
	if hasher.NeedsRehash(encoded) {
		t.Error("a fresh hash must not need rehash")
	}
}

func TestPasswordSaltingMakesDistinctHashes(t *testing.T) {
	hasher := NewHasher()
	first, _ := hasher.Hash("same-password")
	second, _ := hasher.Hash("same-password")
	if first == second {
		t.Error("the same password hashed twice must produce different strings (fresh salt)")
	}
	if err := hasher.Verify("same-password", first); err != nil {
		t.Errorf("first hash verifies: %v", err)
	}
	if err := hasher.Verify("same-password", second); err != nil {
		t.Errorf("second hash verifies: %v", err)
	}
}

func TestPasswordMalformedHashIsAnErrorNotAMatch(t *testing.T) {
	hasher := NewHasher()
	err := hasher.Verify("x", "not-a-phc-string")
	if err == nil || errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("malformed stored hash must be a loud error, got %v", err)
	}
	if !strings.Contains(err.Error(), "decode stored hash") {
		t.Errorf("error should name the decode step, got %v", err)
	}
}

func TestPasswordLegacyParamsTriggerRehashSignal(t *testing.T) {
	current := NewHasher()
	legacy := &Hasher{Params: LegacyHashParams}
	legacyHash := legacy.HashWithSalt("correct-horse", []byte("legacy-salt-----"))
	if err := current.Verify("correct-horse", legacyHash); err != nil {
		t.Fatalf("legacy-cost hash must still verify: %v", err)
	}
	if !current.NeedsRehash(legacyHash) {
		t.Error("legacy-cost hash must fire the rehash signal")
	}
}

func TestPasswordVerifyUniformBurnsDecoyForUnknownUsers(t *testing.T) {
	hasher := NewHasher()
	// The burn must be REAL WORK: an argon2 run at default cost takes
	// single-digit milliseconds at minimum, so a lower bound catches the
	// short-circuit regression (an `!ok || verify` that skips the decoy
	// and answers in microseconds).
	started := time.Now()
	if hasher.VerifyUniform("whatever", "") {
		t.Error("unknown user (empty hash) must fail uniform verify")
	}
	if elapsed := time.Since(started); elapsed < time.Millisecond {
		t.Errorf("decoy burn took %v — the uniform check must do real argon2 work", elapsed)
	}
	if !hasher.VerifyUniform("correct-horse", hasher.HashWithSalt("correct-horse", []byte("s"))) {
		t.Error("correct credential must pass uniform verify")
	}
	if hasher.VerifyUniform("wrong", hasher.HashWithSalt("correct-horse", []byte("s"))) {
		t.Error("wrong password must fail uniform verify")
	}
}

// ── Session engine ──────────────────────────────────────────────────────

func TestSessionLoginIssuesSealedCookieAndInspectResolves(t *testing.T) {
	_, sessions, _, _, _, _, _ := newTestEngines()
	login, err := sessions.Login(t.Context(), "ada", "correct-horse", testNow)
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	if login.Identity.Username != "ada" || login.Identity.Via != "session" {
		t.Errorf("identity = %+v", login.Identity)
	}
	cookie := login.Cookie
	if cookie.Name != SessionCookieName || !cookie.HTTPOnly || cookie.SameSite != "lax" || cookie.Secure {
		t.Errorf("cookie flags wrong: %+v", cookie)
	}
	if cookie.MaxAgeSeconds != int(SessionTTL.Seconds()) {
		t.Errorf("cookie Max-Age = %d, want %d", cookie.MaxAgeSeconds, int(SessionTTL.Seconds()))
	}
	row, err := sessions.Inspect(t.Context(), cookie.Value, testNow.Add(time.Minute))
	if err != nil {
		t.Fatalf("inspect: %v", err)
	}
	if row.Username != "ada" {
		t.Errorf("inspected row = %+v", row)
	}
}

func TestSessionLoginFailuresAreUniform(t *testing.T) {
	_, sessions, _, _, _, _, _ := newTestEngines()
	for _, tc := range []struct {
		name     string
		username string
		password string
	}{
		{"unknown user", "ghost", "correct-horse"},
		{"wrong password", "ada", "wrong-horse"},
	} {
		_, err := sessions.Login(t.Context(), tc.username, tc.password, testNow)
		if !errors.Is(err, ErrInvalidCredentials) {
			t.Errorf("%s: err = %v, want ErrInvalidCredentials", tc.name, err)
		}
	}
}

func TestSessionTamperedAndForgedAndExpiredValuesAllRefused(t *testing.T) {
	_, sessions, _, _, _, _, _ := newTestEngines()
	login, err := sessions.Login(t.Context(), "ada", "correct-horse", testNow)
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	tampered := flipSealedChar(login.Cookie.Value)
	if _, err := sessions.Inspect(t.Context(), tampered, testNow); !errors.Is(err, ErrNotAuthenticated) {
		t.Errorf("tampered cookie = %v, want ErrNotAuthenticated", err)
	}
	forged := "attacker-minted-id.AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	if _, err := sessions.Inspect(t.Context(), forged, testNow); !errors.Is(err, ErrNotAuthenticated) {
		t.Errorf("forged cookie = %v, want ErrNotAuthenticated", err)
	}
	// Expired row: the same sealed value, past the TTL.
	if _, err := sessions.Inspect(t.Context(), login.Cookie.Value, testNow.Add(SessionTTL+time.Second)); !errors.Is(err, ErrNotAuthenticated) {
		t.Errorf("expired session = %v, want ErrNotAuthenticated", err)
	}
}

func TestSessionLogoutDeletesRowAndIsIdempotent(t *testing.T) {
	_, sessions, _, _, _, _, _ := newTestEngines()
	login, err := sessions.Login(t.Context(), "ada", "correct-horse", testNow)
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	revoked, clear, err := sessions.Logout(t.Context(), login.Cookie.Value)
	if err != nil || !revoked {
		t.Fatalf("logout: revoked=%v err=%v", revoked, err)
	}
	if clear.MaxAgeSeconds != 0 || clear.Value != "" || clear.Name != SessionCookieName {
		t.Errorf("clearing cookie = %+v", clear)
	}
	if _, err := sessions.Inspect(t.Context(), login.Cookie.Value, testNow); !errors.Is(err, ErrNotAuthenticated) {
		t.Errorf("post-logout inspect = %v, want ErrNotAuthenticated (the value is dead)", err)
	}
	revokedAgain, _, err := sessions.Logout(t.Context(), login.Cookie.Value)
	if err != nil || revokedAgain {
		t.Errorf("second logout: revoked=%v err=%v, want false/nil (idempotent)", revokedAgain, err)
	}
}

// flipSealedChar changes a character of a sealed value while keeping it
// in the base64url alphabet — one-character tamper. The SECOND-to-last
// character, never the last: a base64 final character carries ignored
// low-order bits, so a last-character swap can decode to byte-identical
// output (the MAC then still verifies — a 1-in-4 flake), while any
// interior character always changes the decoded bytes.
func flipSealedChar(s string) string {
	runes := []byte(s)
	i := len(runes) - 2
	if runes[i] == 'A' {
		runes[i] = 'B'
	} else {
		runes[i] = 'A'
	}
	return string(runes)
}

// ── API key engine ──────────────────────────────────────────────────────

func TestAPIKeyGenerateVerifyRevokeLifecycle(t *testing.T) {
	ctx := t.Context()
	_, _, keys, _, _, _, _ := newTestEngines()
	created, err := keys.Generate(ctx, "ci key", []string{"posts:read"}, testNow)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if !strings.HasPrefix(created.Prefix, "gdg_") || len(created.Prefix) != len("gdg_")+8 {
		t.Errorf("prefix = %q, want gdg_<8 hex>", created.Prefix)
	}
	record, err := keys.Verify(ctx, created.FullKey)
	if err != nil {
		t.Fatalf("verify fresh key: %v", err)
	}
	if record.Name != "ci key" || record.KeySHA256 != SHA256Hex(created.FullKey) {
		t.Errorf("record = %+v", record)
	}
	// Wrong secret under the right prefix: constant-time reject.
	wrong := created.Prefix + "." + "totally-wrong-secret-value"
	if _, err := keys.Verify(ctx, wrong); !errors.Is(err, ErrNotAuthenticated) {
		t.Errorf("wrong secret = %v, want ErrNotAuthenticated", err)
	}
	// Malformed (no separator) still refuses, after the dummy burn.
	if _, err := keys.Verify(ctx, "no-separator-here"); !errors.Is(err, ErrNotAuthenticated) {
		t.Errorf("malformed key = %v, want ErrNotAuthenticated", err)
	}
	// Revocation is death for every copy.
	revoked, err := keys.Revoke(ctx, created.Prefix)
	if err != nil || !revoked {
		t.Fatalf("revoke: %v %v", revoked, err)
	}
	if _, err := keys.Verify(ctx, created.FullKey); !errors.Is(err, ErrNotAuthenticated) {
		t.Errorf("post-revoke verify = %v, want ErrNotAuthenticated", err)
	}
}

// ── JWT engine ──────────────────────────────────────────────────────────

func TestJWTMintAndVerifyValid(t *testing.T) {
	tokens := NewTokenService([]byte("jwt-secret"))
	minted, err := tokens.Mint(MintTokenRequest{Subject: "ada", Scopes: []string{"posts:read"}, Now: testNow})
	if err != nil {
		t.Fatalf("mint: %v", err)
	}
	if len(strings.Split(minted.Token, ".")) != 3 {
		t.Errorf("token %q is not three parts", minted.Token)
	}
	verdict := tokens.Verify(minted.Token, testNow.Add(time.Minute))
	if verdict.Outcome != TokenValid || verdict.Subject != "ada" {
		t.Errorf("verdict = %+v", verdict)
	}
	identity, err := tokens.Require(minted.Token, testNow.Add(time.Minute))
	if err != nil || identity.Username != "ada" || identity.Via != "jwt" {
		t.Errorf("require = %+v %v", identity, err)
	}
}

func TestJWTClassifiesEveryFailureMode(t *testing.T) {
	tokens := NewTokenService([]byte("jwt-secret"))
	minted, _ := tokens.Mint(MintTokenRequest{Subject: "ada", Scopes: nil, Now: testNow})

	forged, _ := tokens.Mint(MintTokenRequest{Subject: "ada", SignWith: "forged", Now: testNow})
	tampered, _ := tokens.TamperPayload(minted.Token, map[string]any{
		"sub": "admin", "iat": testNow.Unix(), "exp": testNow.Add(time.Hour).Unix(),
		"nbf": testNow.Unix(), "iss": TokenIssuer, "scopes": []string{"posts:write"}, "jti": "x",
	})
	unsigned, _ := StripSignatureForAlgNone(minted.Token)
	expired, _ := tokens.Mint(MintTokenRequest{Subject: "ada", TTL: -time.Minute, Now: testNow})
	future, _ := tokens.Mint(MintTokenRequest{Subject: "ada", NotBefore: time.Hour, Now: testNow})

	cases := []struct {
		name  string
		token string
		want  string
	}{
		{"forged signature", forged.Token, TokenBadSignature},
		{"tampered payload", tampered, TokenBadSignature},
		{"alg=none attack", unsigned, TokenAlgNone},
		{"expired token", expired.Token, TokenExpired},
		{"not-yet-valid token", future.Token, TokenNotYetValid},
		{"garbage", "not-a-jwt", TokenMalformed},
		{"two parts", "two.parts", TokenMalformed},
	}
	for _, tc := range cases {
		verdict := tokens.Verify(tc.token, testNow)
		if verdict.Outcome != tc.want {
			t.Errorf("%s: outcome = %q (%s), want %q", tc.name, verdict.Outcome, verdict.Reason, tc.want)
		}
	}
}

func TestJWTWrongAlgHeaderRefused(t *testing.T) {
	tokens := NewTokenService([]byte("jwt-secret"))
	// A well-formed header claiming HS512, carrying an original payload
	// and signature: refused on the ALG claim, before any signature work.
	minted := mustMint(t, tokens)
	parts := strings.Split(minted.Token, ".")
	hostileHeader, err := encodePart(map[string]string{"alg": "HS512", "typ": "JWT"})
	if err != nil {
		t.Fatalf("encode hostile header: %v", err)
	}
	verdict := tokens.Verify(hostileHeader+"."+parts[1]+"."+parts[2], testNow)
	if verdict.Outcome != TokenWrongAlg {
		t.Errorf("HS512-claiming outcome = %q, want wrong-alg (refused before signature work)", verdict.Outcome)
	}
}

func TestJWTRequireRefusesEverythingButValid(t *testing.T) {
	tokens := NewTokenService([]byte("jwt-secret"))
	if _, err := tokens.Require("garbage", testNow); !errors.Is(err, ErrNotAuthenticated) {
		t.Errorf("require garbage = %v, want ErrNotAuthenticated chain", err)
	}
}

func TestJWTLoginIsUniform(t *testing.T) {
	_, _, _, tokens, _, _, _ := newTestEngines()
	directory := newTestDirectory()
	hasher := NewHasher()
	if _, _, err := tokens.Login(t.Context(), directory, hasher, "ghost", "x", testNow); !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("unknown user login = %v", err)
	}
	if _, _, err := tokens.Login(t.Context(), directory, hasher, "ada", "wrong", testNow); !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("wrong password login = %v", err)
	}
	minted, identity, err := tokens.Login(t.Context(), directory, hasher, "ada", "correct-horse", testNow)
	if err != nil || identity.Username != "ada" {
		t.Fatalf("login ok: %+v %v", identity, err)
	}
	if tokens.Verify(minted.Token, testNow).Outcome != TokenValid {
		t.Error("login token must verify")
	}
}

func mustMint(t *testing.T, tokens *TokenService) MintedToken {
	t.Helper()
	minted, err := tokens.Mint(MintTokenRequest{Subject: "ada", Now: testNow})
	if err != nil {
		t.Fatalf("mint: %v", err)
	}
	return minted
}
