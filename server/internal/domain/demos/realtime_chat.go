// The realtime:chat story builder — every number it quotes comes from a
// REAL hub choreography run synchronously inside the builder (three
// members, one of whom never reads: delivered counts, drops, and the
// eviction are the hub's own answers) and from the shared ChatPolicy
// the live handler enforces. No goroutines, no I/O — deterministic and
// byte-stable per fetch.
package demos

import (
	"fmt"
	"strings"

	"guide/internal/domain/realtime"
)

// BuildRealtimeChatPayload shapes the realtime:chat story: the upgrade
// handshake, the room broadcast a real hub served, the ceiling's
// numbers, the slow-consumer eviction the hub actually performed, the
// ping/pong and close-code table, and the pointer to the live room.
func BuildRealtimeChatPayload() RealtimeStoryPayload {
	// A real, deterministic choreography: buffer 2, drop budget 2, three
	// members. ada and grace drain their channels; "stalled" never
	// reads — the eviction below is the hub's own verdict, staged.
	hub, err := realtime.NewHub(realtime.HubPolicy{Buffer: 2, MaxDrops: 2})
	if err != nil {
		panic(fmt.Sprintf("chat story hub: %v", err))
	}
	ada, grace, stalled := &realtime.Subscriber{ID: "ada"},
		&realtime.Subscriber{ID: "grace"},
		&realtime.Subscriber{ID: "stalled"}
	for _, sub := range []*realtime.Subscriber{ada, grace, stalled} {
		hub.Join("lobby", sub)
	}
	publish := func(from, body string) realtime.BroadcastStats {
		_, stats := hub.Publish("lobby", "chat", from, body, realtimeSampleAt)
		select {
		case <-ada.Messages():
		default:
		}
		select {
		case <-grace.Messages():
		default:
		}
		return stats
	}
	first := publish("ada", "hello room — first broadcast")
	second := publish("grace", "and the room hears back") // stalled buffer: 2/2
	third := publish("ada", "a third line")               // stalled drops: 1
	fourth := publish("ada", "a fourth line")             // stalled drops: 2 → evicted

	policy := realtime.DefaultChatPolicy()
	closeCodes := []realtime.CloseCode{
		realtime.CloseNormal, realtime.CloseGoingAway, realtime.CloseUnsupportedData,
		realtime.ClosePolicyViolation, realtime.CloseMessageTooLarge,
		realtime.CloseInternalError, realtime.CloseSlowConsumer,
	}
	codeFacts := make([]RealtimeStoryFact, 0, len(closeCodes))
	for _, code := range closeCodes {
		codeFacts = append(codeFacts, RealtimeStoryFact{
			Label: fmt.Sprintf("%d", uint16(code)),
			Value: code.Explain(),
		})
	}

	beats := []RealtimeStoryBeat{
		{
			Beat: "the handshake", Verdict: RealtimeVerdictOK,
			Detail: "a WebSocket starts life as ONE ordinary HTTP request the server promotes: the browser asks to Upgrade, answers with 101 Switching Protocols, and from that byte on the connection is no longer HTTP — it is a bidirectional frame protocol. Before anything is upgraded the server still has every HTTP-level control: the origin check runs at accept time, and a refused handshake is a plain HTTP error, never a socket:",
			Facts: []RealtimeStoryFact{
				{Label: "the request", Value: "GET " + RealtimeChatLivePath + " with Upgrade: websocket, Connection: Upgrade, Sec-WebSocket-Key, Sec-WebSocket-Version: 13"},
				{Label: "the answer", Value: "101 Switching Protocols + Sec-WebSocket-Accept — computed from the key, per RFC 6455"},
				{Label: "origin policy", Value: "the request's own host is always authorized; the dev Vite proxy's origin is pattern-allowed explicitly — cross-origin by accident is refused pre-upgrade"},
				{Label: "subprotocol", Value: "none negotiated — this chat speaks JSON text frames, which needs no subprotocol"},
			},
		},
		{
			Beat: "rooms and broadcast", Verdict: RealtimeVerdictOK,
			Detail: "the live room is a hub with named rooms: joining wires your bounded channel, leaving unwires it, and one Publish stamps a per-room sequence number and hands the message to EVERY member non-blockingly. The counts below are from a real choreography this payload just ran — one broadcast into a three-member room (sender included: chat echoes to everyone, the sender's own line coming back is the confirmation it was served):",
			Facts: []RealtimeStoryFact{
				{Label: "the choreography", Value: fmt.Sprintf("publish #1 (seq %d) → all %d members served instantly", first.Seq, first.Delivered)},
				{Label: "publish #2", Value: fmt.Sprintf("seq %d, delivered %d — the room is in full swing", second.Seq, second.Delivered)},
				{Label: "who hears", Value: "every member of the room, the sender included — one room, one shared sequence"},
				{Label: "system notices", Value: "joins and leaves publish kind:\"system\" messages through the same path the chat lines take"},
			},
		},
		{
			Beat: "the ceiling", Verdict: RealtimeVerdictOK,
			Detail: "a chat server's real risk is not CPU — it is the read that never comes. Every connection costs one goroutine per direction (a read pump and a write pump), every subscriber one bounded buffer, and every publish is a non-blocking send: O(members) per broadcast, and no slow reader can ever stall the publisher. The numbers are the live room's own policy:",
			Facts: []RealtimeStoryFact{
				{Label: "per connection", Value: "2 goroutines (read pump + write pump) and 1 buffered channel"},
				{Label: "per subscriber", Value: fmt.Sprintf("a %d-message buffer — the memory ceiling per member", policy.Buffer)},
				{Label: "per broadcast", Value: "non-blocking send to each member — a full buffer is the subscriber's problem, never the publisher's"},
				{Label: "inbound ceiling", Value: fmt.Sprintf("frames are capped at %d bytes (SetReadLimit) — the library closes oversized frames itself", policy.ReadLimit)},
				{Label: "liveness", Value: fmt.Sprintf("a protocol ping every %s; the library answers inbound pings automatically", policy.Heartbeat)},
			},
		},
		{
			Beat: "the slow consumer", Verdict: RealtimeVerdictRejected,
			Detail: "the ceiling is enforced, not advisory — this eviction actually happened in the choreography above. \"stalled\" joined, then stopped reading: its 2-slot buffer filled, the next two broadcasts each cost it one counted drop, and the second drop reached its budget — the hub removed it and closed its channel, and its write pump ends the connection with the policy close code. The healthy members never noticed:",
			Facts: []RealtimeStoryFact{
				{Label: "publish #3", Value: fmt.Sprintf("seq %d: %d delivered, %d dropped — stalled's buffer was full", third.Seq, third.Delivered, third.Dropped)},
				{Label: "publish #4", Value: fmt.Sprintf("seq %d: the drop budget hit — evicted: [%s]", fourth.Seq, strings.Join(fourth.Evicted, ", "))},
				{Label: "the eviction close", Value: fmt.Sprintf("code %d — %s", uint16(realtime.CloseSlowConsumer), realtime.CloseSlowConsumer.Explain())},
				{Label: "why evict", Value: "an unread buffer is unbounded memory by another name — the room's cost must be bounded by engaged readers"},
			},
		},
		{
			Beat: "ping/pong and the close codes", Verdict: RealtimeVerdictInfo,
			Detail: "liveness and endings are part of the protocol, not afterthoughts. The library answers inbound pings with pongs in the background (no handler code); the write pump pings on the heartbeat. Endings are named: a close frame carries a code, and every way this stack ends a connection has its own — the table is the domain's single source, the same strings the handler closes with:",
			Facts:  codeFacts,
		},
		{
			Beat: "this very room", Verdict: RealtimeVerdictInfo,
			Detail: "the live room runs beside this story at the endpoint below — the live panel in this demo page connects as a guest (join, say something, watch the system notices and your own echo arrive), and from a terminal websocat or wscat speak the same JSON protocol. The room's hub, buffers, and eviction policy are the ones this story just quoted:",
			Facts: []RealtimeStoryFact{
				{Label: "endpoint", Value: "GET " + RealtimeChatLivePath + " (Upgrade: websocket)"},
				{Label: "client → server", Value: `{"type":"chat","body":"your line"}` + " — anything else is ignored defensively"},
				{Label: "server → client", Value: `{"seq":12,"kind":"chat"|"system","from":"guest-3","body":"…","at":"…"}`},
				{Label: "terminal", Value: "websocat ws://localhost:8100" + RealtimeChatLivePath},
			},
		},
	}
	return newRealtimeStoryPayload("ws-chat",
		beats,
		[]RealtimeStoryFact{
			{Label: "ordering", Value: fmt.Sprintf("per-room seq, strictly monotonic (publishes %d, %d, %d, %d) — clients render in seq order, never arrival order", first.Seq, second.Seq, third.Seq, fourth.Seq)},
			{Label: "fan-out", Value: "one Publish = one non-blocking send per member — a stalled member costs a drop, never a stall"},
			{Label: "eviction math", Value: fmt.Sprintf("live policy: buffer %d + budget %d drops → close %d", policy.Buffer, policy.MaxDrops, uint16(realtime.CloseSlowConsumer))},
			{Label: "protocol", Value: "JSON over text frames; binary frames are refused with 1003; oversized frames the library closes with 1009"},
		},
		"Every count above is the hub's own answer to a choreography this payload ran live — delivered to three members, then the overflow, then the eviction of the member that stopped reading. That is the WebSocket half of realtime in one picture: an upgraded connection per guest, one hub holding the rooms, bounded buffers keeping every party honest, sequence numbers making order survivable, pings keeping liveness cheap, and named close codes making every ending legible. SSE carries the stream; WebSockets carry the conversation; the hub beneath them is the same discipline — bounded fan-out, explicit slow-consumer policy, honest teardown.")
}
