// Topic 28 realtime's LIVE transports, over the realtime domain package:
// the SSE ticker stream (one shared feed, every client seeing the same
// ids, http.Flusher-delivered frames, Last-Event-ID resume) and the
// WebSocket chat room (coder/websocket handshake, one read pump and one
// write pump per connection, hub broadcast with the bounded-buffer
// ceiling). The handlers hold only transport concerns: everything
// fan-out-shaped lives in the domain, so what the topic teaches is
// exactly what runs.
package presentation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"guide/internal/core"
	"guide/internal/domain/realtime"

	"github.com/coder/websocket"
)

// Compile-time proofs the two live handlers satisfy http.Handler.
var (
	_ http.Handler = (*RealtimeTickerStreamHandler)(nil)
	_ http.Handler = (*RealtimeChatHandler)(nil)
)

// errFlusherUnavailable is the one failure the stream handler can hit at
// the edge: a ResponseWriter that cannot flush (every real server and
// httptest server can).
var errFlusherUnavailable = errors.New("the response writer cannot flush — no http.Flusher")

// realtimeFrame renders one broadcast Message as its SSE event frame —
// the single rendering path replayed ticks, live ticks, and any other
// room message flow through, so the wire format has one implementation.
func realtimeFrame(msg realtime.Message) string {
	encoded, err := json.Marshal(msg)
	if err != nil {
		// Structurally impossible for this struct (strings, an int64, a
		// time.Time); refusing loudly is the honest shape if it changes.
		panic(fmt.Sprintf("realtime: marshal broadcast message: %v", err))
	}
	return realtime.FormatEvent(msg.Seq, msg.Kind, string(encoded))
}

// RealtimeTickerStreamHandler serves GET /api/demo/realtime/ticker/stream:
// the live event stream the realtime:ticker story runs beside. Every
// connection joins the SAME hub room, so every browser sees the same
// sequence numbers; a connection carrying Last-Event-ID is first caught
// up from the feed's replay log, then continues live; a stream that goes
// quiet gets a comment-frame keepalive so proxies do not reap it.
type RealtimeTickerStreamHandler struct {
	feed           *realtime.TickerFeed
	retryMillis    int
	keepaliveEvery time.Duration
	logger         *slog.Logger
}

// NewRealtimeTickerStreamHandler wires the live stream: the SHARED feed
// (one per process, ticked by the composition root), the reconnect
// advice the first frame carries, how long a stream may stay silent
// before a keepalive goes out, and the process logger.
func NewRealtimeTickerStreamHandler(feed *realtime.TickerFeed, retryMillis int, keepaliveEvery time.Duration, logger *slog.Logger) *RealtimeTickerStreamHandler {
	return &RealtimeTickerStreamHandler{
		feed:           feed,
		retryMillis:    retryMillis,
		keepaliveEvery: keepaliveEvery,
		logger:         logger,
	}
}

// ServeHTTP implements http.Handler for the live event stream.
func (h *RealtimeTickerStreamHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		core.WriteError(w, r, core.Wrap(errFlusherUnavailable, core.KindInternal,
			"This endpoint streams; the response writer cannot flush."), h.logger)
		return
	}
	w.Header().Set("Content-Type", realtime.SSEContentType)
	for key, value := range realtime.SSEStreamRequestHeaders {
		w.Header().Set(key, value)
	}
	w.WriteHeader(http.StatusOK)

	// writeFrame is the ONLY way bytes reach the stream: write, then
	// flush — an unflushed frame is not an event yet.
	writeFrame := func(frame string) bool {
		if _, err := io.WriteString(w, frame); err != nil {
			return false
		}
		flusher.Flush()
		return true
	}

	// Reconnection advice first: EventSource reads it, the page never
	// manages the reconnect.
	if !writeFrame(realtime.FormatRetry(h.retryMillis)) {
		return
	}

	// Resume: a reconnection carries the id of the last event processed.
	// Everything retained after that id is replayed BEFORE the live edge,
	// so the client sees no gap it cannot detect (ids make any gap left
	// visible).
	if last, known := realtime.ParseLastEventID(r.Header.Get("Last-Event-ID")); known {
		for _, missed := range h.feed.ReplayAfter(last) {
			if !writeFrame(realtimeFrame(missed)) {
				return
			}
		}
	}

	sub := &realtime.Subscriber{ID: core.RequestIDFromContext(r.Context())}
	h.feed.Hub().Join(h.feed.Room(), sub)
	defer h.feed.Hub().Leave(h.feed.Room(), sub)

	keepalive := time.NewTimer(h.keepaliveEvery)
	defer keepalive.Stop()
	for {
		select {
		case <-r.Context().Done():
			// The client hung up (or the server is shutting down): the
			// stream ends when its request does.
			return
		case <-keepalive.C:
			if !writeFrame(realtime.FormatComment("keepalive")) {
				return
			}
			keepalive.Reset(h.keepaliveEvery)
		case msg, open := <-sub.Messages():
			if !open {
				// The hub evicted this stream: its bounded buffer stayed
				// full past the drop budget. SSE has no close frame — the
				// honest ending is to stop writing; EventSource reconnects
				// with Last-Event-ID and resume decides what was missed.
				return
			}
			if !writeFrame(realtimeFrame(msg)) {
				return
			}
			keepalive.Reset(h.keepaliveEvery)
		}
	}
}

