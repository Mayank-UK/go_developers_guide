// Demo API handlers for topic 24 authorization. Two shapes, the family
// invariants throughout: the matrix endpoint is the topic's narrative
// payload — one request, the whole matrix evaluated server-side through
// the real evaluator (unauthenticated like every /api/demo/* story);
// the two guarded proofs are the middleware made visible — RequireIdentity
// composed with RequirePermission / RequireOwnership, wired at the
// composition root, answering the identity the auth middleware resolved
// and the decision the authz gate recorded (never re-derived here).
package presentation

import (
	"log/slog"
	"net/http"

	"guide/internal/core"
	domauthz "guide/internal/domain/authorization"
	"guide/internal/domain/demos"
)

// Compile-time proofs the three handlers satisfy http.Handler.
var (
	_ http.Handler = (*AuthzMatrixHandler)(nil)
	_ http.Handler = (*AuthzGuardedHandler)(nil)
	_ http.Handler = (*AuthzGuardedPostHandler)(nil)
)

// AuthzMatrixHandler serves GET /api/demo/authz/matrix — the topic's
// one-request tour: roles, subjects, owned posts, and every decision
// the evaluator answered over them.
type AuthzMatrixHandler struct {
	runner demos.AuthzMatrixRunner
	logger *slog.Logger
}

// NewAuthzMatrixHandler wires the matrix story endpoint.
func NewAuthzMatrixHandler(runner demos.AuthzMatrixRunner, logger *slog.Logger) *AuthzMatrixHandler {
	return &AuthzMatrixHandler{runner: runner, logger: logger}
}

// ServeHTTP implements http.Handler for the matrix story.
func (h *AuthzMatrixHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	run, err := h.runner.RunAuthzMatrixStory(r.Context())
	if err != nil {
		core.WriteError(w, r, core.Wrap(err, core.KindInternal,
			"The authorization matrix demo could not run its story."), h.logger)
		return
	}
	writeJSON(w, http.StatusOK, demos.BuildAuthzMatrixPayload(run))
}

// AuthzGuardedProof is the permission gate's answer when it OPENS: who
// the identity middleware resolved, which permission was demanded, and
// the decision the gate recorded — echo, never re-derivation.
type AuthzGuardedProof struct {
	Username    string   `json:"username"`
	DisplayName string   `json:"displayName"`
	Via         string   `json:"via"`
	Scopes      []string `json:"scopes"`
	Permission  string   `json:"permission"`
	Reason      string   `json:"reason"`
}

// AuthzGuardedHandler serves GET /api/demo/authz/guarded, wired behind
// RequireIdentity ∘ RequirePermission(posts:write): it only ever runs
// for a caller the gate already passed.
type AuthzGuardedHandler struct {
	permission domauthz.Permission
	logger     *slog.Logger
}

// NewAuthzGuardedHandler wires the guarded proof endpoint; the
// permission is echoed from what the composition root guarded with.
func NewAuthzGuardedHandler(permission domauthz.Permission, logger *slog.Logger) *AuthzGuardedHandler {
	return &AuthzGuardedHandler{permission: permission, logger: logger}
}

// ServeHTTP implements http.Handler for the guarded proof.
func (h *AuthzGuardedHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	identity, ok := IdentityFromContext(r.Context())
	if !ok {
		// Unreachable behind RequireIdentity — a programmer bug if hit.
		core.WriteError(w, r, core.Unauthorized("not authenticated"), h.logger)
		return
	}
	decision, _ := DecisionFromContext(r.Context())
	scopes := identity.Scopes
	if scopes == nil {
		scopes = []string{}
	}
	writeJSON(w, http.StatusOK, AuthzGuardedProof{
		Username:    identity.Username,
		DisplayName: identity.DisplayName,
		Via:         identity.Via,
		Scopes:      scopes,
		Permission:  string(h.permission),
		Reason:      decision.Reason,
	})
}

// AuthzGuardedPostProof is the ownership gate's answer when it OPENS:
// the identity, the object, its owner, and the rule that passed.
type AuthzGuardedPostProof struct {
	Username string `json:"username"`
	PostID   string `json:"postId"`
	Owner    string `json:"owner"`
	Action   string `json:"action"`
	Reason   string `json:"reason"`
}

// AuthzGuardedPostHandler serves GET /api/demo/authz/guarded/post/{id},
// wired behind RequireIdentity ∘ RequireOwnership(posts:write).
type AuthzGuardedPostHandler struct {
	action    domauthz.Permission
	ownership domauthz.PostOwnership
	logger    *slog.Logger
}

// NewAuthzGuardedPostHandler wires the guarded post proof endpoint; it
// needs the ownership port to echo the object's owner.
func NewAuthzGuardedPostHandler(action domauthz.Permission, ownership domauthz.PostOwnership,
	logger *slog.Logger) *AuthzGuardedPostHandler {
	return &AuthzGuardedPostHandler{action: action, ownership: ownership, logger: logger}
}

// ServeHTTP implements http.Handler for the guarded post proof.
func (h *AuthzGuardedPostHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	identity, ok := IdentityFromContext(r.Context())
	if !ok {
		// Unreachable behind RequireIdentity — a programmer bug if hit.
		core.WriteError(w, r, core.Unauthorized("not authenticated"), h.logger)
		return
	}
	id := r.PathValue("id")
	owner, found, err := h.ownership.PostOwner(r.Context(), id)
	if err != nil || !found {
		// Unreachable behind RequireOwnership's 404-first stance.
		core.WriteError(w, r, core.NotFound("No post with id "+id+"."), h.logger)
		return
	}
	decision, _ := DecisionFromContext(r.Context())
	writeJSON(w, http.StatusOK, AuthzGuardedPostProof{
		Username: identity.Username,
		PostID:   id,
		Owner:    owner,
		Action:   string(h.action),
		Reason:   decision.Reason,
	})
}
