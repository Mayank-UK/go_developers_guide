// CORS teaching exhibit: a compilable, tested CORS middleware the
// security topic quotes. It exists here because the guide's own client
// is served through the same-origin Vite proxy, so no production route
// carries CORS yet — the day one does, this exhibit is the shape to
// lift. The snippet ratchet treats it like any other source of truth.
package teaching

import (
	"net/http"
	"strings"
)

// CORSConfig is the policy one cross-origin route declares. Everything
// a browser needs, nothing more: one origin (or "*" for credentialed-
// free public data), the methods actually served, and the request
// headers the route's own clients send.
type CORSConfig struct {
	Origin           string
	Methods          string
	AllowHeaders     string
	AllowCredentials bool
}

// DefaultCORSConfig is the exhibit's example policy: the guide's own
// client origin, read-only methods, no credentials — the smallest
// policy that is still honest.
var DefaultCORSConfig = CORSConfig{
	Origin:       "http://localhost:5175",
	Methods:      "GET, OPTIONS",
	AllowHeaders: "Content-Type, x-request-id",
}

// CORSMiddleware answers preflights and stamps simple responses per
// the config. The two browser behaviors it serves: an OPTIONS
// preflight must be answered with the allow-headers BEFORE any handler
// logic runs (it carries no credentials and no body), and every actual
// response must name the one origin allowed to READ it — the browser
// enforces what the header promises, the server just has to say it
// truthfully. A wildcard origin is refused together with credentials:
// `Access-Control-Allow-Origin: *` plus `Allow-Credentials: true` is
// the combination browsers reject and attackers love.
func CORSMiddleware(config CORSConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin != "" && origin != config.Origin && config.Origin != "*" {
				// An origin we do not serve: no CORS headers, so the
				// browser blocks the read. The request itself may still
				// proceed for same-origin callers — CORS governs READS
				// by the browser, not who may send.
				next.ServeHTTP(w, r)
				return
			}
			w.Header().Set("Access-Control-Allow-Origin", config.Origin)
			if config.AllowCredentials {
				w.Header().Set("Access-Control-Allow-Credentials", "true")
				w.Header().Set("Vary", "Origin")
			}
			if r.Method == http.MethodOptions {
				w.Header().Set("Access-Control-Allow-Methods", config.Methods)
				w.Header().Set("Access-Control-Allow-Headers", config.AllowHeaders)
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// originMatches is the exhibit's helper for multi-origin configs: an
// exact, case-insensitive membership test — never a prefix or substring
// match, which would let evil-example.com ride an example.com policy.
func originMatches(allowed []string, origin string) bool {
	for _, candidate := range allowed {
		if strings.EqualFold(candidate, origin) {
			return true
		}
	}
	return false
}
