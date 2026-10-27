package authorization

import (
	"strings"
	"testing"

	domauth "guide/internal/domain/auth"
)

func TestAuthorizationScopePermitsGrammar(t *testing.T) {
	cases := []struct {
		name   string
		scopes []string
		perm   Permission
		want   bool
	}{
		{"exact match", []string{"posts:read"}, PermPostsRead, true},
		{"namespace matches every verb", []string{"posts"}, PermPostsWrite, true},
		{"namespace matches the other verb", []string{"posts"}, PermPostsRead, true},
		{"star matches everything", []string{"*"}, PermUsersManage, true},
		{"star matches posts too", []string{"*"}, PermPostsWrite, true},
		{"other verb does not match", []string{"posts:read"}, PermPostsWrite, false},
		{"other namespace does not match", []string{"users:manage"}, PermPostsWrite, false},
		{"empty scopes permit nothing", nil, PermPostsRead, false},
		{"empty-string scope permits nothing", []string{""}, PermPostsRead, false},
		{"malformed separator permits nothing", []string{"posts:"}, PermPostsRead, false},
		{"whitespace is trimmed", []string{" posts:read "}, PermPostsRead, true},
		{"one of many", []string{"users:manage", "posts:write"}, PermPostsWrite, true},
	}
	for _, tc := range cases {
		if got := ScopePermits(tc.scopes, tc.perm); got != tc.want {
			t.Errorf("%s: ScopePermits(%v, %s) = %t, want %t", tc.name, tc.scopes, tc.perm, got, tc.want)
		}
	}
}

func TestAuthorizationAuthorizeDeniesByDefault(t *testing.T) {
	cases := []struct {
		name     string
		identity domauth.Identity
		roles    []Role
		perm     Permission
		want     bool
		reason   string // substring expected in the denial
	}{
		{"anonymous denies first", domauth.Identity{}, []Role{RoleAdmin}, PermPostsRead, false, "not authenticated"},
		{"empty scopes deny despite roles", domauth.Identity{Username: "ada", Via: "jwt"}, []Role{RoleAdmin}, PermPostsRead, false, "scopes"},
		{"narrowed scope denies write", domauth.Identity{Username: "kim", Scopes: []string{"posts:read"}}, []Role{RoleAuthor}, PermPostsWrite, false, "narrowed"},
		{"no roles deny", domauth.Identity{Username: "guest", Scopes: []string{"posts:read", "posts:write"}}, nil, PermPostsWrite, false, "no role"},
		{"unknown role denies", domauth.Identity{Username: "guest", Scopes: []string{"*"}}, []Role{Role("intern")}, PermPostsRead, false, "no role"},
		{"namespace isolation", domauth.Identity{Username: "ada", Scopes: []string{"*"}}, []Role{RoleAuthor}, PermUsersManage, false, "no role"},
		{"reader read allows", domauth.Identity{Username: "linus", Scopes: []string{"posts:read"}}, []Role{RoleReader}, PermPostsRead, true, ""},
		{"author write allows", domauth.Identity{Username: "ada", Scopes: []string{"posts:read", "posts:write"}}, []Role{RoleAuthor}, PermPostsWrite, true, ""},
		{"admin manage via star", domauth.Identity{Username: "root", Scopes: []string{"*"}}, []Role{RoleAdmin}, PermUsersManage, true, ""},
	}
	for _, tc := range cases {
		got := Authorize(tc.identity, tc.roles, tc.perm)
		if got.Allowed != tc.want {
			t.Errorf("%s: Allowed = %t (reason %q), want %t", tc.name, got.Allowed, got.Reason, tc.want)
		}
		if !tc.want && !strings.Contains(got.Reason, tc.reason) {
			t.Errorf("%s: denial reason %q lacks %q", tc.name, got.Reason, tc.reason)
		}
	}
}

// The narrowing property as a property test: a scope set that does not
// permit the permission must deny REGARDLESS of roles — the strongest
// role list in the system cannot widen a narrowed credential.
func TestAuthorizationScopesNarrowNeverWiden(t *testing.T) {
	narrowed := domauth.Identity{Username: "ada", Scopes: []string{"posts:read"}}
	for _, role := range Roles() {
		for _, extra := range []Role{RoleAdmin, RoleEditor, RoleAuthor} {
			if Authorize(narrowed, []Role{role, extra}, PermPostsWrite).Allowed {
				t.Fatalf("roles %v+ %s widened scopes [posts:read] to posts:write", role, extra)
			}
		}
	}
}

func TestAuthorizationDecisionString(t *testing.T) {
	if got := (Decision{Allowed: true, Reason: "owner"}).String(); got != "allow — owner" {
		t.Errorf("allow String() = %q", got)
	}
	if got := (Decision{Allowed: false, Reason: "no role"}).String(); got != "deny — no role" {
		t.Errorf("deny String() = %q", got)
	}
}
