package presentation

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"guide/internal/core"
	domauth "guide/internal/domain/auth"
	"guide/internal/domain/demos"
)

// The topic 23 endpoints, exercised through the same middleware chain
// production runs behind (the family rule that every new demo endpoint
// ships with a Go test). The seven story runners are faked here — the
// REAL console stories have their own suite in internal/data/auth —
// and the auth middleware is tested against a fake resolver that
// counts which carrier it saw, so the precedence order and the 401
// envelope shape are pinned without any store.

// ── fakes ───────────────────────────────────────────────────────────────

type fakeAuthStoryRunners struct {
	sessionsErr error
	keysErr     error
	compareErr  error
	jwtErr      error
	oauthErr    error
	hashingErr  error
	refreshErr  error
}

func (f fakeAuthStoryRunners) RunAuthSessionsStory(context.Context) (demos.AuthSessionsRun, error) {
	return demos.AuthSessionsRun{
		Username: "ada", Cookie: demos.AuthCookieSnapshot{Name: "c", ValuePreview: "v", SameSite: "lax", HTTPOnly: true},
		CookieAnatomy: "anatomy", InspectedUser: "ada", InspectedScopes: []string{"posts:read"},
		TamperedRefused: true, WrongPasswordMs: "40.0 ms", UnknownUserMs: "40.0 ms",
		UniformRefusals: true, Revoked: true, PostLogoutRefused: true,
	}, f.sessionsErr
}

func (f fakeAuthStoryRunners) RunAuthAPIKeysStory(context.Context) (demos.AuthAPIKeysRun, error) {
	return demos.AuthAPIKeysRun{
		Prefix: "gdg_0123abcd", Name: "ci", Scopes: []string{"posts:read"}, FullKey: "gdg_0123abcd.x",
		SHA256Preview: "abc", RosterCount: 1, RosterPreviews: []string{"sha256:abc"},
		VerifiedPrefix: "gdg_0123abcd", VerifiedName: "ci",
		WrongSecretRefused: true, MalformedRefused: true, Revoked: true, PostRevokeRefused: true,
	}, f.keysErr
}

func (f fakeAuthStoryRunners) RunAuthCompareStory(context.Context) (demos.AuthCompareRun, error) {
	return demos.AuthCompareRun{Subject: "ada", Mechanisms: []demos.AuthCompareMechanismRow{
		{Mechanism: "session cookie", VerifyElapsedMs: "1.0 ms", Consults: "store", AfterRevoke: "dead"},
		{Mechanism: "api key", VerifyElapsedMs: "1.0 ms", Consults: "sha256", AfterRevoke: "dead"},
		{Mechanism: "jwt (HS256)", VerifyElapsedMs: "1.0 ms", Consults: "nothing", AfterRevoke: "STILL VALID (valid) — until exp"},
	}}, f.compareErr
}

func (f fakeAuthStoryRunners) RunAuthJWTStory(context.Context) (demos.AuthJWTRun, error) {
	return demos.AuthJWTRun{
		Subject: "ada", TokenPreview: "p", HeaderJSON: `{"alg":"HS256"}`, PayloadJSON: `{"sub":"ada"}`,
		SignatureChars: 43, ExpiresAt: "soon", Verdicts: []demos.AuthJWTVerdictRow{
			{Case: "honest verify", Outcome: "valid", Reason: "ok"},
			{Case: "alg=none attack", Outcome: "alg-none-rejected", Reason: "refused"},
		},
	}, f.jwtErr
}

func (f fakeAuthStoryRunners) RunAuthOAuthStory(context.Context) (demos.AuthOAuthRun, error) {
	return demos.AuthOAuthRun{
		VerifierPreview: "v", ChallengePreview: "c", State: "s", RedirectTo: "https://x?code=a&state=s",
		CodePreview: "a", AccessTokenPreview: "t", Scope: "posts:read", Username: "ada",
		AccessTokenOutcome: "valid", ReplayRefused: "invalid_grant …",
		WrongVerifierRefused: "invalid_grant …", BurnedCodeProof: "invalid_grant …",
	}, f.oauthErr
}

func (f fakeAuthStoryRunners) RunAuthHashingStory(context.Context) (demos.AuthHashingRun, error) {
	return demos.AuthHashingRun{
		Parameters: "argon2id", FirstHashPreview: "a", SecondHashPreview: "b", Distinct: true,
		CorrectMs: "40.0 ms", WrongMs: "40.0 ms", LegacyNeedsRehash: true, LegacyMs: "10.0 ms",
		UnknownUserMs: "40.0 ms", MalformedRefused: "loud error",
	}, f.hashingErr
}

