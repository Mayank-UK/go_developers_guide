// Demo API handlers for topic 28 realtime's two STORY endpoints — the
// family's thin-handler invariant at its thinnest: both builders are
// pure functions over the real realtime package (the ticker story
// derives its frames and resume math through the real feed; the chat
// story runs a real hub choreography), so there is no input to parse, no
// state to consult, no failure to map — build the payload, write the
// JSON. The LIVE transports these stories point at live in websocket.go.
package presentation

import (
	"net/http"

	"guide/internal/domain/demos"
)

// Compile-time proofs the two story handlers satisfy http.Handler.
var (
	_ http.Handler = (*RealtimeTickerStoryHandler)(nil)
	_ http.Handler = (*RealtimeChatStoryHandler)(nil)
)

// RealtimeTickerStoryHandler serves GET /api/demo/realtime/ticker: the
// SSE story — frame anatomy rendered by the real formatter, the response
// contract, the resume math off a real replay log, keepalive bytes, the
// at-least-once honesty, and the pointer to the live stream.
type RealtimeTickerStoryHandler struct{}

// NewRealtimeTickerStoryHandler wires the ticker story endpoint. The
// builder is pure and cannot fail, so the handler holds no dependencies.
func NewRealtimeTickerStoryHandler() *RealtimeTickerStoryHandler {
	return &RealtimeTickerStoryHandler{}
}

// ServeHTTP implements http.Handler for the ticker story payload.
func (h *RealtimeTickerStoryHandler) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, demos.BuildRealtimeTickerPayload())
}

// RealtimeChatStoryHandler serves GET /api/demo/realtime/chat: the WS
// story — the handshake, a real hub choreography with a real eviction,
// the ceiling's numbers, the close-code table, and the pointer to the
// live room.
type RealtimeChatStoryHandler struct{}

// NewRealtimeChatStoryHandler wires the chat story endpoint.
func NewRealtimeChatStoryHandler() *RealtimeChatStoryHandler {
	return &RealtimeChatStoryHandler{}
}

// ServeHTTP implements http.Handler for the chat story payload.
func (h *RealtimeChatStoryHandler) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, demos.BuildRealtimeChatPayload())
}
