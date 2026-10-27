package presentation

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	domauth "guide/internal/domain/auth"
	domauthz "guide/internal/domain/authorization"
	"guide/internal/domain/demos"
)

// authzTestLogger discards log output — these tests assert status codes
// and envelope shapes, not log lines.
func authzTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// The topic 24 endpoints and middleware, exercised through the same
// composition production runs (RequireIdentity ∘ RequirePermission /
// RequireOwnership). The role directory and ownership port are faked
// here — the REAL console story has its own suite in
// internal/data/authorization — and the identity is injected the way
// RequireIdentity does it (WithIdentity), so the 401-vs-403 mapping is
// pinned without any store.

// ── fakes ───────────────────────────────────────────────────────────────

type fakeAuthzDirectory struct {
	err error
}

func (f fakeAuthzDirectory) RolesFor(_ context.Context, username string) ([]domauthz.Role, error) {
	if f.err != nil {
		return nil, f.err
	}
	switch username {
	case "ada":
		return []domauthz.Role{domauthz.RoleAuthor}, nil
	case "grace":
		return []domauthz.Role{domauthz.RoleEditor}, nil
	case "linus":
		return []domauthz.Role{domauthz.RoleReader}, nil
	default:
		return nil, nil // unassigned: deny by default
	}
}

type fakeAuthzOwnership struct {
	err error
}

func (f fakeAuthzOwnership) PostOwner(_ context.Context, id string) (string, bool, error) {
	if f.err != nil {
		return "", false, f.err
	}
	switch id {
	case "p-ada":
		return "ada", true, nil
	case "p-grace":
		return "grace", true, nil
	case "p-kim":
		return "kim", true, nil
	default:
		return "", false, nil
	}
}

type fakeAuthzMatrixRunner struct {
	run demos.AuthzMatrixRun
	err error
}

func (f fakeAuthzMatrixRunner) RunAuthzMatrixStory(context.Context) (demos.AuthzMatrixRun, error) {
	return f.run, f.err
}

// authzIdentity builds one of the canned identities the tests inject.
func authzIdentity(username string) domauth.Identity {
	switch username {
	case "ada":
		return domauth.Identity{Username: "ada", DisplayName: "Ada", Scopes: []string{"posts:read", "posts:write"}, Via: "session"}
	case "grace":
		return domauth.Identity{Username: "grace", DisplayName: "Grace", Scopes: []string{"posts:read", "posts:write"}, Via: "jwt"}
	case "linus":
		return domauth.Identity{Username: "linus", DisplayName: "Linus", Scopes: []string{"posts:read"}, Via: "session"}
	case "kim":
		return domauth.Identity{Username: "kim", DisplayName: "Kim", Scopes: []string{"posts:read"}, Via: "api-key"}
	default:
		return domauth.Identity{Username: username, Scopes: []string{"*"}, Via: "session"}
	}
}

// authzServe runs one request through a gate composition and answers
// recorder + response body.
func authzServe(t *testing.T, gate Middleware, identity *domauth.Identity, method, target string) (*httptest.ResponseRecorder, string) {
	t.Helper()
	reached := false
	var handler http.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reached = true
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{"reached": true})
	})
	if gate != nil {
		handler = gate(handler)
	}
	req := httptest.NewRequest(method, target, nil)
	if id := strings.TrimPrefix(target, "/api/demo/authz/guarded/post/"); id != target {
		req.SetPathValue("id", id) // the route pattern's {id}, as ServeMux would set it
	}
	if identity != nil {
		req = req.WithContext(WithIdentity(req.Context(), *identity))
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if reached && rec.Code != http.StatusOK {
		t.Fatalf("inner handler reached but status = %d", rec.Code)
	}
	return rec, rec.Body.String()
}

// ── the matrix endpoint ─────────────────────────────────────────────────

