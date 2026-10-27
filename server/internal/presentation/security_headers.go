// The topic 25 security-headers middleware: apply the domain's header
// policy to every response a wrapped route serves. This is
// deliberately a NEW file, not an edit of middleware.go — the
// production chain's composition lines are quoted teaching material
// (topic 08), so the header policy is wired per-route at the
// composition root (the topic 25 demo routes today, any future surface
// the same way) and the chain's composition lines stay byte-stable.
package presentation

import (
	"net/http"

	domsecurity "guide/internal/domain/security"
)

// SecurityHeaders wraps a handler so every response the WRAPPED route
// serves — success bodies AND error envelopes — carries the policy's
// headers. The headers are set BEFORE the wrapped handler runs, so
// nothing the handler or a deeper middleware does can skip them,
// including the central error writer's envelopes. Placement is the
// honest caveat: wrapped per-route (this guide's wiring), the headers
// ride that route's responses but not the mux's own unmatched-path
// 404s — those never reach route middleware; wrapping the whole router
// covers those too, at the cost of applying the policy to every route
// including ones that need a looser CSP. Header.Set is idempotent:
// applying this middleware twice (or nesting two wrapped routes)
// leaves one clean copy of each header, never a folded list.
func SecurityHeaders(policy []domsecurity.HeaderRule) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			for _, rule := range policy {
				w.Header().Set(rule.Header, rule.Value)
			}
			next.ServeHTTP(w, r)
		})
	}
}