func (f fakeAuthStoryRunners) RunAuthRefreshStory(context.Context) (demos.AuthRefreshRun, error) {
	return demos.AuthRefreshRun{
		FamilyID: "fam_x", Username: "ada", LoginTokenPreview: "rt_x",
		Rotations:     []demos.AuthRefreshRotationRow{{From: 1, To: 2}, {From: 2, To: 3}},
		ReusePosition: 2, TokensRevoked: 3, PostReuseRefused: "refused",
	}, f.refreshErr
}

// fakeCredentialResolver records which carrier was asked and answers
// per its fields.
type fakeCredentialResolver struct {
	via        string
	identity   domauth.Identity
	err        error
	sawSession bool
	sawKey     bool
	sawBearer  bool
}

func (f *fakeCredentialResolver) ResolveSessionCookie(context.Context, string, time.Time) (domauth.Identity, error) {
	f.sawSession = true
	return f.identity, f.err
}

func (f *fakeCredentialResolver) ResolveAPIKey(context.Context, string, time.Time) (domauth.Identity, error) {
	f.sawKey = true
	return f.identity, f.err
}

func (f *fakeCredentialResolver) ResolveBearerToken(context.Context, string, time.Time) (domauth.Identity, error) {
	f.sawBearer = true
	return f.identity, f.err
}

// ── the seven story endpoints ───────────────────────────────────────────

func newAuthDemoServer(t *testing.T, runners fakeAuthStoryRunners, resolver *fakeCredentialResolver) http.Handler {
	t.Helper()
	logger := newTestLogger(&strings.Builder{})
	mux := http.NewServeMux()
	mux.Handle("GET /api/demo/auth/sessions", NewAuthSessionsHandler(runners, logger))
	mux.Handle("GET /api/demo/auth/api-keys", NewAuthAPIKeysHandler(runners, logger))
	mux.Handle("GET /api/demo/auth/compare", NewAuthCompareHandler(runners, logger))
	mux.Handle("GET /api/demo/auth/jwt", NewAuthJWTHandler(runners, logger))
	mux.Handle("GET /api/demo/auth/oauth", NewAuthOAuthHandler(runners, logger))
	mux.Handle("GET /api/demo/auth/hashing", NewAuthHashingHandler(runners, logger))
	mux.Handle("GET /api/demo/auth/refresh", NewAuthRefreshHandler(runners, logger))
	mux.Handle("GET /api/demo/auth/whoami",
		RequireIdentity(resolver, logger)(NewAuthWhoamiHandler(logger)))
	return Chain(RequestID, RequestLogger(logger), Recover(logger), EnvelopeMuxErrors(logger))(mux)
}

func TestAuthStoryEndpointsServeShapedPayloads(t *testing.T) {
	server := newAuthDemoServer(t, fakeAuthStoryRunners{}, &fakeCredentialResolver{
		identity: domauth.Identity{Username: "ada", Via: "jwt"},
	})
	for _, tc := range []struct {
		path      string
		mechanism string
		beats     int
	}{
		{"/api/demo/auth/sessions", "sessions", 6},
		{"/api/demo/auth/api-keys", "api-keys", 7},
		{"/api/demo/auth/compare", "compare", 3},
		{"/api/demo/auth/jwt", "jwt", 3},
		{"/api/demo/auth/oauth", "oauth", 6},
		{"/api/demo/auth/hashing", "hashing", 6},
		{"/api/demo/auth/refresh", "refresh", 4},
	} {
		rec := doRequest(server, http.MethodGet, tc.path, "")
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: status = %d: %s", tc.path, rec.Code, rec.Body.String())
		}
		decoded := struct {
			Mechanism string `json:"mechanism"`
			Beats     []struct {
				Beat    string `json:"beat"`
				Verdict string `json:"verdict"`
				Facts   []any  `json:"facts"`
			} `json:"beats"`
			Proofs    []any  `json:"proofs"`
			Reasoning string `json:"reasoning"`
		}{}
		if err := json.Unmarshal(rec.Body.Bytes(), &decoded); err != nil {
			t.Fatalf("%s: body not the story DTO shape: %v", tc.path, err)
		}
		if decoded.Mechanism != tc.mechanism {
			t.Errorf("%s: mechanism = %q", tc.path, decoded.Mechanism)
		}
		if len(decoded.Beats) != tc.beats {
			t.Errorf("%s: beats = %d, want %d", tc.path, len(decoded.Beats), tc.beats)
		}
		if decoded.Beats[0].Facts == nil {
			t.Errorf("%s: first beat facts marshaled null", tc.path)
		}
		if len(decoded.Proofs) == 0 || decoded.Reasoning == "" {
			t.Errorf("%s: payload lacks proofs/reasoning", tc.path)
		}
	}
}

