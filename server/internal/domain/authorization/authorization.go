// Package authorization is the guide's authorization domain area (topic
// 24): what an authenticated caller may DO. It consumes topic 23's
// Identity (scopes are vocabulary there; here they become verdicts) and
// the posts resource entities, and answers decisions from three rules
// stacked in a fixed order — deny by default, scopes narrow, roles
// grant, ownership decides at the object. Everything here is pure
// policy: no storage, no clock, no I/O, which is what makes the
// evaluator table-testable against every cell of the matrix. The
// assignments (who holds which role) and the owned posts live behind
// the two ports at the bottom of this file; the in-memory adapters with
// the demo seeds live in internal/data/authorization.
package authorization

import (
	"context"

	domauth "guide/internal/domain/auth"
)

// Permission is one capability in the service's closed vocabulary —
// namespace:verb, the same string shape a credential's scopes use, so a
// scope can narrow a permission by exact string comparison with no
// translation layer between the two worlds.
type Permission string

// The permission vocabulary this guide protects. Two namespaces is
// deliberate: it proves namespace isolation (a posts grant never
// touches users) the moment more than one exists.
const (
	// PermPostsRead reads any post — collection-level; no object rule.
	PermPostsRead Permission = "posts:read"
	// PermPostsWrite creates, updates, and deletes posts — the verb the
	// object-level ownership rule polices.
	PermPostsWrite Permission = "posts:write"
	// PermUsersManage manages user accounts — the admin-only island.
	PermUsersManage Permission = "users:manage"
)

// Role is a named bundle of permissions assigned to users. Roles earn
// their keep over raw permission lists because they are assigned in
// bulk, audited in one place, and — the part beginners miss — carry
// OBJECT-LEVEL semantics too (an author and an editor hold the same
// permissions yet differ on whose posts they may write; see DecidePost).
type Role string

// The role ladder, coarsest privilege first.
const (
	// RoleReader reads posts; the smallest honest role.
	RoleReader Role = "reader"
	// RoleAuthor reads and writes THEIR OWN posts.
	RoleAuthor Role = "author"
	// RoleEditor reads and writes ANY post — the object-level override
	// for trusted maintainers.
	RoleEditor Role = "editor"
	// RoleAdmin holds every permission and overrides ownership.
	RoleAdmin Role = "admin"
)

// roleRegistry is the single place a role's permissions are defined.
// A role NOT in this map grants NOTHING — the deny-by-default property
// the evaluator leans on: unknown role, empty role list, unassigned
// user, all land in the same bucket (no grant).
var roleRegistry = map[Role][]Permission{
	RoleReader: {PermPostsRead},
	RoleAuthor: {PermPostsRead, PermPostsWrite},
	RoleEditor: {PermPostsRead, PermPostsWrite},
	RoleAdmin:  {PermPostsRead, PermPostsWrite, PermUsersManage},
}

// Roles answers the role vocabulary in privilege order — the roster the
// demo renders and the order tests walk.
func Roles() []Role {
	return []Role{RoleReader, RoleAuthor, RoleEditor, RoleAdmin}
}

// RoleGrants reports whether one role grants a permission. Unknown
// roles grant nothing: the map miss answers false, which is the whole
// deny-by-default stance in one lookup.
func RoleGrants(role Role, perm Permission) bool {
	for _, granted := range roleRegistry[role] {
		if granted == perm {
			return true
		}
	}
	return false
}

// RolesGrant reports whether ANY of the roles grants the permission —
// the union semantics real assignments expect (a user holding both
// reader and author gets the author's write). An empty list grants
// nothing.
func RolesGrant(roles []Role, perm Permission) bool {
	for _, role := range roles {
		if RoleGrants(role, perm) {
			return true
		}
	}
	return false
}

// PermissionsOf answers a role's permission set, or false when the
// role is unknown — callers can render the roster without guarding
// against fabricated entries. The slice is a copy; callers may keep it.
func PermissionsOf(role Role) ([]Permission, bool) {
	granted, ok := roleRegistry[role]
	if !ok {
		return nil, false
	}
	out := make([]Permission, len(granted))
	copy(out, granted)
	return out, true
}

// RoleDirectory resolves a username to its roles — the port the
// middleware consults per request. The contract is deliberately flat:
// an unassigned username answers an empty slice (deny by default),
// never an error; errors are reserved for a store that cannot answer.
// Production backs this with a roles table or group claims from the
// identity provider; the guide backs it with the seeded directory in
// internal/data/authorization.
type RoleDirectory interface {
	// RolesFor answers the roles assigned to username; empty means
	// unassigned, which the evaluator turns into a universal deny.
	RolesFor(ctx context.Context, username string) ([]Role, error)
}

// PostOwnership answers who owns a post — the port the object-level
// rule needs, because ownership lives with the resource, not the
// caller. found=false means no such post (the handler maps it to the
// 404 envelope BEFORE any 403, so existence never leaks through
// authorization refusals... in this guide the reverse is taught
// honestly: 404 first is the chosen stance, argued in the topic).
type PostOwnership interface {
	// PostOwner answers the owning username of the post at id.
	PostOwner(ctx context.Context, id string) (owner string, found bool, err error)
}

// The input the whole package judges: topic 23's Identity — WHO is
// calling and which scopes the credential carries. The zero Identity
// (empty Username) is anonymous, and every evaluator here denies it.

// Anonymous reports whether the identity is the zero value — the shape
// the auth middleware never produces but a bare call site might.
func Anonymous(identity domauth.Identity) bool {
	return identity.Username == ""
}
