package authorization

import (
	"fmt"
	"strings"

	domauth "guide/internal/domain/auth"
)

// Decision is one verdict with its reason. The reason is part of the
// contract, not decoration: authorization refusals TEACH (which gate
// said no — scope, role, or object) where authentication refusals hide
// (topic 23's uniform 401 tells the caller nothing). A denial that
// cannot explain itself cannot be audited.
type Decision struct {
	// Allowed is the verdict: true means the action may proceed.
	Allowed bool
	// Reason names the rule that decided — carried to the 403 envelope
	// and the audit log.
	Reason string
}

// String renders the verdict for logs and demos: "allow — …" / "deny — …".
func (d Decision) String() string {
	if d.Allowed {
		return "allow — " + d.Reason
	}
	return "deny — " + d.Reason
}

func allow(reason string) Decision { return Decision{Allowed: true, Reason: reason} }
func deny(reason string) Decision  { return Decision{Allowed: false, Reason: reason} }

// ScopePermits reports whether a credential's scopes permit the
// permission — the NARROWING half of the model. The grammar is three
// forms, nothing more: an exact permission ("posts:read"), a whole
// namespace ("posts"), or everything ("*"). An empty scope list permits
// NOTHING: absent scopes are no access, never full access — the
// fail-closed reading that makes scope checks safe to add late.
// Scopes only ever narrow: no scope string can grant a permission the
// caller's roles do not already carry (Authorize intersects the two).
func ScopePermits(scopes []string, perm Permission) bool {
	namespace, _, _ := strings.Cut(string(perm), ":")
	for _, scope := range scopes {
		switch trimmed := strings.TrimSpace(scope); trimmed {
		case "*", namespace, string(perm):
			return true
		}
	}
	return false
}

// Authorize is the capability check — identity × scopes × roles for one
// permission, with no object in hand. It is the gate middleware can
// apply per-route (RequirePermission), and the first half of every
// object-level decision. Order is fixed and load-bearing:
//
//  1. anonymous — the zero Identity denies; authentication precedes
//     authorization, always.
//  2. scopes — the credential's grant is checked FIRST, so a narrowed
//     credential denies before any role lookup: no role can widen what
//     the credential narrowed.
//  3. roles — at least one assigned role must grant the permission;
//     unassigned users and unknown roles land here, denied.
//  4. allowed — both gates opened.
func Authorize(identity domauth.Identity, roles []Role, perm Permission) Decision {
	if Anonymous(identity) {
		return deny("not authenticated — authorization runs after authentication, and an anonymous caller holds nothing")
	}
	if !ScopePermits(identity.Scopes, perm) {
		return deny(fmt.Sprintf("the credential's scopes [%s] do not permit %q — a role cannot widen what the credential narrowed",
			strings.Join(identity.Scopes, " "), perm))
	}
	if !RolesGrant(roles, perm) {
		return deny(fmt.Sprintf("no role in [%s] grants %q — unassigned and unknown roles grant nothing",
			joinRoles(roles), perm))
	}
	return allow(fmt.Sprintf("%q is granted by a role and within the credential's scopes", perm))
}

// joinRoles renders a role list for reasons and demos.
func joinRoles(roles []Role) string {
	names := make([]string, len(roles))
	for i, role := range roles {
		names[i] = string(role)
	}
	return strings.Join(names, " ")
}
