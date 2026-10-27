// Demo API handlers for topic 23 authentication — the family's
// thin-handler invariant over the seven story ports. The stories run
// the REAL engines over the shared in-memory console, so these handlers
// hold only their runner port and a logger: run the story, shape the
// payload, write the JSON. The substrate is process-local (no optional
// store to degrade over), so a story failure is an honest internal
// error, never a fabricated answer.
package presentation

import (
	"log/slog"
	"net/http"

	"guide/internal/core"
	"guide/internal/domain/demos"
)

// Compile-time proofs the seven handlers satisfy http.Handler.
var (
	_ http.Handler = (*AuthSessionsHandler)(nil)
	_ http.Handler = (*AuthAPIKeysHandler)(nil)
	_ http.Handler = (*AuthCompareHandler)(nil)
	_ http.Handler = (*AuthJWTHandler)(nil)
	_ http.Handler = (*AuthOAuthHandler)(nil)
	_ http.Handler = (*AuthHashingHandler)(nil)
	_ http.Handler = (*AuthRefreshHandler)(nil)
)

// authStoryHandler is the shared body every story endpoint serves: one
// runner call, one payload build, one write — with the story's own
// error mapped to the internal envelope.
func authStoryHandler[Run any](w http.ResponseWriter, r *http.Request, logger *slog.Logger,
	run func() (Run, error), build func(Run) demos.AuthStoryPayload, what string) {
	result, err := run()
	if err != nil {
		core.WriteError(w, r, core.Wrap(err, core.KindInternal,
			"The "+what+" demo could not run its story against the auth console."), logger)
		return
	}
	writeJSON(w, http.StatusOK, build(result))
}

// AuthSessionsHandler serves GET /api/demo/auth/sessions.
type AuthSessionsHandler struct {
	runner demos.AuthSessionsRunner
	logger *slog.Logger
}

// NewAuthSessionsHandler wires the sessions story endpoint.
func NewAuthSessionsHandler(runner demos.AuthSessionsRunner, logger *slog.Logger) *AuthSessionsHandler {
	return &AuthSessionsHandler{runner: runner, logger: logger}
}

// ServeHTTP implements http.Handler for the sessions story.
func (h *AuthSessionsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	authStoryHandler(w, r, h.logger, func() (demos.AuthSessionsRun, error) {
		return h.runner.RunAuthSessionsStory(r.Context())
	}, demos.BuildAuthSessionsPayload, "sessions")
}

// AuthAPIKeysHandler serves GET /api/demo/auth/api-keys.
type AuthAPIKeysHandler struct {
	runner demos.AuthAPIKeysRunner
	logger *slog.Logger
}

// NewAuthAPIKeysHandler wires the API-keys story endpoint.
func NewAuthAPIKeysHandler(runner demos.AuthAPIKeysRunner, logger *slog.Logger) *AuthAPIKeysHandler {
	return &AuthAPIKeysHandler{runner: runner, logger: logger}
}

// ServeHTTP implements http.Handler for the API-keys story.
func (h *AuthAPIKeysHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	authStoryHandler(w, r, h.logger, func() (demos.AuthAPIKeysRun, error) {
		return h.runner.RunAuthAPIKeysStory(r.Context())
	}, demos.BuildAuthAPIKeysPayload, "api-keys")
}

// AuthCompareHandler serves GET /api/demo/auth/compare.
type AuthCompareHandler struct {
	runner demos.AuthCompareRunner
	logger *slog.Logger
}

// NewAuthCompareHandler wires the comparison story endpoint.
func NewAuthCompareHandler(runner demos.AuthCompareRunner, logger *slog.Logger) *AuthCompareHandler {
	return &AuthCompareHandler{runner: runner, logger: logger}
}

// ServeHTTP implements http.Handler for the comparison story.
func (h *AuthCompareHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	authStoryHandler(w, r, h.logger, func() (demos.AuthCompareRun, error) {
		return h.runner.RunAuthCompareStory(r.Context())
	}, demos.BuildAuthComparePayload, "compare")
}

// AuthJWTHandler serves GET /api/demo/auth/jwt.
type AuthJWTHandler struct {
	runner demos.AuthJWTRunner
	logger *slog.Logger
}

// NewAuthJWTHandler wires the JWT story endpoint.
func NewAuthJWTHandler(runner demos.AuthJWTRunner, logger *slog.Logger) *AuthJWTHandler {
	return &AuthJWTHandler{runner: runner, logger: logger}
}