func TestAuthStoryEndpointMapsRunnerFailureToInternal(t *testing.T) {
	server := newAuthDemoServer(t, fakeAuthStoryRunners{sessionsErr: errors.New("boom")}, &fakeCredentialResolver{})
	rec := doRequest(server, http.MethodGet, "/api/demo/auth/sessions", "")
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	var envelope struct {
		Error struct {
			Code      string `json:"code"`
			RequestID string `json:"requestId"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("envelope: %v", err)
	}
	if envelope.Error.Code != string(core.KindInternal) || envelope.Error.RequestID == "" {
		t.Errorf("envelope = %+v", envelope.Error)
	}
}

// ── the auth middleware + whoami ────────────────────────────────────────

func TestAuthMiddlewareResolvesAllThreeCarriers(t *testing.T) {
	for _, tc := range []struct {
		name    string
		headers map[string]string
		cookie  string
		via     string
	}{
		{"bearer token", map[string]string{"Authorization": "Bearer a.token.here"}, "", "jwt"},
		{"api key", map[string]string{"x-api-key": "gdg_x.secret"}, "", "api-key"},
		{"session cookie", nil, "sealed-value.sig", "session"},
	} {
		resolver := &fakeCredentialResolver{identity: domauth.Identity{Username: "ada", DisplayName: "Ada", Via: tc.via}}
		server := newAuthDemoServer(t, fakeAuthStoryRunners{}, resolver)
		rec := httptestRecorder(server, http.MethodGet, "/api/demo/auth/whoami", tc.headers, tc.cookie)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: status = %d: %s", tc.name, rec.Code, rec.Body.String())
		}
		decoded := struct {
			Username    string   `json:"username"`
			DisplayName string   `json:"displayName"`
			Scopes      []string `json:"scopes"`
			Via         string   `json:"via"`
		}{}
		if err := json.Unmarshal(rec.Body.Bytes(), &decoded); err != nil {
			t.Fatalf("%s: body: %v", tc.name, err)
		}
		if decoded.Username != "ada" || decoded.Via != tc.via || decoded.Scopes == nil {
			t.Errorf("%s: identity = %+v", tc.name, decoded)
		}
	}
}

func TestAuthMiddlewareRefusesWithoutCredentialsWithTheEnvelope(t *testing.T) {
	resolver := &fakeCredentialResolver{}
	server := newAuthDemoServer(t, fakeAuthStoryRunners{}, resolver)
	rec := doRequest(server, http.MethodGet, "/api/demo/auth/whoami", "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
	var envelope struct {
		Error struct {
			Code      string `json:"code"`
			Message   string `json:"message"`
			RequestID string `json:"requestId"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("envelope: %v", err)
	}
	if envelope.Error.Code != string(core.KindUnauthorized) {
		t.Errorf("code = %q, want unauthorized", envelope.Error.Code)
	}
	if envelope.Error.RequestID == "" {
		t.Error("401 must carry the request id")
	}
	if resolver.sawBearer || resolver.sawKey || resolver.sawSession {
		t.Error("no resolver should have been consulted with no credentials")
	}
}

func TestAuthMiddlewareUniformFortyOneForBrokenCredentials(t *testing.T) {
	for _, tc := range []struct {
		name    string
		headers map[string]string
	}{
		{"malformed Authorization header", map[string]string{"Authorization": "Basic dXNlcjpwYXNz"}},
		{"empty bearer", map[string]string{"Authorization": "Bearer "}},
		{"bearer that fails to verify", map[string]string{"Authorization": "Bearer forged"}},
		{"api key that fails to verify", map[string]string{"x-api-key": "gdg_x.wrong"}},
	} {
		resolver := &fakeCredentialResolver{err: domauth.ErrNotAuthenticated}
		server := newAuthDemoServer(t, fakeAuthStoryRunners{}, resolver)
		rec := httptestRecorder(server, http.MethodGet, "/api/demo/auth/whoami", tc.headers, "")
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("%s: status = %d", tc.name, rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "not authenticated") {
			t.Errorf("%s: envelope message = %s", tc.name, rec.Body.String())
		}
	}
}

func TestAuthMiddlewareBearerWinsOverCookie(t *testing.T) {
	resolver := &fakeCredentialResolver{identity: domauth.Identity{Username: "ada", Via: "jwt"}}
	server := newAuthDemoServer(t, fakeAuthStoryRunners{}, resolver)
	headers := map[string]string{"Authorization": "Bearer a.token"}
	rec := httptestRecorder(server, http.MethodGet, "/api/demo/auth/whoami", headers, "leftover-cookie-value")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if !resolver.sawBearer || resolver.sawSession {
		t.Errorf("precedence broken: bearer=%t session=%t (want true/false)", resolver.sawBearer, resolver.sawSession)
	}
}

// httptestRecorder is doRequest with headers and a cookie — the shape
// the auth middleware's cases need.
func httptestRecorder(server http.Handler, method, path string, headers map[string]string, cookieValue string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, nil)
	for name, value := range headers {
		req.Header.Set(name, value)
	}
	if cookieValue != "" {
		req.AddCookie(&http.Cookie{Name: domauth.SessionCookieName, Value: cookieValue})
	}
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	return rec
}
