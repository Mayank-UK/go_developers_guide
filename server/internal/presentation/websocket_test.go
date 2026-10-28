package presentation

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"guide/internal/domain/demos"
	"guide/internal/domain/realtime"

	"github.com/coder/websocket"
)

// Topic 28's live transports over the REAL stack: the SSE stream serves
// real HTTP with real flushing (byte-exact frames read off the wire),
// and the chat room serves real websocket connections dialed with the
// same library the guide teaches — join notices, broadcasts, close
// codes, and the slow-consumer eviction all observed end-to-end under
// -race. The story endpoints are pinned the family way (shape + the
// builder's own derived bytes, which their suite in domain/demos
// already pins byte-for-byte).

// fixedNow is the deterministic clock the test feeds tick on — frames
// become byte-comparable to realtime.FormatEvent's own output.
var realtimeTestNow = time.Date(2026, 10, 27, 10, 0, 0, 0, time.UTC)

func newTickerStreamServer(t *testing.T, keepaliveEvery time.Duration) (*httptest.Server, *realtime.TickerFeed) {
	t.Helper()
	logger := newTestLogger(&strings.Builder{})
	hub, err := realtime.NewHub(realtime.DefaultHubPolicy())
	if err != nil {
		t.Fatalf("hub: %v", err)
	}
	// The interval is an hour: the TEST is the pacer — TickOnce drives
	// the feed while the stream is live, exactly like the story says.
	feed, err := realtime.NewTickerFeed(hub, "ticker", 64, realtime.FeedOptions{
		Interval: time.Hour,
		Now:      func() time.Time { return realtimeTestNow },
	})
	if err != nil {
		t.Fatalf("feed: %v", err)
	}
	mux := http.NewServeMux()
	mux.Handle("GET /api/demo/realtime/ticker/stream",
		NewRealtimeTickerStreamHandler(feed, 3000, keepaliveEvery, logger))
	server := httptest.NewServer(Chain(RequestID, Recover(logger))(mux))
	t.Cleanup(server.Close)
	return server, feed
}

// readSSEFrames reads n blank-line-terminated frames off a live stream
// body, failing the test on stall instead of hanging forever.
func readSSEFrames(t *testing.T, body io.Reader, n int) []string {
	t.Helper()
	scanned := bufio.NewScanner(body)
	scanned.Buffer(make([]byte, 0, 64*1024), 64*1024)
	frames := make([]string, 0, n)
	var current strings.Builder
	deadline := time.After(5 * time.Second)
	done := make(chan error, 1)
	go func() {
		for scanned.Scan() {
			line := scanned.Text()
			if line == "" {
				// The blank line IS the terminator: keep it in the frame
				// so reconstructions compare byte-exactly with what
				// FormatEvent renders (every frame ends "\n\n").
				current.WriteByte('\n')
				frames = append(frames, current.String())
				current.Reset()
				if len(frames) == n {
					done <- nil
					return
				}
				continue
			}
			current.WriteString(line)
			current.WriteByte('\n')
		}
		done <- scanned.Err()
	}()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("stream read ended early after %d frames: %v", len(frames), err)
		}
	case <-deadline:
		t.Fatalf("timed out waiting for frame %d (got %d: %q)", n, len(frames), frames)
	}
	return frames
}

func openStream(t *testing.T, server *httptest.Server, lastEventID string) (*http.Response, context.CancelFunc) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, server.URL+"/api/demo/realtime/ticker/stream", nil)
	if err != nil {
		cancel()
		t.Fatalf("request: %v", err)
	}
	if lastEventID != "" {
		req.Header.Set("Last-Event-ID", lastEventID)
	}
	resp, err := server.Client().Do(req)
	if err != nil {
		cancel()
		t.Fatalf("stream connect: %v", err)
	}
	t.Cleanup(func() {
		resp.Body.Close()
		cancel()
	})
	return resp, cancel
}