func TestAuthzMatrixHandlerServesTheStory(t *testing.T) {
	run := demos.AuthzMatrixRun{
		Roles:    []demos.AuthzRoleGrantRow{{Role: "reader", Permissions: []string{"posts:read"}}},
		Subjects: []demos.AuthzSubjectRow{{Username: "ada", Roles: []string{"author"}, Scopes: []string{"posts:read"}}},
		Posts:    []demos.AuthzPostRow{{ID: "p", Title: "T", Owner: "ada"}},
		Cells:    []demos.AuthzCellRow{{Subject: "ada", Action: "posts:write", PostID: "p", Allowed: true, Reason: "owner"}},
	}
	handler := NewAuthzMatrixHandler(fakeAuthzMatrixRunner{run: run}, authzTestLogger())
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/demo/authz/matrix", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", rec.Code, rec.Body.String())
	}
	var payload demos.AuthStoryPayload
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("payload: %v", err)
	}
	if payload.Mechanism != "rbac-matrix" || len(payload.Beats) == 0 || payload.Proofs == nil {
		t.Errorf("payload shape drifted: %+v", payload)
	}
}

func TestAuthzMatrixHandlerMapsRunnerFailureToInternal(t *testing.T) {
	handler := NewAuthzMatrixHandler(fakeAuthzMatrixRunner{err: errors.New("boom")}, authzTestLogger())
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/demo/authz/matrix", nil))
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"code":"internal"`) {
		t.Errorf("body lacks internal envelope: %s", rec.Body.String())
	}
}

// ── RequirePermission: the 401/403 mapping ─────────────────────────────

func TestAuthzRequirePermissionNeverAnswers401ForAuthenticatedCallers(t *testing.T) {
	directory := fakeAuthzDirectory{}
	gate := RequirePermission(directory, domauthz.PermPostsWrite, authzTestLogger())

	cases := []struct {
		name     string
		identity *domauth.Identity
		want     int
	}{
		{"no identity in context (gate run bare)", nil, http.StatusUnauthorized},
		{"ada author full scopes passes", &[]domauth.Identity{authzIdentity("ada")}[0], http.StatusOK},
		{"linus reader denied → 403", &[]domauth.Identity{authzIdentity("linus")}[0], http.StatusForbidden},
		{"kim author narrowed scopes denied → 403", &[]domauth.Identity{authzIdentity("kim")}[0], http.StatusForbidden},
		{"guest unassigned denied → 403", &[]domauth.Identity{authzIdentity("guest")}[0], http.StatusForbidden},
	}
	for _, tc := range cases {
		rec, _ := authzServe(t, gate, tc.identity, http.MethodGet, "/api/demo/authz/guarded")
		if rec.Code != tc.want {
			t.Errorf("%s: status = %d, want %d (body %s)", tc.name, rec.Code, tc.want, rec.Body.String())
		}
		if tc.want == http.StatusForbidden {
			var envelope struct {
				Error struct {
					Code    string `json:"code"`
					Message string `json:"message"`
				} `json:"error"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
				t.Fatalf("%s: envelope: %v", tc.name, err)
			}
			if envelope.Error.Code != "forbidden" {
				t.Errorf("%s: code = %q, want forbidden", tc.name, envelope.Error.Code)
			}
		}
	}
}

// The 403 body carries the evaluator's reason — authorization refusals
// teach; contrast with topic 23's deliberately uniform 401.
func TestAuthzRequirePermissionForbiddenNamesItsGate(t *testing.T) {
	gate := RequirePermission(fakeAuthzDirectory{}, domauthz.PermPostsWrite, authzTestLogger())
	// linus holds scopes [posts:read]: the SCOPE gate refuses first.
	rec, _ := authzServe(t, gate, &[]domauth.Identity{authzIdentity("linus")}[0], http.MethodGet, "/x")
	if !strings.Contains(rec.Body.String(), "narrowed") {
		t.Errorf("403 body must name the scope gate: %s", rec.Body.String())
	}
	// guest holds scopes [*] but no role: the ROLE gate refuses.
	rec, _ = authzServe(t, gate, &[]domauth.Identity{authzIdentity("guest")}[0], http.MethodGet, "/x")
	if !strings.Contains(rec.Body.String(), "no role") {
		t.Errorf("403 body must name the role gate: %s", rec.Body.String())
	}
}

