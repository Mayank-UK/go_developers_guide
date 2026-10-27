// The topic 25 rate-limit middleware: one per-client token bucket, one
// typed 429 envelope. Like the security-headers middleware this is a
// NEW file wired per-route at the composition root — the production
// chain's composition lines stay byte-stable.
package presentation

import (
	"fmt"
	"log/slog"
	"math"
	"net"
	"net/http"

	"guide/internal/core"
	domsecurity "guide/internal/domain/security"
)

// RetryAfterHeader carries the seconds a refused client should wait
// before retrying — the standard header the envelope's retryAfterMs
// detail mirrors.
const RetryAfterHeader = "Retry-After"

// ClientKey derives the per-client limiter key for a request: the
// authenticated identity when the auth middleware has run (one bucket
// per user, whatever device they call from), else the remote address
// with its port stripped. X-Forwarded-For is deliberately NOT read:
// the header is client-writable, so trusting it would let any caller
// mint a fresh bucket per request and walk straight past the limit.
// Behind a trusted proxy, derive the real client IP at the edge and
// set RemoteAddr (or agree on an explicit trusted-hop contract) — the
// code here takes the position of trusting only what the transport
// guarantees.
func ClientKey(r *http.Request) string {
	if identity, ok := IdentityFromContext(r.Context()); ok && identity.Username != "" {
		return "user:" + identity.Username
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	return "ip:" + host
}

// RateLimit wraps a handler with the per-client limiter. An allowed
// request proceeds; a refused one answers the typed 429 envelope —
// code rate_limited, x-request-id stamped, Retry-After (ceil seconds)
// and a machine-readable retryAfterMs detail riding along. The key is
// derived per request, so a caller carrying an identity shares one
// bucket across IPs, and anonymous callers get one bucket per address.
// The refusal message is key-free: which user or IP tripped lives in
// the server's own logs (request id joins them), never in a response
// the triggering client controls the audience of.
func RateLimit(limiter *domsecurity.ClientRateLimiter, logger *slog.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			decision := limiter.Allow(ClientKey(r))
			if decision.Allowed {
				next.ServeHTTP(w, r)
				return
			}
			retrySeconds := int(math.Ceil(decision.RetryAfter.Seconds()))
			w.Header().Set(RetryAfterHeader, fmt.Sprintf("%d", retrySeconds))
			refusal := core.RateLimited(fmt.Sprintf(
				"rate limit exceeded — the sustained allowance refills continuously; retry after %s",
				decision.RetryAfterText()))
			refusal.Details = map[string]any{
				"retryAfterMs": decision.RetryAfter.Milliseconds(),
				"policy":       limiter.Policy().String(),
			}
			core.WriteError(w, r, refusal, logger)
		})
	}
}