// tickFrame is the byte-exact expectation for one feed tick, derived
// the same way the handler derives it.
func tickFrame(t *testing.T, seq int64) string {
	t.Helper()
	encoded, err := json.Marshal(realtime.Message{
		Seq: seq, Kind: "tick", From: "feed", Body: "tick", At: realtimeTestNow,
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return realtime.FormatEvent(seq, "tick", string(encoded))
}

// waitForRoomCount blocks until the feed's room holds exactly n
// subscribers. The handler flushes its retry frame BEFORE joining the
// room, so a tick driven on the flush alone races the join — gating on
// the registered subscription makes tick-then-read deterministic.
func waitForRoomCount(t *testing.T, feed *realtime.TickerFeed, n int) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for {
		if got := feed.Hub().RoomCount(feed.Room()); got == n {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("room count = %d, want %d — the subscription never registered",
				feed.Hub().RoomCount(feed.Room()), n)
		}
		time.Sleep(time.Millisecond)
	}
}

func TestRealtimeTickerStreamServesHeadersRetryAndByteExactFrames(t *testing.T) {
	server, feed := newTickerStreamServer(t, time.Minute) // no keepalives mid-test
	resp, _ := openStream(t, server, "")
	waitForRoomCount(t, feed, 1)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	if got := resp.Header.Get("Content-Type"); got != realtime.SSEContentType {
		t.Errorf("Content-Type = %q, want %q", got, realtime.SSEContentType)
	}
	if got := resp.Header.Get("Cache-Control"); got != "no-cache" {
		t.Errorf("Cache-Control = %q", got)
	}
	if got := resp.Header.Get("X-Accel-Buffering"); got != "no" {
		t.Errorf("X-Accel-Buffering = %q", got)
	}

	feed.TickOnce()
	feed.TickOnce()

	frames := readSSEFrames(t, resp.Body, 3)
	if frames[0] != "retry: 3000\n\n" {
		t.Errorf("first frame = %q, want the retry advice", frames[0])
	}
	// Byte-exactness: what the handler wrote is what FormatEvent
	// renders for the same message — id, event, data, nothing else.
	if frames[1] != tickFrame(t, 1) {
		t.Errorf("tick 1 frame = %q, want %q", frames[1], tickFrame(t, 1))
	}
	if frames[2] != tickFrame(t, 2) {
		t.Errorf("tick 2 frame = %q, want %q", frames[2], tickFrame(t, 2))
	}
	if !strings.HasPrefix(frames[1], "id: 1\nevent: tick\ndata: {") {
		t.Errorf("frame field order = %q", frames[1])
	}
}

func TestRealtimeTickerStreamResumesFromLastEventID(t *testing.T) {
	server, feed := newTickerStreamServer(t, time.Minute)
	for i := 0; i < 5; i++ {
		feed.TickOnce() // seqs 1..5 retained
	}

	// A reconnect that last processed seq 3: exactly 4 and 5 replay,
	// then the stream continues LIVE at 6 — no duplicate of 3, no skip.
	resp, _ := openStream(t, server, "3")
	frames := readSSEFrames(t, resp.Body, 3)
	if frames[0] != "retry: 3000\n\n" {
		t.Fatalf("first frame = %q", frames[0])
	}
	if frames[1] != tickFrame(t, 4) || frames[2] != tickFrame(t, 5) {
		t.Errorf("replay frames = %q %q, want seqs 4 and 5", frames[1], frames[2])
	}

	feed.TickOnce() // seq 6 — live, after the replay
	live := readSSEFrames(t, resp.Body, 1)
	if live[0] != tickFrame(t, 6) {
		t.Errorf("live frame = %q, want seq 6 after the replay", live[0])
	}

	// An unparsable id is an unknown id: start fresh, replay nothing.
	resp2, _ := openStream(t, server, "not-a-number")
	frames2 := readSSEFrames(t, resp2.Body, 1)
	if frames2[0] != "retry: 3000\n\n" {
		t.Fatalf("garbage-id first frame = %q", frames2[0])
	}
	feed.TickOnce() // seq 7
	live2 := readSSEFrames(t, resp2.Body, 1)
	if live2[0] != tickFrame(t, 7) {
		t.Errorf("fresh stream live frame = %q, want seq 7 with no replay", live2[0])
	}
}

func TestRealtimeTickerStreamKeepalivesAnIdleConnection(t *testing.T) {
	server, _ := newTickerStreamServer(t, 20*time.Millisecond)
	resp, _ := openStream(t, server, "")

	// No ticks are driven: the ONLY bytes that can arrive are the retry
	// advice and then comment frames every 20ms.
	frames := readSSEFrames(t, resp.Body, 2)
	if frames[0] != "retry: 3000\n\n" {
		t.Fatalf("first frame = %q", frames[0])
	}
	if frames[1] != ": keepalive\n\n" {
		t.Errorf("keepalive frame = %q, want the comment frame", frames[1])
	}
}

func TestRealtimeTickerStreamEndsWithItsRequest(t *testing.T) {
	server, feed := newTickerStreamServer(t, time.Minute)
	resp, cancel := openStream(t, server, "")
	waitForRoomCount(t, feed, 1)
	feed.TickOnce()
	readSSEFrames(t, resp.Body, 2) // retry + tick 1: subscribed and live

	if got := feed.Hub().RoomCount(feed.Room()); got != 1 {
		t.Fatalf("room count while streaming = %d, want 1", got)
	}
	cancel() // the client hangs up (Ctrl-C on the curl tour)
	// The property is structural — the room reaches zero when the request
	// ends — so the bound only fails a REAL leak (a subscriber that never
	// leaves). 10s is the generous wall-clock window for the handler
	// goroutine to observe the hangup under full-suite container load; a
	// leak misses it by orders of magnitude either way.
	deadline := time.Now().Add(10 * time.Second)
	for feed.Hub().RoomCount(feed.Room()) != 0 {
		if time.Now().After(deadline) {
			t.Fatalf("subscriber leaked after hangup: %d still in the room", feed.Hub().RoomCount(feed.Room()))
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// ── the WebSocket chat room ───────────────────────────────────────────────

// testChatPolicy is the room's policy tuned for tests: a 2-slot buffer
// with a 2-drop budget (the eviction is four publishes away), a fast
// heartbeat, and a short write timeout.
func testChatPolicy() realtime.ChatPolicy {
	return realtime.ChatPolicy{
		ReadLimit:    512,
		Buffer:       2,
		MaxDrops:     2,
		Heartbeat:    40 * time.Millisecond,
		WriteTimeout: 500 * time.Millisecond,
	}
}

type chatServer struct {
	*httptest.Server
	hub *realtime.Hub
}

func newChatServer(t *testing.T, policy realtime.ChatPolicy) *chatServer {
	t.Helper()
	logger := newTestLogger(&strings.Builder{})
	hub, err := realtime.NewHub(realtime.HubPolicy{Buffer: policy.Buffer, MaxDrops: policy.MaxDrops})
	if err != nil {
		t.Fatalf("hub: %v", err)
	}
	mux := http.NewServeMux()
	mux.Handle("GET /api/demo/realtime/chat/ws",
		NewRealtimeChatHandler(hub, "chat", policy, nil, logger))
	mux.Handle("GET /api/demo/realtime/ticker", NewRealtimeTickerStoryHandler())
	mux.Handle("GET /api/demo/realtime/chat", NewRealtimeChatStoryHandler())
	server := &chatServer{
		Server: httptest.NewServer(Chain(RequestID, Recover(logger), EnvelopeMuxErrors(logger))(mux)),
		hub:    hub,
	}
	t.Cleanup(server.Close)
	return server
}

func dialChat(t *testing.T, server *chatServer, origin string) *websocket.Conn {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	options := &websocket.DialOptions{}
	if origin != "" {
		options.HTTPHeader = http.Header{"Origin": []string{origin}}
	}
	conn, _, err := websocket.Dial(ctx, server.URL+"/api/demo/realtime/chat/ws", options)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	conn.SetReadLimit(1 << 20)
	t.Cleanup(func() {
		// Best-effort orderly goodbye; the connection may already be gone.
		_ = conn.Close(websocket.StatusNormalClosure, "test done")
	})
	return conn
}

// chatMessage is the wire shape the room broadcasts.
type chatMessage struct {
	Seq  int64  `json:"seq"`
	Kind string `json:"kind"`
	From string `json:"from"`
	Body string `json:"body"`
}

// readChatMessage reads one server frame and decodes it, failing the
// test on stall.
func readChatMessage(t *testing.T, conn *websocket.Conn) chatMessage {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_, data, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("read chat frame: %v", err)
	}
	var msg chatMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		t.Fatalf("decode %q: %v", data, err)
	}
	return msg
}

// waitFor drains frames until one matches, or fails on stall — the
// honest way to step over join notices to the message under test.
func waitFor(t *testing.T, conn *websocket.Conn, match func(chatMessage) bool) chatMessage {
	t.Helper()
	deadline := time.After(4 * time.Second)
	for {
		select {
		case <-deadline:
			t.Fatal("the expected message never arrived")
		default:
		}
		msg := readChatMessage(t, conn)
		if match(msg) {
			return msg
		}
	}
}

func sendChat(t *testing.T, conn *websocket.Conn, payload string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := conn.Write(ctx, websocket.MessageText, []byte(payload)); err != nil {
		t.Fatalf("write %q: %v", payload, err)
	}
}

func TestRealtimeChatBroadcastsToEveryMemberIncludingTheSender(t *testing.T) {
	server := newChatServer(t, testChatPolicy())
	ada := dialChat(t, server, "")
	grace := dialChat(t, server, "")

	// Both members see BOTH join notices — joins are room events through
	// the same broadcast path chat lines take, and the count is honest.
	join2 := waitFor(t, ada, func(m chatMessage) bool {
		return strings.Contains(m.Body, "joined") && strings.Contains(m.Body, "2 online")
	})
	if join2.Kind != "system" || join2.From != "hub" || join2.Seq == 0 {
		t.Errorf("join notice = %+v, want a hub system message with a seq", join2)
	}
	waitFor(t, grace, func(m chatMessage) bool { return strings.Contains(m.Body, "2 online") })

	// One chat line from ada: BOTH members receive it — grace hears it,
	// and ada's own echo is the confirmation the room served the line.
	sendChat(t, ada, `{"type":"chat","body":"hello room"}`)
	echo := waitFor(t, ada, func(m chatMessage) bool { return m.Kind == "chat" })
	heard := waitFor(t, grace, func(m chatMessage) bool { return m.Kind == "chat" })
	if echo.Body != "hello room" || heard.Body != "hello room" {
		t.Errorf("chat line = %q / %q", echo.Body, heard.Body)
	}
	if echo.Seq != heard.Seq || echo.From != echo.From {
		t.Errorf("the room's one broadcast must be identical everywhere: %+v vs %+v", echo, heard)
	}
	if echo.From != "guest-1" || heard.From != "guest-1" {
		t.Errorf("from = %q / %q, want guest-1 on both", echo.From, heard.From)
	}

	// Frames the room does not understand are ignored defensively:
	// neither a wrong type nor undecodable JSON ends or echoes anything.
	sendChat(t, grace, `{"type":"presence","body":"ping"}`)
	sendChat(t, grace, `not json at all`)
	sendChat(t, grace, `{"type":"chat","body":"   "}`) // blank: ignored
	sendChat(t, ada, `{"type":"chat","body":"second line"}`)
	next := waitFor(t, grace, func(m chatMessage) bool { return m.Kind == "chat" })
	if next.Body != "second line" {
		t.Errorf("after hostile frames the next broadcast = %q, want second line (the noise ignored)", next.Body)
	}
}

func TestRealtimeChatRefusesBinaryFramesWithCloseUnsupportedData(t *testing.T) {
	server := newChatServer(t, testChatPolicy())
	ada := dialChat(t, server, "")
	readChatMessage(t, ada) // the join notice: the connection is live

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := ada.Write(ctx, websocket.MessageBinary, []byte{0x00, 0x01, 0x02}); err != nil {
		t.Fatalf("write binary: %v", err)
	}
	// The server must end the connection with 1003 — the read loop
	// returns the typed close and the connection is gone.
	_, _, err := ada.Read(readCtx(t))
	if code := websocket.CloseStatus(err); code != websocket.StatusUnsupportedData {
		t.Fatalf("close status after a binary frame = %v (err %v), want 1003", code, err)
	}
}

func TestRealtimeChatLibraryClosesOversizedFramesWithTooBig(t *testing.T) {
	policy := testChatPolicy()
	policy.ReadLimit = 64 // small enough to exceed with one frame
	server := newChatServer(t, policy)
	ada := dialChat(t, server, "")
	readChatMessage(t, ada) // join notice

	sendChat(t, ada, fmt.Sprintf(`{"type":"chat","body":"%s"}`, strings.Repeat("x", 200)))
	_, _, err := ada.Read(readCtx(t))
	if code := websocket.CloseStatus(err); code != websocket.StatusMessageTooBig {
		t.Fatalf("close status after an oversized frame = %v (err %v), want 1009", code, err)
	}
}

func TestRealtimeChatEvictsSlowConsumersThroughTheLiveRoom(t *testing.T) {
	server := newChatServer(t, testChatPolicy())
	ada := dialChat(t, server, "")
	readChatMessage(t, ada) // join notice

	// A second seat in the SAME room that never reads — the slow
	// consumer, by construction (kernel buffers cannot save it: its
	// buffer is the hub's channel, not a socket).
	stalled := &realtime.Subscriber{ID: "stalled"}
	server.hub.Join("chat", stalled)

	// Buffer 2 + budget 2: publishes 1 and 2 fill the stalled seat,
	// 3 and 4 drop it twice — the fourth evicts. ada reads every echo
	// as it is sent (a reading member keeps full service — and its
	// pongs keep the connection alive), while the stalled seat cannot.
	for i := 1; i <= 4; i++ {
		body := fmt.Sprintf("line %d", i)
		sendChat(t, ada, fmt.Sprintf(`{"type":"chat","body":"%s"}`, body))
		waitFor(t, ada, func(m chatMessage) bool { return m.Kind == "chat" && m.Body == body })
	}
	// The eviction closes the stalled channel (after its two buffered
	// messages drain) with the policy code — bounded, never a bare
	// blocking receive.
	deadline := time.After(3 * time.Second)
	for open := true; open; {
		var drained realtime.Message
		select {
		case drained, open = <-stalled.Messages():
		case <-deadline:
			t.Fatal("the stalled subscriber was never evicted")
		}
		_ = drained
	}
	code, why := stalled.Eviction()
	if code != realtime.CloseSlowConsumer || why == "" {
		t.Errorf("eviction = (%d, %q), want 1013 with the policy reason", uint16(code), why)
	}
	// The healthy member lost nothing: the send loop's per-line waitFor
	// already consumed all four echoes — reading members keep full
	// service while the stalled seat loses its own.
}

func TestRealtimeChatLeavesTheRoomOnClientClose(t *testing.T) {
	server := newChatServer(t, testChatPolicy())
	ada := dialChat(t, server, "")
	grace := dialChat(t, server, "")
	waitFor(t, grace, func(m chatMessage) bool { return strings.Contains(m.Body, "2 online") })

	// ada says goodbye with an orderly close: the room must publish the
	// leave notice and the seat must be gone.
	ada.Close(websocket.StatusNormalClosure, "goodbye")
	left := waitFor(t, grace, func(m chatMessage) bool { return strings.Contains(m.Body, "left the room") })
	if !strings.Contains(left.Body, "1 online") {
		t.Errorf("leave notice = %q, want the honest count after the seat left", left.Body)
	}
	deadline := time.Now().Add(3 * time.Second)
	for server.hub.RoomCount("chat") != 1 {
		if time.Now().After(deadline) {
			t.Fatalf("seat leaked after close: %d still in the room", server.hub.RoomCount("chat"))
		}
		time.Sleep(5 * time.Millisecond)
	}

	// The room still works for the remaining member.
	sendChat(t, grace, `{"type":"chat","body":"still here"}`)
	echo := waitFor(t, grace, func(m chatMessage) bool { return m.Kind == "chat" })
	if echo.Body != "still here" {
		t.Errorf("post-leave broadcast = %q", echo.Body)
	}
}

func TestRealtimeChatRefusesCrossOriginHandshakes(t *testing.T) {
	server := newChatServer(t, testChatPolicy())
	// An origin the room never authorized: the handshake is refused
	// BEFORE any upgrade — the dial sees the plain HTTP 403, and the
	// room never gains the seat.
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, _, err := websocket.Dial(ctx, server.URL+"/api/demo/realtime/chat/ws",
		&websocket.DialOptions{HTTPHeader: http.Header{"Origin": []string{"https://evil.example"}}})
	if err == nil {
		t.Fatal("an unauthorized origin must be refused before the upgrade")
	}
	if !strings.Contains(err.Error(), "403") {
		t.Errorf("refusal = %v, want the plain HTTP 403", err)
	}
	if got := server.hub.RoomCount("chat"); got != 0 {
		t.Errorf("refused origin still joined: room count = %d", got)
	}
}

// readCtx is a bounded context for one client-side read.
func readCtx(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	t.Cleanup(cancel)
	return ctx
}

// ── the two story endpoints ───────────────────────────────────────────────

func TestRealtimeStoryEndpointsServeTheStoryShape(t *testing.T) {
	server := newChatServer(t, testChatPolicy())
	for _, tc := range []struct {
		path       string
		mechanism  string
		beats      int
		liveNeedle string
	}{
		{"/api/demo/realtime/ticker", "sse-ticker", 6, demos.RealtimeTickerLivePath},
		{"/api/demo/realtime/chat", "ws-chat", 6, demos.RealtimeChatLivePath},
	} {
		resp, err := server.Client().Get(server.URL + tc.path)
		if err != nil {
			t.Fatalf("%s: %v", tc.path, err)
		}
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("%s: status = %d", tc.path, resp.StatusCode)
		}
		var payload struct {
			Mechanism string `json:"mechanism"`
			Beats     []struct {
				Beat    string `json:"beat"`
				Verdict string `json:"verdict"`
				Facts   []any  `json:"facts"`
			} `json:"beats"`
			Proofs    []any  `json:"proofs"`
			Reasoning string `json:"reasoning"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
			t.Fatalf("%s: decode: %v", tc.path, err)
		}
		resp.Body.Close()
		if payload.Mechanism != tc.mechanism || len(payload.Beats) != tc.beats {
			t.Errorf("%s: mechanism/beats = %q/%d", tc.path, payload.Mechanism, len(payload.Beats))
		}
		if len(payload.Beats) == 0 || payload.Proofs == nil || payload.Reasoning == "" {
			t.Errorf("%s: payload lacks beats/proofs/reasoning", tc.path)
		}
		for _, beat := range payload.Beats {
			if beat.Facts == nil {
				t.Errorf("%s: beat %q marshals null facts", tc.path, beat.Beat)
			}
		}
		// The story must point at the LIVE route this test server itself
		// would serve — the terminal companion and client panel follow it.
		var encoded []byte
		resp2, _ := server.Client().Get(server.URL + tc.path)
		encoded, _ = io.ReadAll(resp2.Body)
		resp2.Body.Close()
		if !strings.Contains(string(encoded), tc.liveNeedle) {
			t.Errorf("%s: story never names its live endpoint %s", tc.path, tc.liveNeedle)
		}
	}
}
