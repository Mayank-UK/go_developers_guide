// Package realtime is the topic 28 substrate: the push machinery both
// live demos run on, pure and websocket-library-free. One Message type
// carries every broadcast payload (ticker ticks, chat lines, system
// notices); one Hub fans Messages out to per-connection subscribers
// through bounded buffers with an explicit slow-consumer policy; one
// ReplayLog retains the newest Messages per stream so a reconnecting
// SSE client can be caught up by id; the SSE frame helpers render the
// wire format byte-exactly; the close-code set names every way a
// WebSocket connection ends in this stack. The presentation layer owns
// the transport (http.Flusher streaming, the coder/websocket handshake)
// and is a thin shell over these pieces.
package realtime

import "time"

// Message is one broadcast payload — the single unit every room fans
// out. Ticker ticks, chat lines, and system notices are all Messages so
// the hub, the replay log, and both live protocols speak one shape:
// SSE marshals it into a data field, WS writes it as one text frame.
type Message struct {
	// Seq is the per-room, hub-assigned sequence number: strictly
	// monotonic per room, never reused, and the id SSE clients resume
	// from (Last-Event-ID) and WS clients order and dedupe by.
	Seq int64 `json:"seq"`
	// Kind is "tick" (the SSE ticker feed), "chat" (a guest line), or
	// "system" (join/leave notices the hub's callers publish).
	Kind string `json:"kind"`
	// From names the publisher: a guest id for chat, "hub" for system
	// notices, "feed" for ticks. Empty means the server itself.
	From string `json:"from,omitempty"`
	// Body is the text payload. For ticks it is the tick label; for
	// chat, the guest's line verbatim (bounded by the read limit).
	Body string `json:"body"`
	// At is the server-side publication time (UTC).
	At time.Time `json:"at"`
}

// CloseCode is a WebSocket close status code (RFC 6455 section 7.4.1)
// as this stack uses it. The domain names its own set — presentation
// maps it onto the websocket library's StatusCode — so the teaching
// table and the handler behavior come from one place.
type CloseCode uint16

// The codes this topic's stack actually closes with, and why each one
// fires. 1005/1006 are reserved (no payload / abnormal) and never sent
// by a server; they appear only on the CLIENT side of a dropped
// connection.
const (
	// CloseNormal is an orderly goodbye: the guest left the room, the
	// demo connection ended by request.
	CloseNormal CloseCode = 1000
	// CloseGoingAway fires when the server is shutting the connection
	// down as the process exits or the request context is canceled.
	CloseGoingAway CloseCode = 1001
	// CloseUnsupportedData refuses a binary frame: this chat speaks
	// JSON text frames only.
	CloseUnsupportedData CloseCode = 1003
	// ClosePolicyViolation refuses a frame the protocol forbids —
	// reserved in this stack for structurally hostile input.
	ClosePolicyViolation CloseCode = 1008
	// CloseMessageTooLarge is the library's automatic answer when a
	// frame exceeds the read limit this stack sets (512 bytes).
	CloseMessageTooLarge CloseCode = 1009
	// CloseInternalError is the honest answer when a write fails
	// underneath us.
	CloseInternalError CloseCode = 1011
	// CloseSlowConsumer evicts a subscriber that stopped reading: its
	// bounded buffer stayed full past the drop budget, so the hub
	// removes it rather than let its backlog grow without bound.
	CloseSlowConsumer CloseCode = 1013
)

// Explain states when this stack closes with the code — the story
// payload's close-code table and the handler share this one string.
func (c CloseCode) Explain() string {
	switch c {
	case CloseNormal:
		return "the guest left the room — an orderly goodbye after the read loop ends"
	case CloseGoingAway:
		return "the request context was canceled — the server is going away underneath the connection"
	case CloseUnsupportedData:
		return "a binary frame arrived — this chat speaks JSON text frames only"
	case ClosePolicyViolation:
		return "input the protocol forbids — structurally hostile frames are refused, not parsed"
	case CloseMessageTooLarge:
		return "the frame exceeded the 512-byte read limit — the library answers this one itself"
	case CloseInternalError:
		return "a write failed underneath the connection — the honest failure, not a hang"
	case CloseSlowConsumer:
		return "the subscriber stopped reading — evicted by the hub's slow-consumer policy after its buffer stayed full"
	default:
		return "an unmapped close code — see RFC 6455 section 7.4.1"
	}
}

// ChatPolicy is the single source of the demo chat's numbers: the
// handler enforces them and the story payload quotes them, so the two
// can never drift apart.
type ChatPolicy struct {
	// ReadLimit caps one inbound frame in bytes (SetReadLimit plus an
	// explicit check): the ceiling on what a client may say at once.
	ReadLimit int64
	// Buffer is each subscriber's outbound channel capacity.
	Buffer int
	// MaxDrops is how many publishes a full-buffered subscriber may
	// miss before the hub evicts it.
	MaxDrops int
	// Heartbeat is the interval between protocol-level pings.
	Heartbeat time.Duration
	// WriteTimeout bounds one outbound frame write.
	WriteTimeout time.Duration
}

// DefaultChatPolicy is the policy the live chat demo runs: generous
// enough for humans, tight enough that the ceiling is real.
func DefaultChatPolicy() ChatPolicy {
	return ChatPolicy{
		ReadLimit:    512,
		Buffer:       16,
		MaxDrops:     8,
		Heartbeat:    15 * time.Second,
		WriteTimeout: 5 * time.Second,
	}
}

// SSEContentType is the one content type an event stream carries;
// charset is stated because the frames are UTF-8 text by definition.
const SSEContentType = "text/event-stream; charset=utf-8"

// SSEStreamRequestHeaders are the response headers every event stream
// needs beside the content type: no-cache so intermediaries do not
// serve a stale stream, and X-Accel-Buffering: no so nginx-style
// proxies pass bytes through instead of batching them.
var SSEStreamRequestHeaders = map[string]string{
	"Cache-Control":     "no-cache",
	"X-Accel-Buffering": "no",
}
