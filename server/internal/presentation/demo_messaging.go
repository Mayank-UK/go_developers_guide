// Demo API handlers for topic 22 messaging — the family's thin-handler
// invariant with the optional-service twist both demos carry: the
// stories run REAL commands against the guide's RUNNING Redis (the
// stream story) and Redis + PostgreSQL (the outbox story), so the
// handlers hold the domain's runner ports, mint a throwaway namespace
// per request, and degrade honestly when a store is unreachable (a
// typed service_unavailable through the central error writer — never a
// panic, never a 500, never a stale answer).
package presentation

import (
	"log/slog"
	"net/http"

	"guide/internal/core"
	"guide/internal/domain/demos"
)

// Compile-time proofs both handlers satisfy http.Handler.
var (
	_ http.Handler = (*MessagingStreamHandler)(nil)
	_ http.Handler = (*MessagingOutboxHandler)(nil)
)

// MessagingStreamHandler serves GET /api/demo/msg/stream, the consumer
// group and ordering demo. It depends on the domain's
// MessagingStreamRunner port — the go-redis-backed adapter is wired at
// the composition root from the configured address — and maps any store
// failure to the 503 envelope: Redis is an OPTIONAL service in this
// stack, so its absence is an honest service_unavailable.
type MessagingStreamHandler struct {
	runner demos.MessagingStreamRunner
	logger *slog.Logger
}

// NewMessagingStreamHandler wires the stream story demo endpoint over
// its runner port.
func NewMessagingStreamHandler(runner demos.MessagingStreamRunner, logger *slog.Logger) *MessagingStreamHandler {
	return &MessagingStreamHandler{runner: runner, logger: logger}
}

// ServeHTTP implements http.Handler for the stream story payload: mint
// a throwaway namespace (concurrent requests never share streams), walk
// the group lifecycle, shape the run, write the JSON.
func (h *MessagingStreamHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	namespace, err := demos.NewMessagingDemoNamespace()
	if err != nil {
		// Minting needs crypto/rand; a failing entropy source is a genuine
		// internal error — not the client's fault and not the store's.
		core.WriteError(w, r, core.Wrap(err, core.KindInternal,
			"The messaging stream demo could not mint a throwaway namespace."), h.logger)
		return
	}
	run, err := h.runner.RunStreamStory(r.Context(), namespace)
	if err != nil {
		core.WriteError(w, r, core.Wrapf(err, core.KindServiceUnavailable,
			"The messaging stream demo runs real commands against the guide's Redis, which is not reachable right now."), h.logger)
		return
	}
	writeJSON(w, http.StatusOK, demos.BuildMessagingStreamPayload(run))
}

// MessagingOutboxHandler serves GET /api/demo/msg/outbox, the
// transactional outbox demo. It depends on the domain's
// MessagingOutboxRunner port — the pgx + go-redis-backed adapter is
// wired at the composition root — and degrades to the 503 envelope when
// either store is unreachable: this endpoint's truth is "what the two
// live stores answered", so an unreachable store is an honest
// service_unavailable, never a stale answer.
type MessagingOutboxHandler struct {
	runner demos.MessagingOutboxRunner
	logger *slog.Logger
}

// NewMessagingOutboxHandler wires the outbox story demo endpoint over
// its runner port.
func NewMessagingOutboxHandler(runner demos.MessagingOutboxRunner, logger *slog.Logger) *MessagingOutboxHandler {
	return &MessagingOutboxHandler{runner: runner, logger: logger}
}

// ServeHTTP implements http.Handler for the outbox story payload: mint
// a throwaway namespace, walk the outbox pattern across both stores,
// shape the run, write the JSON.
func (h *MessagingOutboxHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	namespace, err := demos.NewMessagingDemoNamespace()
	if err != nil {
		core.WriteError(w, r, core.Wrap(err, core.KindInternal,
			"The messaging outbox demo could not mint a throwaway namespace."), h.logger)
		return
	}
	run, err := h.runner.RunOutboxStory(r.Context(), namespace)
	if err != nil {
		core.WriteError(w, r, core.Wrapf(err, core.KindServiceUnavailable,
			"The messaging outbox demo runs a real transaction against the guide's PostgreSQL and Redis, "+
				"which are not both reachable right now."), h.logger)
		return
	}
	writeJSON(w, http.StatusOK, demos.BuildMessagingOutboxPayload(run))
}
