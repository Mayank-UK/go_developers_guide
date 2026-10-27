// The topic 24 authorization middleware: enforcement that BUILDS ON
// topic 23's RequireIdentity rather than replacing it. RequireIdentity
// resolves a credential to the Identity in the request context (or
// answers the uniform 401); these wrappers read that identity and
// answer the question authentication never asks — may the caller DO
// this? The split is the wire contract's: 401 means "I do not know
// you" (authentication), 403 means "I know you and the answer is no"
// (authorization) — an authenticated-but-forbidden request NEVER gets a
// 401 here. Like topic 23's middleware, these are wired per-route at
// the composition root, composing AFTER RequireIdentity.
package presentation

import (
	"context"
	"log/slog"
	"net/http"

	"guide/internal/core"
	domauthz "guide/internal/domain/authorization"
)

// decisionContextKey namespaces the authorization decision the gate
// recorded, so the guarded handler can echo WHY it opened without
// re-running policy.
type decisionContextKey struct{}

// WithDecision returns a context carrying the gate's decision.
func WithDecision(ctx context.Context, decision domauthz.Decision) context.Context {
	return context.WithValue(ctx, decisionContextKey{}, decision)
}

// DecisionFromContext answers the decision the authorization
// middleware recorded, or false when no gate ran.
func DecisionFromContext(ctx context.Context) (domauthz.Decision, bool) {
	decision, ok := ctx.Value(decisionContextKey{}).(domauthz.Decision)
	return decision, ok
}

// RequirePermission wraps a handler so it only runs for callers whose
// identity may hold one permission — the COARSE, per-route gate. It
// resolves the identity's roles through the RoleDirectory port, runs
// the domain evaluator (scopes narrow, roles grant, deny by default),
// and on refusal answers the 403 envelope carrying the evaluator's own
// reason — authorization refusals TEACH which gate said no, where
// topic 23's 401 deliberately said nothing. Wire it AFTER
// RequireIdentity: a missing identity is a composition bug and answers
// 401, never a silent pass-through.
func RequirePermission(directory domauthz.RoleDirectory, permission domauthz.Permission, logger *slog.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			identity, ok := IdentityFromContext(r.Context())
			if !ok {
				core.WriteError(w, r, core.Unauthorized(
					"not authenticated — the permission gate runs after the identity middleware"), logger)
				return
			}
			roles, err := directory.RolesFor(r.Context(), identity.Username)
			if err != nil {
				core.WriteError(w, r, core.Wrap(err, core.KindInternal,
					"The authorization directory could not answer."), logger)
				return
			}
			decision := domauthz.Authorize(identity, roles, permission)
			if !decision.Allowed {
				core.WriteError(w, r, core.Forbidden(
					"forbidden — "+decision.Reason), logger)
				return
			}
			next.ServeHTTP(w, r.WithContext(WithDecision(r.Context(), decision)))
		})
	}
}

// RequireOwnership wraps a handler so it only runs for callers the
// OBJECT-level rule passes: the FINE gate that needs the resource in
// hand. The post id comes from the route's {id} pattern value; the
// owner from the PostOwnership port; the verdict from DecidePost —
// so owners, editors, and admins pass while strangers (and narrowed
// credentials) are refused with the rule that refused them. An unknown
// id answers the 404 envelope BEFORE any 403 — existence is not a
// secret in this guide, and 404-first is the honest stance.
func RequireOwnership(directory domauthz.RoleDirectory, ownership domauthz.PostOwnership,
	action domauthz.Permission, logger *slog.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			identity, ok := IdentityFromContext(r.Context())
			if !ok {
				core.WriteError(w, r, core.Unauthorized(
					"not authenticated — the ownership gate runs after the identity middleware"), logger)
				return
			}
			id := r.PathValue("id")
			owner, found, err := ownership.PostOwner(r.Context(), id)
			if err != nil {
				core.WriteError(w, r, core.Wrap(err, core.KindInternal,
					"The ownership lookup could not answer."), logger)
				return
			}
			if !found {
				core.WriteError(w, r, core.NotFound("No post with id "+id+"."), logger)
				return
			}
			roles, err := directory.RolesFor(r.Context(), identity.Username)
			if err != nil {
				core.WriteError(w, r, core.Wrap(err, core.KindInternal,
					"The authorization directory could not answer."), logger)
				return
			}
			post := domauthz.OwnedPost{Owner: owner}
			post.ID = id
			decision := domauthz.DecidePost(identity, roles, action, post)
			if !decision.Allowed {
				core.WriteError(w, r, core.Forbidden(
					"forbidden — "+decision.Reason), logger)
				return
			}
			next.ServeHTTP(w, r.WithContext(WithDecision(r.Context(), decision)))
		})
	}
}
