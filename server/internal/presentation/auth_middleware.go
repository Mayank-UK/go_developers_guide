// The topic 23 auth middleware: resolve any of the topic's three
// carriers — Authorization: Bearer (JWT), x-api-key, or the session
// cookie — to the one Identity protected handlers consume, or refuse
// with the standard 401 envelope. This is deliberately a NEW file, not
// an edit of middleware.go: the production chain's composition lines
// are quoted teaching material (topic 08), so the auth middleware is
// wired per-route — demo-scoped guarded routes today, any future
// protected surface the same way — and the production chain's
// composition root lines stay byte-stable.
package presentation

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"guide/internal/core"
	domauth "guide/internal/domain/auth"
)

// BearerScheme prefixes the Authorization header value this middleware
// accepts ("Bearer <token>", case-insensitive per RFC 6750).
const BearerScheme = "bearer "

// APIKeyHeader carries a presented API key.
const APIKeyHeader = "x-api-key"

// identityContextKey namespaces the identity in the request context.
type identityContextKey struct{}

// WithIdentity returns a context carrying the authenticated identity.
func WithIdentity(ctx context.Context, identity domauth.Identity) context.Context {
	return context.WithValue(ctx, identityContextKey{}, identity)
}

// IdentityFromContext answers the authenticated identity, or false
// when the request never passed the auth middleware.
func IdentityFromContext(ctx context.Context) (domauth.Identity, bool) {
	identity, ok := ctx.Value(identityContextKey{}).(domauth.Identity)
	return identity, ok
}

// RequireIdentity wraps a handler so it only runs for an authenticated
// caller. Carrier precedence: an explicit Authorization: Bearer header
// wins, then x-api-key, then the session cookie — the deliberate order
// is "the header the CLIENT named wins over the cookie the BROWSER
// attaches", so a curl with a token is never hijacked by a leftover
// cookie. Every failure mode (missing, malformed, bad signature,
// unknown handle, expired) is ONE uniform 401 envelope: the response
// teaches the caller nothing about which half failed.
func RequireIdentity(resolver domauth.CredentialResolver, logger *slog.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			now := time.Now().UTC()
			var identity domauth.Identity
			var err error
			switch {
			case r.Header.Get("Authorization") != "":
				token, ok := bearerToken(r.Header.Get("Authorization"))
				if !ok {
					core.WriteError(w, r, core.Unauthorized(
						"not authenticated — the Authorization header must be “Bearer <token>”"), logger)
					return
				}
				identity, err = resolver.ResolveBearerToken(r.Context(), token, now)
			case r.Header.Get(APIKeyHeader) != "":
				identity, err = resolver.ResolveAPIKey(r.Context(), r.Header.Get(APIKeyHeader), now)
			case sessionCookie(r) != "":
				identity, err = resolver.ResolveSessionCookie(r.Context(), sessionCookie(r), now)
			default:
				core.WriteError(w, r, core.Unauthorized(
					"not authenticated — send a session cookie, an x-api-key header, or an Authorization: Bearer token"), logger)
				return
			}
			if err != nil {
				// The 401 body stays uniform — the caller must not learn
				// which half failed — but the operator must: a forged token
				// and a key-store outage answer the same envelope, so the
				// resolver's reason goes to the log at warn with the
				// request's id and shape.
				logger.Warn("auth: credential did not verify",
					"requestId", core.RequestIDFromContext(r.Context()),
					"method", r.Method,
					"path", r.URL.Path,
					slog.Any("error", err))
				core.WriteError(w, r, core.Unauthorized("not authenticated — the presented credential did not verify"), logger)
				return
			}
			next.ServeHTTP(w, r.WithContext(WithIdentity(r.Context(), identity)))
		})
	}
}

// bearerToken extracts the token from "Bearer <token>".
func bearerToken(header string) (string, bool) {
	if len(header) < len(BearerScheme) || !strings.EqualFold(header[:len(BearerScheme)], BearerScheme) {
		return "", false
	}
	token := strings.TrimSpace(header[len(BearerScheme):])
	if token == "" {
		return "", false
	}
	return token, true
}

// sessionCookie answers the guide's session cookie value, or "".
func sessionCookie(r *http.Request) string {
	cookie, err := r.Cookie(domauth.SessionCookieName)
	if err != nil {
		return ""
	}
	return cookie.Value
}
