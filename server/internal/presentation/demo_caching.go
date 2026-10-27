// Demo API handler for topic 21 caching — the family's thin-handler
// invariant plus the topic's own subject: the response itself is an HTTP
// caching exhibit. The lab runs against throwaway in-memory stores (nothing
// external to degrade against), so failures are genuine internal errors;
// the payload, being deterministic by construction, is certified with a
// strong ETag and revalidated honestly: a replayed If-None-Match answers
// 304 Not Modified with no body, in the same middleware chain as every
// other response.
package presentation

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"guide/internal/core"
	"guide/internal/domain/demos"
)

// Compile-time proof the handler satisfies http.Handler.
var _ http.Handler = (*CachingLabHandler)(nil)

// CachingLabHandler serves GET /api/demo/caching/lab, the caching topic's
// single demo. It depends on the domain's CachingLabRunner port — the
// cachinglab-backed adapter is wired at the composition root — and shapes
// the run into the deterministic payload whose bytes back the ETag.
type CachingLabHandler struct {
	runner demos.CachingLabRunner
	logger *slog.Logger
}

// NewCachingLabHandler wires the caching lab demo endpoint over its runner
// port.
func NewCachingLabHandler(runner demos.CachingLabRunner, logger *slog.Logger) *CachingLabHandler {
	return &CachingLabHandler{runner: runner, logger: logger}
}

// ServeHTTP implements http.Handler for the lab payload: run the beats,
// encode once, certify the bytes with a strong ETag, and answer a matching
// If-None-Match with 304 — the conditional round trip the topic teaches,
// served by the endpoint that teaches it.
func (h *CachingLabHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	run, err := h.runner.Run(r.Context())
	if err != nil {
		core.WriteError(w, r, core.Wrap(err, core.KindInternal,
			"The caching lab could not run its choreography."), h.logger)
		return
	}

	body, err := json.Marshal(demos.BuildCachingLabPayload(run))
	if err != nil {
		core.WriteError(w, r, core.Wrap(err, core.KindInternal,
			"The caching lab payload could not be encoded."), h.logger)
		return
	}

	// A strong ETag over the exact bytes: two identical payloads hash the
	// same, and the lab's determinism makes every payload identical — the
	// validator can never drift from the representation it certifies.
	digest := sha256.Sum256(body)
	etag := `"` + hex.EncodeToString(digest[:8]) + `"`

	w.Header().Set("Content-Type", "application/json")
	// no-cache does not mean "do not store": it means every reuse must
	// REVALIDATE against this ETag first — the 304 path stays exercised
	// instead of the entry going dark for max-age.
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("ETag", etag)

	if matchesETag(r.Header.Get("If-None-Match"), etag) {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}

// matchesETag answers whether the request's If-None-Match condition holds
// for the response's validator: an exact strong match, or a match anywhere
// in the header's comma-separated list (the grammar HTTP allows). Weak
// validators (the W/ prefix) and the * wildcard are out of this demo's
// scope — honest simplicity, stated rather than half-implemented.
func matchesETag(header, etag string) bool {
	if header == "" {
		return false
	}
	for _, candidate := range strings.Split(header, ",") {
		if strings.TrimSpace(candidate) == etag {
			return true
		}
	}
	return false
}