// ServeHTTP implements http.Handler for the JWT story.
func (h *AuthJWTHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	authStoryHandler(w, r, h.logger, func() (demos.AuthJWTRun, error) {
		return h.runner.RunAuthJWTStory(r.Context())
	}, demos.BuildAuthJWTPayload, "jwt")
}

// AuthOAuthHandler serves GET /api/demo/auth/oauth.
type AuthOAuthHandler struct {
	runner demos.AuthOAuthRunner
	logger *slog.Logger
}

// NewAuthOAuthHandler wires the OAuth story endpoint.
func NewAuthOAuthHandler(runner demos.AuthOAuthRunner, logger *slog.Logger) *AuthOAuthHandler {
	return &AuthOAuthHandler{runner: runner, logger: logger}
}

// ServeHTTP implements http.Handler for the OAuth story.
func (h *AuthOAuthHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	authStoryHandler(w, r, h.logger, func() (demos.AuthOAuthRun, error) {
		return h.runner.RunAuthOAuthStory(r.Context())
	}, demos.BuildAuthOAuthPayload, "oauth")
}

// AuthHashingHandler serves GET /api/demo/auth/hashing.
type AuthHashingHandler struct {
	runner demos.AuthHashingRunner
	logger *slog.Logger
}

// NewAuthHashingHandler wires the hashing story endpoint.
func NewAuthHashingHandler(runner demos.AuthHashingRunner, logger *slog.Logger) *AuthHashingHandler {
	return &AuthHashingHandler{runner: runner, logger: logger}
}

// ServeHTTP implements http.Handler for the hashing story.
func (h *AuthHashingHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	authStoryHandler(w, r, h.logger, func() (demos.AuthHashingRun, error) {
		return h.runner.RunAuthHashingStory(r.Context())
	}, demos.BuildAuthHashingPayload, "hashing")
}

// AuthRefreshHandler serves GET /api/demo/auth/refresh.
type AuthRefreshHandler struct {
	runner demos.AuthRefreshRunner
	logger *slog.Logger
}

// NewAuthRefreshHandler wires the refresh story endpoint.
func NewAuthRefreshHandler(runner demos.AuthRefreshRunner, logger *slog.Logger) *AuthRefreshHandler {
	return &AuthRefreshHandler{runner: runner, logger: logger}
}

// ServeHTTP implements http.Handler for the refresh story.
func (h *AuthRefreshHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	authStoryHandler(w, r, h.logger, func() (demos.AuthRefreshRun, error) {
		return h.runner.RunAuthRefreshStory(r.Context())
	}, demos.BuildAuthRefreshPayload, "refresh")
}

// AuthWhoamiIdentity is the /api/demo/auth/whoami response: exactly
// the identity the middleware resolved — the proof that bearer, key,
// and cookie all converge on one shape for topic 24 to consume.
type AuthWhoamiIdentity struct {
	Username    string   `json:"username"`
	DisplayName string   `json:"displayName"`
	Scopes      []string `json:"scopes"`
	Via         string   `json:"via"`
}

// AuthWhoamiHandler serves GET /api/demo/auth/whoami — GUARDED by
// RequireIdentity at the composition root. It answers with whatever
// identity the middleware placed in the context, proving the chain:
// credential in, identity out, one shape regardless of carrier.
type AuthWhoamiHandler struct {
	logger *slog.Logger
}

// NewAuthWhoamiHandler wires the guarded whoami endpoint.
func NewAuthWhoamiHandler(logger *slog.Logger) *AuthWhoamiHandler {
	return &AuthWhoamiHandler{logger: logger}
}

// ServeHTTP implements http.Handler for the whoami proof.
func (h *AuthWhoamiHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	identity, ok := IdentityFromContext(r.Context())
	if !ok {
		// Unreachable behind RequireIdentity — a programmer bug if hit.
		core.WriteError(w, r, core.Unauthorized("not authenticated"), h.logger)
		return
	}
	scopes := identity.Scopes
	if scopes == nil {
		scopes = []string{}
	}
	writeJSON(w, http.StatusOK, AuthWhoamiIdentity{
		Username:    identity.Username,
		DisplayName: identity.DisplayName,
		Scopes:      scopes,
		Via:         identity.Via,
	})
}