// chatInbound is the one shape a chat client may send: a JSON text frame
// with a type discriminator. Unknown types and undecodable frames are
// ignored defensively — the room's protocol never crashes on input it
// does not understand.
type chatInbound struct {
	Type string `json:"type"`
	Body string `json:"body"`
}

// RealtimeChatHandler serves GET /api/demo/realtime/chat/ws: the live
// WebSocket room the realtime:chat story runs beside. Each connection is
// one guest with two goroutines (a read pump on the handler's own
// goroutine, a write pump it spawns) and one bounded subscriber buffer;
// chat lines and system notices fan out through the SAME hub the ticker
// stream uses, in a different room.
type RealtimeChatHandler struct {
	hub            *realtime.Hub
	room           string
	policy         realtime.ChatPolicy
	originPatterns []string
	logger         *slog.Logger
	guests         atomic.Int64
}

// NewRealtimeChatHandler wires the live room: the SHARED hub, the room
// name, the policy the room enforces (read limit, buffers, heartbeat —
// the numbers the story payload quotes), the cross-origin patterns the
// handshake allows beside the always-authorized request host, and the
// process logger.
func NewRealtimeChatHandler(hub *realtime.Hub, room string, policy realtime.ChatPolicy, originPatterns []string, logger *slog.Logger) *RealtimeChatHandler {
	return &RealtimeChatHandler{
		hub:            hub,
		room:           room,
		policy:         policy,
		originPatterns: originPatterns,
		logger:         logger,
	}
}

// ServeHTTP implements http.Handler for the live chat room.
func (h *RealtimeChatHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// The handshake: one ordinary HTTP request promoted to a frame
	// protocol. Origin is checked HERE, before any upgrade — the
	// request's own host is always authorized; anything cross-origin
	// must match a stated pattern. A refused handshake is a plain HTTP
	// error already written by the library.
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{OriginPatterns: h.originPatterns})
	if err != nil {
		h.logger.Warn("realtime chat: handshake refused", slog.Any("error", err))
		return
	}

	// One context per connection: whichever side ends first — the client
	// (read pump exits), the server (request context), or the hub
	// (eviction) — cancels the other pump through it.
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	guest := fmt.Sprintf("guest-%d", h.guests.Add(1))
	conn.SetReadLimit(h.policy.ReadLimit)

	sub := &realtime.Subscriber{ID: guest}
	h.hub.Join(h.room, sub)
	// Idempotent by design: after a hub eviction the subscriber is
	// already gone and Leave is a no-op.
	defer h.hub.Leave(h.room, sub)

	h.notice(fmt.Sprintf("%s joined the room — %d online", guest, h.hub.RoomCount(h.room)))

	writeDone := make(chan struct{})
	go func() {
		defer close(writeDone)
		h.writePump(ctx, conn, sub)
	}()

	code, reason := h.readPump(ctx, conn, guest)

	cancel()    // wind the write pump down…
	<-writeDone // …and never write to the conn after this point
	conn.Close(code, reason)
	// The seat goes back BEFORE the room is told: the notice's count
	// must not include the departing guest.
	h.hub.Leave(h.room, sub)
	h.notice(fmt.Sprintf("%s left the room — %d online", guest, h.hub.RoomCount(h.room)))
}

