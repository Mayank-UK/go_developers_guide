package authorization

import (
	"strings"
	"testing"
	"time"

	domauth "guide/internal/domain/auth"
)

// owned fixtures: ada owns the sessions post, grace owns the draft —
// the two objects every ownership test judges against.
var (
	adaPost   = NewOwnedPost("p-sessions", "Sealed cookie sessions", "body", "ada", []string{"auth"}, time.Date(2026, 10, 27, 9, 0, 0, 0, time.UTC))
	gracePost = NewOwnedPost("p-draft", "Editing someone else's draft", "body", "grace", []string{"process"}, time.Date(2026, 10, 27, 9, 30, 0, 0, time.UTC))
)

// The exhaustive subject matrix — every caller kind the topic teaches,
// judged on both actions against both objects.
func TestAuthorizationDecidePostMatrix(t *testing.T) {
	subjects := []struct {
		name string
		iden domauth.Identity
		role []Role
	}{
		{"anonymous", domauth.Identity{}, nil},
		{"linus reader", domauth.Identity{Username: "linus", Scopes: []string{"posts:read"}}, []Role{RoleReader}},
		{"ada author own-scope", domauth.Identity{Username: "ada", Scopes: []string{"posts:read", "posts:write"}}, []Role{RoleAuthor}},
		{"kim author narrowed", domauth.Identity{Username: "kim", Scopes: []string{"posts:read"}}, []Role{RoleAuthor}},
		{"grace editor", domauth.Identity{Username: "grace", Scopes: []string{"posts:read", "posts:write"}}, []Role{RoleEditor}},
		{"root admin", domauth.Identity{Username: "root", Scopes: []string{"*"}}, []Role{RoleAdmin}},
		{"guest no role", domauth.Identity{Username: "guest", Scopes: []string{"*"}}, nil},
	}
	// want[actionIdx][subjectIdx][postIdx] — read column then write column.
	readWant := []bool{false, true, true, true, true, true, false}
	writeAdaWant := []bool{false, false, true, false, true, true, false}
	writeGraceWant := []bool{false, false, false, false, true, true, false}
	for si, subject := range subjects {
		if got := DecidePost(subject.iden, subject.role, PermPostsRead, adaPost); got.Allowed != readWant[si] {
			t.Errorf("%s read ada's post = %t (%s), want %t", subject.name, got.Allowed, got.Reason, readWant[si])
		}
		if got := DecidePost(subject.iden, subject.role, PermPostsRead, gracePost); got.Allowed != readWant[si] {
			t.Errorf("%s read grace's post = %t, want %t", subject.name, got.Allowed, readWant[si])
		}
		if got := DecidePost(subject.iden, subject.role, PermPostsWrite, adaPost); got.Allowed != writeAdaWant[si] {
			t.Errorf("%s write ada's post = %t (%s), want %t", subject.name, got.Allowed, got.Reason, writeAdaWant[si])
		}
		if got := DecidePost(subject.iden, subject.role, PermPostsWrite, gracePost); got.Allowed != writeGraceWant[si] {
			t.Errorf("%s write grace's post = %t (%s), want %t", subject.name, got.Allowed, got.Reason, writeGraceWant[si])
		}
	}
}

// The three write refusals must be distinguishable by REASON: scope
// narrowing, missing capability, and object-level — auditability is the
// point of carrying reasons.
func TestAuthorizationDecidePostRefusalReasonsNameTheirGate(t *testing.T) {
	cases := []struct {
		name   string
		iden   domauth.Identity
		roles  []Role
		action Permission
		post   OwnedPost
		gate   string
	}{
		{"narrowed credential", domauth.Identity{Username: "kim", Scopes: []string{"posts:read"}}, []Role{RoleAuthor}, PermPostsWrite, adaPost, "narrowed"},
		{"missing capability", domauth.Identity{Username: "linus", Scopes: []string{"posts:read", "posts:write"}}, []Role{RoleReader}, PermPostsWrite, adaPost, "no role"},
		{"stranger to the object", domauth.Identity{Username: "ada", Scopes: []string{"posts:read", "posts:write"}}, []Role{RoleAuthor}, PermPostsWrite, gracePost, "object-level refusal"},
		{"anonymous", domauth.Identity{}, []Role{RoleAdmin}, PermPostsWrite, adaPost, "not authenticated"},
	}
	for _, tc := range cases {
		got := DecidePost(tc.iden, tc.roles, tc.action, tc.post)
		if got.Allowed {
			t.Fatalf("%s: expected denial", tc.name)
		}
		if !strings.Contains(got.Reason, tc.gate) {
			t.Errorf("%s: reason %q does not name its gate %q", tc.name, got.Reason, tc.gate)
		}
	}
}

// The star trap: a narrowed credential denies on the owner's OWN post —
// the capability gate runs before ownership, so scope narrowing is
// never bypassed by "but it's mine".
func TestAuthorizationNarrowedScopeDeniesEvenOnOwnPost(t *testing.T) {
	kim := domauth.Identity{Username: "grace", Scopes: []string{"posts:read"}}
	got := DecidePost(kim, []Role{RoleEditor}, PermPostsWrite, gracePost)
	if got.Allowed {
		t.Fatal("editor may write their own post only through a credential that carries the write scope")
	}
	if !strings.Contains(got.Reason, "narrowed") {
		t.Errorf("reason %q must name the scope gate, not the object rule", got.Reason)
	}
}

// Readers read everywhere (collection-level); the owner rule lets an
// author write their own; editors span posts; admins override — the
// ladder of allow reasons, one sentence each.
func TestAuthorizationAllowReasonsNameTheirRule(t *testing.T) {
	linus := domauth.Identity{Username: "linus", Scopes: []string{"posts:read"}}
	if got := DecidePost(linus, []Role{RoleReader}, PermPostsRead, gracePost); !got.Allowed ||
		!strings.Contains(got.Reason, "collection-level") {
		t.Errorf("reader read reason = %q", got.Reason)
	}
	ada := domauth.Identity{Username: "ada", Scopes: []string{"posts:read", "posts:write"}}
	if got := DecidePost(ada, []Role{RoleAuthor}, PermPostsWrite, adaPost); !got.Allowed ||
		!strings.Contains(got.Reason, "owns") {
		t.Errorf("owner write reason = %q", got.Reason)
	}
	grace := domauth.Identity{Username: "grace", Scopes: []string{"posts:read", "posts:write"}}
	if got := DecidePost(grace, []Role{RoleEditor}, PermPostsWrite, adaPost); !got.Allowed ||
		!strings.Contains(got.Reason, "editor") {
		t.Errorf("editor write reason = %q", got.Reason)
	}
	root := domauth.Identity{Username: "root", Scopes: []string{"*"}}
	if got := DecidePost(root, []Role{RoleAdmin}, PermPostsWrite, gracePost); !got.Allowed ||
		!strings.Contains(got.Reason, "admin") {
		t.Errorf("admin write reason = %q", got.Reason)
	}
}

func TestAuthorizationNewOwnedPostCarriesEntityFields(t *testing.T) {
	post := NewOwnedPost("id", "Title", "Body", "owner", []string{"a", "b"}, time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	if post.Owner != "owner" || post.ID != "id" || post.Title != "Title" || post.Body != "Body" {
		t.Fatalf("OwnedPost fields = %+v", post)
	}
	if len(post.Tags) != 2 || post.Tags[0] != "a" {
		t.Fatalf("OwnedPost tags = %v", post.Tags)
	}
	if post.CreatedAt.IsZero() || post.UpdatedAt.IsZero() {
		t.Fatal("OwnedPost timestamps must be stamped")
	}
}