func TestAuthzRequirePermissionMapsDirectoryFailureTo500(t *testing.T) {
	gate := RequirePermission(fakeAuthzDirectory{err: errors.New("store down")}, domauthz.PermPostsWrite, authzTestLogger())
	rec, _ := authzServe(t, gate, &[]domauth.Identity{authzIdentity("ada")}[0], http.MethodGet, "/x")
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
}

// The gate records its decision where the guarded handler can echo it.
func TestAuthzRequirePermissionRecordsItsDecision(t *testing.T) {
	var recorded domauthz.Decision
	var present bool
	handler := RequirePermission(fakeAuthzDirectory{}, domauthz.PermPostsWrite, authzTestLogger())(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			recorded, present = DecisionFromContext(r.Context())
			w.WriteHeader(http.StatusOK)
		}))
	req := httptest.NewRequest(http.MethodGet, "/x", nil).
		WithContext(WithIdentity(context.Background(), authzIdentity("ada")))
	handler.ServeHTTP(httptest.NewRecorder(), req)
	if !present || !recorded.Allowed {
		t.Fatalf("decision = %+v present=%t, want recorded allow", recorded, present)
	}
}

// ── RequireOwnership: the object-level gate ─────────────────────────────

func TestAuthzRequireOwnershipDecidesOwnersEditorsStrangers(t *testing.T) {
	directory := fakeAuthzDirectory{}
	ownership := fakeAuthzOwnership{}
	gate := RequireOwnership(directory, ownership, domauthz.PermPostsWrite, authzTestLogger())

	cases := []struct {
		name     string
		username string
		postID   string
		want     int
	}{
		{"owner passes", "ada", "p-ada", http.StatusOK},
		{"editor passes on any post", "grace", "p-ada", http.StatusOK},
		{"stranger refused", "ada", "p-grace", http.StatusForbidden},
		{"narrowed author refused even on own post", "kim", "p-kim", http.StatusForbidden},
		{"unassigned refused", "guest", "p-ada", http.StatusForbidden},
		{"unknown post answers 404 first", "ada", "p-missing", http.StatusNotFound},
	}
	for _, tc := range cases {
		identity := authzIdentity(tc.username)
		if tc.postID == "p-kim" {
			// kim owns p-kim in this fixture's world; the fake answers
			// it as kim's by id so the narrowed-scope beat is honest.
			identity.Scopes = []string{"posts:read"}
		}
		rec, _ := authzServe(t, gate, &identity, http.MethodGet, "/api/demo/authz/guarded/post/"+tc.postID)
		if rec.Code != tc.want {
			t.Errorf("%s: status = %d, want %d (body %s)", tc.name, rec.Code, tc.want, rec.Body.String())
		}
		if tc.want == http.StatusNotFound && !strings.Contains(rec.Body.String(), `"code":"not_found"`) {
			t.Errorf("%s: body %s", tc.name, rec.Body.String())
		}
	}
}

func TestAuthzRequireOwnershipMapsLookupFailureTo500(t *testing.T) {
	gate := RequireOwnership(fakeAuthzDirectory{}, fakeAuthzOwnership{err: errors.New("store down")},
		domauthz.PermPostsWrite, authzTestLogger())
	identity := authzIdentity("ada")
	rec, _ := authzServe(t, gate, &identity, http.MethodGet, "/api/demo/authz/guarded/post/p-ada")
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
}

// ── the guarded proof handlers ──────────────────────────────────────────

func TestAuthzGuardedHandlerEchoesIdentityAndDecision(t *testing.T) {
	handler := NewAuthzGuardedHandler(domauthz.PermPostsWrite, authzTestLogger())
	req := httptest.NewRequest(http.MethodGet, "/api/demo/authz/guarded", nil).
		WithContext(WithIdentity(WithDecision(context.Background(),
			domauthz.Decision{Allowed: true, Reason: "granted"}), authzIdentity("ada")))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var proof AuthzGuardedProof
	if err := json.Unmarshal(rec.Body.Bytes(), &proof); err != nil {
		t.Fatalf("proof: %v", err)
	}
	if proof.Username != "ada" || proof.Permission != "posts:write" || proof.Reason != "granted" {
		t.Errorf("proof = %+v", proof)
	}
	if proof.Scopes == nil {
		t.Error("scopes must render [] never null")
	}
}