// notice publishes one system message through the same path chat lines
// take — joins and leaves are room events, not side channels.
func (h *RealtimeChatHandler) notice(body string) {
	h.hub.Publish(h.room, "system", "hub", body, time.Now())
}

// readPump is the connection's inbound half, on the handler's own
// goroutine: text frames become room broadcasts, binary frames end the
// connection with the unsupported-data close code, and anything the room
// does not understand is ignored defensively. It returns the close code
// and reason the connection should end with.
func (h *RealtimeChatHandler) readPump(ctx context.Context, conn *websocket.Conn, guest string) (websocket.StatusCode, string) {
	for {
		frameType, data, err := conn.Read(ctx)
		if err != nil {
			return realtimeCloseCode(err)
		}
		switch frameType {
		case websocket.MessageBinary:
			return websocket.StatusUnsupportedData, realtime.CloseUnsupportedData.Explain()
		case websocket.MessageText:
			var line chatInbound
			if json.Unmarshal(data, &line) != nil {
				continue // not the protocol: ignored, never guessed for
			}
			if line.Type == "chat" && strings.TrimSpace(line.Body) != "" {
				h.hub.Publish(h.room, "chat", guest, line.Body, time.Now())
			}
		}
	}
}

// writePump is the connection's outbound half, one goroutine per
// subscriber and the ONLY writer on the conn: it drains the subscriber's
// bounded channel into text frames under a write timeout, pings on the
// heartbeat, and — when the hub evicts a subscriber that stopped
// reading — ends the connection with the policy's close code. On any
// other exit the caller owns the closing handshake and speaks the close
// code the READ pump verdicted.
func (h *RealtimeChatHandler) writePump(ctx context.Context, conn *websocket.Conn, sub *realtime.Subscriber) {
	heartbeat := time.NewTicker(h.policy.Heartbeat)
	defer heartbeat.Stop()
	for {
		select {
		case <-ctx.Done():
			// The reader (or the server) ended the connection; its close
			// verdict — the code readPump returns — is the one the peer
			// must see, and ServeHTTP sends it after this pump unwinds.
			return
		case <-heartbeat.C:
			// Ping waits for a pong the peer's read loop must produce, so
			// it is bounded by the write timeout: a peer that went silent
			// without closing cannot wedge both pumps — the ping times
			// out, the connection closes, the read pump unblocks.
			pingCtx, cancelPing := context.WithTimeout(ctx, h.policy.WriteTimeout)
			err := conn.Ping(pingCtx)
			cancelPing()
			if err != nil {
				conn.Close(websocket.StatusGoingAway, "the peer stopped answering pings")
				return
			}
		case msg, open := <-sub.Messages():
			if !open {
				// The hub closed the channel: this subscriber was
				// evicted by the slow-consumer policy. Buffered messages
				// were drained first; now the connection ends with the
				// eviction's own code and reason.
				code, why := sub.Eviction()
				conn.Close(websocket.StatusCode(code), why)
				return
			}
			encoded, err := json.Marshal(msg)
			if err != nil {
				panic(fmt.Sprintf("realtime: marshal chat message: %v", err))
			}
			writeCtx, cancelWrite := context.WithTimeout(ctx, h.policy.WriteTimeout)
			err = conn.Write(writeCtx, websocket.MessageText, encoded)
			cancelWrite()
			if err != nil {
				conn.Close(websocket.StatusInternalError, realtime.CloseInternalError.Explain())
				return
			}
		}
	}
}

// realtimeCloseCode maps a read-pump error onto the honest ending: a
// close frame from the peer is echoed with its own code, anything else
// (context canceled, dead socket) ends GoingAway — the server
// disappearing underneath the connection, never a hang.
func realtimeCloseCode(err error) (websocket.StatusCode, string) {
	if code := websocket.CloseStatus(err); code != -1 {
		return code, "the client ended the connection"
	}
	return websocket.StatusGoingAway, realtime.CloseGoingAway.Explain()
}