func TestAuthzGuardedPostHandlerEchoesObjectAndRule(t *testing.T) {
	handler := NewAuthzGuardedPostHandler(domauthz.PermPostsWrite, fakeAuthzOwnership{}, authzTestLogger())
	req := httptest.NewRequest(http.MethodGet, "/api/demo/authz/guarded/post/p-ada", nil)
	req.SetPathValue("id", "p-ada")
	req = req.WithContext(WithIdentity(WithDecision(context.Background(),
		domauthz.Decision{Allowed: true, Reason: "object-level pass"}), authzIdentity("ada")))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var proof AuthzGuardedPostProof
	if err := json.Unmarshal(rec.Body.Bytes(), &proof); err != nil {
		t.Fatalf("proof: %v", err)
	}
	if proof.Username != "ada" || proof.PostID != "p-ada" || proof.Owner != "ada" ||
		proof.Action != "posts:write" || proof.Reason != "object-level pass" {
		t.Errorf("proof = %+v", proof)
	}
}

// ── the full composition: RequireIdentity ∘ RequirePermission ───────────

// fakeAuthzBearerResolver answers one identity for any bearer token.
type fakeAuthzBearerResolver struct {
	identity domauth.Identity
	err      error
}

func (f fakeAuthzBearerResolver) ResolveSessionCookie(context.Context, string, time.Time) (domauth.Identity, error) {
	return domauth.Identity{}, domauth.ErrNotAuthenticated
}

func (f fakeAuthzBearerResolver) ResolveAPIKey(context.Context, string, time.Time) (domauth.Identity, error) {
	return domauth.Identity{}, domauth.ErrNotAuthenticated
}

func (f fakeAuthzBearerResolver) ResolveBearerToken(_ context.Context, _ string, _ time.Time) (domauth.Identity, error) {
	return f.identity, f.err
}

// The composition a protected route actually wires: no credential →
// 401 (the auth middleware's uniform envelope, never 403); a verified
// credential the policy refuses → 403 (never 401); a credential the
// policy passes → the handler runs.
func TestAuthzCompositionMaps401Then403Correctly(t *testing.T) {
	resolver := fakeAuthzBearerResolver{identity: authzIdentity("linus")}
	gate := RequireIdentity(resolver, authzTestLogger())(
		RequirePermission(fakeAuthzDirectory{}, domauthz.PermPostsWrite, authzTestLogger())(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"reached":true}`))
			})))

	// No credential: 401 from the auth middleware, uniform envelope.
	req := httptest.NewRequest(http.MethodGet, "/api/demo/authz/guarded", nil)
	rec := httptest.NewRecorder()
	gate.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("no credential: status = %d, want 401", rec.Code)
	}

	// Verified credential (linus, reader): 403 from the authz gate.
	req = httptest.NewRequest(http.MethodGet, "/api/demo/authz/guarded", nil)
	req.Header.Set("Authorization", "Bearer a-token")
	rec = httptest.NewRecorder()
	gate.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("reader with token: status = %d, want 403", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"code":"forbidden"`) {
		t.Errorf("body = %s", rec.Body.String())
	}

	// A credential the policy passes: the handler runs.
	resolver = fakeAuthzBearerResolver{identity: authzIdentity("ada")}
	gate = RequireIdentity(resolver, authzTestLogger())(
		RequirePermission(fakeAuthzDirectory{}, domauthz.PermPostsWrite, authzTestLogger())(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				identity, ok := IdentityFromContext(r.Context())
				if !ok || identity.Username != "ada" {
					t.Error("identity must reach the handler")
				}
				w.WriteHeader(http.StatusOK)
			})))
	req = httptest.NewRequest(http.MethodGet, "/api/demo/authz/guarded", nil)
	req.Header.Set("Authorization", "Bearer a-token")
	rec = httptest.NewRecorder()
	gate.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("author with token: status = %d, want 200 (body %s)", rec.Code, rec.Body.String())
	}
}
